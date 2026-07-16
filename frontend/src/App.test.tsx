import { render, screen, fireEvent, waitFor } from '@testing-library/react';
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
        GetAppVersion: r(), ScanRepairIssuesLoaded: r(),
        AnalyzeGaItemRepack: r(), ExecuteGaItemRepack: r(),
        SetDiagnosticDebugMode: r(),
        RecordDiagnosticClientNavigation: r(),
        RecordDiagnosticIntegrityModalShown: r(),
        RecordDiagnosticIntegrityModalRepairOutcome: r(),
    };
});

// Mock every child so App renders just its shell + header. Factories must be
// self-contained (vi.mock is hoisted above module scope).
vi.mock('./components/CharacterTab', () => ({ CharacterTab: () => null }));
vi.mock('./components/InventoryTab', () => ({ InventoryTab: () => null }));
vi.mock('./components/WorldTab', () => ({ WorldTab: () => null }));
// The stub exposes the App-owned callback so a test can open the shared modal.
vi.mock('./components/SettingsTab', () => ({
    SettingsTab: (props: { onOptimizeGaItem?: () => void; onResolveDuplicateGaItem?: (s: number, h: number) => void; setDebugMode?: (v: boolean) => void }) => (
        <>
            <button onClick={() => props.onOptimizeGaItem?.()}>open-gaitem-repack</button>
            <button onClick={() => props.onResolveDuplicateGaItem?.(0, 0x80000102)}>settings-resolve-dup</button>
            <button onClick={() => props.setDebugMode?.(true)}>settings-enable-debug</button>
        </>
    ),
}));
vi.mock('./components/DiagnosticsModal', () => ({ DiagnosticsModal: () => null }));
vi.mock('./components/InventoryIssuesModal', () => ({ InventoryIssuesModal: () => null }));
// Stub the shared duplicate modal so its endpoints are not exercised here; it
// exposes the App-owned handle/char context plus an onRefresh trigger.
vi.mock('./components/GaItemDuplicateRepairModal', () => ({
    GaItemDuplicateRepairModal: (props: { charIndex: number; handle: number; onRefresh: () => void }) => (
        <div data-testid="dup-modal" data-char={props.charIndex} data-handle={props.handle}>
            <button onClick={props.onRefresh}>dup-refresh</button>
        </div>
    ),
}));
// The editable stub exposes the App-owned CTA callback with display context.
vi.mock('./components/DatabaseTab', () => ({
    DatabaseTab: (props: { onOptimizeGaItem?: (ctx: { neededGaItems: number }) => void }) =>
        props.onOptimizeGaItem
            ? <button onClick={() => props.onOptimizeGaItem?.({ neededGaItems: 7 })}>db-optimize-gaitem</button>
            : null,
}));
vi.mock('./components/AppearanceTab', () => ({ AppearanceTab: () => null }));
vi.mock('./components/PvPTab', () => ({ PvPTab: () => null }));
vi.mock('./components/SortOrderTab', () => ({ SortOrderTab: () => null }));
// The stub exposes the App-owned Repair callback so a test can drive
// handleRepairIntegrity without the real modal.
vi.mock('./components/integrity/InventoryIntegrityModal', () => ({
    InventoryIntegrityModal: (props: { onRepair: () => void }) => (
        <button onClick={props.onRepair}>integrity-repair</button>
    ),
}));
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
import { SelectAndOpenSave, GetSaveInventoryIntegrityReport, AnalyzeGaItemRepack, ExecuteGaItemRepack, WriteSave, SetDiagnosticDebugMode, RecordDiagnosticClientNavigation, RecordDiagnosticIntegrityModalShown, RecordDiagnosticIntegrityModalRepairOutcome, RepairDuplicateInventoryIndices } from '../wailsjs/go/main/App';
import toast from './lib/toast';

const CAP = (physicalEmpty: number, cursorRoom: number, usable: number) => ({ physicalEmpty, cursorRoom, usable });

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

