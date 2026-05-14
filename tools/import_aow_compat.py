#!/usr/bin/env python3
"""
import_aow_compat.py

Generates Go data files from ER regulation.bin dump:

  backend/db/data/aow_compat.go        — AoW canMountWep 40-bit bitmask + WepTypeToCanMountBit
  backend/db/data/weapon_gem_mount.go  — weapon base ID → WepType + GemMountType
  backend/db/data/aow_dlc_compat.go    — DLC fallback via mountWepTextId + swordArtsParamId
  backend/db/data/aow_affinity.go      — AoW configurable affinity bitmask + default
  backend/db/data/weapon_extras.go     — weapon default skill + reinforce type

Source CSVs:
  tmp/regulation-bin-dump/csv/EquipParamWeapon.csv
  tmp/regulation-bin-dump/csv/EquipParamGem.csv

Compatibility model (Phase 1.6):

  Layer 1+2: 40-bit canMountWep mask.
    - bits 0..35 from EquipParamGem.canMountWep_Dagger..canMountWep_Torch
    - bits 36..39 from EquipParamGem.reserved_canMountWep (4-bit DLC field)
    Empirical bit ↔ wepType (Phase 1.5/1.6 verified via swordArtsParamId cross-ref):
      bit 36 (reserved bit 0) → wepType 88 (DLC Hand-to-Hand / Dryleaf Arts)
      bit 37 (reserved bit 1) → (Perfume Bottles — wepType missing in current dump)
      bit 38 (reserved bit 2) → wepType 69, 90 (DLC shield-like)
      bit 39 (reserved bit 3) → wepType 91 (DLC Throwing Blades)

  Layer 3: mountWepTextId + swordArtsParamId fallback for DLC weapon classes whose
    AoWs have zero std/reserved bits but share an mwtid group whose default-skill SAP
    matches a weapon's swordArtsParamId. Empirically maps:
      mwtid 63055 → wepType 92 (Backhand Blades)
      mwtid 63056 → wepType 93 (Great Katanas)
      mwtid 63057 → wepType 94 (Light Greatswords)
      mwtid 63058 → wepType 95 (Beast Claws)
    Plus 63051 (88), 63054 (91) redundantly cover Layer 1+2 cases.

Reserved2_canMountWep is currently 0 across all 242 gem rows — unused; ignored.

NOTE: disableParam_NT=1 occurs on many vanilla mountable AoWs (Hoarfrost Stomp,
Sword Dance, Bloody Slash, Golden Vow, Parry, etc.). It is NOT a mountable filter
and must not be used to exclude AoW or weapon rows from generation.
"""

import csv
import sys
import os
from collections import defaultdict
from pathlib import Path
from typing import Dict, List, Set, Tuple


def find_repo_root() -> Path:
    """Locate the repository root by walking up from this script until go.mod
    is found. Robust to the script being moved (tmp/scripts/ vs tools/)."""
    here = Path(__file__).resolve()
    for candidate in [here.parent] + list(here.parents):
        if (candidate / 'go.mod').exists():
            return candidate
    raise SystemExit(f"Cannot find go.mod above {here}")

# Phase 1.5/1.6 empirical reserved-bit → wepType mapping.
# Each reserved bit position (0..3) maps to a list of wepTypes that should be
# considered compatible when that bit is set in EquipParamGem.reserved_canMountWep.
RESERVED_BIT_TO_WEPTYPES: Dict[int, List[int]] = {
    0: [88],       # Hand-to-Hand DLC (verified via Palm Blast SAP 4010 → 28 base weapons)
    1: [],         # Perfume Bottles — no weapon with matching SAP in current dump
    2: [69, 90],   # Shield-like DLC (verified via Shield Bash/Strike SAP)
    3: [91],       # Throwing Blades DLC (verified via Piercing Throw SAP 4020)
}

