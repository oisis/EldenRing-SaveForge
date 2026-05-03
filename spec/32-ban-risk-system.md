# 32 — Ban-Risk Awareness System (UI/UX)

> **Type**: Design doc  
> **Scope**: Architecture of the risky-edit warning system in the editor UI. Tier 0/1/2 + Online Safety Mode + `RISK_INFO` dictionary + React components (`RiskInfoIcon`, `RiskBadge`, `RiskActionButton`, `RiskSectionBanner`).

> **Status**: ✅ Deployed on branch `feat/ban-risk-system` (Apr 2026, phases 1-5). Phase 6 (polish + documentation) — this document.

---

## Goal

Easy Anti-Cheat (EAC) validates save state during online synchronization and flags configurations impossible to achieve in retail. The system educates the user about **why** a given edit may result in a ban — rather than just blocking or scaring them.

**Philosophy**: we never officially claim that "FromSoftware will ban for X". Text descriptions use phrasing like *"community-reported"*, *"reported bans on r/Eldenring"*, *"detection rules whose exact mechanism is not publicly documented"*.

---

## Tier system

| Tier | Meaning | UI reaction |
|---|---|---|
| **0** | Cosmetic / read-only / safe | No indicator |
| **1** | Caution — popular but detectable (bulk grace, map reveal, quest skip) | Modal-confirm only when Online Safety Mode enabled; off-mode = immediate action |
| **2** | High risk — known to cause bans (cut content, stat >99, runes >999M) | Modal-confirm (when Safety Mode on) + field outline + input clamping |

---

## Online Safety Mode

Global toggle in `Settings → Safety`. State: `localStorage.setItem('setting:onlineSafetyMode', 'true'|'false')`.

When active:
- **Global banner** at the top of the application (yellow bar `SafetyModeBanner.tsx`)
- **Tier 1 / Tier 2**: every action with `RiskActionButton` shows a modal-confirm; modal has no "Don't ask again" checkbox — instead a permanent note "Online Safety Mode is on — confirmation required"
- **Tier 2 inputs**: edits are clamped to legal values (e.g. Runes auto-cap to `999_999_999` with toast)

