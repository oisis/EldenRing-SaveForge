# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [1.5.7] - 2026-07-21

### fix(storage): preserve native allocation for direct item additions

Adding items to an empty Storage now follows the game's allocation pattern,
both for a single batch and for several separate Add operations in one editing
session. Newly added items remain visible after saving and restarting the game.

## [1.5.6] - 2026-07-21

### fix(save-manager): correct Steam ID handling for PC saves

Loading a PC Steam save now shows the correct current Steam ID, and saving
writes it back to the right metadata offset instead of a stale location.

### fix(cookbooks): persist crafting unlocks in Key Items

Cookbooks are now saved as Key Items, and their crafting unlocks are kept
intact instead of being lost on save.

### fix(item-add): restore native counters for newly added items

Adding new Inventory items (goods, ammo, weapons, armor, talismans, and Ashes
of War) now updates the same internal counters the game itself maintains, so
newly added items show up correctly in-game instead of being invisible.

## [1.5.5] - 2026-07-19

### feat(diagnostics): complete Debug Mode coverage

Debug Mode now traces World, Advanced and Tools actions with the same
privacy-safe operation lifecycle used by Character and Game Items. Coverage
includes World progression changes, Network parameter updates, repair tools,
backups, diagnostic export, save-manager actions, deploy operations and local
save loading. Operation records use only closed action/status metadata, keeping
target names, paths, commands, account data and error text out of support logs.

## [1.5.4] - 2026-07-19

### feat(diagnostics): trace Character and Game Items mutations in Debug Mode

Debug Mode now records every changed Character field and Game Items operation
as a privacy-safe `before → planned → finished` lifecycle. Coverage includes
Database add/remove/unlock actions, Inventory Workspace edits and saves,
auto-repairs, and GaItem duplicate repair and optimisation. Finished records
read the real post-operation state, including error and rollback paths where
they exist, so support logs distinguish a plan from what actually landed.

Diagnostics redact PSN and account identifiers in addition to existing secret,
path, token, and credential protection.

### fix(database): correct standard weapon upgrade caps

Troll's Golden Sword, Crystal Knife, and Ghostflame Torch now use the correct
maximum upgrade level of +25.

## [1.5.3] - 2026-07-16

### feat(diagnostics): add durable, privacy-safe support logs

The new Diagnostics Console records application activity in a durable local
journal, so a crash or a report can be investigated without requesting a save
file. Debug Mode increases diagnostic detail while sanitising paths, account
identifiers, credentials, and other sensitive values. The console supports
searching, level filtering, scoped export, and recovery of an unclosed prior
session.

Save loading, automatic repair prompts and repair outcomes, item operations,
World, Advanced, Tools, backup, conversion, deployment, and save/export
operations now emit privacy-safe lifecycle events for support diagnosis.

### fix(assets): align item icons with item categories

Item icon paths now consistently use their item's category directory. Missing
dedicated artwork uses a neutral in-app placeholder, so every database item
has a valid icon asset.

## [1.5.2] - 2026-07-16

### fix(inventory): preserve canonical crafting-container records

Cracked Pot, Ritual Pot, Hefty Cracked Pot, and Perfume Bottle now update the
container record already present in the save. Vanilla container records in
Inventory Common Items are updated in place; legacy Key Items records created
by earlier SaveForge versions remain supported without creating another copy.
New containers are written to Common Items, so a full Key Items section no
longer blocks pot or perfume additions.

## [1.5.1] - 2026-07-16

### fix(save): preserve game-owned equip counters when adding items

Adding items now advances only the acquisition-sort counter. SaveForge leaves
the separate `NextEquipIndex` values untouched, matching game-written saves
and preventing a character-load crash after large weapon batches.

## [1.5.0] - 2026-07-15

### feat(gaitem): analyse and optimize GaItem allocation

Game Items now reports physical empty records, allocator cursor room, and
recoverable capacity before applying stable GaItem compaction. Optimisation is
transactional: it preserves non-empty records, their handles, and all
Inventory/Storage data, with preflight and postcondition validation.

### feat(gaitem): repair duplicate physical GaItem handles

A dedicated repair flow detects duplicate physical GaItem handles and lets you
keep a chosen record safely before running allocation optimisation.

### fix(save): write weapon and Ash of War data safely

Active GaItemData entries are now written using their real 8-byte layout.
Adding large batches of weapons uses even acquisition indices with a stride of
two in both Inventory and Storage, avoiding game-side acquisition-sort bucket
collisions and load crashes.

### fix(gaitem): allow safe optimisation of game-written index duplicates

GaItem optimisation no longer refuses a save only because the game wrote
duplicate full acquisition indices. Repack does not rewrite Inventory or
Storage, and preserves those records unchanged.

### fix(repair): apply weapon upgrade limits consistently

Repair actions clamp weapon upgrades through the central game-limit resolver.

## [1.4.1] - 2026-07-14

### fix(database): handle all crafting containers consistently

Perfume Bottle is no longer available as a manually addable Key Item in Item
Database, matching Cracked Pot, Ritual Pot, and Hefty Cracked Pot. All four
containers remain managed automatically when their associated craftable items
are added or edited.

## [1.4.0] - 2026-07-14

### feat(appearance): complete Type B preset support

Character Appearance presets now apply Type B characters correctly and can add
them to Mirror Favorites using verified game model IDs. Casca and Fire Keeper
are correctly classified as Type B. The same resolved appearance payload is
used for direct Apply and Mirror Favorites, including hair, eyelashes, and
tattoos or marks.

### feat(appearance): identify matching saved appearances

The Character profile now shows a preset name and thumbnail only when the
current appearance exactly matches one known preset. Mirror Favorites uses the
same exact recognition after an application reload; unknown in-game entries
remain labelled `In-game favorite` without a guessed thumbnail. Recognised
Mirror thumbnails enlarge on hover for easier inspection.

### change(appearance): remove Mirror Favorites undo

The separate Undo action for Mirror Favorites was removed. Adding or removing
a Mirror Favorite now takes effect directly; the existing global character Undo
continues to revert character changes only.

## [1.3.3] - 2026-07-13

### fix(inventory): keep pot and perfume containers consistent

Adding craftable pots or perfume items now creates or updates their matching
container in Inventory Key Items, rather than routing it to Common Items or
leaving an existing container quantity unchanged. Storage-only additions create
the required minimum of one container; larger Storage stacks do not consume
additional containers.

Editing pot or perfume quantities in `Character → Inventory` now updates the
matching container when `Save Changes` is pressed. The Inventory table refreshes
immediately after saving, so the new container quantity is visible without
changing tabs or filters.

## [1.3.2] - 2026-07-13

### fix(ui): simplify the Flask subcategory label

The Tools subcategory containing Flask of Wondrous Physick and the Crimson and
Cerulean Flask variants is now labelled simply `Flasks`.

### fix(db): separate base-game and DLC Larval Tears

Larval Tears now retain their distinct game records: the base-game item has a
Safe limit of 18 per playthrough, while the DLC item has its own `Larval Tear
(DLC)` entry and a Safe limit of 6. In Expanded Limits, both records use their
canonical game caps of 99 in Inventory and 600 in Storage, and adding one no
longer changes the other.

### fix(database): show expanded Remembrance stack quantities accurately

In Expanded Limits, Remembrances now show their actual owned quantities in the
Database tab (up to 99 in Inventory and 600 in Storage), rather than displaying
each legitimate stack as one item because Safe-mode caps are conservative.

## [1.3.1] - 2026-07-12

### fix(workspace): refine Sort Order navigation, scrolling, and default grouping

`Game Items → Weapons & Sort Order` is renamed to `Game Items → Sort Order`.
The Inventory and Storage grids now advance exactly one 5×6 card per wheel
gesture instead of occasionally skipping two cards.

Selecting `Default` in the shared Sort control now actively restores a grouped
weapon order in both grids: melee armaments, shields, then ranged weapons and
catalysts; known weapon classes remain contiguous within each section.

## [1.3.0] - 2026-07-11

### feat(workspace): card-based item ordering and atomic shared sorting

`Game Items → Weapon & Sort Order` now uses vertically scrolling 5×6 cards
instead of pagination. A full card keeps a trailing empty card for placement,
wheel movement advances a full card at a time, and same-container drag-and-drop
auto-scrolls at the frame edges to reach other cards. Empty cells are valid
reorder targets; Inventory↔Storage transfers still append to the destination.

The shared sorting control now reorders Inventory and Storage through one atomic
workspace operation. It validates complete UID permutations before mutation and
returns one final snapshot, eliminating the visible intermediate shuffling from
the former per-item move loop.

### feat(ui): clarify game item navigation and workspace controls

The top-level `Inventory` navigation is now `Game Items`; its former
`Equipment` view is now `Inventory`. Weapon & Sort Order uses compact Category
and Sort dropdowns, and its count headers identify `Storage` and `Inventory`.

### fix(save): reject ambiguous save containers safely

Opening a file now accepts only unambiguous native PC BND4 or native PlayStation
containers. Ambiguous or encrypted containers are rejected before load, retain
the currently open save, and show an `Unsupported Save Format` modal instead of
silently assigning the wrong platform. Format conversion remains visibly
disabled while it is unsupported.

## [1.2.0] - 2026-07-11

### feat(db): add subcategory filtering to the Item Database

The Database tab now supports subcategory filtering with dynamic column widths,
letting the name column flex to consume remaining space while numeric columns
stay compact. Sword Lance (and its Flame Art variant) is reclassified under
Heavy Thrusting Swords.

### feat(inventory): capacity strip and equipment layout refinements

The capacity strip is reordered to `INVENTORY | STORAGE`, drops the `ALL ITEMS`
readout, and colours the `used/max` figure by threshold (white below 80%, orange
80–95%, red above 95%) with subtle separators between entries. The Equipment
table adopts the Item Database column model — flexible name column, compact
no-wrap numeric columns.

### fix(inventory): cursor-aware GaItem capacity preflight

Add batches containing weapons/armor/AoW are now rejected before mutation when
they cannot fit from the allocator cursor. Capacity accounts for both total empty
GaItem records and remaining cursor room (`len(GaItems) - NextArmamentIndex`);
holes below the cursor remain untouched (no compaction or reuse). The add-capacity
result surfaces needed-vs-available GaItem counts for a clearer placement error.

### fix(repair): manual removal for unknown_handle_type

The Inventory Issues repair flow exposes the existing fingerprint-protected
`remove_record` action for `unknown_handle_type` records (inventory, key items,
and storage scopes). The default stays `leave_unchanged` and it is never
auto-removed by "repair all"; the UI makes clear the removal is irrecoverable.

## [1.1.0] - 2026-07-11

### fix(db): resolve scorpion stew item aliases

Scorpion Stew now exposes exactly two canonical picker rows — `Scorpion Stew`
(`0x401E8930`) and `Gourmet Scorpion Stew` (`0x401E8933`) — each carrying its FMG
item text. The two technical twin rows without text alias to their canonical
entries (`0x401E8932` → Scorpion Stew, `0x401E8931` → Gourmet Scorpion Stew) so a
save carrying the reported handle `0xB01E8931` resolves instead of scanning as
`unknown_item_id`, while the twins stay hidden from picker listings.

### fix(db): group flask upgrades by family identity

Upgraded Crimson/Cerulean Tears flasks keep their exact inventory identity (ID,
name, and BaseID — e.g. `Flask of Crimson Tears +12` stays `0x40000401`) but now
expose a `familyId` grouping key via `FlaskFamilyBaseID`, which collapses any
level, its even technical alias, or its goods handle to the +0 picker base
(`0x400003E9` / `0x4000041B`). The Database tab counts owned copies under
`familyId || baseId || id`, so an upgraded flask registers against its +0 picker
row instead of reading as unowned. Wondrous Physick is intentionally excluded;
`GetItemDataFuzzy` still returns exact flask IDs.

### fix(app): skip duplicate flask family adds

Adding the +0 Crimson/Cerulean picker row is now skipped when the save already
holds any level of that flask family in inventory or storage. The skip is
reported through `SkippedExisting`, the existing upgraded flask row is left
untouched, and capacity preflight is unaffected because skipped flasks never
reach the capacity item set. Non-flask stackables, weapons, armor, talismans,
key items, and Wondrous Physick are unaffected.

### fix(editor): honor sort order add quantity

Adding a record-based item (weapon, armor, talisman) with quantity greater than
one now materializes that many separate records — each `Quantity=1` with a fresh
collision-safe UID — inserted consecutively at the requested target position,
instead of a single stacked row. Quantity `0` still normalizes to one item, and
one-row stackable behavior is preserved for non-record categories.

### fix(ui): paginate sort order grids

The Sort Order grids page at 30 cells (5 columns × 6 rows); a full page spawns a
trailing empty page. Inventory and storage paginate independently, drag/drop
target positions account for the active page offset, and sort controls still
operate on the full tab list rather than the current page.

### feat(ui): add explicit safety profiles

The editor now uses a single Safety Profile selector instead of separate
Online Safety, Show Cut & Ban-Risk Items, and Chaos Mode toggles. The available
profiles are Safe, Expanded Limits, and Chaos, with Safe as the default.

Expanded Limits uses technical/game item caps for normal items while keeping
cut-content and ban-risk items hidden. Chaos uses technical/game caps and
reveals cut-content / ban-risk items behind the existing warning flow. The top
bar now shows the active profile consistently as SAFE MODE, EXPANDED LIMITS, or
CHAOS MODE, replacing the old global Online Safety banner.

### fix(core): preserve key item header on removal

`RemoveItemFromSlot` located the Key Items section without skipping the 4-byte
`key_count` header that sits between the common and key sections, so a removal
wrote the zeroed record 4 bytes early — clobbering the header and neighbouring
rows. The offset now includes `InvKeyCountHeader`, matching the parser.

### fix(vm): preserve key item handles on character save

Saving a character (`ApplyVMToParsedSlot`) wrote each Key Item's quantity onto
its own handle field due to the same missing `key_count` header skip. On the
next reload the corrupted handle no longer resolved, so Key Items appeared to
vanish after switching views. The write now targets the correct quantity field.

### fix(app): bump existing stackable key items

Full Chaos / Game Max now raises stackable Key Items already present in
`Inventory.KeyItems` (e.g. Lost Ashes of War, Larval Tear) to the requested
quantity in place, instead of skipping them as already owned. Non-stackable
owned items (cookbooks, prayerbooks) still skip; no duplicate `CommonItems`
records are created.

### fix(db): resolve technical item aliases, whetblade routing, scorpion stew

Technical item ID aliases now resolve to their canonical entries, whetblades are
exposed through unlock routing, the missing second Scorpion Stew row is added,
and stimulating boluses are classified as consumables.

### fix(inventory): model talisman copies without GaItem capacity

Adding talismans no longer falsely fails preflight as `gaitem_full`. Talismans are
handle-encoded and allocate no serialized `GaItem`, yet each copy is a distinct
quantity-1 physical inventory/storage record. A single shared classifier
(`classifyItemAdd`) now drives both `CheckAddCapacity` and `AddItemsToSlotBatch`,
so preflight counts N physical slots and zero `GaItem`s for N talismans — fixing
both the false `gaitem_full` rejection and the inverse risk of under-counting
physical slots for a single `ItemToAdd{InvQty: N}`. The redundant caller-provided
`ItemToAdd.IsStackable` flag was removed; classification is now derived purely
from the item ID (plus the explicit `ForceStackable` arrow override).

### fix(ci): stop publishing the standalone Linux AppImage as a release asset

The Linux job still bundles the AppImage inside its release zip; it is no longer
uploaded or attached separately. The release now ships only the per-platform zip
archives.

### feat(chaos): make Chaos Mode reach real game caps, reveal risk items, and warn before enabling

Chaos Mode now behaves as intended end to end. The DB owned-count columns use the
active mode's effective cap, so an item added to its regulation game limit (e.g.
Ancient Dragon Smithing Stone at 999) reads `999 / 999` instead of the previous
`999 / 18`. The artificial 99-copy ceiling on non-stackable items is removed in
both Normal and Chaos Mode; the add-quantity input is now bounded by free
inventory/storage slots (`GetSlotCapacity`), with `CheckAddCapacity` still the
hard all-or-nothing guard. Chaos Mode reveals all risk-flagged items
(`cut_content`, `ban_risk`, `pre_order`, `dlc_duplicate`) regardless of the "show
flagged" toggle.

Enabling Chaos Mode now shows a warning modal (OK / Cancel — Cancel does not
enable) stating that edits are irreversible (restore-from-backup only) and that
using it online is practically a guaranteed ban, with a default-on "create a
backup first" checkbox wired to the new `BackupCurrentSave` endpoint. While
active, the top navigation bar turns red with a pulsing "CHAOS MODE!!!" label.
Renamed "Full Chaos Mode" to "Chaos Mode" and corrected the stale
"bypasses all caps" comment.

### feat(inventory): on-load validation modal, auto-repair engine, diagnostics integration

Loading a save now automatically scans the selected character slot and shows an
inventory issues modal: workspace errors/warnings grouped with checkboxes,
per-issue Fix buttons, Fix-all per group, and a Repair Selected batch action.
Skipping the modal shows a toast pointing to Tools → Diagnostic.

New App endpoints back this: `ScanInventoryIssues` (unified scan combining the
binary corruption check `core.DiagnoseSaveCorruption` with workspace semantic
validation `editor.Validate`, returning per-issue repair metadata) and
`RepairInventoryWorkspaceItem` / `RepairInventoryWorkspaceItems` (apply
auto-repairs for `upgrade_out_of_range`, `pending_aow_unknown`,
`pending_aow_conflict` and commit via `SaveInventoryWorkspaceChanges`). The
reusable `editor.AutoRepairWorkspaceItem` engine drives both. Tools → Diagnostic
now also surfaces weapon workspace issues alongside the binary scan results.

### fix(db): register five legal technical variant item IDs

Five real GoodsParam rows observed in live saves (`0x40002AFA`, `0x40002AFC`,
`0x40002B08`, `0x40001FAD`, `0x40001FD2`) were missing from the item DB and so
raised false-positive `unknown_item_id`. They share their base item's params and
are now registered (flagged `no_database`, kept out of the item picker) so the
scanner recognises them while the add-item UI stays clean.

### fix(scanner): drop false-positive inventory_reserved rule for low acquisition indices

Removed the corruption check that flagged any record with `AcquisitionIndex <=
432`. Genuine game-created records legitimately use low indices (e.g. Memory of
Grace at 432, Lordsworn weapons/shields). `InvEquipReservedMax` is retained
purely as a conservative floor for newly generated editor indices — not a
validation rule for existing records. Duplicate-index detection
(`duplicate_acquisition_index`) is unaffected. Dropped the now-dead
`RepairCodeInventoryReserved` constant, its app-layer action mapping, and the UI
label; writer semantics are unchanged.

### fix(scanner): cap pots by owned container; goods storage uses maxRepositoryNum

Two remaining quantity/container false-positive classes fixed:

- **Pot/aromatic craftables** (Volcano Pot, etc.) are now capped by the owned
  container count (Cracked Pot, Ritual Pot, Perfume Bottle, Hefty Cracked Pot)
  rather than raw `maxNum`, matching the game's runtime container limit. Volcano
  Pot ×20 with 20 Cracked Pots no longer flags `quantity_above_max`. The scanner
  and clamp repair share the same `EffectiveQuantityCap` logic. A new
  **report-only** `container_overuse` aggregate flags when the total mapped to one
  container exceeds the owned count (several pot types overflowing a shared
  container) — no auto-repair.
- **Goods storage** now uses regulation `maxRepositoryNum` regardless of
  `isDeposit`. `isDeposit=0` is a deposit-prompt flag, not a storage prohibition,
  so items like Festering Bloody Finger (`maxRepositoryNum=99`) are no longer
  falsely reported as `item_not_allowed_in_container`. A known
  `maxRepositoryNum=0` remains a genuine prohibition. `game_limits_generated.go`
  was regenerated accordingly.

### fix(caps): separate conservative editor caps from game limits

Added generated `GameMaxInventory` / `GameMaxStorage` metadata sourced from
regulation data. Normal Mode keeps its existing conservative caps and NG+
scaling. Full Chaos Mode now uses technical game caps through a dedicated
backend endpoint instead of the previous frontend-only `999` override.

The loaded-save scanner and quantity clamp now use only known game limits.
Unknown limits are skipped rather than treated as zero. Goods storage uses
`maxRepositoryNum` (see the later "goods storage uses maxRepositoryNum" entry
in this release, which supersedes the earlier `isDeposit` gating); ammunition
uses `maxArrowQuantity` and the 600-unit repository limit. This removes false
positives for flask allocations, Festering Bloody Finger, duplicate spell
quantities, Prattling Pates, and Remembrances in storage. Full Crimson/Cerulean flask records correctly use their regulation
per-record cap of 20; the normal combined allocation of 14 remains a separate
aggregate rule.

### feat(repair): add safe quantity clamp action

Loaded-save repair can now fix over-cap inventory/storage quantities. The
scanner splits the old `quantity_above_max` condition into two codes:

- **positive cap** → `quantity_above_max`, offering `clamp_quantity` (default)
  and `leave_unchanged`. The new `ClampInventoryQuantityAt` primitive recomputes
  the authoritative cap at apply time (never trusting the frontend), preserves
  the raw high quantity bit, and updates both raw bytes and in-memory state;
- **zero cap** → `item_not_allowed_in_container`, offering `remove_record` and
  `leave_unchanged` (default). The quantity is never clamped to zero.

Cap semantics are unified in one exported `EffectiveQuantityCap` helper shared by
the scanner and the repair. The clamp runs through the existing repair pipeline
(fingerprint stale-check, undo snapshot, rollback, post-repair rescan) with an
added clamp-specific postcondition rejecting any result that still leaves
`quantity_above_max` or `quantity_zero` at the targeted row. No DTO shape or Wails
binding changed — the new issue-code and action-ID strings flow through existing
fields.

### docs(templates): record v2 items layout manual validation (Phase 8G)

Closes the Templates v2 items + layout milestone by recording the
manual validation pass the user reported as `manual OK`. Docs-only
update — no runtime code, no test code, no Wails bindings, no
`tmp/manual-validation/templates-v2/` change.

Manual validation gate was reported as passed by the user across the
items + layout matrix on the branch's current HEAD
(`feature/templates-v2-spell-writer-foundation`, commit
`4d1df246a68922b67e5c9cd1859ccdf08e117c05`). Validated paths:

- items addMissing direct apply,
- Apply with overrides… weapon level override on items,
- inventoryLayout reorderOnly,
- storageLayout reorderOnly,
- items + layout combined (items first, then layout),
- no active session guard on layout-bearing applies,
- persistence sanity: without `Save changes` the save file stays
  unchanged on disk; after `Save changes` the workspace state is
  persisted and survives an app restart + save reload.

The user did not report any fail case, and no further runtime
adjustment was requested.

Spec updates (EN + PL twins of `spec/56-templates-v2.md`):
- New row in §17a.1 "Manual validation log" recording the 2026-06-09
  pass on the items + layout matrix.
- New §17a.2l "Phase 8G items + layout manual validation flow
  exercised" subsection summarising the validated paths above. No
  fabricated numbers, no fabricated save slots, no fabricated
  platform / screenshot details — only what the user reported.
- §17a.2k "Non-goals deferred beyond Phase 8E.2" updated: the
  "Manual validation — deferred to Phase 8G" non-goal is now marked
  as completed (2026-06-09, `manual OK`), and the "Final merge to
  `main`" entry is restated as "pending final merge safeguards" now
  that Phase 8G has passed.

ROADMAP (`docs/ROADMAP.md`):
- Templates v2 "Done" list gains a one-line Phase 8G manual
  validation bullet (2026-06-09, `manual OK`).
- "Planned" header trimmed: Phase 8G no longer appears in the planned
  list. The remaining work for the milestone is the create-bridge
  (Phase 7d.4b), appearance preset (Phase 8), and multi-character
  pack (Phase 10); items + layout are now apply-complete and
  manually validated.

Validation in this phase is docs-only:
- `git diff --check` — clean (no whitespace / conflict markers).
- Public JSON regression sweep — grep confirms no docs claim a
  public JSON exchange surface exists; `JSON file dialog` / `Export
  JSON` etc. are absent from the new copy. The Phase 8A removal
  remains the authoritative statement.
- Heading parity across `spec/56-templates-v2.md` (EN) and
  `spec/lang-pl/56-templates-v2.md` (PL) — H2 / H3 counts match
  after the §17a.2l addition.
- `git diff --stat` — four files changed (`docs/CHANGELOG.md`,
  `docs/ROADMAP.md`, `spec/56-templates-v2.md`,
  `spec/lang-pl/56-templates-v2.md`).

Backend, frontend, tests, generated bindings, and the
`tmp/manual-validation/templates-v2/` scratch directory are
untouched. The branch is ready for final merge to `main` once the
usual merge safeguards (PR review / clean rebase against `main` /
final automated sweep) are agreed.

### docs(templates): document v2 items and layout apply (Phase 8F)

Cumulative docs/spec sweep that closes the Templates v2 items + layout
milestone (Phase 8A through Phase 8E.2) ahead of manual validation
(Phase 8G). No runtime code changes in this commit.

Spec updates (EN + PL twins of `spec/56-templates-v2.md`):
- New "Phase 8D.1 — backend v2 items apply (addMissing only)" section.
- New "Phase 8D.2 — UI items apply controls + weapon level override
  exposure" section.
- New "Phase 8D.3 — items apply polish (explicit addMissing on wire,
  warning grouping, result modal)" section.
- New "Phase 8E.1 — backend v2 inventory/storage layout apply
  (reorderOnly only)" section.
- New "Phase 8E.2 — UI layout apply controls + result modal layout
  counters" section.
- New "Cumulative summary after Phase 8E.2" section in §17a covering
  public exchange surface (YAML-only since 8A), items semantics
  (entryID + match key + duplicate-same-itemID), layout semantics
  (reorderOnly, extras preserved, missing skipped, sparse normalized,
  ambiguous resolved first-match-wins), apply behavior (session gate,
  items-first-then-layout ordering, weapon level override scope), the
  full warning code catalogue, and the explicit non-goals deferred
  beyond Phase 8E.2.

ROADMAP (`docs/ROADMAP.md`):
- Templates v2 "Done" entry extended with one-line summaries of
  Phase 8D.1 / 8D.2 / 8D.3 / 8E.1 / 8E.2 (matching the existing
  one-liner-per-phase style of the surrounding bullets).
- "Planned" Phase 7d.4b+ paragraph trimmed to reflect that items +
  layout no longer block the milestone — what remains is the
  create-bridge for spells / equipment, appearance via preset, and
  multi-character pack.
- New Phase 8G placeholder under "Planned" for the manual validation
  sweep (matrix specified in spec/56 §17a).

Validation in this phase is automated-only:
- `go test ./backend/... . -count=1` — green.
- `go test -race -run 'Test.*ApplyBuildTemplateV2' .` — green.
- `go vet ./backend/... ./` — clean.
- `go build ./backend/... ./` — green.
- `cd frontend && npx tsc --noEmit` — clean.
- `cd frontend && npx vitest run src/components/templates
   src/wails-bindings.contract` — green (295/295).

Manual validation across the real-save items + layout matrix is
intentionally deferred to Phase 8G per the workflow agreement.

### feat(templates): apply v2 layout reorder-only UI (Phase 8E.2)

Frontend-only flip that lifts inventory layout / storage layout from
export-only to apply-eligible (reorder-only). Backend Phase 8E.1
writer already handles the mutation; this phase only exposes it.

UI gating
(`frontend/src/components/templates/ImportTemplatePreviewModal.tsx`,
`TemplateLibraryModal.tsx`, `TemplatesShellModal.tsx`):
- `V2_APPLY_SUPPORTED_SECTIONS` now includes `inventoryLayout` and
  `storageLayout`. The `LAYOUT_ONLY_SECTIONS` short-circuit and the
  "no apply path in Phase 8D.2" tooltip are gone.
- Library `v2HasApplyableSections` widens to enable Apply on
  layout-bearing entries (items-less layout-only templates included).
- Old `library-apply-v2-layout-ignored` confirm-row copy is replaced
  by `library-apply-v2-layout-reorder-only`:
  - layout-only: "Layout mode: Reorder only — no items are added or
    removed. Layout reorders matching workspace items. Extras are
    preserved and appended; missing entries are skipped with
    warnings."
  - items + layout: same "Reorder only" header + "Missing items are
    added first, then layout is applied."
- Preview modal renders `import-preview-layout-reorder-only` for
  layout-only templates (four lines of plain-English semantics).

Session gate
(`frontend/src/components/templates/TemplatesShellModal.tsx`):
- The Inventory Edit Session requirement now also covers layout
  applies (layout writer reorders `sess.Workspace`). Layout-only
  templates require an active session; missing session surfaces the
  same `NO_SESSION_MESSAGE` toast as items and inventory.workspace.

Explicit defaults on the wire
(`frontend/src/components/templates/TemplatesShellModal.tsx`):
- `injectExplicitAddMissing` renamed to `injectExplicitApplyDefaults`
  and extended. Helper now writes
  `applyOptions.items.mode = "addMissing"`,
  `applyOptions.inventoryLayout.mode = "reorderOnly"`, and
  `applyOptions.storageLayout.mode = "reorderOnly"` whenever the
  corresponding section is selected. Backend defaults nil layout
  mode to `reorderOnly` (Phase 8E.1) — the UI just makes the intent
  testable from the JSON payload.
- Library routing: layout-bearing entries now go through
  `PreviewBuildTemplateFromLibrary` → JSON apply (so explicit modes
  ride the wire), mirroring the items routing from Phase 8D.3.
  Profile/stats/equipment/spells-only entries stay on the fast
  `ApplyBuildTemplateV2FromLibraryToCharacter` path.

Result modal extensions (`ApplyItemsResultModal`):
- Modal aria-label and headline change to "Template apply result".
- New "Layout reordered" section with six counters
  (`items-apply-result-layout-{inv,sto}-{applied,missing,extras}`)
  + a plain-English footnote explaining reorder-only semantics.
- Five new warning groups for layout codes:
  `items-apply-result-layout-missing` (amber),
  `items-apply-result-layout-ambiguous` (amber),
  `items-apply-result-layout-sparse` (muted/info),
  `items-apply-result-layout-extras` (muted/info),
  `items-apply-result-layout-mode-unsupported` (amber). All five
  carry `data-warning-severity="info"` because layout warnings are
  reporting, not blocking.
- Modal now opens for layout-only applies (items-only used to be
  the only trigger).

Tests:
- `src/wails-bindings.contract.test.ts` gains a contract test pinning
  the six `ApplyTemplateV2Result.layout*` fields so a binding rename
  would zero the counters loudly instead of silently.
- `__tests__/ImportTemplatePreviewModal.test.tsx`: two updated cases
  (items + layout copy, layout-only enabled with reorder-only copy)
  + tooltip text update.
- `__tests__/TemplateLibraryModal.test.tsx`: two updated cases (items
  + layout confirm row uses new copy; layout-only entry Apply
  enabled with reorder-only confirm row).
- `__tests__/TemplatesShellModal.test.tsx`: five new Phase 8E.2 cases
  (layout-only without session, layout-only with session + reorderOnly
  on the wire + result modal counters, items + layout both modes on
  the wire, library layout entry routes via JSON, result modal warning
  grouping severity).

Validation:
- `cd frontend && npx tsc --noEmit` — clean.
- `cd frontend && npx vitest run src/components/templates
   src/wails-bindings.contract` — 295 / 295 green.

Backend untouched. No new writers. No public JSON re-introduction.
No `frontend/wailsjs/runtime/*` content change.

### feat(templates): apply v2 layout reorder-only (Phase 8E.1)

Backend-side apply path for `sections.inventoryLayout` and
`sections.storageLayout` in `reorderOnly` mode. The UI piece lands
in Phase 8E.2.

Writer wiring (`app_templates_v2_apply.go`):
- `applyTemplateLayoutToWorkspace(sess.Workspace, container, section,
  mode)` reorders the workspace list in place. Mode allowlist is
  `reorderOnly` (and the nil/empty default — also resolved to
  `reorderOnly`); `ignore` is a documented no-op; `append`, `replace`,
  and any other unknown mode emit `layout_mode_unsupported` and
  skip that section.
- Match key is the same tuple the items resolver uses (itemID +
  category + upgrade + infusion + AoW + container), keyed by the
  template-local entryID. A LayoutEntry whose entryRef cannot resolve
  to a live workspace item emits `layout_entry_missing` and is
  skipped. Two live items matching the same entry emit
  `layout_entry_ambiguous`; the first live match wins so the apply
  remains deterministic.
- Sparse / non-zero-based positions are accepted and normalised to
  compact 0..N-1; the writer emits
  `layout_sparse_normalized` (info) when normalisation actually
  happens so producers see what changed.
- Live items not referenced by the template layout are NOT deleted.
  They are appended after the template-ordered prefix in their
  original relative order and the writer emits
  `layout_extra_items_preserved` (info) with the count.

Apply ordering (`app_templates_v2_apply.go`):
- Items addMissing runs **before** the layout writer. Newly added
  items become eligible for ordering inside the same apply.
- Layout writer dispatches inside the existing Phase 7a dual snapshot
  scope, so any writer failure rolls both `slot.Data` and the
  workspace value snapshot.

Result + summary plumbing (`backend/templates/import.go`,
`frontend/wailsjs/go/models.ts`):
- New issue codes
  `IssueCodeLayoutModeUnsupported = "layout_mode_unsupported"`,
  `IssueCodeLayoutEntryMissing = "layout_entry_missing"`,
  `IssueCodeLayoutEntryAmbiguous = "layout_entry_ambiguous"`,
  `IssueCodeLayoutSparseNormalized = "layout_sparse_normalized"`,
  `IssueCodeLayoutExtraItemsPreserved = "layout_extra_items_preserved"`.
- `ApplyTemplateV2Result` gains six counters:
  `LayoutInventoryEntriesApplied`, `LayoutStorageEntriesApplied`,
  `LayoutInventoryEntriesMissing`, `LayoutStorageEntriesMissing`,
  `LayoutInventoryExtrasPreserved`, `LayoutStorageExtrasPreserved`.
  Wails models regenerated (`frontend/wailsjs/go/models.ts`).

Apply gate
(`app_templates_v2_apply.go` scope detection):
- Layout sections require an active Inventory Edit Session — the
  writer mutates `sess.Workspace`. Without a session the apply
  returns `inventory_session_required` and zero side effects, same
  as items and inventory.workspace.

Tests: new `app_templates_v2_apply_layout_test.go` (24 cases): mode
allowlist (reorderOnly, ignore, append/replace/other), missing /
ambiguous / sparse / extras paths, items + layout interop ordering,
no-session reject, both containers, atomicity (writer failure leaves
slot and workspace untouched), warning counts, deterministic
first-match-wins, weapon match key reuse.

Validation:
- `go vet ./backend/templates/... ./` — clean.
- `go test . -run 'Test.*ApplyBuildTemplateV2.*Layout|Test.*Layout|Test.*ApplyBuildTemplateV2.*Items' -count=1` — green.
- `go test -race -run 'Test.*ApplyBuildTemplateV2' .` — green.
- `go build ./backend/... ./` — green.

Not implemented (Phase 8E.2 / Phase 8F+ scope):
- Frontend exposure of the apply path (Phase 8E.2).
- `append` / `replace` layout modes — schema-allowed, writer-rejected.
- Items modes beyond `addMissing`.

### chore(templates): polish v2 items apply UX (Phase 8D.3)

Follow-up to Phase 8D.2 with three focused improvements; no schema,
no writer, no new section.

Explicit `addMissing` on the wire
(`frontend/src/components/templates/TemplatesShellModal.tsx`):
- New `injectExplicitAddMissing(canonicalJSON)` helper rewrites
  `applyOptions.items.mode` to `"addMissing"` whenever
  `selection.items` is present (pass-through otherwise). Backend
  Phase 8D.1 already defaults to `addMissing` when the option is
  nil, but the UI now makes the intent testable from the JSON
  payload itself. Applied on both the imported preview JSON apply
  path and the overrides apply path.
- Library items entries now route through
  `PreviewBuildTemplateFromLibrary` → `injectExplicitAddMissing` →
  `ApplyBuildTemplateV2ToCharacterJSON` instead of the fast
  `ApplyBuildTemplateV2FromLibraryToCharacter` path, so explicit
  `addMissing` rides the wire for the library surface too. Non-items
  entries (profile / stats / equipment / spells) remain on the fast
  FromLibrary path — no extra round-trip noise.

