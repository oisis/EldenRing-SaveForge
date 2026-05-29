# Elden Ring Save File Format — Podręcznik dla moderów

> **Cel**: Kompletna dokumentacja formatu binarnego pliku zapisu Elden Ring (`.sl2` / `memory.dat`) oraz systemów edytora SaveForge. Wystarczająca do implementacji niezależnego edytora save od zera.
>
> **Stan**: 🚧 **Work in progress — book cleanup**.
> Phase 1 (reorganizacja katalogowa) ✅ ukończona. Phase 2 (GaItem + Inventory + Storage + Transfer + Sort Order + Categories + Equipment) ✅ ukończona dla głównych rozdziałów (03, 06, 07, 10, 35, 36, 39, 43, 52, 53). Phase 3 (Ash of War + Build Template) ✅ ukończona dla głównych rozdziałów (54, 55). Phase 4 (Map / World / Event Flags / Game State) ✅ ukończona dla głównych rozdziałów (11, 14, 15, 16, 17, 19, 27, 29, 47, 48, 50). Następna: Phase 5 — Ban-risk / unsafe edits / validation / safety consolidation. Dalsze fazy (5–6) — patrz `BOOK_PLAN.md`.
>
> **Plan dalszych prac**: zobacz [`BOOK_PLAN.md`](BOOK_PLAN.md). Wynik audytu źródłowego: [`tmp/docs-book-audit.md`](../../tmp/docs-book-audit.md) (lokalny, gitignored).

---

## Jak czytać tę dokumentację

Wszystkie dokumenty żyją bezpośrednio w `spec/lang-pl/`. Większość to **kanoniczne rozdziały podręcznika** — zweryfikowana wiedza o formacie binarnym i zaimplementowanych systemach edytora. Kilka dokumentów ma status `research` lub `planned` — wyraźnie oznaczone w spisie treści — i pozostają w głównym katalogu jako pomocniczy materiał referencyjny.

**Legenda statusów** używana w spisie treści poniżej:

| Status | Znaczenie |
|---|---|
| `canonical` | Aktualny, zgodny z kodem, kandydat na finalny rozdział książki |
| `implemented, needs rewrite` | Kod istnieje i działa, ale dokument wymaga przepisania w canonical template |
| `partial` | Częściowo zweryfikowane / częściowo zaimplementowane — wymaga uzupełnień |
| `needs verification` | Konflikt doc vs code — wymaga manualnej weryfikacji per sekcja |
| `research` | Negatywny wynik lub wstrzymane badanie |
| `planned` | Design bez implementacji w kodzie |

---

## Platformy

| Platforma | Plik | Szyfrowanie | Checksumy |
|---|---|---|---|
| PC (Steam) | `ER0000.sl2` | AES-128-CBC (opcjonalne) | MD5 per slot |
| PS4 | `memory.dat` | Brak | Brak |
| PS5 | `memory.dat` | Brak | Brak |

---

## Struktura pliku — przegląd

Plik save składa się z następujących głównych bloków (w kolejności sekwencyjnej):

```
┌─────────────────────────────────────────────┐
│  HEADER (platforma-specyficzny)             │  → 01-header.md
├─────────────────────────────────────────────┤
│  SLOT 0: Character Save Data                │
│    ├── Slot Header                          │
│    ├── GaItem Map (mapa przedmiotów)        │  → 03-gaitem-map.md
│    ├── PlayerGameData (statystyki)          │  → 04-player-game-data.md
│    ├── SP Effects (efekty statusu)          │  → 05-sp-effects.md
│    ├── Equipment (ekwipunek)                │  → 06-equipment.md
│    ├── Inventory (inwentarz)                │  → 07-inventory.md
│    ├── Spells & Gestures (zaklęcia/gesty)   │  → 08-spells-gestures.md
│    ├── Face Data (kreator postaci)          │  → 09-face-data.md
│    ├── Storage Box (skrzynia)               │  → 10-storage.md
│    ├── Regions (odblokowane regiony)        │  → 11-regions.md
│    ├── Torrent (koń)                        │  → 12-torrent.md
│    ├── Blood Stain (plama krwi)             │  → 13-blood-stain.md
│    ├── Game State (stan gry)                │  → 14-game-state.md
│    ├── Event Flags (flagi zdarzeń)          │  → 15-event-flags.md
│    ├── World State (stan świata)            │  → 16-world-state.md
│    ├── Player Coordinates (pozycja)         │  → 17-player-coordinates.md
│    ├── Network Manager                      │  → 18-network.md
│    ├── Weather & Time (pogoda/czas)         │  → 19-weather-time.md
│    ├── Version & Platform Data              │  → 20-version-platform.md
│    ├── DLC                                  │  → 21-dlc.md
│    └── Player Data Hash                     │  → 22-player-hash.md
│  SLOT 1-9: (identyczna struktura)           │
├─────────────────────────────────────────────┤
│  USER_DATA_10 (profil konta)                │  → 23-user-data-10.md
├─────────────────────────────────────────────┤
│  USER_DATA_11 (regulation.bin)              │  → 24-user-data-11.md
└─────────────────────────────────────────────┘
```