When disabled:
- **No modals** for `RiskActionButton` — click triggers action immediately. Education remains available on-demand via the ⚠ info icon next to each button
- **Inputs** still validate/clamp (Tier 2 doesn't need a modal — UI prevents entering out-of-range values)

Hook: `useSafetyMode()` from `frontend/src/state/safetyMode.tsx` returns `{enabled, setEnabled, isDisabledFor(tier), requireConfirmFor(tier)}`. `requireConfirmFor` returns `true` only when `enabled && tier >= 1`.

---

## Dictionary `RISK_INFO`

Location: `frontend/src/data/riskInfo.ts`.

Entry structure:
```ts
interface RiskEntry {
    tier: 0 | 1 | 2;
    level: 'low' | 'medium' | 'high';  // affects color (yellow/orange/red)
    title: string;
    whyBan: string;       // detection mechanism description
    reports: string;      // scale/frequency of reports without inventing numbers
    mitigation: string;   // specific advice on how to limit risk
    sources: { label: string; url?: string }[];  // URL optional (empty when unverified)
}
```

`RiskKey` (string union) is typed — new entries extend the union. TypeScript enforces presence of every key in `Record<RiskKey, RiskEntry>`.

### Current entries

**Per-flag (Tier 2)** — synchronized with `backend/db/db.go::ItemEntry.Flags`:
- `cut_content`, `pre_order`, `dlc_duplicate`, `ban_risk`

**Per-action / per-field (Tier 2)** — Phase 3:
- `runes_above_999m`, `stat_above_99`, `level_above_713`, `talisman_pouch_above_3`
- `quantity_above_max`, `spirit_ash_above_10`, `derived_stat_manual`

**Per-bulk-action (Tier 1)** — Phase 4:
- `bulk_grace_unlock`, `bulk_boss_kill`, `bulk_cookbook`, `bulk_bell_bearing`
- `bulk_gestures_unlock`, `bulk_region_unlock`, `bulk_summoning_pool`, `bulk_colosseum`
- `map_reveal_full`, `fow_remove`
- `quest_step_skip`, `ng_plus_write`, `character_import`

### Helper functions

```ts
getRunesRiskKey(runes: number): RiskKey | null;          // > 999_999_999
getAttributeRiskKey(value: number): RiskKey | null;       // > 99
getLevelRiskKey(level: number): RiskKey | null;           // > 713
getTalismanPouchRiskKey(slots: number): RiskKey | null;   // > 3
getQuantityRiskKey(qty, max): RiskKey | null;             // qty > max
getSpiritAshRiskKey(upgrade: number): RiskKey | null;     // > 10
```

These return a `RiskKey` (string) when the value is Tier 2, `null` otherwise — ready for use in conditional rendering `value && <Component riskKey={value}/>`.

---

## Components

### `<RiskInfoIcon riskKey="..."/>`
File: `frontend/src/components/RiskInfoIcon.tsx`.

Clickable ⚠ icon (color synced with `level`: yellow/orange/red). Click → popover (positioned via `getBoundingClientRect`, rendered through `createPortal(..., document.body)` — independent of parent overflow).

Popover contains: title, risk dots (`● ○ ○` / `● ● ○` / `● ● ●`), 3 sections (Why / Reports / Mitigation), source list, ESC + outside-click + click-toggle dismissal.

### `<RiskBadge flag="cut_content"/>`
File: `RiskBadge.tsx`.

Renders a styled badge (`CUT`, `⚠ BAN`, `PRE-ORDER`, `DLC DUP`) + adjacent clickable `RiskInfoIcon`. Only for per-flag riskKeys (per-action keys don't have a badge).

### `<RiskActionButton riskKey="..." onConfirm={...}>Label</RiskActionButton>`
File: `RiskActionButton.tsx`.

Wrapper around `<button>`:
- Renders button + adjacent clickable ⚠ icon (separate target — icon click does NOT trigger action)
- Button click: if `RISK_INFO[riskKey]` exists and `safetyMode.requireConfirmFor(tier)` → modal-confirm; otherwise `onConfirm()` immediately
- Modal: description from dictionary, permanent note "Online Safety Mode is on — confirmation required", Cancel/Proceed buttons
- Without SafetyMode → ⚠ icon remains as on-demand educational affordance

### `<RiskSectionBanner riskKey="..."/>`
File: `RiskSectionBanner.tsx`.

Banner above a section warning about risks of the entire edit category (e.g. "Quest Step Skip" above the quest list). First sentence from `whyBan` + info icon. Color synced with `level`.

### `<SafetyModeBanner/>`
File: `SafetyModeBanner.tsx`.

Global banner at the top of the application visible when `useSafetyMode().enabled`. Static text "Online Safety Mode — Tier 2 edits disabled, Tier 1 requires confirmation".

---

## Coverage map (where used)

| Component | Tier | Pattern |
|---|---|---|
| **CharacterTab** | | |
| ↳ Runes input | 2 | Outline + ⚠ icon + clamping under SafetyMode |
| **InventoryTab / DatabaseTab** | | |
| ↳ Item with ban_risk/cut_content flag | 2 | RiskBadge inline |
| ↳ "Add Anyway" modal before adding ban_risk item | 2 | Modal warning (separate from RiskActionButton) |
| **WorldTab** | | |
| ↳ Map → Reveal All | 1 | RiskActionButton (`map_reveal_full`) + section banner |
| ↳ Graces → Unlock All | 1 | RiskActionButton (`bulk_grace_unlock`) |
| ↳ Summoning Pools → Activate All | 1 | RiskActionButton (`bulk_summoning_pool`) |
| ↳ Colosseums → Unlock All | 1 | RiskActionButton (`bulk_colosseum`) |
| ↳ Bosses → Kill All | 1 | RiskActionButton (`bulk_boss_kill`) |
| ↳ Quests → Set (per step) | 1 | RiskActionButton (`quest_step_skip`) + section banner |
| ↳ Gestures → Unlock All | 1 | RiskActionButton (`bulk_gestures_unlock`) |
| ↳ Gestures with ban_risk flag | 2 | RiskInfoIcon next to label |
| ↳ Cookbooks → Unlock All | 1 | RiskActionButton (`bulk_cookbook`) |
| ↳ Bell Bearings → Unlock All | 1 | RiskActionButton (`bulk_bell_bearing`) |
| ↳ Regions → Unlock All | 1 | RiskActionButton (`bulk_region_unlock`) |
| **CharacterImporter (Tools)** | 1 | RiskActionButton (`character_import`) on Confirm |
| **SettingsTab** | | |
| ↳ Online Safety Mode toggle | — | Checkbox + description |
| ↳ Show Cut & Ban-Risk Items toggle | — | Checkbox display filter |

**NOT covered** (intentionally):
- `Lock All` / `Reset` / `Respawn All` — Tier 0 (revert to safe state)
- Talisman pouch / NG+ / attributes / quantity inputs — clamping in `onChange` prevents Tier 2 via UI; outline is dead for these fields
- Whetblades — low risk (bonus weapon upgrade), skipped in first iteration

---

## How to add a new risk

1. **Dictionary entry**: add a key to `RiskKey` union and an entry in `RISK_INFO` in `frontend/src/data/riskInfo.ts`. TypeScript will enforce that you don't forget fields.
2. **Per-flag (badge)**: add an entry in `STYLE` in `RiskBadge.tsx` (label + Tailwind classes).
3. **Per-action (modal-confirm)**: use `<RiskActionButton riskKey="...">` instead of `<button>` where the action is triggered.
4. **Per-field (outline)**: add helper `getXxxRiskKey()` in `riskInfo.ts`, conditionally render outline + `<RiskInfoIcon>` in the component.
5. **Per-section (banner)**: use `<RiskSectionBanner riskKey="...">` at the top of the section.

---

## Extension plan (future)

- **Per-action override for per-flag entries**: when a specific action has a different description than generic `cut_content` (e.g. adding a cut helm vs a cut quest item) — an override layer can be introduced.
- **CharacterTab top banner when save has Tier 2 values**: detect after load and display a warning "This save was edited with values flagged as Tier 2 by the community".
- **Sources URLs**: verification and completion of empty `url` fields in `sources` (e.g. links to specific Reddit threads).
- **Dictionary completeness test**: script that greps all uses of `riskKey="..."` / `RISK_INFO[...]` and checks consistency with the `RiskKey` type.

---

## Sources

- **Files**: `frontend/src/data/riskInfo.ts`, `frontend/src/state/safetyMode.tsx`, `frontend/src/components/Risk{InfoIcon,Badge,ActionButton,SectionBanner}.tsx`, `frontend/src/components/SafetyModeBanner.tsx`.
- **Backend tags**: `backend/db/db.go::ItemEntry.Flags` — flag list (`cut_content`, `pre_order`, `dlc_duplicate`, `ban_risk`, `dlc`, `stackable`).
- **Community**: r/Eldenring ban threads (2022-2024), Fextralife notes on cut content.
