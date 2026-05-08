# 11 — Regions (Unlocked Regions list)

> **Scope:** binary format of the `unlocked_regions` array. Variable-length
> section that drives fast travel and per-region game state.
> Higher-level semantics (regions ≠ map reveal): see `spec/27-map-reveal.md`.

---

## Location in slot

The list sits at `gesturesOff` in the dynamic offset chain:

```
StorageBoxOffset
  + DynStorageBox        (0x6010)   = storageEnd
  + DynStorageToGestures (0x100)    = gesturesOff
  [ gesturesOff ]                   = unlocked_regions header
  + 4 + count * 4                   = afterRegs (horse / bloodstain start)
```

Constants live in `backend/core/offset_defs.go`.

---

## Binary format

```
┌───────────────────────────────────┐
│ count : u32 (little-endian)       │  4 bytes
├───────────────────────────────────┤
│ region_id[0..count-1] : u32       │  count × 4 bytes
└───────────────────────────────────┘
Total: 4 + count * 4 bytes
```

- Endianness: little-endian.
- Order: stored unsorted on disk; the editor sorts on write for stable
  diffs (`core.SetUnlockedRegions`).
- Duplicates: not observed in any reference save; `SetUnlockedRegions`
  dedupes defensively.

A fresh character has 6 entries: `1001000, 1001001, 1001002, 1800001,
1800090, 6100000` (3 internal startup markers + Stranded Graveyard +
Cave of Knowledge + The First Step). Late-game saves observed with up
to 395 entries.

Note: `1800090` (Cave of Knowledge) is present in the **default** fresh-character
list — it is safe to include in `unlocked_regions` on any save.

---

## Region ID ranges

| Range | Area type | Example regions |
|-------|-----------|-----------------|
| 1001000–1001002 | Internal startup markers (purpose unknown) | — |
| 1xxxxxx (1010–1099, 1200–1207, 1300, 1400, 1500, 1600, 1800, 1900) | Legacy dungeons / endgame zones | Stormveil Castle, Leyndell, Haligtree, Volcano Manor, Crumbling Farum Azula |
| 3xxxxxx | Catacombs / caves / tunnels | Stormfoot Catacombs, Murkwater Cave |
| 6100xxx | Limgrave overworld | The First Step, Stormhill |
| 6102xxx | Weeping Peninsula | Castle Morne |
| 6200xxx | Liurnia of the Lakes | Caria Manor |
| 6300xxx | Altus Plateau | Mt. Gelmir, Altus Highway |
| 6400xxx | Caelid / Dragonbarrow | Bestial Sanctum |
| 6500xxx | Mountaintops / Snowfield | Zamor Ruins, Forbidden Lands |

Full database: `backend/db/data/regions.go` (104 base-game entries),
exposed via `db.GetAllRegions()`. The 104 entries cover all known matchmaking
regions from the er-save-manager reference list (103 IDs) plus Roundtable Hold.
DLC region IDs (6900xxx) are included. See `spec/46-faster-invasions-research.md`
for coverage history.

---

## Editing

Use `core.SetUnlockedRegions` — the only supported entry point:

```go
err := core.SetUnlockedRegions(slot, []uint32{6100000, 6100100, ...})
```

Behavior:

1. Dedupes the input.
2. Sorts ascending.
3. Calls `core.RebuildSlot` to re-serialize the slot from typed
   structures (R-1 Step 13 — see `spec/30-slot-rebuild-research.md`).
4. Updates dynamic offsets (`GaItemDataOffset`, `IngameTimerOffset`,
   `EventFlagsOffset`) to match the new layout.

`RebuildSlot` is safe for any realistic count: post-rebuild end-of-data
sits ~2.2 MB into the 0x280000-byte slot, leaving 408–432 KB of zero
tail padding. Tested up to ~100,000 regions in synthetic stress tests;
the user-facing "Unlock All" path adds at most ~104 regions.

> **Do not** edit the list by raw byte insertion / shifting. The
> historical "shift in place, max 10–20 regions" path was removed in
> R-1 Step 14 because it truncated the hash region beyond ~205 inserts
> and corrupted the save.

---

## Effect (and what it does NOT do)

Setting a region ID:

- ✅ Enables fast travel between Sites of Grace inside that region.
- ✅ Marks the region as "visited" for invasions / multiplayer
  matchmaking.

Setting a region ID does **not**:

- ❌ Reveal the map texture (that's the 62xxx event flag — see
  `spec/27-map-reveal.md` §2).
- ❌ Remove fog of war (FoW bitfield is independent).
- ❌ Remove DLC black tiles (Cover Layer is independent — see
  `spec/29-dlc-black-tiles.md`).

These layers were verified empirically as independent (test 1 in
`spec/27-map-reveal.md` §5).

---

## References

- Format source: `er-save-manager/parser/world.py::Regions` (lines 92–117);
  `ER-Save-Editor` (Rust) `src/save/common/save_slot.rs` length-prefixed
  list.
- Editor entry point: `backend/core/writer.go::SetUnlockedRegions`.
- Slot rebuild: `backend/core/slot_rebuild.go::RebuildSlot`.
- Region database: `backend/db/data/regions.go`,
  `backend/db/db.go::GetAllRegions`.
- Wails API: `app.go::GetUnlockedRegions`, `SetRegionUnlocked`,
  `BulkSetUnlockedRegions`.
