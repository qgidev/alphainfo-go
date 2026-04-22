# Changelog

All notable changes to alphainfo-go.

## 1.5.14 — Version parity bump

No code changes in this SDK. Bumped only to keep the version number in
sync with the Python SDK (which shipped 1.5.14 to fix a stale
`__version__` string that the other SDKs never had). All functional
behaviour is identical to 1.5.13.

## 1.5.13 — Response contract refinement and documentation improvements

Server response shape has been neutralised — the following keys have
new names:
  • metrics.scale_entropy                            → metrics.complexity_index
  • metrics.multiscale.curvature                     → metrics.multiscale.scale_profile
  • metrics.multiscale.summary.scale_curvature_score → metrics.multiscale.summary.profile_score

The 5D fingerprint contract (sim_local/sim_spectral/sim_fractal/
sim_transition/sim_trend + fingerprint_available + fingerprint_reason)
is unchanged.

## [1.5.12] - 2026-04-20

Added automatic domain inference; `Domain` field now optional with
sensible default.

- New `DomainInference` struct exported from `alphainfo`.
- `AnalysisResult.DomainApplied string` — populated by server 1.5.12+.
- `AnalysisResult.DomainInference *DomainInference` — populated only
  when the caller passed `Domain="auto"`.
- New `client.AnalyzeAuto(ctx, req)` — sugar for `Analyze` with
  `req.Domain = "auto"`.
- Godoc on `Analyze` explains "auto", aliases, and the "Did you mean …?"
  suggestion path.

Backwards-compatible: existing callers unaffected.

## [1.5.11] - 2026-04-20

### Connection cleanup improvements.

- `Client.Close()` added. Drains the underlying `http.Transport`'s
  idle keep-alive connection pool so short-lived scripts, CLIs and
  tests can exit without leaving sockets in TIME_WAIT.
- Idempotent; safe to call from `defer`. Long-lived server processes
  can ignore it — Go's default pool management is still optimal for
  those.
- No API surface break.

## [1.5.10] - 2026-04-20

### Initial release — parity with Python SDK 1.5.10.

- `Client` with `Analyze`, `Fingerprint`, `AnalyzeBatch`, `AnalyzeMatrix`,
  `AnalyzeVector`, `AuditList`, `AuditReplay`, `Health`, `Plans`,
  `Guide`. Every method takes `context.Context` for timeouts/cancel.
- Module-level `Guide` and `Health` helpers (no API key).
- Public constants `MinFingerprintSamples` (192) and
  `MinFingerprintSamplesWithBaseline` (50).
- Honest fingerprint contract: `Sim*` fields are `*float64`,
  `Vector()` returns `nil` when incomplete — never substitutes zeros.
- Typed errors via `errors.Is`/`errors.As` against `ErrAuth`,
  `ErrRateLimit`, `ErrValidation`, `ErrNotFound`, `ErrAPI`,
  `ErrNetwork`. `*RateLimitError.RetryAfter` exposes the server's
  `Retry-After` header value.
- `log.Printf` warning when `Fingerprint` is called with a signal
  shorter than the threshold.
- Uses `net/http` stdlib only — no external dependencies.
