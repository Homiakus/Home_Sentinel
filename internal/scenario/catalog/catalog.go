package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/compiler"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

type Catalog struct {
	mu          sync.RWMutex
	records     map[string]*ScenarioRecord
	compiler    *compiler.Compiler
	deps        *DependencyGraph
	timeSource  func() time.Time
}

func NewCatalog(comp *compiler.Compiler) *Catalog {
	return &Catalog{
		records:    make(map[string]*ScenarioRecord),
		compiler:   comp,
		deps:       NewDependencyGraph(),
		timeSource: time.Now,
	}
}

func (c *Catalog) now() time.Time {
	if c.timeSource != nil {
		return c.timeSource()
	}
	return time.Now()
}

func generateETag(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:8])
}

// CreateDraft initializes a new Scenario with its first draft revision.
func (c *Catalog) CreateDraft(scenario model.Scenario, author string) (*ScenarioRecord, *Revision, error) {
	if err := scenario.Validate(); err != nil {
		return nil, nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	scenID := string(scenario.ID)
	if scenID == "" {
		return nil, nil, fmt.Errorf("catalog: scenario ID is required")
	}

	record, exists := c.records[scenID]
	now := c.now()

	revID := fmt.Sprintf("rev_%d", now.UnixNano())
	etag := generateETag(fmt.Sprintf("%s:%s:%d", scenID, revID, now.UnixNano()))

	scenario.RevisionID = model.RevisionID(revID)

	rev := Revision{
		ScenarioID: scenID,
		RevisionID: revID,
		Version:    0, // Drafts have version 0
		State:      StateDraft,
		ETag:       etag,
		Author:     author,
		CreatedAt:  now,
		UpdatedAt:  now,
		Scenario:   scenario,
	}

	if !exists {
		record = &ScenarioRecord{
			ID:              scenID,
			Name:            scenario.Name,
			Description:     scenario.Description,
			Status:          StatusDisabled, // New scenarios start disabled until published
			DraftRevisionID: revID,
			CreatedAt:       now,
			UpdatedAt:       now,
			Revisions:       []Revision{rev},
			AuditLog: []AuditEntry{
				{
					Timestamp:  now,
					Actor:      author,
					Action:     "create_scenario",
					RevisionID: revID,
				},
			},
		}
		c.records[scenID] = record
	} else {
		record.DraftRevisionID = revID
		record.UpdatedAt = now
		record.Revisions = append(record.Revisions, rev)
		record.AuditLog = append(record.AuditLog, AuditEntry{
			Timestamp:  now,
			Actor:      author,
			Action:     "create_draft",
			RevisionID: revID,
		})
	}

	return record, &rev, nil
}

// UpdateDraft updates an existing draft revision with optimistic concurrency protection (ETag).
func (c *Catalog) UpdateDraft(scenarioID, draftRevID string, expectedETag string, scenario model.Scenario, author string) (*Revision, error) {
	if err := scenario.Validate(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return nil, ErrNotFound
	}

	rev, err := record.GetRevision(draftRevID)
	if err != nil {
		return nil, err
	}

	if rev.State == StatePublished {
		return nil, ErrImmutable
	}

	if expectedETag != "" && rev.ETag != expectedETag {
		return nil, fmt.Errorf("%w: current ETag is %q, expected %q", ErrConflict, rev.ETag, expectedETag)
	}

	now := c.now()
	scenario.ID = model.ID(scenarioID)
	scenario.RevisionID = model.RevisionID(draftRevID)

	rev.Scenario = scenario
	rev.Author = author
	rev.UpdatedAt = now
	rev.State = StateDraft
	rev.ETag = generateETag(fmt.Sprintf("%s:%s:%d", scenarioID, draftRevID, now.UnixNano()))
	rev.Manifest = nil // Invalidate compiled manifest on edit

	record.UpdatedAt = now
	record.Name = scenario.Name
	record.Description = scenario.Description
	record.AuditLog = append(record.AuditLog, AuditEntry{
		Timestamp:  now,
		Actor:      author,
		Action:     "update_draft",
		RevisionID: draftRevID,
	})

	return rev, nil
}

// ValidateDraft compiles a draft revision and saves the resulting Manifest in the revision.
func (c *Catalog) ValidateDraft(scenarioID, draftRevID string) (*compiler.Manifest, compiler.Diagnostics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return nil, nil, ErrNotFound
	}

	rev, err := record.GetRevision(draftRevID)
	if err != nil {
		return nil, nil, err
	}

	manifest, diags := c.compiler.Compile(rev.Scenario)
	if !diags.HasErrors() {
		rev.State = StateValidated
		rev.Manifest = manifest
	}

	return manifest, diags, nil
}

