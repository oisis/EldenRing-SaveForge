import {describe, it, expect} from 'vitest';
import {clampNetworkDraft, networkDraftError, applyGroupPreset, NETWORK_GROUP_KEYS} from './networkClamp';

const vanilla = (): Record<string, number> => ({
    maxBreakInTargetListCount: 5,
    breakInRequestIntervalTimeSec: 30,
    breakInRequestTimeOutSec: 20,
    breakInRequestAreaCount: 5,
    reloadSignIntervalTime2: 60,
    reloadSignTotalCount: 20,
    reloadSignCellCount: 10,
    updateSignIntervalTime: 30,
    singGetMax: 32,
    signDownloadSpan: 30,
    signUpdateSpan: 60,
    reloadVisitListCoolTime: 20,
    maxCoopBlueSummonCount: 2,
    maxVisitListCount: 5,
    reloadSearchCoopBlueMin: 30,
    reloadSearchCoopBlueMax: 180,
    allAreaSearchRateCoopBlue: 30,
    allAreaSearchRateVsBlue: 30,
    visitorListMax: 10,
    visitorTimeOutTime: 60,
    visitorDownloadSpan: 60,
});

describe('clampNetworkDraft', () => {
    it('leaves a valid vanilla draft untouched', () => {
        const d = vanilla();
        expect(clampNetworkDraft(d)).toEqual(d);
    });

    it('caps reloadSignCellCount at reloadSignTotalCount', () => {
        const d = {...vanilla(), reloadSignCellCount: 30, reloadSignTotalCount: 20};
        expect(clampNetworkDraft(d).reloadSignCellCount).toBe(20);
    });

    it('caps reloadSignTotalCount at singGetMax and cascades to cellCount', () => {
        const d = {...vanilla(), reloadSignTotalCount: 40, reloadSignCellCount: 40, singGetMax: 32};
        const out = clampNetworkDraft(d);
        expect(out.reloadSignTotalCount).toBe(32);
        expect(out.reloadSignCellCount).toBe(32);
    });

    it('caps reloadSearchCoopBlueMin at reloadSearchCoopBlueMax', () => {
        const d = {...vanilla(), reloadSearchCoopBlueMin: 100, reloadSearchCoopBlueMax: 40};
        expect(clampNetworkDraft(d).reloadSearchCoopBlueMin).toBe(40);
    });

    it('clamps allAreaSearchRate fields to 0-100', () => {
        const d = {...vanilla(), allAreaSearchRateCoopBlue: 150, allAreaSearchRateVsBlue: -5};
        const out = clampNetworkDraft(d);
        expect(out.allAreaSearchRateCoopBlue).toBe(100);
        expect(out.allAreaSearchRateVsBlue).toBe(0);
    });

    it('does not mutate the input object', () => {
        const d = {...vanilla(), reloadSignCellCount: 30, reloadSignTotalCount: 20};
        clampNetworkDraft(d);
        expect(d.reloadSignCellCount).toBe(30);
    });
});

// Preset sources mirror backend NetworkParamFaster* (Defaults + group changes).
const fasterReds = (): Record<string, number> => ({...vanilla(), maxBreakInTargetListCount: 8, breakInRequestIntervalTimeSec: 12, breakInRequestTimeOutSec: 15});
const fasterSummons = (): Record<string, number> => ({...vanilla(), reloadSignIntervalTime2: 20, reloadSignTotalCount: 40, reloadSignCellCount: 20, updateSignIntervalTime: 15, singGetMax: 64, signDownloadSpan: 15, signUpdateSpan: 20});
const fasterBlue = (): Record<string, number> => ({...vanilla(), reloadVisitListCoolTime: 8, reloadSearchCoopBlueMin: 10, reloadSearchCoopBlueMax: 40, maxVisitListCount: 10, allAreaSearchRateCoopBlue: 60});

