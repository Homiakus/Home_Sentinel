package incident

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/axiom/adgo"
)

var ErrCallbackDecisionConflict = errors.New("incident: callback decision conflicts with durable receipt")

const callbackDecisionReceiptVersion = 1

type callbackDecisionReceipt struct {
	Version  int                     `json:"version"`
	EventID  string                  `json:"event_id"`
	Subject  string                  `json:"subject"`
	Decision domainincident.Decision `json:"decision"`
	Digest   string                  `json:"digest"`
}

type callbackDecisionEnvelope struct {
	Receipt callbackDecisionReceipt `json:"callback_receipt"`
	Payload any                     `json:"payload,omitempty"`
}

type callbackDecisionInput struct {
	Version  int                     `json:"version"`
	Subject  string                  `json:"subject"`
	Decision domainincident.Decision `json:"decision"`
	Reason   string                  `json:"reason"`
	Payload  any                     `json:"payload,omitempty"`
}

// ResolveOwnerCallbackDecision applies a human decision with a durable receipt.
// The receipt turns a one-shot callback into repeatable exactly-once semantics:
// the same event and semantic input return the already-decided execution, while
// reuse of the event id with different input fails closed as a conflict.
func (s *Service) ResolveOwnerCallbackDecision(
	ctx context.Context,
	executionID string,
	eventID string,
	decision domainincident.Decision,
	actor string,
	reason string,
	payload any,
) (*adgo.Execution, error) {
	executionID = strings.TrimSpace(executionID)
	eventID = strings.TrimSpace(eventID)
	actor = strings.TrimSpace(actor)
	reason = strings.TrimSpace(reason)
	if executionID == "" {
		return nil, errors.New("incident: callback execution id is required")
	}
	if eventID == "" {
		return nil, errors.New("incident: callback event id is required")
	}
	if actor == "" {
		return nil, errors.New("incident: callback actor is required")
	}

	mapped, err := mapDecision(decision)
	if err != nil {
		return nil, err
	}
	bundle, _, err := s.executionBundle(ctx, executionID)
	if err != nil {
		return nil, err
	}
	target := strings.TrimSpace(bundle.bindings.ownerDecisionNode)
	if target == "" {
		return nil, fmt.Errorf("%w: callback owner decision for plan version %s", ErrBundleOperationUnsupported, bundle.plan.Version)
	}
	envelope, err := newCallbackDecisionEnvelope(eventID, actor, decision, reason, payload)
	if err != nil {
		return nil, err
	}

	// Preflight is required for decisions such as retry that may put the same
	// human node back into a waiting state. Without this check, a duplicate
	// delivery could resolve that new wait a second time before stale detection.
	current, err := s.Get(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if receipt, ok, receiptErr := callbackReceiptFromExecution(current); receiptErr != nil {
		return nil, receiptErr
	} else if ok && receipt.EventID == eventID {
		if !sameCallbackDecision(receipt, envelope.Receipt) {
			return nil, callbackDecisionConflict(eventID)
		}
		return s.Drive(ctx, executionID)
	}

	if _, err := bundle.engine.ResolveHuman(ctx, executionID, target, adgo.HumanResolution{
		Decision: mapped,
		Actor:    actor,
		Reason:   reason,
		Payload:  envelope,
	}); err != nil {
		if !errors.Is(err, adgo.ErrStaleTask) {
			return nil, err
		}
		// A concurrent delivery may have won between the preflight Get and the
		// durable mutation. Re-read the receipt and converge on that result.
		return s.finishStaleCallbackDecision(ctx, executionID, eventID, envelope.Receipt, err)
	}
	return s.Drive(ctx, executionID)
}

func (s *Service) finishStaleCallbackDecision(
	ctx context.Context,
	executionID string,
	eventID string,
	expected callbackDecisionReceipt,
	staleErr error,
) (*adgo.Execution, error) {
	loaded, loadErr := s.Get(ctx, executionID)
	if reconcileErr := reconcileStaleCallbackDecision(loaded, loadErr, eventID, expected, staleErr); reconcileErr != nil {
		return nil, reconcileErr
	}
	return s.Drive(ctx, executionID)
}

func newCallbackDecisionEnvelope(
	eventID string,
	subject string,
	decision domainincident.Decision,
	reason string,
	payload any,
) (callbackDecisionEnvelope, error) {
	input := callbackDecisionInput{
		Version:  callbackDecisionReceiptVersion,
		Subject:  subject,
		Decision: decision,
		Reason:   reason,
		Payload:  payload,
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return callbackDecisionEnvelope{}, fmt.Errorf("encode callback decision input: %w", err)
	}
	sum := sha256.Sum256(raw)
	return callbackDecisionEnvelope{
		Receipt: callbackDecisionReceipt{
			Version:  callbackDecisionReceiptVersion,
			EventID:  eventID,
			Subject:  subject,
			Decision: decision,
			Digest:   hex.EncodeToString(sum[:]),
		},
		Payload: payload,
	}, nil
}

func callbackHumanPayloadKey() string {
	return "human:" + NodeHumanDecision + ":payload"
}

func callbackReceiptFromExecution(execution *adgo.Execution) (callbackDecisionReceipt, bool, error) {
	if execution == nil || execution.Data == nil {
		return callbackDecisionReceipt{}, false, nil
	}
	raw, ok := execution.Data[callbackHumanPayloadKey()]
	if !ok || len(raw) == 0 {
		return callbackDecisionReceipt{}, false, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		// Legacy human payloads may be arrays, scalars or arbitrary objects. They
		// predate callback receipts and must retain stale-task behavior.
		return callbackDecisionReceipt{}, false, nil
	}
	receiptRaw, ok := object["callback_receipt"]
	if !ok {
		return callbackDecisionReceipt{}, false, nil
	}
	var receipt callbackDecisionReceipt
	if err := json.Unmarshal(receiptRaw, &receipt); err != nil {
		return callbackDecisionReceipt{}, false, fmt.Errorf("decode durable callback receipt: %w", err)
	}
	if receipt.Version != callbackDecisionReceiptVersion ||
		strings.TrimSpace(receipt.EventID) == "" ||
		strings.TrimSpace(receipt.Subject) == "" ||
		receipt.Decision == "" ||
		strings.TrimSpace(receipt.Digest) == "" {
		return callbackDecisionReceipt{}, false, errors.New("incident: durable callback receipt is invalid")
	}
	return receipt, true, nil
}

func reconcileStaleCallbackDecision(
	loaded *adgo.Execution,
	loadErr error,
	eventID string,
	expected callbackDecisionReceipt,
	staleErr error,
) error {
	if loadErr != nil {
		return errors.Join(staleErr, loadErr)
	}
	receipt, ok, receiptErr := callbackReceiptFromExecution(loaded)
	if receiptErr != nil {
		return errors.Join(staleErr, receiptErr)
	}
	if !ok || receipt.EventID != eventID {
		return staleErr
	}
	if !sameCallbackDecision(receipt, expected) {
		return callbackDecisionConflict(eventID)
	}
	return nil
}

func sameCallbackDecision(left, right callbackDecisionReceipt) bool {
	return left.Version == right.Version &&
		left.EventID == right.EventID &&
		left.Subject == right.Subject &&
		left.Decision == right.Decision &&
		left.Digest == right.Digest
}

func callbackDecisionConflict(eventID string) error {
	return errors.Join(
		ErrCallbackDecisionConflict,
		adgo.ErrStaleTask,
		fmt.Errorf("incident: callback event %q already resolved with different semantic input", eventID),
	)
}
