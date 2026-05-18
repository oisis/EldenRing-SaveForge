# 36 — Inventory Categories and Game Order

> **Typ**: Design doc + canonical reference
> **Status**: ✅ Active — canonical reference dla taksonomii kategorii w edytorze.
> **Zakres**: Mapowanie itemów na zakładki gry (18 DB categories), klasy handle prefix (5 binary classes), grupowanie w SortOrderTab (6 tabs), sub-grupy (76 stałych), mechanizm flagi `dlc`, oraz semantyka "unknown category".

---

## Cel rozdziału

Rozdział opisuje, jak edytor klasyfikuje itemy i jak ta klasyfikacja jest używana w UI (DatabaseTab dropdown, InventoryTab, SortOrderTab) oraz w backendzie (`GetItemsByCategory`, Sort Order, Add Items).

Kluczowe pytania, na które rozdział odpowiada:

- Jaka jest **źródło prawdy** dla nazw kategorii i ich kolejności w grze?
- Jak handle prefix (`0x80`/`0x90`/`0xA0`/`0xB0`/`0xC0`) ma się do `ItemData.Category`?
- Jak 18 DB categories mapuje się na 6 zakładek `SortOrderTab`?
- Co dzieje się z itemem bez kategorii (`Category == ""`) lub z DLC handle bez mapowania w DB?
- Jak rozpoznawane są DLC itemy?

Mechanika sortowania per kategoria (acquisition stride-2) — kanonicznie w [52](52-acquisition-sort-stride2.md). Add Items vs DB category — w [43](43-transactional-item-adding.md). Transfer i workspace UI — w [53](53-inventory-storage-transfer.md).

---

## Status

| Komponent | Status |
|---|---|
| 18 DB categories (game order) | ✅ Canonical w `frontend/src/components/CategorySelect.tsx:10-29` |
| Handle prefix classes (5 grup) | ✅ Canonical w `backend/core/structures.go:20-24` + `backend/db/db.go:572` |
| Sub-categories (76 stałych) | ✅ Canonical w `backend/db/data/subcategories.go` |
| `SortOrderTab` tabs (6) → DB categories (9) | ✅ Canonical w `app_inventory_order.go:32-39` |
| DLC flag mechanism (`Flags: []string{"dlc"}`) | ✅ Active w `backend/db/data/types.go:7` + per-item entries |
| Final counts per kategoria (snapshot 2026-04) | `needs verification` — liczby mogą się różnić od bieżącego stanu DB |
| Game order in-game verification | ✅ kwiecień 2026 (Steam Deck per CHANGELOG); kolejne patche FromSoft — `needs verification` |

---

## Source of truth w kodzie

| Topic | Plik / funkcja |
|---|---|
| Handle prefix → handle class | `backend/db/db.go:572` — `GetItemCategoryFromHandle(handle uint32) string` (Weapon/Armor/Talisman/Item/Ash of War/Unknown) |
| Handle ↔ DB item ID bit-swap | `backend/db/db.go:594` — `HandleToItemID`; `:615` — `ItemIDToHandlePrefix` |
| Handle type constants | `backend/core/structures.go:20-24` — `ItemTypeWeapon=0x80000000`, `ItemTypeArmor=0x90000000`, `ItemTypeAccessory=0xA0000000`, `ItemTypeItem=0xB0000000`, `ItemTypeAow=0xC0000000` |
| `ItemData` shape | `backend/db/data/types.go:16` — `ItemData{Name, Category, SubCategory, IconPath, Flags []string, …}` |
| `ItemEntry` shape (DB→frontend) | `backend/db/db.go:23` — `ItemEntry{ID, Name, Category, SubCategory, …}` |
| 18 DB categories (game order) | `frontend/src/components/CategorySelect.tsx:10-29` — `GAME_CATEGORIES`, eksportowane przez `CATEGORY_VALUES` |
| Category dispatch (backend) | `backend/db/db.go:636` — `GetItemsByCategory(category, platform string)` z 18-case switch |
| Sub-categories | `backend/db/data/subcategories.go` — 76 `Subcat*` const |
| Sub-classifiers | `backend/db/data/melee_subcat.go`, `key_items_subcat.go`, `info_subcat.go`, `ranged_and_catalysts_subcat.go`, `shields_subcat.go` |
| Tools inline SubCategory | `backend/db/data/tools.go` — wszystkie entries z `SubCategory: SubcatXxx` w-line |
| SortOrderTab tab → DB categories | `app_inventory_order.go:32-39` — `inventoryOrderTabs` (backend), `frontend/src/components/SortOrderTab.tsx:47-54` — `TAB_CATEGORIES` (frontend mirror) |
| Unarmed exclusion | `app_inventory_order.go:54-58` — `invUnarmedBaseID = 0x0001ADB0`, `isWeaponOrderTechnical` |
| Public list endpoints | `app.go:273` — `GetItemList(category)`; `:283` — `GetItemListChunk` (alias dla progressive load) |
| Infusion support per category | `app.go:316` — `weaponCategorySupportsInfusion` (tylko `melee_armaments` i `shields`) |
| Infuse variant filter | `backend/db/db.go:988` — `filterInfuseVariants` |
| Arrow ID detection | `backend/db/db.go:566` — `IsArrowID` |

