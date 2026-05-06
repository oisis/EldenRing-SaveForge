package vm

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

const PlayerGameDataOffset = 0x15420

// MaxLevel is the highest achievable level in Elden Ring (all 8 stats at 99).
const MaxLevel = 713

// RecalculateLevel updates the character level based on current attributes.
// Formula: Level = Vigor + Mind + Endurance + Strength + Dexterity + Intelligence + Faith + Arcane - 79
func (vm *CharacterViewModel) RecalculateLevel() {
	sum := vm.Vigor + vm.Mind + vm.Endurance + vm.Strength +
		vm.Dexterity + vm.Intelligence + vm.Faith + vm.Arcane
	if sum > 79 {
		vm.Level = sum - 79
	} else {
		vm.Level = 1
	}
}

// UpdateMatchmakingLevel scans the inventory and updates the matchmaking weapon level.
// Located at PlayerGameDataOffset + 0x93 (0x154B3).
func UpdateMatchmakingLevel(slotData []byte, maxUpgrade uint8) {
	const MatchmakingLvlOffset = PlayerGameDataOffset + 0x93

	currentLvl := slotData[MatchmakingLvlOffset]
	if maxUpgrade > currentLvl {
		slotData[MatchmakingLvlOffset] = maxUpgrade
	}
}

// ValidateStats ensures all attributes are within legal game limits (1-99).
func (vm *CharacterViewModel) ValidateStats() {
	limit := func(val *uint32) {
		if *val > 99 {
			*val = 99
		}
		if *val < 1 {
			*val = 1
		}
	}
	limit(&vm.Vigor)
	limit(&vm.Mind)
	limit(&vm.Endurance)
	limit(&vm.Strength)
	limit(&vm.Dexterity)
	limit(&vm.Intelligence)
	limit(&vm.Faith)
	limit(&vm.Arcane)
}

// StatValidationResult holds the results of class-aware stat validation.
type StatValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// ValidateStatsConsistency performs class-aware validation of character stats.
// It checks:
//   - Each attribute >= starting class base stat
//   - Level == sum(attributes) - 79
//   - Level in [1, 713]
//   - Class ID is valid (0-9)
//
// Returns validation result with errors (hard violations) and warnings (soft issues).
func (vm *CharacterViewModel) ValidateStatsConsistency(classID uint8) StatValidationResult {
	result := StatValidationResult{Valid: true}

	cs := db.GetClassStats(classID)
	if cs == nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Unknown class ID %d — skipping class-based validation", classID))
		// Still validate level formula and bounds
	}

	// 1. Check each attribute against class minimum
	if cs != nil {
		type attrCheck struct {
			name    string
			current uint32
			base    uint32
		}
		attrs := []attrCheck{
			{"Vigor", vm.Vigor, cs.Vigor},
			{"Mind", vm.Mind, cs.Mind},
			{"Endurance", vm.Endurance, cs.Endurance},
			{"Strength", vm.Strength, cs.Strength},
			{"Dexterity", vm.Dexterity, cs.Dexterity},
			{"Intelligence", vm.Intelligence, cs.Intelligence},
			{"Faith", vm.Faith, cs.Faith},
			{"Arcane", vm.Arcane, cs.Arcane},
		}
		for _, a := range attrs {
			if a.current < a.base {
				result.Valid = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s (%d) below %s base (%d)", a.name, a.current, cs.Name, a.base))
			}
		}
	}

	// 2. Attribute bounds [1, 99]
	type boundCheck struct {
		name string
		val  uint32
	}
	bounds := []boundCheck{
		{"Vigor", vm.Vigor},
		{"Mind", vm.Mind},
		{"Endurance", vm.Endurance},
		{"Strength", vm.Strength},
		{"Dexterity", vm.Dexterity},
		{"Intelligence", vm.Intelligence},
		{"Faith", vm.Faith},
		{"Arcane", vm.Arcane},
	}
	for _, b := range bounds {
		if b.val < 1 || b.val > 99 {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s (%d) outside valid range [1, 99]", b.name, b.val))
		}
	}

	// 3. Level formula: Level = sum(attributes) - 79
	sum := vm.Vigor + vm.Mind + vm.Endurance + vm.Strength +
		vm.Dexterity + vm.Intelligence + vm.Faith + vm.Arcane
	var expectedLevel uint32
	if sum > 79 {
		expectedLevel = sum - 79
	} else {
		expectedLevel = 1
	}
	if vm.Level != expectedLevel {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Level %d does not match stat sum (expected %d)", vm.Level, expectedLevel))
	}

	// 4. Level bounds [1, 713]
	if vm.Level < 1 || vm.Level > MaxLevel {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Level %d outside valid range [1, %d]", vm.Level, MaxLevel))
	}

	return result
}

// ClampToClassMinimums ensures no attribute is below the starting class minimum.
// If classID is unknown, only applies the global [1, 99] clamp.
func (vm *CharacterViewModel) ClampToClassMinimums(classID uint8) {
	cs := db.GetClassStats(classID)

	clamp := func(val *uint32, min uint32) {
		if *val < min {
			*val = min
		}
		if *val > 99 {
			*val = 99
		}
	}

	var baseMin uint32 = 1
	if cs != nil {
		clamp(&vm.Vigor, cs.Vigor)
		clamp(&vm.Mind, cs.Mind)
		clamp(&vm.Endurance, cs.Endurance)
		clamp(&vm.Strength, cs.Strength)
		clamp(&vm.Dexterity, cs.Dexterity)
		clamp(&vm.Intelligence, cs.Intelligence)
		clamp(&vm.Faith, cs.Faith)
		clamp(&vm.Arcane, cs.Arcane)
	} else {
		clamp(&vm.Vigor, baseMin)
		clamp(&vm.Mind, baseMin)
		clamp(&vm.Endurance, baseMin)
		clamp(&vm.Strength, baseMin)
		clamp(&vm.Dexterity, baseMin)
		clamp(&vm.Intelligence, baseMin)
		clamp(&vm.Faith, baseMin)
		clamp(&vm.Arcane, baseMin)
	}
}
