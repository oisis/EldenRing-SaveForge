import { useCallback, useState } from 'react';
import toast from '../../lib/toast';
import {
    ApplyBuildTemplateV2FromLibraryToCharacter,
    ApplyBuildTemplateV2ToCharacterJSON,
    ExportLibraryBuildTemplateAsYAMLToFile,
    PreviewBuildTemplateFromLibrary,
    PreviewBuildTemplateImportYAMLFromFile,
    SaveImportedBuildTemplateJSONToLibrary,
} from '../../../wailsjs/go/main/App';
import { main, templates } from '../../../wailsjs/go/models';
import { TemplateLibraryModal } from './TemplateLibraryModal';
import { ImportTemplatePreviewModal, isCancelledPreview } from './ImportTemplatePreviewModal';
import { CreateTemplateV2Modal } from './CreateTemplateV2Modal';
import { ApplyOverridesModal } from './ApplyOverridesPanel';

// TemplatesShellModal is the global, sidebar-mounted Templates surface.
// Phase 1 scope: library-only. Apply / Create-from-current-workspace
// stay on the existing SortOrderTab dropdown because they require an
// active InventoryEditSession, which the global shell does not own.
// See spec/56 §6 and §17 Phase 1.
//
// Phase 2B adds public-YAML sharing without giving the global shell an
// Apply path:
//   - per-entry "Export YAML" wired through TemplateLibraryModal's new
//     onExportAsYAML prop;
//   - a top-level "Import YAML from File…" action that opens the
//     existing ImportTemplatePreviewModal in import-to-library mode;
//   - canonical JSON returned by the YAML preview is held in component
//     state and handed verbatim to SaveImportedBuildTemplateJSONToLibrary
//     when the user clicks Save to Library (anti-TOCTOU contract — the
//     file on disk is never re-read between Preview and Save).
//
// Two distinct preview shapes coexist:
//   - libraryPreview: user clicked Preview on an existing library entry
//     (read-only; no Save to Library, no Apply);
//   - importedPreview: user just imported a YAML file (read-only vs
//     save/workspace; gains Save to Library when report.ok; no Apply).

interface Props {
    onClose: () => void;
    // charIndex/saveLoaded gate the Phase 3D.2b "Create from Character…"
    // action and the Phase 5D.1 v2 library-apply confirm flow. The
    // library listing remains usable without a loaded save; only the
    // create and v2 apply buttons react to them.
    charIndex?: number;
    saveLoaded?: boolean;
    // onCharacterTemplateApplied fires after a successful v2 library
    // apply so App can refresh sidebar / undo depth / tab state. The
    // backend has already mutated the slot in RAM by this point; the
    // callback is the bridge from shell-local apply to global state.
    onCharacterTemplateApplied?: (charIndex: number) => void;
}

// ImportedYAMLPreview bundles the report with the canonical JSON the
// backend re-serialised from the parsed YAML. The string is fed back to
// SaveImportedBuildTemplateJSONToLibrary so the bytes that pass through
// validation in Preview are the exact bytes persisted at Save time.
interface ImportedYAMLPreview {
    report: templates.ImportPreviewReport;
    canonicalJSON: string;
    path: string;
}

// OverridesSource — Phase 6. Identifies which surface opened the
// overrides modal so the success / failure handler can:
//   * close the right parent state (importedPreview vs library row),
//   * surface the right label in the toast,
//   * leave fast-Apply paths from the library untouched.
type OverridesSource =
    | { kind: 'import'; canonicalJSON: string; sourceLabel: string; path: string }
    | { kind: 'library'; canonicalJSON: string; sourceLabel: string; entryID: string };

