// Ban-risk awareness dictionary.
// Phase 1: per-flag baseline (4 entries — cut_content, pre_order, dlc_duplicate, ban_risk).
// Phase 3: per-action entries (stat overflows, runes, pouch, etc.) for field-level outlines.
//
// Editorial guidance:
//   - Frame statements as community-reported, not officially confirmed by FromSoftware.
//   - "Why ban?" explains the detection mechanism in plain English.
//   - "Reports" cites volume/recency without inventing numbers.
//   - "Mitigation" gives the user a concrete way to reduce risk.
//   - Source URLs are optional — leave empty when not verified.

export type RiskLevel = 'low' | 'medium' | 'high';
export type RiskTier = 0 | 1 | 2;

export interface RiskSource {
    label: string;
    url?: string;
}

export interface RiskEntry {
    tier: RiskTier;
    level: RiskLevel;
    title: string;
    whyBan: string;
    reports: string;
    mitigation: string;
    sources: RiskSource[];
}

export type RiskKey =
    // Phase 1 — per-flag baseline (matches backend item flags)
    | 'cut_content'
    | 'pre_order'
    | 'dlc_duplicate'
    | 'ban_risk'
    // Phase 3 — per-action / per-field (Tier 2)
    | 'runes_above_999m'
    | 'stat_above_99'
    | 'level_above_713'
    | 'talisman_pouch_above_3'
    | 'quantity_above_max'
    | 'spirit_ash_above_10'
    | 'derived_stat_manual'
    // Phase 4 — bulk actions (Tier 1)
    | 'bulk_grace_unlock'
    | 'bulk_boss_kill'
    | 'bulk_cookbook'
    | 'bulk_bell_bearing'
    | 'bulk_gestures_unlock'
    | 'bulk_region_unlock'
    | 'bulk_summoning_pool'
    | 'bulk_colosseum'
    | 'map_reveal_full'
    | 'fow_remove'
    | 'quest_step_skip'
    | 'ng_plus_write'
    | 'character_import'
    | 'preset_apply';

