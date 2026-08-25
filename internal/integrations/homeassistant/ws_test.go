package homeassistant

import "testing"

func TestWebSocketURL(t *testing.T) {
	got, err := websocketURL("https://ha.example.local/base/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "wss://ha.example.local/base/api/websocket" {
		t.Fatalf("url=%s", got)
	}
}
