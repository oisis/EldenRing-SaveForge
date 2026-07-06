import { useState, useMemo } from 'react';
import { createPortal } from 'react-dom';
import {
    RepairInventoryWorkspaceItem,
    RepairInventoryWorkspaceItems,
} from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
import toast from '../lib/toast';

interface Props {
    report: main.InventoryIssuesScanReport;
    onClose: () => void;   // Skip — no repairs done
    onSaved: () => void;   // After repair + workspace save confirmed
}

type Phase =
    | { kind: 'issues' }
    | { kind: 'repairing' }
    | { kind: 'report'; fixed: string[]; skipped: string[] };

const AUTO_REPAIR_CODES = new Set(['upgrade_out_of_range', 'pending_aow_unknown', 'pending_aow_conflict']);

function codeLabel(code: string): string {
    switch (code) {
        case 'upgrade_out_of_range':  return 'Upgrade out of range';
        case 'pending_aow_unknown':   return 'Pending AoW unknown';
        case 'pending_aow_conflict':  return 'Pending AoW conflict';
        case 'duplicate_uid':         return 'Duplicate UID';
        case 'duplicate_handle':      return 'Duplicate handle';
        case 'unknown_item_id':       return 'Unknown item ID';
        case 'quantity_zero':         return 'Quantity zero';
        case 'pass_through_records':  return 'Pass-through records';
        default:                      return code;
    }
}

