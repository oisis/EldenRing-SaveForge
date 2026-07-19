package main

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// actionGameItemsAdd is the internal action tag for a Database Add mutation
// (AddItemsToCharacter / AddItemsToCharacterWithGameLimits) in the Game Items
// lifecycle journal.
const actionGameItemsAdd = "add_items"

// actionGameItemsRemove is the internal action tag for a Database Remove mutation
// (RemoveItemsFromCharacter) in the Game Items lifecycle journal.
const actionGameItemsRemove = "remove_items"

// Action tags for the four single-item unlock operations exposed by the Game
// Items Database tab. They deliberately identify the user-visible operation,
// rather than the shared World-tab implementation that happens to own it.
const (
	actionGameItemsUnlockCookbook      = "unlock_cookbook"
	actionGameItemsBulkSetCookbooks    = "bulk_set_cookbooks"
	actionGameItemsUnlockWhetblade     = "unlock_whetblade"
	actionGameItemsUnlockBellBearing   = "unlock_bell_bearing"
	actionGameItemsBulkSetBellBearings = "bulk_set_bell_bearings"
	actionGameItemsUnlockMapFragment   = "unlock_map_fragment"
	actionGameItemsWorkspaceMove       = "workspace_move_item"
	actionGameItemsWorkspaceTransfer   = "workspace_transfer_item"
	actionGameItemsWorkspaceReorder    = "workspace_reorder_items"
	actionGameItemsWorkspaceAdd        = "workspace_add_item"
	actionGameItemsWorkspaceWeapon     = "workspace_update_weapon"
	actionGameItemsWorkspaceRemove     = "workspace_remove_item"
	actionGameItemsWorkspaceRepair     = "workspace_auto_repair"
	actionGameItemsWorkspaceSave       = "workspace_save"
	actionGameItemsGaItemDeduplicate   = "gaitem_deduplicate"
	actionGameItemsGaItemRepack        = "gaitem_repack"
)

// stageGameItemsApplyAddPlan is the finished-phase stage reported when the real
// executor (applyItemAddMutationPlan) fails and the slot was rolled back.
const stageGameItemsApplyAddPlan = "apply_add_plan"

// stageGameItemsRemoveItem is the finished-phase stage reported when a single
// core.RemoveItemFromSlot call fails and the real slot is left partially mutated
// (RemoveItemsFromCharacter performs no rollback).
const stageGameItemsRemoveItem = "remove_item"

const stageGameItemsUnlock = "apply_unlock"

const (
	stageGameItemsWorkspace     = "apply_workspace"
	stageGameItemsWorkspaceSave = "save_workspace"
)

// Lifecycle event names for a single Game Items field mutation. Like the
// Character lifecycle, a future Game Items mutation endpoint emits, per changed
// field, one record for each phase in this order: before -> planned -> finished.
// These are the on-disk contract for a Game Items save mutation; this file is
// the foundation only and is not yet wired into any mutation.
const (
	eventGameItemsChangeBefore   = "game_items_change_before"
	eventGameItemsChangePlanned  = "game_items_change_planned"
	eventGameItemsChangeFinished = "game_items_change_finished"
)

// Game Items lifecycle reuses the Character change-field and outcome types on
// purpose: the journal contract (action, character_index, field, before, after,
// outcome, stage) is identical, so a diagnostics reader treats a Game Items
// mutation exactly like a Character one, and there is no parallel unrestricted
// outcome vocabulary to special-case. The three helpers below route through the
// same journalChangeRecords loop as their Character counterparts.

// journalGameItemChangeBefore records the pre-change state of each Game Items
// field, one debug record per field, before any new value is computed. Only
// "before" is meaningful at this phase, so "after" is omitted.
func (a *App) journalGameItemChangeBefore(action string, characterIndex int, changes []characterFieldChange) {
	a.journalChangeRecords(eventGameItemsChangeBefore, "game items change before", action, characterIndex, changes, nil)
}

// journalGameItemChangePlanned records the intended new value of each Game Items
// field, one debug record per field, after it is computed but before it is applied.
func (a *App) journalGameItemChangePlanned(action string, characterIndex int, changes []characterFieldChange) {
	a.journalChangeRecords(eventGameItemsChangePlanned, "game items change planned", action, characterIndex, changes, changePlannedTail)
}

// journalGameItemChangeFinished records the terminal state of each Game Items
// field, one debug record per field, once the mutation has run. outcome and
// stage report how and where it ended; After holds the actual applied (or
// attempted) value.
func (a *App) journalGameItemChangeFinished(action string, characterIndex int, outcome characterChangeOutcome, stage string, changes []characterFieldChange) {
	a.journalChangeRecords(eventGameItemsChangeFinished, "game items change finished", action, characterIndex, changes, changeFinishedTail(outcome, stage))
}

// journalGameItemsRemoveFinished emits the finished phase for a Database Remove:
// direct physical rows first, then the two count headers, re-reading every field
// out of the (possibly partially mutated) real slot so After reflects what
// actually landed. It is a no-op when no field changed, so a no-op removal emits
// nothing. RemoveItemsFromCharacter performs no rollback, so on error the same
// re-read simply reports the partial state.
func (a *App) journalGameItemsRemoveFinished(charIdx int, outcome characterChangeOutcome, stage string, direct []gameItemFieldPlan, invHeader, storageHeader, cleanupFlags []gameItemSideEffectPlan, slot *core.SaveSlot) {
	if len(direct) == 0 && len(invHeader) == 0 && len(storageHeader) == 0 && len(cleanupFlags) == 0 {
		return
	}
	finished := append(gameItemFinishedRecords(direct, slot), gameItemSideEffectFinishedRecords(invHeader, slot)...)
	finished = append(finished, gameItemSideEffectFinishedRecords(storageHeader, slot)...)
	finished = append(finished, gameItemSideEffectFinishedRecords(cleanupFlags, slot)...)
	a.journalGameItemChangeFinished(actionGameItemsRemove, charIdx, outcome, stage, finished)
}

