# 54 — Ash of War

> **Type**: Design doc
> **Status**: ✅ Implemented
> **Scope**: Built-in vs custom Ash of War semantics in the save, no-custom AoW sentinels, shared-handle invariant, writer/reader rules, and how SaveForge keeps the on-disk output aligned with vanilla in-game saves.

---

## 1. Overview

Every weapon in Elden Ring has a skill exposed in-game as its **Ash of War**. The skill can come from two distinct sources:

- **Built-in / default weapon skill** — defined in `regulation.bin` per weapon row (`EquipParamWeapon.swordArtsParamId`). Always available, never stored in the save.
- **Custom external Ash of War gem** — an optional in-inventory item (with its own GaItem entry) that the player can attach to a weapon. When attached, it overrides the weapon's built-in skill with the gem's own skill (`EquipParamGem.swordArtsParamId`).

SaveForge must model both layers separately. Failing to do so produces two classes of user-facing bugs:

- Treating "no custom AoW" as "no skill at all" — confuses the user into thinking removal destroys the weapon's skill, which it does not.
- Treating two custom-AoW copies of the same gem ItemID as the same instance — leads either to spurious "in use" UI states or to the much more dangerous shared-handle crash described in §6.

---

## 2. Data model

### Game parameter tables (regulation.bin, read-only)

- `EquipParamWeapon` — one row per weapon ItemID. The `swordArtsParamId` column points to the **built-in** skill row in `SwordArtsParam`. This value is the fallback skill when no custom AoW is attached.
- `EquipParamGem` — one row per Ash of War gem. Its `swordArtsParamId` column points to the skill the gem grants when attached, and `canMountWep_*` exposes a 36-bit bitmask of compatible weapon `wepType` values.
- `SwordArtsParam` — skill definitions (animations, costs, FMG textId for the in-game name).

These three tables live in `regulation.bin` (UserData11). SaveForge **never writes** to them — see the project's hard rule on regulation.bin in `CLAUDE.md`.

### Save layout

