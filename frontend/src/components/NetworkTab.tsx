import {useState, useEffect, useCallback} from 'react';
import toast from '../lib/toast';
import {
    GetNetworkParams, SetNetworkParams, ResetNetworkParams, GetNetworkPreset,
} from '../../wailsjs/go/main/App';
import {core} from '../../wailsjs/go/models';
import {AccordionSection} from './AccordionSection';
import {clampNetworkDraft, networkDraftError, applyGroupPreset, NETWORK_GROUP_KEYS, type NetDraft} from './networkClamp';

interface NetworkTabProps {
    platform: string | null;
}

type GroupId = 'invader' | 'cooperator' | 'blue';

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

// --- Default-visible groups ---

const INVADER_SLIDERS: SliderDef[] = [
    {key: 'maxBreakInTargetListCount',     label: 'Max Targets',      desc: 'How many invasion target candidates are polled per matchmaking search. Higher = more potential targets found at once.', min: 1,  max: 20, step: 1, unit: '',  defaultVal: 5},
    {key: 'breakInRequestAreaCount',       label: 'Search Areas',     desc: 'How many break-in matchmaking areas are queried per invasion search cycle. Tune with Max Targets so search breadth and candidate list size stay aligned.', min: 1, max: 20, step: 1, unit: '', defaultVal: 5},
    {key: 'breakInRequestIntervalTimeSec', label: 'Request Interval', desc: 'Delay in seconds between matchmaking retries when no target is found. Lower = faster retry loop. Below ~8s the search message flickers almost continuously.', min: 2,  max: 30, step: 1, unit: 's', defaultVal: 30},
    {key: 'breakInRequestTimeOutSec',      label: 'Request Timeout',  desc: 'Seconds before a single matchmaking request is abandoned. Too low (e.g. 3s) cancels near-and-far requests before they can complete.', min: 3,  max: 20, step: 1, unit: 's', defaultVal: 20},
];

const COOPERATOR_SLIDERS: SliderDef[] = [
    {key: 'reloadSignIntervalTime2', label: 'Sign Refresh',    desc: 'How often (seconds) the game fetches the summon sign list. Lower = signs from other players appear faster.', min: 1,  max: 120, step: 1, unit: 's', defaultVal: 60},
    {key: 'reloadSignTotalCount',    label: 'Signs Retrieved', desc: 'Maximum number of summon signs downloaded per refresh cycle. Must be ≤ Sign Get Max.',                       min: 1,  max: 128, step: 1, unit: '',  defaultVal: 20},
    {key: 'reloadSignCellCount',     label: 'Signs Per Cell',  desc: 'Maximum signs visible within a single map cell. Must be ≤ Signs Retrieved.',                                min: 1,  max: 99,  step: 1, unit: '',  defaultVal: 10},
    {key: 'updateSignIntervalTime',  label: 'Sign Upload',     desc: 'How often your own summon sign is re-uploaded to the server. Lower = your sign stays fresher for hosts.',   min: 1,  max: 120, step: 1, unit: 's', defaultVal: 30},
    {key: 'singGetMax',              label: 'Sign Get Max',    desc: 'Hard cap on the total number of signs retrievable. Acts as the ceiling for Signs Retrieved.',               min: 1,  max: 128, step: 1, unit: '',  defaultVal: 32},
    {key: 'signDownloadSpan',        label: 'Download Span',   desc: 'Interval between full sign list download cycles.',                                                          min: 1,  max: 120, step: 1, unit: 's', defaultVal: 30},
    {key: 'signUpdateSpan',          label: 'Upload Span',     desc: 'Interval between sign data uploads to the matchmaking server.',                                             min: 1,  max: 120, step: 1, unit: 's', defaultVal: 60},
];

