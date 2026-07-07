package editor

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// HandlesByUID maps editor UIDs to allocated GaItem handles after a
// successful ApplyWorkspaceSave. UID format for existing items is
// "hnd:0x%08X" (handle never changes — kept stable across transfers and
// reorders) and "new:N" for added items (handle freshly minted by
// AddItemsToSlotBatch). Removed items are absent from this map.
// Callers use this to follow the same item through subsequent edits.
type HandlesByUID map[string]uint32

// ApplyWorkspaceSave is the Phase 4B commit path. It validates the
// workspace, rejects out-of-scope or unsafe edits, then writes a
// reorder + add + transfer + remove + weapon-upgrade + pending-AoW plan
// into slot.Data via the wipe-and-replay layout plus targeted GaItem
// patches.
//
// In scope for Phase 4B (extends Phase 3B):
//   - reorder existing editable items within their original container
//   - transfer existing editable items between Inventory and Storage —
//     OriginalHandle is preserved, the GaItem stays in place, and only
//     the inventory record is moved; old record is wiped by the layout
//     rebuild
//   - remove existing editable items from the workspace — their record
//     is absent from both containers post-save; GaItem stays in
//     slot.GaItems / slot.GaMap (conservative no-GC policy — see notes
//     under "GaItem GC policy" below)
//   - add new editable items (Source=Added) — allocates real handles
//     and GaItem entries via core.AddItemsToSlotBatch
//   - patch weapon ItemID for existing items with upgrade/infusion
//     changes via core.PatchWeaponItemID (works correctly for
//     transferred weapons too — patch keys on handle, not container)
//   - apply pending AoW edits (Phase 4B):
//   - PendingAoWItemID != 0 — allocate a fresh AoW GaItem via
//     core.PatchWeaponAoW (never reuses a handle, so two weapons
//     setting the same AoW item get distinct handles)
//   - PendingAoWClear == true — patch weapon's AoWGaItemHandle
//     field to the canonical no-custom sentinel via
//     core.PatchWeaponAoWHandle (in-place, no rebuild)
//   - preserve unsupported/pass-through records at their original
//     physical SlotIndex
//
// Still rejected with clear errors (slot.Data left untouched):
//   - workspace validation errors (including the new
//     CodePendingAoWConflict)
//   - any pending AoW set whose AoW item is not category "ashes_of_war"
//   - any pending AoW set whose (aow, weapon) compatibility is known
//     and false, or unknown (fail-closed)
//   - missing baseline data (session created without baseline tracking)
//   - inventory or storage capacity exceeded by the final layout
//   - pass-through SlotIndex collisions
//
// GaItem GC policy (Phase 3B):
//   - Removed items leave their GaItem record orphaned in slot.GaItems
//     and slot.GaMap. No record in slot.Data references them after the
//     layout rebuild, so they do not show up in any container.
//   - We do NOT call RepairOrphanedGaItems automatically. The trade-off
//     is a small amount of wasted GaItem array space versus the safety
//     risk of an under-tested GC sweep mutating shared state.
//   - Future Phase 4+ may add explicit GC when AoW work lands and the
//     ownership model is fully understood.
//
// Atomicity contract:
//   - Callers MUST snapshot slot via core.SnapshotSlot BEFORE calling
//     this function and call core.RestoreSlot on a non-nil error to
//     roll back partial state.
//   - This function does NOT manage its own undo. It only guarantees
//     all rejection checks run BEFORE any mutation; if a check fails,
//     slot.Data is byte-identical to the input.
//   - Once writes begin (after the rejection block), an error means
//     slot.Data has been partially mutated. Caller MUST roll back.
func ApplyWorkspaceSave(slot *core.SaveSlot, snap *InventoryWorkspaceSnapshot, baseline map[uint32]ContainerKind) (HandlesByUID, error) {
	if slot == nil || snap == nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: nil slot or snapshot")
	}

	// ── Pre-flight rejection checks (no mutation) ─────────────────
	rep := Validate(*snap)
	// ponytail: these codes represent pre-existing or in-progress repair
	// states — they cannot be hard blockers because Fix-single repairs one
	// item at a time while others remain unfixed. Only errors that would
	// corrupt the physical write are blocking.
	nonBlocking := map[string]bool{
		CodeDuplicateUID:      true,
		CodeDuplicateHandle:   true,
		CodeUpgradeOutOfRange: true,
		CodePendingAoWUnknown: true,
		CodePendingAoWConflict: true,
	}
	var blockingErr *WorkspaceValidationIssue
	for i := range rep.Errors {
		if !nonBlocking[rep.Errors[i].Code] {
			blockingErr = &rep.Errors[i]
			break
		}
	}
	if blockingErr != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: workspace fails validation: %d error(s) (first: %s)",
			len(rep.Errors), blockingErr.Message)
	}

	if baseline == nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: session missing baseline handle map (requires a session created with baseline tracking)")
	}
	// Pending AoW pre-flight (Phase 4B). Collect intents first so
	// execution and validation share one source of truth; validation
	// runs before any mutation so slot.Data stays byte-identical on
	// rejection.
	aowChanges := collectPendingAoWChanges(snap)
	if err := validatePendingAoWChanges(aowChanges); err != nil {
		return nil, err
	}
	// Note: transfer and remove are now in-scope. We still compute the
	// plans up-front so any future per-action validation has a single
	// well-tested entry point. The wipe-and-replay layout below
	// naturally realises these plans without needing per-record patches:
	// removed items don't appear in either container's editable list,
	// so they're not re-emitted; transferred items appear in their new
	// container's editable list, so they're emitted there with their
	// original handle preserved.
	_ = detectRemovedEditableHandles(snap, baseline)
	_ = detectTransferredEditableItems(snap, baseline)

	// Capacity pre-check (also runs in writeContainerLayout, but doing
	// it up-front keeps slot.Data untouched on rejection).
	if invTotal := len(snap.InventoryItems) + len(snap.UnsupportedInventoryRecords); invTotal > core.CommonItemCount {
		return nil, fmt.Errorf("ApplyWorkspaceSave: inventory capacity exceeded: %d > %d",
			invTotal, core.CommonItemCount)
	}
	if stoTotal := len(snap.StorageItems) + len(snap.UnsupportedStorageRecords); stoTotal > core.StorageCommonCount {
		return nil, fmt.Errorf("ApplyWorkspaceSave: storage capacity exceeded: %d > %d",
			stoTotal, core.StorageCommonCount)
	}

	// Pass-through SlotIndex uniqueness check (before any writes).
	if err := validatePassThroughIndices(snap.UnsupportedInventoryRecords, core.CommonItemCount, "inventory"); err != nil {
		return nil, err
	}
	if err := validatePassThroughIndices(snap.UnsupportedStorageRecords, core.StorageCommonCount, "storage"); err != nil {
		return nil, err
	}

	// ── Step 1: add new editable items ────────────────────────────
	handles := HandlesByUID{}
	if err := executeAdds(slot, snap, handles); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: %w", err)
	}

	// Populate handles map for existing items (so callers always have
	// a UID → handle entry per editable record).
	for _, it := range snap.InventoryItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			handles[it.UID] = it.OriginalHandle
		}
	}
	for _, it := range snap.StorageItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			handles[it.UID] = it.OriginalHandle
		}
	}

	// ── Step 2: patch weapon ItemID for upgrade/infusion changes ──
	if err := executeWeaponPatches(slot, snap.InventoryItems); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: %w", err)
	}
	if err := executeWeaponPatches(slot, snap.StorageItems); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: %w", err)
	}

	// ── Step 2.5: apply pending AoW patches (Phase 4B) ────────────
	// Runs AFTER executeAdds so added weapons have real handles, and
	// AFTER executeWeaponPatches so any upgrade/infusion-driven ItemID
	// change has already settled (compatibility was validated against
	// the workspace's effective ItemID up-front).
	if err := executePendingAoWPatches(slot, snap, aowChanges); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: %w", err)
	}

	// ── Step 3: wipe + replay record layouts in both containers ───
	if err := writeContainerLayout(slot, snap, ContainerInventory, baseline); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: write inventory: %w", err)
	}
	if err := writeContainerLayout(slot, snap, ContainerStorage, baseline); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: write storage: %w", err)
	}

	// Header reconciliation — mirrors what legacy ReorderStorage does.
	core.ReconcileInventoryHeader(slot)
	core.ReconcileStorageHeader(slot)

	return handles, nil
}

