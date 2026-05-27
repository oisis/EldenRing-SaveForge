# 54 — Ash of War

> **Type**: Design doc
> **Status**: ✅ Implemented
> **Scope**: Canonical chapter for Ash of War — save semantics, sentinels, two write paths (strict vs allocation), the allocation guard, availability scan, compatibility model, workspace/WeaponEditModal semantics, and known gaps requiring verification.

---

## 1. Chapter purpose

This chapter ties together all the layers that handle assigning / detaching Ash of War in SaveForge:

- the on-disk format (relation weapon GaItem ↔ AoW GaItem, "no custom AoW" sentinels),
- the allocation model and its non-trivial constraints (the guard concerning the armament zone),
- the write paths (`PatchWeaponAoWHandle` in-place handle reuse, `PatchWeaponAoW` allocates + rebuild),
- availability scanning and shared-handle detection,
- the compatibility model and mount semantics,
- the workspace path (RAM-only) from the perspective of the `WeaponEditModal` modal.

Reference chapters whose contents we do **not** repeat in 54:

- [03-gaitem-map](03-gaitem-map.md) — GaItem binary layout (handle prefix, maps, 8B vs 21B record).
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — full allocator invariants (`NextAoWIndex`, `NextArmamentIndex`, NextGaItemHandle), post-mutation validation rules, capacity rules.
- [43-transactional-item-adding](43-transactional-item-adding.md) — the transactional Add Items path, post-add hooks (incl. the AoW acquisition flag).
- [36-inventory-categories-game-order](36-inventory-categories-game-order.md) — the AoW category in the inventory mapping.
- [06-equipment](06-equipment.md) — the read-only equipment model and transfer guards.
- [55-build-template](55-build-template.md) — portable JSON snapshot (carrying AoW as a semantic ItemID, not as a handle).

---

## 2. Status

| Layer | Status |
|---|---|
| Reader (`core.SaveSlot.parseFromData`) | ✅ accepts both sentinels (`0x00000000` and `0xFFFFFFFF`). |
| Canonical writer (`core.PatchWeaponAoWHandle`) | ✅ canonicalizes output to `0x00000000`, guards shared-handle. |
| Allocation writer (`core.PatchWeaponAoW`) | ✅ allocates a new AoW + rebuilds the slot (active AoW-set path of the workspace save). |
| Allocator guard | ✅ guard `NextArmamentIndex >= maxEntries` in `allocateGaItem` (commit `6881cb9`). |
| Availability scan (`core.ScanAoWAvailability`) | ✅ two-pass, detects shared-handle. |
| Compatibility check (`db.IsAshOfWarCompatibleWithWeapon`) | ✅ with the `canMountWep_*` bitmask + `WepTypeToCanMountBit`. |
| Workspace AoW (`editor.UpdateWeapon` + `EditableItem` pending fields) | ✅ pending pattern with "fail-closed on unknown compat" validation at save. |
| Frontend modal (`WeaponEditModal.tsx`) | ✅ workspace-only path (the former non-workspace/legacy mode was removed). |
| DLC wepTypes 69/94/95 | ⚠️ DB lookup returns `known==false`; the active workspace save fail-closes, so these AoW edits are currently refused — `needs verification`. |

---

## 3. Source of truth in code

| Area | Files / symbols |
|---|---|
| Format constants | `backend/core/structures.go`: `ItemTypeWeapon`, `ItemTypeAow`, `GaHandleTypeMask`, `NoCustomAoWHandle`, `LegacyNoCustomAoWHandle`, `IsNoCustomAoWHandle`, `GaItemFull.AoWGaItemHandle`. |
| Record sizes | `backend/core/offset_defs.go`: `GaRecordWeapon = 21`, `GaRecordItem = 8`, `GaHandleTypeMask = 0xF0000000`. |
| Allocator + guard | `backend/core/writer.go`: `allocateGaItem` (lines ~430–501), guard lines 461–462. |
| Strict patch | `backend/core/writer.go`: `PatchWeaponAoWHandle` (lines ~1131–1207). |
| Allocation + rebuild | `backend/core/writer.go`: `PatchWeaponAoW` (lines ~1209–1325). |
| Availability scan | `backend/core/aow_availability.go`: `ScanAoWAvailability`, `AoWCopyRaw`. |
| Compat data | `backend/db/data/aow_compat.go`: `AoWCompatMasks`, `WepTypeToCanMountBit`, `CanMountWepNames`. |
| Weapon mount data | `backend/db/data/weapon_gem_mount.go`: `WeaponGemMounts`. |
| Compat helpers | `backend/db/db.go`: `CanWeaponMountAoW`, `IsAoWCompatibleWithWepType`, `IsAshOfWarCompatibleWithWeapon`. |
| App entrypoints | `app.go`: `GetAoWAvailability` (availability scan). The former direct-write endpoints `ApplyWeaponAoW` / `ApplyWeaponAoWStrict` (and `ApplyWeaponInfusion` / `ApplyWeaponUpgradeLevel`) were removed — AoW/weapon writes now run only through the workspace save. |
| Editor patch DTO | `backend/editor/weapon.go`: `WeaponPatch`, `UpdateWeapon`. |
| Editor workspace state | `backend/editor/workspace.go`: `EditableItem.Current*`/`Pending*`/`CanMountAoW`/`WepType`, constants `AoWStatus*`. |
| Save-side execution | `backend/editor/save.go`: `collectPendingAoWChanges`, `validatePendingAoWChanges`, `executePendingAoWPatches`. |
| Workspace validator | `backend/editor/validate.go`: `CodePendingAoWUnknown`, `CodePendingAoWConflict`. |
| Frontend modal | `frontend/src/components/WeaponEditModal.tsx` (workspace-only path, its own `WEP_TYPE_TO_BIT` mirror). |
| Workspace integration | `frontend/src/components/SortOrderTab.tsx` (injects `workspace` + `workspaceItem` into `WeaponEditModal` — see §16). |

