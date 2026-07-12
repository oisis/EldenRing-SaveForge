import {useEffect, useRef, useState} from 'react';
import toast from '../lib/toast';
import {GetCharacter, SaveCharacter, ListAppearancePresets, ApplyMirrorFavoriteToCharacter, WriteSelectedToFavorites, GetFavoritesStatus, RemoveFavoritePreset, GetStartingClasses, SetCharacterGender, ApplyPresetToCharacter, GetFavoritesUndoDepth, RevertFavorites} from '../../wailsjs/go/main/App';
import {vm, main, db} from '../../wailsjs/go/models';
import {AccordionSection} from './AccordionSection';
import {RiskInfoIcon} from './RiskInfoIcon';
import {getRunesRiskKey} from '../data/riskInfo';
import {useSafetyMode} from '../state/safetyMode';
import type {AddSettings} from '../App';
import {WarningModal} from './WarningModal';

const RUNES_LEGAL_MAX = 999_999_999;

function runesCostForLevel(level: number): number {
    let total = 0;
    for (let n = 2; n <= level; n++) {
        const cost = Math.floor(0.02 * n * n * n + 3.06 * n * n + 105.6 * n - 895);
        if (cost > 0) total += cost;
    }
    return Math.min(total, 4_294_967_295);
}

interface Props {
    charIndex: number;
    onNameChange?: () => void;
    onMutate: () => void;
    refreshKey?: number;
    addSettings: AddSettings;
    onAddSettingsChange: (s: AddSettings) => void;
    infuseTypes: db.InfuseType[];
}

const ATTRIBUTES = [
    { id: 'vigor', label: 'Vigor', abbr: 'Vig' },
    { id: 'mind', label: 'Mind', abbr: 'Min' },
    { id: 'endurance', label: 'Endurance', abbr: 'End' },
    { id: 'strength', label: 'Strength', abbr: 'Str' },
    { id: 'dexterity', label: 'Dexterity', abbr: 'Dex' },
    { id: 'intelligence', label: 'Intelligence', abbr: 'Int' },
    { id: 'faith', label: 'Faith', abbr: 'Fai' },
    { id: 'arcane', label: 'Arcane', abbr: 'Arc' },
];

