# 46 — Fast Invasions Research

> **Type**: Design doc
> **Status**: ✅ Investigation Complete
> **Scope**: End-to-end investigation into PS4/PC Elden Ring invasion matchmaking mechanics
> through save file analysis: UD11 NetworkParam patching, UD10 session state, UD0
> MatchmakingCandidateSection binary structure, and final state machine model.

---

## Save File Architecture

Elden Ring `.sl2` / `.dat` is a BND4 container with exactly 12 sections:

```
UserData0–9   — character slots (per-character save data, 0x280000 bytes each)
UserData10    — system data (0x60000 bytes / 384 KB)
UserData11    — embedded regulation.bin snapshot/cache; not proven to be runtime source
```

Base offsets (file-absolute):

```
UD0  = 0x70
UD1  = 0x70 + 0x280000
...
UD10 = 0x70 + 10 × 0x280000          = 0x1900070  (size: 0x60000, not 0x280000)
UD11 = 0x70 + 10 × 0x280000 + 0x60000 = 0x1960070
```

---

## Saves Analyzed

| Label | File | Session State |
|-------|------|---------------|
| H | `oisis_pl-vanilla-nopvpactivity.dat` | vanilla, passive/session-ready |
| J | `oisis_pl-vanilla-bf-on.dat` | vanilla, BF-init (queue active) |
| F | `oisis_pl-pvp-ready-netparam-finger-on.dat` | patched UD11, full active BF |
| I | `oisis_pl-vanilla-invasion-timeout.dat` | vanilla, invasion success/cleared |
| G | `oisis_pl-pvp-ready-netparam-invasion-timeout-break.dat` | patched UD11, timeout/cleared |
| E | `oisis_pl-pvp-ready-netparam-nopvpactivity.dat` | patched UD11, idle (unexplained UD10 state) |

Scripts: `tmp/scripts/diag/bf_statemachine.go`, `bf_targetlist.go`, `bf_candidatelist.go`
Raw output: `tmp/regulation-bin-debug/final-report.md`

---

## 1. Initial Hypothesis

The goal: find save file parameters that make PvP invasions faster or more frequent.

**Working hypothesis**: `NetworkParam` inside `regulation.bin` (UserData11) controls invasion
timing. Editing `breakInRequestIntervalTimeSec` (30s → 5s) and `breakInRequestTimeOutSec`
(20s → 5s) should accelerate invasions.

**Secondary hypothesis**: NetworkParam values might also be cached somewhere in UD10,
allowing a save-file-only edit without touching UD11.

**Investigation path**:

1. Full UD10 scan for NetworkParam signatures → stable regions mapped, no persistent NetworkParam
2. NetMan structure inside character slots → read-only history cache, not configurable
3. EventFlags → quest/area flags, not timing parameters
4. UD11 regulation.bin → values writable, but runtime effect unconfirmed (see §3)
5. UD10 BF state machine → real session state discovered (`UD10+0x5070`)
6. UD0 binary scan → `MatchmakingCandidateSection` at `UD0+0x209B00..0x209C43` found

---

## 2. UD11 Decomposition

### File Structure

**PC** (`.sl2`):

```
ud11[0x00:0x10]   MD5 checksum of ud11[0x10:]
ud11[0x10:]       AES-256 encrypted + DCX compressed regulation.bin
```

**PS4** (`.dat`):

```
ud11[0x00:0x10]   GER header (not MD5)
ud11[0x10:]       embedded regulation.bin blob, including IV + AES-encrypted compressed payload
```

> Note: tool analyses operate on raw/decrypted `.dat` files exported from the console.
> Whether the live PS4 OS layer adds further encryption before writing to NAND is out of scope.

### Sanity Checks (PC)

| File | MD5 prefix | MD5(ud11[0x10:]) | Match? |
|------|------------|------------------|--------|
| vanilla | `7256cc79…` | `7256cc79…` | ✅ |
| patched, no MD5 recalc | `7256cc79…` | `317a411a…` | ❌ → game reverts |
| game-rewritten post-patch | `7256cc79…` | `7256cc79…` | ✅ |

**Root cause of old PC patch failure**: `PatchNetworkParams` re-encrypted the ciphertext but
left the original MD5 prefix. The game detected the mismatch and reverted to its local copy.
**Fix**: recalculate `ud11[0x00:0x10] = MD5(ud11[0x10:])` after patching.
Implemented in `backend/core/regulation.go` (2026-05-08). → spec/44

**PS4**: patched values survive the upload/download cycle and are readable after downloading
back from the console. The earlier assumption that "the server always overwrites" was
unverified and has been removed.

### regulation.bin Contents

Inside the DCX blob: BND4 container of `.param` tables.
`NetworkParam.param` contains the relevant network timing row used in this analysis.

Key fields used in this research:

| Parameter | CSV offset | Vanilla | Patched | Description |
|-----------|-----------|---------|-------------------|-------------|
| `maxBreakInTargetListCount` | `0x70` | 5 | 10 | In-memory target list size; >200 crashes client |
| `breakInRequestIntervalTimeSec` | `0x74` | 30.0 | 5.0 | How often game sends invasion request [s] |
| `breakInRequestTimeOutSec` | `0x78` | 20.0 | 5.0 | Wait for server response before retry [s] |
| `breakInRequestAreaCount` | `0x7C` | 5 | 10 | Regions searched per attempt (field hidden in PARAMDEF as `dummy8 pad[4]`) |

> `breakInRequestAreaCount` offset confirmed at `0x7C` via `backend/core/regulation.go`;
> labelled `dummy8 pad[4]` in PARAMDEF — FromSoftware intentionally hid it from editors.