// debugEnabled reports whether debug-level records are currently admitted. The
// Database Add planner consults it to skip the clone-and-replay overhead (a full
// duplicate mutation) when Debug Mode is off; a nil receiver is a safe false.
func (j *DiagnosticJournal) debugEnabled() bool {
	if j == nil {
		return false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.debug
}

// giSection identifies which fixed-size core record array a Game Items field
// lives in. Combined with a physical row it forms the stable field identity, so
// an empty row becoming populated is visible without an unstable Handle in the
// field name.
type giSection uint8

const (
	giSecInventoryCommon giSection = iota
	giSecInventoryKey
	giSecStorageCommon
	giSecGaItem
	giSecCounter
)

// giKind identifies which scalar of a record (or which trailing counter) a
// field reads. The counter kinds carry the whole field on their own; the
// section is giSecCounter and the row is unused.
type giKind uint8

const (
	giHandle giKind = iota
	giItemID
	giQuantity
	giIndex
	giCounterInvEquip
	giCounterInvAcq
	giCounterStoreEquip
	giCounterStoreAcq
	giGaItemUnk2
	giGaItemUnk3
	giGaItemAoWHandle
	giGaItemUnk5
)

// giAbsent marks a semantically unavailable field: an unresolved inventory
// item_id, or a GaItem field not serialized by that record's type.
const giAbsent = "absent"

// gameItemFieldPlan is one in-scope direct core field a Database Add changed.
// before/planned are captured against the pre-mutation slot and the plan-applied
// clone; section/row/kind re-read the same field from the real post-execution
// slot for the finished phase, so finished reports what actually landed.
type gameItemFieldPlan struct {
	name    string
	before  string
	planned string
	section giSection
	row     int
	kind    giKind
}

func giHex(v uint32) string { return fmt.Sprintf("0x%08X", v) }
func giDec(v uint32) string { return strconv.FormatUint(uint64(v), 10) }

// giResolveItemID maps a live inventory/storage handle to its uppercase item ID,
// or giAbsent when the row has no resolvable live item (empty/invalid handle or
// an unmapped handle that resolves to nothing). Mirrors the handle→id resolution
// the add path and container planner use (GaMap first, then HandleToItemID).
func giResolveItemID(slot *core.SaveSlot, handle uint32) string {
	if handle == core.GaHandleEmpty || handle == core.GaHandleInvalid {
		return giAbsent
	}
	if id, ok := slot.GaMap[handle]; ok && id != 0 {
		return giHex(id)
	}
	if id := db.HandleToItemID(handle); id != 0 {
		return giHex(id)
	}
	return giAbsent
}

// derivedContainerHandles returns the physical inventory handles of every
// container touched only as a side effect of adding a gated item (Fire Pot ->
// Cracked Pot). Those container mutations are container side effects, out of
// scope for Task 2B, so their direct rows are excluded from the lifecycle. A
// container that was itself in plan.batch is a primary Database Add of that
// container and is deliberately kept. The handle is derived exactly as the
// writer (upsertInventoryContainerQuantity) computes it.
func derivedContainerHandles(plan itemAddMutationPlan) map[uint32]bool {
	if len(plan.usedContainers) == 0 {
		return nil
	}
	explicit := make(map[uint32]bool, len(plan.batch))
	for _, it := range plan.batch {
		explicit[it.ItemID] = true
	}
	out := make(map[uint32]bool)
	for cID := range plan.usedContainers {
		if explicit[cID] {
			continue
		}
		out[db.ItemIDToHandlePrefix(cID)|(cID&0x0FFFFFFF)] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// giInvRowHandle returns the raw GaItemHandle at a physical inventory/storage
// row, or GaHandleEmpty when the row is out of range.
func giInvRowHandle(slot *core.SaveSlot, sec giSection, row int) uint32 {
	rows := giInvRows(slot, sec)
	if row < 0 || row >= len(rows) {
		return core.GaHandleEmpty
	}
	return rows[row].GaItemHandle
}

// giInvRows returns the InventoryItem array backing an inventory/storage section.
func giInvRows(slot *core.SaveSlot, sec giSection) []core.InventoryItem {
	switch sec {
	case giSecInventoryCommon:
		return slot.Inventory.CommonItems
	case giSecInventoryKey:
		return slot.Inventory.KeyItems
	case giSecStorageCommon:
		return slot.Storage.CommonItems
	}
	return nil
}

// readGameItemField reads one direct scalar field from slot in the formatting the
// journal contract requires: handles and item IDs as uppercase 0x%08X, quantities
// / row indexes / counters as decimal, item_id as giAbsent when no live item. An
// out-of-range row reads as an empty record so a shrinking array still diffs.
func readGameItemField(slot *core.SaveSlot, sec giSection, row int, kind giKind) string {
	switch sec {
	case giSecCounter:
		switch kind {
		case giCounterInvEquip:
			return giDec(slot.Inventory.NextEquipIndex)
		case giCounterInvAcq:
			return giDec(slot.Inventory.NextAcquisitionSortId)
		case giCounterStoreEquip:
			return giDec(slot.Storage.NextEquipIndex)
		case giCounterStoreAcq:
			return giDec(slot.Storage.NextAcquisitionSortId)
		}
		return ""
	case giSecGaItem:
		if row < 0 || row >= len(slot.GaItems) {
			if kind == giHandle {
				return giHex(core.GaHandleEmpty)
			}
			return giAbsent
		}
		g := slot.GaItems[row]
		switch kind {
		case giHandle:
			return giHex(g.Handle)
		case giItemID:
			if g.IsEmpty() {
				return giAbsent
			}
			return giHex(g.ItemID)
		case giGaItemUnk2:
			if g.IsEmpty() || core.GaItemRecordSize(g.ItemID) < core.GaRecordArmor {
				return giAbsent
			}
			return strconv.FormatInt(int64(g.Unk2), 10)
		case giGaItemUnk3:
			if g.IsEmpty() || core.GaItemRecordSize(g.ItemID) < core.GaRecordArmor {
				return giAbsent
			}
			return strconv.FormatInt(int64(g.Unk3), 10)
		case giGaItemAoWHandle:
			if g.IsEmpty() || core.GaItemRecordSize(g.ItemID) < core.GaRecordWeapon {
				return giAbsent
			}
			return giHex(g.AoWGaItemHandle)
		case giGaItemUnk5:
			if g.IsEmpty() || core.GaItemRecordSize(g.ItemID) < core.GaRecordWeapon {
				return giAbsent
			}
			return giDec(uint32(g.Unk5))
		}
		return ""
	default:
		rows := giInvRows(slot, sec)
		if row < 0 || row >= len(rows) {
			switch kind {
			case giHandle:
				return giHex(core.GaHandleEmpty)
			case giItemID:
				return giAbsent
			default:
				return giDec(0)
			}
		}
		it := rows[row]
		switch kind {
		case giHandle:
			return giHex(it.GaItemHandle)
		case giItemID:
			return giResolveItemID(slot, it.GaItemHandle)
		case giQuantity:
			return giDec(it.Quantity)
		case giIndex:
			return giDec(it.Index)
		}
	}
	return ""
}

// planGameItemsAddRecords diffs every in-scope direct scalar field between the
// pre-mutation slot (before) and the plan-applied clone (planned), returning one
// gameItemFieldPlan per changed field in a stable order: inventory common, key,
// storage common, GaItems, then the four counters. A field is emitted only when
// before != planned. Rows belonging to a derived container (a container raised
// only as a side effect of adding a gated item, never itself in plan.batch) are
// excluded: container mutations are out of scope for Task 2B. The plan is the
// immutable source of that container context.
func planGameItemsAddRecords(before, planned *core.SaveSlot, plan itemAddMutationPlan) []gameItemFieldPlan {
	return planGameItemsDirectRecords(before, planned, derivedContainerHandles(plan))
}

// planGameItemsDirectRecords is the shared low-level direct-record scanner behind
// both Add and Remove. It diffs every direct scalar field between before and
// planned in the same stable order (inventory common, key, storage common,
// GaItems, then the four counters), emitting one gameItemFieldPlan per changed
// field. The optional derived map excludes the inventory rows of derived
// containers (Task 2B's Add-only exclusion); pass nil to scan every changed
// direct record, as Remove does.
func planGameItemsDirectRecords(before, planned *core.SaveSlot, derived map[uint32]bool) []gameItemFieldPlan {
	var plans []gameItemFieldPlan
	add := func(name string, sec giSection, row int, kind giKind) {
		b := readGameItemField(before, sec, row, kind)
		p := readGameItemField(planned, sec, row, kind)
		if b == p {
			return
		}
		plans = append(plans, gameItemFieldPlan{name: name, before: b, planned: p, section: sec, row: row, kind: kind})
	}

	invSections := []struct {
		sec    giSection
		prefix string
	}{
		{giSecInventoryCommon, "inventory_common"},
		{giSecInventoryKey, "inventory_key"},
		{giSecStorageCommon, "storage_common"},
	}
	for _, s := range invSections {
		n := len(giInvRows(before, s.sec))
		if m := len(giInvRows(planned, s.sec)); m > n {
			n = m
		}
		// Containers are written only to the inventory sections; storage rows are
		// never touched by the container sync, so exclusion applies there only.
		checkDerived := derived != nil && (s.sec == giSecInventoryCommon || s.sec == giSecInventoryKey)
		for row := 0; row < n; row++ {
			if checkDerived {
				if derived[giInvRowHandle(before, s.sec, row)] || derived[giInvRowHandle(planned, s.sec, row)] {
					continue
				}
			}
			add(fmt.Sprintf("%s_row_%d_handle", s.prefix, row), s.sec, row, giHandle)
			add(fmt.Sprintf("%s_row_%d_item_id", s.prefix, row), s.sec, row, giItemID)
			add(fmt.Sprintf("%s_row_%d_quantity", s.prefix, row), s.sec, row, giQuantity)
			add(fmt.Sprintf("%s_row_%d_index", s.prefix, row), s.sec, row, giIndex)
		}
	}

	nGa := len(before.GaItems)
	if m := len(planned.GaItems); m > nGa {
		nGa = m
	}
	for row := 0; row < nGa; row++ {
		add(fmt.Sprintf("gaitem_row_%d_handle", row), giSecGaItem, row, giHandle)
		add(fmt.Sprintf("gaitem_row_%d_item_id", row), giSecGaItem, row, giItemID)
		add(fmt.Sprintf("gaitem_row_%d_unk2", row), giSecGaItem, row, giGaItemUnk2)
		add(fmt.Sprintf("gaitem_row_%d_unk3", row), giSecGaItem, row, giGaItemUnk3)
		add(fmt.Sprintf("gaitem_row_%d_aow_gaitem_handle", row), giSecGaItem, row, giGaItemAoWHandle)
		add(fmt.Sprintf("gaitem_row_%d_unk5", row), giSecGaItem, row, giGaItemUnk5)
	}

	add("inventory_next_equip_index", giSecCounter, 0, giCounterInvEquip)
	add("inventory_next_acquisition_sort_id", giSecCounter, 0, giCounterInvAcq)
	add("storage_next_equip_index", giSecCounter, 0, giCounterStoreEquip)
	add("storage_next_acquisition_sort_id", giSecCounter, 0, giCounterStoreAcq)

	return plans
}

// gameItemPlannedRecords maps plans to before/planned change records. The before
// phase ignores After; the planned phase uses it — both share this list.
func gameItemPlannedRecords(plans []gameItemFieldPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.name, Before: p.before, After: p.planned}
	}
	return out
}

// gameItemFinishedRecords maps plans to finished records, re-reading each field's
// actual value out of the post-execution slot so After reflects what really
// landed rather than the planned value.
func gameItemFinishedRecords(plans []gameItemFieldPlan, post *core.SaveSlot) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.name, Before: p.before, After: readGameItemField(post, p.section, p.row, p.kind)}
	}
	return out
}

