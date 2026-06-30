import {useState} from 'react';
import toast from '../lib/toast';
import {GetNetworkPreset, ResetNetworkParams, SetNetworkParams} from '../../wailsjs/go/main/App';
import {core} from '../../wailsjs/go/models';
import {AccordionSection} from './AccordionSection';

interface NetworkSpeedPanelProps {
    platform: string | null;
    onMutate?: () => void;
}

type NetworkPresetKey =
    | 'vanilla'
    | 'light-invasions'
    | 'fast-invasions'
    | 'fast-summons'
    | 'fast-blue'
    | 'aggressive-host';

interface PresetDef {
    key: NetworkPresetKey;
    group: string;
    label: string;
    risk: string;
    riskStyle: string;
    desc: string;
    values: string[];
    highlight?: string;
}

const PRESETS: PresetDef[] = [
    {
        key: 'light-invasions',
        group: 'Reds / Invader',
        label: 'Light Reds',
        risk: 'Low risk',
        riskStyle: 'text-green-400',
        desc: 'Moderate faster retry for red invasion search.',
        values: ['Targets: 8', 'Retry interval: 10s', 'Timeout: 8s', 'Area count: 5'],
    },
    {
        key: 'fast-invasions',
        group: 'Reds / Invader',
        label: 'Fast Reds',
        risk: 'Aggressive',
        riskStyle: 'text-orange-400',
        desc: 'Aggressive retry cycle for red invasion attempts.',
        values: ['Targets: 10', 'Retry interval: 4s', 'Timeout: 4s', 'Area count: 5'],
        highlight: 'Highest risk red invasion preset.',
    },
    {
        key: 'fast-summons',
        group: 'Summons / Co-op Signs',
        label: 'Fast Summons',
        risk: 'Experimental',
        riskStyle: 'text-blue-400',
        desc: 'Faster summon sign refresh and larger sign fetch limits.',
        values: ['Sign download: 10s', 'Sign update: 15s', 'Reload interval: 15s', 'Total signs: 40', 'Cells: 20', 'Get max: 64'],
    },
    {
        key: 'fast-blue',
        group: 'Blue / Hunter',
        label: 'Fast Blue',
        risk: 'Experimental',
        riskStyle: 'text-blue-400',
        desc: 'Faster Blue Cipher / hunter matchmaking search.',
        values: ['Visit cooldown: 5s', 'Blue summon count: 4', 'Visit list count: 15', 'Blue search: 10–30s', 'All-area rate: 75%'],
    },
    {
        key: 'aggressive-host',
        group: "Host / Taunter's Tongue",
        label: 'Aggressive Host',
        risk: 'Experimental',
        riskStyle: 'text-blue-400',
        desc: "Faster visitor list refresh for hosts / Taunter's Tongue style play.",
        values: ['Visitor list max: 20', 'Visitor timeout: 60s', 'Visitor download: 10s'],
    },
];

const GROUPS = Array.from(new Set(PRESETS.map(p => p.group)));

