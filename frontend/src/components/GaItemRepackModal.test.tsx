import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../wailsjs/go/main/App', () => ({
    AnalyzeGaItemRepack: vi.fn(),
    ExecuteGaItemRepack: vi.fn(),
}));

import { GaItemRepackModal } from './GaItemRepackModal';
import { AnalyzeGaItemRepack, ExecuteGaItemRepack } from '../../wailsjs/go/main/App';

const analyze = AnalyzeGaItemRepack as ReturnType<typeof vi.fn>;
const execute = ExecuteGaItemRepack as ReturnType<typeof vi.fn>;

const CAP = (physicalEmpty: number, cursorRoom: number, usable: number) => ({ physicalEmpty, cursorRoom, usable });

function readyAnalysis(overrides: Record<string, unknown> = {}) {
    return {
        outcome: 'ready',
        characterIndex: 1,
        analysisToken: 'tok-1',
        before: CAP(2, 3, 40),
        projectedAfter: CAP(7, 8, 45),
        recovered: 5,
        nonEmptyRecords: 123,
        blockers: [],
        ...overrides,
    };
}

function renderModal(props: Partial<React.ComponentProps<typeof GaItemRepackModal>> = {}) {
    const p = {
        charIndex: 1,
        characterName: 'Tarnished',
        onWriteSave: vi.fn().mockResolvedValue(undefined),
        onRefresh: vi.fn(),
        onCloseSaveWithoutSaving: vi.fn().mockResolvedValue(undefined),
        onClose: vi.fn(),
        ...props,
    };
    render(<GaItemRepackModal {...p} />);
    return p;
}

beforeEach(() => {
    analyze.mockReset();
    execute.mockReset();
});

describe('GaItemRepackModal dry-run', () => {
    it('analyzes for the given charIndex on open', async () => {
        analyze.mockResolvedValue(readyAnalysis());
        renderModal({ charIndex: 3 });
        expect(analyze).toHaveBeenCalledWith(3);
        await screen.findByText('Ready to optimize');
    });

    it('ready shows API metrics and enables Continue only', async () => {
        analyze.mockResolvedValue(readyAnalysis());
        renderModal();
        await screen.findByText('Ready to optimize');
        // API-provided capacity values are rendered verbatim.
        expect(screen.getByText('Usable GaItem capacity')).toBeInTheDocument();
        expect(screen.getByText('45')).toBeInTheDocument(); // projected usable
        expect(screen.getByText('+5')).toBeInTheDocument(); // recovered
        expect(screen.getByText(/123 non-empty GaItem records/)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /^Continue$/ })).toBeInTheDocument();
    });

    it('no_op recovers nothing and offers no Continue', async () => {
        analyze.mockResolvedValue(readyAnalysis({ outcome: 'no_op', analysisToken: '', recovered: 0, projectedAfter: CAP(2, 3, 40) }));
        renderModal();
        await screen.findByText('Native hole allocation is active');
        expect(screen.getByText('Every free physical GaItem index is already usable. Moving records cannot recover additional capacity, so no changes are needed.')).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /^Continue$/ })).not.toBeInTheDocument();
    });

    it('refusal lists every blocker and offers no Continue', async () => {
        analyze.mockResolvedValue(readyAnalysis({
            outcome: 'refusal', analysisToken: '', projectedAfter: undefined, recovered: 0,
            blockers: [
                { code: 'orphan_handle', message: 'Inventory references a missing record.' },
                { code: 'shared_handle', message: 'Two records share a handle.' },
            ],
        }));
        renderModal();
        await screen.findByText('Optimization unavailable');
        expect(screen.getByText('orphan_handle')).toBeInTheDocument();
        expect(screen.getByText('Two records share a handle.')).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /^Continue$/ })).not.toBeInTheDocument();
    });

    it('duplicate_handle refusal with a nonzero handle opens the shared modal via callback', async () => {
        const onResolveDuplicateGaItem = vi.fn();
        analyze.mockResolvedValue(readyAnalysis({
            outcome: 'refusal', analysisToken: '', projectedAfter: undefined, recovered: 0,
            blockers: [
                { code: 'duplicate_handle', message: 'GaItem[2] reuses handle 0x80000102.', handle: 0x80000102 },
            ],
        }));
        renderModal({ onResolveDuplicateGaItem });
        await screen.findByText('Optimization unavailable');
        fireEvent.click(screen.getByRole('button', { name: /Resolve duplicate GaItem/ }));
        expect(onResolveDuplicateGaItem).toHaveBeenCalledWith(0x80000102);
    });

    it('does not show the duplicate CTA for a non-duplicate blocker or a zero handle', async () => {
        const onResolveDuplicateGaItem = vi.fn();
        analyze.mockResolvedValue(readyAnalysis({
            outcome: 'refusal', analysisToken: '', projectedAfter: undefined, recovered: 0,
            blockers: [
                { code: 'orphan_handle', message: 'Inventory references a missing record.' },
                { code: 'duplicate_handle', message: 'A container duplicate with no physical handle.', handle: 0 },
            ],
        }));
        renderModal({ onResolveDuplicateGaItem });
        await screen.findByText('Optimization unavailable');
        expect(screen.queryByRole('button', { name: /Resolve duplicate GaItem/ })).not.toBeInTheDocument();
    });

    it('unavailable shows the failure message and no Continue', async () => {
        analyze.mockResolvedValue(readyAnalysis({
            outcome: 'unavailable', analysisToken: '', projectedAfter: undefined, recovered: 0,
            failure: { stage: 'app', code: 'inventory_edit_session_active', message: 'Save or discard the active Inventory Workspace first.' },
        }));
        renderModal();
        await screen.findByText('Save or discard the active Inventory Workspace first.');
        expect(screen.queryByRole('button', { name: /^Continue$/ })).not.toBeInTheDocument();
    });

    it('rejected analysis shows the could-not-analyze state with no Continue', async () => {
        analyze.mockRejectedValue(new Error('no save'));
        renderModal();
        await screen.findByText('Could not analyze GaItem allocation');
        expect(screen.getByText('The slot and the save file were not changed.')).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /^Continue$/ })).not.toBeInTheDocument();
    });
});