describe('App navigation wording', () => {
    beforeEach(() => localStorage.clear());
    afterEach(() => vi.clearAllMocks());

    it('labels the top-level Game Items tab, not the legacy Inventory label', () => {
        renderApp('safe');
        expect(screen.getByRole('button', { name: 'Game Items' })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /^Inventory$/ })).not.toBeInTheDocument();
    });

    it('records a main-tab transition for the diagnostic journal', async () => {
        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: 'tools' }));
        await waitFor(() => expect(RecordDiagnosticClientNavigation).toHaveBeenCalledWith('main_tab', 'character', 'tools'));
    });

    it('names the equipment submenu view Inventory once a save is loaded', async () => {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport).mockResolvedValue({ clean: true, slots: [] } as never);

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));
        await new Promise(r => setTimeout(r, 0));

        fireEvent.click(screen.getByRole('button', { name: 'Game Items' }));
        expect(await screen.findByRole('button', { name: 'Inventory' })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'Equipment' })).not.toBeInTheDocument();
    });

    it('labels the sort submenu view "Sort Order", not the legacy "Weapons & Sort Order"', async () => {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport).mockResolvedValue({ clean: true, slots: [] } as never);

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));
        await new Promise(r => setTimeout(r, 0));

        fireEvent.click(screen.getByRole('button', { name: 'Game Items' }));
        expect(await screen.findByRole('button', { name: 'Sort Order' })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Weapons & Sort Order/ })).not.toBeInTheDocument();
    });
});

describe('App open-save button wording', () => {
    beforeEach(() => localStorage.clear());
    afterEach(() => vi.clearAllMocks());

    it('keeps only the sidebar "Open Save File" action before a save is loaded', () => {
        renderApp('safe');
        expect(screen.getByRole('button', { name: /Open Save File/i })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'Open Save' })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Change Save/i })).not.toBeInTheDocument();
    });

    it('reads "Open Save" once a save is loaded', async () => {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport).mockResolvedValue({ clean: true, slots: [] } as never);

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));

        // Let the async open→integrity→setPlatform chain settle before asserting.
        await new Promise(r => setTimeout(r, 0));
        expect(await screen.findByRole('button', { name: /^Open Save$/i })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Change Save/i })).not.toBeInTheDocument();
    });
});

describe('App inventory integrity diagnostics', () => {
    beforeEach(() => localStorage.clear());
    afterEach(() => vi.clearAllMocks());

    const dirtyReport = {
        clean: false,
        slots: [{ slotIndex: 0, characterName: 'X', active: true, duplicateEntryCount: 1, conflictingIndexCount: 1, conflicts: [] }],
    };

    it('records the modal shown when a dirty report gates the save', async () => {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport).mockResolvedValue(dirtyReport as never);

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));

        await waitFor(() => expect(RecordDiagnosticIntegrityModalShown).toHaveBeenCalledTimes(1));
    });

    it('records resolved when repair clears the report', async () => {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport)
            .mockResolvedValueOnce(dirtyReport as never)
            .mockResolvedValueOnce({ clean: true, slots: [] } as never);
        vi.mocked(RepairDuplicateInventoryIndices).mockResolvedValue({} as never);

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'integrity-repair' }));

        await waitFor(() => expect(RecordDiagnosticIntegrityModalRepairOutcome).toHaveBeenCalledWith('resolved'));
    });

    it('records unresolved when repair leaves the report dirty', async () => {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport).mockResolvedValue(dirtyReport as never);
        vi.mocked(RepairDuplicateInventoryIndices).mockResolvedValue({} as never);

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'integrity-repair' }));

        await waitFor(() => expect(RecordDiagnosticIntegrityModalRepairOutcome).toHaveBeenCalledWith('unresolved'));
    });

    it('records error when a repair call rejects', async () => {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport).mockResolvedValue(dirtyReport as never);
        vi.mocked(RepairDuplicateInventoryIndices).mockRejectedValue(new Error('boom'));

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'integrity-repair' }));

        await waitFor(() => expect(RecordDiagnosticIntegrityModalRepairOutcome).toHaveBeenCalledWith('error'));
    });
});