---

## 3. NetworkParam Test

### UD10 Scan for NetworkParam Values

Full scan of the two stable UD10 regions for `f32=30.0`, `f32=20.0`, `f32=5.0`:

- Scattered hits exist in volatile UD10 (`0x00C5F8`, `0x020BE8`) — reset to `0.0` after the
  game overwrites UD10 on next save.
- **Stable regions contain no persistent NetworkParam copy.**

### Cross-Save Comparison (Patched vs Vanilla)

Saves with patched UD11 (F, G, E) vs saves with vanilla UD11 (H, I, J):

- `UD10+0x5070` state word: no difference attributable to NetworkParam values
- `UD0` V-queue configuration: identical layout for equivalent BF states
- Patched saves show the same state machine transitions as vanilla saves

**Result**: No observable UD0 or UD10 difference between patched and vanilla UD11 in
equivalent session states.

### Rejected lead: NetMan history cache

NetMan appears to be an encounter/history cache, not a tunable matchmaking configuration.

NetMan is a 131,076-byte structure inside each character slot. It is a **read-only history
cache** — not a configuration area.

```
NetMan total = 0x20004 bytes
  ├── unk0x0       4 bytes   always 2
  └── data     0x20000 bytes
        ├── header      0x0A0 bytes
        ├── records   0x134A0 bytes  (128 × 0x268 player encounter records)
        └── tail      0x0CB60 bytes  (zeros)
```

Sub-list headers at `data[0x000..0x01F]`:

| Offset | Field | Value | Meaning |
|--------|-------|-------|---------|
| `0x000` | `list0_type` | 2 | Coop/summon sign history |
| `0x004` | `list0_capacity` | 8 | Max cached entries |
| `0x010` | `list1_type` | **5** | Invasion target history |
| `0x014` | `list1_capacity` | 8 | Max cached entries |

`list1_type=5` classifies this as invasion break-in target history. The value 5 is an
internal classifier — it is NOT related to `NetworkParam.maxBreakInTargetListCount=5`.

Attempted edits (all reset by game on next save):
- `list1_capacity` 8 → 10: no effect
- `breakInRequestIntervalTimeSec` in tail region (`0x134A0+`): game resets to `0.0`

**Conclusion**: NetMan is a history log. Editing it does not affect matchmaking behavior.

### Open Question

Whether the game runtime reads NetworkParam **from UD11** or from a **separate
in-installation copy** is unconfirmed. They are structurally indistinguishable from the
save file alone. Definitive verification requires measuring actual in-game invasion
intervals (seconds between attempts) before and after patching.

---

## 4. UD10 Runtime / Session State

UD10 = 384 KB system data. ~90% is volatile (rewritten on every game save).

Two stable regions:

| Region | Range | Content |
|--------|-------|---------|
| Stable 1 | `0x000000–0x001984` | System settings (graphics, audio, matchmaking toggle) |
| Stable 2 | `0x001988–0x00511C` | FaceData profile cache + matchmaking sub-area ID cache |

**`perform_matchmaking` at `UD10[0x0013]`**: `0x01` = online, `0x00` = offline.
Confirmed online/offline toggle — not an acceleration parameter.
This is the only directly actionable network setting confirmed in the save file.

### BF State Markers

| Marker | Description |
|--------|-------------|
| `UD10+0x5070` | Primary BF state word (most reliable discriminator) |
| `UD10+0x194E4` | `0xFFFFFFFF` = ACTIVE-BF sentinel (F only) |
| `UD10+0x19504` | `0xFFFFFFFF` = PATCHED-IDLE sentinel (E only) |
| `UD10+0x19508` | `f32=90.0` = active search window timer (F only) |
| `UD10+0x194F4` | `f32=-150.0` = active search countdown (F only) |
| `UD10+0x19524` | `f32≈0.0333` — likely reciprocal interval / tick rate field candidate (ACTIVE-BF / F only; origin unconfirmed) |
| `UD10+0x42C54` | `f32` — coordinates or counter, active search (J/F) |
| `UD10+0x42C58` | `f32` — coordinates or counter, active search (J/F) |
| `UD10+0x5080` | `f32=1.0` = invasion success/cleared (I only) |

### Full Marker Table

| Marker | H | J | F | I | G | E |
|--------|---|---|---|---|---|---|
| `UD10+0x5070` | `0x0100018F` | `0x00000001` | `0x01000081` | `0x00000000` | `0x00000000` | `0x00002F60` |
| `UD10+0x194E4` | `0` | `0` | **`0xFFFFFFFF`** | `0` | `0` | `0` |
| `UD10+0x19504` | `0` | `0` | `0` | `0` | `0` | **`0xFFFFFFFF`** |
| `UD10+0x19508` | `0` | `~0` | **`f32=90.0`** | `~0` | `~0` | `0` |
| `UD10+0x194F4` | `0` | `0` | **`f32=-150.0`** | `0` | `0` | `0` |
| `UD10+0x42C54` | `0` | `f32=2.0` | `f32=-15.0` | `0` | `0` | `0` |
| `UD10+0x42C58` | `0` | `f32=1.0` | `f32=0.1` | `0` | `0` | `0` |
| `UD10+0x5080` | `0` | `~0` | `~0` | **`f32=1.0`** | `0` | `0x01000054` |

`~0` = near-zero float noise (not a signal value).

### UD10+0x5070 Full Distribution (Batch — 62 active slots)

Complete value distribution observed across all 25 save files, 62 active slots:

| Value | Count | Classifier label | Notes |
|-------|-------|-----------------|-------|
| `0x00000000` | 20 | CLEARED | success + timeout + some zeroed slots |
| `0x3F4A1AF3` | 14 | **UNKNOWN** | likely baseline — no PvP session history |
| `0xFF545C5D` | 8 | **UNKNOWN** | likely uninitialized (`oisisk_ps4.txt`) |
| `0x0100018F` | 6 | PASSIVE | confirmed primary state |
| `0x30307964` | 4 | **UNKNOWN** | garbage / LE bytes = ASCII `dy00` (`oisis_pl-org.txt`) |
| `0xC20FFFFF` | 4 | **UNKNOWN** | unknown; possible timer artifact |
| `0x00000001` | 2 | BF-INIT | confirmed primary state |
| `0x00002F60` | 2 | PATCHED-IDLE | confirmed primary state |
| `0x01000081` | 2 | ACTIVE-BF | confirmed primary state |

The 4 UNKNOWN values (`0x3F4A1AF3`, `0xFF545C5D`, `0x30307964`, `0xC20FFFFF`) together
account for 30/62 slots (48%). All 30 slots also have `sect_result = NOT-INITIALIZED` or
`DEVIATES` — their UD0 candidate sections are uninitialized or garbage. These values
should **not** be added to the primary BF state machine until reproduced in SPEC-VALID
active session states where UD0 is also valid.

---

## 5. UD0 MatchmakingCandidateSection

> **SPEC-VALID inference caveat**: All structural conclusions in this section are based
> on `SPEC-VALID` slots only. Batch analysis (62 active slots across 25 files) found:
> 8 SPEC-VALID (all PS4), 35 NOT-INITIALIZED, 19 DEVIATES. Raw aggregates across all
> slots contain significant noise — candidate ID values, flag patterns, and queue layouts
> from NOT-INITIALIZED / DEVIATES slots must not be counted as evidence for or against
> any structural claim.

### Location

```
UD0+0x209B00..0x209C43
File offset = UD0 base (0x70) + 0x209B00 = 0x209B70
Total size  = 0x144 = 324 bytes
```

### Layout Map

```
UD0 offset    size    subsection
0x209B00      0x14    Header record        (record_type=0x00014000, CONST)
0x209B14      0x14    A01 entry[0]         (record_type=0x00000C00, CONST)
0x209B28      0x14    A01 entry[1]         (record_type=0x00000C00, CONST)
0x209B3C      0x64    Static block C0-C4   (5 × 0x14, CONST)
0x209BA0      0xA0    Dynamic queue V0-V7  (8 × 0x14, state-dependent)
0x209C40      0x04    Terminator           (0x00000000)
```

### CandidateEntry Struct (stride = 0x14)

```c
struct CandidateEntry {
    uint32_t record_type;  // +0x00  CONST = 0x00000C00
    uint32_t entry_id;     // +0x04  matchmaking_entry_id / candidate_id
    uint32_t flag_a;       // +0x08  entry class
    uint32_t flag_b;       // +0x0C  selection state
    uint32_t flag_c;       // +0x10  positional sentinel (V7 only)
};
```

Flag semantics:

| Field | Value | Meaning |
|-------|-------|---------|
| `flag_a` | `0x00000A3E` | target-class — selected for invasion |
| `flag_a` | `0x00000A00` | regular candidate |
| `flag_a` | `0x00000A01` | A01-subtype (special/NPC, pre-list block only) |
| `flag_b` | `0x01010000` | selected (active match target) |
| `flag_b` | `0x01000000` | registered (known to session, not selected) |
| `flag_c` | `0x00FFFF00` | tail sentinel — **always at physical V7**, does NOT travel with `entry_id` |
| `flag_c` | `0x00000000` | regular position |

### Header Record (CONST, all 6 saves)

```
UD0+0x209B00:  record_type=0x00014000  unk04=0x00000100  unk08=0x00000100
               unk0C=0x00000100        unk10=0x00000000
```

`0x00014000` type tag is unique to this header — no CandidateEntry shares it.
`unk04/08/0C=0x00000100`: semantics unknown (count? capacity? type flags?).

### A01 Entries (CONST, all 6 saves)

```
UD0+0x209B14:  id=0x12B01F00  flag_a=0x00000A01  flag_b=0x01000000  flag_c=0x00000000
UD0+0x209B28:  id=0x12B01E00  flag_a=0x00000A01  flag_b=0x01000000  flag_c=0x00000000
```

`flag_b=0x01000000` (registered, not selected). Not target-class. Semantics unknown
(NPC hosts? special session type? area markers?).

### Static Block C0-C4 (CONST, all 6 saves)

```
C0  +0x209B3C  id=0x21556E00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
C1  +0x209B50  id=0x30498E00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
C2  +0x209B64  id=0x11C50E00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
C3  +0x209B78  id=0x212E5F00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
C4  +0x209B8C  id=0x212E5E00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
```

All permanently marked as target-class + selected (`flag_a=A3E`, `flag_b=0x01010000`).
Present in all states. `entry_id` values have zero cross-references outside this section.

### Dynamic Queue V0-V7

Two configurations depending on BF state:

**IDLE** (saves H / I / G / E):

