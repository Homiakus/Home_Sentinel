package stress_test

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/scenario/capability"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/catalog"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/compiler"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/model"
	"github.com/Homiakus/Home_Sentinel/internal/scenario/simulator"
)

func setupTestCapabilityRegistry(t *testing.T) *capability.Registry {
	reg := capability.NewRegistry(nil)

	notifDesc, _ := capability.NewActionDescriptor("notification.send", "1.0.0", "core", "notify", "alert", "Send Notification", "notification:send")
	notifDesc.Input = capability.Schema{
		Fields: []capability.FieldSchema{
			{Name: "title", Type: model.TypeRef{Kind: model.TypeString}, Required: true},
			{Name: "priority", Type: model.TypeRef{Kind: model.TypeInt}, Required: false},
		},
	}
	_ = reg.Register(notifDesc)

	sirenDesc, _ := capability.NewActionDescriptor("siren.sound", "1.0.0", "core", "alarm", "siren", "Sound Siren", "siren:sound")
	sirenDesc.Risk = model.RiskHigh
	sirenDesc.ExternalEffect = true
	sirenDesc.Idempotency = capability.IdempotencySupported
	sirenDesc.EntityKinds = []string{"siren"}
	_ = reg.Register(sirenDesc)

	doorDesc, _ := capability.NewActionDescriptor("door.unlock", "1.0.0", "core", "access", "door", "Unlock Door", "door:unlock")
	doorDesc.Risk = model.RiskCritical
	doorDesc.ExternalEffect = true
	doorDesc.Idempotency = capability.IdempotencyRequired
	doorDesc.EntityKinds = []string{"door"}
	_ = reg.Register(doorDesc)

	camDesc, _ := capability.NewTriggerDescriptor("camera.person.detected", "1.0.0", "frigate", "nvr", "vision", "Person Detected", "camera:read")
	camDesc.EntityKinds = []string{"camera"}
	_ = reg.Register(camDesc)

	motionDesc, _ := capability.NewTriggerDescriptor("sensor.motion", "1.0.0", "zigbee", "sensors", "motion", "Motion Detected", "sensor:read")
	motionDesc.EntityKinds = []string{"sensor"}
	_ = reg.Register(motionDesc)

	return reg
}

func generateDynamicScenario(id string, complexity int) model.Scenario {
	allowedModes := []string{"armed_away", "armed_home", "armed_night", "disarmed", "vacation"}
	modeVal, _ := model.NewEnumValue("home_mode", allowedModes, "armed_away")
	titleVal, _ := model.ValueOf(fmt.Sprintf("Alert from %s", id))

	steps := make([]model.Step, 0, complexity*2)
	for i := 0; i < complexity; i++ {
		steps = append(steps, model.Step{
			ID:   model.StepID(fmt.Sprintf("notify-%d", i)),
			Kind: model.StepAction,
			Action: &model.ActionStep{
				Capability: model.CapabilityRef{ID: "notification.send", Version: "1.0.0"},
				Arguments:  map[string]model.Expr{"title": model.Literal(titleVal)},
			},
		})
		steps = append(steps, model.Step{
			ID:   model.StepID(fmt.Sprintf("wait-%d", i)),
			Kind: model.StepWait,
			Wait: &model.WaitStep{
				Duration: 50 * time.Millisecond,
			},
		})
	}

	return model.Scenario{
		ID:         model.ID(id),
		RevisionID: "rev-1",
		Name:       fmt.Sprintf("Stress Scenario %s", id),
		Triggers: []model.Trigger{
			{
				ID:   "trig-cam",
				Kind: model.TriggerDeviceEvent,
				Capability: model.CapabilityRef{
					ID:      "camera.person.detected",
					Version: "1.0.0",
					Entity:  &model.EntityRef{ID: "front_camera", Kind: "camera"},
				},
			},
		},
		Condition: model.Expr{
			Op: "eq",
			Args: []model.Expr{
				model.Ref("home.mode"),
				model.Literal(modeVal),
			},
		},
		Flow: model.Flow{
			Steps: steps,
		},
	}
}

