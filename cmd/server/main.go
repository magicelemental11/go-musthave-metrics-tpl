package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/audit"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func main() {
	cfg, err := config.LoadServer(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	var pinger handler.Pinger
	var storage repository.Storage
	var fileStorage *repository.FileStorage

	switch backendFor(cfg) {
	case backendPostgres:
		// sql.Open does not connect to the database, Migrate will check the connection.
		db, err := sql.Open("pgx", cfg.DatabaseDSN)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		if err := repository.Migrate(db); err != nil {
			log.Fatalf("migrations failed: %v", err)
		}
		pinger = db
		storage = repository.NewPgStorage(db)
	case backendFile:
		mem := repository.NewMemStorage()
		fileStorage = repository.NewFileStorage(mem, cfg.FileStoragePath, cfg.StoreInterval == 0)
		storage = fileStorage

		if cfg.Restore {
			if err := fileStorage.Restore(); err != nil {
				log.Printf("restore failed: %v", err)
			}
		}
		if cfg.StoreInterval > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				savePeriodically(ctx, fileStorage, cfg.StoreInterval)
			}()
		}
	default:
		storage = repository.NewMemStorage()
	}

	h := handler.NewHandler(storage, pinger)
	subject, err := buildAudit(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if subject != nil {
		defer subject.Close()
		h.WithAudit(subject)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Sign(cfg.Key))
	r.Use(middleware.Gzip)

	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

	r.Post("/update/{type}/{name}/", http.NotFound)
	r.Post("/update/{type}/", http.NotFound)
	r.Post("/update/", http.NotFound)

	r.Post("/update", h.UpdateMetricJSON)
	r.Post("/updates/", h.UpdateMetricsBatch)
	r.Post("/value", h.GetMetricJSON)

	r.Get("/value/{type}/{name}", h.GetMetric)

	r.Get("/ping", h.Ping)

	r.Get("/", h.GetAllMetrics)

	if cfg.PprofAddress != "" {
		defer startPprof(cfg.PprofAddress)()
	}

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Println("Server address:", cfg.Address)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}

	stop()
	wg.Wait() // background saving must finish before the final save

	if fileStorage != nil {
		if err := fileStorage.Save(); err != nil {
			log.Printf("final save failed: %v", err)
		}
	}
}

const (
	backendPostgres = "postgres"
	backendFile     = "file"
	backendMemory   = "memory"
)

func backendFor(cfg config.ServerConfig) string {
	switch {
	case cfg.DatabaseDSN != "":
		return backendPostgres
	case cfg.FileStoragePath != "":
		return backendFile
	default:
		return backendMemory
	}
}

func startPprof(addr string) (stop func()) {
	srv := &http.Server{Addr: addr, Handler: pprofMux(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Printf("pprof server: %v", err)
		}
	}()
	log.Println("pprof address:", addr)
	return func() { srv.Close() }
}

func pprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

func buildAudit(cfg config.ServerConfig) (*audit.Subject, error) {
	subject := &audit.Subject{}
	attached := 0
	if cfg.AuditFile != "" {
		fo, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			return nil, err
		}
		subject.Attach(fo)
		attached++
	}
	if cfg.AuditURL != "" {
		subject.Attach(audit.NewHTTPObserver(cfg.AuditURL))
		attached++
	}
	if attached == 0 {
		return nil, nil
	}
	return subject, nil
}

func savePeriodically(ctx context.Context, s *repository.FileStorage, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Save(); err != nil {
				log.Printf("periodic save failed: %v", err)
			}
		}
	}
}
