package main

import (
	"fmt"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// appVersion is stamped into exported artifacts and exposed to the UI.
// Build commands generate app_version_generated.go from Makefile VERSION.
var appVersion = "dev"

// PresetInfo is the frontend-facing summary of an appearance preset.
type PresetInfo struct {
	Name     string `json:"name"`
	Image    string `json:"image"`    // filename in presets/ dir (e.g. "geralt.jpg")
	BodyType string `json:"bodyType"` // "Type A" or "Type B"
}

// writePresetAppearance resolves a preset through the shared appearance builder
// and writes it into slot's FaceData blob, setting slot.Player.Gender/VoiceType.
// fd must be the FaceData blob start (slot.FaceDataStart()); the unk0x6c block is
// preserved. An unmapped Type B preset returns an error with the slot untouched.
func writePresetAppearance(slot *core.SaveSlot, fd int, preset *data.AppearancePreset) error {
	resolved, err := resolveAppearance(preset)
	if err != nil {
		return err
	}
	applyResolvedAppearance(slot, fd, resolved)
	return nil
}

// findPresetByName returns a pointer to the named preset or nil if not found.
func findPresetByName(name string) *data.AppearancePreset {
	for i := range data.Presets {
		if data.Presets[i].Name == name {
			return &data.Presets[i]
		}
	}
	return nil
}

// ApplyPresetToCharacter applies a named appearance preset directly to a character's FaceData
// blob, replicating the appearance-change behaviour of SetCharacterGender but for any preset.
func (a *App) ApplyPresetToCharacter(charIndex int, presetName string) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return fmt.Errorf("invalid character index")
	}

	preset := findPresetByName(presetName)
	if preset == nil {
		return fmt.Errorf("preset %q not found", presetName)
	}

	// Resolve BEFORE locking/Undo so an unmapped Type B preset fails with no
	// snapshot and no mutation. Mapped Type B (verified UI→PartsId) is accepted.
	resolved, err := resolveAppearance(preset)
	if err != nil {
		return err
	}

	a.slotMu[charIndex].Lock()
	defer a.slotMu[charIndex].Unlock()

	slot := &a.save.Slots[charIndex]
	fd := slot.FaceDataStart()
	if fd < 0 || fd+core.FaceDataBlobSize > len(slot.Data) {
		return fmt.Errorf("FaceData blob out of bounds: start=0x%X", fd)
	}

	// Diagnostics: plan the Appearance changes against the pre-write slot, emit all
	// before then all planned records, apply, then read the real slot back for the
	// finished phase. Only fields the preset actually changes are logged.
	plans := planApplyAppearance(slot, fd, resolved)
	planned := appearancePlannedRecords(plans)
	a.journalCharacterChangeBefore(actionApplyAppearancePreset, charIndex, planned)
	a.journalCharacterChangePlanned(actionApplyAppearancePreset, charIndex, planned)

	a.pushUndoLocked(charIndex)
	applyResolvedAppearance(slot, fd, resolved)

	a.journalCharacterChangeFinished(actionApplyAppearancePreset, charIndex, characterChangeSuccess, characterStageCompleted, appearanceFinishedRecords(plans, slot, fd))
	return nil
}

// SetCharacterGender changes the body type of a character and applies the default appearance
// preset for the target gender (Geralt for male, Ciri for female).
func (a *App) SetCharacterGender(charIndex int, targetGender uint8) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return fmt.Errorf("invalid character index")
	}
	if targetGender > 1 {
		return fmt.Errorf("invalid gender: 0=female, 1=male")
	}

	var defaultName string
	if targetGender == 1 {
		defaultName = data.DefaultMalePresetName
	} else {
		defaultName = data.DefaultFemalePresetName
	}

	preset := findPresetByName(defaultName)
	if preset == nil {
		return fmt.Errorf("default preset %q not found", defaultName)
	}

	// Resolve BEFORE locking/Undo so an unmapped default preset fails with no
	// snapshot and no mutation. The mapped Ciri default enables Type B here.
	resolved, err := resolveAppearance(preset)
	if err != nil {
		return err
	}

	a.slotMu[charIndex].Lock()
	defer a.slotMu[charIndex].Unlock()

	slot := &a.save.Slots[charIndex]
	fd := slot.FaceDataStart()
	if fd < 0 || fd+core.FaceDataBlobSize > len(slot.Data) {
		return fmt.Errorf("FaceData blob out of bounds: start=0x%X", fd)
	}

	a.pushUndoLocked(charIndex)
	applyResolvedAppearance(slot, fd, resolved)
	return nil
}

// presetToInfo converts an appearance preset to its frontend PresetInfo shape.
func presetToInfo(p *data.AppearancePreset) PresetInfo {
	bt := "Type A"
	if p.BodyType == 0 {
		bt = "Type B"
	}
	return PresetInfo{Name: p.Name, Image: p.Image, BodyType: bt}
}

