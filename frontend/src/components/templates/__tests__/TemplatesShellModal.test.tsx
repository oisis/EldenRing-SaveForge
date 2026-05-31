import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../../../wailsjs/go/main/App', () => ({
    ListBuildTemplateLibrary: vi.fn(),
    PreviewBuildTemplateFromLibrary: vi.fn(),
    ApplyBuildTemplateFromLibrary: vi.fn(),
    ApplyBuildTemplateV2FromLibraryToCharacter: vi.fn(),
    ApplyBuildTemplateV2ToCharacterJSON: vi.fn(),
    DeleteBuildTemplateFromLibrary: vi.fn(),
    RenameBuildTemplateInLibrary: vi.fn(),
    ExportLibraryBuildTemplateToFile: vi.fn(),
    ExportLibraryBuildTemplateAsYAMLToFile: vi.fn(),
    PreviewBuildTemplateImportYAMLFromFile: vi.fn(),
    SaveImportedBuildTemplateJSONToLibrary: vi.fn(),
    PreviewBuildTemplateV2FromCharacter: vi.fn(),
    SaveBuildTemplateV2FromCharacterToLibrary: vi.fn(),
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

    it('v1 entries in the global shell render Apply but disabled (no active session)', async () => {
        // Phase 5D.1 enables Apply per-entry, gated by allowApply + entry
        // kind. v1 entries still require a sessionID, which the shell
        // never owns, so the button surfaces but stays inert.
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        const applyBtn = screen.getByTestId('library-apply');
        expect(applyBtn).toBeInTheDocument();
        expect(applyBtn).toBeDisabled();
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

describe('TemplatesShellModal — Phase 3D.2b create-from-character', () => {
    const v2EntryFromSave = {
        id: 'tpl-v2-new',
        name: 'My RL150',
        description: '',
        tags: [],
        filename: 'my-rl150-tpl-v2-new.json',
        createdAt: '2026-05-31T10:00:00Z',
        updatedAt: '2026-05-31T10:00:00Z',
        inventoryItems: 0,
        storageItems: 0,
        warnings: 0,
        version: 2,
        selectedSections: ['profile'],
    };

    function v2OKPreviewBundle() {
        return {
            report: {
                ok: true,
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
                    version: 2,
                    selectedSections: ['profile'],
                    profileFieldsPresent: ['level'],
                    statFieldsPresent: [],
                },
            },
        };
    }

    it('renders the Create from Character button enabled when charIndex and saveLoaded are provided', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={2} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        const btn = screen.getByTestId('templates-shell-create-v2') as HTMLButtonElement;
        expect(btn).toBeInTheDocument();
        expect(btn.disabled).toBe(false);
        expect(btn.textContent).toMatch(/Create from Character/);
    });

    it('Create button is disabled when saveLoaded is false or omitted', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={0} />);
        await screen.findAllByTestId('library-entry');
        const btn = screen.getByTestId('templates-shell-create-v2') as HTMLButtonElement;
        expect(btn.disabled).toBe(true);
        expect(btn.getAttribute('title')).toMatch(/Load a save/i);
    });

    it('Create button is disabled when charIndex is undefined', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        const btn = screen.getByTestId('templates-shell-create-v2') as HTMLButtonElement;
        expect(btn.disabled).toBe(true);
        expect(btn.getAttribute('title')).toMatch(/Select a character/i);
    });

    it('clicking Create from Character opens the CreateTemplateV2Modal', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={3} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-create-v2'));
        });
        expect(screen.getByTestId('create-template-v2-modal')).toBeInTheDocument();
    });

    it('Save success refreshes library, closes the create modal, and toasts', async () => {
        mocks.PreviewBuildTemplateV2FromCharacter.mockResolvedValue(v2OKPreviewBundle());
        mocks.SaveBuildTemplateV2FromCharacterToLibrary.mockResolvedValue(v2EntryFromSave);
        mocks.ListBuildTemplateLibrary
            .mockResolvedValueOnce(sampleEntries)
            .mockResolvedValue([...sampleEntries, v2EntryFromSave]);

        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastSuccess.mockClear();

        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-create-v2'));
        });
        fireEvent.click(screen.getByTestId('create-template-v2-profile-level'));
        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });
        await screen.findByTestId('import-preview-save-to-library');
        await act(async () => {
            fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        });
        await waitFor(() => {
            expect(mocks.SaveBuildTemplateV2FromCharacterToLibrary).toHaveBeenCalledTimes(1);
        });
        expect(mocks.SaveBuildTemplateV2FromCharacterToLibrary.mock.calls[0][0]).toBe(1);
        await waitFor(() => {
            expect(screen.queryByTestId('create-template-v2-modal')).not.toBeInTheDocument();
        });
        await waitFor(() => expect(toastSuccess).toHaveBeenCalled());
        // Library refresh signal is the user-visible contract: after save
        // the new v2 entry must appear without the user clicking Refresh.
        await waitFor(() => {
            expect(screen.getByText('My RL150')).toBeInTheDocument();
        });
    });

    it('Create modal preview error surfaces as a toast and leaves the create modal open', async () => {
        mocks.PreviewBuildTemplateV2FromCharacter.mockRejectedValue(new Error('no save loaded'));

        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastError.mockClear();

        render(<TemplatesShellModal onClose={vi.fn()} charIndex={0} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-create-v2'));
        });
        fireEvent.click(screen.getByTestId('create-template-v2-profile-level'));
        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });
        await waitFor(() => expect(toastError).toHaveBeenCalled());
        expect(screen.getByTestId('create-template-v2-modal')).toBeInTheDocument();
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

