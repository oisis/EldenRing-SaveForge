# 45 — Ban Risk Documentation

> **Type**: Reference document
> **Status**: ✅ Current (last verification 2026-05)
> **Scope**: Community-reported ban triggers, penalty levels, and safe-editing rules for the online mode in Elden Ring. The basis for the risk tier system described in spec/32.

**Important disclaimer**: FromSoftware and Bandai Namco have not published the exact detection rules. Sections marked **[Official]** come from official primary sources. **[RE/verified]** sections were confirmed from our own game data files (the regulation.bin dump in `tmp/regulation-bin-dump/`). **[Community-technical]** sections are technically credible community RE reports. **[Community]** sections are unverified player reports.

---

## 1. Detection system architecture

### 1.1 Easy Anti-Cheat (EAC)

**[Community-technical]** — Runs locally in **user-mode** (not kernel-level; Elden Ring uses user-mode EAC, unlike Nightreign 2025, which uses kernel-level EAC). On game start EAC:

1. Detects processes hooking or manipulating game memory (e.g., Cheat Engine running alongside the game).
2. Verifies the integrity of the game binaries on disk before launch.
3. Scans memory regions reserved by the game process at runtime.

EAC does **not** read the `.sl2` file — it verifies only the live game process and the binary on disk.

**[Official]** — In June 2024 FromSoftware officially acknowledged that "Inappropriate activity detected" can appear as a **false positive** without any cheat, caused by corrupted game files. The official recommendation was to use "Verify integrity of game files" in Steam.
> Source: [ELDENRING on X, June 2024](https://x.com/ELDENRING/status/1806689176449855497)

**EAC bypass methods** — documented on the SoulsSpeedruns Wiki:
- **The `steam_appid.txt` method** (does not block online): create `steam_appid.txt` with the value `1245620` in the `Game/` directory, run `eldenring.exe` directly. EAC does not load; online is still available through Steam.
- **The `start_protected_game.exe` rename method** (blocks online): rename `start_protected_game.exe` + replace it with a copy of `eldenring.exe`. Online disabled until restored.
> Source: [SoulsSpeedruns — EAC Bypass](https://soulsspeedruns.com/eldenring/eac-bypass)

Bypassing EAC disables only the local scan — it does **not** bypass the server-side save file validation.

### 1.2 Server-side save file validation

**[Community-technical]** — Runs on FromSoftware's servers during online synchronization. On an online connection the server loads the uploaded character state and checks it against what can be achieved in legitimate gameplay. Detection is triggered by:

- Item IDs that do not exist in the retail item tables.
- Stat combinations outside the achievable range (see §3.2 — Soul Memory check).
- Inventory quantities exceeding the vanilla maxima.
- Quest/world flag states inconsistent with the flags required as a prerequisite.
- Items from boss drops without the corresponding boss-kill flags.

Bans for save file editing come from here, not from EAC. A player can play offline with a modified save while EAC is active — the penalty triggers only at the next online synchronization. A confirmed case: ~5 months of delay between the violation and the penalty.

**[Community-technical]** — The server does **not** validate the client's `regulation.bin` hash. EAC verifies the binary file on disk before launch; the server checks the resulting character state on connection. Editing `regulation.bin` values (e.g., `NetworkParam`) is not detected by a server-side hash comparison, because FromSoftware does not implement one — EAC catches a modified client-side file before connection (when EAC is active). With EAC disabled offline, a modified `regulation.bin` loads normally; whether changed NetworkParam values cause detectable character-state anomalies on online synchronization — unconfirmed.
> Source: FearlessRevolution community RE consensus; [waygate-server RE project](https://github.com/vswarte/waygate-server)

**[Official]** — The text of the in-game ban message:
> *"Unauthorized tampering with the game detected. A quarantine penalty of 180 days will be imposed."*

### 1.3 "Inappropriate activity detected" vs "Your account has been penalized" — two separate systems

These are definitively **two separate messages** from two separate systems:

| Message | System | Meaning |
| :--- | :--- | :--- |
| "Inappropriate activity detected" (yellow banner on start) | Local — EAC or binary check | The save or binary is flagged as suspicious, OR a false positive (corrupted files). Playing online is still possible. |
| "Your account has been penalized" (red screen on start) | Server-side — FromSoftware server response | An active 180-day quarantine. Matchmaking restricted to the pool of penalized players. |

---

## 2. Penalty levels (inferred by the community)

**[Community]** FromSoftware has not published a formal penalty ladder.

| Step | In-game message | Community description |
| :--- | :--- | :--- |
| **1 — Warning** | "Inappropriate activity detected" (yellow banner) | The save is flagged. Playing online is still possible. May be a false positive. |
| **2 — Softban (180 days)** | "Your account has been penalized" (red screen) | Moved to a restricted matchmaking pool. |
| **3 — Another softban** | The same red screen | Further 180-day cycles. See §2.1. |
| **4 — Permanent ban** | Permanent exclusion | Community-reported, usually after repeated violations or malicious actions. |

### 2.1 Why loading the same save causes further bans — the mechanism explained

**[RE/verified + Community-technical]** — The 180-day quarantine state is **server-side**, not in the `.sl2` file. An exhaustive RE of all known save editors found no "ban flag" field in the file format. The `unk0x*` fields in UserData10 remain undocumented across all public RE sources.

The reason for further bans after the same save: the **flagged content** (illegal IDs, impossible stat sums, etc.) is still in the save. At the next synchronization the server detects the same violation and issues a new quarantine.

The safe recovery path: restore a clean backup from before the flagged edits.

### 2.2 "Ban Tuesdays" — Unconfirmed

**[Unverified rumor]** — Bans are processed in waves, not in real time (a confirmed case of a 5-month delay). A specific day-of-the-week pattern has no confirmed source.

---

## 3. Known ban triggers

### 3.1 Illegal items — the `disableMultiDropShare` mechanism

**[Official + RE/verified]** — The main mechanism for flagging items as illegal to share in multiplayer mode is the bit field `disableMultiDropShare`, present in all equipment param tables in `regulation.bin`.

**Verified from `tmp/regulation-bin-dump/` (our own regulation.bin dump):**

| Param table | Field | Japanese name | Items flagged in the current regulation.bin |
| :--- | :--- | :--- | :--- |
| `EquipParamWeapon` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **1** (ID 24590000 — isDrop=1, isDiscard=1) |
| `EquipParamGoods` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **306** (mostly key items/runes with isDrop=0; 7 with isDrop=1) |
| `EquipParamProtector` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |
| `EquipParamAccessory` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |
| `EquipParamGem` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |

Translation: `マルチドロップ共有禁止か` = "Is multi-drop sharing forbidden?"

`EquipParamGoods` also contains `u8 isUseMultiPenaltyOnly:1` (Japanese: `クライアント切断ペナルティが発生しているときのみ使用可能` = "Item available only when a client disconnect penalty is active"). **No item** has this flag set in the current regulation.bin.

**[Official]** — **Deathbed Smalls** (April 2022): cut-content underwear distributed via a drop. Patch 1.04:
> *"Fixed a bug that allowed unauthorized items to be passed to other players."*

Deathbed Smalls is cut content — it does not appear in the current `EquipParamProtector` (no row). The item ID does not pass the server-side item legality check regardless of the flags.

> Source: [Elden Ring Patch Notes 1.04 — Bandai Namco Europe (Official)](https://en.bandainamcoent.eu/elden-ring/news/elden-ring-patch-notes-104)

### 3.2 Impossible stat values — Soul Memory check

**[Community-technical]** — The server detects stat combinations impossible to achieve through normal play. Based on the analysis of previous FromSoftware games (Dark Souls 3 uses the same mechanism), the server likely implements a **Soul Memory check**: the sum `current_runes + total_runes_spent` must equal `total_runes_earned`. Editing `current_runes` directly without updating `total_runes_earned` creates a detectable discrepancy. That is why adding rune consumables and spending them in-game is safer — it updates all three counters normally.

> Source: [FearLess Cheat Engine forum, "InfiniteWant" (DS3/ER anti-cheat analysis)](https://fearlessrevolution.com/viewtopic.php?t=19320)

| Edit type | Risk | Notes |
| :--- | :--- | :--- |
| **Direct write of the `souls` field** | High | Creates a Soul Memory discrepancy. |
| **Any attribute above 99** | High | A hard cap in the vanilla game. |
| **Character level above 713** | N/A | Level 713 = all 8 attributes at 99 = the actual maximum. |

### 3.3 Boss items without kill flags

**[Community-technical]** — Adding boss drops (remembrances, boss weapons) to the inventory without the corresponding boss-kill flags is a confirmed ban trigger. The server knows the player's event flag state; boss loot without a kill flag is a detectable inconsistency.

> Source: [FearLess CE forum, controlled test "duducasarotto"](https://fearlessrevolution.com/viewtopic.php?t=19320)

### 3.4 Weapon upgrade level violations

**[Community — plausible, unconfirmed as a separate rule]** — A weapon upgraded above the vanilla maximum represents values impossible to achieve legitimately.

Upgrade caps:
- Standard weapons: +0 to +25
- Special/Somber: +0 to +10

### 3.5 Save file rollback

**[Community-verified]** — Restoring a save to an earlier state is detectable. A confirmed case: Bandai Namco Support acknowledged a ban for restoring a save from several months ago. Delay ~5 months.

> Source: [Steam: Prohibited Activity PSA](https://steamcommunity.com/app/1245620/discussions/0/3820780968128167841/)

### 3.6 SteamID mismatch

**[Community-verified]** — Each `.sl2` is tied to a Steam ID. Loading someone else's save under your own account = a detectable mismatch.

### 3.7 Custom gesture/animation mods

**[Community-verified, April 2024]** — A cosmetic gesture mod replacing animations with content outside the retail set caused a penalty warning. This indicates that the server validates gesture/animation IDs in addition to item IDs.

### 3.8 NetworkParam editing (disconnect penalty system)

**[RE/verified]** — `NetworkParam` in `regulation.bin` contains a **session disconnect penalty scoring system**, separate from the save file banning system. Values verified from our regulation.bin dump:

| Field | Value | Meaning |
| :--- | :--- | :--- |
| `penaltyPointLanDisconnect` | 10 | Points for a LAN disconnect |
| `penaltyPointSignout` | 0 | Points for a sign-out |
| `penaltyPointReboot` | 10 | Points for a reboot/shutdown |
| `penaltyPointBeginPenalize` | 100 | Penalty activation threshold |
| `penaltyForgiveItemLimitTime` | 36000.0 sec | Forgiveness time for "Seedbed Curse" |

The system penalizes DC-quitting in multiplayer sessions. It is **separate** from the save file content banning system.

**Risk of editing NetworkParam**: EAC verifies `regulation.bin` on disk before launch (with EAC active, a modified file would be detected). With EAC disabled offline, a modified `NetworkParam` loads; whether the changed values cause server-side anomalies on online sync — unconfirmed. There is no public report of a ban solely from NetworkParam editing.

---

## 4. Debug rows in regulation.bin

**[RE/verified]** — All equipment param tables contain a `u8 disableParam_NT` field. Rows with this flag set to `1` are debug/internal placeholders (e.g., weapon IDs 1000, 1100, 1200–1260, 1400). Adding items with these IDs to a save would be detected as content outside the retail set.

---

## 5. Network protocol RE

**[RE]** — The matchmaking protocol has been partially reverse-engineered by the community:
- The protocol uses **NaCl key exchange** (libsodium KX) with hardcoded keypairs in the game binary.
- The RE covers: blood stains, ghosts, messages, summon signs, invasions, quickmatches, group passwords.
- **No documented penalty/quarantine endpoint.** The mechanism for the server checking a penalty has not been publicly reverse-engineered.

> Source: [waygate-server — open-source ER matchmaking server (Rust)](https://github.com/vswarte/waygate-server)

---

## 6. Risk assessment table

| Category | Example — High risk | Example — Lower risk |
| :--- | :--- | :--- |
| **Items** | Cut content / debug items (no row in retail param) | Legitimate items with retail IDs |
| **Stats** | Direct write of the `souls` field (Soul Memory discrepancy); attribute >99 | Adding rune consumables and spending in-game |
| **Equipment** | Weapon above the cap +25/+10 | Any weapon within the standard limits |
| **Event flags** | Boss loot without kill flags | — |
| **Save state** | Restoring a backup from weeks/months ago | Restoring a backup from the same session |
| **Level** | N/A (713 = hard maximum) | Adding rune consumables |
| **Identity** | Someone else's `.sl2` under your own account | — |

---

## 7. Safe-editing rules

1. **Offline only while editing**: Disconnect or use the Anti-Cheat Toggler before opening a modified save.
2. **Keep a clean backup**: On "Inappropriate activity detected", first "Verify integrity of game files" (may be a false positive). If that does not help — restore a backup before the next synchronization.
3. **Prefer indirect edits**: Add rune consumables and spend them in-game instead of writing directly to the `souls` field.
4. **Use only legitimate IDs**: The `cut_content` / `ban_risk` flags in `backend/db/db.go` identify known risky items.
5. **Do not add boss loot without kill flags**: The server checks event flag consistency.
6. **Do not load flagged content after a ban**: Load a clean backup — the quarantine is server-side, but the content is still in the file.
7. **Do not load someone else's save**: A SteamID mismatch is a confirmed trigger.

---

## 8. PS4 platform

**[Community-verified, no official confirmation]**

- PS4 does not use EAC — a PC-only technology.
- Sony does not implement its own game-level anti-cheat.
- FromSoftware's servers serve all platforms — server-side validation applies to saves from every platform.
- **No RE** confirming a ban flag in `memory.dat`. Since the penalty state is server-side on PC, the same architecture almost certainly applies on PS4.
- No confirmed PS4-specific ban cases with technical documentation.

---

## 9. Relation to this editor

| This document | spec/32 implementation |
| :--- | :--- |
| §3.1 — Illegal items / `disableMultiDropShare` | `cut_content`, `ban_risk` flags + `RiskBadge` |
| §3.2 — Soul Memory / attribute >99 | Risk key `stat_above_99` + `RiskInfoIcon` on attributes |
| §3.2 — Direct stat editing | Risk key `derived_stat_manual` |
| §3.4 — Upgrade cap | `quantity_above_max` + item flags |
| §3.8 — NetworkParam editing | Tier 1 ban-risk labels in the Networking tab |
| §4 — Debug rows | The `cut_content` flag on items with a debug ID range |

---

## Sources

| Source | URL | Type |
| :--- | :--- | :--- |
| Elden Ring Patch Notes 1.04 | https://en.bandainamcoent.eu/elden-ring/news/elden-ring-patch-notes-104 | **Official** |
| ELDENRING on X — false positive | https://x.com/ELDENRING/status/1806689176449855497 | **Official** |
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
