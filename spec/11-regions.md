# 11 — Regions (UnlockedRegions — fast travel + invasion eligibility)

> **Type**: Binary format spec + region database reference
> **Status**: ✅ canonical (Phase 4 Step 3 — 2026-05-21)
> **Scope**: Detail chapter for the L0 layer from [27 — Map Reveal](27-map-reveal.md). Describes the `UnlockedRegions` array binary layout, the write path via `core.SetUnlockedRegions` (with `RebuildSlot`), the contents of `backend/db/data/regions.go` (104 entries, snapshot 2026-05-21), and integrations (`App.SetRegionUnlocked`, `App.BulkSetUnlockedRegions`, `ApplyPvPPreparation` matchmaking module).

---

## 1. Chapter purpose

`UnlockedRegions` is a variable-length `u32` array in the save slot controlling:

- **Fast travel** between Sites of Grace within an unlocked region.
- **Invasion eligibility** (PvP / NPC invaders) for an area.
- **Blue summons** (Recusant Henricus, Bloody Finger questline).
- **"You have entered <X>" label** after teleport.

This is **Layer 0** in the 4-layer Map Reveal model (see [27 §5](27-map-reveal.md#5-map-reveal-layers--overview)). L0 does not reveal the map texture — L1 (event flags 62xxx + Map Fragment items) does that.

This chapter describes:

- the `UnlockedRegions` array binary layout
- the `core.SetUnlockedRegions` write pipeline + `RebuildSlot` integration
- the `RegionData` static DB (`backend/db/data/regions.go`, 104 entries, snapshot 2026-05-21)
- the 3 App-level Wails methods (`GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions`)
- the integration with the `ApplyPvPPreparation` MatchmakingRegions module
- 4 round-trip + mutation + shrink tests
- safety + `needs verification` items

This chapter does **not** duplicate:

- the 4-layer Map Reveal model → [27 — Map Reveal](27-map-reveal.md)
- event flag bit/byte indexing → [15 — Event Flags](15-event-flags.md)
- DLC Cover Layer detail → [29 — DLC Black Tiles](29-dlc-black-tiles.md)
- item companion semantics → [50 — Item Companion Flags](50-item-companion-flags.md)
- the full slot rebuild research → [30](30-slot-rebuild-research.md)

---

## 2. Status

| Component | Implementation | Test coverage | Note |
|---|---|---|---|
| Binary read (`u32 count + count*u32 IDs`) | ✅ `Slot.UnlockedRegionsOffset` + `Slot.UnlockedRegions` parsed in `structures.go:402–411` | ✅ covered by round-trip parsing | — |
| Write — `core.SetUnlockedRegions` | ✅ `backend/core/writer.go:911` with `RebuildSlot` + rollback | ✅ 4 tests (InMemory, PS4 round-trip, AfterAddItem, PC round-trip) | Atomic via rollback |
| Static DB — `Regions` map | ✅ `backend/db/data/regions.go`, 104 entries | ⚠️ covered only by a `GetAllRegions()` smoke (no dedicated test) | Snapshot 2026-05-21 |
| App-level toggles | ✅ `app_world.go:186/211/247` (`GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions`) | ⚠️ covered indirectly by `writer_regions_test.go` | — |
| PvP integration | ✅ `app_pvp.go:47` MatchmakingRegions module — `core.SetUnlockedRegions(slot, allRegions)` | ✅ covered by `pvp_test.go` round-trip | — |
| Frontend | ✅ `WorldTab.tsx` accordion (Invasion Regions) + `RiskSectionBanner` | — | Tier 1 risk banner |
| Empirical fresh-save markers (1001000–1001002, 1800001, 1800090, 6100000) | ⚠️ observed in saves, NOT in `regions.go` | ❌ no dedicated test | `needs verification` of the markers' purpose |

**Last verification**: 2026-05-21 on `tmp/save/ER0000.sl2` (PC, 5 slots) + `tmp/save/oisis_pl-org.txt` (PS4) — 4 round-trip tests + Add-Item interaction.

---

## 3. Source of truth in code

| Component | File / lines | Note |
|---|---|---|
| Slot field | `backend/core/structures.go:245–246` | `UnlockedRegionsOffset int`, `UnlockedRegions []uint32` |
| Read parser | `backend/core/structures.go:402–411` | `calculateDynamicOffsets` populates from `gesturesOff` |
| Layout constants | `backend/core/offset_defs.go:97–98` | `DynStorageBox = 0x6010`, `DynStorageToGestures = 0x100` |
| Writer | `backend/core/writer.go:911 SetUnlockedRegions` | Dedup + sort + `RebuildSlot` + rollback |
| Slot rebuilder | `backend/core/slot_rebuild.go::RebuildSlot` | Full sequential serializer |
| Static DB struct | `backend/db/data/regions.go::RegionData` | `{Name, Area}` |
| Static DB map | `backend/db/data/regions.go::Regions` | 104 entries (snapshot 2026-05-21) |
| DB API | `backend/db/db.go:1114 GetAllRegions`, `:1116 IsKnownRegionID` | `RegionEntry{ID, Name, Area, ...}` with a `sync.OnceValue` cache |
| Entry struct | `backend/db/db.go:126 RegionEntry` | `ID, Name, Area, Unlocked` (Unlocked added by `GetUnlockedRegions` per-slot) |
| App methods | `app_world.go:186 GetUnlockedRegions`, `:211 SetRegionUnlocked`, `:247 BulkSetUnlockedRegions` | Wails-exposed |
| PvP integration | `app_pvp.go:47 ApplyPvPPreparation` (MatchmakingRegions module) | Uses `GetAllRegions()` + `SetUnlockedRegions(slot, ids)` |
| Tests | `backend/core/writer_regions_test.go:11/48/106/158` | 4 tests |
| Frontend | `frontend/src/components/WorldTab.tsx:2 imports`, `:252/258/264/270/276 handlers` | Accordion + bulk select/clear + risk banner |

---

## 4. Mental model

`UnlockedRegions` is a **simple variable-length array** in the slot:

```
slot.Data
  ├── StorageBox (DynStorageBox = 0x6010 bytes)
  ├── ...gap (DynStorageToGestures = 0x100 bytes — gestures?)...
  ├── gesturesOff                                ← slot.UnlockedRegionsOffset
  │     ├── count : u32 (little-endian)
  │     └── region_id[0..count-1] : u32
  ├── afterRegs (= gesturesOff + 4 + count*4)
  │     ├── (BloodStain + DLC tile coords)       ← L2 (see 29)
  │     └── (Fog of War bitfield)                ← L3 (see 27 §9)
  └── ...subsequent sections...
```

**Implication**: adding/removing a region changes the size of the variable-length array → changes the `afterRegs` offset → everything **after** `gesturesOff` must be re-serialized. That is why `SetUnlockedRegions` uses `RebuildSlot` (full sequential serializer), not an in-place byte-shift.

A region ID is opaque (the player only sees areas, not IDs). The mapping ID → human-readable name + area group is in `backend/db/data/regions.go`.

---

## 5. Region data model

### 5.1 Binary layout

```
┌────────────────────────────────┐
│ count : u32 (little-endian)    │  4 bytes
├────────────────────────────────┤
│ region_id[0..count-1] : u32    │  count × 4 bytes
└────────────────────────────────┘
Total: 4 + count*4 bytes
```

### 5.2 Endianness and order

- **Little-endian** (all u32 fields).
- **Order on disk**: no sorting guarantee — the game stores IDs in acquisition order.
- **The editor sorts on write** ascending (for stable diffs). See `SetUnlockedRegions` step 2.
- **Duplicates**: not observed in any reference save; `SetUnlockedRegions` deduplicates preventively.

### 5.3 RegionData struct (DB)

```go
// backend/db/data/regions.go
type RegionData struct {
    Name string
    Area string  // 11 unique values — see §5.5
}

var Regions = map[uint32]RegionData{
    6100000: {Name: "The First Step", Area: "Limgrave"},
    ...
}
```

### 5.4 RegionEntry (API exposed)

```go
// backend/db/db.go:126
type RegionEntry struct {
    ID       uint32 `json:"id"`
    Name     string `json:"name"`
    Area     string `json:"area"`
    Unlocked bool   `json:"unlocked"`  // populated per-slot by App.GetUnlockedRegions
}
```

### 5.5 Area enum (11 unique values in the current snapshot)

The `Area` field values from the `Regions` map (2026-05-21):

```
"Altus Plateau", "Caelid", "Farum Azula", "Haligtree",
"Land of Shadow", "Legacy Dungeons", "Limgrave", "Liurnia",
"Mountaintops", "Mt. Gelmir", "Underground"
```

⚠️ DLC regions use `Area: "Land of Shadow"`, **not** `"DLC"`. Inferred manually from er-save-manager + curated; there is no strict enum in the code — it is a `string` field. `GetAllRegions()` sorts by `Area` (alphabetic) → `Name`.

---

## 6. Region IDs and grouping

### 6.1 Entry count (snapshot 2026-05-21)

| Group | Count | Range |
|---|---|---|
| Overworld base game | 62 | 6100000–6800000 (Limgrave / Liurnia / Altus / Caelid / Mountaintops / Mt. Gelmir / Underground / Farum Azula / Haligtree) |
| Overworld DLC (Shadow of the Erdtree) | 7 | 6900000–6900006 (Land of Shadow) |
| Legacy dungeons | 35 | 1000000–1900001 (Stormveil, Leyndell, Roundtable Hold, Academy of Raya Lucaria, Volcano Manor, Stranded Graveyard, Stone Platform / Elden Beast) |
| **Total** | **104** | — |

⚠️ Snapshot 2026-05-21. After a game patch update (e.g., a new DLC), a re-extraction of `regions.go` from er-save-manager / community RE is required.

### 6.2 Prefix overview

Full per-prefix list (from the `Regions` map):

| Prefix | Count | Area (from `RegionData.Area`) | Examples |
|---|---|---|---|
| 1000xxx | 5 | Legacy Dungeons | Stormveil Castle, Main Gate, Rampart Tower |
| 1100xxx | 8 | Legacy Dungeons | Leyndell, Royal Capital, Erdtree Sanctuary |
| 1101xxx | 1 | Legacy Dungeons | Roundtable Hold |
| 1105xxx | 4 | Legacy Dungeons | Leyndell, Capital of Ash (post-Farum Azula) |
| 1400xxx | 5 | Legacy Dungeons | Academy of Raya Lucaria |
| 1600xxx | 8 | Legacy Dungeons | Volcano Manor (interior), Temple of Eiglay |
| 1800xxx | 2 | Legacy Dungeons | Stranded Graveyard, Cave of Knowledge |
| 1900xxx | 2 | Legacy Dungeons | Stone Platform, Elden Beast |
| 6100xxx | 6 | Limgrave | The First Step, Agheel Lake, Mistwood |
| 6101xxx | 2 | Limgrave | Stormhill Shack, Margit the Fell Omen |
| 6102xxx | 3 | Limgrave | Weeping Peninsula |
| 6200xxx | 10 | Liurnia | Lake-Facing Cliffs, Caria Manor, Liurnia Highway |
| 6201xxx | 1 | Liurnia | Bellum Church |
| 6202xxx | 1 | Liurnia | Moonlight Altar |
| 6300xxx | 6 | Altus Plateau | Altus overworld |
| 6301xxx | 2 | Altus Plateau | Capital Outskirts, Capital Rampart |
| 6302xxx | 3 | Mt. Gelmir | Ninth Mt. Gelmir Campsite, Road of Iniquity |
| 6400xxx | 6 | Caelid | Caelid overworld |
| 6401xxx | 1 | Caelid | Swamp of Aeonia |
| 6402xxx | 2 | Caelid | Dragonbarrow West, Bestial Sanctum |
| 6500xxx | 2 | Mountaintops | Mountaintops West |
| 6501xxx | 2 | Mountaintops | Zamor Ruins, Central Mountaintops |
| 6502xxx | 3 | Mountaintops | Consecrated Snowfield, Ordina |
| 6600xxx | 8 | Underground | Siofra, Ainsel, Deeproot, Mohgwyn, Lake of Rot |
| 6700xxx | 3 | Farum Azula | Crumbling Farum Azula, Dragon Temple, Beside the Great Bridge |
| 6800xxx | 1 | Haligtree | Miquella's Haligtree |
| 6900xxx | 7 | Land of Shadow | DLC (Gravesite Plain, Scadu Altus, Shadow Keep, etc.) |

Full list: `backend/db/data/regions.go` (104 named entries).

### 6.3 Empirical fresh-save markers (NOT in the DB)

In every fresh save (post-character-creation) the observed region IDs are:

```
1001000, 1001001, 1001002  ← internal startup markers (NOT in regions.go)
1800001                    ← Stranded Graveyard (in regions.go)
1800090                    ← Cave of Knowledge (in regions.go)
6100000                    ← The First Step (in regions.go)
```

⚠️ The markers `1001000–1001002` are **not** registered in the `Regions` map nor in any reference editor (er-save-manager, ER-Save-Editor). They are probably internal startup tokens used by the engine at the tutorial level. `needs verification` of their purpose. The editor preserves them on round-trip (does not filter them), but `GetAllRegions()` does not return them → the UI does not show them as toggle-able.

### 6.4 Late-game saves

In late-game saves (post-elden-beast) **up to 395 entries** were observed in `UnlockedRegions` (more than the 104 from the DB — the difference being sub-region IDs used internally by the engine + invasion area subdivisions). `GetAllRegions()` returns only the 104 known "named regions"; the remaining IDs are preserved by round-trip but invisible in the UI.

---

## 7. Relation to Map Reveal L0

`UnlockedRegions` is **Layer 0** in the 4-layer Map Reveal model (see [27 — Map Reveal](27-map-reveal.md)).

### 7.1 What L0 does

- Enables fast travel within the region.
- Marks the region as "visited" for PvP matchmaking.
- Displays the "You have entered <X>" label after teleport.

### 7.2 What L0 does **not** do

| What | Responsible layer |
|---|---|
| Reveals the region's map texture | L1 (event flags 62xxx) — [27 §7](27-map-reveal.md#7-l1--detailed-bitmap-event-flags--map-fragments) |
| Adds a map fragment to the inventory | L1 (`MapFragmentItems`) — [27 §7.3](27-map-reveal.md#73-map-fragment-items) |
| Removes DLC black tiles | L2 (Cover Layer) — [29](29-dlc-black-tiles.md) |
| Removes fog of war (FoW) | L3 (FoW bitfield) — [27 §9](27-map-reveal.md#9-l3--fog-of-war-removefogofwar) |

Empirical verification: [27 §17 test 1](27-map-reveal.md#19-verification-log-historical) — adding a region_id (without flag 62xxx) → the map texture is unchanged.

### 7.3 Order of operations in an in-game teleport

After teleporting to a new area, the game:

1. Adds the region ID to `UnlockedRegions` (L0).
2. Sets the visibility event flag 62xxx (L1) — the texture is revealed.
3. Sets 356 bits in the FoW bitfield (L3) within a 157-byte area — the local fog disappears.

The editor simulates this via `RevealAllMap` (L1 + L2) + `RemoveFogOfWar` (L3), but L0 (regions) must be set separately via `SetRegionUnlocked` / `BulkSetUnlockedRegions` / `ApplyPvPPreparation` MatchmakingRegions.

---

## 8. Relation to event flags

`UnlockedRegions` is an **independent structure** in the slot, **not** the event flag bitfield. Cross-cutting different storage:

| Aspect | `UnlockedRegions` (L0) | Event Flags (L1) |
|---|---|---|
| Storage | Variable-length `[]uint32` at `gesturesOff` | Fixed bitfield `0x1BF99F` bytes in `EventFlagsBlock` |
| Helper API | `core.SetUnlockedRegions` (slot-level, full rebuild) | `db.SetEventFlag` (bit-level, in-place) |
| Atomicity | Atomic via rollback (snapshot prev state + slot.Data) | Stateless bit mutation, no rollback |
| Sorting | Dedup + sort ascending on write | n/a |

Generic event flag foundation (bit/byte indexing, BST resolver, helper API, bounds check) → [15 — Event Flags](15-event-flags.md).

---

## 9. Current implemented behavior

### 9.1 Wails-exposed API (`app_world.go`)

| Endpoint | Signature | Purpose |
|---|---|---|
| `GetUnlockedRegions(slotIdx)` | `([]db.RegionEntry, error)` | Read all 104 DB regions with the `Unlocked` bool per-slot |
| `SetRegionUnlocked(slotIdx, regionID, unlocked)` | `error` | Toggle a single region (add/remove + RebuildSlot) |
| `BulkSetUnlockedRegions(slotIdx, regionIDs)` | `error` | Replace the full list (idempotent) |

### 9.2 Internal (`backend/core`)

| Function | Signature | Note |
|---|---|---|
| `SetUnlockedRegions(slot, ids)` | `error` | Dedup + sort + `RebuildSlot` + rollback |
| `RebuildSlot(slot)` | `([]byte, error)` | Full sequential serializer of the slot (see [30](30-slot-rebuild-research.md)) |

### 9.3 PvP integration

`ApplyPvPPreparation` MatchmakingRegions module (`app_pvp.go:47`):

```go
if opts.MatchmakingRegions {
    allRegions := db.GetAllRegions()
    ids := make([]uint32, len(allRegions))
    for i, r := range allRegions { ids[i] = r.ID }
    if err := core.SetUnlockedRegions(slot, ids); err != nil {
        return nil, fmt.Errorf("matchmaking regions: %w", err)
    }
    // SetUnlockedRegions calls RebuildSlot which replaces slot.Data.
    // Offsets are refreshed automatically; no manual offset recalc needed.
}
```

Effect: all 104 known regions (62 + 7 + 35) unlocked for PvP matchmaking.

### 9.4 Frontend integration

`WorldTab.tsx` "Invasion Regions" accordion:

- Per-region checkbox → `SetRegionUnlocked(charIdx, r.id, unlocked)` (line 252)
- "Select All in area" → `BulkSetUnlockedRegions(charIdx, next)` (lines 258, 264)
- "Unlock All" → `BulkSetUnlockedRegions(charIdx, regionEntries.map(r => r.id))` (line 270)
- "Reset" → `BulkSetUnlockedRegions(charIdx, [])` (line 276)
- `RiskSectionBanner` in the accordion header (see [32](32-ban-risk-system.md))

---

## 10. Generated data and snapshot caveats

### 10.1 Origin

`backend/db/data/regions.go` is a **static catalog generated offline** from [er-save-manager](https://github.com/Jeius/er-save-manager) — community-researched IDs. Source comment in the file:

> `Source: er-save-manager/src/er_save_manager/data/regions.py — community-researched IDs.`

### 10.2 Snapshot date

**2026-05-21** — 104 entries. After a game patch update (e.g., a new DLC, area rebalancing), a re-extraction is required. There is no automatic diff vs `regulation.bin`.

### 10.3 What may change after a patch

- **New DLC** → new IDs in the range 69xxxxx or another.
- **Re-balancing areas** → possible new sub-region IDs (e.g., extra Stormveil sub-zones).
- **Unknown** IDs returned as `unknown` by `IsKnownRegionID(id)` → the editor preserves them on round-trip, but does not show them in the UI.

### 10.4 Inferred Area grouping

The `Area` field in `RegionData` is **manually curated** — 11 unique values in the current snapshot (see §5.5).

⚠️ Inferred — not from the save format. `needs verification` that each region is classified into the correct area group (the UI groups regions per area in the accordion). DLC regions use `"Land of Shadow"`, not `"DLC"`.

---

## 11. Validation and rollback caveats

### 11.1 `core.SetUnlockedRegions` pipeline

```
1. calculateDynamicOffsets()       — refresh offsets (from stale slot.Data)
2. buildSectionMap()                — refresh section map
3. dedup + sort ascending           — stable output
4. snapshot prev (UnlockedRegions, slot.Data)
5. RebuildSlot(slot)                — full rebuild
    └── on error → restore prev (slot.UnlockedRegions, slot.Data)
6. recalc offsets                   — slot.calculateDynamicOffsets
    └── on error → restore prev (slot.UnlockedRegions, slot.Data)
7. recalc SectionMap                — slot.buildSectionMap
    └── on error → append warning (non-fatal)
```

### 11.2 Critical pre-refresh

Comment in `writer.go:911`:

> Other writers (`AddItemsToSlot`, `FlushGaItems`, `revealDLCMap`, …) mutate `slot.Data` without updating `slot.UnlockedRegionsOffset` or `SectionMap`, so without this refresh we would rebuild from stale boundaries and produce a corrupted save (observed when the user added an item, then revealed the map, then unlocked regions — slot 4 of ER0000.sl2 corrupted at the regCount offset).

⚠️ Implication: calling `SetUnlockedRegions` after `AddItemsToSlot` / `revealDLCMap` without `calculateDynamicOffsets` → corrupted save. The current code does this refresh internally. The `TestSetUnlockedRegionsAfterAddItem` test verifies this case.

### 11.3 Rollback granularity

- **Full rollback** for `slot.Data` + `slot.UnlockedRegions` (snapshot at the start).
- **Warning-only rollback** for the `SectionMap` rebuild (non-fatal — continues with the stale section map).

### 11.4 Stress test

R-1 step 17: tested up to ~100,000 regions in synthetic stress tests. After the rebuild the data ends ~2.2 MB into the slot of `SlotSize = 0x280000`, leaving 408–432 KB of zero padding. **No truncation risk** for realistic ranges (`Unlock All` adds 104 regions, a fresh save has 6).

### 11.5 Historical: removed byte-shift path

The old implementation (the Stage-1 invasion-regions feature) inserted region IDs in-place by shifting the rest of the slot. The "max 10–20 regions" limit followed from the missing slack at the end of the slot — exceeding it crashed the save (region hash truncation). **Removed** in R-1 Step 14 (see the CHANGELOG entry "feature/invasion-regions — Stage 2"). `SetUnlockedRegions` with `RebuildSlot` is the only supported entry point.

---

## 12. Safety notes

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| V1 | Stale `regions.go` after a game patch | ⚠️ medium | `needs verification` after every DLC/patch update; there is no automatic diff vs `regulation.bin`. |
| V2 | `Unlock All` (104 regions) on a non-DLC save → DLC IDs 69xxxxx in the save | ⚠️ low | The game tolerates it (safe), but it is logically incorrect. The UI could warn on DLC IDs — `needs verification` whether it does. |
| V3 | Empirical fresh-save markers 1001000–1001002 not in the DB | ⚠️ medium | Round-trip preserves them, but `GetAllRegions()` does not return them → the UI does not show them. `needs verification` of their purpose. |
| V4 | Setting a region without L1 (visible flag) → fast travel ok, but the map is empty | ⚠️ low | User-side: must also call `RevealAllMap` or `SetMapRegionFlags` (L1). |
| V5 | `BulkSetUnlockedRegions([])` → loss of all regions (no fast travel) | 🔴 high | Atomic via rollback in `SetUnlockedRegions`; the UI snapshot-undo (`pushUndo`) is a workaround. `RiskSectionBanner` warns. |
| V6 | `SetUnlockedRegions` after `AddItemsToSlot` without refreshing offsets | 🔴 high | Already solved in the code (line 911 with a refresh before the rebuild). The `TestSetUnlockedRegionsAfterAddItem` test verifies it. |
| V7 | `RebuildSlot` corrupts the slot on a large dataset (>100k regions) | ⚠️ low | Stress-tested up to ~100k; a user's `Unlock All` is 104. No real risk. |
| V8 | Late-game save with ~395 entries containing unknown sub-region IDs | ⚠️ medium | Round-trip preserves them, but the UI does not show them. `needs verification` whether they are writeable without side effects. |
| V9 | DLC regions 69xxxxx on a non-DLC save → the save is still valid but the UI mismatches | ⚠️ low | See V2 (duplicate). |
| V10 | `SectionMap` rebuild failure is non-fatal | ⚠️ low | Append warning + continue with the stale section map. `needs verification` whether the stale section map causes other issues in subsequent mutations. |

---

## 13. Test coverage

| Test | Purpose | File |
|---|---|---|
| `TestSetUnlockedRegionsInMemory` | In-memory mutation (dedup + sort + recalc) | `backend/core/writer_regions_test.go:11` |
| `TestSetUnlockedRegionsRoundTripPS4` | PS4 round-trip (read → mutate → write → read → byte-equal mutated state) | `backend/core/writer_regions_test.go:48` |
| `TestSetUnlockedRegionsAfterAddItem` | Interaction with `AddItemsToSlot` — verify the refresh offsets path | `backend/core/writer_regions_test.go:106` |
| `TestSetUnlockedRegionsRoundTripPC` | PC round-trip like PS4 | `backend/core/writer_regions_test.go:158` |

No dedicated test for:

- `BulkSetUnlockedRegions([])` (empty replace — V5 risk).
- DLC region 69xxxxx on a non-DLC save (V2).
- 1001000–1001002 markers preservation through round-trip (V3).
- PS4↔PC cross-platform conversion test (`TestConvert*` does not exist).
- Late-game save with ~395 entries (V8).

---

## 14. Known limits / needs verification

| # | Limit / gap | Status | Note |
|---|---|---|---|
| L1 | Snapshot 2026-05-21 (104 entries) | ⚠️ stale after a patch | A re-extraction from er-save-manager is required after a game update. |
| L2 | Fresh-save markers 1001000–1001002 | ❓ purpose unknown | `needs verification`. Round-trip OK, the UI does not show them. |
| L3 | Late-game saves with ~395 entries contain sub-region IDs outside the DB | ⚠️ partial coverage | `GetAllRegions()` returns 104; the rest are preserved but invisible. |
| L4 | DLC region ownership cross-check | ❌ none | The editor does not check whether the save owns the DLC before setting 69xxxxx. The UI could warn. |
| L5 | PS4↔PC cross-platform bit-equal | ❌ no `TestConvert*` | Round-trip per-platform OK; no test for "PS4 → save → convert → load PC → identical IDs". |
| L6 | `Area` grouping inferred manually | ⚠️ snapshot | Possible discrepancy after adding new regions (e.g., a DLC area unclassified). |
| L7 | `SectionMap` rebuild non-fatal | ⚠️ design choice | A stale section map may cause subtle bugs in subsequent mutations — `needs verification`. |
| L8 | Sub-region IDs (e.g., 6101000 vs 6100000) | ⚠️ semantics unclear | The engine uses sub-areas internally (Stormhill is a sub-area of Limgrave). The subarea-to-parent mapping is not implemented. |
| L9 | Empty `BulkSetUnlockedRegions([])` | ❌ no guard | A possible "reset all regions" use case — atomic via rollback, but no UI warning. |
| L10 | `RegionData` source comes from er-save-manager | ⚠️ dependency | If upstream errors / removes IDs, our DB may diverge. No CI check. |

---

## 15. Cross-references

| Topic | Master chapter |
|---|---|
| 4-layer Map Reveal model (regions = L0) | [27 — Map Reveal](27-map-reveal.md) |
| Event flag foundation (bit/byte indexing, helper API, BST) | [15 — Event Flags](15-event-flags.md) |
| DLC Cover Layer (L2 in the map layers — independent of L0) | [29 — DLC Black Tiles](29-dlc-black-tiles.md) |
| Sites of Grace (related — fast travel between graces) | [47 — Sites of Grace](47-site-of-grace-activation.md) |
| World state (FieldArea, WorldArea — read-only, no relation to L0) | [16 — World State](16-world-state.md) |
| PvP modular MatchmakingRegions module | [48 — PvP Modular Presets](48-pvp-ready-modular-presets.md) |
| Slot rebuild research (`RebuildSlot` design) | [30 — Slot Rebuild Research](30-slot-rebuild-research.md) |
| Ban-risk Tier 1 UI (Invasion Regions risk banner) | [32 — Ban-Risk System](32-ban-risk-system.md) |

---

## 16. Sources

### Code

- `backend/core/structures.go:245–246` — `Slot.UnlockedRegionsOffset` + `Slot.UnlockedRegions`
- `backend/core/structures.go:402–411` — parser (`calculateDynamicOffsets`)
- `backend/core/offset_defs.go:97–98` — `DynStorageBox = 0x6010`, `DynStorageToGestures = 0x100`
- `backend/core/writer.go:911 SetUnlockedRegions` — writer + rollback
- `backend/core/slot_rebuild.go::RebuildSlot` — full sequential serializer
- `backend/db/data/regions.go` — 104 entries + `RegionData` struct (snapshot 2026-05-21)
- `backend/db/db.go:126 RegionEntry`, `:1114 GetAllRegions`, `:1116 IsKnownRegionID`
- `app_world.go:186/211/247` — `GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions`
- `app_pvp.go:47` — `ApplyPvPPreparation` MatchmakingRegions module
- `frontend/src/components/WorldTab.tsx` — UI accordion + handlers

### Tests

- `backend/core/writer_regions_test.go` — 4 tests (InMemory, RoundTripPS4, AfterAddItem, RoundTripPC)
- `tests/pvp_test.go` — covers the PvP MatchmakingRegions path

### Reference parsers / community

- er-save-manager (Python): `parser/world.py::Regions` + `data/regions.py` — community-researched IDs (upstream source)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — length-prefixed list reference

### Hex-verified saves

- `tmp/save/ER0000.sl2` (PC, 5 slots) — round-trip 2026-05-21
- `tmp/save/oisis_pl-org.txt` (PS4) — round-trip 2026-05-21
