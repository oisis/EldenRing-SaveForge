# 14 — Game State

> **Type**: Binary format spec + canonical chapter
> **Scope**: Game / profile / progression state in SaveForge — `PreEventFlagsScalars` (29 B), `ClearCount` (NG+ cycle), the related write paths via `CharacterViewModel`, and a clear separation from World State ([16](16-world-state.md)).

Cross-refs: [01-header.md](01-header.md), [15-event-flags.md](15-event-flags.md), [16-world-state.md](16-world-state.md), [17-player-coordinates.md](17-player-coordinates.md), [19-weather-time.md](19-weather-time.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md), [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md), [50-item-companion-flags.md](50-item-companion-flags.md), [38-boss-multiflag.md](38-boss-multiflag.md).

---

## 1. Chapter purpose

To define unambiguously:

- what SaveForge treats as **Game State** (`PreEventFlagsScalars` 29 B + `ClearCount` + related fields in `slot.Player`),
- which fields are write-capable via `SaveCharacter` / `CharacterViewModel`,
- which fields are read-only (parse + verbatim round-trip without a setter),
- what the relation is to World State ([16](16-world-state.md)) and Event Flags ([15](15-event-flags.md)),
- what is planned or research-only (boss multi-flag → [38](38-boss-multiflag.md)).

It does not duplicate the event flag helper API details (see [15](15-event-flags.md)), the World State subsystem map (see [16](16-world-state.md)), or the Sites of Grace activation flow (see [47](47-site-of-grace-activation.md)).

## 2. Status

| Aspect | Status |
|---|---|
| `PreEventFlagsScalars` struct parser (29 B) | ✅ `backend/core/section_eventflags.go:27-91` |
| `PreEventFlagsScalarsSize = 29` | ✅ `section_eventflags.go:41` (3+4+4+1+4+4+1+4+4) |
| `ClearCount` parsing (NG+ cycle) | ✅ `backend/core/structures.go:314-318` (dynamic chain offset) |
| `ClearCount` write path | ✅ via `SaveCharacter` → `vm.ApplyVMToParsedSlot` (cap 7) + event flag sync 50-57 |
| Player stats / Level / Souls write | ✅ via `SaveCharacter` (see §13) |
| `LastRestedGrace` write endpoint | ❌ **none** — explicit comment in `app_world.go:41` "Does not touch LastRestedGrace — the game updates that automatically on arrival" |
| `TotalDeathsCount` write endpoint | ❌ none — read-only (verbatim round-trip) |
| `CharacterType`, `InOnlineSessionFlag`, etc. | ❌ none — read-only verbatim |
| Boss multi-flag editor | ❌ **planned only** — see [38](38-boss-multiflag.md) |
| Frontend UI for NG+ (`clearCount`) | ✅ `DatabaseTab.tsx` (item caps scaling) — read-only display, NOT editor |
| Test coverage | ✅ `section_eventflags_test.go`, `section_world_geom_test.go`, `pvp_test.go`, `tests/map_flags_test.go` (round-trip + EventFlagsOffset correctness) |

## 3. Source of truth in code

