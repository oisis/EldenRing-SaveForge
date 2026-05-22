# 27 — Map Reveal (4-layer model)

> **Type**: Binary format spec + endpoint reference
> **Status**: ✅ canonical (Phase 4 Step 2 — 2026-05-21)
> **Scope**: Master chapter for map reveal. Defines the 4-layer model (regions, detailed bitmap, DLC cover layer, fog of war) and all `App.*Map*` / `App.*FogOfWar` endpoints in SaveForge. Domain details (UnlockedRegions layout, DLC tile coords, event flag bit-order) are in derived chapters — see §18 Cross-references.

---

## 1. Chapter purpose

The map in Elden Ring is a **stack of four independent layers** — each controlled by a different mechanism in the save file. To "reveal the map for the player" requires modifying at least layers 1 and 2; layer 0 (regions) is responsible for fast travel; layer 3 (fog of war) is cosmetic and exposed separately.

This chapter describes:

- the 4-layer mental model (L0–L3)
- what each layer does and where it is stored
- all `App.*Map*` / `App.*FogOfWar` endpoints in `app_world.go`
- the write path of each layer + rollback caveats
- ban-risk Tier 1 (UI integration)
- test coverage + `needs verification` items

This chapter does **not** duplicate:

- event flags bit/byte indexing → [15](15-event-flags.md)
- `UnlockedRegions` array binary layout → [11](11-regions.md)
- DLC tile coordinate detail → [29](29-dlc-black-tiles.md)
- companion item / fragment add-pickup semantics → [50](50-item-companion-flags.md)
- full slot rebuild research → `30-slot-rebuild-research.md`

---

## 2. Status

| Layer | Implementation | Test coverage | In-game verification |
|---|---|---|---|
| L0 — UnlockedRegions | ✅ `core.SetUnlockedRegions` + `RebuildSlot` | ✅ 4 tests in `writer_regions_test.go` (PS4/PC, in-memory, after-add-item) | ✅ teleport adds region 6101000 |
| L1 — Detailed Bitmap | ✅ `revealBaseMap` + `revealDLCMap` Phase 1+2 | ✅ `TestMapFlagsRoundtrip` + `TestGetAllMapEntries` | ✅ map texture visible after revealAll |
| L2 — DLC Cover Layer | ✅ `revealDLCMap` Phase 3 (synthetic coords) | ⚠️ covered by offset round-trip, no dedicated test | ⚠️ black tiles removed (DLC fix Apr 2026) |
| L3 — Fog of War | ✅ `RemoveFogOfWar` (fill 0xFF) | ⚠️ covered by slot round-trip, no dedicated test | ✅ gray overlay removed |
| Reset | ✅ `ResetMapExploration` (clear L1 + items, preserve L0 + system flags) | ⚠️ no dedicated test | ⚠️ `needs verification` after a full reset+save+reload |

**Multi-phase atomic rollback**: ❌ none — Phase 1 (flags) and Phase 2 (item add) in `revealBaseMap` / `revealDLCMap` are not atomic. If `AddItemsToSlot` returns an error after SetEventFlag, the flags remain in the save. The UI has a save-level snapshot-undo (`pushUndo` before `RevealAllMap`).

**Last verification**: 2026-05-21 on `tmp/save/ER0000.sl2` (PC, 5 slots) + `tmp/save/oisis_pl-org.txt` (PS4) — round-trip + flag toggle tests.

---

## 3. Source of truth in code

| Component | File / lines | Note |
|---|---|---|
| `GetMapProgress` | `app_world.go:955` | Returns `[]db.MapEntry` for the UI (status `enabled` per flag) |
| `SetMapRegionFlags` | `app_world.go:982` | Set/clear a single visible flag + add/remove the map fragment item |
| `SetMapFlag` | `app_world.go:1017` | Set/clear a raw flag (system, visible, unsafe) — no item side effect |
| `RevealAllMap` | `app_world.go:1041` | Composite: `pushUndo` + `revealBaseMap` + `revealDLCMap` |
| `revealBaseMap` (internal) | `app_world.go:1067` | Phase 1 flags (MapSystem + MapVisible non-DLC) + Phase 2 items |
| `revealDLCMap` (internal) | `app_world.go:1094` | Phase 1 DLC flags (62002, 82002) + DLC visible flags + Phase 2 items + Phase 3 cover layer coords |
| `ResetMapExploration` | `app_world.go:1162` | Clear visible flags + remove fragment items + clear MapAcquired + clear MapUnsafe; preserve system flags |
| `RemoveFogOfWar` | `app_world.go:1201` | Fill the bitfield `afterRegs+0x087E..afterRegs+0x10B0` with `0xFF` |
| Map data | `backend/db/data/maps.go` | `MapSystem` (4 entries), `MapVisible` (263 entries), `MapUnsafe` (8 entries), `MapFragmentItems` (24 entries), `MapAcquired` (24 entries — unused; the game clears them) |
| DLC detection | `backend/db/data/maps.go::IsDLCMapFlag` | Range `62080–62084` OR `62800–62999` |
| Region writer | `backend/core/writer.go::SetUnlockedRegions` | Dedupe + sort + `RebuildSlot` + rollback |
| Slot rebuilder | `backend/core/slot_rebuild.go::RebuildSlot` | Full sequential serializer |
| DLC tile constants | `backend/core/offset_defs.go:309–322` | `DLCTileZeroStart/End`, `DLCTileRec1X/Y/Flag`, `DLCTileRec2X/Y/Z/W/Flag` |
| `afterRegs` helper | `app_world.go::resolveAfterRegs` | `storageEnd + DynStorageToGestures + 4 + regCount*4` |
| Tests | `tests/map_flags_test.go`, `backend/core/writer_regions_test.go` | See §16 |
| Frontend | `frontend/src/components/WorldTab.tsx` | Map exploration accordion + `RiskActionButton riskKey="map_reveal_full"` (Tier 1) |

