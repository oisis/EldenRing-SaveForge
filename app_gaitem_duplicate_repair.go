package main

import (
	"fmt"
	"reflect"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// GaItemDuplicateCandidate is one physical GaItem record the user may keep. The
// frontend renders both and must send back exactly one Index as the keep choice.
type GaItemDuplicateCandidate struct {
	Index  int    `json:"index"`
	ItemID uint32 `json:"itemId"`
}

// GaItemDuplicateAnalysis is the read-only result returned to the UI before the
// user chooses which physical GaItem to keep. A "ready" outcome carries a
// one-use token bound to the exact slot state; all other outcomes return none.
type GaItemDuplicateAnalysis struct {
	Outcome        string                     `json:"outcome"`
	CharacterIndex int                        `json:"characterIndex"`
	Handle         uint32                     `json:"handle"`
	AnalysisToken  string                     `json:"analysisToken,omitempty"`
	Candidates     []GaItemDuplicateCandidate `json:"candidates"`
	RefusalCode    string                     `json:"refusalCode,omitempty"`
	RefusalMessage string                     `json:"refusalMessage,omitempty"`
	Failure        *GaItemRepackFailure       `json:"failure,omitempty"`
}

type GaItemDuplicateExecuteRequest struct {
	CharacterIndex int    `json:"characterIndex"`
	Handle         uint32 `json:"handle"`
	KeepIndex      int    `json:"keepIndex"`
	AnalysisToken  string `json:"analysisToken"`
}

type GaItemDuplicateExecutionResult struct {
	Outcome        string                `json:"outcome"`
	CharacterIndex int                   `json:"characterIndex"`
	Handle         uint32                `json:"handle"`
	KeptIndex      int                   `json:"keptIndex"`
	RemovedIndex   int                   `json:"removedIndex"`
	Failure        *GaItemRepackFailure  `json:"failure,omitempty"`
	Rollback       *GaItemRepackRollback `json:"rollback,omitempty"`
}

type gaItemDedupToken struct {
	CharacterIndex int
	Handle         uint32
	SaveGeneration uint64
	SlotRevision   uint64
	Snapshot       core.SlotSnapshot
	Candidates     [2]core.GaItemDuplicateCandidate
}

// AnalyzeGaItemDuplicate runs the read-only, user-gated preflight for one
// physical duplicate handle on one active character. A ready result carries a
// one-use token bound to the exact slot state and requires the caller to choose
// which physical index to keep; it never picks a candidate automatically.
func (a *App) AnalyzeGaItemDuplicate(charIdx int, handle uint32) (GaItemDuplicateAnalysis, error) {
	result := GaItemDuplicateAnalysis{CharacterIndex: charIdx, Handle: handle, Candidates: []GaItemDuplicateCandidate{}}

	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return result, fmt.Errorf("AnalyzeGaItemDuplicate: no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return result, fmt.Errorf("AnalyzeGaItemDuplicate: invalid character index %d", charIdx)
	}

	a.lifecycleMu[charIdx].Lock()
	defer a.lifecycleMu[charIdx].Unlock()
	if a.gaItemRepackHasActiveWorkspaceLocked(charIdx) {
		result.Outcome = "unavailable"
		result.Failure = gaItemRepackFailure("app", "inventory_edit_session_active", "Save or discard the active Inventory Workspace before deduplicating GaItem records.")
		return result, nil
	}

	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	if a.save.Slots[charIdx].Version == 0 {
		return result, fmt.Errorf("AnalyzeGaItemDuplicate: slot %d is empty", charIdx)
	}

	a.invalidateGaItemDedupTokensLocked(charIdx)
	analysis := core.AnalyzeGaItemDuplicate(&a.save.Slots[charIdx], handle)
	result.Candidates = mapGaItemDuplicateCandidates(analysis.Candidates)
	if !analysis.Repairable {
		result.Outcome = "refusal"
		result.RefusalCode = analysis.RefusalCode
		result.RefusalMessage = analysis.RefusalMsg
		return result, nil
	}

	result.Outcome = "ready"
	result.AnalysisToken = a.issueGaItemDedupTokenLocked(charIdx, &a.save.Slots[charIdx], handle, analysis.Candidates)
	return result, nil
}

// ExecuteGaItemDuplicateRepair consumes a ready analysis token and the explicit
// keep index, then runs the core deduplication on a clone. The active slot is
// replaced only after the candidate passes every core postcondition; a single
// undo snapshot is pushed just before publishing. It never writes a save file
// and never runs a repack afterwards.
func (a *App) ExecuteGaItemDuplicateRepair(req GaItemDuplicateExecuteRequest) (GaItemDuplicateExecutionResult, error) {
	result := GaItemDuplicateExecutionResult{CharacterIndex: req.CharacterIndex, Handle: req.Handle, KeptIndex: req.KeepIndex}

	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return result, fmt.Errorf("ExecuteGaItemDuplicateRepair: no save loaded")
	}
	if req.CharacterIndex < 0 || req.CharacterIndex >= len(a.save.Slots) {
		return result, fmt.Errorf("ExecuteGaItemDuplicateRepair: invalid character index %d", req.CharacterIndex)
	}

	charIdx := req.CharacterIndex
	a.lifecycleMu[charIdx].Lock()
	defer a.lifecycleMu[charIdx].Unlock()
	if a.gaItemRepackHasActiveWorkspaceLocked(charIdx) {
		return gaItemDedupCouldNotStart(result, gaItemRepackFailure("app", "inventory_edit_session_active", "Save or discard the active Inventory Workspace before deduplicating GaItem records.")), nil
	}

	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	token, ok := a.gaItemDedupTokens[req.AnalysisToken]
	if !ok || req.AnalysisToken == "" {
		return gaItemDedupCouldNotStart(result, gaItemRepackFailure("app", "analysis_stale", "Run GaItem duplicate analysis again before repairing.")), nil
	}
	delete(a.gaItemDedupTokens, req.AnalysisToken) // every recognized token is one-use

	slot := &a.save.Slots[charIdx]
	if token.CharacterIndex != charIdx || token.Handle != req.Handle || token.SaveGeneration != a.saveGeneration ||
		token.SlotRevision != a.slotRevisions[charIdx] || !reflect.DeepEqual(core.SnapshotSlot(slot), token.Snapshot) {
		return gaItemDedupCouldNotStart(result, gaItemRepackFailure("app", "analysis_stale", "The slot changed after analysis; run GaItem duplicate analysis again.")), nil
	}
	if req.KeepIndex != token.Candidates[0].Index && req.KeepIndex != token.Candidates[1].Index {
		return gaItemDedupCouldNotStart(result, gaItemRepackFailure("app", "invalid_keep_index", "Choose which duplicate GaItem record to keep before repairing.")), nil
	}
	result.RemovedIndex = token.Candidates[0].Index
	if req.KeepIndex == token.Candidates[0].Index {
		result.RemovedIndex = token.Candidates[1].Index
	}

	original := core.CloneSlot(slot)
	candidate := core.CloneSlot(slot)
	if err := core.RepairGaItemDuplicate(candidate, req.Handle, req.KeepIndex); err != nil {
		return a.gaItemDedupDiscardCandidate(result, slot, original, gaItemRepackFailure("transform", "repair_failed", err.Error())), nil
	}

	// The snapshot is pushed only after the candidate passes every core
	// postcondition; pushUndoLocked invalidates tokens and bumps the revision.
	a.pushUndoLocked(charIdx)
	a.save.Slots[charIdx] = *candidate
	a.invalidateGaItemDedupTokensLocked(charIdx)

	result.Outcome = "success"
	return result, nil
}