| pos | UD0 offset | `entry_id` | `flag_a` | `flag_b` | `flag_c` |
|-----|-----------|-----------|---------|---------|---------|
| V0 | `+0x209BA0` | `0x989E2000` | `A00` | `0x01000000` | `0x00` |
| V1 | `+0x209BB4` | `0x989E2100` | `A00` | `0x01000000` | `0x00` |
| V2 | `+0x209BC8` | `0x989E2200` | `A00` | `0x01000000` | `0x00` |
| V3 | `+0x209BDC` | `0x989E2300` | `A00` | `0x01000000` | `0x00` |
| V4 | `+0x209BF0` | `0x989E2400` | `A00` | `0x01000000` | `0x00` |
| V5 | `+0x209C04` | `0x989E2500` | `A00` | `0x01000000` | `0x00` |
| V6 | `+0x209C18` | `0x989E2600` | `A00` | `0x01000000` | `0x00` |
| **V7** | **`+0x209C2C`** | **`0x3097AE00`** | **`A3E`** | **`0x01010000`** | **`0x00FFFF00`** |

**ACTIVE BF** (saves J / F):

| pos | UD0 offset | `entry_id` | `flag_a` | `flag_b` | `flag_c` |
|-----|-----------|-----------|---------|---------|---------|
| **V0** | **`+0x209BA0`** | **`0x3097AE00`** | **`A3E`** | **`0x01010000`** | **`0x00`** |
| V1 | `+0x209BB4` | `0x989E2600` | `A00` | `0x01000000` | `0x00` |
| V2 | `+0x209BC8` | `0x989E2500` | `A00` | `0x01000000` | `0x00` |
| V3 | `+0x209BDC` | `0x989E2400` | `A00` | `0x01000000` | `0x00` |
| V4 | `+0x209BF0` | `0x989E2300` | `A00` | `0x01000000` | `0x00` |
| V5 | `+0x209C04` | `0x989E2200` | `A00` | `0x01000000` | `0x00` |
| V6 | `+0x209C18` | `0x989E2100` | `A00` | `0x01000000` | `0x00` |
| **V7** | **`+0x209C2C`** | **`0x989E2000`** | `A00` | `0x01000000` | **`0x00FFFF00`** |

Key invariants:
- `flag_c=0x00FFFF00` is bound to **physical position V7** — it does NOT follow `0x3097AE00`
- `flag_a` and `flag_b` are **entry properties** — they travel with `entry_id` through all reorderings
- Non-target entries V1-V6 in ACTIVE state = **exact REVERSE** of their IDLE order (not a rotation)

### Candidate ID Stability — Batch Analysis Note

Batch analysis across 25 save files (62 active slots) showed **9 distinct A01 signature
variants** and **9 distinct C0-C4 signature variants** in raw aggregates. These apparent
variants are noise: all non-canonical variants come exclusively from `NOT-INITIALIZED`
and `DEVIATES` slots containing uninitialized or garbage data at `UD0+0x209B00..0x209C43`.

After filtering to `SPEC-VALID` slots only (8 PS4 slots):

- **A01 entries** have exactly 1 known variant: `0x12B01F00 | 0x12B01E00`
- **C0-C4 entries** have exactly 1 known variant: `0x21556E00 | 0x30498E00 | 0x11C50E00 | 0x212E5F00 | 0x212E5E00`

Global stability of these IDs is confirmed **for SPEC-VALID slots only**. Raw aggregate
counts from all 62 slots are meaningless for ID stability claims.

### Terminator

`UD0+0x209C40`: `0x00000000` (4 bytes).
All of `0x209C40..0x209D00` is zero in all 6 saves — no head/tail pointer found.

### Cross-Reference Scan

All `entry_id` values from the candidate list searched in:
- Full UD0 (0x280000 bytes) outside the list region → **zero hits**
- Full UD10 (0x60000 bytes) → **zero hits**
- UD10+0x42B00..0x42E00 (dedicated search block) → **zero hits**

Entry IDs are self-contained within the section.

---

## 6. Final State Machine

### States

| State | Save | `UD10+0x5070` | V-queue | Description |
|-------|------|--------------|---------|-------------|
| PASSIVE | H | `0x0100018F` | IDLE | Session-ready, no active invasion search |
| BF-INIT | J | `0x00000001` | ACTIVE | Queue rewritten, search initiated |
| ACTIVE-BF | F | `0x01000081` | ACTIVE | Full active BF, timers running |
| SUCCESS | I | `0x00000000` | IDLE | Invasion completed successfully |
| TIMEOUT | G | `0x00000000` | IDLE | Invasion search timed out |
| PATCHED-IDLE | E | `0x00002F60` | IDLE | Unexplained state (patched UD11 only) |

### Transition Graph

```
PASSIVE (H)
  UD10+0x5070 = 0x0100018F
  V-queue: IDLE — target 0x3097AE00 @ V7
        │
        │  Festering Bloody Finger used
        ▼
  BF-INIT (J)
  UD10+0x5070 = 0x00000001
  V-queue: ACTIVE — target 0x3097AE00 promoted V7→V0
                    remaining V1-V6 reversed
        │
        │  match found, connection established
        ▼
  ACTIVE-BF (F)
  UD10+0x5070 = 0x01000081
  UD10+0x194E4 = 0xFFFFFFFF
  UD10+0x19508 = f32=90.0 (search window timer)
  UD10+0x194F4 = f32=-150.0 (countdown)
  V-queue: ACTIVE
        │
        ├────────────────────────────────────┐
        │ invasion resolves                  │ timer expires
        ▼                                    ▼
  SUCCESS (I)                          TIMEOUT (G)
  UD10+0x5070 = 0x00000000             UD10+0x5070 = 0x00000000
  UD10+0x5080 = f32=1.0                UD10+0x5080 = 0x00000000
  V-queue: IDLE (target reset to V7)   V-queue: IDLE (target reset to V7)
```

