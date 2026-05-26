# 11 — Regions (UnlockedRegions — szybka podróż + invasion eligibility)

> **Typ**: Binary format spec + region database reference
> **Status**: ✅ canonical (2026-05-26 — curated invasion allowlist, 274 wpisy)
> **Zakres**: Detail rozdział dla L0 warstwy z [27 — Map Reveal](27-map-reveal.md). Opisuje binary layout `UnlockedRegions` array, write path przez `core.SetUnlockedRegions` (z `RebuildSlot`), zawartość `backend/db/data/regions.go` (**274 skuratowanych regionów invasion/blue** snapshot 2026-05-26), oraz integracje (`App.SetRegionUnlocked`, `App.BulkSetUnlockedRegions`, `ApplyPvPPreparation` matchmaking module).
>
> **`MatchmakingRegions` to skuratowana allowlista, nie cały świat.** `backend/db/data/regions.go` jest podzbiorem wierszy `PlayRegionParam` z regulation.bin, potwierdzonym jako standardowe cele invasion / blue-summon, pochodzącym 1:1 z dedykowanej listy Elden-Ring-CT-TGA "Invasion Regions". Realny Row ID z `PlayRegionParam` **nie** jest automatycznie legalnym celem PvP: huby multiplayer (Roundtable Hold w ogóle nie ma wiersza `PlayRegion`), kolosea (osobny matchmaking) oraz wewnętrzne/network-only wiersze sub-obszarów są celowo wykluczone. "Unlock All" odblokowuje każdy **zweryfikowany legalny** region PvP — nigdy "PvP wszędzie" — i teraz **zachowuje wszelkie nieskuratowane surowe region IDs** już obecne w save (zob. §9.1).

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
- `RegionData` static DB (`backend/db/data/regions.go`, 274 skuratowane wpisy snapshot 2026-05-26)
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
| Static DB — `Regions` map | ✅ `backend/db/data/regions.go`, 274 skuratowane wpisy (208 base + 66 DLC) | ✅ `backend/db/regions_test.go` (liczniki, mapowanie DLC, brak sfabrykowanych IDs, brak forbidden locations) | Snapshot 2026-05-26 |
| App-level toggles | ✅ `app_world.go:186/211/247` (`GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions`) | ✅ `app_world_regions_test.go` (preservation nieskuratowanych surowych IDs) + pośrednio `writer_regions_test.go` | Operacje grupowe zachowują nieskuratowane surowe IDs |
| PvP integration | ✅ `app_pvp.go:47` MatchmakingRegions module — `core.SetUnlockedRegions(slot, allRegions)` | ✅ pokryte przez `pvp_test.go` round-trip | — |
| Frontend | ✅ `WorldTab.tsx` accordion (Invasion Regions) + `RiskSectionBanner` | — | Tier 1 risk banner |
| Empirical fresh-save markers (1001000–1001002, 1800001, 1800090, 6100000) | ⚠️ obserwowane w save, NIE są w skuratowanym `regions.go` | ✅ `app_world_regions_test.go` używa 1001000 jako fixture preservation | Nieskuratowane surowe IDs zachowywane przez każdą operację grupową |

**Ostatnia weryfikacja**: 2026-05-26 — dane regionów zwalidowane wobec `tmp/regulation-bin-dump/csv/PlayRegionParam.csv`; round-trip na `tmp/save/ER0000.sl2` (PC) + `tmp/save/oisis_pl-org.txt` (PS4).

---

## 3. Source of truth w kodzie