// gameItemSideEffectPlan is one Game Items side effect whose target differs from
// the current value: a container quantity, a container pickup/vendor flag, or a
// direct item-driven Event Flag (AoW / world pickup / companion). before/planned
// are captured against the pre-mutation real slot and the plan-applied clone;
// read pulls the field's live value back out of the post-execution real slot for
// the finished phase. Game Items owns this type so the container/flag planners
// never depend on a Character-owned plan type — only the shared semantic readers
// (readContainerQuantity / readContainerFlag) are reused.
type gameItemSideEffectPlan struct {
	field   string
	before  string
	planned string
	read    func(*core.SaveSlot) string
}

// gameItemSideEffectPlannedRecords maps side-effect plans to before/planned
// change records (the before phase ignores After, the planned phase uses it),
// mirroring gameItemPlannedRecords for the direct physical fields.
func gameItemSideEffectPlannedRecords(plans []gameItemSideEffectPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return out
}

// gameItemSideEffectFinishedRecords maps side-effect plans to finished records,
// reading each field's actual value back out of the post-execution real slot so
// After reflects what really landed (or, on rollback, the restored value).
func gameItemSideEffectFinishedRecords(plans []gameItemSideEffectPlan, slot *core.SaveSlot) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.read(slot)}
	}
	return out
}

