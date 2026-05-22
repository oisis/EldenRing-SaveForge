# 06 — Equipment

> **Type**: Design doc + canonical reference
> **Status**: ✅ Active — canonical reference for the current **read-only** equipment model in SaveForge.
> **Scope**: What the editor parses from the equipment section in a save (`EquippedItemsItemIds` 88B + `EquippedGreatRune`), how this section is used (`IsHandleEquipped` transfer guard, hash 7/8 dependency) and what the editor **does not** implement (write API for equipment slots).

---

## Chapter goal

This chapter documents how SaveForge treats the equipment section in a save file: which data is parsed, what it is used for, and explicitly separates this model from the hypothetical structures from `er-save-manager` / Cheat Engine that the current code **does not** parse.

Key questions the chapter answers:

- What exactly does SaveForge read from the equipment section in a save?
- In what form does ChrAsmEquipment store item references (encoded item-ID form, not raw itemID nor handle)?
- How does `IsHandleEquipped` protect against transfers of equipped items?
- What does SaveForge **not** implement (no public API for changing equipment)?
- Which structures from the previous version of the chapter are external hypotheses (er-save-manager) and remain unverified in our code?

Related mechanisms — in cross-references at the end of the chapter: [03](03-gaitem-map.md) (GaItem/GaMap), [07](07-inventory.md) (Inventory), [53](53-inventory-storage-transfer.md) (transfer + equipped guard), [54](54-ash-of-war.md) (AoW patching).

---

## Status

| Component | Status |
|---|---|
| `EquippedItemsItemIds` section (88B = 22 × u32) | ✅ Parsed in `parseFromData` via `EquipItemsIDOffset` |
| Encoded item-ID form (3 conventions) | ✅ Active in `IsHandleEquipped` |
| `IsHandleEquipped` (transfer guard) | ✅ Active, test coverage `TestMoveEquippedInvToStorageSkipped` |
| Hash 7 (weapons) and hash 8 (armor+talismans) — input from ChrAsmEquipment | ✅ Computed in `ComputeSlotHash` |
| `EquippedGreatRune` (1 × u32 inside the section, offset 0x28) | ✅ Read+Write in `parseFromData` / `WriteSave` |
| App-level write API for equipment slots | ❌ None — equipment is read-only from the UI perspective |
| Hash recompute (`RecalculateSlotHash`) at runtime | ⚠️ Called **only** in tests `hash_test.go`; main code path does not update hashes |
| Slots 10/11/16 marked in code as `unk0x28/0x2C/0x40` | `needs verification` — names `Arrows 3 / Bolts 3 / Hair` from the previous document came from `er-save-manager` / Cheat Engine, not verified by our parser |
| Hypothetical structures `EquippedItemsEquipIndex`, `ActiveWeaponSlotsAndArmStyle`, `EquippedItemsGaitemHandles` | ❌ Not parsed in our code — hypotheses from `er-save-manager` |

---

## Source of truth in code

