import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import type { RepairIssueReport } from '../lib/repairIssues';

const scanRepairIssuesLoaded = vi.fn<(idx: number) => Promise<RepairIssueReport>>();

vi.mock('../lib/repairIssues', () => ({
    scanRepairIssuesLoaded: (idx: number) => scanRepairIssuesLoaded(idx),
}));

vi.mock('../lib/toast', () => {
    const fn = vi.fn() as unknown as Record<string, unknown> & ((...args: unknown[]) => void);
    fn.success = vi.fn();
    fn.error = vi.fn();
    fn.loading = vi.fn();
    return { default: fn };
});

vi.mock('../../wailsjs/go/main/App', () => ({
    GetSteamIDString: vi.fn().mockResolvedValue(''),
    SetSteamIDFromString: vi.fn(),
    GetDeployTargets: vi.fn().mockResolvedValue([]),
    SaveDeployTarget: vi.fn(),
    DeleteDeployTarget: vi.fn(),
    TestSSHConnection: vi.fn(),
    DeploySave: vi.fn(),
    DownloadRemoteSave: vi.fn(),
    LaunchRemoteGame: vi.fn(),
    CloseRemoteGame: vi.fn(),
    DeployAndLaunch: vi.fn(),
    CloseAndDownload: vi.fn(),
    PrepareConversion: vi.fn(),
    ExecuteConversion: vi.fn(),
    BackupCurrentSave: vi.fn().mockResolvedValue(undefined),
}));

// Stub the modal to a marker so we can assert it opened with the scan report.
// It also exposes the threaded duplicate-repair callback for one test.
vi.mock('./InventoryIssuesModal', () => ({
    InventoryIssuesModal: (props: { reports: RepairIssueReport[]; onResolveDuplicateGaItem?: (s: number, h: number) => void }) => (
        <div data-testid="inv-issues-modal">
            reports:{props.reports.length}
            {props.onResolveDuplicateGaItem && (
                <button onClick={() => props.onResolveDuplicateGaItem!(0, 0x80000102)}>stub-resolve-dup</button>
            )}
        </div>
    ),
}));
vi.mock('./SaveManagerModal', () => ({ SaveManagerModal: () => null }));
vi.mock('./FavoritesManager', () => ({ FavoritesManager: () => null }));

import { SettingsTab } from './SettingsTab';
import { SafetyModeProvider } from '../state/safetyMode';
import { FavoritesProvider } from '../state/favorites';
import type { SafetyProfile } from '../state/safetyProfile';
import toast from '../lib/toast';
import { PrepareConversion } from '../../wailsjs/go/main/App';

// jsdom here has no localStorage; SettingsTab reads it for the Full Chaos toggle.
const lsStore: Record<string, string> = {};
vi.stubGlobal('localStorage', {
    getItem: (k: string) => lsStore[k] ?? null,
    setItem: (k: string, v: string) => { lsStore[k] = v; },
    removeItem: (k: string) => { delete lsStore[k]; },
});

function fakeReport(hasIssues: boolean): RepairIssueReport {
    return {
        slotIndex: 0,
        charName: 'Tarnished',
        hasIssues,
        issues: [],
        coverage: {
            totalPhysical: 5, resolved: 5, knownDB: 5, technicalPlaceholder: 0, unknown: 0,
            resolutionChecksApplied: 5, structuralChecksApplied: 5, categoryChecksApplied: 5,
            perCategory: {}, unknownByReason: {},
        },
    };
}

