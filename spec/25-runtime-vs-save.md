# 25 — Runtime vs Save File Offsets

> **Scope**: Mapping between memory offsets (Cheat Engine) and save-file offsets. Warnings and conversions.

---

## The problem

Cheat Engine tables operate on **runtime memory** — data in the game process's RAM. The save file (.sl2) has a **different layout** than runtime memory. You cannot directly use CT offsets as save-file offsets.

---

## Key differences

### 1. Runtime pointers vs sequential save

In memory the data is scattered and accessed via pointer chains:
```
GameDataMan → +0x08 → PlayerGameData (RAM structure)
GameDataMan → +0x08 → +0x408 → EquipInventoryData
GameDataMan → +0x08 → +0x518 → EquipMagicData
```

In the save file the data is **sequential** — one section after another with no pointers.

### 2. Runtime header offset

PlayerGameData in memory has an extra runtime header (vtable pointer, PlayerNo):
- **Memory**: HP at offset +0x10 from base
- **Save file**: HP at offset ~0x08 from the start of the PlayerGameData section

The difference is NOT constant:
| Field | Memory (CT) | Save file | Diff |
|---|---|---|---|
| HP | +0x10 | +0x08 | 0x08 |
| Vigor | +0x3C | +0x34 | 0x08 |
| Level | +0x68 | +0x60 | 0x08 |
| Name | +0x9C | +0x94 | 0x08 |
| Gender | +0xBE | +0xB6 | 0x08 |

For PlayerGameData: **save_offset ≈ memory_offset - 0x08** (after subtracting the 8-byte runtime header).

### 3. Structures available only in memory

Some CT data **does not exist in the save file** — it is computed on load:
- Current resistances (Immunity/Robustness/Focus/Vitality current values)
- Character flags (NoDead, NoDamage, etc.)
- Team type (host/phantom/invader)
- Poise/Toughness (live calculation)
- Animation state
- AI state (NPC)

### 4. Structures available only in the save

Some data exists in the save but has no direct runtime counterpart:
- GaItem Map (managed by the game engine)
- Event flags raw bitfield (accessed via EventFlagMan API)
- World state blobs (FieldArea, WorldGeomMan, RendMan)
- PlayerGameData Hash

---

## Safe mapping (confirmed)

These fields have **confirmed** counterparts in both domains:

| Data | CT (memory) | Save file |
|---|---|---|
| Attributes (8×u32) | GameDataMan+0x08+0x3C..0x58 | PlayerGameData+0x34..0x50 |
| Level | GameDataMan+0x08+0x68 | PlayerGameData+0x60 |
| Runes | GameDataMan+0x08+0x6C | PlayerGameData+0x64 |
| Name | GameDataMan+0x08+0x9C | PlayerGameData+0x94 |
| Gender | GameDataMan+0x08+0xBE | PlayerGameData+0xB6 |
| Class | GameDataMan+0x08+0xBF | PlayerGameData+0xB7 |
| Death Count | GameDataMan+0x94 | Game State section [0x00] |
| NG+ | GameDataMan+0x120 | Game State section 1 [0x00] |
| Event Flags | EventFlagMan+0x28+offset | EventFlags section [offset] |

---

## Rules for using CT data

1. **Field names and types**: Always reliable (e.g., "Vigor" = u32, "Gender" = u8)
2. **Field order**: Reliable (fields within the same structure keep their order)
3. **Enum values**: Reliable (e.g., Class: 0=Vagabond, ArmStyle: 0=Empty/1=OneHand)
4. **Absolute offsets**: NEVER use directly — recompute or verify with a hex dump
5. **Pointer chains**: Tell you about logical structure, not the save-file layout
6. **"Runtime only" data**: Don't look for it in the save (HP regen rate, AI flags, poise)

---

## Offset verification — method

To confirm an offset in the save file:
1. Load a known save into a hex editor
2. Find a known value (e.g., character name in UTF-16LE)
3. Compute the offset from the start of the slot
4. Compare with the expected offset from the parser (er-save-manager)

---

## Sources

- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — runtime pointer chains
- Cheat Engine: `ER_TGA_v1.9.0` — runtime offsets
- er-save-manager: `parser/` — save-file sequential parsing (ground truth)