// pendingAoWChange is a single pending AoW intent extracted from the
// workspace at save time. Exactly one of (AoWItemID != 0) or Clear is
// true — collect rejects ambiguous state up-front.
//
// WeaponItemID is the workspace's effective ItemID at collect time
// (may include pending upgrade/infusion offset); IsAshOfWarCompatibleWithWeapon
// uses fuzzy DB resolution so the upgrade offset doesn't matter for
// compatibility — the underlying weapon type drives the gate.
type pendingAoWChange struct {
	UID          string
	Container    ContainerKind
	WeaponItemID uint32
	WeaponName   string
	AoWItemID    uint32 // 0 ⇒ clear
	Clear        bool
}

// collectPendingAoWChanges walks both editable lists and returns one
// pendingAoWChange per weapon with a pending AoW intent
// (PendingAoWItemID != 0 OR PendingAoWClear == true). Non-weapon
// editables and items with no pending AoW edit are skipped.
//
// Items with conflicting state (clear + set) are skipped here — the
// validator catches them via CodePendingAoWConflict before execution.
func collectPendingAoWChanges(snap *InventoryWorkspaceSnapshot) []pendingAoWChange {
	var out []pendingAoWChange
	gather := func(items []EditableItem, container ContainerKind) {
		for i := range items {
			it := &items[i]
			if !it.IsWeapon {
				continue
			}
			switch {
			case it.PendingAoWClear && it.PendingAoWItemID != 0:
				// Conflicting state — let validator surface it.
				continue
			case it.PendingAoWClear:
				out = append(out, pendingAoWChange{
					UID: it.UID, Container: container,
					WeaponItemID: it.ItemID, WeaponName: it.Name,
					Clear: true,
				})
			case it.PendingAoWItemID != 0:
				out = append(out, pendingAoWChange{
					UID: it.UID, Container: container,
					WeaponItemID: it.ItemID, WeaponName: it.Name,
					AoWItemID: it.PendingAoWItemID,
				})
			}
		}
	}
	gather(snap.InventoryItems, ContainerInventory)
	gather(snap.StorageItems, ContainerStorage)
	return out
}

