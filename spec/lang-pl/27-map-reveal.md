# 27 — Odkrywanie mapy (4-warstwowy model)

> **Typ**: Binary format spec + endpoint reference
> **Status**: ✅ canonical (Phase 4 Step 2 — 2026-05-21)
> **Zakres**: Master rozdział dla map reveal. Definiuje 4-warstwowy model (regiony, detailed bitmap, DLC cover layer, fog of war) i wszystkie endpointy `App.*Map*` / `App.*FogOfWar` w SaveForge. Szczegóły domenowe (UnlockedRegions layout, DLC tile coords, event flag bit-order) są w pochodnych rozdziałach — patrz §18 Cross-references.

---

## 1. Cel rozdziału

Mapa w Elden Ring to **stos czterech niezależnych warstw** — każda kontrolowana innym mechanizmem w pliku save. „Odkryć mapę dla gracza" wymaga modyfikacji co najmniej warstw 1 i 2; warstwa 0 (regiony) odpowiada za szybką podróż; warstwa 3 (fog of war) jest kosmetyczna i eksponowana osobno.

Ten rozdział opisuje:

- 4-warstwowy mental model (L0–L3)
- co każda warstwa robi i gdzie jest przechowywana
- wszystkie endpointy `App.*Map*` / `App.*FogOfWar` w `app_world.go`
- write path każdej warstwy + caveaty rollback
- ban-risk Tier 1 (UI integration)
- test coverage + `needs verification` items

Ten rozdział **nie** powiela:

- bit/byte indexing event flags → [15](15-event-flags.md)
- binary layout `UnlockedRegions` array → [11](11-regions.md)
- DLC tile coordinate detail → [29](29-dlc-black-tiles.md)
- companion item / fragment add-pickup semantics → [50](50-item-companion-flags.md)
- pełny slot rebuild research → `30-slot-rebuild-research.md`

---

## 2. Status

| Warstwa | Implementation | Test coverage | In-game verification |
|---|---|---|---|
| L0 — UnlockedRegions | ✅ `core.SetUnlockedRegions` + `RebuildSlot` | ✅ 4 testy w `writer_regions_test.go` (PS4/PC, in-memory, after-add-item) | ✅ teleport adds region 6101000 |
| L1 — Detailed Bitmap | ✅ `revealBaseMap` + `revealDLCMap` Phase 1+2 | ✅ `TestMapFlagsRoundtrip` + `TestGetAllMapEntries` | ✅ tekstura mapy widoczna po revealAll |
| L2 — DLC Cover Layer | ✅ `revealDLCMap` Phase 3 (synthetic coords) | ⚠️ pokryte przez offset round-trip, brak dedicated test | ⚠️ czarne kafelki usunięte (DLC fix Apr 2026) |
| L3 — Fog of War | ✅ `RemoveFogOfWar` (fill 0xFF) | ⚠️ pokryte przez slot round-trip, brak dedicated test | ✅ szary overlay usunięty |
| Reset | ✅ `ResetMapExploration` (clear L1 + items, preserve L0 + system flags) | ⚠️ brak dedicated test | ⚠️ `needs verification` po pełnym reset+save+reload |

**Multi-phase atomic rollback**: ❌ brak — Phase 1 (flags) i Phase 2 (item add) w `revealBaseMap` / `revealDLCMap` nie są atomic. Jeśli `AddItemsToSlot` zwróci błąd po SetEventFlag, flagi pozostają w save'ie. UI ma snapshot-undo na poziomie save (`pushUndo` przed `RevealAllMap`).

**Ostatnia weryfikacja**: 2026-05-21 na `tmp/save/ER0000.sl2` (PC, 5 slotów) + `tmp/save/oisis_pl-org.txt` (PS4) — round-trip + flag toggle tests.

---

## 3. Source of truth w kodzie

