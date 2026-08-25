package setup

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
	ha "github.com/Homiakus/Home_Sentinel/internal/integrations/homeassistant"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

const homeAssistantDesiredID = "home_assistant_desired"

type HomeAssistantDesired struct {
	Enabled         bool        `json:"enabled"`
	URL             string      `json:"url"`
	TokenRef        secrets.Ref `json:"token_ref"`
	VerifiedVersion string      `json:"verified_version"`
	ConfiguredAt    time.Time   `json:"configured_at"`
}

type HomeAssistantDiagnostic struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Version string `json:"version,omitempty"`
}

type HomeAssistantSetup struct {
	Store      *repository.Store[HomeAssistantDesired]
	Secrets    secrets.FileStore
	HTTPClient *http.Client
}

func (s *HomeAssistantSetup) Probe(ctx context.Context, rawURL, token string) HomeAssistantDiagnostic {
	client, err := ha.NewRESTClient(ha.RESTOptions{BaseURL: rawURL, Token: token, HTTPClient: s.HTTPClient})
	if err != nil {
		return HomeAssistantDiagnostic{Code: "INVALID_INPUT", Message: err.Error()}
	}
	if _, err := client.Ping(ctx); err != nil {
		var he *ha.HTTPError
		if errors.As(err, &he) && he.Status == http.StatusUnauthorized {
			return HomeAssistantDiagnostic{Code: "AUTH_FAILED", Message: "Home Assistant rejected the access token."}
		}
		return HomeAssistantDiagnostic{Code: "CONNECTION_FAILED", Message: fmt.Sprintf("Home Assistant API is unreachable: %v", err)}
	}
	cfg, err := client.Config(ctx)
	if err != nil {
		return HomeAssistantDiagnostic{Code: "CAPABILITY_CHECK_FAILED", Message: fmt.Sprintf("Home Assistant API is reachable, but config capability check failed: %v", err)}
	}
	return HomeAssistantDiagnostic{OK: true, Code: "OK", Message: "Home Assistant API verified.", Version: cfg.Version}
}

func (s *HomeAssistantSetup) Configure(ctx context.Context, rawURL, token string) (HomeAssistantDesired, HomeAssistantDiagnostic, error) {
	if s == nil || s.Store == nil {
		return HomeAssistantDesired{}, HomeAssistantDiagnostic{}, errors.New("Home Assistant setup store unavailable")
	}
	diag := s.Probe(ctx, rawURL, token)
	if !diag.OK {
		return HomeAssistantDesired{}, diag, nil
	}
	secretID, err := domain.NewID("sec")
	if err != nil {
		return HomeAssistantDesired{}, diag, err
	}
	name := "ha-token-" + secretID.String()
	old, oldErr := s.Get(ctx)
	if oldErr != nil && !errors.Is(oldErr, repository.ErrNotFound) {
		return HomeAssistantDesired{}, diag, oldErr
	}
	ref, err := s.Secrets.Put(name, []byte(token))
	if err != nil {
		return HomeAssistantDesired{}, diag, err
	}
	desired := HomeAssistantDesired{Enabled: true, URL: rawURL, TokenRef: ref, VerifiedVersion: diag.Version, ConfiguredAt: time.Now().UTC()}
	if _, err := s.Store.Put(ctx, homeAssistantDesiredID, desired); err != nil {
		_ = s.Secrets.Delete(name)
		return HomeAssistantDesired{}, diag, err
	}
	if oldErr == nil && old.TokenRef != "" && old.TokenRef != ref {
		_ = s.Secrets.DeleteRef(old.TokenRef)
	}
	return desired, diag, nil
}

func (s *HomeAssistantSetup) Get(ctx context.Context) (HomeAssistantDesired, error) {
	if s == nil || s.Store == nil {
		return HomeAssistantDesired{}, errors.New("Home Assistant setup store unavailable")
	}
	r, err := s.Store.Get(ctx, homeAssistantDesiredID)
	return r.Value, err
}