# Vanilla wepType → canMountWep bit mapping (empirically derived; bits 0..35).
# Phase 1.6: removed incorrect DLC mappings (87..93 → bow bits) and added
# verified DLC mappings (88, 69, 90, 91 → reserved bits 36..39).
# Unverified DLC wepTypes (87, 89, 92, 93, 94, 95) are intentionally absent here:
# they are either covered by Layer 3 (92, 93, 94, 95 via mwtid+SAP) or fail-closed
# (87, 89) until manual in-game verification.
WEP_TYPE_TO_BIT: Dict[int, int] = {
    # Phase 2B audit (every active wepType cross-referenced with the named weapons
    # in backend/db/data/melee_armaments.go, shields.go, ranged_and_catalysts.go).
    # The canonical bit name (canMountWep_*) matches the English class name of the
    # weapons living in that wepType bucket.

    # Straight / curved / piercing swords
    1:  0,   # Dagger              (Dagger, Misericorde, Parrying Dagger, ...)
    3:  1,   # Straight Sword      (Longsword, Short Sword, ...)
    5:  2,   # Greatsword          (Bastard Sword, Lizard Greatsword, ...)
    7:  3,   # Colossal Sword      (Watchdog's Greatsword, Maliketh's Black Blade, ...)
    9:  4,   # Curved Sword        (Falchion, Scimitar, Shotel, ...)               — was 8 ❌
    11: 5,   # Curved Greatsword   (Dismounter, Omen Cleaver, ...)                 — was 9 ❌
    13: 6,   # Katana              (Uchigatana, Nagakiba — verified by Unsheathe single-bit AoW)
    14: 7,   # Twinblade           (Twinblade, Godskin Peeler, ...)                — was 5 ❌
    15: 8,   # Thrusting Sword     (Estoc, Rapier, Cleanrot Knight's Sword, ...)   — was 4 ❌
    16: 9,   # Heavy Thrusting Sword (Sword Lance, Godskin Stitcher, ...)          — was 7 ❌

    # Axes / hammers / flails
    17: 10,  # Axe                 (Battle Axe, Hand Axe, ...)                    — was 7 ❌
    19: 11,  # Greataxe            (Greataxe, Great Omenkiller Cleaver, ...)
    21: 12,  # Hammer              (Mace, Club, ...)                              — was 13 ❌
    23: 13,  # Great Hammer        (Large Club, Greathorn Hammer, Battle Hammer, ...) — was 10 ❌
    24: 14,  # Flail               (Nightrider Flail, Family Heads, ...)          — was 10 ❌

    # Spears / halberds / reapers
    25: 15,  # Spear               (Spear, Short Spear, Bolt of Gransax, ...)     — was 12 ❌
    28: 16,  # Great Spear         (Lance, Bloody Lance, Serpent-Hunter, ...)     — was 14 ❌
    29: 18,  # Halberd             (Halberd, Lucerne, Pest's Glaive, ...)         — was 14 ❌
    31: 19,  # Reaper / Sickle     (Scythe, Halo Scythe, Grave Scythe, ...)       — was 15 ❌

    # Fists / claws / whips / colossal weapons
    35: 20,  # Fist                (Caestus, Iron Ball, Cipher Pata, Grafted Dragon, ...) — was 16 ❌
    37: 21,  # Claw                (Hookclaws, Venomous Fang, Bloodhound Claws, ...) — was 19 ❌
    39: 22,  # Whip                (Whip, Thorned Whip, Hoslow's Petal Whip, ...) — was 20 ❌
    41: 23,  # Colossal Weapon     (Bloodfiend's Arm, Anvil Hammer, Putrescence Cleaver, ...) — was 21 ❌
            #   bit name in regulation is canMountWep_AxhammerLarge (= Colossal Weapon class).

    # Bows / crossbows / ballista (Phase 2A — kept)
    50: 24,  # Light Bow           (Shortbow, Composite Bow, ...)
    51: 25,  # Bow                 (Longbow, Black Bow, Ansbach's Longbow, ...)
    53: 26,  # Greatbow            (Lion Greatbow, Erdtree Greatbow, Igon's Greatbow, ...)
    55: 27,  # Crossbow            (Light/Heavy/Repeating Crossbow, Pulley Crossbow, ...)
    56: 28,  # Ballista            (Hand Ballista)

    # Catalysts
    57: 29,  # Glintstone Staff
    61: 30,  # Sacred Seal         (canonical bit name: canMountWep_Sorcery)

    # Shields
    65: 32,  # Small Shield        (Buckler, Perfumer's Shield, Rickety Shield, ...)
    66: 33,  # Medium Shield
    67: 34,  # Greatshield         (Kite Shield, Beastman's Jar-Shield, Dragon Towershield variants, ...)

    # Torch
    87: 35,  # Torch               (Torch, Steel-Wire Torch, Beast-Repellent Torch, ...) — Phase 2B fix
            #   Pre-Phase 2B mapped wt 68 here, but wt 68 has 0 base weapons in regulation.
            #   wt 87 is the actual Torch wepType (verified via local DB cross-ref).

    # DLC reserved-bit categories (40-bit mask, bits 36..39 = reserved_canMountWep)
    88: 36,  # DLC Hand-to-Hand / Dryleaf Arts (reserved bit 0)
    89: 37,  # DLC Perfume Bottles (reserved bit 1)                                 — Phase 2B fix
            #   Local DB: Firespark/Chilling/Frenzyflame Perfume Bottle.
            #   Wall of Sparks (gem 404000, rsv=2) and Rolling Sparks (405000, rsv=2)
            #   have bit 37 set → now correctly mount on Perfume Bottles.
            #   Closes the "Wall/Rolling Sparks gap" reported in Phase 1.6.
    69: 38,  # DLC shield-like #1 (reserved bit 2 — e.g. Black Steel Greatshield)
    90: 38,  # DLC shield-like #2 (reserved bit 2 — Dueling Shield class)
    91: 39,  # DLC Throwing Blades (reserved bit 3 — Smithscript Dagger, ...)

    # Intentionally NOT mapped — Layer 3 mwtid+SAP fallback covers these:
    #   92 — Backhand Blades   (mwtid 63055)
    #   93 — Great Katanas     (mwtid 63056)
    #   94 — Light Greatswords (mwtid 63057)
    #   95 — Beast Claws       (mwtid 63058)
    #
    # Intentionally NOT mapped — fail-closed:
    #   0  — placeholder / empty wepType
    #   33 — Unarmed (single "no weapon" pseudo-item, no AoW)
    #   43 — dead in pre-Phase-2B (no base weapon in regulation)
    #   68 — dead in pre-Phase-2B (Torch is wt 87)
    #
    # Edge case: wt 33 had 91 disableParam_NT=1 rows (gm=2) in regulation but every one
    # has the same SAP and looks like a duplicate-Longsword cut-content placeholder set.
    # No active weapon for the player exists at this wepType.
}

