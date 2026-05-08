# 42 — Summoning Pools: UI Works, No In-Game Effect

> **Type**: Investigation / Bug
> **Extracted from**: docs/ROADMAP.md (2026-05-03 cleanup)
> **Status**: ✅ Fixed — `summoning_pools.go` updated to v1.12+ IDs (`670xxx`)

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

## Root Cause (identified 2026-05-07)

**Patch v1.12 (released ~March 2025) changed all summoning pool flag IDs.**

- Flags `10000040`, `1035530040`, etc. worked only in game versions `< v1.12`.
- Current game (`>= v1.12`) uses IDs in the `670xxx` range (e.g., Stormveil Castle: `670130–670135`).
- Reference: `Elden-Ring-CT-TGA/Event Flags/Unlock all/Unlock all Summoning Pools.cea` — contains the deprecated block with old IDs and the current active block with new IDs.

Verified through diagnostics:
- `EventFlagsOffset` is correct (static + dynamic match on PC and PS4) ✅
- Bit survives `Save → Load` round-trip on both platforms ✅
- BST block `670` already present in `eventflag_bst.txt` (position 107) — no lookup table changes needed ✅
- Root cause confirmed: `summoning_pools.go` uses old IDs which the game ignores ❌

## Fix plan

1. Replace all 165 IDs in `backend/db/data/summoning_pools.go` with new `670xxx` IDs from CT-TGA
   - Base game: 162 IDs (`flagsBase` in CT-TGA)
   - DLC (Shadow of the Erdtree): remaining entries from `flagsDLC1`
   - Names mapped sequentially by region (CT-TGA preserves region order; per-pool names from our existing data)
2. `event_flags.go` and `eventflag_bst.txt` — **no changes needed** (BST block 670 handles new IDs)
3. Same root cause likely applies to Colosseums (`60350`, `60360`, `60370` may also need a post-v1.12 audit)

## Related

- Colosseum toggle — same symptom, same probable cause (flag IDs may have changed in v1.12)
- Sites of Grace toggle — partially works (map visible, not fast-travel) — unrelated, different mechanism
- Diagnostic scripts in `tmp/scripts/diag/`: `eventflags_offset_check.go`, `eventflags_persist_check.go`
