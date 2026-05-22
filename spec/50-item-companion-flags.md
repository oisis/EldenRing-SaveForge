# 50 — Item Companion Flags

> **Type**: Binary format spec + design doc (canonical chapter)
> **Scope**: Synchronization of event flags with item add / remove in SaveForge — the symmetric SET+CLEAR contract, mapping of 6 items to companion flag sets, the write path in `AddItemsToCharacter` / `RemoveItemsFromCharacter`.

Cross-refs: [11-regions.md](11-regions.md), [14-game-state.md](14-game-state.md), [15-event-flags.md](15-event-flags.md), [16-world-state.md](16-world-state.md), [27-map-reveal.md](27-map-reveal.md), [29-dlc-black-tiles.md](29-dlc-black-tiles.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md), [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md).

---

## 1. Chapter purpose

To define unambiguously:

- what an **item companion flag** is (a set of event flags "added" to the bitfield together with an added item),
- which items are supported in the current code (6 entries as of 2026-05-21),
- how the **symmetric SET+CLEAR** works: SET on add, CLEAR on removal of the last instance,
- how it differs from the asymmetric SET-only contract in `grace_companion_flags` (see [47-site-of-grace-activation.md](47-site-of-grace-activation.md)),
- how it differs from `MapFragmentItems` (a separate mechanism in [27-map-reveal.md](27-map-reveal.md) — **not** part of item companion flags),
- where the implementation ends and `needs verification` begins.

It does not duplicate the event flag helper API (see [15-event-flags.md](15-event-flags.md)) nor the SET-only policy for graces (see [47-site-of-grace-activation.md](47-site-of-grace-activation.md)).

## 2. Status

| Aspect | Status |
|---|---|
| Backend map `itemCompanionEventFlags` | ✅ `backend/db/data/item_companion_flags.go` — 6 entries (Whistle + 5 multiplayer items) |
| API `CompanionEventFlagsForItem(itemID)` | ✅ same file, line 104 |
| Hook SET — `AddItemsToCharacter` | ✅ `app.go:569-578` |
| Hook CLEAR — `RemoveItemsFromCharacter` | ✅ `app.go:706-725` (condition: last instance of the item removed) |
| Repair-mode SET (an item at max quantity also SETs the flags) | ✅ — see §11 |
| Unit tests | ✅ 11 tests in `backend/db/data/item_companion_flags_test.go` |
| Integration tests | ✅ 17 tests in `tests/item_companion_flags_test.go` |
| Runtime verification PS4 | ✅ historically confirmed PS4 test 2026-05-11 (Spectral Steed Whistle) |
| Talisman Pouch sync (60500/60510/60520) | ❌ **not** in `itemCompanionEventFlags` (`needs verification` of historical iterations) |
| `MapFragmentItems` as item companion | ❌ a separate mechanism (see §10) |

## 3. Source of truth in code

| File / symbol | What it contains |
|---|---|
| `backend/db/data/item_companion_flags.go::itemCompanionEventFlags` | Map `itemID → []companionFlag` (6 entries) |
| `backend/db/data/item_companion_flags.go::CompanionEventFlagsForItem` | Lookup API |
| `backend/db/data/item_companion_flags.go` (const block) | 6 `Item...` constants (item IDs) + 9 `EventFlag...` constants (companion flag IDs) |
| `app.go::AddItemsToCharacter` (lines 569-578) | Hook SET — called **per-prepared-item** after `AboutTutorialID` |
| `app.go::RemoveItemsFromCharacter` (lines 658-666 pre-scan, 706-725 CLEAR) | Hook CLEAR — called only when `slot.GaItems` after removal no longer contains the given itemID |
| `backend/db/data/item_companion_flags_test.go` | 11 unit tests (Whistle, Unknown, NoTransientFlags, 4 multiplayer per-item, SmallGoldenEffigy NoForbiddenFlags, Multiplayer NoForbiddenFlags, Multiplayer NoCrossContamination) |
| `tests/item_companion_flags_test.go` | 17 integration tests (SetOnRealSave, ForbiddenAbsent, MechanicFlagPresent, ClearOnFlagData, RemainingItemPreventsClearing, per-item Set/Clear/Repair, NoSummoningPoolFlags, UnrelatedItemDoesNotAffect) |

