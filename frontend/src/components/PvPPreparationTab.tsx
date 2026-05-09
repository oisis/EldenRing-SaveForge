import {useState} from 'react';
import toast from '../lib/toast';
import {ApplyPvPPreparation} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';

interface PvPPreparationTabProps {
    charIdx: number;
    platform?: string | null;
    onMutate?: () => void;
}

interface Module {
    key: keyof main.PvPPreparationOptions;
    label: string;
    tier: string;
    tierStyle: string;
    desc: string;
    defaultOn: boolean;
    disabled?: boolean;
    disabledNote?: string;
}

const MODULES: Module[] = [
    {
        key: 'matchmakingRegions',
        label: 'Matchmaking Regions',
        tier: 'Recommended · Tier 1',
        tierStyle: 'text-green-400',
        desc: 'Unlocks all known invasion matchmaking regions. Required for area-specific PvP eligibility.',
        defaultOn: true,
    },
    {
        key: 'colosseums',
        label: 'Colosseums',
        tier: 'Optional · Tier 1',
        tierStyle: 'text-blue-400',
        desc: 'Sets colosseum matchmaking and map flags for all three arenas. Physical gates may still need to be opened once in-game.',
        defaultOn: false,
    },
    {
        key: 'revealMap',
        label: 'Map Reveal',
        tier: 'QoL · Tier 0',
        tierStyle: 'text-muted-foreground',
        desc: 'Reveals all map tiles (base game + Shadow of the Erdtree DLC).',
        defaultOn: false,
    },
    {
        key: 'summoningPools',
        label: 'Summoning Pools',
        tier: 'Co-op/Summon · Tier 1',
        tierStyle: 'text-blue-400',
        desc: 'Activates all Martyr Effigy co-op summon pool flags. Bloody Finger invasion impact is unconfirmed.',
        defaultOn: false,
    },
    {
        key: 'sitesOfGrace',
        label: 'Sites of Grace',
        tier: 'QoL · Tier 0 · planned',
        tierStyle: 'text-muted-foreground/50',
        desc: 'Unlocks map marker and fast-travel layer for all Sites of Grace. Some graces may still play the activation animation.',
        defaultOn: false,
        disabled: true,
        disabledNote: 'Coming soon — broad QoL module, needs UX confirmation',
    },
];

const Chk = ({checked, onChange, disabled}: {checked: boolean; onChange: (v: boolean) => void; disabled?: boolean}) => (
    <div className="relative flex items-center justify-center flex-shrink-0">
        <input
            type="checkbox"
            checked={checked}
            disabled={disabled}
            onChange={e => onChange(e.target.checked)}
            className={`peer appearance-none w-3.5 h-3.5 rounded border border-border bg-background transition-all ${disabled ? 'opacity-30 cursor-not-allowed' : 'checked:bg-primary checked:border-primary cursor-pointer'}`}
        />
        {!disabled && (
            <svg className="absolute w-2 h-2 text-white pointer-events-none hidden peer-checked:block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3.5" d="M5 13l4 4L19 7" />
            </svg>
        )}
    </div>
);

export function PvPPreparationTab({charIdx, platform, onMutate}: PvPPreparationTabProps) {
    const [opts, setOpts] = useState<main.PvPPreparationOptions>(
        new main.PvPPreparationOptions({
            matchmakingRegions: true,
            colosseums: false,
            revealMap: false,
            summoningPools: false,
            sitesOfGrace: false,
        })
    );
    const [applying, setApplying] = useState(false);
    const [resultWarnings, setResultWarnings] = useState<string[]>([]);

    const handleToggle = (key: keyof main.PvPPreparationOptions, value: boolean) => {
        setOpts(prev => new main.PvPPreparationOptions({...prev, [key]: value}));
    };

    const handleApply = async () => {
        if (!platform) return;
        setApplying(true);
        setResultWarnings([]);
        try {
            const warnings = await ApplyPvPPreparation(charIdx, opts);
            setResultWarnings(warnings ?? []);
            toast.success('PvP preparation applied.');
            onMutate?.();
        } catch (e) {
            toast.error('PvP preparation failed: ' + e);
        } finally {
            setApplying(false);
        }
    };

    const anySelected = MODULES.some(m => !m.disabled && opts[m.key]);

    return (
        <div className="space-y-5 animate-in fade-in duration-300 max-w-2xl">
            <div>
                <h2 className="text-[10px] font-black uppercase tracking-[0.2em] text-foreground mb-1">PvP Preparation</h2>
                <p className="text-[10px] text-muted-foreground leading-relaxed">
                    Apply world-state modules to prepare a character for PvP. World tab remains available for granular edits.
                </p>
            </div>

            <div className="space-y-2">
                {MODULES.map(mod => (
                    <div
                        key={mod.key}
                        className={`flex items-start gap-3 px-3 py-2.5 rounded-lg border transition-all ${
                            mod.disabled
                                ? 'border-border/40 bg-muted/10 opacity-60'
                                : opts[mod.key]
                                ? 'border-primary/40 bg-primary/5'
                                : 'border-border bg-muted/10 hover:bg-muted/20'
                        }`}
                    >
                        <div className="mt-0.5">
                            <Chk
                                checked={!mod.disabled && opts[mod.key]}
                                onChange={v => handleToggle(mod.key, v)}
                                disabled={mod.disabled}
                            />
                        </div>
                        <div className="flex-1 min-w-0">
                            <div className="flex items-baseline gap-2 flex-wrap">
                                <span className={`text-[10px] font-black uppercase tracking-widest ${mod.disabled ? 'text-muted-foreground/50' : 'text-foreground'}`}>
                                    {mod.label}
                                </span>
                                <span className={`text-[9px] font-medium ${mod.tierStyle}`}>
                                    {mod.tier}
                                </span>
                            </div>
                            <p className="text-[9px] text-muted-foreground mt-0.5 leading-relaxed">
                                {mod.disabled && mod.disabledNote
                                    ? mod.disabledNote
                                    : mod.desc}
                            </p>
                        </div>
                    </div>
                ))}
            </div>

            <div className="flex items-center gap-3">
                <button
                    onClick={handleApply}
                    disabled={!platform || applying || !anySelected}
                    className="px-4 py-1.5 rounded-lg bg-primary text-primary-foreground text-[9px] font-black uppercase tracking-widest transition-all hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                    {applying ? 'Applying…' : 'Apply PvP Preparation'}
                </button>
                {!platform && (
                    <span className="text-[9px] text-muted-foreground">Load a save file first.</span>
                )}
            </div>

            {resultWarnings.length > 0 && (
                <div className="space-y-1.5 border border-border rounded-lg p-3">
                    <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground mb-2">Notes</p>
                    {resultWarnings.map((w, i) => (
                        <div key={i} className="flex items-start gap-2">
                            <span className="text-primary mt-0.5 text-[10px] flex-shrink-0">·</span>
                            <span className="text-[10px] text-foreground/80 leading-relaxed">{w}</span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