| Komponent | Plik / linie | Notatka |
|---|---|---|
| Slot field | `backend/core/structures.go:245–246` | `UnlockedRegionsOffset int`, `UnlockedRegions []uint32` |
| Read parser | `backend/core/structures.go:402–411` | `calculateDynamicOffsets` populates from `gesturesOff` |
| Layout constants | `backend/core/offset_defs.go:97–98` | `DynStorageBox = 0x6010`, `DynStorageToGestures = 0x100` |
| Writer | `backend/core/writer.go:911 SetUnlockedRegions` | Dedup + sort + `RebuildSlot` + rollback |
| Slot rebuilder | `backend/core/slot_rebuild.go::RebuildSlot` | Pełny sekwencyjny serializator |
| Static DB struct | `backend/db/data/regions.go::RegionData` | `{Name, Area, DLC}` |
| Static DB map | `backend/db/data/regions.go::Regions` | 274 skuratowane wpisy (208 base + 66 DLC, snapshot 2026-05-26) |
| DB API | `backend/db/db.go:1114 GetAllRegions`, `:1116 IsKnownRegionID` | `RegionEntry{ID, Name, Area, ...}` z `sync.OnceValue` cache; `IsKnownRegionID` = członkostwo w skuratowanej allowliście |
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
    Area string  // 13 unikalnych wartości — zob. §5.5
    DLC  bool     // true dla regionów Shadow of the Erdtree (nieciągła przestrzeń ID)
}

