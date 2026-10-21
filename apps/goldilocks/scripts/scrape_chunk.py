#!/usr/bin/env python3
"""One resumable TerraClimate stripe per run → Goldilocks level 3 (1/24°)."""

from __future__ import annotations

import argparse
import json
import sqlite3
import struct
import sys
import time
from pathlib import Path

NLAT = 1440
NLON = 2880
MONTHS = 12
LEVEL = 3
YEARS = list(range(2016, 2026))
VARS = ("tmax", "tmin", "ppt")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--db", required=True)
    ap.add_argument("--tc", default="/tmp/tc")
    args = ap.parse_args()
    tc = Path(args.tc)
    db = sqlite3.connect(args.db)
    db.execute("PRAGMA journal_mode=WAL")
    row = db.execute(
        "SELECT payload FROM scrape_cursor WHERE source='terraclima' AND cursor_key='main'"
    ).fetchone()
    state = json.loads(row[0]) if row and row[0] else {"year_i": 0, "var_i": 0, "lat_i": 0}
    yi, vi, lat_i = state["year_i"], state["var_i"], state["lat_i"]
    if yi >= len(YEARS):
        print("scrape: all years done")
        db.close()
        return 0
    year, var = YEARS[yi], VARS[vi]
    path = tc / f"TerraClimate_{var}_{year}.nc"
    if not path.exists():
        print(f"scrape: missing {path.name}, advance")
        _advance(db, state)
        db.close()
        return 0
    try:
        import numpy as np
        import netCDF4 as nc
    except ImportError:
        print("scrape: install numpy netCDF4 (skipping chunk)")
        _advance(db, state)
        db.close()
        return 0

    t0 = time.time()
    ds = nc.Dataset(path)
    variable = ds.variables[var]
    touched = 0
    for month in range(MONTHS):
        raw = np.ma.filled(variable[month], np.nan).astype(np.float32)
        stripe = raw.reshape(NLAT, 3, NLON, 3)[lat_i]
        for lon_i in range(NLON):
            cell = stripe[:, lon_i, :]
            if not np.isfinite(cell).any():
                continue
            val = float(np.nanmean(cell))
            existing = db.execute(
                "SELECT temp_c, precip_mm FROM cells WHERE level=? AND lat_i=? AND lon_i=?",
                (LEVEL, lat_i, lon_i),
            ).fetchone()
            temp_b = bytearray(existing[0]) if existing and existing[0] else bytearray(MONTHS * 2)
            rain_b = bytearray(existing[1]) if existing and existing[1] else bytearray(MONTHS * 2)
            if len(temp_b) < MONTHS * 2:
                temp_b = bytearray(MONTHS * 2)
            if len(rain_b) < MONTHS * 2:
                rain_b = bytearray(MONTHS * 2)
            if var == "ppt":
                struct.pack_into("<H", rain_b, month * 2, int(round(val)))
            elif var == "tmax":
                t = struct.unpack_from("<h", temp_b, month * 2)[0]
                tmin_guess = t
                tmin_row = db.execute(
                    "SELECT temp_c FROM cells WHERE level=? AND lat_i=? AND lon_i=?",
                    (LEVEL, lat_i, lon_i),
                ).fetchone()
                if tmin_row and tmin_row[0] and len(tmin_row[0]) >= (month + 1) * 2:
                    tmin_guess = struct.unpack_from("<h", tmin_row[0], month * 2)[0]
                mean = int(round((val * 100 + tmin_guess) / 2))
                struct.pack_into("<h", temp_b, month * 2, mean)
            else:
                t = struct.unpack_from("<h", temp_b, month * 2)[0]
                tmax_guess = t
                mean = int(round((tmax_guess + val * 100) / 2))
                struct.pack_into("<h", temp_b, month * 2, mean)
            db.execute(
                """
                INSERT INTO cells(level, lat_i, lon_i, temp_c, precip_mm, year_count)
                VALUES(?, ?, ?, ?, ?, 10)
                ON CONFLICT(level, lat_i, lon_i) DO UPDATE SET
                  temp_c = excluded.temp_c,
                  precip_mm = excluded.precip_mm
                """,
                (LEVEL, lat_i, lon_i, bytes(temp_b), bytes(rain_b)),
            )
            touched += 1
    ds.close()
    state["lat_i"] = lat_i + 1
    if state["lat_i"] >= NLAT:
        state["lat_i"] = 0
        state["var_i"] = vi + 1
        if state["var_i"] >= len(VARS):
            state["var_i"] = 0
            state["year_i"] = yi + 1
    db.execute(
        """
        INSERT INTO scrape_cursor(source, cursor_key, payload, updated_at)
        VALUES('terraclima', 'main', ?, datetime('now'))
        ON CONFLICT(source, cursor_key) DO UPDATE SET
          payload = excluded.payload, updated_at = excluded.updated_at
        """,
        (json.dumps(state),),
    )
    db.commit()
    db.close()
    print(f"scrape: {var} {year} lat_i={lat_i} touched={touched} in {time.time()-t0:.1f}s")
    return 0


def _advance(db: sqlite3.Connection, state: dict) -> None:
    state["lat_i"] = 0
    state["var_i"] = state.get("var_i", 0) + 1
    if state["var_i"] >= len(VARS):
        state["var_i"] = 0
        state["year_i"] = state.get("year_i", 0) + 1
    db.execute(
        """
        INSERT INTO scrape_cursor(source, cursor_key, payload, updated_at)
        VALUES('terraclima', 'main', ?, datetime('now'))
        ON CONFLICT(source, cursor_key) DO UPDATE SET payload=excluded.payload
        """,
        (json.dumps(state),),
    )
    db.commit()


if __name__ == "__main__":
    raise SystemExit(main())
