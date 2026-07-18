package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// invHeaderItemID is a plain stackable crafting material (Aeonian Butterfly)
// added through the public Database Add path with no container/flag/tutorial side
// effects, so the only side-effect lifecycle these tests exercise is the
// held-inventory CommonItems count header itself. Adding it into an empty
// inventory allocates a new Inventory.CommonItems record; re-adding it stacks the
// existing record in place.
const invHeaderItemID = uint32(0x40005141)

// invCommonHeaderOffset returns the physical offset of the 4-byte held-inventory
// CommonItems count header ("common_item_count") — the scalar the writer bumps in
// place and this task instruments — mirroring the reader under test.
func invCommonHeaderOffset(slot *core.SaveSlot) int {
	return slot.MagicOffset + core.InvStartFromMagic - 4
}

// seedInventoryCommonHeader stamps the raw held-inventory CommonItems count
// header with value, letting a test seed a deliberately stale count independent
// of the actual number of physical records.
func seedInventoryCommonHeader(slot *core.SaveSlot, value uint32) {
	binary.LittleEndian.PutUint32(slot.Data[invCommonHeaderOffset(slot):], value)
}

// rawInventoryHeader reads the physical 4-byte held-inventory CommonItems count
// scalar back out of the real slot after an operation, so tests can prove
// finished equals the raw header the writer actually wrote.
func rawInventoryHeader(slot *core.SaveSlot) string {
	return giDec(binary.LittleEndian.Uint32(slot.Data[invCommonHeaderOffset(slot):]))
}

// A. New inventory record. Starting from an empty inventory with header 0, a
// database-valid inventory add allocates one new Inventory.CommonItems record and
// the writer increments the count header in place: 0 -> 1 -> 1.
func TestGameItemsAddInventoryHeaderNewRecord(t *testing.T) {
	app := gameItemAddApp(true)
	slot := &app.save.Slots[0]

	if got := rawInventoryHeader(slot); got != "0" {
		t.Fatalf("precondition: raw inventory header = %q, want 0", got)
	}

	res, err := app.AddItemsToCharacter(0, []uint32{invHeaderItemID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	// empty header -> one new record -> one new record.
	assertContainerLifecycle(t, lc, "inventory_common_header_count", "0", "1", "1")

	// finished equals the real raw header scalar after the operation.
	if got := rawInventoryHeader(slot); got != lc.finished["inventory_common_header_count"] {
		t.Errorf("finished inventory_common_header_count %q != real raw header %q", lc.finished["inventory_common_header_count"], got)
	}
}

// B. Existing stack update. A stackable record already present in inventory is
// bumped in place by a re-add — no new Inventory.CommonItems record is allocated —
// so the writer never touches the count header and no
// inventory_common_header_count lifecycle is emitted. The direct quantity change
// still carries its own lifecycle.
func TestGameItemsAddInventoryHeaderStackUpdate(t *testing.T) {
	app := gameItemAddApp(false) // seed silently, journal only the second add
	slot := &app.save.Slots[0]

	// Seed a valid existing stackable record + matching header via the public add.
	if _, err := app.AddItemsToCharacter(0, []uint32{invHeaderItemID}, 0, 0, 0, 0, 2, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	if got := rawInventoryHeader(slot); got != "1" {
		t.Fatalf("precondition: raw inventory header after seed = %q, want 1", got)
	}

	// Fresh journal so only the stacking add is observed.
	journal := newInMemoryDiagnosticJournal()
	journal.SetDebugEnabled(true)
	app.journal = journal

	res, err := app.AddItemsToCharacter(0, []uint32{invHeaderItemID}, 0, 0, 0, 0, 5, 0)
	if err != nil {
		t.Fatalf("stack AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	// In-place stack update allocates no new record: header self-excludes.
	assertNoField(t, lc, "inventory_common_header_count")

	// The direct quantity change still carries its own lifecycle: 2 -> 5 -> 5.
	assertContainerLifecycle(t, lc, "inventory_common_row_0_quantity", "2", "5", "5")

	// Header untouched by the stacking add.
	if got := rawInventoryHeader(slot); got != "1" {
		t.Errorf("raw inventory header after stack = %q, want 1", got)
	}
}

// C. Stale header preserved. The writer increments the count header in place and
// is NOT reconciled on the add path, so a deliberately stale header advances by a
// raw +1 (42 -> 43), never collapsing to the reconciled physical count. This is a
// regression guard against changing writer semantics while instrumenting them.
func TestGameItemsAddInventoryHeaderStalePreserved(t *testing.T) {
	app := gameItemAddApp(true)
	slot := &app.save.Slots[0]

	const stale = uint32(42)
	seedInventoryCommonHeader(slot, stale) // empty inventory, deliberately stale header

	res, err := app.AddItemsToCharacter(0, []uint32{invHeaderItemID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	// Raw +1, not a reconciled count (which would be 1).
	assertContainerLifecycle(t, lc, "inventory_common_header_count", giDec(stale), giDec(stale+1), giDec(stale+1))

	// finished equals the real raw header scalar after the operation.
	if got := rawInventoryHeader(slot); got != lc.finished["inventory_common_header_count"] {
		t.Errorf("finished inventory_common_header_count %q != real raw header %q", lc.finished["inventory_common_header_count"], got)
	}
}

// D. Unavailable header. A fixture with no valid magic region (MagicOffset zeroed)
// has no readable held-inventory count header, so the reader returns the
// unavailable sentinel in both slots and no inventory_common_header_count field is
// emitted — while the normal direct add lifecycle still runs and the add succeeds.
func TestGameItemsAddInventoryHeaderUnavailable(t *testing.T) {
	app := gameItemAddApp(true)
	slot := &app.save.Slots[0]
	slot.MagicOffset = 0 // no valid magic region → count header unavailable

	res, err := app.AddItemsToCharacter(0, []uint32{invHeaderItemID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	assertNoField(t, lc, "inventory_common_header_count")
	assertContainerLifecycle(t, lc, "inventory_common_row_0_item_id", giAbsent, giHex(invHeaderItemID), giHex(invHeaderItemID))
}
