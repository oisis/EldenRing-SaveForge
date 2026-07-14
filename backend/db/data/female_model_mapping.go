package data

// Verified Type B (female) UI → PartsId model mappings, gathered from the
// controlled test save tmp/save/ER0000-apperence-test.sl2 on 2026-07-13. Female
// PartsId ranges differ entirely from male, so the male UI-1 formula does NOT
// apply. These tables cover exactly the values used by the current Type B
// presets; any UI value outside them is rejected — there is no fallback.
//
// Extended 2026-07-14 from the controlled save tmp/save/ER0000-apperence-test3.sl2
// (character "fkr", raw tuple [20, 100, 0, 2, 0, 0, 17, 3]), which proves female
// Hair UI 6→100, Eyebrow UI 3→2 and Decal UI 18→17. These enable the two newly
// converted Type B presets "Casca, Berserk's Band of the Falcon Commander" and
// "Fire Keeper, the Dark Souls 3 NPC".
var (
	femaleFaceUIToPartsID     = map[uint8]uint8{3: 20, 5: 40, 6: 50}
	femaleHairUIToPartsID     = map[uint8]uint8{1: 0, 6: 100, 22: 106, 24: 109, 31: 116, 37: 124}
	femaleEyeUIToPartsID      = map[uint8]uint8{0: 0}
	femaleEyebrowUIToPartsID  = map[uint8]uint8{1: 0, 3: 2, 10: 9, 15: 14, 16: 15}
	femaleBeardUIToPartsID    = map[uint8]uint8{1: 0}
	femaleEyepatchUIToPartsID = map[uint8]uint8{1: 0}
	femaleDecalUIToPartsID    = map[uint8]uint8{1: 0, 9: 8, 12: 11, 18: 17, 29: 29, 33: 33}
	femaleEyelashUIToPartsID  = map[uint8]uint8{1: 0, 3: 2, 4: 3}
)

// LookupFemaleModelIDs resolves a Type B preset's eight UI model values to their
// verified raw save-file PartsIds. The returned tuple order is
// Face, Hair, Eye, Eyebrow, Beard, Eyepatch, Decal, Eyelash.
//
// It returns (tuple, true) only when every model maps within the verified table;
// if any single value is unmapped it returns (zero, false) with no fallback, so
// callers never write a guessed or scrambled female appearance.
func LookupFemaleModelIDs(p AppearancePreset) ([8]uint8, bool) {
	var out [8]uint8
	lookups := [8]struct {
		table map[uint8]uint8
		ui    uint8
	}{
		{femaleFaceUIToPartsID, p.FaceModel},
		{femaleHairUIToPartsID, p.HairModel},
		{femaleEyeUIToPartsID, p.EyeModel},
		{femaleEyebrowUIToPartsID, p.EyebrowModel},
		{femaleBeardUIToPartsID, p.BeardModel},
		{femaleEyepatchUIToPartsID, p.EyepatchModel},
		{femaleDecalUIToPartsID, p.DecalModel},
		{femaleEyelashUIToPartsID, p.EyelashModel},
	}
	for i, l := range lookups {
		partsID, ok := l.table[l.ui]
		if !ok {
			return [8]uint8{}, false
		}
		out[i] = partsID
	}
	return out, true
}
