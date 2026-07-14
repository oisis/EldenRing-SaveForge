package core

// SlotSnapshot holds a deep copy of all mutable SaveSlot state for rollback.
type SlotSnapshot struct {
	Data                  []byte
	Version               uint32
	Player                PlayerGameData
	GaMap                 map[uint32]uint32
	GaItems               []GaItemFull
	Inventory             EquipInventoryData
	Storage               EquipInventoryData
	SteamID               uint64
	Warnings              []string
	MagicOffset           int
	InventoryEnd          int
	EventFlagsOffset      int
	PlayerDataOffset      int
	FaceDataOffset        int
	StorageBoxOffset      int
	IngameTimerOffset     int
	GaItemDataOffset      int
	TutorialDataOffset    int
	ClearCountOffset      int
	EquipItemsIDOffset    int
	EquippedSpellsOffset  int
	UnlockedRegionsOffset int
	UnlockedRegions       []uint32
	SectionMap            []SectionRange
	NextAoWIndex          int
	NextArmamentIndex     int
	NextGaItemHandle      uint32
	PartGaItemHandle      uint8
}

// SnapshotSlot creates a deep copy of all mutable slot state.
func SnapshotSlot(slot *SaveSlot) SlotSnapshot {
	dataCopy := make([]byte, len(slot.Data))
	copy(dataCopy, slot.Data)

	var gaMapCopy map[uint32]uint32
	if slot.GaMap != nil {
		gaMapCopy = make(map[uint32]uint32, len(slot.GaMap))
		for k, v := range slot.GaMap {
			gaMapCopy[k] = v
		}
	}

	var gaItemsCopy []GaItemFull
	if slot.GaItems != nil {
		gaItemsCopy = make([]GaItemFull, len(slot.GaItems))
		copy(gaItemsCopy, slot.GaItems)
	}

	return SlotSnapshot{
		Data:                  dataCopy,
		Version:               slot.Version,
		Player:                slot.Player,
		GaMap:                 gaMapCopy,
		GaItems:               gaItemsCopy,
		Inventory:             slot.Inventory.Clone(),
		Storage:               slot.Storage.Clone(),
		SteamID:               slot.SteamID,
		Warnings:              append([]string(nil), slot.Warnings...),
		MagicOffset:           slot.MagicOffset,
		InventoryEnd:          slot.InventoryEnd,
		EventFlagsOffset:      slot.EventFlagsOffset,
		PlayerDataOffset:      slot.PlayerDataOffset,
		FaceDataOffset:        slot.FaceDataOffset,
		StorageBoxOffset:      slot.StorageBoxOffset,
		IngameTimerOffset:     slot.IngameTimerOffset,
		GaItemDataOffset:      slot.GaItemDataOffset,
		TutorialDataOffset:    slot.TutorialDataOffset,
		ClearCountOffset:      slot.ClearCountOffset,
		EquipItemsIDOffset:    slot.EquipItemsIDOffset,
		EquippedSpellsOffset:  slot.EquippedSpellsOffset,
		UnlockedRegionsOffset: slot.UnlockedRegionsOffset,
		UnlockedRegions:       append([]uint32(nil), slot.UnlockedRegions...),
		SectionMap:            append([]SectionRange(nil), slot.SectionMap...),
		NextAoWIndex:          slot.NextAoWIndex,
		NextArmamentIndex:     slot.NextArmamentIndex,
		NextGaItemHandle:      slot.NextGaItemHandle,
		PartGaItemHandle:      slot.PartGaItemHandle,
	}
}

// RestoreSlot overwrites all mutable slot state from a snapshot.
func RestoreSlot(slot *SaveSlot, snap SlotSnapshot) {
	slot.Data = append([]byte(nil), snap.Data...)
	slot.Version = snap.Version
	slot.Player = snap.Player

	if snap.GaMap == nil {
		slot.GaMap = nil
	} else {
		slot.GaMap = make(map[uint32]uint32, len(snap.GaMap))
		for k, v := range snap.GaMap {
			slot.GaMap[k] = v
		}
	}

	if snap.GaItems != nil {
		slot.GaItems = make([]GaItemFull, len(snap.GaItems))
		copy(slot.GaItems, snap.GaItems)
	} else {
		slot.GaItems = nil
	}

	slot.Inventory = snap.Inventory.Clone()
	slot.Storage = snap.Storage.Clone()
	slot.SteamID = snap.SteamID
	slot.Warnings = append([]string(nil), snap.Warnings...)
	slot.MagicOffset = snap.MagicOffset
	slot.InventoryEnd = snap.InventoryEnd
	slot.EventFlagsOffset = snap.EventFlagsOffset
	slot.PlayerDataOffset = snap.PlayerDataOffset
	slot.FaceDataOffset = snap.FaceDataOffset
	slot.StorageBoxOffset = snap.StorageBoxOffset
	slot.IngameTimerOffset = snap.IngameTimerOffset
	slot.GaItemDataOffset = snap.GaItemDataOffset
	slot.TutorialDataOffset = snap.TutorialDataOffset
	slot.ClearCountOffset = snap.ClearCountOffset
	slot.EquipItemsIDOffset = snap.EquipItemsIDOffset
	slot.EquippedSpellsOffset = snap.EquippedSpellsOffset
	slot.UnlockedRegionsOffset = snap.UnlockedRegionsOffset
	slot.UnlockedRegions = append([]uint32(nil), snap.UnlockedRegions...)
	slot.SectionMap = append([]SectionRange(nil), snap.SectionMap...)
	slot.NextAoWIndex = snap.NextAoWIndex
	slot.NextArmamentIndex = snap.NextArmamentIndex
	slot.NextGaItemHandle = snap.NextGaItemHandle
	slot.PartGaItemHandle = snap.PartGaItemHandle
}

// CloneSlot returns a deep, independent copy of slot. It is intended for
// candidate-only mutations: callers can run a transaction on the clone and
// publish it only after all validation succeeds.
func CloneSlot(slot *SaveSlot) *SaveSlot {
	if slot == nil {
		return nil
	}
	clone := &SaveSlot{}
	RestoreSlot(clone, SnapshotSlot(slot))
	return clone
}
