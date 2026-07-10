import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SafetyProfile } from './state/safetyProfile';

// jsdom here can lack localStorage; App reads it at init for the safety profile.
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

vi.mock('../wailsjs/runtime/runtime', () => ({
    EventsOn: () => () => {},
}));

// App only calls a handful of these on mount; resolve everything to empty.
vi.mock('../wailsjs/go/main/App', () => {
    const r = () => vi.fn().mockResolvedValue([]);
    return {
        SelectAndOpenSave: r(), GetSlotStates: r(), CleanResidualSlot: r(),
        SetSlotActivity: r(), WriteSave: r(), CloneSlot: r(), DeleteSlot: r(),
        GetCharacter: r(), RevertSlot: r(), GetUndoDepth: r(), GetInfuseTypes: r(),
        GetSlotCapacity: r(), AuditLoadedSaveIssues: r(), GetSaveInventoryIntegrityReport: r(),
        RepairDuplicateInventoryIndices: r(), CloseSave: r(), RunDiagnosticsAllLoaded: r(),
        GetAppVersion: r(),
    };
});

// Mock every child so App renders just its shell + header. Factories must be
// self-contained (vi.mock is hoisted above module scope).
vi.mock('./components/CharacterTab', () => ({ CharacterTab: () => null }));
vi.mock('./components/InventoryTab', () => ({ InventoryTab: () => null }));
vi.mock('./components/WorldTab', () => ({ WorldTab: () => null }));
vi.mock('./components/SettingsTab', () => ({ SettingsTab: () => null }));
vi.mock('./components/DiagnosticsModal', () => ({ DiagnosticsModal: () => null }));
vi.mock('./components/InventoryIssuesModal', () => ({ InventoryIssuesModal: () => null }));
vi.mock('./components/DatabaseTab', () => ({ DatabaseTab: () => null }));
vi.mock('./components/AppearanceTab', () => ({ AppearanceTab: () => null }));
vi.mock('./components/PvPTab', () => ({ PvPTab: () => null }));
vi.mock('./components/SortOrderTab', () => ({ SortOrderTab: () => null }));
vi.mock('./components/integrity/InventoryIntegrityModal', () => ({ InventoryIntegrityModal: () => null }));
vi.mock('./components/templates/TemplatesShellModal', () => ({ TemplatesShellModal: () => null }));
// toast pulls ToastBar's log helpers at import; stub the whole module.
vi.mock('./lib/toast', () => {
    const fn = vi.fn() as unknown as Record<string, unknown> & ((...a: unknown[]) => void);
    fn.success = vi.fn(); fn.error = vi.fn(); fn.loading = vi.fn();
    return { default: fn };
});
vi.mock('./components/ToastBar', () => ({ ToastBar: () => null }));

import App from './App';
import { SafetyModeProvider } from './state/safetyMode';

function renderApp(profile: SafetyProfile) {
    localStorage.setItem('setting:safetyProfile', profile);
    return render(
        <SafetyModeProvider>
            <App />
        </SafetyModeProvider>,
    );
}

describe('App top-bar safety profile indicator', () => {
    beforeEach(() => localStorage.clear());
    afterEach(() => vi.clearAllMocks());

    it('shows SAFE MODE for the safe profile and no legacy online-safety banner', () => {
        renderApp('safe');
        expect(screen.getByText('SAFE MODE')).toBeInTheDocument();
        expect(screen.queryByText(/Tier 2 edits disabled/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/Online Safety Mode/i)).not.toBeInTheDocument();
        expect(screen.queryByText('EXPANDED LIMITS')).not.toBeInTheDocument();
        expect(screen.queryByText('CHAOS MODE!!!')).not.toBeInTheDocument();
    });

    it('shows EXPANDED LIMITS for the expanded_limits profile', () => {
        renderApp('expanded_limits');
        expect(screen.getByText('EXPANDED LIMITS')).toBeInTheDocument();
        expect(screen.queryByText('SAFE MODE')).not.toBeInTheDocument();
    });

    it('shows CHAOS MODE!!! for the chaos profile', () => {
        renderApp('chaos');
        expect(screen.getByText('CHAOS MODE!!!')).toBeInTheDocument();
        expect(screen.queryByText('SAFE MODE')).not.toBeInTheDocument();
    });
});
