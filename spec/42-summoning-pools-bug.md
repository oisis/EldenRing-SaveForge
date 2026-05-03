# 42 — Summoning Pools: UI Works, No In-Game Effect

> **Type**: Investigation / Bug
> **Extracted from**: ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🐛 Paused (since 2026-04-25)

---

## Symptom

UI toggles summoning pools correctly (no errors), but toggled pools are NOT active in-game (tested offline to avoid bans). All pools affected, not specific ones.

## Diagnostic checklist (all passed ✅)

- [x] Database covers all pool IDs (165 pools, more than ClayAmore/ER-Save-Editor reference of 162)
- [x] Lookup table `event_flags.go` includes pool IDs with byte/bit offsets bit-for-bit identical to ER-Save-Editor
- [x] BST resolver produces identical offsets (verified `1037530040`, `1051570840`, `1060440040`)
- [x] `SetEventFlag` flips the correct bit in `slot.Data[EventFlagsOffset:]` slice (backing array — modifications propagate)
- [x] `SaveSlot.Write()` does NOT overwrite event flag region (only writes level/stats/name/runes)
- [x] `SaveFile()` serializes `slot.Data` directly without rebuild from parsed structs

## Remaining hypotheses

1. **Persistence test missing** — write integration test: `LoadSave → Set → SaveFile → LoadSave → Get` to verify the bit survives the round-trip. If it doesn't survive, look at `core/writer.go` or encryption pipeline.

2. **Game requires secondary state** — bit may be set in event_flags but game might also check:
   - `unlocked_regions` for the pool's map area (dependency on Invasion Regions feature)
   - Trophy data section (`trophy_data` 52 bytes)
   - `world_area` / `gaitem_game` cross-references

3. **Hash region (`CSPlayerGameDataHash`, last 0x80 bytes of slot)** — currently preserved verbatim. Game may validate it against runtime state when DLC is installed.

4. **PS4-specific** — PS4 saves are unencrypted, but PC SteamID-bound encryption may interact with our flag write.

## Action plan (when resumed)

1. Write `tests/event_flag_persistence_test.go` covering Set → Save → Load → Get round-trip
2. If round-trip persists → investigate game-side requirements (compare with reference save where pools are activated)
3. If round-trip fails → trace where the bit gets lost in the writer/encryption pipeline
4. Cross-check with Invasion Regions — maybe pool activation requires the matching region to be unlocked

## Related

- Colosseum toggle has the same symptom (flags set, no in-game effect)
- Sites of Grace toggle partially works (map visible but not fast-travel activated)
- These may all share a common root cause (secondary game-side validation beyond event flags)
