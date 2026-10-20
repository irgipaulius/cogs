package todo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/irgipaulius/cogs/pkg/blobstore"
)

type API struct {
	Store     *Store
	Blobs     blobstore.Store
	MaxUpload int64
	KeyPrefix string
}

func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/todos", a.list)
	mux.HandleFunc("POST /api/todos", a.create)
	mux.HandleFunc("PATCH /api/todos/{id}", a.patch)
	mux.HandleFunc("DELETE /api/todos/{id}", a.delete)
	mux.HandleFunc("POST /api/todos/{id}/image", a.uploadImage)
	mux.HandleFunc("GET /api/todos/{id}/image", a.getImage)
	mux.HandleFunc("POST /api/stress/seed", a.stressSeed)
}

func clientID(r *http.Request) string {
	if c := r.Header.Get("X-Client-Id"); c != "" {
		return c
	}
	return "anonymous"
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	items, err := a.Store.List(r.Context(), since, clientID(r))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"todos": items})
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	var it Item
	if err := json.NewDecoder(r.Body).Decode(&it); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	it.ClientID = clientID(r)
	if err := a.Store.Upsert(r.Context(), it); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, it)
}

func (a *API) patch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var it Item
	if err := json.NewDecoder(r.Body).Decode(&it); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	it.ID = id
	it.ClientID = clientID(r)
	if err := a.Store.Upsert(r.Context(), it); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, it)
}

func (a *API) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.Store.SoftDelete(r.Context(), id, clientID(r)); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	w.WriteHeader(204)
}

func (a *API) uploadImage(w http.ResponseWriter, r *http.Request) {
	if a.Blobs == nil {
		http.Error(w, "blob storage not configured", 503)
		return
	}
	id := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUpload)
	if err := r.ParseMultipartForm(a.MaxUpload); err != nil {
		http.Error(w, "file too large or bad multipart", 413)
		return
	}
	file, hdr, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image field", 400)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	key := path.Join(a.KeyPrefix, id, hdr.Filename)
	ct := hdr.Header.Get("Content-Type")
	if err := a.Blobs.Put(r.Context(), key, data, ct); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	cid := clientID(r)
	if err := a.Store.SetImageKey(r.Context(), id, cid, key); err != nil {
		http.Error(w, "todo not found for this device — sync the item first", 404)
		return
	}
	writeJSON(w, map[string]string{"image_r2_key": key})
}

func (a *API) getImage(w http.ResponseWriter, r *http.Request) {
	if a.Blobs == nil {
		http.Error(w, "blob storage not configured", 503)
		return
	}
	id := r.PathValue("id")
	key, err := a.Store.ImageKeyByID(r.Context(), id)
	if err != nil {
		http.Error(w, "no image", 404)
		return
	}
	data, err := a.Blobs.Get(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	ct := "application/octet-stream"
	if strings.HasSuffix(strings.ToLower(key), ".png") {
		ct = "image/png"
	} else if strings.HasSuffix(strings.ToLower(key), ".jpg") || strings.HasSuffix(strings.ToLower(key), ".jpeg") {
		ct = "image/jpeg"
	}
	w.Header().Set("Content-Type", ct)
	_, _ = w.Write(data)
}

func (a *API) stressSeed(w http.ResponseWriter, r *http.Request) {
	n := 100
	fmt.Sscanf(r.URL.Query().Get("n"), "%d", &n)
	if n > 5000 {
		n = 5000
	}
	if err := a.Store.Seed(r.Context(), n, clientID(r)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]int{"seeded": n})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
