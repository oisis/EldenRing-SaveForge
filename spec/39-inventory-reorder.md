# 39 — Inventory Custom Order (Drag & Drop Grid)

> **Type**: Design doc
> **Extracted from**: ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planned — blocked on Phase 0 in-game verification

---

## Goal

Allow the player to choose sort mode (Acquisition / Alphabetical / Item Type / Weight / Attack Power / Upgrade) and arrange items in **any custom drag & drop order** in an in-game-style grid view. Starting scope: **weapons + armor + talismans** (non-stackable). Shields + AoW + Ashes — optional follow-up.

**Unique feature** — neither `er-save-manager` nor `ER-Save-Editor/Rust` offer this.

---

## Mechanics (verified in research, BEFORE coding requires Phase 0 in-game test)

- Save has `acquisition_index` field (offset `0x08` in 12-byte `InventoryItem`, mapped to `core.InventoryItem.Index`)
- This is a **global counter** incremented at each pickup (`next_acquisition_sort_id` at section end)
- **This field controls "Acquisition Order" sorting** in-game. Custom order can ONLY be set by manipulating `acquisition_index`
- Other in-game sorts (Item Type / Weight / Attack Power / Alphabetical) are **computed at runtime** from `regulation.bin` params (`EquipParamWeapon.sortGroupId`/`sortId`, etc.) — NOT in save, editor cannot change them
- Consequence: our custom order is visible only when player has sorting set to "Acquisition Order" (default after fresh load)

---

## Pitfalls

- **Reserved index range 0-432** (`InvEquipReservedMax`) — reserved for equipment slots. Custom order MUST use `Index >= 433` — best: `base = max(NextAcquisitionSortId, 1000)` as buffer
- **Reorder per category** — game shows sub-tabs (Tools / Melee / Shields / etc.), sort is per-tab. `sortGroupId` defines group. Weapon reorder doesn't affect talismans
- **Stackables** (consumables, materials, AoW) — game may group differently; starting scope limited to non-stackable equipment where `acquisition_index` mechanics are verified
- **Missing data for other sort modes** — `ItemData` lacks `Weight` / `AttackPower` / `SortGroupId`. Need to import from `tmp/erdb/1.10.0/EquipParam*.csv` via `scripts/import_erdb.go`

---

## Phase 0 — Verify in-game (CRITICAL, 1-2h)

Test:
1. Save with 5 weapons in slots
2. Hex edit `acquisition_index` of 5 weapons → 1000, 1001, 1002, 1003, 1004
3. Steam Deck deploy → load save
4. In-game: switch sorting to Acquisition Order
5. Verify weapons are in order [1000..1004] ascending
6. Pickup new item in-game → verify our 5 weapons keep their order (new item at end as `next_acquisition_sort_id`)

**If not confirmed → ABORT entire feature** (hypothesis about `acquisition_index` is wrong).

---

## Phase 1 — Backend reorder API + 2 sort modes (3-5h)

```go
// Returns ordered handle list per category for current state.
func (a *App) GetInventoryOrder(charIdx int, category string) ([]uint32, error)

// Sets new acquisition_index per handle in given order.
// Indices: base, base+1, base+2... where base = max(NextAcquisitionSortId, 1000)
// Updates next_acquisition_sort_id counter on completion.
func (a *App) ReorderInventory(charIdx int, category string, orderedHandles []uint32) error

// Bulk sort by mode. Phase 1: "acquisition" | "alphabetical".
// Phase 2 adds: "weight" | "attackPower" | "sortGroupId" | "upgradeLevel".
func (a *App) SortInventory(charIdx int, category string, sortMode string) error
```

**Categories (Phase 1 scope)**: `"melee_armaments"`, `"head"`, `"chest"`, `"arms"`, `"legs"`, `"talismans"`.

