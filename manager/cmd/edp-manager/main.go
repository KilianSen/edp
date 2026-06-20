// Command edp-manager orchestrates multiple edp instances: a registry of
// instances (seeded declaratively and editable at runtime), aggregate fan-out
// views across them, a per-instance pass-through proxy, and a bundled web UI.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"edp-manager/internal/bootstrap"
	"edp-manager/internal/config"
	"edp-manager/internal/server"
	"edp-manager/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("edp-manager ")

	cfg := config.Load()

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("create data dir %s: %v", cfg.DataDir, err)
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Seed the registry from the declarative config file (idempotent).
	if err := bootstrap.SeedFromFile(context.Background(), st, cfg.ConfigFile); err != nil {
		log.Fatalf("seed instances: %v", err)
	}

	srv, err := server.New(cfg, st)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	httpSrv := &http.Server{
		Addr:        cfg.Addr,
		Handler:     srv.Handler(),
		ReadTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (data=%s)", cfg.Addr, cfg.DataDir)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}
