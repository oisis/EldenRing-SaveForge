# 36 — Inventory Categories: Game-Accurate Order & Sub-Grouping

> **Zakres**: Pełne wyrównanie zakładek `Inventory` i `Item Database` z układem
> ekwipunku w grze (1:1 — nazwy, kolejność, sub-grupy). Następuje po
> [spec/33 — DB Categorization Audit](33-db-categorization-audit.md), które
> ustaliło source-of-truth na **Fextralife per-item breadcrumb** + verification
> in-game. Spec/36 dokończa pracę: definiuje 18 zakładek głównych w kolejności
> z gry, mapuje sub-grupy (gdzie gra je pokazuje) i refactoruje kod
> backendu / frontendu / ikon do tej taksonomii.
>
> **Status**: ✅ Wdrożone na `feat/inventory-game-accurate-categories`
> (kwiecień 2026).
>
> **Źródło prawdy**: in-game UI (PC make dev + Steam Deck verification),
> Fextralife per-item breadcrumb, Eldenpedia inventory tab list.

---

## 1. Filozofia

Aktualny dropdown kategorii w `Item Database` był półtechniczny —
niektóre wartości pochodziły z `er-save-manager`'s `Goods/*.txt` (community
taxonomy), inne były nazwami plików `.go`. Gracz otwierający edytor obok gry
musiał tłumaczyć "Information" na "Informacje" i szukać Larval Tears w
`Bolstering Materials` mimo że gra pokazuje je w `Key Items`. Spec/36 usuwa
ten warstwę tłumaczenia: każda nazwa zakładki, każda sub-grupa i każda klasa
broni odpowiada 1:1 temu, co widzi gracz w menu ekwipunku gry.

**Naming rules** (egzekwowane):
- `Ashes` (nie "Spirit Ashes")
- `Info` (nie "Information")
- `/` jako separator (nie `&`) — np. `Ranged Weapons / Catalysts`, `Arrows / Bolts`

---

## 2. Kanoniczna kolejność 18 zakładek

| # | Tab w grze | DB `category` | Sub-grupy? |
|---|---|---|---|
| 1 | Tools | `tools` | tak (12) |
| 2 | Ashes | `ashes` | nie |
| 3 | Crafting Materials | `crafting_materials` | nie |
| 4 | Bolstering Materials | `bolstering_materials` | tak (6) |
| 5 | Key Items | `key_items` | tak (9) |
| 6 | Sorceries | `sorceries` | nie |
| 7 | Incantations | `incantations` | nie |
| 8 | Ashes of War | `ashes_of_war` | nie |
| 9 | Melee Armaments | `melee_armaments` | tak (29) |
| 10 | Ranged Weapons / Catalysts | `ranged_and_catalysts` | tak (7) |
| 11 | Arrows / Bolts | `arrows_and_bolts` | tak (4) |
| 12 | Shields | `shields` | tak (4) |
| 13 | Head | `head` | nie |
| 14 | Chest | `chest` | nie |
| 15 | Arms | `arms` | nie |
| 16 | Legs | `legs` | nie |
| 17 | Talismans | `talismans` | nie |
| 18 | Info | `info` | tak (3) |

Frontend dropdown (`CategorySelect.tsx`) renderuje powyższą kolejność jako
płaską listę 18 `<option>` (bez `<optgroup>`) — `value` matchuje kolumnę
`DB category`.

---

## 3. Sub-grupy per zakładka (kolejność z gry)

### Tools (12 sub-grup)

```
1.  Sacred Flasks, Reusable Tools & FP Regenerators
2.  Consumables
3.  Throwing Pots
4.  Perfume Arts                   ← consumables, NIE Perfume Bottles weapon
5.  Throwables
6.  Catalyst Tools
7.  Grease
8.  Miscellaneous Tools
9.  Quest Tools                    ← currently empty (quest items live in info/key_items)
10. Golden Runes                   ← przeniesione z bolstering_materials
11. Remembrances                   ← przeniesione z key_items (spec/33)
12. Multiplayer Items              ← przeniesione z key_items (spec/33)
```

### Bolstering Materials (6 sub-grup)

```
1. Flask Enhancers                 (Golden Seeds + Sacred Tears)
2. Shadow Realm Blessings (DLC)    (Scadutree Fragment + Revered Spirit Ash)
3. Smithing Stones [1-8] + Ancient Dragon Smithing Stone
4. Somberstones [1-9] + Somber Ancient Dragon Smithing Stone
5. Grave Glovewort [1-9] + Great Grave Glovewort
6. Ghost Glovewort [1-9] + Great Ghost Glovewort
```

### Key Items (9 sub-grup)

