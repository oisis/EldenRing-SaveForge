package core

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// canonicalEmptyMarker is the verified 8-byte native empty GaItem record:
// handle=0, itemID=0xFFFFFFFF.
var canonicalEmptyMarker = []byte{0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF}

func TestGaItemFullSerialize_ZeroValueEmitsCanonicalMarker(t *testing.T) {
	var g GaItemFull // zero value: Handle=0, ItemID=0
	buf := make([]byte, 8)
	n := g.Serialize(buf)
	if n != GaRecordItem {
		t.Fatalf("size = %d, want %d", n, GaRecordItem)
	}
	if !bytes.Equal(buf, canonicalEmptyMarker) {
		t.Fatalf("bytes = % x, want % x", buf, canonicalEmptyMarker)
	}
	// Must not mutate the in-memory object.
	if g.Handle != 0 || g.ItemID != 0 {
		t.Fatalf("object mutated: %+v", g)
	}
}

func TestGaItemFullSerialize_InvalidItemIDEmitsCanonicalMarker(t *testing.T) {
	g := GaItemFull{Handle: 0x1234, ItemID: GaHandleInvalid}
	buf := make([]byte, 8)
	n := g.Serialize(buf)
	if n != GaRecordItem {
		t.Fatalf("size = %d, want %d", n, GaRecordItem)
	}
	if !bytes.Equal(buf, canonicalEmptyMarker) {
		t.Fatalf("bytes = % x, want % x", buf, canonicalEmptyMarker)
	}
}

func TestGaItemFullSerialize_NonEmptyWeaponUnchanged(t *testing.T) {
	g := GaItemFull{
		Handle:          0x0A0B0C0D,
		ItemID:          0x00112233, // weapon (top nibble 0)
		Unk2:            -1,
		Unk3:            -1,
		AoWGaItemHandle: 0xDEADBEEF,
		Unk5:            0x7F,
	}
	buf := make([]byte, GaRecordWeapon)
	n := g.Serialize(buf)
	if n != GaRecordWeapon {
		t.Fatalf("size = %d, want %d", n, GaRecordWeapon)
	}
	want := make([]byte, GaRecordWeapon)
	binary.LittleEndian.PutUint32(want[0:], g.Handle)
	binary.LittleEndian.PutUint32(want[4:], g.ItemID)
	binary.LittleEndian.PutUint32(want[8:], uint32(g.Unk2))
	binary.LittleEndian.PutUint32(want[12:], uint32(g.Unk3))
	binary.LittleEndian.PutUint32(want[16:], g.AoWGaItemHandle)
	want[20] = g.Unk5
	if !bytes.Equal(buf, want) {
		t.Fatalf("bytes = % x, want % x", buf, want)
	}
}
