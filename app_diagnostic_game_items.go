package main

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// actionGameItemsAdd is the internal action tag for a Database Add mutation
// (AddItemsToCharacter / AddItemsToCharacterWithGameLimits) in the Game Items
// lifecycle journal.
const actionGameItemsAdd = "add_items"

// stageGameItemsApplyAddPlan is the finished-phase stage reported when the real
// executor (applyItemAddMutationPlan) fails and the slot was rolled back.
const stageGameItemsApplyAddPlan = "apply_add_plan"

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
)

// giAbsent marks an item_id whose row holds no resolvable live item.
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
		if kind == giHandle {
			return giHex(g.Handle)
		}
		if g.IsEmpty() {
			return giAbsent
		}
		return giHex(g.ItemID)
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
	var plans []gameItemFieldPlan
	add := func(name string, sec giSection, row int, kind giKind) {
		b := readGameItemField(before, sec, row, kind)
		p := readGameItemField(planned, sec, row, kind)
		if b == p {
			return
		}
		plans = append(plans, gameItemFieldPlan{name: name, before: b, planned: p, section: sec, row: row, kind: kind})
	}

	derived := derivedContainerHandles(plan)

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
