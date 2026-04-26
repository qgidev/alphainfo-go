package alphainfo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	sdkVersion     = "1.5.26"
	defaultBaseURL = "https://www.alphainfo.io"
	defaultTimeout = 30 * time.Second
	analyzeTimeout = 120 * time.Second
)

// Client talks to the alphainfo.io Structural Intelligence API.
//
// Create one per process and reuse it — the underlying http.Client has
// its own connection pool.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// rateLimit is updated on every response that carries X-RateLimit-*.
	// Zero value (Limit == 0) means "not populated yet".
	rateLimit RateLimitInfo
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the default https://www.alphainfo.io.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient swaps the underlying http.Client (e.g. to tune transport).
// The provided client must carry its own timeout if you want one; the
// per-request context still bounds each call.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// NewClient constructs a Client with the given API key.
// Returns ErrValidation if apiKey is empty.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, &Error{
			Message: "apiKey is required. Get one at https://alphainfo.io/register (format: 'ai_...')",
			Kind:    ErrValidation,
		}
	}
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// RateLimitInfo returns the last observed X-RateLimit-* values, or the
// zero value (Limit == 0) if nothing has been seen yet.
func (c *Client) RateLimitInfo() RateLimitInfo {
	return c.rateLimit
}

// Close releases idle HTTP connections held by the client's transport.
//
// Go's net/http keeps a pool of keep-alive TCP/TLS connections on the
// transport so subsequent requests reuse them. For long-lived processes
// this is the right default; for short-lived scripts, CLIs, or tests
// that exit quickly, calling Close() on shutdown drains the pool so the
// OS doesn't see dangling sockets in TIME_WAIT longer than necessary.
//
// Close is safe to call more than once. It is a no-op if the underlying
// transport does not implement http.Transport's CloseIdleConnections.
//
// Idiomatic usage:
//
//	c, err := alphainfo.NewClient(apiKey)
//	if err != nil { return err }
//	defer c.Close()
func (c *Client) Close() error {
	if c == nil || c.httpClient == nil {
		return nil
	}
	if t, ok := c.httpClient.Transport.(interface{ CloseIdleConnections() }); ok {
		t.CloseIdleConnections()
		return nil
	}
	// net/http.DefaultTransport satisfies the interface above via the
	// embedded *http.Transport; when the caller injected a custom
	// RoundTripper that doesn't, we fall back to the top-level helper.
	c.httpClient.CloseIdleConnections()
	return nil
}

// ---------------------------------------------------------------------------
// Request plumbing
// ---------------------------------------------------------------------------

func (c *Client) doRequest(
	ctx context.Context, method, path string, body interface{}, timeout time.Duration,
) ([]byte, error) {
	fullURL := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, &Error{Message: "failed to marshal request: " + err.Error(), Kind: ErrValidation}
		}
		reqBody = bytes.NewReader(buf)
	}

	// Bound this single request with a timeout derived from the caller's ctx.
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, fullURL, reqBody)
	if err != nil {
		return nil, &Error{Message: "failed to build request: " + err.Error(), Kind: ErrNetwork}
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "alphainfo-go/"+sdkVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &Error{
			Message: fmt.Sprintf("network error: %v", err),
			Kind:    ErrNetwork,
		}
	}
	defer resp.Body.Close()

	c.captureRateLimit(resp)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Message: "failed to read response: " + err.Error(), Kind: ErrNetwork}
	}

	if resp.StatusCode >= 400 {
		return nil, errorFromResponse(resp.StatusCode, resp.Header, data)
	}
	return data, nil
}

func (c *Client) captureRateLimit(resp *http.Response) {
	limit, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))
	if limit <= 0 {
		return
	}
	remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	reset, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Reset"))
	c.rateLimit = RateLimitInfo{Limit: limit, Remaining: remaining, Reset: reset}
}

