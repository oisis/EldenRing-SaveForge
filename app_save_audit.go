package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// SaveIssue is one actionable problem found in the loaded save, annotated with
// where the user can fix it. Surfaced before a file save so edits/corruption are
// never written silently.
type SaveIssue struct {
	Slot    int    `json:"slot"`    // 0-based character slot index
	Code    string `json:"code"`    // editor validation code
	Message string `json:"message"` // human-readable problem
	FixTab  string `json:"fixTab"`  // where to repair it in the UI
}

// fixLocationForCode maps a validation code to the UI location that can repair it.
func fixLocationForCode(code string) string {
	switch code {
	case editor.CodeUpgradeOutOfRange:
		return "Inventory → Weapons & Sort order (open the weapon, pick a valid level, Apply)"
	case "duplicate_handle", "unknown_item":
		return "Inventory → Weapons & Sort order"
	default:
		return "Inventory"
	}
}

// AuditLoadedSaveIssues scans every non-empty character slot of the loaded save
// and returns the blocking validation errors (e.g. upgrade out of range). It is
// read-only — no session, no mutation. The frontend calls it before a file save
// so it can show the user what is wrong (and where to fix it) without blocking.
func (a *App) AuditLoadedSaveIssues() ([]SaveIssue, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	// Multi-slot reader — every Slots[i] is read under its slotMu[i].
	// Take all 10 ascending so the audit sees a consistent snapshot
	// across every slot at the same wall-clock instant.
	a.lockAllSlots()
	defer a.unlockAllSlots()
	var issues []SaveIssue
	for i := range a.save.Slots {
		slot := &a.save.Slots[i]
		if slot.Version == 0 {
			continue
		}
		snap, err := editor.BuildSnapshot(slot, fmt.Sprintf("audit-%d", i), i)
		if err != nil {
			continue // unparseable slot — not an editable-item issue
		}
		rep := editor.Validate(snap)
		for _, e := range rep.Errors {
			issues = append(issues, SaveIssue{
				Slot:    i,
				Code:    e.Code,
				Message: e.Message,
				FixTab:  fixLocationForCode(e.Code),
			})
		}
	}
	return issues, nil
}
