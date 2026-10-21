package seed

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/store"
)

// ImportFromLegacy reads WIP_map goldilocks.sqlite (0.125° cells) into level LevelEighth,
// builds coarser levels, and optionally indexes countries from areasPath.
func ImportFromLegacy(ctx context.Context, dest *store.Store, sourcePath, areasPath string) error {
	src, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()

	log.Printf("seed: reading %s", sourcePath)
	rows, err := src.QueryContext(ctx, `SELECT lat_i, lon_i, temp_c, precip_mm, cloud_pct, sunny_days FROM cells`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		var latI, lonI int
		var tb, pb, cb []byte
		var sunny sql.NullInt64
		if err := rows.Scan(&latI, &lonI, &tb, &pb, &cb, &sunny); err != nil {
			return err
		}
		c := store.Cell{
			Level:    grid.LevelEighth,
			LatI:     latI,
			LonI:     lonI,
			TempC:    store.UnpackTemp(tb),
			PrecipMM: store.UnpackPrecip(pb),
			CloudPct: store.UnpackCloud(cb),
			YearCount: 10,
		}
		if sunny.Valid {
			v := int(sunny.Int64)
			c.SunnyDays = &v
		}
		if err := dest.UpsertCell(ctx, c); err != nil {
			return err
		}
		n++
		if n%100000 == 0 {
			log.Printf("seed: imported %d cells", n)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	log.Printf("seed: imported %d cells at 0.125°", n)

	if err := aggregateLevel(ctx, dest, grid.LevelEighth, grid.LevelHalf, 4); err != nil {
		return err
	}
	if err := aggregateLevel(ctx, dest, grid.LevelHalf, grid.Level2Deg, 4); err != nil {
		return err
	}
	_ = dest.SetMeta(ctx, "seed_source", sourcePath)
	_ = dest.SetMeta(ctx, "tile_cache_rev", "1")
	if areasPath != "" {
		if err := IndexCountries(ctx, dest, areasPath, grid.LevelEighth); err != nil {
			log.Printf("seed: country index: %v", err)
		}
	}
	return nil
}

func aggregateLevel(ctx context.Context, dest *store.Store, fromLevel, toLevel, bin int) error {
	log.Printf("seed: aggregating level %d -> %d (bin %d)", fromLevel, toLevel, bin)
	rows, err := dest.DB.QueryContext(ctx, `
SELECT lat_i, lon_i, temp_c, precip_mm, cloud_pct, sunny_days, year_count FROM cells WHERE level = ?`, fromLevel)
	if err != nil {
		return err
	}
	defer rows.Close()
	type acc struct {
		temp  [12]int64
		rain  [12]int64
		cloud [12]int64
		cn    [12]int
		sunny int64
		sn    int
		n     int
	}
	bins := map[[2]int]*acc{}
	for rows.Next() {
		var latI, lonI, yc int
		var tb, pb, cb []byte
		var sunny sql.NullInt64
		if err := rows.Scan(&latI, &lonI, &tb, &pb, &cb, &sunny, &yc); err != nil {
			return err
		}
		c := store.Cell{
			TempC:    store.UnpackTemp(tb),
			PrecipMM: store.UnpackPrecip(pb),
			CloudPct: store.UnpackCloud(cb),
		}
		bi := latI / bin
		bj := lonI / bin
		key := [2]int{bi, bj}
		a := bins[key]
		if a == nil {
			a = &acc{}
			bins[key] = a
		}
		a.n++
		for i := 0; i < 12; i++ {
			a.temp[i] += int64(c.TempC[i])
			a.rain[i] += int64(c.PrecipMM[i])
		}
		if c.CloudPct != nil {
			for i := 0; i < 12; i++ {
				a.cloud[i] += int64(c.CloudPct[i])
				a.cn[i]++
			}
		}
		if sunny.Valid {
			a.sunny += sunny.Int64
			a.sn++
		}
	}
	for key, a := range bins {
		if a.n == 0 {
			continue
		}
		var t [12]int16
		var p [12]uint16
		for i := 0; i < 12; i++ {
			t[i] = int16(a.temp[i] / int64(a.n))
			p[i] = uint16(a.rain[i] / int64(a.n))
		}
		var cloud *[12]uint8
		if a.cn[0] > 0 {
			var c [12]uint8
			for i := 0; i < 12; i++ {
				if a.cn[i] > 0 {
					c[i] = uint8((a.cloud[i] + int64(a.cn[i]/2)) / int64(a.cn[i]))
				}
			}
			cloud = &c
		}
		var sunnyPtr *int
		if a.sn > 0 {
			v := int((a.sunny + int64(a.sn/2)) / int64(a.sn))
			sunnyPtr = &v
		}
		if err := dest.UpsertCell(ctx, store.Cell{
			Level:     toLevel,
			LatI:      key[0],
			LonI:      key[1],
			TempC:     t,
			PrecipMM:  p,
			CloudPct:  cloud,
			SunnyDays: sunnyPtr,
			YearCount: 10,
		}); err != nil {
			return err
		}
	}
	log.Printf("seed: wrote %d aggregated cells at level %d", len(bins), toLevel)
	return rows.Err()
}

func NeedsSeed(ctx context.Context, dest *store.Store) (bool, error) {
	n, err := dest.CellCount(ctx, grid.LevelEighth)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

func DefaultSourcePath() string {
	return "/seed/goldilocks.sqlite"
}

func SeedMessage(n int64) string {
	return fmt.Sprintf("seed complete: %d cells at 0.125°", n)
}