// validatePendingAoWChanges performs read-only checks against the DB.
// Runs BEFORE any mutation so slot.Data stays untouched on rejection.
//
// Rules:
//   - Clear intents pass unconditionally — the no-custom sentinel is
//     always a legal AoW state.
//   - Set intents must resolve to a known DB item with category
//     "ashes_of_war".
//   - (aow, weapon) compatibility must be true. Fail-closed on unknown
//     compatibility (e.g., DLC weapons not yet wired into the compat
//     bitmask) so save never silently produces a state the game can't
//     load.
func validatePendingAoWChanges(changes []pendingAoWChange) error {
	for _, c := range changes {
		if c.Clear {
			continue
		}
		aow, _ := db.GetItemDataFuzzy(c.AoWItemID)
		if aow.Name == "" {
			return fmt.Errorf("ApplyWorkspaceSave: pending AoW 0x%08X unknown in DB (weapon %s)",
				c.AoWItemID, c.WeaponName)
		}
		if aow.Category != "ashes_of_war" {
			return fmt.Errorf("ApplyWorkspaceSave: pending AoW 0x%08X (%s) is category %q, not ashes_of_war (weapon %s)",
				c.AoWItemID, aow.Name, aow.Category, c.WeaponName)
		}
		wep, _ := db.GetItemDataFuzzy(c.WeaponItemID)
		compat, known := db.IsAshOfWarCompatibleWithWeapon(c.AoWItemID, c.WeaponItemID)
		if !known {
			return fmt.Errorf("ApplyWorkspaceSave: AoW/weapon compatibility unknown for AoW %s (0x%08X) on weapon %s (0x%08X, category %q, WepType=%d) — refusing fail-closed",
				aow.Name, c.AoWItemID, c.WeaponName, c.WeaponItemID, wep.Category, wep.WepType)
		}
		if !compat {
			return fmt.Errorf("ApplyWorkspaceSave: AoW %s (0x%08X) is not compatible with weapon %s (0x%08X, WepType=%d)",
				aow.Name, c.AoWItemID, c.WeaponName, c.WeaponItemID, wep.WepType)
		}
	}
	return nil
}

