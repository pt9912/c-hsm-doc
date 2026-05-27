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
		if closeErr := grpcLn.Close(); closeErr != nil {
			logger.Warn("failed to close grpc listener after health-listen error", "error", closeErr)
		}
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
	// TODO(slice-002): MaxRecvMsgSize/Keepalive konfigurieren, sobald
	// Encrypt-Stream-Chunks landen. Default-Recv-Cap (4 MiB) reicht für
	// das Skeleton, deckt aber HSM-FA-CHUNK-008 nicht ab.
	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsCfg)))
	chsmdocv1.RegisterHsmDocServiceServer(srv, grpcadapter.NewServer())
	return srv
}

func loadServerTLS(certPath, keyPath string) (*tls.Config, error) {
	// TODO(slice-006): TLS-Material-Reload ohne Prozess-Restart, sobald
	// mTLS-Identitäts-Material rotiert wird (HSM-API-GRPC-003).
	cert, err := tls.LoadX509KeyPair(certPath, keyPath) //nolint:gosec // G304: Pfade kommen aus validierter Konfig (HSM-OPS-CFG-002).
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
	// runCtx kaskadiert beide Shutdown-Pfade in einen einzigen Auslöser:
	// bei Signal-Empfang ODER bei Goroutine-Fehler werden Health- und
	// gRPC-Listener gemeinsam abgebaut. Ohne den abgeleiteten Cancel
	// würde ein gRPC-Goroutine-Fehler den Health-Server weiterlaufen
	// lassen, weil sein eigener ctx noch nicht abgebrochen ist.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	sendErr := func(err error) {
		if err == nil {
			return
		}
		select {
		case errs <- err:
		default:
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSrv.Serve(grpcLn); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			sendErr(fmt.Errorf("grpc serve: %w", err))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := healthadapter.Run(runCtx, healthSrv, healthLn); err != nil {
			sendErr(err)
		}
	}()

	select {
	case <-runCtx.Done():
		logger.Info("shutdown signal received", "reason", runCtx.Err())
	case err := <-errs:
		logger.Error("server goroutine failed", "error", err)
		cancel() // bricht die Health-Goroutine
	}

	shutdownGRPC(grpcSrv, logger)
	wg.Wait()
	close(errs)

	var collected []error
	for err := range errs {
		if err != nil {
			collected = append(collected, err)
		}
	}
	return errors.Join(collected...)
}

func shutdownGRPC(srv *grpc.Server, logger *slog.Logger) {
	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()
	t := time.NewTimer(10 * time.Second)
	defer t.Stop()
	select {
	case <-done:
		return
	case <-t.C:
		logger.Warn("graceful shutdown timed out, forcing stop")
		srv.Stop()
		<-done
	}
}
