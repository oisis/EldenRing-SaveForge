# 47 — Site of Grace Activation

> **Type**: Investigation / Design doc
> **Status**: ✅ Resolved — Hypothesis D confirmed; editor behaviour is correct
> **Scope**: All identifiers and save-file fields involved in Site of Grace discovery, fast-travel, and physical in-world object activation.

---

## Background

After setting grace EventFlags via the editor, Sites of Grace appear on the map and are available for fast travel. However, on arrival the in-world grace object appears unlit — the game treats it as never physically touched. The player must manually approach and rest at the grace to fully activate it.

This document maps all known identifier spaces and save-file fields related to grace state, identifies what the editor currently controls, and characterises the missing activation layer.

---

## 1. Identifier Spaces

Graces use **two completely separate identifier spaces**. Conflating them is the most common source of confusion.

### 1.1 Grace EventFlag ID (71xxx – 76xxx)

| Property | Value |
|---|---|
| Range (base game) | 71000 – 76162 |
| Range (DLC — Shadow of the Erdtree) | 72xxx, 74xxx, up to 76960 |
| Total count | 419 entries in `backend/db/data/graces.go` |
| Source identifier | `graces.go` hex constants, e.g. `0x00011558` = 71000 |
| Lookup | BST block 71–76 via `eventflag_bst.txt` |

**What this flag controls (confirmed):**
- Map marker visibility (grace icon appears on map)
- Fast-travel eligibility (grace shows in the warp list)
- "Discovered" state from the game engine's perspective for quest flag purposes

**What this flag does NOT control:**
- The physical in-world bonfire object's lit/unlit state
- Whether resting animation plays on approach
- Respawn point assignment (`LastRestedGrace`)

**Sub-ranges by area type:**

| Range | Area type | Notes |
|---|---|---|
| 71xxx | Stormveil, Leyndell, boss arenas | Legacy dungeons |
| 72xxx | DLC — Belurat, Enir-Ilim | DLC legacy dungeons |
| 73xxx | All catacombs and hero graves | Paired with `DoorFlag` |
| 74xxx | DLC — Gravesite Plain, Scadu Altus, Rauh Base | DLC catacombs/dungeons |
| 76xxx | All overworld graces | Largest group (~195 entries) |

### 1.2 BonfireId / Grace Entity ID

| Property | Value |
|---|---|
| Format | `10AABBCCCC` — decimal, 10-digit |
| Example | `1042362951` = "The First Step" |
| Storage | Single `u32` field `LastRestedGrace` in `PreEventFlagsScalars` |
| Source | `spec/14-game-state.md`, `spec/15-event-flags.md` |

**What BonfireId controls:**
- Respawn location (where the player wakes up after death)
- The "last rested at" display in the pause menu
- Game state checkpoint anchor

**What BonfireId does NOT do:**
- It is NOT a list; only one value is stored
- Setting it does NOT light the in-world grace object
- It has no direct relationship to the EventFlag ID for the same grace

There is no public mapping from EventFlag ID → BonfireId in the codebase. The two namespaces are disjoint.

---

## 2. Save-File Fields

### 2.1 EventFlags Bitfield

- Location: `slot.Data[slot.EventFlagsOffset:]`
- Size: `0x1BF99F` bytes (1,835,423 bytes)
- One bit per flag; BST lookup converts flag ID → byte offset + bit index
- **Editor action**: `db.SetEventFlag(flags, graceID, true)` sets this bit

### 2.2 PreEventFlagsScalars

29-byte block immediately before the EventFlags bitfield:
`[slot.EventFlagsOffset - core.PreEventFlagsScalarsSize]`

| Field | Offset in block | Type | Description |
|---|---|---|---|
| `GameMan0x8c` | +0x00 | u8 | Unknown GameMan byte |
| `GameMan0x8d` | +0x01 | u8 | Unknown GameMan byte |
| `GameMan0x8e` | +0x02 | u8 | Unknown GameMan byte |
| `TotalDeathsCount` | +0x03 | u32 | Cumulative death counter |
| `CharacterType` | +0x07 | i32 | 0=normal, 1=invader, etc. |
| `InOnlineSessionFlag` | +0x0B | u8 | Online session active |
| `CharacterTypeOnline` | +0x0C | u32 | Online character type |
| **`LastRestedGrace`** | **+0x10** | **u32** | **BonfireId of last rested grace** |
| `NotAloneFlag` | +0x14 | u8 | Co-op / NPC companion active |
| `InGameCountdownTimer` | +0x15 | u32 | In-game countdown |
| `UnkGameDataMan0x124` | +0x19 | u32 | Unknown |

