package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/ai"
	"github.com/Homiakus/Home_Sentinel/internal/ai/ollama"
	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/backup"
	resticint "github.com/Homiakus/Home_Sentinel/internal/backup/restic"
	"github.com/Homiakus/Home_Sentinel/internal/buildinfo"
	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	eventadapters "github.com/Homiakus/Home_Sentinel/internal/events/adapters"
	"github.com/Homiakus/Home_Sentinel/internal/hardware"
	"github.com/Homiakus/Home_Sentinel/internal/health"
	"github.com/Homiakus/Home_Sentinel/internal/incidents"
	frigateint "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate"
	haint "github.com/Homiakus/Home_Sentinel/internal/integrations/homeassistant"
	mqttint "github.com/Homiakus/Home_Sentinel/internal/integrations/mqtt"
	tgapi "github.com/Homiakus/Home_Sentinel/internal/integrations/telegram"
	"github.com/Homiakus/Home_Sentinel/internal/intercom"
	"github.com/Homiakus/Home_Sentinel/internal/locks"
	"github.com/Homiakus/Home_Sentinel/internal/realtime"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	searchsvc "github.com/Homiakus/Home_Sentinel/internal/search"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
	"github.com/Homiakus/Home_Sentinel/internal/security/netpolicy"
	setupsvc "github.com/Homiakus/Home_Sentinel/internal/setup"
	tgsvc "github.com/Homiakus/Home_Sentinel/internal/telegram"
	"github.com/Homiakus/Home_Sentinel/internal/telemetry"
	"github.com/Homiakus/Home_Sentinel/internal/watchdog"
)

type App struct {
	Config                 config.Config
	Log                    *slog.Logger
	DB                     *sql.DB
	Locks                  *locks.Manager
	Revisions              *repository.RevisionStore
	Audit                  *repository.AuditStore
	Users                  *auth.UserStore
	Sessions               *auth.SessionStore
	Realtime               *realtime.Broker
	Events                 *events.Bus
	Outbox                 events.Outbox
	Hardware               hardware.Profile
	HardwareRecommendation hardware.Recommendation
	Health                 *health.Registry
	Cameras                *cameras.Service
	Secrets                secrets.Resolver
	CallbackSecurity       CallbackSecurity
	Frigate                *frigateint.Service
	MQTT                   *mqttint.Client
	HomeAssistant          *haint.Service
	HomeAssistantSetup     *setupsvc.HomeAssistantSetup
	Intercom               *intercom.Service
	Incidents              *incidents.Service
	AI                     *ai.Service
	AIPolicies             ai.PolicyStore
	Telegram               *tgsvc.Service
	IncidentRuntime        *IncidentRuntime
	Metrics                *telemetry.Metrics
	Search                 *searchsvc.Service
	Watchdog               *watchdog.Manager
	Backup                 *backup.Manager
	BackupScheduler        *backup.Scheduler
	BackupRestic           *resticint.Client
	runCtx                 context.Context
	runCancel              context.CancelFunc
	started                time.Time
}

// New creates a side-effect-free application object. Tests that do not need
// persistence can use it directly. Production startup must use Open.
func New(cfg config.Config, log *slog.Logger) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	return &App{Config: cfg, Log: log, Locks: locks.New(), Realtime: realtime.New(), Events: events.NewBus(), Health: health.NewRegistry(), Metrics: &telemetry.Metrics{}, started: time.Now().UTC()}, nil
}

