package homeassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mqttint "github.com/Homiakus/Home_Sentinel/internal/integrations/mqtt"
)

type callbackPublisher struct{ online atomic.Bool }

func (p *callbackPublisher) Publish(_ context.Context, msg mqttint.Message) error {
	if strings.Contains(msg.Topic, "/probe/") && string(msg.Payload) == "online" {
		p.online.Store(true)
	}
	return nil
}
func TestCheckMQTTDiscovery(t *testing.T) {
	pub := &callbackPublisher{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config" {
			_ = json.NewEncoder(w).Encode(ConfigInfo{Components: []string{"mqtt"}})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/states/binary_sensor.hap_") {
			if !pub.online.Load() {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(State{EntityID: strings.TrimPrefix(r.URL.Path, "/api/states/"), State: "on"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	rest, err := NewRESTClient(RESTOptions{BaseURL: srv.URL, Token: "x"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := CheckMQTTDiscovery(context.Background(), rest, DiscoveryPublisher{MQTT: pub}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !res.ComponentLoaded || !res.DiscoveryVerified {
		t.Fatalf("result=%+v", res)
	}
}
