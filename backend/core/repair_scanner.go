package core

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// Issue code constants — match the codes used in backend/editor/validate.go
// and in the UI so the frontend can use a single stable key for each problem.
const (
	RepairCodeDuplicateHandle           = "duplicate_handle"
	RepairCodeDuplicateUID              = "duplicate_uid"
	RepairCodeUnknownItemID             = "unknown_item_id"
	RepairCodeQuantityZero              = "quantity_zero"
	RepairCodePassThroughRecords        = "pass_through_records"
	RepairCodeCurrentAoWMissing         = "current_aow_missing"
	RepairCodeCurrentAoWShared          = "current_aow_shared"
	RepairCodeCurrentAoWNonAoWCategory  = "current_aow_non_aow_category"
	RepairCodeInventoryReserved         = "inventory_reserved"
	RepairCodeDuplicateAcquisitionIndex = "duplicate_acquisition_index"
	RepairCodeStatsFormula              = "stats_formula"
)

// Repair action identifiers — proposed by the scanner, executed by the apply endpoint.
const (
	RepairActionCreateCopy   = "create_copy"
	RepairActionRemoveRecord = "remove_record"
	RepairActionClearAoW     = "clear_aow"
	RepairActionPickAoW      = "pick_aow"
	RepairActionRepairIndex  = "repair_index"
	RepairActionFixLevel     = "fix_level"
	RepairActionNoAction     = "no_action"
)

const (
	repairDomainInventory = "inventory"
	repairDomainAoW       = "aow"
	repairDomainStats     = "stats"

	repairScopeInventoryCommon = "inventory_common"
	repairScopeInventoryKey    = "inventory_key"
	repairScopeStorageCommon   = "storage_common"
	repairScopeStats           = "stats"

	repairSeverityError   = "error"
	repairSeverityWarning = "warning"
	repairSeverityInfo    = "info"
)

// IssueKey uniquely and structurally identifies one repair issue.
// issueID = hex(SHA-256(canonical JSON of IssueKey)[:8]) — 16 hex chars.
// debugKey is the human-readable pipe-separated form for logs only; never parse it.
type IssueKey struct {
	Slot   int    `json:"slot"`
	Domain string `json:"domain"`
	Code   string `json:"code"`
	Scope  string `json:"scope"`
	Row    int    `json:"row"`
	Handle uint32 `json:"handle"`
	Field  string `json:"field,omitempty"`
	Value  string `json:"value,omitempty"`
}

// RepairIssue is one problem found by ScanRepairIssues.
// It is read-only — no slot mutation occurs during scanning.
type RepairIssue struct {
	IssueID       string   `json:"issueID"`
	DebugKey      string   `json:"debugKey"`
	Fingerprint   string   `json:"fingerprint"`
	Key           IssueKey `json:"key"`
	Description   string   `json:"description"`
	Severity      string   `json:"severity"`
	Actions       []string `json:"actions"`
	DefaultAction string   `json:"defaultAction"`
}

// ScanRepairIssues returns all repair issues found in slot. Read-only.
func ScanRepairIssues(slotIndex int, slot *SaveSlot) []RepairIssue {
	var out []RepairIssue
	out = append(out, scanInventoryRepairIssues(slotIndex, slot)...)
	out = append(out, scanAoWRepairIssues(slotIndex, slot)...)
	out = append(out, scanStatsRepairIssues(slotIndex, slot)...)
	return out
}

// ---- helpers ----------------------------------------------------------------

func repairIssueID(key IssueKey) string { return IssueKeyID(key) }