---

## Mental model

```
                           ┌──────────────────────────────────────────┐
                           │  Save (binary)                           │
                           │  GaItem handle: 0x80…/0x90…/0xA0…/       │
                           │                 0xB0…/0xC0… (5 classes)  │
                           └────────────────┬─────────────────────────┘
                                            │ GetItemCategoryFromHandle
                                            ▼
                           ┌──────────────────────────────────────────┐
                           │  Handle class (5)                        │
                           │  Weapon / Armor / Talisman / Item /      │
                           │  Ash of War / Unknown                    │
                           └────────────────┬─────────────────────────┘
                                            │ HandleToItemID + GaMap
                                            ▼
                           ┌──────────────────────────────────────────┐
                           │  DB item ID prefix (0x00/0x10/0x20/      │
                           │                     0x40/0x80)            │
                           │  → ItemData lookup w backend/db/data/    │
                           └────────────────┬─────────────────────────┘
                                            │ ItemData.Category
                                            ▼
                           ┌──────────────────────────────────────────┐
                           │  DB category (18 tabs, game order)       │
                           │  e.g. melee_armaments / talismans /      │
                           │  tools / info / ashes_of_war …           │
                           └─────┬──────────────────┬─────────────────┘
                                 │                  │
                                 ▼                  ▼
                  ┌────────────────────┐   ┌────────────────────┐
                  │ DatabaseTab        │   │ SortOrderTab       │
                  │ dropdown (18)      │   │ 6 tabs subsetting  │
                  └────────────────────┘   └────────────────────┘
```

---

## Two orthogonal taxonomies

Edytor używa **dwóch niezależnych warstw kategoryzacji**:

| Warstwa | Liczba grup | Źródło | Zastosowanie |
|---|---|---|---|
| Handle prefix class | 5 + Unknown | `GetItemCategoryFromHandle` (operuje na `handle & 0xF0000000`) | Transfer (instance-move vs quantity-merge), GaItem allocation, equipped guard, prefix routing w `HandleToItemID` |
| DB category | 18 | `ItemData.Category` w `backend/db/data/*.go` | UI dropdown, SortOrderTab grouping, `GetItemsByCategory` dispatch, infusion eligibility |

Wiele DB categories mapuje na **jeden** handle prefix:

| Handle prefix | Handle class | DB categories |
|---|---|---|
| `0x80000000` | Weapon | `melee_armaments`, `ranged_and_catalysts`, `shields` |
| `0x90000000` | Armor | `head`, `chest`, `arms`, `legs` |
| `0xA0000000` | Talisman | `talismans` |
| `0xB0000000` | Item | `tools`, `key_items`, `info`, `crafting_materials`, `bolstering_materials`, `arrows_and_bolts`, `sorceries`, `incantations`, `ashes` |
| `0xC0000000` | Ash of War | `ashes_of_war` |

