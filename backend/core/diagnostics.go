package core

import (
	"encoding/binary"
	"fmt"
)

// DiagnosticSeverity classifies corruption findings.
type DiagnosticSeverity string

const (
	SeverityCritical DiagnosticSeverity = "critical" // save will crash the game
	SeverityWarning  DiagnosticSeverity = "warning"  // save may behave unexpectedly
	SeverityInfo     DiagnosticSeverity = "info"     // observation, not necessarily harmful
)

// DiagnosticIssue represents a single corruption finding.
type DiagnosticIssue struct {
	Severity    DiagnosticSeverity `json:"severity"`
	Category    string             `json:"category"` // "offset_chain", "gaitem", "inventory", "stats", "dlc"
	Description string             `json:"description"`
}

// SlotDiagnostics holds the results of a comprehensive slot corruption scan.
type SlotDiagnostics struct {
	SlotIndex int               `json:"slotIndex"`
	Issues    []DiagnosticIssue `json:"issues"`
}

// DiagnoseSaveCorruption performs a comprehensive corruption scan on a slot.
// Returns all found issues sorted by severity.
func DiagnoseSaveCorruption(slot *SaveSlot, slotIndex int) SlotDiagnostics {
	diag := SlotDiagnostics{SlotIndex: slotIndex}

	// 1. Data length
	if len(slot.Data) != SlotSize {
		diag.addCritical("data_size", "slot data length %d != expected %d", len(slot.Data), SlotSize)
		return diag // nothing else is safe to check
	}

	// 2. Offset chain monotonicity
	diag.checkOffsetChain(slot)

	// 3. GaItem integrity
	diag.checkGaItems(slot)

	// 4. Inventory index collisions
	diag.checkInventoryIndices(slot)

	// 5. Stat bounds
	diag.checkStats(slot)

	// DLC section check intentionally omitted — the CSDlc format is not fully
	// understood; bytes [3-49] contain DLC progress data for SotE-active characters
	// and must not be zeroed.

	// 7. GaItemData count
	diag.checkGaItemData(slot)

	// 8. Storage count header
	diag.checkStorageHeader(slot)

	return diag
}

func (d *SlotDiagnostics) addCritical(cat, format string, args ...interface{}) {
	d.Issues = append(d.Issues, DiagnosticIssue{SeverityCritical, cat, fmt.Sprintf(format, args...)})
}

func (d *SlotDiagnostics) addWarning(cat, format string, args ...interface{}) {
	d.Issues = append(d.Issues, DiagnosticIssue{SeverityWarning, cat, fmt.Sprintf(format, args...)})
}

func (d *SlotDiagnostics) addInfo(cat, format string, args ...interface{}) {
	d.Issues = append(d.Issues, DiagnosticIssue{SeverityInfo, cat, fmt.Sprintf(format, args...)})
}

// checkOffsetChain validates monotonicity and bounds of the dynamic offset chain.
func (d *SlotDiagnostics) checkOffsetChain(slot *SaveSlot) {
	if slot.MagicOffset < MinMagicOffset {
		d.addCritical("offset_chain", "MagicOffset 0x%X < minimum 0x%X", slot.MagicOffset, MinMagicOffset)
	}
	if slot.MagicOffset >= SlotSize {
		d.addCritical("offset_chain", "MagicOffset 0x%X >= SlotSize", slot.MagicOffset)
	}

	if slot.InventoryEnd > slot.MagicOffset {
		d.addCritical("offset_chain", "InventoryEnd 0x%X > MagicOffset 0x%X", slot.InventoryEnd, slot.MagicOffset)
	}

	if slot.PlayerDataOffset < slot.MagicOffset {
		d.addWarning("offset_chain", "PlayerDataOffset 0x%X < MagicOffset 0x%X", slot.PlayerDataOffset, slot.MagicOffset)
	}
	if slot.FaceDataOffset < slot.PlayerDataOffset {
		d.addWarning("offset_chain", "FaceDataOffset 0x%X < PlayerDataOffset 0x%X", slot.FaceDataOffset, slot.PlayerDataOffset)
	}
	if slot.StorageBoxOffset < slot.FaceDataOffset {
		d.addWarning("offset_chain", "StorageBoxOffset 0x%X < FaceDataOffset 0x%X", slot.StorageBoxOffset, slot.FaceDataOffset)
	}

	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset >= SlotSize {
		d.addCritical("offset_chain", "EventFlagsOffset 0x%X >= SlotSize", slot.EventFlagsOffset)
	}

	if slot.GaItemDataOffset > 0 && slot.GaItemDataOffset >= SlotSize {
		d.addCritical("offset_chain", "GaItemDataOffset 0x%X >= SlotSize", slot.GaItemDataOffset)
	}
}