| Topic | File / function |
|---|---|
| `EquipItemsIDOffset` (absolute section offset) | `backend/core/structures.go:244` + offset chain in `parseFromData` (`structures.go:351-354`) |
| `ChrAsmFieldCount` (22) and `ChrAsmEquipmentSize` (88) | `backend/core/hash.go:24-27` |
| 22-slot layout (docstring comment) | `backend/core/hash.go:29-42` |
| `weaponSlotIndices` (hash 7 input) | `backend/core/hash.go:45` — `[0..9]` |
| `armorSlotIndices` (hash 8 input) | `backend/core/hash.go:49` — `[12, 13, 14, 15, 17, 18, 19, 20, 21]` |
| `readEquipSection` | `backend/core/hash.go:124` — reads 22 × u32 |
| `extractSlots` | `backend/core/hash.go:137` — picks values from indices |
| `equipmentHash` | `backend/core/hash.go:107` — `bytesHash` over u32 slice |
| `ComputeSlotHash` (hash 7/8 entry) | `backend/core/hash.go:197-258` |
| `RecalculateSlotHash` | `backend/core/hash.go:290` — called **only** in tests (`hash_test.go`) |
| `IsHandleEquipped` (encoded item-ID matching) | `backend/core/transfer.go:592-633` |
| `EquippedGreatRune` field | `backend/core/structures.go:216` (type), `:321-324` (read), `:855-857` (write in `WriteSave`) |
| `DynEquipGreatRune = 0x28` (offset inside the section) | `backend/core/offset_defs.go:100` |
| `DynEquip*` offsets for offset chain | `backend/core/offset_defs.go:86-100` |
| Transfer equipped guard usage | `backend/core/transfer.go:194-196` (`SkipReasonEquipped` for `TransferToStorage`) |
| Test covering equipped guard | `tests/transfer_test.go::TestMoveEquippedInvToStorageSkipped` |

---

## Mental model

```
                       ┌──────────────────────────────────────────┐
                       │  Save (binary)                           │
                       │  EquipItemsIDOffset ──▶  ChrAsmEquipment │
                       │                          88 bytes        │
                       │                          22 × u32        │
                       │                          (encoded form)  │
                       └──────────────────────┬───────────────────┘
                                              │
                                  parseFromData reads
                                              │
                                              ▼
                       ┌──────────────────────────────────────────┐
                       │  slot.EquipItemsIDOffset (int)           │
                       │  slot.Player.EquippedGreatRune (u32)     │
                       │  (Great Rune lives inside the section    │
                       │   at offset +0x28 = slot 10)             │
                       └────────────┬──────────────┬──────────────┘
                                    │              │
                  IsHandleEquipped  │              │  readEquipSection
                  (transfer guard)  │              │  (hash 7 / 8 input)
                                    ▼              ▼
                          ┌─────────────────────────────────────┐
                          │  Transfer (legacy core) ─▶ skip     │
                          │    handle when IsHandleEquipped     │
                          │    (see spec/53)                    │
                          │                                     │
                          │  ComputeSlotHash:                   │
                          │    hash[7] = equipmentHash(slots    │
                          │              0..9)                  │
                          │    hash[8] = equipmentHash(slots    │
                          │              12-15, 17-21)          │
                          └─────────────────────────────────────┘
                                              ▲
                                              │
                                  (RecalculateSlotHash
                                   called **only**
                                   in tests — main code
                                   does not update hashes)
```

The editor **has no** code that writes ChrAsmEquipment slots 0..9, 12-15, 17-21. The only write within this section is `EquippedGreatRune` in slot 10 (offset 0x28), implemented in `WriteSave` as part of `Player` state.

---

## What SaveForge currently parses

In this chapter "parses" means: the code extracts a value from `slot.Data` to a typed field in `SaveSlot` / `Player` and optionally uses it in business logic.

### `EquippedItemsItemIds` section

- Absolute offset in `slot.Data`: `slot.EquipItemsIDOffset` (`structures.go:244`).
- Computed in `parseFromData` (`structures.go:351-354`) as:
  ```go
  spEffect           := mo + DynSpEffect
  equipedItemIndex   := spEffect + DynEquipedItemIndex
  activeEquipedItems := equipedItemIndex + DynActiveEquipedItems
  equipedItemsID     := activeEquipedItems + DynEquipedItemsID
  s.EquipItemsIDOffset = equipedItemsID
  ```
  where `DynEquipedItemsID = 0x58` (`offset_defs.go:88`).
- The section has size `ChrAsmEquipmentSize = 88` bytes (`hash.go:27`), contains `ChrAsmFieldCount = 22` u32 values (`hash.go:24`).
- **There is no** typed `ChrAsmEquipment` struct in our code — the section is read ad-hoc by `readEquipSection(data, off)` (`hash.go:124-134`), which returns a `[]uint32` of length 22.

