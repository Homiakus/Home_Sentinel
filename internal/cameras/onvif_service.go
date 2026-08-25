package cameras

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras/discovery"
	"github.com/Homiakus/Home_Sentinel/internal/cameras/onvif"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

type ONVIFOnboardRequest struct {
	Name        string
	DeviceURL   string
	Username    string
	PasswordRef secrets.Ref
}

func (s *Service) DiscoverONVIF(ctx context.Context, duration time.Duration) ([]discovery.Candidate, error) {
	if s == nil {
		return nil, errors.New("camera service unavailable")
	}
	return discovery.Scan(ctx, s.Network.Allowed, duration)
}

func (s *Service) OnboardONVIF(ctx context.Context, in ONVIFOnboardRequest) (Camera, error) {
	if s == nil || s.Store == nil {
		return Camera{}, errors.New("camera service unavailable")
	}
	u, _, err := s.Network.ValidateURL(ctx, in.DeviceURL, "http", "https")
	if err != nil {
		return Camera{}, err
	}
	var password string
	if in.Username != "" {
		if in.PasswordRef == "" || s.Secrets == nil {
			return Camera{}, errors.New("ONVIF credentials require password secret reference")
		}
		b, err := s.Secrets.Resolve(in.PasswordRef)
		if err != nil {
			return Camera{}, err
		}
		password = string(b)
	}
	transport := &http.Transport{DialContext: s.Network.DialContext, Proxy: nil, ForceAttemptHTTP2: false}
	client := onvif.New(in.DeviceURL, in.Username, password)
	client.HTTP = &http.Client{Transport: transport, Timeout: 10 * time.Second}
	info, err := client.GetDeviceInformation(ctx)
	if err != nil {
		return Camera{}, err
	}
	caps, err := client.GetCapabilities(ctx)
	if err != nil {
		return Camera{}, err
	}
	if _, _, err := s.Network.ValidateURL(ctx, caps.MediaXAddr, "http", "https"); err != nil {
		return Camera{}, err
	}
	profiles, err := client.GetProfiles(ctx, caps.MediaXAddr)
	if err != nil {
		return Camera{}, err
	}
	main, detect, ok := selectProfiles(profiles)
	if !ok {
		return Camera{}, errors.New("ONVIF camera exposes no usable video profiles")
	}
	id, err := domain.NewID("cam")
	if err != nil {
		return Camera{}, err
	}
	cam := Camera{ID: id.String(), Name: in.Name, Type: TypeONVIF, Host: u.Hostname(), Manufacturer: info.Manufacturer, Model: info.Model, Firmware: info.FirmwareVersion, Capabilities: Capabilities{ONVIF: true, PTZ: caps.PTZXAddr != ""}}
	for i, selected := range []struct {
		profile onvif.Profile
		role    StreamRole
	}{{main, RoleMain}, {detect, RoleDetect}} {
		if i == 1 && detect.Token == "" {
			continue
		}
		uri, err := client.GetStreamURI(ctx, caps.MediaXAddr, selected.profile.Token)
		if err != nil {
			if selected.role == RoleMain {
				return Camera{}, err
			}
			continue
		}
		streamID, err := domain.NewID("str")
		if err != nil {
			return Camera{}, err
		}
		st := Stream{ID: streamID.String(), Role: selected.role, Endpoint: Endpoint{URL: uri, Username: in.Username, PasswordRef: in.PasswordRef}}
		if selected.profile.Video != nil {
			st.Codec = selected.profile.Video.Encoding
			st.Width = selected.profile.Video.Width
			st.Height = selected.profile.Video.Height
			st.FPS = selected.profile.Video.FPS
		}
		if selected.profile.Audio != nil {
			st.AudioCodec = selected.profile.Audio.Encoding
			cam.Capabilities.Audio = true
		}
		cam.Streams = append(cam.Streams, st)
	}
	if len(cam.Streams) == 0 {
		return Camera{}, errors.New("ONVIF camera returned no usable RTSP stream URIs")
	}
	// Media probe is mandatory for the main stream; advertised ONVIF metadata
	// alone is not accepted as proof that the stream is decodable.
	probed, err := s.ProbeRTSP(ctx, cam.Streams[0].Endpoint)
	if err != nil {
		return Camera{}, err
	}
	if len(probed.Probe.Media.Video) > 0 {
		v := probed.Probe.Media.Video[0]
		cam.Streams[0].Codec, cam.Streams[0].Width, cam.Streams[0].Height, cam.Streams[0].FPS, cam.Streams[0].Bitrate = v.Codec, v.Width, v.Height, v.FPS, v.Bitrate
	}
	if _, err := client.GetSnapshotURI(ctx, caps.MediaXAddr, main.Token); err == nil {
		cam.Capabilities.Snapshot = true
	}
	cam.Observed = Health{Status: "HEALTHY", CheckedAt: time.Now().UTC(), Latency: probed.Probe.Media.ProbeLatency}
	if err := cam.Validate(); err != nil {
		return Camera{}, err
	}
	res, err := s.Store.Put(ctx, cam.ID, cam)
	if err != nil {
		return Camera{}, err
	}
	return res.Value, nil
}

func selectProfiles(profiles []onvif.Profile) (main onvif.Profile, detect onvif.Profile, ok bool) {
	usable := make([]onvif.Profile, 0, len(profiles))
	for _, p := range profiles {
		if p.Token != "" && p.Video != nil && p.Video.Width > 0 && p.Video.Height > 0 {
			usable = append(usable, p)
		}
	}
	if len(usable) == 0 {
		return onvif.Profile{}, onvif.Profile{}, false
	}
	sort.SliceStable(usable, func(i, j int) bool { return area(usable[i]) > area(usable[j]) })
	main = usable[0]
	if len(usable) == 1 {
		return main, onvif.Profile{}, true
	}
	const target = 1280 * 720
	best := math.MaxFloat64
	for _, p := range usable[1:] {
		a := area(p)
		if a <= 0 {
			continue
		}
		score := math.Abs(math.Log(float64(a) / float64(target)))
		if score < best {
			best, detect = score, p
		}
	}
	return main, detect, true
}
func area(p onvif.Profile) int {
	if p.Video == nil {
		return 0
	}
	return p.Video.Width * p.Video.Height
}
