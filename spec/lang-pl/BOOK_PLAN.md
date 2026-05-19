# BOOK_PLAN — `spec/lang-pl/` jako podręcznik dla moderów

> **Cel**: roadmapa konsolidacji `spec/lang-pl/` w spójną książkę. Źródłem prawdy są: kod w `backend/`, `app*.go`, `frontend/src/` oraz aktualny audyt `tmp/docs-book-audit.md` (lokalny, gitignored).
>
> **Aktualna faza**: Phase 2 ✅ ukończona dla głównych rozdziałów (GaItem + Inventory + Storage + Sort Order + Transfer + Categories + Equipment). Phase 3 (Ash of War + Build Template) — następna.
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
| E | Research log — negative results | 30 (pełny), 41, 42, 46 |
| F | Planned features | 37, 38, 40, 51 |
| G | DB categorization history | 33 |

**Konsolidacja**: 52 dokumenty → 30 rozdziałów + 7 załączników (1.7×).

---

## Fazy dalszych prac

### Phase 1 — Reorganizacja katalogowa + README ✅ UKOŃCZONA

- **Cel**: rozdzielić referencję od research/planned/archive bez zmian merytorycznych.
- **Zakres**: utworzenie `research/`, `planned/`, `archive/`; `git mv` 8 dokumentów; rewrite `README.md` + nowy `BOOK_PLAN.md`.
- **Akceptacja**: główny katalog `spec/lang-pl/` zawiera tylko dokumenty kandydujące do rozdziałów; brak dangling linków; vanilla MD5 nietknięty. ✅
- **Effort**: 2–3 h.
- **Commit**: `1c2fbab docs(lang-pl): reorganize book structure`.

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

### Phase 3 — Ash of War + Build Template (Ch.7, Ch.25) — NASTĘPNA

- **Cel**: zamknąć temat AoW (54 + AoW guard z commit `6881cb9` + invariant z 03/35 + UI z `WeaponEditModal.tsx`) oraz udokumentować Build Template (55).
- **Pliki**: 54 (dodać sekcję o guard, sprawdzić zgodność z aktualnymi 03/35), 55 (refresh dla workspace integration).
- **Kod**: `aow_availability.go`, `editor/weapon.go`, `writer.go::allocateGaItem` (guard), `gaitem_placement_test.go::TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity`, `backend/templates/*.go`, `app_templates.go`, `frontend/src/components/templates/*`.
- **Akceptacja**: Ch.7 zawiera tabelę „what happens when allocateGaItem returns error"; Ch.25 ma kompletny schemat JSON v1 + przykład eksportu/importu.
- **Stan wejściowy**: 03 i 35 po Phase 2 zawierają cross-refs do 54 zamiast duplikacji — Phase 3 dokończa konsolidację po stronie 54.
- **Effort**: 6–8 h.

### Phase 4 — Map / World / Event Flags (Ch.9, Ch.10, Ch.11, Ch.12, Ch.13, Ch.30)

- **Cel**: zebrać wszystko o mapie i flagach w spójną sekcję.
- **Pliki**: 15 (kanon), 27 (kanon), 29 (split — layout do Ch.11, research do App. E), 11 (merge do Ch.11 §Layer 0), 16, 17, 47, 50.
- **Kod**: `app_world.go::reveal*Map / RemoveFogOfWar / SetBossDefeated`, `db/data/maps.go`, `db/data/graces.go`, `db/data/bosses.go`, `db/data/grace_companion_flags.go`.
- **Akceptacja**: Ch.11 nie powtarza 29 dziennika testów; Ch.30 jasno mówi „kod aktualnie ustawia jedną flagę pokonania; planowana rozbudowa multi-flag w `planned/38`".
- **Effort**: 8–10 h.

### Phase 5 — Ban-risk + unsafe edits reference (App. A, App. B)

- **Cel**: ustabilizować appendiksy z referencją ryzyka bana.
- **Pliki**: 45 → App. A (community triggers); 32 → App. B (UI tier system); cross-refs do każdego rozdziału z edycjami Tier 1/2.
- **Kod**: brak nowych — sync z `frontend/src/state/safetyMode.tsx`, `RISK_INFO` w `components/Risk*.tsx`.
- **Akceptacja**: każdy rozdział Part III/IV ma footer „Ban-risk / safety notes" z linkiem do App. A.
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
| ~~03 §AoW~~ | Ch.7 (z 54) | duplikacja semantyki AoW | ✅ Phase 2 Step 2: cross-ref do 54, no duplication |
| ~~10~~ | Ch.4 §4.2 (z 07) | identyczny format jak 07, inne countery | ✅ Phase 2 Step 3: 10 zachowane jako osobny canonical (oba przepisane) |
| 18 | Ch.14 (z disclaimerem) | jeden krótki opaque blob, nie wart własnego rozdziału | nadal otwarte |