---

## Phase 2 — cross-cutting gaps

Po ukończeniu Phase 2 (canonical rewrites: 03, 06, 07, 10, 35, 36, 39, 43, 52, 53) pozostają następujące `needs verification` rozproszone po rozdziałach. Są to **nie**-blockery dla Phase 3+, ale powinny być adresowane w przyszłych iteracjach:

- **Storage Apply in-game / Steam Deck verification** — wspólny gap dla [52](52-acquisition-sort-stride2.md) i [53](53-inventory-storage-transfer.md). Brak świeżego raportu z fixtura PS4 po reorder/transfer.
- **Workspace path equipped guard** — `editor.ApplyWorkspaceSave` nie ma explicit `IsHandleEquipped` check (zob. [53](53-inventory-storage-transfer.md) "Equipped guard / Workspace path", [06](06-equipment.md) "Workspace path gap").
- **Workspace post-mutation validation** — w odróżnieniu od [43](43-transactional-item-adding.md), workspace save nie ma `ValidatePostMutation` (zob. [53](53-inventory-storage-transfer.md) "Validation and rollback caveats").
- **UI counters vs allocator capacity** — `SortOrderTab` counter per zakładka jest `view.length`, nie total kontenera; allocator capacity ([35](35-gaitem-allocator-invariants.md)) operuje na innym poziomie. Brak end-to-end testu cross-check.
- **DLC sub-mapping completeness** — czy każdy DLC item w DB ma assigned sub-group (zob. [36](36-inventory-categories-game-order.md) "DLC flag mechanism"). Best-effort `melee_subcat.go` curated lookup.
- **Equipment write API not implemented** — edytor jest read-only dla ChrAsmEquipment (zob. [06](06-equipment.md) "What SaveForge does not implement"). `EquippedGreatRune` round-tripuje, ale brak public setter z UI.
- **Hash recompute discipline** — `RecalculateSlotHash` wywoływany **tylko w testach** (zob. [06](06-equipment.md) "Hash recompute discipline"). `needs verification` całościowe.
- **Game order in-game verification dla bieżącej wersji gry** — ostatnia weryfikacja kwiecień 2026 (zob. [36](36-inventory-categories-game-order.md) "Status"). Patche FromSoftware mogą reorganizować menu.

---

## Phase 3 — cross-cutting gaps

Po ukończeniu Phase 3 (canonical rewrites: 54, 55) pozostają następujące `needs verification` rozproszone po rozdziałach. Są to **nie**-blockery dla Phase 4+, ale powinny być adresowane w przyszłych iteracjach:

- **AoW affinity gating** — `EquipParamWeapon.defaultWepAttr` / `configurableWepAttr00..23` nie są zaimportowane do `WeaponGemMounts`; preview Build Template waliduje compat tylko po `wepType`. `needs verification` w [54 §22.L1](54-ash-of-war.md) i [55 §21.L1](55-build-template.md).
- **DLC wepType 69/94/95 user-facing behavior** — backend allow-passthrough, UI traktuje jako `unknown` i fail-closes widoczność sekcji AoW; brak informacji dla użytkownika, że to DLC z nieznaną kompatybilnością. `needs verification` w [54 §22.L2](54-ash-of-war.md).
- **Frontend/backend `WEP_TYPE_TO_BIT` drift** — pojedynczy frontend mirror (`WeaponEditModal.tsx`), ręcznie podtrzymywany; identyczny z backendem ale brak guardu CI / generatora. `needs verification` przy każdej zmianie backendu. Zob. [54 §17 / §22.L4](54-ash-of-war.md).
- **`gemMountType == 1` (somber) edge cases w UI** — `CanMountAoW = false` wyłącza sekcję AoW, ale dokumentacja nie potwierdza placeholdera/wyjaśnienia. `needs verification` w [54 §22.L3](54-ash-of-war.md).
- **`AoWCompatMasks` completeness po update'cie regulation** — bitmask generowany z `EquipParamGem`; nowe DLC rows mogą nie być re-imported. `needs verification` w [54 §22.L5](54-ash-of-war.md).
- **Orphan AoW GaItem GC / save bloat** — alokator nie zwalnia handle po reset AoW; save rośnie liniowo z liczbą AoW edits. `needs verification` w [54 §22.L6](54-ash-of-war.md).
- **Build Template equipment write API** — ❌ nie zaimplementowane; apply zostawia bronie unequipped. `needs verification` dla future Phase w [55 §12 / §21.L3](55-build-template.md).
- **Build Template spell loadout / character stats / affinity / DLC presence cross-check** — schema v1 nie eksportuje attunement slotów ani statystyk; DLC presence przy apply flaguje pojedyncze itemy bez globalnego "needs DLC X" warning. `needs verification` w [55 §6 / §21.L6](55-build-template.md).
- **Build Template `replace-*` modes** — `replace-weapons`, `replace-armors` itd. są zarezerwowane w schemacie ale nie zaimplementowane; v1 obsługuje tylko `merge`. Zob. [55 §6](55-build-template.md).
- **Build Template forward-compat `version=2` testy** — `SchemaVersion=1` jedyny akceptowany; brak testów scenariuszy unknown-future-fields. `needs verification` w [55 §18 / §21.L8](55-build-template.md).
- **Cross-platform PS4 vs PC portability dla Build Template** — schema jest portable z założenia (no save-local handles), ale brak E2E testu eksportu PS4 → import PC i odwrotnie. `needs verification`.

---

## Phase 4 — cross-cutting gaps

Po ukończeniu Phase 4 (canonical rewrites i refresh: 11, 14, 15, 16, 17, 19, 27, 29, 47, 48, 50) pozostają następujące `needs verification` rozproszone po rozdziałach. Są to **nie**-blockery dla Phase 5+, ale powinny być adresowane w przyszłych iteracjach:

- **Stale generated snapshots po patchu gry / regulation.bin** — `data.Graces` (419), `data.Regions` (104), `data.MapVisible` (263), `data.SummoningPools` (~213), `data.ColosseumFlagSets` (3), `itemCompanionEventFlags` (6) są statycznymi snapshotami; brak auto-detection „regulation.bin newer than snapshot" (zob. [11 §16](11-regions.md), [15](15-event-flags.md), [27 §13](27-map-reveal.md), [47 §16.1](47-site-of-grace-activation.md), [50 §12.1](50-item-companion-flags.md)).
- **PS4 ↔ PC parity tests** — `tests/roundtrip_test.go` pokrywa I/O round-trip, ale brak per-endpoint platform parity (np. `SetGraceVisited` PC vs PS4 effect identyczny). `needs verification` w [11](11-regions.md), [16 §18.6](16-world-state.md), [14 §16.5](14-game-state.md).
- **Brak cross-subsystem atomic transaction** — orchestratory (`RevealAllMap`, `ApplyPvPPreparation`, `SaveCharacter`) używają single `pushUndo`, ale per-fazowy / per-modular rollback nie istnieje (zob. [16 §17](16-world-state.md), [27 §12](27-map-reveal.md), [48 §14](48-pvp-ready-modular-presets.md), [14 §15](14-game-state.md)).
- **Manual undo / rollback limits** — undo stack depth limit jest nieznany (`needs verification`); bulk operations (UI Promise.all) tworzą N osobnych snapshotów; po `WriteSave` jedyna ścieżka rollback to backup `.sl2.YYYYMMDD_HHMMSS.bkp` (zob. [47 §15.2](47-site-of-grace-activation.md), [50 §11](50-item-companion-flags.md), [16 §18.5](16-world-state.md)).
- **In-game runtime verification gaps** — większość helperów ma ad-hoc CHANGELOG entries, brak automated in-game loop w CI dla map reveal / DLC tiles / Sites of Grace / PvP matchmaking / NG+ flag sync (zob. [27 §13](27-map-reveal.md), [29 §13.6](29-dlc-black-tiles.md), [47 §16](47-site-of-grace-activation.md), [48 §16.7](48-pvp-ready-modular-presets.md), [14 §16.6](14-game-state.md)).
- **Event flag ID correctness / stale precomputed caches** — 3-tier resolver (precomputed → BST → fallback formula) jest snapshotem; nowe flagi z patchy mogą fallback'ować w nieprzewidziane miejsca (zob. [15](15-event-flags.md)).
- **Map reveal visual vs gameplay/progression effects** — UI nota `WorldTab.tsx:406` mówi „Some graces may still play their in-world activation sequence"; brak izolowanego testu czy zdjęcie L2 wpływa na trofea / achievementy (zob. [27 §13](27-map-reveal.md), [29 §13.5](29-dlc-black-tiles.md), [47 §7](47-site-of-grace-activation.md)).
- **DLC black tile coordinates stale-after-patch risk** — wartości syntetyczne `9648/9124` i `3037/1869/7880/7803` są empiryczne (snapshot z 2 slotów); nie game-guaranteed (zob. [29 §13.1](29-dlc-black-tiles.md)).
- **Sites of Grace SET-only intent + PvP module E placeholder** — companion flag SET-only contract w grace lifecycle jest celowy; PvP module E (`opts.SitesOfGrace`) jest placeholder bez mutation (zob. [47 §9](47-site-of-grace-activation.md), [48 §7](48-pvp-ready-modular-presets.md)).
- **Item companion flag IDs stale-after-patch** — 6 wpisów `itemCompanionEventFlags` z hardcoded literałami; brak mechanizmu detection (zob. [50 §12.1](50-item-companion-flags.md)).
- **PvP "ready" scope ograniczony** — Matchmaking Regions ✅; Colosseums z fizycznymi bramami w `WorldGeomMan` blob (nieedytowalne); Summoning Pools impact „Bloody Finger invasion impact is unconfirmed"; Sites of Grace module **disabled** (zob. [48 §16.1](48-pvp-ready-modular-presets.md)).
- **Player Coordinates / Weather-Time read-only** — brak publicznych setterów; `grep "Set..." → 0`; każda mutacja przez direct hex edit jest na odpowiedzialności użytkownika (zob. [17 §6](17-player-coordinates.md), [19 §6](19-weather-time.md)).
- **Game State: LastRestedGrace read-only, ClearCount jako jedyny write path** — `LastRestedGrace` BonfireId jest read-only (gra zarządza runtime); `ClearCount` ma write przez `SaveCharacter` + event flag sync 50-57, ale brak progression consistency validator (zob. [14 §8.3](14-game-state.md), [14 §9.2](14-game-state.md)).
- **Boss multi-flag editor pozostaje planned** — aktualny `SetBossDefeated` jest single-flag; multi-flag design w [38-boss-multiflag.md](38-boss-multiflag.md) (zob. [14 §12](14-game-state.md)).

---

## Spis treści książki

### Part I — Save File Format Fundamentals

