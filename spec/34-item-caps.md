# 34 — Item Caps Enforcement (NG+ Scaling + Full Chaos Mode)

## Purpose

`MaxInventory` / `MaxStorage` are conservative **Normal Mode** editing limits,
usually based on useful or single-playthrough availability. They are not
save-integrity invariants. `GameMaxInventory` / `GameMaxStorage` are separate
technical limits sourced from regulation data and are used by Full Chaos Mode,
the loaded-save scanner, and quantity repair.

**Primary goal:** reduce EAC detection risk by keeping item counts in a range statistically consistent with vanilla. Secondary: consistency with in-game UI (e.g. cookbook = 1 piece, physically impossible to have 2).

## Scaling mechanism

```
effective_cap = MaxInventory × (ClearCount + 1)   if item.flags has "scales_with_ng"
effective_cap = MaxInventory                      otherwise
effective_cap = GameMaxInventory                  if Full Chaos Mode is ON and known
effective_cap = conservative cap                  if Full Chaos Mode is ON and game limit is unknown
```

`ClearCount` (uint32, 0..7+) sits in the save in a dynamic offset chain after BloodStain — see `backend/core/structures.go:178` and `offset_defs.go:101`. Exposed to the frontend as `CharacterViewModel.clearCount` (`backend/vm/character_vm.go:44,72`).

| ClearCount | Game cycle | Multiplier |
|---|---|---|
| 0 | NG | ×1 |
| 1 | NG+1 | ×2 |
| 2 | NG+2 | ×3 |
| ... | ... | ... |
| 7 | NG+7 | ×8 |

**TODO**: verify on a real save when the game increments `ClearCount` — on killing Elden Beast or on entering the NG+ menu? Currently we assume 0 = pre-Elden-Beast of the first cycle.

## Items with the `scales_with_ng` flag

All with a base per single playthrough per Fextralife (v1.16):

| Item | File | Base cap | Source |
|---|---|---|---|
| Stonesword Key | `key_items.go` | 55 | ~47 base + ~10 DLC purchases/drops; NG+ respawns imp statues |
| Dragon Heart | `key_items.go` | 22 | ~16 base (Greyoll 5 + named dragons) + ~6 DLC; NG+ respawns dragons |
| Larval Tear | `key_items.go` | 24 | 18 base + 6 DLC; NG+ respawns Albinaurics and drops |

## Items with a hard cap (no scaling) — caps as "max useful"

The cap does not change with NG+ — a game limit, not a collection limit:

| Item | Cap | Reason |
|---|---|---|
| Memory Stone | 8 | Spell slot cap = 8 (game hard limit) |
| Talisman Pouch | 3 | Talisman slot count = 3 (game hard limit) |
| Golden Seed | 30 | Full flask charges (14 charges, cumulative cost 30 seeds) |
| Sacred Tear | 12 | Full flask potency (+12) |
| Scadutree Fragment | 50 | Max Scadutree Blessing (+20, cumulative cost 50) |
| Revered Spirit Ash | 25 | Max Revered Spirit Ash Blessing (+10, cumulative cost 25) |

**Design rule**: if the cap = "max useful" (exceeding it yields no functional gain), do not scale with NG+. Even after Elden Beast in NG+1, the player already has +12 potency / +14 flask charges / +20 blessing — additional seeds/tears/fragments are useless.

## Items with cap 1/0 (unique drops)

Per single playthrough physically 1 piece (consumable / unique pickup):

- All **gestures** (54), **sorceries** (85), **incantations** (128) — memorized once.
- All **About \* tutorials** (~37) and **letters / story keys** in `info.go`.
- **Paintings** (3 base + 9 DLC) — read-once items.
- **Notes** (15 base + 2 DLC) — read-once items.
- **Crystal Tears** (~22 base + 5 DLC) — unique pickups, Twin Maidens 1-time purchases (Cerulean/Viridian Hidden, etc.).
- **Great Runes / Bell Bearings / keys** (~270) in `key_items.go`.
- **Cookbooks** (109) — all unique (the cookbook is "consumed" when given to a merchant).
- **Multiplayer Fingers** (11) and **Remembrances** (25 = 15 base + 10 DLC) in `tools.go`.

## Full Chaos Mode

Toggle in `SettingsTab.tsx` → Safety section (red-bordered, ban-risk copy):

```
[setting:fullChaosMode] → 'true' / 'false' (default false)
```

- `localStorage` persisted
- Cross-component sync via `window` CustomEvent `'fullChaosModeChanged'` (detail: boolean)
- When enabled: `effectiveCap` uses `GameMaxInventory` / `GameMaxStorage` where
  known, with a conservative fallback where regulation data is unavailable.

**UX**: when enabled, the modal shows a red *"technical game caps"* banner and
states when any selected item uses a conservative fallback.