export function InventoryIssuesModal({ report, onClose, onSaved }: Props) {
    const [phase, setPhase] = useState<Phase>({ kind: 'issues' });
    const [fixingKey, setFixingKey] = useState<string | null>(null);

    // Repairable issues (mutable — shrinks as items get individually fixed)
    const [remaining, setRemaining] = useState<main.WorkspaceIssueDetail[]>(
        () => report.workspaceIssues.filter(i => i.canRepair && i.uid),
    );
    const nonRepairable = useMemo(
        () => report.workspaceIssues.filter(i => !i.canRepair || !i.uid),
        [report.workspaceIssues],
    );

    // Checkboxes — key: uid:code
    const [checked, setChecked] = useState<Set<string>>(
        () => new Set(report.workspaceIssues.filter(i => i.canRepair && i.uid).map(i => `${i.uid}:${i.code}`)),
    );

    // Group repairable issues by code
    const groups = useMemo(() => {
        const map = new Map<string, main.WorkspaceIssueDetail[]>();
        for (const iss of remaining) {
            const list = map.get(iss.code) ?? [];
            list.push(iss);
            map.set(iss.code, list);
        }
        return map;
    }, [remaining]);

    const totalChecked = remaining.filter(i => checked.has(`${i.uid}:${i.code}`)).length;

    const toggleCheck = (uid: string, code: string) => {
        const k = `${uid}:${code}`;
        setChecked(prev => {
            const next = new Set(prev);
            if (next.has(k)) next.delete(k); else next.add(k);
            return next;
        });
    };

    const toggleGroupCheck = (code: string, items: main.WorkspaceIssueDetail[]) => {
        const keys = items.map(i => `${i.uid}:${i.code}`);
        const allChecked = keys.every(k => checked.has(k));
        setChecked(prev => {
            const next = new Set(prev);
            if (allChecked) keys.forEach(k => next.delete(k));
            else keys.forEach(k => next.add(k));
            return next;
        });
    };

    const removeFromRemaining = (uid: string, code: string) => {
        setRemaining(prev => prev.filter(i => !(i.uid === uid && i.code === code)));
        setChecked(prev => { const n = new Set(prev); n.delete(`${uid}:${code}`); return n; });
    };

    const handleFixOne = async (uid: string, code: string) => {
        const key = `${uid}:${code}`;
        setFixingKey(key);
        try {
            await RepairInventoryWorkspaceItem(report.sessionID, uid, code);
            removeFromRemaining(uid, code);
        } catch (e) {
            toast.error(`Fix failed: ${String(e)}`);
        } finally {
            setFixingKey(null);
        }
    };

    const handleFixGroup = async (code: string, items: main.WorkspaceIssueDetail[]) => {
        const specs = items.map(i => main.WorkspaceRepairSpec.createFrom({ uid: i.uid, code: i.code }));
        setPhase({ kind: 'repairing' });
        try {
            await RepairInventoryWorkspaceItems(report.sessionID, specs);
            const fixed = items.map(i => i.itemName ? `${i.itemName} — ${i.repairDesc}` : i.message);
            setRemaining(prev => prev.filter(i => i.code !== code));
            setChecked(prev => {
                const n = new Set(prev);
                items.forEach(i => n.delete(`${i.uid}:${i.code}`));
                return n;
            });
            setPhase({ kind: 'report', fixed, skipped: [] });
        } catch (e) {
            toast.error(`Repair failed: ${String(e)}`);
            setPhase({ kind: 'issues' });
        }
    };

    const handleRepairSelected = async () => {
        const specs = remaining
            .filter(i => checked.has(`${i.uid}:${i.code}`))
            .map(i => main.WorkspaceRepairSpec.createFrom({ uid: i.uid, code: i.code }));
        if (specs.length === 0) return;
        setPhase({ kind: 'repairing' });
        try {
            const snap = await RepairInventoryWorkspaceItems(report.sessionID, specs);
            const newErrorUIDs = new Set([
                ...(snap.validation?.errors ?? []).map(e => `${e.uid}:${e.code}`),
                ...(snap.validation?.warnings ?? []).map(w => `${w.uid}:${w.code}`),
            ]);
            const fixed: string[] = [];
            const skipped: string[] = [];
            for (const s of specs) {
                const key = `${s.uid}:${s.code}`;
                const iss = remaining.find(i => `${i.uid}:${i.code}` === key);
                const label = iss?.itemName ? `${iss.itemName} — ${iss.repairDesc || s.code}` : s.code;
                if (newErrorUIDs.has(key)) skipped.push(label);
                else fixed.push(label);
            }
            setPhase({ kind: 'report', fixed, skipped });
        } catch (e) {
            toast.error(`Repair failed: ${String(e)}`);
            setPhase({ kind: 'issues' });
        }
    };

    const handleSkip = () => {
        const count = report.workspaceIssues.length + report.binaryIssues.length;
        if (count > 0) {
            toast(`${count} issue(s) found. You can repair them anytime via Tools → Diagnostic.`);
        }
        onClose();
    };

    const hasBinaryIssues = report.binaryIssues.length > 0;
    const isRepairing = phase.kind === 'repairing';
    const isBusy = isRepairing || fixingKey !== null;

    return createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm animate-in fade-in duration-150 p-4">
            <div
                className="relative bg-card border border-border rounded-xl shadow-2xl flex flex-col max-h-[85vh] w-full max-w-xl animate-in zoom-in-95 duration-200"
                onClick={e => e.stopPropagation()}
            >
                {/* Header */}
                <div className="flex items-start gap-3 px-6 pt-6 pb-4 shrink-0">
                    <div className="w-9 h-9 rounded-lg bg-destructive/10 border border-destructive/30 flex items-center justify-center shrink-0">
                        <svg className="w-5 h-5 text-destructive" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
                                d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
                        </svg>
                    </div>
                    <div className="flex-1 min-w-0">
                        <h3 className="text-[13px] font-black uppercase tracking-[0.12em] text-foreground">
                            Issues Found
                        </h3>
                        <p className="text-[10px] text-muted-foreground mt-0.5">
                            {phase.kind === 'issues' && `${report.workspaceIssues.length} workspace · ${report.binaryIssues.length} binary`}
                            {phase.kind === 'repairing' && 'Applying repairs…'}
                            {phase.kind === 'report' && 'Repair complete'}
                        </p>
                    </div>
                </div>

                {/* Body */}
                <div className="flex-1 overflow-y-auto px-6 pb-4 space-y-4 min-h-0">

                    {/* Spinner */}
                    {phase.kind === 'repairing' && (
                        <div className="flex flex-col items-center justify-center py-10 gap-3">
                            <svg className="w-8 h-8 text-primary animate-spin" fill="none" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                            </svg>
                            <p className="text-[10px] text-muted-foreground uppercase tracking-widest">Applying repairs…</p>
                        </div>
                    )}

                    {/* Issue list */}
                    {phase.kind === 'issues' && (
                        <>
                            {/* Binary issues — display only */}
                            {hasBinaryIssues && (
                                <div>
                                    <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground mb-1.5">
                                        Binary Issues — use Tools → Diagnostic to repair
                                    </p>
                                    <div className="space-y-1">
                                        {report.binaryIssues.map((iss, i) => (
                                            <div key={i} className="flex items-start gap-2 p-2 rounded-lg bg-red-500/10 border border-red-500/20 text-[9px] text-foreground">
                                                <span className={`shrink-0 font-bold ${iss.severity === 'critical' ? 'text-red-400' : 'text-yellow-400'}`}>
                                                    [{iss.severity}]
                                                </span>
                                                <span className="text-muted-foreground">{iss.description}</span>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}

                            {/* Auto-repairable groups */}
                            {groups.size > 0 && (
                                <div>
                                    <p className="text-[9px] font-black uppercase tracking-widest text-green-400 mb-2">
                                        Auto-Repairable
                                    </p>
                                    <div className="space-y-3">
                                        {Array.from(groups.entries()).map(([code, items]) => {
                                            const allGroupChecked = items.every(i => checked.has(`${i.uid}:${i.code}`));
                                            return (
                                                <div key={code} className="rounded-lg border border-border/40 bg-background/30 overflow-hidden">
                                                    {/* Group header */}
                                                    <div className="flex items-center gap-2 px-3 py-2 bg-muted/20">
                                                        <input
                                                            type="checkbox"
                                                            checked={allGroupChecked}
                                                            onChange={() => toggleGroupCheck(code, items)}
                                                            disabled={isBusy}
                                                            className="accent-primary"
                                                        />
                                                        <span className="flex-1 text-[9px] font-black uppercase tracking-widest text-foreground">
                                                            {codeLabel(code)} ({items.length})
                                                        </span>
                                                        <button
                                                            onClick={() => handleFixGroup(code, items)}
                                                            disabled={isBusy}
                                                            className="px-2 py-0.5 text-[8px] font-black uppercase tracking-widest rounded bg-primary/20 text-primary border border-primary/30 hover:bg-primary/30 disabled:opacity-40 disabled:cursor-not-allowed transition-all"
                                                        >
                                                            Fix all
                                                        </button>
                                                    </div>
                                                    {/* Group items */}
                                                    <ul className="divide-y divide-border/30">
                                                        {items.map(iss => {
                                                            const k = `${iss.uid}:${iss.code}`;
                                                            const isFixingThis = fixingKey === k;
                                                            return (
                                                                <li key={k} className="flex items-center gap-2 px-3 py-1.5">
                                                                    <input
                                                                        type="checkbox"
                                                                        checked={checked.has(k)}
                                                                        onChange={() => toggleCheck(iss.uid, iss.code)}
                                                                        disabled={isBusy}
                                                                        className="accent-primary shrink-0"
                                                                    />
                                                                    <span className="flex-1 text-[9px] text-foreground/80 min-w-0 truncate" title={iss.message}>
                                                                        {iss.itemName || iss.message}
                                                                    </span>
                                                                    {iss.repairDesc && (
                                                                        <span className="text-[8px] text-muted-foreground shrink-0">{iss.repairDesc}</span>
                                                                    )}
                                                                    <button
                                                                        onClick={() => handleFixOne(iss.uid, iss.code)}
                                                                        disabled={isBusy}
                                                                        className="shrink-0 px-2 py-0.5 text-[8px] font-black uppercase tracking-widest rounded border border-primary/30 text-primary hover:bg-primary/20 disabled:opacity-40 disabled:cursor-not-allowed transition-all"
                                                                    >
                                                                        {isFixingThis ? '…' : 'Fix'}
                                                                    </button>
                                                                </li>
                                                            );
                                                        })}
                                                    </ul>
                                                </div>
                                            );
                                        })}
                                    </div>
                                </div>
                            )}

                            {/* Non-repairable */}
                            {nonRepairable.length > 0 && (
                                <div>
                                    <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground mb-1.5">
                                        Unrepairable (manual intervention required)
                                    </p>
                                    <div className="space-y-1">
                                        {nonRepairable.map((iss, i) => (
                                            <div key={i} className="flex items-start gap-2 p-2 rounded-lg bg-muted/10 border border-border/30 text-[9px]">
                                                <span className="font-bold text-muted-foreground shrink-0">[{codeLabel(iss.code)}]</span>
                                                <span className="text-muted-foreground">{iss.message}</span>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}

                            {/* Empty state — all fixed individually */}
                            {groups.size === 0 && nonRepairable.length === 0 && !hasBinaryIssues && (
                                <div className="flex flex-col items-center justify-center py-8 gap-3 text-center">
                                    <svg className="w-10 h-10 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                    <p className="text-[11px] font-black uppercase tracking-widest text-green-400">All issues resolved</p>
                                </div>
                            )}
                        </>
                    )}

                    {/* Repair report */}
                    {phase.kind === 'report' && (
                        <>
                            {phase.fixed.length > 0 && (
                                <div>
                                    <p className="text-[9px] font-black uppercase tracking-widest text-green-400 mb-1.5">Fixed</p>
                                    <div className="space-y-1">
                                        {phase.fixed.map((f, i) => (
                                            <div key={i} className="flex items-center gap-2 p-2 rounded-lg bg-green-500/10 border border-green-500/30 text-[9px] text-foreground">
                                                <svg className="w-3 h-3 text-green-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                                                </svg>
                                                {f}
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}
                            {phase.skipped.length > 0 && (
                                <div>
                                    <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground mb-1.5">Skipped</p>
                                    <div className="space-y-1">
                                        {phase.skipped.map((s, i) => (
                                            <div key={i} className="flex items-center gap-2 p-2 rounded-lg bg-muted/20 border border-border text-[9px] text-muted-foreground">
                                                <svg className="w-3 h-3 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
                                                </svg>
                                                {s}
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}
                            {phase.fixed.length === 0 && phase.skipped.length === 0 && (
                                <p className="text-[10px] text-muted-foreground text-center py-4">No changes applied.</p>
                            )}
                        </>
                    )}
                </div>

                {/* Footer */}
                <div className="flex items-center justify-between gap-3 px-6 py-4 border-t border-border shrink-0">
                    {phase.kind === 'issues' && (
                        <>
                            <button
                                onClick={handleSkip}
                                disabled={isBusy}
                                className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30 disabled:opacity-40 disabled:cursor-not-allowed transition-all"
                            >
                                Skip — fix later in Tools
                            </button>
                            <button
                                onClick={handleRepairSelected}
                                disabled={isBusy || totalChecked === 0}
                                className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-40 disabled:cursor-not-allowed transition-all"
                            >
                                Repair selected ({totalChecked})
                            </button>
                        </>
                    )}
                    {phase.kind === 'report' && (
                        <>
                            <button
                                onClick={onSaved}
                                className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-all"
                            >
                                Close
                            </button>
                            <button
                                onClick={onSaved}
                                className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded bg-primary text-primary-foreground hover:bg-primary/90 transition-all"
                            >
                                Save
                            </button>
                        </>
                    )}
                    {/* When all fixed individually — show only Close */}
                    {phase.kind === 'issues' && groups.size === 0 && nonRepairable.length === 0 && !hasBinaryIssues && (
                        <button
                            onClick={onSaved}
                            className="ml-auto px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded bg-primary text-primary-foreground hover:bg-primary/90 transition-all"
                        >
                            Done
                        </button>
                    )}
                </div>
            </div>
        </div>,
        document.body,
    );
}
