# 36 — Inventory Categories and Game Order

> **Type**: Design doc + canonical reference
> **Status**: ✅ Active — canonical reference for the editor's category taxonomy.
> **Scope**: Mapping items to in-game tabs (18 DB categories), handle prefix classes (5 binary classes), grouping in SortOrderTab (6 tabs), sub-groups (76 constants), the `dlc` flag mechanism, and "unknown category" semantics.

---

## Chapter goal

This chapter describes how the editor classifies items and how that classification is used in the UI (DatabaseTab dropdown, InventoryTab, SortOrderTab) and in the backend (`GetItemsByCategory`, Sort Order, Add Items).

Key questions the chapter answers:

- What is the **source of truth** for category names and their in-game order?
- How does the handle prefix (`0x80`/`0x90`/`0xA0`/`0xB0`/`0xC0`) relate to `ItemData.Category`?
- How do the 18 DB categories map to the 6 `SortOrderTab` tabs?
- What happens to an item with no category (`Category == ""`) or with a DLC handle that has no mapping in the DB?
- How are DLC items recognised?

Per-category sort mechanics (acquisition stride-2) — canonically in [52](52-acquisition-sort-stride2.md). Add Items vs DB category — in [43](43-transactional-item-adding.md). Transfer and workspace UI — in [53](53-inventory-storage-transfer.md).

---

## Status

| Component | Status |
|---|---|
| 18 DB categories (game order) | ✅ Canonical in `frontend/src/components/CategorySelect.tsx:10-29` |
| Handle prefix classes (5 groups) | ✅ Canonical in `backend/core/structures.go:20-24` + `backend/db/db.go:572` |
| Sub-categories (76 constants) | ✅ Canonical in `backend/db/data/subcategories.go` |
| `SortOrderTab` tabs (6) → DB categories (9) | ✅ Canonical in `app_inventory_order.go:32-39` |
| DLC flag mechanism (`Flags: []string{"dlc"}`) | ✅ Active in `backend/db/data/types.go:7` + per-item entries |
| Final counts per category (snapshot 2026-04) | `needs verification` — numbers may differ from current DB state |
| Game order in-game verification | ✅ April 2026 (Steam Deck per CHANGELOG); future FromSoft patches — `needs verification` |

---

## Source of truth in code

| Topic | File / function |
|---|---|
| Handle prefix → handle class | `backend/db/db.go:572` — `GetItemCategoryFromHandle(handle uint32) string` (Weapon/Armor/Talisman/Item/Ash of War/Unknown) |
| Handle ↔ DB item ID bit-swap | `backend/db/db.go:594` — `HandleToItemID`; `:615` — `ItemIDToHandlePrefix` |
| Handle type constants | `backend/core/structures.go:20-24` — `ItemTypeWeapon=0x80000000`, `ItemTypeArmor=0x90000000`, `ItemTypeAccessory=0xA0000000`, `ItemTypeItem=0xB0000000`, `ItemTypeAow=0xC0000000` |
| `ItemData` shape | `backend/db/data/types.go:16` — `ItemData{Name, Category, SubCategory, IconPath, Flags []string, …}` |
| `ItemEntry` shape (DB→frontend) | `backend/db/db.go:23` — `ItemEntry{ID, Name, Category, SubCategory, …}` |
| 18 DB categories (game order) | `frontend/src/components/CategorySelect.tsx:10-29` — `GAME_CATEGORIES`, exported via `CATEGORY_VALUES` |
| Category dispatch (backend) | `backend/db/db.go:636` — `GetItemsByCategory(category, platform string)` with 18-case switch |
| Sub-categories | `backend/db/data/subcategories.go` — 76 `Subcat*` const |
| Sub-classifiers | `backend/db/data/melee_subcat.go`, `key_items_subcat.go`, `info_subcat.go`, `ranged_and_catalysts_subcat.go`, `shields_subcat.go` |
| Tools inline SubCategory | `backend/db/data/tools.go` — all entries with `SubCategory: SubcatXxx` inline |
| SortOrderTab tab → DB categories | `app_inventory_order.go:32-39` — `inventoryOrderTabs` (backend), `frontend/src/components/SortOrderTab.tsx:47-54` — `TAB_CATEGORIES` (frontend mirror) |
| Unarmed exclusion | `app_inventory_order.go:54-58` — `invUnarmedBaseID = 0x0001ADB0`, `isWeaponOrderTechnical` |
| Public list endpoints | `app.go:273` — `GetItemList(category)`; `:283` — `GetItemListChunk` (alias for progressive load) |
| Infusion support per category | `app.go:316` — `weaponCategorySupportsInfusion` (only `melee_armaments` and `shields`) |
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
                           │  → ItemData lookup in backend/db/data/   │
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

