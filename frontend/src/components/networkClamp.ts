// Pure clamping helpers for NetworkParam editing.
//
// These enforce the same cross-field invariants that the Go backend validates in
// core.ValidateNetworkParams, so the UI can never submit a contradictory config:
//   - reloadSignCellCount <= reloadSignTotalCount <= singGetMax
//   - reloadSearchCoopBlueMin <= reloadSearchCoopBlueMax
//   - allAreaSearchRateCoopBlue / allAreaSearchRateVsBlue in [0, 100]
//
// Clamping is downward-only and deterministic: lowering a ceiling pulls the
// dependent values down with it; raising a floor above its ceiling is clamped to
// the ceiling. The user raises the ceiling first, then the dependent value.

export type NetDraft = Record<string, number>;

// Canonical field lists for the three functional preset groups. These are the
// ONLY fields a group's Faster / Vanilla button is allowed to touch. They
// deliberately exclude fields that are still treated as Experimental:
// - blue excludes maxCoopBlueSummonCount and allAreaSearchRateVsBlue (Blue extras)
// - none include Visitor / legacy ring-search fields
// This keeps presets modular: applying one group never overwrites another
// group's manual values or any field outside that group.
export const NETWORK_GROUP_KEYS = {
    invader: ['maxBreakInTargetListCount', 'breakInRequestAreaCount', 'breakInRequestIntervalTimeSec', 'breakInRequestTimeOutSec'],
    cooperator: ['reloadSignIntervalTime2', 'reloadSignTotalCount', 'reloadSignCellCount', 'updateSignIntervalTime', 'singGetMax', 'signDownloadSpan', 'signUpdateSpan'],
    blue: ['reloadVisitListCoolTime', 'reloadSearchCoopBlueMin', 'reloadSearchCoopBlueMax', 'maxVisitListCount', 'allAreaSearchRateCoopBlue'],
} as const;

export type NetworkGroupId = keyof typeof NETWORK_GROUP_KEYS;

// Merge ONLY listed group keys from `source` into `current`, then clamp.
// Every other field (other groups, Blue extras, Visitor / legacy) is preserved
// exactly. Used by both Faster per-group and Vanilla buttons so presets stay
// modular and composable.
export function applyGroupPreset(current: NetDraft, groupKeys: readonly string[], source: NetDraft): NetDraft {
    const next = {...current};
    for (const k of groupKeys) {
        if (k in source) next[k] = source[k];
    }
    return clampNetworkDraft(next);
}

export function clampNetworkDraft(input: NetDraft): NetDraft {
    const d = {...input};

    const clamp = (key: string, lo: number, hi: number) => {
        if (typeof d[key] === 'number') d[key] = Math.min(hi, Math.max(lo, d[key]));
    };

    // Percentage rates.
    clamp('allAreaSearchRateCoopBlue', 0, 100);
    clamp('allAreaSearchRateVsBlue', 0, 100);

    // Sign buffer chain: cell <= total <= getMax.
    if (typeof d.singGetMax === 'number' && typeof d.reloadSignTotalCount === 'number') {
        d.reloadSignTotalCount = Math.min(d.reloadSignTotalCount, d.singGetMax);
    }
    if (typeof d.reloadSignTotalCount === 'number' && typeof d.reloadSignCellCount === 'number') {
        d.reloadSignCellCount = Math.min(d.reloadSignCellCount, d.reloadSignTotalCount);
    }

    // Blue reload window: min <= max.
    if (typeof d.reloadSearchCoopBlueMin === 'number' && typeof d.reloadSearchCoopBlueMax === 'number') {
        d.reloadSearchCoopBlueMin = Math.min(d.reloadSearchCoopBlueMin, d.reloadSearchCoopBlueMax);
    }

    return d;
}

// Returns a human-readable reason if the draft violates an invariant, else null.
// Used as a final guard before submitting (clampNetworkDraft should already
// prevent these, but this protects against any path that bypasses clamping).
export function networkDraftError(d: NetDraft): string | null {
    if (d.reloadSignCellCount > d.reloadSignTotalCount) {
        return 'Signs Per Cell cannot exceed Signs Retrieved';
    }
    if (d.reloadSignTotalCount > d.singGetMax) {
        return 'Signs Retrieved cannot exceed Sign Get Max';
    }
    if (d.reloadSearchCoopBlueMin > d.reloadSearchCoopBlueMax) {
        return 'Blue Reload Min cannot exceed Reload Max';
    }
    return null;
}
