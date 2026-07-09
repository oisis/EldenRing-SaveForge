# Changelog

## [Unreleased]

### Added
- **On-load inventory issues modal**: automatically scans the selected character slot on save load;
  shows workspace errors/warnings with grouped checkboxes, individual Fix buttons, Fix-all per group,
  and a Repair Selected batch action. Skip path shows a toast pointing to Tools → Diagnostic.
- **`ScanInventoryIssues` App endpoint**: unified scan combining binary corruption check
  (`core.DiagnoseSaveCorruption`) and workspace semantic validation (`editor.Validate`).
  Returns `InventoryIssuesScanReport` with repair metadata per issue.
- **`RepairInventoryWorkspaceItem` / `RepairInventoryWorkspaceItems` App endpoints**: apply
  auto-repairs for `upgrade_out_of_range`, `pending_aow_unknown`, `pending_aow_conflict` and
  immediately commit via `SaveInventoryWorkspaceChanges`.
- **`editor.AutoRepairWorkspaceItem`**: reusable repair engine in the editor package.
- **`Repair auto-fixable` button in Inventory → Weapons & Sort Order**: visible in the
  ValidationPanel whenever auto-repairable issues are present; applies all in one workspace save.
- **Tools → Diagnostic** now includes weapon workspace issues (`upgrade_out_of_range`, etc.)
  alongside binary scan results (category: `weapons`).

### Removed
- **`inventory_reserved` scanner rule**: removed the false-positive corruption check that
  flagged any record with `AcquisitionIndex <= 432`. Genuine game-created records legitimately
  use low indices (e.g. Memory of Grace at 432, Lordsworn weapons/shields). `InvEquipReservedMax`
  is retained purely as a conservative floor for newly generated editor indices — it is not a
  validation rule for existing records. Duplicate-index detection (`duplicate_acquisition_index`)
  is unaffected. Dropped the now-dead `RepairCodeInventoryReserved` constant, its app-layer action
  mapping, and the UI label; writer semantics are unchanged.

### Fixed
- Repair scanner no longer reports false-positive `unknown_item_id` for five legal technical
  variant GoodsParam rows (`0x40002AFA`, `0x40002AFC`, `0x40002B08`, `0x40001FAD`, `0x40001FD2`).
  These share their base item's params and are now registered in the item DB (flagged
  `no_database`, kept out of the item picker).
- Workspace validation issues (`upgrade_out_of_range`, `pending_aow_unknown`, `pending_aow_conflict`)
  are now surfaced in Tools → Diagnostic, resolving the gap between the two separate scan systems.
- DiagnosticsModal "No repairable issues found" state now correctly considers all displayable
  categories including `weapons` — not only binary-repairable ones.
