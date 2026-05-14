import { useCallback, useEffect, useMemo, useState } from 'react';
import {
    ApplyWeaponAoWStrict,
    ApplyWeaponInfusion,
    ApplyWeaponUpgradeLevel,
    GetAoWAvailability,
    GetCharacter,
    GetInfuseTypes,
    GetItemList,
} from '../../wailsjs/go/main/App';
import { db, main } from '../../wailsjs/go/models';

interface Props {
    charIndex: number;
    item: main.InventoryOrderItem;
    source: 'inventory' | 'storage';
    onClose: () => void;
    onApplied?: (patch: WeaponPatch) => void;
}

// WeaponPatch — fields that may change after an Apply.
// SortOrderTab uses this to patch its local previewItems / baseItems by handle
// without refetching (preserves any pending preview reorder).
export interface WeaponPatch {
    handle: number;
    itemId: number;
    currentUpgrade: number;
    infusionName?: string;
}

// Mirrors backend data.WepTypeToCanMountBit (also duplicated in WeaponEditTab).
// Maps weapon wepType → bit position in AoWCompatBitmask.
const WEP_TYPE_TO_BIT: Record<number, number> = {
    1: 0, 3: 1, 5: 2, 7: 3, 9: 8, 11: 9, 13: 6, 14: 5, 15: 4, 16: 7, 17: 7,
    19: 11, 21: 13, 23: 10, 24: 10, 25: 12, 28: 14, 29: 14, 31: 15, 32: 17,
    33: 18, 35: 16, 37: 19, 39: 20, 41: 21, 43: 22, 50: 23, 51: 24, 52: 25,
    53: 26, 54: 27, 55: 28, 57: 29, 61: 30, 65: 32, 66: 33, 67: 34, 68: 35,
    87: 25, 88: 25, 89: 26, 90: 27, 91: 26, 92: 26, 93: 26,
};

type AoWCompatStatus = 'compatible' | 'incompatible' | 'unknown';

function getAoWCompatStatus(aowCompatBitmask: number, wepType: number): AoWCompatStatus {
    if (aowCompatBitmask === 0 || wepType === 0) return 'unknown';
    const bitPos = WEP_TYPE_TO_BIT[wepType];
    if (bitPos === undefined) return 'unknown';
    const bit = (BigInt(aowCompatBitmask) >> BigInt(bitPos)) & BigInt(1);
    return bit === BigInt(1) ? 'compatible' : 'incompatible';
}