// planGameItemsAddContainerRecords derives the container side effects of a
// Database Add — the semantic container quantity plus each pickup/vendor event
// flag the writer synchronizes — by comparing the pre-mutation real slot
// (before) against the plan-applied clone (planned). It reuses the Character
// diagnostics' semantic readers (readContainerQuantity / readContainerFlag) so
// the field naming and formatting match, and never touches or replays the real
// slot. Containers are iterated in ascending numeric order so Go map iteration
// order never leaks into the log.
//
// A container that the user explicitly requested (present in plan.batch) emits
// no container_*_quantity: Task 2B already logs its direct physical row as the
// primary add, so re-logging the same quantity here would duplicate the change.
// Its pickup/vendor flags are still genuine synchronization side effects, so
// they are emitted. A derived container (used only because a gated item pulled
// it in) emits both quantity and flags. Every field self-excludes when its
// before value already equals the planned value, and flag records disappear
// entirely when no Event Flags region exists (the clone leaves them unset).
func planGameItemsAddContainerRecords(before, planned *core.SaveSlot, plan itemAddMutationPlan) []gameItemSideEffectPlan {
	if len(plan.usedContainers) == 0 {
		return nil
	}

	explicit := make(map[uint32]bool, len(plan.batch))
	for _, it := range plan.batch {
		explicit[it.ItemID] = true
	}

	ids := make([]uint32, 0, len(plan.usedContainers))
	for cID := range plan.usedContainers {
		ids = append(ids, cID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var plans []gameItemSideEffectPlan
	for _, cID := range ids {
		itemID := cID
		if !explicit[itemID] {
			if b, p := readContainerQuantity(before, itemID), readContainerQuantity(planned, itemID); b != p {
				plans = append(plans, gameItemSideEffectPlan{
					field:   fmt.Sprintf("container_0x%08X_quantity", itemID),
					before:  b,
					planned: p,
					read:    func(s *core.SaveSlot) string { return readContainerQuantity(s, itemID) },
				})
			}
		}
		for _, f := range data.ContainerPickupFlags[itemID] {
			plans = appendGameItemContainerFlagPlan(plans, before, planned, itemID, f, "pickup")
		}
		for _, f := range data.ContainerVendorPurchaseFlags[itemID] {
			plans = appendGameItemContainerFlagPlan(plans, before, planned, itemID, f, "vendor")
		}
	}
	return plans
}

// planGameItemsAddFlagRecords derives the direct item-driven Event Flag side
// effects of a Database Add — the Ash of War duplication flag, the world-pickup
// flag, and each companion flag — by comparing the pre-mutation real slot
// (before) against the plan-applied clone (planned). It walks plan.items in the
// writer's order and, per item, inspects exactly the three direct mappings the
// executor's POST-FLAGS block sets: AoWItemToFlagID, WorldPickupFlagID and
// CompanionEventFlagsForItem, in that order and in each mapping's natural order.
// Bolstering pickups, tutorial IDs and container flags are out of scope and are
// left to their own planners.
//
// A field is emitted only when the clone actually flipped the flag (before !=
// planned): a flag already set self-excludes, and no records appear at all when
// the slot has no Event Flags region (the clone leaves every flag unset). The
// real slot is never replayed or mutated for diagnostics; read pulls each flag
// back out of the post-execution real slot for the finished phase. Repeated
// submitted IDs that resolve to the same <item, source kind, flag> tuple are
// deduplicated by field name so one flag never yields two lifecycle entries. It
// reuses only the shared container flag reader for value semantics; the plan
// vehicle and mappers are Game Items-owned.
func planGameItemsAddFlagRecords(before, planned *core.SaveSlot, plan itemAddMutationPlan) []gameItemSideEffectPlan {
	var plans []gameItemSideEffectPlan
	seen := make(map[string]bool)
	add := func(itemID, flagID uint32, kind string) {
		field := fmt.Sprintf("item_0x%08X_%s_flag_%d", itemID, kind, flagID)
		if seen[field] {
			return
		}
		b := readContainerFlag(before, flagID)
		p := readContainerFlag(planned, flagID)
		if b == p {
			return
		}
		seen[field] = true
		plans = append(plans, gameItemSideEffectPlan{
			field:   field,
			before:  b,
			planned: p,
			read:    func(s *core.SaveSlot) string { return readContainerFlag(s, flagID) },
		})
	}
	for _, it := range plan.items {
		if flagID, ok := data.AoWItemToFlagID[it.baseID]; ok {
			add(it.baseID, flagID, "aow")
		}
		if flagID, ok := data.WorldPickupFlagID[it.baseID]; ok {
			add(it.baseID, flagID, "world_pickup")
		}
		for _, f := range data.CompanionEventFlagsForItem(it.baseID) {
			add(it.baseID, f, "companion")
		}
	}
	return plans
}

// planGameItemsAddBolsteringRecords derives the bolstering pickup flag side
// effects of a Database Add — the world-pickup flags the writer marks so a
// bolstering material added through the editor no longer respawns in the world —
// by comparing the pre-mutation real slot (before) against the plan-applied
// clone (planned). Per plan.items entry that carries a data.BolsteringPickupFlags
// list, it walks a sorted-ascending copy of that list (mirroring the writer,
// which sorts before consuming its `set` counter) and emits one lifecycle per
// flag the clone actually flipped (before != planned), in ascending flag-ID
// order.
//
// The clone comparison — not the requested inv/storage quantity — is the sole
// authority on which flags the writer's counter reached: an already-set leading
// flag self-excludes and never consumes the count, so the planner cannot drift
// from the writer by predicting flags from quantity. Records vanish entirely
// when the slot has no Event Flags region (the clone leaves every flag unset). A
// field is deduplicated by name, so a submitted ID repeated within one request
// still yields at most one lifecycle per physical flag. The real slot is never
// replayed or mutated; read pulls each flag back out of the post-execution real
// slot for the finished phase. Only the shared container flag reader is reused;
// the plan vehicle and mapping are Game Items-owned.
func planGameItemsAddBolsteringRecords(before, planned *core.SaveSlot, plan itemAddMutationPlan) []gameItemSideEffectPlan {
	var plans []gameItemSideEffectPlan
	seen := make(map[string]bool)
	for _, it := range plan.items {
		flagList, ok := data.BolsteringPickupFlags[it.baseID]
		if !ok {
			continue
		}
		sorted := make([]uint32, len(flagList))
		copy(sorted, flagList)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		for _, flagID := range sorted {
			field := fmt.Sprintf("item_0x%08X_bolstering_pickup_flag_%d", it.baseID, flagID)
			if seen[field] {
				continue
			}
			b := readContainerFlag(before, flagID)
			p := readContainerFlag(planned, flagID)
			if b == p {
				continue
			}
			seen[field] = true
			fID := flagID
			plans = append(plans, gameItemSideEffectPlan{
				field:   field,
				before:  b,
				planned: p,
				read:    func(s *core.SaveSlot) string { return readContainerFlag(s, fID) },
			})
		}
	}
	return plans
}

// planGameItemsRemoveCleanupFlagRecords records only the Event Flags changed by
// RemoveItemsFromCharacter after its physical removals succeed: bolstering
// pickup flags restored for removed materials and companion flags cleared after
// the last matching GaItem disappears. The clone has already run the same shared
// cleanup as the real slot, so before-vs-clone is the sole authority; no flag is
// predicted from requested quantities or handles.
//
// Diagnostic output is deterministic even though the writer intentionally keeps
// its historical map iteration: bolstering item IDs then companion item IDs are
// ascending, as are flag IDs within each item. A field self-excludes unless the
// clone actually changed it, and no Event Flags region naturally produces no
// records because both reads are false.
func planGameItemsRemoveCleanupFlagRecords(before, planned *core.SaveSlot, bolsteringRemovals map[uint32]int, companionRemovals map[uint32]bool) []gameItemSideEffectPlan {
	var plans []gameItemSideEffectPlan
	seen := make(map[string]bool)
	appendFlag := func(itemID, flagID uint32, kind string) {
		field := fmt.Sprintf("item_0x%08X_%s_flag_%d", itemID, kind, flagID)
		if seen[field] {
			return
		}
		beforeValue := readContainerFlag(before, flagID)
		plannedValue := readContainerFlag(planned, flagID)
		if beforeValue == plannedValue {
			return
		}
		seen[field] = true
		readFlagID := flagID
		plans = append(plans, gameItemSideEffectPlan{
			field:   field,
			before:  beforeValue,
			planned: plannedValue,
			read:    func(s *core.SaveSlot) string { return readContainerFlag(s, readFlagID) },
		})
	}

	bolsteringIDs := make([]uint32, 0, len(bolsteringRemovals))
	for itemID := range bolsteringRemovals {
		bolsteringIDs = append(bolsteringIDs, itemID)
	}
	sort.Slice(bolsteringIDs, func(i, j int) bool { return bolsteringIDs[i] < bolsteringIDs[j] })
	for _, itemID := range bolsteringIDs {
		flagIDs := append([]uint32(nil), data.BolsteringPickupFlags[itemID]...)
		sort.Slice(flagIDs, func(i, j int) bool { return flagIDs[i] < flagIDs[j] })
		for _, flagID := range flagIDs {
			appendFlag(itemID, flagID, "bolstering_pickup")
		}
	}

	companionIDs := make([]uint32, 0, len(companionRemovals))
	for itemID := range companionRemovals {
		companionIDs = append(companionIDs, itemID)
	}
	sort.Slice(companionIDs, func(i, j int) bool { return companionIDs[i] < companionIDs[j] })
	for _, itemID := range companionIDs {
		flagIDs := append([]uint32(nil), data.CompanionEventFlagsForItem(itemID)...)
		sort.Slice(flagIDs, func(i, j int) bool { return flagIDs[i] < flagIDs[j] })
		for _, flagID := range flagIDs {
			appendFlag(itemID, flagID, "companion")
		}
	}

	return plans
}

// readTutorialMembership reports whether tutorialID is registered in the slot's
// TutorialData block as the scalar "true"/"false", reading live through the
// exported core API (core.HasTutorialID). TutorialData is not an Event Flags
// region: an unavailable/invalid block surfaces a parser error from core, which
// is treated as "false" here so no parser error text or raw TutorialData bytes
// ever leak into a journal value.
func readTutorialMembership(slot *core.SaveSlot, tutorialID uint32) string {
	if has, err := core.HasTutorialID(slot, tutorialID); err == nil && has {
		return "true"
	}
	return "false"
}

// planGameItemsAddTutorialRecords derives the TutorialData side effects of a
// Database Add — the tutorial IDs the writer appends via core.AppendTutorialID so
// a tutorial-bound "About *" item no longer drops on the ground — by comparing
// the pre-mutation real slot (before) against the plan-applied clone (planned).
// Per plan.items entry mapped in data.AboutTutorialID, it emits one lifecycle
// only when the clone shows an added membership (before != planned): an
// already-registered ID self-excludes. This task logs the semantic TutorialData
// membership exposed by the exported core read API, not raw TutorialData bytes:
// when the block is unreadable/malformed, core.ReadTutorialIDs rejects it in both
// the before slot and the clone, so there is no observable membership transition
// and no tutorial_id lifecycle is emitted — regardless of whatever raw bytes
// core.AppendTutorialID may have written to the malformed header. Deduplicated by
// physical tutorial ID, so a repeated mapping or submitted ID yields at most one
// lifecycle. The real slot is never replayed or mutated; read pulls membership
// back out of the post-execution real slot for the finished phase. Only the
// exported core read API is reused; the plan vehicle and mapping are Game
// Items-owned.
func planGameItemsAddTutorialRecords(before, planned *core.SaveSlot, plan itemAddMutationPlan) []gameItemSideEffectPlan {
	var plans []gameItemSideEffectPlan
	seen := make(map[string]bool)
	for _, it := range plan.items {
		tutorialID, ok := data.AboutTutorialID[it.baseID]
		if !ok {
			continue
		}
		field := fmt.Sprintf("tutorial_id_%d", tutorialID)
		if seen[field] {
			continue
		}
		b := readTutorialMembership(before, tutorialID)
		p := readTutorialMembership(planned, tutorialID)
		if b == p {
			continue
		}
		seen[field] = true
		tID := tutorialID
		plans = append(plans, gameItemSideEffectPlan{
			field:   field,
			before:  b,
			planned: p,
			read:    func(s *core.SaveSlot) string { return readTutorialMembership(s, tID) },
		})
	}
	return plans
}

// giStorageHeaderUnavailable is the stable storage_header_count value returned
// when the slot has no valid 4-byte storage-header region at StorageBoxOffset.
// It is a non-numeric sentinel so it never collides with a real decimal count,
// which also means an unavailable header self-excludes: when both the real slot
// and the clone read unavailable, before == planned and no lifecycle is emitted.
const giStorageHeaderUnavailable = "unavailable"

// readStorageHeaderCount reads the physical 4-byte storage-header count scalar
// directly from slot.Data at slot.StorageBoxOffset as an unsigned decimal
// integer. It is bounds-safe: a slot with no valid storage-header region reads
// as the unavailable sentinel, and no raw bytes ever leak into the value.
func readStorageHeaderCount(slot *core.SaveSlot) string {
	if slot.StorageBoxOffset <= 0 || slot.StorageBoxOffset+4 > len(slot.Data) {
		return giStorageHeaderUnavailable
	}
	return giDec(binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:]))
}

