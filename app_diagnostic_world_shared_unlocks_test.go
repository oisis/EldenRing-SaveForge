package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// WorldTab invokes these shared App endpoints directly. They deliberately keep
// their existing Game Items lifecycle because that contract already includes
// both World flags and the physical inventory effects; a second World lifecycle
// would duplicate the same mutation in the durable journal.
func TestWorldSharedUnlockEndpointsUseOneDetailedLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		action string
		setup  func(*App)
		apply  func(*App) error
	}{
		{
			name:   "cookbook",
			action: actionGameItemsUnlockCookbook,
			setup:  func(app *App) { withUnlockEventFlags(app, itemEventFlagsRegionSize) },
			apply:  func(app *App) error { return app.SetCookbookUnlocked(0, 67000, true) },
		},
		{
			name:   "bell_bearing",
			action: actionGameItemsUnlockBellBearing,
			setup:  func(app *App) { withUnlockEventFlags(app, 0x160000) },
			apply:  func(app *App) error { return app.SetBellBearingUnlocked(0, 11109710, true) },
		},
		{
			name:   "whetblade",
			action: actionGameItemsUnlockWhetblade,
			setup:  func(app *App) { withUnlockEventFlags(app, whetbladeEventFlagsRegionSize) },
			apply: func(app *App) error {
				_, err := app.SetWhetbladeUnlocked(0, data.WhetstoneKnifeFlag, true)
				return err
			},
		},
		{
			name:   "map_fragment",
			action: actionGameItemsUnlockMapFragment,
			setup:  func(app *App) { withUnlockEventFlags(app, itemEventFlagsRegionSize) },
			apply:  func(app *App) error { return app.SetMapRegionFlags(0, 62010, true) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gameItemAddApp(false)
			tt.setup(app)
			journal := freshRemoveJournal(app, true)
			if err := tt.apply(app); err != nil {
				t.Fatalf("shared World endpoint: %v", err)
			}

			lc := collectUnlockLifecycle(t, journal.Tail(), tt.action, "0")
			assertUnlockSuccess(t, lc)
			for _, rec := range journal.Tail() {
				switch rec.Event {
				case eventWorldChangeBefore, eventWorldChangePlanned, eventWorldChangeFinished:
					t.Fatalf("shared endpoint emitted duplicate World event %q", rec.Event)
				}
			}
		})
	}
}