export const RISK_INFO: Record<RiskKey, RiskEntry> = {
    cut_content: {
        tier: 2,
        level: 'high',
        title: 'Cut Content',
        whyBan:
            'These item IDs exist in the game data but were never released to retail. They cannot be obtained through normal play. Easy Anti-Cheat treats their presence as injected content because no legitimate progression can produce them.',
        reports:
            'Multiple ban reports across r/Eldenring and Discord communities (2022-2024) for cut armor sets, prototype talismans, and unfinished key items. Bans typically follow the first online connection after the edit.',
        mitigation:
            'Use only on save copies for offline experimentation. Remove cut items from inventory and storage before going online — for Bell Bearings and key items also clear the matching event flag, otherwise the acquisition record persists.',
        sources: [
            {label: 'r/Eldenring ban discussion threads (2022-2024)'},
            {label: 'Fextralife wiki — cut content notes'},
        ],
    },
    pre_order: {
        tier: 2,
        level: 'medium',
        title: 'Pre-Order Bonus',
        whyBan:
            'Items granted only with the pre-order edition of the game (or a specific bundle). Easy Anti-Cheat checks the account entitlement when these items appear in the save. If the account does not own the corresponding entitlement, the item is treated as injected.',
        reports:
            'Lower volume than cut content but consistently reported. Pre-order rings (e.g. Ring of Miquella variants) and the Carian Oath gesture are the most common offenders.',
        mitigation:
            'Safe if the account owns the pre-order entitlement. Otherwise do not add. Removing the item from inventory may not clear the acquisition flag — verify before going online.',
        sources: [
            {label: 'r/Eldenring pre-order item discussions'},
        ],
    },
    dlc_duplicate: {
        tier: 2,
        level: 'medium',
        title: 'DLC Duplicate ID',
        whyBan:
            'Some IDs were duplicated when Shadow of the Erdtree integrated, leaving two variants of the same item with different internal codes. Using the wrong variant (legacy ID without DLC, or DLC variant on a non-DLC account) can produce a save state inconsistent with the player\'s entitlements.',
        reports:
            'Sporadic. Mostly affects gestures (e.g. Ring of Miquella alternate slot) and a few duplicated key items.',
        mitigation:
            'Prefer the DLC-active variant if you own Shadow of the Erdtree, otherwise use the base game variant. When in doubt, do not add and pick the canonical equivalent instead.',
        sources: [
            {label: 'er-save-manager DLC duplicate notes'},
        ],
    },
    ban_risk: {
        tier: 2,
        level: 'high',
        title: 'Ban Risk (Generic)',
        whyBan:
            'This item or action has been associated with ban reports for reasons that may include cut content, illegal stat values, impossible game states, or detection rules whose exact mechanism is not publicly documented.',
        reports:
            'Aggregate flag used when the specific cause is unclear or overlaps multiple categories. Treat as worst-case until a more specific entry is available.',
        mitigation:
            'High-risk by default. Use only offline. Remove or revert the change before connecting online.',
        sources: [
            {label: 'Aggregate community ban reports'},
        ],
    },
    runes_above_999m: {
        tier: 2,
        level: 'high',
        title: 'Runes Above 999,999,999',
        whyBan:
            'The in-game maximum total runes a character can hold is 999,999,999 (≈1 billion). Values above this cap are not producible through any farming or item use — Easy Anti-Cheat reads the rune total during online sync and flags overflows as injected.',
        reports:
            'Consistently reported across Souls editor communities. Typical bans follow within hours of the first multiplayer match.',
        mitigation:
            'Keep total runes ≤ 999,999,999. If you need more for an offline run, raise temporarily then lower before going online.',
        sources: [
            {label: 'r/Eldenring rune cap discussions'},
        ],
    },
    stat_above_99: {
        tier: 2,
        level: 'high',
        title: 'Attribute Above 99',
        whyBan:
            'Each character attribute (Vigor, Mind, Endurance, Strength, Dexterity, Intelligence, Faith, Arcane) is hard-capped at 99 in-game. EAC validates per-attribute caps during online sync. Any value above 99 is impossible through legitimate level-ups.',
        reports:
            'High volume of reports — overflowing attributes is one of the oldest and most-detected save edits across the Souls series.',
        mitigation:
            'Keep every attribute in the 1-99 range. The level total derives automatically from the sum of attributes.',
        sources: [
            {label: 'r/Eldenring stat cap reports'},
        ],
    },
    level_above_713: {
        tier: 2,
        level: 'high',
        title: 'Character Level Above 713',
        whyBan:
            'The legal maximum character level is 713 = Σ(attribute caps 99×8) − 79 (starting class base). Any level above 713 implies an attribute over the 99 cap or a corrupted level value.',
        reports:
            'Detected together with attribute overflows; both flags often appear in the same ban incidents.',
        mitigation:
            'Let the editor recalculate level from attributes (level = sum(attrs) − 79). Do not manually override the level field.',
        sources: [
            {label: 'r/Eldenring level cap analysis'},
        ],
    },
    talisman_pouch_above_3: {
        tier: 2,
        level: 'high',
        title: 'Talisman Pouch Above 3',
        whyBan:
            'The maximum number of additional talisman slots is 3 (4 total including the starting slot), unlocked through specific Stonesword Key gates and boss rewards. A pouch count above 3 is impossible in retail play.',
        reports:
            'Less common than stat overflows but reported when used together with other illegal modifications.',
        mitigation:
            'Keep additional pouch ≤ 3.',
        sources: [
            {label: 'Fextralife — Talisman Pouch progression'},
        ],
    },
    quantity_above_max: {
        tier: 2,
        level: 'medium',
        title: 'Quantity Above MaxInventory',
        whyBan:
            'Each item has a per-inventory cap defined in regulation params (e.g. flasks 14, most consumables 99). Quantities above the cap are not producible through normal play and are visible to EAC during inventory sync.',
        reports:
            'Reported alongside cut content and stat overflows. Lower individual visibility but compounds with other flags.',
        mitigation:
            'Keep each stack ≤ the item\'s MaxInventory value. Use storage for excess.',
        sources: [
            {label: 'er-save-manager regulation cap notes'},
        ],
    },
    spirit_ash_above_10: {
        tier: 2,
        level: 'high',
        title: 'Spirit Ash Upgrade Above +10',
        whyBan:
            'Spirit Ashes max out at +10 in retail. Each tier is a distinct item ID (baseID + N for +N). IDs beyond +10 do not exist in regulation, so the game falls back to invalid entries which EAC flags.',
        reports:
            'Specific to spirit ash upgrade edits. Reported when invalid +N IDs appear in the inventory.',
        mitigation:
            'Keep upgrade level in the 0-10 range when adding spirit ashes from the database.',
        sources: [
            {label: 'er-save-manager spirit ash upgrade chains'},
        ],
    },
    derived_stat_manual: {
        tier: 2,
        level: 'high',
        title: 'Manually Overridden Derived Stat',
        whyBan:
            'HP, FP, and Stamina are computed at runtime from Vigor, Mind, and Endurance using fixed formulas. Manually overriding the stored values produces a save inconsistent with the attribute breakdown — EAC compares stored vs. derived during sync and flags mismatches.',
        reports:
            'Detected indirectly through stat-consistency checks. Pairs with attribute overflows in most ban incidents.',
        mitigation:
            'Edit Vigor / Mind / Endurance instead — the editor recalculates HP/FP/Stamina automatically on save.',
        sources: [
            {label: 'r/Eldenring stat consistency analyses'},
        ],
    },
    bulk_grace_unlock: {
        tier: 1,
        level: 'low',
        title: 'Bulk Grace Unlock',
        whyBan:
            'Unlocking many Sites of Grace at once via event flags creates a discovery state that does not match a normal exploration timeline. EAC may correlate grace flags with map reveal and combat history during online sync.',
        reports:
            'Occasional reports, almost always combined with map reveal or other bulk unlocks. Standalone bulk grace unlock has not been a frequently reported cause.',
        mitigation:
            'Do this offline. If concerned, keep the "Skip Boss Arenas" filter on so the timeline stays plausible, or unlock graces individually as you progress.',
        sources: [{label: 'Community discussions on grace flag editing'}],
    },
    bulk_boss_kill: {
        tier: 1,
        level: 'medium',
        title: 'Bulk Boss Defeat Flags',
        whyBan:
            'Setting many boss-defeated flags at once shifts rune rewards, NPC quest gates, and arena states in ways inconsistent with the player\'s combat history. Some boss flags trigger world-state cascades that EAC inspects during sync.',
        reports:
            'Mid-volume reports. More frequent when combined with attribute overflows or rune injection.',
        mitigation:
            'Use offline only. Defeat key bosses normally; use this for replay convenience on saves you do not intend to take online.',
        sources: [{label: 'r/Eldenring boss flag investigations'}],
    },
    bulk_cookbook: {
        tier: 1,
        level: 'low',
        title: 'Bulk Cookbook Unlock',
        whyBan:
            'Cookbooks tie to specific merchants and quest lines. Unlocking many at once skips merchant interactions and crafting progression that the game expects to see in the save.',
        reports:
            'Very few reports tied specifically to cookbook unlocks. Aggregate-only signal.',
        mitigation:
            'Low standalone risk. Combine cautiously with bulk grace or quest skip flags.',
        sources: [],
    },
    bulk_bell_bearing: {
        tier: 1,
        level: 'low',
        title: 'Bulk Bell Bearing Unlock',
        whyBan:
            'Bell Bearings expand merchant wares. Bulk unlocking sets the acquisition flags directly, skipping merchant kills or pickups that normally produce them.',
        reports:
            'Few standalone reports.',
        mitigation:
            'Low standalone risk. Consider unlocking individually to preserve merchant interactions.',
        sources: [],
    },
    bulk_gestures_unlock: {
        tier: 1,
        level: 'low',
        title: 'Bulk Gesture Unlock',
        whyBan:
            'Gestures are stored in a fixed array of unlocked IDs. Bulk unlocking adds them all without the in-game NPC interactions that normally grant them. The "Unlock All" button intentionally skips ban-risk gestures (cut content / pre-order / DLC duplicates).',
        reports:
            'Very few standalone reports. Risk concentrates in the individual ban-risk entries that "Unlock All" already excludes.',
        mitigation:
            'Low risk for vanilla gestures. Do not manually toggle the ban-risk gestures (cut / pre-order / DLC duplicate).',
        sources: [],
    },
    bulk_region_unlock: {
        tier: 1,
        level: 'low',
        title: 'Bulk Invasion Region Unlock',
        whyBan:
            'Unlocked invasion regions affect online matchmaking pools and phantom slot availability. Mass unlocking the entire region list creates a state most players reach only after very long playtime.',
        reports:
            'Very few reports.',
        mitigation:
            'Low risk for solo play. If you actively play online invasions, prefer naturally discovered regions.',
        sources: [],
    },
    bulk_summoning_pool: {
        tier: 1,
        level: 'medium',
        title: 'Bulk Summoning Pool Activation',
        whyBan:
            'Summoning pools (Martyr Effigies) tie to co-op matchmaking. Activating them all at once exposes the character to summon ranges that do not match their progression — EAC may correlate during online sync.',
        reports:
            'Sporadic. This feature is also flagged as buggy in this editor (UI toggles but in-game state may not update).',
        mitigation:
            'Activate offline only. For online co-op, activate gradually with progression.',
        sources: [],
    },
    bulk_colosseum: {
        tier: 1,
        level: 'low',
        title: 'Bulk Colosseum Unlock',
        whyBan:
            'Colosseum unlock flags only affect availability of arena multiplayer modes.',
        reports:
            'Reported as having no visible in-game effect when toggled offline. Risk likely low.',
        mitigation:
            'Low risk.',
        sources: [],
    },
    map_reveal_full: {
        tier: 1,
        level: 'low',
        title: 'Full Map Reveal',
        whyBan:
            'Setting all map fragment + acquisition flags reveals every region without exploration. EAC sees the discovery state during sync but the reveal is widely used in the community.',
        reports:
            'Almost no specific ban reports tied to map reveal alone.',
        mitigation:
            'Generally safe. Mostly a spoiler concern rather than a ban concern.',
        sources: [],
    },
    fow_remove: {
        tier: 1,
        level: 'low',
        title: 'Remove Fog of War',
        whyBan:
            'Removing the Fog of War exploration bitfield (2099 bytes filled with 0xFF) reveals every map tile without walking it.',
        reports:
            'No specific ban reports tied to FoW removal alone.',
        mitigation:
            'Low risk. Cosmetic detection is theoretically possible but not widely seen.',
        sources: [],
    },
    quest_step_skip: {
        tier: 1,
        level: 'medium',
        title: 'Quest Step Skip',
        whyBan:
            'Setting quest flags out of order or jumping to a late step can break NPC dialogue trees, lock out follow-up steps, or produce conflicting world states. EAC checks some flag combinations against expected sequences during sync; the more common failure mode is breaking the questline itself.',
        reports:
            'Occasional reports when combined with other bulk flag edits. Most damage is to the in-game questline, not the account.',
        mitigation:
            'Backup before skipping. Avoid jumping past quest gates that NPCs expect to see in order.',
        sources: [],
    },
    ng_plus_write: {
        tier: 1,
        level: 'low',
        title: 'Set NG+ Cycle',
        whyBan:
            'Changing the NG+ cycle directly skips the ending sequence that normally advances cycles. Mass NG+ jumps combined with rune totals and other progression flags can look anomalous.',
        reports:
            'Few standalone reports.',
        mitigation:
            'Low standalone risk. Avoid combining with rune injection or attribute overflows.',
        sources: [],
    },
    character_import: {
        tier: 1,
        level: 'medium',
        title: 'Character Import',
        whyBan:
            'Importing a character from another save copies stats, items, and progression flags wholesale. If the source save contains cut content, illegal stats, or other risky edits, those problems transfer to the destination.',
        reports:
            'Risk depends entirely on the source save. Clean source = low risk; edited source with cut content = high risk.',
        mitigation:
            'Audit the source save first. After import, scan the destination for ban badges (CUT / ⚠ BAN) and Tier 2 outlines, and clean before going online.',
        sources: [],
    },
    preset_apply: {
        tier: 1,
        level: 'medium',
        title: 'Apply Character Preset',
        whyBan:
            'Applying a preset replaces stats, inventory, or storage wholesale. If the preset contains illegitimate items (cut content, impossible upgrade levels, excessive quantities), those problems transfer to the character.',
        reports:
            'Risk depends on the preset source. Clean preset = low risk; preset with cap-breaking or cut-content items = high risk.',
        mitigation:
            'Review the preset preview carefully before applying. After apply, scan inventory for ban badges.',
        sources: [],
    },
};

// Helper: visual styling for fields whose current value falls into a Tier 2 risk.
// Concatenate with the field's existing class string when riskKey is non-null.
export const RISK_FIELD_CLASS = 'border-red-500/60 ring-1 ring-red-500/30 focus:border-red-500 focus:ring-red-500/40';

// Helpers for evaluating common Tier 2 conditions.
export function getRunesRiskKey(runes: number): RiskKey | null {
    return runes > 999_999_999 ? 'runes_above_999m' : null;
}

export function getAttributeRiskKey(value: number): RiskKey | null {
    return value > 99 ? 'stat_above_99' : null;
}

export function getLevelRiskKey(level: number): RiskKey | null {
    return level > 713 ? 'level_above_713' : null;
}

export function getTalismanPouchRiskKey(additionalSlots: number): RiskKey | null {
    return additionalSlots > 3 ? 'talisman_pouch_above_3' : null;
}

export function getQuantityRiskKey(qty: number, maxInventory: number): RiskKey | null {
    return maxInventory > 0 && qty > maxInventory ? 'quantity_above_max' : null;
}

export function getSpiritAshRiskKey(upgrade: number): RiskKey | null {
    return upgrade > 10 ? 'spirit_ash_above_10' : null;
}
