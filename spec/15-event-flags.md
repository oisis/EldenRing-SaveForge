# 15 — Event Flags

> **Type**: Binary format spec + helper API reference
> **Status**: ✅ canonical (Phase 4 Step 1 — 2026-05-21)
> **Scope**: Foundation of event-flag storage + addressing + helper API. Feature-specific IDs (map, graces, bosses, item companions, PvP) live in derived chapters — see §18 Cross-references.

---

## 1. Chapter purpose

Event Flags are the main game-progression mechanism — a fixed-size bitfield of `0x1BF99F` bytes (1,833,375 B, ~1.75 MB) in every slot. Each flag is a single bit controlling one aspect of game state (boss defeated, NPC quest stage, item picked up, map revealed, cutscene watched, mechanic unlocked, etc.).

This chapter describes **only the foundation**:

- where flags are stored (`EventFlagsBlock`, slot offset)
- how the code addresses a single flag by its ID (3-tier resolver)
- byte/bit indexing (big-endian bit order)
- helper API (`db.GetEventFlag`, `db.SetEventFlag`)
- set/get semantics (idempotent vs SET-only — depending on the feature)
- where to find domain IDs (cross-refs)

This chapter does **not** contain:

- tables of map / grace / boss / item IDs → see the respective chapters
- feature-specific semantics (companion flags items vs graces) → see 50 / 47
- map reveal / PvP module details → see 27 / 48

---

## 2. Status

- ✅ Storage layout (`0x1BF99F` + 1B terminator) — hex-verified PS4/PC.
- ✅ Tier-3 resolver (precomputed → BST → fallback) — covered by the `TestBSTLookupMatchesEventFlags` test.
- ✅ Helper API (`GetEventFlag`/`SetEventFlag`) — round-trip stable, used by 20+ callsites in `app_world.go`.
- ✅ BST embed (`eventflag_bst.txt`, 11,919 entries — snapshot 2026-05-21) — generated offline from a script, embedded via `//go:embed`.
- ✅ Precomputed lookup (`event_flags.go`, 865 entries `EventFlagInfo{Byte, Bit}` — snapshot 2026-05-21).
- ⚠️ Boss multi-flag editing — **not implemented** (`SetEventFlag` sets a single bit; multi-flag is planned in `38-boss-multiflag.md`).
- ⚠️ PvP modular preset module E (Sites of Grace) — **not active** (warning "planned but not enabled" in `app_pvp.go:109`).
- ⚠️ Summoning pools module D — flags are set, but the game runtime does not honor them (see `42-summoning-pools-bug.md`).

**Last verification**: 2026-05-21 on `tmp/save/ER0000.sl2` (PC) + `tmp/save/oisis_pl-org.txt` (PS4) via round-trip tests.

---

## 3. Source of truth in code

| Component | File / lines | Note |
|---|---|---|
| Helper read | `backend/db/db.go::GetEventFlag` | Resolver + bit test |
| Helper write | `backend/db/db.go::SetEventFlag` | Resolver + bit set/clear |
| Position resolver | `backend/db/db.go::resolveEventFlagPosition` | 3-tier (precomputed → BST → fallback) |
| Precomputed table | `backend/db/data/event_flags.go` | `var EventFlags map[uint32]EventFlagInfo` — 865 entries |
| BST table | `backend/db/data/bst.go::EventFlagBST` + `eventflag_bst.txt` | 11,919 entries, `BSTBlockSize=125`, `BSTFlagDivisor=1000` |
| Container struct | `backend/core/section_eventflags.go::EventFlagsBlock` | `EventFlagsByteCount = 0x1BF99F`, `EventFlagsBlockSize = 0x1BF9A0` |
| Slot integration | `backend/core/structures.go::Slot.EventFlagsOffset` | `int` offset into `slot.Data`; may be 0 if the chain is unreachable |
| Tests (round-trip) | `backend/core/section_eventflags_test.go` | `TestEventFlagsBlockPS4`, `TestEventFlagsBlockPC` |
| Tests (lookup) | `tests/map_flags_test.go::TestBSTLookupMatchesEventFlags` | Verifies that BST-computed positions = precomputed positions |
| Tests (offset) | `tests/map_flags_test.go::TestEventFlagsOffsetCorrectness` | Verifies that `slot.EventFlagsOffset` points at the bitfield |
| Tests (write semantics) | `tests/save_modify_test.go` | `TestEventFlagsBossToggle`, `TestEventFlagsBossKillAll`, `TestEventFlagsGraceToggle`, `TestEventFlagsGraceUnlockAll` |