Bit-swap między handle prefix a DB item ID prefix opisany w [03](03-gaitem-map.md); funkcje `HandleToItemID` / `ItemIDToHandlePrefix` realizują tę konwersję.

---

## Handle prefix classes

Pełna semantyka prefiksów — [03](03-gaitem-map.md). Z perspektywy 36 istotne są tylko dwie konsekwencje:

1. **Routing w transferze** ([53](53-inventory-storage-transfer.md)): handle z prefiksem `0x80/0x90/0xA0/0xC0` → instance-move; `0xB0` → quantity-merge cap-aware.
2. **Item ID lookup**: `HandleToItemID(handle)` produkuje DB-zgodny ID, którego można użyć w `GetItemData(id)` / `GetItemDataFuzzy(id)` → `ItemData` → `ItemData.Category`.

`GetItemCategoryFromHandle` zwraca jednolinijkowy string (`"Weapon"`, `"Armor"`, `"Talisman"`, `"Item"`, `"Ash of War"`, `"Unknown"`) — przeznaczony do logging / debug. **NIE** jest to ta sama wartość co `ItemData.Category` (`"melee_armaments"` itd.); nie należy ich mieszać.

---

## DB item categories

### Naming rules (egzekwowane)

- `Ashes` (nie "Spirit Ashes")
- `Info` (nie "Information")
- `/` jako separator w wieloczłonowych nazwach (nie `&`) — np. `Ranged Weapons / Catalysts`, `Arrows / Bolts`

Display labels są w `CategorySelect.tsx` (frontend); DB string keys są w `db.go::GetItemsByCategory` switch (backend).

### Kanoniczna lista 18 zakładek (kolejność w grze)

| # | Tab w grze | DB `category` | Sub-grupy? (count, classifier) |
|---|---|---|---|
| 1 | Tools | `tools` | tak (13, inline w `tools.go`) |
| 2 | Ashes | `ashes` | nie |
| 3 | Crafting Materials | `crafting_materials` | nie |
| 4 | Bolstering Materials | `bolstering_materials` | tak (6, manualne) |
| 5 | Key Items | `key_items` | tak (9, `key_items_subcat.go`) |
| 6 | Sorceries | `sorceries` | nie |
| 7 | Incantations | `incantations` | nie |
| 8 | Ashes of War | `ashes_of_war` | nie |
| 9 | Melee Armaments | `melee_armaments` | tak (30, `melee_subcat.go`) |
| 10 | Ranged Weapons / Catalysts | `ranged_and_catalysts` | tak (7, `ranged_and_catalysts_subcat.go`) |
| 11 | Arrows / Bolts | `arrows_and_bolts` | tak (4) |
| 12 | Shields | `shields` | tak (4, `shields_subcat.go`) |
| 13 | Head | `head` | nie |
| 14 | Chest | `chest` | nie |
| 15 | Arms | `arms` | nie |
| 16 | Legs | `legs` | nie |
| 17 | Talismans | `talismans` | nie |
| 18 | Info | `info` | tak (3, `info_subcat.go`) |

Backend `GetItemsByCategory` ma **dodatkowo** dispatch dla `gestures` (poza top-level 18 zakładek inventory; oddzielne menu w grze) oraz wartość `"all"` → `GetAllItems`.

### Sub-categories layer (76 stałych)

`backend/db/data/subcategories.go` zawiera 76 stałych `SubcatXxx string` (zweryfikowane). 8 z 18 zakładek ma sub-grupy (suma 13+6+9+30+7+4+4+3 = 76):

