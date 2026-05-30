import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../../../wailsjs/go/main/App', () => ({
    ListBuildTemplateLibrary: vi.fn(),
    PreviewBuildTemplateFromLibrary: vi.fn(),
    ApplyBuildTemplateFromLibrary: vi.fn(),
    DeleteBuildTemplateFromLibrary: vi.fn(),
    RenameBuildTemplateInLibrary: vi.fn(),
    ExportLibraryBuildTemplateToFile: vi.fn(),
    ExportLibraryBuildTemplateAsYAMLToFile: vi.fn(),
    PreviewBuildTemplateImportYAMLFromFile: vi.fn(),
    SaveImportedBuildTemplateJSONToLibrary: vi.fn(),
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

const importedEntry = {
    id: 'tpl-imported',
    name: 'Imported Build',
    description: 'via YAML import',
    tags: [],
    filename: 'imported-build-tpl-imported.json',
    createdAt: '2026-05-11T00:00:00Z',
    updatedAt: '2026-05-11T00:00:00Z',
    inventoryItems: 4,
    storageItems: 0,
    warnings: 0,
};

const okSummary = {
    inventoryItems: 4,
    storageItems: 0,
    weapons: 1,
    armor: 0,
    talismans: 0,
    stackables: 3,
    aowAssignments: 0,
};

beforeEach(() => {
    Object.values(mocks).forEach(m => typeof m?.mockReset === 'function' && m.mockReset());
    mocks.ListBuildTemplateLibrary.mockResolvedValue(sampleEntries);
    mocks.GetBuildTemplateLibraryPath.mockResolvedValue('/fake/library');
});

afterEach(() => {
    vi.clearAllMocks();
});

describe('TemplatesShellModal — baseline (Phase 1 invariants)', () => {
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

    it('Close calls onClose', async () => {
        const onClose = vi.fn();
        render(<TemplatesShellModal onClose={onClose} />);
        await screen.findAllByTestId('library-entry');
        const libraryDialog = screen.getByRole('dialog', { name: 'Templates' });
        const closeBtn = Array.from(libraryDialog.querySelectorAll('button'))
            .find(b => b.textContent?.trim() === 'Close');
        expect(closeBtn).toBeTruthy();
        fireEvent.click(closeBtn!);
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('library-entry Preview opens read-only ImportTemplatePreviewModal without Save to Library', async () => {
        mocks.PreviewBuildTemplateFromLibrary.mockResolvedValue({
            report: {
                ok: true,
                errors: [],
                warnings: [],
                summary: okSummary,
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
        // Library-entry preview must never expose Apply or Save to Library.
        expect(screen.queryByTestId('import-preview-apply')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-save-to-library')).not.toBeInTheDocument();
    });

    it('does not render Create, URL import, or placeholder copy for future phases', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        expect(screen.queryByText(/coming soon/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/Create from current/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/URL/i)).not.toBeInTheDocument();
    });
});

describe('TemplatesShellModal — Phase 2B YAML import', () => {
    it('renders the "Import YAML from File…" action in the header', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        expect(screen.getByTestId('templates-shell-import-yaml')).toBeInTheDocument();
    });

    it('click triggers PreviewBuildTemplateImportYAMLFromFile', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: okSummary },
            json: '{"schema":"saveforge.build-template","version":1}',
            path: '/fake/imported.yaml',
        });
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await waitFor(() => {
            expect(mocks.PreviewBuildTemplateImportYAMLFromFile).toHaveBeenCalledTimes(1);
        });
    });

    it('cancelled dialog (sentinel report) does not open preview or surface a toast', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue({
            report: {
                ok: false,
                errors: [],
                warnings: [],
                summary: {
                    inventoryItems: 0,
                    storageItems: 0,
                    weapons: 0,
                    armor: 0,
                    talismans: 0,
                    stackables: 0,
                    aowAssignments: 0,
                },
            },
            json: '',
            path: '',
        });
        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastSuccess.mockClear();
        toastError.mockClear();

        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await waitFor(() => {
            expect(mocks.PreviewBuildTemplateImportYAMLFromFile).toHaveBeenCalled();
        });
        // No second modal opens — only the library dialog remains.
        const dialogs = screen.queryAllByRole('dialog');
        expect(dialogs).toHaveLength(1);
        expect(toastSuccess).not.toHaveBeenCalled();
        expect(toastError).not.toHaveBeenCalled();
    });

    it('OK preview opens import-mode modal with Save to Library and no Apply', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: okSummary },
            json: '{"schema":"saveforge.build-template","version":1}',
            path: '/fake/imported.yaml',
        });
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const previewModal = await screen.findByTestId('import-preview-modal');
        expect(previewModal).toBeInTheDocument();
        const saveBtn = screen.getByTestId('import-preview-save-to-library');
        expect(saveBtn).toBeEnabled();
        expect(screen.queryByTestId('import-preview-apply')).not.toBeInTheDocument();
    });

    it('blocking preview opens modal with Save to Library disabled', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue({
            report: {
                ok: false,
                errors: [{
                    severity: 'error',
                    code: 'structure_invalid',
                    message: 'multi-document YAML payloads are not supported',
                }],
                warnings: [],
                summary: { ...okSummary, inventoryItems: 0 },
            },
            // Backend does not produce canonical JSON for blocking previews.
            json: '',
            path: '/fake/bad.yaml',
        });
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const previewModal = await screen.findByTestId('import-preview-modal');
        expect(previewModal).toBeInTheDocument();
        expect(screen.getByText(/structure_invalid/)).toBeInTheDocument();
        expect(screen.getByTestId('import-preview-save-to-library')).toBeDisabled();
    });

    it('Save to Library passes the canonical JSON returned by preview to the backend', async () => {
        const canonical = '{"schema":"saveforge.build-template","version":1,"createdAt":"2026-05-11T00:00:00Z"}';
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: okSummary },
            json: canonical,
            path: '/fake/imported.yaml',
        });
        mocks.SaveImportedBuildTemplateJSONToLibrary.mockResolvedValue(importedEntry);
        // After save, the library reload should show both entries.
        mocks.ListBuildTemplateLibrary
            .mockResolvedValueOnce(sampleEntries)
            .mockResolvedValueOnce([...sampleEntries, importedEntry]);

        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await screen.findByTestId('import-preview-save-to-library');
        await act(async () => {
            fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        });
        await waitFor(() => {
            expect(mocks.SaveImportedBuildTemplateJSONToLibrary).toHaveBeenCalledWith(canonical);
        });
    });

    it('successful Save closes preview, toasts, and reveals the new entry after refresh', async () => {
        const canonical = '{"schema":"saveforge.build-template","version":1}';
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: okSummary },
            json: canonical,
            path: '/fake/imported.yaml',
        });
        mocks.SaveImportedBuildTemplateJSONToLibrary.mockResolvedValue(importedEntry);
        mocks.ListBuildTemplateLibrary
            .mockResolvedValueOnce(sampleEntries)
            .mockResolvedValueOnce([...sampleEntries, importedEntry]);

        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastSuccess.mockClear();

        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await screen.findByTestId('import-preview-save-to-library');
        await act(async () => {
            fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        });
        await waitFor(() => {
            expect(mocks.SaveImportedBuildTemplateJSONToLibrary).toHaveBeenCalled();
        });
        await waitFor(() => {
            expect(screen.queryByTestId('import-preview-modal')).not.toBeInTheDocument();
        });
        await waitFor(() => {
            expect(toastSuccess).toHaveBeenCalled();
        });
        // The library refresh signal is the user-visible contract:
        // after save the new entry must appear without the user
        // clicking Refresh manually. Asserting on the DOM is more
        // robust than counting fetcher calls (refresh() can be
        // invoked multiple times across re-renders).
        await waitFor(() => {
            expect(screen.getByText('Imported Build')).toBeInTheDocument();
        });
    });

    it('failed Save keeps preview open and shows an error toast', async () => {
        const canonical = '{"schema":"saveforge.build-template","version":1}';
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: okSummary },
            json: canonical,
            path: '/fake/imported.yaml',
        });
        mocks.SaveImportedBuildTemplateJSONToLibrary.mockRejectedValue(new Error('disk full'));

        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastError.mockClear();

        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await screen.findByTestId('import-preview-save-to-library');
        await act(async () => {
            fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        });
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        // Preview must remain mounted so the user can react.
        expect(screen.getByTestId('import-preview-modal')).toBeInTheDocument();
    });
});

