# Changelog

All notable changes to alphainfo-go.

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
