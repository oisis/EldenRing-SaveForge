package core

import (
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"unicode/utf16"

	"github.com/klauspost/compress/zstd"
)

// regulationKey is the AES-256-CBC key used to encrypt/decrypt regulation.bin
// inside UserData11 (both PC and PS4 saves use the same key).
var regulationKey = []byte{
	0x99, 0xBF, 0xFC, 0x36, 0x6A, 0x6B, 0xC8, 0xC6,
	0xF5, 0x82, 0x7D, 0x09, 0x36, 0x02, 0xD6, 0x76,
	0xC4, 0x28, 0x92, 0xA0, 0x1C, 0x20, 0x7F, 0xB0,
	0x24, 0xD3, 0xAF, 0x4E, 0x49, 0x3F, 0xEF, 0x99,
}

const (
	ud11UnkSize = 0x10
	ud11MD5Size = 0x10

	dcxMagic  = "DCX\x00"
	bnd4Magic = "BND4"

	dcxFormatDFLT = "DFLT"
	dcxFormatZSTD = "ZSTD"

	networkParamName = "NetworkParam.param"

	// Byte offsets within NETWORK_PARAM_ST (Row 0 data).
	// Calculated sequentially from NetworkParam.xml PARAMDEF.

	// Group: Summon Signs (Cooperator role)
	offsetReloadSignIntervalTime2 = 0x1C
	offsetReloadSignTotalCount    = 0x20
	offsetReloadSignCellCount     = 0x24
	offsetUpdateSignIntervalTime  = 0x28
	offsetSingGetMax              = 0x60
	offsetSignDownloadSpan        = 0x64
	offsetSignUpdateSpan          = 0x68

	// Group: Break-In / Invasions (Invader role)
	offsetMaxBreakInTargets = 0x70
	offsetBreakInInterval   = 0x74
	offsetBreakInTimeout    = 0x78
	// 0x7C is labelled "dummy8 pad[4]" in PARAMDEF but holds vanilla=5 — it is the
	// undocumented breakInRequestAreaCount field (hidden from editors by FromSoftware).
	offsetBreakInAreaCount  = 0x7C

	// Group: Visit / Blue Phantom (Blue + Host role)
	offsetReloadVisitListCoolTime  = 0x180
	offsetMaxCoopBlueSummonCount   = 0x184
	offsetMaxVisitListCount        = 0x18C
	offsetReloadSearchCoopBlueMin  = 0x190
	offsetReloadSearchCoopBlueMax  = 0x194

	// Group: Extra (all roles)
	offsetAllAreaSearchRateCoopBlue = 0x1D8
	offsetAllAreaSearchRateVsBlue   = 0x1D9

	// Group: Visitor / Taunter's Tongue (Host role)
	offsetVisitorListMax     = 0x240
	offsetVisitorTimeOutTime = 0x244
	offsetVisitorDownloadSpan = 0x248
)

// NetworkParamValues holds tunable PvP/multiplayer parameters grouped by player role.
type NetworkParamValues struct {
	// --- Invader role ---
	MaxBreakInTargetListCount     int32   `json:"maxBreakInTargetListCount"`
	BreakInRequestIntervalTimeSec float32 `json:"breakInRequestIntervalTimeSec"`
	BreakInRequestTimeOutSec      float32 `json:"breakInRequestTimeOutSec"`
	BreakInRequestAreaCount       int32   `json:"breakInRequestAreaCount"`

	// --- Cooperator role (summon signs) ---
	ReloadSignIntervalTime2 float32 `json:"reloadSignIntervalTime2"`
	ReloadSignTotalCount    int32   `json:"reloadSignTotalCount"`
	ReloadSignCellCount     int32   `json:"reloadSignCellCount"`
	UpdateSignIntervalTime  float32 `json:"updateSignIntervalTime"`
	SingGetMax              int32   `json:"singGetMax"`
	SignDownloadSpan         float32 `json:"signDownloadSpan"`
	SignUpdateSpan           float32 `json:"signUpdateSpan"`

	// --- Blue role (Blue Cipher Ring) ---
	ReloadVisitListCoolTime    float32 `json:"reloadVisitListCoolTime"`
	MaxCoopBlueSummonCount     int32   `json:"maxCoopBlueSummonCount"`
	MaxVisitListCount          int32   `json:"maxVisitListCount"`
	ReloadSearchCoopBlueMin    float32 `json:"reloadSearchCoopBlueMin"`
	ReloadSearchCoopBlueMax    float32 `json:"reloadSearchCoopBlueMax"`
	AllAreaSearchRateCoopBlue  int32   `json:"allAreaSearchRateCoopBlue"`
	AllAreaSearchRateVsBlue    int32   `json:"allAreaSearchRateVsBlue"`

	// --- Host role (Taunter's Tongue / visitor) ---
	VisitorListMax      int32   `json:"visitorListMax"`
	VisitorTimeOutTime  float32 `json:"visitorTimeOutTime"`
	VisitorDownloadSpan float32 `json:"visitorDownloadSpan"`
}