The editor uses **two independent classification layers**:

| Layer | Group count | Source | Use |
|---|---|---|---|
| Handle prefix class | 5 + Unknown | `GetItemCategoryFromHandle` (operates on `handle & 0xF0000000`) | Transfer (instance-move vs quantity-merge), GaItem allocation, equipped guard, prefix routing in `HandleToItemID` |
| DB category | 18 | `ItemData.Category` in `backend/db/data/*.go` | UI dropdown, SortOrderTab grouping, `GetItemsByCategory` dispatch, infusion eligibility |

Many DB categories map onto **one** handle prefix:

| Handle prefix | Handle class | DB categories |
|---|---|---|
| `0x80000000` | Weapon | `melee_armaments`, `ranged_and_catalysts`, `shields` |
| `0x90000000` | Armor | `head`, `chest`, `arms`, `legs` |
| `0xA0000000` | Talisman | `talismans` |
| `0xB0000000` | Item | `tools`, `key_items`, `info`, `crafting_materials`, `bolstering_materials`, `arrows_and_bolts`, `sorceries`, `incantations`, `ashes` |
| `0xC0000000` | Ash of War | `ashes_of_war` |

The bit-swap between the handle prefix and the DB item ID prefix is described in [03](03-gaitem-map.md); the functions `HandleToItemID` / `ItemIDToHandlePrefix` perform this conversion.

---

## Handle prefix classes

Full prefix semantics — [03](03-gaitem-map.md). From the perspective of 36 only two consequences matter:

1. **Routing in transfer** ([53](53-inventory-storage-transfer.md)): a handle with prefix `0x80/0x90/0xA0/0xC0` → instance-move; `0xB0` → quantity-merge cap-aware.
2. **Item ID lookup**: `HandleToItemID(handle)` produces a DB-compatible ID that can be used in `GetItemData(id)` / `GetItemDataFuzzy(id)` → `ItemData` → `ItemData.Category`.

`GetItemCategoryFromHandle` returns a one-line string (`"Weapon"`, `"Armor"`, `"Talisman"`, `"Item"`, `"Ash of War"`, `"Unknown"`) — intended for logging / debug. It is **NOT** the same value as `ItemData.Category` (`"melee_armaments"` etc.); the two should not be mixed.

---

## DB item categories

### Naming rules (enforced)

- `Ashes` (not "Spirit Ashes")
- `Info` (not "Information")
- `/` as a separator in multi-part names (not `&`) — e.g. `Ranged Weapons / Catalysts`, `Arrows / Bolts`

Display labels are in `CategorySelect.tsx` (frontend); DB string keys are in the `db.go::GetItemsByCategory` switch (backend).

### Canonical list of 18 tabs (game order)

| # | Tab in game | DB `category` | Sub-groups? (count, classifier) |
|---|---|---|---|
| 1 | Tools | `tools` | yes (13, inline in `tools.go`) |
| 2 | Ashes | `ashes` | no |
| 3 | Crafting Materials | `crafting_materials` | no |
| 4 | Bolstering Materials | `bolstering_materials` | yes (6, manual) |
| 5 | Key Items | `key_items` | yes (9, `key_items_subcat.go`) |
| 6 | Sorceries | `sorceries` | no |
| 7 | Incantations | `incantations` | no |
| 8 | Ashes of War | `ashes_of_war` | no |
| 9 | Melee Armaments | `melee_armaments` | yes (30, `melee_subcat.go`) |
| 10 | Ranged Weapons / Catalysts | `ranged_and_catalysts` | yes (7, `ranged_and_catalysts_subcat.go`) |
| 11 | Arrows / Bolts | `arrows_and_bolts` | yes (4) |
| 12 | Shields | `shields` | yes (4, `shields_subcat.go`) |
| 13 | Head | `head` | no |
| 14 | Chest | `chest` | no |
| 15 | Arms | `arms` | no |
| 16 | Legs | `legs` | no |
| 17 | Talismans | `talismans` | no |
| 18 | Info | `info` | yes (3, `info_subcat.go`) |

