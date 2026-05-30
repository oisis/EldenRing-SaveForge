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

import * as App from '../../../../wailsjs/go/main/App';
import { TemplateLibraryModal } from '../TemplateLibraryModal';

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
    {
        id: 'tpl-2',
        name: 'Trade Fodder',
        description: '',
        tags: [],
        filename: 'trade-fodder-tpl-2.json',
        createdAt: '2026-04-20T09:00:00Z',
        updatedAt: '2026-04-20T09:00:00Z',
        inventoryItems: 0,
        storageItems: 50,
        warnings: 0,
    },
];

beforeEach(() => {
    Object.values(mocks).forEach(m => typeof m?.mockReset === 'function' && m.mockReset());
    mocks.ListBuildTemplateLibrary.mockResolvedValue(sampleEntries);
    mocks.GetBuildTemplateLibraryPath.mockResolvedValue('/tmp/library');
});

afterEach(() => {
    vi.clearAllMocks();
});

function defaultProps(overrides: Partial<Parameters<typeof TemplateLibraryModal>[0]> = {}) {
    return {
        sessionID: 'ses-test',
        onClose: vi.fn(),
        onApplied: vi.fn(),
        onError: vi.fn(),
        ...overrides,
    };
}

describe('TemplateLibraryModal — listing', () => {
    it('renders entries from ListBuildTemplateLibrary', async () => {
        render(<TemplateLibraryModal {...defaultProps()} />);
        await waitFor(() => {
            expect(mocks.ListBuildTemplateLibrary).toHaveBeenCalled();
        });
        const entries = await screen.findAllByTestId('library-entry');
        expect(entries).toHaveLength(2);
        expect(entries[0]).toHaveTextContent('RL150 Greatsword');
        expect(entries[1]).toHaveTextContent('Trade Fodder');
    });

    it('shows empty state when the library has no entries', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([]);
        render(<TemplateLibraryModal {...defaultProps()} />);
        expect(await screen.findByTestId('library-empty')).toBeInTheDocument();
    });

    it('routes list errors through onError', async () => {
        mocks.ListBuildTemplateLibrary.mockRejectedValue(new Error('disk gone'));
        const onError = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onError })} />);
        await waitFor(() => {
            expect(onError).toHaveBeenCalled();
        });
    });
});

describe('TemplateLibraryModal — Preview', () => {
    it('calls PreviewBuildTemplateFromLibrary and forwards via onPreviewed', async () => {
        mocks.PreviewBuildTemplateFromLibrary.mockResolvedValue({
            report: { ok: true, errors: [], warnings: [], summary: {} },
            json: '{}',
            path: 'tpl-1',
        });
        const onPreviewed = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onPreviewed })} />);
        const btns = await screen.findAllByTestId('library-preview');
        await act(async () => {
            fireEvent.click(btns[0]);
        });
        await waitFor(() => {
            expect(mocks.PreviewBuildTemplateFromLibrary).toHaveBeenCalledWith('tpl-1');
        });
        expect(onPreviewed).toHaveBeenCalled();
        const [arg0, arg1] = onPreviewed.mock.calls[0];
        expect(arg0.json).toBe('{}');
        expect(arg1.id).toBe('tpl-1');
    });
});

