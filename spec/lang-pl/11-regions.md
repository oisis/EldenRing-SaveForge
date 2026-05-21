# 11 — Regions (UnlockedRegions — szybka podróż + invasion eligibility)

> **Typ**: Binary format spec + region database reference
> **Status**: ✅ canonical (Phase 4 Step 3 — 2026-05-21)
> **Zakres**: Detail rozdział dla L0 warstwy z [27 — Map Reveal](27-map-reveal.md). Opisuje binary layout `UnlockedRegions` array, write path przez `core.SetUnlockedRegions` (z `RebuildSlot`), zawartość `backend/db/data/regions.go` (104 wpisy snapshot 2026-05-21), oraz integracje (`App.SetRegionUnlocked`, `App.BulkSetUnlockedRegions`, `ApplyPvPPreparation` matchmaking module).

---

## 1. Cel rozdziału

`UnlockedRegions` to wariadyczna tablica `u32` w slocie save'a kontrolująca:

- **Szybką podróż** między Sites of Grace w obrębie odblokowanego regionu.
- **Invasion eligibility** (PvP / NPC invaders) dla obszaru.
- **Blue summons** (Recusant Henricus, Bloody Finger questline).
- **"You have entered <X>" label** po teleporcie.

To jest **Layer 0** w 4-warstwowym modelu Map Reveal (patrz [27 §5](27-map-reveal.md#5-map-reveal-layers--overview)). L0 nie odkrywa tekstury mapy — to robi L1 (event flags 62xxx + Map Fragment items).

Ten rozdział opisuje:

- binary layout `UnlockedRegions` array
- `core.SetUnlockedRegions` write pipeline + `RebuildSlot` integration
- `RegionData` static DB (`backend/db/data/regions.go`, 104 wpisy snapshot 2026-05-21)
- 3 App-level Wails methods (`GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions`)
- integrację z `ApplyPvPPreparation` MatchmakingRegions module
- 4 testy round-trip + mutation + shrink
- safety + `needs verification` items

Ten rozdział **nie** powiela:

- 4-warstwowego modelu Map Reveal → [27 — Map Reveal](27-map-reveal.md)
- event flag bit/byte indexing → [15 — Event Flags](15-event-flags.md)
- DLC Cover Layer detail → [29 — DLC Black Tiles](29-dlc-black-tiles.md)
- item companion semantics → [50 — Item Companion Flags](50-item-companion-flags.md)
- pełnego slot rebuild research → [30](30-slot-rebuild-research.md)

---

## 2. Status

| Komponent | Implementation | Test coverage | Notatka |
|---|---|---|---|
| Binary read (`u32 count + count*u32 IDs`) | ✅ `Slot.UnlockedRegionsOffset` + `Slot.UnlockedRegions` parsed in `structures.go:402–411` | ✅ pokryte przez round-trip parsing | — |
| Write — `core.SetUnlockedRegions` | ✅ `backend/core/writer.go:911` z `RebuildSlot` + rollback | ✅ 4 testy (InMemory, PS4 round-trip, AfterAddItem, PC round-trip) | Atomic via rollback |
| Static DB — `Regions` map | ✅ `backend/db/data/regions.go`, 104 wpisy | ⚠️ pokryte tylko przez `GetAllRegions()` smoke (brak dedicated test) | Snapshot 2026-05-21 |
| App-level toggles | ✅ `app_world.go:186/211/247` (`GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions`) | ⚠️ pokryte pośrednio przez `writer_regions_test.go` | — |
| PvP integration | ✅ `app_pvp.go:47` MatchmakingRegions module — `core.SetUnlockedRegions(slot, allRegions)` | ✅ pokryte przez `pvp_test.go` round-trip | — |
| Frontend | ✅ `WorldTab.tsx` accordion (Invasion Regions) + `RiskSectionBanner` | — | Tier 1 risk banner |
| Empirical fresh-save markers (1001000–1001002, 1800001, 1800090, 6100000) | ⚠️ obserwowane w save, NIE są w `regions.go` DB | ❌ brak dedicated test | `needs verification` przeznaczenia markerów |

**Ostatnia weryfikacja**: 2026-05-21 na `tmp/save/ER0000.sl2` (PC, 5 slotów) + `tmp/save/oisis_pl-org.txt` (PS4) — 4 testy round-trip + Add-Item interaction.

---

## 3. Source of truth w kodzie

| Komponent | Plik / linie | Notatka |
|---|---|---|
| Slot field | `backend/core/structures.go:245–246` | `UnlockedRegionsOffset int`, `UnlockedRegions []uint32` |
| Read parser | `backend/core/structures.go:402–411` | `calculateDynamicOffsets` populates from `gesturesOff` |
| Layout constants | `backend/core/offset_defs.go:97–98` | `DynStorageBox = 0x6010`, `DynStorageToGestures = 0x100` |
| Writer | `backend/core/writer.go:911 SetUnlockedRegions` | Dedup + sort + `RebuildSlot` + rollback |
| Slot rebuilder | `backend/core/slot_rebuild.go::RebuildSlot` | Pełny sekwencyjny serializator |
| Static DB struct | `backend/db/data/regions.go::RegionData` | `{Name, Area}` |
| Static DB map | `backend/db/data/regions.go::Regions` | 104 wpisy (snapshot 2026-05-21) |
| DB API | `backend/db/db.go:1114 GetAllRegions`, `:1116 IsKnownRegionID` | `RegionEntry{ID, Name, Area, ...}` z `sync.OnceValue` cache |
| Entry struct | `backend/db/db.go:126 RegionEntry` | `ID, Name, Area, Unlocked` (Unlocked dodawane przez `GetUnlockedRegions` per-slot) |
| App methods | `app_world.go:186 GetUnlockedRegions`, `:211 SetRegionUnlocked`, `:247 BulkSetUnlockedRegions` | Wails-exposed |
| PvP integration | `app_pvp.go:47 ApplyPvPPreparation` (MatchmakingRegions module) | Uses `GetAllRegions()` + `SetUnlockedRegions(slot, ids)` |
| Tests | `backend/core/writer_regions_test.go:11/48/106/158` | 4 testy |
| Frontend | `frontend/src/components/WorldTab.tsx:2 imports`, `:252/258/264/270/276 handlers` | Accordion + bulk select/clear + risk banner |

---

## 4. Mental model

`UnlockedRegions` jest **prostym wariadycznym arrayem** w slocie:

```
slot.Data
  ├── StorageBox (DynStorageBox = 0x6010 bytes)
  ├── ...gap (DynStorageToGestures = 0x100 bytes — gestures?)...
  ├── gesturesOff                                ← slot.UnlockedRegionsOffset
  │     ├── count : u32 (little-endian)
  │     └── region_id[0..count-1] : u32
  ├── afterRegs (= gesturesOff + 4 + count*4)
  │     ├── (BloodStain + DLC tile coords)       ← L2 (zob. 29)
  │     └── (Fog of War bitfield)                ← L3 (zob. 27 §9)
  └── ...kolejne sekcje...
```

**Implikacja**: dodanie/usunięcie regionu zmienia rozmiar wariadycznej tablicy → zmienia `afterRegs` offset → wszystko **po** `gesturesOff` musi być re-serialized. To dlatego `SetUnlockedRegions` używa `RebuildSlot` (pełny sequential serializer), nie in-place byte-shift.

Region ID jest opaque (gracz widzi tylko obszary, nie ID). Mapping ID → human-readable name + area group jest w `backend/db/data/regions.go`.

---

## 5. Region data model

### 5.1 Binary layout

```
┌────────────────────────────────┐
│ count : u32 (little-endian)    │  4 bytes
├────────────────────────────────┤
│ region_id[0..count-1] : u32    │  count × 4 bytes
└────────────────────────────────┘
Razem: 4 + count*4 bytes
```

### 5.2 Endianness i kolejność

- **Little-endian** (wszystkie pola u32).
- **Kolejność na dysku**: brak gwarancji sortowania — gra przechowuje IDs w kolejności acquisition.
- **Edytor sortuje przy zapisie** ascending (dla stabilnych diffów). Patrz `SetUnlockedRegions` step 2.
- **Duplikaty**: nie obserwowane w żadnym referencyjnym save'ie; `SetUnlockedRegions` deduplikuje prewencyjnie.

### 5.3 RegionData struct (DB)

```go
// backend/db/data/regions.go
type RegionData struct {
    Name string
    Area string  // 11 unikalnych wartości — zob. §5.5
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

### 5.5 Area enum (11 unikalnych wartości w current snapshot)

Wartości pola `Area` z `Regions` map (2026-05-21):

```
"Altus Plateau", "Caelid", "Farum Azula", "Haligtree",
"Land of Shadow", "Legacy Dungeons", "Limgrave", "Liurnia",
"Mountaintops", "Mt. Gelmir", "Underground"
```

⚠️ DLC regions używają `Area: "Land of Shadow"`, **nie** `"DLC"`. Inferred manualnie z er-save-manager + curated; brak strict enum w kodzie — to `string` pole. `GetAllRegions()` sortuje po `Area` (alphabetic) → `Name`.

---

## 6. Region IDs and grouping

### 6.1 Liczba wpisów (snapshot 2026-05-21)

| Grupa | Liczba | Range |
|---|---|---|
| Overworld base game | 62 | 6100000–6800000 (Limgrave / Liurnia / Altus / Caelid / Mountaintops / Mt. Gelmir / Underground / Farum Azula / Haligtree) |
| Overworld DLC (Shadow of the Erdtree) | 7 | 6900000–6900006 (Land of Shadow) |
| Legacy dungeons | 35 | 1000000–1900001 (Stormveil, Leyndell, Roundtable Hold, Academy of Raya Lucaria, Volcano Manor, Stranded Graveyard, Stone Platform / Elden Beast) |
| **Total** | **104** | — |

⚠️ Snapshot 2026-05-21. Po update'cie patcha gry (np. nowy DLC) wymagana re-extracja `regions.go` z er-save-manager / community RE.

### 6.2 Prefix overview

Pełna lista per-prefix (z `Regions` map):

| Prefix | Count | Area (z `RegionData.Area`) | Przykłady |
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
| 6900xxx | 7 | Land of Shadow | DLC (Gravesite Plain, Scadu Altus, Shadow Keep itd.) |

Pełna lista: `backend/db/data/regions.go` (104 named entries).

### 6.3 Empirical fresh-save markers (NIE w DB)

W każdym fresh save (post-character-creation) zaobserwowane region IDs:

```
1001000, 1001001, 1001002  ← wewnętrzne markery startowe (NIE w regions.go)
1800001                    ← Stranded Graveyard (w regions.go)
1800090                    ← Cave of Knowledge (w regions.go)
6100000                    ← The First Step (w regions.go)
```

⚠️ Markery `1001000–1001002` **nie są** zarejestrowane w `Regions` map ani w żadnym referencyjnym edytorze (er-save-manager, ER-Save-Editor). Prawdopodobnie internal startup tokens używane przez engine na poziomie tutorial. `needs verification` ich przeznaczenia. Edytor je zachowuje przy round-trip (nie filtruje), ale `GetAllRegions()` ich nie zwraca → UI nie pokazuje ich jako toggle-able.

### 6.4 Late-game saves

W save'ach z późnego etapu gry (post-elden-beast) zaobserwowano **do 395 wpisów** w `UnlockedRegions` (więcej niż 104 z DB — różnica to sub-region IDs używane przez engine internalnie + invasion area subdivisions). `GetAllRegions()` zwraca tylko 104 znanych „named regions"; pozostałe IDs są zachowywane przez round-trip ale niewidoczne w UI.

---

## 7. Relation to Map Reveal L0

`UnlockedRegions` jest **Layer 0** w 4-warstwowym modelu Map Reveal (patrz [27 — Map Reveal](27-map-reveal.md)).

### 7.1 Co L0 robi

- Włącza szybką podróż w obrębie regionu.
- Oznacza region jako „odwiedzony" dla matchmakingu PvP.
- Wyświetla label „You have entered <X>" po teleporcie.

### 7.2 Co L0 **nie** robi

| Co | Warstwa odpowiedzialna |
|---|---|
| Odkrywa teksturę mapy regionu | L1 (event flags 62xxx) — [27 §7](27-map-reveal.md#7-l1--detailed-bitmap-event-flags--map-fragments) |
| Dodaje fragment mapy do ekwipunku | L1 (`MapFragmentItems`) — [27 §7.3](27-map-reveal.md#73-map-fragment-items) |
| Usuwa czarne kafelki DLC | L2 (Cover Layer) — [29](29-dlc-black-tiles.md) |
| Usuwa mgłę wojny (FoW) | L3 (FoW bitfield) — [27 §9](27-map-reveal.md#9-l3--fog-of-war-removefogofwar) |

Weryfikacja empiryczna: [27 §17 test 1](27-map-reveal.md#19-verification-log-historical) — dodanie region_id (bez flagi 62xxx) → tekstura mapy bez zmian.

### 7.3 Kolejność operacji w teleporcie in-game

Gra po teleporcie do nowego obszaru:

1. Dodaje region ID do `UnlockedRegions` (L0).
2. Ustawia event flag widoczności 62xxx (L1) — tekstura odkrywa się.
3. Ustawia 356 bitów w FoW bitfield (L3) w obszarze 157 bajtów — lokalna mgła znika.

Edytor symuluje to przez `RevealAllMap` (L1 + L2) + `RemoveFogOfWar` (L3), ale L0 (regions) muszą być ustawione osobno przez `SetRegionUnlocked` / `BulkSetUnlockedRegions` / `ApplyPvPPreparation` MatchmakingRegions.

---

## 8. Relation to event flags

`UnlockedRegions` to **niezależna struktura** w slocie, **nie** event flag bitfield. Cross-cutting different storage:

| Aspekt | `UnlockedRegions` (L0) | Event Flags (L1) |
|---|---|---|
| Storage | Wariadyczny `[]uint32` w `gesturesOff` | Stały bitfield `0x1BF99F` bytes w `EventFlagsBlock` |
| Helper API | `core.SetUnlockedRegions` (slot-level, full rebuild) | `db.SetEventFlag` (bit-level, in-place) |
| Atomicity | Atomic via rollback (snapshot prev state + slot.Data) | Stateless bit mutation, no rollback |
| Sortowanie | Dedup + sort ascending przy zapisie | n/d |

Generic event flag fundament (bit/byte indexing, BST resolver, helper API, bounds check) → [15 — Event Flags](15-event-flags.md).

---

## 9. Current implemented behavior

### 9.1 Wails-exposed API (`app_world.go`)

| Endpoint | Sygnatura | Cel |
|---|---|---|
| `GetUnlockedRegions(slotIdx)` | `([]db.RegionEntry, error)` | Read all 104 DB regions z `Unlocked` bool per-slot |
| `SetRegionUnlocked(slotIdx, regionID, unlocked)` | `error` | Toggle pojedynczy region (add/remove + RebuildSlot) |
| `BulkSetUnlockedRegions(slotIdx, regionIDs)` | `error` | Replace full list (idempotent) |

### 9.2 Internal (`backend/core`)

| Funkcja | Sygnatura | Notatka |
|---|---|---|
| `SetUnlockedRegions(slot, ids)` | `error` | Dedup + sort + `RebuildSlot` + rollback |
| `RebuildSlot(slot)` | `([]byte, error)` | Pełny sekwencyjny serializer slotu (zob. [30](30-slot-rebuild-research.md)) |

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

Skutek: wszystkie 104 znanych regionów (62 + 7 + 35) unlocked dla PvP matchmaking.

### 9.4 Frontend integration

`WorldTab.tsx` accordion „Invasion Regions":

- Per-region checkbox → `SetRegionUnlocked(charIdx, r.id, unlocked)` (linia 252)
- „Select All in area" → `BulkSetUnlockedRegions(charIdx, next)` (linie 258, 264)
- „Unlock All" → `BulkSetUnlockedRegions(charIdx, regionEntries.map(r => r.id))` (linia 270)
- „Reset" → `BulkSetUnlockedRegions(charIdx, [])` (linia 276)
- `RiskSectionBanner` w nagłówku accordion (zob. [32](32-ban-risk-system.md))

---

## 10. Generated data and snapshot caveats

### 10.1 Origin

`backend/db/data/regions.go` jest **statyczny katalog generated offline** z [er-save-manager](https://github.com/Jeius/er-save-manager) — community-researched IDs. Source comment w pliku:

> `Source: er-save-manager/src/er_save_manager/data/regions.py — community-researched IDs.`

### 10.2 Snapshot date

**2026-05-21** — 104 wpisy. Po update'cie patcha gry (np. nowy DLC, balansowanie areas) wymagana re-extracja. Brak automatycznego diff vs `regulation.bin`.

### 10.3 Co może się zmienić po patchu

- **Nowe DLC** → nowe IDs w range 69xxxxx lub innym.
- **Re-balancing areas** → możliwe nowe sub-region IDs (np. dodatkowe Stormveil sub-zones).
- **Nieznane** IDs zwracane jako `unknown` przez `IsKnownRegionID(id)` → edytor zachowa je w round-trip, ale nie pokaże w UI.

### 10.4 Inferred grupowanie Area

Pole `Area` w `RegionData` jest **manualnie skuratowane** — 11 unikalnych wartości w current snapshot (zob. §5.5).

⚠️ Inferred — nie z save format. `needs verification` że każdy region jest zakwalifikowany do właściwego area-grupy (UI grupuje regiony per area w accordion). DLC regions używają `"Land of Shadow"`, nie `"DLC"`.

---

## 11. Validation and rollback caveats

### 11.1 `core.SetUnlockedRegions` pipeline

```
1. calculateDynamicOffsets()       — refresh offsets (z stale slot.Data)
2. buildSectionMap()                — refresh section map
3. dedup + sort ascending           — stable output
4. snapshot prev (UnlockedRegions, slot.Data)
5. RebuildSlot(slot)                — pełny rebuild
    └── on error → restore prev (slot.UnlockedRegions, slot.Data)
6. recalc offsets                   — slot.calculateDynamicOffsets
    └── on error → restore prev (slot.UnlockedRegions, slot.Data)
7. recalc SectionMap                — slot.buildSectionMap
    └── on error → append warning (non-fatal)
```

### 11.2 Krytyczna prerefresha

Komentarz w `writer.go:911`:

> Other writers (`AddItemsToSlot`, `FlushGaItems`, `revealDLCMap`, …) mutate `slot.Data` without updating `slot.UnlockedRegionsOffset` or `SectionMap`, so without this refresh we would rebuild from stale boundaries and produce a corrupted save (observed when the user added an item, then revealed the map, then unlocked regions — slot 4 of ER0000.sl2 corrupted at the regCount offset).

⚠️ Implikacja: wywołanie `SetUnlockedRegions` po `AddItemsToSlot` / `revealDLCMap` bez `calculateDynamicOffsets` → corrupted save. Aktualny kod robi tę refresh internalnie. Test `TestSetUnlockedRegionsAfterAddItem` weryfikuje ten case.

### 11.3 Rollback granularity

- **Pełen rollback** dla `slot.Data` + `slot.UnlockedRegions` (snapshot na początku).
- **Warning-only rollback** dla `SectionMap` rebuild (non-fatal — kontynuuje z stale section map).

### 11.4 Stress test

R-1 step 17: testowane do ~100 000 regionów w syntetycznych testach obciążeniowych. Po rebuild końc danych ~2.2 MB wgłąb slotu o `SlotSize = 0x280000`, pozostawiając 408–432 KB zerowego paddingu. **Brak ryzyka obcięcia** dla realistycznych zakresów (`Unlock All` dodaje 104 regiony, fresh save ma 6).

### 11.5 Historical: usunięta byte-shift path

Stara implementacja (Stage-1 invasion-regions feature) wstawiała region IDs in-place przesuwając resztę slotu. Limit „max 10–20 regionów" wynikał z brakującego slack na końcu slotu — przekroczenie crashowało save (obcięcie hash region). **Usunięta** w R-1 Step 14 (patrz CHANGELOG entry „feature/invasion-regions — Stage 2"). `SetUnlockedRegions` z `RebuildSlot` jest jedynym wspieranym entry point.

---

## 12. Safety notes

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| V1 | Stale `regions.go` po patchu gry | ⚠️ medium | `needs verification` po każdym DLC/patch update; brak automatycznego diff vs `regulation.bin`. |
| V2 | `Unlock All` (104 regiony) na non-DLC save → DLC IDs 69xxxxx w save | ⚠️ low | Gra toleruje (safe), ale logicznie niepoprawne. UI mogłoby warn przy DLC IDs — `needs verification` czy ostrzega. |
| V3 | Empirical fresh-save markers 1001000–1001002 nie w DB | ⚠️ medium | Round-trip zachowuje, ale `GetAllRegions()` nie zwraca → UI nie pokazuje. `needs verification` przeznaczenia. |
| V4 | Setting region bez L1 (visible flag) → fast travel ok, ale mapa pusta | ⚠️ low | Po user-side: musi też wywołać `RevealAllMap` lub `SetMapRegionFlags` (L1). |
| V5 | `BulkSetUnlockedRegions([])` → utrata wszystkich regionów (no fast travel) | 🔴 high | Atomic via rollback w `SetUnlockedRegions`; UI snapshot-undo (`pushUndo`) jest workaround. `RiskSectionBanner` ostrzega. |
| V6 | `SetUnlockedRegions` po `AddItemsToSlot` bez refresh offsets | 🔴 high | Już rozwiązane w kodzie (linia 911 z refresh przed rebuild). Test `TestSetUnlockedRegionsAfterAddItem` weryfikuje. |
| V7 | `RebuildSlot` corrupted slot na large dataset (>100k regions) | ⚠️ low | Stress-tested do ~100k; `Unlock All` użytkownika to 104. Brak realnego ryzyka. |
| V8 | Late-game save z ~395 wpisami zawiera nieznane sub-region IDs | ⚠️ medium | Round-trip zachowuje, ale UI nie pokazuje. `needs verification` czy są writeable bez side effects. |
| V9 | DLC regions 69xxxxx na non-DLC save → save nadal poprawny ale UI mismatch | ⚠️ low | Patrz V2 (duplicate). |
| V10 | `SectionMap` rebuild fail jest non-fatal | ⚠️ low | Append warning + kontynuuje z stale section map. `needs verification` czy stale section map nie powoduje innych issues w kolejnych mutacjach. |

---

## 13. Test coverage

| Test | Cel | Plik |
|---|---|---|
| `TestSetUnlockedRegionsInMemory` | In-memory mutation (dedup + sort + recalc) | `backend/core/writer_regions_test.go:11` |
| `TestSetUnlockedRegionsRoundTripPS4` | PS4 round-trip (read → mutate → write → read → byte-equal mutated state) | `backend/core/writer_regions_test.go:48` |
| `TestSetUnlockedRegionsAfterAddItem` | Interaction z `AddItemsToSlot` — verify refresh offsets path | `backend/core/writer_regions_test.go:106` |
| `TestSetUnlockedRegionsRoundTripPC` | PC round-trip jak PS4 | `backend/core/writer_regions_test.go:158` |

Brak dedicated test:

- `BulkSetUnlockedRegions([])` (empty replace — V5 risk).
- DLC region 69xxxxx na non-DLC save (V2).
- 1001000–1001002 markery zachowanie przez round-trip (V3).
- PS4↔PC cross-platform conversion test (`TestConvert*` nie istnieje).
- Late-game save z ~395 wpisami (V8).

---

## 14. Known limits / needs verification

| # | Limit / gap | Status | Notatka |
|---|---|---|---|
| L1 | Snapshot 2026-05-21 (104 wpisy) | ⚠️ stale po patchu | Re-extracja z er-save-manager wymagana po update gry. |
| L2 | Fresh-save markers 1001000–1001002 | ❓ przeznaczenie nieznane | `needs verification`. Round-trip OK, UI nie pokazuje. |
| L3 | Late-game saves z ~395 wpisami zawierają sub-region IDs poza DB | ⚠️ partial coverage | `GetAllRegions()` zwraca 104; reszta zachowana ale niewidoczna. |
| L4 | DLC region ownership cross-check | ❌ brak | Edytor nie sprawdza czy save posiada DLC przed ustawieniem 69xxxxx. UI mogłoby warn. |
| L5 | PS4↔PC cross-platform bit-equal | ❌ brak `TestConvert*` | Round-trip per-platforma OK; brak testu „PS4 → save → convert → load PC → identyczne IDs". |
| L6 | `Area` grouping inferred manualnie | ⚠️ snapshot | Możliwa rozbieżność po dodaniu nowych regions (np. DLC area niezakwalifikowane). |
| L7 | `SectionMap` rebuild non-fatal | ⚠️ design choice | Stale section map może powodować subtelne bugi w kolejnych mutacjach — `needs verification`. |
| L8 | Sub-region IDs (np. 6101000 vs 6100000) | ⚠️ semantyka niejasna | Engine używa internalnie sub-areas (Stormhill jest sub-area Limgrave). Mapping subarea-do-parent nie zaimplementowany. |
| L9 | Empty `BulkSetUnlockedRegions([])` | ❌ brak guard | Możliwy use case „reset all regions" — atomic via rollback, ale brak warningu UI. |
| L10 | `RegionData` source pochodzi z er-save-manager | ⚠️ dependency | Jeśli upstream errors / removes IDs, nasza baza może być rozbieżna. Brak CI check. |

---

## 15. Cross-references

| Topic | Master rozdział |
|---|---|
| 4-warstwowy model Map Reveal (regions = L0) | [27 — Map Reveal](27-map-reveal.md) |
| Event flag fundament (bit/byte indexing, helper API, BST) | [15 — Event Flags](15-event-flags.md) |
| DLC Cover Layer (L2 w warstwach mapy — niezależna od L0) | [29 — DLC Black Tiles](29-dlc-black-tiles.md) |
| Sites of Grace (related — fast travel between graces) | [47 — Sites of Grace](47-site-of-grace-activation.md) |
| World state (FieldArea, WorldArea — read-only, brak związku z L0) | [16 — World State](16-world-state.md) |
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
- `backend/core/slot_rebuild.go::RebuildSlot` — pełny sequential serializer
- `backend/db/data/regions.go` — 104 wpisy + `RegionData` struct (snapshot 2026-05-21)
- `backend/db/db.go:126 RegionEntry`, `:1114 GetAllRegions`, `:1116 IsKnownRegionID`
- `app_world.go:186/211/247` — `GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions`
- `app_pvp.go:47` — `ApplyPvPPreparation` MatchmakingRegions module
- `frontend/src/components/WorldTab.tsx` — UI accordion + handlers

### Tests

- `backend/core/writer_regions_test.go` — 4 testy (InMemory, RoundTripPS4, AfterAddItem, RoundTripPC)
- `tests/pvp_test.go` — pokrywa PvP MatchmakingRegions path

### Reference parsers / community

- er-save-manager (Python): `parser/world.py::Regions` + `data/regions.py` — community-researched IDs (upstream source)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — length-prefixed list reference

### Hex-verified saves

- `tmp/save/ER0000.sl2` (PC, 5 slotów) — round-trip 2026-05-21
- `tmp/save/oisis_pl-org.txt` (PS4) — round-trip 2026-05-21
