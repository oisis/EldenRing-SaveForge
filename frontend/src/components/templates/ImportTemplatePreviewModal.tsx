import { useEffect, useRef } from 'react';
import { templates } from '../../../wailsjs/go/models';

// ImportTemplatePreviewModal renders the dry-run report produced by
// PreviewBuildTemplateImportFromFile / PreviewBuildTemplateImportJSON.
//
// Phase C scope: read-only display. The modal has no "Apply" button —
// import-to-workspace is Phase D/E. The wording on the panel ("Preview
// only — does not change your workspace or save.") is load-bearing for
// user trust and is checked by tests.

interface Props {
    report: templates.ImportPreviewReport;
    onClose: () => void;
    // onApply is optional. When provided AND report.ok is true, the
    // modal renders an "Apply to workspace" button that the caller can
    // wire to ApplyBuildTemplateToWorkspaceJSON. Phase D users pass it
    // in; Phase C callers (preview-only) leave it undefined and the
    // button stays hidden.
    onApply?: () => void;
    applying?: boolean;
    // onSaveToLibrary is optional and independent of onApply. When
    // provided, the modal renders a "Save to Library" button used by
    // the Phase 2B global Templates shell after a YAML import preview.
    // It writes the previewed template (carried by the caller as the
    // canonical JSON returned alongside the report) to the local
    // library; the modal itself owns no payload state. Disabled when
    // report.ok is false or savingToLibrary is true.
    onSaveToLibrary?: () => void;
    savingToLibrary?: boolean;
    // Phase 5D.2 — direct v2 apply from an imported preview. When
    // onApplyV2 is provided AND the report carries a schema v2 summary,
    // the modal renders an "Apply to character" button. The button is
    // visible only for v2 previews; v1 previews never expose it,
    // regardless of whether the caller passes onApplyV2. Enabled only
    // when all gating rules hold (report.ok, saveLoaded, charIndex,
    // non-empty selectedSections fully contained in the supported
    // profile/stats subset). On disabled states, the title attribute
    // surfaces the reason. The button itself owns no apply state — the
    // caller drives the JSON apply (ApplyBuildTemplateV2ToCharacterJSON)
    // and toasts/closes the modal on success.
    onApplyV2?: () => void;
    applyingV2?: boolean;
    charIndex?: number;
    saveLoaded?: boolean;
    // Phase 6 — v2 apply with editable overrides. When onApplyV2WithOverrides
    // is provided AND the same v2 gates hold, the modal renders a second
    // "Apply with overrides…" button next to "Apply to character". The
    // button uses the same gating logic as the plain v2 Apply — anything
    // that disables Apply also disables overrides. The caller owns the
    // overrides modal lifecycle; this button is only the trigger.
    onApplyV2WithOverrides?: () => void;
}

// V2_APPLY_SUPPORTED_SECTIONS — selectedSections values the backend
// v2 apply layer accepts. Phase 5 added profile/stats; Phase 7a added
// inventory.workspace behind an active-session check that the SHELL
// performs after the user clicks Apply (the modal does not know
// whether a session is currently open). The button stays enabled here
// for inventory.workspace templates and the shell either forwards the
// session ID or surfaces the "Open the Sort Order workspace first"
// toast.
//
// Phase 7b.1 adds equipment. Equipment-only templates do NOT require
// an active session — the writer reads slot.GaMap directly. The
// combination equipment + inventory.workspace is hard-rejected at the
// backend preview layer with the dedicated
// equipment_inventory_combo_unsupported code, so this modal sees a
// non-OK report and disables Apply automatically without any extra
// gating here.
//
// Phase 7d.4 adds spells. Spells-only templates do NOT require an
// active session — WriteSpells operates directly on slot.Data and
// recomputes hash[10] inline. Spells coexist freely with profile/
// stats/equipment/inventory.workspace; no combo restrictions apply.
//
// Phase 8D.2 adds items. sections.items applies as add-missing only
// and shares the active-session gate with inventory.workspace; the
// shell performs the session lookup.
//
// Phase 8E.2 promotes inventoryLayout / storageLayout from export-only
// to apply-eligible as reorder-only (the only mode the writer supports
// today). The backend defaults nil/empty layout mode to reorderOnly
// and the UI sends the mode explicitly via injectExplicitApplyDefaults.
// Layout apply also requires an active inventory edit session because
// the writer mutates sess.Workspace ordering.
const V2_APPLY_SUPPORTED_SECTIONS = [
    'profile',
    'stats',
    'inventory.workspace',
    'equipment',
    'spells',
    'items',
    'inventoryLayout',
    'storageLayout',
];
const LAYOUT_SECTIONS = ['inventoryLayout', 'storageLayout'];

