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
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

// Load resolves the runtime configuration in deterministic precedence order:
// defaults < optional SENTINEL_CONFIG JSON < SENTINEL_* environment overrides.
func Load() (Config, error) {
	cfg := Default()
	if path := strings.TrimSpace(os.Getenv("SENTINEL_CONFIG")); path != "" {
		if err := overlayRuntimeFile(&cfg, path); err != nil {
			return Config{}, err
		}
	}
	if err := applyRuntimeEnv(&cfg, os.LookupEnv); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type runtimeFileConfig struct {
	Server        *runtimeFileServer        `json:"server,omitempty"`
	Database      *runtimeFileDatabase      `json:"database,omitempty"`
	Network       *runtimeFileNetwork       `json:"network,omitempty"`
	Security      *runtimeFileSecurity      `json:"security,omitempty"`
	HomeAssistant *runtimeFileHomeAssistant `json:"home_assistant,omitempty"`
	Frigate       *runtimeFileFrigate       `json:"frigate,omitempty"`
	MQTT          *runtimeFileMQTT          `json:"mqtt,omitempty"`
	AI            *runtimeFileAI            `json:"ai,omitempty"`
	Telegram      *runtimeFileTelegram      `json:"telegram,omitempty"`
	Backup        *runtimeFileBackup        `json:"backup,omitempty"`
	Features      *runtimeFileFeatures      `json:"features,omitempty"`
}

type runtimeFileServer struct {
	ListenAddress *string `json:"listen,omitempty"`
	ReadTimeout   *string `json:"read_timeout,omitempty"`
	WriteTimeout  *string `json:"write_timeout,omitempty"`
	ShutdownGrace *string `json:"shutdown_grace,omitempty"`
}

type runtimeFileDatabase struct {
	Path        *string `json:"path,omitempty"`
	BusyTimeout *string `json:"busy_timeout,omitempty"`
}

type runtimeFileNetwork struct {
	CameraCIDRs *[]string `json:"camera_cidrs,omitempty"`
}

type runtimeFileSecurity struct {
	SessionTTL   *string               `json:"session_ttl,omitempty"`
	SecureCookie *bool                 `json:"secure_cookie,omitempty"`
	Callbacks    *runtimeFileCallbacks `json:"callbacks,omitempty"`
}

type runtimeFileCallbacks struct {
	Enabled        *bool              `json:"enabled,omitempty"`
	ActiveKeyID    *string            `json:"active_key_id,omitempty"`
	Keys           *map[string]string `json:"keys,omitempty"`
	MaxTTL         *string            `json:"max_ttl,omitempty"`
	ClockSkew      *string            `json:"clock_skew,omitempty"`
	ReplayCapacity *int               `json:"replay_capacity,omitempty"`
}

type runtimeFileHomeAssistant struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	URL      *string `json:"url,omitempty"`
	TokenRef *string `json:"token_ref,omitempty"`
}

type runtimeFileFrigate struct {
	Enabled              *bool     `json:"enabled,omitempty"`
	URL                  *string   `json:"url,omitempty"`
	Go2RTCURL            *string   `json:"go2rtc_url,omitempty"`
	WebRTCCandidates     *[]string `json:"webrtc_candidates,omitempty"`
	TokenRef             *string   `json:"token_ref,omitempty"`
	CredentialsDirectory *string   `json:"credentials_directory,omitempty"`
}

type runtimeFileMQTT struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	URL            *string `json:"url,omitempty"`
	ClientID       *string `json:"client_id,omitempty"`
	Username       *string `json:"username,omitempty"`
	PasswordRef    *string `json:"password_ref,omitempty"`
	KeepAlive      *string `json:"keep_alive,omitempty"`
	SessionExpiry  *string `json:"session_expiry,omitempty"`
	ConnectTimeout *string `json:"connect_timeout,omitempty"`
}

type runtimeFileAI struct {
	Enabled *bool   `json:"enabled,omitempty"`
	URL     *string `json:"url,omitempty"`
	Model   *string `json:"model,omitempty"`
}