---

## 4. Mental model

Every weapon has a skill visible in-game as an **Ash of War**. It comes from one of two sources:

- **Built-in weapon skill** — `EquipParamWeapon.swordArtsParamId` in `regulation.bin`. Always available, never stored in the save.
- **Custom Ash of War gem** — an inventory item with its own GaItem (handle prefix `0xC0…`). When mounted it overrides the built-in skill with its own `EquipParamGem.swordArtsParamId`.

Three `regulation.bin` parameters are the reference layer. SaveForge **never** modifies them:

- `EquipParamWeapon.swordArtsParamId` — fallback skill.
- `EquipParamGem.swordArtsParamId` + `canMountWep_*` — the gem's skill and 36-bit compatibility mask.
- `SwordArtsParam` — skill definitions (animations, costs, in-game name).

The consequences of overlooking this distinction (and the bugs that historically followed):

- Treating "no custom AoW" as "no skill at all" misled the user — removing a custom AoW restores the built-in skill, it does not erase it. Fixed in commit `cb1a822`.
- Treating two copies of a custom-AoW of the same ItemID as one instance led to false "in use" in the UI or to the more dangerous shared-handle crash (§10).

---

## 5. Weapon GaItem ↔ AoW GaItem relation

Binary layout is described in [03-gaitem-map](03-gaitem-map.md). Here only the AoW-specific part:

| Element | Value |
|---|---|
| Weapon GaItem handle prefix | `0x80000000` (`ItemTypeWeapon`). |
| AoW GaItem handle prefix | `0xC0000000` (`ItemTypeAow`). |
| Weapon GaItem size | `GaRecordWeapon = 21` bytes. |
| AoW GaItem size | `GaRecordItem = 8` bytes (same as other stackables). |
| Offset of the `AoWGaItemHandle` field in the weapon GaItem | `[+0x10:+0x14]` (u32 LE). |
| AoW gem ItemID prefix | upper nibble `0x8` (e.g., Lion's Claw = `0x80002710`). |

The relation is **handle-based**: the weapon GaItem references the mounted gem by handle, not by ItemID. The same ItemID may have multiple separate copies (different handles) — each physically exists in `slot.GaItems` (e.g., scarab drops, Lost Ashes of War).

```
weapon GaItem (21B)               AoW GaItem (8B)
+------+-------------+      +---->+--------+--------+
|  ... | Handle      |      |     | Handle | ItemID |
| [0x00] 0x80xxxxxx  |      |     | 0xC0.. | 0x80.. |
|  ... | ItemID      |      |     +--------+--------+
| [0x10] AoWGaItemHandle ---+
|       (u32: 0x00000000     resolved via slot.GaMap[handle] → ItemID
|        sentinel OR 0xC0…)
+--------------------+
```

The game's resolver on load:

```
if !IsNoCustomAoWHandle(weapon.AoWGaItemHandle):
    gem    = GaMap[weapon.AoWGaItemHandle]
    skill  = EquipParamGem[gem.ItemID].swordArtsParamId
else:
    skill  = EquipParamWeapon[weapon.ItemID].swordArtsParamId
```

---

## 6. AoWGaItemHandle and sentinel values

The `AoWGaItemHandle` field in the weapon GaItem carries one of four classes of value:

| Value | Meaning | Writer | Reader |
|---|---|---|---|
| `0x00000000` | No custom AoW — the **canonical vanilla sentinel** (the game writes this value for every freshly created weapon). | ✅ canonicalized to this value | ✅ accepted |
| `0xFFFFFFFF` | No custom AoW — the **legacy SaveForge sentinel** emitted by builds before commit `4e800b9`. | ❌ never emitted | ✅ accepted for compat. |
| `0xC0xxxxxx` (prefix `ItemTypeAow`) | Valid custom AoW handle. Must resolve to an existing AoW GaItem in the slot. | ✅ written on attach | ✅ accepted |
| Anything else | Invalid / corrupted. | n/a | flagged a layer up |

Constants and helper:

```go
// backend/core/structures.go
const NoCustomAoWHandle       uint32 = 0x00000000
const LegacyNoCustomAoWHandle uint32 = 0xFFFFFFFF

func IsNoCustomAoWHandle(h uint32) bool {
    return h == NoCustomAoWHandle || h == LegacyNoCustomAoWHandle
}
```

Accepting both sentinels allows opening saves edited by older SaveForge releases without re-flagging every weapon. Emitting a single canonical sentinel keeps freshly saved files indistinguishable from vanilla output at this offset (anti-flag).

---

## 7. AoW item data and categories

AoW gems are entered into the SaveForge DB with the category `ashes_of_war` (see [36-inventory-categories-game-order](36-inventory-categories-game-order.md)). Workspace edits validate the category in `editor.UpdateWeapon`:

```go
aow, _ := db.GetItemDataFuzzy(patch.AoWItemID)
if aow.Name == "" { return ErrUnknown }
if aow.Category != "ashes_of_war" { return ErrWrongCategory }
```

Save-side validation (`validatePendingAoWChanges`) repeats the same check defense-in-depth. The workspace validator (`validate.go`) has a separate error code `CodePendingAoWUnknown` for direct field mutations that bypassed `UpdateWeapon`.

---

## 8. Compatibility model

Compatibility (AoW × weapon) has three gate levels computed in `backend/db/db.go`:

### 8.1. Per-weapon: `CanWeaponMountAoW`

```go
func CanWeaponMountAoW(baseItemID uint32) bool {
    return GetItemData(baseItemID).GemMountType == 2
}
```

`GemMountType == 2` = "standard infusable" (e.g., Longsword); `1` = "special/somber" (e.g., Sword of Night and Flame); `0` = no mount.

### 8.2. Per-AoW × wepType: `IsAoWCompatibleWithWepType`

```go
func IsAoWCompatibleWithWepType(aowItemID uint32, wepType uint16) (compatible, known bool) {
    aow := GetItemData(aowItemID)
    if aow.AoWCompatBitmask == 0 { return false, false }
    bitPos, ok := data.WepTypeToCanMountBit[wepType]
    if !ok { return false, false }
    return (aow.AoWCompatBitmask>>bitPos)&1 == 1, true
}
```

The 36-bit bitmask comes from `EquipParamGem.canMountWep_*` (columns from `Dagger` to `Torch` — full list in `data.CanMountWepNames`). `WepTypeToCanMountBit` maps `EquipParamWeapon.wepType` to the bit position.

The signature `(compatible, known)`:

- `known == false` → SaveForge **does not have data** sufficient to assess (missing bitmask or wepType outside the map). The caller decides: block (fail-closed) or passthrough.
- `known == true` → a binary verdict.

### 8.3. Combined: `IsAshOfWarCompatibleWithWeapon`

```go
func IsAshOfWarCompatibleWithWeapon(aowItemID uint32, weaponItemID uint32) (compatible, known bool) {
    wep, _ := GetItemDataFuzzy(weaponItemID)
    if wep.GemMountType != 2 { return false, true }     // somber / no-mount
    if wep.WepType == 0      { return false, false }   // wepType unknown
    return IsAoWCompatibleWithWepType(aowItemID, wep.WepType)
}
```

`GetItemDataFuzzy` returns data for the baseID (after stripping the infusion offset and upgrade level), so `wp+15 Cold` and `wp+0` resolve to the same base.

### 8.4. Call rules (who fail-closes vs fail-opens)

| Caller | `known == false` | `known && !compatible` |
|---|---|---|
| `editor.validatePendingAoWChanges` (workspace save) | **fail-closed** (`refusing fail-closed`) | block |
| `WeaponEditModal` (UI default view) | hide (fail-closed visibility) | hide |

The active write path is the workspace save, and it is uniformly fail-closed on `known == false`: an AoW × weapon combination SaveForge cannot assess is refused at save (templates and auto-apply can introduce combinations the UI never presented). The UI mirrors this by hiding `unknown`/`incompatible` entries. A previous generation of direct-write endpoints fail-opened (passthrough) on `known == false`, but those endpoints were removed; no passthrough path remains.

---

## 9. Weapon gem mount semantics

`backend/db/data/weapon_gem_mount.go` is generated by `tmp/scripts/import_aow_compat.py` from `EquipParamWeapon.csv`. The keys cover base variants (upgrade 0) and infusion variants (+100, +200, …). Only weapons with `gemMountType != 0` are in the map. `db.go` merges this into `globalItemIndex`, setting `entry.GemMountType` and `entry.WepType`.

| `gemMountType` | Meaning | UI / writer |
|---|---|---|
| `0` | No mount (e.g., catalyst, torch in some cases) | the weapon is not AoW-editable; the modal does not show the AoW section |
| `1` | Special / somber (the skill is fixed in `EquipParamWeapon`, the gem cannot change it) | `CanMountAoW == false`; the modal may be open, but AoW actions are disabled |
| `2` | Standard infusable | `CanMountAoW == true`; the full AoW path is available |

`needs verification`: in the current code `EditableItem.CanMountAoW = (itemData.GemMountType == 2)` — `gm == 1` is treated as "AoW cannot be changed", but **does not** disable affinity/upgrade modifications. The document does not confirm all edge cases (e.g., whether the UI shows information that this is a `gm==1` weapon).

---

## 10. Availability scanning

`core.ScanAoWAvailability(slot *SaveSlot) []AoWCopyRaw` is a single two-pass scan over `slot.GaItems`.

### 10.1. Pass 1 — collect AoW + weapon→AoW references

```go
for i := range slot.GaItems {
    g := &slot.GaItems[i]
    if g.IsEmpty() { continue }
    switch g.Handle & GaHandleTypeMask {
    case ItemTypeAow:
        copies = append(copies, AoWCopyRaw{ItemID: g.ItemID, Handle: g.Handle})
    case ItemTypeWeapon:
        if !IsNoCustomAoWHandle(g.AoWGaItemHandle) {
            weaponRefs[g.AoWGaItemHandle] = append(weaponRefs[g.AoWGaItemHandle], g.Handle)
        }
    }
}
```

### 10.2. Pass 2 — used / free / shared

```go
for i := range copies {
    weapons := weaponRefs[copies[i].Handle]
    if len(weapons) == 0 { continue }       // free copy
    copies[i].UsedByWeaponHandle = weapons[0]
    if len(weapons) > 1 {
        copies[i].HasSharedHandleConflict = true
    }
}
```

### 10.3. Aggregate per ItemID (`app.GetAoWAvailability`)

`app.go::GetAoWAvailability` aggregates the result into `vm.AoWAvailabilityEntry`:

| Field | Meaning |
|---|---|
| `TotalCopies` | Number of copies of this ItemID in the slot. |
| `AvailableCopies` | Copies that no weapon references. |
| `UsedCopies` | Copies referenced by at least one weapon. |
| `UsedByWeaponHandles` | List of weapon handles (first ref per copy). |
| `IsMissing` | Always `false` from app.go (the UI treats a missing entry as missing). |
| `HasSharedHandleConflict` | OR over all copies. |

The UI maps this to five statuses:

| Status | Condition |
|---|---|
| `current` | ItemID == currentAoWId of the edited weapon. |
| `available` | `availableCopies > 0`. |
| `in_use` | All copies referenced by weapons. |
| `missing` | No entry (`!aowAvailability.has(id)` in the UI). |
| `conflict` | `hasSharedHandleConflict == true`. |

The scan does **not** check compatibility (`canMountWep_*`) nor `gemMountType`. These are independent layers.

---

## 11. Shared-handle conflicts

A single AoW gem handle identifies one physical instance. Two different weapons can **never** reference the same non-sentinel `AoWGaItemHandle`. A violation → `EXCEPTION_ACCESS_VIOLATION` in the game on load.

Three paths enforce the invariant:

| Path | Enforcement |
|---|---|
| `ScanAoWAvailability` | Detects a multi-referenced handle (`len(weapons) > 1`), marks both copies `HasSharedHandleConflict`. |
| `PatchWeaponAoWHandle` (strict) | Scans the slot and rejects the attach if another weapon GaItem already references this handle. |
| `PatchWeaponAoW` (legacy) | `generateUniqueHandle(slot, ItemTypeAow)` mints a fresh handle for each call — never reuses an existing one. |

What is legal (and often confuses):

- The same AoW **ItemID** may have multiple separate copies (different handles). This is exactly how Lost Ashes of War and drops accumulate.
- Two different weapons may both show the same Ash of War in-game, as long as they reference **different** gem handles of the same ItemID.

The strict path UI presents the `conflict` status in the AoW listing and blocks the apply of that copy (rather than both weapons). At the writer level, `PatchWeaponAoWHandle` itself refuses an attach whose target handle is already referenced by another weapon GaItem (shared-handle guard, §13).

---

## 12. Write paths overview

SaveForge has **two** AoW write paths at the core level. Both are now reached only through the workspace save; the choice between them is a function of clear-vs-set:

| Path | Core function | Entry point | UI |
|---|---|---|---|
| **Strict** (in-place, no rebuild) | `core.PatchWeaponAoWHandle` | `editor.executePendingAoWPatches` — clear (`PendingAoWClear`) | `WeaponEditModal` (Sort Order) |
| **Allocate + rebuild** | `core.PatchWeaponAoW` | `editor.executePendingAoWPatches` — set (`PendingAoWItemID`) | `WeaponEditModal` (Sort Order) |

Both are invoked by `executePendingAoWPatches`, which `ApplyWorkspaceSave` runs at save time.

The "strict vs allocate" choice has operational significance:

- **Strict** requires an existing free copy of an AoW GaItem of the desired ItemID — the UI must confirm this via the availability scan.
- **Allocate** always mints a fresh handle and allocates a new AoW GaItem; it requires a full `RebuildSlotFull` and reparse — more expensive and subject to the guard from §14.

The workspace save bridges both paths: clear → strict (in-place sentinel), set → allocate (allocates a new GaItem). Consequence: every change with `PendingAoWClear=true` is cheap; a change with `PendingAoWItemID != 0` rebuilds the slot.

---

## 13. Strict patch path

`core.PatchWeaponAoWHandle(slot, weaponHandle, newAoWHandle) error`

Contract:

1. Locates the weapon GaItem by `weaponHandle` and its byte offset. Missing → error.
2. If the record type ≠ `ItemTypeWeapon` → error.
3. If `IsNoCustomAoWHandle(newAoWHandle)` → canonicalizes to `NoCustomAoWHandle` and writes it at offset `[+0x10]`. (Accepts both sentinels on input, emits only `0x00000000`.)
4. Otherwise:
   - the prefix of `newAoWHandle` must be `ItemTypeAow` → otherwise error,
   - an AoW GaItem with this handle must exist in the slot → otherwise error,
   - no **other** weapon GaItem may reference this handle (shared-handle guard) → otherwise error.
5. Writes 4 bytes at `[weaponOff+16]`. Mutates `slot.GaItems[idx].AoWGaItemHandle`.

Does not allocate, does not rebuild, does not modify other offsets. The entire operation is **exactly 4 bytes** of change on disk.

`Weapon.ItemID` is **never** touched — affinity and upgrade level survive every attach/detach (see the test `TestPatchWeaponAoWHandle_RemovePreservesWeaponItemID`).

---

## 14. Allocation / rebuild path

`core.PatchWeaponAoW(slot, weaponHandle, newAoWItemID) error`

Two modes:

### 14.1. Remove (`newAoWItemID == 0`)

Identical effect to strict-remove: writes `NoCustomAoWHandle` at `[weaponOff+16]`. No allocation, no rebuild.

### 14.2. Set (`newAoWItemID != 0`)

1. Validates the ItemID upper nibble == `0x8`.
2. `generateUniqueHandle(slot, ItemTypeAow)` → a fresh `0xC0…` handle.
3. `allocateGaItem(slot, newAoWHandle, newAoWItemID)` — inserts the AoW record at position `NextAoWIndex`, advances `NextAoWIndex` and `NextArmamentIndex` (see §15).
4. `slot.GaMap[newAoWHandle] = newAoWItemID`.
5. `upsertGaItemData(slot, newAoWItemID)` — registers the ItemID in the GaItemData section (if it does not already exist).
6. Snapshots `NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`.
7. `RebuildSlotFull(slot)` + `parseFromData()` — the GaItems section grew by 8B, every downstream offset is recomputed.
8. Restores the indexes if `parseFromData` underscanned.
9. Re-locates the weapon GaItem by `weaponHandle` (the byte offset may have shifted after the rebuild).
10. Writes `newAoWHandle` at `[weaponOff+16]`.

The old AoW GaItem (previously referenced) is **not** garbage-collected. The game tolerates orphans (orphan copies) in the GaMap; the strict path can re-attach them later without allocation.

This is the active AoW-set path: it is invoked by the workspace save (`editor.executePendingAoWPatches`) whenever a pending AoW Set (`PendingAoWItemID != 0`) requires a fresh GaItem — see §16.3.

---

## 15. AoW Allocation Safety (guard `6881cb9`)

The allocator `allocateGaItem` has one AoW-specific guard, whose full capacity-rules context is in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md). Here we describe only the AoW consequence.

