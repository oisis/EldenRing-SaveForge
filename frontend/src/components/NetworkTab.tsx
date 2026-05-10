import {useState, useEffect, useCallback} from 'react';
import toast from '../lib/toast';
import {
    GetNetworkParams, SetNetworkParams, ResetNetworkParams,
} from '../../wailsjs/go/main/App';
import {core} from '../../wailsjs/go/models';
import {AccordionSection} from './AccordionSection';

interface NetworkTabProps {
    platform: string | null;
}

type RoleTab = 'invader' | 'cooperator' | 'blue' | 'host';
type GlobalPreset = 'vanilla' | 'faster' | 'aggressive';

interface SliderDef {
    key: string;
    label: string;
    desc: string;
    min: number;
    max: number;
    step: number;
    unit: string;
    defaultVal: number;
}

const INVADER_SLIDERS: SliderDef[] = [
    {key: 'maxBreakInTargetListCount',     label: 'Max Targets',      desc: 'How many invasion target candidates are polled per matchmaking search. Higher = more potential targets found at once.',           min: 1,  max: 20,  step: 1, unit: '',  defaultVal: 5},
    {key: 'breakInRequestIntervalTimeSec', label: 'Request Interval',  desc: 'Delay in seconds between matchmaking retries when no target is found. Lower = faster retry loop.',                                 min: 2,  max: 30,  step: 1, unit: 's', defaultVal: 30},
    {key: 'breakInRequestTimeOutSec',      label: 'Request Timeout',   desc: 'Seconds before a single matchmaking request is considered failed and retried. Lower = faster failure recovery.',                   min: 3,  max: 20,  step: 1, unit: 's', defaultVal: 20},
    {key: 'breakInRequestAreaCount',       label: 'Area Count',        desc: 'Number of map areas searched per invasion cycle. Higher = wider net, more CPU/bandwidth on From Software servers.',               min: 1,  max: 20,  step: 1, unit: '',  defaultVal: 5},
];

const COOPERATOR_SLIDERS: SliderDef[] = [
    {key: 'reloadSignIntervalTime2', label: 'Sign Refresh',    desc: 'How often (seconds) the game fetches the summon sign list. Lower = signs from other players appear faster.',             min: 1,  max: 120, step: 1, unit: 's', defaultVal: 60},
    {key: 'reloadSignTotalCount',    label: 'Signs Retrieved', desc: 'Maximum number of summon signs downloaded per refresh cycle. Higher = more signs visible.',                             min: 1,  max: 128, step: 1, unit: '',  defaultVal: 20},
    {key: 'reloadSignCellCount',     label: 'Signs Per Cell',  desc: 'Maximum signs visible within a single map cell. Higher = denser sign pools in busy areas.',                             min: 1,  max: 99,  step: 1, unit: '',  defaultVal: 10},
    {key: 'updateSignIntervalTime',  label: 'Sign Upload',     desc: 'How often your own summon sign is re-uploaded to the server. Lower = your sign stays fresher for hosts.',              min: 1,  max: 120, step: 1, unit: 's', defaultVal: 30},
    {key: 'singGetMax',              label: 'Sign Get Max',    desc: 'Hard cap on the total number of signs retrievable regardless of other settings.',                                        min: 1,  max: 128, step: 1, unit: '',  defaultVal: 32},
    {key: 'signDownloadSpan',        label: 'Download Span',   desc: 'Interval between full sign list download cycles.',                                                                      min: 1,  max: 120, step: 1, unit: 's', defaultVal: 30},
    {key: 'signUpdateSpan',          label: 'Upload Span',     desc: 'Interval between sign data uploads to the matchmaking server.',                                                         min: 1,  max: 120, step: 1, unit: 's', defaultVal: 60},
];