| File / symbol | What it contains | Mode |
|---|---|---|
| `backend/core/section_eventflags.go::PreEventFlagsScalars` (lines 27-39) | 29 B struct: 11 fields (GameMan x3 + scalars) | read/write verbatim |
| `backend/core/section_eventflags.go::PreEventFlagsScalarsSize` (line 41) | `= 29` | const |
| `backend/core/section_eventflags.go::Read/Write` (lines 43-106) | Parser + serializer per field | RW |
| `backend/core/section_eventflags.go::EventFlagsByteCount = 0x1BF99F` | Size of the event_flags bitfield (outside Game State scope, but adjacent) | const |
| `backend/core/structures.go::Player.ClearCount` (line 214) | `uint32` — NG+ cycle (0=NG, 1-7=NG+N) | RW via VM |
| `backend/core/structures.go::SaveSlot.ClearCountOffset` (line 243) | Offset in the dynamic chain (`horse + DynClearCount`) | computed |
| `backend/core/structures.go::ClearCount read` (lines 314-318) | Read + cap @ 7 | parser |
| `backend/core/offset_defs.go::DynClearCount = 0x44` (line 101) | Offset relative from horse to ClearCount | const |
| `backend/vm/character_vm.go::CharacterViewModel.ClearCount` (line 82) | VM field | bridge |
| `backend/vm/character_vm.go::ApplyVMToParsedSlot` (cap @ 7 for ClearCount, lines 319-322) | Cap + write back from `vm.CharacterViewModel` to `slot.Player` | mutator |
| `app.go::SaveCharacter` (lines 178-217) | Public Wails endpoint wrapper | RW |
| `app.go:204-209` | NG+ event flag sync (50-57) after SaveCharacter | side effect |

## 4. Mental model

```
slot.Data
  ├─ ... (other sections)
  ├─ Dynamic chain (post-horse):
  │    ├─ horse  +  DynClearCount (0x44)  →  ClearCount (u32, capped @ 7)
  │    └─ ... (other dynamic offsets)
  ├─ ... (TutorialData + final fields)
  ├─ PreEventFlagsScalars (29 B):  ←─── this chapter
  │    ├─ GameMan0x8c/0x8d/0x8e   3× u8  (unk)
  │    ├─ TotalDeathsCount        u32
  │    ├─ CharacterType           i32  (0=Host, 1=WhitePhantom, ...)
  │    ├─ InOnlineSessionFlag     u8
  │    ├─ CharacterTypeOnline     u32
  │    ├─ LastRestedGrace         u32  (BonfireId — NOT an Event Flag ID!)
  │    ├─ NotAloneFlag            u8
  │    ├─ InGameCountdownTimer    u32
  │    └─ UnkGameDataMan0x124     u32
  ├─ EventFlagsBlock (0x1BF99F + 1 terminator)  ←─── see [15]
  └─ ... (other sections — World State, Trailing block, etc.)
```

**Write path for ClearCount**:

```
User → CharacterViewModel.ClearCount → SaveCharacter(index, vm)
                                          ├─ a.pushUndo(index)                 ← snapshot
                                          ├─ vm.ApplyVMToParsedSlot()
                                          │    ├─ if ClearCount > 7: cap @ 7
                                          │    └─ data.ClearCount = vm.ClearCount
                                          ├─ a.applyMemoryStonesToSlot()
                                          ├─ slot.SyncPlayerToData()           ← flush to slot.Data
                                          ├─ app.go:204-209: SetEventFlag(50+i, i == ClearCount) for i in 0..7
                                          │                                    ← NG+ flags sync
                                          └─ ProfileSummary[index].Level/Name update
```

**No write path for** `LastRestedGrace`, `TotalDeathsCount`, and other `PreEventFlagsScalars` fields — verbatim round-trip.

## 5. Game State vs World State

Chapter table:

| Domain | What it covers | Canonical chapter |
|---|---|---|
| **Game State** (this chapter) | Character profile, stats, ClearCount/NG+, `PreEventFlagsScalars` (29 B), `LastRestedGrace` as BonfireId | 14 |
| **World State** | Maps / regions / DLC tiles / graces / item flags / PvP presets — all via Event Flags or UnlockedRegions | [16](16-world-state.md) overview |
| **Event Flags foundation** | Bitfield API (BST + helpers) | [15](15-event-flags.md) |
| **Player Coordinates** | XYZ + MapID + quaternion | [17](17-player-coordinates.md) |
| **Weather / Time** | `WorldAreaWeather` + `WorldAreaTime` (12+12 B) | [19](19-weather-time.md) |
| **Sites of Grace** | Activation flags 71xxx-76xxx, NOT BonfireId | [47](47-site-of-grace-activation.md) |