// NetworkParamDefaults returns the vanilla game defaults for all fields.
func NetworkParamDefaults() NetworkParamValues {
	return NetworkParamValues{
		// Invader
		MaxBreakInTargetListCount:     5,
		BreakInRequestIntervalTimeSec: 30.0,
		BreakInRequestTimeOutSec:      20.0,
		BreakInRequestAreaCount:       5,
		// Cooperator
		ReloadSignIntervalTime2: 60.0,
		ReloadSignTotalCount:    20,
		ReloadSignCellCount:     10,
		UpdateSignIntervalTime:  30.0,
		SingGetMax:              32,
		SignDownloadSpan:        30.0,
		SignUpdateSpan:          60.0,
		// Blue
		ReloadVisitListCoolTime:   20.0,
		MaxCoopBlueSummonCount:    2,
		MaxVisitListCount:         5,
		ReloadSearchCoopBlueMin:   30.0,
		ReloadSearchCoopBlueMax:   180.0,
		AllAreaSearchRateCoopBlue: 30,
		AllAreaSearchRateVsBlue:   30,
		// Host
		VisitorListMax:      10,
		VisitorTimeOutTime:  60.0,
		VisitorDownloadSpan: 60.0,
	}
}

// NetworkParamFastInvasions returns the "Fast Invasions" preset (Invader role).
func NetworkParamFastInvasions() NetworkParamValues {
	d := NetworkParamDefaults()
	d.MaxBreakInTargetListCount = 10
	d.BreakInRequestIntervalTimeSec = 4.0
	d.BreakInRequestTimeOutSec = 4.0
	return d
}

// NetworkParamLightInvasions returns the "Light / Safer" invasions preset.
// Moderate speed-up with a slightly lower detection surface than Fast Invasions.
func NetworkParamLightInvasions() NetworkParamValues {
	d := NetworkParamDefaults()
	d.MaxBreakInTargetListCount = 8
	d.BreakInRequestIntervalTimeSec = 10.0
	d.BreakInRequestTimeOutSec = 8.0
	return d
}

// --- Functional presets (v0.9 unified system) ---
//
// These three presets are the single source of truth for the active Network UI
// (frontend fetches them via GetNetworkPreset). Each preset touches only its own
// role group; the unconfirmed 0x7C field (BreakInRequestAreaCount) is always kept
// at vanilla 5. They replace the removed global "Aggressive" profile.

// NetworkParamFasterReds returns the "Faster Reds" preset (Invader role).
// Faster red invasion search without the regressions of the removed Aggressive
// preset: the request timeout stays generous (15s) so near-and-far matchmaking
// can complete, and the unconfirmed 0x7C field stays at vanilla 5.
func NetworkParamFasterReds() NetworkParamValues {
	d := NetworkParamDefaults()
	d.MaxBreakInTargetListCount = 8
	d.BreakInRequestIntervalTimeSec = 12.0
	d.BreakInRequestTimeOutSec = 15.0
	// BreakInRequestAreaCount (0x7C) intentionally left at vanilla 5 — semantics unconfirmed.
	return d
}

// NetworkParamFasterSummons returns the "Faster Summons & Pools" preset (Cooperator role).
// Faster sign download/upload and a larger, internally consistent sign buffer
// (cellCount <= totalCount <= singGetMax). Spatial cellGroup ranges are left at
// vanilla — they are an Experimental option, not part of this preset.
func NetworkParamFasterSummons() NetworkParamValues {
	d := NetworkParamDefaults()
	d.ReloadSignIntervalTime2 = 20.0
	d.ReloadSignTotalCount = 40
	d.ReloadSignCellCount = 20
	d.UpdateSignIntervalTime = 15.0
	d.SingGetMax = 64
	d.SignDownloadSpan = 15.0
	d.SignUpdateSpan = 20.0
	return d
}

// NetworkParamFasterBlue returns the "Faster Blue / Hunter" preset (Blue role).
// Faster and wider co-op blue search. MaxCoopBlueSummonCount stays at vanilla 2
// (the server caps actual joins; raising it only inflates client-side search).
// AllAreaSearchRateVsBlue stays at vanilla 30 (retribution blue likely legacy in ER).
func NetworkParamFasterBlue() NetworkParamValues {
	d := NetworkParamDefaults()
	d.ReloadVisitListCoolTime = 8.0
	d.ReloadSearchCoopBlueMin = 10.0
	d.ReloadSearchCoopBlueMax = 40.0
	d.MaxVisitListCount = 10
	d.AllAreaSearchRateCoopBlue = 60
	return d
}

// NetworkParamFastSummons returns the "Fast Summons" preset (Cooperator role). Experimental.
func NetworkParamFastSummons() NetworkParamValues {
	d := NetworkParamDefaults()
	d.ReloadSignIntervalTime2 = 15.0
	d.ReloadSignTotalCount = 40
	d.ReloadSignCellCount = 20
	d.UpdateSignIntervalTime = 10.0
	d.SingGetMax = 64
	d.SignDownloadSpan = 10.0
	d.SignUpdateSpan = 15.0
	return d
}

// NetworkParamFastBlue returns the "Fast Blue" preset (Blue role). Experimental.
func NetworkParamFastBlue() NetworkParamValues {
	d := NetworkParamDefaults()
	d.ReloadVisitListCoolTime = 5.0
	d.MaxCoopBlueSummonCount = 4
	d.MaxVisitListCount = 15
	d.ReloadSearchCoopBlueMin = 10.0
	d.ReloadSearchCoopBlueMax = 30.0
	d.AllAreaSearchRateCoopBlue = 75
	d.AllAreaSearchRateVsBlue = 75
	return d
}

// NetworkParamAggressiveHost returns the "Aggressive Host" preset (Host role). Experimental.
func NetworkParamAggressiveHost() NetworkParamValues {
	d := NetworkParamDefaults()
	d.VisitorListMax = 20
	d.VisitorTimeOutTime = 60.0
	d.VisitorDownloadSpan = 10.0
	return d
}

// NetworkParamFast returns the legacy "Fast Invasions" preset for backward compatibility.
func NetworkParamFast() NetworkParamValues {
	return NetworkParamFastInvasions()
}

