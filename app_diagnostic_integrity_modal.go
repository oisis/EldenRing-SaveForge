package main

import (
	"strconv"
	"strings"
)

// diagnosticIntegrityModalItemsMax bounds the affected_items summary so a heavily
// corrupted save cannot write an unbounded list into the durable journal; the
// remainder is reported as a safe count in additional_items.
const diagnosticIntegrityModalItemsMax = 20

// RecordDiagnosticIntegrityModalShown journals that the inventory integrity
// repair modal was actually shown for a dirty active save. It takes no argument
// and is the backend source of truth: it re-runs the read-only integrity scan
// itself (never trusting a frontend-supplied report) and, only when the scan is
// dirty, records a single event with safe aggregate counters plus a bounded,
// deterministic list of affected item names and conflict kinds.
//
// GetSaveInventoryIntegrityReport releases saveMu and every slot lock before it
// returns, so the journal Sync below never runs under a save/slot lock. No item
// ID, handle, row, acquisition index, character name, path, or raw error is
// logged — only DB item names and the conflict kind.
func (a *App) RecordDiagnosticIntegrityModalShown() {
	report, err := a.GetSaveInventoryIntegrityReport()
	if err != nil || report.Clean {
		return
	}

	var duplicateEntries, conflictingIndices, totalItems int
	items := make([]string, 0, diagnosticIntegrityModalItemsMax)
	for _, slot := range report.Slots {
		duplicateEntries += slot.DuplicateEntryCount
		conflictingIndices += slot.ConflictingIndexCount
		for _, conflict := range slot.Conflicts {
			for _, item := range conflict.Items {
				totalItems++
				if len(items) < diagnosticIntegrityModalItemsMax {
					items = append(items, integrityModalItemLabel(item, conflict.Kind))
				}
			}
		}
	}

	a.journalLog(levelInfo, "inventory_integrity_modal_shown",
		"inventory integrity repair modal shown",
		field("affected_slots", strconv.Itoa(len(report.Slots))),
		field("duplicate_inventory_entries", strconv.Itoa(duplicateEntries)),
		field("conflicting_indices", strconv.Itoa(conflictingIndices)),
		field("affected_items", strings.Join(items, "; ")),
		field("additional_items", strconv.Itoa(totalItems-len(items))))
}

// integrityModalItemLabel renders one conflict item as "<name> (<kind>)" using
// only the resolved DB name — never the raw item ID or handle. An unresolved or
// nameless item degrades to the fixed "unknown_item" placeholder.
func integrityModalItemLabel(item InventoryIntegrityConflictItem, kind string) string {
	name := item.Name
	if item.Unknown || name == "" {
		name = "unknown_item"
	}
	if kind == "" {
		return name
	}
	return name + " (" + kind + ")"
}

// RecordDiagnosticIntegrityModalRepairOutcome journals the terminal outcome of
// the modal's single Repair action. Outcome is validated against the closed set
// {resolved, unresolved, error}; any other value is dropped without a record so
// a renderer cannot write an arbitrary event. resolved/unresolved log at info,
// error at error; the only field is the validated outcome.
func (a *App) RecordDiagnosticIntegrityModalRepairOutcome(outcome string) {
	level := levelInfo
	switch outcome {
	case "resolved", "unresolved":
	case "error":
		level = levelError
	default:
		return
	}

	a.journalLog(level, "inventory_integrity_modal_repair_finished",
		"inventory integrity modal repair finished",
		field("outcome", outcome))
}
