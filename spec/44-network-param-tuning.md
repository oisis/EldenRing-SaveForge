# 44 — NetworkParam PvP Tuning Reference

> **Type**: Design doc
> **Status**: ✅ Implemented
> **Scope**: Complete field reference for NETWORK_PARAM_ST regulation.bin parameters relevant to PvP/multiplayer tuning.

## Overview

`NetworkParam.param` is a single-row PARAM table inside `regulation.bin` (embedded in UserData11).
It controls all client-side matchmaking timings, search limits, and multiplayer behavior.

Row 0 data starts at offset `0x58` within the .param file. All fields are little-endian.

## Architecture

```
UserData11 → AES-256-CBC decrypt → DCX decompress → BND4 unpack → NetworkParam.param → Row 0
```

Platform differences:
- **PC**: DCX uses ZSTD compression
- **PS4**: DCX uses DFLT (zlib) compression

## Field Groups

### Group 1: Summon Signs (NET_SUMMON_SIGN_PARAM)

Controls visibility, refresh rate, and retrieval of summon signs (white/gold/red).

| Field | Type | Offset | Vanilla | Description |
|---|---|---|---|---|
| `signPuddleActiveMessageIntervalSec` | f32 | 0x10 | 30.0 | Interval for "sign puddle active" notification |
| `reloadSignIntervalTime1` | f32 | 0x18 | ~0 | Sign refresh wait (low population mode) — effectively unused |
| `reloadSignIntervalTime2` | f32 | 0x1C | 60.0 | Sign list refresh interval (normal mode). Lower = signs appear faster |
| `reloadSignTotalCount` | u32 | 0x20 | 20 | Max signs retrieved per download (global). Higher = see more signs |
| `reloadSignCellCount` | u32 | 0x24 | 10 | Max signs per map cell. Higher = denser sign visibility in area |
| `updateSignIntervalTime` | f32 | 0x28 | 30.0 | How often YOUR placed sign is updated on server |
| `basicExclusiveRange` | f32 | 0x2C | 2.0 | Min horizontal distance between rendered signs [m] |
| `basicExclusiveHeight` | f32 | 0x30 | 2.2 | Min vertical distance between rendered signs [m] |
| `previewChrWaitingTime` | f32 | 0x34 | 0.5 | Delay before sign preview character model renders |
| `signVisibleRange` | f32 | 0x38 | 30.0 | Max render distance for signs [m] |
| `singGetMax` | u32 | 0x60 | 32 | Hard cap on total retrievable signs |
| `signDownloadSpan` | f32 | 0x64 | 30.0 | Sign list download interval |
| `signUpdateSpan` | f32 | 0x68 | 60.0 | Sign data upload interval to server |

**Tuning notes:**
- `reloadSignIntervalTime2` is the most impactful — reducing from 60→10s means signs appear 6x faster
- `reloadSignTotalCount` and `singGetMax` should be raised together
- Safe to modify: these only affect how aggressively the client polls for sign data

### Group 2: Break-In / Invasions (NET_BREAKIN_PARAM)

Controls invasion matchmaking. **Already implemented in v0.8.0.**

| Field | Type | Offset | Vanilla | Description |
|---|---|---|---|---|
| `maxBreakInTargetListCount` | u32 | 0x70 | 5 | Invasion target candidates per search |
| `breakInRequestIntervalTimeSec` | f32 | 0x74 | 30.0 | Delay between matchmaking retry requests |
| `breakInRequestTimeOutSec` | f32 | 0x78 | 20.0 | Timeout per matchmaking request |
| `breakInRequestAreaCount` (unconfirmed) | u32 (code) / u8 (defs) | 0x7C | 5 | **Semantics UNCONFIRMED.** Labelled `dummy8 pad[4]` ("予約"/Reserved) in the local PARAMDEF and `u8 unknown_0x7c` in the community TGA def — no external source names it `breakInRequestAreaCount`; that name exists only in SaveForge. Vanilla binary value is `5` (verified). SaveForge presets always keep it at `5`; it is exposed only as an Experimental field in the UI. Do not treat it as a confirmed "area search" knob. |

