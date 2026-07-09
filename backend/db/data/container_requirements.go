package data

// Container key item IDs.
const (
	CrackedPotKeyItemID      uint32 = 0x4000251C
	RitualPotKeyItemID       uint32 = 0x4000251D
	PerfumeBottleKeyItemID   uint32 = 0x40002526
	HeftyCrackedPotKeyItemID uint32 = 0x401EA99C
)

// RequiredContainer maps a craftable item ID (Throwing Pot or Aromatic) to the
// key-item container it requires. The container's MaxInventory cap limits how
// many distinct items mapping to it the character may carry in Inventory.
// Storage is NOT subject to this cap.
var RequiredContainer = map[uint32]uint32{
	// === Cracked Pot (cap 20) — 23 Throwing Pots ===
	// Standard (11)
	0x4000012C: CrackedPotKeyItemID, // Fire Pot
	0x40000140: CrackedPotKeyItemID, // Lightning Pot
	0x4000014A: CrackedPotKeyItemID, // Fetid Pot
	0x40000154: CrackedPotKeyItemID, // Swarm Pot
	0x4000015E: CrackedPotKeyItemID, // Holy Water Pot
	0x40000172: CrackedPotKeyItemID, // Poison Pot
	0x4000017C: CrackedPotKeyItemID, // Oil Pot
	0x40000258: CrackedPotKeyItemID, // Volcano Pot
	0x40000280: CrackedPotKeyItemID, // Sleep Pot
	0x4000028A: CrackedPotKeyItemID, // Rancor Pot
	0x40000294: CrackedPotKeyItemID, // Magic Pot
	// Roped (9)
	0x40000190: CrackedPotKeyItemID, // Roped Fire Pot
	0x400001A4: CrackedPotKeyItemID, // Roped Lightning Pot
	0x400001AE: CrackedPotKeyItemID, // Roped Fetid Pot
	0x400001B8: CrackedPotKeyItemID, // Roped Poison Pot
	0x400001C2: CrackedPotKeyItemID, // Roped Oil Pot
	0x400001CC: CrackedPotKeyItemID, // Roped Magic Pot
	0x400001D6: CrackedPotKeyItemID, // Roped Fly Pot
	0x400001E0: CrackedPotKeyItemID, // Roped Freezing Pot (cut)
	0x400001EA: CrackedPotKeyItemID, // Roped Volcano Pot
	0x400001FE: CrackedPotKeyItemID, // Roped Holy Water Pot
	// DLC (3)
	0x401E873C: CrackedPotKeyItemID, // Red Lightning Pot
	0x401E8746: CrackedPotKeyItemID, // Frenzied Flame Pot
	0x401E8778: CrackedPotKeyItemID, // Roped Frenzied Flame Pot
	// Cut/unused
	0x4000D17E: CrackedPotKeyItemID, // ?GoodsName? Holy Water Pot

	// === Ritual Pot (cap 10) — 12 ===
	0x4000012D: RitualPotKeyItemID, // Redmane Fire Pot
	0x40000141: RitualPotKeyItemID, // Ancient Dragonbolt Pot
	0x4000012E: RitualPotKeyItemID, // Giantsflame Fire Pot
	0x4000015F: RitualPotKeyItemID, // Sacred Order Pot
	0x40000168: RitualPotKeyItemID, // Freezing Pot
	0x40000186: RitualPotKeyItemID, // Alluring Pot
	0x40000187: RitualPotKeyItemID, // Beastlure Pot
	0x40000262: RitualPotKeyItemID, // Albinauric Pot
	0x40000276: RitualPotKeyItemID, // Cursed-Blood Pot
	0x40000295: RitualPotKeyItemID, // Academy Magic Pot
	0x4000029E: RitualPotKeyItemID, // Rot Pot
	0x401E8764: RitualPotKeyItemID, // Eternal Sleep Pot (DLC)

	// === Hefty Cracked Pot (cap 10) — 15 DLC ===
	0x401E85AC: HeftyCrackedPotKeyItemID, // Hefty Fire Pot
	0x401E85B6: HeftyCrackedPotKeyItemID, // Hefty Rock Pot
	0x401E85C0: HeftyCrackedPotKeyItemID, // Hefty Lightning Pot
	0x401E85CA: HeftyCrackedPotKeyItemID, // Hefty Fetid Pot
	0x401E85D4: HeftyCrackedPotKeyItemID, // Hefty Fly Pot
	0x401E85E8: HeftyCrackedPotKeyItemID, // Hefty Freezing Pot
	0x401E85F2: HeftyCrackedPotKeyItemID, // Hefty Poison Pot
	0x401E85FC: HeftyCrackedPotKeyItemID, // Hefty Oil Pot
	0x401E86D8: HeftyCrackedPotKeyItemID, // Hefty Volcano Pot
	0x401E86EC: HeftyCrackedPotKeyItemID, // Hefty Frenzied Flame Pot
	0x401E870A: HeftyCrackedPotKeyItemID, // Hefty Rancor Pot
	0x401E8714: HeftyCrackedPotKeyItemID, // Hefty Magic Pot
	0x401E871E: HeftyCrackedPotKeyItemID, // Hefty Rot Pot
	0x401E8728: HeftyCrackedPotKeyItemID, // Hefty Furnace Pot
	0x401E8732: HeftyCrackedPotKeyItemID, // Hefty Red Lightning Pot

	// === Perfume Bottle (cap 10) — 7 Aromatics (Tools/Perfume Arts) ===
	0x40000DAC: PerfumeBottleKeyItemID, // Uplifting Aromatic
	0x40000DB6: PerfumeBottleKeyItemID, // Spark Aromatic
	0x40000DC0: PerfumeBottleKeyItemID, // Ironjar Aromatic
	0x40000DDE: PerfumeBottleKeyItemID, // Bloodboil Aromatic
	0x40000DFC: PerfumeBottleKeyItemID, // Poison Spraymist
	0x40000E1A: PerfumeBottleKeyItemID, // Acid Spraymist
	0x401E90C4: PerfumeBottleKeyItemID, // Perfumed Oil of Ranah (DLC)
}

