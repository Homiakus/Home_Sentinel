package homeassistant

import (
	"context"
	"errors"
)

type Service struct {
	REST      *RESTClient
	Stream    *WSStream
	Discovery DiscoveryPublisher
	Actions   *ActionBridge
}

type Status struct {
	Reachable      bool   `json:"reachable"`
	Version        string `json:"version,omitempty"`
	MQTTLoaded     bool   `json:"mqtt_loaded"`
	FrigateLoaded  bool   `json:"frigate_loaded"`
	WebSocketReady bool   `json:"websocket_ready"`
	Error          string `json:"error,omitempty"`
}

func (s *Service) Status(ctx context.Context) Status {
	if s == nil || s.REST == nil {
		return Status{Error: "Home Assistant service unavailable"}
	}
	cfg, err := s.REST.Config(ctx)
	if err != nil {
		return Status{Error: err.Error()}
	}
	return Status{Reachable: true, Version: cfg.Version, MQTTLoaded: HasComponent(cfg, "mqtt"), FrigateLoaded: HasComponent(cfg, "frigate"), WebSocketReady: s.Stream != nil && s.Stream.Ready()}
}
func (s *Service) Close() error {
	if s == nil || s.Stream == nil {
		return nil
	}
	return s.Stream.Close()
}
func (s *Service) PublishSystemDiscovery(ctx context.Context, version string) error {
	if s == nil || s.Discovery.MQTT == nil {
		return errors.New("Home Assistant MQTT discovery unavailable")
	}
	return s.Discovery.PublishDevice(ctx, "home_sentinel", SentinelSystemDevice(version))
}
