import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

const RepairAllLoadedSlots = vi.fn();

vi.mock('../../wailsjs/go/main/App', () => ({
    RepairAllLoadedSlots: () => RepairAllLoadedSlots(),
}));

// Imported after the mock so DiagnosticsModal binds to the mocked endpoint.
import { DiagnosticsModal } from './DiagnosticsModal';
import { main } from '../../wailsjs/go/models';

function reportWithIssue(): main.DiagnosticsReport {
    return new main.DiagnosticsReport({
        source: 'loaded',
        canRepair: true,
        slots: [{
            slotIndex: 0,
            charName: 'Tarnished',
            issues: [{ category: 'inventory', severity: 'warning', description: 'Duplicate inventory index' }],
        }],
    });
}

describe('DiagnosticsModal (post-load all-loaded report)', () => {
    it('renders the initial all-loaded report issues', () => {
        render(<DiagnosticsModal initialReport={reportWithIssue()} onClose={vi.fn()} />);
        expect(screen.getByText('Duplicate inventory index')).toBeInTheDocument();
        expect(screen.getByText(/Tarnished/)).toBeInTheDocument();
    });

    it('has no loaded/external choice controls', () => {
        render(<DiagnosticsModal initialReport={reportWithIssue()} onClose={vi.fn()} />);
        expect(screen.queryByText('Loaded save')).not.toBeInTheDocument();
        expect(screen.queryByText('External file')).not.toBeInTheDocument();
    });

    it('applies repairs via RepairAllLoadedSlots', async () => {
        RepairAllLoadedSlots.mockResolvedValue({ fixed: ['Fixed duplicate index'], skipped: [] });
        render(<DiagnosticsModal initialReport={reportWithIssue()} onClose={vi.fn()} />);

        fireEvent.click(screen.getByRole('button', { name: 'Repair' }));

        await waitFor(() => expect(RepairAllLoadedSlots).toHaveBeenCalledTimes(1));
        expect(await screen.findByText('Fixed duplicate index')).toBeInTheDocument();
    });
});
