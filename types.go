// Package alphainfo is the Go client for the alphainfo.io Structural
// Intelligence API — detect structural regime changes in any time series.
//
// Quick start:
//
//	client, _ := alphainfo.NewClient("ai_...")
//	result, err := client.Analyze(ctx, alphainfo.AnalyzeRequest{
//	    Signal:       signal,
//	    SamplingRate: 100,
//	})
//
// Get a free key at https://alphainfo.io/register.
package alphainfo

// MinFingerprintSamples is the minimum signal length required for a full
// 5-dimensional fingerprint when no baseline is provided. Below this
// threshold the server returns FingerprintAvailable=false with
// FingerprintReason="signal_too_short".
const MinFingerprintSamples = 192

// MinFingerprintSamplesWithBaseline is the minimum length when an explicit
// baseline of comparable size is provided.
const MinFingerprintSamplesWithBaseline = 50

// ConfidenceBand classifies the overall structural similarity.
type ConfidenceBand string

const (
	BandStable     ConfidenceBand = "stable"
	BandTransition ConfidenceBand = "transition"
	BandUnstable   ConfidenceBand = "unstable"
)

// AlertLevel is the semantic layer alert classification.
type AlertLevel string

const (
	AlertNormal    AlertLevel = "normal"
	AlertAttention AlertLevel = "attention"
	AlertAlert     AlertLevel = "alert"
	AlertCritical  AlertLevel = "critical"
)

// SemanticResult is the human-readable interpretation layer.
type SemanticResult struct {
	Summary           string                 `json:"summary"`
	AlertLevel        AlertLevel             `json:"alert_level"`
	RecommendedAction *string                `json:"recommended_action,omitempty"`
	Trend             *string                `json:"trend,omitempty"`
	Severity          *string                `json:"severity,omitempty"`
	SeverityScore     *float64               `json:"severity_score,omitempty"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

// AnalysisResult is the full response from Analyze.
type AnalysisResult struct {
	StructuralScore float64                `json:"structural_score"`
	ChangeDetected  bool                   `json:"change_detected"`
	ChangeScore     float64                `json:"change_score"`
	ConfidenceBand  ConfidenceBand         `json:"confidence_band"`
	EngineVersion   string                 `json:"engine_version"`
	AnalysisID      string                 `json:"analysis_id"`
	Metrics         map[string]interface{} `json:"metrics,omitempty"`
	Provenance      map[string]interface{} `json:"provenance,omitempty"`
	Semantic        *SemanticResult        `json:"semantic,omitempty"`
	Warning         *string                `json:"warning,omitempty"`
}

// FingerprintResult is the 5-dimensional structural fingerprint.
//
// Each Sim* field is a pointer so the contract can honestly say "not
// computed" (nil) vs "zero divergence" (non-nil 0.0). Always check
// FingerprintAvailable or call IsComplete() before using Vector().
type FingerprintResult struct {
	AnalysisID           string         `json:"analysis_id"`
	StructuralScore      float64        `json:"structural_score"`
	ConfidenceBand       ConfidenceBand `json:"confidence_band"`
	SimLocal             *float64       `json:"sim_local"`
	SimSpectral          *float64       `json:"sim_spectral"`
	SimFractal           *float64       `json:"sim_fractal"`
	SimTransition        *float64       `json:"sim_transition"`
	SimTrend             *float64       `json:"sim_trend"`
	FingerprintAvailable bool           `json:"fingerprint_available"`
	// FingerprintReason is "signal_too_short", "structural_degenerate",
	// "internal_error", or nil when the fingerprint is available.
	FingerprintReason *string `json:"fingerprint_reason"`
}

// IsComplete reports whether the five Sim* dimensions are all populated.
// Convenience wrapper around FingerprintAvailable.
func (f *FingerprintResult) IsComplete() bool {
	return f.FingerprintAvailable
}

// Vector returns the 5D fingerprint as a slice, or nil when the
// fingerprint is not available. Callers indexing vectors for ANN must
// skip on nil rather than substituting zeros.
func (f *FingerprintResult) Vector() []float64 {
	if !f.FingerprintAvailable {
		return nil
	}
	if f.SimLocal == nil || f.SimSpectral == nil || f.SimFractal == nil ||
		f.SimTransition == nil || f.SimTrend == nil {
		return nil
	}
	return []float64{
		*f.SimLocal,
		*f.SimSpectral,
		*f.SimFractal,
		*f.SimTransition,
		*f.SimTrend,
	}
}

// BatchItemResult is one result inside a BatchResult.
type BatchItemResult struct {
	Index           int                    `json:"index"`
	StructuralScore *float64               `json:"structural_score,omitempty"`
	ChangeDetected  *bool                  `json:"change_detected,omitempty"`
	ChangeScore     *float64               `json:"change_score,omitempty"`
	ConfidenceBand  *ConfidenceBand        `json:"confidence_band,omitempty"`
	EngineVersion   *string                `json:"engine_version,omitempty"`
	AnalysisID      *string                `json:"analysis_id,omitempty"`
	Metrics         map[string]interface{} `json:"metrics,omitempty"`
	Semantic        *SemanticResult        `json:"semantic,omitempty"`
	Error           *string                `json:"error,omitempty"`
}

// BatchResult is the response from AnalyzeBatch.
type BatchResult struct {
	Results          []BatchItemResult `json:"results"`
	AnalysesConsumed int               `json:"analyses_consumed"`
	TotalSignals     int               `json:"total_signals"`
}

// ChannelResult is one channel of a VectorResult.
type ChannelResult struct {
	Name            string          `json:"-"`
	StructuralScore *float64        `json:"structural_score,omitempty"`
	ChangeDetected  *bool           `json:"change_detected,omitempty"`
	ChangeScore     *float64        `json:"change_score,omitempty"`
	ConfidenceBand  *ConfidenceBand `json:"confidence_band,omitempty"`
	EngineVersion   *string         `json:"engine_version,omitempty"`
	Error           *string         `json:"error,omitempty"`
}

// VectorResult aggregates per-channel results for multi-channel analysis.
type VectorResult struct {
	StructuralScore float64                  `json:"structural_score"`
	ChangeScore     float64                  `json:"change_score"`
	ChangeDetected  bool                     `json:"change_detected"`
	ConfidenceBand  ConfidenceBand           `json:"confidence_band"`
	AnalysisID      string                   `json:"analysis_id"`
	EngineVersion   string                   `json:"engine_version"`
	Channels        map[string]ChannelResult `json:"channels"`
	Warning         *string                  `json:"warning,omitempty"`
}

// MatrixResult is the pairwise similarity matrix from AnalyzeMatrix.
type MatrixResult struct {
	Matrix           [][]float64 `json:"matrix"`
	Labels           []string    `json:"labels"`
	NSignals         int         `json:"n_signals"`
	NPairs           int         `json:"n_pairs"`
	AnalysesConsumed int         `json:"analyses_consumed"`
}

// HealthStatus is the response from /health.
type HealthStatus struct {
	Status        string            `json:"status"`
	Version       string            `json:"version"`
	Message       string            `json:"message"`
	UptimeSeconds *float64          `json:"uptime_seconds,omitempty"`
	Services      map[string]string `json:"services,omitempty"`
}

// PlanInfo is a billing plan entry.
type PlanInfo struct {
	ID           interface{}            `json:"id,omitempty"`
	Slug         string                 `json:"slug"`
	Name         string                 `json:"name"`
	PriceCents   *int                   `json:"price_cents,omitempty"`
	MonthlyLimit *int                   `json:"monthly_limit,omitempty"`
	Features     map[string]interface{} `json:"features,omitempty"`
}

// RateLimitInfo is parsed from X-RateLimit-* response headers.
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     int
}

// AuditSummary is one entry from AuditList.
type AuditSummary struct {
	AnalysisID      string   `json:"analysis_id"`
	Timestamp       string   `json:"timestamp"`
	SignalLength    int      `json:"signal_length,omitempty"`
	Domain          *string  `json:"domain,omitempty"`
	StructuralScore *float64 `json:"structural_score,omitempty"`
	ChangeDetected  *bool    `json:"change_detected,omitempty"`
}

// AuditReplay is the response from AuditReplay.
type AuditReplay struct {
	AnalysisID   string                 `json:"analysis_id"`
	Timestamp    string                 `json:"timestamp"`
	SignalLength int                    `json:"signal_length"`
	SamplingRate float64                `json:"sampling_rate"`
	Domain       *string                `json:"domain,omitempty"`
	InputHash    *string                `json:"input_hash,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Output       map[string]interface{} `json:"output"`
}