type runtimeFileTelegram struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	TokenRef *string `json:"token_ref,omitempty"`
}

type runtimeFileBackup struct {
	Enabled     *bool     `json:"enabled,omitempty"`
	Repository  *string   `json:"repository,omitempty"`
	PasswordRef *string   `json:"password_ref,omitempty"`
	Interval    *string   `json:"interval,omitempty"`
	ConfigFiles *[]string `json:"config_files,omitempty"`
	KeepHourly  *int      `json:"keep_hourly,omitempty"`
	KeepDaily   *int      `json:"keep_daily,omitempty"`
	KeepWeekly  *int      `json:"keep_weekly,omitempty"`
	KeepMonthly *int      `json:"keep_monthly,omitempty"`
}

type runtimeFileFeatures struct {
	Experimental *bool `json:"experimental,omitempty"`
}

func overlayRuntimeFile(cfg *Config, path string) error {
	if cfg == nil {
		return errors.New("config: runtime target is required")
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("config: open runtime file %q: %w", path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxConfigBytes+1))
	if err != nil {
		return fmt.Errorf("config: read runtime file %q: %w", path, err)
	}
	if len(data) > MaxConfigBytes {
		return fmt.Errorf("config: runtime file exceeds %d bytes", MaxConfigBytes)
	}
	var wire runtimeFileConfig
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return fmt.Errorf("config: decode runtime file: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("config: multiple JSON values are not allowed")
		}
		return fmt.Errorf("config: trailing runtime data: %w", err)
	}
	return applyRuntimeFile(cfg, wire)
}

