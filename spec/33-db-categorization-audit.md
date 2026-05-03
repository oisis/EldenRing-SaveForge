# 33 — DB Categorization Audit (Information Tab + Multiplayer/Remembrances/Crystal Tears Reclassification)

> **Type**: Design doc  
> **Scope**: Audit and migration of items in `backend/db/data/*.go` for per-tab game categorization. Creation of new `info` category (Information), reorganization of `tools.go`, `key_items.go`, `crafting_materials.go`. Audit of `cut_content` / `ban_risk` flags.

> **Status**: ✅ Deployed on `feat/db-info-category` (Apr 2026).
>
> **Extended by**: [spec/36 — Inventory Categories: Game-Accurate Order &
> Sub-Grouping](36-inventory-categories-game-order.md). Spec/33 established the source-of-truth
> (Fextralife per-item) and extracted the Information tab; spec/36 completes the work
> by defining 18 main tabs in game order, mapping sub-groups, and moving the last
> misclassifications (Larval Tears, Torches, Region Maps, Golden Runes,
> Whetblades/Cookbooks visibility).

---

## Goal

Current item grouping in our DB did not match the in-game inventory tabs 1:1. Fixes:

- **Information tab in-game** had no equivalent in DB. About tutorials were in `key_items.go`, Letters/Maps/Notes in `tools.go` or `key_items.go`.
- **`tools.go` was a catch-all** — contained Multiplayer Items, Remembrances, Crystal Tears, Keys, Scrolls, Materials. All of these have separate sub-categories in-game.
- **`key_items.go` contained Multiplayer Items and Remembrances** even though the game shows them in the Items tab under sub-categories.

---

## Source-of-truth philosophy

After two iterations, the first audit version (based on `er-save-manager/Goods/*.txt`) proved unreliable — *user verified in-game that Volcano Manor / Patches / Bernahl Letters are on the Information tab, despite er-save-manager classifying them in `KeyItems.txt`*.

**New source ranking:**

1. **In-game observation** (user on PC with make dev / Steam Deck) — authoritative.
2. **Fextralife per-item page breadcrumb** (e.g. *Equipment & Magic / Items / Multiplayer Items*) — authoritative.
3. **Fextralife master lists** (Info Items, Multiplayer Items, Crystal Tears) — supplementary, may diverge from per-item pages.
4. **er-save-manager `Goods/*.txt`** — initial hint, requires cross-check (NotesPaintings.txt is their subjective grouping, not a game tab).
5. **ER-Save-Editor (Rust) `db/item_name.rs`** — names + IDs only, no per-tab categorization.

---

## DB category → in-game tab mapping

| `Category` in DB | In-game tab | Notes |
|---|---|---|
| `weapons` / `ranged_and_catalysts` / `shields` / `arrows_and_bolts` | Equipment > Weapons | OK, clean |
| `head` / `chest` / `arms` / `legs` | Equipment > Armor | OK, clean |
| `talismans` | Equipment > Talismans | OK |
| `sorceries` / `incantations` | Equipment > Spells | OK |
| `gestures` | Gestures menu (separate) | OK |
| `ashes` | Items > Spirit Ashes | OK |
| `ashes_of_war` (`aows.go`) | Items > Ashes of War | OK |
| `crafting_materials` | Items > Materials > Crafting | OK |
| `bolstering_materials` | Items > Materials > Upgrade (Smithing/Somber Stones, Runes) | Project decision: Golden/Hero/Lord/Numen/Lands Between Runes + Rune Arc here, despite er-save-manager having them in Consumables. Defensible. |
| `key_items` | Key Items tab (with Crystal Tears sub-tab) | Post-audit: contains Crystal Tears (sub-tab), Cookbooks/Bell Bearings/Whetblades (filtered out via `Is*ItemID()`) |
| `tools` | Items tab (Consumables, Multiplayer, Remembrances, Throwables, Pots, Greases, Perfumes, Flasks etc.) | Catch-all for Items. Contains 13 Multiplayer Items + 25 Remembrances + 27 Flasks (full) + 27 Flasks (empty) + remaining consumables |
| `info` (new) | Information tab (About tutorials, Letters, Notes, Paintings, Maps, Cross messages) | 114 entries |

---

## Migrations performed

### 1. Extraction of Information tab (105 entries)

**From `key_items.go` → `info.go`** (52):
- 35 About* tutorials (`0x4000238C` — `0x400023B4`, plus `0x400023EB` cut)
- 7 Paintings (`0x40002008` — `0x4000200E`)
- 1 Map: Dragonbarrow (`0x400021A2`)
- 2 DLC About (`0x401EA848`, `0x401EA84A`)
- 7 DLC info messages: Castle Cross, Ancient Ruins Cross, Monk's Missive, Storehouse Cross, Torn Diary, Message from Leda, Tower of Shadow

