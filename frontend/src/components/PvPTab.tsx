import {useState} from 'react';
import {NetworkTab} from './NetworkTab';
import type {PvPOptions} from '../App';

type PvPSubTab = 'presets' | 'network';

interface PvPTabProps {
    charIdx: number;
    platform: string | null;
    pvpOpts: PvPOptions;
    onPvpOptsChange: (opts: PvPOptions) => void;
    onMutate?: () => void;
}

export function PvPTab({charIdx: _charIdx, platform, pvpOpts: _pvpOpts, onPvpOptsChange: _onPvpOptsChange, onMutate}: PvPTabProps) {
    const [subTab, setSubTab] = useState<PvPSubTab>('network');

    return (
        <div className="flex-1 flex flex-col min-h-0 gap-4">
            <div className="flex gap-1.5 p-1 bg-muted/30 rounded-lg border border-border/50 shrink-0 self-start">
                {(['network', 'presets'] as PvPSubTab[]).map(t => (
                    <button key={t} onClick={() => setSubTab(t)}
                        className={`px-4 py-1.5 rounded-md text-[10px] font-black uppercase tracking-wider transition-all ${
                            subTab === t
                                ? 'bg-card shadow-sm border border-border/80 text-foreground'
                                : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                        }`}>
                        {t}
                    </button>
                ))}
            </div>

            {subTab === 'presets' && (
                <div className="flex-1 flex flex-col items-center justify-center gap-3 text-center px-6">
                    <div className="w-10 h-10 rounded-full bg-muted/30 border border-border/50 flex items-center justify-center">
                        <svg className="w-5 h-5 text-muted-foreground/40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                        </svg>
                    </div>
                    <div>
                        <p className="text-[11px] font-black uppercase tracking-[0.15em] text-foreground/60">Coming Soon</p>
                        <p className="text-[10px] text-muted-foreground mt-1 max-w-xs leading-relaxed">
                            PvP preparation presets, world-state profiles and quick-apply modules are in the works.
                        </p>
                    </div>
                </div>
            )}
            {subTab === 'network' && (
                <div className="flex-1 overflow-y-auto custom-scrollbar pr-2">
                    <NetworkTab platform={platform} />
                </div>
            )}
        </div>
    );
}
