# 19 — Weather & Time

> **Type**: Binary format spec (read-only reference)
> **Scope**: `WorldAreaWeather` (12 B) + `WorldAreaTime` (12 B) — what the code parses, why there is no setter, what the editing risks are.

Cross-refs: [14-game-state.md](14-game-state.md), [16-world-state.md](16-world-state.md).

---

## 1. Chapter purpose

To define unambiguously:

- what the code parses as the weather state (`WorldAreaWeather`, 12 B),
- what the code parses as the time (`WorldAreaTime`, 12 B),
- why SaveForge **does not expose** a public endpoint to edit weather/time,
- what risks come with manual mutation of these fields.

It does not duplicate the World State overview (see [16-world-state.md](16-world-state.md)).

## 2. Status

| Aspect | Status |
|---|---|
| `WorldAreaWeather` struct parser | ✅ `backend/core/section_trailing.go:7-45` |
| `WorldAreaWeatherSize = 12` | ✅ `section_trailing.go:14` |
| `WorldAreaTime` struct parser | ✅ `backend/core/section_trailing.go:48-79` |
| `WorldAreaTimeSize = 12` | ✅ `section_trailing.go:54` |
| Read/Write verbatim within `TrailingFixedBlock` | ✅ round-trip preserved |
| App-level public endpoint (`SetWeather`, `SetTime`, etc.) | ❌ **none** — `grep` in `app*.go` returns 0 results |
| Frontend UI for weather/time | ❌ none |
| Test coverage | ✅ `section_trailing_test.go` — `TestTrailingRoundTripPS4`/`PC` (2 tests, covering the whole `TrailingFixedBlock`) |

## 3. Source of truth in code

| File / symbol | What it contains | Mode |
|---|---|---|
| `backend/core/section_trailing.go::WorldAreaWeather` (lines 7-12) | 12 B struct: `AreaID + WeatherType + Timer + Padding` | read/write verbatim |
| `backend/core/section_trailing.go::WorldAreaWeatherSize` (line 14) | `= 12` | const |
| `backend/core/section_trailing.go::WorldAreaWeather.Read/Write` (lines 16-45) | Parser + serializer of fields | RW |
| `backend/core/section_trailing.go::WorldAreaTime` (lines 48-52) | 12 B struct: `Hour + Minute + Second` (3× u32) | read/write verbatim |
| `backend/core/section_trailing.go::WorldAreaTimeSize` (line 54) | `= 12` | const |
| `backend/core/section_trailing.go::WorldAreaTime.Read/Write` (lines 56-79) | Parser + serializer | RW |
| `backend/core/section_trailing.go::TrailingFixedBlock` (lines 187-196) | Aggregate structure: `Weather + Time + BaseVersion + SteamID + PS5Activity + DLC` (130 B total) | aggregate |
| `backend/core/section_trailing_test.go` | 2 round-trip tests (PS4 + PC) — covering the whole `TrailingFixedBlock` | tests |

Both structures are part of `TrailingFixedBlock` — the slot's trailing block that also contains `BaseVersion`, `SteamID`, `PS5Activity`, `DLCSection`. This chapter describes **only** Weather + Time; the remaining fields are in other chapters.

## 4. Mental model

```
TrailingFixedBlock (130 B total)
  ├─ WorldAreaWeather (12 B)
  │    ├─ AreaID       u16  (2 B)  → weather region identifier
  │    ├─ WeatherType  u16  (2 B)  → weather type (in-game weather enum)
  │    ├─ Timer        u32  (4 B)  → duration of the current weather
  │    └─ Padding      u32  (4 B)  → preserved verbatim
  ├─ WorldAreaTime (12 B)
  │    ├─ Hour    u32  (4 B)  → hour (0-23)
  │    ├─ Minute  u32  (4 B)  → minute (0-59)
  │    └─ Second  u32  (4 B)  → second (0-59)
  ├─ BaseVersion (16 B)   → see other sections
  ├─ SteamID (8 B)        → see [01-header.md] / steamid section
  ├─ PS5Activity (32 B)   → trophy/activity data
  └─ DLCSection (50 B)    → DLC entry flag — see [29]
```

The current code treats both sections as **read-write verbatim** — load parses into typed fields, save writes those same bytes 1:1. No validation or transformation.

## 5. Current parsed data

### 5.1 `WorldAreaWeather`

