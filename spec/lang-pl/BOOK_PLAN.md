# BOOK_PLAN — `spec/lang-pl/` jako podręcznik dla moderów

> **Cel**: roadmapa konsolidacji `spec/lang-pl/` w spójną książkę. Źródłem prawdy są: kod w `backend/`, `app*.go`, `frontend/src/` oraz aktualny audyt `tmp/docs-book-audit.md` (lokalny, gitignored).
>
> **Aktualna faza**: Phase 4 ✅ ukończona dla głównych rozdziałów (Map / World / Event Flags / Game State — canonical rewrites 11, 14, 15, 16, 17, 19, 27, 29, 47, 48, 50). Phase 5 (Ban-risk + unsafe edits + validation/safety consolidation) — następna.
>
> **Format**: każdy planowany rozdział otrzyma docelowo template z sekcji [Template rozdziału](#template-rozdziału) (na końcu pliku).

---

## Docelowy spis treści

Po wszystkich fazach `spec/lang-pl/` ma się składać z 30 rozdziałów + 7 załączników, podzielonych na pięć części.

### PART I — Orientation

| Ch | Tytuł rozdziału | Źródłowe dokumenty |
|---|---|---|
| Ch.0 | Why this book exists (nowy) | — |
| Ch.1 | Container & Platforms | 01, 20, 49 |
| Ch.2 | Slot anatomy & sequential parsing | 02, 25 |

### PART II — Binary Format Reference

| Ch | Tytuł rozdziału | Źródłowe dokumenty |
|---|---|---|
| Ch.3 | GaItem Map | 03 (layout), 14 (GaItem Game Data); AoW semantyka → Ch.7 |
| Ch.4 | Inventory & Storage model | 07, 10 |
| Ch.5 | Player attributes (PlayerGameData) | 04 + atrybuty z 26 |
| Ch.6 | Equipment & Active weapon slots | 06, 08 |
| Ch.7 | Ash of War | 54 + AoW część z 03/06 |
| Ch.8 | Appearance & Face Data | 09 + 31 |
| Ch.9 | Event Flags | 15 |
| Ch.10 | World, Weather, Coordinates | 16, 17, 19 |
| Ch.11 | Map Reveal — 4 layers | 27 + 11 (regions) + część 29 |
| Ch.12 | Sites of Grace | 47 + 14 §LastRestedGrace |
| Ch.13 | Game State, NG+, tutorials | 14 |
| Ch.14 | Network Manager (slot-local) | 18 |
| Ch.15 | DLC flags & version | 20, 21 |
| Ch.16 | Player Data Hash & trailing | 22 |
| Ch.17 | UserData10 — account, presets, summaries | 23 |
| Ch.18 | UserData11 — regulation.bin (read-only) | 24 |

### PART III — Editor Internals (how SaveForge does it)

| Ch | Tytuł rozdziału | Źródłowe dokumenty |
|---|---|---|
| Ch.19 | Transactional item adding | 43 + 50 (companion flags) |
| Ch.20 | Acquisition sort (stride-2) | 52 |
| Ch.21 | Inventory ↔ Storage transfer | 53 |
| Ch.22 | Item caps & NG+ scaling | 34 |
| Ch.23 | Inventory categories (game-accurate) | 36 |
| Ch.24 | Slot rebuild & post_unlocked_regions blob | 30 (skondensowane) + nota inżynierska |
| Ch.25 | Build Templates (JSON export/import) | 55 |

### PART IV — Platform Tuning & PvP

| Ch | Tytuł rozdziału | Źródłowe dokumenty |
|---|---|---|
| Ch.26 | NetworkParam tuning | 44 |
| Ch.27 | PS4 ZSTD raw-block patch | 49 |
| Ch.28 | Modular PvP presets (current scope) | 48 (skrócone — bez planowanych modułów) |

### PART V — Runtime Observations

| Ch | Tytuł rozdziału | Źródłowe dokumenty |
|---|---|---|
| Ch.29 | Runtime vs save — using Cheat Engine alongside | 25 |
| Ch.30 | Boss defeat — current 1-flag model | 14 §boss + opis aktualnego `SetBossDefeated` |

### APPENDICES

| App | Tytuł | Źródłowe dokumenty |
|---|---|---|
| A | Ban-risk reference | 45 |
| B | Ban-risk UI tier system | 32 |
| C | Verification methodology | 99 |
| D | Parameter softcaps & quick reference | 26 (reszta po wyjęciu atrybutów do Ch.5) |
| E | Research log — negative results | 30 (pełny), 42 |
| F | Planned features | 37, 38, 56 |

**Konsolidacja**: 47 dokumentów → 30 rozdziałów + 6 załączników (1.6×).

---

## Fazy dalszych prac

### Phase 1 — Reorganizacja katalogowa + README ✅ UKOŃCZONA

- **Cel**: rozdzielić referencję od research-, planned- i archive-docs bez zmian merytorycznych.
- **Zakres (historyczny)**: utworzenie podkatalogów `research`, `planned`, `archive`; `git mv` 8 dokumentów; rewrite `README.md` + nowy `BOOK_PLAN.md`.
- **Akceptacja**: główny katalog `spec/lang-pl/` zawiera tylko dokumenty kandydujące do rozdziałów; brak dangling linków; vanilla MD5 nietknięty. ✅
- **Effort**: 2–3 h.
- **Commit**: `1c2fbab docs(lang-pl): reorganize book structure`.
- **Cleanup w Phase 4+** (2026-05): podkatalogi `research`, `planned`, `archive` zostały usunięte. Trzy aktywne dokumenty (38, 30, 42) wróciły do root `spec/lang-pl/`; pozostałe (33, 40, 41, 46, 51) skasowane jako obsolete.

### Phase 2 — Inventory + Storage + GaItem + Equipment ✅ UKOŃCZONA (główne rozdziały)

- **Cel**: skonsolidować 03, 06, 07, 10, 35, 36, 39, 43, 52, 53 w spójną narrację canonical.
- **Wykonane**:
  - **Step 1**: `35-gaitem-allocator-invariants.md` — nowy canonical chapter (alokacja handle, capacity invariants). Commit `9f25084 docs(lang-pl): document GaItem allocator invariants`.
  - **Step 2**: `03-gaitem-map.md` — rewrite jako canonical (GaItem layout + GaMap, AoW przeniesione cross-ref do 54). Commit `4f616c8 docs(lang-pl): rewrite GaItem map model`.
  - **Step 3**: `07-inventory.md` + `10-storage.md` — oba przepisane jako canonical (read-side rekord 12B). Commit `398d2a4 docs(lang-pl): rewrite inventory and storage models`.
  - **Step 4**: `43-transactional-item-adding.md` — canonical refresh (pre-flight + `SnapshotSlot`/`RestoreSlot` + `ValidatePostMutation`). Commit `7ff576f docs(lang-pl): rewrite transactional item adding`.
  - **Step 5**: `52-acquisition-sort-stride2.md` + `39-inventory-reorder.md` — 52 canonical (stride-2 + 3 ścieżki write); 39 jako historical/superseded design note. Commit `91291e7 docs(lang-pl): document acquisition stride sort order`.
  - **Step 6**: `53-inventory-storage-transfer.md` — canonical (dwie ścieżki: legacy core + workspace; rehandle; equipped guard; SortOrderTab workspace UI). Commit `e3ee3aa docs(lang-pl): rewrite inventory storage transfer`.
  - **Step 7**: `36-inventory-categories-game-order.md` — canonical (18 DB tabs + handle prefix bridge + 76 sub-categories + DLC flag mechanism). Commit `e1ce20d docs(lang-pl): rewrite inventory category mapping`.
  - **Step 8**: `06-equipment.md` — canonical, read-only model (`EquippedItemsItemIds` + `EquippedGreatRune`, brak publicznego write API, hipotetyczne 3 struktury z er-save-manager → Historical notes). Commit `84b7daf docs(lang-pl): rewrite equipment reference`.
- **Akceptacja**: wszystkie 10 rozdziałów ma canonical template, cross-refs między sobą, source-of-truth w kodzie z liniami, `needs verification` markers tam gdzie kod nie potwierdza w 100%.
- **Effort rzeczywisty**: ~10 commitów na branchu `docs/lang-pl-book-cleanup`.

#### Cross-cutting gaps z Phase 2 (do adresowania w przyszłości)

- Storage Apply in-game / Steam Deck verification (wspólny gap dla 52 i 53).
- Workspace path equipped guard (brak explicit `IsHandleEquipped` check w `editor.ApplyWorkspaceSave`).
- Workspace post-mutation validation (brak `ValidatePostMutation` analogicznego do 43).
- UI counters vs allocator capacity end-to-end cross-check.
- DLC sub-mapping completeness (best-effort `melee_subcat.go` curated lookup).
- Equipment write API — **nie zaimplementowane** (read-only). `EquippedGreatRune` round-tripuje, ale brak public setter.
- Hash recompute discipline — `RecalculateSlotHash` wywoływany tylko w testach.
- Game order in-game verification dla bieżącej wersji gry (ostatnia weryfikacja kwiecień 2026).

### Phase 3 — Ash of War + Build Template (Ch.7, Ch.25) ✅ UKOŃCZONA (główne rozdziały)

- **Cel**: zamknąć temat AoW (54 + AoW guard z commit `6881cb9` + invariant z 03/35 + UI z `WeaponEditModal.tsx`) oraz udokumentować Build Template (55).
- **Wykonane**:
  - **Step 1**: `54-ash-of-war.md` — canonical rewrite (sentinele 0x00/0xFFFFFFFF, strict vs allocate+rebuild write paths, AoW Allocation Safety guard z commit `6881cb9`, shared-handle invariant, ScanAoWAvailability 2-pass, workspace/WeaponEditModal state, fail-closed compat na unknown wepType, DLC wepType 69/94/95 allow-passthrough, frontend `WEP_TYPE_TO_BIT` mirror drift, 8 explicit needs-verification items). Commit `e3a634f docs(lang-pl): rewrite ash of war reference`.
  - **Step 2**: `55-build-template.md` — canonical rewrite (Build Template JSON v1 schema, portable payload rule bez save-local handles, AoW relation z fail-closed compat, capacity preflight `CommonItemCount=2688` / `StorageCommonCount=1920`, RAM-only apply z `deepCopySnapshot` rollback, Phase E local library `$UserConfigDir/EldenRing-SaveEditor/templates/` z atomic writes + `_index.json` z `LibraryIndexVersion=1`, 110 testów Go, 12 needs-verification items). Commit `a2e455c docs(lang-pl): rewrite build template reference`.
- **Akceptacja**: 54 zawiera tabelę „what happens when allocateGaItem returns error" (§15 AoW Allocation Safety) + cross-ref do 55 dla Build Template apply; 55 ma kompletny schemat JSON v1 + portable payload rule + przykład export/import/apply + cross-ref do 54 dla AoW relation. Oba rozdziały bez duplikacji semantyki AoW.
- **Effort rzeczywisty**: 2 commity na branchu `docs/lang-pl-book-cleanup`.

#### Cross-cutting gaps z Phase 3 (do adresowania w przyszłości)

- **AoW affinity gating** — `EquipParamWeapon.defaultWepAttr` / `configurableWepAttr00..23` nie są zaimportowane do `WeaponGemMounts`. Preview Build Template waliduje compat tylko po `wepType`, nie po infusion variant. (54 §22.L1, 55 §21.L1)
- **DLC wepType gaps (69/94/95)** — backend allow-passthrough; UI fail-closes widoczność sekcji AoW; brak user-facing informacji „DLC, kompatybilność nieznana". (54 §22.L2)
- **`gemMountType == 1` semantyka** — `CanMountAoW = false` wyłącza sekcję AoW, ale brak placeholdera/wyjaśnienia w UI. (54 §22.L3)
- **`AoWCompatMasks` completeness po regulation update** — bitmask generowany z `EquipParamGem`; nowe DLC rows mogą nie być re-imported. (54 §22.L5)
- **Orphan AoW GaItem GC / save bloat** — alokator nie zwalnia handle po reset AoW; save rośnie liniowo z liczbą AoW edits. (54 §22.L6)
- **Build Template equipment write API** — ❌ nie zaimplementowane; apply zostawia bronie unequipped. (55 §12, §21.L3)
- **Build Template spell loadout / character stats** — schema v1 nie eksportuje attunement slotów ani statystyk PlayerGameData. (55 §6)
- **Build Template forward-compat `version=2` testy** — `SchemaVersion=1` jedyny akceptowany; brak testów scenariuszy unknown-future-fields. (55 §18, §21.L8)
- **Cross-platform PS4 vs PC portability dla Build Template** — schema portable z założenia, ale brak E2E testu PS4↔PC roundtrip.
- **Frontend/backend `WEP_TYPE_TO_BIT` drift** — pojedynczy frontend mirror (`WeaponEditModal.tsx`) vs backend, bez guardu CI / generatora. (54 §17, §22.L4)
- **`replace-*` modes nie zaimplementowane** — `replace-weapons`, `replace-armors` itd. zarezerwowane w schemacie; v1 obsługuje tylko `merge`. (55 §6)

### Phase 4 — Map / World / Event Flags / Game State (Ch.9, Ch.10, Ch.11, Ch.12, Ch.13, Ch.30) ✅ UKOŃCZONA (główne rozdziały)

- **Cel**: zebrać wszystko o mapie, flagach i game state w spójną sekcję; rozwiązać konflikty F6 (27/11), F7 (13/29) i F9 (48 overclaim).
- **Wykonane** (10 osobnych commitów na branchu `docs/lang-pl-book-cleanup`):
  - **Step 1**: `15-event-flags.md` — canonical rewrite (3-tier resolver: precomputed → BST → fallback formula, helper API, 4 testy). Commit `316066e docs(lang-pl): rewrite event flags reference`.
  - **Step 2**: `27-map-reveal.md` — canonical master rewrite (4-warstwowy model L0–L3, MapVisible 263 entries, MapSystem 4 entries, MapFragmentItems 24 entries, RevealAllMap/RemoveFogOfWar/ResetMapExploration). Commit `5c962a9 docs(lang-pl): rewrite map reveal reference`.
  - **Step 3**: `11-regions.md` — canonical detail dla L0 (104 regions: 35 legacy + 62 overworld + 7 DLC, 11 unique Area values, RebuildSlot relation). Commit `0a3e3d7 docs(lang-pl): rewrite regions reference`.
  - **Step 4**: `29-dlc-black-tiles.md` — canonical detail dla L2 (DLCTile* constants z `offset_defs.go:309-322`, syntetyczne koordy 9648/9124 i 3037/1869/7880/7803, Phase 3 w revealDLCMap). Commit `0b59e87 docs(lang-pl): rewrite dlc black tiles reference`.
  - **Step 5**: `47-site-of-grace-activation.md` — canonical rewrite (419 entries, Grace EventFlag 71xxx-76xxx + DoorFlag + companion flags SET-only Gatefront 76111, 6 integration testów). Commit `f96ce6e docs(lang-pl): rewrite site of grace reference`.
  - **Step 6**: `50-item-companion-flags.md` — canonical rewrite (SET+CLEAR symmetric, 6 wpisów: Whistle + 5 multiplayer items, hook SET w AddItemsToCharacter linie 569-578, hook CLEAR w RemoveItemsFromCharacter linie 706-725, 11 unit + 17 integration testów). Commit `a1b8422 docs(lang-pl): rewrite item companion flags reference`.
  - **Step 7**: `48-pvp-ready-modular-presets.md` — current reference rewrite (5 modułów: 4 active + 1 placeholder, single pushUndo, fail-fast bez auto-restore, Sites of Grace module E placeholder explicit). Commit `b25fbd2 docs(lang-pl): rewrite pvp modular presets reference`.
  - **Step 8**: `16-world-state.md` — overview/index rewrite (subsystem map 11 wierszy, read-only verbatim blobs vs write-capable via bitfield, WorldGeomBlock corruption risk). Commit `5a00cdd docs(lang-pl): rewrite world state overview`.
  - **Step 9**: `17-player-coordinates.md` + `19-weather-time.md` — read-only refresh (**17 fix: 57→61 B z `12+4+16+1+12+16`**, brak setterów; 19 brak setterów, usunięte stare heurystyki korupcji). Commit `d7228a5 docs(lang-pl): refresh coordinates weather and time references`.
  - **Step 10**: `14-game-state.md` — canonical rewrite (PreEventFlagsScalars 29 B z 11 polami, ClearCount write path z SaveCharacter + NG+ event flag sync 50-57, LastRestedGrace read-only, boss multi-flag → 38-boss-multiflag.md). Commit `5c729a7 docs(lang-pl): rewrite game state reference`.
- **Akceptacja**: wszystkie 10 rozdziałów ma canonical template, cross-refs między sobą bez duplikacji, source-of-truth w kodzie z liniami, `needs verification` markers gdzie kod nie potwierdza w 100%, brak overclaimów (najważniejsze poprawki: `PlayerCoordinatesSize 57→61 B` w 17, "Faza 1 kompletna" → "4 active + 1 placeholder" w 48, "SaveCharacter nie ma pushUndo" → "SaveCharacter ma pushUndo" w 14).
- **Effort rzeczywisty**: 10 commitów na branchu `docs/lang-pl-book-cleanup`.

#### Cross-cutting gaps z Phase 4 (do adresowania w przyszłości)

- **Stale generated snapshots po patchu gry / regulation.bin** — `data.Graces/Regions/MapVisible/SummoningPools/ColosseumFlagSets/itemCompanionEventFlags` są statyczne; brak auto-detection.
- **PS4 ↔ PC parity tests** — round-trip pokryty; brak per-endpoint platform parity.
- **Brak cross-subsystem atomic transaction** — orchestratory używają single pushUndo bez per-fazowego rollback.
- **Manual undo / rollback limits** — undo stack depth nieznany; bulk operations N osobnych snapshotów.
- **In-game runtime verification gaps** — brak CI in-game loop.
- **Event flag ID correctness po patchu** — 3-tier resolver fallback dla nowych flag.
- **Map reveal visual vs progression effects** — `WorldTab.tsx:406` UI nota; trofea impact niezweryfikowany.
- **DLC black tile coordinates stale-after-patch** — wartości empiryczne, nie game-guaranteed.
- **Sites of Grace SET-only intent + PvP module E placeholder** — SET-only companion flags w grace lifecycle; PvP module zwraca warning.
- **Item companion flag IDs stale-after-patch** — 6 hardcoded literałów.
- **PvP "ready" scope ograniczony** — physical colosseum gates w WorldGeomMan blob nieedytowalne; Summoning Pools "Bloody Finger impact unconfirmed".
- **Player Coordinates / Weather-Time read-only** — brak publicznych setterów, każda mutacja przez direct hex edit.
- **Game State: LastRestedGrace read-only** — ClearCount jedyny write path z `SaveCharacter` + flag sync 50-57; brak progression consistency validator.
- **Boss multi-flag editor pozostaje planned** — single-flag `SetBossDefeated` aktualnie; multi-flag w `38-boss-multiflag.md`.

### Phase 4 Step 11 — index update (bieżący)

- **Cel**: zaktualizować README.md + BOOK_PLAN.md po Phase 4.
- **Zakres**: README.md (status header, Phase 4 cross-cutting gaps, spis treści entries), BOOK_PLAN.md (Phase 4 ✅ UKOŃCZONA z listą Step 1-10 commit hash, resolved conflicts F6/F7/F9, merge/rewrite candidates 29/48 marked resolved).
- **Akceptacja**: README i BOOK_PLAN odzwierciedlają stan Phase 4; brak overclaimów; linki valid.

### Phase 5 — Ban-risk + unsafe edits reference (App. A, App. B) — NASTĘPNA

- **Cel**: ustabilizować appendiksy z referencją ryzyka bana + skonsolidować safety/validation/rollback wiedzę rozproszoną po Phase 2-4 rozdziałach.
- **Pliki**: 45 → App. A (community triggers); 32 → App. B (UI tier system); cross-refs do każdego rozdziału z edycjami Tier 1/2.
- **Kod**: brak nowych — sync z `frontend/src/state/safetyMode.tsx`, `RISK_INFO` w `components/Risk*.tsx`.
- **Akceptacja**: każdy rozdział Part III/IV ma footer „Ban-risk / safety notes" z linkiem do App. A; centralized safety/validation/rollback index z cross-refs do `Phase 2-4` safety notes.
- **Effort**: 3–4 h.

### Phase 6 — Glossary, code-to-doc index, offset index

- **Cel**: dodać dwa nowe pliki referencyjne.
- **Pliki**: `spec/lang-pl/INDEX.md` (offsety hex → rozdziały); `spec/lang-pl/GLOSSARY.md` (GaItem, AoW, ClearCount, Mirror Favorites, Acquisition Index, ...).
- **Akceptacja**: każdy hex offset ≥ 0x100 w jakimkolwiek rozdziale ma wpis w INDEX; każdy unikalny termin w 5+ dokumentach jest w GLOSSARY.
- **Effort**: 4–6 h.

**Łączny effort**: ~35–45 h. Każda faza wymaga osobnego brancha + osobnego review.

---

## Dokumenty do późniejszego merge / rewrite

Lista dokumentów, które po Phase 1 zostały w głównym katalogu, ale wymagają interwencji w późniejszych fazach.

### Merge candidates (do scalenia z innym rozdziałem)

| Doc | Target chapter | Powód | Status |
|---|---|---|---|
| ~~03 §AoW~~ | Ch.7 (z 54) | duplikacja semantyki AoW | ✅ Phase 2 Step 2 + Phase 3 Step 1: cross-ref do 54, no duplication; konsolidacja AoW domknięta po stronie 54 |
| ~~10~~ | Ch.4 §4.2 (z 07) | identyczny format jak 07, inne countery | ✅ Phase 2 Step 3: 10 zachowane jako osobny canonical (oba przepisane) |
| ~~54 §AoW write paths / availability / compat~~ | Ch.7 | scattered between 03/06/35/UI komponenty | ✅ Phase 3 Step 1: 54 jest single source of truth dla strict vs allocate+rebuild write paths, ScanAoWAvailability, fail-closed compat, AoW guard |
| ~~55 §Build Template portable payload + Phase E library~~ | Ch.25 | rozproszone w backend/templates + app_templates + frontend templates | ✅ Phase 3 Step 2: 55 obejmuje JSON v1 schema, portable rule, capacity preflight, RAM-only apply + Phase E local library |
| 18 | Ch.14 (z disclaimerem) | jeden krótki opaque blob, nie wart własnego rozdziału | nadal otwarte |

### Rewrite candidates (do przepisania w canonical template)

| Doc | Powód | Status |
|---|---|---|
| 05-sp-effects | krótki, „wymaga weryfikacji" w treści — albo doczytać `structures.go`, albo zaznaczyć jako stub | nadal otwarte |
| ~~07-inventory~~ | solidne fakty, chaotyczna kolejność | ✅ Phase 2 Step 3 |
| 09-face-data | większość pól „przybliżone"; `app_appearance.go` zna więcej | nadal otwarte |
| 26-parameter-reference | rozbić: atrybuty → Ch.5, softcaps → App. D | nadal otwarte |
| ~~29-dlc-black-tiles~~ | split: spec → Ch.11; research log → App. E | ✅ Phase 4 Step 4: canonical detail dla L2 DLC Cover Layer; historical binary search test 7-18 przeniesiony do `docs/CHANGELOG.md` cross-ref. |
| ~~39-inventory-reorder~~ | status nieaktualny — kod istnieje, stride-2 odkryty (patrz konflikt F2) | ✅ Phase 2 Step 5: historical/superseded note |
| ~~48-pvp-ready-modular-presets~~ | rozdzielić: design+wdrożone (Ch.28), planowane (App. F) | ✅ Phase 4 Step 7: current reference rewrite; 4 active modules + 1 placeholder (Sites of Grace module E) jasno udokumentowane bez overclaimu. |

---

## Konflikty doc vs code (skrót z audytu)

Pełna lista w `tmp/docs-book-audit.md` § F. Skrót:

| # | Dok | Twierdzenie z dokumentu | Rzeczywistość z kodu |
|---|---|---|---|
| F1 | 38 | `BossData{ EventFlags []uint32 }` + multi-flag boss kill | `backend/db/data/bosses.go:4` — tylko `Name/Region/Type/Remembrance`; `app_world.go:113 SetBossDefeated` przyjmuje pojedynczy `bossID` |
| F2 | 39 | Status: `🔲 Planowany — zablokowany w Fazie 0` | ✅ **Rozwiązany w Phase 2 Step 5**: 39 jest historical/superseded design note; canonical mechanika w 52, transfer UX w 53. |
| F3 | 37 | Status: `🔲 Planowany` | `backend/vm/preset.go` ma `CharacterPreset/VMToPreset/PresetToVM/ValidatePreset` — `needs verification` per Phase |
| F4 | 03/54 | Sentinele AoW + invariant unikalności handle | ✅ **Rozwiązany w Phase 2 Step 2 + Phase 3 Step 1**: 03 ma cross-ref do 54, 54 jest single source of truth dla obu sentineli (`NoCustomAoWHandle = 0x00000000` canonical, `LegacyNoCustomAoWHandle = 0xFFFFFFFF`) + shared-handle invariant + AoW Allocation Safety guard. |
| F5 | 33/36 | 33 deklaruje reklasyfikację Information tab | ✅ **Rozwiązany w Phase 1 + Phase 2 Step 7**: 33 usunięte (post-mortem zarchiwizowane w git history); 36 canonical (handle prefix bridge + sub-categories + DLC flag mechanism). |
| F6 | 27/11 | Obie sekcje cytują `core.SetUnlockedRegions` | ✅ **Rozwiązany w Phase 4 Step 2 + Step 3**: 27 jest master dla 4-warstwowego Map Reveal; 11 jest L0 detail i linkuje do 27 dla orchestration. Brak duplikacji. |
| F7 | 13/29 | 13: pola `unk_0x1c..0x40` jako Unknown | ✅ **Częściowo rozwiązany w Phase 4 Step 4**: 29 jest canonical detail dla L2 DLC Cover Layer w `BloodStain` (zakres `[0x0088..0x0110)`); 13 pozostaje `partial` z odsyłaczem (rewrite 13 → Phase 5+). |
| F8 | 14/34 | TODO o ClearCount inkrementacji | ✅ **Częściowo rozwiązany w Phase 4 Step 10**: 14 dokumentuje ClearCount write path przez SaveCharacter + NG+ flag sync 50-57; cap @ 7; TODO o auto-inkrementacji nadal otwarte (intencjonalnie). |
| F9 | 48 | Status: `✅ Faza 1 kompletna` | ✅ **Rozwiązany w Phase 4 Step 7**: 48 status zmieniony na `current reference`; module status table jasno pokazuje 4 active + 1 placeholder; Sites of Grace module E explicit warning. |
| F10 | 51 | Status: `🔲 Planowany`, sekcja „Układ UI" | `grep AdvancedSaveEditor` → 0 wyników; pure design doc |
| F11 | 05 | Layout SpEffect 16 B (id/duration/unk1/unk2) + „liczba wpisów wymaga weryfikacji" | Brak `parseSpEffects` w `backend/core/` — sekcja jest opaque blob w write path |
| F12 | 09 | Rozmiar 303 B, pola 0x20+ „przybliżone" | `db/data/presets_generated.go` (4 KB+) opisuje całe presety — wewnętrzna struktura lepiej znana w kodzie niż w spec |
| F13 | 26 | „Kompletna referencja wszystkich edytowalnych parametrów" | Pokrywa głównie atrybuty + softcaps; nie obejmuje pełni `RuneArc`, `Crucible Aspect`, `Scadutree Blessing` |

**Decyzja właściciela**: każdy konflikt wymaga decyzji „zaktualizować doc do kodu" lub „zaktualizować kod do doc". Audytor nie podjął tych decyzji — są zadaniem Phase 2-5.

---

## Template rozdziału

Każdy finalny rozdział książki musi używać tej kolejności sekcji:

````markdown
# Ch.N — Title

> **Part**: I / II / III / IV / V
> **Status**: ✅ Implemented | 🔲 Planned | 🐛 Research | 📚 Reference
> **Code-of-record**: `backend/core/<file>.go`, `app_<feature>.go`
> **Tests-of-record**: `backend/core/<file>_test.go`, `tests/<file>_test.go`
> **Source docs (pre-merge)**: NN-foo.md, NN-bar.md

---

## Purpose
Jedno-dwa zdania: po co istnieje ta sekcja save'a / ta funkcja edytora. Po co czytelnik tu jest.

## Status
- Co jest zweryfikowane (✅), co cross-reference only (⚠️), co niepewne (❓).
- Ostatnia weryfikacja: <data> na <ścieżka save>.

## Source of truth in code
Lista funkcji + plików z linkami do konkretnych linii (np. `writer.go:430 allocateGaItem`).
To jest sekcja, którą czytelnik klika gdy chce zobaczyć ground truth.

## Binary layout / data model
Tabela offsetów hex z typami i opisem. Diagram ASCII gdzie pomaga. Cytować rozmiary w hex + dec.

## Read path
Jak edytor wczytuje tę sekcję (lub jak gra ją interpretuje). Wskazać `reader.go` / `structures.go`.

## Write path
Jak edytor zapisuje, jakie invarianty utrzymuje, jakie countery aktualizuje. Wskazać `writer.go` / `editor/save.go`.

## Validation / invariants
Lista zasad, które MUSZĄ być spełnione (np. „NextEquipIndex > każdy acquisition_index w slocie").
Z konsekwencjami niespełnienia (crash, niewidoczne itemy, ban risk).

## Tests
Lista testów weryfikujących inwarianty. Komenda do uruchomienia, oczekiwany wynik.

## Known limits
Co aktualnie NIE działa, co jest TODO, czego edytor nie obsługuje. Bez ukrywania.

## Ban-risk / safety notes
Jeśli edycja tej sekcji ma poziom ryzyka — Tier 0/1/2 z spec/45 + spec/32. Link do App. A.

## Historical notes (opcjonalne)
Jeśli było research → discovery → implementation, krótkie how-we-got-here. Inaczej pomijać.

## See also
Linki do innych rozdziałów + powiązanych research lub planned docs w root `spec/lang-pl/`.

## Sources
- Reference parsers (er-save-manager, ER-Save-Editor)
- Cheat tables
- Community wiki / Fextralife
- Hex-verified save files (`tmp/save/...`)
````

**Zalety**: każdy rozdział ma identyczny szkielet, czytelnik wie gdzie szukać. „Code-of-record" i „Tests-of-record" to twardy kontrakt — gdy ktoś zmienia kod, łatwo zobaczyć który rozdział aktualizować.