**Implementation `ReorderInventory`:**
1. `pushUndo(charIdx)`
2. Validate: every handle exists in `slot.Inventory.CommonItems`, belongs to given category
3. Reserve range: `base = max(slot.Inventory.NextAcquisitionSortId, 1000)`
4. Per handle: find slot in `CommonItems`, set `Index = base + i`, write-back via `SlotAccessor.WriteU32` at `commonStart + slotIdx*12 + 8`
5. Update `slot.Inventory.NextAcquisitionSortId = base + len(orderedHandles)`, write-back at `nextAcqSortIdOff`

**Tests:**
- Reorder 5 weapons → reload save → order matches
- Reorder respects reserved range (Index >= 433 always)
- Mixing categories: weapon reorder doesn't change talisman indices
- `next_acquisition_sort_id` correctly updated
- Round-trip: Save → Reorder → Write → Read → indices match

---

## Phase 2 — Import data for remaining sort modes (2-3h)

Import from `tmp/erdb/1.10.0/EquipParam*.csv`:
- `weight` (weapons + armor) → `ItemData.Weight float32`
- `attackBasePhysics` (weapons) → `ItemData.AttackPower uint32`
- `sortGroupId` (weapons + armor) → `ItemData.SortGroupId uint32`

Test: `SortInventory(weight)` → top 10 weapons must match manual in-game sort.

---

## Phase 3 — In-game-style grid UI + drag & drop (8-12h)

**New component**: `InventoryGrid.tsx` — toggle List view / Grid view in `InventoryTab.tsx`.

**DnD library**: `@dnd-kit/core` + `@dnd-kit/sortable` + `@dnd-kit/utilities` (~30KB combined)

**Visual details (in-game look):**
- Background: `bg-zinc-900` with grain pattern
- Cell: `64x64px`, golden border on selected/hovered
- Icon: 56x56 centered, quantity badge (bottom-right), upgrade badge (bottom-left)
- Drag preview: semi-transparent copy, drop indicator: golden vertical line
- Empty cells at grid end for visual consistency

**Interactions:**
- Click → preview in `ItemDetailPanel`
- Drag → reorder
- Right-click / long-press → context menu (Remove / Set Quantity / Upgrade)

**State flow:**
```ts
const [sortMode, setSortMode] = useState<SortMode>('acquisition');
const [items, setItems] = useState<ItemViewModel[]>([]);
// On sortMode change → SortInventory(charIdx, cat, sortMode), refresh
// On drag end → optimistic local reorder + ReorderInventory(charIdx, cat, newHandles)
// "Reset to default" → SortInventory(charIdx, cat, 'sortGroupId')
```

---

## Phase 4 (optional) — Per-character preset persistence (2-3h)

Integration with Character Preset Export/Import (spec/37):
- Add `inventoryOrder: { weapons: [handle1, ...], armor: [...], talismans: [...] }` to `CharacterPreset`
- Apply: after `AddItemsToCharacter` → call `ReorderInventory` per category

---

## Phase summary

| Phase | Effort |
|---|---|
| **0** Verify in-game (critical!) | 1-2h |
| **1** Backend API + 2 sort modes | 3-5h |
| **2** Import erdb + 4 sort modes | 2-3h |
| **3** Grid UI + drag & drop | 8-12h |
| 4 (opt.) Preset persistence | 2-3h |
| **Total minimum (0+1+3)** | **12-19h** |
| **Total full (0-3)** | **14-22h** |

---

## Open questions

1. **Phase 0 verification**: user prepares save with hex-edit, or editor produces test save?
2. **Category scope**: weapons + armor + talismans only, or also shields / AoW / Ashes / consumables?
3. **Reset button behavior**: return to `sortGroupId` (in-game default), `acquisition` (original pickup order), or disable after drag?
4. **Persistence after WriteSave**: keep custom order forever (until Reset), or invalidate after Add Items?
5. **Reserved slots 0-432**: our reorder skips them (recommended) or re-indexes? Suggest: don't touch `Index <= 432` — those are physically equipped items controlled by game via separate offsets.
