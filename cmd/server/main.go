package main

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
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
	"google.golang.org/grpc"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/audit"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/buildinfo"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/grpcserver"
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

// run serves HTTP, and gRPC when cfg.GRPCAddress is set, until ctx is cancelled or a server fails.
// Shutdown order: stop accepting and drain HTTP and gRPC requests in parallel, stop periodic saving,
// save once more, then close audit and the database (deferred).
// After shutdownTimeout the remaining connections of both servers are closed before saving.
// Both ports are bound before anything is served: if either is taken, run returns without saving,
// so a file that was not restored is not overwritten. Once serving has started, the final save
// runs even if a server fails.
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
			if closeErr := subject.Close(); closeErr != nil {
				log.Printf("close audit: %v", closeErr)
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
	httpLis, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}
	var grpcLis net.Listener
	if cfg.GRPCAddress != "" {
		if grpcLis, err = net.Listen("tcp", cfg.GRPCAddress); err != nil {
			httpLis.Close()
			return fmt.Errorf("grpc listen: %w", err)
		}
	}

	// Both servers report here; the buffer lets the second one exit after the first has failed.
	errCh := make(chan error, 2)
	go func() { errCh <- srv.Serve(httpLis) }()
	log.Println("Server address:", cfg.Address)

	var grpcSrv *grpc.Server
	if grpcLis != nil {
		grpcSrv = grpcserver.NewServer(h, trusted, logger)
		go func() { errCh <- grpcSrv.Serve(grpcLis) }()
		log.Println("gRPC address:", cfg.GRPCAddress)
	}

	var runErr error
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
			runErr = err
		}
	case <-ctx.Done():
		log.Println("shutting down")
	}
	// After a failure the other server is still running and must be stopped too.
	shutdown(srv, grpcSrv)

	cancel()
	wg.Wait() // background saving must finish before the final save

	if fileStorage != nil {
		if err := fileStorage.Save(); err != nil {
			log.Printf("final save failed: %v", err)
		}
	}
	return runErr
}

const shutdownTimeout = 5 * time.Second

// shutdown drains both servers in parallel within shutdownTimeout and then closes what is left.
// grpcSrv may be nil.
func shutdown(srv *http.Server, grpcSrv *grpc.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var wg sync.WaitGroup
	if grpcSrv != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stopGRPC(ctx, grpcSrv)
		}()
	}
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
		if err := srv.Close(); err != nil {
			log.Printf("close server: %v", err)
		}
	}
	wg.Wait()
}

// stopGRPC waits for in-flight calls like GracefulStop, which takes no context, and falls back
// to Stop when ctx expires first.
func stopGRPC(ctx context.Context, s *grpc.Server) {
	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		log.Printf("grpc shutdown: %v", ctx.Err())
		s.Stop()
		<-done
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