func errorFromResponse(status int, headers http.Header, body []byte) error {
	var parsed map[string]interface{}
	_ = json.Unmarshal(body, &parsed)

	detail := ""
	if v, ok := parsed["detail"]; ok {
		switch t := v.(type) {
		case string:
			detail = t
		case map[string]interface{}:
			if m, ok := t["message"].(string); ok {
				detail = m
			}
		}
	}

	base := &Error{
		Message:      detail,
		StatusCode:   status,
		ResponseData: parsed,
	}

	switch {
	case status == 401:
		base.Kind = ErrAuth
		if base.Message == "" {
			base.Message = "Invalid or missing API key. Get a free key at https://alphainfo.io/register and pass it to NewClient."
		}
		return base
	case status == 429:
		base.Kind = ErrRateLimit
		retryAfter, _ := strconv.Atoi(headers.Get("Retry-After"))
		if base.Message == "" {
			base.Message = "rate limit exceeded"
		}
		return &RateLimitError{Base: base, RetryAfter: retryAfter}
	case status == 400 || status == 413 || status == 422:
		base.Kind = ErrValidation
		if base.Message == "" {
			base.Message = "validation failed"
		}
		return base
	case status == 404:
		base.Kind = ErrNotFound
		if base.Message == "" {
			base.Message = "not found"
		}
		return base
	case status >= 500:
		base.Kind = ErrAPI
		if base.Message == "" {
			base.Message = "server error"
		}
		return base
	default:
		base.Kind = ErrAPI
		if base.Message == "" {
			base.Message = fmt.Sprintf("HTTP %d", status)
		}
		return base
	}
}

// ---------------------------------------------------------------------------
// Module-level helpers — no API key required
// ---------------------------------------------------------------------------

// Guide fetches the public /v1/guide. No API key needed; useful for
// exploring the API shape before signing up.
func Guide(ctx context.Context, baseURL ...string) (map[string]interface{}, error) {
	base := defaultBaseURL
	if len(baseURL) > 0 && baseURL[0] != "" {
		base = strings.TrimRight(baseURL[0], "/")
	}
	return fetchJSON(ctx, base+"/v1/guide")
}

// Health fetches the public /health. No API key needed.
func Health(ctx context.Context, baseURL ...string) (*HealthStatus, error) {
	base := defaultBaseURL
	if len(baseURL) > 0 && baseURL[0] != "" {
		base = strings.TrimRight(baseURL[0], "/")
	}
	data, err := fetchJSON(ctx, base+"/health")
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(data)
	var out HealthStatus
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &Error{Message: "failed to parse /health: " + err.Error(), Kind: ErrAPI}
	}
	return &out, nil
}

func fetchJSON(ctx context.Context, fullURL string) (map[string]interface{}, error) {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, &Error{Message: "build request: " + err.Error(), Kind: ErrNetwork}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "alphainfo-go/"+sdkVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &Error{Message: "network error: " + err.Error(), Kind: ErrNetwork}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Message: err.Error(), Kind: ErrNetwork}
	}
	if resp.StatusCode >= 400 {
		return nil, errorFromResponse(resp.StatusCode, resp.Header, data)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &Error{Message: "parse: " + err.Error(), Kind: ErrAPI}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Endpoints
// ---------------------------------------------------------------------------

// Analyze runs a full structural analysis on a single signal.
//
// If Domain is empty, "generic" is used. Pass "auto" to have the server
// infer the calibration from the signal — read AnalysisResult.DomainApplied
// to see what it chose and AnalysisResult.DomainInference for the reasoning.
// Specific domains ("biomedical", "finance", "seismic", …) apply their
// calibration directly; aliases ("fintech"→"finance", "biomed"→"biomedical",
// "grid"→"power_grid", …) resolve server-side; real typos receive an HTTP
// 400 with a "Did you mean …?" suggestion.
func (c *Client) Analyze(ctx context.Context, req AnalyzeRequest) (*AnalysisResult, error) {
	if req.Domain == "" {
		req.Domain = "generic"
	}
	data, err := c.doRequest(ctx, http.MethodPost, "/v1/analyze/stream", req, analyzeTimeout)
	if err != nil {
		return nil, err
	}
	var out AnalysisResult
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &Error{Message: "parse analysis: " + err.Error(), Kind: ErrAPI}
	}
	return &out, nil
}

