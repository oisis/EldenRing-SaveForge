package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// ── scanRunesCostForLevel ────────────────────────────────────────────────────

func TestScanRunesCostForLevel(t *testing.T) {
	tests := []struct {
		level    uint32
		expected uint32
	}{
		{0, 0},   // no iterations
		{1, 0},   // Wretch start — loop n=2..1 → no iterations
		{9, 473}, // Vagabond start — cumulative n=7(1)+n=8(155)+n=9(317)
	}
	for _, tt := range tests {
		got := scanRunesCostForLevel(tt.level)
		if got != tt.expected {
			t.Errorf("scanRunesCostForLevel(%d) = %d, want %d", tt.level, got, tt.expected)
		}
	}
	// Monotonically increasing for levels > 9.
	prev := scanRunesCostForLevel(9)
	for lvl := uint32(10); lvl <= 713; lvl++ {
		cur := scanRunesCostForLevel(lvl)
		if cur < prev {
			t.Errorf("scanRunesCostForLevel not monotonic at level %d: %d < %d", lvl, cur, prev)
		}
		prev = cur
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// cleanSlot returns a minimal valid slot (Vagabond, Lv9, SM=0, no items).
func cleanSlot() core.SaveSlot {
	return core.SaveSlot{
		Player: core.PlayerGameData{
			Level:       9,
			Class:       0, // Vagabond — starts at Lv 9
			Vigor:       15,
			Mind:        10,
			Endurance:   11,
			Strength:    14,
			Dexterity:   13,
			Intelligence: 9,
			Faith:        9,
			Arcane:       7,
			SoulMemory:  0,
		},
	}
}

// countCategory returns the number of findings with the given category.
func countCategory(findings []BanFinding, category string) int {
	n := 0
	for _, f := range findings {
		if f.Category == category {
			n++
		}
	}
	return n
}

// ── scanSlotFindings ─────────────────────────────────────────────────────────

func TestScanSlotFindings_CleanVagabond(t *testing.T) {
	slot := cleanSlot()
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if len(findings) != 0 {
		t.Errorf("clean Vagabond Lv9 got %d findings, want 0: %+v", len(findings), findings)
	}
}

func TestScanSlotFindings_CleanWretch(t *testing.T) {
	slot := core.SaveSlot{
		Player: core.PlayerGameData{
			Level: 1, Class: 9, // Wretch starts at Lv 1
			Vigor: 10, Mind: 10, Endurance: 10, Strength: 10,
			Dexterity: 10, Intelligence: 10, Faith: 10, Arcane: 10,
			SoulMemory: 0,
		},
	}
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if len(findings) != 0 {
		t.Errorf("clean Wretch Lv1 got %d findings, want 0: %+v", len(findings), findings)
	}
}

func TestScanSlotFindings_LevelAbove713(t *testing.T) {
	slot := cleanSlot()
	slot.Player.Level = 714
	// Ensure SM is high enough not to trigger soul_memory_mismatch at this level.
	slot.Player.SoulMemory = scanRunesCostForLevel(714)
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if countCategory(findings, "level_above_713") != 1 {
		t.Errorf("expected level_above_713 finding, got: %+v", findings)
	}
}

func TestScanSlotFindings_StatAbove99(t *testing.T) {
	tests := []struct {
		name  string
		mutFn func(p *core.PlayerGameData)
	}{
		{"Vigor=100", func(p *core.PlayerGameData) { p.Vigor = 100 }},
		{"Mind=100", func(p *core.PlayerGameData) { p.Mind = 100 }},
		{"Endurance=100", func(p *core.PlayerGameData) { p.Endurance = 100 }},
		{"Strength=100", func(p *core.PlayerGameData) { p.Strength = 100 }},
		{"Dexterity=100", func(p *core.PlayerGameData) { p.Dexterity = 100 }},
		{"Intelligence=100", func(p *core.PlayerGameData) { p.Intelligence = 100 }},
		{"Faith=100", func(p *core.PlayerGameData) { p.Faith = 100 }},
		{"Arcane=100", func(p *core.PlayerGameData) { p.Arcane = 100 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot := cleanSlot()
			tt.mutFn(&slot.Player)
			findings := scanSlotFindings(&slot, core.PlatformPC, 0)
			if countCategory(findings, "stat_above_99") < 1 {
				t.Errorf("%s: expected stat_above_99 finding, got: %+v", tt.name, findings)
			}
		})
	}
}

func TestScanSlotFindings_AllStatsOver99(t *testing.T) {
	slot := cleanSlot()
	slot.Player.Level = 713
	slot.Player.SoulMemory = scanRunesCostForLevel(713)
	slot.Player.Vigor = 100
	slot.Player.Mind = 100
	slot.Player.Endurance = 100
	slot.Player.Strength = 100
	slot.Player.Dexterity = 100
	slot.Player.Intelligence = 100
	slot.Player.Faith = 100
	slot.Player.Arcane = 100
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if n := countCategory(findings, "stat_above_99"); n != 8 {
		t.Errorf("expected 8 stat_above_99 findings, got %d: %+v", n, findings)
	}
}

func TestScanSlotFindings_SoulMemoryMismatch(t *testing.T) {
	// Vagabond leveled to 50 but SoulMemory not updated.
	slot := cleanSlot()
	slot.Player.Level = 50
	slot.Player.SoulMemory = 0

	minRequired := scanRunesCostForLevel(50) - scanRunesCostForLevel(9)

	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if countCategory(findings, "soul_memory_mismatch") != 1 {
		t.Errorf("expected soul_memory_mismatch (minRequired=%d), got: %+v", minRequired, findings)
	}
}

func TestScanSlotFindings_SoulMemoryExactMinimum(t *testing.T) {
	// SoulMemory set to exactly the minimum — must not trigger.
	slot := cleanSlot()
	slot.Player.Level = 50
	slot.Player.SoulMemory = scanRunesCostForLevel(50) - scanRunesCostForLevel(9)

	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if countCategory(findings, "soul_memory_mismatch") != 0 {
		t.Errorf("SoulMemory at exact minimum should not flag, got: %+v", findings)
	}
}

func TestScanSlotFindings_SoulMemoryBelowMinByOne(t *testing.T) {
	slot := cleanSlot()
	slot.Player.Level = 50
	minSM := scanRunesCostForLevel(50) - scanRunesCostForLevel(9)
	if minSM == 0 {
		t.Skip("min SM is 0, cannot test below-by-one")
	}
	slot.Player.SoulMemory = minSM - 1

	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if countCategory(findings, "soul_memory_mismatch") != 1 {
		t.Errorf("SoulMemory one below minimum should flag, got: %+v", findings)
	}
}

func TestScanSlotFindings_SteamIDMismatch_PC(t *testing.T) {
	slot := cleanSlot()
	slot.SteamID = 111111111
	findings := scanSlotFindings(&slot, core.PlatformPC, 222222222)
	if countCategory(findings, "steamid_mismatch") != 1 {
		t.Errorf("expected steamid_mismatch on PC, got: %+v", findings)
	}
}

func TestScanSlotFindings_SteamIDMismatch_PS4Exempt(t *testing.T) {
	// PS4 saves don't use SteamID — mismatch must not be flagged.
	slot := cleanSlot()
	slot.SteamID = 111111111
	findings := scanSlotFindings(&slot, core.PlatformPS, 222222222)
	if countCategory(findings, "steamid_mismatch") != 0 {
		t.Errorf("PS4 should not flag steamid_mismatch, got: %+v", findings)
	}
}

func TestScanSlotFindings_SteamIDMatch(t *testing.T) {
	slot := cleanSlot()
	slot.SteamID = 123456789
	findings := scanSlotFindings(&slot, core.PlatformPC, 123456789)
	if countCategory(findings, "steamid_mismatch") != 0 {
		t.Errorf("matching SteamIDs should not flag, got: %+v", findings)
	}
}

func TestScanSlotFindings_CutContentItem(t *testing.T) {
	// Entwining Umbilical Cord — talisman with cut_content + ban_risk flags.
	// Item ID 0x200017D4, handle prefix 0xA0 → handle 0xA00017D4.
	slot := cleanSlot()
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xA00017D4, Quantity: 1},
	}
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if countCategory(findings, "cut_content")+countCategory(findings, "ban_risk") == 0 {
		t.Errorf("expected cut_content or ban_risk finding for Entwining Umbilical Cord, got: %+v", findings)
	}
}

