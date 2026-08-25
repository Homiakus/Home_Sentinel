package stress_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	dooraction "github.com/Homiakus/Home_Sentinel/internal/orchestration/action/door"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/resourceguard"
	"github.com/Homiakus/axiom/adgo"
)

type testPayload struct {
	Resource string `json:"resource"`
}

func testResourceResolver(execution *adgo.Execution) (string, error) {
	if execution == nil || execution.Data == nil {
		return "", errors.New("missing data")
	}
	raw, ok := execution.Data["request"]
	if !ok {
		return "", errors.New("missing request")
	}
	var p testPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", err
	}
	return p.Resource, nil
}

func TestResourceGuardHighContentionStress(t *testing.T) {
	ctx := context.Background()
	store := adgo.NewMemoryStore()

	const numResources = 5
	const contestantsPerResource = 40
	const totalRounds = 5

	for round := 0; round < totalRounds; round++ {
		var wg sync.WaitGroup
		var mu sync.Mutex

		// For each resource, exactly one winner is expected in each concurrent batch
		winners := make(map[string]string)
		var busyCount int64
		var winCount int64

		for resIdx := 0; resIdx < numResources; resIdx++ {
			resName := fmt.Sprintf("lock:door-zone-%d", resIdx)

			for c := 0; c < contestantsPerResource; c++ {
				execID := fmt.Sprintf("round-%d-res-%d-contestant-%03d", round, resIdx, c)
				wg.Add(1)

				go func(resource, id string) {
					defer wg.Done()

					payloadBytes, _ := json.Marshal(testPayload{Resource: resource})

					// Simulate the check-and-reserve transaction critical section
					mu.Lock()
					err := resourceguard.Check(ctx, store, "plan-door", id, resource, testResourceResolver)
					if err == nil {
						createErr := store.Create(ctx, &adgo.Execution{
							ID:          id,
							PlanID:      "plan-door",
							PlanVersion: "1.0",
							Status:      adgo.StatusRunning,
							Data:        map[string]json.RawMessage{"request": payloadBytes},
						})
						if createErr != nil {
							mu.Unlock()
							t.Errorf("store create failed: %v", createErr)
							return
						}
						winners[resource] = id
						atomic.AddInt64(&winCount, 1)
						mu.Unlock()
					} else if errors.Is(err, resourceguard.ErrBusy) {
						atomic.AddInt64(&busyCount, 1)
						mu.Unlock()
					} else {
						mu.Unlock()
						t.Errorf("unexpected guard error: %v", err)
					}
				}(resName, execID)
			}
		}

		wg.Wait()

		if winCount != int64(numResources) {
			t.Fatalf("round %d: expected %d winners, got %d", round, numResources, winCount)
		}
		expectedBusy := int64(numResources * (contestantsPerResource - 1))
		if busyCount != expectedBusy {
			t.Fatalf("round %d: expected %d busy rejections, got %d", round, expectedBusy, busyCount)
		}

		// Now terminate all winners and verify resources are released for next round
		for _, winnerID := range winners {
			cur, loadErr := store.Load(ctx, winnerID)
			if loadErr != nil {
				t.Fatalf("load winner %s: %v", winnerID, loadErr)
			}
			_, err := store.Commit(ctx, winnerID, cur.Version, func(exec *adgo.Execution) error {
				exec.Status = adgo.StatusCompleted
				return nil
			})
			if err != nil {
				t.Fatalf("failed to complete winner %s: %v", winnerID, err)
			}
		}
	}

	t.Logf("ResourceGuard stress completed: %d rounds, %d total requests successfully arbitrated",
		totalRounds, numResources*contestantsPerResource*totalRounds)
}

func TestDoorServicePebbleDurableStress(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{
		"front_door": gateway.LockLocked,
		"back_door":  gateway.LockLocked,
	})

	service, err := dooraction.Open(dooraction.Config{
		Production: adgo.DefaultProductionConfig(root),
		WorkerID:   "stress-door-worker",
	}, dooraction.Dependencies{Door: controller})
	if err != nil {
		t.Fatalf("open door service with pebble backend: %v", err)
	}
	defer service.Close()

	const concurrentRuns = 30
	var wg sync.WaitGroup
	wg.Add(concurrentRuns)

	var successCount int64
	var busyCount int64

	for i := 0; i < concurrentRuns; i++ {
		reqID := fmt.Sprintf("stress-req-%03d", i)
		go func(id string) {
			defer wg.Done()

			req := domainaction.DoorRequest{
				RequestID:   id,
				DoorID:      "front_door",
				Desired:     gateway.LockUnlocked,
				RequestedBy: "stress-test",
			}
			exec, startErr := service.Start(ctx, req)
			if startErr == nil {
				atomic.AddInt64(&successCount, 1)
				// Drive execution to human gate
				_, _ = service.Drive(ctx, exec.ID)
			} else if errors.Is(startErr, resourceguard.ErrBusy) {
				atomic.AddInt64(&busyCount, 1)
			} else {
				t.Errorf("unexpected start error: %v", startErr)
			}
		}(reqID)
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful reservation, got %d", successCount)
	}
	if busyCount != concurrentRuns-1 {
		t.Fatalf("expected %d busy rejections, got %d", concurrentRuns-1, busyCount)
	}

	t.Logf("Pebble Durable Door Service stress passed: 1 winner, %d busy rejections", busyCount)
}