// ReadNetworkParams extracts current NetworkParam values from UserData11.
func ReadNetworkParams(ud11 []byte) (*NetworkParamValues, error) {
	paramData, _, _, err := locateNetworkParam(ud11)
	if err != nil {
		return nil, err
	}

	vals := &NetworkParamValues{}

	// Invader
	vals.MaxBreakInTargetListCount = int32(binary.LittleEndian.Uint32(paramData[offsetMaxBreakInTargets:]))
	vals.BreakInRequestIntervalTimeSec = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetBreakInInterval:]))
	vals.BreakInRequestTimeOutSec = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetBreakInTimeout:]))
	vals.BreakInRequestAreaCount = int32(binary.LittleEndian.Uint32(paramData[offsetBreakInAreaCount:]))

	// Cooperator
	vals.ReloadSignIntervalTime2 = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetReloadSignIntervalTime2:]))
	vals.ReloadSignTotalCount = int32(binary.LittleEndian.Uint32(paramData[offsetReloadSignTotalCount:]))
	vals.ReloadSignCellCount = int32(binary.LittleEndian.Uint32(paramData[offsetReloadSignCellCount:]))
	vals.UpdateSignIntervalTime = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetUpdateSignIntervalTime:]))
	vals.SingGetMax = int32(binary.LittleEndian.Uint32(paramData[offsetSingGetMax:]))
	vals.SignDownloadSpan = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetSignDownloadSpan:]))
	vals.SignUpdateSpan = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetSignUpdateSpan:]))

	// Blue
	vals.ReloadVisitListCoolTime = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetReloadVisitListCoolTime:]))
	vals.MaxCoopBlueSummonCount = int32(binary.LittleEndian.Uint32(paramData[offsetMaxCoopBlueSummonCount:]))
	vals.MaxVisitListCount = int32(binary.LittleEndian.Uint32(paramData[offsetMaxVisitListCount:]))
	vals.ReloadSearchCoopBlueMin = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetReloadSearchCoopBlueMin:]))
	vals.ReloadSearchCoopBlueMax = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetReloadSearchCoopBlueMax:]))
	vals.AllAreaSearchRateCoopBlue = int32(paramData[offsetAllAreaSearchRateCoopBlue])
	vals.AllAreaSearchRateVsBlue = int32(paramData[offsetAllAreaSearchRateVsBlue])

	// Host
	vals.VisitorListMax = int32(binary.LittleEndian.Uint32(paramData[offsetVisitorListMax:]))
	vals.VisitorTimeOutTime = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetVisitorTimeOutTime:]))
	vals.VisitorDownloadSpan = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetVisitorDownloadSpan:]))

	return vals, nil
}