// AnalyzeRequest is the input for Analyze and similar methods.
type AnalyzeRequest struct {
	Signal          []float64              `json:"signal"`
	SamplingRate    float64                `json:"sampling_rate"`
	Domain          string                 `json:"domain,omitempty"`
	Baseline        []float64              `json:"baseline,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	IncludeSemantic *bool                  `json:"include_semantic,omitempty"`
	UseMultiscale   *bool                  `json:"use_multiscale,omitempty"`
}

// BatchRequest is the input for AnalyzeBatch.
type BatchRequest struct {
	Signals         [][]float64 `json:"signals"`
	SamplingRate    float64     `json:"sampling_rate"`
	Domain          string      `json:"domain,omitempty"`
	Baselines       [][]float64 `json:"baselines,omitempty"`
	IncludeSemantic *bool       `json:"include_semantic,omitempty"`
	UseMultiscale   *bool       `json:"use_multiscale,omitempty"`
}

// MatrixRequest is the input for AnalyzeMatrix.
type MatrixRequest struct {
	Signals       [][]float64 `json:"signals"`
	SamplingRate  float64     `json:"sampling_rate"`
	Domain        string      `json:"domain,omitempty"`
	UseMultiscale *bool       `json:"use_multiscale,omitempty"`
}

// VectorRequest is the input for AnalyzeVector.
type VectorRequest struct {
	Channels        map[string][]float64 `json:"channels"`
	SamplingRate    float64              `json:"sampling_rate"`
	Domain          string               `json:"domain,omitempty"`
	Baselines       map[string][]float64 `json:"baselines,omitempty"`
	IncludeSemantic *bool                `json:"include_semantic,omitempty"`
	UseMultiscale   *bool                `json:"use_multiscale,omitempty"`
}
