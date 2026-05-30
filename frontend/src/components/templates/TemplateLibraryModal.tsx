import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
    ApplyBuildTemplateFromLibrary,
    DeleteBuildTemplateFromLibrary,
    ExportLibraryBuildTemplateToFile,
    GetBuildTemplateLibraryPath,
    ListBuildTemplateLibrary,
    PreviewBuildTemplateFromLibrary,
    RebuildBuildTemplateLibraryIndex,
    RenameBuildTemplateInLibrary,
} from '../../../wailsjs/go/main/App';
import { main, templates } from '../../../wailsjs/go/models';

// TemplateLibraryModal lists locally-stored build templates and exposes
// the actions a user needs to manage them: preview, apply to workspace,
// export to file, rename (inline), delete (with custom confirm).
//
// Delete confirmation lives inside this component as a small inline
// confirm row, not a Wails native dialog — this keeps the flow testable
// under jsdom and matches the rest of the SortOrderTab UI.
//
// Apply still produces a RAM-only workspace mutation; the modal does not
// touch the save file. The parent supplies onApplied which receives the
// new ApplyTemplateResult so it can update workspace state and surface
// the "click Save changes to persist" notice.

interface Props {
    sessionID: string;
    onClose: () => void;
    onApplied: (result: main.ApplyTemplateResult, entry: templates.LibraryTemplateEntry) => void;
    onPreviewed?: (preview: main.LoadedTemplatePreview, entry: templates.LibraryTemplateEntry) => void;
    onError: (err: unknown) => void;
    onExportedToFile?: (result: main.BuildTemplateExportResult, entry: templates.LibraryTemplateEntry) => void;
    // onExportAsYAML, when provided, surfaces an additional per-entry
    // "Export YAML" action alongside the existing JSON Export. Phase 1
    // callers (SortOrderTab) omit it and keep JSON-only behaviour; the
    // Phase 2B global Templates shell passes it to drive the public
    // YAML sharing format.
    onExportAsYAML?: (entry: templates.LibraryTemplateEntry) => void | Promise<void>;
    onDeleted?: (id: string) => void;
    // onRefreshed fires after a successful rebuild so the parent can
    // raise a toast or other ambient signal. Receives the post-rebuild
    // entry list for parity with the modal's internal state.
    onRefreshed?: (entries: templates.LibraryTemplateEntry[]) => void;
    // allowApply gates the Apply action. The global Templates shell mounted
    // from the sidebar has no active InventoryEditSession, so it passes
    // false to suppress the action entirely (rather than rendering a
    // permanently-disabled button). Defaults to true to preserve the
    // existing SortOrderTab caller behavior.
    allowApply?: boolean;
    // title overrides the modal headline. Defaults to the v1 wording so
    // existing callers are unaffected; the global shell passes "Templates".
    title?: string;
    // headerExtras renders additional action buttons in the modal header
    // next to Refresh and Close. The Phase 2B global shell uses this slot
    // to mount its "Import YAML from File…" entry point. Existing callers
    // (SortOrderTab) omit it for a no-op.
    headerExtras?: ReactNode;
    // reloadSignal lets a parent imperatively trigger a list refetch
    // without unmounting the modal. Every increment re-runs the same
    // ListBuildTemplateLibrary path used on mount and by the Refresh
    // button. The Phase 2B shell bumps this after a successful YAML
    // import-to-library so the new entry appears immediately.
    reloadSignal?: number;
}

