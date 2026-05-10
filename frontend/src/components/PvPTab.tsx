import {useState} from 'react';
import {PvPPreparationTab} from './PvPPreparationTab';
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

export function PvPTab({charIdx, platform, pvpOpts, onPvpOptsChange, onMutate}: PvPTabProps) {
    const [subTab, setSubTab] = useState<PvPSubTab>('presets');

    return (
        <div className="flex-1 flex flex-col min-h-0 gap-4">
            <div className="flex gap-1.5 p-1 bg-muted/30 rounded-lg border border-border/50 shrink-0 self-start">
                {(['presets', 'network'] as PvPSubTab[]).map(t => (
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
                <div className="flex-1 overflow-y-auto custom-scrollbar pr-2">
                    <PvPPreparationTab
                        charIdx={charIdx}
                        platform={platform}
                        pvpOpts={pvpOpts}
                        onPvpOptsChange={onPvpOptsChange}
                        onMutate={onMutate}
                    />
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