describe('App unsupported-container handling', () => {
    beforeEach(() => localStorage.clear());
    afterEach(() => vi.clearAllMocks());

    it('opens a blocking modal with the safety explanation, not integrity scan', async () => {
        vi.mocked(SelectAndOpenSave).mockRejectedValue(
            new Error('ERR_UNSUPPORTED_CONTAINER: this file could not be identified safely'),
        );

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));

        expect(await screen.findByText('Unsupported Save Format')).toBeInTheDocument();
        expect(screen.getByText(/not opened/i)).toBeInTheDocument();
        expect(screen.getByText(/will not rewrite it/i)).toBeInTheDocument();
        expect(screen.getByText(/conversion is currently unavailable/i)).toBeInTheDocument();
        // A rejected open must never proceed to the integrity check.
        expect(GetSaveInventoryIntegrityReport).not.toHaveBeenCalled();

        fireEvent.click(screen.getByRole('button', { name: /^OK$/i }));
        await new Promise(r => setTimeout(r, 0));
        expect(screen.queryByText('Unsupported Save Format')).not.toBeInTheDocument();
    });

    it('is blocking: clicking the backdrop does not close the modal', async () => {
        vi.mocked(SelectAndOpenSave).mockRejectedValue(
            new Error('ERR_UNSUPPORTED_CONTAINER: this file could not be identified safely'),
        );

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));

        expect(await screen.findByText('Unsupported Save Format')).toBeInTheDocument();
        fireEvent.click(screen.getByTestId('unsupported-save-backdrop'));
        await new Promise(r => setTimeout(r, 0));
        // Still open — only the OK button dismisses it.
        expect(screen.getByText('Unsupported Save Format')).toBeInTheDocument();
    });

    it('keeps toast behavior for a generic open error (no modal)', async () => {
        vi.mocked(SelectAndOpenSave).mockRejectedValue(new Error('some other failure'));

        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));

        await new Promise(r => setTimeout(r, 0));
        expect(screen.queryByText('Unsupported Save Format')).not.toBeInTheDocument();
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('some other failure'));
    });
});

describe('App GaItem repack modal wiring', () => {
    beforeEach(() => localStorage.clear());
    afterEach(() => vi.clearAllMocks());

    async function loadSave() {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport).mockResolvedValue({ clean: true, slots: [] } as never);
        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));
        await new Promise(r => setTimeout(r, 0));
    }

    it('opens the shared modal for the selected character and analyzes it', async () => {
        vi.mocked(AnalyzeGaItemRepack).mockResolvedValue({
            outcome: 'ready', characterIndex: 0, analysisToken: 't',
            before: CAP(2, 3, 40), projectedAfter: CAP(7, 8, 45),
            recovered: 5, nonEmptyRecords: 9, blockers: [],
        } as never);

        await loadSave();
        fireEvent.click(screen.getByRole('button', { name: /tools/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'open-gaitem-repack' }));

        await screen.findByText('Ready to optimize');
        expect(AnalyzeGaItemRepack).toHaveBeenCalledWith(0);
    });

    it('opens with CTA context from the Database, and context-free from Tools', async () => {
        vi.mocked(AnalyzeGaItemRepack).mockResolvedValue({
            outcome: 'ready', characterIndex: 0, analysisToken: 't',
            before: CAP(2, 3, 40), projectedAfter: CAP(7, 8, 45),
            recovered: 5, nonEmptyRecords: 9, blockers: [],
        } as never);

        await loadSave();
        // Reach the editable DatabaseTab and fire its CTA.
        fireEvent.click(screen.getByRole('button', { name: /game items/i }));
        fireEvent.click(screen.getByRole('button', { name: /Item Database/i }));
        fireEvent.click(screen.getByRole('button', { name: 'db-optimize-gaitem' }));

        await screen.findByText('Ready to optimize');
        expect(screen.getByText('After optimizing, try adding the rejected batch again.')).toBeInTheDocument();

        // Close, then reopen from Tools — the modal must carry no context.
        fireEvent.click(screen.getByRole('button', { name: /^Close$/ }));
        fireEvent.click(screen.getByRole('button', { name: /tools/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'open-gaitem-repack' }));

        await screen.findByText('Ready to optimize');
        expect(screen.queryByText('After optimizing, try adding the rejected batch again.')).not.toBeInTheDocument();
    });

    it('routes the success Write Save through the central App path', async () => {
        vi.mocked(AnalyzeGaItemRepack).mockResolvedValue({
            outcome: 'ready', characterIndex: 0, analysisToken: 't',
            before: CAP(2, 3, 40), projectedAfter: CAP(7, 8, 45),
            recovered: 5, nonEmptyRecords: 9, blockers: [],
        } as never);
        vi.mocked(ExecuteGaItemRepack).mockResolvedValue({
            outcome: 'success', characterIndex: 0,
            before: CAP(2, 3, 40), after: CAP(7, 8, 45), recovered: 5,
        } as never);

        await loadSave();
        fireEvent.click(screen.getByRole('button', { name: /tools/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'open-gaitem-repack' }));
        fireEvent.click(await screen.findByRole('button', { name: /^Continue$/ }));
        fireEvent.click(await screen.findByRole('button', { name: /^Optimize allocation$/ }));
        await screen.findByText('GaItem allocation optimized');

        fireEvent.click(screen.getByRole('button', { name: /^Write Save$/ }));
        await waitFor(() => expect(WriteSave).toHaveBeenCalled());
    });
});