// ValidateNetworkParams checks all field boundaries. Returns nil if valid.
func ValidateNetworkParams(p NetworkParamValues) error {
	// Invader
	if p.MaxBreakInTargetListCount < 1 || p.MaxBreakInTargetListCount > 20 {
		return fmt.Errorf("maxBreakInTargetListCount must be 1-20, got %d", p.MaxBreakInTargetListCount)
	}
	if p.BreakInRequestIntervalTimeSec < 2.0 || p.BreakInRequestIntervalTimeSec > 30.0 {
		return fmt.Errorf("breakInRequestIntervalTimeSec must be 2-30, got %.0f", p.BreakInRequestIntervalTimeSec)
	}
	if p.BreakInRequestTimeOutSec < 3.0 || p.BreakInRequestTimeOutSec > 20.0 {
		return fmt.Errorf("breakInRequestTimeOutSec must be 3-20, got %.0f", p.BreakInRequestTimeOutSec)
	}
	if p.BreakInRequestAreaCount < 1 || p.BreakInRequestAreaCount > 50 {
		return fmt.Errorf("breakInRequestAreaCount must be 1-50, got %d", p.BreakInRequestAreaCount)
	}
	// Cooperator
	if p.ReloadSignIntervalTime2 < 1.0 || p.ReloadSignIntervalTime2 > 1000.0 {
		return fmt.Errorf("reloadSignIntervalTime2 must be 1-1000, got %.0f", p.ReloadSignIntervalTime2)
	}
	if p.ReloadSignTotalCount < 1 || p.ReloadSignTotalCount > 128 {
		return fmt.Errorf("reloadSignTotalCount must be 1-128, got %d", p.ReloadSignTotalCount)
	}
	if p.ReloadSignCellCount < 1 || p.ReloadSignCellCount > 99 {
		return fmt.Errorf("reloadSignCellCount must be 1-99, got %d", p.ReloadSignCellCount)
	}
	if p.UpdateSignIntervalTime < 1.0 || p.UpdateSignIntervalTime > 1000.0 {
		return fmt.Errorf("updateSignIntervalTime must be 1-1000, got %.0f", p.UpdateSignIntervalTime)
	}
	if p.SingGetMax < 1 || p.SingGetMax > 128 {
		return fmt.Errorf("singGetMax must be 1-128, got %d", p.SingGetMax)
	}
	if p.SignDownloadSpan < 1.0 || p.SignDownloadSpan > 1000.0 {
		return fmt.Errorf("signDownloadSpan must be 1-1000, got %.0f", p.SignDownloadSpan)
	}
	if p.SignUpdateSpan < 1.0 || p.SignUpdateSpan > 1000.0 {
		return fmt.Errorf("signUpdateSpan must be 1-1000, got %.0f", p.SignUpdateSpan)
	}
	// Blue
	if p.ReloadVisitListCoolTime < 1.0 || p.ReloadVisitListCoolTime > 1000.0 {
		return fmt.Errorf("reloadVisitListCoolTime must be 1-1000, got %.0f", p.ReloadVisitListCoolTime)
	}
	if p.MaxCoopBlueSummonCount < 1 || p.MaxCoopBlueSummonCount > 10 {
		return fmt.Errorf("maxCoopBlueSummonCount must be 1-10, got %d", p.MaxCoopBlueSummonCount)
	}
	if p.MaxVisitListCount < 1 || p.MaxVisitListCount > 50 {
		return fmt.Errorf("maxVisitListCount must be 1-50, got %d", p.MaxVisitListCount)
	}
	if p.ReloadSearchCoopBlueMin < 1.0 || p.ReloadSearchCoopBlueMin > 999.0 {
		return fmt.Errorf("reloadSearchCoopBlueMin must be 1-999, got %.0f", p.ReloadSearchCoopBlueMin)
	}
	if p.ReloadSearchCoopBlueMax < 1.0 || p.ReloadSearchCoopBlueMax > 999.0 {
		return fmt.Errorf("reloadSearchCoopBlueMax must be 1-999, got %.0f", p.ReloadSearchCoopBlueMax)
	}
	if p.AllAreaSearchRateCoopBlue < 0 || p.AllAreaSearchRateCoopBlue > 100 {
		return fmt.Errorf("allAreaSearchRateCoopBlue must be 0-100, got %d", p.AllAreaSearchRateCoopBlue)
	}
	if p.AllAreaSearchRateVsBlue < 0 || p.AllAreaSearchRateVsBlue > 100 {
		return fmt.Errorf("allAreaSearchRateVsBlue must be 0-100, got %d", p.AllAreaSearchRateVsBlue)
	}
	// Host
	if p.VisitorListMax < 1 || p.VisitorListMax > 100 {
		return fmt.Errorf("visitorListMax must be 1-100, got %d", p.VisitorListMax)
	}
	if p.VisitorTimeOutTime < 1.0 || p.VisitorTimeOutTime > 600.0 {
		return fmt.Errorf("visitorTimeOutTime must be 1-600, got %.0f", p.VisitorTimeOutTime)
	}
	if p.VisitorDownloadSpan < 1.0 || p.VisitorDownloadSpan > 600.0 {
		return fmt.Errorf("visitorDownloadSpan must be 1-600, got %.0f", p.VisitorDownloadSpan)
	}

	// Cross-field invariants (defensive — the UI also clamps these).
	if p.ReloadSignCellCount > p.ReloadSignTotalCount {
		return fmt.Errorf("reloadSignCellCount (%d) must not exceed reloadSignTotalCount (%d)", p.ReloadSignCellCount, p.ReloadSignTotalCount)
	}
	if p.ReloadSignTotalCount > p.SingGetMax {
		return fmt.Errorf("reloadSignTotalCount (%d) must not exceed singGetMax (%d)", p.ReloadSignTotalCount, p.SingGetMax)
	}
	if p.ReloadSearchCoopBlueMin > p.ReloadSearchCoopBlueMax {
		return fmt.Errorf("reloadSearchCoopBlueMin (%.0f) must not exceed reloadSearchCoopBlueMax (%.0f)", p.ReloadSearchCoopBlueMin, p.ReloadSearchCoopBlueMax)
	}
	return nil
}

