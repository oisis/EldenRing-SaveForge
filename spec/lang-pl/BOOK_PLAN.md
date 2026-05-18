# BOOK_PLAN — `spec/lang-pl/` jako podręcznik dla moderów

> **Cel**: roadmapa konsolidacji `spec/lang-pl/` w spójną książkę. Źródłem prawdy są: kod w `backend/`, `app*.go`, `frontend/src/` oraz aktualny audyt `tmp/docs-book-audit.md` (lokalny, gitignored).
>
> **Aktualna faza**: Phase 1 — reorganizacja katalogowa (✅ in progress, ten branch).
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

### Phase 1 — Reorganizacja katalogowa + README (W TRAKCIE)

- **Cel**: rozdzielić referencję od research/planned/archive bez zmian merytorycznych.
- **Zakres**: utworzenie `research/`, `planned/`, `archive/`; `git mv` 8 dokumentów; rewrite `README.md` + nowy `BOOK_PLAN.md`.
- **Akceptacja**: główny katalog `spec/lang-pl/` zawiera tylko dokumenty kandydujące do rozdziałów; brak dangling linków; vanilla MD5 nietknięty.
- **Effort**: 2–3 h.

### Phase 2 — Inventory + Storage + GaItem (Ch.3, Ch.4, Ch.5, Ch.6, Ch.21)

- **Cel**: skonsolidować 43, 53, 52, 03 (layout), 07, 10, 39 w spójną narrację.
- **Pliki do ruszenia**:
  - 03 — split: AoW out (do 54), layout in (do Ch.3)
  - 07 — rewrite w canonical template
  - 10 — merge do 07 jako §4.2
  - 39 — rewrite jako historyczny rationale + cross-ref do 52
  - 43, 52, 53 — keep, lekki refresh
- **Kod do skrosowania**: `writer.go::AddItemsToSlotBatch`, `transfer.go`, `app_inventory_order.go::ReorderInventory`, `editor/save.go::executeAdds`, `structures.go::mapInventory/mapStorage/ReconcileInventoryHeader`.
- **Akceptacja**: wszystkie odsyłacze `file:line` zgodne z aktualnym HEAD; jednolity template.
- **Effort**: 8–12 h.

### Phase 3 — Ash of War + Build Template (Ch.7, Ch.25)

- **Cel**: zamknąć temat AoW (54 + AoW guard z commit `6881cb9` + invariant z 03 + UI z `WeaponEditModal.tsx`) oraz udokumentować Build Template (55).
- **Pliki**: 54 (dodać sekcję o guard), 03 §AoW (przeniesione → 54 cross-ref), 55 (refresh).
- **Kod**: `aow_availability.go`, `editor/weapon.go`, `writer.go::allocateGaItem` (guard), `gaitem_placement_test.go::TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity`, `backend/templates/*.go`, `app_templates.go`, `frontend/src/components/templates/*`.
- **Akceptacja**: Ch.7 zawiera tabelę „what happens when allocateGaItem returns error"; Ch.25 ma kompletny schemat JSON v1 + przykład eksportu/importu.
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

| Doc | Target chapter | Powód |
|---|---|---|
| 03 §AoW | Ch.7 (z 54) | duplikacja semantyki AoW |
| 10 | Ch.4 §4.2 (z 07) | identyczny format jak 07, inne countery |
| 18 | Ch.14 (z disclaimerem) | jeden krótki opaque blob, nie wart własnego rozdziału |

### Rewrite candidates (do przepisania w canonical template)

| Doc | Powód |
|---|---|
| 05-sp-effects | krótki, „wymaga weryfikacji" w treści — albo doczytać `structures.go`, albo zaznaczyć jako stub |
| 07-inventory | solidne fakty, chaotyczna kolejność |
| 09-face-data | większość pól „przybliżone"; `app_appearance.go` zna więcej |
| 26-parameter-reference | rozbić: atrybuty → Ch.5, softcaps → App. D |
| 29-dlc-black-tiles | split: spec → Ch.11; research log → App. E |
| 39-inventory-reorder | status nieaktualny — kod istnieje, stride-2 odkryty (patrz konflikt F2) |
| 48-pvp-ready-modular-presets | rozdzielić: design+wdrożone (Ch.28), planowane (App. F) |

---

## Konflikty doc vs code (skrót z audytu)

Pełna lista w `tmp/docs-book-audit.md` § F. Skrót:

| # | Dok | Twierdzenie z dokumentu | Rzeczywistość z kodu |
|---|---|---|---|
| F1 | 38 | `BossData{ EventFlags []uint32 }` + multi-flag boss kill | `backend/db/data/bosses.go:4` — tylko `Name/Region/Type/Remembrance`; `app_world.go:113 SetBossDefeated` przyjmuje pojedynczy `bossID` |
| F2 | 39 | Status: `🔲 Planowany — zablokowany w Fazie 0` | `app_inventory_order.go::ReorderInventory` zaimplementowany, `SortOrderTab.tsx` ma UI, stride-2 odkryty (spec/52) |
| F3 | 37 | Status: `🔲 Planowany` | `backend/vm/preset.go` ma `CharacterPreset/VMToPreset/PresetToVM/ValidatePreset` — `needs verification` per Phase |
| F4 | 03/54 | Sentinele AoW + invariant unikalności handle | Ta sama treść w obu dokumentach; po commicie `cb1a822` source-of-truth to 54 |
| F5 | 33/36 | 33 deklaruje reklasyfikację Information tab | 36 explicit: „spec/36 dokończa pracę… Larval Tears, Torches, Region Maps, Golden Runes" — 33 to pre-36 snapshot |
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