export function ImportTemplatePreviewModal({
    report,
    onClose,
    onApply,
    applying,
    onSaveToLibrary,
    savingToLibrary,
    onApplyV2,
    applyingV2,
    charIndex,
    saveLoaded,
    onApplyV2WithOverrides,
}: Props) {
    const dialogRef = useRef<HTMLDivElement | null>(null);
    useEffect(() => {
        dialogRef.current?.focus();
    }, []);

    const errors = report.errors ?? [];
    const warnings = report.warnings ?? [];
    const summary = report.summary;

    // Schema v2 metadata surfaced from ImportPreviewSummary (Phase 3C.0
    // backend; Phase 3D.0 bindings). v1 reports leave Version=0 and the
    // string slices empty, so the v2 block stays hidden — keeping the
    // existing inventory-template UI visually quiet.
    const schemaVersion = summary?.version ?? 0;
    const selectedSections = summary?.selectedSections ?? [];
    const profileFieldsPresent = summary?.profileFieldsPresent ?? [];
    const statFieldsPresent = summary?.statFieldsPresent ?? [];
    const equipmentSlotsPresent = summary?.equipmentSlotsPresent ?? [];
    const spellSlotsPresent = summary?.spellSlotsPresent ?? [];
    const itemsEntries = summary?.itemsEntries ?? 0;
    const inventoryLayoutCount = summary?.inventoryLayoutCount ?? 0;
    const storageLayoutCount = summary?.storageLayoutCount ?? 0;
    const isV2 = schemaVersion >= 2;
    const showV2Meta =
        isV2 ||
        profileFieldsPresent.length > 0 ||
        statFieldsPresent.length > 0 ||
        equipmentSlotsPresent.length > 0 ||
        spellSlotsPresent.length > 0;
    const showItemsBlock =
        itemsEntries > 0 || inventoryLayoutCount > 0 || storageLayoutCount > 0;

    // Phase 5D.2 — direct v2 apply button visibility & gating. v1
    // previews never expose the v2 buttons; for v2 previews the same
    // gating reason is shared by the plain Apply (Phase 5D.2) and the
    // Apply with overrides (Phase 6) buttons.
    const v2ApplyVisible = !!onApplyV2 && isV2;
    const v2OverridesVisible = !!onApplyV2WithOverrides && isV2;
    const layoutSectionsPresent = selectedSections.filter(s =>
        LAYOUT_SECTIONS.includes(s),
    );
    const v2HasUnsupportedSection =
        selectedSections.length > 0 &&
        selectedSections.some(s => !V2_APPLY_SUPPORTED_SECTIONS.includes(s));
    const v2DisabledReason: string = !isV2
        ? ''
        : !report.ok
            ? 'Preview blocked — fix errors before applying.'
            : !saveLoaded
                ? 'Load a save before applying a v2 template.'
                : charIndex === undefined
                    ? 'Select a character before applying.'
                    : selectedSections.length === 0
                        ? 'No sections selected.'
                        : v2HasUnsupportedSection
                            ? 'Unsupported v2 sections — apply is available only for profile, stats, inventory.workspace, equipment, spells, items, inventoryLayout, and storageLayout in this phase.'
                            : '';
    // Phase 8D.2 — `items` section apply visibility helpers.
    const itemsSectionSelected = selectedSections.includes('items');
    const hasLayoutAlongsideItems =
        itemsSectionSelected && layoutSectionsPresent.length > 0;
    // Phase 8E.2 — layout-only template helper: layout is selected
    // without items. The preview block surfaces the reorder-only copy
    // (no items added or removed) so users understand what apply
    // actually does.
    const isLayoutOnly =
        !itemsSectionSelected && layoutSectionsPresent.length > 0;
    const v2ApplyDisabledReason = v2ApplyVisible ? v2DisabledReason : '';
    const v2ApplyEnabled =
        v2ApplyVisible && !applyingV2 && v2ApplyDisabledReason === '';
    const v2OverridesDisabledReason = v2OverridesVisible ? v2DisabledReason : '';
    const v2OverridesEnabled =
        v2OverridesVisible && !applyingV2 && v2OverridesDisabledReason === '';

    return (
        <div
            data-testid="import-preview-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Build Template Import Preview"
            ref={dialogRef}
            tabIndex={-1}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
        >
            <div className="w-full max-w-2xl rounded-lg bg-card border border-border/60 shadow-xl flex flex-col max-h-[80vh]">
                <div className="px-4 py-3 border-b border-border/60">
                    <h2 className="text-sm font-black uppercase tracking-wider">Build Template Import — Preview</h2>
                    <p
                        data-testid="import-preview-disclaimer"
                        className="mt-1 text-[11px] text-muted-foreground"
                    >
                        Preview only — this does not change your workspace or save.
                    </p>
                    {onApply && (
                        <p
                            data-testid="import-preview-apply-note"
                            className="mt-1 text-[11px] text-muted-foreground"
                        >
                            Apply will add the template to your current workspace. The save file is not touched until you click Save changes.
                        </p>
                    )}
                </div>

                <div className="px-4 py-3 space-y-3 overflow-y-auto text-[12px]">
                    {/* Schema v2 metadata. Rendered only when the report
                        actually carries v2-shaped data — v1 reports keep
                        their existing minimal layout untouched. */}
                    {showV2Meta && (
                        <section
                            data-testid="import-preview-v2-meta"
                            aria-label="Schema v2 metadata"
                            className="space-y-0.5"
                        >
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground mb-1">
                                Schema
                            </h3>
                            {isV2 && (
                                <div data-testid="import-preview-schema-version">
                                    Schema: <span className="font-bold">v{schemaVersion}</span>
                                </div>
                            )}
                            {isV2 && selectedSections.length > 0 && (
                                <div data-testid="import-preview-selected-sections">
                                    Sections: <span className="font-bold">{selectedSections.join(', ')}</span>
                                </div>
                            )}
                            {profileFieldsPresent.length > 0 && (
                                <div data-testid="import-preview-profile-fields">
                                    Profile fields: <span className="font-bold">{profileFieldsPresent.join(', ')}</span>
                                </div>
                            )}
                            {statFieldsPresent.length > 0 && (
                                <div data-testid="import-preview-stat-fields">
                                    Stats: <span className="font-bold">{statFieldsPresent.join(', ')}</span>
                                </div>
                            )}
                            {equipmentSlotsPresent.length > 0 && (
                                <div data-testid="import-preview-equipment-slots">
                                    Equipment slots: <span className="font-bold">{equipmentSlotsPresent.join(', ')}</span>
                                </div>
                            )}
                            {spellSlotsPresent.length > 0 && (
                                <div data-testid="import-preview-spell-slots">
                                    Spell slots: <span className="font-bold">{spellSlotsPresent.join(', ')}</span>
                                </div>
                            )}
                            {showItemsBlock && (
                                <>
                                    <div data-testid="import-preview-items-entries">
                                        Items entries: <span className="font-bold">{itemsEntries}</span>
                                    </div>
                                    <div data-testid="import-preview-inventory-layout-count">
                                        Inventory layout entries:{' '}
                                        <span className="font-bold">{inventoryLayoutCount}</span>
                                    </div>
                                    <div data-testid="import-preview-storage-layout-count">
                                        Storage layout entries:{' '}
                                        <span className="font-bold">{storageLayoutCount}</span>
                                    </div>
                                    {itemsSectionSelected && (
                                        <>
                                            <div
                                                data-testid="import-preview-items-apply-supported"
                                                className="text-[10px] text-emerald-300/90"
                                            >
                                                Items: apply supported (add missing only).
                                                {hasLayoutAlongsideItems
                                                    ? ' Missing items are added first, then layout is applied (reorder-only).'
                                                    : ''}
                                            </div>
                                            <div
                                                data-testid="import-preview-items-weapon-hint"
                                                className="text-[10px] text-muted-foreground italic"
                                            >
                                                Direct Apply uses template / default upgrade levels. Use “Apply with overrides…” to override standard (+0–25) or somber (+0–10) weapon levels for newly added items.
                                            </div>
                                        </>
                                    )}
                                    {isLayoutOnly && (
                                        <div
                                            data-testid="import-preview-layout-reorder-only"
                                            className="text-[10px] text-emerald-300/90 space-y-0.5"
                                        >
                                            <div>Layout apply is reorder-only.</div>
                                            <div className="text-muted-foreground">
                                                It reorders matching existing workspace items. It does
                                                not add, remove, or replace items.
                                            </div>
                                            <div className="text-muted-foreground">
                                                Extra items are preserved and appended after
                                                template-ordered items. Missing template entries are
                                                skipped with warnings.
                                            </div>
                                        </div>
                                    )}
                                </>
                            )}
                        </section>
                    )}

                    {/* Summary */}
                    <section data-testid="import-preview-summary" aria-label="Summary">
                        <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground mb-1">Summary</h3>
                        <ul className="grid grid-cols-2 gap-x-4 gap-y-1">
                            <li>Inventory items: <span className="font-bold">{summary?.inventoryItems ?? 0}</span></li>
                            <li>Storage items: <span className="font-bold">{summary?.storageItems ?? 0}</span></li>
                            <li>Weapons: <span className="font-bold">{summary?.weapons ?? 0}</span></li>
                            <li>Armor: <span className="font-bold">{summary?.armor ?? 0}</span></li>
                            <li>Talismans: <span className="font-bold">{summary?.talismans ?? 0}</span></li>
                            <li>Stackables: <span className="font-bold">{summary?.stackables ?? 0}</span></li>
                            <li>AoW assignments: <span className="font-bold">{summary?.aowAssignments ?? 0}</span></li>
                            <li>
                                Status:{' '}
                                <span className={report.ok ? 'text-green-300 font-bold' : 'text-red-300 font-bold'}>
                                    {report.ok ? 'OK' : 'Blocked'}
                                </span>
                            </li>
                        </ul>
                    </section>

                    {/* Errors */}
                    {errors.length > 0 && (
                        <section data-testid="import-preview-errors" aria-label="Errors">
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-red-300 mb-1">
                                Errors ({errors.length}) — must be fixed before import
                            </h3>
                            <ul className="space-y-1">
                                {errors.map((e, i) => (
                                    <li
                                        key={`err-${i}`}
                                        data-testid="import-preview-error"
                                        data-code={e.code}
                                        className="rounded border border-red-500/40 bg-red-500/10 px-2 py-1 text-red-200"
                                    >
                                        <div className="font-bold">{e.code}</div>
                                        <div>{e.message}</div>
                                        <PositionalTrailer issue={e} />
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}

                    {/* Warnings */}
                    {warnings.length > 0 && (
                        <section data-testid="import-preview-warnings" aria-label="Warnings">
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-warning-foreground mb-1">
                                Warnings ({warnings.length}) — informational, will not block import
                            </h3>
                            <ul className="space-y-1">
                                {warnings.map((w, i) => (
                                    <li
                                        key={`warn-${i}`}
                                        data-testid="import-preview-warning"
                                        data-code={w.code}
                                        className="rounded border border-amber-500/40 bg-amber-500/10 px-2 py-1 text-foreground/85"
                                    >
                                        <div className="font-bold">{w.code}</div>
                                        <div>{w.message}</div>
                                        <PositionalTrailer issue={w} />
                                    </li>
                                ))}
                            </ul>
                        </section>
                    )}

                    {errors.length === 0 && warnings.length === 0 && report.ok && (
                        <p className="text-muted-foreground italic">
                            Template validated cleanly. Apply / import flow lands in a later phase.
                        </p>
                    )}
                </div>

                <div className="px-4 py-3 border-t border-border/60 flex items-center justify-end gap-2">
                    <button
                        type="button"
                        onClick={onClose}
                        disabled={applying || savingToLibrary || applyingV2}
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40"
                    >
                        Close
                    </button>
                    {onSaveToLibrary && (
                        <button
                            type="button"
                            data-testid="import-preview-save-to-library"
                            onClick={onSaveToLibrary}
                            disabled={!report.ok || savingToLibrary}
                            className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                report.ok && !savingToLibrary
                                    ? 'bg-blue-700/80 text-white hover:bg-blue-700 shadow-sm'
                                    : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                            }`}
                        >
                            {savingToLibrary ? 'Saving…' : 'Save to Library'}
                        </button>
                    )}
                    {onApply && (
                        <button
                            type="button"
                            data-testid="import-preview-apply"
                            onClick={onApply}
                            disabled={!report.ok || applying}
                            className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                report.ok && !applying
                                    ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                    : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                            }`}
                        >
                            {applying ? 'Applying…' : 'Apply to workspace'}
                        </button>
                    )}
                    {v2ApplyVisible && (
                        <button
                            type="button"
                            data-testid="import-preview-apply-v2"
                            onClick={onApplyV2}
                            disabled={!v2ApplyEnabled}
                            title={v2ApplyDisabledReason || 'Apply schema v2 profile/stats to the selected character.'}
                            aria-label={v2ApplyDisabledReason || 'Apply schema v2 profile/stats to the selected character.'}
                            className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                v2ApplyEnabled
                                    ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                    : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                            }`}
                        >
                            {applyingV2 ? 'Applying…' : 'Apply to character'}
                        </button>
                    )}
                    {v2OverridesVisible && (
                        <button
                            type="button"
                            data-testid="import-preview-apply-v2-overrides"
                            onClick={onApplyV2WithOverrides}
                            disabled={!v2OverridesEnabled}
                            title={
                                v2OverridesDisabledReason ||
                                'Edit profile/stats values before applying schema v2 to the selected character.'
                            }
                            aria-label={
                                v2OverridesDisabledReason ||
                                'Apply schema v2 with editable profile/stats overrides.'
                            }
                            className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                v2OverridesEnabled
                                    ? 'border border-green-700/70 text-green-200 hover:bg-green-900/30'
                                    : 'opacity-40 cursor-not-allowed border border-border/60 text-muted-foreground'
                            }`}
                        >
                            Apply with overrides…
                        </button>
                    )}
                </div>
            </div>
        </div>
    );
}

