// Package health stellt /healthz (Liveness) und /readyz (Readiness)
// als reine HTTP-Endpoints bereit (HSM-API-CFG-001).
//
// Slice 001: beide Endpoints liefern immer 200 ("SERVING").
// HSM-Verfügbarkeit (`HSM-FA-FAIL-002`: Trennung Liveness/Readiness)
// wird in M2 verdrahtet.
package health

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// NewMux liefert einen ServeMux mit den beiden Endpoints registriert.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", livenessHandler)
	mux.HandleFunc("/readyz", readinessHandler)
	return mux
}

// NewServer baut einen http.Server, der den Health-Mux bedient.
func NewServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           NewMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// Run startet den Server und beendet sich sauber, wenn ctx fertig ist.
// Verwendet einen vorgegebenen Listener, damit Tests einen
// ephemeralen Port wählen können.
func Run(ctx context.Context, srv *http.Server, ln net.Listener) error {
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("health server: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("health server shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("SERVING\n"))
}

func readinessHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("SERVING\n"))
}
