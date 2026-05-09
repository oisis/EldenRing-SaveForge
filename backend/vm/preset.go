package vm

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

const PresetFormatVersion = 1

type CharacterPreset struct {
	FormatVersion int                 `json:"formatVersion"`
	ExportedAt    string              `json:"exportedAt"`
	AppVersion    string              `json:"appVersion"`
	Character     CharacterPresetCore `json:"character"`
	Inventory     []PresetItem        `json:"inventory"`
	Storage       []PresetItem        `json:"storage"`
	AddSettings   *PresetAddSettings  `json:"addSettings,omitempty"`
	World         *WorldPresetData    `json:"world,omitempty"`
}

type WorldPresetData struct {
	Graces         []uint32 `json:"graces,omitempty"`
	Bosses         []uint32 `json:"bosses,omitempty"`
	SummoningPools []uint32 `json:"summoningPools,omitempty"`
	Colosseums     []uint32 `json:"colosseums,omitempty"`
	MapFlags       []uint32 `json:"mapFlags,omitempty"`
	Cookbooks      []uint32 `json:"cookbooks,omitempty"`
	BellBearings   []uint32 `json:"bellBearings,omitempty"`
	Whetblades     []uint32 `json:"whetblades,omitempty"`
	Gestures       []uint32 `json:"gestures,omitempty"`
	Regions        []uint32 `json:"regions,omitempty"`
	WorldPickups   []uint32 `json:"worldPickups,omitempty"`
}

type CharacterPresetCore struct {
	Name              string `json:"name"`
	Class             uint8  `json:"class"`
	ClassName         string `json:"className"`
	Level             uint32 `json:"level"`
	Souls             uint32 `json:"souls"`
	Vigor             uint32 `json:"vigor"`
	Mind              uint32 `json:"mind"`
	Endurance         uint32 `json:"endurance"`
	Strength          uint32 `json:"strength"`
	Dexterity         uint32 `json:"dexterity"`
	Intelligence      uint32 `json:"intelligence"`
	Faith             uint32 `json:"faith"`
	Arcane            uint32 `json:"arcane"`
	TalismanSlots     uint8  `json:"talismanSlots"`
	ClearCount        uint32 `json:"clearCount"`
	MemoryStones      uint32 `json:"memoryStones"`
}

type PresetAddSettings struct {
	Upgrade25            int  `json:"upgrade25"`
	Upgrade10            int  `json:"upgrade10"`
	InfuseOffset         int  `json:"infuseOffset"`
	UpgradeAsh           int  `json:"upgradeAsh"`
	TalismansHighestOnly bool `json:"talismansHighestOnly"`
}

type PresetItem struct {
	BaseID         uint32 `json:"baseId"`
	Name           string `json:"name"`
	Quantity       uint32 `json:"quantity"`
	CurrentUpgrade uint32 `json:"upgrade"`
	InfuseOffset   uint32 `json:"infuse,omitempty"`
}

type ApplyOptions struct {
	ReplaceStats     bool `json:"replaceStats"`
	ReplaceInventory bool `json:"replaceInventory"`
	ReplaceStorage   bool `json:"replaceStorage"`
	ReplaceWorld     bool `json:"replaceWorld"`
	KeepName         bool `json:"keepName"`
	KeepClass        bool `json:"keepClass"`
}

type PresetApplyResult struct {
	StatsApplied bool     `json:"statsApplied"`
	WorldApplied bool     `json:"worldApplied"`
	ItemsAdded   int      `json:"itemsAdded"`
	ItemsSkipped int      `json:"itemsSkipped"`
	ItemsRemoved int      `json:"itemsRemoved"`
	Warnings     []string `json:"warnings"`
}

var validInfuseOffsets = map[uint32]bool{
	0: true, 100: true, 200: true, 300: true, 400: true, 500: true,
	600: true, 700: true, 800: true, 900: true, 1000: true, 1100: true, 1200: true,
}