The backend `GetItemsByCategory` **additionally** has dispatch for `gestures` (outside the top-level 18 inventory tabs; a separate in-game menu) and the value `"all"` → `GetAllItems`.

### Sub-categories layer (76 constants)

`backend/db/data/subcategories.go` contains 76 `SubcatXxx string` constants (verified). 8 of the 18 tabs have sub-groups (sum 13+6+9+30+7+4+4+3 = 76):

| Tab | Sub-groups | Classifier |
|---|---|---|
| Tools | Sacred Flasks, Consumables, Throwing Pots, Perfume Arts, Throwables, Catalyst Tools, Grease, Reusable Tools, Misc, Quest Tools, Golden Runes, Remembrances, Multiplayer | inline `SubCategory: SubcatXxx` in `tools.go` |
| Bolstering Materials | Flask Enhancers, Shadow Realm Blessings (DLC), Smithing Stones, Somberstones, Grave Glovewort, Ghost Glovewort | manual entries in `bolstering_materials.go` |
| Key Items | Active Great Runes, Crystal Tears, Containers + Slot Upgrades, Inactive Great Runes + Keys + Medallions, DLC Keys, Larval Tears + Deathroot + Lost AoW, Cookbooks, World Maps, Sorcery/Incantation Scrolls | `key_items_subcat.go` (curated ID set + name patterns) |
| Melee Armaments | 30 weapon classes (Daggers, Throwing Blades (DLC), Straight Swords, …, Beast Claws (DLC), Colossal Weapons, Perfume Bottles (DLC)) | `melee_subcat.go` (curated lookup + suffix fallback, strip infusion prefixes) |
| Ranged / Catalysts | Bows, Light Bows, Greatbows, Crossbows, Ballistas, Glintstone Staffs, Sacred Seals | `ranged_and_catalysts_subcat.go` (suffix-based) |
| Arrows / Bolts | Arrows, Greatarrows, Bolts, Greatbolts | per-item classifier in `arrows_and_bolts.go` |
| Shields | Torches, Small Shields, Medium Shields, Greatshields | `shields_subcat.go` (name-based curated) |
| Info | Letters / Maps / Paintings (base), Letters / Maps / Paintings (DLC), Mechanics / Locations Info | `info_subcat.go` (prefix `"About "`/`"Note: "`, flag `dlc`) |

The exact item lists per sub-group live **in code**, not in the doc — `backend/db/data/*_subcat.go` is the source-of-truth.

### Per-category filtering in `GetItemsByCategory`

- `melee_armaments` / `ranged_and_catalysts` / `shields` → `filterInfuseVariants(items)` (`db.go:988`) removes duplicates with an infusion offset.
- `ashes` → filter " +N" suffixed variants (returns only base +0; every upgrade level is a separate entry in `data.StandardAshes`).
- `tools` → filter Whetblades (`IsWhetbladeItemID`) + filter upgraded Flask variants (`"Flask of"` + `" +"`).
- `key_items` → filter Bell Bearings (`IsBellBearingItemID`) + Cookbooks (`IsCookbookItemID`) + `Flags` containing `"no_database"`.
- `ashes_of_war` → `AoWCompatBitmask` enrichment from `globalItemIndex`.
- `arrows_and_bolts`, `crafting_materials`, `bolstering_materials`, `info`, `sorceries`, `incantations`, `head`/`chest`/`arms`/`legs`/`talismans`, `gestures` → no special filters (only skip for `Name == ""`).

---

## SortOrderTab category grouping

`SortOrderTab` (the two-column Storage|Inventory view — see [53](53-inventory-storage-transfer.md)) uses **6 tabs** that map to 9 of the 18 DB categories:

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

### What does NOT appear in SortOrderTab

9 of the 18 DB categories **have no equivalent** in any `SortOrderTab` tab:

- `tools`, `ashes`, `crafting_materials`, `bolstering_materials`, `key_items`, `sorceries`, `incantations`, `ashes_of_war`, `arrows_and_bolts`, `info`.