// checkGaItems validates GaItem handles and type prefixes.
func (d *SlotDiagnostics) checkGaItems(slot *SaveSlot) {
	if slot.GaItems == nil {
		return
	}

	gaLimit := slot.MagicOffset - DynPlayerData + 1
	invalidHandles := 0
	for i, ga := range slot.GaItems {
		if ga.IsEmpty() {
			continue
		}
		// Validate handle type prefix (upper nibble). ItemID uses different prefixes
		// (0x0=weapon record, 0x1=armor record) and must not be checked here.
		prefix := ga.Handle & GaHandleTypeMask
		switch prefix {
		case ItemTypeWeapon, ItemTypeArmor, ItemTypeAccessory, ItemTypeItem, ItemTypeAow:
			// valid
		default:
			invalidHandles++
			if invalidHandles <= 5 {
				d.addWarning("gaitem", "GaItem[%d] has unknown type prefix 0x%X (handle=0x%08X)", i, prefix, ga.Handle)
			}
		}
	}

	if invalidHandles > 5 {
		d.addWarning("gaitem", "... and %d more invalid GaItem handles", invalidHandles-5)
	}

	// Check if GaItems extend beyond gaLimit
	usedCount := 0
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() {
			usedCount++
		}
	}
	d.addInfo("gaitem", "GaItems: %d used / %d total (limit=0x%X)", usedCount, len(slot.GaItems), gaLimit)
}

// checkInventoryIndices detects duplicate item indices which cause crashes.
func (d *SlotDiagnostics) checkInventoryIndices(slot *SaveSlot) {
	indexMap := make(map[uint32]int)
	duplicates := 0

	checkItems := func(items []InventoryItem, label string) {
		for _, item := range items {
			if item.GaItemHandle == 0 || item.GaItemHandle == GaHandleInvalid {
				continue
			}
			if item.Index <= InvEquipReservedMax {
				d.addWarning("inventory_reserved", "%s item handle 0x%08X has reserved Index=%d (<=432)", label, item.GaItemHandle, item.Index)
			}
			indexMap[item.Index]++
			if indexMap[item.Index] == 2 {
				duplicates++
				if duplicates <= 5 {
					d.addCritical("inventory", "duplicate item Index=%d in %s (handle 0x%08X)", item.Index, label, item.GaItemHandle)
				}
			}
		}
	}

	checkItems(slot.Inventory.CommonItems, "inventory/common")
	checkItems(slot.Inventory.KeyItems, "inventory/key")

	if duplicates > 5 {
		d.addCritical("inventory", "... and %d more duplicate indices", duplicates-5)
	}
}

// checkStats validates player stat ranges.
func (d *SlotDiagnostics) checkStats(slot *SaveSlot) {
	if slot.Player.Level == 0 || slot.Player.Level > 713 {
		d.addCritical("stats", "Level %d out of range [1, 713]", slot.Player.Level)
	}

	type statDef struct {
		name string
		val  uint32
	}
	stats := []statDef{
		{"Vigor", slot.Player.Vigor},
		{"Mind", slot.Player.Mind},
		{"Endurance", slot.Player.Endurance},
		{"Strength", slot.Player.Strength},
		{"Dexterity", slot.Player.Dexterity},
		{"Intelligence", slot.Player.Intelligence},
		{"Faith", slot.Player.Faith},
		{"Arcane", slot.Player.Arcane},
	}
	for _, s := range stats {
		if s.val < 1 || s.val > 99 {
			d.addCritical("stats", "%s=%d out of range [1, 99]", s.name, s.val)
		}
	}

	if slot.Player.Class > 9 {
		d.addWarning("stats", "Class=%d out of range [0, 9]", slot.Player.Class)
	}
	if slot.Player.Gender > 1 {
		d.addWarning("stats", "Gender=%d out of range [0, 1]", slot.Player.Gender)
	}

	// Level formula check: Level = sum(attrs) - 79
	attrSum := slot.Player.Vigor + slot.Player.Mind + slot.Player.Endurance +
		slot.Player.Strength + slot.Player.Dexterity + slot.Player.Intelligence +
		slot.Player.Faith + slot.Player.Arcane
	expectedLevel := attrSum - 79
	if slot.Player.Level != expectedLevel {
		// Category "stats_formula" — not automatically repairable (would change character level)
		d.addWarning("stats_formula", "Level %d != expected %d (sum(attrs)=%d - 79)", slot.Player.Level, expectedLevel, attrSum)
	}
}