> **Source of truth note**: vanilla values above come from the binary `NetworkParam.param` (Row 0). The exported `csv/NetworkParam.csv` shows `reloadSignTotalCount=32` / `reloadSignCellCount=8` for the sign group, which is **wrong** — the binary (and `NetworkParamDefaults()`) holds `20` / `10`. The binary is authoritative. Note also that invasions have **no** `allAreaSearchRate_*` field (those exist only for CoopBlue/VsBlue/BellGuard), so there is no confirmed local-vs-worldwide knob for break-in search.

### Group 3: Visit / Blue Phantom System (NET_VISIT_PARAM)

Controls Blue Cipher Ring auto-summoning and Hunter system.

| Field | Type | Offset | Vanilla | Description |
|---|---|---|---|---|
| `reloadVisitListCoolTime` | f32 | 0x180 | 20.0 | Cooldown between blue phantom searches. Lower = faster blue matching |
| `maxCoopBlueSummonCount` | u32 | 0x184 | 2 | Max blue phantoms the ring system searches for simultaneously. Server enforces actual session cap — this only affects client-side search parallelism |
| `maxBellGuardSummonCount` | u32 | 0x188 | 4 | Max Bell Guard (area defender) summon candidates |
| `maxVisitListCount` | u32 | 0x18C | 5 | Number of visit targets retrieved per search |
| `reloadSearch_CoopBlue_Min` | f32 | 0x190 | 30.0 | Min delay between co-op blue reload searches |
| `reloadSearch_CoopBlue_Max` | f32 | 0x194 | 180.0 | Max delay (randomized between min/max) |
| `reloadSearch_BellGuard_Min` | f32 | 0x198 | 120.0 | Min delay between Bell Guard reload |
| `reloadSearch_BellGuard_Max` | f32 | 0x19C | 240.0 | Max delay for Bell Guard reload |
| `reloadSearch_RatKing_Min` | f32 | 0x1A0 | 180.0 | Min delay for Rat King covenant reload |
| `reloadSearch_RatKing_Max` | f32 | 0x1A4 | 300.0 | Max delay for Rat King covenant reload |

**Tuning notes:**
- `maxCoopBlueSummonCount` = 4 is safe. The server caps actual joins to available session slots. More candidates = faster first-match probability.
- `reloadSearch_CoopBlue_Min/Max` has the biggest impact on "how long until a blue arrives" — reducing from 30-180s to 5-20s is dramatic.
- `allAreaSearchRate_*` (in Extra section) stacks with this — 100% means search globally, not just nearby.

### Group 4: Extra / Miscellaneous (NET_EXTRA_PARAM)

