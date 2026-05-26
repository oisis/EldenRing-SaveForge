# 48 — PvP‑ready Modular Presets

> **Type**: Binary format spec + design doc (canonical chapter)
> **Scope**: Aktualny stan `ApplyPvPPreparation` w SaveForge — 5 modułów (4 aktywne + 1 placeholder), single snapshot undo, write path opcjonalnie per module, status UI w `PvPPreparationTab.tsx`.

Cross‑refs: [11-regions.md](11-regions.md), [14-game-state.md](14-game-state.md), [15-event-flags.md](15-event-flags.md), [16-world-state.md](16-world-state.md), [27-map-reveal.md](27-map-reveal.md), [29-dlc-black-tiles.md](29-dlc-black-tiles.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md), [50-item-companion-flags.md](50-item-companion-flags.md).

---

## 1. Cel rozdziału

Zdefiniować jednoznacznie:

- co realnie robi `ApplyPvPPreparation` (`app_pvp.go`),
- które moduły są **aktywne** (4): Matchmaking Regions, Colosseums, Reveal Map, Summoning Pools,
- który moduł jest **placeholder** (1): Sites of Grace — zwraca warning bez mutacji,
- jak działa single‑snapshot undo i propagacja błędów (fail‑fast po pierwszym module),
- gdzie kończy się implementacja a zaczyna `needs verification`.

Nie powiela helper API event flag (patrz [15-event-flags.md](15-event-flags.md)), modelu Map Reveal (patrz [27-map-reveal.md](27-map-reveal.md)), regionów (patrz [11-regions.md](11-regions.md)), gracji (patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md)), item companion flags (patrz [50-item-companion-flags.md](50-item-companion-flags.md)).

## 2. Status

| Aspekt | Status |
|---|---|
| Backend endpoint `ApplyPvPPreparation(slotIndex, opts)` | ✅ `app_pvp.go:25` |
| Struktura `PvPPreparationOptions` (5 bool fields) | ✅ `app_pvp.go:13‑19` |
| Module: Matchmaking Regions | ✅ active — `core.SetUnlockedRegions` |
| Module: Colosseums | ✅ active — `ColosseumFlagSets` + `ColosseumGlobalFlags` |
| Module: Reveal Map | ✅ active — `revealBaseMap` + `revealDLCMap` |
| Module: Summoning Pools | ✅ active — bulk `SetEventFlag` |
| Module: Sites of Grace | ❌ **placeholder** — warning only, no mutation |
| Single snapshot undo (`pushUndo`) | ✅ raz na początku |
| Frontend `PvPPreparationTab.tsx` | ✅ 5 modułów + 3 profile (minimal/full/coop) + custom |
| Sites of Grace w UI | 🔒 `disabled: true` z notą „Coming soon — broad QoL module, needs UX confirmation” |
| Pokrycie testem | ✅ 10 testów w `pvp_test.go` (4 validation + 3 warnings + 3 mutation) |

## 3. Source of truth w kodzie

| Plik / symbol | Co zawiera |
|---|---|
| `app_pvp.go::PvPPreparationOptions` (linie 13‑19) | Struct z 5 polami `bool` |
| `app_pvp.go::ApplyPvPPreparation` (linie 25‑113) | Single endpoint, single `pushUndo`, sekwencyjne `if opts.X { ... }` per module |
| `app_pvp.go::revealBaseMap` / `revealDLCMap` (wołane z `RevealMap` module) | Patrz [27-map-reveal.md](27-map-reveal.md) — same funkcje są używane przez `RevealAllMap` w `app_world.go` |
| `backend/db/data/summoning_pools.go::ColosseumFlagSet` (linie 335‑367) | Struktura `{Activate, MapPOI, NPC, Gate}` + `AllFlags()` zwraca w stabilnej kolejności |
| `backend/db/data/summoning_pools.go::ColosseumFlagSets` (linia 349) | 3 wpisy: Caelid `60350`, Limgrave `60360`, Royal `60370` |
| `backend/db/data/summoning_pools.go::ColosseumGlobalFlags` (linia 357) | `[6080, 60100, 69480]` — globalne flagi „dowolny colosseum unlocked” |
| `backend/db/db.go::GetAllRegions` / `GetAllColosseums` / `GetAllSummoningPools` | Static DB lookups (sync.OnceValue) |
| `frontend/src/components/PvPPreparationTab.tsx` | UI: 5 modułów `MODULES`, 3 profile `PROFILE_OPTS` (minimal/full/coop), custom resolver |
| `pvp_test.go` | 10 testów: 4 validation, 3 warning, 3 mutation + roundtrip |

