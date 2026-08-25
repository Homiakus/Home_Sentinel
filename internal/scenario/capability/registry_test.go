package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
)

func TestDuplicateRegistrationFails(t *testing.T) {
	registry := NewRegistry(NoDependencies{})
	descriptor := cameraTrigger("1.0.0")
	if err := registry.Register(descriptor); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(descriptor); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("expected ErrAlreadyRegistered, got %v", err)
	}
}

func TestResolveCompatibleChoosesHighestCompatibleVersion(t *testing.T) {
	registry := NewRegistry(NoDependencies{})
	for _, version := range []string{"1.0.0", "1.3.0", "2.0.0"} {
		if err := registry.Register(cameraTrigger(version)); err != nil {
			t.Fatal(err)
		}
	}
	resolved, ok := registry.ResolveCompatible("camera.person.detected", "1.0.0")
	if !ok || resolved.Version != "1.3.0" {
		t.Fatalf("unexpected resolution: %#v, %v", resolved, ok)
	}
	if _, ok := registry.ResolveCompatible("camera.person.detected", "3.0.0"); ok {
		t.Fatal("resolved incompatible major")
	}
}

func TestEntityCapabilityDiscoveryAndRoleFiltering(t *testing.T) {
	registry := NewRegistry(NoDependencies{})
	public := cameraTrigger("1.0.0")
	admin := cameraAction("camera.reconnect", VisibilityAdmin)
	if err := registry.Register(public); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(admin); err != nil {
		t.Fatal(err)
	}
	entity := EntityDescriptor{
		ID: "front-camera", Kind: "camera", ProviderID: "rtsp", IntegrationID: "rtsp-main", Title: "Front camera",
		Capabilities: []Key{public.Key(), admin.Key()}, Visibility: VisibilityPublic, Available: true, Health: HealthHealthy,
	}
	if err := registry.RegisterEntity(entity); err != nil {
		t.Fatal(err)
	}
	viewer, err := registry.List(Filter{Role: RoleViewer, EntityID: entity.ID, IncludeUnavailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(viewer) != 1 || viewer[0].ID != public.ID {
		t.Fatalf("viewer received wrong capabilities: %#v", viewer)
	}
	adminList, err := registry.List(Filter{Role: RoleSecurityAdmin, EntityID: entity.ID, IncludeUnavailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(adminList) != 2 {
		t.Fatalf("admin expected two capabilities, got %d", len(adminList))
	}
}

func TestUnavailableCapabilityIsNotDeleted(t *testing.T) {
	registry := NewRegistry(NoDependencies{})
	descriptor := cameraTrigger("1.0.0")
	if err := registry.Register(descriptor); err != nil {
		t.Fatal(err)
	}
	if err := registry.SetCapabilityHealth(descriptor.Key(), false, HealthOffline); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Get(descriptor.ID, descriptor.Version); !ok {
		t.Fatal("offline capability disappeared from registry")
	}
	visible, err := registry.List(Filter{Role: RoleViewer})
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 0 {
		t.Fatal("offline capability should be hidden without IncludeUnavailable")
	}
	visible, err = registry.List(Filter{Role: RoleViewer, IncludeUnavailable: true})
	if err != nil || len(visible) != 1 || visible[0].Health != HealthOffline {
		t.Fatalf("offline capability was not discoverable: %#v, %v", visible, err)
	}
}

func TestRemovalFailsWhenScenarioDependsOnCapability(t *testing.T) {
	usage := &fakeUsage{capabilityUses: []ScenarioUse{{ScenarioID: model.ID("scenario-security"), Version: 3}}}
	registry := NewRegistry(usage)
	descriptor := cameraTrigger("1.0.0")
	if err := registry.Register(descriptor); err != nil {
		t.Fatal(err)
	}
	if err := registry.Remove(context.Background(), descriptor.Key()); !errors.Is(err, ErrInUse) {
		t.Fatalf("expected ErrInUse, got %v", err)
	}
}

func TestRemovalFailsWhenEntityStillReferencesCapability(t *testing.T) {
	registry := NewRegistry(NoDependencies{})
	descriptor := cameraTrigger("1.0.0")
	if err := registry.Register(descriptor); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterEntity(EntityDescriptor{
		ID: "front-camera", Kind: "camera", ProviderID: "rtsp", IntegrationID: "rtsp-main", Title: "Front camera",
		Capabilities: []Key{descriptor.Key()}, Visibility: VisibilityPublic, Available: true, Health: HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Remove(context.Background(), descriptor.Key()); !errors.Is(err, ErrEntityReference) {
		t.Fatalf("expected ErrEntityReference, got %v", err)
	}
}

func TestRegistryDigestIsIndependentOfRegistrationOrder(t *testing.T) {
	left := NewRegistry(NoDependencies{})
	right := NewRegistry(NoDependencies{})
	first := cameraTrigger("1.0.0")
	second := cameraAction("camera.reconnect", VisibilityAdmin)
	if err := left.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := left.Register(second); err != nil {
		t.Fatal(err)
	}
	if err := right.Register(second); err != nil {
		t.Fatal(err)
	}
	if err := right.Register(first); err != nil {
		t.Fatal(err)
	}
	leftDigest, err := left.Digest()
	if err != nil {
		t.Fatal(err)
	}
	rightDigest, err := right.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if leftDigest != rightDigest {
		t.Fatalf("registry digest differs: %s != %s", leftDigest, rightDigest)
	}
}

func cameraTrigger(version string) Descriptor {
	return Descriptor{
		ID: "camera.person.detected", Version: version, ProviderID: "rtsp", IntegrationID: "rtsp-main",
		Kind: KindTrigger, Category: "security", Title: "Person detected", EntityKinds: []string{"camera"},
		Risk: model.RiskLow, Permission: "camera.read", Visibility: VisibilityPublic,
		Idempotency: IdempotencyNone, Verification: VerificationNone, Compensation: CompensationNone,
		Available: true, Health: HealthHealthy,
	}
}

func cameraAction(id string, visibility Visibility) Descriptor {
	return Descriptor{
		ID: id, Version: "1.0.0", ProviderID: "rtsp", IntegrationID: "rtsp-main",
		Kind: KindAction, Category: "recovery", Title: "Reconnect camera", EntityKinds: []string{"camera"},
		Risk: model.RiskMedium, Permission: "camera.recovery", Visibility: visibility,
		ExternalEffect: true, Idempotency: IdempotencyRequired, Verification: VerificationReadback,
		Compensation: CompensationReconcile, Available: true, Health: HealthHealthy,
	}
}

type fakeUsage struct {
	capabilityUses []ScenarioUse
	entityUses     []ScenarioUse
}

func (f *fakeUsage) UsesCapability(context.Context, Key) ([]ScenarioUse, error) {
	return append([]ScenarioUse(nil), f.capabilityUses...), nil
}

func (f *fakeUsage) UsesEntity(context.Context, string) ([]ScenarioUse, error) {
	return append([]ScenarioUse(nil), f.entityUses...), nil
}
