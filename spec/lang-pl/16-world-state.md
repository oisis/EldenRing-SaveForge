# 16 — World State

> **Type**: Binary format spec + overview chapter
> **Scope**: Index po subsystemach World State w SaveForge — co jest read‑only blob, co ma write helpers, jak rozdziela się od Game State i Event Flags.

Cross‑refs: [11-regions.md](11-regions.md), [14-game-state.md](14-game-state.md), [15-event-flags.md](15-event-flags.md), [17-player-coordinates.md](17-player-coordinates.md), [19-weather-time.md](19-weather-time.md), [27-map-reveal.md](27-map-reveal.md), [29-dlc-black-tiles.md](29-dlc-black-tiles.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md), [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md), [50-item-companion-flags.md](50-item-companion-flags.md).

---

## 1. Cel rozdziału

Zdefiniować jednoznacznie:

- co SaveForge traktuje jako **World State** (sekcje binarne reprezentujące stan fizycznego świata gry),
- które części są **read‑only blobami** (parsowane verbatim, zapisywane bez edycji),
- które części mają **write helpers** w `app_world.go` (głównie pośrednio przez event flag bitfield),
- jak World State rozdziela się od Game State ([14](14-game-state.md)) i Event Flags ([15](15-event-flags.md)),
- gdzie jest cienka granica „blob read‑only ale mamy 1 partial mutator” — `BloodStain` w kontekście DLC tiles.

Ten rozdział jest **overview chapter** — nie powiela szczegółów write paths (są w 11/15/27/29/47/48/50). Pełni rolę index'u po subsystemach.

## 2. Status

| Aspekt | Status |
|---|---|
| Binary structure parsers (`backend/core/section_world*.go`) | ✅ implemented — read/write verbatim |
| `WorldGeomBlock` (5 size‑prefixed blobs) | 🔒 read‑only (`SizePrefixedBlob` verbatim copy) |
| `BloodStain` section | 🔒 read‑only **z 1 wyjątkiem**: L2 DLC cover layer (`revealDLCMap` Phase 3) — patrz [29](29-dlc-black-tiles.md) |
| `RideGameData` (Torrent horse state) | 🔒 read‑only verbatim |
| `WorldAreaWeather` / `WorldAreaTime` | 🔒 read‑only verbatim — patrz [19](19-weather-time.md) |
| Event flag bitfield (caller domain) | ✅ write helpers w `app_world.go` (~34 App methods po ~12 kategoriach: Graces / Bosses / Pools / Regions / Colosseums / Gestures / Quests / Cookbooks / Bells / Whetblades / Ashes / Map) |
| `UnlockedRegions` array | ✅ write helpers (`core.SetUnlockedRegions` + UI) — patrz [11](11-regions.md) |
| Map reveal orchestration | ✅ `RevealAllMap` / `RemoveFogOfWar` / `ResetMapExploration` — patrz [27](27-map-reveal.md) |
| PvP preset orchestration | ✅ `ApplyPvPPreparation` (5 modules, 1 placeholder) — patrz [48](48-pvp-ready-modular-presets.md) |
| Game State (LastRestedGrace, TotalDeathsCount itp.) | 🔒 NOT modyfikowane przez `app_world.go` — patrz [14](14-game-state.md) |

## 3. Source of truth w kodzie

| Plik / symbol | Co zawiera | Tryb |
|---|---|---|
| `backend/core/section_world_geom.go::WorldGeomBlock` (linie 54‑96, struct + Read/Write/ByteSize) | `FieldArea + WorldArea + WorldGeomMan + WorldGeomMan2 + RendMan` — 5 `SizePrefixedBlob`s | read/write verbatim |
| `backend/core/section_world_geom.go::SizePrefixedBlob` (linie 17‑48) | `(size: i32, data: bytes[size])` z sanity ceilings | helper |
| `backend/core/section_world.go::RideGameData` | Torrent horse state — `Coordinates + MapID + Angle + HP + State` (40 B) | read/write verbatim |
| `backend/core/section_trailing.go::WorldAreaWeather` / `WorldAreaTime` (12+12 B) | Trailing fixed block, patrz [19](19-weather-time.md) | read/write verbatim |
| `backend/core/offset_defs.go::DynBloodStain` (`= 0x4C`) | Offset blob `BloodStain` | constant |
| `backend/core/offset_defs.go::DLCTile*` (linie 309‑322) | Partial mutator dla L2 DLC tiles wewnątrz `BloodStain` | patrz [29](29-dlc-black-tiles.md) |
| `app_world.go` (1200+ linii) | Wszystkie write helpers per‑subsystem | App‑level (Wails binding) |
| `app_pvp.go::ApplyPvPPreparation` | Orchestrator 5 modułów | App‑level |
| `frontend/src/components/WorldTab.tsx` | UI: graces / bosses / pools / colosseums / regions / map / gestures / cookbooks / bell bearings / whetblades / quests / Ashes | UI |
| `frontend/src/components/PvPPreparationTab.tsx` | UI dla preset orchestrator | UI |

