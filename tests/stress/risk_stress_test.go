package stress_test

import (
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/domain/incident"
	"github.com/Homiakus/Home_Sentinel/internal/policy/risk"
)

func TestRiskAssessmentStressUnderHeavyLoad(t *testing.T) {
	policy := risk.DefaultPolicy()
	workers := runtime.NumCPU() * 4
	if workers < 8 {
		workers = 8
	}
	const totalOps = 500000

	opsPerWorker := totalOps / workers
	var completed uint64
	var criticalCount uint64
	var highCount uint64
	var mediumCount uint64
	var lowCount uint64

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w + 1000)))
			identities := []incident.IdentityState{
				incident.IdentityUnknown,
				incident.IdentityUncertain,
				incident.IdentityKnown,
			}
			alarmModes := []string{"away", "armed_away", "home", "disarmed", "vacation"}

			for i := 0; i < opsPerWorker; i++ {
				features := risk.Features{
					DetectionConfidence: rng.Float64(),
					EvidenceCount:       rng.Intn(10),
					PersonDetected:      rng.Float32() < 0.6,
					Identity:            identities[rng.Intn(len(identities))],
					AlarmMode:           alarmModes[rng.Intn(len(alarmModes))],
					EntryActive:         rng.Float32() < 0.3,
					DwellSeconds:        rng.Float64() * 300,
					CrossCameraMatches:  rng.Intn(5),
				}

				first, err := policy.Assess(features)
				if err != nil {
					t.Errorf("worker %d assess error: %v", w, err)
					return
				}

				// Determinism verification under concurrent load
				second, err := policy.Assess(features)
				if err != nil {
					t.Errorf("worker %d second assess error: %v", w, err)
					return
				}
				if first.Score != second.Score || first.Risk != second.Risk {
					t.Errorf("non-deterministic scoring detected: %f vs %f", first.Score, second.Score)
					return
				}

				switch first.Risk {
				case incident.RiskCritical:
					atomic.AddUint64(&criticalCount, 1)
				case incident.RiskHigh:
					atomic.AddUint64(&highCount, 1)
				case incident.RiskMedium:
					atomic.AddUint64(&mediumCount, 1)
				case incident.RiskLow:
					atomic.AddUint64(&lowCount, 1)
				}
				atomic.AddUint64(&completed, 1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	opsPerSec := float64(completed) / elapsed.Seconds()

	t.Logf("Risk Stress Test Completed: %d operations across %d workers in %v (%.2f ops/sec)",
		completed, workers, elapsed, opsPerSec)
	t.Logf("Distribution: Critical=%d, High=%d, Medium=%d, Low=%d",
		criticalCount, highCount, mediumCount, lowCount)

	if completed != uint64(workers*opsPerWorker) {
		t.Fatalf("expected %d completed ops, got %d", workers*opsPerWorker, completed)
	}
}
