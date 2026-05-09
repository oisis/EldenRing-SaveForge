# 47 — Site of Grace Activation

> **Type**: Investigation / Design doc
> **Status**: ✅ Resolved for map/fast-travel unlock; ⚠️ full in-world activation animation remains open
> **Scope**: All identifier spaces and save-file fields involved in Site of Grace discovery, fast-travel, and physical in-world object state.

---

## Background

This document was opened to investigate whether `SetGraceVisited()` — which sets the grace EventFlag — is sufficient to fully unlock a Site of Grace, or whether additional save-file fields need to be written.

**Conclusion (2026-05-09)**: The editor sets exactly the same EventFlag the game sets. `LastRestedGrace` is auto-managed by the game on arrival. No secondary save-file field was found in the Church of Elleh test case. The current implementation is correct for map marker and fast-travel unlock. Some graces may still play their in-world activation sequence after teleport; that behaviour remains a separate open research question. See §5 for the runtime diff.

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

**In-world visual/activation state (case-specific):**
- May be derived by EMEVD from the EventFlag at area load for some grace categories
- Not confirmed globally across all grace types — see §4 and §8 (Future Research)

**What this flag does NOT control:**
- Respawn point assignment (`LastRestedGrace`) — managed separately by the game

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
| Example | `1042362951` = "The First Step"; `1042362950` = "Church of Elleh" |
| Storage | Single `u32` field `LastRestedGrace` in `PreEventFlagsScalars` |
| Source | `spec/14-game-state.md`, `spec/15-event-flags.md` |

**What BonfireId controls:**
- Respawn location (where the player wakes up after death)
- The "last rested at" display in the pause menu
- Game state checkpoint anchor

**What BonfireId does NOT do:**
- It is NOT a list; only one value is stored
- It has no direct relationship to the EventFlag ID for the same grace
- The editor does NOT need to set it — the game updates `LastRestedGrace` automatically whenever the player arrives at a grace (teleport or walk)

There is no public mapping from EventFlag ID → BonfireId in the codebase. The two namespaces are disjoint.

---

## 2. Save-File Fields

### 2.1 EventFlags Bitfield

- Location: `slot.Data[slot.EventFlagsOffset:]`
- Size: `0x1BF99F` bytes (1,833,375 bytes)
- One bit per flag; BST lookup converts flag ID → byte offset + bit index
- **Editor action**: `db.SetEventFlag(flags, graceID, true)` sets this bit

Confirmed offsets (Church of Elleh test save, slot 0):

| Field | Offset in `slot.Data` |
|---|---|
| `PreEventFlagsScalarsBase` | `0x3649A` |
| `EventFlagsOffset` | `0x364B7` |
| `EventFlagsEnd` | `0x1F5E56` |
| `LastRestedGrace` (raw) | `0x364AA` |

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

`LastRestedGrace` is the only save-file field that stores a BonfireId. It is a **single scalar** — not an array, not a set. The game writes it automatically on grace arrival; the editor leaves it untouched.

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
4. Does NOT touch `LastRestedGrace` (correct — game manages this automatically)
5. Does NOT set any MapFlags
6. Does NOT set any BonfireId-indexed data

This is **identical** to all three reference implementations:

| Project | Implementation |
|---|---|
| er-save-manager (Python) | `EventFlags.set_flag(event_flags, flag_id, True)` — single bit |
| ER-Save-Editor (Rust) | Single `u32` EventFlag ID per grace, no BonfireId |
| Elden-Ring-Save-Editor (Python) | `toggle_grace()`: sets one bit at `grace["offset"] + grace["index"]` |

---

## 4. Confirmed Activation Model

### What the editor controls

| Layer | Controlled by | How |
|---|---|---|
| Map marker | Grace EventFlag (71xxx–76xxx) | `SetGraceVisited()` → `SetEventFlag()` |
| Fast-travel list entry | Grace EventFlag | same |
| "Discovered" quest state | Grace EventFlag | same |
| Dungeon entrance door | DoorFlag (companion EventFlag) | `SetGraceVisited()` for Cat/HG graces |
| In-world object visual state | Not fully modeled — Church of Elleh revealed no persistent field; other grace categories may still play activation animation after editor unlock | open research question |
| Respawn point (`LastRestedGrace`) | Game, automatic on arrival | not set by editor; game writes it |

### Church of Elleh confirmed values

| Item | Value |
|---|---|
| Grace EventFlag ID | `76100` (0x00012944) |
| The First Step BonfireId | `1042362951` (0x3E213247) |
| Church of Elleh BonfireId | `1042362950` (0x3E213246) |

### Additional flags observed in physical-touch and teleport sessions

| Flag | Physical touch | Teleport | Likely meaning |
|---|---|---|---|
| `69300` | ✅ | ✅ | Area-load trigger (Church of Elleh region entered) |
| `78101` | ✅ | ✅ | Area-load trigger (Church of Elleh region entered) |
| `69070` | ✅ | ❌ | Physical proximity trigger — NPC/cutscene (Kalé, Ranni), NOT grace lighting |

None of these flags are required for editor-side grace unlock (map marker + fast travel). They are set by EMEVD when the player is in the area and do not control the save-file unlock layer.

### Hypothesis verdicts