## 4. Mental model

```
ApplyPvPPreparation(slotIndex, opts)
  ├─ validation: save loaded, slot index, slot non-empty, EventFlagsOffset valid
  │            → wszystkie błędy: return nil, error (przed pushUndo)
  ├─ a.pushUndo(slotIndex)                              ← single snapshot
  ├─ flags := slot.Data[slot.EventFlagsOffset:]
  │
  ├─ if opts.MatchmakingRegions:
  │     core.SetUnlockedRegions(slot, allRegionIDs)     ← realloc slot.Data + RebuildSlot
  │     flags = slot.Data[slot.EventFlagsOffset:]       ← REFRESH
  │     warnings += "Applied %d matchmaking regions."
  │
  ├─ if opts.Colosseums:
  │     for c in GetAllColosseums():
  │       for f in ColosseumFlagSets[c.ID].AllFlags():
  │         SetEventFlag(flags, f, true)                ← error → return err
  │     for f in ColosseumGlobalFlags:
  │       SetEventFlag(flags, f, true)                  ← error → return err
  │     warnings += "Colosseum matchmaking flags set. Physical gates may still need..."
  │
  ├─ if opts.RevealMap:
  │     revealBaseMap(slot)                              ← Phase 1 flags + Phase 2 items (patrz 27)
  │     revealDLCMap(slot)                               ← Phase 1+2+3 (patrz 27/29)
  │     flags = slot.Data[slot.EventFlagsOffset:]       ← REFRESH
  │     warnings += "Map revealed (base game + DLC)."
  │
  ├─ if opts.SummoningPools:
  │     for p in GetAllSummoningPools():
  │       SetEventFlag(flags, p.ID, true)               ← error → return err
  │     warnings += "Activated %d summoning pools. Bloody Finger invasion impact is unconfirmed."
  │
  └─ if opts.SitesOfGrace:
        warnings += "Sites of Grace module is planned but not enabled in this version."
        ← NO MUTATION
```

**Fail‑fast**: pierwszy error w module 1–4 powoduje `return nil, err` — kolejne moduły **nie wykonują się**, ale snapshot z `pushUndo` *nie* jest automatycznie odzyskany. Użytkownik musi ręcznie nacisnąć Undo.

## 5. Module status table

| # | Field w `opts` | Tier (UI) | Status backendu | Co robi | Główne flagi/sekcje |
|---|---|---|---|---|---|
| 1 | `MatchmakingRegions` | Recommended · Tier 1 | ✅ active | odblokowuje 274-regionową skuratowaną allowlistę invasion/blue przez `core.SetUnlockedRegions`, zachowując nieskuratowane surowe IDs | `slot.UnlockedRegions` (patrz [11](11-regions.md)) |
| 2 | `Colosseums` | Optional · Tier 1 | ✅ active | SET 12 flag (3 × 4 per‑colosseum) + 3 globalne | event flags w pasmach `60xxx`, `62xxx`, `69xxx`, `710xxx` |
| 3 | `RevealMap` | QoL · Tier 0 | ✅ active | `revealBaseMap` + `revealDLCMap` | event flags `62xxx`/`63xxx`/`82xxx` + map fragment items + DLC BloodStain (patrz [27](27-map-reveal.md)/[29](29-dlc-black-tiles.md)) |
| 4 | `SummoningPools` | Co‑op/Summon · Tier 1 | ✅ active | SET wszystkich pool IDs (`670xxx`) | event flags `670xxx` |
| 5 | `SitesOfGrace` | QoL · Tier 0 · planned | ❌ **placeholder** | NO‑OP — appendWarning only | brak (patrz §7) |

`needs verification`: dokładna liczba aktywowanych pooli i regionów w runtime zależy od `GetAllSummoningPools`/`GetAllRegions` snapshotu. Patrz [11-regions.md](11-regions.md) dla 274-regionowej skuratowanej allowlisty invasion/blue (podzbiór `PlayRegionParam`, **nie** "PvP wszędzie"); pooli ~213 (patrz CHANGELOG, brak izolowanego testu liczby).

## 6. Current implemented behavior

### 6.1 Pre‑mutation validation

Przed `pushUndo`:

- `a.save == nil` → `"no save loaded"`.
- `slotIndex < 0 || slotIndex >= 10` → `"invalid slot index"`.
- `slot.Version == 0` → `"slot %d is empty"`.
- `slot.EventFlagsOffset <= 0 || >= len(slot.Data)` → `"event flags offset not computed for slot %d"`.

Wszystkie 4 błędy zwracane **przed** `pushUndo` — czyli nie tworzą snapshotu.

