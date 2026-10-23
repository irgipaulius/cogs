#!/usr/bin/env python3
"""One 1° climate tile (level 3) per run: decade monthly means from TerraClimate."""

from __future__ import annotations

import argparse
import json
import math
import sqlite3
import struct
import sys
import time
import urllib.request
from pathlib import Path

MONTHS = 12
LEVEL = 3
LEVEL_PARENT = 2
STEP = 1.0 / 24.0
STEP_PARENT = 0.125
YEARS = list(range(2016, 2026))
VARS = ("tmax", "tmin", "ppt")
BASE_URL = "https://climate.northwestknowledge.net/TERRACLIMATE-DATA/"
GLK_MAGIC = b"GLK1"

try:
    import numpy as np
    import netCDF4 as nc
    import zstandard as zstd
except ImportError:
    print("scrape: install numpy netCDF4 zstandard", file=sys.stderr)
    sys.exit(1)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--db", required=True)
    ap.add_argument("--tc", default="/tmp/tc")
    ap.add_argument("--download", action="store_true")
    args = ap.parse_args()

    tc = Path(args.tc)
    tc.mkdir(parents=True, exist_ok=True)
    db = sqlite3.connect(args.db)
    db.execute("PRAGMA journal_mode=WAL")

    row = db.execute(
        "SELECT payload FROM scrape_cursor WHERE source='terraclima' AND cursor_key='tile_v1'"
    ).fetchone()
    state = json.loads(row[0]) if row and row[0] else {"sy": -90, "sx": -180}
    sy, sx = int(state["sy"]), int(state["sx"])
    if sy >= 90:
        print("scrape: world pass complete")
        db.close()
        return 0

    if not files_ready(tc, args.download):
        print(f"scrape: NetCDF not ready for {sy}_{sx}, advance")
        advance(db, sy, sx)
        db.commit()
        db.close()
        return 0

    t0 = time.time()
    cells = build_tile(db, tc, sy, sx)
    raw = encode_glk1(STEP, float(sy), float(sx), cells)
    comp = zstd.ZstdCompressor(level=3).compress(raw)
    db.execute(
        """
        INSERT INTO climate_tiles(level, sy, sx, cells, bytes) VALUES(?,?,?,?,?)
        ON CONFLICT(level, sy, sx) DO UPDATE SET cells=excluded.cells, bytes=excluded.bytes
        """,
        (LEVEL, sy, sx, len(cells), comp),
    )
    advance(db, sy, sx)
    db.commit()
    db.close()
    print(f"scrape: tile {sy}_{sx} cells={len(cells)} in {time.time()-t0:.1f}s")
    return 0


def files_ready(tc: Path, download: bool) -> bool:
    for year in YEARS:
        for var in VARS:
            p = tc / f"TerraClimate_{var}_{year}.nc"
            if not p.exists():
                if not download:
                    return False
                url = BASE_URL + p.name
                print(f"scrape: download {p.name}")
                try:
                    urllib.request.urlretrieve(url, p)
                except Exception as e:
                    print(f"scrape: download failed {p.name}: {e}")
                    return False
    return True


def advance(db: sqlite3.Connection, sy: int, sx: int) -> None:
    sx += 1
    if sx >= 180:
        sx = -180
        sy += 1
    payload = json.dumps({"sy": sy, "sx": sx})
    db.execute(
        """
        INSERT INTO scrape_cursor(source, cursor_key, payload, updated_at)
        VALUES('terraclima', 'tile_v1', ?, datetime('now'))
        ON CONFLICT(source, cursor_key) DO UPDATE SET payload=excluded.payload, updated_at=excluded.updated_at
        """,
        (payload,),
    )


def lat_i(level_step: float, lat: float) -> int:
    if lat > 90:
        lat = 90 - 1e-9
    if lat < -90:
        lat = -90 + 1e-9
    return int(math.floor((90 - lat) / level_step - 0.5))


def lon_i(level_step: float, lon: float) -> int:
    if lon >= 180:
        lon = 180 - 1e-9
    if lon <= -180:
        lon = -180 + 1e-9
    return int(math.floor((lon + 180) / level_step - 0.5))


def parent_cloud_sunny(db: sqlite3.Connection, lat: float, lon: float) -> tuple[list[int] | None, int | None]:
    li, lj = lat_i(STEP_PARENT, lat), lon_i(STEP_PARENT, lon)
    row = db.execute(
        "SELECT cloud_pct, sunny_days FROM cells WHERE level=? AND lat_i=? AND lon_i=?",
        (LEVEL_PARENT, li, lj),
    ).fetchone()
    if not row:
        return None, None
    cloud_b, sunny = row
    cloud = None
    if cloud_b and len(cloud_b) >= MONTHS:
        cloud = list(cloud_b[:MONTHS])
    sunny_v = int(sunny) if sunny is not None else None
    return cloud, sunny_v


