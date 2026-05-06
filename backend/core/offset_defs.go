package core

// SlotSize is the fixed size of each save slot in bytes (2,621,440 = 0x280000).
// Source: SPEC.md §3.1 BND4 Container.
const SlotSize = 0x280000

// FallbackMagicBase is the hardcoded base used when MagicPattern is not found.
const FallbackMagicBase = 0x15420 + 432

// Offsets relative to MagicOffset (negative = before the pattern).
// Source: SPEC.md §5.2 PlayerGameData.
const (
	OffLevel               = -335
	OffVigor               = -379
	OffMind                = -375
	OffEndurance           = -371
	OffStrength            = -367
	OffDexterity           = -363
	OffIntelligence        = -359
	OffFaith               = -355
	OffArcane              = -351
	OffSouls               = -331
	OffGender              = -249
	OffClass               = -248
	OffGreatRuneOn         = -184 // GreatRuneActive (PGD 0xF7, u8 bool, 0=off 1=on)
	OffTalismanSlots       = -241 // AdditionalTalismanSlotsCount (PGD 0xBE, u8, range 0-3)
	OffScadutreeBlessing   = -187
	OffShadowRealmBlessing = -186
	OffVoiceType           = -245   // Voice type (0=Young1, 1=Young2, 2=Mature1, 3=Mature2, 4=Aged1, 5=Aged2)
	OffCharacterName       = -0x11B // 16 x uint16 UTF-16LE

	// MagicOffset must be at least this value; otherwise negative stat offsets
	// would access memory before the start of the slot buffer.
	MinMagicOffset = 400 // abs(OffVigor) + margin
)

// GaItems section.
// Source: SPEC.md §5.3 GaItems.
const (
	GaItemsStart = 0x20 // scan starts here
)

// GaItem record sizes by handle type prefix (upper nibble).
// Source: SPEC.md §5.3 GaItems.
const (
	GaRecordWeapon    = 21
	GaRecordArmor     = 16
	GaRecordAccessory = 8
	GaRecordItem      = 8
	GaRecordAoW       = 8
)

// GaItem handle constants.
const (
	GaHandleEmpty    = 0x00000000
	GaHandleInvalid  = 0xFFFFFFFF
	GaHandleTypeMask = 0xF0000000 // upper nibble = item type
)

// Inventory layout (relative to MagicOffset).
// Source: SPEC.md §5.4 Dynamic Offsets.
const (
	InvStartFromMagic  = 505    // MagicOffset + 505 — points to first common item (common_count header at -4)
	CommonItemCount    = 0xA80  // 2688 common item slots
	KeyItemCount       = 0x180  // 384 key item slots
	StorageItemCount   = 2048   // storage box capacity (read limit for ReadStorage)
	StorageCommonCount = 0x780  // 1920 actual common item slots in storage
	StorageKeyCount    = 0x80   // 128 key item slots in storage
	InvRecordLen       = 12     // bytes per inventory record (handle + qty + index)
	InvSafetyMargin    = 0x9000 // max distance from invStart to validate section
	StorageSafetyMarg  = 0x6000 // max distance from storageStart to validate section
	StorageHeaderSkip  = 4      // skip 4-byte header at StorageBoxOffset
	InvKeyCountHeader  = 4      // 4-byte key_count header between common and key items

	// Offsets of trailing counters relative to (StorageBoxOffset + StorageHeaderSkip).
	// Layout: StorageCommonCount×12 + key_count(4) + StorageKeyCount×12 + next_equip_index(4) + next_acq_sort_id(4)
	StorageNextEquipIdxRel = StorageCommonCount*InvRecordLen + InvKeyCountHeader + StorageKeyCount*InvRecordLen
	StorageNextAcqSortRel  = StorageNextEquipIdxRel + 4
)

