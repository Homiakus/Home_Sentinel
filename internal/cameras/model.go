package cameras

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/secrets"
)

type Type string

const (
	TypeONVIF Type = "onvif"
	TypeRTSP  Type = "rtsp"
	TypeUVC   Type = "uvc"
	TypeMJPEG Type = "mjpeg"
)

type StreamRole string

const (
	RoleMain   StreamRole = "main"
	RoleDetect StreamRole = "detect"
)

type Endpoint struct {
	URL         string      `json:"url"`
	Username    string      `json:"username,omitempty"`
	PasswordRef secrets.Ref `json:"password_ref,omitempty"`
}
type Stream struct {
	ID         string     `json:"id"`
	Role       StreamRole `json:"role"`
	Endpoint   Endpoint   `json:"endpoint"`
	Codec      string     `json:"codec,omitempty"`
	Width      int        `json:"width,omitempty"`
	Height     int        `json:"height,omitempty"`
	FPS        float64    `json:"fps,omitempty"`
	Bitrate    int64      `json:"bitrate,omitempty"`
	AudioCodec string     `json:"audio_codec,omitempty"`
}
type Capabilities struct {
	Snapshot bool `json:"snapshot"`
	Audio    bool `json:"audio"`
	Talk     bool `json:"talk"`
	PTZ      bool `json:"ptz"`
	ONVIF    bool `json:"onvif"`
}
type Health struct {
	Status     string        `json:"status"`
	CheckedAt  time.Time     `json:"checked_at"`
	Latency    time.Duration `json:"latency,omitempty"`
	ReasonCode string        `json:"reason_code,omitempty"`
}
type Camera struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Type         Type         `json:"type"`
	Host         string       `json:"host,omitempty"`
	Manufacturer string       `json:"manufacturer,omitempty"`
	Model        string       `json:"model,omitempty"`
	Firmware     string       `json:"firmware,omitempty"`
	Streams      []Stream     `json:"streams"`
	Capabilities Capabilities `json:"capabilities"`
	Observed     Health       `json:"observed"`
}

func (c Camera) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("camera id required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("camera name required")
	}
	switch c.Type {
	case TypeONVIF, TypeRTSP, TypeUVC, TypeMJPEG:
	default:
		return errors.New("invalid camera type")
	}
	if c.Type != TypeUVC && len(c.Streams) == 0 {
		return errors.New("at least one stream required")
	}
	seen := map[StreamRole]bool{}
	for _, s := range c.Streams {
		if s.ID == "" {
			return errors.New("stream id required")
		}
		if seen[s.Role] {
			return errors.New("duplicate stream role")
		}
		seen[s.Role] = true
		if s.Endpoint.URL != "" && c.Type != TypeUVC {
			u, err := url.Parse(s.Endpoint.URL)
			if err != nil || u.Scheme == "" {
				return errors.New("invalid stream URL")
			}
			if u.User != nil {
				return errors.New("stream URL must not contain credentials; use username/password_ref")
			}
		}
	}
	return nil
}
func (c Camera) StreamByRole(role StreamRole) (Stream, bool) {
	for _, s := range c.Streams {
		if s.Role == role {
			return s, true
		}
	}
	return Stream{}, false
}