CANMOUNT_NAMES_36: List[str] = [
    "Dagger", "SwordNormal", "SwordLarge", "SwordGigantic",
    "SaberNormal", "SaberLarge", "katana", "SwordDoubleEdge",
    "SwordPierce", "RapierHeavy", "AxeNormal", "AxeLarge",
    "HammerNormal", "HammerLarge", "Flail", "SpearNormal",
    "SpearLarge", "SpearHeavy", "SpearAxe", "Sickle",
    "Knuckle", "Claw", "Whip", "AxhammerLarge",
    "BowSmall", "BowNormal", "BowLarge", "ClossBow",
    "Ballista", "Staff", "Sorcery", "Talisman",
    "ShieldSmall", "ShieldNormal", "ShieldLarge", "Torch",
]
CANMOUNT_NAMES_40 = CANMOUNT_NAMES_36 + [
    "DLC_HandToHand",   # bit 36 — reserved bit 0 (wepType 88)
    "DLC_PerfumeBottle",# bit 37 — reserved bit 1 (wepType 89; Phase 2B)
    "DLC_ShieldLike",   # bit 38 — reserved bit 2 (wepTypes 69, 90)
    "DLC_ThrowingBlade",# bit 39 — reserved bit 3 (wepType 91)
]


def find_col(header: List[str], name: str) -> int:
    """Return 0-based column index for header `name`, raising if absent."""
    try:
        return header.index(name)
    except ValueError:
        raise SystemExit(f"Column not found in CSV header: {name!r}")