| Tab | Sub-grupy | Klasyfikator |
|---|---|---|
| Tools | Sacred Flasks, Consumables, Throwing Pots, Perfume Arts, Throwables, Catalyst Tools, Grease, Reusable Tools, Misc, Quest Tools, Golden Runes, Remembrances, Multiplayer | inline `SubCategory: SubcatXxx` w `tools.go` |
| Bolstering Materials | Flask Enhancers, Shadow Realm Blessings (DLC), Smithing Stones, Somberstones, Grave Glovewort, Ghost Glovewort | manualne wpisy w `bolstering_materials.go` |
| Key Items | Active Great Runes, Crystal Tears, Containers + Slot Upgrades, Inactive Great Runes + Keys + Medallions, DLC Keys, Larval Tears + Deathroot + Lost AoW, Cookbooks, World Maps, Sorcery/Incantation Scrolls | `key_items_subcat.go` (curated ID set + name patterns) |
| Melee Armaments | 30 klas broni (Daggers, Throwing Blades (DLC), Straight Swords, …, Beast Claws (DLC), Colossal Weapons, Perfume Bottles (DLC)) | `melee_subcat.go` (curated lookup + suffix fallback, strip infusion prefixes) |
| Ranged / Catalysts | Bows, Light Bows, Greatbows, Crossbows, Ballistas, Glintstone Staffs, Sacred Seals | `ranged_and_catalysts_subcat.go` (suffix-based) |
| Arrows / Bolts | Arrows, Greatarrows, Bolts, Greatbolts | classifier per-item w `arrows_and_bolts.go` |
| Shields | Torches, Small Shields, Medium Shields, Greatshields | `shields_subcat.go` (name-based curated) |
| Info | Letters / Maps / Paintings (base), Letters / Maps / Paintings (DLC), Mechanics / Locations Info | `info_subcat.go` (prefix `"About "`/`"Note: "`, flag `dlc`) |

Dokładne listy itemów per sub-grupa są **w kodzie**, nie w doc — `backend/db/data/*_subcat.go` jest source-of-truth.

### Per-category filtering w `GetItemsByCategory`

- `melee_armaments` / `ranged_and_catalysts` / `shields` → `filterInfuseVariants(items)` (`db.go:988`) usuwa duplikaty z offsetem infuzji.
- `ashes` → filter " +N" suffixed variants (zwraca tylko base +0; każdy upgrade level jest osobnym wpisem w `data.StandardAshes`).
- `tools` → filter Whetblades (`IsWhetbladeItemID`) + filter upgraded Flask variants (`"Flask of"` + `" +"`).
- `key_items` → filter Bell Bearings (`IsBellBearingItemID`) + Cookbooks (`IsCookbookItemID`) + `Flags` zawierający `"no_database"`.
- `ashes_of_war` → enrichment `AoWCompatBitmask` z `globalItemIndex`.
- `arrows_and_bolts`, `crafting_materials`, `bolstering_materials`, `info`, `sorceries`, `incantations`, `head`/`chest`/`arms`/`legs`/`talismans`, `gestures` → bez specjalnych filtrów (tylko skip dla `Name == ""`).

---

## SortOrderTab category grouping

`SortOrderTab` (dwukolumnowy widok Storage|Inventory — zob. [53](53-inventory-storage-transfer.md)) używa **6 zakładek**, które mapują na 9 z 18 DB categories:

```go
// app_inventory_order.go:32-39
var inventoryOrderTabs = map[string][]string{
    "weapons":   {"melee_armaments", "ranged_and_catalysts", "shields"},
    "talismans": {"talismans"},
    "head":      {"head"},
    "chest":     {"chest"},
    "arms":      {"arms"},
    "legs":      {"legs"},
}
```

Frontend mirror: `SortOrderTab.tsx:47-54` (`TAB_CATEGORIES: Record<SortOrderTabKey, ReadonlySet<string>>`).

### Co NIE pojawia się w SortOrderTab

9 z 18 DB categories **nie ma odpowiednika** w żadnej zakładce `SortOrderTab`:

- `tools`, `ashes`, `crafting_materials`, `bolstering_materials`, `key_items`, `sorceries`, `incantations`, `ashes_of_war`, `arrows_and_bolts`, `info`.

Te kategorie są widoczne wyłącznie w `DatabaseTab` (Item Database — dropdown 18 zakładek) i `InventoryTab` (legacy widok listowy). Workspace session w `SortOrderTab` filtruje per `TAB_CATEGORIES[tab].has(it.category)` — itemy z innych DB categories są w workspace snapshot, ale nie pojawiają się w siatce żadnej z 6 zakładek `SortOrderTab`.

