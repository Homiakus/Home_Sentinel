package frigate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Capabilities struct {
	Version       string `json:"version"`
	ConfigRead    bool   `json:"config_read"`
	ConfigSave    bool   `json:"config_save"`
	Go2RTCStreams bool   `json:"go2rtc_streams"`
	Events        bool   `json:"events"`
	Media         bool   `json:"media"`
}

type CapabilityDiagnostic struct {
	Compatible bool     `json:"compatible"`
	Reasons    []string `json:"reasons,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

func ProbeCapabilities(ctx context.Context, c *Client) (Capabilities, CapabilityDiagnostic, error) {
	var caps Capabilities
	var diag CapabilityDiagnostic
	v, err := c.Version(ctx)
	if err != nil {
		return caps, diag, fmt.Errorf("Frigate version probe failed: %w", err)
	}
	caps.Version = v
	if strings.TrimSpace(v) == "" {
		return caps, diag, errors.New("Frigate returned an empty version")
	}
	if _, err := c.Config(ctx); err == nil {
		caps.ConfigRead = true
		caps.ConfigSave = true
	} else if !isNotFound(err) {
		diag.Warnings = append(diag.Warnings, "configuration API probe failed: "+briefErr(err))
	}
	if _, err := c.Go2RTCStreams(ctx); err == nil {
		caps.Go2RTCStreams = true
	} else if !isNotFound(err) {
		diag.Warnings = append(diag.Warnings, "go2rtc stream API probe failed: "+briefErr(err))
	}
	// Event and media endpoints are part of the API surface of supported Frigate releases.
	// They are marked available only after the base/version API is reachable; individual 404s
	// are still handled by the typed client at runtime.
	caps.Events, caps.Media = true, true
	diag.Compatible = caps.ConfigRead && caps.ConfigSave && caps.Go2RTCStreams
	if !caps.ConfigRead {
		diag.Reasons = append(diag.Reasons, "Frigate configuration API is unavailable")
	}
	if !caps.ConfigSave {
		diag.Reasons = append(diag.Reasons, "Frigate configuration validation/save API is unavailable")
	}
	if !caps.Go2RTCStreams {
		diag.Reasons = append(diag.Reasons, "Frigate go2rtc stream introspection API is unavailable")
	}
	return caps, diag, nil
}

func RequireCapabilities(c Capabilities, names ...string) error {
	for _, n := range names {
		var ok bool
		switch n {
		case "config_read":
			ok = c.ConfigRead
		case "config_save":
			ok = c.ConfigSave
		case "go2rtc_streams":
			ok = c.Go2RTCStreams
		case "events":
			ok = c.Events
		case "media":
			ok = c.Media
		default:
			return fmt.Errorf("unknown Frigate capability %q", n)
		}
		if !ok {
			return fmt.Errorf("Frigate %s does not provide required capability %s", c.Version, n)
		}
	}
	return nil
}

func isNotFound(err error) bool {
	var e *HTTPError
	return errors.As(err, &e) && e.StatusCode == http.StatusNotFound
}
func briefErr(err error) string {
	var he *HTTPError
	if errors.As(err, &he) {
		return fmt.Sprintf("HTTP %d", he.StatusCode)
	}
	var ne *NetworkError
	if errors.As(err, &ne) {
		return "network error"
	}
	return err.Error()
}