func applyRuntimeFile(cfg *Config, wire runtimeFileConfig) error {
	if v := wire.Server; v != nil {
		if v.ListenAddress != nil {
			cfg.Server.ListenAddress = strings.TrimSpace(*v.ListenAddress)
		}
		if err := assignDuration("server.read_timeout", v.ReadTimeout, &cfg.Server.ReadTimeout); err != nil {
			return err
		}
		if err := assignDuration("server.write_timeout", v.WriteTimeout, &cfg.Server.WriteTimeout); err != nil {
			return err
		}
		if err := assignDuration("server.shutdown_grace", v.ShutdownGrace, &cfg.Server.ShutdownGrace); err != nil {
			return err
		}
	}
	if v := wire.Database; v != nil {
		if v.Path != nil {
			cfg.Database.Path = strings.TrimSpace(*v.Path)
		}
		if err := assignDuration("database.busy_timeout", v.BusyTimeout, &cfg.Database.BusyTimeout); err != nil {
			return err
		}
	}
	if v := wire.Network; v != nil && v.CameraCIDRs != nil {
		cfg.Network.CameraCIDRs = append([]string(nil), (*v.CameraCIDRs)...)
	}
	if v := wire.Security; v != nil {
		if err := assignDuration("security.session_ttl", v.SessionTTL, &cfg.Security.SessionTTL); err != nil {
			return err
		}
		if v.SecureCookie != nil {
			cfg.Security.SecureCookie = *v.SecureCookie
		}
		if cb := v.Callbacks; cb != nil {
			assignBool(cb.Enabled, &cfg.Security.Callbacks.Enabled)
			assignString(cb.ActiveKeyID, &cfg.Security.Callbacks.ActiveKeyID)
			if cb.Keys != nil {
				cfg.Security.Callbacks.Keys = make(map[string]secrets.Ref, len(*cb.Keys))
				for id, ref := range *cb.Keys {
					cfg.Security.Callbacks.Keys[strings.TrimSpace(id)] = secrets.Ref(strings.TrimSpace(ref))
				}
			}
			if err := assignDuration("security.callbacks.max_ttl", cb.MaxTTL, &cfg.Security.Callbacks.MaxTTL); err != nil {
				return err
			}
			if err := assignDuration("security.callbacks.clock_skew", cb.ClockSkew, &cfg.Security.Callbacks.ClockSkew); err != nil {
				return err
			}
			assignInt(cb.ReplayCapacity, &cfg.Security.Callbacks.ReplayCapacity)
		}
	}
	if v := wire.HomeAssistant; v != nil {
		assignBool(v.Enabled, &cfg.HomeAssistant.Enabled)
		assignString(v.URL, &cfg.HomeAssistant.URL)
		assignSecretRef(v.TokenRef, &cfg.HomeAssistant.TokenRef)
	}
	if v := wire.Frigate; v != nil {
		assignBool(v.Enabled, &cfg.Frigate.Enabled)
		assignString(v.URL, &cfg.Frigate.URL)
		assignString(v.Go2RTCURL, &cfg.Frigate.Go2RTCURL)
		assignSecretRef(v.TokenRef, &cfg.Frigate.TokenRef)
		assignString(v.CredentialsDirectory, &cfg.Frigate.CredentialsDirectory)
		if v.WebRTCCandidates != nil {
			cfg.Frigate.WebRTCCandidates = append([]string(nil), (*v.WebRTCCandidates)...)
		}
	}
	if v := wire.MQTT; v != nil {
		assignBool(v.Enabled, &cfg.MQTT.Enabled)
		assignString(v.URL, &cfg.MQTT.URL)
		assignString(v.ClientID, &cfg.MQTT.ClientID)
		assignString(v.Username, &cfg.MQTT.Username)
		assignSecretRef(v.PasswordRef, &cfg.MQTT.PasswordRef)
		if err := assignDuration("mqtt.keep_alive", v.KeepAlive, &cfg.MQTT.KeepAlive); err != nil {
			return err
		}
		if err := assignDuration("mqtt.session_expiry", v.SessionExpiry, &cfg.MQTT.SessionExpiry); err != nil {
			return err
		}
		if err := assignDuration("mqtt.connect_timeout", v.ConnectTimeout, &cfg.MQTT.ConnectTimeout); err != nil {
			return err
		}
	}
	if v := wire.AI; v != nil {
		assignBool(v.Enabled, &cfg.AI.Enabled)
		assignString(v.URL, &cfg.AI.URL)
		assignString(v.Model, &cfg.AI.Model)
	}
	if v := wire.Telegram; v != nil {
		assignBool(v.Enabled, &cfg.Telegram.Enabled)
		assignSecretRef(v.TokenRef, &cfg.Telegram.TokenRef)
	}
	if v := wire.Backup; v != nil {
		assignBool(v.Enabled, &cfg.Backup.Enabled)
		assignString(v.Repository, &cfg.Backup.Repository)
		assignSecretRef(v.PasswordRef, &cfg.Backup.PasswordRef)
		if err := assignDuration("backup.interval", v.Interval, &cfg.Backup.Interval); err != nil {
			return err
		}
		if v.ConfigFiles != nil {
			cfg.Backup.ConfigFiles = append([]string(nil), (*v.ConfigFiles)...)
		}
		assignInt(v.KeepHourly, &cfg.Backup.KeepHourly)
		assignInt(v.KeepDaily, &cfg.Backup.KeepDaily)
		assignInt(v.KeepWeekly, &cfg.Backup.KeepWeekly)
		assignInt(v.KeepMonthly, &cfg.Backup.KeepMonthly)
	}
	if v := wire.Features; v != nil {
		assignBool(v.Experimental, &cfg.Features.Experimental)
	}
	return nil
}

