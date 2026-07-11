package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
)

// ps4HeaderTemplate is the canonical 0x70-byte PS4 save header.
// Identical across all saves (verified against two different PS4 saves).
var ps4HeaderTemplate = [0x70]byte{
	0xCB, 0x01, 0x9C, 0x2C, 0x00, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F, 0x00, 0x00, 0x00, 0x00,
	0x07, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F, 0x08, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F,
	0x09, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F, 0x0A, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F,
	0x0B, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F, 0x0C, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F,
	0x0D, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F, 0x0E, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F,
	0x0F, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F, 0x10, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F,
	0x11, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F, 0x12, 0x00, 0x00, 0x00, 0x7F, 0x7F, 0x7F, 0x7F,
}

// buildPCBND4Header constructs the canonical 0x300-byte BND4 container header for PC saves.
// The only variable field is UserData11 (regulation) size; all slot and UserData10 offsets
// are fixed because each slot is always 0x280000 bytes and UserData10 is always 0x60000 bytes.
func buildPCBND4Header(userData11Len int) []byte {
	le := binary.LittleEndian
	h := make([]byte, 0x300)

	// BND4 outer header (0x00–0x3F)
	copy(h[0x00:], []byte("BND4"))
	le.PutUint32(h[0x08:], 0x00010000)
	le.PutUint32(h[0x0C:], 12) // 12 entries
	le.PutUint64(h[0x10:], 0x40)
	copy(h[0x18:], []byte("00000001"))
	le.PutUint64(h[0x20:], 0x20) // entry size
	le.PutUint64(h[0x28:], 0x300)
	le.PutUint64(h[0x30:], 0x2001)

	// Entry table (12 × 0x20 bytes starting at 0x40).
	// Slot entries: [0x10 MD5 + 0x280000 data] each.
	// UserData10:   [0x10 MD5 + 0x60000 data].
	// UserData11:   no MD5 prefix, variable size.
	const slotBlock = uint32(0x280010) // 0x10 MD5 + 0x280000 data
	const ud10Block = uint32(0x60010)  // 0x10 MD5 + 0x60000 data
	const nameStride = uint32(0x1A)    // 12 chars UTF-16LE + 2-byte null = 26 bytes

	for i := 0; i < 12; i++ {
		off := 0x40 + i*0x20
		var size, dataOff, nameOff uint32

		switch {
		case i < 10:
			size = slotBlock
			dataOff = 0x300 + uint32(i)*slotBlock
			nameOff = 0x1C0 + uint32(i)*nameStride
		case i == 10:
			size = ud10Block
			dataOff = 0x300 + 10*slotBlock
			nameOff = 0x1C0 + 10*nameStride
		default: // i == 11: UserData11 (regulation)
			size = uint32(userData11Len)
			dataOff = 0x300 + 10*slotBlock + ud10Block
			nameOff = 0x1C0 + 11*nameStride
		}

		le.PutUint32(h[off+0x00:], 0x50)
		le.PutUint32(h[off+0x04:], 0xFFFFFFFF)
		le.PutUint32(h[off+0x08:], size)
		le.PutUint32(h[off+0x10:], dataOff)
		le.PutUint32(h[off+0x14:], nameOff)
	}

	// Name table: "USER_DATAxxx\0" in UTF-16LE starting at 0x1C0.
	for i := 0; i < 12; i++ {
		nameOff := 0x1C0 + i*int(nameStride)
		name := fmt.Sprintf("USER_DATA%03d", i)
		for j := 0; j < len(name); j++ {
			h[nameOff+j*2] = name[j] // high byte stays 0 (ASCII-safe)
		}
	}

	return h
}

type Platform string

const (
	PlatformPC Platform = "PC"
	PlatformPS Platform = "PS4"
)

type SaveFile struct {
	Platform         Platform
	Encrypted        bool
	IV               []byte
	Header           []byte
	Slots            [10]SaveSlot
	SteamID          uint64
	UserData10       CSMenuSystemSaveLoad
	ActiveSlots      [10]bool
	ProfileSummaries [10]ProfileSummary
	UserData11       []byte
}

// MinSaveFileSize is the minimum valid save file size: 10 slots × 0x280000 + UserData10 (0x60000).
const MinSaveFileSize = 10*SlotSize + 0x60000

func LoadSave(path string) (*SaveFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if len(data) < MinSaveFileSize {
		return nil, fmt.Errorf("file too small (%d bytes, minimum %d) — not a valid save", len(data), MinSaveFileSize)
	}

	save := &SaveFile{}

	// Strict, unambiguous container detection only. We accept a native PC
	// container (raw BND4) or a native PS4 container (raw PS4 magic) and
	// nothing else. Encrypted/unknown containers are NOT decrypted-and-guessed:
	// treating AES→BND4 as PC would let a PS-origin file be opened and later
	// written back in the wrong platform format. Reject instead — no platform
	// is ever guessed on load.
	switch ClassifyContainer(data) {
	case PlatformPC:
		save.Platform = PlatformPC
		save.Encrypted = false
		return loadPCSequential(NewReader(data), save)
	case PlatformPS:
		save.Platform = PlatformPS
		return loadPSSequential(NewReader(data), save)
	default:
		return nil, ErrUnsupportedContainer
	}
}

