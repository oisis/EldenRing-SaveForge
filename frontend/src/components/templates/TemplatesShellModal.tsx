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
// Phase 8E.2 — layout sections share the active-session gate with items
// and inventory.workspace. The backend Phase 8E.1 writer mutates
// sess.Workspace ordering, so an active edit session is mandatory even
// for layout-only templates.
const INVENTORY_LAYOUT_SECTION = 'inventoryLayout';
const STORAGE_LAYOUT_SECTION = 'storageLayout';
const ITEM_APPLY_MODE_ADD_MISSING = 'addMissing';
const LAYOUT_APPLY_MODE_REORDER_ONLY = 'reorderOnly';

// canonicalJSONHasSelection returns true when the canonical JSON's
// selection nominates the given section key. Truthy when the value is
// `true`, an object with `all: true`, or any object with at least one
// concrete child entry. Used to gate per-section rewrites (e.g. explicit
// mode injection) and the per-section session check on the JSON apply
// path.
function canonicalJSONHasSelection(canonical: string, sectionKey: string): boolean {
    if (!canonical) return false;
    try {
        const parsed = JSON.parse(canonical) as { selection?: Record<string, unknown> };
        const sel = parsed?.selection?.[sectionKey];
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

function canonicalJSONHasItems(canonical: string): boolean {
    return canonicalJSONHasSelection(canonical, ITEMS_SECTION);
}

function canonicalJSONHasInventoryLayout(canonical: string): boolean {
    return canonicalJSONHasSelection(canonical, INVENTORY_LAYOUT_SECTION);
}

function canonicalJSONHasStorageLayout(canonical: string): boolean {
    return canonicalJSONHasSelection(canonical, STORAGE_LAYOUT_SECTION);
}

function canonicalJSONHasAnyLayout(canonical: string): boolean {
    return (
        canonicalJSONHasInventoryLayout(canonical) ||
        canonicalJSONHasStorageLayout(canonical)
    );
}

// injectExplicitApplyDefaults rewrites the canonical JSON so its
// applyOptions block carries the UI's explicit defaults on the wire:
//   - items → mode = "addMissing"
//   - inventoryLayout → mode = "reorderOnly"
//   - storageLayout → mode = "reorderOnly"
// Phase 8D.1 / 8E.1 backends both default these modes when the option
// is nil, but the UI sends them explicitly so the intent is testable
// from the JSON payload itself. Pass-through on any parse error so the
// caller never blocks on a rewrite failure — the backend will surface
// the underlying issue.
function injectExplicitApplyDefaults(canonical: string): string {
    if (!canonical) return canonical;
    const wantItems = canonicalJSONHasItems(canonical);
    const wantInvLayout = canonicalJSONHasInventoryLayout(canonical);
    const wantStoLayout = canonicalJSONHasStorageLayout(canonical);
    if (!wantItems && !wantInvLayout && !wantStoLayout) return canonical;
    try {
        const parsed = JSON.parse(canonical) as Record<string, unknown>;
        const ao = (parsed.applyOptions as Record<string, unknown> | undefined) ?? {};
        let mutated = false;
        if (wantItems) {
            const items = (ao.items as Record<string, unknown> | undefined) ?? {};
            if (items.mode !== ITEM_APPLY_MODE_ADD_MISSING) {
                items.mode = ITEM_APPLY_MODE_ADD_MISSING;
                ao.items = items;
                mutated = true;
            }
        }
        if (wantInvLayout) {
            const il = (ao.inventoryLayout as Record<string, unknown> | undefined) ?? {};
            if (il.mode !== LAYOUT_APPLY_MODE_REORDER_ONLY) {
                il.mode = LAYOUT_APPLY_MODE_REORDER_ONLY;
                ao.inventoryLayout = il;
                mutated = true;
            }
        }
        if (wantStoLayout) {
            const sl = (ao.storageLayout as Record<string, unknown> | undefined) ?? {};
            if (sl.mode !== LAYOUT_APPLY_MODE_REORDER_ONLY) {
                sl.mode = LAYOUT_APPLY_MODE_REORDER_ONLY;
                ao.storageLayout = sl;
                mutated = true;
            }
        }
        if (!mutated) return canonical;
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
            //
            // Phase 8E.2 — layout sections (inventoryLayout / storageLayout)
            // share the same gate. The 8E.1 writer reorders sess.Workspace
            // so an active edit session is mandatory even for layout-only
            // entries.
            const hasItems = selectedSections.includes(ITEMS_SECTION);
            const hasLayout =
                selectedSections.includes(INVENTORY_LAYOUT_SECTION) ||
                selectedSections.includes(STORAGE_LAYOUT_SECTION);
            const needsSession =
                selectedSections.includes(INVENTORY_WORKSPACE_SECTION) || hasItems || hasLayout;
            const sessionID = await fetchActiveSessionID(charIndex);
            if (needsSession && !sessionID) {
                toast.error(`Templates: ${NO_SESSION_MESSAGE}`);
                throw new Error(NO_SESSION_MESSAGE);
            }
            // Phase 8D.3 / 8E.2 — for items- or layout-bearing library
            // entries we want applyOptions.items.mode and
            // applyOptions.{inventory,storage}Layout.mode to land on
            // the wire explicitly, mirroring the imported preview /
            // overrides apply paths.
            // ApplyBuildTemplateV2FromLibraryToCharacter accepts only
            // (charIdx, id, opts) and the entry's stored applyOptions
            // are read inside the backend with no per-call override
            // hook, so we route via PreviewBuildTemplateFromLibrary
            // (which returns canonical JSON) + the JSON apply binding.
            // Profile/stats/equipment/spells-only entries skip the
            // round-trip and stay on the FromLibrary path. The two
            // backends share applyBuildTemplateV2 internally —
            // switching paths does not change semantics.
            const needsJSONRoute = hasItems || hasLayout;
            let result: main.ApplyTemplateV2Result;
            try {
                if (needsJSONRoute) {
                    const preview = await PreviewBuildTemplateFromLibrary(entry.id);
                    if (!preview.report?.ok) {
                        const firstErr =
                            preview.report?.errors?.[0]?.message ?? 'Library preview rejected.';
                        toast.error(`Templates: ${firstErr}`);
                        throw new Error(firstErr);
                    }
                    const payloadJSON = injectExplicitApplyDefaults(preview.json ?? '');
                    result = await ApplyBuildTemplateV2ToCharacterJSON(
                        charIndex,
                        payloadJSON,
                        main.ApplyTemplateV2Options.createFrom({
                            mode: 'append',
                            sessionID: sessionID ?? '',
                        }),
                    );
                } else {
                    result = await ApplyBuildTemplateV2FromLibraryToCharacter(
                        charIndex,
                        entry.id,
                        main.ApplyTemplateV2Options.createFrom({
                            mode: 'append',
                            sessionID: sessionID ?? '',
                        }),
                    );
                }
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
            if (hasItems || hasLayout) {
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
            //
            // Phase 8E.2 — inventoryLayout / storageLayout also reorder
            // sess.Workspace and therefore share the active-session
            // requirement.
            const hasItems = canonicalJSONHasItems(mutatedJSON);
            const hasLayout = canonicalJSONHasAnyLayout(mutatedJSON);
            const needsSession =
                canonicalJSONNeedsSession(mutatedJSON) || hasItems || hasLayout;
            const sessionID = await fetchActiveSessionID(charIndex);
            if (needsSession && !sessionID) {
                toast.error(`Templates: ${NO_SESSION_MESSAGE}`);
                return;
            }
            // Phase 8D.2 / 8E.2 — surface the explicit addMissing and
            // layout reorderOnly modes on the wire when their respective
            // sections are part of this apply. injectExplicitApplyDefaults
            // is a no-op when none of items / inventoryLayout /
            // storageLayout are selected.
            const payloadJSON =
                hasItems || hasLayout
                    ? injectExplicitApplyDefaults(mutatedJSON)
                    : mutatedJSON;
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
                if (hasItems || hasLayout) {
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
        //
        // Phase 8E.2 — inventoryLayout / storageLayout reorder
        // sess.Workspace too; same gate, same guidance.
        const selectedSections = importedPreview.report.summary?.selectedSections ?? [];
        const hasItems = selectedSections.includes(ITEMS_SECTION);
        const hasLayout =
            selectedSections.includes(INVENTORY_LAYOUT_SECTION) ||
            selectedSections.includes(STORAGE_LAYOUT_SECTION);
        const needsSession =
            selectedSections.includes(INVENTORY_WORKSPACE_SECTION) || hasItems || hasLayout;
        const sessionID = await fetchActiveSessionID(charIndex);
        if (needsSession && !sessionID) {
            toast.error(`Templates: ${NO_SESSION_MESSAGE}`);
            return;
        }
        // Phase 8D.2 / 8E.2 — make addMissing (items) and reorderOnly
        // (inventoryLayout / storageLayout) intents explicit on the
        // wire. injectExplicitApplyDefaults is a no-op when none of
        // these sections are selected.
        const payloadJSON =
            hasItems || hasLayout
                ? injectExplicitApplyDefaults(importedPreview.canonicalJSON)
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
            // Phase 8D.2 / 8E.2 — surface per-items / per-layout
            // detail when sections.items or layout sections were part
            // of this apply. Profile/stats-only applies keep the
            // existing toast UX without an extra modal.
            if (hasItems || hasLayout) {
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

// ApplyItemsResultModal — Phase 8D.2 (8D.3 polish). Surfaces the
// per-items detail of an ApplyTemplateV2Result after sections.items
// was part of the apply: applied inventory / storage counts, the
// canonical items_already_present skip list, layout-ignored warnings,
// weapon override clamps, and any other backend issue codes that
// flowed through preview.warnings. Profile/stats-only applies never
// open this modal — those keep the existing toast UX.
//
// Phase 8D.4 follow-up: backend AppliedFields currently emits the
// literal "items" string rather than per-entry IDs. Once that lands,
// the Added section can list the actual entryIDs that materialised
// (today only counts come from inventoryItemsApplied /
// storageItemsApplied). Warnings already carry entryID prefixes in
// their `message` field — we render those as-is.
interface ApplyItemsResultModalProps {
    sourceLabel: string;
    charIndex: number;
    result: main.ApplyTemplateV2Result;
    onClose: () => void;
}

// WARNING_PREVIEW_LIMIT — Phase 8D.3 polish: cap each warning section
// at this many lines, with a "+ N more (total: X)" suffix when the
// real list is longer. Limit chosen so a 200-entry add-missing apply
// with a fully-occupied inventory still fits on a single 80-vh modal
// without scrolling jail.
const WARNING_PREVIEW_LIMIT = 5;

interface WarningGroupProps {
    testId: string;
    title: string;
    items: templates.ImportPreviewIssue[];
    tone: 'muted' | 'amber';
    showCodePrefix?: boolean;
}

function WarningGroup({
    testId,
    title,
    items,
    tone,
    showCodePrefix = false,
}: WarningGroupProps) {
    if (items.length === 0) return null;
    const visible = items.slice(0, WARNING_PREVIEW_LIMIT);
    const hiddenCount = Math.max(0, items.length - visible.length);
    const headerClass =
        tone === 'amber'
            ? 'text-[10px] font-bold uppercase tracking-wider text-warning-foreground'
            : 'text-[10px] font-bold uppercase tracking-wider text-muted-foreground';
    const itemClass = tone === 'amber' ? 'text-amber-100' : 'text-muted-foreground';
    return (
        <section
            data-testid={testId}
            aria-label={title}
            data-warning-severity="info"
            className="space-y-1"
        >
            <h3 className={headerClass}>
                {title} ({items.length})
            </h3>
            <ul className="space-y-0.5 list-disc pl-5">
                {visible.map((w, i) => (
                    <li key={i} className={itemClass}>
                        {showCodePrefix && (
                            <span className="font-mono text-[10px] text-muted-foreground">
                                [{w.code}]
                            </span>
                        )}
                        {showCodePrefix ? ' ' : ''}
                        {w.container ? (
                            <span className="font-mono text-[10px] text-muted-foreground">
                                ({w.container}){' '}
                            </span>
                        ) : null}
                        {w.message}
                    </li>
                ))}
                {hiddenCount > 0 && (
                    <li
                        data-testid={`${testId}-more`}
                        className="text-[10px] text-muted-foreground italic list-none -ml-5"
                    >
                        + {hiddenCount} more (total: {items.length})
                    </li>
                )}
            </ul>
        </section>
    );
}

function ApplyItemsResultModal({
    sourceLabel,
    charIndex,
    result,
    onClose,
}: ApplyItemsResultModalProps) {
    const warnings = result.preview?.warnings ?? [];
    const skippedFields = result.skippedFields ?? [];
    const grouped = new Map<string, templates.ImportPreviewIssue[]>();
    for (const w of warnings) {
        const arr = grouped.get(w.code) ?? [];
        arr.push(w);
        grouped.set(w.code, arr);
    }
    const layoutIgnored = grouped.get('items_layout_ignored') ?? [];
    const alreadyPresent = grouped.get('items_already_present') ?? [];
    const unsupportedCategory = grouped.get('unsupported_category') ?? [];
    const weaponClamped = [
        ...(grouped.get('weapon_level_clamped') ?? []),
        ...(grouped.get('weapon_unupgradeable') ?? []),
    ];
    const templateOverrideIgnored = grouped.get('items_template_override_ignored') ?? [];
    // Phase 8E.2 — layout warning groups. layout_entry_missing and
    // layout_entry_ambiguous are amber (user-actionable: template
    // referenced something the live workspace can't resolve).
    // layout_sparse_normalized and layout_extra_items_preserved are
    // informational ("nothing went wrong, but here's what the writer
    // did"). layout_mode_unsupported flags a skipped layout section
    // (append/replace/etc.) that the UI never sends — it can still
    // surface from hand-authored YAML.
    const layoutEntryMissing = grouped.get('layout_entry_missing') ?? [];
    const layoutEntryAmbiguous = grouped.get('layout_entry_ambiguous') ?? [];
    const layoutSparseNormalized = grouped.get('layout_sparse_normalized') ?? [];
    const layoutExtraItemsPreserved = grouped.get('layout_extra_items_preserved') ?? [];
    const layoutModeUnsupported = grouped.get('layout_mode_unsupported') ?? [];
    const KNOWN_CODES = new Set([
        'items_layout_ignored',
        'items_already_present',
        'unsupported_category',
        'weapon_level_clamped',
        'weapon_unupgradeable',
        'items_template_override_ignored',
        'layout_entry_missing',
        'layout_entry_ambiguous',
        'layout_sparse_normalized',
        'layout_extra_items_preserved',
        'layout_mode_unsupported',
    ]);
    const otherWarnings = warnings.filter(w => !KNOWN_CODES.has(w.code));
    // Phase 8E.2 — layout counters are non-zero only when the apply
    // touched inventoryLayout / storageLayout. Show the layout block
    // whenever any of the counters or warnings is non-zero so the
    // modal stays quiet for items-only applies.
    const layoutInventoryApplied = result.layoutInventoryEntriesApplied ?? 0;
    const layoutStorageApplied = result.layoutStorageEntriesApplied ?? 0;
    const layoutInventoryMissing = result.layoutInventoryEntriesMissing ?? 0;
    const layoutStorageMissing = result.layoutStorageEntriesMissing ?? 0;
    const layoutInventoryExtras = result.layoutInventoryExtrasPreserved ?? 0;
    const layoutStorageExtras = result.layoutStorageExtrasPreserved ?? 0;
    const showLayoutCounters =
        layoutInventoryApplied > 0 ||
        layoutStorageApplied > 0 ||
        layoutInventoryMissing > 0 ||
        layoutStorageMissing > 0 ||
        layoutInventoryExtras > 0 ||
        layoutStorageExtras > 0 ||
        layoutEntryMissing.length > 0 ||
        layoutEntryAmbiguous.length > 0 ||
        layoutSparseNormalized.length > 0 ||
        layoutExtraItemsPreserved.length > 0 ||
        layoutModeUnsupported.length > 0;
    return (
        <div
            data-testid="items-apply-result-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Template apply result"
            className="fixed inset-0 z-[70] flex items-center justify-center bg-black/60 p-4"
        >
            <div className="w-full max-w-xl rounded-lg bg-card border border-border/60 shadow-xl flex flex-col max-h-[80vh]">
                <div className="px-4 py-3 border-b border-border/60">
                    <h2 className="text-sm font-black uppercase tracking-wider">
                        Template apply result
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
                    {showLayoutCounters && (
                        <section
                            data-testid="items-apply-result-layout-counters"
                            aria-label="Layout reorder counts"
                            className="space-y-0.5"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                                Layout reordered
                            </h3>
                            <div data-testid="items-apply-result-layout-inv-applied">
                                Inventory entries applied:{' '}
                                <span className="font-bold">{layoutInventoryApplied}</span>
                            </div>
                            <div data-testid="items-apply-result-layout-sto-applied">
                                Storage entries applied:{' '}
                                <span className="font-bold">{layoutStorageApplied}</span>
                            </div>
                            <div data-testid="items-apply-result-layout-inv-missing">
                                Inventory entries missing (skipped):{' '}
                                <span className="font-bold">{layoutInventoryMissing}</span>
                            </div>
                            <div data-testid="items-apply-result-layout-sto-missing">
                                Storage entries missing (skipped):{' '}
                                <span className="font-bold">{layoutStorageMissing}</span>
                            </div>
                            <div data-testid="items-apply-result-layout-inv-extras">
                                Inventory extras preserved (appended):{' '}
                                <span className="font-bold">{layoutInventoryExtras}</span>
                            </div>
                            <div data-testid="items-apply-result-layout-sto-extras">
                                Storage extras preserved (appended):{' '}
                                <span className="font-bold">{layoutStorageExtras}</span>
                            </div>
                            <p
                                data-testid="items-apply-result-layout-note"
                                className="mt-1 text-[10px] text-muted-foreground"
                            >
                                Layout apply is reorder-only — no items are added, removed, or
                                replaced. Extras stay; missing entries are skipped with
                                warnings.
                            </p>
                        </section>
                    )}
                    <WarningGroup
                        testId="items-apply-result-already-present"
                        title="Skipped — already present"
                        items={alreadyPresent}
                        tone="muted"
                    />
                    <WarningGroup
                        testId="items-apply-result-unsupported-category"
                        title="Skipped — unsupported category"
                        items={unsupportedCategory}
                        tone="amber"
                    />
                    <WarningGroup
                        testId="items-apply-result-layout-ignored"
                        title="Layout ignored (items+layout interop)"
                        items={layoutIgnored}
                        tone="amber"
                    />
                    <WarningGroup
                        testId="items-apply-result-layout-missing"
                        title="Layout entries missing — skipped"
                        items={layoutEntryMissing}
                        tone="amber"
                    />
                    <WarningGroup
                        testId="items-apply-result-layout-ambiguous"
                        title="Layout entries ambiguous — first match used"
                        items={layoutEntryAmbiguous}
                        tone="amber"
                    />
                    <WarningGroup
                        testId="items-apply-result-layout-sparse"
                        title="Layout normalized (sparse positions)"
                        items={layoutSparseNormalized}
                        tone="muted"
                    />
                    <WarningGroup
                        testId="items-apply-result-layout-extras"
                        title="Extras preserved (appended)"
                        items={layoutExtraItemsPreserved}
                        tone="muted"
                    />
                    <WarningGroup
                        testId="items-apply-result-layout-mode-unsupported"
                        title="Layout mode unsupported — section skipped"
                        items={layoutModeUnsupported}
                        tone="amber"
                    />
                    <WarningGroup
                        testId="items-apply-result-weapon-warnings"
                        title="Weapon level overrides"
                        items={weaponClamped}
                        tone="amber"
                    />
                    <WarningGroup
                        testId="items-apply-result-template-override-ignored"
                        title="Template weapon override ignored"
                        items={templateOverrideIgnored}
                        tone="muted"
                    />
                    <WarningGroup
                        testId="items-apply-result-other-warnings"
                        title="Other warnings"
                        items={otherWarnings}
                        tone="amber"
                        showCodePrefix
                    />
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
