package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ConversionInfo is returned by PrepareConversion.
type ConversionInfo struct {
	Path     string `json:"path"`
	Platform string `json:"platform"`
}

// PrepareConversion opens a file-picker dialog and returns the selected save's
// path and detected platform ("PC" or "PS4"). The file is not loaded into
// memory; platform detection reads only the first 4 magic bytes.
func (a *App) PrepareConversion() (*ConversionInfo, error) {
	a.journalLog(levelInfo, "save_conversion_source_requested", "save conversion source requested")
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Save File to Convert",
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat)", Pattern: "*.sl2;*.dat"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		a.journalLog(levelError, "save_conversion_source_failed", "save conversion source selection failed", field("stage", "dialog"))
		return nil, err
	}
	if path == "" {
		a.journalLog(levelInfo, "save_conversion_source_cancelled", "save conversion source selection cancelled")
		return nil, fmt.Errorf("no file selected")
	}

	platform, err := peekSavePlatform(path)
	if err != nil {
		a.journalLog(levelError, "save_conversion_source_failed", "save conversion source selection failed", field("stage", "detect_platform"))
		return nil, err
	}
	fields := []diagnosticField{field("platform", platform)}
	if name := safeSaveFileName(path); name != "" {
		fields = append(fields, field("save_file", name))
	}
	a.journalLog(levelInfo, "save_conversion_source_selected", "save conversion source selected", fields...)
	return &ConversionInfo{Path: path, Platform: platform}, nil
}

// ExecuteConversion loads sourcePath into a local variable (never touching
// a.save), converts it to targetPlatform, opens a save-file dialog and writes
// the result. steamIDStr is applied only when targetPlatform == "PC"; pass ""
// for PC→PS4 conversions.
func (a *App) ExecuteConversion(sourcePath string, targetPlatform string, steamIDStr string) (string, error) {
	fields := []diagnosticField{field("target_platform", targetPlatform)}
	if name := safeSaveFileName(sourcePath); name != "" {
		fields = append(fields, field("source_file", name))
	}
	a.journalLog(levelInfo, "save_conversion_requested", "save conversion requested", fields...)
	save, err := core.LoadSave(sourcePath)
	if err != nil {
		a.journalLog(levelError, "save_conversion_failed", "save conversion failed", field("stage", "parse"))
		return "", fmt.Errorf("failed to load source save: %w", err)
	}

	switch targetPlatform {
	case "PC":
		steamID, err := strconv.ParseUint(steamIDStr, 10, 64)
		if err != nil {
			a.journalLog(levelError, "save_conversion_failed", "save conversion failed", field("stage", "steam_id"))
			return "", fmt.Errorf("invalid Steam ID: %w", err)
		}
		iv := make([]byte, 16)
		if _, err := rand.Read(iv); err != nil {
			a.journalLog(levelError, "save_conversion_failed", "save conversion failed", field("stage", "iv"))
			return "", fmt.Errorf("failed to generate IV: %w", err)
		}
		save.Platform = core.PlatformPC
		save.Encrypted = true
		save.IV = iv
		save.SteamID = steamID // flushMetadata writes this to UserData10
	case "PS4":
		save.Platform = core.PlatformPS
		save.Encrypted = false
	default:
		a.journalLog(levelError, "save_conversion_failed", "save conversion failed", field("stage", "target_platform"))
		return "", fmt.Errorf("unknown target platform: %s", targetPlatform)
	}

	destPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "Save Converted File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat)", Pattern: "*.sl2;*.dat"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		a.journalLog(levelError, "save_conversion_failed", "save conversion failed", field("stage", "dialog"))
		return "", err
	}
	if destPath == "" {
		a.journalLog(levelInfo, "save_conversion_cancelled", "save conversion cancelled")
		return "", fmt.Errorf("no destination selected")
	}

	if err := save.SaveFile(destPath); err != nil {
		a.journalLog(levelError, "save_conversion_failed", "save conversion failed", field("stage", "write"))
		return "", fmt.Errorf("failed to write converted save: %w", err)
	}
	completed := []diagnosticField{field("target_platform", targetPlatform)}
	if name := safeSaveFileName(destPath); name != "" {
		completed = append(completed, field("save_file", name))
	}
	a.journalLog(levelInfo, "save_conversion_finished", "save conversion finished", completed...)
	return destPath, nil
}

// peekSavePlatform reads the first 4 bytes of path to detect the save format.
// "BND4" magic → PC; anything else → PS4.
func peekSavePlatform(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return "", fmt.Errorf("cannot read magic bytes: %w", err)
	}
	if string(magic) == "BND4" {
		return "PC", nil
	}
	return "PS4", nil
}
