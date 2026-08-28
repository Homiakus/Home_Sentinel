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
	CallbackSecurity       CallbackSecurity
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
			OnMessage:      a.ingestMQTT,
		})
		if mqttErr != nil {
			_ = a.Close()
			return nil, mqttErr
		}
		a.MQTT = client
		a.Health.Set("mqtt", health.Starting, "NOT_VERIFIED", "MQTT configured; readiness not yet verified")
	}
	if cfg.HomeAssistant.Enabled {
		token, resolveErr := a.Secrets.Resolve(cfg.HomeAssistant.TokenRef)
		if resolveErr != nil {
			_ = a.Close()
			return nil, fmt.Errorf("resolve Home Assistant token: %w", resolveErr)
		}
		client, clientErr := haint.NewClient(haint.ClientOptions{BaseURL: cfg.HomeAssistant.URL, BearerToken: string(token)})
		if clientErr != nil {
			_ = a.Close()
			return nil, clientErr
		}
		a.HomeAssistant = &haint.Service{Client: client, State: repository.NewStore[haint.AppliedState](db, repository.KindIntegration), Locks: a.Locks}
		a.Health.Set("home_assistant", health.Starting, "NOT_VERIFIED", "Home Assistant configured; readiness not yet verified")
	}
	if cfg.AI.Enabled {
		client, clientErr := ollama.NewClient(ollama.ClientOptions{BaseURL: cfg.AI.URL, Model: cfg.AI.Model})
		if clientErr != nil {
			_ = a.Close()
			return nil, clientErr
		}
		a.AI = ai.NewService(client, a.AIPolicies, a.Events)
		a.Health.Set("ai", health.Starting, "NOT_VERIFIED", "AI configured; readiness not yet verified")
	}
	if cfg.Backup.Enabled {
		manager, managerErr := backup.NewManager(db, backup.Options{
			DataDir:     filepath.Dir(cfg.Database.Path),
			ConfigFiles: append([]string(nil), cfg.Backup.ConfigFiles...),
		})
		if managerErr != nil {
			_ = a.Close()
			return nil, managerErr
		}
		a.Backup = manager
		if cfg.Backup.Repository != "" && cfg.Backup.PasswordRef != "" {
			secret, resolveErr := a.Secrets.Resolve(cfg.Backup.PasswordRef)
			if resolveErr != nil {
				_ = a.Close()
				return nil, fmt.Errorf("resolve restic password: %w", resolveErr)
			}
			restic, resticErr := resticint.New(resticint.Options{
				Repository: cfg.Backup.Repository,
				Password:   secret,
			})
			for i := range secret {
				secret[i] = 0
			}
			if resticErr != nil {
				_ = a.Close()
				return nil, resticErr
			}
			a.BackupRestic = restic
			a.Health.Set("backup_remote", health.Starting, "NOT_VERIFIED", "restic configured; restore has not yet been verified")
		}
		a.BackupScheduler = backup.NewScheduler(manager, cfg.Backup.Interval, backup.RetentionPolicy{KeepHourly: cfg.Backup.KeepHourly, KeepDaily: cfg.Backup.KeepDaily, KeepWeekly: cfg.Backup.KeepWeekly, KeepMonthly: cfg.Backup.KeepMonthly})
		a.BackupScheduler.Start(a.runCtx)
	}
	callbackSecurity, err := openCallbackSecurity(a.Secrets, cfg.Security.Callbacks)
	if err != nil {
		_ = a.Close()
		return nil, fmt.Errorf("initialize callback security: %w", err)
	}
	a.CallbackSecurity = callbackSecurity
	if cfg.Telegram.Enabled {
		if cfg.Telegram.TokenRef == "" {
			_ = a.Close()
			return nil, errors.New("Telegram token reference required")
		}
		token, resolveErr := a.Secrets.Resolve(cfg.Telegram.TokenRef)
		if resolveErr != nil {
			_ = a.Close()
			return nil, fmt.Errorf("resolve Telegram token: %w", resolveErr)
		}
		client := tgapi.NewClient(string(token))
		for i := range token {
			token[i] = 0
		}
		a.Telegram = &tgsvc.Service{Client: client, Pairings: tgsvc.PairingStore{DB: db}, Actions: tgsvc.ActionStore{DB: db}, Users: a.Users, Intercom: a.Intercom, Events: a.Events}
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
	if a == nil || a.Health == nil || a.runCtx == nil || a.Config.Watchdog.Interval <= 0 {
		return
	}
	checks := []watchdog.Check{
		{Name: "database", Timeout: 2 * time.Second, Run: func(ctx context.Context) error { return a.DB.PingContext(ctx) }},
		{Name: "sentinel", Timeout: time.Second, Run: func(context.Context) error { return nil }},
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
		return err
	}
	client, err := haint.NewClient(haint.ClientOptions{BaseURL: rawURL, BearerToken: string(secret)})
	if err != nil {
		return err
	}
	if a.HomeAssistant != nil {
		_ = a.HomeAssistant.Close()
	}
	a.HomeAssistant = &haint.Service{Client: client, State: repository.NewStore[haint.AppliedState](a.DB, repository.KindIntegration), Locks: a.Locks}
	return nil
}

func (a *App) WriteConfigRevision(ctx context.Context, actor, requestID, correlationID string, cfg any) (repository.Revision, error) {
	b, err := jsonMarshalStable(cfg)
	if err != nil {
		return repository.Revision{}, err
	}
	return a.Revisions.Apply(ctx, "config", "sentinel", actor, requestID, correlationID, b, func(context.Context) error { return nil })
}

func (a *App) Version() buildinfo.Info { return buildinfo.Current() }

func jsonMarshalStable(v any) ([]byte, error) {
	return json.Marshal(v)
}
