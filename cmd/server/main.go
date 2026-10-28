package main

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"errors"
	"fmt"
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
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/buildinfo"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/rsacrypt"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	buildinfo.Fprint(os.Stdout, buildVersion, buildDate, buildCommit)

	cfg, err := config.LoadServer(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	// After the first signal the default behaviour is restored, so a second one kills a hung shutdown.
	go func() {
		<-ctx.Done()
		stop()
	}()

	if err := run(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}

// run serves until ctx is cancelled. Shutdown order: stop accepting and drain HTTP requests,
// stop periodic saving, save once more, then close audit and the database (deferred).
// After a Shutdown timeout the remaining connections are closed before saving.
func run(ctx context.Context, cfg config.ServerConfig) error {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		wg.Wait()
	}()

	var privKey *rsa.PrivateKey
	if cfg.CryptoKey != "" {
		var err error
		if privKey, err = rsacrypt.LoadPrivateKey(cfg.CryptoKey); err != nil {
			return err
		}
	}

	trusted, err := subnet.New(cfg.TrustedSubnet)
	if err != nil {
		return err
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	var pinger handler.Pinger
	var storage repository.Storage
	var fileStorage *repository.FileStorage

	switch backendFor(cfg) {
	case backendPostgres:
		// sql.Open does not connect to the database, Migrate will check the connection.
		var db *sql.DB
		db, err = sql.Open("pgx", cfg.DatabaseDSN)
		if err != nil {
			return err
		}
		defer db.Close()
		if err = repository.Migrate(db); err != nil {
			return fmt.Errorf("migrations failed: %w", err)
		}
		pinger = db
		storage = repository.NewPgStorage(db)
	case backendFile:
		mem := repository.NewMemStorage()
		fileStorage = repository.NewFileStorage(mem, cfg.FileStoragePath, cfg.StoreInterval == 0)
		storage = fileStorage

		if cfg.Restore {
			if err = fileStorage.Restore(); err != nil {
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
		return err
	}
	if subject != nil {
		defer func() {
			if err := subject.Close(); err != nil {
				log.Printf("close audit: %v", err)
			}
		}()
		h.WithAudit(subject)
	}

	bodyMiddleware := []func(http.Handler) http.Handler{
		middleware.Decrypt(privKey),
		middleware.Sign(cfg.Key),
		middleware.Gzip,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logging(logger))

	// Only writes are restricted to the trusted subnet. The check comes first, so a body
	// from an untrusted agent is not decrypted or verified.
	r.Group(func(r chi.Router) {
		r.Use(middleware.TrustedSubnet(trusted))
		r.Use(bodyMiddleware...)

		r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
		r.Post("/update", h.UpdateMetricJSON)
		r.Post("/updates/", h.UpdateMetricsBatch)
	})

	r.Group(func(r chi.Router) {
		r.Use(bodyMiddleware...)

		// Incomplete update paths store nothing, so they answer 404 without the subnet check.
		r.Post("/update/{type}/{name}/", http.NotFound)
		r.Post("/update/{type}/", http.NotFound)
		r.Post("/update/", http.NotFound)

		r.Post("/value", h.GetMetricJSON)

		r.Get("/value/{type}/{name}", h.GetMetric)

		r.Get("/ping", h.Ping)

		r.Get("/", h.GetAllMetrics)
	})

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

	var runErr error
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = err
		}
	case <-ctx.Done():
		log.Println("shutting down")
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancelShutdown()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
			if err := srv.Close(); err != nil {
				log.Printf("close server: %v", err)
			}
		}
	}

	cancel()
	wg.Wait() // background saving must finish before the final save

	// A server that never started has nothing to save and the file may hold data that was not restored.
	if fileStorage != nil && runErr == nil {
		if err := fileStorage.Save(); err != nil {
			log.Printf("final save failed: %v", err)
		}
	}
	return runErr
}

const shutdownTimeout = 5 * time.Second

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
	return func() {
		if err := srv.Close(); err != nil {
			log.Printf("close pprof server: %v", err)
		}
	}
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
