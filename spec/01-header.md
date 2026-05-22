# 01 — Header and File Layout

> **Scope**: Platform detection, BND4 structure, slot layout and checksums in the file.

---

## Platform detection

The first 4 bytes of the file determine the platform:

| Magic Bytes | Platform | Notes |
|---|---|---|
| `42 4E 44 34` ("BND4") | PC (Steam) | Unencrypted |
| `53 4C 32 00` ("SL2\0") | PC (alternative) | Rarely encountered |
| `CB 01 9C 2C` | PS4 / PS5 | No encryption or checksums |

If the first 4 bytes do not match any of the above — the file may be **AES-128-CBC encrypted**. The IV is the first 16 bytes of the file. After decryption it should start with "BND4".

### Sources
- er-save-manager: `parser/save.py` lines 131-142
- Steam Guide: https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037

---

## PC layout (ER0000.sl2)

```
Offset          Size            Content
─────────────────────────────────────────────────────────
0x000           0x300           BND4 Header (FromSoftware container)
0x300           0x010           MD5 Checksum — Slot 0
0x310           0x280000        SaveSlot[0] — character data
0x280310        0x010           MD5 Checksum — Slot 1
0x280320        0x280000        SaveSlot[1]
...             ...             (repeats ×10 slots)
0x19003A0       0x010           MD5 Checksum — UserData10
0x19003B0       0x60000         UserData10 (account profile)
0x19603B0       ~0x240010       UserData11 (regulation.bin)
─────────────────────────────────────────────────────────
TOTAL:          ~28.9 MB
```

### Slot N offset formula (PC):
- **Checksum**: `0x300 + N × 0x280010`
- **Data**: `0x310 + N × 0x280010`

---

## PS4 layout (memory.dat)

```
Offset          Size            Content
─────────────────────────────────────────────────────────
0x000           0x070           PS4 Header (constant)
0x070           0x280000        SaveSlot[0] (no MD5)
0x280070        0x280000        SaveSlot[1]
...             ...             (×10 slots)
0x1900070       0x60000         UserData10 (no MD5)
0x1960070       ~0x240010       UserData11
─────────────────────────────────────────────────────────
```

### Slot N offset formula (PS4):
- **Data**: `0x70 + N × 0x280000`

---

## BND4 Header (0x300 bytes) — PC only

Standard FromSoftware container. Contains metadata about 11 "files" inside (10 slots + UserData).

### Header structure (first 0x40 bytes):

| Offset | Type | Value | Description |
|---|---|---|---|
| 0x00 | char[4] | "BND4" | Magic identifier |
| 0x04 | u32 | 0x00000000 | Constant |
| 0x08 | u32 | 0x00010000 | Revision number (speculation) |
| 0x0C | u32 | 11 | Slot count (10 characters + UserData) |
| 0x10 | u32 | 0x00000040 | Constant |
| 0x14 | u32 | 0x00000000 | Constant |
| 0x18 | u32 | 0x00000020 | Slot header entry size |
| 0x1C | u32 | 0x000002C0 | Whole file header size? |
| 0x20 | u32 | 0x00000000 | Constant |
| 0x24 | u32 | 0x00002001 | Flags? |
| 0x28 | u8[12] | 0x00... | Padding |

After the header — 11 × Slot Header Entry (0x20 bytes each) describing the offset and size of each "file".

### Sources
- SoulsFormats: https://github.com/JKAnderson/SoulsFormats
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=format:sl2
- ER-Save-Editor (Rust): `src/save/pc/save_header.rs` — reads 0x300 as an opaque blob

---

## MD5 Checksums (PC only)

Each slot has a 16-byte prefix with an MD5 hash computed from the slot data (0x280000 bytes).

- **On read**: the game verifies MD5 — mismatch = save rejected
- **On write**: the editor MUST recompute MD5 after modifying slot data
- **PS4/PS5**: no checksums — slot data begins directly

### Recomputation algorithm:
```
checksum = MD5(slot_data[0:0x280000])
write checksum at (0x300 + slot_index * 0x280010)
```

### Sources
- Steam Guide: https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037
- er-save-manager: `parser/save.py` method `recalculate_checksums()`

---

## AES-128-CBC encryption (PC only, optional)

Older save versions may be encrypted. Newer game versions write unencrypted "BND4".

- **IV**: first 16 bytes of the file
- **Key**: constant, embedded in the game exe
- **After decryption**: the file starts with "BND4"

### Sources
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=format:sl2
- ER-Save-Editor (Rust): handles decrypt in `src/save/pc/`

---

## PS4 Header (0x70 bytes)

Constant PS4 header. Magic: `CB 01 9C 2C`. The rest of the header is constant and requires no editing.

### Sources
- er-save-manager: `parser/save.py` line 146 (`header_size = 0x6C` after magic)

---

## Empty slots

A slot is empty when:
- **PC**: checksum = 16 × `0x00` (all zeros)
- **In general**: `version` (the first u32 of slot data) = 0

### Sources
- er-save-manager: `parser/save.py` lines 174-178
- er-save-manager: `parser/user_data_x.py` — method `is_empty()`