func VMToPreset(vm *CharacterViewModel, appVersion string) *CharacterPreset {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		AppVersion:    appVersion,
		Character: CharacterPresetCore{
			Name:              vm.Name,
			Class:             vm.Class,
			ClassName:         vm.ClassName,
			Level:             vm.Level,
			Souls:             vm.Souls,
			Vigor:             vm.Vigor,
			Mind:              vm.Mind,
			Endurance:         vm.Endurance,
			Strength:          vm.Strength,
			Dexterity:         vm.Dexterity,
			Intelligence:      vm.Intelligence,
			Faith:             vm.Faith,
			Arcane:            vm.Arcane,
			TalismanSlots:     vm.TalismanSlots,
			ClearCount:        vm.ClearCount,
			MemoryStones:      vm.MemoryStones,
		},
		Inventory: vmItemsToPresetItems(vm.Inventory),
		Storage:   vmItemsToPresetItems(vm.Storage),
	}
	return preset
}

func vmItemsToPresetItems(items []ItemViewModel) []PresetItem {
	result := make([]PresetItem, 0, len(items))
	for _, item := range items {
		pi := PresetItem{
			BaseID:         item.BaseID,
			Name:           item.Name,
			Quantity:       item.Quantity,
			CurrentUpgrade: item.CurrentUpgrade,
		}
		if item.ID != item.BaseID && item.MaxUpgrade == 25 {
			diff := item.ID - item.BaseID
			pi.InfuseOffset = (diff / 100) * 100
		}
		result = append(result, pi)
	}
	return result
}

func ValidatePreset(preset *CharacterPreset) []string {
	var warnings []string

	if preset.FormatVersion != PresetFormatVersion {
		warnings = append(warnings,
			fmt.Sprintf("Unsupported format version %d (expected %d)", preset.FormatVersion, PresetFormatVersion))
	}

	c := &preset.Character
	if c.Class > 9 {
		warnings = append(warnings, fmt.Sprintf("Unknown class ID %d", c.Class))
	}

	cs := db.GetClassStats(c.Class)
	if cs != nil {
		type attrCheck struct {
			name string
			val  uint32
			base uint32
		}
		attrs := []attrCheck{
			{"Vigor", c.Vigor, cs.Vigor},
			{"Mind", c.Mind, cs.Mind},
			{"Endurance", c.Endurance, cs.Endurance},
			{"Strength", c.Strength, cs.Strength},
			{"Dexterity", c.Dexterity, cs.Dexterity},
			{"Intelligence", c.Intelligence, cs.Intelligence},
			{"Faith", c.Faith, cs.Faith},
			{"Arcane", c.Arcane, cs.Arcane},
		}
		for _, a := range attrs {
			if a.val < a.base {
				warnings = append(warnings,
					fmt.Sprintf("%s (%d) below %s class base (%d)", a.name, a.val, cs.Name, a.base))
			}
			if a.val > 99 {
				warnings = append(warnings,
					fmt.Sprintf("%s (%d) exceeds maximum (99)", a.name, a.val))
			}
		}
	}

	for i, item := range preset.Inventory {
		validatePresetItem(&item, "inventory", i, &warnings)
	}
	for i, item := range preset.Storage {
		validatePresetItem(&item, "storage", i, &warnings)
	}

	if preset.World != nil {
		validateWorldSummoningPools(preset.World.SummoningPools, &warnings)
		validateWorldRegions(preset.World.Regions, &warnings)
		validateWorldGraces(preset.World.Graces, &warnings)
	}

	return warnings
}

func validatePresetItem(item *PresetItem, location string, idx int, warnings *[]string) {
	itemData, _ := db.GetItemDataFuzzy(item.BaseID)
	if itemData.Name == "" {
		*warnings = append(*warnings,
			fmt.Sprintf("%s[%d]: unknown BaseID 0x%08X — will be skipped", location, idx, item.BaseID))
		return
	}

	if item.CurrentUpgrade > itemData.MaxUpgrade {
		*warnings = append(*warnings,
			fmt.Sprintf("%s[%d] %s: upgrade %d exceeds max %d — will be clamped",
				location, idx, itemData.Name, item.CurrentUpgrade, itemData.MaxUpgrade))
	}

	if item.InfuseOffset > 0 && itemData.MaxUpgrade != 25 {
		*warnings = append(*warnings,
			fmt.Sprintf("%s[%d] %s: infuse offset %d on non-infusable item — will be ignored",
				location, idx, itemData.Name, item.InfuseOffset))
	}

	if item.InfuseOffset > 0 && !validInfuseOffsets[item.InfuseOffset] {
		*warnings = append(*warnings,
			fmt.Sprintf("%s[%d] %s: invalid infuse offset %d — will use Standard",
				location, idx, itemData.Name, item.InfuseOffset))
	}

	if location == "inventory" && itemData.MaxInventory > 0 && item.Quantity > itemData.MaxInventory {
		*warnings = append(*warnings,
			fmt.Sprintf("%s[%d] %s: quantity %d exceeds max %d — will be clamped",
				location, idx, itemData.Name, item.Quantity, itemData.MaxInventory))
	}
	if location == "storage" && itemData.MaxStorage > 0 && item.Quantity > itemData.MaxStorage {
		*warnings = append(*warnings,
			fmt.Sprintf("%s[%d] %s: quantity %d exceeds max %d — will be clamped",
				location, idx, itemData.Name, item.Quantity, itemData.MaxStorage))
	}
}

