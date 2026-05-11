# 51 — Advanced Save Editor

> **Type**: Design doc
> **Status**: 🔲 Planned
> **Scope**: Single "Advanced → Save Editor" tab for power-users to read/write known and experimental save values across three technically distinct layers: Event Flags, regulation snapshot params, and raw offsets.

---

## Overview

A single editor tab exposed under **Advanced → Save Editor** that lets the user build a list of pending changes, preview them, and apply them atomically.

Intended audience: researchers, modders, power-users who need access to values not exposed by the main editor tabs.

> **Warning shown in UI:**
> Advanced editor. Do not change values unless you know exactly what they do.
> Wrong values may corrupt the save, break quests, or increase online enforcement/ban risk.
> Always keep a backup.

A confirmation checkbox ("I understand this can corrupt my save or increase ban risk") must be checked before **Apply changes** is enabled.

---

## Critical Distinction

```
regulation.bin params ≠ event flags ≠ raw offsets
```

| Layer | Source | R/W |
|---|---|---|
| Event Flags | Slot EventFlags bitfield (1.8 MB) | R/W |
| Regulation Snapshot Params | UD11 NETWORK_PARAM_ST snapshot in slot | R/W |
| Raw Offsets | Arbitrary UD10/UD11 byte position | R/W (Phase 2+) |

These are shown in **one tab** but are technically different storage regions. The UI must make this distinction explicit via target-type labeling.

---

## UI Layout

### Row format (pending change entry)

```
[Target dropdown/search] [Current/default value] [New value] [+]
```

### Pending changes list

```
Pending changes:
- EventFlag 60290: false → true
- NetworkParam.breakInRequestIntervalTimeSec: 30.0 → 4.0
- RawOffset UD10+0x19524 f32: 0.033333 → 0.1

[Apply changes] [Reset] [Export patch report]
```

### Preview diff (shown before Apply)

```
Will change:
EventFlag 60290: false → true
NetworkParam.breakInRequestIntervalTimeSec: 30.0 → 4.0
RawOffset UD10+0x19524 f32: 0.033333 → 0.1
```

---

## Target Types

### 1. Known EventFlags

Named flags with descriptions selectable from a dropdown/search.

| Flag ID | Description |
|---|---|
| 60100 | Obtained Spectral Steed Whistle |
| 4680 | Melina accord / whistle-given state |
| 4681 | Melina accord popup state |
| 710520 | Whistle world state |
| 60230 | Small Golden Effigy obtained |
| 60240 | Duelist's Furled Finger obtained |
| 60250 | Small Red Effigy obtained |
| 60280 | White Cipher Ring obtained |
| 60290 | Blue Cipher Ring shop/obtained state |
| 60300 | Taunter's Tongue obtained |
| 76111 | Gatefront grace |
| 10009655 | Roundtable invitation handled |
| 11109658 | RTH / Gideon marker |
| 11109659 | Gideon advice marker |

Display format: `EventFlag 60290 — Blue Cipher Ring shop/obtained state`

Value type: `bool` (true/false).

### 2. Known Regulation Snapshot Params

Params from the regulation.bin snapshot that have verified R/W support in the save file.

**MVP scope: NetworkParam only** (UD11 NETWORK_PARAM_ST, index 0).

| Field | Offset | Type | Vanilla default |
|---|---|---|---|
| maxBreakInTargetListCount | +0x70 | u32 | 5 |
| breakInRequestIntervalTimeSec | +0x74 | f32 | 30.0 |
| breakInRequestTimeOutSec | +0x78 | f32 | 20.0 |
| breakInRequestAreaCount | +0x7C | u32 | 5 |

Display format: `NetworkParam[0] +0x74 — breakInRequestIntervalTimeSec`

Later extensions (Phase 3+): ShopLineupParam viewer, ItemLotParam viewer, EquipParamGoods viewer, read-only regulation param browser.

### 3. App-known Macros

Predefined sets of flag changes that mirror what the application already does internally.

| Macro | Changes |
|---|---|
| Mark Spectral Steed Whistle obtained | sets 60100, 4680, 4681, 710520 |
| Mark Blue Cipher Ring purchased | sets 60290 |
| Mark Gatefront / Melina accord handled | sets 60100, 4680, 4681, 710520 |

Selecting a macro expands into individual pending change entries, each shown explicitly in the pending list.

### 4. Manual EventFlag by ID

Expert mode for unknown/research flags.

```
EventFlag ID: [input field]
Current:      [read from save]
New:          [true / false]
```

Use cases: research, testing newly discovered flags, quick debugging.

### 5. Raw / Unknown Offsets

**Phase 2 — not in MVP.**

Format: `RawOffset UD10+0x19524 — Unknown f32`

Type selector: `u8 / u16 / u32 / s32 / f32 / bytes`

Rules:
- High risk of save corruption.
- Backup required before use.
- No default value unless there is a known reference source.
- Reset to current/original value only (no vanilla default).
- Mandatory patch report on apply.

---

## MVP Scope

**In MVP:**
- Known EventFlags read/write
- Manual EventFlag by ID
- Known NetworkParam fields read/write (NETWORK_PARAM_ST)
- App-known macros
- Pending changes list
- Preview diff before Apply
- Apply changes (atomic)
- Export patch report

**Not in MVP:**
- Raw offset editor
- Arbitrary binary patching
- Full regulation param browser
- ShopLineupParam write support
- ItemLotParam write support

---

## Phase Plan

### Phase 1 — MVP (described above)

Known EventFlags + manual flag + NetworkParam + macros + pending list + apply + patch export.

### Phase 2 — Raw Offset Editor

- UD10/UD11 known candidate offsets with type selector
- Current value read, new value write
- Mandatory backup warning
- Patch report on apply

### Phase 3 — Regulation Browser

- Read-only browsing of extracted XML/CSV regulation params
- Search by param type, row ID, field name, offset, description
- Source: `tmp/regulation-bin-dump/csv/` + `tmp/regulation-bin-dump/defs/`

### Phase 4 — Patch Import/Export

- Export reusable patch JSON (flag set + param set + description)
- Import patch JSON
- Share/publish research patches safely
- Validate patch against current save state before apply

---

## Safety Requirements

- Backup must exist before Apply executes (use existing backup manager).
- Apply is atomic: all-or-nothing within the pending list.
- Preview diff shown before Apply.
- Confirmation checkbox required.
- Patch report exported on request (text summary of what was changed).
- Ban risk annotation per target type (mirrors spec/32 Tier system).

---

## Sources

- `spec/15-event-flags.md` — EventFlags bitfield layout
- `spec/18-network.md` — Network Manager section
- `spec/24-user-data-11.md` — regulation.bin snapshot (UD11)
- `spec/44-network-param-tuning.md` — NETWORK_PARAM_ST field reference
- `spec/32-ban-risk-system.md` — Risk tier architecture
- `spec/50-item-companion-flags.md` — companion flag mechanic (macros reference)
