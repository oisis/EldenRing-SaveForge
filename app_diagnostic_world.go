package main

import "strconv"

// RecordDiagnosticWorldAction records one user-visible World-tab operation.
// The renderer can choose only a closed action/outcome vocabulary and bounded
// counts; it cannot inject text, identifiers, event names, or log levels.
func (a *App) RecordDiagnosticWorldAction(action, outcome string, characterIndex, requestedCount, completedCount int, failureReason string) {
	if !validDiagnosticWorldAction(action) || characterIndex < 0 || characterIndex >= 10 || requestedCount < 0 || requestedCount > 10000 {
		return
	}

	fields := []diagnosticField{
		field("action", action),
		field("character_index", strconv.Itoa(characterIndex)),
		field("requested_count", strconv.Itoa(requestedCount)),
	}
	switch outcome {
	case "requested":
		a.journalLogFrom(levelInfo, "frontend", "world_action_requested", "world action requested", fields...)
	case "succeeded":
		if completedCount < 0 || completedCount > requestedCount {
			return
		}
		fields = append(fields, field("completed_count", strconv.Itoa(completedCount)))
		a.journalLogFrom(levelInfo, "frontend", "world_action_finished", "world action finished", fields...)
	case "error":
		if !validDiagnosticWorldFailureReason(failureReason) {
			return
		}
		// Concurrent Promise-based bulk calls can fail after an unknown subset
		// has been applied. Never claim a count that the renderer cannot prove.
		fields = append(fields,
			field("completed_count", "unknown"),
			field("reason", failureReason))
		a.journalLogFrom(levelError, "frontend", "world_action_finished", "world action failed", fields...)
	}
}

func validDiagnosticWorldFailureReason(reason string) bool {
	switch reason {
	case "no_active_save", "invalid_character", "event_flags_unavailable", "world_operation_failed":
		return true
	default:
		return false
	}
}

func validDiagnosticWorldAction(action string) bool {
	switch action {
	case "grace_set", "graces_unlock_region", "graces_unlock_all", "graces_lock_all",
		"boss_set_defeated", "bosses_kill_region", "bosses_respawn_region", "bosses_kill_filtered", "bosses_respawn_filtered",
		"summoning_pool_set", "summoning_pools_activate_region", "summoning_pools_activate_all", "summoning_pools_deactivate_all",
		"colosseum_set", "colosseums_unlock_all", "colosseums_lock_all",
		"gesture_set", "gestures_unlock_all", "gestures_lock_all",
		"cookbook_set", "cookbooks_unlock_all", "cookbooks_lock_all",
		"bell_bearing_set", "bell_bearings_unlock_all", "bell_bearings_lock_all",
		"whetblade_set", "whetblades_unlock_all", "whetblades_lock_all",
		"quest_set_step", "quest_unset_step", "quest_toggle_flag",
		"map_region_set", "map_system_flag_set", "map_system_flags_normalized", "map_reveal_all", "map_reset",
		"invasion_region_set", "invasion_regions_unlock_area", "invasion_regions_lock_area", "invasion_regions_unlock_all", "invasion_regions_lock_all":
		return true
	default:
		return false
	}
}
