package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// advancedApp builds a journal-only App: the Advanced planner and phase helpers
// touch only a.journal, so no save fixture (and no tmp/save) is required.
func advancedApp(debug bool) *App {
	app := NewApp()
	journal := newInMemoryDiagnosticJournal()
	journal.SetDebugEnabled(debug)
	app.journal = journal
	return app
}

// emitAdvancedLifecycle drives the three phase helpers exactly as the future
// SetNetworkParams/ResetNetworkParams wiring will: plan once, emit before(all),
// planned(all), finished(all). before/planned come from the plan; finished
// re-reads from the actual post-write struct.
func emitAdvancedLifecycle(app *App, action string, before, planned, actual core.NetworkParamValues, outcome characterChangeOutcome, stage string) []advancedNetworkFieldPlan {
	plans := planAdvancedNetworkChanges(before, planned)
	records := advancedPlannedRecords(plans)
	app.journalAdvancedChangeBefore(action, records)
	app.journalAdvancedChangePlanned(action, records)
	app.journalAdvancedChangeFinished(action, outcome, stage, advancedFinishedRecords(plans, actual))
	return plans
}

type advancedLifecycle struct {
	before   map[string]string
	planned  map[string]string
	finished map[string]string
	beforeIx []int
	planIx   []int
	finIx    []int
	outcome  string
	stage    string
}

func collectAdvancedLifecycle(t *testing.T, records []diagnosticRecord, action string) advancedLifecycle {
	t.Helper()
	lc := advancedLifecycle{before: map[string]string{}, planned: map[string]string{}, finished: map[string]string{}}
	for i, rec := range records {
		switch rec.Event {
		case eventAdvancedChangeBefore, eventAdvancedChangePlanned, eventAdvancedChangeFinished:
		default:
			continue
		}
		if got := operationField(rec, "action"); got != action {
			t.Errorf("%s action = %q, want %q", rec.Event, got, action)
		}
		if got := operationField(rec, "character_index"); got != "-1" {
			t.Errorf("%s character_index = %q, want -1", rec.Event, got)
		}
		field := operationField(rec, "field")
		switch rec.Event {
		case eventAdvancedChangeBefore:
			if got := operationField(rec, "after"); got != "" {
				t.Errorf("before %s carries after=%q", field, got)
			}
			if operationField(rec, "outcome") != "" || operationField(rec, "stage") != "" {
				t.Errorf("before %s carries terminal fields", field)
			}
			lc.before[field] = operationField(rec, "before")
			lc.beforeIx = append(lc.beforeIx, i)
		case eventAdvancedChangePlanned:
			if operationField(rec, "outcome") != "" || operationField(rec, "stage") != "" {
				t.Errorf("planned %s carries terminal fields", field)
			}
			lc.planned[field] = operationField(rec, "after")
			lc.planIx = append(lc.planIx, i)
		case eventAdvancedChangeFinished:
			lc.finished[field] = operationField(rec, "after")
			lc.finIx = append(lc.finIx, i)
			lc.outcome = operationField(rec, "outcome")
			lc.stage = operationField(rec, "stage")
		}
	}
	return lc
}

// advancedNetworkFieldOrder is the independently-declared expected order, so a
// reordering of the production spec list is caught rather than tautologically
// mirrored.
var advancedNetworkFieldOrder = []string{
	"network_max_break_in_target_list_count",
	"network_break_in_request_interval_time_sec",
	"network_break_in_request_time_out_sec",
	"network_break_in_request_area_count",
	"network_summon_timeout_time",
	"network_reload_sign_interval_time_2",
	"network_reload_sign_total_count",
	"network_reload_sign_cell_count",
	"network_update_sign_interval_time",
	"network_sing_get_max",
	"network_sign_download_span",
	"network_sign_update_span",
	"network_reload_visit_list_cool_time",
	"network_max_coop_blue_summon_count",
	"network_max_visit_list_count",
	"network_reload_search_coop_blue_min",
	"network_reload_search_coop_blue_max",
	"network_all_area_search_rate_coop_blue",
	"network_all_area_search_rate_vs_blue",
	"network_visitor_list_max",
	"network_visitor_time_out_time",
	"network_visitor_download_span",
}

// allNonZeroNetworkParams sets every field to a distinct non-zero value so a
// plan against the zero value returns all 22 fields in order.
func allNonZeroNetworkParams() core.NetworkParamValues {
	return core.NetworkParamValues{
		MaxBreakInTargetListCount:     1,
		BreakInRequestIntervalTimeSec: 2.5,
		BreakInRequestTimeOutSec:      3.5,
		BreakInRequestAreaCount:       4,
		SummonTimeoutTime:             5.5,
		ReloadSignIntervalTime2:       6.5,
		ReloadSignTotalCount:          7,
		ReloadSignCellCount:           8,
		UpdateSignIntervalTime:        9.5,
		SingGetMax:                    10,
		SignDownloadSpan:              11.5,
		SignUpdateSpan:                12.5,
		ReloadVisitListCoolTime:       13.5,
		MaxCoopBlueSummonCount:        14,
		MaxVisitListCount:             15,
		ReloadSearchCoopBlueMin:       16.5,
		ReloadSearchCoopBlueMax:       17.5,
		AllAreaSearchRateCoopBlue:     18,
		AllAreaSearchRateVsBlue:       19,
		VisitorListMax:                20,
		VisitorTimeOutTime:            21.5,
		VisitorDownloadSpan:           22.5,
	}
}