export function TemplateLibraryModal({
    sessionID,
    onClose,
    onApplied,
    onPreviewed,
    onError,
    onExportedToFile,
    onExportAsYAML,
    onDeleted,
    onRefreshed,
    allowApply = true,
    title = 'Build Template Library',
    headerExtras,
    reloadSignal,
}: Props) {
    const [entries, setEntries] = useState<templates.LibraryTemplateEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [refreshing, setRefreshing] = useState(false);
    const [busyID, setBusyID] = useState<string>('');
    const [confirmDeleteID, setConfirmDeleteID] = useState<string>('');
    const [editingID, setEditingID] = useState<string>('');
    const [editName, setEditName] = useState('');
    const [editDescription, setEditDescription] = useState('');
    const [editTags, setEditTags] = useState('');
    const [libraryPath, setLibraryPath] = useState<string>('');

    const dialogRef = useRef<HTMLDivElement | null>(null);

    const refresh = useCallback(async () => {
        setLoading(true);
        try {
            const list = await ListBuildTemplateLibrary();
            setEntries(list ?? []);
        } catch (err) {
            onError(err);
        } finally {
            setLoading(false);
        }
    }, [onError]);

    useEffect(() => {
        dialogRef.current?.focus();
        refresh();
        // Library path is fetched once per modal open. The directory
        // does not move at runtime, so refetching on every render
        // would be wasted IPC. Errors are swallowed silently — the
        // footer + empty-state copy degrade gracefully to no path.
        GetBuildTemplateLibraryPath()
            .then(setLibraryPath)
            .catch(() => setLibraryPath(''));
    }, [refresh]);

    // Parent-driven reload. We deliberately omit reloadSignal=undefined
    // from triggering on first mount: refresh() is already called by the
    // mount effect above, so the initial render does not double-fetch.
    const lastReloadSignal = useRef<number | undefined>(reloadSignal);
    useEffect(() => {
        if (reloadSignal === undefined) return;
        if (reloadSignal === lastReloadSignal.current) return;
        lastReloadSignal.current = reloadSignal;
        refresh();
    }, [reloadSignal, refresh]);

    const onRefreshLibrary = async () => {
        setRefreshing(true);
        try {
            const list = await RebuildBuildTemplateLibraryIndex();
            const next = list ?? [];
            setEntries(next);
            onRefreshed?.(next);
        } catch (err) {
            onError(err);
        } finally {
            setRefreshing(false);
        }
    };

    const onPreview = async (entry: templates.LibraryTemplateEntry) => {
        setBusyID(entry.id);
        try {
            const preview = await PreviewBuildTemplateFromLibrary(entry.id);
            onPreviewed?.(preview, entry);
        } catch (err) {
            onError(err);
        } finally {
            setBusyID('');
        }
    };

    const onApply = async (entry: templates.LibraryTemplateEntry) => {
        if (!sessionID) {
            onError(new Error('No active workspace session; open a character first.'));
            return;
        }
        setBusyID(entry.id);
        try {
            const result = await ApplyBuildTemplateFromLibrary(
                sessionID,
                entry.id,
                main.ApplyTemplateOptions.createFrom({ mode: 'append' }),
            );
            onApplied(result, entry);
        } catch (err) {
            onError(err);
        } finally {
            setBusyID('');
        }
    };

    const onExport = async (entry: templates.LibraryTemplateEntry) => {
        setBusyID(entry.id);
        try {
            const result = await ExportLibraryBuildTemplateToFile(entry.id);
            onExportedToFile?.(result, entry);
        } catch (err) {
            onError(err);
        } finally {
            setBusyID('');
        }
    };

    const onExportYAML = async (entry: templates.LibraryTemplateEntry) => {
        if (!onExportAsYAML) return;
        setBusyID(entry.id);
        try {
            await onExportAsYAML(entry);
        } catch (err) {
            onError(err);
        } finally {
            setBusyID('');
        }
    };

    const onDelete = (entry: templates.LibraryTemplateEntry) => {
        setConfirmDeleteID(entry.id);
    };

    const onConfirmDelete = async (entry: templates.LibraryTemplateEntry) => {
        setBusyID(entry.id);
        try {
            await DeleteBuildTemplateFromLibrary(entry.id);
            onDeleted?.(entry.id);
            await refresh();
        } catch (err) {
            onError(err);
        } finally {
            setBusyID('');
            setConfirmDeleteID('');
        }
    };

    const onRenameStart = (entry: templates.LibraryTemplateEntry) => {
        setEditingID(entry.id);
        setEditName(entry.name ?? '');
        setEditDescription(entry.description ?? '');
        setEditTags((entry.tags ?? []).join(', '));
    };

    const onRenameCancel = () => {
        setEditingID('');
        setEditName('');
        setEditDescription('');
        setEditTags('');
    };

    const onRenameSave = async (entry: templates.LibraryTemplateEntry) => {
        setBusyID(entry.id);
        try {
            const tags = editTags
                .split(',')
                .map(t => t.trim())
                .filter(t => t.length > 0);
            await RenameBuildTemplateInLibrary(entry.id, editName, editDescription, tags);
            setEditingID('');
            await refresh();
        } catch (err) {
            onError(err);
        } finally {
            setBusyID('');
        }
    };

    const empty = useMemo(() => !loading && entries.length === 0, [loading, entries]);

    return (
        <div
            data-testid="template-library-modal"
            role="dialog"
            aria-modal="true"
            aria-label={title}
            ref={dialogRef}
            tabIndex={-1}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
        >
            <div className="w-full max-w-3xl rounded-lg bg-card border border-border/60 shadow-xl">
                <div className="px-4 py-3 border-b border-border/60 flex items-center justify-between gap-3">
                    <div className="min-w-0">
                        <h2 className="text-sm font-black uppercase tracking-wider">{title}</h2>
                        <p className="mt-1 text-[11px] text-muted-foreground">
                            Saved templates from this device. Apply to workspace stages a RAM-only change — click
                            <strong className="px-1">Save changes</strong>
                            to persist.
                        </p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                        {headerExtras}
                        <button
                            type="button"
                            data-testid="library-refresh"
                            onClick={onRefreshLibrary}
                            disabled={refreshing}
                            title="Rescan the templates folder and rebuild the index."
                            className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40"
                        >
                            {refreshing ? 'Refreshing…' : 'Refresh'}
                        </button>
                        <button
                            type="button"
                            onClick={onClose}
                            className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                        >
                            Close
                        </button>
                    </div>
                </div>

                <div className="max-h-[60vh] overflow-y-auto px-4 py-3 text-[12px]">
                    {loading && <div data-testid="library-loading">Loading…</div>}
                    {empty && (
                        <div data-testid="library-empty" className="text-muted-foreground py-6 text-center space-y-2">
                            <div>
                                Your local template library is empty. Export a template from the current workspace
                                or drop <code>.json</code> templates into the templates folder, then click Refresh.
                            </div>
                            {libraryPath && (
                                <div data-testid="library-empty-path" className="text-[10px] font-mono break-all">
                                    {libraryPath}
                                </div>
                            )}
                        </div>
                    )}
                    {!empty && libraryPath && (
                        <div
                            data-testid="library-footer-path"
                            className="mb-3 rounded border border-border/40 bg-background/40 px-3 py-1.5 text-[10px] text-muted-foreground"
                        >
                            <span className="font-bold uppercase tracking-wider">Library folder:</span>{' '}
                            <span className="font-mono break-all">{libraryPath}</span>
                        </div>
                    )}
                    {!empty && (
                        <ul className="space-y-2">
                            {entries.map(entry => {
                                const busy = busyID === entry.id;
                                const editing = editingID === entry.id;
                                const confirming = confirmDeleteID === entry.id;
                                return (
                                    <li
                                        key={entry.id}
                                        data-testid="library-entry"
                                        data-entry-id={entry.id}
                                        className="rounded border border-border/40 bg-background/40 px-3 py-2"
                                    >
                                        {!editing && (
                                            <div className="flex items-start justify-between gap-2">
                                                <div className="min-w-0 flex-1">
                                                    <div className="font-semibold truncate" data-testid="library-entry-name">
                                                        {entry.name || '(unnamed)'}
                                                    </div>
                                                    {entry.description && (
                                                        <div className="text-[11px] text-muted-foreground mt-0.5">
                                                            {entry.description}
                                                        </div>
                                                    )}
                                                    <div className="text-[10px] text-muted-foreground mt-1 flex flex-wrap gap-x-3">
                                                        <span>
                                                            {entry.inventoryItems} inv / {entry.storageItems} storage
                                                        </span>
                                                        {entry.tags && entry.tags.length > 0 && (
                                                            <span>tags: {entry.tags.join(', ')}</span>
                                                        )}
                                                        {entry.updatedAt && (
                                                            <span>updated: {entry.updatedAt.slice(0, 19).replace('T', ' ')}</span>
                                                        )}
                                                    </div>
                                                </div>
                                                <div className="flex flex-col gap-1 shrink-0">
                                                    <button
                                                        type="button"
                                                        data-testid="library-preview"
                                                        disabled={busy}
                                                        onClick={() => onPreview(entry)}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 hover:bg-muted/40 disabled:opacity-40"
                                                    >
                                                        Preview
                                                    </button>
                                                    {allowApply && (
                                                        <button
                                                            type="button"
                                                            data-testid="library-apply"
                                                            disabled={busy || !sessionID}
                                                            onClick={() => onApply(entry)}
                                                            className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded bg-green-700/80 text-white hover:bg-green-700 disabled:opacity-40"
                                                        >
                                                            Apply
                                                        </button>
                                                    )}
                                                    <button
                                                        type="button"
                                                        data-testid="library-export"
                                                        disabled={busy}
                                                        onClick={() => onExport(entry)}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 hover:bg-muted/40 disabled:opacity-40"
                                                    >
                                                        Export
                                                    </button>
                                                    {onExportAsYAML && (
                                                        <button
                                                            type="button"
                                                            data-testid="library-export-yaml"
                                                            disabled={busy}
                                                            onClick={() => onExportYAML(entry)}
                                                            className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 hover:bg-muted/40 disabled:opacity-40"
                                                        >
                                                            Export YAML
                                                        </button>
                                                    )}
                                                    <button
                                                        type="button"
                                                        data-testid="library-rename"
                                                        disabled={busy}
                                                        onClick={() => onRenameStart(entry)}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 hover:bg-muted/40 disabled:opacity-40"
                                                    >
                                                        Rename
                                                    </button>
                                                    <button
                                                        type="button"
                                                        data-testid="library-delete"
                                                        disabled={busy}
                                                        onClick={() => onDelete(entry)}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-red-700/60 text-red-300 hover:bg-red-900/30 disabled:opacity-40"
                                                    >
                                                        Delete
                                                    </button>
                                                </div>
                                            </div>
                                        )}

                                        {editing && (
                                            <div data-testid="library-rename-form" className="space-y-2">
                                                <input
                                                    type="text"
                                                    aria-label="Rename: name"
                                                    data-testid="library-rename-name"
                                                    value={editName}
                                                    onChange={e => setEditName(e.target.value)}
                                                    className="w-full rounded border border-border/60 bg-background/60 px-2 py-1 text-foreground"
                                                />
                                                <textarea
                                                    aria-label="Rename: description"
                                                    data-testid="library-rename-description"
                                                    value={editDescription}
                                                    onChange={e => setEditDescription(e.target.value)}
                                                    rows={2}
                                                    className="w-full rounded border border-border/60 bg-background/60 px-2 py-1 text-foreground"
                                                />
                                                <input
                                                    type="text"
                                                    aria-label="Rename: tags"
                                                    data-testid="library-rename-tags"
                                                    value={editTags}
                                                    onChange={e => setEditTags(e.target.value)}
                                                    placeholder="comma-separated tags"
                                                    className="w-full rounded border border-border/60 bg-background/60 px-2 py-1 text-foreground"
                                                />
                                                <div className="flex justify-end gap-2">
                                                    <button
                                                        type="button"
                                                        onClick={onRenameCancel}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 hover:bg-muted/40"
                                                    >
                                                        Cancel
                                                    </button>
                                                    <button
                                                        type="button"
                                                        data-testid="library-rename-save"
                                                        onClick={() => onRenameSave(entry)}
                                                        disabled={busy}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded bg-green-700/80 text-white hover:bg-green-700 disabled:opacity-40"
                                                    >
                                                        Save
                                                    </button>
                                                </div>
                                            </div>
                                        )}

                                        {confirming && (
                                            <div
                                                data-testid="library-delete-confirm"
                                                className="mt-2 rounded border border-red-700/40 bg-red-900/20 px-3 py-2"
                                            >
                                                <div className="text-[11px]">
                                                    Delete <strong>{entry.name || '(unnamed)'}</strong> permanently? This removes the
                                                    template file from disk.
                                                </div>
                                                <div className="mt-2 flex justify-end gap-2">
                                                    <button
                                                        type="button"
                                                        onClick={() => setConfirmDeleteID('')}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 hover:bg-muted/40"
                                                    >
                                                        Cancel
                                                    </button>
                                                    <button
                                                        type="button"
                                                        data-testid="library-delete-confirm-yes"
                                                        onClick={() => onConfirmDelete(entry)}
                                                        disabled={busy}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded bg-red-700 text-white hover:bg-red-600 disabled:opacity-40"
                                                    >
                                                        Delete
                                                    </button>
                                                </div>
                                            </div>
                                        )}
                                    </li>
                                );
                            })}
                        </ul>
                    )}
                </div>
            </div>
        </div>
    );
}
