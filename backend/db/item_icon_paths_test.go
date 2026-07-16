package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// numberedBellBearing matches non-canonical Bell Bearing icon paths that still
// carry a trailing _N variant suffix. All numbered variants must resolve to the
// canonical family icon (…_bell_bearing.png).
var numberedBellBearing = regexp.MustCompile(`_bell_bearing_\d+\.png$`)

// TestItemIconPaths guards the icon-routing invariants for every ItemData record
// in the database. It reads only committed assets under frontend/public and must
// never depend on tmp/ scratch directories.
func TestItemIconPaths(t *testing.T) {
	const publicRoot = "../../frontend/public"

	for id, item := range globalItemIndex {
		if item.IconPath == "" {
			continue
		}

		wantPrefix := "items/" + item.Category + "/"
		if !strings.HasPrefix(item.IconPath, wantPrefix) {
			t.Errorf("item %#x (%q): IconPath %q must be routed under %q (Category=%q)",
				id, item.Name, item.IconPath, wantPrefix, item.Category)
		}

		if numberedBellBearing.MatchString(item.IconPath) {
			t.Errorf("item %#x (%q): numbered Bell Bearing IconPath %q must use the canonical path without _N",
				id, item.Name, item.IconPath)
		}

		if _, err := os.Stat(filepath.Join(publicRoot, filepath.FromSlash(item.IconPath))); err != nil {
			t.Errorf("item %#x (%q): icon file missing under %s: %s", id, item.Name, publicRoot, item.IconPath)
		}
	}
}