### 15.1. Rule

AoW insertion **unconditionally** advances `NextArmamentIndex` (the right edge of the armament zone). If `NextArmamentIndex` already equals `len(slot.GaItems)`, the advance would push it beyond the array, breaking the post-mutation validator's invariant (`NextArmamentIndex > len(GaItems)`).

```go
// backend/core/writer.go:445–479 (the isAoW branch in allocateGaItem)
idx := slot.NextAoWIndex
if idx >= maxEntries {
    return error("AoW array full")
}
if slot.NextArmamentIndex >= maxEntries {
    return error("cannot insert AoW — armament zone at capacity (NextArmamentIndex %d == %d)")
}
// ... shift/insert ...
slot.NextAoWIndex++
slot.NextArmamentIndex++ // AoW insertion shifts armament zone right
```

### 15.2. Why, historically

Before commit `6881cb9` the AoW branch checked only `NextAoWIndex < maxEntries`, but also incremented `NextArmamentIndex`. PS4 saves observed in the field (slot 1 "Bydlaczka") had `NextAoWIndex=3` (room in the AoW zone), but `NextArmamentIndex == len(GaItems)` (the highest-indexed non-empty entry sat at position `maxEntries-1`). Every AoW add caused an overflow and `ValidatePostMutation` reported `"NextArmamentIndex N > len(GaItems) N"` — a numeric message, not informing the user of the real cause.

