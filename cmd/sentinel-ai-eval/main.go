package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/ai/eval"
	"github.com/Homiakus/Home_Sentinel/internal/ai/ollama"
)

func main() {
	var dataset, rawURL, model string
	var timeout time.Duration
	flag.StringVar(&dataset, "dataset", "", "path to evaluation manifest JSON")
	flag.StringVar(&rawURL, "url", "http://127.0.0.1:11434", "Ollama base URL")
	flag.StringVar(&model, "model", "", "Ollama vision model")
	flag.DurationVar(&timeout, "timeout", 10*time.Minute, "maximum evaluation duration")
	flag.Parse()
	if dataset == "" || model == "" {
		fmt.Fprintln(os.Stderr, "usage: sentinel-ai-eval -dataset manifest.json -model MODEL [-url URL]")
		os.Exit(2)
	}
	manifest, base, err := eval.LoadManifest(dataset)
	if err != nil {
		fail(err)
	}
	provider, err := ollama.New(ollama.Options{BaseURL: rawURL, Model: model})
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	report := eval.Run(ctx, provider, manifest, base)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fail(err)
	}
	if report.Passed != report.Total {
		os.Exit(1)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, "sentinel-ai-eval:", err); os.Exit(1) }
