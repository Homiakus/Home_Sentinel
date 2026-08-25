package catalog

import (
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/compiler"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

func setupTestCatalog(t *testing.T) (*Catalog, *capability.Registry) {
	reg := capability.NewRegistry(nil)

	notifDesc, _ := capability.NewActionDescriptor("notification.send", "1.0.0", "core", "notify", "alert", "Send Notification", "notification:send")
	notifDesc.Input = capability.Schema{
		Fields: []capability.FieldSchema{
			{Name: "title", Type: model.TypeRef{Kind: model.TypeString}, Required: true},
		},
	}
	_ = reg.Register(notifDesc)

	camDesc, _ := capability.NewTriggerDescriptor("camera.motion", "1.0.0", "frigate", "nvr", "vision", "Motion", "camera:read")
	camDesc.EntityKinds = []string{"camera"}
	_ = reg.Register(camDesc)

	comp := compiler.NewCompiler(reg)
	cat := NewCatalog(comp)
	return cat, reg
}

func TestCatalog_DraftAndPublishLifecycle(t *testing.T) {
	cat, _ := setupTestCatalog(t)

	titleVal, _ := model.ValueOf("Alert 1")
	scen := model.Scenario{
		ID:         "entry-camera-alert",
		RevisionID: "rev-1",
		Name:       "Entry Camera Alert",
		Triggers: []model.Trigger{
			{
				ID:   "trig",
				Kind: model.TriggerDeviceEvent,
				Capability: model.CapabilityRef{
					ID:      "camera.motion",
					Version: "1.0.0",
					Entity:  &model.EntityRef{ID: "front_cam", Kind: "camera"},
				},
			},
		},
		Flow: model.Flow{
			Steps: []model.Step{
				{
					ID:   "step1",
					Kind: model.StepAction,
					Action: &model.ActionStep{
						Capability: model.CapabilityRef{ID: "notification.send", Version: "1.0.0"},
						Arguments:  map[string]model.Expr{"title": model.Literal(titleVal)},
					},
				},
			},
		},
	}

	// 1. Create Draft
	record, rev1, err := cat.CreateDraft(scen, "admin@home.local")
	if err != nil {
		t.Fatalf("CreateDraft failed: %v", err)
	}
	if rev1.State != StateDraft || rev1.Version != 0 {
		t.Fatalf("unexpected draft state: state=%s, version=%d", rev1.State, rev1.Version)
	}

	// 2. Validate Draft
	manifest, diags, err := cat.ValidateDraft(record.ID, rev1.RevisionID)
	if err != nil || diags.HasErrors() {
		t.Fatalf("ValidateDraft failed: err=%v, diags=%v", err, diags.Error())
	}
	if manifest.PlanDigest == "" {
		t.Fatalf("expected PlanDigest to be computed")
	}

	// 3. Publish Draft -> Version 1
	pubRev1, err := cat.PublishDraft(record.ID, rev1.RevisionID, "admin@home.local", "Initial release")
	if err != nil {
		t.Fatalf("PublishDraft failed: %v", err)
	}
	if pubRev1.State != StatePublished || pubRev1.Version != 1 {
		t.Fatalf("expected version 1 published, got version=%d state=%s", pubRev1.Version, pubRev1.State)
	}

	// 4. Immutability check: cannot edit published revision
	_, err = cat.UpdateDraft(record.ID, pubRev1.RevisionID, pubRev1.ETag, scen, "admin@home.local")
	if err != ErrImmutable {
		t.Fatalf("expected ErrImmutable on editing published revision, got: %v", err)
	}

	// 5. Dependency Index check
	canDelete, scens := cat.CanDeleteCapability("camera.motion")
	if canDelete || len(scens) != 1 || scens[0] != "entry-camera-alert" {
		t.Fatalf("expected capability in use by entry-camera-alert, got canDelete=%v, scens=%v", canDelete, scens)
	}

	canDeleteEnt, scensEnt := cat.CanDeleteEntity("camera", "front_cam")
	if canDeleteEnt || len(scensEnt) != 1 {
		t.Fatalf("expected entity in use, got canDelete=%v, scens=%v", canDeleteEnt, scensEnt)
	}

	// 6. Create second draft for revision 2
	titleVal2, _ := model.ValueOf("Alert 2 - Updated")
	scen2 := scen
	scen2.Flow.Steps[0].Action.Arguments["title"] = model.Literal(titleVal2)

	_, rev2, err := cat.CreateDraft(scen2, "admin@home.local")
	if err != nil {
		t.Fatalf("CreateDraft rev2 failed: %v", err)
	}

	// Optimistic concurrency test on update
	_, err = cat.UpdateDraft(record.ID, rev2.RevisionID, "wrong-etag", scen2, "admin@home.local")
	if err == nil {
		t.Fatalf("expected conflict on wrong ETag")
	}

	updatedRev2, err := cat.UpdateDraft(record.ID, rev2.RevisionID, rev2.ETag, scen2, "admin@home.local")
	if err != nil {
		t.Fatalf("UpdateDraft rev2 failed: %v", err)
	}

	// Publish second draft -> Version 2
	pubRev2, err := cat.PublishDraft(record.ID, updatedRev2.RevisionID, "admin@home.local", "Updated title")
	if err != nil {
		t.Fatalf("PublishDraft rev2 failed: %v", err)
	}
	if pubRev2.Version != 2 {
		t.Fatalf("expected version 2, got %d", pubRev2.Version)
	}

	// Active revision should now be v2
	activeRev, err := cat.GetActiveRevision(record.ID)
	if err != nil {
		t.Fatalf("GetActiveRevision failed: %v", err)
	}
	if activeRev.Version != 2 {
		t.Fatalf("expected active version 2, got %d", activeRev.Version)
	}

	// 7. Rollback to Version 1
	rbRev, err := cat.RollbackToVersion(record.ID, 1, "admin@home.local", "Revert to initial stable version")
	if err != nil {
		t.Fatalf("RollbackToVersion 1 failed: %v", err)
	}
	if rbRev.Version != 1 {
		t.Fatalf("expected rollback to version 1, got %d", rbRev.Version)
	}

	activeAfterRb, _ := cat.GetActiveRevision(record.ID)
	if activeAfterRb.Version != 1 {
		t.Fatalf("expected active version 1 after rollback, got %d", activeAfterRb.Version)
	}
}