// validateWorldGraces detects unknown or duplicate Site of Grace EventFlag IDs.
// DLC grace IDs (72xxx, 74xxx) are valid entries in data.Graces and do not
// produce warnings. DoorFlag companion flags are set automatically by
// SetGraceVisited() and are not required in the preset.
func validateWorldGraces(ids []uint32, warnings *[]string) {
	seen := make(map[uint32]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			*warnings = append(*warnings,
				fmt.Sprintf("world.graces: ID %d appears more than once — duplicate will be ignored", id))
			continue
		}
		seen[id] = true
		if !db.IsKnownGraceID(id) {
			*warnings = append(*warnings,
				fmt.Sprintf("world.graces: ID %d not found in grace database", id))
		}
	}
}

// validateWorldRegions detects unknown or duplicate invasion-region IDs.
// Legacy dungeon IDs (1000000–1999999) and DLC region IDs (6900000–6999999)
// are valid entries in data.Regions and do not produce warnings.
func validateWorldRegions(ids []uint32, warnings *[]string) {
	seen := make(map[uint32]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			*warnings = append(*warnings,
				fmt.Sprintf("world.regions: ID %d appears more than once — duplicate will be ignored", id))
			continue
		}
		seen[id] = true
		if !db.IsKnownRegionID(id) {
			*warnings = append(*warnings,
				fmt.Sprintf("world.regions: ID %d not found in region database", id))
		}
	}
}

// validateWorldSummoningPools detects outdated pre-v1.12 pool IDs (1_000_000_000+ range)
// and unknown IDs not present in the current 670xxx database.
func validateWorldSummoningPools(ids []uint32, warnings *[]string) {
	for _, id := range ids {
		if id >= 1_000_000 {
			// Current valid IDs are all in the 670xxx range (< 700000).
			// IDs >= 1_000_000 are pre-v1.12 format (e.g. 10000040, 1035530040).
			*warnings = append(*warnings,
				fmt.Sprintf("world.summoningPools: ID %d is a pre-v1.12 flag — ignored by current game; update preset to use 670xxx IDs", id))
			continue
		}
		if !db.IsKnownSummoningPoolID(id) {
			*warnings = append(*warnings,
				fmt.Sprintf("world.summoningPools: ID %d not found in current database (670xxx range)", id))
		}
	}
}

func PresetItemToFinalID(item PresetItem) uint32 {
	itemData, _ := db.GetItemDataFuzzy(item.BaseID)

	upgrade := item.CurrentUpgrade
	if itemData.Name != "" && upgrade > itemData.MaxUpgrade {
		upgrade = itemData.MaxUpgrade
	}

	var infuse uint32
	if itemData.Name != "" && itemData.MaxUpgrade == 25 && validInfuseOffsets[item.InfuseOffset] {
		infuse = item.InfuseOffset
	}

	return item.BaseID + infuse + upgrade
}

func PresetItemsToItemsToAdd(items []PresetItem, isInventory bool) ([]core.ItemToAdd, []string) {
	var result []core.ItemToAdd
	var warnings []string

	for _, item := range items {
		itemData, _ := db.GetItemDataFuzzy(item.BaseID)
		if itemData.Name == "" {
			warnings = append(warnings,
				fmt.Sprintf("Skipping unknown BaseID 0x%08X", item.BaseID))
			continue
		}

		finalID := PresetItemToFinalID(item)
		qty := item.Quantity

		if isInventory {
			if itemData.MaxInventory > 0 && qty > itemData.MaxInventory {
				qty = itemData.MaxInventory
			}
		} else {
			if itemData.MaxStorage > 0 && qty > itemData.MaxStorage {
				qty = itemData.MaxStorage
			}
		}
		if qty == 0 {
			qty = 1
		}

		handlePrefix := db.ItemIDToHandlePrefix(finalID)
		isStackable := handlePrefix == core.ItemTypeItem || handlePrefix == core.ItemTypeAccessory

		toAdd := core.ItemToAdd{
			ItemID:         finalID,
			ForceStackable: db.IsArrowID(finalID),
			IsStackable:    isStackable,
		}
		if isInventory {
			toAdd.InvQty = int(qty)
		} else {
			toAdd.StorageQty = int(qty)
		}

		result = append(result, toAdd)
	}

	return result, warnings
}