// checkDLCSection validates the CSDlc region.
func (d *SlotDiagnostics) checkDLCSection(slot *SaveSlot) {
	if DlcSectionOffset+DlcSectionSize > len(slot.Data) {
		return
	}

	dlc := slot.Data[DlcSectionOffset : DlcSectionOffset+DlcSectionSize]

	// Byte[1] = SotE entry flag
	if dlc[DlcEntryFlagByte] != 0 {
		d.addInfo("dlc", "Shadow of the Erdtree entry flag is set (byte[1]=0x%02X)", dlc[DlcEntryFlagByte])
	}

	// Bytes[3-49] should be 0x00
	for i := 3; i < DlcSectionSize; i++ {
		if dlc[i] != 0 {
			d.addWarning("dlc", "DLC section byte[%d]=0x%02X (expected 0x00)", i, dlc[i])
			break // report first occurrence only
		}
	}
}

// checkGaItemData validates the GaItemData section count.
func (d *SlotDiagnostics) checkGaItemData(slot *SaveSlot) {
	if slot.GaItemDataOffset <= 0 || slot.GaItemDataOffset+GaItemDataArrayOff >= len(slot.Data) {
		return
	}

	count := binary.LittleEndian.Uint32(slot.Data[slot.GaItemDataOffset : slot.GaItemDataOffset+4])
	if count > GaItemDataMaxCount {
		d.addCritical("gaitemdata", "GaItemData count %d > max %d", count, GaItemDataMaxCount)
	} else {
		d.addInfo("gaitemdata", "GaItemData: %d entries (max %d)", count, GaItemDataMaxCount)
	}
}

// checkStorageHeader validates the storage item count header.
func (d *SlotDiagnostics) checkStorageHeader(slot *SaveSlot) {
	if slot.StorageBoxOffset <= 0 || slot.StorageBoxOffset+4 >= len(slot.Data) {
		return
	}

	headerCount := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset : slot.StorageBoxOffset+4])

	// Count actual non-empty storage items
	actualCount := uint32(0)
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	for i := 0; i < StorageCommonCount; i++ {
		off := storageStart + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h != GaHandleEmpty && h != GaHandleInvalid {
			actualCount++
		}
	}

	if headerCount != actualCount {
		d.addWarning("storage", "storage header count %d != actual items %d", headerCount, actualCount)
	}
}

// IntegrityError describes a single post-mutation invariant violation.
type IntegrityError struct {
	Check   string `json:"check"`
	Message string `json:"message"`
}

func (e IntegrityError) Error() string {
	return fmt.Sprintf("%s: %s", e.Check, e.Message)
}

// DuplicateInventoryIndexIssue describes a single Index collision discovered by
// ScanDuplicateInventoryIndices. Used by pre-flight guards to abort mutations on
// already-corrupt saves with a precise diagnostic instead of a misleading
// post-mutation rollback.
type DuplicateInventoryIndexIssue struct {
	Index           uint32 `json:"index"`
	Scope           string `json:"scope"` // "inventory_common" | "inventory_key"
	FirstRow        int    `json:"firstRow"`
	FirstHandle     uint32 `json:"firstHandle"`
	DuplicateRow    int    `json:"duplicateRow"`
	DuplicateHandle uint32 `json:"duplicateHandle"`
}

// ScanDuplicateInventoryIndices walks Inventory.CommonItems and Inventory.KeyItems
// and reports every Index value that appears more than once across the combined
// inventory list. Empty / invalid handles are ignored. Storage is not scanned —
// duplicate post-mutation validation only covers inventory.
//
// Read-only: never modifies slot. Safe to call before snapshot/mutation as a
// pre-flight guard.
func ScanDuplicateInventoryIndices(slot *SaveSlot) []DuplicateInventoryIndexIssue {
	if slot == nil {
		return nil
	}
	type seenEntry struct {
		row    int
		handle uint32
	}
	seen := make(map[uint32]seenEntry)
	var issues []DuplicateInventoryIndexIssue

	for i, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		if prev, ok := seen[item.Index]; ok {
			issues = append(issues, DuplicateInventoryIndexIssue{
				Index:           item.Index,
				Scope:           "inventory_common",
				FirstRow:        prev.row,
				FirstHandle:     prev.handle,
				DuplicateRow:    i,
				DuplicateHandle: item.GaItemHandle,
			})
			continue
		}
		seen[item.Index] = seenEntry{row: i, handle: item.GaItemHandle}
	}
	for i, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		if prev, ok := seen[item.Index]; ok {
			issues = append(issues, DuplicateInventoryIndexIssue{
				Index:           item.Index,
				Scope:           "inventory_key",
				FirstRow:        prev.row,
				FirstHandle:     prev.handle,
				DuplicateRow:    i,
				DuplicateHandle: item.GaItemHandle,
			})
			continue
		}
		seen[item.Index] = seenEntry{row: i, handle: item.GaItemHandle}
	}
	return issues
}