func applyRuntimeEnv(cfg *Config, lookup LookupEnv) error {
	if cfg == nil {
		return errors.New("config: runtime target is required")
	}
	if lookup == nil {
		return nil
	}
	stringEnv := func(key string, target *string) {
		if value, ok := lookup(key); ok {
			*target = strings.TrimSpace(value)
		}
	}
	boolEnv := func(key string, target *bool) error {
		value, ok := lookup(key)
		if !ok {
			return nil
		}
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("config: %s: %w", key, err)
		}
		*target = parsed
		return nil
	}
	durationEnv := func(key string, target *time.Duration) error {
		value, ok := lookup(key)
		if !ok {
			return nil
		}
		parsed, err := time.ParseDuration(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("config: %s: %w", key, err)
		}
		*target = parsed
		return nil
	}
	secretEnv := func(key string, target *secrets.Ref) {
		if value, ok := lookup(key); ok {
			*target = secrets.Ref(strings.TrimSpace(value))
		}
	}
	csvEnv := func(key string, target *[]string) {
		value, ok := lookup(key)
		if !ok {
			return
		}
		*target = splitCSV(value)
	}
	intEnv := func(key string, target *int) error {
		value, ok := lookup(key)
		if !ok {
			return nil
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("config: %s: %w", key, err)
		}
		*target = parsed
		return nil
	}

	stringEnv("SENTINEL_LISTEN", &cfg.Server.ListenAddress)
	stringEnv("SENTINEL_DB_PATH", &cfg.Database.Path)
	stringEnv("SENTINEL_HA_URL", &cfg.HomeAssistant.URL)
	stringEnv("SENTINEL_FRIGATE_URL", &cfg.Frigate.URL)
	stringEnv("SENTINEL_GO2RTC_URL", &cfg.Frigate.Go2RTCURL)
	stringEnv("SENTINEL_FRIGATE_CREDENTIALS_DIR", &cfg.Frigate.CredentialsDirectory)
	stringEnv("SENTINEL_MQTT_URL", &cfg.MQTT.URL)
	stringEnv("SENTINEL_MQTT_CLIENT_ID", &cfg.MQTT.ClientID)
	stringEnv("SENTINEL_MQTT_USERNAME", &cfg.MQTT.Username)
	stringEnv("SENTINEL_AI_URL", &cfg.AI.URL)
	stringEnv("SENTINEL_AI_MODEL", &cfg.AI.Model)
	stringEnv("SENTINEL_BACKUP_REPOSITORY", &cfg.Backup.Repository)
	stringEnv("SENTINEL_CALLBACK_ACTIVE_KEY_ID", &cfg.Security.Callbacks.ActiveKeyID)

	csvEnv("SENTINEL_CAMERA_CIDRS", &cfg.Network.CameraCIDRs)
	csvEnv("SENTINEL_FRIGATE_WEBRTC_CANDIDATES", &cfg.Frigate.WebRTCCandidates)
	csvEnv("SENTINEL_BACKUP_CONFIG_FILES", &cfg.Backup.ConfigFiles)

	secretEnv("SENTINEL_HA_TOKEN_REF", &cfg.HomeAssistant.TokenRef)
	secretEnv("SENTINEL_FRIGATE_TOKEN_REF", &cfg.Frigate.TokenRef)
	secretEnv("SENTINEL_MQTT_PASSWORD_REF", &cfg.MQTT.PasswordRef)
	secretEnv("SENTINEL_TELEGRAM_TOKEN_REF", &cfg.Telegram.TokenRef)
	secretEnv("SENTINEL_BACKUP_PASSWORD_REF", &cfg.Backup.PasswordRef)
	if raw, ok := lookup("SENTINEL_CALLBACK_KEYS"); ok {
		refs, err := parseSecretRefMap(raw)
		if err != nil {
			return fmt.Errorf("config: SENTINEL_CALLBACK_KEYS: %w", err)
		}
		cfg.Security.Callbacks.Keys = refs
	}

	for key, target := range map[string]*bool{
		"SENTINEL_HA_ENABLED":       &cfg.HomeAssistant.Enabled,
		"SENTINEL_FRIGATE_ENABLED":  &cfg.Frigate.Enabled,
		"SENTINEL_MQTT_ENABLED":     &cfg.MQTT.Enabled,
		"SENTINEL_AI_ENABLED":       &cfg.AI.Enabled,
		"SENTINEL_TELEGRAM_ENABLED": &cfg.Telegram.Enabled,
		"SENTINEL_BACKUP_ENABLED":   &cfg.Backup.Enabled,
		"SENTINEL_SECURE_COOKIE":    &cfg.Security.SecureCookie,
		"SENTINEL_CALLBACK_ENABLED": &cfg.Security.Callbacks.Enabled,
		"SENTINEL_EXPERIMENTAL":     &cfg.Features.Experimental,
	} {
		if err := boolEnv(key, target); err != nil {
			return err
		}
	}
	for key, target := range map[string]*time.Duration{
		"SENTINEL_READ_TIMEOUT":          &cfg.Server.ReadTimeout,
		"SENTINEL_WRITE_TIMEOUT":         &cfg.Server.WriteTimeout,
		"SENTINEL_SHUTDOWN_GRACE":        &cfg.Server.ShutdownGrace,
		"SENTINEL_DB_BUSY_TIMEOUT":       &cfg.Database.BusyTimeout,
		"SENTINEL_SESSION_TTL":           &cfg.Security.SessionTTL,
		"SENTINEL_CALLBACK_MAX_TTL":      &cfg.Security.Callbacks.MaxTTL,
		"SENTINEL_CALLBACK_CLOCK_SKEW":   &cfg.Security.Callbacks.ClockSkew,
		"SENTINEL_MQTT_KEEP_ALIVE":       &cfg.MQTT.KeepAlive,
		"SENTINEL_MQTT_SESSION_EXPIRY":   &cfg.MQTT.SessionExpiry,
		"SENTINEL_MQTT_CONNECT_TIMEOUT":  &cfg.MQTT.ConnectTimeout,
		"SENTINEL_BACKUP_INTERVAL":       &cfg.Backup.Interval,
	} {
		if err := durationEnv(key, target); err != nil {
			return err
		}
	}
	for key, target := range map[string]*int{
		"SENTINEL_CALLBACK_REPLAY_CAPACITY": &cfg.Security.Callbacks.ReplayCapacity,
		"SENTINEL_BACKUP_KEEP_HOURLY":       &cfg.Backup.KeepHourly,
		"SENTINEL_BACKUP_KEEP_DAILY":        &cfg.Backup.KeepDaily,
		"SENTINEL_BACKUP_KEEP_WEEKLY":       &cfg.Backup.KeepWeekly,
		"SENTINEL_BACKUP_KEEP_MONTHLY":      &cfg.Backup.KeepMonthly,
	} {
		if err := intEnv(key, target); err != nil {
			return err
		}
	}
	return nil
}

