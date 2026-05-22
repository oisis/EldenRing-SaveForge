# 03 — GaItem Map: binary model, handles and item references

> **Type**: Binary format spec
> **Status**: ✅ canonical, implemented, source-of-truth hex-verified against the current backend. Allocator details, capacity invariants and transactional safety are described in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
> **Scope**: Read-side binary model of the `slot.GaItems` array, the `slot.GaMap` map, per-item-type record formats, handle prefixes and the handle ↔ ItemID relationship. The chapter covers what the parser does when loading a save and how runtime structures mirror the raw bytes. Write-side allocation (cursors, capacity, validation, rollback) is out of scope here.

---

## Chapter goal

The chapter describes the **read-side binary model** of GaItem:

- what the `GaItems` array is and how the parser reads it;
- how `slot.GaMap` works (the `handle → ItemID` mapping);
- how entry types (weapon / armor / talisman / goods / AoW) are distinguished by the handle prefix;
- how inventory and storage reference GaItems via handle, not via ItemID;
- which relationships must hold between these layers so that the game accepts the save.

What the chapter does **NOT** do:

- It does not describe the allocator (`allocateGaItem`), the counters (`NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`), the 5118/5120 capacity, the AoW guard, snapshot/rollback nor post-mutation validation — all of that is in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
- It does not go into the full Ash of War semantics (weapon `AoWGaItemHandle` sentinels, dual-destination, strict apply) — that is in [54-ash-of-war](54-ash-of-war.md).
- It does not describe the inventory/storage section format nor the `NextEquipIndex` visibility gate — that is in [07-inventory](07-inventory.md), [10-storage](10-storage.md), [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

---

## Source of truth in code

| Topic | Files / functions | Note |
|---|---|---|
| Entry struct | `backend/core/structures.go:66-73` (`GaItemFull`) | fields: `Handle`, `ItemID`, `Unk2/Unk3`, `AoWGaItemHandle`, `Unk5` |
| Record size | `backend/core/structures.go:75-89` (`GaItemRecordSize`) | record size depends on `ItemID` (not on handle), matched to Rust ER-Save-Editor |
| Size constants | `backend/core/offset_defs.go:46-49` | `GaRecordWeapon = 21`, `GaRecordArmor = 16`, `GaRecordItem = 8` |
| Handle constants | `backend/core/offset_defs.go:55-57` | `GaHandleEmpty = 0x00000000`, `GaHandleInvalid = 0xFFFFFFFF`, `GaHandleTypeMask = 0xF0000000` |
| Type prefixes | `backend/core/structures.go:20-24` | `ItemTypeWeapon = 0x80000000`, `ItemTypeArmor = 0x90000000`, `ItemTypeAccessory = 0xA0000000`, `ItemTypeItem = 0xB0000000`, `ItemTypeAow = 0xC0000000` |
| Section start | `backend/core/offset_defs.go:40` | `GaItemsStart = 0x20` |
| `GaItems` array | `backend/core/structures.go:226` (`SaveSlot.GaItems []GaItemFull`) | length = `len(slot.GaItems)`, fixed once at load |
| `GaMap` map | `backend/core/structures.go::SaveSlot.GaMap map[uint32]uint32` | builds in `scanGaItems`; non-empty entries only; key = handle, value = ItemID |
| Parser | `backend/core/structures.go:612-728` (`scanGaItems`) | two-pass scan: first parses entries, second reconstructs the counters (counters documented in 35) |
| Container reference | `backend/core/structures.go:118-122` (`InventoryItem`) | fields: `GaItemHandle`, `Quantity`, `Index` |

---

## Mental model

The GaItem model consists of three layers:

```
┌─────────────────────────────────────────────────────────┐
│  Container layer                                         │
│  Inventory.CommonItems, Inventory.KeyItems, Storage.*    │
│  → 12-byte records { handle, quantity, index }           │
└─────────────────────────────────────────────────────────┘
                      │ handle (u32)
                      ▼
┌─────────────────────────────────────────────────────────┐
│  Reference layer                                         │
│  slot.GaMap : map[uint32]uint32                          │
│  → quick lookup handle → ItemID                          │
│  → filled in scanGaItems for non-empty entries           │
└─────────────────────────────────────────────────────────┘
                      │ handle (u32)
                      ▼
┌─────────────────────────────────────────────────────────┐
│  Object layer                                            │
│  slot.GaItems : []GaItemFull                             │
│  → physical array (5118 or 5120 entries, depending on    │
│    slot.Version — details in 35)                         │
│  → an entry holds handle, ItemID, and variant-specific   │
│    fields (Unk2, Unk3, AoWGaItemHandle, Unk5)            │
└─────────────────────────────────────────────────────────┘
```

The three layers are independent in size:

- **`GaItems`** — array of objects; `len(slot.GaItems) ∈ {5118, 5120}` (per `slot.Version`).
- **`GaMap`** — reference map; size ≤ count of non-empty entries in `GaItems`.
- **Container layer** — `Inventory.CommonItems` has 2688 slots, `Inventory.KeyItems` 384, `Storage.CommonItems` 1920, `Storage.KeyItems` 128. An empty slot = `handle == 0` or `handle == 0xFFFFFFFF`.

These sizes are independent: you may have 60 non-empty entries in `GaItems` while all 2688 inventory slots remain available for references — as long as every used handle exists in `GaMap`.

> ℹ️ The number of **GaItem instances** is not the same as the **number of inventory/storage entries**. An inventory entry carries only a handle (a reference); the physical object lives in `GaItems`. Goods (`0xB0`) are **stackable**: a single `GaItems` entry may represent an N-stack whose quantity is stored in the container record's `Quantity` (`InventoryItem.Quantity`), not in `GaItemFull`.

---

## Binary structures

### `GaItems` array

- Location: from `GaItemsStart = 0x20` to `slot.MagicOffset - DynPlayerData + 1` (`structures.go:618-620`).
- Entry count: 5118 for `slot.Version ∈ [1, 81]`, 5120 for `slot.Version > 81` (the decision in `scanGaItems`, [35](35-gaitem-allocator-invariants.md#gaitem-capacity-by-slot-version)).
- Every entry is a `GaItemFull`; the size of the serialized record depends on **`ItemID`**, not on the handle (`structures.go:75-89`):

```go
func GaItemRecordSize(itemID uint32) int {
    if itemID == 0 || itemID == GaHandleInvalid {
        return GaRecordItem // 8
    }
    switch itemID & 0xF0000000 {
    case 0x00000000:
        return GaRecordWeapon // 21
    case 0x10000000:
        return GaRecordArmor // 16
    default:
        return GaRecordItem // 8
    }
}
```

### Layout per type (from `GaItemFull.Serialize`, `structures.go:103-116`)

| Offset | Type | Field | Presence |
|---|---|---|---|
| 0x00 | u32 | `Handle` | always |
| 0x04 | u32 | `ItemID` | always |
| 0x08 | i32 | `Unk2` (default `-1`) | only when `recSize ≥ GaRecordArmor` (16 B) — armor and weapon |
| 0x0C | i32 | `Unk3` (default `-1`) | only when `recSize ≥ GaRecordArmor` (16 B) — armor and weapon |
| 0x10 | u32 | `AoWGaItemHandle` | only when `recSize ≥ GaRecordWeapon` (21 B) — weapon only |
| 0x14 | u8 | `Unk5` (default `0`) | weapon only |

Sizes:

| Item class | `ItemID` upper nibble | Record size | Fields |
|---|---|---|---|
| Weapon | `0x0...` | `GaRecordWeapon = 21` B | `Handle`, `ItemID`, `Unk2`, `Unk3`, `AoWGaItemHandle`, `Unk5` |
| Armor | `0x1...` | `GaRecordArmor = 16` B | `Handle`, `ItemID`, `Unk2`, `Unk3` |
| Goods / Accessory / Talisman / Ash of War | other | `GaRecordItem = 8` B | `Handle`, `ItemID` |

> ℹ️ The size is determined by `ItemID`, but the **usage type** of the item follows from the **handle** prefix (see "Handle prefixes" below). These two channels are consistent for legal saves; a mismatch between them is a symptom of corruption. Full validation: `ValidatePostMutation` in [35](35-gaitem-allocator-invariants.md#post-mutation-validation).

### `GaMap` map

- Type: `map[uint32]uint32` (handle → ItemID).
- Built in `scanGaItems` (`structures.go:660-672`): for every non-empty entry whose `typeBits ∈ {ItemTypeWeapon, ItemTypeArmor, ItemTypeAccessory, ItemTypeItem, ItemTypeAow}`, it adds the entry `GaMap[handle] = itemID`.
- **Does not contain empty entries** — `IsEmpty()` (`structures.go:96-99`) returns true when `ItemID == 0` or `ItemID == 0xFFFFFFFF`.
- Does not contain entries with `typeBits` outside the five known types — if the parser encounters a handle with a different prefix, the entry lands in `GaItems` but not in `GaMap` (`structures.go:668-672`).

### Empty entry

`scanGaItems` uses two "empty" representations:

1. Entries outside the data range: `GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}` (handle = 0, ItemID = 0).
2. Entries within range where `Handle == 0` or `ItemID ∈ {0, 0xFFFFFFFF}` — `IsEmpty()` returns true.

Implication: `slot.GaItems` is **physically always a full array** of fixed size. An empty slot is an entry with `IsEmpty() == true`, not the absence of an entry.

---

## Handle prefixes and item classes

A handle is a `u32` whose upper nibble (`handle & 0xF0000000`) determines the type. Five confirmed prefixes in the code (`structures.go:20-24`):

| Handle prefix | Constant | Class | ItemID upper nibble (after `HandleToItemID`) | Typical GaItem size |
|---|---|---|---|---|
| `0x80000000` | `ItemTypeWeapon` | Weapon | `0x0...` | 21 B |
| `0x90000000` | `ItemTypeArmor` | Armor (Protector) | `0x1...` | 16 B |
| `0xA0000000` | `ItemTypeAccessory` | Accessory / Talisman | `0x2...` | 8 B |
| `0xB0000000` | `ItemTypeItem` | Goods (consumables, materials, key items in `Inventory.KeyItems`) | `0x4...` | 8 B |
| `0xC0000000` | `ItemTypeAow` | Ash of War gem (instance) | `0x8...` | 8 B |

Additionally:

| Value | Constant | Meaning |
|---|---|---|
| `0x00000000` | `GaHandleEmpty` | empty slot (canonical) |
| `0xFFFFFFFF` | `GaHandleInvalid` | empty slot (legacy / invalid) |

> ℹ️ The **handle prefix ↔ ItemID prefix mapping** is deterministic for the five classes above (`backend/db/db.go:589-611`, `HandleToItemID`):
>
> ```
> 0x80 (weapon)    → 0x00
> 0x90 (armor)     → 0x10
> 0xA0 (talisman)  → 0x20
> 0xB0 (goods)     → 0x40
> 0xC0 (AoW gem)   → 0x80
> ```
>
> The function returns `(handle & 0x0FFFFFFF) | <ItemID prefix>`. The inverse mapping is `ItemIDToHandlePrefix` (`backend/db/db.go:613-630`). The table above describes the high-level classification — the exact mapping of ItemID sub-ranges for specific items (e.g., all DLC weapon classes) is curated per category in [36-inventory-categories-game-order](36-inventory-categories-game-order.md). DLC sub-mapping completeness: `needs verification`.

### Special case: weapon → AoW reference

A weapon record (`GaRecordWeapon = 21` B) contains a 4-byte `AoWGaItemHandle` field at offset `0x10`. This field references **another GaItem** in the same array — an Ash of War gem instance (a handle with prefix `0xC0`). Values:

| Value | Meaning |
|---|---|
| `0x00000000` | No custom AoW — canonical sentinel written by the writer and the game. |
| `0xFFFFFFFF` | No custom AoW — legacy sentinel accepted by the reader (written by older SaveForge builds before commit `4e800b9`). |
| `0xC0xxxxxx` | Valid custom AoW handle. Must resolve to an existing AoW entry in `GaMap`. |

> ⚠️ Full semantics (built-in vs custom skill resolution, handle uniqueness invariant, strict apply, dual-destination) — [54-ash-of-war](54-ash-of-war.md). Allocator-side rules for a new AoW (capacity guard `6881cb9`) — [35](35-gaitem-allocator-invariants.md#aow-allocation-edge-case-fixed-by-6881cb9).

---

## GaItems vs containers

The three layers from "Mental model" have separate capacities, roles and lifecycles.

| Layer | Size | What it holds | Lifecycle |
|---|---|---|---|
| `slot.GaItems` | 5118 or 5120 entries (fixed per slot) | the object/instance of an item with metadata (upgrade, infusion via ItemID, AoW reference) | persistent — an entry does not disappear when removed from inventory (unless someone clears it intentionally) |
| `slot.GaMap` | ≤ count of non-empty entries | quick handle → ItemID lookup | rebuilt from `GaItems` at load |
| `slot.Inventory.CommonItems` / `KeyItems` | 2688 / 384 | a reference `{handle, quantity, index}` | disappears from inventory when the slot is cleared; the object in `GaItems` may remain orphaned |
| `slot.Storage.CommonItems` / `KeyItems` | 1920 / 128 | as above | analogously |

### Why the size of GaItems ≠ the size of inventory

- Inventory + Storage common together = 2688 + 1920 = **4608 reference slots**.
- `GaItems` = 5120 entries (for new saves).
- The difference (`5120 - 4608 = 512`) provides room for:
  - Stackable goods (`0xB0`) — one GaItem may be referenced by `1..N` inventory entries sharing the same handle (the same stack).
  - AoW gems (`0xC0`) — referenced by a weapon GaItem through the `AoWGaItemHandle` field, not through a container entry.
  - Orphans — GaItem entries that no container/weapon-AoW reference points at. They accumulate when the editor removes a container record without clearing the GaItem. Repair: `RepairOrphanedGaItems` (described in [35](35-gaitem-allocator-invariants.md#source-of-truth-in-code)).

### Goods stackability (model)

For goods `0xB0`:
- One GaItem (8 B) represents one **type instance**.
- The inventory entry holds the stack's quantity.
- The same handle may be in **two different container layers** (e.g., Inventory.CommonItems and Storage.CommonItems) — both sides then use the same reference. Full transfer semantics: [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

For instance-move types (`0x80/0x90/0xA0/0xC0`):
- One GaItem = one physical instance.
- Every "copy" of the same item (e.g., two Uchigatanas with different affinities) has a separate GaItem with a separate handle.

---

## Read path

The parser loads the GaItem section for each non-empty slot in a single call (`structures.go:301`):

```
s.scanGaItems(GaItemsStart)        // GaItemsStart = 0x20
```

`scanGaItems` (`structures.go:612-728`) performs two passes:

### Pass 1 — entry parsing (`structures.go:615-672`)

1. Computes `gaLimit = s.MagicOffset - DynPlayerData + 1` (upper section bound within the slot data).
2. Selects capacity (5118 or 5120) per `slot.Version` (details in [35](35-gaitem-allocator-invariants.md#gaitem-capacity-by-slot-version)).
3. Allocates `s.GaItems = make([]GaItemFull, maxEntries)` and initializes `s.GaMap`.
4. Iterates from `start` to `gaLimit`, cursor `curr`:
   - If `curr + GaRecordItem > gaLimit` → leaves the remaining entries as empty (`Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF`).
   - Otherwise reads `Handle` (offset 0), `ItemID` (offset 4).
   - Computes `recSize := GaItemRecordSize(itemID)`.
   - If `recSize >= GaRecordArmor` (16) and `curr + 16 ≤ len(Data)` → reads `Unk2` and `Unk3`.
   - If `recSize >= GaRecordWeapon` (21) and `curr + 21 ≤ len(Data)` → reads `AoWGaItemHandle` and `Unk5`.
   - Increments `curr += recSize`.
   - If `!entry.IsEmpty()` and `typeBits ∈ {Weapon, Armor, Accessory, Item, Aow}` → adds `GaMap[handle] = itemID`.

### Pass 2 — counter reconstruction (`structures.go:681-722`)

The second pass sets `NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`, `PartGaItemHandle`. The algorithm and the properties of those counters are fully described in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md#counter-reconstruction-on-load) — chapter 03 does not duplicate it.

### After scanning

- `s.GaItems` is a full fixed-length array.
- `s.GaMap` contains entries only for non-empty entries with a legal prefix.
- `s.InventoryEnd` (`structures.go:677`) points to the byte offset after the last parsed entry — used for validation and the next slot-section segment.

---

## Write path overview

Write-side semantics for GaItem **are not described in this chapter**. Crucial for an implementer:

- New-entry allocation: `allocateGaItem` ([35 → Allocation zones](35-gaitem-allocator-invariants.md#allocation-zones)).
- Unique handle generation: `generateUniqueHandle` ([35 → Handle generation](35-gaitem-allocator-invariants.md#handle-generation)).
- Batch add: `AddItemsToSlotBatch` + `RebuildSlotFull` ([35 → Transactional safety](35-gaitem-allocator-invariants.md#transactional-safety-and-rollback)).
- Rollback: `SnapshotSlot` / `RestoreSlot` ([35](35-gaitem-allocator-invariants.md#transactional-safety-and-rollback)).
- Full architecture pre-flight → snapshot → mutate → reconcile → validate: [43-transactional-item-adding](43-transactional-item-adding.md).
- Single-entry serialization: `GaItemFull.Serialize` (`structures.go:103-116`) — used internally by the writer and by `RebuildSlotFull`.

> **Allocator/counter/capacity invariants are documented in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).** Chapter 03 intentionally does not duplicate that content.

---

## Validation relationships

The read-side GaItem layer must satisfy several relationships for the save to be interpretable. Full list of invariants and their enforcement in `ValidatePostMutation` — [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants). Read-side summary:

- Every non-empty inventory handle of type `0x80/0x90/0xC0` (weapon/armor/AoW) must exist as a key in `slot.GaMap`. A handle missing from the map → "orphan handle" (`diagnostics.go::ValidatePostMutation`, check `orphan_handle`).
- `slot.GaMap[h]` may never map to `ItemID == 0` (check `gamap_zero_id`).
- The `AoWGaItemHandle` field in a weapon record:
  - `0x00000000` or `0xFFFFFFFF` (sentinels for "no custom AoW") → legal.
  - `0xC0xxxxxx` → the handle MUST exist as an AoW entry in `GaMap`.
  - Any other value → invalid / corrupted.
- No two weapons may reference the same non-sentinel `AoWGaItemHandle` (handle uniqueness; full analysis in [54-ash-of-war](54-ash-of-war.md)).

---

## Examples

### Container entry → handle → GaMap → ItemID → DB

The save contains in `Inventory.CommonItems[5]`:

```
GaItemHandle = 0x80000003
Quantity     = 1
Index        = 15
```

Step 1: the container holds only the handle reference, nothing more.

Step 2: `slot.GaMap[0x80000003] = 0x000F4240` (1 000 000 dec) — lookup in the runtime map.

Step 3: lookup of the full record `slot.GaItems[i]` by walking the array (or by `i` known from parsing):

```
GaItems[3].Handle           = 0x80000003
GaItems[3].ItemID           = 0x000F4240  → Uchigatana +0
GaItems[3].Unk2             = -1
GaItems[3].Unk3             = -1
GaItems[3].AoWGaItemHandle  = 0x00000000   ← no custom AoW (canonical sentinel)
GaItems[3].Unk5             = 0
```

Step 4: DB lookup: `db.GetItemDataFuzzy(0x000F4240)` → `ItemData{Name: "Uchigatana", Category: "melee_armaments", ...}`.

### Weapon with a custom AoW

Suppose a weapon has Lion's Claw attached:

```
GaItems[3].Handle           = 0x80000003       ← Uchigatana +0 weapon
GaItems[3].ItemID           = 0x000F4240
GaItems[3].AoWGaItemHandle  = 0xC0000017       ← reference to the AoW gem

GaItems[42].Handle          = 0xC0000017       ← separate AoW instance
GaItems[42].ItemID          = 0x80002710       → Lion's Claw
```

- The weapon and the AoW gem are **two separate GaItem entries**.
- The AoW handle must be unique in `GaMap` (see the uniqueness invariant in [54](54-ash-of-war.md)).
- Removing the custom AoW = setting `weapon.AoWGaItemHandle = 0x00000000`; the AoW gem record remains as a free copy (detach semantics and strict apply in [54](54-ash-of-war.md)).

---

## Known limits / needs verification

- **Full list of DLC item ID ranges** per class — `needs verification`. We know the five main handle prefixes and their relationship to the ItemID upper nibble; the complete sub-range mapping for DLC (e.g., Backhand Blades, Perfume Bottles) is being curated per category in [36](36-inventory-categories-game-order.md), but chapter 03 does not attempt to duplicate it.
- **`PartGaItemHandle` semantic meaning** — the `slot.PartGaItemHandle` field (1 byte, default `0x80`) influences bits 16–23 of generated handles. Its full effect on the game (whether the game validates this field or ignores it) — `needs verification`.
- **Entries with a handle prefix outside the five known types** — the parser reads them as `GaItem` (if their size matches), but does not add them to `GaMap`. Whether the game tolerates them or treats them as corruption — `needs verification`.

---

## Cross-references

- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator, counters, capacity per slot version, snapshot/rollback, post-mutation validation, AoW guard `6881cb9`.
- [07-inventory](07-inventory.md) — Inventory layout (CommonItems, KeyItems, visibility gate, headers).
- [10-storage](10-storage.md) — Storage layout (size differences vs inventory).
- [43-transactional-item-adding](43-transactional-item-adding.md) — full Add Items architecture (pre-flight → snapshot → mutate → reconcile → validate).
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — `acquisition_index` in container entries: how the game sorts and why stride-2.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — transfer between inventory and storage; rehandle path for duplicate handle.
- [54-ash-of-war](54-ash-of-war.md) — full AoW semantics: `AoWGaItemHandle` sentinels, handle uniqueness invariant, strict apply, dual-destination.

---

## Sources

- `backend/core/structures.go` — `GaItemFull`, `GaItemRecordSize`, `scanGaItems`, `SaveSlot.GaItems`, `SaveSlot.GaMap`, `InventoryItem`, `IsEmpty`, `Serialize`
- `backend/core/offset_defs.go` — `GaItemsStart`, `GaRecordWeapon/Armor/Item`, `GaHandleEmpty/Invalid/TypeMask`
- `backend/core/diagnostics.go` — `ValidatePostMutation` (read-side invariants)
- `backend/db/db.go` — `HandleToItemID`, `ItemIDToHandlePrefix`, `GetItemDataFuzzy`
- Rust ER-Save-Editor: `src/save/common/save_slot.rs` (`GaItem2` struct, reference model)
- er-save-manager: `parser/er_types.py` (`Gaitem` class), `parser/user_data_x.py` (`gaitem_map`)
