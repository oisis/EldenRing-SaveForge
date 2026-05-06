package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const appVersion = "0.8.0"

// PresetInfo is the frontend-facing summary of an appearance preset.
type PresetInfo struct {
	Name     string `json:"name"`
	Image    string `json:"image"`    // filename in presets/ dir (e.g. "geralt.jpg")
	BodyType string `json:"bodyType"` // "Type A" or "Type B"
}

// writePresetAppearance writes FaceShape, Body, Skin and Model IDs from preset into slot's
// FaceData blob and sets slot.Player.Gender. fd must be the byte offset of the FaceData blob
// start within slot.Data (i.e. slot.FaceDataStart()). The unk0x6c block is preserved.
func writePresetAppearance(slot *core.SaveSlot, fd int, preset *data.AppearancePreset) {
	copy(slot.Data[fd+core.FDOffFaceShape:fd+core.FDOffFaceShape+64], preset.FaceShape[:])
	copy(slot.Data[fd+core.FDOffHead:fd+core.FDOffHead+7], preset.Body[:])
	copy(slot.Data[fd+core.FDOffSkinR:fd+core.FDOffSkinR+91], preset.Skin[:])

	writePartsID := func(fdOff int, partsId uint8) {
		binary.LittleEndian.PutUint32(slot.Data[fd+fdOff:], uint32(partsId))
	}

	if preset.BodyType == 1 {
		// Male: UI-1 applies; hair uses dedicated lookup table.
		ui1 := func(v uint8) uint8 {
			if v > 0 {
				return v - 1
			}
			return 0
		}
		writePartsID(core.FDOffFaceModel, ui1(preset.FaceModel))
		writePartsID(core.FDOffEyeModel, ui1(preset.EyeModel))
		writePartsID(core.FDOffEyebrowModel, ui1(preset.EyebrowModel))
		writePartsID(core.FDOffBeardModel, ui1(preset.BeardModel))
		writePartsID(core.FDOffEyepatchModel, ui1(preset.EyepatchModel))
		writePartsID(core.FDOffDecalModel, ui1(preset.DecalModel))
		writePartsID(core.FDOffEyelashModel, ui1(preset.EyelashModel))
		if partsId, ok := data.LookupMaleHairPartsID(preset.HairModel); ok {
			writePartsID(core.FDOffHairModel, partsId)
		} else {
			writePartsID(core.FDOffHairModel, ui1(preset.HairModel))
		}
	} else {
		// Female: UI-1 does NOT apply — female PartsId ranges differ entirely from male.
		// Use empirically confirmed safe values (tmp/re-character/facedata_dump.txt).
		f := data.FemaleModelIDs
		writePartsID(core.FDOffFaceModel, f.FaceModel)
		writePartsID(core.FDOffHairModel, f.HairModel)
		writePartsID(core.FDOffEyeModel, f.EyeModel)
		writePartsID(core.FDOffEyebrowModel, f.EyebrowModel)
		writePartsID(core.FDOffBeardModel, f.BeardModel)
		writePartsID(core.FDOffEyepatchModel, f.EyepatchModel)
		writePartsID(core.FDOffDecalModel, f.DecalModel)
		writePartsID(core.FDOffEyelashModel, f.EyelashModel)
	}

	slot.Player.Gender = preset.BodyType
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

	slot := &a.save.Slots[charIndex]
	fd := slot.FaceDataStart()
	if fd < 0 || fd+core.FaceDataBlobSize > len(slot.Data) {
		return fmt.Errorf("FaceData blob out of bounds: start=0x%X", fd)
	}

	a.pushUndo(charIndex)
	writePresetAppearance(slot, fd, preset)
	return nil
}

