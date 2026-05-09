import {useState, useEffect, useCallback} from 'react';
import toast from '../lib/toast';
import {
    GetNetworkParams, SetNetworkParams, ResetNetworkParams, GetNetworkPreset,
} from '../../wailsjs/go/main/App';
import {core} from '../../wailsjs/go/models';

interface NetworkTabProps {
    platform: string | null;
}

type RoleTab = 'invader' | 'cooperator' | 'blue' | 'host';

interface SliderDef {
    key: string;
    label: string;
    desc: string;
    min: number;
    max: number;
    step: number;
    unit: string;
    defaultVal: number;
    banRisk?: 'low' | 'moderate';
}

const INVADER_SLIDERS: SliderDef[] = [
    {key: 'maxBreakInTargetListCount', label: 'Max Targets', desc: 'Invasion target candidates per search', min: 1, max: 20, step: 1, unit: '', defaultVal: 5},
    {key: 'breakInRequestIntervalTimeSec', label: 'Request Interval', desc: 'Delay between matchmaking retries', min: 2, max: 30, step: 1, unit: 's', defaultVal: 30},
    {key: 'breakInRequestTimeOutSec', label: 'Request Timeout', desc: 'Timeout per matchmaking request', min: 3, max: 20, step: 1, unit: 's', defaultVal: 20},
    {key: 'breakInRequestAreaCount', label: 'Area Count', desc: 'Number of map areas searched per invasion cycle', min: 1, max: 20, step: 1, unit: '', defaultVal: 5},
];

const COOPERATOR_SLIDERS: SliderDef[] = [
    {key: 'reloadSignIntervalTime2', label: 'Sign Refresh', desc: 'How often the sign list refreshes (lower = signs appear faster)', min: 1, max: 120, step: 1, unit: 's', defaultVal: 60},
    {key: 'reloadSignTotalCount', label: 'Signs Retrieved', desc: 'Max signs downloaded per cycle', min: 1, max: 128, step: 1, unit: '', defaultVal: 20},
    {key: 'reloadSignCellCount', label: 'Signs Per Cell', desc: 'Max signs visible per map cell', min: 1, max: 99, step: 1, unit: '', defaultVal: 10},
    {key: 'updateSignIntervalTime', label: 'Sign Upload', desc: 'How often YOUR sign is updated on server', min: 1, max: 120, step: 1, unit: 's', defaultVal: 30},
    {key: 'singGetMax', label: 'Sign Get Max', desc: 'Hard cap on total retrievable signs', min: 1, max: 128, step: 1, unit: '', defaultVal: 32},
    {key: 'signDownloadSpan', label: 'Download Span', desc: 'Sign list download interval', min: 1, max: 120, step: 1, unit: 's', defaultVal: 30},
    {key: 'signUpdateSpan', label: 'Upload Span', desc: 'Sign data upload interval to server', min: 1, max: 120, step: 1, unit: 's', defaultVal: 60},
];

const BLUE_SLIDERS: SliderDef[] = [
    {key: 'reloadVisitListCoolTime', label: 'Search Cooldown', desc: 'Cooldown between blue phantom searches', min: 1, max: 120, step: 1, unit: 's', defaultVal: 20, banRisk: 'moderate'},
    {key: 'maxCoopBlueSummonCount', label: 'Search Parallelism', desc: 'Max blue phantoms searched simultaneously', min: 1, max: 10, step: 1, unit: '', defaultVal: 2, banRisk: 'moderate'},
    {key: 'maxVisitListCount', label: 'Visit List Size', desc: 'Number of visit targets retrieved per search', min: 1, max: 50, step: 1, unit: '', defaultVal: 5, banRisk: 'moderate'},
    {key: 'reloadSearchCoopBlueMin', label: 'Reload Min', desc: 'Min delay between co-op blue reload searches', min: 1, max: 180, step: 1, unit: 's', defaultVal: 30, banRisk: 'moderate'},
    {key: 'reloadSearchCoopBlueMax', label: 'Reload Max', desc: 'Max delay (randomized between min/max)', min: 1, max: 300, step: 1, unit: 's', defaultVal: 180, banRisk: 'moderate'},
    {key: 'allAreaSearchRateCoopBlue', label: 'Global Search %', desc: 'Chance to search ALL areas (vs local only)', min: 0, max: 100, step: 5, unit: '%', defaultVal: 30, banRisk: 'moderate'},
    {key: 'allAreaSearchRateVsBlue', label: 'Global Retribution %', desc: 'Chance for retribution blue global search', min: 0, max: 100, step: 5, unit: '%', defaultVal: 30, banRisk: 'moderate'},
];