export function CharacterTab({charIndex, onNameChange, onMutate, refreshKey, addSettings, onAddSettingsChange, infuseTypes}: Props) {
    const safetyMode = useSafetyMode();
    const [char, setChar] = useState<vm.CharacterViewModel | null>(null);
    const [loading, setLoading] = useState(false);
    const [startingClasses, setStartingClasses] = useState<db.ClassStats[]>([]);
    const isDirty = useRef(false);
    const prevCharIndex = useRef(charIndex);

    // Appearance state
    const [presets, setPresets] = useState<main.PresetInfo[]>([]);
    const [addingPreset, setAddingPreset] = useState<string | null>(null);
    const [applyingPreset, setApplyingPreset] = useState<string | null>(null);
    const [favSlots, setFavSlots] = useState<main.FavoriteSlotInfo[]>([]);
    const [favUndoDepth, setFavUndoDepth] = useState(0);
    const [zoomed, setZoomed] = useState<string | null>(null);
    const [typeBWarning, setTypeBWarning] = useState<string | null>(null);
    const [presetSearch, setPresetSearch] = useState('');
    const [showMale, setShowMale] = useState(true);
    const [showFemale, setShowFemale] = useState(true);

    useEffect(() => {
        ListAppearancePresets().then(setPresets).catch(e => toast.error("" + e));
        GetStartingClasses().then(setStartingClasses).catch(e => toast.error("" + e));
        refreshFavStatus();
    }, []);

    useEffect(() => {
        const charChanged = prevCharIndex.current !== charIndex;
        prevCharIndex.current = charIndex;
        if (charChanged) {
            isDirty.current = false;
        } else if (isDirty.current) {
            return;
        }
        setLoading(true);
        GetCharacter(charIndex)
            .then(res => { setChar(res); setLoading(false); })
            .catch(() => setLoading(false));
    }, [charIndex, refreshKey]);

    const refreshFavStatus = () => {
        GetFavoritesStatus().then(setFavSlots).catch(() => {});
        GetFavoritesUndoDepth().then(setFavUndoDepth).catch(() => {});
    };

    const handleUndoMirrorChange = async () => {
        try {
            await RevertFavorites();
            toast.success('Undid last Mirror change');
            refreshFavStatus();
        } catch (e) { toast.error("" + e); }
    };

    const freeSlots = favSlots.filter(s => s.safe && !s.active).length;
    const usedSafeSlots = favSlots.filter(s => s.safe && s.active);


    const getStatMin = (statId: string): number => {
        return char?.classBaseStats?.[statId] || 1;
    };

    const updateStat = (key: string, val: number) => {
        if (!char) return;
        const min = getStatMin(key);
        const clampedVal = Math.min(99, Math.max(min, val));
        const updatedData = {...char, [key]: clampedVal} as any;
        const sum = updatedData.vigor + updatedData.mind + updatedData.endurance + updatedData.strength +
                    updatedData.dexterity + updatedData.intelligence + updatedData.faith + updatedData.arcane;
        updatedData.level = Math.max(1, sum - 79);
        isDirty.current = true;
        setChar(vm.CharacterViewModel.createFrom(updatedData));
    };

    const handleClassChange = (classId: number) => {
        if (!char) return;
        const nc = startingClasses.find(c => c.id === classId);
        if (!nc) return;
        const vigor        = Math.max(char.vigor,        nc.vigor);
        const mind         = Math.max(char.mind,         nc.mind);
        const endurance    = Math.max(char.endurance,    nc.endurance);
        const strength     = Math.max(char.strength,     nc.strength);
        const dexterity    = Math.max(char.dexterity,    nc.dexterity);
        const intelligence = Math.max(char.intelligence, nc.intelligence);
        const faith        = Math.max(char.faith,        nc.faith);
        const arcane       = Math.max(char.arcane,       nc.arcane);
        const level = Math.max(1, vigor + mind + endurance + strength + dexterity + intelligence + faith + arcane - 79);
        isDirty.current = true;
        setChar(vm.CharacterViewModel.createFrom({
            ...char,
            class: classId,
            className: nc.name,
            classBaseStats: { vigor: nc.vigor, mind: nc.mind, endurance: nc.endurance, strength: nc.strength, dexterity: nc.dexterity, intelligence: nc.intelligence, faith: nc.faith, arcane: nc.arcane },
            vigor, mind, endurance, strength, dexterity, intelligence, faith, arcane, level,
        }));
    };

    const handleGenderChange = async (targetGender: number) => {
        if (!char) return;
        try {
            await SetCharacterGender(charIndex, targetGender);
            const updated = await GetCharacter(charIndex);
            setChar(updated);
            const label = targetGender === 1 ? 'Type A (Male) — Geralt defaults applied' : 'Type B (Female) — Ciri defaults applied';
            toast.success(label);
        } catch (e) {
            toast.error('Gender change failed: ' + e);
        }
    };

    const handleSave = () => {
        if (!char) return;
        SaveCharacter(charIndex, char)
            .then(() => {
                isDirty.current = false;
                toast.success('Character data updated in memory');
                onNameChange?.();
                GetCharacter(charIndex).then(updated => { if (updated) setChar(updated); }).catch(() => {});
            })
            .catch(err => toast.error('Error: ' + err));
    };

    const handleFixSoulMemory = () => {
        if (!char) return;
        const minRequired = runesCostForLevel(char.level);
        const buffered = Math.min(Math.floor(minRequired * 1.1), 4_294_967_295);
        const updated = vm.CharacterViewModel.createFrom({...char, soulMemory: buffered});
        setChar(updated);
        SaveCharacter(charIndex, updated)
            .then(() => {
                isDirty.current = false;
                toast.success('Soul Memory corrected');
                onNameChange?.();
                GetCharacter(charIndex).then(res => { if (res) setChar(res); }).catch(() => {});
            })
            .catch(err => toast.error('Fix failed: ' + err));
    };

    // Appearance handlers
    const handleAddPreset = async (name: string, bodyType: string) => {
        if (freeSlots === 0 || addingPreset !== null) return;
        // Type B presets currently corrupt Mirror slots (Model IDs left at zero by
        // WriteSelectedToFavorites — see spec/31). Block until presets.go is re-sourced
        // as raw 0x130-byte blobs. Apply to Character still works for in-game presets.
        if (bodyType === 'Type B') {
            toast.error(`Type B (female) presets cannot be written to Mirror — would create bald, male-faced slot. Create the preset in-game instead.`);
            setTypeBWarning(name);
            return;
        }
        setAddingPreset(name);
        try {
            await WriteSelectedToFavorites(charIndex, [name]);
            toast.success(`Added "${name.split(',')[0].trim()}" to Mirror Favorites`);
            refreshFavStatus();
        } catch (e) { toast.error("" + e); }
        finally { setAddingPreset(null); }
    };

    const handleApplyPreset = async (name: string) => {
        if (applyingPreset !== null) return;
        setApplyingPreset(name);
        try {
            await ApplyPresetToCharacter(charIndex, name);
            const updated = await GetCharacter(charIndex);
            setChar(updated);
            toast.success(`Applied "${name.split(',')[0].trim()}" to character`);
            onMutate();
        } catch (e) { toast.error('Apply failed: ' + e); }
        finally { setApplyingPreset(null); }
    };

    const handleRemoveFav = async (slotIndex: number) => {
        try {
            await RemoveFavoritePreset(slotIndex);
            toast.success(`Cleared Favorites slot ${slotIndex + 1}`);
            refreshFavStatus();
        } catch (e) { toast.error("" + e); }
    };

    const handleApplyFromMirror = async (slotIndex: number) => {
        try {
            await ApplyMirrorFavoriteToCharacter(charIndex, slotIndex);
            toast.success(`Applied Mirror slot ${slotIndex + 1} to character`);
            onMutate();
        } catch (e) { toast.error("" + e); }
    };

    // Summaries for collapsed sections
    const profileSummary = char
        ? <span className="flex items-center gap-2 text-center">
            <span className="text-xs font-black text-primary">{char.name}</span>
            <span className="text-[11px] text-muted-foreground font-medium">RL {char.level} | NG+{char.clearCount || 0} | {(char.souls || 0).toLocaleString()} Runes</span>
          </span>
        : undefined;

    const attrSummary = char
        ? ATTRIBUTES.map(a => `${a.abbr} ${(char as any)[a.id]}`).join(' | ')
        : '';

    if (loading) return (
        <div className="py-10 flex flex-col items-center justify-center space-y-3">
            <div className="w-5 h-5 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
            <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Loading...</p>
        </div>
    );

    if (!char) return (
        <div className="py-10 text-center border border-dashed border-border rounded-lg">
            <p className="text-xs text-muted-foreground">No character data.</p>
        </div>
    );

    return (
        <div className="space-y-3 animate-in fade-in duration-500 max-w-5xl mx-auto">
            {/* ═══ PROFILE ═══ */}
            <AccordionSection
                id="char-profile"
                title="Profile"
                summary={profileSummary}
                headerRight={
                    <div className="flex items-center gap-1.5">
                        <span className="text-[11px] font-black text-muted-foreground uppercase tracking-[0.2em]">RL</span>
                        <span className="text-lg font-black tracking-tighter text-primary leading-none">{char.level}</span>
                    </div>
                }
            >
                <div className="space-y-4">
                    <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
                        <div className="space-y-1.5">
                            <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1">Character Name</label>
                            <input type="text" value={char.name} maxLength={16}
                                onChange={e => { isDirty.current = true; setChar(vm.CharacterViewModel.createFrom({...char, name: e.target.value})); }}
                                className="w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs focus:ring-1 focus:ring-primary/30 outline-none transition-all" />
                        </div>
                        <div className="space-y-1.5">
                            <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1">Starting Class</label>
                            <select value={char.class ?? 0}
                                onChange={e => handleClassChange(parseInt(e.target.value))}
                                className="w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-black text-primary focus:ring-1 focus:ring-primary/30 outline-none transition-all cursor-pointer h-[34px]">
                                {startingClasses.map(c => (
                                    <option key={c.id} value={c.id}>{c.name}</option>
                                ))}
                            </select>
                        </div>
                        <div className="space-y-1.5">
                            <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1">Body Type</label>
                            <select
                                value={char.gender ?? 1}
                                onChange={e => handleGenderChange(parseInt(e.target.value))}
                                className="w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-black text-primary focus:ring-1 focus:ring-primary/30 outline-none transition-all cursor-pointer h-[34px]">
                                <option value={1}>Type A (Male)</option>
                                <option value={0}>Type B (Female)</option>
                            </select>
                        </div>
                        <div className="space-y-1.5">
                            <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1">
                                Talisman Slots <span className="text-primary font-mono">{1 + (char.talismanSlots || 0)}/4</span>
                            </label>
                            <input type="number" min={0} max={3} value={char.talismanSlots || 0}
                                onChange={e => {
                                    const v = Math.min(3, Math.max(0, parseInt(e.target.value) || 0));
                                    isDirty.current = true;
                                    setChar(vm.CharacterViewModel.createFrom({...char, talismanSlots: v}));
                                }}
                                className="w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-mono focus:ring-1 focus:ring-primary/30 outline-none transition-all" />
                        </div>
                        <div className="space-y-1.5">
                            <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1">
                                Memory Stones <span className="text-primary font-mono">{char.memoryStones || 0}/8</span>
                            </label>
                            <input type="number" min={0} max={8} value={char.memoryStones || 0}
                                onChange={e => {
                                    const v = Math.min(8, Math.max(0, parseInt(e.target.value) || 0));
                                    isDirty.current = true;
                                    setChar(vm.CharacterViewModel.createFrom({...char, memoryStones: v}));
                                }}
                                className="w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-mono focus:ring-1 focus:ring-primary/30 outline-none transition-all" />
                        </div>
                    </div>

                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        <div className="space-y-1.5">
                            <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1">
                                NG+ Cycle <span className="text-primary font-mono">{char.clearCount || 0}/7</span>
                            </label>
                            <input type="number" min={0} max={7} value={char.clearCount || 0}
                                onChange={e => {
                                    const v = Math.min(7, Math.max(0, parseInt(e.target.value) || 0));
                                    isDirty.current = true;
                                    setChar(vm.CharacterViewModel.createFrom({...char, clearCount: v}));
                                }}
                                className="w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-mono focus:ring-1 focus:ring-primary/30 outline-none transition-all" />
                        </div>
                        <div className="space-y-1.5">
                            <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1 flex items-center gap-1.5">
                                <span>Runes</span>
                                {getRunesRiskKey(char.souls) && <RiskInfoIcon riskKey={getRunesRiskKey(char.souls)!} />}
                            </label>
                            <input type="number" value={char.souls}
                                onChange={e => {
                                    let v = parseInt(e.target.value) || 0;
                                    if (safetyMode.enabled && v > RUNES_LEGAL_MAX) {
                                        v = RUNES_LEGAL_MAX;
                                        toast.error(`Online Safety Mode: clamped to legal max ${RUNES_LEGAL_MAX.toLocaleString()}`);
                                    }
                                    isDirty.current = true;
                                    setChar(vm.CharacterViewModel.createFrom({...char, souls: v}));
                                }}
                                title={safetyMode.enabled ? `Online Safety Mode caps Runes at ${RUNES_LEGAL_MAX.toLocaleString()}` : undefined}
                                className={
                                    getRunesRiskKey(char.souls)
                                        ? 'w-full bg-red-500/10 border-2 border-red-500 rounded-md px-3 py-2 text-xs font-mono text-red-300 focus:ring-2 focus:ring-red-500/40 outline-none transition-all'
                                        : 'w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-mono focus:ring-1 focus:ring-primary/30 outline-none transition-all'
                                } />
                        </div>
                        {(() => {
                            const minSM = runesCostForLevel(char.level);
                            const consistent = (char.soulMemory || 0) >= minSM;
                            return (
                                <div className="space-y-1.5">
                                    <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1 flex items-center gap-1.5">
                                        <span>Soul Memory</span>
                                        <span className={consistent ? 'text-green-400 font-black' : 'text-red-400 font-black'}>
                                            {consistent ? '✓' : '✗'}
                                        </span>
                                    </label>
                                    <div className="flex items-center gap-1.5">
                                        <span className="flex-1 bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-mono text-muted-foreground truncate">
                                            {(char.soulMemory || 0).toLocaleString()}
                                        </span>
                                        {!consistent && (
                                            <button onClick={handleFixSoulMemory}
                                                title={`Set to ${Math.min(Math.floor(minSM * 1.1), 4_294_967_295).toLocaleString()} (+10% buffer)`}
                                                className="px-2 py-2 text-[10px] font-black uppercase tracking-widest bg-red-500/20 border border-red-500/50 text-red-300 rounded-md hover:bg-red-500/30 transition-colors whitespace-nowrap">
                                                Fix
                                            </button>
                                        )}
                                    </div>
                                </div>
                            );
                        })()}
                    </div>
                </div>
            </AccordionSection>

            {/* ═══ ATTRIBUTES ═══ */}
            <AccordionSection
                id="char-attributes"
                title="Attributes"
                summary={attrSummary}
            >
                <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6">
                    {ATTRIBUTES.map(stat => {
                        const statMin = getStatMin(stat.id);
                        const redZonePct = ((statMin - 1) / 98) * 100;
                        return (
                            <div key={stat.id} className="flex items-center gap-3 py-1.5 border-b border-border/30">
                                <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider w-20 flex-shrink-0"
                                    title={`Base: ${statMin}`}>
                                    {stat.label}
                                </span>
                                <input
                                    type="range" min={1} max={99}
                                    value={(char as any)[stat.id]}
                                    onChange={e => updateStat(stat.id, parseInt(e.target.value))}
                                    className="flex-1 h-1.5 rounded-lg appearance-none cursor-pointer"
                                    style={{
                                        background: `linear-gradient(to right, rgb(239 68 68 / 0.4) 0%, rgb(239 68 68 / 0.4) ${redZonePct}%, hsl(var(--border)) ${redZonePct}%, hsl(var(--border)) 100%)`,
                                    }}
                                />
                                <input
                                    type="number" min={statMin} max={99}
                                    value={(char as any)[stat.id]}
                                    onChange={e => updateStat(stat.id, parseInt(e.target.value) || statMin)}
                                    className="w-12 bg-muted/30 border border-border rounded text-center text-xs py-1 focus:ring-1 focus:ring-primary/30 outline-none"
                                />
                            </div>
                        );
                    })}
                </div>
            </AccordionSection>

            {/* ═══ ADD SETTINGS ═══ */}
            <AccordionSection
                id="char-add-settings"
                title="Add Settings"
                summary={`+${addSettings.upgrade25} · +${addSettings.upgrade10} · ${infuseTypes.find(t => t.offset === addSettings.infuseOffset)?.name ?? 'Standard'} · Ash +${addSettings.upgradeAsh}`}
            >
                {(() => {
                    const set = (patch: Partial<AddSettings>) => onAddSettingsChange({...addSettings, ...patch});
                    return (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-10 gap-y-5 py-2">
                            <div className="flex items-center space-x-3">
                                <span className="text-[11px] font-normal uppercase tracking-widest text-foreground w-24 shrink-0">Weapon +25</span>
                                <input type="range" min={0} max={25} value={addSettings.upgrade25} onChange={e => set({upgrade25: parseInt(e.target.value)})}
                                    className="flex-1 h-1.5 rounded-lg appearance-none cursor-pointer"
                                    style={{background: 'hsl(var(--border))'}} />
                                <span className="text-[10px] font-mono font-bold text-primary w-6 text-right">+{addSettings.upgrade25}</span>
                            </div>
                            <div className="flex items-center space-x-3">
                                <span className="text-[11px] font-normal uppercase tracking-widest text-foreground w-24 shrink-0">Weapon +10</span>
                                <input type="range" min={0} max={10} value={addSettings.upgrade10} onChange={e => set({upgrade10: parseInt(e.target.value)})}
                                    className="flex-1 h-1.5 rounded-lg appearance-none cursor-pointer"
                                    style={{background: 'hsl(var(--border))'}} />
                                <span className="text-[10px] font-mono font-bold text-primary w-5 text-right">+{addSettings.upgrade10}</span>
                            </div>
                            <div className="flex items-center space-x-3">
                                <span className="text-[11px] font-normal uppercase tracking-widest text-foreground w-24 shrink-0">Infuse</span>
                                <select value={addSettings.infuseOffset} onChange={e => set({infuseOffset: parseInt(e.target.value)})}
                                    className="flex-1 bg-muted/20 border border-border rounded-md px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider focus:ring-1 focus:ring-primary/30 outline-none transition-all cursor-pointer">
                                    {infuseTypes.map(t => <option key={t.offset} value={t.offset}>{t.name}</option>)}
                                </select>
                            </div>
                            <div className="flex items-center space-x-3">
                                <span className="text-[11px] font-normal uppercase tracking-widest text-foreground w-24 shrink-0">Spirit Ash</span>
                                <input type="range" min={0} max={10} value={addSettings.upgradeAsh} onChange={e => set({upgradeAsh: parseInt(e.target.value)})}
                                    className="flex-1 h-1.5 rounded-lg appearance-none cursor-pointer"
                                    style={{background: 'hsl(var(--border))'}} />
                                <span className="text-[10px] font-mono font-bold text-primary w-5 text-right">+{addSettings.upgradeAsh}</span>
                            </div>
                            <div className="flex items-center gap-8 md:col-span-2 pt-1 border-t border-border/30">
                                <label title="When enabled, only the highest-tier variant of each talisman family is shown — lower upgrade levels are hidden." className="flex items-center gap-2 cursor-pointer">
                                    <input type="checkbox" checked={addSettings.talismansHighestOnly} onChange={e => set({talismansHighestOnly: e.target.checked})}
                                        className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20" />
                                    <span className="text-[11px] font-normal uppercase tracking-widest text-foreground">Talismans: highest only</span>
                                </label>
                                <label title="When enabled, 'Unlock All' Sites of Grace in World tab will also include Leyndell, Ashen Capital graces. Disable if you haven't triggered the capital's transformation yet." className="flex items-center gap-2 cursor-pointer">
                                    <input type="checkbox" checked={addSettings.includeAshenCapital} onChange={e => set({includeAshenCapital: e.target.checked})}
                                        className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20" />
                                    <span className="text-[11px] font-normal uppercase tracking-widest text-foreground">SoG: Leyndell, Ashen Capital</span>
                                </label>
                            </div>
                        </div>
                    );
                })()}
            </AccordionSection>

            {/* ═══ APPEARANCE PRESETS ═══ */}
            <AccordionSection
                id="char-presets"
                title="Appearance Presets"
                badge={`${presets.length} presets`}
            >
                <div className="space-y-3">
                    <div className="flex items-center gap-3">
                        <input
                            type="text"
                            placeholder="Search…"
                            value={presetSearch}
                            onChange={e => setPresetSearch(e.target.value)}
                            className="flex-1 bg-muted/20 border border-border rounded-md px-3 py-1.5 text-xs focus:ring-1 focus:ring-primary/30 outline-none transition-all"
                        />
                        <label className="flex items-center gap-1.5 cursor-pointer select-none">
                            <input type="checkbox" checked={showMale} onChange={e => setShowMale(e.target.checked)} className="accent-primary" />
                            <span className="text-[11px] font-bold text-muted-foreground uppercase tracking-wider">Male</span>
                        </label>
                        <label className="flex items-center gap-1.5 cursor-pointer select-none">
                            <input type="checkbox" checked={showFemale} onChange={e => setShowFemale(e.target.checked)} className="accent-primary" />
                            <span className="text-[11px] font-bold text-muted-foreground uppercase tracking-wider">Female</span>
                        </label>
                    </div>

                    <p className="text-[10px] text-muted-foreground">
                        Click image to preview. Checkbox to select. Apply ✓ on a Mirror slot to copy preset onto current character.
                    </p>

                    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
                        {presets.filter(p => {
                            if (p.bodyType === 'Type A' && !showMale) return false;
                            if (p.bodyType === 'Type B' && !showFemale) return false;
                            if (presetSearch && !p.name.toLowerCase().includes(presetSearch.toLowerCase())) return false;
                            return true;
                        }).map(p => {
                            const isAdding = addingPreset === p.name;
                            const isApplying = applyingPreset === p.name;
                            const canAdd = freeSlots > 0 && addingPreset === null;
                            const canApply = applyingPreset === null;
                            return (
                                <div key={p.name} className="group relative rounded-lg border border-border hover:border-primary/30 overflow-hidden transition-all">
                                    <div className="relative aspect-[3/4] bg-muted/30 overflow-hidden cursor-pointer"
                                         onClick={() => setZoomed(p.image ? `presets/${p.image}` : null)}>
                                        {p.image ? (
                                            <img src={`presets/${p.image}`} alt={p.name}
                                                className="w-full h-full object-cover object-top transition-all duration-500 group-hover:scale-105" />
                                        ) : (
                                            <div className="w-full h-full flex items-center justify-center">
                                                <svg className="w-10 h-10 text-muted-foreground/30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z" />
                                                </svg>
                                            </div>
                                        )}
                                        <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent" />
                                    </div>
                                    <div className="p-2 bg-background">
                                        <div className="text-[10px] font-black uppercase tracking-wider leading-tight text-foreground truncate">{p.name}</div>
                                        <div className="flex items-center justify-between mt-1 gap-1">
                                            <span className="text-[9px] text-muted-foreground font-medium uppercase tracking-widest">{p.bodyType}</span>
                                            <div className="flex gap-1">
                                                <button
                                                    onClick={() => handleApplyPreset(p.name)}
                                                    disabled={!canApply || isApplying}
                                                    title="Apply appearance to current character"
                                                    className="px-2 py-0.5 border border-blue-700/50 text-blue-700 rounded text-[9px] font-black uppercase tracking-wider hover:bg-blue-700/10 transition-all disabled:opacity-40 disabled:cursor-not-allowed shrink-0">
                                                    {isApplying ? '…' : 'Apply'}
                                                </button>
                                                <button
                                                    onClick={() => handleAddPreset(p.name, p.bodyType)}
                                                    disabled={!canAdd || isAdding}
                                                    title={freeSlots === 0 ? 'No free Mirror slots' : 'Add to Mirror Favorites'}
                                                    className="px-2 py-0.5 border border-primary/40 text-primary rounded text-[9px] font-black uppercase tracking-wider hover:bg-primary/10 transition-all disabled:opacity-40 disabled:cursor-not-allowed shrink-0">
                                                    {isAdding ? '…' : 'Add'}
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            );
                        })}
                    </div>

                    {/* Mirror Favorites */}
                    {usedSafeSlots.length > 0 && (
                        <div className="pt-3 border-t border-border/50">
                            <p className="text-[11px] font-normal uppercase tracking-widest text-foreground mb-2">Mirror Favorites ({usedSafeSlots.length} used · {freeSlots} free)</p>
                            <div className="flex flex-wrap gap-2">
                                {usedSafeSlots.map(s => (
                                    <div key={s.index} className="flex items-center gap-2 bg-muted/30 rounded-md px-3 py-1.5">
                                        <div className="flex flex-col leading-tight min-w-[40px]">
                                            <span className="text-[10px] font-bold uppercase tracking-wider">{s.name ? s.name.split(',')[0].trim() : 'N/A'}</span>
                                            <span className="text-[9px] text-muted-foreground">Slot {s.index + 1}</span>
                                        </div>
                                        <button onClick={() => handleApplyFromMirror(s.index)}
                                            className="text-primary hover:text-primary/80 transition-colors" title="Apply this preset to character">
                                            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                                            </svg>
                                        </button>
                                        <button onClick={() => handleRemoveFav(s.index)}
                                            className="text-red-400 hover:text-red-300 transition-colors" title="Remove">
                                            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                                            </svg>
                                        </button>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}

                    {favUndoDepth > 0 && (
                        <div className="pt-3">
                            <button onClick={handleUndoMirrorChange}
                                className="text-[10px] font-bold uppercase tracking-widest text-amber-400 hover:text-amber-300 transition-colors border border-amber-400/40 rounded-md px-3 py-1.5"
                                title="Revert the last Mirror Favorites change (add or remove)">
                                Undo last Mirror change ({favUndoDepth})
                            </button>
                        </div>
                    )}
                </div>
            </AccordionSection>

            {/* ═══ APPLY CHANGES ═══ */}
            <div className="flex justify-end items-center space-x-4 pt-4 pb-2 border-t border-border/30">
                <p className="text-[11px] font-bold text-muted-foreground uppercase tracking-widest italic opacity-50">Staged in memory</p>
                <button onClick={handleSave}
                    className="bg-primary text-primary-foreground hover:brightness-110 active:scale-95 transition-all font-black px-6 py-2 rounded-md text-[10px] uppercase tracking-widest shadow-lg shadow-primary/20">
                    Apply Changes
                </button>
            </div>

            {/* Zoom modal */}
            {zoomed && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm cursor-pointer"
                     onClick={() => setZoomed(null)}>
                    <img src={zoomed} alt="Preview"
                        className="max-h-[85vh] max-w-[85vw] rounded-xl shadow-2xl object-contain animate-in zoom-in-90 duration-300"
                        onClick={e => e.stopPropagation()} />
                    <button onClick={() => setZoomed(null)}
                        className="absolute top-6 right-6 text-white/70 hover:text-white transition-colors">
                        <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>
            )}
            {typeBWarning && (
                <WarningModal title="Cannot write to Mirror Favorites" onClose={() => setTypeBWarning(null)}>
                    <p>
                        <strong>Type B (female)</strong> presets cannot be written to Mirror Favorites.
                        Writing them would leave Model IDs at zero — resulting in a bald, male-faced slot.
                    </p>
                    <p>Preset: <strong>{typeBWarning.split(',')[0].trim()}</strong></p>
                    <p>Use <strong>Apply to Character</strong> instead, or create the preset directly in-game.</p>
                </WarningModal>
            )}
        </div>
    );
}