**Critical distinction**: `LastRestedGrace` (Game State, BonfireId namespace) ≠ Grace EventFlag ID (World State, 71xxx-76xxx band).

- `LastRestedGrace = 1042362950` → BonfireId Church of Elleh (a 10-digit ID stored in `PreEventFlagsScalars`).
- Grace EventFlag 76100 → a bit in the event_flags bitfield, sets marker visibility and fast travel.
- **SaveForge edits the EventFlag only** (via `SetGraceVisited`, [47](47-site-of-grace-activation.md) §10). `LastRestedGrace` is read-only.

## 6. Current parsed data

### 6.1 `PreEventFlagsScalars` (29 B)

| Field | Type | Size | Write endpoint | Comment |
|---|---|---|---|---|
| `GameMan0x8c` | u8 | 1 B | ❌ none | unk, runtime GameMan offset |
| `GameMan0x8d` | u8 | 1 B | ❌ none | unk |
| `GameMan0x8e` | u8 | 1 B | ❌ none | unk |
| `TotalDeathsCount` | u32 | 4 B | ❌ none | total death count of the character |
| `CharacterType` | i32 | 4 B | ❌ none | `0=Host, 1=WhitePhantom, 2=DarkSpirit, 3=Ghost` (`needs verification`) |
| `InOnlineSessionFlag` | u8 | 1 B | ❌ none | `0=offline, 1=in session` (`needs verification`) |
| `CharacterTypeOnline` | u32 | 4 B | ❌ none | online character type (`needs verification`) |
| `LastRestedGrace` | u32 | 4 B | ❌ none (the game manages it runtime) | BonfireId — see §10 |
| `NotAloneFlag` | u8 | 1 B | ❌ none | `1 = in co-op/invasion` (`needs verification`) |
| `InGameCountdownTimer` | u32 | 4 B | ❌ none | countdown timer (`needs verification` — semantics) |
| `UnkGameDataMan0x124` | u32 | 4 B | ❌ none | unk |

**Sum**: `3 + 4 + 4 + 1 + 4 + 4 + 1 + 4 + 4 = 29 B`.

### 6.2 `ClearCount` (outside `PreEventFlagsScalars`)

- Type: `uint32`
- Location: dynamic chain offset `horse + DynClearCount (0x44)`
- **Maximum value**: 7 (the code caps it on read and write — `structures.go:316-317`, `character_vm.go:319-322`)
- Mapping: `0 = Journey 1 (NG), 1 = NG+1, ..., 7 = NG+7`
- Write endpoint: ✅ `SaveCharacter` via `vm.CharacterViewModel.ClearCount`

### 6.3 `slot.Player` (wider Game State)

`slot.Player` additionally contains:

- `Name` (`[16]uint16` UTF-16 LE)
- `Level` (uint32)
- `Souls` (uint32)
- `SoulMemory` (uint32, clamped to `runesCostForLevel(level)`)
- `Class` (uint8)
- 8 attributes (Vigor/Mind/Endurance/Str/Dex/Int/Faith/Arcane) — uint32 each
- `TalismanSlots` (uint8, capped @ 3)
- `Gender` (uint8)
- `ScadutreeBlessing`, `ShadowRealmBlessing` (uint8 each — DLC)

All these fields have a write endpoint via `SaveCharacter`. The slot layout details are **not** in this chapter — the spec for `slot.Player` is out of scope (see `backend/core/structures.go`).

`needs verification`: whether `SoulMemory` clamping to `runesCostForLevel(level)` is game-accurate — the code signals a minimum, but there is no isolated runtime test of whether the game would accept a lower value.

## 7. Read-only vs write-capable fields

### 7.1 Write-capable (via `SaveCharacter` / `CharacterViewModel`)