### `EquippedGreatRune` (1 × u32 inside the section)

- Field: `slot.Player.EquippedGreatRune uint32` (`structures.go:216`).
- Offset: `slot.EquipItemsIDOffset + DynEquipGreatRune`, where `DynEquipGreatRune = 0x28` (`offset_defs.go:100`).
- Offset 0x28 = byte 40 = **slot 10** in the 22-slot ChrAsmEquipment table. So `EquippedGreatRune` *physically lives inside* the 88B section, just with a separate typed accessor.
- Read: `parseFromData` (`structures.go:321-324`).
- Write: `WriteSave` → `SyncPlayerToData` (`structures.go:855-857`).
- Value = u32 item ID (0 = none).

### What is NOT typed

The remaining 21 values from the 22-slot table (slots 0-9, 11-21) **have no** dedicated fields in `Player` or `SaveSlot`. They are accessible ad-hoc via `readEquipSection(slot.Data, slot.EquipItemsIDOffset)`.

---

## EquippedItemsItemIds section

This section is the only parsed equipment section in SaveForge. Layout of 22 u32 (`hash.go:29-42`):

| # | Offset | Name per code | Comment |
|---|---|---|---|
| 0 | 0x00 | `LeftHandArmament1` | hash 7 (weapon) |
| 1 | 0x04 | `RightHandArmament1` | hash 7 |
| 2 | 0x08 | `LeftHandArmament2` | hash 7 |
| 3 | 0x0C | `RightHandArmament2` | hash 7 |
| 4 | 0x10 | `LeftHandArmament3` | hash 7 |
| 5 | 0x14 | `RightHandArmament3` | hash 7 |
| 6 | 0x18 | `Arrows1` | hash 7 |
| 7 | 0x1C | `Bolts1` | hash 7 |
| 8 | 0x20 | `Arrows2` | hash 7 |
| 9 | 0x24 | `Bolts2` | hash 7 |
| 10 | 0x28 | `unk0x28` (`EquippedGreatRune` at this offset) | outside hash 7 and 8 |
| 11 | 0x2C | `unk0x2C` | outside hash 7 and 8 |
| 12 | 0x30 | `Head` | hash 8 |
| 13 | 0x34 | `Chest` | hash 8 |
| 14 | 0x38 | `Arms` | hash 8 |
| 15 | 0x3C | `Legs` | hash 8 |
| 16 | 0x40 | `unk0x40` | outside hash 7 and 8 |
| 17 | 0x44 | `Talisman1` | hash 8 |
| 18 | 0x48 | `Talisman2` | hash 8 |
| 19 | 0x4C | `Talisman3` | hash 8 |
| 20 | 0x50 | `Talisman4` | hash 8 |
| 21 | 0x54 | `Talisman5` | hash 8 |

Slots 10/11/16 are in the comment as `unk` — the code assigns no semantic names to them. The previous version of the document described them as `Arrows 3 / Bolts 3 / Hair`; that attribution came from `er-save-manager` / Cheat Engine and remains `needs verification` in our parser.

Slot 21 (`Talisman5`) — the previous version of the document described it as `Accessory 5 (reserved, unused)`. The code treats it as Talisman5 (hash 8 includes it as one of the 9 armor+talisman hash inputs).

Special value `0xFFFFFFFF` in a slot = empty slot.

---

## Encoded item-ID forms

The values in the 22-slot table are **not** directly item IDs nor handles. From the docstring of `IsHandleEquipped` (`transfer.go:600-613`):

> ChrAsmEquipment uses an encoded item-ID form: for weapons/armor/AoW the value is `itemID | 0x80000000` (the item ID resolved via GaMap with a 0x80 high-bit flag). For talismans the value is the bare lower 28 bits of the handle. Match against every plausible representation to keep the check robust across upgrade levels and infusions.

Three encoded-form conventions:

