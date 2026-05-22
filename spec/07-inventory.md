# 07 — Inventory: data model and read path

> **Type**: Binary format spec
> **Status**: ✅ canonical, implemented, source-of-truth verified against the current backend. Allocator/capacity/counter invariants are described in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md). Transactional Add Items — in [43-transactional-item-adding](43-transactional-item-adding.md).
> **Scope**: Read-side binary model of `slot.Inventory` (common items + key items), trailing counters, load-time reconciliation, relation to GaItems/GaMap. The chapter covers what the parser does when loading inventory and how the runtime structures mirror raw bytes. Full semantics of Inventory ↔ Storage transfer, sorting and drag-and-drop UI remain in [53-inventory-storage-transfer](53-inventory-storage-transfer.md) and [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).

---

## Chapter goal

The chapter describes:

- where inventory sits in the save file (offset relative to `slot.MagicOffset`);
- how the two collections are built: `CommonItems` (2688 slots) and `KeyItems` (384 slots);
- the format of a single inventory record (12 B: `handle`, `quantity`, `index`);
- trailing counters: `NextEquipIndex`, `NextAcquisitionSortId`, and the `common_item_count` header;
- what the parser does on load and which reconciliations it performs automatically;
- how an inventory entry references the GaMap / GaItems / DB layers;
- which invariants must hold on the read side, and which are enforced by the allocator/validator (link to 35).

What the chapter does **NOT** do:

