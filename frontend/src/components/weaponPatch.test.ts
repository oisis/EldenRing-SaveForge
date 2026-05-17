import { describe, expect, it } from 'vitest';
import { aowApplyPatch, aowAssignPatch, aowClearPatch, infusionPatch, upgradePatch } from './weaponPatch';

describe('weaponPatch helpers', () => {
    it('upgradePatch sets only the upgrade fields', () => {
        const p = upgradePatch(15);
        expect(p.setUpgrade).toBe(true);
        expect(p.upgrade).toBe(15);
        expect(p.setInfusionName).toBeFalsy();
        expect(p.setAoWItemID).toBeFalsy();
        expect(p.clearAoW).toBeFalsy();
    });

    it('upgradePatch accepts level 0 (downgrade to base)', () => {
        const p = upgradePatch(0);
        expect(p.setUpgrade).toBe(true);
        expect(p.upgrade).toBe(0);
    });

    it('infusionPatch encodes Standard as empty string', () => {
        const p = infusionPatch('Standard');
        expect(p.setInfusionName).toBe(true);
        expect(p.infusionName).toBe('');
    });

    it('infusionPatch passes other names through unchanged', () => {
        const p = infusionPatch('Keen');
        expect(p.setInfusionName).toBe(true);
        expect(p.infusionName).toBe('Keen');
    });

    it('infusionPatch accepts pre-normalized empty string', () => {
        const p = infusionPatch('');
        expect(p.setInfusionName).toBe(true);
        expect(p.infusionName).toBe('');
    });

    it('aowAssignPatch sets only setAoWItemID + aowItemID', () => {
        const p = aowAssignPatch(0x80002710);
        expect(p.setAoWItemID).toBe(true);
        expect(p.aowItemID).toBe(0x80002710);
        expect(p.clearAoW).toBeFalsy();
        expect(p.setUpgrade).toBeFalsy();
        expect(p.setInfusionName).toBeFalsy();
    });

    it('aowClearPatch sets only clearAoW=true', () => {
        const p = aowClearPatch();
        expect(p.clearAoW).toBe(true);
        expect(p.setAoWItemID).toBeFalsy();
        expect(p.aowItemID).toBeFalsy();
    });

    it('aowApplyPatch routes id=0 to clear', () => {
        const p = aowApplyPatch(0);
        expect(p.clearAoW).toBe(true);
        expect(p.setAoWItemID).toBeFalsy();
    });

    it('aowApplyPatch routes non-zero id to assign', () => {
        const p = aowApplyPatch(0x80002710);
        expect(p.setAoWItemID).toBe(true);
        expect(p.aowItemID).toBe(0x80002710);
        expect(p.clearAoW).toBeFalsy();
    });
});
