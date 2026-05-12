package vm

import (
	"fmt"
	"math"
	"slices"
	"unicode/utf16"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	gamedata "github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// runesCostForLevel returns the minimum total runes a character must have accumulated
// to reach the given level. Uses the official ER per-level cost formula:
// cost(n) = floor(0.02*n^3 + 3.06*n^2 + 105.6*n − 895), clamped to 0 for low levels.
func runesCostForLevel(level uint32) uint32 {
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

type ItemViewModel struct {
	Handle           uint32   `json:"handle"`
	ID               uint32   `json:"id"`
	BaseID           uint32   `json:"baseId"`
	Name             string   `json:"name"`
	Category         string   `json:"category"`    // broad type from handle prefix: "Weapon"/"Armor"/"Talisman"/"Item"/"Ash of War"
	SubCategory      string   `json:"subCategory"` // main game tab: "tools", "key_items", "melee_armaments", ...
	SubGroup         string   `json:"subGroup"`    // sub-grouping within tab: "Sacred Flasks", "Daggers", ...
	Quantity         uint32   `json:"quantity"`
	MaxInventory     uint32   `json:"maxInventory"`
	MaxStorage       uint32   `json:"maxStorage"`
	MaxUpgrade       uint32   `json:"maxUpgrade"`
	CurrentUpgrade   uint32   `json:"currentUpgrade"`
	IconPath         string   `json:"iconPath"`
	Flags            []string `json:"flags"`
	ReadOnly         bool     `json:"readOnly"`
	AoWID            uint32   `json:"aowId"`       // item ID of the AoW gem attached to this weapon (0 = none / not a weapon)
	CanMountAoW      bool     `json:"canMountAoW"` // true iff gemMountType==2 (standard infusable weapon)
	WepType          uint16   `json:"wepType"`     // weapon category integer from EquipParamWeapon (0 for non-weapons)
	AoWCompatBitmask uint64   `json:"aowCompatBitmask"` // 36-bit canMountWep bitmask (non-zero for AoWs only)
}

type CharacterViewModel struct {
	Name                string                `json:"name"`
	Level               uint32                `json:"level"`
	Souls               uint32                `json:"souls"`
	Class               uint8                 `json:"class"`
	ClassName           string                `json:"className"`
	Vigor               uint32                `json:"vigor"`
	Mind                uint32                `json:"mind"`
	Endurance           uint32                `json:"endurance"`
	Strength            uint32                `json:"strength"`
	Dexterity           uint32                `json:"dexterity"`
	Intelligence        uint32                `json:"intelligence"`
	Faith               uint32                `json:"faith"`
	Arcane              uint32                `json:"arcane"`
	TalismanSlots       uint8                 `json:"talismanSlots"`
	ClearCount          uint32                `json:"clearCount"`
	ScadutreeBlessing   uint8                 `json:"scadutreeBlessing"`
	ShadowRealmBlessing uint8                 `json:"shadowRealmBlessing"`
	MemoryStones        uint32                `json:"memoryStones"`
	Gender              uint8                 `json:"gender"`
	SoulMemory          uint32                `json:"soulMemory"`
	Inventory           []ItemViewModel       `json:"inventory"`
	Storage             []ItemViewModel       `json:"storage"`
	Warnings            []string              `json:"warnings"`
	StatValidation      *StatValidationResult `json:"statValidation,omitempty"`
	EventFlagsAvailable bool                  `json:"eventFlagsAvailable"`
	ClassBaseStats      map[string]uint32     `json:"classBaseStats"`
}

func MapParsedSlotToVM(slot *core.SaveSlot) (*CharacterViewModel, error) {
	data := slot.Player
	vm := &CharacterViewModel{
		Level:               data.Level,
		Souls:               data.Souls,
		SoulMemory:          data.SoulMemory,
		Class:               data.Class,
		Vigor:               data.Vigor,
		Mind:                data.Mind,
		Endurance:           data.Endurance,
		Strength:            data.Strength,
		Dexterity:           data.Dexterity,
		Intelligence:        data.Intelligence,
		Faith:               data.Faith,
		Arcane:              data.Arcane,
		TalismanSlots:       data.TalismanSlots,
		ClearCount:          data.ClearCount,
		ScadutreeBlessing:   data.ScadutreeBlessing,
		ShadowRealmBlessing: data.ShadowRealmBlessing,
		Gender:              data.Gender,
		Inventory:           []ItemViewModel{},
		Storage:             []ItemViewModel{},
	}

	vm.Name = core.UTF16ToString(data.CharacterName[:])
	vm.Warnings = slot.Warnings
	vm.EventFlagsAvailable = slot.EventFlagsOffset > 0

	// Set class name and base stats
	cs := db.GetClassStats(data.Class)
	if cs != nil {
		vm.ClassName = cs.Name
		vm.ClassBaseStats = map[string]uint32{
			"vigor":        cs.Vigor,
			"mind":         cs.Mind,
			"endurance":    cs.Endurance,
			"strength":     cs.Strength,
			"dexterity":    cs.Dexterity,
			"intelligence": cs.Intelligence,
			"faith":        cs.Faith,
			"arcane":       cs.Arcane,
		}
	} else {
		vm.ClassName = fmt.Sprintf("Unknown (%d)", data.Class)
		vm.ClassBaseStats = map[string]uint32{}
	}

	// Run stat consistency validation
	validation := vm.ValidateStatsConsistency(data.Class)
	vm.StatValidation = &validation

	// Build handle→GaItemFull index to resolve per-weapon AoW handles.
	gaItemsByHandle := make(map[uint32]core.GaItemFull, len(slot.GaItems))
	for _, gi := range slot.GaItems {
		if !gi.IsEmpty() {
			gaItemsByHandle[gi.Handle] = gi
		}
	}

	// Map Inventory
	vm.Inventory = mapItems(slot.Inventory, slot.GaMap, gaItemsByHandle)

	// Map Storage
	vm.Storage = mapItems(slot.Storage, slot.GaMap, gaItemsByHandle)

	// Populate MemoryStones — addToInventory writes to CommonItems; stones obtained
	// in-game may reside in KeyItems. Scan both to handle either case.
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == 0xB000272E {
			vm.MemoryStones = item.Quantity & 0x7FFFFFFF
			break
		}
	}
	if vm.MemoryStones == 0 {
		for _, item := range slot.Inventory.KeyItems {
			if item.GaItemHandle == 0xB000272E {
				vm.MemoryStones = item.Quantity & 0x7FFFFFFF
				break
			}
		}
	}

	return vm, nil
}

func mapItems(data core.EquipInventoryData, gaMap map[uint32]uint32, gaItemsByHandle map[uint32]core.GaItemFull) []ItemViewModel {
	items := []ItemViewModel{}

	processItem := func(item core.InventoryItem) {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			return
		}

		category := db.GetItemCategoryFromHandle(item.GaItemHandle)
		var itemID uint32
		var ok bool

		if category == "Weapon" || category == "Armor" || category == "Ash of War" {
			// For Weapons, Armor, and AoW, we MUST use the GaMap to find the real ItemID.
			itemID, ok = gaMap[item.GaItemHandle]
		} else if category != "Unknown" {
			// For stackable items (Talisman=0xA0, Goods=0xB0), handle encodes the item ID
			// with handle prefix. Convert back to DB-compatible item ID prefix:
			// 0xA0→0x20 (talisman), 0xB0→0x40 (goods).
			itemID = db.HandleToItemID(item.GaItemHandle)
			ok = true
		}

		if ok {
			// Filter Unarmed and Empty
			if itemID == 0 || itemID == 110000 {
				return
			}

			itemData, baseID := db.GetItemDataFuzzy(itemID)
			name := itemData.Name

			// Strict filtering: skip items that are not in our database (Unknown)
			// to avoid garbage data from misaligned offsets.
			if name == "" {
				return
			}

			var currentUpgrade uint32
			if baseID != itemID && itemID > baseID {
				diff := itemID - baseID
				// Strip infusion offset (multiples of 100: Heavy=100, Keen=200, ..., Occult=1200)
				// to get the actual upgrade level (0-25)
				currentUpgrade = diff % 100
			}

			displayQuantity := item.Quantity
			// For non-stackable items, force quantity to 1.
			// Exception: arrows/bolts have weapon-like handles (0x82...) but are stackable.
			isArrow := itemData.Category == "arrows_and_bolts"
			if (category == "Weapon" || category == "Armor" || category == "Talisman" || category == "Ash of War") && !isArrow {
				displayQuantity = 1
			} else {
				// For stackable items, mask the high bit which is often used by the engine
				displayQuantity = item.Quantity & 0x7FFFFFFF
			}

			// Resolve the AoW gem attached to this weapon instance.
			// AoWGaItemHandle == 0xFFFFFFFF means no AoW gem is applied.
			var aowID uint32
			if category == "Weapon" {
				if gi, ok2 := gaItemsByHandle[item.GaItemHandle]; ok2 && gi.AoWGaItemHandle != 0xFFFFFFFF {
					if aowItemID, ok3 := gaMap[gi.AoWGaItemHandle]; ok3 && aowItemID != 0 {
						aowID = aowItemID
					}
				}
			}

			items = append(items, ItemViewModel{
				Handle:           item.GaItemHandle,
				ID:               itemID,
				BaseID:           baseID,
				Name:             name,
				Category:         category,
				SubCategory:      db.GetItemSubCategory(itemID, itemData, category),
				SubGroup:         itemData.SubCategory,
				Quantity:         displayQuantity,
				MaxInventory:     itemData.MaxInventory,
				MaxStorage:       itemData.MaxStorage,
				MaxUpgrade:       itemData.MaxUpgrade,
				CurrentUpgrade:   currentUpgrade,
				IconPath:         itemData.IconPath,
				Flags:            itemData.Flags,
				ReadOnly:         gamedata.IsCookbookItemID(itemID) || gamedata.IsWhetbladeItemID(itemID) || gamedata.IsBellBearingItemID(itemID) || slices.Contains(itemData.Flags, "no_database"),
				AoWID:            aowID,
				CanMountAoW:      itemData.GemMountType == 2,
				WepType:          itemData.WepType,
				AoWCompatBitmask: itemData.AoWCompatBitmask,
			})
		}
	}

	// Common Items
	for _, item := range data.CommonItems {
		processItem(item)
	}

	// Key Items
	for _, item := range data.KeyItems {
		processItem(item)
	}

	return items
}
func ApplyVMToParsedSlot(vm *CharacterViewModel, slot *core.SaveSlot) error {
	data := &slot.Player
	data.Level = vm.Level
	data.Class = vm.Class
	data.Souls = vm.Souls
	data.SoulMemory = vm.SoulMemory
	if minSM := runesCostForLevel(data.Level); data.SoulMemory < minSM {
		data.SoulMemory = minSM
		vm.SoulMemory = minSM
	}
	data.Vigor = vm.Vigor
	data.Mind = vm.Mind
	data.Endurance = vm.Endurance
	data.Strength = vm.Strength
	data.Dexterity = vm.Dexterity
	data.Intelligence = vm.Intelligence
	data.Faith = vm.Faith
	data.Arcane = vm.Arcane
	if vm.TalismanSlots > 3 {
		vm.TalismanSlots = 3
	}
	data.TalismanSlots = vm.TalismanSlots
	if vm.ClearCount > 7 {
		vm.ClearCount = 7
	}
	data.ClearCount = vm.ClearCount
	data.ScadutreeBlessing = vm.ScadutreeBlessing
	data.ShadowRealmBlessing = vm.ShadowRealmBlessing

	u16 := utf16.Encode([]rune(vm.Name))
	for i := 0; i < 16; i++ {
		if i < len(u16) {
			data.CharacterName[i] = u16[i]
		} else {
			data.CharacterName[i] = 0
		}
	}

	// Update Inventory (with write-back to slot.Data)
	if err := updateItemsAndSync(vm.Inventory, &slot.Inventory, slot, false); err != nil {
		return fmt.Errorf("inventory sync failed: %w", err)
	}

	// Update Storage (with write-back to slot.Data)
	if err := updateItemsAndSync(vm.Storage, &slot.Storage, slot, true); err != nil {
		return fmt.Errorf("storage sync failed: %w", err)
	}

	return nil
}