// Aggressive preset sources mirror backend NetworkParamAggressive* (Defaults + group changes).
const aggressiveReds = (): Record<string, number> => ({...vanilla(), maxBreakInTargetListCount: 12, breakInRequestIntervalTimeSec: 8, breakInRequestTimeOutSec: 12});
const aggressiveSummons = (): Record<string, number> => ({...vanilla(), reloadSignIntervalTime2: 10, reloadSignTotalCount: 64, reloadSignCellCount: 32, updateSignIntervalTime: 10, singGetMax: 96, signDownloadSpan: 10, signUpdateSpan: 10});
const aggressiveBlue = (): Record<string, number> => ({...vanilla(), reloadVisitListCoolTime: 5, reloadSearchCoopBlueMin: 5, reloadSearchCoopBlueMax: 20, maxVisitListCount: 15, allAreaSearchRateCoopBlue: 100});

describe('NETWORK_GROUP_KEYS — group scope excludes Experimental', () => {
    it('invader does not include the unknown 0x7C field', () => {
        expect(NETWORK_GROUP_KEYS.invader).not.toContain('breakInRequestAreaCount');
        expect(NETWORK_GROUP_KEYS.invader).toHaveLength(3);
    });
    it('blue does not include Blue extras', () => {
        expect(NETWORK_GROUP_KEYS.blue).not.toContain('maxCoopBlueSummonCount');
        expect(NETWORK_GROUP_KEYS.blue).not.toContain('allAreaSearchRateVsBlue');
        expect(NETWORK_GROUP_KEYS.blue).toHaveLength(5);
    });
    it('no group includes Visitor / legacy fields', () => {
        const all = [...NETWORK_GROUP_KEYS.invader, ...NETWORK_GROUP_KEYS.cooperator, ...NETWORK_GROUP_KEYS.blue];
        expect(all).not.toContain('visitorListMax');
        expect(all).not.toContain('visitorTimeOutTime');
        expect(all).not.toContain('visitorDownloadSpan');
    });
});