- Does not describe the allocator side (allocation of new entries, capacity, AoW guard, post-mutation validation, snapshot/rollback) — that is all in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
- Does not describe the full semantics of Inventory ↔ Storage transfer (rehandle, cap-aware partial merge) — that is in [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- Does not describe the stride-2 algorithm of Sort Order / Acquisition reorder — that is in [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- Does not describe the Add Items architecture (pre-flight, snapshot, rollback) — that is in [43-transactional-item-adding](43-transactional-item-adding.md).

---

## Status

- canonical
- implemented (parser in `mapInventory` verified against round-trip PS4 + PC fixtures)
- source-of-truth: backend code
- allocator details: **chapter 35**

---

## Source of truth in code

| Topic | Files / functions | Note |
|---|---|---|
| Inventory structure | `backend/core/structures.go:124-131` (`EquipInventoryData`) | fields: `CommonItems`, `KeyItems`, `NextEquipIndex`, `NextAcquisitionSortId`, private `nextEquipIndexOff` and `nextAcqSortIdOff` |
| Single record | `backend/core/structures.go:118-122` (`InventoryItem`) | `GaItemHandle uint32`, `Quantity uint32`, `Index uint32` |
| Layout constants | `backend/core/offset_defs.go:60-73` | `InvStartFromMagic = 505`, `CommonItemCount = 0xA80 (2688)`, `KeyItemCount = 0x180 (384)`, `InvRecordLen = 12`, `InvKeyCountHeader = 4`, `InvSafetyMargin = 0x9000` |
| Container parser | `backend/core/structures.go:156-195` (`(*EquipInventoryData).Read`) | sequence: common × `commonCount`, skip 4 B key_count header, key × `keyCount`, NextEquipIndex (remembered offset), NextAcquisitionSortId (remembered offset) |
| Top-level load | `backend/core/structures.go:757-804` (`(*SaveSlot).mapInventory`) | calls `Read`, reconciles counters (acq sort id, equip index), reconciles the binary `common_item_count` header via `ReconcileInventoryHeader` |
| Reconcile header | `backend/core/writer.go:993-1013` (`ReconcileInventoryHeader`) | computes non-empty count and writes it at `invStart - 4` |
| Deep copy | `backend/core/structures.go:138-154` (`(*EquipInventoryData).Clone`) | used by `SnapshotSlot` |
| UI capacity binding | `app_deploy.go:31-38, 289-306` (`SlotCapacity`, `GetSlotCapacity`) | exposes `non_empty / max` (2688) per Inventory container; UI fix for "armament zone free" — see [35](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity) |

---

## Mental model

Inventory sits in a fixed data area of the slot, addressed by `slot.MagicOffset + InvStartFromMagic`. This is the **container layer** of the GaItem model (see [03-gaitem-map](03-gaitem-map.md#mental-model)) — it holds references to `GaItems` entries via `GaItemHandle`, plus per-stack `Quantity` and acquisition `Index` used by in-game sorting.

```
slot.Data
 │
 ├── [MagicOffset + InvStartFromMagic − 4] ┐
 │       common_item_count (u32)           │  4 B header
 │       (game uses this as "next insert    │
 │        index" and "inventory full"       │
 │        gate; reconciled at load)         │
 ├── [MagicOffset + InvStartFromMagic]     ┘
 │       CommonItems × 2688                    32 256 B  (0x7E00)
 │       — each record 12 B: {handle, qty, index}
 │
 ├── (after common)                        ┐
 │       key_count header (u32)            │  4 B (skipped on read, not used at runtime)
 │
 ├── (next)                                ┘
 │       KeyItems × 384                        4 608 B  (0x1200)
 │       — identical 12 B record
 │
 ├── (after key)                           ┐
 │       NextEquipIndex (u32)              │  4 B  (offset remembered in nextEquipIndexOff)
 │       NextAcquisitionSortId (u32)       │  4 B  (offset remembered in nextAcqSortIdOff)
 └── ───────────────────────────────────────┘
```

Total size of the section (without the leading 4 B header, but with trailing counters):

```
CommonItems   = 2688 × 12 = 32 256  (0x7E00)
key_count hdr =                  4  (0x0004)
KeyItems      =  384 × 12 =  4 608  (0x1200)
NextEquipIdx  =                  4  (0x0004)
NextAcqSortId =                  4  (0x0004)
──────────────────────────────────
TOTAL                     = 36 876  (0x900C)
```

`InvSafetyMargin = 0x9000` (`offset_defs.go:70`) is the validation threshold for "whether the slot has space at all for the inventory section". The value is **lower** than the full section length (`0x900C`) by 12 B; the parser calls `Read` only if `invStart + InvSafetyMargin < len(slot.Data)`. Whether this 12-byte difference is intentional (accepting extremely short saves in which trailing counters may extend past the margin) or a historical mistake — `needs verification`.

---

## Binary / runtime structures

### `InventoryItem` (12 B)

```go
type InventoryItem struct {
    GaItemHandle uint32  // 0x00 — handle into GaMap (or 0/0xFFFFFFFF = empty)
    Quantity     uint32  // 0x04 — count in the stack (1 for instance items)
    Index        uint32  // 0x08 — per-record acquisition index
}
```

- Empty slot: `GaItemHandle == GaHandleEmpty (0x00000000)` or `GaItemHandle == GaHandleInvalid (0xFFFFFFFF)`.
- `Quantity` for instance-move types (weapon `0x80`, armor `0x90`, talisman `0xA0`, AoW `0xC0`) is always `1`. For stackable goods (`0xB0`) it holds the actual count; the top bit (`& 0x80000000`) was historically used in other slots of the game as a flag — in Elden Ring inventory not observed; applications reading qty mask with `& 0x7FFFFFFF` (see e.g. `app.go:364, 378, 389` in `AddItemsToCharacter`).
- `Index` is the acquisition counter used by the in-game sort "Acquisition Order" — full semantics (stride-2 + even base + `InvEquipReservedMax`) in [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).

### `EquipInventoryData`

```go
type EquipInventoryData struct {
    CommonItems           []InventoryItem  // length: CommonItemCount = 2688
    KeyItems              []InventoryItem  // length: KeyItemCount = 384
    NextEquipIndex        uint32
    NextAcquisitionSortId uint32
    nextEquipIndexOff     int  // absolute byte offset in slot.Data (write-back)
    nextAcqSortIdOff      int  // absolute byte offset in slot.Data (write-back)
}
```

- `nextEquipIndexOff` / `nextAcqSortIdOff` are **private** — they hold the byte offset in `slot.Data` for in-place write-back. Reconcile at load and every mutation of the counters use these offsets. The public method `NextEquipIndexOff()` exposes the first offset for tests (`structures.go:135`).
- `Clone()` (`structures.go:138-154`) — deep copy, including private offsets. Used by `SnapshotSlot` (see [35 → Transactional safety](35-gaitem-allocator-invariants.md#transactional-safety-and-rollback)).

---

## Container entries

Each inventory slot (both common and key) is physically present in the binary — it is never "compacted" when no item is held. An empty slot = 12 B of zeros or one with handle `0xFFFFFFFF`.

| Count | Constant | Bytes | Section in slot.Data |
|---|---|---|---|
| 2688 | `CommonItemCount` (`0xA80`) | 32 256 (0x7E00) | `MagicOffset + InvStartFromMagic ... + 0x7E00` |
| 384 | `KeyItemCount` (`0x180`) | 4 608 (0x1200) | after `key_count` header (4 B) |

**Why common and key are split**: the game renders them in separate UI tabs, and some operations (e.g. key_count header inventory accounting, sorting) treat the two sections differently. From the data-model perspective both sections use the identical 12 B record.

> ℹ️ The number of inventory records is not the same as the number of `GaItems` entries. An inventory entry carries only a reference (handle) — the physical object lives in `GaItems`. Goods (`0xB0`) can be represented by a single `GaItems` entry shared between inventory and storage records via the same handle. Full analysis of layer relationships — [03 → GaItems vs containers](03-gaitem-map.md#gaitems-vs-containers).

### `common_item_count` header

At offset `invStart - 4` (i.e. `MagicOffset + InvStartFromMagic - 4`) lies a 4-byte `common_item_count` field. The comment in `structures.go:797-803` describes the role of the header in the game:

> "External editors may leave this counter stale (er-save-manager increments on add but not on remove; our old code never touched it at all). A stale counter causes the game to use the wrong insertion index, overwriting valid items or incorrectly triggering 'inventory full'."

Save Forge fixes the mismatch on every load via `ReconcileInventoryHeader` (`writer.go:993-1013`) — recomputes the non-empty count in `Inventory.CommonItems` and writes it into the binary. The exact mechanism of how the game uses this value in the current game patch (literal "next insert index" for pickups + literal "inventory full" threshold) follows from the code comment and editor-side observations; full runtime semantics — `needs verification`.

> ℹ️ The header for key items (`key_count`) lies between the common and key sections (offset `invStart + CommonItemCount*12`). In the current code the parser skips it (`structures.go:170-172` — `r.ReadU32()` without assignment). Save Forge does not reconcile the key_count header — `needs verification` whether the game uses it actively.

---

## Relationship to GaItems and GaMap

An inventory entry connects to the GaItem model via the handle (`InventoryItem.GaItemHandle`). Lookup chain:

```
Inventory.CommonItems[i].GaItemHandle  →  slot.GaMap[handle]  →  ItemID
                                                           ↓
                                       slot.GaItems[?]  with handle = ...
                                       (full data: ItemID, AoWGaItemHandle, Unk2/3/5)
```

Required relations (read-side):

- Every non-empty handle of type **weapon (0x80)**, **armor (0x90)** or **AoW (0xC0)** must have an entry in `slot.GaMap` (enforced by `ValidatePostMutation` as `orphan_handle`, [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants)).
- Talisman (`0xA0`) and goods (`0xB0`) — enforcement identical (the validator accepts all five prefixes; the concrete check is narrowed in code to `0x80/0x90/0xC0` as the ones whose absence in GaMap leads to a crash — full description in [35](35-gaitem-allocator-invariants.md#post-mutation-validation)).
- `slot.GaMap[handle]` may not return `ItemID == 0` (validator check `gamap_zero_id`).
- No duplicates of `Index` in the joint set `CommonItems ∪ KeyItems` (validator check `duplicate_index`); repair: `RepairDuplicateInventoryIndices` (`backend/core/inventory_index_repair.go`).

> ⚠️ An inventory entry **need not** have a corresponding `GaItems` entry for stackable goods (`0xB0`) with a shared handle between inventory and storage — that is a legal configuration after batch add. Full semantics of the handle ↔ record relation for stackable items — [03 → GaItems vs containers](03-gaitem-map.md#gaitems-vs-containers) and [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

---

## Read path

The parser loads inventory in two steps.

### Step 1 — `EquipInventoryData.Read` (`structures.go:156-195`)

1. Allocates `CommonItems = make([]InventoryItem, commonCount)` and reads `commonCount × 12 B` records.
2. Skips the 4 B key_count header (`r.ReadU32()` without storing).
3. Allocates `KeyItems = make([]InventoryItem, keyCount)` and reads `keyCount × 12 B` records.
4. Writes `r.Pos()` as `nextEquipIndexOff`, reads `NextEquipIndex`.
5. Writes `r.Pos()` as `nextAcqSortIdOff`, reads `NextAcquisitionSortId`.

### Step 2 — `(*SaveSlot).mapInventory` (`structures.go:757-804`)

Called once during `LoadSave`:

1. `invStart := s.MagicOffset + InvStartFromMagic`.
2. Validates `invStart + InvSafetyMargin < len(s.Data)` — when the slot is empty or corrupted, the parser skips inventory.
3. `Inventory.Read(ir, CommonItemCount, KeyItemCount)` loads records + counters.
4. **Reconcile `NextAcquisitionSortId`**: find `maxIdx = max(Index)` over `CommonItems` with non-empty handle; if `NextAcquisitionSortId <= maxIdx`, set `NextAcquisitionSortId = maxIdx + 1` (in-memory only).
5. **Reconcile `NextEquipIndex`**: if `NextEquipIndex < NextAcquisitionSortId`, set `NextEquipIndex = NextAcquisitionSortId` **and** perform a binary write-back at `nextEquipIndexOff` (for `Version > 0`).
6. `ReconcileInventoryHeader(s)` — recomputes and writes the binary `common_item_count` header at `invStart - 4`.

After these steps:
- `slot.Inventory.CommonItems` has 2688 records (physical array).
- `slot.Inventory.KeyItems` has 384 records.
- `nextEquipIndexOff` and `nextAcqSortIdOff` point to positions in `slot.Data` used by runtime mutations for write-back.

---

## Write path overview

Write-side semantics for inventory:

- **Add Items** — pre-flight + snapshot + batch add + reconcile + post-mutation validation: [43-transactional-item-adding](43-transactional-item-adding.md).
- **Remove / repair orphans** — `writer.go::RepairOrphanedGaItems`: see [35 → Source of truth](35-gaitem-allocator-invariants.md#source-of-truth-in-code).
- **Reorder (Acquisition Sort)** — `app_inventory_order.go::ReorderInventory` with stride-2: [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- **Transfer Inventory → Storage** — `transfer.go::MoveItemsBetweenContainers` with equipped guard and rehandle path: [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- **Single counter mutation** — e.g. `Inventory.NextEquipIndex` or `NextAcquisitionSortId`: in-memory update + write-back at `nextEquipIndexOff` / `nextAcqSortIdOff`. `common_item_count` header written by `ReconcileInventoryHeader`.

---

## Capacity and counters

### Container capacity (Inventory layer)

| What | Value | Constant | Enforced by |
|---|---|---|---|
| Common items max slots | 2688 | `CommonItemCount` | `Inventory.Read` (slice allocation of this length); `ReconcileInventoryHeader` |
| Key items max slots | 384 | `KeyItemCount` | as above |
| Bytes per record | 12 | `InvRecordLen` | `Inventory.Read`, allocator, transfer |
| Reserved acquisition range | 0..432 | `InvEquipReservedMax` | `app_inventory_order.go` (acquisition stride-2 base) |

### Trailing counters

| Counter | Type | Location | Reconcile on load? |
|---|---|---|---|
| `common_item_count` header | u32 | `invStart - 4` | yes, `ReconcileInventoryHeader` (binary write-back) |
| `key_count` header | u32 | `invStart + CommonItemCount × 12` | no (parser skips; `needs verification` whether the game uses it) |
| `NextEquipIndex` | u32 | `nextEquipIndexOff` (after KeyItems) | yes, if `< NextAcquisitionSortId` (binary write-back) |
| `NextAcquisitionSortId` | u32 | `nextAcqSortIdOff` (after NextEquipIndex) | yes, in-memory without write-back (binary updated by allocator/reorder) |

### UI capacity gap (callout)

> ⚠️ `INVENTORY used/max` shown in the UI (`app_deploy.go::SlotCapacity` → `frontend/src/App.tsx:529-531`) reports `non_empty / CommonItemCount` — that is, **container** capacity (2688 common slots). This is **not** the same as actual free allocator space for new weapons/AoW. The allocator on the `slot.GaItems` side has its own zone-aware limits (`NextArmamentIndex`, capacity `len(GaItems)`), which can be exhausted even though Inventory shows 60/2688. Full explanation and pathological example: [35 → UI counters vs allocator capacity](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity).

---

## Validation relationships

The read-side inventory layer must satisfy several relations. Full list of invariants and their enforcement by `ValidatePostMutation` — [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants). Inventory summary:

- Every non-empty inventory handle of type `0x80/0x90/0xC0` must exist in `slot.GaMap` (check `orphan_handle`).
- No two entries from `CommonItems ∪ KeyItems` with a non-empty handle may share the same `Index` (check `duplicate_index`); repair: `RepairDuplicateInventoryIndices`.
- `slot.GaMap[h] != 0` for every handle.
- `common_item_count` header == number of non-empty records in `CommonItems` (enforced by `ReconcileInventoryHeader` at load and after every mutation).
- `NextEquipIndex >= NextAcquisitionSortId` after reconcile (enforced by `mapInventory`).
- `NextAcquisitionSortId > max(Index)` after reconcile (enforced by `mapInventory`).

### Mechanics of `NextEquipIndex`

Historically called the "visibility gate" — hypothesis that the game hides entries with `Index >= NextEquipIndex` (the item exists in the binary, but the UI does not show it). Save Forge detects and fixes `NextEquipIndex < NextAcquisitionSortId` on every load. The mechanic itself (whether the game **definitely** hides items with `Index >= NextEquipIndex` in the current game patch) — `needs verification` via fresh in-game verification.

---

## Examples

### Stackable goods in common inventory

The save contains in `Inventory.CommonItems[42]`:

```
GaItemHandle = 0xB0001234       ← goods, prefix 0xB0
Quantity     = 50               ← count in stack
Index        = 312              ← acquisition index
```

Lookup:

```
slot.GaMap[0xB0001234] = 0x40001234   ← goods ItemID (prefix 0x4 after HandleToItemID)
slot.GaItems[?].Handle = 0xB0001234   ← 8-byte record {Handle, ItemID}
db.GetItemDataFuzzy(0x40001234)       → e.g. Smithing Stone [3]
```

`Inventory.CommonItems[42]` and (optionally) `Storage.CommonItems[X]` may share the same handle — both sides then share the physical stack via the same GaMap record. Quantity-merge operations on transfer: [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

### Instance weapon

```
Inventory.CommonItems[5].GaItemHandle = 0x80000003
Inventory.CommonItems[5].Quantity     = 1
Inventory.CommonItems[5].Index        = 15

slot.GaMap[0x80000003] = 0x000F4240        ← Uchigatana ItemID
slot.GaItems[3] = GaItemFull{
    Handle:          0x80000003,
    ItemID:          0x000F4240,
    Unk2:            -1,
    Unk3:            -1,
    AoWGaItemHandle: 0x00000000,             ← canonical sentinel
    Unk5:            0,
}
```

---

## Known limits / needs verification

- **Mechanics of `NextEquipIndex` as a "visibility gate"** — the code reconciles the value on every load, but the effect on the game (whether items with `Index >= NextEquipIndex` really are hidden in the current game patch) — `needs verification`.
- **`key_count` header (4 B between common and key)** — the parser skips it, Save Forge does not reconcile. Whether the game uses this value at runtime or ignores it — `needs verification`.
- **Top bit of `Quantity` (`& 0x80000000`)** — the application masks via `& 0x7FFFFFFF` in many places (`app.go:364, 378, 389`), suggesting that the game uses this bit as a flag in some slots. Specific meaning — `needs verification`.
- **`InvSafetyMargin = 0x9000`** vs actual section size `0x900C` — the 12-byte difference is an intentional rounding of the validation threshold, but **not** a reserve — `needs verification` whether this margin ever causes legal inventory to be skipped for edge-case saves.

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — GaItem/GaMap model, handle prefixes, inventory → GaMap → GaItems relation.
- [10-storage](10-storage.md) — Storage data model, differences vs Inventory.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator, capacity, counters, validation, snapshot/rollback, UI capacity gap.
- [43-transactional-item-adding](43-transactional-item-adding.md) — full Add Items architecture (pre-flight → snapshot → mutate → reconcile → validate).
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — `acquisition_index` in `InventoryItem.Index`: how the game sorts "Acquisition Order" and why stride-2.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — Inventory ↔ Storage transfer (instance-move vs quantity-merge, rehandle, equipped guard, cap-aware partial).
- [54-ash-of-war](54-ash-of-war.md) — semantics of custom AoW pinned to a weapon stored in inventory.

---

## Sources

- `backend/core/structures.go` — `EquipInventoryData`, `InventoryItem`, `(*EquipInventoryData).Read`, `(*SaveSlot).mapInventory`
- `backend/core/offset_defs.go` — `InvStartFromMagic`, `CommonItemCount`, `KeyItemCount`, `InvRecordLen`, `InvKeyCountHeader`, `InvSafetyMargin`, `InvEquipReservedMax`
- `backend/core/writer.go` — `ReconcileInventoryHeader`, `RepairOrphanedGaItems`
- `backend/core/diagnostics.go` — `ValidatePostMutation` (orphan_handle, duplicate_index, gamap_zero_id, gaitem_indices)
- `app.go::AddItemsToCharacter` — top-level orchestrator (pre-flight → snapshot → batch → reconcile → validate)
- `app_deploy.go::SlotCapacity`, `GetSlotCapacity` — UI binding (`non_empty / max`)
- Round-trip fixtures in `tests/roundtrip_test.go`, `tests/save_modify_test.go`, `tests/capacity_test.go`
