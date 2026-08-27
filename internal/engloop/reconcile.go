package engloop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Severity string

const (
	SeverityInfo    Severity = "INFO"
	SeverityWarning Severity = "WARNING"
	SeverityBlocker Severity = "BLOCKER"
)

type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Path     string   `json:"path,omitempty"`
	Message  string   `json:"message"`
}

type ReconcileReport struct {
	Root     string    `json:"root"`
	Findings []Finding `json:"findings"`
}

func (r ReconcileReport) HasBlockers() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityBlocker {
			return true
		}
	}
	return false
}

func Reconcile(root string) (ReconcileReport, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return ReconcileReport{}, fmt.Errorf("resolve root: %w", err)
	}
	report := ReconcileReport{Root: abs}

	required := []string{
		"docs/PLAN_INDEX.md",
		"docs/engineering/ENGINEERING_LOOP.md",
		"docs/testing/EDGE_SPACE_MODEL.md",
		"docs/testing/MUTATION_TESTING.md",
		"docs/testing/STRATEGY.md",
		"docs/AXIOM_IMPLEMENTATION_PLAN.md",
		"docs/SCENARIO_SYSTEM_PLAN.md",
	}
	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(abs, filepath.FromSlash(rel))); err != nil {
			if os.IsNotExist(err) {
				report.Findings = append(report.Findings, Finding{
					ID:       "required-file-missing",
					Severity: SeverityBlocker,
					Path:     rel,
					Message:  "required engineering-loop source of truth is missing",
				})
				continue
			}
			return ReconcileReport{}, fmt.Errorf("stat %s: %w", rel, err)
		}
	}

	planIndex, err := readOptional(abs, "docs/PLAN_INDEX.md")
	if err != nil {
		return ReconcileReport{}, err
	}
	if planIndex != "" && !strings.Contains(planIndex, "engineering/ENGINEERING_LOOP.md") {
		report.Findings = append(report.Findings, Finding{
			ID:       "plan-index-loop-unbound",
			Severity: SeverityBlocker,
			Path:     "docs/PLAN_INDEX.md",
			Message:  "PLAN_INDEX does not bind roadmap execution to ENGINEERING_LOOP.md",
		})
	}

	status, err := readOptional(abs, "docs/IMPLEMENTATION_STATUS.md")
	if err != nil {
		return ReconcileReport{}, err
	}
	_, goSumErr := os.Stat(filepath.Join(abs, "go.sum"))
	if goSumErr == nil && statusClaimsMissingGoSum(status) {
		report.Findings = append(report.Findings, Finding{
			ID:       "status-go-sum-stale",
			Severity: SeverityWarning,
			Path:     "docs/IMPLEMENTATION_STATUS.md",
			Message:  "recorded status says go.sum is absent, but go.sum exists in the observed checkout",
		})
	} else if goSumErr != nil && !os.IsNotExist(goSumErr) {
		return ReconcileReport{}, fmt.Errorf("stat go.sum: %w", goSumErr)
	}

	ci, err := readOptional(abs, ".github/workflows/ci.yml")
	if err != nil {
		return ReconcileReport{}, err
	}
	if goSumErr == nil && strings.Contains(ci, "cache: false") && strings.Contains(ci, "go.sum is not committed") {
		report.Findings = append(report.Findings, Finding{
			ID:       "ci-module-lock-comment-stale",
			Severity: SeverityWarning,
			Path:     ".github/workflows/ci.yml",
			Message:  "CI still carries the pre-go.sum cache-disable assumption; Stage 14 needs reconciliation",
		})
	}
	if ci != "" && !strings.Contains(ci, "sentinel-engloop") {
		report.Findings = append(report.Findings, Finding{
			ID:       "ci-engloop-not-wired",
			Severity: SeverityWarning,
			Path:     ".github/workflows/ci.yml",
			Message:  "CI does not yet execute the engineering-loop reconciler",
		})
	}

	mutation, err := readOptional(abs, "docs/testing/MUTATION_TESTING.md")
	if err != nil {
		return ReconcileReport{}, err
	}
	if mutation != "" && !strings.Contains(mutation, "v0.6.0") {
		report.Findings = append(report.Findings, Finding{
			ID:       "mutation-engine-unpinned",
			Severity: SeverityWarning,
			Path:     "docs/testing/MUTATION_TESTING.md",
			Message:  "mutation policy names Gremlins but does not pin the reviewed v0.6.0 tool version",
		})
	}

	if len(report.Findings) == 0 {
		report.Findings = append(report.Findings, Finding{
			ID:       "reconcile-clean",
			Severity: SeverityInfo,
			Message:  "no known roadmap/status contradictions detected",
		})
	}
	return report, nil
}

func statusClaimsMissingGoSum(s string) bool {
	lower := strings.ToLower(s)
	claims := []string{
		"no committed `go.sum`",
		"still has no committed `go.sum`",
		"repository still has no committed `go.sum`",
		"репозиторий всё ещё не имеет committed `go.sum`",
	}
	for _, claim := range claims {
		if strings.Contains(lower, strings.ToLower(claim)) {
			return true
		}
	}
	return false
}

func readOptional(root, rel string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", rel, err)
	}
	return string(b), nil
}
