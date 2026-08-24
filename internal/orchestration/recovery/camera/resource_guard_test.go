package camera

import (
	"context"
	"errors"
	"testing"

	domainrecovery "github.com/Homiakus/Home_Sentinel/internal/domain/recovery"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/resourceguard"
)

func TestSameCameraCannotStartConcurrentRecovery(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"front": true, "rear": true},
		map[string]bool{"front": false, "rear": false},
	)
	service := openMemory(t, controller)
	if _, err := service.Start(ctx, domainrecovery.CameraRequest{RequestID: "recover-1", CameraID: "front"}); err != nil {
		t.Fatalf("start first: %v", err)
	}
	if _, err := service.Start(ctx, domainrecovery.CameraRequest{RequestID: "recover-2", CameraID: "front"}); !errors.Is(err, resourceguard.ErrBusy) {
		t.Fatalf("same camera was not reserved: %v", err)
	}
	if _, err := service.Start(ctx, domainrecovery.CameraRequest{RequestID: "recover-3", CameraID: "rear"}); err != nil {
		t.Fatalf("different camera blocked: %v", err)
	}
}