function PositionalTrailer({ issue }: { issue: templates.ImportPreviewIssue }) {
    const parts: string[] = [];
    if (issue.container) parts.push(issue.container);
    if (issue.position !== undefined) parts.push(`pos ${issue.position}`);
    if (issue.baseItemID) parts.push(`baseItemID 0x${issue.baseItemID.toString(16).toUpperCase()}`);
    if (issue.aowItemID) parts.push(`aowItemID 0x${issue.aowItemID.toString(16).toUpperCase()}`);
    if (parts.length === 0) return null;
    return <div className="mt-0.5 text-[10px] text-muted-foreground">{parts.join(' · ')}</div>;
}

// isCancelledPreview detects the sentinel report returned by the backend
// when the user dismissed the open-file dialog: not OK, no issues, no
// items. Kept as a tiny helper so SortOrderTab and tests can share the
// detection logic.
export function isCancelledPreview(report: templates.ImportPreviewReport): boolean {
    if (report.ok) return false;
    if ((report.errors ?? []).length > 0) return false;
    if ((report.warnings ?? []).length > 0) return false;
    const s = report.summary;
    if (!s) return true;
    return (
        (s.inventoryItems ?? 0) === 0 &&
        (s.storageItems ?? 0) === 0 &&
        (s.weapons ?? 0) === 0 &&
        (s.armor ?? 0) === 0 &&
        (s.talismans ?? 0) === 0 &&
        (s.stackables ?? 0) === 0 &&
        (s.aowAssignments ?? 0) === 0
    );
}
