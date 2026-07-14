import {useCallback, useEffect, useState} from 'react';
import {AnalyzeGaItemRepack, ExecuteGaItemRepack} from '../../wailsjs/go/main/App';
import type {main} from '../../wailsjs/go/models';

type Analysis = main.GaItemRepackAnalysis;
type ExecutionResult = main.GaItemRepackExecutionResult;
type Capacity = main.GaItemCapacity;

interface GaItemRepackModalProps {
    charIndex: number;
    characterName?: string;
    // Central App write path (audit + WriteSave). The modal must never call
    // WriteSave directly.
    onWriteSave: () => Promise<void> | void;
    // Refresh App state after a successful in-memory repack (same as any mutation).
    onRefresh: () => void;
    // Existing App close-without-saving path, used only for the critical state.
    onCloseSaveWithoutSaving: () => Promise<void> | void;
    onClose: () => void;
}

type Stage = 'analyzing' | 'dry-run' | 'confirm' | 'optimizing' | 'result';

const STABLE_NOTE =
    'Stable compaction; non-empty GaItem records keep their relative order and their handles.';
const SAVEFORGE_CAVEAT =
    'SaveForge validation is not a substitute for loading the save in-game.';
const NO_CHANGES = 'No changes have been made.';

// Capacity metric rows are rendered straight from the API — the frontend never
// derives usable capacity, recovery or eligibility itself.
function CapacityTable({before, after, recovered, afterLabel = 'After (projected)'}: {before: Capacity; after?: Capacity; recovered: number; afterLabel?: string}) {
    const rows: [string, number, number | undefined][] = [
        ['Physical empty GaItem records', before.physicalEmpty, after?.physicalEmpty],
        ['Allocator cursor room', before.cursorRoom, after?.cursorRoom],
        ['Usable GaItem capacity', before.usable, after?.usable],
    ];
    return (
        <div className="rounded border border-border/50 overflow-hidden">
            <table className="w-full text-[10px] font-mono">
                <thead>
                    <tr className="bg-muted/30 text-muted-foreground">
                        <th className="text-left px-2.5 py-1.5 font-black uppercase tracking-widest text-[8px]">Metric</th>
                        <th className="text-right px-2.5 py-1.5 font-black uppercase tracking-widest text-[8px]">Before</th>
                        <th className="text-right px-2.5 py-1.5 font-black uppercase tracking-widest text-[8px]">{afterLabel}</th>
                    </tr>
                </thead>
                <tbody>
                    {rows.map(([label, b, a]) => (
                        <tr key={label} className="border-t border-border/40">
                            <td className="px-2.5 py-1.5 text-foreground">{label}</td>
                            <td className="px-2.5 py-1.5 text-right text-muted-foreground">{b}</td>
                            <td className="px-2.5 py-1.5 text-right text-foreground">{a ?? '—'}</td>
                        </tr>
                    ))}
                    <tr className="border-t border-border/40 bg-muted/10">
                        <td className="px-2.5 py-1.5 font-bold text-foreground">Capacity recovered</td>
                        <td className="px-2.5 py-1.5" />
                        <td className="px-2.5 py-1.5 text-right font-black text-primary">+{recovered}</td>
                    </tr>
                </tbody>
            </table>
        </div>
    );
}