## 4. Mental model

```
AddItemsToCharacter(prepared)
  for each p in prepared:
    ... (item add logic) ...
    AppendTutorialID(...)
    if companions := CompanionEventFlagsForItem(p.baseID); len(companions) > 0:
        for f in companions:
            SetEventFlag(eflags, f, true)        ← SET-only-on-add hook

RemoveItemsFromCharacter(handles)
  pre-scan: collect itemIDs with companion flags being removed
  ... (item remove logic) ...
  for itemID in companionRemovals:
      if no remaining instance of itemID in slot.GaItems:
          for f in CompanionEventFlagsForItem(itemID):
              SetEventFlag(eflags, f, false)    ← CLEAR-on-last-removed hook
```

The **SET/CLEAR symmetry** is, however, limited — see §9.

## 5. Item companion flag data model

`itemCompanionEventFlags` (`item_companion_flags.go:13`) has the type `map[uint32][]uint32` — the key is the **item baseID** (`0x4000xxxx`, etc.), the value is a slice of **event flag IDs** set together with the item. Currently 6 entries:

| Entry | Item baseID | Companion flag count |
|---|---|---|
| `ItemSpectralSteedWhistle` | `0x40000082` | 4 |
| `ItemSmallGoldenEffigy` | `0x4000006D` | 1 |
| `ItemDuelistsFurledFinger` | `0x40000065` | 1 |
| `ItemSmallRedEffigy` | `0x4000006E` | 1 |
| `ItemWhiteCipherRing` | `0x40000068` | 1 |
| `ItemBlueCipherRing` | `0x40000069` | 1 |