def read_weapons(csv_path: str) -> List[Dict]:
    """Read EquipParamWeapon. Return list of all base rows (rid % 100 == 0).

    Does NOT filter by disableParam_NT (Phase 1.6 — that flag is not a mountable
    filter; cut-content rows are excluded indirectly by absence in local DB)."""
    weapons = []
    with open(csv_path, newline='', encoding='utf-8') as f:
        r = csv.reader(f, delimiter=';')
        header = next(r)
        col_rid = find_col(header, 'Row ID')
        col_wt  = find_col(header, 'wepType')
        col_gm  = find_col(header, 'gemMountType')
        col_sap = find_col(header, 'swordArtsParamId')
        col_rti = find_col(header, 'reinforceTypeId')
        col_dis = find_col(header, 'disableParam_NT')
        for row in r:
            if not row[col_rid] or not row[col_rid].isdigit():
                continue
            rid = int(row[col_rid])
            if rid % 100 != 0:
                continue
            def safe_int(s, default=0):
                try:
                    return int(s) if s and s != '-1' else default
                except ValueError:
                    return default
            weapons.append({
                'rid': rid,
                'wepType': safe_int(row[col_wt]),
                'gm': safe_int(row[col_gm]),
                'sap': safe_int(row[col_sap]),
                'rti': safe_int(row[col_rti]),
                'disabled_nt': row[col_dis] == '1',
            })
    return weapons


def read_gems(csv_path: str) -> List[Dict]:
    """Read EquipParamGem. Return list of all rows with full compat data.

    Does NOT filter by disableParam_NT (Phase 1.6 verified vanilla mountable AoWs
    like Hoarfrost Stomp, Sword Dance carry this flag too)."""
    gems = []
    with open(csv_path, newline='', encoding='utf-8') as f:
        r = csv.reader(f, delimiter=';')
        header = next(r)
        col_rid = find_col(header, 'Row ID')
        col_sap = find_col(header, 'swordArtsParamId')
        col_def_attr = find_col(header, 'defaultWepAttr')
        col_rsv1 = find_col(header, 'reserved_canMountWep')
        col_rsv2 = find_col(header, 'reserved2_canMountWep')
        col_mwtid = find_col(header, 'mountWepTextId')
        canm_cols = [find_col(header, f'canMountWep_{n}') for n in CANMOUNT_NAMES_36]
        cfg_cols = [find_col(header, f'configurableWepAttr{i:02d}') for i in range(24)]
        for row in r:
            if not row[col_rid] or not row[col_rid].isdigit():
                continue
            rid = int(row[col_rid])
            # 40-bit canMountWep mask
            mask = 0
            for i, col in enumerate(canm_cols):
                if row[col] == '1':
                    mask |= (1 << i)
            rsv = int(row[col_rsv1]) if row[col_rsv1] and row[col_rsv1] != '-1' else 0
            for bit_in_rsv in range(4):
                if (rsv >> bit_in_rsv) & 1:
                    mask |= (1 << (36 + bit_in_rsv))
            # affinity
            cfg_mask = 0
            for i, col in enumerate(cfg_cols):
                if row[col] == '1':
                    cfg_mask |= (1 << i)
            def safe_int(s, default=0):
                try:
                    return int(s) if s and s != '-1' else default
                except ValueError:
                    return default
            gems.append({
                'rid': rid,
                'item_id': 0x80000000 | rid,
                'sap': safe_int(row[col_sap]),
                'default_attr': safe_int(row[col_def_attr]),
                'cfg_mask': cfg_mask,
                'mask': mask,
                'rsv': rsv,
                'rsv2': int(row[col_rsv2]) if row[col_rsv2] and row[col_rsv2] != '-1' else 0,
                'mwtid': int(row[col_mwtid]) if row[col_mwtid] and row[col_mwtid] != '-1' else -1,
            })
    return gems


