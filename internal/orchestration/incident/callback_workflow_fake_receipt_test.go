package incident

import (
	"context"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/axiom/adgo"
)

func (f *callbackWorkflowFake) ResolveOwnerCallbackDecision(
	_ context.Context,
	executionID string,
	eventID string,
	decision domainincident.Decision,
	actor string,
	reason string,
	payload any,
) (*adgo.Execution, error) {
	f.decisionCalls++
	f.executionID = executionID
	f.eventID = eventID
	f.decision = decision
	f.actor = actor
	f.reason = reason
	f.payload = payload
	return &adgo.Execution{ID: executionID}, nil
}