`needs verification`: lista 5 blobów w `WorldGeomBlock` jest snapshotem; sanity ceilings (`fieldAreaMaxSize = 0x10000`, `worldAreaMaxSize = 0x10000`, `worldGeomManMaxSize = 0x100000`, `rendManMaxSize = 0x100000`) mogą się zmienić po patchu gry.

## 4. Mental model

```
┌────────────────────────────── World State ─────────────────────────────┐
│                                                                          │
│  ┌─ EVENT FLAG BITFIELD (write helpers via app_world.go) ─────────┐      │
│  │   • Graces / Bosses / Pools / Colosseums / Regions / Map       │      │
│  │     / Gestures / Cookbooks / Bell Bearings / Whetblades        │      │
│  │     / Quests / Ashes (Tier 1 / Tier 2 per category)            │      │
│  │   • Wszystko przechodzi przez db.SetEventFlag (15)              │      │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ UNLOCKED REGIONS array (write helper core.SetUnlockedRegions) ┐      │
│  │   • Variable-length []u32; mutacja przez RebuildSlot            │      │
│  │   • Patrz [11]                                                  │      │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ BloodStain section (offset DynBloodStain = 0x4C) ──────────────┐    │
│  │   • Verbatim blob z 1 wyjątkiem: L2 DLC tile coords              │    │
│  │   • Patrz [29] dla DLCTile* constants i partial mutator           │    │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ WorldGeomBlock (verbatim blobs, 5 sekcji size-prefixed) ──────┐     │
│  │   FieldArea + WorldArea + WorldGeomMan + WorldGeomMan2 + RendMan│     │
│  │   • Nigdy nie mutowane przez SaveForge                            │     │
│  │   • Read/write through SizePrefixedBlob.Read/Write                │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ RideGameData (Torrent horse) ──────────────────────────────────┐    │
│  │   40 B; verbatim blob — żaden endpoint nie modyfikuje              │    │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ WorldAreaWeather + WorldAreaTime (trailing) ───────────────────┐    │
│  │   24 B total; verbatim — patrz [19]                                │    │
│  └─────────────────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────────────────┘

  ─── poza World State ───
  
  ┌─ Game State (PreEventFlagsScalars + LastRestedGrace) ───┐
  │   Patrz [14]; NOT modyfikowane przez app_world.go         │
  └────────────────────────────────────────────────────────────┘
  
  ┌─ Player Coordinates ─────────────────────────────────────┐
  │   Patrz [17]; oddzielna sekcja                             │
  └────────────────────────────────────────────────────────────┘
```

## 5. World State subsystem map

Tabela subsystemów z dokładnie jednym wskaźnikiem do canonical chapter:

| Subsystem | Canonical chapter | Tryb edycji | Główne endpointy |
|---|---|---|---|
| Event Flags (fundament) | [15](15-event-flags.md) | RW helper API | `db.GetEventFlag` / `db.SetEventFlag` |
| Regions / `UnlockedRegions` | [11](11-regions.md) | RW (RebuildSlot) | `GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions` |
| Map Reveal (4 warstwy) | [27](27-map-reveal.md) | RW orchestrator | `GetMapProgress`, `SetMapFlag`, `SetMapRegionFlags`, `RevealAllMap`, `ResetMapExploration`, `RemoveFogOfWar` |
| DLC Black Tiles (L2) | [29](29-dlc-black-tiles.md) | RW partial mutator w `BloodStain` | tylko via `revealDLCMap` Phase 3 |
| Sites of Grace | [47](47-site-of-grace-activation.md) | RW | `GetGraces`, `SetGraceVisited` |
| Item Companion Flags | [50](50-item-companion-flags.md) | RW hooks SET+CLEAR | tylko via `AddItemsToCharacter` / `RemoveItemsFromCharacter` |
| PvP modular presets | [48](48-pvp-ready-modular-presets.md) | RW orchestrator | `ApplyPvPPreparation` (5 modules, 1 placeholder) |
| Bosses / Pools / Colosseums / Gestures / Cookbooks / Bells / Whetblades / Quests / Ashes | n/d — direct app_world.go | RW per‑category | `GetXxx` / `SetXxxYyy` / `BulkSetXxx` |
| Game State (LastRestedGrace itp.) | [14](14-game-state.md) | RO **z perspektywy World State** | `app_world.go` **nie modyfikuje** |
| WorldAreaWeather / Time | [19](19-weather-time.md) | RO verbatim | `section_trailing.go` |
| Player Coordinates | [17](17-player-coordinates.md) | RO/RW per chapter | `section_player_coords.go` |

`needs verification`: kategorie „Bosses / Pools / Colosseums / Gestures / Cookbooks / Bells / Whetblades / Quests / Ashes” nie mają dedykowanych canonical chapterów w Phase 4 — pokryte są poprzez `15-event-flags.md` (generic API) + per‑category snapshots w `backend/db/data/`. Brak dedykowanego rozdziału jest aktualnym stanem; przyszłe rozdziały mogą dojść.

## 6. Read‑only vs write‑capable areas

### 6.1 Verbatim read‑only blobs (NIE mutowane przez SaveForge)

| Sekcja | Rozmiar typowy | Sanity ceiling | Komentarz |
|---|---|---|---|
| `FieldArea` | zmienne | `0x10000` (64 KB) | dane regionu w którym gracz się znajduje |
| `WorldArea` | zmienne | `0x10000` (64 KB) | NPC/postaci w świecie per mapa (`WorldAreaChrData`) |
| `WorldGeomMan` (instancja 1) | zmienne | `0x100000` (1 MB) | geometria świata per mapa |
| `WorldGeomMan2` (instancja 2) | zmienne | `0x100000` (1 MB) | druga instancja — prawdopodobnie before/after lub two layers |
| `RendMan` | zmienne | `0x100000` (1 MB) | renderer manager (oświetlenie, particles) |
| `RideGameData` | 40 B (`RideGameDataSize`) | n/d | Torrent horse: `Coordinates + MapID + Angle + HP + State` |
| `WorldAreaWeather` | 12 B | n/d | patrz [19](19-weather-time.md) |
| `WorldAreaTime` | 12 B | n/d | patrz [19](19-weather-time.md) |

`needs verification`: dokładny offset i kolejność tych sekcji w binarnym save'ie. Aktualny kod używa typowanego parsera (`SizePrefixedBlob.Read/Write`), nie hex offsetów per sekcja. Patrz `backend/core/section_world_geom.go` dla kolejności w `WorldGeomBlock.Read`.

### 6.2 Częściowo mutowane (1 wyjątek)

| Sekcja | Co edytujemy | Caller |
|---|---|---|
| `BloodStain` (offset `DynBloodStain = 0x4C`) | tylko `[0x0088..0x0110)` (DLC tile coords) | `revealDLCMap` Phase 3 — patrz [29](29-dlc-black-tiles.md) |

To **jedyny** punkt, w którym SaveForge mutuje cokolwiek wewnątrz blob'a World State innego niż `WorldGeomBlock` / `RideGameData` / weather/time. Reszta `BloodStain` (poza zakresem `[0x0088..0x0110)`) jest verbatim.

### 6.3 Write‑capable via event flag bitfield

Wszystkie poniższe endpointy z `app_world.go` operują **wyłącznie** na bitfield Event Flags (patrz [15](15-event-flags.md)) — **nie** na blobach World State:

```
GetGraces / SetGraceVisited
GetBosses / SetBossDefeated
GetSummoningPools / SetSummoningPoolActivated
GetUnlockedRegions / SetRegionUnlocked / BulkSetUnlockedRegions  ← jedyny operujący na NIE-bitfieldzie (UnlockedRegions array)
GetColosseums / SetColosseumUnlocked
GetGestures / SetGestureUnlocked / BulkSetGesturesUnlocked
GetQuestNPCs / GetQuestProgress / SetQuestStep
GetCookbooks / SetCookbookUnlocked / BulkSetCookbooksUnlocked
GetBellBearings / SetBellBearingUnlocked / BulkSetBellBearings
GetWhetblades / SetWhetbladeUnlocked
GetAshOfWarFlags / SetAshOfWarFlagUnlocked / BulkSetAshOfWarFlags
GetMapProgress / SetMapFlag / SetMapRegionFlags / RevealAllMap / ResetMapExploration / RemoveFogOfWar
```

Wszystkie write paths przechodzą przez `db.SetEventFlag(flags, id, value)` z generic helper API ([15](15-event-flags.md)). Per‑category snapshoty ID są w `backend/db/data/`.

## 7. Event Flags relation

Event flag bitfield ([15](15-event-flags.md)) jest **fundamentem** większości mutacji World State. Wszystkie subsystemy z §5 (z wyjątkiem `UnlockedRegions` / DLC tiles / verbatim blobs / Player Coordinates) operują wyłącznie na flagach.

Ten rozdział **nie powiela** byte/bit indexing, BST resolvera ani helper API. Patrz [15](15-event-flags.md) §1–§9.

## 8. Map Reveal relation

Map Reveal ([27](27-map-reveal.md)) to 4‑layer model:

- **L0** `UnlockedRegions` — patrz [11](11-regions.md),
- **L1** event flags 62xxx/63xxx + Map Fragment items,
- **L2** DLC Cover Layer (BloodStain coords) — patrz [29](29-dlc-black-tiles.md),
- **L3** Fog of War bitfield.

`RevealAllMap` w `app_world.go:1041` jest orchestratorem L1+L2+L3 (poza L0). Patrz [27](27-map-reveal.md) §11 dla pełnego pseudokodu.

Ten rozdział **nie powiela** modelu warstw. Linkuje.

## 9. Regions relation

`UnlockedRegions` ([11](11-regions.md)) jest **jedynym** subsystemem World State, który:

- nie jest bitfieldem event flag,
- ma własny variable‑length array `[]u32`,
- wymaga `core.SetUnlockedRegions` + `RebuildSlot` (realokacja `slot.Data`),
- jest L0 layer Map Reveal.

Skutkiem `RebuildSlot` jest rekalkulacja `EventFlagsOffset` — wszystkie write helpers z §6.3 **muszą** odświeżyć slice `flags = slot.Data[slot.EventFlagsOffset:]` po `BulkSetUnlockedRegions` lub `SetRegionUnlocked` z mutacją długości. W `ApplyPvPPreparation` to się dzieje explicit (`app_pvp.go:57`).

## 10. DLC black tiles relation

L2 DLC Cover Layer ([29](29-dlc-black-tiles.md)) to **jedyna** partial mutation wewnątrz `BloodStain` blobu. Aktualizowana wyłącznie przez `revealDLCMap` Phase 3 z 9 stałymi koordynat (Rec1 X/Y/Flag + Rec2 X/Y/Z/W/Flag).

Stale‑after‑patch caveat: koordynaty są empiryczne (snapshot z 2 slotów referencyjnych), nie game‑guaranteed. Patrz [29](29-dlc-black-tiles.md) §13.1 + §15.

## 11. Sites of Grace relation

Sites of Grace ([47](47-site-of-grace-activation.md)) operują na event flag bitfield z pasm 71xxx–76xxx + opcjonalny `DoorFlag` + companion flags (Gatefront only).

Ważne rozróżnienie z PvP context:

- **Standalone**: `GetGraces` / `SetGraceVisited` w `app_world.go` są aktywne; UI w `WorldTab.tsx` (per‑grace, per‑region, Unlock All Tier 1).
- **PvP preset module E** (`opts.SitesOfGrace` w `ApplyPvPPreparation`): **placeholder** — zwraca warning bez mutacji. Patrz [48](48-pvp-ready-modular-presets.md) §7.

