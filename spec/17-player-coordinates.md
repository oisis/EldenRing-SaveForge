# 17 — Player Coordinates

> **Type**: Binary format spec (read-only reference)
> **Scope**: The `PlayerCoordinates` section (61 B) + `SpawnPointBlock` (≥10 B version-gated) — what the code parses, why there is no writer, what the editing risks are.

Cross-refs: [14-game-state.md](14-game-state.md), [16-world-state.md](16-world-state.md).

---

## 1. Chapter purpose

To define unambiguously:

- what the code parses as the player position (`PlayerCoordinates` struct, 61 B),
- what the code parses as the spawn point (`SpawnPointBlock`, version-gated),
- why SaveForge **does not expose** a public endpoint to edit coordinates,
- what risks come with manual mutation of these fields.

It does not duplicate the World State overview (see [16-world-state.md](16-world-state.md)) or Game State (see [14-game-state.md](14-game-state.md)).

## 2. Status

| Aspect | Status |
|---|---|
| `PlayerCoordinates` struct parser | ✅ `backend/core/section_player_coords.go:14-46` |
| `PlayerCoordinatesSize = 61` | ✅ `section_player_coords.go:23` (12+4+16+1+12+16) |
| `SpawnPointBlock` parser (version-gated) | ✅ `section_player_coords.go:73-128` |
| Read/Write verbatim within the slot section | ✅ round-trip preserved |
| App-level public endpoint (`SetPlayerCoords`, `Teleport`, etc.) | ❌ **none** — `grep` in `app*.go` returns 0 results |
| Frontend UI for coordinates | ❌ none |
| Test coverage | ✅ `section_player_coords_test.go` — `TestPlayerCoordsRoundTripPS4`/`PC` (2 tests) |

`needs verification`: the old doc claimed `PlayerCoordinatesSize = 57`. The current code has **61** (`const PlayerCoordinatesSize = 12 + 4 + 16 + 1 + 12 + 16 = 61`). A comment in the code explains: "er-save-manager labels this 57 bytes in a comment — the actual struct is 61 bytes; the comment is stale." The old value of 57 B in the PL spec was inherited from a stale er-save-manager comment.

## 3. Source of truth in code

| File / symbol | What it contains | Mode |
|---|---|---|
| `backend/core/section_player_coords.go::PlayerCoordinates` (lines 14-21) | 61 B struct: `Coordinates + MapID + Angle + GameMan0xbf0 + UnkCoords + UnkAngle` | read/write verbatim |
| `backend/core/section_player_coords.go::PlayerCoordinatesSize` (line 23) | `= 12+4+16+1+12+16 = 61` | const |
| `backend/core/section_player_coords.go::PlayerCoordinates.Read/Write` (lines 25-56) | Parser + serializer per field | RW |
| `backend/core/section_player_coords.go::SpawnPointBlock` (lines 73-81) | `PadAfterCoords + SpawnPointEntityID + GameMan0xb64 + (v>=65) TempSpawnPointEntityID + (v>=66) GameMan0xcb3` | read/write verbatim |
| `backend/core/section_player_coords.go::SpawnPointBlock.Read/Write/ByteSize` (lines 83-140) | Parser gated by the slot's `version` | RW |
| `FloatVector3` / `FloatVector4` / `MapID` | Helper primitives for parsing | reader/writer |
| `backend/core/section_player_coords_test.go` | 2 round-trip tests (PS4 + PC) | tests |

`PlayerCoordinates` and `SpawnPointBlock` are part of a wider slot section — `Read()` is invoked by the slot parser above (`backend/core/structures.go` or equivalent); this chapter does not duplicate the entire parsing pipeline.

## 4. Mental model

```
slot data
  ├─ ... (other sections)
  ├─ PlayerCoordinates (61 B)
  │    ├─ Coordinates    FloatVector3 (12 B)  → current XYZ position
  │    ├─ MapID          MapID        (4 B)   → map identifier (opaque 4-byte ID)
  │    ├─ Angle          FloatVector4 (16 B)  → rotation quaternion
  │    ├─ GameMan0xbf0   u8           (1 B)   → unknown (`unk`)
  │    ├─ UnkCoords      FloatVector3 (12 B)  → second set of coordinates (unknown semantics)
  │    └─ UnkAngle       FloatVector4 (16 B)  → second quaternion
  ├─ SpawnPointBlock (≥10 B, version-gated)
  │    ├─ PadAfterCoords           [2]byte (verbatim)
  │    ├─ SpawnPointEntityID       u32
  │    ├─ GameMan0xb64             u32
  │    ├─ TempSpawnPointEntityID   u32  (only if version >= 65)
  │    └─ GameMan0xcb3             u8   (only if version >= 66)
  └─ ... (other sections)
```

The current code treats both sections as **read-write verbatim** — load parses into typed fields, save writes those same bytes 1:1. No validation or transformation.

## 5. Current parsed data

`PlayerCoordinates`:

| Field | Type | Size | Code comment |
|---|---|---|---|
| `Coordinates` | `FloatVector3` | 12 B | main XYZ position |
| `MapID` | `MapID` | 4 B | map identifier |
| `Angle` | `FloatVector4` | 16 B | rotation quaternion |
| `GameMan0xbf0` | `u8` | 1 B | `unk` (named from the address in `GameMan`) |
| `UnkCoords` | `FloatVector3` | 12 B | semantics **unverified** |
| `UnkAngle` | `FloatVector4` | 16 B | semantics **unverified** |

**Sum**: `12 + 4 + 16 + 1 + 12 + 16 = 61 B`.

`SpawnPointBlock` (version-gated):

| Field | Type | Size | Gating |
|---|---|---|---|
| `PadAfterCoords` | `[2]byte` | 2 B | always verbatim |
| `SpawnPointEntityID` | `u32` | 4 B | always |
| `GameMan0xb64` | `u32` | 4 B | always |
| `TempSpawnPointEntityID` | `u32` | 4 B | only `version >= 65` (all our slots have `version >= 230`, so always present) |
| `GameMan0xcb3` | `u8` | 1 B | only `version >= 66` (same) |

`ByteSize()` returns the actual length based on the `HasTempSpawnPoint`/`HasGameMan0xcb3` flags set on `Read`. See `section_player_coords.go:131-140`.

`needs verification`: the meaning of `UnkCoords`/`UnkAngle` — the old doc proposed "backup ground position" / "spawn anchor". In the current code there is no caller using these fields in any logic — they are only preserved verbatim. The old doc about "spawn point / last stable position" remains a hypothesis without code confirmation.

`needs verification`: the meaning of `GameMan0xbf0`/`GameMan0xb64`/`GameMan0xcb3` (`needs verification` for all 3 fields — the names come from their address in the runtime `GameMan` structure, semantics unknown in SaveForge).

## 6. Read-only status

**SaveForge does not expose a public endpoint to edit the player position.**

`grep -rn "PlayerCoordinates\|SpawnPointBlock\|setCoord\|Teleport" app*.go` returns **0 results** for all `app*.go` files. The public API (Wails bindings) **does not allow**:

- setting `Coordinates`,
- setting `MapID`,
- setting `Angle`,
- setting `SpawnPointEntityID`,
- setting `TempSpawnPointEntityID`,
- setting any fields in either structure.

The frontend (`frontend/src/components`) has no component displaying or editing coordinates.

Any mutation would have to be done manually via a direct memory hex edit outside SaveForge — see §8 for risks.

## 7. What SaveForge does not implement

| Function | Status |
|---|---|
| `SetPlayerCoordinates(x, y, z)` | ❌ not present |
| `SetMapID(id)` | ❌ not present |
| `SetSpawnPoint(entityID)` | ❌ not present |
| `Teleport(x, y, z, mapID)` | ❌ not present |
| `RestoreLastBloodstain()` | ❌ not present |
| `MapID` validation (whether the ID exists in `regulation.bin`) | ❌ not present |
| Coordinates in-bounds validation | ❌ not present |
| Frontend UI for editing the position | ❌ not present |
| Mapping table `MapID` → region name | ❌ not present (`needs verification`: the old doc had a hypothetical pre-byte table; the current code does not use it) |

`needs verification`: whether a planned teleport function exists — not found in `docs/ROADMAP.md` nor in the issue tracker. Currently all position changes must be made through the game (rest at grace, teleport via menu).

## 8. Relation to World State

See [16-world-state.md](16-world-state.md) §6.1 + §5. `PlayerCoordinates` is a **separate** binary section — not part of `WorldGeomBlock` (`FieldArea / WorldArea / WorldGeomMan / WorldGeomMan2 / RendMan`). It is also not part of `BloodStain` (where L2 DLC tile coords have a separate partial mutator — see [29-dlc-black-tiles.md](29-dlc-black-tiles.md)).

The World State subsystem map table ([16](16-world-state.md) §5) classifies `Player Coordinates` as a separate chapter with the mode "RO/RW per chapter" — in the current code de facto **RO** (read/write verbatim, no app-level mutator).

## 9. Relation to Game State

See [14-game-state.md](14-game-state.md). `PlayerCoordinates` is **not** part of `PreEventFlagsScalars` nor `Game State`:

