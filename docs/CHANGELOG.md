# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Critical fix — FlushGaItems in-place shift overwriting DLC + Hash regions (game crash)

User-reported game crash (`EXCEPTION_ACCESS_VIOLATION` at `eldenring.exe+0x1EB9989`) after adding many weapons / armor pieces in a single session. Game read DLC entry-flag byte as non-zero (interpreted as "player entered DLC"), tried to load DLC area state with stale prerequisites, dereferenced NULL pointer.

**Root cause** — `backend/core/writer.go::FlushGaItems`:
- When adding non-stackable items (weapons 21B, armor 16B replacing empty 8B slots), GaItems section grew by `delta` bytes.
- FlushGaItems shifted slot.Data right by `delta` via `copy(slot.Data[oldGaLimit+delta:SlotSize], slot.Data[oldGaLimit:SlotSize-delta])`.
- Comment claimed "loses trailing padding bytes at end of slot" — but the tail is **NOT** padding: it contains DLC section (50 B at `SlotSize-0xB2`) and PlayerGameDataHash (128 B at `SlotSize-0x80`).
- Whenever `delta > 0x132` (306 B), DLC + Hash bytes were overwritten by data from preceding sections (often ASCII resource names from EventFlags region — user's debug save had `Ride_Enemy_Attack3018_CMSG` written into DLC offset).
- Phase 1 ROADMAP explicitly required PlayerGameDataHash region preservation verbatim — FlushGaItems silently violated this for every batch with growth.

**Diagnostic helper** — `tmp/scripts/inspect_slot4.go` (deep-dive sanity check on key offsets), `tmp/scripts/find_pattern.go` (locate ASCII pattern occurrences), `tmp/scripts/rebuild_identity_debug.go` (RebuildSlot identity check on debug save). All in `tmp/scripts/` for repro.

**Fix** — replaced `FlushGaItems` in `AddItemsToSlot` Phase 2 with new `RebuildSlotFull` (full from-scratch slot rebuild, no in-place shift):

- **`backend/core/slot_rebuild.go`** — new `RebuildSlotFull(slot)` function (~120 lines):
  - Serializes new GaItems from `slot.GaItems` array
  - Copies pre-regions blob (between GaItems end and UnlockedRegions) verbatim
  - Re-serializes regions via `slot.UnlockedRegions` slice
  - Re-serializes all post-regions typed sections (head/menu/trophy/gaitem/tut/scalars/ef/wgb/pc/sp/nm/trail/hash) from structs parsed at slot.Read() time
  - Copies tail rest verbatim (truncated if needed to fit SlotSize)
  - Pads to SlotSize with zeros
  - **Final step (critical):** copies original DLC section + PlayerGameDataHash bytes verbatim from `slot.Data` to fixed end-of-slot positions. Typed `trail.Read`/`hash.Read` parse bytes at a position INSIDE the typed-section span (around SlotSize - 419 KB), but the game reads DLC at `SlotSize-0xB2` and Hash at `SlotSize-0x80` — both inside the tailRest region. When mutation grew GaItems, tailRest was trimmed from end, dropping original DLC+Hash. Explicit verbatim copy restores them.
- **`backend/core/writer.go::AddItemsToSlot` Phase 2** — replaced `FlushGaItems(slot)` call with `RebuildSlotFull` + state refresh:
  - Snapshot `slot.GaMap` + `NextAoWIndex` / `NextArmamentIndex` / `NextGaItemHandle` before rebuild (parseFromData rebuilds GaMap from GaItem records only, dropping stackable handles which aren't backed by GaItem records; tracked indices are advanced past rescanned values from Phase 1 `allocateGaItem` mutations)
  - `copy(slot.Data, rebuilt)` — replace slot bytes
  - `slot.parseFromData()` — re-parse all derived state (MagicOffset re-located via FindPattern, dynamic offsets recomputed, inventory counter offsets refreshed for Phase 3)
  - Re-merge stackable handles dropped by GaMap rebuild
  - Restore tracked indices that are higher than rescanned values
- **`backend/core/structures.go`** — extracted `parseFromData()` from `Read()`. `Read()` now does ReadBytes + parseFromData. parseFromData mirrors original Read() body 1:1 but operates on existing `s.Data` buffer (no ReadBytes).
- **`backend/core/writer.go::FlushGaItems`** — marked `Deprecated:` in doc comment. Kept for backward compatibility with external callers; no in-tree callers remain after AddItemsToSlot migration.

**New tests** in `backend/core/slot_rebuild_test.go`:
- `TestRebuildSlotFullIdentityPC` / `TestRebuildSlotFullIdentityPS4` — RebuildSlotFull on unmutated slot must produce byte-identical output (sanity check + invariant)
- `TestAddItemsPreservesDLCAndHash` — load PC save, snapshot DLC + Hash, add 30 weapons (force GaItems growth ~390 B = above 0x132 threshold), verify DLC bytes at `[SlotSize-0xB2..SlotSize-0x80)` and Hash bytes at `[SlotSize-0x80..SlotSize)` are unchanged. Direct regression for the reported crash.

**Test results**: full `go test ./backend/...` passes. `go test ./tests/...` has 3 pre-existing failures (`TestMassiveAddAllCategories`, `TestMaxCapacityFill`, `TestBulkAddPerCategory`) that try to add more items than fit in the 5120-entry GaItems array — these failed identically on master before the fix and are not regressed. `TestArrowsRustCompatibility` is pre-existing flaky (Go map iteration randomness in test assertion — fails ~40% on both master and current branch). Wails app builds cleanly via `make build`.

**Recovery for users with corrupted saves**: saves edited with a pre-fix build that crashed in-game cannot be fully repaired — the original DLC + Hash bytes were overwritten in slot.Data and cannot be reconstructed. Restore from a backup (`*.sl2.YYYYMMDD_HHMMSS.bkp` files in the save directory, created automatically by the editor before each save) made BEFORE heavy weapon/armor additions.

**Known related concern**: `RebuildSlot` (the existing function for unlocked-regions mutations) had the same latent bug for region growth — it never showed in tests because TestRebuildSlotMutationPC checks Player.Level / Souls but not DLC / Hash. The RebuildSlot path is now also patched (DLC + Hash verbatim restore at end). Region-only mutations will benefit equally.

### Container enforcement for Throwing Pots & Aromatics

- **New `backend/db/data/container_requirements.go`**: maps each gated craftable (24 Cracked Pot pots + 12 Ritual Pot pots + 15 Hefty Cracked Pot pots + 7 Aromatics) to its container key item (`Cracked Pot 0x4000251C` cap 20, `Ritual Pot 0x4000251D` cap 10, `Hefty Cracked Pot 0x401EA99C` cap 10, `Perfume Bottle 0x40002526` cap 10). Pure helper `ApplyContainerCap` performs partial-add bookkeeping for unit-testability.
- **Cap semantics**: each unit of pot/aromatic in inventory consumes one container slot. Cap is on TOTAL UNITS across all stacks per container (NOT distinct types). Example: Cracked Pot cap 20 → max 20 individual pot units in inventory in any combination of pot types.
- **`AddItemsToCharacter` signature change** (`app.go`): now returns `(skipped []SkippedAdd, err error)`. Each `SkippedAdd{ItemID, CutQty}` reports an item whose requested inventory qty was trimmed because the container's total-quantity cap was reached. Storage adds are unaffected (storage has no cap).
- **FCFS-by-ID iteration**: gated items in the batch are processed in ascending item-ID order (Fire Pot 0x4000012C wins Cracked Pot cap, Redmane Fire Pot wins Ritual, etc.) regardless of frontend sort. Non-gated items keep their original order. This makes `Add All Tools / Max` predictable: canonical first-of-group always wins the inventory slot.
- **Container key items auto-added** to Key Items with qty = total pots in inventory after the batch (never lowered if existing qty is higher).
- **Pickup flag gating (ban-proof)** — new `backend/db/data/container_pickup_flags.go`: maps each container to its 10–20 world-pickup event flags (Cracked Pot 66000–66190, Ritual 66400–66490, Perfume Bottle 66700–66790, Hefty Cracked Pot 66900–66990, source er-save-manager `event_flags_db.py`). After auto-updating container key item qty to N, `AddItemsToCharacter` flips flags `1..N` to TRUE so the game won't offer those world pickups again — eliminates the duplicate-stack-via-pickup attack surface that anti-cheat could flag as inconsistent state.
- **Vendor purchase flag gating** — same file adds `ContainerVendorPurchaseFlags` map. Vanilla vendors track stock via separate flags from world pickups; the only container sold by a merchant in vanilla is Cracked Pot at Kale (Church of Elleh, Limgrave) — flag `710580` "Purchasing Cracked Pot" (source er-save-manager quest_flags_db.py). After pickup flags, `AddItemsToCharacter` flips vendor flags too so Kale stops listing Cracked Pot for the current NG cycle. Note: NG+ resets these flags (vanilla behavior, beyond editor reach).
- **`DatabaseTab.tsx`**: handles new return type, sums `cutQty` across the batch, surfaces a toast like "Cut N unit(s) across M item(s) from Inventory — container cap reached. Storage unaffected."
- **`tests/`**: 13 unit tests covering map consistency, container-key existence/caps, first-add fits, partial-cut across batch (user point #3: 2×12 → cut 4), independent caps per container, SET-semantic merge, non-gated pass-through, no-op delta, pickup-flag count match (one per pickup), step-10 spacing, no-overlap between containers, full container coverage, Kale Cracked Pot vendor flag (710580), and absence of vendor flags for the three container types vanilla doesn't sell (`container_requirements_test.go` + `container_pickup_flags_test.go`).
- **Pre-fixes bundled**:
  - `tools.go:200` Perfumed Oil of Ranah `MaxInventory` 10 → 1 (per user / Fextralife: it's a single-shot perfume art, not stackable like Spark / Bloodboil).
  - `melee_armaments.go` + `melee_subcat.go`: added missing **Lightning Perfume Bottle** (`0x03AADF90`, DLC weapon, cap 1/1) — 5th Perfume Bottle weapon, was missing from DB despite presence in er-save-manager `AllWeapons.txt:2921`.

### Item Database fixes (post spec/36)

- **DatabaseTab "Add Selected" modal — respect per-item caps in mixed selections** (`frontend/src/components/DatabaseTab.tsx`):
  - Replaced `Math.min(...)` aggregations (`modalMaxInv`/`modalMaxStorage`) with `*Hi` = `Math.max(...)` and `modalAny*Allowed` = `some(>0)` semantics.
  - Storage row no longer disabled wholesale when *one* selected item has `MaxStorage=0`; gate now keys off `modalAnyStorageAllowed`. Backend (`resolveQty`) already skips items with cap 0 per-item.
  - "Max" checkbox visible whenever the highest cap in the group exceeds 1 (previously hidden when min cap was 1, e.g. mixing Glovewort + Remembrance).
  - `openConfirmModal` enables Storage by default if any selected item allows it (was: only when none had cap 0).
  - Numerical input upper bound + clamp now follows `*Hi` cap. Mixed-cap banner copy reworded: "backend applies each item's own vanilla cap".
  - Single-item flow unchanged.
- **Spectral Steed Whistle — wrong hex** (`backend/db/data/tools.go`, `descriptions.go`):
  - Was `0x400000B5` (item ID 181 — duplicate sitting in the Multiplayer items block 170–184; in-game render mapped to the wrong icon).
  - Fixed to `0x40000082` (item ID 130 — canonical entry, paired with Phantom Great Rune 135). Cross-checked against er-save-manager `AllGoods.txt` and ER-Save-Editor `item_name.rs` — both reference projects list both IDs but only 130 is the actual fabular Whistle from Melina.
- **Lulling Branch caps** (`tools.go`): MaxInventory 99 → 10 (storage 600 unchanged) per user audit.
- **Cured Meat caps** (`tools.go`): all 8 entries (Immunizing/Clarifying/Dappled/Invigorating + 4 White variants) 99/600 → 10/999 per user audit.
- **Pickled / Fowl / Turtle caps** (`tools.go`): Silver-Pickled Fowl Foot, Gold-Pickled Fowl Foot, Pickled Turtle Neck, Well-Pickled Turtle Neck (DLC) — all 99/600 → 10/999 per user audit.
- **Deathsbane White Jerky** (`tools.go`): added `cut_content` + `ban_risk` flags (parity with Deathsbane Jerky `0x400004C4`).
- **Flesh / Meat caps** (`tools.go`): Exalted Flesh, Dragon Communion Flesh (DLC), Dragonscale Flesh (DLC), Sacred Bloody Flesh (DLC) — 99/600 → 10/999 per user audit. Innard Meat (DLC) kept at 99/600.
- **Horn Tender caps** (`tools.go`): Silver Horn Tender (DLC), Golden Horn Tender (DLC) — 99/600 → 10/999 per user audit (DLC odpowiednik Pickled Fowl Foot, rune + item discovery booster).
- **Scorpion Stew DLC variants TODO** (`tools.go`, `ROADMAP.md`): er-save-manager DLCConsumables.txt lists 4 IDs (2001200..2001203) with duplicate names; we currently expose only `0x401E8932` and `0x401E8933`. Missing IDs flagged for in-game verification: `0x401E8934`, `0x401E8935`.
- **Innard Meat reclassified** (`tools.go`): moved from `SubcatToolsConsumables` to `SubcatToolsThrowables` (cap 40/600). User verified in-game position: it's a throwable bait in Tools/Throwables sub-tab alongside Bone Darts. er-save-manager DLCConsumables.txt was misleading (file mixes throwables with edibles). My earlier "cut content" diagnosis was wrong — fixed.
- **Throwing Pots reorganized by container** (`tools.go`): grouped per crafting container (Cracked Pot / Ritual Pot / Hefty Cracked Pot) with section comments. Source: erdb 1.10.0 `EquipParamGoods.csv` field `potGroupId` (1=Cracked, 3=Ritual). DLC pots classified per Fextralife per-item recipe pages.
- **Ancient Dragonbolt Pot reclassified** (`tools.go`): moved from `SubcatToolsConsumables` to `SubcatToolsThrowingPots`. erdb potGroupId=3 (Ritual Pot recipe) confirmed by user — it's a Ritual Pot variant, not a generic consumable.
- **Ritual Pot caps audit** (`tools.go`): all 12 Ritual Pot recipe items (Redmane Fire / Giantsflame Fire / Ancient Dragonbolt / Sacred Order / Freezing / Alluring / Beastlure / Albinauric / Cursed-Blood / Academy Magic / Rot + DLC Eternal Sleep) — MaxInventory 20 → 10 per user audit (matches in-game cap).

### Branch: feat/inventory-game-accurate-categories — 1:1 game-aligned Inventory & Item Database

**Goal:** Drop the last technical/community-taxonomy seams between the editor's category dropdown and the in-game inventory layout. After spec/33 introduced the `Information` tab and migrated Multiplayer/Remembrances/Crystal Tears, the dropdown was still partly Eldenring.wiki-flavoured (no sub-groups, alphabetical-ish order, "Information" instead of "Info", `&` instead of `/`). Phase 36 finishes the alignment: 18 tabs in exact in-game order, sub-grouping per tab where the game has them, reclassifications driven by Fextralife per-item breadcrumb + in-game user verification.

**Why:**
- Player toggling between editor and game shouldn't translate names — `Ashes` not "Spirit Ashes", `Info` not "Information", `/` not `&`.
- Larval Tears living under Bolstering Materials was historical; the game shows them in Key Items / Larval Tears + Deathroot. Torches under Melee Armaments was wrong — game shows them in Shields. Region Maps were duplicated across `info.go` / `tools.go` / `key_items.go`. Golden Runes were under Bolstering but the game lists them in Tools.
- "All Categories" view in Item Database loaded ~1530 items synchronously, blocking the UI thread for ~600 ms on Steam Deck. Progressive 18× chunked load makes first items visible <100 ms with non-blocking scroll.

**Backend (`backend/db/data/`):**
- `types.go` — added `SubCategory string` field to `ItemData`.
- `subcategories.go` (new) — single source of truth for ~70 sub-category constants across 8 tabs that have sub-grouping (Tools 12, Bolstering 6, Key Items 9, Melee 30, Ranged 7, Arrows/Bolts 4, Shields 4, Info 3).
- `weapons.go` → `melee_armaments.go`, `aows.go` → `ashes_of_war.go`, `helms.go` → `head.go` (file renames only — var symbols stay to avoid `db.go` cascade).
- `melee_subcat.go` (new, 408 lines) — curated lookup tables for 30 weapon classes + suffix fallback. Strips infusion prefixes (Heavy/Keen/Cold/Sacred/Fire/…) before lookup. Bastard Sword → Greatswords, Bolt of Gransax → Great Spears, Bloody Helice → Heavy Thrusting Swords (corrected from initial Straight Swords catch-all).
- `key_items_subcat.go` (new) — Crystal Tears curated ID set; Cookbook/Container/Larval/SpellScroll/DLCKey name patterns; fallback to "Inactive Great Runes + Keys + Medallions".
- `info_subcat.go` (new) — `"About "` prefix + `"Note: "` → Mechanics/Locations; `dlc` flag → DLC Letters/Maps; else base Letters/Maps/Paintings.
- `ranged_and_catalysts_subcat.go` (new) — suffix-based (Greatbow/Bow/Crossbow/Ballista/Staff/Seal).
- `shields_subcat.go` (new) — name-based curated lookup, 4 sets.
- `tools.go` — all 328 entries inline `SubCategory: SubcatXxx` (Phase 2f generator inlined and flattened IconPath in single pass; old `tools_subcat.go` removed).
- `bolstering_materials.go` — rewritten with 6 inline sub-groups; **33 Golden Runes moved to `tools.go`** (sub: Golden Runes — game-accurate placement).
- `arrows_and_bolts.go` — rewritten with 4 inline sub-groups (Arrows 33, Greatarrows 6, Bolts 20, Greatbolts 5). Fixed Igon's Harpoon mislabel (was Greatarrows, is Bolt).
- `key_items.go` — added 24 Region Maps (sub: World Maps — moved from `info.go`, no duplication). Surface Cookbooks + Whetblades by removing `IsCookbookItemID` / `IsWhetbladeItemID` filters from `db.GetItemsByCategory`. Bell Bearings stay filtered (managed exclusively from World → Bell Bearings UI per user decision 2026-04-28).
- `info.go` — 24 Region Maps removed (relocated to key_items). IconPath updated `items/key_items/X.png` → `items/info/X.png` for the 49 info icons that were actually info, not key items.
- `shields.go` — added stray Torchpole `0x00F55C80` (was in `weapons.go` despite `Category: "shields"`).

**Frontend:**
- `CategorySelect.tsx` — flat 18 `<option>` (no `<optgroup>`) in canonical in-game order. Exports `CATEGORY_VALUES` for the progressive loader. Display labels match game UI: `Ashes` (not "Spirit Ashes"), `Info` (not "Information"), `/` separator (not `&`).
- `InventoryTab.tsx` — top bar restructure `[Cat][Owned/Total badge][Search]`; `Category` column → `Sub-Category` (hidden for tabs without sub-cats: Talismans/Sorceries/Incantations/Ashes/Crafting/Head/Chest/Arms/Legs/AshesOfWar; main category shown as fallback in `'all'` view); 200 ms search debounce via `useDeferredValue`. Capacity bar lifted out (now in App.tsx).
- `DatabaseTab.tsx` — same top bar + column rules as InventoryTab. Progressive load: replace single `GetItemList('all')` with 18× `GetItemListChunk(cat)` loop; thin progress strip above table (`pointer-events: none` — scroll works during load).
- `App.tsx` — Inventory header: `[toggle pills][global capacity bar]` one row. Database header: `[toggle pills][▶ Add Settings accordion]` one row. Add Settings summary now shows 4 params (`+25 · +10 · Standard · Ash +10`, was 3 missing Infuse).

**Backend API:**
- `app.go` — new `GetItemListChunk(category string) []db.ItemEntry`. Wails bindings regenerated.
- `db.go` — `ItemEntry` now carries `SubCategory string` (propagated through `GetItemsByCategory`).
- `vm/character_vm.go` — `ItemViewModel` adds `SubGroup string` field (true sub-cat). `Category`/`SubCategory` semantics on the VM left intact (broad type / main tab) to avoid touching every consumer.

**Icons:**
- `frontend/public/items/tools/<11 sub-folders>/` — flattened to `frontend/public/items/tools/`. IconPath strings in `tools.go` updated by the same Phase 2f generator (sed regex). One conflict resolved (`celestial_dew.png` existed in both `tools/` and `tools/misc/` with identical MD5 — kept root, removed `misc/` copy).
- `frontend/public/items/info/` (new) — 49 info icons moved from `key_items/`. 2 pre-existing absent on disk (`message_from_leda.png`, `tower_of_shadow_message.png` — DLC; need manual Fextralife CDN drop-in).

**Counts (per tab, after refactor):**
| Tab | Count |
|---|---|
| Tools | 328 |
| Bolstering Materials | 43 |
| Key Items | 356 |
| Melee Armaments | 427 (30 weapon classes) |
| Ranged / Catalysts | 69 (7 sub-groups) |
| Arrows / Bolts | 64 |
| Shields | 166 (Torches 9, Small 49, Medium 68, Greatshields 40) |
| Info | 87 (Base 15, DLC 15, Mechanics 57) |

**Verification:**
- `go test -v ./backend/...` — pass
- `cd frontend && npx tsc --noEmit && npm run build` — pass
- Manual verification in `make dev` pending — Phase 4 checklist in `spec/36`.

**Source of truth:** Eldenpedia inventory tab list (canonical 18 tabs) + Fextralife per-item breadcrumb + in-game observation (PC make dev + Steam Deck verification). er-save-manager `Goods/*.txt` continues to be used as initial hint only — its taxonomy is community, not 1:1 with game UI.

**Spec doc:** [spec/36-inventory-categories-game-order.md](spec/36-inventory-categories-game-order.md) (extends spec/33).

**Known limitations / future work:**
- Active vs Inactive Great Runes — DB doesn't distinguish; sub-group "Active Great Runes" currently empty.
- Quest Tools sub-group empty — quest items live in `info.go` (Notes/Letters) and `key_items.go` (story keys).
- Best-effort Melee placements — 427 weapons through curated lookup + suffix fallback; some exotic DLC weapons may land in wrong class. Verification pending in `make dev` (Phase 4) — user reports drive `melee_subcat.go` patches.

---

### Branch: feat/inventory-caps-enforcement — vanilla-realistic item caps + NG+ scaling + Full Chaos Mode

**Goal:** Reduce EAC ban risk by tightening `MaxInventory` / `MaxStorage` caps for items where the previous numbers (e.g. 999/999 cookbooks, 99/600 paintings) were anomalously above what a single playthrough can yield. Caps now reflect Fextralife-verified single-playthrough obtainable counts. For items that scale per NG+ cycle, a new `scales_with_ng` flag drives `effective_cap = base × (ClearCount + 1)` so caps grow naturally on NG+1...NG+7. A new **Full Chaos Mode** Settings toggle bypasses all caps for power users (max 999), with a clearly red-flagged ban-risk warning.

**Why:**
- **Ban risk reduction** — EAC pattern detection flags statistically anomalous quantities. 22 Dragon Hearts looks vanilla; 999 does not.
- **In-game realism** — caps now match what the player can actually collect, making the editor a transparent "vanilla-aligned" tool rather than a chaos generator.
- **No data loss for existing saves** — clamp is enforced **only at user-add** (the Database tab modal); legacy save loads/writes are untouched. `vm/character_vm.go` clamp behavior intentionally not changed.

**Backend data (`backend/db/data/`):**
- `types.go` — documented new flag set including `scales_with_ng`.
- `info.go` — 29 paintings/notes 99/600 → 1/0, removed `stackable`.
- `key_items.go` — 11 Crystal Tears 99/600 → 1/0; **109 cookbooks** (Greater Potentate's, Glintstone Craftsman's, Perfumer's) varied caps → 1/0; **Stonesword Key** 99/600 → 55/0 + scales_with_ng; **Dragon Heart** 99/600 → 22/0 + scales_with_ng; **Larval Tear** 99/600 → 24/0 + scales_with_ng.
- `bolstering_materials.go` — **Golden Seed** 30/0 + scales_with_ng (DLC adds 0); **Sacred Tear** 12/0 + scales_with_ng (capped at flask potency limit, world surplus is unusable); **Scadutree Fragment** 999/999 → 50/0 + scales_with_ng; **Revered Spirit Ash** 99/600 → 25/0 + scales_with_ng.
- **Mohg's Great Rune** moved `bolstering_materials.go` → `key_items.go` (correct in-game tab).

**NG+ scaling formula:**
```
effective_cap = MaxInventory × (ClearCount + 1)
```
where `ClearCount` is the save's NG+ cycle counter (0 = NG, 1 = NG+1, …, 7 = NG+7), already exposed via `CharacterViewModel.clearCount`.

**Frontend:**
- `DatabaseTab.tsx` — `effectiveCap()` helper computes per-item cap with chaos override + scales_with_ng multiplier; modal min/max inputs use effective cap; new banner shows *"NG+ Scaling · Vanilla NG: X · NG+Y: Z · Adding more increases EAC ban risk"* when any selected item has the flag, and a red **Full Chaos Mode** banner when bypass is active. Listens for `'fullChaosModeChanged'` window CustomEvent.
- `SettingsTab.tsx` — new **Full Chaos Mode** toggle in Safety section (red-bordered, dangerous) with explicit ban-risk copy. Persists to `localStorage:setting:fullChaosMode` and dispatches `'fullChaosModeChanged'` event for cross-component sync.

**Counts source:** Fextralife wiki per-item Locations sections, cross-checked with Game8 / PowerPyx / GamesRadar for Scadutree Fragment, Revered Spirit Ash, Stonesword Key, Dragon Heart, Larval Tear, Golden Seed, Sacred Tear (all v1.16 patch).

**Verification:**
- `go test ./backend/...` — pass
- `npx tsc --noEmit && npm run build` — pass
- Manual: Database modal pokazuje cap 55 dla Stonesword Key na NG, 110 na NG+1, 999 z chaos mode. Cookbooks pokazują 1. Paintings/notes pokazują 1.

**Round 2 fixes (po user testach):**
- **Bug:** Information items (paintings/notes/letters) były dodawane do save ale niewidoczne w UI inventory — `data.Information` mapy brakowało w `globalItemIndex` (`backend/db/db.go:168-176`), więc `GetItemDataFuzzy` zwracał empty i `mapItems` skipował. **Fix:** dodano `data.Information` do listy maps w `init()`.
- **Bug:** Dragon Heart, Larval Tear, Crystal Tears nie były widoczne w "All" view zakładki Inventory — pre-existing filtr `(item.subCategory !== 'key_items' || item.readOnly)` w `InventoryTab.tsx:281` ukrywał editable key_items, co miało sens dla story keys ale nie dla nowych usable item types. **Fix:** filtr w 'all' view uproszczony do samego `matchesSearch`.
- **Bug:** Storage row był aktywny dla itemów z MaxStorage=0 (paintings/cookbooks/etc.), pozwalając użytkownikowi odhaczyć "dodaj do storage" co było silentnie ignorowane przez backend resolveQty. **Fix:** modal Storage row teraz `pointer-events-none + opacity-40` i pokazuje *"Not allowed"* gdy `modalMaxStorage===0`; `openConfirmModal` automatycznie ustawia `addToStorage=false` jeśli którykolwiek z wybranych itemów ma cap storage 0.
- **Bug:** Scadutree Fragment skalował z NG+ (50 → 100 na NG+1) mimo że nadwyżka jest bezużyteczna (50 = max blessing cumulative cost). **Fix:** zdjęto `scales_with_ng` z 4 itemów których cap to "max useful" zamiast "world count": Golden Seed (30 = full flask), Sacred Tear (12 = full potency), Scadutree Fragment (50 = max blessing), Revered Spirit Ash (25 = max ash blessing). Scaling pozostaje na: Stonesword Key, Dragon Heart, Larval Tear (consumable z otwartym usage).

---

### Branch: feat/db-info-category — extract Information tab + DB-wide categorization audit

**Goal:** Make the Database tab's category dropdown match the actual in-game inventory tabs. Previously `key_items.go` and `tools.go` were used as catch-all dumping grounds — items that the game shows on the **Information** tab (Polish: Informacje), under **Items > Multiplayer Items**, **Items > Remembrances**, etc. were scattered across both. Now each item lives in the file matching its real game tab.

**Why:**
The user, while exploring the Database tab, kept finding items in the wrong dropdown bucket: 38 "About *" tutorials sitting in Key Items but actually shown on Information; Letters (Volcano Manor, Patches, Bernahl, etc.) sitting in Tools but shown on Information; Bloody Finger / Tarnished's Furled Finger / Remembrances sitting in Key Items but shown on Items > Multiplayer / Items > Remembrances. The misclassifications stem from `tmp/repos/er-save-manager`'s own categorization being a community taxonomy, not 1:1 with what the game UI shows. Source of truth was switched to **Fextralife per-item pages** (breadcrumb category) cross-checked with in-game observation by the user.

**Architecture:**
- New `backend/db/data/info.go` — `var Information map[uint32]ItemData` with **114 entries**: 35 base-game About tutorials + 7 Paintings + 19 Maps + 17 Notes + 6 Letters + 1 Mirage Riddle, plus DLC: 2 About + 5 Maps + 3 Paintings + 2 Notes + 8 Cross/Diary/Message items + 6 misc.
- New `info` category in `backend/db/db.go` `GetItemsByCategory` dispatch + appended to `GetAllItems` list.
- New `<option value="info">Information</option>` in `frontend/src/components/CategorySelect.tsx` under the "Items" optgroup.
- Pretty rendering in `frontend/src/components/DatabaseTab.tsx` — `info` → "Information" in both Category column and detail panel header.
- `tools.go` / `key_items.go` / `crafting_materials.go` reorganized to match in-game tabs.

**Migrations performed:**
- **52 entries `key_items.go` → `info.go`** (35 About tutorials, 7 Paintings, 1 Map: Dragonbarrow, 2 DLC About, 7 DLC info messages: Castle Cross / Ancient Ruins Cross / Monk's Missive / Storehouse Cross / Torn Diary / Message from Leda / Tower of Shadow).
- **53 entries `tools.go` → `info.go`** (1 About the Map, 19 base + 5 DLC region maps, 6 Letters: Volcano Manor / Patches / Bernahl / Burial Crow's / Zorayas's / Rogier's, 17 Notes range 0x4000222E–0x4000223D + Miquella's Needle + Lord of Frenzied Flame, 1 DLC Letter for Freyja, 2 DLC Notes, 3 DLC Paintings).
- **6 more `tools.go` → `info.go`** after user verification of the first migration uncovered additional misclassifications: Irina's Letter, Red Letter, Cross Map (DLC), 3× Ruins Map (DLC).
- **13 entries `tools.go` → `key_items.go`** — Stonesword Key, Rusty Key, Drawing-Room Key, Imbued Sword Key, Royal House Scroll, Well Depths Key (DLC), Gaol Upper/Lower Level Key (DLC), Storeroom Key (DLC), Secret Rite Scroll (DLC), Keep Wall Key (DLC, cut+ban_risk preserved), Prayer Room Key (DLC, **icon TODO**), Academy Glintstone Key.
- **11 entries `tools.go` → `key_items.go` (Crystal Tears)** — Speckled / Crimson / Opaline (Bubble + Hard) / Leaden / Crimsonwhorl / Cerulean Hidden + DLC Viridian Hidden / Crimsonburst Dried / Oil-Soaked / Deflecting Hardtear. Fextralife: *"Crystal Tears in Elden Ring are Key Items that can be mixed in the Flask of Wondrous Physick."* User confirmed.
- **7 entries `tools.go` → `key_items.go`** — Whetstone Knife, Glintstone Whetblade, Conspectus Scroll, Academy Scroll, Margit's Shackle, Mohg's Shackle, Pureblood Knight's Medal. Five had IconPath already pointing at `items/key_items/`.
- **5 entries `tools.go` → `crafting_materials.go`** — Golden Centipede, Sanctuary Stone, Glintstone Firefly, Volcanic Stone, Gravel Stone.
- **13 entries `key_items.go` → `tools.go`** (Multiplayer Items) — Bloody Finger, Tarnished's/Phantom Bloody Finger, Tarnished's/Phantom Recusant Finger, Recusant Finger, Festering Bloody Finger, Tarnished's Wizened Finger, Tarnished's Furled Finger, Duelist's Furled Finger, Igon's Furled Finger (DLC), Furlcalling Finger Remedy, Small Golden/Red Effigy, Taunter's Tongue. Fextralife breadcrumb: *Equipment & Magic / Items / Multiplayer Items*.
- **25 entries `key_items.go` → `tools.go`** (Remembrances) — 15 base-game + 10 DLC. Fextralife breadcrumb: *Equipment & Magic / Items / Remembrances*.

**Cut-content / ban-risk audit performed during this work:**
- `0x400023EB` "About Multiplayer" — confirmed truly cut (Fextralife: "Unavailable", spawned copies prefixed `[ERROR]`). Kept `cut_content + ban_risk`.
- `0x400023A7` "About Monument Icon" — **NOT** cut content (shipped on disc v1.0); broken in patch 1.06. Dropped `cut_content`, kept `ban_risk` because EAC doesn't whitelist by version. Comment explains the distinction.
- `0x4000229D` "Erdtree Codex" — confirmed cut (Fextralife/Fandom/GameRant unanimous). Kept `cut_content + ban_risk`. Stays in `key_items.go` (Fextralife taxonomy = Key Item, not Information).
- `0x40001FF5` "Burial Crow's Letter" — Fextralife says cut Key Item, but user verified in-game that on a save that received it, it shows on Information tab. Kept `cut_content + ban_risk`, classified Information.
- `0x401EA3CF` "Letter for Freyja" — Fextralife master list says Information, per-item page says Key Item. Tagged Information per master list with TODO comment to verify in-game.

**Items intentionally NOT moved:**
- Empty Flasks (Flask of Cerulean/Crimson Tears (Empty), Flask of Wondrous Physick (Empty)) — Fextralife says "Items / Consumables" which equals our `tools.go` semantically. User confirmed.
- Dragon Heart (`0x4000274C`) — Fextralife: "Dragon Heart is a Key Item". User confirmed. Stays in `key_items.go`.
- 5 borderline pot/bottle/kit items (Cracked Pot, Ritual Pot, Hefty Cracked Pot, Perfume Bottle, Crafting Kit) — Fextralife says "Key Item, Crafting Material, and optional Keepsake" for Cracked Pot. User decision: stay in `key_items.go`.
- Cookbooks / Bell Bearings / Whetblades / 3 Igon's quest items — intentionally remain in `key_items.go` (data home for these is filtered separately via `IsCookbookItemID` / `IsBellBearingItemID` / `IsWhetbladeItemID` in `db.go`).

**Known issue / TODO:**
- `frontend/public/items/tools/quest/prayer_room_key.png` — the icon for Prayer Room Key is currently a binary copy of `frontend/public/items/gestures/prayer.png` (both 8762 bytes, identical bytes). Someone substituted the gesture's prayer-emote icon for the actual key artwork. Fextralife / Fandom CDNs block direct curl, so the real artwork couldn't be fetched in this branch. TODO comment added at the entry; user will manually drop in the correct artwork later.

**Counts (after migration):**
| File | Before | After | Δ |
|---|---|---|---|
| `info.go` (new) | — | 114 | +114 |
| `key_items.go` | 396 | 343 | -53 |
| `tools.go` | 364 | 315 | -49 |
| `crafting_materials.go` | 80 | 85 | +5 |

**Tests:**
- `go vet ./backend/...` — clean.
- `go test ./backend/db/...` — passes.
- `npx tsc --noEmit` (frontend) — clean.
- `make build` — full Wails build successful.
- Manual `make dev` verification by user, performed iteratively across the migration. User signed off on each batch.

**Research methodology:**
- **Primary source**: Fextralife wiki per-item pages (breadcrumb category, e.g. *Equipment & Magic / Items / Multiplayer Items*). Cross-verified with the Fextralife master "Info Items" / "Multiplayer Items" / "Crystal Tears" pages.
- **Discarded as authoritative**: `tmp/repos/er-save-manager/.../Goods/*.txt` — community taxonomy, not 1:1 with in-game UI. The user's in-game observation overrode er-save-manager's `KeyItems.txt` placement of Letters that actually live on Information.
- **In-game verification**: user spot-checked every batch by loading a save, opening Database, and matching the displayed dropdown against the in-game inventory tabs.
- **Cross-file duplicates check**: `awk` aggregation across all 19 ItemData maps in `backend/db/data/` confirmed zero ID collisions across files.



**Goal:** Make the World tab usable without expanding every section. Each section's bulk actions (Unlock All / Lock All / Reveal All / Reset / Activate All / Deactivate All / Kill All / Respawn All) now sit on the collapsed header next to the progress bar, so single-click bulk edits no longer require an extra expand step. Section open/closed state is per-session and resets when a different save is loaded — clean baseline on every fresh load. Also reworked the Online Safety Mode contract: confirmation modals now appear only when Safety Mode is enabled.

**Why these changes:**
- The previous behavior persisted accordion state in `localStorage` indefinitely and forced expansion to access bulk actions — clunky for users who routinely toggle one or two flags per session.
- The previous Online Safety Mode contract showed Tier 1 modals on first use even when Safety Mode was off, then suppressed them after "Don't ask again". Inconsistent: users got an unexpected modal once per action, then nothing. The new contract is binary and predictable: Safety Mode off = no modals (info ⚠ icon stays for on-demand education); Safety Mode on = always modal.

**Change:**
- `frontend/src/components/AccordionSection.tsx`:
    - New `resetSignal?: number | string` prop. When defined, state persists in `sessionStorage` (survives tab switches / remounts) but resets to `defaultOpen` and wipes its session entry whenever the value changes.
    - Equality-guarded reset (`lastResetSignalRef`) instead of "first run" boolean — needed because React 18 StrictMode double-invokes effects in dev, which would otherwise collapse a section on every remount.
    - `actions` prop is now rendered in the **collapsed** state too, between the progress bar and the `current/total` counter (previously only visible when open).
- `frontend/src/App.tsx`: new `saveLoadKey` state, incremented after `SelectAndOpenSave()` succeeds. Passed to `<WorldTab>`.
- `frontend/src/components/WorldTab.tsx`:
    - `saveLoadKey` prop wired into all 11 `<AccordionSection resetSignal={…}>` calls (map / graces / pools / colosseums / bosses / quests / gestures / cookbooks / bells / whetblades / regions).
    - Renamed "Map & Fog of War" → "Map".
    - Pools: added `handleGlobalDeactivateAllPools` + "Deactivate All" button.
    - Colosseums: added `handleLockAllColosseums` + "Lock All" button.
    - Bosses: renamed global "Respawn" → "Respawn All" for consistency.
    - `btnSm` style: `border-border/50` → `border-foreground/30 bg-foreground/5` — kept readable in both light and dark themes (previous `border-border` washed out in light mode).
- `frontend/src/components/RiskActionButton.tsx`:
    - Confirmation gating simplified to `requiresConfirm = !!entry && safetyMode.requireConfirmFor(entry.tier)`. Removed `dismissedKey` / `isDismissed` / `localStorage.setItem('setting:dismissedRisk:…')` plumbing — no longer needed because dismissal only existed to suppress the off-mode modal that no longer fires.
    - Modal: dropped "Don't ask again" checkbox + `allowDismiss` prop; replaced two-branch (allowDismiss true / false) UI with a single fixed amber notice "Online Safety Mode is on — confirmation required".
    - The ⚠ info icon next to each action is unchanged — still always-on, click-to-popover.

**Test plan executed:**
- `npx tsc --noEmit` — clean.
- `make build` — full Wails build OK.
- Manual (`make dev`):
    - Load save → all 11 World sections collapsed ✅
    - Expand Map → switch to Character tab → switch back to World → Map still expanded ✅ (the StrictMode double-effect bug surfaced and was fixed mid-iteration)
    - Load a different save → all sections collapsed again, including Map ✅
    - Collapsed-state actions: Unlock All / Lock All / Reveal All / Reset / Activate All / Deactivate All / Kill All / Respawn All all execute without expanding ✅
    - Light theme: button borders clearly visible ✅
    - Safety Mode off → click Gestures → Unlock All → action runs immediately, no modal ✅
    - Safety Mode on → same click → modal appears ✅

**Bug surfaced and fixed during this branch:**
- First implementation of the reset gate used a `firstResetRef` "skip first invocation" boolean. In React 18 dev StrictMode, every `useEffect` fires twice on mount (mount → simulated unmount → mount). The first invocation flipped `firstResetRef.current` to `false`, the second invocation passed the gate and called `setOpen(defaultOpen)` — collapsing the section on every remount, including returning to the World tab. Replaced with a value-comparison ref (`lastResetSignalRef`): only collapse when the actual `resetSignal` value differs from the last seen value. StrictMode double-invocations no longer trigger a reset because the value is unchanged.

**Decisions during planning (recorded so we don't reopen them):**
- Kept "Reveal All" instead of normalising to "Unlock All" for the Map section — semantically a map is *revealed*, not *unlocked*; "Unlock map" reads like a DLC purchase. Pools stay on "Activate All / Deactivate All", Bosses stay on "Kill All / Respawn All" — same reasoning, the verbs match the underlying state.
- NPC Quests deliberately has no global bulk button (per-NPC step-set is the only sensible operation; a "complete all quests" would be both incoherent and high-risk).



**Goal:** Educate the user about which save edits are commonly reported to trigger Easy Anti-Cheat detection during online sync, instead of silently allowing or hard-blocking them. Cover the full UI: Item Database, Inventory, Character, World (graces / bosses / quests / map / gestures / cookbooks / bell bearings / regions / colosseums / summoning pools), Tools (Character Importer), Settings.

**Why this approach:**
EAC detection is probabilistic and undocumented. Hard blocks would be paternalistic; silent allow leaves users uninformed. A 3-tier system with explicit "why" descriptions framed as community-reported (not officially confirmed) lets users make informed decisions. Online Safety Mode toggle gives a one-click switch for users actively playing online — Tier 2 edits are clamped to legal values, Tier 1 actions force confirmation regardless of prior "Don't ask again" dismissals.

**Architecture:**
- **Tier 0** (cosmetic / read-only): no marking
- **Tier 1** (caution: bulk grace unlock, map reveal, quest step skip, character import …): modal-confirm with per-action `localStorage` opt-out
- **Tier 2** (high risk: cut content, attribute > 99, runes > 999M, talisman pouch > 3 …): modal-confirm + field outline + (under Online Safety Mode) auto-clamp/disabled
- **`RISK_INFO` dictionary** (`frontend/src/data/riskInfo.ts`) — 24 entries, each with `tier / level / title / whyBan / reports / mitigation / sources[]`. Source URLs intentionally left optional/empty when not verified
- **Online Safety Mode** — `useSafetyMode()` hook (Context + `localStorage`) exposes `enabled / setEnabled / isDisabledFor(tier) / requireConfirmFor(tier)`. Global amber banner at top of app when active

**Components added:**
- `RiskInfoIcon.tsx` — clickable ⚠ trojkąt + popover via `createPortal(document.body)` (Why / Reports / Mitigation / Sources, ESC + outside-click dismissal)
- `RiskBadge.tsx` — inline `CUT` / `⚠ BAN` / `PRE-ORDER` / `DLC DUP` tag + adjacent `RiskInfoIcon`
- `RiskActionButton.tsx` — wraps `<button>`, shows confirm modal with opt-out checkbox; under Online Safety Mode the dismissal checkbox is replaced by a notice and prior dismissals are ignored
- `RiskSectionBanner.tsx` — colored bar above sections (Map, Quests) with first-sentence summary + info icon
- `SafetyModeBanner.tsx` — global top-of-app banner when Online Safety Mode is on
- `state/safetyMode.tsx` — Context + Provider, wired in `main.tsx` around `<App/>`

**Coverage rollout (5 phases):**
- **Phase 1** — `RISK_INFO` baseline (4 per-flag entries) + `RiskInfoIcon` + `RiskBadge`. Refactored existing inline `CUT` / `⚠ BAN` tags in `DatabaseTab`, `InventoryTab` and gestures (in `WorldTab`) to the shared component
- **Phase 2** — `useSafetyMode()` hook + global `SafetyModeBanner` + Settings toggle. Reorganized Settings: split out a dedicated "Safety" section, moved "Show Cut & Ban-Risk Items" filter from "UI Customization" to live next to Online Safety Mode (independent — one is list visibility, the other is action gating)
- **Phase 3** — 7 per-action Tier 2 entries + `getRunesRiskKey` / `getAttributeRiskKey` / etc. helpers. Wired Runes input in `CharacterTab` to show red outline + ⚠ icon when value exceeds 999M. (Other inputs already clamped 1-99 / 0-3 / etc. — outline would be dead, kept the dictionary entries for future use if clamping is ever lifted)
- **Phase 4** — 13 per-action Tier 1 entries + `RiskActionButton` component. Wired 11 bulk action buttons in `WorldTab` (Reveal All, Unlock All Graces / Cookbooks / Bell Bearings / Regions / Gestures, Activate All Pools, Unlock All Colosseums, Kill All Bosses, Set quest step) and `CharacterImporter` Confirm Import
- **Phase 5** — section banners on `World → Map` (`map_reveal_full`) and `World → Quests` (`quest_step_skip`); Runes input clamping under Online Safety Mode (auto-cap to 999,999,999 with toast)
- **Phase 6** — this changelog entry, ROADMAP entry, `spec/32-ban-risk-system.md` reference doc

**Cleanup (chore commits during this branch):**
- Removed `frontend/src/components/GeneralTab.tsx` (legacy "Character" tab — not imported anywhere)
- Removed `frontend/src/components/StatsTab.tsx` (legacy "Stats" tab — not imported anywhere)
- Removed `frontend/src/components/WorldProgressTab.tsx` (legacy "World Progress" tab — not imported anywhere)
  These were leftovers from a UI refactor; Grep for "Runes" / "Unlock All" / "Reveal All" surfaced them and led to two iterations of edits landing in dead code before the user pointed it out. Updated `~/.claude/projects/.../memory/feedback_react_component_mapping.md` with explicit list of dead components to ignore + a map of preview-mode vs editing-mode component routing in `App.tsx`

**Bug fixes during the branch:**
- `RiskInfoIcon` initial implementation used `position: fixed` inside the React tree — overflow/transform on ancestor elements clipped the popover, plus Tailwind 4 arbitrary `z-[200]` was unreliable. Switched to `createPortal(document.body)` + inline `zIndex: 9999` — popover now renders above everything regardless of layout
- `RiskInfoIcon` inside a `<label>` element in gestures sub-tab: native HTML label-for-control behavior captured the click on the icon and toggled the checkbox without React's `stopPropagation` ever running. Restructured the gesture row: the `<label>` now wraps only checkbox + name; the `RiskInfoIcon` lives as a sibling outside the label
- Phase 1 changes initially landed only in dead `WorldProgressTab.tsx`; after user reported the icons missing in the live "World" tab, mirrored the changes to `WorldTab.tsx`
- Phase 3 changes initially landed in dead `GeneralTab.tsx`; after user pointed it out, moved to `CharacterTab.tsx`

**Tests:**
- `npx tsc --noEmit` — clean across all phases
- `make build` — full Wails build successful (verified after Phase 1)
- Manual verification per phase: load real save → exercise each tab → confirm popover / modal / outline / banner / clamp behavior matches phase test plan. User confirmed each phase before commit

**UX preferences captured (saved to memory):**
- User runs the app via `make dev` and fully restarts the dev server after every change (Cmd+R doesn't work in Wails dev on macOS) — don't suggest HMR-only refresh
- When grep surfaces multiple components matching a feature, always start from `frontend/src/App.tsx` routes to identify the active one. Editing dead code wastes a round trip with the user

**Known limitations / future work (Phase 6+):**
- `RISK_INFO.sources[]` URLs are mostly empty placeholders — to be filled with verified links
- No automated test that grep-checks every `riskKey="..."` literal in `*.tsx` against the `RiskKey` union (TypeScript catches it at compile time, but a CI safety net would help)
- Boss kill flag still single-flag (see open ROADMAP item) — bulk_boss_kill warning is correct in spirit but the in-game effect is limited until the multi-flag boss defeat refactor lands
- ToolsTab placeholders (Save Comparison, Diagnostics, Backup Manager) are not yet implemented; once they are, evaluate whether they need risk markers

### Branch: feat/apply-mirror-favorite — Apply in-game Mirror Favorites preset onto character (cross-gender works) + remove buggy direct preset apply

**Goal:** Implement the in-game "apply preset to character" action that the player triggers at the Roundtable Hold mirror, so the editor can do it without launching the game. As a side effect, eliminate the long-standing cross-gender bug where applying a Type B preset produced a bald, male-faced character.

**Why this works (RE):**
The game's apply algorithm copies five FaceData segments verbatim from a Mirror Favorites preset slot to the character's FaceData blob: 32 B Model IDs (real PartsIds, not UI indices), 64 B face shape, 7 B body proportions, 91 B skin & cosmetics. The 64 B `unk0x6c` block is preserved unchanged (per-character state, not preset-encoded). `slot.Player.Gender` is set from `preset.body_type` (inverted: Mirror `body_type=0` → `Gender=1` male; `body_type=1` → `Gender=0` female). Trailing flags at FaceData `0x125..0x126` are zeroed. Equipment is NOT cleared (game does, but we leave gear intact — user's choice). Verified on real saves (`tmp/re-character/ER0000-{before,after}.sl2` byte-for-byte) and in-game on Steam Deck (cross-gender M↔F applied on slots 0 and 4 with `tmp/re-character/ER0000-2-presets.sl2`).

**Why direct apply was removed:**
`ApplyAppearancePreset` (the old "Apply to Character" button) skipped Model IDs entirely for Type B presets — there is no female PartsId mapping in `presets.go`, only male hair via `LookupMaleHairPartsID`. Result: every Type B apply produced a character with `face=0, hair=0, ...` which the game renders as default male model + bald, ignoring the female `Gender` field. The new Mirror-Favorites-based apply sidesteps this entirely because Mirror slots hold real game-generated PartsIds (when the player creates the preset in-game). Apply to Character is now redundant — workflow is: Add to Mirror (Type A) or create preset in-game (Type B) → click ✓ on Mirror slot.

**Change:**
- `app.go`:
    - **New:** `ApplyMirrorFavoriteToCharacter(charIndex, mirrorSlotIndex)` — validates indices + FACE magic, copies 5 FaceData segments, preserves `unk0x6c`, flips Gender, zeros trailing flags. Pushes undo.
    - **Removed:** `ApplyAppearancePreset` (~80 lines including `LookupMaleHairPartsID` call site). The lookup table itself is retained — still used by `WriteSelectedToFavorites` for Type A.
- `backend/core/offset_defs.go`: corrected stale comments (`FavOffMarker`, `FavOffSkin = 91 B not 69 B`, `FavOffUnkBlock`, `FavOffBody`).
- `frontend/src/components/CharacterTab.tsx`:
    - Added ✓ button next to × per active Mirror slot → `ApplyMirrorFavoriteToCharacter`.
    - Added Type B guard in `handleWriteFavorites` (Add to Mirror) — blocks with toast: `Type B (female) presets cannot be written to Mirror — would create bald, male-faced slot. Create the preset in-game instead.`
    - Removed `handleApplyPreset`, `applyingPreset`/`confirmApply` state, "Apply" + "Confirm" buttons, `ApplyAppearancePreset` import.
    - Updated tooltip text to describe new flow.
- `frontend/src/components/AppearanceTab.tsx` (preview-only readOnly tab): mirrored same removals + guard for consistency.
- `frontend/wailsjs/go/main/App.{d.ts,js}`: regenerated — `ApplyAppearancePreset` removed, `ApplyMirrorFavoriteToCharacter(arg1: number, arg2: number): Promise<void>` added.
- `spec/31-appearance-presets.md`: corrected trailing-flag offset (`0x124..0x126`, was `0x12C..0x12E` — RE-verified via `tmp/re-character/facedata_dump.txt`); added implementation status section.

**Tests:**
- `app_apply_mirror_test.go` (new) — `TestApplyMirrorFavoriteToCharacter` reproduces the in-game M→F apply on `ER0000-before.sl2`, asserts byte-for-byte match with `ER0000-after.sl2` for ModelIDs/FaceShape/Body/Skin segments + Gender flip + `unk0x6c` preservation + trailing flags. `TestApplyMirrorFavoriteToCharacter_Errors` covers no-save / bad indices.
- `go test ./backend/... .` — all suites pass.
- `make build` — full Wails build OK.
- **Manual in-game (Steam Deck):** applied Mirror slot 0 (female preset 1) → char slot 4 (male) and Mirror slot 1 (male preset) → char slot 0 (female). Both characters render correctly post-apply, gender flipped, voice changed, model/hair/decals match preset. Other slots untouched, equipment intact, no crashes.

**Known limitations / future:**
- `WriteSelectedToFavorites` still skips Model IDs for Type B presets (the underlying bug — UI guard now blocks the bad path). Resolved via separate task: re-source `presets.go` Type B as raw 0x130-byte blobs.
- Equipment clearing (game zeroes gender-specific gear on apply) is intentionally NOT replicated — preserves player's gear at the cost of possibly invisible armor for cross-gender slots (no crash, just visual).

### Branch: fix/profile-summary-offset — Fix wrong UserData10 ProfileSummary offsets (was corrupting Mirror Favorites slot 1)

**Goal:** Reverse-engineer the real UserData10 layout from real saves and fix our wrong ProfileSummary read/write offsets which had been silently corrupting Mirror Favorites preset slot 1 on every save.

**Root cause (RE):**
Two reference repos (`er-save-manager` Python, `ER-Save-Editor` Rust) place ProfileSummary at `0x1954` post-checksum. Our code read/wrote at `0x310/0x31A` (PC) and `0x300/0x30A` (PS4). Verified on real saves (`tmp/re-character/ER0000-{before,after}.sl2` for PC + `tmp/save/oisisk_ps4.txt` for PS4) — both files show clear ProfileSummary structure starting at `0x195E` (after 10-byte ActiveSlots block at `0x1954`). The offset `0x31A` lies inside Mirror Favorites preset slot 1 (which spans `0x284..0x3B3`), so every metadata flush via our editor injected bytes into that preset slot — explaining the user's reports of "preset 1 looks weird after edit".

**Other findings during RE:**
- ProfileSummary stride is **`0x24C` bytes** (588 bytes), not the `0x100` we used.
- ProfileSummary name field is **17 u16** (16 chars + null terminator = 34 bytes), so Level u32 lives at `+0x22`, not `+0x20` as our code assumed.
- **PC and PS4 share IDENTICAL UserData10 layout** — only the file-level container header and PC-only MD5 checksum differ; the post-checksum data structure is the same. Our code historically had separate offsets for PC vs PS4 (`0x310/0x31A` vs `0x300/0x30A`) which was a coincidental near-match by 0x10-byte difference, not a real platform difference.
- The 64-byte `unk0x6c` block inside FaceData is **NOT touched** by the game when applying a preset — it's per-character state, not preset-encoded. Our previous attempts to re-derive it for cross-gender apply were unnecessary.

**Change:**
- `backend/core/save_manager.go`:
    - PC + PS4 readers: `udReader.Seek(0x310/0x300, 0)` → `udReader.Seek(0x1954, 0)` (unified).
    - `flushMetadata`: ActiveSlots at `0x1954+i`, ProfileSummary at `0x195E+i*0x24C`. PC/PS4 paths merged (only the SteamID prefix at `0x00` differs between platforms).
- `backend/core/structures.go`:
    - `ProfileSummary.Read`: read 16 u16 name, seek to `start+0x22` for Level, advance to `start+0x24C` for next entry.
    - `ProfileSummary.Serialize`: write name at `+0`, Level at `+0x22`. The remaining 0x226 bytes per slot (face data snapshot, equipment summary, archetype, gift, body_type) are intentionally NOT written by us — they retain the game's last serialized values, which is correct.
- `backend/core/offset_defs.go`:
    - `FavSafeSlots` expanded from `[6]int{0, 10..14}` to `[15]int{0..14}`. Slots 1-9 are no longer at risk of corruption; the historical `FavSafeSlots` band-aid is no longer needed (kept for backward compatibility but deprecated).
- `spec/23-user-data-10.md` — rewrite with correct offsets, mark layout as verified PC+PS4.
- `spec/31-appearance-presets.md` (new) — full Mirror Favorites preset slot layout + apply algorithm reverse-engineered from real saves, plus implications for `ApplyAppearancePreset` and `WriteSelectedToFavorites`.
- `spec/README.md` — index entry for #31.

**Tests:**
- `go test ./backend/...` — all suites pass (core, db, db/data, vm).
- `make build` — full Wails build OK (frontend tsc + Vite + Go compile + macOS package).
- Custom round-trip diagnostic on `tmp/re-character/ER0000-before.sl2`: load → save → reload, **0 bytes of diff** in the entire 28.9 MB file. Mirror Favorites slot 1 specifically verified intact (was being corrupted on every save before this fix).
- Verified parsing: names "[PL] Jagna", "[PL] Zofia", "Tester", "Ada", "Random" with levels 30, 129, 17, 34, 9 — all match in-game ground truth.

**Backward compatibility:**
Saves edited with the previous (buggy) version of our editor have garbage data at `0x31A+i*0x100` (= mid-stream in Mirror Favorites slots 1-8). With this fix those slots are **no longer touched**, so the corruption stops; existing corruption in slots 1-8 of those saves remains until the user re-creates the affected presets in-game. ProfileSummary at `0x195E+i*0x24C` was always preserved by the game itself across save/reload (game writes it correctly, only our editor was reading/writing the wrong location), so name and level data in the character-select UI are unaffected by the historical bug.

**NOT in this branch (deferred):**
- New feature "Apply from Mirror Favorites slot N to character" — direct byte-copy preset → FaceData using the algorithm in `spec/31-appearance-presets.md`. Bigger UI + backend work; separate session.
- Re-sourcing `presets.go` as raw 0x130-byte blobs — large data sourcing effort, separate task.

### Branch: fix/dbtab-flag-filter-too-broad — Restore visibility of arrows/bolstering/crafting/most tools in Item Database

**Goal:** Regression fix. After the `dlc` (701 entries) and `stackable` (532 entries) flag sweeps, the "Cut & Ban-Risk" toggle in Settings (when off — the default for many users) was hiding **all** items with any flag in `Flags`. That meant nearly every entry in `arrows_and_bolts.go`, `bolstering_materials.go`, `crafting_materials.go`, and most of `tools.go` became invisible in both the Item Database tab and the inventory list.

**Root cause:** `DatabaseTab.tsx:120` and `InventoryTab.tsx:273` had:
```ts
if (!showFlaggedItems && item.flags?.length > 0) return false;
```
The check is intentionally inverse-of-toggle, but `length > 0` was too broad — it matched any flag including informational ones (`dlc`, `stackable`).

**Fix:** Both files now filter only on the four "risky" flag values that the toggle is actually designed for:
```ts
const RISKY_FLAGS = ['cut_content', 'ban_risk', 'pre_order', 'dlc_duplicate'];
if (!showFlaggedItems && item.flags?.some(f => RISKY_FLAGS.includes(f))) return false;
```

`WorldTab.tsx` and `WorldProgressTab.tsx` were not affected — they use the narrow check `flags?.includes('ban_risk')` directly.

**Impact:** With the toggle off, users see all stackable consumables/materials/arrows again. Risky items (cut content, ban risk, pre-order, duplicates) remain hidden as before — toggle behavior unchanged.

**Tests:** `cd frontend && npx tsc --noEmit` ✅, `make build` ✅.

### Branch: fix/ui-remove-redundant-hex — Drop hex-below-item-name (duplicates the optional ID column)

**Goal:** When the user enabled the "ID (HEX)" column toggle in Settings, the hex value rendered twice — once below the item name, once in the dedicated column. Per user feedback, remove the always-shown hex below the name. Users who want the hex still get it via the column toggle.

**Change:**
- `frontend/src/components/DatabaseTab.tsx` — the cell-below-name had a `showPreview ? <preview> : <hex>` ternary. Removed the else-branch so hex is no longer rendered as a fallback. Preview (e.g. `Heavy +25` for upgradeable weapons) still shows when applicable; otherwise the cell is empty.
- `frontend/src/components/InventoryTab.tsx` — the `<span>` rendering hex below the name was always shown. Removed entirely. The outer `flex flex-col gap-0.5` wrapper now has a single child (the name+flags row); kept as-is — cosmetic, no visual difference.

**No state, props, or styling changes elsewhere.** Settings checkbox `columnVisibility.id` continues to gate the optional "ID (HEX)" column unchanged.

**Tests:** `cd frontend && npx tsc --noEmit` ✅, `make build` ✅.

### Branch: feat/ban-risk-warning-modal — Warn when adding ban-risk-flagged items + minor icon downloader iteration

#### Part 1 — Ban-risk warning modal in `DatabaseTab`

**Goal:** Per user direction — when a user adds an item flagged `"ban_risk"` from the Item Database, gate the regular Add modal behind a warning that explains the risk (Easy Anti-Cheat detection if going online with cut-content / cheat-flagged items in inventory). Modal includes an "Ignore all ban risk warnings" checkbox so power users can opt out permanently.

**Change (`frontend/src/components/DatabaseTab.tsx`):**
- New state: `banRiskWarning: ItemEntry[] | null` and `ignoreBanRisk: boolean` (initialized from `localStorage["setting:ignoreBanRiskWarning"]`).
- `openModal(items)` now branches:
    - If `!ignoreBanRisk && items.some(i => i.flags?.includes('ban_risk'))` → set `banRiskWarning(items)` and return.
    - Otherwise → call `openConfirmModal(items)` (the previous body of `openModal`, refactored).
- New "Add Anyway" path: calls `openConfirmModal(items)` directly, bypassing the gate for this batch.
- New `handleIgnoreBanRiskChange(checked)`: writes the toggle to `localStorage` so it persists across sessions.

**Modal UI (red-themed, z-index above confirmModal):**
- Triangle alert icon + "Ban Risk Warning" header.
- Single-item path: bold item name in copy. Multi-item path: count + bullet list of all ban-risk items in the selection (scrollable when long).
- Checkbox: "Ignore all ban risk warnings" — bound to `ignoreBanRisk` state, persists immediately on toggle.
- Two actions: "Cancel" (closes warning, no add) vs "Add Anyway" (closes warning, opens regular Add modal).

**No backend changes required:** The `Flags []string` field on `ItemEntry` is already exposed via Wails bindings (used elsewhere in `WorldTab.tsx` / `WorldProgressTab.tsx` for similar filtering). Only frontend logic added.

**Tests:** `cd frontend && npx tsc --noEmit` ✅, `make build` ✅ (Wails generate-bindings + Vite + Go compile + macOS package — 9.3s).

**Manual verification deferred** to user — they confirmed previous UI changes manually after build green.

#### Part 2 — Icon downloader iteration 5 (minor refinements)

**Bugs fixed in `scripts/download_icons.go`:**
- Removed `"Bloody"` from affinity-strip list — it's NOT an infusion (like Heavy/Keen/Magic) but rather part of unique DLC weapon names ("Bloody Lance", "Bloody Buckler"). Stripping yielded base weapons, breaking lookups.
- Replaced `insertPossessive` (single-variant) with `insertPossessiveVariants` (returns 0..N variants). Handles double-`s` words like `Thopss` (from "Thops's") that the previous heuristic skipped.
- Added "un-stripped name" as a fallback variant for weapons/shields/ranged/arrows — when affinity stripping yielded a name and that fails, retries with the full original name (catches cases like "Flame_Art_Main_Gauche" where wiki actually has the full name).

**Imports:** 1 new icon — `frontend/public/items/sorceries/thopss_barrier.png` (Thops's Barrier, double-s possessive case proven).

**Remaining:** ~78 icons still not auto-fetchable — mostly cut-content gestures (no wiki page), DLC notes with combined wiki entries (e.g. Zorayas's & Rogier's Letter on a single page), items with multi-apostrophe names. Manual import or per-item WebFetch needed; defer.

### Branch: feat/stackable-flag-sweep — Add `stackable` flag to all items with MaxInventory > 1

**Goal:** Per user direction (audit cycle) — explicitly mark which items stack (vs single-instance) so future UI work can filter without re-deriving from `MaxInventory`. Adds `Flags: []string{"stackable"}` to every entry where `MaxInventory > 1`.

**Implementation:** Subagent ran a one-shot Go script that:
1. For each backend file, scanned every `0xXX:` map entry.
2. Matched `MaxInventory: <N>` with `N >= 2`.
3. Added/merged `"stackable"` into the existing `Flags` slice (preserves existing flags like `dlc`).
4. Skipped `MaxInventory == 1` entries (intrinsically non-stackable).

**Per-file count: 532 new `"stackable"` markers**

| File | +stackable |
|---|---:|
| `tools.go` | +281 (68 merged with existing flags, e.g. `dlc → dlc, stackable`) |
| `bolstering_materials.go` | +76 (2 merged) |
| `crafting_materials.go` | +71 (9 merged) |
| `arrows_and_bolts.go` | +64 (5 merged) |
| `key_items.go` | +40 (0 merged) |

**Other 13 backend files unchanged:** all entries have `MaxInventory == 1` (equipment, spells, gestures, ashes-of-war, talismans, etc.) — none stackable.

**Total merge cases: 84** — `Flags: ["dlc"]` became `Flags: ["dlc", "stackable"]`. No entries already contained `"stackable"`.

**Verification:**
- Independent script confirmed 0 `MaxInventory > 1` entries are missing the flag.
- Spot-checked diffs in 3 files — only Flags changed, no other fields touched.
- `go build ./backend/...` ✅, `go test ./backend/db/...` ✅.

**App impact:** Zero — flag is data-only marker until UI work consumes it.

### Branch: feat/icon-downloader-improvements — Improve `scripts/download_icons.go` + import 60 icons + DLC flag sweep across DB

**Two related changes shipped together** — both flow from the audit (`tmp/items_audit_report.md`) follow-up.

#### Part 1 — Icon downloader fixes + 60 new PNG imports

**Goal:** Pre-fix run had ~12% success rate on missing icons (15/130). Most failures were script bugs in URL/name generation, not genuinely-missing assets on `eldenring.wiki.gg`.

**Bugs fixed in `scripts/download_icons.go`:**
1. **Lowercase prepositions** — `toWikiName` capitalized "of"/"the"/"a"/"in"/"to"/"with"/"from"/"for"/"on"/"at"/"by"/"or"/"as"/"and". Wiki URLs keep these lowercase (e.g. `Rain_of_Fire`, not `Rain_Of_Fire`). Added `lowercaseWords` set, applied for non-leading positions.
2. **Missing prefixes** — `isCrafting` only tried `ER_Icon_Item_`/`ER_Icon_Tool_`. Added category-specific prefixes: `ER_Icon_Crafting_Material_`, `ER_Icon_Bolstering_Material_` (for `bolstering_materials/` paths). For `isTools` added `ER_Icon_Key_Item_` (cookbooks), `ER_Icon_Note_`, `ER_Icon_Info_` (notes/letters), `ER_Icon_Map_`, `ER_Info_` (combined entries like Zorayas's Letter).
3. **Possessive variant** — many local filenames strip apostrophes (`tibias_cookbook.png`) while wiki uses URL-encoded apostrophe (`Tibia%27s_Cookbook.png`). New `insertPossessive` helper inserts `%27` before trailing `s` of any 4+-char word ending in `s` (skipping double-`s` like "Hess"). Tries both original and possessive variants per prefix.
4. **`commons.wiki.gg` fallback** — wiki.gg serves some images on the commons subdomain. Added as host fallback after `eldenring.wiki.gg`.
5. **Affinity stripping** — added missing `Bloody` (DLC infusion variant for some weapons). Added multi-word `Flame_Art` BEFORE single-word `Flame` to avoid prefix-shadowing on names like `Flame_Art_Main_Gauche`.

**Result:** Success rate climbed from ~12% to ~46% (60/130 net icons imported across 4 iterations). Remaining ~80 are mostly genuinely-not-on-wiki assets (cut content like `The Carian Oath` / `Fetal Position`, obscure DLC notes with combined entries on wiki, items with multi-apostrophe names).

**Imports** (sample — 60 total): `revered_spirit_ash.png`, `scadutree_fragment.png`, 7 DLC crafting materials (`frozen_maggot`, `ghostflame_bloom`, etc.), 5 DLC incantations (`furious_blade_of_ansbach`, `dragonbolt_of_florissax`, etc.), 6 sorceries (`rain_of_fire`, `glintstone_nail`, etc.), 5 Prattling Pates, ~15 DLC quest items (cookbooks, keys, maps, notes), various others.

**Not committed:** `missing_icons.txt` (build artifact — regenerated on demand by re-running the script).

#### Part 2 — DLC flag sweep across entire item database

**Goal:** Mark every Elden Ring DLC (Shadow of the Erdtree) item with `Flags: []string{"dlc"}` so future UI can filter. Previously only 16 entries had `"dlc"` (the 14 added in `feat/add-missing-tools-63` plus 2 Crystal Tears in `key_items.go`).

**Implementation:** Subagent ran a one-shot Go script (`/tmp/mark_dlc.go`) that:
1. Loaded all DLC reference files from `tmp/repos/er-save-manager/.../DLC/**/*.txt` and built a decimal-ID → category map.
2. For each backend item, computed decimal from hex (after stripping the per-category prefix), looked up in DLC set, edited Flags field if matched.
3. Preserved existing flags — when an entry already had flags, appended `"dlc"` to the slice; otherwise added `Flags: []string{"dlc"}`.
4. Skipped entries already containing `"dlc"`.

**Per-file count: 701 new `"dlc"` markers** distributed:

| File | +dlc | File | +dlc |
|---|---:|---|---:|
| `ashes.go` | +220 | `arms.go` | +29 |
| `weapons.go` | +73 | `incantations.go` | +27 |
| `tools.go` | +72 | `aows.go` | +25 |
| `helms.go` | +43 | `sorceries.go` | +15 |
| `chest.go` | +43 | `ranged_and_catalysts.go` | +11 |
| `key_items.go` | +43 | `shields.go` | +10 |
| `talismans.go` | +39 | `crafting_materials.go` | +9 |
| `legs.go` | +30 | `arrows_and_bolts.go` | +5 |
| | | `gestures.go` | +5 |
| | | `bolstering_materials.go` | +2 |

Plus 16 pre-existing `"dlc"` flags = **717 total DLC entries marked**.

**Prefix conventions discovered (verified against reference):**
- `weapons.go`/`shields.go`/`arrows_and_bolts.go`/`ranged_and_catalysts.go`: **no prefix** — backend stores raw decimal as hex (e.g. `0x0016E360` = decimal 1500000 = "Main-gauche").
- `helms.go`/`chest.go`/`arms.go`/`legs.go`: prefix `0x10000000`.
- `talismans.go`: prefix `0x20000000`.
- `aows.go`: prefix `0x80000000`.
- All Goods (`tools.go`/`key_items.go`/`ashes.go`/`crafting_materials.go`/`bolstering_materials.go`/`cookbooks.go`/`gestures.go`/`sorceries.go`/`incantations.go`): prefix `0x40000000`.

**Flag merge cases:** 1 entry — `tools.go` `Keep Wall Key` → `Flags: []string{"cut_content", "ban_risk", "dlc"}` (was `cut_content, ban_risk`).

**Skipped:** `cookbooks.go` (uses `CookbookData` struct keyed by event-flag IDs, not item IDs — no Flags field). `gestures.go` `AllGestures` slice (`GestureDef` struct, separate from the `Gestures` ItemData map; agent only modified the latter).

**Tests:** `go build ./backend/...` ✅, `go test ./backend/db/...` ✅, `go test ./tests/roundtrip_test.go` ✅, `go vet ./backend/...` ✅.

**App impact:** Zero — `Flags` slice is data-only until UI work consumes it. The new `"dlc"` flag is a marker for future filter functionality (per user direction in this audit cycle: *"chcę mieć te informacje w db, mogą być na razie jako komentarze, później zaimplementujemy w apce"*).

### Branch: feat/add-missing-tools-63 — Add 63 missing consumables/tools/tears + introduce `dlc` flag

**Goal:** Item-DB audit (`tmp/items_audit_report.md`) flagged 63 items present in `er-save-manager` reference but absent from our backend `tools.go` (and 2 DLC Crystal Tears that belong in `key_items.go` per project convention). Filling these gaps unblocks players who want to add them via the Item Database UI.

**Cross-validation:**
- All 63 IDs converted from reference decimal → hex with `0x40000000` prefix and verified.
- Icons confirmed present for 54 of 63 entries (sampled `find frontend/public/items -iname "<keyword>*"`).
- 9 Prattling Pates have no icons yet — placeholder paths `items/tools/multiplayer/prattling_pate_*.png` written so a later icon import slots in without code change. Frontend `DatabaseTab` already gracefully handles missing icons (broken-icon "?" placeholder).
- Empty Crimson/Cerulean flask variants (26) reuse the filled-flask icons (filled flasks already share one icon across all upgrade levels).

**Change:**
- `backend/db/data/tools.go` — added 61 entries:
    - Batch 1 (12) — base game consumables: Boiled Prawn, Neutralizing/Thawfrost/Preserving/Rejuvenating/Clarifying Boluses, Pickled Turtle Neck, Gold-Pickled Fowl Foot, Soft Cotton, Baldachin's Blessing, Poison Spraymist, Acid Spraymist.
    - Batch 2 (27) — empty flask variants: Wondrous Physick (Empty), Crimson Tears (Empty) +0..+12, Cerulean Tears (Empty) +0..+12.
    - Batch 3 (9) — Prattling Pates: Hello / Thank you / Apologies / Wonderful / Please help / My beloved / Let's get to it / You're beautiful / Lamentation (DLC).
    - Batch 4 (13) — DLC consumables: Lulling Branch, Dragonscale Flesh, 5 Pickled Livers (Spellproof/Fireproof/Lightningproof/Holyproof/Opaline), Well-Pickled Turtle Neck, Sacred Bloody Flesh, Silver/Golden Horn Tender, Innard Meat, Horned Bairn.
- `backend/db/data/key_items.go` — added 2 entries: Bloodsucking Cracked Tear (`0x401EAFAA`), Glovewort Crystal Tear (`0x401EAFB4`). Routed to `key_items.go` (not `tools.go` per ref) because the project's existing Crystal Tears all live in `key_items.go` with icons under `items/key_items/`.

**New flag value `"dlc"`:** introduced for the 14 DLC entries added in this branch (batches 3-DLC + 4 + 2 Crystal Tears). Existing flag vocabulary was `cut_content`, `ban_risk`, `dlc_duplicate`, `pre_order`. The new `dlc` flag marks legitimate DLC content (not duplicates) — used for filtering in future UI work. **No app changes in this branch** — flag is data-only for now.

**Defaults applied (all new entries):**
- Most consumables: `MaxInventory: 99, MaxStorage: 600` (matches existing convention for Stanching Boluses, Pickled livers, etc.).
- Wondrous Physick (Empty): `1 / 0` (matches filled variant).
- Crystal Tears: `1 / 0` (matches existing Crystal Tear entries in `key_items.go`).
- Empty flasks (Crimson/Cerulean): `99 / 600` (matches filled-flask convention; quirky for "1 sacred flask total in game" but not changing existing pattern in this branch).

**Intentionally NOT done in this branch:**
- 101 "category mismatches" reported by audit (Remembrances, Maps, Notes, Letters classified as `Consumables.txt` in reference but as `key_items.go` in our backend). Our project aligns categories with the in-game UI Inventory tabs; reshuffling would diverge from UI. **Confirmed false-positive.** Memory updated.
- 2 spell category swaps reported by audit (`Death Lightning`, `Night Maiden's Mist`). Verified against Fextralife: backend was already correct — the reference `Magic.txt` tags are wrong for these entries. **Confirmed false-positive.** Memory updated.
- 3 cut-content entries flagged in `gestures.go` (`?GoodsName?`, Carian Oath, Fetal Position) are intentionally retained with `cut_content`+`ban_risk` flags per documented design. **Confirmed false-positive.**

**Tests:** `go build ./backend/...` ✅, `go test ./backend/db/...` ✅, `go test ./tests/roundtrip_test.go` ✅.

### Branch: feat/add-missing-armor-12 — Add 12 base-game armor pieces missing from item DB

**Goal:** Item-DB audit (`tmp/items_audit_report.md`) flagged 12 base-game armor entries present in `er-save-manager` reference but absent from our backend. All 12 already had matching icon assets shipped under `frontend/public/items/{head,chest,arms}/` from a previous icon import — only the Go entries were missing.

**Cross-validation:**
- All 12 IDs verified by computing `decimal // 0x10000000 prefix` from the audit report.
- Each item belongs to a known set whose other pieces already exist in our DB (set-completion check):
    - **Scaled** set — `Scaled Armor` (chest) was missing; `Scaled Helm`, `Scaled Armor (Altered)`, `Scaled Gauntlets`, `Scaled Greaves` already present.
    - **Sanguine Noble** set (DLC) — `Hood` (head) + `Robe` (chest) missing; `Waistcloth` (legs) already present.
    - **Fire Prelate** set — `Helm` was missing; chest/arms/legs present.
    - **Elden Lord** set — `Crown` was missing; chest/arms/legs present.
    - **Depraved Perfumer** set — `Gloves` (arms) missing; headscarf/robe/trousers present.
    - **Prophet** set — `Blindfold` (head) + `Robe` (chest, base, non-altered) missing; `(Altered)` and `Trousers` present.
    - **Imp Head (Elder)** — joins the existing `Cat`/`Fanged`/`Long-Tongued`/`Corpse`/`Wolf`/`Lion` series.
    - **Mushroom** set — `Head` + `Body` missing; `Crown` (different item), `Mushroom Arms`, `Mushroom Legs` already present.

**Change:**
- `backend/db/data/helms.go` — added 7 entries: Great Horned Headband (`0x100493E0`), Sanguine Noble Hood (`0x1004E200`), Fire Prelate Helm (`0x10057E40`), Elden Lord Crown (`0x100704E0`), Prophet Blindfold (`0x100975E0`), Imp Head (Elder) (`0x10108E48`), Mushroom Head (`0x10113E10`).
- `backend/db/data/chest.go` — added 4 entries: Scaled Armor (`0x100138E4`), Sanguine Noble Robe (`0x1004E264`), Prophet Robe (`0x10097E14`), Mushroom Body (`0x10113E74`).
- `backend/db/data/arms.go` — added 1 entry: Depraved Perfumer Gloves (`0x10083E28`).

All entries: `MaxInventory: 1, MaxStorage: 1, MaxUpgrade: 0`, `Category: "head"|"chest"|"arms"`, IconPath references the existing PNG that was already shipped.

**Tests:** `go build ./backend/...` ✅, `go test ./backend/db/...` ✅, `go test ./tests/roundtrip_test.go` ✅.

### Branch: fix/wails-dev-restart-loop — UI preview mode without save + embed dotfile fix

**Two unrelated changes shipped together on the same branch** (per user direction).

#### Part 1 — Wails dev restart-loop fix

**Goal:** `make dev` window kept closing and reopening on its own, even without code changes. Logs showed Vite HMR reload + `[AssetServer] Unable to write content index.html: request has been stopped` × N + `Done.`, then app restart.

**Root cause:** `main.go:13` had `//go:embed all:frontend/dist`. The `all:` prefix includes hidden files (dotfiles). `frontend/dist/` contains 6 `.DS_Store` files which macOS Finder/Spotlight touch periodically. `wails dev` watches the embed source tree — every `.DS_Store` mtime bump triggered a Go rebuild + app restart cycle.

**Change:**
- `main.go:13`: `//go:embed all:frontend/dist` → `//go:embed frontend/dist`. Without `all:`, Go's default embed semantics exclude `.` and `_` prefixed files. Maps, icons, `index.html`, JS/CSS bundles are normal files — still embedded. Production binary identical in content (118 MB after rebuild).

**Why not delete `.DS_Store`:** macOS Finder recreates them on first directory browse — would not be a stable fix. The embed-directive change makes it irrelevant whether they exist.

#### Part 2 — Preview mode without loaded save

**Goal:** With no save file loaded, the editor previously showed all 5 tabs but each one was an empty "No Save File" placeholder. Wanted: 3 tabs (Character / Inventory / Settings) with view-only content, so users can browse the appearance presets and item database before opening a save.

**Change:**
- `frontend/src/App.tsx`:
    - `tabs` array gated on `platform`: 5 tabs when save loaded, 3 tabs (`character`, `inventory`, `settings`) otherwise. Hides `world` and `tools` until a save is loaded.
    - Replaced the centered "No Save File" placeholder with a slim "Preview mode — load a save file to enable editing" banner + `Open Save` button on top of the tab content.
    - In preview mode, `character` renders `<AppearanceTab readOnly />`, `inventory` renders `<DatabaseTab readOnly />`. `settings` continues to use the existing branch unchanged (Steam ID already self-disables when `!platform`).
- `frontend/src/components/AppearanceTab.tsx`:
    - New `readOnly?: boolean` prop. When `true`: skip `GetFavoritesStatus()` call (avoids backend error), hide preset checkbox overlay, hide action buttons block (Apply / Add to Mirror), hide existing Mirror Favorites section. Image preview/zoom remains active. Description text adjusted to "Click image to preview. Load a save file to apply presets to a character."
- `frontend/src/components/DatabaseTab.tsx`:
    - New `readOnly?: boolean` prop. When `true`: omit checkbox column from header and rows, hide "Add Selected (N)" button, adjusted `colCount` accordingly. Search, sort, category filter, ItemDetailPanel preview all still work.

**Notes:**
- `AppearanceTab.tsx` was previously orphan code (imported nowhere). Now wired into App.tsx for the preview Character tab.
- `DatabaseTab` is the same component used inside the loaded-save Inventory tab (database sub-view), reused in both modes — single source of truth.
- No backend / Wails-binding changes. No regression risk in save-loaded code paths (the existing 5-tab branch is untouched).

**Tests:** `make build` ✅ (frontend tsc + vite build + go compile + macOS package). Manual UI verification done by user.

### Branch: fix/gourmet-scorpion-stew-limits — Correct stack limits for Gourmet Scorpion Stew

**Goal:** Fix MaxInventory / MaxStorage for `0x401E8933` Gourmet Scorpion Stew. Database had `99 / 600` (default consumable values) — actual game limits are `1 / 1`. Bug originated when the entry was first imported with placeholder defaults; never verified against wiki.

**Cross-validation:**
- **Fextralife wiki** (Gourmet Scorpion Stew page): "You can hold up to 1 in inventory" + "You can store up to 1 in your item box" — explicit non-stackable. Note from page also distinguishes regular Scorpion Stew (which sends overflow to storage) vs Gourmet (strict 1-per-location).
- Symmetric with regular Scorpion Stew (`0x401E8932`, also `1 / 1`) added in previous commit `ac18cd7`.

**Change:**
- `backend/db/data/tools.go`: `0x401E8933` Gourmet Scorpion Stew — MaxInventory `99 → 1`, MaxStorage `600 → 1`. Other fields unchanged.

**Tests:** `go build ./backend/...` ✅, `go test ./backend/...` ✅, `go test ./tests/roundtrip_test.go` ✅, `npx tsc --noEmit` ✅.

### Branch: feat/add-scorpion-stew — Add missing regular Scorpion Stew

**Goal:** Add `0x401E8932` "Scorpion Stew" (regular). We already had Gourmet variant (`0x401E8933`); the canonical non-Gourmet was missing.

**Cross-validation:**
- **er-save-manager**: `DLC/DLCGoods/DLCConsumables.txt:37` — `2001200 Scorpion Stew`. `2001200 dec = 0x1E8932` → `0x401E8932`.
- **Elden-Ring-Save-Editor** (Final.py): `goods.json:33` — `"Scorpion Stew": "32 89 1E B0"` → matches.
- **Fextralife wiki:** Item Type "Consumable", +10% physical damage negation + 8 HP/s regen for 60s, MaxInv 1, MaxStorage 1. Obtained from Hornsent Grandam (infinite supply on revisit).

**Change:**
- `backend/db/data/tools.go`: added `0x401E8932: {Name: "Scorpion Stew", Category: "tools", MaxInventory: 1, MaxStorage: 1, MaxUpgrade: 0, IconPath: "items/tools/consumables/scorpion_stew.png"}` directly above the existing Gourmet entry. Icon already shipped.

**Intentionally NOT added — ESM duplicate IDs:**
- `0x401E8934` (ESM `2001202 Scorpion Stew`) and `0x401E8935` (ESM `2001203 Gourmet Scorpion Stew`) appear in `AllGoods.txt` and `DLCConsumables.txt` but have no Fextralife page or distinct in-game role. Likely cut content / quest-state variants. Adding them blindly would pollute the Item Database UI; defer until function is verified.

**Counts after:** `tools.go` 291 (+1).

**Tests:** `go build ./backend/...` ✅, `go test ./backend/...` ✅, `go test ./tests/roundtrip_test.go` ✅, `npx tsc --noEmit` ✅.

### Branch: feat/add-blessing-of-marika — Add missing DLC consumable

**Goal:** Add `0x401E8804` "Blessing of Marika" to the item database. Item is missing from our DB despite being a known DLC consumable (Shadow of the Erdtree); icon already shipped at `frontend/public/items/tools/consumables/blessing_of_marika.png` from a previous icon import.

**Cross-validation:**
- **er-save-manager** (priority 1): `DLC/DLCGoods/DLCConsumables.txt:28` — `2000900 Blessing of Marika`. `2000900 dec = 0x1E8804`; with our `0x40000000` base prefix → `0x401E8804`.
- **Elden-Ring-Save-Editor** (Final.py): `goods.json:22` — `"Blessing of Marika": "04 88 1E B0"`. Bytes read LE = `0xB01E8804`; strip top nibble (item-type marker) → `0x01E8804` → matches.
- **ER-Save-Editor** (Rust): not in DB (predates SoE additions). Not blocking — two refs agree.
- **Fextralife wiki:** Item Type "Consumable", full HP restore + clears all status ailments, 3 per playthrough (Church of Consolation / Fort of Reprimand / Two Tree Sentinels in Scaduview), no respawn at Grace. MaxInventory 1, MaxStorage 600.

**Change:**
- `backend/db/data/tools.go`: added `0x401E8804: {Name: "Blessing of Marika", Category: "tools", MaxInventory: 1, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/tools/consumables/blessing_of_marika.png"}` next to other DLC consumables in the `0x401E88xx` block.

**Counts after:** `tools.go` 290 (+1).

**Tests:** `go build ./backend/...` ✅, `go test ./backend/...` ✅, `go test ./tests/roundtrip_test.go` ✅, `npx tsc --noEmit` ✅.

### Branch: fix/call-of-tibia-category — Revert Call of Tibia mismove

**Goal:** Undo the mistaken move of `0x401E90CE` Call of Tibia from `tools.go` → `incantations.go` in `0834d8b`. The previous fix justified the move with "mirrors prior Furious Blade of Ansbach fix in 1ad864e" — but the precedents do not match. Furious Blade of Ansbach has `IconPath: items/incantations/...` (data self-consistent → keep in incantations). Call of Tibia has `IconPath: items/tools/consumables/call_of_tibia.png` and is listed in `er-save-manager/.../DLC/DLCGoods/DLCConsumables.txt:84` — both sources say "DLC consumable". In game it is a Sky Chariot summon item dropped by Tibia Mariner, used from inventory. Not an incantation.

**Change:**
- `0x401E90CE` Call of Tibia: `incantations.go` → **`tools.go`** with `Category: "tools"` (icon path unchanged, was already `tools/consumables/`).

**Counts after:** `tools.go` 289 (+1), `incantations.go` 128 (−1).

**Tests:** `go build ./backend/...` ✅, `go test ./backend/...` ✅, `go test ./tests/roundtrip_test.go` ✅, `npx tsc --noEmit` ✅.

### Branch: docs/spec-map-reveal — Map reveal documentation overhaul

**Goal:** align `spec/` with the actual `RevealAllMap` / `RemoveFogOfWar` / `SetUnlockedRegions` implementation. The old `spec/27-fog-of-war.md` advertised "fill bitfield 0xFF" as the recommended map-reveal path — the editor has not used that approach for months. `spec/11-regions.md` was a placeholder ("requires verification") and warned about a 10–20-region byte-shift crash that was eliminated by `RebuildSlot` in R-1 Step 14.

**Changes:**
- **Renamed** `spec/27-fog-of-war.md` → `spec/27-map-reveal.md` (git-tracked rename, history preserved). Rewritten as a 4-layer model: Unlocked Regions / Detailed Bitmap (event flags 62xxx + Map Fragment items + system flags 62000/62001/62002/82001/82002) / DLC Cover Layer (cross-ref `spec/29`) / FoW bitfield. Each layer maps to its `app.go` / `core` entry point. FoW is documented as `RemoveFogOfWar` (separate user action, not part of `RevealAllMap`) — corrected after audit found the function does exist and is wired into `WorldTab.tsx` / `WorldProgressTab.tsx`.
- **Rewrote** `spec/11-regions.md` — now documents the binary format precisely (`u32 count + count × u32`, little-endian), points at `core.SetUnlockedRegions` as the only supported entry point, lists what regions do (fast travel + multiplayer state) and explicitly what they do NOT do (no map texture, no FoW, no Cover Layer). Removed the outdated "max 10–20 regions" warning.
- **Updated** `spec/README.md` index — refreshed entry #11, added missing entries for #27, #29, #30.
- **Updated** code/doc references — `app.go::RemoveFogOfWar` comment + 3 `ROADMAP.md` mentions now point at `27-map-reveal.md`.

**Tests:** `go build ./backend/...` ✅, `go vet ./backend/...` ✅, `go build .` (root incl. `app.go`) ✅. Pre-existing `tmp/`-script build errors are unrelated to this change.



**Goal:** Cross-validate item category assignments across `key_items.go`, `tools.go`, `crafting_materials.go`, `bolstering_materials.go` against three independent sources — `er-save-manager`, `ER-Save-Editor` (Rust), `Elden-Ring-Save-Editor` (Final.py / Goods/*.txt) — and Fextralife wiki.

**Process:** Spawn agent for full cross-check → 91 candidate "miscategorisations" reported. Hand-verified each finding by grep + wiki lookup; agent's count was inflated ~10× — all runes (Golden, Hero's, Numen's, Lord's) live correctly in `bolstering_materials.go`, not scattered as agent claimed. Talisman Pouch confirmed as Key Item per wiki, not Upgrade Material per ESM. Tools/Consumables boundary kept arbitrary on our side (no `consumables.go` split).

**Real bugs found (6 entries):**

A) Internal Category mismatch (file vs `Category` field):
- `0x401E90CE` Call of Tibia — was in `tools.go` with `Category: "incantations"`. **Moved to `incantations.go`** (mirrors prior Furious Blade of Ansbach fix in `1ad864e`).
- `0x400000B6` Furlcalling Finger Remedy — was in `key_items.go` with `Category: "tools"`. **Fixed `Category` to "key_items"** (file is correct, field was wrong).

B) ESM-confirmed shifts (3 ref repos + wiki agree):
- `0x4000085C` Margit's Shackle: `key_items.go` → **`tools.go`** (ESM Tools.txt:1; tactical multiplayer/boss tool)
- `0x40000866` Mohg's Shackle: `key_items.go` → **`tools.go`** (ESM Tools.txt:2)
- `0x40000870` Pureblood Knight's Medal: `key_items.go` → **`tools.go`** (ESM Tools.txt:3; multiplayer summon tool)
- `0x40002005` Sewer-Gaol Key: `tools.go` → **`key_items.go`** (ESM KeyItems.txt:92; it's a door key)

**Counts after:** `tools.go` 288 (+1), `key_items.go` 388 (−2), `incantations.go` 129 (+1).

**Rejected agent suggestions (after verification):**
- ❌ Create `consumables.go` (245 items): Tools/Consumables boundary in-game is arbitrary; split would force frontend Item Database filter refactor with low ROI.
- ❌ Add 470 "missing" items: most are flask variants (already covered), DLC merchant junk, covenant duplicates.
- ❌ Audit 744 "extras": legit flask variants and already-flagged cut content.
- ❌ Move Talisman Pouch to consumables: wiki confirms it's a Key Item.

**Tests:** `go build ./backend/...` ✅, `go test ./backend/...` ✅, `go test ./tests/roundtrip_test.go` ✅, `npx tsc --noEmit` ✅, `make build` ✅.

### Branch: fix/bosses-data-correctness — Boss name/region accuracy

**Goal:** Cross-validate `bosses.go` (110 entries) against three independent sources — `er-save-manager` (Python, flag-based), `ER-Save-Editor` (Rust, arena-flag-based), and Fextralife wiki — and fix wording where the references plus wiki agreed our entry was wrong.

**Process:** Spawn agent for diff against both ref repos → 91 discrepancies found (6 MAJOR-NAME, 6 MINOR-NAME, 79 MINOR-REGION). Web-verified each MAJOR/MINOR-NAME case. Most apparent "errors" turned out to be ref-repo issues (e.g. `er-save-manager` ships "Lorretta" / "Bayle, the Dread" / "Spirit-Caller Snail" for the Spiritcaller Cave fight — all wrong per wiki). 79 region differences kept as-is: our specific dungeon names ("Tombsward Catacombs") are more useful than ESM's broad regions ("Weeping Peninsula").

**Corrected — `bosses.go` (5 entries):**
- `9210` "Crucible Knight Ordovis" → **"Crucible Knight & Crucible Knight Ordovis"** (Auriza Hero's Grave is a duo fight)
- `9238` "Crystalians" → **"Crystalian Duo"** (Academy Crystal Cave = Spear + Staff; consistent with `9265` Crystalian Duo)
- `9239` "Kindred of Rot" → **"Kindred of Rot Duo"** (Seethewater Cave has two)
- `9241` "Omenkiller & Miranda" → **"Omenkiller & Miranda, the Blighted Bloom"** (Miranda's full Fextralife name)
- `9246` "Putrid Crystalians" → **"Putrid Crystalian Trio"** (Sellia Hideaway has three; canonical name)

**Confirmed correct (no change despite ref-repo disagreement):**
- `9119` Loretta — wiki spells "Loretta" (one t)
- `9163` Bayle the Dread — no comma per wiki
- `9173` Godskin Apostle / Divine Tower of Caelid — wiki confirms tower-specific location
- `9248` Godskin Apostle & Noble (Spiritcaller's Cave) — Snail is summoner only, real fight is Apostle + Noble

**Tests:** `go build ./backend/...` ✅, `go test ./backend/...` ✅, `go test ./tests/roundtrip_test.go` ✅ (4/4), `npx tsc --noEmit` ✅.

### Branch: feat/dlc-spells-cleanup — DLC sorceries / incantations + miscategorisation cleanup

**Goal:** Add 10 missing DLC spells, move 1 spell to its correct category, and remove 5 historical miscategorisations of DLC spells that lived in `tools.go` and `key_items.go`.

**Verified via Fextralife wiki for every ID before merging.**

**Added — `incantations.go` (+6):**
- `0x401E9D1C` Furious Blade of Ansbach (was wrongly in `sorceries.go`)
- `0x401E9E7A` Aspects of the Crucible: Thorns
- `0x401E9F7E` Dragonbolt of Florissax — Dragon Cult incantation
- `0x401E9FD8` Bayle's Tyranny — Dragon Cult incantation
- `0x401EA0AA` Pest-Thread Spears
- `0x401EA2BC` Divine Bird Feathers

**Added — `sorceries.go` (+4):**
- `0x401E9614` Glintstone Nail (Finger Sorcery, Ymir)
- `0x401E961E` Glintstone Nails (Glintstone Sorcery, Ymir)
- `0x401E96DC` Blades of Stone (Gaius Remembrance Sorcery)
- `0x401EA17C` Cherishing Fingers (Finger Sorcery, Ymir)

**Removed — `sorceries.go` (-1):**
- `0x401E9D1C` Furious Blade of Ansbach (incantation, moved to `incantations.go`)

**Removed — miscategorised duplicates (-5):**
- `key_items.go`: `0x401EA17C` Cherishing Fingers (sorcery, never a key item)
- `tools.go`: `0x401E9614` Glintstone Nail, `0x401E961E` Glintstone Nails, `0x401E96DC` Blades of Stone (all sorceries, not throwables)
- `tools.go`: `0x401E9F7E` Dragonbolt of Florissax (incantation, not grease)

**Tests:** `go build ./backend/... ./` ✅, `go test ./backend/...` ✅, `make build` ✅.

### Branch: fix/console-ux — Gestures: free slots before write + hide ban-risk behind setting

**Follow-up #2 to the gesture bug.** User feedback after the previous build:
1. **First Unlock All did nothing** — save still held 44 legacy even-ID garbage entries plus 13 valid odd, leaving only 7 sentinel slots; backend errored with "no empty gesture slot available" and the frontend silently swallowed it. After Lock All cleared everything, a second Unlock All worked.
2. **Ban-risk gestures still visible** in the Gestures grid — user wants them gated behind the existing `showFlaggedItems` toggle (Tools / Settings).

**Changes:**
- `app.go`:
  - New helper `purgeUnknownGestures(slots)` — replaces any non-canonical slot value (legacy even garbage, unknown IDs) with the empty sentinel.
  - `SetGestureUnlocked` and `BulkSetGesturesUnlocked` now call `purgeUnknownGestures` first, freeing space so a single Unlock All on a save corrupted by older builds succeeds in one click.
  - Lock path simplified back to canonical IDs only (the purge already wiped legacy evens, no `id-1` extension needed).
- `frontend/src/App.tsx`: pipes the existing `showFlaggedItems` setting into `<WorldTab>`.
- `frontend/src/components/WorldTab.tsx`: new `WorldTabProps.showFlaggedItems`; computes `visibleGestures` (filter out `ban_risk` unless toggle is on) and uses it for both rendering and the Unlock All bulk list. Lock All still iterates all known gestures so the save gets wiped clean. Progress counter switched to `visibleGestures.length`.

**Tests:** `tsc --noEmit` ✅, `make build` ✅, `go build ./backend/... ./` ✅.

### Branch: fix/console-ux — Gestures: stop auto-recovering ghost unlocks + ban-risk filter

**Follow-up to the previous gesture fix.** User feedback after that build:
1. **Read still showed all 57 unlocked** — auto-`SanitizeGestureSlots` on every `GetGestures` call was rewriting in-memory legacy even IDs to odd, so the UI claimed the user had every gesture even though only ~13 were really unlocked from gameplay.
2. **Unlock All triggered ban-risk content** — Pre-order Rings, "The Carian Oath", "Fetal Position", "?GoodsName?" appeared in-game with placeholder `ICON` text, indicating cut content / pre-order entries that violate online anti-cheat.

**Changes:**
- `backend/db/data/gestures.go`: added `Flags []string` to `GestureDef`. Tagged 6 entries with `cut_content` / `pre_order` / `dlc_duplicate` plus a shared `ban_risk` flag (IDs 111, 193, 217, 221, 227, 233).
- `backend/db/db.go`: `GestureEntry` now carries `Flags`; `GetAllGestureSlots` propagates them.
- `app.go`:
  - `GetGestures`: removed sanitize-on-read. Only canonical (odd) IDs count as unlocked, matching what the game actually displays.
  - `SetGestureUnlocked` lock path: also clears the `(id-1)` even legacy slot.
  - `BulkSetGesturesUnlocked` lock path: `removeSet` includes both `id` and `(id-1)` for every odd ID, so Lock All wipes the array clean even when previous builds left even garbage behind.
  - Removed sanitize-on-write call (Lock All extension is sufficient and avoids silently re-adding gestures the user didn't ask for).
- `frontend/src/components/WorldTab.tsx`, `WorldProgressTab.tsx`:
  - Unlock All filters `g.flags?.includes('ban_risk')` so ban-risk entries are never bulk-added.
  - Lock All sends every known gesture (not just unlocked ones) to ensure legacy garbage gets cleared.
  - Each gesture row shows a ⚠ next to ban-risk entries with a tooltip explaining the risk.

**User flow after the fix:**
1. Reload save → UI shows only really-unlocked gestures (13 in user's slot 4).
2. Click Lock All → all 64 entries become sentinel, including legacy even garbage.
3. Click Unlock All → 51 safe gestures written; the 6 ban-risk ones must be toggled individually if the user truly owns them.

**Tests:** `go test ./backend/...` ✅, `tsc --noEmit` ✅, `make build` ✅.

### Branch: fix/console-ux — Gestures invisible in-game (root cause + auto-repair)

**Bug reported by user:** "Gesty się nie pojawiają pomimo tego że je dodałem w apce" — gestures unlocked via the editor were silently ignored by the game.

**Root cause:** The previous editor build encoded an "EvenID / OddID body-type variant" theory in `AllGestures`. In practice all vanilla gesture slot IDs are odd (verified against `er-save-manager/data/gestures.py`, which only writes odd IDs and is known to work). When the editor wrote the EvenID, the game silently ignored it, so up to 44/57 gestures became invisible in slots edited by previous builds.

**Diagnosis (`tmp/diag-gesture/main.go`):**
- User's slot 4 contained 13 odd (correct) + 44 even (broken) + 7 sentinel = 64 entries — matches "almost all unlocks invisible in-game" report.
- All 5 active slots had the same pattern.

**Fix:**
- `backend/db/data/gestures.go`: rebuilt `GestureDef` with a single canonical `ID` (always odd) plus the matching `ItemID` from er-save-manager. Removed `EvenID`/`OddID`, removed `DetectBodyTypeOffset`. Added `SanitizeGestureSlots(slots)` which rewrites any even slot whose `(id+1)` is a known gesture to `(id+1)` — silent, idempotent migration.
- `backend/db/db.go`: `GetAllGestureSlots` returns the new canonical ID.
- `app.go`: `GetGestures` runs `SanitizeGestureSlots` on the in-memory copy before computing unlock state, so the UI immediately reflects the repaired state. `SetGestureUnlocked` and `BulkSetGesturesUnlocked` sanitise the slot before any write so the next save commits the repair to disk. Removed `resolveGestureWriteID` and `gestureMatchesCanonical` (no body-type variants exist). Added `writeGestureSlots` helper.
- Cut-content / unknown DLC entries kept under their canonical IDs so saves containing them still display correctly.

**Tests:** `go test ./backend/...` ✅, `make build` ✅. Diag verifies sanitize repairs all 5 user slots from {13–45 known + many broken} → {57 known + 0 unmatched}. Manual in-game test required to confirm gestures now appear (user simply reopens the slot and toggles Unlock All / Lock All to commit the repair).

### Branch: fix/console-ux — BB refactor to backend-driven readOnly + ROADMAP cookbook sync

**Goal:** Drop the BB-specific Wails getter and frontend Set in favour of the same backend pattern already used for Cookbooks and Whetblades. ROADMAP updated to reflect that cookbook inventory sync is in fact already shipped.

**Changes:**
- `backend/db/data/bell_bearing_flags.go`: new `IsBellBearingItemID(id)` helper.
- `backend/db/db.go`: `GetItemsByCategory("key_items")` now also skips BB items.
- `backend/vm/character_vm.go`: `ReadOnly` is now true for BB items as well.
- `app.go`: removed `GetBellBearingItemIDs()` Wails method (no longer needed).
- `frontend/src/components/DatabaseTab.tsx`: reverted BB Set + filter (backend already hides them).
- `frontend/src/components/InventoryTab.tsx`: reverted BB Set + readOnly OR (VM already marks them ReadOnly).
- `ROADMAP.md`: cookbook entry — replaced “Known issue: physical item missing” with the actual implementation note (`CookbookFlagToItemID` + `IsCookbookItemID` + `ReadOnly` in VM).

**Tests:** `tsc --noEmit` ✅, `go test ./backend/db/data/...` ✅, `make build` ✅.

### Branch: fix/console-ux — Bell Bearing single source of truth (World tab)

**Goal:** Make Bell Bearings reachable from exactly one place — World → Unlocks → Bell Bearings — and keep the acquisition flag and the matching key item perfectly in sync.

**Changes:**
- `backend/db/data/bell_bearing_flags.go`: added auto-derived reverse map `BellBearingFlagToItemID` for the World tab toggle.
- `app.go`:
  - `SetBellBearingUnlocked` now calls new helper `syncBellBearingItem`: unlock → add 1 of the matching key item to inventory if absent; lock → remove from inventory and storage. Mirrors the Whetblade pattern.
  - `BulkSetBellBearings` runs the same sync per flag.
  - New Wails method `GetBellBearingItemIDs() []uint32` for the frontend to identify managed BB items.
- `frontend/src/components/DatabaseTab.tsx`: BB items are filtered out of the Item Database list (no Add path). Loaded via `GetBellBearingItemIDs` once on mount.
- `frontend/src/components/InventoryTab.tsx`: BB items appear in the Inventory list as `readOnly` — no Remove button, no selection checkbox — so users can preview but only manage them via World → Unlocks.

**Tests:** `tsc --noEmit` ✅, `make build` ✅, manual round-trip TBD (toggle ON adds BB, toggle OFF removes from inv+storage, no DB add path remains).

### Branch: fix/console-ux — Bell Bearing acquisition flag + ROADMAP sync

**Goal:** Round out the auto-flag-on-add hooks so Bell Bearings behave like Ashes of War (Twin Maiden Husks expand wares); sync ROADMAP with already-shipped Spirit Ash and AoW work.

**Changes:**
- `backend/db/data/bell_bearing_flags.go` (new): `BellBearingItemToFlagID` map (62 entries) — itemID → acquisition event flag, generated from `BellBearings` × `key_items.go` (59 exact name matches + 3 aliases for Kalé/Kale, Spell-Machinist, String-seller). Cut-content `Nomadic [11]` excluded.
- `backend/db/data/bell_bearing_flags_test.go` (new): coverage test verifying every non-cut-content BB key item is mapped and every flag exists in `BellBearings`.
- `app.go`: `AddItemsToCharacter` now also flips `BellBearingItemToFlagID[id]` after the AoW hook.
- `ROADMAP.md`: marked **Spirit Ash Upgrade Level Editing** ✅ (already shipped via `upgradeAsh` slider) and **AoW Acquisition Flag** ✅ (already shipped via `AoWItemToFlagID`). Split the old BB roadmap entry into shipped Acquisition flag (✅) and remaining Merchant Kill flag (🔲, RE-heavy follow-up).

**Tests:** `go test ./backend/...` ✅, `tsc --noEmit` ✅, `make build` ✅. Pre-existing `tests/bulk_add_test.go` failures (unrelated, GaItem array exhaustion) confirmed present on clean main.

### Branch: fix/console-ux — Quake console UX fixes

**Goal:** Eliminate three UX papercuts in the Quake console that hurt visibility during long-running operations.

**Changes:**
- `frontend/src/components/ToastBar.tsx`: render logs reversed (`logs.slice().reverse()`) so newest entry is on top — no auto-scroll needed, latest is always in view.
- Removed click-outside `useEffect` so the console stays open while user interacts with the rest of the UI. Toggle is now strictly via backtick or X button.
- Cleaned up stale `Spectral Steed Whistle duplicate` ROADMAP entry — the duplicate `0x40000082` no longer exists in `descriptions.go`; only `0x400000B5` (correct entry in `tools.go`) remains.

**Tests:** `tsc --noEmit` ✅, `make build` ✅, manual UI verification by user.

### Branch: feature/invasion-regions — Stage 2 (write support via R-1 full slot rebuild)

**Goal:** Implement write support for the per-slot Regions struct so players can unlock/lock invasion regions from the editor. Required a full slot rebuild because shift-based in-place patching corrupted saves (first attempt rolled back).

**Approach (Option B — full struct rebuild, see `PLAN-R1.md` for the 17-step checklist):**
- Replaced shift-based `core.SetUnlockedRegions` with sequential rebuild that re-serializes every section after `unlocked_regions` from typed Go structs, then zero-pads the tail to `SlotSize`.
- 19 new section types parsed and serialized: `RideGameData`, `BloodStain`, `MenuSaveLoad`, `TrophyEquipData`, `GaitemGameData` (7000 entries × 16B), `TutorialData`, `PreEventFlagsScalars`, `EventFlagsBlock`, 5× `SizePrefixedBlob` (`field_area`, `world_area`, `world_geom_man`×2, `rend_man`), `PlayerCoordinates`, `SpawnPointBlock` (version-gated), `NetMan`, `WorldAreaWeather`, `WorldAreaTime`, `BaseVersion`, `PS5Activity`, `DLCSection`, `PlayerGameDataHash`.
- Each section has a per-slot byte-for-byte round-trip test (`backend/core/section_*_test.go`).

**Key insight (`spec/30-slot-rebuild-research.md`):** Initial slack analysis was misleading — it assumed DLC was pinned at `SlotSize - 0xB2`. After full sequential parsing we discovered every slot has 408–432 KB of zero tail padding past the parsed sections, on both PS4 and PC. DLC and hash slide left/right naturally as `unlocked_regions` grows or shrinks; the tail rest absorbs the delta.

**New files:**
- `backend/core/section_io.go` — `SectionWriter` helper (mirrors `Reader`).
- `backend/core/section_types.go` — `FloatVector3`, `FloatVector4`, `MapID` primitives.
- `backend/core/section_world.go` — `RideGameData`, `BloodStain`, `WorldHead`.
- `backend/core/section_menu.go` — `MenuSaveLoad`, `TrophyEquipData`, `GaitemGameData`(+`Entry`), `TutorialData`.
- `backend/core/section_eventflags.go` — `PreEventFlagsScalars`, `EventFlagsBlock`.
- `backend/core/section_world_geom.go` — `SizePrefixedBlob`, `WorldGeomBlock`.
- `backend/core/section_player_coords.go` — `PlayerCoordinates`, `SpawnPointBlock`.
- `backend/core/section_netman.go` — `NetMan`.
- `backend/core/section_trailing.go` — `WorldAreaWeather`, `WorldAreaTime`, `BaseVersion`, `PS5Activity`, `DLCSection`, `TrailingFixedBlock`.
- `backend/core/section_hash.go` — `PlayerGameDataHash`.
- `backend/core/slot_rebuild.go` — `RebuildSlot` (sequential rebuild driver) + `SectionRange` / `buildSectionMap`.
- `spec/30-slot-rebuild-research.md` — slack analysis with 2026-04-26 update.
- `tmp/r1-stagedeck/main.go` — Steam Deck preflight CLI.

**Modified:**
- `backend/core/writer.go`: `SetUnlockedRegions(slot, ids)` now dedupe+sort, call `RebuildSlot`, replace `slot.Data`, refresh dynamic offsets. Rolls back on error.
- `backend/core/structures.go`: `SaveSlot.SectionMap` populated during `Read()` for use by `RebuildSlot`.
- `app.go`: `SetRegionUnlocked(slotIdx, regionID, unlocked)` and `BulkSetUnlockedRegions(slotIdx, regionIDs)` Wails methods.
- `frontend/src/components/WorldTab.tsx`: actionable checkboxes, per-area `+`/`−` quick-toggle buttons, global Unlock All / Lock All.

**Tests:** `go test ./backend/...` ✅ (incl. identity round-trip, mutation +50 regions PC, shrink -5 regions PS4, full Set→Save→Load→Get round-trip on both platforms). Manual Steam Deck verification ✅ (PS4 save loaded in-game, characters intact, grace/map/gestures preserved).

**Steam Deck test save:** `tmp/r1-stagedeck/oisis-r1-test-PS4.sl2` (380 + 81 regions across 2 slots).

### Branch: feature/invasion-regions — Stage 1 (read-only)

**Goal:** Surface the per-slot Regions struct (count + u32 IDs) in the UI so players can see which map areas are unlocked for invasions / blue summons. Stage 1 is read-only; Stage 2 will add write support (variable-size slot rebuild).

**Changes:**
- `backend/core/structures.go`: Added `SaveSlot.UnlockedRegionsOffset` and `UnlockedRegions []uint32`; parser populates the list during `Read()`.
- `backend/db/data/regions.go` (new): Ported 78 region IDs from `er-save-manager/data/regions.py` — Limgrave, Liurnia, Altus/Mt. Gelmir, Caelid, Mountaintops, Underground, Farum Azula, Haligtree, Land of Shadow (DLC), and legacy dungeon aliases. Each entry has `Name` + `Area` for grouping. Helper `IsDLCRegion()`.
- `backend/db/db.go`: New `RegionEntry` type and `GetAllRegions()` returning all known regions sorted by Area then Name.
- `app.go`: `GetUnlockedRegions(slotIdx)` Wails binding — merges the database with the slot's unlocked list.
- `frontend/src/components/WorldTab.tsx`: New "Invasion Regions" accordion in the Unlocks sub-tab. Per-area expand/collapse (matching Summoning Pools/Graces). Read-only badge + tooltip on checkboxes.
- `frontend/wailsjs/go/{main,models}`: Auto-regenerated bindings (added `RegionEntry` + `GetUnlockedRegions`).

**Tests:** `go test ./backend/...` ✅, round-trip PS4/PC/conversion ✅, `tsc --noEmit` ✅, `make build` ✅. Manual verification by user — unlocked regions match in-game progress.

### Branch: feature/database-tab-owned-counts — owned/max counts in Item Database

**Goal:** Show players how many of each item they currently own (in inventory and storage) and the per-slot max, directly in the Item Database tab — without switching to Owned Items.

**Changes:**
- `backend/vm/character_vm.go`: Added `BaseID` field to `ItemViewModel` so the frontend can match upgrade/infusion variants of the same weapon back to its base DB entry.
- `frontend/src/components/DatabaseTab.tsx`:
  - Fetches character via `GetCharacter` (refreshes on `inventoryVersion` bump).
  - Builds a `Map<BaseID, {inv, storage}>` of owned counts (sums stack quantity for stackable, counts copies for non-stackable).
  - New columns **Inventory** and **Storage** rendered as `owned / max` in every view (All Categories + per-category).
  - **Category** column forced visible in "All Categories" regardless of column-visibility setting.
  - Color coding: gray = 0, green = owned, amber = at/over max.
- `frontend/src/App.tsx`: Passes `inventoryVersion` to `<DatabaseTab>` for live refresh after Add/Remove.
- `frontend/wailsjs/go/models.ts`: Auto-regenerated bindings (added `baseId`).

**Tests:** `go test ./backend/...` ✅, round-trip PS4/PC ✅, `tsc --noEmit` ✅, `make build` ✅.

### Branch: fix/dlc-map-reveal-v2 — DLC black tile removal (SOLVED)

**Problem:** DLC "Shadow of the Erdtree" map had persistent black tiles that could not be removed via event flags, FoW bitfield, map items, or any known mechanism.

**Root cause:** The DLC map cover layer is controlled by two position records in the BloodStain section (afterRegs+0x0088..0x0110). These records contain DLC-area coordinates that tell the game the player has physically explored the DLC map. Without them, the game renders black tiles over the DLC map regardless of all other flags.

**Solution:** Write synthetic DLC coordinates into the BloodStain section:
- Record 1 (afterRegs+0x008D): X=9648.0, Y=9124.0, flag=0x01
- Record 2 (afterRegs+0x00C5): X=3037.0, Y=1869.0, Z=7880.0, W=7803.0, flag=0x01

**Changes:**
- `app.go`: `revealDLCMap()` — added Phase 3 (DLC black tile removal via synthetic coordinates)
- `backend/core/offset_defs.go`: Added `DLCTile*` constants for BloodStain position offsets
- `backend/db/data/maps.go`: Added 237 dungeon map flags (62100-62999) to `MapVisible`, updated `IsDLCMapFlag()` range
- `spec/29-dlc-black-tiles.md`: Full research documentation with binary search results

**Testing:** 20 iterative tests on Steam Deck, confirmed working with base game + DLC map fully revealed.

### Branch: fix/dlc-map-reveal (experimental, not merged)

Deep research into DLC (Shadow of the Erdtree) map black tiles removal.

**Research findings (2025-04-25):**
- DLC ownership confirmed — not a runtime entitlement check
- Game resets ALL map event flags on load, then rebuilds from ground truth
- DLC map fragment items survive game reset (persist in inventory)
- FoW bitfield is shared between base game and DLC (same 2099-byte range)
- CsDlc bytes[3-49] are NOT always zero (contradicts earlier spec)
- Event flags 62080-62084 survive game load when set alongside grace flags
- **Black tiles persist despite correct flags, items, regions, and graces**

**Approaches tested (all failed to remove black tiles):**
1. Event flags 62080-62084 + 82002 only
2. Above + DLC map fragment items
3. Above + CsDlc byte[1]=1 (DLC entry flag)
4. Above + story progression flags
5. FoW bitfield extension beyond 0x10B0
6. er-save-manager flag toggle (same flags, no items/FoW)
7. Above + 105 DLC grace flags (72xxx, 74xxx, 76xxx) + acquisition flags 63080-63084
8. Above + 10 DLC region IDs (6800000-6941000) via byte insertion

**Code changes (on branch, not merged):**
- `backend/db/data/maps.go`: Added `DLCGraces` (105 DLC grace flags), `DLCRegions` (10 DLC overworld region IDs), fixed `IsDLCMapFlag()` range (removed incorrect 62800-62999)
- `backend/core/writer.go`: Added `AddRegionsToSlot()` — bulk region ID insertion with byte shifting and offset update
- `app.go`: Rewrote `revealDLCMap()` — now sets regions, graces, visibility flags, system flags, and items
- `spec/28-dlc-map-reveal.md`: Full research documentation

**Key remaining hypothesis:**
- CsDlc byte[1] (SotE entry flag) = 48 in reference save, 0 in ours. Game only sets it via proper DLC entry (Miquella's hand), not via teleport. This may be the master switch for DLC map rendering. Untested with current region/grace setup.

**Diagnostic tools created (tmp/, not committed):**
- `tmp/diag-dlc/main.go` — applies revealDLCMap to a save copy, compares with clean and reference slot
- Multiple Python analysis scripts used in-session for binary diff, flag scanning, region analysis
