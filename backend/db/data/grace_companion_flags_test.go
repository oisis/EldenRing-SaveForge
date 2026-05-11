package data

import "testing"

func TestCompanionEventFlagsForGrace_Gatefront(t *testing.T) {
	flags := CompanionEventFlagsForGrace(GatefrontGraceEventFlagID)
	if len(flags) == 0 {
		t.Fatal("expected companion flags for Gatefront grace, got none")
	}

	want := map[uint32]bool{
		EventFlagObtainedSpectralSteedWhistle: true,
		EventFlagMelinaGaveWhistle:            true,
		EventFlagWhistleWorldState:            true,
		EventFlagMelinaAcceptRefusePopup:      true,
	}
	for _, f := range flags {
		if !want[f] {
			t.Errorf("unexpected flag %d in grace companion set", f)
		}
		delete(want, f)
	}
	for missing := range want {
		t.Errorf("missing required grace companion flag %d", missing)
	}
}

func TestCompanionEventFlagsForGrace_Unknown(t *testing.T) {
	flags := CompanionEventFlagsForGrace(0xDEADBEEF)
	if flags != nil {
		t.Errorf("expected nil for unknown grace, got %v", flags)
	}
}

func TestCompanionEventFlagsForGrace_NoForbiddenFlags(t *testing.T) {
	forbidden := []uint32{
		10009655, // Melina RTH invitation trigger
		11109658, // Gideon welcome (RTH visited marker)
		11109659, // Gideon advice
		11109786, // RTH transport trigger (transient)
		4698,     // Melina cutscene trigger (transient)
		4656,     // Level up performed (separate user action)
		710770,   // Melina leaves Gatefront (research candidate)
		69090,    // Melina leaves Gatefront (research candidate)
		69370,    // Melina leaves Gatefront (research candidate)
	}
	for graceID, companions := range graceCompanionEventFlags {
		for _, cf := range companions {
			for _, bad := range forbidden {
				if cf == bad {
					t.Errorf("grace 0x%08X companion set contains forbidden flag %d", graceID, bad)
				}
			}
		}
	}
}
