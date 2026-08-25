package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	mqttint "github.com/Homiakus/Home_Sentinel/internal/integrations/mqtt"
)

var discoveryID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type MQTTPublisher interface {
	Publish(context.Context, mqttint.Message) error
}

type DiscoveryDevice struct {
	Device       map[string]any            `json:"dev"`
	Origin       map[string]any            `json:"o"`
	Components   map[string]map[string]any `json:"cmps"`
	Availability []map[string]any          `json:"availability,omitempty"`
	QoS          int                       `json:"qos,omitempty"`
}

type DiscoveryPublisher struct {
	MQTT   MQTTPublisher
	Prefix string
}

func (p DiscoveryPublisher) Topic(objectID string) (string, error) {
	prefix := strings.TrimSpace(p.Prefix)
	if prefix == "" {
		prefix = "homeassistant"
	}
	if !discoveryID.MatchString(prefix) || !discoveryID.MatchString(objectID) {
		return "", errors.New("invalid Home Assistant discovery identifier")
	}
	return fmt.Sprintf("%s/device/%s/config", prefix, objectID), nil
}

func (p DiscoveryPublisher) PublishDevice(ctx context.Context, objectID string, device DiscoveryDevice) error {
	if p.MQTT == nil {
		return errors.New("MQTT publisher unavailable")
	}
	if len(device.Device) == 0 || len(device.Origin) == 0 || len(device.Components) == 0 {
		return errors.New("Home Assistant device discovery requires device, origin and components")
	}
	for id, component := range device.Components {
		if !discoveryID.MatchString(id) {
			return fmt.Errorf("invalid discovery component id %q", id)
		}
		if component["p"] == nil || component["unique_id"] == nil {
			return fmt.Errorf("component %q requires platform and unique_id", id)
		}
	}
	topic, err := p.Topic(objectID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(device)
	if err != nil {
		return err
	}
	return p.MQTT.Publish(ctx, mqttint.Message{Topic: topic, Payload: payload, QoS: 1, Retained: true})
}

func (p DiscoveryPublisher) RemoveDevice(ctx context.Context, objectID string) error {
	if p.MQTT == nil {
		return errors.New("MQTT publisher unavailable")
	}
	topic, err := p.Topic(objectID)
	if err != nil {
		return err
	}
	return p.MQTT.Publish(ctx, mqttint.Message{Topic: topic, Payload: []byte{}, QoS: 1, Retained: true})
}

type SystemState struct {
	Health                 string `json:"health"`
	RecordingDiskFreeBytes *int64 `json:"recording_disk_free_bytes,omitempty"`
	AIStatus               string `json:"ai_status,omitempty"`
	BackupStatus           string `json:"backup_status,omitempty"`
}

func (p DiscoveryPublisher) PublishAvailability(ctx context.Context, topic string, online bool) error {
	if p.MQTT == nil {
		return errors.New("MQTT publisher unavailable")
	}
	if err := mqttint.ValidatePublishTopic(topic); err != nil {
		return err
	}
	payload := []byte("offline")
	if online {
		payload = []byte("online")
	}
	return p.MQTT.Publish(ctx, mqttint.Message{Topic: topic, Payload: payload, QoS: 1, Retained: true})
}

func (p DiscoveryPublisher) PublishSystemState(ctx context.Context, state SystemState) error {
	if p.MQTT == nil {
		return errors.New("MQTT publisher unavailable")
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return p.MQTT.Publish(ctx, mqttint.Message{Topic: "sentinel/system/state", Payload: payload, QoS: 1, Retained: true})
}

func (p DiscoveryPublisher) PublishCameraConnectivity(ctx context.Context, cameraID string, online bool) error {
	if !discoveryID.MatchString(cameraID) {
		return errors.New("invalid camera id for Home Assistant state")
	}
	return p.PublishAvailability(ctx, "sentinel/camera/"+cameraID+"/state/availability", online)
}

func SentinelSystemDevice(version string) DiscoveryDevice {
	return DiscoveryDevice{
		Device: map[string]any{
			"ids":  []string{"home_sentinel"},
			"name": "Home Sentinel",
			"mf":   "Home Sentinel",
			"mdl":  "Local Security Control Plane",
			"sw":   version,
		},
		Origin: map[string]any{"name": "Home Sentinel", "sw": version},
		Availability: []map[string]any{{
			"topic":                 "sentinel/system/availability",
			"payload_available":     "online",
			"payload_not_available": "offline",
		}},
		QoS: 1,
		Components: map[string]map[string]any{
			"system_health": {
				"p": "sensor", "unique_id": "home_sentinel_system_health", "default_entity_id": "sensor.home_sentinel_system_health",
				"name": "System health", "state_topic": "sentinel/system/state", "value_template": "{{ value_json.health }}", "entity_category": "diagnostic",
			},
			"recording_disk_free": {
				"p": "sensor", "unique_id": "home_sentinel_recording_disk_free", "default_entity_id": "sensor.home_sentinel_recording_disk_free",
				"name": "Recording disk free", "state_topic": "sentinel/system/state", "value_template": "{{ value_json.recording_disk_free_bytes }}", "unit_of_measurement": "B", "entity_category": "diagnostic",
			},
			"ai_status": {
				"p": "sensor", "unique_id": "home_sentinel_ai_status", "default_entity_id": "sensor.home_sentinel_ai_status",
				"name": "AI status", "state_topic": "sentinel/system/state", "value_template": "{{ value_json.ai_status }}", "entity_category": "diagnostic",
			},
			"backup_status": {
				"p": "sensor", "unique_id": "home_sentinel_backup_status", "default_entity_id": "sensor.home_sentinel_backup_status",
				"name": "Backup status", "state_topic": "sentinel/system/state", "value_template": "{{ value_json.backup_status }}", "entity_category": "diagnostic",
			},
		},
	}
}

func SentinelCameraDevice(cameraID, name, version string) (DiscoveryDevice, error) {
	if !discoveryID.MatchString(cameraID) {
		return DiscoveryDevice{}, errors.New("invalid camera id for Home Assistant discovery")
	}
	base := "sentinel/camera/" + cameraID + "/state/"
	return DiscoveryDevice{
		Device: map[string]any{"ids": []string{"home_sentinel_camera_" + cameraID}, "name": name, "via_device": "home_sentinel", "mf": "Home Sentinel", "mdl": "Managed camera"},
		Origin: map[string]any{"name": "Home Sentinel", "sw": version},
		Components: map[string]map[string]any{
			"connectivity": {
				"p": "binary_sensor", "unique_id": "home_sentinel_camera_" + cameraID + "_connectivity", "default_entity_id": "binary_sensor.home_sentinel_camera_" + cameraID + "_connectivity",
				"name": "Connectivity", "device_class": "connectivity", "state_topic": base + "availability", "payload_on": "online", "payload_off": "offline", "entity_category": "diagnostic",
			},
			"recording": {
				"p": "binary_sensor", "unique_id": "home_sentinel_camera_" + cameraID + "_recording", "default_entity_id": "binary_sensor.home_sentinel_camera_" + cameraID + "_recording",
				"name": "Recording", "state_topic": base + "recording", "payload_on": "on", "payload_off": "off",
			},
			"detection": {
				"p": "binary_sensor", "unique_id": "home_sentinel_camera_" + cameraID + "_detection", "default_entity_id": "binary_sensor.home_sentinel_camera_" + cameraID + "_detection",
				"name": "Detection", "state_topic": base + "detection", "payload_on": "on", "payload_off": "off",
			},
		},
	}, nil
}

func SentinelIntercomDevice(deviceID, name, version string) (DiscoveryDevice, error) {
	if !discoveryID.MatchString(deviceID) {
		return DiscoveryDevice{}, errors.New("invalid intercom id for Home Assistant discovery")
	}
	base := "sentinel/intercom/" + deviceID + "/state/"
	value := "{{ value_json.value }}"
	return DiscoveryDevice{
		Device: map[string]any{"ids": []string{"home_sentinel_intercom_" + deviceID}, "name": name, "via_device": "home_sentinel", "mf": "Home Sentinel", "mdl": "Managed intercom"},
		Origin: map[string]any{"name": "Home Sentinel", "sw": version},
		Components: map[string]map[string]any{
			"connectivity": {
				"p": "binary_sensor", "unique_id": "home_sentinel_intercom_" + deviceID + "_connectivity", "default_entity_id": "binary_sensor.home_sentinel_intercom_" + deviceID + "_connectivity",
				"name": "Connectivity", "device_class": "connectivity", "state_topic": base + "availability", "value_template": value, "payload_on": "online", "payload_off": "offline", "entity_category": "diagnostic",
			},
			"door": {
				"p": "binary_sensor", "unique_id": "home_sentinel_intercom_" + deviceID + "_door", "default_entity_id": "binary_sensor.home_sentinel_intercom_" + deviceID + "_door",
				"name": "Door", "device_class": "door", "state_topic": base + "door", "value_template": value, "payload_on": "open", "payload_off": "closed",
			},
			"lock_state": {
				"p": "sensor", "unique_id": "home_sentinel_intercom_" + deviceID + "_lock_state", "default_entity_id": "sensor.home_sentinel_intercom_" + deviceID + "_lock_state",
				"name": "Lock state", "state_topic": base + "lock", "value_template": value,
			},
		},
	}, nil
}