**PATCHED-IDLE (E)** does not fit the above graph. `UD10+0x5070=0x00002F60` and
`UD10+0x19504=0xFFFFFFFF` have no equivalent in the other 5 saves. May represent a partial
online-init code path introduced by the NetworkParam patch interacting with a session
state the vanilla code does not reach.

---

## 7. Minimal Classifier

The following 4 fields uniquely identify all 6 observed BF states:

```
Field 1:  UD0+0x209BA4   (= V0.entry_id in dynamic queue)
Field 2:  UD10+0x5070    (primary BF state word)
Field 3:  UD10+0x194E4   (ACTIVE-BF sentinel)
Field 4:  UD10+0x5080    (success/cleared marker)
```

### Classification Table

| Save | `UD0+0x209BA4` | `UD10+0x5070` | `UD10+0x194E4` | `UD10+0x5080` | State |
|------|---------------|--------------|--------------|--------------|-------|
| H | `0x989E2000` | `0x0100018F` | `0x00000000` | `0x00000000` | PASSIVE |
| J | `0x3097AE00` | `0x00000001` | `0x00000000` | `0x00000000` | BF-INIT |
| F | `0x3097AE00` | `0x01000081` | `0xFFFFFFFF` | `0x00000000` | ACTIVE-BF |
| I | `0x989E2000` | `0x00000000` | `0x00000000` | `0x3F800000` | SUCCESS |
| G | `0x989E2000` | `0x00000000` | `0x00000000` | `0x00000000` | TIMEOUT |
| E | `0x989E2000` | `0x00002F60` | `0x00000000` | `0x01000054` | PATCHED-IDLE |

**Note**: with only 3 fields (`UD0+0x209BA4`, `UD10+0x5070`, `UD10+0x194E4`), states I and
G are **indistinguishable** — both return `(0x989E2000, 0x00000000, 0x00000000)`.
`UD10+0x5080` is required to resolve this.

### Decision Tree

```
UD0+0x209BA4 == 0x3097AE00?
├── YES → BF active
│   ├── UD10+0x5070 == 0x00000001  →  BF-INIT
│   └── UD10+0x5070 == 0x01000081  →  ACTIVE-BF
└── NO  (= 0x989E2000)
    ├── UD10+0x5070 == 0x0100018F  →  PASSIVE
    ├── UD10+0x5070 == 0x00002F60  →  PATCHED-IDLE
    └── UD10+0x5070 == 0x00000000
        ├── UD10+0x5080 == 0x3F800000  →  SUCCESS
        └── UD10+0x5080 == 0x00000000  →  TIMEOUT
```

### Slot-Level Classifier Caveat

UD10 is **global to the save file**, while UD0 is **per character slot**. During an active
invasion session, inactive slots will see the global UD10 state (e.g., `0x01000081`), but
their own UD0 candidate queue may be empty or unrelated to the active session.

Example: `oisis_pl-pvp-ready-netparam-finger-on.dat` slot 1 (`Bydlaczka_150`) had
`UD10+0x5070=0x01000081`, but `V0ID=0x00000000` (no candidate queue), so the classifier
correctly returns `UNKNOWN` rather than `ACTIVE-BF`.

The classifier must require **both**:
- global UD10 state matching a known BF pattern, AND
- matching per-slot UD0 candidate queue in SPEC-VALID state (`sect_result=SPEC-VALID`).

A non-zero V0 entry alone is not sufficient — DEVIATES slots may have non-zero bytes at
the V0 offset that are garbage, not a valid candidate ID.

### Structural Deviations Summary

Batch analysis found 19 DEVIATES slots across 25 save files. Key patterns:

- All 10 PC BND4 active slots with characters (in `ER0000.sl2`, `ER0000-out.sl2`) have
  random bytes at the candidate section offset — either `0xFF`-filled or garbage structs.
- Several PS4 saves (`oisis_pl-org.txt`, `oisis_pl-pvp-ready.dat`, intermediate `.dat`
  files) show partial or byte-swapped structures at this offset, indicating the section
  was written in a different layout or left uninitialized.
- `ER0000-kro55-vanilla.sl2` (PC, single slot, `Random` lvl 9) is NOT-INITIALIZED.

This confirms: structural deviations do not indicate a parser bug. In the current dataset,
the `MatchmakingCandidateSection` at `UD0+0x209B00` produced SPEC-VALID results only in
PS4 saves with prior online invasion session history.

---

## 8. Confirmed Findings

All items below are hex-verified and consistent across all 6 saves.

- CandidateEntry struct: stride=`0x14`, 5 fields at offsets `+0x00..+0x10`
- `record_type=0x00000C00` for all 13 entries in the main candidate list
- Section bounds: `UD0+0x209B00..0x209C43`
- Subsection sizes: header `0x14`, A01-block `0x28`, C0-C4 `0x64`, V0-V7 `0xA0`, terminator `0x04`
- Header, A01-block, C0-C4 block: CONST in all 6 saves
- `flag_a` and `flag_b` are entry properties — they follow `entry_id` through all queue reorderings
- `flag_c=0x00FFFF00` is a positional marker — always at physical V7, regardless of which `entry_id` occupies it
- BF activation = physical rewrite of V-queue — **NOT a ring buffer with pointer**
- Rotation pattern: target moves V7 → V0; remaining 7 entries are REVERSED (V6..V0 order)
- No external head/tail pointer exists in UD0 (`0x209C40..0x209D00` and `0x209AE0..0x209B00` are all zeros)
- Zero cross-references for any `entry_id` in full UD0 (0x280000 bytes) and full UD10 (0x60000 bytes) outside the list region
- `UD10+0x5070` correlates with BF state — confirmed across all 6 saves
- `UD10+0x194E4=0xFFFFFFFF` → unique marker for ACTIVE-BF (F only)
- `UD10+0x5080=f32=1.0` → unique marker for SUCCESS (I only)
- `UD10+0x19504=0xFFFFFFFF` → unique marker for PATCHED-IDLE (E only)
- States I (success) and G (timeout): both have `UD10+0x5070=0x00000000`; distinguished only by `UD10+0x5080`
- NetworkParam values from patched UD11 are NOT present in any stable UD0 or stable UD10 region
- Saves with patched vs vanilla UD11: `UD10+0x5070` and UD0 V-queue show NO difference attributable to NetworkParam