// IssueKeyID computes the stable issueID for a given IssueKey (hex SHA-256[:8]).
func IssueKeyID(key IssueKey) string {
	b, _ := json.Marshal(key)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

func repairDebugKey(key IssueKey) string {
	s := fmt.Sprintf("slot:%d|domain:%s|code:%s|scope:%s|row:%d|handle:0x%08X",
		key.Slot, key.Domain, key.Code, key.Scope, key.Row, key.Handle)
	if key.Field != "" {
		s += fmt.Sprintf("|field:%s|value:%s", key.Field, key.Value)
	}
	return s
}

// FingerprintRecordAt returns the fingerprint of the inventory/storage record
// currently at scope+row, so a repair apply endpoint can stale-check a target
// against the state captured at scan time before dispatching a primitive.
// ok=false when scope is unknown or row is out of range.
func FingerprintRecordAt(slot *SaveSlot, scope string, row int) (string, bool) {
	if slot == nil {
		return "", false
	}
	var list []InventoryItem
	switch scope {
	case repairScopeInventoryCommon:
		list = slot.Inventory.CommonItems
	case repairScopeInventoryKey:
		list = slot.Inventory.KeyItems
	case repairScopeStorageCommon:
		list = slot.Storage.CommonItems
	default:
		return "", false
	}
	if row < 0 || row >= len(list) {
		return "", false
	}
	return fingerprintInventoryItem(list[row]), true
}

func fingerprintInventoryItem(item InventoryItem) string {
	var b [12]byte
	binary.LittleEndian.PutUint32(b[0:], item.GaItemHandle)
	binary.LittleEndian.PutUint32(b[4:], item.Quantity)
	binary.LittleEndian.PutUint32(b[8:], item.Index)
	h := sha256.Sum256(b[:])
	return fmt.Sprintf("%x", h[:8])
}

func mkIssue(key IssueKey, desc, severity string, actions []string, def, fp string) RepairIssue {
	return RepairIssue{
		IssueID:       repairIssueID(key),
		DebugKey:      repairDebugKey(key),
		Fingerprint:   fp,
		Key:           key,
		Description:   desc,
		Severity:      severity,
		Actions:       actions,
		DefaultAction: def,
	}
}

// resolveItemID returns the itemID for an inventory handle:
// slot.GaMap first (weapon/armor GaItems), then db.HandleToItemID (goods/talismans).
func resolveItemID(slot *SaveSlot, h uint32) uint32 {
	if id, ok := slot.GaMap[h]; ok {
		return id
	}
	return db.HandleToItemID(h)
}

// ---- inventory scanner ------------------------------------------------------

func scanInventoryRepairIssues(slotIndex int, slot *SaveSlot) []RepairIssue {
	type scopedSection struct {
		items      []InventoryItem
		scope      string
		indexDedup bool // participates in duplicate_acquisition_index check
	}
	// Storage does NOT share the acquisition-index dedup map with inventory
	// (matches semantics of core.ScanDuplicateInventoryIndices).
	sections := []scopedSection{
		{slot.Inventory.CommonItems, repairScopeInventoryCommon, true},
		{slot.Inventory.KeyItems, repairScopeInventoryKey, true},
		{slot.Storage.CommonItems, repairScopeStorageCommon, false},
	}

	var out []RepairIssue
	seenHandles := make(map[uint32]bool)
	seenIndices := make(map[uint32]bool) // only populated when sec.indexDedup == true
	passthroughCount := 0

	for _, sec := range sections {
		for row, item := range sec.items {
			h := item.GaItemHandle
			if h == GaHandleEmpty || h == GaHandleInvalid {
				continue
			}

			prefix := h & GaHandleTypeMask
			knownType := prefix == ItemTypeWeapon || prefix == ItemTypeArmor ||
				prefix == ItemTypeAccessory || prefix == ItemTypeItem || prefix == ItemTypeAow
			if !knownType {
				passthroughCount++
				continue
			}

			fp := fingerprintInventoryItem(item)

			// duplicate_handle + duplicate_uid: both emitted when the same
			// handle appears more than once. duplicate_uid is a consequence of
			// duplicate_handle (UID = "hnd:0x%08X" in workspace layer).
			//
			// Excluded for ItemTypeAccessory (talismans): that handle is
			// item-derived (db.HandleToItemID), not allocated per-instance,
			// so every copy of the same talisman legitimately shares a
			// handle — in one container or split across inventory/storage.
			// That is normal save state, not corruption (see
			// backend/editor's matching exception in classifyRecord /
			// Validate).
			if prefix != ItemTypeAccessory {
				if seenHandles[h] {
					keyH := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeDuplicateHandle,
						Scope: sec.scope, Row: row, Handle: h}
					out = append(out, mkIssue(keyH,
						fmt.Sprintf("handle 0x%08X appears more than once", h),
						repairSeverityError,
						[]string{RepairActionCreateCopy},
						RepairActionCreateCopy, fp))

					keyU := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeDuplicateUID,
						Scope: sec.scope, Row: row, Handle: h}
					out = append(out, mkIssue(keyU,
						fmt.Sprintf("handle 0x%08X causes duplicate UID (hnd:0x%08X)", h, h),
						repairSeverityError,
						[]string{RepairActionCreateCopy},
						RepairActionCreateCopy, fp))
				}
				seenHandles[h] = true
			}

			// unknown_item_id: resolve itemID via GaMap then db.HandleToItemID,
			// then check DB. Omitting GaMap lookup is NOT sufficient — goods and
			// talismans are handle-encoded and don't need a GaItem entry.
			itemID := resolveItemID(slot, h)
			itemData, _ := db.GetItemDataFuzzy(itemID)
			if itemData.Name == "" {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeUnknownItemID,
					Scope: sec.scope, Row: row, Handle: h}
				out = append(out, mkIssue(key,
					fmt.Sprintf("handle 0x%08X (itemID 0x%08X) not found in item DB", h, itemID),
					repairSeverityError,
					[]string{RepairActionNoAction},
					RepairActionNoAction, fp))
			}

			if item.Quantity&0x7FFFFFFF == 0 {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeQuantityZero,
					Scope: sec.scope, Row: row, Handle: h}
				out = append(out, mkIssue(key,
					fmt.Sprintf("item 0x%08X has quantity 0", h),
					repairSeverityError,
					[]string{RepairActionRemoveRecord},
					RepairActionRemoveRecord, fp))
			}

			if item.Index > 0 && item.Index <= 432 {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeInventoryReserved,
					Scope: sec.scope, Row: row, Handle: h,
					Field: "index", Value: fmt.Sprintf("%d", item.Index)}
				out = append(out, mkIssue(key,
					fmt.Sprintf("item 0x%08X has reserved acquisition index %d (≤432)", h, item.Index),
					repairSeverityWarning,
					[]string{RepairActionRepairIndex},
					RepairActionRepairIndex, fp))
			}

			if sec.indexDedup {
				if item.Index > 0 && seenIndices[item.Index] {
					key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeDuplicateAcquisitionIndex,
						Scope: sec.scope, Row: row, Handle: h,
						Field: "index", Value: fmt.Sprintf("%d", item.Index)}
					out = append(out, mkIssue(key,
						fmt.Sprintf("item 0x%08X shares acquisition index %d", h, item.Index),
						repairSeverityError,
						[]string{RepairActionRepairIndex},
						RepairActionRepairIndex, fp))
				}
				seenIndices[item.Index] = true
			}
		}
	}

	if passthroughCount > 0 {
		key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodePassThroughRecords}
		out = append(out, mkIssue(key,
			fmt.Sprintf("%d records with unrecognised handle type will round-trip unchanged", passthroughCount),
			repairSeverityInfo,
			[]string{RepairActionNoAction},
			RepairActionNoAction, ""))
	}

	return out
}

