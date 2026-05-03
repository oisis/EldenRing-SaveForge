# 40 — DB Cleanup: Cut-Content Registry & Multiplayer Dedup

> **Type**: Design doc
> **Extracted from**: ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planned

---

## Goal

Comprehensive cleanup of in-app Item Database based on user-reported in-game evidence (2026-04-28). Many items appear with `[ERROR]` prefix in-game (missing FMG entries), in wrong inventory section (ban risk), or as visual duplicates.

**Source of truth:** user in-game screenshots (`tmp/screeny-apki/`).

---

## Phase A — Empty Flask variants cleanup

**File:** `backend/db/data/tools.go`

1. Verify in `tmp/erdb/1.10.0/EquipParamGoods.csv` whether `(Empty)` IDs have distinct game params or are placeholder duplicates of `(Filled)` IDs
2. If verified as redundant: remove ~27 `(Empty)` entries:
   - Crimson Tears Flask Empty: 0x400003E8 + 12 upgrade variants
   - Cerulean Tears Flask Empty: base + 12 upgrade variants
   - Wondrous Physick Flask Empty: 0x400000FA
3. Cross-check `goodsType` column — writer routing may depend on type

---

## Phase B — Multiplayer active/inactive dedup

**Files:** `backend/core/writer.go`, `backend/db/db.go`

**Hypothesis:** save stores separate handles for "inactive" (held) vs "active" (deployed/used) state of multiplayer items. Game auto-rewrites handle on activation. Our writer adds inactive handle even when active variant exists → user sees 2 in-game.

1. Forensic on `tmp/crash/ER0000.sl2` — search CommonItems for known multiplayer pairs to identify active-state IDs
2. Build `MultiplayerStatePairs map[uint32]uint32` (active→inactive) in `db/db.go`
3. In `addToInventory`: before insert, check if active OR inactive variant already present. Skip if yes
4. Affected items: Tarnished's Wizened Finger (0x4000006A), Tarnished's Furled Finger (0x400000AA), Small Golden Effigy (0x400000B3), Small Red Effigy (0x400000B4) + likely all 11 multiplayer items in `tools.go:7-19`

---

## Phase C — Cut-content registry + flag uncertain items

Items to flag `cut_content, ban_risk` (user reported `[ERROR]` in-game OR wrong save section):

**Notes (info.go) — flag, keep in DB:**

| ID | Name |
|---|---|
| 0x4000222E | Note: Hidden Cave |
| 0x4000222F | Note: Imp Shades |
| 0x40002230 | Note: Flask of Wondrous Physick |
| 0x40002231 | Note: Stonedigger Trolls |
| 0x40002232 | Note: Walking Mausoleum |
| 0x40002233 | Note: Unseen Assassins |
| 0x40002235 | Note: Flame Chariots |
| 0x40002236 | Note: Demi-human Mobs |
| 0x40002237 | Note: Land Squirts |
| 0x40002238 | Note: Gravity's Advantage |
| 0x4000223A | Note: Waypoint Ruins |
| 0x4000223D | Note: Frenzied Flame Village |

**Tools (tools.go) — flag:**
- 0x40000BCC Miranda's Prayer (user reported `[Error]Modlitwa Mirandy` in-game)
- Scorpion Stew DLC — er-save-manager lists 4 IDs (2001200..2001203) with duplicate names; missing IDs to verify: `0x401E8934`, `0x401E8935`

**Key Items (key_items.go) — flag:**
- 0x4000229E Golden Order Principia (candidate for `[ERROR]Zasady Złotego Porządku`)

**Helms/Chest — unidentified set from screenshot:**
- Mark as cut_content after identifying via Fextralife + EquipParamProtector.csv comparison

---

## Phase D — Remove auto-given notes

Player receives these automatically at game start (confirmed) and cannot remove → no value in DB:
- 0x40002234 Note: Great Coffins
- 0x40002239 Note: Revenants
- 0x4000223B Note: Gateway

Check `presets/`, `audit/`, tests for references before removing.

---

## Phase E — Memory of Grace duplicate investigation

User reports Memory of Grace (0x40000073) appears 2× in in-game inventory after add, despite single DB entry and existing dedup logic. Hypothesis: legacy duplicate from pre-fix writer version persists in user's save.

Options:
1. Add load-time deduplicator: scan CommonItems for duplicate handles, merge stacks
2. Add save-time validator: warn if duplicate handles detected
3. Document as "fix on next clean save" if too risky

---

## Phase F — Tests + build verification

1. `go test -v ./backend/db/...`
2. `go test -v ./backend/core/...` (multiplayer dedup unit tests)
3. `cd frontend && npx tsc --noEmit && npm run lint`
4. `make build`

---

## Phase G — Docs

1. `CHANGELOG.md` — entry per phase
2. `ROADMAP.md` — mark completed
3. New `spec/` doc if cut-content registry grows large enough

---

## Open questions

- Helmet/chest item from screenshot — exact EN name needed (user to provide)
- Whether Phase B (multiplayer dedup) requires separate "active state" IDs in DB or just runtime save inspection
- Whether to keep flagged-but-not-removed notes in DB (current proposal) or move to a separate `cut_content_archive.go` file

**Effort estimate:** 4-6 iterations (Phase A trivial, B+C+D+E investigation-heavy)
