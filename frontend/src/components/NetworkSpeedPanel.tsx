import {useState} from 'react';
import toast from '../lib/toast';
import {GetNetworkPreset, ResetNetworkParams, SetNetworkParams} from '../../wailsjs/go/main/App';
import {core} from '../../wailsjs/go/models';
import {AccordionSection} from './AccordionSection';

interface NetworkSpeedPanelProps {
    platform: string | null;
    onMutate?: () => void;
}

type NetPreset = 'vanilla' | 'light' | 'fast';

interface PresetMeta {
    label: string;
    risk: string;
    riskStyle: string;
    desc: string;
}

const PRESET_META: Record<NetPreset, PresetMeta> = {
    vanilla: {
        label: 'Vanilla',
        risk: 'No risk',
        riskStyle: 'text-muted-foreground',
        desc: 'Restore game defaults (interval 30s, timeout 20s, targets 5)',
    },
    light: {
        label: 'Light / Safer',
        risk: 'Lower risk',
        riskStyle: 'text-green-400',
        desc: 'Moderate speed-up (interval 10s, timeout 8s, targets 8)',
    },
    fast: {
        label: 'Fast Invasions',
        risk: 'Higher risk',
        riskStyle: 'text-orange-400',
        desc: 'Aggressive retry cycle (interval 4s, timeout 4s, targets 10)',
    },
};

const PRESET_ORDER: NetPreset[] = ['vanilla', 'light', 'fast'];

export function NetworkSpeedPanel({platform, onMutate}: NetworkSpeedPanelProps) {
    const [applying, setApplying] = useState(false);
    const [activePreset, setActivePreset] = useState<NetPreset | null>(null);

    const handleApply = async () => {
        if (!activePreset || !platform) return;
        setApplying(true);
        try {
            if (activePreset === 'vanilla') {
                await ResetNetworkParams();
            } else {
                const presetId = activePreset === 'fast' ? 'fast-invasions' : 'light-invasions';
                const p = await GetNetworkPreset(presetId);
                await SetNetworkParams(new core.NetworkParamValues(p));
            }
            toast.success(
                `"${PRESET_META[activePreset].label}" applied. ` +
                'Load character → Exit to menu → Load again to activate.',
            );
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
                {/* Warning + activation instructions */}
                <div className="flex items-start gap-2 px-3 py-2.5 rounded-lg bg-orange-500/8 border border-orange-500/20">
                    <span className="text-orange-400 text-[11px] flex-shrink-0 mt-0.5">⚠</span>
                    <div className="space-y-1.5">
                        <p className="text-[10px] text-orange-300 leading-relaxed font-medium">
                            Aggressive network settings may increase online enforcement risk.
                        </p>
                        <p className="text-[9px] text-muted-foreground leading-relaxed">
                            <span className="font-bold text-foreground/70">Activation required after import: </span>
                            Load character once → Exit to main menu → Load character again.
                        </p>
                        <p className="text-[9px] text-muted-foreground leading-relaxed">
                            These settings patch the save's UD11 regulation snapshot. They are global
                            for the whole save, not per character.
                        </p>
                    </div>
                </div>

                {/* Preset selector */}
                <div>
                    <p className="text-[9px] font-black uppercase tracking-[0.15em] text-muted-foreground mb-2">
                        Preset
                    </p>
                    <div className="flex flex-wrap gap-2">
                        {PRESET_ORDER.map(id => {
                            const meta = PRESET_META[id];
                            const selected = activePreset === id;
                            return (
                                <button
                                    key={id}
                                    onClick={() => setActivePreset(selected ? null : id)}
                                    disabled={!platform || applying}
                                    className={`flex flex-col items-start px-3 py-2 rounded-lg border text-left transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                        selected
                                            ? 'bg-primary/10 border-primary/50 shadow-sm'
                                            : 'border-border bg-muted/10 hover:bg-muted/20 cursor-pointer'
                                    }`}
                                >
                                    <span className={`text-[10px] font-black uppercase tracking-wider ${selected ? 'text-primary' : 'text-foreground'}`}>
                                        {meta.label}
                                    </span>
                                    <span className={`text-[9px] ${meta.riskStyle}`}>{meta.risk}</span>
                                    <span className="text-[9px] text-muted-foreground mt-0.5 max-w-[160px] leading-snug">
                                        {meta.desc}
                                    </span>
                                </button>
                            );
                        })}
                    </div>
                </div>

                {/* Apply button */}
                <div className="flex items-center gap-3">
                    <button
                        onClick={handleApply}
                        disabled={!platform || applying || activePreset === null}
                        className="px-4 py-1.5 rounded-lg bg-primary text-primary-foreground text-[9px] font-black uppercase tracking-widest transition-all hover:opacity-90 disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                        {applying ? 'Applying…' : 'Apply Network Preset'}
                    </button>
                    {!platform && (
                        <span className="text-[9px] text-muted-foreground">Load a save file first.</span>
                    )}
                </div>
            </div>
        </AccordionSection>
    );
}