// ---- AoW scanner ------------------------------------------------------------

func scanAoWRepairIssues(slotIndex int, slot *SaveSlot) []RepairIssue {
	// Replicate editor.buildWeaponAoWMaps without importing that package.
	weaponAoWRefs := make(map[uint32]uint32) // weaponHandle → aowHandle
	aowRefCount := make(map[uint32]int)      // aowHandle → reference count

	for _, g := range slot.GaItems {
		if g.IsEmpty() {
			continue
		}
		if g.Handle&GaHandleTypeMask != ItemTypeWeapon {
			continue
		}
		if IsNoCustomAoWHandle(g.AoWGaItemHandle) {
			continue
		}
		weaponAoWRefs[g.Handle] = g.AoWGaItemHandle
		aowRefCount[g.AoWGaItemHandle]++
	}

	type scopedSection struct {
		items []InventoryItem
		scope string
	}
	sections := []scopedSection{
		{slot.Inventory.CommonItems, repairScopeInventoryCommon},
		{slot.Storage.CommonItems, repairScopeStorageCommon},
	}

	var out []RepairIssue
	for _, sec := range sections {
		for row, item := range sec.items {
			h := item.GaItemHandle
			if h == GaHandleEmpty || h == GaHandleInvalid {
				continue
			}
			if h&GaHandleTypeMask != ItemTypeWeapon {
				continue
			}
			aowHandle, hasRef := weaponAoWRefs[h]
			if !hasRef {
				continue
			}
			fp := fingerprintInventoryItem(item)

			// current_aow_missing: AoW handle not in GaMap (matches populateCurrentAoW).
			aowItemID, mapped := slot.GaMap[aowHandle]
			if !mapped || aowItemID == 0 {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainAoW, Code: RepairCodeCurrentAoWMissing,
					Scope: sec.scope, Row: row, Handle: h,
					Field: "aow", Value: fmt.Sprintf("0x%08X", aowHandle)}
				out = append(out, mkIssue(key,
					fmt.Sprintf("weapon 0x%08X references AoW 0x%08X which has no GaItem", h, aowHandle),
					repairSeverityWarning,
					[]string{RepairActionClearAoW, RepairActionPickAoW},
					RepairActionClearAoW, fp))
				continue
			}

			// current_aow_shared: same AoW handle used by more than one weapon.
			if aowRefCount[aowHandle] > 1 {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainAoW, Code: RepairCodeCurrentAoWShared,
					Scope: sec.scope, Row: row, Handle: h,
					Field: "aow", Value: fmt.Sprintf("0x%08X", aowHandle)}
				out = append(out, mkIssue(key,
					fmt.Sprintf("weapon 0x%08X shares AoW 0x%08X with %d other weapon(s)", h, aowHandle, aowRefCount[aowHandle]-1),
					repairSeverityWarning,
					[]string{RepairActionCreateCopy, RepairActionClearAoW},
					RepairActionCreateCopy, fp))
				continue
			}

			// current_aow_non_aow_category: AoW itemID resolves in DB but category
			// is not "ashes_of_war". Matches editor.Validate / validate.go check.
			aowData, _ := db.GetItemDataFuzzy(aowItemID)
			if aowData.Name != "" && aowData.Category != "ashes_of_war" {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainAoW, Code: RepairCodeCurrentAoWNonAoWCategory,
					Scope: sec.scope, Row: row, Handle: h,
					Field: "aow", Value: fmt.Sprintf("0x%08X", aowHandle)}
				out = append(out, mkIssue(key,
					fmt.Sprintf("weapon 0x%08X AoW 0x%08X is %q (category %q, not ashes_of_war)",
						h, aowHandle, aowData.Name, aowData.Category),
					repairSeverityWarning,
					[]string{RepairActionClearAoW},
					RepairActionClearAoW, fp))
			}
		}
	}
	return out
}

