package camera

import (
	"context"
	"testing"

	domainrecovery "github.com/Homiakus/Home_Sentinel/internal/domain/recovery"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func openMemory(t *testing.T, controller *gatewayfake.CameraRecoveryController) *Service {
	t.Helper()
	service, err := Open(Config{
		Production: adgo.ProductionConfig{Backend: adgo.BackendMemory},
		WorkerID:   "camera-recovery-test", WorkerConcurrency: 1,
	}, Dependencies{Controller: controller})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func TestPlanCompiles(t *testing.T) {
	if _, err := CompilePlan(); err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestHealthyCameraDoesNotReconnect(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewCameraRecoveryController(map[string]bool{"front": true}, map[string]bool{"front": true})
	service := openMemory(t, controller)
	execution, err := service.Start(ctx, domainrecovery.CameraRequest{RequestID: "health-1", CameraID: "front"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusCompleted || controller.ReconnectCalls != 0 {
		t.Fatalf("healthy path unexpected: status=%s reconnects=%d", execution.Status, controller.ReconnectCalls)
	}
}

func TestBrokenStreamReconnectsAndVerifies(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewCameraRecoveryController(map[string]bool{"front": true}, map[string]bool{"front": false})
	service := openMemory(t, controller)
	execution, err := service.Start(ctx, domainrecovery.CameraRequest{RequestID: "recover-1", CameraID: "front"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusCompleted || controller.ReconnectCalls != 1 {
		t.Fatalf("recovery path failed: status=%s reconnects=%d", execution.Status, controller.ReconnectCalls)
	}
}

func TestNetworkFailureEscalatesThenAllowsOneExplicitRetry(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewCameraRecoveryController(map[string]bool{"front": false}, map[string]bool{"front": false})
	service := openMemory(t, controller)
	request := domainrecovery.CameraRequest{RequestID: "recover-network", CameraID: "front"}
	execution, err := service.Start(ctx, request)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	execution, err = service.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if execution.Status != adgo.StatusHuman || execution.WaitingFor[NodeOperator] != OperatorEvent {
		t.Fatalf("network outage did not escalate: status=%s waiting=%v", execution.Status, execution.WaitingFor)
	}

	controller.SetNetwork("front", true)
	execution, err = service.ResolveOperator(ctx, execution.ID, domainrecovery.OperatorRetry, "operator", "network restored")
	if err != nil {
		t.Fatalf("operator retry: %v", err)
	}
	if execution.Status != adgo.StatusCompleted || controller.ReconnectCalls != 1 {
		t.Fatalf("operator recovery failed: status=%s reconnects=%d", execution.Status, controller.ReconnectCalls)
	}
}

func TestOperatorRejectIsTerminalAndAuditable(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewCameraRecoveryController(map[string]bool{"front": false}, map[string]bool{"front": false})
	service := openMemory(t, controller)
	execution, _ := service.Start(ctx, domainrecovery.CameraRequest{RequestID: "reject-1", CameraID: "front"})
	execution, _ = service.Drive(ctx, execution.ID)
	execution, err := service.ResolveOperator(ctx, execution.ID, domainrecovery.OperatorReject, "operator", "camera intentionally offline")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if execution.Status != adgo.StatusCompleted || execution.Nodes[NodeRejected].Status != adgo.NodeCompleted {
		t.Fatalf("reject branch failed: status=%s node=%+v", execution.Status, execution.Nodes[NodeRejected])
	}
}
