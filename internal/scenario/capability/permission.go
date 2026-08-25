package capability

import "fmt"

type Permission string

type Role string

const (
	RoleViewer        Role = "viewer"
	RoleOperator      Role = "operator"
	RoleSecurityAdmin Role = "security-admin"
	RoleSystem        Role = "system"
)

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityOperator Visibility = "operator"
	VisibilityAdmin    Visibility = "admin"
	VisibilityInternal Visibility = "internal"
)

func (v Visibility) valid() bool {
	switch v {
	case VisibilityPublic, VisibilityOperator, VisibilityAdmin, VisibilityInternal:
		return true
	default:
		return false
	}
}

func visibleTo(visibility Visibility, role Role) bool {
	switch visibility {
	case VisibilityPublic:
		return role == RoleViewer || role == RoleOperator || role == RoleSecurityAdmin
	case VisibilityOperator:
		return role == RoleOperator || role == RoleSecurityAdmin
	case VisibilityAdmin:
		return role == RoleSecurityAdmin
	case VisibilityInternal:
		return role == RoleSystem
	default:
		return false
	}
}

func validateRole(role Role) error {
	switch role {
	case RoleViewer, RoleOperator, RoleSecurityAdmin, RoleSystem:
		return nil
	default:
		return fmt.Errorf("capability: invalid role %q", role)
	}
}