### 15.3. What the guard guarantees

The reject is **before-mutation**: neither `NextAoWIndex`, nor `NextArmamentIndex`, nor `slot.GaItems[idx]` are touched. `ValidatePostMutation` passes cleanly for the pre-call state. The test `TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity` (`backend/core/gaitem_placement_test.go:324`) locks:

- `slot.NextAoWIndex` stays `3`,
- `slot.NextArmamentIndex` stays `8` (== `maxEntries`),
- `slot.GaItems[3]` is still `IsEmpty()`,
- `ValidatePostMutation` → 0 violations.

### 15.4. What the guard does NOT guarantee

- It does not protect against a situation where a weapon/armor add fills the last slot — that is covered by the `!isAoW` branch ("armament/armor array full").
- It does not check whether there is a gap between `NextAoWIndex` and `NextArmamentIndex` (the shift-right logic handles that independently).
- The full capacity rules (NextEquipIndex, NextAcquisitionSortId, NextGaItemHandle) — described in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).

---

## 16. Workspace and WeaponEditModal state

WeaponEditModal is **workspace-only**. The `workspace` + `workspaceItem` props are required; the former non-workspace/legacy mode (and its `GetCharacter` fallback) was removed.

| Mode | Props | Read state | Write state |
|---|---|---|---|
| Workspace (only) | `workspace` + `workspaceItem` | `EditableItem` (RAM, pending fields included) | `workspace.updateWeapon(uid, patch)` → mutates the RAM snapshot |

### 16.1. Workspace read path

```typescript
const [currentAoWId, setCurrentAoWId]   = useState(workspaceItem?.currentAoWItemID ?? 0);
const [canMountAoW, setCanMountAoW]     = useState(workspaceItem?.canMountAoW ?? false);
const [wepType, setWepType]             = useState(workspaceItem?.wepType ?? 0);
```

After each `workspace.updateWeapon(...)` the returned fresh `EditableItem` synchronizes the state via `useEffect`.

### 16.2. Why the modal does NOT use `GetCharacter`

The AoW-mount metadata (`currentAoWId` / `canMountAoW` / `wepType`) comes straight from the editable workspace item. `GetCharacter` reads the *save* state, which can drift from the workspace, so the modal never relies on it:

- An item added in the workspace (Source=Added) **does not yet have** a real `OriginalHandle` in the save — `GetCharacter` returns a snapshot without that item.
- Recent saves may re-allocate handles on `core.PatchWeaponAoW` (slot rebuild) — the `GetCharacter` cache may return the old handle.
- Pending edits (`PendingAoWItemID`, `PendingAoWClear`) are exclusively in the workspace RAM — `GetCharacter` does not see them.

### 16.3. Workspace write path

`workspace.updateWeapon(uid, patch)` → `editor.UpdateWeapon`:

| `WeaponPatch` field | Effect |
|---|---|
| `SetAoWItemID = true, AoWItemID != 0` | Validates the DB (category `ashes_of_war`), sets `PendingAoWItemID` + `PendingAoWName`, zeroes `PendingAoWClear`. |
| `SetAoWItemID = true, AoWItemID == 0` | Treated like `ClearAoW` — `PendingAoWClear = true`. |
| `ClearAoW = true` | `PendingAoWClear = true`, zeroes `PendingAoWItemID`/`PendingAoWName`. |
| Any of the above | `HasPendingWeaponPatch = true`, `snap.Dirty = true`, re-validation. |

Save-time validation (`editor.validatePendingAoWChanges`) **additionally** checks compatibility and is **fail-closed on unknown**:

```
if !known {
    return error("AoW/weapon compatibility unknown ... refusing fail-closed")
}
if !compatible {
    return error("AoW ... is not compatible with weapon ...")
}
```

Execution (`executePendingAoWPatches`):

- `Clear` → `core.PatchWeaponAoWHandle(slot, handle, NoCustomAoWHandle)` (strict, in-place).
- `Set`  → `core.PatchWeaponAoW(slot, handle, c.AoWItemID)` (legacy, alloc + rebuild).

### 16.4. Workspace AoW status (`EditableItem.CurrentAoWStatus`)

Constants in `backend/editor/workspace.go`:

| Value | Meaning |
|---|---|
| `AoWStatusNone = "none"` | No custom AoW (sentinel handle). |
| `AoWStatusCustom = "custom"` | Custom AoW resolved to a known DB ItemID. |
| `AoWStatusMissing = "missing"` | Handle non-sentinel, but does not resolve to an AoW GaItem (orphan / dangling). |
| `AoWStatusShared = "shared"` | Handle referenced by >1 weapon (save corruption). |

`needs verification`: the full population path of `CurrentAoW*` is in `populateCurrentAoW` + `buildWeaponAoWMaps` (`workspace.go:367+`). Edge cases (e.g., an Added item without a handle, a Removed item with a pending change waiting) have been verified by the tests `current_aow_test.go`, but not all combinations are described here — we do not repeat the full matrix.

---

## 17. Frontend / backend compatibility drift

The UI maintains a single frontend mirror of `WepTypeToCanMountBit`, in `WeaponEditModal.tsx` (lines ~48–54). It is today identical to `backend/db/data/aow_compat.go::WepTypeToCanMountBit`, but it is maintained by hand — no generator or CI test enforces consistency:

```typescript
// frontend/src/components/WeaponEditModal.tsx
const WEP_TYPE_TO_BIT: Record<number, number> = {
    1: 0, 3: 1, 5: 2, 7: 3, 9: 8, 11: 9, 13: 6, 14: 5, 15: 4, 16: 7, 17: 7,
    19: 11, 21: 13, 23: 10, 24: 10, 25: 12, 28: 14, 29: 14, 31: 15, 32: 17,
    33: 18, 35: 20, 37: 19, 39: 20, 41: 21, 43: 22, 50: 23, 51: 24, 52: 25,
    53: 26, 54: 27, 55: 28, 57: 29, 61: 30, 65: 32, 66: 33, 67: 34, 68: 35,
    87: 25, 88: 25, 89: 26, 90: 27, 91: 26, 92: 26, 93: 26,
};
```

**Drift risk**: the backend `data.WepTypeToCanMountBit` is the source of truth (see `backend/db/data/aow_compat.go:245–291`). Every backend-side update (e.g., a new DLC wepType) must be propagated **manually** to the frontend mirror; there is no CI guard and no generator. Until a shared frontend helper or generator exists, every such change is `needs verification` in `WeaponEditModal.tsx`.

The UI fail-closes on an unknown `wepType`:

```typescript
function getAoWCompatStatus(aowCompatBitmask: number, wepType: number) {
    if (aowCompatBitmask === 0 || wepType === 0) return 'unknown';
    const bitPos = WEP_TYPE_TO_BIT[wepType];
    if (bitPos === undefined) return 'unknown';
    // …
}
```

`unknown` is blocked in the default view (`Show unavailable = false`). After enabling the `Show unavailable` toggle, `unknown`/`incompatible` entries are visible, but **not applicable** (`canApplyAoW` requires `compat === 'compatible'`).

Contrast across the active layers:

| Layer | `known == false` behavior |
|---|---|
| `editor.validatePendingAoWChanges` (workspace save) | Fail-closed. |
| `WeaponEditModal` UI | Hide / disable. |

Both active layers fail-closed on unknown compatibility — the UI hides the entry and the workspace save refuses it. This is deliberate, because templates and auto-apply can introduce combinations the UI never confirmed. (A previous generation of direct-write endpoints fail-opened on `known == false`; they were removed, so no passthrough path remains.)

