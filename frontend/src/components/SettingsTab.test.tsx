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
}));

// Stub the modal to a marker so we can assert it opened with the scan report.
vi.mock('./InventoryIssuesModal', () => ({
    InventoryIssuesModal: (props: { reports: RepairIssueReport[] }) =>
        <div data-testid="inv-issues-modal">reports:{props.reports.length}</div>,
}));
vi.mock('./SaveManagerModal', () => ({ SaveManagerModal: () => null }));
vi.mock('./FavoritesManager', () => ({ FavoritesManager: () => null }));

import { SettingsTab } from './SettingsTab';
import { SafetyModeProvider } from '../state/safetyMode';
import { FavoritesProvider } from '../state/favorites';
import toast from '../lib/toast';

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

function renderSettings(charIndex = 2) {
    render(
        <SafetyModeProvider>
            <FavoritesProvider>
                <SettingsTab
                    theme="dark" setTheme={vi.fn()}
                    columnVisibility={{ id: false, category: false }} setColumnVisibility={vi.fn()}
                    showFlaggedItems={false} setShowFlaggedItems={vi.fn()}
                    debugMode={false} setDebugMode={vi.fn()}
                    platform="steam"
                    selectedDeployTarget="" setSelectedDeployTarget={vi.fn()}
                    onAfterLoad={vi.fn()}
                    charIndex={charIndex}
                    onComplete={vi.fn()}
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