### 6.2 Single snapshot undo

`a.pushUndo(slotIndex)` (linia 41) tworzy **jeden** snapshot dla całej operacji `ApplyPvPPreparation`. Wszystkie moduły mutują pod tym jednym snapshocie. Brak per‑module snapshotów — Undo cofa **całość**, nie konkretny moduł.

### 6.3 Module 1 — Matchmaking Regions

Moduł odblokowuje **skuratowaną allowlistę invasion/blue** (274 IDs z `db.GetAllRegions()` — podzbiór `PlayRegionParam`, nie cały świat; patrz [11](11-regions.md)). Jest niedestrukcyjny: IDs są scalane przez `mergeUnlockedRegions(ids, slot.UnlockedRegions)`, więc wszelkie nieskuratowane surowe region IDs już obecne w save są zachowane (`existingRaw ∪ curatedAllowlist`). Odblokowuje każdy **zweryfikowany legalny** region PvP — **nie** włącza "PvP wszędzie".

`core.SetUnlockedRegions(slot, ids)` wywołuje wewnętrznie `RebuildSlot`, które:

- realokuje `slot.Data` (zmienia rozmiar `UnlockedRegions` array),
- rekalkuluje `EventFlagsOffset`,
- może zmienić `StorageBoxOffset` itp.

Po tym `flags` slice jest stale — kod **explicit refreshuje** `flags = slot.Data[slot.EventFlagsOffset:]` w linii 57.

`needs verification`: czy `core.SetUnlockedRegions` zwraca error w jakichkolwiek realistycznych scenariuszach (np. duże listy regionów). Test `TestApplyPvPPreparation_BadFlagsOffset` (`pvp_test.go:54`) pokrywa scenariusz złego offsetu, ale nie failure path z `SetUnlockedRegions`.

### 6.4 Module 2 — Colosseums

Per‑colosseum:

```go
flagSet, ok := data.ColosseumFlagSets[c.ID]
if !ok {
    flagSet = data.ColosseumFlagSet{Activate: c.ID}
}
for _, id := range flagSet.AllFlags() {
    if id == 0 { continue }
    SetEventFlag(flags, id, true)
}
```

Czyli:

- Jeśli `c.ID` (z `GetAllColosseums`) jest w `ColosseumFlagSets` → SET 4 flag (`Activate`, `MapPOI`, `NPC`, `Gate`).
- Jeśli nie ma — fallback: SET tylko `Activate` (czyli `c.ID`).
- Po pętli per‑colosseum: SET 3 globalnych flag z `ColosseumGlobalFlags` (`6080`, `60100`, `69480`).

Aktualnie `ColosseumFlagSets` ma 3 wpisy (Caelid/Limgrave/Royal). `GetAllColosseums` zwraca dokładnie te 3 — fallback nie jest realnie używany. `needs verification` w przypadku przyszłego DLC z nowym colosseum: `GetAllColosseums` musi być zsynchronizowane z `ColosseumFlagSets`, inaczej będzie SET tylko `Activate`.

**Note o `60100` — Spectral Steed Whistle flag** — patrz [50-item-companion-flags.md](50-item-companion-flags.md): flaga `60100` jest **współdzielona** między `ColosseumGlobalFlags` (PvP prep) a item companion set dla Spectral Steed Whistle. Włączenie modułu Colosseums **ustawia Torrent‑unlock flag** jako efekt uboczny.

### 6.5 Module 3 — Reveal Map

Wywołuje **te same** funkcje co `RevealAllMap` z `app_world.go`:

```go
revealBaseMap(slot)   // Phase 1: 4 system flags + 219 non-DLC visible + 19 base fragment items
revealDLCMap(slot)    // Phase 1: 62002+82002 + 44 DLC visible. Phase 2: 5 DLC fragment items. Phase 3: BloodStain coords (L2).
```

Po obu: `flags = slot.Data[slot.EventFlagsOffset:]` (linia 94) bo `AddItemsToSlot` (wewnątrz Phase 2) realokuje `slot.Data`.

Patrz [27-map-reveal.md](27-map-reveal.md) §11 + [29-dlc-black-tiles.md](29-dlc-black-tiles.md) §11 dla szczegółów. Ten rozdział **nie powiela** modelu 4 warstw.

`needs verification`: czy `revealBaseMap` / `revealDLCMap` w kontekście PvP prep mają inny side‑effect surface niż wywoływane przez `RevealAllMap` — z analizy kodu wynika, że nie (te same funkcje, te same fazy), ale brak izolowanego testu kontekstu PvP.

### 6.6 Module 4 — Summoning Pools

