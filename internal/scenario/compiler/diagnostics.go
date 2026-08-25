package compiler

import (
	"fmt"
	"strings"
)

type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// Standard diagnostic error code namespaces:
// HS-SCN-1xx — syntax / schema / structure
// HS-SCN-2xx — type / binding / expression
// HS-SCN-3xx — capability / entity resolution
// HS-SCN-4xx — temporal / scheduling / duration
// HS-SCN-5xx — safety / permission / physical risk
// HS-SCN-6xx — graph / static conflict / reachability
// HS-SCN-7xx — runtime lowering / engine compatibility
// HS-SCN-8xx — catalog / versioning
// HS-SCN-9xx — internal compiler invariants
const (
	// HS-SCN-1xx
	CodeInvalidScenarioID   = "HS-SCN-101"
	CodeInvalidStepID       = "HS-SCN-102"
	CodeEmptyFlow           = "HS-SCN-103"
	CodeDuplicateStepID     = "HS-SCN-104"
	CodeMalformedStructure  = "HS-SCN-105"
	CodeMissingPayload      = "HS-SCN-106"

	// HS-SCN-2xx
	CodeTypeMismatch        = "HS-SCN-201"
	CodeUnknownReference    = "HS-SCN-202"
	CodeInvalidExprOperator = "HS-SCN-203"
	CodeMissingRequiredArg  = "HS-SCN-204"
	CodeUnknownArgument     = "HS-SCN-205"
	CodeInvalidEnumValue    = "HS-SCN-206"
	CodeUnitMismatch        = "HS-SCN-207"

	// HS-SCN-3xx
	CodeCapabilityNotFound  = "HS-SCN-301"
	CodeCapabilityIncompatible = "HS-SCN-302"
	CodeCapabilityUnavailable = "HS-SCN-303"
	CodeEntityNotFound      = "HS-SCN-304"
	CodeEntityKindMismatch  = "HS-SCN-305"

	// HS-SCN-4xx
	CodeInvalidDuration     = "HS-SCN-401"
	CodeInvalidSchedule     = "HS-SCN-402"
	CodeInvalidTimezone     = "HS-SCN-403"
	CodeOverlappingSchedule = "HS-SCN-404"

	// HS-SCN-5xx
	CodePermissionDenied    = "HS-SCN-501"
	CodeHighRiskUnapproved  = "HS-SCN-502"
	CodeMissingSafetyGate   = "HS-SCN-503"
	CodeResourceContention  = "HS-SCN-504"
	CodeUnboundedSirenDuration = "HS-SCN-505"
	CodeDisallowedSystemUnlock = "HS-SCN-506"

	// HS-SCN-6xx
	CodeUnreachableStep     = "HS-SCN-601"
	CodeImpossibleBranch    = "HS-SCN-602"
	CodeSelfRecursion       = "HS-SCN-603"
	CodeMutualRecursion     = "HS-SCN-604"
	CodeOppositeDesiredState = "HS-SCN-605"
	CodeCircularDependency  = "HS-SCN-606"
	CodeDuplicateIrreversibleAction = "HS-SCN-607"

	// HS-SCN-7xx
	CodeAxiomUnsupportedFeature = "HS-SCN-701"
	CodeADGOLoweringFailed     = "HS-SCN-702"

	// HS-SCN-9xx
	CodeCompilerInvariantBroken = "HS-SCN-901"
)

type Diagnostic struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
	Related  []string `json:"related,omitempty"`
}

func (d Diagnostic) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[%s] %s: %s (%s)", d.Severity, d.Code, d.Message, d.Path))
	if d.Hint != "" {
		b.WriteString(fmt.Sprintf(" (Hint: %s)", d.Hint))
	}
	return b.String()
}

type Diagnostics []Diagnostic

func (ds Diagnostics) HasErrors() bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (ds Diagnostics) Errors() Diagnostics {
	var out Diagnostics
	for _, d := range ds {
		if d.Severity == SeverityError {
			out = append(out, d)
		}
	}
	return out
}

func (ds Diagnostics) Warnings() Diagnostics {
	var out Diagnostics
	for _, d := range ds {
		if d.Severity == SeverityWarning {
			out = append(out, d)
		}
	}
	return out
}

func (ds Diagnostics) Error() string {
	errs := ds.Errors()
	if len(errs) == 0 {
		return "compiler: no errors"
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.String()
	}
	return strings.Join(msgs, "; ")
}