describe('TemplatesShellModal — Phase 2B YAML export', () => {
    it('renders an Export YAML action on each library entry', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} />);
        const yamlButtons = await screen.findAllByTestId('library-export-yaml');
        expect(yamlButtons).toHaveLength(1);
    });

    it('Export YAML click invokes the YAML export binding with the entry id', async () => {
        mocks.ExportLibraryBuildTemplateAsYAMLToFile.mockResolvedValue({
            path: '/fake/exported.yaml',
            warnings: [],
            skippedItems: 0,
        });
        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastSuccess.mockClear();

        render(<TemplatesShellModal onClose={vi.fn()} />);
        const btns = await screen.findAllByTestId('library-export-yaml');
        await act(async () => {
            fireEvent.click(btns[0]);
        });
        await waitFor(() => {
            expect(mocks.ExportLibraryBuildTemplateAsYAMLToFile).toHaveBeenCalledWith('tpl-1');
        });
        await waitFor(() => {
            expect(toastSuccess).toHaveBeenCalled();
        });
    });

    it('Cancelled YAML export (empty path) does not toast success', async () => {
        mocks.ExportLibraryBuildTemplateAsYAMLToFile.mockResolvedValue({
            path: '',
            warnings: [],
            skippedItems: 0,
        });
        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastSuccess.mockClear();
        toastError.mockClear();

        render(<TemplatesShellModal onClose={vi.fn()} />);
        const btns = await screen.findAllByTestId('library-export-yaml');
        await act(async () => {
            fireEvent.click(btns[0]);
        });
        await waitFor(() => {
            expect(mocks.ExportLibraryBuildTemplateAsYAMLToFile).toHaveBeenCalled();
        });
        expect(toastSuccess).not.toHaveBeenCalled();
        expect(toastError).not.toHaveBeenCalled();
    });
});
