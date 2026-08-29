package siren

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domainaction "github.com/Homiakus/Home_Sentinel/internal/domain/action"
	gatewayfake "github.com/Homiakus/Home_Sentinel/internal/gateway/fake"
	"github.com/Homiakus/axiom/adgo"
)

func TestParsePlanVersionDurationCanonicalRoundTrip(t *testing.T) {
	valid := []struct {
		version string
		want    time.Duration
	}{
		{version: "1-1s", want: time.Second},
		{version: "1-1m0s", want: time.Minute},
		{version: "1-1h30m0s", want: 90 * time.Minute},
	}
	for _, tc := range valid {
		t.Run(tc.version, func(t *testing.T) {
			got, err := parsePlanVersionDuration(tc.version)
			if err != nil {
				t.Fatalf("parsePlanVersionDuration(%q): %v", tc.version, err)
			}
			if got != tc.want {
				t.Fatalf("parsePlanVersionDuration(%q)=%s want=%s", tc.version, got, tc.want)
			}
			plan, err := CompilePlan(got)
			if err != nil {
				t.Fatalf("CompilePlan(%s): %v", got, err)
			}
			if plan.Version != tc.version {
				t.Fatalf("round-trip version=%q want=%q", plan.Version, tc.version)
			}
		})
	}

	invalid := []string{
		"",
		"2-1s",
		"1-",
		"1-0s",
		"1--1s",
		"1-60s",
		"1-1000ms",
		"1- 1s",
	}
	for _, version := range invalid {
		t.Run("reject_"+strings.ReplaceAll(version, "/", "_"), func(t *testing.T) {
			if _, err := parsePlanVersionDuration(version); err == nil {
				t.Fatalf("parsePlanVersionDuration(%q) unexpectedly succeeded", version)
			}
		})
	}
}

func TestSirenPebbleReopenRoutesHistoricalDurationCompensation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewSirenController(map[string]bool{"main": false})

	cfgA := DefaultConfig(root)
	cfgA.WorkerID = "siren-upgrade-a"
	cfgA.MaxActivationDuration = time.Hour
	first, err := Open(cfgA, Dependencies{Siren: controller})
	if err != nil {
		t.Fatalf("open duration A: %v", err)
	}
	oldRequest := domainaction.SirenRequest{RequestID: "alarm-before-config-change", SirenID: "main", RequestedBy: "incident"}
	old, err := first.Start(ctx, oldRequest)
	if err != nil {
		t.Fatalf("start duration A: %v", err)
	}
	old, err = first.Drive(ctx, old.ID)
	if err != nil {
		t.Fatalf("drive duration A: %v", err)
	}
	if old.Status != adgo.StatusWaiting {
		t.Fatalf("duration-A status=%s want=%s", old.Status, adgo.StatusWaiting)
	}
	oldVersion, oldDigest := old.PlanVersion, old.PlanDigest
	if oldVersion != "1-1h0m0s" {
		t.Fatalf("duration-A version=%q", oldVersion)
	}
	enabled, err := controller.Enabled(ctx, "main")
	if err != nil || !enabled {
		t.Fatalf("siren not enabled before restart: enabled=%v err=%v", enabled, err)
	}
	if controller.Applied != 1 {
		t.Fatalf("physical applications before restart=%d want=1", controller.Applied)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close duration A: %v", err)
	}

	cfgB := DefaultConfig(root)
	cfgB.WorkerID = "siren-upgrade-b"
	cfgB.MaxActivationDuration = 2 * time.Hour
	second, err := Open(cfgB, Dependencies{Siren: controller})
	if err != nil {
		t.Fatalf("reopen duration B: %v", err)
	}
	defer second.Close()

	reloaded, err := second.Get(ctx, old.ID)
	if err != nil {
		t.Fatalf("get historical execution: %v", err)
	}
	if reloaded.PlanVersion != oldVersion || reloaded.PlanDigest != oldDigest {
		t.Fatalf("historical identity drifted: version=%s digest=%s", reloaded.PlanVersion, reloaded.PlanDigest)
	}
	if second.bundles.active.plan.Version != "1-2h0m0s" || second.bundles.active.plan.Digest == oldDigest {
		t.Fatalf("active duration-B identity=%s/%s", second.bundles.active.plan.Version, second.bundles.active.plan.Digest)
	}
	same, err := second.Start(ctx, oldRequest)
	if err != nil {
		t.Fatalf("idempotent start of historical execution: %v", err)
	}
	if same.ID != old.ID || same.PlanVersion != oldVersion || same.PlanDigest != oldDigest {
		t.Fatalf("idempotent historical start drifted: id=%s version=%s digest=%s", same.ID, same.PlanVersion, same.PlanDigest)
	}
	if _, err := second.Stop(ctx, old.ID, "operator stop after config change"); err != nil {
		t.Fatalf("stop historical execution: %v", err)
	}
	old, err = second.Drive(ctx, old.ID)
	if err != nil {
		t.Fatalf("drive historical compensation: %v", err)
	}
	if old.Status != adgo.StatusCanceled || old.PlanVersion != oldVersion || old.PlanDigest != oldDigest {
		t.Fatalf("historical completion status=%s identity=%s/%s", old.Status, old.PlanVersion, old.PlanDigest)
	}
	enabled, err = controller.Enabled(ctx, "main")
	if err != nil {
		t.Fatalf("read siren after compensation: %v", err)
	}
	if enabled || controller.Applied != 2 || controller.Calls != 2 {
		t.Fatalf("historical compensation mismatch: enabled=%v applied=%d calls=%d", enabled, controller.Applied, controller.Calls)
	}

	fresh, err := second.Start(ctx, domainaction.SirenRequest{RequestID: "alarm-after-config-change", SirenID: "main"})
	if err != nil {
		t.Fatalf("start duration B: %v", err)
	}
	if fresh.PlanVersion != "1-2h0m0s" || fresh.PlanDigest == oldDigest {
		t.Fatalf("new execution did not use duration B: version=%s digest=%s", fresh.PlanVersion, fresh.PlanDigest)
	}
}