| Field | Cap / validation | NG+ event flag side effect |
|---|---|---|
| `Player.Name` | 16 UTF-16 characters | — |
| `Player.Level` | none | — |
| `Player.Souls` | none | — |
| `Player.SoulMemory` | clamp ≥ `runesCostForLevel(level)` | — |
| `Player.Class` | no validation | — |
| Vigor/Mind/Endurance/Str/Dex/Int/Faith/Arcane | none (UI may limit) | — |
| `Player.TalismanSlots` | cap @ 3 | — |
| `Player.ClearCount` | cap @ 7 | ✅ event flags 50-57 sync (app.go:204-209) |
| `Player.ScadutreeBlessing` / `ShadowRealmBlessing` | none | — |
| `Player.Gender` | none | — |
| `Player.Inventory` / `Player.Storage` | via item lifecycle (see [50](50-item-companion-flags.md)) | — |

### 7.2 Read-only (parse + verbatim round-trip, no setter)

All 11 `PreEventFlagsScalars` fields (§6.1) — none has a public endpoint in `app*.go`. `grep -rn "SetLastRested\|SetTotalDeaths\|SetCharacterType\|SetGameMan" app*.go` returns **0 results**.

Other read-only sections related to Game State:

| Section | Mode | Location |
|---|---|---|
| `TutorialData` (variable) | RW (append via `core.AppendTutorialID`) | `backend/core/` |
| `GaItemData` (7000 × 16 B) | RW (auto-managed by item operations) | `backend/core/` |
| `MenuSaveLoad` (variable) | verbatim | `backend/core/` |
| `TrophyEquipData` | verbatim | `backend/core/` |
| `ProfileSummary` (per slot, in `SaveFile`) | RW via SaveCharacter (Level + Name) | `backend/core/` |
| `WorldGeomBlock` (5 blobs) | verbatim | see [16](16-world-state.md) §6.1 |

## 8. ClearCount / NG+ relation

### 8.1 Write flow

```
User UI → vm.CharacterViewModel.ClearCount (0-7)
       → SaveCharacter(index, vm)
            ├─ vm.ApplyVMToParsedSlot(slot)
            │    └─ cap @ 7 (character_vm.go:319-322)
            │    └─ data.ClearCount = vm.ClearCount
            ├─ slot.SyncPlayerToData()
            └─ SetEventFlag(50+i, i == ClearCount) for i in 0..7
                                ← NG+ event flags sync
```

### 8.2 NG+ event flags 50-57

`app.go:204-209` synchronizes event flags `50, 51, 52, 53, 54, 55, 56, 57` with the current `ClearCount`:

- Flag `50 + ClearCount` is set to `true`.
- The other 7 flags in the 50-57 range are set to `false`.

This maintains the **invariant**: exactly one flag from `50..57` is SET at any moment, matching the current NG+ cycle.

`needs verification`: whether the game uses flags `50..57` to determine the NG+ cycle, or only the `ClearCount` field. The comment in `app.go:204` says "Sync NG+ event flags (50-57) with ClearCount" — the implication: the game reads both paths, so the sync is defensive.

### 8.3 NG+ side effects

Editing `ClearCount` from 0 → N (or N → M) **does not reverse**:

- Defeated bosses (event flags in the boss bands — see [38](38-boss-multiflag.md)).
- Completed quests.
- Acquired Map Fragments.
- Activated Sites of Grace.
- Unlocked regions (UnlockedRegions — see [11](11-regions.md)).

**Consequence**: the NG+ value may become desynchronized with the actual progression flag state. The game probably trusts `ClearCount` as the main counter (`needs verification`).

Mitigation: none. SaveForge **does not validate** ClearCount ↔ flags progression consistency.

## 9. LastRestedGrace / BonfireId relation

### 9.1 Field

- Location: `PreEventFlagsScalars.LastRestedGrace` (`u32`, offset +0x10 within the 29 B block)
- Format: **BonfireId** — a 10-digit ID (e.g., `1042362950` = Church of Elleh, `1042362951` = The First Step)
- It is **NOT** an Event Flag ID (the 71xxx-76xxx range from [47](47-site-of-grace-activation.md))