## UI implementation (`DatabaseTab.tsx`)

Helper:
```typescript
function effectiveCap(item, kind, clearCount, chaos): number {
    if (chaos && gameLimitKnown(item, kind)) return gameLimit(item, kind);
    const base = kind === 'inv' ? item.maxInventory : item.maxStorage;
    if (item.flags?.includes('scales_with_ng')) return base * (clearCount + 1);
    return base;
}
```

The modal `min`/`max` uses `effectiveCap()` instead of `item.maxInventory`/`item.maxStorage`. The **NG+ Scaling** banner is rendered when:
- any of the selected items has the `scales_with_ng` flag
- AND `Full Chaos Mode` is off

The banner shows:
- `Vanilla NG: X` (always)
- `NG+Y: Z` (when `clearCount > 0`)
- An educational line *"Adding more increases EAC ban risk"*

## Architecture: where the clamp is enforced

| Layer | Clamp enforcement |
|---|---|
| `core` (binary load/save) | **None** — read/write is transparent |
| `vm.MapParsedSlotToVM` | **No** changes — existing clamp in handleSetItemQty remains |
| `vm.handleSetItemQty` | Clamp to `MaxInventory`/`MaxStorage` (legacy behavior) |
| `frontend DatabaseTab modal` | **Primary enforcement site** — `min/max` on `<input>`, validation before `AddItemsToCharacter()` |
| `Full Chaos Mode` | Uses the dedicated `AddItemsToCharacterWithGameLimits` backend path and regulation-derived game caps |

**Conscious decision**: the clamp at load/save **does NOT** modify values in an existing save. A player who has 999 Larval Tears (e.g. from another editor) keeps them on the load → save round-trip.

## Repair: safe quantity clamp (loaded save only)

The integrity scanner reports records whose effective quantity exceeds their
technical game container cap. `EffectiveQuantityCap(rec, clearCount)` in
`backend/core/repair_scanner.go` is the single source of cap semantics, shared
verbatim by the scanner and the repair primitive, so the two can never disagree.
Unknown game limits are skipped rather than treated as zero. Known zero remains
a real container prohibition.

- **Positive cap** → `quantity_above_max`. Offered actions: `clamp_quantity`
  (default) and `leave_unchanged`. `ClampInventoryQuantityAt`
  (`backend/core/quantity_clamp.go`) recomputes the authoritative cap at apply
  time (never trusting the frontend), writes both the raw 4-byte quantity field
  and the in-memory value, and **preserves the high quantity bit** (`0x80000000`).
- **Zero cap** (item not permitted in the container, e.g. Stonesword Key in
  storage) → a separate `item_not_allowed_in_container` issue. Offered actions:
  `remove_record` and `leave_unchanged` (**default**, because removal is
  destructive). The quantity is **never** clamped to zero — that would only
  manufacture a new `quantity_zero` defect.

The clamp runs through the existing loaded-save repair pipeline: fingerprint
stale-check, one pre-batch undo snapshot (only when a mutation succeeds),
rollback on failure, post-repair rescan, and a clamp-specific postcondition that
rejects any result still leaving `quantity_above_max` or `quantity_zero` at the
targeted row.

The scanner, repair primitive, and Full Chaos add path share `GameMax*` values.
Normal Mode continues to use the conservative caps and NG+ scaling.

### Regulation sources and flask semantics

- Goods inventory: `EquipParamGoods.maxNum`.
- Goods storage: `maxRepositoryNum` only when `isDeposit=1`; `isDeposit=0` is a
  known storage prohibition.
- Ammunition inventory: `EquipParamWeapon.maxArrowQuantity`; repository cap 600.
- Full Crimson/Cerulean flask records have a technical per-record cap of 20.
  Their quantities represent allocated charges (for example 12 + 2). The normal
  gameplay total of 14 is a separate aggregate invariant and is not
  automatically repaired by the per-record clamp.

## Verification

- `go test ./backend/...` — pass (data, db, vm, core)
- `npx tsc --noEmit && npm run build` — pass
- Manual test plan:
  1. NG save → Database → Stonesword Key → modal shows cap **55**
  2. NG+3 save → cap **220** (=55×4), tooltip *"Vanilla NG: 55 · NG+3: 220"*
  3. Settings → Full Chaos Mode ON → cap **999**, red banner visible
  4. Cookbook (Glintstone Craftsman's [1]) → cap **1**, attempt to add 2 is blocked
  5. Painting → cap **1**, no storage row
  6. Mohg's Great Rune is shown in the Key Items category, absent from Bolstering

## Sources

- Fextralife wiki per-item Locations (eldenring.wiki.fextralife.com) — primary source
- Cross-checked with: Game8, PowerPyx, GamesRadar, GameSpot, PCGamer
- v1.16 patch (2026-04 state)
