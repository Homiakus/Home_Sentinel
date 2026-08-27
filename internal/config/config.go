package config

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/security/callback"
)

const (
	CurrentVersion = 1
	MaxConfigBytes = 1 << 20
)

var (
	ErrUnsupportedVersion = errors.New("config: unsupported version")
	ErrInsecureRemoteBind = errors.New("config: remote bind requires TLS")
)

// HardenedConfig is the versioned, fail-closed configuration document used by
// the security/bootstrap configuration path. It is intentionally distinct
// from Config, the runtime application model consumed by app.Open.
//
// Keeping the two contracts explicitly named prevents accidental structural
// aliasing while the runtime configuration migration is completed.
type HardenedConfig struct {
	Version  int                    `json:"version"`
	Server   HardenedServerConfig   `json:"server"`
	Storage  StorageConfig          `json:"storage"`
	Security HardenedSecurityConfig `json:"security"`
	Runtime  RuntimeConfig          `json:"runtime"`
}

type HardenedServerConfig struct {
	Listen                   string    `json:"listen"`
	MaxBodyBytes             int64     `json:"maxBodyBytes"`
	ReadHeaderTimeoutSeconds int       `json:"readHeaderTimeoutSeconds"`
	ReadTimeoutSeconds       int       `json:"readTimeoutSeconds"`
	WriteTimeoutSeconds      int       `json:"writeTimeoutSeconds"`
	IdleTimeoutSeconds       int       `json:"idleTimeoutSeconds"`
	TLS                      TLSConfig `json:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"certFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty"`
}

type StorageConfig struct {
	Root         string `json:"root"`
	MinFreeBytes int64  `json:"minFreeBytes"`
}

type RuntimeConfig struct {
	WorkerConcurrency      int `json:"workerConcurrency"`
	ShutdownTimeoutSeconds int `json:"shutdownTimeoutSeconds"`
}

type HardenedSecurityConfig struct {
	Callbacks CallbackConfig `json:"callbacks"`
}

type CallbackConfig struct {
	Enabled          bool                 `json:"enabled"`
	ActiveKeyID      string               `json:"activeKeyId,omitempty"`
	Keys             map[string]SecretRef `json:"keys,omitempty"`
	MaxTTLSeconds    int                  `json:"maxTTLSeconds"`
	ClockSkewSeconds int                  `json:"clockSkewSeconds"`
	ReplayCapacity   int                  `json:"replayCapacity"`
}

type SecretRef struct {
	// Env contains the environment-variable name, never the secret value.
	Env string `json:"env"`
}

func DefaultHardened() HardenedConfig {
	return HardenedConfig{
		Version: CurrentVersion,
		Server: HardenedServerConfig{
			Listen:                   "127.0.0.1:8080",
			MaxBodyBytes:             1 << 20,
			ReadHeaderTimeoutSeconds: 5,
			ReadTimeoutSeconds:       15,
			WriteTimeoutSeconds:      30,
			IdleTimeoutSeconds:       60,
		},
		Storage: StorageConfig{
			Root:         "./data",
			MinFreeBytes: 256 << 20,
		},
		Runtime: RuntimeConfig{
			WorkerConcurrency:      4,
			ShutdownTimeoutSeconds: 30,
		},
		Security: HardenedSecurityConfig{Callbacks: CallbackConfig{
			MaxTTLSeconds:    int(callback.DefaultOptions.MaxTTL / time.Second),
			ClockSkewSeconds: int(callback.DefaultOptions.ClockSkew / time.Second),
			ReplayCapacity:   4096,
		}},
	}
}

func (c HardenedConfig) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("%w: got %d want %d", ErrUnsupportedVersion, c.Version, CurrentVersion)
	}
	if err := c.Server.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Storage.Root) == "" {
		return errors.New("config: storage root is required")
	}
	if filepath.Clean(c.Storage.Root) == "." && c.Storage.Root != "." && c.Storage.Root != "./" {
		return errors.New("config: invalid storage root")
	}
	if c.Storage.MinFreeBytes < 0 {
		return errors.New("config: minFreeBytes must be >= 0")
	}
	if c.Runtime.WorkerConcurrency <= 0 || c.Runtime.WorkerConcurrency > 256 {
		return errors.New("config: workerConcurrency must be between 1 and 256")
	}
	if c.Runtime.ShutdownTimeoutSeconds <= 0 || c.Runtime.ShutdownTimeoutSeconds > 600 {
		return errors.New("config: shutdownTimeoutSeconds must be between 1 and 600")
	}
	if err := c.Security.Callbacks.Validate(); err != nil {
		return err
	}
	return nil
}

func (c HardenedServerConfig) Validate() error {
	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return fmt.Errorf("config: invalid server listen address: %w", err)
	}
	if c.MaxBodyBytes <= 0 || c.MaxBodyBytes > 16<<20 {
		return errors.New("config: maxBodyBytes must be between 1 and 16777216")
	}
	if c.ReadHeaderTimeoutSeconds <= 0 || c.ReadTimeoutSeconds <= 0 || c.WriteTimeoutSeconds <= 0 || c.IdleTimeoutSeconds <= 0 {
		return errors.New("config: all HTTP timeout values must be > 0")
	}
	if !loopbackHost(host) && !c.TLS.Enabled {
		return ErrInsecureRemoteBind
	}
	if c.TLS.Enabled && (strings.TrimSpace(c.TLS.CertFile) == "" || strings.TrimSpace(c.TLS.KeyFile) == "") {
		return errors.New("config: TLS certFile and keyFile are required when TLS is enabled")
	}
	return nil
}

func loopbackHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c CallbackConfig) Validate() error {
	if c.MaxTTLSeconds <= 0 || c.MaxTTLSeconds > 3600 {
		return errors.New("config: callback maxTTLSeconds must be between 1 and 3600")
	}
	if c.ClockSkewSeconds < 0 || c.ClockSkewSeconds > 300 {
		return errors.New("config: callback clockSkewSeconds must be between 0 and 300")
	}
	if c.ReplayCapacity <= 0 || c.ReplayCapacity > 1_000_000 {
		return errors.New("config: callback replayCapacity must be between 1 and 1000000")
	}
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.ActiveKeyID) == "" {
		return errors.New("config: callback activeKeyId is required when callbacks are enabled")
	}
	if len(c.Keys) == 0 {
		return errors.New("config: callback keys are required when callbacks are enabled")
	}
	if _, ok := c.Keys[c.ActiveKeyID]; !ok {
		return errors.New("config: callback activeKeyId is not present in keys")
	}
	for id, ref := range c.Keys {
		if strings.TrimSpace(id) == "" {
			return errors.New("config: callback key id cannot be empty")
		}
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("config: callback key %q: %w", id, err)
		}
	}
	return nil
}

func (r SecretRef) Validate() error {
	value := strings.TrimSpace(r.Env)
	if value == "" {
		return errors.New("secret reference env name is required")
	}
	for i, ch := range value {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || (i > 0 && ch >= '0' && ch <= '9') {
			continue
		}
		return errors.New("secret reference contains invalid environment-variable name")
	}
	return nil
}

func (c CallbackConfig) Options() callback.Options {
	return callback.Options{
		MaxTTL:    time.Duration(c.MaxTTLSeconds) * time.Second,
		ClockSkew: time.Duration(c.ClockSkewSeconds) * time.Second,
	}
}