// planGameItemsAddStorageHeaderRecords derives the storage-header side effect of
// a Database Add — the physical 4-byte count scalar at StorageBoxOffset that the
// executor rewrites via core.ReconcileStorageHeader after container
// synchronization — by comparing the pre-mutation real slot (before) against the
// plan-applied clone (planned). It emits one lifecycle only when the scalar
// actually changed (before != planned): a canonical header already equal to its
// physical row count self-excludes, and an unavailable header (no valid region
// in either slot) reads unavailable in both and self-excludes too. The real slot
// is never replayed or mutated; read pulls the scalar back out of the
// post-execution real slot for the finished phase. Only the shared
// gameItemSideEffectPlan vehicle is reused; the reader is Game Items-owned.
func planGameItemsAddStorageHeaderRecords(before, planned *core.SaveSlot) []gameItemSideEffectPlan {
	b := readStorageHeaderCount(before)
	p := readStorageHeaderCount(planned)
	if b == p {
		return nil
	}
	return []gameItemSideEffectPlan{{
		field:   "storage_header_count",
		before:  b,
		planned: p,
		read:    readStorageHeaderCount,
	}}
}

// giInventoryHeaderUnavailable is the stable inventory_common_header_count value
// returned when the slot has no valid 4-byte held-inventory CommonItems count
// region at MagicOffset + InvStartFromMagic - 4. A non-numeric sentinel so it
// never collides with a real decimal count and self-excludes: when both the real
// slot and the clone read unavailable, before == planned and no lifecycle emits.
const giInventoryHeaderUnavailable = "unavailable"

