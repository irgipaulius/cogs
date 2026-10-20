package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var dist embed.FS

// Handler serves the embedded SPA (index.html for unknown paths).
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "ui not built", http.StatusInternalServerError)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && !fileExists(sub, r.URL.Path) {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func fileExists(fsys fs.FS, path string) bool {
	path = stringsTrimPrefix(path, "/")
	_, err := fs.Stat(fsys, path)
	return err == nil
}

func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