### Unarmed exclusion

Zakładka `weapons` w `SortOrderTab` dodatkowo wyklucza placeholder Unarmed (`invUnarmedBaseID = 0x0001ADB0`):

- Backend: `isWeaponOrderTechnical(name, baseID)` w `app_inventory_order.go:56-58` — używane przez `GetInventoryOrder`/`GetStorageOrder`/`ReorderInventory`/`ReorderStorage` do pominięcia 3 technicznych slotów Unarmed.
- Frontend: `tabFilter` w `SortOrderTab.tsx:74-78` — `if (tab === 'weapons' && it.baseItemID === UNARMED_BASE_ID) return false`.

Gra utrzymuje dokładnie 3 wpisy Unarmed jako rezerwowy stan "pusta dłoń"; nie powinny być widoczne w UI sortowania ani podlegać reorder.

---

## Inventory vs Storage category behavior

Zarówno Inventory, jak i Storage używają **tych samych** reguł kategoryzacji (`TAB_CATEGORIES` w `SortOrderTab` filtruje obie strony przez ten sam `tabFilter`; backend `GetInventoryOrder` i `GetStorageOrder` w `app_inventory_order.go` operują na identycznym mapowaniu).

**needs verification**: czy w grze Storage menu rzeczywiście renderuje identyczne zakładki co Inventory (np. czy Storage filtruje cokolwiek innego niż założone itemy). Z perspektywy edytora — symetryczne.

---

## Game order vs app order vs acquisition order

W systemie współistnieją **trzy niezależne porządki**:

| Porządek | Zakres | Źródło | Zastosowanie |
|---|---|---|---|
| **Game (category) order** | 18 zakładek — `tools` na pierwszym miejscu, `info` na ostatnim | `CategorySelect.tsx:10-29` | Display w DatabaseTab dropdown; kolejność tabs w SortOrderTab (ograniczone do 6) |
| **App (sort) order** | wewnątrz jednej zakładki — alfabetyczny po `Name`, po enrichmencie | `db.go::GetItemsByCategory` sortuje wynik per category przed cache'owaniem | Display per kategoria w DatabaseTab/InventoryTab |
| **Acquisition order** | wewnątrz `slot.Inventory.CommonItems` / `slot.Storage.CommonItems` — per-rekord `Index` (stride-2 base ≥ `InvEquipReservedMax+2`) | `app_inventory_order.go::ReorderInventory`/`ReorderStorage`; workspace save w `backend/editor/save.go::writeContainerLayout` | Sortowanie itemów w danej zakładce w grze; widok w SortOrderTab |

Acquisition order jest **niezależny** od category — operuje na pozycji w obrębie jednego kontenera. Stride-2 algorytm i bucket-collision guard — kanonicznie w [52](52-acquisition-sort-stride2.md).

---

## DLC flag mechanism

Edytor **nie** używa osobnej DB category dla DLC. DLC content jest mieszany w obrębie istniejących 18 zakładek; rozpoznawany przez flagę:

```go
// backend/db/data/types.go:5-15 (docstring nad ItemData)
// Flags reference (string set; combine freely):
//   - "stackable"       — item stacks in a single inventory slot (vs. unique drops)
//   - "dlc"             — Shadow of the Erdtree content
//   - "cut_content"     — never shipped legitimately; spawning may flag EAC
//   - "ban_risk"        — adding this item carries elevated EAC ban risk
//   - "scales_with_ng"  — vanilla obtainable count scales linearly with NG+ cycle

type ItemData struct {
    Name        string
    Category    string
    SubCategory string
    Flags       []string
    // …
}
```

Wpisy DLC w `data.*` mają np. `Flags: []string{"dlc"}` (zob. `melee_armaments.go` — wiele wpisów z `Flags: []string{"dlc"}`). Sub-classifier (`melee_subcat.go`, `info_subcat.go` …) używają flag `"dlc"` do przypisania sub-grupy zawierającej w nazwie suffix `(DLC)` (np. `Backhand Blades (DLC)`, `Throwing Blades (DLC)`, DLC Keys, Shadow Realm Blessings (DLC)).

