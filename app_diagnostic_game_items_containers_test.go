package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// assertContainerLifecycle asserts one container field carries the full
// before -> planned -> finished lifecycle with the given values.
func assertContainerLifecycle(t *testing.T, lc gameItemLifecycle, field, before, planned, finished string) {
	t.Helper()
	if got, ok := lc.before[field]; !ok {
		t.Errorf("missing before record for %q", field)
	} else if got != before {
		t.Errorf("before %s = %q, want %q", field, got, before)
	}
	if got, ok := lc.planned[field]; !ok {
		t.Errorf("missing planned record for %q", field)
	} else if got != planned {
		t.Errorf("planned %s = %q, want %q", field, got, planned)
	}
	if got, ok := lc.finished[field]; !ok {
		t.Errorf("missing finished record for %q", field)
	} else if got != finished {
		t.Errorf("finished %s = %q, want %q", field, got, finished)
	}
}

// assertNoField asserts no lifecycle phase emitted the named field.
func assertNoField(t *testing.T, lc gameItemLifecycle, field string) {
	t.Helper()
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		if _, ok := phase[field]; ok {
			t.Errorf("unexpected lifecycle record for %q", field)
			return
		}
	}
}

// assertGameItemPhaseGrouping asserts every before record precedes every planned record,
// and every planned precedes every finished — the global phase grouping.
func assertGameItemPhaseGrouping(t *testing.T, lc gameItemLifecycle) {
	t.Helper()
	if len(lc.beforeIx) == 0 || len(lc.planIx) == 0 || len(lc.finIx) == 0 {
		t.Fatalf("missing a lifecycle phase: before=%d planned=%d finished=%d",
			len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	if maxInt(lc.beforeIx) >= minInt(lc.planIx) {
		t.Errorf("a before record does not precede all planned records")
	}
	if maxInt(lc.planIx) >= minInt(lc.finIx) {
		t.Errorf("a planned record does not precede all finished records")
	}
}

// A. Derived inventory container success: adding Fire Pot (inventory qty 2)
// raises the Cracked Pot container as a side effect. The semantic container
// quantity and the pickup/vendor flags the writer sets must carry the full
// lifecycle, while the derived container's direct physical row stays absent.
func TestGameItemsAddContainerDiagnosticDerivedInventory(t *testing.T) {
	app := gameItemAddApp(true)
	withContainerEventFlags(app)

	res, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemPhaseGrouping(t, lc)

	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}

	// Semantic container quantity: no physical container existed, the sync raised
	// it to 2 (two gated units), and the real slot kept it at 2.
	assertContainerLifecycle(t, lc, "container_0x4000251C_quantity", giAbsent, "2", "2")

	// The first two Cracked Pot pickup flags flip (finalQty = 2).
	pickup := data.ContainerPickupFlags[data.CrackedPotKeyItemID]
	for i := 0; i < 2; i++ {
		field := "container_0x4000251C_pickup_flag_" + giDec(pickup[i])
		assertContainerLifecycle(t, lc, field, "false", "true", "true")
	}

	// Every Cracked Pot vendor flag the writer sets carries the lifecycle.
	for _, f := range data.ContainerVendorPurchaseFlags[data.CrackedPotKeyItemID] {
		field := "container_0x4000251C_vendor_flag_" + giDec(f)
		assertContainerLifecycle(t, lc, field, "false", "true", "true")
	}

	// The real final slot state matches the finished quantity record.
	slot := &app.save.Slots[0]
	if got := readContainerQuantity(slot, data.CrackedPotKeyItemID); got != lc.finished["container_0x4000251C_quantity"] {
		t.Errorf("finished container quantity %q != real slot %q", lc.finished["container_0x4000251C_quantity"], got)
	}

	// Task 2B non-goal: the derived Cracked Pot direct physical row must NOT be
	// logged. No direct inventory/gaitem field may carry its handle or item ID.
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		for field, val := range phase {
			if val == "0xB000251C" || val == "0x4000251C" {
				t.Errorf("derived container leaked into direct record %s = %q", field, val)
			}
		}
	}
}