// PatchNetworkParams modifies NetworkParam in UserData11 and returns the patched UserData11.
func PatchNetworkParams(ud11 []byte, patch NetworkParamValues) ([]byte, error) {
	if err := ValidateNetworkParams(patch); err != nil {
		return nil, err
	}

	regBlob, iv, dcxFormat, err := extractRegulation(ud11)
	if err != nil {
		return nil, fmt.Errorf("extract regulation: %w", err)
	}

	bnd4Data, err := decompressDCX(regBlob, dcxFormat)
	if err != nil {
		return nil, fmt.Errorf("decompress DCX: %w", err)
	}

	paramOffset, _, rowDataOffset, err := findNetworkParamInBND4(bnd4Data)
	if err != nil {
		return nil, fmt.Errorf("find NetworkParam: %w", err)
	}

	d := bnd4Data[paramOffset+rowDataOffset:]

	// Invader
	binary.LittleEndian.PutUint32(d[offsetMaxBreakInTargets:], uint32(patch.MaxBreakInTargetListCount))
	binary.LittleEndian.PutUint32(d[offsetBreakInInterval:], math.Float32bits(patch.BreakInRequestIntervalTimeSec))
	binary.LittleEndian.PutUint32(d[offsetBreakInTimeout:], math.Float32bits(patch.BreakInRequestTimeOutSec))
	binary.LittleEndian.PutUint32(d[offsetBreakInAreaCount:], uint32(patch.BreakInRequestAreaCount))

	// Cooperator
	binary.LittleEndian.PutUint32(d[offsetReloadSignIntervalTime2:], math.Float32bits(patch.ReloadSignIntervalTime2))
	binary.LittleEndian.PutUint32(d[offsetReloadSignTotalCount:], uint32(patch.ReloadSignTotalCount))
	binary.LittleEndian.PutUint32(d[offsetReloadSignCellCount:], uint32(patch.ReloadSignCellCount))
	binary.LittleEndian.PutUint32(d[offsetUpdateSignIntervalTime:], math.Float32bits(patch.UpdateSignIntervalTime))
	binary.LittleEndian.PutUint32(d[offsetSingGetMax:], uint32(patch.SingGetMax))
	binary.LittleEndian.PutUint32(d[offsetSignDownloadSpan:], math.Float32bits(patch.SignDownloadSpan))
	binary.LittleEndian.PutUint32(d[offsetSignUpdateSpan:], math.Float32bits(patch.SignUpdateSpan))

	// Blue
	binary.LittleEndian.PutUint32(d[offsetReloadVisitListCoolTime:], math.Float32bits(patch.ReloadVisitListCoolTime))
	binary.LittleEndian.PutUint32(d[offsetMaxCoopBlueSummonCount:], uint32(patch.MaxCoopBlueSummonCount))
	binary.LittleEndian.PutUint32(d[offsetMaxVisitListCount:], uint32(patch.MaxVisitListCount))
	binary.LittleEndian.PutUint32(d[offsetReloadSearchCoopBlueMin:], math.Float32bits(patch.ReloadSearchCoopBlueMin))
	binary.LittleEndian.PutUint32(d[offsetReloadSearchCoopBlueMax:], math.Float32bits(patch.ReloadSearchCoopBlueMax))
	d[offsetAllAreaSearchRateCoopBlue] = byte(patch.AllAreaSearchRateCoopBlue)
	d[offsetAllAreaSearchRateVsBlue] = byte(patch.AllAreaSearchRateVsBlue)

	// Host
	binary.LittleEndian.PutUint32(d[offsetVisitorListMax:], uint32(patch.VisitorListMax))
	binary.LittleEndian.PutUint32(d[offsetVisitorTimeOutTime:], math.Float32bits(patch.VisitorTimeOutTime))
	binary.LittleEndian.PutUint32(d[offsetVisitorDownloadSpan:], math.Float32bits(patch.VisitorDownloadSpan))

	regStart := ud11RegulationOffset(ud11)
	originalCiphertextLen := len(ud11) - regStart - 16

	var newRegBlob []byte
	// PS4 saves have no MD5 prefix (regStart == ud11UnkSize), so any recompression of the
	// ZSTD frame produces different ciphertext that PS4 rejects. Use the rawblock approach:
	// replace only the ZSTD block(s) containing the patched fields with Raw blocks,
	// keeping all other blocks byte-for-byte identical to the original encrypted frame.
	//
	// PC saves carry a 16-byte MD5 prefix covering the entire regulation blob; we
	// recalculate the MD5 below, so any valid recompression is accepted on PC.
	if dcxFormat == dcxFormatZSTD && regStart == ud11UnkSize {
		firstBND4 := paramOffset + rowDataOffset + offsetReloadSignIntervalTime2
		lastBND4 := paramOffset + rowDataOffset + offsetVisitorDownloadSpan + 3
		newRegBlob, err = patchZSTDStreamRawBlock(regBlob, bnd4Data, firstBND4, lastBND4)
		if err != nil {
			return nil, fmt.Errorf("rawblock patch: %w", err)
		}
	} else {
		newRegBlob, err = compressDCX(bnd4Data, dcxFormat)
		if err != nil {
			return nil, fmt.Errorf("compress DCX: %w", err)
		}
	}

	if len(newRegBlob) > originalCiphertextLen {
		return nil, fmt.Errorf("patched regulation blob (%d bytes) exceeds ciphertext capacity (%d bytes)",
			len(newRegBlob), originalCiphertextLen)
	}

	reencrypted, err := encryptRegulation(newRegBlob, iv, originalCiphertextLen)
	if err != nil {
		return nil, fmt.Errorf("encrypt regulation: %w", err)
	}

	result := make([]byte, len(ud11))
	copy(result, ud11)
	copy(result[regStart:], reencrypted)

	// PC saves have a 16-byte MD5 prefix at result[0:0x10] covering result[0x10:].
	// The prefix must be recalculated after patching — the game validates it and
	// rejects (overwrites from game files) if it doesn't match.
	if regStart == ud11MD5Size+ud11UnkSize {
		h := md5.Sum(result[ud11MD5Size:])
		copy(result[:ud11MD5Size], h[:])
	}

	return result, nil
}

// --- internal pipeline ---

// ud11RegulationOffset returns the byte offset where regulation blob starts within UserData11.
// PC saves have a 16-byte MD5 prefix before the unk header; PS4 saves start directly with unk.
// The unk header always begins with 0x20474552 (" GER").
func ud11RegulationOffset(ud11 []byte) int {
	if len(ud11) > ud11MD5Size+ud11UnkSize {
		if ud11[ud11MD5Size] == 0x20 && ud11[ud11MD5Size+1] == 0x47 {
			return ud11MD5Size + ud11UnkSize
		}
	}
	return ud11UnkSize
}

func extractRegulation(ud11 []byte) (decrypted []byte, iv []byte, dcxFormat string, err error) {
	regStart := ud11RegulationOffset(ud11)
	if len(ud11) <= regStart+16 {
		return nil, nil, "", fmt.Errorf("UserData11 too short: %d bytes", len(ud11))
	}

	regRaw := ud11[regStart:]
	iv = make([]byte, 16)
	copy(iv, regRaw[:16])
	ciphertext := regRaw[16:]

	block, err := aes.NewCipher(regulationKey)
	if err != nil {
		return nil, nil, "", err
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, nil, "", fmt.Errorf("regulation ciphertext not aligned to block size")
	}

	decrypted = make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decrypted, ciphertext)

	if len(decrypted) < 4 || string(decrypted[:4]) != dcxMagic {
		return nil, nil, "", fmt.Errorf("decrypted regulation does not start with DCX header")
	}

	// Detect compression format at offset 40
	if len(decrypted) < 44 {
		return nil, nil, "", fmt.Errorf("decrypted regulation too short for format detection")
	}
	dcxFormat = string(decrypted[40:44])
	if dcxFormat != dcxFormatDFLT && dcxFormat != dcxFormatZSTD {
		return nil, nil, "", fmt.Errorf("unknown DCX format: %q", dcxFormat)
	}

	return decrypted, iv, dcxFormat, nil
}

