import { useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import toast from '../lib/toast';
import {
    applyRepairsExternal,
    applyRepairsLoaded,
    makeRepairTarget,
    type RepairApplyReport,
    type RepairApplyTarget,
    type RepairIssue,
    type RepairIssueReport,
    type RepairSource,
} from '../lib/repairIssues';

interface Props {
    reports: RepairIssueReport[];
    source: RepairSource;
    charIndex: number;
    onClose: () => void;
    onSaved: () => void;
    applyRepairs?: (targets: RepairApplyTarget[], stopOnFirstFailure: boolean) => Promise<RepairApplyReport>;
}

type Phase =
    | { kind: 'issues' }
    | { kind: 'repairing' }
    | { kind: 'report'; report: RepairApplyReport };

type FlatIssue = RepairIssue & {
    slotIndex: number;
    charName: string;
};

const NO_OP_ACTIONS = new Set(['leave_unchanged', 'no_action', 'report_only']);

function codeLabel(code: string): string {
    switch (code) {
        case 'upgrade_out_of_range': return 'Upgrade out of range';
        case 'pending_aow_unknown': return 'Pending AoW unknown';
        case 'pending_aow_conflict': return 'Pending AoW conflict';
        case 'duplicate_uid': return 'Duplicate UID';
        case 'duplicate_handle': return 'Duplicate handle';
        case 'unknown_item_id': return 'Unknown item ID';
        case 'quantity_zero': return 'Quantity zero';
        case 'pass_through_records': return 'Pass-through records';
        case 'inventory_reserved': return 'Reserved acquisition index';
        case 'duplicate_acquisition_index': return 'Duplicate acquisition index';
        case 'current_aow_missing': return 'Current AoW missing';
        case 'current_aow_shared': return 'Shared current AoW';
        case 'current_aow_non_aow_category': return 'Current AoW category mismatch';
        case 'stats_formula': return 'Level/stat mismatch';
        case 'category_unsupported': return 'Unsupported category';
        default: return code.replaceAll('_', ' ');
    }
}

function formatHex(value: number | undefined): string {
    if (!value) return '0x00000000';
    return '0x' + value.toString(16).toUpperCase().padStart(8, '0');
}

function actionLabel(issue: RepairIssue, id: string): string {
    return issue.actions.find(a => a.id === id)?.label ?? id.replaceAll('_', ' ');
}

function issueTitle(issue: RepairIssue): string {
    const record = issue.record;
    if (record?.name) {
        const name = record.currentUpgrade > 0 ? `${record.name} +${record.currentUpgrade}` : record.name;
        if (record.infusionName) return `${name} · ${record.infusionName}`;
        return name;
    }
    if (record?.unknown) {
        return `Unknown item · ${formatHex(record.itemId)}`;
    }
    if (issue.key.domain === 'aow') {
        return `Weapon ${formatHex(issue.key.handle)}`;
    }
    return codeLabel(issue.key.code);
}

function issueSubtitle(issue: RepairIssue): string {
    const parts: string[] = [];
    if (issue.record?.category) parts.push(issue.record.category);
    if (issue.record && issue.record.quantity > 1) parts.push(`x${issue.record.quantity}`);
    if (issue.key.scope && issue.key.row >= 0) parts.push(`${issue.key.scope} row ${issue.key.row}`);
    if (issue.key.domain === 'aow') parts.push('Ash of War');
    return parts.join(' · ');
}

function severityClass(severity: string): string {
    switch (severity) {
        case 'error':
        case 'critical':
            return 'bg-red-500/10 border-red-500/30 text-red-400';
        case 'warning':
            return 'bg-yellow-500/10 border-yellow-500/30 text-warning-foreground';
        default:
            return 'bg-blue-500/10 border-blue-500/30 text-blue-400';
    }
}

function isMutatingAction(action: string): boolean {
    return !NO_OP_ACTIONS.has(action);
}

export function InventoryIssuesModal({ reports, source, charIndex, onClose, onSaved, applyRepairs }: Props) {
    const [phase, setPhase] = useState<Phase>({ kind: 'issues' });
    const [checked, setChecked] = useState<Set<string>>(() => new Set(
        reports.flatMap(r => r.issues).filter(i => isMutatingAction(i.defaultAction)).map(i => i.issueID),
    ));
    const [selectedActions, setSelectedActions] = useState<Record<string, string>>(() => {
        const map: Record<string, string> = {};
        for (const issue of reports.flatMap(r => r.issues)) map[issue.issueID] = issue.defaultAction;
        return map;
    });
    const [expanded, setExpanded] = useState<Set<string>>(new Set());
    const [stopOnFirstFailure, setStopOnFirstFailure] = useState(false);

    const issues = useMemo<FlatIssue[]>(() => reports.flatMap(report =>
        report.issues.map(issue => ({ ...issue, slotIndex: report.slotIndex, charName: report.charName })),
    ), [reports]);

    const totalMutating = issues.filter(issue => isMutatingAction(selectedActions[issue.issueID] ?? issue.defaultAction)).length;
    const selectedCount = issues.filter(issue => checked.has(issue.issueID)).length;

    const invokeApply = applyRepairs ?? (async (targets, stop) => {
        if (source === 'external') return applyRepairsExternal(targets, stop);
        return applyRepairsLoaded(charIndex, targets, stop);
    });

    const setAction = (issue: RepairIssue, action: string) => {
        setSelectedActions(prev => ({ ...prev, [issue.issueID]: action }));
        setChecked(prev => {
            const next = new Set(prev);
            if (isMutatingAction(action)) next.add(issue.issueID);
            else next.delete(issue.issueID);
            return next;
        });
    };

    const toggleIssue = (issueID: string) => {
        setChecked(prev => {
            const next = new Set(prev);
            if (next.has(issueID)) next.delete(issueID);
            else next.add(issueID);
            return next;
        });
    };

    const toggleExpanded = (issueID: string) => {
        setExpanded(prev => {
            const next = new Set(prev);
            if (next.has(issueID)) next.delete(issueID);
            else next.add(issueID);
            return next;
        });
    };

    const applyTargets = async (targets: RepairApplyTarget[]) => {
        if (targets.length === 0) return;
        setPhase({ kind: 'repairing' });
        try {
            const report = await invokeApply(targets, stopOnFirstFailure);
            setPhase({ kind: 'report', report });
        } catch (e) {
            toast.error(`Repair failed: ${String(e)}`);
            setPhase({ kind: 'issues' });
        }
    };

    const handleRepairSelected = () => {
        const targets = issues
            .filter(issue => checked.has(issue.issueID))
            .map(issue => makeRepairTarget(issue, selectedActions[issue.issueID] ?? issue.defaultAction));
        void applyTargets(targets);
    };

    const handleSkip = () => {
        if (issues.length > 0) toast(`${issues.length} issue(s) left unchanged.`);
        onClose();
    };

    const isRepairing = phase.kind === 'repairing';

    return createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm animate-in fade-in duration-150 p-4">
            <div
                className="relative bg-card border border-border rounded-xl shadow-2xl flex flex-col max-h-[85vh] w-full max-w-3xl animate-in zoom-in-95 duration-200"
                onClick={e => e.stopPropagation()}
            >
                <div className="flex items-start gap-3 px-6 pt-6 pb-4 shrink-0">
                    <div className="w-9 h-9 rounded-lg bg-destructive/10 border border-destructive/30 flex items-center justify-center shrink-0">
                        <svg className="w-5 h-5 text-destructive" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
                        </svg>
                    </div>
                    <div className="flex-1 min-w-0">
                        <h3 className="text-[13px] font-black uppercase tracking-[0.12em] text-foreground">
                            Repair Issues
                        </h3>
                        <p className="text-[10px] text-muted-foreground mt-0.5">
                            {phase.kind === 'issues' && `${issues.length} issue(s) · ${totalMutating} repair action(s) available`}
                            {phase.kind === 'repairing' && 'Applying selected actions...'}
                            {phase.kind === 'report' && `${phase.report.applied} applied · ${phase.report.failed} failed · ${phase.report.skipped} skipped`}
                        </p>
                    </div>
                    <button onClick={onClose} disabled={isRepairing} className="text-muted-foreground/50 hover:text-foreground transition-colors p-1 shrink-0 disabled:opacity-40">
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                <div className="flex-1 overflow-y-auto px-6 pb-4 space-y-4 min-h-0">
                    {phase.kind === 'repairing' && (
                        <div className="flex flex-col items-center justify-center py-10 gap-3">
                            <svg className="w-8 h-8 text-primary animate-spin" fill="none" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                            </svg>
                            <p className="text-[10px] text-muted-foreground uppercase tracking-widest">Applying repairs...</p>
                        </div>
                    )}

                    {phase.kind === 'issues' && (
                        <>
                            {issues.length === 0 ? (
                                <div className="flex flex-col items-center justify-center py-8 gap-3 text-center">
                                    <svg className="w-10 h-10 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                    <p className="text-[11px] font-black uppercase tracking-widest text-green-400">No repair issues found</p>
                                </div>
                            ) : (
                                <div className="space-y-3">
                                    {reports.map(report => (
                                        <div key={`${report.source}-${report.slotIndex}`} className="space-y-2">
                                            {reports.length > 1 && (
                                                <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">
                                                    Slot {report.slotIndex + 1}{report.charName ? ` - ${report.charName}` : ''}
                                                </p>
                                            )}
                                            {report.issues.map(issue => {
                                                const selected = selectedActions[issue.issueID] ?? issue.defaultAction;
                                                const isExpanded = expanded.has(issue.issueID);
                                                const pickRequiresInput = selected === 'pick_aow';
                                                return (
                                                    <div key={issue.issueID} className="rounded-lg border border-border/50 bg-background/30 overflow-hidden">
                                                        <div className="grid grid-cols-[auto_minmax(0,1fr)_minmax(150px,220px)_auto] gap-3 items-center p-3">
                                                            <input
                                                                aria-label={`Select ${issueTitle(issue)}`}
                                                                type="checkbox"
                                                                checked={checked.has(issue.issueID)}
                                                                disabled={!isMutatingAction(selected) || isRepairing}
                                                                onChange={() => toggleIssue(issue.issueID)}
                                                                className="accent-primary"
                                                            />
                                                            <div className="min-w-0">
                                                                <div className="flex items-center gap-2 min-w-0">
                                                                    <span className={`px-1.5 py-0.5 rounded border text-[8px] font-black uppercase tracking-widest ${severityClass(issue.severity)}`}>
                                                                        {codeLabel(issue.key.code)}
                                                                    </span>
                                                                    <span className="text-[10px] font-bold text-foreground truncate">{issueTitle(issue)}</span>
                                                                </div>
                                                                <p className="text-[9px] text-muted-foreground mt-1 truncate">{issueSubtitle(issue) || issue.description}</p>
                                                            </div>
                                                            {issue.actions.length > 1 ? (
                                                                <>
                                                                    <label className="sr-only" htmlFor={`action-${issue.issueID}`}>Repair action</label>
                                                                    <select
                                                                        id={`action-${issue.issueID}`}
                                                                        value={selected}
                                                                        onChange={e => setAction(issue, e.target.value)}
                                                                        disabled={isRepairing}
                                                                        className="w-full rounded border border-border bg-card px-2 py-1.5 text-[9px] text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                                                                    >
                                                                        {issue.actions.map(action => (
                                                                            <option key={action.id} value={action.id}>{action.label}</option>
                                                                        ))}
                                                                    </select>
                                                                </>
                                                            ) : (
                                                                <span className="w-full rounded border border-border/60 bg-muted/10 px-2 py-1.5 text-[9px] text-muted-foreground">
                                                                    {actionLabel(issue, selected)}
                                                                </span>
                                                            )}
                                                            <button
                                                                type="button"
                                                                onClick={() => toggleExpanded(issue.issueID)}
                                                                className="px-2 py-1 text-[8px] font-black uppercase tracking-widest rounded border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-all"
                                                            >
                                                                {isExpanded ? 'Hide' : 'Details'}
                                                            </button>
                                                        </div>
                                                        {pickRequiresInput && (
                                                            <div className="mx-3 mb-3 rounded border border-yellow-500/30 bg-yellow-500/10 px-3 py-2 text-[9px] text-warning-foreground">
                                                                Compatible Ash of War selection needs a concrete AoW handle. Use Clear Ash of War or Create separate copy until the picker is wired.
                                                            </div>
                                                        )}
                                                        {isExpanded && (
                                                            <div className="border-t border-border/40 bg-muted/10 p-3 text-[9px] text-muted-foreground space-y-2">
                                                                <p className="text-foreground">{issue.description}</p>
                                                                {issue.capacity && (
                                                                    <p>Capacity: {issue.capacity.resource} needs {issue.capacity.needed}, available {issue.capacity.available}</p>
                                                                )}
                                                                <div className="grid grid-cols-2 gap-x-4 gap-y-1">
                                                                    <span>Scope: {issue.key.scope || 'n/a'}</span>
                                                                    <span>Row: {issue.key.row}</span>
                                                                    <span>Handle: {formatHex(issue.key.handle)}</span>
                                                                    <span>ItemID: {formatHex(issue.record?.itemId)}</span>
                                                                    <span>Fingerprint: {issue.fingerprint || 'n/a'}</span>
                                                                    <span>Debug: {issue.debugKey || 'n/a'}</span>
                                                                </div>
                                                            </div>
                                                        )}
                                                    </div>
                                                );
                                            })}
                                        </div>
                                    ))}
                                </div>
                            )}
                        </>
                    )}

                    {phase.kind === 'report' && (
                        <div className="space-y-3">
                            {phase.report.results.length === 0 ? (
                                <p className="text-[10px] text-muted-foreground text-center py-4">No actions were submitted.</p>
                            ) : phase.report.results.map((result, i) => (
                                <div key={`${result.issueID}-${i}`} className={`p-2 rounded-lg border text-[9px] ${
                                    result.outcome === 'applied' ? 'bg-green-500/10 border-green-500/30 text-green-400' :
                                    result.outcome === 'failed' ? 'bg-red-500/10 border-red-500/30 text-red-400' :
                                    result.outcome === 'needsUserInput' ? 'bg-yellow-500/10 border-yellow-500/30 text-warning-foreground' :
                                    'bg-muted/20 border-border text-muted-foreground'
                                }`}>
                                    <span className="font-black uppercase tracking-widest">{result.outcome}</span>
                                    <span className="ml-2 text-foreground/80">Slot {result.slotIndex + 1} · {result.action}</span>
                                    {result.message && <p className="mt-1 text-muted-foreground">{result.message}</p>}
                                </div>
                            ))}
                            {phase.report.stopped && (
                                <p className="text-[9px] text-warning-foreground">Batch stopped after the first failed action.</p>
                            )}
                        </div>
                    )}
                </div>

                <div className="flex flex-wrap items-center justify-between gap-3 px-6 py-4 border-t border-border shrink-0">
                    {phase.kind === 'issues' && (
                        <>
                            <label className="flex items-center gap-2 text-[9px] text-muted-foreground uppercase tracking-widest">
                                <input
                                    type="checkbox"
                                    checked={stopOnFirstFailure}
                                    onChange={e => setStopOnFirstFailure(e.target.checked)}
                                    disabled={isRepairing}
                                    className="accent-primary"
                                />
                                Stop on each problem
                            </label>
                            <div className="ml-auto flex flex-wrap items-center gap-2">
                                <button
                                    onClick={handleSkip}
                                    disabled={isRepairing}
                                    className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30 disabled:opacity-40 disabled:cursor-not-allowed transition-all"
                                >
                                    Close
                                </button>
                                <button
                                    onClick={handleRepairSelected}
                                    disabled={isRepairing || selectedCount === 0}
                                    className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-40 disabled:cursor-not-allowed transition-all"
                                >
                                    Repair selected ({selectedCount})
                                </button>
                            </div>
                        </>
                    )}
                    {phase.kind === 'report' && (
                        <>
                            <button
                                onClick={onClose}
                                className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-all"
                            >
                                Close
                            </button>
                            <button
                                onClick={onSaved}
                                className="ml-auto px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded bg-primary text-primary-foreground hover:bg-primary/90 transition-all"
                            >
                                Close and refresh
                            </button>
                        </>
                    )}
                </div>
            </div>
        </div>,
        document.body,
    );
}
