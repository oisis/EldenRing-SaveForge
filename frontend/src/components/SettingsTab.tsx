import {useState, useEffect, useCallback} from 'react';
import toast from '../lib/toast';
import {
    GetSteamIDString, SetSteamIDFromString,
    GetDeployTargets, SaveDeployTarget, DeleteDeployTarget,
    TestSSHConnection, DeploySave, DownloadRemoteSave,
    LaunchRemoteGame, CloseRemoteGame, DeployAndLaunch, CloseAndDownload,
} from '../../wailsjs/go/main/App';
import {deploy} from '../../wailsjs/go/models';
import {useSafetyMode} from '../state/safetyMode';

interface SettingsTabProps {
    theme: 'light' | 'dark' | 'golden';
    setTheme: (theme: 'light' | 'dark' | 'golden') => void;
    columnVisibility: { id: boolean; category: boolean };
    setColumnVisibility: (visibility: { id: boolean; category: boolean }) => void;
    showFlaggedItems: boolean;
    setShowFlaggedItems: (value: boolean) => void;
    debugMode: boolean;
    setDebugMode: (value: boolean) => void;
    platform: string | null;
    setPlatform: (platform: string | null) => void;
    refreshSlots: () => void;
    selectedDeployTarget: string;
    setSelectedDeployTarget: (v: string) => void;
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
    showFlaggedItems, setShowFlaggedItems, debugMode, setDebugMode,
    platform, setPlatform, refreshSlots,
    selectedDeployTarget: selectedTarget, setSelectedDeployTarget: setSelectedTarget,
}: SettingsTabProps) {
    const safetyMode = useSafetyMode();
    const [steamIdInput, setSteamIdInput] = useState('');
    const [steamIdSaved, setSteamIdSaved] = useState('');
    const [steamIdError, setSteamIdError] = useState('');
    const [steamIdApplying, setSteamIdApplying] = useState(false);

    const [fullChaosMode, setFullChaosMode] = useState<boolean>(() =>
        localStorage.getItem('setting:fullChaosMode') === 'true');
    const handleChaosToggle = (checked: boolean) => {
        setFullChaosMode(checked);
        localStorage.setItem('setting:fullChaosMode', String(checked));
        window.dispatchEvent(new CustomEvent('fullChaosModeChanged', { detail: checked }));
    };


    // Deploy state
    const [targets, setTargets] = useState<deploy.Target[]>([]);
    const [editTarget, setEditTarget] = useState<deploy.Target>(new deploy.Target(EMPTY_SSH_TARGET));
    const [showForm, setShowForm] = useState(false);
    const [deploying, setDeploying] = useState(false);

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
        try { const plat = await DownloadRemoteSave(selectedTarget); setPlatform(plat); refreshSlots(); toast.success('Downloaded & loaded', { id: tid }); }
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
        try { const plat = await CloseAndDownload(selectedTarget); setPlatform(plat); refreshSlots(); toast.success('Game closed & save loaded', { id: tid }); }
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

    return (
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
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
                    {/* UI Customization — all toggles inline */}
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
                <div className="card px-4 py-3 space-y-2">
                    <label className="flex items-start justify-between gap-4 p-2.5 rounded bg-muted/20 border border-border/50 cursor-pointer hover:bg-muted/30 transition-all">
                        <div className="flex-1 space-y-1">
                            <span className="text-[10px] font-black uppercase tracking-widest text-foreground block">Online Safety Mode</span>
                            <p className="text-[10px] text-muted-foreground leading-relaxed">
                                <strong>Action gating.</strong> When enabled: Tier 2 edits (cut content, illegal stat values, runes &gt; 999M) are <strong>disabled</strong>;
                                Tier 1 actions (bulk grace unlock, map reveal, etc.) <strong>require explicit confirmation</strong> every time.
                                Recommended when actively playing online.
                            </p>
                        </div>
                        <input
                            type="checkbox"
                            checked={safetyMode.enabled}
                            onChange={e => safetyMode.setEnabled(e.target.checked)}
                            className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20 shrink-0 mt-1"
                        />
                    </label>
                    <label className="flex items-start justify-between gap-4 p-2.5 rounded bg-muted/20 border border-border/50 cursor-pointer hover:bg-muted/30 transition-all">
                        <div className="flex-1 space-y-1">
                            <span className="text-[10px] font-black uppercase tracking-widest text-foreground block">Show Cut &amp; Ban-Risk Items</span>
                            <p className="text-[10px] text-muted-foreground leading-relaxed">
                                <strong>List visibility.</strong> When enabled, cut content and ban-risk items appear in Item Database, Inventory and Gestures lists (with the ⚠ marker).
                                Disable to hide them from view entirely. Independent from Online Safety Mode.
                            </p>
                        </div>
                        <input
                            type="checkbox"
                            checked={showFlaggedItems}
                            onChange={e => setShowFlaggedItems(e.target.checked)}
                            className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20 shrink-0 mt-1"
                        />
                    </label>
                    <label className="flex items-start justify-between gap-4 p-2.5 rounded bg-red-500/5 border border-red-500/30 cursor-pointer hover:bg-red-500/10 transition-all">
                        <div className="flex-1 space-y-1">
                            <span className="text-[10px] font-black uppercase tracking-widest text-red-500 block">Full Chaos Mode</span>
                            <p className="text-[10px] text-muted-foreground leading-relaxed">
                                <strong className="text-red-500/90">Bypasses all item caps.</strong> When enabled, the Item Database modal allows
                                adding any quantity ignoring vanilla single-playthrough limits and NG+ scaling.
                                <strong> Strongly increases EAC ban risk.</strong> Use only on offline / experimental saves.
                            </p>
                        </div>
                        <input
                            type="checkbox"
                            checked={fullChaosMode}
                            onChange={e => handleChaosToggle(e.target.checked)}
                            className="w-3.5 h-3.5 rounded border-red-500/40 text-red-500 focus:ring-red-500/20 shrink-0 mt-1"
                        />
                    </label>
                </div>
            </section>
        </div>
    );
}
