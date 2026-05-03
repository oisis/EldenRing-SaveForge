package core

import (
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
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

	// Byte offsets within NETWORK_PARAM_ST (Row 0 data)
	offsetMaxBreakInTargets = 0x70
	offsetBreakInInterval   = 0x74
	offsetBreakInTimeout    = 0x78
)

// NetworkParamValues holds the current invasion matchmaking parameters.
type NetworkParamValues struct {
	MaxBreakInTargetListCount    int32   `json:"maxBreakInTargetListCount"`
	BreakInRequestIntervalTimeSec float32 `json:"breakInRequestIntervalTimeSec"`
	BreakInRequestTimeOutSec     float32 `json:"breakInRequestTimeOutSec"`
}

// NetworkParamDefaults returns the vanilla game defaults.
func NetworkParamDefaults() NetworkParamValues {
	return NetworkParamValues{
		MaxBreakInTargetListCount:    5,
		BreakInRequestIntervalTimeSec: 30.0,
		BreakInRequestTimeOutSec:     20.0,
	}
}

// NetworkParamFast returns the "Fast Invasions" preset.
func NetworkParamFast() NetworkParamValues {
	return NetworkParamValues{
		MaxBreakInTargetListCount:    10,
		BreakInRequestIntervalTimeSec: 4.0,
		BreakInRequestTimeOutSec:     4.0,
	}
}

// ReadNetworkParams extracts current NetworkParam values from UserData11.
func ReadNetworkParams(ud11 []byte) (*NetworkParamValues, error) {
	paramData, _, _, err := locateNetworkParam(ud11)
	if err != nil {
		return nil, err
	}

	vals := &NetworkParamValues{}
	vals.MaxBreakInTargetListCount = int32(binary.LittleEndian.Uint32(paramData[offsetMaxBreakInTargets:]))
	vals.BreakInRequestIntervalTimeSec = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetBreakInInterval:]))
	vals.BreakInRequestTimeOutSec = math.Float32frombits(binary.LittleEndian.Uint32(paramData[offsetBreakInTimeout:]))
	return vals, nil
}

// PatchNetworkParams modifies NetworkParam in UserData11 and returns the patched UserData11.
func PatchNetworkParams(ud11 []byte, patch NetworkParamValues) ([]byte, error) {
	if patch.MaxBreakInTargetListCount < 1 || patch.MaxBreakInTargetListCount > 20 {
		return nil, fmt.Errorf("maxBreakInTargetListCount must be 1-20, got %d", patch.MaxBreakInTargetListCount)
	}
	if patch.BreakInRequestIntervalTimeSec < 2.0 || patch.BreakInRequestIntervalTimeSec > 30.0 {
		return nil, fmt.Errorf("breakInRequestIntervalTimeSec must be 2.0-30.0, got %.1f", patch.BreakInRequestIntervalTimeSec)
	}
	if patch.BreakInRequestTimeOutSec < 3.0 || patch.BreakInRequestTimeOutSec > 20.0 {
		return nil, fmt.Errorf("breakInRequestTimeOutSec must be 3.0-20.0, got %.1f", patch.BreakInRequestTimeOutSec)
	}

	regBlob, iv, dcxFormat, err := extractRegulation(ud11)
	if err != nil {
		return nil, fmt.Errorf("extract regulation: %w", err)
	}

	bnd4Data, err := decompressDCX(regBlob, dcxFormat)
	if err != nil {
		return nil, fmt.Errorf("decompress DCX: %w", err)
	}

	paramOffset, paramSize, rowDataOffset, err := findNetworkParamInBND4(bnd4Data)
	if err != nil {
		return nil, fmt.Errorf("find NetworkParam: %w", err)
	}

	absDataStart := paramOffset + rowDataOffset
	binary.LittleEndian.PutUint32(bnd4Data[absDataStart+offsetMaxBreakInTargets:], uint32(patch.MaxBreakInTargetListCount))
	binary.LittleEndian.PutUint32(bnd4Data[absDataStart+offsetBreakInInterval:], math.Float32bits(patch.BreakInRequestIntervalTimeSec))
	binary.LittleEndian.PutUint32(bnd4Data[absDataStart+offsetBreakInTimeout:], math.Float32bits(patch.BreakInRequestTimeOutSec))

	_ = paramSize

	recompressed, err := compressDCX(bnd4Data, dcxFormat)
	if err != nil {
		return nil, fmt.Errorf("compress DCX: %w", err)
	}

	regStart := ud11RegulationOffset(ud11)
	originalCiphertextLen := len(ud11) - regStart - 16

	reencrypted, err := encryptRegulation(recompressed, iv, originalCiphertextLen)
	if err != nil {
		return nil, fmt.Errorf("encrypt regulation: %w", err)
	}

	result := make([]byte, len(ud11))
	copy(result, ud11)
	copy(result[regStart:], reencrypted)
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
