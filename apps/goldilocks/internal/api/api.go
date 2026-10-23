package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/store"
)

type API struct {
	Store *store.Store
	Blobs BlobCache
}

type BlobCache interface {
	Get(key string) ([]byte, bool)
	Put(key string, data []byte) error
}

type noopBlob struct{}

func (noopBlob) Get(string) ([]byte, bool) { return nil, false }
func (noopBlob) Put(string, []byte) error  { return nil }

func (a *API) Register(m *http.ServeMux) {
	m.HandleFunc("GET /world/overview.json", a.handleOverview)
	m.HandleFunc("GET /world/t/{path...}", a.handleWorldTilePath)
	m.HandleFunc("GET /tiles/{level}/{sy}/{sx}", a.handleClimateBin)
	m.HandleFunc("GET /api/tiles/{z}/{x}/{y}", a.handleMapTile)
	m.HandleFunc("GET /api/cells", a.handleCells)
}

func (a *API) handleWorldTilePath(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("path"), ".json")
	id = strings.TrimSuffix(id, "/")
	a.handleWorldTile(w, r, id)
}

func (a *API) handleOverview(w http.ResponseWriter, r *http.Request) {
	a.writeTileFile(w, r, grid.Level2Deg, -90, -180, 90, 180, grid.Step(grid.Level2Deg))
}

func (a *API) handleWorldTile(w http.ResponseWriter, r *http.Request, id string) {
	parts := strings.Split(id, "_")
	if len(parts) != 2 {
		http.Error(w, "bad tile id", http.StatusBadRequest)
		return
	}
	south, err1 := strconv.Atoi(parts[0])
	west, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		http.Error(w, "bad tile id", http.StatusBadRequest)
		return
	}
	north := float64(south + 10)
	east := float64(west + 10)
	if west >= 180 {
		east = 180
	}
	a.writeTileFile(w, r, grid.LevelHalf, float64(south), float64(west), north, east, 0.5)
}

func (a *API) handleMapTile(w http.ResponseWriter, r *http.Request) {
	z, _ := strconv.Atoi(r.PathValue("z"))
	x, _ := strconv.Atoi(r.PathValue("x"))
	y, _ := strconv.Atoi(r.PathValue("y"))
	north, west, south, east := grid.TileBounds(z, x, y)
	prefer := grid.LevelForZoom(z)
	cells, level, err := a.Store.CellsInBoxWithFallback(r.Context(), prefer, south, west, north, east)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	step := grid.Step(level)
	payload := map[string]any{
		"d": step,
		"z": z,
		"x": x,
		"y": y,
		"p": encodeRows(cells, step),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *API) handleCells(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	country := q.Get("country")
	south, _ := strconv.ParseFloat(q.Get("s"), 64)
	west, _ := strconv.ParseFloat(q.Get("w"), 64)
	north, _ := strconv.ParseFloat(q.Get("n"), 64)
	east, _ := strconv.ParseFloat(q.Get("e"), 64)
	level := grid.LevelHalf
	if lv := q.Get("level"); lv != "" {
		if n, err := strconv.Atoi(lv); err == nil {
			level = n
		}
	}
	hasBox := q.Has("s") && q.Has("w") && q.Has("n") && q.Has("e")
	var cells []store.Cell
	var err error
	if country != "" {
		area, err := loadAreas()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		def := area[country]
		if def == nil {
			http.Error(w, "unknown country", http.StatusBadRequest)
			return
		}
		south, west, north, east = def.bbox[0], def.bbox[1], def.bbox[2], def.bbox[3]
		boxS, boxW, boxN, boxE := south, west, north, east
		if hasBox {
			boxS, _ = strconv.ParseFloat(q.Get("s"), 64)
			boxW, _ = strconv.ParseFloat(q.Get("w"), 64)
			boxN, _ = strconv.ParseFloat(q.Get("n"), 64)
			boxE, _ = strconv.ParseFloat(q.Get("e"), 64)
			south = maxf(south, boxS)
			west = maxf(west, boxW)
			north = minf(north, boxN)
			east = minf(east, boxE)
		}
		cells, level, err = a.Store.CellsInBoxWithFallback(r.Context(), level, south, west, north, east)
		if err == nil {
			cells, err = filterCellsByCountry(cells, level, country, boxS, boxW, boxN, boxE, hasBox)
		}
	} else {
		cells, level, err = a.Store.CellsInBoxWithFallback(r.Context(), level, south, west, north, east)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	step := grid.Step(level)
	payload := map[string]any{"d": step, "level": level, "p": encodeRows(cells, step)}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *API) writeTileFile(w http.ResponseWriter, r *http.Request, level int, south, west, north, east, step float64) {
	cacheKey := ""
	if rev, _ := a.Store.Meta(r.Context(), "tile_cache_rev"); rev != "" {
		cacheKey = "tiles/" + rev + "/" + strconv.Itoa(level) + "/" + strconv.FormatFloat(south, 'f', 0, 64) + "_" + strconv.FormatFloat(west, 'f', 0, 64)
		if a.Blobs != nil {
			if b, ok := a.Blobs.Get(cacheKey); ok {
				w.Header().Set("Content-Type", "application/json")
				w.Write(b)
				return
			}
		}
	}
	cells, useLevel, err := a.Store.CellsInBoxWithFallback(r.Context(), level, south, west, north, east)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if step <= 0 {
		step = grid.Step(useLevel)
	}
	body, _ := json.Marshal(map[string]any{"d": step, "p": encodeRows(cells, step)})
	if cacheKey != "" && a.Blobs != nil && len(cells) > 0 {
		_ = a.Blobs.Put(cacheKey, body)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func encodeRows(cells []store.Cell, step float64) [][]any {
	out := make([][]any, 0, len(cells))
	for _, c := range cells {
		out = append(out, store.ExportRow(c, step))
	}
	return out
}
