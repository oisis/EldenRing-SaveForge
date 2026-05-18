import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../../../wailsjs/go/main/App', () => ({
    ExportBuildTemplateToFile: vi.fn(),
    SaveBuildTemplateToLibrary: vi.fn(),
}));

import * as App from '../../../../wailsjs/go/main/App';
import { ExportTemplateModal, formatWarningsSummary } from '../ExportTemplateModal';

const mocks = App as unknown as Record<string, ReturnType<typeof vi.fn>>;

beforeEach(() => {
    Object.values(mocks).forEach(m => typeof m?.mockReset === 'function' && m.mockReset());
});

afterEach(() => {
    vi.clearAllMocks();
});

function defaultProps(overrides: Partial<Parameters<typeof ExportTemplateModal>[0]> = {}) {
    return {
        sessionID: 'ses-test',
        dirty: false,
        onClose: vi.fn(),
        onSuccess: vi.fn(),
        onError: vi.fn(),
        ...overrides,
    };
}

describe('ExportTemplateModal', () => {
    it('renders name/description/tags fields and section checkboxes', () => {
        render(<ExportTemplateModal {...defaultProps()} />);
        expect(screen.getByLabelText(/^Include inventory$/)).toBeInTheDocument();
        expect(screen.getByLabelText(/^Include storage$/)).toBeInTheDocument();
        expect(screen.getByLabelText(/^Name$/)).toBeInTheDocument();
        expect(screen.getByLabelText(/^Description$/)).toBeInTheDocument();
        expect(screen.getByLabelText(/^Author$/)).toBeInTheDocument();
        expect(screen.getByLabelText(/Tags/)).toBeInTheDocument();
    });

    it('shows unsaved-state note only when dirty=true', () => {
        const { rerender } = render(<ExportTemplateModal {...defaultProps({ dirty: false })} />);
        expect(screen.queryByTestId('export-dirty-note')).not.toBeInTheDocument();
        rerender(<ExportTemplateModal {...defaultProps({ dirty: true })} />);
        expect(screen.getByTestId('export-dirty-note')).toBeInTheDocument();
    });

    it('calls ExportBuildTemplateToFile with checkbox + metadata values', async () => {
        mocks.ExportBuildTemplateToFile.mockResolvedValue({
            path: '/tmp/build.json',
            warnings: [],
            skippedItems: 0,
        });
        const onSuccess = vi.fn();
        render(<ExportTemplateModal {...defaultProps({ onSuccess })} />);

        // Uncheck storage, fill name + tags
        fireEvent.click(screen.getByLabelText(/^Include storage$/));
        fireEvent.change(screen.getByLabelText(/^Name$/), { target: { value: 'My Build' } });
        fireEvent.change(screen.getByLabelText(/Tags/), { target: { value: 'pvp, rl150' } });

        await act(async () => {
            fireEvent.click(screen.getByRole('button', { name: /Export JSON file/i }));
        });

        await waitFor(() => {
            expect(mocks.ExportBuildTemplateToFile).toHaveBeenCalledTimes(1);
        });
        const call = mocks.ExportBuildTemplateToFile.mock.calls[0];
        expect(call[0]).toBe('ses-test');
        const opts = call[1] as {
            includeInventory: boolean;
            includeStorage: boolean;
            name: string;
            tags: string[];
        };
        expect(opts.includeInventory).toBe(true);
        expect(opts.includeStorage).toBe(false);
        expect(opts.name).toBe('My Build');
        expect(opts.tags).toEqual(['pvp', 'rl150']);
        expect(onSuccess).toHaveBeenCalled();
    });

    it('disables Export button when both sections excluded', () => {
        render(<ExportTemplateModal {...defaultProps()} />);
        fireEvent.click(screen.getByLabelText(/^Include inventory$/));
        fireEvent.click(screen.getByLabelText(/^Include storage$/));
        const btn = screen.getByRole('button', { name: /Export JSON file/i });
        expect(btn).toBeDisabled();
    });

    it('hides "Save to local library" button when onSavedToLibrary is omitted', () => {
        render(<ExportTemplateModal {...defaultProps()} />);
        expect(screen.queryByTestId('export-save-to-library')).not.toBeInTheDocument();
    });

    it('shows "Save to local library" button when onSavedToLibrary is provided', () => {
        render(<ExportTemplateModal {...defaultProps({ onSavedToLibrary: vi.fn() })} />);
        expect(screen.getByTestId('export-save-to-library')).toBeInTheDocument();
    });

    it('calls SaveBuildTemplateToLibrary when the library button is clicked', async () => {
        mocks.SaveBuildTemplateToLibrary.mockResolvedValue({ id: 'lib-1', name: 'My Build' });
        const onSavedToLibrary = vi.fn();
        render(<ExportTemplateModal {...defaultProps({ onSavedToLibrary })} />);
        fireEvent.change(screen.getByLabelText(/^Name$/), { target: { value: 'My Build' } });
        await act(async () => {
            fireEvent.click(screen.getByTestId('export-save-to-library'));
        });
        await waitFor(() => {
            expect(mocks.SaveBuildTemplateToLibrary).toHaveBeenCalledTimes(1);
        });
        const call = mocks.SaveBuildTemplateToLibrary.mock.calls[0];
        expect(call[0]).toBe('ses-test');
        expect((call[1] as { name: string }).name).toBe('My Build');
        expect(onSavedToLibrary).toHaveBeenCalledWith(expect.objectContaining({ id: 'lib-1' }));
    });

    it('routes thrown errors through onError', async () => {
        mocks.ExportBuildTemplateToFile.mockRejectedValue(new Error('disk full'));
        const onError = vi.fn();
        render(<ExportTemplateModal {...defaultProps({ onError })} />);
        await act(async () => {
            fireEvent.click(screen.getByRole('button', { name: /Export JSON file/i }));
        });
        await waitFor(() => {
            expect(onError).toHaveBeenCalled();
        });
    });
});

describe('formatWarningsSummary', () => {
    it('returns null for empty/undefined warnings', () => {
        expect(formatWarningsSummary(undefined)).toBeNull();
        expect(formatWarningsSummary([])).toBeNull();
    });

    it('returns single-warning message verbatim', () => {
        const out = formatWarningsSummary([
            { code: 'aow_missing_skipped', message: 'weapon X has dangling AoW', position: 0 } as never,
        ]);
        expect(out).toContain('1 warning');
        expect(out).toContain('weapon X has dangling AoW');
    });

    it('returns aggregate count for multiple warnings', () => {
        const out = formatWarningsSummary([
            { code: 'a', message: 'x', position: 0 } as never,
            { code: 'b', message: 'y', position: 0 } as never,
        ]);
        expect(out).toContain('2 warnings');
    });
});
