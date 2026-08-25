package mqtt

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var stableID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)

const (
	FrigateReviews             = "frigate/reviews"
	FrigateTrackedObjectUpdate = "frigate/tracked_object_update"
	HomeAssistantStatus        = "homeassistant/status"
)

func CameraState(cameraID, key string) (string, error) {
	return entityTopic("camera", cameraID, "state", key)
}
func IntercomState(deviceID, key string) (string, error) {
	return entityTopic("intercom", deviceID, "state", key)
}
func IntercomEvent(deviceID, key string) (string, error) {
	return entityTopic("intercom", deviceID, "event", key)
}
func IntercomCommand(deviceID, key string) (string, error) {
	return entityTopic("intercom", deviceID, "command", key)
}

func entityTopic(kind, id, class, key string) (string, error) {
	if !stableID.MatchString(id) {
		return "", errors.New("invalid stable id for MQTT topic")
	}
	if key == "" || strings.ContainsAny(key, "+#\x00") || strings.Contains(key, "//") || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") {
		return "", errors.New("invalid MQTT topic key")
	}
	t := fmt.Sprintf("sentinel/%s/%s/%s/%s", kind, id, class, key)
	return t, ValidatePublishTopic(t)
}

func ValidatePublishTopic(topic string) error {
	if topic == "" || strings.ContainsAny(topic, "+#\x00") || strings.HasPrefix(topic, "$") || strings.Contains(topic, "//") || strings.HasPrefix(topic, "/") || strings.HasSuffix(topic, "/") {
		return errors.New("invalid publish topic")
	}
	return nil
}

func ValidateSubscribeTopic(topic string) error {
	if topic == "" || strings.ContainsRune(topic, '\x00') || strings.HasPrefix(topic, "$") || strings.Contains(topic, "//") || strings.HasPrefix(topic, "/") || strings.HasSuffix(topic, "/") {
		return errors.New("invalid subscribe topic")
	}
	parts := strings.Split(topic, "/")
	for i, part := range parts {
		if strings.Contains(part, "#") && (part != "#" || i != len(parts)-1) {
			return errors.New("# wildcard must occupy final topic level")
		}
		if strings.Contains(part, "+") && part != "+" {
			return errors.New("+ wildcard must occupy complete topic level")
		}
	}
	return nil
}