func encryptRegulation(dcxData []byte, iv []byte, originalCiphertextLen int) ([]byte, error) {
	plaintext := make([]byte, originalCiphertextLen)
	copy(plaintext, dcxData)

	block, err := aes.NewCipher(regulationKey)
	if err != nil {
		return nil, err
	}

	encrypted := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encrypted, plaintext)

	result := make([]byte, 16+originalCiphertextLen)
	copy(result[:16], iv)
	copy(result[16:], encrypted)
	return result, nil
}

func decompressDCX(dcxData []byte, format string) ([]byte, error) {
	if len(dcxData) < 76 {
		return nil, fmt.Errorf("DCX data too short")
	}

	decompressedSize := int(binary.BigEndian.Uint32(dcxData[28:32]))
	compressedSize := int(binary.BigEndian.Uint32(dcxData[32:36]))

	available := len(dcxData) - 76
	dataLen := compressedSize
	if dataLen > available {
		dataLen = available
	}

	switch format {
	case dcxFormatZSTD:
		compressed := dcxData[76 : 76+dataLen]
		decoder, err := zstd.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, fmt.Errorf("zstd reader: %w", err)
		}
		defer decoder.Close()
		buf := bytes.NewBuffer(make([]byte, 0, decompressedSize))
		if _, err := io.Copy(buf, decoder); err != nil {
			return nil, fmt.Errorf("zstd decompress: %w", err)
		}
		return buf.Bytes(), nil

	case dcxFormatDFLT:
		compressed := dcxData[76 : 76+dataLen]
		if len(compressed) < 2 {
			return nil, fmt.Errorf("DFLT compressed data too short")
		}
		reader := flate.NewReader(bytes.NewReader(compressed[2:]))
		defer reader.Close()
		result, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("deflate decompress: %w", err)
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported DCX format: %s", format)
	}
}

func compressDCX(bnd4Data []byte, format string) ([]byte, error) {
	var compressed []byte

	switch format {
	case dcxFormatZSTD:
		encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
		if err != nil {
			return nil, fmt.Errorf("zstd encoder: %w", err)
		}
		compressed = encoder.EncodeAll(bnd4Data, nil)
		encoder.Close()

	case dcxFormatDFLT:
		var buf bytes.Buffer
		// Write 2-byte zlib header (0x78 0x9C = default compression)
		buf.Write([]byte{0x78, 0x9C})
		writer, err := flate.NewWriter(&buf, flate.DefaultCompression)
		if err != nil {
			return nil, fmt.Errorf("deflate writer: %w", err)
		}
		if _, err := writer.Write(bnd4Data); err != nil {
			return nil, fmt.Errorf("deflate write: %w", err)
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("deflate close: %w", err)
		}
		compressed = buf.Bytes()

	default:
		return nil, fmt.Errorf("unsupported DCX format: %s", format)
	}

	// Build DCX header
	dcx := make([]byte, 76+len(compressed))

	copy(dcx[0:4], []byte(dcxMagic))
	binary.BigEndian.PutUint32(dcx[4:8], 0x11000)
	binary.BigEndian.PutUint32(dcx[8:12], 0x18)
	binary.BigEndian.PutUint32(dcx[12:16], 0x24)
	binary.BigEndian.PutUint32(dcx[16:20], 0x44)
	binary.BigEndian.PutUint32(dcx[20:24], 0x4C)

	// DCS section
	copy(dcx[24:28], []byte("DCS\x00"))
	binary.BigEndian.PutUint32(dcx[28:32], uint32(len(bnd4Data)))
	binary.BigEndian.PutUint32(dcx[32:36], uint32(len(compressed)))

	// DCP section
	copy(dcx[36:40], []byte("DCP\x00"))
	copy(dcx[40:44], []byte(format))
	binary.BigEndian.PutUint32(dcx[44:48], 0x20)

	switch format {
	case dcxFormatZSTD:
		dcx[48] = 0x15 // compression level hint
		// bytes 49-63: zeros
		binary.BigEndian.PutUint32(dcx[64:68], 0x00010100)
	case dcxFormatDFLT:
		dcx[48] = 9  // compression level
		dcx[52] = 15 // window bits
		binary.BigEndian.PutUint32(dcx[64:68], 0x00010100)
	}

	// DCA section
	copy(dcx[68:72], []byte("DCA\x00"))
	binary.BigEndian.PutUint32(dcx[72:76], 8)

	// Compressed data
	copy(dcx[76:], compressed)

	return dcx, nil
}

// --- ZSTD rawblock patch ---

// zstdBlock holds position and type info for one block within a ZSTD compressed stream.
type zstdBlock struct {
	streamStart, streamEnd int
	btype                  int // 0=Raw, 1=RLE, 2=Compressed
	last                   bool
}

// zstdDecompBlockSize is the decompressed size of each ZSTD block in FromSoftware's
// regulation.bin. FromSoftware encodes with FLUSH_BLOCK every 64 KB of plaintext.
const zstdDecompBlockSize = 64 * 1024

