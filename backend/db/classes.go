package db

import "sort"

// ClassStats holds the base stats for a starting class in Elden Ring.
// These are used for stat consistency validation — no attribute can be
// edited below the starting class minimum, and Level must match the
// sum formula relative to the class starting level.
type ClassStats struct {
	ID           uint8  `json:"id"`
	Name         string `json:"name"`
	Level        uint32 `json:"level"`
	Vigor        uint32 `json:"vigor"`
	Mind         uint32 `json:"mind"`
	Endurance    uint32 `json:"endurance"`
	Strength     uint32 `json:"strength"`
	Dexterity    uint32 `json:"dexterity"`
	Intelligence uint32 `json:"intelligence"`
	Faith        uint32 `json:"faith"`
	Arcane       uint32 `json:"arcane"`
}

// StartingClasses maps Class ID (0-9) to base stats.
// Source: Elden Ring game data (regulation.bin), cross-referenced with
// soulsmods.github.io and ClayAmore/EldenRingSaveTemplate.
var StartingClasses = map[uint8]ClassStats{
	0: {ID: 0, Name: "Vagabond",   Level: 9,  Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9,  Faith: 9,  Arcane: 7},
	1: {ID: 1, Name: "Warrior",    Level: 8,  Vigor: 11, Mind: 12, Endurance: 11, Strength: 10, Dexterity: 16, Intelligence: 10, Faith: 8,  Arcane: 9},
	2: {ID: 2, Name: "Hero",       Level: 7,  Vigor: 14, Mind: 9,  Endurance: 12, Strength: 16, Dexterity: 9,  Intelligence: 7,  Faith: 8,  Arcane: 11},
	3: {ID: 3, Name: "Bandit",     Level: 5,  Vigor: 10, Mind: 11, Endurance: 10, Strength: 9,  Dexterity: 13, Intelligence: 9,  Faith: 8,  Arcane: 14},
	4: {ID: 4, Name: "Astrologer", Level: 6,  Vigor: 9,  Mind: 15, Endurance: 9,  Strength: 8,  Dexterity: 12, Intelligence: 16, Faith: 7,  Arcane: 9},
	5: {ID: 5, Name: "Prophet",    Level: 7,  Vigor: 10, Mind: 14, Endurance: 8,  Strength: 11, Dexterity: 10, Intelligence: 7,  Faith: 16, Arcane: 10},
	6: {ID: 6, Name: "Samurai",    Level: 9,  Vigor: 12, Mind: 11, Endurance: 13, Strength: 12, Dexterity: 15, Intelligence: 9,  Faith: 8,  Arcane: 8},
	7: {ID: 7, Name: "Prisoner",   Level: 9,  Vigor: 11, Mind: 12, Endurance: 11, Strength: 11, Dexterity: 14, Intelligence: 14, Faith: 6,  Arcane: 9},
	8: {ID: 8, Name: "Confessor",  Level: 10, Vigor: 10, Mind: 13, Endurance: 10, Strength: 12, Dexterity: 12, Intelligence: 9,  Faith: 14, Arcane: 9},
	9: {ID: 9, Name: "Wretch",     Level: 1,  Vigor: 10, Mind: 10, Endurance: 10, Strength: 10, Dexterity: 10, Intelligence: 10, Faith: 10, Arcane: 10},
}

// GetClassStats returns the starting stats for a given class ID.
// Returns nil if the class ID is not recognized.
func GetClassStats(classID uint8) *ClassStats {
	cs, ok := StartingClasses[classID]
	if !ok {
		return nil
	}
	return &cs
}

// GetAllClassStats returns all starting classes sorted by ID.
func GetAllClassStats() []ClassStats {
	result := make([]ClassStats, 0, len(StartingClasses))
	for _, cs := range StartingClasses {
		result = append(result, cs)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
