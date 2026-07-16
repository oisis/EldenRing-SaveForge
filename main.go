package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Open the durable per-session diagnostic journal before wails.Run so
	// startup and Wails logging are captured. If it cannot be opened the
	// app still runs: we fall back to Wails' default stdout logger and
	// leave app.journal nil (journalLog then no-ops).
	appLogger := logger.NewDefaultLogger()
	if journal, err := NewSessionDiagnosticJournal(); err != nil {
		fmt.Fprintf(os.Stderr, "diagnostic journal unavailable: %v\n", err)
	} else {
		app.journal = journal
		appLogger = newWailsJournalLogger(journal)
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:         "Elden Ring SaveForge by OiSiS",
		Width:         1280,
		Height:        800,
		MinWidth:      1024,
		MinHeight:     768,
		DisableResize: false,
		LogLevel:      logger.INFO,
		Logger:        appLogger,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       false,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Elden Ring SaveForge by OiSiS",
				Message: "© 2026 OiSiS",
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