```go
pools := db.GetAllSummoningPools()
for _, p := range pools {
    SetEventFlag(flags, p.ID, true)  // error → return err
}
```

Każdy pool ma 1 flagę (jego ID). Brak fan‑outu na inne flagi. `needs verification`: czy aktywacja pool flag wystarcza, żeby Martyr Effigy w grze faktycznie pojawiał się jako co‑op summon — runtime ad‑hoc test (CHANGELOG), brak CI.

**Warning literal**: „Activated %d summoning pools. **Bloody Finger invasion impact is unconfirmed.**” — explicit nie obiecujemy że pool activation pomaga PvP/Bloody Finger.

### 6.7 Module 5 — Sites of Grace (PLACEHOLDER)

```go
if opts.SitesOfGrace {
    warnings = append(warnings, "Sites of Grace module is planned but not enabled in this version.")
}
```

To **jedyna** linia. Brak:

- czytania `data.Graces`,
- wywołania `SetGraceVisited`,
- ustawiania jakiejkolwiek flagi,
- mutacji `slot.Data`.

Patrz §7.

## 7. Sites of Grace module E status

| Aspekt | Status |
|---|---|
| Backend `app_pvp.go::ApplyPvPPreparation` | 🔒 **placeholder** — `warning` literal, brak mutacji |
| Frontend `PvPPreparationTab.tsx` `MODULES[4]` | 🔒 `disabled: true`, `disabledNote: "Coming soon — broad QoL module, needs UX confirmation"` |
| Standalone grace endpoints (`GetGraces`/`SetGraceVisited`) | ✅ dostępne **niezależnie** w `WorldTab.tsx` — patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §10.2 |
| Bulk unlock w `WorldTab` | ✅ Tier 1 `RiskActionButton riskKey="bulk_grace_unlock"` |
| Test `TestApplyPvPPreparation_SitesOfGraceWarning` | ✅ `pvp_test.go:66` — assert warning string |

**Konsekwencja**: użytkownik, który chce odblokować wszystkie gracje w ramach PvP prep, musi **osobno** użyć Unlock All w `WorldTab` — wybór `sitesOfGrace=true` w `ApplyPvPPreparation` jest **no‑op**.

`needs verification`: docelowa semantyka modułu (bulk unlock wszystkich gracji vs tylko arena bossów vs selected by region) nie jest udokumentowana w kodzie. `disabledNote` mówi „needs UX confirmation”.

## 8. Disabled / placeholder modules

Tylko `SitesOfGrace` jest oznaczone jako placeholder. Pozostałe 4 moduły są aktywne backend + UI. UI dla Sites of Grace jest **wyrenderowany jako disabled checkbox** z różnym styling (`opacity-60`, `cursor-not-allowed`, szary `tierStyle`).

`needs verification`: czy w przyszłości pojawią się dodatkowe moduły (np. „Bosses defeated”, „Item bundle”) — `PvPPreparationOptions` ma sztywno 5 pól, dodanie wymaga zmiany struct + UI + tests.

## 9. Relation to Event Flags

Moduły 2 (Colosseums) i 4 (Summoning Pools) używają **wyłącznie** `db.SetEventFlag` z generic helper API ([15-event-flags.md](15-event-flags.md)):

- Per‑colosseum: 4 flagi `Activate/MapPOI/NPC/Gate` + 3 globalne — łącznie 12 + 3 = **15 flag** (dla 3 colosseów).
- Per pool: 1 flaga ID poola.

Moduły 1 (Regions) i 3 (RevealMap) **też** finalnie operują na bitfieldzie, ale przez wyższe API:

- Module 1 — `core.SetUnlockedRegions` mutuje **osobną strukturę** `UnlockedRegions`, nie bitfield. Patrz [11-regions.md](11-regions.md) §15.
- Module 3 — `revealBaseMap`/`revealDLCMap` ustawiają event flags `62xxx`/`63xxx`/`82xxx` + dodają items + Phase 3 BloodStain. Patrz [27-map-reveal.md](27-map-reveal.md).

Generic helper API (`GetEventFlag`/`SetEventFlag`/BST resolver) jest udokumentowane w 15 — ten rozdział nie powiela.

## 10. Relation to Map Reveal

Moduł 3 (`RevealMap`) wywołuje **dokładnie te same** wewnętrzne funkcje co public endpoint `RevealAllMap` w `app_world.go`:

| Aspekt | `App.RevealAllMap` (`app_world.go:1041`) | `ApplyPvPPreparation` z `RevealMap=true` |
|---|---|---|
| Wywołanie `revealBaseMap` | ✅ | ✅ |
| Wywołanie `revealDLCMap` | ✅ | ✅ |
| Phase 1 flags / Phase 2 items / Phase 3 BloodStain | ✅ identyczne | ✅ identyczne |
| `pushUndo` | ✅ raz (w `RevealAllMap`) | ✅ raz (w `ApplyPvPPreparation`) — wspólny snapshot z innymi modułami |
| UI risk gate | Tier 1 `map_reveal_full` w `WorldTab` | Tier 0 w `PvPPreparationTab` (zgrupowane z innymi PvP modułami) |

`needs verification`: rozjazd Tier 1 (WorldTab) vs Tier 0 (PvPPreparationTab) — celowy (PvP prep ma zbiorczy gate), czy oversight? Brak udokumentowanego rationale.

Phase 3 BloodStain (L2 DLC Cover Layer) — patrz [29-dlc-black-tiles.md](29-dlc-black-tiles.md). PvP prep dziedziczy wszystkie caveats DLC ownership / stale coords / nadpisana eksploracja.

## 11. Relation to Sites of Grace

Patrz §7. Krótko:

- PvP module 5 = **placeholder**. Wybór `sitesOfGrace=true` w `opts` to no‑op + warning.
- Independent grace endpoints (`GetGraces`, `SetGraceVisited`) działają **bezpośrednio** w `WorldTab`. Patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md).
- Companion flags Gatefront (`grace_companion_flags.go::GatefrontGraceEventFlagID`) **nie są** dotykane przez PvP prep.

## 12. Relation to Item Companion Flags

`ApplyPvPPreparation` **nie używa** `data.CompanionEventFlagsForItem` ani item lifecycle hooks (`AddItemsToCharacter` / `RemoveItemsFromCharacter`).

Pośrednio: moduł 3 (`RevealMap`) dodaje fragment items przez `core.AddItemsToSlot` (low‑level), nie przez `App.AddItemsToCharacter` (app‑level). Hook SET companion flag z `app.go:569` **nie odpala się** dla map fragment items. Patrz [50-item-companion-flags.md](50-item-companion-flags.md) §9.

Pośrednio inaczej: moduł 2 (`Colosseums`) ustawia `60100` (jako część `ColosseumGlobalFlags`). Ta sama flaga jest companion dla Spectral Steed Whistle ([50](50-item-companion-flags.md) §6.1). Wybór `Colosseums=true` ustawia `60100` **niezależnie od** posiadania Whistle. Patrz §6.4.

## 13. Relation to Game / World State

`ApplyPvPPreparation` **nie modyfikuje**:

- `PreEventFlagsScalars` (`LastRestedGrace`, `TotalDeathsCount` itp. — patrz [14-game-state.md](14-game-state.md)),
- `WorldGeomMan` / `WorldArea` (patrz [16-world-state.md](16-world-state.md)),
- `PlayerCoordinates` (patrz [17-player-coordinates.md](17-player-coordinates.md)),
- `WorldAreaWeather` / `WorldAreaTime` (patrz [19-weather-time.md](19-weather-time.md)),
- DLC entry flag `CSDlc[1]` (`DlcSectionOffset` — patrz [29-dlc-black-tiles.md](29-dlc-black-tiles.md) §13.3).

Note z `app_pvp.go:82`: „Colosseum matchmaking flags set. **Physical gates may still need to be opened once in-game.**” — flagi `Gate` (710xxx) w `ColosseumFlagSet` to *matchmaking gate marker*, **nie** fizyczna brama. Fizyczna brama jest w `WorldGeomMan` blob i nie da się jej edytować z poziomu save editora.

## 14. Write path and rollback caveats

### 14.1 Single snapshot, no per‑module rollback

`pushUndo` raz na linii 41. Wszystkie mutacje 4 aktywnych modułów dzielą ten snapshot. Pop snapshot przez Undo cofa **całość**. Brak per‑module Undo (np. „cofnij tylko Colosseums”).

### 14.2 Fail‑fast bez auto‑restore

Pierwszy error w module 1–4 powoduje `return nil, err`. Kolejne moduły nie wykonują się, ale **mutacje wcześniejszych modułów pozostają w `slot.Data`**. Snapshot z `pushUndo` jest na stosie Undo, ale **nie jest** automatycznie poppowany. Użytkownik widzi error w UI (toast) i musi sam nacisnąć Undo.

`needs verification`: czy istnieje user‑facing test fail‑fast — np. moduł 4 (Summoning Pools) failuje a moduły 1–3 wykonane. Aktualne 10 testów nie pokrywa tego scenariusza.