function renderSettings(charIndex = 2, safetyProfile: SafetyProfile = 'safe', onOptimizeGaItem = vi.fn(), onResolveDuplicateGaItem = vi.fn()) {
    render(
        <SafetyModeProvider>
            <FavoritesProvider>
                <SettingsTab
                    theme="dark" setTheme={vi.fn()}
                    columnVisibility={{ id: false, category: false }} setColumnVisibility={vi.fn()}
                    safetyProfile={safetyProfile}
                    debugMode={false} setDebugMode={vi.fn()}
                    platform="steam"
                    selectedDeployTarget="" setSelectedDeployTarget={vi.fn()}
                    onAfterLoad={vi.fn()}
                    charIndex={charIndex}
                    onComplete={vi.fn()}
                    onOptimizeGaItem={onOptimizeGaItem}
                    onResolveDuplicateGaItem={onResolveDuplicateGaItem}
                />
            </FavoritesProvider>
        </SafetyModeProvider>,
    );
}

describe('SettingsTab diagnostics', () => {
    beforeEach(() => {
        scanRepairIssuesLoaded.mockReset();
        (toast.error as ReturnType<typeof vi.fn>).mockReset();
    });

    it('scans the loaded save directly and opens the modal for a clean report', async () => {
        scanRepairIssuesLoaded.mockResolvedValue(fakeReport(false));
        renderSettings(2);

        fireEvent.click(screen.getByRole('button', { name: /Diagnostics/i }));

        expect(scanRepairIssuesLoaded).toHaveBeenCalledWith(2);
        expect(await screen.findByTestId('inv-issues-modal')).toBeInTheDocument();
    });

    it('threads the duplicate-repair callback into its InventoryIssuesModal', async () => {
        const onResolveDuplicateGaItem = vi.fn();
        scanRepairIssuesLoaded.mockResolvedValue(fakeReport(true));
        renderSettings(2, 'safe', vi.fn(), onResolveDuplicateGaItem);

        fireEvent.click(screen.getByRole('button', { name: /Diagnostics/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'stub-resolve-dup' }));
        expect(onResolveDuplicateGaItem).toHaveBeenCalledWith(0, 0x80000102);
    });

    it('opens the modal for a report with issues', async () => {
        scanRepairIssuesLoaded.mockResolvedValue(fakeReport(true));
        renderSettings();

        fireEvent.click(screen.getByRole('button', { name: /Diagnostics/i }));

        expect(await screen.findByTestId('inv-issues-modal')).toBeInTheDocument();
    });

    it('shows a scanning state and disables the button while pending, deduping clicks', async () => {
        let resolveScan!: (r: RepairIssueReport) => void;
        scanRepairIssuesLoaded.mockReturnValue(new Promise<RepairIssueReport>(res => { resolveScan = res; }));
        renderSettings();

        const btn = screen.getByRole('button', { name: /Diagnostics/i });
        fireEvent.click(btn);

        const scanning = await screen.findByRole('button', { name: /Scanning/i });
        expect(scanning).toBeDisabled();

        fireEvent.click(scanning); // second click must not start another scan
        expect(scanRepairIssuesLoaded).toHaveBeenCalledTimes(1);

        resolveScan(fakeReport(false));
        await waitFor(() => expect(screen.getByTestId('inv-issues-modal')).toBeInTheDocument());
    });

    it('reports a scan failure via toast and does not open the modal', async () => {
        scanRepairIssuesLoaded.mockRejectedValue(new Error('no save loaded'));
        renderSettings();

        fireEvent.click(screen.getByRole('button', { name: /Diagnostics/i }));

        await waitFor(() => expect(toast.error).toHaveBeenCalled());
        expect(screen.queryByTestId('inv-issues-modal')).not.toBeInTheDocument();
    });
});