describe('GaItemRepackModal CTA context', () => {
    it('renders the rejected-batch message while analysis is in progress', async () => {
        analyze.mockReturnValue(new Promise(() => {})); // never resolves — stays analyzing
        renderModal({ ctaContext: { neededGaItems: 12 } });
        await screen.findByText(/An add batch that needed 12 GaItem allocation records was rejected/);
    });

    it('renders the retry-after-optimization message in Ready to optimize', async () => {
        analyze.mockResolvedValue(readyAnalysis());
        renderModal({ ctaContext: { neededGaItems: 12 } });
        await screen.findByText('Ready to optimize');
        expect(screen.getByText('After optimizing, try adding the rejected batch again.')).toBeInTheDocument();
    });

    it('renders neither message without context', async () => {
        analyze.mockResolvedValue(readyAnalysis());
        renderModal();
        await screen.findByText('Ready to optimize');
        expect(screen.queryByText(/was rejected/)).not.toBeInTheDocument();
        expect(screen.queryByText('After optimizing, try adding the rejected batch again.')).not.toBeInTheDocument();
    });
});

describe('GaItemRepackModal confirm & execute', () => {
    async function toConfirm() {
        analyze.mockResolvedValue(readyAnalysis());
        const p = renderModal();
        await screen.findByText('Ready to optimize');
        fireEvent.click(screen.getByRole('button', { name: /^Continue$/ }));
        await screen.findByText('Optimize GaItem allocation?');
        return p;
    }

    it('Cancel returns to the dry-run without executing', async () => {
        await toConfirm();
        fireEvent.click(screen.getByRole('button', { name: /^Cancel$/ }));
        await screen.findByText('Ready to optimize');
        expect(execute).not.toHaveBeenCalled();
    });

    it('executes exactly once with the current index and token, deduping clicks', async () => {
        let resolve!: (v: unknown) => void;
        execute.mockReturnValue(new Promise(res => { resolve = res; }));
        await toConfirm();

        const optimize = screen.getByRole('button', { name: /^Optimize allocation$/ });
        fireEvent.click(optimize);
        await screen.findByText('Optimizing GaItem allocation…');
        expect(execute).toHaveBeenCalledTimes(1);
        expect(execute).toHaveBeenCalledWith({ characterIndex: 1, analysisToken: 'tok-1' });

        resolve({ outcome: 'success', characterIndex: 1, before: CAP(2, 3, 40), after: CAP(7, 8, 45), recovered: 5 });
        await screen.findByText('GaItem allocation optimized');
    });

    it('blocks closing while optimizing', async () => {
        execute.mockReturnValue(new Promise(() => {})); // never resolves
        const p = await toConfirm();
        fireEvent.click(screen.getByRole('button', { name: /^Optimize allocation$/ }));
        await screen.findByText('Optimizing GaItem allocation…');
        // No close affordance, and Escape is ignored.
        expect(screen.queryByRole('button', { name: /Close/i })).not.toBeInTheDocument();
        fireEvent.keyDown(window, { key: 'Escape' });
        expect(p.onClose).not.toHaveBeenCalled();
    });
});

