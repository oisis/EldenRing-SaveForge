import { db } from '../../../wailsjs/go/models';

// Presentational modal that warns before adding ban-risk-flagged items. All
// state, the confirm flow and the save mutation stay in DatabaseTab — this
// component only renders and reports user intent via callbacks.
interface BanRiskWarningModalProps {
    // The items the user is about to add (the same array DatabaseTab gated on).
    items: db.ItemEntry[];
    ignoreBanRisk: boolean;
    onIgnoreChange: (checked: boolean) => void;
    onCancel: () => void;
    onConfirm: () => void;
}

export function BanRiskWarningModal({ items, ignoreBanRisk, onIgnoreChange, onCancel, onConfirm }: BanRiskWarningModalProps) {
    const banRiskItems = items.filter(i => i.flags?.includes('ban_risk'));
    return (
        <div className="fixed inset-0 z-[120] flex items-center justify-center bg-background/80 backdrop-blur-sm animate-in fade-in duration-300">
            <div className="bg-card p-8 rounded-2xl border-2 border-red-500/40 flex flex-col space-y-5 max-w-md w-full mx-4 shadow-2xl shadow-red-500/20 animate-in zoom-in-95 duration-300">
                {/* Header */}
                <div className="flex items-center space-x-3">
                    <div className="w-10 h-10 rounded-full bg-red-500/15 border border-red-500/40 flex items-center justify-center">
                        <svg className="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                        </svg>
                    </div>
                    <div>
                        <h3 className="text-sm font-black uppercase tracking-[0.15em] text-red-500">Ban Risk Warning</h3>
                        <p className="text-[9px] font-bold text-muted-foreground uppercase tracking-widest">Cut content / cheat-flagged item</p>
                    </div>
                </div>

                {/* Warning text */}
                <p className="text-[11px] text-foreground leading-relaxed">
                    {banRiskItems.length === 1 ? (
                        <>
                            <strong>{banRiskItems[0].name}</strong> is flagged as <strong>ban-risk</strong>.
                            Adding it to your save may trigger Easy Anti-Cheat detection if you go online.
                        </>
                    ) : (
                        <>
                            <strong>{banRiskItems.length}</strong> of the selected items are flagged as <strong>ban-risk</strong>.
                            Adding them to your save may trigger Easy Anti-Cheat detection if you go online.
                        </>
                    )}
                </p>

                {/* List of ban-risk items */}
                {banRiskItems.length > 1 && (
                    <div className="bg-red-500/5 border border-red-500/20 rounded-md p-3 max-h-32 overflow-y-auto custom-scrollbar">
                        <ul className="text-[10px] text-red-500/90 list-disc list-inside space-y-0.5">
                            {banRiskItems.map(i => <li key={i.id}>{i.name}</li>)}
                        </ul>
                    </div>
                )}

                {/* Ignore checkbox */}
                <label className="flex items-center justify-between p-2.5 rounded-md bg-muted/20 border border-border/50 cursor-pointer hover:bg-muted/30 transition-all">
                    <span className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">
                        Ignore all ban risk warnings
                    </span>
                    <input
                        type="checkbox"
                        checked={ignoreBanRisk}
                        onChange={e => onIgnoreChange(e.target.checked)}
                        className="w-3.5 h-3.5 rounded border-border text-red-500 focus:ring-red-500/20"
                    />
                </label>

                {/* Actions */}
                <div className="flex space-x-2">
                    <button
                        onClick={onCancel}
                        className="flex-1 px-4 py-2.5 bg-muted/30 text-muted-foreground rounded-md text-[10px] font-black uppercase tracking-widest border border-border hover:bg-muted/50 transition-all"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={onConfirm}
                        className="flex-1 px-4 py-2.5 bg-red-500 text-white rounded-md text-[10px] font-black uppercase tracking-widest hover:brightness-110 active:scale-95 transition-all"
                    >
                        Add Anyway
                    </button>
                </div>
            </div>
        </div>
    );
}
