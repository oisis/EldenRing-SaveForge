import {useState} from 'react';
import toast from '../lib/toast';
import {FavoritesManager} from './FavoritesManager';
import {PrepareConversion, ExecuteConversion} from '../../wailsjs/go/main/App';
import {useFavorites} from '../state/favorites';

interface ToolsTabProps {
    charIndex: number;
    onComplete: () => void;
    onMutate?: () => void;
}

type ToolView = 'overview' | 'favorites';

export function ToolsTab({charIndex, onComplete, onMutate}: ToolsTabProps) {
    const [view, setView] = useState<ToolView>('overview');
    const {count: favCount} = useFavorites();

    // Conversion flow state
    const [convStep, setConvStep] = useState<'idle' | 'selecting' | 'steamid' | 'converting'>('idle');
    const [convSourcePath, setConvSourcePath] = useState('');
    const [convTargetPlatform, setConvTargetPlatform] = useState('');
    const [steamIDInput, setSteamIDInput] = useState('');
    const [steamIDError, setSteamIDError] = useState('');

    const handleConvertClick = async () => {
        setConvStep('selecting');
        try {
            const info = await PrepareConversion();
            const target = info.platform === 'PC' ? 'PS4' : 'PC';
            setConvSourcePath(info.path);
            setConvTargetPlatform(target);

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
        <div className="space-y-6 animate-in fade-in duration-500 max-w-4xl mx-auto">
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

            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {/* Favorite Items */}
                <button onClick={() => setView('favorites')}
                    className="card p-5 text-left hover:border-amber-500/40 hover:bg-amber-500/5 transition-all group">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-amber-500/10 flex items-center justify-center flex-shrink-0 group-hover:bg-amber-500/20 transition-colors">
                            <svg className="w-5 h-5 text-amber-500" fill="currentColor" viewBox="0 0 24 24">
                                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                            </svg>
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">Favorite Items</h4>
                            <p className="text-[9px] text-muted-foreground mt-1">Browse and manage your favorite items{favCount > 0 ? ` (${favCount})` : ''}</p>
                        </div>
                    </div>
                </button>

                {/* Convert Save Format */}
                <button onClick={handleConvertClick} disabled={converting}
                    className="card p-5 text-left hover:border-violet-500/40 hover:bg-violet-500/5 transition-all group disabled:opacity-50 disabled:cursor-not-allowed">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-violet-500/10 flex items-center justify-center flex-shrink-0 group-hover:bg-violet-500/20 transition-colors">
                            {converting
                                ? <div className="w-5 h-5 border-2 border-violet-500/20 border-t-violet-500 rounded-full animate-spin" />
                                : <svg className="w-5 h-5 text-violet-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                                </svg>
                            }
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">
                                Convert Format
                            </h4>
                            <p className="text-[9px] text-muted-foreground mt-1">
                                Convert any save file between PC (.sl2) and PS4 (.dat)
                            </p>
                        </div>
                    </div>
                </button>

                {/* Save Comparison — placeholder */}
                <div className="card p-5 text-left opacity-50 cursor-not-allowed">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-info/10 flex items-center justify-center flex-shrink-0">
                            <svg className="w-5 h-5 text-info" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                            </svg>
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">Save Comparison</h4>
                            <p className="text-[9px] text-muted-foreground mt-1">Compare two save files side by side (coming soon)</p>
                        </div>
                    </div>
                </div>

                {/* Diagnostics — placeholder */}
                <div className="card p-5 text-left opacity-50 cursor-not-allowed">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-warning/10 flex items-center justify-center flex-shrink-0">
                            <svg className="w-5 h-5 text-warning" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                            </svg>
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">Diagnostics</h4>
                            <p className="text-[9px] text-muted-foreground mt-1">Detect and repair save file corruption (coming soon)</p>
                        </div>
                    </div>
                </div>

                {/* Backup Manager — placeholder */}
                <div className="card p-5 text-left opacity-50 cursor-not-allowed">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-destructive/10 flex items-center justify-center flex-shrink-0">
                            <svg className="w-5 h-5 text-destructive" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" />
                            </svg>
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">Backup Manager</h4>
                            <p className="text-[9px] text-muted-foreground mt-1">Browse and restore backup save files (coming soon)</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