### 9.2 Write path

**None.** Explicit comment in `app_world.go:41`:

```
// Does not touch LastRestedGrace — the game updates that automatically on arrival.
```

`SetGraceVisited` (from [47](47-site-of-grace-activation.md) §10) sets **only** the grace event flag + optional DoorFlag + companion flags — it does **not** change `LastRestedGrace`. The game itself overwrites this field on a physical touch / teleport to another grace.

### 9.3 Second BonfireId offset

Empirical observation (CHANGELOG 2026-05-09): a second occurrence of BonfireId in the slot data was observed at offset ≈`0x1F636A` in one test slot, ~1300 B past the end of EventFlags. It updates identically with `LastRestedGrace`.

`needs verification`: whether this second offset is stable cross-slot/cross-platform, or an artifact of a specific save. Currently SaveForge **does not parse** it as a separate field — it is part of the raw bytes of the `NetworkManager` section. See [16](16-world-state.md) §3.

### 9.4 BonfireId ↔ Grace EventFlag ID mapping

**None in the current code**. There is no public table nor `GetBonfireIDForGrace(eventFlagID)` API nor its inverse. These two namespaces are disjoint in SaveForge.

`needs verification`: whether a paired table (`BonfireParam.csv` or similar) exists in `regulation.bin`. Currently not parsed.

## 10. Event Flags relation

`PreEventFlagsScalars` (29 B) is directly adjacent to the `EventFlagsBlock` bitfield (`EventFlagsByteCount = 0x1BF99F`). Comment in `section_eventflags.go:10-11`:

> PreEventFlagsScalars — block of scalar fields between TutorialData and the event_flags bitfield.

Functionally:

- `PreEventFlagsScalars` are **Game State scalars** (per-character progress counters).
- Event Flags are **World State bits** (per-world boolean state).

This chapter **does not duplicate** the Event Flags helper API (BST resolver, byte/bit indexing) — see [15](15-event-flags.md).

Side effect: `app.go:204-209` synchronizes 8 event flags (50-57) with `ClearCount` — a bridge between a Game State scalar and Event Flags bits.

## 11. Sites of Grace relation

See [47](47-site-of-grace-activation.md). Critical distinction:

| Aspect | Game State (this chapter) | Sites of Grace ([47](47-site-of-grace-activation.md)) |
|---|---|---|
| Field | `LastRestedGrace` (BonfireId) | event flags 71xxx-76xxx |
| Namespace | BonfireId (10-digit format) | Event Flag ID (`data.Graces` keys) |
| Write endpoint | ❌ none (the game manages it runtime) | ✅ `SetGraceVisited` |
| What it controls | Spawn point after death, "last rested" in the pause menu | Map marker + fast travel + quest gates |
| Companion flag fan-out | n/a | ✅ Gatefront only (see [47](47-site-of-grace-activation.md) §8) |
| Cross-mapping | none in SaveForge | none (`needs verification`) |

## 12. Boss / progression flags

Boss state in the current code:

- **Read endpoint**: `GetBosses(slotIndex)` in `app_world.go:87` — returns the full list of bosses from `data.Bosses` with `Defeated=true/false` from the event flags bitfield.
- **Write endpoint**: `SetBossDefeated(slotIndex, bossID, defeated)` in `app_world.go:113` — a single event flag SET/CLEAR.

This is a **single-flag editor** — it operates on individual event flags per boss.

The **boss multi-flag editor** (a complex sync of multiple flags per boss, e.g., cutscene flags + memory flags + reward flags) is **planned only**:

- See [38-boss-multiflag.md](38-boss-multiflag.md).
- The current code has NO multi-flag sync implementation.
- The UI in `WorldTab.tsx` uses only the single-flag `SetBossDefeated`.

