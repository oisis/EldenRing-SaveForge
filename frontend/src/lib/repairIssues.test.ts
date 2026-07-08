import { describe, expect, it } from 'vitest';
import { shouldAutoOpenOnLoad, type RepairIssueReport } from './repairIssues';

function report(hasIssues: boolean): RepairIssueReport {
    return {
        slotIndex: 0,
        charName: 'T',
        source: 'loaded',
        hasIssues,
        issues: [],
        coverage: {
            totalPhysical: 1, resolved: 1, knownDB: 1, technicalPlaceholder: 0, unknown: 0,
            resolutionChecksApplied: 1, structuralChecksApplied: 1, categoryChecksApplied: 1,
            perCategory: {}, unknownByReason: {},
        },
    };
}

describe('shouldAutoOpenOnLoad', () => {
    // The automatic on-load scan must stay issue-only: a clean load never pops a modal.
    it('is false for a clean report', () => {
        expect(shouldAutoOpenOnLoad(report(false))).toBe(false);
    });

    it('is true when the report has issues', () => {
        expect(shouldAutoOpenOnLoad(report(true))).toBe(true);
    });
});
