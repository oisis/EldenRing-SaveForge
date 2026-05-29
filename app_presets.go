package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// BuiltinCharacterPresetInfo is the frontend-facing summary of a built-in character preset.
type BuiltinCharacterPresetInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Modules     []string `json:"modules"`
	Level       uint32   `json:"level"`
	ClassName   string   `json:"className"`
}

type builtinCharacterPreset struct {
	info   BuiltinCharacterPresetInfo
	preset vm.CharacterPreset
}

// builtinCharacterPresets is the static registry of built-in character presets.
// Stat-only presets: no inventory, no storage, no world flags.
// Inventory presets: non-empty Inventory, no storage, no world, no weapons/infusion.
// Level = Vigor+Mind+Endurance+Strength+Dexterity+Intelligence+Faith+Arcane - 79
var builtinCharacterPresets = []builtinCharacterPreset{
	{
		info: BuiltinCharacterPresetInfo{
			ID:          "wretch-rl1",
			Name:        "Wretch RL1",
			Description: "All attributes at the Wretch class minimum of 10. Used for challenge runs or as a clean slate for custom stat distribution.",
			Tags:        []string{"wretch", "rl1", "challenge"},
			Modules:     []string{"Stats"},
			Level:       1,
			ClassName:   "Wretch",
		},
		// Vigor 10 + Mind 10 + End 10 + Str 10 + Dex 10 + Int 10 + Fai 10 + Arc 10 = 80; 80-79 = 1
		preset: vm.CharacterPreset{
			FormatVersion: vm.PresetFormatVersion,
			Character: vm.CharacterPresetCore{
				Class:         9, // Wretch
				ClassName:     "Wretch",
				Level:         1,
				Vigor:         10,
				Mind:          10,
				Endurance:     10,
				Strength:      10,
				Dexterity:     10,
				Intelligence:  10,
				Faith:         10,
				Arcane:        10,
				TalismanSlots: 2,
			},
			Inventory: []vm.PresetItem{},
			Storage:   []vm.PresetItem{},
		},
	},
	{
		info: BuiltinCharacterPresetInfo{
			ID:          "dex-pvp-60",
			Name:        "Dex PvP 60",
			Description: "Dexterity-focused build at Soul Level 60. Standard entry point for low-level PvP invasions. Samurai base class.",
			Tags:        []string{"pvp", "dex", "level60", "invasion"},
			Modules:     []string{"Stats"},
			Level:       60,
			ClassName:   "Samurai",
		},
		// Vigor 25 + Mind 14 + End 20 + Str 12 + Dex 40 + Int 9 + Fai 8 + Arc 11 = 139; 139-79 = 60
		// All values >= Samurai minimums (Vig 12, Min 11, End 13, Str 12, Dex 15, Int 9, Fai 8, Arc 8)
		preset: vm.CharacterPreset{
			FormatVersion: vm.PresetFormatVersion,
			Character: vm.CharacterPresetCore{
				Class:         6, // Samurai
				ClassName:     "Samurai",
				Level:         60,
				Vigor:         25,
				Mind:          14,
				Endurance:     20,
				Strength:      12,
				Dexterity:     40,
				Intelligence:  9,
				Faith:         8,
				Arcane:        11,
				TalismanSlots: 2,
			},
			Inventory: []vm.PresetItem{},
			Storage:   []vm.PresetItem{},
		},
	},
	{
		info: BuiltinCharacterPresetInfo{
			ID:          "quality-pvp-125",
			Name:        "Quality PvP 125",
			Description: "Balanced Strength/Dexterity build at Soul Level 125. The standard PvP meta bracket. Vagabond base class.",
			Tags:        []string{"pvp", "quality", "level125", "str", "dex"},
			Modules:     []string{"Stats"},
			Level:       125,
			ClassName:   "Vagabond",
		},
		// Vigor 50 + Mind 20 + End 25 + Str 40 + Dex 40 + Int 9 + Fai 9 + Arc 11 = 204; 204-79 = 125
		// All values >= Vagabond minimums (Vig 15, Min 10, End 11, Str 14, Dex 13, Int 9, Fai 9, Arc 7)
		preset: vm.CharacterPreset{
			FormatVersion: vm.PresetFormatVersion,
			Character: vm.CharacterPresetCore{
				Class:         0, // Vagabond
				ClassName:     "Vagabond",
				Level:         125,
				Vigor:         50,
				Mind:          20,
				Endurance:     25,
				Strength:      40,
				Dexterity:     40,
				Intelligence:  9,
				Faith:         9,
				Arcane:        11,
				TalismanSlots: 2,
			},
			Inventory: []vm.PresetItem{},
			Storage:   []vm.PresetItem{},
		},
	},
	// ─── Inventory presets ─────────────────────────────────────────────────────
	{
		info: BuiltinCharacterPresetInfo{
			ID:          "pvp-consumables",
			Name:        "PvP Consumables",
			Description: "Replaces inventory with essential PvP consumables: Rune Arcs and Furlcalling Finger Remedies. Warning: clears existing inventory.",
			Tags:        []string{"pvp", "consumables", "tools"},
			Modules:     []string{"Inventory"},
			Level:       0,
			ClassName:   "",
		},
		preset: vm.CharacterPreset{
			FormatVersion: vm.PresetFormatVersion,
			Character:     vm.CharacterPresetCore{},
			Inventory: []vm.PresetItem{
				// Rune Arc — activates equipped Great Rune
				{BaseID: 0x400000BE, Name: "Rune Arc", Quantity: 10},
				// Furlcalling Finger Remedy — reveals cooperative summon signs
				{BaseID: 0x40000096, Name: "Furlcalling Finger Remedy", Quantity: 50},
			},
			Storage: []vm.PresetItem{},
		},
	},
}

