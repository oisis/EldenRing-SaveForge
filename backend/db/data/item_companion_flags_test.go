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

func TestCompanionEventFlagsForItem_SmallGoldenEffigy(t *testing.T) {
	flags := CompanionEventFlagsForItem(ItemSmallGoldenEffigy)
	if len(flags) == 0 {
		t.Fatal("expected companion flags for Small Golden Effigy, got none")
	}
	if len(flags) != 1 {
		t.Errorf("expected exactly 1 companion flag, got %d: %v", len(flags), flags)
	}
	if flags[0] != EventFlagObtainedSmallGoldenEffigy {
		t.Errorf("expected flag %d, got %d", EventFlagObtainedSmallGoldenEffigy, flags[0])
	}
}

func TestCompanionEventFlagsForItem_SmallGoldenEffigy_NoForbiddenFlags(t *testing.T) {
	// Flags that must never appear in the Small Golden Effigy companion set.
	forbidden := []uint32{
		60220, 60240, 60250, 60260, 60270, 60300, 60310, // other multiplayer item flags
		60100, 4680, 4681, 710520,                        // Spectral Steed Whistle flags
		670000, 670010, 670020, 670030, 670040, 670050,   // Summoning Pool 670xxx (sample)
	}
	flags := CompanionEventFlagsForItem(ItemSmallGoldenEffigy)
	for _, cf := range flags {
		for _, bad := range forbidden {
			if cf == bad {
				t.Errorf("Small Golden Effigy companion set contains forbidden flag %d", cf)
			}
		}
		// Reject entire 670xxx range.
		if cf >= 670000 && cf < 680000 {
			t.Errorf("Small Golden Effigy companion set contains Summoning Pool flag %d (670xxx range)", cf)
		}
	}
}

func TestCompanionEventFlagsForItem_DuelistsFurledFinger(t *testing.T) {
	flags := CompanionEventFlagsForItem(ItemDuelistsFurledFinger)
	if len(flags) != 1 {
		t.Fatalf("expected exactly 1 companion flag for Duelist's Furled Finger, got %d: %v", len(flags), flags)
	}
	if flags[0] != EventFlagObtainedDuelistsFurledFinger {
		t.Errorf("expected flag %d, got %d", EventFlagObtainedDuelistsFurledFinger, flags[0])
	}
}

func TestCompanionEventFlagsForItem_SmallRedEffigy(t *testing.T) {
	flags := CompanionEventFlagsForItem(ItemSmallRedEffigy)
	if len(flags) != 1 {
		t.Fatalf("expected exactly 1 companion flag for Small Red Effigy, got %d: %v", len(flags), flags)
	}
	if flags[0] != EventFlagObtainedSmallRedEffigy {
		t.Errorf("expected flag %d, got %d", EventFlagObtainedSmallRedEffigy, flags[0])
	}
}

func TestCompanionEventFlagsForItem_MultiplayerPickup_NoForbiddenFlags(t *testing.T) {
	// Flags that must never appear in any multiplayer pickup item companion set.
	forbidden := []uint32{
		60220, 60260, 60270, 60300, 60310, // other multiplayer item flags not in this commit
		60100, 4680, 4681, 710520,          // Spectral Steed Whistle flags
	}
	items := []uint32{ItemSmallGoldenEffigy, ItemDuelistsFurledFinger, ItemSmallRedEffigy}
	for _, itemID := range items {
		for _, cf := range CompanionEventFlagsForItem(itemID) {
			for _, bad := range forbidden {
				if cf == bad {
					t.Errorf("item 0x%08X companion set contains forbidden flag %d", itemID, bad)
				}
			}
			if cf >= 670000 && cf < 680000 {
				t.Errorf("item 0x%08X companion set contains Summoning Pool flag %d (670xxx)", itemID, cf)
			}
		}
	}
}

func TestCompanionEventFlagsForItem_MultiplayerPickup_NoCrossContamination(t *testing.T) {
	// Each item must contain only its own obtained flag and not those of the others.
	cases := []struct {
		itemID   uint32
		ownFlag  uint32
		otherFlags []uint32
	}{
		{ItemSmallGoldenEffigy, EventFlagObtainedSmallGoldenEffigy,
			[]uint32{EventFlagObtainedDuelistsFurledFinger, EventFlagObtainedSmallRedEffigy}},
		{ItemDuelistsFurledFinger, EventFlagObtainedDuelistsFurledFinger,
			[]uint32{EventFlagObtainedSmallGoldenEffigy, EventFlagObtainedSmallRedEffigy}},
		{ItemSmallRedEffigy, EventFlagObtainedSmallRedEffigy,
			[]uint32{EventFlagObtainedSmallGoldenEffigy, EventFlagObtainedDuelistsFurledFinger}},
	}
	for _, tc := range cases {
		flags := CompanionEventFlagsForItem(tc.itemID)
		found := false
		for _, f := range flags {
			if f == tc.ownFlag {
				found = true
			}
			for _, other := range tc.otherFlags {
				if f == other {
					t.Errorf("item 0x%08X companion set contains flag %d belonging to a different item", tc.itemID, f)
				}
			}
		}
		if !found {
			t.Errorf("item 0x%08X missing its own obtained flag %d", tc.itemID, tc.ownFlag)
		}
	}
}