describe('preset composition (modular merge)', () => {
    // TC-01 — composing all three presets accumulates every group's changes.
    it('TC-01: applying Reds → Summons → Blue keeps all three groups, 0x7C/visitor/extras vanilla', () => {
        let s = vanilla();
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.invader, fasterReds());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.cooperator, fasterSummons());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.blue, fasterBlue());

        // Reds
        expect(s.maxBreakInTargetListCount).toBe(8);
        expect(s.breakInRequestIntervalTimeSec).toBe(12);
        expect(s.breakInRequestTimeOutSec).toBe(15);
        // Summons
        expect(s.reloadSignIntervalTime2).toBe(20);
        expect(s.reloadSignTotalCount).toBe(40);
        expect(s.singGetMax).toBe(64);
        // Blue
        expect(s.reloadVisitListCoolTime).toBe(8);
        expect(s.allAreaSearchRateCoopBlue).toBe(60);
        // Untouched
        expect(s.breakInRequestAreaCount).toBe(5);
        expect(s.maxCoopBlueSummonCount).toBe(2);
        expect(s.allAreaSearchRateVsBlue).toBe(30);
        expect(s.visitorListMax).toBe(10);
        expect(s.visitorTimeOutTime).toBe(60);
    });

    // TC-02 — Reds does not reset manually-tuned Summons / Visitor / 0x7C.
    it('TC-02: Faster Reds changes only the 3 Reds fields', () => {
        const start = {
            ...vanilla(),
            reloadSignIntervalTime2: 45, reloadSignTotalCount: 30, reloadSignCellCount: 15, singGetMax: 50, // custom Summons (valid)
            visitorListMax: 42,                                                                              // custom Visitor
            breakInRequestAreaCount: 8,                                                                      // custom Experimental 0x7C
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.invader, fasterReds());

        expect(s.maxBreakInTargetListCount).toBe(8);
        expect(s.breakInRequestIntervalTimeSec).toBe(12);
        expect(s.breakInRequestTimeOutSec).toBe(15);
        // preserved
        expect(s.reloadSignIntervalTime2).toBe(45);
        expect(s.reloadSignTotalCount).toBe(30);
        expect(s.reloadSignCellCount).toBe(15);
        expect(s.singGetMax).toBe(50);
        expect(s.visitorListMax).toBe(42);
        expect(s.breakInRequestAreaCount).toBe(8);
    });

    // TC-03 — Summons does not reset Reds or Blue.
    it('TC-03: Faster Summons changes only Summons fields', () => {
        const start = {
            ...vanilla(),
            maxBreakInTargetListCount: 7, breakInRequestIntervalTimeSec: 18, breakInRequestTimeOutSec: 12, // custom Reds
            reloadVisitListCoolTime: 12, maxVisitListCount: 8, allAreaSearchRateCoopBlue: 45,               // custom Blue
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.cooperator, fasterSummons());

        // Summons applied
        expect(s.reloadSignIntervalTime2).toBe(20);
        expect(s.reloadSignTotalCount).toBe(40);
        expect(s.singGetMax).toBe(64);
        // Reds preserved
        expect(s.maxBreakInTargetListCount).toBe(7);
        expect(s.breakInRequestIntervalTimeSec).toBe(18);
        expect(s.breakInRequestTimeOutSec).toBe(12);
        // Blue preserved
        expect(s.reloadVisitListCoolTime).toBe(12);
        expect(s.maxVisitListCount).toBe(8);
        expect(s.allAreaSearchRateCoopBlue).toBe(45);
    });

    // TC-04 — Blue does not reset other groups nor Blue extras.
    it('TC-04: Faster Blue changes only the 5 Blue fields, keeps extras manual', () => {
        const start = {
            ...vanilla(),
            maxBreakInTargetListCount: 7,                          // custom Reds
            reloadSignTotalCount: 30,                              // custom Summons
            maxCoopBlueSummonCount: 5,                             // custom Blue extra
            allAreaSearchRateVsBlue: 80,                           // custom Blue extra
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.blue, fasterBlue());

        // Blue main applied
        expect(s.reloadVisitListCoolTime).toBe(8);
        expect(s.reloadSearchCoopBlueMin).toBe(10);
        expect(s.reloadSearchCoopBlueMax).toBe(40);
        expect(s.maxVisitListCount).toBe(10);
        expect(s.allAreaSearchRateCoopBlue).toBe(60);
        // Blue extras preserved
        expect(s.maxCoopBlueSummonCount).toBe(5);
        expect(s.allAreaSearchRateVsBlue).toBe(80);
        // Other groups preserved
        expect(s.maxBreakInTargetListCount).toBe(7);
        expect(s.reloadSignTotalCount).toBe(30);
    });

    // TC-05 — per-group Vanilla resets only its own group.
    it('TC-05: group Vanilla resets only that group', () => {
        const custom = {
            ...vanilla(),
            maxBreakInTargetListCount: 9, breakInRequestIntervalTimeSec: 8, breakInRequestTimeOutSec: 10,
            reloadSignTotalCount: 50, singGetMax: 64,
            reloadVisitListCoolTime: 6, maxVisitListCount: 12, allAreaSearchRateCoopBlue: 70,
            breakInRequestAreaCount: 7, maxCoopBlueSummonCount: 4, visitorListMax: 30,
        };

        // Vanilla for Reds only.
        const reds = applyGroupPreset(custom, NETWORK_GROUP_KEYS.invader, vanilla());
        expect(reds.maxBreakInTargetListCount).toBe(5);
        expect(reds.breakInRequestIntervalTimeSec).toBe(30);
        expect(reds.breakInRequestTimeOutSec).toBe(20);
        // everything else preserved
        expect(reds.reloadSignTotalCount).toBe(50);
        expect(reds.reloadVisitListCoolTime).toBe(6);
        expect(reds.breakInRequestAreaCount).toBe(7);
        expect(reds.maxCoopBlueSummonCount).toBe(4);
        expect(reds.visitorListMax).toBe(30);

        // Vanilla for Blue only — leaves Blue extras and other groups intact.
        const blue = applyGroupPreset(custom, NETWORK_GROUP_KEYS.blue, vanilla());
        expect(blue.reloadVisitListCoolTime).toBe(20);
        expect(blue.maxVisitListCount).toBe(5);
        expect(blue.allAreaSearchRateCoopBlue).toBe(30);
        expect(blue.maxCoopBlueSummonCount).toBe(4); // extra preserved
        expect(blue.maxBreakInTargetListCount).toBe(9); // Reds preserved
        expect(blue.breakInRequestAreaCount).toBe(7); // 0x7C preserved
    });
});

