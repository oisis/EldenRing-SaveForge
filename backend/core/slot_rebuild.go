package core

import "fmt"

// Section names emitted by buildSectionMap. Stable identifiers so callers
// (rebuild logic, tests, diagnostics) can reference sections by name.
const (
	SectionEmptySlot        = "empty_slot"
	SectionPreUnlockedRegs  = "pre_unlocked_regions"
	SectionUnlockedRegs     = "unlocked_regions"
	SectionPostUnlockedRegs = "post_unlocked_regions"
	SectionDLC              = "dlc"
	SectionHash             = "player_data_hash"
)

// SectionRange describes a contiguous byte range inside slot.Data.
// End is exclusive. Size returns End - Start.
type SectionRange struct {
	Name  string
	Start int
	End   int
}

// Size returns the byte length of the section.
func (r SectionRange) Size() int { return r.End - r.Start }

// buildSectionMap computes the section boundaries used by RebuildSlot.
//
// For empty / unparseable slots (Version == 0 or UnlockedRegionsOffset == 0)
// we emit a single "empty_slot" section spanning the whole 0x280000 buffer —
// rebuilding such a slot is a verbatim copy.
//
// For active slots we emit five sections that together cover [0, SlotSize):
//
//	pre_unlocked_regions   [0, UnlockedRegionsOffset)
//	unlocked_regions       [UnlockedRegionsOffset, UnlockedRegionsOffset + 4 + 4*N)
//	post_unlocked_regions  [unlocked_regions_end, DlcSectionOffset)
//	dlc                    [DlcSectionOffset, DlcSectionOffset + DlcSectionSize)
//	player_data_hash       [HashOffset, HashOffset + HashSize)
//
// The post_unlocked_regions blob folds together every section after the
// regions block (horse, blood_stain, ..., event_flags, world_area, net_man,
// weather, time, base_version, steam_id, ps5_activity). Future steps may
// split it further if a struct rebuild is needed for any of them.
func (s *SaveSlot) buildSectionMap() error {
	if len(s.Data) != SlotSize {
		return fmt.Errorf("buildSectionMap: slot.Data size %d, want %d", len(s.Data), SlotSize)
	}

	// Empty / unparseable slot — single covering section.
	if s.Version == 0 || s.UnlockedRegionsOffset == 0 {
		s.SectionMap = []SectionRange{{Name: SectionEmptySlot, Start: 0, End: SlotSize}}
		return nil
	}

	regionsStart := s.UnlockedRegionsOffset
	regionsEnd := regionsStart + 4 + 4*len(s.UnlockedRegions)

	if regionsStart < 0 || regionsStart > DlcSectionOffset {
		return fmt.Errorf("buildSectionMap: UnlockedRegionsOffset 0x%X outside valid range", regionsStart)
	}
	if regionsEnd > DlcSectionOffset {
		return fmt.Errorf("buildSectionMap: regions end 0x%X past DlcSectionOffset 0x%X",
			regionsEnd, DlcSectionOffset)
	}

	s.SectionMap = []SectionRange{
		{Name: SectionPreUnlockedRegs, Start: 0, End: regionsStart},
		{Name: SectionUnlockedRegs, Start: regionsStart, End: regionsEnd},
		{Name: SectionPostUnlockedRegs, Start: regionsEnd, End: DlcSectionOffset},
		{Name: SectionDLC, Start: DlcSectionOffset, End: DlcSectionOffset + DlcSectionSize},
		{Name: SectionHash, Start: HashOffset, End: HashOffset + HashSize},
	}
	return validateSectionMap(s.SectionMap)
}

// validateSectionMap checks that sections cover [0, SlotSize) contiguously,
// in ascending order, with no gaps and no overlaps.
func validateSectionMap(sections []SectionRange) error {
	if len(sections) == 0 {
		return fmt.Errorf("section map is empty")
	}
	if sections[0].Start != 0 {
		return fmt.Errorf("first section %q starts at 0x%X, want 0", sections[0].Name, sections[0].Start)
	}
	for i, sec := range sections {
		if sec.End <= sec.Start {
			return fmt.Errorf("section %q has non-positive size [0x%X, 0x%X)", sec.Name, sec.Start, sec.End)
		}
		if i > 0 && sec.Start != sections[i-1].End {
			return fmt.Errorf("gap or overlap between %q and %q (0x%X != 0x%X)",
				sections[i-1].Name, sec.Name, sections[i-1].End, sec.Start)
		}
	}
	last := sections[len(sections)-1]
	if last.End != SlotSize {
		return fmt.Errorf("last section %q ends at 0x%X, want SlotSize 0x%X", last.Name, last.End, SlotSize)
	}
	return nil
}

