import { useCallback, useState } from 'react';
import toast from '../../lib/toast';
import {
    ApplyBuildTemplateV2FromLibraryToCharacter,
    ApplyBuildTemplateV2ToCharacterJSON,
    ExportLibraryBuildTemplateAsYAMLToFile,
    GetActiveInventoryEditSessionForCharacter,
    PreviewBuildTemplateFromLibrary,
    PreviewBuildTemplateImportYAMLFromFile,
    PreviewBuildTemplateImportYAMLFromURL,
    SaveImportedBuildTemplateJSONToLibrary,
} from '../../../wailsjs/go/main/App';
import { main, templates } from '../../../wailsjs/go/models';
import { TemplateLibraryModal } from './TemplateLibraryModal';
import { ImportTemplatePreviewModal, isCancelledPreview } from './ImportTemplatePreviewModal';
import { CreateTemplateV2Modal } from './CreateTemplateV2Modal';
import { ApplyOverridesModal } from './ApplyOverridesPanel';
import { WeaponOverridePayload } from './WeaponLevelOverridePanel';
import { ImportTemplateFromURLModal } from './ImportTemplateFromURLModal';

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

// INVENTORY_WORKSPACE_SECTION is the canonical selection key used by
// the backend schema. Kept as a constant so the two call sites that
// peek into a template's selectedSections list agree on the spelling.
const INVENTORY_WORKSPACE_SECTION = 'inventory.workspace';
// ITEMS_SECTION — Phase 8D.2 canonical selection key. Items share the
// active-session gate with inventory.workspace; both also surface the
// runtime weapon level override in the overrides modal.
const ITEMS_SECTION = 'items';
const ITEM_APPLY_MODE_ADD_MISSING = 'addMissing';

// canonicalJSONHasItems returns true when the canonical JSON's
// selection nominates sections.items. Used to gate items-specific
// rewrites (e.g. explicit addMissing injection) and the
// items-apply session check on the JSON apply path.
function canonicalJSONHasItems(canonical: string): boolean {
    if (!canonical) return false;
    try {
        const parsed = JSON.parse(canonical) as {
            selection?: { items?: unknown };
        };
        const sel = parsed?.selection?.[ITEMS_SECTION];
        if (sel === undefined || sel === null) return false;
        if (typeof sel === 'boolean') return sel;
        if (typeof sel === 'object') {
            const obj = sel as Record<string, unknown>;
            if (obj.all === true) return true;
            return Object.keys(obj).length > 0;
        }
        return false;
    } catch {
        return false;
    }
}

// injectExplicitAddMissing rewrites the canonical JSON so its
// applyOptions.items.mode is set to "addMissing" when sections.items
// is selected. Phase 8D.1 backend already defaults to addMissing when
// ApplyOptions.Items is nil, but Phase 8D.2 UI sends the mode
// explicitly so the intent is testable from the JSON payload itself.
// Pass-through on any parse error so the caller never blocks on a
// rewrite failure — the backend will surface the underlying issue.
function injectExplicitAddMissing(canonical: string): string {
    if (!canonical) return canonical;
    if (!canonicalJSONHasItems(canonical)) return canonical;
    try {
        const parsed = JSON.parse(canonical) as Record<string, unknown>;
        const ao = (parsed.applyOptions as Record<string, unknown> | undefined) ?? {};
        const items = (ao.items as Record<string, unknown> | undefined) ?? {};
        if (items.mode === ITEM_APPLY_MODE_ADD_MISSING) {
            return canonical;
        }
        items.mode = ITEM_APPLY_MODE_ADD_MISSING;
        ao.items = items;
        parsed.applyOptions = ao;
        return JSON.stringify(parsed);
    } catch {
        return canonical;
    }
}

// NO_SESSION_MESSAGE is the shared toast/inline copy surfaced when the
// user tries to apply a v2 inventory.workspace template without an
// active Inventory Edit Session. The string mirrors the backend's
// IssueCodeInventorySessionRequired message so frontend + backend stay
// in lock-step.
const NO_SESSION_MESSAGE =
    'Open the Sort Order workspace before applying inventory templates.';