func TestScanSlotFindings_BanRiskItemInStorage(t *testing.T) {
	// Same item placed in storage instead of inventory — must also be detected.
	slot := cleanSlot()
	slot.Storage.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xA00017D4, Quantity: 1},
	}
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if countCategory(findings, "cut_content")+countCategory(findings, "ban_risk") == 0 {
		t.Errorf("expected finding for ban_risk item in storage, got: %+v", findings)
	}
}

func TestScanSlotFindings_UpgradeCapViolation(t *testing.T) {
	// Uchigatana base ID 0x00895440, MaxUpgrade 25.
	// +26 item ID = 0x00895440 + 26 = 0x0089545A, handle = 0x8089545A.
	slot := cleanSlot()
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0x8089545A, Quantity: 1},
	}
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if countCategory(findings, "upgrade_cap") != 1 {
		t.Errorf("expected upgrade_cap for Uchigatana +26, got: %+v", findings)
	}
}

func TestScanSlotFindings_UpgradeAtCap(t *testing.T) {
	// Uchigatana +25 — exactly at cap, must not flag.
	slot := cleanSlot()
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0x80895459, Quantity: 1}, // base 0x00895440 + 25 = 0x00895459
	}
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if countCategory(findings, "upgrade_cap") != 0 {
		t.Errorf("Uchigatana +25 is at cap and should not flag, got: %+v", findings)
	}
}

func TestScanSlotFindings_NullHandlesIgnored(t *testing.T) {
	// Null and sentinel handles (0x00000000 / 0xFFFFFFFF) must not crash or produce findings.
	slot := cleanSlot()
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0x00000000},
		{GaItemHandle: 0xFFFFFFFF},
	}
	findings := scanSlotFindings(&slot, core.PlatformPC, 0)
	if len(findings) != 0 {
		t.Errorf("null/sentinel handles should produce no findings, got: %+v", findings)
	}
}