describe('TemplateLibraryModal — Apply', () => {
    it('calls ApplyBuildTemplateFromLibrary with mode=append', async () => {
        mocks.ApplyBuildTemplateFromLibrary.mockResolvedValue({
            preview: { ok: true, errors: [], warnings: [], summary: {} },
            workspace: { sessionID: 'ses-test' },
            applied: true,
        });
        const onApplied = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onApplied })} />);
        const btns = await screen.findAllByTestId('library-apply');
        await act(async () => {
            fireEvent.click(btns[0]);
        });
        await waitFor(() => {
            expect(mocks.ApplyBuildTemplateFromLibrary).toHaveBeenCalled();
        });
        const call = mocks.ApplyBuildTemplateFromLibrary.mock.calls[0];
        expect(call[0]).toBe('ses-test');
        expect(call[1]).toBe('tpl-1');
        expect((call[2] as { mode: string }).mode).toBe('append');
        expect(onApplied).toHaveBeenCalled();
    });

    it('disables Apply when no sessionID is present', async () => {
        render(<TemplateLibraryModal {...defaultProps({ sessionID: '' })} />);
        const btns = await screen.findAllByTestId('library-apply');
        expect(btns[0]).toBeDisabled();
    });

    it('passes the Apply result to onApplied even when applied=false', async () => {
        mocks.ApplyBuildTemplateFromLibrary.mockResolvedValue({
            preview: { ok: false, errors: [{ code: 'capacity_exceeded' }], warnings: [], summary: {} },
            workspace: { sessionID: 'ses-test' },
            applied: false,
        });
        const onApplied = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onApplied })} />);
        const btns = await screen.findAllByTestId('library-apply');
        await act(async () => {
            fireEvent.click(btns[0]);
        });
        await waitFor(() => {
            expect(onApplied).toHaveBeenCalled();
        });
        const [result] = onApplied.mock.calls[0];
        expect(result.applied).toBe(false);
    });
});

describe('TemplateLibraryModal — Delete with confirm', () => {
    it('opens a confirm row before calling Delete', async () => {
        render(<TemplateLibraryModal {...defaultProps()} />);
        const btns = await screen.findAllByTestId('library-delete');
        fireEvent.click(btns[0]);
        expect(await screen.findByTestId('library-delete-confirm')).toBeInTheDocument();
        expect(mocks.DeleteBuildTemplateFromLibrary).not.toHaveBeenCalled();
    });

    it('calls Delete and refreshes the list when confirm is clicked', async () => {
        mocks.DeleteBuildTemplateFromLibrary.mockResolvedValue(undefined);
        mocks.ListBuildTemplateLibrary
            .mockResolvedValueOnce(sampleEntries)
            .mockResolvedValueOnce([sampleEntries[1]]);
        const onDeleted = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onDeleted })} />);
        const btns = await screen.findAllByTestId('library-delete');
        fireEvent.click(btns[0]);
        await act(async () => {
            fireEvent.click(await screen.findByTestId('library-delete-confirm-yes'));
        });
        await waitFor(() => {
            expect(mocks.DeleteBuildTemplateFromLibrary).toHaveBeenCalledWith('tpl-1');
        });
        expect(onDeleted).toHaveBeenCalledWith('tpl-1');
        await waitFor(() => {
            expect(mocks.ListBuildTemplateLibrary).toHaveBeenCalledTimes(2);
        });
    });
});

describe('TemplateLibraryModal — inline Rename', () => {
    it('opens the inline rename form and calls Rename on save', async () => {
        mocks.RenameBuildTemplateInLibrary.mockResolvedValue({
            ...sampleEntries[0],
            name: 'New Name',
            description: 'updated',
            tags: ['fresh'],
        });
        render(<TemplateLibraryModal {...defaultProps()} />);
        const renames = await screen.findAllByTestId('library-rename');
        fireEvent.click(renames[0]);

        const nameInput = await screen.findByTestId('library-rename-name');
        fireEvent.change(nameInput, { target: { value: 'New Name' } });
        fireEvent.change(screen.getByTestId('library-rename-description'), { target: { value: 'updated' } });
        fireEvent.change(screen.getByTestId('library-rename-tags'), { target: { value: 'fresh' } });

        await act(async () => {
            fireEvent.click(screen.getByTestId('library-rename-save'));
        });
        await waitFor(() => {
            expect(mocks.RenameBuildTemplateInLibrary).toHaveBeenCalled();
        });
        const call = mocks.RenameBuildTemplateInLibrary.mock.calls[0];
        expect(call[0]).toBe('tpl-1');
        expect(call[1]).toBe('New Name');
        expect(call[2]).toBe('updated');
        expect(call[3]).toEqual(['fresh']);
    });
});