Companion flag policy: SET‑only asymmetric ([47](47-site-of-grace-activation.md) §9). Kontrastuje z SET+CLEAR symmetric w item companion flags ([50](50-item-companion-flags.md) §7).

## 12. Item Companion Flags relation

Item companion flags ([50](50-item-companion-flags.md)) są **bridge** między item ownership a event flag state. Hook SET w `AddItemsToCharacter`, hook CLEAR w `RemoveItemsFromCharacter` (po usunięciu ostatniej instancji).

Aktualnie 6 wpisów: Spectral Steed Whistle (4 companion flags) + 5 multiplayer items (1 flaga each). Wpływ na World State: ustawiane flagi są w pasmach 60xxx (obtained) + 4680/4681 (Melina state) + 710520 (whistle world state).

`MapFragmentItems` (z [27](27-map-reveal.md)) NIE jest częścią item companion flags — osobny mechanizm. Patrz [50](50-item-companion-flags.md) §9 dla detail.

## 13. PvP‑ready presets relation

PvP modular presets ([48](48-pvp-ready-modular-presets.md)) to **orchestrator** kilku subsystemów World State w jednym wywołaniu `ApplyPvPPreparation`:

- Module 1 `MatchmakingRegions` → `core.SetUnlockedRegions` (L0 — [11](11-regions.md)),
- Module 2 `Colosseums` → bulk event flag SET (15 flag total),
- Module 3 `RevealMap` → `revealBaseMap` + `revealDLCMap` (L1+L2+L3 — [27](27-map-reveal.md), [29](29-dlc-black-tiles.md)),
- Module 4 `SummoningPools` → bulk event flag SET,
- Module 5 `SitesOfGrace` → **placeholder** (no mutation).

Single `pushUndo`, fail‑fast bez auto‑restore. Patrz [48](48-pvp-ready-modular-presets.md) §14 dla rollback caveats.

## 14. Game State relation

Game State ([14](14-game-state.md)) zawiera `PreEventFlagsScalars` (29 B blok bezpośrednio przed bitfield Event Flags) z polami:

- `LastRestedGrace` (BonfireId — patrz [47](47-site-of-grace-activation.md) §6.3),
- `TotalDeathsCount`, `ClearCount`, `CharacterType`, `InOnlineSessionFlag`, `InGameCountdownTimer` itp.

`app_world.go` **nie modyfikuje** żadnego z tych pól. Mutacje Game State (jeśli istnieją) idą przez osobne endpointy (`app.go`, `app_character.go` etc. — patrz [14](14-game-state.md)).

`needs verification`: lista write endpointów dla Game State w aktualnym kodzie — outside scope tego rozdziału.

## 15. Current implemented behavior

### 15.1 Read endpoints

Wszystkie `GetXxx` endpointy z §6.3 są **tolerancyjne**:

- Brak `EventFlagsOffset` → zwraca pełną listę z `Visited/Defeated/Activated/...=false`.
- Brak save / invalid slot → error.
- Per‑entry error w bitfieldzie → `fmt.Printf` warning, kontynuuje iterację (NIE propagacja).

Wynik: `GetXxx` może zwrócić częściową informację z 0 errorów — UI musi to świadomie obsłużyć.

### 15.2 Per‑category write endpointy

Każdy `SetXxxYyy` w `app_world.go`:

1. Validuje `save != nil`, slot index in range, `EventFlagsOffset > 0`.
2. Wywołuje `a.pushUndo(slotIndex)` — pojedynczy snapshot per call.
3. Mutuje 1 lub kilka bitów event flag.
4. (Per niektóre kategorie) Dodatkowo mutuje powiązane flagi (np. `DoorFlag` w `SetGraceVisited`).

Bulk endpointy (`BulkSetXxx`) iterują po liście ID w **jednym** wywołaniu — single `pushUndo`, sekwencyjny SET. Brak rollbacku przy partial failure.

### 15.3 Orchestratory