---

## 9. Likely Interpretations

All items below are consistent with all data and have no counter-evidence, but are not
directly confirmed from save file data alone. Treat as working hypotheses.

- `flag_b=0x01010000` = "selected for matchmaking" — second byte `0x01` is the selection bit
- `flag_b=0x01000000` = "registered in session" — present but not actively selected
- `flag_a=0x00000A3E` = target-class / invasion priority target (preferred candidate)
- `flag_a=0x00000A00` = regular candidate (lower priority)
- `flag_a=0x00000A01` = special class — different handling than A00/A3E; possibly NPC hosts
  or a distinct session subtype
- `UD10+0x5070` MSB byte = activity state; LSB word = substatus code
- `UD10+0x19508=f32=90.0` = invasion search window length in seconds
- `UD10+0x194F4=f32=-150.0` = countdown timer from -150 toward 0 during active search
- `UD10+0x19524=f32≈0.0333` may be a reciprocal interval / tick rate field used during
  active search (1/30 = `breakInRequestIntervalTimeSec` reciprocal in vanilla regulation)
- `UD10+0x42C54/0x42C58` = coordinates or elapsed-time counters used during active search
- V-queue rewrite = "promoted LRU": selected target goes to head, remaining entries reversed
  (consistent with MRU stack pop semantics — most recently encountered candidates bubble down)
- UD11 NetworkParam patch values survive on PS4 but may not affect gameplay if the runtime
  loads params from an in-installation copy rather than from UD11

---

## 10. Unknown / Cannot Be Determined Offline

- **Semantic meaning of individual `entry_id` values** (`0x989E20xx`, `0x3097AE00`,
  `0x12B01Exx`) — zero cross-references found anywhere in UD0 or UD10.
  Do **NOT** call these "PSN account IDs" or "host IDs" — use `candidate_id` or
  `matchmaking_entry_id`. Origin is unconfirmed.
- Why non-target entries are **REVERSED** rather than rotated on BF activation — algorithm
  artifact? MRU/LIFO stack behavior?
- What `header.unk_04/08/0C=0x00000100` encodes (count? capacity? reserved flags?)
- What A01-class entries (`0x12B01Exx`) represent — constant across all saves, not target-class
- Whether `flag_c=0x00FFFF00` encodes the 16-bit value `0xFFFF` semantically or is simply
  a magic sentinel pattern
- Whether I and G differ in any UD field beyond `UD10+0x5080` — offline data insufficient
- `E`'s `UD10+0x5070=0x00002F60` — timer residue? partial online-init path? no match to any
  known NetworkParam or timing field
- Why `UD10+0x19504=0xFFFFFFFF` appears in E (patched-idle) and not in any other idle save
- Full semantics of `UD10+0x42B00..0x42E00` — heterogeneous struct, no decoder available
- **`UD10+0x19524` origin**: field observed only in ACTIVE-BF save (F); likely `f32=1/30`
  (reciprocal of vanilla `breakInRequestIntervalTimeSec=30s`), but origin is unknown —
  could be canonical runtime params, server state, or local regulation. This field does NOT
  prove UD11 NetworkParam runtime usage.
- Whether the game reads NetworkParam from UD11 or from an in-installation copy — requires
  measuring actual invasion interval timing before and after patching
- **`UD10+0x42B00..0x42E00` float array**: region contains dense `f32=1.0` values in bf-on
  and org saves, scattered non-zero offsets in other states. This is a heterogeneous float
  array (weight table / probability table) rather than a single scalar marker; do not treat
  individual `f32=1.0` hits as independent signals. No decoder is available.

---

## 11. Final Verdict

### Hex-confirmed findings

All items below are verified from binary analysis of 6 known save states. They do not
depend on gameplay observation or runtime measurements.

**UD11 NetworkParam patch does not measurably affect UD0/UD10 runtime state.**
Patched values survive in the PS4 file and are readable after upload/download.
No copy appears in stable UD0 or UD10 regions. BF state machine shows no observable
difference between patched and vanilla saves in equivalent session states.

**Real matchmaking/search state is represented in UD10 and UD0.**
Primary session state word: `UD10+0x5070`.
Active candidate queue: `UD0+0x209BA0..0x209C3F`.
These are the ground-truth indicators of what the game engine is doing with invasion
matchmaking at save time — not regulation.bin.

**The most valuable structure is `UD0+0x209B00..0x209C43`.**
The `MatchmakingCandidateSection` contains the complete invasion candidate list in a
well-defined binary format. CandidateEntry (stride=0x14) with `entry_id`, `flag_a`,
`flag_b`, `flag_c` fields is fully characterized. The queue is physically rewritten on
BF activation with a confirmed rotation pattern.

**Do not call `entry_id` a PSN account ID or host ID.**
Use neutral names: `candidate_id` or `matchmaking_entry_id`. No cross-references exist
to decode these values from the save file alone.