// Open initializes durable dependencies in dependency order and returns only
// after migrations have completed. Partial startup is cleaned up on failure.
func Open(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	a, err := New(cfg, log)
	if err != nil {
		return nil, err
	}
	db, err := database.Open(ctx, database.Options{Path: cfg.Database.Path, BusyTimeout: cfg.Database.BusyTimeout})
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	a.DB = db
	a.Health.Set("database", health.Healthy, "", "")
	a.Health.Set("sentinel", health.Healthy, "", "")
	a.runCtx, a.runCancel = context.WithCancel(context.Background())
	a.Outbox = events.Outbox{DB: db}
	a.Revisions = repository.NewRevisionStore(db)
	a.Audit = repository.NewAuditStore(db)
	a.Users = auth.NewUserStore(db)
	a.Sessions = auth.NewSessionStore(db, cfg.Security.SessionTTL)
	a.Hardware = hardware.Collect(ctx, hardware.ExecRunner{Timeout: 3 * time.Second})
	a.HardwareRecommendation = hardware.Recommend(a.Hardware)
	guard, err := netpolicy.New(cfg.Network.CameraCIDRs)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	secretRoot := filepath.Join(filepath.Dir(cfg.Database.Path), "secrets")
	a.Secrets = secrets.Resolver{File: secrets.FileProvider{Root: secretRoot}}
	a.CallbackSecurity, err = openCallbackSecurity(cfg.Security.Callbacks, a.Secrets)
	if err != nil {
		_ = a.Close()
		return nil, err
	}
	secretStore := secrets.FileStore{Root: secretRoot}
	a.Cameras = &cameras.Service{Store: repository.NewStore[cameras.Camera](db, repository.KindCamera), Secrets: a.Secrets, Network: guard}
	a.HomeAssistantSetup = &setupsvc.HomeAssistantSetup{Store: repository.NewStore[setupsvc.HomeAssistantDesired](db, repository.KindIntegration), Secrets: secretStore}
	a.Intercom = &intercom.Service{Devices: repository.NewStore[intercom.Device](db, repository.KindIntercom), States: repository.NewStore[intercom.ObservedState](db, repository.KindIntercomState), Commands: intercom.CommandStore{DB: db}, Events: a.Events, Audit: a.Audit, Locks: a.Locks, Cameras: a.Cameras}
	a.Incidents = incidents.NewService(a.Events, repository.NewStore[events.Envelope](db, repository.KindEvent), repository.NewStore[incidents.Incident](db, repository.KindIncident))
	a.Search = &searchsvc.Service{Cameras: a.Cameras, Incidents: a.Incidents, Intercom: a.Intercom}
	if err := a.Incidents.Start(a.runCtx); err != nil {
		_ = a.Close()
		return nil, fmt.Errorf("start incident service: %w", err)
	}
	a.AIPolicies = ai.PolicyStore{Store: repository.NewStore[ai.PrivacyPolicy](db, repository.KindPolicy), ProviderLocal: true}
	if cfg.Frigate.Enabled {
		var token string
		if cfg.Frigate.TokenRef != "" {
			b, resolveErr := a.Secrets.Resolve(cfg.Frigate.TokenRef)
			if resolveErr != nil {
				_ = db.Close()
				return nil, fmt.Errorf("resolve Frigate token: %w", resolveErr)
			}
			token = string(b)
		}
		client, clientErr := frigateint.NewClient(frigateint.ClientOptions{BaseURL: cfg.Frigate.URL, BearerToken: token})
		if clientErr != nil {
			_ = db.Close()
			return nil, clientErr
		}
		a.Health.Set("frigate", health.Starting, "NOT_VERIFIED", "Frigate configured; readiness not yet verified")
		a.Frigate = &frigateint.Service{Client: client, Cameras: a.Cameras, Secrets: a.Secrets, Hardware: a.HardwareRecommendation, State: repository.NewStore[frigateint.AppliedState](db, repository.KindIntegration), Locks: a.Locks, SecretSink: frigateint.CredentialDirectorySink{Dir: cfg.Frigate.CredentialsDirectory}, WebRTCCandidates: append([]string(nil), cfg.Frigate.WebRTCCandidates...)}
	}
	if cfg.MQTT.Enabled {
		var password []byte
		if cfg.MQTT.PasswordRef != "" {
			password, err = a.Secrets.Resolve(cfg.MQTT.PasswordRef)
			if err != nil {
				_ = a.Close()
				return nil, fmt.Errorf("resolve MQTT password: %w", err)
			}
		}
		client, mqttErr := mqttint.NewClient(a.runCtx, mqttint.Options{
			URL:            cfg.MQTT.URL,
			ClientID:       cfg.MQTT.ClientID,
			Username:       cfg.MQTT.Username,
			Password:       password,
			KeepAlive:      cfg.MQTT.KeepAlive,
			SessionExpiry:  cfg.MQTT.SessionExpiry,
			ConnectTimeout: cfg.MQTT.ConnectTimeout,
			Subscriptions: []mqttint.Subscription{
				{Topic: mqttint.FrigateReviews, QoS: 1},
				{Topic: mqttint.FrigateTrackedObjectUpdate, QoS: 1},
				{Topic: mqttint.HomeAssistantStatus, QoS: 1},
				{Topic: "sentinel/intercom/+/state/+", QoS: 1},
				{Topic: "sentinel/intercom/+/event/+", QoS: 1},
			},
			Handler: a.ingestMQTT,
			OnConnectionStateChange: func(up bool) {
				if up {
					a.Health.Set("mqtt", health.Healthy, "", "")
					go a.publishHADiscovery(a.runCtx)
				} else {
					a.Health.Set("mqtt", health.Degraded, "MQTT_DISCONNECTED", "broker connection unavailable")
				}
			},
		})
		for i := range password {
			password[i] = 0
		}
		if mqttErr != nil {
			_ = a.Close()
			return nil, fmt.Errorf("initialize MQTT: %w", mqttErr)
		}
		a.MQTT = client
		if a.Intercom != nil {
			a.Intercom.MQTT = client
		}
	}
	if cfg.HomeAssistant.Enabled {
		if err := a.StartHomeAssistant(a.runCtx, cfg.HomeAssistant.URL, cfg.HomeAssistant.TokenRef); err != nil {
			a.Health.Set("home_assistant", health.Degraded, "HA_UNAVAILABLE", "initial connection failed")
			a.Log.WarnContext(ctx, "Home Assistant unavailable; continuing in degraded mode", "error", err)
		} else {
			a.Health.Set("home_assistant", health.Healthy, "", "")
		}
	}
	if cfg.AI.Enabled {
		provider, err := ollama.New(ollama.Options{BaseURL: cfg.AI.URL, Model: cfg.AI.Model})
		if err != nil {
			_ = a.Close()
			return nil, fmt.Errorf("initialize Ollama: %w", err)
		}
		profile := ai.Recommend(a.Hardware)
		if profile.MaxParallel < 1 {
			profile.Level = "FORCED"
			profile.MaxParallel = 1
			profile.MaxFrames = 2
			profile.Warnings = append(profile.Warnings, "AI was explicitly enabled despite conservative hardware recommendation")
		}
		a.AI, err = ai.NewService(a.runCtx, provider, true, profile, 32)
		if err != nil {
			_ = a.Close()
			return nil, fmt.Errorf("initialize AI service: %w", err)
		}
		a.Health.Set("ai", health.Starting, "NOT_VERIFIED", "AI runtime configured; model health not yet verified")
	}
	if cfg.Backup.Enabled {
		password, resolveErr := a.Secrets.Resolve(cfg.Backup.PasswordRef)
		if resolveErr != nil {
			_ = a.Close()
			return nil, fmt.Errorf("resolve backup password: %w", resolveErr)
		}
		client := &resticint.Client{Repository: cfg.Backup.Repository, Password: append([]byte(nil), password...)}
		for i := range password {
			password[i] = 0
		}
		configFiles := append([]string(nil), cfg.Backup.ConfigFiles...)
		if source := strings.TrimSpace(os.Getenv("SENTINEL_CONFIG")); source != "" {
			configFiles = append(configFiles, source)
		}
		excluded := managedFileSecretNames(cfg.Backup.PasswordRef)
		a.BackupRestic = client
		a.Backup = &backup.Manager{DB: db, Restic: client, Jobs: repository.NewStore[backup.JobRecord](db, repository.KindBackupJob), ConfigFiles: dedupeStrings(configFiles), SecretRoot: secretRoot, ExcludedSecretNames: excluded, OnResult: func(operation string, opErr error) {
			if opErr != nil {
				a.Health.Set("backup", health.Degraded, "BACKUP_"+strings.ToUpper(operation)+"_FAILED", "backup operation failed")
				return
			}
			a.Health.Set("backup", health.Healthy, "", "")
		}}
		a.BackupScheduler = &backup.Scheduler{Manager: a.Backup, Schedule: backup.Schedule{Interval: cfg.Backup.Interval}}
		a.Health.Set("backup", health.Starting, "NOT_VERIFIED", "backup repository configured; integrity not yet verified")
		if err := a.BackupScheduler.Start(a.runCtx); err != nil {
			_ = a.Close()
			return nil, fmt.Errorf("start backup scheduler: %w", err)
		}
	}
	if cfg.Telegram.Enabled {
		tokenBytes, resolveErr := a.Secrets.Resolve(cfg.Telegram.TokenRef)
		if resolveErr != nil {
			_ = a.Close()
			return nil, fmt.Errorf("resolve Telegram token: %w", resolveErr)
		}
		token := string(tokenBytes)
		for i := range tokenBytes {
			tokenBytes[i] = 0
		}
		client, clientErr := tgapi.New(tgapi.Options{Token: token})
		if clientErr != nil {
			_ = a.Close()
			return nil, fmt.Errorf("initialize Telegram client: %w", clientErr)
		}
		a.Telegram = &tgsvc.Service{Client: client, Pairings: tgsvc.PairingStore{DB: db}, Actions: tgsvc.ActionStore{DB: db}, Users: a.Users, Intercom: a.Intercom, Events: a.Events}
		a.Health.Set("telegram", health.Starting, "NOT_VERIFIED", "Telegram worker configured")
		if err := a.Telegram.Start(a.runCtx); err != nil {
			_ = a.Close()
			return nil, fmt.Errorf("start Telegram service: %w", err)
		}
	}
	if err := a.startIncidentRuntime(); err != nil {
		_ = a.Close()
		return nil, err
	}
	a.startWatchdog()
	return a, nil
}

