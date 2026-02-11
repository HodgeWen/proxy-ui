package api

import (
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
			if stat, err := f.Stat(); err == nil && !stat.IsDir() {
				r.URL.Path = "/" + path
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, r)
	}
}
