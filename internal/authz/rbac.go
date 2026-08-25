package authz

import "github.com/Homiakus/Home_Sentinel/internal/auth"

type Capability string

const (
	ViewSystem          Capability = "system:view"
	ViewLive            Capability = "camera:live"
	AcknowledgeIncident Capability = "incident:ack"
	ChangeConfig        Capability = "config:change"
	RunBackup           Capability = "backup:run"
	UnlockDoor          Capability = "door:unlock"
	ManageUsers         Capability = "users:manage"
)

var grants = map[auth.Role]map[Capability]bool{
	auth.RoleViewer:   {ViewSystem: true, ViewLive: true},
	auth.RoleOperator: {ViewSystem: true, ViewLive: true, AcknowledgeIncident: true, RunBackup: true},
	auth.RoleAdmin:    {ViewSystem: true, ViewLive: true, AcknowledgeIncident: true, ChangeConfig: true, RunBackup: true, UnlockDoor: true, ManageUsers: true},
}

func Allowed(role auth.Role, cap Capability) bool { return grants[role][cap] }
