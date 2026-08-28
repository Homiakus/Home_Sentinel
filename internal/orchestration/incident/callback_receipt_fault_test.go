package incident

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/axiom/adgo"
)

func testCallbackReceipt(t *testing.T, eventID string) callbackDecisionReceipt {
	t.Helper()
	envelope, err := newCallbackDecisionEnvelope(
		eventID,
		testAdminID,
		domainincident.DecisionApprove,
		"verified",
		map[string]any{"source": "test"},
	)
	if err != nil {
		t.Fatalf("new callback envelope: %v", err)
	}
	return envelope.Receipt
}

func executionWithReceipt(t *testing.T, receipt callbackDecisionReceipt) *adgo.Execution {
	t.Helper()
	raw, err := json.Marshal(callbackDecisionEnvelope{Receipt: receipt})
	if err != nil {
		t.Fatalf("marshal callback receipt: %v", err)
	}
	return &adgo.Execution{Data: map[string]json.RawMessage{callbackHumanPayloadKey(): raw}}
}

func TestCallbackHumanPayloadKeyMatchesADGOContract(t *testing.T) {
	if got, want := callbackHumanPayloadKey(), "human:"+NodeHumanDecision+":payload"; got != want {
		t.Fatalf("payload key=%q want=%q", got, want)
	}
}

func TestReconcileStaleCallbackDecisionFaultMatrix(t *testing.T) {
	stale := adgo.ErrStaleTask
	expected := testCallbackReceipt(t, "event-1")

	t.Run("load failure", func(t *testing.T) {
		loadErr := errors.New("load failed")
		err := reconcileStaleCallbackDecision(nil, loadErr, expected.EventID, expected, stale)
		if !errors.Is(err, stale) || !errors.Is(err, loadErr) {
			t.Fatalf("error=%v want stale+load failure", err)
		}
	})

	t.Run("malformed durable receipt", func(t *testing.T) {
		raw, marshalErr := json.Marshal(map[string]any{"callback_receipt": "broken"})
		if marshalErr != nil {
			t.Fatalf("marshal malformed receipt fixture: %v", marshalErr)
		}
		loaded := &adgo.Execution{Data: map[string]json.RawMessage{callbackHumanPayloadKey(): raw}}
		err := reconcileStaleCallbackDecision(loaded, nil, expected.EventID, expected, stale)
		if !errors.Is(err, stale) || !strings.Contains(err.Error(), "decode durable callback receipt") {
			t.Fatalf("error=%v want stale+receipt decode failure", err)
		}
	})

	t.Run("missing receipt", func(t *testing.T) {
		err := reconcileStaleCallbackDecision(&adgo.Execution{Data: map[string]json.RawMessage{}}, nil, expected.EventID, expected, stale)
		if !errors.Is(err, stale) || errors.Is(err, ErrCallbackDecisionConflict) {
			t.Fatalf("error=%v want stale only", err)
		}
	})

	t.Run("different receipt event", func(t *testing.T) {
		other := testCallbackReceipt(t, "event-other")
		err := reconcileStaleCallbackDecision(executionWithReceipt(t, other), nil, expected.EventID, expected, stale)
		if !errors.Is(err, stale) || errors.Is(err, ErrCallbackDecisionConflict) {
			t.Fatalf("error=%v want stale only", err)
		}
	})

	t.Run("same event changed semantic input", func(t *testing.T) {
		other := expected
		other.Digest = strings.Repeat("f", 64)
		err := reconcileStaleCallbackDecision(executionWithReceipt(t, other), nil, expected.EventID, expected, stale)
		if !errors.Is(err, ErrCallbackDecisionConflict) || !errors.Is(err, stale) {
			t.Fatalf("error=%v want callback conflict+stale", err)
		}
	})

	t.Run("exact receipt converges", func(t *testing.T) {
		if err := reconcileStaleCallbackDecision(executionWithReceipt(t, expected), nil, expected.EventID, expected, stale); err != nil {
			t.Fatalf("exact receipt did not converge: %v", err)
		}
	})
}

func TestCallbackReceiptParserRejectsInvalidReceiptFields(t *testing.T) {
	valid := testCallbackReceipt(t, "event-1")
	cases := []struct {
		name   string
		mutate func(*callbackDecisionReceipt)
	}{
		{name: "version", mutate: func(r *callbackDecisionReceipt) { r.Version++ }},
		{name: "event", mutate: func(r *callbackDecisionReceipt) { r.EventID = " " }},
		{name: "subject", mutate: func(r *callbackDecisionReceipt) { r.Subject = " " }},
		{name: "decision", mutate: func(r *callbackDecisionReceipt) { r.Decision = "" }},
		{name: "digest", mutate: func(r *callbackDecisionReceipt) { r.Digest = " " }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			receipt := valid
			tc.mutate(&receipt)
			_, ok, err := callbackReceiptFromExecution(executionWithReceipt(t, receipt))
			if err == nil || ok {
				t.Fatalf("invalid receipt accepted: receipt=%+v ok=%v err=%v", receipt, ok, err)
			}
		})
	}
}