def compute_dlc_fallback(gems: List[Dict], weapons: List[Dict]) -> Tuple[Dict[int, List[int]], Dict[int, List[int]], List[str]]:
    """Layer 3: for each AoW with mwtid != -1, compute the set of wepTypes whose
    weapons have a default swordArtsParamId in the SAP-set of that mwtid group.

    Returns:
      - aow_fallback: {aow_item_id: [wepType, ...]}
      - mwtid_to_wepTypes: {mwtid: [wepType, ...]} (for diagnostics)
      - warnings: human-readable audit warnings
    """
    warnings = []
    # Group gems by mwtid → set of SAPs
    mwtid_saps: Dict[int, Set[int]] = defaultdict(set)
    mwtid_aows: Dict[int, List[int]] = defaultdict(list)  # for warning emission
    for g in gems:
        if g['mwtid'] == -1:
            continue
        if g['sap'] > 0:
            mwtid_saps[g['mwtid']].add(g['sap'])
        mwtid_aows[g['mwtid']].append(g['item_id'])
    # Map SAP → set of wepTypes (from base, gemMountType=2 weapons; SAP must be its DEFAULT skill)
    sap_to_weptypes: Dict[int, Set[int]] = defaultdict(set)
    for w in weapons:
        if w['gm'] != 2:
            continue
        if w['sap'] > 0:
            sap_to_weptypes[w['sap']].add(w['wepType'])
    # mwtid → set of wepTypes via union of sap_to_weptypes for SAPs in group
    mwtid_to_weptypes: Dict[int, Set[int]] = {}
    for mwtid, saps in mwtid_saps.items():
        wts = set()
        for sap in saps:
            wts.update(sap_to_weptypes.get(sap, set()))
        mwtid_to_weptypes[mwtid] = wts
        if not wts:
            warnings.append(
                f"mwtid {mwtid} has {len(mwtid_aows[mwtid])} AoW(s) but no weapon "
                f"with matching default swordArtsParamId in current regulation dump "
                f"(SAPs in group: {sorted(saps)}). AoW item IDs: "
                f"{[f'0x{x:08X}' for x in mwtid_aows[mwtid]]}"
            )
    # Per-AoW fallback list (sorted for stable output)
    aow_fallback: Dict[int, List[int]] = {}
    for g in gems:
        if g['mwtid'] == -1:
            continue
        wts = mwtid_to_weptypes.get(g['mwtid'], set())
        if wts:
            aow_fallback[g['item_id']] = sorted(wts)
    return aow_fallback, {k: sorted(v) for k, v in mwtid_to_weptypes.items()}, warnings


def gen_header() -> str:
    return ("// Code generated by tools/import_aow_compat.py; DO NOT EDIT.\n\n"
            "package data\n\n")


