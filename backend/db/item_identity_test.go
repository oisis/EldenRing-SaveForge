package db

import "testing"

func TestWondrousPhysickIdentity(t *testing.T) {
	cases := []struct {
		name    string
		id      uint32
		is      bool
		display uint32
	}{
		{"filled item id", 0x400000FA, true, 0x400000FB},
		{"empty item id", 0x400000FB, true, 0x400000FB},
		{"filled handle", 0xB00000FA, true, 0x400000FB},
		{"empty handle", 0xB00000FB, true, 0x400000FB},
		{"other item", 0x400000FC, false, 0x400000FC},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsWondrousPhysick(tc.id); got != tc.is {
				t.Fatalf("IsWondrousPhysick(0x%08X) = %v, want %v", tc.id, got, tc.is)
			}
			if got := WondrousPhysickDisplayID(tc.id); got != tc.display {
				t.Fatalf("WondrousPhysickDisplayID(0x%08X) = 0x%08X, want 0x%08X", tc.id, got, tc.display)
			}
		})
	}
}
