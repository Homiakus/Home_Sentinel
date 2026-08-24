package incident

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	riskpolicy "github.com/Homiakus/Home_Sentinel/internal/policy/risk"
	"github.com/Homiakus/axiom/adgo"
)

type TimelineEntry struct {
	Seq     uint64         `json:"seq"`
	At      time.Time      `json:"at"`
	Type    string         `json:"type"`
	NodeID  string         `json:"nodeId,omitempty"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type IncidentView struct {
	ID            string                 `json:"id"`
	PlanVersion   string                 `json:"planVersion"`
	PlanDigest    string                 `json:"planDigest"`
	Version       uint64                 `json:"version"`
	Status        string                 `json:"status"`
	Failure       string                 `json:"failure,omitempty"`
	Waiting       map[string]string      `json:"waiting,omitempty"`
	Risk          domainincident.Risk    `json:"risk,omitempty"`
	RiskAssessment *riskpolicy.Assessment `json:"riskAssessment,omitempty"`
	Timeline      []TimelineEntry        `json:"timeline"`
}

type ExplanationView struct {
	ExecutionID string         `json:"executionId"`
	NodeID      string         `json:"nodeId,omitempty"`
	Status      string         `json:"status"`
	Reason      string         `json:"reason"`
	BlockedBy   []string       `json:"blockedBy,omitempty"`
	Evidence    map[string]any `json:"evidence,omitempty"`
}

type DiagnosticIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	NodeID   string `json:"nodeId,omitempty"`
	TaskID   string `json:"taskId,omitempty"`
	Message  string `json:"message"`
}

type DiagnosticsView struct {
	Status      string            `json:"status"`
	Ready       []string          `json:"ready,omitempty"`
	Waiting     map[string]string `json:"waiting,omitempty"`
	ActiveTasks int               `json:"activeTasks"`
	Issues      []DiagnosticIssue `json:"issues,omitempty"`
}

func (s *Service) View(ctx context.Context, executionID string) (IncidentView, error) {
	execution, err := s.production.Engine.Get(ctx, executionID)
	if err != nil {
		return IncidentView{}, err
	}
	view := IncidentView{
		ID: execution.ID, PlanVersion: execution.PlanVersion, PlanDigest: execution.PlanDigest,
		Version: execution.Version, Status: string(execution.Status), Failure: execution.Failure,
		Waiting: cloneStrings(execution.WaitingFor), Timeline: make([]TimelineEntry, 0, len(execution.History)),
	}
	if raw, ok := execution.Data["risk"]; ok {
		_ = json.Unmarshal(raw, &view.Risk)
	}
	if raw, ok := execution.Data["risk_assessment"]; ok {
		var assessment riskpolicy.Assessment
		if json.Unmarshal(raw, &assessment) == nil {
			view.RiskAssessment = &assessment
		}
	}
	for _, entry := range execution.History {
		view.Timeline = append(view.Timeline, TimelineEntry{
			Seq: entry.Seq, At: entry.At, Type: entry.Type, NodeID: entry.NodeID,
			Message: entry.Message, Data: redactMap(entry.Data),
		})
	}
	return view, nil
}

func (s *Service) Explain(ctx context.Context, executionID, nodeID string) (ExplanationView, error) {
	execution, err := s.production.Engine.Get(ctx, executionID)
	if err != nil {
		return ExplanationView{}, err
	}
	explanation := adgo.Explain(s.plan, execution, nodeID)
	return ExplanationView{
		ExecutionID: explanation.ExecutionID, NodeID: explanation.NodeID,
		Status: explanation.Status, Reason: explanation.Reason,
		BlockedBy: append([]string(nil), explanation.BlockedBy...), Evidence: redactMap(explanation.Evidence),
	}, nil
}

func (s *Service) Diagnostics(ctx context.Context, executionID string) (DiagnosticsView, error) {
	diagnostics, err := s.production.Engine.Diagnostics(ctx, executionID)
	if err != nil {
		return DiagnosticsView{}, err
	}
	view := DiagnosticsView{
		Status: string(diagnostics.Summary.Status), Ready: append([]string(nil), diagnostics.Ready...),
		Waiting: cloneStrings(diagnostics.Waiting), ActiveTasks: len(diagnostics.ActiveTasks),
		Issues: make([]DiagnosticIssue, 0, len(diagnostics.Diagnostics)),
	}
	for _, issue := range diagnostics.Diagnostics {
		view.Issues = append(view.Issues, DiagnosticIssue{
			Severity: string(issue.Severity), Code: issue.Code, NodeID: issue.NodeID,
			TaskID: issue.TaskID, Message: issue.Message,
		})
	}
	return view, nil
}

func cloneStrings(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func redactMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || strings.Contains(lower, "cookie") || strings.Contains(lower, "token") {
			out[key] = "[REDACTED]"
			continue
		}
		out[key] = value
	}
	return out
}
