package core

import (
	"bytes"
	"testing"
)

// The ProfileSummary region helpers are the only way to move or preserve the
// FULL 0x24C record (ProfileSummary.Serialize writes name+level only). They must
// fail closed: a rejected call reports false/nil and leaves the buffer byte-for-
// byte untouched, so a caller can preflight on them.

func fullUserData10() []byte {
	return make([]byte, ProfileSummaryOffset+10*ProfileSummaryStride)
}

func TestProfileSummaryRegionRejectsBadIndexAndShortBuffer(t *testing.T) {
	data := fullUserData10()
	for _, idx := range []int{-1, 10, 11} {
		if got := ProfileSummaryRegion(data, idx); got != nil {
			t.Errorf("ProfileSummaryRegion(idx=%d) = %d bytes, want nil", idx, len(got))
		}
	}
	// One byte short of holding slot 9's record.
	short := data[:len(data)-1]
	if got := ProfileSummaryRegion(short, 9); got != nil {
		t.Errorf("ProfileSummaryRegion on short buffer = %d bytes, want nil", len(got))
	}
	if got := ProfileSummaryRegion(nil, 0); got != nil {
		t.Errorf("ProfileSummaryRegion(nil) = %d bytes, want nil", len(got))
	}
	if got := ProfileSummaryRegion(data, 0); len(got) != ProfileSummaryStride {
		t.Errorf("ProfileSummaryRegion(valid) = %d bytes, want %d", len(got), ProfileSummaryStride)
	}
}

func TestProfileSummaryRegionReturnsIndependentCopy(t *testing.T) {
	data := fullUserData10()
	data[ProfileSummaryOffset] = 0xAB
	rec := ProfileSummaryRegion(data, 0)
	rec[0] = 0xCD
	if data[ProfileSummaryOffset] != 0xAB {
		t.Fatalf("ProfileSummaryRegion aliased the source buffer")
	}
}

func TestSetProfileSummaryRegionFailsClosedWithoutMutating(t *testing.T) {
	valid := bytes.Repeat([]byte{0x5A}, ProfileSummaryStride)
	cases := []struct {
		name string
		idx  int
		rec  []byte
		size int // UserData10 length; 0 = full
	}{
		{"negative index", -1, valid, 0},
		{"index past last slot", 10, valid, 0},
		{"record too short", 0, valid[:ProfileSummaryStride-1], 0},
		{"record too long", 0, append(append([]byte(nil), valid...), 0x00), 0},
		{"nil record", 0, nil, 0},
		{"buffer one byte short", 9, valid, ProfileSummaryOffset + 10*ProfileSummaryStride - 1},
		{"nil buffer", 0, valid, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var data []byte
			switch {
			case tc.size < 0:
				data = nil
			case tc.size == 0:
				data = fullUserData10()
			default:
				data = make([]byte, tc.size)
			}
			before := append([]byte(nil), data...)
			if SetProfileSummaryRegion(data, tc.idx, tc.rec) {
				t.Fatalf("SetProfileSummaryRegion succeeded, want rejection")
			}
			if !bytes.Equal(data, before) {
				t.Fatalf("rejected SetProfileSummaryRegion mutated the buffer")
			}
		})
	}

	data := fullUserData10()
	if !SetProfileSummaryRegion(data, 3, valid) {
		t.Fatalf("SetProfileSummaryRegion(valid) = false, want true")
	}
	off := ProfileSummaryOffset + 3*ProfileSummaryStride
	if !bytes.Equal(data[off:off+ProfileSummaryStride], valid) {
		t.Fatalf("SetProfileSummaryRegion did not write the record")
	}
	if !bytes.Equal(data[:off], make([]byte, off)) {
		t.Fatalf("SetProfileSummaryRegion wrote outside the target record")
	}
}