---

## 4. Mental model

```
slot.Data
  ├── ... earlier sections ...
  ├── PreEventFlagsScalars (29 B)            — GameMan bytes, deaths, LastRestedGrace, etc.
  ├── EventFlagsBlock                        — slot.EventFlagsOffset
  │     ├── Flags []byte (0x1BF99F = 1 833 375 B)
  │     └── 1-byte terminator
  └── ... subsequent sections ...
```

A flag is addressed by its **uint32 ID** (e.g., `71190` = Roundtable Hold grace). The resolver maps ID → `(byteIdx, bitIdx)`:

1. **Precomputed** — `EventFlags[id]` → exact `(Byte, Bit)`. Fastest. 865 entries for the most frequently used flags.
2. **BST lookup** — block = `id / 1000`. If `EventFlagBST[block]` exists → `bstPos*125 + (id%1000)/8` + `7 - (id%1000)%8`. Covers most of the remaining flags (the game uses `CSFD4VirtualMemoryFlag` block layout).
3. **Fallback formula** — `byte = id/8, bit = 7 - (id%8)`. Only for small IDs (< 1000) or as a last resort.

The `TestBSTLookupMatchesEventFlags` test verifies that for every ID in the precomputed table the BST formula yields the same result (i.e., both tier-1 and tier-2 are consistent — tier-1 is only a cache).

---

## 5. Event flag storage model

### 5.1 Container

```go
const EventFlagsByteCount = 0x1BF99F       // 1 833 375 bytes (~1.75 MB)
const EventFlagsBlockSize = EventFlagsByteCount + 1  // + 1-byte terminator

type EventFlagsBlock struct {
    Flags      []byte // length = EventFlagsByteCount
    // 1-byte terminator written/read after Flags
}
```

- **Size**: fixed, does not change when editing.
- **Terminator**: 1 byte after the bitfield (original value preserved through round-trip).
- **Endian**: the bitfield is a flat byte array; bit order **within a byte** is big-endian (bit 7 = the highest bit of ID-modulo-8).

### 5.2 Slot integration

```go
type Slot struct {
    ...
    EventFlagsOffset int  // offset into slot.Data; may be 0 if the chain is unreachable
}
```

- The offset is computed dynamically during parsing (`structures.go:423`):

  ```go
  s.EventFlagsOffset = s.IngameTimerOffset + DynEventFlags
  ```

- If the offset ≥ `SlotSize` → clamped to 0 + warning (`event flags disabled`).
- All callsites in `app_world.go` use `slot.Data[slot.EventFlagsOffset:]` as the `flags []byte` argument.

### 5.3 Round-trip invariant

Test `TestEventFlagsBlockPS4` / `PC`:

1. Read `EventFlagsBlock` from the save.
2. Marshal to bytes.
3. Compare `expectedSize = PreEventFlagsScalarsSize + EventFlagsBlockSize` (29 + 0x1BF9A0).
4. Byte-equal to the original.

---

## 6. Flag ID addressing — 3-tier resolver

```go
func resolveEventFlagPosition(id uint32) (byteIdx uint32, bitIdx uint8) {
    // Tier 1: Precomputed lookup
    if info, ok := data.EventFlags[id]; ok {
        return info.Byte, info.Bit
    }
    // Tier 2: BST lookup
    data.LoadBST()
    block := id / data.BSTFlagDivisor          // = 1000
    if bstPos, ok := data.EventFlagBST[block]; ok {
        idx := id % data.BSTFlagDivisor
        return bstPos*data.BSTBlockSize + idx/8, uint8(7 - (idx % 8))
    }
    // Tier 3: Fallback formula
    return id / 8, uint8(7 - (id % 8))
}
```