const BLUE_SLIDERS: SliderDef[] = [
    {key: 'reloadVisitListCoolTime',   label: 'Search Cooldown',   desc: 'Cooldown in seconds between Blue Cipher Ring search cycles. Lower = searches for invaded hosts more frequently.', min: 1,  max: 120, step: 1, unit: 's', defaultVal: 20},
    {key: 'reloadSearchCoopBlueMin',   label: 'Reload Min',        desc: 'Minimum delay (seconds) between co-op blue reload searches. Must be ≤ Reload Max.',                              min: 1,  max: 180, step: 1, unit: 's', defaultVal: 30},
    {key: 'reloadSearchCoopBlueMax',   label: 'Reload Max',        desc: 'Maximum delay (seconds) for the reload interval. Actual delay is randomised between Min and Max each cycle.',     min: 1,  max: 300, step: 1, unit: 's', defaultVal: 180},
    {key: 'maxVisitListCount',         label: 'Visit List Size',   desc: 'Number of potential invaded-host targets fetched per search cycle. Higher = more options evaluated.',            min: 1,  max: 50,  step: 1, unit: '',  defaultVal: 5},
    {key: 'allAreaSearchRateCoopBlue', label: 'Global Search %',   desc: 'Percentage chance the blue search covers ALL map areas instead of only the local area. Higher = wider reach.',    min: 0,  max: 100, step: 5, unit: '%', defaultVal: 30},
];

// --- Experimental sliders (shown inside their functional group, never touched by presets) ---

const EXPERIMENTAL_BLUE: SliderDef[] = [
    {key: 'maxCoopBlueSummonCount', label: 'Blue Search Parallelism', desc: 'Client-side blue search parallelism (maxCoopBlueSummonCount). The server caps the actual number of active blues, so raising this rarely helps. Experimental — active Blue presets never change it. Vanilla 2.', min: 1, max: 10, step: 1, unit: '', defaultVal: 2},
    {key: 'allAreaSearchRateVsBlue', label: 'Retribution Global %',   desc: 'Global-search rate for the retribution blue role (allAreaSearchRateVsBlue). Its effect in Elden Ring is unverified. Experimental — active Blue presets never change it. Vanilla 30.', min: 0, max: 100, step: 5, unit: '%', defaultVal: 30},
];

// Visitor / legacy ring-search fields. Hidden from the active UI until a confirmed
// use is found (no confirmed link to Taunter's Tongue). The backend fields and save
// compatibility are kept — paramsToDict / VANILLA_VALUES still carry them, so they
// round-trip untouched; they are simply not rendered and not part of any preset.

// Group keys come from NETWORK_GROUP_KEYS (single source of truth in networkClamp.ts).
// The `sliders` here are UI metadata only; their keys equal NETWORK_GROUP_KEYS[group].
interface GroupMeta {
    label: string;
    icon: string;
    desc: string;
    titleClassName: string;
    sliders: SliderDef[];
    fasterKey: string;
    fasterLabel: string;
    aggressiveKey: string;
    aggressiveLabel: string;
    experimental?: SliderDef[];
}

const GROUP_META: Record<GroupId, GroupMeta> = {
    invader: {
        label: 'Reds / Invader', icon: '⚔', desc: 'Red invasion matchmaking speed (Bloody / Recusant Finger)',
        titleClassName: 'text-red-800 dark:text-red-700',
        sliders: INVADER_SLIDERS,
        fasterKey: 'faster-reds', fasterLabel: 'Faster',
        aggressiveKey: 'aggressive-reds', aggressiveLabel: 'Aggressive',
    },
    cooperator: {
        label: 'Summon Signs', icon: '☀', desc: 'Summon sign refresh & upload. Summoning Pool activation is configured separately in World / Exploration.',
        titleClassName: 'text-orange-800 dark:text-orange-600',
        sliders: COOPERATOR_SLIDERS,
        fasterKey: 'faster-summons', fasterLabel: 'Faster',
        aggressiveKey: 'aggressive-summons', aggressiveLabel: 'Aggressive',
    },
    blue: {
        label: 'Blue / Hunter', icon: '🛡', desc: 'Blue Cipher Ring response time & reach (test hunter-side with an active Blue Cipher Ring)',
        titleClassName: 'text-blue-800 dark:text-blue-700',
        sliders: BLUE_SLIDERS,
        fasterKey: 'faster-blue', fasterLabel: 'Faster',
        aggressiveKey: 'aggressive-blue', aggressiveLabel: 'Aggressive',
        experimental: EXPERIMENTAL_BLUE,
    },
};

