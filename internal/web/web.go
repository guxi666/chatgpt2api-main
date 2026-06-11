package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed all:dist
var dist embed.FS

var staticFS = mustSubFS(dist, "dist")

func IndexHTML() ([]byte, error) {
	if data, err := fs.ReadFile(staticFS, "index.html"); err == nil {
		return data, nil
	}
	return fs.ReadFile(diskStaticFS(), "index.html")
}

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.Trim(strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/"), "/")
		if serveAsset(w, r, clean) {
			return
		}
		last := path.Base(clean)
		if strings.HasPrefix(clean, "assets/") || strings.Contains(last, ".") {
			http.NotFound(w, r)
			return
		}
		if serveAsset(w, r, "index.html") {
			return
		}
		http.NotFound(w, r)
	})
}

func serveAsset(w http.ResponseWriter, r *http.Request, name string) bool {
	if name == "" {
		name = "index.html"
	}
	for _, candidate := range assetCandidates(name) {
		for _, currentFS := range []fs.FS{staticFS, diskStaticFS()} {
			info, err := fs.Stat(currentFS, candidate)
			if err == nil && !info.IsDir() {
				if candidate == "index.html" {
					w.Header().Set("Cache-Control", "no-store")
				}
				http.ServeFileFS(w, r, currentFS, candidate)
				return true
			}
		}
	}
	return false
}

func assetCandidates(name string) []string {
	if name == "index.html" {
		return []string{name}
	}
	return []string{name, path.Join(name, "index.html"), name + ".html"}
}

func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func diskStaticFS() fs.FS {
	cwd, err := os.Getwd()
	if err != nil {
		return os.DirFS(".")
	}
	return os.DirFS(filepath.Join(cwd, "internal", "web", "dist"))
}