// Dynamic offset chain constants (relative to InventoryEnd).
// Source: SPEC.md §5.4 Dynamic Offsets.
const (
	DynPlayerData           = 0x1B0
	DynSpEffect             = 0xD0
	DynEquipedItemIndex     = 0x58
	DynActiveEquipedItems   = 0x1C
	DynEquipedItemsID       = 0x58
	DynActiveEquipedItemsGa = 0x58
	DynInventoryHeld        = 0x9011
	DynEquipedSpells        = 0x74
	DynEquipedItems         = 0x8C
	DynEquipedGestures      = 0x18
	DynEquipedArmaments     = 0x9C
	DynEquipePhysics        = 0x0C
	DynFaceData             = 0x12F
	DynStorageBox           = 0x6010
	DynStorageToGestures    = 0x100
	DynHorse                = 0x29
	DynEquipGreatRune       = 0x28 // offset within EquippedItemsItemIds to Great Rune slot (u32)
	DynClearCount           = 0x44 // offset from horse to ClearCount (NG+ cycle, uint32)
	DynBloodStain           = 0x4C
	DynMenuProfile          = 0x103C
	DynGaItemsOther         = 0x1B588
	DynTutorialData         = 0x40B
	DynIngameTimer          = 0x1A
	DynEventFlags           = 0
)

// CSMenuSystemSaveLoad / Favorites preset constants.
// Located in UserData10.Data. The game's Mirror at Roundtable Hold reads presets from here.
// All 15 slots (0..14) are safe for use after the ProfileSummary offset fix
// (ProfileSummary now correctly writes at 0x195E + i*0x24C, well past all preset slots
// which span 0x154..0x1323). See spec/23-user-data-10.md and spec/31-appearance-presets.md.
const (
	FavBaseOffset = 0x154 // first preset slot in UserData10.Data (same for PC and PS4)
	FavSlotSize   = 0x130 // 304 bytes per preset slot
	FavSlotCount  = 15    // total preset slots in CSMenuSystemSaveLoad

	// Offsets within a single 0x130-byte preset slot
	FavOffBodyFlag  = 0x08 // u8: body flag
	FavOffBodyType  = 0x09 // u8: 0=female, 1=male
	FavOffMarker    = 0x14 // i32: -1 (0xFFFFFFFF) = empty, 0 = active (per er-save-manager)
	FavOffMagic     = 0x18 // "FACE" (4 bytes) — indicates slot is populated
	FavOffAlignment = 0x1C // u32: 4
	FavOffInnerSize = 0x20 // u32: 0x120 (288)
	FavOffModelIDs  = 0x24 // 8 × u32: model IDs (same layout as FaceData blob)
	FavOffFaceShape = 0x44 // 64 bytes: face shape sliders
	FavOffUnkBlock  = 0x84 // 64 bytes: unk0x6c — opaque, preserved on apply (game ignores preset's value)
	FavOffBody      = 0xC4 // 7 bytes: body proportions (head, chest, abdomen, arm_r, leg_r, arm_l, leg_l)
	FavOffSkin      = 0xCB // 91 bytes: skin & cosmetics (same length as slot's FaceData skin block)
)

// FavHeaderUnk is the constant u32 at preset header offset 0x04 (observed in all active presets).
const FavHeaderUnk = 0x11D0