---

## 4. Mental model

```
       SAVE SLOT
┌──────────────────────────────────────────────────────────────────┐
│  ... slot header ...                                             │
│  StorageBox                                                      │
│  Gestures count + Regions array                ← L0 (region ids) │
│  afterRegs                                                       │
│    + 0x0088..0x0110  DLC tile coords           ← L2 (cover layer)│
│    + 0x087E..0x10B0  Fog of War bitfield       ← L3 (FoW overlay)│
│  ... MenuProfile ...                                             │
│  PreEventFlagsScalars                                            │
│  EventFlagsBlock (0x1BF99F bytes)               ← L1 (visible)   │
│    flags 62xxx visible, 62000/01/82001/02 system                 │
│  ... other sections ...                                          │
│  Inventory                                                       │
│    Map Fragment items 0x40002198..0x401EA61C    ← L1 (items)     │
└──────────────────────────────────────────────────────────────────┘
```

The four layers are **independent**:

- Setting a region ID (L0) **does not reveal** the texture — see verification log §17 test 1.
- Setting a visible flag (L1) without a map fragment item reveals the texture, but the UI does not show the fragment — §17 test 7.
- Setting a DLC visible flag (L1) without the Phase 3 cover layer coords (L2) → the texture is visible, but black tiles still cover the area — §17 test 8.
- Fog of war (L3) is purely cosmetic — it does not unlock content.

`RevealAllMap` in the app handles L0 (indirectly via teleport later) + L1 + L2 — it does **not** touch L3 (FoW). The UI uses `RemoveFogOfWar` as a separate action.

---

## 5. Map reveal layers — overview

| L | Layer | Storage | Endpoint | Master detail |
|---|---|---|---|---|
| 0 | Unlocked Regions | `slot.UnlockedRegions []u32` (variable-length) | `core.SetUnlockedRegions`, `App.SetRegionUnlocked`, `App.BulkSetUnlockedRegions` | [11 — Regions](11-regions.md) |
| 1 | Detailed Bitmap | Event flags 62xxx (visible) + 4 system flags + Map Fragment items in inventory | `App.SetMapRegionFlags`, `App.SetMapFlag`, `App.RevealAllMap` (composite) | [15 — Event Flags](15-event-flags.md) (helper), [50 — Item Companion Flags](50-item-companion-flags.md) (fragment items) |
| 2 | DLC Cover Layer | 8 floats + 2 flags in BloodStain (afterRegs + `DLCTileRec*`) | `revealDLCMap` Phase 3 (internal) | [29 — DLC Black Tiles](29-dlc-black-tiles.md) |
| 3 | Fog of War | Bitfield `afterRegs+0x087E..+0x10B0` (2099 B) in the BloodStain→MenuProfile gap | `App.RemoveFogOfWar` | (master here — §9) |

---

## 6. L0 — Unlocked Regions

L0 controls **fast travel between Sites of Grace within a region** + multiplayer matchmaking. **It has no effect on map texture visibility.**

### 6.1 Edit path

```go
err := core.SetUnlockedRegions(slot, []uint32{...})
```

- Dedupe + sort ascending → `RebuildSlot` (full rebuild of the 0x280000-B slot) → recalculate dynamic offsets → SectionMap refresh → rollback on error.
- App-level methods: `SetRegionUnlocked(slotIdx, regionID, unlocked)`, `BulkSetUnlockedRegions(slotIdx, ids)`, `GetUnlockedRegions(slotIdx)`.

### 6.2 Number of regions

`backend/db/data/regions.go` contains 104 entries: 62 overworld base + 7 DLC overworld + 35 legacy dungeons (ranges: 6100000–6899999 base, 6900000–6999999 DLC, 1000000–1999999 dungeons). A fresh save has 6 region IDs; "Unlock All" sets ~211 base+DLC IDs.

