import { useCallback, useState } from 'react';
import toast from '../../lib/toast';
import {
    ExportLibraryBuildTemplateAsYAMLToFile,
    PreviewBuildTemplateImportYAMLFromFile,
    SaveImportedBuildTemplateJSONToLibrary,
} from '../../../wailsjs/go/main/App';
import { main, templates } from '../../../wailsjs/go/models';
import { TemplateLibraryModal } from './TemplateLibraryModal';
import { ImportTemplatePreviewModal, isCancelledPreview } from './ImportTemplatePreviewModal';
import { CreateTemplateV2Modal } from './CreateTemplateV2Modal';

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
    // action. The library-only flow remains usable without a loaded save,
    // so both are optional; only the create button reacts to them.
    charIndex?: number;
    saveLoaded?: boolean;
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

export function TemplatesShellModal({ onClose, charIndex, saveLoaded }: Props) {
    const [libraryPreview, setLibraryPreview] = useState<templates.ImportPreviewReport | null>(null);
    const [importedPreview, setImportedPreview] = useState<ImportedYAMLPreview | null>(null);
    const [importing, setImporting] = useState(false);
    const [savingToLibrary, setSavingToLibrary] = useState(false);
    const [createTemplateOpen, setCreateTemplateOpen] = useState(false);
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
        /* allowApply=false hides the Apply button; this never fires */
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

    return (
        <>
            <TemplateLibraryModal
                sessionID=""
                allowApply={false}
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
