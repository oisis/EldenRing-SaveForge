import {useState, useEffect, useCallback} from 'react';
import {EventsOn} from '../wailsjs/runtime/runtime';
import toast from './lib/toast';
import {SelectAndOpenSave, GetActiveSlots, SetSlotActivity, GetCharacterNames, WriteSave, CloneSlot, DeleteSlot, GetCharacter, RevertSlot, GetUndoDepth, GetSlotDiff, GetSaveDiffSummary, GetInfuseTypes, GetSlotCapacity} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {CharacterTab} from './components/CharacterTab';
import {InventoryTab} from './components/InventoryTab';
import {WorldTab} from './components/WorldTab';
import {ToolsTab} from './components/ToolsTab';
import {SettingsTab} from './components/SettingsTab';
import {DatabaseTab} from './components/DatabaseTab';
import {AppearanceTab} from './components/AppearanceTab';

import {ToastBar} from './components/ToastBar';
import {SafetyModeBanner} from './components/SafetyModeBanner';
import {db} from '../wailsjs/go/models';

type Theme = 'light' | 'dark' | 'golden';

export type AddSettings = {
    upgrade25: number;
    upgrade10: number;
    infuseOffset: number;
    upgradeAsh: number;
    talismansHighestOnly: boolean;
    includeAshenCapital: boolean;
};

const DEFAULT_ADD_SETTINGS: AddSettings = { upgrade25: 0, upgrade10: 0, infuseOffset: 0, upgradeAsh: 0, talismansHighestOnly: false, includeAshenCapital: false };

