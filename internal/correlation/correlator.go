package correlation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain/artifact"
	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/domain/observation"
)

type Status string

const (
	StatusNewGroup  Status = "new_group"
	StatusUpdated   Status = "updated"
	StatusDuplicate Status = "duplicate"
	StatusLate      Status = "late"
)

type Config struct {
	Window            time.Duration
	MaxLateness       time.Duration
	MaxEventsPerGroup int
	MaxSeenEvents     int
	MaxArtifacts      int
}

func DefaultConfig() Config {
	return Config{
		Window:            15 * time.Second,
		MaxLateness:       5 * time.Second,
		MaxEventsPerGroup: 64,
		MaxSeenEvents:     4096,
		MaxArtifacts:      32,
	}
}

type Candidate struct {
	CorrelationID string           `json:"correlationId"`
	SubjectKey    string           `json:"subjectKey"`
	EventIDs      []string         `json:"eventIds"`
	SourceCount   int              `json:"sourceCount"`
	EventCount    int              `json:"eventCount"`
	Trigger       incident.Trigger `json:"trigger"`
}

type Result struct {
	Status    Status    `json:"status"`
	Candidate Candidate `json:"candidate"`
}

type seenRecord struct {
	correlationID string
}

type group struct {
	id          string
	subjectKey  string
	maxOccurred time.Time
	events      []observation.Observation
}

type Correlator struct {
	mu        sync.Mutex
	config    Config
	groups    map[string]*group
	seen      map[string]seenRecord
	seenOrder []string
}

func New(config Config) (*Correlator, error) {
	if config.Window <= 0 {
		return nil, errors.New("correlation: window must be > 0")
	}
	if config.MaxLateness < 0 || config.MaxLateness > config.Window {
		return nil, errors.New("correlation: max lateness must be within [0, window]")
	}
	if config.MaxEventsPerGroup <= 0 || config.MaxSeenEvents <= 0 || config.MaxArtifacts <= 0 {
		return nil, errors.New("correlation: capacity limits must be > 0")
	}
	return &Correlator{
		config: config,
		groups: map[string]*group{},
		seen:   map[string]seenRecord{},
	}, nil
}

func MustNew(config Config) *Correlator {
	c, err := New(config)
	if err != nil {
		panic(err)
	}
	return c
}