// ListAppearancePresets returns the list of available character appearance presets.
func (a *App) ListAppearancePresets() []PresetInfo {
	result := make([]PresetInfo, len(data.Presets))
	for i := range data.Presets {
		result[i] = presetToInfo(&data.Presets[i])
	}
	return result
}

// GetCharacterAppearancePreset returns the canonical preset whose resolved
// appearance the character at charIndex exactly matches, or (nil, nil) when
// nothing matches. Uses only the exact matcher (matchCharacterAppearance) —
// no fuzzy matching. Read-only: it takes saveMu.RLock + slotMu and never
// mutates the slot.
func (a *App) GetCharacterAppearancePreset(charIndex int) (*PresetInfo, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return nil, fmt.Errorf("invalid character index")
	}
	a.slotMu[charIndex].Lock()
	defer a.slotMu[charIndex].Unlock()

	p := matchCharacterAppearance(&a.save.Slots[charIndex])
	if p == nil {
		return nil, nil
	}
	info := presetToInfo(p)
	return &info, nil
}

// maxMemoryStones is the in-game maximum Memory Stone count. It lives next to
// the writer that enforces it; the diagnostic planner reuses the shared clamp
// below so a logged plan can never disagree with what the writer persists.
const maxMemoryStones = 8

// normalizeMemoryStones clamps a requested Memory Stone count to the game
// maximum. Single source of truth shared by applyMemoryStonesToSlot and the
// SaveCharacter Memory Stones diagnostic planner.
func normalizeMemoryStones(desired uint32) uint32 {
	if desired > maxMemoryStones {
		return maxMemoryStones
	}
	return desired
}

// hasEventFlagsRegion reports whether the slot exposes a usable Event Flags
// region — the shared guard used before any code touches event flags (Memory
// Stones pickup flags, NG+ flags, and their diagnostics). When it is false the
// writer leaves those flags untouched.
func hasEventFlagsRegion(slot *core.SaveSlot) bool {
	return slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data)
}

// applyMemoryStonesToSlot sets the quantity of memory stones in a slot to the desired count,
// adding them if absent, and syncs the corresponding pickup event flags.
func (a *App) applyMemoryStonesToSlot(slot *core.SaveSlot, desired uint32) error {
	desired = normalizeMemoryStones(desired)
	stoneFound := false
	for i := range slot.Inventory.CommonItems {
		if slot.Inventory.CommonItems[i].GaItemHandle == 0xB000272E {
			slot.Inventory.CommonItems[i].Quantity = desired
			stoneFound = true
			break
		}
	}
	if !stoneFound && desired > 0 {
		if err := core.AddItemsToSlot(slot, []uint32{0x4000272E}, int(desired), 0, false); err != nil {
			return fmt.Errorf("add memory stone: %w", err)
		}
	}
	if hasEventFlagsRegion(slot) {
		flags := slot.Data[slot.EventFlagsOffset:]
		flagList := data.BolsteringPickupFlags[0x4000272E]
		sorted := make([]uint32, len(flagList))
		copy(sorted, flagList)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		currentSet := 0
		for _, f := range sorted {
			if val, err := db.GetEventFlag(flags, f); err == nil && val {
				currentSet++
			}
		}
		if int(desired) > currentSet {
			toSet := int(desired) - currentSet
			set := 0
			for _, f := range sorted {
				if set >= toSet {
					break
				}
				if val, err := db.GetEventFlag(flags, f); err == nil && !val {
					if err := db.SetEventFlag(flags, f, true); err == nil {
						set++
					}
				}
			}
		} else if int(desired) < currentSet {
			toUnset := currentSet - int(desired)
			sortedDesc := make([]uint32, len(sorted))
			copy(sortedDesc, sorted)
			sort.Slice(sortedDesc, func(i, j int) bool { return sortedDesc[i] > sortedDesc[j] })
			unset := 0
			for _, f := range sortedDesc {
				if unset >= toUnset {
					break
				}
				if val, err := db.GetEventFlag(flags, f); err == nil && val {
					if err := db.SetEventFlag(flags, f, false); err == nil {
						unset++
					}
				}
			}
		}
	}
	return nil
}