const BLUE_SLIDERS: SliderDef[] = [
    {key: 'reloadVisitListCoolTime',    label: 'Search Cooldown',      desc: 'Cooldown in seconds between Blue Cipher Ring search cycles. Lower = searches for invaded hosts more frequently.',           min: 1,  max: 120, step: 1,  unit: 's', defaultVal: 20},
    {key: 'maxCoopBlueSummonCount',     label: 'Search Parallelism',   desc: 'How many blue phantom searches run simultaneously. Higher = covers more invaded hosts at once.',                              min: 1,  max: 10,  step: 1,  unit: '',  defaultVal: 2},
    {key: 'maxVisitListCount',          label: 'Visit List Size',      desc: 'Number of potential invaded-host targets fetched per search cycle. Higher = more options evaluated.',                         min: 1,  max: 50,  step: 1,  unit: '',  defaultVal: 5},
    {key: 'reloadSearchCoopBlueMin',    label: 'Reload Min',           desc: 'Minimum delay (seconds) between co-op blue reload searches. Acts as a floor for the randomised interval.',                    min: 1,  max: 180, step: 1,  unit: 's', defaultVal: 30},
    {key: 'reloadSearchCoopBlueMax',    label: 'Reload Max',           desc: 'Maximum delay (seconds) for the reload interval. Actual delay is randomised between Min and Max each cycle.',                min: 1,  max: 300, step: 1,  unit: 's', defaultVal: 180},
    {key: 'allAreaSearchRateCoopBlue',  label: 'Global Search %',      desc: 'Percentage chance the blue search covers ALL map areas instead of only the local area. Higher = wider reach.',                min: 0,  max: 100, step: 5,  unit: '%', defaultVal: 30},
    {key: 'allAreaSearchRateVsBlue',    label: 'Global Retribution %', desc: 'Percentage chance the retribution hunter search spans all areas. Mirrors Global Search % but for the retribution hunter role.', min: 0,  max: 100, step: 5,  unit: '%', defaultVal: 30},
];

const HOST_SLIDERS: SliderDef[] = [
    {key: 'visitorListMax',      label: 'Visitor List Max',   desc: "Maximum number of visitor (invader / Taunter's Tongue) targets fetched per search.",            min: 1, max: 100, step: 1,  unit: '',  defaultVal: 10},
    {key: 'visitorTimeOutTime',  label: 'Visitor Timeout',    desc: 'Seconds the host system waits for a visitor connection before giving up and retrying.',          min: 1, max: 600, step: 5,  unit: 's', defaultVal: 60},
    {key: 'visitorDownloadSpan', label: 'Visitor Download',   desc: 'Interval in seconds between visitor list downloads from the matchmaking server.',                min: 1, max: 600, step: 5,  unit: 's', defaultVal: 60},
];

const ROLE_META: Record<RoleTab, {label: string; icon: string; desc: string; titleClassName: string; banRisk?: boolean}> = {
    invader:    {label: 'Invader', icon: '⚔',  desc: 'Invasion matchmaking speed',            titleClassName: 'text-red-800 dark:text-red-700'},
    cooperator: {label: 'Summon',  icon: '☀',  desc: 'Summon sign visibility & refresh',       titleClassName: 'text-orange-800 dark:text-orange-600'},
    blue:       {label: 'Hunter',  icon: '🛡',  desc: 'Blue Cipher Ring response time',         titleClassName: 'text-blue-800 dark:text-blue-700',   banRisk: true},
    host:       {label: 'Host',    icon: '👑',  desc: "Taunter's Tongue / visitor system",      titleClassName: 'text-foreground',                     banRisk: true},
};

const ROLE_SLIDERS: Record<RoleTab, SliderDef[]> = {
    invader: INVADER_SLIDERS,
    cooperator: COOPERATOR_SLIDERS,
    blue: BLUE_SLIDERS,
    host: HOST_SLIDERS,
};

const ROLES: RoleTab[] = ['invader', 'cooperator', 'blue', 'host'];

const VANILLA_VALUES: Record<string, number> = {
    maxBreakInTargetListCount: 5,    breakInRequestIntervalTimeSec: 30, breakInRequestTimeOutSec: 20,  breakInRequestAreaCount: 5,
    reloadSignIntervalTime2: 60,     reloadSignTotalCount: 20,          reloadSignCellCount: 10,        updateSignIntervalTime: 30,
    singGetMax: 32,                  signDownloadSpan: 30,              signUpdateSpan: 60,
    reloadVisitListCoolTime: 20,     maxCoopBlueSummonCount: 2,         maxVisitListCount: 5,
    reloadSearchCoopBlueMin: 30,     reloadSearchCoopBlueMax: 180,      allAreaSearchRateCoopBlue: 30,  allAreaSearchRateVsBlue: 30,
    visitorListMax: 10,              visitorTimeOutTime: 60,            visitorDownloadSpan: 60,
};

