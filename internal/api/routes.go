package api

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
)

func Routes(staticFS fs.FS, sm *scs.SessionManager) chi.Router {
	r := chi.NewRouter()

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
		r.Get("/me", MeHandler(sm))
		r.Post("/setup", SetupHandler(sm))
		r.Post("/login", LoginHandler(sm))
		r.Post("/logout", LogoutHandler(sm))
	})

	r.NotFound(spaHandler(staticFS))
	r.Get("/*", spaHandler(staticFS))

	return r
}

func spaHandler(staticFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(staticFS))
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := staticFS.Open(path)
		if err == nil {
			defer f.Close()
			stat, err := f.Stat()
			if err == nil && !stat.IsDir() && path != "index.html" {
				r.URL.Path = "/" + path
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Serve index.html directly to avoid FileServer's redirect of /index.html -> ./
		index, err := staticFS.Open("index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer index.Close()
		stat, err := index.Stat()
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		data, err := io.ReadAll(index)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", stat.ModTime(), bytes.NewReader(data))
	}
}
