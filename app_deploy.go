package main

import (
	"fmt"
	"os"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/deploy"
)

// SlotCapacity reports used vs max counts for GaItems, Inventory, and Storage.
type SlotCapacity struct {
	GaItemsUsed   int `json:"gaItemsUsed"`
	GaItemsMax    int `json:"gaItemsMax"`
	InventoryUsed int `json:"inventoryUsed"`
	InventoryMax  int `json:"inventoryMax"`
	StorageUsed   int `json:"storageUsed"`
	StorageMax    int `json:"storageMax"`
}

// GetSlotCapacity returns capacity usage for a character slot.
func (a *App) GetSlotCapacity(charIdx int) (*SlotCapacity, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()

	usage := core.CountSlotUsage(&a.save.Slots[charIdx])
	return &SlotCapacity{
		GaItemsUsed:   usage.GaItemsUsed,
		GaItemsMax:    usage.GaItemsMax,
		InventoryUsed: usage.InventoryUsed,
		InventoryMax:  usage.InventoryMax,
		StorageUsed:   usage.StorageUsed,
		StorageMax:    usage.StorageMax,
	}, nil
}

// GetDeployTargets returns all configured deploy targets.
func (a *App) GetDeployTargets() []deploy.Target {
	if a.deployStore == nil {
		return nil
	}
	return a.deployStore.List()
}

// SaveDeployTarget adds or updates a deploy target.
func (a *App) SaveDeployTarget(t deploy.Target) error {
	a.journalToolsOperationRequested(actionToolsSaveDeployTarget)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsSaveDeployTarget, characterChangeError, toolsStageConfiguration)
		return fmt.Errorf("deploy not initialized")
	}
	if err := a.deployStore.Save(t); err != nil {
		a.journalToolsOperationFinished(actionToolsSaveDeployTarget, characterChangeError, toolsStageSaveDeployTarget)
		return err
	}
	a.journalToolsOperationFinished(actionToolsSaveDeployTarget, characterChangeSuccess, toolsStageCompleted)
	return nil
}

// DeleteDeployTarget removes a deploy target by name.
func (a *App) DeleteDeployTarget(name string) error {
	a.journalToolsOperationRequested(actionToolsDeleteDeployTarget)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsDeleteDeployTarget, characterChangeError, toolsStageConfiguration)
		return fmt.Errorf("deploy not initialized")
	}
	if err := a.deployStore.Delete(name); err != nil {
		a.journalToolsOperationFinished(actionToolsDeleteDeployTarget, characterChangeError, toolsStageDeleteDeployTarget)
		return err
	}
	a.journalToolsOperationFinished(actionToolsDeleteDeployTarget, characterChangeSuccess, toolsStageCompleted)
	return nil
}

// isLocalTarget returns true if the named target is configured as local.
func (a *App) isLocalTarget(name string) bool {
	if a.deployStore == nil {
		return false
	}
	t, ok := a.deployStore.Get(name)
	return ok && t.IsLocal()
}

// TestSSHConnection tests connectivity to a target (SSH or local path).
func (a *App) TestSSHConnection(targetName string) (string, error) {
	a.journalToolsOperationRequested(actionToolsTestDeployConnection)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsTestDeployConnection, characterChangeError, toolsStageConfiguration)
		return "", fmt.Errorf("deploy not initialized")
	}
	var message string
	var err error
	if a.isLocalTarget(targetName) {
		message, err = a.deployLocal.TestConnection(targetName)
	} else {
		message, err = a.deploySSH.TestConnection(targetName)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsTestDeployConnection, characterChangeError, toolsStageTestConnection)
		return "", err
	}
	a.journalToolsOperationFinished(actionToolsTestDeployConnection, characterChangeSuccess, toolsStageCompleted)
	return message, nil
}