// patchZSTDStreamRawBlock replaces the ZSTD block(s) covering the BND4 byte range
// [firstPatchedBND4Offset, lastPatchedBND4Offset] with Raw blocks carrying the
// modified BND4 payload. All other blocks are preserved byte-for-byte from regBlob.
// Returns a new DCX blob with an updated compressedSize field in the header.
//
// Using Raw blocks avoids full ZSTD recompression, which would produce a frame with
// different encoder parameters (window size, frame header flags) that PS4 rejects.
// The patched fields span up to two blocks when the NetworkParam row data straddles
// a 64 KB boundary.
func patchZSTDStreamRawBlock(regBlob, bnd4 []byte, firstPatchedBND4Offset, lastPatchedBND4Offset int) ([]byte, error) {
	if len(regBlob) < 76 {
		return nil, fmt.Errorf("regBlob too short for DCX header (%d bytes)", len(regBlob))
	}
	compSize := int(binary.BigEndian.Uint32(regBlob[32:36]))
	if 76+compSize > len(regBlob) {
		return nil, fmt.Errorf("DCX compressedSize %d exceeds regBlob length %d", compSize, len(regBlob))
	}
	stream := regBlob[76 : 76+compSize]

	firstBlockIdx := firstPatchedBND4Offset / zstdDecompBlockSize
	lastBlockIdx := lastPatchedBND4Offset / zstdDecompBlockSize

	// Walk enough blocks: last target block + 2 (to check the block after for Treeless_Literals).
	blocks, err := walkZSTDBlocks(stream, lastBlockIdx+3)
	if err != nil {
		return nil, fmt.Errorf("walkZSTDBlocks: %w", err)
	}
	if lastBlockIdx >= len(blocks) {
		return nil, fmt.Errorf("ZSTD block %d not found (walked %d blocks)", lastBlockIdx, len(blocks))
	}

	// All target blocks must be Compressed (type 2) or Raw (type 0, from a previous patch).
	// RLE blocks (type 1) are not expected in regulation.bin.
	for idx := firstBlockIdx; idx <= lastBlockIdx; idx++ {
		if b := blocks[idx]; b.btype != 2 && b.btype != 0 {
			return nil, fmt.Errorf("unexpected ZSTD block type %d at idx %d (want Compressed=2 or Raw=0)", b.btype, idx)
		}
	}

	// The block immediately after our replacement range may use Treeless_Literals,
	// which reuses the Huffman tree from the last Compressed block we are removing.
	// Replace it with a Raw block too to avoid decompression failure.
	afterIdx := lastBlockIdx + 1
	replaceStreamEnd := blocks[lastBlockIdx].streamEnd
	replaceAfterBlock := false
	if afterIdx < len(blocks) {
		nxt := blocks[afterIdx]
		if nxt.btype == 2 {
			dataStart := nxt.streamStart + 3
			if dataStart < len(stream) && stream[dataStart]&0x03 == 3 { // Treeless_Literals
				replaceAfterBlock = true
				replaceStreamEnd = nxt.streamEnd
			}
		}
	}

	var newStream bytes.Buffer
	newStream.Write(stream[:blocks[firstBlockIdx].streamStart])

	// Write raw blocks for the target range.
	for idx := firstBlockIdx; idx <= lastBlockIdx; idx++ {
		decompStart := idx * zstdDecompBlockSize
		decompEnd := decompStart + zstdDecompBlockSize
		if decompEnd > len(bnd4) {
			decompEnd = len(bnd4)
		}
		newStream.Write(makeRawBlockHeader(decompEnd-decompStart, false))
		newStream.Write(bnd4[decompStart:decompEnd])
	}

	// Write raw block for the Treeless successor, if any.
	if replaceAfterBlock {
		decompStart := afterIdx * zstdDecompBlockSize
		decompEnd := decompStart + zstdDecompBlockSize
		if decompEnd > len(bnd4) {
			decompEnd = len(bnd4)
		}
		newStream.Write(makeRawBlockHeader(decompEnd-decompStart, false))
		newStream.Write(bnd4[decompStart:decompEnd])
	}

	newStream.Write(stream[replaceStreamEnd:])
	newStreamBytes := newStream.Bytes()

	newDCX := make([]byte, 76+len(newStreamBytes))
	copy(newDCX, regBlob[:76])
	binary.BigEndian.PutUint32(newDCX[32:36], uint32(len(newStreamBytes)))
	copy(newDCX[76:], newStreamBytes)

	return newDCX, nil
}

// walkZSTDBlocks parses the ZSTD frame header and collects block metadata.
// Stops after maxBlocks blocks or when the last block is reached.
func walkZSTDBlocks(stream []byte, maxBlocks int) ([]zstdBlock, error) {
	if len(stream) < 6 {
		return nil, fmt.Errorf("ZSTD stream too short (%d bytes)", len(stream))
	}
	if stream[0] != 0x28 || stream[1] != 0xB5 || stream[2] != 0x2F || stream[3] != 0xFD {
		return nil, fmt.Errorf("invalid ZSTD magic: %X", stream[:4])
	}

	fhd := stream[4]
	singleSeg := (fhd>>5)&1 != 0
	didFlag := int(fhd & 3)
	didSizes := [4]int{0, 1, 2, 4}
	fcsFlag := int((fhd >> 6) & 3)
	// Content_Size field size when Single_Segment=0: FCS=0→0, 1→2, 2→4, 3→8 bytes.
	// When Single_Segment=1: FCS=0→1, 1→2, 2→4, 3→8 bytes.
	fcsSizes := [4]int{0, 2, 4, 8}

	pos := 5
	if !singleSeg {
		pos++ // window descriptor byte
	}
	pos += didSizes[didFlag]
	if singleSeg && fcsFlag == 0 {
		pos++ // 1-byte content size for single-segment with FCS_Flag=0
	} else {
		pos += fcsSizes[fcsFlag]
	}

	var blocks []zstdBlock
	for pos+3 <= len(stream) && len(blocks) < maxBlocks {
		hdr := int(stream[pos]) | int(stream[pos+1])<<8 | int(stream[pos+2])<<16
		last := hdr&1 != 0
		btype := (hdr >> 1) & 3
		bsize := hdr >> 3
		start := pos
		pos += 3

		var end int
		switch btype {
		case 0: // Raw: bsize payload bytes
			end = pos + bsize
			pos += bsize
		case 1: // RLE: 1 byte payload
			end = pos + 1
			pos++
		case 2: // Compressed: bsize payload bytes
			end = pos + bsize
			pos += bsize
		default:
			return nil, fmt.Errorf("reserved ZSTD block type at stream offset %d", start)
		}

		blocks = append(blocks, zstdBlock{start, end, btype, last})
		if last {
			break
		}
	}
	return blocks, nil
}

