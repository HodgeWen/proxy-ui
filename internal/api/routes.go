package api

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/s-ui/s-ui/internal/config"
)

func Routes(staticFS fs.FS, sm *scs.SessionManager, cfg *config.Config) chi.Router {
	r := chi.NewRouter()

	r.Get("/sub/{token}", SubscriptionHandler)

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
			r.Get("/status", StatusHandler(sm, cfg))
			r.Get("/versions", VersionsHandler(sm, cfg))
			r.Post("/start", StartHandler(sm, cfg))
			r.Post("/stop", StopHandler(sm, cfg))
			r.Post("/restart", RestartHandler(sm, cfg))
			r.Get("/logs", LogsHandler(sm, cfg))
			r.Post("/config", ConfigHandler(sm, cfg))
			r.Get("/config-file", ConfigFileHandler(sm, cfg))
			r.Post("/update", UpdateHandler(sm, cfg))
			r.Post("/rollback", RollbackHandler(sm, cfg))
		})
		r.Route("/stats", func(r chi.Router) {
			r.Use(RequireAuth(sm))
			r.Get("/summary", StatsSummaryHandler(sm))
		})
		r.Route("/inbounds", func(r chi.Router) {
			r.Use(RequireAuth(sm))
			r.Get("/", ListInboundsHandler(sm))
			r.Get("/{id}", GetInboundHandler(sm))
			r.Post("/", CreateInboundHandler(sm, cfg))
			r.Put("/{id}", UpdateInboundHandler(sm, cfg))
			r.Delete("/{id}", DeleteInboundHandler(sm, cfg))
		})
		r.Route("/certs", func(r chi.Router) {
			r.Use(RequireAuth(sm))
			r.Get("/", ListCertificatesHandler(sm))
			r.Get("/{id}", GetCertificateHandler(sm))
			r.Post("/", CreateCertificateHandler(sm))
			r.Put("/{id}", UpdateCertificateHandler(sm, cfg))
			r.Delete("/{id}", DeleteCertificateHandler(sm))
		})
		r.Route("/users", func(r chi.Router) {
			r.Use(RequireAuth(sm))
			r.Get("/", ListUsersHandler(sm))
			r.Post("/", CreateUserHandler(sm, cfg))
			r.Post("/batch", BatchUsersHandler(sm, cfg))
			r.Post("/{id}/reset-subscription", ResetSubscriptionHandler(sm))
			r.Get("/{id}", GetUserHandler(sm))
			r.Put("/{id}", UpdateUserHandler(sm, cfg))
			r.Delete("/{id}", DeleteUserHandler(sm, cfg))
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