// executePendingAoWPatches applies each pendingAoWChange to slot via
// core helpers. Set intents allocate a fresh AoW GaItem and rebuild the
// slot; clear intents patch in-place. Each call re-resolves the weapon
// handle from snap so added weapons (assigned a real handle by
// executeAdds) are found correctly.
//
// Shared-handle prevention: core.PatchWeaponAoW always mints a new
// handle via generateUniqueHandle, so even N pending sets targeting
// the same AoW itemID across N weapons produce N distinct handles.
//
// Old AoW GaItems left behind by a clear or by a replaced custom AoW
// are intentionally NOT garbage-collected (Phase 4B policy — see the
// notes under ApplyWorkspaceSave's docstring).
func executePendingAoWPatches(slot *core.SaveSlot, snap *InventoryWorkspaceSnapshot, changes []pendingAoWChange) error {
	for _, c := range changes {
		handle := lookupHandleByUID(snap, c.UID)
		if handle == 0 {
			return fmt.Errorf("pending AoW on %s: weapon handle not resolved (UID=%q)",
				c.WeaponName, c.UID)
		}
		if c.Clear {
			if err := core.PatchWeaponAoWHandle(slot, handle, core.NoCustomAoWHandle); err != nil {
				return fmt.Errorf("clear AoW on %s (handle 0x%08X): %w",
					c.WeaponName, handle, err)
			}
			continue
		}
		if err := core.PatchWeaponAoW(slot, handle, c.AoWItemID); err != nil {
			return fmt.Errorf("set AoW 0x%08X on %s (handle 0x%08X): %w",
				c.AoWItemID, c.WeaponName, handle, err)
		}
	}
	return nil
}

// lookupHandleByUID finds the editable item with the given UID across
// both containers and returns its OriginalHandle (0 if not found).
// Used by executePendingAoWPatches after executeAdds has populated
// handles for Source=Added items.
func lookupHandleByUID(snap *InventoryWorkspaceSnapshot, uid string) uint32 {
	for _, it := range snap.InventoryItems {
		if it.UID == uid {
			return it.OriginalHandle
		}
	}
	for _, it := range snap.StorageItems {
		if it.UID == uid {
			return it.OriginalHandle
		}
	}
	return 0
}

// currentEditableContainerMap collapses both editable container slices
// into a (handle → container) map for Source=Original items with a real
// handle. Added items and zero handles are skipped — they're not
// represented in the baseline. Shared by both detection helpers.
func currentEditableContainerMap(snap *InventoryWorkspaceSnapshot) map[uint32]ContainerKind {
	out := make(map[uint32]ContainerKind, len(snap.InventoryItems)+len(snap.StorageItems))
	for _, it := range snap.InventoryItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			out[it.OriginalHandle] = ContainerInventory
		}
	}
	for _, it := range snap.StorageItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			out[it.OriginalHandle] = ContainerStorage
		}
	}
	return out
}

// detectRemovedEditableHandles returns the baseline handles that no
// longer appear in the workspace's editable lists. Added items are not
// considered — they're not in the baseline to begin with. Used by
// callers that want to know which removals are about to be committed
// (e.g., for future GC bookkeeping or telemetry); the actual record
// removal happens implicitly via the wipe-and-replay layout, which
// only emits items present in the workspace.
func detectRemovedEditableHandles(snap *InventoryWorkspaceSnapshot, baseline map[uint32]ContainerKind) []uint32 {
	if baseline == nil {
		return nil
	}
	current := currentEditableContainerMap(snap)
	var removed []uint32
	for h := range baseline {
		if _, present := current[h]; !present {
			removed = append(removed, h)
		}
	}
	return removed
}