// AnalyzeAuto is syntactic sugar for Analyze with Domain="auto". The server
// picks the calibration from cheap signal statistics; inspect
// AnalysisResult.DomainInference for confidence + reasoning.
func (c *Client) AnalyzeAuto(ctx context.Context, req AnalyzeRequest) (*AnalysisResult, error) {
	req.Domain = "auto"
	return c.Analyze(ctx, req)
}

// Fingerprint extracts the 5D structural fingerprint (fast path).
//
// Emits a log.Print warning when the signal is shorter than the minimum
// needed for a complete decomposition (MinFingerprintSamples, or
// MinFingerprintSamplesWithBaseline when a baseline is provided). The
// call still goes through — the warning just tells you the response
// will likely come back with FingerprintAvailable=false.
func (c *Client) Fingerprint(ctx context.Context, req AnalyzeRequest) (*FingerprintResult, error) {
	warnIfTooShortForFingerprint(len(req.Signal), len(req.Baseline))

	if req.Domain == "" {
		req.Domain = "generic"
	}
	f := false
	req.IncludeSemantic = &f
	req.UseMultiscale = &f

	data, err := c.doRequest(ctx, http.MethodPost, "/v1/analyze/stream", req, analyzeTimeout)
	if err != nil {
		return nil, err
	}
	return parseFingerprintFromStream(data)
}

type streamEnvelope struct {
	AnalysisID      string                 `json:"analysis_id"`
	StructuralScore float64                `json:"structural_score"`
	ConfidenceBand  ConfidenceBand         `json:"confidence_band"`
	Metrics         map[string]interface{} `json:"metrics"`
}

func parseFingerprintFromStream(data []byte) (*FingerprintResult, error) {
	var env streamEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, &Error{Message: "parse fingerprint: " + err.Error(), Kind: ErrAPI}
	}
	fp := &FingerprintResult{
		AnalysisID:      env.AnalysisID,
		StructuralScore: env.StructuralScore,
		ConfidenceBand:  env.ConfidenceBand,
	}
	if env.Metrics != nil {
		fp.SimLocal = optFloat(env.Metrics, "sim_local")
		fp.SimSpectral = optFloat(env.Metrics, "sim_spectral")
		fp.SimFractal = optFloat(env.Metrics, "sim_fractal")
		fp.SimTransition = optFloat(env.Metrics, "sim_transition")
		fp.SimTrend = optFloat(env.Metrics, "sim_trend")

		if v, ok := env.Metrics["fingerprint_available"]; ok {
			if b, ok := v.(bool); ok {
				fp.FingerprintAvailable = b
			}
		} else {
			// Older server — infer from presence of all five dimensions.
			fp.FingerprintAvailable = fp.SimLocal != nil && fp.SimSpectral != nil &&
				fp.SimFractal != nil && fp.SimTransition != nil && fp.SimTrend != nil
		}
		if v, ok := env.Metrics["fingerprint_reason"]; ok {
			if s, ok := v.(string); ok {
				fp.FingerprintReason = &s
			}
		} else if !fp.FingerprintAvailable {
			r := "internal_error"
			fp.FingerprintReason = &r
		}
	}
	return fp, nil
}

func optFloat(m map[string]interface{}, key string) *float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	f, ok := v.(float64)
	if !ok {
		return nil
	}
	return &f
}

func warnIfTooShortForFingerprint(signalLen, baselineLen int) {
	threshold := MinFingerprintSamples
	qualifier := "without baseline"
	if baselineLen > 0 {
		threshold = MinFingerprintSamplesWithBaseline
		qualifier = "with baseline"
	}
	if signalLen >= threshold {
		return
	}
	log.Printf(
		"[alphainfo] Signal has %d samples; the 5D fingerprint needs >=%d %s. "+
			"Response will likely come back with fingerprint_available=false "+
			"(reason=\"signal_too_short\"). Use Analyze() for shorter signals.",
		signalLen, threshold, qualifier,
	)
}