describe('TemplatesShellModal — Phase 5D.1 v2 Apply orchestration', () => {
    const v2LibraryEntry = {
        id: 'tpl-v2-apply',
        name: 'V2 RL150',
        description: 'profile + stats',
        tags: [],
        filename: 'v2-rl150-tpl-v2-apply.json',
        createdAt: '2026-05-20T10:00:00Z',
        updatedAt: '2026-05-20T10:00:00Z',
        inventoryItems: 0,
        storageItems: 0,
        warnings: 0,
        version: 2,
        selectedSections: ['profile', 'stats'],
    };

    function applyOKResult(extra: Partial<Record<string, unknown>> = {}) {
        return {
            preview: { ok: true, errors: [], warnings: [], summary: {} },
            applied: true,
            charIndex: 1,
            appliedFields: ['profile.level', 'stats.vigor'],
            skippedFields: [],
            ...extra,
        };
    }

    beforeEach(() => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([v2LibraryEntry]);
    });

    it('exposes v2 Apply for v2 profile/stats entries when saveLoaded + charIndex', async () => {
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        const applyBtn = await screen.findByTestId('library-apply');
        expect(applyBtn).toBeEnabled();
        expect(applyBtn).toHaveAttribute('title', 'Apply schema v2 template to character slot 2');
    });

    it('confirm OK calls ApplyBuildTemplateV2FromLibraryToCharacter with charIndex, id and mode=append', async () => {
        mocks.ApplyBuildTemplateV2FromLibraryToCharacter.mockResolvedValue(applyOKResult());
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        fireEvent.click(await screen.findByTestId('library-apply'));
        await screen.findByTestId('library-apply-v2-confirm');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-v2-confirm-button'));
        });
        await waitFor(() => {
            expect(mocks.ApplyBuildTemplateV2FromLibraryToCharacter).toHaveBeenCalledTimes(1);
        });
        const call = mocks.ApplyBuildTemplateV2FromLibraryToCharacter.mock.calls[0];
        expect(call[0]).toBe(1);
        expect(call[1]).toBe('tpl-v2-apply');
        expect((call[2] as { mode: string }).mode).toBe('append');
    });

    it('applied=true success toasts and calls onCharacterTemplateApplied', async () => {
        mocks.ApplyBuildTemplateV2FromLibraryToCharacter.mockResolvedValue(applyOKResult());
        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastSuccess.mockClear();
        const onCharacterTemplateApplied = vi.fn();
        render(
            <TemplatesShellModal
                onClose={vi.fn()}
                charIndex={1}
                saveLoaded
                onCharacterTemplateApplied={onCharacterTemplateApplied}
            />,
        );
        fireEvent.click(await screen.findByTestId('library-apply'));
        await screen.findByTestId('library-apply-v2-confirm');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-v2-confirm-button'));
        });
        await waitFor(() => {
            expect(toastSuccess).toHaveBeenCalled();
        });
        const successArg = toastSuccess.mock.calls[0][0] as string;
        expect(successArg).toMatch(/V2 RL150/);
        expect(successArg).toMatch(/slot 2/);
        expect(onCharacterTemplateApplied).toHaveBeenCalledWith(1);
    });

    it('emits an info toast when skippedFields includes profile.class', async () => {
        mocks.ApplyBuildTemplateV2FromLibraryToCharacter.mockResolvedValue(
            applyOKResult({ skippedFields: ['profile.class'] }),
        );
        const { default: toast } = await import('../../../lib/toast');
        const toastFn = toast as unknown as ReturnType<typeof vi.fn> & {
            success: ReturnType<typeof vi.fn>;
        };
        toastFn.mockClear();
        toastFn.success.mockClear();
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={0} saveLoaded />);
        fireEvent.click(await screen.findByTestId('library-apply'));
        await screen.findByTestId('library-apply-v2-confirm');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-v2-confirm-button'));
        });
        await waitFor(() => {
            expect(toastFn.success).toHaveBeenCalled();
        });
        await waitFor(() => {
            expect(toastFn).toHaveBeenCalled();
        });
        const infoCall = toastFn.mock.calls.find(args => /class/i.test(String(args[0])));
        expect(infoCall).toBeTruthy();
    });

    it('applied=false with preview errors toasts error and does not refresh state', async () => {
        mocks.ApplyBuildTemplateV2FromLibraryToCharacter.mockResolvedValue({
            preview: {
                ok: false,
                errors: [{ severity: 'error', code: 'unsupported_category', message: 'inventory.workspace not supported' }],
                warnings: [],
                summary: {},
            },
            applied: false,
            charIndex: 1,
            appliedFields: [],
            skippedFields: [],
        });
        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastError.mockClear();
        toastSuccess.mockClear();
        const onCharacterTemplateApplied = vi.fn();
        render(
            <TemplatesShellModal
                onClose={vi.fn()}
                charIndex={1}
                saveLoaded
                onCharacterTemplateApplied={onCharacterTemplateApplied}
            />,
        );
        fireEvent.click(await screen.findByTestId('library-apply'));
        await screen.findByTestId('library-apply-v2-confirm');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-v2-confirm-button'));
        });
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        expect(toastSuccess).not.toHaveBeenCalled();
        expect(onCharacterTemplateApplied).not.toHaveBeenCalled();
        // Confirm row stays open so the user can react.
        expect(screen.getByTestId('library-apply-v2-confirm')).toBeInTheDocument();
    });

    it('binding throw toasts error and does not refresh state', async () => {
        mocks.ApplyBuildTemplateV2FromLibraryToCharacter.mockRejectedValue(new Error('rpc broken'));
        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastError.mockClear();
        const onCharacterTemplateApplied = vi.fn();
        render(
            <TemplatesShellModal
                onClose={vi.fn()}
                charIndex={1}
                saveLoaded
                onCharacterTemplateApplied={onCharacterTemplateApplied}
            />,
        );
        fireEvent.click(await screen.findByTestId('library-apply'));
        await screen.findByTestId('library-apply-v2-confirm');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-v2-confirm-button'));
        });
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        expect(onCharacterTemplateApplied).not.toHaveBeenCalled();
        expect(screen.getByTestId('library-apply-v2-confirm')).toBeInTheDocument();
    });

    it('v1 entries in the shell remain disabled (no sessionID) even with saveLoaded + charIndex', async () => {
        const v1Entry = {
            id: 'tpl-v1',
            name: 'V1 Sample',
            description: '',
            tags: [],
            filename: 'tpl-v1.json',
            createdAt: '2026-05-01T10:00:00Z',
            updatedAt: '2026-05-01T10:00:00Z',
            inventoryItems: 4,
            storageItems: 0,
            warnings: 0,
            version: 1,
            selectedSections: ['inventory.workspace'],
        };
        mocks.ListBuildTemplateLibrary.mockResolvedValue([v1Entry]);
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={0} saveLoaded />);
        const applyBtn = await screen.findByTestId('library-apply');
        expect(applyBtn).toBeDisabled();
    });
});