// updateItemsAndSync writes quantity changes from VM items back to slot.Data.
// It operates on a snapshot of slot.Data: if any write fails, the original is preserved (rollback).
// Uses SlotAccessor for bounds-checked writes instead of raw binary.LittleEndian.
func updateItemsAndSync(vmItems []ItemViewModel, data *core.EquipInventoryData, slot *core.SaveSlot, isStorage bool) error {
	vmMap := make(map[uint32]ItemViewModel)
	for _, item := range vmItems {
		vmMap[item.Handle] = item
	}

	var commonStart int
	if isStorage {
		commonStart = slot.StorageBoxOffset + 4
	} else {
		commonStart = slot.MagicOffset + 505
	}

	// Phase 1: pre-validate all write offsets before modifying anything.
	sa := core.NewSlotAccessor(slot.Data)
	for i := range data.CommonItems {
		handle := data.CommonItems[i].GaItemHandle
		if handle == 0 || handle == 0xFFFFFFFF {
			continue
		}
		if _, ok := vmMap[handle]; ok {
			off := commonStart + i*12 + 4
			if err := sa.CheckBounds(off, 4, "common item qty"); err != nil {
				return err
			}
		}
	}
	if !isStorage {
		keyStart := commonStart + 0xa80*12
		for i := range data.KeyItems {
			handle := data.KeyItems[i].GaItemHandle
			if handle == 0 || handle == 0xFFFFFFFF {
				continue
			}
			if _, ok := vmMap[handle]; ok {
				off := keyStart + i*12 + 4
				if err := sa.CheckBounds(off, 4, "key item qty"); err != nil {
					return err
				}
			}
		}
	}

	// Phase 2: snapshot slot.Data, apply writes to the copy.
	snapshot := make([]byte, len(slot.Data))
	copy(snapshot, slot.Data)
	ssa := core.NewSlotAccessor(snapshot)

	for i := range data.CommonItems {
		handle := data.CommonItems[i].GaItemHandle
		if handle == 0 || handle == 0xFFFFFFFF {
			continue
		}
		if vmItem, ok := vmMap[handle]; ok {
			qty := vmItem.Quantity
			if isStorage {
				if vmItem.MaxStorage > 0 && qty > vmItem.MaxStorage {
					qty = vmItem.MaxStorage
				}
			} else {
				if vmItem.MaxInventory > 0 && qty > vmItem.MaxInventory {
					qty = vmItem.MaxInventory
				}
			}
			data.CommonItems[i].Quantity = qty
			off := commonStart + i*12 + 4
			if err := ssa.WriteU32(off, qty); err != nil {
				return fmt.Errorf("common item %d write failed: %w", i, err)
			}
		}
	}

	if !isStorage {
		keyStart := commonStart + 0xa80*12
		for i := range data.KeyItems {
			handle := data.KeyItems[i].GaItemHandle
			if handle == 0 || handle == 0xFFFFFFFF {
				continue
			}
			if vmItem, ok := vmMap[handle]; ok {
				qty := vmItem.Quantity
				if vmItem.MaxInventory > 0 && qty > vmItem.MaxInventory {
					qty = vmItem.MaxInventory
				}
				data.KeyItems[i].Quantity = qty
				off := keyStart + i*12 + 4
				if err := ssa.WriteU32(off, qty); err != nil {
					return fmt.Errorf("key item %d write failed: %w", i, err)
				}
			}
		}
	}

	// Phase 3: all writes succeeded — commit snapshot to slot.Data.
	copy(slot.Data, snapshot)
	return nil
}

func MapSlotToVM(slotData []byte) (*CharacterViewModel, error) {
	r := core.NewReader(slotData)
	slot := &core.SaveSlot{}
	if err := slot.Read(r, "PC"); err != nil {
		return nil, err
	}
	return MapParsedSlotToVM(slot)
}

func ApplyVMToSlot(vm *CharacterViewModel, slotData []byte) error {
	r := core.NewReader(slotData)
	slot := &core.SaveSlot{}
	if err := slot.Read(r, "PC"); err != nil {
		return err
	}
	if err := ApplyVMToParsedSlot(vm, slot); err != nil {
		return err
	}
	updated := slot.Write("PC")
	copy(slotData, updated)
	return nil
}