| Item type | Handle prefix | Encoded form in equipment slot |
|---|---|---|
| Weapon / Armor / AoW | `0x80` / `0x90` / `0xC0` | `itemID \| 0x80000000` (item ID from `GaMap` + 0x80 flag) |
| Talisman (Accessory) | `0xA0` | `handle & 0x0FFFFFFF` (bare lower 28 bits = itemID) |
| Goods | `0xB0` (usually not in equipment) | prefix-swap `0xB0 → 0x40`: `lower \| 0x40000000` |

### `IsHandleEquipped` candidate matching

`IsHandleEquipped(slot, handle)` (`transfer.go:592`) builds a **candidate set** for the given `handle`, and then scans all 22 slots of the section looking for a match. The set contains (from `transfer.go:605-621`):

1. `handle` directly (defensive — some saves may hold the handle directly).
2. `handle & 0x0FFFFFFF` — bare lower 28 bits (talisman form).
3. `(handle & 0x0FFFFFFF) | 0x80000000` — bare lower + 0x80 flag.
4. `GaMap[handle]` — true item ID (returned by `GaMap` lookup) — for weapon/armor/AoW.
5. `GaMap[handle] | 0x80000000` — `GaMap` value + 0x80 flag.
6. Prefix-swap depending on type:
   - `0xA0` (talisman) → adds `lower | 0x20000000` (talisman item ID prefix).
   - `0xB0` (goods) → adds `lower | 0x40000000` (goods item ID prefix).

A match against any slot (other than `0` and `0xFFFFFFFF`) → `true`. Multi-form matching is defensive: different versions of the game may write equipment in different canons, so `IsHandleEquipped` accepts all known representations.

---

## Slot names and unknown slots

Slots 10/11/16 are in the code comment as `unk0x28 / unk0x2C / unk0x40`. Our parser assigns no semantics to them and does not use them in any business-logic function.

**Hypotheses from previous versions of the document** (kept as `needs verification`):

- Slot 10 / `unk0x28` — in the previous 06 as "Arrows 3" with a note "CT-confirmed". In our code this offset contains `EquippedGreatRune`, so Arrows 3 is inconsistent with the current use. **`needs verification`** — whether the game uses this slot as a third arrow set or exclusively as Great Rune.
- Slot 11 / `unk0x2C` — in the previous 06 as "Bolts 3". Our code does not read it. **`needs verification`**.
- Slot 16 / `unk0x40` — in the previous 06 as "Hair" (associated with Face Data Hair_Model_Id). Our code does not read it. **`needs verification`** — in particular the hypothetical link to Face Data is not confirmed by any save parser in SaveForge.

Slots `Talisman3` (19), `Talisman4` (20), `Talisman5` (21) — the code treats all as full-fledged talisman slots and includes them in `armorSlotIndices` (hash 8). The hypothetical note "Talisman 3&4 require quest unlock; Accessory 5 unused" from the previous document — `needs verification` as to enforcement by the game.

---

## EquippedGreatRune

`Player.EquippedGreatRune` (`structures.go:216`) is:
- Read in `parseFromData` (`structures.go:321-324`) from offset `EquipItemsIDOffset + 0x28` = slot 10 in the ChrAsmEquipment section.
- Written in `WriteSave` → `SyncPlayerToData` (`structures.go:855-857`) to the same offset.

The editor **does not** expose a public API for changing `Player.EquippedGreatRune` from the UI (no `App.SetGreatRune` / similar functions exported to JS). The field is *preserved* during save/load round-trip but is not *editable* from the interface.

**Per the current hash 7/8 functions**: slot 10 (offset 0x28), where Great Rune lives, is in neither `weaponSlotIndices` (`[0..9]`) nor `armorSlotIndices` (`[12-15, 17-21]`). So the **inputs** to `equipmentHash` for hash 7 and hash 8 do not contain the value from this slot. This implies that **in our hash implementation** changing the value at offset `EquipItemsIDOffset+0x28` would not change the `ComputeSlotHash` result for entries 7 and 8. **`needs verification`** whether the game uses the hashes identically (there may be other slot-level hashes besides these 12 that our code does not compute).

