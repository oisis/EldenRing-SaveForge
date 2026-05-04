import {useEffect, useState} from 'react';
import {GetGraces, SetGraceVisited, GetBosses, SetBossDefeated, GetSummoningPools, SetSummoningPoolActivated, GetColosseums, SetColosseumUnlocked, GetMapProgress, SetMapFlag, SetMapRegionFlags, RevealAllMap, ResetMapExploration, RemoveFogOfWar, GetCookbooks, SetCookbookUnlocked, BulkSetCookbooksUnlocked, GetGestures, SetGestureUnlocked, BulkSetGesturesUnlocked, GetQuestNPCs, GetQuestProgress, SetQuestStep, GetBellBearings, SetBellBearingUnlocked, BulkSetBellBearings, GetWhetblades, SetWhetbladeUnlocked, GetUnlockedRegions, SetRegionUnlocked, BulkSetUnlockedRegions} from '../../wailsjs/go/main/App';
import {db} from '../../wailsjs/go/models';
import {AccordionSection} from './AccordionSection';
import {RiskInfoIcon} from './RiskInfoIcon';
import {RiskActionButton} from './RiskActionButton';
import {RiskSectionBanner} from './RiskSectionBanner';
import {RiskKey} from '../data/riskInfo';

interface WorldTabProps {
    charIdx: number;
    showFlaggedItems?: boolean;
    saveLoadKey?: number;
    onMutate?: () => void;
}

const MiniProgress = ({current, total}: {current: number; total: number}) => {
    const pct = total > 0 ? Math.round((current / total) * 100) : 0;
    return (
        <div className="flex items-center gap-1.5 ml-auto">
            <div className="w-16 h-1.5 bg-border rounded-full overflow-hidden">
                <div className="h-full bg-primary rounded-full transition-all duration-300" style={{width: `${pct}%`}} />
            </div>
            <span className="text-[8px] font-mono text-muted-foreground">{current}/{total}</span>
        </div>
    );
};

const Chk = ({checked, onChange}: {checked: boolean; onChange: (v: boolean) => void}) => (
    <div className="relative flex items-center justify-center">
        <input type="checkbox" checked={checked} onChange={e => onChange(e.target.checked)}
            className="peer appearance-none w-3.5 h-3.5 rounded border border-border bg-background checked:bg-primary checked:border-primary transition-all cursor-pointer" />
        <svg className="absolute w-2 h-2 text-white pointer-events-none hidden peer-checked:block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3.5" d="M5 13l4 4L19 7"></path>
        </svg>
    </div>
);

const ChkX = ({checked, onChange}: {checked: boolean; onChange: (v: boolean) => void}) => (
    <div className="relative flex items-center justify-center">
        <input type="checkbox" checked={checked} onChange={e => onChange(e.target.checked)}
            className="peer appearance-none w-3.5 h-3.5 rounded border border-border bg-background checked:bg-red-500 checked:border-red-500 transition-all cursor-pointer" />
        <svg className="absolute w-2 h-2 text-white pointer-events-none hidden peer-checked:block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3.5" d="M6 18L18 6M6 6l12 12"></path>
        </svg>
    </div>
);

const btnSm = "text-[8px] font-black uppercase tracking-widest text-muted-foreground border border-foreground/30 bg-foreground/5 px-2 py-0.5 rounded transition-all";