### 6.3 Region IDs vs map visibility

Region IDs do **not** reveal the map texture nor remove the fog of war. See §17 test 1: adding a region_id alone (without flag 62xxx) → the map texture is unchanged.

### 6.4 Layout details

The full list of region IDs, binary layout, `RebuildSlot` pipeline → [11 — Regions](11-regions.md).

---

## 7. L1 — Detailed Bitmap (event flags + map fragments)

The main "map reveal" mechanism. Implemented in `revealBaseMap` + `revealDLCMap` (Phase 1+2).

### 7.1 System flags (`MapSystem`)

```go
var MapSystem = map[uint32]MapRegionData{
    62000: {Name: "Allow Map Display", Area: "System"},
    62001: {Name: "Allow Underground Map Display", Area: "System"},
    82001: {Name: "Show Underground", Area: "System"},
    82002: {Name: "Show Shadow Realm Map", Area: "System"},
}
```

Without these flags the in-game map UI may stay empty — a single region's visibility flag is not enough if the system display is off.

⚠️ Note: `revealBaseMap` iterates over **all** `MapSystem` entries (including `82002` Shadow Realm), without filtering. `revealDLCMap` additionally sets **`62002`** (Allow Shadow Realm Map Display) inline — which is **not** in the `MapSystem` map. If you want to reveal only the base game without the DLC display, it is better to use `App.SetMapFlag` per-ID instead of `RevealAllMap`. `needs verification` whether this layout is intentional (the code treats 62002 as DLC-only, but 82002 as system).

### 7.2 Visibility flags (`MapVisible`)

263 entries in `MapVisible` (219 non-DLC + 44 DLC). Each 62xxx flag reveals the region's texture in the map UI. Ranges:

| Range | Area |
|---|---|
| 62010–62012 | Limgrave |
| 62020–62022 | Liurnia |
| 62030–62032 | Altus Plateau |
| 62040–62041 | Caelid |
| 62050–62052 | Mountaintops / Snowfield |
| 62060–62064 | Underground |
| 62080–62084 | DLC overworld |
| 62100–62799 | Dungeon maps (base game) |
| 62800–62999 | Dungeon maps (DLC) |

Full list: `MapVisible` in `maps.go`. DLC detection: `IsDLCMapFlag(id)` returns true for `62080–62084` OR `62800–62999`.

### 7.3 Map Fragment items

24 entries in `MapFragmentItems` (19 base + 5 DLC). Each overworld visible flag (`62010–62064`, `62080–62084`) has a paired item:

```go
var MapFragmentItems = map[uint32]uint32{
    62010: 0x40002198, // Limgrave, West
    ...
    62084: 0x401EA61C, // Abyss (DLC)
}
```

Range: base game `0x40002198..0x400021AA`, DLC `0x401EA618..0x401EA61C`.

**Why we add item + flag**: in a normal playthrough the player picks up the map fragment, the game sets the flag + adds the item. The editor replicates both for consistency (if you set only the flag, the player sees the texture but the fragment is missing from the inventory — see §17 test 7).

### 7.4 Map Acquired (63xxx) — NOT used

`MapAcquired` maps 63xxx flags (offset = `visible+1000`). These are **transient pickup-popup trigger flags** — the game sets them to display the "Map Fragment acquired" toast, then clears them. **The editor intentionally does not set them.** `ResetMapExploration` clears them (defensive) but never sets them.

### 7.5 MapUnsafe sub-region flags

8 entries in `MapUnsafe` (`62004–62009, 62053, 62065`) — sub-region flags that, when set manually, can cause black tiles in the UI (Cover Layer corruption). Excluded from `RevealAllMap`. Exposed in the UI (as a known list) but with a warning.

### 7.6 `revealBaseMap` / `revealDLCMap` algorithm

```text
revealBaseMap(slot):
    flags = slot.Data[slot.EventFlagsOffset:]

    # Phase 1 — flags (slot.Data not shifted)
    for id in MapSystem:                         SetEventFlag(flags, id, true)
    for id in MapVisible where !IsDLCMapFlag(id):
        SetEventFlag(flags, id, true)
        if id in MapFragmentItems: items.append(MapFragmentItems[id])

    # Phase 2 — add items (shifts slot.Data, flags slice invalidated)
    for itemID in items: AddItemsToSlot(slot, [itemID], 1, 0, false)

revealDLCMap(slot):
    flags = slot.Data[slot.EventFlagsOffset:]

    # Phase 1 — DLC flags
    SetEventFlag(flags, 62002, true)            # Allow Shadow Realm Display
    SetEventFlag(flags, 82002, true)            # Show Shadow Realm Map
    for id in MapVisible where IsDLCMapFlag(id):
        SetEventFlag(flags, id, true)
        if id in MapFragmentItems: items.append(MapFragmentItems[id])

    # Phase 2 — add DLC items
    for itemID in items: AddItemsToSlot(slot, [itemID], 1, 0, false)

    # Phase 3 — cover layer coords (see §8 and 29)
```