| Field | Type | Offset | Vanilla | Description |
|---|---|---|---|---|
| `srttMaxLimit` | f32 | 0x1B0 | 10000.0 | SRTT (Smoothed RTT) upper limit [ms]. Connection quality gate |
| `srttMeanLimit` | f32 | 0x1B4 | 10000.0 | Mean SRTT limit [ms]. Affects matchmaking quality filter |
| `srttMeanDeviationLimit` | f32 | 0x1B8 | 10000.0 | RTT deviation limit [ms]. Connection stability gate |
| `darkPhantomLimitBoostTime` | f32 | 0x1BC | 60.0 | ⚠️ **LEGACY/UNVERIFIED** — DS3 mechanic: after N seconds invader timer accelerates. In ER invaders have no observable time limit — this field likely has no effect |
| `darkPhantomLimitBoostScale` | f32 | 0x1C0 | 1.2 | ⚠️ **LEGACY/UNVERIFIED** — DS3 mechanic: timer acceleration multiplier. Likely inactive in ER |
| `multiplayDisableLifeTime` | f32 | 0x1C4 | 1800.0 | How long multiplayer stays disabled after certain events [s] |
| `abyssMultiplayLimit` | u8 | 0x1C8 | 8 | Max times abyss spirit can enter host's world |
| `phantomWarpMinimumTime` | u8 | 0x1C9 | 6 | Min time before phantom can warp [s] |
| `phantomReturnDelayTime` | u8 | 0x1CA | 2 | Delay after Black Crystal before return [s] |
| `terminateTimeoutTime` | u8 | 0x1CB | 30 | Disconnect detection timeout [s] |
| `penaltyPointLanDisconnect` | u16 | 0x1CC | 10 | Penalty points for LAN disconnect |
| `penaltyPointSignout` | u16 | 0x1CE | 0 | Penalty points for signout (vanilla=0, no penalty) |
| `penaltyPointReboot` | u16 | 0x1D0 | 10 | Penalty points for hard reboot/power off |
| `penaltyPointBeginPenalize` | u16 | 0x1D2 | 100 | Threshold: penalties activate when points >= this |
| `penaltyForgiveItemLimitTime` | f32 | 0x1D4 | 36000.0 | Way of White Circlet cooldown [s]. 36000 = 10 hours |
| `allAreaSearchRate_CoopBlue` | u8 | 0x1D8 | 30 | % chance to search ALL areas for co-op blue (vs local only) |
| `allAreaSearchRate_VsBlue` | u8 | 0x1D9 | 30 | % chance for retribution blue global search |
| `allAreaSearchRate_BellGuard` | u8 | 0x1DA | 30 | % chance for Bell Guard global search |
| `bloodMessageEvalHealRate` | u8 | 0x1DB | 20 | HP heal % when your message gets rated |
| `signDisplayMax` | u8 | 0x1E4 | 10 | Max signs rendered simultaneously |
| `bloodStainDisplayMax` | u8 | 0x1E5 | 7 | Max bloodstains rendered |
| `bloodMessageDisplayMax` | u8 | 0x1E6 | 3 | Max blood messages rendered |

**Tuning notes:**
- `darkPhantomLimitBoostTime/Scale`: **Likely legacy from Dark Souls 3.** ER invaders have no observable session timer — they can camp indefinitely until host dies, rests, or enters boss fog. These fields exist in the struct but probably aren't read by ER game logic. **Do not include in presets.**
- `allAreaSearchRate_*` at 100% = always search globally. Dramatically speeds up blue phantom arrival but increases server load.
- `penaltyPoint*` fields: setting to 0 removes disconnect penalties client-side. **HIGH BAN RISK** — server may validate these values.
- `penaltyForgiveItemLimitTime` = 0: instant Way of White Circlet availability. Moderate ban risk.

### Group 5: Quick Match / Colosseum (QUICK_MATCH)

| Field | Type | Offset | Vanilla | Description |
|---|---|---|---|---|
| `summonMessageInterval` | f32 | 0x1F8 | 10.0 | "Searching for match..." message interval [s] |
| `hostRegisterUpdateTime` | f32 | 0x1FC | 60.0 | Host periodic status update to server |
| `hostTimeOutTime` | f32 | 0x200 | 30.0 | How long host waits for guest to join before timeout |
| `guestUpdateTime` | f32 | 0x204 | 30.0 | Guest authentication wait timeout |
| `guestPlayerNoTimeOutTime` | f32 | 0x208 | 55.0 | Guest PlayNo sync timeout |
| `hostPlayerNoTimeOutTime` | f32 | 0x20C | 45.0 | Host PlayNo sync timeout |
| `requestSearchQuickMatchLimit` | u32 | 0x210 | 5 | Max results per quick match search |

### Group 6: Visitor System (VISITOR)

Controls the Taunter's Tongue / summoning pool visitor mechanics.

| Field | Type | Offset | Vanilla | Description |
|---|---|---|---|---|
| `VisitorListMax` | u32 | 0x240 | 10 | Max visitor target list entries |
| `VisitorTimeOutTime` | f32 | 0x244 | 60.0 | Visitor wait timeout [s] |
| `DownloadSpan` | f32 | 0x248 | 60.0 | Visitor list download interval [s] |
| `VisitorGuestRequestMessageIntervalSec` | f32 | 0x24C | 30.0 | "Searching for visit target..." message interval |

## Ban Risk Assessment

