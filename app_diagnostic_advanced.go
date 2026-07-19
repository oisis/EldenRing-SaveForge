package main

import (
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// Advanced-tab (Network) diagnostics reuse the same before -> planned ->
// finished lifecycle as Character, Game Items and World, through the shared
// journalChangeRecords loop, characterFieldChange type and change*Tail helpers
// in app_diagnostic_character.go. This file only supplies the Advanced-specific
// event names, thin phase helpers and the deterministic NetworkParamValues
// planner; the sanitizer and per-field emission are never re-implemented.

// Action tags every Advanced diagnostic record carries. They mirror the labels
// on the existing general network_params_requested / network_params_finished
// events, so a diagnostics reader can correlate the coarse and detailed records
// of one write. The strings are closed and backend-owned — never derived from
// renderer input.
const (
	actionAdvancedSetNetworkParams   = "network_params_apply"
	actionAdvancedResetNetworkParams = "network_params_reset"
)

// stageAdvancedPatch is the closed technical stage a NetworkParam mutation
// reports on a failed finished record; a successful write reports the shared
// characterStageCompleted, so the finished stage stays a small typed vocabulary.
const stageAdvancedPatch = "patch"

// advancedCharacterIndex is the character_index every Advanced record carries.
// NetworkParam is a global regulation state of the save (UserData11), not the
// state of any one character, so the slot index is a fixed "no character" -1.
const advancedCharacterIndex = -1

// Lifecycle event names for a single NetworkParam field mutation. Per changed
// field one record is emitted for each phase in this order:
// before -> planned -> finished.
const (
	eventAdvancedChangeBefore   = "advanced_change_before"
	eventAdvancedChangePlanned  = "advanced_change_planned"
	eventAdvancedChangeFinished = "advanced_change_finished"
)

// journalAdvancedChangeBefore records the pre-change value of each field, one
// debug record per field, before any new value is applied. Only "before" is
// meaningful at this phase, so "after"/outcome/stage are omitted.
func (a *App) journalAdvancedChangeBefore(action string, changes []characterFieldChange) {
	a.journalChangeRecords(eventAdvancedChangeBefore, "advanced change before", action, advancedCharacterIndex, changes, nil)
}

// journalAdvancedChangePlanned records the intended new value of each field,
// one debug record per field, after it is computed but before it is applied.
func (a *App) journalAdvancedChangePlanned(action string, changes []characterFieldChange) {
	a.journalChangeRecords(eventAdvancedChangePlanned, "advanced change planned", action, advancedCharacterIndex, changes, changePlannedTail)
}

// journalAdvancedChangeFinished records the terminal state of each field, one
// debug record per field, once the mutation has run. outcome and stage report
// how and where it ended; After holds the actual applied (or attempted) value.
func (a *App) journalAdvancedChangeFinished(action string, outcome characterChangeOutcome, stage string, changes []characterFieldChange) {
	a.journalChangeRecords(eventAdvancedChangeFinished, "advanced change finished", action, advancedCharacterIndex, changes, changeFinishedTail(outcome, stage))
}

// advancedNetworkFieldSpec is one NetworkParamValues field: a stable journal
// field name and a scalar reader. read is a closure over the typed struct so the
// 22-field list is declared exactly once; before, planned and finished all
// project the same specs against a different NetworkParamValues value. Integers
// format decimally; floats use strconv.FormatFloat(_, 'f', -1, 32) for a stable
// plain scalar. No raw UserData11 bytes, preset names, paths or account data are
// ever exposed.
type advancedNetworkFieldSpec struct {
	field string
	read  func(core.NetworkParamValues) string
}

// advancedNetworkFieldSpecs is the single source of truth for the NetworkParam
// field order and formatting. The order is fixed and matches the struct
// declaration order in backend/core/regulation.go, so the journal never leaks
// map iteration order and the next task can plan against the same list without
// duplicating it.
func advancedNetworkFieldSpecs() []advancedNetworkFieldSpec {
	i := func(v int32) string { return strconv.FormatInt(int64(v), 10) }
	f := func(v float32) string { return strconv.FormatFloat(float64(v), 'f', -1, 32) }
	return []advancedNetworkFieldSpec{
		{"network_max_break_in_target_list_count", func(p core.NetworkParamValues) string { return i(p.MaxBreakInTargetListCount) }},
		{"network_break_in_request_interval_time_sec", func(p core.NetworkParamValues) string { return f(p.BreakInRequestIntervalTimeSec) }},
		{"network_break_in_request_time_out_sec", func(p core.NetworkParamValues) string { return f(p.BreakInRequestTimeOutSec) }},
		{"network_break_in_request_area_count", func(p core.NetworkParamValues) string { return i(p.BreakInRequestAreaCount) }},
		{"network_summon_timeout_time", func(p core.NetworkParamValues) string { return f(p.SummonTimeoutTime) }},
		{"network_reload_sign_interval_time_2", func(p core.NetworkParamValues) string { return f(p.ReloadSignIntervalTime2) }},
		{"network_reload_sign_total_count", func(p core.NetworkParamValues) string { return i(p.ReloadSignTotalCount) }},
		{"network_reload_sign_cell_count", func(p core.NetworkParamValues) string { return i(p.ReloadSignCellCount) }},
		{"network_update_sign_interval_time", func(p core.NetworkParamValues) string { return f(p.UpdateSignIntervalTime) }},
		{"network_sing_get_max", func(p core.NetworkParamValues) string { return i(p.SingGetMax) }},
		{"network_sign_download_span", func(p core.NetworkParamValues) string { return f(p.SignDownloadSpan) }},
		{"network_sign_update_span", func(p core.NetworkParamValues) string { return f(p.SignUpdateSpan) }},
		{"network_reload_visit_list_cool_time", func(p core.NetworkParamValues) string { return f(p.ReloadVisitListCoolTime) }},
		{"network_max_coop_blue_summon_count", func(p core.NetworkParamValues) string { return i(p.MaxCoopBlueSummonCount) }},
		{"network_max_visit_list_count", func(p core.NetworkParamValues) string { return i(p.MaxVisitListCount) }},
		{"network_reload_search_coop_blue_min", func(p core.NetworkParamValues) string { return f(p.ReloadSearchCoopBlueMin) }},
		{"network_reload_search_coop_blue_max", func(p core.NetworkParamValues) string { return f(p.ReloadSearchCoopBlueMax) }},
		{"network_all_area_search_rate_coop_blue", func(p core.NetworkParamValues) string { return i(p.AllAreaSearchRateCoopBlue) }},
		{"network_all_area_search_rate_vs_blue", func(p core.NetworkParamValues) string { return i(p.AllAreaSearchRateVsBlue) }},
		{"network_visitor_list_max", func(p core.NetworkParamValues) string { return i(p.VisitorListMax) }},
		{"network_visitor_time_out_time", func(p core.NetworkParamValues) string { return f(p.VisitorTimeOutTime) }},
		{"network_visitor_download_span", func(p core.NetworkParamValues) string { return f(p.VisitorDownloadSpan) }},
	}
}

// advancedNetworkFieldPlan is one NetworkParam field whose planned value differs
// from its before value. before/planned are captured once; spec.read re-reads
// the field's actual value from the post-write state for the finished phase, so
// the field list is never duplicated across the three phases.
type advancedNetworkFieldPlan struct {
	spec    advancedNetworkFieldSpec
	before  string
	planned string
}

// planAdvancedNetworkChanges builds the ordered list of NetworkParam fields
// whose planned value differs from the before value, in the fixed spec order.
// Unchanged fields self-exclude, so a no-op (before == planned) yields no plans
// and therefore no records. The next task computes planned via
// core.PatchNetworkParams and reads the finished state via core.ReadNetworkParams
// while passing the same before/planned/actual structs through here.
func planAdvancedNetworkChanges(before, planned core.NetworkParamValues) []advancedNetworkFieldPlan {
	var plans []advancedNetworkFieldPlan
	for _, s := range advancedNetworkFieldSpecs() {
		b := s.read(before)
		p := s.read(planned)
		if b == p {
			continue
		}
		plans = append(plans, advancedNetworkFieldPlan{spec: s, before: b, planned: p})
	}
	return plans
}

// advancedPlannedRecords maps plans to before/planned records: the before phase
// ignores After, the planned phase uses it, mirroring plannedChangeRecords.
func advancedPlannedRecords(plans []advancedNetworkFieldPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.spec.field, Before: p.before, After: p.planned}
	}
	return out
}