| Tier | Coverage | Source | Usage |
|---|---|---|---|
| 1 Precomputed | 865 most frequently used IDs | `event_flags.go` — generated offline | Hot path (boss/grace/map/companion) |
| 2 BST | ~11,919 blocks (each 1000 flags) | `eventflag_bst.txt` (embed) | Most of the remaining IDs |
| 3 Fallback | ID < 1000 or unknown block | Inline formula | Rare, edge cases |

**Constant `BSTBlockSize = 125`** = 1000 flags / 8 bits = 125 bytes per BST block.
**Constant `BSTFlagDivisor = 1000`** = number of flags per BST block.

`LoadBST()` is idempotent (`sync.Once`) — the first call parses the embedded TXT (`block,offset` format) into a map, subsequent calls are no-ops.

---

## 7. Byte and bit indexing

### 7.1 Convention

Bits within a byte are addressed **big-endian**:

```
byte at byteIdx:    [bit7][bit6][bit5][bit4][bit3][bit2][bit1][bit0]
bit ID modulo 8:      0     1     2     3     4     5     6     7
```

For the BST resolver:

```
idx     = id % 1000               // flag position within the block
byteIdx = bstPos*125 + idx/8      // byte within the block
bitIdx  = 7 - (idx % 8)           // bit within the byte (big-endian)
```

For the fallback (ID < 1000):

```
byteIdx = id / 8
bitIdx  = 7 - (id % 8)
```

### 7.2 Example

Flag `71190` (Roundtable Hold grace):

- Tier 1 (precomputed): `event_flags.go` returns `{Byte: 0xA58, Bit: 1}` → flag in byte 0xA58, bit 1.
- Tier 2 (BST): block = 71, idx = 190, `bstPos = EventFlagBST[71]`. For `bstPos*125 + 190/8 = bstPos*125 + 23`, bit = `7 - (190 % 8) = 7 - 6 = 1`. Must match Tier 1 (the `TestBSTLookupMatchesEventFlags` test verifies this).

### 7.3 Read

```go
flag_value = (flags[byteIdx] & (1 << bitIdx)) != 0
```

### 7.4 Write

```go
if value {
    flags[byteIdx] |= (1 << bitIdx)
} else {
    flags[byteIdx] &= ^(1 << bitIdx)
}
```

---

## 8. Helper API

### 8.1 Signature

```go
// backend/db/db.go
func GetEventFlag(flags []byte, id uint32) (bool, error)
func SetEventFlag(flags []byte, id uint32, value bool) error
```

- **`flags []byte`** — the bitfield slice, usually `slot.Data[slot.EventFlagsOffset:]`.
- **`id uint32`** — flag ID (e.g., 71190).
- **Returns error** — when the resolver points at a byte offset >= len(flags) (out of bounds).

### 8.2 Bounds check

Both helpers validate `int(byteIdx) >= len(flags)`:

```go
if int(byteIdx) >= len(flags) {
    return ..., fmt.Errorf("event flag %d (byte %d) out of bounds (flags len %d)", id, byteIdx, len(flags))
}
```

Consequence: when a slot has `EventFlagsOffset == 0` (chain unreachable, warning from `structures.go:469`), `flags []byte` may have length 0 → all calls will return an error. The caller (e.g., `GetGraces`, `SetBossDefeated`) must handle this condition.

### 8.3 No rollback

`SetEventFlag` is a **stateless byte mutation** — it modifies a single bit in the slice. There is no snapshot/rollback. Rollback (if needed) must be on the caller's side (e.g., `ApplyPvPPreparation` in `app_pvp.go` has snapshot-based undo at the save level).

---

## 9. BST / flag block model

### 9.1 Origin

The game uses `CSFD4VirtualMemoryFlag` (Cheat Engine / RE) as a block-based layout. Block size = 1000 flags = 125 bytes. Blocks are not contiguous — the order of blocks in memory/save does not match the order of IDs. The `block → bstPos` mapping is extracted once and embedded in `eventflag_bst.txt`.

### 9.2 File format

```
block,offset
0,0
1,1
2,2
...
```

- `block` = `id / 1000` (e.g., 71 for flags in the 71xxx group).
- `offset` = the position of the block in the bitfield (multiplied by `BSTBlockSize=125` yields the byte offset).