func assignDuration(name string, source *string, target *time.Duration) error {
	if source == nil {
		return nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(*source))
	if err != nil {
		return fmt.Errorf("config: %s: %w", name, err)
	}
	*target = parsed
	return nil
}

func assignString(source *string, target *string) {
	if source != nil {
		*target = strings.TrimSpace(*source)
	}
}

func assignBool(source *bool, target *bool) {
	if source != nil {
		*target = *source
	}
}

func assignInt(source *int, target *int) {
	if source != nil {
		*target = *source
	}
}

func assignSecretRef(source *string, target *secrets.Ref) {
	if source != nil {
		*target = secrets.Ref(strings.TrimSpace(*source))
	}
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseSecretRefMap(raw string) (map[string]secrets.Ref, error) {
	out := map[string]secrets.Ref{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		id, ref, ok := strings.Cut(entry, "=")
		id = strings.TrimSpace(id)
		ref = strings.TrimSpace(ref)
		if !ok || id == "" || ref == "" {
			return nil, fmt.Errorf("expected key-id=secret-ref entry, got %q", entry)
		}
		if _, exists := out[id]; exists {
			return nil, fmt.Errorf("duplicate callback key id %q", id)
		}
		out[id] = secrets.Ref(ref)
	}
	if len(out) == 0 {
		return nil, errors.New("at least one callback key reference is required")
	}
	return out, nil
}