⚠️ **Order matters**: `AddItemsToSlot` shifts bytes inside the slot, which invalidates the previously computed `flags` slice. Phase 1 must finish **before** Phase 2.

### 7.7 Cross-reference to event flags

Event flag bit/byte indexing, the helper API (`db.SetEventFlag`/`GetEventFlag`), the 3-tier resolver, BST → [15 — Event Flags](15-event-flags.md).

---

## 8. L2 — DLC Cover Layer

The DLC map texture remains covered by "black tiles" until the player reveals the area in-game. The flags 62080–62084 alone are not enough — the game requires **synthetic coordinates** in the BloodStain section pointing at the DLC area as revealed.

### 8.1 Write path (Phase 3 in `revealDLCMap`)

```go
afterRegs := resolveAfterRegs(slot)             // = storageEnd + DynStorageToGestures + 4 + regCount*4

// Zero out the range
for i := afterRegs+DLCTileZeroStart; i < afterRegs+DLCTileZeroEnd; i++ {
    slot.Data[i] = 0x00
}

// Record 1: DLC center coords
putF32(slot.Data, afterRegs+DLCTileRec1X, 9648.0)
putF32(slot.Data, afterRegs+DLCTileRec1Y, 9124.0)
slot.Data[afterRegs+DLCTileRec1Flag] = 0x01

// Record 2: DLC area anchor
putF32(slot.Data, afterRegs+DLCTileRec2X, 3037.0)
putF32(slot.Data, afterRegs+DLCTileRec2Y, 1869.0)
putF32(slot.Data, afterRegs+DLCTileRec2Z, 7880.0)
putF32(slot.Data, afterRegs+DLCTileRec2W, 7803.0)
slot.Data[afterRegs+DLCTileRec2Flag] = 0x01
```

### 8.2 Offset constants

```go
// backend/core/offset_defs.go:309–322
DLCTileZeroStart = 0x0088   // start of range to zero out
DLCTileZeroEnd   = 0x0110   // end (exclusive)

DLCTileRec1X    = 0x008D    // f32 X
DLCTileRec1Y    = 0x0091    // f32 Y
DLCTileRec1Flag = 0x0095    // u8 visited

DLCTileRec2X    = 0x00C5    // f32 X
DLCTileRec2Y    = 0x00C9    // f32 Y
DLCTileRec2Z    = 0x00CD    // f32 Z
DLCTileRec2W    = 0x00D1    // f32 W
DLCTileRec2Flag = 0x00D5    // u8 visited
```

f32 format: little-endian (`binary.LittleEndian.PutUint32(math.Float32bits(v))`).

### 8.3 Synthetic coords — values

The values `9648, 9124` and `3037, 1869, 7880, 7803` are synthetic (not from the game) — they mean "the DLC area is revealed at the central + anchor level". `needs verification` whether these values still work for the latest game patch (last verification: Apr 2026 DLC fix).

### 8.4 Layout detail + research log

The full Cover Layer analysis, the investigation log, the alternatives tested → [29 — DLC Black Tiles](29-dlc-black-tiles.md).

---

## 9. L3 — Fog of War (`RemoveFogOfWar`)

A dense bitmask between BloodStain and MenuProfile representing the per-tile exploration state. The editor exposes a dedicated action that **fills the entire range with the value `0xFF`** — i.e., "everything revealed". Selective per-tile clearing is **not** implemented (it requires reverse-engineering the bit-to-tile mapping — see §17 L4).

### 9.1 Location

```text
afterRegs        = storageEnd + DynStorageToGestures + 4 + regCount*4
bitfield_start   = afterRegs + 0x087E
bitfield_end     = afterRegs + 0x10B0  (inclusive last safe byte)
usable_range     = 2099 bytes (0x087E .. 0x10B0)
section_size     = 0x103C bytes total (BloodStain → MenuProfile)
```

⚠️ **Critical**: writing past `+0x10B0` overlaps `MenuProfile` and **crashes the game**. The prefix `+0x0000..+0x087D` contains structured horse + bloodstain data — do not touch it from this layer.

### 9.2 Bitfield format

A flat bitmask, LSB-first within each byte. `1` = tile revealed, `0` = hidden. The bit-to-tile mapping is **unknown** and cannot be inferred from the region ID. A single in-game teleport flips ~356 bits in a contiguous 157-byte window (see §17 test 6).

### 9.3 Algorithm

