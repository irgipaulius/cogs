package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/store"
)

func (a *API) handleClimateBin(w http.ResponseWriter, r *http.Request) {
	level, err1 := strconv.Atoi(r.PathValue("level"))
	sy, err2 := strconv.Atoi(r.PathValue("sy"))
	sx, err3 := strconv.Atoi(r.PathValue("sx"))
	if err1 != nil || err2 != nil || err3 != nil {
		http.Error(w, "bad tile path", http.StatusBadRequest)
		return
	}
	if level < grid.Level2Deg || level > grid.LevelNative {
		http.Error(w, "bad level", http.StatusBadRequest)
		return
	}

	useLevel, fsy, fsx, cells, compressed, err := a.Store.GetClimateTileWithFallback(r.Context(), level, sy, sx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(compressed) == 0 {
		http.Error(w, "tile not found", http.StatusNotFound)
		return
	}

	rev, _ := a.Store.Meta(r.Context(), "tile_cache_rev")
	etag := fmt.Sprintf(`"%s-%d-%d-%d"`, rev, useLevel, fsy, fsx)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	raw, err := store.DecompressGLK1(compressed)
	if err != nil {
		http.Error(w, "bad tile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-GLK-Level", strconv.Itoa(useLevel))
	w.Header().Set("X-GLK-SY", strconv.Itoa(fsy))
	w.Header().Set("X-GLK-SX", strconv.Itoa(fsx))
	w.Header().Set("X-GLK-Cells", strconv.Itoa(cells))
	w.Header().Set("X-Content-Encoding", "glk1")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("ETag", etag)
	_, _ = w.Write(raw)
}