describe('Aggressive preset composition (modular merge)', () => {
    // TC-A1 — three Aggressive presets compose; Experimental + Visitor untouched.
    it('TC-A1: Aggressive Reds → Summons → Blue keeps all three groups, 0x7C/extras/visitor vanilla', () => {
        let s = vanilla();
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.invader, aggressiveReds());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.cooperator, aggressiveSummons());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.blue, aggressiveBlue());

        // Reds
        expect(s.maxBreakInTargetListCount).toBe(12);
        expect(s.breakInRequestIntervalTimeSec).toBe(8);
        expect(s.breakInRequestTimeOutSec).toBe(12);
        // Summons (invariant cell<=total<=getMax holds: 32<=64<=96)
        expect(s.reloadSignIntervalTime2).toBe(10);
        expect(s.reloadSignTotalCount).toBe(64);
        expect(s.reloadSignCellCount).toBe(32);
        expect(s.singGetMax).toBe(96);
        expect(s.signDownloadSpan).toBe(10);
        expect(s.signUpdateSpan).toBe(10);
        // Blue
        expect(s.reloadVisitListCoolTime).toBe(5);
        expect(s.reloadSearchCoopBlueMin).toBe(5);
        expect(s.reloadSearchCoopBlueMax).toBe(20);
        expect(s.maxVisitListCount).toBe(15);
        expect(s.allAreaSearchRateCoopBlue).toBe(100);
        // Untouched — Experimental + Visitor
        expect(s.breakInRequestAreaCount).toBe(5);
        expect(s.maxCoopBlueSummonCount).toBe(2);
        expect(s.allAreaSearchRateVsBlue).toBe(30);
        expect(s.visitorListMax).toBe(10);
        expect(s.visitorTimeOutTime).toBe(60);
        expect(s.visitorDownloadSpan).toBe(60);
    });

    // TC-A2 — Aggressive Reds keeps Experimental 0x7C and other groups.
    it('TC-A2: Aggressive Reds changes only the 3 Reds fields', () => {
        const start = {
            ...vanilla(),
            breakInRequestAreaCount: 9,                                   // manual Experimental 0x7C
            reloadSignTotalCount: 50, reloadSignCellCount: 25, singGetMax: 80, // manual Summons
            reloadVisitListCoolTime: 7, allAreaSearchRateCoopBlue: 55,    // manual Blue
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.invader, aggressiveReds());

        expect(s.maxBreakInTargetListCount).toBe(12);
        expect(s.breakInRequestIntervalTimeSec).toBe(8);
        expect(s.breakInRequestTimeOutSec).toBe(12);
        // preserved
        expect(s.breakInRequestAreaCount).toBe(9);
        expect(s.reloadSignTotalCount).toBe(50);
        expect(s.reloadSignCellCount).toBe(25);
        expect(s.singGetMax).toBe(80);
        expect(s.reloadVisitListCoolTime).toBe(7);
        expect(s.allAreaSearchRateCoopBlue).toBe(55);
    });

    // TC-A3 — Aggressive Summons keeps other groups; invariants hold.
    it('TC-A3: Aggressive Summons changes only Summons fields, invariants hold', () => {
        const start = {
            ...vanilla(),
            maxBreakInTargetListCount: 7, breakInRequestTimeOutSec: 14,   // manual Reds
            reloadVisitListCoolTime: 9, maxVisitListCount: 11,            // manual Blue
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.cooperator, aggressiveSummons());

        expect(s.reloadSignTotalCount).toBe(64);
        expect(s.reloadSignCellCount).toBe(32);
        expect(s.singGetMax).toBe(96);
        expect(s.reloadSignCellCount <= s.reloadSignTotalCount).toBe(true);
        expect(s.reloadSignTotalCount <= s.singGetMax).toBe(true);
        // other groups preserved
        expect(s.maxBreakInTargetListCount).toBe(7);
        expect(s.breakInRequestTimeOutSec).toBe(14);
        expect(s.reloadVisitListCoolTime).toBe(9);
        expect(s.maxVisitListCount).toBe(11);
    });

    // TC-A4 — Aggressive Blue keeps Blue Experimental extras and other groups.
    it('TC-A4: Aggressive Blue changes only the 5 Blue fields, keeps extras manual', () => {
        const start = {
            ...vanilla(),
            maxCoopBlueSummonCount: 6,                                    // manual Blue Search Parallelism (Experimental)
            allAreaSearchRateVsBlue: 85,                                  // manual Retribution Global % (Experimental)
            maxBreakInTargetListCount: 9,                                 // manual Reds
            reloadSignTotalCount: 30,                                     // manual Summons (<= vanilla singGetMax 32)
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.blue, aggressiveBlue());

        // Blue main applied
        expect(s.reloadVisitListCoolTime).toBe(5);
        expect(s.reloadSearchCoopBlueMin).toBe(5);
        expect(s.reloadSearchCoopBlueMax).toBe(20);
        expect(s.maxVisitListCount).toBe(15);
        expect(s.allAreaSearchRateCoopBlue).toBe(100);
        // Blue Experimental preserved
        expect(s.maxCoopBlueSummonCount).toBe(6);
        expect(s.allAreaSearchRateVsBlue).toBe(85);
        // other groups preserved
        expect(s.maxBreakInTargetListCount).toBe(9);
        expect(s.reloadSignTotalCount).toBe(30);
    });

    // TC-A5 — group Vanilla after Aggressive resets only stable fields, keeps Experimental.
    it('TC-A5: group Vanilla resets only stable group fields, leaves Experimental untouched', () => {
        let s = vanilla();
        // Aggressive everything + manual Experimental values.
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.invader, aggressiveReds());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.blue, aggressiveBlue());
        s = {...s, breakInRequestAreaCount: 8, maxCoopBlueSummonCount: 5, allAreaSearchRateVsBlue: 70};

        // Vanilla Reds: resets 3 Reds fields, keeps 0x7C.
        const vReds = applyGroupPreset(s, NETWORK_GROUP_KEYS.invader, vanilla());
        expect(vReds.maxBreakInTargetListCount).toBe(5);
        expect(vReds.breakInRequestIntervalTimeSec).toBe(30);
        expect(vReds.breakInRequestTimeOutSec).toBe(20);
        expect(vReds.breakInRequestAreaCount).toBe(8); // Experimental untouched
        // Blue still aggressive (other group untouched by Reds vanilla)
        expect(vReds.allAreaSearchRateCoopBlue).toBe(100);

        // Vanilla Blue: resets 5 Blue fields, keeps Blue Experimental.
        const vBlue = applyGroupPreset(s, NETWORK_GROUP_KEYS.blue, vanilla());
        expect(vBlue.reloadVisitListCoolTime).toBe(20);
        expect(vBlue.allAreaSearchRateCoopBlue).toBe(30);
        expect(vBlue.maxCoopBlueSummonCount).toBe(5);   // Experimental untouched
        expect(vBlue.allAreaSearchRateVsBlue).toBe(70); // Experimental untouched
    });
});

describe('networkDraftError', () => {
    it('returns null for a valid draft', () => {
        expect(networkDraftError(vanilla())).toBeNull();
    });

    it('flags cell > total', () => {
        expect(networkDraftError({...vanilla(), reloadSignCellCount: 30, reloadSignTotalCount: 20}))
            .toMatch(/Per Cell/);
    });

    it('flags total > getMax', () => {
        expect(networkDraftError({...vanilla(), reloadSignTotalCount: 40, singGetMax: 32}))
            .toMatch(/Sign Get Max/);
    });

    it('flags blue min > max', () => {
        expect(networkDraftError({...vanilla(), reloadSearchCoopBlueMin: 100, reloadSearchCoopBlueMax: 40}))
            .toMatch(/Reload Min/);
    });
});