// Mirrors the sum of all backend role presets (fast-invasions + fast-summons + fast-blue + aggressive-host)
const FASTER_VALUES: Record<string, number> = {
    maxBreakInTargetListCount: 10,   breakInRequestIntervalTimeSec: 10, breakInRequestTimeOutSec: 5,   breakInRequestAreaCount: 10,
    reloadSignIntervalTime2: 30,     reloadSignTotalCount: 25,          reloadSignCellCount: 15,        updateSignIntervalTime: 20,
    singGetMax: 40,                  signDownloadSpan: 15,              signUpdateSpan: 30,
    reloadVisitListCoolTime: 10,     maxCoopBlueSummonCount: 4,         maxVisitListCount: 10,
    reloadSearchCoopBlueMin: 15,     reloadSearchCoopBlueMax: 30,       allAreaSearchRateCoopBlue: 50,  allAreaSearchRateVsBlue: 50,
    visitorListMax: 15,              visitorTimeOutTime: 30,            visitorDownloadSpan: 30,
};

// Maximum aggressiveness — moderate ban risk, offline use recommended
const AGGRESSIVE_VALUES: Record<string, number> = {
    maxBreakInTargetListCount: 15,   breakInRequestIntervalTimeSec: 6,  breakInRequestTimeOutSec: 3,   breakInRequestAreaCount: 15,
    reloadSignIntervalTime2: 10,     reloadSignTotalCount: 30,          reloadSignCellCount: 20,        updateSignIntervalTime: 10,
    singGetMax: 50,                  signDownloadSpan: 5,               signUpdateSpan: 10,
    reloadVisitListCoolTime: 5,      maxCoopBlueSummonCount: 8,         maxVisitListCount: 15,
    reloadSearchCoopBlueMin: 5,      reloadSearchCoopBlueMax: 10,       allAreaSearchRateCoopBlue: 75,  allAreaSearchRateVsBlue: 75,
    visitorListMax: 20,              visitorTimeOutTime: 10,            visitorDownloadSpan: 10,
};

interface PresetParamDesc {label: string; value: string; effect: string}
type SectionPresetDescs = Record<GlobalPreset, PresetParamDesc[]>;