### Great Rune item IDs (reference)

| Hex ID | Decimal | Great Rune |
|---|---|---|
| `0x00000000` | 0 | None |
| `0xB00000BF` | 2952790207 | Godrick's |
| `0xB00000C0` | 2952790208 | Radahn's |
| `0xB00000C1` | 2952790209 | Morgott's |
| `0xB00000C2` | 2952790210 | Rykard's |
| `0xB00000C3` | 2952790211 | Mohg's |
| `0xB00000C4` | 2952790212 | Malenia's |

Source: Cheat Engine community tables + Fextralife. **`needs verification`** as to DLC Great Runes (e.g. Bayle's, Promised Consort Radahn's) — our code treats the field as an opaque u32 and has no constants list.

Relation to the buff: `Player.GreatRuneOn` (`structures.go:215`, `OffGreatRuneOn = -184` = PGD 0xF7) — 1 = active, 0 = off. This is a separate field outside the equipment section.

---

## Read path and hash dependency

### Hash 7 (Equipped Weapons)

From `ComputeSlotHash` (`hash.go:248-254`):

```go
equipSection := readEquipSection(slot.Data, equipItemsIDOff)
weaponIDs := extractSlots(equipSection, weaponSlotIndices)  // slots [0..9]
writeEntry(7, equipmentHash(weaponIDs))
```

Input: 10 u32 values from slots 0-9 (L1, R1, L2, R2, L3, R3, Arrows1, Bolts1, Arrows2, Bolts2).
Hash: `bytesHash(weaponIDs)` (`hash.go:75-85`) — modified Adler-like checksum.

### Hash 8 (Equipped Armors + Talismans)

From `ComputeSlotHash` (`hash.go:256-258`):

```go
armorIDs := extractSlots(equipSection, armorSlotIndices)  // slots [12,13,14,15,17,18,19,20,21]
writeEntry(8, equipmentHash(armorIDs))
```

Input: 9 u32 values from slots 12-15 (Head, Chest, Arms, Legs) and 17-21 (Talisman1..Talisman5).

### Hash recompute discipline

`RecalculateSlotHash` (`hash.go:290`) writes the result of `ComputeSlotHash` to `slot.Data[HashOffset:HashOffset+HashSize]`. **In our main code path this function is not called** — all references in the repo are in `backend/core/hash_test.go`. This means:

- The editor does not update hashes on `WriteSave`.
- The game probably validates hashes on load — `needs verification` how the game behaves when we save with an unchanged equipment state (the hash should remain valid from the previous save / runtime).
- **`needs verification`** holistically: whether the editor should call `RecalculateSlotHash` after edits that affect the hash (e.g. stat, level, soul changes — hashes 0, 1, 5).

---

## Current runtime usage

The `EquippedItemsItemIds` section is used in SaveForge in three places:

1. **`IsHandleEquipped`** (`transfer.go:592`) — guard used by the legacy core transfer path (`core.MoveItemsBetweenContainers`, [53](53-inventory-storage-transfer.md)). Scans all 22 slots with multi-form candidate matching.
2. **`ComputeSlotHash`** entries 7 and 8 — `readEquipSection` + `extractSlots` + `equipmentHash`.
3. **`EquippedGreatRune`** read/write in `parseFromData` / `WriteSave` — inside the section at offset 0x28.

No other runtime usages in our code. UI components (frontend) do not call any App-level bindings for equipment slots.

---

## Equipped guard in transfer

`core.MoveItemsBetweenContainers` from [53](53-inventory-storage-transfer.md):
- For each handle passed to the transfer:
  - If `direction == TransferToStorage` and `IsHandleEquipped(slot, handle)` → skip with `SkipReasonEquipped`.
  - Storage → Inventory **does not** check equipped (assumption: items in Storage are not equipped).
