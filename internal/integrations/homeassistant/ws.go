package homeassistant

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

var ErrWebSocketRuntimeUnsupported = errors.New("Home Assistant WebSocket runtime requires Go 1.24 or newer in this build")

type HAEvent struct {
	EventType string         `json:"event_type"`
	Data      map[string]any `json:"data"`
	Origin    string         `json:"origin"`
	TimeFired string         `json:"time_fired"`
	Context   map[string]any `json:"context,omitempty"`
}

type StreamEvent struct {
	SubscriptionID int     `json:"subscription_id"`
	Event          HAEvent `json:"event"`
}

type WSOptions struct {
	BaseURL      string
	Token        string
	EventTypes   []string
	Buffer       int
	ReconnectMin time.Duration
	ReconnectMax time.Duration
}

type wsRuntime interface{ Close() error }

type WSStream struct {
	runtime wsRuntime
	events  chan StreamEvent
	ready   atomic.Bool
	dropped atomic.Uint64
}

func NewWSStream(ctx context.Context, opts WSOptions) (*WSStream, error) {
	if ctx == nil {
		return nil, errors.New("Home Assistant WebSocket context required")
	}
	if strings.TrimSpace(opts.Token) == "" {
		return nil, errors.New("Home Assistant WebSocket token required")
	}
	if _, err := websocketURL(opts.BaseURL); err != nil {
		return nil, err
	}
	if opts.Buffer <= 0 {
		opts.Buffer = 128
	}
	if opts.ReconnectMin <= 0 {
		opts.ReconnectMin = time.Second
	}
	if opts.ReconnectMax <= 0 {
		opts.ReconnectMax = 30 * time.Second
	}
	if opts.ReconnectMax < opts.ReconnectMin {
		return nil, errors.New("Home Assistant reconnect max must be >= min")
	}
	if len(opts.EventTypes) == 0 {
		opts.EventTypes = []string{"state_changed"}
	}
	for _, eventType := range opts.EventTypes {
		if eventType == "" || strings.ContainsAny(eventType, "\x00\r\n") {
			return nil, errors.New("invalid Home Assistant event type")
		}
	}
	s := &WSStream{events: make(chan StreamEvent, opts.Buffer)}
	r, err := newWSRuntime(ctx, opts, func(up bool) { s.ready.Store(up) }, func(e StreamEvent) {
		select {
		case s.events <- e:
		default:
			s.dropped.Add(1)
		}
	})
	if err != nil {
		close(s.events)
		return nil, err
	}
	s.runtime = r
	return s, nil
}

func websocketURL(base string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Host == "" {
		return "", errors.New("Home Assistant base URL invalid")
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", errors.New("Home Assistant base URL must use http or https")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/websocket"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func (s *WSStream) Events() <-chan StreamEvent { return s.events }
func (s *WSStream) Ready() bool                { return s != nil && s.ready.Load() }
func (s *WSStream) Dropped() uint64 {
	if s == nil {
		return 0
	}
	return s.dropped.Load()
}
func (s *WSStream) Close() error {
	if s == nil || s.runtime == nil {
		return nil
	}
	return s.runtime.Close()
}
