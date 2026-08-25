package update

import (
	"strings"
	"testing"
)

func TestParseEnvAndInventory(t *testing.T) {
	env, err := ParseEnv(strings.NewReader(`SENTINEL_IMAGE=ghcr.io/x/sentinel:1.0.0
MOSQUITTO_IMAGE=eclipse-mosquitto:2.0.22
HOMEASSISTANT_IMAGE=ghcr.io/home-assistant/home-assistant:2026.8.1
FRIGATE_IMAGE=ghcr.io/blakeblackshear/frigate:0.17.0
OLLAMA_IMAGE=ollama/ollama:0.11.4
`))
	if err != nil {
		t.Fatal(err)
	}
	cur, err := Inventory(env, SystemReleaseInfo{Version: "1.0.0", SchemaVersion: 7})
	if err != nil {
		t.Fatal(err)
	}
	if cur.Components["frigate"] == "" || cur.SchemaVersion != 7 {
		t.Fatalf("%+v", cur)
	}
}
func TestComponentsRejectLatest(t *testing.T) {
	env := map[string]string{"SENTINEL_IMAGE": "x:latest", "MOSQUITTO_IMAGE": "x:1", "HOMEASSISTANT_IMAGE": "x:1", "FRIGATE_IMAGE": "x:1", "OLLAMA_IMAGE": "x:1"}
	if _, err := ComponentsFromEnv(env); err == nil {
		t.Fatal("expected rejection")
	}
}