- `RevealAllMap` ([27](27-map-reveal.md)) — single `pushUndo` + 3 fazy (L1 flagi → L1 items → L2 BloodStain).
- `ApplyPvPPreparation` ([48](48-pvp-ready-modular-presets.md)) — single `pushUndo` + 5 modułów fail‑fast.
- `ResetMapExploration` — single `pushUndo` + bulk CLEAR map flags.
- `RemoveFogOfWar` — single `pushUndo` + bulk SET FoW bitfield.

## 16. Planned / placeholder / research‑only areas

| Obszar | Status | Lokalizacja |
|---|---|---|
| PvP module E (Sites of Grace) | placeholder — warning only | `app_pvp.go:108‑110`; patrz [48](48-pvp-ready-modular-presets.md) §7 |
| Companion flags poza Whistle + 5 multiplayer items | research candidate (Talisman Pouch, others) | patrz [50](50-item-companion-flags.md) §6.3 i §14 |
| Companion flags poza Gatefront | research candidate dla pozostałych ~418 gracji | patrz [47](47-site-of-grace-activation.md) §8 |
| L2 DLC tile coords cross‑version | empiryczne — `needs verification` po patchu | patrz [29](29-dlc-black-tiles.md) §13.1 |
| In‑world activation per‑category gracji | zweryfikowane tylko Church of Elleh | patrz [47](47-site-of-grace-activation.md) §7 |
| Physical colosseum gates (WorldGeomMan blob) | NIE da się edytować z poziomu save editora | patrz [48](48-pvp-ready-modular-presets.md) §13 |
| WorldAreaWeather/Time write helpers | brak — verbatim blob | patrz [19](19-weather-time.md) |
| RideGameData (Torrent horse) write helpers | brak — verbatim blob | `section_world.go` |
| WorldGeomBlock direct edit | NIE robione — corruption risk | patrz §17.1 |

## 17. Write path and rollback caveats

### 17.1 Cross‑subsystem atomicity

`app_world.go` nie ma **żadnego** mechanizmu transakcyjnego dla mutacji obejmujących wiele subsystemów. Każdy endpoint robi własny `pushUndo`. Bulk endpointy (`BulkSetXxx`) robią **jeden** snapshot dla całej listy, nie per‑item.

Orchestratory (`RevealAllMap`, `ApplyPvPPreparation`, `ResetMapExploration`, `RemoveFogOfWar`) też mają jeden `pushUndo` na całą operację.

**Konsekwencja**: jeśli orchestrator failuje w środku (np. moduł 4 w PvP prep), wcześniejsze mutacje (moduły 1–3) zostają w `slot.Data`. Snapshot z `pushUndo` jest dostępny przez Undo, ale **nie jest** automatycznie poppowany.

### 17.2 WorldGeomBlock corruption risk

`WorldGeomBlock` (5 sekcji size‑prefixed) jest **zawsze** czytany i pisany verbatim przez `SizePrefixedBlob.Read/Write`. Każda ręczna mutacja:

- może uszkodzić `Size` header → parser przy następnym load zwróci `out‑of‑range` lub crash,
- może uszkodzić wewnętrzny layout (`WorldBlockChrData`, `WorldGeomDataChunk`) → respawn NPC w złych miejscach, brak obiektów, crash gry przy load obszaru.

SaveForge **celowo** nie eksponuje API do edycji `WorldGeomBlock`. Cross‑platform conversion (PC ↔ PS4) kopiuje te bloby verbatim.

`needs verification`: czy istnieją save'y z PS4 vs PC, w których layout `WorldGeomBlock` różni się binarnie (poza paddingiem). Aktualne testy round‑trip (`tests/roundtrip_test.go`) sprawdzają identity round‑trip, nie cross‑platform diff.

### 17.3 BloodStain partial mutator caveats

`revealDLCMap` Phase 3 zeruje `[0x0085..0x0110)` w `BloodStain` i wpisuje 9 wartości syntetycznych. To **destrukcyjne** dla autentycznej eksploracji DLC przez gracza (jeśli była). Patrz [29](29-dlc-black-tiles.md) §13.2.

### 17.4 Slice invalidation