// UnsupportedContainerCode is a stable, machine-matchable discriminator prefixed
// to ErrUnsupportedContainer's message so the frontend can reliably detect this
// specific failure (and show a dedicated modal) without matching prose.
const UnsupportedContainerCode = "ERR_UNSUPPORTED_CONTAINER"

// ErrUnsupportedContainer is returned by LoadSave when the input container is
// not an unambiguous native save (raw BND4 for PC, raw PS4 magic for PS4).
// User-facing so the frontend can explain why the file was not opened.
var ErrUnsupportedContainer = fmt.Errorf("%s: this file's save format could not be identified safely (not a native PC or PS4 save). It will not be opened, to avoid writing it back in the wrong platform format. Format conversion is currently unavailable", UnsupportedContainerCode)

// ClassifyContainer returns the platform of an unambiguous native save
// container, or "" when the container is ambiguous/unsupported. Detection is
// purely by leading container magic — never by decryption, and never by
// filename. An AES-encrypted PC save (no raw BND4 prefix) is intentionally
// classified as unsupported.
func ClassifyContainer(data []byte) Platform {
	if bytes.HasPrefix(data, []byte("BND4")) {
		return PlatformPC
	}
	if bytes.HasPrefix(data, ps4HeaderTemplate[:4]) {
		return PlatformPS
	}
	return ""
}

func loadPCSequential(r *Reader, save *SaveFile) (*SaveFile, error) {
	var err error
	save.Header, err = r.ReadBytes(0x300)
	if err != nil {
		return nil, fmt.Errorf("failed to read PC header: %w", err)
	}

	for i := 0; i < 10; i++ {
		slotStart := r.Pos()
		if _, err := r.ReadBytes(0x10); err != nil {
			return nil, fmt.Errorf("failed to read MD5 for slot %d: %w", i, err)
		}
		if err := save.Slots[i].Read(r, "PC"); err != nil {
			fmt.Printf("Warning: failed to parse slot %d: %v\n", i, err)
		}
		r.Seek(int64(slotStart+0x10+0x280000), 0)
	}

	// UserData10
	udStart := r.Pos()
	if _, err := r.ReadBytes(0x10); err != nil {
		return nil, fmt.Errorf("failed to read UserData10 MD5: %w", err)
	}
	save.UserData10.Data, err = r.ReadBytes(0x60000)
	if err != nil {
		return nil, fmt.Errorf("failed to read UserData10: %w", err)
	}

	udReader := NewReader(save.UserData10.Data)

	// SteamID is at the beginning of UserData10 data on PC
	save.SteamID, err = udReader.ReadU64()
	if err != nil {
		return nil, fmt.Errorf("failed to read SteamID: %w", err)
	}

	// Active Slots @ 0x1954 (10 × u8); ProfileSummary[i] @ 0x195E + i*0x24C.
	// Verified Apr 2026 from real saves (tmp/re-character/ + tmp/save/oisisk_ps4.txt).
	// Layout is identical across PC and PS4. See spec/23-user-data-10.md.
	udReader.Seek(0x1954, 0)
	for i := 0; i < 10; i++ {
		b, err := udReader.ReadU8()
		if err != nil {
			return nil, fmt.Errorf("failed to read ActiveSlot %d: %w", i, err)
		}
		save.ActiveSlots[i] = b == 1
	}
	for i := 0; i < 10; i++ {
		save.ProfileSummaries[i].Read(udReader)
	}

	r.Seek(int64(udStart+0x10+0x60000), 0)
	remaining := r.Len() - r.Pos()
	if remaining > 0 {
		save.UserData11, err = r.ReadBytes(int(remaining))
		if err != nil {
			return nil, fmt.Errorf("failed to read UserData11: %w", err)
		}
	}

	return save, nil
}

func loadPSSequential(r *Reader, save *SaveFile) (*SaveFile, error) {
	var err error
	save.Header, err = r.ReadBytes(0x70)
	if err != nil {
		return nil, fmt.Errorf("failed to read PS4 header: %w", err)
	}

	for i := 0; i < 10; i++ {
		slotStart := r.Pos()
		if err := save.Slots[i].Read(r, "PS4"); err != nil {
			fmt.Printf("Warning: failed to parse slot %d: %v\n", i, err)
		}
		r.Seek(int64(slotStart+0x280000), 0)
	}

	save.UserData10.Data, err = r.ReadBytes(0x60000)
	if err != nil {
		return nil, fmt.Errorf("failed to read PS4 UserData10: %w", err)
	}
	udReader := NewReader(save.UserData10.Data)

	// PS4 UserData10 layout is identical to PC (verified Apr 2026, oisisk_ps4.txt):
	// Active Slots @ 0x1954, ProfileSummary[i] @ 0x195E + i*0x24C. See spec/23-user-data-10.md.
	udReader.Seek(0x1954, 0)
	for i := 0; i < 10; i++ {
		b, err := udReader.ReadU8()
		if err != nil {
			return nil, fmt.Errorf("failed to read PS4 ActiveSlot %d: %w", i, err)
		}
		save.ActiveSlots[i] = b == 1
	}
	for i := 0; i < 10; i++ {
		save.ProfileSummaries[i].Read(udReader)
	}

	remaining := r.Len() - r.Pos()
	if remaining > 0 {
		save.UserData11, err = r.ReadBytes(int(remaining))
		if err != nil {
			return nil, fmt.Errorf("failed to read PS4 UserData11: %w", err)
		}
	}

	return save, nil
}