11,919 entries = ~99% coverage of possible blocks (a few blocks are empty / unused, absent from the file).

### 9.3 Loader

```go
//go:embed eventflag_bst.txt
var eventflagBSTRaw string

var EventFlagBST map[uint32]uint32

func LoadBST() {
    bstOnce.Do(func() { /* parse eventflagBSTRaw */ })
}
```

- Embed via `//go:embed` (Go 1.16+).
- `sync.Once` → thread-safe lazy init.
- The map comment says `12000` (pre-allocate hint), but the actual number of entries = 11,919.

### 9.4 Update

`eventflag_bst.txt` is generated offline (the script is out of `make` scope — `tmp/scripts/eventflag_bst_build.go` by intent). After updating `regulation.bin` from a new game patch, re-extraction may be required.

---

## 10. Generic set/get semantics

The helper API is **idempotent at the bit level**:

- `SetEventFlag(flags, id, true)` × N times = a one-time set. The bit stays `1`.
- `SetEventFlag(flags, id, false)` × N times = the bit stays `0`.
- `SetEventFlag(flags, id, true)` after `SetEventFlag(flags, id, false)` = `1` (clean toggle).

**No side effects** — `SetEventFlag` does not call other functions, does not set related flags, does not affect other sections of the save. Each bit is independent.

Side effects (companion flags, door flags, region flags, etc.) are realized by **callers** — usually in `app_world.go` (e.g., `SetGraceVisited` sets the grace flag + door flag + companion flags in a single call).

---

## 11. Feature-specific policy differences

⚠️ **Very important**: the helper API is generic (set/clear/toggle), but specific features may have a **different policy** for applying set/clear. The most important differences:

| Feature | Policy | Chapter |
|---|---|---|
| Item companion flags (e.g., Spectral Steed Whistle → 60100, 4680, 710520, 4681) | **SET + CLEAR symmetric** — SET on add, CLEAR on remove | [50](50-item-companion-flags.md) |
| Grace companion flags (Gatefront 76111 → the same 4 IDs) | **SET-only asymmetric** — SET on visit, NEVER CLEAR | [47](47-site-of-grace-activation.md) |
| Site of Grace `visited` flag (71xxx–76xxx) | Symmetric — the caller toggles freely | [47](47-site-of-grace-activation.md) |
| DoorFlag (auto-open on SetGraceVisited) | Set together with grace; CLEAR together with grace | [47](47-site-of-grace-activation.md) |
| Boss defeat flag (9xxx) | Symmetric single bit; ⚠️ a full defeat would require multi-flag (38-boss-multiflag.md) | [14](14-game-state.md), [38](38-boss-multiflag.md) |
| Map visible flag (62xxx) | Symmetric; revealBaseMap SETs the whole group | [27](27-map-reveal.md) |
| Map fragment item-pickup flag (63xxx, 66xxx) | SET-only in the current code (no CLEAR-on-remove for container pickup flags) — `needs verification` whether this absence is intentional | [27](27-map-reveal.md), [50](50-item-companion-flags.md) |
| Colosseum unlock (60xxx + global 6080/60100/69480) | Symmetric, but the global flags are shared between colosseums | [48](48-pvp-ready-modular-presets.md) |
| Summoning pool activation (670xxx) | Symmetric in the save, but the **runtime does not honor it** | [48](48-pvp-ready-modular-presets.md), [42](42-summoning-pools-bug.md) |
| Ash of War menu unlock (`AoWMenuUnlockedFlag` = 65800) | Symmetric, auto-managed on whetblade unlocks | (see `app_world.go::SetWhetbladeUnlocked`) |

**Rule**: chapter `15` does not adjudicate individual policies. Each domain chapter (47/50/48/27/14) is the source of truth for its own policy.

---

## 12. Selected callers (known usages)

Selected callsites of `db.SetEventFlag` / `db.GetEventFlag` in the current code (`app_world.go`, 2026-05-21). The table is **not a complete list** — see §12.2 for the remaining files.

### 12.1 `app_world.go` (40 calls — 11× `GetEventFlag`, 29× `SetEventFlag`)

