package config

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Network       NetworkConfig
	Security      SecurityConfig
	HomeAssistant HomeAssistantConfig
	Frigate       FrigateConfig
	MQTT          MQTTConfig
	AI            AIConfig
	Telegram      TelegramConfig
	Backup        BackupConfig
	Features      FeatureConfig
}

type ServerConfig struct {
	ListenAddress string
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	ShutdownGrace time.Duration
}

type DatabaseConfig struct {
	Path        string
	BusyTimeout time.Duration
}

type NetworkConfig struct{ CameraCIDRs []string }

type SecurityConfig struct {
	SessionTTL   time.Duration
	SecureCookie bool
	Callbacks    CallbackRuntimeConfig
}

type HomeAssistantConfig struct {
	Enabled  bool
	URL      string
	TokenRef secrets.Ref
}
type FrigateConfig struct {
	Enabled              bool
	URL                  string
	Go2RTCURL            string
	WebRTCCandidates     []string
	TokenRef             secrets.Ref
	CredentialsDirectory string
}
type MQTTConfig struct {
	Enabled        bool
	URL            string
	ClientID       string
	Username       string
	PasswordRef    secrets.Ref
	KeepAlive      time.Duration
	SessionExpiry  time.Duration
	ConnectTimeout time.Duration
}
type AIConfig struct {
	Enabled bool
	URL     string
	Model   string
}
type TelegramConfig struct {
	Enabled  bool
	TokenRef secrets.Ref
}
type BackupConfig struct {
	Enabled     bool
	Repository  string
	PasswordRef secrets.Ref
	Interval    time.Duration
	ConfigFiles []string
	KeepHourly  int
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
}
type FeatureConfig struct{ Experimental bool }

func Default() Config {
	return Config{
		Server: ServerConfig{
			ListenAddress: "127.0.0.1:8080",
			ReadTimeout:   15 * time.Second,
			WriteTimeout:  30 * time.Second,
			ShutdownGrace: 10 * time.Second,
		},
		Database: DatabaseConfig{Path: filepath.FromSlash("data/sentinel.db"), BusyTimeout: 5 * time.Second},
		Network:  NetworkConfig{CameraCIDRs: []string{"192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"}},
		Security: SecurityConfig{SessionTTL: 12 * time.Hour, SecureCookie: true, Callbacks: DefaultCallbackRuntimeConfig()},
		AI:       AIConfig{URL: "http://127.0.0.1:11434"},
		Frigate:  FrigateConfig{CredentialsDirectory: filepath.FromSlash("data/frigate-secrets")},
		Backup:   BackupConfig{Interval: 6 * time.Hour, KeepHourly: 24, KeepDaily: 14, KeepWeekly: 8, KeepMonthly: 12},
		MQTT: MQTTConfig{
			URL:            "mqtt://127.0.0.1:1883",
			ClientID:       "home-sentinel",
			KeepAlive:      30 * time.Second,
			SessionExpiry:  10 * time.Minute,
			ConnectTimeout: 10 * time.Second,
		},
	}
}

func (c Config) Validate() error {
	if c.Server.ListenAddress == "" {
		return errors.New("server listen address required")
	}
	host, _, err := net.SplitHostPort(c.Server.ListenAddress)
	if err != nil {
		return errors.New("server listen address must be host:port")
	}
	// The production HTTP server currently uses ListenAndServe and does not
	// terminate TLS itself. Until a verified TLS listener is wired into the
	// runtime, fail closed rather than allowing credentials or control commands
	// to be exposed over a remote plaintext bind.
	if !loopbackHost(host) {
		return ErrInsecureRemoteBind
	}
	if c.Server.ReadTimeout <= 0 || c.Server.WriteTimeout <= 0 || c.Server.ShutdownGrace <= 0 {
		return errors.New("server timeouts must be positive")
	}
	if c.Database.Path == "" || c.Database.BusyTimeout <= 0 {
		return errors.New("database path and positive busy timeout required")
	}
	if c.Security.SessionTTL <= 0 {
		return errors.New("security session TTL must be positive")
	}
	if err := c.Security.Callbacks.Validate(); err != nil {
		return err
	}
	for _, raw := range c.Network.CameraCIDRs {
		if _, err := netip.ParsePrefix(raw); err != nil {
			return errors.New("invalid camera CIDR: " + raw)
		}
	}
	for name, pair := range map[string]struct {
		enabled bool
		raw     string
	}{
		"home_assistant": {c.HomeAssistant.Enabled, c.HomeAssistant.URL},
		"frigate":        {c.Frigate.Enabled, c.Frigate.URL},
		"mqtt":           {c.MQTT.Enabled, c.MQTT.URL},
		"ai":             {c.AI.Enabled, c.AI.URL},
	} {
		if !pair.enabled {
			continue
		}
		u, err := url.Parse(pair.raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return errors.New(name + " URL invalid")
		}
	}
	if c.AI.Enabled && c.AI.Model == "" {
		return errors.New("ai model required when AI is enabled")
	}
	if c.HomeAssistant.Enabled && c.HomeAssistant.TokenRef == "" {
		return errors.New("home_assistant token secret reference required")
	}
	if c.Frigate.Enabled && c.Frigate.CredentialsDirectory == "" {
		return errors.New("frigate credentials directory required")
	}
	if c.Frigate.Go2RTCURL != "" {
		u, err := url.Parse(c.Frigate.Go2RTCURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("frigate go2rtc URL invalid")
		}
	}
	for _, candidate := range c.Frigate.WebRTCCandidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || len(candidate) > 160 || strings.ContainsAny(candidate, "\r\n\t ") {
			return errors.New("frigate WebRTC candidate invalid")
		}
	}
	if c.MQTT.Enabled {
		if c.MQTT.ClientID == "" {
			return errors.New("mqtt client id required")
		}
		if c.MQTT.KeepAlive <= 0 || c.MQTT.ConnectTimeout <= 0 || c.MQTT.SessionExpiry < 0 {
			return errors.New("mqtt timing settings invalid")
		}
		if c.MQTT.PasswordRef == "" && c.MQTT.Username != "" {
			return errors.New("mqtt password secret reference required when username is set")
		}
	}
	if c.Telegram.Enabled && c.Telegram.TokenRef == "" {
		return errors.New("telegram token secret reference required")
	}
	if c.Backup.Enabled && c.Backup.Repository == "" {
		return errors.New("backup repository required")
	}
	if c.Backup.Enabled && c.Backup.PasswordRef == "" {
		return errors.New("backup password secret reference required")
	}
	if c.Backup.Enabled && c.Backup.Interval <= 0 {
		return errors.New("backup interval must be positive")
	}
	if c.Backup.KeepHourly < 0 || c.Backup.KeepDaily < 0 || c.Backup.KeepWeekly < 0 || c.Backup.KeepMonthly < 0 {
		return errors.New("backup retention values cannot be negative")
	}
	return nil
}