Format binarny pliku save — kontener, sloty, layout sekcji.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 01 | [Header i layout pliku](01-header.md) | `canonical` | Magic bytes, detekcja platformy, BND4, MD5 |
| 02 | [Slot — struktura ogólna](02-slot-structure.md) | `canonical` | Rozmiar slotu, sekwencyjny parsing |
| 03 | [GaItem Map](03-gaitem-map.md) | `canonical` | GaItem layout + GaMap; AoW semantyka w 54 (cross-ref, no duplication) — Phase 2 Step 2 |
| 04 | [PlayerGameData](04-player-game-data.md) | `canonical` | 432 B, atrybuty, runy, online settings |
| 05 | [SP Effects](05-sp-effects.md) | `needs verification` | Sekcja krótka, „wymaga weryfikacji" w treści; brak parsera w `backend/core/` |
| 06 | [Equipment](06-equipment.md) | `canonical` (read-only) | `EquippedItemsItemIds` (88B) + `EquippedGreatRune`; **brak publicznego write API** dla equipment — Phase 2 Step 8 |
| 07 | [Inventory](07-inventory.md) | `canonical` | Read-side rekord 12B + offsety CommonItems — Phase 2 Step 3 |
| 08 | [Spells & Gestures](08-spells-gestures.md) | `canonical` | 14 attunement + 8B gesture stride |
| 09 | [Face Data](09-face-data.md) | `partial` | 303 B, pola 0x20-0x5F „przybliżone" — kod (`app_appearance.go`) zna więcej |
| 10 | [Storage Box](10-storage.md) | `canonical` | Read-side: `StorageBoxOffset` + `StorageHeaderSkip`, sparse list — Phase 2 Step 3 |
| 11 | [Regions](11-regions.md) | `canonical` | `core.SetUnlockedRegions`, L0 Map Reveal detail (cross-link do 27) — Phase 4 Step 3 |
| 12 | [Torrent](12-torrent.md) | `canonical` | State enum 1/3/13; bug HP=0+State=13 |
| 13 | [Blood Stain](13-blood-stain.md) | `partial` | unk_0x1c..0x40 — w spec/29 te offsety to DLC Cover Layer (konflikt do rozwiązania) |
| 14 | [Game State](14-game-state.md) | `canonical` | `PreEventFlagsScalars` (29 B), ClearCount/NG+ write path, LastRestedGrace read-only — Phase 4 Step 10 |
| 15 | [Event Flags](15-event-flags.md) | `canonical` | 3-tier resolver (precomputed → BST → fallback), helper API — Phase 4 Step 1 |
| 16 | [World State](16-world-state.md) | `canonical` (overview/index) | Subsystem map, read-only vs write-capable; linkuje 11/15/27/29/47/48/50/14 — Phase 4 Step 8 |
| 17 | [Player Coordinates](17-player-coordinates.md) | `canonical` (read-only) | `PlayerCoordinates` (**61 B**, NIE 57 B), `SpawnPointBlock` version-gated, brak setterów — Phase 4 Step 9 |
| 18 | [Network Manager](18-network.md) | `partial` (merge candidate) | 131 KB opaque — slot-local, nie regulation NetworkParam |
| 19 | [Weather & Time](19-weather-time.md) | `canonical` (read-only) | `WorldAreaWeather` + `WorldAreaTime` (12+12 B), brak setterów — Phase 4 Step 9 |
| 20 | [Version & Platform](20-version-platform.md) | `canonical` | Steam ID + BaseVersion + PS5Activity |
| 21 | [DLC](21-dlc.md) | `canonical` | 50 B, invariant bytes 3-49 = 0 |
| 22 | [Player Data Hash](22-player-hash.md) | `canonical` | „Gra ignoruje hash" — `backend/core/hash.go` potwierdza |
| 23 | [UserData10](23-user-data-10.md) | `canonical` | PC + PS4 offsety zweryfikowane Apr 2026 |
| 24 | [UserData11](24-user-data-11.md) | `canonical` (read-only) | TWARDA REGUŁA — nie modyfikujemy |
| 25 | [Runtime vs Save](25-runtime-vs-save.md) | `canonical` | CT memory vs save offsety — wartość edukacyjna |
| 26 | [Parameter Reference](26-parameter-reference.md) | `partial` (needs rewrite) | Tytuł obiecuje więcej niż dostarcza — atrybuty + softcaps |

### Part II — Implemented SaveForge Systems

