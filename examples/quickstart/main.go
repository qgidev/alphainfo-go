// alphainfo — Hello World (Go)
//
// Build & run:
//
//	export ALPHAINFO_API_KEY=ai_...
//	go run github.com/qgidev/alphainfo-go/examples/quickstart@latest
//
// Get a free key at https://alphainfo.io/register (50 analyses/month).
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

	// Toy signal: sine that abruptly changes amplitude at the midpoint.
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

	fmt.Printf("structural_score: %.3f\n", result.StructuralScore)
	fmt.Printf("confidence_band:  %s\n", result.ConfidenceBand)
	fmt.Printf("change_detected:  %v\n", result.ChangeDetected)
	fmt.Printf("analysis_id:      %s\n", result.AnalysisID)
}