const SECTION_PRESET_DESCS: Record<RoleTab, SectionPresetDescs> = {
    invader: {
        vanilla: [
            {label: 'Max Targets',      value: '5',   effect: 'Small pool — standard wait times in populated areas, may struggle in niche zones.'},
            {label: 'Request Interval', value: '30s', effect: '30s gap between retries — noticeable dead time between failed searches.'},
            {label: 'Request Timeout',  value: '20s', effect: 'Slow failure recovery — a stuck connection hangs up to 20s before being abandoned.'},
            {label: 'Area Count',       value: '5',   effect: 'Nearby zones only — targets in adjacent or remote areas are missed.'},
        ],
        faster: [
            {label: 'Max Targets',      value: '10',  effect: 'Doubled pool — more candidates per cycle, faster lock in busy areas.'},
            {label: 'Request Interval', value: '10s', effect: 'Retries 3× more often — failed searches recover within seconds.'},
            {label: 'Request Timeout',  value: '5s',  effect: 'Failed connections cut at 5s — matchmaker moves to next candidate quickly.'},
            {label: 'Area Count',       value: '10',  effect: 'Double coverage — better odds in moderately populated maps.'},
        ],
        aggressive: [
            {label: 'Max Targets',      value: '15',  effect: 'Large pool — matchmaker rarely comes up empty; wait times drop sharply.'},
            {label: 'Request Interval', value: '6s',  effect: 'Near-continuous retry — the game barely pauses between invasion attempts.'},
            {label: 'Request Timeout',  value: '3s',  effect: 'Minimum practical timeout — bad connections dropped almost instantly.'},
            {label: 'Area Count',       value: '15',  effect: 'Wide net — effective even in rarely-invaded areas with few active players.'},
        ],
    },
    cooperator: {
        vanilla: [
            {label: 'Sign Refresh',    value: '60s', effect: 'Once per minute — freshly placed signs can be invisible for up to 60s.'},
            {label: 'Signs Retrieved', value: '20',  effect: '20 signs per batch — busy areas appear sparsely signed.'},
            {label: 'Signs Per Cell',  value: '10',  effect: '10 per cell — popular spots look empty even when packed.'},
            {label: 'Sign Upload',     value: '30s', effect: 'Your sign re-uploaded every 30s — can disappear from host screens between cycles.'},
            {label: 'Sign Get Max',    value: '32',  effect: 'Hard cap of 32 — sign pool truncated in very busy areas.'},
            {label: 'Download Span',   value: '30s', effect: 'Full sign list re-downloaded every 30s — moderate staleness.'},
            {label: 'Upload Span',     value: '60s', effect: 'Your sign data pushed to server once per minute — metadata can lag.'},
        ],
        faster: [
            {label: 'Sign Refresh',    value: '30s', effect: 'Twice-per-minute refresh — sign pool stays reasonably current.'},
            {label: 'Signs Retrieved', value: '25',  effect: 'Slightly larger batch — a few more signs visible in crowded zones.'},
            {label: 'Signs Per Cell',  value: '15',  effect: '50% more per cell — summoning zones feel more populated.'},
            {label: 'Sign Upload',     value: '20s', effect: 'More frequent re-uploads — your sign stays visible to hosts more consistently.'},
            {label: 'Sign Get Max',    value: '40',  effect: 'Higher cap — reduces truncation in moderately busy areas.'},
            {label: 'Download Span',   value: '15s', effect: 'Twice as frequent — sign pool stays more current.'},
            {label: 'Upload Span',     value: '30s', effect: 'Twice as frequent — your sign data is more current on the server.'},
        ],
        aggressive: [
            {label: 'Sign Refresh',    value: '10s', effect: 'Near real-time — new signs appear within seconds of placement.'},
            {label: 'Signs Retrieved', value: '30',  effect: 'Largest batch — best visibility of available signs in high-traffic areas.'},
            {label: 'Signs Per Cell',  value: '20',  effect: 'Double vanilla — cells reflect actual player density more accurately.'},
            {label: 'Sign Upload',     value: '10s', effect: 'Very frequent re-uploads — your sign almost never goes stale on the server.'},
            {label: 'Sign Get Max',    value: '50',  effect: 'High cap — virtually removes the bottleneck in all but the most crowded areas.'},
            {label: 'Download Span',   value: '5s',  effect: 'Minimal latency between sign placement and your visibility of it.'},
            {label: 'Upload Span',     value: '10s', effect: 'Near-constant sync — sign metadata always up to date.'},
        ],
    },
    blue: {
        vanilla: [
            {label: 'Search Cooldown',      value: '20s',  effect: 'Searches for invaded hosts every 20s — moderate response time.'},
            {label: 'Search Parallelism',   value: '2',    effect: 'Two simultaneous searches — limited simultaneous coverage.'},
            {label: 'Visit List Size',      value: '5',    effect: 'Five candidates per cycle — narrow pool, can miss invasions in less-populated areas.'},
            {label: 'Reload Min',           value: '30s',  effect: 'Minimum 30s between reload cycles — conservative, can feel slow.'},
            {label: 'Reload Max',           value: '180s', effect: 'Can stretch to 3 minutes between reloads — occasional very long waits.'},
            {label: 'Global Search %',      value: '30%',  effect: '30% of searches cover all areas — mostly local hunting.'},
            {label: 'Global Retribution %', value: '30%',  effect: 'Retribution searches are mostly local — 30% global coverage.'},
        ],
        faster: [
            {label: 'Search Cooldown',      value: '10s',  effect: 'Searches twice as often — faster dispatch to ongoing invasions.'},
            {label: 'Search Parallelism',   value: '4',    effect: 'Four parallel searches — significantly better coverage across invasion zones.'},
            {label: 'Visit List Size',      value: '10',   effect: 'Double the pool — better match quality and coverage.'},
            {label: 'Reload Min',           value: '15s',  effect: 'Minimum halved — more frequent reloads during active periods.'},
            {label: 'Reload Max',           value: '30s',  effect: 'Capped at 30s — wait times short and predictable.'},
            {label: 'Global Search %',      value: '50%',  effect: 'Half of searches are global — better reach in under-populated regions.'},
            {label: 'Global Retribution %', value: '50%',  effect: 'Half go global — wider retribution coverage.'},
        ],
        aggressive: [
            {label: 'Search Cooldown',      value: '5s',   effect: 'Near-continuous monitoring — minimal delay between invasion start and dispatch.'},
            {label: 'Search Parallelism',   value: '8',    effect: 'Eight simultaneous searches — high throughput for rapidly finding active invasions.'},
            {label: 'Visit List Size',      value: '15',   effect: 'Larger pool — system has more options and is less likely to come up empty.'},
            {label: 'Reload Min',           value: '5s',   effect: 'Very short minimum — rapid reloads in bursts of invasion activity.'},
            {label: 'Reload Max',           value: '10s',  effect: 'Tight 5–10s range — virtually no long-wait outliers.'},
            {label: 'Global Search %',      value: '75%',  effect: 'Most searches are global — hunting across the entire map, not just nearby zones.'},
            {label: 'Global Retribution %', value: '75%',  effect: 'Most retribution searches are global — mirrors Global Search % coverage.'},
        ],
    },
    host: {
        vanilla: [
            {label: 'Visitor List Max',  value: '10',  effect: 'Up to 10 invader candidates per cycle — adequate for normal play.'},
            {label: 'Visitor Timeout',   value: '60s', effect: 'Waits 60s per connection attempt — slow to recover from bad connections.'},
            {label: 'Visitor Download',  value: '60s', effect: 'Invader list refreshed every 60s — queue can be significantly stale.'},
        ],
        faster: [
            {label: 'Visitor List Max',  value: '15',  effect: 'Larger pool — better odds of a quick match.'},
            {label: 'Visitor Timeout',   value: '30s', effect: 'Half the wait — failed connections abandoned faster, freeing the system sooner.'},
            {label: 'Visitor Download',  value: '30s', effect: 'Twice-per-minute refresh — available invader pool stays more current.'},
        ],
        aggressive: [
            {label: 'Visitor List Max',  value: '20',  effect: 'Double vanilla — reduces the chance of an empty queue when demand is high.'},
            {label: 'Visitor Timeout',   value: '10s', effect: 'Near-instant recovery — moves to next candidate within seconds of connection failure.'},
            {label: 'Visitor Download',  value: '10s', effect: 'Frequent refreshes — near real-time view of who\'s trying to invade.'},
        ],
    },
};