Zaimplementowane mechanizmy edytora — działają w aktualnym kodzie.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 32 | [Ban-Risk System (UI)](32-ban-risk-system.md) | `canonical` | SafetyMode, RISK_INFO, komponenty `Risk*` |
| 34 | [Item Caps Enforcement](34-item-caps.md) | `canonical` | `scales_with_ng` + NG+ scaling — TODO o ClearCount otwarte |
| 35 | [GaItem Allocator & Invariants](35-gaitem-allocator-invariants.md) | `canonical` | Alokacja handle, capacity invariants, `generateUniqueHandle`/`allocateGaItem` — Phase 2 Step 1 |
| 36 | [Inventory Categories — Game Order](36-inventory-categories-game-order.md) | `canonical` | 18 DB tabs + handle prefix bridge + sub-categories (76) + DLC flag mechanism — Phase 2 Step 7 (nadpisuje 33) |
| 39 | [Inventory Reorder](39-inventory-reorder.md) | `historical / superseded` | Design note z fazy projektowej; **superseded by 52** dla acquisition/stride mechanics, **covered by 53** dla transfer UX — Phase 2 Step 5 |
| 43 | [Transactional Item Adding](43-transactional-item-adding.md) | `canonical` | Pre-flight + `SnapshotSlot`/`RestoreSlot` + `ValidatePostMutation` — Phase 2 Step 4 |
| 44 | [NetworkParam Tuning](44-network-param-tuning.md) | `canonical` | `regulation.go::PatchNetworkParams` |
| 50 | [Item Companion Flags](50-item-companion-flags.md) | `canonical` | SET+CLEAR symmetric mechanism, 6 wpisów (Whistle + 5 multiplayer items); osobny od grace SET-only ([47](47-site-of-grace-activation.md)) i `MapFragmentItems` ([27](27-map-reveal.md)) — Phase 4 Step 6 |
| 52 | [Acquisition Sort — Stride-2](52-acquisition-sort-stride2.md) | `canonical` | Stride-2 algorytm, bucket-collision guard, 3 ścieżki write (`ReorderInventory`/`ReorderStorage`/`writeContainerLayout`) — Phase 2 Step 5 |
| 53 | [Inventory ↔ Storage Transfer](53-inventory-storage-transfer.md) | `canonical` | Dwie ścieżki transferu (legacy core + workspace), rehandle, equipped guard, SortOrderTab workspace UI — Phase 2 Step 6 |
| 54 | [Ash of War](54-ash-of-war.md) | `canonical` | Sentinele 0x00/0xFFFFFFFF + invariant unikalności + AoW guard (commit `6881cb9`), workspace/strict write paths, fail-closed compat — Phase 3 Step 1 |
| 55 | [Build Template](55-build-template.md) | `canonical` | JSON v1, portable export bez save-local handles, capacity preflight, RAM-only apply z rollback, Phase E local library — Phase 3 Step 2 |

### Part III — Verified Game Mechanics

Mechaniki gry zweryfikowane przez RE / testy in-game.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 27 | [Map Reveal](27-map-reveal.md) | `canonical` | 4-warstwowy model (L0–L3); `RevealAllMap` / `revealBaseMap` / `revealDLCMap` / `RemoveFogOfWar` / `ResetMapExploration` — Phase 4 Step 2 |
| 29 | [DLC Black Tiles](29-dlc-black-tiles.md) | `canonical` (detail) | L2 DLC Cover Layer w `BloodStain`; `DLCTile*` constants + syntetyczne koordy + `revealDLCMap` Phase 3 — Phase 4 Step 4 |
| 31 | [Appearance Presets](31-appearance-presets.md) | `canonical` | Apply algorithm + Mirror Favorites; PC verified |
| 47 | [Sites of Grace — Activation](47-site-of-grace-activation.md) | `canonical` | Grace EventFlag 71xxx-76xxx + DoorFlag + companion flags SET-only (Gatefront); 419 entries — Phase 4 Step 5 |
| 48 | [PvP Modular Presets](48-pvp-ready-modular-presets.md) | `current reference` | `ApplyPvPPreparation` z 5 modułami (4 active + 1 placeholder); Sites of Grace module E zwraca warning bez mutacji (`app_pvp.go:109`) — Phase 4 Step 7 |
| 49 | [PS4 ZSTD Raw-Block Patch](49-ps4-zstd-rawblock-patch.md) | `canonical` | Krytyczna wiedza PS4 — `regulation.go:604` |

### Part IV — Research Archive / Negative Results

Historia badań, wstrzymane investigacje, negatywne wyniki.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 30 | [Slot Rebuild — Research](30-slot-rebuild-research.md) | `research` | Dziennik pomiarów slack 11 slotów; finalna implementacja: `RebuildSlot` |
| 42 | [Summoning Pools Bug](42-summoning-pools-bug.md) | `research` | 🐛 Wstrzymane — UI działa, brak efektu in-game |

### Planowane

Design doci bez implementacji w kodzie.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 37 | [Character Presets (JSON)](37-character-presets.md) | `needs verification` ⚠️ | `backend/vm/preset.go` ma `CharacterPreset/VMToPreset/PresetToVM/ValidatePreset`, ale doc deklaruje „Planowany". Wymaga weryfikacji per faza. |
| 38 | [Boss Multi-Flag](38-boss-multiflag.md) | `planned` | Kod ma 1-flag model; design wymaga `EventFlags []uint32` (nie wdrożone) |
| 56 | [Templates v2 — Planned Extension](56-templates-v2.md) | `planned` | Addytywne rozszerzenie [55](55-build-template.md): publiczny format YAML, sidebar entry, sekcje profile/stats/equipment/talismans/spells, granular selection, import z pliku/URL, weapon level override, single-character first, multi-character pack później. Brak zmiany kodu na razie. |