const GROUPS: GroupId[] = ['invader', 'cooperator', 'blue'];

// Vanilla baseline (from binary NetworkParam.param — source of truth).
// reloadSignTotalCount=20 / reloadSignCellCount=10 (NOT the old doc value 32/8).
const VANILLA_VALUES: NetDraft = {
    maxBreakInTargetListCount: 5,    breakInRequestIntervalTimeSec: 30, breakInRequestTimeOutSec: 20,  breakInRequestAreaCount: 5,
    reloadSignIntervalTime2: 60,     reloadSignTotalCount: 20,          reloadSignCellCount: 10,        updateSignIntervalTime: 30,
    singGetMax: 32,                  signDownloadSpan: 30,              signUpdateSpan: 60,
    reloadVisitListCoolTime: 20,     maxCoopBlueSummonCount: 2,         maxVisitListCount: 5,
    reloadSearchCoopBlueMin: 30,     reloadSearchCoopBlueMax: 180,      allAreaSearchRateCoopBlue: 30,  allAreaSearchRateVsBlue: 30,
    visitorListMax: 10,              visitorTimeOutTime: 60,            visitorDownloadSpan: 60,
};

function paramsToDict(p: core.NetworkParamValues): NetDraft {
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

function groupMatches(keys: readonly string[], draft: NetDraft, source: NetDraft): boolean {
    return keys.every(k => draft[k] === source[k]);
}

export function NetworkTab({platform}: NetworkTabProps) {
    const [params, setParams] = useState<core.NetworkParamValues | null>(null);
    const [draft, setDraft] = useState<NetDraft>({});
    const [presets, setPresets] = useState<Record<string, NetDraft>>({});
    const [dirty, setDirty] = useState(false);
    const [applying, setApplying] = useState(false);
    const [descModal, setDescModal] = useState<{label: string; desc: string} | null>(null);

    const load = useCallback(() => {
        if (!platform) { setParams(null); return; }
        GetNetworkParams().then(p => {
            setParams(p);
            setDraft(paramsToDict(p));
            setDirty(false);
        }).catch(() => setParams(null));

        // Fetch functional presets from the backend — single source of truth, no drift.
        Promise.all([
            GetNetworkPreset('faster-reds'),
            GetNetworkPreset('faster-summons'),
            GetNetworkPreset('faster-blue'),
            GetNetworkPreset('aggressive-reds'),
            GetNetworkPreset('aggressive-summons'),
            GetNetworkPreset('aggressive-blue'),
            GetNetworkPreset('vanilla'),
        ]).then(([fReds, fSummons, fBlue, aReds, aSummons, aBlue, vanilla]) => {
            setPresets({
                'faster-reds': paramsToDict(fReds),
                'faster-summons': paramsToDict(fSummons),
                'faster-blue': paramsToDict(fBlue),
                'aggressive-reds': paramsToDict(aReds),
                'aggressive-summons': paramsToDict(aSummons),
                'aggressive-blue': paramsToDict(aBlue),
                'vanilla': paramsToDict(vanilla),
            });
        }).catch(() => setPresets({}));
    }, [platform]);

    useEffect(() => { load(); }, [load]);

    const updateDraft = (key: string, value: number) => {
        setDraft(prev => clampNetworkDraft({...prev, [key]: value}));
        setDirty(true);
    };

    // Applies ONLY the group's canonical fields (NETWORK_GROUP_KEYS) — modular,
    // never touches other groups or Experimental fields. Used by Vanilla / Faster / Aggressive.
    const applyGroup = (group: GroupId, which: 'faster' | 'aggressive' | 'vanilla') => {
        const meta = GROUP_META[group];
        let source: NetDraft | undefined;
        if (which === 'faster') source = presets[meta.fasterKey];
        else if (which === 'aggressive') source = presets[meta.aggressiveKey];
        else source = presets['vanilla'] ?? VANILLA_VALUES;
        if (!source) return;
        const resolved = source;
        setDraft(prev => applyGroupPreset(prev, NETWORK_GROUP_KEYS[group], resolved));
        setDirty(true);
    };

    const handleApply = async () => {
        const clamped = clampNetworkDraft(draft);
        const err = networkDraftError(clamped);
        if (err) { toast.error(err); return; }
        setApplying(true);
        try {
            await SetNetworkParams(new core.NetworkParamValues(clamped));
            toast.success('Network params applied. Load character → Exit to menu → Load again to activate.');
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

    if (!platform) {
        return <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Load a save file first</div>;
    }
    if (!params) {
        return <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Loading regulation data...</div>;
    }

    const renderSlider = (s: SliderDef) => (
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
    );

    const makeGroupButtons = (group: GroupId) => {
        const meta = GROUP_META[group];
        const keys = NETWORK_GROUP_KEYS[group];
        const fasterSrc = presets[meta.fasterKey];
        const aggressiveSrc = presets[meta.aggressiveKey];
        const vanillaSrc = presets['vanilla'] ?? VANILLA_VALUES;
        const isVanilla = groupMatches(keys, draft, vanillaSrc);
        const isFaster = fasterSrc ? groupMatches(keys, draft, fasterSrc) : false;
        const isAggressive = aggressiveSrc ? groupMatches(keys, draft, aggressiveSrc) : false;
        const btn = (active: boolean) =>
            `px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-wider transition-all border ${
                active ? 'bg-primary/20 border-primary/60 text-primary'
                       : 'border-border/50 text-muted-foreground/50 hover:border-primary/40 hover:text-foreground'}`;
        return (
            <div className="flex items-center gap-1">
                <button onClick={e => { e.stopPropagation(); applyGroup(group, 'vanilla'); }} className={btn(isVanilla)}>Vanilla</button>
                <button onClick={e => { e.stopPropagation(); applyGroup(group, 'faster'); }} className={btn(isFaster)}>{meta.fasterLabel}</button>
                <button onClick={e => { e.stopPropagation(); applyGroup(group, 'aggressive'); }} className={btn(isAggressive)}>{meta.aggressiveLabel}</button>
            </div>
        );
    };

    return (
        <>
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
                {/* Default-visible functional groups */}
                {GROUPS.map(group => {
                    const meta = GROUP_META[group];
                    return (
                        <AccordionSection
                            key={group}
                            id={`network-${group}`}
                            title={`${meta.icon} ${meta.label}`}
                            titleClassName={meta.titleClassName}
                            summary={meta.desc}
                            headerRight={makeGroupButtons(group)}
                        >
                            <div className="grid grid-cols-2 lg:grid-cols-4 gap-2 pt-1">
                                {meta.sliders.map(renderSlider)}
                            </div>
                            {meta.experimental && (
                                <div className="mt-2 pt-2 border-t border-amber-500/20">
                                    <div className="flex items-center gap-1.5 mb-2">
                                        <span className="px-1.5 py-0.5 rounded text-[8px] font-black uppercase tracking-wider bg-amber-500/15 text-amber-500 border border-amber-500/30">Experimental</span>
                                        <span className="text-[9px] text-muted-foreground/80">Unconfirmed effect — never changed by Vanilla / Faster / Aggressive.</span>
                                    </div>
                                    <div className="grid grid-cols-2 lg:grid-cols-4 gap-2">{meta.experimental.map(renderSlider)}</div>
                                </div>
                            )}
                        </AccordionSection>
                    );
                })}

                {/* Apply / Reset */}
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

                {/* How-to / disclaimer */}
                <div className="space-y-2 pt-1">
                    <div className="rounded-lg border border-amber-700/55 bg-amber-50 p-3 space-y-1 dark:border-amber-500/60 dark:bg-stone-950">
                        <p className="text-sm font-black uppercase tracking-[0.15em] text-stone-950 dark:text-amber-200">Network tuning notice</p>
                        <p className="text-sm text-stone-800 dark:text-stone-100 leading-relaxed">
                            These Network tab settings were tested for 4 weeks without a ban, so the ban risk appears low.
                            This notice applies only to settings on this tab; other save edits may still carry a ban risk.
                            I take no responsibility for any account banned as a result of using these settings.
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