// FaceData blob layout constants.
// FaceData is a 303-byte (0x12F) block stored at FaceDataOffset-FaceDataBlobSize.
// All offsets below are relative to the start of the FaceData blob.
const (
	FaceDataBlobSize = 0x12F // 303 bytes total

	// Header (16 bytes)
	FDOffMarker    = 0x00 // u32 = 0xFFFFFFFF
	FDOffMagic     = 0x04 // "FACE"
	FDOffAlignment = 0x08 // u32 = 4
	FDOffInnerSize = 0x0C // u32 = 0x120 (288)

	// Model IDs (8 × u32, effective u8 + 3 padding each)
	FDOffFaceModel     = 0x10
	FDOffHairModel     = 0x14
	FDOffEyeModel      = 0x18
	FDOffEyebrowModel  = 0x1C
	FDOffBeardModel    = 0x20
	FDOffEyepatchModel = 0x24
	FDOffDecalModel    = 0x28 // tattoo/mark
	FDOffEyelashModel  = 0x2C

	// Face shape parameters (64 × u8, 0x30-0x6F)
	FDOffFaceShape = 0x30

	// Unknown block (64 bytes, 0x70-0xAF) — leave unchanged
	FDOffUnknownBlock = 0x70

	// Body proportions (7 × u8, 0xB0-0xB6)
	FDOffHead    = 0xB0
	FDOffChest   = 0xB1
	FDOffAbdomen = 0xB2
	FDOffArmR    = 0xB3
	FDOffLegR    = 0xB4
	FDOffArmL    = 0xB5
	FDOffLegL    = 0xB6

	// Skin & cosmetics (91 bytes, 0xB7-0x111)
	FDOffSkinR       = 0xB7
	FDOffSkinG       = 0xB8
	FDOffSkinB       = 0xB9
	FDOffSkinLuster  = 0xBA
	FDOffPores       = 0xBB
	FDOffStubble     = 0xBC
	FDOffDarkCircles = 0xBD
	FDOffDarkCircleR = 0xBE
	FDOffDarkCircleG = 0xBF
	FDOffDarkCircleB = 0xC0
	FDOffCheeksInt   = 0xC1
	FDOffCheekR      = 0xC2
	FDOffCheekG      = 0xC3
	FDOffCheekB      = 0xC4
	FDOffEyeliner    = 0xC5
	FDOffEyelinerR   = 0xC6
	FDOffEyelinerG   = 0xC7
	FDOffEyelinerB   = 0xC8
	FDOffEyeShadLow  = 0xC9
	FDOffEyeShadLowR = 0xCA
	FDOffEyeShadLowG = 0xCB
	FDOffEyeShadLowB = 0xCC
	FDOffEyeShadUp   = 0xCD
	FDOffEyeShadUpR  = 0xCE
	FDOffEyeShadUpG  = 0xCF
	FDOffEyeShadUpB  = 0xD0
	FDOffLipstick    = 0xD1
	FDOffLipstickR   = 0xD2
	FDOffLipstickG   = 0xD3
	FDOffLipstickB   = 0xD4
	FDOffTattooH     = 0xD5
	FDOffTattooV     = 0xD6
	FDOffTattooAngle = 0xD7
	FDOffTattooExp   = 0xD8
	FDOffTattooR     = 0xD9
	FDOffTattooG     = 0xDA
	FDOffTattooB     = 0xDB
	FDOffTattooUnk   = 0xDC
	FDOffTattooFlip  = 0xDD
	FDOffBodyHair    = 0xDE
	FDOffBodyHairR   = 0xDF
	FDOffBodyHairG   = 0xE0
	FDOffBodyHairB   = 0xE1
	// Right eye
	FDOffRIrisR    = 0xE2
	FDOffRIrisG    = 0xE3
	FDOffRIrisB    = 0xE4
	FDOffRIrisSize = 0xE5
	FDOffRClouding = 0xE6
	FDOffRCloudR   = 0xE7
	FDOffRCloudG   = 0xE8
	FDOffRCloudB   = 0xE9
	FDOffRWhiteR   = 0xEA
	FDOffRWhiteG   = 0xEB
	FDOffRWhiteB   = 0xEC
	FDOffREyePos   = 0xED
	// Left eye
	FDOffLIrisR    = 0xEE
	FDOffLIrisG    = 0xEF
	FDOffLIrisB    = 0xF0
	FDOffLIrisSize = 0xF1
	FDOffLClouding = 0xF2
	FDOffLCloudR   = 0xF3
	FDOffLCloudG   = 0xF4
	FDOffLCloudB   = 0xF5
	FDOffLWhiteR   = 0xF6
	FDOffLWhiteG   = 0xF7
	FDOffLWhiteB   = 0xF8
	FDOffLEyePos   = 0xF9
	// Hair colors
	FDOffHairR      = 0xFA
	FDOffHairG      = 0xFB
	FDOffHairB      = 0xFC
	FDOffHairLuster = 0xFD
	FDOffHairRoot   = 0xFE
	FDOffHairWhite  = 0xFF
	// Beard colors
	FDOffBeardR      = 0x100
	FDOffBeardG      = 0x101
	FDOffBeardB      = 0x102
	FDOffBeardLuster = 0x103
	FDOffBeardRoot   = 0x104
	FDOffBeardWhite  = 0x105
	// Eyebrow colors
	FDOffBrowR      = 0x106
	FDOffBrowG      = 0x107
	FDOffBrowB      = 0x108
	FDOffBrowLuster = 0x109
	FDOffBrowRoot   = 0x10A
	FDOffBrowWhite  = 0x10B
	// Eyelash colors
	FDOffLashR = 0x10C
	FDOffLashG = 0x10D
	FDOffLashB = 0x10E
	// Eyepatch colors
	FDOffPatchR = 0x10F
	FDOffPatchG = 0x110
	FDOffPatchB = 0x111
)