```go
func (a *App) RemoveFogOfWar(slotIndex int) error {
    ...
    storageEnd  := slot.StorageBoxOffset + core.DynStorageBox
    gesturesOff := storageEnd + core.DynStorageToGestures
    regCount    := int(binary.LittleEndian.Uint32(slot.Data[gesturesOff:]))
    afterRegs   := gesturesOff + 4 + regCount*4
    for i := afterRegs + 0x087E; i <= afterRegs+0x10B0; i++ {
        slot.Data[i] = 0xFF
    }
    return nil
}
```

In-place fill, no byte shifting, no offset recalculation. Idempotent — repeated calls leave the same state.

### 9.4 Why separated from `RevealAllMap`

- The Detailed Bitmap (L1) gives the player a **useful signal** — textures, dungeon icons, fragment ownership. This is what most users mean by "show me the map".
- Fog of War (L3) is **purely cosmetic** — it removes the gray "you haven't been here yet" overlay. It does not unlock new content.
- Including L3 in `RevealAllMap` would force the loss of the natural exploration signal. Keeping L3 behind its own action preserves user choice.

Frontend (`WorldTab.tsx:219`):

```ts
const handleRevealAllMap = async () => {
    await RevealAllMap(charIdx);
    await RemoveFogOfWar(charIdx);  // user-facing: one click = L1+L2+L3
    ...
};
```

The UI combines both actions behind the "Reveal All" button with `RiskActionButton riskKey="map_reveal_full"` (Tier 1 ban-risk confirmation). See [32](32-ban-risk-system.md).

### 9.5 Selective FoW removal

❌ Not implemented. It would require a bit-to-tile mapping (FoW bitfield → tile + area + region). See §17 L4 — unknown mapping.

---

## 10. Event flag relation

Layers L1 and L2 use the event flag helper API. Byte/bit indexing convention, the BST resolver, snapshot/rollback semantics → [15 — Event Flags](15-event-flags.md).