describe('TemplateLibraryModal — Refresh', () => {
    it('calls RebuildBuildTemplateLibraryIndex and replaces the list with returned entries', async () => {
        const rebuilt = [
            ...sampleEntries,
            {
                id: 'tpl-3',
                name: 'Dropped Template',
                description: 'manual drop',
                tags: [],
                filename: 'dropped-tpl-3.json',
                createdAt: '2026-05-15T12:00:00Z',
                updatedAt: '2026-05-15T12:00:00Z',
                inventoryItems: 4,
                storageItems: 0,
                warnings: 0,
            },
        ];
        mocks.RebuildBuildTemplateLibraryIndex.mockResolvedValue(rebuilt);
        const onRefreshed = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onRefreshed })} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-refresh'));
        });
        await waitFor(() => {
            expect(mocks.RebuildBuildTemplateLibraryIndex).toHaveBeenCalledTimes(1);
        });
        const entries = await screen.findAllByTestId('library-entry');
        expect(entries).toHaveLength(3);
        expect(onRefreshed).toHaveBeenCalled();
        const [list] = onRefreshed.mock.calls[0];
        expect(list).toHaveLength(3);
    });

    it('routes refresh errors through onError', async () => {
        mocks.RebuildBuildTemplateLibraryIndex.mockRejectedValue(new Error('disk full'));
        const onError = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onError })} />);
        await screen.findAllByTestId('library-entry');
        await act(async () => {
            fireEvent.click(screen.getByTestId('library-refresh'));
        });
        await waitFor(() => {
            expect(onError).toHaveBeenCalled();
        });
    });
});

describe('TemplateLibraryModal — Empty state and library path', () => {
    it('shows the new drop-and-refresh empty-state copy', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([]);
        render(<TemplateLibraryModal {...defaultProps()} />);
        const empty = await screen.findByTestId('library-empty');
        expect(empty).toHaveTextContent(/drop .json templates/i);
        expect(empty).toHaveTextContent(/Refresh/);
    });

    it('shows the resolved library path inside the empty state when available', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([]);
        mocks.GetBuildTemplateLibraryPath.mockResolvedValue('/Users/dev/library');
        render(<TemplateLibraryModal {...defaultProps()} />);
        const pathEl = await screen.findByTestId('library-empty-path');
        expect(pathEl).toHaveTextContent('/Users/dev/library');
    });

    it('renders the library path footer when entries are present', async () => {
        mocks.GetBuildTemplateLibraryPath.mockResolvedValue('/Users/dev/library');
        render(<TemplateLibraryModal {...defaultProps()} />);
        const footer = await screen.findByTestId('library-footer-path');
        expect(footer).toHaveTextContent('/Users/dev/library');
    });

    it('degrades gracefully when GetBuildTemplateLibraryPath fails', async () => {
        mocks.ListBuildTemplateLibrary.mockResolvedValue([]);
        mocks.GetBuildTemplateLibraryPath.mockRejectedValue(new Error('no config dir'));
        render(<TemplateLibraryModal {...defaultProps()} />);
        await screen.findByTestId('library-empty');
        // Path element absent — no path-element render path crashes the modal.
        expect(screen.queryByTestId('library-empty-path')).not.toBeInTheDocument();
    });
});

describe('TemplateLibraryModal — allowApply gate', () => {
    it('renders Apply by default (existing caller behavior)', async () => {
        render(<TemplateLibraryModal {...defaultProps()} />);
        const applyBtns = await screen.findAllByTestId('library-apply');
        expect(applyBtns.length).toBeGreaterThan(0);
    });

    it('hides Apply when allowApply={false}', async () => {
        render(<TemplateLibraryModal {...defaultProps({ allowApply: false })} />);
        await screen.findAllByTestId('library-entry');
        expect(screen.queryByTestId('library-apply')).not.toBeInTheDocument();
    });

    it('keeps session-independent actions available when allowApply={false}', async () => {
        render(<TemplateLibraryModal {...defaultProps({ allowApply: false })} />);
        await screen.findAllByTestId('library-entry');
        expect(screen.getAllByTestId('library-preview').length).toBeGreaterThan(0);
        expect(screen.getAllByTestId('library-export').length).toBeGreaterThan(0);
        expect(screen.getAllByTestId('library-rename').length).toBeGreaterThan(0);
        expect(screen.getAllByTestId('library-delete').length).toBeGreaterThan(0);
        expect(screen.getByTestId('library-refresh')).toBeInTheDocument();
    });

    it('uses the custom title when provided', async () => {
        render(<TemplateLibraryModal {...defaultProps({ title: 'Templates' })} />);
        const dialog = await screen.findByRole('dialog');
        expect(dialog).toHaveAttribute('aria-label', 'Templates');
        expect(dialog).toHaveTextContent('Templates');
    });
});