```
1. Active Great Runes              ← currently empty (DB nie rozróżnia active/inactive)
2. Crystal Tears
3. Containers + Slot Upgrades      (Empty Cracked/Ritual/Perfume Pots/Bottles, Memory Stones, Talisman Pouches)
4. Inactive Great Runes + Keys + Medallions    ← catch-all (story keys, medallions, quest tokens)
5. DLC Keys
6. Larval Tears + Deathroot + Lost Ashes of War
7. Cookbooks                       (incl. Crafting Kit, Spirit Calling Bell, Whetblades, Sewing Needles)
8. World Maps                      (24 region maps — przeniesione z info.go w spec/36)
9. Sorcery Scrolls + Incantation Scrolls
```

### Melee Armaments (29 klas broni — base + DLC interleaved)

```
Daggers → Throwing Blades (DLC) → Straight Swords → Light Greatswords (DLC) →
Greatswords → Colossal Swords → Thrusting Swords → Heavy Thrusting Swords →
Curved Swords → Curved Greatswords → Backhand Blades (DLC) → Katanas →
Great Katanas (DLC) → Twinblades → Axes → Greataxes → Hammers → Great Hammers →
Flails → Spears → Great Spears → Halberds → Reapers → Whips → Fists →
Hand-to-Hand (DLC) → Claws → Beast Claws (DLC) → Colossal Weapons →
Perfume Bottles (DLC, weapon)
```

### Ranged Weapons / Catalysts (7 sub-grup)

```
Bows → Light Bows → Greatbows → Crossbows → Ballistas → Glintstone Staffs → Sacred Seals
```

### Arrows / Bolts (4 sub-grup)

```
Arrows → Greatarrows → Bolts → Greatbolts
```

### Shields (4 sub-grup)

```
Torches  ← TOP (przeniesione z melee_armaments)
Small Shields
Medium Shields
Greatshields
```

### Info (3 sub-grup)

```
1. Letters / Maps / Paintings           (base game)
2. Letters / Maps / Paintings (DLC)
3. Mechanics / Locations Info           (About* tutoriale + Notes)
```

---

## 4. Reklasyfikacje wykonane (vs poprzedni stan)

| Item / grupa | Z (przed) | Do (gra) | Powód |
|---|---|---|---|
| **Larval Tears** | (już) `key_items.go` | Key Items / Larval Tears + Deathroot + Lost AoW | Plan zakładał Bolstering, ale były już w Key Items — zmienił się tylko SubCategory. |
| **Whetblades + Cookbooks** | filtered out z `key_items` | Key Items / Cookbooks (sub) | `IsCookbookItemID` / `IsWhetbladeItemID` filtry usunięte z `db.GetItemsByCategory`. Zostają zarządzane z dedykowanego World UI **i** widoczne w Item Database. |
| **Bell Bearings** | filtered out z `tools` | (zostawione filtered) | Decyzja user (option A, 2026-04-28): zarządzane wyłącznie z dedykowanego World → Bell Bearings UI (single source of truth — patrz ROADMAP Phase 4). Filtr `IsBellBearingItemID` zachowany. |
| **Torches** (9) | `melee_armaments` | **Shields** (sub: Torches, top) | Gra pokazuje torches w Shields tab. Stray `Torchpole` (`0x00F55C80`) miał już `Category: "shields"` ale leżał w `weapons.go` — przeniesiony do `shields.go`. |
| **Region Maps** (24) | `info.go` | **`key_items.go`** (sub: World Maps) | Wcześniej zduplikowane między tools/info/key_items. Gra pokazuje je w Key Items. Zero duplikacji — usunięte z info.go, dodane raz w key_items.go. |
| **Golden Runes** (33) | `bolstering_materials.go` | **`tools.go`** (sub: Golden Runes) | Gra grupuje wszystkie Runes pod Tools. User decyzja (option A, 2026-04-28): runy z bolstering. |
| **Perfume Bottles** (3 weapons) | (Tools, niejasne) | **`melee_armaments.go`** (sub: Perfume Bottles, DLC) | DLC weapon class. Confused z Perfume Arts (consumables) — ostatecznie obie grupy zachowane: Tools/Perfume Arts (6 consumables) + Melee/Perfume Bottles (3 weapons). |
| **Bastard Sword, Bolt of Gransax, Bloody Helice** | catch-all Straight Swords (best-effort initial) | poprawione w `melee_subcat.go` curated lookup | Bastard Sword → Greatswords; Bolt of Gransax → Great Spears; Bloody Helice → Heavy Thrusting Swords. Verification passes opisane w sekcji 7. |

---

## 5. Architektura kodu

### Backend (`backend/db/data/`)

**Pliki przemianowane (Phase 0 — git mv):**
- `weapons.go` → `melee_armaments.go`
- `aows.go` → `ashes_of_war.go`
- `helms.go` → `head.go`

(Var names — `Weapons`, `Aows`, `Helms` — pozostały bez zmian; rename plików nie pociąga rename symboli, żeby nie rozsadzać `db.go::GetItemsByCategory`.)

