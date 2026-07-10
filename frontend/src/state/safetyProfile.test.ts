import { beforeEach, describe, expect, it } from 'vitest';
import {
    loadSafetyProfile,
    saveSafetyProfile,
    usesTechnicalCaps,
    revealsRiskyItems,
    onlineSafetyEnabled,
    type SafetyProfile,
} from './safetyProfile';

// jsdom with an opaque origin can leave localStorage undefined.
if (typeof globalThis.localStorage === 'undefined') {
    const store = new Map<string, string>();
    Object.defineProperty(globalThis, 'localStorage', {
        configurable: true,
        value: {
            getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
            setItem: (k: string, v: string) => { store.set(k, String(v)); },
            removeItem: (k: string) => { store.delete(k); },
            clear: () => { store.clear(); },
        },
    });
}

describe('loadSafetyProfile migration', () => {
    beforeEach(() => localStorage.clear());

    it('defaults to safe on first launch', () => {
        expect(loadSafetyProfile()).toBe('safe');
    });

    it('uses a valid stored profile', () => {
        localStorage.setItem('setting:safetyProfile', 'expanded_limits');
        expect(loadSafetyProfile()).toBe('expanded_limits');
    });

    it('ignores an invalid stored profile and falls back to safe', () => {
        localStorage.setItem('setting:safetyProfile', 'bogus');
        expect(loadSafetyProfile()).toBe('safe');
    });

    it('migrates legacy fullChaosMode=true to chaos and persists it', () => {
        localStorage.setItem('setting:fullChaosMode', 'true');
        expect(loadSafetyProfile()).toBe('chaos');
        expect(localStorage.getItem('setting:safetyProfile')).toBe('chaos');
    });

    it('does NOT reveal risky items for legacy showFlaggedItems alone — resolves to safe', () => {
        localStorage.setItem('setting:showFlaggedItems', 'true');
        expect(loadSafetyProfile()).toBe('safe');
    });
});

describe('saveSafetyProfile', () => {
    beforeEach(() => localStorage.clear());

    it('persists the profile and keeps the legacy chaos key only for chaos', () => {
        saveSafetyProfile('expanded_limits');
        expect(localStorage.getItem('setting:safetyProfile')).toBe('expanded_limits');
        expect(localStorage.getItem('setting:fullChaosMode')).toBe('false');

        saveSafetyProfile('chaos');
        expect(localStorage.getItem('setting:fullChaosMode')).toBe('true');
    });

    it('dispatches the sync event with the new profile', () => {
        const seen: SafetyProfile[] = [];
        const handler = (e: Event) => seen.push((e as CustomEvent<SafetyProfile>).detail);
        window.addEventListener('safetyProfileChanged', handler);
        saveSafetyProfile('chaos');
        window.removeEventListener('safetyProfileChanged', handler);
        expect(seen).toEqual(['chaos']);
    });
});

describe('profile derivations', () => {
    it('technical caps: expanded_limits and chaos only', () => {
        expect(usesTechnicalCaps('safe')).toBe(false);
        expect(usesTechnicalCaps('expanded_limits')).toBe(true);
        expect(usesTechnicalCaps('chaos')).toBe(true);
    });

    it('risky visibility: chaos only', () => {
        expect(revealsRiskyItems('safe')).toBe(false);
        expect(revealsRiskyItems('expanded_limits')).toBe(false);
        expect(revealsRiskyItems('chaos')).toBe(true);
    });

    it('online safety: safe only', () => {
        expect(onlineSafetyEnabled('safe')).toBe(true);
        expect(onlineSafetyEnabled('expanded_limits')).toBe(false);
        expect(onlineSafetyEnabled('chaos')).toBe(false);
    });
});