| Caller | Role | Typical ID range | Cross-ref |
|---|---|---|---|
| `GetGraces`/`SetGraceVisited` | Read+write grace visited; auto door flag + companion flags | 71000–76960 | [47](47-site-of-grace-activation.md) |
| `GetBosses`/`SetBossDefeated` | Read+write boss defeat (single flag) | 9xxx (+DLC) | [14](14-game-state.md), [38](38-boss-multiflag.md) |
| `GetSummoningPools`/`SetSummoningPoolActivated` | Read+write pool activation | 670xxx | [48](48-pvp-ready-modular-presets.md), [42](42-summoning-pools-bug.md) |
| `GetColosseums`/`SetColosseumUnlocked` | Read+write colosseum + global flags | 60xxx + 6080/60100/69480 | [48](48-pvp-ready-modular-presets.md) |
| `GetCookbooks`/`SetCookbookUnlocked` | Read+write recipe unlock | 67xxx, 68xxx | — |
| `revealBaseMap`/`revealDLCMap`/`ResetMapExploration` | Set map visible / clear; add map fragment items | 62xxx, 63xxx, 82xxx | [27](27-map-reveal.md) |
| `SetMapFlag`/`SetMapRegionFlags` | Set a single or region map flag | 62xxx, 63xxx | [27](27-map-reveal.md) |
| `SetWhetbladeUnlocked` | Auto-manage `AoWMenuUnlockedFlag` (65800), Storm Stomp dup (65841) | 65610–65720, 65800, 65841 | (`app_world.go`) |
| `SetBellBearingUnlocked` | Bell bearing trader unlocks | mixed | — |
| `SetGestureUnlocked` | Gesture unlocks | 60800–60849 | — |
| `SetRegionUnlocked` (`app_world.go`) | Region flag toggle (NOT to be confused with `core.SetUnlockedRegions`) | 6100000–6999999 | [11](11-regions.md) |
| `SetQuestStep` | Bulk quest step skip (Tier 1 in `RISK_INFO`) | mixed | [32](32-ban-risk-system.md) |

### 12.2 Other files with `db.GetEventFlag`/`SetEventFlag`

| File | Purpose | Short note |
|---|---|---|
| `app_pvp.go::ApplyPvPPreparation` | Composite — calls per module (colosseums, summoning pools) | [48](48-pvp-ready-modular-presets.md) |
| `app_appearance.go` | Companion flag toggles for appearance/character imports | (see `app_appearance.go:201–229`) |
| `app.go` (incl. ClearCount sync) | E.g., `db.SetEventFlag(flags, 50+i, i == slot.Player.ClearCount)` (line 208) — flags 50–58 NG+ tracking | [14](14-game-state.md) |
| `backend/vm/preset.go` | Character preset round-trip | (see `preset.go`) |

A complete enumeration of callers is out of scope for `15` — that is a domain catalog in the derived chapters.

Every caller uses `slot.Data[slot.EventFlagsOffset:]` as the `flags []byte` argument.

---

## 13. Hardcoded flag IDs

Hardcoded IDs in the code (as `const` constants or in `data.*` maps), not only in the static database:

| Constant / location | ID | Purpose |
|---|---|---|
| `data.AoWMenuUnlockedFlag` | `65800` | Enable the "Ashes of War" menu in-game |
| `data.StormStompDupFlag` | `65841` | AoW duplication for Storm Stomp |
| `data.GatefrontGraceEventFlagID` | `0x1294F` (= 76111) | Gatefront grace — trigger companion flags |
| `data.ItemSpectralSteedWhistle` etc. (constants in `backend/db/data/`) | `0x40000082` etc. | Item IDs used as keys in `itemCompanionEventFlags` |
| `data.EventFlagObtainedSpectralSteedWhistle` etc. (constants) | (actual values in `backend/db/data/` companion flag constants) | Companion flag IDs for item/grace |

The remaining IDs are in static databases:

- `backend/db/data/graces.go` → 419 entries `GraceData{Name, Region, DoorFlag, IsBossArena, DungeonType}`
- `backend/db/data/bosses.go` → 110 entries `BossData{Name, Region, Type, Remembrance}`
- `backend/db/data/summoning_pools.go` → 222+ entries
- `backend/db/data/colosseums.go` → 3 entries + companion flags
- `backend/db/data/container_pickup_flags.go` → a flag map for each container key item