export function GaItemRepackModal({
    charIndex, characterName,
    onWriteSave, onRefresh, onCloseSaveWithoutSaving, onClose,
}: GaItemRepackModalProps) {
    const [stage, setStage] = useState<Stage>('analyzing');
    const [analysis, setAnalysis] = useState<Analysis | null>(null);
    const [analysisFailed, setAnalysisFailed] = useState(false);
    const [result, setResult] = useState<ExecutionResult | null>(null);
    const [executing, setExecuting] = useState(false);
    const [writing, setWriting] = useState(false);
    const [closing, setClosing] = useState(false);
    const [criticalConfirm, setCriticalConfirm] = useState(false);
    const [closeSaveError, setCloseSaveError] = useState<string | null>(null);
    const [copied, setCopied] = useState(false);

    const runAnalysis = useCallback(() => {
        setStage('analyzing');
        setAnalysis(null);
        setAnalysisFailed(false);
        setResult(null);
        setCriticalConfirm(false);
        AnalyzeGaItemRepack(charIndex)
            .then(report => { setAnalysis(report); setStage('dry-run'); })
            .catch(() => { setAnalysisFailed(true); setStage('dry-run'); });
    }, [charIndex]);

    useEffect(() => { runAnalysis(); }, [runAnalysis]);

    const isCritical = stage === 'result' && result?.outcome === 'rollback_failed';
    const locked = stage === 'optimizing' || isCritical;

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !locked) onClose(); };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [locked, onClose]);

    const canOptimize =
        !!analysis &&
        analysis.outcome === 'ready' &&
        !!analysis.analysisToken &&
        !!analysis.projectedAfter &&
        analysis.recovered > 0;

    const execute = () => {
        if (executing || !canOptimize || !analysis) return; // dedupe + gate
        setExecuting(true);
        setStage('optimizing');
        ExecuteGaItemRepack({characterIndex: charIndex, analysisToken: analysis.analysisToken!})
            .then(res => {
                setResult(res);
                setStage('result');
                if (res.outcome === 'success') onRefresh();
            })
            .catch(err => {
                // A rejected execute call is treated as a could_not_start: nothing
                // was mutated in memory or on disk.
                setResult({
                    outcome: 'could_not_start',
                    characterIndex: charIndex,
                    before: analysis.before,
                    recovered: 0,
                    failure: {stage: 'app', code: 'execute_rejected', message: String(err)},
                } as ExecutionResult);
                setStage('result');
            })
            .finally(() => setExecuting(false));
    };

    const handleWriteSave = async () => {
        setWriting(true);
        try { await onWriteSave(); onClose(); }
        finally { setWriting(false); }
    };

    const handleCloseSave = async () => {
        setClosing(true);
        setCloseSaveError(null);
        try { await onCloseSaveWithoutSaving(); }
        // Keep the modal in its blocking critical state and let the user retry;
        // never let the rejection escape as an unhandled promise.
        catch (err) { setCloseSaveError(`Could not close the save without saving: ${String(err)}`); }
        finally { setClosing(false); }
    };

    const copyDiagnostics = async () => {
        if (!result) return;
        const details = [
            `outcome: ${result.outcome}`,
            result.failure && `failure: ${result.failure.stage}/${result.failure.code}: ${result.failure.message}`,
            result.rollback && `rollback: attempted=${result.rollback.attempted} complete=${result.rollback.complete} mode=${result.rollback.mode}`,
            result.rollback?.failure && `rollback failure: ${result.rollback.failure.stage}/${result.rollback.failure.code}: ${result.rollback.failure.message}`,
        ].filter(Boolean).join('\n');
        if (!navigator.clipboard?.writeText) { setCopied(false); return; }
        try { await navigator.clipboard.writeText(details); setCopied(true); }
        catch { setCopied(false); }
    };

    const slotLabel = `Slot ${charIndex + 1}${characterName ? ` · ${characterName}` : ''}`;

    const btn = 'px-3 py-1.5 rounded text-[9px] font-black uppercase tracking-widest transition-all disabled:opacity-50';
    const btnPrimary = `${btn} bg-primary text-primary-foreground shadow-sm hover:brightness-110 active:scale-95`;
    const btnSecondary = `${btn} bg-muted/30 text-foreground border border-border hover:bg-muted/50`;
    const btnDanger = `${btn} bg-red-600 text-white shadow-sm hover:brightness-110 active:scale-95`;
    const caveat = 'text-[9px] text-muted-foreground leading-relaxed';

    let header = 'GaItem allocation analysis';
    if (stage === 'confirm') header = 'Optimize GaItem allocation?';
    else if (stage === 'result' && result) {
        header = ({
            success: 'GaItem allocation optimized',
            rolled_back: 'GaItem allocation failed — changes rolled back',
            rollback_failed: 'GaItem allocation failed — do not save this session',
            could_not_start: 'GaItem allocation could not start',
        } as Record<string, string>)[result.outcome] ?? header;
    }

    return (
        <div
            className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 p-4"
            onMouseDown={e => { if (e.target === e.currentTarget && !locked) onClose(); }}
        >
            <div className="card p-6 w-full max-w-lg space-y-4 max-h-[90vh] overflow-y-auto custom-scrollbar">
                <div className="flex items-start justify-between gap-3">
                    <div className="space-y-1">
                        <h3 className="text-[12px] font-black uppercase tracking-wider text-foreground">{header}</h3>
                        <p className="text-[9px] font-mono text-muted-foreground">{slotLabel}</p>
                    </div>
                    {!locked && (
                        <button onClick={onClose} aria-label="Close dialog"
                            className="text-muted-foreground hover:text-foreground transition-colors">
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    )}
                </div>

                {/* Analyzing */}
                {stage === 'analyzing' && (
                    <div className="flex items-center gap-3 py-6 justify-center">
                        <div className="w-4 h-4 border-2 border-muted-foreground/30 border-t-primary rounded-full animate-spin" />
                        <span className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">Analyzing GaItem allocation…</span>
                    </div>
                )}

                {/* Optimizing */}
                {stage === 'optimizing' && (
                    <div className="flex items-center gap-3 py-6 justify-center">
                        <div className="w-4 h-4 border-2 border-muted-foreground/30 border-t-primary rounded-full animate-spin" />
                        <span className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">Optimizing GaItem allocation…</span>
                    </div>
                )}

                {/* Dry-run */}
                {stage === 'dry-run' && (
                    <div className="space-y-3">
                        {analysisFailed && (
                            <div className="space-y-2">
                                <p className="text-[11px] font-bold text-red-500">Could not analyze GaItem allocation</p>
                                <p className={caveat}>The slot and the save file were not changed.</p>
                            </div>
                        )}

                        {!analysisFailed && analysis?.outcome === 'ready' && analysis.projectedAfter && (
                            <div className="space-y-3">
                                <p className="text-[11px] font-bold text-primary">Ready to optimize</p>
                                <CapacityTable before={analysis.before} after={analysis.projectedAfter} recovered={analysis.recovered} />
                                <p className={caveat}>
                                    Stable compaction repositions the {analysis.nonEmptyRecords} non-empty GaItem records; their handles and data are unchanged.
                                </p>
                            </div>
                        )}

                        {!analysisFailed && analysis?.outcome === 'no_op' && (
                            <div className="space-y-3">
                                <p className="text-[11px] font-bold text-foreground">No capacity to recover</p>
                                <CapacityTable before={analysis.before} after={analysis.projectedAfter} recovered={0} />
                                <p className={caveat}>This slot already has full usable GaItem capacity. No changes are needed.</p>
                            </div>
                        )}

                        {!analysisFailed && analysis?.outcome === 'refusal' && (
                            <div className="space-y-2">
                                <p className="text-[11px] font-bold text-red-500">Optimization unavailable</p>
                                <ul className="space-y-1.5">
                                    {analysis.blockers.map((b, i) => (
                                        <li key={i} className="rounded border border-red-500/30 bg-red-500/5 px-2.5 py-1.5">
                                            <p className="text-[8px] font-mono uppercase tracking-widest text-red-400">{b.code}</p>
                                            <p className="text-[10px] text-foreground">{b.message}</p>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                        )}

                        {!analysisFailed && analysis?.outcome === 'unavailable' && (
                            <div className="space-y-2">
                                <p className="text-[11px] font-bold text-amber-500">Optimization unavailable</p>
                                <p className="text-[10px] text-foreground">{analysis.failure?.message}</p>
                            </div>
                        )}

                        <div className="space-y-1.5 pt-1">
                            <p className={caveat}>{NO_CHANGES}</p>
                            <p className={caveat}>Stable compaction only changes the positions of non-empty records — never their handles or data.</p>
                            <p className={caveat}>{SAVEFORGE_CAVEAT}</p>
                        </div>

                        <div className="flex justify-end gap-2 pt-1">
                            {canOptimize && (
                                <button onClick={() => setStage('confirm')} className={btnPrimary}>Continue</button>
                            )}
                            <button onClick={onClose} className={btnSecondary}>Close</button>
                        </div>
                    </div>
                )}

                {/* Confirm */}
                {stage === 'confirm' && analysis && analysis.projectedAfter && (
                    <div className="space-y-3">
                        <div className="rounded border border-border/50 bg-muted/10 px-3 py-2.5 space-y-1">
                            <p className="text-[10px] font-mono text-foreground">
                                Usable capacity: <span className="text-muted-foreground">{analysis.before.usable}</span> → <span className="font-black text-primary">{analysis.projectedAfter.usable}</span>
                            </p>
                            <p className="text-[10px] font-mono font-black text-primary">+{analysis.recovered}</p>
                        </div>
                        <p className={caveat}>{STABLE_NOTE}</p>
                        <p className={caveat}>The slot is rebuilt, reparsed and validated by SaveForge before it replaces the active slot.</p>
                        <p className={caveat}>
                            This changes memory only. No file is written and no backup is created now; persisting the change requires a later Write Save.
                        </p>
                        <p className={caveat}>{SAVEFORGE_CAVEAT}</p>
                        <div className="flex justify-end gap-2 pt-1">
                            <button onClick={() => setStage('dry-run')} className={btnSecondary}>Cancel</button>
                            <button onClick={execute} disabled={executing || !canOptimize} className={btnPrimary}>Optimize allocation</button>
                        </div>
                    </div>
                )}

                {/* Result: success */}
                {stage === 'result' && result?.outcome === 'success' && (
                    <div className="space-y-3">
                        <CapacityTable before={result.before} after={result.after} recovered={result.recovered} afterLabel="After" />
                        <p className={caveat}>Rebuild, reparse, and SaveForge validation completed successfully.</p>
                        <p className="text-[10px] font-bold text-amber-500">The active slot has changed in memory. No file has been written yet.</p>
                        <div className="flex justify-end gap-2 pt-1">
                            <button onClick={onClose} className={btnSecondary}>Close</button>
                            <button onClick={handleWriteSave} disabled={writing} className={btnPrimary}>{writing ? 'Writing…' : 'Write Save'}</button>
                        </div>
                    </div>
                )}

                {/* Result: rolled_back */}
                {stage === 'result' && result?.outcome === 'rolled_back' && (
                    <div className="space-y-3">
                        <p className="text-[10px] font-mono text-muted-foreground">
                            Stage: {result.failure?.stage} · {result.failure?.code}
                        </p>
                        <p className="text-[10px] text-foreground">{result.failure?.message}</p>
                        <p className={caveat}>The active slot was restored to its previous state. No file and no backup were changed.</p>
                        <div className="flex justify-end pt-1">
                            <button onClick={onClose} className={btnSecondary}>Close</button>
                        </div>
                    </div>
                )}

                {/* Result: could_not_start */}
                {stage === 'result' && result?.outcome === 'could_not_start' && (
                    <div className="space-y-3">
                        <p className="text-[10px] text-foreground">{result.failure?.message}</p>
                        <p className={caveat}>Nothing changed in memory or on disk.</p>
                        <div className="flex justify-end gap-2 pt-1">
                            <button onClick={runAnalysis} className={btnPrimary}>Run analysis again</button>
                            <button onClick={onClose} className={btnSecondary}>Close</button>
                        </div>
                    </div>
                )}

                {/* Result: rollback_failed (critical) */}
                {stage === 'result' && result?.outcome === 'rollback_failed' && (
                    <div className="space-y-3">
                        <p className="text-[10px] font-bold text-red-500">
                            No file on disk has been changed, but the in-memory state may be inconsistent. Do not save this session.
                        </p>
                        <p className="text-[10px] font-mono text-muted-foreground">
                            Stage: {result.failure?.stage} · {result.failure?.code}
                        </p>
                        <p className="text-[10px] text-foreground">{result.failure?.message}</p>
                        <div className="flex flex-wrap justify-end gap-2 pt-1">
                            <button onClick={copyDiagnostics} className={btnSecondary}>
                                {copied ? 'Copied' : 'Copy diagnostic details'}
                            </button>
                            {!criticalConfirm ? (
                                <button onClick={() => setCriticalConfirm(true)} className={btnDanger}>Close save without saving</button>
                            ) : (
                                <button onClick={handleCloseSave} disabled={closing} className={btnDanger}>
                                    {closing ? 'Closing…' : 'Confirm — close without saving'}
                                </button>
                            )}
                        </div>
                        {criticalConfirm && (
                            <p className={`${caveat} text-right`}>This discards the loaded save from memory without writing any file.</p>
                        )}
                        {closeSaveError && (
                            <p className="text-[10px] font-bold text-red-500 text-right">{closeSaveError}</p>
                        )}
                    </div>
                )}
            </div>
        </div>
    );
}
