package main

// GaItemMutationFailure describes a failed GaItem repair transaction. It is
// shared by the duplicate-repair analysis and execution results.
type GaItemMutationFailure struct {
	Stage   string `json:"stage"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// GaItemMutationRollback reports whether a discarded repair candidate left the
// active slot untouched.
type GaItemMutationRollback struct {
	Attempted bool                   `json:"attempted"`
	Complete  bool                   `json:"complete"`
	Mode      string                 `json:"mode"`
	Failure   *GaItemMutationFailure `json:"failure,omitempty"`
}

func gaItemMutationFailure(stage, code, message string) *GaItemMutationFailure {
	return &GaItemMutationFailure{Stage: stage, Code: code, Message: message}
}

func (a *App) hasActiveInventoryWorkspaceLocked(charIdx int) bool {
	a.editSessionsMu.Lock()
	_, active := a.editSessionByChar[charIdx]
	a.editSessionsMu.Unlock()
	return active
}
