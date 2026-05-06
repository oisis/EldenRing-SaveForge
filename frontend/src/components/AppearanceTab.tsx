import {useState, useEffect} from 'react';
import toast from '../lib/toast';
import {ListAppearancePresets, ApplyMirrorFavoriteToCharacter, WriteSelectedToFavorites, GetFavoritesStatus, RemoveFavoritePreset} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {WarningModal} from './WarningModal';

interface Props {
    charIndex: number;
    onMutate: () => void;
    readOnly?: boolean;
}

export function AppearanceTab({charIndex, onMutate, readOnly = false}: Props) {
    const [presets, setPresets] = useState<main.PresetInfo[]>([]);
    const [checked, setChecked] = useState<Set<string>>(new Set());
    const [writingFav, setWritingFav] = useState(false);
    const [favSlots, setFavSlots] = useState<main.FavoriteSlotInfo[]>([]);
    const [zoomed, setZoomed] = useState<string | null>(null);
    const [typeBWarning, setTypeBWarning] = useState<string[]>([]);

    useEffect(() => {
        ListAppearancePresets().then(setPresets).catch(e => toast.error("" + e));
        if (!readOnly) refreshFavStatus();
    }, [readOnly]);

    const refreshFavStatus = () => {
        GetFavoritesStatus().then(setFavSlots).catch(() => {});
    };

    const freeSlots = favSlots.filter(s => s.safe && !s.active).length;
    const usedSafeSlots = favSlots.filter(s => s.safe && s.active);

    const toggleCheck = (name: string) => {
        setChecked(prev => {
            const next = new Set(prev);
            if (next.has(name)) {
                next.delete(name);
            } else {
                next.add(name);
            }
            return next;
        });
    };

    const handleWriteFavorites = async () => {
        if (checked.size === 0) return;
        if (checked.size > freeSlots) {
            toast.error(`Too many selected: ${checked.size} > ${freeSlots} free slots`);
            return;
        }
        // Type B presets currently corrupt Mirror slots (Model IDs left at zero by
        // WriteSelectedToFavorites — see spec/31). Block until presets.go is re-sourced
        // as raw 0x130-byte blobs. Apply to Character still works for in-game presets.
        const typeB = Array.from(checked).filter(n =>
            presets.find(p => p.name === n)?.bodyType === 'Type B');
        if (typeB.length > 0) {
            toast.error(`Type B (female) presets cannot be written to Mirror — would create bald, male-faced slot. Create the preset in-game instead.`);
            setTypeBWarning(typeB);
            return;
        }
        setWritingFav(true);
        try {
            const names = Array.from(checked);
            const count = await WriteSelectedToFavorites(charIndex, names);
            toast.success(`Wrote ${count} presets to Mirror Favorites`);
            setChecked(new Set());
            refreshFavStatus();
        } catch (e) {
            toast.error("" + e);
        } finally {
            setWritingFav(false);
        }
    };

    const handleRemoveFav = async (slotIndex: number) => {
        try {
            await RemoveFavoritePreset(slotIndex);
            toast.success(`Cleared Favorites slot ${slotIndex + 1}`);
            refreshFavStatus();
        } catch (e) {
            toast.error("" + e);
        }
    };

    const handleApplyFromMirror = async (slotIndex: number) => {
        try {
            await ApplyMirrorFavoriteToCharacter(charIndex, slotIndex);
            toast.success(`Applied Mirror slot ${slotIndex + 1} to character`);
            onMutate();
        } catch (e) {
            toast.error("" + e);
        }
    };

    return (
        <div className="space-y-6 p-4">
            <div className="flex items-center space-x-3">
                <div className="w-1 h-5 bg-primary rounded-full" />
                <h3 className="text-sm font-black uppercase tracking-[0.15em]">Appearance Presets</h3>
                <span className="text-[9px] text-muted-foreground font-medium uppercase tracking-wider">
                    {presets.length} presets
                </span>
            </div>

            <div className="card p-4 space-y-2">
                <p className="text-[10px] text-muted-foreground leading-relaxed">
                    {readOnly ? (
                        <><strong>Click image</strong> to preview. Load a save file to add presets to Mirror Favorites.</>
                    ) : (
                        <><strong>Click image</strong> to preview. <strong>Checkbox</strong> to select.
                            <strong> Add to Mirror</strong> writes to in-game Favorites; then click ✓ on a slot to apply onto the current character.</>
                    )}
                </p>
                {!readOnly && (
                    <div className="flex items-center gap-4 text-[9px] font-bold uppercase tracking-wider">
                        <span className="text-primary">{freeSlots} free mirror slots</span>
                        {checked.size > 0 && (
                            <span className="text-amber-500">{checked.size} selected</span>
                        )}
                    </div>
                )}
            </div>

            {/* Preset grid */}
            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
                {presets.map(p => {
                    const isChecked = checked.has(p.name);
                    return (
                        <div key={p.name} className={`
                            group relative rounded-lg border overflow-hidden transition-all
                            ${isChecked
                                ? 'border-primary ring-1 ring-primary shadow-lg shadow-primary/10'
                                : 'border-border hover:border-primary/30'}
                        `}>
                            {/* Checkbox — hidden in read-only mode */}
                            {!readOnly && (
                                <div className="absolute top-2 left-2 z-10 cursor-pointer" onClick={() => toggleCheck(p.name)}>
                                    <div className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-all
                                        ${isChecked
                                            ? 'bg-primary border-primary'
                                            : 'border-white/50 bg-black/40 hover:border-white/80'}`}>
                                        {isChecked && (
                                            <svg className="w-3 h-3 text-primary-foreground" fill="currentColor" viewBox="0 0 20 20">
                                                <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                                            </svg>
                                        )}
                                    </div>
                                </div>
                            )}

                            {/* Image — click to zoom */}
                            <div className="relative aspect-[3/4] bg-muted/30 overflow-hidden cursor-pointer"
                                 onClick={() => setZoomed(p.image ? `presets/${p.image}` : null)}>
                                {p.image ? (
                                    <img
                                        src={`presets/${p.image}`}
                                        alt={p.name}
                                        className="w-full h-full object-cover object-top transition-all duration-500 group-hover:scale-105"
                                    />
                                ) : (
                                    <div className="w-full h-full flex items-center justify-center">
                                        <svg className="w-10 h-10 text-muted-foreground/30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z" />
                                        </svg>
                                    </div>
                                )}
                                <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent" />
                                {/* Zoom icon */}
                                <div className="absolute bottom-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity">
                                    <svg className="w-4 h-4 text-white/70" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7" />
                                    </svg>
                                </div>
                            </div>

                            {/* Name / body type */}
                            <div className={`p-2.5 text-center transition-colors ${isChecked ? 'bg-primary/5' : 'bg-background'}`}>
                                <div className={`text-[10px] font-black uppercase tracking-wider leading-tight ${
                                    isChecked ? 'text-primary' : 'text-foreground'
                                }`}>{p.name}</div>
                                <div className="text-[8px] text-muted-foreground font-medium uppercase tracking-widest mt-0.5">{p.bodyType}</div>
                            </div>
                        </div>
                    );
                })}
            </div>

            {/* Action buttons */}
            {!readOnly && checked.size > 0 && (
                <div className="flex items-center justify-center gap-4 pt-2 flex-wrap animate-in slide-in-from-bottom-2 duration-300">
                    {/* Add to Mirror */}
                    <button
                        disabled={writingFav || checked.size > freeSlots}
                        onClick={handleWriteFavorites}
                        className={`border border-primary/30 text-primary hover:bg-primary/10 hover:scale-[1.02] active:scale-[0.98]
                            transition-all font-black px-6 py-3 rounded-md text-[10px] uppercase tracking-[0.2em]
                            ${(writingFav || checked.size > freeSlots) ? 'opacity-50 cursor-not-allowed' : ''}`}
                    >
                        {writingFav ? 'Writing...' : `Add ${checked.size} to Mirror`}
                    </button>

                    {checked.size > freeSlots && (
                        <span className="text-[9px] text-red-400 font-bold uppercase tracking-wider">
                            Too many! Max {freeSlots}
                        </span>
                    )}
                </div>
            )}

            {/* Existing Favorites slots */}
            {!readOnly && usedSafeSlots.length > 0 && (
                <div className="space-y-2 pt-4 border-t border-border">
                    <div className="flex items-center space-x-3">
                        <div className="w-1 h-5 bg-amber-500 rounded-full" />
                        <h3 className="text-sm font-black uppercase tracking-[0.15em]">Mirror Favorites</h3>
                        <span className="text-[9px] text-muted-foreground font-medium uppercase tracking-wider">
                            {usedSafeSlots.length} used
                        </span>
                    </div>
                    <div className="flex flex-wrap gap-2">
                        {usedSafeSlots.map(s => (
                            <div key={s.index} className="flex items-center gap-2 bg-muted/30 rounded-md px-3 py-1.5">
                                <span className="text-[10px] font-bold uppercase tracking-wider">
                                    Slot {s.index + 1}
                                </span>
                                <button
                                    onClick={() => handleApplyFromMirror(s.index)}
                                    className="text-primary hover:text-primary/80 transition-colors"
                                    title="Apply this preset to character"
                                >
                                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                                    </svg>
                                </button>
                                <button
                                    onClick={() => handleRemoveFav(s.index)}
                                    className="text-red-400 hover:text-red-300 transition-colors"
                                    title="Remove from Favorites"
                                >
                                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                                    </svg>
                                </button>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Zoom modal */}
            {zoomed && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm cursor-pointer"
                     onClick={() => setZoomed(null)}>
                    <img
                        src={zoomed}
                        alt="Preview"
                        className="max-h-[85vh] max-w-[85vw] rounded-xl shadow-2xl object-contain animate-in zoom-in-90 duration-300"
                        onClick={e => e.stopPropagation()}
                    />
                    <button onClick={() => setZoomed(null)}
                        className="absolute top-6 right-6 text-white/70 hover:text-white transition-colors">
                        <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>
            )}
            {typeBWarning.length > 0 && (
                <WarningModal title="Cannot write to Mirror Favorites" onClose={() => setTypeBWarning([])}>
                    <p>
                        <strong>Type B (female)</strong> presets cannot be written to Mirror Favorites.
                        Writing them would leave Model IDs at zero — resulting in a bald, male-faced slot.
                    </p>
                    <p>Affected: <strong>{typeBWarning.join(', ')}</strong></p>
                    <p>Use <strong>Apply to Character</strong> instead, or create the preset directly in-game.</p>
                </WarningModal>
            )}
        </div>
    );
}