**From `tools.go` → `info.go`** (53 + 6 from second iteration):
- 1 About the Map (`0x40002393`) — was in tools due to IconPath `tools/quest/`
- 19 base region maps (`0x40002198` — `0x400021AA` minus Dragonbarrow)
- 5 DLC region maps (`0x401EA618` — `0x401EA61C`)
- 6 Letters: Volcano Manor, Patches, Bernahl, Burial Crow's, Zorayas's, Rogier's
- 17 Notes (`0x4000222E` — `0x4000223D` + `0x4000220A` Miquella's Needle + `0x4000220C` Lord of Frenzied Flame)
- 1 DLC Letter for Freyja (`0x401EA3CF`)
- 2 DLC Notes (`0x401EA3D9` Furnace Keeper's, `0x401EA443` Sealed Spiritsprings)
- 3 DLC Paintings (`0x401EA488` — `0x401EA48A`)
- **Second iteration after user feedback** (6 added — user noticed missing items): Irina's Letter (`0x40001FC3`), Red Letter (`0x40001FC5`), Cross Map (`0x401EA3C7`), 3× Ruins Map (`0x401EA3D0` / `D1` / `D2`)

### 2. Reclassification `tools.go` → `key_items.go` (31 entries)

**Crystal Tears** (11) — Fextralife: *"Crystal Tears in Elden Ring are Key Items that can be mixed in the Flask of Wondrous Physick."*: Speckled Hardtear, Crimson/Opaline Bubbletear, Opaline/Leaden Hardtear, Crimsonwhorl Bubbletear, Cerulean Hidden Tear + 4 DLC (Viridian Hidden, Crimsonburst Dried, Oil-Soaked, Deflecting Hardtear).

**Keys & Scrolls** (13): Stonesword Key, Rusty Key, Drawing-Room Key, Imbued Sword Key, Royal House Scroll, Well Depths Key (DLC), Gaol Upper/Lower Level Key (DLC), Storeroom Key (DLC), Secret Rite Scroll (DLC), Keep Wall Key (DLC, cut_content + ban_risk preserved), Prayer Room Key (DLC), Academy Glintstone Key.

**Items with IconPath `key_items/`** (7): Whetstone Knife, Glintstone Whetblade, Conspectus Scroll, Academy Scroll, Margit's Shackle, Mohg's Shackle, Pureblood Knight's Medal. 5 of 7 already had IconPath pointing to `items/key_items/` — `tools.go` simply had the wrong category.

### 3. Reclassification `tools.go` → `crafting_materials.go` (5 entries)

Golden Centipede, Sanctuary Stone, Glintstone Firefly, Volcanic Stone, Gravel Stone — all crafting materials per Fextralife per-item pages.

### 4. Reclassification `key_items.go` → `tools.go` (38 entries)

**Multiplayer Items** (13) — Fextralife breadcrumb *Equipment & Magic / Items / Multiplayer Items*: Bloody Finger, Tarnished's/Phantom Bloody Finger, Tarnished's/Phantom Recusant Finger, Recusant Finger, Festering Bloody Finger, Tarnished's Wizened Finger, Tarnished's/Duelist's Furled Finger, Igon's Furled Finger (DLC), Furlcalling Finger Remedy, Small Golden/Red Effigy, Taunter's Tongue.

**Remembrances** (25, 15 base + 10 DLC) — Fextralife breadcrumb *Equipment & Magic / Items / Remembrances*. All boss remembrances for trading with Finger Reader Enia and Twin Maiden Husks.

---

## `cut_content` / `ban_risk` flag audit

Three entries were flagged as `cut_content + ban_risk`. Web research verified:

| ID | Item | Verdict | Action |
|---|---|---|---|
| `0x400023EB` | About Multiplayer | ✅ Cut (Fextralife: "Unavailable" + spawned has `[ERROR]` prefix) | Keep `cut_content + ban_risk`. Information item. |
| `0x400023A7` | About Monument Icon | ❌ NOT cut (was reachable v1.0 disc, broken in patch 1.06) | Drop `cut_content`, keep `ban_risk` (EAC doesn't whitelist versions). Comment explains. |
| `0x4000229D` | Erdtree Codex | ✅ Cut (Fextralife/Fandom/GameRant unanimous) | Keep `cut_content + ban_risk`. **Key Item, NOT Information** — stays in `key_items.go`. |
| `0x40001FF5` | Burial Crow's Letter | ⚠️ Fextralife says cut, user sees in-game | Keep `cut_content + ban_risk`. Information item per user observation. |
| `0x401EA3CF` | Letter for Freyja | ⚠️ Master list: Info, per-item: Key Item | Information per master list. TODO comment to verify in-game. |

---

## Items intentionally NOT moved

| Item | In-game tab | Decision | Reason |
|---|---|---|---|
| Empty Flasks (Cerulean/Crimson/Wondrous Physick) | Items > Consumables (Flasks) | Remain in `tools.go` | Fextralife: "Items / Consumables" = our tools (catch-all consumables). |
| Dragon Heart (`0x4000274C`) | Key Items | Remains in `key_items.go` | Fextralife explicitly: *"Dragon Heart is a Key Item"*. (er-save-manager has it in UpgradeMaterials.txt — Fextralife trumps.) |
| 5 borderline pot/bottle/kit (Cracked Pot, Ritual Pot, Hefty Cracked Pot, Perfume Bottle, Crafting Kit) | Multiple (Key Item + Crafting Material + Keepsake) | Remain in `key_items.go` | Fextralife explicitly for Cracked Pot: *"Key Item, Crafting Material, and optional Keepsake"* — functionally 3 tabs. |
| Cookbooks (`IsCookbookItemID`), Bell Bearings (`IsBellBearingItemID`), Whetblades (`IsWhetbladeItemID`) | Key Items / Bell Bearings sub / Whetblades sub | Remain in `key_items.go` | Project decision — data home. Filtered out from dropdown via `Is*ItemID()` in `db.go::GetItemsByCategory("key_items")`. |

---

## Discrepancies requiring in-game verification (TODO)

| ID | Item | Fextralife master | Fextralife per-item | Decision |
|---|---|---|---|---|
| `0x4000219E` | Meeting Place Map | Information | Key Item | Left in `tools.go` as Map (= info via current categorization). User did not verify. |
| `0x400021D4` | Mirage Riddle | Information | Key Item | Left in `info.go` (decimal 8660 is in er-save-manager NotesPaintings.txt). |
| `0x401EA3CF` | Letter for Freyja | Information | Key Item | Left in `info.go` per master list. TODO verify. |

---

## Counts (final)

| File | Before | After | Δ |
|---|---|---|---|
| `info.go` (new) | — | 114 | +114 |
| `key_items.go` | 396 | 343 | -53 |
| `tools.go` | 364 | 315 | -49 |
| `crafting_materials.go` | 80 | 85 | +5 |

Total: 1,840 → 1,857 entries (+17 net — a few added during merge).

---

## Known issues

### Prayer Room Key icon

**Problem**: `frontend/public/items/tools/quest/prayer_room_key.png` is **binary identical** to `frontend/public/items/gestures/prayer.png` (both 8762 bytes, identical bits). Someone substituted the prayer gesture icon instead of the real key.

**Fix attempts**: Downloading from Fextralife/Fandom CDN via `curl` returned a 27-byte text "Source image is unreachable" — CDNs block direct download. Manual download from a browser will be needed.

**Workaround**: Code has a `// TODO icon:` comment at the entry. User will verify and manually replace the file.

---

## Sources

### Web (authoritative for per-item tab)

- https://eldenring.wiki.fextralife.com/Crystal+Tears — *"Crystal Tears in Elden Ring are Key Items that can be mixed in the Flask of Wondrous Physick."*
- https://eldenring.wiki.fextralife.com/Multiplayer+Items — breadcrumb *Items / Multiplayer Items*.
- https://eldenring.wiki.fextralife.com/Remembrance+of+the+Grafted — breadcrumb *Items / Remembrances*.
- https://eldenring.wiki.fextralife.com/About+Multiplayer — Unavailable, `[ERROR]` prefix when spawned.
- https://eldenring.wiki.fextralife.com/About+Monument+Icon — *"It can no longer be triggered in version 1.6 [sic, 1.06]"*.
- https://eldenring.wiki.fextralife.com/Erdtree+Codex — *"Indication points to this being cut content"*.
- https://eldenring.wiki.fextralife.com/Cracked+Pot — *"Cracked Pot is a Key Item, Crafting Material, and optional Keepsake."*
- https://eldenring.wiki.fextralife.com/Dragon+Heart — *"Dragon Heart is a Key Item."*
- https://eldenring.wiki.fextralife.com/Info+Items — master list 100+ entries.
- https://gamerant.com/elden-ring-cut-content/ — Erdtree Codex listed.
- https://kotaku.com/elden-ring-patch-1-06-fia-underwear-item-ban-fromsoft-1849389818 — illegal-item warnings.
- https://www.svg.com/961328/elden-rings-new-illegal-item-warnings-explained/ — same.

### Local

- `backend/db/data/info.go` (new), `key_items.go`, `tools.go`, `crafting_materials.go`, `db.go`
- `frontend/src/components/CategorySelect.tsx`, `DatabaseTab.tsx`
- Verification: `tmp/repos/er-save-manager/.../Goods/*.txt` (used as initial hint, not authoritative).

---

## Future work

1. **Empty Flasks** — may be worth extracting to a separate `flasks.go` and UI sub-category "Flasks" (Fextralife explicitly has "Flasks" as a sub-tab in Items). Currently remain in `tools.go` per user decision.
2. **Crystal Tears subcategory** — currently under `key_items` category. If a separate filter in the dropdown is ever needed, can be extracted to `crystal_tears.go`. 41 entries (11 from tools + 30 already in key_items).
3. **Prayer Room Key icon fix** — manual artwork drop-in.
4. **Tools.go cleanup** — still a catch-all (~315 entries: pots, throwables, perfumes, greases, multiplayer, remembrances, flasks). May be worth sub-folders in DB per Items sub-tab.
5. **5 borderline items in-game verification** — Cracked Pot / Ritual Pot / Hefty Cracked Pot / Perfume Bottle / Crafting Kit. Fextralife says multiple tabs, in-game verification will settle it.
