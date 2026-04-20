# alphainfo-go

[![Go Reference](https://pkg.go.dev/badge/github.com/qgidev/alphainfo-go.svg)](https://pkg.go.dev/github.com/qgidev/alphainfo-go)
[![Go 1.21+](https://img.shields.io/badge/go-1.21+-blue.svg)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**Go client for the [alphainfo.io](https://alphainfo.io) Structural Intelligence API.**

Detect structural regime changes in any time series — biomedical signals, financial markets, energy grids, seismic data, IoT sensors, network traffic. One API, no training, no per-domain tuning.

## Install

```bash
go get github.com/qgidev/alphainfo-go
```

## 30-second try

**Step 1 — [get a free API key](https://alphainfo.io/register)** (50 analyses/month).

**Step 2**:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "math"
    "os"

    "github.com/qgidev/alphainfo-go"
)

func main() {
    client, err := alphainfo.NewClient(os.Getenv("ALPHAINFO_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }

    signal := make([]float64, 0, 400)
    for i := 0; i < 200; i++ {
        signal = append(signal, math.Sin(float64(i)/10))
    }
    for i := 0; i < 200; i++ {
        signal = append(signal, math.Sin(float64(i)/10)*3)
    }

    result, err := client.Analyze(context.Background(), alphainfo.AnalyzeRequest{
        Signal:       signal,
        SamplingRate: 100,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("band=%s score=%.3f\n", result.ConfidenceBand, result.StructuralScore)
}
```

## Structural fingerprint

```go
fp, err := client.Fingerprint(ctx, alphainfo.AnalyzeRequest{
    Signal:       signal,
    SamplingRate: 250,
})
if err != nil {
    return err
}
if fp.IsComplete() {
    vector := fp.Vector() // []float64 of length 5; nil when incomplete
    _ = vector            // index in pgvector / Qdrant / Faiss
} else {
    fmt.Printf("unavailable: %s\n", *fp.FingerprintReason)
}
```

**Minimum signal length:**

| Case | Minimum samples | Constant |
|---|---|---|
| No baseline | 192 | `alphainfo.MinFingerprintSamples` |
| With baseline | 50 | `alphainfo.MinFingerprintSamplesWithBaseline` |

Below the threshold, `FingerprintAvailable` is `false`, `Vector()` returns `nil`, and the SDK logs a `log.Printf` warning at call time.

See [`examples/fingerprint/main.go`](examples/fingerprint/main.go) for the full pattern with semantic-layer fallback.

## Error handling

Use `errors.Is` against the sentinel errors:

```go
import (
    "errors"
    "github.com/qgidev/alphainfo-go"
)

result, err := client.Analyze(ctx, req)
switch {
case errors.Is(err, alphainfo.ErrAuth):
    // Invalid API key — get one at https://alphainfo.io/register
case errors.Is(err, alphainfo.ErrRateLimit):
    var rl *alphainfo.RateLimitError
    if errors.As(err, &rl) {
        time.Sleep(time.Duration(rl.RetryAfter) * time.Second)
    }
case errors.Is(err, alphainfo.ErrValidation):
    // Bad input — fix and retry
case errors.Is(err, alphainfo.ErrNotFound):
    // analysis_id not found
case errors.Is(err, alphainfo.ErrAPI):
    // 5xx
case errors.Is(err, alphainfo.ErrNetwork):
    // DNS/TCP/TLS/timeout/cancel
}
```

| Sentinel | HTTP | Semantics |
|---|---|---|
| `ErrAuth` | 401 | Invalid or missing API key |
| `ErrValidation` | 400, 413, 422 | Bad input |
| `ErrRateLimit` | 429 | Use `errors.As` for `*RateLimitError.RetryAfter` |
| `ErrNotFound` | 404 | `audit_replay` with unknown id |
| `ErrAPI` | 5xx | Server error |
| `ErrNetwork` | — | Transport / ctx.Canceled |

## Zero-auth exploration

```go
g, err := alphainfo.Guide(ctx)
h, err := alphainfo.Health(ctx)
```

## Configuration

```go
client, _ := alphainfo.NewClient(
    "ai_...",
    alphainfo.WithBaseURL("https://www.alphainfo.io"),
    alphainfo.WithHTTPClient(&http.Client{Timeout: 60 * time.Second}),
)
```

All operations take `context.Context` — use it for timeouts and cancellation.

## Links

- [Web](https://alphainfo.io)
- [Dashboard](https://alphainfo.io/dashboard)
- [Python SDK](https://pypi.org/project/alphainfo/)
- [JS/TS SDK](https://www.npmjs.com/package/alphainfo)
- [Encoding guide](https://www.alphainfo.io/v1/guide)

## About

Built by **QGI Quantum Systems LTDA** — São Paulo, Brazil.
Contact: contato@alphainfo.io · api@alphainfo.io

## License

MIT
