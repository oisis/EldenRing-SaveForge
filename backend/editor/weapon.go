package editor

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// WeaponPatch is the RAM-only request DTO for UpdateWeapon. Each Set*
// flag explicitly opts a sub-field into the patch — this avoids pointer
// fields (which Wails generates as nullable types that surface awkwardly
// in TypeScript bindings) while still distinguishing "field absent" from
// "field set to zero".
//
// Semantics:
//   - SetUpgrade:      Upgrade replaces CurrentUpgrade; ItemID is re-encoded.
//   - SetInfusionName: InfusionName replaces the current infusion; ItemID
//     is re-encoded. "" / "Standard" map to the un-infused offset 0.
//   - SetAoWItemID:    AoWItemID is stored as PendingAoWItemID (and the
//     resolved name as PendingAoWName). AoWItemID == 0 is treated as a
//     clear (same as ClearAoW).
//   - ClearAoW:        clears PendingAoWItemID/PendingAoWName.
//
// Phase 1.7 hard limits:
//   - No real handle allocation. AoW handle minting is Phase 4/Save.
//   - No slot.Data mutation. No legacy ApplyWeapon* calls.
//   - Shared-AoW corruption detection is still deferred (warning emitted
//     by Validate).
type WeaponPatch struct {
	SetUpgrade      bool   `json:"setUpgrade"`
	Upgrade         int    `json:"upgrade"`
	SetInfusionName bool   `json:"setInfusionName"`
	InfusionName    string `json:"infusionName"`
	SetAoWItemID    bool   `json:"setAoWItemID"`
	AoWItemID       uint32 `json:"aowItemID"`
	ClearAoW        bool   `json:"clearAoW"`
}

// UpdateWeapon applies a WeaponPatch to the editable item identified by
// uid. It mutates the EditableItem in place, marks the snapshot Dirty,
// and re-runs Validate.
//
// Errors:
//   - nil snapshot
//   - unknown UID
//   - item is not weapon-editable (must satisfy IsWeapon)
//   - SetUpgrade with value outside [0, MaxUpgrade]
//   - SetInfusionName with an infusion name not in db.InfuseTypes
//   - SetAoWItemID with a non-zero ID that is not a known
//     ashes_of_war DB entry
//
// On error nothing in the snapshot is mutated; the caller's prior state
// is preserved. (We validate up front before touching the item.)
func UpdateWeapon(snap *InventoryWorkspaceSnapshot, uid string, patch WeaponPatch) error {
	if snap == nil {
		return fmt.Errorf("UpdateWeapon: nil snapshot")
	}
	kind, idx, found := findEditable(snap, uid)
	if !found {
		return fmt.Errorf("UpdateWeapon: item %q not found in workspace", uid)
	}
	slice := sliceFor(snap, kind)
	it := &(*slice)[idx]
	if !it.IsWeapon {
		return fmt.Errorf("UpdateWeapon: item %q is not weapon-editable (category %q)",
			uid, it.Category)
	}

	// Phase 1: pre-flight validation. We compute every change against the
	// patch first so any error leaves the item untouched.
	newLevel := it.CurrentUpgrade
	newInf := it.InfusionName

	if patch.SetUpgrade {
		if patch.Upgrade < 0 {
			return fmt.Errorf("UpdateWeapon: upgrade %d is negative", patch.Upgrade)
		}
		if it.MaxUpgrade > 0 && patch.Upgrade > it.MaxUpgrade {
			return fmt.Errorf("UpdateWeapon: upgrade %d exceeds MaxUpgrade %d for %s",
				patch.Upgrade, it.MaxUpgrade, it.Name)
		}
		newLevel = patch.Upgrade
	}

	if patch.SetInfusionName {
		if !isKnownInfusion(patch.InfusionName) {
			return fmt.Errorf("UpdateWeapon: unknown infusion %q", patch.InfusionName)
		}
		newInf = patch.InfusionName
	}

	var pendingAoWID uint32
	var pendingAoWName string
	applyAoW := false
	if patch.SetAoWItemID {
		applyAoW = true
		if patch.AoWItemID != 0 {
			aow, _ := db.GetItemDataFuzzy(patch.AoWItemID)
			if aow.Name == "" {
				return fmt.Errorf("UpdateWeapon: Ash of War 0x%08X unknown in DB", patch.AoWItemID)
			}
			if aow.Category != "ashes_of_war" {
				return fmt.Errorf("UpdateWeapon: item 0x%08X (%s) is category %q, not ashes_of_war",
					patch.AoWItemID, aow.Name, aow.Category)
			}
			pendingAoWID = patch.AoWItemID
			pendingAoWName = aow.Name
		}
	}

	// All checks passed — commit.
	if patch.SetUpgrade || patch.SetInfusionName {
		newID, err := encodeWeaponItemID(it.BaseItemID, newLevel, newInf)
		if err != nil {
			return fmt.Errorf("UpdateWeapon: %w", err)
		}
		it.ItemID = newID
		it.CurrentUpgrade = newLevel
		it.InfusionName = newInf
	}
	if applyAoW {
		it.PendingAoWItemID = pendingAoWID
		it.PendingAoWName = pendingAoWName
	}
	if patch.ClearAoW {
		it.PendingAoWItemID = 0
		it.PendingAoWName = ""
	}

	if patch.SetUpgrade || patch.SetInfusionName || patch.SetAoWItemID || patch.ClearAoW {
		it.HasPendingWeaponPatch = true
	}

	snap.Dirty = true
	snap.Validation = Validate(*snap)
	return nil
}

// encodeWeaponItemID is the inverse of decodeWeaponUpgradeInfusion:
// given a base ID, upgrade level, and infusion name, return the
// effective item ID stored in the inventory record.
//
// "" / "Standard" map to offset 0 (un-infused). Any other name must be
// present in db.InfuseTypes; otherwise the helper returns an error so
// the caller never writes a bogus ItemID.
func encodeWeaponItemID(baseID uint32, level int, infusionName string) (uint32, error) {
	if level < 0 {
		return 0, fmt.Errorf("encodeWeaponItemID: negative level %d", level)
	}
	infOffset := 0
	if infusionName != "" && infusionName != "Standard" {
		found := false
		for _, t := range db.InfuseTypes {
			if t.Name == infusionName {
				infOffset = t.Offset
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("encodeWeaponItemID: unknown infusion %q", infusionName)
		}
	}
	return baseID + uint32(infOffset) + uint32(level), nil
}

// isKnownInfusion accepts "", "Standard", or any name listed in
// db.InfuseTypes.
func isKnownInfusion(name string) bool {
	if name == "" || name == "Standard" {
		return true
	}
	for _, t := range db.InfuseTypes {
		if t.Name == name {
			return true
		}
	}
	return false
}
