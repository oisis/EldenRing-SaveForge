package data

import "testing"

// TestLookupFemaleModelIDs_CoversAllTypeBPresets proves the verified UI→PartsId
// table resolves every model of every current Type B (BodyType == 0) preset.
// If a new Type B preset introduces an un-mapped UI value, this fails loudly
// rather than silently rejecting it at apply time.
func TestLookupFemaleModelIDs_CoversAllTypeBPresets(t *testing.T) {
	seen := 0
	for _, p := range Presets {
		if p.BodyType != 0 {
			continue
		}
		seen++
		if _, ok := LookupFemaleModelIDs(p); !ok {
			t.Errorf("preset %q (Type B) has a model value outside the verified mapping: "+
				"Face=%d Hair=%d Eye=%d Eyebrow=%d Beard=%d Eyepatch=%d Decal=%d Eyelash=%d",
				p.Name, p.FaceModel, p.HairModel, p.EyeModel, p.EyebrowModel,
				p.BeardModel, p.EyepatchModel, p.DecalModel, p.EyelashModel)
		}
	}
	if seen == 0 {
		t.Fatal("no Type B presets found in data.Presets — cannot validate coverage")
	}
}

// TestLookupFemaleModelIDs_MappedAndUnmapped pins the exact tuple order and the
// no-fallback contract: a fully mapped preset returns the verified raw IDs, and
// a single unmapped value rejects the whole tuple.
func TestLookupFemaleModelIDs_MappedAndUnmapped(t *testing.T) {
	// Distinct mapped values incl. a non-zero decal (29 → 29).
	p := AppearancePreset{
		BodyType: 0, FaceModel: 5, HairModel: 24, EyeModel: 0, EyebrowModel: 15,
		BeardModel: 1, EyepatchModel: 1, DecalModel: 29, EyelashModel: 4,
	}
	got, ok := LookupFemaleModelIDs(p)
	if !ok {
		t.Fatal("LookupFemaleModelIDs rejected a fully mapped preset")
	}
	// Order: Face, Hair, Eye, Eyebrow, Beard, Eyepatch, Decal, Eyelash.
	want := [8]uint8{40, 109, 0, 14, 0, 0, 29, 3}
	if got != want {
		t.Errorf("tuple = %v, want %v", got, want)
	}

	// One value outside the table (Face 99) rejects with a zero tuple, no fallback.
	bad := p
	bad.FaceModel = 99
	if tuple, ok := LookupFemaleModelIDs(bad); ok || tuple != ([8]uint8{}) {
		t.Errorf("unmapped preset = (%v, %v), want (zero, false)", tuple, ok)
	}
}
