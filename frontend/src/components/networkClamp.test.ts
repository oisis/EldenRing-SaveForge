import {describe, it, expect} from 'vitest';
import {clampNetworkDraft, networkDraftError, applyGroupPreset, NETWORK_GROUP_KEYS} from './networkClamp';

const vanilla = (): Record<string, number> => ({
    maxBreakInTargetListCount: 5,
    breakInRequestIntervalTimeSec: 30,
    breakInRequestTimeOutSec: 20,
    breakInRequestAreaCount: 5,
    summonTimeoutTime: 45,
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

// Preset sources mirror backend NetworkParamFaster* / NetworkParamAggressive* (Defaults + group changes).
const fasterReds       = (): Record<string, number> => ({...vanilla(), maxBreakInTargetListCount: 8,  breakInRequestAreaCount: 8,  breakInRequestIntervalTimeSec: 12, breakInRequestTimeOutSec: 8});
const aggressiveReds   = (): Record<string, number> => ({...vanilla(), maxBreakInTargetListCount: 12, breakInRequestAreaCount: 12, breakInRequestIntervalTimeSec: 10, breakInRequestTimeOutSec: 7});
const fasterSummonHost     = (): Record<string, number> => ({...vanilla(), summonTimeoutTime: 10, reloadSignIntervalTime2: 20, reloadSignTotalCount: 24, singGetMax: 40, signDownloadSpan: 15});
const aggressiveSummonHost = (): Record<string, number> => ({...vanilla(), summonTimeoutTime: 7,  reloadSignIntervalTime2: 12, reloadSignTotalCount: 20, singGetMax: 48, signDownloadSpan: 10});
const fasterSummonGuest     = (): Record<string, number> => ({...vanilla(), updateSignIntervalTime: 15, signUpdateSpan: 20});
const aggressiveSummonGuest = (): Record<string, number> => ({...vanilla(), updateSignIntervalTime: 10, signUpdateSpan: 12});
const fasterHunter     = (): Record<string, number> => ({...vanilla(), reloadVisitListCoolTime: 10, maxVisitListCount: 8,  reloadSearchCoopBlueMin: 12, reloadSearchCoopBlueMax: 72});
const aggressiveHunter = (): Record<string, number> => ({...vanilla(), reloadVisitListCoolTime: 6,  maxVisitListCount: 12, reloadSearchCoopBlueMin: 8,  reloadSearchCoopBlueMax: 48});

describe('NETWORK_GROUP_KEYS — group scope excludes fields outside each role', () => {
    it('invader includes breakInRequestAreaCount (4 fields)', () => {
        expect(NETWORK_GROUP_KEYS.invader).toContain('breakInRequestAreaCount');
        expect(NETWORK_GROUP_KEYS.invader).toHaveLength(4);
    });
    it('summonHost includes summonTimeoutTime, excludes reloadSignCellCount', () => {
        expect(NETWORK_GROUP_KEYS.summonHost).toContain('summonTimeoutTime');
        expect(NETWORK_GROUP_KEYS.summonHost).not.toContain('reloadSignCellCount');
        expect(NETWORK_GROUP_KEYS.summonHost).toHaveLength(5);
    });
    it('summonGuest has exactly updateSignIntervalTime and signUpdateSpan', () => {
        expect(NETWORK_GROUP_KEYS.summonGuest).toContain('updateSignIntervalTime');
        expect(NETWORK_GROUP_KEYS.summonGuest).toContain('signUpdateSpan');
        expect(NETWORK_GROUP_KEYS.summonGuest).toHaveLength(2);
    });
    it('hunter excludes Blue extras and allAreaSearchRateCoopBlue', () => {
        expect(NETWORK_GROUP_KEYS.hunter).not.toContain('maxCoopBlueSummonCount');
        expect(NETWORK_GROUP_KEYS.hunter).not.toContain('allAreaSearchRateVsBlue');
        expect(NETWORK_GROUP_KEYS.hunter).not.toContain('allAreaSearchRateCoopBlue');
        expect(NETWORK_GROUP_KEYS.hunter).toHaveLength(4);
    });
    it('no group includes Visitor / legacy fields', () => {
        const all = [...NETWORK_GROUP_KEYS.invader, ...NETWORK_GROUP_KEYS.summonHost, ...NETWORK_GROUP_KEYS.summonGuest, ...NETWORK_GROUP_KEYS.hunter];
        expect(all).not.toContain('visitorListMax');
        expect(all).not.toContain('visitorTimeOutTime');
        expect(all).not.toContain('visitorDownloadSpan');
    });
});

describe('preset composition (modular merge)', () => {
    // TC-01 — composing all four presets accumulates every group's changes.
    it('TC-01: applying Reds → SummonHost → SummonGuest → Hunter keeps all groups, visitor/extras vanilla', () => {
        let s = vanilla();
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.invader,     fasterReds());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.summonHost,  fasterSummonHost());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.summonGuest, fasterSummonGuest());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.hunter,      fasterHunter());

        // Reds
        expect(s.maxBreakInTargetListCount).toBe(8);
        expect(s.breakInRequestAreaCount).toBe(8);
        expect(s.breakInRequestIntervalTimeSec).toBe(12);
        expect(s.breakInRequestTimeOutSec).toBe(8);
        // SummonHost
        expect(s.summonTimeoutTime).toBe(10);
        expect(s.reloadSignIntervalTime2).toBe(20);
        expect(s.reloadSignTotalCount).toBe(24);
        expect(s.singGetMax).toBe(40);
        // SummonGuest
        expect(s.updateSignIntervalTime).toBe(15);
        expect(s.signUpdateSpan).toBe(20);
        // Hunter
        expect(s.reloadVisitListCoolTime).toBe(10);
        expect(s.maxVisitListCount).toBe(8);
        expect(s.reloadSearchCoopBlueMin).toBe(12);
        expect(s.reloadSearchCoopBlueMax).toBe(72);
        // Untouched
        expect(s.maxCoopBlueSummonCount).toBe(2);
        expect(s.allAreaSearchRateVsBlue).toBe(30);
        expect(s.visitorListMax).toBe(10);
        expect(s.visitorTimeOutTime).toBe(60);
    });

    // TC-02 — Reds does not reset manually-tuned Summon or Visitor fields.
    it('TC-02: Faster Reds changes only the 4 Reds fields', () => {
        const start = {
            ...vanilla(),
            summonTimeoutTime: 12, reloadSignIntervalTime2: 30, singGetMax: 50, // custom SummonHost
            updateSignIntervalTime: 18, signUpdateSpan: 25,                       // custom SummonGuest
            visitorListMax: 42,                                                    // custom Visitor
            breakInRequestAreaCount: 9,                                            // custom Reds area count
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.invader, fasterReds());

        expect(s.maxBreakInTargetListCount).toBe(8);
        expect(s.breakInRequestIntervalTimeSec).toBe(12);
        expect(s.breakInRequestTimeOutSec).toBe(8);
        expect(s.breakInRequestAreaCount).toBe(8);
        // preserved non-Reds
        expect(s.summonTimeoutTime).toBe(12);
        expect(s.reloadSignIntervalTime2).toBe(30);
        expect(s.updateSignIntervalTime).toBe(18);
        expect(s.visitorListMax).toBe(42);
    });

    // TC-03 — SummonHost does not reset other groups.
    it('TC-03: Faster SummonHost changes only summonHost fields', () => {
        const start = {
            ...vanilla(),
            maxBreakInTargetListCount: 7, breakInRequestIntervalTimeSec: 18, // custom Reds
            updateSignIntervalTime: 20, signUpdateSpan: 25,                    // custom SummonGuest
            reloadVisitListCoolTime: 12, maxVisitListCount: 8,                // custom Hunter
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.summonHost, fasterSummonHost());

        // SummonHost applied
        expect(s.summonTimeoutTime).toBe(10);
        expect(s.reloadSignIntervalTime2).toBe(20);
        expect(s.reloadSignTotalCount).toBe(24);
        expect(s.singGetMax).toBe(40);
        expect(s.signDownloadSpan).toBe(15);
        // Reds preserved
        expect(s.maxBreakInTargetListCount).toBe(7);
        expect(s.breakInRequestIntervalTimeSec).toBe(18);
        // SummonGuest preserved
        expect(s.updateSignIntervalTime).toBe(20);
        expect(s.signUpdateSpan).toBe(25);
        // Hunter preserved
        expect(s.reloadVisitListCoolTime).toBe(12);
        expect(s.maxVisitListCount).toBe(8);
    });

    // TC-04 — Hunter does not reset other groups nor Blue extras.
    it('TC-04: Faster Hunter changes only the 4 hunter fields, keeps extras manual', () => {
        const start = {
            ...vanilla(),
            maxBreakInTargetListCount: 7,    // custom Reds
            summonTimeoutTime: 15,            // custom SummonHost
            maxCoopBlueSummonCount: 5,        // custom Blue extra (Experimental)
            allAreaSearchRateVsBlue: 80,      // custom Blue extra (Experimental)
            allAreaSearchRateCoopBlue: 55,    // custom Blue extra (Experimental)
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.hunter, fasterHunter());

        // Hunter applied
        expect(s.reloadVisitListCoolTime).toBe(10);
        expect(s.maxVisitListCount).toBe(8);
        expect(s.reloadSearchCoopBlueMin).toBe(12);
        expect(s.reloadSearchCoopBlueMax).toBe(72);
        // Extras preserved
        expect(s.maxCoopBlueSummonCount).toBe(5);
        expect(s.allAreaSearchRateVsBlue).toBe(80);
        expect(s.allAreaSearchRateCoopBlue).toBe(55);
        // Other groups preserved
        expect(s.maxBreakInTargetListCount).toBe(7);
        expect(s.summonTimeoutTime).toBe(15);
    });

    // TC-05 — per-group Vanilla resets only its own group.
    it('TC-05: group Vanilla resets only that group', () => {
        const custom = {
            ...vanilla(),
            maxBreakInTargetListCount: 9, breakInRequestIntervalTimeSec: 8, breakInRequestAreaCount: 7,
            summonTimeoutTime: 12, reloadSignTotalCount: 50, singGetMax: 64,
            updateSignIntervalTime: 20,
            reloadVisitListCoolTime: 6, maxVisitListCount: 12,
            maxCoopBlueSummonCount: 4, visitorListMax: 30,
        };

        // Vanilla for Reds only.
        const reds = applyGroupPreset(custom, NETWORK_GROUP_KEYS.invader, vanilla());
        expect(reds.maxBreakInTargetListCount).toBe(5);
        expect(reds.breakInRequestIntervalTimeSec).toBe(30);
        expect(reds.breakInRequestAreaCount).toBe(5);
        expect(reds.summonTimeoutTime).toBe(12); // SummonHost preserved
        expect(reds.reloadVisitListCoolTime).toBe(6); // Hunter preserved

        // Vanilla for Hunter only — leaves Blue extras and other groups intact.
        const hunter = applyGroupPreset(custom, NETWORK_GROUP_KEYS.hunter, vanilla());
        expect(hunter.reloadVisitListCoolTime).toBe(20);
        expect(hunter.maxVisitListCount).toBe(5);
        expect(hunter.maxCoopBlueSummonCount).toBe(4); // Experimental preserved
        expect(hunter.maxBreakInTargetListCount).toBe(9); // Reds preserved
        expect(hunter.summonTimeoutTime).toBe(12); // SummonHost preserved
    });
});