const GLOBAL_PRESETS: Record<GlobalPreset, {label: string; desc: string; values: Record<string, number>; risk?: boolean}> = {
    vanilla:    {label: 'Vanilla',    desc: 'Game defaults — no risk',            values: VANILLA_VALUES},
    faster:     {label: 'Faster',     desc: 'Balanced speed-up, all roles',       values: FASTER_VALUES},
    aggressive: {label: 'Aggressive', desc: 'Max speed — moderate ban risk',      values: AGGRESSIVE_VALUES, risk: true},
};

const PRESET_LIST: GlobalPreset[] = ['vanilla', 'faster', 'aggressive'];

function resolvePreset(draft: Record<string, number>): GlobalPreset | null {
    for (const id of PRESET_LIST) {
        if (Object.entries(GLOBAL_PRESETS[id].values).every(([k, v]) => draft[k] === v)) return id;
    }
    return null;
}

export function NetworkTab({platform}: NetworkTabProps) {
    const [params, setParams] = useState<core.NetworkParamValues | null>(null);
    const [draft, setDraft] = useState<Record<string, number>>({});
    const [dirty, setDirty] = useState(false);
    const [applying, setApplying] = useState(false);
    const [descModal, setDescModal] = useState<{label: string; desc: string} | null>(null);
    const [presetInfoRole, setPresetInfoRole] = useState<RoleTab | null>(null);
    const [presetInfoPreset, setPresetInfoPreset] = useState<GlobalPreset>('vanilla');

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

    const applyGlobalPreset = (preset: GlobalPreset) => {
        setDraft({...GLOBAL_PRESETS[preset].values});
        setDirty(true);
    };

    const updateDraft = (key: string, value: number) => {
        setDraft(prev => ({...prev, [key]: value}));
        setDirty(true);
    };

    const activePreset = resolvePreset(draft);

    const makePresetButtons = (role: RoleTab) => (
        <div className="flex items-center gap-1">
            {PRESET_LIST.map(id => {
                const p = GLOBAL_PRESETS[id];
                const active = activePreset === id;
                return (
                    <button key={id} onClick={() => applyGlobalPreset(id)} title={p.desc}
                        className={`px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-wider transition-all border ${
                            active
                                ? id === 'aggressive'
                                    ? 'bg-red-500/20 border-red-500/60 text-red-400'
                                    : 'bg-primary/20 border-primary/60 text-primary'
                                : id === 'aggressive'
                                    ? 'border-border/50 text-muted-foreground/50 hover:border-red-500/40 hover:text-red-400'
                                    : 'border-border/50 text-muted-foreground/50 hover:border-primary/40 hover:text-foreground'
                        }`}>
                        {p.label}
                    </button>
                );
            })}
            <button
                onClick={() => { setPresetInfoRole(role); setPresetInfoPreset('vanilla'); }}
                className="w-4 h-4 rounded-full border border-foreground/30 text-foreground/50 hover:border-primary hover:text-primary transition-all text-[8px] font-black flex items-center justify-center shrink-0 leading-none ml-0.5"
                title="Preset values explained">
                ⓘ
            </button>
        </div>
    );

    if (!platform) {
        return <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Load a save file first</div>;
    }
    if (!params) {
        return <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Loading regulation data...</div>;
    }

    return (
        <>
            {/* Preset info modal — per section */}
            {presetInfoRole && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm animate-in fade-in duration-150"
                    onClick={() => setPresetInfoRole(null)}>
                    <div className="bg-background border border-border rounded-xl p-5 max-w-sm w-full shadow-2xl mx-4 animate-in zoom-in-95 duration-150 flex flex-col max-h-[80vh]"
                        onClick={e => e.stopPropagation()}>
                        {/* Header */}
                        <div className="flex items-start justify-between gap-3 mb-3 shrink-0">
                            <div>
                                <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">
                                    {ROLE_META[presetInfoRole].icon} {ROLE_META[presetInfoRole].label}
                                </p>
                                <h3 className="text-[11px] font-black uppercase tracking-widest text-foreground mt-0.5">Preset Values Explained</h3>
                            </div>
                            <button onClick={() => setPresetInfoRole(null)}
                                className="text-muted-foreground hover:text-foreground transition-colors shrink-0 mt-0.5">
                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>
                        {/* Preset tabs */}
                        <div className="flex gap-1 mb-3 shrink-0">
                            {PRESET_LIST.map(pid => (
                                <button key={pid} onClick={() => setPresetInfoPreset(pid)}
                                    className={`px-3 py-1 rounded text-[9px] font-black uppercase tracking-wider transition-all border ${
                                        presetInfoPreset === pid
                                            ? pid === 'aggressive'
                                                ? 'bg-red-500/20 border-red-500/60 text-red-400'
                                                : 'bg-primary/20 border-primary/60 text-primary'
                                            : 'border-border/50 text-muted-foreground/50 hover:text-foreground hover:border-border'
                                    }`}>
                                    {GLOBAL_PRESETS[pid].label}
                                </button>
                            ))}
                        </div>
                        {/* Param list */}
                        <div className="overflow-y-auto custom-scrollbar space-y-2 pr-1">
                            {SECTION_PRESET_DESCS[presetInfoRole][presetInfoPreset].map(p => (
                                <div key={p.label} className="rounded-lg bg-muted/10 border border-border/40 px-3 py-2">
                                    <div className="flex items-center justify-between gap-2 mb-0.5">
                                        <span className="text-[10px] font-black text-foreground">{p.label}</span>
                                        <span className="text-[10px] font-mono font-black text-primary shrink-0">{p.value}</span>
                                    </div>
                                    <p className="text-[10px] text-muted-foreground leading-relaxed">{p.effect}</p>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
            )}

            {/* Description modal */}
            {descModal && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm animate-in fade-in duration-150"
                    onClick={() => setDescModal(null)}>
                    <div className="bg-background border border-border rounded-xl p-5 max-w-sm w-full shadow-2xl mx-4 animate-in zoom-in-95 duration-150"
                        onClick={e => e.stopPropagation()}>
                        <div className="flex items-start justify-between gap-3 mb-3">
                            <h3 className="text-[11px] font-black uppercase tracking-widest text-foreground">{descModal.label}</h3>
                            <button onClick={() => setDescModal(null)}
                                className="text-muted-foreground hover:text-foreground transition-colors shrink-0 mt-0.5">
                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>
                        <p className="text-[11px] text-muted-foreground leading-relaxed">{descModal.desc}</p>
                    </div>
                </div>
            )}

            <div className="space-y-2 animate-in fade-in duration-200">
                {ROLES.map(role => {
                    const meta = ROLE_META[role];
                    const sliders = ROLE_SLIDERS[role];

                    return (
                        <AccordionSection
                            key={role}
                            id={`network-${role}`}
                            title={`${meta.icon} ${meta.label}`}
                            titleClassName={meta.titleClassName}
                            summary={meta.desc}
                            actions={meta.banRisk ? (
                                <span className="text-[9px] font-black uppercase tracking-widest text-orange-400 border border-orange-400/40 bg-orange-400/10 px-1.5 py-0.5 rounded">
                                    Moderate Risk
                                </span>
                            ) : undefined}
                            headerRight={makePresetButtons(role)}
                        >
                            {activePreset === 'aggressive' && meta.banRisk && (
                                <p className="text-[9px] font-bold text-orange-400 mb-2">
                                    ⚠ Aggressive preset active — moderate ban risk. Recommended for offline use only.
                                </p>
                            )}
                            <div className="grid grid-cols-2 lg:grid-cols-4 gap-2 pt-1">
                                {sliders.map(s => (
                                    <div key={s.key} className="space-y-1 p-2 rounded-lg bg-card border border-border/50">
                                        <div className="flex items-center justify-between gap-1">
                                            <div className="flex items-center gap-1 min-w-0">
                                                <label className="text-[10px] font-bold text-foreground truncate">{s.label}</label>
                                                <button
                                                    onClick={() => setDescModal({label: s.label, desc: s.desc})}
                                                    className="w-3.5 h-3.5 rounded-full border border-foreground/40 text-foreground/70 hover:border-primary hover:text-primary transition-all text-[8px] font-black flex items-center justify-center shrink-0 leading-none">
                                                    ?
                                                </button>
                                            </div>
                                            <span className="text-[10px] font-mono font-black text-primary shrink-0">
                                                {draft[s.key] ?? s.defaultVal}{s.unit}
                                            </span>
                                        </div>
                                        <div className="flex items-center gap-1.5">
                                            <span className="text-[10px] text-foreground shrink-0 tabular-nums">{s.min}{s.unit}</span>
                                            <input type="range" min={s.min} max={s.max} step={s.step}
                                                value={draft[s.key] ?? s.defaultVal}
                                                onChange={e => updateDraft(s.key, parseInt(e.target.value))}
                                                className="flex-1 h-1.5 rounded-full appearance-none bg-border accent-primary cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:shadow-sm" />
                                            <span className="text-[10px] text-foreground shrink-0 tabular-nums">{s.max}{s.unit}</span>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </AccordionSection>
                    );
                })}

                <div className="flex items-center gap-2 pt-1 border-t border-border/50">
                    <button onClick={handleApply} disabled={applying || !dirty}
                        className="px-4 py-1.5 rounded-md text-[10px] font-bold uppercase tracking-wider bg-primary text-primary-foreground shadow-sm hover:brightness-110 active:scale-95 disabled:opacity-50 disabled:pointer-events-none transition-all">
                        Apply
                    </button>
                    <button onClick={handleReset} disabled={applying}
                        className="px-4 py-1.5 rounded-md text-[10px] font-bold uppercase tracking-wider bg-muted text-foreground border border-border hover:bg-muted/70 active:scale-95 disabled:opacity-50 disabled:pointer-events-none transition-all">
                        Reset All
                    </button>
                    {dirty && (
                        <span className="text-[9px] font-bold text-amber-500">Unsaved changes</span>
                    )}
                </div>

                {/* How-to note */}
                <div className="space-y-2 pt-1">
                    {/* Disclaimer */}
                    <div className="rounded-lg border border-red-500/40 bg-red-500/5 p-3 space-y-1">
                        <p className="text-sm font-black uppercase tracking-[0.15em] text-red-500">⚠ Warning — Read before use</p>
                        <p className="text-sm text-red-500/90 leading-relaxed">
                            These changes may result in a ban. This feature is currently in testing. Use with caution — preferably not on your main account.
                            Remember: any manual modification to a save file carries a ban risk.
                            I take no responsibility for any account being banned as a result of using these settings.
                        </p>
                    </div>

                    <div className="rounded-lg border border-border/60 bg-muted/10 p-3 space-y-1.5">
                        <p className="text-sm font-black uppercase tracking-[0.15em] text-muted-foreground">How to activate on PS4 / PS5</p>
                        <ol className="space-y-1.5 list-none">
                            {[
                                'Apply your changes and save the file (Save As).',
                                'Copy the modified save to your console.',
                                'Launch Elden Ring and load any character. At this point regulation.bin is reset — either pulled from FromSoftware servers or loaded from the game installation on disk. Either way, your custom settings are overwritten.',
                                'Exit that character back to the main menu (do NOT quit the game).',
                                'Load any character again. This time the game reads regulation.bin directly from the save file — your custom NetworkParam values are now active.',
                            ].map((step, i) => (
                                <li key={i} className="flex items-start gap-2">
                                    <span className="text-sm font-black text-primary shrink-0 mt-0.5">{i + 1}.</span>
                                    <span className="text-sm text-foreground/80 leading-relaxed">{step}</span>
                                </li>
                            ))}
                        </ol>
                        <p className="text-sm text-muted-foreground/70 italic mt-2">
                            Note: these settings have only been tested on PS4/PS5. Behaviour on PC is unknown and has not been verified.
                        </p>
                    </div>
                </div>
            </div>
        </>
    );
}

function paramsToDict(p: core.NetworkParamValues): Record<string, number> {
    return {
        maxBreakInTargetListCount:     p.maxBreakInTargetListCount,
        breakInRequestIntervalTimeSec: p.breakInRequestIntervalTimeSec,
        breakInRequestTimeOutSec:      p.breakInRequestTimeOutSec,
        breakInRequestAreaCount:       p.breakInRequestAreaCount,
        reloadSignIntervalTime2:       p.reloadSignIntervalTime2,
        reloadSignTotalCount:          p.reloadSignTotalCount,
        reloadSignCellCount:           p.reloadSignCellCount,
        updateSignIntervalTime:        p.updateSignIntervalTime,
        singGetMax:                    p.singGetMax,
        signDownloadSpan:              p.signDownloadSpan,
        signUpdateSpan:                p.signUpdateSpan,
        reloadVisitListCoolTime:       p.reloadVisitListCoolTime,
        maxCoopBlueSummonCount:        p.maxCoopBlueSummonCount,
        maxVisitListCount:             p.maxVisitListCount,
        reloadSearchCoopBlueMin:       p.reloadSearchCoopBlueMin,
        reloadSearchCoopBlueMax:       p.reloadSearchCoopBlueMax,
        allAreaSearchRateCoopBlue:     p.allAreaSearchRateCoopBlue,
        allAreaSearchRateVsBlue:       p.allAreaSearchRateVsBlue,
        visitorListMax:                p.visitorListMax,
        visitorTimeOutTime:            p.visitorTimeOutTime,
        visitorDownloadSpan:           p.visitorDownloadSpan,
    };
}