def build_tile(db: sqlite3.Connection, tc: Path, sy: int, sx: int) -> list[dict]:
    south, north, west, east = float(sy), float(sy + 1), float(sx), float(sx + 1)
    sample = nc.Dataset(tc / f"TerraClimate_tmax_{YEARS[0]}.nc")
    lats = np.asarray(sample.variables["lat"][:])
    lons = np.asarray(sample.variables["lon"][:])
    sample.close()

    lat_mask = (lats >= south) & (lats < north)
    lon_mask = (lons >= west) & (lons < east)
    if not lat_mask.any() or not lon_mask.any():
        return []
    li = np.where(lat_mask)[0]
    lj = np.where(lon_mask)[0]
    lat_sl = slice(int(li[0]), int(li[-1]) + 1)
    lon_sl = slice(int(lj[0]), int(lj[-1]) + 1)
    sub_lats = lats[lat_sl]
    sub_lons = lons[lon_sl]

    t_acc = np.zeros((MONTHS, sub_lats.size, sub_lons.size), dtype=np.float64)
    p_acc = np.zeros_like(t_acc)
    t_cnt = np.zeros_like(t_acc)
    p_cnt = np.zeros_like(t_acc)

    for year in YEARS:
        tmax = nc.Dataset(tc / f"TerraClimate_tmax_{year}.nc").variables["tmax"][:, lat_sl, lon_sl]
        tmin = nc.Dataset(tc / f"TerraClimate_tmin_{year}.nc").variables["tmin"][:, lat_sl, lon_sl]
        ppt = nc.Dataset(tc / f"TerraClimate_ppt_{year}.nc").variables["ppt"][:, lat_sl, lon_sl]
        tmean = (np.ma.filled(tmax, np.nan) + np.ma.filled(tmin, np.nan)) / 2.0
        pr = np.ma.filled(ppt, np.nan)
        valid_t = np.isfinite(tmean)
        valid_p = np.isfinite(pr)
        t_acc += np.where(valid_t, tmean, 0.0)
        p_acc += np.where(valid_p, pr, 0.0)
        t_cnt += valid_t.astype(np.float64)
        p_cnt += valid_p.astype(np.float64)

    cells: list[dict] = []
    for i, lat in enumerate(sub_lats):
        for j, lon in enumerate(sub_lons):
            if not np.any(t_cnt[:, i, j] > 0):
                continue
            temp = []
            rain = []
            for m in range(MONTHS):
                if t_cnt[m, i, j] > 0:
                    temp.append(int(round(t_acc[m, i, j] / t_cnt[m, i, j] * 10)))
                else:
                    temp.append(0)
                if p_cnt[m, i, j] > 0:
                    rain.append(int(round(p_acc[m, i, j] / p_cnt[m, i, j])))
                else:
                    rain.append(0)
            cloud, sunny = parent_cloud_sunny(db, float(lat), float(lon))
            cells.append(
                {
                    "lat": float(lat),
                    "lon": float(lon),
                    "temp": temp,
                    "rain": rain,
                    "cloud": cloud,
                    "sunny": sunny,
                }
            )
    cells.sort(key=lambda c: (c["lat"], c["lon"]))
    return cells


def encode_glk1(step: float, sy: float, sx: float, cells: list[dict]) -> bytes:
    buf = bytearray()
    buf += GLK_MAGIC
    buf += struct.pack("<f", step)
    buf += struct.pack("<f", sy)
    buf += struct.pack("<f", sx)
    buf += struct.pack("<I", len(cells))
    for c in cells:
        buf += struct.pack("<i", int(round(c["lat"] * 1e6)))
        buf += struct.pack("<i", int(round(c["lon"] * 1e6)))
        for v in c["temp"]:
            buf += struct.pack("<h", int(v))
        for v in c["rain"]:
            buf += struct.pack("<H", int(v))
        cloud = c.get("cloud")
        for m in range(MONTHS):
            if cloud and m < len(cloud):
                buf += bytes([min(254, int(cloud[m]))])
            else:
                buf += bytes([255])
        sunny = c.get("sunny")
        buf += struct.pack("<H", 0xFFFF if sunny is None else int(sunny))
    return bytes(buf)


if __name__ == "__main__":
    raise SystemExit(main())