// readInventoryCommonHeaderCount reads the physical 4-byte held-inventory
// CommonItems count scalar ("common_item_count") directly from slot.Data at
// slot.MagicOffset + core.InvStartFromMagic - 4 as an unsigned decimal integer.
// This is the header the Database Add writer (core addToInventory) increments in
// place by one each time it allocates a NEW Inventory.CommonItems record — a
// distinct scalar from the storage header and, unlike it, never reconciled on the
// add path. It is bounds-safe: a slot with no valid magic region (MagicOffset<=0)
// or whose header location falls outside slot.Data reads as the unavailable
// sentinel, mirroring core.ReconcileInventoryHeader's own guard, and no raw bytes
// ever leak into the value.
func readInventoryCommonHeaderCount(slot *core.SaveSlot) string {
	if slot.MagicOffset <= 0 {
		return giInventoryHeaderUnavailable
	}
	off := slot.MagicOffset + core.InvStartFromMagic - 4
	if off < 0 || off+4 > len(slot.Data) {
		return giInventoryHeaderUnavailable
	}
	return giDec(binary.LittleEndian.Uint32(slot.Data[off:]))
}

// planGameItemsAddInventoryHeaderRecords derives the held-inventory CommonItems
// count-header side effect of a Database Add — the physical "common_item_count"
// scalar at MagicOffset + InvStartFromMagic - 4 that the writer (core
// addToInventory) increments by one whenever it allocates a NEW
// Inventory.CommonItems record — by comparing the pre-mutation real slot (before)
// against the plan-applied clone (planned). Unlike the storage header, this
// scalar is NOT reconciled on the add path: the writer bumps it in place, so the
// lifecycle reports the raw incremented value (e.g. a stale 42 becomes 43), never
// a recomputed count. It emits one lifecycle only when the scalar actually changed
// (before != planned): an in-place stack update of an existing record allocates no
// new record and self-excludes, and an unavailable header (no valid region in
// either slot) reads unavailable in both and self-excludes too. The real slot is
// never replayed or mutated; read pulls the scalar back out of the post-execution
// real slot for the finished phase. Only the shared gameItemSideEffectPlan vehicle
// is reused; the reader is Game Items-owned.
func planGameItemsAddInventoryHeaderRecords(before, planned *core.SaveSlot) []gameItemSideEffectPlan {
	b := readInventoryCommonHeaderCount(before)
	p := readInventoryCommonHeaderCount(planned)
	if b == p {
		return nil
	}
	return []gameItemSideEffectPlan{{
		field:   "inventory_common_header_count",
		before:  b,
		planned: p,
		read:    readInventoryCommonHeaderCount,
	}}
}

// appendGameItemContainerFlagPlan appends a container flag plan only when the
// clone flipped the flag (before != planned). Unlike the Character helper it
// reads the planned value straight from the applied clone rather than assuming
// "true", so a flag the writer never reaches (beyond finalQty) or one that was
// already set self-excludes.
func appendGameItemContainerFlagPlan(plans []gameItemSideEffectPlan, before, planned *core.SaveSlot, itemID, flagID uint32, kind string) []gameItemSideEffectPlan {
	b := readContainerFlag(before, flagID)
	p := readContainerFlag(planned, flagID)
	if b == p {
		return plans
	}
	return append(plans, gameItemSideEffectPlan{
		field:   fmt.Sprintf("container_0x%08X_%s_flag_%d", itemID, kind, flagID),
		before:  b,
		planned: p,
		read:    func(s *core.SaveSlot) string { return readContainerFlag(s, flagID) },
	})
}

// gameItemMutationPlans is the common lifecycle projection for the Database
// tab's unlock operations. Those operations change a small declared set of
// Event Flags and may add/remove normal inventory rows; the direct scanner and
// header readers retain the same physical representation used by Database Add
// and Remove.
type gameItemMutationPlans struct {
	direct        []gameItemFieldPlan
	invHeader     []gameItemSideEffectPlan
	storageHeader []gameItemSideEffectPlan
	flags         []gameItemSideEffectPlan
	// gaItemState holds the GaItem-specific semantic records (GaMap value changes
	// then GaItem allocation cursors) that a GaItem structural mutation (dedup,
	// and later repack) logs after its direct physical rows. It stays nil for the
	// unlock/save callers, so their lifecycle output is byte-for-byte unchanged.
	gaItemState []gameItemSideEffectPlan
}

