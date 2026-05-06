# 45 — Ban Risk Reference

> **Type**: Reference doc
> **Status**: ✅ Current (last reviewed 2026-05)
> **Scope**: Community-reported ban triggers, penalty tiers, and safe-editing practices for Elden Ring online play. Informs the risk tier system documented in spec/32.

**Important disclaimer**: FromSoftware and Bandai Namco have not published the exact detection rules. Sections labeled **[Official]** are from primary sources. Sections labeled **[RE/verified]** were confirmed from Elden Ring's own game data files (regulation.bin dump in `tmp/regulation-bin-dump/`). Sections labeled **[Community-technical]** are from technically credible community RE reports. Sections labeled **[Community]** are unverified player reports.

---

## 1. Detection Architecture

### 1.1 Easy Anti-Cheat (EAC)

**[Community-technical]** — Runs locally in **user-mode** (not kernel-level; Elden Ring uses user-mode EAC unlike Nightreign 2025 which uses kernel-level EAC). At game launch, EAC:

1. Detects processes hooking or manipulating game memory (e.g. Cheat Engine running alongside).
2. Verifies integrity of game binaries on disk before allowing the game to start.
3. Scans memory regions reserved by the game process at runtime.

EAC does **not** read the `.sl2` save file — it inspects only the live game process and binaries.

