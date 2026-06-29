import {useState, useEffect, useCallback} from 'react';
import {createPortal} from 'react-dom';
import {
    ListSaveBackups,
    SetActiveBackup,
    UnsetActiveBackup,
    DeleteSaveBackup,
    CreateManualBackup,
    UpdateBackupMeta,
    DownloadBackupFile,
    LoadSaveFromPath,
} from '../../wailsjs/go/main/App';
import {deploy} from '../../wailsjs/go/models';
import toast from '../lib/toast';

interface Props {
    targets: deploy.Target[];
    initialTarget: string;
    platform: string | null;
    onAfterLoad: (platform: string) => Promise<void> | void;
    onClose: () => void;
}

type EditState = {name: string; tags: string; desc: string} | null;
type ConfirmState =
    | {kind: 'delete'; backupName: string}
    | {kind: 'load'; localPath: string}
    | null;

function formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatTimestamp(ts: string): string {
    if (!ts) return '—';
    try {
        return new Date(ts).toLocaleString();
    } catch {
        return ts;
    }
}

export function SaveManagerModal({targets, initialTarget, platform, onAfterLoad, onClose}: Props) {
    const [selectedTarget, setSelectedTarget] = useState(initialTarget || (targets[0]?.name ?? ''));
    const [backups, setBackups] = useState<deploy.SaveBackupEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [busy, setBusy] = useState(false);
    const [edit, setEdit] = useState<EditState>(null);
    const [confirm, setConfirm] = useState<ConfirmState>(null);

    const loadBackups = useCallback(async (target: string) => {
        if (!target) { setBackups([]); return; }
        setLoading(true);
        try {
            const list = await ListSaveBackups(target);
            setBackups((list ?? []).sort((a, b) => {
                const ta = new Date(a.timestamp).getTime();
                const tb = new Date(b.timestamp).getTime();
                return tb - ta;
            }));
        } catch (e) {
            toast.error(String(e));
            setBackups([]);
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => { loadBackups(selectedTarget); }, [selectedTarget, loadBackups]);

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && !confirm) onClose(); };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, [onClose, confirm]);

    const withBusy = async (fn: () => Promise<void>) => {
        setBusy(true);
        try { await fn(); } catch (e) { toast.error(String(e)); } finally { setBusy(false); }
    };

    const handleNewBackup = () => withBusy(async () => {
        await CreateManualBackup(selectedTarget);
        toast.success('Backup created');
        await loadBackups(selectedTarget);
    });

    const handleSetActive = (name: string) => withBusy(async () => {
        await SetActiveBackup(selectedTarget, name);
        toast.success('Set as active save');
        await loadBackups(selectedTarget);
    });

    const handleUnsetActive = () => withBusy(async () => {
        await UnsetActiveBackup(selectedTarget);
        toast.success('Active save removed');
        await loadBackups(selectedTarget);
    });

    const handleDeleteConfirmed = async (backupName: string) => {
        setConfirm(null);
        setBusy(true);
        try {
            await DeleteSaveBackup(selectedTarget, backupName);
            toast.success('Backup deleted');
            await loadBackups(selectedTarget);
        } catch (e) {
            toast.error(String(e));
        } finally {
            setBusy(false);
        }
    };

    const handleSaveMeta = (name: string, tags: string, desc: string) => withBusy(async () => {
        const tagList = tags.split(',').map(t => t.trim()).filter(Boolean);
        await UpdateBackupMeta(selectedTarget, name, tagList, desc);
        toast.success('Metadata saved');
        setEdit(null);
        await loadBackups(selectedTarget);
    });

    const handleDownload = (name: string) => withBusy(async () => {
        const localPath = await DownloadBackupFile(selectedTarget, name);
        if (!localPath) return; // cancelled
        toast.success('Downloaded');
        // Ask if user wants to load into editor
        setConfirm({kind: 'load', localPath});
    });

    const handleLoadConfirmed = async (localPath: string) => {
        setConfirm(null);
        setBusy(true);
        try {
            const plat = await LoadSaveFromPath(localPath);
            await onAfterLoad(plat);
            toast.success('Save loaded into editor');
            onClose();
        } catch (e) {
            toast.error(String(e));
        } finally {
            setBusy(false);
        }
    };

    const activeBackup = backups.find(b => b.isActive);

    const btnSm = 'px-2.5 py-1 rounded text-[8px] font-black uppercase tracking-widest transition-all disabled:opacity-50';
    const btnAction = `${btnSm} bg-primary text-primary-foreground shadow-sm hover:brightness-110 active:scale-95`;
    const btnSecondary = `${btnSm} bg-muted/30 text-foreground border border-border hover:bg-muted/50`;
    const btnDanger = `${btnSm} bg-red-600 text-white shadow-sm hover:brightness-110 active:scale-95`;
    const btnGreen = `${btnSm} bg-green-600 text-white shadow-sm hover:brightness-110 active:scale-95`;

    const modal = (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
            <div className="bg-background border border-border rounded-xl shadow-2xl w-full max-w-2xl mx-4 flex flex-col max-h-[85vh]">
                {/* Header */}
                <div className="flex items-center justify-between px-5 py-3 border-b border-border flex-shrink-0">
                    <h2 className="text-[11px] font-black uppercase tracking-widest text-foreground">Save Manager</h2>
                    <button onClick={onClose} className="text-muted-foreground hover:text-foreground transition-colors text-lg leading-none">×</button>
                </div>

                {/* Target selector */}
                <div className="flex items-center gap-2 px-5 py-3 border-b border-border flex-shrink-0">
                    <label className="text-[9px] font-black uppercase tracking-widest text-muted-foreground w-14">Target</label>
                    <select
                        value={selectedTarget}
                        onChange={e => setSelectedTarget(e.target.value)}
                        className="flex-1 bg-background border border-border/50 rounded px-2.5 py-1.5 text-[11px] font-mono focus:outline-none focus:ring-1 focus:ring-primary/20"
                    >
                        <option value="">Select target...</option>
                        {targets.map(t => (
                            <option key={t.name} value={t.name}>{t.name} ({t.type === 'local' ? 'local' : t.host})</option>
                        ))}
                    </select>
                    {selectedTarget && (
                        <button onClick={handleNewBackup} disabled={busy} className={btnAction}>
                            + New Backup
                        </button>
                    )}
                </div>

                {/* Active save status */}
                {selectedTarget && !loading && (
                    <div className="px-5 py-2 border-b border-border flex-shrink-0 flex items-center gap-2 min-h-[36px]">
                        {activeBackup ? (
                            <>
                                <span className="text-[8px] font-black uppercase tracking-widest text-green-400">● Active</span>
                                <span className="text-[10px] font-mono text-muted-foreground flex-1 truncate">{activeBackup.name}</span>
                                <button onClick={handleUnsetActive} disabled={busy} className={`${btnSm} bg-orange-600 text-white shadow-sm hover:brightness-110 active:scale-95`}>
                                    Unset Active
                                </button>
                            </>
                        ) : (
                            <span className="text-[9px] text-muted-foreground">No active save set</span>
                        )}
                    </div>
                )}

                {/* Backup list */}
                <div className="overflow-y-auto flex-1 px-5 py-3 space-y-2">
                    {!selectedTarget && (
                        <p className="text-[10px] text-muted-foreground text-center py-6">Select a target to view backups</p>
                    )}
                    {selectedTarget && loading && (
                        <p className="text-[10px] text-muted-foreground text-center py-6">Loading...</p>
                    )}
                    {selectedTarget && !loading && backups.length === 0 && (
                        <p className="text-[10px] text-muted-foreground text-center py-6">No backups found. Create one with "New Backup".</p>
                    )}
                    {backups.map(b => (
                        <div key={b.name} className={`border rounded-lg px-3 py-2.5 space-y-1.5 transition-colors ${b.isActive ? 'border-green-500/40 bg-green-500/5' : 'border-border bg-muted/5'}`}>
                            {/* Row 1: name + badges + actions */}
                            <div className="flex items-center gap-2 flex-wrap">
                                {b.isActive && (
                                    <span className="text-[7px] font-black uppercase tracking-widest bg-green-600 text-white px-1.5 py-0.5 rounded">Active</span>
                                )}
                                <span className="text-[10px] font-mono text-foreground flex-1 min-w-0 truncate">{b.name}</span>
                                <div className="flex items-center gap-1 flex-shrink-0">
                                    {!b.isActive && (
                                        <button onClick={() => handleSetActive(b.name)} disabled={busy} className={btnGreen}>
                                            Set Active
                                        </button>
                                    )}
                                    <button onClick={() => setEdit({name: b.name, tags: (b.tags ?? []).join(', '), desc: b.desc ?? ''})} disabled={busy} className={btnSecondary}>
                                        Edit
                                    </button>
                                    <button onClick={() => handleDownload(b.name)} disabled={busy} className={btnAction}>
                                        Download
                                    </button>
                                    <button
                                        onClick={() => {
                                            if (b.isActive) { toast.error('Cannot delete active backup: unset Active first'); return; }
                                            setConfirm({kind: 'delete', backupName: b.name});
                                        }}
                                        disabled={busy}
                                        className={btnDanger}
                                    >
                                        Delete
                                    </button>
                                </div>
                            </div>

                            {/* Row 2: metadata */}
                            <div className="flex items-center gap-3 text-[9px] text-muted-foreground">
                                <span>{formatTimestamp(b.timestamp)}</span>
                                <span>{formatBytes(b.size)}</span>
                                {b.md5 && <span className="font-mono opacity-60">{b.md5.slice(0, 8)}…</span>}
                            </div>

                            {/* Tags + desc */}
                            {((b.tags ?? []).length > 0 || b.desc) && edit?.name !== b.name && (
                                <div className="flex flex-wrap gap-1 items-start">
                                    {(b.tags ?? []).map(tag => (
                                        <span key={tag} className="text-[8px] bg-primary/20 text-primary px-1.5 py-0.5 rounded">{tag}</span>
                                    ))}
                                    {b.desc && <span className="text-[9px] text-muted-foreground">{b.desc}</span>}
                                </div>
                            )}

                            {/* Inline edit form */}
                            {edit?.name === b.name && (
                                <div className="space-y-1.5 pt-1">
                                    <div className="space-y-0.5">
                                        <label className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Tags (comma-separated)</label>
                                        <input
                                            value={edit.tags}
                                            onChange={e => setEdit({...edit, tags: e.target.value})}
                                            className="w-full bg-background border border-border/50 rounded px-2 py-1 text-[10px] font-mono focus:outline-none focus:ring-1 focus:ring-primary/20"
                                            placeholder="pvp, cosplay, pre-boss"
                                        />
                                    </div>
                                    <div className="space-y-0.5">
                                        <label className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Description</label>
                                        <input
                                            value={edit.desc}
                                            onChange={e => setEdit({...edit, desc: e.target.value})}
                                            className="w-full bg-background border border-border/50 rounded px-2 py-1 text-[10px] font-mono focus:outline-none focus:ring-1 focus:ring-primary/20"
                                            placeholder="Before Malenia attempt"
                                        />
                                    </div>
                                    <div className="flex gap-1.5">
                                        <button onClick={() => handleSaveMeta(edit.name, edit.tags, edit.desc)} disabled={busy} className={btnAction}>Save</button>
                                        <button onClick={() => setEdit(null)} className={btnSecondary}>Cancel</button>
                                    </div>
                                </div>
                            )}
                        </div>
                    ))}
                </div>

                {/* Footer */}
                <div className="px-5 py-3 border-t border-border flex-shrink-0 flex justify-end">
                    <button onClick={onClose} className={btnSecondary}>Close</button>
                </div>
            </div>

            {/* Confirm: delete */}
            {confirm?.kind === 'delete' && (
                <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/50">
                    <div className="bg-background border border-border rounded-xl shadow-2xl p-5 max-w-sm mx-4 space-y-4">
                        <p className="text-[11px] font-bold text-foreground">Delete backup?</p>
                        <p className="text-[10px] text-muted-foreground font-mono break-all">{confirm.backupName}</p>
                        <p className="text-[10px] text-muted-foreground">This action is permanent and cannot be undone.</p>
                        <div className="flex gap-2 justify-end">
                            <button onClick={() => setConfirm(null)} className={btnSecondary}>Cancel</button>
                            <button onClick={() => handleDeleteConfirmed(confirm.backupName)} className={btnDanger}>Delete</button>
                        </div>
                    </div>
                </div>
            )}

            {/* Confirm: load into editor */}
            {confirm?.kind === 'load' && (
                <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/50">
                    <div className="bg-background border border-border rounded-xl shadow-2xl p-5 max-w-sm mx-4 space-y-4">
                        <p className="text-[11px] font-bold text-foreground">Load into editor?</p>
                        {platform && (
                            <p className="text-[10px] text-yellow-400">Warning: unsaved changes to the current file will be lost.</p>
                        )}
                        <p className="text-[10px] text-muted-foreground">Load the downloaded backup into the editor, or just keep the saved file.</p>
                        <div className="flex gap-2 justify-end">
                            <button onClick={() => setConfirm(null)} className={btnSecondary}>Just save</button>
                            <button onClick={() => handleLoadConfirmed(confirm.localPath)} className={btnAction}>Load</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );

    return createPortal(modal, document.body);
}