func (a *App) startWatchdog() {
	checks := []watchdog.Check{{Name: "database", Base: 15 * time.Second, MaxBackoff: time.Minute, Timeout: 2 * time.Second, ReasonCode: "DATABASE_UNAVAILABLE", Probe: func(ctx context.Context) error { return a.DB.PingContext(ctx) }}}
	if a.MQTT != nil {
		checks = append(checks, watchdog.Check{Name: "mqtt", Base: 10 * time.Second, MaxBackoff: time.Minute, Timeout: time.Second, ReasonCode: "MQTT_DISCONNECTED", Probe: func(context.Context) error {
			if !a.MQTT.Ready() {
				return errors.New("MQTT disconnected")
			}
			return nil
		}})
	}
	if a.HomeAssistant != nil {
		checks = append(checks, watchdog.Check{Name: "home_assistant", Base: 30 * time.Second, MaxBackoff: 3 * time.Minute, Timeout: 5 * time.Second, ReasonCode: "HA_UNAVAILABLE", Probe: func(ctx context.Context) error {
			st := a.HomeAssistant.Status(ctx)
			if !st.Reachable {
				return errors.New("Home Assistant unreachable")
			}
			return nil
		}})
	}
	if a.Frigate != nil && a.Frigate.Client != nil {
		checks = append(checks, watchdog.Check{Name: "frigate", Base: 30 * time.Second, MaxBackoff: 3 * time.Minute, Timeout: 5 * time.Second, ReasonCode: "FRIGATE_UNAVAILABLE", Probe: func(ctx context.Context) error { _, err := a.Frigate.Client.Version(ctx); return err }})
	}
	if a.AI != nil && a.AI.Provider != nil {
		checks = append(checks, watchdog.Check{Name: "ai", Base: time.Minute, MaxBackoff: 5 * time.Minute, Timeout: 10 * time.Second, ReasonCode: "AI_UNAVAILABLE", Probe: func(ctx context.Context) error {
			h := a.AI.Provider.Health(ctx)
			if !h.Reachable {
				return errors.New("AI provider unreachable")
			}
			return nil
		}})
	}
	if a.Telegram != nil && a.Telegram.Client != nil {
		checks = append(checks, watchdog.Check{Name: "telegram", Base: time.Minute, MaxBackoff: 5 * time.Minute, Timeout: 5 * time.Second, ReasonCode: "TELEGRAM_UNAVAILABLE", Probe: func(ctx context.Context) error { _, err := a.Telegram.Client.GetMe(ctx); return err }})
	}
	m := &watchdog.Manager{Registry: a.Health, Checks: checks}
	if err := m.Start(a.runCtx); err != nil {
		a.Log.Warn("watchdog disabled", "error", err)
		return
	}
	a.Watchdog = m
}

