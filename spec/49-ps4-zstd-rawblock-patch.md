# 49 — PS4 NetworkParam Patch: ZSTD Raw-Block Strategy

> **Type**: Design doc
> **Status**: ✅ Implemented
> **Scope**: How `PatchNetworkParams` modifies regulation.bin on PS4 without triggering the title-screen crash.

---

## Problem

After applying a network preset (invasions, summons, host) and uploading the modified save to PS4, the game crashed at the title screen ("Press any button").

**Root cause**: `PatchNetworkParams` was calling `compressDCX` to recompress the regulation.bin ZSTD frame after patching. The klauspost ZSTD encoder produces a frame with different header flags than FromSoftware's encoder:

| Parameter | FromSoftware | klauspost `SpeedDefault` |
|---|---|---|
| FHD byte | `0x00` | `0x84` |
| Content size field | absent | present (8 bytes) |
| Checksum flag | off | on |
| Window descriptor | present, 64 MB | different size |

A different plaintext produces different ciphertext. The PS4 game validates ciphertext integrity (the AES-256-CBC encrypted block must be byte-for-byte identical to what it expects) and rejects the file, causing the crash.

**PC is not affected** because the PC game validates only the 16-byte MD5 prefix (covering `ud11[0x10:]`), which is recalculated after patching.

---

## Detection

| Platform | `ud11RegulationOffset()` returns | Reason |
|---|---|---|
| PS4 | `ud11UnkSize = 0x10` | no MD5 prefix |
| PC | `ud11MD5Size + ud11UnkSize = 0x20` | 16-byte MD5 prefix at `ud11[0:0x10]` |

`PatchNetworkParams` dispatches on `regStart == ud11UnkSize` (PS4 path) vs `regStart == ud11MD5Size+ud11UnkSize` (PC path).

---

## Solution: ZSTD Raw-Block Replacement

Instead of decompressing and recompressing the full stream, replace only the specific ZSTD block(s) that contain the patched fields with **Raw blocks** (Block_Type=0). All other blocks remain byte-for-byte identical to the original. The resulting stream is then AES-CBC encrypted with the original IV.

### Why this works

- The ZSTD frame header (magic, FHD, window descriptor) is preserved unchanged.
- Blocks outside the patch range are byte-for-byte identical to the original ciphertext.
- A Raw block carries uncompressed payload — the decompressor treats it as verbatim bytes, no Huffman tree required.
- The only bytes that change within the stream are the replaced block(s). The AES-CBC cipher propagates changes through subsequent 16-byte cipher blocks, but PS4 does not validate individual block checksums — only the decryption itself and the DCX decompression need to succeed.

### Block layout

FromSoftware encodes regulation.bin with FLUSH_BLOCK every 64 KB of plaintext:

```
Block 0: BND4 bytes [0x00000, 0x10000)   → 65536 bytes decompressed
Block 1: BND4 bytes [0x10000, 0x20000)
...
Block N: last block (may be < 65536 bytes)
```

Block index for a given BND4 offset:

```
blockIdx = bnd4Offset / 65536
```

### NetworkParam field range

The patched fields span from `offsetReloadSignIntervalTime2` (`0x1C`) through `offsetVisitorDownloadSpan+3` (`0x24B`) within the NetworkParam row data. Both endpoints are expressed as absolute BND4 offsets:

```
firstBND4 = paramOffset + rowDataOffset + 0x1C
lastBND4  = paramOffset + rowDataOffset + 0x24B
```

If the row data straddles a 64 KB boundary, `firstBlockIdx != lastBlockIdx` and all blocks in `[firstBlockIdx, lastBlockIdx]` are replaced.

### Treeless_Literals guard

If the block immediately after the last replaced block has `Literals_Section_Type = Treeless` (value `3` in the low 2 bits of the literals header byte), it reuses the Huffman tree from the preceding Compressed block. Since that block is being replaced with a Raw block (which has no Huffman tree), the successor block would fail to decompress.

Fix: also replace the Treeless successor with a Raw block carrying the corresponding slice of the (modified) BND4 data.

---

## Implementation

`patchZSTDStreamRawBlock(regBlob, bnd4 []byte, firstBND4, lastBND4 int) ([]byte, error)`  
in `backend/core/regulation.go`

Steps:
1. Extract ZSTD stream from `regBlob[76 : 76+compSize]`.
2. Compute `firstBlockIdx`, `lastBlockIdx` from BND4 offsets.
3. Walk blocks with `walkZSTDBlocks` (stop at `lastBlockIdx+3` to cover Treeless check).
4. Validate target blocks are type 2 (Compressed) or type 0 (Raw from a prior patch).
5. Check `blocks[lastBlockIdx+1]` for Treeless_Literals flag; set `replaceAfterBlock` if true.
6. Build new stream:
   - Vanilla prefix: `stream[:firstBlock.streamStart]`
   - Raw replacement blocks for `[firstBlockIdx, lastBlockIdx]` (and optionally `afterIdx`)
   - Vanilla suffix: `stream[replaceStreamEnd:]`
7. Write new DCX: copy `regBlob[:76]`, update `compressedSize` at offset `[32:36]` (big-endian), append new stream.

`walkZSTDBlocks(stream []byte, maxBlocks int) ([]zstdBlock, error)`  
Parses the ZSTD frame header (FHD, window descriptor, DID, FCS fields) to find the first block, then walks block headers accumulating `{streamStart, streamEnd, btype, last}`.

`makeRawBlockHeader(dataSize int, last bool) []byte`  
Returns the 3-byte ZSTD block header: `Block_Size << 3 | Last_Block_Bit`.

---

## Capacity check

Raw blocks expand the stream (no compression). The patched stream must fit within the original AES-CBC ciphertext allocation (`len(ud11) - regStart - 16`). This is checked before encrypting:

```go
if len(newRegBlob) > originalCiphertextLen {
    return nil, fmt.Errorf("patched regulation blob (%d bytes) exceeds ciphertext capacity (%d bytes)", ...)
}
```

In practice, the expanded Raw blocks for NetworkParam.param are well within capacity because regulation.bin is padded to a fixed AES block boundary.

---

## Idempotency

A second call to `patchZSTDStreamRawBlock` on an already-patched stream works correctly: `walkZSTDBlocks` accepts target blocks of type 0 (Raw) in addition to type 2 (Compressed).

---

## PC path (unchanged)

PC saves use `compressDCX` + MD5 recalculation. The MD5 covers `ud11[0x10:]` and is stored in `ud11[0:0x10]`. Since the game validates only the MD5 (not block-level ciphertext fidelity), any valid ZSTD encoding is accepted.

---

## Sources

- `backend/core/regulation.go` — `PatchNetworkParams`, `patchZSTDStreamRawBlock`, `walkZSTDBlocks`, `makeRawBlockHeader`
- `tests/regulation_test.go` — `TestPatchNetworkParams_PC_RoundTrip`, `TestPatchNetworkParams_PS4_RoundTrip`
- `tmp/scripts/gen_rawblock_ps4.go` — reference script used to generate and verify the rawblock approach
- ZSTD frame format: RFC 8878 / zstd spec §3.1 (Block_Header encoding)
