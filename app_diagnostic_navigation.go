package main

// RecordDiagnosticClientNavigation records a whitelisted frontend navigation
// transition. The narrow contract prevents a renderer client from writing an
// arbitrary event or free-form data into the durable journal.
func (a *App) RecordDiagnosticClientNavigation(scope, from, to string) {
	if from == to || !validDiagnosticNavigation(scope, from) || !validDiagnosticNavigation(scope, to) {
		return
	}
	a.journalLogFrom(levelDebug, "frontend", "navigation_changed", "frontend navigation changed",
		field("scope", scope),
		field("from", from),
		field("to", to))
}

func validDiagnosticNavigation(scope, value string) bool {
	switch scope {
	case "main_tab":
		switch value {
		case "character", "inventory", "world", "advanced", "tools":
			return true
		}
	case "inventory_view":
		switch value {
		case "database", "inventory", "sort_order":
			return true
		}
	}
	return false
}
