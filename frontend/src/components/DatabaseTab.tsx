import {useEffect, useState, useMemo, useRef, useDeferredValue, useCallback} from 'react';
import toast from '../lib/toast';
import {useVirtualizer} from '@tanstack/react-virtual';
import {GetItemList, GetItemListChunk, AddItemsToCharacter, AddItemsToCharacterWithGameLimits, GetCharacter,
        SetCookbookUnlocked, SetWhetbladeUnlocked, SetBellBearingUnlocked, GetBellBearings,
        SetMapRegionFlags, GetSlotCapacity} from '../../wailsjs/go/main/App';
import {db, vm} from '../../wailsjs/go/models';
import type {AddSettings} from '../App';
import {CategorySelect, CATEGORY_VALUES} from './CategorySelect';
import {RiskBadge} from './RiskBadge';
import {isLowerTierTalisman} from '../lib/talismanFamilies';
import {useFavorites} from '../state/favorites';
import {loadSafetyProfile, usesTechnicalCaps, SAFETY_PROFILE_EVENT, type SafetyProfile} from '../state/safetyProfile';
import {ItemDetailPanel} from './ItemDetailPanel';
import {BanRiskWarningModal} from './database/BanRiskWarningModal';
import {ErrorModal} from './database/ErrorModal';

// Categories whose tab has sub-groupings — drives the Sub-Category column visibility.
const CATEGORIES_WITH_SUBGROUPS = new Set([
    'tools', 'bolstering_materials', 'key_items',
    'melee_armaments', 'ranged_and_catalysts', 'arrows_and_bolts', 'shields', 'info',
]);

const CATEGORY_LABEL: Record<string, string> = {
    tools: 'Tools', ashes: 'Ashes', crafting_materials: 'Crafting Materials',
    bolstering_materials: 'Bolstering Materials', key_items: 'Key Items',
    sorceries: 'Sorceries', incantations: 'Incantations', ashes_of_war: 'Ashes of War',
    melee_armaments: 'Melee Armaments', ranged_and_catalysts: 'Ranged Weapons / Catalysts',
    arrows_and_bolts: 'Arrows / Bolts', shields: 'Shields',
    head: 'Head', chest: 'Chest', arms: 'Arms', legs: 'Legs',
    talismans: 'Talismans', info: 'Info',
};

interface DatabaseTabProps {
    columnVisibility: {
        id: boolean;
        category: boolean;
    };
    platform: string | null;
    charIndex: number;
    inventoryVersion: number;
    onItemsAdded?: () => void;
    addSettings: AddSettings;
    showFlaggedItems: boolean;
    category: string;
    setCategory: (value: string) => void;
    onSelectItem?: (item: db.ItemEntry | null) => void;
    selectedDetailItem?: db.ItemEntry | null;
    onCloseDetail?: () => void;
    readOnly?: boolean;
    showOnlyFavorites?: boolean;
    onToggleFavorites?: () => void;
    // Invoked when the user accepts the GaItem optimization CTA on a
    // gaitem_full failure. Eligibility/recovery are decided by the backend.
    onOptimizeGaItem?: (ctx: { neededGaItems: number }) => void;
}

// Determine if ALL selected items are non-stackable (max qty == 1)
function allNonStackable(items: db.ItemEntry[], clearCount: number, chaos: boolean): boolean {
    return items.every(i =>
        effectiveCap(i, 'inv', clearCount, chaos) <= 1 &&
        effectiveCap(i, 'storage', clearCount, chaos) <= 1);
}

// Normal Mode uses conservative authored caps (including NG+ scaling). Chaos
// Mode uses regulation-derived technical game limits where known and falls back
// to the conservative cap when the regulation limit is unavailable.
export function effectiveCap(item: db.ItemEntry, kind: 'inv' | 'storage', clearCount: number, chaos: boolean): number {
    if (chaos) {
        if (kind === 'inv' && item.gameMaxInventoryKnown) return item.gameMaxInventory;
        if (kind === 'storage' && item.gameMaxStorageKnown) return item.gameMaxStorage;
    }
    const base = kind === 'inv' ? item.maxInventory : item.maxStorage;
    if (item.flags?.includes('scales_with_ng')) return base * (clearCount + 1);
    return base;
}

