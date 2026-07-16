package main

import (
	"strconv"

	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SetDiagnosticDebugMode switches diagnostic verbosity for the running
// session. It is driven by the frontend Debug Mode toggle (invoked on mount
// and on every change) and, in order:
//
//  1. sets the DiagnosticJournal's verbosity policy — the single source of
//     truth — so debug records start (enabled) or stop (disabled) being
//     persisted; trace stays dropped either way. This works headless, where
//     a.ctx is nil (tests, pre-startup).
//  2. when a Wails context exists, retargets the runtime logger: DEBUG when
//     enabled, INFO when disabled, so Wails-sourced debug lines reach the sink.
//  3. records a sanitized, info-level change event. Info always passes the
//     verbosity filter, so the event is durable even when disabling the mode.
func (a *App) SetDiagnosticDebugMode(enabled bool) {
	a.journal.SetDebugEnabled(enabled)

	if a.ctx != nil {
		level := logger.INFO
		if enabled {
			level = logger.DEBUG
		}
		runtime.LogSetLogLevel(a.ctx, level)
	}

	a.journalLog(levelInfo, "diagnostic_debug_mode_changed",
		"diagnostic debug mode changed", field("enabled", strconv.FormatBool(enabled)))
}
