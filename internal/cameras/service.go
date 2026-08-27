package cameras

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras/rtsp"
	"github.com/Homiakus/Home_Sentinel/internal/cameras/uvc"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
	"github.com/Homiakus/Home_Sentinel/internal/security/netpolicy"
)

type Service struct {
	Store     *repository.Store[Camera]
	Secrets   CredentialResolver
	Network   netpolicy.Guard
	RTSPProbe func(context.Context, string, time.Duration) (rtsp.Result, error)
}
type RTSPOnboardRequest struct {
	Name        string
	URL         string
	Username    string
	PasswordRef secrets.Ref
	Role        StreamRole
}
type RTSPProbeResult struct {
	PinnedURL string      `json:"-"`
	Probe     rtsp.Result `json:"probe"`
}

func (s *Service) ProbeRTSP(ctx context.Context, ep Endpoint) (RTSPProbeResult, error) {
	if s == nil {
		return RTSPProbeResult{}, errors.New("camera service unavailable")
	}
	raw, err := ResolvedURL(ep, s.Secrets)
	if err != nil {
		return RTSPProbeResult{}, err
	}
	pinned, err := s.Network.PinURL(ctx, raw, "rtsp")
	if err != nil {
		return RTSPProbeResult{}, err
	}
	probe := s.RTSPProbe
	if probe == nil {
		probe = rtsp.Probe
	}
	result, err := probe(ctx, pinned, 12*time.Second)
	if err != nil {
		return RTSPProbeResult{}, err
	}
	return RTSPProbeResult{PinnedURL: pinned, Probe: result}, nil
}
func (s *Service) OnboardRTSP(ctx context.Context, in RTSPOnboardRequest) (Camera, error) {
	if s.Store == nil {
		return Camera{}, errors.New("camera store unavailable")
	}
	if in.Role == "" {
		in.Role = RoleMain
	}
	ep := Endpoint{URL: in.URL, Username: in.Username, PasswordRef: in.PasswordRef}
	probed, err := s.ProbeRTSP(ctx, ep)
	if err != nil {
		return Camera{}, fmt.Errorf("probe RTSP camera: %w", err)
	}
	id, err := domain.NewID("cam")
	if err != nil {
		return Camera{}, err
	}
	streamID, err := domain.NewID("str")
	if err != nil {
		return Camera{}, err
	}
	st := Stream{ID: streamID.String(), Role: in.Role, Endpoint: ep}
	if len(probed.Probe.Media.Video) > 0 {
		v := probed.Probe.Media.Video[0]
		st.Codec = v.Codec
		st.Width = v.Width
		st.Height = v.Height
		st.FPS = v.FPS
		st.Bitrate = v.Bitrate
	}
	if len(probed.Probe.Media.Audio) > 0 {
		st.AudioCodec = probed.Probe.Media.Audio[0].Codec
	}
	cam := Camera{ID: id.String(), Name: in.Name, Type: TypeRTSP, Streams: []Stream{st}, Capabilities: Capabilities{Audio: len(probed.Probe.Media.Audio) > 0}, Observed: Health{Status: "HEALTHY", CheckedAt: time.Now().UTC(), Latency: probed.Probe.Media.ProbeLatency}}
	if err := cam.Validate(); err != nil {
		return Camera{}, err
	}
	res, err := s.Store.Put(ctx, cam.ID, cam)
	if err != nil {
		return Camera{}, err
	}
	return res.Value, nil
}
func (s *Service) List(ctx context.Context, limit int) ([]Camera, error) {
	rs, err := s.Store.List(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Camera, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Value)
	}
	return out, nil
}
func (s *Service) Get(ctx context.Context, id string) (Camera, error) {
	r, err := s.Store.Get(ctx, id)
	if err != nil {
		return Camera{}, err
	}
	return r.Value, nil
}

type UVCOnboardRequest struct {
	Name string     `json:"name"`
	Path string     `json:"path"`
	Role StreamRole `json:"role,omitempty"`
}

func (s *Service) DiscoverUVC(ctx context.Context) ([]uvc.Device, error) {
	if s == nil {
		return nil, errors.New("camera service unavailable")
	}
	return uvc.Discover(ctx), nil
}

func (s *Service) OnboardUVC(ctx context.Context, in UVCOnboardRequest) (Camera, error) {
	if s.Store == nil {
		return Camera{}, errors.New("camera store unavailable")
	}
	if strings.TrimSpace(in.Name) == "" {
		return Camera{}, errors.New("camera name required")
	}
	if strings.TrimSpace(in.Path) == "" {
		return Camera{}, errors.New("device path required")
	}
	if in.Role == "" {
		in.Role = RoleMain
	}
	id, err := domain.NewID("cam")
	if err != nil {
		return Camera{}, err
	}
	streamID, err := domain.NewID("str")
	if err != nil {
		return Camera{}, err
	}
	ep := Endpoint{URL: in.Path}
	st := Stream{ID: streamID.String(), Role: in.Role, Endpoint: ep, Codec: "mjpeg", Width: 640, Height: 480, FPS: 30}
	start := time.Now()
	shot, snapErr := Snapshot(ctx, in.Path, 5*time.Second)
	latency := time.Since(start)
	status := "HEALTHY"
	if snapErr != nil {
		status = "DEGRADED"
	}
	_ = shot
	cam := Camera{
		ID:           id.String(),
		Name:         in.Name,
		Type:         TypeUVC,
		Streams:      []Stream{st},
		Capabilities: Capabilities{Snapshot: snapErr == nil},
		Observed:     Health{Status: status, CheckedAt: time.Now().UTC(), Latency: latency},
	}
	if err := cam.Validate(); err != nil {
		return Camera{}, err
	}
	res, err := s.Store.Put(ctx, cam.ID, cam)
	if err != nil {
		return Camera{}, err
	}
	return res.Value, nil
}