interface AshOfWarOption {
    id: number;
    name: string;
    iconPath: string;
    aowCompatBitmask: number;
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

type AoWStatus = 'current' | 'available' | 'in_use' | 'missing' | 'conflict';

export function WeaponEditModal({ charIndex, item, source, onClose, onApplied }: Props) {
    const [imgError, setImgError] = useState(false);

    // Live working state — starts from props but tracks Apply results so the
    // modal can show the new level / itemId / infusion without being closed.
    const [currentItemId, setCurrentItemId] = useState<number>(item.itemId);
    const [currentLevel, setCurrentLevel] = useState<number>(item.currentUpgrade ?? 0);
    const [currentInfusionName, setCurrentInfusionName] = useState<string>(item.infusionName ?? '');
    const maxUpgrade = item.maxUpgrade ?? 0;

    const [selectedLevel, setSelectedLevel] = useState<number>(item.currentUpgrade ?? 0);
    const [applying, setApplying] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [success, setSuccess] = useState<string | null>(null);

    // Infusion options loaded from backend (small static list, 13 entries).
    const [infuseTypes, setInfuseTypes] = useState<db.InfuseType[]>([]);
    const currentInfuseOffset = useMemo(() => {
        const name = currentInfusionName || 'Standard';
        return infuseTypes.find(t => t.name === name)?.offset ?? 0;
    }, [infuseTypes, currentInfusionName]);
    const [selectedInfuseOffset, setSelectedInfuseOffset] = useState<number>(0);

    useEffect(() => {
        GetInfuseTypes().then(types => {
            setInfuseTypes(types ?? []);
        }).catch(() => setInfuseTypes([]));
    }, []);

    useEffect(() => {
        setSelectedInfuseOffset(currentInfuseOffset);
    }, [currentInfuseOffset]);

    // ─── Ash of War state ─────────────────────────────────────────────────────
    const [ashesOfWar, setAshesOfWar] = useState<AshOfWarOption[]>([]);
    const [aowAvailability, setAowAvailability] = useState<Map<number, AoWAvailabilityEntry>>(new Map());
    const [currentAoWId, setCurrentAoWId] = useState<number>(0);
    const [canMountAoW, setCanMountAoW] = useState<boolean>(false);
    const [wepType, setWepType] = useState<number>(0);
    const [selectedAoW, setSelectedAoW] = useState<number | null>(null);
    const [aowSearch, setAowSearch] = useState('');
    const [showUnavailable, setShowUnavailable] = useState(false);

    // Load AoW item list (one-shot).
    useEffect(() => {
        GetItemList('ashes_of_war').then(items => {
            const list: AshOfWarOption[] = (items ?? []).map((it: any) => ({
                id: it.id,
                name: it.name,
                iconPath: it.iconPath ?? '',
                aowCompatBitmask: it.aowCompatBitmask ?? 0,
            })).sort((a, b) => a.name.localeCompare(b.name));
            setAshesOfWar(list);
        }).catch(() => setAshesOfWar([]));
    }, []);

    // Refresh weapon state (currentAoWId/canMountAoW/wepType) from GetCharacter.
    const refreshWeaponState = useCallback(async () => {
        try {
            const char = await GetCharacter(charIndex);
            const all = [...(char?.inventory ?? []), ...(char?.storage ?? [])];
            const found: any = all.find((it: any) => it.handle === item.handle);
            if (found) {
                setCurrentAoWId((found.aowId as number) || 0);
                setCanMountAoW(found.canMountAoW === true);
                setWepType((found.wepType as number) ?? 0);
            }
        } catch {
            // leave existing state
        }
    }, [charIndex, item.handle]);

    const refreshAvailability = useCallback(async () => {
        try {
            const entries = await GetAoWAvailability(charIndex);
            const m = new Map<number, AoWAvailabilityEntry>();
            (entries ?? []).forEach((e: any) => m.set(e.itemId as number, e as AoWAvailabilityEntry));
            setAowAvailability(m);
        } catch {
            setAowAvailability(new Map());
        }
    }, [charIndex]);

    useEffect(() => {
        refreshWeaponState();
        refreshAvailability();
    }, [refreshWeaponState, refreshAvailability]);

    // ─── Derived ──────────────────────────────────────────────────────────────
    // NOTE: fail-closed on unknown compatibility. This modal does NOT pass through
    // 'unknown' compat (unlike WeaponEditTab on this branch) because there is no
    // backend compatibility API on this branch beyond ApplyWeaponAoWStrict's
    // existing guard, and ApplyWeaponAoW(Strict) silently allows unknown when
    // CanWeaponMountAoW==true. Pending merge of research/aow-weapon-compatibility,
    // we treat unknown as blocked here for safety.

    const getStatus = (aowId: number): AoWStatus => {
        if (currentAoWId !== 0 && aowId === currentAoWId) return 'current';
        const avail = aowAvailability.get(aowId);
        if (!avail) return 'missing';
        if (avail.hasSharedHandleConflict) return 'conflict';
        if (avail.availableCopies > 0) return 'available';
        if (avail.usedCopies > 0) return 'in_use';
        return 'missing';
    };

    const filteredAoW = useMemo(() => {
        const q = aowSearch.trim().toLowerCase();
        return ashesOfWar.filter(a => {
            if (q && !a.name.toLowerCase().includes(q)) return false;
            if (!showUnavailable) {
                const status = getStatus(a.id);
                if (status !== 'available' && status !== 'current') return false;
                const compat = getAoWCompatStatus(a.aowCompatBitmask, wepType);
                // Fail-closed default view: hide incompatible AND unknown.
                if (compat !== 'compatible') return false;
            }
            return true;
        });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [ashesOfWar, aowSearch, showUnavailable, wepType, currentAoWId, aowAvailability]);

    const currentAoWName = useMemo(() => {
        if (currentAoWId === 0) return null;
        return ashesOfWar.find(a => a.id === currentAoWId)?.name ?? null;
    }, [ashesOfWar, currentAoWId]);

    const selectedAoWEntry = selectedAoW !== null && selectedAoW !== 0
        ? ashesOfWar.find(a => a.id === selectedAoW) ?? null
        : null;
    const selectedAoWStatus: AoWStatus | null =
        selectedAoW !== null && selectedAoW !== 0 ? getStatus(selectedAoW) : null;
    const selectedAoWCompat: AoWCompatStatus | null =
        selectedAoWEntry ? getAoWCompatStatus(selectedAoWEntry.aowCompatBitmask, wepType) : null;

    const aowChanged = selectedAoW !== null && selectedAoW !== currentAoWId;
    // Remove (selectedAoW===0) is always allowed when there is a current AoW.
    // Assign requires compat === 'compatible' AND a free copy. 'unknown' is
    // blocked regardless of wepType mapping (fail-closed; see note above).
    const canApplyAoW = canMountAoW
        && aowChanged
        && !applying
        && (selectedAoW === 0
            || (selectedAoWStatus === 'available' && selectedAoWCompat === 'compatible'));

    const canRemoveAoW = canMountAoW && currentAoWId !== 0 && !applying;

    // ─── Level / Infusion gates ────────────────────────────────────────────────
    const canEditLevel = maxUpgrade > 0;
    const levelChanged = selectedLevel !== currentLevel;
    const levelInRange = selectedLevel >= 0 && selectedLevel <= maxUpgrade;
    const canApplyLevel = canEditLevel && levelChanged && levelInRange && !applying;

    const canEditInfusion = maxUpgrade === 25;
    const infusionChanged = selectedInfuseOffset !== currentInfuseOffset;
    const canApplyInfusion =
        canEditInfusion && infusionChanged && infuseTypes.length > 0 && !applying;

    // ─── Display helpers ──────────────────────────────────────────────────────
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') onClose();
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [onClose]);

    const levelOptions = useMemo(() => {
        if (maxUpgrade === 0) return [];
        const arr: number[] = [];
        for (let i = 0; i <= maxUpgrade; i++) arr.push(i);
        return arr;
    }, [maxUpgrade]);

    const showIcon = !!item.iconPath && !imgError;
    const itemIdHex = `0x${currentItemId.toString(16).toUpperCase().padStart(8, '0')}`;
    const handleHex = `0x${item.handle.toString(16).toUpperCase().padStart(8, '0')}`;
    const upgradeLabel =
        currentLevel > 0
            ? currentInfusionName
                ? `${currentInfusionName} +${currentLevel}`
                : `+${currentLevel}`
            : currentInfusionName || '+0';

    // ─── Apply handlers ────────────────────────────────────────────────────────
    const onApplyLevel = () => {
        if (!canApplyLevel) return;
        setApplying(true);
        setError(null);
        setSuccess(null);
        const newItemId = currentItemId - currentLevel + selectedLevel;
        const expectedCurrentItemId = currentItemId;
        ApplyWeaponUpgradeLevel(charIndex, item.handle, expectedCurrentItemId, newItemId)
            .then(() => {
                setCurrentItemId(newItemId);
                setCurrentLevel(selectedLevel);
                setSuccess(`Level updated to +${selectedLevel}`);
                onApplied?.({
                    handle: item.handle,
                    itemId: newItemId,
                    currentUpgrade: selectedLevel,
                    infusionName: currentInfusionName,
                });
            })
            .catch((e: unknown) => {
                const msg = e instanceof Error ? e.message : String(e);
                setError(msg || 'Failed to apply level change');
            })
            .finally(() => setApplying(false));
    };

    const onApplyInfusion = () => {
        if (!canApplyInfusion) return;
        setApplying(true);
        setError(null);
        setSuccess(null);
        const newItemId = currentItemId - currentInfuseOffset + selectedInfuseOffset;
        const expectedCurrentItemId = currentItemId;
        const newName = infuseTypes.find(t => t.offset === selectedInfuseOffset)?.name ?? 'Standard';
        ApplyWeaponInfusion(charIndex, item.handle, expectedCurrentItemId, newItemId)
            .then(() => {
                setCurrentItemId(newItemId);
                const storedName = newName === 'Standard' ? '' : newName;
                setCurrentInfusionName(storedName);
                setSuccess(`Infusion updated to ${newName}`);
                onApplied?.({
                    handle: item.handle,
                    itemId: newItemId,
                    currentUpgrade: currentLevel,
                    infusionName: storedName,
                });
            })
            .catch((e: unknown) => {
                const msg = e instanceof Error ? e.message : String(e);
                setError(msg || 'Failed to apply infusion change');
            })
            .finally(() => setApplying(false));
    };

    const applyAoW = (newAoWItemID: number, label: string) => {
        setApplying(true);
        setError(null);
        setSuccess(null);
        ApplyWeaponAoWStrict(charIndex, item.handle, newAoWItemID)
            .then(async () => {
                await Promise.all([refreshWeaponState(), refreshAvailability()]);
                setSelectedAoW(null);
                setSuccess(label);
                // Save mutated — propagate to SortOrderTab so onMutate fires.
                // Tile metadata (itemId / level / infusion) does not change, so
                // we pass the current values; merge is a logical no-op.
                onApplied?.({
                    handle: item.handle,
                    itemId: currentItemId,
                    currentUpgrade: currentLevel,
                    infusionName: currentInfusionName,
                });
            })
            .catch((e: unknown) => {
                const msg = e instanceof Error ? e.message : String(e);
                setError(msg || 'Failed to apply Ash of War change');
            })
            .finally(() => setApplying(false));
    };

    const onApplyAoW = () => {
        if (!canApplyAoW || selectedAoW === null) return;
        const name = selectedAoW === 0
            ? 'none'
            : ashesOfWar.find(a => a.id === selectedAoW)?.name ?? 'Ash of War';
        applyAoW(selectedAoW, `Ash of War updated to ${name}`);
    };

    const onRemoveAoW = () => {
        if (!canRemoveAoW) return;
        applyAoW(0, 'Ash of War removed');
    };

    const statusBadge = (status: AoWStatus) => {
        const map: Record<AoWStatus, { label: string; cls: string }> = {
            current: { label: 'Current', cls: 'bg-amber-500/15 text-amber-300 border-amber-500/30' },
            available: { label: 'Available', cls: 'bg-green-500/15 text-green-300 border-green-500/30' },
            in_use: { label: 'In use', cls: 'bg-orange-500/15 text-orange-300 border-orange-500/30' },
            missing: { label: 'Missing', cls: 'bg-muted/30 text-muted-foreground border-border/40' },
            conflict: { label: 'Conflict', cls: 'bg-red-500/15 text-red-400 border-red-500/30' },
        };
        const m = map[status];
        return (
            <span className={`text-[7.5px] font-black uppercase tracking-wider border px-1 py-0.5 rounded ${m.cls}`}>
                {m.label}
            </span>
        );
    };

    const compatBadge = (compat: AoWCompatStatus) => {
        if (compat === 'compatible') return null;
        const map: Record<Exclude<AoWCompatStatus, 'compatible'>, { label: string; cls: string }> = {
            incompatible: { label: 'Incompatible', cls: 'bg-red-500/10 text-red-400/90 border-red-500/30' },
            unknown: { label: 'Unknown', cls: 'bg-muted/30 text-muted-foreground border-border/40' },
        };
        const m = map[compat];
        return (
            <span className={`text-[7.5px] font-black uppercase tracking-wider border px-1 py-0.5 rounded ${m.cls}`}>
                {m.label}
            </span>
        );
    };

    return (
        <div
            className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
            onClick={onClose}
        >
            <div
                className="w-full max-w-md bg-card border border-border/60 rounded-xl shadow-2xl max-h-[92vh] flex flex-col"
                onClick={(e) => e.stopPropagation()}
            >
                {/* Header */}
                <div className="flex items-start justify-between gap-3 p-4 border-b border-border/40 shrink-0">
                    <div className="flex items-center gap-3 min-w-0">
                        <div className="w-14 h-14 rounded-lg bg-muted/20 border border-border/50 flex items-center justify-center shrink-0 overflow-hidden">
                            {showIcon ? (
                                <img
                                    src={item.iconPath}
                                    alt=""
                                    className="w-full h-full object-contain p-1"
                                    onError={() => setImgError(true)}
                                />
                            ) : (
                                <span className="text-xl font-black text-muted-foreground/40 select-none">
                                    {item.name.charAt(0).toUpperCase()}
                                </span>
                            )}
                        </div>
                        <div className="min-w-0">
                            <h2 className="text-sm font-black uppercase tracking-wider text-foreground truncate">
                                {item.name}
                            </h2>
                            <div className="flex items-center flex-wrap gap-1.5 mt-1">
                                <span className="text-[9px] font-black text-primary bg-primary/10 border border-primary/20 px-1.5 py-0.5 rounded">
                                    {upgradeLabel}
                                </span>
                                {source === 'inventory' ? (
                                    <span className="text-[8px] font-black uppercase bg-blue-500/10 text-blue-500 border border-blue-500/20 px-1.5 py-0.5 rounded">
                                        Inventory
                                    </span>
                                ) : (
                                    <span className="text-[8px] font-black uppercase bg-muted/30 text-muted-foreground border border-border/30 px-1.5 py-0.5 rounded">
                                        Storage
                                    </span>
                                )}
                            </div>
                        </div>
                    </div>
                    <button
                        onClick={onClose}
                        title="Close (Esc)"
                        className="shrink-0 text-muted-foreground hover:text-foreground transition-colors p-1 rounded hover:bg-muted/30"
                    >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                {/* Body */}
                <div className="p-4 space-y-4 overflow-y-auto">
                    {/* Level edit section */}
                    <section className="rounded-lg border border-border/50 bg-muted/10 p-3 space-y-2">
                        <div className="flex items-center justify-between gap-2">
                            <span className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground">
                                Upgrade Level
                            </span>
                            {canEditLevel ? (
                                <span className="text-[9px] font-mono text-muted-foreground/70">
                                    +{currentLevel} / +{maxUpgrade}
                                </span>
                            ) : null}
                        </div>

                        {!canEditLevel ? (
                            <p className="text-[10px] text-muted-foreground/70 italic">
                                This weapon cannot be upgraded.
                            </p>
                        ) : (
                            <div className="flex items-center gap-2">
                                <select
                                    value={selectedLevel}
                                    onChange={(e) => setSelectedLevel(Number(e.target.value))}
                                    disabled={applying}
                                    className="flex-1 bg-background/60 border border-border/50 rounded-md px-2 py-1.5 text-[11px] font-mono focus:outline-none focus:ring-1 focus:ring-primary/30 focus:border-primary disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    {levelOptions.map(lvl => (
                                        <option key={lvl} value={lvl}>
                                            +{lvl}
                                        </option>
                                    ))}
                                </select>
                                <button
                                    onClick={onApplyLevel}
                                    disabled={!canApplyLevel}
                                    className={`px-3 py-1.5 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                        canApplyLevel
                                            ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                            : 'opacity-40 cursor-not-allowed bg-muted/30 text-muted-foreground'
                                    }`}
                                    title={
                                        !canEditLevel
                                            ? 'Weapon cannot be upgraded'
                                            : !levelChanged
                                              ? 'No level change'
                                              : applying
                                                ? 'Applying…'
                                                : 'Apply new upgrade level'
                                    }
                                >
                                    {applying ? 'Applying…' : 'Apply Level'}
                                </button>
                            </div>
                        )}
                    </section>

                    {/* Infusion edit section */}
                    <section className="rounded-lg border border-border/50 bg-muted/10 p-3 space-y-2">
                        <div className="flex items-center justify-between gap-2">
                            <span className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground">
                                Infusion
                            </span>
                            {canEditInfusion && (
                                <span className="text-[9px] font-mono text-muted-foreground/70">
                                    {currentInfusionName || 'Standard'}
                                </span>
                            )}
                        </div>

                        {!canEditInfusion ? (
                            <p className="text-[10px] text-muted-foreground/70 italic">
                                This weapon does not support infusion changes.
                            </p>
                        ) : (
                            <div className="flex items-center gap-2">
                                <select
                                    value={selectedInfuseOffset}
                                    onChange={(e) => setSelectedInfuseOffset(Number(e.target.value))}
                                    disabled={applying || infuseTypes.length === 0}
                                    className="flex-1 bg-background/60 border border-border/50 rounded-md px-2 py-1.5 text-[11px] font-mono focus:outline-none focus:ring-1 focus:ring-primary/30 focus:border-primary disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    {infuseTypes.map(t => (
                                        <option key={t.offset} value={t.offset}>
                                            {t.name}
                                        </option>
                                    ))}
                                </select>
                                <button
                                    onClick={onApplyInfusion}
                                    disabled={!canApplyInfusion}
                                    className={`px-3 py-1.5 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                        canApplyInfusion
                                            ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                            : 'opacity-40 cursor-not-allowed bg-muted/30 text-muted-foreground'
                                    }`}
                                    title={
                                        !canEditInfusion
                                            ? 'Weapon does not support infusion'
                                            : !infusionChanged
                                              ? 'No infusion change'
                                              : applying
                                                ? 'Applying…'
                                                : 'Apply new infusion'
                                    }
                                >
                                    {applying ? 'Applying…' : 'Apply Infusion'}
                                </button>
                            </div>
                        )}
                    </section>

                    {/* Ash of War edit section */}
                    <section className="rounded-lg border border-border/50 bg-muted/10 p-3 space-y-2">
                        <div className="flex items-center justify-between gap-2">
                            <span className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground">
                                Ash of War
                            </span>
                            {canMountAoW && (
                                <span className="text-[9px] font-mono text-muted-foreground/70 truncate ml-2">
                                    {currentAoWId === 0
                                        ? 'None'
                                        : currentAoWName ?? `Unknown (0x${currentAoWId.toString(16).toUpperCase()})`}
                                </span>
                            )}
                        </div>

                        {!canMountAoW ? (
                            <p className="text-[10px] text-muted-foreground/70 italic">
                                This weapon does not support Ash of War changes.
                            </p>
                        ) : (
                            <>
                                <div className="flex items-center gap-2">
                                    <div className="relative flex-1">
                                        <svg className="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-muted-foreground/50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                                        </svg>
                                        <input
                                            type="text"
                                            placeholder="Search Ashes of War..."
                                            value={aowSearch}
                                            onChange={(e) => setAowSearch(e.target.value)}
                                            disabled={applying}
                                            className="w-full bg-background/60 border border-border/50 rounded-md pl-7 pr-2 py-1.5 text-[10px] focus:outline-none focus:ring-1 focus:ring-primary/30 focus:border-primary disabled:opacity-50"
                                        />
                                    </div>
                                    <button
                                        onClick={onRemoveAoW}
                                        disabled={!canRemoveAoW}
                                        title={canRemoveAoW ? 'Remove current Ash of War' : 'No Ash of War to remove'}
                                        className={`px-2.5 py-1.5 text-[9px] font-black uppercase tracking-wider rounded border transition-all ${
                                            canRemoveAoW
                                                ? 'text-red-300 bg-red-500/10 border-red-500/30 hover:bg-red-500/20'
                                                : 'opacity-40 cursor-not-allowed text-muted-foreground bg-muted/20 border-border/30'
                                        }`}
                                    >
                                        Remove
                                    </button>
                                </div>

                                <label className="flex items-center gap-1.5 text-[9px] text-muted-foreground/80 cursor-pointer select-none">
                                    <input
                                        type="checkbox"
                                        checked={showUnavailable}
                                        onChange={(e) => setShowUnavailable(e.target.checked)}
                                        className="accent-primary"
                                    />
                                    Show unavailable / incompatible
                                </label>

                                <div className="max-h-56 overflow-y-auto rounded border border-border/40 bg-background/30 divide-y divide-border/30">
                                    {ashesOfWar.length === 0 ? (
                                        <p className="text-[10px] text-muted-foreground/60 italic p-3 text-center">Loading…</p>
                                    ) : filteredAoW.length === 0 ? (
                                        <p className="text-[10px] text-muted-foreground/60 italic p-3 text-center">
                                            {aowSearch ? 'No matching Ashes of War.' : 'No compatible Ashes of War available.'}
                                        </p>
                                    ) : (
                                        filteredAoW.map(aow => {
                                            const status = getStatus(aow.id);
                                            const compat = getAoWCompatStatus(aow.aowCompatBitmask, wepType);
                                            const isSelected = selectedAoW === aow.id;
                                            const selectable = status === 'available' || status === 'current';
                                            return (
                                                <button
                                                    key={aow.id}
                                                    type="button"
                                                    disabled={applying}
                                                    onClick={() => setSelectedAoW(isSelected ? null : aow.id)}
                                                    className={`w-full flex items-center gap-2 px-2 py-1.5 text-left transition-colors ${
                                                        isSelected
                                                            ? 'bg-primary/15'
                                                            : selectable
                                                              ? 'hover:bg-muted/30'
                                                              : 'opacity-60 hover:bg-muted/20'
                                                    }`}
                                                >
                                                    <div className="w-6 h-6 rounded bg-muted/30 border border-border/40 shrink-0 flex items-center justify-center overflow-hidden">
                                                        {aow.iconPath ? (
                                                            <img src={aow.iconPath} alt="" className="w-full h-full object-contain" />
                                                        ) : (
                                                            <span className="text-[9px] text-muted-foreground/40">{aow.name.charAt(0)}</span>
                                                        )}
                                                    </div>
                                                    <span className="flex-1 min-w-0 text-[10px] font-bold text-foreground/85 truncate">
                                                        {aow.name}
                                                    </span>
                                                    <div className="shrink-0 flex items-center gap-1">
                                                        {compatBadge(compat)}
                                                        {statusBadge(status)}
                                                    </div>
                                                </button>
                                            );
                                        })
                                    )}
                                </div>

                                {selectedAoW !== null && selectedAoW !== 0 && selectedAoWCompat === 'unknown' && (
                                    <p className="text-[9px] text-muted-foreground/80 italic leading-snug">
                                        Unknown compatibility data — blocked for safety. Will be unblocked once weapon AoW compatibility API is merged.
                                    </p>
                                )}
                                {selectedAoW !== null && selectedAoW !== 0 && selectedAoWCompat === 'incompatible' && (
                                    <p className="text-[9px] text-red-400/85 italic leading-snug">
                                        This Ash of War is not compatible with this weapon type.
                                    </p>
                                )}
                                {selectedAoW !== null && selectedAoW !== 0 && selectedAoWStatus !== 'available' && selectedAoWStatus !== 'current' && (
                                    <p className="text-[9px] text-muted-foreground/80 italic leading-snug">
                                        No free copy of this Ash of War is available in the save.
                                    </p>
                                )}

                                <button
                                    onClick={onApplyAoW}
                                    disabled={!canApplyAoW}
                                    title={
                                        !canMountAoW
                                            ? 'Weapon does not support Ash of War'
                                            : !aowChanged
                                              ? 'No Ash of War change'
                                              : selectedAoW !== 0 && selectedAoWCompat !== 'compatible'
                                                ? selectedAoWCompat === 'unknown'
                                                    ? 'Unknown compatibility — blocked for safety'
                                                    : 'Incompatible Ash of War'
                                                : selectedAoW !== 0 && selectedAoWStatus !== 'available'
                                                  ? 'No free copy of this Ash of War'
                                                  : applying
                                                    ? 'Applying…'
                                                    : 'Apply new Ash of War'
                                    }
                                    className={`w-full px-3 py-1.5 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                        canApplyAoW
                                            ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                            : 'opacity-40 cursor-not-allowed bg-muted/30 text-muted-foreground'
                                    }`}
                                >
                                    {applying ? 'Applying…' : 'Apply Ash of War'}
                                </button>
                            </>
                        )}
                    </section>

                    {error && (
                        <p className="text-[10px] font-mono text-red-400/90 leading-snug break-words">
                            {error}
                        </p>
                    )}
                    {success && !error && (
                        <p className="text-[10px] font-bold text-green-400/90">
                            {success}
                        </p>
                    )}

                    {/* Metadata */}
                    <dl className="grid grid-cols-[110px_1fr] gap-y-1.5 gap-x-3 text-[10px]">
                        <dt className="font-black uppercase tracking-wider text-muted-foreground">Character</dt>
                        <dd className="font-mono text-foreground/80">Slot {charIndex}</dd>

                        <dt className="font-black uppercase tracking-wider text-muted-foreground">Handle</dt>
                        <dd className="font-mono text-foreground/80">{handleHex}</dd>

                        <dt className="font-black uppercase tracking-wider text-muted-foreground">Item ID</dt>
                        <dd className="font-mono text-foreground/80">{itemIdHex}</dd>

                        <dt className="font-black uppercase tracking-wider text-muted-foreground">Infusion</dt>
                        <dd className="font-mono text-foreground/80">
                            {currentInfusionName || 'Standard'}
                        </dd>
                    </dl>
                </div>

                {/* Footer */}
                <div className="flex items-center justify-end gap-2 p-3 border-t border-border/40 shrink-0">
                    <button
                        onClick={onClose}
                        className="px-3 py-1.5 text-[10px] font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                    >
                        Close
                    </button>
                </div>
            </div>
        </div>
    );
}
