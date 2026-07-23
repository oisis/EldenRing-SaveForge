package core

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// MagicPattern matches the 192-byte pattern used in the Python editor for reliability.
// First block: 0x00 + 0xFFFFFFFF + 12 zeros (17 bytes)
// Subsequent blocks: 0xFFFFFFFF + 12 zeros (16 bytes each)
var MagicPattern = []byte{
	0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

const (
	ItemTypeWeapon    = 0x80000000
	ItemTypeArmor     = 0x90000000
	ItemTypeAccessory = 0xA0000000
	ItemTypeItem      = 0xB0000000
	ItemTypeAow       = 0xC0000000
)

// AoWGaItemHandle sentinel values for "no custom Ash of War attached".
//
// Forensic comparison of in-game vanilla saves (ER0000-kro55-vanilla.sl2 and
// fresh fields of ER0000.sl2) versus SaveForge-bulk-edited saves shows the
// game writes 0x00000000 for weapons that have no external AoW gem
// attached; SaveForge historically wrote 0xFFFFFFFF. The game tolerates
// both values, but vanilla-aligned output minimizes anti-cheat/validation
// risk so the writer canonicalizes to NoCustomAoWHandle.
const (
	// NoCustomAoWHandle is the canonical value emitted by writers when a
	// weapon GaItem has no custom Ash of War attached. Matches the value
	// the in-game save writes.
	NoCustomAoWHandle uint32 = 0x00000000

	// LegacyNoCustomAoWHandle is the value historical SaveForge releases
	// (and the GaItemFull zero-value placeholder for empty slots) wrote
	// for the same semantic state. Readers must continue to recognize it
	// so previously edited saves keep working.
	LegacyNoCustomAoWHandle uint32 = 0xFFFFFFFF
)

// IsNoCustomAoWHandle reports whether the given AoWGaItemHandle field
// value means "no external Ash of War attached" — covering both the
// canonical vanilla sentinel (0x00000000) and the legacy SaveForge
// sentinel (0xFFFFFFFF). Use this in every reader and availability scan;
// any check that compares against a single sentinel will misclassify
// half of the save population.
func IsNoCustomAoWHandle(h uint32) bool {
	return h == NoCustomAoWHandle || h == LegacyNoCustomAoWHandle
}

type GaItem struct {
	Handle uint32
	ItemID uint32
}

// GaItemFull represents a complete GaItem entry with all variable-length fields.
// Size depends on item_id type: weapon=21B, armor=16B, everything else=8B.
// Matches Rust ER-Save-Editor save_slot.rs GaItem struct.
type GaItemFull struct {
	Handle          uint32
	ItemID          uint32
	Unk2            int32  // weapon/armor (incl. arrows/bolts): native default 0 (T020, T063/T211)
	Unk3            int32  // weapon/armor (incl. arrows/bolts): native default 0 (T020, T063/T211)
	AoWGaItemHandle uint32 // weapon only: NoCustomAoWHandle when no AoW attached (see IsNoCustomAoWHandle for compat)
	Unk5            uint8  // weapon only: default 0
}

// GaItemRecordSize returns the byte size of a GaItem record based on item_id.
// Uses item_id (not handle) for size determination, matching Rust ER-Save-Editor.
func GaItemRecordSize(itemID uint32) int {
	if itemID == 0 || itemID == GaHandleInvalid {
		return GaRecordItem // 8
	}
	switch itemID & 0xF0000000 {
	case 0x00000000:
		return GaRecordWeapon // 21
	case 0x10000000:
		return GaRecordArmor // 16
	default:
		return GaRecordItem // 8
	}
}

// ByteSize returns the serialized byte size of this entry.
func (g *GaItemFull) ByteSize() int {
	return GaItemRecordSize(g.ItemID)
}

// IsEmpty returns true if this GaItem slot is unused.
func (g *GaItemFull) IsEmpty() bool {
	return g.ItemID == 0 || g.ItemID == GaHandleInvalid
}

// Serialize writes the GaItem entry into buf and returns bytes written.
// buf must have at least g.ByteSize() bytes available.
func (g *GaItemFull) Serialize(buf []byte) int {
	if g.IsEmpty() {
		// Empty slots always persist the canonical native marker (handle=0,
		// itemID=0xFFFFFFFF), regardless of how the caller zeroed the entry.
		binary.LittleEndian.PutUint32(buf[0:], 0)
		binary.LittleEndian.PutUint32(buf[4:], GaHandleInvalid)
		return GaRecordItem
	}
	binary.LittleEndian.PutUint32(buf[0:], g.Handle)
	binary.LittleEndian.PutUint32(buf[4:], g.ItemID)
	sz := g.ByteSize()
	if sz >= GaRecordArmor {
		binary.LittleEndian.PutUint32(buf[8:], uint32(g.Unk2))
		binary.LittleEndian.PutUint32(buf[12:], uint32(g.Unk3))
	}
	if sz >= GaRecordWeapon {
		binary.LittleEndian.PutUint32(buf[16:], g.AoWGaItemHandle)
		buf[20] = g.Unk5
	}
	return sz
}

type InventoryItem struct {
	GaItemHandle uint32
	Quantity     uint32
	Index        uint32
}

type EquipInventoryData struct {
	CommonItems           []InventoryItem
	KeyItems              []InventoryItem
	NextEquipIndex        uint32
	NextAcquisitionSortId uint32
	nextEquipIndexOff     int // absolute byte offset in slot.Data (for write-back)
	nextAcqSortIdOff      int // absolute byte offset in slot.Data (for write-back)
}

// NextEquipIndexOff returns the absolute byte offset of NextEquipIndex in slot.Data.
// Used by tests to exclude intentionally-corrected bytes from round-trip comparison.
func (e *EquipInventoryData) NextEquipIndexOff() int { return e.nextEquipIndexOff }

// Clone returns a deep copy of EquipInventoryData, including unexported offset fields.
func (e *EquipInventoryData) Clone() EquipInventoryData {
	c := EquipInventoryData{
		NextEquipIndex:        e.NextEquipIndex,
		NextAcquisitionSortId: e.NextAcquisitionSortId,
		nextEquipIndexOff:     e.nextEquipIndexOff,
		nextAcqSortIdOff:      e.nextAcqSortIdOff,
	}
	if e.CommonItems != nil {
		c.CommonItems = make([]InventoryItem, len(e.CommonItems))
		copy(c.CommonItems, e.CommonItems)
	}
	if e.KeyItems != nil {
		c.KeyItems = make([]InventoryItem, len(e.KeyItems))
		copy(c.KeyItems, e.KeyItems)
	}
	return c
}

func (e *EquipInventoryData) Read(r *Reader, commonCount, keyCount int) error {
	e.CommonItems = make([]InventoryItem, commonCount)
	var err error
	for i := 0; i < commonCount; i++ {
		if e.CommonItems[i].GaItemHandle, err = r.ReadU32(); err != nil {
			return fmt.Errorf("common[%d].handle: %w", i, err)
		}
		if e.CommonItems[i].Quantity, err = r.ReadU32(); err != nil {
			return fmt.Errorf("common[%d].quantity: %w", i, err)
		}
		if e.CommonItems[i].Index, err = r.ReadU32(); err != nil {
			return fmt.Errorf("common[%d].index: %w", i, err)
		}
	}
	if _, err = r.ReadU32(); err != nil { // skip key_count header
		return fmt.Errorf("key_count header: %w", err)
	}
	e.KeyItems = make([]InventoryItem, keyCount)
	for i := 0; i < keyCount; i++ {
		if e.KeyItems[i].GaItemHandle, err = r.ReadU32(); err != nil {
			return fmt.Errorf("key[%d].handle: %w", i, err)
		}
		if e.KeyItems[i].Quantity, err = r.ReadU32(); err != nil {
			return fmt.Errorf("key[%d].quantity: %w", i, err)
		}
		if e.KeyItems[i].Index, err = r.ReadU32(); err != nil {
			return fmt.Errorf("key[%d].index: %w", i, err)
		}
	}
	// Trailing counters — record offsets for write-back
	e.nextEquipIndexOff = r.Pos()
	if e.NextEquipIndex, err = r.ReadU32(); err != nil {
		return fmt.Errorf("NextEquipIndex: %w", err)
	}
	e.nextAcqSortIdOff = r.Pos()
	if e.NextAcquisitionSortId, err = r.ReadU32(); err != nil {
		return fmt.Errorf("NextAcquisitionSortId: %w", err)
	}
	return nil
}

type PlayerGameData struct {
	Level               uint32
	Vigor               uint32
	Mind                uint32
	Endurance           uint32
	Strength            uint32
	Dexterity           uint32
	Intelligence        uint32
	Faith               uint32
	Arcane              uint32
	Souls               uint32
	SoulMemory          uint32
	CharacterName       [16]uint16
	Gender              uint8
	VoiceType           uint8 // 0=Young1, 1=Young2, 2=Mature1, 3=Mature2, 4=Aged1, 5=Aged2
	Class               uint8
	TalismanSlots       uint8  // additional talisman slots (0-3), total = 1 + this value
	ClearCount          uint32 // NG+ cycle (0=NG, 1=NG+1, ..., 7=NG+7)
	GreatRuneOn         uint8  // Great Rune buff active (0=off, 1=on)
	EquippedGreatRune   uint32 // equipped Great Rune item ID (0=none)
	ScadutreeBlessing   uint8
	ShadowRealmBlessing uint8
}

type SaveSlot struct {
	Data      []byte
	Version   uint32 // slot format version (offset 0x00); 0 = empty slot
	Player    PlayerGameData
	GaMap     map[uint32]uint32
	GaItems   []GaItemFull // parsed GaItem array (5118 or 5120 entries)
	Inventory EquipInventoryData
	Storage   EquipInventoryData
	SteamID   uint64
	Warnings  []string // non-fatal issues detected during parsing

	MagicOffset      int
	InventoryEnd     int
	EventFlagsOffset int

	// Dynamic offsets from Python logic
	PlayerDataOffset      int
	FaceDataOffset        int
	StorageBoxOffset      int
	IngameTimerOffset     int
	GaItemDataOffset      int      // start of GaItemData section (distinct_acquired_items_count header)
	TutorialDataOffset    int      // start of TutorialData block (header at offset, per er-save-manager world.py)
	ClearCountOffset      int      // NG+ cycle counter (uint32) — after BloodStain in dynamic chain
	EquipItemsIDOffset    int      // start of EquippedItemsItemIds section
	EquippedSpellsOffset  int      // start of EquippedSpells section (14×8 + 4 active_index = 0x74)
	UnlockedRegionsOffset int      // start of unlocked_regions struct (count u32 + count*4 IDs)
	UnlockedRegions       []uint32 // parsed unlocked region IDs (drives invasion / blue-summon eligibility)

	// SectionMap holds the section boundaries used by RebuildSlot. Populated
	// after parsing finishes (or in a degraded form for empty / unparseable slots).
	// Sections cover [0, SlotSize) contiguously with no gaps or overlaps.
	SectionMap []SectionRange

	// Tracked indices for type-segregated GaItem placement.
	// The game expects AoW entries at low indices, then armor, then weapons.
	// Matching Rust ER-Save-Editor's next_aow_index / next_armament_or_armor_index.
	NextAoWIndex      int    // next free index for AoW entries (after last AoW + 1)
	NextArmamentIndex int    // next free index for weapon/armor entries (after highest-counter entry + 1)
	NextGaItemHandle  uint32 // global handle counter (lower 16 bits), next value to assign
	PartGaItemHandle  uint8  // part_id (bits 16-23 of handle), extracted from first entry
}

func (s *SaveSlot) Read(r *Reader, platform string) error {
	var err error
	s.Data, err = r.ReadBytes(SlotSize)
	if err != nil {
		return err
	}
	return s.parseFromData()
}

// parseFromData populates SaveSlot fields from s.Data. Used by Read() after
// reading slot bytes, and by AddItemsToSlot after RebuildSlotFull replaces
// s.Data with a re-serialized buffer (where MagicPattern position, GaItems
// boundary, and all dynamic offsets have shifted relative to the previous
// state).
//
// Steps mirror the original Read() body 1:1; the only difference is that
// s.Data is assumed to already hold SlotSize bytes — no ReadBytes call.
func (s *SaveSlot) parseFromData() error {
	// 0. Read slot version (offset 0x00). Version 0 = empty/unused slot.
	s.Version = binary.LittleEndian.Uint32(s.Data[0:4])

	// 1. Find primary anchor
	s.MagicOffset = NewReader(s.Data).FindPattern(MagicPattern)
	if s.MagicOffset == -1 {
		s.MagicOffset = FallbackMagicBase
		s.Warnings = append(s.Warnings,
			fmt.Sprintf("MagicPattern not found, using fallback offset 0x%X", FallbackMagicBase))
	}
	if s.MagicOffset < MinMagicOffset {
		return fmt.Errorf("MagicOffset %d (0x%X) too small (min %d)",
			s.MagicOffset, s.MagicOffset, MinMagicOffset)
	}

	// 2. Read stats
	if err := s.mapStats(); err != nil {
		return fmt.Errorf("mapStats: %w", err)
	}

	// 3. Scan GaItems
	s.scanGaItems(GaItemsStart)

	// 4. Calculate dynamic offsets
	if err := s.calculateDynamicOffsets(); err != nil {
		return fmt.Errorf("dynamic offsets: %w", err)
	}

	// 5. Cross-validate offset chain
	if err := s.validateOffsetChain(); err != nil {
		return fmt.Errorf("offset validation: %w", err)
	}

	// 6. Read ClearCount (NG+ cycle) from dynamic chain offset
	if s.ClearCountOffset > 0 && s.ClearCountOffset+4 <= len(s.Data) {
		s.Player.ClearCount = binary.LittleEndian.Uint32(s.Data[s.ClearCountOffset:])
		if s.Player.ClearCount > 7 {
			s.Player.ClearCount = 7
		}
	}

	// 6b. Read equipped Great Rune from EquippedItemsItemIds+0x28
	grOff := s.EquipItemsIDOffset + DynEquipGreatRune
	if s.EquipItemsIDOffset > 0 && grOff+4 <= len(s.Data) {
		s.Player.EquippedGreatRune = binary.LittleEndian.Uint32(s.Data[grOff:])
	}

	// 7. Map inventory
	if err := s.mapInventory(); err != nil {
		return fmt.Errorf("mapInventory: %w", err)
	}

	// NOTE: Per-slot SteamID is at a dynamic offset within the sequential parsing chain
	// (after BaseVersion, before PS5Activity), NOT at the fixed SlotSize-8 address.
	// SlotSize-8 falls inside the PlayerGameDataHash region. The authoritative SteamID
	// is read from UserData10 by the save_manager and propagated to slots from there.

	// 8. Build section map for RebuildSlot. Failure here is non-fatal — we surface
	// it as a warning so editing of fixed-offset fields still works.
	if err := s.buildSectionMap(); err != nil {
		s.Warnings = append(s.Warnings, "buildSectionMap: "+err.Error())
	}
	return nil
}

func (s *SaveSlot) calculateDynamicOffsets() error {
	sa := NewSlotAccessor(s.Data)

	s.PlayerDataOffset = s.MagicOffset

	spEffect := s.PlayerDataOffset + DynSpEffect
	equipedItemIndex := spEffect + DynEquipedItemIndex
	activeEquipedItems := equipedItemIndex + DynActiveEquipedItems
	equipedItemsID := activeEquipedItems + DynEquipedItemsID
	s.EquipItemsIDOffset = equipedItemsID
	activeEquipedItemsGa := equipedItemsID + DynActiveEquipedItemsGa
	// EquippedSpells starts immediately at the end of inventory_held (no gap).
	// Section is 0x74 = 116 bytes: 14 × 8B per-slot (spell_id u32, unk u32) + 4B active index.
	// Live save verification (Phase 7d.0 audit): raw MagicParam spell_ids appear here.
	spellsOff := activeEquipedItemsGa + DynInventoryHeld
	s.EquippedSpellsOffset = spellsOff
	equipedItemsOff := spellsOff + DynEquipedSpells          // start of EquippedItems (ChrAsm + QuickSlots + Pouch)
	equipedGesturesOff := equipedItemsOff + DynEquipedItems  // start of EquippedGestures
	projHeaderOff := equipedGesturesOff + DynEquipedGestures // start of AcquiredProjectiles count u32

	// Dynamic field #1: acquired_projectiles.
	// The u32 at projHeaderOff is the projectile COUNT. Total skip = count*8 + 4.
	// Source: Final.py:1453 — equiped_projc_size * 8 + 4
	// Without this skip, ALL subsequent offsets (FaceData, StorageBox, GaItemData,
	// EventFlags) are shifted left by projCount*8 bytes, causing storage writes
	// to land at the wrong position and become invisible to the game.
	if err := sa.CheckBounds(projHeaderOff, 4, "projHeader"); err != nil {
		return err
	}
	projCount, err := sa.ReadDynamicSize(projHeaderOff, MaxProjCount, "projCount")
	if err != nil {
		return err
	}
	equipedProjectile := projHeaderOff + projCount*8 + 4

	equipedArmaments := equipedProjectile + DynEquipedArmaments
	equipePhysics := equipedArmaments + DynEquipePhysics
	s.FaceDataOffset = equipePhysics + DynFaceData
	// StorageBoxOffset = start of storage section (EquipInventoryData #2).
	// Rust ER-Save-Editor reads storage immediately after face_data (0x12F bytes).
	// DynStorageBox (0x6010) is the SIZE of the storage section, not a gap.
	s.StorageBoxOffset = s.FaceDataOffset
	storageEnd := s.StorageBoxOffset + DynStorageBox

	// EventFlags offset chain
	gesturesOff := storageEnd + DynStorageToGestures
	if err := sa.CheckBounds(gesturesOff, 4, "gesturesOff"); err != nil {
		s.Warnings = append(s.Warnings, "EventFlags chain unreachable: "+err.Error())
		s.Warnings = append(s.Warnings, sa.Warnings...)
		return nil // non-fatal — event flags are optional for basic editing
	}

	// Dynamic field #2: unlocked_regions.
	// The u32 at gesturesOff is the region COUNT. Total skip = count*4 + 4.
	// Source: Final.py:1462 — unlocked_region_size * 4 + 4
	if err := sa.CheckBounds(gesturesOff, 4, "unlockedRegHeader"); err != nil {
		return err
	}
	regCount, err := sa.ReadDynamicSize(gesturesOff, MaxUnlockedRegCnt, "regCount")
	if err != nil {
		return err
	}
	s.UnlockedRegionsOffset = gesturesOff
	s.UnlockedRegions = make([]uint32, regCount)
	for i := 0; i < regCount; i++ {
		v, rerr := sa.ReadU32(gesturesOff + 4 + i*4)
		if rerr != nil {
			return rerr
		}
		s.UnlockedRegions[i] = v
	}
	unlockedRegion := gesturesOff + regCount*4 + 4

	horse := unlockedRegion + DynHorse
	s.ClearCountOffset = horse + DynClearCount
	bloodStain := horse + DynBloodStain
	menuProfile := bloodStain + DynMenuProfile
	s.GaItemDataOffset = menuProfile // GaitemGameData starts here (i64 count + 7000×16-byte entries)
	gaItemsOther := menuProfile + DynGaItemsOther
	tutorialData := gaItemsOther + DynTutorialData
	s.TutorialDataOffset = gaItemsOther // TutorialData block starts at end of GaitemGameData
	s.IngameTimerOffset = tutorialData + DynIngameTimer
	s.EventFlagsOffset = s.IngameTimerOffset + DynEventFlags

	// Collect SlotAccessor warnings
	s.Warnings = append(s.Warnings, sa.Warnings...)
	return nil
}

// validateOffsetChain verifies that all computed offsets are within bounds
// and in the expected monotonic order. Called after calculateDynamicOffsets().
func (s *SaveSlot) validateOffsetChain() error {
	type check struct {
		name   string
		offset int
		minVal int
		maxVal int
	}

	checks := []check{
		{"MagicOffset", s.MagicOffset, MinMagicOffset, SlotSize},
		{"InventoryEnd", s.InventoryEnd, GaItemsStart, s.MagicOffset - DynPlayerData + 2},
		{"PlayerDataOffset", s.PlayerDataOffset, s.MagicOffset, SlotSize},
		{"FaceDataOffset", s.FaceDataOffset, s.PlayerDataOffset, SlotSize},
		{"StorageBoxOffset", s.StorageBoxOffset, s.FaceDataOffset, SlotSize},
	}

	for _, c := range checks {
		if c.offset < c.minVal || c.offset >= c.maxVal {
			return fmt.Errorf("offset %s = 0x%X out of expected range [0x%X, 0x%X)",
				c.name, c.offset, c.minVal, c.maxVal)
		}
	}

	// Monotonicity: offsets MUST be strictly increasing in this order.
	// StorageBoxOffset == FaceDataOffset (storage starts at face data end).
	if !(s.InventoryEnd <= s.MagicOffset &&
		s.MagicOffset <= s.PlayerDataOffset &&
		s.PlayerDataOffset < s.FaceDataOffset &&
		s.FaceDataOffset <= s.StorageBoxOffset) {
		return fmt.Errorf("offset chain order violated: "+
			"InventoryEnd=0x%X MagicOffset=0x%X PlayerData=0x%X FaceData=0x%X StorageBox=0x%X",
			s.InventoryEnd, s.MagicOffset, s.PlayerDataOffset,
			s.FaceDataOffset, s.StorageBoxOffset)
	}

	// EventFlagsOffset is optional (may be 0 if chain was unreachable)
	if s.EventFlagsOffset > 0 && s.EventFlagsOffset >= SlotSize {
		s.Warnings = append(s.Warnings,
			fmt.Sprintf("EventFlagsOffset 0x%X >= SlotSize, event flags disabled",
				s.EventFlagsOffset))
		s.EventFlagsOffset = 0
	}

	return nil
}

// ValidateSlotIntegrity performs write-ahead validation on a slot before saving.
// It re-checks the offset chain, inventory bounds, data length and stat sanity
// to prevent writing a corrupted save file.
func ValidateSlotIntegrity(slot *SaveSlot) error {
	// 1. Data length must equal SlotSize
	if len(slot.Data) != SlotSize {
		return fmt.Errorf("slot data length %d (0x%X) != expected SlotSize %d (0x%X)",
			len(slot.Data), len(slot.Data), SlotSize, SlotSize)
	}

	// 2. Offset chain re-validation
	if err := slot.validateOffsetChain(); err != nil {
		return fmt.Errorf("offset chain invalid: %w", err)
	}

	// 3. Inventory bounds: invStart and storageStart must be within slot.Data
	invStart := slot.MagicOffset + InvStartFromMagic
	if invStart < 0 || invStart >= SlotSize {
		return fmt.Errorf("inventory start offset 0x%X out of bounds [0, 0x%X)",
			invStart, SlotSize)
	}
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	if storageStart < 0 || storageStart >= SlotSize {
		return fmt.Errorf("storage start offset 0x%X out of bounds [0, 0x%X)",
			storageStart, SlotSize)
	}

	// 4. Stat sanity: Level must be > 0, attributes 1–99
	if slot.Player.Level == 0 || slot.Player.Level > 713 {
		return fmt.Errorf("Level %d out of valid range [1, 713]", slot.Player.Level)
	}
	attrs := []struct {
		name string
		val  uint32
	}{
		{"Vigor", slot.Player.Vigor},
		{"Mind", slot.Player.Mind},
		{"Endurance", slot.Player.Endurance},
		{"Strength", slot.Player.Strength},
		{"Dexterity", slot.Player.Dexterity},
		{"Intelligence", slot.Player.Intelligence},
		{"Faith", slot.Player.Faith},
		{"Arcane", slot.Player.Arcane},
	}
	for _, a := range attrs {
		if a.val < 1 || a.val > 99 {
			return fmt.Errorf("%s = %d out of valid range [1, 99]", a.name, a.val)
		}
	}

	return nil
}

// FaceDataStart returns the byte offset in slot.Data where the FaceData blob begins.
// FaceData is FaceDataBlobSize (303) bytes ending at FaceDataOffset.
func (s *SaveSlot) FaceDataStart() int {
	return s.FaceDataOffset - FaceDataBlobSize
}

func (s *SaveSlot) mapStats() error {
	sa := NewSlotAccessor(s.Data)
	mo := s.MagicOffset
	var err error

	if s.Player.Level, err = sa.ReadU32(mo + OffLevel); err != nil {
		return fmt.Errorf("Level: %w", err)
	}
	if s.Player.Vigor, err = sa.ReadU32(mo + OffVigor); err != nil {
		return fmt.Errorf("Vigor: %w", err)
	}
	if s.Player.Mind, err = sa.ReadU32(mo + OffMind); err != nil {
		return fmt.Errorf("Mind: %w", err)
	}
	if s.Player.Endurance, err = sa.ReadU32(mo + OffEndurance); err != nil {
		return fmt.Errorf("Endurance: %w", err)
	}
	if s.Player.Strength, err = sa.ReadU32(mo + OffStrength); err != nil {
		return fmt.Errorf("Strength: %w", err)
	}
	if s.Player.Dexterity, err = sa.ReadU32(mo + OffDexterity); err != nil {
		return fmt.Errorf("Dexterity: %w", err)
	}
	if s.Player.Intelligence, err = sa.ReadU32(mo + OffIntelligence); err != nil {
		return fmt.Errorf("Intelligence: %w", err)
	}
	if s.Player.Faith, err = sa.ReadU32(mo + OffFaith); err != nil {
		return fmt.Errorf("Faith: %w", err)
	}
	if s.Player.Arcane, err = sa.ReadU32(mo + OffArcane); err != nil {
		return fmt.Errorf("Arcane: %w", err)
	}
	if s.Player.Souls, err = sa.ReadU32(mo + OffSouls); err != nil {
		return fmt.Errorf("Souls: %w", err)
	}
	if s.Player.SoulMemory, err = sa.ReadU32(mo + OffSoulMemory); err != nil {
		return fmt.Errorf("SoulMemory: %w", err)
	}
	if s.Player.Gender, err = sa.ReadU8(mo + OffGender); err != nil {
		return fmt.Errorf("Gender: %w", err)
	}
	if s.Player.VoiceType, err = sa.ReadU8(mo + OffVoiceType); err != nil {
		return fmt.Errorf("VoiceType: %w", err)
	}
	if s.Player.Class, err = sa.ReadU8(mo + OffClass); err != nil {
		return fmt.Errorf("Class: %w", err)
	}
	if s.Player.TalismanSlots, err = sa.ReadU8(mo + OffTalismanSlots); err != nil {
		return fmt.Errorf("TalismanSlots: %w", err)
	}
	if s.Player.TalismanSlots > 3 {
		s.Player.TalismanSlots = 3
	}
	if s.Player.GreatRuneOn, err = sa.ReadU8(mo + OffGreatRuneOn); err != nil {
		return fmt.Errorf("GreatRuneOn: %w", err)
	}
	if s.Player.ScadutreeBlessing, err = sa.ReadU8(mo + OffScadutreeBlessing); err != nil {
		return fmt.Errorf("ScadutreeBlessing: %w", err)
	}
	if s.Player.ShadowRealmBlessing, err = sa.ReadU8(mo + OffShadowRealmBlessing); err != nil {
		return fmt.Errorf("ShadowRealmBlessing: %w", err)
	}

	nameOff := mo + OffCharacterName
	for i := 0; i < 16; i++ {
		val, err := sa.ReadU16(nameOff + i*2)
		if err != nil {
			return fmt.Errorf("CharacterName[%d]: %w", i, err)
		}
		s.Player.CharacterName[i] = val
	}

	return nil
}

// scanGaItems parses ALL GaItem entries (5118 or 5120) into the GaItems array.
// Uses item_id for record size determination, matching Rust ER-Save-Editor.
// Builds GaMap (handle→itemID) for non-empty entries.
func (s *SaveSlot) scanGaItems(start int) {
	s.GaMap = make(map[uint32]uint32)

	gaLimit := s.MagicOffset - DynPlayerData + 1
	if gaLimit < start {
		gaLimit = start
	}

	maxEntries := GaItemCountNew
	if s.Version > 0 && s.Version <= GaItemVersionBreak {
		maxEntries = GaItemCountOld
	}

	s.GaItems = make([]GaItemFull, maxEntries)
	curr := start

	for i := 0; i < maxEntries; i++ {
		if curr+GaRecordItem > gaLimit {
			// Remaining entries are empty (no bytes left in section).
			s.GaItems[i] = GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
			continue
		}

		handle := binary.LittleEndian.Uint32(s.Data[curr:])
		itemID := binary.LittleEndian.Uint32(s.Data[curr+4:])

		entry := GaItemFull{
			Handle:          handle,
			ItemID:          itemID,
			Unk2:            -1,
			Unk3:            -1,
			AoWGaItemHandle: 0xFFFFFFFF,
			Unk5:            0,
		}

		// Determine record size from item_id (matching Rust ER-Save-Editor read logic).
		recSize := GaItemRecordSize(itemID)

		// Read extended fields for weapons (21B) and armor (16B).
		if recSize >= GaRecordArmor && curr+GaRecordArmor <= len(s.Data) {
			entry.Unk2 = int32(binary.LittleEndian.Uint32(s.Data[curr+8:]))
			entry.Unk3 = int32(binary.LittleEndian.Uint32(s.Data[curr+12:]))
		}
		if recSize >= GaRecordWeapon && curr+GaRecordWeapon <= len(s.Data) {
			entry.AoWGaItemHandle = binary.LittleEndian.Uint32(s.Data[curr+16:])
			entry.Unk5 = s.Data[curr+20]
		}

		s.GaItems[i] = entry
		curr += recSize

		// Build GaMap for non-empty entries.
		if !entry.IsEmpty() {
			typeBits := handle & GaHandleTypeMask
			switch typeBits {
			case ItemTypeWeapon, ItemTypeArmor, ItemTypeAccessory, ItemTypeItem, ItemTypeAow:
				s.GaMap[handle] = itemID
			}
		}
	}

	// InventoryEnd = byte offset after all parsed entries (= section end).
	s.InventoryEnd = curr

	// Legacy public cursor fields remain derived metadata for compatibility;
	// allocation itself uses the native physical-index projection.
	refreshGaItemTracking(s)
}

func (e *EquipInventoryData) ReadStorage(r *Reader, count int) error {
	e.CommonItems = []InventoryItem{}
	for i := 0; i < count; i++ {
		handle, err := r.ReadU32()
		if err != nil {
			return fmt.Errorf("storage[%d].handle: %w", i, err)
		}
		quantity, err := r.ReadU32()
		if err != nil {
			return fmt.Errorf("storage[%d].quantity: %w", i, err)
		}
		index, err := r.ReadU32()
		if err != nil {
			return fmt.Errorf("storage[%d].index: %w", i, err)
		}

		// Skip empty/invalid entries but continue reading — storage can have sparse gaps
		// after item removal. Breaking on first empty would lose items after the gap.
		if handle == GaHandleEmpty || handle == GaHandleInvalid {
			continue
		}

		e.CommonItems = append(e.CommonItems, InventoryItem{
			GaItemHandle: handle,
			Quantity:     quantity,
			Index:        index,
		})
	}
	e.KeyItems = []InventoryItem{}
	return nil
}

func (s *SaveSlot) mapInventory() error {
	// Main Inventory
	invStart := s.MagicOffset + InvStartFromMagic
	if invStart+InvSafetyMargin < len(s.Data) {
		ir := NewReader(s.Data)
		ir.Seek(int64(invStart), 0)
		if err := s.Inventory.Read(ir, CommonItemCount, KeyItemCount); err != nil {
			return fmt.Errorf("inventory read: %w", err)
		}
	}

	// Reconcile NextAcquisitionSortId: must be strictly greater than all existing
	// item indices. Saves edited by external tools (er-save-manager, our old
	// max-based writer) may have items with acquisition indices above the stored
	// NextAcquisitionSortId value. Reconcile in-memory so new additions never
	// collide with existing sort IDs. This does not write to binary unless the
	// caller also adds items (addToInventory writes the counter back only then).
	{
		maxIdx := uint32(0)
		for _, item := range s.Inventory.CommonItems {
			if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid && item.Index > maxIdx {
				maxIdx = item.Index
			}
		}
		// ponytail: also scan KeyItems — game-placed key items may have Index >= NextAcquisitionSortId
		// causing new CommonItems additions to collide with their indices (Bug #2).
		for _, item := range s.Inventory.KeyItems {
			if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid && item.Index > maxIdx {
				maxIdx = item.Index
			}
		}
		if s.Inventory.NextAcquisitionSortId <= maxIdx {
			s.Inventory.NextAcquisitionSortId = maxIdx + 1
		}
		// NextEquipIndex is a SEPARATE equip-list counter, NOT a visibility gate:
		// genuine console saves legitimately keep it far below NextAcquisitionSortId
		// (e.g. 1833 vs 7787 with item indices up to 7786, and the game renders fine).
		// The old reconcile here forced NextEquipIndex up to NextAcquisitionSortId and
		// wrote it back on load — which corrupted the slot (CE-108255-1) on a plain
		// load+save. We now leave NextEquipIndex exactly as the game wrote it.
	}

	// Reconcile common_item_count header at invStart-4 to the actual non-empty
	// slot count. External editors may leave this counter stale (er-save-manager
	// increments on add but not on remove; our old code never touched it at all).
	// A stale counter causes the game to use the wrong insertion index, overwriting
	// valid items or incorrectly triggering "inventory full". Written to binary so
	// the corrected value persists when the user saves.
	ReconcileInventoryHeader(s)

	// Storage Box
	storageStart := s.StorageBoxOffset + StorageHeaderSkip
	if storageStart+StorageSafetyMarg < len(s.Data) {
		sr := NewReader(s.Data)
		sr.Seek(int64(storageStart), 0)
		if err := s.Storage.ReadStorage(sr, StorageItemCount); err != nil {
			return fmt.Errorf("storage read: %w", err)
		}

		// Read storage trailing counters from fixed position
		// Layout: StorageCommonCount×12 + key_count(4) + StorageKeyCount×12 + next_equip_index(4) + next_acq_sort_id(4)
		storageNextEquipOff := storageStart + StorageNextEquipIdxRel
		storageNextAcqOff := storageStart + StorageNextAcqSortRel
		if storageNextAcqOff+4 <= len(s.Data) {
			s.Storage.nextEquipIndexOff = storageNextEquipOff
			s.Storage.NextEquipIndex = binary.LittleEndian.Uint32(s.Data[storageNextEquipOff:])
			s.Storage.nextAcqSortIdOff = storageNextAcqOff
			s.Storage.NextAcquisitionSortId = binary.LittleEndian.Uint32(s.Data[storageNextAcqOff:])
		}
	}
	return nil
}

func (s *SaveSlot) SyncPlayerToData() {
	sa := NewSlotAccessor(s.Data)
	mo := s.MagicOffset

	sa.WriteU32(mo+OffLevel, s.Player.Level)
	sa.WriteU32(mo+OffVigor, s.Player.Vigor)
	sa.WriteU32(mo+OffMind, s.Player.Mind)
	sa.WriteU32(mo+OffEndurance, s.Player.Endurance)
	sa.WriteU32(mo+OffStrength, s.Player.Strength)
	sa.WriteU32(mo+OffDexterity, s.Player.Dexterity)
	sa.WriteU32(mo+OffIntelligence, s.Player.Intelligence)
	sa.WriteU32(mo+OffFaith, s.Player.Faith)
	sa.WriteU32(mo+OffArcane, s.Player.Arcane)
	sa.WriteU32(mo+OffSouls, s.Player.Souls)
	sa.WriteU32(mo+OffSoulMemory, s.Player.SoulMemory)
	sa.WriteU8(mo+OffGender, s.Player.Gender)
	sa.WriteU8(mo+OffVoiceType, s.Player.VoiceType)
	sa.WriteU8(mo+OffClass, s.Player.Class)
	sa.WriteU8(mo+OffTalismanSlots, s.Player.TalismanSlots)
	sa.WriteU8(mo+OffGreatRuneOn, s.Player.GreatRuneOn)
	sa.WriteU8(mo+OffScadutreeBlessing, s.Player.ScadutreeBlessing)
	sa.WriteU8(mo+OffShadowRealmBlessing, s.Player.ShadowRealmBlessing)

	if s.ClearCountOffset > 0 && s.ClearCountOffset+4 <= len(s.Data) {
		sa.WriteU32(s.ClearCountOffset, s.Player.ClearCount)
	}

	grOff := s.EquipItemsIDOffset + DynEquipGreatRune
	if s.EquipItemsIDOffset > 0 && grOff+4 <= len(s.Data) {
		sa.WriteU32(grOff, s.Player.EquippedGreatRune)
	}

	nameOff := mo + OffCharacterName
	for i := 0; i < 16; i++ {
		sa.WriteU16(nameOff+i*2, s.Player.CharacterName[i])
	}
}

func (s *SaveSlot) Write(platform string) []byte {
	s.SyncPlayerToData()

	// NOTE: Per-slot SteamID is NOT written here. The offset is at a dynamic position within
	// the sequential data chain (after BaseVersion, before PS5Activity), NOT at SlotSize-8.
	// SlotSize-8 falls inside the PlayerGameDataHash region (last 0x80 bytes). Writing there
	// corrupts hash data. The primary SteamID is stored in UserData10 and flushed by
	// flushMetadata() — that is the authoritative source the game uses.

	// NOTE: CSPlayerGameDataHash (last 0x80 bytes) is intentionally NOT recomputed here.
	// All reference editors (ER-Save-Editor, er-save-manager, Final.py) treat this region
	// as read-only — they preserve the original bytes from the save file. The game does not
	// validate this hash on load. Recomputing it with a wrong algorithm corrupts those bytes
	// and causes EXCEPTION_ACCESS_VIOLATION (the game uses these offsets for equipment lookup).

	return s.Data
}

type ProfileSummary struct {
	CharacterName [16]uint16
	Level         uint32
}

func (p *ProfileSummary) Read(r *Reader) error {
	start := r.Pos()
	// Layout (verified Apr 2026 from real saves; see spec/23-user-data-10.md):
	//   +0x00 .. +0x1F : CharacterName UTF-16LE (16 chars × 2 bytes = 32)
	//   +0x20 .. +0x21 : null u16 terminator (skipped)
	//   +0x22          : Level (u32)
	//   +0x26 .. +0x24B: opaque (face data snapshot, equipment, archetype, etc.)
	//   Total slot stride: 0x24C bytes.
	for i := 0; i < 16; i++ {
		p.CharacterName[i], _ = r.ReadU16()
	}
	r.Seek(int64(start+0x22), 0)
	p.Level, _ = r.ReadU32()
	r.Seek(int64(start+0x24C), 0)
	return nil
}

func (p *ProfileSummary) Serialize(data []byte, offset int) {
	// We write only Name (+0x00, 32 bytes = 16 u16) and Level (+0x22, u32). The
	// 2-byte null u16 terminator at +0x20 and the remaining 0x24C-0x26 = 0x226 bytes
	// (face data snapshot, equipment summary, archetype, gift, body_type) are NOT
	// written — they retain whatever the game wrote on its last save. This is OK:
	// the menu reads our updated name+level plus the game's own face snapshot.
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint16(data[offset+i*2:], p.CharacterName[i])
	}
	binary.LittleEndian.PutUint32(data[offset+0x22:], p.Level)
}

type CSMenuSystemSaveLoad struct {
	Data []byte
}

func (c *CSMenuSystemSaveLoad) Read(r *Reader) {
	c.Data, _ = r.ReadBytes(0x60000)
}

func UTF16ToString(u16 []uint16) string {
	for i, v := range u16 {
		if v == 0 {
			u16 = u16[:i]
			break
		}
	}
	return string(utf16.Decode(u16))
}
