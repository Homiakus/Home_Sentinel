package homeassistant

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
	mqttint "github.com/Homiakus/Home_Sentinel/internal/integrations/mqtt"
)

type MQTTCheckResult struct {
	ComponentLoaded   bool   `json:"component_loaded"`
	DiscoveryVerified bool   `json:"discovery_verified"`
	ProbeEntity       string `json:"probe_entity,omitempty"`
	Recommendation    string `json:"recommendation,omitempty"`
}

func CheckMQTTDiscovery(ctx context.Context, rest *RESTClient, discovery DiscoveryPublisher, timeout time.Duration) (MQTTCheckResult, error) {
	if rest == nil {
		return MQTTCheckResult{}, errors.New("Home Assistant REST client unavailable")
	}
	cfg, err := rest.Config(ctx)
	if err != nil {
		return MQTTCheckResult{}, err
	}
	result := MQTTCheckResult{ComponentLoaded: HasComponent(cfg, "mqtt")}
	if !result.ComponentLoaded {
		result.Recommendation = "Configure the MQTT integration in Home Assistant, then retry discovery verification."
		return result, nil
	}
	if discovery.MQTT == nil {
		result.Recommendation = "Sentinel MQTT is not connected; configure the shared Mosquitto broker before verification."
		return result, nil
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	id, err := domain.NewID("hap")
	if err != nil {
		return result, err
	}
	objectID := id.String()
	entityID := "binary_sensor." + objectID
	stateTopic := "sentinel/system/probe/" + objectID
	device := DiscoveryDevice{
		Device: map[string]any{"ids": []string{"home_sentinel_probe_" + objectID}, "name": "Home Sentinel discovery probe"},
		Origin: map[string]any{"name": "Home Sentinel"},
		Components: map[string]map[string]any{
			"probe": {
				"p": "binary_sensor", "unique_id": "home_sentinel_" + objectID, "default_entity_id": entityID,
				"name": "Discovery probe", "state_topic": stateTopic, "payload_on": "online", "payload_off": "offline", "entity_category": "diagnostic",
			},
		},
	}
	if err := discovery.PublishDevice(ctx, objectID, device); err != nil {
		return result, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = discovery.RemoveDevice(cleanup, objectID)
		_ = discovery.MQTT.Publish(cleanup, mqttint.Message{Topic: stateTopic, Payload: []byte{}, QoS: 1, Retained: true})
	}()
	if err := discovery.MQTT.Publish(ctx, mqttint.Message{Topic: stateTopic, Payload: []byte("online"), QoS: 1, Retained: true}); err != nil {
		return result, err
	}
	result.ProbeEntity = entityID
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		st, err := rest.State(ctx, entityID)
		if err == nil && strings.EqualFold(st.State, "on") {
			result.DiscoveryVerified = true
			return result, nil
		}
		if err != nil {
			var he *HTTPError
			if !errors.As(err, &he) || he.Status != http.StatusNotFound {
				return result, fmt.Errorf("verify Home Assistant discovery probe: %w", err)
			}
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-deadline.C:
			result.Recommendation = "MQTT is loaded, but the discovery probe did not appear. Verify that Home Assistant and Sentinel use the same broker and that MQTT discovery is enabled."
			return result, nil
		case <-tick.C:
		}
	}
}
