package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/store"
)

type areaDef struct {
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	BBox     [4]float64      `json:"bbox"`
	Geometry json.RawMessage `json:"geometry"`
}

type preparedArea struct {
	name string
	bbox [4]float64
	geom *geoGeom
}

type geoGeom struct {
	polys [][][][]float64 // MultiPolygon-style: []Polygon, each Polygon is []Ring, ring is [][2]float64
}

var (
	areasOnce sync.Once
	areasBy   map[string]*preparedArea
	areasErr  error
)

func loadAreas() (map[string]*preparedArea, error) {
	areasOnce.Do(func() {
		path := os.Getenv("COGS_AREAS_JSON")
		if path == "" {
			path = filepath.Join("public", "data", "areas.json")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			areasErr = err
			return
		}
		var list []areaDef
		if err := json.Unmarshal(raw, &list); err != nil {
			areasErr = err
			return
		}
		areasBy = make(map[string]*preparedArea, len(list))
		for _, a := range list {
			if a.Kind != "country" {
				continue
			}
			pa := &preparedArea{name: a.Name, bbox: a.BBox}
			if len(a.Geometry) > 0 {
				var g struct {
					Type        string          `json:"type"`
					Coordinates json.RawMessage `json:"coordinates"`
				}
				if json.Unmarshal(a.Geometry, &g) == nil {
					pa.geom = parseGeoCoords(g.Type, g.Coordinates)
				}
			}
			areasBy[a.Name] = pa
		}
	})
	return areasBy, areasErr
}

func parseGeoCoords(typ string, coords json.RawMessage) *geoGeom {
	switch typ {
	case "Polygon":
		var poly [][][]float64
		if json.Unmarshal(coords, &poly) != nil {
			return nil
		}
		return &geoGeom{polys: [][][][]float64{poly}}
	case "MultiPolygon":
		var mp [][][][]float64
		if json.Unmarshal(coords, &mp) != nil {
			return nil
		}
		return &geoGeom{polys: mp}
	default:
		return nil
	}
}

func pointInRing(x, y float64, ring [][]float64) bool {
	inside := false
	j := len(ring) - 1
	for i := 0; i < len(ring); i++ {
		xi, yi := ring[i][0], ring[i][1]
		xj, yj := ring[j][0], ring[j][1]
		if (yi > y) != (yj > y) && x < ((xj-xi)*(y-yi))/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}

func pointInRings(x, y float64, rings [][][]float64) bool {
	inside := false
	for _, ring := range rings {
		if pointInRing(x, y, ring) {
			inside = !inside
		}
	}
	return inside
}

func pointInArea(lon, lat float64, a *preparedArea) bool {
	if a == nil {
		return false
	}
	s, w, n, e := a.bbox[0], a.bbox[1], a.bbox[2], a.bbox[3]
	if lat < s || lat > n || lon < w || lon > e {
		return false
	}
	if a.geom == nil {
		return true
	}
	for _, poly := range a.geom.polys {
		if pointInRings(lon, lat, poly) {
			return true
		}
	}
	return false
}

func inDrawnBox(lat, lon, south, west, north, east float64, hasBox bool) bool {
	if !hasBox {
		return true
	}
	return lat >= south && lat <= north && lon >= west && lon <= east
}

func filterCellsByCountry(cells []store.Cell, level int, country string, south, west, north, east float64, hasBox bool) ([]store.Cell, error) {
	byName, err := loadAreas()
	if err != nil {
		return nil, err
	}
	a := byName[country]
	if a == nil {
		return nil, nil
	}
	step := grid.Step(level)
	out := make([]store.Cell, 0, len(cells))
	for _, c := range cells {
		lat := grid.CenterLat(level, c.LatI)
		lon := grid.CenterLon(level, c.LonI)
		if !pointInArea(lon, lat, a) {
			continue
		}
		if !inDrawnBox(lat, lon, south, west, north, east, hasBox) {
			continue
		}
		_ = step
		out = append(out, c)
	}
	return out, nil
}