| Komponent | Plik / linie | Notatka |
|---|---|---|
| `GetMapProgress` | `app_world.go:955` | Zwraca `[]db.MapEntry` dla UI (status `enabled` per flag) |
| `SetMapRegionFlags` | `app_world.go:982` | Set/clear pojedynczy visible flag + dodaje/usuwa map fragment item |
| `SetMapFlag` | `app_world.go:1017` | Set/clear raw flag (system, visible, unsafe) — bez item side effect |
| `RevealAllMap` | `app_world.go:1041` | Composite: `pushUndo` + `revealBaseMap` + `revealDLCMap` |
| `revealBaseMap` (internal) | `app_world.go:1067` | Phase 1 flags (MapSystem + MapVisible non-DLC) + Phase 2 items |
| `revealDLCMap` (internal) | `app_world.go:1094` | Phase 1 DLC flags (62002, 82002) + DLC visible flags + Phase 2 items + Phase 3 cover layer coords |
| `ResetMapExploration` | `app_world.go:1162` | Clear visible flags + remove fragment items + clear MapAcquired + clear MapUnsafe; preserve system flags |
| `RemoveFogOfWar` | `app_world.go:1201` | Fill bitfield `afterRegs+0x087E..afterRegs+0x10B0` z `0xFF` |
| Map data | `backend/db/data/maps.go` | `MapSystem` (4 wpisy), `MapVisible` (263 wpisy), `MapUnsafe` (8 wpisów), `MapFragmentItems` (24 wpisy), `MapAcquired` (24 wpisy — unused; gra clearuje) |
| DLC detection | `backend/db/data/maps.go::IsDLCMapFlag` | Range `62080–62084` OR `62800–62999` |
| Region writer | `backend/core/writer.go::SetUnlockedRegions` | Dedupe + sort + `RebuildSlot` + rollback |
| Slot rebuilder | `backend/core/slot_rebuild.go::RebuildSlot` | Pełny sekwencyjny serializator |
| DLC tile constants | `backend/core/offset_defs.go:309–322` | `DLCTileZeroStart/End`, `DLCTileRec1X/Y/Flag`, `DLCTileRec2X/Y/Z/W/Flag` |
| `afterRegs` helper | `app_world.go::resolveAfterRegs` | `storageEnd + DynStorageToGestures + 4 + regCount*4` |
| Tests | `tests/map_flags_test.go`, `backend/core/writer_regions_test.go` | Patrz §16 |
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
│  ... pozostałe sekcje ...                                        │
│  Inventory                                                       │
│    Map Fragment items 0x40002198..0x401EA61C    ← L1 (items)     │
└──────────────────────────────────────────────────────────────────┘
```

Cztery warstwy są **niezależne**:

- Ustawienie region ID (L0) **nie odkrywa** tekstury — patrz log weryfikacji §17 test 1.
- Ustawienie visible flag (L1) bez map fragment item odkrywa teksturę, ale UI nie pokazuje fragmentu — §17 test 7.
- Ustawienie DLC visible flag (L1) bez Phase 3 cover layer coords (L2) → tekstura widoczna, ale czarne kafelki nadal pokrywają obszar — §17 test 8.
- Fog of war (L3) jest czysto kosmetyczny — nie odblokowuje treści.

`RevealAllMap` w aplikacji obsługuje L0 (pośrednio przez teleport later) + L1 + L2 — **nie** dotyka L3 (FoW). UI używa `RemoveFogOfWar` jako oddzielnej akcji.

---

## 5. Map reveal layers — overview

| L | Warstwa | Storage | Endpoint | Master detail |
|---|---|---|---|---|
| 0 | Unlocked Regions | `slot.UnlockedRegions []u32` (variable-length) | `core.SetUnlockedRegions`, `App.SetRegionUnlocked`, `App.BulkSetUnlockedRegions` | [11 — Regions](11-regions.md) |
| 1 | Detailed Bitmap | Event flags 62xxx (visible) + 4 system flags + Map Fragment items w ekwipunku | `App.SetMapRegionFlags`, `App.SetMapFlag`, `App.RevealAllMap` (composite) | [15 — Event Flags](15-event-flags.md) (helper), [50 — Item Companion Flags](50-item-companion-flags.md) (fragment items) |
| 2 | DLC Cover Layer | 8 floatów + 2 flagi w BloodStain (afterRegs + `DLCTileRec*`) | `revealDLCMap` Phase 3 (internal) | [29 — DLC Black Tiles](29-dlc-black-tiles.md) |
| 3 | Fog of War | Bitfield `afterRegs+0x087E..+0x10B0` (2099 B) w luce BloodStain→MenuProfile | `App.RemoveFogOfWar` | (master tutaj — §9) |

---

## 6. L0 — Unlocked Regions

L0 kontroluje **szybką podróż między Sites of Grace w obrębie regionu** + matchmaking multiplayer. **Nie ma wpływu na widoczność tekstury mapy.**

### 6.1 Edit path

```go
err := core.SetUnlockedRegions(slot, []uint32{...})
```

- Dedupe + sort ascending → `RebuildSlot` (pełny rebuild 0x280000-B slotu) → recalculate dynamic offsets → SectionMap refresh → rollback on error.
- App-level methods: `SetRegionUnlocked(slotIdx, regionID, unlocked)`, `BulkSetUnlockedRegions(slotIdx, ids)`, `GetUnlockedRegions(slotIdx)`.

### 6.2 Liczba regionów

`backend/db/data/regions.go` zawiera 104 wpisy: 62 overworld base + 7 DLC overworld + 35 legacy dungeons (zakresy: 6100000–6899999 base, 6900000–6999999 DLC, 1000000–1999999 dungeons). Świeży save ma 6 region IDs; „Unlock All" ustawia ~211 ID base+DLC.

### 6.3 Region IDs vs widoczność mapy

Region ID **nie** odkrywają tekstury mapy ani nie usuwają fog of war. Patrz §17 test 1: dodanie samego region_id (bez flagi 62xxx) → tekstura mapy bez zmian.

### 6.4 Szczegóły layout

Pełna lista region IDs, binary layout, `RebuildSlot` pipeline → [11 — Regions](11-regions.md).

---

## 7. L1 — Detailed Bitmap (event flags + map fragments)

Główny mechanizm „odkrycia mapy". Implementacja w `revealBaseMap` + `revealDLCMap` (Phase 1+2).

### 7.1 Flagi systemowe (`MapSystem`)

```go
var MapSystem = map[uint32]MapRegionData{
    62000: {Name: "Allow Map Display", Area: "System"},
    62001: {Name: "Allow Underground Map Display", Area: "System"},
    82001: {Name: "Show Underground", Area: "System"},
    82002: {Name: "Show Shadow Realm Map", Area: "System"},
}
```

Bez tych flag UI mapy w grze może pozostawać puste — flaga widoczności pojedynczego regionu nie wystarczy, jeśli system display jest off.

⚠️ Notatka: `revealBaseMap` iteruje po **wszystkich** wpisach `MapSystem` (włącznie z `82002` Shadow Realm), nie filtrując. `revealDLCMap` dodatkowo set'uje **`62002`** (Allow Shadow Realm Map Display) inline — która **nie** jest w `MapSystem` mapie. Jeśli chcesz odkryć tylko base game bez DLC display, lepiej użyj `App.SetMapFlag` per-ID zamiast `RevealAllMap`. `needs verification` czy ten layout jest intencjonalny (kod traktuje 62002 jako DLC-only, ale 82002 jako system).

### 7.2 Flagi widoczności (`MapVisible`)

263 wpisy w `MapVisible` (219 non-DLC + 44 DLC). Każda flaga 62xxx odkrywa teksturę regionu w UI mapy. Zakresy:

| Zakres | Obszar |
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

Pełna lista: `MapVisible` w `maps.go`. DLC detection: `IsDLCMapFlag(id)` returns true dla `62080–62084` OR `62800–62999`.

### 7.3 Map Fragment items

24 wpisy w `MapFragmentItems` (19 base + 5 DLC). Każda overworld visible flag (`62010–62064`, `62080–62084`) ma sparowany item:

```go
var MapFragmentItems = map[uint32]uint32{
    62010: 0x40002198, // Limgrave, West
    ...
    62084: 0x401EA61C, // Abyss (DLC)
}
```

Range: base game `0x40002198..0x400021AA`, DLC `0x401EA618..0x401EA61C`.

**Dlaczego dodajemy item + flag**: gracz w normalnym przebiegu gry podnosi fragment z mapy, gra ustawia flagę + dodaje item. Edytor replikuje oba dla spójności (jeśli ustawisz tylko flagę, gracz widzi teksturę, ale w ekwipunku brakuje fragmentu — patrz §17 test 7).

### 7.4 Map Acquired (63xxx) — NIE używane

`MapAcquired` mapuje flagi 63xxx (offset = `visible+1000`). To są **przejściowe trigger flagi pickup-popup** — gra je podnosi by wyświetlić „Map Fragment acquired" toast, a następnie kasuje. **Edytor celowo ich nie ustawia.** `ResetMapExploration` clearuje je (defensive) ale nigdy ich nie set'uje.

### 7.5 MapUnsafe sub-region flags

8 wpisów w `MapUnsafe` (`62004–62009, 62053, 62065`) — flagi sub-regionów, które przy ręcznym ustawieniu mogą powodować czarne kafelki w UI (uszkodzenie Cover Layer). Wykluczone z `RevealAllMap`. Eksponowane w UI (jako known list) ale z ostrzeżeniem.

### 7.6 Algorytm `revealBaseMap` / `revealDLCMap`

```text
revealBaseMap(slot):
    flags = slot.Data[slot.EventFlagsOffset:]

    # Phase 1 — flags (slot.Data nie shift)
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

    # Phase 3 — cover layer coords (patrz §8 i 29)
