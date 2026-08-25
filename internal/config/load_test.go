package config

import "testing"

func TestLoadEnvironmentOverrides(t *testing.T) {
	t.Setenv("SENTINEL_LISTEN", "127.0.0.1:18080")
	t.Setenv("SENTINEL_CAMERA_CIDRS", "192.168.30.0/24,10.44.0.0/16")
	t.Setenv("SENTINEL_EXPERIMENTAL", "true")
	t.Setenv("SENTINEL_READ_TIMEOUT", "3s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ListenAddress != "127.0.0.1:18080" {
		t.Fatalf("listen=%q", cfg.Server.ListenAddress)
	}
	if len(cfg.Network.CameraCIDRs) != 2 {
		t.Fatalf("cidrs=%v", cfg.Network.CameraCIDRs)
	}
	if !cfg.Features.Experimental {
		t.Fatal("experimental override missing")
	}
	if cfg.Server.ReadTimeout.String() != "3s" {
		t.Fatalf("read timeout=%s", cfg.Server.ReadTimeout)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("SENTINEL_CAMERA_CIDRS", "not-a-cidr")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid config")
	}
}

func TestLoadFrigateInternalPortWithoutToken(t *testing.T) {
	t.Setenv("SENTINEL_FRIGATE_ENABLED", "true")
	t.Setenv("SENTINEL_FRIGATE_URL", "http://frigate:5000")
	t.Setenv("SENTINEL_FRIGATE_CREDENTIALS_DIR", "/tmp/frigate-credentials")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Frigate.Enabled || cfg.Frigate.TokenRef != "" {
		t.Fatalf("frigate=%+v", cfg.Frigate)
	}
}

func TestLoadMQTTEnvironment(t *testing.T) {
	t.Setenv("SENTINEL_MQTT_ENABLED", "true")
	t.Setenv("SENTINEL_MQTT_URL", "mqtt://mosquitto:1883")
	t.Setenv("SENTINEL_MQTT_CLIENT_ID", "sentinel-node-a")
	t.Setenv("SENTINEL_MQTT_USERNAME", "sentinel")
	t.Setenv("SENTINEL_MQTT_PASSWORD_REF", "secret://env/MQTT_PASSWORD")
	t.Setenv("SENTINEL_MQTT_KEEP_ALIVE", "45s")
	t.Setenv("SENTINEL_MQTT_SESSION_EXPIRY", "20m")
	t.Setenv("SENTINEL_MQTT_CONNECT_TIMEOUT", "7s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MQTT.Enabled || cfg.MQTT.URL != "mqtt://mosquitto:1883" || cfg.MQTT.ClientID != "sentinel-node-a" {
		t.Fatalf("mqtt=%+v", cfg.MQTT)
	}
	if cfg.MQTT.KeepAlive.String() != "45s" || cfg.MQTT.SessionExpiry.String() != "20m0s" || cfg.MQTT.ConnectTimeout.String() != "7s" {
		t.Fatalf("mqtt timing=%+v", cfg.MQTT)
	}
}

func TestLoadFrigateWebRTCCandidates(t *testing.T) {
	t.Setenv("SENTINEL_FRIGATE_WEBRTC_CANDIDATES", "192.168.1.10:8555,100.64.0.5:8555")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Frigate.WebRTCCandidates) != 2 || cfg.Frigate.WebRTCCandidates[1] != "100.64.0.5:8555" {
		t.Fatalf("candidates=%v", cfg.Frigate.WebRTCCandidates)
	}
}
