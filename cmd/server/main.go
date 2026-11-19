package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/robfig/cron/v3"
	"github.com/s-ui/s-ui/internal/api"
	"github.com/s-ui/s-ui/internal/config"
	"github.com/s-ui/s-ui/internal/core"
	"github.com/s-ui/s-ui/internal/db"
	"github.com/s-ui/s-ui/internal/session"
	"github.com/s-ui/s-ui/web"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	_ = config.EnsureDir(cfg.DataDir)
	_ = os.MkdirAll(filepath.Dir(cfg.SingboxConfigPath), 0755)

	pm := core.NewProcessManagerFromConfig(cfg)
	if !pm.Available() {
		log.Printf("[warn] sing-box binary not found at configured path: %s", cfg.SingboxBinaryPath)
	}

	dbPath := config.DBPath(cfg.DataDir)
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("db init: %v", err)
	}

	if os.Getenv("V2RAY_API_ENABLED") == "true" {
		addr := os.Getenv("V2RAY_API_LISTEN")
		if addr == "" {
			addr = "127.0.0.1:8080"
		}
		statsClient := core.NewStatsClient(addr)
		intervalSec := 60
		if s := os.Getenv("V2RAY_STATS_INTERVAL"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				intervalSec = n
			}
		}
		c := cron.New()
		_, _ = c.AddFunc("@every "+strconv.Itoa(intervalSec)+"s", func() {
			_ = statsClient.FetchAndPersist(context.Background())
		})
		c.Start()
		log.Printf("[stats] cron started, polling every %ds", intervalSec)
	}

	secure := os.Getenv("FORCE_HTTPS") == "true" || os.Getenv("FORCE_HTTPS") == "1"
	sm, err := session.NewManager(db.DB, secure)
	if err != nil {
		log.Fatalf("session: %v", err)
	}

	distFS, err := fs.Sub(web.FS, "dist")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	r := api.Routes(distFS, sm, cfg)
	handler := sm.LoadAndSave(api.RequireSetupMiddleware(sm)(r))
	chi.Walk(r, func(method, route string, h http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("%s %s", method, route)
		return nil
	})

	log.Printf("listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler))
}