// ListBuiltinCharacterPresets returns the frontend-facing summaries of all built-in character presets.
func (a *App) ListBuiltinCharacterPresets() []BuiltinCharacterPresetInfo {
	result := make([]BuiltinCharacterPresetInfo, len(builtinCharacterPresets))
	for i, p := range builtinCharacterPresets {
		result[i] = p.info
	}
	return result
}

// isStatOnlyPreset returns true when the preset has no inventory, storage, or world data.
func isStatOnlyPreset(p vm.CharacterPreset) bool {
	return len(p.Inventory) == 0 && len(p.Storage) == 0 && p.World == nil
}

// GetBuiltinCharacterPreset returns the full CharacterPreset for a given built-in preset ID.
func (a *App) GetBuiltinCharacterPreset(id string) (*vm.CharacterPreset, error) {
	for _, p := range builtinCharacterPresets {
		if p.info.ID == id {
			preset := p.preset
			return &preset, nil
		}
	}
	return nil, fmt.Errorf("built-in preset not found: %s", id)
}

// ApplyBuiltinCharacterPresetStats applies the stat-only portion of a built-in preset to a character slot.
// Inventory, storage, and world state are never modified.
// Uses KeepName=true to preserve the existing character name.
func (a *App) ApplyBuiltinCharacterPresetStats(charIdx int, id string) (*vm.PresetApplyResult, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	for _, p := range builtinCharacterPresets {
		if p.info.ID == id {
			if !isStatOnlyPreset(p.preset) {
				return nil, fmt.Errorf("preset %q is not stat-only", id)
			}
			// Call the internal worker directly: re-entering the public
			// ApplyCharacterPreset would double-acquire slotMu (sync.Mutex
			// is not reentrant). The wrapper already holds the locks.
			return a.applyCharacterPresetLocked(charIdx, p.preset, vm.ApplyOptions{
				ReplaceStats:     true,
				ReplaceInventory: false,
				ReplaceStorage:   false,
				ReplaceWorld:     false,
				KeepName:         true,
				KeepClass:        false,
			})
		}
	}
	return nil, fmt.Errorf("built-in preset not found: %s", id)
}

// isInventoryCompatiblePreset returns true when the preset may be applied as inventory-only:
// it has at least one inventory item, no storage, no world data, and no infused/upgraded items
// (which would indicate weapons — not supported in inventory-only apply).
func isInventoryCompatiblePreset(p vm.CharacterPreset) bool {
	if len(p.Inventory) == 0 || len(p.Storage) > 0 || p.World != nil {
		return false
	}
	for _, item := range p.Inventory {
		if item.InfuseOffset != 0 || item.CurrentUpgrade != 0 {
			return false
		}
	}
	return true
}

// ApplyBuiltinCharacterPresetInventory replaces the inventory of a character slot
// with the items defined in the built-in preset. Stats, storage, and world are never modified.
//
// Semantics of ReplaceInventory=true (from ApplyCharacterPreset):
//   - ClearInventoryItems removes ALL current inventory items (including weapons, armor, talismans).
//   - Preset items are then added to the now-empty inventory.
//   - This is a FULL REPLACE — destructive. The user must backup the save first.
func (a *App) ApplyBuiltinCharacterPresetInventory(charIdx int, id string) (*vm.PresetApplyResult, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	for _, p := range builtinCharacterPresets {
		if p.info.ID == id {
			if !isInventoryCompatiblePreset(p.preset) {
				return nil, fmt.Errorf("preset %q is not inventory-compatible (may have storage, world, or infused items)", id)
			}
			// See ApplyBuiltinCharacterPresetStats: bypass the public
			// entry point to avoid double-acquire of slotMu.
			return a.applyCharacterPresetLocked(charIdx, p.preset, vm.ApplyOptions{
				ReplaceStats:     false,
				ReplaceInventory: true,
				ReplaceStorage:   false,
				ReplaceWorld:     false,
				KeepName:         true,
				KeepClass:        false,
			})
		}
	}
	return nil, fmt.Errorf("built-in preset not found: %s", id)
}

// ValidateBuiltinCharacterPreset runs read-only validation on a built-in preset.
// Returns a list of warning strings (empty = no issues). Does not modify the save.
func (a *App) ValidateBuiltinCharacterPreset(charIdx int, id string) ([]string, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	for _, p := range builtinCharacterPresets {
		if p.info.ID == id {
			preset := p.preset
			return vm.ValidatePreset(&preset), nil
		}
	}
	return nil, fmt.Errorf("built-in preset not found: %s", id)
}