### 14.3 Slice invalidation between modules

`flags` slice (`slot.Data[slot.EventFlagsOffset:]`) jest **explicit refreshowany** w 2 miejscach:

1. Po `MatchmakingRegions` (linia 57) — `core.SetUnlockedRegions` realokuje przez `RebuildSlot`.
2. Po `RevealMap` (linia 94) — `AddItemsToSlot` (wewnątrz Phase 2) realokuje.

Brak refresh przed `Colosseums` (linia 61) — `Colosseums` używa `flags` zaderywowanego na linii 43 lub odświeżonego w linii 57. OK, bo między tymi punktami nie ma realokacji.

Brak refresh przed `SummoningPools` — `flags` z poprzedniego refresh (`RevealMap` lub `MatchmakingRegions`) jest aktualny. OK.

### 14.4 Order matters

Kolejność modułów w kodzie:

1. MatchmakingRegions
2. Colosseums
3. RevealMap
4. SummoningPools
5. SitesOfGrace

Ta kolejność jest **wymuszona** przez sekwencyjne `if opts.X { ... }`. Użytkownik nie może zmienić kolejności przez UI. Jeśli user chce „tylko RevealMap, potem MatchmakingRegions” — nie da się; albo wszystko w wyznaczonej kolejności, albo ręczne wywołania osobno.

`needs verification`: czy kolejność ma wpływ na correctness. Z analizy: `MatchmakingRegions` realokuje (musi być przed innymi, które używają `flags`); `RevealMap` realokuje (musi być przed SummoningPools/SitesOfGrace). Sekwencja kodowana jest correctness‑wise OK.

### 14.5 Brak transakcji per‑module

`db.SetEventFlag` w modułach 2 i 4 — jeśli któraś flaga failuje, błąd jest propagowany (`return nil, err`), **wcześniejsze flagi z tego samego modułu** już są SET. Częściowy efekt jest możliwy.

## 15. UI / frontend status

`frontend/src/components/PvPPreparationTab.tsx`:

| Element UI | Co robi |
|---|---|
| Profile picker | 3 profile (`minimal`, `full`, `coop`) + read‑only `custom` |
| `MODULES` lista 5 elementów | Każdy z labelem, tier‑em, descem; `SitesOfGrace` ma `disabled: true` |
| Per‑module checkbox `<Chk>` | Toggle `pvpOpts[key]` — Sites of Grace nieklikalne |
| Apply button | Wywołuje `ApplyPvPPreparation(charIdx, payload)`, parsuje warnings, pokazuje toast |
| `NetworkSpeedPanel` | Embedded — patrz [44-network-param-tuning.md](44-network-param-tuning.md); osobny endpoint, **nie** część `ApplyPvPPreparation` |
| Notes panel (warnings) | Renderuje warnings zwrócone z backendu |

3 profile (z `PROFILE_OPTS`):

```ts
minimal: { matchmakingRegions: true,  colosseums: false, revealMap: false, summoningPools: false, sitesOfGrace: false }
full:    { matchmakingRegions: true,  colosseums: true,  revealMap: true,  summoningPools: false, sitesOfGrace: false }
coop:    { matchmakingRegions: false, colosseums: false, revealMap: true,  summoningPools: true,  sitesOfGrace: false }
```

`sitesOfGrace` w profilach jest zawsze `false` — żaden profile nie próbuje włączać placeholder modułu.

`resolveProfile` (linia 87) **NIE** porównuje `sitesOfGrace` — porównuje tylko 4 aktywne pola. To znaczy: profil rozpoznawany jest poprawnie nawet gdy `sitesOfGrace=true` (ale to i tak no‑op). `needs verification`: czy to celowe, czy bug.

`anySelected` (linia 152) wyklucza `disabled` moduły: `MODULES.some(m => !m.disabled && pvpOpts[m.key])`. Apply button disabled jeśli żaden aktywny moduł nie zaznaczony.

## 16. Validation and safety notes

### 16.1 Overclaim „PvP ready”

Nazwa „PvP‑ready” sugeruje, że po jednym kliku postać jest gotowa do PvP. W rzeczywistości:

- Matchmaking Regions: ✅ wystarcza dla podstawowej eligibility do invasions.
- Colosseums: ✅ matchmaking flags + map markers, ale **fizyczne bramy** wymagają in‑game open.
- RevealMap: ✅ ale to QoL, nie warunek PvP.
- SummoningPools: warning literal — „**Bloody Finger invasion impact is unconfirmed**”.
- SitesOfGrace: placeholder.