Critical SET/CLEAR policy differences (from [15 §11](15-event-flags.md#11-feature-specific-policy-differences)):

| Flag set | Policy | Master |
|---|---|---|
| MapSystem (4 flags) | Symmetric SET/CLEAR; `ResetMapExploration` does **not** clear them (preserve) | (here) |
| MapVisible (62xxx) | Symmetric SET/CLEAR; `ResetMapExploration` clears all | (here) |
| MapUnsafe (sub-region) | Symmetric SET/CLEAR; excluded from RevealAll, but cleared by ResetMap | (here) |
| MapAcquired (63xxx) | SET by the game only as a trigger; the editor **never** sets them (clears on reset as defensive) | (here) |
| Map Fragment items (`MapFragmentItems`) | SET-only in the current code (no CLEAR-on-remove) → this follows from `ResetMapExploration` using `RemoveItemByBaseID` instead of event flag clearing; companion flag policy → see [50](50-item-companion-flags.md) |
| Container pickup flags (66xxx) | SET-only in the current code — see [15 §11](15-event-flags.md#11-feature-specific-policy-differences) and [50](50-item-companion-flags.md) |

---

## 11. Map region data

All data structures in `backend/db/data/maps.go`:

| Structure | Entry count | Purpose |
|---|---|---|
| `MapSystem` | 4 | Global "enable map display" flags (62000, 62001, 82001, 82002) |
| `MapVisible` | 263 (219 non-DLC + 44 DLC) | Region/dungeon visibility flags (62xxx) — overworld + dungeons + DLC |
| `MapUnsafe` | 8 | Sub-region flags causing black tiles (62004–62009, 62053, 62065) |
| `MapFragmentItems` | 24 | Mapping visible flag → map fragment item ID (19 base + 5 DLC) |
| `MapAcquired` | 24 (63xxx trans.) | Pickup trigger flags — **the editor does not set them**, `ResetMapExploration` clears them (defensive) |

DLC detection: `IsDLCMapFlag(id)` — `62080–62084` (overworld) or `62800–62999` (dungeons).

⚠️ **`MapVisible` 263 entries is a snapshot from 2026-05-21** — generated offline from regulation. After a game patch update a re-extraction is required. `needs verification` after a new DLC.

---

## 12. Map fragment companion behavior

Adding map fragment items in Phase 2 of `revealBaseMap`/`revealDLCMap` uses `core.AddItemsToSlot` — not `App.AddItem` nor `App.AddItemsToCharacter`. Consequences:

- **Item-level companion flags** (e.g., for the Spectral Steed Whistle) are **not** automatically set — `AddItemsToSlot` is a low-level path.
- `MapFragmentItems` (24 fragments) have **no** companion flag set in `itemCompanionEventFlags` — the fragment loads normally via the visible flag (L1.1), with no side effects.
- `ResetMapExploration` uses `RemoveItemByBaseID` to remove fragments — with no CLEAR-on-remove for the MapAcquired event flags (63xxx). This behavior follows from the MapAcquired flags being transient in the game.

Companion flag semantics (item-level vs grace-level) → [50 — Item Companion Flags](50-item-companion-flags.md) (master).

---

## 13. Current implemented endpoints

| Endpoint | Signature | Purpose |
|---|---|---|
| `GetMapProgress(slotIdx)` | `([]db.MapEntry, error)` | Read all flags status for the UI (MapEntry with `enabled bool`) |
| `SetMapRegionFlags(slotIdx, visibleFlagID, enabled)` | `error` | Toggle a single visible flag + add/remove the map fragment item |
| `SetMapFlag(slotIdx, flagID, enabled)` | `error` | Toggle a raw flag (system/visible/unsafe) — no item side effect |
| `RevealAllMap(slotIdx)` | `error` | Composite: `pushUndo` + `revealBaseMap` + `revealDLCMap` (L1 + L2) |
| `ResetMapExploration(slotIdx)` | `error` | Clear visible + remove fragments + clear MapAcquired + clear MapUnsafe; preserve MapSystem |
| `RemoveFogOfWar(slotIdx)` | `error` | Fill the bitfield 0xFF (L3) |

Helpers (internal, not exposed via Wails):

- `revealBaseMap(slot)` — `app_world.go:1067`
- `revealDLCMap(slot)` — `app_world.go:1094`
- `resolveAfterRegs(slot)` — helper for L2 + L3 offset calc
- `putF32(d, off, v)` — helper for LE f32 write

Frontend: `WorldTab.tsx` (tab list). There is no dedicated `MapTab` — map exploration is an accordion within `WorldTab`.

---

## 14. Write path and rollback caveats

### 14.1 `RevealAllMap` — composite, multi-phase

```text
pushUndo(slotIdx)
  → revealBaseMap(slot)
       Phase 1: SetEventFlag × ~223 (4 MapSystem + 219 MapVisible non-DLC)
       Phase 2: AddItemsToSlot × 19 (base game fragments from MapFragmentItems)
  → revealDLCMap(slot)
       Phase 1: SetEventFlag × ~46 (62002 + 82002 inline + 44 MapVisible DLC)
       Phase 2: AddItemsToSlot × 5 (DLC fragments from MapFragmentItems)
       Phase 3: write 8 floats + 2 flags (cover layer)
```

⚠️ **No atomic rollback within Phase 1/2/3**: if Phase 2 `AddItemsToSlot` returns an error after Phase 1 SetEventFlag, the flags remain in the save. Rollback is available only at the save level (`pushUndo` snapshot before `RevealAllMap`).

### 14.2 `SetUnlockedRegions` — atomic via `RebuildSlot`

`core.SetUnlockedRegions` returns an error if the rebuild fails → the caller (`SetRegionUnlocked` / `BulkSetUnlockedRegions`) rolls back by restoring slot.Data. Atomic within a single region change.

### 14.3 `RemoveFogOfWar` — in-place

No byte shifting, no rebuild. A single-pass byte fill — either the entire range is set to 0xFF (success), or a bounds check fails (error pre-write).

### 14.4 `ResetMapExploration` — best-effort

Iterates over MapVisible × `SetEventFlag(false)` + `RemoveItemByBaseID(itemID)`. No atomicity — an individual flag set may return an error (`SetEventFlag` has a bounds check). The code uses `_ = db.SetEventFlag(...)` (discard error) — best effort, no rollback.

---

## 15. Validation and safety notes

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| V1 | `RevealAllMap` sets 82002 (Shadow Realm) even for a user without DLC | ⚠️ medium | The flag is set with no effect if the DLC is not owned; no crash, but a UI mismatch. `needs verification` whether the game fail-closes the UI without DLC. |
| V2 | `RevealAllMap` `RiskActionButton` Tier 1 — the user must confirm | ⚠️ medium | `WorldTab.tsx:321` with `riskKey="map_reveal_full"`. See [32](32-ban-risk-system.md). |
| V3 | Manually setting MapUnsafe (`62004–62009`, `62053`, `62065`) → black tiles | 🔴 high | `MapUnsafe` is exposed in the UI but excluded from `RevealAllMap`. The UI must filter or warn itself. |
| V4 | DLC tile coords (9648/9124, 3037/1869/7880/7803) hardcoded | ⚠️ medium | `needs verification` after every game patch. The Apr 2026 DLC fix confirmed the values, but there is no automatic diff vs regulation. |
| V5 | FoW write past `+0x10B0` → game crash (overlaps MenuProfile) | 🔴 high | Hardcoded bound in `RemoveFogOfWar`; no runtime check that `afterRegs+0x10B0 < menuProfileOff`. `needs verification` after slot layout changes. |
| V6 | Multi-phase rollback in `revealBaseMap`/`revealDLCMap` is missing | ⚠️ medium | A save-level snapshot (`pushUndo`) is a workaround; there is no per-flag rollback. |
| V7 | Region ID for a DLC region on a non-DLC save | ⚠️ low | The game tolerates it (safe), but it is logically incorrect. The UI could warn — `needs verification` whether it does. |
| V8 | `ResetMapExploration` discards errors (`_ = db.SetEventFlag(...)`) | ⚠️ medium | Best-effort; an individual bounds error does not block the reset. `needs verification` that the reset is complete in practice. |
| V9 | `SetMapRegionFlags` + duplicate item add (idempotency) | ⚠️ low | Map fragment item in inventory: a second add may result in a duplicate (if `AddItemsToSlot` does not dedupe). `needs verification` on a bulk toggle. |

---

## 16. Test coverage

| Test | Purpose | File |
|---|---|---|
| `TestSetUnlockedRegionsInMemory` | L0 in-memory mutation | `backend/core/writer_regions_test.go:11` |
| `TestSetUnlockedRegionsRoundTripPS4` | L0 PS4 round-trip | `backend/core/writer_regions_test.go:48` |
| `TestSetUnlockedRegionsAfterAddItem` | L0 after `AddItemsToSlot` (slot shift) | `backend/core/writer_regions_test.go:106` |
| `TestSetUnlockedRegionsRoundTripPC` | L0 PC round-trip | `backend/core/writer_regions_test.go:158` |
| `TestBSTLookupMatchesEventFlags` | L1 helper API — BST matches precomputed | `tests/map_flags_test.go:12` |
| `TestGetAllMapEntries` | L1 — `GetMapProgress` returns DB entries | `tests/map_flags_test.go:53` |
| `TestMapFlagsRoundtrip` | L1 — flag toggle → save → reload → state preserved | `tests/map_flags_test.go:96` |
| `TestEventFlagsOffsetCorrectness` | L1 — `slot.EventFlagsOffset` points at the bitfield | `tests/map_flags_test.go:145` |

No dedicated test for:

- `RevealAllMap` end-to-end (composite call → save → reload → all DLC + base visible + fragments in inventory).
- `revealDLCMap` Phase 3 coverage layer values (offsets covered by round-trip, but no assert on the values 9648/9124, etc.).
- `RemoveFogOfWar` post-write byte verification (covered by slot round-trip, no dedicated).
- `ResetMapExploration` round-trip (set → save → reload → reset → save → reload → original state).
- PS4↔PC cross-platform bit-equal for L1 (round-trip per-platform, no `TestConvert*`).

---

## 17. Known limits / needs verification

| # | Limit / gap | Status | Note |
|---|---|---|---|
| L1 | Multi-phase atomic rollback | ❌ none | A save-level snapshot is a workaround; per-flag/per-item rollback does not exist. |
| L2 | DLC tile coords stale after a game patch | ⚠️ snapshot Apr 2026 | `needs verification` after every DLC update. |
| L3 | `MapVisible` 263 entries is a snapshot from 2026-05-21 | ⚠️ generated offline | `needs verification` after a regulation update. |
| L4 | Bit-to-tile mapping in the FoW bitfield | ❌ unknown | Would require systematic per-tile diffs in-game; not on the roadmap. |
| L5 | Region IDs `1001000–1001002` in a fresh save | ⚠️ internal markers | Present in every fresh save, but absent from `regions.go`. Probably internal startup. |
| L6 | Structured prefix `+0x0800..+0x087D` in BloodStain | ⚠️ do not overwrite | A repeating pattern `00 00 01 80 BF FF FF FF FF 00...`. Probably coord anchors. |
| L7 | `RevealAllMap` sets 82002 (DLC) regardless of DLC ownership | ⚠️ flag set, no side effect | `needs verification` whether the game fail-closes the UI when the DLC is not owned. |
| L8 | DLC visible flags `62800–62999` (dungeons) | ⚠️ covered by `MapVisible` but no per-flag verification | `needs verification` that all DLC dungeon flags exist in the current patch. |
| L9 | PS4↔PC cross-platform conversion test for map state | ❌ none | Round-trip per-platform is covered (L0 tests), no `TestConvert*` for an L1+L2+L3 cross-platform diff. |
| L10 | Selective FoW removal | ❌ not implemented | Requires L4 (bit-to-tile mapping). |
| L11 | `SetMapRegionFlags` duplicate item-add | ⚠️ `needs verification` | Repeated toggle: does `AddItemsToSlot` dedupe, or duplicate? |
| L12 | `revealBaseMap` Phase 1 iterates ALL `MapSystem` (including 82002) | ⚠️ design choice vs bug | Is setting 82002 (Shadow Realm) without `62002` (DLC display allow) consistent? `needs verification`. |

---

## 18. Cross-references

| Topic | Master chapter |
|---|---|
| Event flag foundation (byte/bit, helper API, BST, bounds check) | [15 — Event Flags](15-event-flags.md) |
| UnlockedRegions binary layout + `RebuildSlot` pipeline | [11 — Regions](11-regions.md) |
| DLC Cover Layer detail (coordinate research, alternative tests) | [29 — DLC Black Tiles](29-dlc-black-tiles.md) |
| Item companion flag policy (SET/CLEAR symmetric vs grace SET-only) | [50 — Item Companion Flags](50-item-companion-flags.md) |
| Sites of Grace activation (related to L1 flags 71xxx-76xxx, not map reveal) | [47 — Sites of Grace](47-site-of-grace-activation.md) |
| World state (FieldArea, WorldArea, WorldGeomMan — read-only, not map) | [16 — World State](16-world-state.md) |
| Game state (LastRestedGrace, ClearCount, GaItem Game Data) | [14 — Game State](14-game-state.md) |
| PvP modular RevealMap module (uses `revealBaseMap` + `revealDLCMap` internally) | [48 — PvP Modular Presets](48-pvp-ready-modular-presets.md) |
| Slot rebuild research (full sequential serializer) | [30 — Slot Rebuild Research](30-slot-rebuild-research.md) |
| Ban-risk Tier 1 UI confirmation (`map_reveal_full`) | [32 — Ban-Risk System](32-ban-risk-system.md) |

---

## 19. Verification log (historical)

Empirical FoW investigation tests that justified the split into independent layers. Kept as reference; methodology details → [99 — Verification Methodology](99-verification-methodology.md).

| # | Test | Result |
|---|---|---|
| 1 | Adding a region_id (without flag 62xxx, without a fragment) | Map texture unchanged (regions ≠ visibility) |
| 2 | 0xFF in the FoW bitfield (small range) + 1 region | Fog removed locally |
| 3 | 0xFF written past `+0x10B0` (overlaps MenuProfile) | **Game crash** |
| 4 | 0xFF in the full FoW bitfield range, no region change | All fog removed (cosmetic only) |
| 5 | Inserting 205 regions via byte-shift (legacy path) | **Game crash** (slot truncated) — fixed by switching to `RebuildSlot` |
| 6 | In-game teleport (Warmaster's Shack) | Adds region `6101000` + sets 356 FoW bits in a 157-byte window |
| 7 | Setting a 62xxx visible flag without a fragment | Map texture revealed, but no fragment in the UI (inventory) |
| 8 | DLC visible flags only, without Phase 3 Cover Layer | Texture appears, but black tiles still cover the DLC area |

### Test files (`tmp/save/`)

| File | Description |
|---|---|
| `ER0000.sl2` | Original save, full fog of war, 6 regions |
| `ER0000-fow-before.sl2` | After editing (maps + grace added), FoW unchanged |
| `ER0000-from-deck.sl2` | After playing in-game (1 teleport), local fog removed |
| `ER0000-no-fow-test.sl2` | Region + partial bitfield, fog removed locally |
| `ER0000-no-fow.sl2` | Full bitfield fill, all fog removed |

---

## 20. Sources

### Code

- `app_world.go::GetMapProgress`/`SetMapRegionFlags`/`SetMapFlag`/`RevealAllMap`/`revealBaseMap`/`revealDLCMap`/`ResetMapExploration`/`RemoveFogOfWar` — endpoints + internal helpers
- `app_world.go::resolveAfterRegs`/`putF32` — coord helpers
- `backend/db/data/maps.go::MapSystem`/`MapVisible`/`MapUnsafe`/`MapFragmentItems`/`MapAcquired`/`IsDLCMapFlag`
- `backend/db/data/regions.go::GetAllRegions` — 104 regions
- `backend/core/writer.go::SetUnlockedRegions`
- `backend/core/slot_rebuild.go::RebuildSlot`
- `backend/core/offset_defs.go:309–322` — DLC tile constants
- `backend/core/structures.go::DynStorageBox`/`DynStorageToGestures` — slot offset math
- `frontend/src/components/WorldTab.tsx` — UI integration

### Tests

- `backend/core/writer_regions_test.go` — 4 L0 tests (PS4/PC, in-memory, after-add-item)
- `tests/map_flags_test.go` — 4 L1 tests (BST, GetAll, Roundtrip, OffsetCorrectness)

### Reference parsers

- er-save-manager (Python) — `parser/event_flags.py`, region layout reference
- ER-Save-Editor (Rust) — region container layout reference

### Hex-verified saves

- `tmp/save/ER0000.sl2` (PC, 5 slots) — round-trip 2026-05-21
- `tmp/save/oisis_pl-org.txt` (PS4) — round-trip 2026-05-21
- `tmp/save/ER0000-no-fow*.sl2` — FoW investigation snapshots