// fetchActiveSessionID resolves the current session ID for the given
// character via the read-only Wails endpoint. Returns undefined when
// there is no active session — the caller decides whether that is a
// hard error (inventory.workspace template) or a no-op (profile/stats
// template that doesn't need a session).
async function fetchActiveSessionID(charIndex: number): Promise<string | undefined> {
    try {
        const res = await GetActiveInventoryEditSessionForCharacter(charIndex);
        if (res && res.active && res.sessionID) {
            return res.sessionID;
        }
    } catch {
        // The endpoint is best-effort. Treat a backend error the same
        // as "no active session"; the apply path will refuse loudly if
        // the template requires one.
    }
    return undefined;
}

// canonicalJSONNeedsSession returns true when the canonical JSON
// payload's selection nominates inventory.workspace. Used by the
// overrides confirm handler, which works on a mutated JSON blob rather
// than a structured report.
function canonicalJSONNeedsSession(canonical: string): boolean {
    if (!canonical) return false;
    try {
        const parsed = JSON.parse(canonical) as {
            selection?: { 'inventory.workspace'?: unknown };
        };
        const sel = parsed?.selection?.[INVENTORY_WORKSPACE_SECTION];
        if (sel === undefined || sel === null) return false;
        if (typeof sel === 'boolean') return sel;
        if (typeof sel === 'object') {
            for (const v of Object.values(sel as Record<string, unknown>)) {
                if (v === true) return true;
            }
        }
        return false;
    } catch {
        return false;
    }
}