`needs verification`: czy w aktualnym balansie matchmakingu wymagane są dodatkowe warunki (Stats Level range, item w inwentarzu, NG+ tier) — `ApplyPvPPreparation` nie modyfikuje żadnego z tych.

### 16.2 Quest / world progression side effects

Moduł 2 (Colosseums) ustawia 15 flag, w tym `60100` (Spectral Steed Whistle obtained flag — patrz §12). Side effects:

- `60100` SET na save sprzed Melina encounter → Torrent może być summoned bez Whistle (gameplay change).
- Inne flagi (`Gate`/`NPC`/`MapPOI`) — `needs verification`, czy ustawiają cokolwiek poza PvP matchmakingiem.

Moduł 4 (SummoningPools) ustawia ~213 flag. `needs verification`, czy któraś z nich kolizjonuje z innymi systemami gry.

### 16.3 Map reveal side effects

Patrz [27-map-reveal.md](27-map-reveal.md) §13 + [29-dlc-black-tiles.md](29-dlc-black-tiles.md) §13. Główne ryzyka:

- DLC ownership mismatch (`CSDlc` nie sprawdzane),
- Phase 3 BloodStain coords mogą nadpisać autentyczną eksplorację,
- visual reveal vs gameplay progression (trofea — `needs verification`).

### 16.4 Item companion flags

Patrz §12. Krótko: `ApplyPvPPreparation` może SET'ować flagi z item companion sets (Whistle 60100) **bez** odpowiadającego itemu w inwentarzu, jeśli moduł 2 (Colosseums) jest aktywny. To celowe (Colosseums potrzebują 60100 jako globalna flaga), ale tworzy „flag bez itemu” edge case opisany w [50-item-companion-flags.md](50-item-companion-flags.md) §12.4.

### 16.5 Sites of Grace placeholder

User klikający „Sites of Grace” w UI dostanie warning po Apply — ale checkbox jest `disabled`, więc realnie nie da się kliknąć. **Backend jednak akceptuje** `opts.SitesOfGrace=true` (np. od test code / direct JS call) i zwraca warning. `needs verification`: czy są zewnętrzne wywołania, które przekazują `sitesOfGrace=true` z oczekiwaniem mutacji.

### 16.6 Rollback / atomicity gaps

Patrz §14.1, §14.2, §14.5. Główne:

- Brak per‑module Undo.
- Brak auto‑restore po fail‑fast.
- Brak transakcji wewnątrz modułu (partial flag SET przy error).

### 16.7 In‑game verification gaps

10 testów w `pvp_test.go` pokrywa:

- 4 validation paths (no save, invalid slot, empty slot, bad offset),
- 3 warnings (Sites of Grace, Colosseum, Summoning Pools — assert string),
- 3 mutation paths (Colosseums mutate, Summoning Pools mutate, EventFlag roundtrip).

**Nie pokrywa**:

- in‑game runtime verification (czy Bloody Finger faktycznie pojawia się invaderowi),
- fail‑fast cleanup (partial state w `slot.Data` po error),
- cross‑module ordering edge cases,
- PvP module RevealMap (komentarz w `pvp_test.go:14‑15`: „revealBaseMap/revealDLCMap will fail on minimal save — those are tested via roundtrip/integration tests”).

### 16.8 Platform / version differences

ID regionów, pooli, colosseów, gracji są snapshotami z `regulation.bin`. Po patchu gry:

- nowe regiony/poole/colosseum mogą nie być w bazie (ten sam problem co w [11-regions.md](11-regions.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md)),
- semantyka flag może się zmienić (rzadko, ale możliwe dla world state flags).

`needs verification`: brak automatycznej detekcji „regulation.bin newer than data snapshot”.

## 17. Test coverage

10 testów w `pvp_test.go`:

| Test | Linia | Co weryfikuje |
|---|---|---|
| `TestApplyPvPPreparation_NoSave` | 27 | `a.save == nil` → error „no save loaded” |
| `TestApplyPvPPreparation_InvalidSlotIndex` | 35 | `slotIndex < 0 / >= 10` → error |
| `TestApplyPvPPreparation_EmptySlot` | 45 | `slot.Version == 0` → error |
| `TestApplyPvPPreparation_BadFlagsOffset` | 54 | `EventFlagsOffset` poza range → error |
| `TestApplyPvPPreparation_SitesOfGraceWarning` | 66 | `opts.SitesOfGrace=true` → warning literal „planned but not enabled” |
| `TestApplyPvPPreparation_ColosseumWarning` | 77 | `opts.Colosseums=true` → warning o physical gates |
| `TestApplyPvPPreparation_SummoningPoolsWarning` | 94 | `opts.SummoningPools=true` → warning Bloody Finger unconfirmed |
| `TestApplyPvPPreparation_ColosseumsMutate` | 113 | 12+3 flagi colosseums SET w bitfieldzie |
| `TestApplyPvPPreparation_SummoningPoolsMutate` | 137 | Wszystkie pool IDs SET |
| `TestApplyPvPPreparation_EventFlagRoundtrip` | 166 | SET + readback przez `db.GetEventFlag` |