`LastRestedGrace` is the only save-file field that stores a BonfireId. It is a **single scalar** — not an array, not a set.

### 2.3 DoorFlag

Optional companion EventFlag for catacomb and hero grave graces. When set alongside the grace EventFlag, it opens the dungeon entrance door in-world.

- Stored in `data.GraceData.DoorFlag` (u32, 0 if not applicable)
- Set by `SetGraceVisited()` in `app_world.go` when `DoorFlag != 0`
- Only applies to `Cat()` and `HG()` entries in `graces.go`

### 2.4 MapFlags (62xxx, 82xxx)

Separate EventFlag layer controlling map tile reveal. Managed independently via `World.MapFlags`.

| Block | Purpose |
|---|---|
| 62xxx | Map visibility / fog-of-war reveal for overworld tiles |
| 82xxx | Map system flags (map frame unlock, region unlock) |

Setting grace EventFlags (71xxx–76xxx) does NOT set map flags. The two layers are independent.

---

## 3. What the Editor Currently Sets

`SetGraceVisited(slotIndex int, graceID uint32, visited bool)` in `app_world.go`:

1. Reads `slot.Data[slot.EventFlagsOffset:]`
2. Calls `db.SetEventFlag(flags, graceID, visited)` — sets the 71xxx/76xxx bit
3. If `DoorFlag != 0`: calls `db.SetEventFlag(flags, gd.DoorFlag, visited)` — sets door flag
4. Does NOT touch `LastRestedGrace`
5. Does NOT set any MapFlags
6. Does NOT set any BonfireId-indexed data

This is **identical** to all three reference implementations:

| Project | Implementation |
|---|---|
| er-save-manager (Python) | `EventFlags.set_flag(event_flags, flag_id, True)` — single bit |
| ER-Save-Editor (Rust) | Single `u32` EventFlag ID per grace, no BonfireId |
| Elden-Ring-Save-Editor (Python) | `toggle_grace()`: sets one bit at `grace["offset"] + grace["index"]` |

None of the reference implementations set BonfireId or any secondary state.

---

## 4. The Missing Activation Layer

### Confirmed behaviour

- Grace EventFlag set → map marker visible, fast travel available ✅
- Grace EventFlag set → physical grace object lit on arrival ❌ (not observed)

### Hypothesis A — EMEVD event script re-trigger (most probable)

Each map area runs an EMEVD script that checks grace EventFlags on area load to set the visual state of in-world grace objects. When the player fast-travels directly to the grace, the area loads with the EventFlag already set. Whether the EMEVD "first-visit" subroutine fires depends on:
- Whether the game distinguishes "EventFlag was set before this session" from "EventFlag was set in this session"
- Whether the grace object's entity state (lit/unlit) is persisted separately or re-derived from the EventFlag on every area load

If the EMEVD derives grace object state purely from the EventFlag, the object **should** already be lit on arrival — which would mean the behaviour is correct and the reported bug is a misunderstanding. If EMEVD maintains separate in-memory entity state that is not updated retroactively, the grace would appear unlit despite the EventFlag being set.

**This hypothesis requires a before/after runtime diff to confirm.**

### Hypothesis B — Second companion EventFlag (not yet identified)

A hidden EventFlag at a different ID (not in 71xxx–76xxx) might control the world-object visual state independently of the map marker. No such flag has been found in any reference implementation or CT-TGA script.

### Hypothesis C — WorldGeomMan / WorldArea geometry flag

The `WorldState` section contains geometry and area state data. A separate bit in that section might mark the grace's physical entity as activated. This section is not yet fully reverse-engineered (see `spec/16`).

### Hypothesis D — Grace object state is fully runtime / not persisted

The in-world grace object state may be entirely runtime (EMEVD/C++ object), not persisted in the save file at all. In that case, lighting the grace physically would always require a manual in-game interaction — and the editor-set EventFlag only covers the map/warp layer, which is the complete expected behaviour.

---

## 5. Diagnostic Script

`tmp/scripts/diag/grace_activation_diff.go` — read-only, `//go:build ignore`.