export function TemplatesShellModal({ onClose, charIndex, saveLoaded, onCharacterTemplateApplied }: Props) {
    const [libraryPreview, setLibraryPreview] = useState<templates.ImportPreviewReport | null>(null);
    const [importedPreview, setImportedPreview] = useState<ImportedYAMLPreview | null>(null);
    const [importing, setImporting] = useState(false);
    const [savingToLibrary, setSavingToLibrary] = useState(false);
    const [applyingV2FromImport, setApplyingV2FromImport] = useState(false);
    const [createTemplateOpen, setCreateTemplateOpen] = useState(false);
    const [urlImportOpen, setURLImportOpen] = useState(false);
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
    // Phase 8D.2 — items apply result modal state. Opened after a v2
    // apply touches sections.items so the user can see counts, the
    // skipped-already-present list, layout-ignored / weapon-override
    // warnings, and any other backend issue codes. Profile/stats-only
    // applies skip the modal and keep the existing toast UX.
    const [itemsApplyResult, setItemsApplyResult] = useState<
        | {
              sourceLabel: string;
              charIndex: number;
              result: main.ApplyTemplateV2Result;
          }
        | null
    >(null);

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

    // Phase 9 — URL import. The modal owns input + in-flight state +
    // inline error rendering; the shell owns the Wails call and the
    // downstream importedPreview state. On success we close the URL
    // modal and let the existing ImportTemplatePreviewModal take over,
    // identically to the file-import path. On guard rejection the
    // backend returns a LoadedTemplatePreview with Report.OK=false and
    // a single error — we surface it back to the URL modal so the user
    // can fix and retry without losing their URL.
    const onURLImportPreview = useCallback(
        async (rawURL: string): Promise<{ ok: true } | { ok: false; error: string }> => {
            try {
                const bundle = await PreviewBuildTemplateImportYAMLFromURL(rawURL);
                if (!bundle) {
                    return { ok: false, error: 'Empty response from backend.' };
                }
                const reportOK = bundle.report?.ok === true;
                if (!reportOK) {
                    const firstErr = bundle.report?.errors?.[0]?.message ?? 'URL import was rejected.';
                    return { ok: false, error: firstErr };
                }
                setImportedPreview({
                    report: bundle.report,
                    canonicalJSON: bundle.json ?? '',
                    path: bundle.path ?? rawURL,
                });
                setURLImportOpen(false);
                return { ok: true };
            } catch (err) {
                return { ok: false, error: String(err) };
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
            // Phase 7a — inventory.workspace templates require an
            // active Inventory Edit Session for the same charIndex. We
            // look it up before sending the apply so the user sees the
            // "Open the Sort Order workspace first" guidance instead
            // of a generic backend error. Profile/stats-only entries
            // do not need a session; we still forward the ID if one
            // happens to be open so the user can apply mixed templates
            // in the same pass later without re-opening the workspace.
            const selectedSections = entry.selectedSections ?? [];
            // Phase 8D.2 — items joins inventory.workspace under the
            // active-session gate. Library entries that nominate items
            // (with or without layout) need a session for the same
            // reason: both mutate sess.Workspace.
            const hasItems = selectedSections.includes(ITEMS_SECTION);
            const needsSession =
                selectedSections.includes(INVENTORY_WORKSPACE_SECTION) || hasItems;
            const sessionID = await fetchActiveSessionID(charIndex);
            if (needsSession && !sessionID) {
                toast.error(`Templates: ${NO_SESSION_MESSAGE}`);
                throw new Error(NO_SESSION_MESSAGE);
            }
            let result: main.ApplyTemplateV2Result;
            try {
                result = await ApplyBuildTemplateV2FromLibraryToCharacter(
                    charIndex,
                    entry.id,
                    main.ApplyTemplateV2Options.createFrom({
                        mode: 'append',
                        sessionID: sessionID ?? '',
                    }),
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
            const sourceLabel = entry.name || entry.id;
            toast.success(`Applied "${sourceLabel}" to character slot ${charIndex + 1}.`);
            if ((result.skippedFields ?? []).includes('profile.class')) {
                // toast() is the info channel in this codebase — see lib/toast.ts.
                toast('Class was skipped in this phase.');
            }
            onCharacterTemplateApplied?.(charIndex);
            if (hasItems) {
                setItemsApplyResult({ sourceLabel, charIndex, result });
            }
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
        async (mutatedJSON: string, weaponOverride?: WeaponOverridePayload) => {
            if (!overridesSource) return;
            if (!saveLoaded || charIndex === undefined) return;
            if (applyingV2WithOverrides) return;
            // Phase 7a — Apply with overrides may forward a template
            // that carries inventory.workspace alongside the edited
            // profile/stats. Inspect the mutated JSON itself (it is
            // the source of truth at confirm time) and gate on session
            // availability before invoking the apply binding.
            //
            // Phase 8D.2 — sections.items rides the same gate: both
            // inventory.workspace and items mutate sess.Workspace and
            // need an active edit session.
            const hasItems = canonicalJSONHasItems(mutatedJSON);
            const needsSession = canonicalJSONNeedsSession(mutatedJSON) || hasItems;
            const sessionID = await fetchActiveSessionID(charIndex);
            if (needsSession && !sessionID) {
                toast.error(`Templates: ${NO_SESSION_MESSAGE}`);
                return;
            }
            // Phase 8D.2 — surface the explicit addMissing mode on the
            // wire when sections.items is part of this apply. No-op
            // otherwise.
            const payloadJSON = hasItems ? injectExplicitAddMissing(mutatedJSON) : mutatedJSON;
            setApplyingV2WithOverrides(true);
            try {
                // Phase 7a.2 — runtime weapon level override travels as
                // an ApplyTemplateV2Options field, not inside the
                // canonical JSON. Profile/stats-only templates skip the
                // weapon panel entirely and weaponOverride is undefined;
                // the createFrom call passes through omitted fields
                // unchanged.
                const result = await ApplyBuildTemplateV2ToCharacterJSON(
                    charIndex,
                    payloadJSON,
                    main.ApplyTemplateV2Options.createFrom({
                        mode: 'append',
                        sessionID: sessionID ?? '',
                        weaponLevelOverride: weaponOverride,
                    }),
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
                const sourceLabel = overridesSource.sourceLabel;
                setOverridesSource(null);
                if (hasItems) {
                    setItemsApplyResult({ sourceLabel, charIndex, result });
                }
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
        // Phase 7a — inventory.workspace templates need an active
        // session. The preview summary lists the selected sections;
        // when it includes inventory.workspace and no session is open,
        // refuse before the apply binding is invoked.
        //
        // Phase 8D.2 — sections.items shares the same active-session
        // gate. Both apply paths mutate sess.Workspace; both surface
        // the same "open the Sort Order workspace first" guidance.
        const selectedSections = importedPreview.report.summary?.selectedSections ?? [];
        const needsSession =
            selectedSections.includes(INVENTORY_WORKSPACE_SECTION) ||
            selectedSections.includes(ITEMS_SECTION);
        const sessionID = await fetchActiveSessionID(charIndex);
        if (needsSession && !sessionID) {
            toast.error(`Templates: ${NO_SESSION_MESSAGE}`);
            return;
        }
        const hasItems = selectedSections.includes(ITEMS_SECTION);
        // Phase 8D.2 — make the addMissing intent explicit on the
        // wire when sections.items is present. injectExplicitAddMissing
        // is a no-op when items is not selected or the mode is
        // already addMissing.
        const payloadJSON = hasItems
            ? injectExplicitAddMissing(importedPreview.canonicalJSON)
            : importedPreview.canonicalJSON;
        setApplyingV2FromImport(true);
        try {
            const result = await ApplyBuildTemplateV2ToCharacterJSON(
                charIndex,
                payloadJSON,
                main.ApplyTemplateV2Options.createFrom({
                    mode: 'append',
                    sessionID: sessionID ?? '',
                }),
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
            // Phase 8D.2 — surface per-items detail when sections.items
            // was part of this apply. Profile/stats-only applies keep
            // the existing toast UX without an extra modal.
            if (hasItems) {
                setItemsApplyResult({ sourceLabel: label, charIndex, result });
            }
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
                        <button
                            type="button"
                            data-testid="templates-shell-import-url"
                            onClick={() => setURLImportOpen(true)}
                            title="Fetch a public Build Template YAML from an https:// URL under strict SSRF guards."
                            className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                        >
                            Import from URL…
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
            {urlImportOpen && (
                <ImportTemplateFromURLModal
                    onPreview={onURLImportPreview}
                    onCancel={() => setURLImportOpen(false)}
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
            {itemsApplyResult && (
                <ApplyItemsResultModal
                    sourceLabel={itemsApplyResult.sourceLabel}
                    charIndex={itemsApplyResult.charIndex}
                    result={itemsApplyResult.result}
                    onClose={() => setItemsApplyResult(null)}
                />
            )}
        </>
    );
}

// ApplyItemsResultModal — Phase 8D.2. Surfaces the per-items detail
// of an ApplyTemplateV2Result after sections.items was part of the
// apply: applied inventory / storage counts, the canonical
// items_already_present skip list, layout-ignored warnings, weapon
// override clamps, and any other backend issue codes that flowed
// through report.warnings or report.skips. Profile/stats-only applies
// never open this modal — those keep the existing toast UX.
interface ApplyItemsResultModalProps {
    sourceLabel: string;
    charIndex: number;
    result: main.ApplyTemplateV2Result;
    onClose: () => void;
}

function ApplyItemsResultModal({
    sourceLabel,
    charIndex,
    result,
    onClose,
}: ApplyItemsResultModalProps) {
    const warnings = result.preview?.warnings ?? [];
    const skippedFields = result.skippedFields ?? [];
    const layoutIgnored = warnings.filter(
        w => w.code === 'items_layout_ignored',
    );
    const alreadyPresent = warnings.filter(
        w => w.code === 'items_already_present',
    );
    const unsupportedCategory = warnings.filter(
        w => w.code === 'unsupported_category',
    );
    const weaponClamped = warnings.filter(
        w => w.code === 'weapon_level_clamped' || w.code === 'weapon_unupgradeable',
    );
    const templateOverrideIgnored = warnings.filter(
        w => w.code === 'items_template_override_ignored',
    );
    const otherWarnings = warnings.filter(
        w =>
            w.code !== 'items_layout_ignored' &&
            w.code !== 'items_already_present' &&
            w.code !== 'unsupported_category' &&
            w.code !== 'weapon_level_clamped' &&
            w.code !== 'weapon_unupgradeable' &&
            w.code !== 'items_template_override_ignored',
    );
    return (
        <div
            data-testid="items-apply-result-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Items apply result"
            className="fixed inset-0 z-[70] flex items-center justify-center bg-black/60 p-4"
        >
            <div className="w-full max-w-xl rounded-lg bg-card border border-border/60 shadow-xl flex flex-col max-h-[80vh]">
                <div className="px-4 py-3 border-b border-border/60">
                    <h2 className="text-sm font-black uppercase tracking-wider">
                        Items apply result
                    </h2>
                    <p
                        data-testid="items-apply-result-source"
                        className="mt-1 text-[11px] text-muted-foreground break-all"
                    >
                        {sourceLabel} — character slot {charIndex + 1}
                    </p>
                </div>
                <div className="px-4 py-3 space-y-3 overflow-y-auto text-[12px]">
                    <section aria-label="Applied counts" className="space-y-0.5">
                        <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            Added
                        </h3>
                        <div data-testid="items-apply-result-inv-added">
                            Inventory:{' '}
                            <span className="font-bold">
                                {result.inventoryItemsApplied}
                            </span>
                        </div>
                        <div data-testid="items-apply-result-sto-added">
                            Storage:{' '}
                            <span className="font-bold">{result.storageItemsApplied}</span>
                        </div>
                    </section>
                    {alreadyPresent.length > 0 && (
                        <section
                            data-testid="items-apply-result-already-present"
                            aria-label="Skipped — already present"
                            className="space-y-1"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                                Skipped — already present ({alreadyPresent.length})
                            </h3>
                            <ul className="space-y-0.5 list-disc pl-5">
                                {alreadyPresent.map((w, i) => (
                                    <li key={i} className="text-muted-foreground">
                                        {w.message}
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}
                    {unsupportedCategory.length > 0 && (
                        <section
                            data-testid="items-apply-result-unsupported-category"
                            aria-label="Skipped — unsupported category"
                            className="space-y-1"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-amber-200">
                                Skipped — unsupported category ({unsupportedCategory.length})
                            </h3>
                            <ul className="space-y-0.5 list-disc pl-5">
                                {unsupportedCategory.map((w, i) => (
                                    <li key={i} className="text-amber-100">
                                        {w.message}
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}
                    {layoutIgnored.length > 0 && (
                        <section
                            data-testid="items-apply-result-layout-ignored"
                            aria-label="Layout ignored"
                            className="space-y-1"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-amber-200">
                                Layout ignored ({layoutIgnored.length})
                            </h3>
                            <ul className="space-y-0.5 list-disc pl-5">
                                {layoutIgnored.map((w, i) => (
                                    <li key={i} className="text-amber-100">
                                        {w.message}
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}
                    {weaponClamped.length > 0 && (
                        <section
                            data-testid="items-apply-result-weapon-warnings"
                            aria-label="Weapon level override warnings"
                            className="space-y-1"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-amber-200">
                                Weapon level overrides ({weaponClamped.length})
                            </h3>
                            <ul className="space-y-0.5 list-disc pl-5">
                                {weaponClamped.map((w, i) => (
                                    <li key={i} className="text-amber-100">
                                        {w.message}
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}
                    {templateOverrideIgnored.length > 0 && (
                        <section
                            data-testid="items-apply-result-template-override-ignored"
                            aria-label="Template weapon override ignored"
                            className="space-y-1"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                                Template weapon override ignored
                            </h3>
                            <ul className="space-y-0.5 list-disc pl-5">
                                {templateOverrideIgnored.map((w, i) => (
                                    <li key={i} className="text-muted-foreground">
                                        {w.message}
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}
                    {otherWarnings.length > 0 && (
                        <section
                            data-testid="items-apply-result-other-warnings"
                            aria-label="Other warnings"
                            className="space-y-1"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-amber-200">
                                Other warnings ({otherWarnings.length})
                            </h3>
                            <ul className="space-y-0.5 list-disc pl-5">
                                {otherWarnings.map((w, i) => (
                                    <li key={i} className="text-amber-100">
                                        <span className="font-mono text-[10px] text-muted-foreground">
                                            [{w.code}]
                                        </span>{' '}
                                        {w.message}
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}
                    {skippedFields.length > 0 && (
                        <section
                            data-testid="items-apply-result-skipped-fields"
                            aria-label="Skipped fields"
                            className="space-y-1"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                                Skipped fields ({skippedFields.length})
                            </h3>
                            <ul className="space-y-0.5 list-disc pl-5">
                                {skippedFields.map((f, i) => (
                                    <li key={i} className="text-muted-foreground font-mono text-[10px]">
                                        {f}
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}
                </div>
                <div className="px-4 py-3 border-t border-border/60 flex items-center justify-end gap-2">
                    <button
                        type="button"
                        data-testid="items-apply-result-close"
                        onClick={onClose}
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                    >
                        Close
                    </button>
                </div>
            </div>
        </div>
    );
}