// AnalyzeBatch runs analysis on up to 100 signals in one request.
func (c *Client) AnalyzeBatch(ctx context.Context, req BatchRequest) (*BatchResult, error) {
	if req.Domain == "" {
		req.Domain = "generic"
	}
	data, err := c.doRequest(ctx, http.MethodPost, "/v1/analyze/batch", req, analyzeTimeout)
	if err != nil {
		return nil, err
	}
	var out BatchResult
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &Error{Message: "parse batch: " + err.Error(), Kind: ErrAPI}
	}
	return &out, nil
}

// AnalyzeMatrix computes the pairwise similarity matrix for N signals.
func (c *Client) AnalyzeMatrix(ctx context.Context, req MatrixRequest) (*MatrixResult, error) {
	if req.Domain == "" {
		req.Domain = "generic"
	}
	data, err := c.doRequest(ctx, http.MethodPost, "/v1/analyze/matrix", req, analyzeTimeout)
	if err != nil {
		return nil, err
	}
	var out MatrixResult
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &Error{Message: "parse matrix: " + err.Error(), Kind: ErrAPI}
	}
	return &out, nil
}

// AnalyzeVector runs multi-channel analysis.
func (c *Client) AnalyzeVector(ctx context.Context, req VectorRequest) (*VectorResult, error) {
	if req.Domain == "" {
		req.Domain = "generic"
	}
	data, err := c.doRequest(ctx, http.MethodPost, "/v1/analyze/vector", req, analyzeTimeout)
	if err != nil {
		return nil, err
	}
	var out VectorResult
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &Error{Message: "parse vector: " + err.Error(), Kind: ErrAPI}
	}
	// Populate Channel.Name from the map key.
	for name, ch := range out.Channels {
		ch.Name = name
		out.Channels[name] = ch
	}
	return &out, nil
}

// AuditReplay fetches a past analysis by its UUID.
func (c *Client) AuditReplay(ctx context.Context, analysisID string) (*AuditReplay, error) {
	if analysisID == "" {
		return nil, &Error{Message: "analysisID cannot be empty", Kind: ErrValidation}
	}
	data, err := c.doRequest(
		ctx, http.MethodGet,
		"/v1/audit/replay/"+url.PathEscape(analysisID), nil, defaultTimeout,
	)
	if err != nil {
		return nil, err
	}
	var out AuditReplay
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &Error{Message: "parse audit replay: " + err.Error(), Kind: ErrAPI}
	}
	return &out, nil
}

// AuditList returns recent analyses for the current API key.
func (c *Client) AuditList(ctx context.Context, limit int) ([]AuditSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	path := "/v1/audit/list?limit=" + strconv.Itoa(limit)
	data, err := c.doRequest(ctx, http.MethodGet, path, nil, defaultTimeout)
	if err != nil {
		return nil, err
	}
	// Server may return [ ... ] or { "analyses": [ ... ] } — accept both.
	var asList []AuditSummary
	if err := json.Unmarshal(data, &asList); err == nil && len(asList) >= 0 {
		return asList, nil
	}
	var wrapper struct {
		Analyses []AuditSummary `json:"analyses"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, &Error{Message: "parse audit list: " + err.Error(), Kind: ErrAPI}
	}
	return wrapper.Analyses, nil
}

// Health calls the /health endpoint with the client's base URL.
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	return Health(ctx, c.baseURL)
}

// Plans lists available billing plans.
func (c *Client) Plans(ctx context.Context) ([]PlanInfo, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/api/plans", nil, defaultTimeout)
	if err != nil {
		return nil, err
	}
	var out []PlanInfo
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &Error{Message: "parse plans: " + err.Error(), Kind: ErrAPI}
	}
	return out, nil
}

// Guide calls /v1/guide with the client's base URL.
func (c *Client) Guide(ctx context.Context) (map[string]interface{}, error) {
	return Guide(ctx, c.baseURL)
}

// ensure errors package imported (used in errors.go).
var _ = errors.New
