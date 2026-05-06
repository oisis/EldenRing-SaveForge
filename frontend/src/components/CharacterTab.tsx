import {useEffect, useState} from 'react';
import toast from '../lib/toast';
import {GetCharacter, SaveCharacter, ListAppearancePresets, ApplyMirrorFavoriteToCharacter, WriteSelectedToFavorites, GetFavoritesStatus, RemoveFavoritePreset, GetStartingClasses} from '../../wailsjs/go/main/App';
import {vm, main, db} from '../../wailsjs/go/models';
import {AccordionSection} from './AccordionSection';
import {RiskInfoIcon} from './RiskInfoIcon';
import {getRunesRiskKey} from '../data/riskInfo';
import {useSafetyMode} from '../state/safetyMode';
import type {AddSettings} from '../App';

const RUNES_LEGAL_MAX = 999_999_999;

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

    // Appearance state
    const [presets, setPresets] = useState<main.PresetInfo[]>([]);
    const [checked, setChecked] = useState<Set<string>>(new Set());
    const [writingFav, setWritingFav] = useState(false);
    const [favSlots, setFavSlots] = useState<main.FavoriteSlotInfo[]>([]);
    const [zoomed, setZoomed] = useState<string | null>(null);

    useEffect(() => {
        ListAppearancePresets().then(setPresets).catch(e => toast.error("" + e));
        GetStartingClasses().then(setStartingClasses).catch(e => toast.error("" + e));
        refreshFavStatus();
    }, []);

    useEffect(() => {
        setLoading(true);
        GetCharacter(charIndex)
            .then(res => { setChar(res); setLoading(false); })
            .catch(() => setLoading(false));
    }, [charIndex, refreshKey]);

    const refreshFavStatus = () => {
        GetFavoritesStatus().then(setFavSlots).catch(() => {});
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
        setChar(vm.CharacterViewModel.createFrom({
            ...char,
            class: classId,
            className: nc.name,
            classBaseStats: { vigor: nc.vigor, mind: nc.mind, endurance: nc.endurance, strength: nc.strength, dexterity: nc.dexterity, intelligence: nc.intelligence, faith: nc.faith, arcane: nc.arcane },
            vigor, mind, endurance, strength, dexterity, intelligence, faith, arcane, level,
        }));
    };

    const handleSave = () => {
        if (char) {
            SaveCharacter(charIndex, char)
                .then(() => { toast.success('Character data updated in memory'); onNameChange?.(); })
                .catch(err => toast.error('Error: ' + err));
        }
    };

    // Appearance handlers
    const toggleCheck = (name: string) => {
        setChecked(prev => {
            const next = new Set(prev);
            next.has(name) ? next.delete(name) : next.add(name);
            return next;
        });
    };

    const handleWriteFavorites = async () => {
        if (checked.size === 0 || checked.size > freeSlots) return;
        // Type B presets currently corrupt Mirror slots (Model IDs left at zero by
        // WriteSelectedToFavorites — see spec/31). Block until presets.go is re-sourced
        // as raw 0x130-byte blobs. Apply to Character still works for in-game presets.
        const typeB = Array.from(checked).filter(n =>
            presets.find(p => p.name === n)?.bodyType === 'Type B');
        if (typeB.length > 0) {
            toast.error(`Type B (female) presets cannot be written to Mirror — would create bald, male-faced slot. Create the preset in-game instead.`);
            return;
        }
        setWritingFav(true);
        try {
            const count = await WriteSelectedToFavorites(charIndex, Array.from(checked));
            toast.success(`Wrote ${count} presets to Mirror Favorites`);
            setChecked(new Set());
            refreshFavStatus();
        } catch (e) { toast.error("" + e); }
        finally { setWritingFav(false); }
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
                                onChange={e => setChar(vm.CharacterViewModel.createFrom({...char, name: e.target.value}))}
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
                                    setChar(vm.CharacterViewModel.createFrom({...char, souls: v}));
                                }}
                                title={safetyMode.enabled ? `Online Safety Mode caps Runes at ${RUNES_LEGAL_MAX.toLocaleString()}` : undefined}
                                className={
                                    getRunesRiskKey(char.souls)
                                        ? 'w-full bg-red-500/10 border-2 border-red-500 rounded-md px-3 py-2 text-xs font-mono text-red-300 focus:ring-2 focus:ring-red-500/40 outline-none transition-all'
                                        : 'w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-mono focus:ring-1 focus:ring-primary/30 outline-none transition-all'
                                } />
                        </div>
                        <div className="space-y-1.5">
                            <label className="text-[11px] font-bold text-muted-foreground uppercase tracking-tight ml-1">
                                Talisman Slots <span className="text-primary font-mono">{1 + (char.talismanSlots || 0)}/4</span>
                            </label>
                            <input type="number" min={0} max={3} value={char.talismanSlots || 0}
                                onChange={e => {
                                    const v = Math.min(3, Math.max(0, parseInt(e.target.value) || 0));
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
                                    setChar(vm.CharacterViewModel.createFrom({...char, clearCount: v}));
                                }}
                                className="w-full bg-muted/20 border border-border rounded-md px-3 py-2 text-xs font-mono focus:ring-1 focus:ring-primary/30 outline-none transition-all" />
                        </div>
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
                                <span className="text-[11px] font-black uppercase tracking-widest text-muted-foreground w-24 shrink-0">Weapon +25</span>
                                <input type="range" min={0} max={25} value={addSettings.upgrade25} onChange={e => set({upgrade25: parseInt(e.target.value)})}
                                    className="flex-1 h-1.5 rounded-lg appearance-none cursor-pointer [&::-webkit-slider-runnable-track]:bg-border [&::-webkit-slider-runnable-track]:rounded-lg" />
                                <span className="text-[10px] font-mono font-bold text-primary w-6 text-right">+{addSettings.upgrade25}</span>
                            </div>
                            <div className="flex items-center space-x-3">
                                <span className="text-[11px] font-black uppercase tracking-widest text-muted-foreground w-24 shrink-0">Weapon +10</span>
                                <input type="range" min={0} max={10} value={addSettings.upgrade10} onChange={e => set({upgrade10: parseInt(e.target.value)})}
                                    className="flex-1 h-1.5 rounded-lg appearance-none cursor-pointer [&::-webkit-slider-runnable-track]:bg-border [&::-webkit-slider-runnable-track]:rounded-lg" />
                                <span className="text-[10px] font-mono font-bold text-primary w-5 text-right">+{addSettings.upgrade10}</span>
                            </div>
                            <div className="flex items-center space-x-3">
                                <span className="text-[11px] font-black uppercase tracking-widest text-muted-foreground w-24 shrink-0">Infuse</span>
                                <select value={addSettings.infuseOffset} onChange={e => set({infuseOffset: parseInt(e.target.value)})}
                                    className="flex-1 bg-muted/20 border border-border rounded-md px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider focus:ring-1 focus:ring-primary/30 outline-none transition-all cursor-pointer">
                                    {infuseTypes.map(t => <option key={t.offset} value={t.offset}>{t.name}</option>)}
                                </select>
                            </div>
                            <div className="flex items-center space-x-3">
                                <span className="text-[11px] font-black uppercase tracking-widest text-muted-foreground w-24 shrink-0">Spirit Ash</span>
                                <input type="range" min={0} max={10} value={addSettings.upgradeAsh} onChange={e => set({upgradeAsh: parseInt(e.target.value)})}
                                    className="flex-1 h-1.5 rounded-lg appearance-none cursor-pointer [&::-webkit-slider-runnable-track]:bg-border [&::-webkit-slider-runnable-track]:rounded-lg" />
                                <span className="text-[10px] font-mono font-bold text-primary w-5 text-right">+{addSettings.upgradeAsh}</span>
                            </div>
                            <div className="flex items-center gap-8 md:col-span-2 pt-1 border-t border-border/30">
                                <label title="When enabled, only the highest-tier variant of each talisman family is shown — lower upgrade levels are hidden." className="flex items-center gap-2 cursor-pointer">
                                    <input type="checkbox" checked={addSettings.talismansHighestOnly} onChange={e => set({talismansHighestOnly: e.target.checked})}
                                        className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20" />
                                    <span className="text-[11px] font-black uppercase tracking-widest text-muted-foreground">Talismans: highest only</span>
                                </label>
                                <label title="When enabled, 'Unlock All' Sites of Grace in World tab will also include Leyndell, Ashen Capital graces. Disable if you haven't triggered the capital's transformation yet." className="flex items-center gap-2 cursor-pointer">
                                    <input type="checkbox" checked={addSettings.includeAshenCapital} onChange={e => set({includeAshenCapital: e.target.checked})}
                                        className="w-3.5 h-3.5 rounded border-border text-primary focus:ring-primary/20" />
                                    <span className="text-[11px] font-black uppercase tracking-widest text-muted-foreground">SoG: Leyndell, Ashen Capital</span>
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
                    <p className="text-[10px] text-muted-foreground">
                        Click image to preview. Checkbox to select. Apply ✓ on a Mirror slot to copy preset onto current character.
                    </p>

                    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
                        {presets.map(p => {
                            const isChecked = checked.has(p.name);
                            return (
                                <div key={p.name} className={`group relative rounded-lg border overflow-hidden transition-all
                                    ${isChecked ? 'border-primary ring-1 ring-primary shadow-lg shadow-primary/10' : 'border-border hover:border-primary/30'}`}>
                                    <div className="absolute top-2 left-2 z-10 cursor-pointer" onClick={() => toggleCheck(p.name)}>
                                        <div className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-all
                                            ${isChecked ? 'bg-primary border-primary' : 'border-white/50 bg-black/40 hover:border-white/80'}`}>
                                            {isChecked && (
                                                <svg className="w-3 h-3 text-primary-foreground" fill="currentColor" viewBox="0 0 20 20">
                                                    <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                                                </svg>
                                            )}
                                        </div>
                                    </div>
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
                                    <div className={`p-2.5 text-center transition-colors ${isChecked ? 'bg-primary/5' : 'bg-background'}`}>
                                        <div className={`text-[10px] font-black uppercase tracking-wider leading-tight ${isChecked ? 'text-primary' : 'text-foreground'}`}>{p.name}</div>
                                        <div className="text-[11px] text-muted-foreground font-medium uppercase tracking-widest mt-0.5">{p.bodyType}</div>
                                    </div>
                                </div>
                            );
                        })}
                    </div>

                    {/* Mirror Favorites */}
                    {usedSafeSlots.length > 0 && (
                        <div className="pt-3 border-t border-border/50">
                            <p className="text-[11px] font-black uppercase tracking-widest text-muted-foreground mb-2">Mirror Favorites ({usedSafeSlots.length} used)</p>
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

                    {/* Add to presets button */}
                    {checked.size > 0 && (
                        <div className="flex items-center justify-end gap-3 pt-3 border-t border-border/30">
                            <span className="text-[11px] font-bold text-amber-500 uppercase tracking-wider">{checked.size} selected</span>
                            <button onClick={handleWriteFavorites} disabled={writingFav || checked.size > freeSlots}
                                className="px-3 py-1.5 border border-primary/30 text-primary rounded text-[11px] font-black uppercase tracking-wider hover:bg-primary/10 transition-all disabled:opacity-50">
                                Add to presets ({freeSlots} free)
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
        </div>
    );
}