func (a *App) StartedAt() time.Time { return a.started }
func (a *App) Ready(ctx context.Context) bool {
	if a.DB == nil {
		return true
	}
	c, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	return a.DB.PingContext(c) == nil
}
func (a *App) Close() error {
	if a.runCancel != nil {
		a.runCancel()
		a.runCancel = nil
	}
	var first error
	if a.Watchdog != nil {
		a.Watchdog.Close()
		a.Watchdog = nil
	}
	if a.IncidentRuntime != nil {
		if err := a.IncidentRuntime.Close(); err != nil {
			first = err
		}
		a.IncidentRuntime = nil
	}
	if a.Incidents != nil {
		a.Incidents.Close()
		a.Incidents = nil
	}
	if a.BackupScheduler != nil {
		a.BackupScheduler.Close()
		a.BackupScheduler = nil
	}
	if a.Telegram != nil {
		a.Telegram.Close()
		a.Telegram = nil
	}
	if a.BackupRestic != nil {
		a.BackupRestic.WipePassword()
		a.BackupRestic = nil
	}
	a.Backup = nil
	if a.AI != nil {
		a.AI.Close()
		a.AI = nil
	}
	if a.HomeAssistant != nil {
		if err := a.HomeAssistant.Close(); err != nil {
			first = err
		}
		a.HomeAssistant = nil
	}
	if a.MQTT != nil {
		if err := a.MQTT.Close(); err != nil {
			first = err
		}
		a.MQTT = nil
	}
	if a.Events != nil {
		a.Events.Close()
	}
	if a.DB != nil {
		if err := a.DB.Close(); err != nil && first == nil {
			first = err
		}
		a.DB = nil
	}
	return first
}
func (a *App) ingestMQTT(ctx context.Context, msg mqttint.Message) {
	var (
		e   events.Envelope
		err error
	)
	if strings.HasPrefix(msg.Topic, "sentinel/intercom/") {
		if a.Intercom == nil {
			return
		}
		if err := a.Intercom.IngestMQTT(ctx, msg); err != nil {
			a.Log.WarnContext(ctx, "discarding invalid intercom MQTT message", "topic", msg.Topic, "error", err)
		}
		return
	}
	switch msg.Topic {
	case mqttint.HomeAssistantStatus:
		if strings.EqualFold(strings.TrimSpace(string(msg.Payload)), "online") {
			go a.publishHADiscovery(a.runCtx)
		}
		return
	case mqttint.FrigateReviews:
		e, err = eventadapters.FrigateReview(msg.Payload)
	case mqttint.FrigateTrackedObjectUpdate:
		e, err = eventadapters.FrigateTrackedObjectUpdate(msg.Payload)
	default:
		return
	}
	if err != nil {
		a.Log.WarnContext(ctx, "discarding invalid MQTT event", "topic", msg.Topic, "error", err)
		return
	}
	dropped := a.Events.Publish(e)
	if dropped > 0 {
		a.Log.WarnContext(ctx, "event bus subscriber overflow", "event_type", e.Type, "dropped_subscribers", dropped)
	}
}

