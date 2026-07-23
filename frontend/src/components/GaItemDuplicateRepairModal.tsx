import {useCallback, useEffect, useState} from 'react';
import {AnalyzeGaItemDuplicate, ExecuteGaItemDuplicateRepair} from '../../wailsjs/go/main/App';
import type {main} from '../../wailsjs/go/models';

type Analysis = main.GaItemDuplicateAnalysis;
type ExecutionResult = main.GaItemDuplicateExecutionResult;
type Candidate = main.GaItemDuplicateCandidate;

interface GaItemDuplicateRepairModalProps {
    charIndex: number;
    characterName?: string;
    // The physical GaItem handle to deduplicate. Supplied structurally by the
    // Diagnostics issue key, never parsed from a message string.
    handle: number;
    // Refresh App state after a successful in-memory dedup (same as any mutation).
    onRefresh: () => void;
    onClose: () => void;
}

type Stage = 'analyzing' | 'ready' | 'executing' | 'result';

const SAVEFORGE_CAVEAT =
    'SaveForge validation is not a substitute for loading the save in-game.';

function formatHex(value: number): string {
    return '0x' + (value >>> 0).toString(16).toUpperCase().padStart(8, '0');
}

function candidateLabel(c: Candidate): string {
    if (c.unknown || !c.name) return `Unknown item · ${formatHex(c.itemId)}`;
    const name = c.currentUpgrade && c.currentUpgrade > 0 ? `${c.name} +${c.currentUpgrade}` : c.name;
    return c.infusionName ? `${name} · ${c.infusionName}` : name;
}