| Field | Type | Size | Comment |
|---|---|---|---|
| `AreaID` | `u16` | 2 B | Weather region identifier (in-game area ID) |
| `WeatherType` | `u16` | 2 B | Weather type — values map onto the in-game weather enum |
| `Timer` | `u32` | 4 B | Duration of the current weather (runtime counter) |
| `Padding` | `u32` | 4 B | Preserved verbatim |

**Sum**: `2 + 2 + 4 + 4 = 12 B`.

`needs verification`:

- Mapping of `WeatherType` values → human-readable names (rain/sunny/snow/etc.) — does not exist in the SaveForge code.
- The `AreaID` range (which values are legal) — unverified; no validator against `regulation.bin`.
- The semantics of `Timer` (unit — frames/seconds/ms?) — unverified.
- Whether `Padding` is really padding or a hidden field — unverified.

### 5.2 `WorldAreaTime`

| Field | Type | Size | Comment |
|---|---|---|---|
| `Hour` | `u32` | 4 B | Hour (in-game) |
| `Minute` | `u32` | 4 B | Minute |
| `Second` | `u32` | 4 B | Second |

**Sum**: `4 + 4 + 4 = 12 B`.

`needs verification`:

- Whether `Hour` is 0-23 (24-hour) or another convention — the code comment does not specify.
- Whether `Minute`/`Second` have a standard range (0-59) or the game allows other values — unverified.
- Whether the 32-bit types are intentional (the game probably uses smaller ones) — historical compatibility with the parser format.

## 6. Read-only status

**SaveForge does not expose a public endpoint to edit the weather or the time.**

`grep -rn "WorldAreaWeather\|WorldAreaTime\|setWeather\|setTime" app*.go` returns **0 results** for all `app*.go` files. The public API (Wails bindings) **does not allow**:

- setting `AreaID`, `WeatherType`, `Timer`, `Padding`,
- setting `Hour`, `Minute`, `Second`,
- setting any fields in either structure.

The frontend (`frontend/src/components`) has no component displaying or editing weather/time.

Any mutation would have to be done manually via a direct memory hex edit outside SaveForge — see §8 for risks.

## 7. What SaveForge does not implement

| Function | Status |
|---|---|
| `SetWeather(areaID, weatherType)` | ❌ not present |
| `SetTime(hour, minute, second)` | ❌ not present |
| `ResetWeatherTimer()` | ❌ not present |
| `AreaID` validation (whether it exists in the game) | ❌ not present |
| `WeatherType` validation against the `regulation.bin` weather enum | ❌ not present |
| `Hour` 0-23 / `Minute` 0-59 / `Second` 0-59 validation | ❌ not present |
| Frontend UI for editing weather/time | ❌ not present |
| Mapping table `WeatherType` → name (rain/sunny/etc.) | ❌ not present |
| "Corrupted" weather/time detection (the old doc had it) | ❌ removed — the heuristic "area_id == 0 = corruption" has no code confirmation |

`needs verification`: whether a planned weather/time editing function exists — not found in `docs/ROADMAP.md` nor in the issue tracker. Currently all changes must be made through the game (time advances at runtime; weather changes via scripts).

## 8. Relation to World State

See [16-world-state.md](16-world-state.md) §6.1 + §5. `WorldAreaWeather` and `WorldAreaTime` are **separate** sections within `TrailingFixedBlock` — not part of `WorldGeomBlock` (`FieldArea / WorldArea / WorldGeomMan / WorldGeomMan2 / RendMan`).

The World State subsystem map table ([16](16-world-state.md) §5) classifies `WorldAreaWeather / Time` as a separate chapter with the mode "RO verbatim" — in the current code de facto **RO** (read/write verbatim in the I/O round-trip, no app-level mutator).

`needs verification`: whether the game on load enforces consistency of weather/time with the `WorldGeomMan` blobs (script state may be related). Unverified.

## 9. Validation and safety notes

### 9.1 Manual Weather mutation

Without an app-level endpoint, the only editing path is a direct hex edit. Risks:

- **Wrong `AreaID`** → the game may try to load a weather script for a nonexistent region → undefined behavior, probably a no-op or default weather.
- **Wrong `WeatherType`** → outside the game weather enum range → the game may crash or fall back to default.
- **Timer desynchronized** with the weather script → possible visual desync (e.g., weather changes immediately after load).

`needs verification`: whether the game has defensive bounds-checking for the `WeatherType` enum, or trusts the save blind.

### 9.2 Manual Time mutation