describe('TemplateLibraryModal — Export to file', () => {
    it('calls ExportLibraryBuildTemplateToFile and forwards result via onExportedToFile', async () => {
        mocks.ExportLibraryBuildTemplateToFile.mockResolvedValue({
            path: '/tmp/exported.json',
            warnings: [],
            skippedItems: 0,
        });
        const onExportedToFile = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onExportedToFile })} />);
        const btns = await screen.findAllByTestId('library-export');
        await act(async () => {
            fireEvent.click(btns[0]);
        });
        await waitFor(() => {
            expect(mocks.ExportLibraryBuildTemplateToFile).toHaveBeenCalledWith('tpl-1');
        });
        expect(onExportedToFile).toHaveBeenCalled();
        const [result, entry] = onExportedToFile.mock.calls[0];
        expect(result.path).toBe('/tmp/exported.json');
        expect(entry.id).toBe('tpl-1');
    });
});

describe('TemplateLibraryModal — Phase 2B YAML export', () => {
    it('does not render Export YAML when onExportAsYAML is not provided', async () => {
        render(<TemplateLibraryModal {...defaultProps()} />);
        await screen.findAllByTestId('library-entry');
        expect(screen.queryByTestId('library-export-yaml')).not.toBeInTheDocument();
        // Existing JSON Export must remain visible — Phase 2B is additive.
        expect(screen.getAllByTestId('library-export').length).toBeGreaterThan(0);
    });

    it('renders Export YAML on each entry when onExportAsYAML is provided', async () => {
        const onExportAsYAML = vi.fn();
        render(<TemplateLibraryModal {...defaultProps({ onExportAsYAML })} />);
        const yamlButtons = await screen.findAllByTestId('library-export-yaml');
        expect(yamlButtons).toHaveLength(sampleEntries.length);
    });

    it('Export YAML click forwards the entry to onExportAsYAML', async () => {
        const onExportAsYAML = vi.fn().mockResolvedValue(undefined);
        render(<TemplateLibraryModal {...defaultProps({ onExportAsYAML })} />);
        const btns = await screen.findAllByTestId('library-export-yaml');
        await act(async () => {
            fireEvent.click(btns[0]);
        });
        await waitFor(() => {
            expect(onExportAsYAML).toHaveBeenCalledTimes(1);
        });
        const [entry] = onExportAsYAML.mock.calls[0];
        expect(entry.id).toBe('tpl-1');
    });

    it('keeps Apply hidden when allowApply=false even with YAML export available', async () => {
        const onExportAsYAML = vi.fn();
        render(
            <TemplateLibraryModal
                {...defaultProps({ onExportAsYAML, allowApply: false })}
            />,
        );
        await screen.findAllByTestId('library-entry');
        expect(screen.queryByTestId('library-apply')).not.toBeInTheDocument();
        // YAML export and JSON export remain visible.
        expect(screen.getAllByTestId('library-export-yaml').length).toBeGreaterThan(0);
        expect(screen.getAllByTestId('library-export').length).toBeGreaterThan(0);
    });

    it('routes Export YAML errors through onError', async () => {
        const onError = vi.fn();
        const onExportAsYAML = vi.fn().mockRejectedValue(new Error('disk full'));
        render(<TemplateLibraryModal {...defaultProps({ onError, onExportAsYAML })} />);
        const btns = await screen.findAllByTestId('library-export-yaml');
        await act(async () => {
            fireEvent.click(btns[0]);
        });
        await waitFor(() => {
            expect(onError).toHaveBeenCalled();
        });
    });
});