Warning grouping in the new result modal
(`TemplatesShellModal.tsx::ApplyItemsResultModal`):
- Warnings are now grouped by code rather than rendered as a flat
  list. Five named groups so far:
  `items-apply-result-already-present`,
  `items-apply-result-unsupported-category`,
  `items-apply-result-layout-ignored`,
  `items-apply-result-weapon-warnings`
  (covers `weapon_level_clamped` and `weapon_unupgradeable`),
  `items-apply-result-template-override-ignored`. Unknown codes
  fall into `items-apply-result-other-warnings` with the code
  prefix shown.
- Each group caps the visible items at `WARNING_PREVIEW_LIMIT = 5`
  with a `+ N more (total: X)` line, so a noisy apply doesn't blow
  up the modal.
- All groups expose `data-warning-severity="info"` — every code
  surfaced here is informational, not a blocking failure.

UI copy + tests:
- `ImportTemplatePreviewModal.tsx` now renders an
  `import-preview-items-weapon-hint` line ("Direct Apply uses
  template / default upgrade levels. Use 'Apply with overrides…'
  to override standard (+0–25) or somber (+0–10) weapon levels for
  newly added items.") whenever the items section is selected.
- `TemplateLibraryModal.tsx` adds the same hint to the v2 confirm
  row (`library-apply-v2-weapon-hint`).
- Test additions cover wire-format injection on the imported
  preview, overrides, and library routes; warning-group expansion;
  layout-ignored severity stays `info`; weapon hint visibility in
  both the preview modal and the library confirm row.

Validation: `npx tsc --noEmit` clean,
`npx vitest run src/components/templates src/wails-bindings.contract`
— green.

### feat(templates): add v2 items apply controls (Phase 8D.2)

Frontend half of items apply: lift the Phase 8D.1 backend behavior
into the UI without changing any apply semantics.

Apply gating
(`frontend/src/components/templates/ImportTemplatePreviewModal.tsx`,
`TemplateLibraryModal.tsx`):
- `V2_APPLY_SUPPORTED_SECTIONS` adds `'items'`. Items-bearing v2
  templates can now reach the Apply button on the imported preview
  surface, the library list, and the URL preview (the URL preview
  reuses the import preview modal, so it inherits the change for
  free).
- The previous "items / inventoryLayout / storageLayout are
  export-only" tooltip is split: items templates show "Items: apply
  supported (add missing only)"; layout-only templates keep the
  "export-only" reason (until Phase 8E.2 lifts that too).
- Items + layout combo shows the new copy "Inventory layout /
  storage layout are export-only and will be ignored on apply."
  — backend Phase 8D.1 emits `items_layout_ignored` to match.

Session gate + apply path
(`frontend/src/components/templates/TemplatesShellModal.tsx`):
- The Inventory Edit Session requirement now covers items applies
  (items mutate `sess.Workspace`, same as inventory.workspace).
- New `ApplyItemsResultModal` (`items-apply-result-modal` testid)
  opens after every items apply with the counters that the new
  `ApplyTemplateV2Result.{inventoryItemsApplied,
  storageItemsApplied}` fields surface. Profile/stats-only applies
  keep the existing toast UX with no extra modal.

Weapon level override exposure
(`frontend/src/components/templates/ApplyOverridesPanel.tsx`,
`WeaponLevelOverridePanel.tsx`):
- The Phase 7a.2 `WeaponLevelOverridePanel` is now rendered for
  items-bearing v2 templates too — the helper already runs after
  each weapon write, so newly added items pick up the override
  inside the same apply. Direct Apply (without overrides) keeps
  using template / default upgrade levels.

Tests: extensive additions in the four templates test files
covering the new gating, the items confirm row, the result modal,
and the weapon override surface. Validation: `npx tsc --noEmit`
clean, `npx vitest run src/components/templates
src/wails-bindings.contract` — green.

### feat(templates): apply v2 items add-missing (Phase 8D.1)

First v2 apply path for `sections.items`. Mode allowlist is
**`addMissing` only**: existing live items are preserved and
template entries whose `(itemID, category, upgrade, infusion, AoW,
container)` tuple already resolves to a live workspace item are
skipped with a warning. The UI piece lands in Phase 8D.2.

Writer wiring (`app_templates_v2_apply.go`):
- New `applyTemplateItemsAddMissingToWorkspace(sess.Workspace, sec,
  weaponOverride)` walks `sections.items.entries`, builds the match
  tuple for each entry, and dispatches missing items through the
  existing `editor.AddItem` / workspace add helper so storage and
  inventory both go through the same allocator. Duplicate same-itemID
  entries are supported via the template-local `entryID`.
- Categories outside `editor.SupportedCategories` are skipped with
  `unsupported_category` (warning, not fatal). Items with
  `quantity == 0` are rejected at the schema validator (Phase 8B);
  the apply path never sees them.
- Weapon level override (Phase 7a.2) is honoured for newly added
  weapon-like items: after each successful add, the helper runs
  `applyWeaponLevelOverride` so the override applies to template
  additions just like it does to existing weapon patches. Templates
  carry no override field on the wire — the runtime option travels
  through `ApplyTemplateV2Options.WeaponLevelOverride`.
- Layout sections (`inventoryLayout`, `storageLayout`) appearing
  alongside `items` are dropped with `items_layout_ignored` (info,
  not fatal). The dedicated layout writer arrives in Phase 8E.1.

Apply gate (`app_templates_v2_apply.go` scope detection):
- Items section requires an active Inventory Edit Session — same
  reason as `inventory.workspace`. No session → `inventory_session_required`.
- `equipment + items` is allowed; `equipment + inventory.workspace`
  remains hard-rejected (Phase 7b.1).

Result + summary plumbing
(`backend/templates/import.go`, `frontend/wailsjs/go/models.ts`):
- New issue codes
  `IssueCodeItemsAlreadyPresent = "items_already_present"`,
  `IssueCodeItemsUnsupportedCategory = "unsupported_category"`,
  `IssueCodeItemsLayoutIgnored = "items_layout_ignored"`,
  `IssueCodeItemsTemplateOverrideIgnored = "items_template_override_ignored"`.
- `ApplyTemplateV2Result` gains `InventoryItemsApplied` and
  `StorageItemsApplied`.

Tests: new `app_templates_v2_apply_items_v2_test.go` (~25 cases):
add to empty inventory + storage; duplicate same itemID via
entryID; categories outside `SupportedCategories` skipped + warned;
weapon override applied on add; explicit `addMissing` mode on the
wire; no-session reject; items + layout coexistence with the
ignored-layout warning; rollback on writer failure; preserved
existing items; match tuple includes upgrade / infusion / AoW.

Validation:
- `go vet ./backend/templates/... ./` — clean.
- `go test . -run 'Test.*ApplyBuildTemplateV2.*Items' -count=1` —
  green.
- `go test -race -run 'Test.*ApplyBuildTemplateV2' .` — green.
- `go build ./backend/... ./` — green.

Not implemented in this phase:
- Frontend UI surface for items apply (Phase 8D.2).
- Items modes beyond `addMissing` (`updateExisting`, `merge`,
  `replace`).
- Layout writers (Phase 8E.1).

### feat(templates): wire v2 items layout export UI (Phase 8C.1)

App layer + UI wiring for the Phase 8C items / inventoryLayout /
storageLayout sections. **Still export-only**: apply for these
sections remains unsupported and the existing modal-level allowlist
keeps the Apply button disabled when a template carries any of them.

App layer (`app_templates_v2.go`):
- `ExportBuildTemplateV2JSONFromCharacter`,
  `ExportBuildTemplateV2YAMLFromCharacter`,
  `PreviewBuildTemplateV2FromCharacter`, and
  `SaveBuildTemplateV2FromCharacterToLibrary` now derive an
  `ItemsLayoutSource` from the live character whenever the
  selection includes `items`, `inventoryLayout`, or `storageLayout`.
- New helper `(*App).buildItemsSourceForCharacter(charIndex)` reuses
  `editor.BuildSnapshot` under the same `saveMu.RLock + slotMu` pattern
  as `GetCharacter` (mirrors `app_save_audit.go`). No save mutation,
  no edit session required, no raw handles surface to JS.
- Selection that mentions layout without items is rejected up-front by
  `BuildV2Template`'s guard — surfaced verbatim to the JS caller.

Preview / library summary (`backend/templates/import.go`):
- `ImportPreviewSummary` gains `itemsEntries`, `inventoryLayoutCount`,
  `storageLayoutCount`. Counts are read directly from
  `tpl.Sections.{Items,InventoryLayout,StorageLayout}`.
- `selectedSectionsForTemplate` now emits `items`, `inventoryLayout`,
  and `storageLayout` alongside the existing entries so the modal
  gating (`V2_APPLY_SUPPORTED_SECTIONS`) automatically treats them
  as unsupported and disables Apply.
- `LibraryTemplateEntry.SelectedSections` therefore also captures the
  new entries — no library schema rewrite needed.

UI (`frontend/src/components/templates/CreateTemplateV2Modal.tsx`):
- New "Containers" section with three checkboxes: Items, Inventory
  layout, Storage layout. Labelled as **Export-only** via a high-
  contrast pill and a single descriptive line.
- **Decision: layout disabled until Items is checked.** The label
  reads "Inventory layout (requires items)" / "Storage layout
  (requires items)" with a tooltip explaining why. Unchecking Items
  clears + re-disables both layout checkboxes. Alternative considered
  — auto-checking Items when the user picks layout — was rejected
  because the implicit state change can surprise the user. The
  explicit gate matches the backend `BuildV2Template` guard exactly.
- `buildSelectionJSON` accepts an optional `containers` argument and
  emits `items: true` / `inventoryLayout: true` / `storageLayout:
  true` flags at the top level (matches the backend parser).

UI (`frontend/src/components/templates/ImportTemplatePreviewModal.tsx`):
- v2 metadata block now shows Items / Inventory layout / Storage
  layout counts when any are non-zero, plus a short export-only
  note.
- Disabled-Apply tooltip extended: it now explicitly names "Items /
  inventoryLayout / storageLayout are export-only".

Wails bindings regenerated for the three new
`ImportPreviewSummary` fields. No new App methods, no struct rename
— bindings noise stayed minimal.

Tests:
- Backend (`app_templates_v2_items_test.go`): export with items
  only, export with items + layout, layout-without-items rejection,
  preview counts in summary, library save preserves
  items/layout/SelectedSections, apply gating still blocks
  items-only payloads.
- Frontend (`CreateTemplateV2Modal.test.tsx`,
  `ImportTemplatePreviewModal.test.tsx`): containers checkboxes
  render, disabled-without-items behaviour, selection JSON wire
  format, preview counts surface, Apply stays disabled when items
  is in `selectedSections`.

What stays explicitly out of scope (Phase 8D+ candidates):
- Apply for items / inventoryLayout / storageLayout.
- Inventory / storage writers driven from these sections.
- Manual layout validation, weapon-level override apply for
  imported items.
- Public JSON exchange (removed in Phase 8A and still gone).

### feat(templates): export v2 items and layout (Phase 8C)

Backend builder for the v2 items / inventoryLayout / storageLayout
sections. Scope is **export-only**: a producer can now emit the
new sections through `templates.BuildV2Template`, and a consumer can
round-trip them through YAML / library save. **No apply, no importer
preview wiring, no writer, no UI** — that work lands in Phase 8C.1
(App layer + UI) and Phase 8D+ (apply / writer).

`templates.ExportV2Options` gains an `ItemsSource *ItemsLayoutSource`
field carrying `editor.EditableItem` slices for inventory and storage.
The builder:
- Stable-sorts each container by `EditableItem.Position`.
- Translates each row to `TemplateItemEntryV2` with a deterministic
  per-template `entryID = "<container>_<4-digit post-sort index>"`
  (`inv_0000`, `sto_0042`).
- Per-row skips with notice (no fatal): `baseItemID==0`,
  `quantity==0`, `category` outside the Phase 8B allowlist. Surviving
  entries land in `sections.items.entries`.
- Inventory layout entries reference only `inv_*` IDs; storage layout
  entries reference only `sto_*` IDs.
- Layout positions are **compact 0..N-1 after skips** (matches the v1
  `convertItems` rule and avoids leaking workspace acquisition-index
  gaps).
- Weapon upgrade mapping: `MaxUpgrade=25` → `standard`,
  `MaxUpgrade=10` → `somber`, `MaxUpgrade=0` → `none`; non-weapon
  rows emit no upgrade fields.
- AoW emission mirrors the v1 exporter: only
  `CurrentAoWStatus="custom"` with `CurrentAoWItemID!=0` reaches the
  schema. Missing / shared / none / empty statuses drop the AoW
  silently (workspace UI already surfaces these states).

`BuildV2Template` selection guards:
- `selection.items` without an `ItemsSource` → fail-closed.
- `selection.inventoryLayout` / `selection.storageLayout` without
  `selection.items` → fail-closed (layout refs require items).
- `selection.{inventory,storage}Layout` without an `ItemsSource` →
  fail-closed.

Phase 8B validator stays in charge: the exporter feeds its own output
through `ValidateBuildTemplate` and a Phase 8C test asserts that round.

YAML / library / preview behaviour:
- `MarshalBuildTemplateYAML` / `ParseBuildTemplateYAML` round-trip
  the new sections (confirmed by `TestBuildV2_YAMLRoundTrip_*`).
- `TemplateLibrary.SaveTemplate` / `LoadTemplate` preserve items +
  layout (confirmed by `TestBuildV2_LibrarySaveLoad_*`).
- The v1 apply gate (`sections.inventory.workspace`) is NOT touched
  — a Phase 8C-emitted template carries `items` / `inventoryLayout`
  / `storageLayout` but leaves the v1 inventory-workspace section
  `nil`. Existing apply paths therefore continue to see the
  template as having no apply-able inventory content
  (confirmed by `TestBuildV2_TemplateWithItems_RemainsUnsupportedForV1ApplyGate`).

Decisions worth surfacing for Phase 8C.1 (App layer wiring) and 8D+
(apply / writer):
- **EntryID strategy**: `<container>_<4-digit zero-padded post-sort
  index>`. Stable for a given source snapshot, lexicographically
  identical to numerical order, prefix disambiguates the inv / sto
  namespaces.
- **Layout position normalisation**: compact 0..N-1 after skips. The
  Phase 8B validator allows non-dense positions, but the exporter
  picks the dense canonical form so YAML diffs stay readable and a
  future writer doesn't have to guess what the producer meant.
- **Per-row skipping vs whole-template failure**: v2 exporter skips
  individual rows. Rationale in `ItemsExportReport` doc — v2 items
  span every inventory/storage category, so one stray row should
  not kill the whole export.
- **`location: both` is never emitted**: each save snapshot puts a
  stack in exactly one container; the exporter mirrors that and emits
  two separate entries when the same `baseItemID` lives in both.

Tests: 17 new structural tests in
`backend/templates/export_v2_items_test.go` cover baseline negative
paths, items export across categories, upgrade-kind mapping, AoW
emission, per-row skips, layout reference correctness, position
compactness after skips, EntryID prefix invariants, YAML round-trip,
v1 apply gate non-regression, and library save/load round-trip.

### feat(templates): add v2 items layout schema (Phase 8B)

Schema-only foundation for the v2 items / inventory layout / storage
layout surface. Adds new sections, selection keys, and apply-options
DTOs to `backend/templates` together with their structural validators.
**No exporter, no importer, no apply path, no UI, no Wails bindings** —
that work lands in Phase 8C+.

New top-level fields on `BuildTemplate`:
- `applyOptions` (optional) — `items`, `inventoryLayout`, `storageLayout`,
  `weaponLevelOverride` sub-DTOs. v1 ignores it; v2 validator enforces
  mode allowlists and override ranges.

New `sections.*` keys (v2):
- `items.entries[]` — flat list of `TemplateItemEntryV2`. Each entry has
  a template-local `entryID` (stable string slug), `itemID`,
  `category` (fail-closed allowlist of 18 categories from
  `backend/db/data`), `quantity` (≥1, fail-closed on 0),
  `location` (`inventory`/`storage`/`both`), `upgradeKind`
  (`standard|somber|none`, empty string ≡ `none`), `upgradeLevel`
  (validated per kind: standard 0–25, somber 0–10, omitted/nil for
  none), `infusionName`, `ashOfWarItemID`.
- `inventoryLayout.entries[]` / `storageLayout.entries[]` — ordered
  `{entryRef, position}` pairs. `entryRef` must match an existing
  `items.entries[*].entryID`. EntryRef + position must be unique within
  the layout.

New `selection.*` keys (boolean-only — no per-field maps): `items`,
`inventoryLayout`, `storageLayout`. `HasAnySelected()` updated.

Apply-mode allowlists (Phase 8C+ will honor these):
- Items: `addMissing`, `updateExisting`, `merge`, `replace`
  (destructive, opt-in).
- Layout: `ignore`, `append`, `reorderOnly`, `replace`.

`WeaponLevelOverride`: `useTemplateLevels=true` ⊥ `standardOverride`/
`somberOverride`; bare struct (all nil + `useTemplateLevels=false`)
means "leave live levels untouched."

Decisions worth surfacing for Phase 8C:
- **Item identity**: the template-local `entryID` is the only handle
  layouts can use. Multiple entries may share `itemID` so the
  "two Longswords, different Ashes" case round-trips faithfully.
- **Layout reachability**: `inventoryLayout` / `storageLayout` always
  validate against `sections.items`; a layout without an items section
  is rejected by the entryRef check.
- **YAML strict mode**: KnownFields(true) keeps refusing unknown
  top-level keys; `applyOptions` is now part of the allowlist via the
  `BuildTemplate` field.

Tests: 30 new structural tests in
`backend/templates/schema_items_layout_test.go` cover every allowlist,
every fail-closed branch, the YAML round-trip, and the duplicate-item
identity case. All existing tests still pass.

### chore(templates): remove public json template exchange (Phase 8A)

YAML is now the **only** publicly accessible format for sharing build
templates. JSON survives exclusively as the on-disk library storage and
as an internal hand-off contract (canonical JSON re-serialised by the
YAML preview endpoints, consumed by `SaveImportedBuildTemplateJSONToLibrary`
and `ApplyBuildTemplateV2ToCharacterJSON`).

Removed Wails App methods (and their tests / bindings): `ExportBuildTemplateJSON`,
`ExportBuildTemplateToFile`, `PreviewBuildTemplateImportJSON`, `PreviewBuildTemplateImportFromFile`,
`ApplyBuildTemplateToWorkspaceFromFile`, `ExportLibraryBuildTemplateToFile`.
`ApplyBuildTemplateToWorkspaceJSON` was de-exported to `applyBuildTemplateToWorkspaceFromJSON`
(still reachable via `ApplyBuildTemplateFromLibrary` for already-stored
v1 library entries — the only public path that still consumes v1 JSON).

Removed `backend/templates.(*TemplateLibrary).ExportTemplateToFile`
(JSON-file helper); the YAML twin (`ExportTemplateToYAMLFile`) remains.

Removed frontend surface: `ExportTemplateModal.tsx` + its tests, the
entire `SortOrderTab` "Export Template ▾" dropdown (export quick-paths,
import preview, weapon-level override panel, Template Library entry),
the per-entry "Export" (JSON) button + `onExportedToFile` prop on
`TemplateLibraryModal`. The global Templates shell (sidebar entry) is
now the only place to interact with templates.

Spec updated: `spec/56-templates-v2.md` + `spec/lang-pl/56-templates-v2.md`
gain a new "Phase 8A — drop public JSON template exchange" section
documenting the precise removal list and what stays as internal contract.

### feat(templates): enable v2 spells UI apply

Shipped Phase 7d (sub-phases 7d.0 → 7d.4) of the Templates v2 design
(`spec/56-templates-v2.md`): end-to-end spells import / preview / apply
through a new `sections.spells` schema section, a new `(s *core.SaveSlot).
WriteSpells` batch writer with targeted hash[10] recompute, a new
`db.ItemIDToMagicParamID` 28-bit-mask helper, and full frontend gating /
preview row / library badge. Five disciplined sub-phases landed on
`feature/templates-v2-spell-writer-foundation`; the main import / preview /
apply flow was user-confirmed `manual OK` on 2026-06-02 (Test A — full
14-slot loadout — and Test B — partial leave-unchanged — both passed).
The follow-up Phase 7d.4b (create-from-character export + `CreateTemplateV2Modal`
Spells checkbox) remains planned — see spec/56 §17 for the deferred scope.

- **Phase 7d.0 — core spell writer foundation (commit `6cb2e60`)** —
  new `backend/core/spell_writer.go` ships single-slot writer
  `PatchEquippedSpell(slot, slotIndex, spellID) error` for the 14-slot
  `EquippedSpells` region. Strict pre-validation (nil slot, slot-index
  range `[0, EquippedSpellSlotCount)`, uninitialised
  `EquippedSpellsOffset`, out-of-bounds), idempotent no-op when target
  bytes already match, no hash mutation (the Phase 7d.3 batch writer
  owns hash[10]). Constants `EquippedSpellSlotCount = 14`,
  `EquippedSpellSlotSize = 8`, `EquippedSpellEmptySentinel =
  0xFFFFFFFF`, `EquippedSpellOccupiedFollower = 0xFFFFFFFF`. Empty
  slot semantics: `spellID == 0xFFFFFFFF → (spell_id=0xFFFFFFFF,
  follower=0x00000000)`; occupied: `spellID != 0xFFFFFFFF →
  (spell_id=spellID, follower=0xFFFFFFFF)`. 9 PatchEquippedSpell unit
  tests.
- **Phase 7d.1 — schema, DTO, validation (commit `7ac60d0`)** —
  `backend/templates/schema.go` adds `TemplateSections.Spells
  *SpellsSection`, `TemplateSelection.Spells *SectionSelection`,
  `SpellsSection` with 14 named pointer fields `Spell1..Spell14`,
  `SpellSlotRef { BaseItemID uint32; Name string }`, constants
  `SpellSlotCount = 14` / `SpellItemIDPrefix = 0x40000000` /
  `SpellItemIDPrefixMask = 0xF0000000`, canonical iteration order
  `SpellSlotOrder []string` (`"spell1"..."spell14"`),
  `spellsSelectionFields` allowlist, validators
  `validateSpellsSelection` / `validateSpellsSection` /
  `validateSpellSlotRef` (reject any non-`0x4XXXXXXX` prefix at
  ingest), and private getter `spellSlotRef`. Both sorceries AND
  incantations share the `0x40000000` prefix in the SaveForge DB
  (`backend/db/data/sorceries.go`, `incantations.go`); the earlier
  briefing that suggested `0x60XXXXXX` for incantations was wrong and
  is intentionally **not** documented anywhere. `BaseItemID == 0` is
  the explicit-clear sentinel; the save-level `0xFFFFFFFF` never
  appears in the public schema. Nil pointer = leave the live slot
  unchanged. 17 schema unit tests.
- **Phase 7d.2 — export builder + import preview/summary (commit
  `fad1315`)** — `backend/templates/export_v2.go` adds
  `ExportV2Options.EquippedSpellsRaw []uint32` (14 raw IDs from
  `slot.Data` at the EquippedSpells region; strict-reject when length
  ≠ `SpellSlotCount` AND `selection.spells` is selected) and
  `buildSpellsSection(rawIDs, sel)` mapping raw `0xFFFFFFFF` →
  `&SpellSlotRef{BaseItemID: 0}` (explicit clear) and other raw →
  `&SpellSlotRef{BaseItemID: SpellItemIDPrefix | rawID}` (`Name` left
  empty — no DB lookup inside `backend/templates`).
  `backend/templates/import.go` adds `ImportPreviewSummary.SpellSlotsPresent
  []string` populated via `spellSlotsPresent(tpl.Sections.Spells)`
  mirroring `equipmentSlotsPresent`; `selectedSectionsForTemplate`
  adds `"spells"` when `tpl.Selection.Spells.HasAny()`. 9 builder +
  6 preview unit tests. Apply remains blocked until Phase 7d.3.
- **Phase 7d.3 — backend apply + bindings + `db.ItemIDToMagicParamID`
  (commit `5c3e538`)** — `backend/core/spell_writer.go` adds the
  batch writer `(s *SaveSlot) WriteSpells(writes []SpellWrite) error`
  with pre-validation of EVERY write before any byte mutation
  (duplicate slot indices, out-of-range slot indices, uninitialised
  `EquippedSpellsOffset`, out-of-bounds region, missing hash block)
  and a targeted recompute of **only** hash[10] via
  `binary.LittleEndian.PutUint32(s.Data[HashOffset+10*4:],
  equipmentHash(readSpellIDs(s.Data, s.EquippedSpellsOffset)))`. The
  global `RecalculateSlotHash` stays unwired in production;
  `WriteEquipment` continues to own hash[7] / hash[8]. New DB helper
  `db.ItemIDToMagicParamID(itemID uint32) uint32 { return itemID &
  0x0FFFFFFF }` — **28-bit mask**, mirroring the existing
  `ItemIDToHandlePrefix` convention (4-bit prefix + 28-bit payload);
  a 16-bit `0x0000FFFF` mask would truncate high-payload spell IDs
  and is regression-guarded by 4 subtests covering payloads
  `0xFFFF`, `0x10000`, `0x12ABCD`, `0x0FFFFFFF`.
  `backend/templates/schema.go` exposes `SpellSlotRefBySlotKey(sec,
  slotKey) *SpellSlotRef`. `app_templates_v2_spells.go` (new) adds
  `resolveSpellWrites(slot, sel, sec) ([]core.SpellWrite,
  []ImportPreviewIssue, error)` with semantics: `!sel.Selected(key)
  || ref == nil` → no write (live slot unchanged); `BaseItemID == 0`
  → `core.EquippedSpellEmptySentinel` (explicit clear); `BaseItemID
  != 0` → defensive prefix re-check + DB membership check via
  `db.GetItemData(id).Category` (must be `"sorceries"` or
  `"incantations"`) + `db.ItemIDToMagicParamID` → raw spell ID;
  unknown valid-prefix → `IssueCodeUnknownItem` warning + skip
  (mirrors the equipment resolver's not-in-inventory pattern). Go
  error reserved for infrastructure (nil slot, nil section).
  `app_templates_v2_apply.go` adds `SpellSlotsApplied int` on
  `ApplyTemplateV2Result`, extends the empty-selection guard to
  include `hasSpells`, places the resolver block after the equipment
  resolver and before the inventory resolver, and places the
  `WriteSpells` call **after** `vm.MapViewModelToSlot` AND **after**
  `slot.WriteEquipment` — the post-VM placement is critical because
  `vm.MapViewModelToSlot` rewrites the EquippedSpells region from the
  cached VM and would silently clobber any earlier spells write. The
  `applied` list gains `"spells"` whenever `len(spellWrites) > 0`.
  Bindings regen (`frontend/wailsjs/go/models.ts` gains
  `spellSlotsApplied: number` on `ApplyTemplateV2Result` and
  `spellSlotsPresent?: string[]` on `ImportPreviewSummary` — 4 lines
  content diff; the three `frontend/wailsjs/runtime/*` files show
  mode-bit flip 644 → 755 only with zero content diff). 9 WriteSpells
  + 4 DB-helper + 8 apply unit tests.
- **Phase 7d.4 — frontend UI for spells apply (commit `9e8aabe`)** —
  `frontend/src/components/templates/ImportTemplatePreviewModal.tsx`
  extends `V2_APPLY_SUPPORTED_SECTIONS` from
  `['profile','stats','inventory.workspace','equipment']` to
  `['profile','stats','inventory.workspace','equipment','spells']`,
  adds `spellSlotsPresent` extraction, includes it in `showV2Meta`,
  renders a new `<div data-testid="import-preview-spell-slots">Spell
  slots: <span>{joined}</span></div>` row immediately after the
  equipment-slots row, and widens the unsupported-section disabled
  tooltip. `frontend/src/components/templates/TemplateLibraryModal.tsx`
  ORs `selectedSections.includes('spells')` into
  `v2HasApplyableSections`; the library sections row continues to
  render `selectedSections.join(', ')` so `"spells"` appears verbatim.
  `TemplatesShellModal.tsx` unchanged — spells do not need a session.
  `CreateTemplateV2Modal.tsx` **intentionally untouched** because the
  backend create-from-character path does not yet pass
  `EquippedSpellsRaw` (gated by the planned Phase 7d.4b — equipment
  is in the same deferred state for the same reason). Tests:
  +5 new cases in `ImportTemplatePreviewModal.test.tsx` (`Phase 7d.4
  spells section` describe) and +3 new cases in
  `TemplateLibraryModal.test.tsx` (`Phase 7d.4 spells entries`
  describe); two pre-existing tests that used `'spells'` as an
  example of "still unsupported" were rewired to `'inventory.unknown'`
  with a comment. `cd frontend && npx vitest run
  src/components/templates` → 247 / 247 pass.
- **Spec amendment** — `spec/56-templates-v2.md` §17 replaces the
  earlier single planned "Phase 7d — spell loadout writer (new write
  path)" with the five shipped sub-phases (7d.0 → 7d.4) and the new
  planned Phase 7d.4b. §17a.2j adds the v2 spells apply flow
  walkthrough (13 numbered steps). §17a.3 converts the
  "EquippedSpells loadout — gated by Phase 7d" bullet into five `✅
  Shipped 2026-06-02` entries (one per sub-phase) plus a Phase 7d.4b
  planned bullet. The header status badge widens to mention
  "Phase 7d.0 → 7d.4 shipped 2026-06-02" and to flag Phase 7d.4b /
  Phase 8 / Phase 10 as remaining design-only. `spec/lang-pl/56-
  templates-v2.md` mirrors the EN edits faithfully. `spec/README.md`
  and `spec/lang-pl/README.md` extend the spec/56 table row.
- **Manual validation (2026-06-02, user-confirmed `manual OK`)** —
  Test A (full 14-slot loadout: 6 occupied + 8 explicit clear,
  `selection.spells: true`) and Test B (partial leave-unchanged:
  per-field selection of spell1/spell2/spell3 only, slots 4–14
  retain pre-apply state) both passed end-to-end through Import
  preview → Apply → Save. Verified spell IDs against
  `backend/db/data/{sorceries,incantations}.go`: Catch Flame
  `0x40001770` (incantation), Glintstone Pebble `0x40000FA0`
  (sorcery), Rock Sling `0x40001266` (sorcery), Heal `0x40001915`
  (incantation), Rancorcall `0x40001388` (sorcery). The earlier
  briefing that named `0x40001388` as Glintstone Pebble was wrong —
  `0x40001388` is Rancorcall; Glintstone Pebble is `0x40000FA0`.

### feat(templates): apply v2 equipment talisman slots

Shipped Phase 7c of the Templates v2 design (`spec/56-templates-v2.md`):
v2 `sections.equipment` is now extended with the five talisman slots
`talisman1..5` end-to-end — schema, export, preview, apply through the
existing Phase 7b.0 `SaveSlot.WriteEquipment` foundation (extended to
ChrAsmEquipment indices 17–21), and the existing frontend equipment
surface. Phase 7c intentionally extends `sections.equipment` rather than
introducing a separate `sections.equippedTalismans`: talismans live in
the same ChrAsmEquipment struct as weapons/ammo/armor, share hash 8
recompute, and reuse the Phase 7b.1 resolver, combo guard, preview
row, frontend gate, and rollback semantics with no new section. The
earlier spec/56 design note that reserved a separate `sections.
equippedTalismans` is superseded — see the spec amendment below.

- **Core writer** — `backend/core/equipment_writer.go` adds enum values
  `EquipSlotTalisman1..5`, a fourth class `slotClassTalisman`, and the
  five slot-table entries mapping to ChrAsmEquipment indices 17–21
  (talismans share hash 8 with armor; `armorSlotIndices` in
  `hash.go` already covers 12–15 + 17–21, so the recompute path needed
  only the touched-slot range to widen to `r.index >= 17 && r.index <=
  21`). The `encodeEquipmentValue` switch gains a `slotClassTalisman`
  branch that accepts only the `ItemTypeAccessory` (0xA0) handle prefix
  and rejects 0x80 / 0x90 / 0xB0 / 0xC0 with class-specific error
  messages; the existing weapon / armor / ammo classes continue to
  reject 0xA0 handles. The encoded slot value is `GaMap[handle]`
  directly (no OR-mask) because talisman handles map to itemIDs that
  already carry the 0x20 prefix in the GaMap — mirroring how ammo
  encodes goods itemIDs without a `| 0x80000000` mask. The Phase 7b.0
  atomic-batch contract (validate-then-mutate, hash 7 untouched for
  talisman-only writes, hash 8 untouched for weapon-only writes,
  duplicate-slot detection, sentinel rejection) is preserved
  unchanged for the new slot kinds.
- **Schema** — `backend/templates/schema.go` extends `EquipmentSection`
  with `Talisman1..5 *EquipmentItemRef`; `equipmentSelectionFields`,
  `EquipmentSlotOrder`, `equipmentSlotRef`, and `SetEquipmentSlotRef`
  all grow by the five canonical talisman keys (slots appended after
  `armorLegs` to preserve the existing 14-slot prefix order). No new
  ref type is introduced — talismans reuse `EquipmentItemRef` but the
  apply layer ignores `Upgrade`, `InfusionName`, and `AoWItemID` for
  talisman slots, and producers should normally omit those fields
  (talisman items have no upgrade level, no infusion, and no Ash of
  War). The doc comment on `EquipmentSection` records the deviation
  from the spec/56 §17 original design and the Talisman5 / pouch
  gating contract.
- **Issue codes** — one new stable string in
  `backend/templates/import.go`:
  `IssueCodeTalismanSlotPouchInsufficient`
  (`talisman_slot_pouch_insufficient`, warning — the apply resolver
  detected a non-empty talisman ref targeting a slot beyond
  `1 + effective profile.talismanSlots`; the slot is skipped and other
  applied sections still commit). Slot 5 (Talisman5) always triggers
  this warning when populated with a non-empty ref because vanilla
  Elden Ring caps the Talisman Pouch at 4 active slots; an explicit
  clear (`baseItemID = 0`) for Talisman5 bypasses the gate and is
  always accepted. Mixed templates that also set
  `profile.talismanSlots` evaluate pouch state from the template's
  value (lifted to `1 + value`) before equipment apply runs so a +3
  pouch bump in the same template unblocks `talisman4` in the same
  apply.
- **Export / scanner** — `app_templates_v2_equipment.go` extends
  `equipmentSlotChrAsmIndex` and `equipmentSlotKindForKey` with the 5
  talisman keys, adds the `equipmentSlotIsTalisman(slotKey)` helper,
  and routes the talisman path through the existing
  `decodeEquipmentSlotToRef` candidate selection: ammo or talisman
  slots use the raw stored value (no OR-mask), other slots strip the
  `0x80000000` weapon/armor flag. `buildEquipmentSection` already
  iterates `EquipmentSlotOrder` and dispatches through
  `EquipmentSlotRef` / `SetEquipmentSlotRef`, so the existing builder
  picks up talismans without modification.
  `itemToEquipmentRef` continues to set `Upgrade` only when
  `IsWeapon || IsArmor`, so a talisman editable item naturally yields
  a ref with `BaseItemID + Name` only and no `Upgrade` / `InfusionName`
  / `AoWItemID` fields.
- **Apply** — `app_templates_v2_apply.go` introduces
  `computeActiveTalismanSlots(slot, tpl)` which returns the effective
  talisman pouch capacity (1..4): when the template selects
  `profile.talismanSlots` and ships a value in `sections.profile`, the
  template value wins (clamped to `MaxProfileTalismanSlots = 3`),
  otherwise the slot's current persisted `Player.TalismanSlots` is
  used (also clamped). The active capacity is `1 + base`. The Phase
  7b.1 call site to `resolveEquipmentWrites` extends to pass
  `activeTalismanSlots` through to the resolver. The resolver in
  `app_templates_v2_equipment.go` adds the `MaxActiveTalismanSlots = 4`
  constant + `talismanSlotOrdinal(slotKey)` helper and a pouch gate:
  for non-empty talisman refs with ordinal beyond the active capacity
  the resolver emits `talisman_slot_pouch_insufficient` + skips; clear
  refs always pass through (so clearing `talisman5` is always allowed
  even when the slot is unreachable). All other apply semantics —
  rollback atomicity (snapshot covers slot bytes + workspace),
  `equipment + inventory.workspace` hard reject, `equipment_item_not_in_inventory`
  warning + skip, `equipment_item_ambiguous` first-wins warning,
  `equipmentSlotsApplied` counter, fast-apply (no session required for
  equipment-only) — are unchanged.
- **Atomicity** — `WriteEquipment` and the surrounding apply continue
  to take the same `core.SnapshotSlot` snapshot at the top of the slot
  lock that Phase 7b.1 introduced; a mid-batch validation failure
  during a mixed armor + talisman write rolls back both slot bytes
  and hash bytes with zero side effects.
- **Frontend** — no source-code changes needed.
  `V2_APPLY_SUPPORTED_SECTIONS` already lists `equipment`, the existing
  `import-preview-equipment-slots` row enumerates whatever slot keys
  the summary lists, and the Library / direct-YAML / URL apply paths
  reuse the canonical JSON pipeline. Talisman keys appear in the
  preview row when present in the template. Two regression tests are
  added in `ImportTemplatePreviewModal.test.tsx`:
  `talisman1..4` render in the equipment row, and a talisman-only
  equipment template keeps the Apply button enabled.
- **Bindings** — `make build` triggered the standard
  `wails generate module` step; `frontend/wailsjs/go/models.ts` shows
  no diff because the talisman extensions are JSON-payload fields on
  `EquipmentSection`, which is exchanged with the frontend as opaque
  template JSON rather than a typed Wails model. `App.d.ts` /
  `App.js` are untouched (no method signature change). The pre-existing
  `equipmentSlotsApplied` counter naturally includes talisman writes.
- **Tests** — `backend/core/equipment_writer_test.go` adds 10
  talisman-focused cases (encoding, four cross-class rejects, hash 8
  recompute, hash 7 stable on talisman-only, idempotent write, mixed
  armor+talisman batch, atomic rollback). `backend/templates/
  schema_equipment_test.go` adds 8 talisman cases (JSON / YAML
  round-trip, slot-order tail, selection allowlist for 5 keys, explicit
  clear, slot ref helpers, preview equipment-slots row listing,
  `equipment + inventory.workspace` combo guard with a talisman ref,
  stable issue-code string); the existing
  `TestEquipmentSelection_RejectsUnknownSlotKey` switches from
  `"talisman1"` (now valid) to `"equippedSpell1"` and the
  `PerFieldAllowlistAcceptsAll` test updates the expected
  `EquipmentSlotOrder` length from 14 to 19.
  `backend/templates/export_v2_equipment_test.go` adds 3 talisman
  export cases (verbatim copy of populated slots + explicit clear,
  per-field selection drops unsupplied slots, JSON shape includes
  `"talisman1"`). `app_templates_v2_equipment_test.go` adds 4
  scanner cases (talisman1 match with `IsTalisman` editable item;
  five talismans populated; unknown talisman emits raw decoded ID;
  `equipmentSlotIsTalisman` truth table) and renames the negative
  `TalismansAndGreatRuneNotExported` test to
  `GreatRuneAndUnknownSlotsNotExported` because talismans now DO
  export. `app_templates_v2_apply_equipment_test.go` adds 13
  cases: 6 resolver unit tests (with-pouch happy path, beyond-pouch
  warns+skips, Talisman5 always warns when populated, Talisman5
  explicit-clear allowed, clear beyond pouch allowed, missing-item +
  in-pouch warns just `equipment_item_not_in_inventory`); 4
  `computeActiveTalismanSlots` tests (slot value when no profile,
  template overrides slot, template profile not selected ignored,
  clamps slot value); 3 resolver + writer integration tests that
  exercise `resolveEquipmentWritesFromItems` + `WriteEquipment`
  end-to-end without standing up `BuildSnapshot` (happy path 4
  talismans written + hash 8 recomputed, pouch insufficient skips
  slots 2/3/4, Talisman5 clear works on byte level). The seven
  existing resolver call sites in the same file gain the new
  `activeTalismanSlots = 4` (max, no gating) parameter; one test that
  used `"talisman1"` as an example of an unknown selection key now
  uses `"equippedSpell1"`. `frontend/.../ImportTemplatePreviewModal.test.tsx`
  adds 2 cases for talisman slots in the preview row and the Apply
  button.
- **Manual validation** — 2026-06-01, OK. Tests covered: equipment
  template export with active talismans + slot 5 clear, talisman keys
  in v2 preview row + Apply button enabled, equipment-only apply
  (talismans replaced + game-load showed correct HUD + Save & Quit
  retained state), pouch gating with `TalismanSlots = 0` (slot 1 OK,
  slots 2/3/4 `talisman_slot_pouch_insufficient` warn + skip), mixed
  `profile.talismanSlots = 3 + equipment.talisman4` lifted cap inside
  the same apply, `talisman5` with `baseItemID > 0` warn + skip and
  `talisman5 baseItemID = 0` clear OK, missing talisman item warn +
  skip with other slots committing, `equipment + inventory.workspace`
  hard reject unchanged, weapons / ammo / armor apply regression OK,
  Phase 7a inventory.workspace + override + Phase 9 URL import
  regression OK, v1 SortOrderTab weapon override + Profile/Stats apply
  regression OK.
- **Future work** — Phase 7d (EquippedSpells writer + v2 spells apply,
  separate `sections.spells` because spells live in a different binary
  section), optional Phase 7b.2 lifting the
  `equipment + inventory.workspace` hard reject once GaMap refresh
  semantics from the workspace flow are sorted out, EquippedGreatRune
  (out of scope — written by `SyncPlayerToData`, not `WriteEquipment`),
  quick items / pouch slots (no write API), appearance presets, sort
  order packaging, world progress, multi-character pack.

### feat(templates): apply v2 equipment section

Shipped Phase 7b.1 of the Templates v2 design (`spec/56-templates-v2.md`):
v2 `sections.equipment` end-to-end — schema, export, preview, apply through
the Phase 7b.0 `SaveSlot.WriteEquipment` foundation, and frontend surface in
the existing Templates shell. Equipment-only templates can be created from a
character, saved to the library, exported as YAML, imported (file / URL),
previewed, and applied to a target character — no Inventory Edit Session
required and no new App method. The shipped scope matches the Phase 7b.0
writer coverage exactly (no talismans, spells, EquippedGreatRune, quick
items, or unknown slots).

- **Schema** — `backend/templates/schema.go` adds `EquipmentSection`,
  `EquipmentItemRef`, `TemplateSelection.Equipment`,
  `TemplateSections.Equipment`, and an `EquipmentSlotOrder` canonical
  list. `EquipmentSection` carries 14 optional pointer fields:
  weapon LH/RH 1/2/3 (6), `arrows1/2` + `bolts1/2` (4), and
  `armorHead/Chest/Arms/Legs` (4). Talisman slots 17–21, EquippedGreatRune
  (slot 10), unknown slots 11/16, the 14 EquippedSpells, and quick /
  pouch slots are intentionally absent — they have no Phase 7b.0 writer
  entry point. `EquipmentItemRef` carries `BaseItemID` (required;
  `0` is the explicit-clear sentinel), optional informational `Name`,
  optional `Upgrade *int` disambiguator (nil → match any upgrade),
  optional `InfusionName` disambiguator, and optional `AoWItemID *uint32`
  disambiguator. `EquipmentSlotRef` / `SetEquipmentSlotRef` helpers
  expose canonical-key lookups so the resolver, exporter, and preview
  walk the 14 slots without duplicating the switch.
- **Validation** — `validateEquipmentSection` enforces
  `upgrade ∈ [0, MaxEquipmentItemUpgrade=25]` (per-item caps are deferred
  to the apply resolver where the resolved DB entry is known) and
  rejects `aowItemID=0` (use field omission to mean any-AoW).
  `validateEquipmentSelection` enforces a 14-slot allowlist matching the
  EquipmentSlotOrder canonical names. The previously-shipped Phase 7a
  validators (`validateProfileSelection`, `validateStatsSelection`,
  `validateInventoryWorkspaceSelection`) are unchanged.
- **Issue codes** — four new stable strings in
  `backend/templates/import.go`:
  `IssueCodeEquipmentInventoryComboUnsupported`
  (`equipment_inventory_combo_unsupported`, hard error — combination of
  `sections.equipment` + `sections.inventory.workspace` in the same
  template is rejected because the writer needs a fresh `slot.GaMap`
  and inventory.workspace items only land in the workspace until the
  user clicks Save changes),
  `IssueCodeEquipmentItemNotInInventory`
  (`equipment_item_not_in_inventory`, warning — the resolver could not
  find the item; storage is intentionally NOT searched; slot skipped),
  `IssueCodeEquipmentItemAmbiguous` (`equipment_item_ambiguous`,
  warning — multiple matches after disambiguators, first wins), and
  `IssueCodeEquipmentSlotInvalid` (`equipment_slot_invalid`, hard error —
  the writer rejected a resolved EquipmentWrite; dual-snapshot rollback
  restores both slot bytes and the workspace).
- **Preview** — `PreviewBuildTemplateImport` injects the combo guard so
  the modal sees a clean `report.OK=false` with the dedicated code; the
  apply path double-checks the same rule defensively for direct callers
  that bypass the preview. `ImportPreviewSummary.EquipmentSlotsPresent`
  lists the canonical-order slot keys whose pointer is non-nil. The
  existing `selectedSections` list now appends `"equipment"` when the
  selection nominates it (kept after `inventory.workspace` /
  `profile` / `stats` so the order remains stable).
- **Export** — `templates.ExportV2Options.Equipment *EquipmentSection`
  and `BuildV2Template` extend the existing Phase 5 pattern: the
  boolean shortcut copies the whole section verbatim; per-field
  selection normalises to only slot keys with a source value. Pointer
  fields (`Upgrade`, `AoWItemID`) are deep-cloned so the result does
  not alias the source. The app-layer scanner
  `buildEquipmentSectionFromSlot(slot, inventoryItems)` reads
  `ChrAsmEquipment` byte slots at the 14 supported indices, decodes the
  encoded form (weapon / armor: strip `0x80000000`; ammo: raw goods
  ItemID), matches the encoded value against `editor.EditableItem.ItemID`
  for an exact-handle match, falls back to `db.GetItemDataFuzzy` for
  pass-through items, and emits a minimal ref with the raw decoded ID
  as last resort so the user always sees what the slot held.
- **Apply** — `app_templates_v2_apply.go` extends scope detection with
  `hasEquipment := tpl.Selection.Equipment.HasAny()` and tightens the
  empty-selection diagnostic to mention equipment. The combo guard
  hard-rejects `equipment + inventory.workspace` before snapshot or
  session acquire — zero side effects. `resolveEquipmentWrites(slot,
  sel, sec)` runs `editor.BuildSnapshot` once for a consistent
  inventory view, then delegates to the pure-logic
  `resolveEquipmentWritesFromItems(items, sel, sec)`: for each
  selected slot it resolves `baseItemID == 0` to a clear write, walks
  `InventoryItems` (storage NOT searched) matching `BaseItemID` plus
  any supplied disambiguators (upgrade, infusion, AoW), and produces
  a warning + skip on miss or a warning + first-match-wins on
  ambiguity. `SaveSlot.WriteEquipment(equipmentWrites)` runs AFTER
  the profile/stats SyncPlayerToData so any failure rolls back the
  combined slot snapshot taken at the top of the slot lock. The
  resolved batch is reported as `applied = append(applied,
  "equipment")` and counted in
  `ApplyTemplateV2Result.EquipmentSlotsApplied`. `profileOrStatsApplied`
  skips `"equipment"` so equipment-only applies do not trigger the
  unrelated VM flush path.
- **Concurrency / atomicity** — the Phase 7a `rollbackBoth()` closure
  unchanged. The Phase 7b.0 writer's targeted hash 7 / hash 8
  recompute discipline is preserved (writes to slots 0–9 recompute
  hash 7; writes to slots 12–15 recompute hash 8). Validate-then-mutate
  ordering means the resolver may emit warnings without leaving the
  slot in a half-written state.
- **Entry points** — equipment templates apply through the canonical
  `ApplyBuildTemplateV2ToCharacterJSON` exit, so library Apply, direct
  YAML Apply, file Apply, URL preview Apply, and Apply with overrides
  all support equipment via the same backend path. Fast library Apply
  for equipment-only entries does NOT require a session.
- **Frontend** — `ImportTemplatePreviewModal.tsx` adds `equipment` to
  `V2_APPLY_SUPPORTED_SECTIONS`, renders an `import-preview-equipment-slots`
  row from `summary.equipmentSlotsPresent`, and tightens the unsupported
  message to enumerate the four shipped sections. `TemplateLibraryModal.tsx`
  enables Apply for equipment-only entries by extending
  `v2HasApplyableSections`. `TemplatesShellModal.tsx` is **unchanged** —
  the existing
  `needsSession = selectedSections.includes(INVENTORY_WORKSPACE_SECTION)`
  gate already excludes equipment-only templates, and the canonical
  JSON path serves library / direct YAML / URL identically. The
  Phase 6 `ApplyOverridesPanel` and Phase 7a.2 `WeaponLevelOverridePanel`
  are **not** extended — Phase 7b.1 ships without equipment-specific
  override controls.
- **Bindings** — `frontend/wailsjs/go/models.ts` regenerated by
  `make build`: `ApplyTemplateV2Result.equipmentSlotsApplied: number`
  and `ImportPreviewSummary.equipmentSlotsPresent?: string[]`. No App
  method signature changes; `frontend/wailsjs/go/main/App.{d.ts,js}`
  untouched.
- **Intentional non-changes** — no `App.tsx` change, no
  `SortOrderTab.tsx` change, no `ApplyOverridesPanel.tsx` /
  `WeaponLevelOverridePanel.tsx` change, no equipment-specific override
  panel, no new Wails App method, no `frontend/wailsjs/go/main/**`
  diff beyond what `make build` produces. Talisman writer, spell
  writer, EquippedGreatRune through the template path, slot 11/16,
  quick / pouch items, auto-add of missing inventory items, storage
  resolver, and lifting the equipment + inventory.workspace combo all
  remain future work (Phase 7c / 7d / 8 / 10).
- **Tests** — backend: `backend/templates/schema_equipment_test.go`
  (17 cases — JSON / YAML round-trip, per-field allowlist, reject
  unknown / great rune / negative upgrade / upgrade > 25 / zero AoW,
  explicit clear, combo guard in preview, helpers);
  `backend/templates/export_v2_equipment_test.go` (7 cases — boolean
  shortcut, per-field normalisation, deep clone, explicit clear,
  missing source, JSON shape, mixed profile + equipment);
  `app_templates_v2_equipment_test.go` (8 cases — empty section,
  weapon / armor / ammo encoding match, talisman / GreatRune / slots
  11/16 not exported, unreadable offset, unknown item fallback,
  multi-slot);
  `app_templates_v2_apply_equipment_test.go` (11 cases — resolver
  match-by-base-id, missing item warning, ambiguous match first wins,
  upgrade / infusion disambiguators, explicit clear, per-field
  selection, full apply combo rejection with zero side effects,
  sessionID silently ignored on equipment-only, missing item zero
  writes, unsupported selection key rejected via validator). Frontend:
  `ImportTemplatePreviewModal.test.tsx` (+4 Phase 7b.1 cases —
  applyable v2 section, slots row render, hide when empty, combo
  error display); `TemplateLibraryModal.test.tsx` (+2 cases — Apply
  enabled for equipment-only without session, disabled without
  save); `TemplatesShellModal.test.tsx` (+3 cases — no session
  required, sessionID transparently forwarded, backend rejection
  toast). Two previously-shipped tests that used `'equipment'` as the
  "unsupported section" example were updated to use `'spells'` (still
  genuinely planned) so the negative case stays meaningful.
- **Validation totals** — `go test ./backend/templates -run
  'TestEquipment|TestBuildV2Template_Equipment'` PASS; `go test .
  -run 'TestResolveEquipment|TestApplyBuildTemplateV2_Equipment|
  TestBuildEquipmentSection'` PASS; full `go test . ./backend/...
  ./tests/...` PASS (9 packages); `go vet` clean; `tsc --noEmit`
  clean; full `vitest run` 366/366 PASS across 17 suites; `make
  build` PASS with the Wails bundle rebuilt and only the expected
  `frontend/wailsjs/go/models.ts` diff.
- **Manual validation 2026-06-01** — equipment-only template export,
  preview, library round-trip, Apply, game-load reload confirmation,
  missing-item warning skip, ambiguous-match first wins, explicit
  clear, combo rejection, regression sweep (profile/stats Apply,
  inventory.workspace session gating, Phase 6b SortOrderTab dropdown,
  Phase 7a.2 weapon level override on inventory.workspace path,
  URL import). User-confirmed: manual OK.
- **Future work** — Phase 7c talisman writer + template apply (slots
  17–21), Phase 7d spells writer + template apply (14-slot loadout),
  optional lifting of the equipment + inventory.workspace combo
  restriction (would require either auto-commit of the workspace or
  a workspace-backed equipment model), Phase 8 appearance via preset,
  Phase 10 multi-character pack (`scope: pack`). EquippedGreatRune
  (slot 10) and unknown slots 11/16 remain out of scope across the
  Phase 7 family.

### feat(core): add equipment writer foundation

Shipped Phase 7b.0 of the Templates v2 design (`spec/56-templates-v2.md`):
a backend-only `(s *core.SaveSlot).WriteEquipment([]EquipmentWrite) error`
foundation that lifts the long-standing "no public write API for
ChrAsmEquipment" gap for weapon, ammo, and armor slots. This is the writer
foundation only — no template schema change, no template apply integration,
no UI, no Wails App method, no bindings regenerated. The Phase 7b.1
follow-up will wire this into the v2 `sections.equipment` template apply
path with strict "item must be in inventory" enforcement.

- **API shape** — `backend/core/equipment_writer.go` introduces the
  `EquipmentSlotKind` enum, the `EquipmentWrite { Slot, Handle }` request
  struct, and the slot-level method `WriteEquipment(writes []EquipmentWrite)
  error`. Only the 14 supported slots are exposed by the enum: weapon
  slots 0–5 (`LeftHandArmament1..3`, `RightHandArmament1..3`), ammo slots
  6–9 (`Arrows1/2`, `Bolts1/2`), and armor slots 12–15 (`Head`, `Chest`,
  `Arms`, `Legs`). Talisman slots 17–21, `EquippedGreatRune` (slot 10),
  unknown slots 11/16, the 14 `EquippedSpells`, and the 16 quick / pouch
  slots remain unexposed — each belongs to a future phase.
- **Handle encoding** — for each non-zero write, the writer resolves
  `handle → itemID` via `slot.GaMap` and writes the encoded
  ChrAsmEquipment form back to `slot.Data` at
  `slot.EquipItemsIDOffset + index*4`. Weapon and armor slots store
  `itemID | 0x80000000` (the `ItemTypeWeapon` high-bit flag); ammo slots
  store `itemID` directly because goods item IDs already carry the
  `0x40` prefix per the convention codified in
  `transfer.go::IsHandleEquipped` and `materializeRehandledInstance`.
  `Handle == 0` clears the slot to `0xFFFFFFFF`; `Handle == 0xFFFFFFFF`
  is rejected as a defensive guard (callers must use `Handle = 0` to
  clear).
- **Strict class gate** — every non-clear write checks the handle's type
  prefix (`handle & GaHandleTypeMask`) against the slot's class:
  weapon slots accept only `ItemTypeWeapon` (`0x80`); armor slots accept
  only `ItemTypeArmor` (`0x90`); ammo slots accept only `ItemTypeItem`
  (`0xB0`, goods). `ItemTypeAow` (`0xC0`) handles are rejected in weapon
  slots even though the read-side encoding rule (`id | 0x80000000`) would
  technically accept them — AoW equipping is intentionally deferred to a
  later phase. Talisman handles (`ItemTypeAccessory`, `0xA0`) and goods
  handles in non-ammo slots are rejected. Handles absent from `GaMap`
  (i.e. items not currently in the character's inventory) are rejected
  with `"not present in inventory (GaMap)"` — strict-reject, no
  auto-add. Auto-add belongs to the Phase 7b.1 template apply layer, not
  the writer.
- **Targeted hash recompute** — writes that touch any of slots 0–9
  trigger recompute of hash entry 7 (`weaponSlotIndices`); writes that
  touch any of slots 12–15 trigger recompute of hash entry 8 (the armor
  subset of `armorSlotIndices`). Unrelated hash entries — level, stats,
  class, souls, quick items, spells, talisman portion of hash 8 — are
  untouched. The recompute path reuses the existing `readEquipSection`,
  `extractSlots`, and `equipmentHash` helpers from `hash.go` so any
  future change to the hash algorithm propagates automatically.
- **Atomicity** — every write is validated against the slot
  table, handle prefix gate, and `GaMap` presence **before** any
  `slot.Data` byte is mutated. Duplicate slot kinds within a single batch
  are rejected with a deterministic error rather than silently last-wins.
  On any validation failure the equipment section bytes and hash bytes
  remain identical to their pre-call state.
- **Out of scope (intentional non-changes)** — no Templates v2 schema
  section for equipment, no `app_templates*.go` change, no Wails App
  method, no `frontend/wailsjs/**` change, no frontend component change,
  no `app.go` change, no `SortOrderTab.tsx` change, no
  `ApplyOverridesPanel.tsx` change, no `TemplatesShellModal.tsx` change.
  The writer is a callable Go-level API; it has no user-facing entry
  point in this phase. The Phase 7b.1 follow-up will introduce
  `sections.equipment` and route a Templates v2 apply through this
  writer.
- **Tests** — `backend/core/equipment_writer_test.go` adds 24 unit
  tests against a synthetic `SaveSlot` (no real save fixture required —
  `tests/data/pc` and `tests/data/ps4` are scratch dirs and the real
  PC/PS4 fixtures live under `tmp/save/` which the Phase 7b.0 scope
  explicitly excludes). The new tests cover: weapon / armor / ammo
  encoding correctness; rejection of AoW / talisman / goods handles in
  weapon slots; rejection of weapon handles in armor slots; rejection
  of handles missing from `GaMap`; clear-slot semantics; rejection of
  the `0xFFFFFFFF` sentinel; rejection of unknown enum values; rejection
  of duplicate slots within a batch; atomicity rollback when the second
  write in a batch is invalid; hash 7 changes after weapon and ammo
  writes; hash 8 changes after armor writes; hash 7 unchanged on
  armor-only writes; hash 8 unchanged on weapon-only writes; mixed
  weapon + armor batches touch both hashes; idempotent writes leave hash
  stable; nil-receiver guard; empty batch is a no-op; unparseable
  `EquipItemsIDOffset` is rejected; and an end-to-end weapon swap
  (equip A → swap to B → clear) round-trip.
- **PC + PS4 round-trip on a real save** — deliberately deferred to
  Phase 7b.1, when the template apply path will exercise the writer
  end-to-end on the existing round-trip fixtures. The synthetic
  `SaveSlot` tests exhaustively cover the writer's contract; reusing
  the existing PC/PS4 fixtures from `tmp/save/` is out of scope for the
  Phase 7b.0 closure session per the user's tmp-exclusion rule.
- **Validation** — `go test ./backend/core -run 'TestWriteEquipment'`
  passes 24/24; full `go test . ./backend/... ./tests/...` passes; `go
  vet . ./backend/... ./tests/...` clean; `make build` succeeds with
  the Wails bundle rebuilt and `frontend/wailsjs/**` unchanged.

### feat(templates): weapon level override for v2 inventory.workspace apply

Shipped Phase 7a.2 of the Templates v2 design (`spec/56-templates-v2.md`):
apply-time weapon level override threaded into the Phase 7a v2
`inventory.workspace` apply path. The Phase 7a slice deliberately
hard-coded a `nil` weapon override at the two
`applyTemplateItemsToWorkspace` call sites in
`app_templates_v2_apply.go` and tracked the wiring as Phase 7a.2;
this commit lifts that pin without touching the v1 `SortOrderTab.tsx`
Phase 6b dropdown that already shipped the same feature for v1
inventory templates. The override remains a **runtime apply option**,
not a template schema field — the canonical JSON the user previews
never carries it, and the option travels exclusively through the
`ApplyTemplateV2Options` struct.

- **`ApplyTemplateV2Options` extended** —
  `app_templates_v2_apply.go::ApplyTemplateV2Options` gains an additive
  `WeaponLevelOverride *WeaponLevelOverride json:"weaponLevelOverride,omitempty"`.
  The type is reused **verbatim** from the v1 path
  (`app_templates.go::WeaponLevelOverride { Enabled bool, StandardLevel *int, SomberLevel *int }`)
  so the bindings expose a single `WeaponLevelOverride` class shared
  between v1 and v2 surfaces. `validateWeaponLevelOverride` (also from
  the v1 path) runs **before** `acquireSession`, so a structurally
  broken override (`Enabled = true` with both level pointers nil, or a
  negative level) bounces with `templates.IssueCodeStructureInvalid` and
  **zero** side effects — no snapshot, no session acquire, no slot or
  workspace mutation.
- **Apply layer wiring** — `ApplyBuildTemplateV2ToCharacterJSON`
  replaces the two `nil` override arguments at the inventory and
  storage `applyTemplateItemsToWorkspace` call sites with
  `opts.WeaponLevelOverride`. `applyTemplateItemsToWorkspace`,
  `applyWeaponLevelOverride`, and the Phase 7a dual snapshot rollback
  (`rollbackBoth()` over `core.SnapshotSlot` + `deepCopySnapshot(sess.Workspace)`)
  are **unchanged**. Warnings from the override —
  `templates.IssueCodeWeaponLevelClamped` and
  `templates.IssueCodeWeaponUnupgradeable` — flow into
  `ApplyTemplateV2Result.Preview.Warnings` through the existing
  `invWarn`/`stoWarn` aggregation; they are warnings, never errors, so
  the apply does not roll back on a clamp / unupgradeable skip. A
  structurally valid override on a profile/stats-only template
  (`selection.inventory.workspace` absent) is silently ignored — the
  override loop simply has no items to operate on, mirroring the way
  `SessionID` is silently ignored on the non-inventory branch.
- **Issue codes** — **no new codes**. Phase 7a.2 reuses the Phase 6b
  `weapon_level_clamped` and `weapon_unupgradeable` warnings (declared
  in `backend/templates/import.go`) and the standard
  `structure_invalid` error for shape rejection.
- **Bindings** — `frontend/wailsjs/go/models.ts` regenerated by
  `make build`'s internal `wails generate module` step. The only delta
  is `ApplyTemplateV2Options` now carries
  `weaponLevelOverride?: WeaponLevelOverride` plus the matching line in
  its constructor. The `WeaponLevelOverride` class itself was already
  exported from the v1 path; no duplicate class was generated. No file
  in `frontend/wailsjs/` was hand-edited. `App.tsx` is untouched.
- **Frontend — new component** —
  `frontend/src/components/templates/WeaponLevelOverridePanel.tsx`
  (**new**, ~145 lines). Owns its own state (master `enabled`,
  `standardText`, `somberText`), exposes the unique testids
  `apply-overrides-weapon-{panel,enabled,standard,somber,error}` so
  they never collide with the v1 `weapon-override-*` testids on
  `SortOrderTab.tsx`. Standard input range `0..25`, Somber input range
  `0..10`; the panel emits a
  `{ enabled: true, standardLevel?: number, somberLevel?: number } | undefined`
  payload through the `onChange(override, hasInvalid)` callback. When
  the master toggle is enabled with both inputs empty the panel
  surfaces an inline error ("At least one level must be set.") and
  reports `hasInvalid = true`; the error message is range-specific for
  out-of-range numbers ("Standard level must be 0–25." /
  "Somber level must be 0–10."). Disabling the master toggle emits
  `undefined` and clears the invalid state. Default after open:
  disabled, both inputs empty — the user must explicitly opt in and
  type at least one level.
- **Frontend — `ApplyOverridesModal` integration** —
  `frontend/src/components/templates/ApplyOverridesPanel.tsx` is
  extended in two ways. (1) The modal now parses the canonical JSON to
  detect `selection['inventory.workspace']` (accepts either a boolean
  or an object with `all: true` / non-empty keys) and conditionally
  renders `WeaponLevelOverridePanel` under the profile/stats grid only
  when that condition is true. Profile/stats-only templates leave the
  weapon panel unrendered; inventory-only templates may use the modal
  exclusively for weapon level override. (2) `onConfirm` signature is
  widened to
  `(mutatedJSON: string, weaponOverride?: WeaponOverridePayload) ⇒ void | Promise<void>`.
  `canApply` blocks while either the profile/stats panel or the weapon
  panel reports invalid input; the modal status pill shows
  "Fix weapon level override to apply." when the weapon panel is the
  blocker. `ApplyOverridesPanel` itself (the JSON mutator) is
  unchanged — the runtime option does **not** travel through the
  canonical JSON.
- **Frontend — `TemplatesShellModal` wiring** —
  `frontend/src/components/templates/TemplatesShellModal.tsx`'s
  `handleConfirmOverrides` accepts the new second argument
  `weaponOverride?: WeaponOverridePayload` and forwards it as
  `weaponLevelOverride` inside
  `main.ApplyTemplateV2Options.createFrom({ mode, sessionID, weaponLevelOverride })`
  on every Apply-with-overrides surface (library, direct YAML import,
  URL import). The fast library Apply path
  (`ApplyBuildTemplateV2FromLibraryToCharacter`) still sends
  `{ mode, sessionID }` only — no weapon override. Phase 7a session
  gating still wins before the binding is called: an
  `inventory.workspace`-bearing template without an active Inventory
  Edit Session is rejected upstream regardless of the override state,
  and the binding is never reached.
- **What is intentionally not changed** — no new v2 schema section
  (Phase 7a.2 is a runtime option only); no equipment writer
  (Phase 7b remains future); no direct save mutation from the v2
  inventory path — the override runs entirely inside `sess.Workspace`
  via `editor.UpdateWeapon`; no Phase 6b dropdown rewrite — the v1
  `SortOrderTab.tsx` controls remain byte-for-byte the same and the
  v1 apply path (`ApplyBuildTemplateToWorkspaceJSON` with
  `ApplyTemplateOptions.WeaponLevelOverride`) is untouched; no
  `App.tsx` change; no `SortOrderTab.tsx` change. The fast library
  Apply contract is preserved: no overrides ever travel on that path.
  `SaveInventoryWorkspaceChanges` remains the only commit point that
  persists to `slot.Data`.
- **Tests** — `app_templates_v2_apply_weapon_override_test.go`
  (**new**, ~340 lines, 14 cases) covers: nil override no-op;
  disabled override no-op; standard-only override touches only
  standard weapons; somber-only override touches only somber weapons;
  both levels touch each class independently; standard +99 clamps to
  +25 with `weapon_level_clamped`; somber +99 clamps to +10 with the
  same code; `MaxUpgrade == 0` (unupgradeable arm) emits
  `weapon_unupgradeable` and skips; `Enabled = true` with both
  pointers nil rejected with `IssueCodeStructureInvalid` before any
  mutation; negative `StandardLevel` rejected; mixed
  profile+stats+inventory.workspace + override happy path lands all
  three sections and the override; mixed apply with invalid override
  shape rolls back all state (no slot bytes mutated, no workspace
  mutation, `Dirty` flag preserved); profile/stats-only template with
  a valid override silently ignored.
  `app_templates_v2_apply_inventory_test.go` updates the
  `TestApplyTemplateV2Options_FieldSurface` compile-time pin to
  reflect the new three-field shape (Mode, SessionID,
  WeaponLevelOverride). Phase 6b regression tests
  (`TestApplyTemplate_Override*`, `TestValidateWeaponLevelOverride*`)
  all still PASS unchanged.
  `frontend/src/components/templates/__tests__/WeaponLevelOverridePanel.test.tsx`
  (**new**, 9 cases) covers: default state emits undefined; enabling
  reveals both inputs; enable + both empty surfaces error and sets
  `hasInvalid = true`; standard-only / somber-only / both emissions
  match the v1 `buildWeaponLevelOverride` shape; standard +26 and
  somber +11 trigger range-specific errors; disabling after typing
  values emits undefined and clears invalid.
  `frontend/src/components/templates/__tests__/ApplyOverridesPanel.test.tsx`
  gains a `ApplyOverridesModal — Phase 7a.2 weapon override` block
  (+5 cases): the weapon panel renders only when the canonical JSON
  selects `inventory.workspace`; profile/stats-only templates do not
  render it; inventory-only templates do; invalid weapon override
  disables the Apply button; `onConfirm` receives the weapon override
  as its second argument (mutated JSON keeps `inventory.workspace`
  intact and never carries the override field); leaving the weapon
  panel untouched forwards `undefined` for the override.
  `frontend/src/components/templates/__tests__/TemplatesShellModal.test.tsx`
  gains a `Phase 7a.2 v2 weapon level override` block (+6 cases):
  Apply with overrides on an inventory.workspace template + filled
  weapon override forwards `weaponLevelOverride` in the options bag;
  untouched weapon panel sends `weaponLevelOverride: undefined`;
  mixed-template apply forwards both the mutated profile/stats JSON
  and the override; fast library Apply never sends the override;
  invalid weapon override blocks Apply and never calls the backend;
  no-session gating wins before the override reaches the binding.
  Validation: targeted `go test .` for
  `TestApplyBuildTemplateV2_Inventory_Override*`,
  `TestApplyBuildTemplateV2_Mixed_.*Override`,
  `TestApplyBuildTemplateV2_ProfileStatsOnly_WithOverride_Ignored`,
  `TestApplyTemplate_Override*`,
  `TestValidateWeaponLevelOverride*`,
  `TestApplyTemplateV2Options_FieldSurface` all PASS (30/30).
  Full backend (`go test . ./backend/... ./tests/...`) all packages
  PASS; `go vet` clean; `tsc --noEmit` clean; targeted vitest (3
  files) 106/106 PASS; full vitest 17 suites / **357 PASS** (was
  336 before Phase 7a.2, +21 cases); `make build` PASS.
- **Manual validation** — 2026-05-31 on
  `feature/templates-v2-weapon-override` (HEAD `8fccd72`). User-confirmed
  end-to-end on a real save:
  fast Apply without overrides keeps weapon upgrade levels as
  declared in the template;
  `Apply with overrides…` with Standard = 25 sets every standard
  weapon to +25 (or clamps with `weapon_level_clamped`) and leaves
  somber weapons at their template-side levels;
  Somber = 10 mirrors the symmetric case;
  both levels set apply per-class independently;
  enabling the override with both fields empty disables Apply;
  Standard = 26 / Somber = 11 / negative values disable Apply with
  the inline error;
  closing the Sort Order workspace before applying surfaces the
  Phase 7a no-session toast — the override never reaches the
  backend;
  a mixed profile + stats + inventory.workspace template with
  override applied atomically (profile/stats fields, inventory items,
  and weapon levels in a single user action);
  a profile/stats-only template with a valid override applied
  without rendering the weapon panel and without override warnings;
  URL import and library `Apply with overrides…` reused the same
  canonical JSON path and the same override field;
  the v1 `SortOrderTab.tsx` Phase 6b dropdown continued to work
  byte-for-byte unchanged.
- **Out of scope (still future work)** — equipment writer for
  `ChrAsmEquipment` slots 0..9 / 12–15 (Phase 7b); equipped-talismans
  writer (Phase 7c); spell loadout writer (Phase 7d); appearance via
  preset (Phase 8); multi-character pack (Phase 10); optional future
  UX cleanup / consolidation of the v1 `SortOrderTab.tsx` Phase 6b
  dropdown with the Templates shell now that the v2 path also carries
  the override (Phase 12, separately approved).

### feat(templates): apply v2 inventory templates to workspace

Shipped Phase 7a of the Templates v2 design (`spec/56-templates-v2.md`):
the first real v2 apply path for `inventory.workspace`. Until this phase
the v2 apply layer only handled `profile` and `stats`; v2 templates that
also carried `inventory.workspace` were rejected by the
`app_templates_v2_apply.go` scope guard. Phase 7a lifts that guard for
`inventory.workspace` only, routing the new payload through the **active
Inventory Edit Session** so the writes land in the workspace snapshot
that `SortOrderTab` already operates on — never directly into
`slot.Data`. The user opens the Sort Order workspace first, applies the
v2 template (from library, direct YAML import, URL import, or
`Apply with overrides…`), and then commits the change exactly as today
via `Save changes`. Equipment, equipped talismans, spell loadout, and
appearance writers remain future work (Phase 7b / 7c / 7d / 8), and the
Phase 6b weapon level override is **not** wired into the v2 path in
this phase — Phase 6b stays a v1 `SortOrderTab.tsx` Templates dropdown
feature.

- **New backend endpoint** —
  `App.GetActiveInventoryEditSessionForCharacter(charIdx int) (ActiveInventoryEditSession, error)`
  on `app_inventory_session.go`. Returns
  `{ active: bool, sessionID: string }` after a short `editSessionsMu`
  read so the shell can look up the active session without touching
  `App.tsx`. Returns `{ active: false }` for invalid `charIdx` and for
  characters with no active session — never errors.
- **`ApplyTemplateV2Options` extended** —
  `app_templates_v2_apply.go::ApplyTemplateV2Options` gains an additive
  `SessionID string json:"sessionID,omitempty"`. With `SessionID = ""`
  the apply behaves byte-for-byte like the pre-Phase-7a path for
  profile/stats-only templates; `sessionID` is silently ignored on the
  non-inventory path. For a template carrying `inventory.workspace`,
  `SessionID` is **required** — `ApplyBuildTemplateV2ToCharacterJSON`
  rejects an empty session with the new
  `templates.IssueCodeInventorySessionRequired = "inventory_session_required"`
  and rejects an unknown or wrong-character session with the new
  `templates.IssueCodeInventorySessionInvalid = "inventory_session_invalid"`.
  Both codes are surfaced through the existing `ImportPreviewIssue`
  shape on `ApplyTemplateV2Result.Errors`.
- **`ApplyTemplateV2Result` extended** — adds
  `InventoryItemsApplied int`, `StorageItemsApplied int`, and an
  optional `Workspace *editor.InventoryWorkspaceSnapshot` carrying the
  fresh workspace snapshot after a successful inventory apply so the
  UI can refresh the Sort Order view without a second binding round
  trip.
- **Apply layer** — `ApplyBuildTemplateV2ToCharacterJSON` now
  classifies the parsed v2 payload into three flags (`hasProfile`,
  `hasStats`, `hasInventory`) and gates the work on them:
  - `hasInventory == true` → acquires the session through the standard
    `acquireSession(sess.ID)` path (full `lifecycleMu` + `sess.mu`
    ordering preserved); verifies `sess.CharacterIndex == charIdx` or
    unlocks and emits `IssueCodeInventorySessionInvalid`; runs capacity
    preflight on the existing workspace **before** any mutation; takes
    a `core.SnapshotSlot` of `slot.Data` and a value-type deep copy of
    `sess.Workspace`; applies inventory and storage items via
    `applyTemplateItemsToWorkspace(&sess.Workspace, …, editor.ContainerInventory, nil)`
    and the storage equivalent (the `nil` override pin documents that
    Phase 6b weapon override is **not** wired into v2 in this phase);
    marks `sess.Workspace.Dirty = true` and revalidates the snapshot;
    appends the sentinel section `"inventory.workspace"` to the result's
    `Applied` list.
  - `hasProfile || hasStats` runs **after** the inventory branch under
    the same `slotMu[charIdx]` window, reusing the Phase 5 path
    (`vm.ApplyVMToParsedSlot` + `slot.SyncPlayerToData`) unchanged.
  - **Mixed apply** (profile + stats + inventory.workspace in the same
    v2 template) is atomic: a `rollbackBoth()` closure restores both
    the slot byte snapshot (`core.RestoreSlot`) and the workspace value
    snapshot on **every** error exit between acquire and final write.
    If profile/stats validation fails after inventory writes already
    landed, both the slot and the workspace revert in a single step.
  - `hasInventory == false` preserves the Phase 5 edit-session conflict
    guard exactly as before — a session existing for another open
    edit operation still blocks profile/stats-only applies.
- **Frontend — TemplatesShellModal** —
  `frontend/src/components/templates/TemplatesShellModal.tsx` looks up
  the active session whenever the user attempts a v2 apply for a
  template that carries `inventory.workspace`:
  - `handleApplyV2FromLibrary` reads `entry.selectedSections` and, when
    `'inventory.workspace'` is in the list, calls
    `GetActiveInventoryEditSessionForCharacter(charIndex)`. No active
    session → toast "Open the Sort Order workspace before applying
    inventory templates." and the binding is **not** called.
  - `handleApplyV2FromImportedPreview` repeats the same check against
    `importedPreview.report.summary?.selectedSections` so direct YAML
    import and URL import behave identically.
  - `handleConfirmOverrides` parses the mutated canonical JSON,
    inspects `selection['inventory.workspace']`, and runs the same
    session check before posting through `ApplyBuildTemplateV2ToCharacterJSON`.
  - Profile/stats-only v2 applies forward an empty `sessionID` and
    the backend ignores it — no UI change on that path.
- **Frontend — section gating** —
  `frontend/src/components/templates/ImportTemplatePreviewModal.tsx`
  bumps its module-level `V2_APPLY_SUPPORTED_SECTIONS` to
  `['profile', 'stats', 'inventory.workspace']`.
  `frontend/src/components/templates/TemplateLibraryModal.tsx` mirrors
  the change in its per-row `v2HasApplyableSections` test so v2
  library entries that only carry `inventory.workspace` (or any
  combination of the three) finally show the green Apply / overrides
  buttons instead of the disabled "schema v2 not supported" tooltip.
- **Frontend bindings** — `frontend/wailsjs/go/main/App.{d.ts,js}` and
  `frontend/wailsjs/go/models.ts` were regenerated by `make build`'s
  internal `wails generate module` step. Additions: the
  `ActiveInventoryEditSession` class + the new
  `GetActiveInventoryEditSessionForCharacter` method; `sessionID?:
  string` on `ApplyTemplateV2Options`; `inventoryItemsApplied`,
  `storageItemsApplied`, optional `workspace?: editor.InventoryWorkspaceSnapshot`
  on `ApplyTemplateV2Result`. No bindings file was hand-edited.
- **What is intentionally not changed** — no new v2 schema section
  (Phase 7a uses the existing `inventory.workspace` section from v1);
  no equipment writer (Phase 7b remains future); no direct save
  mutation from the v2 inventory path — all writes land in
  `sess.Workspace`; no Phase 6b weapon level override on the v2 path
  (the `nil` override pin in `applyTemplateItemsToWorkspace` is the
  contract); no `App.tsx` change; no `SortOrderTab.tsx` change; no
  `ApplyOverridesPanel.tsx` change. `SaveInventoryWorkspaceChanges`
  remains the only commit point that persists to `slot.Data`.
- **Tests** — `app_templates_v2_apply_inventory_test.go` (**new**,
  ~280 lines, 8 tests) covers: unknown session ID rejected with
  `IssueCodeInventorySessionInvalid`; session for a different
  character rejected; happy-path inventory apply lands the Dagger
  fixture at the end of inventory with `InventoryItemsApplied == 1`
  and `Workspace.Dirty == true`; empty items list with a valid
  session is a no-op (`Applied == false`); mixed
  profile+stats+inventory.workspace lands all three sections;
  `ApplyTemplateV2Options` field surface pinned via a compile-time
  reflection test; strict-decode of unknown sections still rejects.
  `app_templates_v2_apply_test.go` updates the existing three
  "without session" tests to expect the new
  `IssueCodeInventorySessionRequired` instead of the old
  "inventory.workspace not supported" tag.
  `frontend/src/components/templates/__tests__/TemplatesShellModal.test.tsx`
  gains a `Phase 7a v2 inventory.workspace apply` block with 8
  cases: library apply for inventory.workspace + no session → toast
  + binding NOT called; library apply with active session → forwards
  sessionID; mixed entry → forwards sessionID; profile-only entry →
  proceeds with `sessionID = ''`; direct YAML apply + no session →
  error toast; direct YAML apply + session → forwards sessionID;
  overrides apply on inventory-bearing JSON without session → toast;
  overrides apply on profile-only JSON proceeds with `sessionID = ''`.
  `frontend/src/components/templates/__tests__/TemplateLibraryModal.test.tsx`
  rewrites two cases that previously asserted the Phase 5 "inventory
  apply not supported" disabled state — they now assert that v2
  inventory.workspace entries render the Apply / overrides buttons
  with the standard tooltip.
  Validation: targeted Phase 7a backend tests all PASS;
  `TestApplyTemplate_Override*` + `TestValidateWeaponLevelOverride*`
  Phase 6b regression all PASS; full backend (`go test . ./backend/...
  ./tests/...`) 8/8 PASS; `go vet` clean; `tsc --noEmit` clean;
  targeted vitest (TemplatesShellModal 66 PASS, ImportTemplatePreviewModal
  43 PASS, TemplateLibraryModal 45 PASS); full vitest 16 suites / 336
  PASS (was 328, +8 Phase 7a tests); `make build` PASS.
- **Manual validation** — 2026-05-31 on
  `feature/templates-v2-inventory-workspace-apply` (HEAD `3e448f0`).
  Confirmed end-to-end on a real save: applying a v2 template with
  `inventory.workspace` selected without an open Sort Order workspace
  surfaced the "Open the Sort Order workspace before applying
  inventory templates." toast and did **not** call the binding;
  opening the workspace and reapplying landed the items in the
  workspace grid; `Save changes` committed them to `slot.Data`;
  reloading the save showed the items with correct acquisition
  indices and no integrity warnings. A mixed profile + stats +
  inventory.workspace v2 template applied atomically — profile/stats
  fields updated and inventory items landed in a single user action.
  Both URL import and library apply re-used the same canonical JSON
  path. Phase 5 / 5D.2 / 6 v2 Apply paths for profile/stats-only
  templates, Phase 6b weapon level override on the v1 SortOrderTab
  path, and the Phase 9 URL import path were all unaffected.
- **Out of scope (still future work)** — Phase 6b weapon level
  override wired into the v2 `inventory.workspace` apply path
  (Phase 7a.2 / Phase 7b); equipment writer for `ChrAsmEquipment`
  slots 0..9, 12–15 (Phase 7b); equipped-talismans writer
  (Phase 7c); spell loadout writer (Phase 7d); appearance via preset
  (Phase 8); multi-character pack (Phase 10).

### feat(templates): weapon level override for v1 inventory.workspace templates

Shipped Phase 6b of the Templates v2 design (`spec/56-templates-v2.md`):
apply-time weapon level override for the existing v1
`inventory.workspace` Apply path. The override is a runtime apply
option only — no schema change, no new v2 inventory writer, no direct
save mutation, no equipment writer. It lets the user, on the existing
Sort Order Templates Apply, force every standard-upgradeable weapon
the template adds to `+N` and every somber/special weapon to `+M`,
clamped to each weapon's `MaxUpgrade` from the DB; the user still
commits with `Save changes` exactly as today.

- **Runtime option** — `app_templates.go::ApplyTemplateOptions` gains
  an additive `WeaponLevelOverride *WeaponLevelOverride` field. The
  shape is `{ Enabled bool, StandardLevel *int, SomberLevel *int }`
  so the UI can target both classes at once; with `Enabled = false`
  (default) the apply behaves byte-for-byte like the pre-Phase-6b
  path. `validateWeaponLevelOverride` rejects `Enabled = true` with
  both level pointers nil and rejects negative levels — surfaced as
  the same `ApplyBuildTemplate: …` error prefix the rest of
  `app_templates.go` uses, **before** any workspace mutation runs.
- **Apply layer** — `applyTemplateItemsToWorkspace` now threads the
  override through and calls a new `applyWeaponLevelOverride` helper
  **after** the template's own Upgrade / Infusion / AoW patches land
  through `editor.UpdateWeapon`. The helper switches on
  `editor.EditableItem.MaxUpgrade` (already populated by
  `editor.AddItem` via `db.GetItemDataFuzzy`):
  - `MaxUpgrade == 25` → standard; applies `StandardLevel` if non-nil
    via `editor.ClampUpgrade(req, 25)` and `editor.UpdateWeapon` with
    a `WeaponPatch{Upgrade: &clamped}`. Emits
    `templates.IssueCodeWeaponLevelClamped` to `report.Warnings` when
    the requested level was clamped down.
  - `MaxUpgrade == 10` → somber/special; same flow with
    `SomberLevel` and a `+10` clamp.
  - `MaxUpgrade == 0` → unupgradeable; the override is skipped and
    `templates.IssueCodeWeaponUnupgradeable` is appended to
    `report.Warnings`.
  - Any other `MaxUpgrade` → silent skip (non-weapon entries, future
    DB additions). The override never adds, removes, or relocates
    items, and never touches `Infusion` or `AoW`.
- **Issue codes** — `backend/templates/import.go` gains two new
  warning codes: `IssueCodeWeaponLevelClamped = "weapon_level_clamped"`
  and `IssueCodeWeaponUnupgradeable = "weapon_unupgradeable"`. Both
  are surfaced via the standard `ImportPreviewIssue` shape on the
  apply report's `Warnings`, never on `Errors` — the apply never
  blocks on an override warning.
- **Safety** — the override runs entirely inside the active
  `InventoryWorkspaceSnapshot` through `editor.UpdateWeapon`; no
  bytes are written to `slot.Data` from the override path itself.
  `SaveInventoryWorkspaceChanges` remains the only commit point and
  is **never** called automatically by the override. The Phase 5/5D.2
  v2 `inventory.workspace` scope guard in
  `app_templates_v2_apply.go` is preserved unchanged — Phase 6b is
  strictly a v1 apply-path option, not a new v2 schema section, and
  reuses the pre-existing snapshot + rollback already wrapping the
  workspace apply.
- **Frontend** — changed file:
  - `frontend/src/components/SortOrderTab.tsx` — Templates dropdown
    gains an inline `weapon-override-panel` (testid) with a master
    `weapon-override-enabled` checkbox and two number inputs
    (`weapon-override-standard` range `0..25`,
    `weapon-override-somber` range `0..10`). Empty fields stay
    `null` (meaning "leave that class alone"). Inline validation
    (`weapon-override-error`) blocks Apply when `Enabled = true` and
    both fields are empty or either is out of range; the Apply call
    builds an `ApplyTemplateOptions` with a `weaponLevelOverride`
    payload only when the user opts in. Override warnings on the
    report surface as a single dedicated toast so the user can see
    which weapons were clamped or skipped.
- **UI placement decision** — controls live inside the existing
  `SortOrderTab.tsx` Templates dropdown. `TemplatesShellModal`,
  `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, and
  `TemplateLibraryModal` are intentionally **not** touched in this
  phase. The global Templates shell does not expose the override
  because its v2 Apply paths do not reach the v1
  `inventory.workspace` writer; once Phase 6b's v2 counterpart ships
  (Phase 6b/Phase 7+), the panel can be relocated.
- **Bindings** — `frontend/wailsjs/go/models.ts` regenerated by
  `make build`'s internal `wails generate module` step to add the
  `WeaponLevelOverride` class and the optional
  `weaponLevelOverride?: WeaponLevelOverride` field on
  `ApplyTemplateOptions`. No other file in `frontend/wailsjs/` was
  hand-edited. `App.tsx` is untouched.
- **Tests** — `app_templates_weapon_override_test.go` (**new**,
  ~390 lines, 14 test functions / 16 cases with subtests) covers:
  validator accepts `nil` and disabled override; validator rejects
  `Enabled = true` with both level pointers nil; validator rejects
  negative `StandardLevel` and `SomberLevel`; override `nil` and
  `Enabled = false` leave upgrade untouched; `Enabled = true` with
  both fields nil is rejected before any workspace mutation;
  `StandardLevel` only touches standard weapons; `SomberLevel` only
  touches somber weapons; both together touch each class
  independently; clamping at `+26` standard and `+11` somber emits
  `IssueCodeWeaponLevelClamped` and lands at `+25` / `+10`; values
  exactly at `MaxUpgrade` produce no warning;
  `MaxUpgrade == 0` (unarmed) emits `IssueCodeWeaponUnupgradeable`
  and skips; on preview error the workspace stays unchanged; clamp
  to zero from a zero request emits no warning.
  `frontend/src/components/SortOrderTab.test.tsx` adds a new
  `Phase 6b weapon level override` block (+8 cases): panel renders
  collapsed by default; toggling on reveals both inputs and the
  required-one inline error; Apply with override disabled sends
  `weaponLevelOverride = undefined`; Apply with both levels filled
  sends both; Apply with only `StandardLevel` filled sends only
  `standardLevel`; Apply with `Enabled = true` and no levels is
  blocked with an error toast; Apply with an out-of-range
  `StandardLevel` is blocked; override warnings surfacing in the
  apply report toast a hint. Targeted `go test .` for
  `TestApplyTemplate_Override*` + `TestValidateWeaponLevelOverride*`
  all PASS; full backend (`go test . ./backend/... ./tests/...`)
  8/8 PASS; `go vet` clean; `tsc --noEmit` clean; targeted vitest
  for `SortOrderTab.test.tsx` 31 PASS; full vitest 16 suites /
  328 PASS; `make build` PASS.
- **What is intentionally not changed** — no v2 schema section
  added; no v2 `inventory` / `weapons` apply added (the
  `inventory.workspace` scope guard in
  `app_templates_v2_apply.go` is preserved); no
  `TemplatesShellModal` change; no `ApplyOverridesPanel` change; no
  `ImportTemplatePreviewModal` change; no `TemplateLibraryModal`
  change; no direct save mutation through
  `core.PatchWeaponItemID` from the template apply; no equipment
  writer; no inventory / world / sort-order apply added.
- **Manual validation** — 2026-05-31 on
  `feature/templates-v1-weapon-level-override`. Applying a v1
  template with mixed standard + somber + unupgradeable weapons
  under each control combination (override off / standard only /
  somber only / both / standard above `+25` / somber above `+10`)
  produced the expected per-class behaviour with the matching
  warning lines; the workspace mutated only through
  `editor.UpdateWeapon`; the user still committed by `Save changes`;
  the Phase 5/5D.2/6 v2 Apply paths and the Phase 9 URL import path
  were unaffected.

### feat(templates): URL import with SSRF guards

Shipped Phase 9 of the Templates v2 design (`spec/56-templates-v2.md`):
import a build template directly from an `https://` URL through the
existing preview-first flow. Reuses Phase 5D.2's canonical-JSON Apply
and Phase 6's apply-time overrides without any schema, library, or
`App.tsx` change. The fetch lives entirely in the backend, never
mutates the save or the library on its own, and is gated behind the
full §12 SSRF guard list before a single byte is read.

- **New endpoint** —
  `PreviewBuildTemplateImportYAMLFromURL(rawURL string) (LoadedTemplatePreview, error)`
  on the existing `App` struct. Returns the same
  `LoadedTemplatePreview { Report, JSON, Path }` shape that
  `PreviewBuildTemplateImportYAMLFromFile` already returns, with the
  user-supplied URL echoed back in `Path`. A guard violation is
  surfaced as a non-OK `Report` with a tagged error code; the call
  never panics, never returns a typed error for guard rejection, and
  never reaches `previewYAMLPayload` if the fetch failed.
- **Backend fetcher** — `backend/templates/url_import.go::FetchYAMLFromURL`
  implements all 13 §12.3 guards in one pass:
  HTTPS-only scheme; pre-connect IP filter on literal IPs **and**
  every DNS-resolved address (loopback, RFC1918 private, link-local,
  ULA, multicast, broadcast/wildcard, cloud metadata
  `169.254.169.254` + `fd00:ec2::254`); custom
  `http.Client.CheckRedirect` re-checking scheme and re-resolving +
  re-filtering on every hop, capped at 3 redirects; 1 MiB
  `io.LimitReader` body cap; 10 s total / 5 s idle / 5 s
  TLS-handshake / 5 s response-header / 5 s dial timeouts; strict
  TLS (`MinVersion: tls.VersionTLS12`, system root CAs only, no
  `InsecureSkipVerify`); stable identifying User-Agent; Content-Type
  allowlist (`application/json`, `application/yaml`,
  `application/x-yaml`, `text/yaml`, `text/plain`); no auth, no
  cookies, no custom headers; YAML still parsed strictly through the
  shared `previewYAMLPayload` path (struct-typed `gopkg.in/yaml.v3`,
  no aliases / no includes / no executable types); no auto-refresh;
  the fetch alone never mutates anything.
- **Wails handler** — `app_templates_url.go::PreviewBuildTemplateImportYAMLFromURL`
  trims whitespace, surfaces `IssueCodeURLEmpty` on empty input,
  delegates to `templates.FetchYAMLFromURL`, and on success hands the
  bytes to the same `previewYAMLPayload` the file-import path uses.
  `Report.Path` always echoes the user-supplied URL, even on guard
  rejection, so the UI can show "what was rejected" in the inline
  error.
- **Frontend** — changed files:
  - `frontend/src/components/templates/ImportTemplateFromURLModal.tsx`
    (**new**) — small dedicated modal with a single `https://` URL
    input, an `Enter`-to-submit shortcut, an in-flight "Fetching…"
    state, inline error rendering that preserves the input on
    rejection, and a Cancel that calls the parent's callback
    without touching the binding.
  - `frontend/src/components/templates/TemplatesShellModal.tsx` —
    wires a new "Import from URL…" header button (testid
    `templates-shell-import-url`) next to the existing file-import
    button. The shell-level `onURLImportPreview(rawURL)` callback
    invokes `PreviewBuildTemplateImportYAMLFromURL`; on `report.ok =
    true` it opens the existing `ImportTemplatePreviewModal` with
    `{ report, canonicalJSON: bundle.json, path: bundle.path ?? rawURL }`
    and closes the URL modal; on `report.ok = false` it returns
    `{ ok: false, error }` so the URL modal can show the error
    inline without losing the user's input.
- **Preview-first invariant preserved.** The same
  `ImportTemplatePreviewModal` instance backs both URL-imported and
  file-imported templates. There is no parallel "URL preview" surface
  to keep in sync. All three downstream actions ship without
  modification:
  - **Save to Library** — reuses the existing file-import path; the
    URL becomes the source label, and the persisted library entry is
    byte-for-byte the same canonical JSON the preview displayed. **No
    `sourceURL` metadata is added to the library schema in this
    phase** — the library still records what was saved, not where it
    came from.
  - **Apply to character** — reuses Phase 5D.2's
    `ApplyBuildTemplateV2ToCharacterJSON(charIdx, canonicalJSON,
    { mode: "append" })` on the canonical JSON the user just
    confirmed; no second fetch, no TOCTOU re-read, no second URL hit.
  - **Apply with overrides…** — reuses Phase 6's
    `ApplyOverridesModal` end-to-end; the mutated canonical JSON is
    posted through the same Phase 5D.2 endpoint.
- **No library schema change, no `App.tsx` change, no bindings
  hand-edit** — `frontend/wailsjs/go/main/App.{d.ts,js}` were
  regenerated by `make build`'s internal
  `wails generate module` step, picking up the new
  `PreviewBuildTemplateImportYAMLFromURL` signature; `models.ts` was
  unchanged because the new endpoint reuses
  `LoadedTemplatePreview`. `ApplyBuildTemplateV2FromFileToCharacter`
  still exists backend/bindings-side and stays unwired in UI for the
  same WYSIWYG reason Phase 5D.2 documented.
- **Tests** — `backend/templates/url_import_test.go` covers the
  fetcher with 28 cases plus `TestIsAllowedAddr` with 21 subtests
  (public v4 + v6 allowed; loopback / RFC1918 / link-local / ULA /
  metadata / multicast / unspecified / broadcast denied; redirect
  re-check on each hop including resolved-host re-filtering;
  HTTPS-only enforcement; oversized body rejection at the cap;
  Content-Type allowlist enforcement; bad-status mapping;
  redirect-cap enforcement; TLS error mapping; timeout mapping; no
  auth / no cookies / no custom headers under load). The Wails
  handler integration tests in `app_templates_url_test.go` cover the
  six wiring cases — empty URL, whitespace-only URL,
  `http://` scheme rejection, `data:` scheme rejection, literal
  loopback `127.0.0.1`, literal cloud-metadata `169.254.169.254` —
  proving the handler translates `templates.FetchError` into a
  non-OK `LoadedTemplatePreview` with the right `IssueCode*` tag and
  preserves the user-supplied URL in `Path`. Frontend tests:
  `__tests__/ImportTemplateFromURLModal.test.tsx` exercises 10
  cases (render, disabled-empty, disabled-non-http(s), trimmed-URL
  forwarding, in-flight state, inline error preserving input,
  retry clears error, thrown error surfaced, Cancel does not call
  the binding, Enter triggers Preview); the existing
  `__tests__/TemplatesShellModal.test.tsx` gains a `Phase 9 URL
  import` block with 9 cases plus an updated baseline invariant
  (button visible, successful preview opens
  `ImportTemplatePreviewModal` with the URL path, Save to Library
  forwards the canonical JSON, Apply to character forwards the
  canonical JSON, Apply with overrides routes through Phase 6, v1
  URL imports never see v2 buttons, `report.ok = false` keeps the
  URL modal open with the inline error, thrown binding error keeps
  the URL modal open, Cancel closes without calling the binding).
- **Manual validation 2026-05-31** — end-to-end on the
  `feature/templates-v2-url-import` branch: pasting a public
  `https://` URL serving a v2 YAML opens the preview through the
  same modal as a file import; Save to Library persists the URL
  payload as a library entry whose canonical JSON matches what the
  preview displayed; Apply to character writes the selected fields
  through Phase 5D.2's endpoint; Apply with overrides routes the
  edited canonical JSON through the same endpoint. SSRF guards
  rejected, in turn, an `http://` URL, a literal `127.0.0.1`, a
  literal `169.254.169.254`, an empty URL, and a whitespace-only
  URL — each with the correct `IssueCode*` tag visible inline in
  the URL modal.
- **Out of scope (still future work)** — apply-time weapon level
  override (Phase 6b); inventory / storage / equipment / spells /
  appearance / sort order / world progress apply (Phase 7a–7c, 8);
  optional `sourceURL` metadata in the library schema (deferred
  until a separate decision approves widening the library shape);
  authenticated downloads (basic / bearer / cookies) — explicitly
  excluded by §12.3; domain allowlist; URL auto-refresh; direct
  apply without preview.

### refactor(weapons): expose upgrade clamping from editor

Relocated the pure `clampUpgrade(requested, max int) int` helper from
`app.go` (`package main`) to `backend/editor/weapon.go` as the exported
`editor.ClampUpgrade`. Behaviour is byte-for-byte unchanged; the
`AddItemsToCharacter` add path (`app.go`, ashes / standard / somber
branches) calls the relocated helper through the existing `editor`
import. This is the prerequisite refactor called out in
`spec/56-templates-v2.md` §14.4 / §17 Phase 6b: any backend-side plan
layer (e.g. a future Templates v2 weapon-level override or inventory
apply pipeline) must be able to import the clamp helper without
pulling in the `main` package, and now can.

- **No behaviour change.** Same `[0, max]` clamp, same negative-input
  flooring, same defensive negative-`max` treatment. The original
  somber-clamping regression case (`TestAddItemsToCharacter_ClampsSomberUpgrade`
  for Golem Greatbow `+25 → +10`) is unchanged and still passes.
- **Test coverage** — pure-function tests for `editor.ClampUpgrade`
  now live in `backend/editor/weapon_test.go::TestClampUpgrade` (13
  named subtests covering somber over/exact/in-range, standard
  over/exact/in-range, zero, non-upgradeable cap, negative request,
  negative cap, both negative). The previous `app`-package case set is
  kept as a regression guard in
  `app_weapon_upgrade_clamp_test.go::TestClampUpgrade_AppPathContract`,
  which now exercises the relocated helper through the same boundary
  conditions the add path relies on.
- **No frontend, no bindings, no Templates schema change.** The
  `editor` package was already imported by `app.go`; no new dependency
  was introduced. `ApplyTemplateV2Options` keeps its single `Mode`
  field. `frontend/wailsjs/**` is untouched.
- **Weapon-level override is NOT implemented in this commit.** Phase
  6b is still design-only — the relocation only unblocks the import
  path; the apply layer still rejects anything outside profile/stats
  and the v2 schema still has no `sections.weapons` /
  `sections.inventory` apply path. Full weapon / inventory apply
  remains future work (Phase 6b for the override, Phase 7a for the
  equipment writer per spec/56 §17).

### feat(templates): apply-time overrides for schema v2 profile + stats

Shipped Phase 6 of the Templates v2 design (`spec/56-templates-v2.md`):
edit-before-apply for the safe profile + stats subset, reachable from
both the direct YAML import flow and the local library. The user can
override individual values and toggle fields on/off before the apply
ever reaches the backend; the canonical JSON is mutated in the
frontend and forwarded to the existing Phase 5 backend writer with no
schema, options, or bindings changes.

- **No backend, no bindings, no `App.tsx` change.** Phase 6 reuses
  Phase 5D.1's `ApplyBuildTemplateV2ToCharacterJSON(charIdx,
  mutatedCanonicalJSON, { mode: 'append' })` for both surfaces. The
  shell's existing `onCharacterTemplateApplied` callback continues to
  drive the post-apply refresh dance (`inventoryVersion`,
  `saveLoadKey`, slot list, undo depth) unchanged.
  `ApplyBuildTemplateV2FromFileToCharacter` still exists
  backend/bindings-side but stays **unwired in UI**.
- **Frontend (overrides surface)** — changed files:
  - `frontend/src/components/templates/ApplyOverridesPanel.tsx`
    (**new**) — owns the per-field draft state, range-validates on
    every keystroke, and re-emits a mutated canonical JSON whenever
    the draft changes. Also exports a thin `ApplyOverridesModal`
    wrapper used by the shell, plus the pure
    `applyOverridesToCanonical` helper for unit-level coverage.
    Anything outside `sections.profile` / `sections.stats` (other
    sections, `schema`, `version`, metadata, the rest of `selection`)
    is preserved verbatim by the JSON round-trip.
  - `frontend/src/components/templates/ImportTemplatePreviewModal.tsx`
    — adds the second v2 button "Apply with overrides…" (testid
    `import-preview-apply-v2-overrides`) next to the existing "Apply
    to character"; refactored the v2 gating so the disabled-reason
    logic is shared across both v2 buttons. v1 imports continue to
    omit both buttons.
  - `frontend/src/components/templates/TemplateLibraryModal.tsx` —
    adds `onApplyV2WithOverrides` prop and the per-entry
    "Apply with overrides…" button (testid `library-apply-overrides`)
    that is rendered only for v2 entries whose `selectedSections ⊆
    { profile, stats }`. v1 entries never see the button; the
    existing "Apply" (fast path through
    `ApplyBuildTemplateV2FromLibraryToCharacter`) is unchanged.
  - `frontend/src/components/templates/TemplatesShellModal.tsx` —
    adds the shared `OverridesSource` discriminator, opens the
    overrides modal for both surfaces, and on confirm posts the
    mutated canonical JSON through
    `ApplyBuildTemplateV2ToCharacterJSON`. For the library path the
    shell first calls `PreviewBuildTemplateFromLibrary(entry.id)`
    (already shipped Phase 1) to obtain the canonical JSON, then
    routes it through the same overrides modal — no new binding is
    added.
- **Editable scope** — strictly the fields the Phase 5 backend
  writer already accepts:
  - `profile.name`, `profile.level`, `profile.runes`,
    `profile.soulMemory`, `profile.clearCount`,
    `profile.scadutreeBlessing`, `profile.shadowRealmBlessing`,
    `profile.talismanSlots`
  - `stats.vigor`, `stats.mind`, `stats.endurance`, `stats.strength`,
    `stats.dexterity`, `stats.intelligence`, `stats.faith`,
    `stats.arcane`
  - Per-field UI ranges mirror the backend schema validator:
    `level [1, 713]`, `clearCount [0, 7]`, `scadutreeBlessing
    [0, 20]`, `shadowRealmBlessing [0, 10]`, `talismanSlots [0, 3]`,
    stats `[1, 99]`. `runes` carries a soft warning above
    `999_000_000` but is not hard-capped — backend remains the source
    of truth.
- **`profile.class` stays read-only.** When the template carries
  `sections.profile.class`, the panel renders a read-only row with
  the class label and a "Skipped on apply (Phase 5)" hint instead of
  an editable input. The mutated JSON never writes `profile.class`,
  matching the Phase 5A apply-layer skip.
- **Selection semantics on toggle.** When a field is toggled off in
  the overrides UI, both `sections.profile/stats[field]` and the
  matching `selection.profile/stats[field]` are removed from the
  mutated JSON. When a field that was not in the original template
  is toggled on, the mutated JSON adds it to both `sections` and
  `selection`. This preserves the Phase 5 contract that "applied =
  selected ∧ present".
- **v1 templates and unsupported v2 sections remain blocked.** v1
  imports and v1 library entries never render the overrides button.
  v2 templates carrying `sections.equipment`, `sections.spells`,
  `sections.equippedTalismans`, `sections.appearance`, or
  `inventory.workspace` keep both v2 buttons disabled with the
  existing "profile / stats only in this phase" tooltip.
- **Frontend validation pre-checks ranges**; the Apply button in
  the overrides modal stays disabled while any field is out of
  range, with per-field inline error copy. When the backend
  nonetheless rejects (e.g. UTF-16 name overflow), the modal stays
  open and the rejection is surfaced through the existing error
  toast — the user can fix and retry without re-opening the
  preview.
- **Save to Library remains independent.** The new overrides flow
  does not touch the existing `Save to Library` button on the
  imported preview; clicking Save to Library persists the original
  canonical JSON, not the in-memory overrides draft. There is no
  "Save edited copy" affordance in this phase.
- **Tests** — frontend-only, +43 cases across the affected files:
  - `__tests__/ApplyOverridesPanel.test.tsx` (**new**, 19 cases) —
    rendering, mutation, range validation, soft cap, toggle-off
    removal, `profile.class` read-only, preservation of non
    profile/stats sections, invalid-JSON banner, modal apply/cancel
    behaviour.
  - `__tests__/ImportTemplatePreviewModal.test.tsx` — +7 cases
    covering the new button visibility / gating / forwarding for
    v2, and the v1-never-renders rule.
  - `__tests__/TemplateLibraryModal.test.tsx` — +5 cases covering
    `library-apply-overrides` visibility for v2 profile/stats,
    hiding for v1, hiding for unsupported v2, disabled without
    saveLoaded, and the forwarding of the entry to the parent.
  - `__tests__/TemplatesShellModal.test.tsx` — +12 cases covering
    the import flow with overrides (modal open → mutated JSON
    forwarded → success / applied=false / thrown error / cancel /
    invalid disables apply / `Save to Library` independence), and
    the library flow with overrides (`PreviewBuildTemplateFromLibrary`
    load → modal open → mutated JSON via
    `ApplyBuildTemplateV2ToCharacterJSON` (not FromLibrary) → fast
    path still uses `ApplyBuildTemplateV2FromLibraryToCharacter`
    untouched → empty canonical JSON surfaces an error toast →
    `profile.class` skip info toast).
- **Manual validation 2026-05-31** (user "manual OK"): on
  `feature/templates-v2-apply-overrides`, edited profile + stats
  values through both the direct YAML import path and the library
  "Apply with overrides…" path; mutated values landed on the
  selected character without touching unrelated fields; the fast
  library Apply path remained unchanged; v1 imports continued to
  show only the legacy `Save to Library` action with no overrides
  button; v2 imports carrying unsupported sections kept the buttons
  disabled with the supported-scope tooltip; cancelling the
  overrides modal discarded edits with no save mutation; `Save to
  Library` continued to persist the original canonical JSON,
  ignoring the in-modal edits.
- **Out of scope (still future work).** Weapon level override at
  apply time (Phase 6b / Phase 7 in spec/56 §17), new write paths
  for `sections.equipment`, `sections.equippedTalismans`,
  `sections.spells`, appearance via preset, URL import,
  multi-character pack, item quantities, inventory / storage /
  sort order / world progress edits at apply time. None of these
  ships in Phase 6.

### feat(templates): apply imported v2 YAML directly to character

Shipped Phase 5D.2 of the Templates v2 design (`spec/56-templates-v2.md`):
the `Import YAML → Preview → Apply` direct path that bypasses the
intermediate `Save to Library` step. Strictly scoped to schema v2
`sections.profile` + `sections.stats`. v1 imported templates do **not**
get a direct Apply control — they continue through the existing v1 path.

- **No backend, no bindings change.** This phase reuses the already
  shipped Phase 5D.1 endpoint
  `ApplyBuildTemplateV2ToCharacterJSON(charIdx, jsonText, opts)` and
  feeds it the canonical JSON already produced by
  `PreviewBuildTemplateImportYAMLFromFile` for the preview. No second
  file dialog. No TOCTOU re-read of the YAML on disk between Preview
  and Apply — the bytes the user previewed are the bytes that get
  applied. `ApplyBuildTemplateV2FromFileToCharacter` still exists
  backend/bindings-side but remains **unwired in UI**.
- **Frontend** — changed files:
  `frontend/src/components/templates/ImportTemplatePreviewModal.tsx`,
  `frontend/src/components/templates/TemplatesShellModal.tsx`,
  and the two `__tests__` siblings of each.
  - `ImportTemplatePreviewModal` gains an optional `onApplyV2` prop
    (separate from the existing v1 `onApply` placeholder), an
    `applyingV2` flag, and `charIndex` / `saveLoaded` gates. A new
    "Apply to character" footer button (testid
    `import-preview-apply-v2`) is rendered only for v2 documents; its
    enabled state is computed from the preview report, the active
    save, the active character index, the selected sections, and a
    module-level `V2_APPLY_SUPPORTED_SECTIONS = ['profile', 'stats']`
    that mirrors the backend scope guard. v1 imports never see the
    button.
  - `TemplatesShellModal` wires `handleApplyV2FromImportedPreview` —
    invoked from the imported-YAML preview only, never the
    library-preview instance. It calls
    `ApplyBuildTemplateV2ToCharacterJSON(charIndex,
    importedPreview.canonicalJSON, { mode: 'append' })`, closes the
    preview on success, raises a success toast that names the YAML
    path and the slot label, raises an info toast if `profile.class`
    appears in `result.skippedFields`, and on `applied === false` or
    a thrown error raises an error toast and keeps the preview open
    for the user to retry or close.
  - `App.tsx` is **untouched**. The existing
    `onCharacterTemplateApplied` callback already bumps
    `inventoryVersion`, `saveLoadKey`, and triggers `refreshSlots` +
    `refreshUndoDepth`, so the imported-YAML Apply path reuses the
    same post-apply refresh dance as the Phase 5D.1 library path.
  - **Save to Library remains independent** — the new direct Apply
    button is additive; the existing `Save to Library` action on the
    imported preview is unaffected, and the library Apply path
    shipped in Phase 5D.1 is unchanged.
- **UI gating** — `onApplyV2` is wired only for v2 imports; for v1
  imports the prop is omitted, so the new button is not rendered at
  all. For v2 imports the button is disabled (with a tooltip) when:
  the preview report is not OK, no save is loaded, no character is
  selected, no sections are selected, or the selection carries any
  section outside `V2_APPLY_SUPPORTED_SECTIONS`. Disabled-state
  copy explains exactly which gate failed.
- **Apply-time editing remains out of scope.** The direct Apply
  forwards exactly what the preview validated — no value overrides,
  no per-field re-selection at Apply time, no weapon level override
  (still Phase 6). Section scope at Apply equals the scope the user
  picked at Import.
- **Tests** — `ImportTemplatePreviewModal.test.tsx` gains 10 cases
  covering: v1 imports never render the v2 Apply button; v2 imports
  with all gates satisfied render it enabled; each individual gate
  failure (no save loaded, no character, empty sections,
  unsupported section, preview not OK, `applyingV2=true`) renders
  the correct disabled state and tooltip; clicking forwards to
  `onApplyV2`; "Save to Library" stays independent of the new
  button. `TemplatesShellModal.test.tsx` gains 8 cases covering: the
  imported-preview Apply path; binding called with the canonical
  JSON and `{ mode: 'append' }`; `applied=true` closes the preview,
  raises a success toast, fires `onCharacterTemplateApplied`;
  `profile.class` in `skippedFields` raises an info toast;
  `applied=false` raises an error toast and keeps the preview open;
  a thrown binding error behaves the same; v1 imported preview
  never shows the v2 Apply button; v2 imported preview without a
  loaded save keeps Apply disabled.
- **Manual validation 2026-05-31** (user "jest ok"): on
  `feature/templates-v2-direct-yaml-apply`, importing a v2 YAML
  with `selectedSections ⊆ { profile, stats }` through
  `Import YAML from File…` and clicking "Apply to character"
  applied the same fields as the Phase 5D.1 library path; the
  `profile.class` skip surfaced via info toast when `class` was
  selected; the preview closed; `App.tsx`'s refresh dance updated
  the visible character / save state; v1 imported YAMLs continued
  to show only the legacy `Save to Library` action with no v2
  Apply button; v2 imports carrying unsupported sections kept the
  button disabled with the supported-scope tooltip; the Phase 5D.1
  library Apply path remained unchanged.

### feat(templates): apply schema v2 profile + stats from library

Shipped Phase 5 of the Templates v2 design (`spec/56-templates-v2.md`): the
safe profile + stats subset of v2 Apply, reachable from the Templates global
shell against entries already stored in the local library. The v1 Apply path
is untouched; v2 templates that carry any section outside profile / stats
remain disabled at the UI level and refused by the backend guard.

- **Backend apply layer** (`app_templates_v2_apply.go`):
  - `ApplyBuildTemplateV2ToCharacterJSON(charIndex int, jsonText string, opts ApplyTemplateV2Options) (ApplyTemplateV2Result, error)`
    — applies only the supported v2 sections (`sections.profile`,
    `sections.stats`) to the selected `charIdx`. Runs under
    `slotMu[charIdx]` with a `core.SnapshotSlot` taken first and
    `core.RestoreSlot` on any error; recomputes `clearCount` flags
    and `ProfileSummary` side effects on success.
  - `ApplyBuildTemplateV2FromLibraryToCharacter(charIndex int, libraryEntryID string, opts ApplyTemplateV2Options) (ApplyTemplateV2Result, error)`
    — loads the library entry's JSON and delegates to the JSON endpoint.
  - `ApplyBuildTemplateV2FromFileToCharacter(charIndex int, path string, opts ApplyTemplateV2Options) (ApplyTemplateV2Result, error)`
    — reads a `.yaml` / `.json` file and delegates to the JSON
    endpoint. **Not wired into the UI in this release** — exists only
    for future direct-import flows.
  - `ApplyTemplateV2Options` carries the apply `Mode` (`"append"` for the
    current UI path) and the section selection mirror used by the
    backend guard.
  - `ApplyTemplateV2Result.Character` is typed as `vm.CharacterViewModel`
    (not `any`), so the frontend gets a strongly-typed payload after
    apply. The result also surfaces `Skipped[]` listing fields the
    apply layer intentionally did not write — `profile.class` always
    appears here because Phase 5 deliberately skips it.
  - `className` is **not** an alias of `class`; selecting `className`
    in a v2 selection block still fails validation upstream — only
    the canonical `class` key exists and it is intentionally skipped
    by the Phase 5 writer.
- **Wails bindings** (`frontend/wailsjs/go/main/App.{d.ts,js}` +
  `frontend/wailsjs/go/models.ts`) regenerated for
  `ApplyBuildTemplateV2ToCharacterJSON`,
  `ApplyBuildTemplateV2FromLibraryToCharacter`,
  `ApplyBuildTemplateV2FromFileToCharacter`,
  `ApplyTemplateV2Options`, and `ApplyTemplateV2Result` (with
  `character: vm.CharacterViewModel`).
- **Frontend (library-only Apply UI)** — changed files:
  `frontend/src/App.tsx`,
  `frontend/src/components/templates/TemplatesShellModal.tsx`,
  `frontend/src/components/templates/TemplateLibraryModal.tsx`.
  - The Apply button on a `TemplateLibraryModal` row is enabled for a
    v2 entry only when its `selectedSections ⊆ { profile, stats }`.
    Any other v2 section keeps the Apply button disabled with the
    existing "unsupported" tooltip; v1 entries remain handled by the
    v1 Apply path unchanged.
  - Clicking Apply runs an inline confirm directly in the library
    row (no separate dialog), then `TemplatesShellModal` calls
    `ApplyBuildTemplateV2FromLibraryToCharacter` with `mode: "append"`
    and the active `charIdx`.
  - After a successful apply, `App.tsx` bumps `inventoryVersion`,
    `saveLoadKey`, and triggers `refreshSlots` and
    `refreshUndoDepth`, so the visible character / save state
    updates without a reload.
  - The global shell still renders the v1 Apply control for v1
    library entries; it stays disabled in the global shell when
    there is no active `sessionID` (unchanged behaviour).
- **Supported flow** — `Import YAML → Save to Library → Apply from
  Library`. The direct "apply a freshly imported YAML without
  saving to library first" path is intentionally deferred; the
  backend endpoint (`ApplyBuildTemplateV2FromFileToCharacter`) and
  its binding exist, but no UI surface invokes them yet.
- **Apply for unsupported v2 sections remains blocked** — the
  existing schema guard still refuses any v2 template that carries
  `sections.equipment`, `sections.equippedTalismans`,
  `sections.spells`, `sections.appearance`, weapon-level overrides
  or multi-character packs. Out-of-scope sections are unchanged
  from spec/56 §17a.3.
- **Manual validation 2026-05-31** (Phase 5D.1 confirmation, user
  "jest ok"): on `feature/templates-v2-apply-profile-stats`, a v2
  library entry with profile + stats selection was applied to an
  active character; the inline confirm fires, the Apply succeeds,
  the selected fields visibly change, and the post-apply refresh
  reflects the new state. v1 entries remained disabled in the
  global shell (no `sessionID`); v2 entries carrying unsupported
  sections remained disabled. Direct imported-YAML Apply was not
  exercised and remains deferred.

### feat(templates): ship schema v2 library shell and YAML create/save/export/import flow

Shipped Phase 0..4 of the Templates v2 design (`spec/56-templates-v2.md`):
the additive `version: 2` schema, a global Templates library shell, the public
YAML sharing format for v1 payloads, and a create-from-character flow that
produces v2 templates carrying selected profile / stats fields. Apply for v2
templates outside the profile / stats subset remains blocked — Phase 5
profile / stats Apply from the library has shipped (see the entry above);
Phase 6+ (weapon level override, equipment / talismans / spells writers,
URL import, multi-character packs) remain design-only in spec/56.

- **Global Templates shell** (`frontend/src/App.tsx`, new
  `frontend/src/components/templates/TemplatesShellModal.tsx`) — blue
  `Templates` sidebar entry immediately above `Save as...`. Library /
  Create / Import are reachable from a single surface, decoupled from
  `SortOrderTab`. The existing `Export Template ▾` dropdown in
  `SortOrderTab` is intentionally retained as a power-user shortcut.
- **Public YAML import / export** for the v1 payload — new
  `backend/templates/yaml.go` (strict struct-typed `gopkg.in/yaml.v3`
  decode), `App.ExportBuildTemplateAsYAMLToFile`,
  `App.ExportLibraryTemplateAsYAMLToFile`, and file-import that accepts
  both `.yaml` and `.json`. The on-disk library stays JSON-internal;
  YAML imports transcode to JSON transparently when saved to the
  library.
- **Additive schema v2** in `backend/templates/schema.go`:
  - Schema key stays `saveforge.build-template` (no rename).
  - `MaxSchemaVersion = 2`; `SchemaVersion = 1` remains the v1 builder
    emission. v2 documents are produced only by the explicit v2 builder.
  - New top-level `Selection` object, required for `version: 2`.
  - New `sections.profile` (`name`, `level`, `runes`, `class`,
    `clearCount`, `scadutreeBlessing`, `shadowRealmBlessing`,
    `talismanSlots`) and `sections.stats` (`vigor` / `mind` /
    `endurance` / `strength` / `dexterity` / `intelligence` / `faith`
    / `arcane`). Canonical selection key for the class field is
    `class` (not `className`).
  - `validateBuildTemplateV2` enforces non-empty `Selection`, rejects
    unknown profile / stats sub-keys, and clamps player-field ranges.
  - v1 readers still reject v2 cleanly via the existing
    "unsupported version" path; v2 readers always accept v1 (semantic
    equivalence preserved when a v2 document carries only the
    workspace section).
- **Create-from-character v2 flow** —
  `App.BuildBuildTemplateV2FromCharacter`,
  `App.PreviewBuildTemplateV2FromCharacter`,
  `App.ExportBuildTemplateV2JSONFromCharacter`,
  `App.ExportBuildTemplateV2AsYAMLStringFromCharacter`,
  `App.SaveBuildTemplateV2FromCharacterToLibrary` in
  `app_templates_v2.go`, backed by `backend/templates/export_v2.go`
  (pure builder from `charIndex`). Frontend
  `CreateTemplateV2Modal.tsx` (wired into `TemplatesShellModal` and
  `App.tsx`) drives the per-section + per-field selection (profile and
  per-stat booleans), live preview, and Save to Library.
- **v2 metadata surfaced in the UI** — `TemplateLibraryModal` shows a
  `v2` badge and the list of selected sections / fields for v2
  entries; `ImportTemplatePreviewModal` renders the v2 profile / stat
  field summary in the preview panel.
- **Wails bindings regenerated** (`wails generate module`) so all v2
  endpoints and models are visible to the frontend.
- **Manual validation 2026-05-31**: `Templates → Create from
  Character… → profile/stats per-field selection → Preview schema v2
  → Save to Library → v2 badge / selected sections → Export YAML from
  library → Re-import the exported YAML` all work end-to-end on a
  real save. The Apply button for v2 templates remains disabled /
  absent by design.
- **Apply guard now scoped to unsupported v2 sections** — the
  Phase 3B.0 guard in `app_templates.go` still refuses v1 Apply
  for any document declaring `version: 2`. Phase 5 lifted the
  block specifically for profile / stats via the new v2 Apply
  layer (`ApplyBuildTemplateV2*ToCharacter`); v2 documents
  carrying any other section remain refused. v1 apply paths are
  untouched.
- **Out of scope** (still planned in spec/56 after Phase 5): weapon
  level override (Phase 6), equipment / equipped talismans / spell
  loadout writers (Phase 7a / 7b / 7c), appearance via preset
  (Phase 8), URL import with SSRF / redirect / IP guards (Phase 9),
  multi-character packs (Phase 10). Phase 5 profile / stats Apply
  from the library has shipped — see the entry above.

### fix(save-integrity): detect and repair duplicate inventory acquisition indices on load

Treated duplicate inventory acquisition indices as a save-integrity issue
instead of silently tolerating them. Older SaveForge revisions could
leave a slot with duplicated `InventoryItem.Index` values; whether the
game silently accepts such a save is unverified, so the editor now
blocks editing until the user explicitly repairs it.

- Added `GetSaveInventoryIntegrityReport()` (`app_save_integrity.go`),
  a read-only scan over every populated character slot
  (`slot.Version != 0`) that calls `core.ScanDuplicateInventoryIndices`
  on `Inventory.CommonItems` + `Inventory.KeyItems` and returns
  `SaveInventoryIntegrityReport{ Clean, Slots[] }`. Storage is
  intentionally out of scope at this stage.
- Per-slot DTO carries `SlotIndex`, `CharacterName`, `Active` (mirrored
  from `a.save.ActiveSlots[i]` so the UI can label phantom / residual
  slots), `DuplicateEntryCount` (additional occurrences beyond the
  first), `ConflictingIndexCount` (distinct duplicated Index values),
  and `Conflicts[]` grouped per acquisition Index. Each
  `InventoryIntegrityConflictItem` enriches the row through
  `slot.GaMap[handle]` first then `db.HandleToItemID` fallback, so
  weapons / armor / Ashes of War report the encoded `ItemID` (upgrade
  level + infusion) and not just the base ID. Unknown items keep
  `ItemID` + `Handle` for hex fallback (`Unknown=true`) instead of
  being dropped.
- Added `CloseSave()` (`app_save_close.go`) — an idempotent drop of the
  active save that mirrors `installLoadedSave`'s reset surface
  (`a.save = nil`, `lastSavePath`, `favSlotNames`, undo stacks, edit
  sessions) under exclusive `a.saveMu`. Deliberately does NOT touch
  `a.sourceSave` (independent read-only handle owned by Character
  Importer).
- `AddItemsToCharacter` is now fail-closed: when
  `core.ScanDuplicateInventoryIndices(slot)` reports any duplicate
  before mutation, the endpoint returns
  `"inventory integrity issue: slot N contains X duplicate acquisition
  entries; repair is required before adding items"` without pushing
  undo, snapshotting the slot or adding any items. Post-mutation
  validation switched to `core.ValidatePostMutation(slot)` (no
  baseline). Removed the legacy
  `"tolerating (game accepts them)"` warning and the
  `dupBaseline` map entirely.
- Removed `core.ValidatePostMutationBaseline` — only caller delegated
  to it with `nil`, the "tolerate pre-existing duplicates" use case
  no longer exists, and the comment falsely cited
  `spec/52` as justification for legal duplicates (spec/52 actually
  describes stride-2 as a *uniqueness-guaranteeing* write model).
- Wired the load-time gate end-to-end in the UI: new
  `InventoryIntegrityModal` (`frontend/src/components/integrity/`) is a
  blocking modal shown by `App.tsx::finalizeLoadedSaveWithIntegrityCheck`
  after every successful main-save load — both `SelectAndOpenSave` and
  the deploy paths (`DownloadRemoteSave`, `CloseAndDownload`) — via the
  new `SettingsTab` `onAfterLoad` callback. The modal lists each
  affected slot (active character name OR
  `Inactive residual slot N`), the duplicate / conflicting counters
  and an opt-in `Show affected items` panel that groups items per
  acquisition Index with weapon upgrade / infusion / hex-ID fallback.
- `Repair duplicates` button in the modal calls
  `RepairDuplicateInventoryIndices(slotIndex)` for every affected
  slot, re-scans through the same endpoint, unblocks the editor only
  when the re-scan returns `Clean=true` and shows a factual error
  otherwise. `Close save` calls the new `CloseSave()` endpoint and
  resets the editor to the no-save state. No automatic write — the
  user keeps a backup and saves the repaired file manually.
- Removed the now-unreachable `frontend/src/components/database/`
  `RepairPrompt.tsx` together with its `DatabaseTab` plumbing
  (`isDuplicateInventoryIndexError`, `repairPrompt` state,
  `handleRepairAndRetry`, `handleRepairCancel`, render block) and the
  two `DatabaseTab.test.tsx` cases that drove the legacy retry flow
  through a mocked `AddItemsToCharacter` rejection. The backend
  `RepairDuplicateInventoryIndices` endpoint is unchanged — it now
  feeds the new load-time modal instead.
- Backend tests added or strengthened in `app_save_integrity_test.go`,
  `app_save_close_test.go` and `app_additems_duplicate_index_test.go`
  (cross-scope conflicts, empty-handle filtering, inactive residual
  slots, GaMap-aware weapon upgrade / infusion, KeyItems pre-flight
  rejection, CloseSave idempotency / undo / sessions / source-save
  isolation). Frontend `InventoryIntegrityModal.test.tsx` covers
  rendering, counters, residual labelling, affected-items grouping,
  unknown fallback, weapon enrichment, button callbacks and busy
  state.
- Out of scope for this fix (called out explicitly so a follow-up can
  pick them up): storage duplicate scan / repair, source-save
  integrity check for Character Importer, an Open read-only mode for
  the integrity modal.

### fix(save-state): serialize concurrent save and slot access

Closed the remaining whole-save / per-slot / favorites / source-save /
deploy concurrency holes that the previous inventory-session fix left
open. Wails dispatches every bound endpoint in its own goroutine, so
without these locks a `SelectAndOpenSave` could swap `a.save` under a
non-session writer mid-mutation, two non-session writers / readers
could race the same character's `GaMap` and inventory bytes, the
favorites endpoints could panic with `concurrent map writes` on
`favSlotNames`, the diff loop could dereference an `a.sourceSave`
that had just been replaced, and the deploy temp-file MD5 could be
computed over `UserData10` bytes that a parallel favorites writer
was zeroing out.

- Added four App-level mutexes with a strict global lock order
  `saveMu → lifecycleMu → editSessionsMu → sess.mu → favMu →
  sourceSaveMu → slotMu[i]` (slotMu acquired ascending, released
  descending). Lifetimes and helper contracts documented inline on
  `App` in `app.go`:
  - `saveMu sync.RWMutex` — guards the `a.save` pointer plus the
    whole-save metadata mutated outside a single slot (`UserData11`,
    `SteamID`, `Platform`, `IV`, `Encrypted`, `lastSavePath`).
  - `slotMu [10]sync.Mutex` — guards per-character data
    (`Slots[i]`, `ActiveSlots[i]`, `ProfileSummaries[i]`,
    `undoStacks[i]`); always held in addition to `saveMu.RLock`,
    never alone. Multi-slot operations use `lockAllSlots` /
    `unlockAllSlots` / `lockSlotPair` helpers so the ascending /
    descending invariant holds across every caller
    (`CleanResidualSlots`, `GetActiveSlots`, `GetCharacterNames`,
    `AuditLoadedSaveIssues`, `GetSaveDiffSummary`, `writeTempSave`,
    `CloneSlot`).
  - `favMu sync.RWMutex` — guards the favorites blob in
    `a.save.UserData10` plus the `a.favSlotNames` map; closes the
    `concurrent map writes` panic on favSlotNames between
    `RemoveFavoritePreset` and `WriteSelectedToFavorites`.
  - `sourceSaveMu sync.RWMutex` — guards the `a.sourceSave`
    pointer plus every dereference by the diff / source-active-slots
    endpoints.
- Extracted `installLoadedSave` / `installSourceSave` so the file
  dialog, download, decrypt and parse phases run lock-free; the
  helpers run only the atomic commit (pointer swap + derived-state
  reset of `lastSavePath`, `favSlotNames`, undo, edit sessions) under
  exclusive `saveMu.Lock` / `sourceSaveMu.Lock`. Both helpers carry
  caller-contract docs forbidding lock re-acquisition inside.
- Refactored every public endpoint that previously inlined a
  multi-step body into a thin wrapper that takes the appropriate
  locks and delegates to a `…Locked` internal worker: `getItemListLocked`,
  `getInventoryOrderLocked`, `reorderInventoryLocked`,
  `applyCharacterPresetLocked`, `bulkSetCookbooksUnlockedLocked`,
  `bulkSetBellBearingsLocked`, `setNetworkParamsLocked`,
  `getSlotDiffLocked`, `sourceCharacterName`. Sibling public endpoints
  (e.g. `ApplyBuiltinCharacterPresetStats`, `SetCookbookUnlocked`,
  `GetWeaponInventoryOrder`, `ResetNetworkParams`, `GetItemListChunk`,
  `GetSaveDiffSummary`) call the locked worker directly so the same
  goroutine never re-enters the public endpoint and double-acquires a
  non-reentrant mutex (Go `sync.Mutex` / `sync.RWMutex.RLock` are not
  reentrant).
- `SaveInventoryWorkspaceChanges` now takes
  `saveMu.RLock + sess.mu + slotMu[charIdx]` across the entire apply
  → rebuild snapshot → regenerate baseline pass, so non-session
  mutators / readers of the same slot (e.g. `AddItemsToCharacter`,
  `BulkSetCookbooksUnlocked`, `GetInventoryOrder`) cannot race the
  session's `GaMap` / inventory rewrite. The original per-session lock
  remains; this just extends the protection to the cross-mutator case
  the per-session lock could not cover.
- `WriteSave` no longer holds `saveMu` across the file dialog /
  backup I/O. It snapshots `expected := a.save` under a brief
  `saveMu.RLock`, releases, then runs dialog + backup unlocked. The
  new internal `writeSaveCore(path, platform, expected)` re-acquires
  `saveMu.Lock` and verifies `a.save == expected`; on mismatch it
  returns an `active save changed while the save dialog was open;
  write to %q aborted to avoid overwriting it with the wrong save`
  error WITHOUT mutating `Platform` / `IV` / `Encrypted`, WITHOUT
  calling `SaveFile`, and WITHOUT touching undo or `lastSavePath`.
  This blocks the race where a concurrent `SelectAndOpenSave` /
  `DownloadRemoteSave` would otherwise cause save B to be serialised
  under the path picked for save A.
- `writeTempSave` (used by `DeploySave` / `DeployAndLaunch`) takes
  `saveMu.RLock → favMu.RLock → lockAllSlots` for the full
  `a.save.SaveFile(tmpPath)` pass. The favorites region of
  `UserData10` (`0x154..0x1324`, 15 preset slots) is read into the
  PC MD5 / PS4 WriteBytes pass; without `favMu.RLock` a parallel
  `RemoveFavoritePreset` / `WriteSelectedToFavorites` (both run under
  `saveMu.RLock + favMu.Lock`, neither takes `slotMu`) could mutate
  the preset bytes mid-serialisation and produce a deployed `.sl2`
  whose `UserData10` bytes disagreed with the embedded PC MD5.
  `flushMetadata` writes only into SteamID `[0x00..0x08)` and
  ActiveSlots / ProfileSummaries `[0x1954..0x3CDE)`, which is
  disjoint from the favorites region, so `favMu.RLock` (shared,
  not exclusive) is the minimal lock required: `writeTempSave` is a
  reader of favorites bytes and a writer of metadata bytes whose
  ranges no favorites endpoint touches.
- Tests (Go): added four concurrency test files in `package main`
  with 15 scenarios:
  - `app_slot_lock_concurrency_test.go` — slotMu coverage:
    AddItemsToCharacter, BulkSetCookbooksUnlocked, GetInventoryOrder,
    StartInventoryEditSession, SaveInventoryWorkspaceChanges (the
    mandatory cross-mutator gate), two-mutators-same-slot race
    freedom, different-character parallelism, and
    GetActiveSlots-blocks-on-any-slotMu via `lockAllSlots`.
  - `app_save_lifecycle_concurrency_test.go` — installLoadedSave
    drains active `saveMu.RLock` readers before swapping the pointer;
    `writeSaveCore` aborts cleanly when `a.save` changed during a
    simulated dialog and leaves the file system, save metadata and
    undo stack untouched (uses `t.TempDir()`).
  - `app_favorites_concurrency_test.go` — RemoveFavoritePreset
    blocks on `favMu.Lock`; parallel Remove / Write / Read endpoints
    are race-free under the detector; the mandatory deploy gate uses
    a `slotMu[0]` park + `favMu.TryLock` probe to deterministically
    prove `writeTempSave` has acquired `favMu.RLock` before a
    favorites writer fires, then asserts the writer is blocked until
    `writeTempSave` releases.
  - `app_source_save_concurrency_test.go` — GetSlotDiff blocks on
    `sourceSaveMu`; installSourceSave drains active
    `sourceSaveMu.RLock` readers before swapping the pointer.
- All 15 new tests pass under
  `go test -race -count=10 -run 'TestSlot_|TestSaveLifecycle_|TestWriteSave_|TestFavorites_|TestDeploy_|TestSourceSave_' .`
  (≈69 s). The existing nine `TestInventorySession_` scenarios remain
  green under `-race -count=10` (≈31 s). The full canonical suite
  (`go test . ./backend/... ./tests/...`, `TestRoundTripPC`, `go vet`,
  `go build`, `npx tsc --noEmit`, the 153-test Vitest suite) is green.
- No behavioural change to the duplicate-acquisition-index WARN flow,
  the DatabaseTab RepairPrompt UI, the templateLibrary lazy init,
  the frontend `StartInventoryEditSession` guard or any public
  Wails endpoint signature — those were explicitly out of scope.

### fix(inventory-session): serialize concurrent workspace session access

Fixed the backend crash `fatal error: concurrent map writes` originating in
`StartInventoryEditSession` (`app_inventory_session.go`). Wails dispatches every
bound endpoint in its own goroutine, so two concurrent calls hitting the
unsynchronised `editSessions` / `editSessionByChar` maps on `App` — easily
reproduced by React 18 StrictMode / HMR double-firing the SortOrderTab effect —
crashed the process unrecoverably.

- Added a registry-scoped `editSessionsMu sync.Mutex` on `App` that guards every
  lifecycle touch of the two maps (lookup, insert, replace, delete, full clear)
  and nothing else, so two different characters' sessions can still run in
  parallel.
- Added a per-session lock on `editor.InventoryEditSession` exposed via
  `Acquire` / `Unlock` / `Close` / `IsClosed` plus `editor.ErrSessionClosed`.
  Every endpoint that touches `Workspace`, `BaselineEditableHandles` or
  `BaseRevision` (Get, Validate, Move, Transfer, Add, Update, Remove, Save,
  template Export / Apply) acquires the lock first and unlocks on return. The
  multi-step `SaveInventoryWorkspaceChanges` flow now holds the lock across
  apply → rebuild snapshot → regenerate baseline, so no peer can observe a
  half-replaced Workspace or a baseline map in the middle of initialisation.
- Added per-character `lifecycleMu [10]sync.Mutex` on `App` that serialises
  every session lifecycle transition for a given slot (`Start` replacement,
  `Discard`, `clearAllEditSessions`). The new lock closes the cross-session
  slot race that the per-session mutex alone could not cover: a replacement
  `StartInventoryEditSession(charIdx)` now evicts the prior session under
  the registry lock, then drains it via `closeSession` BEFORE calling
  `editor.StartSession` on `a.save.Slots[charIdx]`, so the new snapshot is
  built off a quiesced slot — never alongside a prior session's in-flight
  `SaveInventoryWorkspaceChanges` mutating the same slot. `Discard` and
  `clearAllEditSessions` take the same lifecycle lock(s) before draining,
  so a new `Start` for the same character blocks until the orphaned Save
  has released its per-session lock. Lock order is strict and global:
  `lifecycleMu[charIdx]` → `editSessionsMu` → `sess.Acquire()`.
  `clearAllEditSessions` acquires all 10 lifecycle locks in ascending order
  and releases them in descending order so no reverse-cycle deadlock is
  reachable from concurrent `Start` / `Discard`.
- `StartInventoryEditSession` preserves its replacement / refresh semantics
  (the call always returns a fresh post-event snapshot, never a stale dirty
  workspace) but performs the registry eviction, prior-session drain, and
  publication as a single lifecycle-locked sequence.
- `DiscardInventoryEditSession` and `clearAllEditSessions` evict under the
  registry lock and then Close each affected session — Acquire blocks until any
  in-flight mutator finishes, so once the call returns no orphan goroutine is
  still writing. A subsequent endpoint call against the closed session
  deterministically returns the existing `inventory edit session %q not found`
  wire error, matching the regex the frontend's `useInventoryWorkspace`
  self-heal path already keys on.
- Tests (Go): added `app_inventory_session_concurrency_test.go` with nine
  scenarios covering concurrent same-char Start (the original crash signature),
  concurrent mutations, Save vs mutation, Discard vs mutation, clearAll vs
  mutation, the Close/Acquire contract, and three lifecycle scenarios that
  park a fake Save by holding the per-session lock directly and assert that
  a replacement Start / a Discard+Start / a clearAll+Start cannot complete
  until the simulated Save releases. All pass under
  `go test -race -run TestInventorySession_ -count=10 .`. Existing sequential
  session tests, the full `go test . ./backend/... ./tests/...` suite (incl.
  `TestRoundTripPC`), `go vet`, `npx tsc --noEmit` and the full Vitest suite
  (153 tests) remain green.
- No behavioural change to the duplicate-acquisition-index WARN flow in
  `AddItemsToCharacter`, `RepairDuplicateInventoryIndices`, the DatabaseTab
  `RepairPrompt` UI or its tests — those live in a different subsystem and
  were explicitly out of scope.

### chore(cleanup): remove dead AoW-flags trio and orphaned backing

Removed the unreachable public `AoW-flags trio` App/Wails endpoints
(`GetAshOfWarFlags`, `SetAshOfWarFlagUnlocked`, `BulkSetAshOfWarFlags` in
`app_world.go`) together with their now-orphaned backing: the
`db.AshOfWarFlagEntry` type, `db.GetAllAshOfWarFlags` / `getAllAshOfWarFlags`,
the `data.AshOfWarFlags` duplication-flag table (65810–65934) with its
`AshOfWarFlagData` type, and the `db.AshOfWarFlagEntry` reference in
`_forceExportTypes`. Two read-only audits confirmed the trio had no backend or
frontend callers, no tests, and was unreachable from the active UI; its backing
was consumed only by the trio itself plus the type-export shim. Wails bindings
were regenerated (`wails generate module`), dropping the three exports and the
`AshOfWarFlagEntry` TypeScript model. The active, independent whetblade/affinity
flow is untouched: `SetWhetbladeUnlocked`, `WhetbladeRelatedFlags`,
`AoWMenuUnlockedFlag` (65800) and the Storm Stomp duplication flag (65841) stay
in place — 65841 keeps its own constant `data.StormStompDupFlag` in
`whetblades.go`, independent of the removed table. The active
`data.AoWItemToFlagID` map (batch-add Lost-Ash-of-War flow) was preserved in
`ash_of_war_flags.go`. Docs updated: removed the trio listing from
`spec/16-world-state.md`, the `SetAshOfWarFlagUnlocked` row from
`spec/15-event-flags.md`, and the unimplemented `AshOfWarFlags` field from the
`CharacterPresetWorld` design struct in `spec/37-character-presets.md` (the
real `WorldPresetData` never had it) — all with PL parity.

### chore(icons): remove redundant shackle orphan duplicates

Removed 2 redundant, unreferenced Shackle PNG copies from
`frontend/public/items/key_items/` (Margit's Shackle and Mohg's Shackle). The
active canonical assets with byte-identical content remain in the semantically
correct `frontend/public/items/tools/` directory; the active `Margit's Shackle`
and `Mohg's Shackle` records belong to the `tools` category and use `IconPath`
values under `items/tools/`. The deleted files were not referenced by any active
`IconPath` nor by any tracked source, config, docs or tests. The change removes
44 947 B of redundant files; `IconPath`, resolved targets and missing targets
are unchanged. The remaining orphan duplicates, the two Class B cases and other
deferred topics stay out of scope.

### chore(icons): remove redundant orphan painting duplicates

Removed 7 redundant, unreferenced painting PNG copies from
`frontend/public/items/tools/` (Champion's Song, Flightless Bird, Homing
Instinct, Prophecy, Redmane, Resurrection and Sorcerer). The active canonical
assets with byte-identical content remain in the semantically correct
`frontend/public/items/info/` directory. The deleted files were not referenced
by any active `IconPath` nor by any tracked source, config, docs or tests. The
change reduces physical orphan assets by 7 and removes 234 206 B of redundant
files; `IconPath`, resolved targets and missing targets are unchanged. The two
different-basename cases and other orphan groups stay out of scope.

### fix(build): rename Jolán asset for go embed compatibility

Renamed the physical asset `ashes/jolán_and_anna.png` to the embed-safe ASCII
path `ashes/jolan_and_anna.png` and updated the 11 matching `IconPath` values
for `Jolán and Anna` and its upgrade levels (base plus +1…+10). The displayed
item names `Jolán and Anna` keep their accent. The accented file name broke the
embedded frontend assets (`//go:embed frontend/dist`) and prevented the
application from building; `make build` passes after the rename. The PNG image
and the item semantics are unchanged.

### perf(icons): downscale oversized tools icons

Downscaled 31 actively used `tools` PNG icons from `1024×1024` to `256×256`,
covering maps, cookbooks, prattling pates, notes/letter/scroll, keys and the
Deflecting Hardtear. The shared cookbook icons represent numbered volumes and
were downscaled as single physical assets. File names, PNG format, transparency
and the existing `IconPath` values are unchanged. The batch size dropped from
`29 260 094 B` to `2 242 618 B`, a saving of `27 017 476 B` (`−92.34%`). Icon
quality was verified visually from the image diff. The palette-mode
`oil_soaked_tear.png` and other open icon topics stay out of scope.

### perf(icons): downscale oversized non-tools item icons

Downscaled 14 actively used PNG icons from `1024×1024` to `256×256`, covering
`arrows_and_bolts`, `ashes`, `talismans`, `bolstering_materials`, `ashes_of_war`,
`shields` and `melee_armaments`. Two `ashes` icons are each shared by 11
spirit-ash upgrade records (base plus +1…+10) and were downscaled as single
physical assets. File names, PNG format, transparency and the existing
`IconPath` values are unchanged. The batch size dropped from `13 266 510 B` to
`1 108 915 B`, a saving of `12 157 595 B` (`−91.64%`). Icon quality was verified
visually from the image diff. The `tools` category and the palette-mode icons
stay out of scope.

### perf(icons): downscale oversized crafting and info icons

Downscaled 13 actively used PNG icons from `1024×1024` to `256×256`, covering 7
`crafting_materials` and 6 `info` icons. File names, PNG format, transparency and
the existing `IconPath` values are unchanged. The batch size dropped from
`11 935 360 B` to `986 774 B`, a saving of `10 948 586 B` (`−91.73%`). Icon
quality was verified visually from the image diff. The two large palette-mode
icons in `crafting_materials` and the remaining large-icon categories stay out
of scope.

### perf(icons): downscale oversized spell icons

Downscaled 13 actively used spell PNG icons from `1024×1024` to `256×256`,
covering 9 `sorceries` and 4 `incantations` icons. File names, PNG format,
transparency and the existing `IconPath` values are unchanged. The batch size
dropped from `15 848 791 B` to `1 391 884 B`, a saving of `14 456 907 B`
(`−91.22%`). Quality was verified manually for a representative sample in the
database grid and the large detail panel. The remaining large-icon categories
and the three palette-mode icons stay out of scope.

### perf(icons): downscale oversized key item icons

Downscaled 25 actively used `key_items` PNG icons from `1024×1024` to `256×256`,
covering crystal tears, cracked/knot tears, keys and whetblades (plus Heart of
Bayle and Messmer's Kindling). File names, PNG format, transparency and the
existing `IconPath` values are unchanged. The batch size dropped from
`21 742 723 B` to `1 942 948 B`, a saving of `19 799 775 B` (`−91.1%`). Quality
was verified manually for a representative sample in the database grid and the
large detail panel. The remaining large-icon categories and the three
palette-mode icons stay out of scope.

### perf(icons): downscale four oversized active item icons

Downscaled four actively used PNG icons from `1024×1024` to `256×256` as a pilot
for trimming oversized assets. The set covers representative icons across
`incantation`, `info`, `key_items` and `tools` (Aspects of the Crucible: Thorns,
About the Scadutree Blessing, Crimsonspill Crystal Tear and the Battlefield
Priest's Cookbook shared icon). File names, PNG format, transparency and the
existing `IconPath` values are unchanged. The combined size of these four icons
dropped from `6 927 527 B` to `532 764 B` (`−92.3%`). Quality was verified
manually in the database grid and the large detail panel. This pilot does not
cover the remaining large icons or the palette-mode icons.

### fix(icons): relocate misplaced Flask of Wondrous Physick info icon

Moved the `About Flask of Wondrous Physick` tutorial icon from the wrong
`items/tools/` directory to `items/info/`, the location its existing runtime
`IconPath` already points to. The file was a leftover from the old
`tools/sacred_flasks/` directory flatten while its sibling `about_*` tutorial
icons were grouped under `items/info/`. No `IconPath` was changed and the image
bytes were not modified. Remaining unresolved icons, including Beast Claw, stay
out of scope.

### fix(icons): relocate eight misplaced item icon assets

Relocated eight confirmed item icon assets into the directories already required
by their existing runtime `IconPath`, fixing the icons for two info notes
(About Sorceries and Incantations, About Teardrop Scarabs), Sellia's Secret,
three DLC melee armaments (Fire Knight's Greatsword, Fire Knight's Shortsword,
Lightning Perfume Bottle), Black Syrup and Oil-Soaked Tear. The files were
misplaced under `items/key_items/`; each was moved to the folder its `IconPath`
points to (`items/info/`, `items/tools/` or `items/melee_armaments/`). No
`IconPath` mappings and no image contents were changed. This does not cover the
still-ambiguous About Flask of Wondrous Physick or the remaining missing icons.

### fix(character-importer): restore source-slot avatar icon

Fixed the broken hardcoded path of the decorative Knight Helm icon shown for
the source-slot selector in Character Importer (`items/armor/` → `items/head/`).
The slots no longer render a broken-image glyph from the non-existent
`items/armor/` directory. No change to import logic or save data.

### chore(weapon-edit): remove legacy weapon-apply endpoints

Removed the public direct-write endpoints `ApplyWeaponUpgradeLevel`,
`ApplyWeaponInfusion`, `ApplyWeaponAoW` and `ApplyWeaponAoWStrict` left over from
the former legacy weapon editor flow, together with their generated bindings and
dead frontend mocks. The active weapon-editing flow remains workspace-based and
persists through the workspace save path (`editor.ApplyWorkspaceSave`). The core
writers (`PatchWeaponItemID`, `PatchWeaponAoW`, `PatchWeaponAoWHandle`) stay in
place because the active save still uses them; their stale "legacy tab only"
comment was corrected. Writer and DLC-compatibility regressions are covered by
`backend/core` and `backend/db` tests respectively. No reachable UI behavior
changed.

### chore(weapon-edit): remove unreachable non-workspace mode

Removed the unreachable legacy/non-workspace mode from `WeaponEditModal`. The
active modal is driven exclusively by the workspace flow from `SortOrderTab`,
so the `workspace`/`workspaceItem` props are now required and the dead
`GetCharacter` fallback, the legacy `ApplyWeaponUpgradeLevel` /
`ApplyWeaponInfusion` / `ApplyWeaponAoWStrict` apply paths, and the unused
`onApplied` callback were dropped. Upgrade, infusion, Ash of War and
compatibility behavior for the reachable UI is unchanged.

### chore(database): extract repair prompt

Moved the active duplicate inventory/acquisition-index repair prompt out of
`DatabaseTab` into a dedicated presentational component
(`components/database/RepairPrompt.tsx`). The `repairPrompt` state, the
`RepairDuplicateInventoryIndices` call, the retry of the original add, and the
decision to keep the confirm modal mounted during the prompt all remain in
`DatabaseTab`. User-visible behavior is unchanged.

### chore(database): remove unused error modal items list

Removed the never-used `items?` field and its conditional list JSX from
`ErrorModal` and the `errorModal` state in `DatabaseTab`. No `setErrorModal(...)`
caller (capacity-exceeded, add-failure, repair-failure) ever populated it, so
the list could never render. The error modal's reachable behavior (title,
message, OK) is unchanged.

### chore(database): extract error modal

Moved the active error modal out of `DatabaseTab` into a dedicated
presentational component (`components/database/ErrorModal.tsx`). The capacity-
exceeded, add-failure, and repair-failure paths along with the `errorModal`
state remain managed by `DatabaseTab`. User-visible behavior is unchanged.

### chore(database): extract ban-risk warning modal

Moved the active ban-risk warning modal out of `DatabaseTab` into a dedicated
presentational component (`components/database/BanRiskWarningModal.tsx`). The
modal is now fully prop-driven; the gating state (`banRiskWarning`,
`ignoreBanRisk`), the confirm flow, and the `AddItemsToCharacter` mutation all
remain in `DatabaseTab`. User-visible behavior (Cancel, Add Anyway, the ignore
checkbox) is unchanged.

### chore(database): remove unreachable legacy icon preview modal

Removed the dead, unreachable icon preview modal from `DatabaseTab` along with
its `selectedIcon` state. The modal could never open — `selectedIcon` was only
ever set to `null` (close); no user action set it to a value, and clicking an
item icon opens the active `ItemDetailPanel` instead. The active item detail
panel, the missing-icon handling (`brokenIcons`), the hover preview tooltip, and
the add-item flow are unchanged. No backend, endpoints, or Wails bindings touched.

### chore(cleanup): remove unreachable legacy Weapon Edit Tab

Deleted `frontend/src/components/WeaponEditTab.tsx` and its dead render path. The
Inventory → Weapon Edit pill had already been hidden, leaving the component
unreachable (no setter drove `invView` to `'weapon_edit'`). Weapon editing is now
served exclusively by `WeaponEditModal` (launched from the Weapons & Sort Order
tab), which remains untouched and consumes a superset of the same backend
endpoints. No backend endpoints or Wails bindings were removed.

- `App.tsx`: dropped the `WeaponEditTab` import, the `'weapon_edit'` `invView`
  variant, the dead render branch, and the now-redundant `weapon_edit` guards.

### fix(inventory): self-heal lost edit session so weapon repair always works

Repairing an out-of-range weapon could fail with `inventory edit session "…" not
found`: the frontend held a session id the backend had dropped (sessions were not
cleared when a new save loaded, and a tab remount / reload could evict them). The
B-fix above made the repair button reachable, which exposed this.

- `useInventoryWorkspace.ts`: added `runSessionOp` — session-scoped calls now
  self-heal: on a "session not found" failure they transparently restart the
  session for the current character and retry once. Wired into `updateWeapon` and
  `save`. Validation errors (e.g. out-of-range upgrade) are NOT treated as session
  loss, so they still surface normally.
- `app.go` / `app_deploy.go`: loading a save now calls `clearAllEditSessions()` so
  stale sessions referencing the previous save's slots cannot linger.

### fix(inventory): never write out-of-range weapon upgrades + surface them before save

Root-caused the "Golem Greatbow +25" / `upgrade_out_of_range` report and the silent
save that wouldn't persist a fix. No auto-repair / background magic — the app just
stops creating the corruption and tells the user where to fix what already exists.

- **A — add path can no longer create out-of-range levels** (`app.go`): added
  `clampUpgrade(requested, max)` and applied it in `AddItemsToCharacter` so the
  encoded item ID can never carry a level above the item's real `MaxUpgrade`. A
  somber +10 weapon (Golem Greatbow) requested at +25 is now stored at +10, never
  `base+25` (the invalid encoding the editor rejects). This was the original source
  of the corruption (an add made when the weapon was mis-capped at 25).
- **B — weapon editor tells the truth and lets you repair manually**
  (`WeaponEditModal.tsx`): for an item whose stored level exceeds its max, the level
  dropdown is now seeded with a valid level (so it is not blank/0 with a disabled
  Apply), the real out-of-range level is shown in red, and a note explains how to fix
  it. Picking a valid level + Apply re-encodes the item to a valid ID.
- **C — no more silent save**: new read-only `AuditLoadedSaveIssues` App endpoint
  scans every slot (`editor.BuildSnapshot`+`Validate`) and returns blocking issues
  annotated with the tab to fix them. `App.tsx` "Save As" now runs the audit first;
  if issues exist it shows a modal listing them (grouped, with count + fix location)
  and lets the user **Save Anyway** or **Cancel & Fix**. Saving is allowed, never
  blocked — the user is simply informed instead of losing edits silently.
- Tests: `app_weapon_upgrade_clamp_test.go` — `clampUpgrade` unit cases + integration
  (adding Golem Greatbow at +25 stores +10, never `base+25`). Full Go suite + vet,
  `tsc`, 147 vitest, and frontend build pass.
- Out of scope (unchanged): no auto-repair on load, MatchmakingRegions, NetworkParam,
  SummoningPools, Colosseums.

### fix(slots): positional in-place character delete + phantom-slot cleanup

The app showed duplicate / undeletable characters because it used a different
source of truth for slot occupancy than the game. The game reads the per-slot
active flag (`0x1954`); the app derived occupancy from the slot-data
`CharacterName` and ignored the flag. A character deleted in-game (flag cleared,
data block + ProfileSummary never zeroed) therefore appeared in the app as a
phantom duplicate the user could not remove.

- **Single source of truth = active flag**: `GetActiveSlots`/`GetCharacterNames`
  now report occupancy from `ActiveSlots[]` (matching the game's character-select),
  so a phantom slot shows as empty and the app roster matches the console.
- **`DeleteSlot` is now clear-in-place, NOT shift-down**: confirmed against the
  save format (independent per-slot active flags — gaps between active slots are
  valid; the user's save had an inactive slot 1 between active slots 0 and 2) and
  the reference ER-Save-Editor (purely positional). New `core.SaveSlot` helpers
  `ClearSlot` (zeroes data block + flag + the FULL `0x24C` ProfileSummary region,
  including the opaque face/equipment snapshot `Serialize` never rewrites),
  `SlotHasResidualData`, `CleanResidualSlots`, `ClearProfileSummaryRegion`.
- **Old shift-down delete was the corruption source**: it moved parsed name+level
  but never shifted the opaque summary bytes, desyncing summary vs slot-data, and
  renumbered all subsequent characters needlessly.
- `app.go`: new `CleanResidualSlots()` to clear phantom slots; `frontend/App.tsx`
  `handleDelete` updated for stable indices (no shift). `flushMetadata` magic
  numbers replaced with named constants (`ActiveSlotsOffset`, `ProfileSummaryOffset`,
  `ProfileSummaryStride`).
- Tests: new `slots_test.go` (in-place clear leaves neighbours untouched, residual
  detection, idempotent cleanup). Full PC/PS4 roundtrip + `tsc --noEmit` + `make build` pass.
- **PS4-verified**: cleaned save (cleared phantom slot 1, preserved Niziol slot 0 +
  Bydlaczka lvl 150 slot 2) loads correctly on console; in-app delete works in-place.
- **Delete confirmation fix**: the delete button was gated behind native
  `window.confirm()`, a no-op in the Wails WKWebView (always returns false) — so
  deletion never ran. Replaced with a custom confirmation modal (matching the clone
  modal). (`frontend/src/components/PresetsTab.tsx` has the same `window.confirm`
  pattern — out of scope here.)

### fix(save): stop corrupting NextEquipIndex (CE-108255-1) + storage topup wrote wrong record

Two independent save-write defects, both confirmed fixed in-app and on PS4 console.

- **Crash (CE-108255-1)**: every load/add/transfer/repair path forced
  `NextEquipIndex = NextAcquisitionSortId` and wrote it back to `slot.Data`, even on a
  plain load+save. `NextEquipIndex` is a SEPARATE equip-list counter, NOT a visibility
  gate — genuine console saves keep it far below the acquisition counter (e.g. 1833 vs
  7787 with item indices up to 7786, rendering fine). Forcing it up corrupted the slot.
  Neutralized at 6 sites: `structures.go` (load reconcile removed), `writer.go` (add
  inventory + add storage), `transfer.go` (`advanceDestCounters`), `editor/save.go`
  (`writeContainerLayout`), `inventory_index_repair.go` (repair counter bump). All now
  advance ONLY `NextAcquisitionSortId` and preserve `NextEquipIndex` exactly as the game
  wrote it.
- **Storage quantity topup wrote the wrong record**: `addToInventory` computed the write
  offset from the in-memory slice index (`i*InvRecordLen`), but storage `CommonItems` is a
  COMPACTED slice (empty binary slots skipped on load — see spec/10). With gaps, the topup
  wrote the new quantity to a neighbour record and left the real one unchanged, so crafting
  materials appeared unfilled after reload. New `storageRecordOffset` helper locates the
  real binary record by scanning `slot.Data` for the handle; held inventory (full array)
  unchanged.
- **Duplicate acquisition indices**: now TOLERATED on load/add (the game accepts them;
  `app.go` records a baseline so the post-mutation validator only flags duplicates the
  mutation itself introduces — `ValidatePostMutationBaseline`). Genuine saves carry none,
  so `RepairDuplicateInventoryIndices` (now safe — preserves `NextEquipIndex`) renumbers
  them to fresh unique indices as a one-off cleanup.
- Tests: new `storage_topup_test.go` (regression for the binary-gap topup);
  `inventory_index_repair_test.go` invariant updated (NextEquipIndex preserved, not bumped);
  `save_integrity_test.go` + `app_additems_duplicate_index_test.go` updated for tolerance.
  Full PC/PS4 roundtrip + conversion tests pass; `make build` OK.

### feat(pvp): Network Tuning — Aggressive presets, Summon Signs rename, Experimental relocation

Each functional Network Tuning group now has three modular presets — `Vanilla`,
`Faster`, `Aggressive` — for stronger PS5 PvP test profiles. The active `NetworkTab`
UI is reorganised so Experimental fields live next to the function they belong to
instead of one catch-all section.

- `backend/core/regulation.go`: added `NetworkParamAggressiveReds` (12 / 8 / 12),
  `NetworkParamAggressiveSummons` (10 / 64 / 32 / 10 / 96 / 10 / 10) and
  `NetworkParamAggressiveBlue` (5 / 5 / 20 / 15 / 100). Each is strictly modular and
  keeps the unconfirmed 0x7C, Visitor fields and the other groups at vanilla. This is
  NOT the old removed global `Aggressive` (no cross-group preset, no `aggressive-host`).
- `app.go`: `GetNetworkPreset` dispatches the new keys `aggressive-reds` /
  `aggressive-summons` / `aggressive-blue` (signature unchanged → no bindings regen).
- `frontend/NetworkTab.tsx`: each group exposes Vanilla / Faster / Aggressive buttons
  (still routed through `applyGroupPreset` + `NETWORK_GROUP_KEYS`, so a preset only
  writes its own group's stable fields). Renamed the cooperator group `Summons & Pools`
  → `Summon Signs` with a note that Summoning Pool activation is configured separately
  in World / Exploration. Removed the single Experimental section: 0x7C now sits in
  Reds, Blue Search Parallelism + Retribution Global % in Blue, each flagged Experimental
  and never touched by presets. Visitor controls are hidden from the UI (backend fields
  and save compatibility preserved; they round-trip untouched and join no preset).
- Tests: `tests/regulation_test.go` — `TestNetworkParamAggressive{Reds,Summons,Blue}`
  assert exact values, invariants and that out-of-group/Experimental fields stay vanilla.
  `frontend/networkClamp.test.ts` — TC-A1..A5 cover Aggressive composition, modularity
  and that group Vanilla never resets Experimental/Visitor.
- Docs: `spec/44-network-param-tuning.md` (+ PL) rewritten with per-group
  Vanilla/Faster/Aggressive tables, the Experimental relocation, hidden Visitor fields,
  and the PS5 second-load + Unlock-All + Blue-Cipher-Ring test setup.
- Out of scope / unchanged: MatchmakingRegions, SummoningPools, Colosseums,
  NetworkAreaParam, `cellSize*`, `cellGroup*Range` (documented as future Experimental
  candidates only), Taunter's Tongue, colosseum matchmaking, the orphaned NetworkSpeedPanel.

### feat(pvp): complete MatchmakingRegions from TGA game-data (base + Shadow of the Erdtree)

Expanded the invasion/matchmaking region database from 104 to **274 IDs** so
`Unlock All Invasion Regions` prepares a character for PS5 PvP testing across both
base game and the DLC. This change touches **only** the save-side `MatchmakingRegions`
list (invasion eligibility); it does not modify Network Tuning, Summoning Pools,
Colosseums, RevealMap, SitesOfGrace, NetworkParam, NetworkAreaParam or `cellSize*`.

- `backend/db/data/regions.go`: rebuilt `Regions` against extracted game data — every
  ID is a real Row ID in `regulation.bin` PlayRegionParam (594 rows). Names/areas are
  curated from the Elden-Ring-CT-TGA "Invasion Regions" table (Dasaav; DLC by
  Joel/SeriouslyCasual), which matches PlayRegionParam 1:1. **208 base + 66 DLC = 274.**
  DLC IDs are non-contiguous (`2xxxxxx` legacy dungeons, `4xxxxxx` minor dungeons/gaols/
  forges, `68xxxxx`/`69xxxxx` overworld), so the previous tidy `6900000–6900006` DLC
  block was both incomplete and mislabelled.
- Resolved the `6800000` / `6900000` mapping conflict from game data: PlayRegionParam
  `mapMenuUnlockEventId` (76802 / 76900) → BonfireWarpParam → PlaceName_dlc01 FMG, where
  `680000 = "Gravesite Plain"` and `690000 = "Scadu Altus"`. Both `areaNo = 61` (the m61
  Land of Shadow overworld). So `6800000` = **Gravesite Plain** (DLC), `6900000` =
  **Scadu Altus** (DLC); the real Haligtree interior is the `1500xxx` block (base).
- Removed 20 legacy SaveForge-only IDs (underground `6600xxx`, Farum `6700xxx`,
  `6502001/2`, `1101000`, fabricated DLC `6900001–6900006`): game data is decisive —
  none of them exist in PlayRegionParam, so they are not real PlayRegion IDs and were
  never valid `unlocked_regions` entries. The genuine underground/Farum/Haligtree regions
  use the `12xxxxx`/`13xxxxx`/`15xxxxx` IDs, all now present from the game-data set.
- Added a `DLC bool` field to `RegionData`; rewrote `IsDLCRegion` to be data-driven
  (the old numeric range `6900000–6999999` could not identify the scattered DLC IDs).
- `frontend`: no changes — `WorldTab` reads regions dynamically via `GetAllRegions` and
  groups them by `Area` in collapsible per-area accordions, so the expanded list and the
  two new DLC area groups appear automatically.
- Tests: `backend/db/regions_test.go` (new) — completeness (274/208/66), key DLC IDs
  present, `6800000`/`6900000` conflict resolution, data-driven `IsDLCRegion`, a guard
  against re-adding the fabricated IDs, and `GetAllRegions` uniqueness/ordering. Existing
  `writer_regions_test.go` roundtrip/dedup tests unchanged and passing.
- Note: this is `MatchmakingRegions` (save-side PvP-ready region list) only. Network
  Tuning (search speed/limits), Summoning Pools (separate activation flags, in active
  use), and Colosseums (gate state out of scope) are unchanged.
- Follow-up (deferred): the ~320 PlayRegionParam rows beyond the 274 named regions are
  internal sub-areas/boss-arena/network rows (the bulk of the ~395 entries seen in
  late-game saves) — adding them would need name resolution and is deferred.

### fix(pvp): treat MatchmakingRegions as a curated allowlist + make Unlock/Lock non-destructive

Pre-commit safety review of the 274-region change. Reframed the region list as a
**curated invasion/blue allowlist** (a subset of `PlayRegionParam`), not "every world
region", and made all bulk region operations non-destructive so they can never wipe
advanced raw region IDs a real save carries.

- Safety model / negative validation: confirmed the TGA "Invasion Regions" list is a
  *dedicated invasion-targeting* list (its own guide: "which regions the Near/Far
  invasion option attempts to invade"), with open-world / dungeon / boss-fog categories.
  It deliberately omits multiplayer hubs (Roundtable Hold has **no** `PlayRegion` row at
  all) and colosseums (separate matchmaking). The `isBoss`-tagged entries are kept — they
  are legitimate invasion contexts (several are premier PvP zones, e.g. Haligtree
  Promenade / Town Plaza), not multiplayer-disabled arena interiors. **No IDs removed**:
  the genuinely forbidden categories are already absent by construction.
- `app_world.go`: added `mergeUnlockedRegions(curatedIDs, existingRaw)` and routed
  `BulkSetUnlockedRegions` through it. Bulk ops now set the curated membership to exactly
  the passed IDs while preserving every raw ID outside the allowlist
  (`!db.IsKnownRegionID`). Result: Unlock All = `existingRaw ∪ allowlist`, Lock All =
  `existingRaw − allowlist`, per-area toggles always keep non-curated raw IDs. The
  frontend is unchanged — the guarantee lives in the backend. Per-region toggle
  (`SetRegionUnlocked`) was already non-destructive (operates on the raw list).
- `app_pvp.go`: the `ApplyPvPPreparation` MatchmakingRegions module ("Unlock All" via
  preset) now uses the same `mergeUnlockedRegions`, so the preset path is non-destructive
  too. No Network Tuning / preset content changed.
- `backend/db/db.go`, `backend/db/data/regions.go`: clarified that `IsKnownRegionID` /
  `Regions` are the curated allowlist (membership-based, a subset of the 594-row
  `PlayRegionParam`), not the full world; documented why hubs/colosseums are excluded.
- Tests: `app_world_regions_test.go` (new) — Unlock All / Lock All / per-area preserve a
  real non-curated raw ID (`1001000`, a `PlayRegionParam` row outside the allowlist).
  `backend/db/regions_test.go`: added `TestNoForbiddenPvPLocations` (no hub/colosseum
  names) and reframed the count-test comments as a curated allowlist.
- Docs: `spec/11-regions.md` + `spec/lang-pl/11-regions.md` rewritten for the curated
  allowlist (counts 104→274, area enum 11→13, `RegionData.DLC`, non-destructive
  Unlock/Lock semantics, source = TGA filtered against `PlayRegionParam`, mitigated V3/V5/V8).
  `spec/48` + PL updated for the 274-region non-destructive MatchmakingRegions module.
- System separation (unchanged, restated in docs): `MatchmakingRegions` = region
  eligibility; `Network Tuning` = matchmaking speed/timeouts/limits; `SummoningPools` =
  separate activation system (many pools confirmed working in-game after earlier fixes,
  full completeness not yet formally verified); `Colosseums` = access exists, the physical
  gate remains a deferred problem. NetworkParam on PS5 requires the load → menu →
  second-load procedure before online testing (cause not investigated).

### feat(pvp): unified network presets — Faster Reds / Summons / Blue, remove Aggressive

Replaced the two divergent network-preset systems with one source of truth. The
active UI (`NetworkTab.tsx`, rendered via `PvPTab`) used hardcoded frontend
constants (`VANILLA/FASTER/AGGRESSIVE_VALUES`); the orphaned `NetworkSpeedPanel`
(in the unused `PvPPreparationTab`) used separate backend presets — the two could
drift. NetworkTab now fetches preset values from the backend via `GetNetworkPreset`.

- `backend/core/regulation.go`: new `NetworkParamFasterReds()` (8/12/15, 0x7C stays
  vanilla 5), `NetworkParamFasterSummons()` (sign refresh/buffer), `NetworkParamFasterBlue()`
  (faster+wider co-op blue; `maxCoopBlueSummonCount` and `allAreaSearchRateVsBlue` kept
  vanilla). Added cross-field invariants to `ValidateNetworkParams`:
  `reloadSignCellCount <= reloadSignTotalCount <= singGetMax` and
  `reloadSearchCoopBlueMin <= reloadSearchCoopBlueMax`.
- `app.go` `GetNetworkPreset`: new keys `faster-reds`, `faster-summons`, `faster-blue`,
  `vanilla`. Legacy keys kept for the orphaned panel.
- `frontend`: rewrote `NetworkTab.tsx` — three functional groups (Reds, Summons & Pools,
  Blue/Hunter) each with Vanilla/Faster buttons, a collapsed Experimental section
  (unknown 0x7C field locked at 5 with warning; blue legacy extras; visitor/ring-search
  fields relabelled — "No confirmed Taunter's Tongue speed control found"). Removed the
  global `Aggressive` profile entirely. New `networkClamp.ts` enforces the same invariants
  client-side (+ vitest `networkClamp.test.ts`, 10 cases).
- Removed the broken `Aggressive` invasion profile (was 15/6/3/15 — 3s timeout broke
  near-and-far matchmaking, near-continuous retry, wrote the unconfirmed 0x7C field).
- Docs: `spec/44` (EN+PL) — added the 0x7C unconfirmed-field row, a source-of-truth note
  (binary `NetworkParam.param` is authoritative; `csv` showed wrong 32/8 vs binary 20/10),
  and replaced the obsolete suggested presets with the three implemented presets.
- Tests: `tests/regulation_test.go` — defaults (20/10, 0x7C=5), one test per preset
  (exact values + neighbouring groups untouched), and invariant rejection. Existing PC/PS4
  roundtrip tests unchanged and passing.
- Not implemented (research-only): `NetworkAreaParam.cellSize*`, `cellGroup*Range` backend
  patching, Taunter's Tongue preset, colosseum preset.

### feat(items): polish generated weapon details

Builds on the Phase 3C.4 wiring with a polish pass on the weapon
details panel — adds Critical, fixes Attribute Scaling rendering,
hides empty Attributes Required rows, and surfaces weapon-attached
SpEffects as a new Passive Effects section. Backend `WeaponStatsV1`
gains a `Critical` field and a `PassiveEffects []WeaponPassiveEffect`
slice, both regenerated from `EquipParamWeapon` (with `SpEffectParam`
for effect resolution).

**Critical**:

- New `WeaponStatsV1.Critical int32` populated as
  `100 + EquipParamWeapon.throwAtkRate` (CSV stores the offset above
  a base of 100; pre-adding the base lets the UI render verbatim).
- Examples: Misericorde → 140, Lordsworn's Straight Sword → 110,
  Uchigatana / most weapons → 100.
- Rendered in the Attack Power table instead of the previous
  hard-coded N/A; legacy fallback unchanged when V1 is absent.

**Attribute Scaling**:

- Now rendered as game-like grade plus raw value, e.g.
  `Dex D (50)`, `Int C (60)`. Thresholds follow the community
  standard (S ≥ 175, A ≥ 140, B ≥ 90, C ≥ 60, D ≥ 25, E ≥ 1).
- Zero-scaling rows are hidden; if a weapon has zero scaling
  across the board, a compact `None` placeholder shows instead
  of five empty rows.
- A small `?` help icon next to the heading opens a local popover
  explaining what the grade / raw value mean and that V1 uses
  level +0 raw correction values (upgrade multipliers deferred).

**Attributes Required**:

- Rows with `0` are filtered out, so weapons that only scale on
  Str/Dex no longer show `Int 0 / Fai 0 / Arc 0` placeholders
  (e.g. Moonveil now shows just `Str 12 / Dex 18 / Int 23`).
- Empty list falls back to `None`.

**Passive Effects (backend)**:

- New `WeaponPassiveEffect{Kind, Source, SpEffectID, Label, Value,
  Known}` and `WeaponStatsV1.PassiveEffects []WeaponPassiveEffect`.
- Generator resolves `spEffectBehaviorId0..2` (on-hit) and
  `residentSpEffectId/1/2` (resident) against `SpEffectParam`.
  On-hit slots map status `*AttackPower` columns
  (`poizonAttackPower`, `diseaseAttackPower`, `bloodAttackPower`,
  `freezeAttackPower`, `sleepAttackPower`, `madnessAttackPower`,
  `curseAttackPower`) to user-facing labels: **Poison**,
  **Scarlet Rot**, **Blood Loss**, **Frost**, **Sleep**,
  **Madness**, **Death Blight**.
- Resident effects use a small curated label map:
  - `5141100` → *Restores FP upon defeating enemies* (Sacrificial
    Axe family).
  - `5071100` → *Restores HP upon defeating enemies* (Serpent-God's
    Curved Sword family).
  - `1927` → *Boosts Dragon Communion incantations*.
  - `1919` → *Boosts death sorceries*.
- Unresolved SpEffect IDs are kept with `Known=false` and a generic
  label (`Unknown on-hit effect` / `Unknown resident effect`) plus
  the raw SpEffect ID, so nothing is silently dropped.
- Slot ordering is deterministic (on-hit 0→1→2, then resident
  0→1→2; within a slot, statuses iterate in the order above).

**Passive Effects (UI)**:

- New `Passive Effects` section rendered only when at least one
  effect exists — most weapons have none and we keep the panel
  quiet for them rather than rendering an empty placeholder.
- Effects grouped by `Kind`: `on_hit` → **On hit**, `resident` →
  **While held**. Empty groups (e.g. weapon with only resident
  effects) hide their sub-heading.
- Known on-hit status: `Label (Value)` (e.g. `Blood Loss (45)` for
  Uchigatana, `Blood Loss (50)` for Moonveil / Rivers of Blood).
- Known resident: bare label (e.g. `Restores FP upon defeating
  enemies` for Sacrificial Axe).
- Unknown effects: `Label (SpEffect <id>)` rendered italic/muted
  to differentiate from resolved entries.
- A `?` help icon next to the heading opens a local popover that
  explains the On hit / While held split and the Unknown fallback
  behaviour.

**Tests** (`backend/db/data/`):

- `TestWeaponStatsV1CriticalKnownValues` anchors Critical on
  Misericorde (140), Lordsworn's Straight Sword (110), and
  Uchigatana (100).
- `TestWeaponStatsV1PassiveOnHitBloodLoss` covers Uchigatana (45)
  + Moonveil / Rivers of Blood (50).
- `TestWeaponStatsV1PassiveResidentLabels` covers the four curated
  resident labels.
- `TestWeaponStatsV1PassiveNoneForPlainWeapon` guards Lordsworn's
  Straight Sword against spurious effects from the resolver.

**What did NOT change**:

- Armor / spell panels untouched.
- Legacy `WeaponStats` projection unchanged; legacy `item.weapon`
  consumers still see the same fields.
- `WeaponStatsV1.Status*` fields stay zero (legacy projection is
  still deferred); the `status-deferred` warning is retained for
  that reason and explicitly explained in the regenerated header
  comment of `weapon_stats_generated.go`.
- No reinforcement-level math yet — V1 still ships raw `correct*`
  values; ReinforceParamWeapon multipliers remain on the V2
  roadmap.

### feat(ui): render generated weapon stats in details panel

Phase 3C.4 wires `ItemDetailPanel` to the typed Phase 3C.3 stats
payload (`item.stats.weapon`, `WeaponStatsV1`) while preserving full
fallback to the legacy `item.weapon` projection. The panel layout is
unchanged — the existing Attack / Guard / Scaling / Requirements grid
stays as it was — but the values now come from `EquipParamWeapon`
via the V1 record, and V1-only data finally shows up in the UI.

**Source preference**: `item.stats.weapon` is preferred (nullish-aware:
zero is a valid value, e.g. Longsword Holy = 0); `item.weapon` is the
fallback for IDs not covered by the V1 generator. For weapon-like
categories without either pointer, the previous "stats data missing"
banner still appears.

**R-STA-01 (Dark → Holy)**: the backend renamed Elden Ring's legacy
`attackBaseDark` / `darkGuardCutRate` columns to `AttackHoly` /
`GuardHoly` already in Phase 3C.1. The UI labels every relevant row
as **Holy** — no "Dark" string is ever rendered.

**New / improved rendering**:

- **Attack Power** — V1 source, with optional `Stamina` row when
  `V1.AttackStamina > 0` (most weapons leave it 0, so we keep the
  table compact rather than show a perpetual zero).
- **Guarded Dmg Negation** — previously hard-coded to N/A across all
  six rows; now populated from `V1.GuardPhysical/Magic/Fire/
  Lightning/Holy` and `V1.GuardBoost`. Shields and weapons with
  guard data finally show their real values; entries without V1
  data fall back to N/A as before.
- **Attribute Scaling** — previously had Arc hard-coded to N/A; now
  pulls `V1.ScalingArcRaw`. Str/Dex/Int/Fai prefer V1, legacy
  fallback intact. Labels are still raw numbers (no fake S/A/B/C
  letter grades — those need `CalcCorrectGraph` and are deferred).
- **Attributes Required** — prefers V1 `StatReq*`, falls back to
  legacy `Req*`.
- **Item Info** — `Max Upgrade` now sources from
  `V1.MaxUpgrade` when present (covers Sacred Relic Sword +10,
  Longsword +25, ammo 0). New optional `Reinforcement` row shows
  "Somber" / "Standard" derived from `V1.IsSomber` / `V1.IsInfusable`;
  hidden when neither applies (e.g. ammo).
- **Weight** — V1 weight preferred; legacy `item.weapon.Weight`,
  `item.armor.Weight`, and `item.weight` remain the fallback chain.

**What did NOT change**:

- Armor and Spell panels are untouched. Their resolution path
  remains `item.armor` / `item.spell` from `data.Descriptions`.
- Panel header, icon, caption, description, location, and "no data"
  fallback render exactly as before.
- No Wails bindings change; this is a UI-only commit.
- `WeaponStatsV1.Warnings` is intentionally NOT surfaced — the V1
  generator only emits a `status-deferred` informational warning,
  not anything an end user needs to see.

### feat(db): expose generated weapon stats payload

Adds a new optional `ItemEntry.Stats` payload that exposes the
generated `WeaponStatsV1` record (Phase 3C.1) alongside the legacy
`Weapon` projection. The change is additive and shape-preserving:
existing UI bindings continue to read `item.weapon` exactly as Phase
3C.2 left them; Phase 3C.3 simply attaches `item.stats` so the
frontend can render V1-only fields (guard cuts, stamina attack,
arcane scaling, somber/upgrade flags, etc.) without re-querying the
backend or touching the legacy projection.

**Payload shape (minimal wrapper, no `any` / `interface{}`)**:

```go
type ItemStatsKind string

const (
    ItemStatsKindNone   ItemStatsKind = ""
    ItemStatsKindWeapon ItemStatsKind = "weapon"
    ItemStatsKindArmor  ItemStatsKind = "armor"
    ItemStatsKindSpell  ItemStatsKind = "spell"
    ItemStatsKindGoods  ItemStatsKind = "goods"
    ItemStatsKindAoW    ItemStatsKind = "ash_of_war"
)

type ItemStatsData struct {
    Kind        ItemStatsKind  `json:"kind"`
    Weapon      *WeaponStatsV1 `json:"weapon,omitempty"`
    SourceParam string         `json:"sourceParam,omitempty"`
    SourceRowID uint32         `json:"sourceRowId,omitempty"`
    Warnings    []string       `json:"warnings,omitempty"`
}
```

Future stats kinds (armor / spell / goods / ash_of_war) will get
their own concrete pointer fields when their generated tables land —
the enum already reserves the constants. We deliberately avoid `any`
so the Wails-generated TS bindings stay strongly typed.

`enrichItemEntry` now sets `e.Stats` after the existing
`weaponStatsV1ToLegacy` step:

- `Kind = ItemStatsKindWeapon`
- `Weapon = &copy(v)` (local copy — map values are not addressable)
- `SourceParam = "EquipParamWeapon"`
- `SourceRowID = v.SourceRowID`
- `Warnings = v.Warnings`

Items without a V1 entry get `e.Stats == nil`. Legacy `e.Weapon` /
`e.Armor` / `e.Spell` / `e.Text` pointers remain populated exactly as
before — the Phase 3C.3 hookup is purely additive.

**Wails bindings change** (auto-regenerated by `wails generate
module` during `make build`):

- `data.WeaponStatsV1` — new TS class with all 40 fields typed
  (`number` / `boolean` / `string[]`), no `any`.
- `data.ItemStatsData` — new TS class with `kind` / `weapon?` /
  `sourceParam?` / `sourceRowId?` / `warnings?`.
- `db.ItemEntry.stats?: data.ItemStatsData` — new optional field.

Frontend UI components are not modified in this commit — Phase 3C.4
will render the V1-only fields in `ItemDetailPanel`.

New tests in `backend/db/item_stats_payload_test.go`:

- `TestEnrichItemEntrySetsWeaponStatsPayload` — Lance: payload
  present, `Kind=weapon`, `Weapon.ItemID` matches, `SourceParam`
  and `SourceRowID` populated.
- `TestEnrichItemEntryStatsPayloadHolyMapping` — R-STA-01 payload
  guard on Sacred Relic Sword (`Stats.Weapon.AttackHoly` ==
  legacy `Weapon.HolyDamage`).
- `TestEnrichItemEntryStatsPayloadV1OnlyFields` — Longsword
  (standard +25 / GemMountType=2), Sacred Relic Sword + Icon
  Shield (somber +10 / GemMountType=0); shield `GuardHoly` cross-
  checked against the source map.
- `TestEnrichItemEntryStatsPayloadNilForNonWeapon` — armor / spell
  anchors carry no `Stats` payload but retain legacy `Armor` /
  `Spell` pointers.
- `TestEnrichItemEntryStatsPayloadDoesNotBreakLegacyWeapon` —
  Longsword: legacy `e.Weapon` and `e.Stats.Weapon` both populated;
  legacy field values match the V1 mapper output.
- `TestEnrichItemEntryStatsPayloadNilForMissing` — unknown ID
  produces no panic and `Stats == nil`.

All Phase 3C.2 / 3B regression tests continue to pass unchanged.

### feat(db): enrich legacy weapon stats from generated data

Wires the Phase 3C.1 `WeaponStatsV1ByID` table into runtime enrichment.
`enrichItemEntry` now applies a new step after the existing
descriptions.go / ItemTexts seeds: when the item ID has a V1 entry, the
legacy `ItemEntry.Weapon` pointer is rebuilt from V1 data via a small
explicit mapper, and `e.Weight` is overridden when V1 carries a
non-zero weight.

The change is payload-shape-preserving — `ItemEntry` still exposes the
legacy `Weapon *data.WeaponStats` field with the same JSON tag, so the
frontend renders item details exactly as before. Phase 3C.3 will add a
new payload field for the V1-only fields (stamina attack, guard cuts,
arcane scaling, somber/upgrade flags, etc.) without disturbing the
legacy projection.

**R-STA-01 mapping (critical, runtime layer)**. `V1.AttackHoly` —
sourced from `EquipParamWeapon.attackBaseDark` in Phase 3C.1 — is
projected onto legacy `WeaponStats.HolyDamage`. Legacy
`data.WeaponStats` has no Dark-named field; the Dark→Holy rename
happens entirely inside the V1 generator and the runtime mapper.
Sacred Relic Sword anchor: V1 `AttackHoly=76` → enriched
`Weapon.HolyDamage=76`.

**Mapping table (V1 → legacy)**:

| V1 field          | Legacy `WeaponStats` field |
| ----------------- | -------------------------- |
| `Weight`          | `Weight`                   |
| `AttackPhysical`  | `PhysDamage`               |
| `AttackMagic`     | `MagDamage`                |
| `AttackFire`      | `FireDamage`               |
| `AttackLightning` | `LitDamage`                |
| `AttackHoly`      | `HolyDamage`               |
| `ScalingStrRaw`   | `ScaleStr`                 |
| `ScalingDexRaw`   | `ScaleDex`                 |
| `ScalingIntRaw`   | `ScaleInt`                 |
| `ScalingFaiRaw`   | `ScaleFai`                 |
| `StatReqStr`      | `ReqStr`                   |
| `StatReqDex`      | `ReqDex`                   |
| `StatReqInt`      | `ReqInt`                   |
| `StatReqFai`      | `ReqFai`                   |
| `StatReqArc`      | `ReqArc`                   |

V1-only fields (`AttackStamina`, `Guard*`, `ScalingArcRaw`, `WepType`,
`IsInfusable`, `IsSomber`, `MaxUpgrade`, `Status*`, `DefaultAoWID`,
`SourceRowID`, `Warnings`) intentionally stay on the V1 record — they
are NOT folded into unrelated legacy fields, and will be surfaced
through a new payload field in Phase 3C.3.

Mapper is explicit (no reflection, no name-based auto-copy). Int32 →
uint32 conversion clamps negatives to zero via `nonNegU32`; V1 currently
emits non-negative numbers but the guard keeps the projection safe.

Items without a V1 entry continue to use the legacy `data.Descriptions`
fallback for `Weapon`. `Armor`, `Spell`, `Description`, `Location`, and
`Text` are unaffected — Phase 3C.2 is scoped strictly to weapon stats.

New tests in `backend/db/enrich_weapon_stats_test.go`:

- `TestEnrichItemEntryUsesWeaponStatsV1` — Lance `0x010450A0`: every
  legacy `WeaponStats` field matches the V1-mapped value.
- `TestEnrichItemEntryWeaponStatsV1HolyMapping` — R-STA-01 runtime
  guard on Sacred Relic Sword `0x002F4D60`.
- `TestEnrichItemEntryWeaponStatsV1SomberAndStandardAnchors` — V1
  IsSomber/MaxUpgrade flags lock down on Longsword / Great Épée /
  Fire Knight's Greatsword (standard +25) and Sacred Relic Sword /
  Icon Shield (somber +10); enriched `Weapon.PhysDamage` /
  `HolyDamage` confirm V1 drove enrichment.
- `TestEnrichItemEntryWeaponStatsV1Ammo` — Fire Arrow / Lightning
  Bolt enrich from V1 (legacy `descriptions.go` historically had no
  ammo stats).
- `TestEnrichItemEntryWeaponStatsFallbackToDescriptions` — dynamic
  discovery of a descriptions.go orphan outside V1 coverage.
- `TestEnrichItemEntryPreservesArmorSpellStats` — Armor and Spell
  pointers untouched by the Phase 3C.2 hookup.
- `TestEnrichItemEntryWeaponStatsDoesNotAffectText` — Phase 3B.3
  Text payload + Description / Location regression guard.
- `TestNonNegU32` — clamp helper contract.

### feat(db): add generated weapon stats table

Adds `WeaponStatsV1` — a new generated Go map at
`backend/db/data/weapon_stats_generated.go` shipping base stats for all
736 weapon-like items across the four covered categories:

- `melee_armaments`: 439 items
- `shields`: 165 items
- `ranged_and_catalysts`: 64 items
- `arrows_and_bolts`: 68 items

The data is sourced directly from `EquipParamWeapon.csv` and
`ReinforceParamWeapon.csv` (regulation dump) and decoupled from the
curated `descriptions.go` stat fields, which remain partial. Phase 3C.2
will wire this table into runtime enrichment; Phase 3C.1 is **data
only** — no `enrichItemEntry`, `ItemEntry`, frontend, or Wails binding
changes ship in this commit.

**R-STA-01 mapping (critical)**. Elden Ring's regulation CSV keeps Dark
Souls' legacy "Dark" naming for what the live game (and this app) calls
Holy. The generator maps these columns explicitly:

- `AttackHoly` ← `EquipParamWeapon.attackBaseDark`
- `GuardHoly`  ← `EquipParamWeapon.darkGuardCutRate`

There is no separate "Holy" CSV column. Sacred Relic Sword
(`0x002F4D60`) is the canonical anchor: `attackBaseDark = 76`,
`darkGuardCutRate = 45` → `AttackHoly = 76`, `GuardHoly = 45`.

**Somber / standard inference** comes from `ReinforceParamWeapon` band
sizes at the weapon's `reinforceTypeId`:

- 26-row band → `IsInfusable=true`, `IsSomber=false`, `MaxUpgrade=25`
- 11-row band → `IsInfusable=false`, `IsSomber=true`, `MaxUpgrade=10`
- 1-row band  → no upgrades (arrows, bolts, torches, sealed weapons)

**Deferred to V2** (recorded as per-row warnings):

- Status-effect damage applied by the weapon (poison, bleed, frost,
  sleep, madness, scarlet rot) — derivation requires SpEffect /
  AtkParam traversal; Phase 3C.1 leaves Status* fields at zero with a
  `status-deferred` warning on every entry.
- Letter-grade scaling (S/A/B/C/D/E) — needs CalcCorrectGraph
  evaluation; raw `correctStrength` etc. are shipped as
  `ScalingStrRaw` instead.

**Generator** lives at `tmp/scripts/generate_weapon_stats.go`
(untracked, like other generators). Output is sorted by item ID
ascending, contains no body timestamps, and lists SHA256 of every input
in the header. Reproducibility is asserted in
`TestWeaponStatsV1GeneratorReproducible` which re-runs the generator
under `go run` and compares the file hash before and after.

New tests in `backend/db/data/weapon_stats_generated_test.go`:

- `TestWeaponStatsV1HasKnownIDs` — 9 anchor IDs across all 4
  categories incl. Sacred Relic Sword and ammo.
- `TestWeaponStatsV1Coverage` — every entry in `Weapons`, `Shields`,
  `RangedAndCatalysts`, `ArrowsAndBolts` has a `WeaponStatsV1ByID`
  entry (no allow-list).
- `TestWeaponStatsV1HolyMapping` — Sacred Relic Sword `AttackHoly` and
  `GuardHoly` are non-zero (R-STA-01).
- `TestWeaponStatsV1MaxUpgradeSomberStandard` — Longsword / Great Épée
  +25 standard, Sacred Relic Sword / Icon Shield +10 somber.
- `TestWeaponStatsV1GemMountType` — infusable weapons report
  `gemMountType=2`; unique/somber report `0`.
- `TestWeaponStatsV1Ammo` — Fire Arrow / Lightning Bolt carry
  `MaxUpgrade=0`, ammo-bracket `WepType`, no guard data.
- `TestWeaponStatsV1GeneratorReproducible` — byte-identical re-run.
- `TestWeaponStatsV1NoPanicOnMissing` — zero-value lookup.

### feat(ui): render generated item text in details panel

Wires `ItemDetailPanel.tsx` to the Phase 3B.3 `item.text` payload. Three
text fields are now surfaced with preference for the generated source
and a legacy fallback:

- `Caption` — new optional flavour-text section above `Description`,
  rendered italicised when `item.text.Caption` is non-empty.
- `Description` — prefers `item.text.Description`, falls back to legacy
  `item.description`.
- `Location` — new section, prefers `item.text.Location`, falls back to
  legacy `item.location`. (Previously the panel did not render
  `location` at all — the curated Fextralife-sourced strings shipped in
  `descriptions.go` were dead data on the frontend.)

The panel title still uses `item.name`, so app-curated disambiguations
such as "Letter from Volcano Manor (Istvan)" / "(Rileigh)" and the
Misricorde / Chain Gauntlets style overrides keep their app-side
suffixes. `CanonicalName`, per-field provenance, and `DLCSource` are
deliberately not exposed in this phase to avoid cluttering the panel —
they remain available on the model for future tooling.

The "No data" fallback now considers caption / description / location
plus stats sections, so items with only a caption (or only a location)
no longer trip the empty-state message.

Backend, save-writing paths, the generated `ItemTexts` table, and the
Wails bindings are unchanged.

### feat(db): expose generated item text payload

Surfaces the Phase 3B.1 `ItemTextData` value on the `ItemEntry` JSON
payload via a new optional `Text *data.ItemTextData` field. After
enrichment `e.Text` carries the generated DisplayName / CanonicalName /
Caption / Description / Location plus per-field provenance, or stays
`nil` for IDs without a matching `ItemTexts` entry.

Phase 3B.2 behaviour is unchanged: legacy `Description` and `Location`
on `ItemEntry` keep flowing from the same FMG/curated fallback chain so
existing UI bindings render identically without code changes. Save
writing, stats enrichment, and the generated `item_text_generated.go`
table are untouched. Wails regenerates `frontend/wailsjs/go/models.ts`
to add the `data.ItemTextData` class and the optional `text?` field on
`db.ItemEntry`; no other generated bindings change.

The Go pointer is populated by copying the map value into a local
variable before taking its address (`text := t; e.Text = &text`) — map
values are not addressable, so this avoids a subtle compiler error if
future refactors inline the assignment.

New tests in `backend/db/enrich_text_test.go`:

- `TestEnrichItemEntrySetsTextPayload` — Lance (`0x010450A0`) populates
  `e.Text.DisplayName` / `CanonicalName` / `Caption` from `ItemTexts`.
- `TestEnrichItemEntryTextPayloadPreservesLegacyFields` — legacy
  `Description` and `Location` survive payload exposure.
- `TestEnrichItemEntryTextPayloadNilForMissing` — unknown IDs produce
  `e.Text == nil` with no panic.
- `TestEnrichItemEntryTextPayloadAppDisambiguation` — Volcano Manor
  letters (`0x40001FBF` / `0x40001FC4`) keep their app-suffixed
  DisplayName while CanonicalName carries the bare FMG name.

### feat(db): enrich item descriptions from generated text data

Wires the Phase 3B.1 `ItemTexts` generated table into runtime enrichment:
`enrichItemEntry` now prefers `data.ItemTexts[id].Description` and
`Location` over the legacy `data.Descriptions[id]` values when populated,
falling back to the legacy map for IDs not covered by the new table.

The change is text-only and additive — the `ItemEntry` JSON payload
shape is unchanged, no new fields are exposed yet (Phase 3B.3 will add
`Text *ItemTextData`), and the Wails bindings do not need regeneration.
Legacy `Weight`, `Weapon`, `Armor`, and `Spell` pointers continue to
flow from `data.Descriptions` exactly as before — Phase 3C will replace
them with dedicated generated stats tables.

This unlocks higher-fidelity FMG-sourced descriptions (76.8 % FMG Info
coverage from Phase 3A audit) plus the curated Fextralife-sourced
Location data (94.8 % coverage) without touching frontend code.

New tests in `backend/db/enrich_text_test.go`:

- `TestEnrichItemEntryUsesItemTextsDescription` — Black Syrup
  (`0x401EA3D3`) renders the FMG description supplied by ItemTexts.
- `TestEnrichItemEntryUsesItemTextsLocation` — Lance (`0x010450A0`)
  renders the curated Location surfaced via ItemTexts.
- `TestEnrichItemEntryFallsBackToDescriptions` — orphan IDs present in
  `descriptions.go` but absent from `ItemTexts` keep their legacy text.
- `TestEnrichItemEntryPreservesLegacyStats` — Lance still carries its
  `WeaponStats` pointer after the wiring change.
- `TestEnrichItemEntryNoPanicMissingText` — unknown IDs enrich safely.

### fix(db): add missing SOTE Black Syrup key item

Adds the previously-missing shipped Shadow of the Erdtree key item
`0x401EA3D3` "Black Syrup" to `backend/db/data/key_items.go`.
Regulation row 2008019 (EquipParamGoods, `dlc01` FMG) ships with
`goodsType=1`, `iconId=801`, `sortId=204363`, `maxNum=1`, `refCategory=0` —
the canonical SOTE-questline key item that had been absent from the app
inventory database. The sub-category auto-classifier in
`key_items_subcat.go` routes it to the "Inactive Great Runes + Keys +
Medallions" catch-all (no curated rule matches).

Scope is intentionally narrow per Phase 2B.4 review: the 7 other items
flagged by the audit as missing real shipped Goods entries remain
deferred:

- **Scorpion Stew / Gourmet Scorpion Stew** (Set A `0x401E8930/31`) — Set B
  variants `0x401E8932/33` are already in `tools.go` and both Sets ship
  with full regulation params; canonical A/B choice requires drop-table
  research before any replace/coexist decision.
- **5 Miquella questline phrases** (`0x401EA7A8`–`0x401EA7AC`: Ring of
  Miquella, May the Best Win, The Two Fingers, Let Us Go Together,
  O Mother) — already exposed via `gestures.go` under the same canonical
  IDs. The "missing" classification is an audit-tooling gap
  (Gesture↔EquipParamGoods cross-source matching is not yet wired into
  `tmp/item-audit/scripts/build_comparison.py`) deferred to a tooling
  cleanup commit.

New regression tests in `phase2b4_black_syrup_test.go`:

- `TestPhase2B4BlackSyrupPresent` — verifies the entry's name, category,
  caps, icon path and `dlc` flag.
- `TestPhase2B4BlackSyrupNoDuplicateAcrossMaps` — guards against
  accidentally adding the same ID or display name to `Tools`, `Gestures`,
  `Information`, `StandardAshes`, `ArrowsAndBolts`, `BolsteringMaterials`,
  `CraftingMaterials`, `Incantations`, `Sorceries`.
- `TestPhase2B4ScorpionStewUntouched` — pins the Set A/B status quo so
  the deferred decision isn't silently subverted.
- `TestPhase2B4MiquellaPhrasesStayInGestures` — pins the 5 phrase IDs in
  `Gestures` and asserts no duplicate entry in `KeyItems`/`Tools`.

### fix(db): add missing Volcano Manor letter and disambiguate quest letters

Adds the previously-missing shipped `0x40001FBF` Letter from Volcano
Manor entry to `backend/db/data/info.go` and renames its sibling
`0x40001FC4` so the Add Items UI can distinguish the two. Both IDs
ship with identical FMG canonical name "Letter from Volcano Manor",
identical `iconId=3055`, and consecutive `sortId` (451010, 451020) —
they are two real quest letters from Tanith during the Recusant
questline, discriminated only by their description target NPC.

| App ID         | Display name (after)                     | Quest target           | regulation row |
|----------------|------------------------------------------|------------------------|----------------|
| `0x40001FBF`   | Letter from Volcano Manor (Istvan)       | Old Knight Istvan      | 8127 (new in app) |
| `0x40001FC4`   | Letter from Volcano Manor (Rileigh)      | Rileigh the Idle       | 8132 (renamed)    |
| `0x40001FC5`   | Red Letter (unchanged)                   | Juno Hoslow            | 8133 (already discriminated in FMG) |

The orphan description entry for `0x40001FBF` in
`backend/db/data/descriptions.go` is now naturally picked up by
`enrichItemEntry`. Disambiguating the display names is an
app-only UI/database concern — save behavior, item IDs and FMG
canonical names are unchanged. New unit test
`TestPhase2B3VolcanoManorLettersDisambiguated` guards the rename
plus a regression check ensuring no `Information` entry uses the
bare FMG name without a target-NPC suffix.

### fix(db): replace cut Sealed Spiritsprings note with shipped ID

Replaces the broken Set-B-equivalent `0x401EA443` Note: Sealed
Spiritsprings entry in `backend/db/data/info.go` with the shipped
canonical variant `0x401EA3DF`. The cut variant had
`goodsType=0 / iconId=0 / sortId=999999` in
`tmp/regulation-bin-dump/csv/EquipParamGoods.csv` (row 2008131) and
rendered as an `"ICON"` placeholder under the Tools tab; the shipped
variant (row 2008031) has `goodsType=12`, `iconId=3861`,
`sortId=453100`, full FMG name/description/caption metadata, and slots
naturally next to `0x401EA3D9` Furnace Keeper's Note.

| Field           | Before (`0x401EA443`)                       | After (`0x401EA3DF`)       |
|-----------------|---------------------------------------------|----------------------------|
| In Information map | yes                                       | yes                        |
| Name            | Note: Sealed Spiritsprings                  | Note: Sealed Spiritsprings |
| Category        | info                                        | info                       |
| Flags           | `["dlc", "cut_content", "ban_risk"]`        | `["dlc"]`                  |
| Param goodsType | 0                                           | 12                         |
| Param iconId    | 0                                           | 3861                       |
| Param sortId    | 999999                                      | 453100                     |

Also removed the orphaned `0x401EA443: {Location: "Location unknown
or not yet indexed."}` stub from `backend/db/data/descriptions.go`.
Sibling SOTE Notes (`0x401EA3DB`–`0x401EA3DE`) carry no descriptions
either, so adding one for `0x401EA3DF` is deferred to a separate
descriptions pass.

Tests: `TestPhase2B3SealedSpiritspringsRealVariantAbsent` was flipped
to `TestPhase2B3SealedSpiritspringsCanonicalReplacement`, asserting
the canonical entry is present with `dlc`-only flags and Mechanics /
Locations sub-category, and that the broken `0x401EA443` is absent.

See `tmp/item-audit/sealed_spiritsprings_investigation.md` for the
side-by-side comparison, evidence trail, and the three strategies
considered (A: replace — adopted; B: coexist; C: hold).

### fix(db): add missing information notes

Phase 2B.3 batch 1 of the item database audit adds 13 entries to
`backend/db/data/info.go`. Each entry was verified to have
`goodsType=12`, real `iconId` and finite `sortId` in
`tmp/regulation-bin-dump/csv/EquipParamGoods.csv`, no collision with an
existing prefixed ID in `info.go`, and an FMG name confirmed by
`tmp/regulation-bin-dump/msg/name_mapping.csv`.

| ID (hex)   | ID (dec) | Name                                          | Group         | DLC |
|------------|----------|-----------------------------------------------|---------------|-----|
| 0x40002020 |  8224    | Note: The Preceptor's Secret                  | unique note   | no  |
| 0x40002021 |  8225    | Weathered Map                                 | unique note   | no  |
| 0x40002312 |  8978    | Sellia's Secret                               | unique note   | no  |
| 0x4000238D |  9101    | About Sorceries and Incantations              | About * base  | no  |
| 0x4000239B |  9115    | About Flask of Wondrous Physick               | About * base  | no  |
| 0x400023A0 |  9120    | About Teardrop Scarabs                        | About * base  | no  |
| 0x400023B5 |  9141    | About Great Runes                             | About * base  | no  |
| 0x400023B6 |  9142    | About the Cave of Knowledge                   | About * base  | no  |
| 0x400023BE |  9150    | About Duels                                   | About * base  | no  |
| 0x400023BF |  9151    | About United Combat and Combat Ordeals        | About * base  | no  |
| 0x400023C0 |  9152    | About Combat with Spirit Ashes                | About * base  | no  |
| 0x400023C1 |  9153    | About Marika's Effigy at the Roundtable       | About * base  | no  |
| 0x401EA849 |  2009161 | About the Revered Spirit Ash Blessing         | About * SOTE  | yes |

Sub-category is assigned automatically by `classifyInfoItem` in
`info_subcat.go`: `"About *"` and `"Note: *"` entries land in
`Mechanics / Locations Info`; the remaining two (`Weathered Map`,
`Sellia's Secret`) fall to `Letters / Maps / Paintings`.

Explicitly NOT added in this commit (verified by the new tests):

- 15 Set B duplicate Notes in `0x4000222E–0x4000223F`. They duplicate
  Set A names already in `info.go` and have unfinished regulation
  params (`goodsType=0`, `iconId=0`, `sortId=999999`); the omission
  was already documented in `info.go`.
- The real shipped `Note: Sealed Spiritsprings` (`0x401EA3DF`). The
  current `info.go` entry `0x401EA443` is the broken Set-B-equivalent
  flagged `cut_content/ban_risk`; replacing it is deferred to a
  separate canonical-correction commit.

See `tmp/item-audit/phase2b3_info_reclassification.csv` and
`tmp/item-audit/phase2b3_info_reclassification_summary.md` for the
reclassification trail (15 entries demoted from `real_missing_add` to
`duplicate_variant_ignore`, 1 SOTE flagged `needs_manual_decision`).

### fix(db): add missing arrows and bolts

Phase 2B.2 of the item database audit adds four base-game elemental
projectiles that were absent from `backend/db/data/arrows_and_bolts.go`,
all classified as `real_missing_add` in
`tmp/item-audit/phase2_classification_missing.csv`.

| ID (hex)   | ID (dec) | Name                | Sub-category | wepType |
|------------|----------|---------------------|--------------|--------:|
| 0x02FB1790 | 50010000 | Fire Arrow          | Arrows       | 81      |
| 0x030AA7F0 | 51030000 | Golem's Magic Arrow | Greatarrows  | 83      |
| 0x03199C10 | 52010000 | Lightning Bolt      | Bolts        | 85      |
| 0x0328DE50 | 53010000 | Lightning Greatbolt | Greatbolts   | 86      |

All four are base-game (non-DLC), `MaxUpgrade=0`, `Flags=["stackable"]`.
Stack caps follow the existing sub-category convention:

- Arrows / Bolts: `MaxInventory=99`, `MaxStorage=600`
- Greatarrows: `MaxInventory=30`, `MaxStorage=600`
- Greatbolts (Ballista Bolt class, wepType=86): `MaxInventory=20`,
  `MaxStorage=600` (matching `Ballista Bolt` and `Bone Ballista Bolt`)

Names and IDs verified against `tmp/regulation-bin-dump/csv/EquipParamWeapon.csv`
and `tmp/regulation-bin-dump/msg/name_mapping.csv` (FMG=`WeaponName.fmg`).

Regression test: `backend/db/data/phase2b2_arrows_bolts_test.go` asserts each
entry's Name/Category/SubCategory/MaxInventory/MaxStorage/MaxUpgrade/flags
and verifies that stack sizes match the canonical sibling in the same
sub-category.

### fix(db): add missing base and SOTE weapons and correct Beast Claw item ID

Phase 2B.1 of the item database audit (`tmp/item-audit/phase2_classification_*`)
addresses two issues:

1. **Beast Claw ID collision (paired fix).** `backend/db/data/incantations.go`
   carried an erroneous entry at `0x04153A20` named "Beast Claw" — but that ID
   belongs to the SOTE Beast Claw **weapon** (`EquipParamWeapon` row
   68500000, wepType=95). The real Beast Claw incantation lives at Goods row
   6820 = `0x40001AA4` and is untouched. The bogus incantation entry was
   removed; the weapon entry was added to `melee_armaments.go`.

2. **Five missing weapons.** Items present in regulation but absent from the
   DB were added to `backend/db/data/melee_armaments.go`:

   | ID (hex)     | ID (dec)  | Name                       | Subcat                | MaxUpgrade | DLC |
   |--------------|-----------|----------------------------|-----------------------|-----------:|:---:|
   | 0x002F4D60   | 3100000   | Sacred Relic Sword         | Greatswords           | 10 (somber)| —   |
   | 0x005BDBA0   | 6020000   | Great Épée                 | Heavy Thrusting Swords| 25         | —   |
   | 0x01038D50   | 17010000  | Mohgwyn's Sacred Spear     | Great Spears          | 10 (somber)| —   |
   | 0x00170A70   | 1510000   | Fire Knight's Shortsword   | Daggers               | 25         | dlc |
   | 0x0044F840   | 4520000   | Fire Knight's Greatsword   | Colossal Swords       | 25         | dlc |
   | 0x04153A20   | 68500000  | Beast Claw (SOTE weapon)   | Beast Claws           | 25         | dlc |

   Each entry sets `SubCategory` explicitly because `classifyMelee` in
   `melee_subcat.go` strips infusion prefixes ("Sacred ", "Fire ") destructively,
   which would otherwise mis-classify these names. Data was verified against
   `tmp/regulation-bin-dump/csv/EquipParamWeapon.csv` (wepType, reinforceTypeId)
   and `tmp/regulation-bin-dump/msg/name_mapping.csv` (FMG names).

   Regression test: `backend/db/data/phase2b1_weapons_test.go` asserts the six
   entries exist in `Weapons` with the expected Name/Category/SubCategory/
   MaxUpgrade/DLC flag, and that `0x04153A20` is absent from `Incantations`
   while `0x40001AA4` (the real incantation) is present.

### fix(db): mark 45 somber weapons, catalysts, bows and shields as max upgrade 10

Audit (`tmp/item-audit/`) cross-referenced every entry in
`backend/db/data/{melee_armaments,ranged_and_catalysts,shields}.go` against
`tmp/regulation-bin-dump/csv/EquipParamWeapon.csv` joined with
`tmp/regulation-bin-dump/csv/ReinforceParamWeapon.csv` and surfaced 45 items
flagged as Icon-Shield-bug analogues: the DB carried MaxUpgrade=25 while
regulation derives a somber +10 path (reinforceTypeId ∈ {2200, 2400, 3200,
3300, 8300, 8500}, gemMountType=0). `AddItemsToCharacter` was routing them
through the standard infusable branch, so the editor could fabricate ItemIDs
for affinity variants (`base+100..+1200`) or upgrade levels above +10 — none
of which exist in EquipParamWeapon, so the items vanished after the save
was loaded in-game (same failure mode as Icon Shield in v0.7).

Each entry's MaxUpgrade was changed from 25 → 10. Name, Category, ID and
all other fields are untouched.

Shields (5, `backend/db/data/shields.go`):

- Shield of Night            0x0148D3B0 / 21550000 (SOTE, reinforceTypeId=8500)
- Coil Shield                0x01CCD0C0 / 30200000 (reinforceTypeId=8500)
- Silver Mirrorshield        0x01D9F020 / 31060000 (reinforceTypeId=8300)
- Golden Lion Shield         0x01E11C10 / 31530000 (SOTE, reinforceTypeId=8300)
- Lamenting Visage           0x0175FE30 / 24510000 (SOTE, reinforceTypeId=2200)

Ranged / catalysts (24, `backend/db/data/ranged_and_catalysts.go`):

- Bows / Greatbows: Harp Bow, Erdtree Bow, Serpent Bow, Pulley Bow,
  Black Bow, Ansbach's Longbow (SOTE), Lion Greatbow, Golem Greatbow,
  Erdtree Greatbow (all reinforceTypeId=2200).
- Crossbows / Hand Ballista: Pulley Crossbow (rId=3300), Full Moon Crossbow,
  Repeating Crossbow (SOTE), Crepus's Black-Key Crossbow, Jar Cannon
  (rId=3200).
- Glintstone staves: Rotten Staff (rId=2200), Crystal Staff,
  Carian Regal Scepter, Azur's Glintstone Staff, Lusat's Glintstone Staff,
  Rotten Crystal Staff, Staff of the Great Beyond (SOTE) (rId=2400).
- Sacred Seals: Golden Order Seal, Erdtree Seal, Dragon Communion Seal
  (rId=2400).

Melee armaments (16, `backend/db/data/melee_armaments.go`):

- Stone-Sheathed Sword (SOTE), Spirit Sword (SOTE), Varr's Bouquet,
  Serpent Flail (SOTE), Stormhawk Axe, Bonny Butchering Knife (SOTE),
  Spirit Glaive (SOTE), Tooth Whip (SOTE), Poisoned Hand (SOTE),
  Madding Hand (SOTE), Deadly Poison Perfume Bottle (SOTE),
  Barbed Staff-Spear (SOTE), Scepter of the All-Knowing,
  Devourer's Scepter, Watchdog's Staff, Staff of the Avatar
  (all reinforceTypeId=2200).

After the fix, the frontend renders the +0..+10 slider and hides the
infusion selector (`item.maxUpgrade === 25` gate) for these items, and the
backend's `MaxUpgrade==10` branch ignores `infuseOffset` entirely.

Tests:
- `backend/db/data/weapons_somber_max_upgrade_test.go` — table-driven DB
  guard for all 45 items (asserts presence, name, category and
  MaxUpgrade=10) plus `TestStandardControls_StayAt25` regression for six
  controls (Longsword, Composite Bow, Longbow, Light Crossbow,
  Academy Glintstone Staff, Finger Seal) that must keep MaxUpgrade=25.
- The existing `backend/db/data/shields_somber_test.go` and
  `app_somber_greatshields_test.go` (nine v0.7 greatshields) remain green.

### chore(inventory): hide deprecated Weapon Edit tab and rename Sort Order to Weapons & Sort Order

The Inventory → Weapon Edit pill is removed from the tab list in
`frontend/src/App.tsx`. The `WeaponEditTab` component, its import, the
`'weapon_edit'` variant in the `invView` type union, and the corresponding
render branch are intentionally left in place as a legacy/unreachable path so
that the change is fully reversible. Soft `TODO(deprecated)` comments mark
the removed pill location and the unreachable render branch.

Backend endpoints remain untouched. `App.ApplyWeaponInfusion`,
`App.ApplyWeaponAoWStrict`, `App.GetAoWAvailability`, and
`core.PatchWeaponAoWHandle` continue to power the weapon editor modal used
by Sort Order (`WeaponEditModal.tsx`), which is unaffected by this change.

`App.ApplyWeaponAoW` (non-strict) and `core.PatchWeaponAoW` are now reached
only from the hidden tab; both gained a soft `NOTE` comment but are not
marked `Deprecated:` and their Go tests
(`app_weapon_aow_editor_test.go`, `app_weapon_aow_dlc_test.go`,
`backend/core/...`) remain green.

The remaining Sort Order pill is renamed in the UI to
"Weapons & Sort Order" to better reflect that the per-tile weapon editor
modal is reachable from this view. The pill `id` (`sort_order`) and the
component (`SortOrderTab`) are unchanged.

### fix(db): mark somber greatshields as max upgrade 10

Nine somber greatshields (reinforceTypeId=8300, gemMountType=0 in
regulation.bin → EquipParamWeapon) had MaxUpgrade=25 in
`backend/db/data/shields.go`, which let `AddItemsToCharacter` route them
through the standard infusable branch — applying the infuseOffset selector or
an upgrade level above +10 produced ItemIDs absent from EquipParamWeapon, so
the items vanished after the save was loaded in-game.

Affected items (hex / decimal):

- Crucible Hornshield     0x01E8BD30 / 32030000
- Dragonclaw Shield       0x01E8E440 / 32040000
- Erdtree Greatshield     0x01E98080 / 32080000
- Jellyfish Shield        0x01EA1CC0 / 32120000
- Icon Shield             0x01EA6AE0 / 32140000
- One-Eyed Shield         0x01EA91F0 / 32150000
- Visage Shield           0x01EAB900 / 32160000
- Ant's Skull Plate       0x01EBA360 / 32220000
- Verdigris Greatshield   0x01F03740 / 32520000 (DLC)

After the fix the frontend renders the +0..+10 slider, hides the infusion
selector (`item.maxUpgrade === 25` gate), and the backend's MaxUpgrade==10
branch ignores infuseOffset entirely.

Tests:
- `backend/db/data/shields_somber_test.go` — DB guard for the nine somber
  greatshields plus regressions for Fingerprint Stone Shield and Dragon
  Towershield as standard-shield controls (both must keep MaxUpgrade=25).
- `app_somber_greatshields_test.go` — integration through
  `AddItemsToCharacter` (loads `tmp/save/ER0000.sl2`, skips otherwise):
  Icon Shield default writes the base ID, infuseOffset=100 is ignored,
  upgrade10=10 writes baseID+10, plus table-driven coverage across all nine
  shields for both invariants.

### fix(aow-compat): Fist weapons use the Knuckle compatibility bit

Fixed Ash of War compatibility for Fist weapons such as Star Fist. The
Fist/Knuckle weapon type now maps to the correct `canMountWep_Knuckle` bit, so
compatible Ashes of War such as Lifesteal Fist, Cragblade, Endure, Quickstep,
and Bloodhound's Step are shown and accepted by the backend.

### fix(add-items): detect and repair duplicate inventory acquisition indices

Add Items now detects pre-existing duplicate acquisition indices before
mutating a save and blocks the operation with a clear repair prompt instead of
failing after rollback. Added a repair flow that renumbers only duplicate
acquisition/sort indices, preserves item IDs, handles, quantities, and
containers, supports undo, and retries the original Add Items operation after
user confirmation.

- Added read-only duplicate index scanning for Inventory CommonItems/KeyItems.
- Added `RepairDuplicateInventoryIndices` core helper and App/Wails endpoint.
- Added Database tab "Repair & Retry" prompt for duplicate acquisition index errors.
- Repair is opt-in only: no auto-repair on load, save, upload, or background operations.

### feat(sort-order): per-tile weapon edit modal

Sort Order → Weapons now supports editing a weapon directly from its grid tile through a red edit icon in the top-left corner. The modal works for both Inventory and Storage weapons and preserves selection, drag/drop behavior, pending preview order, and Apply Order state.

- Added upgrade level editing with a new `App.ApplyWeaponUpgradeLevel` backend endpoint. The endpoint validates that only the upgrade level changes: same base weapon, same infusion offset, and level within `MaxUpgrade`.
- Added infusion / affinity editing through the existing `ApplyWeaponInfusion` flow. Level is preserved across infusion changes.
- Added strict Ash of War editing through `ApplyWeaponAoWStrict`, including search, availability badges, compatibility status, and Remove AoW support.
- AoW editing is strict-only: the modal requires a free AoW copy in the save and does not auto-create new AoW copies.
- AoW compatibility is fail-closed for unknown data in this modal. Unknown / unmapped DLC compatibility is shown only when unavailable/incompatible items are visible and cannot be applied until the full AoW compatibility API lands.
- `InventoryOrderItem` now exposes `MaxUpgrade`, allowing the modal to render `+0..+N` from authoritative DB data.
- Added backend unit coverage for weapon upgrade level changes and validated the full modal flow with frontend typecheck/build and manual in-game testing.

Known limitations:
- `WEP_TYPE_TO_BIT` is still duplicated between `WeaponEditTab.tsx` and `WeaponEditModal.tsx`; future refactor should move it to a shared frontend helper.
- The modal currently uses strict AoW assignment only; auto-allocate AoW can be added later as a separate confirmation flow.
- Full DLC AoW compatibility depends on integrating the `research/aow-weapon-compatibility` branch.

### feat(sort-order): dual-grid Inventory + Storage with bidirectional transfer

Sort Order tab is now a dual-grid editor: Storage on the left, Inventory on the right,
per-category tabs (weapons, talismans, head, chest, arms, legs). Both sides drag/drop,
sort, and apply order independently.

Backend:
- `App.MoveItemsBetweenInventoryAndStorage(charIdx, handles, direction)` — App-level
  wrapper around `core.MoveItemsBetweenContainers`; resolves `MaxInventory` / `MaxStorage`
  caps from the DB, pushes undo, returns `core.TransferResult{Moved, Skipped[]}`.
- `core.MoveItemsBetweenContainers` — instance-move for weapon/armor/accessory/AoW
  (handles preserved, new monotonic Index on the destination); quantity-merge for goods
  (cap-aware partial moves with `movedQty` / `remainingQty`); duplicate-instance rehandle:
  if the destination already has the same handle for an instance item, a new globally
  unique handle + new GaItem entry + GaMap mapping are allocated so the same talisman /
  weapon / armor can live in both Storage and Inventory; equipped-item guard rejects
  Inventory → Storage moves with `SkipReasonEquipped`; defensive header reconcile and
  rebuild of `slot.Storage.CommonItems` after the batch.
- `App.GetStorageOrder(charIdx, tab)` — reads `slot.Data[StorageBoxOffset + StorageHeaderSkip…]`
  and sorts by the in-record `Index` ascending (the true storage acquisition order, not
  binary position).
- `App.ReorderStorage(charIdx, tab, orderedHandles)` in `app_inventory_order.go` — mirrors
  `ReorderInventory` on `slot.Storage.CommonItems`: complete-list / no-duplicate /
  category / technical-placeholder validation, stride-2 index assignment with an even
  base above `InvEquipReservedMax` (same `acqIdx >> 1` bucketing applies in storage),
  monotonic counter advance, defensive `ReconcileStorageHeader`. Inventory bytes and
  counters are untouched.
- VM fix: rehandled destination instances are visible in `mapItems` because itemID is
  resolved via `GaMap` first with `HandleToItemID` fallback.

Frontend (`SortOrderTab.tsx`):
- Per-side preview / base / `hasChanges` state; per-side `Reset Preview` + `Apply Order`
  buttons; per-side sort dropdown (Acquisition ↑/↓, Weight ↑/↓, Type ↑/↓) that flips
  to `Custom` on manual drag.
- Drag/drop transfer: drag a selected item moves the whole source-side selection batch
  to the opposite frame; drag an unselected item moves only that item. Source-side
  selection is cleared after a successful transfer.
- Guards prevent loss of an unsaved preview:
  - Cross-transfer blocked when either side `hasChanges` →
    `"Apply or reset the current order before transferring items."`
  - Inventory Apply blocked when Storage has unsaved changes →
    `"Apply or reset Storage order before applying Inventory order."`
  - Storage Apply blocked when Inventory has unsaved changes →
    `"Apply or reset Inventory order before applying Storage order."`
- "Dragging N items" banner moves to whichever frame is the source of the active
  multi-drag.

Tests:
- `tests/transfer_test.go` covers instance-move, quantity-merge + partial caps,
  equipped guard, duplicate-handle rehandle (talisman / weapon / armor), goods merge,
  VM visibility of rehandled instances, `TestStorageOrderUsesRecordIndex`, write+reload
  round-trip.
- `app_storage_order_test.go` (new) covers `ReorderStorage` rejection paths
  (missing/duplicate/incomplete handle, handle from inventory), persistence of the new
  order, no mutation of inventory bytes / counters, re-scan after reorder, and the
  reverse invariant — `ReorderInventory` does not touch any storage byte.

Specs:
- `spec/53-inventory-storage-transfer.md` (EN + PL) — design doc covering the dual-grid
  layout, handle-prefix data model, transfer semantics + rehandle path, frontend guards,
  ordering rules, test coverage, and known limitations.
- `spec/README.md` + `spec/lang-pl/README.md` — spec/53 entry added.
- `docs/ROADMAP.md` — sort order moved from Planned to Done (v0.8.x section), Planned
  list reduced to remaining in-game verification work.

### feat(weapon-edit): apply weapon infusion / affinity in-place

Weapon Edit tab can now save infusion (affinity) changes for infusable weapons (maxUpgrade == 25).

Backend:
- `PatchWeaponItemID` in `backend/core/writer.go` — in-place patch of a single GaItem's
  ItemID in `slot.Data` (4 bytes, no RebuildSlotFull needed — weapon record size stays 21 B).
  Updates `slot.GaItems[i].ItemID`, `slot.GaMap[handle]`, calls `upsertGaItemData(newItemID)`.
  Old GaItemData entry is intentionally left (game tolerates extra entries, same as item removal).
- `ApplyWeaponInfusion(charIdx, handle, expectedCurrentItemID, newItemID)` in `app.go` —
  backend-side validation: weapon IDs, same base/upgrade, valid infusion offset, maxUpgrade==25,
  categorySupportsInfusion, stale-data guard via expectedCurrentItemID.

Frontend (`WeaponEditTab.tsx`):
- `OwnedWeapon` now has `handle: number` and `location: 'inventory' | 'storage'` instead of
  `inInventory/inStorage`. Merge key changed from `baseId` to `handle` — each weapon instance
  (one GaItem) is a separate list entry, preventing accidental multi-copy edits.
- `locationFilter` and location badges updated to use `location` field.
- Apply Changes enabled only for infusion-only changes (`canApply` guard).
- `handleApply()` calls `ApplyWeaponInfusion`, shows toast, triggers re-fetch via `localVersion`.
- Apply button has loading spinner, dynamic tooltip, green active state.
- Bottom bar shows contextual status message for each pending-change scenario.

### fix(items): sync spectral steed whistle companion flags on add and remove

When adding `Spectral Steed Whistle` (`0x40000082`) the editor sets companion EventFlags
(`60100`, `4680`, `4681`, `710520`) that the game co-sets during normal Melina acquisition.
When removing the item (and no other instance remains in the slot), the same flags are cleared.

- SET fires even when the item is already in inventory — repairs saves missing the flags.
- CLEAR fires only when the last instance is removed (checked via `GaItems` scan).
- Roundtable Hold flags (`10009655`, `11109658`, `11109659`) are not touched.
- Transient flags (`4698`, `4651–4653`, `4656`, `11109786`) are never set or cleared.

Implementation: `CompanionEventFlagsForItem()` in `backend/db/data/item_companion_flags.go`,
SET hook in `AddItemsToCharacter()`, CLEAR hook in `RemoveItemsFromCharacter()` (`app.go`).
Design doc: `spec/50-item-companion-flags.md`.

### fix(core): PS4 crash on network preset — ZSTD rawblock patch

Root cause: `PatchNetworkParams` was calling `compressDCX` (klauspost ZSTD encoder) to
recompress the regulation.bin frame after patching. klauspost produces FHD=0x84 (content
size + checksum), window=8 MB — vs FromSoftware's FHD=0x00, window=64 MB. The different
plaintext produces different ciphertext; PS4 rejects the file → title-screen crash.

Fix: added `patchZSTDStreamRawBlock` in `backend/core/regulation.go`. For PS4 saves
(detected by `regStart == ud11UnkSize = 0x10`, no MD5 prefix), only the ZSTD block(s)
covering NetworkParam fields are replaced with Raw blocks (Block_Type=0); all other blocks
remain byte-for-byte identical to the original. Treeless_Literals successor blocks are also
replaced to prevent Huffman tree dependency errors.

PC path unchanged: full `compressDCX` + MD5 recalculation.

- `backend/core/regulation.go`: added `patchZSTDStreamRawBlock`, `walkZSTDBlocks`,
  `makeRawBlockHeader`; `PatchNetworkParams` dispatches on `regStart`
- `tests/regulation_test.go`: fixed pre-existing test expectation mismatches (preset values
  had been changed after tests were written: FastSummons, FastBlue, AggressiveHost)
- `spec/49-ps4-zstd-rawblock-patch.md` (EN + PL): new design doc
- `spec/README.md` + `spec/lang-pl/README.md`: spec/49 entry added

### feat(pvp): add network speed presets

- New component `frontend/src/components/NetworkSpeedPanel.tsx` — collapsed accordion in
  PvP Preparation tab with 3 invasion-speed presets: Vanilla, Light/Safer, Fast Invasions
- Presets patch UD11 NetworkParam; global for the whole save; requires character reload to activate
- Warning text: aggressive settings may increase online enforcement risk; activation instructions included
- New `backend/core/regulation.go`: `NetworkParamLightInvasions()` — targets=8, interval=10s, timeout=8s
- `app.go` `GetNetworkPreset()`: added `"light-invasions"` case
- No new app.go exported methods — uses existing `GetNetworkPreset`, `SetNetworkParams`, `ResetNetworkParams`
- `spec/48` (EN+PL): Phase 4 marked complete, §8.1 Network Speed Panel section added
- `docs/CHANGELOG.md`: removed attribution to third-party names; replaced with neutral description

### docs(faster-invasions): confirm UD11 reload activation model

- `spec/46-faster-invasions-research.md` (EN + PL): replaced all "runtime effect unconfirmed"
  claims for UD11 NetworkParam patch with confirmed verdict (console-tested PS4/PS5, 2026-05-09)
- Added §11 subsections: Confirmed Activation Procedure, Track A vs Track B, NetworkParam Presets
- Track A (UD10 timer patching) confirmed INEFFECTIVE — 287,912-byte runtime rebuild on session init
- Track B (UD11 NetworkParam) confirmed EFFECTIVE after character reload; server-reverts on connect
- Added EAC ban risk warning to §11
- Three presets documented: Vanilla (30/20/5), Light (10/10/5), Fast Invasions (4/4/10)
- `spec/48-pvp-ready-modular-presets.md` (EN + PL): updated Module F warning text, product
  direction note, and sources table to reflect confirmed verdict

### fix(core): recalculate UD11 MD5 prefix after patching regulation.bin

- `backend/core/regulation.go`: `PatchNetworkParams` now recalculates `ud11[0:0x10]`
  (MD5 of `ud11[0x10:]`) after re-encrypting the DCX payload
- Root cause: PC saves carry a 16-byte MD5 prefix inside `UserData11`; the game validates
  it on load and rejects (reverts to its own game files) when it doesn't match — causing
  all patched NetworkParam values to reset to vanilla on the next game session
- PS4 saves are unaffected (no MD5 prefix in PS4 UD11 layout, `regStart == 0x10`)
- `tests/regulation_test.go` (`TestPatchNetworkParams_PC_RoundTrip`): added MD5 prefix
  assertion to prevent regression
- Finding documented in `spec/46-faster-invasions-research.md`

### fix(db): summoning pools broken since patch v1.12 — update all IDs to 670xxx range

- `backend/db/data/summoning_pools.go`: replaced all pre-v1.12 flag IDs (`10000040`,
  `1035530040`, etc.) with current `670xxx` IDs from CT-TGA "Unlock all Summoning Pools.cea"
- Base game: 157 unique pool entries across Legacy Dungeons, Catacombs, Caves, Tunnels,
  Divine Towers, Subterranean, Ruin-Strewn Precipice and Open World areas
- DLC (Shadow of the Erdtree): 55 entries added (Belurat, Enir-Ilim, Shadow Keep,
  Specimen Storehouse, Gaols, Caves, Catacombs, Open World Land of Shadow)
- Total: 213 pools (up from 165 — DLC coverage expanded)
- All IDs resolve via BST block 670 (position 107) — no changes to `event_flags.go` or
  `eventflag_bst.txt` needed
- Added `IsDLCSummoningPool(id uint32) bool` helper (threshold: `>= 670800`)
- Removed stale comment about "Large flag IDs requiring lookup table entries"
- Root cause: CT-TGA confirmed patch v1.12 migrated all pools from legacy block format
  to unified 670xxx namespace; confirmed by community RE analysis (March 2026)
- `spec/42-summoning-pools-bug.md`: status updated to Fixed

### refactor(world): remove Networking sub-tab from World tab

- Removed the `networking` sub-tab from `WorldTab.tsx` — NetworkParam editing is server-side
  and not achievable via save file on PS4/PS5; PC-only offline editing carries ban risk
- Removed `SliderDef`, `NetSection`, `NET_SECTIONS`, `netParamsToDict` and all networking
  state/handlers (`netDraft`, `netDirty`, `loadNetParams`, `handleNetApply`, etc.)
- Backend code (`app.go`, `backend/core/regulation.go`) intentionally preserved for reference

### feat(db): add 27 missing matchmaking regions (77 → 104 total)

- `backend/db/data/regions.go`: added Legacy Dungeon interior IDs from er-save-manager list
  - Stormveil Castle: 1000001, 1000003, 1000005, 1000006
  - Leyndell Royal Capital interior: 1100001, 1100010, 1100012, 1100013, 1100015, 1100016, 1100017
  - Leyndell Ashen Capital (new section): 1105000, 1105001, 1105011, 1105092
  - Raya Lucaria Academy interior: 1400010, 1400011, 1400013, 1400015
  - Volcano Manor interior: 1600006, 1600010, 1600012, 1600014, 1600016, 1600020, 1600022
  - Tutorial/Endgame: 1800090 (Cave of Knowledge), 1900001 (Elden Beast)
- Ban risk for all 27 new IDs: LOW — sourced from er-save-manager verified matchmaking list
- `spec/11-regions.md`: updated count from ~211 to 104; added Cave of Knowledge note

### docs(spec): add spec/46 faster invasions research (EN + PL)

- `spec/46-faster-invasions-research.md`: comprehensive negative-result investigation
  - UD10 scan: only `perform_matchmaking` at UD10[0x0013] is actionable; NetworkParam values
    found in volatile regions reset to 0.0 on re-save
  - NetMan: `list1_type=5` is invasion target history cache, not configuration
  - EventFlags: relevant only for Varré questline items; cannot control timing
  - regulation.bin: NetworkParam entirely server-side; PS4/PS5 always overwrites from server
  - DS3 comparison: Wex Dust mod (DLL injection), Spam Red Eye Orb; same architecture,
    same conclusion
  - Unlocked regions coverage: 77 → 104 IDs documented; ban risk: LOW for all
- `spec/lang-pl/46-faster-invasions-research.md`: Polish translation
- `spec/README.md` and `spec/lang-pl/README.md`: index entries added

### feat(character): Soul Memory tracking, dirty guard, add-settings persistence

- `SoulMemory` (PGD+0x68) added to `PlayerGameData`, read/written in `SyncPlayerToData()`; exposed via `CharacterViewModel.soulMemory`
- `ApplyVMToParsedSlot` auto-floors `SoulMemory` to `runesCostForLevel(level)` on every save — prevents detectable mismatch when level is edited
- `runesCostForLevel(n)` formula: `Σ max(0, ⌊0.02n³ + 3.06n² + 105.6n − 895⌋)` for n=2..level, clamped to uint32 max
- Character → Profile: "Soul Memory" field with ✓/✗ consistency badge; Fix button (+10% buffer) when inconsistent
- `handleSave` now reloads from backend after save, so auto-corrected Soul Memory is reflected immediately in UI
- `CharacterTab` kept always-mounted (CSS hide) — unsaved stat edits no longer lost on tab switch
- `isDirty` ref guard prevents `refreshKey` from wiping unsaved edits when inventory changes in another tab
- All user edit handlers (`updateStat`, `handleClassChange`, all inline onChange) set `isDirty = true`; save clears it
- `charAddSettings` persisted to `localStorage` — Add Settings survive app restart

### fix(core): SaveCharacter changes reverted after AddItemsToCharacter

- `SaveCharacter` called `ApplyVMToParsedSlot` (writes to `slot.Player`) but never called `SyncPlayerToData()` (writes `slot.Player` → `slot.Data` binary)
- `AddItemsToSlotBatch` triggers `RebuildSlotFull` + `parseFromData()` which re-reads `slot.Data` → `mapStats()` → overwrites `slot.Player` with pre-edit binary values
- Fix: `SaveCharacter` now calls `slot.SyncPlayerToData()` immediately after all struct-level mutations, before returning — ensures `slot.Data` is always consistent with `slot.Player`

### fix(world): Gestures and Cookbooks loaded in wrong sub-tab

- `GetCookbooks` and `GetGestures` were called in `loadProgressData()` (triggered on 'progress' sub-tab) but rendered on 'unlocks' sub-tab — items always appeared empty, Unlock All had no effect
- Moved both calls to `loadUnlocksData()` where they belong

### feat(ui): Starting Class editable dropdown with attr clamp and level recalc

- Starting Class field in Character → Profile changed from read-only display to a `<select>` dropdown listing all 10 classes
- On class change: each attribute is clamped up to the new class's minimum (never lowered); level is recalculated as `Σ(attrs) − 79` (formula holds universally across all classes)
- Backend: `ApplyVMToParsedSlot` now writes `data.Class = vm.Class` — class was previously read-only
- Character Name and Starting Class fields swapped in the Profile grid

### fix(db): World Map items hidden from Item Database

- All 24 `Map: *` entries in `backend/db/data/key_items.go` flagged with `"no_database"` — same mechanism as Cookbooks and Bell Bearings
- Maps are managed via World → Exploration → Map; surfacing them in the Item Database was redundant

### fix(ui): Invasion Regions risk banner and updated description

- `RiskSectionBanner` added to the Invasion Regions accordion section in World → Unlocks
- `bulk_region_unlock` risk entry updated: explicitly warns about Roundtable Hold and Stranded Graveyard as non-invadeable areas whose region flag creates an anomalous save state

### fix(ui): minimum font size raised to 11px across WorldTab, CharacterTab, AccordionSection

- All `text-[7px]`, `text-[8px]`, and `text-[9px]` occurrences replaced with `text-[11px]` in WorldTab.tsx, CharacterTab.tsx, and AccordionSection.tsx
- Networking disclaimer text raised to `text-[11px]` to match Apply button

### fix(add): infuse offset not applied to ranged weapons and catalysts

Bows, crossbows, greatbows, staves, and seals cannot be infused in Elden Ring. Previously, `AddItemsToCharacter` applied `infuseOffset` unconditionally to all items with `MaxUpgrade == 25`, including `ranged_and_catalysts`. This produced ItemIDs the game does not recognise (e.g. "Heavy Bone Bow +4" → `0x0269FB88`), making the items invisible in-game after loading the save.

**Root cause** — `app.go::AddItemsToCharacter`: the `MaxUpgrade == 25` branch applied `id + infuseOffset + upgrade25` regardless of whether the weapon category supports infusion.

**Fix** — new helper `weaponCategorySupportsInfusion(category string) bool` returns `true` only for `melee_armaments` and `shields`. `infuseOffset` is now skipped for all other categories (`ranged_and_catalysts` and anything else). The `upgrade25` level is still applied correctly. Melee weapons and shields are unaffected.

Confirmed against `ER0000-kro55-out.sl2`: Bone Bow, Golem Greatbow, Greatbow were added as `base+104` (infuseOffset 100 Heavy + upgrade 4) instead of `base+4`. Items with `MaxUpgrade == 10` (Frenzied Flame Seal, Rabbath's Cannon) were unaffected — they use the `upgrade10` branch which never applied `infuseOffset`.

### feat(db): weight column and sorting in Database tab

- Weight column auto-imported from `regulation.bin` CSV (`EquipParamWeapon` + `EquipParamProtector`) via `tmp/scripts/import_weights.go` → `backend/db/data/weights.go` (4252 entries)
- Armor ID mapping fixed: `saveID = 0x10000000 + rowID` (protector param offset)
- Weight lookup restricted to physical categories only (`melee_armaments`, `ranged_and_catalysts`, `shields`, `head`/`chest`/`arms`/`legs`) — prevents false weight on incantations/sorceries sharing ID space
- Weight column hidden automatically when no item in current filtered view has weight
- Sort by weight: items without weight always at top (alphabetically), items with weight sorted numerically with name tie-break
- "Talismans: highest only" toggle now always visible in Add Settings (not gated on talismans category)

### feat(world): Networking moved into World tab as collapsible sub-tab

- Removed standalone Network tab; networking panel is now a 4th sub-tab inside World
- Four role sections (Invader, Summon, Hunter, Host) rendered as individually collapsible accordions, collapsed by default
- Per-section preset buttons (Fast Invasion / Fast Summons / Fast Hunter / Aggressive Host) with green/red status dot indicating whether sliders are at defaults
- Per-section Reset button; one global Apply button
- Slider descriptions hidden by default behind `?` toggle
- Section header: role label left-aligned, buttons right-aligned with ban-risk badge

### fix(ui): light theme contrast improvements

- `--muted-foreground` darkened from `44%` to `28%` lightness — readable at `/70` opacity (≈4.5:1 WCAG AA)
- `--border` darkened from `89%` to `72%` — visible section separators
- `--muted` / `--secondary` / `--accent` darkened from `94%` to `88%` — distinguishable from card background

### Fix — fuzzy weapon lookup for 0x01/0x02 prefix items with byte-carry upgrade offsets

**Problem**: `GetItemDataFuzzy` only fuzzy-searched weapons with prefix `0x00`. Bows, greatbows, crossbows (prefix `0x02`) and staves/seals/catalysts (prefix `0x01`) with upgrade+infusion offsets that crossed a byte boundary (e.g. Greatbow Heavy+25: base `0x02817AC0` + 125 = `0x02817B3D`) had `id & 0xFFFFFF00 ≠ baseID & 0xFFFFFF00`, so the lookup returned empty → items were filtered from editor view as "unknown".

**Fix** — `backend/db/db.go` (`GetItemDataFuzzy`): replaced byte-mask comparison with a range-based check (`id >= baseID && id-baseID <= 1225`) covering prefixes `0x00`, `0x01`, `0x02`. 1225 = max infusion offset (Occult=1200) + max upgrade (25).

### Fix — add-items success log shows combined total across both API calls

**Problem**: for non-stackable items with `invQty > 1` or `storageQty > 1`, the frontend issued two separate `AddItemsToCharacter` calls; `lastResult` was overwritten by the second call, so the log always showed only the storage-call count (e.g. "12/12" instead of "36/36").

**Fix** — `frontend/src/components/DatabaseTab.tsx` (`handleAdd`): added `totalAdded`/`totalRequested` accumulators updated after each call; success message uses the accumulated totals.

### Fix — binary NextEquipIndex write-back on load (visibility gate for externally-edited saves)

**Problem**: saves edited by external tools (e.g. er-save-manager) may have `NextAcquisitionSortId > NextEquipIndex` gap in the binary. Our editor reconciled this gap in memory (`mapInventory`), but never wrote the corrected value back to `slot.Data`. If the user opened such a save and re-saved without adding items, the game still read the stale (low) `NextEquipIndex` from the binary → items with high indices remained invisible in-game.

**Confirmed case**: `ER0000-kro55-out.sl2` — binary had `NextEquipIndex=2295`, `NextAcqSortId=2434` (gap=139). All 64 ranged weapons/catalysts were correctly present in GaItems and inventory, but 139 of them were invisible because `item.Index >= 2295`.

**Fix** — `backend/core/structures.go` (`mapInventory`):

- When `NextEquipIndex < NextAcquisitionSortId` is detected, the corrected value is immediately written to `slot.Data` at `nextEquipIndexOff` (the absolute byte offset recorded during parse).
- Guard: only fires when `s.Version > 0` (active slot) and `nextEquipIndexOff > 0` (offset was parsed). Empty/inactive slots (`Version=0`, `NextEquipIndex=0, NextAcqSortId=1`) are skipped to avoid corrupting their binary state.
- New exported accessor `EquipInventoryData.NextEquipIndexOff() int` allows tests to identify the corrected bytes.

**Round-trip tests updated** — `tests/roundtrip_test.go` (`recalculatedRegions`):

- Added `NextEquipIndex` bytes (4 bytes at `nextEquipIndexOff`) to the exclusion list for slots where reconciliation was applied. Tests remain meaningful: the intentional correction is expected and excluded; all other bytes must match exactly.

**Test results**: `TestRoundTripPS4`, `TestRoundTripPC`, `TestConversionPS4ToPC`, `TestConversionPCToPS4`, `TestAcquisitionSortIdIncrementFix` — all pass. `go build .` OK.

### Fix — invisible inventory items when adding large batches to saves with NextEquipIndex gap

User-reported: after adding many items in one session, shields, bows, staves and seals were missing in-game. Sometimes UI showed them but game didn't; sometimes they disappeared after switching characters.

**Root cause** — `backend/core/writer.go::addToInventory` (inventory path):

The game treats items with `item.Index >= NextEquipIndex` as invisible. `addToInventory` advanced `NextEquipIndex` by a plain `++` each time an item was added. When `NextAcquisitionSortId > NextEquipIndex` at session start (gap introduced by saves previously edited with external tools, or by the `InvEquipReservedMax` clamp that jumps `NextAcquisitionSortId` to 433 while `NextEquipIndex` may still be lower), the gate never caught up: items assigned `acqIdx >= final NextEquipIndex` stayed invisible permanently.

Diagnostic (all-item bulk-add on `ER0000-kro55-vanilla.sl2`): original save had `NextEquipIndex=405`, `NextAcquisitionSortId=544` (gap=139). After adding 1965 items both counters advanced by the same amount, preserving the gap. Final `NextEquipIndex=2370`, items 1827–1965 got `Index 2370–2508 >= 2370` → 139 invisible. After the fix: gap=0 throughout, `Items with index >= NextEquipIndex (invisible): 0`.

**Fixes** — `backend/core/writer.go` + `structures.go`:

- `addToInventory` (inventory path): replaced unconditional `NextEquipIndex++` with `max(NextEquipIndex, acqIdx) + 1`. Ensures NextEquipIndex is always strictly greater than the item.Index just written, regardless of the starting gap.
- `mapInventory()` (`structures.go`): added in-memory reconciliation `NextEquipIndex = max(NextEquipIndex, NextAcquisitionSortId)` after the existing `NextAcquisitionSortId` reconciliation. Closes the gap before any adds in the current session; binary written back only when `addToInventory` actually fires (per-item write-back unchanged).

**Test results**: all `go test ./backend/...` pass, all 4 round-trip tests (`TestRoundTripPS4`, `TestRoundTripPC`, PS4↔PC conversion) pass. `make build` OK.

### Critical fix — inventory counter and index bugs causing "inventory full" and invisible items

User-reported: "Inventory full" message when transferring from chest to held inventory despite removing items. Missing ranged weapons, seals, magic staves after editor sessions.

**Root causes identified** (four separate bugs, all fixed):

1. **Per-item acquisition index bloat** (`addToInventory` inventory path):
   - Old code: `perItemIndex = max(NextEquipIndex, maxExistingIndex+1)`, then `NextEquipIndex = perItemIndex + 1`
   - On a save where external tools (er-save-manager) had set acquisition indices to 7835+ while `NextEquipIndex` stayed at 1198, our editor's first `addToInventory` jumped `NextEquipIndex` to 15672 for 3 items. Each subsequent session bloated it further.
   - This did NOT immediately break items added in the same session (all had index < updated NextEquipIndex). However, it created a high NextEquipIndex in the binary that confused future loads.

2. **`NextAcquisitionSortId` clobber**:
   - `NextAcquisitionSortId` was set equal to `nextListId + 1` (same as NextEquipIndex), destroying its independent sort-counter semantics. Fixed in a prior session but combined with the index bloat.

3. **`common_item_count` header not updated on add/remove**:
   - At `invStart-4` (= `MagicOffset + InvStartFromMagic - 4`), the game stores a count the runtime uses as the "next insertion index" and for the "inventory full" check (`count == 2688`). Our editor never read or wrote this field.
   - Without decrement on remove: game thinks inventory is fuller than it is after removals.
   - Without increment on add: game inserts at wrong slot position on in-game add, potentially overwriting items we placed.

4. **Orphaned GaItem binary records** (root cause of accumulating "invisible" items):
   - `RemoveItemFromSlot` zeroed the 12-byte inventory slot but left the GaItem binary record (21/16/8 bytes in the GaItems section) intact.
   - On next load, `scanGaItems()` re-read the binary → orphaned handle reappeared in `GaMap` → next `AddItemsToSlot` → `allocateGaItem` placed new items at increasingly high indices in `slot.GaItems`, competing with orphaned entries.
   - Diagnostic on user's save: 786 GaItem records, 280 orphaned (in GaMap but not in inventory/storage): 142 weapons + 94 armor + 44 AoW. These occupied `slot.GaItems` capacity without being accessible to the player.

**Fixes** — `backend/core/writer.go` + `structures.go`:

- `addToInventory` (inventory path): per-item `Index = NextAcquisitionSortId` (before increment), `NextEquipIndex++` (simple +1 per item), `common_item_count` incremented at `invStart-4`.
- `mapInventory()` (`structures.go`): reconciles `NextAcquisitionSortId` to be > all existing item indices on every load (handles saves previously edited with high-index tools without per-call scanning). Also calls `ReconcileInventoryHeader` to correct `common_item_count` on load.
- `RemoveItemFromSlot`: decrements `common_item_count` for removed inventory items; clears the GaItem in-memory entry (`slot.GaItems[i]`) so the next `RebuildSlotFull` compacts it out of binary.
- New `ReconcileInventoryHeader(slot)`: sets `common_item_count` to actual non-empty slot count. Called automatically from `mapInventory`. Also exposed for direct repair calls.
- New `RepairOrphanedGaItems(slot) int`: scans `slot.GaItems`, finds records whose handles are absent from both inventory and storage, clears them in-memory and removes from `GaMap`. Returns count for UI feedback.
- New `App.RepairInventoryGaItems(slotIndex int) (int, error)`: Wails endpoint calling `RepairOrphanedGaItems` + `ReconcileInventoryHeader`. Use in InventoryTab / DiagnosticsTab to repair saves corrupted by prior editor versions.

**Test results**: `TestAcquisitionSortIdIncrementFix` now shows `AcqSort 15669→15672 (+3), EquipIdx 1198→1201 (+3)` (EquipIdx increments by exactly N per N items added, AcqSort reconciled past existing high indices). `TestStressAddManyItems` passes (no index collisions). `TestArrowsRustCompatibility` and `TestArrowsAddToInventoryOnly` pass. 4 pre-existing failures unchanged.

### Critical fix — non-stackable items (weapons / armor / AoW) shared a GaItem handle between inventory and storage

User-reported bug: after adding the same Ash of War to inventory and storage via the editor, equipping the AoW on one of two identical shields applied it to BOTH shields simultaneously, and BOTH AoW copies showed "in use" status. Investigation of `tmp/aow-debug/ER0000-aow.sl2` (slot 4 "Random") found 81 non-stackable handles simultaneously present in inventory AND storage, plus 109 inventory/storage entries with `Index >= NextEquipIndex` (invisible in-game — explains "added many items but don't see them").

**Root cause** — `backend/core/writer.go::AddItemsToSlot` and `AddItemsToSlotBatch`:

- Phase 1 allocated **one** GaItem record (one unique handle) per item, regardless of destination(s).
- Phase 3 wrote that handle into both inventory and storage when the user requested both (`InvQty>0` AND `StorageQty>0`).
- For stackable items (`B-`/`A-`prefix goods/talismans) the shared handle is correct — handles are deterministic from item ID and the game treats them as fungible stacks.
- For **non-stackable** items (`0x80…` weapon, `0x90…` armor, `0xC0…` AoW) the handle uniquely identifies a physical item in the GaItems array. Sharing it caused the game to treat both list entries as the same backing object: equip-AoW write updated the GaItem, both list copies reflected it, and on the next save cycle the game pushed the duplicate copies to invalid `Index` values, hiding them from the in-game inventory UI.

**Fix** — `backend/core/writer.go`:

- Both `AddItemsToSlot` and `AddItemsToSlotBatch` Phase 1 now allocate a **separate** GaItem record per destination for non-stackable items: one for inventory and one for storage. Stackable / arrow paths unchanged (they correctly share or skip GaItems per existing rules).
- Extracted shared `allocNewGaItem(id, prefix)` helper inside both functions covering `generateUniqueHandle` + `allocateGaItem` + `GaMap` registration + `upsertGaItemData` (when needed).

**Capacity check** — `backend/core/capacity.go::CheckAddCapacity`:

- Non-stackable items with both `InvQty>0` and `StorageQty>0` now correctly cost 2 GaItems (and possibly 2 GaItemData entries when the item ID is new). Previously double-counting was elided based on the (incorrect) shared-handle behavior. Pre-flight feasibility now matches the actual mutation cost so users get accurate "inventory full / GaItems full" warnings before the snapshot/rollback path engages.

**New regression test** — `backend/core/aow_dual_destination_test.go::TestNonStackableDualDestinationUniqueHandles`:

- Loads `tmp/save/ER0000.sl2`, picks an active slot with sufficient empty GaItem capacity, adds Lion's Claw AoW (`0x80002710`) with `invQty=1, storageQty=1` via both `AddItemsToSlotBatch` and `AddItemsToSlot`.
- Asserts: exactly 1 new inventory entry and 1 new storage entry, **handles must differ**, and both new handles are present in the GaItems array as distinct records with the same `ItemID`.

**Updated existing test** — `tests/bulk_add_test.go::TestAddWithInventoryAndStorage`:

- Capped `weaponIDs` / `armorIDs` collection at `min(30, free_armament_zone / 4)` because each non-stackable item now consumes 2 armament-zone GaItem slots; the shared test save's armament zone has only 156 free entries.
- Added explicit assertion that for each non-stackable item ID the inventory-side and storage-side handles are **disjoint** (`sharedHandle == 0`).

**Test results**: full `go test ./backend/...` passes. `tests/` package retains 4 pre-existing failures (`TestMassiveAddAllCategories`, `TestMaxCapacityFill`, `TestBulkAddPerCategory` — armament-zone capacity exhaustion when adding 1000+ non-stackable items to a near-full save; `TestAddArrowsStackable` — `GaItemData` arrow check) all of which were failing identically on master before this change. `make build` produces a clean Wails app bundle.

**Recovery for users with corrupted saves**: saves where this bug already manifested (handle shared between inventory and storage) cannot be auto-repaired by the editor — the second list copy may already have been moved by the game's load logic and assigned an invalid `Index`. Restore from a `*.bkp` file in the save directory made before the dual-destination add, or remove the non-stackable items from inventory and storage in the editor (Inventory tab → Remove) and re-add them with the fixed code.

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