describe('GaItemRepackModal results', () => {
    async function execTo(result: Record<string, unknown>, props: Partial<React.ComponentProps<typeof GaItemRepackModal>> = {}) {
        analyze.mockResolvedValue(readyAnalysis());
        execute.mockResolvedValue(result);
        const p = renderModal(props);
        await screen.findByText('Ready to optimize');
        fireEvent.click(screen.getByRole('button', { name: /^Continue$/ }));
        await screen.findByText('Optimize GaItem allocation?');
        fireEvent.click(screen.getByRole('button', { name: /^Optimize allocation$/ }));
        return p;
    }

    it('success refreshes App and Write Save uses the central callback', async () => {
        const p = await execTo({ outcome: 'success', characterIndex: 1, before: CAP(2, 3, 40), after: CAP(7, 8, 45), recovered: 5 });
        await screen.findByText('GaItem allocation optimized');
        expect(p.onRefresh).toHaveBeenCalledTimes(1);
        expect(screen.getByText(/The active slot has changed in memory/)).toBeInTheDocument();

        // Success shows realised values under an "After" header, not "After (projected)".
        expect(screen.getByText('After')).toBeInTheDocument();
        expect(screen.queryByText('After (projected)')).not.toBeInTheDocument();

        fireEvent.click(screen.getByRole('button', { name: /^Write Save$/ }));
        await waitFor(() => expect(p.onWriteSave).toHaveBeenCalledTimes(1));
        await waitFor(() => expect(p.onClose).toHaveBeenCalled());
    });

    it('rolled_back reports restoration and offers only Close', async () => {
        const p = await execTo({
            outcome: 'rolled_back', characterIndex: 1, before: CAP(2, 3, 40), recovered: 0,
            failure: { stage: 'validation', code: 'postcondition_mismatch', message: 'Result did not match analysis.' },
            rollback: { attempted: true, complete: true, mode: 'discard_candidate' },
        });
        await screen.findByText('GaItem allocation failed — changes rolled back');
        expect(screen.getByText(/The active slot was restored/)).toBeInTheDocument();
        expect(p.onRefresh).not.toHaveBeenCalled();
        expect(screen.queryByRole('button', { name: /^Write Save$/ })).not.toBeInTheDocument();
        expect(screen.getByRole('button', { name: /^Close$/ })).toBeInTheDocument();
    });

    it('could_not_start allows re-analysis and Close, and never rollback wording', async () => {
        const p = await execTo({
            outcome: 'could_not_start', characterIndex: 1, before: CAP(2, 3, 40), recovered: 0,
            failure: { stage: 'app', code: 'analysis_stale', message: 'Run analysis again.' },
        });
        await screen.findByText('GaItem allocation could not start');
        expect(screen.getByText('Nothing changed in memory or on disk.')).toBeInTheDocument();
        expect(p.onRefresh).not.toHaveBeenCalled();
        // Re-analysis restarts the dry-run.
        analyze.mockResolvedValue(readyAnalysis());
        fireEvent.click(screen.getByRole('button', { name: /Run analysis again/ }));
        await screen.findByText('Ready to optimize');
        expect(analyze).toHaveBeenCalledTimes(2);
    });

    it('rollback_failed forbids save/close, offers copy + guarded close-save', async () => {
        const clip = vi.fn().mockResolvedValue(undefined);
        Object.assign(navigator, { clipboard: { writeText: clip } });
        const p = await execTo({
            outcome: 'rollback_failed', characterIndex: 1, before: CAP(2, 3, 40), recovered: 0,
            failure: { stage: 'transform', code: 'repack_failed', message: 'Transform crashed.' },
            rollback: { attempted: true, complete: false, mode: 'discard_candidate', failure: { stage: 'rollback', code: 'original_state_changed', message: 'Original changed.' } },
        });
        await screen.findByText('GaItem allocation failed — do not save this session');
        // No normal Close / Write Save / retry.
        expect(screen.queryByRole('button', { name: /^Close$/ })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /^Write Save$/ })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Run analysis again/ })).not.toBeInTheDocument();
        // Escape stays blocked.
        fireEvent.keyDown(window, { key: 'Escape' });
        expect(p.onClose).not.toHaveBeenCalled();

        fireEvent.click(screen.getByRole('button', { name: /Copy diagnostic details/ }));
        await waitFor(() => expect(clip).toHaveBeenCalled());
        expect(clip.mock.calls[0][0]).toContain('rollback_failed');

        // Close-save requires a second explicit confirmation.
        fireEvent.click(screen.getByRole('button', { name: /Close save without saving/ }));
        expect(p.onCloseSaveWithoutSaving).not.toHaveBeenCalled();
        fireEvent.click(screen.getByRole('button', { name: /Confirm — close without saving/ }));
        await waitFor(() => expect(p.onCloseSaveWithoutSaving).toHaveBeenCalledTimes(1));
    });

    it('rollback_failed surfaces a close-save failure and allows a retry', async () => {
        Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
        const onCloseSaveWithoutSaving = vi.fn()
            .mockRejectedValueOnce(new Error('disk busy'))
            .mockResolvedValueOnce(undefined);
        const p = await execTo({
            outcome: 'rollback_failed', characterIndex: 1, before: CAP(2, 3, 40), recovered: 0,
            failure: { stage: 'transform', code: 'repack_failed', message: 'Transform crashed.' },
            rollback: { attempted: true, complete: false, mode: 'discard_candidate' },
        }, { onCloseSaveWithoutSaving });
        await screen.findByText('GaItem allocation failed — do not save this session');

        fireEvent.click(screen.getByRole('button', { name: /Close save without saving/ }));
        fireEvent.click(screen.getByRole('button', { name: /Confirm — close without saving/ }));
        // Rejected callback: error is shown, no unhandled rejection, modal stays critical.
        await screen.findByText(/Could not close the save without saving/);
        expect(p.onClose).not.toHaveBeenCalled();
        expect(screen.queryByRole('button', { name: /^Close$/ })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /^Write Save$/ })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Run analysis again/ })).not.toBeInTheDocument();

        // The button is retryable and a second attempt can succeed.
        fireEvent.click(screen.getByRole('button', { name: /Confirm — close without saving/ }));
        await waitFor(() => expect(onCloseSaveWithoutSaving).toHaveBeenCalledTimes(2));
    });

    it('rollback_failed keeps the Copy label when the Clipboard API is unavailable', async () => {
        Object.assign(navigator, { clipboard: undefined });
        await execTo({
            outcome: 'rollback_failed', characterIndex: 1, before: CAP(2, 3, 40), recovered: 0,
            failure: { stage: 'transform', code: 'repack_failed', message: 'Transform crashed.' },
            rollback: { attempted: true, complete: false, mode: 'discard_candidate' },
        });
        await screen.findByText('GaItem allocation failed — do not save this session');

        fireEvent.click(screen.getByRole('button', { name: /Copy diagnostic details/ }));
        // Nothing was copied, so the button must not flip to "Copied".
        await waitFor(() => expect(screen.queryByRole('button', { name: /^Copied$/ })).not.toBeInTheDocument());
        expect(screen.getByRole('button', { name: /Copy diagnostic details/ })).toBeInTheDocument();
    });
});
