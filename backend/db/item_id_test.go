package db

import "testing"

func TestItemIDToMagicParamID_CatchFlame(t *testing.T) {
	// Catch Flame is stored in backend/db/data/incantations.go as
	// 0x40001770 — the canonical sanity check from the Phase 7d
	// spell-encoding audit.
	got := ItemIDToMagicParamID(0x40001770)
	if got != 0x1770 {
		t.Errorf("ItemIDToMagicParamID(0x40001770) = 0x%X, want 0x1770", got)
	}
}

func TestItemIDToMagicParamID_HighPayloadBitsSurvive28BitMask(t *testing.T) {
	// Regression guard for the rejected 16-bit mask. With a 0x0000FFFF
	// mask, a payload above 0xFFFF would be silently truncated and the
	// apply path would write the wrong raw MagicParam ID. The 28-bit
	// mask 0x0FFFFFFF must preserve every bit below the type prefix.
	cases := []struct {
		name string
		id   uint32
		want uint32
	}{
		{"payload exactly 0xFFFF", 0x4000FFFF, 0x0000FFFF},
		{"payload just above 0xFFFF", 0x40010000, 0x00010000},
		{"payload mid-range",         0x4012ABCD, 0x0012ABCD},
		{"payload near 28-bit cap",   0x4FFFFFFF, 0x0FFFFFFF},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ItemIDToMagicParamID(tc.id)
			if got != tc.want {
				t.Errorf("ItemIDToMagicParamID(0x%08X) = 0x%08X, want 0x%08X", tc.id, got, tc.want)
			}
		})
	}
}

func TestItemIDToMagicParamID_StripsAnyTypePrefix(t *testing.T) {
	// The helper is prefix-agnostic (validation happens upstream). This
	// confirms it doesn't accidentally preserve high nibble bits — a
	// regression here would let mis-validated IDs leak into raw spell
	// writes.
	cases := []struct {
		id   uint32
		want uint32
	}{
		{0x00001770, 0x1770}, // weapon prefix
		{0x40001770, 0x1770}, // goods prefix (sorcery/incantation)
		{0x60001770, 0x1770}, // legacy/incorrect prefix — still stripped
		{0x80001770, 0x1770}, // aow prefix
	}
	for _, tc := range cases {
		got := ItemIDToMagicParamID(tc.id)
		if got != tc.want {
			t.Errorf("ItemIDToMagicParamID(0x%08X) = 0x%X, want 0x%X", tc.id, got, tc.want)
		}
	}
}

func TestItemIDToMagicParamID_ExplicitClearZero(t *testing.T) {
	// BaseItemID == 0 is the template's explicit-clear sentinel. The
	// raw conversion of zero must remain zero — the apply resolver maps
	// this case to the save's empty-slot sentinel (0xFFFFFFFF) via a
	// branch, NOT via this helper.
	if got := ItemIDToMagicParamID(0); got != 0 {
		t.Errorf("ItemIDToMagicParamID(0) = 0x%X, want 0", got)
	}
}