const HOST_SLIDERS: SliderDef[] = [
    {key: 'visitorListMax', label: 'Visitor List Max', desc: 'Max visitor target list entries', min: 1, max: 100, step: 1, unit: '', defaultVal: 10, banRisk: 'moderate'},
    {key: 'visitorTimeOutTime', label: 'Visitor Timeout', desc: 'How long the system waits for a visitor', min: 1, max: 600, step: 5, unit: 's', defaultVal: 60, banRisk: 'moderate'},
    {key: 'visitorDownloadSpan', label: 'Visitor Download', desc: 'Visitor list download interval', min: 1, max: 600, step: 5, unit: 's', defaultVal: 60, banRisk: 'moderate'},
];

const ROLE_PRESETS: Record<RoleTab, {name: string; presetId: string}[]> = {
    invader: [{name: 'Fast Invasions', presetId: 'fast-invasions'}],
    cooperator: [{name: 'Fast Summons', presetId: 'fast-summons'}],
    blue: [{name: 'Fast Blue', presetId: 'fast-blue'}],
    host: [{name: 'Aggressive Host', presetId: 'aggressive-host'}],
};

const ROLE_FIELDS: Record<RoleTab, string[]> = {
    invader: INVADER_SLIDERS.map(s => s.key),
    cooperator: COOPERATOR_SLIDERS.map(s => s.key),
    blue: BLUE_SLIDERS.map(s => s.key),
    host: HOST_SLIDERS.map(s => s.key),
};

const ROLE_META: Record<RoleTab, {label: string; icon: string; color: string; desc: string}> = {
    invader: {label: 'Invader', icon: '⚔', color: 'text-red-400', desc: 'Invasion matchmaking speed'},
    cooperator: {label: 'Cooperator', icon: '☀', color: 'text-yellow-400', desc: 'Summon sign visibility & refresh'},
    blue: {label: 'Blue', icon: '🛡', color: 'text-blue-400', desc: 'Blue Cipher Ring response time'},
    host: {label: 'Host', icon: '👑', color: 'text-emerald-400', desc: "Taunter's Tongue / visitor system"},
};