### Rewrite candidates (do przepisania w canonical template)

| Doc | Powód | Status |
|---|---|---|
| 05-sp-effects | krótki, „wymaga weryfikacji" w treści — albo doczytać `structures.go`, albo zaznaczyć jako stub | nadal otwarte |
| ~~07-inventory~~ | solidne fakty, chaotyczna kolejność | ✅ Phase 2 Step 3 |
| 09-face-data | większość pól „przybliżone"; `app_appearance.go` zna więcej | nadal otwarte |
| 26-parameter-reference | rozbić: atrybuty → Ch.5, softcaps → App. D | nadal otwarte |
| 29-dlc-black-tiles | split: spec → Ch.11; research log → App. E | nadal otwarte (Phase 4) |
| ~~39-inventory-reorder~~ | status nieaktualny — kod istnieje, stride-2 odkryty (patrz konflikt F2) | ✅ Phase 2 Step 5: historical/superseded note |
| 48-pvp-ready-modular-presets | rozdzielić: design+wdrożone (Ch.28), planowane (App. F) | nadal otwarte (Phase 5) |

---

## Konflikty doc vs code (skrót z audytu)

Pełna lista w `tmp/docs-book-audit.md` § F. Skrót:

| # | Dok | Twierdzenie z dokumentu | Rzeczywistość z kodu |
|---|---|---|---|
| F1 | 38 | `BossData{ EventFlags []uint32 }` + multi-flag boss kill | `backend/db/data/bosses.go:4` — tylko `Name/Region/Type/Remembrance`; `app_world.go:113 SetBossDefeated` przyjmuje pojedynczy `bossID` |
| F2 | 39 | Status: `🔲 Planowany — zablokowany w Fazie 0` | ✅ **Rozwiązany w Phase 2 Step 5**: 39 jest historical/superseded design note; canonical mechanika w 52, transfer UX w 53. |
| F3 | 37 | Status: `🔲 Planowany` | `backend/vm/preset.go` ma `CharacterPreset/VMToPreset/PresetToVM/ValidatePreset` — `needs verification` per Phase |
| F4 | 03/54 | Sentinele AoW + invariant unikalności handle | ✅ **Częściowo rozwiązany w Phase 2 Step 2**: 03 (canonical) ma cross-ref do 54 zamiast duplikacji; pełna konsolidacja po stronie 54 w Phase 3. |
| F5 | 33/36 | 33 deklaruje reklasyfikację Information tab | ✅ **Rozwiązany w Phase 1 + Phase 2 Step 7**: 33 w `archive/`, 36 canonical (handle prefix bridge + sub-categories + DLC flag mechanism). |
| F6 | 27/11 | Obie sekcje cytują `core.SetUnlockedRegions` | Po reorganizacji jeden powinien tylko linkować |
| F7 | 13/29 | 13: pola `unk_0x1c..0x40` jako Unknown | 29: ten zakres (`afterRegs+0x88..0x110`) to DLC Cover Layer (8 floats × 2 rekordy) — wiedza nie zaimportowana do 13 |
| F8 | 14/34 | TODO o ClearCount inkrementacji | TODO powtarza się w obu dokumentach — nadal otwarte |
| F9 | 48 | Status: `✅ Faza 1 kompletna` | `app_pvp.go:109` zwraca warning `"Sites of Grace module is planned but not enabled"` |
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
Linki do innych rozdziałów + przeniesionych do appendix research/planned docs.

## Sources
- Reference parsers (er-save-manager, ER-Save-Editor)
- Cheat tables
- Community wiki / Fextralife
- Hex-verified save files (`tmp/save/...`)
````

**Zalety**: każdy rozdział ma identyczny szkielet, czytelnik wie gdzie szukać. „Code-of-record" i „Tests-of-record" to twardy kontrakt — gdy ktoś zmienia kod, łatwo zobaczyć który rozdział aktualizować.
