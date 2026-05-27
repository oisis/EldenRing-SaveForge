// Presentational prompt shown when an add is blocked by duplicate acquisition
// indices. All state, the repair endpoint call, the retry of handleAdd and the
// decision to keep confirmModal mounted stay in DatabaseTab; this component only
// renders and reports the user's intent via callbacks.
interface RepairPromptProps {
    retrying: boolean;
    onRepairAndRetry: () => void;
    onCancel: () => void;
}

export function RepairPrompt({ retrying, onRepairAndRetry, onCancel }: RepairPromptProps) {
    return (
        <div className="fixed inset-0 z-[140] flex items-center justify-center bg-background/80 backdrop-blur-sm animate-in fade-in duration-300">
            <div className="bg-card p-8 rounded-2xl border-2 border-amber-500/40 flex flex-col space-y-5 max-w-md w-full mx-4 shadow-2xl shadow-amber-500/20 animate-in zoom-in-95 duration-300">
                <div className="flex items-center space-x-3">
                    <div className="w-10 h-10 rounded-full bg-amber-500/15 border border-amber-500/40 flex items-center justify-center">
                        <svg className="w-5 h-5 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M12 9v3m0 3h.01M4.93 19h14.14a2 2 0 001.74-3L13.74 4a2 2 0 00-3.48 0L3.19 16a2 2 0 001.74 3z" />
                        </svg>
                    </div>
                    <h3 className="text-sm font-black uppercase tracking-[0.15em] text-amber-500">Inventory index repair required</h3>
                </div>
                <p className="text-[11px] text-foreground leading-relaxed">
                    This save has duplicate acquisition indices before adding items. The editor blocked the add operation to avoid writing an inconsistent save.
                </p>
                <p className="text-[11px] text-muted-foreground leading-relaxed">
                    Repair will only renumber duplicate acquisition/sort indices. No items will be removed and no quantities will change. The fix is undoable from this character slot.
                </p>
                <div className="flex flex-col space-y-2">
                    <button
                        onClick={onRepairAndRetry}
                        disabled={retrying}
                        className="w-full px-4 py-2.5 bg-amber-500 text-white rounded-md text-[10px] font-black uppercase tracking-widest hover:brightness-110 active:scale-95 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {retrying ? 'Repairing…' : 'Repair & Retry'}
                    </button>
                    <button
                        onClick={onCancel}
                        disabled={retrying}
                        className="w-full px-4 py-2.5 bg-muted text-foreground rounded-md text-[10px] font-black uppercase tracking-widest hover:brightness-110 active:scale-95 transition-all disabled:opacity-50"
                    >
                        Cancel
                    </button>
                </div>
            </div>
        </div>
    );
}