// flushMetadata writes in-memory SteamID, ActiveSlots, and ProfileSummaries
// back to UserData10.Data before serialization.
//
// PC and PS4 share the same UserData10 layout — only the SteamID prefix and the
// PC-only MD5 checksum differ. ActiveSlots @ 0x1954, ProfileSummary[i] @ 0x195E + i*0x24C.
// Verified Apr 2026 from real saves; see spec/23-user-data-10.md.
func (s *SaveFile) flushMetadata() {
	if s.Platform == PlatformPC {
		binary.LittleEndian.PutUint64(s.UserData10.Data[0:], s.SteamID)
	}
	for i := 0; i < 10; i++ {
		if s.ActiveSlots[i] {
			s.UserData10.Data[ActiveSlotsOffset+i] = 1
		} else {
			s.UserData10.Data[ActiveSlotsOffset+i] = 0
		}
	}
	for i := 0; i < 10; i++ {
		s.ProfileSummaries[i].Serialize(s.UserData10.Data, ProfileSummaryOffset+i*ProfileSummaryStride)
	}
}

func (s *SaveFile) SaveFile(path string) error {
	// Write-ahead validation: check all active slots before writing anything.
	for i := 0; i < 10; i++ {
		if !s.ActiveSlots[i] {
			continue
		}
		if err := ValidateSlotIntegrity(&s.Slots[i]); err != nil {
			return fmt.Errorf("slot %d integrity check failed: %w", i, err)
		}
	}

	s.flushMetadata()

	// Resolve the header for the target platform, handling cross-platform conversions.
	header := s.Header
	crossPlatform := false
	if s.Platform == PlatformPC {
		if len(header) < 4 || !bytes.HasPrefix(header, []byte("BND4")) {
			// PS4→PC conversion: original header is 0x70 PS4 bytes, rebuild BND4.
			header = buildPCBND4Header(len(s.UserData11))
			crossPlatform = true
		}
	} else {
		if len(header) != 0x70 || bytes.HasPrefix(header, []byte("BND4")) {
			// PC→PS4 conversion: replace BND4 header with canonical PS4 header.
			header = ps4HeaderTemplate[:]
			crossPlatform = true
		}
	}

	// Sanitize DLC entry flag on cross-platform conversion.
	// DLC byte[1] (Shadow of the Erdtree entry flag): non-zero means the character has
	// entered the DLC area. If the target platform/account doesn't own the DLC, this
	// causes infinite loading. Zero it on conversion as a safety measure.
	if crossPlatform {
		for i := 0; i < 10; i++ {
			if !s.ActiveSlots[i] {
				continue
			}
			dlcOff := DlcSectionOffset + DlcEntryFlagByte
			if dlcOff < len(s.Slots[i].Data) {
				s.Slots[i].Data[dlcOff] = 0
			}
		}
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)

	if s.Platform == PlatformPC {
		w.WriteBytes(header)
		for i := 0; i < 10; i++ {
			slotData := s.Slots[i].Write("PC")
			checksum := ComputeMD5(slotData)
			w.WriteBytes(checksum[:])
			w.WriteBytes(slotData)
		}

		udData := s.UserData10.Data
		checksum := ComputeMD5(udData)
		w.WriteBytes(checksum[:])
		w.WriteBytes(udData)
		w.WriteBytes(s.UserData11)
	} else {
		w.WriteBytes(header)
		for i := 0; i < 10; i++ {
			w.WriteBytes(s.Slots[i].Write("PS4"))
		}
		w.WriteBytes(s.UserData10.Data)
		w.WriteBytes(s.UserData11)
	}

	finalData := buf.Bytes()
	if s.Encrypted && s.Platform == PlatformPC {
		var err error
		finalData, err = EncryptSave(finalData, s.IV)
		if err != nil {
			return fmt.Errorf("failed to encrypt save: %w", err)
		}
	}

	// Atomic write: write to a temp file first, then rename into place.
	// On failure, preserve .tmp — it contains the user's data.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, finalData, 0644); err != nil {
		return fmt.Errorf("failed to write save data: %w", err)
	}

	// Windows: os.Rename cannot overwrite an existing file — remove target first.
	if runtime.GOOS == "windows" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot remove old file for atomic write: %w (new data preserved in %s)", err, tmpPath)
		}
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename failed: %w (new data preserved in %s)", err, tmpPath)
	}
	return nil
}