describe('TemplatesShellModal — Phase 5D.2 direct imported YAML Apply', () => {
    const v2CanonicalJSON =
        '{"schema":"saveforge.build-template","version":2,"selection":{"sections":{"profile":{"level":true}}}}';

    function v2OKImportedPreview() {
        return {
            report: {
                ok: true,
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
                    version: 2,
                    selectedSections: ['profile', 'stats'],
                    profileFieldsPresent: ['level'],
                    statFieldsPresent: ['vigor'],
                },
            },
            json: v2CanonicalJSON,
            path: '/fake/imported.yaml',
        };
    }

    function applyV2OKResult(extra: Partial<Record<string, unknown>> = {}) {
        return {
            preview: { ok: true, errors: [], warnings: [], summary: {} },
            applied: true,
            charIndex: 1,
            appliedFields: ['profile.level'],
            skippedFields: [],
            ...extra,
        };
    }

    it('Apply to character is enabled on a v2 imported preview when saveLoaded + charIndex', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreview());
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const btn = await screen.findByTestId('import-preview-apply-v2');
        expect(btn).toBeInTheDocument();
        expect(btn).toBeEnabled();
        // Save to Library remains available next to it.
        expect(screen.getByTestId('import-preview-save-to-library')).toBeEnabled();
    });

    it('click calls ApplyBuildTemplateV2ToCharacterJSON with charIndex, canonicalJSON and mode=append', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreview());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue(applyV2OKResult());

        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const btn = await screen.findByTestId('import-preview-apply-v2');
        await act(async () => {
            fireEvent.click(btn);
        });
        await waitFor(() => {
            expect(mocks.ApplyBuildTemplateV2ToCharacterJSON).toHaveBeenCalledTimes(1);
        });
        const call = mocks.ApplyBuildTemplateV2ToCharacterJSON.mock.calls[0];
        expect(call[0]).toBe(1);
        expect(call[1]).toBe(v2CanonicalJSON);
        expect((call[2] as { mode: string }).mode).toBe('append');
    });

    it('applied=true closes the imported preview, toasts success and calls onCharacterTemplateApplied', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreview());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue(applyV2OKResult());
        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastSuccess.mockClear();
        const onCharacterTemplateApplied = vi.fn();

        render(
            <TemplatesShellModal
                onClose={vi.fn()}
                charIndex={1}
                saveLoaded
                onCharacterTemplateApplied={onCharacterTemplateApplied}
            />,
        );
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const btn = await screen.findByTestId('import-preview-apply-v2');
        await act(async () => {
            fireEvent.click(btn);
        });
        await waitFor(() => {
            expect(toastSuccess).toHaveBeenCalled();
        });
        const successArg = toastSuccess.mock.calls[0][0] as string;
        expect(successArg).toMatch(/slot 2/);
        expect(successArg).toMatch(/imported template/);
        expect(onCharacterTemplateApplied).toHaveBeenCalledWith(1);
        await waitFor(() => {
            expect(screen.queryByTestId('import-preview-modal')).not.toBeInTheDocument();
        });
    });

    it('skippedFields containing profile.class emits an info toast', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreview());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue(
            applyV2OKResult({ skippedFields: ['profile.class'] }),
        );
        const { default: toast } = await import('../../../lib/toast');
        const toastFn = toast as unknown as ReturnType<typeof vi.fn> & {
            success: ReturnType<typeof vi.fn>;
        };
        toastFn.mockClear();
        toastFn.success.mockClear();

        render(<TemplatesShellModal onClose={vi.fn()} charIndex={0} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const btn = await screen.findByTestId('import-preview-apply-v2');
        await act(async () => {
            fireEvent.click(btn);
        });
        await waitFor(() => {
            expect(toastFn.success).toHaveBeenCalled();
        });
        await waitFor(() => {
            expect(toastFn).toHaveBeenCalled();
        });
        const infoCall = toastFn.mock.calls.find(args => /class/i.test(String(args[0])));
        expect(infoCall).toBeTruthy();
    });

    it('applied=false toasts error and leaves the imported preview open', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreview());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue({
            preview: {
                ok: false,
                errors: [{
                    severity: 'error',
                    code: 'unsupported_section',
                    message: 'sections.equipment not supported in Phase 5',
                }],
                warnings: [],
                summary: {},
            },
            applied: false,
            charIndex: 1,
            appliedFields: [],
            skippedFields: [],
        });
        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastError.mockClear();
        toastSuccess.mockClear();
        const onCharacterTemplateApplied = vi.fn();

        render(
            <TemplatesShellModal
                onClose={vi.fn()}
                charIndex={1}
                saveLoaded
                onCharacterTemplateApplied={onCharacterTemplateApplied}
            />,
        );
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const btn = await screen.findByTestId('import-preview-apply-v2');
        await act(async () => {
            fireEvent.click(btn);
        });
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        expect(toastSuccess).not.toHaveBeenCalled();
        expect(onCharacterTemplateApplied).not.toHaveBeenCalled();
        expect(screen.getByTestId('import-preview-modal')).toBeInTheDocument();
    });

    it('thrown binding error toasts error and leaves the imported preview open', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreview());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockRejectedValue(new Error('rpc broken'));
        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastError.mockClear();
        const onCharacterTemplateApplied = vi.fn();

        render(
            <TemplatesShellModal
                onClose={vi.fn()}
                charIndex={1}
                saveLoaded
                onCharacterTemplateApplied={onCharacterTemplateApplied}
            />,
        );
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const btn = await screen.findByTestId('import-preview-apply-v2');
        await act(async () => {
            fireEvent.click(btn);
        });
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        expect(onCharacterTemplateApplied).not.toHaveBeenCalled();
        expect(screen.getByTestId('import-preview-modal')).toBeInTheDocument();
    });

    it('v1 imported preview never exposes Apply to character', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue({
            report: {
                ok: true,
                errors: [],
                warnings: [],
                summary: {
                    inventoryItems: 4,
                    storageItems: 0,
                    weapons: 1,
                    armor: 0,
                    talismans: 0,
                    stackables: 0,
                    aowAssignments: 0,
                    version: 1,
                    selectedSections: ['inventory.workspace'],
                },
            },
            json: '{"schema":"saveforge.build-template","version":1}',
            path: '/fake/v1.yaml',
        });

        render(<TemplatesShellModal onClose={vi.fn()} charIndex={0} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await screen.findByTestId('import-preview-modal');
        expect(screen.queryByTestId('import-preview-apply-v2')).not.toBeInTheDocument();
        // Save to Library remains the only path for v1 imports.
        expect(screen.getByTestId('import-preview-save-to-library')).toBeEnabled();
    });

    it('v2 imported preview without saveLoaded keeps Apply visible but disabled', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreview());
        render(<TemplatesShellModal onClose={vi.fn()} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const btn = await screen.findByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
    });
});

