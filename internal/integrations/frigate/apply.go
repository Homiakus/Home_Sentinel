package frigate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Control interface {
	Version(context.Context) (string, error)
	Config(context.Context) (map[string]any, error)
	SaveConfig(context.Context, []byte, bool) error
	Restart(context.Context) error
	Go2RTCStreams(context.Context) (map[string]json.RawMessage, error)
}

type ApplyRequest struct {
	ConfigJSON      []byte
	SecretEnv       map[string]string
	ExpectedStreams []string
	ReadyTimeout    time.Duration
}
type ApplyResult struct {
	Applied    bool   `json:"applied"`
	RolledBack bool   `json:"rolled_back"`
	Checksum   string `json:"checksum"`
	Version    string `json:"version,omitempty"`
}
type Applier struct {
	Control      Control
	Secrets      SecretEnvSink
	PollInterval time.Duration
}

func (a Applier) Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
	var result ApplyResult
	if a.Control == nil {
		return result, errors.New("Frigate control unavailable")
	}
	if !json.Valid(req.ConfigJSON) {
		return result, errors.New("invalid generated Frigate JSON")
	}
	if len(req.SecretEnv) > 0 {
		if a.Secrets == nil {
			return result, errors.New("Frigate secret environment sink is required")
		}
		if err := a.Secrets.Materialize(req.SecretEnv); err != nil {
			return result, fmt.Errorf("materialize Frigate secrets: %w", err)
		}
	}
	previous, err := a.Control.Config(ctx)
	if err != nil {
		return result, fmt.Errorf("snapshot current Frigate config: %w", err)
	}
	previousJSON, err := json.Marshal(previous)
	if err != nil {
		return result, err
	}
	sum := sha256.Sum256(req.ConfigJSON)
	result.Checksum = hex.EncodeToString(sum[:])
	// Frigate validates /config/save before accepting the new configuration.
	if err := a.Control.SaveConfig(ctx, req.ConfigJSON, true); err != nil {
		return result, fmt.Errorf("Frigate rejected configuration: %w", err)
	}
	if err := a.Control.Restart(ctx); err != nil {
		return a.rollback(ctx, previousJSON, result, fmt.Errorf("restart Frigate: %w", err))
	}
	v, err := a.waitReady(ctx, req.ReadyTimeout, req.ExpectedStreams)
	if err != nil {
		return a.rollback(ctx, previousJSON, result, err)
	}
	result.Applied = true
	result.Version = v
	return result, nil
}
func (a Applier) rollback(ctx context.Context, previous []byte, result ApplyResult, cause error) (ApplyResult, error) {
	result.RolledBack = true
	saveErr := a.Control.SaveConfig(ctx, previous, true)
	if saveErr == nil {
		saveErr = a.Control.Restart(ctx)
	}
	if saveErr != nil {
		return result, fmt.Errorf("apply failed (%v); rollback also failed: %w", cause, saveErr)
	}
	_, readyErr := a.waitReady(ctx, 15*time.Second, nil)
	if readyErr != nil {
		return result, fmt.Errorf("apply failed (%v); rollback saved but did not become healthy: %w", cause, readyErr)
	}
	return result, fmt.Errorf("Frigate apply rolled back: %w", cause)
}
func (a Applier) waitReady(ctx context.Context, timeout time.Duration, expected []string) (string, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	interval := a.PollInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	var last error
	for {
		v, err := a.Control.Version(ctx)
		if err == nil {
			streams, e := a.Control.Go2RTCStreams(ctx)
			if e == nil {
				missing := ""
				for _, n := range expected {
					if _, ok := streams[n]; !ok {
						missing = n
						break
					}
				}
				if missing == "" {
					return v, nil
				}
				last = fmt.Errorf("expected go2rtc stream %s not active", missing)
			} else {
				last = e
			}
		} else {
			last = err
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("Frigate readiness timeout: %w", last)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
	}
}