- `LastRestedGrace` (BonfireId — in `PreEventFlagsScalars`) is a **separate** respawn anchor mechanism. See [14](14-game-state.md) and [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §6.3.
- On respawn the game uses `LastRestedGrace` as the spawn anchor; `PlayerCoordinates` is updated by the game on every player movement.

**Safer "teleport" alternative**: changing `LastRestedGrace` in `PreEventFlagsScalars` — but the current code also does **not** expose that as a public endpoint (the game itself overwrites it on the first rest). See [14](14-game-state.md).

`needs verification`: whether changing `LastRestedGrace` in the save without touching `PlayerCoordinates` causes the game to teleport the player on load to the new grace — historical discussions suggest "no, until there is a physical rest", but there is no isolated test.

## 10. Validation and safety notes

### 10.1 Manual Coordinates mutation

Without an app-level endpoint, the only editing path is a direct hex edit. Risks:

- **Out-of-bounds** XYZ (outside the map geometry) → falling death → respawn at the last grace. Low cost, but frustrating.
- **Below the map** (Y too low) → instant death loop if the respawn is also below the map.
- **Inside geometry** (clip) → softlock, no collision recovery.
- **Inside a boss arena without an active encounter** → possible crash or stuck.

`needs verification`: whether the game has defensive bounds-checking on load, or trusts the save blind.

### 10.2 MapID mismatch

`MapID` is an opaque 4-byte ID. If the ID:

- **Does not exist** in `regulation.bin` → infinite loading / crash.
- **Exists, but the player does not own the DLC** → crash (DLC area IDs).
- **Exists, but inconsistent with Coordinates** → spawn in different geometry than the XYZ indicates. Possible falling/clip.

`needs verification`: whether SaveForge should add a `MapID` validator against the `data.Regions` snapshot — currently none. See [11-regions.md](11-regions.md).

### 10.3 SpawnPointBlock version mismatch

`SpawnPointBlock` has trailing fields gated by the slot's `version` (`>= 65` for TempSpawnPoint, `>= 66` for GameMan0xcb3). If you manually lower `Version` without removing the trailing bytes, the parser on load returns an out-of-range error or a mis-aligned read.

`PadAfterCoords` (2 B) is preserved verbatim — do not try to edit it as "reserve".

### 10.4 Quaternion (Angle)

The `Angle` quaternion (16 B = 4× f32) must be normalized (`x² + y² + z² + w² ≈ 1`). A zeroing or non-normalized mutation can cause undefined renderer behavior. `needs verification`: whether the game normalizes on load or trusts the save.

### 10.5 No write endpoint + no in-game verification

The absence of a public endpoint means:

- No risk gate in the UI (Tier 0/1/2).
- No rollback hook (`pushUndo` etc.).
- No per-field validation.
- No CI test "set coords → reload → assert position".

Any change made outside SaveForge (e.g., a hex editor) is **completely unverified** by SaveForge in the I/O round-trip — only the bytes are preserved, the semantics are the user's responsibility.

### 10.6 Platform / version differences

`PlayerCoordinatesSize = 61` and the `SpawnPointBlock` layout are the same for PC and PS4 (our slots have `version >= 230`, so both trailing fields in `SpawnPointBlock` are always present).

`needs verification`: whether future game patches may extend `PlayerCoordinates` (e.g., add a new `Unk` field). No detection mechanism.

## 11. Test coverage

| Test | File | What it verifies |
|---|---|---|
| `TestPlayerCoordsRoundTripPS4` | `backend/core/section_player_coords_test.go:83` | Load → Write → diff = 0 for a PS4 save |
| `TestPlayerCoordsRoundTripPC` | `backend/core/section_player_coords_test.go:87` | same for a PC save |

**Missing**:

- An isolated `MapID` validation test against `regulation.bin`.
- An XYZ bounds validation test.
- A quaternion normalization validation test.
- A mutation + reload test (because there is no mutator).

## 12. Known limits / needs verification

1. **`UnkCoords` / `UnkAngle` semantics** — unverified, probably spawn anchor or last bloodstain position.
2. **`GameMan0xbf0` / `GameMan0xb64` / `GameMan0xcb3` semantics** — unverified, named from the address in the runtime `GameMan` structure.
3. **`MapID` 4-byte format** — the old doc had a hypothetical byte-breakdown table; the current code treats it as an opaque ID. `needs verification`.
4. **In-bounds checking** in the game on load — no isolated test.
5. **Quaternion normalization tolerance** — no isolated test.
6. **Cross-version layout stability** — no detection mechanism after a patch.
7. **No app-level Set API** — the current state; future implementation would require a validator + risk gate.
8. **No Teleport endpoint** — not planned in the current code (outside SaveForge).
9. **`SpawnPointEntityID` mapping to in-game entity** — unverified; the ID is opaque.
10. **Old spec 57 B** — fixed in this chapter to 61 B.

## 13. Cross-references

- [11-regions.md](11-regions.md) — Region IDs (may be related to `MapID`, but no code-level mapping).
- [14-game-state.md](14-game-state.md) — `LastRestedGrace` as a safer "teleport" alternative (also no public API).
- [16-world-state.md](16-world-state.md) — World State overview; `PlayerCoordinates` is a separate section.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — `BloodStain` partial mutator with synthetic coordinates for L2 DLC tiles (separate from `PlayerCoordinates`).
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — `LastRestedGrace` BonfireId namespace; not modified by the editor.

## 14. Sources

- `backend/core/section_player_coords.go` — `PlayerCoordinates` (61 B), `SpawnPointBlock` (version-gated), FloatVector3/4 helpers.
- `backend/core/section_player_coords_test.go` — 2 round-trip tests.
- No callers in `app*.go` (verified via `grep`).
- No frontend components (verified via `grep`).
