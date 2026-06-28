import {useState, useEffect} from 'react';
import {createPortal} from 'react-dom';
import {
    PickDiagnosticsFile,
    RunDiagnosticsLoaded,
    RunDiagnosticsExternal,
    RepairLoadedSave,
    RepairAllLoadedSlots,
    RepairExternal,
    SaveRepairedExternal,
    WriteSave,
} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';

type Source = 'loaded' | 'all-loaded' | 'external';

type Mode =
    | {step: 'choice'}
    | {step: 'scanning'; source: Source; filePath?: string}
    | {step: 'report'; report: main.DiagnosticsReport; filePath: string; source: Source}
    | {step: 'repairing'; source: Source; filePath: string}
    | {step: 'repair-report'; repairReport: main.RepairReport; source: Source; filePath: string}
    | {step: 'saving'; source: Source; filePath: string};

interface Props {
    charIndex: number;
    platform: string | null;
    onClose: () => void;
    initialReport?: main.DiagnosticsReport; // when set: skip 'choice', open at 'report' for all loaded slots
}

const severityColor: Record<string, string> = {
    critical: 'text-red-400',
    warning: 'text-yellow-400',
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

export function DiagnosticsModal({charIndex, platform, onClose, initialReport}: Props) {
    const [mode, setMode] = useState<Mode>(
        initialReport
            ? {step: 'report', report: initialReport, filePath: 'loaded', source: 'all-loaded'}
            : {step: 'choice'}
    );
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, [onClose]);

    const handleScanLoaded = async () => {
        setMode({step: 'scanning', source: 'loaded'});
        setError(null);
        try {
            const report = await RunDiagnosticsLoaded(charIndex);
            setMode({step: 'report', report, filePath: 'loaded', source: 'loaded'});
        } catch (e) {
            setError(String(e));
            setMode({step: 'choice'});
        }
    };

    const handleScanExternal = async () => {
        setError(null);
        try {
            const path = await PickDiagnosticsFile();
            if (!path) return;
            setMode({step: 'scanning', source: 'external', filePath: path});
            const report = await RunDiagnosticsExternal(path);
            setMode({step: 'report', report, filePath: path, source: 'external'});
        } catch (e) {
            setError(String(e));
            setMode({step: 'choice'});
        }
    };

    const handleRepair = async (report: main.DiagnosticsReport, filePath: string, source: Source) => {
        setMode({step: 'repairing', source, filePath});
        setError(null);
        try {
            const repairReport =
                source === 'external' ? await RepairExternal() :
                source === 'all-loaded' ? await RepairAllLoadedSlots() :
                await RepairLoadedSave(charIndex);
            setMode({step: 'repair-report', repairReport, source, filePath});
        } catch (e) {
            setError(String(e));
            setMode({step: 'report', report, filePath, source});
        }
    };

    const handleSave = async (source: Source, filePath: string) => {
        setMode({step: 'saving', source, filePath});
        setError(null);
        try {
            if (source === 'external') {
                await SaveRepairedExternal(filePath);
            } else {
                await WriteSave();
            }
            onClose();
        } catch (e) {
            setError(String(e));
            setMode({step: 'repair-report', repairReport: {fixed: [], skipped: []}, source, filePath});
        }
    };

    const allIssues = mode.step === 'report' ? mode.report.slots.flatMap(s => s.issues) : [];
    const criticalCount = allIssues.filter(i => i.severity === 'critical').length;
    const warningCount = allIssues.filter(i => i.severity === 'warning').length;

    // Footer buttons depend on current step
    const footer = (() => {
        if (mode.step === 'report') {
            const {report, filePath, source} = mode;
            return (
                <>
                    <BtnSecondary onClick={onClose}>Close</BtnSecondary>
                    {report.canRepair && (
                        <BtnPrimary onClick={() => handleRepair(report, filePath, source)}>Repair</BtnPrimary>
                    )}
                </>
            );
        }
        if (mode.step === 'repair-report') {
            const {repairReport, source, filePath} = mode;
            return (
                <>
                    <BtnSecondary onClick={onClose}>Close</BtnSecondary>
                    {source === 'external' && (
                        <BtnPrimary onClick={() => handleSave(source, filePath)} disabled={repairReport.fixed.length === 0}>Save</BtnPrimary>
                    )}
                </>
            );
        }
        return null;
    })();

    const subtitle = {
        choice: 'Select save to scan',
        scanning: 'Scanning…',
        report: `${allIssues.length} issue(s) found`,
        repairing: 'Repairing…',
        'repair-report': 'Repair complete',
        saving: 'Saving…',
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

                    {/* CHOICE */}
                    {mode.step === 'choice' && (
                        <div className="flex flex-col gap-2 pb-2">
                            <button
                                onClick={handleScanLoaded}
                                disabled={!platform}
                                className="flex items-center gap-3 p-3 rounded-lg border border-border bg-muted/20 hover:bg-muted/40 transition-colors text-left disabled:opacity-40 disabled:cursor-not-allowed"
                            >
                                <svg className="w-4 h-4 text-primary shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                                </svg>
                                <div>
                                    <p className="text-[10px] font-black uppercase tracking-widest text-foreground">Loaded save</p>
                                    <p className="text-[9px] text-muted-foreground mt-0.5">
                                        {platform ? `Scan character slot ${charIndex + 1}` : 'No save loaded'}
                                    </p>
                                </div>
                            </button>
                            <button
                                onClick={handleScanExternal}
                                className="flex items-center gap-3 p-3 rounded-lg border border-border bg-muted/20 hover:bg-muted/40 transition-colors text-left"
                            >
                                <svg className="w-4 h-4 text-primary shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                                </svg>
                                <div>
                                    <p className="text-[10px] font-black uppercase tracking-widest text-foreground">External file</p>
                                    <p className="text-[9px] text-muted-foreground mt-0.5">Pick a .sl2 / .dat file to scan</p>
                                </div>
                            </button>
                        </div>
                    )}

                    {/* SCANNING / REPAIRING / SAVING */}
                    {(mode.step === 'scanning' || mode.step === 'repairing' || mode.step === 'saving') && (
                        <div className="flex flex-col items-center justify-center py-10 gap-3">
                            <svg className="w-8 h-8 text-primary animate-spin" fill="none" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                            </svg>
                            <p className="text-[10px] text-muted-foreground uppercase tracking-widest">
                                {mode.step === 'scanning' ? 'Scanning save file…' : mode.step === 'repairing' ? 'Applying repairs…' : 'Saving…'}
                            </p>
                        </div>
                    )}

                    {/* REPORT */}
                    {mode.step === 'report' && (
                        <div className="pb-2">
                            {!mode.report.canRepair ? (
                                <div className="flex flex-col items-center justify-center py-8 gap-3 text-center">
                                    <svg className="w-10 h-10 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                    <p className="text-[11px] font-black uppercase tracking-widest text-green-400">No repairable issues found</p>
                                    <p className="text-[9px] text-muted-foreground max-w-xs">The save file is clean. Any remaining observations are informational and do not require action.</p>
                                </div>
                            ) : (
                                <>
                                    {/* Summary badges */}
                                    <div className="flex flex-wrap gap-2 mb-3">
                                        {criticalCount > 0 && (
                                            <span className="px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-widest bg-red-500/10 border border-red-500/30 text-red-400">
                                                {criticalCount} critical
                                            </span>
                                        )}
                                        {warningCount > 0 && (
                                            <span className="px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-widest bg-yellow-500/10 border border-yellow-500/30 text-yellow-400">
                                                {warningCount} warning
                                            </span>
                                        )}
                                    </div>
                                    {/* Issue list — each slot */}
                                    {mode.report.slots.map(slot =>
                                        slot.issues.length > 0 ? (
                                            <div key={slot.slotIndex} className="mb-3">
                                                <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground mb-1.5">
                                                    Slot {slot.slotIndex + 1} — {slot.charName || '(unnamed)'}
                                                </p>
                                                <div className="space-y-1">
                                                    {slot.issues.filter(iss => {
                                                        const repairable = ['inventory','stats','dlc','gaitemdata','storage'];
                                                        return iss.severity !== 'info' && repairable.includes(iss.category);
                                                    }).map((iss, idx) => (
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
                                        ) : null
                                    )}
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
                {!footer && (
                    <div className="px-6 pb-5 pt-2 shrink-0 text-center">
                        <p className="text-[8px] uppercase tracking-widest text-muted-foreground/40">Press Esc to close</p>
                    </div>
                )}
            </div>
        </div>,
        document.body
    );
}
