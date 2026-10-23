package store

import (
	"context"
	"database/sql"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
)

func (s *Store) PutClimateTile(ctx context.Context, level, sy, sx, cellCount int, compressed []byte) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO climate_tiles(level, sy, sx, cells, bytes) VALUES(?, ?, ?, ?, ?)
ON CONFLICT(level, sy, sx) DO UPDATE SET cells = excluded.cells, bytes = excluded.bytes`,
		level, sy, sx, cellCount, compressed)
	return err
}

func (s *Store) GetClimateTile(ctx context.Context, level, sy, sx int) (cells int, compressed []byte, err error) {
	err = s.DB.QueryRowContext(ctx,
		`SELECT cells, bytes FROM climate_tiles WHERE level = ? AND sy = ? AND sx = ?`,
		level, sy, sx).Scan(&cells, &compressed)
	if err == sql.ErrNoRows {
		return 0, nil, nil
	}
	return cells, compressed, err
}

func (s *Store) ClimateTileCount(ctx context.Context, level int) (int64, error) {
	var n int64
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM climate_tiles WHERE level = ?`, level).Scan(&n)
	return n, err
}

func snapTileOrigin(level, sy, sx int) (int, int) {
	if level == grid.Level2Deg {
		return -90, -180
	}
	span := int(grid.TileSpan(level))
	return (sy / span) * span, (sx / span) * span
}

func (s *Store) GetClimateTileWithFallback(ctx context.Context, level, sy, sx int) (useLevel, fsy, fsx, cells int, compressed []byte, err error) {
	for lv := level; lv >= grid.Level2Deg; lv-- {
		trySy, trySx := snapTileOrigin(lv, sy, sx)
		c, b, e := s.GetClimateTile(ctx, lv, trySy, trySx)
		if e != nil {
			return 0, 0, 0, 0, nil, e
		}
		if len(b) > 0 {
			return lv, trySy, trySx, c, b, nil
		}
	}
	return 0, 0, 0, 0, nil, nil
}