// ---- stats scanner ----------------------------------------------------------

func scanStatsRepairIssues(slotIndex int, slot *SaveSlot) []RepairIssue {
	if slot.Player.Level == 0 {
		return nil
	}
	attrSum := slot.Player.Vigor + slot.Player.Mind + slot.Player.Endurance +
		slot.Player.Strength + slot.Player.Dexterity + slot.Player.Intelligence +
		slot.Player.Faith + slot.Player.Arcane
	expected := attrSum - 79
	if slot.Player.Level == expected {
		return nil
	}

	// Fingerprint covers Level + all 8 attributes — expectedLevel depends on attrs.
	var b [36]byte
	binary.LittleEndian.PutUint32(b[0:], slot.Player.Level)
	binary.LittleEndian.PutUint32(b[4:], slot.Player.Vigor)
	binary.LittleEndian.PutUint32(b[8:], slot.Player.Mind)
	binary.LittleEndian.PutUint32(b[12:], slot.Player.Endurance)
	binary.LittleEndian.PutUint32(b[16:], slot.Player.Strength)
	binary.LittleEndian.PutUint32(b[20:], slot.Player.Dexterity)
	binary.LittleEndian.PutUint32(b[24:], slot.Player.Intelligence)
	binary.LittleEndian.PutUint32(b[28:], slot.Player.Faith)
	binary.LittleEndian.PutUint32(b[32:], slot.Player.Arcane)
	h := sha256.Sum256(b[:])
	fp := fmt.Sprintf("%x", h[:8])

	key := IssueKey{Slot: slotIndex, Domain: repairDomainStats, Code: RepairCodeStatsFormula,
		Scope: repairScopeStats, Field: "level", Value: fmt.Sprintf("%d", slot.Player.Level)}
	return []RepairIssue{mkIssue(key,
		fmt.Sprintf("Level %d ≠ expected %d (sum(attrs)=%d − 79)", slot.Player.Level, expected, attrSum),
		repairSeverityWarning,
		[]string{RepairActionFixLevel},
		RepairActionFixLevel, fp)}
}
