package main

import (
	"encoding/binary"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// equipmentSlotChrAsmIndex maps a canonical slot key to the corresponding
// index inside the 22-entry core.ChrAsmEquipment array. Keys mirror
// templates.EquipmentSlotOrder. Phase 7c extends the map with the 5
// talisman indices (17–21).
var equipmentSlotChrAsmIndex = map[string]int{
	"weaponLeftHand1":  0,
	"weaponRightHand1": 1,
	"weaponLeftHand2":  2,
	"weaponRightHand2": 3,
	"weaponLeftHand3":  4,
	"weaponRightHand3": 5,
	"arrows1":          6,
	"bolts1":           7,
	"arrows2":          8,
	"bolts2":           9,
	"armorHead":        12,
	"armorChest":       13,
	"armorArms":        14,
	"armorLegs":        15,
	"talisman1":        17,
	"talisman2":        18,
	"talisman3":        19,
	"talisman4":        20,
	"talisman5":        21,
}

// equipmentSlotKindForKey returns the Phase 7b.0 core writer slot kind
// for a Phase 7b.1 canonical slot key. Both maps are stable allowlists;
// an unknown key returns (0, false).
func equipmentSlotKindForKey(slotKey string) (core.EquipmentSlotKind, bool) {
	switch slotKey {
	case "weaponLeftHand1":
		return core.EquipSlotLeftHandArmament1, true
	case "weaponRightHand1":
		return core.EquipSlotRightHandArmament1, true
	case "weaponLeftHand2":
		return core.EquipSlotLeftHandArmament2, true
	case "weaponRightHand2":
		return core.EquipSlotRightHandArmament2, true
	case "weaponLeftHand3":
		return core.EquipSlotLeftHandArmament3, true
	case "weaponRightHand3":
		return core.EquipSlotRightHandArmament3, true
	case "arrows1":
		return core.EquipSlotArrows1, true
	case "bolts1":
		return core.EquipSlotBolts1, true
	case "arrows2":
		return core.EquipSlotArrows2, true
	case "bolts2":
		return core.EquipSlotBolts2, true
	case "armorHead":
		return core.EquipSlotHead, true
	case "armorChest":
		return core.EquipSlotChest, true
	case "armorArms":
		return core.EquipSlotArms, true
	case "armorLegs":
		return core.EquipSlotLegs, true
	case "talisman1":
		return core.EquipSlotTalisman1, true
	case "talisman2":
		return core.EquipSlotTalisman2, true
	case "talisman3":
		return core.EquipSlotTalisman3, true
	case "talisman4":
		return core.EquipSlotTalisman4, true
	case "talisman5":
		return core.EquipSlotTalisman5, true
	}
	return 0, false
}

// equipmentSlotIsAmmo reports whether the slot key is one of the four
// ammo positions (Arrows1/2, Bolts1/2). Ammo slots store goods item IDs
// directly (0x40-prefixed), while weapon/armor slots store
// `itemID | 0x80000000`.
func equipmentSlotIsAmmo(slotKey string) bool {
	switch slotKey {
	case "arrows1", "bolts1", "arrows2", "bolts2":
		return true
	}
	return false
}

// equipmentSlotIsTalisman reports whether the slot key is one of the five
// talisman positions. Talisman slots store the talisman item ID directly
// (the 0x20-prefixed form already present in the GaMap), with no
// `| 0x80000000` mask applied.
func equipmentSlotIsTalisman(slotKey string) bool {
	switch slotKey {
	case "talisman1", "talisman2", "talisman3", "talisman4", "talisman5":
		return true
	}
	return false
}

// buildEquipmentSectionFromSlot scans the supported ChrAsmEquipment
// slots (0–9, 12–15) on the given SaveSlot and returns a
// templates.EquipmentSection whose pointer fields are populated for
// every non-empty equipped slot. The matching strategy mirrors the
// Phase 7b.1 export contract:
//
//   - Read the raw u32 at slot.Data[EquipItemsIDOffset + idx*4].
//   - 0xFFFFFFFF → field stays nil (slot is empty in-game).
//   - Decode the equipped form back to an itemID (weapon/armor: strip
//     the 0x80 high-bit flag; ammo: take the raw value as a goods
//     itemID which already carries the 0x40 prefix).
//   - Look up the corresponding editor.EditableItem in inventoryItems
//     by matching encoded ItemID; copy BaseItemID, Name, current
//     upgrade, infusion, and custom AoW into the EquipmentItemRef.
//   - When no editable item matches (e.g. the equipped item is a
//     pass-through record or absent from CommonItems entirely) but
//     the DB recognises the decoded ID, fall back to the DB-derived
//     baseItemID / name with no upgrade / infusion metadata.
//   - When the DB does not recognise the ID either, the slot still
//     emits a ref carrying just the raw decoded ItemID — the export
//     report shows the user there is something in the slot rather
//     than silently dropping it.
//
// Returns nil when the slot's EquipItemsIDOffset has not been parsed
// (empty / unreadable slot) or when no supported slot is populated.
func buildEquipmentSectionFromSlot(slot *core.SaveSlot, inventoryItems []editor.EditableItem) *templates.EquipmentSection {
	if slot == nil || slot.EquipItemsIDOffset <= 0 {
		return nil
	}
	if slot.EquipItemsIDOffset+core.ChrAsmEquipmentSize > len(slot.Data) {
		return nil
	}

	// Index editable inventory by the equipped-form encoded value so we
	// can do a single O(1) lookup per slot. Weapons/armor encode as
	// `ItemID | 0x80000000`; ammo (goods) encodes as ItemID directly.
	byEquipped := make(map[uint32]*editor.EditableItem, len(inventoryItems))
	for i := range inventoryItems {
		it := &inventoryItems[i]
		// weapons/armor candidate
		byEquipped[it.ItemID|core.ItemTypeWeapon] = it
		// ammo / goods candidate
		byEquipped[it.ItemID] = it
	}

	out := &templates.EquipmentSection{}
	any := false
	for _, slotKey := range templates.EquipmentSlotOrder {
		chrAsmIdx, ok := equipmentSlotChrAsmIndex[slotKey]
		if !ok {
			continue
		}
		off := slot.EquipItemsIDOffset + chrAsmIdx*4
		raw := binary.LittleEndian.Uint32(slot.Data[off:])
		if raw == 0xFFFFFFFF {
			continue
		}

		ref := decodeEquipmentSlotToRef(raw, slotKey, byEquipped)
		if ref == nil {
			continue
		}
		templates.SetEquipmentSlotRef(out, slotKey, ref)
		any = true
	}
	if !any {
		return nil
	}
	return out
}

// decodeEquipmentSlotToRef builds the EquipmentItemRef from a raw u32
// pulled out of ChrAsmEquipment. byEquipped is the encoded-form lookup
// indexed by ItemID|0x80000000 (weapons/armor) and bare ItemID (ammo).
func decodeEquipmentSlotToRef(raw uint32, slotKey string, byEquipped map[uint32]*editor.EditableItem) *templates.EquipmentItemRef {
	if raw == 0 || raw == 0xFFFFFFFF {
		return nil
	}

	// Look up the matching editable item using the exact raw value first;
	// fall back to the decoded ItemID (strip the 0x80 flag for weapon /
	// armor; ammo slots store ItemID directly).
	if it, ok := byEquipped[raw]; ok {
		return itemToEquipmentRef(it)
	}

	var candidateID uint32
	if equipmentSlotIsAmmo(slotKey) || equipmentSlotIsTalisman(slotKey) {
		candidateID = raw
	} else {
		candidateID = raw &^ core.ItemTypeWeapon
	}
	if it, ok := byEquipped[candidateID]; ok {
		return itemToEquipmentRef(it)
	}

	// Editable inventory does not contain the equipped item — fall back
	// to a DB-derived ref so the export still records which item the
	// slot holds. This path is rare for weapons / armor but expected for
	// Unarmed / placeholder slots.
	itemData, baseID := db.GetItemDataFuzzy(candidateID)
	if itemData.Name != "" {
		return &templates.EquipmentItemRef{
			BaseItemID: baseID,
			Name:       itemData.Name,
		}
	}
	// Last resort: unknown item, emit minimal ref with raw decoded ID so
	// the user at least sees "there is something here we could not
	// resolve".
	return &templates.EquipmentItemRef{BaseItemID: candidateID}
}

// MaxActiveTalismanSlots is the vanilla cap on simultaneously equipped
// talismans (1 base slot + 3 Talisman Pouch upgrades). Slot 5 (index 21)
// exists in the binary but is unreachable through in-game gameplay; the
// resolver therefore warns + skips any non-empty talisman5 ref.
const MaxActiveTalismanSlots = 4

// resolveEquipmentWrites walks the selected slots in
// templates.EquipmentSection and produces a []core.EquipmentWrite batch
// ready for SaveSlot.WriteEquipment.
//
// activeTalismanSlots is the effective talisman pouch capacity (1..4)
// the resolver gates talisman slots against. Callers should compute it
// as `1 + effective profile.talismanSlots` where the effective value is
// the template's profile.talismanSlots when present and selected, else
// the slot's current persisted Player.TalismanSlots. See
// computeActiveTalismanSlots in app_templates_v2_apply.go.
//
// Phase 7b.1 / 7c strict-existing-only policy:
//   - sel must be non-nil and HasAny == true at the call site.
//   - sec may be nil only when no slot is selected (defensive — the
//     caller checks hasEquipment before invoking us).
//   - For each selected + populated slot:
//   - BaseItemID == 0 → emit EquipmentWrite{Handle: 0} (explicit
//     clear). No inventory lookup, no pouch gating; clearing an
//     out-of-bounds talisman slot (incl. talisman5) is always allowed.
//   - BaseItemID > 0 → search slot.Inventory.CommonItems for a
//     matching item. Storage is NOT searched. Match keys: BaseItemID
//     (required), Upgrade (optional disambiguator), InfusionName
//     (optional), AoWItemID (optional). Multi-match resolves to the
//     first hit + emits equipment_item_ambiguous warning. No match
//     emits equipment_item_not_in_inventory warning and the slot is
//     skipped.
//   - Talisman slots (talisman1..5) are additionally gated against
//     activeTalismanSlots. A non-empty ref for slot N where
//     N > activeTalismanSlots emits talisman_slot_pouch_insufficient
//     warning and is skipped. Talisman5 always trips the gate when
//     populated (vanilla cap = 4 active slots).
//
// Returned warnings carry the canonical slot key in Container (reusing
// the existing optional string field on ImportPreviewIssue so the UI
// can deep-link to the affected slot without a new field).
//
// The Go error return is reserved for infrastructure problems (nil
// slot, nil section pointer where the caller expected one); per-slot
// resolution issues never surface as a Go error.
func resolveEquipmentWrites(slot *core.SaveSlot, sel *templates.SectionSelection, sec *templates.EquipmentSection, activeTalismanSlots uint8) ([]core.EquipmentWrite, []templates.ImportPreviewIssue, error) {
	if slot == nil {
		return nil, nil, fmt.Errorf("resolveEquipmentWrites: nil slot")
	}
	// Snapshot the editable inventory once so per-slot lookups are
	// against a single consistent view. BuildSnapshot does the GaMap +
	// DB resolution + AoW current-state lookups the resolver needs to
	// match the optional disambiguators (Upgrade, InfusionName,
	// AoWItemID).
	snap, err := editor.BuildSnapshot(slot, "", -1)
	if err != nil {
		return nil, nil, fmt.Errorf("resolveEquipmentWrites: BuildSnapshot: %w", err)
	}
	return resolveEquipmentWritesFromItems(snap.InventoryItems, sel, sec, activeTalismanSlots)
}

// talismanSlotOrdinal returns 1..5 for talisman1..talisman5 and 0 for
// any non-talisman slot key. Used by the resolver to gate non-empty
// talisman refs against the active pouch capacity.
func talismanSlotOrdinal(slotKey string) int {
	switch slotKey {
	case "talisman1":
		return 1
	case "talisman2":
		return 2
	case "talisman3":
		return 3
	case "talisman4":
		return 4
	case "talisman5":
		return 5
	}
	return 0
}

// resolveEquipmentWritesFromItems is the pure-logic core of the
// resolver, taking an already-materialised list of editable inventory
// items. Factored out so tests can exercise the matching / warning
// logic without standing up a full SaveSlot that BuildSnapshot can
// parse.
func resolveEquipmentWritesFromItems(items []editor.EditableItem, sel *templates.SectionSelection, sec *templates.EquipmentSection, activeTalismanSlots uint8) ([]core.EquipmentWrite, []templates.ImportPreviewIssue, error) {
	if sec == nil {
		return nil, nil, fmt.Errorf("resolveEquipmentWrites: nil equipment section")
	}

	var writes []core.EquipmentWrite
	var warnings []templates.ImportPreviewIssue

	for _, slotKey := range templates.EquipmentSlotOrder {
		if !sel.Selected(slotKey) {
			continue
		}
		ref := templates.EquipmentSlotRef(sec, slotKey)
		if ref == nil {
			continue
		}
		kind, ok := equipmentSlotKindForKey(slotKey)
		if !ok {
			// Unreachable — equipmentSlotKindForKey covers every key in
			// EquipmentSlotOrder. Defensive guard so a future
			// EquipmentSlotOrder extension that forgets to update the
			// mapping surfaces as a clear error rather than silently
			// dropping slots.
			return nil, nil, fmt.Errorf("resolveEquipmentWrites: no core slot kind for %q", slotKey)
		}

		if ref.BaseItemID == 0 {
			writes = append(writes, core.EquipmentWrite{Slot: kind, Handle: 0})
			continue
		}

		// Talisman pouch gating runs only for non-empty talisman refs.
		// Vanilla cap = 4 active slots; talisman5 is always out of range
		// because there is no Pouch upgrade that lifts the cap to 5.
		if ord := talismanSlotOrdinal(slotKey); ord > 0 {
			if ord > MaxActiveTalismanSlots || ord > int(activeTalismanSlots) {
				warnings = append(warnings, templates.ImportPreviewIssue{
					Severity:   "warning",
					Code:       templates.IssueCodeTalismanSlotPouchInsufficient,
					Message:    fmt.Sprintf("equipment.%s: talisman slot %d not available (active pouch capacity = %d); slot skipped", slotKey, ord, activeTalismanSlots),
					Container:  slotKey,
					BaseItemID: ref.BaseItemID,
				})
				continue
			}
		}

		handle, ambiguous, found := lookupEquipmentHandle(items, ref)
		if !found {
			warnings = append(warnings, templates.ImportPreviewIssue{
				Severity:   "warning",
				Code:       templates.IssueCodeEquipmentItemNotInInventory,
				Message:    fmt.Sprintf("equipment.%s: baseItemID 0x%08X is not in inventory; slot skipped", slotKey, ref.BaseItemID),
				Container:  slotKey,
				BaseItemID: ref.BaseItemID,
			})
			continue
		}
		if ambiguous {
			warnings = append(warnings, templates.ImportPreviewIssue{
				Severity:   "warning",
				Code:       templates.IssueCodeEquipmentItemAmbiguous,
				Message:    fmt.Sprintf("equipment.%s: multiple inventory matches for baseItemID 0x%08X; first wins", slotKey, ref.BaseItemID),
				Container:  slotKey,
				BaseItemID: ref.BaseItemID,
			})
		}
		writes = append(writes, core.EquipmentWrite{Slot: kind, Handle: handle})
	}
	return writes, warnings, nil
}

// lookupEquipmentHandle returns (handle, ambiguous, found) for the
// first EditableItem in items that matches the ref's BaseItemID and any
// supplied optional disambiguators (Upgrade, InfusionName, AoWItemID).
// ambiguous is true when more than one item satisfied the match.
func lookupEquipmentHandle(items []editor.EditableItem, ref *templates.EquipmentItemRef) (uint32, bool, bool) {
	matches := 0
	var winner uint32
	for i := range items {
		it := &items[i]
		if it.BaseItemID != ref.BaseItemID {
			continue
		}
		if ref.Upgrade != nil && it.CurrentUpgrade != *ref.Upgrade {
			continue
		}
		if ref.InfusionName != "" && it.InfusionName != ref.InfusionName {
			continue
		}
		if ref.AoWItemID != nil {
			if !it.HasCurrentAoW || it.CurrentAoWItemID != *ref.AoWItemID {
				continue
			}
		}
		matches++
		if matches == 1 {
			winner = it.OriginalHandle
		}
	}
	if matches == 0 {
		return 0, false, false
	}
	return winner, matches > 1, true
}

// itemToEquipmentRef projects an EditableItem onto the schema fields
// EquipmentItemRef carries. Upgrade / AoW pointers are heap-allocated so
// the resulting ref does not alias the editor item.
func itemToEquipmentRef(it *editor.EditableItem) *templates.EquipmentItemRef {
	ref := &templates.EquipmentItemRef{
		BaseItemID:   it.BaseItemID,
		Name:         it.Name,
		InfusionName: it.InfusionName,
	}
	if it.IsWeapon || it.IsArmor {
		up := it.CurrentUpgrade
		ref.Upgrade = &up
	}
	if it.HasCurrentAoW && it.CurrentAoWItemID != 0 {
		aow := it.CurrentAoWItemID
		ref.AoWItemID = &aow
	}
	return ref
}