// makeRawBlockHeader returns the 3-byte ZSTD Raw block header for a block of dataSize bytes.
// Block_Type=0 (Raw), Last_Block=last.
func makeRawBlockHeader(dataSize int, last bool) []byte {
	var lastBit uint32
	if last {
		lastBit = 1
	}
	h := uint32(dataSize)<<3 | lastBit
	return []byte{byte(h), byte(h >> 8), byte(h >> 16)}
}

// locateNetworkParam does the full read pipeline and returns the row data slice.
func locateNetworkParam(ud11 []byte) (rowData []byte, paramOffset int, rowDataOffset int, err error) {
	regBlob, _, _, err := extractRegulation(ud11)
	if err != nil {
		return nil, 0, 0, err
	}

	bnd4Data, err := decompressDCX(regBlob, string(regBlob[40:44]))
	if err != nil {
		return nil, 0, 0, err
	}

	paramOff, _, rowOff, err := findNetworkParamInBND4(bnd4Data)
	if err != nil {
		return nil, 0, 0, err
	}

	return bnd4Data[paramOff+rowOff:], paramOff, rowOff, nil
}

// findNetworkParamInBND4 scans BND4 file entries for NetworkParam.param.
// Returns: paramDataOffset, paramSize, rowDataOffset within the .param file.
func findNetworkParamInBND4(bnd4 []byte) (paramOffset int, paramSize int, rowDataOffset int, err error) {
	if len(bnd4) < 0x40 || string(bnd4[:4]) != bnd4Magic {
		return 0, 0, 0, fmt.Errorf("not a BND4 file")
	}

	fileCount := int(binary.LittleEndian.Uint32(bnd4[0x0C:0x10]))
	const entrySize = 0x24

	targetUTF16 := encodeUTF16LE(networkParamName)

	for i := 0; i < fileCount; i++ {
		entryOff := 0x40 + i*entrySize
		if entryOff+entrySize > len(bnd4) {
			break
		}

		nameOff := int(binary.LittleEndian.Uint32(bnd4[entryOff+32 : entryOff+36]))
		if nameOff <= 0 || nameOff >= len(bnd4) {
			continue
		}

		if matchesUTF16Name(bnd4, nameOff, targetUTF16) {
			compSize := int(binary.LittleEndian.Uint64(bnd4[entryOff+8 : entryOff+16]))
			dataOff := int(binary.LittleEndian.Uint32(bnd4[entryOff+24 : entryOff+28]))

			if dataOff+compSize > len(bnd4) {
				return 0, 0, 0, fmt.Errorf("NetworkParam data exceeds BND4 bounds")
			}

			// Parse PARAM to find Row 0 data offset
			paramData := bnd4[dataOff : dataOff+compSize]
			rowOff, err := parseParamRowDataOffset(paramData)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("parse PARAM: %w", err)
			}

			return dataOff, compSize, rowOff, nil
		}
	}

	return 0, 0, 0, fmt.Errorf("NetworkParam.param not found in BND4 (%d files scanned)", fileCount)
}

// parseParamRowDataOffset reads just enough of the PARAM header to locate Row 0's data.
func parseParamRowDataOffset(paramData []byte) (int, error) {
	if len(paramData) < 0x58 {
		return 0, fmt.Errorf("param file too small (%d bytes)", len(paramData))
	}

	formatFlags := paramData[0x2D]
	longDataOffset := (formatFlags & 0x04) != 0

	if !longDataOffset {
		return 0, fmt.Errorf("expected LongDataOffset format flag, got 0x%02X", formatFlags)
	}

	// Row entries start at 0x40 for OffsetParamType + LongDataOffset format
	// Row format: id(4) + pad(4) + data_offset(8) + name_offset(8) = 24 bytes
	rowDataOff := int(binary.LittleEndian.Uint64(paramData[0x48:0x50]))
	if rowDataOff <= 0 || rowDataOff >= len(paramData) {
		return 0, fmt.Errorf("invalid row data offset: 0x%X", rowDataOff)
	}

	return rowDataOff, nil
}

func encodeUTF16LE(s string) []byte {
	runes := utf16.Encode([]rune(s))
	b := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(b[i*2:], r)
	}
	return b
}

func matchesUTF16Name(data []byte, offset int, target []byte) bool {
	// BND4 names include a full path. We match the suffix (filename).
	// Find the end of the UTF-16LE string (null terminator = 0x0000)
	end := offset
	for end+1 < len(data) {
		if data[end] == 0 && data[end+1] == 0 {
			break
		}
		end += 2
	}

	nameBytes := data[offset:end]
	if len(nameBytes) < len(target) {
		return false
	}

	// Check if the name ends with our target (after last backslash)
	suffix := nameBytes[len(nameBytes)-len(target):]
	return bytes.Equal(suffix, target)
}