Komentarz w `pvp_test.go:11‑16`: minimal save z 20 KB EventFlags region; nie inicjalizuje dynamic offsets — moduły `MatchmakingRegions` (`core.SetUnlockedRegions`) i `RevealMap` (`revealBaseMap/revealDLCMap`) **failują** w minimal save i nie są tu testowane. Są pokryte „roundtrip/integration tests that load real save files”.

`needs verification`: lista „integration tests that load real save files” dla PvP prep — czy istnieje konkretny test wywołujący `ApplyPvPPreparation` z `opts.RevealMap=true` na realnym slocie. Nie znaleziono w `tests/`.

## 18. Known limits / needs verification

1. **Sites of Grace docelowa semantyka** — placeholder, brak udokumentowanego targetu.
2. **`ColosseumFlagSets` extensibility** — nowe DLC colosseum wymaga ręcznej synchronizacji `ColosseumFlagSets` z `GetAllColosseums`.
3. **60100 cross‑contamination** — flaga współdzielona Colosseums ↔ Whistle, intencjonalnie? `needs verification`.
4. **Tier mismatch RevealMap** — Tier 1 w `WorldTab`, Tier 0 w `PvPPreparationTab`. Celowe?
5. **Fail‑fast auto‑restore** — brak; mutacje wcześniejszych modułów zostają.
6. **Per‑module Undo** — brak; tylko bulk Undo.
7. **Partial mutation po error w pętli** — możliwe w modułach 2 i 4.
8. **`resolveProfile` ignoruje `sitesOfGrace`** — celowo czy bug.
9. **Integration test `RevealMap` w PvP prep context** — brak izolowanego testu.
10. **Liczba regionów / pooli stale after patch** — brak detekcji.
11. **Backend akceptuje `sitesOfGrace=true` mimo UI disabled** — direct JS call może obejść UI gating.
12. **Physical colosseum gates w WorldGeomMan** — nieosiągalne przez save editor (`needs verification` czy jakikolwiek edycja blob jest planowana).
13. **In‑game verification PvP matchmaking** — manualne, brak CI.

## 19. Cross‑references

- [11-regions.md](11-regions.md) — Moduł 1 (Matchmaking Regions) → `core.SetUnlockedRegions`, 274-regionowa skuratowana allowlista (niedestrukcyjny merge).
- [14-game-state.md](14-game-state.md) — `ApplyPvPPreparation` nieruszone.
- [15-event-flags.md](15-event-flags.md) — generic helper API używany przez moduły 2 i 4.
- [16-world-state.md](16-world-state.md) — `WorldGeomMan` blob nieruszony (physical colosseum gates).
- [27-map-reveal.md](27-map-reveal.md) — Moduł 3 (RevealMap) → `revealBaseMap` + `revealDLCMap`.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — L2 DLC Cover Layer; dziedziczone caveats.
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — standalone grace endpoints; PvP module 5 placeholder.
- [50-item-companion-flags.md](50-item-companion-flags.md) — `60100` współdzielony z Whistle companion set.

## 20. Sources

- `app_pvp.go::ApplyPvPPreparation` (linie 25‑113) — single endpoint.
- `app_pvp.go::PvPPreparationOptions` (linie 13‑19) — 5 bool fields.
- `backend/db/data/summoning_pools.go::ColosseumFlagSets` / `ColosseumGlobalFlags` / `ColosseumFlagSet.AllFlags()` (linie 335‑367).
- `backend/db/db.go::GetAllRegions` / `GetAllColosseums` / `GetAllSummoningPools` (sync.OnceValue cache).
- `frontend/src/components/PvPPreparationTab.tsx` — UI: 5 modułów `MODULES`, 3 profile `PROFILE_OPTS`, `resolveProfile`, `anySelected`, `NetworkSpeedPanel` embed.
- `pvp_test.go` — 10 testów (4 validation + 3 warnings + 3 mutation/roundtrip).
- `app_world.go::revealBaseMap` / `revealDLCMap` — wspólne z `RevealAllMap`.
- `docs/CHANGELOG.md` — historyczne ad‑hoc runtime PvP verification (Steam Deck, PS4).