- **Hour > 23** → undefined behavior; the game probably modulo 24 or crashes.
- **Minute/Second > 59** → same.
- **Time desynchronized** with the dynamic in-game day cycle script → short-lived desync (the game adjusts on the next tick).

`needs verification`: whether changing time affects NPC spawn / event timing (the old doc claimed "affects NPC and event spawns", but that was without code/runtime verification).

### 9.3 Coupling with World State scripts

`WorldAreaWeather` may be part of a wider script state in the `WorldGeomMan` blob (see [16-world-state.md](16-world-state.md) §6.1). Changing weather on its own, without updating the related script state, may cause a visual vs script desync.

`needs verification`: whether `WorldGeomMan` contains weather-related state that requires sync with `WorldAreaWeather`. No isolated test.

### 9.4 No write endpoint + no in-game verification

The absence of a public endpoint means:

- No risk gate in the UI (Tier 0/1/2).
- No rollback hook (`pushUndo` etc.).
- No per-field validation.
- No CI test "set weather → reload → assert state".

Any change made outside SaveForge (e.g., a hex editor) is **completely unverified** by SaveForge in the I/O round-trip — only the bytes are preserved, the semantics are the user's responsibility.

### 9.5 Platform / version differences

Both `WorldAreaWeatherSize = 12` and `WorldAreaTimeSize = 12` are constant for PC and PS4 (slots of both platforms have an identical `TrailingFixedBlock` layout).

`needs verification`: whether future game patches may extend `WorldAreaWeather` (e.g., add extra fields). No detection mechanism.

### 9.6 Old "corruption" heuristic — removed

The old PL doc claimed "`Area ID == 0` → the weather is probably corrupted" and "`Hour == 0 AND Minute == 0 AND Second == 0` → the time is potentially corrupted". In the current code there is **no** such validation — SaveForge does not signal weather/time corruption in any diagnostic endpoint. The heuristics were only suggestions for external tools, not code-confirmed; removed from this chapter.

## 10. Test coverage

| Test | File | What it verifies |
|---|---|---|
| `TestTrailingRoundTripPS4` | `backend/core/section_trailing_test.go:75` | Load → Write → diff = 0 for the whole `TrailingFixedBlock` (PS4); covers Weather + Time + BaseVersion + SteamID + PS5Activity + DLC |
| `TestTrailingRoundTripPC` | `backend/core/section_trailing_test.go:79` | same for a PC save |

**Missing**:

- An isolated `WeatherType` enum validation test.
- An isolated `Hour`/`Minute`/`Second` range validation test.
- An isolated mutation + reload test (because there is no mutator).
- A test of Weather/Time consistency with the `WorldGeomMan` script state.

## 11. Known limits / needs verification

1. **`WeatherType` → human-readable mapping** — none in SaveForge; `regulation.bin` could be parsed for this.
2. **`AreaID` weather-region semantics** — unverified, no mapping onto `data.Regions`.
3. **`Timer` unit** — unverified (frames/sec/ms?).
4. **`Padding` u32 in `WorldAreaWeather`** — unverified, probably padding but may be a hidden field.
5. **`Hour`/`Minute`/`Second` ranges** — the game probably enforces 0-23/0-59/0-59 but there is no validator in SaveForge.
6. **Coupling with `WorldGeomMan` script state** — unverified, a possible desync source.
7. **No app-level Set API** — the current state; future implementation would require a validator + risk gate.
8. **In-game spawn/event timing impact** — the old doc claimed "affects NPC and event spawns"; currently `needs verification`.
9. **Cross-version layout stability** — no detection mechanism after a patch.
10. **Old "corruption" heuristic** — removed (it was without code confirmation).

## 12. Cross-references

- [01-header.md](01-header.md) — SteamID in `TrailingFixedBlock` (a separate 8 B field).
- [14-game-state.md](14-game-state.md) — `PreEventFlagsScalars` (`InGameCountdownTimer`, `LastRestedGrace`, etc.); a separate block.
- [16-world-state.md](16-world-state.md) — World State overview; Weather/Time classified as RO verbatim.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — `DLCSection` in `TrailingFixedBlock`; a separate field.

## 13. Sources

- `backend/core/section_trailing.go` — `WorldAreaWeather` (12 B), `WorldAreaTime` (12 B), `TrailingFixedBlock` aggregator.
- `backend/core/section_trailing_test.go` — 2 round-trip tests (PS4 + PC).
- No callers in `app*.go` (verified via `grep`).
- No frontend components (verified via `grep`).