export function GaItemDuplicateRepairModal({
    charIndex, characterName, handle, onRefresh, onClose,
}: GaItemDuplicateRepairModalProps) {
    const [stage, setStage] = useState<Stage>('analyzing');
    const [analysis, setAnalysis] = useState<Analysis | null>(null);
    const [analysisFailed, setAnalysisFailed] = useState(false);
    const [result, setResult] = useState<ExecutionResult | null>(null);
    const [keepIndex, setKeepIndex] = useState<number | null>(null);
    const [executing, setExecuting] = useState(false);

    const runAnalysis = useCallback(() => {
        setStage('analyzing');
        setAnalysis(null);
        setAnalysisFailed(false);
        setResult(null);
        setKeepIndex(null);
        AnalyzeGaItemDuplicate(charIndex, handle)
            .then(report => {
                setAnalysis(report);
                setStage(report.outcome === 'ready' ? 'ready' : 'result');
            })
            .catch(() => { setAnalysisFailed(true); setStage('result'); });
    }, [charIndex, handle]);

    useEffect(() => { runAnalysis(); }, [runAnalysis]);

    const isCritical = stage === 'result' && result?.outcome === 'rollback_failed';
    const locked = stage === 'executing' || isCritical;

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !locked) onClose(); };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [locked, onClose]);

    const candidates: Candidate[] = analysis?.candidates ?? [];
    const canExecute =
        stage === 'ready' &&
        !!analysis?.analysisToken &&
        keepIndex !== null &&
        candidates.some(c => c.index === keepIndex);

    const execute = () => {
        if (executing || !canExecute || !analysis || keepIndex === null) return; // dedupe + gate
        setExecuting(true);
        setStage('executing');
        ExecuteGaItemDuplicateRepair({
            characterIndex: charIndex,
            handle,
            keepIndex,
            analysisToken: analysis.analysisToken!,
        })
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
                    handle,
                    keptIndex: keepIndex,
                    removedIndex: -1,
                    failure: {stage: 'app', code: 'execute_rejected', message: String(err)},
                } as ExecutionResult);
                setStage('result');
            })
            .finally(() => setExecuting(false));
    };

    const slotLabel = `Slot ${charIndex + 1}${characterName ? ` · ${characterName}` : ''} · handle ${formatHex(handle)}`;

    const btn = 'px-3 py-1.5 rounded text-[9px] font-black uppercase tracking-widest transition-all disabled:opacity-50';
    const btnPrimary = `${btn} bg-primary text-primary-foreground shadow-sm hover:brightness-110 active:scale-95`;
    const btnSecondary = `${btn} bg-muted/30 text-foreground border border-border hover:bg-muted/50`;
    const caveat = 'text-[9px] text-muted-foreground leading-relaxed';

    let header = 'Resolve duplicate GaItem';
    if (stage === 'result' && result) {
        header = ({
            success: 'Duplicate GaItem resolved',
            rolled_back: 'Duplicate repair failed — changes rolled back',
            rollback_failed: 'Duplicate repair failed — do not save this session',
            could_not_start: 'Duplicate repair could not start',
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
                        <span className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">Analyzing duplicate GaItem…</span>
                    </div>
                )}

                {/* Executing */}
                {stage === 'executing' && (
                    <div className="flex items-center gap-3 py-6 justify-center">
                        <div className="w-4 h-4 border-2 border-muted-foreground/30 border-t-primary rounded-full animate-spin" />
                        <span className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">Removing duplicate GaItem…</span>
                    </div>
                )}

                {/* Ready — choose which physical record to keep */}
                {stage === 'ready' && analysis && (
                    <div className="space-y-3">
                        <p className={caveat}>
                            Two physical GaItem records share this handle. Choose which one to keep; the other physical record is removed.
                        </p>
                        <div className="grid grid-cols-1 gap-2">
                            {candidates.map(c => {
                                const selected = keepIndex === c.index;
                                return (
                                    <button
                                        key={c.index}
                                        type="button"
                                        aria-pressed={selected}
                                        onClick={() => setKeepIndex(c.index)}
                                        className={`text-left rounded border px-3 py-2.5 transition-all ${
                                            selected
                                                ? 'border-primary bg-primary/10'
                                                : 'border-border/50 bg-muted/10 hover:border-border'
                                        }`}
                                    >
                                        <p className="text-[11px] font-bold text-foreground">{candidateLabel(c)}</p>
                                        <p className="text-[9px] font-mono text-muted-foreground mt-0.5">Physical index {c.index}</p>
                                    </button>
                                );
                            })}
                        </div>
                        <div className="space-y-1.5 pt-1">
                            <p className={caveat}>The unselected physical record is removed.</p>
                            <p className={caveat}>This changes only the in-memory slot. No file is written.</p>
                            <p className={caveat}>{SAVEFORGE_CAVEAT}</p>
                        </div>
                        <div className="flex justify-end gap-2 pt-1">
                            <button onClick={onClose} className={btnSecondary}>Cancel</button>
                            <button onClick={execute} disabled={!canExecute} className={btnPrimary}>Remove duplicate</button>
                        </div>
                    </div>
                )}

                {/* Result: analysis rejected */}
                {stage === 'result' && analysisFailed && (
                    <div className="space-y-3">
                        <p className="text-[11px] font-bold text-red-500">Could not analyze duplicate GaItem</p>
                        <p className={caveat}>The slot and the save file were not changed.</p>
                        <div className="flex justify-end pt-1">
                            <button onClick={onClose} className={btnSecondary}>Close</button>
                        </div>
                    </div>
                )}

                {/* Result: refusal (Repairable=false) */}
                {stage === 'result' && !analysisFailed && analysis?.outcome === 'refusal' && (
                    <div className="space-y-3">
                        <p className="text-[11px] font-bold text-red-500">Duplicate repair unavailable</p>
                        <div className="rounded border border-red-500/30 bg-red-500/5 px-2.5 py-1.5">
                            {analysis.refusalCode && (
                                <p className="text-[8px] font-mono uppercase tracking-widest text-red-400">{analysis.refusalCode}</p>
                            )}
                            <p className="text-[10px] text-foreground">{analysis.refusalMessage || 'The duplicate cannot be safely repaired.'}</p>
                        </div>
                        <div className="flex justify-end pt-1">
                            <button onClick={onClose} className={btnSecondary}>Close</button>
                        </div>
                    </div>
                )}

                {/* Result: unavailable (workspace active etc.) */}
                {stage === 'result' && !analysisFailed && analysis?.outcome === 'unavailable' && (
                    <div className="space-y-3">
                        <p className="text-[11px] font-bold text-amber-500">Duplicate repair unavailable</p>
                        <p className="text-[10px] text-foreground">{analysis.failure?.message}</p>
                        <div className="flex justify-end pt-1">
                            <button onClick={onClose} className={btnSecondary}>Close</button>
                        </div>
                    </div>
                )}

                {/* Result: success */}
                {stage === 'result' && result?.outcome === 'success' && (
                    <div className="space-y-3">
                        <p className="text-[10px] text-foreground">
                            Kept physical index {result.keptIndex}; removed physical index {result.removedIndex}.
                        </p>
                        <p className="text-[10px] font-bold text-amber-500">The active slot has changed in memory. No file has been written yet.</p>
                        <div className="flex justify-end pt-1">
                            <button onClick={onClose} className={btnPrimary}>Close</button>
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
                        <div className="flex justify-end pt-1">
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
                    </div>
                )}
            </div>
        </div>
    );
}