`needs verification`: the bitmask may be incomplete for DLC AoW gems (rows in `EquipParamGem` not covered by the `import_aow_compat.py` import). Affinity gating for infusion variants (e.g., Heavy Longsword vs Standard Longsword) is **not** handled by the bitmask — `EquipParamWeapon.defaultWepAttr` / `configurableWepAttr00..23` are not imported into `WeaponGemMounts`. No evidence was found that affinity gating is enforced in SaveForge.

---

## 18. Relationship to Equipment

Equipment ([06-equipment](06-equipment.md)) is read-only from the AoW perspective: the equipped weapon handle points to a weapon GaItem in `slot.GaItems` whose `AoWGaItemHandle` is mutated by the §13/§14 paths independently. The equipment slot **does not** hold any reference to the AoW.

Consequences:

- Editing the AoW on an equipped weapon works without touching the equipment section.
- Transferring a weapon between inventory ↔ storage ([53-inventory-storage-transfer](53-inventory-storage-transfer.md)) **preserves** `OriginalHandle` and `AoWGaItemHandle` — the AoW moves with the weapon.
- Removing a weapon from the inventory **does not** delete the mounted AoW GaItem — it becomes an orphan.

---

## 19. Relationship to Build Templates

Build Templates ([55-build-template](55-build-template.md)) export the AoW as a **portable ItemID**, not as a handle:

- The template field `aowItemID` is a pointer + `omitempty` — omitted means "no custom AoW".
- The exporter **never** writes a save-local handle (`OriginalHandle`, `AoWGaItemHandle`, `CurrentAoWHandle`).
- The import preview computes AoW compatibility via `db.IsAshOfWarCompatibleWithWeapon` and **fail-closes on unknown** — rejects the template if the target save has a weapon `wepType` outside `WepTypeToCanMountBit` or an AoW outside `AoWCompatMasks`.
- Apply bridges through the workspace `EditableItem.PendingAoWItemID`, i.e., uses the §16 path. Pending Set → save → `core.PatchWeaponAoW` (allocates a new AoW GaItem).

The full description (schema, phases A–E, library) — in `55-build-template.md` (canonical rewrite in the next step).

---

## 20. Validation and safety notes

| Layer | Rule |
|---|---|
| Reader | Accepts `0x00000000` and `0xFFFFFFFF` as "no custom AoW". |
| Writer (strict + legacy) | Emits only `0x00000000` for no-custom. |
| Writer (strict) | Rejects an attach handle that is neither a sentinel nor a `0xC0` prefix. |
| Writer (strict) | Rejects an attach handle that does not point to an existing AoW GaItem. |
| Writer (strict) | Rejects an attach handle already referenced by another weapon (shared-handle). |
| Writer (legacy set) | Mints a **new** handle for every attach — never reuses an existing one. |
| Allocator | Rejects an AoW add if `NextArmamentIndex == len(GaItems)` (§15). |
| Editor (UpdateWeapon) | Rejects an AoW ItemID outside the DB or outside the `ashes_of_war` category. |
| Editor (validate) | `CodePendingAoWUnknown`, `CodePendingAoWConflict` as defense-in-depth. |
| Editor (save) | Fail-closed on unknown compat (`refusing fail-closed`). |
| UI modal | By default hides `incompatible` and `unknown`; an explicit toggle shows them but does not allow applying. |
| Weapon ItemID | **Never** modified by AoW operations — affinity and upgrade level are invariants. |

Anti-patterns the document must NOT document as "safe":

- ❌ Sharing an AoW handle between weapon entries (game crash).
- ❌ Cloning a handle from save A to save B (the handle is save-local; it has no meaning in another slot).
- ❌ Fail-open on unknown compat in the workspace save path (the workspace fail-closes intentionally).
- ❌ Affinity gating per AoW (`defaultWepAttr`/`configurableWepAttr00..23`) — `needs verification`, not implemented.

---

## 21. Test coverage