**Nowe pliki:**
- `subcategories.go` — wszystkie 70 stałych `Subcat*` w jednej liście prawdy.
- `melee_subcat.go` — curated lookup tables dla 30 klas broni + suffix fallback. Strip infusion prefixes (Heavy/Keen/Cold/Sacred/Fire/…) przed lookup.
- `key_items_subcat.go` — curated ID set dla Crystal Tears, name-pattern dla Cookbooks/Containers/Larval/SpellScrolls/DLCKeys, fallback do `Inactive Great Runes + Keys + Medallions` (catch-all dla story tokens).
- `info_subcat.go` — reguły: prefix `"About "` lub `"Note: "` → Mechanics/Locations; flag `dlc` → DLC Letters/Maps; reszta → base Letters/Maps/Paintings.
- `ranged_and_catalysts_subcat.go` — suffix-based: Greatbow/Shortbow/Bow → Bows klasa; Crossbow/Ballista; "Seal"/"Staff" → catalyst.
- `shields_subcat.go` — name-based curated lookup z 4 setami.

**Plik z inline SubCategory (Phase 2f):**
- `tools.go` — wszystkie 328 entries mają `SubCategory: SubcatXxx` w-line. Generator (`tmp/inline-tools-subcat/main.go`) odczytał populated `data.Tools` po init() i wstrzyknął field + spłaszczył IconPath. Stary `tools_subcat.go` usunięty.

**Schema (`types.go`):**
```go
type ItemData struct {
    Name        string
    Category    string   // 18-tab top-level (tools, key_items, …)
    SubCategory string   // sub-group within tab (Sacred Flasks, Daggers, …) — empty if tab has no sub-cats
    IconPath    string
    // … (caps, flags, …)
}
```

### Backend (`backend/db/db.go`)

`ItemEntry` wraca do frontendu z dodatkowym field `SubCategory string`. Pięć
miejsc tworzących `ItemEntry` w `GetItemsByCategory` zostało zaktualizowanych
do propagacji.

### Backend (`app.go`)

Nowa metoda:
```go
func (a *App) GetItemListChunk(category string) []db.ItemEntry
```
Zwraca jedną zakładkę naraz — frontend `Item Database` w widoku "All
Categories" wywołuje ją 18× w pętli z `await new Promise(r => setTimeout(r, 0))`
między iteracjami → progressive loading bez blokowania scrolla.

### Frontend

| Plik | Zmiana |
|---|---|
| `CategorySelect.tsx` | Płaska lista 18 opcji w kolejności z gry; usunięte `<optgroup>`. Eksportuje `CATEGORY_VALUES` (używane przez progressive loader w DatabaseTab). |
| `InventoryTab.tsx` | Top bar `[Cat][Owned/Total badge][Search]`; kolumna `Category` → `Sub-Category` (chowana dla zakładek bez sub-cats); search debounce 200 ms via `useDeferredValue`; capacity bar przeniesiony do App.tsx. |
| `DatabaseTab.tsx` | Top bar i kolumna jak wyżej; progressive load 18× chunk + thin progress strip nad tabelą (pointer-events: none). |
| `App.tsx` | Header Inventory: `[toggle pills][global capacity bar]`. Header Database: `[toggle pills][▶ Add Settings accordion]`. Add Settings summary: 4 params z `·` (`+25 · +10 · Standard · Ash +10`). |

### Ikony

`frontend/public/items/tools/<sub>/` (11 sub-folderów: consumables, grease, misc, multiplayer, perfume, pots, quest, remembrances, runes, sacred_flasks, throwables) **spłaszczone** do `frontend/public/items/tools/`. IconPath strings w `tools.go` zaktualizowane przez ten sam generator (sed regex). Nowy folder `frontend/public/items/info/` zawiera 49 ikon przeniesionych z `key_items/` — dwie ikony brakują na dysku (`message_from_leda.png`, `tower_of_shadow_message.png`) — pre-existing, nie powstały w tym refactorze.

---

## 6. Final counts (per kategoria)

| Kategoria | Total | Sub-grupy ze stanem |
|---|---|---|
| Tools | 328 | Sacred Flasks 54, Consumables 60, Throwing Pots 52, Perfume Arts 6, Throwables 32, Catalyst Tools 1, Grease 33, Misc 11, Quest Tools 0, Golden Runes 32, Remembrances 25, Multiplayer 22 |
| Bolstering Materials | 43 | Flask Enhancers 2, Shadow Realm Blessings 2, Smithing Stones 9, Somberstones 10, Grave Glovewort 10, Ghost Glovewort 10 |
| Key Items | 356 | Active Runes 0, Crystal Tears 13, Containers 6, Inactive Runes/Keys 169, DLC Keys 7, Larval/Deathroot 2, Cookbooks 120, World Maps 24, Spell Scrolls 15 |
| Melee Armaments | 427 | (30 klas — patrz `subcategories.go`) |
| Ranged / Catalysts | 69 | Bows 9, Light Bows 3, Greatbows 5, Crossbows 9, Ballistas 3, Staffs 28, Seals 12 |
| Arrows / Bolts | 64 | Arrows 33, Greatarrows 6, Bolts 20, Greatbolts 5 |
| Shields | 166 | Torches 9, Small 49, Medium 68, Greatshields 40 |
| Info | 87 | Base Letters/Maps 15, DLC Letters/Maps 15, Mechanics/Locations 57 |