All item IDs and flag IDs are declared as `const` constants in the same file — there are no hardcoded literals in the map. This allows **sharing constants** with other modules (e.g., `EventFlagObtainedSpectralSteedWhistle` is also used by `grace_companion_flags.go` for Gatefront — see [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §8).

`needs verification`: the count of 6 entries is a snapshot from `item_companion_flags.go` as of 2026-05-21. There is no automatic regeneration from the game sources — future entries are added manually. See §13 for the procedure.

## 6. Supported companion mappings

### 6.1 Spectral Steed Whistle — `0x40000082`

| Flag const | ID dec | Code comment |
|---|---|---|
| `EventFlagObtainedSpectralSteedWhistle` | `60100` | Unlocks Torrent. Without it the game refuses to summon Torrent even when whistle is in inventory. |
| `EventFlagMelinaGaveWhistle` | `4680` | Marks Melina quest-give step as complete. Prevents the "accept Torrent?" dialogue from re-triggering at graces. |
| `EventFlagWhistleWorldState` | `710520` | Map/world counterpart set simultaneously with 60100 during the in-game event. |
| `EventFlagMelinaAcceptRefusePopup` | `4681` | Marks accept/refuse popup as shown. Prerequisite before the give step in the Melina quest chain. |

All 4 constants are also used in the `grace_companion_flags.go::GatefrontGraceEventFlagID` companion set — **intentional sharing**. See [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §8.

`needs verification`: the correlation of the 4 flags and the Torrent mechanic is confirmed historically (PS4 runtime test 2026-05-11 — see CHANGELOG). There is no isolated test that checks all 4 flags `=1` after `AddItemsToCharacter` with `ItemSpectralSteedWhistle` (the test `TestCompanionEventFlagsForItem_Whistle` verifies only the *contents* of the slice, not a *roundtrip* on a real slot).

### 6.2 Multiplayer pickup items

Each of the 5 items has **exactly 1** companion flag — its own `Obtained...` flag:

| Item | Item baseID | Companion flag | ID dec | Code comment |
|---|---|---|---|---|
| Small Golden Effigy | `0x4000006D` | `EventFlagObtainedSmallGoldenEffigy` | `60230` | Without it the pickup/interact state at Effigies of the Martyr can remain visible even when the item is already in inventory. |
| Duelist's Furled Finger | `0x40000065` | `EventFlagObtainedDuelistsFurledFinger` | `60240` | Without it the pickup/interact state can remain visible in the world. |
| Small Red Effigy | `0x4000006E` | `EventFlagObtainedSmallRedEffigy` | `60250` | same |
| White Cipher Ring | `0x40000068` | `EventFlagObtainedWhiteCipherRing` | `60280` | Marks item as obtained / shop sold-out. Source: er-save-manager + ShopLineupParam.csv row 101800 `eventFlag_forStock`. |
| Blue Cipher Ring | `0x40000069` | `EventFlagObtainedBlueCipherRing` | `60290` | Confirmed by ShopLineupParam.csv row 101801 + Steam Deck before/after shop purchase diff. |

`needs verification`: for the Cipher Rings the flag serves a "shop sold-out" role in `Twin Maiden Husks` (ShopLineupParam.csv `eventFlag_forStock`). Whether the game validates the Cipher Rings' uniqueness *only* through this flag, or also through the inventory — **has not been resolved**. The code comment signals that without this flag the shop "still offers the item".

### 6.3 What does NOT enter `itemCompanionEventFlags`

Entries explicitly excluded in the code comments:

| Excluded | Reason (from the comment) |
|---|---|
| `710770`, `69090`, `69370` (Melina leaves Gatefront) | Research candidate; the runtime PS4 test 2026-05-11 confirmed they are not required |
| Summoning Pool activation flags (`670xxx`) | A separate mechanism — Martyr Effigy activation |
| Flags of other multiplayer items not listed | Simply not added yet; there is no negative decision |

The test `TestCompanionEventFlagsForItem_NoTransientFlags` (`item_companion_flags_test.go:35`) enforces the exclusion of 5 transient IDs: `4698`, `4651`, `4652`, `4653`, `4656`. The `670xxx` band is enforced separately in `TestCompanionEventFlagsForItem_SmallGoldenEffigy_NoForbiddenFlags` and `TestCompanionEventFlagsForItem_MultiplayerPickup_NoForbiddenFlags`.

`needs verification`: the absence of the Talisman Pouch (`60500`/`60510`/`60520`) — this entry **does not exist** in `itemCompanionEventFlags`. Historical discussions suggested the hypothesis "SetGraceVisited synchronizes Talisman Pouch slots", but the current code has no such mechanism — neither in `item_companion_flags.go`, nor in `grace_companion_flags.go`, nor in `app_world.go`. Whether the game actually requires the sync — `needs verification`.

## 7. SET + CLEAR symmetric behavior

**The symmetric contract** for item companion flags:

| Action | What happens to companion flags |
|---|---|
| `AddItemsToCharacter` with an item that has a companion set | **SET** all flags in the set (idempotent — a bit already SET stays SET) |
| `RemoveItemsFromCharacter` with the handle of the last instance of the item | **CLEAR** all flags in the set |
| `RemoveItemsFromCharacter` with the handle of a non-last instance (other stacks/handles still exist after removal) | **No-op** — the flags stay SET |
| Re-add after removing the last instance | **SET** again |

The test `TestCompanionFlagsRemainingItemPreventsClearing` (`tests/item_companion_flags_test.go:354`) enforces the "non-last instance → no-op CLEAR" condition.

This is a **fundamentally different** contract than in `grace_companion_flags`:

| Trait | `item_companion_flags` (this chapter) | `grace_companion_flags` ([47](47-site-of-grace-activation.md) §9) |
|---|---|---|
| SET on activate | ✅ | ✅ |
| CLEAR on deactivate | ✅ (when the last instance is removed) | ❌ (SET-only) |
| Argument | item baseID | grace event flag ID |
| Called from | `AddItems` / `RemoveItems` (item lifecycle) | `SetGraceVisited` |
| Asymmetry | none — full symmetry | intentional — companion flags may be SET by another path |

The reason why item companion flags **can** be CLEARed: for the item lifecycle the decision "the item disappeared from the slot" is unambiguous (`slot.GaItems` scanned post-removal). For the grace lifecycle such unambiguity does not exist — a companion flag may be SET by an item (Whistle), a grace (Gatefront) or progress, so CLEAR on grace deactivate would risk regression.

## 8. Difference from grace companion flags

Already discussed in §7. In short: **shared constants, different policies**:

- The constants `EventFlagObtainedSpectralSteedWhistle`, `EventFlagMelinaGaveWhistle`, `EventFlagWhistleWorldState`, `EventFlagMelinaAcceptRefusePopup` are **shared** between both files (the only declaration is in `item_companion_flags.go`, `grace_companion_flags.go` re-uses the import).
- The maps `itemCompanionEventFlags` and `graceCompanionEventFlags` are **disjoint** (different keys — item baseID vs grace event flag ID).
- The APIs `CompanionEventFlagsForItem` and `CompanionEventFlagsForGrace` return independently composed sets. There is no cross-lookup.

`needs verification`: whether in the future one of the currently empty fields (e.g., another grace, more items) introduces overlapping companion sets — requires a policy decision. Currently cross-contamination is enforced by the test `TestCompanionEventFlagsForItem_MultiplayerPickup_NoCrossContamination`.

## 9. Difference from map fragment flags

`MapFragmentItems` (`backend/db/data/maps.go:318`) is a **separate** mechanism:

| Aspect | `itemCompanionEventFlags` (this chapter) | `MapFragmentItems` ([27](27-map-reveal.md), [29](29-dlc-black-tiles.md)) |
|---|---|---|
| Map type | `map[uint32][]uint32` (itemID → []eventFlag) | `map[uint32]uint32` (visible flag ID → item ID) |
| Direction | item → companion flags | visible flag → fragment item |
| Caller | `AddItemsToCharacter` / `RemoveItemsFromCharacter` (item lifecycle) | `revealBaseMap` / `revealDLCMap` (map reveal) |
| Entry count | 6 | 24 (19 base game + 5 DLC) |
| Lookup | `CompanionEventFlagsForItem(itemID)` | inline `data.MapFragmentItems[flagID]` |
| Set/Clear contract | symmetric SET+CLEAR | none — `revealBaseMap/DLC` only ADD items (via `AddItemsToSlot`); `ResetMapExploration` REMOVES items via `RemoveItemByBaseID` |

`MapFragmentItems` **does not use** `CompanionEventFlagsForItem`. These are two independent mechanisms:

- Map reveal: visible flag SET → add the corresponding fragment item (`AddItemsToSlot`).
- Item companion: fragment item added → … checks `CompanionEventFlagsForItem(fragmentID)` → returns `nil` (no entry), so **no** event flags are SET as a companion.

That is: setting a `MapVisible` flag in Phase 1 of `revealDLCMap` (see [27](27-map-reveal.md) §11 / [29](29-dlc-black-tiles.md) §11) does not entail a companion flag fan-out. Phase 2 adds a fragment item, but `AddItemsToSlot` (core, directly) does not go through the `AddItemsToCharacter` (app) path, so the SET hook (`app.go:569`) is **not called** for map fragment items.

`needs verification`: whether a fragment item → companion flag mapping *would* be needed (i.e., whether the game does anything special with fragment items beyond their presence in the inventory) — not verified. Currently the assumption is that fragment items are sufficient without companion flags.

## 10. Current implemented behavior

### 10.1 Hook SET — `AddItemsToCharacter`

`app.go:569-578`:

```go
if companions := data.CompanionEventFlagsForItem(p.baseID); len(companions) > 0 {
    if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
        eflags := slot.Data[slot.EventFlagsOffset:]
        for _, f := range companions {
            if err := db.SetEventFlag(eflags, f, true); err != nil {
                runtime.LogWarningf(a.ctx, "companion flag %d for item 0x%08X: %v", f, p.baseID, err)
            }
        }
    }
}
```

Characteristics:

- **Per-prepared-item** — iteration over the `prepared` slice, i.e., for each item matching the request (even if `actualInv == 0 && actualStorage == 0` — see §11 repair mode).
- **After `AboutTutorialID`** — the companion flags are SET after the tutorial ID section, in the same internal block.
- **Pre-rollback** — the hook runs **before** `ValidatePostMutation` (`app.go:626`). In case of a post-validation failure the entire slot is restored from `snapshot` (`core.RestoreSlot`), so companion flags are reverted together with everything else.
- **`EventFlagsOffset` tolerance** — if the offset is not sensible, the SET block is skipped **without** error propagation (silent skip).
- **Per-flag error handling** — `db.SetEventFlag` may return an error (out-of-range, etc.). The error is **logged via `runtime.LogWarningf`**, **not propagated**. Iteration continues — the remaining flags in the set are still SET.

### 10.2 Hook CLEAR — `RemoveItemsFromCharacter`

`app.go:658-666` (pre-scan) + `app.go:706-725` (CLEAR):

```go
// Pre-scan: collect item IDs with companion flags being removed.
companionRemovals := make(map[uint32]bool)
for _, handle := range handles {
    if itemID, ok := slot.GaMap[handle]; ok {
        if len(data.CompanionEventFlagsForItem(itemID)) > 0 {
            companionRemovals[itemID] = true
        }
    }
}
// ... (remove items) ...
// Clear companion flags for items no longer present in slot after removal.
if len(companionRemovals) > 0 && slot.EventFlagsOffset > 0 && ... {
    eflags := slot.Data[slot.EventFlagsOffset:]
    for itemID := range companionRemovals {
        remaining := false
        for _, g := range slot.GaItems {
            if !g.IsEmpty() && g.ItemID == itemID {
                remaining = true
                break
            }
        }
        if !remaining {
            for _, f := range data.CompanionEventFlagsForItem(itemID) {
                if err := db.SetEventFlag(eflags, f, false); err != nil {
                    runtime.LogWarningf(...)
                }
            }
        }
    }
}
```

Characteristics:

- **Pre-scan before remove** — `companionRemovals` is populated before the actual removal by `core.RemoveItemFromSlot`.
- **CLEAR only when the last instance is removed** — a full scan of `slot.GaItems` post-remove checks whether the item still exists anywhere in the slot. If so — the CLEAR is skipped.
- **No `pushUndo` for CLEAR** — `RemoveItemsFromCharacter` has `a.pushUndo(charIdx)` at line 654, **before** the remove loop. The snapshot covers the whole slot, so the CLEAR is undoable via Undo.
- **Per-flag error handling** — analogous to SET: `runtime.LogWarningf`, no propagation.

### 10.3 No `itemID` validation

The SET hook does not validate whether `p.baseID` is a *known* item (`db.ItemByBaseID` lookup). `CompanionEventFlagsForItem(unknownID)` returns `nil`, so the block is a no-op. This is OK from a safety perspective, but it means that a typo in the item ID in an external call will not be detected by this mechanism.

## 11. Write path and rollback caveats

### 11.1 Repair mode

The SET hook is called **for every item in `prepared`** — including items already in the slot at maximum quantity (`actualInv + actualStorage == 0` after `AddItemsToCharacter` clamping). Consequence: re-clicking "Add" on an item like the Spectral Steed Whistle on a slot that already has it, but **without** companion flags (e.g., a save edited before the mechanism was introduced) **repairs** the missing companion flags.

The tests `TestSmallGoldenEffigyRepair` (`tests/item_companion_flags_test.go:274`) + `TestMultiplayerPickupFlagRepair` (`tests/item_companion_flags_test.go:506`) — cover this path.

### 11.2 Atomicity and rollback in `AddItemsToCharacter`

`AddItemsToCharacter` has a **snapshot-level rollback**:

```go
snapshot := core.SnapshotSlot(slot)   // before mutations
... // item add + flags + tutorials + companions
if violations := core.ValidatePostMutation(slot); len(violations) > 0 {
    core.RestoreSlot(slot, snapshot)
    return result, fmt.Errorf("rollback: post-mutation validation failed: %s", ...)
}
```

The SET hook (companion flags) is **pre-validation** — if post-validation fails, the snapshot restore reverts companion flags together with the item add.

There is no per-flag atomicity inside the SET hook. If `SetEventFlag(eflags, f, true)` fails in the middle of the list (e.g., f=3/4 returns an error), the remaining flags are not retried; their state depends on the order and on whether the earlier calls managed to SET. The snapshot restore (if post-validation fails) cleans this up in the end.

### 11.3 Rollback in `RemoveItemsFromCharacter`

`RemoveItemsFromCharacter` does **not** have a post-validation rollback. `a.pushUndo` at the start creates a snapshot for a potential user Undo, but an error during remove does not revert the changes automatically. The CLEAR hook runs **after** the remove loop — if any flag fails, the error is logged, the remaining flags are still CLEARed.

### 11.4 No atomic multi-item

A bulk add of multiple items per call (`prepared` slice) is "atomic" only in the post-validation snapshot sense — either all items + their companion flags land in the slot, or none. A per-item rollback during iteration does not exist.

### 11.5 Slice invalidation

`slot.Data` may be reallocated by `core.AddItemsToSlot` (growing GaItems / inventory). The SET hook (`app.go:569`) re-derives `eflags := slot.Data[slot.EventFlagsOffset:]` *after* the add section — i.e., on a fresh `slot.Data`. Safe.

In `RemoveItemsFromCharacter` analogously — the CLEAR hook re-derives `eflags` after the remove loop.

## 12. Validation and safety notes

### 12.1 Stale hardcoded flag IDs after a patch

The `EventFlag...` constants in `item_companion_flags.go` are **hardcoded** with literals `uint32(60100)`, etc. There is no reference to a `regulation.bin` snapshot. After a game patch that would change the semantics of these flags (unlikely for obtained flags, but possible for world state flags like `710520`), the companion flags may:

- work OK (the game still reads the flag for the same purpose),
- work partially (some flags valid, some obsolete),
- disrupt quests (the flag now means something else).

`needs verification` after every major game patch. There is no detection mechanism.

### 12.2 Wrong flag ID = quest/world side effects

Each flag in a companion set has a "side effect surface" — `60100` unlocks Torrent, `4680` blocks re-trigger of the Melina dialogue, etc. Entering a wrong flag into `itemCompanionEventFlags` (e.g., `60101` instead of `60100`) could accidentally SET a flag of a completely different mechanism (item, quest, world state). There is no validation of the kind "is this flag an event flag obtained pattern".

Mitigation: 8 per-item unit tests + 17 integration tests. All of them verify *specific* IDs, so a typo would be caught at the first `go test ./...`.

### 12.3 Missing companion flag despite the item in the inventory

Possible in 2 scenarios:

1. A save edited before the mechanism was introduced (legacy save) — repair mode (§11.1) fixes it after re-Add.
2. `EventFlagsOffset` not set → silent skip of the SET hook. The item is in the slot, but the flag is not SET. Mitigation: repair mode after setting the offset.

`needs verification`: whether `EventFlagsOffset` can be `<= 0` in a real save after `LoadSave`. The current tests assume it is valid. The defensive skip in the SET hook is "belt and suspenders".

### 12.4 Event flag SET without the item

Possible in the scenario "the user removed the item, but the whistle is past the last instance" (stacked items, multiple handles). The pre-scan in `RemoveItemsFromCharacter` counts by itemID; CLEAR runs only when `remaining == false`. No race condition.

Another scenario: the user manually modifies the bitfield through other endpoints (`SetGraceVisited`, PvP prep) — then the flag may be SET without the item. See [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §9 for the rationale for the grace SET-only policy.

### 12.5 SET/CLEAR mismatch (race)

Within a single call (`AddItemsToCharacter` or `RemoveItemsFromCharacter`) the hook is sequential — no concurrency. Cross-call race (e.g., Add + Remove in parallel from the UI) — `app_world.go` / `app.go` assume single-threaded execution per slot (the Wails binding model). `needs verification`: whether the Wails dispatcher serializes calls per-slot.

### 12.6 Platform / version differences

The flag ID constants are identical for PC and PS4 (the BST resolver is cross-platform — see [15-event-flags.md](15-event-flags.md)). However:

- The flag `710520` (Whistle world state) is in a high band — `needs verification` whether the BST resolver locates it correctly on PS4 (the test `TestBSTLookupMatchesEventFlags` in `tests/map_flags_test.go` covers the 62xxx/63xxx/76xxx/82xxx bands, but not 710xxx).
- The range `1042xxx` is explicitly excluded from the companion sets (code comment: "Out of bounds on PS4 (BST offset ~130 MB vs ~2.3 MB flag table). Physically cannot be set"). The constant `EventFlagWhistleWorldState=710520` is in the 710xxx band — checked separately (`needs verification`: how many bytes the flag 710520 byte address is vs `EventFlagsByteCount=0x1BF99F`).

### 12.7 Rollback gaps

See §11. The main gap: `RemoveItemsFromCharacter` does not have post-mutation validation. If a CLEAR companion flag fails, there is no rollback — the error is only logged. In practice the CLEAR hook should not fail (a write to an existing bitfield bit), but defensively one could add a `pushUndo` snapshot + validation.

## 13. Test coverage

### 13.1 Unit tests (`backend/db/data/item_companion_flags_test.go`) — 11 tests

| Test | Line | What it verifies |
|---|---|---|
| `TestCompanionEventFlagsForItem_Whistle` | 5 | The Whistle companion set = exactly the 4 constants (`EventFlagObtained...`/`MelinaGave.../WhistleWorldState/MelinaAcceptRefusePopup`) |
| `TestCompanionEventFlagsForItem_Unknown` | 28 | `CompanionEventFlagsForItem(0xDEADBEEF)` → `nil` |
| `TestCompanionEventFlagsForItem_NoTransientFlags` | 35 | No `4698`/`4651`/`4652`/`4653`/`4656` in any companion set |
| `TestCompanionEventFlagsForItem_SmallGoldenEffigy` | 55 | SmallGoldenEffigy companion = `[60230]` (exactly 1 flag) |
| `TestCompanionEventFlagsForItem_SmallGoldenEffigy_NoForbiddenFlags` | 68 | No `60220/60240/.../60310` nor `670xxx` nor Whistle flag in SmallGoldenEffigy |
| `TestCompanionEventFlagsForItem_DuelistsFurledFinger` | 89 | DuelistsFurledFinger = `[60240]` |
| `TestCompanionEventFlagsForItem_SmallRedEffigy` | 99 | SmallRedEffigy = `[60250]` |
| `TestCompanionEventFlagsForItem_WhiteCipherRing` | 109 | WhiteCipherRing = `[60280]` |
| `TestCompanionEventFlagsForItem_BlueCipherRing` | 119 | BlueCipherRing = `[60290]` |
| `TestCompanionEventFlagsForItem_MultiplayerPickup_NoForbiddenFlags` | 129 | All 5 multiplayer items — no forbidden, no `670xxx`, no cross-contamination |
| `TestCompanionEventFlagsForItem_MultiplayerPickup_NoCrossContamination` | 177 | Each multiplayer item has only its own `Obtained` flag, not others' |

(11 test functions — the table above lists all of them with their start line.)

### 13.2 Integration tests (`tests/item_companion_flags_test.go`) — 17 tests

| Test | Line | What it verifies |
|---|---|---|
| `TestCompanionFlagsSetOnRealSave` | 17 | Whistle companion flags settable + readable on a real slot |
| `TestCompanionFlagsForbiddenAbsent` | 52 | The forbidden list is absent from the companion sets |
| `TestCompanionFlagsMechanicFlagPresent` | 71 | The mechanic flag `60100` is actually SET for the Whistle |
| `TestCompanionFlagsNoRoundtableFlags` | 84 | RTH flags `10009655`/`11109658`/`11109659` absent |
| `TestCompanionFlagsClearOnFlagData` | 107 | The CLEAR path works on the bitfield |
| `TestCompanionFlagsNotClearedForUnknownItem` | 149 | An item without a companion set → no-op |
| `TestSmallGoldenEffigyFlagSet` | 193 | `60230` SET after `AddItemsToCharacter(SmallGoldenEffigy)` |
| `TestSmallGoldenEffigyFlagClear` | 223 | `60230` CLEAR after `RemoveItemsFromCharacter` of the last instance |
| `TestSmallGoldenEffigyRepair` | 274 | Re-add SmallGoldenEffigy on a slot with the missing flag `60230` → repairs |
| `TestSmallGoldenEffigyNoSummoningPoolFlags` | 309 | `670xxx` are not SET by the Effigy hook |
| `TestSmallGoldenEffigyUnrelatedItemDoesNotAffectFlag` | 320 | Adding another item does not change `60230` |
| `TestCompanionFlagsRemainingItemPreventsClearing` | 354 | Non-last instance → no-op CLEAR |
| `TestMultiplayerPickupFlagSet` | 407 | The `Obtained` flags SET for all 5 multiplayer items |
| `TestMultiplayerPickupFlagClear` | 441 | same CLEAR |
| `TestMultiplayerPickupFlagRepair` | 506 | Repair mode for multiplayer items |
| `TestMultiplayerPickupNoSummoningPoolFlags` | 542 | same `670xxx` |
| `TestMultiplayerPickupUnrelatedItemDoesNotAffectFlags` | 554 | same unrelated item |

`needs verification`: there is no isolated **Whistle full roundtrip** test (PS4 platform, 4 flags SET + readback). The PS4 runtime test 2026-05-11 was ad-hoc, not automated.

## 14. Known limits / needs verification

A condensed list of open questions:

1. **The count of 6 entries stale after a patch** — no auto-regeneration.
2. **Talisman Pouch sync** (`60500`/`60510`/`60520`) — a historical hypothesis, **not** in the current code; unknown whether the game requires it.
3. **Cipher Rings shop uniqueness** — whether the game validates only through the flag, or also through the inventory.
4. **Whistle 4-flag combo runtime test** — manual PS4 2026-05-11, no CI test.
5. **Cross-platform flags 710xxx** — the BST resolver coverage of the band is unverified by a test.
6. **`EventFlagsOffset <= 0` silent skip** — defensive, but creates a silent sync break.
7. **Stale flag IDs after a game patch** — no detection.
8. **Wails serialization** — the assumption of single-threaded execution per slot is unverified.
9. **MapFragmentItems do not use the companion hook** — confirmed, but `needs verification` whether they *should* (whether the game does anything more with fragment items).
10. **Cross-module flag policy decisions** — in the future may require a policy "which policy wins" for overlapping companion sets.

## 15. Cross-references

- [11-regions.md](11-regions.md) — L0 Map Reveal; untouched by item companion flags.
- [14-game-state.md](14-game-state.md) — `LastRestedGrace` / `PreEventFlagsScalars`; untouched.
- [15-event-flags.md](15-event-flags.md) — master event flag API; this chapter is a caller.
- [16-world-state.md](16-world-state.md) — `WorldGeomMan` / `WorldArea`; untouched.
- [27-map-reveal.md](27-map-reveal.md) — `MapFragmentItems` as a separate mechanism (§9).
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — L2 DLC Cover Layer; independent.
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — `grace_companion_flags`; shared constants, different policy SET-only vs SET+CLEAR.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — `ColosseumGlobalFlags` SET independently of item companion flags (context: `60100` may be SET by PvP prep, independently of the Whistle).

## 16. Sources

- `backend/db/data/item_companion_flags.go` — `itemCompanionEventFlags`, `CompanionEventFlagsForItem`, 6 item ID constants, 9 companion flag ID constants.
- `backend/db/data/item_companion_flags_test.go` — 11 unit test functions.
- `tests/item_companion_flags_test.go` — 17 integration test functions.
- `app.go::AddItemsToCharacter` lines 569-578 — hook SET.
- `app.go::RemoveItemsFromCharacter` lines 658-666 + 706-725 — hook CLEAR.
- `backend/db/data/grace_companion_flags.go` — reference for the grace SET-only policy.
- `backend/db/data/maps.go::MapFragmentItems` — reference for the separate fragment items mechanism.
- `docs/CHANGELOG.md` — historical PS4 runtime test 2026-05-11.