export function NetworkTab({platform}: NetworkTabProps) {
    const [role, setRole] = useState<RoleTab>('invader');
    const [params, setParams] = useState<core.NetworkParamValues | null>(null);
    const [draft, setDraft] = useState<Record<string, number>>({});
    const [dirty, setDirty] = useState(false);
    const [applying, setApplying] = useState(false);

    const load = useCallback(() => {
        if (!platform) { setParams(null); return; }
        GetNetworkParams().then(p => {
            setParams(p);
            setDraft(paramsToDict(p));
            setDirty(false);
        }).catch(() => setParams(null));
    }, [platform]);

    useEffect(() => { load(); }, [load]);

    const handleApply = async () => {
        setApplying(true);
        try {
            await SetNetworkParams(new core.NetworkParamValues(draft));
            toast.success('Network params applied');
            load();
        } catch (e) { toast.error(String(e)); }
        finally { setApplying(false); }
    };

    const handleReset = async () => {
        setApplying(true);
        try {
            await ResetNetworkParams();
            toast.success('Reset to vanilla defaults');
            load();
        } catch (e) { toast.error(String(e)); }
        finally { setApplying(false); }
    };

    const handlePreset = async (presetId: string) => {
        setApplying(true);
        try {
            const p = await GetNetworkPreset(presetId);
            const presetDict = paramsToDict(p);
            const roleKeys = ROLE_FIELDS[role];
            setDraft(prev => {
                const next = {...prev};
                for (const key of roleKeys) next[key] = presetDict[key];
                return next;
            });
            setDirty(true);
            toast.success(`Preset "${presetId}" loaded — click Apply to save`);
        } catch (e) { toast.error(String(e)); }
        finally { setApplying(false); }
    };

    const updateDraft = (key: string, value: number) => {
        setDraft(prev => ({...prev, [key]: value}));
        setDirty(true);
    };

    const sliders = role === 'invader' ? INVADER_SLIDERS
        : role === 'cooperator' ? COOPERATOR_SLIDERS
        : role === 'blue' ? BLUE_SLIDERS
        : HOST_SLIDERS;

    const presets = ROLE_PRESETS[role];

    if (!platform) {
        return <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Load a save file first</div>;
    }
    if (!params) {
        return <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Loading regulation data...</div>;
    }

    return (
        <div className="h-full flex flex-col gap-3 p-4 overflow-y-auto">
            {/* Role tabs */}
            <div className="flex gap-1.5 p-1 bg-muted/30 rounded-lg border border-border/50 shrink-0">
                {(Object.keys(ROLE_META) as RoleTab[]).map(r => (
                    <button key={r} onClick={() => setRole(r)}
                        className={`flex-1 px-3 py-1.5 rounded-md text-[10px] font-black uppercase tracking-wider transition-all ${
                            role === r
                                ? 'bg-card shadow-sm border border-border/80 text-foreground'
                                : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                        }`}>
                        <span className={ROLE_META[r].color}>{ROLE_META[r].icon}</span>{' '}{ROLE_META[r].label}
                    </button>
                ))}
            </div>

            {/* Description */}
            <p className="text-[10px] text-muted-foreground leading-relaxed shrink-0">
                {ROLE_META[role].desc}. Changes modify <code className="text-[9px] bg-muted/50 px-1 rounded">regulation.bin</code> NetworkParam.
                {role !== 'invader' && <span className="ml-1 text-orange-400 font-bold">[Moderate ban risk]</span>}
            </p>

            {/* Sliders grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 flex-1 content-start">
                {sliders.map(s => (
                    <div key={s.key} className="space-y-1.5 p-3 rounded-lg bg-card border border-border/50" title={s.desc}>
                        <div className="flex items-center justify-between">
                            <label className="text-[11px] font-bold text-foreground">{s.label}</label>
                            <span className="text-[11px] font-mono font-bold text-primary">
                                {draft[s.key] ?? s.defaultVal}{s.unit}
                            </span>
                        </div>
                        <input type="range" min={s.min} max={s.max} step={s.step}
                            value={draft[s.key] ?? s.defaultVal}
                            onChange={e => updateDraft(s.key, parseInt(e.target.value))}
                            className="w-full h-2 rounded-full appearance-none bg-border accent-primary cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:shadow-md" />
                        <div className="flex justify-between text-[9px] text-muted-foreground">
                            <span>{s.min}{s.unit}</span>
                            <span className="font-medium">Default: {s.defaultVal}{s.unit}</span>
                            <span>{s.max}{s.unit}</span>
                        </div>
                        <p className="text-[9px] text-muted-foreground/80 leading-snug">{s.desc}</p>
                    </div>
                ))}
            </div>

            {/* Actions */}
            <div className="flex gap-2 pt-2 border-t border-border/50 shrink-0">
                <button onClick={handleApply} disabled={applying || !dirty}
                    className="px-4 py-1.5 rounded-md text-[10px] font-bold uppercase tracking-wider bg-primary text-primary-foreground shadow-sm hover:brightness-110 active:scale-95 disabled:opacity-50 disabled:pointer-events-none transition-all">
                    Apply
                </button>
                <button onClick={handleReset} disabled={applying}
                    className="px-4 py-1.5 rounded-md text-[10px] font-bold uppercase tracking-wider bg-muted text-foreground border border-border hover:bg-muted/70 active:scale-95 disabled:opacity-50 disabled:pointer-events-none transition-all">
                    Reset All
                </button>
                {presets.map(p => (
                    <button key={p.presetId} onClick={() => handlePreset(p.presetId)} disabled={applying}
                        className="px-4 py-1.5 rounded-md text-[10px] font-bold uppercase tracking-wider bg-orange-600 text-white shadow-sm hover:brightness-110 active:scale-95 disabled:opacity-50 disabled:pointer-events-none transition-all">
                        {p.name}
                    </button>
                ))}
            </div>
        </div>
    );
}

function paramsToDict(p: core.NetworkParamValues): Record<string, number> {
    return {
        maxBreakInTargetListCount: p.maxBreakInTargetListCount,
        breakInRequestIntervalTimeSec: p.breakInRequestIntervalTimeSec,
        breakInRequestTimeOutSec: p.breakInRequestTimeOutSec,
        breakInRequestAreaCount: p.breakInRequestAreaCount,
        reloadSignIntervalTime2: p.reloadSignIntervalTime2,
        reloadSignTotalCount: p.reloadSignTotalCount,
        reloadSignCellCount: p.reloadSignCellCount,
        updateSignIntervalTime: p.updateSignIntervalTime,
        singGetMax: p.singGetMax,
        signDownloadSpan: p.signDownloadSpan,
        signUpdateSpan: p.signUpdateSpan,
        reloadVisitListCoolTime: p.reloadVisitListCoolTime,
        maxCoopBlueSummonCount: p.maxCoopBlueSummonCount,
        maxVisitListCount: p.maxVisitListCount,
        reloadSearchCoopBlueMin: p.reloadSearchCoopBlueMin,
        reloadSearchCoopBlueMax: p.reloadSearchCoopBlueMax,
        allAreaSearchRateCoopBlue: p.allAreaSearchRateCoopBlue,
        allAreaSearchRateVsBlue: p.allAreaSearchRateVsBlue,
        visitorListMax: p.visitorListMax,
        visitorTimeOutTime: p.visitorTimeOutTime,
        visitorDownloadSpan: p.visitorDownloadSpan,
    };
}