**There is no global "all known flag IDs" table** — IDs live in dedicated domain tables, and the `EventFlags` map in `event_flags.go` is only a **byte/bit lookup cache**, not an index of meanings.

---

## 14. Validation and rollback caveats

### 14.1 Bounds check

`GetEventFlag` / `SetEventFlag` return an error when `byteIdx >= len(flags)`:

```
event flag 71190 (byte 2648) out of bounds (flags len 0)
```

Callers must handle this condition — in `app_world.go` they typically return the error to the UI with a `wrap` (`return fmt.Errorf("set boss defeated: %w", err)`).

### 14.2 No transactional API

`SetEventFlag` is a **stateless bit mutation**. There is no snapshot/rollback. Consequences:

- Multi-flag operations (e.g., `SetGraceVisited` with 4 companion flags) are **not atomic** — if flag 3 of 4 fails (bounds error), the other 2 are already set. Fixing it on the caller's side requires manual cleanup.
- Bulk operations (e.g., `ApplyPvPPreparation` with N modules) usually use a snapshot at the save level (before Apply), not at the individual-flag level.

### 14.3 Slot offset 0

If `slot.EventFlagsOffset == 0` (chain unreachable, warning from the parser), `slot.Data[0:]` starts at the beginning of the slot → all `SetEventFlag` calls will operate on the **wrong byte** or return a bounds error. The UI must check the parser warning before invoking flag mutations.

### 14.4 Semantic validation

The tier-3 resolver (fallback) **does not validate** whether the ID is known. Any `uint32` is accepted — for IDs outside precomputed/BST the resolver will use the fallback formula (`byte = id/8, bit = 7-(id%8)`), which **does not match** the game's BST layout. Result: setting an unknown ID may hit a random bit and corrupt other state.

⚠️ **Rule**: do not call `SetEventFlag` for an ID you do not know from precomputed or BST. The `TestBSTLookupMatchesEventFlags` test covers precomputed × BST, but does not reject IDs outside those ranges.

---

## 15. Safety notes

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| S1 | Setting a boss-defeat flag without the related flags (arena, quest, drop, world) = inconsistent state (Tier 1 ban-risk) | 🔴 high | Multi-flag planned in [38](38-boss-multiflag.md); current single-flag = best effort. UI warns via `RISK_INFO` "Kill All Bosses". |
| S2 | CLEAR a grace companion flag (e.g., manually clearing 60100) while the player owns the Whistle via quest = quest regression | 🔴 high | Grace companion flags are SET-only by design (see [47](47-site-of-grace-activation.md)). Never call `SetEventFlag(60100, false)` manually. |
| S3 | Setting an unknown ID (outside precomputed + BST) → the fallback formula hits a random bit | 🔴 high | Do not call `SetEventFlag` for a raw uint32 from the UI; always go through the domain API (`SetGraceVisited`, etc.). |
| S4 | Bulk unlock (e.g., `Unlock All Graces`) without snapshot-undo → the user may not be able to revert | ⚠️ medium | The UI has Tier 1 confirmation (see [32](32-ban-risk-system.md)) + a save-level snapshot before Apply. |
| S5 | Map visible flag set without the fragment item → the map is visible but the game may try to re-trigger the pickup cutscene | ⚠️ medium | `revealBaseMap` does BOTH (flag + item) atomically — use it, not a raw flag. See [27](27-map-reveal.md). |
| S6 | Summoning pool flag set, but the game runtime does not honor it → the user thinks the pool is active, but invasions do not work | ⚠️ low | `app_pvp.go` returns the warning "Bloody Finger invasion impact is unconfirmed". See [42](42-summoning-pools-bug.md). |
| S7 | Slot offset 0 (chain unreachable) → SetEventFlag on byte 0 of the slot = corrupted other section | 🔴 high | The parser emits the warning `EventFlagsOffset 0x... >= SlotSize` → the caller must detect it. The `TestEventFlagsOffsetCorrectness` test verifies the offset. |

---

## 16. Test coverage

