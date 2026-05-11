package data

import "testing"

func TestCompanionEventFlagsForItem_Whistle(t *testing.T) {
	flags := CompanionEventFlagsForItem(ItemSpectralSteedWhistle)
	if len(flags) == 0 {
		t.Fatal("expected companion flags for Spectral Steed Whistle, got none")
	}

	want := map[uint32]bool{
		EventFlagObtainedSpectralSteedWhistle: true,
		EventFlagMelinaGaveWhistle:            true,
		EventFlagWhistleWorldState:            true,
		EventFlagMelinaAcceptRefusePopup:      true,
	}
	for _, f := range flags {
		if !want[f] {
			t.Errorf("unexpected flag %d in companion set", f)
		}
		delete(want, f)
	}
	for missing := range want {
		t.Errorf("missing required companion flag %d", missing)
	}
}

func TestCompanionEventFlagsForItem_Unknown(t *testing.T) {
	flags := CompanionEventFlagsForItem(0xDEADBEEF)
	if flags != nil {
		t.Errorf("expected nil for unknown item, got %v", flags)
	}
}

func TestCompanionEventFlagsForItem_NoTransientFlags(t *testing.T) {
	// These flags are transient (cleared by game after use) — must never appear.
	forbidden := []uint32{
		4698, // Melina cutscene trigger — cleared in step 7
		4651, // Melina dialogue state
		4652, // Melina dialogue state
		4653, // Melina dialogue state
		4656, // Level up performed — separate user action
	}
	for itemID, companions := range itemCompanionEventFlags {
		for _, cf := range companions {
			for _, bad := range forbidden {
				if cf == bad {
					t.Errorf("item 0x%08X companion set contains forbidden transient flag %d", itemID, bad)
				}
			}
		}
	}
}