### Appendix (planowany)

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 45 | [Dokumentacja Ryzyka Bana](45-ban-risk-reference.md) | `canonical` (App. A) | Community triggers — knowledge base, podstawa dla tiers w 32 |
| 99 | [Verification Methodology](99-verification-methodology.md) | `canonical` (App. B) | Metodyka research |

---

## Kluczowe właściwości formatu

- **Endianness**: Little-endian (wszystkie wartości liczbowe)
- **Stringi**: UTF-16LE z null-terminatorem
- **Rozmiar slotu**: 0x280000 (2,621,440 bajtów) — stały
- **Sekcje zmiennej długości**: inventory projectiles, regions, world areas — wymagają sekwencyjnego parsingu
- **Checksumy**: MD5 (tylko PC), przeliczane przy zapisie
- **Szyfrowanie**: AES-128-CBC (tylko PC, opcjonalne — nowsze wersje gry)

---

## Źródła wiedzy

### Projekty referencyjne (lokalne kopie w `tmp/repos/`)

| Projekt | Język | Priorytet | Opis |
|---|---|---|---|
| [er-save-manager](https://github.com/Jeius/er-save-manager) | Python | **1 (najwyższy)** | Najnowszy, pełny sekwencyjny parser z DLC support |
| [ER-Save-Editor](https://github.com/ClayAmore/ER-Save-Editor) | Rust | **2** | Dobrze typowany parser, potwierdza rozmiary struktur |
| [Elden-Ring-Save-Editor](https://github.com/shalzuth/Elden-Ring-Save-Editor) | Python | **3 (najniższy)** | Stary, pattern-matching approach, ale pierwsze odkrycia offsetów |

### Dokumentacja online

| Źródło | URL | Zawartość |
|---|---|---|
| Souls Modding Wiki — SL2 Format | https://www.soulsmodding.com/doku.php?id=format:sl2 | Format kontenera save |
| Souls Modding Wiki — Event Flags | https://www.soulsmodding.com/doku.php?id=er-refmat:event-flag-list | Lista flag zdarzeń |
| Event Flags GitHub Pages | https://soulsmods.github.io/elden-ring-eventparam/ | Pełna lista 1000+ flag z opisami |
| Event Flags Spreadsheet | https://docs.google.com/spreadsheets/d/1Nn-d4_mzEtGUSQXscCkQ41AhtqO_wF2Aw3yoTBdW9lk | Szczegółowy arkusz flag |
| Steam Guide — Save Structure | https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037 | Offsety slotów, MD5 checksumy |
| SoulsFormats (C#) | https://github.com/JKAnderson/SoulsFormats | BND4 format parsing library |
| TGA Cheat Engine Table | https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA | Event flags, param scripts, item IDs |
| Souls Modding Wiki — Params | https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam | SpEffect i inne param tables |

### Pliki danych (lokalne)

| Plik | Ścieżka | Opis |
|---|---|---|
| eventflag_bst.txt | `tmp/repos/er-save-manager/src/resources/eventflag_bst.txt` | 11919 wpisów — mapowanie block→offset dla event flags |
| PC Save | `tmp/save/ER0000.sl2` | Prawdziwy save PC (5 slotów) |
| PS4 Save | `tmp/save/oisisk_ps4.txt` | Prawdziwy save PS4 |

---

## Konwencje w dokumentacji

- **Offsety** zapisujemy jako hex: `0x1B0`
- **Rozmiary** w bajtach hex i dziesiętnie: `0x12F (303 bytes)`
- **Typy danych**: u8, u16, u32, i32, f32, u64 (little-endian)
- **Stringi**: UTF-16LE, podajemy max liczbę znaków (nie bajtów)
- **Sekcje zmiennej długości**: oznaczamy jako `[VARIABLE]`
- **Nieznane pola**: oznaczamy jako `unk_0xNN` z notatką co wiemy
- **Status weryfikacji**: ✅ zweryfikowano hex | ⚠️ cross-reference only | ❓ niepewne

---

## Tłumaczenia

Angielskie wersje wszystkich dokumentów specyfikacji znajdują się w [`spec/`](../) (katalog nadrzędny). **Uwaga**: reorganizacja i cleanup Phase 1–4 dotyczy tylko `spec/lang-pl/` — wersja angielska zostanie zsynchronizowana w późniejszej fazie.
