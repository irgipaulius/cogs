package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
)

type Store struct {
	DB *sql.DB
}

func (s *Store) Meta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	return v, err
}

func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	return err
}

func (s *Store) CellCount(ctx context.Context, level int) (int64, error) {
	var n int64
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM cells WHERE level = ?`, level).Scan(&n)
	return n, err
}

func (s *Store) UpsertCell(ctx context.Context, c Cell) error {
	var cloud []byte
	if c.CloudPct != nil {
		cloud = PackCloud(*c.CloudPct)
	}
	var sunny sql.NullInt64
	if c.SunnyDays != nil {
		sunny = sql.NullInt64{Int64: int64(*c.SunnyDays), Valid: true}
	}
	yc := c.YearCount
	if yc <= 0 {
		yc = 10
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO cells(level, lat_i, lon_i, temp_c, precip_mm, cloud_pct, sunny_days, year_count)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(level, lat_i, lon_i) DO UPDATE SET
  temp_c = excluded.temp_c,
  precip_mm = excluded.precip_mm,
  cloud_pct = excluded.cloud_pct,
  sunny_days = excluded.sunny_days,
  year_count = excluded.year_count`,
		c.Level, c.LatI, c.LonI, PackTemp(c.TempC), PackPrecip(c.PrecipMM), cloud, sunny, yc)
	return err
}

func scanCell(level int, latI, lonI int, tb, pb, cb []byte, sunny sql.NullInt64, yc int) Cell {
	c := Cell{
		Level:     level,
		LatI:      latI,
		LonI:      lonI,
		TempC:     UnpackTemp(tb),
		PrecipMM:  UnpackPrecip(pb),
		CloudPct:  UnpackCloud(cb),
		YearCount: yc,
	}
	if sunny.Valid {
		v := int(sunny.Int64)
		c.SunnyDays = &v
	}
	return c
}

func (s *Store) CellsInBox(ctx context.Context, level int, south, west, north, east float64) ([]Cell, error) {
	minLat, maxLat := grid.LatRange(level, south, north)
	minLon, maxLon := grid.LonRange(level, west, east)
	rows, err := s.DB.QueryContext(ctx, `
SELECT lat_i, lon_i, temp_c, precip_mm, cloud_pct, sunny_days, year_count
FROM cells
WHERE level = ? AND lat_i BETWEEN ? AND ? AND lon_i BETWEEN ? AND ?`,
		level, minLat, maxLat, minLon, maxLon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cell
	for rows.Next() {
		var latI, lonI, yc int
		var tb, pb, cb []byte
		var sunny sql.NullInt64
		if err := rows.Scan(&latI, &lonI, &tb, &pb, &cb, &sunny, &yc); err != nil {
			return nil, err
		}
		out = append(out, scanCell(level, latI, lonI, tb, pb, cb, sunny, yc))
	}
	return out, rows.Err()
}

func (s *Store) CellsInBoxWithFallback(ctx context.Context, preferLevel int, south, west, north, east float64) ([]Cell, int, error) {
	for level := preferLevel; level >= grid.Level2Deg; level-- {
		cells, err := s.CellsInBox(ctx, level, south, west, north, east)
		if err != nil {
			return nil, 0, err
		}
		if len(cells) > 0 {
			return cells, level, nil
		}
	}
	return nil, preferLevel, nil
}

func (s *Store) CellsByCountry(ctx context.Context, level int, country string, south, west, north, east float64) ([]Cell, error) {
	minLat, maxLat := grid.LatRange(level, south, north)
	minLon, maxLon := grid.LonRange(level, west, east)
	rows, err := s.DB.QueryContext(ctx, `
SELECT c.lat_i, c.lon_i, c.temp_c, c.precip_mm, c.cloud_pct, c.sunny_days, c.year_count
FROM cells c
INNER JOIN cell_countries cc ON cc.level = c.level AND cc.lat_i = c.lat_i AND cc.lon_i = c.lon_i
WHERE c.level = ? AND cc.country = ? AND c.lat_i BETWEEN ? AND ? AND c.lon_i BETWEEN ? AND ?`,
		level, country, minLat, maxLat, minLon, maxLon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cell
	for rows.Next() {
		var latI, lonI, yc int
		var tb, pb, cb []byte
		var sunny sql.NullInt64
		if err := rows.Scan(&latI, &lonI, &tb, &pb, &cb, &sunny, &yc); err != nil {
			return nil, err
		}
		out = append(out, scanCell(level, latI, lonI, tb, pb, cb, sunny, yc))
	}
	return out, rows.Err()
}

func (s *Store) SetCellCountry(ctx context.Context, level, latI, lonI int, country string) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO cell_countries(level, lat_i, lon_i, country) VALUES(?, ?, ?, ?)
ON CONFLICT(level, lat_i, lon_i) DO UPDATE SET country = excluded.country`,
		level, latI, lonI, country)
	return err
}

func (s *Store) ScrapeCursorGet(ctx context.Context, source, key string) (string, error) {
	var payload string
	err := s.DB.QueryRowContext(ctx,
		`SELECT payload FROM scrape_cursor WHERE source = ? AND cursor_key = ?`, source, key).Scan(&payload)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return payload, err
}

func (s *Store) ScrapeCursorSet(ctx context.Context, source, key, payload string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO scrape_cursor(source, cursor_key, payload, updated_at) VALUES(?, ?, ?, ?)
ON CONFLICT(source, cursor_key) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at`,
		source, key, payload, now)
	return err
}

func (s *Store) BumpTileCacheRev(ctx context.Context) error {
	rev, err := s.Meta(ctx, "tile_cache_rev")
	if err == sql.ErrNoRows || rev == "" {
		return s.SetMeta(ctx, "tile_cache_rev", "1")
	}
	if err != nil {
		return err
	}
	var n int
	fmt.Sscanf(rev, "%d", &n)
	return s.SetMeta(ctx, "tile_cache_rev", fmt.Sprintf("%d", n+1))
}