func TestScenarioCompilerConcurrentStress(t *testing.T) {
	reg := setupTestCapabilityRegistry(t)
	comp := compiler.NewCompiler(reg)

	workers := runtime.NumCPU() * 4
	if workers < 8 {
		workers = 8
	}
	const scenariosPerWorker = 200

	var totalCompiled uint64
	var totalErrors uint64

	var wg sync.WaitGroup
	wg.Add(workers)

	start := time.Now()
	for w := 0; w < workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < scenariosPerWorker; i++ {
				scenID := fmt.Sprintf("stress-w%02d-scen-%04d", w, i)
				scen := generateDynamicScenario(scenID, (i%5)+1)

				manifest, err := comp.Compile(scen)
				if err != nil {
					atomic.AddUint64(&totalErrors, 1)
					t.Errorf("compile failure: %v", err)
					return
				}
				if manifest == nil || string(manifest.ScenarioID) != scenID {
					atomic.AddUint64(&totalErrors, 1)
					t.Errorf("manifest scenario ID mismatch: %v", manifest)
					return
				}
				atomic.AddUint64(&totalCompiled, 1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	if totalErrors > 0 {
		t.Fatalf("scenario compiler stress encountered %d errors", totalErrors)
	}

	t.Logf("Scenario Compiler Stress: %d scenarios compiled across %d workers in %v (%.2f compilations/sec)",
		totalCompiled, workers, elapsed, float64(totalCompiled)/elapsed.Seconds())
}

func TestScenarioSimulatorMassiveTraceStress(t *testing.T) {
	reg := setupTestCapabilityRegistry(t)
	comp := compiler.NewCompiler(reg)
	sim := simulator.NewSimulator(comp)

	workers := runtime.NumCPU() * 2
	if workers < 4 {
		workers = 4
	}
	const tracesPerWorker = 50

	var completedSims uint64
	var passedCount uint64

	allowedModes := []string{"armed_away", "armed_home", "armed_night", "disarmed", "vacation"}
	awayVal, _ := model.NewEnumValue("home_mode", allowedModes, "armed_away")
	disarmedVal, _ := model.NewEnumValue("home_mode", allowedModes, "disarmed")

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w + 5000)))

			for i := 0; i < tracesPerWorker; i++ {
				scenID := fmt.Sprintf("sim-w%02d-scen-%03d", w, i)
				scen := generateDynamicScenario(scenID, 2)

				isArmed := rng.Float32() > 0.3
				curMode := disarmedVal
				if isArmed {
					curMode = awayVal
				}

				personVal, _ := model.ValueOf(true)
				clock := simulator.NewVirtualClock(time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
				simCtx := simulator.SimulationContext{
					Mode: simulator.ModePure,
					HomeState: map[string]model.Value{
						"home.mode": curMode,
					},
					TriggerEvent: map[string]model.Value{
						"person_detected": personVal,
					},
				}

				result, err := sim.Simulate(scen, simCtx, clock)
				if err != nil {
					t.Errorf("simulation error: %v", err)
					return
				}

				if !result.Passed {
					t.Errorf("simulation did not pass: %v", result.Errors)
					return
				}

				atomic.AddUint64(&passedCount, 1)
				atomic.AddUint64(&completedSims, 1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Simulator Stress: %d full headless simulations executed in %v (Passed=%d, %.2f sims/sec)",
		completedSims, elapsed, passedCount, float64(completedSims)/elapsed.Seconds())
}

func TestCatalogIndexAndDependencyStress(t *testing.T) {
	reg := setupTestCapabilityRegistry(t)
	comp := compiler.NewCompiler(reg)
	cat := catalog.NewCatalog(comp)

	const totalScenarios = 100
	for i := 0; i < totalScenarios; i++ {
		scenID := fmt.Sprintf("cat-scen-%04d", i)
		scen := generateDynamicScenario(scenID, 2)
		record, rev, err := cat.CreateDraft(scen, "stress@test.local")
		if err != nil {
			t.Fatalf("catalog create draft failed: %v", err)
		}
		if _, err := cat.PublishDraft(record.ID, rev.RevisionID, "stress@test.local", "publish"); err != nil {
			t.Fatalf("catalog publish draft failed: %v", err)
		}
	}

	// Concurrent querying on the dependency index
	workers := runtime.NumCPU() * 2
	const queriesPerWorker = 500
	var wg sync.WaitGroup
	wg.Add(workers)

	start := time.Now()
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for q := 0; q < queriesPerWorker; q++ {
				// Query dependency safety check for camera trigger
				canDelete, scens := cat.CanDeleteCapability("camera.person.detected")
				if canDelete || len(scens) != totalScenarios {
					t.Errorf("expected %d scenarios depending on capability, got canDelete=%v len=%d",
						totalScenarios, canDelete, len(scens))
					return
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Catalog Index Query Stress: %d concurrent index lookups over %d published scenarios in %v",
		workers*queriesPerWorker, totalScenarios, elapsed)
}