// GetRequiredContainer returns the container key-item ID that the given craftable
// item requires, and ok=true if the item is gated by a container.
func GetRequiredContainer(itemID uint32) (uint32, bool) {
	c, ok := RequiredContainer[itemID]
	return c, ok
}

// ContainerItemIDs is the set of key-item container IDs referenced as required
// containers. Membership means "this item IS a container" — used to total the
// owned container quantity that caps how many gated craftables may be carried.
var ContainerItemIDs = map[uint32]struct{}{
	CrackedPotKeyItemID:      {},
	RitualPotKeyItemID:       {},
	PerfumeBottleKeyItemID:   {},
	HeftyCrackedPotKeyItemID: {},
}

// IsContainerItem reports whether itemID is one of the container key items.
func IsContainerItem(itemID uint32) bool {
	_, ok := ContainerItemIDs[itemID]
	return ok
}

// CapDecision is the result of applying a container cap to one item add.
type CapDecision struct {
	EffectiveQty int // target stack qty actually used (after cap)
	CutQty       int // qty trimmed because container would overflow
}

// ApplyContainerCap enforces a per-container TOTAL QUANTITY cap on a single
// add operation. Cap counts total pot units across all stacks mapped to that
// container — NOT distinct types.
//
// Semantics match `addToInventory` in core/writer.go: `targetQty` is the SET
// value for the item's stack (not a delta). The container occupancy delta is
// `targetQty - existingItemQty[id]`.
//
// Mutates `existingItemQty[id]` and `existingByContainer[cID]` to reflect the
// post-decision state, so caller can chain calls per item in batch.
//
// For non-gated items (no RequiredContainer entry), simply records the new
// stack qty and returns EffectiveQty = targetQty.
//
// `containerCap` returns the cap (e.g. 20 for Cracked Pot).
func ApplyContainerCap(
	id uint32, targetQty int,
	existingItemQty map[uint32]int,
	existingByContainer map[uint32]int,
	containerCap func(uint32) int,
) CapDecision {
	cID, ok := RequiredContainer[id]
	if !ok {
		existingItemQty[id] = targetQty
		return CapDecision{EffectiveQty: targetQty}
	}
	existing := existingItemQty[id]
	delta := targetQty - existing
	if delta <= 0 {
		// SET to lower or equal qty: frees (or leaves unchanged) container slots.
		existingItemQty[id] = targetQty
		existingByContainer[cID] += delta
		return CapDecision{EffectiveQty: targetQty}
	}
	cap := containerCap(cID)
	remain := cap - existingByContainer[cID]
	if remain < 0 {
		remain = 0
	}
	if delta <= remain {
		existingItemQty[id] = targetQty
		existingByContainer[cID] += delta
		return CapDecision{EffectiveQty: targetQty}
	}
	// Container would overflow — cut overflow off the requested delta.
	allowed := remain
	cut := delta - allowed
	newTarget := existing + allowed
	existingItemQty[id] = newTarget
	existingByContainer[cID] += allowed
	return CapDecision{EffectiveQty: newTarget, CutQty: cut}
}