These categories are visible only in `DatabaseTab` (Item Database — dropdown of 18 tabs) and `InventoryTab` (the legacy list view). A workspace session in `SortOrderTab` filters by `TAB_CATEGORIES[tab].has(it.category)` — items from other DB categories are in the workspace snapshot, but do not appear in the grid of any `SortOrderTab` tab.

### Unarmed exclusion

The `weapons` tab in `SortOrderTab` additionally excludes the Unarmed placeholder (`invUnarmedBaseID = 0x0001ADB0`):

- Backend: `isWeaponOrderTechnical(name, baseID)` in `app_inventory_order.go:56-58` — used by `GetInventoryOrder`/`GetStorageOrder`/`ReorderInventory`/`ReorderStorage` to skip the 3 technical Unarmed slots.
- Frontend: `tabFilter` in `SortOrderTab.tsx:74-78` — `if (tab === 'weapons' && it.baseItemID === UNARMED_BASE_ID) return false`.

The game keeps exactly 3 Unarmed entries as the fallback "empty hand" state; they should not be visible in the sort UI nor subject to reorder.

---

## Inventory vs Storage category behavior

Both Inventory and Storage use **the same** classification rules (`TAB_CATEGORIES` in `SortOrderTab` filters both sides via the same `tabFilter`; the backend `GetInventoryOrder` and `GetStorageOrder` in `app_inventory_order.go` operate on identical mapping).

**needs verification**: whether the in-game Storage menu really renders identical tabs to Inventory (e.g. whether Storage filters anything different than equipped items). From the editor perspective — symmetric.

---

## Game order vs app order vs acquisition order

**Three independent orderings** coexist in the system:

| Ordering | Scope | Source | Use |
|---|---|---|---|
| **Game (category) order** | 18 tabs — `tools` first, `info` last | `CategorySelect.tsx:10-29` | Display in DatabaseTab dropdown; tab order in SortOrderTab (limited to 6) |
| **App (sort) order** | within one tab — alphabetical by `Name`, after enrichment | `db.go::GetItemsByCategory` sorts the result per category before caching | Per-category display in DatabaseTab/InventoryTab |
| **Acquisition order** | within `slot.Inventory.CommonItems` / `slot.Storage.CommonItems` — per-record `Index` (stride-2 base ≥ `InvEquipReservedMax+2`) | `app_inventory_order.go::ReorderInventory`/`ReorderStorage`; workspace save in `backend/editor/save.go::writeContainerLayout` | Item sorting within a tab in-game; view in SortOrderTab |

Acquisition order is **independent** of category — it operates on the position within a single container. The stride-2 algorithm and bucket-collision guard — canonically in [52](52-acquisition-sort-stride2.md).

---

## DLC flag mechanism

The editor **does not** use a separate DB category for DLC. DLC content is mixed within the existing 18 tabs; it is recognised by a flag:

```go
// backend/db/data/types.go:5-15 (docstring above ItemData)
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

DLC entries in `data.*` carry e.g. `Flags: []string{"dlc"}` (see `melee_armaments.go` — many entries with `Flags: []string{"dlc"}`). Sub-classifiers (`melee_subcat.go`, `info_subcat.go` …) use the `"dlc"` flag to assign a sub-group whose name contains the `(DLC)` suffix (e.g. `Backhand Blades (DLC)`, `Throwing Blades (DLC)`, DLC Keys, Shadow Realm Blessings (DLC)).

In addition to the docstring values, the code also uses the `"no_database"` flag (filter in `GetItemsByCategory` for `key_items` — `db.go:758`); this flag is not listed in the `types.go` docstring but is active in the current code.

`GetItemsByCategory` **does not filter** by the `"dlc"` flag — DLC entries are returned together with base. Sub-groups are the only mechanism of separation in the UI.

**needs verification**: completeness of DLC sub-mapping — whether every DLC item in the DB has an assigned sub-group, whether there are DLC items falling into a catch-all sub-group / "Other". In `melee_subcat.go` the curated lookup may miss exotic DLC weapons with non-standard names (see "Known limits" section below).

---

## Unknown categories and category gaps

### Items with empty `Category`

- `GetItemsByCategory` filters out entries with `Name == ""` (the most common cause of an empty category — placeholder in `data.*` maps).
- `GetItemSubCategory(id, item, broadCategory)` (`db.go:803`) returns `item.Category` if `!= ""`; otherwise a fallback for broad categories.

### Items with a handle prefix unmapped in the DB

- A save may contain a `GaItem` with handle `0xB0XXXXXX` (or another) whose `HandleToItemID(h)` yields a DB-compatible ID, but that ID **is not** in any `data.*` map (e.g. cut content, DB not updated after a patch).
- In the workspace session model (see [53](53-inventory-storage-transfer.md)) such items land in `EditableItem` with missing DB metadata or in `UnsupportedInventoryRecords` / `UnsupportedStorageRecords` (pass-through layout in `writeContainerLayout`).
- `SortOrderTab.tsx::tabFilter` uses `TAB_CATEGORIES[tab].has(it.category)` — items with an empty or unknown category **do not appear** in any sort tab.
- **needs verification**: exact rendering of such an item in the workspace UI — whether visible in the grid with some fallback, or completely skipped.

### Category gaps and Sort Order / transfer / Add Items

- **Sort Order** ([52](52-acquisition-sort-stride2.md)): `ReorderInventory`/`ReorderStorage` validate handle membership against the category for the chosen tab; handles with an unknown category are rejected as wrong-tab.
- **Transfer** ([53](53-inventory-storage-transfer.md)): `MoveItemsBetweenContainers` operates per handle, **does not** use DB category — the category does not influence whether the transfer succeeds. The workspace path also does not reference the DB category during the wipe-and-replay layout.
- **Add Items** ([43](43-transactional-item-adding.md)): requires knowing the ID in the DB (`GetItemDataFuzzy`); adding an item outside the DB is not possible through `AddItemsToCharacter` from the UI.

---

## Relationship to Sort Order

- SortOrderTab tabs are a **subset** of DB categories (6 of 18). Items from DB categories outside this subset do not appear in the sort grid.
- Stride-2 reorder operates per Sort Order tab (`"weapons"`, `"talismans"`, …), NOT per individual DB category. A weapons reorder mixes `melee_armaments` + `ranged_and_catalysts` + `shields` in a single operation.
- The workspace save (see [53](53-inventory-storage-transfer.md)) writes the layout for **all** items in the container regardless of category; per-tab UI visibility is a separate dimension.

Canonical mechanisms — in [52](52-acquisition-sort-stride2.md) (algorithm) and [53](53-inventory-storage-transfer.md) (UI integration).

---

## Historical notes

The current taxonomy emerged in two phases:

1. **DB Categorization Audit (2025)**: introduction of the `info` category (Information tab), reclassification of Multiplayer Items / Remembrances / Crystal Tears, audit of `cut_content` / `ban_risk` flags. Source-of-truth shifted from `er-save-manager/Goods/*.txt` (community taxonomy) to Fextralife per-item breadcrumb + in-game observation. The full post-mortem is preserved in git history (the former `33-db-categorization-audit.md` from an archive subdirectory, removed in Phase 4+ cleanup).
2. **Phase 36** (April 2026, `feat/inventory-game-accurate-categories`, merged): finalisation of game-order alignment — 18 tabs in game order, sub-groups per tab, minor reclassifications (Larval Tears → Key Items / Larval Tears + Deathroot; Torches → Shields; Region Maps → Key Items / World Maps; Golden Runes → Tools; Perfume Bottles → Melee / Perfume Bottles (DLC); Bastard Sword → Greatswords). File renames: `weapons.go` → `melee_armaments.go`, `aows.go` → `ashes_of_war.go`, `helms.go` → `head.go` (var names `Weapons`, `Aows`, `Helms` kept unchanged).

The current code state is the source-of-truth; the above remains only as context for design decisions.

---

## Test coverage

DB-level tests touching category:

| Class | Test files |
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

**Missing**:
- A dedicated test validating cross-language consistency between `CategorySelect.tsx GAME_CATEGORIES` (18 strings) and the backend `db.go::GetItemsByCategory` switch.
- A test validating that `inventoryOrderTabs` in `app_inventory_order.go` is in sync with `TAB_CATEGORIES` in `SortOrderTab.tsx`.

---

## Known limits / needs verification

- **Final counts per category** (April 2026 snapshot: Tools 328, Bolstering 43, Key Items 356, Melee 427, Ranged/Catalysts 69, Arrows/Bolts 64, Shields 166, Info 87 — total ~1540 entries) — `needs verification`: the current state can be obtained at runtime via `GetItemList(category)` per category; numbers may differ from today's DB.
- **Best-effort Melee sub-classification**: `melee_subcat.go` uses a curated lookup + suffix fallback. Exotic DLC weapons with non-standard names may end up in the wrong class; every user report → patch in `melee_subcat.go`. `needs verification` for completeness of coverage.
- **Unknown category behavior in SortOrderTab**: empirically unconfirmed how the workspace renders an item with handle `0xB0XXXXXX` and an unmapped ID. `needs verification`.
- **Bell Bearings visibility in Item Database**: filtered out of DatabaseTab (decision: the dedicated World UI = single source of truth). If a use-case appears (e.g. seed save) — un-filter analogously to Cookbooks/Whetblades. `needs verification` whether the decision still stands.
- **Active vs Inactive Great Runes split**: the DB does not distinguish; the sub-group "Active Great Runes" in Key Items is empty. It would require integration with `event_flags`. `needs verification` whether implementation is planned.
- **Quest Tools sub-group** in Tools: currently empty. `needs verification` whether the game actually shows this sub-group and which items should go there.
- **2 missing info icons**: `frontend/public/items/info/message_from_leda.png`, `tower_of_shadow_message.png` — do not exist on disk (pre-existing). Require a manual artwork drop-in.
- **Game order in-game verification**: last verification April 2026 (Steam Deck per CHANGELOG `feat(database): 18 in-game category tabs`). FromSoftware patches may reorganise the menu; `needs verification` for the current game version.
- **DLC sub-mapping completeness**: whether every DLC item in the DB has an assigned sub-group. `needs verification`.
- **Storage tab differences vs Inventory**: whether the game renders identical tabs for Storage as for Inventory. `needs verification` on an in-game fixture.

---

## Cross-references

- [03 — GaItem map](03-gaitem-map.md) — handle prefix, `HandleToItemID`, GaMap binary model.
- [07 — Inventory model](07-inventory.md) — read-side 12B record, CommonItems offsets.
- [10 — Storage model](10-storage.md) — read-side record, StorageBoxOffset.
- [35 — GaItem allocator invariants](35-gaitem-allocator-invariants.md) — handle allocation (independent of category).
- [43 — Transactional item adding](43-transactional-item-adding.md) — `AddItemsToCharacter`, per-category validation (e.g. `weaponCategorySupportsInfusion`).
- [52 — Acquisition stride-2 sort order](52-acquisition-sort-stride2.md) — stride-2 reorder per Sort Order tab.
- [53 — Inventory ↔ Storage transfer](53-inventory-storage-transfer.md) — SortOrderTab workspace UI, transfer mechanics.

---

## Sources

- `frontend/src/components/CategorySelect.tsx` — canonical list of 18 tabs in game order.
- `frontend/src/components/SortOrderTab.tsx` — 6 workspace UI tabs + `TAB_CATEGORIES`.
- `app_inventory_order.go` — `inventoryOrderTabs`, `invUnarmedBaseID`, `isWeaponOrderTechnical`.
- `app.go:273+` — `GetItemList`, `GetItemListChunk`, `weaponCategorySupportsInfusion`.
- `backend/db/db.go` — `GetItemsByCategory`, `GetItemCategoryFromHandle`, `HandleToItemID`, `ItemIDToHandlePrefix`, `filterInfuseVariants`, `GetItemSubCategory`, `IsArrowID`.
- `backend/db/data/types.go` — `ItemData` shape + `Flags` semantics.
- `backend/db/data/subcategories.go` — 76 `Subcat*` constants.
- `backend/db/data/*_subcat.go` — per-category classifier (Melee, Key Items, Info, Ranged/Catalysts, Shields).
- `backend/db/data/tools.go` — inline SubCategory for all Tools.
- `backend/core/structures.go:20-24` — `ItemTypeWeapon/Armor/Accessory/Item/Aow` constants.
- `docs/CHANGELOG.md` — entries `feat(database): 18 in-game category tabs` and `feat(db): info category extraction`.