Każdy endpoint, który robi `slot.Data` realloc (przez `RebuildSlot` lub `AddItemsToSlot`), musi odświeżyć `flags := slot.Data[slot.EventFlagsOffset:]` jeśli operuje dalej na bitfieldzie. Wzorzec widoczny w `ApplyPvPPreparation` (linie 57, 94) i `revealDLCMap` (linia 1096).

### 17.5 Brak per‑subsystem audit log

`app_world.go` nie ma scentralizowanego loggingu „co zostało zmienione w tej operacji”. Pojedyncze warnings są emitowane przez `fmt.Printf` (niektóre endpointy) lub `runtime.LogWarningf` (write hooks). UI w `WorldTab` cache'uje stan lokalnie po mutacji (`setGraces(prev => ...)` itp.) — co tworzy ryzyko cache stale jeśli backend mutacja częściowo się powiodła.

## 18. Validation and safety notes

### 18.1 Stale generated data

Snapshoty per‑category w `backend/db/data/` (graces.go, summoning_pools.go, maps.go, regions.go itd.) są **statyczne**. Po patchu DLC:

- nowe wpisy nie pojawią się w UI,
- bulk operations nie obejmą nowych ID,
- `IsKnownXxxID(newID)` zwróci `false`.

Brak automatycznej detekcji „regulation.bin newer than snapshot”. `needs verification` po każdym dużym patchu.

### 18.2 Wrong flag IDs

Per‑category endpointy nie waliduja `id` przeciw odpowiedniej `data.Xxx` mapie — `SetGraceVisited`, `SetBossDefeated` itp. akceptują dowolne ID i delegują do `db.SetEventFlag`. Typo w argumencie albo direct JS call z dowolnym ID może SET'ować flagę zupełnie niezwiązaną. Patrz [15](15-event-flags.md) §3.

### 18.3 Read‑only vs write confusion

`WorldGeomBlock` / `RideGameData` / weather / time są verbatim — żaden endpoint w `app_world.go` ich nie modyfikuje. Użytkownik może oczekiwać że jakiś endpoint to robi (np. „reset world state” → reset all NPC) — nie istnieje.

Wyjątek: `BloodStain` ma 1 partial mutator dla L2 DLC tiles (§17.3).

### 18.4 Non‑atomic multi‑module mutations

Patrz §17.1. Orchestratory są fail‑fast bez auto‑restore. Partial mutation jest możliwy.

### 18.5 Rollback / manual undo limits

Wszystkie write endpointy używają `a.pushUndo(slotIndex)`. Undo stack ma limit głębokości (`needs verification`: dokładna wartość w `app.go`). Bulk operations + Promise.all w UI mogą stworzyć N osobnych snapshotów rosnących stack liniowo.

### 18.6 Platform / version differences

ID flag/regionów/gracji/pooli są snapshotami `regulation.bin`. PS4 vs PC offsets save file różnią się — patrz [01-header.md](01-header.md) i [49-ps4-zstd-rawblock-patch.md](49-ps4-zstd-rawblock-patch.md). Bitfield resolver (BST) jest cross‑platform.

`needs verification`: czy wszystkie write endpointy z §6.3 dają identyczny wynik na PC i PS4 — aktualny `tests/roundtrip_test.go` pokrywa I/O round‑trip, ale per‑endpoint platform parity nie jest izolowanym testem.

### 18.7 In‑game verification gaps

Większość helperów ma ad‑hoc runtime verification (CHANGELOG entries), brak automatycznego in‑game test loop. Każdy patch gry wymaga manualnego re‑testu.

### 18.8 Quest / NPC progression side effects

Niektóre flagi (np. `4680`/`4681` Melina state, `60100` Whistle obtained, `710520` Whistle world state) są progression flagami quest‑NPC. Ustawienie ich z `SetGraceVisited(76111)` (Gatefront companion fan‑out) lub `ApplyPvPPreparation` (Colosseums 60100) **może** wpływać na inne questy / cutsceny. SaveForge **nie ostrzega** pre‑mutation.

## 19. Test coverage