// applyCharacterPresetLocked is the internal worker for ApplyCharacterPreset.
//
// Contract: caller MUST have validated `a.save != nil` and `charIdx` in range,
// and MUST hold a.saveMu.RLock + a.slotMu[charIdx].Lock for the entire call.
// The helper takes a single pushUndoLocked snapshot and never recurses into
// a public endpoint.
func (a *App) applyCharacterPresetLocked(charIdx int, preset vm.CharacterPreset, opts vm.ApplyOptions) (*vm.PresetApplyResult, error) {
	slot := &a.save.Slots[charIdx]
	slotName := core.UTF16ToString(slot.Player.CharacterName[:])
	if slotName == "" {
		return nil, fmt.Errorf("slot %d is empty", charIdx)
	}

	a.pushUndoLocked(charIdx)
	snapshot := core.SnapshotSlot(slot)

	result := &vm.PresetApplyResult{}
	result.Warnings = vm.ValidatePreset(&preset)

	if opts.ReplaceStats {
		tempVM := vm.PresetToVM(&preset)

		// Preserve blessings not stored in preset — do not zero them out.
		tempVM.ScadutreeBlessing = slot.Player.ScadutreeBlessing
		tempVM.ShadowRealmBlessing = slot.Player.ShadowRealmBlessing

		if opts.KeepName {
			tempVM.Name = slotName
		}
		effectiveClass := preset.Character.Class
		if opts.KeepClass {
			effectiveClass = slot.Player.Class
			tempVM.Class = slot.Player.Class
			cs := db.GetClassStats(slot.Player.Class)
			if cs != nil {
				tempVM.ClassName = cs.Name
			}
		}

		tempVM.ClampToClassMinimums(effectiveClass)
		tempVM.ValidateStats()
		tempVM.RecalculateLevel()

		if err := vm.ApplyVMToParsedSlot(tempVM, slot); err != nil {
			core.RestoreSlot(slot, snapshot)
			return nil, fmt.Errorf("failed to apply stats: %w", err)
		}
		slot.SyncPlayerToData()

		// When inventory is NOT replaced, apply MemoryStones from preset separately.
		if !opts.ReplaceInventory && preset.Character.MemoryStones > 0 {
			if err := a.applyMemoryStonesToSlot(slot, preset.Character.MemoryStones); err != nil {
				result.Warnings = append(result.Warnings, "memory stones: "+err.Error())
			}
		}

		result.StatsApplied = true
	}

	if opts.ReplaceInventory {
		removed, err := vm.ClearInventoryItems(slot)
		if err != nil {
			core.RestoreSlot(slot, snapshot)
			return nil, fmt.Errorf("failed to clear inventory: %w", err)
		}
		result.ItemsRemoved += removed

		itemsToAdd, addWarnings := vm.PresetItemsToItemsToAdd(preset.Inventory, true)
		result.Warnings = append(result.Warnings, addWarnings...)
		result.ItemsSkipped += len(preset.Inventory) - len(itemsToAdd)

		if len(itemsToAdd) > 0 {
			capReport := core.CheckAddCapacity(slot, itemsToAdd)
			if !capReport.CanFitAll {
				core.RestoreSlot(slot, snapshot)
				return nil, fmt.Errorf("inventory capacity exceeded: %s (need %d inv slots, have %d free)",
					capReport.CapHit, capReport.NeededInv, capReport.FreeInv)
			}

			if err := core.AddItemsToSlotBatch(slot, itemsToAdd); err != nil {
				core.RestoreSlot(slot, snapshot)
				return nil, fmt.Errorf("failed to add inventory items: %w", err)
			}
			result.ItemsAdded += len(itemsToAdd)
		}
	}

	if opts.ReplaceStorage {
		removed, err := vm.ClearStorageItems(slot)
		if err != nil {
			core.RestoreSlot(slot, snapshot)
			return nil, fmt.Errorf("failed to clear storage: %w", err)
		}
		result.ItemsRemoved += removed

		itemsToAdd, addWarnings := vm.PresetItemsToItemsToAdd(preset.Storage, false)
		result.Warnings = append(result.Warnings, addWarnings...)
		result.ItemsSkipped += len(preset.Storage) - len(itemsToAdd)

		if len(itemsToAdd) > 0 {
			capReport := core.CheckAddCapacity(slot, itemsToAdd)
			if !capReport.CanFitAll {
				core.RestoreSlot(slot, snapshot)
				return nil, fmt.Errorf("storage capacity exceeded: %s", capReport.CapHit)
			}

			if err := core.AddItemsToSlotBatch(slot, itemsToAdd); err != nil {
				core.RestoreSlot(slot, snapshot)
				return nil, fmt.Errorf("failed to add storage items: %w", err)
			}
			result.ItemsAdded += len(itemsToAdd)
		}
	}

	if opts.ReplaceWorld && preset.World != nil {
		worldWarnings := vm.ApplyWorldState(slot, preset.World)
		result.Warnings = append(result.Warnings, worldWarnings...)
		result.WorldApplied = true
	}

	core.ReconcileStorageHeader(slot)

	if violations := core.ValidatePostMutation(slot); len(violations) > 0 {
		core.RestoreSlot(slot, snapshot)
		return nil, fmt.Errorf("post-mutation validation failed: %s", violations[0].Error())
	}

	return result, nil
}
