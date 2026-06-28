import {useState, useEffect, useCallback} from 'react';
import {EventsOn} from '../wailsjs/runtime/runtime';
import toast from './lib/toast';
import {SelectAndOpenSave, GetSlotStates, CleanResidualSlot, SetSlotActivity, WriteSave, CloneSlot, DeleteSlot, GetCharacter, RevertSlot, GetUndoDepth, GetSlotDiff, GetSaveDiffSummary, GetInfuseTypes, GetSlotCapacity, AuditLoadedSaveIssues, GetSaveInventoryIntegrityReport, RepairDuplicateInventoryIndices, CloseSave} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {CharacterTab} from './components/CharacterTab';
import {InventoryTab} from './components/InventoryTab';
import {WorldTab} from './components/WorldTab';
import {ToolsTab} from './components/ToolsTab';
import {SettingsTab} from './components/SettingsTab';
import {DatabaseTab} from './components/DatabaseTab';
import {AppearanceTab} from './components/AppearanceTab';
import {PvPTab} from './components/PvPTab';
import {SortOrderTab} from './components/SortOrderTab';

import {ToastBar} from './components/ToastBar';
import {SafetyModeBanner} from './components/SafetyModeBanner';
import {InventoryIntegrityModal} from './components/integrity/InventoryIntegrityModal';
import {TemplatesShellModal} from './components/templates/TemplatesShellModal';
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

export interface PvPOptions {
    matchmakingRegions: boolean;
    colosseums: boolean;
    revealMap: boolean;
    summoningPools: boolean;
    sitesOfGrace: boolean;
}

const DEFAULT_PVP_OPTIONS: PvPOptions = {
    matchmakingRegions: false,
    colosseums: false,
    revealMap: false,
    summoningPools: false,
    sitesOfGrace: false,
};