func (a *App) StartHomeAssistant(ctx context.Context, rawURL string, tokenRef secrets.Ref) error {
	if strings.TrimSpace(rawURL) == "" || tokenRef == "" {
		return errors.New("Home Assistant URL and token reference required")
	}
	secret, err := a.Secrets.Resolve(tokenRef)
	if err != nil {
		return fmt.Errorf("resolve Home Assistant token: %w", err)
	}
	token := string(secret)
	for i := range secret {
		secret[i] = 0
	}
	rest, err := haint.NewRESTClient(haint.RESTOptions{BaseURL: rawURL, Token: token})
	if err != nil {
		return err
	}
	var stream *haint.WSStream
	stream, err = haint.NewWSStream(a.runCtx, haint.WSOptions{BaseURL: rawURL, Token: token, EventTypes: []string{"state_changed"}})
	if err != nil && !errors.Is(err, haint.ErrWebSocketRuntimeUnsupported) {
		return err
	}
	discovery := haint.DiscoveryPublisher{Prefix: "homeassistant"}
	if a.MQTT != nil {
		discovery.MQTT = a.MQTT
	}
	service := &haint.Service{REST: rest, Stream: stream, Discovery: discovery}
	service.Actions, err = haint.NewActionBridge(rest, nil)
	if err != nil {
		_ = service.Close()
		return err
	}
	if a.HomeAssistant != nil {
		_ = a.HomeAssistant.Close()
	}
	a.HomeAssistant = service
	if a.MQTT != nil && a.MQTT.Ready() {
		go a.publishHADiscovery(a.runCtx)
	}
	return nil
}

