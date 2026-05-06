import {useState} from 'react';
import toast from '../lib/toast';
import {CharacterImporter} from './CharacterImporter';
import {PresetImporter} from './PresetImporter';
import {FavoritesManager} from './FavoritesManager';
import {BanScanPanel} from './BanScanPanel';
import {ExportCharacterPresetToFile} from '../../wailsjs/go/main/App';
import {useFavorites} from '../state/favorites';
import type {AddSettings} from '../App';
import {vm} from '../../wailsjs/go/models';

interface ToolsTabProps {
    charIndex: number;
    onComplete: () => void;
    onMutate?: () => void;
    addSettings: AddSettings;
    onAddSettingsApplied?: (s: AddSettings) => void;
}

type ToolView = 'overview' | 'importer' | 'preset-import' | 'favorites' | 'ban-scan';

export function ToolsTab({charIndex, onComplete, onMutate, addSettings, onAddSettingsApplied}: ToolsTabProps) {
    const [view, setView] = useState<ToolView>('overview');
    const {count: favCount} = useFavorites();

    const handleExportPreset = async () => {
        try {
            const s = new vm.PresetAddSettings({
                upgrade25: addSettings.upgrade25,
                upgrade10: addSettings.upgrade10,
                infuseOffset: addSettings.infuseOffset,
                upgradeAsh: addSettings.upgradeAsh,
                talismansHighestOnly: addSettings.talismansHighestOnly,
            });
            const path = await ExportCharacterPresetToFile(charIndex, s);
            if (path) {
                toast.success('Preset exported to: ' + path);
            }
        } catch (e) {
            toast.error('Export failed: ' + e);
        }
    };

    if (view === 'importer') {
        return (
            <div className="space-y-3 animate-in fade-in duration-300">
                <button onClick={() => setView('overview')}
                    className="flex items-center gap-1.5 text-[9px] font-black uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors">
                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M15 19l-7-7 7-7" />
                    </svg>
                    Back to Tools
                </button>
                <CharacterImporter destSlot={charIndex} onComplete={onComplete} />
            </div>
        );
    }

    if (view === 'preset-import') {
        return (
            <div className="space-y-3 animate-in fade-in duration-300">
                <button onClick={() => setView('overview')}
                    className="flex items-center gap-1.5 text-[9px] font-black uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors">
                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M15 19l-7-7 7-7" />
                    </svg>
                    Back to Tools
                </button>
                <PresetImporter charIndex={charIndex} onComplete={onComplete} onMutate={onMutate} onAddSettingsApplied={onAddSettingsApplied} />
            </div>
        );
    }

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

    if (view === 'ban-scan') {
        return (
            <div className="space-y-3 animate-in fade-in duration-300">
                <button onClick={() => setView('overview')}
                    className="flex items-center gap-1.5 text-[9px] font-black uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors">
                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M15 19l-7-7 7-7" />
                    </svg>
                    Back to Tools
                </button>
                <BanScanPanel />
            </div>
        );
    }

    return (
        <div className="space-y-6 animate-in fade-in duration-500 max-w-4xl mx-auto">
            <div className="flex items-center space-x-2">
                <div className="w-1 h-3 bg-primary rounded-full" />
                <h3 className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Tools</h3>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {/* Export Preset */}
                <button onClick={handleExportPreset}
                    className="card p-5 text-left hover:border-green-500/40 hover:bg-green-500/5 transition-all group">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-green-500/10 flex items-center justify-center flex-shrink-0 group-hover:bg-green-500/20 transition-colors">
                            <svg className="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                            </svg>
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">Export Preset</h4>
                            <p className="text-[9px] text-muted-foreground mt-1">Save character stats, inventory and storage to a .preset.json file</p>
                        </div>
                    </div>
                </button>

                {/* Import Preset */}
                <button onClick={() => setView('preset-import')}
                    className="card p-5 text-left hover:border-blue-500/40 hover:bg-blue-500/5 transition-all group">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-blue-500/10 flex items-center justify-center flex-shrink-0 group-hover:bg-blue-500/20 transition-colors">
                            <svg className="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                            </svg>
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">Import Preset</h4>
                            <p className="text-[9px] text-muted-foreground mt-1">Load a .preset.json file and apply to the current character</p>
                        </div>
                    </div>
                </button>

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

                {/* Character Importer */}
                <button onClick={() => setView('importer')}
                    className="card p-5 text-left hover:border-primary/40 hover:bg-muted/10 transition-all group">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center flex-shrink-0 group-hover:bg-primary/20 transition-colors">
                            <svg className="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                            </svg>
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">Character Importer</h4>
                            <p className="text-[9px] text-muted-foreground mt-1">Import character from another save file into the selected slot</p>
                        </div>
                    </div>
                </button>

                {/* Ban Risk Scan */}
                <button onClick={() => setView('ban-scan')}
                    className="card p-5 text-left hover:border-red-500/40 hover:bg-red-500/5 transition-all group">
                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-lg bg-red-500/10 flex items-center justify-center flex-shrink-0 group-hover:bg-red-500/20 transition-colors">
                            <svg className="w-5 h-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                            </svg>
                        </div>
                        <div>
                            <h4 className="text-[11px] font-black uppercase tracking-wider text-foreground">Ban Risk Scan</h4>
                            <p className="text-[9px] text-muted-foreground mt-1">Scan all slots for cut content, illegal stats, upgrade violations and SteamID mismatches</p>
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