export function DatabaseTab({columnVisibility, platform, charIndex, inventoryVersion, onItemsAdded, addSettings, showFlaggedItems, category, setCategory, onSelectItem, selectedDetailItem, onCloseDetail, readOnly = false, showOnlyFavorites = false, onToggleFavorites, onOptimizeGaItem}: DatabaseTabProps) {
    const {upgrade25, upgrade10, infuseOffset, upgradeAsh} = addSettings;
    const {isFav, toggle: toggleFav} = useFavorites();
    const [search, setSearch] = useState('');
    const [subCategory, setSubCategory] = useState('all');
    const [viewMode, setViewMode] = useState<'table' | 'grid'>('table');
    const [dbItems, setDbItems] = useState<db.ItemEntry[]>([]);
    const [loading, setLoading] = useState(false);

    // Sorting
    const [sortCol, setSortCol] = useState<string>('name');
    const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');

    // Selection
    const [selectedDbItems, setSelectedDbItems] = useState<Set<number>>(new Set());

    // Modal state
    const [confirmModal, setConfirmModal] = useState<db.ItemEntry[] | null>(null);
    const [banRiskWarning, setBanRiskWarning] = useState<db.ItemEntry[] | null>(null);
    const [ignoreBanRisk, setIgnoreBanRisk] = useState<boolean>(() => {
        return localStorage.getItem('setting:ignoreBanRiskWarning') === 'true';
    });
    const [isSaving, setIsSaving] = useState(false);
    const [brokenIcons, setBrokenIcons] = useState<Set<string>>(new Set());
    const [errorModal, setErrorModal] = useState<{title: string; message: string; cta?: {label: string; onClick: () => void}} | null>(null);

    // Quantity state for modal
    const [addToInv, setAddToInv] = useState(true);
    const [invMax, setInvMax] = useState(false);
    const [invQtyVal, setInvQtyVal] = useState(1);
    const [addToStorage, setAddToStorage] = useState(false);
    const [storageMax, setStorageMax] = useState(false);
    const [storageQtyVal, setStorageQtyVal] = useState(1);

    // Icon hover preview tooltip
    const [hoverTooltip, setHoverTooltip] = useState<{name: string, path: string, x: number, y: number} | null>(null);

    // Detail panel
    const [detailItem, setDetailItem] = useState<db.ItemEntry | null>(null);
    const activeDetail = selectedDetailItem ?? detailItem;
    const handleCloseDetail = () => {
        if (onCloseDetail) onCloseDetail();
        setDetailItem(null);
    };

    // Owned items (for "Inventory" / "Storage" columns)
    const [charInventory, setCharInventory] = useState<vm.ItemViewModel[]>([]);
    const [charStorage, setCharStorage] = useState<vm.ItemViewModel[]>([]);
    const [clearCount, setClearCount] = useState<number>(0);
    // Free inventory/storage slots (Max - Used). Ceiling for how many copies of a
    // non-stackable item can be added at once. Backend CheckAddCapacity is the hard
    // guard (all-or-nothing); these just pre-bound the input.
    const [freeInv, setFreeInv] = useState<number>(999);
    const [freeStorage, setFreeStorage] = useState<number>(999);

    // Safety profile drives cap mode + the add path. expanded_limits and chaos
    // both use technical/game caps; risky-item visibility comes from the
    // showFlaggedItems prop (chaos only). Cross-component sync via custom event.
    const [safetyProfile, setSafetyProfile] = useState<SafetyProfile>(() => loadSafetyProfile());
    const useTechnicalCapsMode = usesTechnicalCaps(safetyProfile);
    const isChaosProfile = safetyProfile === 'chaos';

    // Unlocked bell bearing flag IDs — used to show state in DB tab for flag-only items.
    const [unlockedFlagIds, setUnlockedFlagIds] = useState<Set<number>>(new Set());

    useEffect(() => {
        const handler = (e: Event) => setSafetyProfile((e as CustomEvent<SafetyProfile>).detail);
        window.addEventListener(SAFETY_PROFILE_EVENT, handler);
        return () => window.removeEventListener(SAFETY_PROFILE_EVENT, handler);
    }, []);

    useEffect(() => {
        GetCharacter(charIndex).then(res => {
            setCharInventory(res?.inventory || []);
            setCharStorage(res?.storage || []);
            setClearCount(res?.clearCount || 0);
        }).catch(() => {
            setCharInventory([]);
            setCharStorage([]);
            setClearCount(0);
        });
        GetSlotCapacity(charIndex)
            .then(cap => {
                setFreeInv(Math.max(1, (cap?.inventoryMax ?? 0) - (cap?.inventoryUsed ?? 0)));
                setFreeStorage(Math.max(1, (cap?.storageMax ?? 0) - (cap?.storageUsed ?? 0)));
            })
            .catch(() => { setFreeInv(999); setFreeStorage(999); });
    }, [charIndex, inventoryVersion]);

    useEffect(() => {
        GetBellBearings(charIndex).then(res => {
            setUnlockedFlagIds(new Set((res ?? []).filter(bb => bb.unlocked).map(bb => bb.id)));
        }).catch(() => {});
    }, [charIndex, inventoryVersion]);

    // Bell bearing item IDs whose shop flag is set — drives owned display for flag-only items.
    const bellBearingOwnedIds = useMemo(() =>
        new Set(
            dbItems
                .filter(i => i.unlockCategory === 'bell_bearing' && i.flagId != null && unlockedFlagIds.has(i.flagId))
                .map(i => i.id)
        ),
    [dbItems, unlockedFlagIds]);

    // Map family/base ID → {inv, storage} owned counts.
    // Stackable items contribute their stack quantity; non-stackable contribute one per copy.
    // BaseID groups weapon upgrade/infusion variants; familyId additionally groups
    // Crimson/Cerulean flask upgrade levels (separate DB rows) under their +0 picker row.
    const ownedByBaseID = useMemo(() => {
        const m = new Map<number, {inv: number; storage: number}>();
        const bump = (baseId: number, key: 'inv' | 'storage', n: number) => {
            const cur = m.get(baseId) ?? {inv: 0, storage: 0};
            cur[key] += n;
            m.set(baseId, cur);
        };
        charInventory.forEach(it => {
            // Goods are always quantity stacks, even when Safe mode intentionally
            // exposes a conservative 1/0 cap. Using the authored cap here would
            // turn a legitimate Expanded Limits stack (e.g. 99 Remembrances)
            // into a displayed count of 1.
            const qty = it.category === 'Item' || it.maxInventory > 1 || it.maxStorage > 1 ? it.quantity : 1;
            bump(it.familyId || it.baseId || it.id, 'inv', qty);
        });
        charStorage.forEach(it => {
            const qty = it.category === 'Item' || it.maxInventory > 1 || it.maxStorage > 1 ? it.quantity : 1;
            bump(it.familyId || it.baseId || it.id, 'storage', qty);
        });
        return m;
    }, [charInventory, charStorage]);

    // Owned inv/storage counts for a row — flag-only bell bearings resolve to a
    // 0/1 unlocked flag; everything else reads the family/base owned map. Shared
    // by the Inventory/Storage cells and their numeric sort.
    const resolveOwned = useCallback((item: db.ItemEntry) =>
        item.unlockCategory === 'bell_bearing'
            ? {inv: bellBearingOwnedIds.has(item.id) ? 1 : 0, storage: 0}
            : ownedByBaseID.get(item.id) ?? {inv: 0, storage: 0},
    [ownedByBaseID, bellBearingOwnedIds]);

    // Progressive loading: for "all", fetch one category per IPC roundtrip and
    // append as chunks arrive — the UI stays responsive on a 5000-item dataset.
    // For a single category, fall back to the original blocking call.
    const [chunkProgress, setChunkProgress] = useState<{ done: number; total: number } | null>(null);
    useEffect(() => {
        let cancelled = false;
        setSelectedDbItems(new Set());

        if (category !== 'all') {
            setLoading(true);
            setChunkProgress(null);
            GetItemList(category).then(res => {
                if (cancelled) return;
                setDbItems(res || []);
                setLoading(false);
            }).catch(err => {
                if (cancelled) return;
                console.error("Failed to load items:", err);
                setLoading(false);
            });
            return () => { cancelled = true; };
        }

        // Progressive 'all' load.
        setDbItems([]);
        setLoading(false); // no overlay — progress strip handles status
        setChunkProgress({ done: 0, total: CATEGORY_VALUES.length });
        (async () => {
            const accumulated: db.ItemEntry[] = [];
            for (let i = 0; i < CATEGORY_VALUES.length; i++) {
                if (cancelled) return;
                try {
                    const chunk = await GetItemListChunk(CATEGORY_VALUES[i]);
                    if (cancelled) return;
                    accumulated.push(...(chunk || []));
                    setDbItems([...accumulated]);
                } catch (err) {
                    console.error(`Failed to load chunk ${CATEGORY_VALUES[i]}:`, err);
                }
                setChunkProgress({ done: i + 1, total: CATEGORY_VALUES.length });
                // Yield to the event loop so React can flush + UI can scroll.
                await new Promise(r => setTimeout(r, 0));
            }
            if (!cancelled) setChunkProgress(null);
        })();
        return () => { cancelled = true; };
    }, [category]);

    const deferredSearch = useDeferredValue(search);

    // Reset the subcategory filter whenever the category changes.
    useEffect(() => { setSubCategory('all'); }, [category]);

    // Subcategories derived from the currently loaded items in the active
    // category scope — never a hardcoded list. Empty values are dropped.
    const availableSubCategories = useMemo(() => {
        const scope = category === 'all' ? dbItems : dbItems.filter(i => i.category === category);
        return Array.from(new Set(scope.map(i => i.subCategory).filter(Boolean))).sort();
    }, [dbItems, category]);
    const hasSubCategories = availableSubCategories.length > 0;

    const filteredItems = useMemo(() => dbItems.filter(item => {
        if (subCategory !== 'all' && item.subCategory !== subCategory) return false;
        if (showOnlyFavorites && !isFav(item.id)) return false;
        // "Cut & Ban-Risk" toggle hides only risky-flagged items, not informational flags
        // (dlc, stackable) which are now present on most entries.
        // Risky-item visibility is the showFlaggedItems prop (revealed only in the
        // chaos profile); expanded_limits keeps them hidden despite raised caps.
        const RISKY_FLAGS = ['cut_content', 'ban_risk', 'pre_order', 'dlc_duplicate'];
        if (!showFlaggedItems && item.flags?.some(f => RISKY_FLAGS.includes(f))) return false;
        // "Talismans: highest only" toggle hides lower-tier variants of upgrade families.
        if (category === 'talismans' && addSettings.talismansHighestOnly && isLowerTierTalisman(item.id)) return false;
        const q = deferredSearch.toLowerCase();
        if (!q) return true;
        return item.name.toLowerCase().includes(q) || item.id.toString(16).includes(q);
    }).sort((a, b) => {
        if (sortCol === 'weight') {
            const aW = a.weight, bW = b.weight;
            const aHas = aW !== undefined && aW > 0;
            const bHas = bW !== undefined && bW > 0;
            if (!aHas && !bHas) return a.name.localeCompare(b.name);
            if (!aHas) return -1;
            if (!bHas) return 1;
            const diff = sortDir === 'asc' ? aW! - bW! : bW! - aW!;
            return diff !== 0 ? diff : a.name.localeCompare(b.name);
        }
        // Numeric columns — sort by the value shown in the cell (max upgrade,
        // owned inventory/storage), name as the stable tiebreak.
        if (sortCol === 'maxUpgrade') {
            const diff = sortDir === 'asc' ? a.maxUpgrade - b.maxUpgrade : b.maxUpgrade - a.maxUpgrade;
            return diff !== 0 ? diff : a.name.localeCompare(b.name);
        }
        if (sortCol === 'ownedInv' || sortCol === 'ownedStorage') {
            const key = sortCol === 'ownedInv' ? 'inv' : 'storage';
            const aV = resolveOwned(a)[key], bV = resolveOwned(b)[key];
            const diff = sortDir === 'asc' ? aV - bV : bV - aV;
            return diff !== 0 ? diff : a.name.localeCompare(b.name);
        }
        const aVal = a[sortCol as keyof db.ItemEntry] ?? '';
        const bVal = b[sortCol as keyof db.ItemEntry] ?? '';
        if (aVal < bVal) return sortDir === 'asc' ? -1 : 1;
        if (aVal > bVal) return sortDir === 'asc' ? 1 : -1;
        return 0;
    }), [dbItems, deferredSearch, sortCol, sortDir, showFlaggedItems, useTechnicalCapsMode, category, subCategory, addSettings.talismansHighestOnly, showOnlyFavorites, isFav, resolveOwned]);

    const showWeightColumn = useMemo(() => filteredItems.some(i => i.weight !== undefined && i.weight > 0), [filteredItems]);

    // Owned count: items with at least 1 in inventory or storage in the current
    // category/subcategory scope. Deliberately derived from dbItems (not
    // filteredItems) so text search never affects it.
    const ownedCount = useMemo(() => {
        const matching = (category === 'all' ? dbItems : dbItems.filter(i => i.category === category))
            .filter(i => subCategory === 'all' || i.subCategory === subCategory);
        return matching.filter(i => {
            const o = ownedByBaseID.get(i.id);
            return !!o && (o.inv > 0 || o.storage > 0);
        }).length;
    }, [dbItems, ownedByBaseID, category, subCategory]);

    const favoritesInView = useMemo(() =>
        dbItems.filter(item => {
            if (subCategory !== 'all' && item.subCategory !== subCategory) return false;
            if (!isFav(item.id)) return false;
            const RISKY_FLAGS = ['cut_content', 'ban_risk', 'pre_order', 'dlc_duplicate'];
            if (!showFlaggedItems && item.flags?.some(f => RISKY_FLAGS.includes(f))) return false;
            if (category === 'talismans' && addSettings.talismansHighestOnly && isLowerTierTalisman(item.id)) return false;
            return true;
        }),
        [dbItems, isFav, showFlaggedItems, useTechnicalCapsMode, category, subCategory, addSettings.talismansHighestOnly]);

    // Sub-Category column: shown for grouped categories, but hidden when the
    // whole current table has no subcategory value ('all' shows category labels).
    const showSubGroupColumn = (category === 'all' || CATEGORIES_WITH_SUBGROUPS.has(category))
        && (category === 'all' || filteredItems.some(i => i.subCategory));

    const handleSort = (col: string) => {
        if (sortCol === col) setSortDir(sortDir === 'asc' ? 'desc' : 'asc');
        else { setSortCol(col); setSortDir('asc'); }
    };

    // A search is only a view filter. Keep selections across searches so a user
    // can collect items from different families and submit them as one atomic
    // add operation. The category-loading effect above still clears selection
    // when the underlying database scope changes.
    const selectedItems = useMemo(
        () => dbItems.filter(i => selectedDbItems.has(i.id)),
        [dbItems, selectedDbItems],
    );
    const selectedCount = selectedItems.length;

    const toggleItem = (id: number) => {
        const next = new Set(selectedDbItems);
        if (next.has(id)) next.delete(id); else next.add(id);
        setSelectedDbItems(next);
    };

    const toggleAll = () => {
        const allVisibleSelected = filteredItems.length > 0 && filteredItems.every(i => selectedDbItems.has(i.id));
        setSelectedDbItems(prev => {
            const next = new Set(prev);
            for (const item of filteredItems) {
                if (allVisibleSelected) next.delete(item.id);
                else next.add(item.id);
            }
            return next;
        });
    };

    const handleAdd = async () => {
        if (!confirmModal || isSaving) return;
        setIsSaving(true);
        try {
            const addItems = useTechnicalCapsMode ? AddItemsToCharacterWithGameLimits : AddItemsToCharacter;
            const baseIds = confirmModal.map(i => i.id);
            type AddRes = { added: number; requested: number; trimmed: { itemID: number; cutQty: number }[]; skippedExisting: { itemID: number; cutQty: number }[]; capHit: string; freeInv: number; freeStore: number; neededInv: number; neededStore: number; freeGaItems: number; neededGaItems: number;
                gaItemCapacity?: { physicalEmpty: number; cursorRoom: number; usable: number };
                gaItemRepackCTA?: { eligible: boolean; recovered: number } };
            let lastResult: AddRes | null = null;
            let totalAdded = 0;
            let totalRequested = 0;
            const allTrimmed: { itemID: number; cutQty: number }[] = [];
            const allSkippedExisting: { itemID: number; cutQty: number }[] = [];

            if (modalNonStackable) {
                const bothActive = addToInv && invQtyVal > 0 && addToStorage && storageQtyVal > 0;
                if (bothActive && invQtyVal === 1 && storageQtyVal === 1) {
                    const res = await addItems(charIndex, baseIds, upgrade25, upgrade10, infuseOffset, upgradeAsh, 1, 1) as AddRes;
                    lastResult = res;
                    totalAdded += res?.added ?? 0;
                    totalRequested += res?.requested ?? 0;
                    if (res?.trimmed) allTrimmed.push(...res.trimmed);
                    if (res?.skippedExisting) allSkippedExisting.push(...res.skippedExisting);
                } else {
                    if (addToInv && invQtyVal > 0) {
                        const ids = invQtyVal > 1
                            ? confirmModal.flatMap(i => Array<number>(invQtyVal).fill(i.id))
                            : baseIds;
                        const res = await addItems(charIndex, ids, upgrade25, upgrade10, infuseOffset, upgradeAsh, 1, 0) as AddRes;
                        lastResult = res;
                        totalAdded += res?.added ?? 0;
                        totalRequested += res?.requested ?? 0;
                        if (res?.trimmed) allTrimmed.push(...res.trimmed);
                        if (res?.skippedExisting) allSkippedExisting.push(...res.skippedExisting);
                    }
                    if (!lastResult?.capHit && addToStorage && storageQtyVal > 0) {
                        const ids = storageQtyVal > 1
                            ? confirmModal.flatMap(i => Array<number>(storageQtyVal).fill(i.id))
                            : baseIds;
                        const res = await addItems(charIndex, ids, upgrade25, upgrade10, infuseOffset, upgradeAsh, 0, 1) as AddRes;
                        lastResult = res;
                        totalAdded += res?.added ?? 0;
                        totalRequested += res?.requested ?? 0;
                        if (res?.trimmed) allTrimmed.push(...res.trimmed);
                        if (res?.skippedExisting) allSkippedExisting.push(...res.skippedExisting);
                    }
                }
            } else {
                const invQty = !addToInv ? 0 : invMax ? -1 : invQtyVal;
                const storQty = !addToStorage ? 0 : storageMax ? -1 : storageQtyVal;
                const res = await addItems(charIndex, baseIds, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storQty) as AddRes;
                lastResult = res;
                totalAdded += res?.added ?? 0;
                totalRequested += res?.requested ?? 0;
                if (res?.trimmed) allTrimmed.push(...res.trimmed);
                if (res?.skippedExisting) allSkippedExisting.push(...res.skippedExisting);
            }

            if (lastResult?.capHit) {
                const labels: Record<string, string> = {
                    inventory_full: 'Inventory Full',
                    storage_full: 'Storage Full',
                    gaitem_full: 'GaItem Slots Full',
                    gaitemdata_full: 'GaItemData Full',
                };
                const showInv = lastResult.neededInv > 0;
                const showStore = lastResult.neededStore > 0;
                const showGaItems = lastResult.capHit === 'gaitem_full' && lastResult.neededGaItems > 0;
                const invOverflow = Math.max(0, lastResult.neededInv - lastResult.freeInv);
                const storeOverflow = Math.max(0, lastResult.neededStore - lastResult.freeStore);
                const removals: string[] = [];
                if (invOverflow > 0) removals.push(`${invOverflow} item(s) from Inventory`);
                if (storeOverflow > 0) removals.push(`${storeOverflow} item(s) from Storage`);

                const cap = lastResult.gaItemCapacity;
                const parts: string[] = [];
                if (showInv) parts.push(`Inventory: need ${lastResult.neededInv}, free ${lastResult.freeInv}`);
                if (showStore) parts.push(`Storage: need ${lastResult.neededStore}, free ${lastResult.freeStore}`);
                if (showGaItems) {
                    // Prefer backend capacity metrics; usable is computed server-side.
                    if (cap) {
                        parts.push(`GaItem allocation: need ${lastResult.neededGaItems}, usable ${cap.usable} (physical empty ${cap.physicalEmpty}, cursor room ${cap.cursorRoom})`);
                        if (lastResult.neededGaItems > cap.physicalEmpty) {
                            parts.push('This batch needs more physical GaItem records than are empty; optimization cannot create records.');
                        }
                    } else {
                        parts.push(`GaItem allocation: need ${lastResult.neededGaItems}, free ${lastResult.freeGaItems}`);
                    }
                }
                if (removals.length > 0) parts.push(`\nRemove at least ${removals.join(' and ')} to make room.`);
                parts.push(`\n0 / ${lastResult.requested} items added.`);

                const neededGaItems = lastResult.neededGaItems;
                const showCta = lastResult.capHit === 'gaitem_full'
                    && lastResult.gaItemRepackCTA?.eligible === true
                    && !!onOptimizeGaItem;

                setConfirmModal(null);
                setErrorModal({
                    title: labels[lastResult.capHit] || 'Capacity Exceeded',
                    message: parts.join('\n'),
                    cta: showCta ? {
                        label: 'Review GaItem optimization',
                        onClick: () => {
                            setErrorModal(null);
                            onOptimizeGaItem!({ neededGaItems });
                        },
                    } : undefined,
                });
                return;
            }

            let msg = `Added ${totalAdded} / ${totalRequested} item(s) successfully.`;
            if (allTrimmed.length > 0) {
                const totalCut = allTrimmed.reduce((sum, s) => sum + s.cutQty, 0);
                const distinctItems = new Set(allTrimmed.map(s => s.itemID)).size;
                msg += ` ${distinctItems} pot/perfume type(s) trimmed by container cap (−${totalCut} units).`;
            }
            if (allSkippedExisting.length > 0) {
                const distinctItems = new Set(allSkippedExisting.map(s => s.itemID)).size;
                msg += ` ${distinctItems} item(s) skipped because already present in Key Items.`;
            }
            if (totalAdded === 0 && allSkippedExisting.length > 0) toast(msg);
            else toast.success(msg);
            setConfirmModal(null);
            setSelectedDbItems(new Set());
            if (totalAdded > 0) onItemsAdded?.();
        } catch (err) {
            setConfirmModal(null);
            setErrorModal({
                title: 'Add Failed',
                message: String(err),
            });
        } finally {
            setIsSaving(false);
        }
    };

    const openModal = (items: db.ItemEntry[]) => {
        const unlockItems = items.filter(i => i.unlockCategory);
        const normalItems = items.filter(i => !i.unlockCategory);
        if (unlockItems.length > 0) handleUnlockItems(unlockItems);
        if (normalItems.length > 0) {
            if (!ignoreBanRisk && normalItems.some(i => i.flags?.includes('ban_risk'))) {
                setBanRiskWarning(normalItems);
            } else {
                openConfirmModal(normalItems);
            }
        }
    };

    const openConfirmModal = (items: db.ItemEntry[]) => {
        setAddToInv(true);
        setInvMax(false);
        setInvQtyVal(1);
        // Storage on by default if at least one selected item allows storage (>0 cap).
        // Backend skips items with cap 0 per-item, so it's safe to enable storage even
        // when the selection is mixed (e.g. Glovewort + Sacred Flask).
        const anyStorageAllowed = items.some(i => effectiveCap(i, 'storage', clearCount, useTechnicalCapsMode) > 0);
        setAddToStorage(anyStorageAllowed);
        setStorageMax(false);
        setStorageQtyVal(1);
        setConfirmModal(items);
    };

    const handleIgnoreBanRiskChange = (checked: boolean) => {
        setIgnoreBanRisk(checked);
        localStorage.setItem('setting:ignoreBanRiskWarning', String(checked));
    };

    const handleUnlockItems = async (items: db.ItemEntry[]) => {
        try {
            const results = await Promise.all(items.map(item => {
                if (item.unlockCategory === 'cookbook') return SetCookbookUnlocked(charIndex, item.flagId!, true);
                if (item.unlockCategory === 'whetblade') return SetWhetbladeUnlocked(charIndex, item.flagId!, true);
                if (item.unlockCategory === 'bell_bearing') return SetBellBearingUnlocked(charIndex, item.flagId!, true);
                if (item.unlockCategory === 'map') return SetMapRegionFlags(charIndex, item.flagId!, true);
                return Promise.resolve();
            }));
            // SetWhetbladeUnlocked returns bool: true = was already unlocked/owned before this call
            const skipped = results.filter(r => r === true).length;
            const newlyUnlocked = items.length - skipped;
            if (newlyUnlocked > 0) {
                toast.success(`Unlocked ${newlyUnlocked} item(s).`);
            } else {
                toast(`${skipped} item(s) already unlocked — skipped.`);
            }
            setSelectedDbItems(new Set());
            if (newlyUnlocked > 0) onItemsAdded?.();
        } catch (err) {
            setErrorModal({title: 'Unlock Failed', message: String(err)});
        }
    };

    const handleImageError = (iconPath: string) => {
        setBrokenIcons(prev => new Set(prev).add(iconPath));
    };

    // "Max Up" column shows each item's maximum allowed upgrade (item identity),
    // independent of the add-modal's selected upgrade/infusion. Hidden when no
    // visible item is upgradeable.
    const showMaxUpColumn = useMemo(() => filteredItems.some(i => i.maxUpgrade > 0), [filteredItems]);

    const scrollRef = useRef<HTMLDivElement>(null);
    const rowVirtualizer = useVirtualizer({
        count: filteredItems.length,
        getScrollElement: () => scrollRef.current,
        estimateSize: () => 52,
        overscan: 20,
    });

    // Header has: (optional checkbox) + fav + icon + name (3 fixed) + optional ID + optional Sub-Category + Inv/Storage (always 2)
    const colCount = 5
        + (readOnly ? 0 : 1)
        + (columnVisibility.id ? 1 : 0)
        + (columnVisibility.category && showSubGroupColumn ? 1 : 0)
        + (showMaxUpColumn ? 1 : 0)
        + (showWeightColumn ? 1 : 0);

    // Whether the modal items are all non-stackable (weapons/armor/talismans)
    const modalNonStackable = confirmModal ? allNonStackable(confirmModal, clearCount, useTechnicalCapsMode) : true;
    // "Hi" caps = max effective cap in the selection. Used for input upper bounds and the Max checkbox label.
    // Backend clamps per item (resolveQty), so UI must expose the highest cap; ratcheting to the lowest
    // would prevent a Glovewort (cap 999) from receiving its full stack just because a Remembrance (cap 1)
    // is also selected. Items with cap 0 are skipped server-side.
    const modalMaxInvHi = confirmModal ? Math.max(...confirmModal.map(i => effectiveCap(i, 'inv', clearCount, useTechnicalCapsMode))) : 1;
    const modalMaxStorageHi = confirmModal ? Math.max(...confirmModal.map(i => effectiveCap(i, 'storage', clearCount, useTechnicalCapsMode))) : 1;
    const modalAnyStorageAllowed = !!confirmModal && confirmModal.some(i => effectiveCap(i, 'storage', clearCount, useTechnicalCapsMode) > 0);
    const modalMixedMaxes = confirmModal && confirmModal.length > 1 && !modalNonStackable &&
        (new Set(confirmModal.map(i => effectiveCap(i, 'inv', clearCount, useTechnicalCapsMode))).size > 1 ||
            new Set(confirmModal.map(i => effectiveCap(i, 'storage', clearCount, useTechnicalCapsMode))).size > 1);
    const modalHasUnknownGameLimits = !!confirmModal && confirmModal.some(i =>
        !i.gameMaxInventoryKnown || !i.gameMaxStorageKnown);
    // True if any selected item has scales_with_ng flag (drives tooltip rendering).
    const modalHasNgScaling = !!confirmModal && confirmModal.some(i => i.flags?.includes('scales_with_ng'));
    // Vanilla NG cap (clearCount=0) used to show "Vanilla NG: X / NG+Y: Z" tooltip.
    const modalVanillaInv = confirmModal ? Math.min(...confirmModal.map(i => i.maxInventory)) : 1;

    return (
        <div className="flex-1 flex flex-col min-h-0 space-y-3">
            {/* Ban-risk Warning Modal — gates confirmModal when any selected item has ban_risk flag */}
            {banRiskWarning && (
                <BanRiskWarningModal
                    items={banRiskWarning}
                    ignoreBanRisk={ignoreBanRisk}
                    onIgnoreChange={handleIgnoreBanRiskChange}
                    onCancel={() => setBanRiskWarning(null)}
                    onConfirm={() => {
                        const items = banRiskWarning;
                        setBanRiskWarning(null);
                        openConfirmModal(items);
                    }}
                />
            )}

            {/* Error Modal — capacity / container cap / add failure */}
            {errorModal && (
                <ErrorModal
                    title={errorModal.title}
                    message={errorModal.message}
                    cta={errorModal.cta}
                    onClose={() => setErrorModal(null)}
                />
            )}

            {/* Confirm Modal */}
            {confirmModal && (
                <div className="fixed inset-0 z-[110] flex items-center justify-center bg-background/80 backdrop-blur-sm animate-in fade-in duration-300">
                    <div className="bg-card p-8 rounded-2xl border border-primary/20 flex flex-col space-y-6 max-w-sm w-full mx-4 shadow-2xl shadow-primary/20 animate-in zoom-in-95 duration-300">
                        {/* Header */}
                        <div className="flex items-center space-x-4">
                            {confirmModal.length === 1 ? (
                                <>
                                    <div className="w-12 h-12 rounded bg-muted/30 border border-border/50 flex items-center justify-center overflow-hidden">
                                        {brokenIcons.has(confirmModal[0].iconPath) ? (
                                            <span className="text-[10px] font-black text-muted-foreground/30 select-none">?</span>
                                        ) : (
                                            <img src={confirmModal[0].iconPath} alt="" className="w-8 h-8 object-contain" onError={() => handleImageError(confirmModal[0].iconPath)} />
                                        )}
                                    </div>
                                    <div>
                                        <h3 className="text-sm font-black uppercase tracking-widest text-foreground">{confirmModal[0].name}</h3>
                                        <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">{confirmModal[0].category}</p>
                                    </div>
                                </>
                            ) : (
                                <>
                                    <div className="w-12 h-12 rounded bg-primary/10 border border-primary/30 flex items-center justify-center">
                                        <span className="text-lg font-black text-primary">{confirmModal.length}</span>
                                    </div>
                                    <div>
                                        <h3 className="text-sm font-black uppercase tracking-widest text-foreground">Add {confirmModal.length} Items</h3>
                                        <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Bulk Action</p>
                                    </div>
                                </>
                            )}
                        </div>

                        {/* Cap info banners */}
                        {isChaosProfile && (
                            <p className="text-[9px] font-black uppercase tracking-widest text-red-500 bg-red-500/10 border border-red-500/30 rounded px-3 py-1.5">
                                ⚠ Chaos Mode — technical game caps
                                {modalHasUnknownGameLimits && ' · conservative fallback where unknown'}
                            </p>
                        )}
                        {useTechnicalCapsMode && !isChaosProfile && (
                            <p className="text-[9px] font-black uppercase tracking-widest text-amber-500 bg-amber-500/10 border border-amber-500/30 rounded px-3 py-1.5">
                                Expanded Limits — technical game caps
                                {modalHasUnknownGameLimits && ' · conservative fallback where unknown'}
                            </p>
                        )}
                        {!useTechnicalCapsMode && modalHasNgScaling && (
                            <p className="text-[9px] font-bold text-primary/90 bg-primary/5 border border-primary/20 rounded px-3 py-1.5 leading-relaxed">
                                <span className="font-black uppercase tracking-widest">NG+ Scaling</span>
                                {' · '}Vanilla NG: <strong>{modalVanillaInv}</strong>
                                {clearCount > 0 && <> · NG+{clearCount}: <strong>{modalVanillaInv * (clearCount + 1)}</strong></>}
                                <span className="block text-muted-foreground mt-0.5">Adding more increases EAC ban risk.</span>
                            </p>
                        )}

                        {/* Inventory row */}
                        <div className="space-y-3">
                            <div className="flex items-center space-x-3">
                                <div
                                    onClick={() => setAddToInv(!addToInv)}
                                    className={`w-5 h-5 rounded border flex items-center justify-center transition-all cursor-pointer shrink-0 ${addToInv ? 'bg-primary border-primary' : 'bg-muted/30 border-border hover:border-primary/50'}`}
                                >
                                    {addToInv && <svg className="w-3.5 h-3.5 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="4" d="M5 13l4 4L19 7"/></svg>}
                                </div>
                                <span className="text-[11px] font-bold uppercase tracking-widest text-foreground/80 w-20 shrink-0">Inventory</span>
                                <input
                                    type="number"
                                    min={1}
                                    max={modalNonStackable ? freeInv : modalMaxInvHi}
                                    value={invMax ? modalMaxInvHi : invQtyVal}
                                    disabled={!addToInv || invMax}
                                    onChange={e => setInvQtyVal(Math.max(1, Math.min(modalNonStackable ? freeInv : modalMaxInvHi, parseInt(e.target.value) || 1)))}
                                    className="w-20 bg-background border border-border/50 rounded px-2 py-1 text-[10px] font-mono text-center focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all disabled:opacity-40"
                                />
                                {modalNonStackable && (
                                    <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Copies</span>
                                )}
                                {!modalNonStackable && modalMaxInvHi > 1 && (
                                    <div
                                        onClick={() => addToInv && setInvMax(!invMax)}
                                        className={`flex items-center space-x-1.5 cursor-pointer group ${!addToInv ? 'opacity-40 pointer-events-none' : ''}`}
                                        title={modalMixedMaxes ? 'Apply each item\'s own vanilla cap (NG+ scaled if applicable)' : `Cap: ${modalMaxInvHi}`}
                                    >
                                        <div className={`w-4 h-4 rounded border flex items-center justify-center transition-all ${invMax ? 'bg-primary border-primary' : 'bg-muted/30 border-border group-hover:border-primary/50'}`}>
                                            {invMax && <svg className="w-2.5 h-2.5 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="4" d="M5 13l4 4L19 7"/></svg>}
                                        </div>
                                        <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">{modalMixedMaxes ? 'Max (per item)' : `Max (${modalMaxInvHi})`}</span>
                                    </div>
                                )}
                            </div>

                            {/* Storage row — disabled only when NO selected item allows storage (every cap=0).
                                Mixed selection: enabled, backend skips items with cap 0 per-item. */}
                            <div className={`flex items-center space-x-3 ${!modalAnyStorageAllowed ? 'opacity-40 pointer-events-none' : ''}`}>
                                <div
                                    onClick={() => modalAnyStorageAllowed && setAddToStorage(!addToStorage)}
                                    className={`w-5 h-5 rounded border flex items-center justify-center transition-all cursor-pointer shrink-0 ${addToStorage && modalAnyStorageAllowed ? 'bg-primary border-primary' : 'bg-muted/30 border-border hover:border-primary/50'}`}
                                >
                                    {addToStorage && modalAnyStorageAllowed && <svg className="w-3.5 h-3.5 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="4" d="M5 13l4 4L19 7"/></svg>}
                                </div>
                                <span className="text-[11px] font-bold uppercase tracking-widest text-foreground/80 w-20 shrink-0">Storage</span>
                                <input
                                    type="number"
                                    min={1}
                                    max={modalNonStackable ? freeStorage : modalMaxStorageHi}
                                    value={!modalAnyStorageAllowed ? 0 : storageMax ? modalMaxStorageHi : storageQtyVal}
                                    disabled={!addToStorage || storageMax || !modalAnyStorageAllowed}
                                    onChange={e => setStorageQtyVal(Math.max(1, Math.min(modalNonStackable ? freeStorage : modalMaxStorageHi, parseInt(e.target.value) || 1)))}
                                    className="w-20 bg-background border border-border/50 rounded px-2 py-1 text-[10px] font-mono text-center focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all disabled:opacity-40"
                                />
                                {!modalAnyStorageAllowed && (
                                    <span className="text-[9px] font-black uppercase tracking-widest text-red-500/70">Not allowed</span>
                                )}
                                {modalAnyStorageAllowed && modalNonStackable && (
                                    <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Copies</span>
                                )}
                                {modalAnyStorageAllowed && !modalNonStackable && modalMaxStorageHi > 1 && (
                                    <div
                                        onClick={() => addToStorage && setStorageMax(!storageMax)}
                                        className={`flex items-center space-x-1.5 cursor-pointer group ${!addToStorage ? 'opacity-40 pointer-events-none' : ''}`}
                                        title={modalMixedMaxes ? 'Apply each item\'s own vanilla storage cap (skip items with cap 0)' : `Cap: ${modalMaxStorageHi}`}
                                    >
                                        <div className={`w-4 h-4 rounded border flex items-center justify-center transition-all ${storageMax ? 'bg-primary border-primary' : 'bg-muted/30 border-border group-hover:border-primary/50'}`}>
                                            {storageMax && <svg className="w-2.5 h-2.5 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="4" d="M5 13l4 4L19 7"/></svg>}
                                        </div>
                                        <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">{modalMixedMaxes ? 'Max (per item)' : `Max (${modalMaxStorageHi})`}</span>
                                    </div>
                                )}
                            </div>
                        </div>

                        {modalMixedMaxes && (
                            <p className="text-[9px] font-bold text-amber-500 bg-amber-500/10 border border-amber-500/20 rounded px-3 py-1.5">
                                Mixed caps in selection — backend applies each item's own vanilla cap (highest in group: Inv {modalMaxInvHi}{modalAnyStorageAllowed ? `, Storage ${modalMaxStorageHi}` : ''}). Items with cap 0 skipped.
                            </p>
                        )}

                        <div className="flex space-x-3 pt-2">
                            <button onClick={handleAdd} disabled={isSaving || (!addToInv && !addToStorage)} className="flex-1 px-4 py-2.5 bg-primary text-primary-foreground rounded-md text-[10px] font-black uppercase tracking-widest shadow-lg shadow-primary/20 hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-50">
                                {isSaving ? 'Adding...' : 'Add'}
                            </button>
                            <button onClick={() => setConfirmModal(null)} className="flex-1 px-4 py-2.5 bg-muted/30 text-muted-foreground rounded-md text-[10px] font-black uppercase tracking-widest border border-border hover:bg-muted/50 transition-all">
                                Cancel
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Top Bar: [Category] [Subcategory] [Owned badge] [buttons] [spacer] [view toggle] [Search] */}
            <div className="flex items-center gap-4 bg-muted/10 rounded-xl backdrop-blur-sm sticky top-0 z-20">
                <CategorySelect value={category} onChange={setCategory} className="w-56 shrink-0" />

                {/* Subcategory filter — disabled placeholder keeps the layout stable
                    when the active category has no subcategories. */}
                <div className="relative w-48 shrink-0">
                    <select
                        aria-label="Subcategory"
                        value={subCategory}
                        disabled={!hasSubCategories}
                        onChange={e => setSubCategory(e.target.value)}
                        className="w-full appearance-none bg-muted/30 border border-border rounded-md px-4 py-2.5 pr-10 text-[10px] font-black uppercase tracking-widest text-muted-foreground outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                        <option value="all">{hasSubCategories ? 'All Subcategories' : 'No Subcategories'}</option>
                        {availableSubCategories.map(s => (
                            <option key={s} value={s}>{s}</option>
                        ))}
                    </select>
                    <div className="absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none text-muted-foreground">
                        <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M19 9l-7 7-7-7"></path></svg>
                    </div>
                </div>

                <div className="flex items-center gap-2 px-3 py-2 rounded-md bg-muted/20 border border-border/50">
                    <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Owned:</span>
                    <span className="text-[10px] font-bold tabular-nums text-foreground">{ownedCount}</span>
                </div>

                {!readOnly && selectedCount > 0 && (
                    <button
                        onClick={() => openModal(selectedItems)}
                        disabled={!platform}
                        aria-label={`Add Selected (${selectedCount})`}
                        title={`Add Selected (${selectedCount})`}
                        className="px-5 py-2 bg-primary text-primary-foreground rounded-lg text-[9px] font-black uppercase tracking-[0.2em] shadow-xl shadow-primary/20 hover:brightness-110 active:scale-95 transition-all animate-in zoom-in-95 duration-300 disabled:opacity-50 disabled:grayscale disabled:cursor-not-allowed flex items-center gap-1.5"
                    >
                        <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="4" d="M5 13l4 4L19 7"/></svg>
                        Add ({selectedCount})
                    </button>
                )}
                {!readOnly && favoritesInView.length > 0 && selectedCount === 0 && (
                    <button
                        onClick={() => openModal(favoritesInView)}
                        disabled={!platform}
                        aria-label={`Add Favorites (${favoritesInView.length})`}
                        title={`Add Favorites (${favoritesInView.length})`}
                        className="px-5 py-2 bg-blue-900/80 text-white rounded-lg text-[9px] font-black uppercase tracking-[0.2em] shadow-xl shadow-blue-900/20 hover:brightness-110 active:scale-95 transition-all animate-in zoom-in-95 duration-300 disabled:opacity-50 disabled:grayscale disabled:cursor-not-allowed flex items-center gap-1.5"
                    >
                        <svg className="w-3 h-3 fill-amber-600" viewBox="0 0 24 24"><path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" /></svg>
                        Add ({favoritesInView.length})
                    </button>
                )}

                <div className="flex-1" />

                <div className="flex items-center gap-1 shrink-0">
                    {onToggleFavorites && (
                        <>
                            <button
                                onClick={onToggleFavorites}
                                className={`p-1.5 rounded transition-all ${showOnlyFavorites ? 'bg-amber-500/20 text-amber-500' : 'text-amber-700/60 hover:text-amber-500'}`}
                                title="Show favorites only"
                            >
                                <svg className="w-4 h-4" fill={showOnlyFavorites ? 'currentColor' : 'none'} stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2">
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                                </svg>
                            </button>
                            <div className="w-px h-4 bg-border/50 mx-0.5" />
                        </>
                    )}
                    <button onClick={() => setViewMode('table')} className={`p-1.5 rounded transition-all ${viewMode === 'table' ? 'bg-primary/20 text-primary' : 'text-muted-foreground/40 hover:text-muted-foreground'}`} title="Table view">
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" /></svg>
                    </button>
                    <button onClick={() => setViewMode('grid')} className={`p-1.5 rounded transition-all ${viewMode === 'grid' ? 'bg-primary/20 text-primary' : 'text-muted-foreground/40 hover:text-muted-foreground'}`} title="Grid view">
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2"><path strokeLinecap="round" strokeLinejoin="round" d="M4 5a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1V5zm10 0a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1V5zM4 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1v-4zm10 0a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" /></svg>
                    </button>
                </div>

                <div className="relative w-full max-w-xs shrink-0">
                    <div className="absolute inset-y-0 left-3 flex items-center pointer-events-none">
                        <svg className="w-3.5 h-3.5 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
                    </div>
                    <input
                        type="text"
                        placeholder="Search by name or ID..."
                        value={search}
                        onChange={e => setSearch(e.target.value)}
                        className="w-full bg-background border border-border/50 rounded-lg py-2 pl-10 pr-4 text-[10px] font-bold uppercase tracking-wider focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all"
                    />
                </div>
            </div>

            {/* Progress strip hidden — loading handled silently in background */}

            {/* Content — Table or Grid + Detail Panel */}
            <div className="flex-1 bg-card rounded-xl border border-border overflow-hidden flex flex-col relative">
                {loading && (
                    <div className="absolute inset-0 bg-background/50 backdrop-blur-[2px] z-30 flex items-center justify-center">
                        <div className="flex flex-col items-center space-y-4">
                            <div className="w-10 h-10 border-4 border-primary/20 border-t-primary rounded-full animate-spin" />
                            <span className="text-[10px] font-black uppercase tracking-[0.2em] text-primary animate-pulse">Loading Database...</span>
                        </div>
                    </div>
                )}

                <div className="flex-1 flex min-h-0 relative">
                <div className={`flex-1 min-w-0 flex flex-col ${activeDetail ? 'max-w-[60%]' : ''}`}>
                {viewMode === 'grid' ? (
                    <div className="flex-1 overflow-y-auto custom-scrollbar p-3">
                        {filteredItems.length === 0 ? (
                            <div className="flex flex-col items-center justify-center py-16 text-muted-foreground/50">
                                <svg className="w-8 h-8 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-2.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/></svg>
                                <span className="text-[10px] font-black uppercase tracking-widest">No items found</span>
                            </div>
                        ) : (
                            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-2">
                                {filteredItems.map(item => {
                                    const owned = resolveOwned(item);
                                    const hasOwned = owned.inv > 0 || owned.storage > 0;
                                    return (
                                        <div key={item.id}
                                            className={`relative rounded-xl border bg-card p-1.5 flex flex-col items-center gap-1 transition-all hover:border-primary/40 hover:bg-primary/[0.03] group ${!readOnly && selectedDbItems.has(item.id) ? 'border-primary/50 bg-primary/[0.05]' : 'border-border/50'}`}
                                        >
                                            <button onClick={() => toggleFav(item.id)} className="absolute top-2 right-2 p-0.5 transition-all hover:scale-125 z-10">
                                                <svg className={`w-3.5 h-3.5 ${isFav(item.id) ? 'text-amber-500 fill-amber-500' : 'text-muted-foreground/20 fill-none hover:text-amber-500/50'}`} stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2">
                                                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                                                </svg>
                                            </button>
                                            {!readOnly && (
                                                <button
                                                    onClick={() => toggleItem(item.id)}
                                                    aria-label={`${selectedDbItems.has(item.id) ? 'Deselect' : 'Select'} ${item.name}`}
                                                    className="absolute top-2 left-2 z-10"
                                                >
                                                    <div className={`w-4 h-4 rounded border flex items-center justify-center transition-all ${selectedDbItems.has(item.id) ? 'bg-primary border-primary' : 'bg-muted/30 border-border hover:border-primary/50'}`}>
                                                        {selectedDbItems.has(item.id) && <svg className="w-3 h-3 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="4" d="M5 13l4 4L19 7"/></svg>}
                                                    </div>
                                                </button>
                                            )}
                                            <div className="w-20 h-20 rounded-lg bg-muted/20 border border-border/50 flex items-center justify-center overflow-hidden cursor-pointer"
                                                onClick={() => onSelectItem ? onSelectItem(item) : setDetailItem(item)}
                                                onMouseEnter={(e) => { const r = e.currentTarget.getBoundingClientRect(); setHoverTooltip({name: item.name, path: item.iconPath, x: r.left + r.width / 2, y: r.top}); }}
                                                onMouseLeave={() => setHoverTooltip(null)}>
                                                {brokenIcons.has(item.iconPath)
                                                    ? <span className="text-[10px] font-black text-muted-foreground/30">?</span>
                                                    : <img src={item.iconPath} alt="" className="w-full h-full p-0.5 object-contain drop-shadow-md group-hover:scale-110 transition-transform duration-300" onError={() => handleImageError(item.iconPath)} />
                                                }
                                            </div>
                                            <div className="text-center w-full cursor-pointer" onClick={() => onSelectItem ? onSelectItem(item) : setDetailItem(item)}>
                                                <div className="text-[10px] font-bold text-foreground truncate group-hover:text-primary transition-colors" title={item.name}>{item.name}</div>
                                                <div className="flex items-center justify-center gap-1.5 mt-1">
                                                    {item.flags?.includes('ban_risk') && <RiskBadge flag="ban_risk" />}
                                                    {item.flags?.includes('cut_content') && <RiskBadge flag="cut_content" />}
                                                </div>
                                            </div>
                                            {hasOwned && (
                                                <div className="flex items-center gap-2 text-[8px] font-black tabular-nums">
                                                    <span className={`px-1.5 py-0.5 rounded border ${owned.inv > 0 ? 'text-green-500 bg-green-500/10 border-green-500/30' : 'text-muted-foreground/30 bg-muted/10 border-border/30'}`}>I:{owned.inv}</span>
                                                    <span className={`px-1.5 py-0.5 rounded border ${owned.storage > 0 ? 'text-green-500 bg-green-500/10 border-green-500/30' : 'text-muted-foreground/30 bg-muted/10 border-border/30'}`}>S:{owned.storage}</span>
                                                </div>
                                            )}
                                        </div>
                                    );
                                })}
                            </div>
                        )}
                    </div>
                ) : (

                <div ref={scrollRef} className="flex-1 overflow-y-auto custom-scrollbar">
                    <table className="w-full text-left border-collapse">
                        <thead className="bg-muted/30 text-[10px] font-black text-muted-foreground uppercase tracking-[0.2em] sticky top-0 z-20 backdrop-blur-md border-b border-border">
                            <tr>
                                {!readOnly && (
                                    <th className="px-2 py-4 w-8">
                                        <div
                                            onClick={toggleAll}
                                            title="Select all"
                                            className={`w-4 h-4 rounded border flex items-center justify-center transition-all cursor-pointer ${filteredItems.length > 0 && filteredItems.every(i => selectedDbItems.has(i.id)) ? 'bg-primary border-primary' : 'bg-muted/30 border-border hover:border-primary/50'}`}
                                        >
                                            {filteredItems.length > 0 && filteredItems.every(i => selectedDbItems.has(i.id)) &&
                                                <svg className="w-3 h-3 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="4" d="M5 13l4 4L19 7"/></svg>}
                                        </div>
                                    </th>
                                )}
                                <th className="px-1 py-4 w-8"></th>
                                <th className="px-2 py-4 w-14">Icon</th>
                                <th className="px-2 py-4 w-full cursor-pointer hover:text-primary transition-colors" onClick={() => handleSort('name')}>
                                    Name {sortCol === 'name' && (sortDir === 'asc' ? '↑' : '↓')}
                                </th>
                                {columnVisibility.id && (
                                    <th className="px-3 py-4 cursor-pointer hover:text-primary transition-colors whitespace-nowrap" onClick={() => handleSort('id')}>
                                        ID {sortCol === 'id' && (sortDir === 'asc' ? '↑' : '↓')}
                                    </th>
                                )}
                                {columnVisibility.category && showSubGroupColumn && (
                                    <th className="px-3 py-4 cursor-pointer hover:text-primary transition-colors whitespace-nowrap" onClick={() => handleSort(category === 'all' ? 'category' : 'subCategory')}>
                                        Sub-Category {sortCol === (category === 'all' ? 'category' : 'subCategory') && (sortDir === 'asc' ? '↑' : '↓')}
                                    </th>
                                )}
                                {showMaxUpColumn && (
                                    <th className="px-3 py-4 text-center cursor-pointer hover:text-primary transition-colors whitespace-nowrap" onClick={() => handleSort('maxUpgrade')}>
                                        Max Up {sortCol === 'maxUpgrade' && (sortDir === 'asc' ? '↑' : '↓')}
                                    </th>
                                )}
                                {showWeightColumn && (
                                    <th className="px-3 py-4 cursor-pointer hover:text-primary transition-colors text-right w-20" onClick={() => handleSort('weight')}>
                                        Weight {sortCol === 'weight' && (sortDir === 'asc' ? '↑' : '↓')}
                                    </th>
                                )}
                                <th className="px-3 py-4 text-center w-28 cursor-pointer hover:text-primary transition-colors" onClick={() => handleSort('ownedInv')}>
                                    Inventory {sortCol === 'ownedInv' && (sortDir === 'asc' ? '↑' : '↓')}
                                </th>
                                <th className="px-3 py-4 text-center w-28 cursor-pointer hover:text-primary transition-colors" onClick={() => handleSort('ownedStorage')}>
                                    Storage {sortCol === 'ownedStorage' && (sortDir === 'asc' ? '↑' : '↓')}
                                </th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-border/30">
                            {rowVirtualizer.getVirtualItems().length > 0 && rowVirtualizer.getVirtualItems()[0].start > 0 && (
                                <tr><td colSpan={colCount} style={{ height: rowVirtualizer.getVirtualItems()[0].start, padding: 0, border: 'none' }} /></tr>
                            )}
                            {rowVirtualizer.getVirtualItems().map(virtualRow => {
                                const item = filteredItems[virtualRow.index];

                                return (
                                    <tr key={item.id} data-index={virtualRow.index} ref={node => { if (node) rowVirtualizer.measureElement(node); }} className={`group hover:bg-primary/[0.03] transition-colors ${selectedDbItems.has(item.id) ? 'bg-primary/[0.02]' : ''}`}>
                                        {!readOnly && (
                                            <td className="px-2 py-2">
                                                <div
                                                    onClick={() => toggleItem(item.id)}
                                                    className={`w-4 h-4 rounded border flex items-center justify-center transition-all cursor-pointer ${selectedDbItems.has(item.id) ? 'bg-primary border-primary' : 'bg-muted/30 border-border group-hover:border-primary/50'}`}
                                                >
                                                    {selectedDbItems.has(item.id) &&
                                                        <svg className="w-3 h-3 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="4" d="M5 13l4 4L19 7"/></svg>}
                                                </div>
                                            </td>
                                        )}
                                        <td className="px-1 py-2 text-center">
                                            <button onClick={e => { e.stopPropagation(); toggleFav(item.id); }} className="p-0.5 transition-all hover:scale-125">
                                                <svg className={`w-4 h-4 ${isFav(item.id) ? 'text-amber-500 fill-amber-500' : 'text-muted-foreground/20 fill-none hover:text-amber-500/50'}`} stroke="currentColor" viewBox="0 0 24 24" strokeWidth="2">
                                                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                                                </svg>
                                            </button>
                                        </td>
                                        <td className="px-2 py-0.5">
                                            <div
                                                className="w-12 h-12 bg-muted/20 rounded-lg border border-border/50 flex items-center justify-center overflow-hidden group-hover:border-primary/30 transition-all cursor-pointer"
                                                onClick={() => onSelectItem ? onSelectItem(item) : setDetailItem(item)}
                                                onMouseEnter={(e) => { const r = e.currentTarget.getBoundingClientRect(); setHoverTooltip({name: item.name, path: item.iconPath, x: r.left + r.width / 2, y: r.top}); }}
                                                onMouseLeave={() => setHoverTooltip(null)}
                                            >
                                                {brokenIcons.has(item.iconPath) ? (
                                                    <span className="text-[10px] font-black text-muted-foreground/30 select-none">?</span>
                                                ) : (
                                                    <img
                                                        src={item.iconPath}
                                                        alt={item.name}
                                                        className="w-full h-full object-contain drop-shadow-md group-hover:scale-110 transition-transform duration-300"
                                                        onError={() => handleImageError(item.iconPath)}
                                                    />
                                                )}
                                            </div>
                                        </td>
                                        <td className="px-2 py-2 w-full">
                                            <div className="flex items-center gap-1.5 flex-wrap">
                                                <span
                                                    className="text-[13px] font-semibold text-foreground group-hover:text-primary transition-colors cursor-pointer hover:underline decoration-primary/40 underline-offset-2"
                                                    onClick={e => { e.stopPropagation(); onSelectItem ? onSelectItem(item) : setDetailItem(item); }}
                                                >{item.name}</span>
                                                {item.flags?.includes('cut_content') && (
                                                    <RiskBadge flag="cut_content" />
                                                )}
                                                {item.flags?.includes('ban_risk') && (
                                                    <RiskBadge flag="ban_risk" />
                                                )}
                                            </div>
                                        </td>
                                        {columnVisibility.id && (
                                            <td className="px-3 py-2 text-[10px] font-mono text-muted-foreground whitespace-nowrap">0x{item.id.toString(16).toUpperCase()}</td>
                                        )}
                                        {columnVisibility.category && showSubGroupColumn && (
                                            <td className="px-3 py-2 whitespace-nowrap">
                                                <span className="text-[8px] font-black uppercase tracking-widest px-2 py-1 bg-muted/30 rounded-md text-muted-foreground border border-border/20">
                                                    {category === 'all'
                                                        ? (CATEGORY_LABEL[item.category] ?? item.category.replace(/_/g, ' '))
                                                        : (item.subCategory || '—')}
                                                </span>
                                            </td>
                                        )}
                                        {showMaxUpColumn && (
                                            <td className="px-3 py-2 text-center whitespace-nowrap">
                                                <span className="text-[10px] font-black tabular-nums text-muted-foreground bg-muted/20 px-2 py-1 rounded border border-border/30">
                                                    {item.maxUpgrade > 0 ? `+${item.maxUpgrade}` : '—'}
                                                </span>
                                            </td>
                                        )}
                                        {showWeightColumn && (
                                            <td className="px-3 py-2 text-right text-[11px] font-mono text-muted-foreground tabular-nums w-20">
                                                {item.weight !== undefined && item.weight > 0 ? item.weight.toFixed(1) : '—'}
                                            </td>
                                        )}
                                        {(() => {
                                            const owned = resolveOwned(item);
                                            const cellClass = (have: number, max: number): string => {
                                                if (have === 0) return 'text-muted-foreground/50 bg-muted/20 border-border/30';
                                                if (max > 0 && have >= max) return 'text-amber-500 bg-amber-500/10 border-amber-500/30';
                                                return 'text-green-500 bg-green-500/10 border-green-500/30';
                                            };
                                            return (
                                                <>
                                                    <td className="px-3 py-2 text-center">
                                                        <span className={`inline-block text-[10px] font-black tabular-nums whitespace-nowrap px-2 py-1 rounded border ${cellClass(owned.inv, effectiveCap(item, 'inv', clearCount, useTechnicalCapsMode))}`}>
                                                            {owned.inv} / {effectiveCap(item, 'inv', clearCount, useTechnicalCapsMode)}
                                                        </span>
                                                    </td>
                                                    <td className="px-3 py-2 text-center">
                                                        <span className={`inline-block text-[10px] font-black tabular-nums whitespace-nowrap px-2 py-1 rounded border ${cellClass(owned.storage, effectiveCap(item, 'storage', clearCount, useTechnicalCapsMode))}`}>
                                                            {owned.storage} / {effectiveCap(item, 'storage', clearCount, useTechnicalCapsMode)}
                                                        </span>
                                                    </td>
                                                </>
                                            );
                                        })()}
                                    </tr>
                                );
                            })}
                            {(() => {
                                const virtualItems = rowVirtualizer.getVirtualItems();
                                const paddingBottom = virtualItems.length > 0
                                    ? rowVirtualizer.getTotalSize() - virtualItems[virtualItems.length - 1].end
                                    : 0;
                                return paddingBottom > 0 ? <tr><td colSpan={colCount} style={{ height: paddingBottom, padding: 0, border: 'none' }} /></tr> : null;
                            })()}
                        </tbody>
                    </table>
                </div>
                )}
                </div>

                {activeDetail && (
                    <div className="absolute top-0 right-0 bottom-0 w-[40%] animate-in slide-in-from-right duration-200">
                        <ItemDetailPanel item={activeDetail} onClose={handleCloseDetail} />
                    </div>
                )}
                </div>
            </div>

            {/* Hover Icon Tooltip */}
            {hoverTooltip && (
                <div className="fixed z-[60] pointer-events-none" style={{left: hoverTooltip.x, top: hoverTooltip.y - 8, transform: 'translate(-50%, -100%)'}}>
                    <div className="bg-card border border-border rounded-lg shadow-xl p-2">
                        <img src={hoverTooltip.path} alt="" className="w-36 h-36 object-contain drop-shadow-md" onError={(e) => (e.currentTarget.style.display = 'none')} />
                    </div>
                </div>
            )}
        </div>
    );
}
