package authz

import (
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
)

func TestRBAC(t *testing.T) {
	if !Allowed(auth.RoleViewer, ViewLive) {
		t.Fatal("viewer should view live")
	}
	if Allowed(auth.RoleViewer, UnlockDoor) {
		t.Fatal("viewer may not unlock")
	}
	if !Allowed(auth.RoleAdmin, UnlockDoor) || !Allowed(auth.RoleAdmin, ManageUsers) {
		t.Fatal("admin grants incomplete")
	}
}

func TestIncidentCapabilitiesAreRiskSeparated(t *testing.T) {
	if Allowed(auth.RoleViewer, AcknowledgeIncident) || Allowed(auth.RoleViewer, ResolveIncident) {
		t.Fatal("viewer may not acknowledge or resolve incidents")
	}
	if !Allowed(auth.RoleOperator, AcknowledgeIncident) {
		t.Fatal("operator should acknowledge incidents")
	}
	if Allowed(auth.RoleOperator, ResolveIncident) {
		t.Fatal("operator may not resolve high-risk incident decisions")
	}
	if !Allowed(auth.RoleAdmin, AcknowledgeIncident) || !Allowed(auth.RoleAdmin, ResolveIncident) {
		t.Fatal("admin incident grants incomplete")
	}
}