| Test file | What it locks |
|---|---|
| `backend/core/aow_strict_test.go::TestIsNoCustomAoWHandle` | The helper accepts both sentinels. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoWHandle_RemoveWritesZeroSentinel` | Strict remove emits `0x00000000`. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoWHandle_RemovePreservesWeaponItemID` | The weapon ItemID survives a remove. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoWHandle_AoWAlreadyUsed` | Shared-handle guard in strict. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoWHandle_AttachFreeHandle` | Attaching a free copy works. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoW_LegacyRemoveWritesZeroSentinel` | Legacy remove emits `0x00000000`. |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_FreeAndUsedCopies` | The scan distinguishes free vs used. |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_SharedHandleConflict` | The scan flags shared-handle. |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_ZeroSentinelNotCounted` | The `0x00000000` sentinel does not count as usage. |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_LegacyFFFFFFFFSentinelNotCounted` | The `0xFFFFFFFF` sentinel does not count as usage. |
| `backend/core/aow_strict_test.go::TestAllocateGaItem_NewWeaponUsesZeroSentinel` | The allocator initializes a weapon GaItem with `NoCustomAoWHandle`. |
| `backend/core/gaitem_placement_test.go::TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity` | Guard `6881cb9` (§15). |
| `backend/core/aow_dual_destination_test.go::TestNonStackableDualDestinationUniqueHandles` | Adding an AoW to inv + storage creates two separate handles. |
| `backend/editor/current_aow_test.go::*` | Population of `CurrentAoW*` after scan + pending flow. |
| `backend/editor/weapon_test.go::*` | `WeaponPatch` semantics (SetAoWItemID, ClearAoW, HasPendingWeaponPatch). |
| `backend/editor/save_test.go::TestValidatePendingAoWChanges_*` | Fail-closed on unknown compat, reject non-AoW category, accept clear. |
| `backend/db/compat_test.go::TestIsAshOfWarCompatibleWithWeapon_DLCUnmappedWepType_Unknown` | Real DLC weapons with unmapped `wepType` (Dragon Towershield 69, Great Katana 94) resolve to `known=false` at the DB layer (fail-closed `compatible=false`). |
| `backend/db/compat_test.go::TestIsAshOfWarCompatibleWithWeapon_*` | Known-compatible / known-incompatible / non-mountable (`gemMountType != 2`) verdicts at the DB layer. |
| `backend/core/writer_weapon_itemid_test.go::*` | `PatchWeaponItemID` byte-patch contract (locate-by-handle, stale-data guard, in-place 4-byte ItemID overwrite). |
| `frontend/src/components/WeaponEditModal.workspace.test.tsx` | Workspace mode read/write with workspaceItem. |

---

## 22. Known limits / needs verification

| # | Area | Status |
|---|---|---|
| L1 | Affinity gating per AoW (`defaultWepAttr`/`configurableWepAttr00..23`) | `needs verification` — no gating path found in the code. The UI does not differentiate infusion variants on the compat check. |
| L2 | DLC wepType 69/94/95 | No data in `WepTypeToCanMountBit`, so the DB lookup returns `known=false` (locked by `backend/db/compat_test.go`). The UI treats them as `unknown` and the workspace save fail-closes, so an AoW edit on these weapons is currently refused. `needs verification` whether these wepTypes should be mapped to allow legitimate edits, and whether the UI explains "DLC, compatibility unknown" to the user. |
| L3 | `gemMountType == 1` (somber) AoW editing semantics | The UI sets `CanMountAoW = false` → the AoW section is disabled. `needs verification` whether there is a placeholder/explanation that this is not an error. |
| L4 | Frontend ↔ backend `WEP_TYPE_TO_BIT` drift | Single frontend mirror (`WeaponEditModal.tsx`), maintained by hand; currently identical with the backend. No CI guard / generator. `needs verification` on every backend change. |
| L5 | Compat bitmask completeness | `AoWCompatMasks` is generated from `EquipParamGem`; it is possible that new DLC rows were not re-imported. `needs verification` after a regulation update. |
| L6 | Orphan AoW GaItem garbage collection | Does not exist (intentionally). The game tolerates it, the strict path can re-attach. `needs verification` in long user-facing workflows (whether the save grows linearly with the number of AoW edits). |
| L7 | Workspace `populateCurrentAoW` edge cases | The full Added × Removed × Pending matrix is not described here — covered by tests in `current_aow_test.go`. `needs verification` for new sources/sinks. |

---

## 23. Cross-references

- [03-gaitem-map](03-gaitem-map.md) — GaItem binary model and handle prefix semantics.
- [06-equipment](06-equipment.md) — read-only equipment relation.
- [07-inventory](07-inventory.md) and [10-storage](10-storage.md) — inventory / storage section model.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — full capacity rules (`NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`, post-mutation validator).
- [36-inventory-categories-game-order](36-inventory-categories-game-order.md) — the `ashes_of_war` category in the inventory mapping.
- [43-transactional-item-adding](43-transactional-item-adding.md) — transactional add with AoW companion flags.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — AoW as non-stackable; sorting rules.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — transfer of a weapon with a mounted AoW.
- [55-build-template](55-build-template.md) — portable JSON snapshot, AoW as a semantic ItemID.
- [45-ban-risk-reference](45-ban-risk-reference.md) — anti-flag rationale for emitting the canonical sentinel.

---

## 24. Sources

- Code: `backend/core/structures.go`, `backend/core/offset_defs.go`, `backend/core/writer.go`, `backend/core/aow_availability.go`, `backend/db/db.go`, `backend/db/data/aow_compat.go`, `backend/db/data/weapon_gem_mount.go`, `backend/editor/weapon.go`, `backend/editor/save.go`, `backend/editor/validate.go`, `backend/editor/workspace.go`, `app.go`, `frontend/src/components/WeaponEditModal.tsx`, `frontend/src/components/SortOrderTab.tsx`.
- Tests: `backend/core/aow_strict_test.go`, `backend/core/aow_dual_destination_test.go`, `backend/core/gaitem_placement_test.go`, `backend/core/writer_weapon_itemid_test.go`, `backend/db/compat_test.go`, `backend/editor/current_aow_test.go`, `backend/editor/weapon_test.go`, `backend/editor/save_test.go`, `frontend/src/components/WeaponEditModal.workspace.test.tsx`.
- Game data: `tmp/regulation-bin-dump/csv/EquipParamWeapon.csv` (columns `swordArtsParamId`, `wepType`, `gemMountType`), `EquipParamGem.csv` (`swordArtsParamId`, `canMountWep_*`), `SwordArtsParam.csv`.
- History / forensic: commits `4e800b9` (`fix(aow): use vanilla no-custom sentinel`), `cb1a822` (`fix(ui): clarify custom Ash of War removal`), `6881cb9` (`fix(core): guard AoW allocation at armament capacity`), `f3d64c1` (`fix(inventory): restore AoW editing in workspace mode`), `0b62cfd` (`feat(inventory): save pending Ashes of War edits`), `8fcc97f` (`feat(inventory): expose current AoW in workspace`).
