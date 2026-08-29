package incident

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func TestClassifyRiskV1PreservesHistoricalThresholdBoundaries(t *testing.T) {
	cases := []struct {
		score float64
		want  domainincident.Risk
	}{
		{score: 0.49, want: domainincident.RiskLow},
		{score: 0.50, want: domainincident.RiskMedium},
		{score: 0.74, want: domainincident.RiskMedium},
		{score: 0.75, want: domainincident.RiskHigh},
		{score: 0.89, want: domainincident.RiskHigh},
		{score: 0.90, want: domainincident.RiskCritical},
		{score: 1.00, want: domainincident.RiskCritical},
	}
	for _, tc := range cases {
		if got := classifyRiskV1(tc.score); got != tc.want {
			t.Fatalf("classifyRiskV1(%0.2f)=%s want=%s", tc.score, got, tc.want)
		}
	}
}

func TestAssessRiskV1PreservesEvidenceArithmeticAndCap(t *testing.T) {
	cases := []struct {
		name       string
		confidence float64
		kind       string
		evidence   int
		wantScore  float64
		wantRisk   domainincident.Risk
	}{
		{name: "zero evidence", confidence: 0.20, kind: "motion", evidence: 0, wantScore: 0.11, wantRisk: domainincident.RiskLow},
		{name: "single evidence", confidence: 0.20, kind: "motion", evidence: 1, wantScore: 0.16, wantRisk: domainincident.RiskLow},
		{name: "evidence saturates", confidence: 0.20, kind: "motion", evidence: 10, wantScore: 0.26, wantRisk: domainincident.RiskLow},
		{name: "negative evidence contributes nothing", confidence: 0.20, kind: "motion", evidence: -1, wantScore: 0.11, wantRisk: domainincident.RiskLow},
		{name: "person and score cap", confidence: 1.20, kind: "person", evidence: 10, wantScore: 1.00, wantRisk: domainincident.RiskCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			triggerRaw, err := json.Marshal(domainincident.Trigger{
				EventID: "evt-risk-v1", SourceID: "source", Kind: tc.kind,
				OccurredAt: time.Now().UTC(), Confidence: tc.confidence,
			})
			if err != nil {
				t.Fatal(err)
			}
			evidenceRaw, err := json.Marshal(tc.evidence)
			if err != nil {
				t.Fatal(err)
			}
			result, err := assessRiskV1(context.Background(), adgo.ActivityRequest{Data: map[string]json.RawMessage{
				"trigger":        triggerRaw,
				"evidence_count": evidenceRaw,
			}})
			if err != nil {
				t.Fatalf("assessRiskV1: %v", err)
			}
			var score float64
			if err := decodeFact(result.Facts["risk_score"], &score); err != nil {
				t.Fatalf("decode score: %v", err)
			}
			var risk domainincident.Risk
			if err := decodeFact(result.Facts["risk"], &risk); err != nil {
				t.Fatalf("decode risk: %v", err)
			}
			if math.Abs(score-tc.wantScore) > 1e-12 || risk != tc.wantRisk {
				t.Fatalf("score/risk=%0.12f/%s want=%0.12f/%s", score, risk, tc.wantScore, tc.wantRisk)
			}
		})
	}
}

func decodeFact(value any, out any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func TestServePreflightRequiresCompleteRuntimeComponents(t *testing.T) {
	valid := memoryService(t, gatewayfake.NewNotifier())

	var nilService *Service
	if err := nilService.servePreflight(context.Background()); err == nil || !strings.Contains(err.Error(), "service is not open") {
		t.Fatalf("nil service preflight error=%v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Service)
	}{
		{name: "host", mutate: func(service *Service) { service.host = nil }},
		{name: "production", mutate: func(service *Service) { service.production = nil }},
		{name: "schedule runner", mutate: func(service *Service) {
			production := *service.production
			production.ScheduleRunner = nil
			service.production = &production
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := *valid
			tc.mutate(&copy)
			if err := copy.servePreflight(context.Background()); err == nil || !strings.Contains(err.Error(), "service is not open") {
				t.Fatalf("servePreflight error=%v", err)
			}
		})
	}
}

func TestServePreflightRejectsUnknownNonTerminalBundle(t *testing.T) {
	ctx := context.Background()
	service := memoryService(t, gatewayfake.NewNotifier())
	execution, err := service.Start(ctx, legacyTrigger("evt-serve-unknown-bundle"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	_, err = service.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanDigest = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		return nil
	})
	if err != nil {
		t.Fatalf("inject unknown digest: %v", err)
	}
	if err := service.servePreflight(ctx); !errors.Is(err, ErrUnknownExecutionBundle) {
		t.Fatalf("servePreflight error=%v want ErrUnknownExecutionBundle", err)
	}
}

func TestNormalizeServeErrorPreservesParentCancellationSemantics(t *testing.T) {
	sentinel := errors.New("sentinel")
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		name           string
		ctx            context.Context
		err            error
		want           error
		forbidCanceled bool
	}{
		{name: "active parent cancellation result remains cancellation", ctx: context.Background(), err: context.Canceled, want: context.Canceled},
		{name: "canceled parent owns cancellation", ctx: canceledCtx, err: context.Canceled, want: context.Canceled},
		{name: "canceled parent does not hide runtime error", ctx: canceledCtx, err: sentinel, want: sentinel, forbidCanceled: true},
		{name: "active parent preserves runtime error", ctx: context.Background(), err: sentinel, want: sentinel, forbidCanceled: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeServeError(tc.ctx, tc.err)
			if !errors.Is(got, tc.want) {
				t.Fatalf("normalizeServeError=%v want errors.Is(%v)", got, tc.want)
			}
			if tc.forbidCanceled && errors.Is(got, context.Canceled) {
				t.Fatalf("normalizeServeError=%v unexpectedly became context.Canceled", got)
			}
		})
	}
}

func TestServeReturnsCanceledContext(t *testing.T) {
	service := memoryService(t, gatewayfake.NewNotifier())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.Serve(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve error=%v want context.Canceled", err)
	}
}
