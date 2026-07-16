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

// RecordDiagnosticClientAssetLoadFailure records that a specific item icon
// actually failed to load in the renderer (a real <img> error, not asset-server
// fallback chatter). The renderer supplies only the public, relative icon path
// items/<category>/<file>.png and cannot choose the event, message, level, or
// any other field. An asset failing strict validation is dropped without a
// record; a validated asset is the single deliberate exception to the journal
// path sanitizer (see sanitizeFields / isValidIconAsset).
func (a *App) RecordDiagnosticClientAssetLoadFailure(asset string) {
	if !isValidIconAsset(asset) {
		return
	}
	a.journalLogFrom(levelWarn, "frontend", "asset_load_failed", "item icon failed to load",
		field("asset", asset))
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