func PresetToVM(preset *CharacterPreset) *CharacterViewModel {
	c := preset.Character
	vm := &CharacterViewModel{
		Name:                c.Name,
		Level:               c.Level,
		Souls:               c.Souls,
		Class:               c.Class,
		ClassName:           c.ClassName,
		Vigor:               c.Vigor,
		Mind:                c.Mind,
		Endurance:           c.Endurance,
		Strength:            c.Strength,
		Dexterity:           c.Dexterity,
		Intelligence:        c.Intelligence,
		Faith:               c.Faith,
		Arcane:              c.Arcane,
		TalismanSlots:     c.TalismanSlots,
		ClearCount:        c.ClearCount,
		MemoryStones:      c.MemoryStones,
		Inventory:         []ItemViewModel{},
		Storage:           []ItemViewModel{},
	}
	return vm
}

func ClearInventoryItems(slot *core.SaveSlot) (int, error) {
	removed := 0

	handles := make([]uint32, 0)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != 0 && item.GaItemHandle != 0xFFFFFFFF {
			handles = append(handles, item.GaItemHandle)
		}
	}

	seen := make(map[uint32]bool)
	for _, h := range handles {
		if seen[h] {
			continue
		}
		seen[h] = true
		if err := core.RemoveItemFromSlot(slot, h, true, false); err != nil {
			return removed, fmt.Errorf("clear inventory handle 0x%08X: %w", h, err)
		}
		removed++
	}

	purgeOrphanedGaItems(slot)
	return removed, nil
}

func ClearStorageItems(slot *core.SaveSlot) (int, error) {
	removed := 0

	handles := make([]uint32, 0)
	for _, item := range slot.Storage.CommonItems {
		if item.GaItemHandle != 0 && item.GaItemHandle != 0xFFFFFFFF {
			handles = append(handles, item.GaItemHandle)
		}
	}

	seen := make(map[uint32]bool)
	for _, h := range handles {
		if seen[h] {
			continue
		}
		seen[h] = true
		if err := core.RemoveItemFromSlot(slot, h, false, true); err != nil {
			return removed, fmt.Errorf("clear storage handle 0x%08X: %w", h, err)
		}
		removed++
	}

	purgeOrphanedGaItems(slot)
	return removed, nil
}

func purgeOrphanedGaItems(slot *core.SaveSlot) {
	for i := range slot.GaItems {
		if slot.GaItems[i].IsEmpty() {
			continue
		}
		h := slot.GaItems[i].Handle
		if _, inMap := slot.GaMap[h]; !inMap {
			slot.GaItems[i] = core.GaItemFull{
				Unk2:            -1,
				Unk3:            -1,
				AoWGaItemHandle: 0xFFFFFFFF,
			}
		}
	}
}