def gen_weapon_gem_mount(weapons: List[Dict]) -> str:
    """Generate WeaponGemMounts map. Includes ALL base rows with gm != 0, including
    disableParam_NT=1 ones. Reason: filtering by disableParam_NT is not safe (Phase 1.6).
    Cut-content rows are naturally not in the local DB so they get ignored at index time."""
    lines = [gen_header()]
    lines.append("// WeaponGemMount holds AoW-mount metadata for a weapon base item ID.\n")
    lines.append("type WeaponGemMount struct {\n")
    lines.append("\tWepType      uint16 // EquipParamWeapon.wepType (weapon category integer)\n")
    lines.append("\tGemMountType uint8  // 0=none, 1=special/somber, 2=standard infusable\n")
    lines.append("}\n\n")
    lines.append("// WeaponGemMounts maps weapon base item ID → AoW mount data.\n")
    lines.append("// Includes only weapons with gemMountType != 0.\n")
    lines.append("// Note: a small number of rows have disableParam_NT=1 (cut-content); these are\n")
    lines.append("// kept here because the filter is unreliable (vanilla mountable AoWs in EquipParamGem\n")
    lines.append("// also carry that flag). They are harmless at lookup time since item IDs absent from\n")
    lines.append("// the gameplay DB never reach the index.\n")
    lines.append("var WeaponGemMounts = map[uint32]WeaponGemMount{\n")
    for w in sorted(weapons, key=lambda x: x['rid']):
        if w['gm'] == 0:
            continue
        lines.append(f"\t0x{w['rid']:08X}: {{WepType: {w['wepType']}, GemMountType: {w['gm']}}},\n")
    lines.append("}\n")
    return ''.join(lines)


def gen_aow_compat(gems: List[Dict]) -> str:
    lines = [gen_header()]
    lines.append("// AoWCompatMasks maps AoW item ID → 40-bit canMountWep bitmask.\n")
    lines.append("//\n")
    lines.append("// Bits 0..35 are vanilla canMountWep_Dagger..canMountWep_Torch (see CanMountWepNames).\n")
    lines.append("// Bits 36..39 are reserved_canMountWep bits 0..3 (DLC weapon classes).\n")
    lines.append("// Empirical bit → wepType mapping (Phase 1.6):\n")
    lines.append("//   bit 36 → wepType 88 (Hand-to-Hand / Dryleaf Arts)\n")
    lines.append("//   bit 37 → (Perfume Bottles — wepType absent in current regulation dump)\n")
    lines.append("//   bit 38 → wepType 69, 90 (DLC shield-like)\n")
    lines.append("//   bit 39 → wepType 91 (Throwing Blades)\n")
    lines.append("//\n")
    lines.append("// AoW item ID = EquipParamGem.rowID | 0x80000000.\n")
    lines.append("var AoWCompatMasks = map[uint32]uint64{\n")
    for g in sorted(gems, key=lambda x: x['rid']):
        if g['mask'] == 0:
            continue
        lines.append(f"\t0x{g['item_id']:08X}: 0x{g['mask']:016X}, // row {g['rid']}\n")
    lines.append("}\n\n")
    lines.append("// WepTypeToCanMountBit maps weapon wepType → bit position in AoWCompatMasks.\n")
    lines.append("// Phase 1.6: DLC wepTypes 87/89/92/93/94/95 are intentionally NOT mapped here.\n")
    lines.append("//   87, 89 — insufficient empirical evidence (small sample, vanilla SAPs).\n")
    lines.append("//   92, 93, 94, 95 — covered via Layer 3 mwtid+SAP fallback in AoWDLCFallbackWepTypes.\n")
    lines.append("var WepTypeToCanMountBit = map[uint16]uint8{\n")
    for wt, bit in sorted(WEP_TYPE_TO_BIT.items()):
        comment = CANMOUNT_NAMES_40[bit] if bit < len(CANMOUNT_NAMES_40) else f"bit{bit}"
        lines.append(f"\t{wt}: {bit}, // canMountWep_{comment}\n")
    lines.append("}\n\n")
    lines.append("// CanMountWepNames lists canMountWep column names in bit order (0..39).\n")
    lines.append("// Bits 36..39 are synthetic names for reserved_canMountWep bits.\n")
    lines.append("var CanMountWepNames = []string{\n")
    for n in CANMOUNT_NAMES_40:
        lines.append(f'\t"{n}",\n')
    lines.append("}\n")
    return ''.join(lines)