`needs verification`: whether, with a "fake defeat" via a single event flag, the game re-triggers the boss cutscene or quests. The old doc and CHANGELOG suggest "yes for some bosses" — the boss multi-flag editor is planned as a fix.

## 13. Current implemented behavior

### 13.1 `SaveCharacter(index, vm)`

`app.go:178-217`:

1. Validates `a.save != nil` and `index` ∈ [0, 10).
2. **`a.pushUndo(index)`** (line 186) — snapshot before all mutations.
3. Calls `vm.ApplyVMToParsedSlot(&charVM, &a.save.Slots[index])` — flush VM fields to `slot.Player` (incl. cap ClearCount @ 7).
4. Applies MemoryStones via `a.applyMemoryStonesToSlot(slot, charVM.MemoryStones)`.
5. `slot.SyncPlayerToData()` — flush `slot.Player` → `slot.Data` (binary serialize).
6. **NG+ event flag sync 50-57** (`app.go:204-209`) — error swallowed.
7. Updates `ProfileSummary[index].Level` + `.CharacterName`.

`SaveCharacter` has single-snapshot rollback via user Undo (snapshot from `pushUndo`). Per-step rollback does not exist — if `ApplyVMToParsedSlot` or `applyMemoryStonesToSlot` returns an error, the code does `return err` without auto-restore, but the snapshot remains on the Undo stack for user-initiated rollback.

`needs verification`: whether the snapshot from `pushUndo` remains on the stack even after `return err` (typically yes — `pushUndo` is a synchronous push, not wrapped in a defer/conditional pop).

### 13.2 `GetCharacter(index)`

`app.go:164-176` — read-only, maps `slot.Player` + flag values onto `CharacterViewModel`.

### 13.3 Read-only verbatim round-trip

The 11 `PreEventFlagsScalars` fields are preserved by `Read`/`Write` per field. The `section_eventflags_test.go` test verifies the round-trip (parse → serialize → diff = 0).

## 14. Planned / research-only areas

| Area | Status | Location |
|---|---|---|
| Boss multi-flag editor | planned | [38-boss-multiflag.md](38-boss-multiflag.md) |
| `LastRestedGrace` write endpoint | none — the game manages it runtime, no planned change | n/a |
| `TotalDeathsCount` / `CharacterType` / other `PreEventFlagsScalars` writes | none — read-only | n/a |
| BonfireId ↔ Grace EventFlag mapping | none — not parsed from `regulation.bin` | `needs verification` |
| NG+ progression validator (ClearCount vs flags consistency) | none | n/a |
| Boss/quest re-trigger detection after `ClearCount` change | none | `needs verification` |
| `InGameCountdownTimer` semantics | unknown | `needs verification` |
| Second occurrence of BonfireId in NetworkManager | empirically observed, not parsed | see [47](47-site-of-grace-activation.md) §10 |

## 15. Write path and rollback caveats

### 15.1 `SaveCharacter` as a single transaction

`SaveCharacter` performs a sequence of mutations (`ApplyVMToParsedSlot` → MemoryStones → SyncPlayerToData → event flag sync → ProfileSummary update) **under one `pushUndo` snapshot** (`app.go:186`). A user-initiated Undo reverts **all** changes to the pre-`SaveCharacter` state.

If any step fails (e.g., `applyMemoryStonesToSlot` returns an error), the code does `return err` without auto-restore — the earlier mutations remain in `slot.Player` and probably in `slot.Data`. The snapshot stays on the Undo stack, so the **user can manually revert** (`needs verification` — see §13.1 note on stack behavior after `return err`).

### 15.2 Cap @ 7 for ClearCount

`character_vm.go:319-322` always caps ClearCount at 7 on write — even if the UI sends 99. This is a safe limit (the game has NG+0 through NG+7).

### 15.3 Event flag sync 50-57

