package stress_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domainincident "github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	domainrecovery "github.com/Homiakus/Home_Sentinel/internal/domain/recovery"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	incidentaction "github.com/Homiakus/Home_Sentinel/internal/orchestration/incident"
	cameraaction "github.com/Homiakus/Home_Sentinel/internal/orchestration/recovery/camera"
	"github.com/Homiakus/axiom/adgo"
)

func TestIncidentDurableLifecycleStress(t *testing.T) {
	ctx := context.Background()
	notifier := gatewayfake.NewNotifier()
	service, err := incidentaction.Open(incidentaction.Config{
		Production:        adgo.ProductionConfig{Backend: adgo.BackendMemory},
		WorkerID:          "incident-stress-worker",
		WorkerConcurrency: 8,
	}, incidentaction.Dependencies{Notifier: notifier})
	if err != nil {
		t.Fatalf("open incident service: %v", err)
	}
	defer service.Close()

	const concurrentIncidents = 50
	var wg sync.WaitGroup
	wg.Add(concurrentIncidents)

	var createdCount uint64
	var completedCount uint64

	start := time.Now()

	for i := 0; i < concurrentIncidents; i++ {
		idx := i
		go func() {
			defer wg.Done()

			trigger := domainincident.Trigger{
				EventID:    fmt.Sprintf("evt-stress-%04d", idx),
				SourceID:   fmt.Sprintf("cam-zone-%d", idx%5),
				Kind:       "vision.person.detected.v1",
				OccurredAt: time.Now().UTC(),
				Confidence: 0.95,
			}

			// 1. Start incident
			exec, startErr := service.Start(ctx, trigger)
			if startErr != nil {
				t.Errorf("incident %d start failed: %v", idx, startErr)
				return
			}
			atomic.AddUint64(&createdCount, 1)

			// 2. Drive to human wait state
			exec, driveErr := service.Drive(ctx, exec.ID)
			if driveErr != nil {
				t.Errorf("incident %d drive failed: %v", idx, driveErr)
				return
			}
			if exec.Status != adgo.StatusWaiting {
				t.Errorf("incident %d expected status waiting, got %s", idx, exec.Status)
				return
			}

			// 3. Owner response resolving human gate
			completed, resErr := service.OwnerResponse(ctx, exec.ID, "owner-ack", nil)
			if resErr != nil {
				t.Errorf("incident %d owner response failed: %v", idx, resErr)
				return
			}
			if completed.Status != adgo.StatusCompleted {
				t.Errorf("incident %d expected completed status, got %s", idx, completed.Status)
				return
			}
			atomic.AddUint64(&completedCount, 1)
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	if createdCount != concurrentIncidents || completedCount != concurrentIncidents {
		t.Fatalf("incident lifecycle stress mismatch: created=%d completed=%d expected=%d",
			createdCount, completedCount, concurrentIncidents)
	}

	t.Logf("Incident Durable Lifecycle Stress: %d full lifecycle executions in %v (%.2f incidents/sec)",
		completedCount, elapsed, float64(completedCount)/elapsed.Seconds())
}

func TestCameraRecoveryChaosAndStress(t *testing.T) {
	ctx := context.Background()
	controller := gatewayfake.NewCameraRecoveryController(
		map[string]bool{"cam_0": true, "cam_1": true, "cam_2": true},
		map[string]bool{"cam_0": false, "cam_1": false, "cam_2": true},
	)

	service, err := cameraaction.Open(cameraaction.Config{
		Production:        adgo.ProductionConfig{Backend: adgo.BackendMemory},
		WorkerID:          "camera-recovery-stress",
		WorkerConcurrency: 4,
	}, cameraaction.Dependencies{Controller: controller})
	if err != nil {
		t.Fatalf("open camera recovery service: %v", err)
	}
	defer service.Close()

	const iterations = 30
	var successCount uint64

	start := time.Now()

	for i := 0; i < iterations; i++ {
		camID := fmt.Sprintf("cam_%d", i%3)
		reqID := fmt.Sprintf("rec-req-%04d", i)

		exec, startErr := service.Start(ctx, domainrecovery.CameraRequest{
			RequestID: reqID,
			CameraID:  camID,
		})
		if startErr != nil {
			t.Fatalf("camera recovery start failed: %v", startErr)
		}

		// Drive recovery workflow
		completed, driveErr := service.Drive(ctx, exec.ID)
		if driveErr != nil {
			t.Fatalf("camera recovery drive failed: %v", driveErr)
		}
		if completed.Status != adgo.StatusCompleted {
			t.Fatalf("expected completed status, got %s", completed.Status)
		}
		atomic.AddUint64(&successCount, 1)
	}

	elapsed := time.Since(start)
	t.Logf("Camera Recovery Stress: %d recovery cycles successfully executed in %v", successCount, elapsed)
}