def gen_aow_dlc_compat(aow_fallback: Dict[int, List[int]], mwtid_to_weptypes: Dict[int, List[int]]) -> str:
    lines = [gen_header()]
    lines.append("// AoWDLCFallbackWepTypes is the Layer 3 compatibility fallback for DLC Ashes of War\n")
    lines.append("// that are not encoded in the standard or reserved canMountWep bits.\n")
    lines.append("//\n")
    lines.append("// Mechanism (Phase 1.6 empirical, deterministic from local CSVs):\n")
    lines.append("//   1. Group EquipParamGem rows by mountWepTextId (UI tag).\n")
    lines.append("//   2. Each group collects the set of swordArtsParamIds across its gems.\n")
    lines.append("//   3. A weapon base row whose default swordArtsParamId is in the group's SAP-set\n")
    lines.append("//      indicates the group's compatible wepType(s).\n")
    lines.append("//   4. Per-AoW: emit the list of wepTypes derived from its group.\n")
    lines.append("//\n")
    lines.append("// Used by CheckAoWCompatibility after the standard 40-bit canMountWep check fails.\n")
    lines.append("// AoW is mountable on a weapon if its wepType appears in this list.\n")
    lines.append("var AoWDLCFallbackWepTypes = map[uint32][]uint16{\n")
    for aow_id in sorted(aow_fallback):
        wts = ", ".join(str(w) for w in aow_fallback[aow_id])
        lines.append(f"\t0x{aow_id:08X}: {{{wts}}},\n")
    lines.append("}\n\n")
    lines.append("// AoWMwtidGroups is a diagnostic map: mountWepTextId → list of compatible wepTypes\n")
    lines.append("// derived from the matching SAP set. Useful for tests and audit; NOT used by the\n")
    lines.append("// compat algorithm directly (per-AoW AoWDLCFallbackWepTypes is sufficient).\n")
    lines.append("var AoWMwtidGroups = map[int32][]uint16{\n")
    for mwtid in sorted(mwtid_to_weptypes):
        if not mwtid_to_weptypes[mwtid]:
            continue
        wts = ", ".join(str(w) for w in mwtid_to_weptypes[mwtid])
        lines.append(f"\t{mwtid}: {{{wts}}},\n")
    lines.append("}\n")
    return ''.join(lines)


def gen_aow_affinity(gems: List[Dict]) -> str:
    lines = [gen_header()]
    lines.append("// AoWDefaultAffinity maps AoW item ID → default affinity ID (0..23).\n")
    lines.append("// Set automatically when the gem is first mounted. Affinity enum (0..12):\n")
    lines.append("//   0=Standard, 1=Heavy, 2=Keen, 3=Quality, 4=Magic, 5=Fire, 6=Flame Art,\n")
    lines.append("//   7=Lightning, 8=Sacred, 9=Poison, 10=Blood, 11=Cold, 12=Occult.\n")
    lines.append("var AoWDefaultAffinity = map[uint32]uint8{\n")
    for g in sorted(gems, key=lambda x: x['rid']):
        lines.append(f"\t0x{g['item_id']:08X}: {g['default_attr']},\n")
    lines.append("}\n\n")
    lines.append("// AoWAffinityMasks maps AoW item ID → 24-bit mask of allowed affinity IDs.\n")
    lines.append("// Bit N set iff configurableWepAttrNN == 1 in EquipParamGem.\n")
    lines.append("var AoWAffinityMasks = map[uint32]uint32{\n")
    for g in sorted(gems, key=lambda x: x['rid']):
        if g['cfg_mask'] == 0:
            continue
        lines.append(f"\t0x{g['item_id']:08X}: 0x{g['cfg_mask']:08X},\n")
    lines.append("}\n")
    return ''.join(lines)


