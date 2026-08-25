package soak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Options struct {
	BaseURL          string
	Duration         time.Duration
	Interval         time.Duration
	RequestTimeout   time.Duration
	MaxReadyFailures int
}

type Report struct {
	StartedAt           time.Time     `json:"started_at"`
	FinishedAt          time.Time     `json:"finished_at"`
	RequestedDuration   time.Duration `json:"requested_duration_ns"`
	Samples             int           `json:"samples"`
	HealthFailures      int           `json:"health_failures"`
	ReadyFailures       int           `json:"ready_failures"`
	MetricsFailures     int           `json:"metrics_failures"`
	MaxConsecutiveReady int           `json:"max_consecutive_ready_failures"`
	Passed              bool          `json:"passed"`
	FailureReason       string        `json:"failure_reason,omitempty"`
}

type Runner struct{ Client *http.Client }

func (r Runner) Run(ctx context.Context, o Options) (Report, error) {
	if strings.TrimSpace(o.BaseURL) == "" {
		return Report{}, errors.New("base URL required")
	}
	if o.Duration <= 0 || o.Interval <= 0 {
		return Report{}, errors.New("duration and interval must be positive")
	}
	if o.RequestTimeout <= 0 {
		o.RequestTimeout = 5 * time.Second
	}
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: o.RequestTimeout}
	}
	rep := Report{StartedAt: time.Now().UTC(), RequestedDuration: o.Duration, Passed: true}
	deadline := time.NewTimer(o.Duration)
	defer deadline.Stop()
	tick := time.NewTicker(o.Interval)
	defer tick.Stop()
	consecutive := 0
	probe := func() {
		rep.Samples++
		if !getOK(ctx, client, o.BaseURL+"/healthz") {
			rep.HealthFailures++
		}
		if !getOK(ctx, client, o.BaseURL+"/readyz") {
			rep.ReadyFailures++
			consecutive++
			if consecutive > rep.MaxConsecutiveReady {
				rep.MaxConsecutiveReady = consecutive
			}
		} else {
			consecutive = 0
		}
		if !getOK(ctx, client, o.BaseURL+"/metrics") {
			rep.MetricsFailures++
		}
	}
	probe()
	for {
		select {
		case <-ctx.Done():
			rep.FinishedAt = time.Now().UTC()
			return rep, ctx.Err()
		case <-deadline.C:
			rep.FinishedAt = time.Now().UTC()
			if rep.HealthFailures > 0 {
				rep.Passed = false
				rep.FailureReason = fmt.Sprintf("health failures=%d", rep.HealthFailures)
			}
			if rep.MetricsFailures > 0 {
				rep.Passed = false
				rep.FailureReason = fmt.Sprintf("metrics failures=%d", rep.MetricsFailures)
			}
			if rep.ReadyFailures > o.MaxReadyFailures {
				rep.Passed = false
				rep.FailureReason = fmt.Sprintf("ready failures=%d > %d", rep.ReadyFailures, o.MaxReadyFailures)
			}
			return rep, nil
		case <-tick.C:
			probe()
		}
	}
}

func getOK(ctx context.Context, c *http.Client, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func WriteJSON(w io.Writer, r Report) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(r)
}