| Hypothesis | Verdict |
|---|---|
| A — EMEVD re-triggers from EventFlag on area load | ✅ Confirmed for Church of Elleh (76xxx overworld) |
| B — Hidden companion EventFlag controls visual state | No companion flag found for Church of Elleh. Not globally ruled out for all grace categories. |
| C — WorldGeomMan geometry flag persists visual state | No evidence in Church of Elleh diffs. Not globally ruled out. |
| D — Grace object state is fully runtime, not persisted | No persistent lit/unlit field found in Church of Elleh test. Likely runtime/EMEVD-managed, but not globally proven for all grace categories. |

---

## 5. Runtime Save Diff: Church of Elleh

> Completed 2026-05-09. Five save files compared: `vanilla` / `A` (editor) / `B` (physical touch) / `C` (rest) / `D` (teleport to editor-unlocked grace).

### Flag presence matrix

| Flag | vanilla | A (editor) | B (touch) | C (rest) | D (teleport) |
|---|---|---|---|---|---|
| **76100** Grace EventFlag | 0 | **1** | **1** | 1 | 1 |
| 62120 Stormveil Castle map | 0 | 0 | 1 | 1 | 1 |
| 69070 Unknown (NPC trigger?) | 0 | 0 | **1** | 1 | **0** |
| 69300 Unknown (area-load) | 0 | 0 | 1 | 1 | 1 |
| 78101 Unknown (area-load) | 0 | 0 | 1 | 1 | 1 |

Flags 62001 / 82001 / 82002 (underground map display) appeared in save A because the user had also run `RevealBaseMap` — they are NOT set by `SetGraceVisited()`.

### `PreEventFlagsScalars` across saves

| Scalar | vanilla | A | B | C | D |
|---|---|---|---|---|---|
| `LastRestedGrace` | 1042362951 | 1042362951 | **1042362950** | 1042362950 | **1042362950** |
| `UnkGameDataMan0x124` | 61 | 61 | 61 | **73** | **40** |

`LastRestedGrace` transitions The First Step → Church of Elleh in both B (physical touch) and D (teleport). The game sets it automatically; the editor does not need to.

### Second BonfireId occurrence

BonfireId was also found at `slot.Data[0x1F636A]` — 1,300 bytes past EventFlags end, likely early NetworkManager. It updates identically with `LastRestedGrace`. Probably used for multiplayer respawn sync.

### Key finding

Grace 76100 is **identical** in A (editor) and B (game). The editor sets exactly what the game sets. No additional save-file fields are required for the map/fast-travel unlock layer.

---

## 6. Diagnostic Script

`tmp/scripts/diag/grace_activation_diff.go` — read-only, `//go:build ignore`.

**Usage:**
```
go run tmp/scripts/diag/grace_activation_diff.go \
  -before tmp/site-of-grace-debug/ER0000-kro55-vanilla.sl2 \
  -after  tmp/site-of-grace-debug/ER0000-b.sl2 \
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

Reference save files: `tmp/site-of-grace-debug/` (vanilla, A, B, C, D).  
Analysis report: `tmp/site-of-grace-debug/grace-activation-analysis.md`.

---

## 7. Repair Models

### Model 1 — No backend change ✅ Implemented

`SetGraceVisited()` is correct. No logic changes needed.

### Model 2 — UI clarification ✅ Implemented

`SetGraceVisited()` unlocks the map marker and fast-travel entry. It does not guarantee that all in-world activation animations are suppressed.

Short note added to the `WorldTab` Sites of Grace section:

> "Sites of Grace unlocked here will appear on the map and become available for fast travel. Some graces may still play their in-world activation sequence when visited."

### Model 3 — Optional: investigate flag 69070

Flag 69070 is set only during physical approach (not by editor or teleport). If users report that NPC cutscenes at Church of Elleh (Kalé's greeting, Ranni's first appearance) don't trigger when arriving via editor-unlocked fast travel, setting 69070 alongside the grace EventFlag may fix that. Not required for grace unlock itself.

**Not pursued:**
- Setting `LastRestedGrace` — not needed; game manages it automatically on arrival
- Building EventFlag ID → BonfireId lookup table — not needed
- Searching for a hidden companion flag for grace activation — not found for Church of Elleh; open for other grace categories (see §8)

---

## 8. Future Research

Further SoG investigation is paused; the main focus has returned to the Faster Invasions thread.

If resumed, test save pairs across multiple grace categories to determine whether category-specific companion flags exist:

| Category | Range | Notes |
|---|---|---|
| Overworld (tested) | 76xxx | Church of Elleh — no extra field found |
| Catacombs / hero graves | 73xxx | Paired with `DoorFlag`; different activation path |
| Legacy dungeons | 71xxx | Stormveil, Leyndell — boss-adjacent graces |
| DLC | 72xxx, 74xxx | Shadow of the Erdtree — may have additional activation layers |

Required save set per grace category:
1. vanilla / before
2. editor-unlocked (EventFlag only)
3. after manual in-world activation
4. after teleport without manual touch (optional)

Goal: determine whether any category produces a category-specific companion EventFlag, WorldState bit, or other persistent field absent from the Church of Elleh overworld test.

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
| `tmp/site-of-grace-debug/grace-activation-analysis.md` | Full runtime diff analysis report |
