package editor

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// WorkspaceValidationReport summarizes the result of Validate.
//
// OK == false signals at least one Error; warnings never flip OK to
// false. Pass-through records are always surfaced as a warning so the
// UI can show "N items will round-trip unchanged" without blocking save.
type WorkspaceValidationReport struct {
	OK                        bool                       `json:"ok"`
	Errors                    []WorkspaceValidationIssue `json:"errors"`
	Warnings                  []WorkspaceValidationIssue `json:"warnings"`
	InventoryItemCount        int                        `json:"inventoryItemCount"`
	StorageItemCount          int                        `json:"storageItemCount"`
	UnsupportedInventoryCount int                        `json:"unsupportedInventoryCount"`
	UnsupportedStorageCount   int                        `json:"unsupportedStorageCount"`
	DuplicateUIDs             []string                   `json:"duplicateUIDs"`
	DuplicateHandles          []uint32                   `json:"duplicateHandles"`
}

// WorkspaceValidationIssue is one row in Errors or Warnings.
type WorkspaceValidationIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	UID      string `json:"uid,omitempty"`
	Handle   uint32 `json:"handle,omitempty"`
}

// Severity constants.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Validation issue codes. Frontend / tests use these as stable keys.
const (
	CodeDuplicateUID            = "duplicate_uid"
	CodeDuplicateHandle         = "duplicate_handle"
	CodeUnknownItemID           = "unknown_item_id"
	CodeQuantityZero            = "quantity_zero"
	CodeUpgradeOutOfRange       = "upgrade_out_of_range"
	CodeCategoryUnsupported     = "category_unsupported"
	CodePassThroughRecords      = "pass_through_records"
	CodeSharedAoWConflict       = "shared_aow_conflict"
	CodeSharedAoWNotChecked     = "shared_aow_not_checked"
	CodePendingAoWUnknown       = "pending_aow_unknown"
	CodePendingAoWConflict      = "pending_aow_conflict"
	CodeCurrentAoWMissing       = "current_aow_missing"
	CodeCurrentAoWShared        = "current_aow_shared"
	CodeCurrentAoWNonAoWCategory = "current_aow_non_aow_category"
)

