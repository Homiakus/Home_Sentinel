package authz

import (
	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"testing"
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