// DeploySave writes the current in-memory save to a temp file and uploads/copies it to a target.
// Returns a human-readable success message with file size.
func (a *App) DeploySave(targetName string) (string, error) {
	a.journalToolsOperationRequested(actionToolsDeploySave)
	a.journalLog(levelInfo, "deploy_save_requested", "save deploy requested")
	if a.deployStore == nil {
		a.journalLog(levelError, "deploy_save_failed", "save deploy failed", field("stage", "configuration"))
		a.journalToolsOperationFinished(actionToolsDeploySave, characterChangeError, toolsStageConfiguration)
		return "", fmt.Errorf("deploy not initialized")
	}
	// Brief saveMu.RLock just for the user-facing nil-check; writeTempSave
	// takes its own locks (saveMu.RLock + all slotMu) for the serialisation
	// pass so we do not nest RLock on a single goroutine.
	a.saveMu.RLock()
	noSave := a.save == nil
	a.saveMu.RUnlock()
	if noSave {
		a.journalLog(levelError, "deploy_save_failed", "save deploy failed", field("stage", "no_active_save"))
		a.journalToolsOperationFinished(actionToolsDeploySave, characterChangeError, toolsStageNoActiveSave)
		return "", fmt.Errorf("no save loaded")
	}

	// Write current working state to a temp file for upload
	tmpPath, err := a.writeTempSave()
	if err != nil {
		a.journalLog(levelError, "deploy_save_failed", "save deploy failed", field("stage", "serialize"))
		a.journalToolsOperationFinished(actionToolsDeploySave, characterChangeError, toolsStageSerialize)
		return "", err
	}
	defer os.Remove(tmpPath)

	info, _ := os.Stat(tmpPath)
	sizeMB := float64(info.Size()) / (1024 * 1024)

	transport := "remote"
	if a.isLocalTarget(targetName) {
		transport = "local"
		if err := a.deployLocal.UploadSave(targetName, tmpPath); err != nil {
			a.journalLog(levelError, "deploy_save_failed", "save deploy failed", field("stage", "upload"), field("transport", transport))
			a.journalToolsOperationFinished(actionToolsDeploySave, characterChangeError, toolsStageUpload)
			return "", err
		}
	} else {
		if err := a.deploySSH.UploadSave(targetName, tmpPath); err != nil {
			a.journalLog(levelError, "deploy_save_failed", "save deploy failed", field("stage", "upload"), field("transport", transport))
			a.journalToolsOperationFinished(actionToolsDeploySave, characterChangeError, toolsStageUpload)
			return "", err
		}
	}

	t, _ := a.deployStore.Get(targetName)
	a.journalLog(levelInfo, "deploy_save_completed", "save deploy completed", field("transport", transport))
	a.journalToolsOperationFinished(actionToolsDeploySave, characterChangeSuccess, toolsStageCompleted)
	return fmt.Sprintf("Uploaded %.1f MB to %s", sizeMB, t.Name), nil
}

// DownloadRemoteSave downloads/copies a save file from a target and loads it.
// The temp file is removed after loading into memory.
func (a *App) DownloadRemoteSave(targetName string) (string, error) {
	a.journalToolsOperationRequested(actionToolsDownloadRemoteSave)
	platform, stage, err := a.downloadRemoteSave(targetName)
	if err != nil {
		a.journalToolsOperationFinished(actionToolsDownloadRemoteSave, characterChangeError, stage)
		return "", err
	}
	a.journalToolsOperationFinished(actionToolsDownloadRemoteSave, characterChangeSuccess, toolsStageCompleted)
	return platform, nil
}

// downloadRemoteSave performs the existing remote-load operation without a
// Tools operation wrapper. CloseAndDownload reuses it so a combined action emits
// one requested/finished pair rather than a misleading nested download pair.
func (a *App) downloadRemoteSave(targetName string) (string, string, error) {
	a.journalLog(levelInfo, "save_load_requested", "save load requested", field("origin", string(loadOriginRemoteDownload)))
	if a.deployStore == nil {
		a.journalLog(levelError, "save_load_failed", "save load failed", field("origin", string(loadOriginRemoteDownload)), field("stage", "configuration"))
		return "", toolsStageConfiguration, fmt.Errorf("deploy not initialized")
	}

	tmpDir, err := os.MkdirTemp("", "er-save-download-")
	if err != nil {
		a.journalLog(levelError, "save_load_failed", "save load failed", field("origin", string(loadOriginRemoteDownload)), field("stage", "temp_dir"))
		return "", toolsStageTempDir, fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	localPath := tmpDir + "/ER0000.sl2"

	if a.isLocalTarget(targetName) {
		if err := a.deployLocal.DownloadSave(targetName, localPath); err != nil {
			a.journalLog(levelError, "save_load_failed", "save load failed", field("origin", string(loadOriginRemoteDownload)), field("stage", "download"), field("transport", "local"))
			return "", toolsStageDownload, err
		}
	} else {
		if err := a.deploySSH.DownloadSave(targetName, localPath); err != nil {
			a.journalLog(levelError, "save_load_failed", "save load failed", field("origin", string(loadOriginRemoteDownload)), field("stage", "download"), field("transport", "remote"))
			return "", toolsStageDownload, err
		}
	}

	save, err := core.LoadSave(localPath)
	if err != nil {
		a.journalLog(levelError, "save_load_failed", "save load failed", field("origin", string(loadOriginRemoteDownload)), field("stage", "parse"))
		return "", toolsStageParse, fmt.Errorf("downloaded file is not a valid save: %w", err)
	}
	a.journalLog(levelInfo, "save_load_parsed", "save load parsed", field("origin", string(loadOriginRemoteDownload)), field("platform", string(save.Platform)))
	// Commit phase under exclusive saveMu — see SelectAndOpenSave.
	a.commitLoadedSave(save, "", loadOriginRemoteDownload)
	return string(save.Platform), toolsStageCompleted, nil
}

// LaunchRemoteGame starts the game on a target (SSH or local).
func (a *App) LaunchRemoteGame(targetName string) (string, error) {
	a.journalToolsOperationRequested(actionToolsLaunchRemoteGame)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsLaunchRemoteGame, characterChangeError, toolsStageConfiguration)
		return "", fmt.Errorf("deploy not initialized")
	}
	var message string
	var err error
	if a.isLocalTarget(targetName) {
		message, err = a.deployLocal.LaunchGame(targetName)
	} else {
		message, err = a.deploySSH.LaunchGame(targetName)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsLaunchRemoteGame, characterChangeError, toolsStageLaunch)
		return "", err
	}
	a.journalToolsOperationFinished(actionToolsLaunchRemoteGame, characterChangeSuccess, toolsStageCompleted)
	return message, nil
}

