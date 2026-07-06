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

### Fixed
- Workspace validation issues (`upgrade_out_of_range`, `pending_aow_unknown`, `pending_aow_conflict`)
  are now surfaced in Tools → Diagnostic, resolving the gap between the two separate scan systems.
- DiagnosticsModal "No repairable issues found" state now correctly considers all displayable
  categories including `weapons` — not only binary-repairable ones.