Poza wartościami z docstring kod używa też flagi `"no_database"` (filter w `GetItemsByCategory` dla `key_items` — `db.go:758`); flaga ta nie jest wymieniona w docstring `types.go`, ale jest aktywna w bieżącym kodzie.

`GetItemsByCategory` **nie filtruje** po `"dlc"` flag — DLC entries są zwracane razem z base. Sub-grupy są jedynym mechanizmem separacji w UI.

**needs verification**: kompletność DLC sub-mapping — czy każdy DLC item w DB ma assigned sub-group, czy są DLC items lecące do catch-all sub-grupy / "Other". W `melee_subcat.go` curated lookup może pominąć exotic DLC weapons z nietypową nazwą (sekcja "Known limits" niżej).

---

## Unknown categories and category gaps

### Itemy z pustą `Category`

- `GetItemsByCategory` filtruje wpisy z `Name == ""` (najczęsta przyczyna pustej kategorii — placeholder w `data.*` map).
- `GetItemSubCategory(id, item, broadCategory)` (`db.go:803`) zwraca `item.Category` jeśli `!= ""`; w przeciwnym razie fallback dla broad categories.

### Itemy z handle prefiksem niezmapowanym w DB

- Save może zawierać `GaItem` z handle `0xB0XXXXXX` (lub innym), którego `HandleToItemID(h)` daje DB-zgodny ID, ale tego ID **nie ma** w żadnej `data.*` mapie (np. cut content, niezaktualizowany DB po patchu).
- W workspace session model (zob. [53](53-inventory-storage-transfer.md)) takie itemy trafiają do `EditableItem` z brakującymi metadanymi DB albo do `UnsupportedInventoryRecords` / `UnsupportedStorageRecords` (pass-through layout w `writeContainerLayout`).
- `SortOrderTab.tsx::tabFilter` używa `TAB_CATEGORIES[tab].has(it.category)` — itemy z pustą lub nieznaną kategorią **nie pojawiają się** w żadnej zakładce sortowania.
- **needs verification**: dokładne renderowanie takiego itemu w workspace UI — czy widzialny w siatce z dowolnym fallback, czy całkowicie pominięty.

### Category gaps a Sort Order / transfer / Add Items

- **Sort Order** ([52](52-acquisition-sort-stride2.md)): `ReorderInventory`/`ReorderStorage` walidują przynależność handle do kategorii odpowiedniej dla wybranego taba; handle z nieznaną kategorią są odrzucane jako wrong-tab.
- **Transfer** ([53](53-inventory-storage-transfer.md)): `MoveItemsBetweenContainers` operuje per handle, **nie** używa DB category — kategoria nie wpływa na to, czy transfer się uda. Workspace path także nie odwołuje się do DB category przy wipe-and-replay layout.
- **Add Items** ([43](43-transactional-item-adding.md)): wymaga znajomości ID w DB (`GetItemDataFuzzy`); dodawanie itemu spoza DB nie jest możliwe przez `AddItemsToCharacter` z UI.

---

## Relationship to Sort Order

- SortOrderTab tabs są **podzbiorem** DB categories (6 z 18). Itemy DB category poza tym podzbiorem nie pojawiają się w siatce sortowania.
- Stride-2 reorder operuje per Sort Order tab (`"weapons"`, `"talismans"`, …), NIE per pojedyncza DB category. Reorder broni miesza `melee_armaments` + `ranged_and_catalysts` + `shields` w jednej operacji.
- Workspace save (zob. [53](53-inventory-storage-transfer.md)) pisze layout dla **wszystkich** itemów w kontenerze niezależnie od kategorii; widoczność w UI per zakładka jest osobnym wymiarem.

Kanoniczne mechanizmy — w [52](52-acquisition-sort-stride2.md) (algorytm) i [53](53-inventory-storage-transfer.md) (UI integracja).

