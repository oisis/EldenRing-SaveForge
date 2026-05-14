import { useEffect, useMemo, useState } from 'react';
import {
    ApplyWeaponInfusion,
    ApplyWeaponUpgradeLevel,
    GetInfuseTypes,
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

    // Keep selected infusion in sync when infuseTypes load or current changes (e.g. after Apply).
    useEffect(() => {
        setSelectedInfuseOffset(currentInfuseOffset);
    }, [currentInfuseOffset]);

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

    const canEditLevel = maxUpgrade > 0;
    const levelChanged = selectedLevel !== currentLevel;
    const levelInRange = selectedLevel >= 0 && selectedLevel <= maxUpgrade;
    const canApplyLevel = canEditLevel && levelChanged && levelInRange && !applying;

    // Backend ApplyWeaponInfusion requires baseData.MaxUpgrade == 25.
    // Mirror that gate so the dropdown is disabled for somber/unique weapons.
    const canEditInfusion = maxUpgrade === 25;
    const infusionChanged = selectedInfuseOffset !== currentInfuseOffset;
    const canApplyInfusion =
        canEditInfusion && infusionChanged && infuseTypes.length > 0 && !applying;

    const showIcon = !!item.iconPath && !imgError;
    const itemIdHex = `0x${currentItemId.toString(16).toUpperCase().padStart(8, '0')}`;
    const handleHex = `0x${item.handle.toString(16).toUpperCase().padStart(8, '0')}`;
    const upgradeLabel =
        currentLevel > 0
            ? currentInfusionName
                ? `${currentInfusionName} +${currentLevel}`
                : `+${currentLevel}`
            : currentInfusionName || '+0';

    const onApplyLevel = () => {
        if (!canApplyLevel) return;
        setApplying(true);
        setError(null);
        setSuccess(null);
        // newItemID = itemId - currentLevel + selectedLevel keeps baseID and
        // infusionOffset intact; backend re-validates this invariant.
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
        // newItemID = itemId - currentInfuseOffset + selectedInfuseOffset
        // Preserves baseID and upgrade level. Backend re-validates that only
        // the infusion offset changed (same baseID, same level).
        const newItemId = currentItemId - currentInfuseOffset + selectedInfuseOffset;
        const expectedCurrentItemId = currentItemId;
        const newName = infuseTypes.find(t => t.offset === selectedInfuseOffset)?.name ?? 'Standard';
        ApplyWeaponInfusion(charIndex, item.handle, expectedCurrentItemId, newItemId)
            .then(() => {
                setCurrentItemId(newItemId);
                // Backend / inventory_order convention: empty string for Standard.
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

    return (
        <div
            className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
            onClick={onClose}
        >
            <div
                className="w-full max-w-md bg-card border border-border/60 rounded-xl shadow-2xl"
                onClick={(e) => e.stopPropagation()}
            >
                {/* Header */}
                <div className="flex items-start justify-between gap-3 p-4 border-b border-border/40">
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
                <div className="p-4 space-y-4">
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

                    {/* Phase placeholder */}
                    <div className="rounded-lg border border-dashed border-border/50 bg-muted/10 p-3">
                        <p className="text-[10px] font-black uppercase tracking-wider text-muted-foreground/80">
                            Coming next
                        </p>
                        <ul className="mt-1.5 space-y-0.5 text-[10px] text-muted-foreground/70 list-disc list-inside">
                            <li>Ash of War picker with search and compatibility</li>
                        </ul>
                    </div>
                </div>

                {/* Footer */}
                <div className="flex items-center justify-end gap-2 p-3 border-t border-border/40">
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