// ValidatePostMutation performs fast invariant checks after a slot mutation.
// Only checks crash-causing conditions — not full diagnostic scan.
// Returns nil if all checks pass, or a slice of violations.
//
// Fail-closed: every duplicate acquisition Index across Inventory.CommonItems
// + KeyItems is reported. Callers run a fail-closed pre-flight
// (ScanDuplicateInventoryIndices) before mutating and refuse to proceed when
// the slot already holds duplicates, so any duplicate observed here was
// introduced by the mutation itself and must roll back.
func ValidatePostMutation(slot *SaveSlot) []IntegrityError {
	var errs []IntegrityError

	// 1. Every non-empty inventory handle must exist in GaMap (for non-stackable types).
	for i, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		typeBits := item.GaItemHandle & GaHandleTypeMask
		if typeBits == ItemTypeWeapon || typeBits == ItemTypeArmor || typeBits == ItemTypeAow {
			if _, ok := slot.GaMap[item.GaItemHandle]; !ok {
				errs = append(errs, IntegrityError{
					Check:   "orphan_handle",
					Message: fmt.Sprintf("inventory[%d] handle 0x%08X not in GaMap", i, item.GaItemHandle),
				})
			}
		}
	}

	// 2. No duplicate Index values across inventory CommonItems + KeyItems.
	indexSeen := make(map[uint32]bool)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		if indexSeen[item.Index] {
			errs = append(errs, IntegrityError{
				Check:   "duplicate_index",
				Message: fmt.Sprintf("duplicate Index %d in inventory common (handle 0x%08X)", item.Index, item.GaItemHandle),
			})
		}
		indexSeen[item.Index] = true
	}
	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		if indexSeen[item.Index] {
			errs = append(errs, IntegrityError{
				Check:   "duplicate_index",
				Message: fmt.Sprintf("duplicate Index %d in inventory key (handle 0x%08X)", item.Index, item.GaItemHandle),
			})
		}
		indexSeen[item.Index] = true
	}

	// 3. GaItemData count header within bounds.
	if slot.GaItemDataOffset > 0 && slot.GaItemDataOffset+4 <= len(slot.Data) {
		count := binary.LittleEndian.Uint32(slot.Data[slot.GaItemDataOffset:])
		if count > GaItemDataMaxCount {
			errs = append(errs, IntegrityError{
				Check:   "gaitemdata_count",
				Message: fmt.Sprintf("GaItemData count %d > max %d", count, GaItemDataMaxCount),
			})
		}
	}

	// 5. Storage count header matches actual non-empty items.
	if slot.StorageBoxOffset > 0 && slot.StorageBoxOffset+4 <= len(slot.Data) {
		headerCount := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:])
		actualCount := uint32(0)
		storageStart := slot.StorageBoxOffset + StorageHeaderSkip
		for i := 0; i < StorageCommonCount; i++ {
			off := storageStart + i*InvRecordLen
			if off+InvRecordLen > len(slot.Data) {
				break
			}
			h := binary.LittleEndian.Uint32(slot.Data[off:])
			if h != GaHandleEmpty && h != GaHandleInvalid {
				actualCount++
			}
		}
		if headerCount != actualCount {
			errs = append(errs, IntegrityError{
				Check:   "storage_count",
				Message: fmt.Sprintf("storage header count %d != actual %d", headerCount, actualCount),
			})
		}
	}

	// 6. NextAoWIndex <= NextArmamentIndex <= len(GaItems).
	if slot.NextAoWIndex > slot.NextArmamentIndex {
		errs = append(errs, IntegrityError{
			Check:   "gaitem_indices",
			Message: fmt.Sprintf("NextAoWIndex %d > NextArmamentIndex %d", slot.NextAoWIndex, slot.NextArmamentIndex),
		})
	}
	if slot.NextArmamentIndex > len(slot.GaItems) {
		errs = append(errs, IntegrityError{
			Check:   "gaitem_indices",
			Message: fmt.Sprintf("NextArmamentIndex %d > len(GaItems) %d", slot.NextArmamentIndex, len(slot.GaItems)),
		})
	}

	// 7. No GaMap entry references itemID=0.
	for handle, itemID := range slot.GaMap {
		if itemID == 0 {
			errs = append(errs, IntegrityError{
				Check:   "gamap_zero_id",
				Message: fmt.Sprintf("GaMap handle 0x%08X maps to itemID=0", handle),
			})
		}
	}

	return errs
}