| Test | Purpose | File |
|---|---|---|
| `TestEventFlagsBlockPS4` | PS4 round-trip of `EventFlagsBlock` (read → marshal → byte-equal) | `backend/core/section_eventflags_test.go:100` |
| `TestEventFlagsBlockPC` | PC round-trip as above | `backend/core/section_eventflags_test.go:104` |
| `TestBSTLookupMatchesEventFlags` | BST formula matches precomputed positions for all IDs in the precomputed table | `tests/map_flags_test.go:12` |
| `TestGetAllMapEntries` | Map flag DB coverage (non-zero entries, no duplicates) | `tests/map_flags_test.go:53` |
| `TestMapFlagsRoundtrip` | Map flag toggle → save → reload → state preserved | `tests/map_flags_test.go:96` |
| `TestEventFlagsOffsetCorrectness` | `slot.EventFlagsOffset` points at the actual bitfield | `tests/map_flags_test.go:145` |
| `TestEventFlagsBossToggle` | Single boss toggle round-trip | `tests/save_modify_test.go:190` |
| `TestEventFlagsBossKillAll` | Bulk all bosses set → save → reload → all defeated | `tests/save_modify_test.go:277` |
| `TestEventFlagsGraceToggle` | Single grace toggle round-trip | `tests/save_modify_test.go:327` |
| `TestEventFlagsGraceUnlockAll` | Bulk all graces set → save → reload → all visited | `tests/save_modify_test.go:399` |

10 Phase 4-foundation + map tests. They cover round-trip, lookup consistency, offset correctness, bulk + single ops, map flag DB coverage.

No dedicated test for:

- **`SetEventFlag` with an out-of-bounds ID** (negative case for the bounds check).
- **Multi-flag atomic rollback** (callers have no snapshot — a design choice).
- **PS4↔PC cross-platform bit-equal diff** for the same logical state (round-trip is per-platform; there is no `TestConvert*` in `backend/core/` or `tests/`).
- **Companion flag policy enforcement in the 15 scope** — covered by `grace_companion_flags_test.go` and `item_companion_flags_test.go` (in the 47/50 domain).

---

## 17. Known limits / needs verification

| # | Limit / gap | Status | Note |
|---|---|---|---|
| L1 | Boss multi-flag editing | ❌ not implemented | See [38](38-boss-multiflag.md). Current = single flag 9xxx. |
| L2 | PvP modular preset module E (Sites of Grace) | ❌ placeholder | `app_pvp.go:109` warning "planned but not enabled". See [48](48-pvp-ready-modular-presets.md). |
| L3 | Summoning pool runtime honor | ⚠️ flag set, but the game does not honor it | See [42](42-summoning-pools-bug.md). |
| L4 | Validation of an unknown ID in `SetEventFlag` | ❌ none | Tier-3 fallback hits a random bit. `needs verification` on every change to an external ID. |
| L5 | BST coverage after a regulation update | ⚠️ `eventflag_bst.txt` may require re-extraction after a game patch | No automatic diff vs regulation. |
| L6 | Multi-flag atomic rollback | ❌ none | Multi-flag ops are not atomic. Caller-level snapshot required. |
| L7 | Cross-platform PS4 vs PC bit-equal | ⚠️ round-trip covered per-platform | There is no `TestConvert*` in `backend/core/` or `tests/` verifying that PS4 and PC store the same logical state (toggle grace on PS4 → save → convert → load PC → the same bit). `needs verification`. |
| L8 | DLC flag-block coverage (82xxx, 83xxx, 84xxx, etc.) | ⚠️ DLC patches may add new blocks | `needs verification` on a new DLC. |
| L9 | "Talisman Pouch sync" historical note | ⚠️ mentioned in earlier iterations as 60500/60510/60520 | In the current code (2026-05-21): `Talisman Pouch` is the key item `0x40002738`; there is **no** hardcoded 60500/60510/60520 in `itemCompanionEventFlags` (the only item there with a 4-flag companion = Spectral Steed Whistle). `needs verification` whether 60500/60510/60520 sync was ever implemented, or just an intent from research notes. |
| L10 | Tier 1 cache stale | ⚠️ `event_flags.go` is an offline-generated snapshot | If `event_flags.go` was not regenerated after a regulation update, Tier 1 may have stale `(Byte, Bit)`. The `TestBSTLookupMatchesEventFlags` test will detect a Tier 1 vs Tier 2 discrepancy for the same IDs, but only if both precomputed and BST are regenerated consistently. |