// CloseRemoteGame stops the game on a target (SSH or local).
func (a *App) CloseRemoteGame(targetName string) (string, error) {
	a.journalToolsOperationRequested(actionToolsCloseRemoteGame)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsCloseRemoteGame, characterChangeError, toolsStageConfiguration)
		return "", fmt.Errorf("deploy not initialized")
	}
	var message string
	var err error
	if a.isLocalTarget(targetName) {
		message, err = a.deployLocal.CloseGame(targetName)
	} else {
		message, err = a.deploySSH.CloseGame(targetName)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsCloseRemoteGame, characterChangeError, toolsStageClose)
		return "", err
	}
	a.journalToolsOperationFinished(actionToolsCloseRemoteGame, characterChangeSuccess, toolsStageCompleted)
	return message, nil
}

// DeployAndLaunch performs: write temp → upload → launch (no close).
func (a *App) DeployAndLaunch(targetName string) error {
	a.journalToolsOperationRequested(actionToolsDeployAndLaunch)
	a.journalLog(levelInfo, "deploy_and_launch_requested", "save deploy and launch requested")
	if a.deployStore == nil {
		a.journalLog(levelError, "deploy_and_launch_failed", "save deploy and launch failed", field("stage", "configuration"))
		a.journalToolsOperationFinished(actionToolsDeployAndLaunch, characterChangeError, toolsStageConfiguration)
		return fmt.Errorf("deploy not initialized")
	}
	// Brief saveMu.RLock for the nil-check; writeTempSave takes its own
	// locks for the serialisation pass.
	a.saveMu.RLock()
	noSave := a.save == nil
	a.saveMu.RUnlock()
	if noSave {
		a.journalLog(levelError, "deploy_and_launch_failed", "save deploy and launch failed", field("stage", "no_active_save"))
		a.journalToolsOperationFinished(actionToolsDeployAndLaunch, characterChangeError, toolsStageNoActiveSave)
		return fmt.Errorf("no save loaded")
	}

	tmpPath, err := a.writeTempSave()
	if err != nil {
		a.journalLog(levelError, "deploy_and_launch_failed", "save deploy and launch failed", field("stage", "serialize"))
		a.journalToolsOperationFinished(actionToolsDeployAndLaunch, characterChangeError, toolsStageSerialize)
		return err
	}
	defer os.Remove(tmpPath)

	// Upload save
	transport := "remote"
	if a.isLocalTarget(targetName) {
		transport = "local"
		if err := a.deployLocal.UploadSave(targetName, tmpPath); err != nil {
			a.journalLog(levelError, "deploy_and_launch_failed", "save deploy and launch failed", field("stage", "upload"), field("transport", transport))
			a.journalToolsOperationFinished(actionToolsDeployAndLaunch, characterChangeError, toolsStageUpload)
			return fmt.Errorf("upload failed: %w", err)
		}
	} else {
		if err := a.deploySSH.UploadSave(targetName, tmpPath); err != nil {
			a.journalLog(levelError, "deploy_and_launch_failed", "save deploy and launch failed", field("stage", "upload"), field("transport", transport))
			a.journalToolsOperationFinished(actionToolsDeployAndLaunch, characterChangeError, toolsStageUpload)
			return fmt.Errorf("upload failed: %w", err)
		}
	}

	// Launch game
	if a.isLocalTarget(targetName) {
		if _, err := a.deployLocal.LaunchGame(targetName); err != nil {
			a.journalLog(levelError, "deploy_and_launch_failed", "save deploy and launch failed", field("stage", "launch"), field("transport", transport))
			a.journalToolsOperationFinished(actionToolsDeployAndLaunch, characterChangeError, toolsStageLaunch)
			return fmt.Errorf("launch failed: %w", err)
		}
	} else {
		if _, err := a.deploySSH.LaunchGame(targetName); err != nil {
			a.journalLog(levelError, "deploy_and_launch_failed", "save deploy and launch failed", field("stage", "launch"), field("transport", transport))
			a.journalToolsOperationFinished(actionToolsDeployAndLaunch, characterChangeError, toolsStageLaunch)
			return fmt.Errorf("launch failed: %w", err)
		}
	}

	a.journalLog(levelInfo, "deploy_and_launch_completed", "save deploy and launch completed", field("transport", transport))
	a.journalToolsOperationFinished(actionToolsDeployAndLaunch, characterChangeSuccess, toolsStageCompleted)
	return nil
}

