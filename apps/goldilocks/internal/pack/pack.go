package pack

import (
	"context"
	"fmt"
	"log"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/store"
)

const metaPacked = "climate_tiles_packed"

// EnsurePacked builds zstd GLK1 tiles for levels 0–2 when cells exist but tiles do not.
func EnsurePacked(ctx context.Context, st *store.Store) error {
	if v, _ := st.Meta(ctx, metaPacked); v == "1" {
		n, err := st.ClimateTileCount(ctx, grid.Level2Deg)
		if err == nil && n > 0 {
			return nil
		}
	}
	n2, err := st.CellCount(ctx, grid.LevelEighth)
	if err != nil {
		return err
	}
	if n2 == 0 {
		return nil
	}
	log.Print("pack: building climate tiles for levels 0–2…")
	if err := packLevel(ctx, st, grid.Level2Deg); err != nil {
		return err
	}
	if err := packLevel1(ctx, st); err != nil {
		return err
	}
	if err := packLevel2(ctx, st); err != nil {
		return err
	}
	_ = st.SetMeta(ctx, metaPacked, "1")
	_ = st.BumpTileCacheRev(ctx)
	log.Print("pack: climate tiles ready")
	return nil
}

func packLevel(ctx context.Context, st *store.Store, level int) error {
	south, west, north, east := grid.TileBoundsForOrigin(level, -90, -180)
	cells, err := st.CellsInBox(ctx, level, south, west, north, east)
	if err != nil {
		return err
	}
	if len(cells) == 0 {
		return nil
	}
	step := grid.Step(level)
	raw := store.EncodeGLK1(step, -90, -180, cells)
	comp := store.CompressGLK1(raw)
	return st.PutClimateTile(ctx, level, -90, -180, len(cells), comp)
}

func packLevel1(ctx context.Context, st *store.Store) error {
	for sy := -90; sy < 90; sy += 10 {
		for sx := -180; sx < 180; sx += 10 {
			south, west, north, east := grid.TileBoundsForOrigin(grid.LevelHalf, sy, sx)
			cells, err := st.CellsInBox(ctx, grid.LevelHalf, south, west, north, east)
			if err != nil {
				return err
			}
			if len(cells) == 0 {
				continue
			}
			step := grid.Step(grid.LevelHalf)
			raw := store.EncodeGLK1(step, float64(sy), float64(sx), cells)
			comp := store.CompressGLK1(raw)
			if err := st.PutClimateTile(ctx, grid.LevelHalf, sy, sx, len(cells), comp); err != nil {
				return err
			}
		}
	}
	return nil
}

func packLevel2(ctx context.Context, st *store.Store) error {
	for sy := -90; sy < 90; sy += 2 {
		for sx := -180; sx < 180; sx += 2 {
			south, west, north, east := grid.TileBoundsForOrigin(grid.LevelEighth, sy, sx)
			cells, err := st.CellsInBox(ctx, grid.LevelEighth, south, west, north, east)
			if err != nil {
				return err
			}
			if len(cells) == 0 {
				continue
			}
			step := grid.Step(grid.LevelEighth)
			raw := store.EncodeGLK1(step, float64(sy), float64(sx), cells)
			comp := store.CompressGLK1(raw)
			if err := st.PutClimateTile(ctx, grid.LevelEighth, sy, sx, len(cells), comp); err != nil {
				return err
			}
		}
	}
	return nil
}

// RepackTile rebuilds one tile from cells table (used after scrape).
func RepackTileFromCells(ctx context.Context, st *store.Store, level, sy, sx int) error {
	south, west, north, east := grid.TileBoundsForOrigin(level, sy, sx)
	cells, err := st.CellsInBox(ctx, level, south, west, north, east)
	if err != nil {
		return err
	}
	step := grid.Step(level)
	raw := store.EncodeGLK1(step, float64(sy), float64(sx), cells)
	comp := store.CompressGLK1(raw)
	if err := st.PutClimateTile(ctx, level, sy, sx, len(cells), comp); err != nil {
		return fmt.Errorf("put tile: %w", err)
	}
	return nil
}
