# 36 — Inventory Categories: Game-Accurate Order & Sub-Grouping

> **Type**: Design doc  
> **Scope**: Full alignment of `Inventory` and `Item Database` tabs with in-game
> equipment layout (1:1 — names, order, sub-groups). Follows
> [spec/33 — DB Categorization Audit](33-db-categorization-audit.md), which
> established the source-of-truth as **Fextralife per-item breadcrumb** + in-game
> verification. Spec/36 completes the work: defines 18 main tabs in game order,
> maps sub-groups (where the game shows them), and refactors backend / frontend /
> icon code to this taxonomy.
>
> **Status**: ✅ Deployed on `feat/inventory-game-accurate-categories`
> (April 2026).
>
> **Source of truth**: in-game UI (PC make dev + Steam Deck verification),
> Fextralife per-item breadcrumb, Eldenpedia inventory tab list.

---

## 1. Philosophy

The current category dropdown in `Item Database` was semi-technical —
some values came from `er-save-manager`'s `Goods/*.txt` (community
taxonomy), others were `.go` file names. A player opening the editor alongside
the game had to translate "Information" to the in-game tab and search for
Larval Tears in `Bolstering Materials` despite the game showing them in
`Key Items`. Spec/36 removes this translation layer: every tab name, every
sub-group, and every weapon class corresponds 1:1 to what the player sees
in the in-game equipment menu.

**Naming rules** (enforced):
- `Ashes` (not "Spirit Ashes")
- `Info` (not "Information")
- `/` as separator (not `&`) — e.g. `Ranged Weapons / Catalysts`, `Arrows / Bolts`

---

## 2. Canonical order of 18 tabs

| # | In-game tab | DB `category` | Sub-groups? |
|---|---|---|---|
| 1 | Tools | `tools` | yes (12) |
| 2 | Ashes | `ashes` | no |
| 3 | Crafting Materials | `crafting_materials` | no |
| 4 | Bolstering Materials | `bolstering_materials` | yes (6) |
| 5 | Key Items | `key_items` | yes (9) |
| 6 | Sorceries | `sorceries` | no |
| 7 | Incantations | `incantations` | no |
| 8 | Ashes of War | `ashes_of_war` | no |
| 9 | Melee Armaments | `melee_armaments` | yes (29) |
| 10 | Ranged Weapons / Catalysts | `ranged_and_catalysts` | yes (7) |
| 11 | Arrows / Bolts | `arrows_and_bolts` | yes (4) |
| 12 | Shields | `shields` | yes (4) |
| 13 | Head | `head` | no |
| 14 | Chest | `chest` | no |
| 15 | Arms | `arms` | no |
| 16 | Legs | `legs` | no |
| 17 | Talismans | `talismans` | no |
| 18 | Info | `info` | yes (3) |

Frontend dropdown (`CategorySelect.tsx`) renders the above order as a
flat list of 18 `<option>` (no `<optgroup>`) — `value` matches the
`DB category` column.

---

## 3. Sub-groups per tab (game order)

### Tools (12 sub-groups)

```
1.  Sacred Flasks, Reusable Tools & FP Regenerators
2.  Consumables
3.  Throwing Pots
4.  Perfume Arts                   ← consumables, NOT Perfume Bottles weapon
5.  Throwables
6.  Catalyst Tools
7.  Grease
8.  Miscellaneous Tools
9.  Quest Tools                    ← currently empty (quest items live in info/key_items)
10. Golden Runes                   ← moved from bolstering_materials
11. Remembrances                   ← moved from key_items (spec/33)
12. Multiplayer Items              ← moved from key_items (spec/33)
```

### Bolstering Materials (6 sub-groups)

```
1. Flask Enhancers                 (Golden Seeds + Sacred Tears)
2. Shadow Realm Blessings (DLC)    (Scadutree Fragment + Revered Spirit Ash)
3. Smithing Stones [1-8] + Ancient Dragon Smithing Stone
4. Somberstones [1-9] + Somber Ancient Dragon Smithing Stone
5. Grave Glovewort [1-9] + Great Grave Glovewort
6. Ghost Glovewort [1-9] + Great Ghost Glovewort
```

### Key Items (9 sub-groups)

