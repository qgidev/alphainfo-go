package alphainfo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConstantsMatchServer(t *testing.T) {
	if MinFingerprintSamples != 192 {
		t.Fatalf("MinFingerprintSamples: expected 192, got %d", MinFingerprintSamples)
	}
	if MinFingerprintSamplesWithBaseline != 50 {
		t.Fatalf("MinFingerprintSamplesWithBaseline: expected 50, got %d", MinFingerprintSamplesWithBaseline)
	}
}

func TestNewClientRequiresKey(t *testing.T) {
	_, err := NewClient("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "alphainfo.io/register") {
		t.Fatalf("error should point to register URL, got %q", err.Error())
	}
}

func TestFingerprintParsing_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"analysis_id": "abc",
			"structural_score": 0.9,
			"confidence_band": "stable",
			"change_detected": false,
			"change_score": 0.1,
			"engine_version": "t",
			"metrics": {
				"sim_local": 0.9,
				"sim_spectral": 0.8,
				"sim_fractal": 0.85,
				"sim_transition": 0.95,
				"sim_trend": 0.88,
				"fingerprint_available": true,
				"fingerprint_reason": null
			}
		}`)
	}))
	defer srv.Close()

	c, _ := NewClient("ai_test", WithBaseURL(srv.URL))
	fp, err := c.Fingerprint(context.Background(), AnalyzeRequest{
		Signal:       make([]float64, MinFingerprintSamples),
		SamplingRate: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fp.IsComplete() {
		t.Fatal("expected complete fingerprint")
	}
	v := fp.Vector()
	if len(v) != 5 {
		t.Fatalf("expected 5D vector, got %d", len(v))
	}
	if v[0] != 0.9 {
		t.Fatalf("sim_local: expected 0.9, got %v", v[0])
	}
}

func TestFingerprintParsing_IncompleteHonorsNull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"analysis_id": "abc",
			"structural_score": 0.5,
			"confidence_band": "transition",
			"change_detected": false,
			"change_score": 0.5,
			"engine_version": "t",
			"metrics": {
				"sim_local": null,
				"sim_spectral": null,
				"sim_fractal": null,
				"sim_transition": null,
				"sim_trend": null,
				"fingerprint_available": false,
				"fingerprint_reason": "signal_too_short"
			}
		}`)
	}))
	defer srv.Close()

	c, _ := NewClient("ai_test", WithBaseURL(srv.URL))
	// Short signal — should also fire the warn (logged, not asserted here).
	fp, err := c.Fingerprint(context.Background(), AnalyzeRequest{
		Signal:       make([]float64, 20),
		SamplingRate: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fp.IsComplete() {
		t.Fatal("expected incomplete fingerprint")
	}
	if fp.Vector() != nil {
		t.Fatalf("vector must be nil when incomplete — got %v", fp.Vector())
	}
	if fp.SimLocal != nil {
		t.Fatal("sim_local must be nil (not 0.0) when server returns null")
	}
	if fp.FingerprintReason == nil || *fp.FingerprintReason != "signal_too_short" {
		t.Fatalf("expected reason signal_too_short, got %v", fp.FingerprintReason)
	}
}

func TestAuthErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"detail": "Invalid API key"}`)
	}))
	defer srv.Close()

	c, _ := NewClient("ai_bad", WithBaseURL(srv.URL))
	_, err := c.Analyze(context.Background(), AnalyzeRequest{
		Signal: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, SamplingRate: 1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}

func TestRateLimitMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		_, _ = io.WriteString(w, `{"detail": "Rate limit exceeded"}`)
	}))
	defer srv.Close()

	c, _ := NewClient("ai_test", WithBaseURL(srv.URL))
	_, err := c.Analyze(context.Background(), AnalyzeRequest{
		Signal: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, SamplingRate: 1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrRateLimit) {
		t.Fatalf("expected ErrRateLimit, got %v", err)
	}
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatal("expected *RateLimitError via errors.As")
	}
	if rl.RetryAfter != 42 {
		t.Fatalf("retryAfter: expected 42, got %d", rl.RetryAfter)
	}
}

func TestRateLimitHeadersCaptured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", "73")
		w.Header().Set("X-RateLimit-Reset", "1234567890")
		_, _ = io.WriteString(w, `{
			"analysis_id": "a",
			"structural_score": 0.9,
			"change_detected": false,
			"change_score": 0.1,
			"confidence_band": "stable",
			"engine_version": "t"
		}`)
	}))
	defer srv.Close()

	c, _ := NewClient("ai_test", WithBaseURL(srv.URL))
	if c.RateLimitInfo().Limit != 0 {
		t.Fatal("RateLimitInfo should start zeroed")
	}
	_, err := c.Analyze(context.Background(), AnalyzeRequest{
		Signal: make([]float64, 200), SamplingRate: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	rl := c.RateLimitInfo()
	if rl.Limit != 100 || rl.Remaining != 73 || rl.Reset != 1234567890 {
		t.Fatalf("unexpected rate limit info: %+v", rl)
	}
}

func TestContextCancellation(t *testing.T) {
	// Server that takes a long time to respond — client must cut it off.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(5 * time.Second):
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	defer srv.Close()

	c, _ := NewClient("ai_test", WithBaseURL(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Analyze(ctx, AnalyzeRequest{
		Signal: make([]float64, 200), SamplingRate: 1,
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error on cancel")
	}
	if !errors.Is(err, ErrNetwork) {
		t.Fatalf("expected ErrNetwork on cancel, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("context cancellation took too long: %v — cancel is not being propagated", elapsed)
	}
}

// ── Bloco 1.2 — Close() cleanup ──────────────────────────────────────────

func TestCloseIsIdempotent(t *testing.T) {
	c, err := NewClient("ai_test_fake")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestCloseOnNilClient(t *testing.T) {
	var c *Client
	if err := c.Close(); err != nil {
		t.Fatalf("Close on nil: %v", err)
	}
}

func TestCloseWithCustomTransport(t *testing.T) {
	// Provide a custom http.Client so we exercise the CloseIdleConnections
	// path on the underlying transport.
	custom := &http.Client{Transport: &http.Transport{}, Timeout: time.Second}
	c, err := NewClient("ai_test_fake", WithHTTPClient(custom))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
