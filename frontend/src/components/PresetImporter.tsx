import {useState} from 'react';
import toast from '../lib/toast';
import {
    LoadCharacterPresetFromFile,
    LoadCharacterPresetFromURL,
    ValidateCharacterPreset,
    ApplyCharacterPreset,
} from '../../wailsjs/go/main/App';
import {vm} from '../../wailsjs/go/models';
import {RiskActionButton} from './RiskActionButton';
import type {AddSettings} from '../App';

interface Props {
    charIndex: number;
    onComplete: () => void;
    onMutate?: () => void;
    onAddSettingsApplied?: (s: AddSettings) => void;
}

export function PresetImporter({charIndex, onComplete, onMutate, onAddSettingsApplied}: Props) {
    const [preset, setPreset] = useState<vm.CharacterPreset | null>(null);
    const [warnings, setWarnings] = useState<string[]>([]);
    const [options, setOptions] = useState({
        replaceStats: true,
        replaceInventory: true,
        replaceStorage: true,
        replaceWorld: true,
        keepName: false,
        keepClass: false,
    });
    const [loading, setLoading] = useState(false);
    const [result, setResult] = useState<vm.PresetApplyResult | null>(null);
    const [urlMode, setUrlMode] = useState(false);
    const [url, setUrl] = useState('');
    const [urlLoading, setUrlLoading] = useState(false);
    const [customName, setCustomName] = useState('');

    const loadPreset = async (p: vm.CharacterPreset) => {
        setPreset(p);
        setCustomName(p.character.name || '');
        const w = await ValidateCharacterPreset(p);
        setWarnings(w || []);
    };

    const handleSelectFile = async () => {
        try {
            const p = await LoadCharacterPresetFromFile();
            if (p) await loadPreset(p);
        } catch (e) {
            toast.error('Error: ' + e);
        }
    };

    const handleLoadFromURL = async () => {
        if (!url.trim()) return;
        setUrlLoading(true);
        try {
            const p = await LoadCharacterPresetFromURL(url.trim());
            if (p) await loadPreset(p);
        } catch (e) {
            toast.error('URL load failed: ' + e);
        } finally {
            setUrlLoading(false);
        }
    };

    const handleApply = async () => {
        if (!preset) return;
        setLoading(true);
        try {
            if (customName && !options.keepName) {
                preset.character.name = customName;
            }
            const opts = new vm.ApplyOptions({
                replaceStats: options.replaceStats,
                replaceInventory: options.replaceInventory,
                replaceStorage: options.replaceStorage,
                replaceWorld: options.replaceWorld,
                keepName: options.keepName,
                keepClass: options.keepClass,
            });
            const res = await ApplyCharacterPreset(charIndex, preset, opts);
            setResult(res);
            onMutate?.();
            if (preset.addSettings && onAddSettingsApplied) {
                onAddSettingsApplied({
                    upgrade25: preset.addSettings.upgrade25 ?? 0,
                    upgrade10: preset.addSettings.upgrade10 ?? 0,
                    infuseOffset: preset.addSettings.infuseOffset ?? 0,
                    upgradeAsh: preset.addSettings.upgradeAsh ?? 0,
                    talismansHighestOnly: preset.addSettings.talismansHighestOnly ?? false,
                });
            }
            toast.success('Preset applied successfully!');
        } catch (e) {
            toast.error('Apply failed: ' + e);
        } finally {
            setLoading(false);
        }
    };

    const world = preset?.world;
    const hasWorld = world && (
        (world.graces?.length || 0) + (world.bosses?.length || 0) +
        (world.summoningPools?.length || 0) + (world.colosseums?.length || 0) +
        (world.mapFlags?.length || 0) + (world.cookbooks?.length || 0) +
        (world.bellBearings?.length || 0) + (world.whetblades?.length || 0) +
        (world.gestures?.length || 0) + (world.regions?.length || 0) +
        (world.worldPickups?.length || 0)
    ) > 0;

    if (result) {
        return (
            <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
                <div className="card p-10 text-center max-w-2xl mx-auto relative overflow-hidden">
                    <div className="relative space-y-6">
                        <div className="w-14 h-14 bg-green-500/10 border border-green-500/20 rounded-2xl flex items-center justify-center mx-auto">
                            <svg className="w-7 h-7 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                            </svg>
                        </div>
                        <div className="space-y-2">
                            <h3 className="text-xl font-black tracking-tight">Preset Applied</h3>
                            <div className="text-sm text-muted-foreground space-y-1">
                                {result.statsApplied && <p>Stats updated</p>}
                                {result.worldApplied && <p>World data applied</p>}
                                {result.itemsAdded > 0 && <p>{result.itemsAdded} items added</p>}
                                {result.itemsRemoved > 0 && <p>{result.itemsRemoved} items removed</p>}
                                {result.itemsSkipped > 0 && <p>{result.itemsSkipped} items skipped</p>}
                            </div>
                        </div>
                        {result.warnings && result.warnings.length > 0 && (
                            <div className="text-left bg-yellow-500/5 border border-yellow-500/20 rounded-md p-3 max-h-32 overflow-y-auto">
                                <p className="text-[9px] font-black uppercase tracking-widest text-yellow-600 mb-1">Warnings</p>
                                {result.warnings.map((w, i) => (
                                    <p key={i} className="text-[10px] text-yellow-600/80">{w}</p>
                                ))}
                            </div>
                        )}
                        <button
                            onClick={onComplete}
                            className="bg-foreground text-background hover:scale-[1.02] active:scale-[0.98] transition-all font-black px-10 py-3 rounded-md text-[11px] shadow-xl uppercase tracking-[0.2em]"
                        >
                            Done
                        </button>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
            <div className="card p-10 text-center max-w-2xl mx-auto relative overflow-hidden group">
                <div className="absolute top-0 right-0 w-32 h-32 bg-primary/5 rounded-full -mr-16 -mt-16 group-hover:bg-primary/10 transition-all duration-1000" />

                <div className="relative space-y-6">
                    <div className="w-14 h-14 bg-muted/50 border border-border rounded-2xl flex items-center justify-center mx-auto shadow-inner">
                        <svg className="w-7 h-7 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                        </svg>
                    </div>

                    <div className="space-y-2">
                        <h3 className="text-xl font-black tracking-tight">Import Character Preset</h3>
                        <p className="text-sm text-muted-foreground leading-relaxed max-w-sm mx-auto font-medium">
                            Load a .preset.json file and apply stats, inventory, storage and world data to the current character.
                        </p>
                    </div>

                    {!preset ? (
                        <div className="pt-4 space-y-4">
                            {!urlMode ? (
                                <>
                                    <button
                                        onClick={handleSelectFile}
                                        className="bg-foreground text-background hover:scale-[1.02] active:scale-[0.98] transition-all font-black px-10 py-3 rounded-md text-[11px] shadow-xl uppercase tracking-[0.2em]"
                                    >
                                        Select Preset File
                                    </button>
                                    <div>
                                        <button
                                            onClick={() => setUrlMode(true)}
                                            className="text-muted-foreground hover:text-foreground text-[10px] font-black uppercase tracking-[0.15em] transition-colors"
                                        >
                                            or load from URL
                                        </button>
                                    </div>
                                </>
                            ) : (
                                <div className="space-y-3 max-w-md mx-auto animate-in slide-in-from-bottom-2 duration-200">
                                    <input
                                        type="url"
                                        value={url}
                                        onChange={(e) => setUrl(e.target.value)}
                                        onKeyDown={(e) => e.key === 'Enter' && handleLoadFromURL()}
                                        placeholder="https://example.com/build.preset.json"
                                        className="w-full bg-muted/30 border border-border rounded-md px-3 py-2 text-xs font-mono placeholder:text-muted-foreground/40 focus:outline-none focus:ring-1 focus:ring-primary/50"
                                        autoFocus
                                    />
                                    <div className="flex items-center justify-center gap-4">
                                        <button
                                            onClick={() => { setUrlMode(false); setUrl(''); }}
                                            className="text-muted-foreground hover:text-foreground text-[10px] font-black uppercase tracking-[0.15em] transition-colors"
                                        >
                                            Back
                                        </button>
                                        <button
                                            onClick={handleLoadFromURL}
                                            disabled={urlLoading || !url.trim()}
                                            className={`bg-foreground text-background hover:scale-[1.02] active:scale-[0.98] transition-all font-black px-8 py-2.5 rounded-md text-[11px] shadow-xl uppercase tracking-[0.2em] ${urlLoading || !url.trim() ? 'opacity-50 cursor-not-allowed' : ''}`}
                                        >
                                            {urlLoading ? 'Loading...' : 'Load'}
                                        </button>
                                    </div>
                                </div>
                            )}
                        </div>
                    ) : (
                        <div className="space-y-6 pt-4 animate-in slide-in-from-bottom-2 duration-300">
                            {/* Preview card */}
                            <div className="text-left bg-muted/30 border border-border rounded-md p-4 space-y-3">
                                <div className="flex items-center justify-between">
                                    <div className="flex-1">
                                        <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground mb-1">Character Name</p>
                                        <input
                                            type="text"
                                            value={customName}
                                            onChange={(e) => setCustomName(e.target.value.slice(0, 16))}
                                            disabled={options.keepName}
                                            maxLength={16}
                                            className="w-full bg-background/50 border border-border rounded px-2 py-1 text-sm font-black focus:outline-none focus:ring-1 focus:ring-primary/50 disabled:opacity-40 disabled:cursor-not-allowed"
                                        />
                                    </div>
                                    <div className="text-right ml-4">
                                        <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Class</p>
                                        <p className="text-sm font-bold text-muted-foreground">{preset.character.className}</p>
                                    </div>
                                </div>
                                <div className="grid grid-cols-4 gap-2">
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Level</p>
                                        <p className="text-xs font-bold">{preset.character.level}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">VIG</p>
                                        <p className="text-xs font-bold">{preset.character.vigor}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">STR</p>
                                        <p className="text-xs font-bold">{preset.character.strength}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">DEX</p>
                                        <p className="text-xs font-bold">{preset.character.dexterity}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">MND</p>
                                        <p className="text-xs font-bold">{preset.character.mind}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">END</p>
                                        <p className="text-xs font-bold">{preset.character.endurance}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">INT</p>
                                        <p className="text-xs font-bold">{preset.character.intelligence}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">FTH</p>
                                        <p className="text-xs font-bold">{preset.character.faith}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">ARC</p>
                                        <p className="text-xs font-bold">{preset.character.arcane}</p>
                                    </div>
                                    <div>
                                        <p className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Souls</p>
                                        <p className="text-xs font-bold">{preset.character.souls?.toLocaleString()}</p>
                                    </div>
                                </div>
                                <div className="flex gap-4 text-[10px] text-muted-foreground pt-1 border-t border-border/50">
                                    <span>{preset.inventory?.length || 0} inventory items</span>
                                    <span>{preset.storage?.length || 0} storage items</span>
                                    {preset.character.memoryStones > 0 && <span>{preset.character.memoryStones} memory stones</span>}
                                </div>
                                {preset.addSettings && (
                                    <div className="flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground pt-1 border-t border-border/50">
                                        <span className="text-[9px] font-black uppercase tracking-widest w-full">Add Settings</span>
                                        {preset.addSettings.upgrade25 > 0 && <span>+{preset.addSettings.upgrade25} (25)</span>}
                                        {preset.addSettings.upgrade10 > 0 && <span>+{preset.addSettings.upgrade10} (10)</span>}
                                        {preset.addSettings.upgradeAsh > 0 && <span>ash +{preset.addSettings.upgradeAsh}</span>}
                                        {preset.addSettings.infuseOffset > 0 && <span>infuse {preset.addSettings.infuseOffset}</span>}
                                        {preset.addSettings.talismansHighestOnly && <span>talismans: highest only</span>}
                                    </div>
                                )}
                                {hasWorld && (
                                    <div className="flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground pt-1 border-t border-border/50">
                                        {(world!.graces?.length || 0) > 0 && <span>{world!.graces!.length} graces</span>}
                                        {(world!.bosses?.length || 0) > 0 && <span>{world!.bosses!.length} bosses</span>}
                                        {(world!.summoningPools?.length || 0) > 0 && <span>{world!.summoningPools!.length} pools</span>}
                                        {(world!.mapFlags?.length || 0) > 0 && <span>{world!.mapFlags!.length} map flags</span>}
                                        {(world!.cookbooks?.length || 0) > 0 && <span>{world!.cookbooks!.length} cookbooks</span>}
                                        {(world!.bellBearings?.length || 0) > 0 && <span>{world!.bellBearings!.length} bell bearings</span>}
                                        {(world!.whetblades?.length || 0) > 0 && <span>{world!.whetblades!.length} whetblades</span>}
                                        {(world!.gestures?.length || 0) > 0 && <span>{world!.gestures!.length} gestures</span>}
                                        {(world!.regions?.length || 0) > 0 && <span>{world!.regions!.length} regions</span>}
                                        {(world!.worldPickups?.length || 0) > 0 && <span>{world!.worldPickups!.length} pickups</span>}
                                    </div>
                                )}
                            </div>

                            {/* Warnings */}
                            {warnings.length > 0 && (
                                <div className="text-left bg-yellow-500/5 border border-yellow-500/20 rounded-md p-3 max-h-32 overflow-y-auto">
                                    <p className="text-[9px] font-black uppercase tracking-widest text-yellow-600 mb-1">Warnings ({warnings.length})</p>
                                    {warnings.map((w, i) => (
                                        <p key={i} className="text-[10px] text-yellow-600/80">{w}</p>
                                    ))}
                                </div>
                            )}

                            {/* Options */}
                            <div className="text-left space-y-2">
                                <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Apply Options</p>
                                <div className="grid grid-cols-2 gap-2">
                                    {([
                                        ['replaceStats', 'Replace Stats'],
                                        ['replaceInventory', 'Replace Inventory'],
                                        ['replaceStorage', 'Replace Storage'],
                                        ...(hasWorld ? [['replaceWorld', 'Replace World Data'] as const] : []),
                                        ['keepName', 'Keep Current Name'],
                                        ['keepClass', 'Keep Current Class'],
                                    ] as const).map(([key, label]) => (
                                        <label key={key} className="flex items-center gap-2 text-[10px] font-medium cursor-pointer">
                                            <input
                                                type="checkbox"
                                                checked={options[key as keyof typeof options]}
                                                onChange={(e) => setOptions(prev => ({...prev, [key]: e.target.checked}))}
                                                className="rounded border-border"
                                            />
                                            {label}
                                        </label>
                                    ))}
                                </div>
                            </div>

                            {/* Actions */}
                            <div className="flex items-center justify-center space-x-6 pt-2">
                                <button
                                    onClick={() => { setPreset(null); setWarnings([]); setResult(null); }}
                                    className="text-muted-foreground hover:text-foreground text-[10px] font-black uppercase tracking-[0.2em] transition-colors px-4 py-2"
                                >
                                    Cancel
                                </button>
                                <RiskActionButton
                                    riskKey="preset_apply"
                                    disabled={loading}
                                    onConfirm={handleApply}
                                    className={`
                                        bg-foreground text-background hover:scale-[1.02] active:scale-[0.98] transition-all font-black px-8 py-3 rounded-md text-[11px] shadow-xl uppercase tracking-[0.2em]
                                        ${loading ? 'opacity-50 cursor-not-allowed' : ''}
                                    `}
                                >
                                    {loading ? 'Applying...' : 'Apply Preset'}
                                </RiskActionButton>
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