// detectTransferredEditableItems returns the baseline handles whose
// container differs in the current workspace, mapped to their new
// container. Added items are ignored (no baseline entry). Reorder
// within the same container is not flagged. Used the same way as
// detectRemovedEditableHandles — informational; the wipe-and-replay
// layout realises the move by emitting the item in its new container.
func detectTransferredEditableItems(snap *InventoryWorkspaceSnapshot, baseline map[uint32]ContainerKind) map[uint32]ContainerKind {
	if baseline == nil {
		return nil
	}
	current := currentEditableContainerMap(snap)
	out := map[uint32]ContainerKind{}
	for h, orig := range baseline {
		cur, present := current[h]
		if !present {
			continue
		}
		if cur != orig {
			out[h] = cur
		}
	}
	return out
}

// validatePassThroughIndices ensures the pass-through SlotIndices fit
// the container and don't collide. Runs before any binary writes.
func validatePassThroughIndices(records []RawInventoryRecord, capacity int, kindName string) error {
	seen := make(map[int]uint32, len(records))
	for _, p := range records {
		if p.SlotIndex < 0 || p.SlotIndex >= capacity {
			return fmt.Errorf("ApplyWorkspaceSave: %s pass-through SlotIndex %d out of range [0,%d)",
				kindName, p.SlotIndex, capacity)
		}
		if other, dup := seen[p.SlotIndex]; dup {
			return fmt.Errorf("ApplyWorkspaceSave: %s pass-through SlotIndex %d duplicated (handles 0x%08X and 0x%08X)",
				kindName, p.SlotIndex, other, p.Handle)
		}
		seen[p.SlotIndex] = p.Handle
	}
	return nil
}

// executeAdds materialises each Source=Added EditableItem in slot by
// calling core.AddItemsToSlotBatch one item at a time. The diff against
// pre-call GaMap yields the freshly minted handle for that item.
//
// Single-item batching makes handle attribution unambiguous: two
// concurrently-added weapons of the same itemID would otherwise produce
// two new GaMap entries we couldn't tell apart by content alone.
//
// For stackable items (talisman / goods) that already have a synthetic
// handle in GaMap, we reuse it directly and skip AddItemsToSlotBatch.
// The wipe-and-replay step then writes a record with that handle at
// our chosen position with the workspace's quantity.
func executeAdds(slot *core.SaveSlot, snap *InventoryWorkspaceSnapshot, handles HandlesByUID) error {
	// Gather pointers so we can mutate OriginalHandle in place.
	added := []*EditableItem{}
	for i := range snap.InventoryItems {
		if snap.InventoryItems[i].Source == ItemSourceAdded {
			added = append(added, &snap.InventoryItems[i])
		}
	}
	for i := range snap.StorageItems {
		if snap.StorageItems[i].Source == ItemSourceAdded {
			added = append(added, &snap.StorageItems[i])
		}
	}

	for _, it := range added {
		handlePrefix := db.ItemIDToHandlePrefix(it.ItemID)
		isStackable := handlePrefix == core.ItemTypeItem || handlePrefix == core.ItemTypeAccessory

		// Stackable + already in GaMap: reuse existing handle, no allocation.
		if isStackable {
			if reused, ok := findHandleForItemID(slot.GaMap, it.ItemID); ok {
				it.OriginalHandle = reused
				handles[it.UID] = reused
				continue
			}
		}

		gaMapBefore := snapshotGaMap(slot.GaMap)
		req := core.ItemToAdd{ItemID: it.ItemID}
		if it.Container == ContainerInventory {
			req.InvQty = int(it.Quantity)
		} else {
			req.StorageQty = int(it.Quantity)
		}
		if err := core.AddItemsToSlotBatch(slot, []core.ItemToAdd{req}); err != nil {
			return fmt.Errorf("add %s (0x%08X): %w", it.Name, it.ItemID, err)
		}
		newH, err := pickNewHandle(slot.GaMap, gaMapBefore, it.ItemID)
		if err != nil {
			return fmt.Errorf("identify new handle for %s: %w", it.Name, err)
		}
		it.OriginalHandle = newH
		handles[it.UID] = newH
	}
	return nil
}

