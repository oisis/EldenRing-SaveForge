import { useState, useEffect, useMemo } from 'react';
import { GetCharacter, GetItemList, ApplyWeaponInfusion, ApplyWeaponAoW, ApplyWeaponAoWStrict, GetAoWAvailability, RevertSlot } from '../../wailsjs/go/main/App';
import { db } from '../../wailsjs/go/models';
import toast from '../lib/toast';

const WEAPON_CATEGORIES = new Set(['melee_armaments', 'ranged_and_catalysts', 'shields']);

// Mirrors backend data.WepTypeToCanMountBit — maps weapon wepType → bit position in AoWCompatBitmask.
const WEP_TYPE_TO_BIT: Record<number, number> = {
    1: 0, 3: 1, 5: 2, 7: 3, 9: 8, 11: 9, 13: 6, 14: 5, 15: 4, 16: 7, 17: 7,
    19: 11, 21: 13, 23: 10, 24: 10, 25: 12, 28: 14, 29: 14, 31: 15, 32: 17,
    33: 18, 35: 16, 37: 19, 39: 20, 41: 21, 43: 22, 50: 23, 51: 24, 52: 25,
    53: 26, 54: 27, 55: 28, 57: 29, 61: 30, 65: 32, 66: 33, 67: 34, 68: 35,
    87: 25, 88: 25, 89: 26, 90: 27, 91: 26, 92: 26, 93: 26,
};

type AoWCompatStatus = 'compatible' | 'incompatible' | 'unknown';

// Returns three-way compatibility status. Uses BigInt for bits 32–35 (shields).
// Fail-closed: bitmask==0 or wepType==0/unknown → 'unknown'.
function getAoWCompatStatus(aowCompatBitmask: number, wepType: number): AoWCompatStatus {
    if (aowCompatBitmask === 0 || wepType === 0) return 'unknown';
    const bitPos = WEP_TYPE_TO_BIT[wepType];
    if (bitPos === undefined) return 'unknown';
    const bit = (BigInt(aowCompatBitmask) >> BigInt(bitPos)) & BigInt(1);
    return bit === BigInt(1) ? 'compatible' : 'incompatible';
}

const WEAPON_CATEGORY_LABELS: Record<string, string> = {
    melee_armaments: 'Melee Armaments',
    ranged_and_catalysts: 'Ranged / Catalysts',
    shields: 'Shields',
};

interface AshOfWarOption {
    id: number;
    name: string;
    iconPath: string;
    isDlc: boolean;
    aowCompatBitmask: number;
}

interface OwnedWeapon {
    key: string;
    handle: number;
    id: number;
    baseId: number;
    name: string;
    category: string;
    subCategory: string;
    currentUpgrade: number;
    maxUpgrade: number;
    iconPath: string;
    location: 'inventory' | 'storage';
    currentAoWId: number;
    canMountAoW: boolean;
    wepType: number;
}

interface AoWAvailabilityEntry {
    itemId: number;
    totalCopies: number;
    availableCopies: number;
    usedCopies: number;
    usedByWeaponHandles: number[];
    isMissing: boolean;
    hasSharedHandleConflict: boolean;
}

interface Props {
    charIndex: number;
    inventoryVersion: number;
    infuseTypes: db.InfuseType[];
    platform: string | null;
}

