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
}

export function ImportTemplatePreviewModal({ report, onClose }: Props) {
    const dialogRef = useRef<HTMLDivElement | null>(null);
    useEffect(() => {
        dialogRef.current?.focus();
    }, []);

    const errors = report.errors ?? [];
    const warnings = report.warnings ?? [];
    const summary = report.summary;

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
                </div>

                <div className="px-4 py-3 space-y-3 overflow-y-auto text-[12px]">
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
                            <h3 className="text-[10px] font-bold uppercase tracking-wider text-amber-300 mb-1">
                                Warnings ({warnings.length}) — informational, will not block import
                            </h3>
                            <ul className="space-y-1">
                                {warnings.map((w, i) => (
                                    <li
                                        key={`warn-${i}`}
                                        data-testid="import-preview-warning"
                                        data-code={w.code}
                                        className="rounded border border-amber-500/40 bg-amber-500/10 px-2 py-1 text-amber-200"
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
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                    >
                        Close
                    </button>
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