**[Official]** — In June 2024, FromSoftware officially acknowledged that "Inappropriate activity detected" can appear as a **false positive** without any actual cheating, caused by corrupted game files. The official recommendation was to use Steam's "Verify integrity of game files".
> Source: [ELDENRING on X, June 2024](https://x.com/ELDENRING/status/1806689176449855497)

**EAC bypass methods** — documented on SoulsSpeedruns Wiki:
- **`steam_appid.txt` method** (does NOT block online): create `steam_appid.txt` containing `1245620` in the `Game/` directory, then launch `eldenring.exe` directly. EAC does not load; online still accessible via Steam.
- **`start_protected_game.exe` rename method** (blocks online): rename `start_protected_game.exe` + replace with a copy of `eldenring.exe`. Online disabled until restored.
> Source: [SoulsSpeedruns — EAC Bypass](https://soulsspeedruns.com/eldenring/eac-bypass)

Bypassing EAC only prevents the client-side scan — it does **not** bypass server-side save validation.

### 1.2 Server-Side Save Validation

**[Community-technical]** — Runs on FromSoftware servers during online synchronization. When a player connects online, the server loads their uploaded character state and checks it against what is achievable in a legitimate playthrough. Detection is triggered by:

- Item IDs not present in the retail item tables.
- Stat combinations outside the achievable range (see §3.2 for the Soul Memory check).
- Inventory counts exceeding vanilla maximums.
- Quest/world flag states inconsistent with prerequisite flags.
- Boss drop items present without the corresponding boss kill event flags.

Bans originate here, not from EAC. A player can run offline with a modified save and EAC active — the penalty only triggers on the next online sync. A confirmed case showed a 5-month delay between violation (October 2022) and penalty (March 2023).

**[Community-technical]** — The server does **not** validate a hash of the client's `regulation.bin`. EAC validates the binary on disk before launch; the server checks the resulting character state at connection time. Editing `regulation.bin` values (e.g. `NetworkParam`) is not caught by a server-side file hash comparison because FromSoftware does not appear to implement one — EAC catches a modified binary on the client side before it can connect with EAC active. With EAC bypassed offline, modified `regulation.bin` loads successfully; whether modified NetworkParam values cause detectable character state anomalies at online sync is unconfirmed.
> Source: FearlessRevolution modding Discord consensus; [waygate-server RE project](https://github.com/vswarte/waygate-server)

**[Official]** — The in-game penalty message reads:
> *"Unauthorized tampering with the game detected. A quarantine penalty of 180 days will be imposed."*

### 1.3 "Inappropriate activity detected" vs "Your account has been penalized" — two distinct systems

These are definitively **two separate messages** from two separate systems:

| Message | System | Meaning |
| :--- | :--- | :--- |
| "Inappropriate activity detected" (yellow banner at launch) | Local — EAC or game binary check | Save or binary flagged as suspicious, OR false positive (corrupted files). Still able to play online. |
| "Your account has been penalized" (red screen at launch) | Server-side — FromSoftware server response | Active 180-day quarantine. Matchmaking restricted to penalized-player pool. |

A player can receive the yellow warning without ever receiving the red penalty — false positive confirmed officially. The escalation path from warning to penalty is community-inferred; FromSoftware has not published the exact threshold.

---

## 2. Penalty Tiers (Community-Inferred)

**[Community]** FromSoftware has not published a formal penalty ladder.

| Step | In-game message | Community description |
| :--- | :--- | :--- |
| **1 — Warning** | "Inappropriate activity detected" (yellow banner) | Save or binary flagged. Still able to play online. May be a false positive. |
| **2 — Softban (180 days)** | "Your account has been penalized" (red screen) | Moved to restricted matchmaking pool with other flagged players. |
| **3 — Repeat softban** | Same red screen | Additional 180-day cycles. See §2.1 for mechanism. |
| **4 — Permanent ban** | Permanent exclusion | Community-reported, usually after repeated violations or malicious acts. |

### 2.1 Why loading the same save causes repeated bans — mechanism clarified

**[RE/verified + Community-technical]** — The 180-day quarantine state is **server-side**, not stored inside the `.sl2` file. Exhaustive reverse-engineering of all major save editors (ClayAmore/ER-Save-Editor, Ariescyn/EldenRing-Save-Manager, Jeius/er-save-manager) found no "ban flag" field in the save format. The `unk0x*` fields in UserData10 remain undocumented in all public RE sources.

The reason loading the same save after 180 days triggers a new ban: the **flagged content** (illegal item IDs, impossible stat sums, etc.) is still present in the save. At the next online sync, the server detects the same violation and issues a new quarantine.

Safe recovery: restore a clean backup predating the flagged edits.

### 2.2 "Tuesday ban waves" — Unverified

**[Unverified rumor]** — Bans are processed in **waves**, not in real time (confirmed by 5-month delay case). A specific day-of-week pattern has not been substantiated. The "every Tuesday" claim has no confirmed primary source.

---

## 3. Known Ban Triggers

### 3.1 Illegal Items — `disableMultiDropShare` mechanism

**[Official + RE/verified]** — The primary mechanism for flagging items as illegal for multiplayer sharing is the `disableMultiDropShare` bit field, present in all equipment param tables in `regulation.bin`.

**Verified from `tmp/regulation-bin-dump/` (our own regulation.bin dump):**

| Param table | Field | Japanese display name | Items flagged in current regulation.bin |
| :--- | :--- | :--- | :--- |
| `EquipParamWeapon` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **1** (ID 24590000 — isDrop=1, isDiscard=1) |
| `EquipParamGoods` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **306** (mostly key items/runes with isDrop=0; 7 with isDrop=1) |
| `EquipParamProtector` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |
| `EquipParamAccessory` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |
| `EquipParamGem` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |

Field translation: `マルチドロップ共有禁止か` = "Is multi-drop sharing prohibited?"

Note: `EquipParamGoods` also contains `u8 isUseMultiPenaltyOnly:1` (Japanese: `クライアント切断ペナルティが発生しているときのみ使用可能` = "Item usable only when client disconnect penalty is active"). **No items** currently have this set in regulation.bin, suggesting it is reserved functionality.

**[Official]** — **Deathbed Smalls** (April 2022): Cut content underwear item distributed via in-game drops. The `disableMultiDropShare` flag was not set for this item before patch 1.04 (or was in a state allowing distribution). FromSoftware patched the drop mechanism in **Patch 1.04**:
> *"Fixed a bug that allowed unauthorized items to be passed to other players."*

Deathbed Smalls is cut content — it does not appear in current `EquipParamProtector` at all (no row with `disableMultiDropShare=1` for protectors in the current dump). The item was never in the retail item tables, so its ID fails the server's item legitimacy check regardless of any flag.

> Source: [Elden Ring Patch Notes 1.04 — Bandai Namco Europe (Official)](https://en.bandainamcoent.eu/elden-ring/news/elden-ring-patch-notes-104)

**Sources**:
- [Automaton Media: Deathbed Smalls incident](https://automaton-media.com/en/news/20220416-11617/)
- [Comicbook.com: Deathbed Smalls ban coverage](https://comicbook.com/gaming/news/elden-ring-players-banned-underwear-deathbed-smalls-cut-from-the-game/)
- [Steam: Deathbed Smalls ban wave thread](https://steamcommunity.com/app/1245620/discussions/0/3278065083958673810/)

### 3.2 Impossible Stat Values — Soul Memory check

**[Community-technical]** — The server detects stat combinations that cannot be achieved through normal gameplay. Based on analysis of FromSoftware's prior games (Dark Souls 3 uses the same studio), the server likely implements a **Soul Memory check**: the sum of `current_runes + total_runes_spent` must equal `total_runes_earned`. Directly editing `current_runes` without updating `total_runes_earned` creates a detectable mismatch. This is the reason adding rune consumables and spending them in-game is reported as safer — it updates all three counters normally.

> Source: [FearLess Cheat Engine forum, "InfiniteWant" (DS3/ER anti-cheat analysis)](https://fearlessrevolution.com/viewtopic.php?t=19320)

| Edit type | Risk | Notes |
| :--- | :--- | :--- |
| **Direct `souls` field write** | High | Creates Soul Memory mismatch (current+spent ≠ earned). |
| **Any attribute above 99** | High | Hard cap in vanilla. Impossible legitimately. |
| **Character level above 713** | N/A | Level 713 = all 8 attributes at 99 = actual maximum. |

**Sources**:
- [Steam: Can I get banned for cheating levels?](https://steamcommunity.com/app/1245620/discussions/0/4526764179303674425/?l=english)
- [Fextralife: Stats page (max values)](https://eldenring.wiki.fextralife.com/Stats)

### 3.3 Boss Items Without Kill Flags

**[Community-technical]** — Adding boss-drop items (remembrances, boss weapons) to inventory without the corresponding boss kill event flags being set is a confirmed ban trigger. The server knows the player's event flag state, and boss loot without the kill flag is a detectable inconsistency. This was documented in controlled tests on FearLess CE forum.

> Source: [FearLess CE forum, "duducasarotto" controlled test](https://fearlessrevolution.com/viewtopic.php?t=19320)

### 3.4 Upgrade-Level Violations

**[Community-plausible, unconfirmed as explicit rule]** — Weapons upgraded beyond the vanilla maximum represent values impossible to achieve legitimately. Detection mechanism same as §3.1: values outside the achievable range.

Vanilla upgrade caps:
- Standard weapons: +0 to +25 (Smithing Stones 1-8 + Ancient Dragon Smithing Stone)
- Special/Somber: +0 to +10 (Somber Smithing Stones 1-9 + Ancient Dragon Somber Smithing Stone)

### 3.5 Save Rollback

**[Community-verified]** — Restoring a save to an earlier state is detectable. The server tracks player state progression. Confirmed case: Bandai Namco Support acknowledged a ban caused by restoring a save that was months old. Delay between violation and penalty: approximately 5 months.

> Source: [Steam: Prohibited Activity PSA](https://steamcommunity.com/app/1245620/discussions/0/3820780968128167841/)

### 3.6 SteamID Mismatch

**[Community-verified]** — Each `.sl2` file is tied to a Steam ID. Loading another player's save under your own account creates a detectable mismatch between the SteamID in the save and the authenticated account.

### 3.7 Custom Gesture / Animation Mods

**[Community-verified, April 2024]** — A cosmetic gesture mod replacing retail animations with non-retail content triggered a penalty warning. Indicates the server validates animation/gesture IDs in addition to item IDs.

### 3.8 NetworkParam Edits (Disconnect Penalty System)

**[RE/verified]** — `NetworkParam` in `regulation.bin` contains a **session disconnect penalty scoring system**, distinct from the save-file ban system. Verified values from our regulation.bin dump:

| Field | Value | Meaning |
| :--- | :--- | :--- |
| `penaltyPointLanDisconnect` | 10 | Points added for LAN disconnect |
| `penaltyPointSignout` | 0 | Points added for sign-out |
| `penaltyPointReboot` | 10 | Points added for power-off |
| `penaltyPointBeginPenalize` | 100 | Score threshold to activate penalty |
| `penaltyForgiveItemLimitTime` | 36000.0 sec | Time limit for Seedbed Curse forgiveness |

This system penalizes DC-quitting in multiplayer sessions. It is **separate** from the save-file content ban system.

**Risk of editing NetworkParam**: EAC validates `regulation.bin` on disk before launch (with EAC active, a modified file would be caught). With EAC bypassed offline, modified `NetworkParam` loads; whether changed values cause server-side anomalies at online sync is unconfirmed — no public report of a ban solely from NetworkParam edits exists. Ban-risk icons in the editor (spec/32) on NetworkParam fields reflect theoretical risk, not confirmed cases.

---

## 4. regulation.bin debug rows

**[RE/verified]** — All equip param tables contain a `u8 disableParam_NT` field. Rows with this set to `1` are debug/internal placeholders (e.g. weapon IDs 1000, 1100, 1200–1260, 1400). Adding items with these IDs to a character save would be detected as non-retail content.

---

## 5. Network Protocol RE

**[RE]** — The matchmaking protocol has been partially reverse-engineered by the community. Key findings:
- Protocol uses **NaCl key exchange** (libsodium KX) with fixed client/server keypairs hard-coded in the game binary.
- The RE covers: bloodstains, ghosts, player messages, summon signs, invasions, quickmatches, group passwords.
- **No documented endpoint for penalty/quarantine checking.** The mechanism by which the server issues or checks penalty state has not been publicly reverse-engineered.

> Source: [waygate-server — open-source ER matchmaking server (Rust)](https://github.com/vswarte/waygate-server)

---

## 6. Risk Assessment Table

| Category | Example — High risk | Example — Lower risk |
| :--- | :--- | :--- |
| **Items** | Cut content / debug items (no retail param row) | Legitimate items with retail IDs |
| **Stats** | Direct `souls` field write (Soul Memory mismatch); any attribute >99 | Adding rune consumables and spending in-game |
| **Equipment** | Weapon above +25/+10 cap | Any weapon within standard upgrade caps |
| **Event flags** | Boss loot without corresponding kill flags | — |
| **Save state** | Restoring backup from weeks/months prior | Restoring backup from same session |
| **Level** | N/A (713 = hard maximum) | Adding consumable runes to reach high levels |
| **Identity** | Loading another player's `.sl2` | — |

---

## 7. Safe-Editing Practices

These are community-reported mitigations, not guaranteed protections.

1. **Offline only during edits**: Disconnect from the internet or use the Anti-Cheat Toggler before opening a modified save. Reconnect only after verifying the state is clean.
2. **Keep a clean backup**: Maintain a backup before any risky edit. If "Inappropriate activity detected" appears, first try "Verify integrity of game files" in Steam (may be a false positive). If the warning persists — restore the clean backup before the next online sync.
3. **Prefer indirect edits**: Add rune consumables and spend them in-game rather than writing the `souls` field directly. The server sees a normal Soul Memory progression.
4. **Use legitimate item IDs only**: Never add items with IDs that have no retail param row. The editor's `cut_content` / `ban_risk` flags in `backend/db/db.go` identify known-risky items.
5. **Do not add boss loot without kill flags**: The server checks event flag consistency. Add boss items only after the kill flag is set.
6. **Do not load flagged content after ban expires**: The quarantine is server-side, but the flagged content is still in the save. Load a clean backup to prevent re-triggering on next sync.
7. **Do not load another player's save**: SteamID mismatch is a confirmed trigger.

---

## 8. PS4 Platform

**[Community-verified, no official confirmation]**

- PS4 does **not** use EAC — it is PC-only technology.
- Sony does not implement game-level anti-cheat on PlayStation.
- FromSoftware's servers handle all platforms — server-side validation applies to saves from any platform.
- **No public RE** confirms a ban flag stored locally in `memory.dat`. Given that the ban state is server-side on PC (not in `.sl2`), the same architecture almost certainly applies to PS4.
- **No confirmed PS4-specific ban cases** with technical documentation in public sources.

---

## 9. Relationship to This Editor

| This doc | spec/32 implementation |
| :--- | :--- |
| §3.1 — Illegal items / `disableMultiDropShare` | `cut_content`, `ban_risk` item flags + `RiskBadge` |
| §3.2 — Soul Memory mismatch / stat >99 | `stat_above_99` risk key + `RiskInfoIcon` on attribute inputs |
| §3.2 — Direct stat edit | `derived_stat_manual` risk key |
| §3.4 — Upgrade cap | Covered by `quantity_above_max` + item flags |
| §3.8 — NetworkParam edits | Tier 1 ban-risk labels on Networking tab fields |
| §4 — debug rows | `cut_content` flag on items with debug-range IDs |

---

## Sources

| Source | URL | Type |
| :--- | :--- | :--- |
| Elden Ring Patch Notes 1.04 | https://en.bandainamcoent.eu/elden-ring/news/elden-ring-patch-notes-104 | **Official** |
| ELDENRING on X — false positive acknowledgment | https://x.com/ELDENRING/status/1806689176449855497 | **Official** |
| Elden Ring EULA (Steam) | https://store.steampowered.com/eula/1245620_eula_0 | **Official** |
| regulation.bin dump (local) | `tmp/regulation-bin-dump/defs/` + `csv/` | **RE/verified** |
| waygate-server — ER protocol RE | https://github.com/vswarte/waygate-server | RE |
| SoulsSpeedruns — EAC Bypass | https://soulsspeedruns.com/eldenring/eac-bypass | Community-technical |
| FearLess CE — controlled ban tests | https://fearlessrevolution.com/viewtopic.php?t=19320 | Community-technical |
| Steam: Prohibited Activity PSA | https://steamcommunity.com/app/1245620/discussions/0/3820780968128167841/ | Community |
| Steam: Ban again after 180 days | https://steamcommunity.com/app/1245620/discussions/0/6679473478679984703/ | Community |
| Steam: 180 days ban countdown | https://steamcommunity.com/app/1245620/discussions/0/3758850762509707736/ | Community |
| Steam: 180 Day ban after coming back | https://steamcommunity.com/app/1245620/discussions/0/4343239957176299084/ | Community |
| Steam: Cheating levels thread | https://steamcommunity.com/app/1245620/discussions/0/4526764179303674425/?l=english | Community |
| Steam: Deathbed Smalls ban wave | https://steamcommunity.com/app/1245620/discussions/0/3278065083958673810/ | Community |
| Automaton Media: Deathbed Smalls incident | https://automaton-media.com/en/news/20220416-11617/ | Press (primary) |
| PCGamesN: Inappropriate activity fix | https://www.pcgamesn.com/elden-ring/inappropriate-activity-detected-fix | Press |
| SVG: Illegal Item Warnings | https://www.svg.com/961328/elden-rings-new-illegal-item-warnings-explained/ | Press |
| Comicbook.com: Deathbed Smalls | https://comicbook.com/gaming/news/elden-ring-players-banned-underwear-deathbed-smalls-cut-from-the-game/ | Press |
| Nexus Mods: Anti-Cheat Toggler | https://www.nexusmods.com/eldenring/mods/90 | Tool |
| GitHub: EldenRingEacToggler | https://github.com/techiew/EldenRingEacToggler | Tool (open source) |
| Nexus Mods: 713 max level save | https://www.nexusmods.com/eldenring/mods/4056 | Community |
| Fextralife: Stats | https://eldenring.wiki.fextralife.com/Stats | Wiki |
