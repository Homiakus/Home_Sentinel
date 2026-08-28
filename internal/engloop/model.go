package engloop

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

type PlanState string

const (
	StateOpen                  PlanState = "OPEN"
	StatePartial               PlanState = "PARTIAL"
	StateImplementedUnverified PlanState = "IMPLEMENTED_UNVERIFIED"
	StateVerified              PlanState = "VERIFIED"
	StateBlocked               PlanState = "BLOCKED"
	StateStale                 PlanState = "STALE"
	StateSuperseded            PlanState = "SUPERSEDED"
)

type RiskClass string

const (
	RiskLow      RiskClass = "LOW"
	RiskMedium   RiskClass = "MEDIUM"
	RiskHigh     RiskClass = "HIGH"
	RiskCritical RiskClass = "CRITICAL"
)

type Gate string

const (
	GateStatic      Gate = "static"
	GateUnit        Gate = "unit"
	GateProperty    Gate = "property"
	GateFuzz        Gate = "fuzz"
	GateRace        Gate = "race"
	GateContract    Gate = "contract"
	GateIntegration Gate = "integration"
	GateFault       Gate = "fault"
	GateMutation    Gate = "mutation-critical"
	GateReplay      Gate = "replay"
	GateSecurity    Gate = "security"
	GatePerformance Gate = "performance"
)

type WorkPacket struct {
	PlanItem           string    `json:"plan_item"`
	Intent             string    `json:"intent"`
	StatusBefore       PlanState `json:"status_before"`
	RiskClass          RiskClass `json:"risk_class"`
	Invariants         []string  `json:"invariants"`
	Contracts          []string  `json:"contracts,omitempty"`
	CodeSurfaces       []string  `json:"code_surfaces"`
	RequiredGates      []Gate    `json:"required_gates"`
	AcceptanceEvidence []string  `json:"acceptance_evidence"`
}

func DecodeWorkPacket(r io.Reader) (WorkPacket, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var p WorkPacket
	if err := dec.Decode(&p); err != nil {
		return WorkPacket{}, fmt.Errorf("decode work packet: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return WorkPacket{}, fmt.Errorf("decode work packet: multiple JSON values")
		}
		return WorkPacket{}, fmt.Errorf("decode work packet trailing data: %w", err)
	}
	if err := p.Validate(); err != nil {
		return WorkPacket{}, err
	}
	return p, nil
}

func (p WorkPacket) Validate() error {
	if strings.TrimSpace(p.PlanItem) == "" {
		return fmt.Errorf("plan_item is required")
	}
	if strings.TrimSpace(p.Intent) == "" {
		return fmt.Errorf("intent is required")
	}
	if !validPlanState(p.StatusBefore) {
		return fmt.Errorf("invalid status_before %q", p.StatusBefore)
	}
	if !validRisk(p.RiskClass) {
		return fmt.Errorf("invalid risk_class %q", p.RiskClass)
	}
	if len(nonEmpty(p.Invariants)) == 0 {
		return fmt.Errorf("at least one invariant is required")
	}
	if len(nonEmpty(p.CodeSurfaces)) == 0 {
		return fmt.Errorf("at least one code_surface is required")
	}
	if len(nonEmpty(p.AcceptanceEvidence)) == 0 {
		return fmt.Errorf("at least one acceptance_evidence item is required")
	}
	if len(p.RequiredGates) == 0 {
		return fmt.Errorf("at least one required_gate is required")
	}
	seen := map[Gate]struct{}{}
	for _, gate := range p.RequiredGates {
		if !validGate(gate) {
			return fmt.Errorf("invalid required_gate %q", gate)
		}
		if _, ok := seen[gate]; ok {
			return fmt.Errorf("duplicate required_gate %q", gate)
		}
		seen[gate] = struct{}{}
	}
	if p.RiskClass == RiskCritical {
		for _, gate := range []Gate{GateStatic, GateUnit, GateRace, GateMutation} {
			if _, ok := seen[gate]; !ok {
				return fmt.Errorf("critical work packet requires gate %q", gate)
			}
		}
	}
	return nil
}

func ClassifyPaths(paths []string) RiskClass {
	risk := RiskLow
	for _, raw := range paths {
		p := cleanPath(raw)
		switch {
		case isCriticalSurface(p):
			return RiskCritical
		case hasAnyPrefix(p,
			"internal/api/",
			"internal/config/",
			"internal/database/",
			"internal/events/",
			"internal/scenario/",
			"internal/workflow/",
			"deploy/",
			".github/workflows/",
		):
			risk = maxRisk(risk, RiskHigh)
		case strings.HasPrefix(p, "internal/") || strings.HasPrefix(p, "cmd/"):
			risk = maxRisk(risk, RiskMedium)
		}
	}
	return risk
}

