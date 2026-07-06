package editor

import "fmt"

// CanRepairCode reports whether code can be auto-repaired without binary
// patching (i.e. via AutoRepairWorkspaceItem).
func CanRepairCode(code string) bool {
	switch code {
	case CodeUpgradeOutOfRange, CodePendingAoWUnknown, CodePendingAoWConflict:
		return true
	}
	return false
}

// RepairDescForIssue returns a human-readable description of what
// AutoRepairWorkspaceItem will do for iss. Returns "" for non-repairable
// codes or when the item is not found in snap.
func RepairDescForIssue(snap *InventoryWorkspaceSnapshot, iss WorkspaceValidationIssue) string {
	if iss.UID == "" || !CanRepairCode(iss.Code) {
		return ""
	}
	switch iss.Code {
	case CodeUpgradeOutOfRange:
		kind, idx, found := findEditable(snap, iss.UID)
		if !found {
			return "Clamp upgrade to max"
		}
		it := (*sliceFor(snap, kind))[idx]
		target := ClampUpgrade(it.CurrentUpgrade, it.MaxUpgrade)
		return fmt.Sprintf("Clamp +%d → +%d", it.CurrentUpgrade, target)
	case CodePendingAoWUnknown, CodePendingAoWConflict:
		return "Clear pending AoW"
	}
	return ""
}

// AutoRepairWorkspaceItem applies the appropriate automatic fix for code
// to the item identified by uid. Delegates to UpdateWeapon so the snapshot
// is re-validated and marked Dirty on success. Returns an error for
// non-repairable codes or unknown UIDs.
func AutoRepairWorkspaceItem(snap *InventoryWorkspaceSnapshot, uid, code string) error {
	if !CanRepairCode(code) {
		return fmt.Errorf("AutoRepairWorkspaceItem: code %q is not auto-repairable", code)
	}
	switch code {
	case CodeUpgradeOutOfRange:
		kind, idx, found := findEditable(snap, uid)
		if !found {
			return fmt.Errorf("AutoRepairWorkspaceItem: item %q not found", uid)
		}
		it := (*sliceFor(snap, kind))[idx]
		target := ClampUpgrade(it.CurrentUpgrade, it.MaxUpgrade)
		return UpdateWeapon(snap, uid, WeaponPatch{SetUpgrade: true, Upgrade: target})
	case CodePendingAoWUnknown, CodePendingAoWConflict:
		return UpdateWeapon(snap, uid, WeaponPatch{ClearAoW: true})
	}
	return fmt.Errorf("AutoRepairWorkspaceItem: unhandled code %q", code)
}
