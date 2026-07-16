package main

import (
	"strconv"
	"strings"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// diagnosticPostLoadItemsMax bounds modal and repair summaries. The journal is
// intended to explain a problem without becoming a second copy of a malformed
// save or an unbounded UI payload.
const diagnosticPostLoadItemsMax = 20

// RecordDiagnosticPostLoadDiagnosticsModalShown records the exact class of
// issues that caused the post-load Diagnostics modal to open. It takes no
// renderer payload: the backend re-scans the active save and logs only a
// bounded, privacy-safe summary after RunDiagnosticsAllLoaded has released all
// save and slot locks.
func (a *App) RecordDiagnosticPostLoadDiagnosticsModalShown() {
	report, err := a.RunDiagnosticsAllLoaded()
	if err != nil || !report.CanRepair {
		return
	}

	a.journalLog(levelInfo, "post_load_diagnostics_modal_shown",
		"post-load diagnostics repair modal shown", diagnosticPostLoadIssueFields(report)...)
}

func diagnosticPostLoadIssueFields(report DiagnosticsReport) []diagnosticField {
	affectedSlots := 0
	issues := make([]string, 0, diagnosticPostLoadItemsMax)
	total, repairable, critical, warning := 0, 0, 0, 0

	for _, slot := range report.Slots {
		slotHasVisibleIssue := false
		for _, issue := range slot.Issues {
			if issue.Severity == core.SeverityInfo {
				continue
			}
			slotHasVisibleIssue = true
			total++
			if repairableCategories[issue.Category] {
				repairable++
			}
			switch issue.Severity {
			case core.SeverityCritical:
				critical++
			case core.SeverityWarning:
				warning++
			}
			if len(issues) < diagnosticPostLoadItemsMax {
				issues = append(issues, "slot="+strconv.Itoa(slot.SlotIndex)+
					",severity="+string(issue.Severity)+
					",category="+issue.Category+
					",issue="+diagnosticIssueCode(issue))
			}
		}
		if slotHasVisibleIssue {
			affectedSlots++
		}
	}

	return []diagnosticField{
		field("affected_slots", strconv.Itoa(affectedSlots)),
		field("issue_count", strconv.Itoa(total)),
		field("repairable_issue_count", strconv.Itoa(repairable)),
		field("critical_count", strconv.Itoa(critical)),
		field("warning_count", strconv.Itoa(warning)),
		field("issues", strings.Join(issues, "; ")),
		field("additional_issues", strconv.Itoa(total-len(issues))),
	}
}

// diagnosticIssueCode turns a renderer-facing diagnostic into a stable safe
// identifier. Descriptions can contain raw offsets, handles, indexes, or other
// save-derived values, so they are deliberately never copied into the journal.
func diagnosticIssueCode(issue core.DiagnosticIssue) string {
	description := strings.ToLower(issue.Description)
	switch issue.Category {
	case "data_size":
		return "unexpected_slot_size"
	case "offset_chain":
		return "invalid_offset_chain"
	case "gaitem":
		if strings.Contains(description, "unknown type prefix") {
			return "unknown_gaitem_handle_type"
		}
		return "gaitem_integrity"
	case "inventory":
		return "duplicate_inventory_index"
	case "stats":
		if strings.HasPrefix(description, "level") {
			return "level_out_of_range"
		}
		return "attribute_out_of_range"
	case "stats_formula":
		return "level_formula_mismatch"
	case "gaitemdata":
		return "gaitemdata_count_exceeds_limit"
	case "storage":
		return "storage_count_mismatch"
	case "dlc":
		return "dlc_section_observation"
	default:
		return "diagnostic_issue"
	}
}

func diagnosticPostLoadRepairFields(report RepairReport, processedSlots int, outcome, stage string) []diagnosticField {
	fixedActions, additionalFixed := diagnosticRepairActionList(report.Fixed, false)
	skippedReasons, additionalSkipped := diagnosticRepairActionList(report.Skipped, true)
	fields := []diagnosticField{
		field("outcome", outcome),
		field("processed_slots", strconv.Itoa(processedSlots)),
		field("fixed_count", strconv.Itoa(len(report.Fixed))),
		field("skipped_count", strconv.Itoa(len(report.Skipped))),
		field("fixed_actions", fixedActions),
		field("additional_fixed_actions", strconv.Itoa(additionalFixed)),
		field("skipped_reasons", skippedReasons),
		field("additional_skipped_reasons", strconv.Itoa(additionalSkipped)),
	}
	if stage != "" {
		fields = append(fields, field("stage", stage))
	}
	return fields
}

func diagnosticRepairActionList(results []string, skipped bool) (string, int) {
	actions := make([]string, 0, diagnosticPostLoadItemsMax)
	for _, result := range results {
		if len(actions) < diagnosticPostLoadItemsMax {
			actions = append(actions, diagnosticRepairActionCode(result, skipped))
		}
	}
	if len(actions) == 0 {
		return "none", 0
	}
	return strings.Join(actions, ","), len(results) - len(actions)
}

// diagnosticRepairActionCode preserves the useful repair identity while never
// copying core RepairSlot prose, which can include raw save-derived values in a
// skipped error message.
func diagnosticRepairActionCode(result string, skipped bool) string {
	lower := strings.ToLower(result)
	if skipped {
		switch {
		case strings.Contains(lower, "duplicate inventory indices"):
			return "duplicate_inventory_indices_unrepaired"
		case strings.Contains(lower, "wondrous physick"):
			return "duplicate_wondrous_physick_unrepaired"
		default:
			return "repair_unavailable"
		}
	}

	switch {
	case strings.HasPrefix(result, "Level "):
		return "clamp_level"
	case strings.Contains(result, " ") && strings.Contains(result, "→"):
		return "clamp_attribute"
	case strings.Contains(lower, "storage count header"):
		return "recalculate_storage_count_header"
	case strings.Contains(lower, "gaitemdata count"):
		return "cap_gaitemdata_count"
	case strings.Contains(lower, "duplicate inventory index"):
		return "repair_duplicate_inventory_indices"
	default:
		return "automated_repair"
	}
}