func planGameItemsMutation(before, planned *core.SaveSlot, flagIDs []uint32) gameItemMutationPlans {
	return gameItemMutationPlans{
		direct:        planGameItemsDirectRecords(before, planned, nil),
		invHeader:     planGameItemsAddInventoryHeaderRecords(before, planned),
		storageHeader: planGameItemsAddStorageHeaderRecords(before, planned),
		flags:         planGameItemsEventFlagRecords(before, planned, flagIDs),
	}
}

// planGameItemsGaItemStructuralMutation projects the lifecycle shared by GaItem
// structural mutations: direct serialized records first, then GaMap and cursor
// state. The caller supplies the already-validated candidate it is about to
// publish, so this projection never needs a second writer replay.
func planGameItemsGaItemStructuralMutation(before, planned *core.SaveSlot) gameItemMutationPlans {
	return gameItemMutationPlans{
		direct:      planGameItemsDirectRecords(before, planned, nil),
		gaItemState: planGameItemsGaItemState(before, planned),
	}
}

// planGameItemsGaItemState is the extensible GaItem-specific semantic projection
// shared by GaItem structural mutations (dedup now, repack in Task 8B): the
// GaMap value changes (ascending handle) followed by the GaItem allocation
// cursor changes. It is empty when neither actually changed, so adding it to an
// otherwise-empty plan emits nothing.
func planGameItemsGaItemState(before, planned *core.SaveSlot) []gameItemSideEffectPlan {
	return append(planGameItemsGaMapRecords(before, planned), planGameItemsGaItemCursorRecords(before, planned)...)
}

// gaMapEntryValue reads one GaMap handle semantically: a present mapping is the
// mapped item_id as uppercase 0x%08X, a missing handle is giAbsent. This keeps a
// removed entry (0x%08X -> absent) and an added entry (absent -> 0x%08X) distinct
// from a real 0x00000000 mapping, which giHex(s.GaMap[handle]) could not.
func gaMapEntryValue(slot *core.SaveSlot, handle uint32) string {
	if id, ok := slot.GaMap[handle]; ok {
		return giHex(id)
	}
	return giAbsent
}

// planGameItemsGaMapRecords logs one lifecycle per GaMap handle whose mapping
// actually changed (before != planned), iterated by ascending handle so Go map
// order never leaks into the journal. It walks the union of before/planned keys
// so a removed entry (0x%08X -> absent) and an added entry (absent -> 0x%08X) are
// logged as well as a changed value. read pulls the mapping back out of the
// post-execution real slot for the finished phase through the same reader.
func planGameItemsGaMapRecords(before, planned *core.SaveSlot) []gameItemSideEffectPlan {
	seen := make(map[uint32]bool, len(before.GaMap)+len(planned.GaMap))
	handles := make([]uint32, 0, len(before.GaMap)+len(planned.GaMap))
	for h := range before.GaMap {
		if !seen[h] {
			seen[h] = true
			handles = append(handles, h)
		}
	}
	for h := range planned.GaMap {
		if !seen[h] {
			seen[h] = true
			handles = append(handles, h)
		}
	}
	sort.Slice(handles, func(i, j int) bool { return handles[i] < handles[j] })

	var plans []gameItemSideEffectPlan
	for _, h := range handles {
		b := gaMapEntryValue(before, h)
		p := gaMapEntryValue(planned, h)
		if b == p {
			continue
		}
		handle := h
		plans = append(plans, gameItemSideEffectPlan{
			field:   fmt.Sprintf("gaitem_map_handle_0x%08X_item_id", handle),
			before:  b,
			planned: p,
			read:    func(s *core.SaveSlot) string { return gaMapEntryValue(s, handle) },
		})
	}
	return plans
}

// planGameItemsGaItemCursorRecords logs one lifecycle per GaItem allocation
// cursor whose value actually changed. Indexes are decimal, handles uppercase
// 0x%08X; an unchanged cursor self-excludes. read pulls each cursor back out of
// the post-execution real slot for the finished phase.
func planGameItemsGaItemCursorRecords(before, planned *core.SaveSlot) []gameItemSideEffectPlan {
	cursors := []struct {
		field string
		read  func(*core.SaveSlot) string
	}{
		{"gaitem_next_aow_index", func(s *core.SaveSlot) string { return strconv.Itoa(s.NextAoWIndex) }},
		{"gaitem_next_armament_index", func(s *core.SaveSlot) string { return strconv.Itoa(s.NextArmamentIndex) }},
		{"gaitem_next_handle", func(s *core.SaveSlot) string { return giHex(s.NextGaItemHandle) }},
		{"gaitem_part_handle", func(s *core.SaveSlot) string { return giHex(uint32(s.PartGaItemHandle)) }},
	}
	var plans []gameItemSideEffectPlan
	for _, c := range cursors {
		b := c.read(before)
		p := c.read(planned)
		if b == p {
			continue
		}
		plans = append(plans, gameItemSideEffectPlan{field: c.field, before: b, planned: p, read: c.read})
	}
	return plans
}