Suma: 1540 entries widocznych w `Item Database` z populated SubCategory (porównaj: pre-Phase-1 ~1530, +10 z un-filter cookbooks/whetblades widocznych jako oddzielne wpisy).

---

## 7. Verification

### Automated
- `go test -v ./backend/...` — pass
- `go test -v ./tests/roundtrip_test.go` — PS4/PC round-trip + cross-platform conversion pass (Phase 4)
- `cd frontend && npx tsc --noEmit` — clean
- `make build` — clean

### Manual checklist (Phase 4 — do wykonania w `make dev`)
- Dropdown pokazuje 18 kategorii w kolejności z gry
- Wybór `Tools` pokazuje kolumnę Sub-Category z sub-grupami
- Wybór `Talismans` chowa kolumnę Sub-Category
- Wybór `All Categories` pokazuje main category w kolumnie Sub-Category
- All Categories ładuje się progressywnie (pierwsze itemy <100 ms, scroll działa)
- Top bar Inventory: `[toggle][capacity bar]` jedna linia, `[Cat][badge][Search]` druga
- Top bar Database: `[toggle][Add Settings]` jedna linia, `[Cat][badge][Search]` druga
- Add Settings summary: 4 params (`+25 · +10 · Standard · Ash +10`)
- Larval Tears w Key Items / Larval Tears + Deathroot
- Torches w Shields (top)
- Whetblades w Key Items / Cookbooks (po un-filter)
- Search debounce — brak laggów per-keystroke
- Per-category Owned/Total badge updates correctly per category change

---

## 8. Źródła

### Web
- https://eldenpedia.com/wiki/Inventory — kanonicznych 18 zakładek + kolejność
- https://eldenring.wiki.fextralife.com/Items — top-level items list (per-item breadcrumb tab)
- https://eldenring.wiki.fextralife.com/Equipment+%26+Magic — 30 weapon classes
- https://eldenring.wiki.fextralife.com/Crystal+Tears — Crystal Tears as Key Items
- https://eldenring.wiki.fextralife.com/Multiplayer+Items — Multiplayer breadcrumb
- https://eldenring.wiki.fextralife.com/Remembrances — Remembrances breadcrumb

### Local
- [spec/33 — DB Categorization Audit](33-db-categorization-audit.md) — poprzedni krok (Information tab + Multiplayer/Remembrances/Crystal Tears)
- [spec/34 — Item Caps Enforcement](34-item-caps.md) — vanilla-realistic caps + NG+ scaling (powiązane: scales_with_ng dla Larval Tear, Stonesword Key, Dragon Heart)
- `backend/db/data/subcategories.go` — single source of truth dla nazw sub-cats
- `backend/db/data/*_subcat.go` — per-category init() classifiers

---

## 9. Future work

1. **Active vs Inactive Great Runes split** — DB obecnie nie rozróżnia (oba dzielą ID). Wymagałoby active-flag (event_flag 1xxx aktywacji) i podziału na dwie sub-grupy w UI. Currently sub-grupa "Active Great Runes" jest pusta.
2. **Quest Tools sub-grupa** — currently pusta; quest items rozproszone po `info.go` (Notes, Letters) i `key_items.go` (story keys/tokens). Re-audit może je tu skonsolidować, jeśli matche in-game `Quest Tools` (do weryfikacji w-game).
3. **Bell Bearings widocznosc w Item Database** — aktualnie filtered out (decyzja user 2026-04-28). Jeśli pojawi się use case (np. seed save bez dedykowanego BB UI), un-filter analogicznie do Cookbooks/Whetblades.
4. **Best-effort Melee placements** — `melee_subcat.go` używa curated lookup + suffix fallback. Niektóre z 427 broni mogą trafiać do złej klasy (np. exotic DLC weapons z nietypową nazwą). Verification odbędzie się dopiero w `make dev` (Phase 4) z porównaniem do gry. Każdy report użytkownika → patch w `melee_subcat.go`.
5. **2 brakujące ikony info** — `message_from_leda.png`, `tower_of_shadow_message.png` (DLC) nie istnieją na dysku (pre-existing). Wymagają manual artwork drop-in lub Fextralife CDN download.