export function WorldTab({charIdx, showFlaggedItems, saveLoadKey, onMutate}: WorldTabProps) {
    const [graces, setGraces] = useState<db.GraceEntry[]>([]);
    const [bosses, setBosses] = useState<db.BossEntry[]>([]);
    const [pools, setPools] = useState<db.SummoningPoolEntry[]>([]);
    const [colosseums, setColosseums] = useState<db.ColosseumEntry[]>([]);
    const [mapEntries, setMapEntries] = useState<db.MapEntry[]>([]);
    const [cookbooks, setCookbooks] = useState<db.CookbookEntry[]>([]);
    const [gestures, setGesturesList] = useState<db.GestureEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [expandedRegions, setExpandedRegions] = useState<Record<string, boolean>>({});
    const [expandedBossRegions, setExpandedBossRegions] = useState<Record<string, boolean>>({});
    const [expandedPoolRegions, setExpandedPoolRegions] = useState<Record<string, boolean>>({});
    const [selectedMap, setSelectedMap] = useState<{name: string, path: string} | null>(null);
    const [bossFilter, setBossFilter] = useState<'all' | 'main' | 'field'>('all');
    const [bossSort, setBossSort] = useState<'name' | 'defeated'>('name');
    const [bellBearings, setBellBearings] = useState<db.BellBearingEntry[]>([]);
    const [whetblades, setWhetblades] = useState<db.WhetbladeEntry[]>([]);
    const [regionEntries, setRegionEntries] = useState<db.RegionEntry[]>([]);
    const [expandedBBCategories, setExpandedBBCategories] = useState<Record<string, boolean>>({});
    const [expandedCookbookCategories, setExpandedCookbookCategories] = useState<Record<string, boolean>>({});
    const [expandedRegionAreas, setExpandedRegionAreas] = useState<Record<string, boolean>>({});
    const [expandedMapAreas, setExpandedMapAreas] = useState<Record<string, boolean>>({});
    const [skipBossArenas, setSkipBossArenas] = useState(true);
    const [questNPCs, setQuestNPCs] = useState<string[]>([]);
    const [selectedNPC, setSelectedNPC] = useState<string>('');
    const [questProgress, setQuestProgress] = useState<db.QuestNPC | null>(null);
    const [expandedSteps, setExpandedSteps] = useState<Record<number, boolean>>({});
    const [questLoading, setQuestLoading] = useState(false);
    const [activeSubTab, setActiveSubTab] = useState<'exploration' | 'progress' | 'unlocks'>('exploration');

    const loadData = () => {
        setLoading(true);
        Promise.all([
            GetGraces(charIdx).then(res => setGraces(res || [])),
            GetBosses(charIdx).then(res => setBosses(res || [])),
            GetSummoningPools(charIdx).then(res => setPools(res || [])),
            GetColosseums(charIdx).then(res => setColosseums(res || [])),
            GetMapProgress(charIdx).then(async (res) => {
                const entries = res || [];
                const systemFlags = entries.filter(e => e.category === 'system' && !e.enabled);
                if (systemFlags.length > 0) {
                    await Promise.all(systemFlags.map(e => SetMapFlag(charIdx, e.id, true)));
                    for (const sf of systemFlags) {
                        const idx = entries.findIndex(e => e.id === sf.id);
                        if (idx >= 0) entries[idx] = {...entries[idx], enabled: true};
                    }
                }
                setMapEntries(entries);
            }),
            GetCookbooks(charIdx).then(res => setCookbooks(res || [])),
            GetGestures(charIdx).then(res => setGesturesList(res || [])),
            GetQuestNPCs().then(res => setQuestNPCs(res || [])),
            GetBellBearings(charIdx).then(res => setBellBearings(res || [])),
            GetWhetblades(charIdx).then(res => setWhetblades(res || [])),
            GetUnlockedRegions(charIdx).then(res => setRegionEntries(res || [])),
        ]).finally(() => setLoading(false));
    };
    useEffect(() => { loadData(); }, [charIdx]);

    // --- Grace logic ---
    const regions = graces.reduce((acc, g) => { const r = g.region || 'Unknown'; (acc[r] ??= []).push(g); return acc; }, {} as Record<string, db.GraceEntry[]>);
    const handleGraceToggle = async (grace: db.GraceEntry, visited: boolean) => { await SetGraceVisited(charIdx, grace.id, visited); setGraces(prev => prev.map(g => g.id === grace.id ? {...g, visited} : g)); onMutate?.(); };
    const handleUnlockRegionGraces = async (rg: db.GraceEntry[]) => { await Promise.all(rg.filter(g => !g.visited).map(g => SetGraceVisited(charIdx, g.id, true))); const ids = new Set(rg.map(g => g.id)); setGraces(prev => prev.map(g => ids.has(g.id) ? {...g, visited: true} : g)); onMutate?.(); };
    const handleUnlockAllGraces = async () => { const u = graces.filter(g => !g.visited && (!skipBossArenas || !g.isBossArena)); if (!u.length) return; await Promise.all(u.map(g => SetGraceVisited(charIdx, g.id, true))); const ids = new Set(u.map(g => g.id)); setGraces(prev => prev.map(g => ids.has(g.id) ? {...g, visited: true} : g)); onMutate?.(); };
    const handleLockAllGraces = async () => { const u = graces.filter(g => g.visited); if (!u.length) return; await Promise.all(u.map(g => SetGraceVisited(charIdx, g.id, false))); const ids = new Set(u.map(g => g.id)); setGraces(prev => prev.map(g => ids.has(g.id) ? {...g, visited: false} : g)); onMutate?.(); };

    // --- Boss logic ---
    const filteredBosses = bosses.filter(b => bossFilter === 'all' || b.type === bossFilter);
    const sortedFilteredBosses = [...filteredBosses].sort((a, b) => { if (bossSort === 'defeated' && a.defeated !== b.defeated) return a.defeated ? -1 : 1; return a.name.localeCompare(b.name); });
    const bossRegions = sortedFilteredBosses.reduce((acc, b) => { const r = b.region || 'Unknown'; (acc[r] ??= []).push(b); return acc; }, {} as Record<string, db.BossEntry[]>);
    const handleBossToggle = async (boss: db.BossEntry, defeated: boolean) => { await SetBossDefeated(charIdx, boss.id, defeated); setBosses(prev => prev.map(b => b.id === boss.id ? {...b, defeated} : b)); onMutate?.(); };
    const handleKillAll = async (rb: db.BossEntry[]) => { await Promise.all(rb.filter(b => !b.defeated).map(b => SetBossDefeated(charIdx, b.id, true))); const ids = new Set(rb.map(b => b.id)); setBosses(prev => prev.map(b => ids.has(b.id) ? {...b, defeated: true} : b)); onMutate?.(); };
    const handleRespawnAll = async (rb: db.BossEntry[]) => { await Promise.all(rb.filter(b => b.defeated).map(b => SetBossDefeated(charIdx, b.id, false))); const ids = new Set(rb.map(b => b.id)); setBosses(prev => prev.map(b => ids.has(b.id) ? {...b, defeated: false} : b)); onMutate?.(); };
    const handleGlobalKillAll = async () => { const a = filteredBosses.filter(b => !b.defeated); if (!a.length) return; await Promise.all(a.map(b => SetBossDefeated(charIdx, b.id, true))); const ids = new Set(a.map(b => b.id)); setBosses(prev => prev.map(b => ids.has(b.id) ? {...b, defeated: true} : b)); onMutate?.(); };
    const handleGlobalRespawnAll = async () => { const d = filteredBosses.filter(b => b.defeated); if (!d.length) return; await Promise.all(d.map(b => SetBossDefeated(charIdx, b.id, false))); const ids = new Set(d.map(b => b.id)); setBosses(prev => prev.map(b => ids.has(b.id) ? {...b, defeated: false} : b)); onMutate?.(); };

    // --- Pool logic ---
    const poolRegions = pools.reduce((acc, p) => { const r = p.region || 'Unknown'; (acc[r] ??= []).push(p); return acc; }, {} as Record<string, db.SummoningPoolEntry[]>);
    const handlePoolToggle = async (pool: db.SummoningPoolEntry, activated: boolean) => { await SetSummoningPoolActivated(charIdx, pool.id, activated); setPools(prev => prev.map(p => p.id === pool.id ? {...p, activated} : p)); onMutate?.(); };
    const handleActivateAllPools = async (rp: db.SummoningPoolEntry[]) => { await Promise.all(rp.filter(p => !p.activated).map(p => SetSummoningPoolActivated(charIdx, p.id, true))); const ids = new Set(rp.map(p => p.id)); setPools(prev => prev.map(p => ids.has(p.id) ? {...p, activated: true} : p)); onMutate?.(); };
    const handleGlobalActivateAllPools = async () => { const i = pools.filter(p => !p.activated); if (!i.length) return; await Promise.all(i.map(p => SetSummoningPoolActivated(charIdx, p.id, true))); const ids = new Set(i.map(p => p.id)); setPools(prev => prev.map(p => ids.has(p.id) ? {...p, activated: true} : p)); onMutate?.(); };
    const handleGlobalDeactivateAllPools = async () => { const a = pools.filter(p => p.activated); if (!a.length) return; await Promise.all(a.map(p => SetSummoningPoolActivated(charIdx, p.id, false))); const ids = new Set(a.map(p => p.id)); setPools(prev => prev.map(p => ids.has(p.id) ? {...p, activated: false} : p)); onMutate?.(); };

    // --- Colosseum logic ---
    const handleColosseumToggle = async (c: db.ColosseumEntry, unlocked: boolean) => { await SetColosseumUnlocked(charIdx, c.id, unlocked); setColosseums(prev => prev.map(x => x.id === c.id ? {...x, unlocked} : x)); onMutate?.(); };
    const handleUnlockAllColosseums = async () => { const l = colosseums.filter(c => !c.unlocked); if (!l.length) return; await Promise.all(l.map(c => SetColosseumUnlocked(charIdx, c.id, true))); setColosseums(prev => prev.map(c => ({...c, unlocked: true}))); onMutate?.(); };
    const handleLockAllColosseums = async () => { const u = colosseums.filter(c => c.unlocked); if (!u.length) return; await Promise.all(u.map(c => SetColosseumUnlocked(charIdx, c.id, false))); setColosseums(prev => prev.map(c => ({...c, unlocked: false}))); onMutate?.(); };

    // --- Gesture logic ---
    const handleGestureToggle = async (g: db.GestureEntry, unlocked: boolean) => { await SetGestureUnlocked(charIdx, g.id, unlocked); setGesturesList(prev => prev.map(x => x.id === g.id ? {...x, unlocked} : x)); onMutate?.(); };
    // Visible list = all gestures unless showFlaggedItems is off, in which case ban_risk entries are hidden.
    // Unlock All operates on the visible list and additionally skips ban_risk (so even with the toggle on, a single click never adds cut/pre-order content unless the user picks them individually).
    // Lock All sends every known gesture so legacy "even body-type" garbage in the save also gets cleared.
    const visibleGestures = gestures.filter(g => showFlaggedItems || !g.flags?.includes('ban_risk'));
    const handleUnlockAllGestures = async () => { const l = visibleGestures.filter(g => !g.unlocked && !g.flags?.includes('ban_risk')); if (!l.length) return; await BulkSetGesturesUnlocked(charIdx, l.map(g => g.id), true); const ids = new Set(l.map(g => g.id)); setGesturesList(prev => prev.map(g => ids.has(g.id) ? {...g, unlocked: true} : g)); onMutate?.(); };
    const handleLockAllGestures = async () => { await BulkSetGesturesUnlocked(charIdx, gestures.map(g => g.id), false); setGesturesList(prev => prev.map(g => ({...g, unlocked: false}))); onMutate?.(); };
    const unlockedGestures = visibleGestures.filter(g => g.unlocked).length;

    // --- Cookbook logic ---
    const cookbookCategories = cookbooks.reduce((acc, c) => { const cat = c.category || 'Other'; (acc[cat] ??= []).push(c); return acc; }, {} as Record<string, db.CookbookEntry[]>);
    const handleCookbookToggle = async (c: db.CookbookEntry, unlocked: boolean) => { await SetCookbookUnlocked(charIdx, c.id, unlocked); setCookbooks(prev => prev.map(x => x.id === c.id ? {...x, unlocked} : x)); onMutate?.(); };
    const handleUnlockAllCookbooks = async () => { const l = cookbooks.filter(c => !c.unlocked); if (!l.length) return; await BulkSetCookbooksUnlocked(charIdx, l.map(c => c.id), true); setCookbooks(prev => prev.map(c => ({...c, unlocked: true}))); onMutate?.(); };
    const handleLockAllCookbooks = async () => { const u = cookbooks.filter(c => c.unlocked); if (!u.length) return; await BulkSetCookbooksUnlocked(charIdx, u.map(c => c.id), false); setCookbooks(prev => prev.map(c => ({...c, unlocked: false}))); onMutate?.(); };
    const unlockedCookbooks = cookbooks.filter(c => c.unlocked).length;

    // --- Bell Bearing logic ---
    const bbCategories = bellBearings.reduce((acc, b) => { const cat = b.category || 'Other'; (acc[cat] ??= []).push(b); return acc; }, {} as Record<string, db.BellBearingEntry[]>);
    const handleBBToggle = async (b: db.BellBearingEntry, unlocked: boolean) => { await SetBellBearingUnlocked(charIdx, b.id, unlocked); setBellBearings(prev => prev.map(x => x.id === b.id ? {...x, unlocked} : x)); onMutate?.(); };
    const handleUnlockAllBBs = async () => { const l = bellBearings.filter(b => !b.unlocked); if (!l.length) return; await BulkSetBellBearings(charIdx, l.map(b => b.id), true); setBellBearings(prev => prev.map(b => ({...b, unlocked: true}))); onMutate?.(); };
    const handleLockAllBBs = async () => { const u = bellBearings.filter(b => b.unlocked); if (!u.length) return; await BulkSetBellBearings(charIdx, u.map(b => b.id), false); setBellBearings(prev => prev.map(b => ({...b, unlocked: false}))); onMutate?.(); };
    const unlockedBBs = bellBearings.filter(b => b.unlocked).length;

    // --- Whetblade logic ---
    const handleWBToggle = async (w: db.WhetbladeEntry, unlocked: boolean) => { await SetWhetbladeUnlocked(charIdx, w.id, unlocked); setWhetblades(prev => prev.map(x => x.id === w.id ? {...x, unlocked} : x)); onMutate?.(); };
    const handleUnlockAllWBs = async () => { const l = whetblades.filter(w => !w.unlocked); if (!l.length) return; for (const w of l) { await SetWhetbladeUnlocked(charIdx, w.id, true); } setWhetblades(prev => prev.map(w => ({...w, unlocked: true}))); onMutate?.(); };
    const handleLockAllWBs = async () => { const u = whetblades.filter(w => w.unlocked); if (!u.length) return; for (const w of u) { await SetWhetbladeUnlocked(charIdx, w.id, false); } setWhetblades(prev => prev.map(w => ({...w, unlocked: false}))); onMutate?.(); };
    const unlockedWBs = whetblades.filter(w => w.unlocked).length;

    // --- Quest logic ---
    const loadQuestProgress = async (npc: string) => {
        if (!npc) { setQuestProgress(null); return; }
        setQuestLoading(true);
        try { const p = await GetQuestProgress(charIdx, npc); setQuestProgress(p); setExpandedSteps({}); }
        catch { setQuestProgress(null); }
        finally { setQuestLoading(false); }
    };
    const handleSelectNPC = (npc: string) => { setSelectedNPC(npc); loadQuestProgress(npc); };
    const handleSetQuestStep = async (stepIndex: number) => {
        if (!selectedNPC) return;
        await SetQuestStep(charIdx, selectedNPC, stepIndex);
        await loadQuestProgress(selectedNPC);
        onMutate?.();
    };
    const questCompletedSteps = questProgress?.steps?.filter(s => s.complete).length ?? 0;
    const questTotalSteps = questProgress?.steps?.length ?? 0;

    // --- Map logic ---
    const mapRegionEntries = mapEntries.filter(e => e.category === 'visible');
    const mapSystemEntries = mapEntries.filter(e => e.category === 'system');
    const mapAreas = mapRegionEntries.reduce((acc, e) => { const a = e.area || 'Unknown'; (acc[a] ??= []).push(e); return acc; }, {} as Record<string, db.MapEntry[]>);
    const handleMapRegionToggle = async (entry: db.MapEntry, enabled: boolean) => {
        await SetMapRegionFlags(charIdx, entry.id, enabled);
        if (enabled) await RemoveFogOfWar(charIdx);
        const acquiredId = entry.id + 1000;
        setMapEntries(prev => prev.map(e => { if (e.id === entry.id) return {...e, enabled}; if (e.id === acquiredId && e.category === 'acquired') return {...e, enabled}; return e; }));
        onMutate?.();
    };
    const handleSystemFlagToggle = async (entry: db.MapEntry, enabled: boolean) => { await SetMapFlag(charIdx, entry.id, enabled); setMapEntries(prev => prev.map(e => e.id === entry.id ? {...e, enabled} : e)); onMutate?.(); };
    const handleRevealAllMap = async () => { await RevealAllMap(charIdx); await RemoveFogOfWar(charIdx); setMapEntries(prev => prev.map(e => ({...e, enabled: true}))); onMutate?.(); };
    const handleResetMap = async () => { await ResetMapExploration(charIdx); loadData(); onMutate?.(); };
    const totalMapRegions = mapRegionEntries.length;
    const enabledMapRegions = mapRegionEntries.filter(e => e.enabled).length;

    const REGION_MAP_ALIASES: Record<string, string | null> = {
        'limgrave': 'limgrave', 'limgrave, west': 'limgrave', 'limgrave, east': 'limgrave',
        'liurnia of the lakes': 'liurnia_of_the_lakes', 'liurnia, north': 'liurnia_of_the_lakes', 'liurnia, east': 'liurnia_of_the_lakes', 'liurnia, west': 'liurnia_of_the_lakes',
        'weeping peninsula': null, 'crumbling farum azula': null, "miquella's haligtree": null, 'shadow of the erdtree': null,
    };
    const getRegionMapPath = (region: string): string | null => {
        const k = region.toLowerCase();
        if (k in REGION_MAP_ALIASES) { const v = REGION_MAP_ALIASES[k]; return v ? `maps/${v}.jpg` : null; }
        return `maps/${k.replace(/'/g, '').replace(/,/g, '').replace(/\s+/g, '_')}.jpg`;
    };

    // Stats
    const visitedGraces = graces.filter(g => g.visited).length;
    const defeatedBosses = bosses.filter(b => b.defeated).length;
    const mainBosses = bosses.filter(b => b.type === 'main');
    const defeatedMain = mainBosses.filter(b => b.defeated).length;
    const activatedPools = pools.filter(p => p.activated).length;
    const unlockedColosseums = colosseums.filter(c => c.unlocked).length;

    // --- Invasion Regions ---
    const regionAreas = regionEntries.reduce((acc, r) => {
        const a = r.area || 'Other';
        (acc[a] ??= []).push(r);
        return acc;
    }, {} as Record<string, db.RegionEntry[]>);
    const unlockedRegionsCount = regionEntries.filter(r => r.unlocked).length;

    const handleRegionToggle = async (r: db.RegionEntry, unlocked: boolean) => {
        await SetRegionUnlocked(charIdx, r.id, unlocked);
        setRegionEntries(prev => prev.map(x => x.id === r.id ? {...x, unlocked} : x));
        onMutate?.();
    };
    const handleUnlockAreaRegions = async (area: string) => {
        const next = regionEntries.filter(r => r.unlocked || r.area === area).map(r => r.id);
        await BulkSetUnlockedRegions(charIdx, next);
        setRegionEntries(prev => prev.map(r => r.area === area ? {...r, unlocked: true} : r));
        onMutate?.();
    };
    const handleLockAreaRegions = async (area: string) => {
        const next = regionEntries.filter(r => r.unlocked && r.area !== area).map(r => r.id);
        await BulkSetUnlockedRegions(charIdx, next);
        setRegionEntries(prev => prev.map(r => r.area === area ? {...r, unlocked: false} : r));
        onMutate?.();
    };
    const handleUnlockAllRegions = async () => {
        if (!regionEntries.length) return;
        await BulkSetUnlockedRegions(charIdx, regionEntries.map(r => r.id));
        setRegionEntries(prev => prev.map(r => ({...r, unlocked: true})));
        onMutate?.();
    };
    const handleLockAllRegions = async () => {
        if (!regionEntries.some(r => r.unlocked)) return;
        await BulkSetUnlockedRegions(charIdx, []);
        setRegionEntries(prev => prev.map(r => ({...r, unlocked: false})));
        onMutate?.();
    };

    if (loading) return (
        <div className="py-16 flex flex-col items-center justify-center space-y-3">
            <div className="w-5 h-5 border-2 border-foreground/20 border-t-foreground rounded-full animate-spin" />
            <p className="text-[9px] font-bold text-muted-foreground uppercase tracking-widest">Scanning...</p>
        </div>
    );

    return (
        <div className="flex-1 min-h-0 space-y-3 animate-in fade-in slide-in-from-bottom-4 duration-700 pb-8 overflow-y-auto custom-scrollbar pr-2">
            {/* Map Popover */}
            {selectedMap && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/90 backdrop-blur-sm animate-in fade-in duration-300 p-4 md:p-8" onClick={() => setSelectedMap(null)}>
                    <div className="relative max-w-4xl w-full h-full flex flex-col items-center justify-center animate-in zoom-in-95 duration-300">
                        <img src={selectedMap.path} alt={selectedMap.name} className="max-w-full max-h-full object-contain rounded-lg shadow-2xl border border-border/50" onError={e => (e.currentTarget.src = '/src/assets/images/logo-universal.png')} />
                        <div className="absolute bottom-3 left-1/2 -translate-x-1/2 bg-background/80 backdrop-blur-md px-4 py-2 rounded-full border border-border/50 shadow-xl">
                            <h3 className="text-xs font-black uppercase tracking-widest text-foreground text-center">{selectedMap.name}</h3>
                        </div>
                    </div>
                </div>
            )}

            {/* Sub-tab selector */}
            <div className="flex items-center space-x-1">
                {(['exploration', 'progress', 'unlocks'] as const).map(st => (
                    <button key={st} onClick={() => setActiveSubTab(st)}
                        className={`px-4 py-1.5 rounded-full text-[9px] font-black uppercase tracking-[0.15em] transition-all ${activeSubTab === st ? 'bg-primary text-primary-foreground shadow-md shadow-primary/20' : 'text-muted-foreground hover:text-foreground hover:bg-muted/30'}`}>
                        {st}
                    </button>
                ))}
            </div>

            {/* ═══════════════════════════════════════════
                EXPLORATION SUB-TAB
               ═══════════════════════════════════════════ */}
            {activeSubTab === 'exploration' && (
                <div className="space-y-3 animate-in fade-in duration-200">
                    {/* Map */}
                    <AccordionSection id="world-map" title="Map" progress={{current: enabledMapRegions, total: totalMapRegions}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="map_reveal_full" onConfirm={handleRevealAllMap} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Reveal All</RiskActionButton>
                            <button onClick={handleResetMap} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Reset</button>
                        </>}>
                        <RiskSectionBanner riskKey="map_reveal_full" className="mb-3" />
                        {mapSystemEntries.length > 0 && (
                            <div className="flex items-center flex-wrap gap-x-4 gap-y-1 mb-2 pb-2 border-b border-border/30">
                                {mapSystemEntries.map(e => (
                                    <label key={e.id} className="flex items-center space-x-1.5 group cursor-pointer">
                                        <Chk checked={e.enabled} onChange={v => handleSystemFlagToggle(e, v)} />
                                        <span className={`text-[9px] font-bold uppercase tracking-widest ${e.enabled ? 'text-foreground' : 'text-muted-foreground'}`}>{e.name}</span>
                                    </label>
                                ))}
                            </div>
                        )}
                        <div className="grid grid-cols-2 gap-x-4 gap-y-1">
                            {Object.entries(mapAreas).sort(([a], [b]) => a.localeCompare(b)).map(([area, ae]) => (
                                <div key={area} className="break-inside-avoid">
                                    <button onClick={() => setExpandedMapAreas(p => ({...p, [area]: !p[area]}))}
                                        className="w-full flex items-center gap-1.5 py-1 hover:bg-muted/20 rounded transition-all">
                                        <svg className={`w-2.5 h-2.5 transition-transform ${expandedMapAreas[area] ? 'rotate-90 text-primary' : 'text-muted-foreground'}`}
                                            fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                        <span className="text-[9px] font-bold uppercase tracking-wider text-foreground">{area}</span>
                                        <MiniProgress current={ae.filter(e => e.enabled).length} total={ae.length} />
                                    </button>
                                    {expandedMapAreas[area] && ae.map(e => (
                                        <label key={e.id} className="flex items-center space-x-2 py-0.5 px-4 cursor-pointer hover:bg-muted/20 rounded">
                                            <Chk checked={e.enabled} onChange={v => handleMapRegionToggle(e, v)} />
                                            <span className={`text-[10px] font-semibold truncate ${e.enabled ? 'text-foreground' : 'text-muted-foreground'}`}>{e.name}</span>
                                        </label>
                                    ))}
                                </div>
                            ))}
                        </div>
                    </AccordionSection>

                    {/* Sites of Grace */}
                    <AccordionSection id="world-graces" title="Sites of Grace" progress={{current: visitedGraces, total: graces.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="bulk_grace_unlock" onConfirm={handleUnlockAllGraces} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Unlock All</RiskActionButton>
                            <button onClick={handleLockAllGraces} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Lock All</button>
                            <label className="flex items-center space-x-1 cursor-pointer">
                                <Chk checked={skipBossArenas} onChange={setSkipBossArenas} />
                                <span className="text-[7px] font-black uppercase tracking-widest text-muted-foreground">Skip Bosses</span>
                            </label>
                        </>}>
                        <div className="accordion-grid">
                            {Object.entries(regions).sort().map(([region, rg]) => {
                                const vc = rg.filter(g => g.visited).length;
                                const mapPath = getRegionMapPath(region);
                                return (
                                    <div key={region} className="accordion-grid-item border border-border/50 rounded-lg overflow-hidden">
                                        <div className="flex items-center justify-between px-2 py-1.5 bg-muted/10 hover:bg-muted/20 transition-all">
                                            <button onClick={() => setExpandedRegions(p => ({...p, [region]: !p[region]}))} className="flex items-center gap-1.5 flex-1 text-left">
                                                <svg className={`w-2.5 h-2.5 transition-transform ${expandedRegions[region] ? 'rotate-90 text-primary' : 'text-muted-foreground'}`}
                                                    fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                                <span className="text-[9px] font-bold uppercase tracking-wider">{region}</span>
                                            </button>
                                            <div className="flex items-center gap-1.5">
                                                {vc < rg.length && <button onClick={() => handleUnlockRegionGraces(rg)} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Unlock</button>}
                                                {mapPath && (
                                                    <button onClick={() => setSelectedMap({name: region, path: mapPath})}
                                                        className="w-5 h-5 rounded bg-muted/50 border border-border/50 overflow-hidden hover:border-primary/50 transition-all">
                                                        <img src={mapPath} alt="" className="w-full h-full object-cover opacity-60 hover:opacity-100" onError={e => (e.currentTarget.style.display = 'none')} />
                                                    </button>
                                                )}
                                                <MiniProgress current={vc} total={rg.length} />
                                            </div>
                                        </div>
                                        {expandedRegions[region] && (
                                            <div className="px-2 py-1 space-y-0.5">
                                                {rg.map(g => (
                                                    <label key={g.id} className="flex items-center space-x-2 py-0.5 px-1 rounded hover:bg-muted/30 cursor-pointer">
                                                        <Chk checked={g.visited} onChange={v => handleGraceToggle(g, v)} />
                                                        <span className={`text-[10px] truncate font-semibold ${g.visited ? 'text-foreground' : 'text-muted-foreground'}`}>{g.name}</span>
                                                        {g.isBossArena && <span className="text-[7px] font-black text-amber-500 bg-amber-500/10 px-1 rounded">B</span>}
                                                    </label>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    </AccordionSection>

                    {/* Summoning Pools */}
                    <AccordionSection id="world-pools" title="Summoning Pools" progress={{current: activatedPools, total: pools.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="bulk_summoning_pool" onConfirm={handleGlobalActivateAllPools} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Activate All</RiskActionButton>
                            <button onClick={handleGlobalDeactivateAllPools} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Deactivate All</button>
                        </>}>
                        <div className="accordion-grid">
                            {Object.entries(poolRegions).sort().map(([region, rp]) => {
                                const ac = rp.filter(p => p.activated).length;
                                return (
                                    <div key={region} className="accordion-grid-item border border-border/50 rounded-lg overflow-hidden">
                                        <div className="flex items-center justify-between px-2 py-1.5 bg-muted/10 hover:bg-muted/20 transition-all">
                                            <button onClick={() => setExpandedPoolRegions(p => ({...p, [region]: !p[region]}))} className="flex items-center gap-1.5 flex-1 text-left">
                                                <svg className={`w-2.5 h-2.5 transition-transform ${expandedPoolRegions[region] ? 'rotate-90 text-primary' : 'text-muted-foreground'}`}
                                                    fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                                <span className="text-[9px] font-bold uppercase tracking-wider">{region}</span>
                                            </button>
                                            <div className="flex items-center gap-1.5">
                                                {ac < rp.length && <button onClick={() => handleActivateAllPools(rp)} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Activate</button>}
                                                <MiniProgress current={ac} total={rp.length} />
                                            </div>
                                        </div>
                                        {expandedPoolRegions[region] && (
                                            <div className="px-2 py-1 space-y-0.5">
                                                {rp.map(p => (
                                                    <label key={p.id} className="flex items-center space-x-2 py-0.5 px-1 rounded hover:bg-muted/30 cursor-pointer">
                                                        <Chk checked={p.activated} onChange={v => handlePoolToggle(p, v)} />
                                                        <span className={`text-[10px] truncate font-semibold ${p.activated ? 'text-foreground' : 'text-muted-foreground'}`}>{p.name}</span>
                                                    </label>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    </AccordionSection>

                    {/* Colosseums */}
                    <AccordionSection id="world-colosseums" title="Colosseums" progress={{current: unlockedColosseums, total: colosseums.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="bulk_colosseum" onConfirm={handleUnlockAllColosseums} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Unlock All</RiskActionButton>
                            <button onClick={handleLockAllColosseums} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Lock All</button>
                        </>}>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
                            {colosseums.map(c => (
                                <label key={c.id} className="flex items-center space-x-3 cursor-pointer py-2 px-3 rounded border border-border hover:border-primary/40 transition-all">
                                    <Chk checked={c.unlocked} onChange={v => handleColosseumToggle(c, v)} />
                                    <div>
                                        <p className={`text-[11px] font-black uppercase tracking-wide ${c.unlocked ? 'text-foreground' : 'text-muted-foreground'}`}>{c.name}</p>
                                        <p className="text-[8px] font-bold text-muted-foreground uppercase tracking-widest">{c.region}</p>
                                    </div>
                                </label>
                            ))}
                        </div>
                    </AccordionSection>
                </div>
            )}

            {/* ═══════════════════════════════════════════
                PROGRESS SUB-TAB
               ═══════════════════════════════════════════ */}
            {activeSubTab === 'progress' && (
                <div className="space-y-3 animate-in fade-in duration-200">
                    {/* Bosses */}
                    <AccordionSection id="world-bosses" title="Bosses"
                        progress={{current: defeatedBosses, total: bosses.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="bulk_boss_kill" onConfirm={handleGlobalKillAll} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Kill All</RiskActionButton>
                            <button onClick={handleGlobalRespawnAll} className={`${btnSm} hover:text-green-400 hover:border-green-400/50`}>Respawn All</button>
                            <div className="w-px h-3 bg-border/50" />
                            {(['all', 'main', 'field'] as const).map(f => (
                                <button key={f} onClick={() => setBossFilter(f)}
                                    className={`px-2 py-0.5 rounded text-[7px] font-black uppercase tracking-widest transition-all ${bossFilter === f ? 'bg-muted text-foreground border border-border' : 'text-muted-foreground hover:text-foreground'}`}>{f}</button>
                            ))}
                            <span className="text-[8px] font-bold text-muted-foreground">{defeatedMain}/{mainBosses.length}m</span>
                        </>}>
                        <div className="accordion-grid">
                            {Object.entries(bossRegions).sort().map(([region, rb]) => {
                                const dc = rb.filter(b => b.defeated).length;
                                return (
                                    <div key={region} className="accordion-grid-item border border-border/50 rounded-lg overflow-hidden">
                                        <div className="flex items-center justify-between px-2 py-1.5 bg-muted/10 hover:bg-muted/20 transition-all">
                                            <button onClick={() => setExpandedBossRegions(p => ({...p, [region]: !p[region]}))} className="flex items-center gap-1.5 flex-1 text-left">
                                                <svg className={`w-2.5 h-2.5 transition-transform ${expandedBossRegions[region] ? 'rotate-90 text-primary' : 'text-muted-foreground'}`}
                                                    fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                                <span className="text-[9px] font-bold uppercase tracking-wider">{region}</span>
                                            </button>
                                            <div className="flex items-center gap-1.5">
                                                {dc < rb.length && <button onClick={() => handleKillAll(rb)} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Kill</button>}
                                                {dc > 0 && <button onClick={() => handleRespawnAll(rb)} className={`${btnSm} hover:text-green-400 hover:border-green-400/50`}>Respawn</button>}
                                                <MiniProgress current={dc} total={rb.length} />
                                            </div>
                                        </div>
                                        {expandedBossRegions[region] && (
                                            <div className="px-2 py-1 space-y-0.5">
                                                {rb.map(b => (
                                                    <label key={b.id} className="flex items-center space-x-2 py-0.5 px-1 rounded hover:bg-muted/30 cursor-pointer">
                                                        <ChkX checked={b.defeated} onChange={v => handleBossToggle(b, v)} />
                                                        <span className={`text-[10px] truncate font-semibold ${b.defeated ? 'text-foreground line-through opacity-60' : 'text-muted-foreground'}`}>{b.name}</span>
                                                        {b.remembrance && <span className="text-[7px] font-black text-amber-500">R</span>}
                                                        {b.type === 'main' && !b.remembrance && <span className="text-[7px] font-black text-primary">M</span>}
                                                    </label>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    </AccordionSection>

                    {/* NPC Quests */}
                    <AccordionSection id="world-quests" title="NPC Quests" badge={`${questNPCs.length} NPCs`}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <select value={selectedNPC} onChange={e => handleSelectNPC(e.target.value)}
                                className="bg-background border border-border rounded px-2 py-0.5 text-[10px] font-bold text-foreground focus:outline-none focus:border-primary max-w-[200px]">
                                <option value="">Select NPC...</option>
                                {questNPCs.map(n => <option key={n} value={n}>{n}</option>)}
                            </select>
                            {questProgress && <span className="text-[8px] font-bold text-muted-foreground">{questCompletedSteps}/{questTotalSteps}</span>}
                        </>}>
                        <RiskSectionBanner riskKey="quest_step_skip" className="mb-3" />
                        {!selectedNPC && (
                            <div className="py-6 text-center">
                                <p className="text-[10px] text-muted-foreground font-bold uppercase tracking-wider">Select an NPC to view quest progress</p>
                            </div>
                        )}
                        {questLoading && (
                            <div className="py-6 flex justify-center">
                                <div className="w-4 h-4 border-2 border-foreground/20 border-t-foreground rounded-full animate-spin" />
                            </div>
                        )}
                        {selectedNPC && questProgress && !questLoading && (
                            <div className="space-y-1">
                                {questProgress.steps.map((step, idx) => {
                                    const isExpanded = !!expandedSteps[idx];
                                    const matchedFlags = step.flags?.filter(f => f.current === (f.target === 1)).length ?? 0;
                                    const totalFlags = step.flags?.length ?? 0;
                                    const partial = !step.complete && matchedFlags > 0;
                                    return (
                                        <div key={idx} className={`border rounded-lg overflow-hidden ${step.complete ? 'border-primary/30' : 'border-border/50'}`}>
                                            <div className={`px-2 py-1.5 flex items-start gap-2 ${isExpanded ? 'bg-muted/20 border-b border-border/50' : 'hover:bg-muted/10'}`}>
                                                <span className={`flex-shrink-0 w-5 h-5 rounded-full flex items-center justify-center text-[8px] font-black mt-0.5
                                                    ${step.complete ? 'bg-primary text-primary-foreground' : partial ? 'bg-amber-500/20 text-amber-500 border border-amber-500/40' : 'bg-muted text-muted-foreground border border-border'}`}>
                                                    {step.complete ? <svg className="w-2.5 h-2.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3.5" d="M5 13l4 4L19 7" /></svg> : idx + 1}
                                                </span>
                                                <button onClick={() => setExpandedSteps(p => ({...p, [idx]: !p[idx]}))} className="flex-1 text-left min-w-0">
                                                    <p className={`text-[10px] font-semibold ${step.complete ? 'text-foreground' : 'text-muted-foreground'}`}>{step.description}</p>
                                                    {step.location && <p className="text-[8px] text-muted-foreground/70 font-bold uppercase tracking-widest mt-0.5">{step.location}</p>}
                                                </button>
                                                <div className="flex items-center gap-2 flex-shrink-0">
                                                    {totalFlags > 0 && (
                                                        <button onClick={() => setExpandedSteps(p => ({...p, [idx]: !p[idx]}))}
                                                            className={`text-[7px] font-black uppercase px-1.5 py-0.5 rounded border ${step.complete ? 'text-primary border-primary/50 bg-primary/10' : partial ? 'text-amber-500 border-amber-500/40 bg-amber-500/10' : 'text-muted-foreground border-border bg-muted/50'}`}>
                                                            {matchedFlags}/{totalFlags}
                                                        </button>
                                                    )}
                                                    {!step.complete && <RiskActionButton riskKey="quest_step_skip" onConfirm={() => handleSetQuestStep(idx)} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Set</RiskActionButton>}
                                                </div>
                                            </div>
                                            {isExpanded && step.flags && step.flags.length > 0 && (
                                                <div className="px-2 py-1 bg-muted/5 grid grid-cols-2 gap-x-4 gap-y-0.5">
                                                    {step.flags.map((f, fi) => {
                                                        const matches = f.current === (f.target === 1);
                                                        return (
                                                            <div key={fi} className="flex items-center gap-1.5 py-0.5">
                                                                <span className={`w-1.5 h-1.5 rounded-full ${matches ? 'bg-primary' : 'bg-muted-foreground/30'}`} />
                                                                <span className="text-[8px] font-mono text-muted-foreground/70">{f.id}</span>
                                                                <span className={`text-[7px] font-black uppercase ${matches ? 'text-primary' : 'text-muted-foreground/50'}`}>{f.current ? 'ON' : 'OFF'}</span>
                                                                <span className="text-[7px] text-muted-foreground/40">{'\u2192'}</span>
                                                                <span className="text-[7px] font-black uppercase text-muted-foreground/70">{f.target === 1 ? 'ON' : 'OFF'}</span>
                                                            </div>
                                                        );
                                                    })}
                                                </div>
                                            )}
                                        </div>
                                    );
                                })}
                            </div>
                        )}
                    </AccordionSection>
                </div>
            )}

            {/* ═══════════════════════════════════════════
                UNLOCKS SUB-TAB
               ═══════════════════════════════════════════ */}
            {activeSubTab === 'unlocks' && (
                <div className="space-y-3 animate-in fade-in duration-200">
                    {/* Gestures */}
                    <AccordionSection id="world-gestures" title="Gestures" progress={{current: unlockedGestures, total: visibleGestures.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="bulk_gestures_unlock" onConfirm={handleUnlockAllGestures} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Unlock All</RiskActionButton>
                            <button onClick={handleLockAllGestures} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Lock All</button>
                        </>}>
                        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-x-6 gap-y-0.5">
                            {visibleGestures.map(g => {
                                const banRisk = g.flags?.includes('ban_risk');
                                const riskKey: RiskKey | null = g.flags?.includes('cut_content') ? 'cut_content'
                                    : g.flags?.includes('pre_order') ? 'pre_order'
                                    : g.flags?.includes('dlc_duplicate') ? 'dlc_duplicate'
                                    : banRisk ? 'ban_risk' : null;
                                return (
                                    <div key={g.id} className="flex items-center space-x-2 py-0.5 px-1 rounded hover:bg-muted/30">
                                        <label className="flex items-center space-x-2 flex-1 min-w-0 cursor-pointer">
                                            <Chk checked={g.unlocked} onChange={v => handleGestureToggle(g, v)} />
                                            <span className={`text-[10px] truncate font-semibold ${g.unlocked ? 'text-foreground' : 'text-muted-foreground'}`}>
                                                {g.name}
                                            </span>
                                        </label>
                                        {riskKey && <RiskInfoIcon riskKey={riskKey} />}
                                    </div>
                                );
                            })}
                        </div>
                    </AccordionSection>

                    {/* Cookbooks */}
                    <AccordionSection id="world-cookbooks" title="Cookbooks" progress={{current: unlockedCookbooks, total: cookbooks.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="bulk_cookbook" onConfirm={handleUnlockAllCookbooks} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Unlock All</RiskActionButton>
                            <button onClick={handleLockAllCookbooks} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Lock All</button>
                        </>}>
                        <div className="accordion-grid">
                            {Object.entries(cookbookCategories).sort(([a], [b]) => a.localeCompare(b)).map(([cat, cbs]) => {
                                const uc = cbs.filter(c => c.unlocked).length;
                                return (
                                    <div key={cat} className="accordion-grid-item border border-border/50 rounded-lg overflow-hidden">
                                        <div className="flex items-center justify-between px-2 py-1.5 bg-muted/10 hover:bg-muted/20 transition-all">
                                            <button onClick={() => setExpandedCookbookCategories(p => ({...p, [cat]: !p[cat]}))} className="flex items-center gap-1.5 flex-1 text-left">
                                                <svg className={`w-2.5 h-2.5 transition-transform ${expandedCookbookCategories[cat] ? 'rotate-90 text-primary' : 'text-muted-foreground'}`}
                                                    fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                                <span className="text-[9px] font-bold uppercase tracking-wider">{cat}</span>
                                            </button>
                                            <MiniProgress current={uc} total={cbs.length} />
                                        </div>
                                        {expandedCookbookCategories[cat] && (
                                            <div className="px-2 py-1 space-y-0.5">
                                                {cbs.map(c => (
                                                    <label key={c.id} className="flex items-center space-x-2 py-0.5 px-1 rounded hover:bg-muted/30 cursor-pointer">
                                                        <Chk checked={c.unlocked} onChange={v => handleCookbookToggle(c, v)} />
                                                        <span className={`text-[10px] truncate font-semibold ${c.unlocked ? 'text-foreground' : 'text-muted-foreground'}`}>{c.name}</span>
                                                    </label>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    </AccordionSection>

                    {/* Bell Bearings */}
                    <AccordionSection id="world-bells" title="Bell Bearings" progress={{current: unlockedBBs, total: bellBearings.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="bulk_bell_bearing" onConfirm={handleUnlockAllBBs} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Unlock All</RiskActionButton>
                            <button onClick={handleLockAllBBs} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Lock All</button>
                        </>}>
                        <div className="accordion-grid">
                            {Object.entries(bbCategories).sort(([a], [b]) => a.localeCompare(b)).map(([cat, bbs]) => {
                                const uc = bbs.filter(b => b.unlocked).length;
                                return (
                                    <div key={cat} className="accordion-grid-item border border-border/50 rounded-lg overflow-hidden">
                                        <div className="flex items-center justify-between px-2 py-1.5 bg-muted/10 hover:bg-muted/20 transition-all">
                                            <button onClick={() => setExpandedBBCategories(p => ({...p, [cat]: !p[cat]}))} className="flex items-center gap-1.5 flex-1 text-left">
                                                <svg className={`w-2.5 h-2.5 transition-transform ${expandedBBCategories[cat] ? 'rotate-90 text-primary' : 'text-muted-foreground'}`}
                                                    fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                                <span className="text-[9px] font-bold uppercase tracking-wider">{cat}</span>
                                            </button>
                                            <MiniProgress current={uc} total={bbs.length} />
                                        </div>
                                        {expandedBBCategories[cat] && (
                                            <div className="px-2 py-1 space-y-0.5">
                                                {bbs.map(b => (
                                                    <label key={b.id} className="flex items-center space-x-2 py-0.5 px-1 rounded hover:bg-muted/30 cursor-pointer">
                                                        <Chk checked={b.unlocked} onChange={v => handleBBToggle(b, v)} />
                                                        <span className={`text-[10px] truncate font-semibold ${b.unlocked ? 'text-foreground' : 'text-muted-foreground'}`}>{b.name}</span>
                                                    </label>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    </AccordionSection>

                    {/* Whetblades */}
                    <AccordionSection id="world-whetblades" title="Whetblades" progress={{current: unlockedWBs, total: whetblades.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <button onClick={handleUnlockAllWBs} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Unlock All</button>
                            <button onClick={handleLockAllWBs} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Lock All</button>
                        </>}>
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-0.5">
                            {whetblades.map(w => (
                                <label key={w.id} className="flex items-center space-x-2 py-0.5 px-1 rounded hover:bg-muted/30 cursor-pointer">
                                    <Chk checked={w.unlocked} onChange={v => handleWBToggle(w, v)} />
                                    <span className={`text-[10px] truncate font-semibold ${w.unlocked ? 'text-foreground' : 'text-muted-foreground'}`}>{w.name}</span>
                                </label>
                            ))}
                        </div>
                    </AccordionSection>

                    {/* Invasion Regions */}
                    <AccordionSection id="world-regions" title="Invasion Regions" progress={{current: unlockedRegionsCount, total: regionEntries.length}}
                        resetSignal={saveLoadKey}
                        actions={<>
                            <RiskActionButton riskKey="bulk_region_unlock" onConfirm={handleUnlockAllRegions} className={`${btnSm} hover:text-primary hover:border-primary/50`}>Unlock All</RiskActionButton>
                            <button onClick={handleLockAllRegions} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`}>Lock All</button>
                        </>}>
                        <p className="text-[9px] text-muted-foreground/70 italic px-1 pb-2">
                            Map regions stored in the save's Regions struct. Controls invasion eligibility (PvP / NPC) and blue summons.
                        </p>
                        <div className="accordion-grid">
                            {Object.entries(regionAreas).sort().map(([area, rs]) => {
                                const uc = rs.filter(r => r.unlocked).length;
                                return (
                                    <div key={area} className="accordion-grid-item border border-border/50 rounded-lg overflow-hidden">
                                        <div className="flex items-center justify-between px-2 py-1.5 bg-muted/10 hover:bg-muted/20 transition-all gap-1">
                                            <button onClick={() => setExpandedRegionAreas(p => ({...p, [area]: !p[area]}))} className="flex items-center gap-1.5 flex-1 text-left">
                                                <svg className={`w-2.5 h-2.5 transition-transform ${expandedRegionAreas[area] ? 'rotate-90 text-primary' : 'text-muted-foreground'}`}
                                                    fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                                <span className="text-[9px] font-bold uppercase tracking-wider">{area}</span>
                                            </button>
                                            <MiniProgress current={uc} total={rs.length} />
                                            <button onClick={() => handleUnlockAreaRegions(area)} className={`${btnSm} ml-1 hover:text-primary hover:border-primary/50`} title="Unlock all in area">+</button>
                                            <button onClick={() => handleLockAreaRegions(area)} className={`${btnSm} hover:text-red-400 hover:border-red-400/50`} title="Lock all in area">−</button>
                                        </div>
                                        {expandedRegionAreas[area] && (
                                            <div className="px-2 py-1 space-y-0.5">
                                                {rs.map(r => (
                                                    <label key={r.id} className="flex items-center space-x-2 py-0.5 px-1 rounded hover:bg-muted/10 cursor-pointer">
                                                        <Chk checked={r.unlocked} onChange={v => handleRegionToggle(r, v)} />
                                                        <span className={`text-[10px] truncate font-semibold ${r.unlocked ? 'text-foreground' : 'text-muted-foreground'}`}>{r.name}</span>
                                                    </label>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    </AccordionSection>
                </div>
            )}
        </div>
    );
}