```
1. Active Great Runes              ← currently empty (DB doesn't distinguish active/inactive)
2. Crystal Tears
3. Containers + Slot Upgrades      (Empty Cracked/Ritual/Perfume Pots/Bottles, Memory Stones, Talisman Pouches)
4. Inactive Great Runes + Keys + Medallions    ← catch-all (story keys, medallions, quest tokens)
5. DLC Keys
6. Larval Tears + Deathroot + Lost Ashes of War
7. Cookbooks                       (incl. Crafting Kit, Spirit Calling Bell, Whetblades, Sewing Needles)
8. World Maps                      (24 region maps — moved from info.go in spec/36)
9. Sorcery Scrolls + Incantation Scrolls
```

### Melee Armaments (29 weapon classes — base + DLC interleaved)

```
Daggers → Throwing Blades (DLC) → Straight Swords → Light Greatswords (DLC) →
Greatswords → Colossal Swords → Thrusting Swords → Heavy Thrusting Swords →
Curved Swords → Curved Greatswords → Backhand Blades (DLC) → Katanas →
Great Katanas (DLC) → Twinblades → Axes → Greataxes → Hammers → Great Hammers →
Flails → Spears → Great Spears → Halberds → Reapers → Whips → Fists →
Hand-to-Hand (DLC) → Claws → Beast Claws (DLC) → Colossal Weapons →
Perfume Bottles (DLC, weapon)
```

### Ranged Weapons / Catalysts (7 sub-groups)

```
Bows → Light Bows → Greatbows → Crossbows → Ballistas → Glintstone Staffs → Sacred Seals
```

### Arrows / Bolts (4 sub-groups)

```
Arrows → Greatarrows → Bolts → Greatbolts
```

### Shields (4 sub-groups)

```
Torches  ← TOP (moved from melee_armaments)
Small Shields
Medium Shields
Greatshields
```

### Info (3 sub-groups)

```
1. Letters / Maps / Paintings           (base game)
2. Letters / Maps / Paintings (DLC)
3. Mechanics / Locations Info           (About* tutorials + Notes)
```

---

## 4. Reclassifications performed (vs previous state)

| Item / group | From (before) | To (game) | Reason |
|---|---|---|---|
| **Larval Tears** | (already) `key_items.go` | Key Items / Larval Tears + Deathroot + Lost AoW | Plan called for Bolstering, but they were already in Key Items — only SubCategory changed. |
| **Whetblades + Cookbooks** | filtered out from `key_items` | Key Items / Cookbooks (sub) | `IsCookbookItemID` / `IsWhetbladeItemID` filters removed from `db.GetItemsByCategory`. Remain managed from dedicated World UI **and** visible in Item Database. |
| **Bell Bearings** | filtered out from `tools` | (kept filtered) | User decision (option A, 2026-04-28): managed exclusively from dedicated World → Bell Bearings UI (single source of truth — see ROADMAP Phase 4). `IsBellBearingItemID` filter preserved. |
| **Torches** (9) | `melee_armaments` | **Shields** (sub: Torches, top) | Game shows torches in Shields tab. Stray `Torchpole` (`0x00F55C80`) already had `Category: "shields"` but was in `weapons.go` — moved to `shields.go`. |
| **Region Maps** (24) | `info.go` | **`key_items.go`** (sub: World Maps) | Previously duplicated between tools/info/key_items. Game shows them in Key Items. Zero duplication — removed from info.go, added once in key_items.go. |
| **Golden Runes** (33) | `bolstering_materials.go` | **`tools.go`** (sub: Golden Runes) | Game groups all Runes under Tools. User decision (option A, 2026-04-28): runes from bolstering. |
| **Perfume Bottles** (3 weapons) | (Tools, unclear) | **`melee_armaments.go`** (sub: Perfume Bottles, DLC) | DLC weapon class. Confused with Perfume Arts (consumables) — ultimately both groups preserved: Tools/Perfume Arts (6 consumables) + Melee/Perfume Bottles (3 weapons). |
| **Bastard Sword, Bolt of Gransax, Bloody Helice** | catch-all Straight Swords (best-effort initial) | corrected in `melee_subcat.go` curated lookup | Bastard Sword → Greatswords; Bolt of Gransax → Great Spears; Bloody Helice → Heavy Thrusting Swords. Verification passes described in section 7. |

---

## 5. Code architecture

### Backend (`backend/db/data/`)

