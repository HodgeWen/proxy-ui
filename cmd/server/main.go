package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/s-ui/s-ui/internal/api"
	"github.com/s-ui/s-ui/internal/db"
	"github.com/s-ui/s-ui/internal/session"
	"github.com/s-ui/s-ui/web"
)

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	os.MkdirAll(dataDir, 0755)

	dbPath := filepath.Join(dataDir, "s-ui.db")
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("db init: %v", err)
	}

	sm, err := session.NewManager(db.DB)
	if err != nil {
		log.Fatalf("session: %v", err)
	}

	distFS, err := fs.Sub(web.FS, "dist")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	r := api.Routes(distFS, sm)
	handler := sm.LoadAndSave(api.RequireSetupMiddleware(sm)(r))
	chi.Walk(r, func(method, route string, h http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("%s %s", method, route)
		return nil
	})

	addr := ":8080"
	if a := os.Getenv("ADDR"); a != "" {
		addr = a
	}
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