// B. Storage-only container: a Storage-only Spark Aromatic add consumes no
// per-unit container but forces exactly one Perfume Bottle. Only the first
// pickup flag flips; the second must not be logged.
func TestGameItemsAddContainerDiagnosticStorageOnly(t *testing.T) {
	app := gameItemAddApp(true)
	withContainerEventFlags(app)

	res, err := app.AddItemsToCharacter(0, []uint32{sparkAromaticID}, 0, 0, 0, 0, 0, 2)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemPhaseGrouping(t, lc)

	// Storage-only forces the container to exactly 1.
	assertContainerLifecycle(t, lc, "container_0x40002526_quantity", giAbsent, "1", "1")

	pickup := data.ContainerPickupFlags[data.PerfumeBottleKeyItemID]
	assertContainerLifecycle(t, lc,
		"container_0x40002526_pickup_flag_"+giDec(pickup[0]), "false", "true", "true")
	// finalQty = 1, so the second pickup flag never flips and never logs.
	assertNoField(t, lc, "container_0x40002526_pickup_flag_"+giDec(pickup[1]))
}

// C. No Event Flags region: the fixture leaves EventFlagsOffset at 0 (invalid),
// so the writer sets no flags. The semantic container quantity lifecycle must
// still be present, but no pickup/vendor flag field may be emitted.
func TestGameItemsAddContainerDiagnosticNoEventFlags(t *testing.T) {
	app := gameItemAddApp(true) // no withContainerEventFlags: EventFlagsOffset stays 0

	res, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())

	// The quantity side effect is independent of the Event Flags region.
	assertContainerLifecycle(t, lc, "container_0x4000251C_quantity", giAbsent, "2", "2")

	// No container flag field may exist without a valid Event Flags region.
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		for field := range phase {
			if strings.HasPrefix(field, "container_0x4000251C_pickup_flag_") ||
				strings.HasPrefix(field, "container_0x4000251C_vendor_flag_") {
				t.Errorf("flag field emitted without an Event Flags region: %q", field)
			}
		}
	}
}

// D. Explicit container anti-duplication: adding Fire Pot and Cracked Pot
// together makes Cracked Pot both a gated (used) container and an explicit
// plan.batch entry. Its direct physical row keeps the Task 2B lifecycle, and no
// container_0x4000251C_quantity side-effect record may be emitted; flag changes
// still carry their lifecycle.
func TestGameItemsAddContainerDiagnosticExplicitAntiDuplication(t *testing.T) {
	app := gameItemAddApp(true)
	withContainerEventFlags(app)

	const crackedPotID = uint32(0x4000251C)
	res, err := app.AddItemsToCharacter(0, []uint32{firePotID, crackedPotID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 2 {
		t.Fatalf("res.Added = %d, want 2", res.Added)
	}

	slot := &app.save.Slots[0]
	row, qty := findKeyItem(slot, crackedPotHandle)
	if row < 0 {
		t.Fatalf("Cracked Pot not written to KeyItems")
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemPhaseGrouping(t, lc)

	// Task 2B direct physical row lifecycle is present for the explicit container
	// (KeyItems is the native destination per T074 — see nativeKeyItemFamily).
	prefix := "inventory_key_row_" + giDec(uint32(row)) + "_"
	assertContainerLifecycle(t, lc, prefix+"handle", "0x00000000", giHex(crackedPotHandle), giHex(crackedPotHandle))
	assertContainerLifecycle(t, lc, prefix+"item_id", giAbsent, giHex(crackedPotID), giHex(crackedPotID))
	assertContainerLifecycle(t, lc, prefix+"quantity", "0", giDec(qty), giDec(qty))

	// Anti-duplication: no container quantity side-effect record for the explicit
	// container — its quantity is already described by the direct row above.
	assertNoField(t, lc, "container_0x4000251C_quantity")

	// Flag side effects still emit their lifecycle.
	pickup := data.ContainerPickupFlags[data.CrackedPotKeyItemID]
	assertContainerLifecycle(t, lc,
		"container_0x4000251C_pickup_flag_"+giDec(pickup[0]), "false", "true", "true")
	for _, f := range data.ContainerVendorPurchaseFlags[data.CrackedPotKeyItemID] {
		assertContainerLifecycle(t, lc,
			"container_0x4000251C_vendor_flag_"+giDec(f), "false", "true", "true")
	}
}