describe('TemplatesShellModal — Phase 6 Apply with overrides', () => {
    const phase6CanonicalJSON = JSON.stringify({
        schema: 'saveforge.build-template',
        version: 2,
        selection: { profile: { level: true }, stats: { vigor: true } },
        sections: { profile: { level: 50 }, stats: { vigor: 25 } },
    });

    function v2OKImportedPreviewPhase6() {
        return {
            report: {
                ok: true,
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
                    version: 2,
                    selectedSections: ['profile', 'stats'],
                    profileFieldsPresent: ['level'],
                    statFieldsPresent: ['vigor'],
                },
            },
            json: phase6CanonicalJSON,
            path: '/fake/imported.yaml',
        };
    }

    function applyV2OKResult(extra: Partial<Record<string, unknown>> = {}) {
        return {
            preview: { ok: true, errors: [], warnings: [], summary: {} },
            applied: true,
            charIndex: 1,
            appliedFields: ['profile.level', 'stats.vigor'],
            skippedFields: [],
            ...extra,
        };
    }

    const v2LibraryEntry = {
        id: 'tpl-lib-v2',
        name: 'Library v2 sample',
        description: '',
        tags: [],
        filename: 'tpl-lib-v2.json',
        createdAt: '2026-05-12T00:00:00Z',
        updatedAt: '2026-05-12T00:00:00Z',
        inventoryItems: 0,
        storageItems: 0,
        warnings: 0,
        version: 2,
        selectedSections: ['profile', 'stats'],
    };

    it('Import: clicking "Apply with overrides…" opens the overrides modal with parsed fields', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreviewPhase6());
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        const btn = await screen.findByTestId('import-preview-apply-v2-overrides');
        await act(async () => {
            fireEvent.click(btn);
        });
        expect(await screen.findByTestId('apply-overrides-modal')).toBeInTheDocument();
        expect(screen.getByTestId('apply-overrides-stats-input-vigor')).toHaveValue('25');
        expect(screen.getByTestId('apply-overrides-profile-input-level')).toHaveValue('50');
        expect(screen.getByTestId('apply-overrides-source-label')).toHaveTextContent(/imported\.yaml/);
    });

    it('Import: confirming overrides calls ApplyBuildTemplateV2ToCharacterJSON with mutated JSON and mode=append', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreviewPhase6());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue(applyV2OKResult());
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await act(async () => {
            fireEvent.click(await screen.findByTestId('import-preview-apply-v2-overrides'));
        });
        await screen.findByTestId('apply-overrides-modal');
        fireEvent.change(screen.getByTestId('apply-overrides-stats-input-vigor'), {
            target: { value: '40' },
        });
        await act(async () => {
            fireEvent.click(screen.getByTestId('apply-overrides-apply'));
        });
        await waitFor(() => {
            expect(mocks.ApplyBuildTemplateV2ToCharacterJSON).toHaveBeenCalledTimes(1);
        });
        const call = mocks.ApplyBuildTemplateV2ToCharacterJSON.mock.calls[0];
        expect(call[0]).toBe(1);
        const parsed = JSON.parse(call[1] as string);
        expect(parsed.sections.stats.vigor).toBe(40);
        expect(parsed.selection.stats.vigor).toBe(true);
        expect((call[2] as { mode: string }).mode).toBe('append');
        // FromLibrary endpoint must NOT have been touched — Phase 6 import
        // path goes through the JSON endpoint.
        expect(mocks.ApplyBuildTemplateV2FromLibraryToCharacter).not.toHaveBeenCalled();
    });

    it('Import: applied=true closes both modals, toasts success, and calls onCharacterTemplateApplied', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreviewPhase6());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue(applyV2OKResult());
        const { default: toast } = await import('../../../lib/toast');
        const toastSuccess = (toast as unknown as { success: ReturnType<typeof vi.fn> }).success;
        toastSuccess.mockClear();
        const onCharacterTemplateApplied = vi.fn();
        render(
            <TemplatesShellModal
                onClose={vi.fn()}
                charIndex={1}
                saveLoaded
                onCharacterTemplateApplied={onCharacterTemplateApplied}
            />,
        );
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await act(async () => {
            fireEvent.click(await screen.findByTestId('import-preview-apply-v2-overrides'));
        });
        await screen.findByTestId('apply-overrides-modal');
        await act(async () => {
            fireEvent.click(screen.getByTestId('apply-overrides-apply'));
        });
        await waitFor(() => {
            expect(toastSuccess).toHaveBeenCalled();
        });
        expect(onCharacterTemplateApplied).toHaveBeenCalledWith(1);
        await waitFor(() => {
            expect(screen.queryByTestId('apply-overrides-modal')).not.toBeInTheDocument();
        });
        await waitFor(() => {
            expect(screen.queryByTestId('import-preview-modal')).not.toBeInTheDocument();
        });
    });

    it('Import: applied=false leaves the overrides modal open and toasts the error', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreviewPhase6());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue({
            preview: {
                ok: false,
                errors: [{ severity: 'error', code: 'x', message: 'something wrong' }],
                warnings: [],
                summary: {},
            },
            applied: false,
            charIndex: 1,
            appliedFields: [],
            skippedFields: [],
        });
        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastError.mockClear();
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await act(async () => {
            fireEvent.click(await screen.findByTestId('import-preview-apply-v2-overrides'));
        });
        await screen.findByTestId('apply-overrides-modal');
        await act(async () => {
            fireEvent.click(screen.getByTestId('apply-overrides-apply'));
        });
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        expect(screen.getByTestId('apply-overrides-modal')).toBeInTheDocument();
    });

    it('Import: thrown binding error leaves the overrides modal open and toasts the error', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreviewPhase6());
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockRejectedValue(new Error('rpc dead'));
        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastError.mockClear();
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await act(async () => {
            fireEvent.click(await screen.findByTestId('import-preview-apply-v2-overrides'));
        });
        await screen.findByTestId('apply-overrides-modal');
        await act(async () => {
            fireEvent.click(screen.getByTestId('apply-overrides-apply'));
        });
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        expect(screen.getByTestId('apply-overrides-modal')).toBeInTheDocument();
    });

    it('Import: Cancel closes the overrides modal without calling the binding and preserves the import preview', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreviewPhase6());
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await act(async () => {
            fireEvent.click(await screen.findByTestId('import-preview-apply-v2-overrides'));
        });
        await screen.findByTestId('apply-overrides-modal');
        fireEvent.change(screen.getByTestId('apply-overrides-stats-input-vigor'), {
            target: { value: '40' },
        });
        fireEvent.click(screen.getByTestId('apply-overrides-cancel'));
        await waitFor(() => {
            expect(screen.queryByTestId('apply-overrides-modal')).not.toBeInTheDocument();
        });
        expect(mocks.ApplyBuildTemplateV2ToCharacterJSON).not.toHaveBeenCalled();
        expect(screen.getByTestId('import-preview-modal')).toBeInTheDocument();
    });

    it('Import: invalid override value disables Apply and does not call the binding', async () => {
        mocks.PreviewBuildTemplateImportYAMLFromFile.mockResolvedValue(v2OKImportedPreviewPhase6());
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('templates-shell-import-yaml'));
        });
        await act(async () => {
            fireEvent.click(await screen.findByTestId('import-preview-apply-v2-overrides'));
        });
        await screen.findByTestId('apply-overrides-modal');
        fireEvent.change(screen.getByTestId('apply-overrides-stats-input-vigor'), {
            target: { value: '999' },
        });
        const applyBtn = screen.getByTestId('apply-overrides-apply');
        expect(applyBtn).toBeDisabled();
        fireEvent.click(applyBtn);
        expect(mocks.ApplyBuildTemplateV2ToCharacterJSON).not.toHaveBeenCalled();
    });

    it('Library: clicking Apply with overrides loads canonical JSON via PreviewBuildTemplateFromLibrary then opens overrides modal', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([v2LibraryEntry]);
        mocks.PreviewBuildTemplateFromLibrary.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: { version: 2, selectedSections: ['profile', 'stats'] } },
            json: phase6CanonicalJSON,
            path: '/fake/library/tpl-lib-v2.json',
        });
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-overrides'));
        });
        await waitFor(() => {
            expect(mocks.PreviewBuildTemplateFromLibrary).toHaveBeenCalledWith('tpl-lib-v2');
        });
        expect(await screen.findByTestId('apply-overrides-modal')).toBeInTheDocument();
        expect(screen.getByTestId('apply-overrides-source-label')).toHaveTextContent(/Library/);
        expect(screen.getByTestId('apply-overrides-stats-input-vigor')).toHaveValue('25');
    });

    it('Library: confirming overrides calls ApplyBuildTemplateV2ToCharacterJSON with mutated JSON (not FromLibrary)', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([v2LibraryEntry]);
        mocks.PreviewBuildTemplateFromLibrary.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: { version: 2, selectedSections: ['profile', 'stats'] } },
            json: phase6CanonicalJSON,
            path: '/fake/library/tpl-lib-v2.json',
        });
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue(applyV2OKResult());
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={2} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-overrides'));
        });
        await screen.findByTestId('apply-overrides-modal');
        fireEvent.change(screen.getByTestId('apply-overrides-profile-input-level'), {
            target: { value: '99' },
        });
        await act(async () => {
            fireEvent.click(screen.getByTestId('apply-overrides-apply'));
        });
        await waitFor(() => {
            expect(mocks.ApplyBuildTemplateV2ToCharacterJSON).toHaveBeenCalledTimes(1);
        });
        const call = mocks.ApplyBuildTemplateV2ToCharacterJSON.mock.calls[0];
        expect(call[0]).toBe(2);
        const parsed = JSON.parse(call[1] as string);
        expect(parsed.sections.profile.level).toBe(99);
        // Fast library Apply path must NOT have been used.
        expect(mocks.ApplyBuildTemplateV2FromLibraryToCharacter).not.toHaveBeenCalled();
    });

    it('Library: fast Apply path still calls ApplyBuildTemplateV2FromLibraryToCharacter, untouched by Phase 6', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([v2LibraryEntry]);
        mocks.ApplyBuildTemplateV2FromLibraryToCharacter.mockResolvedValue(applyV2OKResult());
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={0} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply'));
        });
        await screen.findByTestId('library-apply-v2-confirm');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-v2-confirm-button'));
        });
        await waitFor(() => {
            expect(mocks.ApplyBuildTemplateV2FromLibraryToCharacter).toHaveBeenCalledTimes(1);
        });
        const call = mocks.ApplyBuildTemplateV2FromLibraryToCharacter.mock.calls[0];
        expect(call[0]).toBe(0);
        expect(call[1]).toBe('tpl-lib-v2');
        // Phase 6 JSON endpoint must NOT have been touched by fast Apply.
        expect(mocks.ApplyBuildTemplateV2ToCharacterJSON).not.toHaveBeenCalled();
    });

    it('Library: PreviewBuildTemplateFromLibrary returning no canonical JSON toasts an error and does not open the modal', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([v2LibraryEntry]);
        mocks.PreviewBuildTemplateFromLibrary.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: {} },
            json: '',
            path: '/fake/library/tpl-lib-v2.json',
        });
        const { default: toast } = await import('../../../lib/toast');
        const toastError = (toast as unknown as { error: ReturnType<typeof vi.fn> }).error;
        toastError.mockClear();
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={0} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-overrides'));
        });
        await waitFor(() => {
            expect(toastError).toHaveBeenCalled();
        });
        expect(screen.queryByTestId('apply-overrides-modal')).not.toBeInTheDocument();
    });

    it('Library: skippedFields containing profile.class emits an info toast on success', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([v2LibraryEntry]);
        mocks.PreviewBuildTemplateFromLibrary.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: { version: 2, selectedSections: ['profile'] } },
            json: phase6CanonicalJSON,
            path: '/fake/library/tpl-lib-v2.json',
        });
        mocks.ApplyBuildTemplateV2ToCharacterJSON.mockResolvedValue(
            applyV2OKResult({ skippedFields: ['profile.class'] }),
        );
        const { default: toast } = await import('../../../lib/toast');
        const toastFn = toast as unknown as ReturnType<typeof vi.fn> & {
            success: ReturnType<typeof vi.fn>;
        };
        toastFn.mockClear();
        toastFn.success.mockClear();
        render(<TemplatesShellModal onClose={vi.fn()} charIndex={1} saveLoaded />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-apply-overrides'));
        });
        await screen.findByTestId('apply-overrides-modal');
        await act(async () => {
            fireEvent.click(screen.getByTestId('apply-overrides-apply'));
        });
        await waitFor(() => {
            expect(toastFn.success).toHaveBeenCalled();
        });
        const infoCall = toastFn.mock.calls.find(args => /class/i.test(String(args[0])));
        expect(infoCall).toBeTruthy();
    });
});
