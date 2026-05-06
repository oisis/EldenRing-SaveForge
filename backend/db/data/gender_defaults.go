package data

// DefaultMalePresetName is the appearance preset applied when switching to male body type.
const DefaultMalePresetName = "Geralt of Rivia, the Witcher"

// DefaultFemalePresetName is the appearance preset applied when switching to female body type.
const DefaultFemalePresetName = "Ciri, the Princess of Cintra (Witcher)"

// FemaleModelIDs holds safe female Model ID PartsIds to write into FaceData when switching
// to female body type. These are empirically extracted from a real save after applying a
// female Mirror Favorites preset (see tmp/re-character/facedata_dump.txt).
//
// Female PartsId ranges are completely different from male — the male UI-1 formula does NOT
// apply. Until a full female UI→PartsId lookup table is built, these values serve as the
// guaranteed-visible fallback for all female preset applies.
//
//	FaceModel=21    confirmed female bone structure (Yennefer apply result)
//	HairModel=124   confirmed female hair style
//	EyeModel=0      shared across both genders
//	EyebrowModel=14 confirmed female eyebrow
//	BeardModel=0    no beard for female
//	EyepatchModel=0 no eyepatch
//	DecalModel=29   confirmed female decal/tattoo (zeroed by caller if unwanted)
//	EyelashModel=3  confirmed female eyelash
var FemaleModelIDs = struct {
	FaceModel     uint8
	HairModel     uint8
	EyeModel      uint8
	EyebrowModel  uint8
	BeardModel    uint8
	EyepatchModel uint8
	DecalModel    uint8
	EyelashModel  uint8
}{
	FaceModel:     21,
	HairModel:     124,
	EyeModel:      0,
	EyebrowModel:  14,
	BeardModel:    0,
	EyepatchModel: 0,
	DecalModel:    0, // zeroed to avoid unwanted tattoo on unrelated presets
	EyelashModel:  3,
}
