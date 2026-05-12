import {useState} from 'react';
import {NetworkTab} from './NetworkTab';
import {PresetsTab} from './PresetsTab';
import type {PvPOptions} from '../App';

type PvPSubTab = 'presets' | 'network';

interface PvPTabProps {
    charIdx: number;
    platform: string | null;
    pvpOpts: PvPOptions;
    onPvpOptsChange: (opts: PvPOptions) => void;
    onMutate?: () => void;
}

export function PvPTab({charIdx, platform, pvpOpts: _pvpOpts, onPvpOptsChange: _onPvpOptsChange, onMutate}: PvPTabProps) {
    const [subTab, setSubTab] = useState<PvPSubTab>('network');

    return (
        <div className="flex-1 flex flex-col min-h-0 gap-4">
            <div className="flex gap-1.5 p-1 bg-muted/30 rounded-lg border border-border/50 shrink-0 self-start">
                {(['network', 'presets'] as PvPSubTab[]).map(t => (
                    <button key={t} onClick={() => setSubTab(t)}
                        className={`px-4 py-1.5 rounded-md text-[10px] font-black uppercase tracking-wider transition-all ${
                            subTab === t
                                ? 'bg-green-700/80 shadow-sm text-white'
                                : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                        }`}>
                        {t}
                    </button>
                ))}
            </div>

            {subTab === 'presets' && (
                <PresetsTab charIdx={charIdx} />
            )}
            {subTab === 'network' && (
                <div className="flex-1 overflow-y-auto custom-scrollbar pr-2">
                    <NetworkTab platform={platform} />
                </div>
            )}
        </div>
    );
}