```

⚠️ **Kolejność ma znaczenie**: `AddItemsToSlot` przesuwa bajty wewnątrz slotu, co unieważnia wcześniej obliczony slice `flags`. Phase 1 musi się zakończyć **przed** Phase 2.

### 7.7 Cross-reference do event flags

Event flag bit/byte indexing, helper API (`db.SetEventFlag`/`GetEventFlag`), 3-tier resolver, BST → [15 — Event Flags](15-event-flags.md).

---

## 8. L2 — DLC Cover Layer

Tekstura mapy DLC pozostaje pokryta „czarnymi kafelkami" dopóki gracz nie odkryje obszaru in-game. Same flagi 62080–62084 nie wystarczą — gra wymaga **syntetycznych współrzędnych** w sekcji BloodStain wskazujących obszar DLC jako odkryty.

### 8.1 Write path (Phase 3 w `revealDLCMap`)

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

### 8.2 Stałe offsetowe

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

Format f32: little-endian (`binary.LittleEndian.PutUint32(math.Float32bits(v))`).

### 8.3 Synthetic coords — values

Wartości `9648, 9124` i `3037, 1869, 7880, 7803` są syntetyczne (nie z gry) — oznaczają „obszar DLC jest odkryty na poziomie central + anchor". `needs verification` czy te wartości nadal działają dla najnowszego patcha gry (ostatnia weryfikacja: Apr 2026 DLC fix).

### 8.4 Szczegóły layout + research log

Pełna analiza Cover Layer, dziennik investigacji, alternatywy testowane → [29 — DLC Black Tiles](29-dlc-black-tiles.md).

---

## 9. L3 — Fog of War (`RemoveFogOfWar`)

Gęsta maska bitowa pomiędzy BloodStain a MenuProfile reprezentująca stan eksploracji per-kafelek. Edytor eksponuje dedykowaną akcję, która **wypełnia cały zakres wartością `0xFF`** — czyli „wszystko odkryte". Selektywne per-tile clearing **nie jest** zaimplementowane (wymaga reverse-engineeringu mapowania bit-do-kafelka — patrz §17 L4).

### 9.1 Lokalizacja

```text
afterRegs        = storageEnd + DynStorageToGestures + 4 + regCount*4
bitfield_start   = afterRegs + 0x087E
bitfield_end     = afterRegs + 0x10B0  (inclusive last safe byte)
usable_range     = 2099 bytes (0x087E .. 0x10B0)
section_size     = 0x103C bytes total (BloodStain → MenuProfile)
```

⚠️ **Krytyczne**: zapis za `+0x10B0` nachodzi na `MenuProfile` i **powoduje crash gry**. Prefiks `+0x0000..+0x087D` zawiera ustrukturyzowane dane horse + bloodstain — nie dotykaj z tej warstwy.

### 9.2 Format bitfield

Płaska maska bitowa, LSB-first w każdym bajcie. `1` = kafelek odkryty, `0` = ukryty. Mapowanie bit-do-kafelka jest **nieznane** i nie da się wywnioskować z region ID. Jeden teleport in-game przerzuca ~356 bitów w ciągłym oknie 157 bajtów (zob. §17 test 6).

### 9.3 Algorytm

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

In-place fill, bez przesuwania bajtów, bez przeliczania offsetów. Idempotent — wielokrotny call zostawia ten sam stan.

### 9.4 Dlaczego oddzielone od `RevealAllMap`

- Detailed Bitmap (L1) daje graczowi **użyteczny sygnał** — tekstury, ikony dungeonów, posiadanie fragmentów. To jest co większość użytkowników rozumie przez „pokaż mi mapę".
- Fog of War (L3) jest **czysto kosmetyczny** — usuwa szary overlay „tu jeszcze nie byłeś". Nie odblokowuje nowej zawartości.
- Włączenie L3 do `RevealAllMap` wymusiłoby utratę naturalnego sygnału eksploracji. Trzymanie L3 za własną akcją zachowuje wybór użytkownika.

Frontend (`WorldTab.tsx:219`):

```ts
const handleRevealAllMap = async () => {
    await RevealAllMap(charIdx);
    await RemoveFogOfWar(charIdx);  // user-facing: jedno kliknięcie = L1+L2+L3
    ...
};
```

UI łączy obie akcje za przyciskiem „Reveal All" z `RiskActionButton riskKey="map_reveal_full"` (Tier 1 ban-risk confirmation). Patrz [32](32-ban-risk-system.md).

### 9.5 Selektywne FoW removal

❌ Nie zaimplementowane. Wymagałoby mapowania bit-do-kafelka (FoW bitfield → kafel + obszar + region). Patrz §17 L4 — nieznane mapping.

---

## 10. Event flag relation

Warstwy L1 i L2 używają event flag helper API. Konwencja byte/bit indexing, BST resolver, snapshot/rollback semantyka → [15 — Event Flags](15-event-flags.md).

Krytyczne różnice polityki SET/CLEAR (z [15 §11](15-event-flags.md#11-feature-specific-policy-differences)):

| Flag set | Polityka | Master |
|---|---|---|
| MapSystem (4 flagi) | Symmetric SET/CLEAR; `ResetMapExploration` **nie** czyści (preserve) | (tu) |
| MapVisible (62xxx) | Symmetric SET/CLEAR; `ResetMapExploration` clearuje wszystkie | (tu) |
| MapUnsafe (sub-region) | Symmetric SET/CLEAR; wykluczone z RevealAll, ale clearowane przez ResetMap | (tu) |
| MapAcquired (63xxx) | SET przez grę tylko jako trigger; edytor **nigdy** nie set'uje (clearuje przy reset jako defensive) | (tu) |
| Map Fragment items (`MapFragmentItems`) | SET-only w aktualnym kodzie (brak CLEAR-on-remove) → wynika z tego, że `ResetMapExploration` używa `RemoveItemByBaseID` zamiast event flag clearing; companion flag policy → patrz [50](50-item-companion-flags.md) |
| Container pickup flags (66xxx) | SET-only w aktualnym kodzie — patrz [15 §11](15-event-flags.md#11-feature-specific-policy-differences) i [50](50-item-companion-flags.md) |

---

## 11. Map region data

Wszystkie struktury danych w `backend/db/data/maps.go`:

| Struktura | Liczba wpisów | Cel |
|---|---|---|
| `MapSystem` | 4 | Globalne flagi „enable map display" (62000, 62001, 82001, 82002) |
| `MapVisible` | 263 (219 non-DLC + 44 DLC) | Region/dungeon visibility flagi (62xxx) — overworld + dungeons + DLC |
| `MapUnsafe` | 8 | Sub-region flagi powodujące black tiles (62004–62009, 62053, 62065) |
| `MapFragmentItems` | 24 | Mapowanie visible flag → map fragment item ID (19 base + 5 DLC) |
| `MapAcquired` | 24 (63xxx trans.) | Trigger pickup flagi — **edytor nie ustawia**, `ResetMapExploration` clearuje (defensive) |

DLC detection: `IsDLCMapFlag(id)` — `62080–62084` (overworld) lub `62800–62999` (dungeons).

⚠️ **`MapVisible` 263 wpisy to snapshot 2026-05-21** — generowane offline z regulation. Po update'cie patcha gry wymagana re-extracja. `needs verification` po nowym DLC.

---

## 12. Map fragment companion behavior

Dodawanie map fragment items w Phase 2 `revealBaseMap`/`revealDLCMap` używa `core.AddItemsToSlot` — nie `App.AddItem` ani `App.AddItemsToCharacter`. Konsekwencje:

- **Item-level companion flags** (np. dla Spectral Steed Whistle) **nie są** automatycznie ustawiane — `AddItemsToSlot` to low-level path.
- `MapFragmentItems` (24 fragmenty) **nie ma** companion flag setu w `itemCompanionEventFlags` — fragment się normalnie loaduje przez visible flag (L1.1), bez side effects.
- `ResetMapExploration` używa `RemoveItemByBaseID` do usunięcia fragmentów — bez CLEAR-on-remove dla event flagów MapAcquired (63xxx). To zachowanie wynika z tego, że flagi MapAcquired są transient w grze.

Companion flag semantyka (item-level vs grace-level) → [50 — Item Companion Flags](50-item-companion-flags.md) (master).

---

## 13. Current implemented endpoints

| Endpoint | Sygnatura | Cel |
|---|---|---|
| `GetMapProgress(slotIdx)` | `([]db.MapEntry, error)` | Read all flags status dla UI (MapEntry z `enabled bool`) |
| `SetMapRegionFlags(slotIdx, visibleFlagID, enabled)` | `error` | Toggle pojedynczy visible flag + add/remove map fragment item |
| `SetMapFlag(slotIdx, flagID, enabled)` | `error` | Toggle raw flag (system/visible/unsafe) — bez item side effect |
| `RevealAllMap(slotIdx)` | `error` | Composite: `pushUndo` + `revealBaseMap` + `revealDLCMap` (L1 + L2) |
| `ResetMapExploration(slotIdx)` | `error` | Clear visible + remove fragments + clear MapAcquired + clear MapUnsafe; preserve MapSystem |
| `RemoveFogOfWar(slotIdx)` | `error` | Fill bitfield 0xFF (L3) |

Pomocnicze (internal, nie eksponowane przez Wails):

- `revealBaseMap(slot)` — `app_world.go:1067`
- `revealDLCMap(slot)` — `app_world.go:1094`
- `resolveAfterRegs(slot)` — pomocnicza dla L2 + L3 offset calc
- `putF32(d, off, v)` — pomocnicza dla LE f32 write

Frontend: `WorldTab.tsx` (lista zakładek). Brak dedicated `MapTab` — map exploration jest accordion w `WorldTab`.

---

## 14. Write path and rollback caveats

### 14.1 `RevealAllMap` — composite, multi-phase

```text
pushUndo(slotIdx)
  → revealBaseMap(slot)
       Phase 1: SetEventFlag × ~223 (4 MapSystem + 219 MapVisible non-DLC)
       Phase 2: AddItemsToSlot × 19 (base game fragments z MapFragmentItems)
  → revealDLCMap(slot)
       Phase 1: SetEventFlag × ~46 (62002 + 82002 inline + 44 MapVisible DLC)
       Phase 2: AddItemsToSlot × 5 (DLC fragments z MapFragmentItems)
       Phase 3: write 8 floats + 2 flags (cover layer)