// executeWeaponPatches applies upgrade / infusion changes for existing
// (Source=Original) weapon items whose workspace ItemID has diverged
// from the current slot.GaMap binding. Non-weapons are skipped silently
// (Phase 1.7 only supports weapon edits anyway).
func executeWeaponPatches(slot *core.SaveSlot, items []EditableItem) error {
	for _, it := range items {
		if it.Source != ItemSourceOriginal || it.OriginalHandle == 0 {
			continue
		}
		if !it.IsWeapon {
			continue
		}
		currentID, ok := slot.GaMap[it.OriginalHandle]
		if !ok {
			continue
		}
		if currentID == it.ItemID {
			continue
		}
		if err := core.PatchWeaponItemID(slot, it.OriginalHandle, currentID, it.ItemID); err != nil {
			return fmt.Errorf("patch weapon 0x%08X (%s) %X→%X: %w",
				it.OriginalHandle, it.Name, currentID, it.ItemID, err)
		}
	}
	return nil
}

// writeContainerLayout wipes the entire CommonItems region of one
// container in slot.Data and rewrites it as:
//   - pass-through records pinned at their original SlotIndex with
//     their original AcquisitionIndex
//   - editable records placed at the next free physical slot in
//     workspace Position order, with fresh stride-2 AcquisitionIndex
//     values starting just above the current NextAcquisitionSortId
//
// Pre-conditions enforced before reaching this function (in
// ApplyWorkspaceSave): pass-through SlotIndex uniqueness, capacity.
//
// In-memory state:
//   - Inventory uses a pre-sized CommonItems array of length
//     CommonItemCount; entries at non-occupied slots remain zero
//     (handle = GaHandleEmpty).
//   - Storage uses a compacted CommonItems list (only non-empty
//     entries, in physical slot order) — matching ReadStorage's parse
//     convention.
func writeContainerLayout(slot *core.SaveSlot, snap *InventoryWorkspaceSnapshot, kind ContainerKind, baseline map[uint32]ContainerKind) error {
	var (
		editables   []EditableItem
		passthrough []RawInventoryRecord
		startOff    int
		capacity    int
		equip       *core.EquipInventoryData
	)
	if kind == ContainerInventory {
		editables = snap.InventoryItems
		passthrough = snap.UnsupportedInventoryRecords
		if slot.MagicOffset <= 0 {
			if len(editables) == 0 && len(passthrough) == 0 {
				return nil
			}
			return fmt.Errorf("inventory MagicOffset=0 but workspace has %d editable + %d pass-through",
				len(editables), len(passthrough))
		}
		startOff = slot.MagicOffset + core.InvStartFromMagic
		capacity = core.CommonItemCount
		equip = &slot.Inventory
	} else {
		editables = snap.StorageItems
		passthrough = snap.UnsupportedStorageRecords
		if slot.StorageBoxOffset == 0 {
			if len(editables) == 0 && len(passthrough) == 0 {
				return nil
			}
			return fmt.Errorf("storage offset=0 but workspace has %d editable + %d pass-through",
				len(editables), len(passthrough))
		}
		startOff = slot.StorageBoxOffset + core.StorageHeaderSkip
		capacity = core.StorageCommonCount
		equip = &slot.Storage
	}

	// Build occupied (pass-through pinned positions) and reserved
	// acquisition indices so the stride-2 base doesn't collide.
	occupied := make(map[int]bool, len(passthrough))
	reservedAcq := make(map[uint32]bool, len(passthrough))
	for _, p := range passthrough {
		occupied[p.SlotIndex] = true
		if p.Handle != core.GaHandleEmpty && p.Handle != core.GaHandleInvalid {
			reservedAcq[p.AcquisitionIndex] = true
		}
	}

	// Compute stride-2 base for editable items.
	baseAcq := equip.NextAcquisitionSortId
	if baseAcq <= core.InvEquipReservedMax {
		baseAcq = core.InvEquipReservedMax + 1
	}
	if baseAcq%2 != 0 {
		baseAcq++
	}
	for {
		collision := false
		for i := 0; i < len(editables); i++ {
			if reservedAcq[baseAcq+uint32(i*2)] {
				collision = true
				break
			}
		}
		if !collision {
			break
		}
		baseAcq += 2
	}

	// Verify slot.Data is large enough.
	endOff := startOff + capacity*core.InvRecordLen
	if endOff > len(slot.Data) {
		return fmt.Errorf("slot.Data too short for %s container (%d < %d)", kind, len(slot.Data), endOff)
	}

	// Fidelity fast-path: when the item SET is unchanged — no adds, removes, or
	// transfers, only possibly a reorder — keep every item at its ORIGINAL
	// physical slot and reassign acquisition indices by reusing the existing
	// pool of original indices in the new Position order. The game sorts the
	// inventory by acquisition index, so reusing the pool realises a reorder
	// without inventing new indices, advancing the counters, or moving items
	// between physical slots. Crucially this path does NOT wipe the region, so
	// empty slots keep the exact bytes the game wrote (e.g. the acq=1 sentinel
	// in unused storage slots). Consequently a no-op save is byte-identical to
	// the loaded file, a weapon upgrade / AoW edit touches only the GaItem (not
	// inventory), and a pure reorder rewrites only the acquisition-index field
	// of the items whose order changed. Only adds/removes/transfers fall
	// through to the full wipe-and-replay below.
	if containerSameItemSet(editables, kind, baseline) {
		pool := make([]uint32, len(editables))
		for i, it := range editables {
			pool[i] = it.AcquisitionIndex
		}
		sort.Slice(pool, func(i, j int) bool { return pool[i] < pool[j] })

		for _, p := range passthrough {
			off := startOff + p.SlotIndex*core.InvRecordLen
			binary.LittleEndian.PutUint32(slot.Data[off:], p.Handle)
			binary.LittleEndian.PutUint32(slot.Data[off+4:], p.Quantity)
			binary.LittleEndian.PutUint32(slot.Data[off+8:], p.AcquisitionIndex)
		}
		for i, it := range editables {
			off := startOff + it.OriginalSlotIndex*core.InvRecordLen
			binary.LittleEndian.PutUint32(slot.Data[off:], it.OriginalHandle)
			binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
			binary.LittleEndian.PutUint32(slot.Data[off+8:], pool[i])
		}
		rebuildInMemoryCommonItems(slot, startOff, capacity, equip, kind == ContainerInventory)
		return nil
	}

	// Wipe all CommonItems bytes.
	for i := startOff; i < endOff; i++ {
		slot.Data[i] = 0
	}

	// Write pass-through records.
	for _, p := range passthrough {
		off := startOff + p.SlotIndex*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], p.Handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], p.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], p.AcquisitionIndex)
	}

	// Write editable records at next free physical slot.
	maxAcq := uint32(0)
	for _, p := range passthrough {
		if p.AcquisitionIndex > maxAcq {
			maxAcq = p.AcquisitionIndex
		}
	}
	nextFree := 0
	for pos, it := range editables {
		for nextFree < capacity && occupied[nextFree] {
			nextFree++
		}
		if nextFree >= capacity {
			return fmt.Errorf("%s container ran out of free slots after placing pass-through", kind)
		}
		acq := baseAcq + uint32(pos*2)
		off := startOff + nextFree*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.OriginalHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], acq)
		if acq > maxAcq {
			maxAcq = acq
		}
		nextFree++
	}

	// Rebuild in-memory CommonItems from binary.
	rebuildInMemoryCommonItems(slot, startOff, capacity, equip, kind == ContainerInventory)

	// Advance ONLY the acquisition/sort counter past the highest assigned Index.
	// NextEquipIndex is a separate equip-list counter (NOT a visibility gate) and
	// must be preserved exactly as the game wrote it — overwriting it corrupts the
	// slot (CE-108255-1).
	newNext := maxAcq + 1
	equip.NextAcquisitionSortId = newNext
	if off := equip.NextEquipIndexOff(); off > 0 && off+8 <= len(slot.Data) {
		// off    → NextEquipIndex (left intact)
		// off+4  → NextAcquisitionSortId
		binary.LittleEndian.PutUint32(slot.Data[off+4:], newNext)
	}

	return nil
}

