package frigate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

type EventReference struct {
	ID       string          `json:"id"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	Media    MediaReference  `json:"media"`
}
type MediaReference struct {
	EventID      string `json:"event_id"`
	SnapshotPath string `json:"snapshot_path"`
	ClipPath     string `json:"clip_path"`
}
type EventService struct{ Client *Client }

func (s EventService) Get(ctx context.Context, id string) (EventReference, error) {
	if s.Client == nil {
		return EventReference{}, errors.New("Frigate client unavailable")
	}
	m, err := s.Client.Event(ctx, id)
	if err != nil {
		return EventReference{}, err
	}
	return EventReference{ID: id, Metadata: m, Media: ReferenceMedia(id)}, nil
}
func (s EventService) List(ctx context.Context, q url.Values) ([]EventReference, error) {
	if s.Client == nil {
		return nil, errors.New("Frigate client unavailable")
	}
	items, err := s.Client.Events(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]EventReference, 0, len(items))
	for _, raw := range items {
		var header struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &header) != nil || header.ID == "" {
			continue
		}
		out = append(out, EventReference{ID: header.ID, Metadata: raw, Media: ReferenceMedia(header.ID)})
	}
	return out, nil
}
func ReferenceMedia(id string) MediaReference {
	if !safeID(id) {
		return MediaReference{}
	}
	return MediaReference{EventID: id, SnapshotPath: fmt.Sprintf("/api/events/%s/snapshot.jpg", id), ClipPath: fmt.Sprintf("/api/events/%s/clip.mp4", id)}
}
