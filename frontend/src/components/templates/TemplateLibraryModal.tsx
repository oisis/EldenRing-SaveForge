import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
    ApplyBuildTemplateFromLibrary,
    DeleteBuildTemplateFromLibrary,
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
    // Phase 8A removed the public JSON file export. onExportAsYAML, when
    // provided, surfaces the per-entry "Export YAML" action — now the
    // sole public exchange format.
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
    // Phase 5D.1 — v2 library apply props. charIndex/saveLoaded gate the
    // Apply button for schema v2 entries; onApplyV2 is invoked from the
    // inline confirm row (the modal owns confirmation state, the parent
    // owns the binding call + toasts + post-apply state refresh).
    // Existing v1 callers can omit all three — Apply for v1 entries
    // still drives onApply with sessionID exactly as before.
    charIndex?: number;
    saveLoaded?: boolean;
    onApplyV2?: (entry: templates.LibraryTemplateEntry) => Promise<void> | void;
    // Phase 6 — v2 apply with editable overrides. Rendered as a second
    // button next to the existing Apply for v2 entries whose
    // selectedSections fall inside { profile, stats }. v1 entries never
    // render this button. Disabled under the same conditions as the
    // plain v2 Apply (no saveLoaded, no charIndex, unsupported sections).
    // The parent owns the overrides modal and the eventual JSON apply;
    // this callback is only the trigger.
    onApplyV2WithOverrides?: (entry: templates.LibraryTemplateEntry) => Promise<void> | void;
}

