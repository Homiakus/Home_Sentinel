package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type LookupEnv func(string) (string, bool)

func Decode(reader io.Reader, maxBytes int64) (Config, error) {
	if reader == nil {
		return Config{}, errors.New("config: reader is required")
	}
	if maxBytes <= 0 {
		maxBytes = MaxConfigBytes
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("config: read: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return Config{}, fmt.Errorf("config: file exceeds %d bytes", maxBytes)
	}
	cfg := Default()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("config: decode: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("config: multiple JSON values are not allowed")
		}
		return Config{}, fmt.Errorf("config: trailing data: %w", err)
	}
	return cfg, nil
}

func LoadFile(path string, lookup LookupEnv) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: open %q: %w", path, err)
	}
	defer file.Close()
	cfg, err := Decode(file, MaxConfigBytes)
	if err != nil {
		return Config{}, err
	}
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if err := ApplyEnv(&cfg, lookup); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ApplyEnv overlays only non-secret values and secret references. Secret bytes
// themselves are never copied into Config.
func ApplyEnv(cfg *Config, lookup LookupEnv) error {
	if cfg == nil {
		return errors.New("config: target is required")
	}
	if lookup == nil {
		return nil
	}
	if value, ok := lookup("HOME_SENTINEL_LISTEN"); ok {
		cfg.Server.Listen = strings.TrimSpace(value)
	}
	if value, ok := lookup("HOME_SENTINEL_DATA_ROOT"); ok {
		cfg.Storage.Root = strings.TrimSpace(value)
	}
	if value, ok := lookup("HOME_SENTINEL_CALLBACK_ACTIVE_KEY_ID"); ok {
		cfg.Security.Callbacks.ActiveKeyID = strings.TrimSpace(value)
	}
	if value, ok := lookup("HOME_SENTINEL_CALLBACK_ENABLED"); ok {
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("config: HOME_SENTINEL_CALLBACK_ENABLED: %w", err)
		}
		cfg.Security.Callbacks.Enabled = parsed
	}
	return nil
}

func SanitizedJSON(cfg Config) ([]byte, error) {
	// Config deliberately contains only references to secret material, so normal
	// JSON serialization is safe for support diagnostics.
	return json.MarshalIndent(cfg, "", "  ")
}