| Risk | Parameters | Rationale |
|---|---|---|
| **Low** | Group 1 (signs), Group 5 (quick match timings) | Client-side polling rates only; server sees normal traffic patterns |
| **Moderate** | Group 3 (`maxCoopBlueSummonCount`, `allAreaSearchRate`), Group 6 (visitor timings) | Changes matchmaking behavior visibly but doesn't break protocol |
| **High** | Group 4 (`penaltyPoint*`, `penaltyForgiveItemLimitTime`) | Server likely validates penalty state; mismatch = flag |

## Functional Presets (implemented — v0.9 unified system)

Three role-scoped presets, defined once in `backend/core/regulation.go` and fetched
by the frontend via `GetNetworkPreset` (single source of truth — frontend and backend
cannot drift). The previous global `Aggressive` profile (15 / 6 / 3 / 15 for invasion)
was **removed** — it cut `breakInRequestTimeOutSec` to 3s (broke near-and-far matchmaking),
ran a near-continuous retry loop, and wrote the unconfirmed 0x7C field. None of the new
presets touch 0x7C (it stays at vanilla 5).

### "Faster Reds" — `NetworkParamFasterReds()` (Invader)
```
maxBreakInTargetListCount:      5 → 8
breakInRequestIntervalTimeSec: 30 → 12   (≥8s avoids the constant search-message flicker)
breakInRequestTimeOutSec:      20 → 15   (kept generous so near-and-far can complete)
breakInRequestAreaCount (0x7C): 5 → 5    (unchanged — unconfirmed semantics)
```

### "Faster Summons & Pools" — `NetworkParamFasterSummons()` (Cooperator)
```
reloadSignIntervalTime2:  60 → 20
reloadSignTotalCount:     20 → 40
reloadSignCellCount:      10 → 20
updateSignIntervalTime:   30 → 15
singGetMax:               32 → 64
signDownloadSpan:         30 → 15
signUpdateSpan:           60 → 20
```
Invariant enforced (backend + UI clamp): `reloadSignCellCount ≤ reloadSignTotalCount ≤ singGetMax`.
`cellGroupHorizontalRange` (spatial reach) is **not** part of this preset — it is a planned
Experimental option (backend patching not yet implemented).

### "Faster Blue / Hunter" — `NetworkParamFasterBlue()` (Blue)
```
reloadVisitListCoolTime:     20 → 8
reloadSearch_CoopBlue_Min:   30 → 10
reloadSearch_CoopBlue_Max:  180 → 40
maxVisitListCount:            5 → 10
allAreaSearchRate_CoopBlue:  30 → 60
maxCoopBlueSummonCount:       2 → 2     (unchanged — server caps actual joins)
allAreaSearchRate_VsBlue:    30 → 30    (unchanged — retribution likely legacy in ER)
```
Invariant enforced: `reloadSearch_CoopBlue_Min ≤ reloadSearch_CoopBlue_Max`.

### Not implemented (research-only)
- **Taunter's Tongue / host-side reds** — no confirmed rate parameter found (Goods 108/178 → SpEffect 533 sets session state only). The Visitor fields (`VisitorListMax/TimeOutTime/DownloadSpan`) belong to the ring-search visitor system and are exposed only as Experimental "legacy ring-search fields".
- **Colosseum / arena** — no dedicated matchmaking table; `AvatarMatchSearchMax` / `BattleRoyalMatchSearch*` are marked 未使用 (unused); `requestSearchQuickMatchLimit` usage in ER is unconfirmed.
- **`NetworkAreaParam.cellSize*`** — high risk (changing the local cell grid can desync matchmaking buckets vs the server); out of scope.
- **`penaltyPoint*` / `penaltyForgiveItemLimitTime`** — high ban risk (server likely validates penalty state); intentionally not offered as a preset.

## Sources

- `tmp/regulation-bin-dump/defs/NetworkParam.xml` — PARAMDEF schema (field types, names, ranges)
- `tmp/regulation-bin-dump/csv/NetworkParam.csv` — actual vanilla values (Row ID 0)
- `backend/core/regulation.go` — implementation of read/patch pipeline
- Japanese field names translated via FromSoftware param naming conventions