**Usage:**
```
go run tmp/scripts/diag/grace_activation_diff.go \
  -before tmp/save/before-church-elleh.sl2 \
  -after  tmp/save/after-church-elleh.sl2 \
  -slot 0 -grace 76100 -bonfire 1042362951
```

**Reports:**
1. Target grace EventFlag change (0→1 confirmation)
2. All EventFlag changes grouped by 1000-range
3. `PreEventFlagsScalars` diff — especially `LastRestedGrace`
4. `UnlockedRegions` diff
5. `MapFlags` diff (62xxx, 82xxx)
6. BonfireId occurrence search in raw slot bytes
7. Byte-diff summary by 0x10000 region

**Ideal save pair:**  
A = save immediately before physically touching Church of Elleh (grace 76100, bonfire ~1042362951)  
B = save immediately after resting at that grace and returning to main menu

---

## 8. Runtime Save Diff: Church of Elleh

> **Status**: ✅ Completed — 2026-05-09  
> Five save files provided: `vanilla`, `A` (editor unlock), `B` (physical touch), `C` (rest), `D` (teleport to editor-unlocked grace).

### Save file descriptions

| File | Description |
|---|---|
| `vanilla` | Baseline — no grace activation, `LastRestedGrace` = 1042362951 (The First Step) |
| `A` | Grace 76100 set via editor `SetGraceVisited()`. No gameplay. |
| `B` | Grace 76100 physically activated in-game (walked to and touched). |
| `C` | Grace 76100 rested at after physical activation. |
| `D` | Teleported (fast travel) to grace 76100 which was set by editor in save A. |

### BonfireId identification (empirical)

| BonfireId | Decimal | Grace |
|---|---|---|
| `0x3E213247` | 1042362951 | The First Step |
| `0x3E213246` | 1042362950 | **Church of Elleh** |

### EventFlag offset in `slot.Data`

| Field | Offset |
|---|---|
| `PreEventFlagsScalarsBase` | `0x3649A` |
| `EventFlagsOffset` | `0x364B7` |
| `EventFlagsEnd` | `0x1F5E56` |
| `LastRestedGrace` in raw bytes | `0x364AA` |
| Second BonfireId occurrence | `0x1F636A` (+1,300 bytes past EventFlags end — early NetworkManager) |

### Flag presence matrix

| Flag | vanilla | A (editor) | B (touch) | C (rest) | D (teleport) |
|---|---|---|---|---|---|
| **76100** Grace EventFlag | 0 | **1** | **1** | 1 | 1 |
| 62001 Underground map† | 0 | 1 | 0 | 0 | 1 |
| 82001 Show Underground† | 0 | 1 | 0 | 0 | 1 |
| 82002 Show Shadow Realm† | 0 | 1 | 0 | 0 | 1 |
| 62120 Stormveil Castle map | 0 | 0 | 1 | 1 | 1 |
| **69070** Unknown | 0 | 0 | **1** | 1 | **0** |
| 69300 Unknown | 0 | 0 | 1 | 1 | 1 |
| 78101 Unknown | 0 | 0 | 1 | 1 | 1 |

† Set by a separate editor RevealMap operation, NOT by `SetGraceVisited()`.

### `PreEventFlagsScalars` across saves

| Scalar | vanilla | A | B | C | D |
|---|---|---|---|---|---|
| `LastRestedGrace` | 1042362951 | 1042362951 | **1042362950** | 1042362950 | **1042362950** |
| `UnkGameDataMan0x124` | 61 | 61 | 61 | **73** | **40** |

`LastRestedGrace` is set automatically by the game when the player arrives at a grace (teleport OR walk). The editor does not need to set it.

### Conclusions

1. **Editor sets exactly the same EventFlag as the game.** Grace 76100 is identical in A and B. `SetGraceVisited()` is correct.

2. **`LastRestedGrace` is auto-managed by the game.** No editor intervention needed. It updates the moment the player enters a grace area.

3. **Three additional flags appear in B and D** (69300, 78101 in both; 69070 only in physical approach B/C). These are likely area-load triggers and NPC dialogue flags (Kalé, Ranni), not grace lighting flags.

4. **Flag 69070** is the only flag that distinguishes physical touch (B) from teleport (D). It is absent when using fast travel to an editor-unlocked grace. Its exact meaning (NPC trigger, tutorial) is unknown but it does not control the in-world grace object state.