export function NetworkSpeedPanel({platform, onMutate}: NetworkSpeedPanelProps) {
    const [applying, setApplying] = useState(false);
    const [activePreset, setActivePreset] = useState<NetworkPresetKey | null>(null);

    const handleVanilla = async () => {
        if (!platform || applying) return;
        setApplying(true);
        try {
            await ResetNetworkParams();
            toast.success('"Vanilla" applied. Load character → Exit to menu → Load again to activate.');
            setActivePreset(null);
            onMutate?.();
        } catch (e) {
            toast.error(String(e));
        } finally {
            setApplying(false);
        }
    };

    const handleApply = async () => {
        if (!activePreset || !platform) return;
        setApplying(true);
        try {
            const p = await GetNetworkPreset(activePreset);
            await SetNetworkParams(new core.NetworkParamValues(p));
            const label = PRESETS.find(pr => pr.key === activePreset)?.label ?? activePreset;
            toast.success(`"${label}" applied. Load character → Exit to menu → Load again to activate.`);
            setActivePreset(null);
            onMutate?.();
        } catch (e) {
            toast.error(String(e));
        } finally {
            setApplying(false);
        }
    };

    return (
        <AccordionSection title="Advanced: Network Speed" defaultOpen={false}>
            <div className="space-y-4 py-1">
                {/* Warning */}
                <div className="flex items-start gap-2 px-3 py-2.5 rounded-lg bg-orange-500/8 border border-orange-500/20">
                    <span className="text-orange-400 text-[11px] flex-shrink-0 mt-0.5">⚠</span>
                    <div className="space-y-1.5">
                        <p className="text-[10px] text-orange-700 leading-relaxed font-medium">
                            Aggressive network settings may increase online enforcement risk.
                        </p>
                        <p className="text-[9px] text-muted-foreground leading-relaxed">
                            <span className="font-bold text-foreground/70">Activation required after import: </span>
                            Load character once → Exit to main menu → Load character again.
                        </p>
                        <p className="text-[9px] text-muted-foreground leading-relaxed">
                            These settings are global for the whole save, not per character.
                        </p>
                        <p className="text-[9px] text-muted-foreground leading-relaxed">
                            Red invasion retry is confirmed after character reload.
                            Summons, Blue, and Host presets modify related NetworkParam fields
                            but should be treated as experimental until tested in their specific online flows.
                        </p>
                    </div>
                </div>

                {/* Vanilla reset */}
                <div className="flex items-center gap-3">
                    <button
                        onClick={handleVanilla}
                        disabled={!platform || applying}
                        className="px-3 py-1.5 rounded-lg border border-border bg-muted/10 text-[9px] font-black uppercase tracking-widest text-muted-foreground hover:bg-muted/20 transition-all disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                        Restore Vanilla Network Params
                    </button>
                    <span className="text-[9px] text-muted-foreground">
                        Targets 5 · interval 30s · timeout 20s · sign download 30s
                    </span>
                </div>

                {/* Groups */}
                {GROUPS.map(group => (
                    <div key={group}>
                        <p className="text-[9px] font-black uppercase tracking-[0.15em] text-muted-foreground mb-2">
                            {group}
                        </p>
                        <div className="flex flex-wrap gap-2">
                            {PRESETS.filter(p => p.group === group).map(preset => {
                                const selected = activePreset === preset.key;
                                return (
                                    <button
                                        key={preset.key}
                                        onClick={() => setActivePreset(selected ? null : preset.key)}
                                        disabled={!platform || applying}
                                        className={`flex flex-col items-start px-3 py-2 rounded-lg border text-left transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                            selected
                                                ? 'bg-primary/10 border-primary/50 shadow-sm'
                                                : 'border-border bg-muted/10 hover:bg-muted/20 cursor-pointer'
                                        }`}
                                    >
                                        <span className={`text-[10px] font-black uppercase tracking-wider ${selected ? 'text-primary' : 'text-foreground'}`}>
                                            {preset.label}
                                        </span>
                                        <span className={`text-[9px] font-medium ${preset.riskStyle}`}>
                                            {preset.risk}
                                        </span>
                                        <span className="text-[9px] text-muted-foreground mt-1 max-w-[200px] leading-snug">
                                            {preset.desc}
                                        </span>
                                        <ul className="mt-1.5 space-y-0.5">
                                            {preset.values.map(v => (
                                                <li key={v} className="text-[9px] text-foreground/60 font-mono leading-tight">
                                                    {v}
                                                </li>
                                            ))}
                                        </ul>
                                        {preset.highlight && (
                                            <span className="mt-1.5 text-[9px] text-orange-400 font-medium leading-snug max-w-[200px]">
                                                {preset.highlight}
                                            </span>
                                        )}
                                    </button>
                                );
                            })}
                        </div>
                    </div>
                ))}

                {/* Apply button */}
                <div className="flex items-center gap-3">
                    <button
                        onClick={handleApply}
                        disabled={!platform || applying || activePreset === null}
                        className="px-4 py-1.5 rounded-lg bg-primary text-primary-foreground text-[9px] font-black uppercase tracking-widest transition-all hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                        {applying ? 'Applying…' : 'Apply Selected Preset'}
                    </button>
                    {!platform && (
                        <span className="text-[9px] text-muted-foreground">Load a save file first.</span>
                    )}
                </div>
            </div>
        </AccordionSection>
    );
}
