// c-hsm-doc-server — gRPC-Skeleton (Slice 001).
//
// Verkabelt den driving gRPC-Adapter (codes.Unimplemented-Stubs für
// alle vier Methoden) und den HTTP-Health-Endpoint hinter TLS 1.3.
// mTLS, Identity-Source-Konfiguration (`HSM-API-GRPC-006..008`),
// PKCS#11-Anbindung und Audit-Log kommen in M1-Folge-Slices.
//
// Bezug:
//   - Slice 001 (docs/plan/planning/in-progress/001-grpc-skeleton.md)
//   - HSM-API-GRPC-001..004, HSM-API-CFG-001..002, HSM-NFA-OPS-001..003
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	chsmdocv1 "github.com/pt9912/c-hsm-doc/internal/gen/chsmdocv1"
	"github.com/pt9912/c-hsm-doc/internal/config"

	healthadapter "github.com/pt9912/c-hsm-doc/internal/adapter/driving/health"
	grpcadapter "github.com/pt9912/c-hsm-doc/internal/adapter/driving/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const version = "0.1.0-slice-001"

func main() {
	os.Exit(appMain())
}

// appMain trennt die Programm-Logik vom os.Exit-Aufruf, damit
// `defer`-Aufräumarbeit (signal.NotifyContext) garantiert läuft;
// gocritic-Regel exitAfterDefer.
func appMain() int {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println("c-hsm-doc-server", version)
			return 0
		case "--help", "-h":
			fmt.Println("c-hsm-doc-server — gRPC skeleton (Slice 001).")
			fmt.Println("Configuration via environment variables: HSMDOC_GRPC_PORT, HSMDOC_HEALTH_PORT, HSMDOC_TLS_CERT, HSMDOC_TLS_KEY.")
			return 0
		}
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "error", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("server exited with error", "error", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	tlsCfg, err := loadServerTLS(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		return err
	}

	grpcLn, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("listen gRPC :%d: %w", cfg.GRPCPort, err)
	}
	healthLn, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HealthPort))
	if err != nil {
		_ = grpcLn.Close()
		return fmt.Errorf("listen health :%d: %w", cfg.HealthPort, err)
	}

	grpcSrv := newGRPCServer(tlsCfg)
	healthSrv := healthadapter.NewServer("")

	logger.Info("starting",
		"version", version,
		"grpc_port", cfg.GRPCPort,
		"health_port", cfg.HealthPort,
	)

	return serveAll(ctx, grpcSrv, grpcLn, healthSrv, healthLn, logger)
}

func newGRPCServer(tlsCfg *tls.Config) *grpc.Server {
	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsCfg)))
	chsmdocv1.RegisterHsmDocServiceServer(srv, grpcadapter.NewServer())
	return srv
}

func loadServerTLS(certPath, keyPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load TLS material: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}, nil
}

func serveAll(
	ctx context.Context,
	grpcSrv *grpc.Server,
	grpcLn net.Listener,
	healthSrv *http.Server,
	healthLn net.Listener,
	logger *slog.Logger,
) error {
	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSrv.Serve(grpcLn); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errs <- fmt.Errorf("grpc serve: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := healthadapter.Run(ctx, healthSrv, healthLn); err != nil {
			errs <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received", "reason", ctx.Err())
	case err := <-errs:
		logger.Error("server goroutine failed", "error", err)
		// Falls noch nicht beendet, stoppen wir den gRPC-Server, damit
		// die Health-Goroutine ihre eigene Shutdown-Sequenz laufen kann.
	}

	shutdownGRPC(grpcSrv, logger)

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func shutdownGRPC(srv *grpc.Server, logger *slog.Logger) {
	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-time.After(10 * time.Second):
		logger.Warn("graceful shutdown timed out, forcing stop")
		srv.Stop()
	}
}