func (a *App) ConfigureHomeAssistant(ctx context.Context, rawURL, token string) (setupsvc.HomeAssistantDesired, setupsvc.HomeAssistantDiagnostic, error) {
	if a.HomeAssistantSetup == nil {
		return setupsvc.HomeAssistantDesired{}, setupsvc.HomeAssistantDiagnostic{}, errors.New("Home Assistant setup unavailable")
	}
	desired, diag, err := a.HomeAssistantSetup.Configure(ctx, rawURL, token)
	if err != nil || !diag.OK {
		return desired, diag, err
	}
	if err := a.StartHomeAssistant(ctx, desired.URL, desired.TokenRef); err != nil {
		return desired, diag, fmt.Errorf("activate Home Assistant: %w", err)
	}
	a.Config.HomeAssistant = config.HomeAssistantConfig{Enabled: true, URL: desired.URL, TokenRef: desired.TokenRef}
	return desired, diag, nil
}

func (a *App) publishHADiscovery(ctx context.Context) {
	if a.HomeAssistant == nil || a.HomeAssistant.Discovery.MQTT == nil || ctx == nil || ctx.Err() != nil {
		return
	}
	if err := a.HomeAssistant.PublishSystemDiscovery(ctx, buildinfo.Version); err != nil {
		a.Log.WarnContext(ctx, "publish Home Assistant system discovery", "error", err)
		return
	}
	_ = a.HomeAssistant.Discovery.PublishAvailability(ctx, "sentinel/system/availability", true)
	aiStatus := "DISABLED"
	if a.Config.AI.Enabled {
		aiStatus = "CONFIGURED"
	}
	backupStatus := "DISABLED"
	if a.Config.Backup.Enabled {
		backupStatus = "CONFIGURED"
	}
	_ = a.HomeAssistant.Discovery.PublishSystemState(ctx, haint.SystemState{Health: "HEALTHY", AIStatus: aiStatus, BackupStatus: backupStatus})
	if a.Cameras == nil {
		return
	}
	cams, err := a.Cameras.List(ctx, 1000)
	if err != nil {
		a.Log.WarnContext(ctx, "list cameras for Home Assistant discovery", "error", err)
		return
	}
	for _, cam := range cams {
		device, err := haint.SentinelCameraDevice(cam.ID, cam.Name, buildinfo.Version)
		if err != nil {
			continue
		}
		if err := a.HomeAssistant.Discovery.PublishDevice(ctx, "camera_"+cam.ID, device); err != nil {
			a.Log.WarnContext(ctx, "publish Home Assistant camera discovery", "camera_id", cam.ID, "error", err)
			continue
		}
		online := strings.EqualFold(cam.Observed.Status, "HEALTHY")
		_ = a.HomeAssistant.Discovery.PublishCameraConnectivity(ctx, cam.ID, online)
	}
	if a.Intercom != nil {
		devices, err := a.Intercom.List(ctx, 1000)
		if err == nil {
			for _, dev := range devices {
				discovered, err := haint.SentinelIntercomDevice(dev.ID, dev.Name, buildinfo.Version)
				if err != nil {
					continue
				}
				if err := a.HomeAssistant.Discovery.PublishDevice(ctx, "intercom_"+dev.ID, discovered); err != nil {
					a.Log.WarnContext(ctx, "publish Home Assistant intercom discovery", "intercom_id", dev.ID, "error", err)
				}
			}
		}
	}
}

func (a *App) RequireDB() (*sql.DB, error) {
	if a.DB == nil {
		return nil, errors.New("application database is not initialized")
	}
	return a.DB, nil
}

func managedFileSecretNames(ref secrets.Ref) []string {
	const prefix = "secret://file/"
	raw := ref.String()
	if !strings.HasPrefix(raw, prefix) {
		return nil
	}
	name := strings.TrimPrefix(raw, prefix)
	if name == "" || strings.Contains(name, "/") {
		return nil
	}
	return []string{name}
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
