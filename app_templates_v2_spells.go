package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// resolveSpellWrites translates a Phase 7d.2 SpellsSection (full
// DB-style item IDs like 0x40001770) into Phase 7d.0/7d.3 core
// SpellWrite tuples (raw MagicParam IDs / empty-slot sentinels).
//
// Semantics mirror resolveEquipmentWrites:
//   - sel.Selected(key) == false → no write (live slot unchanged)
//   - ref == nil                 → no write (omitted in template)
//   - ref.BaseItemID == 0        → explicit clear via
//                                  core.EquippedSpellEmptySentinel
//   - ref.BaseItemID != 0        → strip prefix, write raw ID
//
// Two safety nets fire before the prefix strip:
//
//  1. Defensive prefix re-check. Phase 7d.1's validator already
//     rejected non-0x40 prefixes at ingest, but a direct in-process
//     caller could conceivably hand-construct a SpellsSection that
//     bypasses validation. A mismatch here downgrades to a warning
//     (slot skipped), matching the equipment resolver's
//     not-in-inventory style.
//
//  2. DB membership check. A structurally valid 0x40XXXXXX ID that
//     resolves to a non-spell goods item (e.g. a consumable that
//     happens to share the prefix) becomes a warning. Categories are
//     "sorceries" and "incantations" — confirmed in
//     backend/db/data/{sorceries,incantations}.go.
//
// The Go error return is reserved for infrastructure problems (nil
// slot, nil section); per-slot resolution issues never escalate to
// errors. This is intentional symmetry with resolveEquipmentWrites so
// the apply dispatch's "rollback on Go error, accumulate warnings
// otherwise" branch covers both.
func resolveSpellWrites(slot *core.SaveSlot, sel *templates.SectionSelection, sec *templates.SpellsSection) ([]core.SpellWrite, []templates.ImportPreviewIssue, error) {
	if slot == nil {
		return nil, nil, fmt.Errorf("resolveSpellWrites: nil slot")
	}
	if sec == nil {
		return nil, nil, fmt.Errorf("resolveSpellWrites: nil spells section")
	}

	var writes []core.SpellWrite
	var warnings []templates.ImportPreviewIssue

	for slotIdx, slotKey := range templates.SpellSlotOrder {
		if !sel.Selected(slotKey) {
			continue
		}
		ref := templates.SpellSlotRefBySlotKey(sec, slotKey)
		if ref == nil {
			continue
		}
		if ref.BaseItemID == 0 {
			writes = append(writes, core.SpellWrite{
				SlotIndex: slotIdx,
				SpellID:   core.EquippedSpellEmptySentinel,
			})
			continue
		}

		if (ref.BaseItemID & templates.SpellItemIDPrefixMask) != templates.SpellItemIDPrefix {
			warnings = append(warnings, templates.ImportPreviewIssue{
				Severity:   "warning",
				Code:       templates.IssueCodeUnknownItem,
				Message:    fmt.Sprintf("spells.%s: baseItemID 0x%08X has wrong prefix (expected 0x4XXXXXXX); slot skipped", slotKey, ref.BaseItemID),
				Container:  slotKey,
				BaseItemID: ref.BaseItemID,
			})
			continue
		}

		meta := db.GetItemData(ref.BaseItemID)
		if meta.Category != "sorceries" && meta.Category != "incantations" {
			warnings = append(warnings, templates.ImportPreviewIssue{
				Severity:   "warning",
				Code:       templates.IssueCodeUnknownItem,
				Message:    fmt.Sprintf("spells.%s: baseItemID 0x%08X is not a known sorcery or incantation; slot skipped", slotKey, ref.BaseItemID),
				Container:  slotKey,
				BaseItemID: ref.BaseItemID,
			})
			continue
		}

		writes = append(writes, core.SpellWrite{
			SlotIndex: slotIdx,
			SpellID:   db.ItemIDToMagicParamID(ref.BaseItemID),
		})
	}
	return writes, warnings, nil
}