func GatePlan(paths []string, requested RiskClass) []Gate {
	risk := requested
	if !validRisk(risk) {
		risk = ClassifyPaths(paths)
	}
	set := map[Gate]struct{}{
		GateStatic: {},
		GateUnit:   {},
	}
	if riskRank(risk) >= riskRank(RiskMedium) {
		set[GateProperty] = struct{}{}
		set[GateRace] = struct{}{}
	}
	if riskRank(risk) >= riskRank(RiskHigh) {
		set[GateFuzz] = struct{}{}
		set[GateContract] = struct{}{}
	}
	if risk == RiskCritical {
		set[GateFault] = struct{}{}
		set[GateMutation] = struct{}{}
		set[GateReplay] = struct{}{}
		set[GateSecurity] = struct{}{}
	}
	for _, raw := range paths {
		p := cleanPath(raw)
		if strings.Contains(p, "benchmark") || strings.HasSuffix(p, "PERFORMANCE.md") {
			set[GatePerformance] = struct{}{}
		}
		if strings.Contains(p, "adapter") || strings.Contains(p, "gateway") || strings.HasPrefix(p, "deploy/") {
			set[GateIntegration] = struct{}{}
		}
	}
	order := []Gate{
		GateStatic, GateUnit, GateProperty, GateFuzz, GateRace, GateContract,
		GateIntegration, GateFault, GateMutation, GateReplay, GateSecurity, GatePerformance,
	}
	out := make([]Gate, 0, len(set))
	for _, gate := range order {
		if _, ok := set[gate]; ok {
			out = append(out, gate)
		}
	}
	return out
}

func MutationTargets(paths []string) []string {
	set := map[string]struct{}{}
	for _, raw := range paths {
		p := cleanPath(raw)
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || !isCriticalSurface(p) {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(p))
		if dir == "." || dir == "" {
			continue
		}
		set["./"+dir] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for target := range set {
		out = append(out, target)
	}
	sort.Strings(out)
	return out
}

func validPlanState(v PlanState) bool {
	switch v {
	case StateOpen, StatePartial, StateImplementedUnverified, StateVerified, StateBlocked, StateStale, StateSuperseded:
		return true
	default:
		return false
	}
}

func validRisk(v RiskClass) bool {
	return riskRank(v) >= 0
}

func riskRank(v RiskClass) int {
	switch v {
	case RiskLow:
		return 0
	case RiskMedium:
		return 1
	case RiskHigh:
		return 2
	case RiskCritical:
		return 3
	default:
		return -1
	}
}

func maxRisk(a, b RiskClass) RiskClass {
	if riskRank(b) > riskRank(a) {
		return b
	}
	return a
}

func validGate(v Gate) bool {
	switch v {
	case GateStatic, GateUnit, GateProperty, GateFuzz, GateRace, GateContract, GateIntegration, GateFault, GateMutation, GateReplay, GateSecurity, GatePerformance:
		return true
	default:
		return false
	}
}

func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func cleanPath(p string) string {
	p = filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
	return strings.TrimPrefix(p, "./")
}

func isCriticalSurface(p string) bool {
	return hasAnyPrefix(p,
		"internal/security/",
		"internal/auth/",
		"internal/authz/",
		"internal/authorization/",
		"internal/gateway/",
		"internal/gateways/",
		"internal/admission/",
		"internal/reconciliation/",
		"internal/database/migrations/",
		"internal/scenario/safety/",
		"internal/scenario/compiler/",
		"internal/scenario/catalog/",
		"internal/workflow/physical/",
		// App bootstrap and durable incident lifecycle are an authority boundary:
		// changing start/stop ordering can expose callback ingress without a
		// durable workflow or close stores beneath running external-effect work.
		"internal/app/app.go",
		"internal/app/incident_runtime",
		// Durable notifiers are external side-effect gateways even though the
		// concrete adapter lives with its integration-facing service package.
		// Missing this prefix would let a CRITICAL work packet request mutation
		// testing while producing zero mutation targets for notifier semantics.
		"internal/telegram/notifier",
	)
}

func hasAnyPrefix(p string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}