func (c *Correlator) Ingest(obs observation.Observation) (Result, error) {
	if err := obs.Validate(); err != nil {
		return Result{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if seen, ok := c.seen[obs.EventID]; ok {
		if current := c.groups[obs.SubjectKey]; current != nil && current.id == seen.correlationID {
			return Result{Status: StatusDuplicate, Candidate: c.buildCandidate(current)}, nil
		}
		return Result{Status: StatusDuplicate}, nil
	}

	current := c.groups[obs.SubjectKey]
	if current != nil && !current.maxOccurred.IsZero() && obs.OccurredAt.Before(current.maxOccurred.Add(-c.config.MaxLateness)) {
		c.remember(obs.EventID, current.id)
		return Result{Status: StatusLate, Candidate: c.buildCandidate(current)}, nil
	}

	status := StatusUpdated
	if current == nil || obs.OccurredAt.After(current.maxOccurred.Add(c.config.Window)) {
		current = &group{
			id:          correlationID(obs.SubjectKey, obs.EventID),
			subjectKey:  obs.SubjectKey,
			maxOccurred: obs.OccurredAt,
		}
		c.groups[obs.SubjectKey] = current
		status = StatusNewGroup
	}

	current.events = append(current.events, obs)
	if obs.OccurredAt.After(current.maxOccurred) {
		current.maxOccurred = obs.OccurredAt
	}
	sort.SliceStable(current.events, func(i, j int) bool {
		if current.events[i].OccurredAt.Equal(current.events[j].OccurredAt) {
			return current.events[i].EventID < current.events[j].EventID
		}
		return current.events[i].OccurredAt.Before(current.events[j].OccurredAt)
	})
	if len(current.events) > c.config.MaxEventsPerGroup {
		current.events = append([]observation.Observation(nil), current.events[len(current.events)-c.config.MaxEventsPerGroup:]...)
	}
	c.remember(obs.EventID, current.id)
	return Result{Status: status, Candidate: c.buildCandidate(current)}, nil
}

// Prune drops inactive subject groups whose newest event is older than before.
// Seen-event deduplication has its own bounded FIFO and is not coupled to group lifetime.
func (c *Correlator) Prune(before time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	for subject, g := range c.groups {
		if g.maxOccurred.Before(before) {
			delete(c.groups, subject)
			removed++
		}
	}
	return removed
}

func (c *Correlator) remember(eventID, correlationID string) {
	if _, exists := c.seen[eventID]; exists {
		return
	}
	c.seen[eventID] = seenRecord{correlationID: correlationID}
	c.seenOrder = append(c.seenOrder, eventID)
	for len(c.seenOrder) > c.config.MaxSeenEvents {
		oldest := c.seenOrder[0]
		c.seenOrder = c.seenOrder[1:]
		delete(c.seen, oldest)
	}
}

func (c *Correlator) buildCandidate(g *group) Candidate {
	if g == nil || len(g.events) == 0 {
		return Candidate{}
	}

	eventIDs := make([]string, 0, len(g.events))
	sources := map[string]struct{}{}
	artifacts := make([]artifact.Ref, 0, c.config.MaxArtifacts)
	artifactSeen := map[string]struct{}{}
	confidence := 0.0
	context := incident.SecurityContext{}
	kind := g.events[len(g.events)-1].Kind
	latest := g.events[len(g.events)-1]
	personDetected := false

	for _, event := range g.events {
		eventIDs = append(eventIDs, event.EventID)
		sources[event.SourceID] = struct{}{}
		if event.Confidence > confidence {
			confidence = event.Confidence
		}
		if strings.Contains(strings.ToLower(event.Kind), "person") {
			personDetected = true
		}
		if event.Context.AlarmMode != "" {
			context.AlarmMode = event.Context.AlarmMode
		}
		context.Identity = mergeIdentity(context.Identity, event.Context.Identity)
		context.EntryActive = context.EntryActive || event.Context.EntryActive
		if event.Context.DwellSeconds > context.DwellSeconds {
			context.DwellSeconds = event.Context.DwellSeconds
		}
		if event.Context.CrossCameraMatches > context.CrossCameraMatches {
			context.CrossCameraMatches = event.Context.CrossCameraMatches
		}
		for _, ref := range event.Artifacts {
			key := ref.Digest + "\x00" + ref.URI
			if _, ok := artifactSeen[key]; ok || len(artifacts) >= c.config.MaxArtifacts {
				continue
			}
			artifactSeen[key] = struct{}{}
			artifacts = append(artifacts, ref)
		}
	}

	if personDetected {
		kind = "correlated.person.activity.v1"
	}
	if len(sources) > 1 && len(sources)-1 > context.CrossCameraMatches {
		context.CrossCameraMatches = len(sources) - 1
	}
	span := g.events[len(g.events)-1].OccurredAt.Sub(g.events[0].OccurredAt).Seconds()
	if span > context.DwellSeconds {
		context.DwellSeconds = span
	}

	return Candidate{
		CorrelationID: g.id,
		SubjectKey:    g.subjectKey,
		EventIDs:      eventIDs,
		SourceCount:   len(sources),
		EventCount:    len(g.events),
		Trigger: incident.Trigger{
			EventID:       g.id,
			SourceID:      latest.SourceID,
			Kind:          kind,
			OccurredAt:    g.events[0].OccurredAt,
			Confidence:    confidence,
			Artifacts:     artifacts,
			CorrelationID: g.id,
			Context:       context,
		},
	}
}

func mergeIdentity(current, next incident.IdentityState) incident.IdentityState {
	rank := func(value incident.IdentityState) int {
		switch value {
		case incident.IdentityUnknown:
			return 3
		case incident.IdentityUncertain:
			return 2
		case incident.IdentityKnown:
			return 1
		default:
			return 0
		}
	}
	if rank(next) > rank(current) {
		return next
	}
	return current
}

func correlationID(subjectKey, firstEventID string) string {
	sum := sha256.Sum256([]byte(subjectKey + "\x00" + firstEventID))
	return "corr-" + hex.EncodeToString(sum[:16])
}