export function TemplatesShellModal({ onClose, charIndex, saveLoaded, onCharacterTemplateApplied }: Props) {
    const [libraryPreview, setLibraryPreview] = useState<templates.ImportPreviewReport | null>(null);
    const [importedPreview, setImportedPreview] = useState<ImportedYAMLPreview | null>(null);
    const [importing, setImporting] = useState(false);
    const [savingToLibrary, setSavingToLibrary] = useState(false);
    const [applyingV2FromImport, setApplyingV2FromImport] = useState(false);
    const [createTemplateOpen, setCreateTemplateOpen] = useState(false);
    // Phase 6 — apply-with-overrides shared state. A single modal handles
    // both the direct-import and the library entry-points; the source
    // discriminator decides which parent state to close + which label to
    // surface in the toast.
    const [overridesSource, setOverridesSource] = useState<OverridesSource | null>(null);
    const [applyingV2WithOverrides, setApplyingV2WithOverrides] = useState(false);
    const [openingOverridesFromLibrary, setOpeningOverridesFromLibrary] = useState(false);
    // libraryReloadSignal tells TemplateLibraryModal to re-run its
    // ListBuildTemplateLibrary fetch without unmounting. Bumping the
    // signal after a successful YAML import surfaces the new entry
    // immediately while preserving the modal's existing state
    // (selection, edit mode, etc.).
    const [libraryReloadSignal, setLibraryReloadSignal] = useState(0);

    const refreshLibrary = useCallback(() => {
        setLibraryReloadSignal(s => s + 1);
    }, []);

    const onExportAsYAML = useCallback(
        async (entry: templates.LibraryTemplateEntry) => {
            const result = await ExportLibraryBuildTemplateAsYAMLToFile(entry.id);
            // Backend returns an empty Path when the user cancelled the
            // native save-file dialog. Silent no-op for cancel, toast on
            // real success.
            if (result?.path) {
                toast.success(`Template "${entry.name || entry.id}" exported as YAML to ${result.path}`);
            }
        },
        [],
    );

    const onImportYAML = useCallback(async () => {
        if (importing) return;
        setImporting(true);
        try {
            const bundle = await PreviewBuildTemplateImportYAMLFromFile();
            if (!bundle || isCancelledPreview(bundle.report)) {
                // User backed out of the open-file dialog. Silent no-op.
                return;
            }
            setImportedPreview({
                report: bundle.report,
                canonicalJSON: bundle.json ?? '',
                path: bundle.path ?? '',
            });
        } catch (err) {
            toast.error(`Templates: ${String(err)}`);
        } finally {
            setImporting(false);
        }
    }, [importing]);

    const onLibraryError = useCallback((err: unknown) => toast.error(`Templates: ${String(err)}`), []);
    const onLibraryApplied = useCallback(() => {
        /* v1 Apply requires sessionID; shell passes "" so v1 Apply is
           always disabled here. v2 entries route through onApplyV2
           instead, so this v1 callback never fires from the shell. */
    }, []);
    const onLibraryPreviewed = useCallback((preview: main.LoadedTemplatePreview) => {
        // Preview of an existing library entry — read-only, no Save to
        // Library, no Apply. Routed to a separate state slot so the
        // imported-YAML preview cannot be confused with it.
        setLibraryPreview(preview.report);
    }, []);
    const onLibraryExportedToFile = useCallback(
        (result: main.BuildTemplateExportResult, entry: templates.LibraryTemplateEntry) => {
            if (result.path) {
                toast.success(`Template "${entry.name || entry.id}" exported to ${result.path}`);
            }
        },
        [],
    );
    const onLibraryDeleted = useCallback(
        (id: string) => toast.success(`Template ${id} deleted from library.`),
        [],
    );
    const onLibraryRefreshed = useCallback(
        (list: templates.LibraryTemplateEntry[]) => toast.success(`Template library refreshed (${list.length} entries).`),
        [],
    );

    const handleApplyV2FromLibrary = useCallback(
        async (entry: templates.LibraryTemplateEntry) => {
            // The library modal disables the button under the same
            // conditions, but the guard belongs here too — the parent
            // owns the bindings call and must never invoke it without a
            // loaded save + selected character.
            if (!saveLoaded || charIndex === undefined) {
                const msg = 'Load a save and select a character before applying a v2 template.';
                toast.error(`Templates: ${msg}`);
                throw new Error(msg);
            }
            let result: main.ApplyTemplateV2Result;
            try {
                result = await ApplyBuildTemplateV2FromLibraryToCharacter(
                    charIndex,
                    entry.id,
                    main.ApplyTemplateV2Options.createFrom({ mode: 'append' }),
                );
            } catch (err) {
                toast.error(`Templates: ${String(err)}`);
                throw err;
            }
            if (!result.applied) {
                const firstErr = result.preview?.errors?.[0]?.message ?? 'Apply did not complete.';
                toast.error(`Templates: ${firstErr}`);
                throw new Error(firstErr);
            }
            toast.success(
                `Applied "${entry.name || entry.id}" to character slot ${charIndex + 1}.`,
            );
            if ((result.skippedFields ?? []).includes('profile.class')) {
                // toast() is the info channel in this codebase — see lib/toast.ts.
                toast('Class was skipped in this phase.');
            }
            onCharacterTemplateApplied?.(charIndex);
        },
        [saveLoaded, charIndex, onCharacterTemplateApplied],
    );

    const onCreateV2Saved = useCallback(
        (entry: templates.LibraryTemplateEntry) => {
            toast.success(`Template "${entry.name || entry.id}" saved to library.`);
            setCreateTemplateOpen(false);
            refreshLibrary();
        },
        [refreshLibrary],
    );
    const onCreateV2Error = useCallback((err: unknown) => {
        toast.error(`Templates: ${String(err)}`);
    }, []);

    const createDisabled = !saveLoaded || charIndex === undefined;
    const createTitle = !saveLoaded
        ? 'Load a save to create a character template'
        : charIndex === undefined
            ? 'Select a character to create a template'
            : 'Create template from selected character';

    const onSaveImportedToLibrary = useCallback(async () => {
        if (!importedPreview) return;
        if (!importedPreview.report.ok) return;
        if (!importedPreview.canonicalJSON) return;
        if (savingToLibrary) return;
        setSavingToLibrary(true);
        try {
            const entry = await SaveImportedBuildTemplateJSONToLibrary(importedPreview.canonicalJSON);
            toast.success(`Template "${entry.name || entry.id}" saved to library.`);
            setImportedPreview(null);
            refreshLibrary();
        } catch (err) {
            toast.error(`Templates: ${String(err)}`);
        } finally {
            setSavingToLibrary(false);
        }
    }, [importedPreview, savingToLibrary, refreshLibrary]);

    const handleOpenOverridesFromImport = useCallback(() => {
        if (!importedPreview) return;
        if (!importedPreview.report.ok) return;
        if (!importedPreview.canonicalJSON) return;
        if (!saveLoaded || charIndex === undefined) return;
        if (applyingV2FromImport) return;
        setOverridesSource({
            kind: 'import',
            canonicalJSON: importedPreview.canonicalJSON,
            sourceLabel: importedPreview.path
                ? `Imported YAML — ${importedPreview.path}`
                : 'Imported YAML',
            path: importedPreview.path,
        });
    }, [importedPreview, saveLoaded, charIndex, applyingV2FromImport]);

    const handleOpenOverridesFromLibrary = useCallback(
        async (entry: templates.LibraryTemplateEntry) => {
            if (!saveLoaded || charIndex === undefined) {
                toast.error('Templates: Load a save and select a character before applying a v2 template.');
                return;
            }
            if (openingOverridesFromLibrary) return;
            setOpeningOverridesFromLibrary(true);
            try {
                const preview = await PreviewBuildTemplateFromLibrary(entry.id);
                const canonical = preview?.json ?? '';
                if (!canonical) {
                    toast.error('Templates: Library entry has no canonical JSON to edit.');
                    return;
                }
                if (preview && !preview.report.ok) {
                    const firstErr = preview.report.errors?.[0]?.message ?? 'Library entry failed validation.';
                    toast.error(`Templates: ${firstErr}`);
                    return;
                }
                setOverridesSource({
                    kind: 'library',
                    canonicalJSON: canonical,
                    sourceLabel: `Library — ${entry.name || entry.id}`,
                    entryID: entry.id,
                });
            } catch (err) {
                toast.error(`Templates: ${String(err)}`);
            } finally {
                setOpeningOverridesFromLibrary(false);
            }
        },
        [saveLoaded, charIndex, openingOverridesFromLibrary],
    );

    const handleConfirmOverrides = useCallback(
        async (mutatedJSON: string) => {
            if (!overridesSource) return;
            if (!saveLoaded || charIndex === undefined) return;
            if (applyingV2WithOverrides) return;
            setApplyingV2WithOverrides(true);
            try {
                const result = await ApplyBuildTemplateV2ToCharacterJSON(
                    charIndex,
                    mutatedJSON,
                    main.ApplyTemplateV2Options.createFrom({ mode: 'append' }),
                );
                if (!result.applied) {
                    const firstErr = result.preview?.errors?.[0]?.message ?? 'Apply did not complete.';
                    toast.error(`Templates: ${firstErr}`);
                    return;
                }
                toast.success(
                    `Applied ${overridesSource.sourceLabel} with overrides to character slot ${charIndex + 1}.`,
                );
                if ((result.skippedFields ?? []).includes('profile.class')) {
                    toast('Class was skipped in this phase.');
                }
                onCharacterTemplateApplied?.(charIndex);
                if (overridesSource.kind === 'import') {
                    setImportedPreview(null);
                }
                setOverridesSource(null);
            } catch (err) {
                toast.error(`Templates: ${String(err)}`);
            } finally {
                setApplyingV2WithOverrides(false);
            }
        },
        [overridesSource, saveLoaded, charIndex, applyingV2WithOverrides, onCharacterTemplateApplied],
    );

    const handleCancelOverrides = useCallback(() => {
        if (applyingV2WithOverrides) return;
        setOverridesSource(null);
    }, [applyingV2WithOverrides]);

    const handleApplyV2FromImportedPreview = useCallback(async () => {
        // Defensive parent-side guards. The modal already disables the
        // button under the same conditions, but we never want to invoke
        // the v2 apply binding without a loaded save + selected character
        // + non-empty canonical JSON from the preview.
        if (!importedPreview) return;
        if (!importedPreview.report.ok) return;
        if (!importedPreview.canonicalJSON) return;
        if (!saveLoaded || charIndex === undefined) return;
        if (applyingV2FromImport) return;
        setApplyingV2FromImport(true);
        try {
            const result = await ApplyBuildTemplateV2ToCharacterJSON(
                charIndex,
                importedPreview.canonicalJSON,
                main.ApplyTemplateV2Options.createFrom({ mode: 'append' }),
            );
            if (!result.applied) {
                const firstErr = result.preview?.errors?.[0]?.message ?? 'Apply did not complete.';
                toast.error(`Templates: ${firstErr}`);
                return;
            }
            const label = importedPreview.path
                ? `imported template (${importedPreview.path})`
                : 'imported template';
            toast.success(`Applied ${label} to character slot ${charIndex + 1}.`);
            if ((result.skippedFields ?? []).includes('profile.class')) {
                toast('Class was skipped in this phase.');
            }
            onCharacterTemplateApplied?.(charIndex);
            setImportedPreview(null);
        } catch (err) {
            toast.error(`Templates: ${String(err)}`);
        } finally {
            setApplyingV2FromImport(false);
        }
    }, [importedPreview, saveLoaded, charIndex, applyingV2FromImport, onCharacterTemplateApplied]);

    return (
        <>
            <TemplateLibraryModal
                sessionID=""
                allowApply
                title="Templates"
                onClose={onClose}
                onApplied={onLibraryApplied}
                onError={onLibraryError}
                onPreviewed={onLibraryPreviewed}
                onExportedToFile={onLibraryExportedToFile}
                onExportAsYAML={onExportAsYAML}
                reloadSignal={libraryReloadSignal}
                onDeleted={onLibraryDeleted}
                onRefreshed={onLibraryRefreshed}
                charIndex={charIndex}
                saveLoaded={saveLoaded}
                onApplyV2={handleApplyV2FromLibrary}
                onApplyV2WithOverrides={handleOpenOverridesFromLibrary}
                headerExtras={
                    <>
                        <button
                            type="button"
                            data-testid="templates-shell-create-v2"
                            onClick={() => setCreateTemplateOpen(true)}
                            disabled={createDisabled}
                            title={createTitle}
                            aria-label={createTitle}
                            className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40"
                        >
                            Create from Character…
                        </button>
                        <button
                            type="button"
                            data-testid="templates-shell-import-yaml"
                            onClick={onImportYAML}
                            disabled={importing}
                            title="Read a public Build Template YAML file and preview before saving to your library."
                            className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40"
                        >
                            {importing ? 'Opening…' : 'Import YAML from File…'}
                        </button>
                    </>
                }
            />
            {libraryPreview && (
                <ImportTemplatePreviewModal
                    report={libraryPreview}
                    onClose={() => setLibraryPreview(null)}
                />
            )}
            {importedPreview && (
                <ImportTemplatePreviewModal
                    report={importedPreview.report}
                    onClose={() => setImportedPreview(null)}
                    onSaveToLibrary={onSaveImportedToLibrary}
                    savingToLibrary={savingToLibrary}
                    onApplyV2={handleApplyV2FromImportedPreview}
                    applyingV2={applyingV2FromImport}
                    charIndex={charIndex}
                    saveLoaded={saveLoaded}
                    onApplyV2WithOverrides={handleOpenOverridesFromImport}
                />
            )}
            {overridesSource && (
                <ApplyOverridesModal
                    sourceLabel={overridesSource.sourceLabel}
                    canonicalJSON={overridesSource.canonicalJSON}
                    onCancel={handleCancelOverrides}
                    onConfirm={handleConfirmOverrides}
                    applying={applyingV2WithOverrides}
                />
            )}
            {createTemplateOpen && saveLoaded && charIndex !== undefined && (
                <CreateTemplateV2Modal
                    charIndex={charIndex}
                    onClose={() => setCreateTemplateOpen(false)}
                    onSaved={onCreateV2Saved}
                    onError={onCreateV2Error}
                />
            )}
        </>
    );
}
