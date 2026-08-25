package catalog

import (
	"errors"
	"fmt"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/compiler"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

var (
	ErrNotFound      = errors.New("catalog: scenario or revision not found")
	ErrConflict      = errors.New("catalog: optimistic concurrency conflict (stale ETag/version)")
	ErrImmutable     = errors.New("catalog: published revision is immutable")
	ErrNotValidated  = errors.New("catalog: scenario has compilation errors and cannot be published")
	ErrDependencyInUse = errors.New("catalog: dependency is currently in use by active scenarios")
)

type RevisionState string

const (
	StateDraft     RevisionState = "draft"
	StateValidated RevisionState = "validated"
	StateSimulated RevisionState = "simulated"
	StateReviewed  RevisionState = "reviewed"
	StatePublished RevisionState = "published"
)

type ScenarioStatus string

const (
	StatusActive   ScenarioStatus = "active"
	StatusDisabled ScenarioStatus = "disabled"
	StatusArchived ScenarioStatus = "archived"
)

type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"` // e.g. "create_draft", "update_draft", "publish", "rollback", "disable", "archive"
	Reason    string    `json:"reason,omitempty"`
	RevisionID string   `json:"revisionId,omitempty"`
	Version   int       `json:"version,omitempty"`
}

type SimulationSummary struct {
	SimulatedAt time.Time `json:"simulatedAt"`
	Passed      bool      `json:"passed"`
	TraceSteps  int       `json:"traceSteps"`
	Summary     string    `json:"summary"`
}

type Revision struct {
	ScenarioID     string             `json:"scenarioId"`
	RevisionID     string             `json:"revisionId"`
	Version        int                `json:"version"` // Monotonically increasing published version (1, 2, ...) or 0 for drafts
	State          RevisionState      `json:"state"`
	ETag           string             `json:"etag"`
	Author         string             `json:"author"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	PublishedAt    *time.Time         `json:"publishedAt,omitempty"`
	PublishedBy    string             `json:"publishedBy,omitempty"`
	PublishReason  string             `json:"publishReason,omitempty"`
	Scenario       model.Scenario     `json:"scenario"`
	Manifest       *compiler.Manifest `json:"manifest,omitempty"`
	Simulation     *SimulationSummary `json:"simulation,omitempty"`
}

type ScenarioRecord struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Description      string         `json:"description,omitempty"`
	Status           ScenarioStatus `json:"status"`
	ActiveRevisionID string         `json:"activeRevisionId,omitempty"`
	ActiveVersion    int            `json:"activeVersion,omitempty"`
	DraftRevisionID  string         `json:"draftRevisionId,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	Revisions        []Revision     `json:"revisions"`
	AuditLog         []AuditEntry   `json:"auditLog"`
}

func (s *ScenarioRecord) GetRevision(revID string) (*Revision, error) {
	for i := range s.Revisions {
		if s.Revisions[i].RevisionID == revID {
			return &s.Revisions[i], nil
		}
	}
	return nil, fmt.Errorf("%w: revision %s", ErrNotFound, revID)
}

func (s *ScenarioRecord) GetVersion(version int) (*Revision, error) {
	for i := range s.Revisions {
		if s.Revisions[i].Version == version && s.Revisions[i].State == StatePublished {
			return &s.Revisions[i], nil
		}
	}
	return nil, fmt.Errorf("%w: version %d", ErrNotFound, version)
}
