package frigate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/hardware"
	fgconfig "github.com/Homiakus/Home_Sentinel/internal/integrations/frigate/config"
	"github.com/Homiakus/Home_Sentinel/internal/integrations/go2rtc"
	"github.com/Homiakus/Home_Sentinel/internal/locks"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

const integrationStateID = "frigate"

type AppliedState struct {
	Ownership      fgconfig.Ownership `json:"ownership"`
	ConfigChecksum string             `json:"config_checksum"`
	FrigateVersion string             `json:"frigate_version"`
	AppliedAt      time.Time          `json:"applied_at"`
}
type Plan struct {
	Version    string             `json:"version"`
	ConfigJSON []byte             `json:"config_json"`
	Ownership  fgconfig.Ownership `json:"ownership"`
	SecretEnv  map[string]string  `json:"-"`
	Checksum   string             `json:"checksum"`
	Preflight  PreflightReport    `json:"preflight"`
}
type Service struct {
	Client           *Client
	Cameras          *cameras.Service
	Secrets          go2rtc.SecretResolver
	Hardware         hardware.Recommendation
	State            *repository.Store[AppliedState]
	Locks            *locks.Manager
	SecretSink       SecretEnvSink
	WebRTCCandidates []string
}

func (s *Service) Capabilities(ctx context.Context) (Capabilities, CapabilityDiagnostic, error) {
	if s == nil || s.Client == nil {
		return Capabilities{}, CapabilityDiagnostic{}, errors.New("Frigate integration disabled")
	}
	return ProbeCapabilities(ctx, s.Client)
}
func (s *Service) currentState(ctx context.Context) AppliedState {
	if s.State == nil {
		return AppliedState{}
	}
	r, err := s.State.Get(ctx, integrationStateID)
	if err != nil {
		return AppliedState{}
	}
	return r.Value
}
func (s *Service) Plan(ctx context.Context) (Plan, error) {
	var p Plan
	if s == nil || s.Client == nil || s.Cameras == nil {
		return p, errors.New("Frigate service unavailable")
	}
	caps, diag, err := ProbeCapabilities(ctx, s.Client)
	if err != nil {
		return p, err
	}
	if !diag.Compatible {
		return p, fmt.Errorf("Frigate %s incompatible: %v", caps.Version, diag.Reasons)
	}
	cams, err := s.Cameras.List(ctx, 1000)
	if err != nil {
		return p, err
	}
	built, err := BuildManaged(ctx, cams, s.Hardware, s.Secrets, nil)
	if err != nil {
		return p, err
	}
	if len(s.WebRTCCandidates) > 0 {
		built.Managed.Go2RTC.WebRTC = &fgconfig.WebRTC{Candidates: append([]string(nil), s.WebRTCCandidates...)}
		built.Ownership.ManageGo2RTCWebRTC = true
	}
	actual, err := s.Client.Config(ctx)
	if err != nil {
		return p, err
	}
	prev := s.currentState(ctx).Ownership
	rendered, err := fgconfig.Render(actual, built.Managed, prev, built.Ownership)
	if err != nil {
		return p, err
	}
	report, err := Preflight(ctx, s.Client, caps, rendered, built.Ownership, managedMediaVerifier{service: s.Cameras, cameras: cams})
	if err != nil {
		return p, err
	}
	h := sha256.Sum256(rendered)
	p = Plan{Version: caps.Version, ConfigJSON: rendered, Ownership: built.Ownership, SecretEnv: built.SecretEnv, Checksum: hex.EncodeToString(h[:]), Preflight: report}
	return p, nil
}
func (s *Service) Apply(ctx context.Context) (ApplyResult, error) {
	release := func() {}
	if s.Locks != nil {
		release = s.Locks.Lock("integration/frigate/config")
	}
	defer release()
	plan, err := s.Plan(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	names := append([]string(nil), plan.Ownership.StreamNames...)
	sort.Strings(names)
	result, err := (Applier{Control: s.Client, Secrets: s.SecretSink}).Apply(ctx, ApplyRequest{ConfigJSON: plan.ConfigJSON, SecretEnv: plan.SecretEnv, ExpectedStreams: names, ReadyTimeout: 45 * time.Second})
	if err != nil {
		return result, err
	}
	if s.State != nil {
		_, storeErr := s.State.Put(ctx, integrationStateID, AppliedState{Ownership: plan.Ownership, ConfigChecksum: plan.Checksum, FrigateVersion: result.Version, AppliedAt: time.Now().UTC()})
		if storeErr != nil {
			return result, fmt.Errorf("Frigate applied but state persistence failed: %w", storeErr)
		}
	}
	return result, nil
}
func (s *Service) Reconcile(ctx context.Context) (ReconcileReport, error) {
	plan, err := s.Plan(ctx)
	if err != nil {
		return ReconcileReport{}, err
	}
	actual, err := s.Client.Config(ctx)
	if err != nil {
		return ReconcileReport{}, err
	}
	return Reconcile(plan.ConfigJSON, actual, plan.Ownership), nil
}
func (s *Service) Events() EventService { return EventService{Client: s.Client} }

type managedMediaVerifier struct {
	service *cameras.Service
	cameras []cameras.Camera
}

func (v managedMediaVerifier) VerifyManagedMedia(ctx context.Context) error {
	for _, cam := range v.cameras {
		for _, st := range cam.Streams {
			if st.Endpoint.URL == "" {
				continue
			}
			switch cam.Type {
			case cameras.TypeRTSP, cameras.TypeONVIF:
				if _, err := v.service.ProbeRTSP(ctx, st.Endpoint); err != nil {
					return fmt.Errorf("camera %s stream %s: %w", cam.ID, st.ID, err)
				}
			}
		}
	}
	return nil
}

func (p Plan) MarshalPublicJSON() ([]byte, error) {
	type public struct {
		Version   string             `json:"version"`
		Ownership fgconfig.Ownership `json:"ownership"`
		Checksum  string             `json:"checksum"`
		Preflight PreflightReport    `json:"preflight"`
	}
	return json.Marshal(public{p.Version, p.Ownership, p.Checksum, p.Preflight})
}