describe('Aggressive preset composition (modular merge)', () => {
    // TC-A1 — four Aggressive presets compose; extras + Visitor untouched.
    it('TC-A1: Aggressive Reds → SummonHost → SummonGuest → Hunter keeps all groups, extras/visitor vanilla', () => {
        let s = vanilla();
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.invader,     aggressiveReds());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.summonHost,  aggressiveSummonHost());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.summonGuest, aggressiveSummonGuest());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.hunter,      aggressiveHunter());

        // Reds
        expect(s.maxBreakInTargetListCount).toBe(12);
        expect(s.breakInRequestAreaCount).toBe(12);
        expect(s.breakInRequestIntervalTimeSec).toBe(10);
        expect(s.breakInRequestTimeOutSec).toBe(7);
        // SummonHost
        expect(s.summonTimeoutTime).toBe(7);
        expect(s.reloadSignIntervalTime2).toBe(12);
        expect(s.reloadSignTotalCount).toBe(20);
        expect(s.singGetMax).toBe(48);
        // SummonGuest
        expect(s.updateSignIntervalTime).toBe(10);
        expect(s.signUpdateSpan).toBe(12);
        // Hunter
        expect(s.reloadVisitListCoolTime).toBe(6);
        expect(s.reloadSearchCoopBlueMin).toBe(8);
        expect(s.reloadSearchCoopBlueMax).toBe(48);
        expect(s.maxVisitListCount).toBe(12);
        // Untouched — extras + Visitor
        expect(s.maxCoopBlueSummonCount).toBe(2);
        expect(s.allAreaSearchRateVsBlue).toBe(30);
        expect(s.visitorListMax).toBe(10);
        expect(s.visitorTimeOutTime).toBe(60);
        expect(s.visitorDownloadSpan).toBe(60);
    });

    // TC-A2 — Aggressive Reds keeps other groups.
    it('TC-A2: Aggressive Reds changes only the 4 Reds fields', () => {
        const start = {
            ...vanilla(),
            breakInRequestAreaCount: 9,                                   // manual Reds area count
            summonTimeoutTime: 15, reloadSignTotalCount: 50, singGetMax: 80, // manual SummonHost
            reloadVisitListCoolTime: 7,                                    // manual Hunter
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.invader, aggressiveReds());

        expect(s.maxBreakInTargetListCount).toBe(12);
        expect(s.breakInRequestAreaCount).toBe(12);
        expect(s.breakInRequestIntervalTimeSec).toBe(10);
        expect(s.breakInRequestTimeOutSec).toBe(7);
        // preserved
        expect(s.summonTimeoutTime).toBe(15);
        expect(s.reloadSignTotalCount).toBe(50);
        expect(s.singGetMax).toBe(80);
        expect(s.reloadVisitListCoolTime).toBe(7);
    });

    // TC-A3 — Aggressive SummonHost keeps other groups; invariants hold.
    it('TC-A3: Aggressive SummonHost changes only summonHost fields, invariants hold', () => {
        const start = {
            ...vanilla(),
            maxBreakInTargetListCount: 7, breakInRequestTimeOutSec: 14, // manual Reds
            reloadVisitListCoolTime: 9, maxVisitListCount: 11,          // manual Hunter
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.summonHost, aggressiveSummonHost());

        expect(s.summonTimeoutTime).toBe(7);
        expect(s.reloadSignTotalCount).toBe(20);
        expect(s.singGetMax).toBe(48);
        expect(s.reloadSignTotalCount <= s.singGetMax).toBe(true);
        // other groups preserved
        expect(s.maxBreakInTargetListCount).toBe(7);
        expect(s.breakInRequestTimeOutSec).toBe(14);
        expect(s.reloadVisitListCoolTime).toBe(9);
        expect(s.maxVisitListCount).toBe(11);
    });

    // TC-A4 — Aggressive Hunter keeps Experimental extras and other groups.
    it('TC-A4: Aggressive Hunter changes only the 4 hunter fields, keeps extras manual', () => {
        const start = {
            ...vanilla(),
            maxCoopBlueSummonCount: 6,     // manual Blue Search Parallelism (Experimental)
            allAreaSearchRateVsBlue: 85,   // manual Retribution Global % (Experimental)
            allAreaSearchRateCoopBlue: 70, // manual Global Search % (Experimental)
            maxBreakInTargetListCount: 9,  // manual Reds
            summonTimeoutTime: 12,         // manual SummonHost
        };
        const s = applyGroupPreset(start, NETWORK_GROUP_KEYS.hunter, aggressiveHunter());

        // Hunter applied
        expect(s.reloadVisitListCoolTime).toBe(6);
        expect(s.reloadSearchCoopBlueMin).toBe(8);
        expect(s.reloadSearchCoopBlueMax).toBe(48);
        expect(s.maxVisitListCount).toBe(12);
        // Experimental preserved
        expect(s.maxCoopBlueSummonCount).toBe(6);
        expect(s.allAreaSearchRateVsBlue).toBe(85);
        expect(s.allAreaSearchRateCoopBlue).toBe(70);
        // other groups preserved
        expect(s.maxBreakInTargetListCount).toBe(9);
        expect(s.summonTimeoutTime).toBe(12);
    });

    // TC-A5 — group Vanilla after Aggressive resets only that group, keeps extras.
    it('TC-A5: group Vanilla resets only stable group fields, leaves extras untouched', () => {
        let s = vanilla();
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.invader, aggressiveReds());
        s = applyGroupPreset(s, NETWORK_GROUP_KEYS.hunter, aggressiveHunter());
        s = {...s, breakInRequestAreaCount: 8, maxCoopBlueSummonCount: 5, allAreaSearchRateVsBlue: 70};

        // Vanilla Reds: resets 4 Reds fields.
        const vReds = applyGroupPreset(s, NETWORK_GROUP_KEYS.invader, vanilla());
        expect(vReds.maxBreakInTargetListCount).toBe(5);
        expect(vReds.breakInRequestIntervalTimeSec).toBe(30);
        expect(vReds.breakInRequestTimeOutSec).toBe(20);
        expect(vReds.breakInRequestAreaCount).toBe(5);
        // Hunter still aggressive (other group untouched by Reds vanilla)
        expect(vReds.reloadVisitListCoolTime).toBe(6);

        // Vanilla Hunter: resets 4 hunter fields, keeps Experimental.
        const vHunter = applyGroupPreset(s, NETWORK_GROUP_KEYS.hunter, vanilla());
        expect(vHunter.reloadVisitListCoolTime).toBe(20);
        expect(vHunter.maxVisitListCount).toBe(5);
        expect(vHunter.maxCoopBlueSummonCount).toBe(5);   // Experimental untouched
        expect(vHunter.allAreaSearchRateVsBlue).toBe(70); // Experimental untouched
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
