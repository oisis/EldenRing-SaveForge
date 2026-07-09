import {useState, useEffect} from 'react';
import {createPortal} from 'react-dom';
import {RepairAllLoadedSlots} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';

// DiagnosticsModal now serves a single responsibility: display the post-load
// all-slots DiagnosticsReport that App.tsx produces via RunDiagnosticsAllLoaded
// and let the user apply automated repairs to every loaded slot at once.
// External-file scanning and the loaded/external choice screen were removed —
// diagnostics operate only on the currently loaded save.
type Mode =
    | {step: 'report'}
    | {step: 'repairing'}
    | {step: 'repair-report'; repairReport: main.RepairReport};

interface Props {
    initialReport: main.DiagnosticsReport;
    onClose: () => void;
}

const severityColor: Record<string, string> = {
    critical: 'text-red-400',
    warning: 'text-warning-foreground',
    info: 'text-blue-400',
};

const severityBg: Record<string, string> = {
    critical: 'bg-red-500/10 border-red-500/30',
    warning: 'bg-yellow-500/10 border-yellow-500/30',
    info: 'bg-blue-500/10 border-blue-500/30',
};

const BtnSecondary = ({onClick, children}: {onClick: () => void; children: React.ReactNode}) => (
    <button
        onClick={onClick}
        className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-all"
    >
        {children}
    </button>
);

const BtnPrimary = ({onClick, disabled, children}: {onClick: () => void; disabled?: boolean; children: React.ReactNode}) => (
    <button
        onClick={onClick}
        disabled={disabled}
        className="px-4 py-1.5 text-[9px] font-black uppercase tracking-widest rounded bg-primary text-primary-foreground hover:brightness-110 active:scale-[0.98] transition-all disabled:opacity-40 disabled:cursor-not-allowed"
    >
        {children}
    </button>
);

