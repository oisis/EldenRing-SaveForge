package vm

import (
	"testing"
	"unicode/utf16"
)

// TestNormalizeCharacterName pins the fixed 16-code-unit UTF-16 CharacterName
// normalization shared by ApplyVMToParsedSlot and the SaveCharacter diagnostics:
// short names pad, over-length names truncate at the buffer boundary.
func TestNormalizeCharacterName(t *testing.T) {
	decode := func(buf [16]uint16) string {
		n := 0
		for n < len(buf) && buf[n] != 0 {
			n++
		}
		return string(utf16.Decode(buf[:n]))
	}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"short pads", "Hero", "Hero"},
		{"empty", "", ""},
		{"exactly 16", "0123456789ABCDEF", "0123456789ABCDEF"},
		{"truncates past 16", "0123456789ABCDEFGHIJ", "0123456789ABCDEF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decode(NormalizeCharacterName(tt.in)); got != tt.want {
				t.Errorf("NormalizeCharacterName(%q) decoded = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRecalculateLevel(t *testing.T) {
	tests := []struct {
		name     string
		vm       CharacterViewModel
		expected uint32
	}{
		{
			name:     "Wretch base stats",
			vm:       CharacterViewModel{Vigor: 10, Mind: 10, Endurance: 10, Strength: 10, Dexterity: 10, Intelligence: 10, Faith: 10, Arcane: 10},
			expected: 1,
		},
		{
			name:     "Vagabond base stats",
			vm:       CharacterViewModel{Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
			expected: 9,
		},
		{
			name:     "Max level all 99",
			vm:       CharacterViewModel{Vigor: 99, Mind: 99, Endurance: 99, Strength: 99, Dexterity: 99, Intelligence: 99, Faith: 99, Arcane: 99},
			expected: 713,
		},
		{
			name:     "Confessor base stats",
			vm:       CharacterViewModel{Vigor: 10, Mind: 13, Endurance: 10, Strength: 12, Dexterity: 12, Intelligence: 9, Faith: 14, Arcane: 9},
			expected: 10,
		},
		{
			name:     "Sum below 80",
			vm:       CharacterViewModel{Vigor: 1, Mind: 1, Endurance: 1, Strength: 1, Dexterity: 1, Intelligence: 1, Faith: 1, Arcane: 1},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.vm.RecalculateLevel()
			if tt.vm.Level != tt.expected {
				t.Errorf("got Level=%d, want %d", tt.vm.Level, tt.expected)
			}
		})
	}
}

func TestValidateStatsConsistency_ValidVagabond(t *testing.T) {
	vm := &CharacterViewModel{
		Level:        9,
		Vigor:        15,
		Mind:         10,
		Endurance:    11,
		Strength:     14,
		Dexterity:    13,
		Intelligence: 9,
		Faith:        9,
		Arcane:       7,
	}

	result := vm.ValidateStatsConsistency(0) // Vagabond
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
}

func TestValidateStatsConsistency_StatBelowClassMin(t *testing.T) {
	vm := &CharacterViewModel{
		Level:        9,
		Vigor:        10, // Vagabond base is 15 — this is below minimum
		Mind:         10,
		Endurance:    11,
		Strength:     14,
		Dexterity:    13,
		Intelligence: 9,
		Faith:        9,
		Arcane:       7,
	}

	result := vm.ValidateStatsConsistency(0) // Vagabond
	if result.Valid {
		t.Error("expected invalid — Vigor below class minimum")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error")
	}
}

func TestValidateStatsConsistency_LevelMismatch(t *testing.T) {
	vm := &CharacterViewModel{
		Level:        50, // Wrong — should be 9 for these stats
		Vigor:        15,
		Mind:         10,
		Endurance:    11,
		Strength:     14,
		Dexterity:    13,
		Intelligence: 9,
		Faith:        9,
		Arcane:       7,
	}

	result := vm.ValidateStatsConsistency(0)
	if result.Valid {
		t.Error("expected invalid — level does not match stat sum")
	}
}

func TestValidateStatsConsistency_MaxLevel(t *testing.T) {
	vm := &CharacterViewModel{
		Level:        713,
		Vigor:        99,
		Mind:         99,
		Endurance:    99,
		Strength:     99,
		Dexterity:    99,
		Intelligence: 99,
		Faith:        99,
		Arcane:       99,
	}

	result := vm.ValidateStatsConsistency(9) // Wretch
	if !result.Valid {
		t.Errorf("expected valid at max level, got errors: %v", result.Errors)
	}
}

func TestValidateStatsConsistency_AttributeOver99(t *testing.T) {
	vm := &CharacterViewModel{
		Level:        100,
		Vigor:        100, // Over 99
		Mind:         10,
		Endurance:    10,
		Strength:     10,
		Dexterity:    10,
		Intelligence: 10,
		Faith:        10,
		Arcane:       10,
	}

	result := vm.ValidateStatsConsistency(9)
	if result.Valid {
		t.Error("expected invalid — Vigor over 99")
	}
}

func TestValidateStatsConsistency_UnknownClass(t *testing.T) {
	vm := &CharacterViewModel{
		Level:        1,
		Vigor:        10,
		Mind:         10,
		Endurance:    10,
		Strength:     10,
		Dexterity:    10,
		Intelligence: 10,
		Faith:        10,
		Arcane:       10,
	}

	result := vm.ValidateStatsConsistency(255) // Invalid class
	if !result.Valid {
		t.Errorf("expected valid (unknown class skips class check), got errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning about unknown class")
	}
}

func TestClampToClassMinimums(t *testing.T) {
	vm := &CharacterViewModel{
		Vigor:        5, // Below Vagabond base (15)
		Mind:         5, // Below Vagabond base (10)
		Endurance:    5, // Below Vagabond base (11)
		Strength:     5, // Below Vagabond base (14)
		Dexterity:    5, // Below Vagabond base (13)
		Intelligence: 5, // Below Vagabond base (9)
		Faith:        5, // Below Vagabond base (9)
		Arcane:       5, // Below Vagabond base (7)
	}

	vm.ClampToClassMinimums(0) // Vagabond

	if vm.Vigor != 15 {
		t.Errorf("Vigor: got %d, want 15", vm.Vigor)
	}
	if vm.Mind != 10 {
		t.Errorf("Mind: got %d, want 10", vm.Mind)
	}
	if vm.Endurance != 11 {
		t.Errorf("Endurance: got %d, want 11", vm.Endurance)
	}
	if vm.Strength != 14 {
		t.Errorf("Strength: got %d, want 14", vm.Strength)
	}
	if vm.Dexterity != 13 {
		t.Errorf("Dexterity: got %d, want 13", vm.Dexterity)
	}
	if vm.Intelligence != 9 {
		t.Errorf("Intelligence: got %d, want 9", vm.Intelligence)
	}
	if vm.Faith != 9 {
		t.Errorf("Faith: got %d, want 9", vm.Faith)
	}
	if vm.Arcane != 7 {
		t.Errorf("Arcane: got %d, want 7", vm.Arcane)
	}
}

func TestClampToClassMinimums_UnknownClass(t *testing.T) {
	vm := &CharacterViewModel{
		Vigor: 0, Mind: 0, Endurance: 0, Strength: 0,
		Dexterity: 0, Intelligence: 0, Faith: 0, Arcane: 0,
	}

	vm.ClampToClassMinimums(255) // Unknown

	// All should be clamped to 1 (global minimum)
	attrs := []uint32{vm.Vigor, vm.Mind, vm.Endurance, vm.Strength,
		vm.Dexterity, vm.Intelligence, vm.Faith, vm.Arcane}
	for i, v := range attrs {
		if v != 1 {
			t.Errorf("attr[%d]: got %d, want 1", i, v)
		}
	}
}

func TestAllClassBaseStatsConsistent(t *testing.T) {
	// Verify that every starting class's base stats produce the correct level
	classData := []struct {
		classID                                 uint8
		level                                   uint32
		vig, mnd, end, str, dex, int_, fai, arc uint32
	}{
		{0, 9, 15, 10, 11, 14, 13, 9, 9, 7},
		{1, 8, 11, 12, 11, 10, 16, 10, 8, 9},
		{2, 7, 14, 9, 12, 16, 9, 7, 8, 11},
		{3, 5, 10, 11, 10, 9, 13, 9, 8, 14},
		{4, 6, 9, 15, 9, 8, 12, 16, 7, 9},
		{5, 7, 10, 14, 8, 11, 10, 7, 16, 10},
		{6, 9, 12, 11, 13, 12, 15, 9, 8, 8},
		{7, 9, 11, 12, 11, 11, 14, 14, 6, 9},
		{8, 10, 10, 13, 10, 12, 12, 9, 14, 9},
		{9, 1, 10, 10, 10, 10, 10, 10, 10, 10},
	}

	for _, c := range classData {
		vm := &CharacterViewModel{
			Level:        c.level,
			Vigor:        c.vig,
			Mind:         c.mnd,
			Endurance:    c.end,
			Strength:     c.str,
			Dexterity:    c.dex,
			Intelligence: c.int_,
			Faith:        c.fai,
			Arcane:       c.arc,
		}

		result := vm.ValidateStatsConsistency(c.classID)
		if !result.Valid {
			t.Errorf("class %d base stats invalid: %v", c.classID, result.Errors)
		}
	}
}
