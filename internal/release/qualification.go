package release

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type CheckStatus string

const (
	StatusPending CheckStatus = "PENDING"
	StatusPass    CheckStatus = "PASS"
	StatusFail    CheckStatus = "FAIL"
	StatusSkip    CheckStatus = "SKIP"
)

type QualificationCheck struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Mandatory bool        `json:"mandatory"`
	Status    CheckStatus `json:"status"`
	Evidence  []string    `json:"evidence,omitempty"`
	Note      string      `json:"note,omitempty"`
}

type QualificationReport struct {
	Version     string               `json:"version"`
	GeneratedAt time.Time            `json:"generated_at"`
	Checks      []QualificationCheck `json:"checks"`
}

func (r QualificationReport) Validate() error {
	if strings.TrimSpace(r.Version) == "" {
		return errors.New("release version required")
	}
	seen := map[string]struct{}{}
	for i, c := range r.Checks {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Name) == "" {
			return fmt.Errorf("check %d requires id and name", i)
		}
		if _, ok := seen[c.ID]; ok {
			return fmt.Errorf("duplicate check id %q", c.ID)
		}
		seen[c.ID] = struct{}{}
		switch c.Status {
		case StatusPending, StatusPass, StatusFail, StatusSkip:
		default:
			return fmt.Errorf("check %s has invalid status %q", c.ID, c.Status)
		}
		if c.Status == StatusPass && len(c.Evidence) == 0 {
			return fmt.Errorf("check %s PASS requires evidence", c.ID)
		}
	}
	return nil
}

func (r QualificationReport) Releasable() (bool, []string) {
	var blockers []string
	if err := r.Validate(); err != nil {
		return false, []string{err.Error()}
	}
	for _, c := range r.Checks {
		if c.Mandatory && c.Status != StatusPass {
			blockers = append(blockers, fmt.Sprintf("%s=%s", c.ID, c.Status))
		}
	}
	sort.Strings(blockers)
	return len(blockers) == 0, blockers
}

func DecodeQualification(r io.Reader) (QualificationReport, error) {
	var q QualificationReport
	dec := json.NewDecoder(io.LimitReader(r, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&q); err != nil {
		return q, err
	}
	if err := q.Validate(); err != nil {
		return q, err
	}
	if q.GeneratedAt.IsZero() {
		q.GeneratedAt = time.Now().UTC()
	}
	return q, nil
}

func WriteQualificationMarkdown(w io.Writer, r QualificationReport) error {
	ready, blockers := r.Releasable()
	state := "BLOCKED"
	if ready {
		state = "READY"
	}
	if _, err := fmt.Fprintf(w, "# Home Sentinel release qualification — %s\n\n**State:** %s  \n**Generated:** %s\n\n", r.Version, state, r.GeneratedAt.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	if len(blockers) > 0 {
		if _, err := fmt.Fprintf(w, "## Blocking checks\n\n- %s\n\n", strings.Join(blockers, "\n- ")); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "## Checks\n\n| ID | Mandatory | Status | Check | Evidence |\n|---|---:|---|---|---|\n"); err != nil {
		return err
	}
	for _, c := range r.Checks {
		ev := strings.Join(c.Evidence, "; ")
		ev = strings.ReplaceAll(ev, "|", "\\|")
		name := strings.ReplaceAll(c.Name, "|", "\\|")
		mandatory := "no"
		if c.Mandatory {
			mandatory = "yes"
		}
		if _, err := fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n", c.ID, mandatory, c.Status, name, ev); err != nil {
			return err
		}
	}
	return nil
}
