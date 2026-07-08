import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { RepairIssueReport } from '../lib/repairIssues';

const scanRepairIssuesLoaded = vi.fn<(idx: number) => Promise<RepairIssueReport>>();

vi.mock('../lib/repairIssues', () => ({
    scanRepairIssuesLoaded: (idx: number) => scanRepairIssuesLoaded(idx),
    scanRepairIssuesExternal: vi.fn(),
}));

// Imported after the mock so DiagnosticsModal binds to the mocked scan helpers.
import { DiagnosticsModal } from './DiagnosticsModal';

function fakeReport(hasIssues: boolean): RepairIssueReport {
    return {
        slotIndex: 0,
        charName: 'Tarnished',
        source: 'loaded',
        hasIssues,
        issues: [],
        coverage: {
            totalPhysical: 5, resolved: 5, knownDB: 5, technicalPlaceholder: 0, unknown: 0,
            resolutionChecksApplied: 5, structuralChecksApplied: 5, categoryChecksApplied: 5,
            perCategory: {}, unknownByReason: {},
        },
    };
}

describe('DiagnosticsModal manual loaded scan', () => {
    it('opens the inventory issues modal for a clean scan (to show coverage)', async () => {
        scanRepairIssuesLoaded.mockResolvedValue(fakeReport(false));
        const onOpen = vi.fn();
        render(
            <DiagnosticsModal charIndex={0} platform="steam" onClose={vi.fn()} onOpenInventoryIssues={onOpen} />,
        );

        fireEvent.click(screen.getByText('Loaded save'));

        await waitFor(() => expect(onOpen).toHaveBeenCalledTimes(1));
        expect(onOpen.mock.calls[0][1]).toBe('loaded');
        expect(onOpen.mock.calls[0][0][0].hasIssues).toBe(false);
    });

    it('opens the inventory issues modal for a scan with issues', async () => {
        scanRepairIssuesLoaded.mockResolvedValue(fakeReport(true));
        const onOpen = vi.fn();
        render(
            <DiagnosticsModal charIndex={0} platform="steam" onClose={vi.fn()} onOpenInventoryIssues={onOpen} />,
        );

        fireEvent.click(screen.getByText('Loaded save'));

        await waitFor(() => expect(onOpen).toHaveBeenCalledTimes(1));
        expect(onOpen.mock.calls[0][0][0].hasIssues).toBe(true);
    });
});