---

## Historical notes

Aktualna taksonomia powstała w dwóch fazach:

1. **archive/33** — [DB Categorization Audit](archive/33-db-categorization-audit.md): wprowadzenie kategorii `info` (Information tab), reklasyfikacja Multiplayer Items / Remembrances / Crystal Tears, audyt flag `cut_content` / `ban_risk`. Source-of-truth przesunięty z `er-save-manager/Goods/*.txt` (community taxonomy) na Fextralife per-item breadcrumb + in-game observation.
2. **Phase 36** (kwiecień 2026, `feat/inventory-game-accurate-categories`, zmergowany): finalizacja game-order alignment — 18 zakładek w kolejności gry, sub-grupy per zakładka, drobne reklasyfikacje (Larval Tears → Key Items / Larval Tears + Deathroot; Torches → Shields; Region Maps → Key Items / World Maps; Golden Runes → Tools; Perfume Bottles → Melee / Perfume Bottles (DLC); Bastard Sword → Greatswords). Rename plików: `weapons.go` → `melee_armaments.go`, `aows.go` → `ashes_of_war.go`, `helms.go` → `head.go` (var names `Weapons`, `Aows`, `Helms` zachowane bez zmian).

Szczegóły migracji z fazy 1 — w [archive/33](archive/33-db-categorization-audit.md). Bieżący stan kodu jest source-of-truth; powyższe pozostaje wyłącznie kontekstem decyzji projektowych.

---

## Test coverage

Testy DB-level dotykające category:

| Klasa | Pliki testów |
|---|---|
| Weapon sub-category, somber upgrade, stats | `backend/db/data/phase2b1_weapons_test.go`, `weapons_somber_max_upgrade_test.go`, `shields_somber_test.go`, `weapon_stats_critical_test.go`, `weapon_stats_generated_test.go`, `weapon_stats_passive_effects_test.go` |
| Arrows / Bolts | `backend/db/data/phase2b2_arrows_bolts_test.go` |
| Info notes | `backend/db/data/phase2b3_info_notes_test.go` |
| Black Syrup (single-item regression) | `backend/db/data/phase2b4_black_syrup_test.go` |
| Sort IDs | `backend/db/data/sort_ids_test.go` |
| Weights | `backend/db/data/weights_test.go` |
| Flag detection (bell bearings, container, grace, item, pickup) | `backend/db/data/bell_bearing_flags_test.go`, `container_pickup_flags_test.go`, `container_requirements_test.go`, `grace_companion_flags_test.go`, `item_companion_flags_test.go` |
| Generated text | `backend/db/data/item_text_generated_test.go` |
| Classes (handle prefix routing) | `backend/db/classes_test.go` |

**Brakuje**:
- Dedykowanego testu walidującego cross-language consistency między `CategorySelect.tsx GAME_CATEGORIES` (18 stringów) a backendem `db.go::GetItemsByCategory` switch.
- Testu walidującego, że `inventoryOrderTabs` w `app_inventory_order.go` jest zsynchronizowany z `TAB_CATEGORIES` w `SortOrderTab.tsx`.

---

## Known limits / needs verification

