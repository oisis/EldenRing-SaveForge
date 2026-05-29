import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../../../wailsjs/go/main/App', () => ({
    ListBuildTemplateLibrary: vi.fn(),
    PreviewBuildTemplateFromLibrary: vi.fn(),
    ApplyBuildTemplateFromLibrary: vi.fn(),
    DeleteBuildTemplateFromLibrary: vi.fn(),
    RenameBuildTemplateInLibrary: vi.fn(),
    ExportLibraryBuildTemplateToFile: vi.fn(),
    RebuildBuildTemplateLibraryIndex: vi.fn(),
    GetBuildTemplateLibraryPath: vi.fn(),
}));

vi.mock('../../../lib/toast', () => ({
    default: Object.assign(vi.fn(), {
        success: vi.fn(),
        error: vi.fn(),
        loading: vi.fn(),
        dismiss: vi.fn(),
    }),
}));

import * as App from '../../../../wailsjs/go/main/App';
import { TemplatesShellModal } from '../TemplatesShellModal';

const mocks = App as unknown as Record<string, ReturnType<typeof vi.fn>>;

const sampleEntries = [
    {
        id: 'tpl-1',
        name: 'RL150 Greatsword',
        description: 'PvP build',
        tags: ['pvp', 'rl150'],
        filename: 'rl150-greatsword-tpl-1.json',
        createdAt: '2026-05-01T10:00:00Z',
        updatedAt: '2026-05-10T12:34:56Z',
        inventoryItems: 12,
        storageItems: 3,
        warnings: 0,
    },
];

beforeEach(() => {
    Object.values(mocks).forEach(m => typeof m?.mockReset === 'function' && m.mockReset());
    mocks.ListBuildTemplateLibrary.mockResolvedValue(sampleEntries);
    mocks.GetBuildTemplateLibraryPath.mockResolvedValue('/fake/library');
});

afterEach(() => {
    vi.clearAllMocks();
});

describe('TemplatesShellModal', () => {
    it('renders the Templates title and the library entries', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await waitFor(() => {
            expect(mocks.ListBuildTemplateLibrary).toHaveBeenCalled();
        });
        const dialog = await screen.findByRole('dialog');
        expect(dialog).toHaveAttribute('aria-label', 'Templates');
        expect(dialog).toHaveTextContent('Templates');
        const entries = await screen.findAllByTestId('library-entry');
        expect(entries).toHaveLength(1);
        expect(entries[0]).toHaveTextContent('RL150 Greatsword');
    });

    it('renders library-only — Apply is not present', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        expect(screen.queryByTestId('library-apply')).not.toBeInTheDocument();
    });

    it('keeps session-independent actions available', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        // Per-entry actions that do not require a session.
        expect(screen.getByTestId('library-preview')).toBeInTheDocument();
        expect(screen.getByTestId('library-export')).toBeInTheDocument();
        expect(screen.getByTestId('library-rename')).toBeInTheDocument();
        expect(screen.getByTestId('library-delete')).toBeInTheDocument();
        // Global library actions.
        expect(screen.getByTestId('library-refresh')).toBeInTheDocument();
    });

    it('Close calls onClose', async () => {
        const onClose = vi.fn();
        render(<TemplatesShellModal onClose={onClose} />);
        await screen.findAllByTestId('library-entry');
        // Library dialog (the Templates surface) has its own Close button.
        const libraryDialog = screen.getByRole('dialog', { name: 'Templates' });
        const closeBtn = Array.from(libraryDialog.querySelectorAll('button'))
            .find(b => b.textContent?.trim() === 'Close');
        expect(closeBtn).toBeTruthy();
        fireEvent.click(closeBtn!);
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('Preview click opens the read-only ImportTemplatePreviewModal', async () => {
        mocks.PreviewBuildTemplateFromLibrary.mockResolvedValue({
            report: {
                ok: true,
                errors: [],
                warnings: [],
                summary: {
                    inventoryItems: 12,
                    storageItems: 3,
                    weapons: 5,
                    armor: 0,
                    talismans: 1,
                    stackables: 9,
                    aowAssignments: 2,
                },
            },
            json: '{}',
            path: 'tpl-1',
        });
        render(<TemplatesShellModal onClose={vi.fn()} />);
        const previewBtns = await screen.findAllByTestId('library-preview');
        await act(async () => {
            fireEvent.click(previewBtns[0]);
        });
        await waitFor(() => {
            expect(mocks.PreviewBuildTemplateFromLibrary).toHaveBeenCalledWith('tpl-1');
        });
        const previewModal = await screen.findByTestId('import-preview-modal');
        expect(previewModal).toBeInTheDocument();
        expect(previewModal).toHaveTextContent('Preview only');
        // Apply must not be present — global shell has no session.
        expect(screen.queryByTestId('import-preview-apply')).not.toBeInTheDocument();
    });

    it('closing the preview returns to the library view', async () => {
        mocks.PreviewBuildTemplateFromLibrary.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: {} },
            json: '{}',
            path: 'tpl-1',
        });
        render(<TemplatesShellModal onClose={vi.fn()} />);
        const previewBtns = await screen.findAllByTestId('library-preview');
        await act(async () => {
            fireEvent.click(previewBtns[0]);
        });
        const previewModal = await screen.findByTestId('import-preview-modal');
        const closeBtn = Array.from(previewModal.querySelectorAll('button'))
            .find(b => b.textContent?.trim() === 'Close');
        expect(closeBtn).toBeTruthy();
        await act(async () => {
            fireEvent.click(closeBtn!);
        });
        await waitFor(() => {
            expect(screen.queryByTestId('import-preview-modal')).not.toBeInTheDocument();
        });
        // Library is still mounted underneath.
        expect(screen.getByRole('dialog', { name: 'Templates' })).toBeInTheDocument();
    });

    it('does not render Import/Create/YAML/URL/placeholder surfaces', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        // No global Apply.
        expect(screen.queryByTestId('library-apply')).not.toBeInTheDocument();
        // No file-import preview surfacing before user clicks Preview.
        expect(screen.queryByTestId('import-preview-modal')).not.toBeInTheDocument();
        // No export-template (create from current workspace) modal.
        expect(screen.queryByTestId('export-template-modal')).not.toBeInTheDocument();
        // No placeholder copy for future phases.
        expect(screen.queryByText(/coming soon/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/YAML/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/URL/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/Import from file/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/Create from current/i)).not.toBeInTheDocument();
    });

    it('surfaces library errors through the toast helper', async () => {
        mocks.ListBuildTemplateLibrary.mockRejectedValue(new Error('disk gone'));
        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastError.mockClear();
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        expect(String(toastError.mock.calls[0][0])).toMatch(/Templates:/);
    });

    it('refresh path replaces the entry list and notifies via toast', async () => {
        const rebuilt = [
            ...sampleEntries,
            {
                ...sampleEntries[0],
                id: 'tpl-2',
                name: 'Dropped Template',
            },
        ];
        mocks.RebuildBuildTemplateLibraryIndex.mockResolvedValue(rebuilt);
        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastSuccess.mockClear();
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-refresh'));
        });
        await waitFor(() => {
            expect(mocks.RebuildBuildTemplateLibraryIndex).toHaveBeenCalledTimes(1);
        });
        const entries = await screen.findAllByTestId('library-entry');
        expect(entries).toHaveLength(2);
        expect(toastSuccess).toHaveBeenCalled();
    });
});
