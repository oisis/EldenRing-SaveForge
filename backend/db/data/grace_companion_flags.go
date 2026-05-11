package data

// graceCompanionEventFlags maps grace EventFlag ID → event flags that must be SET
// when the grace is activated. These flags are never cleared on deactivation because
// they may also be set by item companion flags or normal game progression.
//
// Rules:
//   - SET-only: flags are set on visited=true, never cleared on visited=false.
//   - Only include flags verified in post-acquisition real save data.
//   - Never include RTH invitation flags (10009655, 11109658, 11109659).
//   - Never include transient trigger flags cleared by the game engine after use.
var graceCompanionEventFlags = map[uint32][]uint32{
	// Gatefront (Limgrave West) — EventFlag 76111 (0x0001294F).
	// Replicates "Initial Melina Accord accepted" state set during first Melina encounter.
	//
	// The game co-sets these flags when the player rests at Gatefront and Melina
	// offers the Spectral Steed Whistle. Without them the game may re-trigger the
	// Melina dialogue or leave Torrent unusable even when the whistle is in inventory.
	//
	// NOT included: RTH flags (10009655, 11109658, 11109659) — separate progression step.
	// NOT included: 710770, 69090, 69370 (Melina leaves Gatefront — research candidates,
	// confirmed runtime not required per spec/50 PS4 test 2026-05-11).
	GatefrontGraceEventFlagID: {
		EventFlagObtainedSpectralSteedWhistle,
		EventFlagMelinaGaveWhistle,
		EventFlagWhistleWorldState,
		EventFlagMelinaAcceptRefusePopup,
	},
}

// GatefrontGraceEventFlagID is the Site of Grace EventFlag for Gatefront (Limgrave West).
const GatefrontGraceEventFlagID = uint32(0x0001294F) // = 76111

// CompanionEventFlagsForGrace returns the list of event flags that must be SET
// when a grace is activated (visited=true). Returns nil if the grace has no
// companion flags. These flags are never cleared on deactivation — they may be
// shared with item companion flags or reflect irreversible game progression.
func CompanionEventFlagsForGrace(graceID uint32) []uint32 {
	return graceCompanionEventFlags[graceID]
}
