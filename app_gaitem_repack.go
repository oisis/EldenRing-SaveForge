package main

import (
	"fmt"
	"reflect"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// GaItemCapacity is the Wails-facing capacity report. Its values are computed
// by backend/core; the frontend must not derive usable capacity itself.
type GaItemCapacity struct {
	PhysicalEmpty int `json:"physicalEmpty"`
	CursorRoom    int `json:"cursorRoom"`
	Usable        int `json:"usable"`
}

// GaItemRepackCTA carries the backend's eligibility decision for surfacing a
// GaItem repack call-to-action on a gaitem_full add rejection. The frontend must
// render it verbatim; it may not derive repack safety or recoverable capacity.
type GaItemRepackCTA struct {
	Eligible  bool `json:"eligible"`
	Recovered int  `json:"recovered"`
}

type GaItemRepackBlocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type GaItemRepackFailure struct {
	Stage   string `json:"stage"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type GaItemRepackRollback struct {
	Attempted bool                 `json:"attempted"`
	Complete  bool                 `json:"complete"`
	Mode      string               `json:"mode"`
	Failure   *GaItemRepackFailure `json:"failure,omitempty"`
}

// GaItemRepackAnalysis is the read-only result returned to the UI before a
// user can confirm an allocation optimization.
type GaItemRepackAnalysis struct {
	Outcome         string                `json:"outcome"`
	CharacterIndex  int                   `json:"characterIndex"`
	AnalysisToken   string                `json:"analysisToken,omitempty"`
	Before          GaItemCapacity        `json:"before"`
	ProjectedAfter  *GaItemCapacity       `json:"projectedAfter,omitempty"`
	Recovered       int                   `json:"recovered"`
	NonEmptyRecords int                   `json:"nonEmptyRecords"`
	Blockers        []GaItemRepackBlocker `json:"blockers"`
	Failure         *GaItemRepackFailure  `json:"failure,omitempty"`
}

type GaItemRepackExecuteRequest struct {
	CharacterIndex int    `json:"characterIndex"`
	AnalysisToken  string `json:"analysisToken"`
}

type GaItemRepackExecutionResult struct {
	Outcome        string                `json:"outcome"`
	CharacterIndex int                   `json:"characterIndex"`
	Before         GaItemCapacity        `json:"before"`
	After          *GaItemCapacity       `json:"after,omitempty"`
	Recovered      int                   `json:"recovered"`
	Failure        *GaItemRepackFailure  `json:"failure,omitempty"`
	Rollback       *GaItemRepackRollback `json:"rollback,omitempty"`
}

type gaItemRepackToken struct {
	CharacterIndex int
	SaveGeneration uint64
	SlotRevision   uint64
	Snapshot       core.SlotSnapshot
	Analysis       core.GaItemRepackAnalysis
}

// AnalyzeGaItemRepack runs the safe, non-mutating preflight for one active
// character. A ready result carries a one-use token bound to the exact slot
// state; all other outcomes intentionally return an empty token.
func (a *App) AnalyzeGaItemRepack(charIdx int) (GaItemRepackAnalysis, error) {
	result := GaItemRepackAnalysis{CharacterIndex: charIdx, Blockers: []GaItemRepackBlocker{}}

	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return result, fmt.Errorf("AnalyzeGaItemRepack: no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return result, fmt.Errorf("AnalyzeGaItemRepack: invalid character index %d", charIdx)
	}

	a.lifecycleMu[charIdx].Lock()
	defer a.lifecycleMu[charIdx].Unlock()
	if a.gaItemRepackHasActiveWorkspaceLocked(charIdx) {
		result.Outcome = "unavailable"
		result.Failure = gaItemRepackFailure("app", "inventory_edit_session_active", "Save or discard the active Inventory Workspace before optimizing GaItem allocation.")
		return result, nil
	}

	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	if a.save.Slots[charIdx].Version == 0 {
		return result, fmt.Errorf("AnalyzeGaItemRepack: slot %d is empty", charIdx)
	}

	a.invalidateGaItemRepackTokensLocked(charIdx)
	preflight := core.PreflightGaItemRepack(&a.save.Slots[charIdx])
	if len(preflight.Blockers) != 0 {
		result.Outcome = "refusal"
		result.Blockers = mapGaItemRepackBlockers(preflight.Blockers)
		return result, nil
	}

	result.Before = mapGaItemCapacity(preflight.Analysis.Before)
	projected := mapGaItemCapacity(preflight.Analysis.ProjectedAfter)
	result.ProjectedAfter = &projected
	result.Recovered = preflight.Analysis.Recovered
	result.NonEmptyRecords = preflight.Analysis.NonEmptyRecords
	if preflight.Analysis.NonEmptyRecords == 0 || preflight.Analysis.Recovered == 0 {
		result.Outcome = "no_op"
		return result, nil
	}

	result.Outcome = "ready"
	result.AnalysisToken = a.issueGaItemRepackTokenLocked(charIdx, &a.save.Slots[charIdx], preflight.Analysis)
	return result, nil
}

// ExecuteGaItemRepack consumes a ready analysis token and runs the core
// transaction on a clone. The active slot is replaced only after the cloned
// slot passes every core postcondition; this method never writes a save file.
func (a *App) ExecuteGaItemRepack(req GaItemRepackExecuteRequest) (GaItemRepackExecutionResult, error) {
	result := GaItemRepackExecutionResult{CharacterIndex: req.CharacterIndex}

	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return result, fmt.Errorf("ExecuteGaItemRepack: no save loaded")
	}
	if req.CharacterIndex < 0 || req.CharacterIndex >= len(a.save.Slots) {
		return result, fmt.Errorf("ExecuteGaItemRepack: invalid character index %d", req.CharacterIndex)
	}

	charIdx := req.CharacterIndex
	a.lifecycleMu[charIdx].Lock()
	defer a.lifecycleMu[charIdx].Unlock()
	if a.gaItemRepackHasActiveWorkspaceLocked(charIdx) {
		return gaItemRepackCouldNotStart(result, gaItemRepackFailure("app", "inventory_edit_session_active", "Save or discard the active Inventory Workspace before optimizing GaItem allocation.")), nil
	}

	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	token, ok := a.gaItemRepackTokens[req.AnalysisToken]
	if !ok || req.AnalysisToken == "" {
		return gaItemRepackCouldNotStart(result, gaItemRepackFailure("app", "analysis_stale", "Run GaItem allocation analysis again before optimizing.")), nil
	}
	delete(a.gaItemRepackTokens, req.AnalysisToken) // every recognized token is one-use

	slot := &a.save.Slots[charIdx]
	result.Before = mapGaItemCapacity(token.Analysis.Before)
	if token.CharacterIndex != charIdx || token.SaveGeneration != a.saveGeneration || token.SlotRevision != a.slotRevisions[charIdx] || !reflect.DeepEqual(core.SnapshotSlot(slot), token.Snapshot) {
		return gaItemRepackCouldNotStart(result, gaItemRepackFailure("app", "analysis_stale", "The slot changed after analysis; run GaItem allocation analysis again.")), nil
	}
	preflight := core.PreflightGaItemRepack(slot)
	if len(preflight.Blockers) != 0 {
		return gaItemRepackCouldNotStart(result, gaItemRepackFailure("preflight", "preflight_refusal", "The slot no longer passes the GaItem repack safety checks.")), nil
	}
	if preflight.Analysis != token.Analysis || preflight.Analysis.NonEmptyRecords == 0 || preflight.Analysis.Recovered == 0 {
		return gaItemRepackCouldNotStart(result, gaItemRepackFailure("preflight", "analysis_stale", "The allocation analysis is no longer current; run it again.")), nil
	}

	original := core.CloneSlot(slot)
	candidate := core.CloneSlot(slot)
	coreResult, err := core.RepackGaItems(candidate)
	if err != nil {
		return a.gaItemRepackDiscardCandidate(result, slot, original, gaItemRepackFailure("transform", "repack_failed", err.Error())), nil
	}
	if !coreResult.Changed || coreResult.Before != token.Analysis.Before || coreResult.After != token.Analysis.ProjectedAfter || coreResult.Recovered != token.Analysis.Recovered {
		return a.gaItemRepackDiscardCandidate(result, slot, original, gaItemRepackFailure("validation", "postcondition_mismatch", "The repack result did not match the approved allocation analysis.")), nil
	}

	// The snapshot is pushed only after every candidate postcondition passes.
	// pushUndoSnapshotLocked invalidates remaining tokens and increments the
	// slot revision before this single assignment publishes the candidate.
	a.pushUndoLocked(charIdx)
	a.save.Slots[charIdx] = *candidate
	a.invalidateGaItemRepackTokensLocked(charIdx)

	after := mapGaItemCapacity(coreResult.After)
	result.Outcome = "success"
	result.After = &after
	result.Recovered = coreResult.Recovered
	return result, nil
}

func (a *App) gaItemRepackDiscardCandidate(result GaItemRepackExecutionResult, active, original *core.SaveSlot, failure *GaItemRepackFailure) GaItemRepackExecutionResult {
	if !reflect.DeepEqual(active, original) {
		result.Outcome = "rollback_failed"
		result.Failure = failure
		result.Rollback = &GaItemRepackRollback{
			Attempted: true,
			Complete:  false,
			Mode:      "discard_candidate",
			Failure:   gaItemRepackFailure("rollback", "original_state_changed", "The active slot changed unexpectedly while the repack candidate was discarded."),
		}
		return result
	}
	result.Outcome = "rolled_back"
	result.Failure = failure
	result.Rollback = &GaItemRepackRollback{Attempted: true, Complete: true, Mode: "discard_candidate"}
	return result
}

func gaItemRepackCouldNotStart(result GaItemRepackExecutionResult, failure *GaItemRepackFailure) GaItemRepackExecutionResult {
	result.Outcome = "could_not_start"
	result.Failure = failure
	return result
}

// gaItemFullCTA builds the capacity breakdown and repack CTA for a gaitem_full
// add rejection. It is intentionally conservative: eligibility is asserted only
// when a non-mutating repack preflight proves the rejected batch would fit
// afterward. It never issues a token, calls AnalyzeGaItemRepack, or mutates slot.
func gaItemFullCTA(slot *core.SaveSlot, cap core.CapacityReport, workspaceActive bool) (GaItemCapacity, GaItemRepackCTA) {
	capacity := GaItemCapacity{
		PhysicalEmpty: cap.FreeGaItems,
		CursorRoom:    cap.FreeGaItemCursor,
		Usable:        min(cap.FreeGaItems, cap.FreeGaItemCursor),
	}

	preflight := core.PreflightGaItemRepack(slot)
	cta := GaItemRepackCTA{}
	if len(preflight.Blockers) != 0 {
		return capacity, cta // preflight refusal: no safe recovery estimate
	}

	// Recovered is preserved for any safe preflight, even if another eligibility
	// condition below fails.
	cta.Recovered = preflight.Analysis.Recovered
	cta.Eligible = !workspaceActive &&
		preflight.Analysis.Recovered > 0 &&
		cap.NeededGaItems <= preflight.Analysis.ProjectedAfter.Usable &&
		cap.NeededInv <= cap.FreeInv &&
		cap.NeededStorage <= cap.FreeStorage &&
		cap.NeededGaItemData <= cap.FreeGaItemData
	return capacity, cta
}

func mapGaItemCapacity(capacity core.GaItemCapacity) GaItemCapacity {
	return GaItemCapacity{PhysicalEmpty: capacity.PhysicalEmpty, CursorRoom: capacity.CursorRoom, Usable: capacity.Usable}
}

func mapGaItemRepackBlockers(blockers []core.GaItemRepackBlocker) []GaItemRepackBlocker {
	mapped := make([]GaItemRepackBlocker, 0, len(blockers))
	for _, blocker := range blockers {
		mapped = append(mapped, GaItemRepackBlocker{Code: blocker.Code, Message: blocker.Message})
	}
	return mapped
}

func gaItemRepackFailure(stage, code, message string) *GaItemRepackFailure {
	return &GaItemRepackFailure{Stage: stage, Code: code, Message: message}
}

func (a *App) gaItemRepackHasActiveWorkspaceLocked(charIdx int) bool {
	a.editSessionsMu.Lock()
	_, active := a.editSessionByChar[charIdx]
	a.editSessionsMu.Unlock()
	return active
}

func (a *App) issueGaItemRepackTokenLocked(charIdx int, slot *core.SaveSlot, analysis core.GaItemRepackAnalysis) string {
	if a.gaItemRepackTokens == nil {
		a.gaItemRepackTokens = make(map[string]gaItemRepackToken)
	}
	a.gaItemRepackNextID++
	token := fmt.Sprintf("gaitem-%d-%d", a.saveGeneration, a.gaItemRepackNextID)
	a.gaItemRepackTokens[token] = gaItemRepackToken{
		CharacterIndex: charIdx,
		SaveGeneration: a.saveGeneration,
		SlotRevision:   a.slotRevisions[charIdx],
		Snapshot:       core.SnapshotSlot(slot),
		Analysis:       analysis,
	}
	return token
}

func (a *App) invalidateGaItemRepackTokensLocked(charIdx int) {
	for token, entry := range a.gaItemRepackTokens {
		if entry.CharacterIndex == charIdx {
			delete(a.gaItemRepackTokens, token)
		}
	}
}

// _forceExportTypesGaItemRepack makes the Wails generator emit bindings for
// every repack DTO. It is never invoked at runtime.
func (a *App) _forceExportTypesGaItemRepack() (GaItemCapacity, GaItemRepackCTA, GaItemRepackBlocker, GaItemRepackFailure, GaItemRepackRollback, GaItemRepackAnalysis, GaItemRepackExecuteRequest, GaItemRepackExecutionResult) {
	return GaItemCapacity{}, GaItemRepackCTA{}, GaItemRepackBlocker{}, GaItemRepackFailure{}, GaItemRepackRollback{}, GaItemRepackAnalysis{}, GaItemRepackExecuteRequest{}, GaItemRepackExecutionResult{}
}