---

## 18. Cross-references

| Topic | Master chapter |
|---|---|
| Map reveal (4 layers: regions, bitmap, DLC cover layer, fog of war) | [27 — Map Reveal](27-map-reveal.md) |
| Unlocked Regions (Layer 0 map reveal) | [11 — Regions](11-regions.md) |
| DLC Cover Layer (Layer 2 map reveal) | [29 — DLC Black Tiles](29-dlc-black-tiles.md) |
| Sites of Grace activation + companion flags (SET-only) | [47 — Sites of Grace Activation](47-site-of-grace-activation.md) |
| Item companion flags (SET + CLEAR symmetric) | [50 — Item Companion Flags](50-item-companion-flags.md) |
| Boss defeat flag + LastRestedGrace + ClearCount | [14 — Game State](14-game-state.md) |
| Boss multi-flag design (planned) | [38 — Boss Multi-Flag](38-boss-multiflag.md) |
| PvP modular presets (regions/colosseums/map/pools/graces) | [48 — PvP Modular Presets](48-pvp-ready-modular-presets.md) |
| Summoning pools runtime gap | [42 — Summoning Pools Bug](42-summoning-pools-bug.md) |
| World state (FieldArea, WorldArea, WorldGeomMan, RendMan — read-only) | [16 — World State](16-world-state.md) |
| Ban-risk Tier 1 UI for bulk flag operations | [32 — Ban-Risk System](32-ban-risk-system.md) |

---

## 19. Sources

### Code

- `backend/db/db.go::GetEventFlag` / `SetEventFlag` / `resolveEventFlagPosition`
- `backend/db/data/event_flags.go` — 865 precomputed `EventFlagInfo{Byte, Bit}`
- `backend/db/data/bst.go` + `eventflag_bst.txt` — 11,919 BST mapping entries
- `backend/core/section_eventflags.go` — `EventFlagsByteCount`, `EventFlagsBlock`, `PreEventFlagsScalars`
- `backend/core/structures.go::Slot.EventFlagsOffset` + chain calc (`s.EventFlagsOffset = s.IngameTimerOffset + DynEventFlags`)
- `app_world.go` — 20+ callsites of `db.SetEventFlag`/`GetEventFlag` (graces, bosses, pools, colosseums, cookbooks, map reveal, whetblades, quest)
- `app_pvp.go::ApplyPvPPreparation` — composite caller
- `backend/db/data/whetblades.go::AoWMenuUnlockedFlag`, `StormStompDupFlag`
- `backend/db/data/grace_companion_flags.go::GatefrontGraceEventFlagID`
- `backend/db/data/item_companion_flags.go`

### Tests

- `backend/core/section_eventflags_test.go` — round-trip PS4/PC
- `tests/map_flags_test.go::TestBSTLookupMatchesEventFlags`, `TestEventFlagsOffsetCorrectness`
- `tests/save_modify_test.go` — `TestEventFlagsBossToggle/KillAll`, `TestEventFlagsGraceToggle/UnlockAll`

### Reference parsers / community

- er-save-manager: `parser/event_flags.py` — the `EventFlags` class (reference BST algorithm)
- er-save-manager: `src/resources/eventflag_bst.txt` — source BST mapping
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs:197–223` — EventFlags container (`0x1bf99f` bytes)
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:event-flag-list
- Event Flags GitHub Pages: https://soulsmods.github.io/elden-ring-eventparam/
- Event Flags Spreadsheet: https://docs.google.com/spreadsheets/d/1Nn-d4_mzEtGUSQXscCkQ41AhtqO_wF2Aw3yoTBdW9lk
- TGA Cheat Engine Table: https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA

### Hex-verified saves

- `tmp/save/ER0000.sl2` (PC, 5 slots) — round-trip 2026-05-21
- `tmp/save/oisis_pl-org.txt` (PS4) — round-trip 2026-05-21