// RebuildSlot serializes a SaveSlot into a fresh 0x280000-byte buffer.
//
// Sequential rebuild strategy (Option B / R-1 final):
//  1. Copy bytes [0, UnlockedRegionsOffset) verbatim (pre-regions blob).
//  2. Reserialize `unlocked_regions` from slot.UnlockedRegions (count u32 +
//     N×u32). This is the only section whose size may change.
//  3. Re-parse every section after `unlocked_regions` from slot.Data
//     starting at the *original* regions end, then write each one back
//     via its typed Write method. Sections written: WorldHead, MenuSaveLoad,
//     TrophyEquipData, GaitemGameData, TutorialData, PreEventFlagsScalars,
//     EventFlagsBlock, WorldGeomBlock, PlayerCoordinates, SpawnPointBlock,
//     NetMan, TrailingFixedBlock (weather/time/base/steam/ps5/dlc),
//     PlayerGameDataHash.
//  4. Pad the remainder of the slot with zeros up to SlotSize. This tail
//     padding absorbs the unlocked_regions delta on saves that have slack
//     (PC saves observed with ~419KB rest; PS4 saves with 0).
//
// For an unmodified slot this produces byte-for-byte identical output to
// slot.Data. For a mutated UnlockedRegions slice it produces a save where
// every other section retains its original bytes, only the regions block
// shifts size, and the tail rest absorbs the delta.
//
// Reference: tmp/repos/er-save-manager/src/er_save_manager/parser/slot_rebuild.py
func RebuildSlot(slot *SaveSlot) ([]byte, error) {
	if slot == nil {
		return nil, fmt.Errorf("RebuildSlot: nil slot")
	}
	if len(slot.Data) != SlotSize {
		return nil, fmt.Errorf("RebuildSlot: slot.Data size %d, want %d", len(slot.Data), SlotSize)
	}

	// Empty / unparseable slot — verbatim copy.
	if slot.Version == 0 || slot.UnlockedRegionsOffset == 0 {
		out := make([]byte, SlotSize)
		copy(out, slot.Data)
		return out, nil
	}

	// Original regions end is captured in the SectionMap recorded at Read time.
	// We need it because slot.UnlockedRegions may have been mutated since then,
	// so the live `len(UnlockedRegions)` may not reflect the on-disk byte boundary.
	origRegsEnd := -1
	for _, sec := range slot.SectionMap {
		if sec.Name == SectionUnlockedRegs {
			origRegsEnd = sec.End
			break
		}
	}
	if origRegsEnd < 0 {
		return nil, fmt.Errorf("RebuildSlot: SectionMap missing unlocked_regions entry")
	}

	// Parse every post-region section from the original on-disk bytes.
	r := NewReader(slot.Data)
	if _, err := r.Seek(int64(origRegsEnd), 0); err != nil {
		return nil, fmt.Errorf("RebuildSlot: seek to regs end: %w", err)
	}
	var (
		head    WorldHead
		menu    MenuSaveLoad
		trophy  TrophyEquipData
		gaitem  GaitemGameData
		tut     TutorialData
		scalars PreEventFlagsScalars
		ef      EventFlagsBlock
		wgb     WorldGeomBlock
		pc      PlayerCoordinates
		sp      SpawnPointBlock
		nm      NetMan
		trail   TrailingFixedBlock
		hash    PlayerGameDataHash
	)
	if err := head.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := menu.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := trophy.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := gaitem.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := tut.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := scalars.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := ef.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := wgb.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := pc.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := sp.Read(r, slot.Version); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := nm.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := trail.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}
	if err := hash.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlot: %w", err)
	}

	// Capture the original tail rest verbatim so identity round-trip preserves
	// any non-zero garbage that may exist past the hash.
	tailStart := r.Pos()
	tailRest := slot.Data[tailStart:]

	// Build output buffer sequentially.
	sw := NewSectionWriter(SlotSize)
	sw.WriteBytes(slot.Data[:slot.UnlockedRegionsOffset])
	sw.WriteU32(uint32(len(slot.UnlockedRegions)))
	for _, id := range slot.UnlockedRegions {
		sw.WriteU32(id)
	}
	head.Write(sw)
	menu.Write(sw)
	trophy.Write(sw)
	gaitem.Write(sw)
	tut.Write(sw)
	scalars.Write(sw)
	ef.Write(sw)
	wgb.Write(sw)
	pc.Write(sw)
	sp.Write(sw)
	nm.Write(sw)
	trail.Write(sw)
	hash.Write(sw)

	// Append original tail rest, then zero-pad to SlotSize.
	written := sw.Len()
	tailToCopy := len(tailRest)
	if written+tailToCopy > SlotSize {
		// Mutation grew the variable section past available rest — trim tail.
		tailToCopy = SlotSize - written
		if tailToCopy < 0 {
			return nil, fmt.Errorf("RebuildSlot: sections overflow SlotSize (%d > %d)", written, SlotSize)
		}
	}
	if tailToCopy > 0 {
		sw.WriteBytes(tailRest[:tailToCopy])
	}
	if sw.Len() < SlotSize {
		sw.PadZeros(SlotSize - sw.Len())
	}

	out := sw.Bytes()
	if len(out) != SlotSize {
		return nil, fmt.Errorf("RebuildSlot: output size %d, want %d", len(out), SlotSize)
	}

	// Preserve DLC + Hash regions verbatim from original slot.Data (defense in
	// depth — same fix as RebuildSlotFull). When a region growth mutation
	// trimmed tailRest, original DLC/Hash bytes at fixed end-of-slot positions
	// would be lost; explicit verbatim copy restores them.
	copy(out[DlcSectionOffset:DlcSectionOffset+DlcSectionSize],
		slot.Data[DlcSectionOffset:DlcSectionOffset+DlcSectionSize])
	copy(out[SlotSize-HashSize:], slot.Data[SlotSize-HashSize:])

	return out, nil
}