`app.go:204-209` sets 8 flags (50, 51, ..., 57) — one SET, seven CLEAR — based on `ClearCount`. There is no `EventFlagsOffset` validation in this block beyond the initial `if slot.EventFlagsOffset > 0`.

If `EventFlagsOffset` is valid, but `SetEventFlag(50+i, ...)` fails (e.g., a resolver error), the error is **ignored** (`_ = db.SetEventFlag(...)`). The NG+ event flag sync may remain partially inconsistent after `SaveCharacter`.

### 15.4 No hash recalculation in SaveCharacter

`SaveCharacter` does not call `RebuildSlot` nor hash recalculation. These are invoked in `WriteSave` (a.go:220) on file write — Save → Write Save is two-stage.

`needs verification`: whether a partially mutated slot.Data after SaveCharacter (without a subsequent WriteSave) can corrupt the save on the next Load. Round-trip tests cover the full Save→Write→Load loop, not partial state.

### 15.5 No multi-field consistency validator

SaveCharacter **does not check** consistency such as "ClearCount > 0 → some bosses should be defeated" nor "Souls > requiredForLevel". Validation is **fail-open** (even if the UI warns, the backend accepts).

## 16. Validation and safety notes

### 16.1 ClearCount progression desync

Editing `ClearCount` from 0 → 7 does **not**:

- Defeat bosses for NG+0..6.
- Reset quest progression.
- Reset boss positions (Roundtable Hold dialogs).

The game may, mid NG+7, fail to find the expected flags from previous cycles → soft-lock of some quest lines or unlocking of scaled enemies without progression rewards.

`needs verification`: the exact boss/quest impact at runtime.

### 16.2 Flag mismatch after ClearCount edit

NG+ event flags 50-57 are synced (§8.2), but **boss event flags** in the defeat-id bands (per boss arena) are **not**. The game may try to re-trigger a boss whose flag remained from a previous NG.

### 16.3 LastRestedGrace invalid BonfireId

`LastRestedGrace` is read-only in SaveForge. If an external tool changes it to an invalid BonfireId:

- The game may crash on load (attempting to spawn at a nonexistent grace).
- The game may fall back to the default starting grace.

`needs verification`: which exact fallback the game performs.

### 16.4 PreEventFlagsScalars verbatim — what if it breaks

The fields `GameMan0x8c/0x8d/0x8e`, `UnkGameDataMan0x124` are verbatim, but their semantics are unknown. Any external mutation is an unpredictable risk.

`InGameCountdownTimer` — `needs verification` what it controls (game timer? quest timer? death respawn delay?).

### 16.5 Platform / version differences

`PreEventFlagsScalarsSize = 29` is identical on PC and PS4. There is no per-platform offset validation for `LastRestedGrace`.

The `ClearCount` offset in the dynamic chain (`horse + DynClearCount = 0x44`) may differ binarily between platforms (`needs verification` — no isolated test).

### 16.6 In-game verification gaps

There are no CI tests of the kind "set ClearCount = 5 → reload save in-game → assert NG+5 displayed in menu". All verifications are ad-hoc runtime (CHANGELOG entries).

### 16.7 Rollback limits

`SaveCharacter` **has** `pushUndo(index)` (`app.go:186`) — a single snapshot covering the whole operation. There is no per-field undo inside `SaveCharacter`.

A subsequent `WriteSave` writes the file to disk (with a `.sl2.YYYYMMDD_HHMMSS.bkp` backup). User-facing rollback paths:

1. **Undo stack** (in-memory) — works up to `WriteSave` or application restart.
2. **Backup file restore** — after `WriteSave`, the only path to revert.

## 17. Test coverage