---

### Gameplay-observed practical mechanism

> This section is a **gameplay/practical observation**, separate from the hex-confirmed
> UD0/UD10 state-machine analysis above. The UD0/UD10 analysis proves where matchmaking
> state is represented in the save, but does not by itself prove that Summoning Pool flags
> are the sole cause of faster invasions.

> ⚠️ **Dataset limitation — Summoning Pool flags**: Offline analysis of the `pvp-ready`
> save pair shows only **one flag** (`670101`) in the `670xxx` range, with no difference
> between vanilla and pvp-ready. Root cause identified: the preset JSON used to prepare
> these saves contains summoning pool IDs in the **old pre-DLC format** (`10000040`,
> `1035530040`…), not the current `670xxx` IDs used by `backend/db/data/summoning_pools.go`.
> These old IDs are being set in the save (18 flags observed in the 1.0e9 range in the
> vanilla→pvp-ready diff), but are not recognised by current game versions for summon pool
> discovery. The `670xxx` ID database (213 entries, 670100–670980) is correct, but has not
> been applied to the pvp-ready preset JSON.
>
> The absence of broad `670xxx` changes in this dataset does **not** prove Summoning Pools
> are irrelevant to matchmaking — it proves the current `pvp-ready` dataset does not
> correctly activate them. **Summoning Pool flag mapping remains an open question. Further
> validation requires a save pair where pools are toggled by the game itself or by applying
> the current `670xxx` ID map from `summoning_pools.go`.**

**Best practical save-file lever found so far: all regions unlocked + PvP items.**

Summoning Pools are primarily a **coop/summon sign discovery mechanism** — they control
host visibility to potential co-op phantoms and sign puddle availability. A possible
indirect relation to Bloody Finger invasion matchmaking eligibility (via area/world flags)
has not been confirmed. Current offline data does not identify Summoning Pools as the
confirmed mechanism behind faster active invasions.

Observed evidence (not hex-confirmed causality):
- regulation.bin was successfully edited (no crash), patched values readable on PS4 — but
  whether the runtime uses them is unverified
- Fast invasions observed on PS4/PS5 without any specific items equipped, across different
  areas; the exact save-file lever responsible could not be isolated in the offline analysis
- Around the v1.12-era data, pool flag IDs were expected in the `670xxx` range; however,
  offline analysis of the current `pvp-ready` dataset does not confirm this range is set
  correctly by the existing editor (see dataset limitation above)

---

### Unknowns (cannot be resolved offline)

- Whether the game runtime reads NetworkParam from UD11 or from a separate in-installation
  copy — requires measuring live invasion interval before and after patching
- Exact semantic meaning of individual `candidate_id` values (`0x989E20xx`, `0x3097AE00`)
- Exact server-side interpretation of Summoning Pool flags and region IDs
- Whether any UD10/UD0 field directly encodes "which pool flags are active"
- Whether Summoning Pool flags (670100–670980 in `summoning_pools.go`) have any direct
  effect on Bloody Finger invasion matchmaking, or only affect coop/summon sign visibility —
  requires a test save where all 213 `670xxx` flags are set and invasion speed is measured

### Practical actions available via save file

| Action | Mechanism | Impact |
|--------|-----------|--------|
| **Activate all Summoning Pools** (flag range TBD; 670xxx unconfirmed) | EventFlags batch set | Impact on invasion **unconfirmed** — primarily a coop/summon mechanism; current editor may use outdated flag IDs |
| Add all 104 matchmaking regions to `unlocked_regions` | `SetUnlockedRegions` | More areas searched per attempt |
| Add Taunter's Tongue to inventory | item inject `0x4000006C` | Enables invasions without co-op phantoms |
| Add Festering Bloody Finger stack | item inject `0x4000006F` | Consumable for active invasions |
| Set Varré questline EventFlags | EventFlags edit | Unlocks Mohgwyn + Festering Bloody Finger without progression |
| `perform_matchmaking = 1` at `UD10[0x0013]` | UD10 byte write | Ensures online mode — not an acceleration parameter |
| NetworkParam patch in UD11 | regulation.bin edit | PS4: values persist; runtime effect **unconfirmed** |

### DS3 Comparison *(approximate reference; DS3 values not backed by local param dump)*

| Aspect | Dark Souls 3 | Elden Ring |
|--------|-------------|------------|
| Invade solo host | Yes (default) | No — requires Taunter's Tongue |
| `breakInRequestIntervalTimeSec` | ~10s | 30s |
| `breakInRequestTimeOutSec` | ~10s | 20s |
| Save file invasion levers | None | `unlocked_regions`; Summoning Pools — impact on invasion unconfirmed |
| regulation.bin protection | EAC online ban risk | EAC online ban risk (PS4 patch persists) |

The bottleneck for invasion frequency is how often and how broadly the client queries the
matchmaking server — controlled by `breakInRequestIntervalTimeSec` and `allAreaSearchRate_*`.
In ER, `unlocked_regions` broadens the area search per query. Summoning Pools may
contribute to broader online eligibility, but current offline data does not confirm this
for Bloody Finger invasions specifically; the current `pvp-ready` dataset is insufficient
to validate Summoning Pool flag effects (see dataset limitation in §§ Gameplay-observed
practical mechanism).

### Unlocked Regions Coverage

Before this research: 77 known region IDs in `backend/db/data/regions.go`.
27 missing regions added (Legacy Dungeon interiors). Total: **104 region IDs**.

The `er-save-manager` community source lists 103 IDs; our database holds 104 after
independently identifying one additional region not in that list.