describe('SettingsTab Optimize GaItem allocation', () => {
    it('renders right after Diagnostics in the Tools list', () => {
        renderSettings();
        const diagnostics = screen.getByRole('button', { name: /Diagnostics/i });
        const optimize = screen.getByRole('button', { name: /Optimize GaItem allocation/i });
        expect(diagnostics.compareDocumentPosition(optimize) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        // Nothing renders between them (they are adjacent siblings).
        expect(diagnostics.nextElementSibling).toBe(optimize);
    });

    it('opens the shared modal via the App-owned callback and never analyzes itself', () => {
        const onOptimize = vi.fn();
        renderSettings(2, 'safe', onOptimize);
        fireEvent.click(screen.getByRole('button', { name: /Optimize GaItem allocation/i }));
        expect(onOptimize).toHaveBeenCalledTimes(1);
    });

    it('is disabled when no platform is loaded', () => {
        render(
            <SafetyModeProvider>
                <FavoritesProvider>
                    <SettingsTab
                        theme="dark" setTheme={vi.fn()}
                        columnVisibility={{ id: false, category: false }} setColumnVisibility={vi.fn()}
                        safetyProfile="safe"
                        debugMode={false} setDebugMode={vi.fn()}
                        platform={null}
                        selectedDeployTarget="" setSelectedDeployTarget={vi.fn()}
                        onAfterLoad={vi.fn()}
                        charIndex={0}
                        onComplete={vi.fn()}
                        onOptimizeGaItem={vi.fn()}
                    />
                </FavoritesProvider>
            </SafetyModeProvider>,
        );
        expect(screen.getByRole('button', { name: /Optimize GaItem allocation/i })).toBeDisabled();
    });
});

describe('SettingsTab Convert Format (disabled)', () => {
    beforeEach(() => {
        (PrepareConversion as ReturnType<typeof vi.fn>).mockReset();
    });

    it('renders Convert Format disabled with an unavailable tooltip', () => {
        renderSettings();
        const btn = screen.getByRole('button', { name: /Convert Format/i });
        expect(btn).toBeDisabled();
        expect(btn).toHaveAttribute('title', 'Format conversion is currently unavailable');
    });

    it('placed at the end of the Tools action list', () => {
        renderSettings();
        const convert = screen.getByRole('button', { name: /Convert Format/i });
        // Convert Format comes after the other active tool buttons in the DOM.
        for (const name of [/Favorite Items/i, /Diagnostics/i]) {
            const other = screen.getByRole('button', { name });
            expect(other.compareDocumentPosition(convert) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        }
    });

    it('cannot invoke the conversion flow when clicked', () => {
        renderSettings();
        fireEvent.click(screen.getByRole('button', { name: /Convert Format/i }));
        expect(PrepareConversion).not.toHaveBeenCalled();
    });
});

describe('SettingsTab safety profile selector', () => {
    beforeEach(() => {
        for (const k of Object.keys(lsStore)) delete lsStore[k];
    });

    it('marks the active profile as checked', () => {
        renderSettings(2, 'expanded_limits');
        expect(screen.getByRole('radio', { name: /Expanded Limits/i })).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('radio', { name: /^Safe/i })).toHaveAttribute('aria-checked', 'false');
    });

    it('selecting Expanded Limits persists it without a Chaos warning', () => {
        renderSettings(2, 'safe');
        fireEvent.click(screen.getByRole('radio', { name: /Expanded Limits/i }));
        expect(lsStore['setting:safetyProfile']).toBe('expanded_limits');
        expect(screen.queryByRole('heading', { name: /Enable Chaos Mode/i })).not.toBeInTheDocument();
    });

    it('selecting Chaos opens the warning; cancel keeps the previous profile', () => {
        renderSettings(2, 'safe');
        fireEvent.click(screen.getByRole('radio', { name: /^Chaos/i }));
        expect(screen.getByRole('heading', { name: /Enable Chaos Mode/i })).toBeInTheDocument();

        fireEvent.click(screen.getByRole('button', { name: /Cancel/i }));
        expect(lsStore['setting:safetyProfile']).toBeUndefined();
    });

    it('confirming the Chaos warning persists chaos', async () => {
        renderSettings(2, 'safe');
        fireEvent.click(screen.getByRole('radio', { name: /^Chaos/i }));
        fireEvent.click(screen.getByRole('button', { name: /^OK$/i }));

        await waitFor(() => expect(lsStore['setting:safetyProfile']).toBe('chaos'));
    });
});
