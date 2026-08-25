package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/soak"
)

func main() {
	base := flag.String("base-url", "http://127.0.0.1:8080", "Sentinel base URL")
	duration := flag.Duration("duration", 72*time.Hour, "soak duration")
	interval := flag.Duration("interval", 30*time.Second, "probe interval")
	out := flag.String("out", "soak-report.json", "report file")
	maxReady := flag.Int("max-ready-failures", 0, "allowed readiness probe failures")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	r, err := (soak.Runner{}).Run(ctx, soak.Options{BaseURL: *base, Duration: *duration, Interval: *interval, RequestTimeout: 5 * time.Second, MaxReadyFailures: *maxReady})
	f, fe := os.Create(*out)
	if fe != nil {
		fatal(fe)
	}
	if e := soak.WriteJSON(f, r); e != nil {
		_ = f.Close()
		fatal(e)
	}
	_ = f.Close()
	if err != nil {
		fatal(err)
	}
	if !r.Passed {
		fmt.Fprintln(os.Stderr, "soak failed:", r.FailureReason)
		os.Exit(2)
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, "sentinel-soak:", e); os.Exit(1) }
