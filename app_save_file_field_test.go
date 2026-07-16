package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func TestSafeSaveFileName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"full path keeps basename only", "/Users/alice/private/ER0000-kro55.sl2", "ER0000-kro55.sl2"},
		{"windows path keeps basename only", `C:\Users\alice\ER0000.dat`, "ER0000.dat"},
		{"txt suffix allowed", "/tmp/notes.txt", "notes.txt"},
		{"empty path", "", ""},
		{"traversal rejected", "../ER0000.sl2", ""},
		{"embedded traversal rejected", "ER0000..sl2", ""},
		{"directory only rejected", "/Users/alice/", ""},
		{"wrong suffix rejected", "/tmp/ER0000.png", ""},
		{"no suffix rejected", "/tmp/ER0000", ""},
		{"over-long rejected", "/tmp/" + strings.Repeat("a", 130) + ".sl2", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeSaveFileName(tc.in); got != tc.want {
				t.Errorf("safeSaveFileName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCommitLoadedSaveRecordsBasenameOnly(t *testing.T) {
	app := newDebugOperationApp(t)
	app.commitLoadedSave(&core.SaveFile{Platform: "PC"}, "/Users/alice/private/ER0000-kro55.sl2", loadOriginFileDialog)

	rec := operationEvent(t, app.journal.Tail(), eventSaveLoaded)
	if got := operationField(rec, "save_file"); got != "ER0000-kro55.sl2" {
		t.Fatalf("save_file = %q, want ER0000-kro55.sl2", got)
	}
	// The directory and any full-path form must never survive anywhere in the
	// record, even after the central sanitizer runs.
	for _, f := range rec.Fields {
		if strings.Contains(f.Value, "alice") || strings.Contains(f.Value, "/") {
			t.Errorf("save_loaded field %q=%q leaked path context", f.Key, f.Value)
		}
	}
}