| Subsystem | Test plik | Pokrycie |
|---|---|---|
| Binary parsers | `backend/core/section_world_geom_test.go`, `section_world_test.go`, `section_trailing_test.go` | parse + round‑trip |
| Event flag resolver | `tests/map_flags_test.go`, `backend/db/event_flag*_test.go` | BST + precomputed |
| Regions | `backend/core/writer_regions_test.go` (4 testy) | RebuildSlot + identity |
| Companion flags item | `tests/item_companion_flags_test.go` (17), `backend/db/data/item_companion_flags_test.go` (11) | hook SET/CLEAR + cross‑contamination |
| Companion flags grace | `tests/grace_companion_flags_test.go` (3), `backend/db/data/grace_companion_flags_test.go` (3) | SET‑only contract |
| PvP preset | `pvp_test.go` (10) | validation + warnings + mutation |

`needs verification`: brak izolowanego testu cross‑subsystem orchestration (np. `RevealAllMap` + `SetGraceVisited` + `ApplyPvPPreparation` w sekwencji). Brak in‑game runtime verification w CI.

## 20. Known limits / needs verification

Skondensowana lista cross‑cutting otwartych pytań:

1. **Snapshot staleness** — brak automatycznej regeneracji `data.Xxx` z `regulation.bin`.
2. **Read‑only verbatim blobs** — brak walidacji magic bytes / structure integrity przy load.
3. **WorldGeomBlock direct edit** — nieosiągalne; physical colosseum gates etc. poza zasięgiem.
4. **Cross‑platform parity** — brak izolowanych testów per endpoint.
5. **Patch detection** — brak mechanizmu „aktualne stałe są stale”.
6. **In‑game verification CI** — brak; każdy patch wymaga manualnego re‑testu.
7. **Quest progression side effects pre‑warning** — brak; mutation może zakłócić questy.
8. **Orchestrator fail‑fast auto‑restore** — brak; partial mutation możliwy.
9. **Undo stack depth** — `needs verification` dokładnej wartości limitu.
10. **Cache stale ryzyko w UI** — `setXxx(prev => ...)` po backend mutacji może rozjechać UI z rzeczywistością.
11. **`WorldGeomMan` vs `WorldGeomMan2` semantyka** — „before/after” czy „two layers”? niezweryfikowane.
12. **RideGameData edit** — niemożliwy; Torrent state cofa się przy odpoczynku.

## 21. Cross‑references

- [11-regions.md](11-regions.md) — L0 UnlockedRegions; jedyny non‑bitfield write helper.
- [14-game-state.md](14-game-state.md) — Game State (LastRestedGrace, TotalDeathsCount); osobna domena.
- [15-event-flags.md](15-event-flags.md) — generic event flag helper API; fundament write helpers.
- [17-player-coordinates.md](17-player-coordinates.md) — osobna sekcja player coords.
- [19-weather-time.md](19-weather-time.md) — WorldAreaWeather + WorldAreaTime (verbatim blobs).
- [27-map-reveal.md](27-map-reveal.md) — 4‑layer Map Reveal.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — L2 DLC Cover Layer (BloodStain partial mutator).
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — Sites of Grace.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — `ApplyPvPPreparation` orchestrator.
- [50-item-companion-flags.md](50-item-companion-flags.md) — item lifecycle ↔ event flag bridge.

## 22. Sources

- `backend/core/section_world_geom.go` — `WorldGeomBlock` (5 blobów), `SizePrefixedBlob`, sanity ceilings.
- `backend/core/section_world.go` — `RideGameData` (40 B verbatim).
- `backend/core/section_trailing.go` — `WorldAreaWeather` + `WorldAreaTime` (12+12 B verbatim), `TrailingFixedBlock`.
- `backend/core/offset_defs.go::DynBloodStain` (`= 0x4C`), `DLCTile*` (partial mutator dla L2).
- `app_world.go` — ~34 App methods (Get/Set per ~12 kategorii, RevealAllMap, ResetMapExploration, RemoveFogOfWar).
- `app_pvp.go` — `ApplyPvPPreparation` orchestrator.
- `frontend/src/components/WorldTab.tsx` + `PvPPreparationTab.tsx` — UI per‑subsystem.
- Tests: `pvp_test.go`, `tests/map_flags_test.go`, `tests/grace_companion_flags_test.go`, `tests/item_companion_flags_test.go`, `backend/core/writer_regions_test.go`, `backend/core/section_world_geom_test.go`.