// planGameItemsEventFlagRecords logs only the flags an operation owns. IDs are
// sorted and deduplicated so a map-backed input can never make the journal
// order nondeterministic. A missing Event Flags region reads false on both
// slots, therefore naturally self-excludes.
func planGameItemsEventFlagRecords(before, planned *core.SaveSlot, flagIDs []uint32) []gameItemSideEffectPlan {
	ids := append([]uint32(nil), flagIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var plans []gameItemSideEffectPlan
	var previous uint32
	for i, flagID := range ids {
		if i > 0 && flagID == previous {
			continue
		}
		previous = flagID
		beforeValue := readContainerFlag(before, flagID)
		plannedValue := readContainerFlag(planned, flagID)
		if beforeValue == plannedValue {
			continue
		}
		readFlagID := flagID
		plans = append(plans, gameItemSideEffectPlan{
			field:   fmt.Sprintf("event_flag_%d", flagID),
			before:  beforeValue,
			planned: plannedValue,
			read:    func(s *core.SaveSlot) string { return readContainerFlag(s, readFlagID) },
		})
	}
	return plans
}

func (p gameItemMutationPlans) records() []characterFieldChange {
	records := append(gameItemPlannedRecords(p.direct), gameItemSideEffectPlannedRecords(p.invHeader)...)
	records = append(records, gameItemSideEffectPlannedRecords(p.storageHeader)...)
	records = append(records, gameItemSideEffectPlannedRecords(p.flags)...)
	return append(records, gameItemSideEffectPlannedRecords(p.gaItemState)...)
}

func (p gameItemMutationPlans) finished(slot *core.SaveSlot) []characterFieldChange {
	records := append(gameItemFinishedRecords(p.direct, slot), gameItemSideEffectFinishedRecords(p.invHeader, slot)...)
	records = append(records, gameItemSideEffectFinishedRecords(p.storageHeader, slot)...)
	records = append(records, gameItemSideEffectFinishedRecords(p.flags, slot)...)
	return append(records, gameItemSideEffectFinishedRecords(p.gaItemState, slot)...)
}

func (a *App) journalGameItemsMutationBefore(action string, charIdx int, plans gameItemMutationPlans) {
	records := plans.records()
	a.journalGameItemChangeBefore(action, charIdx, records)
	a.journalGameItemChangePlanned(action, charIdx, records)
}

func (a *App) journalGameItemsMutationFinished(action string, charIdx int, outcome characterChangeOutcome, stage string, plans gameItemMutationPlans, slot *core.SaveSlot) {
	a.journalGameItemChangeFinished(action, charIdx, outcome, stage, plans.finished(slot))
}

const workspaceAbsent = "absent"

// workspaceItemPlan is a lifecycle field for an in-memory Inventory Workspace
// item. It is deliberately separate from gameItemFieldPlan: until the final
// Save, workspace edits do not have a physical save-slot row to re-read.
type workspaceItemPlan struct {
	field   string
	before  string
	planned string
	uid     string
	kind    string
}

func cloneWorkspaceSnapshot(in editor.InventoryWorkspaceSnapshot) editor.InventoryWorkspaceSnapshot {
	out := in
	out.InventoryItems = append([]editor.EditableItem(nil), in.InventoryItems...)
	out.StorageItems = append([]editor.EditableItem(nil), in.StorageItems...)
	return out
}

func workspaceItemsByUID(snapshot editor.InventoryWorkspaceSnapshot) map[string]editor.EditableItem {
	out := make(map[string]editor.EditableItem, len(snapshot.InventoryItems)+len(snapshot.StorageItems))
	for _, item := range snapshot.InventoryItems {
		out[item.UID] = item
	}
	for _, item := range snapshot.StorageItems {
		out[item.UID] = item
	}
	return out
}

func workspaceItemValue(item editor.EditableItem, present bool, kind string) string {
	if !present {
		return workspaceAbsent
	}
	switch kind {
	case "source":
		return string(item.Source)
	case "container":
		return string(item.Container)
	case "position":
		return strconv.Itoa(item.Position)
	case "item_id":
		return giHex(item.ItemID)
	case "base_item_id":
		return giHex(item.BaseItemID)
	case "quantity":
		return giDec(item.Quantity)
	case "acquisition_index":
		return giDec(item.AcquisitionIndex)
	case "current_upgrade":
		return strconv.Itoa(item.CurrentUpgrade)
	case "infusion_name":
		return item.InfusionName
	case "pending_aow_item_id":
		return giHex(item.PendingAoWItemID)
	case "pending_aow_name":
		return item.PendingAoWName
	case "pending_aow_clear":
		return strconv.FormatBool(item.PendingAoWClear)
	case "has_pending_weapon_patch":
		return strconv.FormatBool(item.HasPendingWeaponPatch)
	}
	return ""
}

var workspaceMutableKinds = []string{
	"source", "container", "position", "item_id", "base_item_id", "quantity",
	"acquisition_index", "current_upgrade", "infusion_name", "pending_aow_item_id",
	"pending_aow_name", "pending_aow_clear", "has_pending_weapon_patch",
}

// planWorkspaceItemChanges produces a stable semantic diff for RAM-only
// workspace mutations. Names and UI metadata are intentionally omitted: they
// are DB-derived display data, not mutable workspace state. UID is a local
// record identity, never an account identifier, and lets additions/removals be
// represented without pretending an unsaved item already has a save handle.
func planWorkspaceItemChanges(before, planned editor.InventoryWorkspaceSnapshot) []workspaceItemPlan {
	beforeItems := workspaceItemsByUID(before)
	plannedItems := workspaceItemsByUID(planned)
	uids := make([]string, 0, len(beforeItems)+len(plannedItems))
	seen := make(map[string]bool, len(beforeItems)+len(plannedItems))
	for uid := range beforeItems {
		seen[uid] = true
		uids = append(uids, uid)
	}
	for uid := range plannedItems {
		if !seen[uid] {
			uids = append(uids, uid)
		}
	}
	sort.Strings(uids)

	var plans []workspaceItemPlan
	for _, uid := range uids {
		beforeItem, beforeOK := beforeItems[uid]
		plannedItem, plannedOK := plannedItems[uid]
		for _, kind := range workspaceMutableKinds {
			beforeValue := workspaceItemValue(beforeItem, beforeOK, kind)
			plannedValue := workspaceItemValue(plannedItem, plannedOK, kind)
			if beforeValue == plannedValue {
				continue
			}
			plans = append(plans, workspaceItemPlan{
				field:   "workspace_item_" + uid + "_" + kind,
				before:  beforeValue,
				planned: plannedValue,
				uid:     uid,
				kind:    kind,
			})
		}
	}
	return plans
}

func workspacePlannedRecords(plans []workspaceItemPlan) []characterFieldChange {
	records := make([]characterFieldChange, len(plans))
	for i, plan := range plans {
		records[i] = characterFieldChange{Field: plan.field, Before: plan.before, After: plan.planned}
	}
	return records
}

func workspaceFinishedRecords(plans []workspaceItemPlan, snapshot editor.InventoryWorkspaceSnapshot) []characterFieldChange {
	items := workspaceItemsByUID(snapshot)
	records := make([]characterFieldChange, len(plans))
	for i, plan := range plans {
		item, ok := items[plan.uid]
		records[i] = characterFieldChange{Field: plan.field, Before: plan.before, After: workspaceItemValue(item, ok, plan.kind)}
	}
	return records
}

func (a *App) journalWorkspaceBefore(action string, charIdx int, plans []workspaceItemPlan) {
	records := workspacePlannedRecords(plans)
	a.journalGameItemChangeBefore(action, charIdx, records)
	a.journalGameItemChangePlanned(action, charIdx, records)
}

func (a *App) journalWorkspaceFinished(action string, charIdx int, outcome characterChangeOutcome, stage string, plans []workspaceItemPlan, snapshot editor.InventoryWorkspaceSnapshot) {
	a.journalGameItemChangeFinished(action, charIdx, outcome, stage, workspaceFinishedRecords(plans, snapshot))
}
