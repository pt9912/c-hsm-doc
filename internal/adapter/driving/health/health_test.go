package health

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMuxLivenessAndReadiness(t *testing.T) {
	mux := NewMux()
	cases := []string{"/healthz", "/readyz"}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			rec := &recorder{header: make(http.Header)}
			mux.ServeHTTP(rec, req)
			if rec.code != http.StatusOK {
				t.Errorf("%s status = %d, want 200", path, rec.code)
			}
			if !strings.Contains(rec.body.String(), "SERVING") {
				t.Errorf("%s body = %q, want substring SERVING", path, rec.body.String())
			}
		})
	}
}

func TestRunStartsAndShutsDownOnContextCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := NewServer("")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, srv, ln)
	}()

	// Server polling: warte, bis er antwortet.
	url := "http://" + ln.Addr().String() + "/healthz"
	if err := waitForOK(url, 2*time.Second); err != nil {
		cancel()
		<-done
		t.Fatalf("waitForOK: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancel within 5s")
	}
}

func waitForOK(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return context.DeadlineExceeded
}

// recorder ist ein minimal-http.ResponseWriter ohne externe Abhängigkeit
// auf net/http/httptest.
type recorder struct {
	header http.Header
	code   int
	body   strings.Builder
}

func (r *recorder) Header() http.Header { return r.header }
func (r *recorder) WriteHeader(code int) {
	if r.code == 0 {
		r.code = code
	}
}
func (r *recorder) Write(p []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.body.Write(p)
}