export function TemplateLibraryModal({
    sessionID,
    onClose,
    onApplied,
    onPreviewed,
    onError,
    onExportAsYAML,
    onDeleted,
    onRefreshed,
    allowApply = true,
    title = 'Build Template Library',
    headerExtras,
    reloadSignal,
    charIndex,
    saveLoaded,
    onApplyV2,
    onApplyV2WithOverrides,
}: Props) {
    const [entries, setEntries] = useState<templates.LibraryTemplateEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [refreshing, setRefreshing] = useState(false);
    const [busyID, setBusyID] = useState<string>('');
    const [confirmDeleteID, setConfirmDeleteID] = useState<string>('');
    const [confirmApplyV2ID, setConfirmApplyV2ID] = useState<string>('');
    const [applyV2BusyID, setApplyV2BusyID] = useState<string>('');
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

    const onApplyV2Click = (entry: templates.LibraryTemplateEntry) => {
        setConfirmApplyV2ID(entry.id);
    };

    const onApplyV2Cancel = () => {
        setConfirmApplyV2ID('');
    };

    const onApplyV2Confirm = async (entry: templates.LibraryTemplateEntry) => {
        if (!onApplyV2) return;
        setApplyV2BusyID(entry.id);
        try {
            await onApplyV2(entry);
            // Close confirm only after the parent handler resolves
            // cleanly. On throw we leave the row open so the user can
            // react to the error (toast surfaced by the caller).
            setConfirmApplyV2ID('');
        } catch {
            /* keep confirm row open; caller is responsible for the toast */
        } finally {
            setApplyV2BusyID('');
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
                <div className="px-4 py-3 border-b border-border/60 flex flex-col gap-2">
                    <div className="min-w-0">
                        <h2 className="text-sm font-black uppercase tracking-wider">{title}</h2>
                        <p className="mt-1 text-[11px] text-muted-foreground">
                            Saved templates from this device. Apply to workspace stages a RAM-only change — click
                            <strong className="px-1">Save changes</strong>
                            to persist.
                        </p>
                    </div>
                    <div className="flex flex-wrap items-center justify-center gap-2">
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
                                const confirmingApplyV2 = confirmApplyV2ID === entry.id;
                                const isV2 = (entry.version ?? 0) >= 2;
                                const selectedSections = entry.selectedSections ?? [];
                                // Phase 7a — inventory.workspace joins profile/stats as an
                                // apply-eligible section. The session lookup happens in the
                                // shell after the user clicks Apply; here we only widen the
                                // visibility gate.
                                //
                                // Phase 7b.1 — equipment joins as a section eligible for fast
                                // Apply. Equipment-only templates do NOT require a session;
                                // the equipment + inventory.workspace combo is hard-rejected
                                // at backend preview time, so we leave Apply enabled here and
                                // let the shell surface the resulting error.
                                //
                                // Phase 7d.4 — spells join as an apply-eligible section.
                                // Spells-only templates do NOT require a session; WriteSpells
                                // writes directly into slot.Data and recomputes hash[10].
                                //
                                // Phase 8D.2 — items joins as an apply-eligible section
                                // (add-missing only).
                                //
                                // Phase 8E.2 — inventoryLayout / storageLayout become
                                // apply-eligible too, as reorder-only. A layout-only
                                // template is applyable when an active session is open;
                                // items + layout applies items first (add-missing) and
                                // then reorders the workspace via the layout writer.
                                const hasItemsSection = selectedSections.includes('items');
                                const hasLayoutSection =
                                    selectedSections.includes('inventoryLayout') ||
                                    selectedSections.includes('storageLayout');
                                const v2HasApplyableSections =
                                    selectedSections.includes('profile') ||
                                    selectedSections.includes('stats') ||
                                    selectedSections.includes('inventory.workspace') ||
                                    selectedSections.includes('equipment') ||
                                    selectedSections.includes('spells') ||
                                    hasItemsSection ||
                                    hasLayoutSection;

                                // Per-entry Apply gating. v1 path is
                                // preserved verbatim: requires sessionID,
                                // calls onApply, never opens the v2
                                // confirm row. v2 entries route through
                                // the inline confirm + parent onApplyV2
                                // callback; the disabled reason is
                                // surfaced via the title/aria-label so
                                // hover and screen readers explain why
                                // the button is inert.
                                let applyDisabled: boolean;
                                let applyTitle: string | undefined;
                                let applyAriaLabel: string;
                                let applyHandler: () => void;
                                if (!isV2) {
                                    applyDisabled = busy || !sessionID;
                                    applyTitle = undefined;
                                    applyAriaLabel = 'Apply';
                                    applyHandler = () => onApply(entry);
                                } else if (!v2HasApplyableSections) {
                                    applyDisabled = true;
                                    applyTitle = 'This schema v2 template has no apply-eligible sections.';
                                    applyAriaLabel = applyTitle;
                                    applyHandler = () => {};
                                } else if (!onApplyV2) {
                                    applyDisabled = true;
                                    applyTitle = 'Apply handler is not available';
                                    applyAriaLabel = applyTitle;
                                    applyHandler = () => {};
                                } else if (!saveLoaded) {
                                    applyDisabled = true;
                                    applyTitle = 'Load a save to apply this template';
                                    applyAriaLabel = applyTitle;
                                    applyHandler = () => {};
                                } else if (charIndex === undefined) {
                                    applyDisabled = true;
                                    applyTitle = 'Select a character to apply this template';
                                    applyAriaLabel = applyTitle;
                                    applyHandler = () => {};
                                } else {
                                    applyDisabled = applyV2BusyID === entry.id || confirmingApplyV2;
                                    applyTitle = `Apply schema v2 template to character slot ${charIndex + 1}`;
                                    applyAriaLabel = 'Apply';
                                    applyHandler = () => onApplyV2Click(entry);
                                }

                                // Phase 6 — "Apply with overrides…" mirrors the
                                // v2 Apply gating but routes through the parent's
                                // overrides callback. Visible only for v2 entries
                                // with applyable sections; v1 entries omit the
                                // button entirely.
                                const overridesVisible =
                                    isV2 && v2HasApplyableSections && !!onApplyV2WithOverrides;
                                let overridesDisabled = true;
                                let overridesTitle: string | undefined;
                                if (overridesVisible) {
                                    if (!saveLoaded) {
                                        overridesTitle = 'Load a save to apply this template';
                                    } else if (charIndex === undefined) {
                                        overridesTitle = 'Select a character to apply this template';
                                    } else if (applyV2BusyID === entry.id || confirmingApplyV2) {
                                        overridesTitle = 'Apply already in progress.';
                                    } else {
                                        overridesDisabled = false;
                                        overridesTitle = `Edit values, then apply schema v2 to character slot ${charIndex + 1}`;
                                    }
                                }
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
                                                    <div
                                                        className="font-semibold truncate flex items-center gap-2"
                                                        data-testid="library-entry-name"
                                                    >
                                                        <span className="truncate">{entry.name || '(unnamed)'}</span>
                                                        {isV2 && (
                                                            <span
                                                                data-testid="library-entry-v2-badge"
                                                                title="Schema v2 template (profile / stats)"
                                                                className="px-1.5 py-0.5 rounded text-[9px] font-black uppercase tracking-wider border border-blue-500/40 bg-blue-500/10 text-blue-600 shrink-0"
                                                            >
                                                                v2
                                                            </span>
                                                        )}
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
                                                        {entry.selectedSections && entry.selectedSections.length > 0 && (
                                                            <span data-testid="library-entry-sections">
                                                                sections: {entry.selectedSections.join(', ')}
                                                            </span>
                                                        )}
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
                                                            disabled={applyDisabled}
                                                            onClick={applyHandler}
                                                            title={applyTitle}
                                                            aria-label={applyAriaLabel}
                                                            className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded bg-green-700/80 text-white hover:bg-green-700 disabled:opacity-40"
                                                        >
                                                            Apply
                                                        </button>
                                                    )}
                                                    {allowApply && overridesVisible && (
                                                        <button
                                                            type="button"
                                                            data-testid="library-apply-overrides"
                                                            disabled={overridesDisabled}
                                                            onClick={() => onApplyV2WithOverrides?.(entry)}
                                                            title={overridesTitle}
                                                            aria-label={overridesTitle ?? 'Apply with overrides'}
                                                            className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 disabled:opacity-40"
                                                        >
                                                            Apply with overrides…
                                                        </button>
                                                    )}
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

                                        {confirmingApplyV2 && (
                                            <div
                                                data-testid="library-apply-v2-confirm"
                                                className="mt-2 rounded border border-green-700/40 bg-green-900/20 px-3 py-2 text-[11px] space-y-1"
                                            >
                                                <div>
                                                    Apply <strong>{entry.name || '(unnamed)'}</strong>
                                                    {charIndex !== undefined && (
                                                        <> to character slot <strong>{charIndex + 1}</strong></>
                                                    )}?
                                                </div>
                                                {selectedSections.length > 0 && (
                                                    <div data-testid="library-apply-v2-sections" className="text-muted-foreground">
                                                        Sections: <span className="font-bold">{selectedSections.join(', ')}</span>
                                                    </div>
                                                )}
                                                <div className="text-warning-foreground">
                                                    This will overwrite the selected profile/stat fields on the selected character.
                                                </div>
                                                {selectedSections.includes('profile') && (
                                                    <div data-testid="library-apply-v2-class-skipped" className="text-muted-foreground italic">
                                                        Class changes are skipped in this phase.
                                                    </div>
                                                )}
                                                {hasItemsSection && (
                                                    <>
                                                        <div
                                                            data-testid="library-apply-v2-items-mode"
                                                            className="text-muted-foreground"
                                                        >
                                                            Items mode: <span className="font-bold">Add missing only</span> — existing items are preserved.
                                                        </div>
                                                        <div
                                                            data-testid="library-apply-v2-weapon-hint"
                                                            className="text-muted-foreground italic"
                                                        >
                                                            Direct Apply uses template / default upgrade levels. Use “Apply with overrides…” to override weapon levels for newly added items.
                                                        </div>
                                                    </>
                                                )}
                                                {hasLayoutSection && (
                                                    <div
                                                        data-testid="library-apply-v2-layout-reorder-only"
                                                        className="text-muted-foreground space-y-0.5"
                                                    >
                                                        <div>
                                                            Layout mode: <span className="font-bold">Reorder only</span> — no items are added or removed.
                                                        </div>
                                                        {hasItemsSection ? (
                                                            <div>
                                                                Missing items are added first, then layout is applied.
                                                            </div>
                                                        ) : (
                                                            <div>
                                                                Layout reorders matching workspace items. Extras are preserved and appended; missing entries are skipped with warnings.
                                                            </div>
                                                        )}
                                                    </div>
                                                )}
                                                <div className="text-muted-foreground">
                                                    Use Save/Write Save to persist changes to disk.
                                                </div>
                                                <div className="mt-2 flex justify-end gap-2">
                                                    <button
                                                        type="button"
                                                        data-testid="library-apply-v2-cancel-button"
                                                        onClick={onApplyV2Cancel}
                                                        disabled={applyV2BusyID === entry.id}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 hover:bg-muted/40 disabled:opacity-40"
                                                    >
                                                        Cancel
                                                    </button>
                                                    <button
                                                        type="button"
                                                        data-testid="library-apply-v2-confirm-button"
                                                        onClick={() => onApplyV2Confirm(entry)}
                                                        disabled={applyV2BusyID === entry.id}
                                                        className="px-2 py-1 text-[10px] font-black uppercase tracking-wider rounded bg-green-700/80 text-white hover:bg-green-700 disabled:opacity-40"
                                                    >
                                                        {applyV2BusyID === entry.id ? 'Applying…' : 'Apply to Character'}
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