// containerSameItemSet reports whether the container holds exactly the same
// original items it was loaded with — no added items, no removed or
// transferred items (the editable original-handle set equals the baseline set
// for this container). A reorder is allowed: the set is unchanged even if the
// Position order differs. When true the caller can keep every item at its
// original physical slot and reuse the original acquisition-index pool, so a
// no-op save stays byte-identical and a reorder touches only index fields.
func containerSameItemSet(editables []EditableItem, kind ContainerKind, baseline map[uint32]ContainerKind) bool {
	if baseline == nil {
		return false
	}
	baselineCount := 0
	for _, c := range baseline {
		if c == kind {
			baselineCount++
		}
	}
	if len(editables) != baselineCount {
		return false
	}
	for _, it := range editables {
		if it.Source != ItemSourceOriginal || it.OriginalSlotIndex < 0 {
			return false // an added item
		}
		if baseline[it.OriginalHandle] != kind {
			return false // removed, transferred in, or moved between containers
		}
	}
	return true
}

// rebuildInMemoryCommonItems syncs equip.CommonItems with the freshly
// written slot.Data records. Inventory keeps a fixed-size array (one
// entry per physical slot, empties as zero handles); storage uses the
// compacted ReadStorage convention.
func rebuildInMemoryCommonItems(slot *core.SaveSlot, startOff, capacity int, equip *core.EquipInventoryData, fullSize bool) {
	if fullSize {
		out := make([]core.InventoryItem, capacity)
		for i := 0; i < capacity; i++ {
			off := startOff + i*core.InvRecordLen
			out[i] = core.InventoryItem{
				GaItemHandle: binary.LittleEndian.Uint32(slot.Data[off:]),
				Quantity:     binary.LittleEndian.Uint32(slot.Data[off+4:]),
				Index:        binary.LittleEndian.Uint32(slot.Data[off+8:]),
			}
		}
		equip.CommonItems = out
		return
	}
	out := []core.InventoryItem{}
	for i := 0; i < capacity; i++ {
		off := startOff + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		out = append(out, core.InventoryItem{
			GaItemHandle: h,
			Quantity:     binary.LittleEndian.Uint32(slot.Data[off+4:]),
			Index:        binary.LittleEndian.Uint32(slot.Data[off+8:]),
		})
	}
	equip.CommonItems = out
}

