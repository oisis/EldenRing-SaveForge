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

// TestLookupFemaleModelIDs_D1RealPresets pins the exact verified tuples for the
// two presets converted to Type B in D1, against the real generated preset data.
// The enabling mappings Hair 6→100, Eyebrow 3→2 and Decal 18→17 come from the
// controlled save tmp/save/ER0000-apperence-test3.sl2 (character "fkr").
func TestLookupFemaleModelIDs_D1RealPresets(t *testing.T) {
	cases := []struct {
		name string
		want [8]uint8
	}{
		{"Casca, Berserk's Band of the Falcon Commander", [8]uint8{20, 100, 0, 2, 0, 0, 0, 3}},
		{"Fire Keeper, the Dark Souls 3 NPC", [8]uint8{50, 106, 0, 9, 0, 0, 17, 3}},
	}
	for _, c := range cases {
		p := presetByName(c.name)
		if p == nil {
			t.Fatalf("preset %q not found in data.Presets", c.name)
		}
		if p.BodyType != 0 {
			t.Errorf("preset %q BodyType = %d, want 0 (Type B)", c.name, p.BodyType)
		}
		got, ok := LookupFemaleModelIDs(*p)
		if !ok {
			t.Fatalf("preset %q rejected by LookupFemaleModelIDs (no fallback)", c.name)
		}
		if got != c.want {
			t.Errorf("preset %q tuple = %v, want %v", c.name, got, c.want)
		}
	}
}

func presetByName(name string) *AppearancePreset {
	for i := range Presets {
		if Presets[i].Name == name {
			return &Presets[i]
		}
	}
	return nil
}