// RebuildSlotFull rebuilds the slot from scratch, including a fresh GaItems
// section serialized from slot.GaItems. This replaces the in-place data shift
// in FlushGaItems which overwrote the last `delta` bytes of slot.Data — the
// DLC section (50 B) and PlayerGameDataHash (128 B) — whenever GaItems grew
// by more than 0x132 bytes.
//
// Layout produced (all written into a fresh SlotSize buffer):
//  1. Header  : slot.Data[0:GaItemsStart]                        (32 bytes)
//  2. GaItems : serialized slot.GaItems                          (variable)
//  3. PreRegs : slot.Data[oldGaLimit:slot.UnlockedRegionsOffset] (variable, verbatim)
//  4. Regions : count u32 + N×u32                                (variable)
//  5. PostRegs: WorldHead..Hash (typed Write sequence)           (~263 KB)
//  6. TailPad : zeros up to SlotSize
//
// DLC + PlayerGameDataHash are preserved because they're written via
// TrailingFixedBlock.Write / PlayerGameDataHash.Write from struct fields
// parsed at slot.Read() time. As long as those parsed values are intact in
// the in-memory SaveSlot, the rebuild restores them at the correct fixed
// end-of-slot positions.
//
// Caller responsibility: after calling this function and copying the result
// over slot.Data, the caller MUST refresh derived state via:
//
//	slot.calculateDynamicOffsets(); slot.mapInventory(); slot.buildSectionMap()
//
// Reference: tmp/repos/ER-Save-Editor/src/save/common/save_slot.rs (Rust
// reference uses a similar full rebuild on every mutation).
func RebuildSlotFull(slot *SaveSlot) ([]byte, error) {
	if slot == nil {
		return nil, fmt.Errorf("RebuildSlotFull: nil slot")
	}
	if len(slot.Data) != SlotSize {
		return nil, fmt.Errorf("RebuildSlotFull: slot.Data size %d, want %d", len(slot.Data), SlotSize)
	}

	// Empty / unparseable slot — verbatim copy.
	if slot.Version == 0 || slot.UnlockedRegionsOffset == 0 {
		out := make([]byte, SlotSize)
		copy(out, slot.Data)
		return out, nil
	}

	// Fail-closed layout refresh. Every dynamic boundary this function slices
	// against — oldGaLimit, preRegsBlob's end (UnlockedRegionsOffset), the
	// SectionMap regions boundary — hangs off slot.MagicOffset, which is a
	// *pattern match* into slot.Data recorded at Read time. A prior writer
	// (AddItemsToSlot, revealDLCMap, FlushGaItems, SetUnlockedRegions rollback,
	// …) can shift slot.Data without a reparse, leaving MagicOffset — and every
	// offset derived from it (UnlockedRegionsOffset, FaceDataOffset,
	// StorageBoxOffset, GaItemDataOffset, …) plus the SectionMap — stale by the
	// same shift delta. A heuristic that compares two of those cached offsets
	// (e.g. UnlockedRegionsOffset vs a MagicOffset-derived inventory end) cannot
	// detect this: both sides move together, so the comparison stays false while
	// the geometry is wrong. Instead rediscover MagicOffset from the current
	// bytes and recompute every dynamic offset and the section map from it,
	// erroring rather than rebuilding from stale or unparseable geometry.
	//
	// scanGaItems is deliberately NOT re-run: slot.GaItems holds the pending
	// mutation we are about to serialize, and re-scanning would clobber it.
	// calculateDynamicOffsets only re-reads the region list (which RebuildSlotFull
	// never mutates) from slot.Data, so the refresh is idempotent on fresh input.
	magic := NewReader(slot.Data).FindPattern(MagicPattern)
	if magic == -1 {
		magic = FallbackMagicBase
	}
	if magic < MinMagicOffset {
		return nil, fmt.Errorf("RebuildSlotFull: MagicOffset 0x%X below min 0x%X (unparseable layout)",
			magic, MinMagicOffset)
	}
	slot.MagicOffset = magic
	if err := slot.calculateDynamicOffsets(); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: refresh dynamic offsets: %w", err)
	}
	if err := slot.buildSectionMap(); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: refresh section map: %w", err)
	}

	// 1. Serialize new GaItems into a temp buffer. Max possible per-entry
	// size is GaRecordWeapon (21 bytes); the actual size depends on each
	// entry's ItemID type prefix.
	maxBuf := len(slot.GaItems) * GaRecordWeapon
	gaBuf := make([]byte, maxBuf)
	pos := 0
	for i := range slot.GaItems {
		pos += slot.GaItems[i].Serialize(gaBuf[pos:])
	}
	newGaBytes := gaBuf[:pos]

	// 2. Pre-regions blob: bytes between GaItems end and UnlockedRegions —
	// magic pattern, PlayerGameData, inventory, FaceData, storage, gestures.
	// Copied verbatim because these are mutated in-place by addToInventory
	// and not part of the typed-section rebuild path.
	oldGaLimit := slot.MagicOffset - DynPlayerData + 1
	if oldGaLimit < GaItemsStart {
		oldGaLimit = GaItemsStart
	}
	if slot.UnlockedRegionsOffset < oldGaLimit {
		return nil, fmt.Errorf("RebuildSlotFull: UnlockedRegionsOffset 0x%X < oldGaLimit 0x%X",
			slot.UnlockedRegionsOffset, oldGaLimit)
	}
	preRegsBlob := slot.Data[oldGaLimit:slot.UnlockedRegionsOffset]

	// 3. Parse all post-regions sections from the original on-disk bytes.
	// SectionMap captures the original regions-end boundary; using the live
	// len(UnlockedRegions) would give the wrong byte position if the slice
	// was mutated between Read() and rebuild.
	origRegsEnd := -1
	for _, sec := range slot.SectionMap {
		if sec.Name == SectionUnlockedRegs {
			origRegsEnd = sec.End
			break
		}
	}
	if origRegsEnd < 0 {
		return nil, fmt.Errorf("RebuildSlotFull: SectionMap missing unlocked_regions entry")
	}

	r := NewReader(slot.Data)
	if _, err := r.Seek(int64(origRegsEnd), 0); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: seek to regs end: %w", err)
	}
	var (
		head    WorldHead
		menu    MenuSaveLoad
		trophy  TrophyEquipData
		gaitem  GaitemGameData
		tut     TutorialData
		scalars PreEventFlagsScalars
		ef      EventFlagsBlock
		wgb     WorldGeomBlock
		pc      PlayerCoordinates
		sp      SpawnPointBlock
		nm      NetMan
		trail   TrailingFixedBlock
		hash    PlayerGameDataHash
	)
	if err := head.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: head: %w", err)
	}
	if err := menu.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: menu: %w", err)
	}
	if err := trophy.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: trophy: %w", err)
	}
	if err := gaitem.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: gaitem: %w", err)
	}
	if err := tut.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: tut: %w", err)
	}
	if err := scalars.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: scalars: %w", err)
	}
	if err := ef.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: ef: %w", err)
	}
	if err := wgb.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: wgb: %w", err)
	}
	if err := pc.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: pc: %w", err)
	}
	if err := sp.Read(r, slot.Version); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: sp: %w", err)
	}
	if err := nm.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: nm: %w", err)
	}
	if err := trail.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: trail: %w", err)
	}
	if err := hash.Read(r); err != nil {
		return nil, fmt.Errorf("RebuildSlotFull: hash: %w", err)
	}
	tailRest := slot.Data[r.Pos():]

	// 4. Build output buffer sequentially.
	sw := NewSectionWriter(SlotSize)
	sw.WriteBytes(slot.Data[:GaItemsStart])
	sw.WriteBytes(newGaBytes)
	sw.WriteBytes(preRegsBlob)
	sw.WriteU32(uint32(len(slot.UnlockedRegions)))
	for _, id := range slot.UnlockedRegions {
		sw.WriteU32(id)
	}
	head.Write(sw)
	menu.Write(sw)
	trophy.Write(sw)
	gaitem.Write(sw)
	tut.Write(sw)
	scalars.Write(sw)
	ef.Write(sw)
	wgb.Write(sw)
	pc.Write(sw)
	sp.Write(sw)
	nm.Write(sw)
	trail.Write(sw)
	hash.Write(sw)

	written := sw.Len()
	if written > SlotSize {
		return nil, fmt.Errorf("RebuildSlotFull: sections overflow SlotSize (%d > %d)", written, SlotSize)
	}

	// Append tail rest, then zero-pad to SlotSize.
	tailToCopy := len(tailRest)
	if written+tailToCopy > SlotSize {
		tailToCopy = SlotSize - written
		if tailToCopy < 0 {
			tailToCopy = 0
		}
	}
	if tailToCopy > 0 {
		sw.WriteBytes(tailRest[:tailToCopy])
	}
	if sw.Len() < SlotSize {
		sw.PadZeros(SlotSize - sw.Len())
	}

	out := sw.Bytes()
	if len(out) != SlotSize {
		return nil, fmt.Errorf("RebuildSlotFull: output size %d, want %d", len(out), SlotSize)
	}

	// IMPORTANT: typed trail.Read/hash.Read parse bytes at a position INSIDE
	// the typed-section span (around SlotSize - 419KB), but the game reads DLC
	// at fixed offset SlotSize-0xB2 and Hash at SlotSize-0x80 — both INSIDE
	// the tailRest region that we just wrote and possibly truncated. When
	// GaItems grew (delta > 0), tailRest was trimmed from the END to fit
	// SlotSize, dropping the original DLC+Hash bytes. Restore them verbatim
	// from slot.Data here so the game finds correct values at fixed offsets.
	copy(out[DlcSectionOffset:DlcSectionOffset+DlcSectionSize],
		slot.Data[DlcSectionOffset:DlcSectionOffset+DlcSectionSize])
	copy(out[SlotSize-HashSize:], slot.Data[SlotSize-HashSize:])

	return out, nil
}
