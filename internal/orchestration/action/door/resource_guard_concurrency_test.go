package door

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	"github.com/Homiakus/Home_Sentinel/internal/gateway"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/Home_Sentinel/internal/orchestration/resourceguard"
)

func TestConcurrentStartsReserveSameDoorExactlyOnce(t *testing.T) {
	controller := gatewayfake.NewDoorController(map[string]gateway.LockState{"front": gateway.LockLocked})
	service := openMemory(t, controller)

	const count = 16
	results := make(chan error, count)
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, err := service.Start(context.Background(), domainaction.DoorRequest{
				RequestID: fmt.Sprintf("concurrent-%02d", i),
				DoorID:    "front",
				Desired:   gateway.LockUnlocked,
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	busy := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, resourceguard.ErrBusy):
			busy++
		default:
			t.Fatalf("unexpected start error: %v", err)
		}
	}
	if successes != 1 || busy != count-1 {
		t.Fatalf("reservation race: successes=%d busy=%d", successes, busy)
	}
}
