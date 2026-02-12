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
		r.Route("/core", func(r chi.Router) {
			r.Use(RequireAuth(sm))
			r.Get("/status", StatusHandler(sm))
			r.Post("/restart", RestartHandler(sm))
			r.Post("/config", ConfigHandler(sm))
		})
		r.Route("/inbounds", func(r chi.Router) {
			r.Use(RequireAuth(sm))
			r.Get("/", ListInboundsHandler(sm))
			r.Get("/{id}", GetInboundHandler(sm))
			r.Post("/", CreateInboundHandler(sm))
			r.Put("/{id}", UpdateInboundHandler(sm))
			r.Delete("/{id}", DeleteInboundHandler(sm))
		})
		r.Route("/certs", func(r chi.Router) {
			r.Use(RequireAuth(sm))
			r.Get("/", ListCertificatesHandler(sm))
			r.Get("/{id}", GetCertificateHandler(sm))
			r.Post("/", CreateCertificateHandler(sm))
			r.Put("/{id}", UpdateCertificateHandler(sm))
			r.Delete("/{id}", DeleteCertificateHandler(sm))
		})
		r.Route("/users", func(r chi.Router) {
			r.Use(RequireAuth(sm))
			r.Get("/", ListUsersHandler(sm))
			r.Post("/", CreateUserHandler(sm))
			r.Post("/batch", BatchUsersHandler(sm))
			r.Get("/{id}", GetUserHandler(sm))
			r.Put("/{id}", UpdateUserHandler(sm))
			r.Delete("/{id}", DeleteUserHandler(sm))
		})
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