// RecordSimulation records simulation output for a draft revision.
func (c *Catalog) RecordSimulation(scenarioID, draftRevID string, summary SimulationSummary) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return ErrNotFound
	}

	rev, err := record.GetRevision(draftRevID)
	if err != nil {
		return err
	}

	rev.Simulation = &summary
	if summary.Passed && (rev.State == StateDraft || rev.State == StateValidated) {
		rev.State = StateSimulated
	}
	return nil
}

// PublishDraft compiles, verifies, and promotes a draft revision to an immutable published version.
func (c *Catalog) PublishDraft(scenarioID, draftRevID string, author string, reason string) (*Revision, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return nil, ErrNotFound
	}

	rev, err := record.GetRevision(draftRevID)
	if err != nil {
		return nil, err
	}

	if rev.State == StatePublished {
		return nil, ErrImmutable
	}

	// Always compile afresh to guarantee validity
	manifest, diags := c.compiler.Compile(rev.Scenario)
	if diags.HasErrors() {
		return nil, fmt.Errorf("%w: %s", ErrNotValidated, diags.Error())
	}

	now := c.now()
	nextVersion := record.ActiveVersion + 1

	rev.State = StatePublished
	rev.Version = nextVersion
	rev.Manifest = manifest
	rev.PublishedAt = &now
	rev.PublishedBy = author
	rev.PublishReason = reason
	rev.ETag = generateETag(fmt.Sprintf("%s:v%d:%s", scenarioID, nextVersion, manifest.PlanDigest))

	record.ActiveRevisionID = draftRevID
	record.ActiveVersion = nextVersion
	record.DraftRevisionID = ""
	record.Status = StatusActive
	record.UpdatedAt = now

	record.AuditLog = append(record.AuditLog, AuditEntry{
		Timestamp:  now,
		Actor:      author,
		Action:     "publish",
		Reason:     reason,
		RevisionID: draftRevID,
		Version:    nextVersion,
	})

	// Update Dependency Index
	c.deps.UpdateScenarioDependencies(scenarioID, manifest, "")

	return rev, nil
}

// RollbackToVersion reverts active execution to an existing published version without mutating history.
func (c *Catalog) RollbackToVersion(scenarioID string, targetVersion int, author string, reason string) (*Revision, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return nil, ErrNotFound
	}

	targetRev, err := record.GetVersion(targetVersion)
	if err != nil {
		return nil, err
	}

	now := c.now()
	record.ActiveRevisionID = targetRev.RevisionID
	record.ActiveVersion = targetRev.Version
	record.Status = StatusActive
	record.UpdatedAt = now

	record.AuditLog = append(record.AuditLog, AuditEntry{
		Timestamp:  now,
		Actor:      author,
		Action:     "rollback",
		Reason:     reason,
		RevisionID: targetRev.RevisionID,
		Version:    targetVersion,
	})

	// Update Dependency Index to the rolled-back revision
	c.deps.UpdateScenarioDependencies(scenarioID, targetRev.Manifest, "")

	return targetRev, nil
}

// SetScenarioStatus sets Active, Disabled, or Archived status.
func (c *Catalog) SetScenarioStatus(scenarioID string, status ScenarioStatus, author string, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return ErrNotFound
	}

	now := c.now()
	record.Status = status
	record.UpdatedAt = now

	if status == StatusArchived {
		c.deps.RemoveScenario(scenarioID)
	}

	record.AuditLog = append(record.AuditLog, AuditEntry{
		Timestamp: now,
		Actor:     author,
		Action:    fmt.Sprintf("set_status_%s", status),
		Reason:    reason,
	})

	return nil
}

func (c *Catalog) GetScenario(scenarioID string) (*ScenarioRecord, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return nil, ErrNotFound
	}
	return record, nil
}

func (c *Catalog) GetRevision(scenarioID, revisionID string) (*Revision, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return nil, ErrNotFound
	}
	return record.GetRevision(revisionID)
}

func (c *Catalog) GetActiveRevision(scenarioID string) (*Revision, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	record, exists := c.records[scenarioID]
	if !exists {
		return nil, ErrNotFound
	}
	if record.ActiveRevisionID == "" {
		return nil, fmt.Errorf("catalog: scenario %q has no active published revision", scenarioID)
	}
	return record.GetRevision(record.ActiveRevisionID)
}

func (c *Catalog) ListScenarios(statusFilter ScenarioStatus) []*ScenarioRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var out []*ScenarioRecord
	for _, rec := range c.records {
		if statusFilter == "" || rec.Status == statusFilter {
			out = append(out, rec)
		}
	}
	return out
}

func (c *Catalog) CanDeleteCapability(capID string) (bool, []string) {
	scens := c.deps.GetScenariosUsingCapability(capID)
	return len(scens) == 0, scens
}

func (c *Catalog) CanDeleteEntity(kind, id string) (bool, []string) {
	scens := c.deps.GetScenariosUsingEntity(kind, id)
	return len(scens) == 0, scens
}