describe('App shared duplicate GaItem repair wiring', () => {
    beforeEach(() => localStorage.clear());
    afterEach(() => vi.clearAllMocks());

    async function loadSave() {
        vi.mocked(SelectAndOpenSave).mockResolvedValue('PC' as never);
        vi.mocked(GetSaveInventoryIntegrityReport).mockResolvedValue({ clean: true, slots: [] } as never);
        renderApp('safe');
        fireEvent.click(screen.getByRole('button', { name: /Open Save File/i }));
        await new Promise(r => setTimeout(r, 0));
    }

    it('opens the shared duplicate modal from a Diagnostics duplicate action with slot + handle', async () => {
        await loadSave();
        fireEvent.click(screen.getByRole('button', { name: /tools/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'settings-resolve-dup' }));

        const modal = await screen.findByTestId('dup-modal');
        expect(modal).toHaveAttribute('data-char', '0');
        expect(modal).toHaveAttribute('data-handle', String(0x80000102));
    });

    it('opens the shared duplicate modal from a repack duplicate_handle refusal and closes the repack modal', async () => {
        vi.mocked(AnalyzeGaItemRepack).mockResolvedValue({
            outcome: 'refusal', characterIndex: 0, blockers: [
                { code: 'duplicate_handle', message: 'GaItem[2] reuses handle 0x80000102.', handle: 0x80000102 },
            ],
            before: CAP(0, 0, 0), recovered: 0, nonEmptyRecords: 0,
        } as never);

        await loadSave();
        fireEvent.click(screen.getByRole('button', { name: /tools/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'open-gaitem-repack' }));
        await screen.findByText('Optimization unavailable');

        fireEvent.click(screen.getByRole('button', { name: /Resolve duplicate GaItem/ }));
        const modal = await screen.findByTestId('dup-modal');
        expect(modal).toHaveAttribute('data-handle', String(0x80000102));
        // The repack modal is gone.
        expect(screen.queryByText('Optimization unavailable')).not.toBeInTheDocument();
    });

    it('refreshes after a successful dedup but never auto-runs GaItem optimization', async () => {
        await loadSave();
        fireEvent.click(screen.getByRole('button', { name: /tools/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'settings-resolve-dup' }));

        await screen.findByTestId('dup-modal');
        vi.mocked(AnalyzeGaItemRepack).mockClear();
        fireEvent.click(screen.getByRole('button', { name: 'dup-refresh' }));

        // The normal post-mutation refresh runs, but optimization is never opened
        // or analyzed automatically.
        await new Promise(r => setTimeout(r, 0));
        expect(AnalyzeGaItemRepack).not.toHaveBeenCalled();
        expect(screen.queryByText('Ready to optimize')).not.toBeInTheDocument();
    });
});

describe('App diagnostic debug-mode sync', () => {
    beforeEach(() => localStorage.clear());
    afterEach(() => vi.clearAllMocks());

    it('pushes the initial debug-mode value to the backend on mount', async () => {
        renderApp('safe');
        await waitFor(() => expect(SetDiagnosticDebugMode).toHaveBeenCalledWith(false));
    });

    it('syncs the new value when debug mode is toggled on', async () => {
        renderApp('safe');
        await waitFor(() => expect(SetDiagnosticDebugMode).toHaveBeenCalledWith(false));
        fireEvent.click(screen.getByRole('button', { name: /tools/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'settings-enable-debug' }));
        await waitFor(() => expect(SetDiagnosticDebugMode).toHaveBeenCalledWith(true));
    });
});
