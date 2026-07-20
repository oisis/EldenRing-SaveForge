package core

import (
	"encoding/binary"
	"testing"
)

// TestFlushMetadataWritesSteamIDAtOffset is the regression test for the PC
// SteamID offset bug: UserData10.Data[0x00:0x04) is metadata/version, not part
// of the SteamID, so flushMetadata must write the SteamID little-endian into
// [SteamIDOffset:SteamIDOffset+8) and leave the leading 4 bytes untouched.
func TestFlushMetadataWritesSteamIDAtOffset(t *testing.T) {
	const expectedSteamID = uint64(0x0123456789ABCDEF)
	save := &SaveFile{
		Platform: PlatformPC,
		SteamID:  expectedSteamID,
	}
	save.UserData10.Data = make([]byte, MinSaveFileSize-10*SlotSize)
	prefix := []byte{0xFB, 0x00, 0x00, 0x00}
	copy(save.UserData10.Data[0:4], prefix)

	save.flushMetadata()

	if got := save.UserData10.Data[0:4]; string(got) != string(prefix) {
		t.Errorf("UserData10.Data[0:4] = %x, want unchanged %x", got, prefix)
	}
	got := binary.LittleEndian.Uint64(save.UserData10.Data[SteamIDOffset : SteamIDOffset+8])
	if got != save.SteamID {
		t.Errorf("UserData10.Data[0x04:0x0C] = %d, want %d", got, save.SteamID)
	}
}