func (a *App) gaItemDedupDiscardCandidate(result GaItemDuplicateExecutionResult, active, original *core.SaveSlot, failure *GaItemRepackFailure) GaItemDuplicateExecutionResult {
	if !reflect.DeepEqual(active, original) {
		result.Outcome = "rollback_failed"
		result.Failure = failure
		result.Rollback = &GaItemRepackRollback{
			Attempted: true,
			Complete:  false,
			Mode:      "discard_candidate",
			Failure:   gaItemRepackFailure("rollback", "original_state_changed", "The active slot changed unexpectedly while the repair candidate was discarded."),
		}
		return result
	}
	result.Outcome = "rolled_back"
	result.Failure = failure
	result.Rollback = &GaItemRepackRollback{Attempted: true, Complete: true, Mode: "discard_candidate"}
	return result
}

func gaItemDedupCouldNotStart(result GaItemDuplicateExecutionResult, failure *GaItemRepackFailure) GaItemDuplicateExecutionResult {
	result.Outcome = "could_not_start"
	result.Failure = failure
	return result
}

func mapGaItemDuplicateCandidates(candidates [2]core.GaItemDuplicateCandidate) []GaItemDuplicateCandidate {
	out := make([]GaItemDuplicateCandidate, 0, 2)
	for _, c := range candidates {
		out = append(out, GaItemDuplicateCandidate{Index: c.Index, ItemID: c.ItemID})
	}
	return out
}

func (a *App) issueGaItemDedupTokenLocked(charIdx int, slot *core.SaveSlot, handle uint32, candidates [2]core.GaItemDuplicateCandidate) string {
	if a.gaItemDedupTokens == nil {
		a.gaItemDedupTokens = make(map[string]gaItemDedupToken)
	}
	a.gaItemRepackNextID++
	token := fmt.Sprintf("gadedup-%d-%d", a.saveGeneration, a.gaItemRepackNextID)
	a.gaItemDedupTokens[token] = gaItemDedupToken{
		CharacterIndex: charIdx,
		Handle:         handle,
		SaveGeneration: a.saveGeneration,
		SlotRevision:   a.slotRevisions[charIdx],
		Snapshot:       core.SnapshotSlot(slot),
		Candidates:     candidates,
	}
	return token
}

func (a *App) invalidateGaItemDedupTokensLocked(charIdx int) {
	for token, entry := range a.gaItemDedupTokens {
		if entry.CharacterIndex == charIdx {
			delete(a.gaItemDedupTokens, token)
		}
	}
}

// _forceExportTypesGaItemDuplicate surfaces every dedup DTO to the Wails type
// generator. It is never invoked at runtime.
func (a *App) _forceExportTypesGaItemDuplicate() (GaItemDuplicateCandidate, GaItemDuplicateAnalysis, GaItemDuplicateExecuteRequest, GaItemDuplicateExecutionResult) {
	return GaItemDuplicateCandidate{}, GaItemDuplicateAnalysis{}, GaItemDuplicateExecuteRequest{}, GaItemDuplicateExecutionResult{}
}