**Files renamed (Phase 0 — git mv):**
- `weapons.go` → `melee_armaments.go`
- `aows.go` → `ashes_of_war.go`
- `helms.go` → `head.go`

(Var names — `Weapons`, `Aows`, `Helms` — remained unchanged; file rename doesn't require symbol rename to avoid breaking `db.go::GetItemsByCategory`.)

**New files:**
- `subcategories.go` — all 70 `Subcat*` constants in one source of truth.
- `melee_subcat.go` — curated lookup tables for 30 weapon classes + suffix fallback. Strips infusion prefixes (Heavy/Keen/Cold/Sacred/Fire/…) before lookup.
- `key_items_subcat.go` — curated ID set for Crystal Tears, name-pattern for Cookbooks/Containers/Larval/SpellScrolls/DLCKeys, fallback to `Inactive Great Runes + Keys + Medallions` (catch-all for story tokens).
- `info_subcat.go` — rules: prefix `"About "` or `"Note: "` → Mechanics/Locations; `dlc` flag → DLC Letters/Maps; rest → base Letters/Maps/Paintings.
- `ranged_and_catalysts_subcat.go` — suffix-based: Greatbow/Shortbow/Bow → Bows class; Crossbow/Ballista; "Seal"/"Staff" → catalyst.
- `shields_subcat.go` — name-based curated lookup with 4 sets.

**File with inline SubCategory (Phase 2f):**
- `tools.go` — all 328 entries have `SubCategory: SubcatXxx` inline. Generator (`tmp/inline-tools-subcat/main.go`) read populated `data.Tools` after init() and injected field + flattened IconPath. Old `tools_subcat.go` removed.

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

`ItemEntry` returns to frontend with additional field `SubCategory string`. Five
places creating `ItemEntry` in `GetItemsByCategory` were updated to propagate it.

### Backend (`app.go`)

New method:
```go
func (a *App) GetItemListChunk(category string) []db.ItemEntry
```
Returns one tab at a time — frontend `Item Database` in "All Categories" view
calls it 18× in a loop with `await new Promise(r => setTimeout(r, 0))` between
iterations → progressive loading without blocking scroll.

### Frontend

| File | Change |
|---|---|
| `CategorySelect.tsx` | Flat list of 18 options in game order; removed `<optgroup>`. Exports `CATEGORY_VALUES` (used by progressive loader in DatabaseTab). |
| `InventoryTab.tsx` | Top bar `[Cat][Owned/Total badge][Search]`; column `Category` → `Sub-Category` (hidden for tabs without sub-cats); search debounce 200ms via `useDeferredValue`; capacity bar moved to App.tsx. |
| `DatabaseTab.tsx` | Top bar and column same as above; progressive load 18× chunk + thin progress strip above table (pointer-events: none). |
| `App.tsx` | Header Inventory: `[toggle pills][global capacity bar]`. Header Database: `[toggle pills][▶ Add Settings accordion]`. Add Settings summary: 4 params with `·` (`+25 · +10 · Standard · Ash +10`). |

### Icons

`frontend/public/items/tools/<sub>/` (11 sub-folders: consumables, grease, misc, multiplayer, perfume, pots, quest, remembrances, runes, sacred_flasks, throwables) **flattened** to `frontend/public/items/tools/`. IconPath strings in `tools.go` updated by the same generator (sed regex). New folder `frontend/public/items/info/` contains 49 icons moved from `key_items/` — two icons missing on disk (`message_from_leda.png`, `tower_of_shadow_message.png`) — pre-existing, not introduced in this refactor.

---

## 6. Final counts (per category)

| Category | Total | Sub-group breakdown |
|---|---|---|
| Tools | 328 | Sacred Flasks 54, Consumables 60, Throwing Pots 52, Perfume Arts 6, Throwables 32, Catalyst Tools 1, Grease 33, Misc 11, Quest Tools 0, Golden Runes 32, Remembrances 25, Multiplayer 22 |
| Bolstering Materials | 43 | Flask Enhancers 2, Shadow Realm Blessings 2, Smithing Stones 9, Somberstones 10, Grave Glovewort 10, Ghost Glovewort 10 |
| Key Items | 356 | Active Runes 0, Crystal Tears 13, Containers 6, Inactive Runes/Keys 169, DLC Keys 7, Larval/Deathroot 2, Cookbooks 120, World Maps 24, Spell Scrolls 15 |
| Melee Armaments | 427 | (30 classes — see `subcategories.go`) |
| Ranged / Catalysts | 69 | Bows 9, Light Bows 3, Greatbows 5, Crossbows 9, Ballistas 3, Staffs 28, Seals 12 |
| Arrows / Bolts | 64 | Arrows 33, Greatarrows 6, Bolts 20, Greatbolts 5 |
| Shields | 166 | Torches 9, Small 49, Medium 68, Greatshields 40 |
| Info | 87 | Base Letters/Maps 15, DLC Letters/Maps 15, Mechanics/Locations 57 |

Total: 1540 entries visible in `Item Database` with populated SubCategory (compare: pre-Phase-1 ~1530, +10 from un-filtered cookbooks/whetblades visible as separate entries).

---

## 7. Verification

### Automated
- `go test -v ./backend/...` — pass
- `go test -v ./tests/roundtrip_test.go` — PS4/PC round-trip + cross-platform conversion pass (Phase 4)
- `cd frontend && npx tsc --noEmit` — clean
- `make build` — clean

### Manual checklist (Phase 4 — to be performed in `make dev`)
- Dropdown shows 18 categories in game order
- Selecting `Tools` shows Sub-Category column with sub-groups
- Selecting `Talismans` hides Sub-Category column
- Selecting `All Categories` shows main category in Sub-Category column
- All Categories loads progressively (first items <100 ms, scroll works)
- Top bar Inventory: `[toggle][capacity bar]` one line, `[Cat][badge][Search]` second
- Top bar Database: `[toggle][Add Settings]` one line, `[Cat][badge][Search]` second
- Add Settings summary: 4 params (`+25 · +10 · Standard · Ash +10`)
- Larval Tears in Key Items / Larval Tears + Deathroot
- Torches in Shields (top)
- Whetblades in Key Items / Cookbooks (after un-filter)
- Search debounce — no per-keystroke lag
- Per-category Owned/Total badge updates correctly on category change

---

## 8. Sources

### Web
- https://eldenpedia.com/wiki/Inventory — canonical 18 tabs + order
- https://eldenring.wiki.fextralife.com/Items — top-level items list (per-item breadcrumb tab)
- https://eldenring.wiki.fextralife.com/Equipment+%26+Magic — 30 weapon classes
- https://eldenring.wiki.fextralife.com/Crystal+Tears — Crystal Tears as Key Items
- https://eldenring.wiki.fextralife.com/Multiplayer+Items — Multiplayer breadcrumb
- https://eldenring.wiki.fextralife.com/Remembrances — Remembrances breadcrumb

### Local
- [spec/33 — DB Categorization Audit](33-db-categorization-audit.md) — previous step (Information tab + Multiplayer/Remembrances/Crystal Tears)
- [spec/34 — Item Caps Enforcement](34-item-caps.md) — vanilla-realistic caps + NG+ scaling (related: scales_with_ng for Larval Tear, Stonesword Key, Dragon Heart)
- `backend/db/data/subcategories.go` — single source of truth for sub-cat names
- `backend/db/data/*_subcat.go` — per-category init() classifiers

---

## 9. Future work

1. **Active vs Inactive Great Runes split** — DB currently doesn't distinguish (both share IDs). Would require an active-flag (event_flag 1xxx activation) and splitting into two sub-groups in UI. Currently the "Active Great Runes" sub-group is empty.
2. **Quest Tools sub-group** — currently empty; quest items are scattered across `info.go` (Notes, Letters) and `key_items.go` (story keys/tokens). A re-audit could consolidate them here, if they match in-game `Quest Tools` (to verify in-game).
3. **Bell Bearings visibility in Item Database** — currently filtered out (user decision 2026-04-28). If a use case emerges (e.g. seed save without dedicated BB UI), un-filter analogously to Cookbooks/Whetblades.
4. **Best-effort Melee placements** — `melee_subcat.go` uses curated lookup + suffix fallback. Some of the 427 weapons may land in the wrong class (e.g. exotic DLC weapons with unusual names). Verification will happen in `make dev` (Phase 4) with in-game comparison. Each user report → patch in `melee_subcat.go`.
5. **2 missing info icons** — `message_from_leda.png`, `tower_of_shadow_message.png` (DLC) don't exist on disk (pre-existing). Require manual artwork drop-in or Fextralife CDN download.