func TestSirenOpenRejectsMalformedNonTerminalHistoricalVersion(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := DefaultConfig(root)
	cfg.MaxActivationDuration = time.Hour
	first, err := Open(cfg, Dependencies{Siren: gatewayfake.NewSirenController(map[string]bool{"main": false})})
	if err != nil {
		t.Fatalf("open fixture runtime: %v", err)
	}
	execution, err := first.Start(ctx, domainaction.SirenRequest{RequestID: "malformed-version", SirenID: "main"})
	if err != nil {
		t.Fatalf("start fixture: %v", err)
	}
	_, err = first.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanVersion = "1-60m"
		current.PlanDigest = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		return nil
	})
	if err != nil {
		t.Fatalf("inject malformed identity: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close fixture runtime: %v", err)
	}

	_, err = Open(cfg, Dependencies{Siren: gatewayfake.NewSirenController(map[string]bool{"main": false})})
	if !errors.Is(err, ErrUnknownExecutionBundle) {
		t.Fatalf("reopen error=%v want ErrUnknownExecutionBundle", err)
	}
}

func TestSirenOpenRejectsHistoricalDigestMismatch(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfgA := DefaultConfig(root)
	cfgA.MaxActivationDuration = time.Hour
	first, err := Open(cfgA, Dependencies{Siren: gatewayfake.NewSirenController(map[string]bool{"main": false})})
	if err != nil {
		t.Fatalf("open fixture runtime: %v", err)
	}
	execution, err := first.Start(ctx, domainaction.SirenRequest{RequestID: "digest-mismatch", SirenID: "main"})
	if err != nil {
		t.Fatalf("start fixture: %v", err)
	}
	_, err = first.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanDigest = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		return nil
	})
	if err != nil {
		t.Fatalf("inject digest mismatch: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close fixture runtime: %v", err)
	}

	cfgB := DefaultConfig(root)
	cfgB.MaxActivationDuration = 2 * time.Hour
	_, err = Open(cfgB, Dependencies{Siren: gatewayfake.NewSirenController(map[string]bool{"main": false})})
	if !errors.Is(err, ErrExecutionBundleMismatch) {
		t.Fatalf("reopen error=%v want ErrExecutionBundleMismatch", err)
	}
}

func TestSirenTerminalHistoricalIdentityDoesNotBlockReopen(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	controller := gatewayfake.NewSirenController(map[string]bool{"main": false})
	cfgA := DefaultConfig(root)
	cfgA.MaxActivationDuration = time.Hour
	first, err := Open(cfgA, Dependencies{Siren: controller})
	if err != nil {
		t.Fatalf("open fixture runtime: %v", err)
	}
	execution, err := first.Start(ctx, domainaction.SirenRequest{RequestID: "terminal-history", SirenID: "main"})
	if err != nil {
		t.Fatalf("start fixture: %v", err)
	}
	if _, err := first.Drive(ctx, execution.ID); err != nil {
		t.Fatalf("drive fixture: %v", err)
	}
	if _, err := first.Stop(ctx, execution.ID, "complete terminal fixture"); err != nil {
		t.Fatalf("stop fixture: %v", err)
	}
	execution, err = first.Drive(ctx, execution.ID)
	if err != nil {
		t.Fatalf("drive terminal compensation: %v", err)
	}
	if execution.Status != adgo.StatusCanceled {
		t.Fatalf("fixture status=%s want=%s", execution.Status, adgo.StatusCanceled)
	}
	mutated, err := first.production.Store.Commit(ctx, execution.ID, execution.Version, func(current *adgo.Execution) error {
		current.PlanVersion = "retired-terminal-format"
		current.PlanDigest = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		return nil
	})
	if err != nil {
		t.Fatalf("inject terminal historical identity: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close fixture runtime: %v", err)
	}

	cfgB := DefaultConfig(root)
	cfgB.MaxActivationDuration = 2 * time.Hour
	second, err := Open(cfgB, Dependencies{Siren: controller})
	if err != nil {
		t.Fatalf("terminal history blocked reopen: %v", err)
	}
	defer second.Close()
	loaded, err := second.Get(ctx, execution.ID)
	if err != nil {
		t.Fatalf("get terminal history: %v", err)
	}
	if loaded.Status != adgo.StatusCanceled || loaded.PlanVersion != mutated.PlanVersion || loaded.PlanDigest != mutated.PlanDigest {
		t.Fatalf("terminal history changed: status=%s version=%s digest=%s", loaded.Status, loaded.PlanVersion, loaded.PlanDigest)
	}
}
