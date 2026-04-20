// alphainfo — Handling fingerprint availability (Go).
//
// Pattern:
//  1. Call Fingerprint
//  2. Check fp.IsComplete()
//  3. If true: use fp.Vector() for ANN / similarity search
//  4. If false: fall back to Analyze() + semantic layer
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
	apiKey := os.Getenv("ALPHAINFO_API_KEY")
	if apiKey == "" {
		log.Fatal("Set ALPHAINFO_API_KEY first: https://alphainfo.io/register")
	}
	client, err := alphainfo.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	run(client, "regime change, 400 pts", longSine())
	run(client, "short signal, 30 pts", shortSine(30))
	run(client, "constant signal, 100 pts", constant(100))
}

func run(c *alphainfo.Client, label string, signal []float64) {
	fmt.Printf("\n--- %s (n=%d) ---\n", label, len(signal))
	ctx := context.Background()
	fp, err := c.Fingerprint(ctx, alphainfo.AnalyzeRequest{
		Signal:       signal,
		SamplingRate: 100,
	})
	if err != nil {
		fmt.Printf("  ! %v\n", err)
		return
	}
	fmt.Printf("  score=%.3f band=%s\n", fp.StructuralScore, fp.ConfidenceBand)
	if fp.IsComplete() {
		fmt.Printf("  ✓ fingerprint available  vector=%v\n", fp.Vector())
		return
	}
	reason := "(unknown)"
	if fp.FingerprintReason != nil {
		reason = *fp.FingerprintReason
	}
	fmt.Printf("  ✗ fingerprint unavailable  reason=%s\n", reason)

	// Fallback: full analyze for the semantic layer.
	incSem := true
	result, err := c.Analyze(ctx, alphainfo.AnalyzeRequest{
		Signal: signal, SamplingRate: 100,
		IncludeSemantic: &incSem,
	})
	if err == nil && result.Semantic != nil {
		trend := "-"
		if result.Semantic.Trend != nil {
			trend = *result.Semantic.Trend
		}
		severity := "-"
		if result.Semantic.Severity != nil {
			severity = *result.Semantic.Severity
		}
		fmt.Printf("    semantic fallback → trend=%s, severity=%s\n", trend, severity)
	}
}

func longSine() []float64 {
	out := make([]float64, 0, 400)
	for i := 0; i < 200; i++ {
		out = append(out, math.Sin(float64(i)/10))
	}
	for i := 0; i < 200; i++ {
		out = append(out, math.Sin(float64(i)/10)*3)
	}
	return out
}

func shortSine(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.Sin(float64(i) / 5)
	}
	return out
}

func constant(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = 1.0
	}
	return out
}