// Validate runs Phase 1 dry-run checks. It does not touch the slot.
//
// Errors:
//   - duplicate UIDs across editable items
//   - duplicate non-zero handles across editable items
//   - unknown itemID / baseItemID (not in DB)
//   - quantity == 0
//   - CurrentUpgrade outside [0, MaxUpgrade]
//   - editable category outside SupportedCategories (defensive — builder
//     should have caught this already)
//
// Warnings:
//   - pass-through records present (count surfaced)
//   - shared-AoW conflict detection not yet wired in Phase 1
func Validate(snap InventoryWorkspaceSnapshot) WorkspaceValidationReport {
	rep := WorkspaceValidationReport{
		OK:                        true,
		InventoryItemCount:        len(snap.InventoryItems),
		StorageItemCount:          len(snap.StorageItems),
		UnsupportedInventoryCount: len(snap.UnsupportedInventoryRecords),
		UnsupportedStorageCount:   len(snap.UnsupportedStorageRecords),
		Errors:                    []WorkspaceValidationIssue{},
		Warnings:                  []WorkspaceValidationIssue{},
		DuplicateUIDs:             []string{},
		DuplicateHandles:          []uint32{},
	}

	uidSeen := make(map[string]bool)
	handleSeen := make(map[uint32]bool)
	dupUIDSet := make(map[string]bool)
	dupHandleSet := make(map[uint32]bool)

	check := func(items []EditableItem) {
		for _, it := range items {
			if uidSeen[it.UID] {
				if !dupUIDSet[it.UID] {
					dupUIDSet[it.UID] = true
					rep.DuplicateUIDs = append(rep.DuplicateUIDs, it.UID)
					rep.Errors = append(rep.Errors, WorkspaceValidationIssue{
						Severity: SeverityError,
						Code:     CodeDuplicateUID,
						Message:  fmt.Sprintf("duplicate UID %s", it.UID),
						UID:      it.UID,
					})
				}
			}
			uidSeen[it.UID] = true

			if it.OriginalHandle != 0 {
				if handleSeen[it.OriginalHandle] {
					if !dupHandleSet[it.OriginalHandle] {
						dupHandleSet[it.OriginalHandle] = true
						rep.DuplicateHandles = append(rep.DuplicateHandles, it.OriginalHandle)
						rep.Errors = append(rep.Errors, WorkspaceValidationIssue{
							Severity: SeverityError,
							Code:     CodeDuplicateHandle,
							Message:  fmt.Sprintf("duplicate handle 0x%08X", it.OriginalHandle),
							UID:      it.UID,
							Handle:   it.OriginalHandle,
						})
					}
				}
				handleSeen[it.OriginalHandle] = true
			}

			if it.Name == "" || it.ItemID == 0 || it.BaseItemID == 0 {
				rep.Errors = append(rep.Errors, WorkspaceValidationIssue{
					Severity: SeverityError,
					Code:     CodeUnknownItemID,
					Message:  fmt.Sprintf("itemID 0x%08X unknown in DB", it.ItemID),
					UID:      it.UID,
					Handle:   it.OriginalHandle,
				})
			}

			if it.Quantity == 0 {
				rep.Errors = append(rep.Errors, WorkspaceValidationIssue{
					Severity: SeverityError,
					Code:     CodeQuantityZero,
					Message:  fmt.Sprintf("quantity must be > 0 (item %s)", it.Name),
					UID:      it.UID,
					Handle:   it.OriginalHandle,
				})
			}

			if it.IsWeapon && it.MaxUpgrade > 0 {
				if it.CurrentUpgrade < 0 || it.CurrentUpgrade > it.MaxUpgrade {
					rep.Errors = append(rep.Errors, WorkspaceValidationIssue{
						Severity: SeverityError,
						Code:     CodeUpgradeOutOfRange,
						Message: fmt.Sprintf("upgrade %d outside [0,%d] for %s",
							it.CurrentUpgrade, it.MaxUpgrade, it.Name),
						UID:    it.UID,
						Handle: it.OriginalHandle,
					})
				}
			}

			if !SupportedCategories[it.Category] {
				rep.Errors = append(rep.Errors, WorkspaceValidationIssue{
					Severity: SeverityError,
					Code:     CodeCategoryUnsupported,
					Message: fmt.Sprintf("category %q is not editable in Phase 1 (item %s)",
						it.Category, it.Name),
					UID:    it.UID,
					Handle: it.OriginalHandle,
				})
			}

			// Pending AoW (Phase 1.7): defense-in-depth check. UpdateWeapon
			// already validates the AoW ID on accept, but anything that
			// bypasses it (direct mutation, future bugs) must not save.
			if it.PendingAoWItemID != 0 {
				aow, _ := db.GetItemDataFuzzy(it.PendingAoWItemID)
				if aow.Name == "" || aow.Category != "ashes_of_war" {
					rep.Errors = append(rep.Errors, WorkspaceValidationIssue{
						Severity: SeverityError,
						Code:     CodePendingAoWUnknown,
						Message: fmt.Sprintf("pending AoW 0x%08X is not a known ashes_of_war item",
							it.PendingAoWItemID),
						UID:    it.UID,
						Handle: it.OriginalHandle,
					})
				}
			}

			// Conflicting pending AoW intent (Phase 4B). PendingAoWClear
			// is mutually exclusive with PendingAoWItemID != 0 —
			// UpdateWeapon enforces this on accept, but defense-in-depth
			// catches any path that bypasses it (direct field mutation,
			// future bugs).
			if it.PendingAoWClear && it.PendingAoWItemID != 0 {
				rep.Errors = append(rep.Errors, WorkspaceValidationIssue{
					Severity: SeverityError,
					Code:     CodePendingAoWConflict,
					Message: fmt.Sprintf("conflicting pending AoW state: clear=true AND set=0x%08X (item %s)",
						it.PendingAoWItemID, it.Name),
					UID:    it.UID,
					Handle: it.OriginalHandle,
				})
			}

			// Current AoW (Phase 4A): read-side anomalies surface as
			// warnings only. Existing saves may carry these states; we
			// want the UI to display them without blocking save. A later
			// phase (4B) will decide whether to repair or refuse.
			switch it.CurrentAoWStatus {
			case AoWStatusMissing:
				rep.Warnings = append(rep.Warnings, WorkspaceValidationIssue{
					Severity: SeverityWarning,
					Code:     CodeCurrentAoWMissing,
					Message: fmt.Sprintf("weapon %s references AoW handle 0x%08X but no GaItem maps to it (dangling AoW)",
						it.Name, it.CurrentAoWHandle),
					UID:    it.UID,
					Handle: it.OriginalHandle,
				})
			case AoWStatusShared:
				rep.Warnings = append(rep.Warnings, WorkspaceValidationIssue{
					Severity: SeverityWarning,
					Code:     CodeCurrentAoWShared,
					Message: fmt.Sprintf("weapon %s shares AoW handle 0x%08X with another weapon (save corruption indicator)",
						it.Name, it.CurrentAoWHandle),
					UID:    it.UID,
					Handle: it.OriginalHandle,
				})
			case AoWStatusCustom:
				if it.CurrentAoWItemID != 0 {
					aow, _ := db.GetItemDataFuzzy(it.CurrentAoWItemID)
					if aow.Name != "" && aow.Category != "ashes_of_war" {
						rep.Warnings = append(rep.Warnings, WorkspaceValidationIssue{
							Severity: SeverityWarning,
							Code:     CodeCurrentAoWNonAoWCategory,
							Message: fmt.Sprintf("weapon %s current AoW 0x%08X resolves to %q (category %q, not ashes_of_war)",
								it.Name, it.CurrentAoWItemID, aow.Name, aow.Category),
							UID:    it.UID,
							Handle: it.OriginalHandle,
						})
					}
				}
			}
		}
	}

	check(snap.InventoryItems)
	check(snap.StorageItems)

	totalPassthrough := rep.UnsupportedInventoryCount + rep.UnsupportedStorageCount
	if totalPassthrough > 0 {
		rep.Warnings = append(rep.Warnings, WorkspaceValidationIssue{
			Severity: SeverityWarning,
			Code:     CodePassThroughRecords,
			Message: fmt.Sprintf("%d records will round-trip unchanged (unsupported categories, technical placeholders, or duplicate handles)",
				totalPassthrough),
		})
	}

	// Phase 4A landed per-item shared-AoW detection via the
	// CurrentAoWStatus=AoWStatusShared path (CodeCurrentAoWShared
	// warning per weapon). The legacy global deferral warning
	// (CodeSharedAoWNotChecked) is no longer emitted — keep the
	// constant for backward-compatible JSON parsing on the frontend.
	_ = CodeSharedAoWNotChecked

	if len(rep.Errors) > 0 {
		rep.OK = false
	}
	return rep
}
