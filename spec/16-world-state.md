# 16 — World State

> **Type**: Binary format spec  
> **Scope**: FieldArea, WorldArea, WorldGeomMan, RendMan — geometry, NPCs, world objects.

---

## Overview

After Event Flags, a series of structures describe the physical game world state — NPC positions, object state, terrain geometry, renderer data. All have variable length (size-prefixed).

---

## Section order

```
1. FieldArea                    [VARIABLE: 4 + size]
2. WorldArea                    [VARIABLE: 4 + size]
3. WorldGeomMan (instance 1)    [VARIABLE: 4 + size]
4. WorldGeomMan (instance 2)    [VARIABLE: 4 + size]
5. RendMan                      [VARIABLE: 4 + size]
```

---

## 1. FieldArea [VARIABLE]

| Offset | Type | Description |
|---|---|---|
| 0x00 | i32 | Size (number of data bytes after this field) |
| 0x04 | u8[size] | Data |

FieldArea content — data for the region where the player is located. Internal details unknown.

---

## 2. WorldArea [VARIABLE]

| Offset | Type | Description |
|---|---|---|
| 0x00 | i32 | Size |
| 0x04 | u8[size] | Data |

Inside: WorldAreaChrData — NPC/character data in the world, divided into blocks per map:

### WorldAreaChrData (internal structure):
```
┌─────────────────────────────────┐
│ Magic (4 bytes)                  │
│ unk_0x21042700 (u32)            │
│ unk0x8 (u32)                    │
│ unk0xc (u32)                    │
├─────────────────────────────────┤
│ WorldBlockChrData[] (repeated)   │
│   ├── magic (4B)                 │
│   ├── map_id (4B)               │
│   ├── size (i32)                │
│   ├── unk0xc (u32)              │
│   └── data[size-0x10]           │
│ ... (until size < 1 = terminator)│
└─────────────────────────────────┘
```

---

## 3-4. WorldGeomMan (×2) [VARIABLE]

| Offset | Type | Description |
|---|---|---|
| 0x00 | i32 | Size |
| 0x04 | u8[size] | Data |

Inside: WorldGeomData — world geometry per map:

### WorldGeomData (internal structure):
```
┌─────────────────────────────────┐
│ Magic (4 bytes)                  │
│ unk_0x4 (u32)                   │
├─────────────────────────────────┤
│ WorldGeomDataChunk[] (repeated)  │
│   ├── map_id (4B)               │
│   ├── size (i32)                │
│   ├── unk_0x8 (u64)            │
│   └── data[size-0x10]           │
│ ... (until size < 1 = terminator)│
└─────────────────────────────────┘
```

Two instances — likely "before" and "after" state, or two geometry layers.

---

## 5. RendMan [VARIABLE]

| Offset | Type | Description |
|---|---|---|
| 0x00 | i32 | Size |
| 0x04 | u8[size] | Data |

Renderer Manager — renderer state data (lighting, particles, world visual effects).

Inside: StageMan — list of fixed-size entries:
```
count (i32) + count × entry_data
```

---

## Editing implications

- World State sections are large and have variable length — editing requires care
- **Typically not edited directly** — these sections regenerate from game data
- Corrupting these sections = NPCs respawning in wrong places, missing objects
- Safe approach: blob-to-blob copy when transferring saves between platforms
- Size == 0: empty section (legal for new characters)

---

## Sources

- er-save-manager: `parser/world.py` — `FieldArea` (410-437), `WorldArea` (525-552), `WorldGeomMan` (621-648), `RendMan` (709-738)
- er-save-manager: `parser/world.py` — `WorldAreaChrData` (486-522), `WorldGeomData` (589-618)
- er-save-manager: `parser/user_data_x.py` lines 168-172