- Effect: the user cannot move an equipped item from Inventory to Storage via the legacy core path. They must unequip it in the game first.

Test coverage: `tests/transfer_test.go::TestMoveEquippedInvToStorageSkipped` + 4 additional locations using `IsHandleEquipped` to skip non-equipped weapons in fixtures.

### Workspace path gap

The workspace save path (`editor.ApplyWorkspaceSave` + `writeContainerLayout`, [53](53-inventory-storage-transfer.md)) **does not** have an explicit `IsHandleEquipped` check. Cross-reference to [53](53-inventory-storage-transfer.md) (section "Equipped guard / Workspace path") where this gap is explicitly documented as `needs verification`. In the current SortOrderTab UI the user may (potentially) move an equipped item cross-grid without a block — in-game implications `needs verification`.

---

## What SaveForge does not implement

The editor is **read-only** for the ChrAsmEquipment section from the public API perspective:

- ❌ No `App.SetEquipment*` — cannot change the item in a specific equipment slot.
- ❌ No `App.SwapWeapon*` / `App.SwapArmor*` — no swap between slots.
- ❌ No `App.UnequipItem*` — cannot unequip an item.
- ❌ No `App.SetGreatRune` — `Player.EquippedGreatRune` is read+write in `parseFromData`/`WriteSave`, but no public setter from the UI.
- ❌ No validation of an equipment reference in the workspace save path (an equipped item may be removed from inventory without clearing the equipment slot — `needs verification` as to a dangling reference in the game).

The `EquippedGreatRune` field round-trips (load → save preserves the value unchanged), but **cannot** be modified from the UI in the current version.

---

## Historical / external hypotheses

The previous version of `06-equipment.md` described **four** equipment structures (4 × 88B = 352B + 28B `ActiveWeaponSlotsAndArmStyle`) based on external sources (`er-save-manager/parser/equipment.py`, ER-Save-Editor Rust, Cheat Engine tables). SaveForge **does not parse** these hypothetical structures:

| Hypothetical structure | Status in SaveForge |
|---|---|
| `EquippedItemsEquipIndex` (88B) — acquisition_index into inventory | ❌ Not parsed. `needs verification` in `slot.Data` — may or may not exist physically in the save. |
| `ActiveWeaponSlotsAndArmStyle` (28B) — `ArmStyle`, `LeftWeaponSlot`, `RightWeaponSlot`, `LeftArrowSlot`, …, `LeftBoltSlot`, `RightBoltSlot` | ❌ Not parsed. `needs verification`. |
| `EquippedItemsItemIds` (88B) — item IDs | ✅ Parsed in SaveForge as `EquipItemsIDOffset` (the only one of the four). |
| `EquippedItemsGaitemHandles` (88B) — GaItem handles per slot | ❌ Not parsed. `needs verification`. |

Hypothetical "Implications for editing" from the previous version of 06:
- "Changing a weapon requires updating ALL 3 structures" — outdated. SaveForge does not implement equipment changes.
- "Active slot values: 0/1/2", "ArmStyle affects animations, invalid value crashes" — `needs verification` (our code does not validate these values).
- "Talisman 3&4 require quest unlock", "Accessory 5 unused" — `needs verification` (the code treats all 5 talismans equally).
- "Item ID encoding: Weapon +X = baseID + X" — semantics used in Add Items (see [43](43-transactional-item-adding.md)), but in the context of equipment slots `needs verification`.
- "Slot Hair linked to Face Data Hair_Model_Id" — `needs verification`, our parser does not have this relationship.

---

## Relationship to GaItems and Inventory