function App() {
    const [platform, setPlatform] = useState<string | null>(null);
    const [activeSlots, setActiveSlots] = useState<boolean[]>([]);
    const [charNames, setCharacterNames] = useState<string[]>([]);
    const [slotStates, setSlotStates] = useState<main.SlotState[]>([]);
    const [selectedChar, setSelectedChar] = useState<number>(0);
    const [activeTab, setActiveTab] = useState('character');
    const [inventoryVersion, setInventoryVersion] = useState(0);
    const [saveLoadKey, setSaveLoadKey] = useState(0);
    const [theme, setTheme] = useState<Theme>(() => {
        return (localStorage.getItem('setting:theme') as Theme) || 'dark';
    });
    const [cloneModal, setCloneModal] = useState<{srcIdx: number} | null>(null);
    const [deleteModal, setDeleteModal] = useState<{idx: number} | null>(null);
    const [cleaningSlot, setCleaningSlot] = useState<number | null>(null);
    const [charAddSettings, setCharAddSettings] = useState<Record<number, AddSettings>>(() => {
        try {
            const saved = localStorage.getItem('setting:charAddSettings');
            return saved ? JSON.parse(saved) : {};
        } catch { return {}; }
    });
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
    const [showEmptySlots, setShowEmptySlots] = useState(false);
    const [showOnlyFavorites, setShowOnlyFavorites] = useState(false);
    const [infuseTypes, setInfuseTypes] = useState<db.InfuseType[]>([]);
    const [invView, setInvView] = useState<'inventory' | 'database' | 'sort_order'>('inventory');
    const [detailItem, setDetailItem] = useState<db.ItemEntry | null>(null);
    const [saving, setSaving] = useState(false);
    const [capacity, setCapacity] = useState<main.SlotCapacity | null>(null);
    const [saveIssuesModal, setSaveIssuesModal] = useState<main.SaveIssue[] | null>(null);
    const [saveDataRevision, setSaveDataRevision] = useState(0);
    const [pvpOpts, setPvpOpts] = useState<PvPOptions>(DEFAULT_PVP_OPTIONS);
    // Load-time inventory integrity gate. When the loaded save contains
    // duplicate acquisition indices, integrityReport is set to the dirty
    // report and the InventoryIntegrityModal blocks every editing path
    // (tabs + Save button) until the user runs Repair (and re-scan returns
    // Clean=true) or chooses Close save. integrityBusy / integrityError
    // back the modal's repair/close UX.
    const [integrityReport, setIntegrityReport] = useState<main.SaveInventoryIntegrityReport | null>(null);
    const [integrityBusy, setIntegrityBusy] = useState(false);
    const [integrityError, setIntegrityError] = useState<string | null>(null);
    // pendingPlatform captures the platform string returned by the load
    // endpoint while the integrity modal is gating the user. On successful
    // repair we promote it to the live `platform` state so the editor opens
    // with the same platform the user originally loaded.
    const [pendingPlatform, setPendingPlatform] = useState<string | null>(null);
    const integrityBlocking = integrityReport !== null && !integrityReport.clean;
    const [templatesShellOpen, setTemplatesShellOpen] = useState(false);

    const refreshUndoDepth = useCallback(() => {
        if (!platform) { setUndoDepth(0); return; }
        GetUndoDepth(selectedChar).then(setUndoDepth).catch(() => setUndoDepth(0));
    }, [platform, selectedChar]);

    const handlePvPMutate = useCallback(() => {
        refreshUndoDepth();
        setSaveDataRevision(v => v + 1);
    }, [refreshUndoDepth]);

    useEffect(() => { refreshUndoDepth(); }, [refreshUndoDepth, inventoryVersion]);

    const tabs = platform
        ? ['character', 'inventory', 'world', 'advanced', 'tools', 'settings']
        : ['character', 'inventory', 'settings'];

    useEffect(() => { localStorage.setItem('setting:theme', theme); }, [theme]);
    useEffect(() => { GetInfuseTypes().then(res => setInfuseTypes(res || [])); }, []);
    useEffect(() => { localStorage.setItem('setting:columnVisibility', JSON.stringify(columnVisibility)); }, [columnVisibility]);
    useEffect(() => { localStorage.setItem('setting:showFlaggedItems', String(showFlaggedItems)); }, [showFlaggedItems]);
    useEffect(() => { localStorage.setItem('setting:debugMode', String(debugMode)); }, [debugMode]);
    useEffect(() => { localStorage.setItem('selectedDeployTarget', selectedDeployTarget); }, [selectedDeployTarget]);
    useEffect(() => { localStorage.setItem('setting:charAddSettings', JSON.stringify(charAddSettings)); }, [charAddSettings]);

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

    // Shared post-load gate: runs the inventory integrity scan on every freshly
    // loaded main save (local file dialog or remote download). On a clean save
    // the normal editor state is materialised; on a dirty save the platform
    // stays null and the blocking modal opens until Repair clears the issue or
    // Close save drops the file.
    const finalizeLoadedSaveWithIntegrityCheck = useCallback(async (plat: string) => {
        setIntegrityError(null);
        let report: main.SaveInventoryIntegrityReport;
        try {
            report = await GetSaveInventoryIntegrityReport();
        } catch (err) {
            // Surface as a blocking error: we cannot certify the save is safe
            // to edit, so the user must close it (or reload) explicitly.
            toast.error('Inventory integrity verification failed: ' + String(err));
            // Attempt to drop the loaded save from backend memory so a stale
            // (potentially malformed) handle does not linger after a failed
            // verification. UI stays in the no-save state in either branch.
            try {
                await CloseSave();
            } catch (closeErr) {
                toast.error('Failed to close the loaded save after inventory integrity verification failed: ' + String(closeErr));
            }
            setPlatform(null);
            setIntegrityReport(null);
            return;
        }
        if (report.clean) {
            setIntegrityReport(null);
            setPendingPlatform(null);
            setPlatform(plat);
            setSaveLoadKey(k => k + 1);
            refreshSlots();
            return;
        }
        // Dirty: keep platform null so tabs/Save remain hidden; let the modal drive next step.
        setIntegrityReport(report);
        setPendingPlatform(plat);
        setPlatform(null);
    }, []);

    const handleOpenSave = async () => {
        try {
            const plat = await SelectAndOpenSave();
            await finalizeLoadedSaveWithIntegrityCheck(plat);
        } catch (err) {
            toast.error(String(err));
        }
    };

    const handleRepairIntegrity = async () => {
        if (!integrityReport || integrityBusy) return;
        setIntegrityBusy(true);
        setIntegrityError(null);
        try {
            for (const slot of integrityReport.slots) {
                await RepairDuplicateInventoryIndices(slot.slotIndex);
            }
            const rescan = await GetSaveInventoryIntegrityReport();
            if (rescan.clean) {
                setIntegrityReport(null);
                if (pendingPlatform) {
                    setPlatform(pendingPlatform);
                }
                setPendingPlatform(null);
                setSaveLoadKey(k => k + 1);
                refreshSlots();
                toast.success('Inventory acquisition indices repaired successfully. Save the file to write repaired changes.');
            } else {
                setIntegrityReport(rescan);
                setIntegrityError('Repair did not resolve all duplicate inventory acquisition indices. Saving remains blocked.');
            }
        } catch (err) {
            // A repair call rejected (possibly after one or more earlier
            // slots already succeeded). Refresh the report so the modal
            // reflects the actual remaining issues; never promote platform
            // here, even if the refreshed scan happens to come back clean.
            // The user must explicitly choose Repair again or Close save.
            let repairMsg = 'Repair failed: ' + String(err);
            try {
                const rescan = await GetSaveInventoryIntegrityReport();
                setIntegrityReport(rescan);
            } catch (rescanErr) {
                repairMsg += ' (and integrity re-scan also failed: ' + String(rescanErr) + ')';
            }
            setIntegrityError(repairMsg);
        } finally {
            setIntegrityBusy(false);
        }
    };

    const handleCloseSaveFromIntegrity = async () => {
        if (integrityBusy) return;
        setIntegrityBusy(true);
        setIntegrityError(null);
        try {
            await CloseSave();
            setIntegrityReport(null);
            setPendingPlatform(null);
            setPlatform(null);
            setActiveSlots([]);
            setCharacterNames([]);
            setSelectedChar(0);
            setSaveLoadKey(k => k + 1);
        } catch (err) {
            setIntegrityError('Close save failed: ' + String(err));
        } finally {
            setIntegrityBusy(false);
        }
    };

    // Performs the actual file write. Separated so it can run either directly
    // (no issues) or after the user acknowledges the pre-save issues modal.
    const doWriteSave = async () => {
        if (!platform) return;
        setSaving(true);
        try { await WriteSave(platform); toast.success('File saved'); }
        catch (err) { toast.error(String(err)); }
        finally { setSaving(false); }
    };

    const handleSaveAs = async () => {
        if (!platform) return;
        // Pre-save audit: surface blocking data issues (e.g. out-of-range weapon
        // upgrades) BEFORE writing, so nothing is saved silently. Saving is still
        // allowed — the modal just informs and points to where to repair.
        try {
            const issues = await AuditLoadedSaveIssues();
            if (issues && issues.length > 0) {
                setSaveIssuesModal(issues);
                return;
            }
        } catch { /* audit is advisory; never block saving on its failure */ }
        await doWriteSave();
    };

    const refreshSlots = async () => {
        const states = await GetSlotStates();
        setSlotStates(states || []);
        setActiveSlots((states || []).map(s => s.active));
        setCharacterNames((states || []).map(s => s.name || 'Empty Slot'));
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

    const handleCleanResidualSlot = async (idx: number) => {
        if (cleaningSlot === idx) return;
        setCleaningSlot(idx);
        try {
            await CleanResidualSlot(idx);
            await refreshSlots();
            toast.success(`Slot ${idx + 1} cleaned`);
        } catch (err) {
            toast.error(String(err));
        } finally {
            setCleaningSlot(null);
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

    // Native window.confirm() is a no-op in the Wails WKWebView (returns false),
    // so confirmation is handled by the deleteModal instead.
    const handleDelete = async (idx: number) => {
        setDeleteModal(null);
        try {
            await DeleteSlot(idx);
            await refreshSlots();
            // Clear-in-place: slot indices are stable (no shift). Only reselect if
            // the deleted slot was the selected one — pick the first active slot.
            if (selectedChar === idx) {
                const slots = await GetSlotStates();
                const next = slots.findIndex(s => s.active);
                setSelectedChar(next >= 0 ? next : 0);
            }
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
                                <p className="text-[7px] text-primary tracking-wider mt-0.5">by OiSiS</p>
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
                                                    <button onClick={(e) => { e.stopPropagation(); setDeleteModal({idx}); }} title="Delete character"
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
                    {slotStates.filter(s => !s.active).length > 0 && (
                                <>
                                    <button onClick={() => setShowEmptySlots(v => !v)}
                                        className="flex items-center gap-1.5 px-1 py-1 text-[9px] font-bold text-muted-foreground hover:text-foreground transition-colors w-full">
                                        <svg className={`w-2.5 h-2.5 transition-transform duration-200 ${showEmptySlots ? 'rotate-90' : ''}`}
                                            fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" /></svg>
                            <span className="uppercase tracking-widest">Inactive Slots</span>
                            <span className="text-[8px] bg-muted/30 px-1.5 py-0.5 rounded-full">{slotStates.filter(s => !s.active).length}</span>
                                    </button>
                                    {showEmptySlots && (
                                        <div className="space-y-1 animate-in fade-in slide-in-from-top-1 duration-200">
                                {charNames.map((name, idx) => {
                                    if (activeSlots[idx]) return null;
                                    const state = slotStates[idx];
                                    const residual = !!state?.residual;
                                    return (
                                        <div key={idx}
                                            className={`group relative p-2.5 rounded-lg border transition-all ${residual ? 'border-amber-500/30 bg-amber-500/5' : 'bg-transparent border-border/30'}`}>
                                            <div className="flex items-center justify-between relative z-10">
                                                <div className="flex items-center space-x-2.5 min-w-0">
                                                    <div className={`w-1.5 h-1.5 flex-shrink-0 rounded-full ${residual ? 'bg-amber-500 shadow-[0_0_6px_rgba(245,158,11,0.35)]' : 'bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.3)]'}`} />
                                                    <span className="text-[9px] text-muted-foreground/50 mr-1">{idx + 1}</span>
                                                    <span className={`text-[10px] font-bold uppercase tracking-tight truncate ${residual ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground/60 italic'}`}>{name}</span>
                                                </div>
                                                {residual ? (
                                                    <button onClick={(e) => { e.stopPropagation(); handleCleanResidualSlot(idx); }}
                                                        disabled={cleaningSlot === idx}
                                                        className="px-2 py-1 rounded-md border border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400 hover:bg-amber-500/20 transition-all text-[8px] font-black uppercase tracking-widest disabled:opacity-50 disabled:cursor-not-allowed">
                                                        {cleaningSlot === idx ? 'Cleaning' : 'Clean'}
                                                    </button>
                                                ) : (
                                                    <span className="text-[8px] font-black uppercase tracking-widest text-muted-foreground/50">Empty</span>
                                                )}
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
                    <button
                        type="button"
                        data-testid="open-templates-shell"
                        onClick={() => setTemplatesShellOpen(true)}
                        disabled={saving || integrityBlocking}
                        className="w-full border border-blue-500/40 bg-blue-500/10 text-blue-600 hover:bg-blue-500/20 font-black py-2 rounded-lg text-[8px] uppercase tracking-[0.15em] transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        Templates
                    </button>
                    {platform && (
                        <button onClick={handleSaveAs} disabled={saving}
                            className="w-full bg-primary text-primary-foreground font-black py-2 rounded-lg text-[8px] uppercase tracking-[0.15em] shadow-lg shadow-primary/20 hover:brightness-110 active:scale-95 transition-all disabled:opacity-50 flex items-center justify-center space-x-1.5">
                            {saving ? <div className="w-3 h-3 border-2 border-primary-foreground/20 border-t-primary-foreground rounded-full animate-spin" /> :
                            <><svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4"></path></svg><span>Save As ({platform})</span></>}
                        </button>
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
                    <nav className="flex gap-1.5 p-1 bg-muted/30 rounded-lg border border-border/50">
                        {tabs.map(tab => (
                            <button
                                key={tab}
                                onClick={() => {
                                    if (tab === 'inventory') setInventoryVersion(v => v + 1);
                                    setActiveTab(tab);
                                }}
                                className={`px-4 py-1.5 rounded-md text-[10px] font-black uppercase tracking-wider transition-all ${activeTab === tab ? 'bg-green-700/80 shadow-sm text-white' : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'}`}
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
                    <div className="w-full h-full px-6 pb-6 pt-3 flex flex-col min-h-0">
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
                                    selectedDeployTarget={selectedDeployTarget}
                                    setSelectedDeployTarget={setSelectedDeployTarget}
                                    onAfterLoad={finalizeLoadedSaveWithIntegrityCheck}
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
                                <div className={['inventory', 'advanced'].includes(activeTab) ? 'flex-1 flex flex-col min-h-0 overflow-hidden' : 'flex-1 overflow-y-auto custom-scrollbar'}>
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
                                <div className={['inventory', 'advanced'].includes(activeTab) ? 'flex-1 flex flex-col min-h-0 overflow-hidden' : 'flex-1 overflow-y-auto custom-scrollbar'}>
                                <div className={activeTab !== 'character' ? 'hidden' : undefined}>
                                    <CharacterTab charIndex={selectedChar} onNameChange={refreshSlots} onMutate={refreshUndoDepth} refreshKey={inventoryVersion} addSettings={charAddSettings[selectedChar] ?? DEFAULT_ADD_SETTINGS} onAddSettingsChange={(s) => setCharAddSettings(prev => ({...prev, [selectedChar]: s}))} infuseTypes={infuseTypes} />
                                </div>
                                {activeTab === 'inventory' && (
                                    <div className="flex-1 flex flex-col min-h-0">
                                        {/* Header consolidation (spec/36): toggle pills + capacity bar (Inventory) OR
                                            Add Settings accordion (Database) on a single row. */}
                                        {/* Mode bar: left = tabs, right = capacity stats */}
                                        <div className="flex items-center justify-between mb-3 shrink-0 gap-4">
                                            <div className="flex gap-1.5 p-1 bg-muted/30 rounded-lg border border-border/50 shrink-0">
                                                {([
                                                    { id: 'database', label: 'Item Database' },
                                                    { id: 'inventory', label: 'Equipment' },
                                                    { id: 'sort_order', label: 'Weapons & Sort Order' },
                                                ] as { id: 'database' | 'inventory' | 'sort_order'; label: string }[]).map(({ id, label }) => (
                                                    <button
                                                        key={id}
                                                        onClick={() => { setInvView(id); if (id !== 'database') setDetailItem(null); }}
                                                        className={`px-4 py-1.5 rounded-md text-[10px] font-black uppercase tracking-wider transition-all ${
                                                            invView === id
                                                                ? 'bg-green-700/80 shadow-sm text-white'
                                                                : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                                                        }`}
                                                    >
                                                        {label}
                                                    </button>
                                                ))}
                                            </div>

                                            {invView !== 'sort_order' && capacity && (
                                                <div className="flex items-center gap-4">
                                                    {[
                                                        { label: 'All Items', used: capacity.gaItemsUsed, max: capacity.gaItemsMax },
                                                        { label: 'Inventory', used: capacity.inventoryUsed, max: capacity.inventoryMax },
                                                        { label: 'Storage', used: capacity.storageUsed, max: capacity.storageMax },
                                                    ].map(({ label, used, max }) => {
                                                        const pct = max > 0 ? (used / max) * 100 : 0;
                                                        const color = pct >= 95 ? 'bg-red-500' : pct >= 80 ? 'bg-amber-500' : 'bg-primary';
                                                        return (
                                                            <div key={label} className="flex items-center gap-2">
                                                                <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground whitespace-nowrap">{label}</span>
                                                                <div className="w-16 h-1.5 bg-muted/30 rounded-full overflow-hidden">
                                                                    <div className={`h-full ${color} rounded-full transition-all duration-500`} style={{ width: `${Math.min(pct, 100)}%` }} />
                                                                </div>
                                                                <span className={`text-[9px] font-bold tabular-nums ${pct >= 95 ? 'text-red-400' : 'text-muted-foreground'}`}>{used}/{max}</span>
                                                            </div>
                                                        );
                                                    })}
                                                </div>
                                            )}
                                        </div>

                                        {invView === 'inventory' ? (
                                            <InventoryTab charIndex={selectedChar} inventoryVersion={inventoryVersion} columnVisibility={columnVisibility} showFlaggedItems={showFlaggedItems} category={category} setCategory={setCategory} onMutate={refreshUndoDepth} showOnlyFavorites={showOnlyFavorites} onToggleFavorites={() => setShowOnlyFavorites(v => !v)} />
                                        ) : invView === 'database' ? (
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
                                                onToggleFavorites={() => setShowOnlyFavorites(v => !v)}
                                            />
                                        ) : (
                                            <SortOrderTab
                                                charIndex={selectedChar}
                                                inventoryVersion={inventoryVersion}
                                                onMutate={refreshUndoDepth}
                                            />
                                        )}
                                    </div>
                                )}
                                {activeTab === 'world' && <WorldTab charIdx={selectedChar} platform={platform} showFlaggedItems={showFlaggedItems} saveLoadKey={saveLoadKey} saveDataRevision={saveDataRevision} onMutate={() => { refreshUndoDepth(); setInventoryVersion(v => v + 1); }} addSettings={charAddSettings[selectedChar] ?? DEFAULT_ADD_SETTINGS} />}
                                {activeTab === 'advanced' && <PvPTab charIdx={selectedChar} platform={platform} pvpOpts={pvpOpts} onPvpOptsChange={setPvpOpts} onMutate={handlePvPMutate} />}
                                {activeTab === 'tools' && <ToolsTab charIndex={selectedChar} platform={platform ?? ''} onComplete={refreshSlots} onMutate={() => { setInventoryVersion(v => v + 1); setSaveLoadKey(k => k + 1); refreshSlots(); refreshUndoDepth(); }} />}
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
                        {slotStates.map((slot, idx) => {
                            if (slot.active) return null;
                            if (slot.residual) {
                                return (
                                    <div key={idx} className="flex items-center justify-between gap-2 px-3 py-2.5 rounded-lg border border-amber-500/30 bg-amber-500/5">
                                        <div className="min-w-0">
                                            <p className="text-[10px] font-bold uppercase tracking-wider text-amber-600 dark:text-amber-400 truncate">Slot {idx + 1} — {slot.name}</p>
                                            <p className="text-[8px] font-bold uppercase tracking-widest text-muted-foreground">Residual data</p>
                                        </div>
                                        <button
                                            onClick={() => handleCleanResidualSlot(idx)}
                                            disabled={cleaningSlot === idx}
                                            className="px-2 py-1 rounded-md border border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400 hover:bg-amber-500/20 transition-all text-[8px] font-black uppercase tracking-widest disabled:opacity-50 disabled:cursor-not-allowed"
                                        >
                                            {cleaningSlot === idx ? 'Cleaning' : 'Clean'}
                                        </button>
                                    </div>
                                );
                            }
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
                        {slotStates.every(slot => slot.active) && (
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
        {saveIssuesModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setSaveIssuesModal(null)}>
                <div className="bg-background border border-border rounded-xl p-6 w-[28rem] max-w-[90vw] space-y-4 shadow-2xl" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center space-x-2">
                        <div className="w-1 h-4 bg-amber-500 rounded-full" />
                        <h3 className="text-[10px] font-black uppercase tracking-widest">Save — {saveIssuesModal.length} issue(s) found</h3>
                    </div>
                    <p className="text-[9px] text-muted-foreground uppercase tracking-wide">
                        The save has invalid data. You can save anyway, or cancel and repair it first.
                    </p>
                    <div className="max-h-56 overflow-y-auto space-y-2">
                        {Object.values(
                            saveIssuesModal.reduce((acc, it) => {
                                const key = `${it.slot}|${it.message}|${it.fixTab}`;
                                if (!acc[key]) acc[key] = {issue: it, count: 0};
                                acc[key].count++;
                                return acc;
                            }, {} as Record<string, {issue: main.SaveIssue; count: number}>),
                        ).map(({issue, count}, i) => (
                            <div key={i} className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-2 space-y-1">
                                <p className="text-[10px] text-foreground">
                                    <span className="font-black">Slot {issue.slot}</span>
                                    {count > 1 ? ` · ${count}×` : ''} — {issue.message}
                                </p>
                                <p className="text-[9px] text-muted-foreground">Fix in: <span className="text-primary">{issue.fixTab}</span></p>
                            </div>
                        ))}
                    </div>
                    <div className="flex gap-2">
                        <button
                            onClick={async () => { setSaveIssuesModal(null); await doWriteSave(); }}
                            className="flex-1 py-2 rounded-lg bg-amber-500/10 border border-amber-500/40 text-amber-500 hover:bg-amber-500/20 transition-all text-[9px] font-black uppercase tracking-widest"
                        >
                            Save Anyway
                        </button>
                        <button
                            onClick={() => setSaveIssuesModal(null)}
                            className="flex-1 py-2 rounded-lg border border-border/50 text-muted-foreground hover:text-foreground hover:bg-muted/20 transition-all text-[9px] font-black uppercase tracking-widest"
                        >
                            Cancel &amp; Fix
                        </button>
                    </div>
                </div>
            </div>
        )}
        {deleteModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setDeleteModal(null)}>
                <div className="bg-background border border-border rounded-xl p-6 w-72 space-y-4 shadow-2xl" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center space-x-2">
                        <div className="w-1 h-4 bg-red-500 rounded-full" />
                        <h3 className="text-[10px] font-black uppercase tracking-widest">Delete Character</h3>
                    </div>
                    <p className="text-[9px] text-muted-foreground uppercase tracking-wide">
                        Delete <span className="text-foreground font-bold">{charNames[deleteModal.idx]}</span>? This cannot be undone.
                    </p>
                    <div className="flex gap-2">
                        <button
                            onClick={() => handleDelete(deleteModal.idx)}
                            className="flex-1 py-2 rounded-lg bg-red-500/10 border border-red-500/40 text-red-500 hover:bg-red-500/20 transition-all text-[9px] font-black uppercase tracking-widest"
                        >
                            Delete
                        </button>
                        <button
                            onClick={() => setDeleteModal(null)}
                            className="flex-1 py-2 rounded-lg border border-border/50 text-muted-foreground hover:text-foreground hover:bg-muted/20 transition-all text-[9px] font-black uppercase tracking-widest"
                        >
                            Cancel
                        </button>
                    </div>
                </div>
            </div>
        )}
        {integrityBlocking && integrityReport && (
            <InventoryIntegrityModal
                report={integrityReport}
                busy={integrityBusy}
                errorMessage={integrityError ?? undefined}
                onRepair={handleRepairIntegrity}
                onCloseSave={handleCloseSaveFromIntegrity}
            />
        )}
        {templatesShellOpen && (
            <TemplatesShellModal
                onClose={() => setTemplatesShellOpen(false)}
                charIndex={selectedChar}
                saveLoaded={!!platform}
                onCharacterTemplateApplied={() => {
                    setInventoryVersion(v => v + 1);
                    setSaveLoadKey(k => k + 1);
                    refreshSlots();
                    refreshUndoDepth();
                }}
            />
        )}
        <ToastBar sidebarWidth={256} />
        </div>
        </div>
    );
}

export default App;