func TestAdvancedNetworkDiagnosticFullLifecycle(t *testing.T) {
	app := advancedApp(true)
	before := core.NetworkParamDefaults()
	planned := before
	planned.MaxBreakInTargetListCount = 10 // int field
	planned.SummonTimeoutTime = 90.0       // float field

	emitAdvancedLifecycle(app, actionAdvancedSetNetworkParams, before, planned, planned, characterChangeSuccess, characterStageCompleted)

	lc := collectAdvancedLifecycle(t, app.journal.Tail(), actionAdvancedSetNetworkParams)
	if len(lc.beforeIx) != 2 || len(lc.planIx) != 2 || len(lc.finIx) != 2 {
		t.Fatalf("phase counts = %d/%d/%d, want 2/2/2", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	// Strict phase grouping: before(all) -> planned(all) -> finished(all).
	if maxInt(lc.beforeIx) >= minInt(lc.planIx) || maxInt(lc.planIx) >= minInt(lc.finIx) {
		t.Errorf("phase grouping is not before -> planned -> finished")
	}
	if lc.outcome != string(characterChangeSuccess) || lc.stage != characterStageCompleted {
		t.Errorf("terminal = %s/%s, want success/%s", lc.outcome, lc.stage, characterStageCompleted)
	}
	assertAdvancedField(t, lc, "network_max_break_in_target_list_count", "5", "10")
	assertAdvancedField(t, lc, "network_summon_timeout_time", "45", "90")
}

func assertAdvancedField(t *testing.T, lc advancedLifecycle, field, before, after string) {
	t.Helper()
	if got := lc.before[field]; got != before {
		t.Errorf("before %s = %q, want %q", field, got, before)
	}
	if got := lc.planned[field]; got != after {
		t.Errorf("planned %s = %q, want %q", field, got, after)
	}
	if got := lc.finished[field]; got != after {
		t.Errorf("finished %s = %q, want %q", field, got, after)
	}
}

func TestAdvancedNetworkDiagnosticAllFieldsStableOrder(t *testing.T) {
	plans := planAdvancedNetworkChanges(core.NetworkParamValues{}, allNonZeroNetworkParams())
	if len(plans) != len(advancedNetworkFieldOrder) {
		t.Fatalf("plan count = %d, want %d", len(plans), len(advancedNetworkFieldOrder))
	}
	for i, want := range advancedNetworkFieldOrder {
		if got := plans[i].spec.field; got != want {
			t.Errorf("field[%d] = %q, want %q", i, got, want)
		}
	}
	// Formatting: int decimal, float plain scalar.
	if plans[0].planned != "1" {
		t.Errorf("int format = %q, want 1", plans[0].planned)
	}
	if plans[1].planned != "2.5" {
		t.Errorf("float format = %q, want 2.5", plans[1].planned)
	}
}

func TestAdvancedNetworkDiagnosticSelfExcludesUnchanged(t *testing.T) {
	before := allNonZeroNetworkParams()
	planned := before
	planned.MaxBreakInTargetListCount = 99 // change first field only
	planned.SingGetMax = 100               // and a later field

	plans := planAdvancedNetworkChanges(before, planned)
	if len(plans) != 2 {
		t.Fatalf("plan count = %d, want 2 (only changed fields)", len(plans))
	}
	if plans[0].spec.field != "network_max_break_in_target_list_count" {
		t.Errorf("plans[0] = %q, want network_max_break_in_target_list_count", plans[0].spec.field)
	}
	if plans[1].spec.field != "network_sing_get_max" {
		t.Errorf("plans[1] = %q, want network_sing_get_max", plans[1].spec.field)
	}
}

func TestAdvancedNetworkDiagnosticNoop(t *testing.T) {
	app := advancedApp(true)
	same := core.NetworkParamDefaults()
	emitAdvancedLifecycle(app, actionAdvancedResetNetworkParams, same, same, same, characterChangeSuccess, characterStageCompleted)
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("noop records = %d, want 0", got)
	}
}

func TestAdvancedNetworkDiagnosticDebugOff(t *testing.T) {
	app := advancedApp(false)
	before := core.NetworkParamDefaults()
	planned := before
	planned.MaxBreakInTargetListCount = 10
	emitAdvancedLifecycle(app, actionAdvancedSetNetworkParams, before, planned, planned, characterChangeSuccess, characterStageCompleted)
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("debug-off records = %d, want 0", got)
	}
}

func TestAdvancedNetworkDiagnosticFinishedError(t *testing.T) {
	app := advancedApp(true)
	before := core.NetworkParamDefaults()
	planned := before
	planned.MaxBreakInTargetListCount = 10
	// A failed patch leaves the actual state at before; finished reports the
	// unchanged value with the error outcome and patch stage.
	emitAdvancedLifecycle(app, actionAdvancedSetNetworkParams, before, planned, before, characterChangeError, stageAdvancedPatch)

	lc := collectAdvancedLifecycle(t, app.journal.Tail(), actionAdvancedSetNetworkParams)
	if lc.outcome != string(characterChangeError) || lc.stage != stageAdvancedPatch {
		t.Errorf("terminal = %s/%s, want error/%s", lc.outcome, lc.stage, stageAdvancedPatch)
	}
	if got := lc.finished["network_max_break_in_target_list_count"]; got != "5" {
		t.Errorf("finished after = %q, want 5 (rolled back)", got)
	}
}