// RepairStats clamps player level and attributes to valid game ranges.
// Calls SyncPlayerToData on any change. Returns list of applied fixes.
func RepairStats(slot *SaveSlot) []string {
	var fixed []string
	changed := false

	if slot.Player.Level == 0 || slot.Player.Level > 713 {
		clamped := slot.Player.Level
		if clamped == 0 {
			clamped = 1
		} else {
			clamped = 713
		}
		fixed = append(fixed, fmt.Sprintf("Level %d → %d", slot.Player.Level, clamped))
		slot.Player.Level = clamped
		changed = true
	}

	type statDef struct {
		name string
		ptr  *uint32
	}
	attrs := []statDef{
		{"Vigor", &slot.Player.Vigor},
		{"Mind", &slot.Player.Mind},
		{"Endurance", &slot.Player.Endurance},
		{"Strength", &slot.Player.Strength},
		{"Dexterity", &slot.Player.Dexterity},
		{"Intelligence", &slot.Player.Intelligence},
		{"Faith", &slot.Player.Faith},
		{"Arcane", &slot.Player.Arcane},
	}
	for _, s := range attrs {
		v := *s.ptr
		if v < 1 || v > 99 {
			c := v
			if c < 1 {
				c = 1
			} else {
				c = 99
			}
			fixed = append(fixed, fmt.Sprintf("%s %d → %d", s.name, v, c))
			*s.ptr = c
			changed = true
		}
	}
	if changed {
		slot.SyncPlayerToData()
	}
	return fixed
}

// RepairStorageCountHeader recalculates the storage box item count header.
// Returns true if the header was corrected.
func RepairStorageCountHeader(slot *SaveSlot) bool {
	if slot.StorageBoxOffset <= 0 || slot.StorageBoxOffset+4 > len(slot.Data) {
		return false
	}
	headerCount := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:])
	actualCount := uint32(0)
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	for i := 0; i < StorageCommonCount; i++ {
		off := storageStart + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h != GaHandleEmpty && h != GaHandleInvalid {
			actualCount++
		}
	}
	if headerCount == actualCount {
		return false
	}
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], actualCount)
	return true
}

// RepairGaItemDataCount caps the GaItemData count header to the allowed maximum.
// Returns true if the value was changed.
func RepairGaItemDataCount(slot *SaveSlot) bool {
	if slot.GaItemDataOffset <= 0 || slot.GaItemDataOffset+4 > len(slot.Data) {
		return false
	}
	count := binary.LittleEndian.Uint32(slot.Data[slot.GaItemDataOffset:])
	if count <= GaItemDataMaxCount {
		return false
	}
	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset:], uint32(GaItemDataMaxCount))
	return true
}

// RepairDLCSection zeros the reserved trailing bytes (indices 3–49) of the DLC section.
// Returns true if any byte was changed.
func RepairDLCSection(slot *SaveSlot) bool {
	if DlcSectionOffset+DlcSectionSize > len(slot.Data) {
		return false
	}
	dlc := slot.Data[DlcSectionOffset : DlcSectionOffset+DlcSectionSize]
	changed := false
	for i := 3; i < DlcSectionSize; i++ {
		if dlc[i] != 0 {
			dlc[i] = 0
			changed = true
		}
	}
	return changed
}

// RepairSlot applies all available automated repairs to the slot.
// Returns lists of what was fixed and what was skipped (unrepairable).
func RepairSlot(slot *SaveSlot) (fixed, skipped []string) {
	fixed = []string{}
	skipped = []string{}
	fixed = append(fixed, RepairStats(slot)...)

	if RepairStorageCountHeader(slot) {
		fixed = append(fixed, "storage count header recalculated")
	}
	if RepairGaItemDataCount(slot) {
		fixed = append(fixed, "GaItemData count capped to maximum")
	}
	// RepairDLCSection intentionally removed — see checkDLCSection comment

	dupes := ScanDuplicateInventoryIndices(slot)
	physick := ScanDuplicateWondrousPhysick(slot)
	if len(dupes) > 0 || len(physick) > 0 {
		r, err := RepairDuplicateInventoryIndices(slot)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("duplicate inventory indices: %v", err))
		} else if r.Changed > 0 {
			fixed = append(fixed, fmt.Sprintf("fixed %d duplicate inventory index(es)", r.Changed))
		}
		if _, err := RepairDuplicateWondrousPhysick(slot); err != nil {
			skipped = append(skipped, fmt.Sprintf("Wondrous Physick duplicates: %v", err))
		}
	}
	return
}