- **Final counts per kategoria** (snapshot kwiecień 2026: Tools 328, Bolstering 43, Key Items 356, Melee 427, Ranged/Catalysts 69, Arrows/Bolts 64, Shields 166, Info 87 — total ~1540 entries) — `needs verification`: aktualny stan można uzyskać przez runtime `GetItemList(category)` per kategoria; liczby mogą się różnić od dzisiejszego DB.
- **Best-effort Melee sub-classification**: `melee_subcat.go` używa curated lookup + suffix fallback. Exotic DLC weapons z nietypową nazwą mogą trafić do złej klasy; każdy report użytkownika → patch w `melee_subcat.go`. `needs verification` dla kompletności pokrycia.
- **Unknown category behavior w SortOrderTab**: empirycznie nie potwierdzone, jak workspace renderuje item z handle `0xB0XXXXXX` i niezmapowanym ID. `needs verification`.
- **Bell Bearings widoczność w Item Database**: filtered out z DatabaseTab (decyzja: dedykowane World UI = single source of truth). Jeśli pojawi się use-case (np. seed save) — un-filter analogicznie do Cookbooks/Whetblades. `needs verification` czy decyzja jest dalej aktualna.
- **Active vs Inactive Great Runes split**: DB nie rozróżnia; sub-grupa "Active Great Runes" w Key Items jest pusta. Wymagałoby integracji z `event_flags`. `needs verification` czy implementacja jest planowana.
- **Quest Tools sub-grupa** w Tools: currently pusta. `needs verification` czy gra rzeczywiście pokazuje tę sub-grupę i które itemy powinny tam trafić.
- **2 brakujące ikony info**: `frontend/public/items/info/message_from_leda.png`, `tower_of_shadow_message.png` — nie istnieją na dysku (pre-existing). Wymagają manual artwork drop-in.
- **Game order in-game verification**: ostatnia weryfikacja kwiecień 2026 (Steam Deck per CHANGELOG `feat(database): 18 in-game category tabs`). Patche FromSoftware mogą reorganizować menu; `needs verification` dla bieżącej wersji gry.
- **DLC sub-mapping completeness**: czy każdy DLC item w DB ma assigned sub-group. `needs verification`.
- **Storage tab differences vs Inventory**: czy gra renderuje identyczne zakładki dla Storage co dla Inventory. `needs verification` na fixturze in-game.

---

## Cross-references

- [03 — GaItem map](03-gaitem-map.md) — handle prefix, `HandleToItemID`, GaMap binary model.
- [07 — Inventory model](07-inventory.md) — read-side rekord 12B, offsety CommonItems.
- [10 — Storage model](10-storage.md) — read-side rekord, StorageBoxOffset.
- [35 — GaItem allocator invariants](35-gaitem-allocator-invariants.md) — alokacja handle (niezależna od category).
- [43 — Transactional item adding](43-transactional-item-adding.md) — `AddItemsToCharacter`, walidacja per category (np. `weaponCategorySupportsInfusion`).
- [52 — Acquisition stride-2 sort order](52-acquisition-sort-stride2.md) — stride-2 reorder per Sort Order tab.
- [53 — Inventory ↔ Storage transfer](53-inventory-storage-transfer.md) — SortOrderTab workspace UI, transfer mechanics.
- [archive/33 — DB Categorization Audit](archive/33-db-categorization-audit.md) — historyczny audit, wprowadzenie kategorii `info`.

---

## Sources

- `frontend/src/components/CategorySelect.tsx` — kanoniczna lista 18 zakładek w kolejności gry.
- `frontend/src/components/SortOrderTab.tsx` — 6 zakładek workspace UI + `TAB_CATEGORIES`.
- `app_inventory_order.go` — `inventoryOrderTabs`, `invUnarmedBaseID`, `isWeaponOrderTechnical`.
- `app.go:273+` — `GetItemList`, `GetItemListChunk`, `weaponCategorySupportsInfusion`.
- `backend/db/db.go` — `GetItemsByCategory`, `GetItemCategoryFromHandle`, `HandleToItemID`, `ItemIDToHandlePrefix`, `filterInfuseVariants`, `GetItemSubCategory`, `IsArrowID`.
- `backend/db/data/types.go` — `ItemData` shape + `Flags` semantyka.
- `backend/db/data/subcategories.go` — 76 stałych `Subcat*`.
- `backend/db/data/*_subcat.go` — per-category classifier (Melee, Key Items, Info, Ranged/Catalysts, Shields).
- `backend/db/data/tools.go` — inline SubCategory dla wszystkich Tools.
- `backend/core/structures.go:20-24` — `ItemTypeWeapon/Armor/Accessory/Item/Aow` konstanty.
- `docs/CHANGELOG.md` — wpisy `feat(database): 18 in-game category tabs` i `feat(db): info category extraction`.
- [archive/33 — DB Categorization Audit](archive/33-db-categorization-audit.md) — wcześniejszy audit jako kontekst historyczny.