// advancedFinishedRecords maps plans to finished records, re-reading each field's
// actual value from the post-write NetworkParamValues so the After column
// reflects what really landed rather than the requested value.
func advancedFinishedRecords(plans []advancedNetworkFieldPlan, actual core.NetworkParamValues) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.spec.field, Before: p.before, After: p.spec.read(actual)}
	}
	return out
}

// journalAdvancedNetworkLifecycle is the shared Apply/Reset Debug Mode driver. It
// wraps a single real NetworkParam write with the advanced_change_* before ->
// planned -> finished lifecycle. before and planned come from the one
// PatchNetworkParams the caller already ran (planned == the bytes that will be
// assigned), so the per-field journal can never diverge from what the writer
// persists. apply performs the assignment and returns the actual post-write state
// (or, on failure, the actual unchanged state) so finished reports what really
// landed rather than the requested plan. All records carry advancedCharacterIndex
// (-1) and only scalar field values — never raw UserData11, preset names or paths.
func (a *App) journalAdvancedNetworkLifecycle(action string, before, planned core.NetworkParamValues, apply func() (core.NetworkParamValues, error)) error {
	plans := planAdvancedNetworkChanges(before, planned)
	records := advancedPlannedRecords(plans)
	a.journalAdvancedChangeBefore(action, records)
	a.journalAdvancedChangePlanned(action, records)

	actual, err := apply()
	if err != nil {
		a.journalAdvancedChangeFinished(action, characterChangeError, stageAdvancedPatch, advancedFinishedRecords(plans, actual))
		return err
	}
	a.journalAdvancedChangeFinished(action, characterChangeSuccess, characterStageCompleted, advancedFinishedRecords(plans, actual))
	return nil
}
