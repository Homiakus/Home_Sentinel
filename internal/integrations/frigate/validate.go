package frigate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	fgconfig "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate/config"
)

type MediaVerifier interface{ VerifyManagedMedia(context.Context) error }
type PreflightReport struct {
	Valid    bool     `json:"valid"`
	Checks   []string `json:"checks"`
	Warnings []string `json:"warnings,omitempty"`
}

func Preflight(ctx context.Context, c *Client, caps Capabilities, generated []byte, ownership fgconfig.Ownership, media MediaVerifier) (PreflightReport, error) {
	r := PreflightReport{}
	if err := RequireCapabilities(caps, "config_read", "config_save", "go2rtc_streams"); err != nil {
		return r, err
	}
	r.Checks = append(r.Checks, "required Frigate API capabilities available")
	if !json.Valid(generated) {
		return r, errors.New("generated Frigate configuration is not valid JSON")
	}
	var root map[string]any
	if err := json.Unmarshal(generated, &root); err != nil {
		return r, err
	}
	if err := validateManagedShape(root, ownership); err != nil {
		return r, err
	}
	if strings.Contains(string(generated), "secret://") {
		return r, errors.New("generated Frigate config contains Sentinel secret references")
	}
	r.Checks = append(r.Checks, "generated configuration shape and secret boundaries valid")
	if c != nil {
		if _, err := c.ConfigSchema(ctx); err != nil {
			return r, fmt.Errorf("Frigate config schema endpoint unavailable: %w", err)
		}
		r.Checks = append(r.Checks, "Frigate config schema endpoint reachable")
	}
	if media != nil {
		if err := media.VerifyManagedMedia(ctx); err != nil {
			return r, fmt.Errorf("camera media preflight failed: %w", err)
		}
		r.Checks = append(r.Checks, "camera media readiness verified")
	}
	r.Valid = true
	return r, nil
}
func validateManagedShape(root map[string]any, o fgconfig.Ownership) error {
	cams, _ := root["cameras"].(map[string]any)
	for _, name := range o.CameraNames {
		if _, ok := cams[name]; !ok {
			return fmt.Errorf("managed camera %s missing from generated config", name)
		}
	}
	g, _ := root["go2rtc"].(map[string]any)
	streams, _ := g["streams"].(map[string]any)
	for _, name := range o.StreamNames {
		if _, ok := streams[name]; !ok {
			return fmt.Errorf("managed go2rtc stream %s missing from generated config", name)
		}
	}
	return nil
}
