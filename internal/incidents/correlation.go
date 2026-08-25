package incidents

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
)

type IDFactory func(string) (domain.ID, error)
type Engine struct {
	Window time.Duration
	NewID  IDFactory
	active []*Incident
	all    []Incident
}

func NewEngine(window time.Duration) *Engine {
	if window <= 0 {
		window = 45 * time.Second
	}
	return &Engine{Window: window, NewID: domain.NewID}
}

type clues struct {
	location, camera, object string
	doorRelated              bool
}

func (e *Engine) Process(ev events.Envelope) (Incident, error) {
	if err := ev.Validate(); err != nil {
		return Incident{}, err
	}
	if e.NewID == nil {
		e.NewID = domain.NewID
	}
	c := extractClues(ev)
	now := ev.OccurredAt
	var best *Incident
	for _, inc := range e.active {
		if inc.State != StateOpen || now.Sub(inc.LastEventAt) > e.Window || inc.LastEventAt.Sub(now) > e.Window {
			continue
		}
		if exactCorrelation(*inc, ev.CorrelationID) {
			best = inc
			break
		}
		if !samePlace(*inc, c) {
			continue
		}
		if c.object != "" && len(inc.ObjectIDs) > 0 && !containsString(inc.ObjectIDs, c.object) && !c.doorRelated {
			continue
		}
		if best == nil || inc.LastEventAt.After(best.LastEventAt) {
			best = inc
		}
	}
	if best == nil {
		id, err := e.NewID("inc")
		if err != nil {
			return Incident{}, err
		}
		inc := Incident{ID: id, State: StateOpen, OpenedAt: now, LastEventAt: now, Location: c.location, CameraID: c.camera}
		e.active = append(e.active, &inc)
		best = &inc
	}
	attach(best, ev, c)
	e.syncAll(*best)
	return *best, nil
}
func (e *Engine) CloseStale(now time.Time) []Incident {
	var closed []Incident
	for _, inc := range e.active {
		if inc.State == StateOpen && now.Sub(inc.LastEventAt) > e.Window {
			inc.State = StateClosed
			closed = append(closed, *inc)
			e.syncAll(*inc)
		}
	}
	return closed
}
func (e *Engine) Incidents() []Incident {
	out := append([]Incident(nil), e.all...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].OpenedAt.Before(out[j].OpenedAt) })
	return out
}
func (e *Engine) syncAll(x Incident) {
	for i := range e.all {
		if e.all[i].ID == x.ID {
			e.all[i] = x
			return
		}
	}
	e.all = append(e.all, x)
}
func attach(i *Incident, e events.Envelope, c clues) {
	if e.OccurredAt.Before(i.OpenedAt) {
		i.OpenedAt = e.OccurredAt
	}
	if e.OccurredAt.After(i.LastEventAt) {
		i.LastEventAt = e.OccurredAt
	}
	if i.Location == "" {
		i.Location = c.location
	}
	if i.CameraID == "" {
		i.CameraID = c.camera
	}
	if !containsID(i.EventIDs, e.ID) {
		i.EventIDs = append(i.EventIDs, e.ID)
	}
	if e.CorrelationID.ValidFor("cor") && !containsID(i.CorrelationIDs, e.CorrelationID) {
		i.CorrelationIDs = append(i.CorrelationIDs, e.CorrelationID)
	}
	if c.object != "" && !containsString(i.ObjectIDs, c.object) {
		i.ObjectIDs = append(i.ObjectIDs, c.object)
	}
}
func exactCorrelation(i Incident, c domain.ID) bool {
	return c.ValidFor("cor") && containsID(i.CorrelationIDs, c)
}
func samePlace(i Incident, c clues) bool {
	if c.location != "" && i.Location != "" {
		return c.location == i.Location
	}
	if c.camera != "" && i.CameraID != "" {
		return c.camera == i.CameraID
	}
	return false
}
func containsID(xs []domain.ID, v domain.ID) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func containsString(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func extractClues(e events.Envelope) clues {
	var m map[string]any
	if json.Unmarshal(e.Payload, &m) != nil {
		return clues{}
	}
	str := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	c := clues{location: str("location"), camera: str("camera_id", "camera"), object: str("object_id", "event_id")}
	if c.location == "" {
		c.location = c.camera
	}
	switch e.Type {
	case "intercom.button.pressed", "door.opened", "door.closed", "door.unlock.requested", "door.unlock.completed":
		c.doorRelated = true
		c.object = ""
	}
	return c
}

var ErrNoEvents = errors.New("no events")

func Correlate(eventsIn []events.Envelope, window time.Duration, idf IDFactory) ([]Incident, error) {
	if len(eventsIn) == 0 {
		return nil, ErrNoEvents
	}
	in := append([]events.Envelope(nil), eventsIn...)
	sort.SliceStable(in, func(i, j int) bool { return in[i].OccurredAt.Before(in[j].OccurredAt) })
	e := NewEngine(window)
	if idf != nil {
		e.NewID = idf
	}
	for _, ev := range in {
		if _, err := e.Process(ev); err != nil {
			return nil, err
		}
	}
	e.CloseStale(in[len(in)-1].OccurredAt.Add(window + time.Nanosecond))
	return e.Incidents(), nil
}
