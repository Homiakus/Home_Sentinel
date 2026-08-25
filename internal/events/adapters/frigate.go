package adapters

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
)

type FrigateReviewPayload struct {
	Change    string   `json:"change"`
	EventID   string   `json:"event_id"`
	CameraID  string   `json:"camera_id"`
	Severity  string   `json:"severity,omitempty"`
	StartTime float64  `json:"start_time,omitempty"`
	EndTime   float64  `json:"end_time,omitempty"`
	Objects   []string `json:"objects,omitempty"`
}

func FrigateReview(raw []byte) (events.Envelope, error) {
	var native struct {
		Type   string          `json:"type"`
		Before json.RawMessage `json:"before"`
		After  struct {
			ID        string  `json:"id"`
			Camera    string  `json:"camera"`
			Severity  string  `json:"severity"`
			StartTime float64 `json:"start_time"`
			EndTime   float64 `json:"end_time"`
			Data      struct {
				Objects []string `json:"objects"`
			} `json:"data"`
		} `json:"after"`
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		return events.Envelope{}, err
	}
	if native.After.ID == "" || native.After.Camera == "" {
		return events.Envelope{}, errors.New("Frigate review event id and camera required")
	}
	sev := events.SeverityInfo
	if strings.EqualFold(native.After.Severity, "alert") {
		sev = events.SeverityWarning
	}
	occurred := time.Now().UTC()
	if native.After.StartTime > 0 {
		sec := int64(native.After.StartTime)
		ns := int64((native.After.StartTime - float64(sec)) * 1e9)
		occurred = time.Unix(sec, ns).UTC()
	}
	p := FrigateReviewPayload{Change: native.Type, EventID: native.After.ID, CameraID: native.After.Camera, Severity: native.After.Severity, StartTime: native.After.StartTime, EndTime: native.After.EndTime, Objects: append([]string(nil), native.After.Data.Objects...)}
	return envelope("frigate.review.updated", "frigate:mqtt", occurred, sev, p, domain.ID(""))
}

type FrigateTrackedUpdatePayload struct {
	UpdateType  string `json:"update_type"`
	EventID     string `json:"event_id"`
	Description string `json:"description,omitempty"`
	SubLabel    string `json:"sub_label,omitempty"`
}

func FrigateTrackedObjectUpdate(raw []byte) (events.Envelope, error) {
	var n struct {
		Type        string `json:"type"`
		ID          string `json:"id"`
		Description string `json:"description"`
		SubLabel    string `json:"sub_label"`
	}
	if err := json.Unmarshal(raw, &n); err != nil {
		return events.Envelope{}, err
	}
	if n.ID == "" || n.Type == "" {
		return events.Envelope{}, errors.New("Frigate tracked update id/type required")
	}
	return envelope("frigate.tracked_object.updated", "frigate:mqtt", time.Now().UTC(), events.SeverityInfo, FrigateTrackedUpdatePayload{UpdateType: n.Type, EventID: n.ID, Description: n.Description, SubLabel: n.SubLabel}, domain.ID(""))
}