def gen_weapon_extras(weapons: List[Dict]) -> str:
    lines = [gen_header()]
    lines.append("// WeaponExtras stores additional weapon metadata for AoW reset-on-remove and\n")
    lines.append("// somber-vs-normal upgrade detection.\n")
    lines.append("type WeaponExtras struct {\n")
    lines.append("\tDefaultSwordArtsParamId int32 // EquipParamWeapon.swordArtsParamId — the weapon's innate skill\n")
    lines.append("\tReinforceTypeId         int16 // EquipParamWeapon.reinforceTypeId — upgrade-track ID\n")
    lines.append("}\n\n")
    lines.append("// WeaponExtrasMap maps weapon base item ID → extras (default skill + reinforce track).\n")
    lines.append("// Used to reset to vanilla skill when AoW is removed and to detect somber-upgrade weapons.\n")
    lines.append("var WeaponExtrasMap = map[uint32]WeaponExtras{\n")
    for w in sorted(weapons, key=lambda x: x['rid']):
        if w['gm'] == 0 and w['sap'] == 0 and w['rti'] == 0:
            continue
        lines.append(f"\t0x{w['rid']:08X}: {{DefaultSwordArtsParamId: {w['sap']}, ReinforceTypeId: {w['rti']}}},\n")
    lines.append("}\n")
    return ''.join(lines)


def write_atomic(path: str, content: str):
    tmp = path + '.tmp'
    with open(tmp, 'w') as f:
        f.write(content)
    os.replace(tmp, path)


def main():
    base_dir = find_repo_root()
    weapon_csv = base_dir / 'tmp' / 'regulation-bin-dump' / 'csv' / 'EquipParamWeapon.csv'
    gem_csv = base_dir / 'tmp' / 'regulation-bin-dump' / 'csv' / 'EquipParamGem.csv'
    out_dir = base_dir / 'backend' / 'db' / 'data'
    weapon_csv = str(weapon_csv)
    gem_csv = str(gem_csv)
    out_dir = str(out_dir)

    print(f"Reading {weapon_csv}")
    weapons = read_weapons(weapon_csv)
    print(f"  {len(weapons)} base weapon rows (mod 100)")

    print(f"Reading {gem_csv}")
    gems = read_gems(gem_csv)
    print(f"  {len(gems)} gem rows")

    aow_fallback, mwtid_to_weptypes, warnings = compute_dlc_fallback(gems, weapons)

    files = [
        ('weapon_gem_mount.go', gen_weapon_gem_mount(weapons)),
        ('aow_compat.go',       gen_aow_compat(gems)),
        ('aow_dlc_compat.go',   gen_aow_dlc_compat(aow_fallback, mwtid_to_weptypes)),
        ('aow_affinity.go',     gen_aow_affinity(gems)),
        ('weapon_extras.go',    gen_weapon_extras(weapons)),
    ]
    for name, content in files:
        path = os.path.join(out_dir, name)
        write_atomic(path, content)
        print(f"Wrote {path}")

    # Stats
    print()
    gm0 = sum(1 for w in weapons if w['gm'] == 0)
    gm1 = sum(1 for w in weapons if w['gm'] == 1)
    gm2 = sum(1 for w in weapons if w['gm'] == 2)
    gems_with_mask = sum(1 for g in gems if g['mask'] != 0)
    gems_with_fallback = len(aow_fallback)
    print(f"Stats:")
    print(f"  weapons gm=0/1/2: {gm0}/{gm1}/{gm2}")
    print(f"  AoW gems with non-zero canMountWep mask: {gems_with_mask}/{len(gems)}")
    print(f"  AoW gems with DLC fallback wepTypes: {gems_with_fallback}")
    print(f"  mwtid groups computed: {len([m for m,v in mwtid_to_weptypes.items() if v])} (with mapped wepTypes)")

    # Warnings
    if warnings:
        print(f"\nAudit warnings ({len(warnings)}):")
        for w in warnings:
            print(f"  WARN: {w}")
    else:
        print("\nNo audit warnings.")


if __name__ == '__main__':
    main()
