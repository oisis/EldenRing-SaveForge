import {useState, useEffect, useCallback} from 'react';
import toast from '../lib/toast';
import {
    GetSteamIDString, SetSteamIDFromString,
    GetDeployTargets, SaveDeployTarget, DeleteDeployTarget,
    TestSSHConnection, DeploySave, DownloadRemoteSave,
    LaunchRemoteGame, CloseRemoteGame, DeployAndLaunch, CloseAndDownload,
    PrepareConversion, ExecuteConversion,
    BackupCurrentSave,
} from '../../wailsjs/go/main/App';
import {deploy} from '../../wailsjs/go/models';
import {saveSafetyProfile, type SafetyProfile} from '../state/safetyProfile';
import {FavoritesManager} from './FavoritesManager';
import {useFavorites} from '../state/favorites';
import {InventoryIssuesModal} from './InventoryIssuesModal';
import {SaveManagerModal} from './SaveManagerModal';
import {ChaosWarningModal} from './ChaosWarningModal';
import {scanRepairIssuesLoaded, type RepairIssueReport} from '../lib/repairIssues';

interface SettingsTabProps {
    theme: 'light' | 'dark' | 'golden';
    setTheme: (theme: 'light' | 'dark' | 'golden') => void;
    columnVisibility: { id: boolean; category: boolean };
    setColumnVisibility: (visibility: { id: boolean; category: boolean }) => void;
    safetyProfile: SafetyProfile;
    debugMode: boolean;
    setDebugMode: (value: boolean) => void;
    platform: string | null;
    selectedDeployTarget: string;
    setSelectedDeployTarget: (v: string) => void;
    onAfterLoad: (platform: string) => Promise<void> | void;
    charIndex: number;
    onComplete: () => void;
    onMutate?: () => void;
}

const EMPTY_SSH_TARGET: deploy.Target = new deploy.Target({
    type: 'ssh', name: '', host: '', port: 22, user: 'deck', keyPath: '~/.ssh/id_rsa',
    savePath: '/home/deck/.local/share/Steam/steamapps/compatdata/1245620/pfx/drive_c/users/steamuser/AppData/Roaming/EldenRing/{STEAM_ID}/ER0000.sl2',
    gameStartCmd: 'steam steam://rungameid/1245620',
    gameStopCmd: 'pkill -TERM -f eldenring.exe',
});

const EMPTY_LOCAL_TARGET: deploy.Target = new deploy.Target({
    type: 'local', name: '', host: '', port: 22, user: '', keyPath: '',
    savePath: '',
    gameStartCmd: '',
    gameStopCmd: '',
});