var Regions = map[uint32]RegionData{
    6100000: {Name: "The First Step", Area: "Limgrave"},
    6800000: {Name: "Gravesite Plain", Area: "Land of Shadow", DLC: true},
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

### 5.5 Area enum (13 unikalnych wartości w current snapshot)

Wartości pola `Area` z `Regions` map (2026-05-26):

```
"Altus Plateau", "Caelid", "Catacombs, Caves & Tunnels", "Farum Azula",
"Haligtree", "Land of Shadow", "Land of Shadow — Dungeons",
"Legacy Dungeons", "Limgrave", "Liurnia", "Mountaintops",
"Mt. Gelmir", "Underground"
```

⚠️ DLC regions używają `Area: "Land of Shadow"` (overworld) lub `"Land of Shadow — Dungeons"`, **nie** `"DLC"` — marker DLC niesie pole `DLC bool`. Obszary skuratowane z kolumny `Map` listy TGA; brak strict enum w kodzie — to `string` pole. `GetAllRegions()` sortuje po `Area` (alphabetic) → `Name`.

---

## 6. Region IDs and grouping

### 6.1 Liczba wpisów (snapshot 2026-05-26)

| Grupa | Liczba |
|---|---|
| Base game | 208 |
| DLC (Shadow of the Erdtree) | 66 |
| **Total (skuratowana allowlista)** | **274** |

Każde ID jest realnym Row ID z `PlayRegionParam` **oraz** obecne 1:1 na liście TGA "Invasion Regions". To skuratowany podzbiór invasion/blue — nie pełna 594-wierszowa tabela `PlayRegionParam`.

⚠️ Snapshot 2026-05-26. Po patchu gry / nowym DLC zwaliduj ponownie `regions.go` wobec `regulation.bin` `PlayRegionParam` + listy TGA (`tmp/scripts/gen_regions.py` + `tmp/scripts/validate_regions.py`).

### 6.2 Area overview

Liczniki per-area (z `Regions` map, sortowane po `Area`):

| Area | Count | DLC | Uwagi |
|---|---|---|---|
| Altus Plateau | 9 | — | Altus overworld + Capital Outskirts |
| Caelid | 9 | — | Caelid + Dragonbarrow + Swamp of Aeonia |
| Catacombs, Caves & Tunnels | 63 | — | Bazowe minor dungeons (3xxxxxx) |
| Farum Azula | 10 | — | Crumbling Farum Azula |
| Haligtree | 7 | — | Miquella's Haligtree (w tym Promenade / Town Plaza — czołowe strefy inwazji) |
| Land of Shadow | 27 | ✅ | DLC overworld (Gravesite Plain 6800000, Scadu Altus 6900000 itd.) |
| Land of Shadow — Dungeons | 39 | ✅ | DLC legacy dungeons / gaols / forges / catacombs (2xxxxxx, 4xxxxxx) |
| Legacy Dungeons | 40 | — | Stormveil, Leyndell, Raya Lucaria, Volcano Manor itd. |
| Limgrave | 13 | — | The First Step, Weeping Peninsula, Stormhill |
| Liurnia | 13 | — | Lake-facing, Caria Manor, Bellum, Moonlight Altar |
| Mountaintops | 10 | — | Mountaintops + Consecrated Snowfield |
| Mt. Gelmir | 3 | — | Road of Iniquity, Ninth Mt. Gelmir Campsite |
| Underground | 31 | — | Siofra, Ainsel, Deeproot, Mohgwyn, Lake of Rot |

> **Regiony `isBoss` są zachowane.** Lista TGA taguje część regionów jako `isBoss` ("regiony *przed* walkami z bossami i fog walls"). To legalne konteksty inwazji — kilka z nich to najaktywniejsze strefy PvP w grze (np. Haligtree Promenade / Town Plaza). **Nie są** to wnętrza aren z wyłączonym multiplayer i **nie** są usuwane. Faktycznie zakazane lokacje (Roundtable Hold, kolosea) nie mają wiersza `PlayRegion` i są nieobecne z konstrukcji.

Pełna lista: `backend/db/data/regions.go` (274 skuratowane wpisy).

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

W save'ach z późnego etapu gry (post-elden-beast) zaobserwowano **do ~395 wpisów** w `UnlockedRegions` (więcej niż 274 skuratowane regiony — różnica to wewnętrzne sub-region IDs + invasion area subdivisions z `PlayRegionParam`, celowo poza skuratowaną allowlistą). `GetAllRegions()` zwraca tylko 274 skuratowane „named regions"; pozostałe surowe IDs są niewidoczne w UI **i teraz zachowywane przez każdą operację grupową** (Unlock All / Lock All / per-area), nie tylko przez round-trip — zob. §9.1.

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
| `GetUnlockedRegions(slotIdx)` | `([]db.RegionEntry, error)` | Czyta wszystkie 274 skuratowane regiony z `Unlocked` bool per-slot. Nieskuratowane surowe IDs nie są pokazywane. |
| `SetRegionUnlocked(slotIdx, regionID, unlocked)` | `error` | Toggle pojedynczy region; operuje na surowej liście (add/remove jednego ID), więc nieskuratowane surowe IDs są z natury zachowane. |
| `BulkSetUnlockedRegions(slotIdx, regionIDs)` | `error` | Ustawia **skuratowane** członkostwo na `regionIDs`, zachowując każde surowe ID spoza skuratowanej allowlisty (`mergeUnlockedRegions`). |

**Niedestrukcyjna semantyka zbiorów** (`mergeUnlockedRegions` w `app_world.go`):

```
result = regionIDs  ∪  { surowe ID ∈ slot.UnlockedRegions : !db.IsKnownRegionID(id) }
```

World tab przekazuje wyłącznie skuratowane IDs, więc bez tego bulk-replace po cichu skasowałby ~120 nieskuratowanych surowych IDs, które niesie late-game save DLC. Konkretnie:

| Akcja UI | Przekazane `regionIDs` | Efektywny wynik |
|---|---|---|
| Unlock All | pełna skuratowana allowlista | `existingRaw ∪ curatedAllowlist` |
| Lock All | `[]` | `existingRaw − curatedAllowlist` (nieskuratowane surowe zachowane) |
| Unlock area | skuratowane-unlocked ∪ area | nieskuratowane surowe zawsze zachowane |
| Lock area | skuratowane-unlocked − area | nieskuratowane surowe zawsze zachowane |

`core.SetUnlockedRegions` deduplikuje + sortuje scalony wynik.

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
    // Niedestrukcyjne: zachowaj surowe IDs spoza skuratowanej allowlisty.
    if err := core.SetUnlockedRegions(slot, mergeUnlockedRegions(ids, slot.UnlockedRegions)); err != nil {
        return nil, fmt.Errorf("matchmaking regions: %w", err)
    }
}
```

Skutek: wszystkie 274 skuratowane regiony (208 base + 66 DLC) unlocked dla PvP matchmaking, przy zachowaniu wszelkich nieskuratowanych surowych region IDs już obecnych w save. To "Unlock All" przez preset — **nie** włącza PvP wszędzie, tylko zweryfikowane legalne regiony invasion/blue.

### 9.4 Frontend integration

`WorldTab.tsx` accordion „Invasion Regions":

- Per-region checkbox → `SetRegionUnlocked(charIdx, r.id, unlocked)` (linia 252) — add/remove na surowej liście, zachowuje resztę
- Per-area „+ / −" → `BulkSetUnlockedRegions(charIdx, next)` (linie 258, 264) — nieskuratowane surowe zachowane przez merge w backendzie
- „Unlock All" → `BulkSetUnlockedRegions(charIdx, regionEntries.map(r => r.id))` (linia 270) → `existingRaw ∪ curatedAllowlist`
- „Lock All" → `BulkSetUnlockedRegions(charIdx, [])` (linia 276) → `existingRaw − curatedAllowlist` (**nie** kasuje nieskuratowanych surowych IDs)
- `RiskSectionBanner` w nagłówku accordion (zob. [32](32-ban-risk-system.md))

Handlery frontendu są bez zmian; gwarancja preservation żyje w całości w backendowym `mergeUnlockedRegions` (§9.1), więc UI nie może przypadkiem skasować surowych IDs, których nigdy nie widzi.

---

## 10. Generated data and snapshot caveats

### 10.1 Origin

`backend/db/data/regions.go` jest **statyczny katalog generated offline** przez `tmp/scripts/gen_regions.py`, który scala dedykowaną listę [Elden-Ring-CT-TGA](https://github.com/Dasaav-dsv/Elden-Ring-CT-TGA) "Invasion Regions" (Dasaav; DLC by Joel/SeriouslyCasual) z istniejącymi skuratowanymi nazwami, a następnie **twardo filtruje każde ID wobec `regulation.bin` `PlayRegionParam`** (`tmp/regulation-bin-dump/csv/PlayRegionParam.csv`), więc przeżywają tylko realne Row IDs danych gry. Etykiety 6800000/6900000 są poprawione z `PlaceName_dlc01` FMG + `BonfireWarpParam`.

### 10.2 Snapshot date

**2026-05-26** — 274 skuratowane wpisy (208 base + 66 DLC). Po patchu gry / nowym DLC uruchom ponownie `gen_regions.py` + `validate_regions.py` aby zwalidować wobec `regulation.bin`. Brak automatycznego CI diff.

### 10.3 Co może się zmienić po patchu

- **Nowe DLC** → nowe IDs w range 69xxxxx lub innym.
- **Re-balancing areas** → możliwe nowe sub-region IDs (np. dodatkowe Stormveil sub-zones).
- **Nieznane** IDs zwracane jako `unknown` przez `IsKnownRegionID(id)` → edytor zachowa je w round-trip, ale nie pokaże w UI.

### 10.4 Inferred grupowanie Area

Pole `Area` w `RegionData` jest **manualnie skuratowane** — 13 unikalnych wartości w current snapshot (zob. §5.5).

⚠️ Inferred — nie z save format. `needs verification` że każdy region jest zakwalifikowany do właściwego area-grupy (UI grupuje regiony per area w accordion). DLC regions używają `"Land of Shadow"` / `"Land of Shadow — Dungeons"`, nie `"DLC"`.

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

R-1 step 17: testowane do ~100 000 regionów w syntetycznych testach obciążeniowych. Po rebuild końc danych ~2.2 MB wgłąb slotu o `SlotSize = 0x280000`, pozostawiając 408–432 KB zerowego paddingu. **Brak ryzyka obcięcia** dla realistycznych zakresów (`Unlock All` ustawia 274 skuratowane regiony plus zachowane surowe IDs, fresh save ma 6).

### 11.5 Historical: usunięta byte-shift path

Stara implementacja (Stage-1 invasion-regions feature) wstawiała region IDs in-place przesuwając resztę slotu. Limit „max 10–20 regionów" wynikał z brakującego slack na końcu slotu — przekroczenie crashowało save (obcięcie hash region). **Usunięta** w R-1 Step 14 (patrz CHANGELOG entry „feature/invasion-regions — Stage 2"). `SetUnlockedRegions` z `RebuildSlot` jest jedynym wspieranym entry point.

---

## 12. Safety notes

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| V1 | Stale `regions.go` po patchu gry | ⚠️ medium | `needs verification` po każdym DLC/patch update; brak automatycznego diff vs `regulation.bin`. |
| V2 | `Unlock All` (274 skuratowane regiony) na non-DLC save → DLC IDs w save | ⚠️ low | Gra toleruje (safe), ale logicznie niepoprawne na save bez DLC. `IsDLCRegion(id)` umożliwia przyszły warning w UI. |
| V3 | Empirical fresh-save markers 1001000–1001002 nie w skuratowanym DB | ✅ zmitygowane | Zachowane przez round-trip **i** każdą operację grupową (`mergeUnlockedRegions`); `GetAllRegions()` ich nie pokazuje, więc UI ich nie toggle'uje. |
| V4 | Setting region bez L1 (visible flag) → fast travel ok, ale mapa pusta | ⚠️ low | Po user-side: musi też wywołać `RevealAllMap` lub `SetMapRegionFlags` (L1). |
| V5 | „Lock All" → utrata wszystkich regionów (no fast travel) | ✅ zmitygowane | „Lock All" usuwa teraz tylko skuratowaną allowlistę (`existingRaw − curated`); nieskuratowane surowe IDs zachowane. Nadal atomic via rollback + `pushUndo`; `RiskSectionBanner` ostrzega. |
| V6 | `SetUnlockedRegions` po `AddItemsToSlot` bez refresh offsets | 🔴 high | Już rozwiązane w kodzie (linia 911 z refresh przed rebuild). Test `TestSetUnlockedRegionsAfterAddItem` weryfikuje. |
| V7 | `RebuildSlot` corrupted slot na large dataset (>100k regions) | ⚠️ low | Stress-tested do ~100k; `Unlock All` użytkownika to 274 skuratowane + zachowane surowe. Brak realnego ryzyka. |
| V8 | Late-game save z ~395 wpisami zawiera nieznane sub-region IDs | ✅ zmitygowane | Zachowane przez round-trip **i** każdą operację grupową (`mergeUnlockedRegions`); UI nie pokazuje. |
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
| `TestRegionsCompleteness` / `TestRegionsKeyDLCPresent` / `TestRegionConflictResolved` | Liczniki skuratowanej allowlisty (274/208/66), kluczowe DLC IDs, mapowanie 6800000/6900000 | `backend/db/regions_test.go` |
| `TestRegionsNoFabricatedIDs` / `TestNoForbiddenPvPLocations` | Brak sfabrykowanych legacy IDs; brak nazw hubów/koloseów | `backend/db/regions_test.go` |
| `TestUnlockAllPreservesNonCuratedRaw` / `TestLockAllPreservesNonCuratedRaw` / `TestPerAreaTogglePreservesNonCuratedRaw` | Operacje grupowe zachowują nieskuratowane surowe IDs (fixture 1001000) | `app_world_regions_test.go` |

Brak dedicated test:

- DLC region na non-DLC save (V2).
- PS4↔PC cross-platform conversion test (`TestConvert*` nie istnieje).
- End-to-end preservation przez `App.BulkSetUnlockedRegions` z realnie wczytanym save (czysta `mergeUnlockedRegions` jest pokryta; wrapper App pośrednio).

---

## 14. Known limits / needs verification

| # | Limit / gap | Status | Notatka |
|---|---|---|---|
| L1 | Snapshot 2026-05-26 (274 skuratowane wpisy) | ⚠️ stale po patchu | Uruchom `gen_regions.py` + `validate_regions.py` wobec `regulation.bin` po update gry. |
| L2 | Fresh-save markers 1001000–1001002 | ❓ przeznaczenie nieznane | `needs verification` przeznaczenia. Zachowane przez round-trip + operacje grupowe; UI nie pokazuje. |
| L3 | Late-game saves z ~395 wpisami zawierają sub-region IDs poza skuratowanym DB | ✅ zachowane | `GetAllRegions()` zwraca 274 skuratowane; reszta zachowana przez round-trip **i** operacje grupowe, tylko niewidoczna w UI. |
| L4 | DLC region ownership cross-check | ❌ brak | Edytor nie sprawdza czy save posiada DLC przed ustawieniem 69xxxxx. UI mogłoby warn. |
| L5 | PS4↔PC cross-platform bit-equal | ❌ brak `TestConvert*` | Round-trip per-platforma OK; brak testu „PS4 → save → convert → load PC → identyczne IDs". |
| L6 | `Area` grouping inferred manualnie | ⚠️ snapshot | Możliwa rozbieżność po dodaniu nowych regions (np. DLC area niezakwalifikowane). |
| L7 | `SectionMap` rebuild non-fatal | ⚠️ design choice | Stale section map może powodować subtelne bugi w kolejnych mutacjach — `needs verification`. |
| L8 | Sub-region IDs (np. 6101000 vs 6100000) | ⚠️ semantyka niejasna | Engine używa internalnie sub-areas (Stormhill jest sub-area Limgrave). Mapping subarea-do-parent nie zaimplementowany. |
| L9 | Empty `BulkSetUnlockedRegions([])` | ❌ brak guard | Możliwy use case „reset all regions" — atomic via rollback, ale brak warningu UI. |
| L10 | `RegionData` źródłem jest lista TGA filtrowana wobec `PlayRegionParam` | ⚠️ dependency | Jeśli lista TGA lub dump `regulation.bin` się zmieni, uruchom ponownie generator. Brak CI check. |

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
- `backend/db/data/regions.go` — 274 skuratowane wpisy + `RegionData{Name, Area, DLC}` (snapshot 2026-05-26)
- `backend/db/db.go:126 RegionEntry`, `:1114 GetAllRegions`, `:1116 IsKnownRegionID` (członkostwo w skuratowanej allowliście)
- `app_world.go:186/211/247` — `GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions` + `mergeUnlockedRegions` (preservation)
- `app_pvp.go:47` — `ApplyPvPPreparation` MatchmakingRegions module (też preserving)
- `frontend/src/components/WorldTab.tsx` — UI accordion + handlers (bez zmian)

### Tests

- `backend/core/writer_regions_test.go` — 4 testy (InMemory, RoundTripPS4, AfterAddItem, RoundTripPC)
- `backend/db/regions_test.go` — liczniki skuratowane, mapowanie DLC, brak sfabrykowanych IDs, brak forbidden locations
- `app_world_regions_test.go` — preservation nieskuratowanych surowych IDs dla Unlock All / Lock All / per-area
- `tests/pvp_test.go` — pokrywa PvP MatchmakingRegions path

### Game data + reference (authoritative)

- `tmp/regulation-bin-dump/csv/PlayRegionParam.csv` — autorytatywna przestrzeń regionów (594 wiersze); skuratowane DB to podzbiór invasion/blue
- `tmp/regulation-bin-dump/msg/.../PlaceName_dlc01.fmg.json` — 680000 "Gravesite Plain", 690000 "Scadu Altus"
- Elden-Ring-CT-TGA "Invasion Regions" (Dasaav; DLC by Joel/SeriouslyCasual) — dedykowana lista invasion-targeting (open-world / dungeon / boss)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — length-prefixed list reference

### Hex-verified saves

- `tmp/save/ER0000.sl2` (PC, 5 slotów) — round-trip
- `tmp/save/oisis_pl-org.txt` (PS4) — round-trip
