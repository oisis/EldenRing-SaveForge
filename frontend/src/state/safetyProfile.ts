// Single source of truth for the editor's safety posture. Replaces the three
// independent legacy toggles (online safety / show-flagged / full-chaos) with
// one explicit, mutually-exclusive profile.
//
//   safe            conservative caps, risky/cut items hidden, online safety on
//   expanded_limits technical/game caps, risky/cut items still hidden
//   chaos           technical/game caps, risky/cut items revealed (strong warning)
//
// Components derive the booleans they need via the helpers below; they must not
// read the legacy localStorage keys as a primary source.
export type SafetyProfile = 'safe' | 'expanded_limits' | 'chaos';

const PROFILE_KEY = 'setting:safetyProfile';
const LEGACY_CHAOS_KEY = 'setting:fullChaosMode';

// Cross-component sync: any writer dispatches this; App/DatabaseTab listen.
export const SAFETY_PROFILE_EVENT = 'safetyProfileChanged';

const VALID: readonly string[] = ['safe', 'expanded_limits', 'chaos'];

// loadSafetyProfile resolves the active profile with legacy migration:
//   1. valid setting:safetyProfile wins.
//   2. else legacy setting:fullChaosMode === 'true' migrates to chaos.
//   3. else safe. (Legacy setting:showFlaggedItems is intentionally NOT migrated
//      to a risk-revealing profile — bare flagged visibility resets to safe.)
export function loadSafetyProfile(): SafetyProfile {
    try {
        const stored = localStorage.getItem(PROFILE_KEY);
        if (stored && VALID.includes(stored)) return stored as SafetyProfile;
        if (localStorage.getItem(LEGACY_CHAOS_KEY) === 'true') {
            saveSafetyProfile('chaos');
            return 'chaos';
        }
    } catch {
        // localStorage unavailable — fall through to the safe default.
    }
    return 'safe';
}

// saveSafetyProfile persists the profile and notifies listeners. It also keeps
// the legacy chaos key pointed at the fully-permissive state (chaos only) for
// backwards compatibility, but that key is never the source of truth.
export function saveSafetyProfile(profile: SafetyProfile): void {
    try {
        localStorage.setItem(PROFILE_KEY, profile);
        localStorage.setItem(LEGACY_CHAOS_KEY, String(profile === 'chaos'));
    } catch {
        // localStorage unavailable — keep the event-driven in-memory sync only.
    }
    window.dispatchEvent(new CustomEvent<SafetyProfile>(SAFETY_PROFILE_EVENT, { detail: profile }));
}

// Use technical/game item caps and the game-limit add path.
export const usesTechnicalCaps = (p: SafetyProfile): boolean =>
    p === 'expanded_limits' || p === 'chaos';

// Reveal cut-content / ban-risk items in lists.
export const revealsRiskyItems = (p: SafetyProfile): boolean => p === 'chaos';

// Online safety gating (Tier 1 confirm / Tier 2 disabled) is on only in safe.
export const onlineSafetyEnabled = (p: SafetyProfile): boolean => p === 'safe';
