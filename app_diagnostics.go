package main

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DiagnosticsReport is returned by RunDiagnostics* endpoints.
type DiagnosticsReport struct {
	Source    string           `json:"source"`    // "loaded" or absolute file path
	Slots     []SlotDiagResult `json:"slots"`
	CanRepair bool             `json:"canRepair"` // true if any repairable issue found
}

// SlotDiagResult wraps core.SlotDiagnostics with character name.
type SlotDiagResult struct {
	SlotIndex int                    `json:"slotIndex"`
	CharName  string                 `json:"charName"`
	Issues    []core.DiagnosticIssue `json:"issues"`
}

// RepairReport is returned by Repair* endpoints.
type RepairReport struct {
	Fixed   []string `json:"fixed"`
	Skipped []string `json:"skipped"`
}

// diagState holds the parsed external save between RunDiagnosticsExternal and
// RepairExternal / SaveRepairedExternal calls. Protected by diagMu.
var diagState struct {
	mu   sync.Mutex
	save *core.SaveFile
}

var repairableCategories = map[string]bool{
	"inventory":     true,
	"stats":         true,
	"dlc":           true,
	"gaitemdata":    true,
	"storage_count": true,
}

func canRepairIssues(issues []core.DiagnosticIssue) bool {
	for _, iss := range issues {
		if repairableCategories[iss.Category] {
			return true
		}
	}
	return false
}

// PickDiagnosticsFile opens the native file dialog and returns the selected path.
// Empty string means the dialog was cancelled.
func (a *App) PickDiagnosticsFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Save File to Diagnose",
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat;*.txt)", Pattern: "*.sl2;*.dat;*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
}

// RunDiagnosticsLoaded runs a corruption scan on the currently loaded save at charIndex.
func (a *App) RunDiagnosticsLoaded(charIndex int) (DiagnosticsReport, error) {
	var empty DiagnosticsReport
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= maxCharacters {
		return empty, fmt.Errorf("invalid character index")
	}
	a.slotMu[charIndex].Lock()
	defer a.slotMu[charIndex].Unlock()

	slot := &a.save.Slots[charIndex]
	if slot.Version == 0 {
		return empty, fmt.Errorf("slot %d is empty", charIndex)
	}
	diag := core.DiagnoseSaveCorruption(slot, charIndex)
	charName := core.UTF16ToString(slot.Player.CharacterName[:])

	return DiagnosticsReport{
		Source: "loaded",
		Slots: []SlotDiagResult{{
			SlotIndex: charIndex,
			CharName:  charName,
			Issues:    diag.Issues,
		}},
		CanRepair: canRepairIssues(diag.Issues),
	}, nil
}

// RunDiagnosticsExternal loads an external save file and runs diagnostics on all slots.
// The parsed file is cached internally for a subsequent RepairExternal / SaveRepairedExternal call.
func (a *App) RunDiagnosticsExternal(filePath string) (DiagnosticsReport, error) {
	var empty DiagnosticsReport
	save, err := core.LoadSave(filePath)
	if err != nil {
		return empty, err
	}

	diagState.mu.Lock()
	diagState.save = save
	diagState.mu.Unlock()

	var slots []SlotDiagResult
	var allIssues []core.DiagnosticIssue
	for i := range save.Slots {
		slot := &save.Slots[i]
		if slot.Version == 0 {
			continue
		}
		diag := core.DiagnoseSaveCorruption(slot, i)
		slots = append(slots, SlotDiagResult{
			SlotIndex: i,
			CharName:  core.UTF16ToString(slot.Player.CharacterName[:]),
			Issues:    diag.Issues,
		})
		allIssues = append(allIssues, diag.Issues...)
	}

	return DiagnosticsReport{
		Source:    filePath,
		Slots:     slots,
		CanRepair: canRepairIssues(allIssues),
	}, nil
}

// RepairLoadedSave applies all automated repairs to the given character slot.
// Pushes an undo snapshot before mutating.
func (a *App) RepairLoadedSave(charIndex int) (RepairReport, error) {
	var empty RepairReport
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= maxCharacters {
		return empty, fmt.Errorf("invalid character index")
	}
	a.slotMu[charIndex].Lock()
	defer a.slotMu[charIndex].Unlock()

	slot := &a.save.Slots[charIndex]
	if slot.Version == 0 {
		return empty, fmt.Errorf("slot %d is empty", charIndex)
	}

	a.pushUndoLocked(charIndex)
	fixed, skipped := core.RepairSlot(slot)
	return RepairReport{Fixed: fixed, Skipped: skipped}, nil
}

// RepairExternal applies all automated repairs to the cached external save file.
func (a *App) RepairExternal() (RepairReport, error) {
	var empty RepairReport
	diagState.mu.Lock()
	defer diagState.mu.Unlock()

	if diagState.save == nil {
		return empty, fmt.Errorf("no external file loaded; run RunDiagnosticsExternal first")
	}

	allFixed := []string{}
	allSkipped := []string{}
	for i := range diagState.save.Slots {
		slot := &diagState.save.Slots[i]
		if slot.Version == 0 {
			continue
		}
		fixed, skipped := core.RepairSlot(slot)
		allFixed = append(allFixed, fixed...)
		allSkipped = append(allSkipped, skipped...)
	}
	return RepairReport{Fixed: allFixed, Skipped: allSkipped}, nil
}

// SaveRepairedExternal opens a native save dialog and writes the repaired external file.
// Returns the chosen output path, or empty string if cancelled.
func (a *App) SaveRepairedExternal(sourceFilePath string) (string, error) {
	diagState.mu.Lock()
	save := diagState.save
	diagState.mu.Unlock()

	if save == nil {
		return "", fmt.Errorf("no repaired file in memory")
	}

	defaultName := "repaired_" + filepath.Base(sourceFilePath)
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Repaired File",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat;*.txt)", Pattern: "*.sl2;*.dat;*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // cancelled
	}

	if err := save.SaveFile(path); err != nil {
		return "", err
	}

	diagState.mu.Lock()
	diagState.save = nil
	diagState.mu.Unlock()

	return path, nil
}