- The encoded item-ID form in ChrAsmEquipment **does not contain** a handle — it contains an item ID (from `GaMap`) or bare lower bits. Direct inference handle → equipment slot requires scanning all candidates (see section "Encoded item-ID forms").
- `IsHandleEquipped(slot, handle)` uses `slot.GaMap` lookup to build the candidate set — requires a valid GaMap (see [03](03-gaitem-map.md)).
- An equipment slot does not point to a specific record in `slot.Inventory.CommonItems` or in `slot.Storage.CommonItems`. The equipment → inventory entry link exists **exclusively** via `GaMap`: equipment holds an item ID, GaMap maps handle → item ID, inventory record has a handle. Inferring the back-reference (item ID → handle → inventory record) requires a scan.

---

## Relationship to Ash of War

AoW patching (see [54](54-ash-of-war.md)) modifies `slot.GaItems[*].AoWGaItemHandle` in the GaItem array, **not** the ChrAsmEquipment section. Equipping a new AoW on a weapon does not change the equipment slot and does not require its update. Hash 7 is not sensitive to AoW handle changes inside GaItem — it only requires consistency of item IDs in the equipment slots.

---

## Validation and safety notes

- **No validation of equipment reference**: SaveForge does not check whether the itemID in a ChrAsmEquipment slot actually exists in `slot.GaItems` / `slot.GaMap`. Hypothetically a dangling reference (slot pointing to a removed item) is possible, but in the current code there is no operation that would introduce one (no delete-equipped path).
- **In-game fallback (empirical)** — `needs verification`:
  - Equipped weapon removed from inventory → the game falls back to "Unarmed" (`invUnarmedBaseID = 0x0001ADB0`, [36](36-inventory-categories-game-order.md) "Unarmed exclusion").
  - Equipped armor removed → default armor.
  - Equipped talisman removed → empty slot.
- **Hash recompute**: in the current code `RecalculateSlotHash` is called **only in tests**. The editor does not update the hash on `WriteSave`. **`needs verification`** holistically: whether the game validates the hash on load, and whether there are scenarios in which the editor should call recompute (stat/level/soul changes which affect hashes 0/1/5).

---

## Test coverage

| Class | Test files |
|---|---|
| Equipped guard transfer | `tests/transfer_test.go::TestMoveEquippedInvToStorageSkipped` |
| `IsHandleEquipped` for non-equipped fixtures | `tests/transfer_test.go` (4 locations: `:226`, `:259`, `:373`, `:618` — skip non-equipped weapons in fixtures) |
| Hash determinism + recompute | `backend/core/hash_test.go::TestComputeSlotHash_Deterministic`, `TestRecalculateSlotHash_WritesToData`, `TestComputeSlotHash_ChangingStatsChangesHash` |

**Missing**:
- Tests of `IsHandleEquipped` for each of the 6 candidate forms (lower 28 bits, prefix-swap for 0xA0/0xB0).
- Tests for equipped armor, equipped talisman, equipped AoW as distinct prefixes — current tests cover mainly weapons.
- Tests for workspace path equipped behavior (whether `Validate(snap)` or `writeContainerLayout` blocks transfer of equipped items).
- A round-trip test of `EquippedGreatRune` (load → write → load, whether the value is preserved).

---

## Known limits / needs verification