Risk assessment for region unlocking: likely lower than regulation/runtime edits, because
the game server validates matchmaking eligibility server-side and only uses the local list
as a client-side eligibility hint. Online safety is not guaranteed.

---

## 12. Tooling and Validation

### save_inspector.go

`tmp/scripts/diag/save_inspector.go` — read-only diagnostic tool for Elden Ring PS4/PC save files.
Not tracked by git (`tmp/` is gitignored); run with `go run`.

Capabilities:

- Single slot inspection (default: slot 0, or `-slot N`)
- `-all-slots` — iterate all active slots with per-slot character metadata and section summary
- `-compare <file1> <file2>` — side-by-side PS4 vs PC format comparison
- UD11 format detection (PC: MD5 prefix present; PS4: GER header, no MD5)
- NetworkParam field extraction with vanilla / patched / unknown classification (compares
  against defaults from `core.NetworkParamDefaults()`)
- UD0 `MatchmakingCandidateSection` spec compliance validation (header type, all 13 entry
  types, V7 tail sentinel, terminator)
- UD10 state classifier — maps the 4-field combination to one of:
  PASSIVE / BF-INIT / ACTIVE-BF / SUCCESS / TIMEOUT / PATCHED-IDLE

### batch_analysis.go

`tmp/scripts/diag/batch_analysis.go` — batch processor that runs save_inspector logic
across a directory of save files and emits CSV + Markdown summary.

Output files:
- `tmp/regulation-bin-debug/batch-save-analysis.csv` — per-slot row for each active slot
- `tmp/regulation-bin-debug/batch-save-report.md` — aggregate statistics and distribution tables

Batch validation result (2026-05-09):

| Metric | Value |
|--------|-------|
| Files processed | 25 |
| Active slots | 62 |
| SPEC-VALID | 8 |
| NOT-INITIALIZED | 35 |
| DEVIATES | 19 |
| UNKNOWN classified state | 32 |
| Load errors | 0 |

All 8 SPEC-VALID slots are PS4. All 16 PC active slots are NOT-INITIALIZED or DEVIATES.

### Regression Tests

**9/9 passing.**

Original fixtures (invasion-state saves, all PS4, slot 0):

| Label | State |
|-------|-------|
| H | PASSIVE |
| J | BF-INIT |
| F | ACTIVE-BF |
| I | SUCCESS |
| G | TIMEOUT |
| E | PATCHED-IDLE |

New validation files (no invasion history):

| Label | Format | Active slots |
|-------|--------|-------------|
| `ER0000.sl2` | PC (BND4 unencrypted) | 5 (0–4) |
| `oisis_pl-org.txt` | PS4 raw | 2 (0–1) |
| `oisisk_ps4.txt` | PS4 raw | 4 (0–3) |

All three verify that `MatchmakingCandidateSection` is not interpreted as spec-valid for
characters with no invasion history.

### Cross-Platform Candidate Section Behavior

PS4 uninvaded/inactive slots typically have **zeroed bytes** at `UD0+0x209B00..0x209C43`.

PC BND4 uninvaded/inactive slots can contain **random non-zero bytes** at the same offset.
This is expected BND4 save behavior, not a parser bug — the section is uninitialized and its
content has no semantic meaning.

**Only active slots with valid character metadata should be interpreted.** Slots with
`version=0` and no character name must be treated as empty regardless of candidate section
content.

### Cross-Platform Validation (Batch Result)

| Platform | Active slots | SPEC-VALID MatchmakingCandidateSection |
|----------|-------------|----------------------------------------|
| PC | 16 | 0 |
| PS4 | 46 | 8 |

PC BND4 saves in this dataset do not initialize the same `MatchmakingCandidateSection`
layout observed in PS4 SPEC-VALID slots. This is not currently treated as a parser bug —
PC slots with no invasion history contain random non-zero bytes at `UD0+0x209B00`, which
fail the spec validator. The UD0 candidate queue model is confirmed for **PS4 SPEC-VALID
slots only** in this dataset; PC behavior may differ.

### `-compare` Confirms Offset Compatibility

`-compare` on PS4 vs PC saves confirms:

- `UD0 MatchmakingCandidateSection` relative offset (`+0x209B00`) is fixed and identical
  across both platforms for active slots.
- `UD10` state marker offsets (`+0x5070`, `+0x194E4`, etc.) are platform-independent.
- UD11 blob format differs by platform (PS4: GER+IV+AES+DCX; PC: MD5+IV+AES+DCX), but
  NetworkParam invasion fields decode identically once decrypted.

---

## Sources

- `tmp/regulation-bin-dump/csv/NetworkParam.csv` — vanilla NetworkParam field values
- `tmp/regulation-bin-dump/defs/NetworkParam.xml` — PARAMDEF field types and offsets
- `tmp/regulation-bin-debug/final-report.md` — raw output from bf_candidatelist.go analysis
- `tmp/regulation-bin-debug/batch-save-analysis.csv` — batch analysis of 25 save files (62 active slots)
- `tmp/regulation-bin-debug/batch-save-report.md` — summary report from batch_analysis.go
- `tmp/repos/er-save-manager/src/er_save_manager/data/region_ids_map.py` — 103 matchmaking region IDs
- `tmp/netman_structure.md` — NetMan binary layout documentation
- `backend/core/section_netman.go` — NetMan struct implementation
- `backend/core/structures.go` — `SaveSlot.UnlockedRegions` parsing
- `backend/core/regulation.go` — MD5-recalculating NetworkParam patcher
- `spec/44-network-param-tuning.md` — full NetworkParam field reference

---
