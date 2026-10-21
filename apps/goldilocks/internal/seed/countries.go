package seed

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/store"
)

type areaDef struct {
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	BBox     [4]float64      `json:"bbox"`
	Geometry json.RawMessage `json:"geometry"`
}

// IndexCountries assigns each land cell at level to a country name (smallest bbox win handled in query layer;
// here we use first containing polygon by area order from file).
func IndexCountries(ctx context.Context, dest *store.Store, areasPath string, level int) error {
	raw, err := os.ReadFile(areasPath)
	if err != nil {
		return err
	}
	var areas []areaDef
	if err := json.Unmarshal(raw, &areas); err != nil {
		return err
	}
	step := grid.Step(level)
	rows, err := dest.DB.QueryContext(ctx, `SELECT lat_i, lon_i FROM cells WHERE level = ?`, level)
	if err != nil {
		return err
	}
	defer rows.Close()
	tx, err := dest.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO cell_countries(level, lat_i, lon_i, country) VALUES(?, ?, ?, ?)
ON CONFLICT(level, lat_i, lon_i) DO UPDATE SET country = excluded.country`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	var n int
	for rows.Next() {
		var latI, lonI int
		if err := rows.Scan(&latI, &lonI); err != nil {
			return err
		}
		lat := 90 - (float64(latI)+0.5)*step
		lon := -180 + (float64(lonI)+0.5)*step
		if lon >= 180 {
			lon -= 360
		}
		country := countryAt(lat, lon, areas)
		if country == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, level, latI, lonI, country); err != nil {
			return err
		}
		n++
		if n%200000 == 0 {
			if err := tx.Commit(); err != nil {
				return err
			}
			log.Printf("seed: indexed %d countries", n)
			tx, err = dest.DB.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			stmt, err = tx.PrepareContext(ctx, `
INSERT INTO cell_countries(level, lat_i, lon_i, country) VALUES(?, ?, ?, ?)
ON CONFLICT(level, lat_i, lon_i) DO UPDATE SET country = excluded.country`)
			if err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("seed: country index %d cells", n)
	return nil
}

func countryAt(lat, lon float64, areas []areaDef) string {
	var best string
	var bestA float64 = 1e18
	for _, a := range areas {
		if a.Kind != "country" {
			continue
		}
		s, w, n, e := a.BBox[0], a.BBox[1], a.BBox[2], a.BBox[3]
		if lat < s || lat > n || lon < w || lon > e {
			continue
		}
		area := (n - s) * (e - w)
		if area < bestA {
			bestA = area
			best = a.Name
		}
	}
	return best
}