- **Slots 10/11/16** (`unk0x28 / unk0x2C / unk0x40`) — the code assigns no semantics. Hypotheses from `er-save-manager`/CE: Arrows3/Bolts3/Hair. `needs verification`.
- **Slot 10 vs Great Rune offset** — slot 10 physically contains `EquippedGreatRune` (offset 0x28 = byte 40 = slot 10). The simultaneous "Arrows 3" hypothesis is inconsistent with this use. `needs verification` as to the actual game semantics.
- **`Talisman3` / `Talisman4` / `Talisman5`** — hypothetically require a quest unlock (Talisman Pouch items). The code treats them all as full slots. `needs verification` as to enforcement by the game when attempting to equip without an unlock.
- **Hypothetical 3 additional equipment structures** (`EquipIndex`, `ActiveWeaponSlotsAndArmStyle`, `GaitemHandles`) — not parsed by SaveForge. `needs verification` as to their actual presence in the binary save.
- **Workspace path equipped guard** — no explicit check. Cross-reference to [53](53-inventory-storage-transfer.md).
- **In-game behavior of a dangling equipment reference** — not empirically verified.
- **DLC Great Rune item IDs** — the list of 7 vanilla IDs does not cover potential DLC additions. `needs verification` on the DB side.
- **Hash recompute discipline** — `RecalculateSlotHash` is called only in tests. `needs verification` holistically: whether the editor should update the hash in `WriteSave` after edits that affect hash 0/1/5 (stats, level, souls).
- **Encoded form and the editor** — if `App.SetEquipment*` is ever added, it would have to:
  1. Write `itemID | 0x80000000` (weapons/armor/AoW) or bare lower (talismans), not raw handle.
  2. Update hash 7/8 if the change touches slots 0-9 / 12-15 / 17-21.
  3. Validate that the item ID exists in `slot.GaMap` / `slot.GaItems`.
  None of these is currently implemented.

---

## Cross-references

- [03 — GaItem map](03-gaitem-map.md) — handle ↔ itemID mapping via `GaMap`, prefix-swap functions (`HandleToItemID`, `ItemIDToHandlePrefix`).
- [07 — Inventory model](07-inventory.md) — read-side 12B record, CommonItems offsets, why equipment does not point directly at an inventory record.
- [10 — Storage model](10-storage.md) — analogous read-side for Storage.
- [35 — GaItem allocator invariants](35-gaitem-allocator-invariants.md) — handle allocation (independent of equipment slots).
- [36 — Inventory Categories and Game Order](36-inventory-categories-game-order.md) — handle prefix classification (`GetItemCategoryFromHandle`), Unarmed placeholder, DLC flag mechanism.
- [43 — Transactional item adding](43-transactional-item-adding.md) — Add Items does not modify equipment slots.
- [53 — Inventory ↔ Storage transfer](53-inventory-storage-transfer.md) — full description of the equipped guard in the legacy core path + workspace path gap.
- [54 — Ash of War](54-ash-of-war.md) — AoW patching modifies GaItem, not equipment.

---

## Sources

### Local code

- `backend/core/structures.go` — `EquipItemsIDOffset`, `Player.EquippedGreatRune`, `parseFromData`, `WriteSave`.
- `backend/core/hash.go` — `ChrAsmFieldCount`, `ChrAsmEquipmentSize`, layout comment, `weaponSlotIndices`, `armorSlotIndices`, `readEquipSection`, `extractSlots`, `equipmentHash`, `ComputeSlotHash`, `RecalculateSlotHash`.
- `backend/core/transfer.go` — `IsHandleEquipped`, candidate matching, equipped guard usage.
- `backend/core/offset_defs.go` — `DynEquipGreatRune = 0x28`, `DynEquipedItemsID = 0x58`, offset chain constants.
- `tests/transfer_test.go` — `TestMoveEquippedInvToStorageSkipped`, uses of `IsHandleEquipped` in fixture skips.
- `backend/core/hash_test.go` — `TestComputeSlotHash_*`, `TestRecalculateSlotHash_WritesToData`.

### External hypotheses (historical context, not used in SaveForge)

- `er-save-manager/parser/equipment.py` — classes `EquipmentSlots`, `ActiveWeaponSlotsAndArmStyle` (lines 20-163).
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — referenced structures.
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — ChrAsm offsets (ArmStyle, weapon slots, equipment IDs).
- Cheat Engine: `ER_TGA_v1.9.0` — ChrAsm 2 (equipment item IDs 0x5D8-0x62C), Quick Items, Pouch, Great Rune.

These sources remain as context for research/experiments, but are **not** source-of-truth for the current SaveForge model.
