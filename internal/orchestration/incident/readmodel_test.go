package incident

import (
	"context"
	"strings"
	"testing"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func TestOperatorReadModelUsesCommittedADGOState(t *testing.T) {
	ctx := context.Background()
	notifier := gatewayfake.NewNotifier()
	service := memoryService(t, notifier)
	trigger := domainincident.Trigger{
		EventID: "read-model", SourceID: "front", Kind: "vision.person.detected.v1",
		OccurredAt: time.Now().UTC(), Confidence: 0.96,
	}
	execution, err := service.Start(ctx, trigger)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusWaiting {
		t.Fatalf("expected waiting incident, got %s", execution.Status)
	}

	view, err := service.View(ctx, execution.ID)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if view.Status != string(adgo.StatusWaiting) || view.Risk != domainincident.RiskMedium || view.RiskAssessment == nil {
		t.Fatalf("unexpected operator view: %#v", view)
	}
	if len(view.Timeline) == 0 {
		t.Fatal("operator timeline is empty")
	}

	explanation, err := service.Explain(ctx, execution.ID, NodeAwaitAck)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if explanation.Status != string(adgo.NodeWaiting) || !strings.Contains(explanation.Reason, "waiting") {
		t.Fatalf("unexpected explanation: %#v", explanation)
	}

	diagnostics, err := service.Diagnostics(ctx, execution.ID)
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if diagnostics.Status != string(adgo.StatusWaiting) || diagnostics.Waiting[NodeAwaitAck] != OwnerResponseEvent {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}

func TestTimelineRedactsCredentialLikeKeys(t *testing.T) {
	redacted := redactMap(map[string]any{
		"token": "secret-value", "Authorization": "Bearer x", "idempotencyKey": "safe-operation-key",
	})
	if redacted["token"] != "[REDACTED]" || redacted["Authorization"] != "[REDACTED]" {
		t.Fatalf("credential-like fields not redacted: %#v", redacted)
	}
	if redacted["idempotencyKey"] != "safe-operation-key" {
		t.Fatalf("safe operation key unexpectedly redacted: %#v", redacted)
	}
}