- AoW gems live in the slot's GaItem map as 8-byte records (see [03-gaitem-map](03-gaitem-map.md)):
    - GaItem handle prefix: `0xC0000000` (ItemTypeAow).
    - AoW gem ItemID prefix: `0x80000000` (e.g. Lion's Claw = `0x80002710`).
- Every weapon GaItem record is 21 bytes. Offset `0x10` holds a `u32` field — `AoWGaItemHandle` — that references the attached AoW gem by **handle**, not by ItemID.
- Inventory rows reference items by handle; the GaItem map resolves handle → ItemID via `slot.GaMap`.

---

## 3. Skill resolution

The game resolves the active skill at load time using both the save and `regulation.bin`. The effective skill is:

    if IsValidCustomAoWHandle(weapon.AoWGaItemHandle):
        gem = GaMap[weapon.AoWGaItemHandle]
        skill = EquipParamGem[gem.ItemID].swordArtsParamId
    else:
        skill = EquipParamWeapon[weapon.ItemID].swordArtsParamId

Two consequences worth stating plainly:

- **Removing a custom AoW does not erase the default skill.** Detaching the external gem only clears the `AoWGaItemHandle` reference; the weapon ItemID stays the same and the game falls back to `EquipParamWeapon.swordArtsParamId` on next load.
- **"No custom AoW" does not mean "no skill".** Most weapons ship with a non-zero `swordArtsParamId` (Lordsworn's Straight Sword → 115 "Square Off", Icerind Hatchet → 109 "Hoarfrost Stomp"). The UI must communicate this clearly — see `WeaponEditModal.tsx` after commit `cb1a822`.

---

## 4. No-custom sentinels

The 4 bytes at `weapon GaItem + 0x10` carry one of the following classes of value:

| Value | Meaning | Writer? | Reader? |
|---|---|---|---|
| `0x00000000` | No custom AoW — **canonical vanilla sentinel** (the game writes this for every freshly created weapon record). | ✅ canonicalized to here | ✅ accepted |
| `0xFFFFFFFF` | No custom AoW — **legacy SaveForge sentinel** emitted by builds before commit `4e800b9`. | ❌ never emitted | ✅ accepted for compat |
| `0xC0xxxxxx` (prefix matches `ItemTypeAow`) | Valid custom AoW handle. Must resolve to an existing AoW GaItem record. | ✅ written when attaching | ✅ accepted |
| Any other value | Invalid / corrupted. | n/a | flagged downstream |

The dual-sentinel acceptance is what lets SaveForge open saves edited by older releases without re-flagging every weapon. The single-sentinel emission keeps newly-written saves indistinguishable from vanilla output at this offset, which is the primary anti-flag concern (see [45-ban-risk-reference](45-ban-risk-reference.md) for the broader risk framework).

---

## 5. Remove semantics

Removing a custom Ash of War is a **detach**, not a delete:

- The 4-byte field at weapon `[+0x10]` is overwritten with the canonical sentinel `0x00000000` in place.
- `Weapon.ItemID` is not touched. Affinity (e.g. Heavy Longsword vs Longsword) lives in the weapon's ItemID and survives the remove unchanged.
- The previously-attached AoW GaItem record stays in the slot's GaItem map. It becomes an **orphan / free copy** that the game tolerates and that the strict writer (`PatchWeaponAoWHandle`) can re-attach later without allocating a fresh GaItem entry.
- On the next game load the engine resolves the skill via the fallback branch in §3 — the weapon's built-in skill reappears in the player's UI.

The user-facing toast and tooltip after commit `cb1a822` state this directly: "Custom Ash of War removed — built-in skill restored."

---

## 6. Shared-handle invariant

A single AoW gem **handle** identifies a single physical instance of an Ash of War. Two distinct weapons must never reference the same non-sentinel `AoWGaItemHandle`. Violating this triggers `EXCEPTION_ACCESS_VIOLATION` in the game on load.

The user-visible behavior the player might confuse with handle sharing is legitimate:

- The same AoW **ItemID** (e.g. `0x80002710` Lion's Claw) may appear in the GaItem map as **multiple separate copies**, each with its own handle (`0xC0...`). This is how Lost Ashes of War duplication, scarab drops, and ordinary picks accumulate copies.
- Two weapons may both display the same Ash of War in-game so long as they point to **different** AoW gem handles — i.e. two distinct gem instances with the same ItemID.

SaveForge enforces the invariant in every write path:

- `core.PatchWeaponAoWHandle` rejects any attach where the requested AoW handle is already referenced by another weapon record in the same slot.
- `core.PatchWeaponAoW` allocates a fresh `0xC0...` handle for every "attach by ItemID" call — it never reuses an existing handle, so it cannot create a conflict.
- The strict UI path (`App.ApplyWeaponAoWStrict`) picks the first **free** AoW GaItem copy (no current weapon references it) before delegating to `PatchWeaponAoWHandle`.

The forensic audit across `tmp/save/ER0000*.sl2` fixtures found **zero** shared-handle occurrences (see prompt 10 audit).

---

## 7. Compatibility and availability

`core.ScanAoWAvailability` walks the GaItem map and emits one `AoWCopyRaw` per AoW gem instance. The VM/UI aggregates by ItemID and surfaces these states:

| Status | Meaning |
|---|---|
| `current` | This AoW ItemID is the one currently attached to the weapon being edited. |
| `available` | At least one AoW GaItem copy of this ItemID exists with no weapon referencing its handle. |
| `in_use` | All copies of this AoW ItemID are already attached to weapons. Strict apply cannot proceed without removing one first; legacy apply would allocate a new GaItem entry. |
| `missing` | No AoW GaItem copy of this ItemID exists in the slot. Strict apply is impossible until the player obtains a copy (e.g. Lost Ashes duplication). |
| `conflict` | `HasSharedHandleConflict == true`. Save corruption indicator; strict path refuses to operate on the affected ItemID. |

Compatibility between a custom AoW and the target weapon is independent of availability and is computed from `EquipParamGem.canMountWep_*` against the weapon's `EquipParamWeapon.wepType`. The mapping `wepType → bit position` lives in `frontend/src/components/WeaponEditModal.tsx` (`wepTypeToBitPos`) and mirrors `backend/db/data` constants. An AoW that is incompatible with the weapon type is blocked at the UI level regardless of availability.

---

## 8. Writer / reader rules

| Rule | Writer | Reader |
|---|---|---|
| Accept both `0x00000000` and `0xFFFFFFFF` as no-custom on input. | yes (canonicalizes to `0x00000000`) | yes |
| Emit only `0x00000000` for no-custom / remove. | yes | n/a |
| Emit a valid AoW gem ItemID instead of a handle. | **never** — the field is a handle, not an ItemID | n/a |
| Reject `newAoWHandle` whose prefix is not `0xC0000000` (and not a no-custom sentinel). | yes | n/a |
| Reject `newAoWHandle` that does not point to an existing AoW GaItem. | yes | n/a |
| Reject `newAoWHandle` already referenced by another weapon (shared-handle guard). | yes | n/a — reported as conflict in availability |
| Preserve `Weapon.ItemID` across any remove. | yes | n/a |
| `PatchWeaponAoW` (legacy) may allocate a fresh `0xC0...` handle. | yes | n/a |
| `PatchWeaponAoWHandle` (strict) reuses an existing free handle, never allocates. | yes | n/a |

---

## 9. Code references

| Concern | File | Symbol |
|---|---|---|
| Constants and helper | `backend/core/structures.go` | `NoCustomAoWHandle`, `LegacyNoCustomAoWHandle`, `IsNoCustomAoWHandle`, `GaItemFull.AoWGaItemHandle` |
| Strict in-place patch | `backend/core/writer.go` | `PatchWeaponAoWHandle` (canonicalizes input, shared-handle guard) |
| Legacy alloc-and-attach | `backend/core/writer.go` | `PatchWeaponAoW` (remove writes `NoCustomAoWHandle`; attach allocates fresh handle) |
| New-weapon allocation | `backend/core/writer.go` | `allocateGaItem` (initializes `AoWGaItemHandle: NoCustomAoWHandle`) |
| Availability scan | `backend/core/aow_availability.go` | `ScanAoWAvailability`, `AoWCopyRaw` |
| UI resolver | `backend/vm/character_vm.go` | `mapItems` AoW branch (uses `core.IsNoCustomAoWHandle`) |
| App-level entry points | `app.go` | `ApplyWeaponAoWStrict`, `ApplyWeaponAoW`, `GetAoWAvailability` |
| UI copy and remove UX | `frontend/src/components/WeaponEditModal.tsx` | "Default skill" header, remove tooltip, restore-built-in-skill toast |

---

## 10. History / forensic notes

- Forensic scan of `tmp/save/ER0000-kro55-vanilla.sl2` and unmodified slots of `tmp/save/ER0000.sl2`: **every** weapon without a custom AoW stored `0x00000000` at offset `[+0x10]`. The game itself never emits `0xFFFFFFFF` for this field.
- Slot 4 of `tmp/save/ER0000-out.sl2` (a slot edited by older SaveForge bulk-add): mixed sentinel histogram — 196 weapons with `0xFFFFFFFF` and 21 with `0x00000000`. The game tolerated this mix at load time (no crash), but the output was no longer vanilla-aligned.
- Audit of all available save fixtures showed `DUPLICATE_AOW_HANDLE = 0` across every slot, confirming the shared-handle invariant has held in practice.
- Commit `4e800b9` (`fix(aow): use vanilla no-custom sentinel`) made the writer emit `0x00000000` everywhere and made every reader/availability path tolerant of both sentinels via `IsNoCustomAoWHandle`.
- Commit `cb1a822` (`fix(ui): clarify custom Ash of War removal`) replaced the misleading "None" header with "Default skill", clarified the Remove tooltip and toast to mention built-in skill restoration, and added an inline explanatory paragraph.

---

## Sources

- Code: `backend/core/structures.go`, `backend/core/writer.go`, `backend/core/aow_availability.go`, `backend/vm/character_vm.go`, `app.go`, `frontend/src/components/WeaponEditModal.tsx`.
- Tests: `backend/core/aow_strict_test.go` (sentinel regression suite — `TestPatchWeaponAoWHandle_RemoveWritesZeroSentinel`, `TestAllocateGaItem_NewWeaponUsesZeroSentinel`, `TestScanAoWAvailability_ZeroSentinelNotCounted`, `TestScanAoWAvailability_LegacyFFFFFFFFSentinelNotCounted`, `TestIsNoCustomAoWHandle`, `TestPatchWeaponAoWHandle_RemovePreservesWeaponItemID`, `TestPatchWeaponAoW_LegacyRemoveWritesZeroSentinel`).
- Game data: `tmp/regulation-bin-dump/csv/EquipParamWeapon.csv` (column 191 `swordArtsParamId`, 196 `wepType`, 248 `gemMountType`), `EquipParamGem.csv` (column 12 `swordArtsParamId`, plus `canMountWep_*` bitmask), `SwordArtsParam.csv`.
- Forensic fixtures: `tmp/save/ER0000.sl2`, `tmp/save/ER0000-kro55-vanilla.sl2`, `tmp/save/ER0000-out.sl2`, `tmp/save/ER0000-kro55-out.sl2`.
- Related specs: [03-gaitem-map](03-gaitem-map.md), [06-equipment](06-equipment.md), [53-inventory-storage-transfer](53-inventory-storage-transfer.md), [45-ban-risk-reference](45-ban-risk-reference.md).
- History: commits `4e800b9`, `cb1a822` on branch `chore/items-audit`.