export function SettingsTab({
    theme, setTheme, columnVisibility, setColumnVisibility,
    safetyProfile, debugMode, setDebugMode,
    platform, charIndex,
    selectedDeployTarget: selectedTarget, setSelectedDeployTarget: setSelectedTarget,
    onAfterLoad,
    onComplete, onMutate,
}: SettingsTabProps) {
    const {count: favCount} = useFavorites();
    const [view, setView] = useState<'overview' | 'favorites'>('overview');

    const [steamIdInput, setSteamIdInput] = useState('');
    const [steamIdSaved, setSteamIdSaved] = useState('');
    const [steamIdError, setSteamIdError] = useState('');
    const [steamIdApplying, setSteamIdApplying] = useState(false);

    const [chaosModalOpen, setChaosModalOpen] = useState(false);
    // Switching into chaos is gated by the warning/backup modal; safe and
    // expanded_limits apply immediately. Cancelling the modal keeps the previous
    // profile (App owns safetyProfile and only updates on saveSafetyProfile).
    const selectProfile = (profile: SafetyProfile) => {
        if (profile === 'chaos') { setChaosModalOpen(true); return; }
        saveSafetyProfile(profile);
    };
    const confirmChaos = async (autoBackup: boolean) => {
        if (autoBackup) {
            try {
                await BackupCurrentSave();
                toast.success('Backup created');
            } catch (err) {
                toast.error(`Backup failed: ${err}`);
                return; // do not enable Chaos Mode without the requested backup
            }
        }
        setChaosModalOpen(false);
        saveSafetyProfile('chaos');
    };

    const [scanning, setScanning] = useState(false);
    const [inventoryIssuesModal, setInventoryIssuesModal] = useState<{ reports: RepairIssueReport[] } | null>(null);

    // Diagnostics now scans only the loaded save directly — no choice modal.
    // A manual scan always opens the issues modal, even with no issues, so the
    // user sees the ValidationCoverage proving the scan ran.
    const handleDiagnostics = async () => {
        if (scanning) return; // prevent duplicate requests
        setScanning(true);
        try {
            const report = await scanRepairIssuesLoaded(charIndex);
            setInventoryIssuesModal({ reports: [report] });
        } catch (e) {
            toast.error('Diagnostics failed: ' + e);
        } finally {
            setScanning(false);
        }
    };

    // Conversion flow
    const [convStep, setConvStep] = useState<'idle' | 'selecting' | 'steamid' | 'converting'>('idle');
    const [convSourcePath, setConvSourcePath] = useState('');
    const [steamIDInput, setSteamIDInput] = useState('');
    const [steamIDError, setSteamIDError] = useState('');

    const handleConvertClick = async () => {
        setConvStep('selecting');
        try {
            const info = await PrepareConversion();
            const target = info.platform === 'PC' ? 'PS4' : 'PC';
            setConvSourcePath(info.path);
            if (target === 'PC') {
                setConvStep('steamid');
            } else {
                await runConversion(info.path, 'PS4', '');
            }
        } catch (e) {
            const msg = String(e);
            if (!msg.includes('no file selected')) toast.error('Conversion failed: ' + e);
            setConvStep('idle');
        }
    };

    const runConversion = async (sourcePath: string, targetPlatform: string, steamID: string) => {
        setConvStep('converting');
        try {
            const destPath = await ExecuteConversion(sourcePath, targetPlatform, steamID);
            toast.success(`Saved to ${destPath}`);
        } catch (e) {
            const msg = String(e);
            if (!msg.includes('no destination selected')) toast.error('Conversion failed: ' + e);
        } finally {
            setConvStep('idle');
            setSteamIDInput('');
            setSteamIDError('');
        }
    };

    const handleSteamIDSubmit = () => {
        const trimmed = steamIDInput.trim();
        if (!/^\d{17}$/.test(trimmed) || !trimmed.startsWith('7656119')) {
            setSteamIDError('Steam ID must be 17 digits starting with 7656119');
            return;
        }
        setSteamIDError('');
        runConversion(convSourcePath, 'PC', trimmed);
    };

    const cancelConversion = () => {
        setConvStep('idle');
        setSteamIDInput('');
        setSteamIDError('');
    };

    const converting = convStep !== 'idle';

    // Deploy state
    const [targets, setTargets] = useState<deploy.Target[]>([]);
    const [editTarget, setEditTarget] = useState<deploy.Target>(new deploy.Target(EMPTY_SSH_TARGET));
    const [showForm, setShowForm] = useState(false);
    const [deploying, setDeploying] = useState(false);
    const [showSaveManager, setShowSaveManager] = useState(false);

    const loadTargets = useCallback(() => {
        GetDeployTargets().then(t => setTargets(t || [])).catch(() => setTargets([]));
    }, []);

    useEffect(() => { loadTargets(); }, [loadTargets]);

    useEffect(() => {
        if (platform !== 'PC') { setSteamIdInput(''); setSteamIdSaved(''); return; }
        GetSteamIDString().then(id => { setSteamIdInput(id); setSteamIdSaved(id); });
    }, [platform]);

    const validateSteamId = (val: string) => {
        if (!/^\d{17}$/.test(val)) return 'SteamID must be exactly 17 digits.';
        if (!val.startsWith('7656119')) return 'SteamID must start with 7656119.';
        return '';
    };

    const handleApplySteamId = async () => {
        const err = validateSteamId(steamIdInput);
        if (err) { setSteamIdError(err); return; }
        setSteamIdApplying(true); setSteamIdError('');
        try { await SetSteamIDFromString(steamIdInput); setSteamIdSaved(steamIdInput); }
        catch (e) { setSteamIdError(String(e)); }
        finally { setSteamIdApplying(false); }
    };

    // Deploy handlers
    const handleSaveTarget = async () => {
        if (!editTarget.name || (editTarget.type === 'ssh' && !editTarget.host)) { toast.error('Name and host required'); return; }
        if (!editTarget.savePath) { toast.error('Save path required'); return; }
        try { await SaveDeployTarget(editTarget); toast.success(`Target "${editTarget.name}" saved`); loadTargets(); setShowForm(false); setSelectedTarget(editTarget.name); }
        catch (e) { toast.error(String(e)); }
    };
    const handleDeleteTarget = async (name: string) => {
        try { await DeleteDeployTarget(name); toast.success(`Deleted "${name}"`); if (selectedTarget === name) setSelectedTarget(''); loadTargets(); }
        catch (e) { toast.error(String(e)); }
    };
    const handleTestConnection = async () => {
        if (!selectedTarget) return;
        const tid = toast.loading('Testing...');
        try { toast.success(await TestSSHConnection(selectedTarget), { id: tid }); }
        catch (e) { toast.error(String(e), { id: tid }); }
    };
    const handleUpload = async () => {
        if (!selectedTarget) return; setDeploying(true);
        const tid = toast.loading('Uploading save...');
        try { const msg = await DeploySave(selectedTarget); toast.success(msg, { id: tid }); }
        catch (e) { toast.error(String(e), { id: tid }); }
        finally { setDeploying(false); }
    };
    const handleDownload = async () => {
        if (!selectedTarget) return; setDeploying(true);
        const tid = toast.loading('Downloading...');
        try {
            const plat = await DownloadRemoteSave(selectedTarget);
            toast.success('Downloaded & loaded', { id: tid });
            await onAfterLoad(plat);
        }
        catch (e) { toast.error(String(e), { id: tid }); }
        finally { setDeploying(false); }
    };
    const handleLaunch = async () => {
        if (!selectedTarget) return;
        try { const msg = await LaunchRemoteGame(selectedTarget); toast.success(msg || 'Game launch sent'); } catch (e) { toast.error(String(e)); }
    };
    const handleClose = async () => {
        if (!selectedTarget) return;
        try { const msg = await CloseRemoteGame(selectedTarget); toast.success(msg || 'Game close sent'); } catch (e) { toast.error(String(e)); }
    };
    const handleDeployAndLaunch = async () => {
        if (!selectedTarget) return; setDeploying(true);
        const tid = toast.loading('Upload → Launch...');
        try { await DeployAndLaunch(selectedTarget); toast.success('Deploy complete', { id: tid }); }
        catch (e) { toast.error(String(e), { id: tid }); }
        finally { setDeploying(false); }
    };
    const handleCloseAndDownload = async () => {
        if (!selectedTarget) return; setDeploying(true);
        const tid = toast.loading('Close → Download...');
        try {
            const plat = await CloseAndDownload(selectedTarget);
            toast.success('Game closed & save loaded', { id: tid });
            await onAfterLoad(plat);
        }
        catch (e) { toast.error(String(e), { id: tid }); }
        finally { setDeploying(false); }
    };

    const inputCls = "w-full bg-background border border-border/50 rounded px-2.5 py-1.5 text-[11px] font-mono focus:outline-none focus:ring-1 focus:ring-primary/20 focus:border-primary transition-all";
    const labelCls = "text-[8px] font-black uppercase tracking-widest text-muted-foreground";
    const btnSm = "px-2.5 py-1 rounded text-[8px] font-black uppercase tracking-widest transition-all disabled:opacity-50";
    const btnAction = `${btnSm} bg-primary text-primary-foreground shadow-sm hover:brightness-110 active:scale-95`;
    const btnSecondary = `${btnSm} bg-muted/30 text-foreground border border-border hover:bg-muted/50`;
    const sectionHdr = "flex items-center space-x-3 px-1";
    const dot = "w-1 h-5 bg-primary rounded-full shadow-[0_0_6px_rgba(var(--primary),0.3)]";
    const hdrText = "text-[10px] font-black uppercase tracking-[0.25em] text-foreground/80";

    if (view === 'favorites') {
        return (
            <div className="space-y-3 animate-in fade-in duration-300">
                <button onClick={() => setView('overview')}
                    className="flex items-center gap-1.5 text-[9px] font-black uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors">
                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M15 19l-7-7 7-7" />
                    </svg>
                    Back to Tools
                </button>
                <FavoritesManager />
            </div>
        );
    }

    return (
        <>
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
            {/* SteamID modal for PS4 → PC conversion */}
            {convStep === 'steamid' && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
                    <div className="card p-6 w-full max-w-sm space-y-4">
                        <h3 className="text-[11px] font-black uppercase tracking-wider text-foreground">Steam ID Required</h3>
                        <p className="text-[9px] text-muted-foreground">
                            PS4 saves don't contain a Steam ID. Enter yours to embed it in the converted PC save.
                            Find it at <span className="text-violet-400">steamcommunity.com/id/me</span> (17-digit number).
                        </p>
                        <div className="space-y-2">
                            <input
                                type="text"
                                value={steamIDInput}
                                onChange={e => { setSteamIDInput(e.target.value); setSteamIDError(''); }}
                                onKeyDown={e => e.key === 'Enter' && handleSteamIDSubmit()}
                                placeholder="76561198..."
                                maxLength={17}
                                className="w-full bg-background border border-border rounded px-3 py-2 text-[11px] font-mono focus:outline-none focus:border-violet-500"
                                autoFocus
                            />
                            {steamIDError && <p className="text-[9px] text-destructive">{steamIDError}</p>}
                        </div>
                        <div className="flex gap-2 justify-end">
                            <button onClick={cancelConversion}
                                className="px-3 py-1.5 text-[9px] font-black uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors">
                                Cancel
                            </button>
                            <button onClick={handleSteamIDSubmit}
                                className="px-3 py-1.5 text-[9px] font-black uppercase tracking-wider bg-violet-500 text-white rounded hover:bg-violet-600 transition-colors">
                                Convert
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Appearance + SteamID + UI Customization */}
            <section className="space-y-3">
                <div className={sectionHdr}><div className={dot} /><h2 className={hdrText}>Appearance & Steam ID</h2></div>
                <div className="card px-4 py-3 space-y-3">
                    <div className="flex items-center gap-6 flex-wrap">
                        {/* Theme */}
                        <div className="flex items-center gap-3">
                            <p className="text-[10px] font-bold text-foreground">Theme</p>
                            <div className="flex bg-muted/30 p-0.5 rounded border border-border">
                                {(['light', 'dark', 'golden'] as const).map(t => (
                                    <button key={t} onClick={() => setTheme(t)}
                                        className={`px-4 py-1 rounded text-[9px] font-black uppercase tracking-widest transition-all ${theme === t ? 'bg-primary text-primary-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                                    >{t === 'golden' ? 'Elden Ring' : t}</button>
                                ))}
                            </div>
                        </div>
                        {/* SteamID */}
                        <div className="flex items-center gap-2 flex-1 min-w-0">
                            <p className="text-[10px] font-bold text-foreground flex-shrink-0">Steam ID</p>
                            {platform !== 'PC' ? (
                                <p className="text-[10px] text-muted-foreground font-medium">{platform ? 'N/A (PS4)' : 'Load a PC save first'}</p>
                            ) : (<>
                                <input type="text" value={steamIdInput} onChange={e => { setSteamIdInput(e.target.value); setSteamIdError(''); }}
                                    maxLength={17} placeholder="76561198XXXXXXXXX"
                                    className="flex-1 min-w-[180px] bg-background border border-border/50 rounded px-2.5 py-1 text-[11px] font-mono focus:outline-none focus:ring-1 focus:ring-primary/20 transition-all" />
                                <button onClick={handleApplySteamId} disabled={steamIdApplying || steamIdInput === steamIdSaved}
                                    className="px-3 py-1 bg-primary text-primary-foreground rounded text-[8px] font-black uppercase tracking-widest shadow-sm hover:brightness-110 transition-all disabled:opacity-50 flex-shrink-0">
                                    {steamIdApplying ? '...' : 'Apply'}
                                </button>
                                {steamIdError && <span className="text-[9px] text-red-400 font-bold flex-shrink-0">{steamIdError}</span>}
                                {steamIdSaved && steamIdInput === steamIdSaved && <span className="text-[9px] text-green-500 font-bold flex-shrink-0">{steamIdSaved}</span>}
                            </>)}
                        </div>
                    </div>
                    {/* UI Customization */}
                    <div className="flex items-center gap-3 flex-wrap pt-3 border-t border-border/40">
                        <p className="text-[10px] font-bold text-foreground">UI</p>
                        <label title="Show the hexadecimal item ID column in Inventory and Item Database tables." className="flex items-center gap-2 px-2.5 py-1.5 rounded bg-muted/20 border border-border/50 cursor-pointer hover:bg-muted/30 transition-all">
                            <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Show ID (HEX)</span>
                            <input type="checkbox" checked={columnVisibility.id} onChange={e => setColumnVisibility({ ...columnVisibility, id: e.target.checked })} className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20" />
                        </label>
                        <label title="Show the Category column in Inventory and Item Database tables." className="flex items-center gap-2 px-2.5 py-1.5 rounded bg-muted/20 border border-border/50 cursor-pointer hover:bg-muted/30 transition-all">
                            <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Show Category Column</span>
                            <input type="checkbox" checked={columnVisibility.category} onChange={e => setColumnVisibility({ ...columnVisibility, category: e.target.checked })} className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20" />
                        </label>
                        <label title="Enable verbose diagnostic logs and developer-only UI helpers." className="flex items-center gap-2 px-2.5 py-1.5 rounded bg-muted/20 border border-border/50 cursor-pointer hover:bg-muted/30 transition-all">
                            <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Debug Mode</span>
                            <input type="checkbox" checked={debugMode} onChange={e => setDebugMode(e.target.checked)} className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20" />
                        </label>
                    </div>
                </div>
            </section>

            {/* Deploy */}
            <section className="space-y-3">
                <div className={sectionHdr}><div className={dot} /><h2 className={hdrText}>Deploy</h2></div>
                <div className="card px-4 py-3 space-y-3">
                    <div className="flex items-center gap-2">
                        <select value={selectedTarget} onChange={e => setSelectedTarget(e.target.value)}
                            className="flex-1 bg-background border border-border/50 rounded px-2.5 py-1.5 text-[11px] font-mono focus:outline-none focus:ring-1 focus:ring-primary/20 transition-all">
                            <option value="">Select target...</option>
                            {targets.map(t => <option key={t.name} value={t.name}>{t.name} ({t.type === 'local' ? 'local' : t.host})</option>)}
                        </select>
                        <button onClick={() => { setEditTarget(new deploy.Target(EMPTY_SSH_TARGET)); setShowForm(true); }}
                            className={btnAction}>+ Add</button>
                        {selectedTarget && <>
                            <button onClick={() => { const t = targets.find(x => x.name === selectedTarget); if (t) { setEditTarget(new deploy.Target(t)); setShowForm(true); } }}
                                className={btnSecondary}>Edit</button>
                            <button onClick={() => handleDeleteTarget(selectedTarget)}
                                className={btnSecondary}>Del</button>
                        </>}
                    </div>
                    {selectedTarget && (
                        <div className="flex flex-wrap gap-1.5">
                            <button onClick={handleTestConnection} disabled={deploying} className={`${btnSm} bg-green-600 text-white shadow-sm hover:brightness-110 active:scale-95`}>Test</button>
                            <button onClick={handleUpload} disabled={deploying || !platform} className={btnAction}>Upload</button>
                            <button onClick={handleDownload} disabled={deploying} className={btnAction}>Download</button>
                            <button onClick={handleLaunch} disabled={deploying} className={btnAction}>Launch</button>
                            <button onClick={handleClose} disabled={deploying} className={`${btnSm} bg-red-600 text-white shadow-sm hover:brightness-110 active:scale-95`}>Close Game</button>
                            <button onClick={handleDeployAndLaunch} disabled={deploying || !platform} className={btnAction}>Deploy & Launch</button>
                            <button onClick={handleCloseAndDownload} disabled={deploying} className={`${btnSm} bg-orange-600 text-white shadow-sm hover:brightness-110 active:scale-95`}>Close & Download</button>
                            <button onClick={() => setShowSaveManager(true)} disabled={deploying} className={btnAction}>Save Manager</button>
                        </div>
                    )}
                    {showForm && (
                        <div className="border border-border/50 rounded p-3 space-y-2.5 bg-muted/10">
                            <div className="flex items-center justify-between">
                                <p className="text-[10px] font-bold text-foreground">{editTarget.name && targets.some(t => t.name === editTarget.name) ? 'Edit Target' : 'Add Target'}</p>
                                <div className="flex bg-muted/30 p-0.5 rounded border border-border">
                                    {(['ssh', 'local'] as const).map(tp => (
                                        <button key={tp} onClick={() => {
                                            const base = tp === 'local' ? EMPTY_LOCAL_TARGET : EMPTY_SSH_TARGET;
                                            setEditTarget(new deploy.Target({...base, name: editTarget.name, savePath: editTarget.savePath} as deploy.Target));
                                        }}
                                            className={`px-3 py-0.5 rounded text-[8px] font-black uppercase tracking-widest transition-all ${editTarget.type === tp ? 'bg-primary text-primary-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                                        >{tp}</button>
                                    ))}
                                </div>
                            </div>
                            <div className={`grid gap-2 ${editTarget.type === 'local' ? 'grid-cols-1' : 'grid-cols-2 md:grid-cols-4'}`}>
                                <div className="space-y-0.5"><label className={labelCls}>Name</label><input value={editTarget.name} onChange={e => setEditTarget({...editTarget, name: e.target.value} as deploy.Target)} placeholder={editTarget.type === 'local' ? 'Local PC' : 'Steam Deck'} className={inputCls} /></div>
                                {editTarget.type === 'ssh' && <>
                                    <div className="space-y-0.5"><label className={labelCls}>Host</label><input value={editTarget.host} onChange={e => setEditTarget({...editTarget, host: e.target.value} as deploy.Target)} placeholder="192.168.1.100" className={inputCls} /></div>
                                    <div className="space-y-0.5"><label className={labelCls}>Port</label><input type="number" value={editTarget.port} onChange={e => setEditTarget({...editTarget, port: parseInt(e.target.value) || 22} as deploy.Target)} className={inputCls} /></div>
                                    <div className="space-y-0.5"><label className={labelCls}>User</label><input value={editTarget.user} onChange={e => setEditTarget({...editTarget, user: e.target.value} as deploy.Target)} placeholder="deck" className={inputCls} /></div>
                                </>}
                            </div>
                            <div className={`grid gap-2 ${editTarget.type === 'local' ? 'grid-cols-1' : 'grid-cols-1 md:grid-cols-2'}`}>
                                {editTarget.type === 'ssh' && (
                                    <div className="space-y-0.5"><label className={labelCls}>SSH Key Path</label><input value={editTarget.keyPath} onChange={e => setEditTarget({...editTarget, keyPath: e.target.value} as deploy.Target)} className={inputCls} /></div>
                                )}
                                <div className="space-y-0.5"><label className={labelCls}>Save Path</label><input value={editTarget.savePath} onChange={e => setEditTarget({...editTarget, savePath: e.target.value} as deploy.Target)} placeholder={editTarget.type === 'local' ? 'C:\\Users\\...\\EldenRing\\{STEAM_ID}\\ER0000.sl2' : ''} className={inputCls} /></div>
                            </div>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                                <div className="space-y-0.5"><label className={labelCls}>Start Command <span className="text-muted-foreground/50">(empty = auto-detect)</span></label><input value={editTarget.gameStartCmd} onChange={e => setEditTarget({...editTarget, gameStartCmd: e.target.value} as deploy.Target)} className={inputCls} /></div>
                                <div className="space-y-0.5"><label className={labelCls}>Stop Command <span className="text-muted-foreground/50">(empty = auto-detect)</span></label><input value={editTarget.gameStopCmd} onChange={e => setEditTarget({...editTarget, gameStopCmd: e.target.value} as deploy.Target)} className={inputCls} /></div>
                            </div>
                            <div className="flex gap-1.5 pt-1">
                                <button onClick={handleSaveTarget} className={btnAction}>Save</button>
                                <button onClick={() => setShowForm(false)} className={btnSecondary}>Cancel</button>
                            </div>
                        </div>
                    )}
                </div>
            </section>

            {/* Safety */}
            <section className="space-y-3">
                <div className={sectionHdr}><div className={dot} /><h2 className={hdrText}>Safety</h2></div>
                <div className="card px-4 py-3">
                    <div role="radiogroup" aria-label="Safety profile" className="flex gap-3">
                        {([
                            {
                                value: 'safe' as const, label: 'Safe',
                                desc: 'Conservative vanilla caps, risk-flagged items hidden, online safety on. Recommended default.',
                                active: 'bg-green-600/10 border-green-600/50', title: 'text-foreground',
                            },
                            {
                                value: 'expanded_limits' as const, label: 'Expanded Limits',
                                desc: 'Technical game caps for normal items — cut-content and ban-risk items stay hidden. Not Chaos Mode.',
                                active: 'bg-amber-500/10 border-amber-500/50', title: 'text-amber-500',
                            },
                            {
                                value: 'chaos' as const, label: 'Chaos',
                                desc: 'Technical game caps and reveals risk-flagged (cut / ban-risk) items. Practically guarantees an EAC ban online.',
                                active: 'bg-red-500/10 border-red-500/50', title: 'text-red-500',
                            },
                        ]).map(p => {
                            const selected = safetyProfile === p.value;
                            return (
                                <button
                                    key={p.value}
                                    type="button"
                                    role="radio"
                                    aria-checked={selected}
                                    onClick={() => selectProfile(p.value)}
                                    className={`flex-1 flex flex-col gap-1.5 p-3 rounded border text-left transition-all cursor-pointer ${selected ? p.active : 'bg-muted/20 border-border/50 hover:bg-muted/30'}`}
                                >
                                    <div className="flex items-center justify-between gap-2">
                                        <span className={`text-[10px] font-black uppercase tracking-widest ${p.title}`}>{p.label}</span>
                                        <span className={`w-3.5 h-3.5 rounded-full border-2 shrink-0 ${selected ? 'border-current bg-current' : 'border-border'}`} />
                                    </div>
                                    <p className="text-[9px] text-muted-foreground leading-relaxed">{p.desc}</p>
                                </button>
                            );
                        })}
                    </div>
                </div>
            </section>

            {/* Tools */}
            <section className="space-y-3">
                <div className={sectionHdr}><div className={dot} /><h2 className={hdrText}>Tools</h2></div>
                <div className="card px-4 py-3">
                    <div className="flex flex-wrap gap-2">
                        <button onClick={() => setView('favorites')}
                            className="flex items-center gap-2 px-3 py-2 rounded bg-primary text-primary-foreground hover:brightness-110 transition-all text-[9px] font-black uppercase tracking-widest shadow-sm active:scale-95">
                            <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
                                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                            </svg>
                            Favorite Items{favCount > 0 ? ` (${favCount})` : ''}
                        </button>
                        <button
                            onClick={handleDiagnostics}
                            disabled={scanning || !platform}
                            className="flex items-center gap-2 px-3 py-2 rounded bg-primary text-primary-foreground hover:brightness-110 transition-all text-[9px] font-black uppercase tracking-widest shadow-sm active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            {scanning
                                ? <div className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                                : <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                                </svg>
                            }
                            {scanning ? 'Scanning…' : 'Diagnostics'}
                        </button>
                        <div className="flex items-center gap-2 px-3 py-2 rounded bg-muted/30 border border-border text-muted-foreground opacity-50 cursor-not-allowed text-[9px] font-black uppercase tracking-widest">
                            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                            </svg>
                            Save Comparison <span className="ml-1 text-[8px] opacity-60">(soon)</span>
                        </div>
                        {/* PS4<->PC format conversion is disabled for now (kept, not removed).
                            Rendered as an inert button so it can never invoke the conversion flow. */}
                        <button
                            type="button"
                            disabled
                            title="Format conversion is currently unavailable"
                            className="flex items-center gap-2 px-3 py-2 rounded bg-muted/30 border border-border text-muted-foreground opacity-50 cursor-not-allowed text-[9px] font-black uppercase tracking-widest">
                            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                            </svg>
                            Convert Format <span className="ml-1 text-[8px] opacity-60">(unavailable)</span>
                        </button>
                    </div>
                </div>
            </section>
        </div>

        {inventoryIssuesModal && (
            <InventoryIssuesModal
                reports={inventoryIssuesModal.reports}
                charIndex={charIndex}
                onClose={() => setInventoryIssuesModal(null)}
                onSaved={() => {
                    setInventoryIssuesModal(null);
                    onMutate?.();
                }}
            />
        )}
        {showSaveManager && (
            <SaveManagerModal
                targets={targets}
                initialTarget={selectedTarget}
                platform={platform}
                onAfterLoad={onAfterLoad}
                onClose={() => setShowSaveManager(false)}
            />
        )}
        {chaosModalOpen && (
            <ChaosWarningModal
                onConfirm={confirmChaos}
                onCancel={() => setChaosModalOpen(false)}
            />
        )}
        </>
    );
}
