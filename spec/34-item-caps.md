# 34 — Item Caps Enforcement (NG+ Scaling + Full Chaos Mode)

> **Type**: Design doc  
> **Scope**: `MaxInventory` / `MaxStorage` cap system with NG+ scaling and Full Chaos Mode bypass.

> **Status**: ✅ Deployed (Apr 2026).

---

## Goal

The `MaxInventory` / `MaxStorage` limit in the item database reflects the **realistic number of items obtainable in a single playthrough**. For items that carry over to NG+ and respawn, the cap scales linearly with the NG+ cycle counter (`ClearCount`). Power users can disable the entire mechanism in Settings (Full Chaos Mode).

**Primary goal:** reduce EAC detection risk by keeping item quantities within statistically vanilla-consistent ranges. Secondary: consistency with in-game UI (e.g. cookbook = 1 copy, physically impossible to have 2).

---

## Scaling mechanism

```
effective_cap = MaxInventory × (ClearCount + 1)   if item.flags has "scales_with_ng"
effective_cap = MaxInventory                      otherwise
effective_cap = 999                               if Full Chaos Mode is ON
```

`ClearCount` (uint32, 0..7+) is located in the save in the dynamic offset chain after BloodStain — see `backend/core/structures.go:178` and `offset_defs.go:101`. Exposed to frontend as `CharacterViewModel.clearCount` (`backend/vm/character_vm.go:44,72`).

| ClearCount | Game cycle | Multiplier |
|---|---|---|
| 0 | NG | ×1 |
| 1 | NG+1 | ×2 |
| 2 | NG+2 | ×3 |
| ... | ... | ... |
| 7 | NG+7 | ×8 |

**TODO**: verify on a real save when the game increments `ClearCount` — upon killing Elden Beast or upon entering the NG+ menu? Currently assuming 0 = pre-Elden-Beast of first cycle.

---

## Items with `scales_with_ng` flag

All with a base per single playthrough per Fextralife (v1.16):

| Item | File | Base cap | Source |
|---|---|---|---|
| Stonesword Key | `key_items.go` | 55 | ~47 base + ~10 DLC purchases/drops; NG+ respawns imp statues |
| Dragon Heart | `key_items.go` | 22 | ~16 base (Greyoll 5 + named dragons) + ~6 DLC; NG+ respawns dragons |
| Larval Tear | `key_items.go` | 24 | 18 base + 6 DLC; NG+ respawns Albinaurics and drops |

---

## Items with hard cap (no scaling) — caps are "max useful"

Cap does not change with NG+ — game limit, not collectability limit:

| Item | Cap | Reason |
|---|---|---|
| Memory Stone | 8 | Spell slot cap = 8 (game hard limit) |
| Talisman Pouch | 3 | Talisman slot count = 3 (game hard limit) |
| Golden Seed | 30 | Full flask charges (14 charges, cumulative cost 30 seeds) |
| Sacred Tear | 12 | Full flask potency (+12) |
| Scadutree Fragment | 50 | Max Scadutree Blessing (+20, cumulative cost 50) |
| Revered Spirit Ash | 25 | Max Revered Spirit Ash Blessing (+10, cumulative cost 25) |

**Design principle**: if cap = "max useful" (exceeding provides no functional benefit), we don't scale with NG+. Even after Elden Beast in NG+1, the player already has +12 potency / +14 flask charges / +20 blessing — additional seeds/tears/fragments are useless.

---

## Items with cap 1/0 (unique drops)

Per single playthrough physically 1 copy (consumable / unique pickup):

- All **gestures** (54), **sorceries** (85), **incantations** (128) — memorized once.
- All **About * tutorials** (~37) and **letters / story keys** in `info.go`.
- **Paintings** (3 base + 9 DLC) — read-once items.
- **Notes** (15 base + 2 DLC) — read-once items.
- **Crystal Tears** (~22 base + 5 DLC) — unique pickups, Twin Maidens 1-time purchases (Cerulean/Viridian Hidden, etc.).
- **Great Runes / Bell Bearings / keys** (~270) in `key_items.go`.
- **Cookbooks** (109) — all unique (cookbook is "consumed" when given to merchant).
- **Multiplayer Fingers** (11) and **Remembrances** (25 = 15 base + 10 DLC) in `tools.go`.

---

## Full Chaos Mode

Toggle in `SettingsTab.tsx` → Safety section (red-bordered, ban-risk copy):

```
[setting:fullChaosMode] → 'true' / 'false' (default false)
```

- `localStorage` persisted
- Cross-component sync via `window` CustomEvent `'fullChaosModeChanged'` (detail: boolean)
- When enabled: `effectiveCap` ignores `MaxInventory`/`scales_with_ng` and returns 999

**UX**: when enabled, the modal in Database tab shows a red banner *"⚠ Full Chaos Mode — caps bypassed (max 999)"*.

---

## UI implementation (`DatabaseTab.tsx`)

Helper:
```typescript
function effectiveCap(item, kind, clearCount, chaos): number {
    if (chaos) return 999;
    const base = kind === 'inv' ? item.maxInventory : item.maxStorage;
    if (item.flags?.includes('scales_with_ng')) return base * (clearCount + 1);
    return base;
}
```

The modal `min`/`max` uses `effectiveCap()` instead of `item.maxInventory`/`item.maxStorage`. **NG+ Scaling** banner renders when:
- any selected item has the `scales_with_ng` flag
- AND `Full Chaos Mode` is disabled

Banner shows:
- `Vanilla NG: X` (always)
- `NG+Y: Z` (when `clearCount > 0`)
- Educational line *"Adding more increases EAC ban risk"*

---

## Architecture: where clamping is enforced

| Layer | Clamp enforcement |
|---|---|
| `core` (binary load/save) | **None** — read/write is transparent |
| `vm.MapParsedSlotToVM` | **No change** — existing clamp in handleSetItemQty remains |
| `vm.handleSetItemQty` | Clamp to `MaxInventory`/`MaxStorage` (legacy behavior) |
| `frontend DatabaseTab modal` | **Primary enforcement** — `min/max` on `<input>`, validation before `AddItemsToCharacter()` |
| `Full Chaos Mode` | Bypasses clamp in UI only (vm clamp still works, but 999 max from UI is sufficient in practice) |

**Intentional decision**: clamping at load/save does **NOT** modify values in an existing save. A player who has 999 Larval Tears (e.g. from another editor) will keep them through the load → save round-trip.

---

## Verification

- `go test ./backend/...` — pass (data, db, vm, core)
- `npx tsc --noEmit && npm run build` — pass
- Manual test plan:
  1. NG save → Database → Stonesword Key → modal shows cap **55**
  2. NG+3 save → cap **220** (=55×4), tooltip *"Vanilla NG: 55 · NG+3: 220"*
  3. Settings → Full Chaos Mode ON → cap **999**, red banner visible
  4. Cookbook (Glintstone Craftsman's [1]) → cap **1**, attempt to add 2 blocked
  5. Painting → cap **1**, no storage row
  6. Mohg's Great Rune displays in Key Items category, absent from Bolstering

---

## Sources

- Fextralife wiki per-item Locations (eldenring.wiki.fextralife.com) — primary source
- Cross-checked with: Game8, PowerPyx, GamesRadar, GameSpot, PCGamer
- v1.16 patch (2026-04 state)
