package search

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/incidents"
	"github.com/Homiakus/Home_Sentinel/internal/intercom"
)

type Hit struct {
	Kind       string    `json:"kind"`
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Subtitle   string    `json:"subtitle,omitempty"`
	CameraID   string    `json:"camera_id,omitempty"`
	OccurredAt time.Time `json:"occurred_at,omitempty"`
	Score      int       `json:"score"`
}

type Service struct {
	Cameras   *cameras.Service
	Incidents *incidents.Service
	Intercom  *intercom.Service
}

func (s *Service) Query(ctx context.Context, raw string, limit int) ([]Hit, error) {
	q := strings.ToLower(strings.TrimSpace(raw))
	if q == "" {
		return []Hit{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var out []Hit
	if s.Cameras != nil {
		items, err := s.Cameras.List(ctx, 500)
		if err != nil {
			return nil, err
		}
		for _, c := range items {
			hay := strings.ToLower(strings.Join([]string{c.ID, c.Name, string(c.Type), c.Host, c.Manufacturer, c.Model, c.Firmware}, " "))
			if score := matchScore(q, hay, strings.ToLower(c.Name)); score > 0 {
				out = append(out, Hit{Kind: "camera", ID: c.ID, Title: c.Name, Subtitle: strings.TrimSpace(c.Manufacturer + " " + c.Model), CameraID: c.ID, Score: score})
			}
		}
	}
	if s.Intercom != nil {
		items, err := s.Intercom.List(ctx, 200)
		if err != nil {
			return nil, err
		}
		for _, d := range items {
			hay := strings.ToLower(strings.Join([]string{d.ID, d.Name, d.Location, d.CameraID}, " "))
			if score := matchScore(q, hay, strings.ToLower(d.Name)); score > 0 {
				out = append(out, Hit{Kind: "intercom", ID: d.ID, Title: d.Name, Subtitle: d.Location, CameraID: d.CameraID, Score: score})
			}
		}
	}
	if s.Incidents != nil {
		items, err := s.Incidents.List(ctx, 500)
		if err != nil {
			return nil, err
		}
		for _, r := range items {
			i := r.Value
			hay := strings.ToLower(strings.Join([]string{i.ID.String(), i.Location, i.CameraID, string(i.State), strings.Join(i.ObjectIDs, " ")}, " "))
			// Event payloads are useful metadata even when semantic search is disabled.
			for _, eid := range i.EventIDs {
				if ev, err := s.Incidents.Event(ctx, eid.String()); err == nil {
					var compact any
					if json.Unmarshal(ev.Value.Payload, &compact) == nil {
						if b, err := json.Marshal(compact); err == nil {
							hay += " " + strings.ToLower(string(b))
						}
					}
					hay += " " + strings.ToLower(ev.Value.Type+" "+ev.Value.Source)
				}
			}
			if score := matchScore(q, hay, strings.ToLower(i.Location)); score > 0 {
				title := "Инцидент " + i.ID.String()
				if i.Location != "" {
					title = i.Location
				}
				out = append(out, Hit{Kind: "incident", ID: i.ID.String(), Title: title, Subtitle: string(i.State), CameraID: i.CameraID, OccurredAt: i.OpenedAt, Score: score})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func matchScore(q, hay, primary string) int {
	if q == "" || !strings.Contains(hay, q) {
		return 0
	}
	score := 10
	if strings.Contains(primary, q) {
		score += 20
	}
	if primary == q {
		score += 30
	}
	for _, token := range strings.Fields(q) {
		if strings.Contains(hay, token) {
			score += 2
		}
	}
	return score
}