| Test | File | What it verifies |
|---|---|---|
| Round-trip `PreEventFlagsScalars` + `EventFlagsBlock` | `backend/core/section_eventflags_test.go:11+` | Parse + serialize identity |
| Round-trip slot integrity | `backend/core/section_world_geom_test.go:18+` | `PreEventFlagsScalars` parsed after `Read` |
| `TestEventFlagsOffsetCorrectness` | `tests/map_flags_test.go:145` | EventFlagsOffset points at the correct bitfield (by verifying known graces visited) |
| `pvp_test.go` | `pvp_test.go` (10 tests) | See [48](48-pvp-ready-modular-presets.md) §17 — including event flag round-trip |

**Missing**:

- An isolated `SaveCharacter` test with ClearCount = N → assert NG+ event flags 50-57 sync.
- Validation of the ClearCount cap @ 7 at the public API level (a VM-level test yes, public API no).
- A cross-platform parity test for the ClearCount offset.
- A rollback test after a failure mid-`SaveCharacter`.

## 18. Known limits / needs verification

1. **`CharacterType` i32 values** — the comment in the old doc was "0=Host, 1=WhitePhantom, 2=DarkSpirit, 3=Ghost"; the current code does not validate this mapping. `needs verification`.
2. **`InGameCountdownTimer` semantics** — completely unknown, probably a game session timer or quest timer.
3. **`NotAloneFlag` boolean ranges** — `needs verification` what sets it (co-op, invasion, NPC summon?).
4. **`UnkGameDataMan0x124`** — completely unknown.
5. **Second occurrence of BonfireId in NetworkManager** — empirical, not parsed as a field.
6. **Boss multi-flag editor** — planned, see [38](38-boss-multiflag.md).
7. **BonfireId ↔ Grace EventFlag mapping** — none in SaveForge.
8. **ClearCount progression validator** — none; possible boss/quest desync.
9. **NG+ event flags 50-57 as the single source of truth** — `needs verification` whether the game uses the flags or the `ClearCount` field.
10. **PreEventFlagsScalars unknown fields** — 4 of 11 fields unknown.
11. **Platform parity of the ClearCount offset** — no isolated test.
12. **`SaveCharacter` rollback after partial mutation** — none; the user must restore from a backup.

## 19. Cross-references

- [01-header.md](01-header.md) — SteamID in `TrailingFixedBlock`; not part of Game State, but a related identifier.
- [15-event-flags.md](15-event-flags.md) — the event flags bitfield and NG+ flags 50-57.
- [16-world-state.md](16-world-state.md) — World State overview; Game State is a separate domain.
- [17-player-coordinates.md](17-player-coordinates.md) — `PlayerCoordinates`, a separate read-only section.
- [19-weather-time.md](19-weather-time.md) — `WorldAreaWeather` / `WorldAreaTime` read-only blobs.
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — Grace EventFlag (71xxx-76xxx) NS, disjoint from BonfireId.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — PvP modular presets (world state orchestrator, not Game State).
- [50-item-companion-flags.md](50-item-companion-flags.md) — item lifecycle hooks, side-effect on event flags.
- [38-boss-multiflag.md](38-boss-multiflag.md) — the planned boss multi-flag editor.

## 20. Sources

- `backend/core/section_eventflags.go` — `PreEventFlagsScalars` (29 B), `EventFlagsBlock`.
- `backend/core/structures.go` — `Player.ClearCount`, `SaveSlot.ClearCountOffset`.
- `backend/core/offset_defs.go` — `DynClearCount = 0x44`.
- `backend/vm/character_vm.go` — `CharacterViewModel.ClearCount` + cap @ 7 in `ApplyVMToParsedSlot`.
- `app.go::SaveCharacter` (lines 178-217) — write path + NG+ event flag sync (lines 204-209).
- `app_world.go:41` — explicit comment "Does not touch LastRestedGrace".
- `frontend/src/components/DatabaseTab.tsx` — `clearCount` as a read-only input for item caps scaling.
- Tests: `backend/core/section_eventflags_test.go`, `section_world_geom_test.go`, `tests/map_flags_test.go`, `pvp_test.go`.