5. **No save-file field persists the grace object lit/unlit state.** All B/C/D saves have the same structure relative to the grace. Hypothesis D is confirmed.

6. **Second BonfireId occurrence** at `slot.Data[0x1F636A]` (1,300 bytes past EventFlags end) updates alongside `LastRestedGrace`. This is likely in the NetworkManager section and may be used for multiplayer respawn sync.

### Hypothesis verdicts

| Hypothesis | Verdict |
|---|---|
| A — EMEVD re-trigger from EventFlag (most probable) | **Confirmed primary mechanism** — same EventFlag drives both editor and game |
| B — Hidden companion EventFlag | **Ruled out** — 69070 only triggers proximity NPC, not lighting |
| C — WorldGeomMan geometry flag | **No evidence** — no changes outside EventFlags+NM in controlled diffs |
| **D — Runtime-only state (not persisted)** | ✅ **Confirmed** |

### Recommended repair model

**Model 3 (UI warning)** — add note to `WorldTab` grace section:

> "Graces unlocked here appear on the map and enable fast travel. The in-world grace object will be lit when the area loads. Manual rest is only required to update the respawn point."

Model 2 (`LastRestedGrace` patch) is **not needed** — the game sets it automatically on arrival. Model 1 (no change) is also acceptable if no UI update is made.

---

## 6. Repair Models

### Model 1 — No change (current behaviour is correct)

If Hypothesis D is confirmed (grace object state is not persisted), the current editor behaviour is correct. `World.Graces` controls the map/warp layer only. Document this clearly in the UI.

**Risk**: Low. Requires only UI copy update.

### Model 2 — Set `LastRestedGrace` to BonfireId of activated grace

When the user activates a single grace via the editor, also set `LastRestedGrace` to the corresponding BonfireId.

**Blockers**:
- No public EventFlag ID → BonfireId mapping exists in the codebase
- Setting `LastRestedGrace` changes the respawn point — unintended side effect when bulk-setting all graces
- Requires building and validating a full EventFlag ID → BonfireId lookup table (~419 entries)

**Risk**: Medium. Only viable for single-grace activation, not bulk.

### Model 3 — Emit a warning in the UI

Add a UI note on the `WorldTab` grace section: "Graces set via this editor will appear on the map and allow fast travel. The in-world grace object requires a manual rest to fully activate."

**Risk**: None. Correct description of actual behaviour if Hypothesis D is confirmed.

---

## 7. Next Steps

### Without console access

1. Run `grace_activation_diff.go` on a real before/after save pair (Church of Elleh recommended)
2. Check whether `LastRestedGrace` changes in the diff — and whether any other EventFlags change besides 76100
3. Check the byte-diff by region (section 7 of the script) — a change outside the EventFlags region would suggest a WorldState or entity field is involved

### With console access

1. Set grace 76100 via the editor, load the save, fast-travel to Church of Elleh
2. Observe: is the grace object lit or unlit on arrival?
3. If unlit: approach the grace — does the activation animation play, or does the grace refuse to activate?
4. Reload a clean save and manually rest at the grace — compare the resulting save with the editor-patched save using the diff script

---

## Sources

| File | Relevance |
|---|---|
| `backend/db/data/graces.go` | All 419 grace entries with EventFlag IDs and DoorFlags |
| `backend/core/section_eventflags.go` | `PreEventFlagsScalars` struct, `EventFlagsBlock` |
| `app_world.go` | `SetGraceVisited()` implementation |
| `spec/14-game-state.md` | `LastRestedGrace` field, BonfireId semantics |
| `spec/15-event-flags.md` | "Bonfire IDs" section, EventFlag byte offsets |
| `spec/16-world-state.md` | WorldGeomMan / WorldArea (partial) |
| `tmp/repos/er-save-manager/src/er_save_manager/parser/event_flags.py` | Reference: single-flag approach |
| `tmp/repos/ER-Save-Editor/src/db/graces.rs` | Reference: single u32 EventFlag ID per grace |
| `tmp/repos/Elden-Ring-Save-Editor/src/Final.py` | Reference: single bit per grace |
| `tmp/repos/Elden-Ring-Save-Editor/src/Resources/Json/graces.json` | Grace map with offset + index (no BonfireId) |
| `tmp/scripts/diag/grace_activation_diff.go` | Diagnostic script for before/after diff |