export function DiagnosticsModal({initialReport, onClose}: Props) {
    const [mode, setMode] = useState<Mode>({step: 'report'});
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, [onClose]);

    const handleRepair = async () => {
        setMode({step: 'repairing'});
        setError(null);
        try {
            const repairReport = await RepairAllLoadedSlots();
            setMode({step: 'repair-report', repairReport});
        } catch (e) {
            setError(String(e));
            setMode({step: 'report'});
        }
    };

    const allIssues = initialReport.slots.flatMap(s => s.issues);
    const displayedIssues = allIssues.filter(i => i.severity !== 'info');
    const criticalCount = displayedIssues.filter(i => i.severity === 'critical').length;
    const warningCount = displayedIssues.filter(i => i.severity === 'warning').length;

    const footer = (() => {
        if (mode.step === 'report') {
            return (
                <>
                    <BtnSecondary onClick={onClose}>Close</BtnSecondary>
                    {initialReport.canRepair && (
                        <BtnPrimary onClick={handleRepair}>Repair</BtnPrimary>
                    )}
                </>
            );
        }
        if (mode.step === 'repair-report') {
            return <BtnSecondary onClick={onClose}>Close</BtnSecondary>;
        }
        return null;
    })();

    const subtitle = {
        report: displayedIssues.length > 0 ? `${displayedIssues.length} issue(s) found` : 'No issues found',
        repairing: 'Repairing…',
        'repair-report': 'Repair complete',
    }[mode.step];

    return createPortal(
        <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm animate-in fade-in duration-150 p-4"
            onClick={onClose}
        >
            {/* Panel: flex-col so header/footer stay fixed, body scrolls */}
            <div
                className="relative bg-card border border-border rounded-xl shadow-2xl flex flex-col max-h-[85vh] w-full max-w-lg animate-in zoom-in-95 duration-200"
                onClick={e => e.stopPropagation()}
            >
                {/* ── Header (never scrolls) ── */}
                <div className="flex items-start gap-3 px-6 pt-6 pb-4 shrink-0">
                    <div className="w-9 h-9 rounded-lg bg-primary/10 border border-primary/30 flex items-center justify-center shrink-0">
                        <svg className="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
                                d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                        </svg>
                    </div>
                    <div className="flex-1 min-w-0">
                        <h3 className="text-[13px] font-black uppercase tracking-[0.12em] text-foreground">Diagnostics</h3>
                        <p className="text-[10px] text-muted-foreground mt-0.5">{subtitle}</p>
                    </div>
                    <button onClick={onClose} className="text-muted-foreground/50 hover:text-foreground transition-colors p-1 shrink-0">
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                {/* ── Scrollable body ── */}
                <div className="flex-1 overflow-y-auto px-6 pb-2">
                    {/* Error banner */}
                    {error && (
                        <div className="mb-3 p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-[10px] text-red-400 break-words">
                            {error}
                        </div>
                    )}

                    {/* REPAIRING */}
                    {mode.step === 'repairing' && (
                        <div className="flex flex-col items-center justify-center py-10 gap-3">
                            <svg className="w-8 h-8 text-primary animate-spin" fill="none" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                            </svg>
                            <p className="text-[10px] text-muted-foreground uppercase tracking-widest">Applying repairs…</p>
                        </div>
                    )}

                    {/* REPORT */}
                    {mode.step === 'report' && (
                        <div className="pb-2">
                            {displayedIssues.length === 0 ? (
                                <div className="flex flex-col items-center justify-center py-8 gap-3 text-center">
                                    <svg className="w-10 h-10 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                    <p className="text-[11px] font-black uppercase tracking-widest text-green-400">No issues found</p>
                                    <p className="text-[9px] text-muted-foreground max-w-xs">The save file is clean. Any remaining observations are informational and do not require action.</p>
                                </div>
                            ) : (
                                <>
                                    {(criticalCount > 0 || warningCount > 0) && (
                                        <div className="flex flex-wrap gap-2 mb-3">
                                            {criticalCount > 0 && (
                                                <span className="px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-widest bg-red-500/10 border border-red-500/30 text-red-400">
                                                    {criticalCount} critical
                                                </span>
                                            )}
                                            {warningCount > 0 && (
                                                <span className="px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-widest bg-yellow-500/10 border border-yellow-500/30 text-warning-foreground">
                                                    {warningCount} warning
                                                </span>
                                            )}
                                        </div>
                                    )}
                                    {initialReport.slots.map(slot => {
                                        const slotIssues = slot.issues.filter(iss => iss.severity !== 'info');
                                        return slotIssues.length > 0 ? (
                                            <div key={slot.slotIndex} className="mb-3">
                                                <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground mb-1.5">
                                                    Slot {slot.slotIndex + 1} — {slot.charName || '(unnamed)'}
                                                </p>
                                                <div className="space-y-1">
                                                    {slotIssues.map((iss, idx) => (
                                                        <div key={idx} className={`flex items-start gap-2 p-2 rounded-lg border text-[9px] ${severityBg[iss.severity] ?? 'bg-muted/20 border-border'}`}>
                                                            <span className={`font-black uppercase shrink-0 ${severityColor[iss.severity] ?? 'text-muted-foreground'}`}>
                                                                {iss.severity}
                                                            </span>
                                                            <span className="text-muted-foreground shrink-0">[{iss.category}]</span>
                                                            <span className="text-foreground break-words min-w-0">{iss.description}</span>
                                                        </div>
                                                    ))}
                                                </div>
                                            </div>
                                        ) : null;
                                    })}
                                </>
                            )}
                        </div>
                    )}

                    {/* REPAIR REPORT */}
                    {mode.step === 'repair-report' && (
                        <div className="pb-2">
                            {mode.repairReport.fixed.length > 0 && (
                                <div className="mb-3">
                                    <p className="text-[9px] font-black uppercase tracking-widest text-green-400 mb-1.5">Fixed</p>
                                    <div className="space-y-1">
                                        {mode.repairReport.fixed.map((f, i) => (
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
                            {mode.repairReport.skipped.length > 0 && (
                                <div className="mb-3">
                                    <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground mb-1.5">Skipped (unrepairable)</p>
                                    <div className="space-y-1">
                                        {mode.repairReport.skipped.map((s, i) => (
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
                            {mode.repairReport.fixed.length === 0 && mode.repairReport.skipped.length === 0 && (
                                <p className="text-[10px] text-muted-foreground py-6 text-center">No changes applied.</p>
                            )}
                        </div>
                    )}
                </div>

                {/* ── Footer (never scrolls) ── */}
                {footer && (
                    <div className="flex justify-end gap-2 px-6 py-4 shrink-0 border-t border-border">
                        {footer}
                    </div>
                )}
            </div>
        </div>,
        document.body
    );
}
