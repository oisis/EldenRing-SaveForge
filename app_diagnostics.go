package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// DiagnosticsReport is returned by RunDiagnostics* endpoints.
type DiagnosticsReport struct {
	Source    string           `json:"source"` // "loaded" or absolute file path
	Slots     []SlotDiagResult `json:"slots"`
	CanRepair bool             `json:"canRepair"` // true if any repairable issue found
}

// SlotDiagResult wraps core.SlotDiagnostics with character name.
type SlotDiagResult struct {
	SlotIndex int                    `json:"slotIndex"`
	CharName  string                 `json:"charName"`
	Issues    []core.DiagnosticIssue `json:"issues"`
}

// RepairReport is returned by Repair* endpoints.
type RepairReport struct {
	Fixed   []string `json:"fixed"`
	Skipped []string `json:"skipped"`
}

var repairableCategories = map[string]bool{
	"inventory":  true,
	"stats":      true,
	"dlc":        true,
	"gaitemdata": true,
	"storage":    true,
}

func canRepairIssues(issues []core.DiagnosticIssue) bool {
	for _, iss := range issues {
		if iss.Severity == core.SeverityInfo {
			continue // info messages are advisory only, nothing to repair
		}
		if repairableCategories[iss.Category] {
			return true
		}
	}
	return false
}

// RunDiagnosticsAllLoaded scans every active slot of the currently loaded save.
// Only slots that have at least one issue are included in the result.
func (a *App) RunDiagnosticsAllLoaded() (DiagnosticsReport, error) {
	var empty DiagnosticsReport
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, fmt.Errorf("no save loaded")
	}
	a.lockAllSlots()
	defer a.unlockAllSlots()

	var slots []SlotDiagResult
	var allIssues []core.DiagnosticIssue
	for i := range a.save.Slots {
		slot := &a.save.Slots[i]
		if slot.Version == 0 {
			continue
		}
		diag := core.DiagnoseSaveCorruption(slot, i)
		if len(diag.Issues) == 0 {
			continue
		}
		slots = append(slots, SlotDiagResult{
			SlotIndex: i,
			CharName:  core.UTF16ToString(slot.Player.CharacterName[:]),
			Issues:    diag.Issues,
		})
		allIssues = append(allIssues, diag.Issues...)
	}

	return DiagnosticsReport{
		Source:    "loaded",
		Slots:     slots,
		CanRepair: canRepairIssues(allIssues),
	}, nil
}

// RepairAllLoadedSlots applies automated repairs to every active slot of the
// currently loaded save. Pushes undo for each slot before mutating.
func (a *App) RepairAllLoadedSlots() (RepairReport, error) {
	var empty RepairReport
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, fmt.Errorf("no save loaded")
	}
	a.lockAllSlots()
	defer a.unlockAllSlots()

	allFixed := []string{}
	allSkipped := []string{}
	for i := range a.save.Slots {
		slot := &a.save.Slots[i]
		if slot.Version == 0 {
			continue
		}
		a.pushUndoLocked(i)
		fixed, skipped := core.RepairSlot(slot)
		allFixed = append(allFixed, fixed...)
		allSkipped = append(allSkipped, skipped...)
	}
	return RepairReport{Fixed: allFixed, Skipped: allSkipped}, nil
}