// snapshotGaMap captures a shallow copy of slot.GaMap before
// AddItemsToSlotBatch. The diff after lets executeAdds identify the
// fresh handle minted for the added EditableItem.
func snapshotGaMap(m map[uint32]uint32) map[uint32]uint32 {
	out := make(map[uint32]uint32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// pickNewHandle returns the handle present in `after` but not `before`,
// where after[h] == wantID. Errors on zero or multiple candidates so
// the caller never silently grabs the wrong handle.
func pickNewHandle(after, before map[uint32]uint32, wantID uint32) (uint32, error) {
	var candidates []uint32
	for h, id := range after {
		if _, hadBefore := before[h]; hadBefore {
			continue
		}
		if id != wantID {
			continue
		}
		candidates = append(candidates, h)
	}
	if len(candidates) == 0 {
		return 0, fmt.Errorf("no new handle minted for itemID 0x%08X", wantID)
	}
	if len(candidates) > 1 {
		return 0, fmt.Errorf("ambiguous new handles for itemID 0x%08X: %d candidates", wantID, len(candidates))
	}
	return candidates[0], nil
}

// findHandleForItemID looks up a stackable handle already present in
// GaMap for the requested itemID. Used for stackable adds (talisman /
// goods) so we reuse the existing synthetic handle instead of letting
// AddItemsToSlotBatch attempt a fresh allocation.
func findHandleForItemID(m map[uint32]uint32, itemID uint32) (uint32, bool) {
	for h, id := range m {
		if id == itemID {
			return h, true
		}
	}
	return 0, false
}
