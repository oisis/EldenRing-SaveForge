package main

const maxClientDiagnosticMessageRunes = 512

// RecordDiagnosticClientError accepts the deliberately small client-error
// contract used by the React error boundary and global browser handlers. It
// never accepts a stack trace, source URL, filename, request payload, or an
// arbitrary event name. The journal sanitizer remains the final privacy
// boundary for the bounded error type and message.
func (a *App) RecordDiagnosticClientError(kind, errorType, message string) {
	event := "frontend_unknown_error"
	switch kind {
	case "render":
		event = "frontend_render_error"
	case "unhandled_error":
		event = "frontend_unhandled_error"
	case "unhandled_rejection":
		event = "frontend_unhandled_rejection"
	}

	a.journalLogFrom(levelError, "frontend", event, "frontend error captured",
		field("error_type", truncateDiagnosticRunes(errorType, maxClientDiagnosticMessageRunes)),
		field("message", truncateDiagnosticRunes(message, maxClientDiagnosticMessageRunes)))
}

func truncateDiagnosticRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