// SetCharacterGender changes the body type of a character and applies the default appearance
// preset for the target gender (Geralt for male, Ciri for female).
func (a *App) SetCharacterGender(charIndex int, targetGender uint8) error {
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

	slot := &a.save.Slots[charIndex]
	fd := slot.FaceDataStart()
	if fd < 0 || fd+core.FaceDataBlobSize > len(slot.Data) {
		return fmt.Errorf("FaceData blob out of bounds: start=0x%X", fd)
	}

	a.pushUndo(charIndex)
	writePresetAppearance(slot, fd, preset)
	return nil
}

// ListAppearancePresets returns the list of available character appearance presets.
func (a *App) ListAppearancePresets() []PresetInfo {
	result := make([]PresetInfo, len(data.Presets))
	for i, p := range data.Presets {
		bt := "Type A"
		if p.BodyType == 0 {
			bt = "Type B"
		}
		result[i] = PresetInfo{Name: p.Name, Image: p.Image, BodyType: bt}
	}
	return result
}

// applyMemoryStonesToSlot sets the quantity of memory stones in a slot to the desired count,
// adding them if absent, and syncs the corresponding pickup event flags.
func (a *App) applyMemoryStonesToSlot(slot *core.SaveSlot, desired uint32) error {
	if desired > 8 {
		desired = 8
	}
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
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
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

func (a *App) ExportCharacterPresetToFile(charIdx int, addSettings vm.PresetAddSettings) (string, error) {
	if a.save == nil {
		return "", fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return "", fmt.Errorf("invalid character index")
	}
	slot := &a.save.Slots[charIdx]
	name := core.UTF16ToString(slot.Player.CharacterName[:])
	if name == "" {
		return "", fmt.Errorf("slot %d is empty", charIdx)
	}

	charVM, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		return "", fmt.Errorf("failed to map slot to VM: %w", err)
	}

	preset := vm.VMToPreset(charVM, appVersion)
	preset.AddSettings = &addSettings

	worldData, err := vm.ExportWorldState(slot)
	if err == nil {
		preset.World = worldData
	}

	jsonData, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal preset: %w", err)
	}

	defaultName := fmt.Sprintf("%s_%d_%s.preset.json", preset.Character.Name, preset.Character.Level, preset.Character.ClassName)

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Character Preset",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "Character Preset (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("no file selected")
	}

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return "", fmt.Errorf("failed to write preset: %w", err)
	}

	return path, nil
}

func (a *App) LoadCharacterPresetFromFile() (*vm.CharacterPreset, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import Character Preset",
		Filters: []runtime.FileFilter{
			{DisplayName: "Character Preset (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("no file selected")
	}

	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read preset file: %w", err)
	}

	var preset vm.CharacterPreset
	if err := json.Unmarshal(fileData, &preset); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if preset.FormatVersion != vm.PresetFormatVersion {
		return nil, fmt.Errorf("unsupported preset format version %d (expected %d)", preset.FormatVersion, vm.PresetFormatVersion)
	}

	return &preset, nil
}

func (a *App) LoadCharacterPresetFromURL(url string) (*vm.CharacterPreset, error) {
	if url == "" {
		return nil, fmt.Errorf("empty URL")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var preset vm.CharacterPreset
	if err := json.Unmarshal(body, &preset); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if preset.FormatVersion != vm.PresetFormatVersion {
		return nil, fmt.Errorf("unsupported preset format version %d (expected %d)", preset.FormatVersion, vm.PresetFormatVersion)
	}

	return &preset, nil
}

func (a *App) ValidateCharacterPreset(preset vm.CharacterPreset) []string {
	return vm.ValidatePreset(&preset)
}

func (a *App) ApplyCharacterPreset(charIdx int, preset vm.CharacterPreset, opts vm.ApplyOptions) (*vm.PresetApplyResult, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return nil, fmt.Errorf("invalid character index")
	}
	slot := &a.save.Slots[charIdx]
	slotName := core.UTF16ToString(slot.Player.CharacterName[:])
	if slotName == "" {
		return nil, fmt.Errorf("slot %d is empty", charIdx)
	}

	a.pushUndo(charIdx)
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