// Sanity limits for dynamic size reads from untrusted save data.
const (
	MaxProjCount      = 200000 // max acquired_projectiles count (projSkip = count*8+4; observed: 67584 PC, 103168 PS4)
	MaxUnlockedRegCnt = 20000  // max unlocked_regions count (regSkip = count*4+4)
	MaxHandleAttempts = 10000  // max iterations for generateUniqueHandle
)

// GaItemData section (distinct_acquired_items_count + GaItem2 array).
// Source: ER-Save-Editor save_slot.rs, GaItemData struct.
// GaItemData records every weapon/AoW ID ever acquired. The game looks up weapon properties
// (reinforce_type etc.) from this list on load. Missing entry → crash.
const (
	GaItemDataEntryLen = 16   // id(4) + unk(4) + reinforce_type(4) + unk1(4)
	GaItemDataArrayOff = 8    // array starts after distinct_count(4) + unk1(4)
	GaItemDataMaxCount = 7000 // 0x1B58 max entries (matches DynGaItemsOther / GaItemDataEntryLen)
)

// DLC black tile removal constants.
// Two position records in the BloodStain section control the DLC map cover layer.
// Writing DLC-area coordinates here removes the black tile overlay.
// See spec/29-dlc-black-tiles.md for details.
const (
	// Offsets relative to afterRegs (end of unlocked regions array).
	DLCTileZeroStart = 0x0088 // start of range to zero out before writing coords
	DLCTileZeroEnd   = 0x0110 // end of range (exclusive)

	// Record 1: DLC map center (2 floats + 1 flag byte)
	DLCTileRec1X    = 0x008D // f32 X coordinate
	DLCTileRec1Y    = 0x0091 // f32 Y coordinate
	DLCTileRec1Flag = 0x0095 // u8 visited flag

	// Record 2: DLC area anchor (4 floats + 1 flag byte)
	DLCTileRec2X    = 0x00C5 // f32 X
	DLCTileRec2Y    = 0x00C9 // f32 Y
	DLCTileRec2Z    = 0x00CD // f32 Z
	DLCTileRec2W    = 0x00D1 // f32 W
	DLCTileRec2Flag = 0x00D5 // u8 visited flag
)

// DLC section constants.
// CSDlc is 0x32 (50) bytes located at SlotSize - 0xB2 (before PlayerGameDataHash).
// Byte[0] = pre-order gesture "The Ring"
// Byte[1] = Shadow of the Erdtree entry flag (non-zero = entered DLC; causes infinite loading without DLC)
// Bytes[2] = pre-order gesture "Ring of Miquella"
// Bytes[3-49] = must be 0x00
const (
	DlcSectionSize   = 0x32                                 // 50 bytes
	DlcSectionOffset = SlotSize - HashSize - DlcSectionSize // SlotSize - 0xB2
	DlcEntryFlagByte = 1                                    // byte index within DLC section for SotE entry flag
)

// InvEquipReservedMax is the highest CSGaItemIns index reserved for equipment slots (0-432).
// Inventory items added via save editor must have Index > InvEquipReservedMax.
// If next_acquisition_sort_id from the save is ≤ InvEquipReservedMax or overlaps an existing
// item's index, the game dereferences the wrong CSGaItemIns entry → EXCEPTION_ACCESS_VIOLATION.
const InvEquipReservedMax = 432

// GaItem entry counts by slot version.
// Source: ER-Save-Editor save_slot.rs, er-save-manager user_data_x.py
const (
	GaItemCountOld     = 5118 // 0x13FE — version ≤ 81
	GaItemCountNew     = 5120 // 0x1400 — version > 81
	GaItemVersionBreak = 81   // version threshold for GaItem count change
)