// CloseAndDownload performs: close game → wait for save flush → download → load.
// The temp file is removed after loading into memory.
func (a *App) CloseAndDownload(targetName string) (string, error) {
	a.journalToolsOperationRequested(actionToolsCloseAndDownload)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsCloseAndDownload, characterChangeError, toolsStageConfiguration)
		return "", fmt.Errorf("deploy not initialized")
	}

	// Close the game (ignore errors — game might not be running)
	if a.isLocalTarget(targetName) {
		a.deployLocal.CloseGame(targetName)
	} else {
		a.deploySSH.CloseGame(targetName)
	}

	// Wait for graceful shutdown and save file flush
	time.Sleep(5 * time.Second)

	// Download save. Use the internal driver so this combined operation owns the
	// single Tools lifecycle pair; the existing save_load_* diagnostics remain
	// emitted exactly once.
	platform, stage, err := a.downloadRemoteSave(targetName)
	if err != nil {
		a.journalToolsOperationFinished(actionToolsCloseAndDownload, characterChangeError, stage)
		return "", err
	}
	a.journalToolsOperationFinished(actionToolsCloseAndDownload, characterChangeSuccess, toolsStageCompleted)
	return platform, nil
}

// writeTempSave serializes the current in-memory save to a temp file, preserving target platform.
//
// Takes its OWN saveMu.RLock, favMu.RLock and all slotMu[0..9] (rosnąco)
// for the duration of a.save.SaveFile, so the resulting bytes correspond
// to a consistent snapshot — no concurrent slot writer can torn-write the
// per-slot byte buffers mid-serialisation, and no concurrent favorites
// writer (RemoveFavoritePreset / WriteSelectedToFavorites — both run
// under saveMu.RLock + favMu.Lock, neither takes slotMu) can mutate the
// preset region of UserData10.Data while SaveFile reads the full 0x60000-
// byte blob for MD5 + WriteBytes. flushMetadata writes only into
// SteamID [0x00..0x08) and ActiveSlots / ProfileSummaries
// [0x1954..0x3CDE), which is disjoint from the favorites region
// [0x154..0x1324), so favMu.RLock (shared, not exclusive) is the
// minimal lock needed: writeTempSave is a reader of the favorites bytes
// and a writer of metadata bytes whose ranges no favorites endpoint
// touches. Lock order saveMu → favMu → slotMu matches the App-level
// hierarchy documented in app.go. The locks are released BEFORE the
// temp path is returned, so the caller's upload/network/launch phase
// runs entirely lock-free. Public callers (DeploySave, DeployAndLaunch)
// must therefore NOT hold saveMu themselves when calling this helper.
func (a *App) writeTempSave() (string, error) {
	tmpFile, err := os.CreateTemp("", "er-deploy-*.sl2")
	if err != nil {
		return "", fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	a.saveMu.RLock()
	a.favMu.RLock()
	a.lockAllSlots()
	if a.save == nil {
		a.unlockAllSlots()
		a.favMu.RUnlock()
		a.saveMu.RUnlock()
		os.Remove(tmpPath)
		return "", fmt.Errorf("no save loaded")
	}
	snapshot := diagnosticSaveSnapshot(a.save, a.saveGeneration)
	serializeErr := a.save.SaveFile(tmpPath)
	a.unlockAllSlots()
	a.favMu.RUnlock()
	a.saveMu.RUnlock()
	if len(snapshot) > 0 {
		a.journalDebug(eventSaveStateBeforeWrite, "privacy-safe save state captured before deploy serialization",
			diagnosticSnapshotForSerialization(snapshot, "deploy_temp")...)
	}

	if serializeErr != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp save: %w", serializeErr)
	}
	return tmpPath, nil
}
