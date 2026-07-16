package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// Snapshot events carry aggregate, privacy-safe save state. They are debug
// records, so they are durable only while Debug Mode is enabled.
const (
	eventSaveStateLoaded      = "save_state_loaded"
	eventSaveStateBeforeWrite = "save_state_before_write"
	eventSaveStateWritten     = "save_state_written"
)

// diagnosticSaveSnapshot builds the safe state summary for one exact save
// state. The caller must hold saveMu (read or write) while calling it; no
// journal operation happens here, so callers can release all app locks before
// appending the returned fields.
//
// The snapshot deliberately contains only aggregate layout and capacity data:
// no character names, Steam IDs, item IDs, item names, raw slot bytes, paths,
// or parse-warning text. Per-slot usage is still enough to investigate a
// capacity, layout, or serialization report without asking for the save file.
func diagnosticSaveSnapshot(save *core.SaveFile, generation uint64) []diagnosticField {
	if save == nil {
		return nil
	}

	activeSlots := 0
	populatedSlots := 0
	residualSlots := 0
	parseWarnings := 0
	fields := make([]diagnosticField, 0, 5+len(save.Slots))
	for i := range save.Slots {
		slot := &save.Slots[i]
		state := "empty"
		if save.ActiveSlots[i] {
			state = "active"
			activeSlots++
		} else if save.SlotHasResidualData(i) {
			state = "residual"
			residualSlots++
		}
		if slot.Version != 0 {
			populatedSlots++
		}
		parseWarnings += len(slot.Warnings)

		usage := core.CountSlotUsage(slot)
		fields = append(fields, field(fmt.Sprintf("slot_%d", i), fmt.Sprintf(
			"state=%s,version=%d,ga_items=%d/%d,ga_item_data=%d/%d,inventory=%d/%d,storage=%d/%d,warnings=%d",
			state,
			slot.Version,
			usage.GaItemsUsed,
			usage.GaItemsMax,
			usage.GaItemDataUsed,
			usage.GaItemDataMax,
			usage.InventoryUsed,
			usage.InventoryMax,
			usage.StorageUsed,
			usage.StorageMax,
			len(slot.Warnings),
		)))
	}

	return append([]diagnosticField{
		field("platform", string(save.Platform)),
		field("save_generation", fmt.Sprintf("%d", generation)),
		field("active_slots", fmt.Sprintf("%d", activeSlots)),
		field("populated_slots", fmt.Sprintf("%d", populatedSlots)),
		field("residual_slots", fmt.Sprintf("%d", residualSlots)),
		field("parse_warnings", fmt.Sprintf("%d", parseWarnings)),
	}, fields...)
}

// diagnosticSnapshotForSerialization adds the transport purpose after the
// snapshot has been captured under the save lock. It returns a new slice so
// the captured fields remain reusable by a caller handling an error.
func diagnosticSnapshotForSerialization(snapshot []diagnosticField, serialization string) []diagnosticField {
	if len(snapshot) == 0 {
		return nil
	}
	fields := make([]diagnosticField, 0, len(snapshot)+1)
	fields = append(fields, field("serialization", serialization))
	return append(fields, snapshot...)
}
