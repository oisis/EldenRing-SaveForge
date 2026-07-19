package main

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// applyRecorder models the production apply closure setNetworkParamsLocked hands
// the driver: it is invoked exactly once per write, publishes the patched state
// (here just returning a captured value) and reports the actual post-write state
// (or, on failure, the unchanged actual). calls proves the single-mutation
// guarantee without a mockable global patch hook.
type applyRecorder struct {
	calls  int
	actual core.NetworkParamValues
	err    error
}

func (r *applyRecorder) apply() (core.NetworkParamValues, error) {
	r.calls++
	return r.actual, r.err
}

// scalarPattern accepts only plain integer/float scalars, so a per-field record
// carrying a raw UserData11 hex blob, a path, or any non-scalar value fails.
var scalarPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

func assertAdvancedScalars(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	for _, rec := range records {
		switch rec.Event {
		case eventAdvancedChangeBefore, eventAdvancedChangePlanned, eventAdvancedChangeFinished:
		default:
			continue
		}
		for _, key := range []string{"before", "after"} {
			if v := operationField(rec, key); v != "" && !scalarPattern.MatchString(v) {
				t.Errorf("%s %s = %q is not a plain scalar", rec.Event, key, v)
			}
		}
	}
}

func TestAdvancedNetworkLifecycleApplySuccess(t *testing.T) {
	app := advancedApp(true)
	before := core.NetworkParamDefaults()
	planned := before
	planned.MaxBreakInTargetListCount = 10 // int
	planned.SummonTimeoutTime = 90.0       // float
	// The real path assigns exactly the patched bytes planned was read from, so
	// the post-write state equals planned.
	actual := planned

	rec := &applyRecorder{actual: actual}
	if err := app.journalAdvancedNetworkLifecycle(actionAdvancedSetNetworkParams, before, planned, rec.apply); err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	if rec.calls != 1 {
		t.Fatalf("apply calls = %d, want exactly 1", rec.calls)
	}

	lc := collectAdvancedLifecycle(t, app.journal.Tail(), actionAdvancedSetNetworkParams)
	if len(lc.beforeIx) != 2 || len(lc.planIx) != 2 || len(lc.finIx) != 2 {
		t.Fatalf("phase counts = %d/%d/%d, want 2/2/2", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	if maxInt(lc.beforeIx) >= minInt(lc.planIx) || maxInt(lc.planIx) >= minInt(lc.finIx) {
		t.Errorf("phase grouping is not before(all) -> planned(all) -> finished(all)")
	}
	if lc.outcome != string(characterChangeSuccess) || lc.stage != characterStageCompleted {
		t.Errorf("terminal = %s/%s, want success/%s", lc.outcome, lc.stage, characterStageCompleted)
	}
	// before = pre-write state; planned == finished == target value.
	assertAdvancedField(t, lc, "network_max_break_in_target_list_count", "5", "10")
	assertAdvancedField(t, lc, "network_summon_timeout_time", "45", "90")
	assertAdvancedScalars(t, app.journal.Tail())
}

func TestAdvancedNetworkLifecycleResetSuccess(t *testing.T) {
	app := advancedApp(true)
	before := core.NetworkParamDefaults()
	before.MaxBreakInTargetListCount = 15 // non-vanilla
	before.VisitorListMax = 50            // non-vanilla
	planned := core.NetworkParamDefaults()
	actual := planned // reset lands exactly on vanilla

	rec := &applyRecorder{actual: actual}
	if err := app.journalAdvancedNetworkLifecycle(actionAdvancedResetNetworkParams, before, planned, rec.apply); err != nil {
		t.Fatalf("lifecycle: %v", err)
	}

	lc := collectAdvancedLifecycle(t, app.journal.Tail(), actionAdvancedResetNetworkParams)
	if len(lc.before) != 2 || len(lc.planned) != 2 || len(lc.finished) != 2 {
		t.Fatalf("reset field counts = %d/%d/%d, want exactly 2 (only non-vanilla)", len(lc.before), len(lc.planned), len(lc.finished))
	}
	assertAdvancedField(t, lc, "network_max_break_in_target_list_count", "15", "5")
	assertAdvancedField(t, lc, "network_visitor_list_max", "50", "10")
	if lc.outcome != string(characterChangeSuccess) || lc.stage != characterStageCompleted {
		t.Errorf("terminal = %s/%s, want success/%s", lc.outcome, lc.stage, characterStageCompleted)
	}
}

func TestAdvancedNetworkLifecycleError(t *testing.T) {
	app := advancedApp(true)
	before := core.NetworkParamDefaults()
	planned := before
	planned.MaxBreakInTargetListCount = 10
	// A failed apply leaves the real state unchanged, so actual == before.
	wantErr := fmt.Errorf("assign failed")
	rec := &applyRecorder{actual: before, err: wantErr}

	err := app.journalAdvancedNetworkLifecycle(actionAdvancedSetNetworkParams, before, planned, rec.apply)
	if err != wantErr {
		t.Fatalf("lifecycle err = %v, want %v", err, wantErr)
	}
	if rec.calls != 1 {
		t.Fatalf("apply calls = %d, want exactly 1", rec.calls)
	}

	lc := collectAdvancedLifecycle(t, app.journal.Tail(), actionAdvancedSetNetworkParams)
	if lc.outcome != string(characterChangeError) || lc.stage != stageAdvancedPatch {
		t.Errorf("terminal = %s/%s, want error/%s", lc.outcome, lc.stage, stageAdvancedPatch)
	}
	// before(all) and planned(all) still precede the error finished.
	if len(lc.beforeIx) != 1 || len(lc.planIx) != 1 || len(lc.finIx) != 1 {
		t.Fatalf("phase counts = %d/%d/%d, want 1/1/1", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	// finished reads the actual post-error state (unchanged == before).
	if got := lc.finished["network_max_break_in_target_list_count"]; got != "5" {
		t.Errorf("finished after = %q, want 5 (real post-error state)", got)
	}
}

func TestAdvancedNetworkLifecycleDebugOffMutatesOnceNoRecords(t *testing.T) {
	app := advancedApp(false) // Debug Mode off
	before := core.NetworkParamDefaults()
	planned := before
	planned.MaxBreakInTargetListCount = 10
	rec := &applyRecorder{actual: planned}

	if err := app.journalAdvancedNetworkLifecycle(actionAdvancedSetNetworkParams, before, planned, rec.apply); err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	if rec.calls != 1 {
		t.Fatalf("apply calls = %d, want exactly 1 (mutation still runs)", rec.calls)
	}
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("debug-off records = %d, want 0", got)
	}
}

func TestSetNetworkParamsWithoutSaveEmitsNoAdvancedRecords(t *testing.T) {
	app := withJournal(NewApp())
	app.journal.SetDebugEnabled(true) // even with Debug on, no save => no advanced records
	if err := app.SetNetworkParams(core.NetworkParamDefaults()); err == nil {
		t.Fatal("SetNetworkParams succeeded without a save")
	}

	records := app.journal.Tail()
	// High-level compatibility lifecycle stays intact.
	requested := operationEvent(t, records, "network_params_requested")
	finished := operationEvent(t, records, "network_params_finished")
	if requested.Seq >= finished.Seq {
		t.Errorf("requested seq %d must precede finished seq %d", requested.Seq, finished.Seq)
	}
	if got := operationField(finished, "stage"); got != "no_active_save" {
		t.Errorf("stage = %q, want no_active_save", got)
	}
	// No per-field advanced records were produced.
	for _, rec := range records {
		switch rec.Event {
		case eventAdvancedChangeBefore, eventAdvancedChangePlanned, eventAdvancedChangeFinished:
			t.Errorf("unexpected advanced record %s without an active save", rec.Event)
		}
	}
}