export function WeaponEditTab({ charIndex, inventoryVersion, infuseTypes, platform }: Props) {
    const [weapons, setWeapons] = useState<OwnedWeapon[]>([]);
    const [weaponsLoading, setWeaponsLoading] = useState(false);
    const [brokenIcons, setBrokenIcons] = useState(new Set<string>());
    const [localVersion, setLocalVersion] = useState(0);

    const [ashesOfWar, setAshesOfWar] = useState<AshOfWarOption[]>([]);
    const [aowLoading, setAowLoading] = useState(false);

    const [search, setSearch] = useState('');
    const [aowSearch, setAowSearch] = useState('');
    const [categoryFilter, setCategoryFilter] = useState<string>('all');
    const [locationFilter, setLocationFilter] = useState<'both' | 'inventory' | 'storage'>('both');

    const [selectedKey, setSelectedKey] = useState<string | null>(null);
    const [selectedAoW, setSelectedAoW] = useState<number | null>(null);
    const [selectedInfusion, setSelectedInfusion] = useState<number | null>(null);
    const [applying, setApplying] = useState(false);
    const [aowAvailability, setAowAvailability] = useState<Map<number, AoWAvailabilityEntry>>(new Map());
    const [showCopyModal, setShowCopyModal] = useState(false);
    const [modalForCombined, setModalForCombined] = useState(false);

    useEffect(() => {
        setAowLoading(true);
        GetItemList('ashes_of_war').then(items => {
            const aows: AshOfWarOption[] = items.map(item => ({
                id: item.id,
                name: item.name,
                iconPath: item.iconPath,
                aowCompatBitmask: (item as any).aowCompatBitmask ?? 0,
                isDlc: (item.flags ?? []).includes('dlc'),
            })).sort((a, b) => a.name.localeCompare(b.name));
            setAshesOfWar(aows);
            setAowLoading(false);
        }).catch(() => setAowLoading(false));
    }, []);

    useEffect(() => {
        if (charIndex < 0) { setWeapons([]); return; }
        setWeaponsLoading(true);
        GetCharacter(charIndex).then(char => {
            if (!char) { setWeapons([]); setWeaponsLoading(false); return; }
            const byHandle = new Map<number, OwnedWeapon>();
            const process = (item: any, location: 'inventory' | 'storage') => {
                if (!WEAPON_CATEGORIES.has(item.subCategory)) return;
                const handle = item.handle as number;
                if (!handle) return;
                byHandle.set(handle, {
                    key: `w-${handle}`,
                    handle,
                    id: item.id,
                    baseId: item.baseId,
                    name: item.name,
                    category: item.category,
                    subCategory: item.subCategory ?? '',
                    currentUpgrade: item.currentUpgrade ?? 0,
                    maxUpgrade: item.maxUpgrade ?? 0,
                    iconPath: item.iconPath ?? '',
                    location,
                    currentAoWId: (item.aowId as number) || 0,
                    canMountAoW: (item as any).canMountAoW === true,
                    wepType: (item as any).wepType ?? 0,
                });
            };
            (char.inventory ?? []).forEach((i: any) => process(i, 'inventory'));
            (char.storage ?? []).forEach((i: any) => process(i, 'storage'));
            const sorted = Array.from(byHandle.values()).sort((a, b) => {
                const nc = a.name.localeCompare(b.name);
                return nc !== 0 ? nc : a.location.localeCompare(b.location);
            });
            setWeapons(sorted);
            setWeaponsLoading(false);
        }).catch(() => { setWeapons([]); setWeaponsLoading(false); });
    }, [charIndex, inventoryVersion, localVersion]);

    useEffect(() => {
        if (charIndex < 0) { setAowAvailability(new Map()); return; }
        GetAoWAvailability(charIndex).then(entries => {
            const m = new Map<number, AoWAvailabilityEntry>();
            (entries ?? []).forEach((e: any) => m.set(e.itemId as number, e as AoWAvailabilityEntry));
            setAowAvailability(m);
        }).catch(() => setAowAvailability(new Map()));
    }, [charIndex, inventoryVersion, localVersion]);

    const filteredWeapons = useMemo(() => weapons.filter(w => {
        if (categoryFilter !== 'all' && w.category !== categoryFilter) return false;
        if (locationFilter === 'inventory' && w.location !== 'inventory') return false;
        if (locationFilter === 'storage' && w.location !== 'storage') return false;
        if (search && !w.name.toLowerCase().includes(search.toLowerCase())) return false;
        return true;
    }), [weapons, categoryFilter, locationFilter, search]);

    const filteredAoW = useMemo(() => {
        if (!aowSearch) return ashesOfWar;
        const q = aowSearch.toLowerCase();
        return ashesOfWar.filter(a => a.name.toLowerCase().includes(q));
    }, [ashesOfWar, aowSearch]);

    const selectedWeapon = weapons.find(w => w.key === selectedKey) ?? null;
    const canInfuse = selectedWeapon ? selectedWeapon.canMountAoW : null;
    const canMountAoW = canInfuse;

    // true when the weapon has a real wepType absent from WEP_TYPE_TO_BIT (DLC types 69/94/95).
    // Backend allows Apply via the known==false passthrough in ApplyWeaponAoW / ApplyWeaponAoWStrict.
    const isWepTypeUnmapped = selectedWeapon !== null
        && selectedWeapon.wepType !== 0
        && WEP_TYPE_TO_BIT[selectedWeapon.wepType] === undefined;

    const currentInfuseOffset = selectedWeapon
        ? selectedWeapon.id - selectedWeapon.baseId - selectedWeapon.currentUpgrade
        : null;
    const currentInfusionName = infuseTypes.find(t => t.offset === currentInfuseOffset)?.name ?? null;
    const selectedInfusionName = infuseTypes.find(t => t.offset === selectedInfusion)?.name ?? null;
    const selectedAoWName = selectedAoW === 0
        ? 'None'
        : selectedAoW !== null ? (ashesOfWar.find(a => a.id === selectedAoW)?.name ?? null) : null;

    const currentAoWId = selectedWeapon?.currentAoWId ?? 0;
    const currentAoWName = currentAoWId !== 0
        ? (ashesOfWar.find(a => a.id === currentAoWId)?.name ?? null)
        : null;

    const infusionChanged = canInfuse === true
        && selectedInfusion !== null
        && selectedInfusion !== currentInfuseOffset;
    const aowChanged = canMountAoW === true && selectedAoW !== null && selectedAoW !== currentAoWId;
    const hasPendingChanges = selectedWeapon !== null && (aowChanged || infusionChanged);

    const getAoWStatus = (aowId: number): 'current' | 'available' | 'equipped' | 'missing' | 'conflict' => {
        if (selectedWeapon && currentAoWId !== 0 && aowId === currentAoWId) return 'current';
        const avail = aowAvailability.get(aowId);
        if (!avail || avail.totalCopies === 0) return 'missing';
        if (avail.hasSharedHandleConflict) return 'conflict';
        if (avail.availableCopies > 0) return 'available';
        return 'equipped';
    };

    const selectedAoWStatus = selectedAoW !== null && selectedAoW !== 0
        ? getAoWStatus(selectedAoW)
        : null;

    const selectedAoWCompatStatus: AoWCompatStatus | null =
        selectedAoW !== null && selectedAoW !== 0 && selectedWeapon !== null
            ? (() => {
                const aowEntry = ashesOfWar.find(a => a.id === selectedAoW);
                if (!aowEntry) return 'unknown';
                return getAoWCompatStatus(aowEntry.aowCompatBitmask, selectedWeapon.wepType);
            })()
            : null;

    const isAoWOnlyChange = aowChanged && !infusionChanged;
    const isInfusionOnlyChange = infusionChanged && !aowChanged;

    // Unified AoW gate — no editor/strict exposed to user.
    // Remove (selectedAoW===0) always passes. Missing/Equipped pass — modal gates copy creation.
    // Conflict blocks. Known incompatible blocks. Standard unknown blocks. DLC unmapped unknown passes.
    const canApplyAoW = isAoWOnlyChange && canMountAoW === true
        && selectedAoWStatus !== 'conflict'
        && (selectedAoW === 0
            || selectedAoWCompatStatus === 'compatible'
            || (selectedAoWCompatStatus === 'unknown' && isWepTypeUnmapped));

    // Combined gate: both infusion + AoW pending at the same time.
    // Same AoW rules as canApplyAoW, but both changes must be valid simultaneously.
    const canApplyCombined = infusionChanged && aowChanged
        && canInfuse === true && canMountAoW === true
        && selectedAoWStatus !== 'conflict'
        && (selectedAoW === 0
            || selectedAoWCompatStatus === 'compatible'
            || (selectedAoWCompatStatus === 'unknown' && isWepTypeUnmapped));

    const canApply = selectedWeapon !== null
        && !applying
        && ((isInfusionOnlyChange && canInfuse === true) || canApplyAoW || canApplyCombined);

    const applyTooltip = (() => {
        if (!selectedWeapon) return 'No weapon selected';
        if (applying) return 'Applying changes...';
        if (aowChanged && infusionChanged) {
            if (selectedAoWStatus === 'conflict') return 'Cannot apply — Ash of War conflict detected';
            if (selectedAoW !== 0 && selectedAoWCompatStatus === 'incompatible') return 'This Ash of War is not compatible with this weapon type';
            if (selectedAoW !== 0 && selectedAoWCompatStatus === 'unknown' && !isWepTypeUnmapped) return 'Ash of War compatibility is unknown for this weapon type';
            if (selectedAoWStatus === 'missing' || selectedAoWStatus === 'equipped') return 'Apply — a confirmation prompt will appear to add a new copy';
            return 'Apply infusion and Ash of War changes';
        }
        if (infusionChanged) return canInfuse === true ? 'Apply infusion change' : 'This weapon does not support affinity changes';
        if (aowChanged) {
            if (canMountAoW !== true) return 'This weapon does not support Ash of War';
            if (selectedAoW !== 0 && selectedAoWCompatStatus === 'unknown' && !isWepTypeUnmapped) return 'Ash of War compatibility is unknown for this weapon type';
            if (selectedAoW !== 0 && selectedAoWCompatStatus === 'incompatible') return 'This Ash of War is not compatible with this weapon type';
            if (selectedAoW === 0) return 'Remove current Ash of War from this weapon.';
            if (selectedAoWStatus === 'conflict') return 'Cannot apply — Ash of War handle conflict detected';
            if (selectedAoWStatus === 'missing' || selectedAoWStatus === 'equipped') return 'Apply — a confirmation prompt will appear to add a new copy';
            return 'Apply Ash of War';
        }
        return 'No pending changes';
    })();

    const handleSelectWeapon = (key: string) => {
        setSelectedKey(key);
        setSelectedAoW(null);
        setSelectedInfusion(null);
        setShowCopyModal(false);
        setModalForCombined(false);
    };

    const handleReset = () => {
        setSelectedAoW(null);
        setSelectedInfusion(null);
        setShowCopyModal(false);
        setModalForCombined(false);
    };

    const doApplyAoW = async (createCopy: boolean) => {
        if (selectedAoW === null || !selectedWeapon) return;
        setApplying(true);
        setShowCopyModal(false);
        setModalForCombined(false);
        try {
            if (createCopy) {
                await ApplyWeaponAoW(charIndex, selectedWeapon.handle, selectedAoW);
            } else {
                await ApplyWeaponAoWStrict(charIndex, selectedWeapon.handle, selectedAoW);
            }
            toast.success('Weapon Ash of War updated successfully.');
            setSelectedAoW(null);
            setLocalVersion(v => v + 1);
        } catch (e) {
            toast.error('Failed to apply Ash of War: ' + e);
        } finally {
            setApplying(false);
        }
    };

    // Applies both infusion and AoW in sequence with rollback on AoW failure.
    // createCopy=true → ApplyWeaponAoW (adds inventory copy); false → ApplyWeaponAoWStrict (links free copy).
    // On AoW failure: attempts 2× RevertSlot to handle both pre-pushUndo (1 snapshot) and
    // post-pushUndo (2 snapshots) failure paths. The second call is silently ignored if stack is empty.
    const doApplyCombined = async (createCopy: boolean) => {
        if (selectedAoW === null || selectedInfusion === null || !selectedWeapon) return;
        setApplying(true);
        setShowCopyModal(false);
        setModalForCombined(false);
        const newItemId = selectedWeapon.baseId + selectedWeapon.currentUpgrade + selectedInfusion;
        try {
            await ApplyWeaponInfusion(charIndex, selectedWeapon.handle, selectedWeapon.id, newItemId);
        } catch (e) {
            toast.error('Failed to apply infusion: ' + e);
            setApplying(false);
            return;
        }
        try {
            if (createCopy) {
                await ApplyWeaponAoW(charIndex, selectedWeapon.handle, selectedAoW);
            } else {
                await ApplyWeaponAoWStrict(charIndex, selectedWeapon.handle, selectedAoW);
            }
        } catch (_e) {
            try { await RevertSlot(charIndex); } catch (_) {}
            try { await RevertSlot(charIndex); } catch (_) {}
            toast.error('Ash of War could not be applied; infusion change was rolled back.');
            setApplying(false);
            return;
        }
        toast.success('Infusion and Ash of War updated successfully.');
        setSelectedAoW(null);
        setSelectedInfusion(null);
        setLocalVersion(v => v + 1);
        setApplying(false);
    };

    const handleApply = async () => {
        if (!selectedWeapon) return;

        if (infusionChanged && aowChanged) {
            if (selectedAoW === null || selectedInfusion === null) return;
            if (selectedAoW === 0 || selectedAoWStatus === 'available') {
                await doApplyCombined(false);
            } else if (selectedAoWStatus === 'missing' || selectedAoWStatus === 'equipped') {
                setModalForCombined(true);
                setShowCopyModal(true);
            }
            return;
        }

        if (isInfusionOnlyChange) {
            if (selectedInfusion === null) return;
            const newItemId = selectedWeapon.baseId + selectedWeapon.currentUpgrade + selectedInfusion;
            setApplying(true);
            try {
                await ApplyWeaponInfusion(charIndex, selectedWeapon.handle, selectedWeapon.id, newItemId);
                toast.success('Weapon infusion updated successfully.');
                setSelectedInfusion(null);
                setLocalVersion(v => v + 1);
            } catch (e) {
                toast.error('Failed to apply infusion: ' + e);
            } finally {
                setApplying(false);
            }
        } else if (isAoWOnlyChange) {
            if (selectedAoW === null) return;
            if (selectedAoW === 0 || selectedAoWStatus === 'available') {
                await doApplyAoW(false);
            } else if (selectedAoWStatus === 'missing' || selectedAoWStatus === 'equipped') {
                setShowCopyModal(true);
            }
        }
    };

    const currentInfusionLabel = (() => {
        if (canInfuse === false && currentInfusionName === null)
            return <span className="text-foreground/40 italic">Unknown / Not supported</span>;
        if (currentInfusionName)
            return <span className="text-foreground font-bold">{currentInfusionName}</span>;
        return <span className="text-foreground font-bold">Standard</span>;
    })();

    const modalText = (() => {
        const base = selectedAoWStatus === 'equipped'
            ? 'This Ash of War is already equipped on another weapon. Do you want to add another copy and apply it here?'
            : 'This Ash of War is not available as a free copy in your inventory. Do you want to add one and apply it to this weapon?';
        return modalForCombined
            ? base + ' The pending infusion change will also be applied at the same time.'
            : base;
    })();

    return (
        <div className="flex-1 flex flex-col min-h-0 gap-3 animate-in fade-in slide-in-from-bottom-4 duration-500">

            {/* Copy-creation confirmation modal */}
            {showCopyModal && (
                <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
                    <div className="bg-background border border-border rounded-xl p-5 max-w-sm w-full mx-4 shadow-2xl">
                        <h3 className="text-[11px] font-black uppercase tracking-[0.15em] text-foreground mb-2">Add Ash of War Copy</h3>
                        <p className="text-[10px] text-muted-foreground leading-relaxed">{modalText}</p>
                        <div className="flex gap-2 mt-4">
                            <button
                                onClick={() => modalForCombined ? doApplyCombined(true) : doApplyAoW(true)}
                                className="flex-1 px-4 py-2 bg-green-700 text-white rounded-lg text-[9px] font-black uppercase tracking-widest hover:bg-green-600 transition-all"
                            >
                                Yes, add copy
                            </button>
                            <button
                                onClick={() => { setShowCopyModal(false); setModalForCombined(false); }}
                                className="flex-1 px-4 py-2 bg-muted/40 text-foreground rounded-lg text-[9px] font-black uppercase tracking-widest border border-border hover:bg-muted/60 transition-all"
                            >
                                Cancel
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Main body */}
            <div className="flex-1 flex min-h-0 gap-3">

                {/* ── Left: weapon list ── */}
                <div className="w-64 flex flex-col min-h-0 card overflow-hidden shrink-0">
                    <div className="px-3 pt-3 pb-2 border-b border-border/50 shrink-0 space-y-2">
                        <div className="flex items-center justify-between">
                            <span className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground">Weapons</span>
                            <span className="text-[9px] font-bold tabular-nums text-muted-foreground bg-muted/30 px-1.5 py-0.5 rounded">
                                {filteredWeapons.length}/{weapons.length}
                            </span>
                        </div>
                        <div className="relative">
                            <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3 h-3 text-muted-foreground/50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                            </svg>
                            <input
                                type="text"
                                placeholder="Search weapons..."
                                value={search}
                                onChange={e => setSearch(e.target.value)}
                                className="w-full bg-muted/20 border border-border/50 rounded-md pl-7 pr-3 py-1.5 text-[10px] focus:outline-none focus:ring-1 focus:ring-primary/20 focus:border-primary transition-all"
                            />
                        </div>
                        <div className="relative">
                            <select
                                value={categoryFilter}
                                onChange={e => setCategoryFilter(e.target.value)}
                                className="w-full appearance-none bg-muted/20 border border-border/50 rounded-md px-3 py-1.5 pr-7 text-[10px] font-black uppercase tracking-widest text-muted-foreground outline-none cursor-pointer focus:ring-1 focus:ring-primary/20 transition-all"
                            >
                                <option value="all">All Weapons</option>
                                <option value="melee_armaments">Melee Armaments</option>
                                <option value="ranged_and_catalysts">Ranged / Catalysts</option>
                                <option value="shields">Shields</option>
                            </select>
                            <div className="absolute right-2.5 top-1/2 -translate-y-1/2 pointer-events-none text-muted-foreground">
                                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M19 9l-7 7-7-7"/></svg>
                            </div>
                        </div>
                        <div className="flex gap-1">
                            {(['both', 'inventory', 'storage'] as const).map(loc => (
                                <button
                                    key={loc}
                                    onClick={() => setLocationFilter(loc)}
                                    className={`flex-1 py-1 text-[9px] font-black uppercase tracking-wider rounded transition-all ${locationFilter === loc ? 'bg-green-700/80 text-white' : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'}`}
                                >
                                    {loc === 'both' ? 'All' : loc === 'inventory' ? 'Inv' : 'Stg'}
                                </button>
                            ))}
                        </div>
                    </div>

                    <div className="flex-1 overflow-y-auto custom-scrollbar">
                        {weaponsLoading ? (
                            <div className="flex items-center justify-center py-12">
                                <div className="w-5 h-5 border-2 border-foreground/20 border-t-foreground rounded-full animate-spin" />
                            </div>
                        ) : filteredWeapons.length === 0 ? (
                            <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
                                <svg className="w-6 h-6 text-muted-foreground/30 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/>
                                </svg>
                                <p className="text-[10px] font-bold text-muted-foreground/50 uppercase tracking-widest">
                                    {platform ? 'No weapons found' : 'No save loaded'}
                                </p>
                            </div>
                        ) : (
                            filteredWeapons.map(w => (
                                <button
                                    key={w.key}
                                    onClick={() => handleSelectWeapon(w.key)}
                                    className={`w-full flex items-center gap-2 px-3 py-2 text-left transition-all border-b border-border/20 hover:bg-muted/20 ${selectedKey === w.key ? 'bg-green-700/10 border-l-2 border-l-green-700' : 'border-l-2 border-l-transparent'}`}
                                >
                                    <div className="w-8 h-8 rounded bg-muted/30 border border-border/30 flex items-center justify-center shrink-0 overflow-hidden">
                                        {brokenIcons.has(w.iconPath)
                                            ? <span className="text-[8px] text-muted-foreground/30">?</span>
                                            : <img src={w.iconPath} alt="" className="w-full h-full object-contain p-0.5" onError={() => setBrokenIcons(p => new Set(p).add(w.iconPath))} />
                                        }
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <div className="text-[10px] font-bold text-foreground truncate">{w.name}</div>
                                        <div className="flex items-center gap-1.5 mt-0.5">
                                            <span className="text-[8px] text-muted-foreground/60 truncate">{w.subCategory || WEAPON_CATEGORY_LABELS[w.category]}</span>
                                            {w.maxUpgrade > 0 && (
                                                <span className="text-[8px] font-bold text-primary/70">+{w.currentUpgrade}</span>
                                            )}
                                        </div>
                                    </div>
                                    <div className="shrink-0">
                                        {w.location === 'inventory'
                                            ? <span className="text-[7px] font-black uppercase bg-blue-500/10 text-blue-500 border border-blue-500/20 px-1 rounded">INV</span>
                                            : <span className="text-[7px] font-black uppercase bg-muted/30 text-muted-foreground border border-border/30 px-1 rounded">STG</span>
                                        }
                                    </div>
                                </button>
                            ))
                        )}
                    </div>
                </div>

                {/* ── Right: detail + editors ── */}
                <div className="flex-1 flex flex-col min-h-0 gap-3 overflow-y-auto custom-scrollbar pr-0.5">

                    {!selectedWeapon ? (
                        <div className="flex-1 flex flex-col items-center justify-center gap-3 text-center card">
                            <div className="w-14 h-14 rounded-full bg-muted/20 border border-border/50 flex items-center justify-center">
                                <svg className="w-7 h-7 text-muted-foreground/30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125" />
                                </svg>
                            </div>
                            <div>
                                <p className="text-[11px] font-black uppercase tracking-[0.15em] text-foreground/60">Select a weapon to edit</p>
                                <p className="text-[10px] text-muted-foreground mt-1 max-w-xs leading-relaxed">
                                    Choose an owned weapon from the list to configure its Ash of War and infusion.
                                </p>
                            </div>
                        </div>
                    ) : (
                        <>
                            {/* Selected weapon header */}
                            <div className="card p-4 shrink-0">
                                <div className="flex items-center gap-4">
                                    <div className="w-16 h-16 rounded-xl bg-muted/20 border border-border/50 flex items-center justify-center shrink-0 overflow-hidden">
                                        {brokenIcons.has(selectedWeapon.iconPath)
                                            ? <span className="text-muted-foreground/30 text-xs">?</span>
                                            : <img src={selectedWeapon.iconPath} alt="" className="w-full h-full object-contain p-1" onError={() => setBrokenIcons(p => new Set(p).add(selectedWeapon.iconPath))} />
                                        }
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <h2 className="text-sm font-black uppercase tracking-wider text-foreground">{selectedWeapon.name}</h2>
                                        <div className="flex items-center flex-wrap gap-2 mt-1">
                                            <span className="text-[9px] font-bold uppercase tracking-wider text-muted-foreground">
                                                {selectedWeapon.subCategory || WEAPON_CATEGORY_LABELS[selectedWeapon.category]}
                                            </span>
                                            {selectedWeapon.maxUpgrade > 0 && (
                                                <span className="text-[9px] font-black text-primary bg-primary/10 border border-primary/20 px-1.5 py-0.5 rounded">
                                                    +{selectedWeapon.currentUpgrade} / +{selectedWeapon.maxUpgrade}
                                                </span>
                                            )}
                                            {selectedWeapon.location === 'inventory'
                                                ? <span className="text-[8px] font-black uppercase bg-blue-500/10 text-blue-500 border border-blue-500/20 px-1.5 py-0.5 rounded">Inventory</span>
                                                : <span className="text-[8px] font-black uppercase bg-muted/30 text-muted-foreground border border-border/30 px-1.5 py-0.5 rounded">Storage</span>
                                            }
                                        </div>
                                        <div className="flex items-center gap-4 mt-2 flex-wrap">
                                            <span className="text-[9px] text-muted-foreground">
                                                Current Ash of War:{' '}
                                                {currentAoWId === 0
                                                    ? <span className="text-foreground/40 italic">None</span>
                                                    : currentAoWName
                                                        ? <span className="text-foreground font-bold">{currentAoWName}</span>
                                                        : <span className="text-foreground/40 italic">Unknown (0x{currentAoWId.toString(16).toUpperCase()})</span>
                                                }
                                            </span>
                                            <span className="text-[9px] text-muted-foreground">
                                                Current Infusion: {currentInfusionLabel}
                                            </span>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* AoW + Infusion editors */}
                            <div className="flex gap-3 shrink-0">

                                {/* Ash of War */}
                                <div className="flex-1 card p-4 flex flex-col gap-3 min-w-0">
                                    <div className="flex items-center justify-between gap-2">
                                        <span className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground shrink-0">Ash of War</span>
                                        <div className="flex items-center gap-2">
                                            {canMountAoW === false ? (
                                                <span className="text-[8px] font-bold text-red-500/70 bg-red-500/10 border border-red-500/20 px-1.5 py-0.5 rounded">
                                                    Not supported
                                                </span>
                                            ) : currentAoWId !== 0 ? (
                                                <button
                                                    onClick={() => setSelectedAoW(selectedAoW === 0 ? null : 0)}
                                                    title="Remove current Ash of War from this weapon."
                                                    className={`px-2.5 py-1 text-[8px] font-black uppercase tracking-wider rounded-md border transition-all ${
                                                        selectedAoW === 0
                                                            ? 'bg-red-700/80 text-white border-red-700'
                                                            : 'text-red-500/70 bg-red-500/10 border-red-500/20 hover:bg-red-500/20'
                                                    }`}
                                                >
                                                    Remove AoW
                                                </button>
                                            ) : null}
                                        </div>
                                    </div>

                                    {canMountAoW === false && (
                                        <div className="flex items-start gap-2 px-2.5 py-2 rounded-lg bg-muted/20 border border-border/40">
                                            <svg className="w-3.5 h-3.5 text-muted-foreground/60 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 15v2m0 0v2m0-2h2m-2 0H10m2-9a3 3 0 110 6 3 3 0 010-6z"/>
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M18.364 5.636A9 9 0 115.636 18.364 9 9 0 0118.364 5.636z"/>
                                            </svg>
                                            <div>
                                                <p className="text-[9px] font-bold text-foreground/70">This weapon does not support Ash of War changes.</p>
                                                <p className="text-[8px] text-muted-foreground/60 mt-0.5">Unique, somber or special weapons have a fixed skill.</p>
                                            </div>
                                        </div>
                                    )}

                                    <div className="relative">
                                        <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3 h-3 text-muted-foreground/50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                                        </svg>
                                        <input
                                            type="text"
                                            placeholder="Search ashes of war..."
                                            value={aowSearch}
                                            onChange={e => setAowSearch(e.target.value)}
                                            disabled={canMountAoW === false}
                                            className="w-full bg-muted/20 border border-border/50 rounded-md pl-7 pr-3 py-1.5 text-[10px] focus:outline-none focus:ring-1 focus:ring-primary/20 focus:border-primary transition-all disabled:opacity-40 disabled:cursor-not-allowed"
                                        />
                                    </div>

                                    {/* AoW grid — 3 columns */}
                                    <div className={`grid grid-cols-3 gap-2 max-h-80 overflow-y-auto custom-scrollbar ${canMountAoW === false ? 'opacity-40 pointer-events-none' : ''}`}>
                                        {aowLoading ? (
                                            <div className="col-span-3 flex items-center justify-center py-8">
                                                <div className="w-4 h-4 border-2 border-foreground/20 border-t-foreground rounded-full animate-spin" />
                                            </div>
                                        ) : ashesOfWar.length === 0 ? (
                                            <p className="col-span-3 text-[9px] text-muted-foreground/50 italic text-center py-4">Ash of War database is not available yet.</p>
                                        ) : filteredAoW.length === 0 && aowSearch ? (
                                            <p className="col-span-3 text-[9px] text-muted-foreground/50 italic text-center py-4">No Ashes of War found.</p>
                                        ) : (
                                            filteredAoW.map(aow => {
                                                const isCurrent = currentAoWId !== 0 && aow.id === currentAoWId;
                                                const isSelected = aow.id === selectedAoW;
                                                const isPending = isSelected && aowChanged;
                                                const itemCompatStatus: AoWCompatStatus = selectedWeapon
                                                    ? getAoWCompatStatus(aow.aowCompatBitmask, selectedWeapon.wepType)
                                                    : 'unknown';
                                                const isIncompatible = itemCompatStatus === 'incompatible';
                                                const isUnknown = itemCompatStatus === 'unknown';
                                                const isClickBlocked = canMountAoW === false
                                                    || (isIncompatible && !isCurrent)
                                                    || (isUnknown && !isCurrent && !isWepTypeUnmapped);
                                                const avSt = getAoWStatus(aow.id);

                                                const tileTitle = (() => {
                                                    if (isClickBlocked && !isCurrent) {
                                                        if (isIncompatible) return 'This Ash of War is not compatible with this weapon type.';
                                                        if (isUnknown && !isWepTypeUnmapped) return 'Ash of War compatibility is unknown for this weapon type.';
                                                    }
                                                    if (isUnknown && isWepTypeUnmapped && !isCurrent)
                                                        return 'Compatibility data for this DLC weapon type is unavailable; backend will allow this save edit.';
                                                    return undefined;
                                                })();

                                                const statusBadge = (() => {
                                                    if (avSt === 'conflict') return <span className="text-[7px] font-black uppercase text-red-500/80 bg-red-500/10 border border-red-500/20 px-1.5 py-0.5 rounded">Conflict</span>;
                                                    if (isCurrent) return <span className="text-[7px] font-black uppercase text-blue-400/80 bg-blue-400/10 border border-blue-400/20 px-1.5 py-0.5 rounded">Current</span>;
                                                    if (isIncompatible) return <span className="text-[7px] font-black uppercase text-red-500/80 bg-red-500/10 border border-red-500/20 px-1.5 py-0.5 rounded">Incompatible</span>;
                                                    if (isUnknown && !isWepTypeUnmapped) return <span className="text-[7px] font-black uppercase text-amber-500/80 bg-amber-500/10 border border-amber-500/20 px-1.5 py-0.5 rounded">Unknown</span>;
                                                    if (avSt === 'available') return <span className="text-[7px] font-black uppercase text-green-500/80 bg-green-500/10 border border-green-500/20 px-1.5 py-0.5 rounded">Available</span>;
                                                    if (avSt === 'equipped') return <span className="text-[7px] font-black uppercase text-orange-400/80 bg-orange-500/10 border border-orange-500/20 px-1.5 py-0.5 rounded">Equipped</span>;
                                                    if (avSt === 'missing') return <span className="text-[7px] font-black uppercase text-muted-foreground/50 bg-muted/20 border border-border/30 px-1.5 py-0.5 rounded">Missing</span>;
                                                    return null;
                                                })();

                                                return (
                                                    <button
                                                        key={aow.id}
                                                        disabled={canMountAoW === false}
                                                        onClick={() => !isClickBlocked && setSelectedAoW(aow.id === selectedAoW ? null : aow.id)}
                                                        title={tileTitle}
                                                        className={`flex items-stretch min-h-[72px] rounded-xl border transition-all overflow-hidden ${
                                                            canMountAoW === false
                                                                ? 'border-border/20 cursor-not-allowed'
                                                                : isClickBlocked && !isCurrent
                                                                    ? 'opacity-35 border-border/20 cursor-not-allowed'
                                                                    : isPending
                                                                        ? 'bg-green-700/80 border-green-700 shadow-sm'
                                                                        : isCurrent && !isSelected
                                                                            ? 'bg-blue-500/10 border-blue-500/30 hover:bg-blue-500/15'
                                                                            : isSelected && !aowChanged
                                                                                ? 'bg-muted/40 border-border'
                                                                                : 'hover:bg-muted/30 border-border/30'
                                                        }`}
                                                    >
                                                        {/* Icon — 50% of tile width, stretches to full tile height */}
                                                        <div className="w-1/2 shrink-0 p-1.5">
                                                            <div className="w-full h-full rounded-lg bg-muted/30 flex items-center justify-center overflow-hidden">
                                                                {brokenIcons.has(aow.iconPath)
                                                                    ? <span className="text-[9px] text-muted-foreground/30">?</span>
                                                                    : <img src={aow.iconPath} alt="" className="w-full h-full object-contain" onError={() => setBrokenIcons(p => new Set(p).add(aow.iconPath))} />
                                                                }
                                                            </div>
                                                        </div>
                                                        {/* Text — 50% of tile width */}
                                                        <div className="w-1/2 flex flex-col justify-center gap-1 pr-2 py-1.5 min-w-0">
                                                            <span className={`text-[9px] font-bold leading-tight line-clamp-3 text-left ${isPending ? 'text-white' : 'text-foreground'}`}>{aow.name}</span>
                                                            <div className="flex flex-wrap gap-0.5">
                                                                {statusBadge}
                                                                {aow.isDlc && (
                                                                    <span className="text-[7px] font-black uppercase text-amber-500/70 bg-amber-500/10 border border-amber-500/20 px-1 py-0.5 rounded">DLC</span>
                                                                )}
                                                            </div>
                                                        </div>
                                                    </button>
                                                );
                                            })
                                        )}
                                    </div>
                                </div>

                                {/* Infusion */}
                                <div className="w-64 card p-4 flex flex-col gap-3 shrink-0">
                                    <div className="flex items-center justify-between">
                                        <span className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground">Infusion / Affinity</span>
                                        {canInfuse === false && (
                                            <span className="text-[8px] font-bold text-red-500/70 bg-red-500/10 border border-red-500/20 px-1.5 py-0.5 rounded">
                                                Not supported
                                            </span>
                                        )}
                                    </div>

                                    {canInfuse === false && (
                                        <div className="flex items-start gap-2 px-2.5 py-2 rounded-lg bg-muted/20 border border-border/40">
                                            <svg className="w-3.5 h-3.5 text-muted-foreground/60 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 15v2m0 0v2m0-2h2m-2 0H10m2-9a3 3 0 110 6 3 3 0 010-6z"/>
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M18.364 5.636A9 9 0 115.636 18.364 9 9 0 0118.364 5.636z"/>
                                            </svg>
                                            <div>
                                                <p className="text-[9px] font-bold text-foreground/70">This weapon does not support affinity changes.</p>
                                                <p className="text-[8px] text-muted-foreground/60 mt-0.5">Unique, somber or special weapons usually cannot be infused.</p>
                                            </div>
                                        </div>
                                    )}

                                    <div className={`grid grid-cols-2 gap-1.5 ${canInfuse === false ? 'opacity-40' : ''}`}>
                                        {infuseTypes.map(t => {
                                            const isCurrent = t.offset === currentInfuseOffset;
                                            const isSelected = t.offset === selectedInfusion;
                                            const isPending = isSelected && infusionChanged;
                                            return (
                                                <button
                                                    key={t.offset}
                                                    disabled={canInfuse === false}
                                                    onClick={() => setSelectedInfusion(t.offset === selectedInfusion ? null : t.offset)}
                                                    className={`relative px-2 py-2 rounded-lg text-[9px] font-black uppercase tracking-wider text-center transition-all border ${
                                                        canInfuse === false
                                                            ? 'border-border/30 text-foreground cursor-not-allowed'
                                                            : isPending
                                                                ? 'bg-green-700/80 text-white border-green-700'
                                                                : isSelected && !infusionChanged
                                                                    ? 'bg-muted/40 text-foreground border-border'
                                                                    : 'hover:bg-muted/30 border-border/30 text-foreground'
                                                    }`}
                                                >
                                                    {t.name}
                                                    {isCurrent && (
                                                        <span className="absolute top-0.5 right-0.5 w-1.5 h-1.5 rounded-full bg-blue-400" title="Current infusion" />
                                                    )}
                                                </button>
                                            );
                                        })}
                                    </div>
                                </div>
                            </div>

                            {/* Pending Changes */}
                            <div className="card p-4 shrink-0">
                                <div className="flex items-center justify-between mb-3">
                                    <span className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground">Pending Changes</span>
                                    {hasPendingChanges && (
                                        <span className="text-[8px] font-black uppercase tracking-wider text-amber-600 bg-amber-500/10 border border-amber-500/20 px-2 py-0.5 rounded-full animate-pulse">
                                            Unsaved
                                        </span>
                                    )}
                                </div>
                                <div className="grid grid-cols-3 gap-4">
                                    <div>
                                        <span className="text-[8px] font-black uppercase tracking-widest text-muted-foreground block mb-1">Weapon</span>
                                        <span className="text-[10px] font-bold text-foreground">{selectedWeapon.name}</span>
                                    </div>
                                    <div>
                                        <span className="text-[8px] font-black uppercase tracking-widest text-muted-foreground block mb-1">Ash of War</span>
                                        {aowChanged ? (
                                            <span className="text-[10px] font-bold text-green-600">
                                                {currentAoWName ?? (currentAoWId !== 0 ? 'Unknown' : 'None')} → {selectedAoWName}
                                            </span>
                                        ) : (
                                            <span className="text-[10px] font-bold text-muted-foreground/40 italic">No change</span>
                                        )}
                                    </div>
                                    <div>
                                        <span className="text-[8px] font-black uppercase tracking-widest text-muted-foreground block mb-1">Infusion</span>
                                        {infusionChanged ? (
                                            <span className="text-[10px] font-bold text-green-600">
                                                {currentInfusionName ?? 'Standard'} → {selectedInfusionName}
                                            </span>
                                        ) : (
                                            <span className="text-[10px] font-bold text-muted-foreground/40 italic">No change</span>
                                        )}
                                    </div>
                                </div>
                                <p className="text-[9px] text-muted-foreground/40 mt-3 italic">Stat preview is not available yet.</p>
                            </div>
                        </>
                    )}
                </div>
            </div>

            {/* Bottom action bar */}
            <div className="flex items-center gap-3 shrink-0 pt-2 border-t border-border/50">
                <button
                    onClick={handleReset}
                    disabled={!hasPendingChanges || applying}
                    className="px-6 py-2 bg-muted/30 text-foreground rounded-lg text-[10px] font-black uppercase tracking-widest border border-border hover:bg-muted/50 transition-all disabled:opacity-40 disabled:cursor-not-allowed"
                >
                    Reset
                </button>
                <button
                    onClick={handleApply}
                    disabled={!canApply}
                    title={applyTooltip}
                    className={`px-6 py-2 rounded-lg text-[10px] font-black uppercase tracking-widest border transition-all flex items-center gap-2 ${
                        canApply
                            ? 'bg-green-700 text-white border-green-700 hover:bg-green-600'
                            : 'bg-muted/20 text-muted-foreground/50 border-border/50 cursor-not-allowed'
                    }`}
                >
                    {applying && <div className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />}
                    Apply Changes
                </button>
                <button
                    disabled
                    title="Loadout saving is not implemented yet."
                    className="px-6 py-2 bg-muted/20 text-muted-foreground/50 rounded-lg text-[10px] font-black uppercase tracking-widest border border-border/50 cursor-not-allowed"
                >
                    Save as Loadout
                </button>
                <span className="ml-auto text-[9px] italic">
                    {(() => {
                        if (aowChanged && infusionChanged) {
                            if (selectedAoWStatus === 'conflict')
                                return <span className="text-red-500/70">Cannot apply — Ash of War conflict detected.</span>;
                            if (selectedAoW !== 0 && selectedAoWCompatStatus === 'incompatible')
                                return <span className="text-red-500/70">This Ash of War is not compatible with this weapon type.</span>;
                            if (selectedAoW !== 0 && selectedAoWCompatStatus === 'unknown' && !isWepTypeUnmapped)
                                return <span className="text-amber-500/70">Ash of War compatibility is unknown for this weapon.</span>;
                            if (selectedAoWStatus === 'missing' || selectedAoWStatus === 'equipped')
                                return <span className="text-amber-500/70">A copy of this Ash of War will be added when applied.</span>;
                            return null;
                        }
                        if (aowChanged && selectedAoW === 0)
                            return <span className="text-sky-400/70">Ash of War will be removed from this weapon.</span>;
                        if (aowChanged && selectedAoW !== 0 && selectedAoWCompatStatus === 'unknown')
                            return isWepTypeUnmapped
                                ? <span className="text-sky-400/70">Compatibility data for this DLC weapon type is unavailable; backend will allow this save edit.</span>
                                : <span className="text-amber-500/70">Ash of War compatibility is unknown for this weapon.</span>;
                        if (aowChanged && selectedAoW !== 0 && selectedAoWCompatStatus === 'incompatible')
                            return <span className="text-red-500/70">This Ash of War is not compatible with this weapon type.</span>;
                        if (aowChanged && selectedAoWStatus === 'missing')
                            return <span className="text-amber-500/70">This Ash of War is missing from the save — applying will prompt to add a copy.</span>;
                        if (aowChanged && selectedAoWStatus === 'equipped')
                            return <span className="text-amber-500/70">Already equipped elsewhere — applying will prompt to add another copy.</span>;
                        if (!hasPendingChanges)
                            return <span className="text-muted-foreground/40">No pending changes.</span>;
                        return null;
                    })()}
                </span>
            </div>
        </div>
    );
}