func ExportWorldState(slot *core.SaveSlot) (*WorldPresetData, error) {
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return nil, fmt.Errorf("event flags not available for this slot")
	}
	flags := slot.Data[slot.EventFlagsOffset:]
	world := &WorldPresetData{}

	for _, g := range db.GetAllGraces() {
		if v, err := db.GetEventFlag(flags, g.ID); err == nil && v {
			world.Graces = append(world.Graces, g.ID)
		}
	}
	for _, b := range db.GetAllBosses() {
		if v, err := db.GetEventFlag(flags, b.ID); err == nil && v {
			world.Bosses = append(world.Bosses, b.ID)
		}
	}
	for _, sp := range db.GetAllSummoningPools() {
		if v, err := db.GetEventFlag(flags, sp.ID); err == nil && v {
			world.SummoningPools = append(world.SummoningPools, sp.ID)
		}
	}
	for _, c := range db.GetAllColosseums() {
		if v, err := db.GetEventFlag(flags, c.ID); err == nil && v {
			world.Colosseums = append(world.Colosseums, c.ID)
		}
	}
	for _, m := range db.GetAllMapEntries() {
		if v, err := db.GetEventFlag(flags, m.ID); err == nil && v {
			world.MapFlags = append(world.MapFlags, m.ID)
		}
	}
	for _, cb := range db.GetAllCookbooks() {
		if v, err := db.GetEventFlag(flags, cb.ID); err == nil && v {
			world.Cookbooks = append(world.Cookbooks, cb.ID)
		}
	}
	for _, bb := range db.GetAllBellBearings() {
		if v, err := db.GetEventFlag(flags, bb.ID); err == nil && v {
			world.BellBearings = append(world.BellBearings, bb.ID)
		}
	}
	for _, wb := range db.GetAllWhetblades() {
		if v, err := db.GetEventFlag(flags, wb.ID); err == nil && v {
			world.Whetblades = append(world.Whetblades, wb.ID)
		}
	}

	// World pickups: collect individual flags that are set
	for _, pickupFlags := range data.BolsteringPickupFlags {
		for _, fid := range pickupFlags {
			if v, err := db.GetEventFlag(flags, fid); err == nil && v {
				world.WorldPickups = append(world.WorldPickups, fid)
			}
		}
	}

	// Gestures: read 64 slots, collect non-sentinel IDs
	gestureDataOff := slot.StorageBoxOffset + core.DynStorageBox
	if gestureDataOff+core.DynStorageToGestures <= len(slot.Data) {
		for i := 0; i < 64; i++ {
			off := gestureDataOff + i*4
			gID := binary.LittleEndian.Uint32(slot.Data[off : off+4])
			if gID != data.GestureEmptySentinel && gID != 0 {
				if _, ok := data.LookupGestureBySlotID(gID); ok {
					world.Gestures = append(world.Gestures, gID)
				}
			}
		}
	}

	// Regions
	world.Regions = append(world.Regions, slot.UnlockedRegions...)

	return world, nil
}

func ApplyWorldState(slot *core.SaveSlot, world *WorldPresetData) []string {
	var warnings []string

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return []string{"event flags not available — world data skipped"}
	}
	flags := slot.Data[slot.EventFlagsOffset:]

	setFlag := func(id uint32) {
		if err := db.SetEventFlag(flags, id, true); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to set flag %d: %v", id, err))
		}
	}

	for _, id := range world.Graces {
		setFlag(id)
	}
	for _, id := range world.Bosses {
		setFlag(id)
	}
	for _, id := range world.SummoningPools {
		setFlag(id)
	}
	for _, id := range world.Colosseums {
		setFlag(id)
	}
	for _, id := range world.MapFlags {
		setFlag(id)
	}
	for _, id := range world.Cookbooks {
		setFlag(id)
	}
	for _, id := range world.BellBearings {
		setFlag(id)
	}

	// Whetblades: set main flag + related affinity flags
	for _, id := range world.Whetblades {
		setFlag(id)
		if related, ok := data.WhetbladeRelatedFlags[id]; ok {
			for _, rid := range related {
				setFlag(rid)
			}
		}
	}
	if len(world.Whetblades) > 0 {
		setFlag(data.AoWMenuUnlockedFlag)
	}

	for _, id := range world.WorldPickups {
		setFlag(id)
	}

	// Gestures: write to StorageBox
	gestureDataOff := slot.StorageBoxOffset + core.DynStorageBox
	if gestureDataOff+core.DynStorageToGestures <= len(slot.Data) && len(world.Gestures) > 0 {
		slots := make([]uint32, 64)
		for i := range slots {
			slots[i] = data.GestureEmptySentinel
		}
		for i, gID := range world.Gestures {
			if i >= 64 {
				warnings = append(warnings, "more than 64 gestures in preset — extras skipped")
				break
			}
			slots[i] = gID
		}
		for i, v := range slots {
			off := gestureDataOff + i*4
			binary.LittleEndian.PutUint32(slot.Data[off:off+4], v)
		}
	}

	// Regions
	if len(world.Regions) > 0 {
		if err := core.SetUnlockedRegions(slot, world.Regions); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to set regions: %v", err))
		}
	}

	return warnings
}
