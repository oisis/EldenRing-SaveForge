import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../wailsjs/go/main/App', () => ({
    AnalyzeGaItemDuplicate: vi.fn(),
    ExecuteGaItemDuplicateRepair: vi.fn(),
}));

import { GaItemDuplicateRepairModal } from './GaItemDuplicateRepairModal';
import { AnalyzeGaItemDuplicate, ExecuteGaItemDuplicateRepair } from '../../wailsjs/go/main/App';

const analyze = AnalyzeGaItemDuplicate as ReturnType<typeof vi.fn>;
const execute = ExecuteGaItemDuplicateRepair as ReturnType<typeof vi.fn>;

const HANDLE = 0x80000102;

function candidate(index: number, itemId: number, extra: Record<string, unknown> = {}) {
    return { index, itemId, ...extra };
}

function readyAnalysis(overrides: Record<string, unknown> = {}) {
    return {
        outcome: 'ready',
        characterIndex: 1,
        handle: HANDLE,
        analysisToken: 'dedup-tok-1',
        candidates: [
            candidate(1, 0x000F4240, { name: 'Dagger', currentUpgrade: 0 }),
            candidate(2, 0x000F4241, { name: 'Dagger', currentUpgrade: 1 }),
        ],
        ...overrides,
    };
}

function renderModal(props: Partial<React.ComponentProps<typeof GaItemDuplicateRepairModal>> = {}) {
    const p = {
        charIndex: 1,
        characterName: 'Tarnished',
        handle: HANDLE,
        onRefresh: vi.fn(),
        onClose: vi.fn(),
        ...props,
    };
    render(<GaItemDuplicateRepairModal {...p} />);
    return p;
}

beforeEach(() => {
    analyze.mockReset();
    execute.mockReset();
});

describe('GaItemDuplicateRepairModal ready', () => {
    it('analyzes with charIndex and handle on open', async () => {
        analyze.mockResolvedValue(readyAnalysis());
        renderModal({ charIndex: 3, handle: HANDLE });
        expect(analyze).toHaveBeenCalledWith(3, HANDLE);
        await screen.findByText('Physical index 1');
    });

    it('renders both candidates and requires an explicit selection before executing', async () => {
        analyze.mockResolvedValue(readyAnalysis());
        renderModal();
        await screen.findByText('Physical index 1');

        // Both candidate labels render; neither is preselected.
        const cards = screen.getAllByRole('button', { name: /Dagger/ });
        expect(cards).toHaveLength(2);
        expect(cards.every(c => c.getAttribute('aria-pressed') === 'false')).toBe(true);

        // Execute is disabled until the user picks a candidate.
        const remove = screen.getByRole('button', { name: /^Remove duplicate$/ });
        expect(remove).toBeDisabled();
        fireEvent.click(remove);
        expect(execute).not.toHaveBeenCalled();

        fireEvent.click(cards[1]);
        expect(remove).not.toBeDisabled();
    });

    it('sends the selected keepIndex and analysis token exactly', async () => {
        analyze.mockResolvedValue(readyAnalysis());
        execute.mockReturnValue(new Promise(() => {})); // never resolves — stays executing
        renderModal({ charIndex: 1 });
        await screen.findByText('Physical index 2');

        fireEvent.click(screen.getAllByRole('button', { name: /Dagger/ })[0]); // keepIndex 1
        fireEvent.click(screen.getByRole('button', { name: /^Remove duplicate$/ }));

        await screen.findByText('Removing duplicate GaItem…');
        expect(execute).toHaveBeenCalledTimes(1);
        expect(execute).toHaveBeenCalledWith({
            characterIndex: 1,
            handle: HANDLE,
            keepIndex: 1,
            analysisToken: 'dedup-tok-1',
        });
    });

    it('uses the hexadecimal fallback for an unknown candidate', async () => {
        analyze.mockResolvedValue(readyAnalysis({
            candidates: [
                candidate(1, 0xEEEEEEEE, { unknown: true }),
                candidate(2, 0x000F4241, { name: 'Dagger', currentUpgrade: 1 }),
            ],
        }));
        renderModal();
        await screen.findByText('Unknown item · 0xEEEEEEEE');
        expect(screen.getByText('Dagger +1')).toBeInTheDocument();
    });
});

describe('GaItemDuplicateRepairModal refusal / failure', () => {
    it('renders a backend refusal safely with no execute action', async () => {
        analyze.mockResolvedValue({
            outcome: 'refusal',
            characterIndex: 1,
            handle: HANDLE,
            candidates: [],
            refusalCode: 'not_a_duplicate',
            refusalMessage: 'This handle is not a physical duplicate.',
        });
        renderModal();
        await screen.findByText('Duplicate repair unavailable');
        expect(screen.getByText('This handle is not a physical duplicate.')).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Remove duplicate/ })).not.toBeInTheDocument();
    });

    it('renders an unavailable failure without execute', async () => {
        analyze.mockResolvedValue({
            outcome: 'unavailable',
            characterIndex: 1,
            handle: HANDLE,
            candidates: [],
            failure: { stage: 'app', code: 'inventory_edit_session_active', message: 'Save or discard the workspace first.' },
        });
        renderModal();
        await screen.findByText('Save or discard the workspace first.');
        expect(screen.queryByRole('button', { name: /Remove duplicate/ })).not.toBeInTheDocument();
    });

    it('renders a rejected analysis without execute', async () => {
        analyze.mockRejectedValue(new Error('no save'));
        renderModal();
        await screen.findByText('Could not analyze duplicate GaItem');
        expect(screen.queryByRole('button', { name: /Remove duplicate/ })).not.toBeInTheDocument();
    });
});

describe('GaItemDuplicateRepairModal success', () => {
    async function execTo(result: Record<string, unknown>) {
        analyze.mockResolvedValue(readyAnalysis());
        execute.mockResolvedValue(result);
        const p = renderModal();
        await screen.findByText('Physical index 1');
        fireEvent.click(screen.getAllByRole('button', { name: /Dagger/ })[0]);
        fireEvent.click(screen.getByRole('button', { name: /^Remove duplicate$/ }));
        return p;
    }

    it('refreshes App once after a successful repair', async () => {
        const p = await execTo({
            outcome: 'success', characterIndex: 1, handle: HANDLE,
            keptIndex: 1, removedIndex: 2,
        });
        await screen.findByText('Duplicate GaItem resolved');
        expect(p.onRefresh).toHaveBeenCalledTimes(1);
        expect(screen.getByText(/No file has been written yet/)).toBeInTheDocument();
    });

    it('rolled_back reports restoration and does not refresh', async () => {
        const p = await execTo({
            outcome: 'rolled_back', characterIndex: 1, handle: HANDLE, keptIndex: 1, removedIndex: 2,
            failure: { stage: 'transform', code: 'repair_failed', message: 'Transform failed.' },
            rollback: { attempted: true, complete: true, mode: 'discard_candidate' },
        });
        await screen.findByText('Duplicate repair failed — changes rolled back');
        expect(p.onRefresh).not.toHaveBeenCalled();
    });
});
