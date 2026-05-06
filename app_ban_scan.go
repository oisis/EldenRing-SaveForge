package main

import (
	"fmt"
	"math"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// scanRunesCostForLevel returns the minimum SoulMemory a character at the given
// level must have. Mirrors runesCostForLevel in backend/vm/character_vm.go.
func scanRunesCostForLevel(level uint32) uint32 {
	total := int64(0)
	for n := uint32(2); n <= level; n++ {
		fn := float64(n)
		cost := int64(0.02*fn*fn*fn + 3.06*fn*fn + 105.6*fn - 895.0)
		if cost > 0 {
			total += cost
		}
	}
	if total > int64(math.MaxUint32) {
		return math.MaxUint32
	}
	return uint32(total)
}

// BanFinding is a single risk item found during a ban-risk scan.
type BanFinding struct {
	Tier     string `json:"tier"`     // "high" | "medium" | "info"
	Category string `json:"category"` // riskKey: cut_content | ban_risk | stat_above_99 | level_above_713 | upgrade_cap | steamid_mismatch | soul_memory_mismatch
	Detail   string `json:"detail"`
	ItemID   int    `json:"itemId"`
	ItemName string `json:"itemName"`
}

// SlotBanReport collects all findings for one character slot.
type SlotBanReport struct {
	SlotIndex int          `json:"slotIndex"`
	CharName  string       `json:"charName"`
	Level     int          `json:"level"`
	Findings  []BanFinding `json:"findings"`
}

// scanSlotFindings runs all ban-risk checks against a single slot and returns
// the findings. platform and saveSteamID are needed for the SteamID mismatch check.
func scanSlotFindings(slot *core.SaveSlot, platform core.Platform, saveSteamID uint64) []BanFinding {
	findings := []BanFinding{}

	// Level cap
	if slot.Player.Level > 713 {
		findings = append(findings, BanFinding{
			Tier:     "high",
			Category: "level_above_713",
			Detail:   fmt.Sprintf("Level %d exceeds maximum of 713", slot.Player.Level),
		})
	}

	// Attribute caps
	type attrCheck struct {
		name  string
		value uint32
	}
	attrs := []attrCheck{
		{"Vigor", slot.Player.Vigor},
		{"Mind", slot.Player.Mind},
		{"Endurance", slot.Player.Endurance},
		{"Strength", slot.Player.Strength},
		{"Dexterity", slot.Player.Dexterity},
		{"Intelligence", slot.Player.Intelligence},
		{"Faith", slot.Player.Faith},
		{"Arcane", slot.Player.Arcane},
	}
	for _, a := range attrs {
		if a.value > 99 {
			findings = append(findings, BanFinding{
				Tier:     "high",
				Category: "stat_above_99",
				Detail:   fmt.Sprintf("%s = %d (max 99)", a.name, a.value),
			})
		}
	}

	// Soul Memory consistency — minimum = cumulative rune cost from class start level to current level
	classStartLevel := uint32(1)
	if cs := db.GetClassStats(slot.Player.Class); cs != nil {
		classStartLevel = uint32(cs.Level)
	}
	minSM := scanRunesCostForLevel(slot.Player.Level) - scanRunesCostForLevel(classStartLevel)
	if slot.Player.SoulMemory < minSM {
		findings = append(findings, BanFinding{
			Tier:     "medium",
			Category: "soul_memory_mismatch",
			Detail:   fmt.Sprintf("Soul Memory %d < minimum required %d for Level %d (class start Lv %d)", slot.Player.SoulMemory, minSM, slot.Player.Level, classStartLevel),
		})
	}

	// SteamID mismatch (PC only)
	if platform == core.PlatformPC && slot.SteamID != 0 && slot.SteamID != saveSteamID {
		findings = append(findings, BanFinding{
			Tier:     "high",
			Category: "steamid_mismatch",
			Detail:   fmt.Sprintf("Slot SteamID %d ≠ save SteamID %d", slot.SteamID, saveSteamID),
		})
	}

	// Inventory + storage: item flags and upgrade caps
	itemSets := [][]core.InventoryItem{
		slot.Inventory.CommonItems,
		slot.Inventory.KeyItems,
		slot.Storage.CommonItems,
		slot.Storage.KeyItems,
	}
	seen := make(map[uint32]bool)
	for _, items := range itemSets {
		for _, invItem := range items {
			if invItem.GaItemHandle == 0 || invItem.GaItemHandle == 0xFFFFFFFF {
				continue
			}
			itemID := db.HandleToItemID(invItem.GaItemHandle)
			itemData, baseID := db.GetItemDataFuzzy(itemID)
			if itemData.Name == "" {
				continue
			}

			// Flag checks — deduplicate per baseID
			if !seen[baseID] {
				for _, flag := range itemData.Flags {
					if flag == "ban_risk" || flag == "cut_content" {
						findings = append(findings, BanFinding{
							Tier:     "high",
							Category: flag,
							Detail:   fmt.Sprintf("Item in inventory/storage: %s", itemData.Name),
							ItemID:   int(baseID),
							ItemName: itemData.Name,
						})
						seen[baseID] = true
						break
					}
				}
			}

			// Upgrade cap (weapons only — MaxUpgrade > 0)
			if itemData.MaxUpgrade > 0 && itemID >= baseID {
				diff := itemID - baseID
				currentUpgrade := diff % 100
				if currentUpgrade > itemData.MaxUpgrade {
					findings = append(findings, BanFinding{
						Tier:     "high",
						Category: "upgrade_cap",
						Detail:   fmt.Sprintf("%s: +%d exceeds cap +%d", itemData.Name, currentUpgrade, itemData.MaxUpgrade),
						ItemID:   int(baseID),
						ItemName: itemData.Name,
					})
				}
			}
		}
	}

	return findings
}

// ScanBanRisk scans all active slots for known ban-risk patterns and returns
// a per-slot report. Returns nil when no save is loaded.
func (a *App) ScanBanRisk() []SlotBanReport {
	if a.save == nil {
		return nil
	}

	var reports []SlotBanReport
	for i := 0; i < 10; i++ {
		if !a.save.ActiveSlots[i] {
			continue
		}
		slot := &a.save.Slots[i]
		reports = append(reports, SlotBanReport{
			SlotIndex: i,
			CharName:  core.UTF16ToString(slot.Player.CharacterName[:]),
			Level:     int(slot.Player.Level),
			Findings:  scanSlotFindings(slot, a.save.Platform, a.save.SteamID),
		})
	}
	return reports
}
