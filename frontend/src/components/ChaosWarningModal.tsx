import {useState} from 'react';

interface ChaosWarningModalProps {
    // onConfirm receives whether the user wants an autobackup before enabling.
    onConfirm: (autoBackup: boolean) => void;
    onCancel: () => void;
}

// Shown when the user enables Chaos Mode. Chaos edits raise item counts to the
// game's technical caps and reveal risk-flagged items; changes are written
// in-place and are irreversible — the only recovery is restoring a backup.
export function ChaosWarningModal({onConfirm, onCancel}: ChaosWarningModalProps) {
    const [autoBackup, setAutoBackup] = useState(true);

    return (
        <div className="fixed inset-0 z-[120] flex items-center justify-center bg-background/80 backdrop-blur-sm animate-in fade-in duration-200">
            <div className="bg-card p-8 rounded-2xl border border-red-500/40 flex flex-col space-y-5 max-w-md w-full mx-4 shadow-2xl shadow-red-500/20 animate-in zoom-in-95 duration-200">
                <div className="flex items-center space-x-3">
                    <span className="text-2xl">⚠</span>
                    <h2 className="text-sm font-black uppercase tracking-widest text-red-500">Enable Chaos Mode</h2>
                </div>

                <div className="space-y-3 text-[11px] leading-relaxed text-muted-foreground">
                    <p>
                        <strong className="text-red-500/90">Changes are irreversible.</strong> Chaos Mode raises item
                        counts to the game's technical caps and reveals risk-flagged
                        (cut / ban-risk) items. Edits are written in place — the only way
                        back is restoring a backup.
                    </p>
                    <p>
                        <strong className="text-red-500/90">Using Chaos Mode online is practically a guaranteed ban</strong>,
                        especially when adding items in bulk. Offline / experimental saves only.
                    </p>
                </div>

                <label className="flex items-center space-x-2.5 cursor-pointer select-none">
                    <input
                        type="checkbox"
                        checked={autoBackup}
                        onChange={e => setAutoBackup(e.target.checked)}
                        className="w-4 h-4 rounded border-border text-red-500 focus:ring-red-500/20"
                    />
                    <span className="text-[11px] font-bold text-foreground/80">Create a backup of the current save first</span>
                </label>

                <div className="flex justify-end space-x-3 pt-1">
                    <button
                        onClick={onCancel}
                        className="px-4 py-2 rounded text-[11px] font-black uppercase tracking-widest text-muted-foreground border border-border/50 hover:bg-muted/30 transition-all"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={() => onConfirm(autoBackup)}
                        className="px-4 py-2 rounded text-[11px] font-black uppercase tracking-widest text-white bg-red-500 hover:bg-red-600 transition-all"
                    >
                        OK
                    </button>
                </div>
            </div>
        </div>
    );
}