```

⚠️ **Brak atomic rollback wewnątrz Phase 1/2/3**: jeśli Phase 2 `AddItemsToSlot` zwróci błąd po Phase 1 SetEventFlag, flagi pozostają w save'ie. Rollback dostępny tylko na poziomie save'a (`pushUndo` snapshot przed `RevealAllMap`).

### 14.2 `SetUnlockedRegions` — atomic via `RebuildSlot`

`core.SetUnlockedRegions` zwraca error jeśli rebuild fail → caller (`SetRegionUnlocked` / `BulkSetUnlockedRegions`) rollback'uje przez restore slot.Data. Atomic w obrębie pojedynczej zmiany regionów.

### 14.3 `RemoveFogOfWar` — in-place

Bez przesuwania bajtów, bez rebuild. Single pass byte fill — albo cały zakres ustawiony na 0xFF (success), albo bounds check fail (error pre-write).

### 14.4 `ResetMapExploration` — best-effort

Iteruje po MapVisible × `SetEventFlag(false)` + `RemoveItemByBaseID(itemID)`. Brak atomic — pojedyncze flag set może zwrócić error (`SetEventFlag` ma bounds check). Kod używa `_ = db.SetEventFlag(...)` (discard error) — best effort, no rollback.

---

## 15. Validation and safety notes

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| V1 | `RevealAllMap` ustawia 82002 (Shadow Realm) nawet dla użytkownika bez DLC | ⚠️ medium | Flaga ustawia się bez efektu jeśli DLC nie posiadane; brak crash, ale UI mismatch. `needs verification` czy gra fail-closes UI przy braku DLC. |
| V2 | `RevealAllMap` `RiskActionButton` Tier 1 — user musi potwierdzić | ⚠️ medium | `WorldTab.tsx:321` z `riskKey="map_reveal_full"`. Patrz [32](32-ban-risk-system.md). |
| V3 | Ręczne ustawienie MapUnsafe (`62004–62009`, `62053`, `62065`) → czarne kafelki | 🔴 high | `MapUnsafe` exposed w UI ale wykluczone z `RevealAllMap`. UI musi sam przefiltrować lub ostrzec. |
| V4 | DLC tile coords (9648/9124, 3037/1869/7880/7803) hardcoded | ⚠️ medium | `needs verification` po każdym patchu gry. Apr 2026 DLC fix potwierdził values, ale brak automatycznego diff vs regulation. |
| V5 | FoW write za `+0x10B0` → crash gry (overlap MenuProfile) | 🔴 high | Hardcoded bound w `RemoveFogOfWar`; brak runtime check że `afterRegs+0x10B0 < menuProfileOff`. `needs verification` po slot layout changes. |
| V6 | Multi-phase rollback w `revealBaseMap`/`revealDLCMap` brakuje | ⚠️ medium | Snapshot na poziomie save (`pushUndo`) jest workaround; nie ma per-flag rollback. |
| V7 | Region ID dla DLC region na non-DLC save | ⚠️ low | Gra toleruje (safe), ale logicznie niepoprawne. UI mogłoby warn — `needs verification` czy ostrzega. |
| V8 | `ResetMapExploration` discard errors (`_ = db.SetEventFlag(...)`) | ⚠️ medium | Best-effort; pojedyncze bounds error nie blokuje reset. `needs verification` że w praktyce reset jest pełen. |
| V9 | `SetMapRegionFlags` + duplikat item add (idempotency) | ⚠️ low | Map fragment item w inventory: drugi add może skutkować duplikatem (jeśli `AddItemsToSlot` nie dedupe'uje). `needs verification` przy bulk toggle. |

---

## 16. Test coverage

| Test | Cel | Plik |
|---|---|---|
| `TestSetUnlockedRegionsInMemory` | L0 in-memory mutation | `backend/core/writer_regions_test.go:11` |
| `TestSetUnlockedRegionsRoundTripPS4` | L0 PS4 round-trip | `backend/core/writer_regions_test.go:48` |
| `TestSetUnlockedRegionsAfterAddItem` | L0 po `AddItemsToSlot` (slot shift) | `backend/core/writer_regions_test.go:106` |
| `TestSetUnlockedRegionsRoundTripPC` | L0 PC round-trip | `backend/core/writer_regions_test.go:158` |
| `TestBSTLookupMatchesEventFlags` | L1 helper API — BST matches precomputed | `tests/map_flags_test.go:12` |
| `TestGetAllMapEntries` | L1 — `GetMapProgress` returns DB entries | `tests/map_flags_test.go:53` |
| `TestMapFlagsRoundtrip` | L1 — flag toggle → save → reload → state preserved | `tests/map_flags_test.go:96` |
| `TestEventFlagsOffsetCorrectness` | L1 — `slot.EventFlagsOffset` points at bitfield | `tests/map_flags_test.go:145` |

Brak dedicated test:

- `RevealAllMap` end-to-end (composite call → save → reload → all DLC + base visible + fragments in inventory).
- `revealDLCMap` Phase 3 coverage layer values (offsets pokryte przez round-trip, ale brak assert na wartościach 9648/9124 etc.).
- `RemoveFogOfWar` post-write byte verification (pokryte przez slot round-trip, brak dedicated).
- `ResetMapExploration` round-trip (set → save → reload → reset → save → reload → original state).
- PS4↔PC cross-platform bit-equal dla L1 (round-trip per-platforma, brak `TestConvert*`).

---

## 17. Known limits / needs verification

| # | Limit / gap | Status | Notatka |
|---|---|---|---|
| L1 | Multi-phase atomic rollback | ❌ brak | Snapshot na save-level jest workaround; per-flag/per-item rollback nie istnieje. |
| L2 | DLC tile coords stale po patchu gry | ⚠️ snapshot Apr 2026 | `needs verification` po każdym DLC update'cie. |
| L3 | `MapVisible` 263 wpisy to snapshot 2026-05-21 | ⚠️ generated offline | `needs verification` po update'cie regulation. |
| L4 | Mapowanie bit-do-kafelka w FoW bitfield | ❌ nieznane | Wymagałoby systematycznych diffów per-tile in-game; nie w roadmapie. |
| L5 | Region IDs `1001000–1001002` w fresh save | ⚠️ wewnętrzne markery | Występują w każdym fresh save, ale nie ma ich w `regions.go`. Prawdopodobnie internal startup. |
| L6 | Ustrukturyzowany prefiks `+0x0800..+0x087D` w BloodStain | ⚠️ nie nadpisywać | Powtarzający się wzorzec `00 00 01 80 BF FF FF FF FF 00...`. Prawdopodobnie coord anchors. |
| L7 | `RevealAllMap` ustawia 82002 (DLC) niezależnie od DLC ownership | ⚠️ flag set, brak side effect | `needs verification` czy gra fail-closes UI gdy DLC nie posiadane. |
| L8 | DLC visible flagi `62800–62999` (dungeons) | ⚠️ pokryte przez `MapVisible` ale brak weryfikacji per-flag | `needs verification` że wszystkie DLC dungeon flagi istnieją w aktualnym patchu. |
| L9 | PS4↔PC cross-platform conversion test dla map state | ❌ brak | Round-trip per-platforma jest pokryty (L0 testy), brak `TestConvert*` dla L1+L2+L3 cross-platform diff. |
| L10 | Selektywne FoW removal | ❌ nie zaimplementowane | Wymaga L4 (bit-to-tile mapping). |
| L11 | `SetMapRegionFlags` duplikat item-add | ⚠️ `needs verification` | Wielokrotny toggle: czy `AddItemsToSlot` dedupe'uje, czy duplikuje? |
| L12 | `revealBaseMap` Phase 1 iteruje ALL `MapSystem` (włącznie z 82002) | ⚠️ design choice vs bug | Czy ustawienie 82002 (Shadow Realm) bez `62002` (DLC display allow) jest spójne? `needs verification`. |

---

## 18. Cross-references

| Topic | Master rozdział |
|---|---|
| Event flag fundament (byte/bit, helper API, BST, bounds check) | [15 — Event Flags](15-event-flags.md) |
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

Empiryczne testy z badania FoW, które uzasadniły podział na niezależne warstwy. Zachowane jako reference; szczegóły metodologii → [99 — Verification Methodology](99-verification-methodology.md).

| # | Test | Wynik |
|---|---|---|
| 1 | Dodanie region_id (bez flagi 62xxx, bez fragmentu) | Tekstura mapy bez zmian (regions ≠ visibility) |
| 2 | 0xFF w FoW bitfield (mały zakres) + 1 region | Mgła usunięta lokalnie |
| 3 | 0xFF zapisane za `+0x10B0` (nakłada się na MenuProfile) | **Crash gry** |
| 4 | 0xFF w pełnym zakresie FoW bitfield, brak zmiany regionów | Cała mgła usunięta (tylko kosmetycznie) |
| 5 | Wstawienie 205 regionów przez byte-shift (legacy path) | **Crash gry** (slot obcięty) — naprawione przejściem na `RebuildSlot` |
| 6 | Teleport in-game (Warmaster's Shack) | Dodaje region `6101000` + ustawia 356 bitów FoW w oknie 157 bajtów |
| 7 | Ustawienie flagi 62xxx visible bez fragmentu | Tekstura mapy odkryta, ale w UI brak fragmentu (ekwipunek) |
| 8 | Tylko flagi DLC visible, bez Phase 3 Cover Layer | Tekstura pojawia się, ale czarne kafelki nadal pokrywają obszar DLC |

### Pliki testowe (`tmp/save/`)

| Plik | Opis |
|---|---|
| `ER0000.sl2` | Oryginalny save, pełna mgła wojny, 6 regionów |
| `ER0000-fow-before.sl2` | Po edytorze (mapy + grace dodane), FoW bez zmian |
| `ER0000-from-deck.sl2` | Po graniu w grze (1 teleport), lokalna mgła usunięta |
| `ER0000-no-fow-test.sl2` | Region + częściowe pole bitowe, mgła usunięta lokalnie |
| `ER0000-no-fow.sl2` | Pełne wypełnienie pola bitowego, cała mgła usunięta |

---

## 20. Sources

### Code

- `app_world.go::GetMapProgress`/`SetMapRegionFlags`/`SetMapFlag`/`RevealAllMap`/`revealBaseMap`/`revealDLCMap`/`ResetMapExploration`/`RemoveFogOfWar` — endpointy + internal helpers
- `app_world.go::resolveAfterRegs`/`putF32` — coord pomocnicze
- `backend/db/data/maps.go::MapSystem`/`MapVisible`/`MapUnsafe`/`MapFragmentItems`/`MapAcquired`/`IsDLCMapFlag`
- `backend/db/data/regions.go::GetAllRegions` — 104 regions
- `backend/core/writer.go::SetUnlockedRegions`
- `backend/core/slot_rebuild.go::RebuildSlot`
- `backend/core/offset_defs.go:309–322` — DLC tile constants
- `backend/core/structures.go::DynStorageBox`/`DynStorageToGestures` — slot offset math
- `frontend/src/components/WorldTab.tsx` — UI integration

### Tests

- `backend/core/writer_regions_test.go` — 4 testy L0 (PS4/PC, in-memory, after-add-item)
- `tests/map_flags_test.go` — 4 testy L1 (BST, GetAll, Roundtrip, OffsetCorrectness)

### Reference parsers

- er-save-manager (Python) — `parser/event_flags.py`, region layout reference
- ER-Save-Editor (Rust) — region container layout reference

### Hex-verified saves

- `tmp/save/ER0000.sl2` (PC, 5 slotów) — round-trip 2026-05-21
- `tmp/save/oisis_pl-org.txt` (PS4) — round-trip 2026-05-21
- `tmp/save/ER0000-no-fow*.sl2` — FoW investigation snapshots
