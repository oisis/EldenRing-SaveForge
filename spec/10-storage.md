# 10 — Storage Box: data model and read path

> **Type**: Binary format spec
> **Status**: ✅ canonical, implemented for read-side and write-side core. Storage Apply (UI flow that writes the order on the Storage side) — still `needs verification` via a fresh in-game / Steam Deck test. Allocator/capacity/counter invariants are described in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md). Transactional Add Items — in [43-transactional-item-adding](43-transactional-item-adding.md).
> **Scope**: Read-side binary model of `slot.Storage` (common items + key items), header count, trailing counters, relation to GaItems/GaMap, differences vs Inventory. Full semantics of Inventory ↔ Storage transfer (instance-move, quantity-merge, rehandle, equipped guard) is in [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

---

## Chapter goal

The chapter describes:

- where Storage Box sits in the save file (dynamic offset — `slot.StorageBoxOffset`);
- how the `slot.Storage` table is built (1920 common + 128 key);
- the format of a single record (12 B — identical to Inventory);
- the header count at the start of the section + trailing counters at the end;
- what the parser does on load and which reconciliations it performs;
- how a storage entry references the GaMap / GaItems layers;
- how Storage differs from Inventory in terms of layout and load path;
- which invariants must hold on the read side.

What the chapter does **NOT** do:

- Does not describe the full Inventory ↔ Storage transfer (rehandle path, cap-aware partial merge, equipped guard) — that is in [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- Does not describe the stride-2 algorithm of Sort Order / Storage reorder — that is in [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- Does not describe the Add Items architecture (Storage Qty branch in `AddItemsToCharacter`) — that is in [43-transactional-item-adding](43-transactional-item-adding.md).
- Does not describe the allocator side (`allocateGaItem`, capacity, validation, snapshot/rollback) — that is in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).

---

## Status

- canonical (read-side + write-side core)
- implemented (parser in `mapInventory` storage branch, transfer in `MoveItemsBetweenContainers`, reorder in `ReorderStorage`)
- source-of-truth: backend code
- **Storage Apply in-game verification**: `needs verification` (see "Known limits" section)
- allocator details: **chapter 35**

---

## Source of truth in code

| Topic | Files / functions | Note |
|---|---|---|
| Storage structure | `backend/core/structures.go:227-228` (`SaveSlot.Storage EquipInventoryData`) | same type as Inventory, different sizes and different header layout |
| Single record | `backend/core/structures.go:118-122` (`InventoryItem`) | `GaItemHandle`, `Quantity`, `Index` — identical 12 B with Inventory |
| Layout constants | `backend/core/offset_defs.go:66-78` | `StorageItemCount = 2048`, `StorageCommonCount = 0x780 (1920)`, `StorageKeyCount = 0x80 (128)`, `StorageHeaderSkip = 4`, `StorageSafetyMarg = 0x6000`, `StorageNextEquipIdxRel`, `StorageNextAcqSortRel` |
| Container parser | `backend/core/structures.go:725-755` (`(*EquipInventoryData).ReadStorage`) | reads `count` records linearly, **skips empty entries** and returns only non-empty ones in `CommonItems` |
| Top-level load | `backend/core/structures.go:805-825` (`mapInventory` — storage branch) | calls `ReadStorage(StorageItemCount)`, reads trailing counters from fixed offsets, remembers offsets for write-back |
| Reconcile header | `backend/core/writer.go:971-991` (`ReconcileStorageHeader`) | recomputes non-empty count in the binary and writes it at `slot.StorageBoxOffset` |
| Storage offset (dynamic) | `backend/core/structures.go::SaveSlot.StorageBoxOffset` (line 239) | computed in the chain of offsets, not relative to `MagicOffset` |
| UI capacity binding | `app_deploy.go:31-38, 289-306` (`SlotCapacity`, `GetSlotCapacity`) | exposes `non_empty / StorageCommonCount` (1920) |

---

## Mental model

The Storage Box is the second container of references to GaItems, available in the game at a Site of Grace. From the data-model perspective it shares with Inventory:

- the same record type (`InventoryItem`, 12 B);
- the same Go container type (`EquipInventoryData`);
- the same trailing-counter pattern (`NextEquipIndex`, `NextAcquisitionSortId`);
- the same invariants (orphan handle, duplicate index — see [35](35-gaitem-allocator-invariants.md#required-invariants)).

Differences from Inventory:

| Aspect | Inventory | Storage |
|---|---|---|
| Base offset | `slot.MagicOffset + InvStartFromMagic` (relative) | `slot.StorageBoxOffset` (**dynamic**) |
| Header count | `invStart − 4` (before the first slot) | `slot.StorageBoxOffset` (the offset value itself; records begin at `+ StorageHeaderSkip = +4`) |
| Common slots | 2688 (`CommonItemCount`) | 1920 (`StorageCommonCount`) |
| Key slots | 384 (`KeyItemCount`) | 128 (`StorageKeyCount`) |
| Parser | `(*EquipInventoryData).Read` — allocates `[]InventoryItem` of fixed length | `(*EquipInventoryData).ReadStorage` — filters out empty, returns a slice of **only non-empty** |
| Reconcile counters on load | yes (NextAcqSortId, NextEquipIndex) | no reconcile in `mapInventory`, only remembers offsets + reads values |
| Reconcile header on load | yes (`ReconcileInventoryHeader`) | not on load; yes after batch add (`AddItemsToCharacter` → `ReconcileStorageHeader`) |

```
slot.Data
 │
 ├── [slot.StorageBoxOffset]              ┐
 │       storage_count (u32)              │  4 B header
 │       (number of non-empty in common;  │
 │        written by ReconcileStorageHeader)
 │
 ├── [+ StorageHeaderSkip = +4]            ┘
 │       Common slots × 1920                  23 040 B  (0x5A00)
 │       — 12 B per record
 │
 ├── (after common)                       ┐
 │       key_count header (u32)           │  4 B
 │
 ├── (next)                                ┘
 │       Key slots × 128                       1 536 B  (0x0600)
 │       — 12 B per record
 │
 ├── (after key)                          ┐
 │       NextEquipIndex (u32)              │  4 B  (offset = StorageBoxOffset + StorageHeaderSkip + StorageNextEquipIdxRel)
 │       NextAcquisitionSortId (u32)       │  4 B  (offset = ... + StorageNextAcqSortRel)
 └── ───────────────────────────────────────┘
```

Total size of the section (from `StorageBoxOffset`, including the header):

```
storage_count hdr =                  4  (0x0004)
Common slots      = 1920 × 12 = 23 040  (0x5A00)
key_count hdr     =                  4  (0x0004)
Key slots         =  128 × 12 =  1 536  (0x0600)
NextEquipIdx      =                  4  (0x0004)
NextAcqSortId     =                  4  (0x0004)
─────────────────────────────────────
TOTAL                          = 24 592  (0x6010)
```

`StorageSafetyMarg = 0x6000` (`offset_defs.go:71`) is the validation threshold — the parser calls `ReadStorage` only if `storageStart + StorageSafetyMarg < len(slot.Data)`.

---

## Binary / runtime structures

### `InventoryItem` (12 B) — identical to Inventory

```go
type InventoryItem struct {
    GaItemHandle uint32  // 0x00 — handle into GaMap (or 0/0xFFFFFFFF = empty)
    Quantity     uint32  // 0x04 — count in the stack (1 for instance items)
    Index        uint32  // 0x08 — per-record acquisition index
}
```

Full description of this record — [07 → Binary / runtime structures](07-inventory.md#binary--runtime-structures). Storage uses an identical layout.

### `EquipInventoryData` after `ReadStorage`

Unlike Inventory, where `CommonItems` physically has 2688 entries (with empty entries as zeroed records), **storage `CommonItems` contains only non-empty entries** — `ReadStorage` filters empty handle entries and appends the rest (`structures.go:741-751`):

```go
// Skip empty/invalid entries but continue reading — storage can have sparse gaps
// after item removal. Breaking on first empty would lose items after the gap.
if handle == GaHandleEmpty || handle == GaHandleInvalid {
    continue
}
e.CommonItems = append(e.CommonItems, InventoryItem{...})
```

Consequences:

- `ReadStorage` is called with `count = StorageItemCount = 2048` (`structures.go:810`), not `StorageCommonCount = 1920`. The upper bound of `len(slot.Storage.CommonItems)` is therefore **2048** (not 1920) — all iterations that do not hit an empty handle are appended. In practice, non-empty handles in the 1920–2048 area are false reads resulting from reader overshoot (see the "`StorageItemCount` vs `StorageCommonCount` inconsistency" callout below and Known limits).
- `ReconcileStorageHeader` (`writer.go:971-991`) operates exclusively on the first `StorageCommonCount = 1920` slots of the binary — i.e. it writes the non-empty count for the correct common section to the header, ignoring the reader's overshoot.
- Storage `KeyItems` is always initialized as an empty slice (`structures.go:753` — `e.KeyItems = []InventoryItem{}`). **Save Forge in the current code does not expose storage key items in the runtime model** — all operations (transfer, reorder, capacity) operate on storage common only. `needs verification` whether the game uses storage key items actively.
- An index in `CommonItems` does not correspond to the binary position — it is the index in a "compacted" slice after empty skipping. The binary position can be reconstructed only by re-scanning `slot.Data`.

### `StorageBoxOffset`

`slot.StorageBoxOffset` is a **dynamic offset** computed in the chain of offsets (`structures.go:239`). Unlike inventory, which is at a fixed position `MagicOffset + 505`, storage may lie at a different place depending on the length of previous sections (e.g. world geometry, event flags). The full chain — out of scope for this chapter (see `structures.go:331-...` for offset chain resolution details).

---

## Container entries

| Count | Constant | Bytes | Section in slot.Data |
|---|---|---|---|
| 1920 | `StorageCommonCount` (`0x780`) | 23 040 (0x5A00) | `StorageBoxOffset + 4 ... + 0x5A04` |
| 128 | `StorageKeyCount` (`0x80`) | 1 536 (0x0600) | after `key_count` header (4 B) |

> ℹ️ **`StorageItemCount` vs `StorageCommonCount` inconsistency**: the constant `StorageItemCount = 2048` (`offset_defs.go:66`) is used as the `count` argument in the `ReadStorage` call (`structures.go:810`), while `StorageCommonCount = 1920` is the physical length of common slots. The difference of 128 corresponds to `StorageKeyCount`. ReadStorage reads `count × 12 = 24576` B linearly (overshooting the common section by 1536 B), filtering empty entries along the way. This can read through the `key_count` header (4 B at offset `StorageCommonCount × 12 = 23040`) and fall into the key items section as a "continuation of common". In practice non-empty handles from this area are either absent or interpreted as common entries. **Status: `needs verification`** — whether this overshoot is intentional (accepting key items as pseudo-common on load), or a historical mistake that works "by accident" because storage is rarely full.

---

## Relationship to GaItems and GaMap

Identical to Inventory (see [07 → Relationship to GaItems and GaMap](07-inventory.md#relationship-to-gaitems-and-gamap)). Summary:

- A storage entry carries only `GaItemHandle`; lookup `slot.GaMap[handle] → ItemID` and `slot.GaItems[?]` with full metadata.
- Storage **shares handle space** with Inventory — the same handle may appear in both containers (legal for stackable goods `0xB0` after batch add or transfer).
- The `orphan_handle` validation in `ValidatePostMutation` covers **Inventory** entries (`diagnostics.go:373-389`); storage entries are not checked directly by this check in the current code — `needs verification` whether storage orphans are detected by another mechanism (e.g. `RepairOrphanedGaItems` scans both containers).

> ⚠️ Full semantics of **rehandle** on transfer of a duplicate handle (talisman, weapon, armor) — [53-inventory-storage-transfer](53-inventory-storage-transfer.md). In short: an instance-move handle (`0x80/0x90/0xA0/0xC0`) already existing on the destination side causes allocation of a new unique handle for the moved instance via `generateUniqueHandle` ([35](35-gaitem-allocator-invariants.md#handle-generation)) — to preserve separation of GaItem entries between containers.

---

## Read path

The parser loads storage in two steps.

### Step 1 — `EquipInventoryData.ReadStorage` (`structures.go:725-755`)

1. Initializes `CommonItems = []InventoryItem{}` (empty slice).
2. Iterates `count` records linearly (in the current code `count = StorageItemCount = 2048`):
   - Reads `handle`, `quantity`, `index` (each u32).
   - If `handle ∈ {GaHandleEmpty, GaHandleInvalid}` — `continue` (skip empty, but keep reading).
   - Otherwise — `append` to `CommonItems`.
3. Initializes `KeyItems = []InventoryItem{}` (always empty; storage key items not supported at runtime).

### Step 2 — `(*SaveSlot).mapInventory` storage branch (`structures.go:805-824`)

Called once during `LoadSave`, after the inventory branch:

1. `storageStart := s.StorageBoxOffset + StorageHeaderSkip`.
2. Validates `storageStart + StorageSafetyMarg < len(s.Data)` — when the slot is empty or corrupted, the parser skips storage.
3. `Storage.ReadStorage(sr, StorageItemCount)` (linear read with empty filtering).
4. Reads trailing counters from **fixed relative offsets** (`StorageNextEquipIdxRel`, `StorageNextAcqSortRel`):
   - `s.Storage.nextEquipIndexOff = storageNextEquipOff`
   - `s.Storage.NextEquipIndex = binary.LittleEndian.Uint32(s.Data[storageNextEquipOff:])`
   - `s.Storage.nextAcqSortIdOff = storageNextAcqOff`
   - `s.Storage.NextAcquisitionSortId = binary.LittleEndian.Uint32(s.Data[storageNextAcqOff:])`

After these steps:

- `slot.Storage.CommonItems` has 0..1920 entries (non-empty only; binary position not preserved).
- `slot.Storage.KeyItems` is an empty slice.
- `slot.Storage.nextEquipIndexOff` and `nextAcqSortIdOff` point to positions in `slot.Data` used by runtime mutations for write-back.

> ℹ️ Unlike the inventory branch, **the storage branch does not reconcile counters or the header on load**. The header reconcile (`ReconcileStorageHeader`) is called by `AddItemsToCharacter` after batch add (`app.go:622-623`) and after a batch transfer (`transfer.go::MoveItemsBetweenContainers:128`). Counter reconcile on the storage side — `needs verification` whether it is done at all (Inventory has it in `mapInventory`; Storage probably does not).

---

## Write path overview

Write-side semantics for storage:

- **Add Items (Storage Qty branch)** — pre-flight + snapshot + batch add (with `ItemToAdd.StorageQty > 0`) + `ReconcileStorageHeader` + post-mutation validation: [43-transactional-item-adding](43-transactional-item-adding.md).
- **Transfer Inventory ↔ Storage** — `transfer.go::MoveItemsBetweenContainers` with bidirectional instance-move/quantity-merge, equipped guard, rehandle path: [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- **Reorder Storage** — `app_inventory_order.go::ReorderStorage` with stride-2: [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- **Reconcile storage_count header** — `writer.go::ReconcileStorageHeader` (recomputes non-empty count in common slots and writes it at `slot.StorageBoxOffset`).

---

## Capacity and counters

### Container capacity (Storage layer)

| What | Value | Constant | Enforced by |
|---|---|---|---|
| Common items max slots | 1920 | `StorageCommonCount` | `ReconcileStorageHeader` (binary write-back); transfer scan dst slots |
| Key items max slots | 128 | `StorageKeyCount` | layout offsets (`StorageNextEquipIdxRel`); runtime model does not expose |
| Read count (linear scan) | 2048 | `StorageItemCount` | `ReadStorage` argument; overshoot vs common — see "Container entries" callout |
| Bytes per record | 12 | `InvRecordLen` | `ReadStorage`, transfer, allocator |
| Safety margin | 0x6000 | `StorageSafetyMarg` | `mapInventory` validation |

### Trailing counters

| Counter | Type | Location | Reconcile on load? |
|---|---|---|---|
| `storage_count` header | u32 | `slot.StorageBoxOffset` (first value) | no on load; yes after batch add / transfer (`ReconcileStorageHeader`) |
| `key_count` header | u32 | `storageStart + StorageCommonCount × 12` | no (parser does not read directly; the overshoot in `ReadStorage` may bypass it) |
| `NextEquipIndex` | u32 | `storageStart + StorageNextEquipIdxRel` | no (storage has no reconcile in `mapInventory`) |
| `NextAcquisitionSortId` | u32 | `storageStart + StorageNextAcqSortRel` | no |

### UI capacity gap (callout)

> ⚠️ `STORAGE used/max` in the UI (`app_deploy.go::SlotCapacity` → `frontend/src/App.tsx:531`) reports `non_empty / StorageCommonCount` — that is, **container** capacity (1920 common slots). This is **not** the same as actual free allocator space for new weapons/AoW on the `slot.GaItems` side. Container capacity (1920 slots) and GaItems allocator capacity (5118/5120 with zone-aware cursors) are **different dimensions**. Full explanation and pathological example: [35 → UI counters vs allocator capacity](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity).

---

## Validation relationships

Full list of invariants and their enforcement by `ValidatePostMutation` — [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants). Storage summary:

- `storage_count` header == number of non-empty records in `Storage.CommonItems` on the binary side (validator check `storage_count`, `diagnostics.go:434-465`); repair: `ReconcileStorageHeader`.
- No orphan GaItem entries on the storage side — repair: `RepairOrphanedGaItems` (scans **both** containers: inventory and storage).
- The `duplicate_index` validator check in the current code scans **only** `Inventory.CommonItems ∪ KeyItems` (`diagnostics.go:392-419`) — it **does not** check Index duplicates in `Storage.CommonItems`. Whether the game requires Index uniqueness in storage the same way as in inventory — `needs verification`.

---

## Examples

### Storage entry → handle → GaMap → ItemID

```
slot.Storage.CommonItems[7]:
    GaItemHandle = 0xA0000005    ← talisman handle, prefix 0xA0
    Quantity     = 1
    Index        = 250

slot.GaMap[0xA0000005]   = 0x20000005    ← talisman ItemID (after HandleToItemID = (handle & 0x0FFFFFFF) | 0x20000000)
slot.GaItems[?].Handle   = 0xA0000005    ← 8-byte record {Handle, ItemID}

db.GetItemDataFuzzy(0x20000005)          → e.g. any talisman from the DB (concrete ItemIDs in backend/db/data/talismans.go)
```

### Storage entry vs Inventory entry — duplicate goods

Stackable goods (`0xB0`) may be **physically present in both** containers with **the same handle** (legal configuration after batch add with `invQty > 0 && storageQty > 0`):

```
slot.Inventory.CommonItems[42]:
    GaItemHandle = 0xB0001234
    Quantity     = 50

slot.Storage.CommonItems[3]:
    GaItemHandle = 0xB0001234        ← THE SAME handle
    Quantity     = 99
```

Both sides use the same `GaItems[?]` via the shared handle. Quantity-merge transfer ([53](53-inventory-storage-transfer.md)) operates on both records independently, but the handle remains shared. In contrast to instance-move (weapon/armor/talisman/AoW), where a duplicate handle on transfer triggers a rehandle (allocation of a new handle for the moved instance).

---

## Verified write contracts (native save evidence)

Native-save laboratory tests establish the following contracts for a
genuinely **empty** Storage — no `Storage.CommonItems` records, fresh
signature (`NextAcquisitionSortId<=1`, `NextEquipIndex==0`). See
[43 → Verified native save-write evidence](43-transactional-item-adding.md#verified-native-save-write-evidence)
for the pipeline-level (App-lifecycle) evidence this pairs with, and
[07 → Verified write contracts](07-inventory.md#verified-write-contracts-native-save-evidence)
for the contrasting Inventory-side rules.

- **T310**: the first direct-add record into that empty Storage initializes at
  `Index=2`, `NextAcquisitionSortId=2`, `NextEquipIndex=128`.
- **T330**: one native batch of six records from that exact same starting
  state uses indices `2, 4, 6, 8, 10, 12` and ends at `NextEquipIndex=133`,
  `NextAcquisitionSortId=7` (`128+5`, `2+5`). Only within a batch that started
  with the T310 signature does every record — not just the first — advance
  both counters by exactly 1.
- **T352**: this empty-start context also applies across **separate** direct
  Database Add calls (multiple `AddItemsToCharacter` invocations, not one
  batch) made within one uninterrupted editing session — not just within a
  single batch. The mechanism that carries this context between calls
  (explicit `App` state, reset at save/load/close) is documented in
  [43 → Verified native save-write evidence](43-transactional-item-adding.md#verified-native-save-write-evidence);
  this chapter states only the counter values the context preserves.

**Already-populated Storage counter behavior is not established by
T310/T330/T352.** The writer's current `default` branch (in
`addToInventory`'s Storage counter switch) — which leaves `NextEquipIndex`
untouched and advances `NextAcquisitionSortId` as a high-water mark — is the
existing fallback implementation, unmodified by this evidence. It is **not**
asserted here as a verified native-save contract; changing it would require
separate native-save evidence and a regression test.

---

## Known limits / needs verification

- **Storage Apply (UI flow Reorder Storage)** — an in-game sanity check on Steam Deck is not freshly confirmed on the current branch. The backend (`ReorderStorage` in `app_inventory_order.go`, stride-2 algorithm) is covered by tests (`app_storage_order_test.go`), but in-game verification of "Acquisition Order ↑" in the box matching the editor's preview for each Sort Order tab — `needs verification` (see also [53 → Known limits](53-inventory-storage-transfer.md#g-znane-ograniczenia--future-work)).
- **`StorageItemCount = 2048` vs `StorageCommonCount = 1920` overshoot** — `ReadStorage` reads 2048 records linearly, i.e. passes through the 4-byte `key_count` header (at offset = 1920 × 12 = 23040) and may fall into the key items section. In practice the empty filter removes most false reads, but the formal semantics — `needs verification`. Possibly this is historical, works "by accident" on typical saves.
- **Storage key items** — `ReadStorage` always initializes `KeyItems = []InventoryItem{}` (empty slice). The Save Forge runtime model **does not expose** storage key items for transfer, reorder or UI. Whether the game uses storage key items actively — `needs verification`.
- **`duplicate_index` validator for storage** — in the current code the validator checks only `Inventory.CommonItems ∪ KeyItems` (see [35 → Required invariants I5](35-gaitem-allocator-invariants.md#required-invariants)). Whether the game requires Index uniqueness in storage — `needs verification`.
- **Storage counter reconcile on load** — the `mapInventory` storage branch does not reconcile `NextAcquisitionSortId`/`NextEquipIndex` (unlike Inventory). Values from saves edited by external tools may remain stale until the first storage mutation. `needs verification` whether this causes observable issues in the game.

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — GaItem/GaMap model, handle prefixes, storage → GaMap → GaItems relation.
- [07-inventory](07-inventory.md) — Inventory data model (`InventoryItem` 12 B record, identical to Storage; full comparison of Inventory vs Storage differences in this chapter's "Mental model").
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator, capacity, counters, validation, snapshot/rollback, UI capacity gap.
- [43-transactional-item-adding](43-transactional-item-adding.md) — full Add Items architecture (Storage Qty branch is part of the same pipeline).
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — `acquisition_index` in `InventoryItem.Index`; `ReorderStorage` uses an algorithm identical to `ReorderInventory`.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — full semantics of Inventory ↔ Storage transfer (instance-move vs quantity-merge, rehandle, equipped guard, cap-aware partial, two-column Sort Order UI).
- [54-ash-of-war](54-ash-of-war.md) — semantics of custom AoW on transfer of a weapon with an AoW handle to/from storage.

---

## Sources

- `backend/core/structures.go` — `EquipInventoryData`, `InventoryItem`, `(*EquipInventoryData).ReadStorage`, `(*SaveSlot).mapInventory` (storage branch)
- `backend/core/offset_defs.go` — `StorageItemCount`, `StorageCommonCount`, `StorageKeyCount`, `StorageHeaderSkip`, `StorageSafetyMarg`, `StorageNextEquipIdxRel`, `StorageNextAcqSortRel`, `InvRecordLen`, `InvKeyCountHeader`
- `backend/core/writer.go` — `ReconcileStorageHeader`, `RepairOrphanedGaItems`
- `backend/core/transfer.go` — `MoveItemsBetweenContainers` (storage-aware transfer + rehandle)
- `backend/core/diagnostics.go` — `ValidatePostMutation` (storage_count check)
- `app.go::AddItemsToCharacter` (Storage Qty branch), `app.go::MoveItemsBetweenInventoryAndStorage`
- `app_inventory_order.go::ReorderStorage`, `GetStorageOrder`
- `app_deploy.go::SlotCapacity`, `GetSlotCapacity`
- Tests: `tests/transfer_test.go`, `app_storage_order_test.go`
