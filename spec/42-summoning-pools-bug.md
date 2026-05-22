# 42 — Summoning Pools: UI works, no in-game effect

> **Type**: Investigation / Bug
> **Extracted from**: docs/ROADMAP.md (cleanup 2026-05-03)
> **Status**: 🐛 Paused (since 2026-04-25)

---

## Symptom

The UI toggles summoning pools correctly (no errors), but the toggled pools are NOT active in the game (tested offline to avoid bans). It affects all pools, not specific ones.

## Diagnostic checklist (all passed ✅)

- [x] The database covers all pool IDs (165 pools, more than the ClayAmore/ER-Save-Editor reference of 162)
- [x] The `event_flags.go` lookup table contains the pool IDs with byte/bit offsets identical bit-for-bit with ER-Save-Editor
- [x] The BST resolver produces identical offsets (verified `1037530040`, `1051570840`, `1060440040`)
- [x] `SetEventFlag` flips the correct bit in the slice `slot.Data[EventFlagsOffset:]` (backing array — modifications propagate)
- [x] `SaveSlot.Write()` does NOT overwrite the event flag region (it writes only level/stats/name/runes)
- [x] `SaveFile()` serializes `slot.Data` directly without rebuilding from parsed structures

## Remaining hypotheses

1. **No persistence test** — write an integration test: `LoadSave → Set → SaveFile → LoadSave → Get` to verify whether the bit survives the round-trip. If it does not survive, look in `core/writer.go` or the encryption pipeline.

2. **The game requires secondary state** — the bit may be set in event_flags, but the game may also check:
   - `unlocked_regions` for the pool's map area (dependency on the Invasion Regions feature)
   - the trophy data section (`trophy_data` 52 bytes)
   - cross-references `world_area` / `gaitem_game`

3. **Region hash (`CSPlayerGameDataHash`, the last 0x80 bytes of the slot)** — currently preserved verbatim. The game may validate it against the runtime state when the DLC is installed.

4. **PS4-specific** — PS4 saves are unencrypted, but the PC encryption tied to the SteamID may interact with our flag write.

## Action plan (after resuming)

1. Write `tests/event_flag_persistence_test.go` covering the round-trip Set → Save → Load → Get
2. If the round-trip holds → investigate the game-side requirements (compare with a reference save where the pools are activated)
3. If the round-trip fails → trace where the bit is lost in the writer/encryption pipeline
4. Cross-check with Invasion Regions — pool activation may require unlocking the matching region

## Related

- The Colosseum toggle has the same symptom (flags set, no effect in the game)
- The Sites of Grace toggle works partially (map visible but fast-travel not activated)
- All may share a common cause (game-side secondary validation beyond event flags)