function App() {
    const [platform, setPlatform] = useState<string | null>(null);
    const [activeSlots, setActiveSlots] = useState<boolean[]>([]);
    const [charNames, setCharacterNames] = useState<string[]>([]);
    const [selectedChar, setSelectedChar] = useState<number>(0);
    const [activeTab, setActiveTab] = useState('character');
    const [inventoryVersion, setInventoryVersion] = useState(0);
    const [saveLoadKey, setSaveLoadKey] = useState(0);
    const [theme, setTheme] = useState<Theme>(() => {
        return (localStorage.getItem('setting:theme') as Theme) || 'dark';
    });
    const [cloneModal, setCloneModal] = useState<{srcIdx: number} | null>(null);
    const [charAddSettings, setCharAddSettings] = useState<Record<number, AddSettings>>({});
    const [columnVisibility, setColumnVisibility] = useState(() => {
        try {
            const saved = localStorage.getItem('setting:columnVisibility');
            return saved ? JSON.parse(saved) : { id: false, category: true };
        } catch { return { id: false, category: true }; }
    });
    const [showFlaggedItems, setShowFlaggedItems] = useState<boolean>(() => {
        return localStorage.getItem('setting:showFlaggedItems') === 'true';
    });
    const [category, setCategory] = useState('all');
    const [charWarnings, setCharWarnings] = useState<string[]>([]);
    const [debugMode, setDebugMode] = useState<boolean>(() => {
        return localStorage.getItem('setting:debugMode') === 'true';
    });
    const [undoDepth, setUndoDepth] = useState(0);
    const [diffModal, setDiffModal] = useState(false);
    const [diffSummary, setDiffSummary] = useState<main.SlotDiffSummary[]>([]);
    const [diffDetails, setDiffDetails] = useState<Record<number, main.DiffEntry[]>>({});
    const [diffExpanded, setDiffExpanded] = useState<Record<number, boolean>>({});
    const [selectedDeployTarget, setSelectedDeployTarget] = useState<string>(() => localStorage.getItem('selectedDeployTarget') || '');
    const [targetPlatform, setTargetPlatform] = useState<string>('PC');
    const [showEmptySlots, setShowEmptySlots] = useState(false);
    const [showOnlyFavorites, setShowOnlyFavorites] = useState(false);
    const [infuseTypes, setInfuseTypes] = useState<db.InfuseType[]>([]);
    const [invView, setInvView] = useState<'inventory' | 'database'>('inventory');
    const [detailItem, setDetailItem] = useState<db.ItemEntry | null>(null);
    const [exporting, setExporting] = useState(false);
    const [capacity, setCapacity] = useState<main.SlotCapacity | null>(null);

    const refreshUndoDepth = useCallback(() => {
        if (!platform) { setUndoDepth(0); return; }
        GetUndoDepth(selectedChar).then(setUndoDepth).catch(() => setUndoDepth(0));
    }, [platform, selectedChar]);

    useEffect(() => { refreshUndoDepth(); }, [refreshUndoDepth, inventoryVersion]);

    const tabs = platform
        ? ['character', 'inventory', 'world', 'tools', 'settings']
        : ['character', 'inventory', 'settings'];

    useEffect(() => { localStorage.setItem('setting:theme', theme); }, [theme]);
    useEffect(() => { GetInfuseTypes().then(res => setInfuseTypes(res || [])); }, []);
    useEffect(() => { localStorage.setItem('setting:columnVisibility', JSON.stringify(columnVisibility)); }, [columnVisibility]);
    useEffect(() => { localStorage.setItem('setting:showFlaggedItems', String(showFlaggedItems)); }, [showFlaggedItems]);
    useEffect(() => { localStorage.setItem('setting:debugMode', String(debugMode)); }, [debugMode]);
    useEffect(() => { localStorage.setItem('selectedDeployTarget', selectedDeployTarget); }, [selectedDeployTarget]);

    useEffect(() => {
        return EventsOn('app:log', (level: string, msg: string) => {
            if (level === 'error') toast.error(msg);
            else toast(msg);
        });
    }, []);

    useEffect(() => {
        document.documentElement.setAttribute('data-theme', theme);
    }, [theme]);

    useEffect(() => {
        if (!platform) return;
        if (charAddSettings[selectedChar] !== undefined) return;
        GetCharacter(selectedChar).then(char => {
            if (!char) return;
            const all = [...(char.inventory || []), ...(char.storage || [])];
            const maxOf = (pred: (i: any) => boolean) =>
                all.filter(pred).reduce((m, i) => Math.max(m, i.currentUpgrade ?? 0), 0);
            setCharAddSettings(prev => ({
                ...prev,
                [selectedChar]: {
                    upgrade25: maxOf(i => i.maxUpgrade === 25),
                    upgrade10: maxOf(i => i.maxUpgrade === 10 && i.category === 'Weapon'),
                    infuseOffset: 0,
                    upgradeAsh: maxOf(i => i.subCategory === 'ashes'),
                    talismansHighestOnly: false,
                    includeAshenCapital: false,
                },
            }));
        }).catch(() => {});
    }, [selectedChar, platform]);

    useEffect(() => {
        if (!platform) return;
        GetCharacter(selectedChar).then(char => {
            setCharWarnings(char?.warnings || []);
        }).catch(() => setCharWarnings([]));
    }, [selectedChar, platform]);

    // Capacity bar — fetched centrally so it can be rendered next to the
    // Inventory/Database toggle pills (header consolidation, spec/36).
    useEffect(() => {
        if (!platform) { setCapacity(null); return; }
        GetSlotCapacity(selectedChar).then(setCapacity).catch(() => setCapacity(null));
    }, [selectedChar, platform, inventoryVersion]);

    const handleOpenSave = async () => {
        try {
            const plat = await SelectAndOpenSave();
            setPlatform(plat);
            setSaveLoadKey(k => k + 1);
            refreshSlots();
        } catch (err) {
            toast.error(String(err));
        }
    };

    const handleExport = async () => {
        setExporting(true);
        try { await WriteSave(targetPlatform); toast.success(`Exported as ${targetPlatform}`); }
        catch (err) { toast.error(String(err)); }
        finally { setExporting(false); }
    };

    const refreshSlots = async () => {
        const slots = await GetActiveSlots();
        const names = await GetCharacterNames();
        setActiveSlots(slots);
        setCharacterNames(names);
        refreshUndoDepth();
    };

    const toggleSlot = async (idx: number) => {
        await SetSlotActivity(idx, !activeSlots[idx]);
        refreshSlots();
    };

    const handleClone = async (srcIdx: number, destIdx: number) => {
        try {
            await CloneSlot(srcIdx, destIdx);
            refreshSlots();
            setCloneModal(null);
        } catch (err) {
            toast.error(String(err));
        }
    };

    const handleRevert = async () => {
        try {
            await RevertSlot(selectedChar);
            refreshUndoDepth();
            setInventoryVersion(v => v + 1);
            toast.success('Reverted to previous state');
        } catch (err) {
            toast.error(String(err));
        }
    };

    const handleReviewChanges = async () => {
        try {
            const summary = await GetSaveDiffSummary();
            setDiffSummary(summary || []);
            setDiffDetails({});
            setDiffExpanded({});
            setDiffModal(true);
        } catch (err) {
            toast.error(String(err));
        }
    };

    const loadSlotDiff = async (slotIdx: number) => {
        if (diffDetails[slotIdx]) {
            setDiffExpanded(prev => ({...prev, [slotIdx]: !prev[slotIdx]}));
            return;
        }
        try {
            const diffs = await GetSlotDiff(slotIdx);
            setDiffDetails(prev => ({...prev, [slotIdx]: diffs || []}));
            setDiffExpanded(prev => ({...prev, [slotIdx]: true}));
        } catch (err) {
            toast.error(String(err));
        }
    };

    const handleDelete = async (idx: number) => {
        const name = charNames[idx];
        if (!confirm(`Delete "${name}"? This cannot be undone.`)) return;
        try {
            await DeleteSlot(idx);
            if (selectedChar > 0 && selectedChar >= idx) setSelectedChar(selectedChar - 1);
            refreshSlots();
        } catch (err) {
            toast.error(String(err));
        }
    };

    return (
        <div className="flex flex-col h-screen bg-background text-foreground overflow-hidden font-sans selection:bg-primary/30 transition-colors duration-300">
            <SafetyModeBanner />
            <div className="flex flex-1 min-h-0">
            {/* Sidebar */}
            <aside className="w-64 border-r border-border bg-muted/5 flex flex-col z-20">
                <div className="p-5 space-y-6 flex-1 overflow-y-auto custom-scrollbar">
                    <div className="flex items-center justify-between px-1">
                        <div className="flex items-center space-x-3">
                            <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center shadow-lg shadow-primary/20">
                                <span className="text-primary-foreground font-black text-lg tracking-tighter">ER</span>
                            </div>
                            <div>
                                <h1 className="text-[10px] font-black tracking-[0.2em] leading-none">Elden Ring SaveForge</h1>
                                <p className="text-[7px] text-primary tracking-wider mt-0.5">by OiSiSk</p>
                            </div>
                        </div>
                    </div>

                    <button 
                        onClick={handleOpenSave}
                        className="w-full bg-primary text-primary-foreground font-black py-3 rounded-lg text-[9px] uppercase tracking-[0.2em] shadow-xl shadow-primary/20 hover:brightness-110 active:scale-95 transition-all"
                    >
                        {platform ? `Change Save (${platform})` : 'Open Save File'}
                    </button>

                    {platform && (
                        <div className="space-y-4 animate-in fade-in slide-in-from-left-2 duration-500">
                            <div className="flex items-center justify-between px-1">
                                <h2 className="text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground">Characters</h2>
                                <span className="text-[8px] font-bold bg-muted/30 px-2 py-0.5 rounded-full text-muted-foreground">{activeSlots.filter(s => s).length}/10</span>
                            </div>
                            {/* Active slots */}
                            <div className="space-y-1">
                                {charNames.map((name, idx) => {
                                    if (!activeSlots[idx]) return null;
                                    return (
                                        <div key={idx} onClick={() => setSelectedChar(idx)}
                                            className={`group relative p-2.5 rounded-lg border transition-all cursor-pointer ${selectedChar === idx ? 'bg-muted/30 border-primary/40 ring-1 ring-primary/10 shadow-lg' : 'bg-transparent border-border/30 hover:border-border hover:bg-muted/10'}`}>
                                            <div className="flex items-center justify-between relative z-10">
                                                <div className="flex items-center space-x-2.5 min-w-0">
                                                    <div className="w-1.5 h-1.5 flex-shrink-0 rounded-full bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.5)]" />
                                                    <span className={`text-[10px] font-bold uppercase tracking-tight truncate transition-colors ${selectedChar === idx ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground'}`}>{name}</span>
                                                </div>
                                                <div className="flex items-center gap-0.5 flex-shrink-0 ml-1">
                                                    <button onClick={(e) => { e.stopPropagation(); setCloneModal({srcIdx: idx}); }} title="Clone character"
                                                        className="p-1 rounded-md opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-primary hover:bg-primary/10">
                                                        <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path></svg>
                                                    </button>
                                                    <button onClick={(e) => { e.stopPropagation(); handleDelete(idx); }} title="Delete character"
                                                        className="p-1 rounded-md opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-red-500 hover:bg-red-500/10">
                                                        <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                                                    </button>
                                                    <button onClick={(e) => { e.stopPropagation(); toggleSlot(idx); }}
                                                        className="p-1 rounded-md transition-all text-green-500 hover:bg-green-500/10">
                                                        <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3" d="M5 13l4 4L19 7"></path></svg>
                                                    </button>
                                                </div>
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>

                            {/* Empty slots toggle */}
                            {activeSlots.filter(s => !s).length > 0 && (
                                <>
                                    <button onClick={() => setShowEmptySlots(v => !v)}
                                        className="flex items-center gap-1.5 px-1 py-1 text-[9px] font-bold text-muted-foreground hover:text-foreground transition-colors w-full">
                                        <svg className={`w-2.5 h-2.5 transition-transform duration-200 ${showEmptySlots ? 'rotate-90' : ''}`}
                                            fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                        <span className="uppercase tracking-widest">Empty Slots</span>
                                        <span className="text-[8px] bg-muted/30 px-1.5 py-0.5 rounded-full">{activeSlots.filter(s => !s).length}</span>
                                    </button>
                                    {showEmptySlots && (
                                        <div className="space-y-1 animate-in fade-in slide-in-from-top-1 duration-200">
                                            {charNames.map((name, idx) => {
                                                if (activeSlots[idx]) return null;
                                                return (
                                                    <div key={idx} onClick={() => setSelectedChar(idx)}
                                                        className={`group relative p-2.5 rounded-lg border transition-all cursor-pointer ${selectedChar === idx ? 'bg-muted/30 border-primary/40 ring-1 ring-primary/10 shadow-lg' : 'bg-transparent border-border/30 hover:border-border hover:bg-muted/10'}`}>
                                                        <div className="flex items-center justify-between relative z-10">
                                                            <div className="flex items-center space-x-2.5 min-w-0">
                                                                <div className="w-1.5 h-1.5 flex-shrink-0 rounded-full bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.3)]" />
                                                                <span className="text-[9px] text-muted-foreground/50 mr-1">{idx + 1}</span>
                                                                <span className={`text-[10px] font-bold uppercase tracking-tight truncate text-muted-foreground/60 italic ${selectedChar === idx ? 'text-foreground' : 'group-hover:text-foreground'}`}>{name}</span>
                                                            </div>
                                                            <button onClick={(e) => { e.stopPropagation(); toggleSlot(idx); }}
                                                                className="p-1 rounded-md transition-all text-red-500 hover:bg-red-500/10">
                                                                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3" d="M6 18L18 6M6 6l12 12"></path></svg>
                                                            </button>
                                                        </div>
                                                    </div>
                                                );
                                            })}
                                        </div>
                                    )}
                                </>
                            )}
                        </div>
                    )}
                </div>
                
                <div className="p-4 border-t border-border bg-muted/5 space-y-3">
                    {platform && (
                        <div className="space-y-1.5">
                            <div className="flex bg-muted/30 p-0.5 rounded border border-border">
                                {(['PC', 'PS4'] as const).map(p => (
                                    <button key={p} onClick={() => setTargetPlatform(p)}
                                        className={`flex-1 py-1 rounded text-[8px] font-black uppercase tracking-widest transition-all ${targetPlatform === p ? 'bg-background text-foreground shadow-sm ring-1 ring-border' : 'text-muted-foreground hover:text-foreground'}`}
                                    >{p}</button>
                                ))}
                            </div>
                            <button onClick={handleExport} disabled={exporting}
                                className="w-full bg-primary text-primary-foreground font-black py-2 rounded-lg text-[8px] uppercase tracking-[0.15em] shadow-lg shadow-primary/20 hover:brightness-110 active:scale-95 transition-all disabled:opacity-50 flex items-center justify-center space-x-1.5">
                                {exporting ? <div className="w-3 h-3 border-2 border-primary-foreground/20 border-t-primary-foreground rounded-full animate-spin" /> :
                                <><svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4"></path></svg><span>Export as {targetPlatform}</span></>}
                            </button>
                        </div>
                    )}
                    <div className="flex items-center justify-between text-[8px] font-bold text-muted-foreground uppercase tracking-widest opacity-50">
                        <span>v0.5.0</span>
                        <span>System Ready</span>
                    </div>
                </div>
            </aside>

            {/* Main Content */}
            <main className="flex-1 flex flex-col relative z-10 bg-background overflow-hidden">
                <header className="h-14 border-b border-border flex items-center justify-between px-8 bg-background/50 backdrop-blur-md sticky top-0 z-30">
                    <nav className="flex space-x-1">
                        {tabs.map(tab => (
                            <button
                                key={tab}
                                onClick={() => {
                                    if (tab === 'inventory') setInventoryVersion(v => v + 1);
                                    setActiveTab(tab);
                                }}
                                className={`px-4 py-1.5 rounded-full text-[9px] font-black uppercase tracking-[0.2em] transition-all ${activeTab === tab ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/20' : 'text-muted-foreground hover:text-foreground hover:bg-muted/30'}`}
                            >
                                {tab}
                            </button>
                        ))}
                    </nav>
                    <div className="flex items-center space-x-4">
                        {platform && (
                            <button
                                onClick={handleReviewChanges}
                                title="Review all pending changes before saving"
                                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-cyan-500/40 bg-cyan-500/10 text-cyan-600 hover:bg-cyan-500/20 transition-all text-[9px] font-black uppercase tracking-widest"
                            >
                                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" /></svg>
                                <span>Review</span>
                            </button>
                        )}
                        {undoDepth > 0 && (
                            <button
                                onClick={handleRevert}
                                title={`Undo last change (${undoDepth} left)`}
                                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-yellow-500/40 bg-yellow-500/10 text-yellow-600 hover:bg-yellow-500/20 transition-all text-[9px] font-black uppercase tracking-widest"
                            >
                                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M3 10h10a5 5 0 015 5v2M3 10l4-4m-4 4l4 4" /></svg>
                                <span>Undo ({undoDepth})</span>
                            </button>
                        )}
                    </div>
                </header>

                <div className="flex-1 flex flex-col min-h-0 relative">
                    <div className="w-full h-full p-6 flex flex-col min-h-0">
                        {activeTab === 'settings' ? (
                            <div className="animate-in fade-in slide-in-from-bottom-2 duration-500 overflow-y-auto custom-scrollbar pr-2">
                                <SettingsTab
                                    theme={theme}
                                    setTheme={setTheme}
                                    columnVisibility={columnVisibility}
                                    setColumnVisibility={setColumnVisibility}
                                    showFlaggedItems={showFlaggedItems}
                                    setShowFlaggedItems={setShowFlaggedItems}
                                    debugMode={debugMode}
                                    setDebugMode={setDebugMode}
                                    platform={platform}
                                    setPlatform={setPlatform}
                                    refreshSlots={refreshSlots}
                                    selectedDeployTarget={selectedDeployTarget}
                                    setSelectedDeployTarget={setSelectedDeployTarget}
                                />
                            </div>
                        ) : !platform ? (
                            <div className="flex-1 flex flex-col min-h-0 animate-in fade-in slide-in-from-bottom-2 duration-500">
                                <div className="mb-3 flex items-center justify-between gap-3 px-1 shrink-0">
                                    <p className="text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground">
                                        Preview mode — load a save file to enable editing
                                    </p>
                                    <button
                                        onClick={handleOpenSave}
                                        className="px-4 py-1.5 bg-primary text-primary-foreground rounded-full text-[9px] font-black uppercase tracking-[0.2em] transition-all shadow-lg shadow-primary/20 hover:brightness-110 active:scale-95"
                                    >
                                        Open Save
                                    </button>
                                </div>
                                <div className={activeTab === 'inventory' ? 'flex-1 flex flex-col min-h-0 overflow-hidden' : 'flex-1 overflow-y-auto custom-scrollbar'}>
                                    {activeTab === 'character' && (
                                        <AppearanceTab charIndex={0} onMutate={() => {}} readOnly />
                                    )}
                                    {activeTab === 'inventory' && (
                                        <DatabaseTab
                                            columnVisibility={columnVisibility}
                                            platform={null}
                                            charIndex={0}
                                            inventoryVersion={0}
                                            addSettings={DEFAULT_ADD_SETTINGS}
                                            showFlaggedItems={showFlaggedItems}
                                            category={category}
                                            setCategory={setCategory}
                                            onSelectItem={setDetailItem}
                                            selectedDetailItem={detailItem}
                                            onCloseDetail={() => setDetailItem(null)}
                                            readOnly
                                            showOnlyFavorites={showOnlyFavorites}
                                        />
                                    )}
                                </div>
                            </div>
                        ) : (
                            <div className="flex-1 flex flex-col min-h-0 animate-in fade-in slide-in-from-bottom-2 duration-500">
                                {(() => {
                                    const debugPatterns = ['clamped to', 'using fallback offset'];
                                    const isDebugWarning = (w: string) => debugPatterns.some(p => w.toLowerCase().includes(p));
                                    const visibleWarnings = debugMode ? charWarnings : charWarnings.filter(w => !isDebugWarning(w));
                                    return visibleWarnings.length > 0 && (
                                        <div className="mx-4 mt-2 p-3 bg-yellow-500/10 border border-yellow-500/30 rounded-lg relative">
                                            <button onClick={() => setCharWarnings([])} className="absolute top-2 right-2 text-yellow-600/60 hover:text-yellow-600 transition-colors">
                                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" /></svg>
                                            </button>
                                            <p className="text-[10px] font-bold text-yellow-600 uppercase tracking-widest">Save loaded with warnings</p>
                                            <ul className="mt-1 text-[9px] text-yellow-600/80 list-disc list-inside">
                                                {visibleWarnings.map((w, i) => <li key={i}>{w}</li>)}
                                            </ul>
                                        </div>
                                    );
                                })()}
                                <div className={activeTab === 'inventory' ? 'flex-1 flex flex-col min-h-0 overflow-hidden' : 'flex-1 overflow-y-auto custom-scrollbar'}>
                                {activeTab === 'character' && <CharacterTab charIndex={selectedChar} onNameChange={refreshSlots} onMutate={refreshUndoDepth} refreshKey={inventoryVersion} addSettings={charAddSettings[selectedChar] ?? DEFAULT_ADD_SETTINGS} onAddSettingsChange={(s) => setCharAddSettings(prev => ({...prev, [selectedChar]: s}))} infuseTypes={infuseTypes} />}
                                {activeTab === 'inventory' && (
                                    <div className="flex-1 flex flex-col min-h-0">
                                        {/* Header consolidation (spec/36): toggle pills + capacity bar (Inventory) OR
                                            Add Settings accordion (Database) on a single row. */}
                                        {(() => {
                                            const togglePills = (
                                                <div className="flex items-center gap-1 shrink-0">
                                                    {(['inventory', 'database'] as const).map(v => (
                                                        <button key={v} onClick={() => { setInvView(v); if (v === 'inventory') setDetailItem(null); }}
                                                            className={`px-3 py-1 rounded-full text-[9px] font-black uppercase tracking-[0.12em] transition-all ${invView === v ? 'bg-primary text-primary-foreground shadow-md shadow-primary/20' : 'text-muted-foreground hover:text-foreground hover:bg-muted/30'}`}>
                                                            {v === 'inventory' ? 'Inventory' : 'Item Database'}
                                                        </button>
                                                    ))}
                                                </div>
                                            );

                                            const favToggle = (
                                                <button
                                                    onClick={() => setShowOnlyFavorites(v => !v)}
                                                    className={`flex items-center gap-1 px-2.5 py-1 rounded-full text-[9px] font-black uppercase tracking-[0.12em] transition-all ${showOnlyFavorites ? 'bg-blue-900/30 text-blue-800 border border-blue-800/50' : 'text-muted-foreground hover:text-blue-800 hover:bg-blue-900/10 border border-transparent'}`}
                                                >
                                                    <svg className={`w-3 h-3 ${showOnlyFavorites ? 'fill-amber-600' : 'fill-none stroke-amber-600'}`} stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2">
                                                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                                                    </svg>
                                                    Favorites
                                                </button>
                                            );

                                            if (invView === 'inventory') {
                                                return (
                                                    <div className="flex items-center gap-4 mb-3 shrink-0">
                                                        {togglePills}
                                                        {favToggle}
                                                        {capacity && (
                                                            <div className="flex flex-wrap items-center gap-3 flex-1">
                                                                {[
                                                                    { label: 'All Items', used: capacity.gaItemsUsed, max: capacity.gaItemsMax },
                                                                    { label: 'Inventory', used: capacity.inventoryUsed, max: capacity.inventoryMax },
                                                                    { label: 'Storage', used: capacity.storageUsed, max: capacity.storageMax },
                                                                ].map(({ label, used, max }) => {
                                                                    const pct = max > 0 ? (used / max) * 100 : 0;
                                                                    const color = pct >= 95 ? 'bg-red-500' : pct >= 80 ? 'bg-amber-500' : 'bg-primary';
                                                                    return (
                                                                        <div key={label} className="flex items-center gap-2 min-w-[170px]">
                                                                            <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground whitespace-nowrap text-right">{label}</span>
                                                                            <div className="flex-1 h-1.5 bg-muted/30 rounded-full overflow-hidden">
                                                                                <div className={`h-full ${color} rounded-full transition-all duration-500`} style={{ width: `${Math.min(pct, 100)}%` }} />
                                                                            </div>
                                                                            <span className={`text-[9px] font-bold tabular-nums ${pct >= 95 ? 'text-red-400' : 'text-muted-foreground'}`}>{used}/{max}</span>
                                                                        </div>
                                                                    );
                                                                })}
                                                            </div>
                                                        )}
                                                    </div>
                                                );
                                            }

                                            // Database view: pills + fav toggle only (Add Settings moved to Character tab).
                                            return (
                                                <div className="flex items-center gap-2 mb-3 shrink-0">
                                                    {togglePills}{favToggle}
                                                    {detailItem && (
                                                        <button onClick={() => setDetailItem(null)}
                                                            className="ml-auto text-[8px] font-bold uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors">
                                                            Close Detail
                                                        </button>
                                                    )}
                                                </div>
                                            );
                                        })()}

                                        {invView === 'inventory' ? (
                                            <InventoryTab charIndex={selectedChar} inventoryVersion={inventoryVersion} columnVisibility={columnVisibility} showFlaggedItems={showFlaggedItems} category={category} setCategory={setCategory} onMutate={refreshUndoDepth} showOnlyFavorites={showOnlyFavorites} />
                                        ) : (
                                            <DatabaseTab
                                                columnVisibility={columnVisibility}
                                                platform={platform}
                                                charIndex={selectedChar}
                                                inventoryVersion={inventoryVersion}
                                                onItemsAdded={() => setInventoryVersion(v => v + 1)}
                                                addSettings={charAddSettings[selectedChar] ?? DEFAULT_ADD_SETTINGS}
                                                showFlaggedItems={showFlaggedItems}
                                                category={category}
                                                setCategory={setCategory}
                                                onSelectItem={setDetailItem}
                                                selectedDetailItem={detailItem}
                                                onCloseDetail={() => setDetailItem(null)}
                                                showOnlyFavorites={showOnlyFavorites}
                                            />
                                        )}
                                    </div>
                                )}
                                {activeTab === 'world' && <WorldTab charIdx={selectedChar} platform={platform} showFlaggedItems={showFlaggedItems} saveLoadKey={saveLoadKey} onMutate={refreshUndoDepth} addSettings={charAddSettings[selectedChar] ?? DEFAULT_ADD_SETTINGS} />}
                                {activeTab === 'tools' && <ToolsTab charIndex={selectedChar} onComplete={refreshSlots} onMutate={() => { setInventoryVersion(v => v + 1); setSaveLoadKey(k => k + 1); refreshSlots(); refreshUndoDepth(); }} addSettings={charAddSettings[selectedChar] ?? DEFAULT_ADD_SETTINGS} onAddSettingsApplied={(s) => setCharAddSettings(prev => ({...prev, [selectedChar]: s}))} />}
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            </main>
        {diffModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setDiffModal(false)}>
                <div className="bg-background border border-border rounded-xl p-6 w-[560px] max-h-[80vh] flex flex-col shadow-2xl" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-between mb-4">
                        <div className="flex items-center space-x-2">
                            <div className="w-1 h-4 bg-cyan-500 rounded-full" />
                            <h3 className="text-[10px] font-black uppercase tracking-widest">Review Changes</h3>
                        </div>
                        <button onClick={() => setDiffModal(false)} className="text-muted-foreground hover:text-foreground transition-colors">
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" /></svg>
                        </button>
                    </div>

                    <div className="flex-1 overflow-y-auto custom-scrollbar space-y-2">
                        {diffSummary.length === 0 || diffSummary.every(s => s.changeCount === 0) ? (
                            <div className="py-12 text-center">
                                <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">No changes detected</p>
                                <p className="text-[9px] text-muted-foreground mt-1">Save file matches the original</p>
                            </div>
                        ) : (
                            diffSummary.filter(s => s.changeCount > 0).map(slot => (
                                <div key={slot.slotIndex} className="border border-border rounded-lg overflow-hidden">
                                    <button
                                        onClick={() => loadSlotDiff(slot.slotIndex)}
                                        className="w-full px-4 py-3 flex items-center justify-between hover:bg-muted/20 transition-all"
                                    >
                                        <div className="flex items-center space-x-3">
                                            <div className={`transition-transform duration-300 ${diffExpanded[slot.slotIndex] ? 'rotate-90 text-cyan-500' : 'text-muted-foreground'}`}>
                                                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                                            </div>
                                            <span className="text-[10px] font-black uppercase tracking-widest">{slot.charName || `Slot ${slot.slotIndex + 1}`}</span>
                                        </div>
                                        <span className="text-[9px] font-black text-cyan-500 bg-cyan-500/10 border border-cyan-500/30 px-2 py-0.5 rounded">
                                            {slot.changeCount} {slot.changeCount === 1 ? 'change' : 'changes'}
                                        </span>
                                    </button>

                                    {diffExpanded[slot.slotIndex] && diffDetails[slot.slotIndex] && (
                                        <div className="border-t border-border px-4 py-3 space-y-1 bg-muted/5">
                                            {(['stat', 'item', 'storage', 'grace'] as const).map(cat => {
                                                const entries = diffDetails[slot.slotIndex].filter(d => d.category === cat);
                                                if (entries.length === 0) return null;
                                                const catLabel = {stat: 'Stats', item: 'Inventory', storage: 'Storage', grace: 'Graces'}[cat];
                                                return (
                                                    <div key={cat} className="mb-2">
                                                        <p className="text-[8px] font-black text-muted-foreground uppercase tracking-[0.3em] mb-1">{catLabel}</p>
                                                        {entries.map((d, i) => (
                                                            <div key={i} className="flex items-center justify-between py-1 px-2 rounded hover:bg-muted/20">
                                                                <div className="flex items-center space-x-2">
                                                                    <span className={`w-1.5 h-1.5 rounded-full ${d.action === 'added' ? 'bg-green-500' : d.action === 'removed' ? 'bg-red-500' : 'bg-yellow-500'}`} />
                                                                    <span className="text-[10px] font-semibold">{d.field}</span>
                                                                </div>
                                                                <div className="text-[9px] font-mono">
                                                                    {d.action === 'changed' && (
                                                                        <span><span className="text-red-400 line-through">{d.oldValue}</span> <span className="text-muted-foreground mx-1">&rarr;</span> <span className="text-green-400">{d.newValue}</span></span>
                                                                    )}
                                                                    {d.action === 'added' && <span className="text-green-400">+ {d.newValue}</span>}
                                                                    {d.action === 'removed' && <span className="text-red-400">- {d.oldValue}</span>}
                                                                </div>
                                                            </div>
                                                        ))}
                                                    </div>
                                                );
                                            })}
                                        </div>
                                    )}
                                </div>
                            ))
                        )}
                    </div>

                    <div className="mt-4 pt-3 border-t border-border">
                        <button
                            onClick={() => setDiffModal(false)}
                            className="w-full py-2 text-[9px] font-black uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors"
                        >
                            Close
                        </button>
                    </div>
                </div>
            </div>
        )}

        {cloneModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setCloneModal(null)}>
                <div className="bg-background border border-border rounded-xl p-6 w-72 space-y-4 shadow-2xl" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center space-x-2">
                        <div className="w-1 h-4 bg-primary rounded-full" />
                        <h3 className="text-[10px] font-black uppercase tracking-widest">Clone to Slot</h3>
                    </div>
                    <p className="text-[9px] text-muted-foreground uppercase tracking-wide">
                        Cloning: <span className="text-foreground font-bold">{charNames[cloneModal.srcIdx]}</span>
                    </p>
                    <div className="space-y-1">
                        {charNames.map((_, idx) => {
                            if (activeSlots[idx]) return null;
                            return (
                                <button
                                    key={idx}
                                    onClick={() => handleClone(cloneModal.srcIdx, idx)}
                                    className="w-full text-left px-3 py-2.5 rounded-lg border border-border/50 hover:border-primary/40 hover:bg-muted/20 transition-all text-[10px] font-bold uppercase tracking-wider"
                                >
                                    Slot {idx + 1} — Empty
                                </button>
                            );
                        })}
                        {charNames.every((_, idx) => activeSlots[idx]) && (
                            <p className="text-[10px] text-muted-foreground text-center py-4">No empty slots available</p>
                        )}
                    </div>
                    <button
                        onClick={() => setCloneModal(null)}
                        className="w-full py-2 text-[9px] font-black uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors"
                    >
                        Cancel
                    </button>
                </div>
            </div>
        )}
        <ToastBar sidebarWidth={256} />
        </div>
        </div>
    );
}

export default App;
