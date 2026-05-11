package data

// itemCompanionEventFlags maps item ID → event flags that must be set together
// with the item to replicate the state the game sets during normal acquisition.
//
// Without these flags the game may re-trigger cutscenes, dialogues, or block
// in-game mechanics even though the item is physically in inventory.
//
// Rules:
//   - Only include flags verified in post-acquisition real save data.
//   - Never include transient trigger flags (cleared by game engine after use).
//   - Never include area-specific or zone flags (out-of-bounds on PS4, optional on PC).
var itemCompanionEventFlags = map[uint32][]uint32{
	// Spectral Steed Whistle — replicates "Torrent received from Melina" state.
	//
	// Confirmed co-set in all 5 post-Melina PC slots (ER0000.sl2, 2026-05-11).
	// Sources: spec/12-torrent.md, spec/15-event-flags.md, er-save-manager event_flags_db.py.
	//
	// NOT included here: 710770, 69090, 69370 (Melina leaves Gatefront — research candidate,
	// deferred until before/after save diff available).
	ItemSpectralSteedWhistle: {
		EventFlagObtainedSpectralSteedWhistle,
		EventFlagMelinaGaveWhistle,
		EventFlagWhistleWorldState,
		EventFlagMelinaAcceptRefusePopup,
	},

	// Obtained flags for multiplayer pickup items. Without these flags,
	// the item pickup/interact state can remain visible in the world even
	// when the item is already in inventory.
	//
	// NOT included: Summoning Pool activation flags (670xxx) — separate mechanism.
	// NOT included: flags for other multiplayer items not listed here.
	ItemSmallGoldenEffigy:    {EventFlagObtainedSmallGoldenEffigy},
	ItemDuelistsFurledFinger: {EventFlagObtainedDuelistsFurledFinger},
	ItemSmallRedEffigy:       {EventFlagObtainedSmallRedEffigy},
	ItemWhiteCipherRing:      {EventFlagObtainedWhiteCipherRing},
	ItemBlueCipherRing:       {EventFlagObtainedBlueCipherRing},
}

// Item IDs with companion flags.
const (
	ItemSpectralSteedWhistle = uint32(0x40000082)

	// Multiplayer pickup items (tools, SubcatToolsMultiplayer).
	ItemSmallGoldenEffigy    = uint32(0x4000006D)
	ItemDuelistsFurledFinger = uint32(0x40000065)
	ItemSmallRedEffigy       = uint32(0x4000006E)
	ItemWhiteCipherRing      = uint32(0x40000068)
	ItemBlueCipherRing       = uint32(0x40000069)
)

// EventFlag IDs for Spectral Steed Whistle companion set.
const (
	// EventFlagObtainedSpectralSteedWhistle unlocks Torrent. Without this flag
	// the game refuses to summon Torrent even when the whistle is in inventory.
	EventFlagObtainedSpectralSteedWhistle = uint32(60100)

	// EventFlagMelinaGaveWhistle marks the Melina quest-give step as complete.
	// Prevents the "accept Torrent?" dialogue from re-triggering at graces.
	EventFlagMelinaGaveWhistle = uint32(4680)

	// EventFlagWhistleWorldState is the map/world counterpart set simultaneously
	// with 60100 during the in-game event.
	EventFlagWhistleWorldState = uint32(710520)

	// EventFlagMelinaAcceptRefusePopup marks the accept/refuse popup as shown.
	// Set as prerequisite before the give step in the Melina quest chain.
	EventFlagMelinaAcceptRefusePopup = uint32(4681)
)

// EventFlag IDs for multiplayer pickup item companion sets.
const (
	// EventFlagObtainedSmallGoldenEffigy marks the item as obtained.
	// Without it the pickup/interact state at Effigies of the Martyr can remain
	// visible even when the item is already in inventory.
	EventFlagObtainedSmallGoldenEffigy = uint32(60230)

	// EventFlagObtainedDuelistsFurledFinger marks the item as obtained.
	// Without it the pickup/interact state can remain visible in the world.
	EventFlagObtainedDuelistsFurledFinger = uint32(60240)

	// EventFlagObtainedSmallRedEffigy marks the item as obtained.
	// Without it the pickup/interact state at Effigies of the Martyr can remain
	// visible even when the item is already in inventory.
	EventFlagObtainedSmallRedEffigy = uint32(60250)

	// EventFlagObtainedWhiteCipherRing marks the item as obtained / shop sold-out.
	// Source: er-save-manager event_flags_db.py + ShopLineupParam.csv row 101800 eventFlag_forStock.
	EventFlagObtainedWhiteCipherRing = uint32(60280)

	// EventFlagObtainedBlueCipherRing marks the item as obtained / shop sold-out.
	// Absent from er-save-manager db, but confirmed by two independent sources:
	// (1) ShopLineupParam.csv row 101801 eventFlag_forStock=60290 (regulation.bin param data);
	// (2) Steam Deck before/after shop purchase diff: only flag in 60xxx range that changed 0→1.
	// Without it, the Twin Maiden Husks shop continues to offer the item after editor-add.
	EventFlagObtainedBlueCipherRing = uint32(60290)
)

// CompanionEventFlagsForItem returns the list of event flags that must be SET
// when an item is added to a character's save, or nil if the item has no
// companion flags.
func CompanionEventFlagsForItem(itemID uint32) []uint32 {
	return itemCompanionEventFlags[itemID]
}
