import { useState, useEffect, useMemo, useRef } from 'react';
import {
    GetInventoryOrder,
    GetStorageOrder,
    MoveItemsBetweenInventoryAndStorage,
    ReorderInventory,
    ReorderStorage,
} from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
import toast from '../lib/toast';
import { WeaponEditModal } from './WeaponEditModal';

type DragSource = 'inventory' | 'storage' | null;
type FrameDropTarget = 'inventory' | 'storage' | null;

const GRID_COLS = 5;
const GRID_MIN_ROWS = 6;
const GRID_MIN_CELLS = GRID_COLS * GRID_MIN_ROWS; // 30

// Fixed-pixel frame dimensions so each grid shows exactly 5×6 = 30 tiles.
// Items beyond 30 stay inside the frame via overflow-y scroll. Both Storage
// and Inventory frames use identical constants to guarantee visual parity.
const CELL_PX = 96;
const GAP_PX = 6; // matches Tailwind gap-1.5
const PAD_PX = 8; // matches Tailwind p-2
const FRAME_WIDTH_PX = GRID_COLS * CELL_PX + (GRID_COLS - 1) * GAP_PX + 2 * PAD_PX;
const FRAME_HEIGHT_PX = GRID_MIN_ROWS * CELL_PX + (GRID_MIN_ROWS - 1) * GAP_PX + 2 * PAD_PX;
const GRID_TEMPLATE_COLUMNS = `repeat(${GRID_COLS}, ${CELL_PX}px)`;

type SortOrderTabKey = 'weapons' | 'talismans' | 'head' | 'chest' | 'arms' | 'legs';

type SortMode = 'acquisition-asc' | 'acquisition-desc' | 'weight-asc' | 'weight-desc' | 'type-asc' | 'type-desc' | 'custom';
type SortOption = Exclude<SortMode, 'custom'>;

const SORT_OPTIONS: { value: SortOption; label: string }[] = [
    { value: 'acquisition-asc', label: 'Acquisition ↑' },
    { value: 'acquisition-desc', label: 'Acquisition ↓' },
    { value: 'weight-asc', label: 'Weight ↑' },
    { value: 'weight-desc', label: 'Weight ↓' },
    { value: 'type-asc', label: 'Type ↑' },
    { value: 'type-desc', label: 'Type ↓' },
];

const SORT_TABS: { key: SortOrderTabKey; label: string }[] = [
    { key: 'weapons', label: 'Weapons' },
    { key: 'talismans', label: 'Talismans' },
    { key: 'head', label: 'Head' },
    { key: 'chest', label: 'Chest' },
    { key: 'arms', label: 'Arms' },
    { key: 'legs', label: 'Legs' },
];

interface Props {
    charIndex: number;
    inventoryVersion: number;
    onMutate?: () => void;
}

export function SortOrderTab({ charIndex, inventoryVersion, onMutate }: Props) {
    const [activeSortTab, setActiveSortTab] = useState<SortOrderTabKey>('weapons');
    const [baseItems, setBaseItems] = useState<main.InventoryOrderItem[]>([]);
    const [previewItems, setPreviewItems] = useState<main.InventoryOrderItem[]>([]);
    const [baseStorageItems, setBaseStorageItems] = useState<main.InventoryOrderItem[]>([]);
    const [previewStorageItems, setPreviewStorageItems] = useState<main.InventoryOrderItem[]>([]);
    const [storageSortMode, setStorageSortMode] = useState<SortMode>('acquisition-asc');
    const [sortMode, setSortMode] = useState<SortMode>('acquisition-asc');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [applying, setApplying] = useState(false);
    const [applyingStorage, setApplyingStorage] = useState(false);
    const [confirmOpen, setConfirmOpen] = useState(false);
    const [confirmStorageOpen, setConfirmStorageOpen] = useState(false);
    const [helpOpen, setHelpOpen] = useState(false);
    const [dragFrom, setDragFrom] = useState<number | null>(null);
    const [dragOver, setDragOver] = useState<number | null>(null);
    const [storageDragFrom, setStorageDragFrom] = useState<number | null>(null);
    const [storageDragOver, setStorageDragOver] = useState<number | null>(null);
    const [selectedHandles, setSelectedHandles] = useState<Set<number>>(new Set());
    const [anchorHandle, setAnchorHandle] = useState<number | null>(null);
    const [isBlockDragging, setIsBlockDragging] = useState(false);
    const [storageSelectedHandles, setStorageSelectedHandles] = useState<Set<number>>(new Set());
    const [storageAnchorHandle, setStorageAnchorHandle] = useState<number | null>(null);
    const [storageBlockDragging, setStorageBlockDragging] = useState(false);
    const didDragRef = useRef(false);
    const dragAnchorHandleRef = useRef<number | null>(null);
    const prevHasChangesRef = useRef(false);
    // Cross-grid transfer state. dragSourceRef tells us whether the in-flight
    // drag originated in Inventory (handled by tile-level reorder + cross-grid
    // frame drop) or Storage (handled only by cross-grid frame drop).
    // draggedStorageHandleRef carries the handle for storage drags because
    // Storage records do not share the Inventory tile drag refs.
    const dragSourceRef = useRef<DragSource>(null);
    const draggedStorageHandleRef = useRef<number | null>(null);
    const storageDragAnchorHandleRef = useRef<number | null>(null);
    const prevStorageHasChangesRef = useRef(false);
    const [frameDropTarget, setFrameDropTarget] = useState<FrameDropTarget>(null);
    const [transferring, setTransferring] = useState(false);
    // Weapon edit modal state — populated only on weapons tab via double-click.
    const [weaponEditor, setWeaponEditor] = useState<{
        item: main.InventoryOrderItem;
        source: 'inventory' | 'storage';
    } | null>(null);

    const openWeaponEditor = (
        item: main.InventoryOrderItem,
        source: 'inventory' | 'storage',
    ) => {
        if (activeSortTab !== 'weapons') return;
        setWeaponEditor({ item, source });
    };

    // reloadTabData fetches fresh Inventory + Storage data for the active tab
    // and applies it to local state. Used both by the on-mount/version useEffect
    // and explicitly after a successful Inventory ↔ Storage transfer, where
    // relying on the parent's `inventoryVersion` bump alone leaves the grids
    // stale (the parent may not bump it, or React may batch the bump on a
    // later tick).
    const reloadTabData = (_resetSort: boolean): Promise<void> => {
        return Promise.all([
            GetInventoryOrder(charIndex, activeSortTab),
            GetStorageOrder(charIndex, activeSortTab),
        ])
            .then(([invData, storageData]) => {
                const sortedInv = sortByMode(invData, 'acquisition-asc');
                setBaseItems(sortedInv);
                setPreviewItems(sortedInv);
                const sortedSto = sortByMode(storageData, 'acquisition-asc');
                setBaseStorageItems(sortedSto);
                setPreviewStorageItems(sortedSto);
            });
    };

    useEffect(() => {
        setLoading(true);
        setError(null);
        setSortMode('acquisition-asc');
        setStorageSortMode('acquisition-asc');
        setDragFrom(null);
        setDragOver(null);
        setStorageDragFrom(null);
        setStorageDragOver(null);
        setSelectedHandles(new Set());
        setAnchorHandle(null);
        setIsBlockDragging(false);
        setStorageSelectedHandles(new Set());
        setStorageAnchorHandle(null);
        setStorageBlockDragging(false);
        didDragRef.current = false;
        dragAnchorHandleRef.current = null;
        storageDragAnchorHandleRef.current = null;
        dragSourceRef.current = null;
        draggedStorageHandleRef.current = null;
        setFrameDropTarget(null);
        prevHasChangesRef.current = false;
        prevStorageHasChangesRef.current = false;
        reloadTabData(true)
            .catch((err: unknown) => setError(String(err)))
            .finally(() => setLoading(false));
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [charIndex, inventoryVersion, activeSortTab]);

    const hasChanges = useMemo(() => {
        if (previewItems.length !== baseItems.length) return false;
        return previewItems.some((item, i) => item.handle !== baseItems[i].handle);
    }, [previewItems, baseItems]);

    const storageHasChanges = useMemo(() => {
        if (previewStorageItems.length !== baseStorageItems.length) return false;
        return previewStorageItems.some((item, i) => item.handle !== baseStorageItems[i].handle);
    }, [previewStorageItems, baseStorageItems]);

    // One-shot toast on clean → dirty transition. Re-arms once hasChanges
    // returns to false (after Apply/Reset/reload), so repeated drags within
    // a single dirty session do not spam.
    useEffect(() => {
        if (hasChanges && !prevHasChangesRef.current) {
            toast('Inventory preview changed. Click Apply Order to save it as in-game Acquisition Order.');
        }
        prevHasChangesRef.current = hasChanges;
    }, [hasChanges]);

    useEffect(() => {
        if (storageHasChanges && !prevStorageHasChangesRef.current) {
            toast('Storage preview changed. Click Apply Order to save it as in-game Acquisition Order.');
        }
        prevStorageHasChangesRef.current = storageHasChanges;
    }, [storageHasChanges]);

    const applySort = (mode: SortMode) => {
        setPreviewItems(sortByMode(previewItems, mode));
        setSortMode(mode);
    };

    const applyStorageSort = (mode: SortMode) => {
        setPreviewStorageItems(sortByMode(previewStorageItems, mode));
        setStorageSortMode(mode);
    };

    const resetPreview = () => {
        setPreviewItems(sortByMode(baseItems, 'acquisition-asc'));
        setSortMode('acquisition-asc');
        setSelectedHandles(new Set());
        setAnchorHandle(null);
    };

    const resetStoragePreview = () => {
        setPreviewStorageItems(sortByMode(baseStorageItems, 'acquisition-asc'));
        setStorageSortMode('acquisition-asc');
        setStorageSelectedHandles(new Set());
        setStorageAnchorHandle(null);
    };

    const openInventoryApplyConfirm = () => {
        if (storageHasChanges) {
            toast('Apply or reset Storage order before applying Inventory order.');
            return;
        }
        setConfirmOpen(true);
    };

    const openStorageApplyConfirm = () => {
        if (hasChanges) {
            toast('Apply or reset Inventory order before applying Storage order.');
            return;
        }
        setConfirmStorageOpen(true);
    };

    const handleApplyConfirm = () => {
        setConfirmOpen(false);
        setApplying(true);
        const handles = previewItems.map((i) => i.handle);
        const tab = activeSortTab;
        const displayName = SORT_TABS.find((t) => t.key === tab)!.label;
        ReorderInventory(charIndex, tab, handles)
            .then(() => {
                toast.success(`${displayName} order updated successfully.`);
                onMutate?.();
                return GetInventoryOrder(charIndex, tab);
            })
            .then((data) => {
                const sorted = sortByMode(data, 'acquisition-asc');
                setBaseItems(sorted);
                setPreviewItems(sorted);
                setSortMode('acquisition-asc');
                setSelectedHandles(new Set());
                setAnchorHandle(null);
                setIsBlockDragging(false);
            })
            .catch((err: unknown) => {
                toast.error(`Failed to apply ${displayName} order: ` + String(err));
            })
            .finally(() => setApplying(false));
    };

    const handleStorageApplyConfirm = () => {
        setConfirmStorageOpen(false);
        setApplyingStorage(true);
        const handles = previewStorageItems.map((i) => i.handle);
        const tab = activeSortTab;
        const displayName = SORT_TABS.find((t) => t.key === tab)!.label;
        ReorderStorage(charIndex, tab, handles)
            .then(() => {
                toast.success(`${displayName} storage order updated successfully.`);
                onMutate?.();
                return GetStorageOrder(charIndex, tab);
            })
            .then((data) => {
                const sorted = sortByMode(data, 'acquisition-asc');
                setBaseStorageItems(sorted);
                setPreviewStorageItems(sorted);
                setStorageSortMode('acquisition-asc');
                setStorageSelectedHandles(new Set());
                setStorageAnchorHandle(null);
                setStorageBlockDragging(false);
            })
            .catch((err: unknown) => {
                toast.error(`Failed to apply ${displayName} storage order: ` + String(err));
            })
            .finally(() => setApplyingStorage(false));
    };

    // ── Selection / DnD handlers ──────────────────────────────────────────────

    const handleTileClick = (item: main.InventoryOrderItem, e: React.MouseEvent) => {
        if (didDragRef.current) {
            didDragRef.current = false;
            return;
        }
        if (e.shiftKey && anchorHandle !== null) {
            const idxA = previewItems.findIndex((it) => it.handle === anchorHandle);
            const idxB = previewItems.findIndex((it) => it.handle === item.handle);
            if (idxA >= 0 && idxB >= 0) {
                const [lo, hi] = idxA < idxB ? [idxA, idxB] : [idxB, idxA];
                const range = new Set(previewItems.slice(lo, hi + 1).map((it) => it.handle));
                setSelectedHandles(range);
            }
            return;
        }
        if (e.ctrlKey || e.metaKey) {
            setSelectedHandles((prev) => {
                const next = new Set(prev);
                if (next.has(item.handle)) next.delete(item.handle);
                else next.add(item.handle);
                return next;
            });
            setAnchorHandle(item.handle);
            return;
        }
        setSelectedHandles(new Set([item.handle]));
        setAnchorHandle(item.handle);
    };

    const handleStorageTileClick = (item: main.InventoryOrderItem, e: React.MouseEvent) => {
        if (didDragRef.current) {
            didDragRef.current = false;
            return;
        }
        if (e.shiftKey && storageAnchorHandle !== null) {
            const idxA = previewStorageItems.findIndex((it) => it.handle === storageAnchorHandle);
            const idxB = previewStorageItems.findIndex((it) => it.handle === item.handle);
            if (idxA >= 0 && idxB >= 0) {
                const [lo, hi] = idxA < idxB ? [idxA, idxB] : [idxB, idxA];
                const range = new Set(previewStorageItems.slice(lo, hi + 1).map((it) => it.handle));
                setStorageSelectedHandles(range);
            }
            return;
        }
        if (e.ctrlKey || e.metaKey) {
            setStorageSelectedHandles((prev) => {
                const next = new Set(prev);
                if (next.has(item.handle)) next.delete(item.handle);
                else next.add(item.handle);
                return next;
            });
            setStorageAnchorHandle(item.handle);
            return;
        }
        setStorageSelectedHandles(new Set([item.handle]));
        setStorageAnchorHandle(item.handle);
    };

    const handleDragStart = (localIdx: number, item: main.InventoryOrderItem) => {
        didDragRef.current = true;
        dragAnchorHandleRef.current = item.handle;
        dragSourceRef.current = 'inventory';
        const isInSelection = selectedHandles.has(item.handle);
        if (!isInSelection) {
            setSelectedHandles(new Set([item.handle]));
            setAnchorHandle(item.handle);
            setIsBlockDragging(false);
        } else {
            setIsBlockDragging(selectedHandles.size > 1);
        }
        setDragFrom(localIdx);
        setDragOver(null);
    };

    const handleStorageDragStart = (localIdx: number, item: main.InventoryOrderItem) => {
        didDragRef.current = true;
        dragSourceRef.current = 'storage';
        draggedStorageHandleRef.current = item.handle;
        storageDragAnchorHandleRef.current = item.handle;
        setFrameDropTarget(null);
        const isInSelection = storageSelectedHandles.has(item.handle);
        if (!isInSelection) {
            setStorageSelectedHandles(new Set([item.handle]));
            setStorageAnchorHandle(item.handle);
            setStorageBlockDragging(false);
        } else {
            setStorageBlockDragging(storageSelectedHandles.size > 1);
        }
        setStorageDragFrom(localIdx);
        setStorageDragOver(null);
    };

    const handleStorageTileDragOver = (e: React.DragEvent, localIdx: number) => {
        e.preventDefault();
        if (storageDragFrom !== null && storageDragFrom !== localIdx) {
            setStorageDragOver(localIdx);
        }
    };

    // Tile-level drop within Storage grid. Reorders previewStorageItems
    // (no backend call). Mirrors handleDrop for Inventory: single-item drops
    // do an in-place insertion; block-selected drags use anchor-aware
    // insertion so the dragged anchor lands on the drop target.
    const handleStorageTileDrop = (localIdx: number) => {
        if (storageDragFrom === null) {
            setStorageDragOver(null);
            return;
        }
        const globalFrom = storageDragFrom;
        const globalTo = localIdx;
        if (globalFrom === globalTo) {
            setStorageDragFrom(null);
            setStorageDragOver(null);
            return;
        }
        const draggedItem = previewStorageItems[globalFrom];
        if (!draggedItem) {
            setStorageDragFrom(null);
            setStorageDragOver(null);
            return;
        }
        const blockMove = storageSelectedHandles.has(draggedItem.handle) && storageSelectedHandles.size > 1;
        if (!blockMove) {
            const next = [...previewStorageItems];
            const [moved] = next.splice(globalFrom, 1);
            next.splice(globalTo, 0, moved);
            setPreviewStorageItems(next);
            setStorageSortMode('custom');
            setStorageDragFrom(null);
            setStorageDragOver(null);
            return;
        }
        const targetItem = previewStorageItems[globalTo];
        if (targetItem && storageSelectedHandles.has(targetItem.handle)) {
            let segStart = globalFrom;
            while (segStart > 0 && storageSelectedHandles.has(previewStorageItems[segStart - 1].handle)) {
                segStart--;
            }
            let segEnd = globalFrom;
            while (
                segEnd < previewStorageItems.length - 1 &&
                storageSelectedHandles.has(previewStorageItems[segEnd + 1].handle)
            ) {
                segEnd++;
            }
            if (globalTo >= segStart && globalTo <= segEnd) {
                setStorageDragFrom(null);
                setStorageDragOver(null);
                return;
            }
        }
        const selectedInOrder = previewStorageItems.filter((it) => storageSelectedHandles.has(it.handle));
        const rest = previewStorageItems.filter((it) => !storageSelectedHandles.has(it.handle));
        const anchorIndexInSelected = selectedInOrder.findIndex(
            (it) => it.handle === draggedItem.handle,
        );
        if (anchorIndexInSelected < 0) {
            setStorageDragFrom(null);
            setStorageDragOver(null);
            return;
        }
        let insertIdx = globalTo - anchorIndexInSelected;
        if (insertIdx < 0) insertIdx = 0;
        if (insertIdx > rest.length) insertIdx = rest.length;
        const next = [...rest.slice(0, insertIdx), ...selectedInOrder, ...rest.slice(insertIdx)];
        setPreviewStorageItems(next);
        setStorageSortMode('custom');
        setStorageDragFrom(null);
        setStorageDragOver(null);
    };

    const handleDragOver = (e: React.DragEvent, localIdx: number) => {
        e.preventDefault();
        if (dragFrom !== null && dragFrom !== localIdx) {
            setDragOver(localIdx);
        }
    };

    const handleDrop = (localIdx: number) => {
        if (dragFrom === null) {
            setDragOver(null);
            return;
        }
        const globalFrom = dragFrom;
        const globalTo = localIdx;
        if (globalFrom === globalTo) {
            setDragFrom(null);
            setDragOver(null);
            return;
        }
        const draggedItem = previewItems[globalFrom];
        if (!draggedItem) {
            setDragFrom(null);
            setDragOver(null);
            return;
        }
        const blockMove = selectedHandles.has(draggedItem.handle) && selectedHandles.size > 1;
        if (!blockMove) {
            const next = [...previewItems];
            const [moved] = next.splice(globalFrom, 1);
            next.splice(globalTo, 0, moved);
            setPreviewItems(next);
            setSortMode('custom');
            setDragFrom(null);
            setDragOver(null);
            return;
        }
        // Block move: anchor-aware insertion.
        // Final anchor index targets globalTo (the slot under cursor). Block
        // wraps around the anchor preserving selected order. Insertion index
        // in `rest` = globalTo - anchorIndexInSelected, clamped to [0, rest.length].
        const targetItem = previewItems[globalTo];
        if (targetItem && selectedHandles.has(targetItem.handle)) {
            // No-op only when drop falls inside the contiguous selected segment
            // surrounding the anchor — moving within own block has no useful effect.
            let segStart = globalFrom;
            while (segStart > 0 && selectedHandles.has(previewItems[segStart - 1].handle)) {
                segStart--;
            }
            let segEnd = globalFrom;
            while (
                segEnd < previewItems.length - 1 &&
                selectedHandles.has(previewItems[segEnd + 1].handle)
            ) {
                segEnd++;
            }
            if (globalTo >= segStart && globalTo <= segEnd) {
                setDragFrom(null);
                setDragOver(null);
                return;
            }
        }
        const selectedInOrder = previewItems.filter((it) => selectedHandles.has(it.handle));
        const rest = previewItems.filter((it) => !selectedHandles.has(it.handle));
        const anchorIndexInSelected = selectedInOrder.findIndex(
            (it) => it.handle === draggedItem.handle,
        );
        if (anchorIndexInSelected < 0) {
            setDragFrom(null);
            setDragOver(null);
            return;
        }
        let insertIdx = globalTo - anchorIndexInSelected;
        if (insertIdx < 0) insertIdx = 0;
        if (insertIdx > rest.length) insertIdx = rest.length;
        const next = [...rest.slice(0, insertIdx), ...selectedInOrder, ...rest.slice(insertIdx)];
        setPreviewItems(next);
        setSortMode('custom');
        setDragFrom(null);
        setDragOver(null);
    };

    const handleDragEnd = () => {
        setDragFrom(null);
        setDragOver(null);
        setStorageDragFrom(null);
        setStorageDragOver(null);
        setIsBlockDragging(false);
        setStorageBlockDragging(false);
        dragAnchorHandleRef.current = null;
        storageDragAnchorHandleRef.current = null;
        dragSourceRef.current = null;
        draggedStorageHandleRef.current = null;
        setFrameDropTarget(null);
        setTimeout(() => {
            didDragRef.current = false;
        }, 0);
    };

    // Cross-grid transfer (single item). Guarded against pending preview
    // changes — user must apply or reset Inventory order before transferring.
    const reasonToMessage = (reason: string, destLabel: string): string => {
        switch (reason) {
            case 'equipped':
                return 'Cannot transfer: item is equipped. Un-equip first.';
            case 'dest_full':
                return `${destLabel} is full.`;
            case 'dest_at_cap':
                return `${destLabel} already at max for this item.`;
            case 'missing_cap':
                return 'Item capacity unknown — transfer blocked.';
            case 'not_found':
                return 'Item not found.';
            case 'invalid_handle':
                return 'Invalid item handle.';
            default:
                return `Transfer skipped: ${reason}.`;
        }
    };

    const handleCrossTransfer = (
        direction: 'to-storage' | 'to-inventory',
        handles: number[],
    ) => {
        if (hasChanges || storageHasChanges) {
            toast('Apply or reset the current order before transferring items.');
            return;
        }
        if (transferring || handles.length === 0) return;
        const destLabel = direction === 'to-storage' ? 'Storage' : 'Inventory';
        const isBatch = handles.length > 1;
        setTransferring(true);
        MoveItemsBetweenInventoryAndStorage(charIndex, handles, direction)
            .then(async (res) => {
                const skipped = res.skipped ?? [];
                if (res.moved > 0 && skipped.length === 0) {
                    if (isBatch) {
                        toast.success(`Moved ${res.moved} items to ${destLabel}.`);
                    } else {
                        toast.success(`Moved item to ${destLabel}.`);
                    }
                } else if (res.moved > 0 && skipped.length > 0) {
                    const skip = skipped[0];
                    if (!isBatch && skip.reason === 'dest_at_cap' && skip.movedQty) {
                        toast(
                            `Moved ${skip.movedQty} to ${destLabel}; ${skip.remainingQty ?? 0} stayed due to cap.`,
                        );
                    } else if (isBatch) {
                        toast(
                            `Moved ${res.moved} item${res.moved === 1 ? '' : 's'} to ${destLabel}. ${skipped.length} skipped.`,
                        );
                    } else {
                        toast(`Moved partially (${skip.reason}).`);
                    }
                } else {
                    const reason = skipped[0]?.reason ?? 'unknown';
                    const base = reasonToMessage(reason, destLabel);
                    if (skipped.length > 1) {
                        toast.error(`${base} (${skipped.length} items skipped)`);
                    } else {
                        toast.error(base);
                    }
                }
                if (res.moved > 0) {
                    // Explicit local reload — onMutate is fire-and-forget and
                    // does not guarantee the parent bumps inventoryVersion in
                    // time to refresh both grids. Pull fresh data and apply
                    // before clearing drag state so the new counts/tiles are
                    // visible immediately.
                    try {
                        await reloadTabData(false);
                    } catch (err) {
                        toast.error(`Refresh failed: ${String(err)}`);
                    }
                    setSelectedHandles(new Set());
                    setAnchorHandle(null);
                    setStorageSelectedHandles(new Set());
                    setStorageAnchorHandle(null);
                    onMutate?.();
                }
            })
            .catch((err: unknown) => {
                toast.error(`Transfer failed: ${String(err)}`);
            })
            .finally(() => {
                dragSourceRef.current = null;
                draggedStorageHandleRef.current = null;
                setFrameDropTarget(null);
                setTransferring(false);
            });
    };

    const busy = applying || applyingStorage || transferring;
    const activeLabel = SORT_TABS.find((t) => t.key === activeSortTab)!.label;

    // ── Grid data ─────────────────────────────────────────────────────────────

    // Each grid always renders at least GRID_MIN_CELLS slots (5×6 = 30). When
    // the item count exceeds the minimum, additional rows are appended and the
    // grid container handles overflow via vertical scrolling.
    const inventoryGridCells: (main.InventoryOrderItem | null)[] = [
        ...previewItems,
        ...Array<null>(Math.max(0, GRID_MIN_CELLS - previewItems.length)).fill(null),
    ];
    const storageGridCells: (main.InventoryOrderItem | null)[] = [
        ...previewStorageItems,
        ...Array<null>(Math.max(0, GRID_MIN_CELLS - previewStorageItems.length)).fill(null),
    ];

    return (
        <>
            {/* ── Help modal ────────────────────────────────────────────────── */}
            {helpOpen && (
                <div
                    className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
                    onClick={() => setHelpOpen(false)}
                >
                    <div
                        className="bg-background border border-border rounded-xl p-6 w-[480px] shadow-2xl"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <div className="flex items-center gap-2 mb-4">
                            <div className="w-1 h-4 bg-primary rounded-full" />
                            <h3 className="text-[10px] font-black uppercase tracking-widest">
                                Sort Order — Help
                            </h3>
                        </div>
                        <ul className="text-[11px] text-muted-foreground leading-relaxed mb-5 space-y-1.5 list-disc pl-4">
                            <li>Drag items to change preview order.</li>
                            <li>Weight/Type sorting is preview-only until applied.</li>
                            <li>Apply Order saves the current order as Acquisition Order.</li>
                            <li>Inventory and Storage will be handled as separate grids.</li>
                            <li>
                                Future transfer support must work both ways: Inventory → Storage and
                                Storage → Inventory.
                            </li>
                            <li>Storage transfer/reorder is not implemented in this step.</li>
                        </ul>
                        <div className="flex justify-end">
                            <button
                                onClick={() => setHelpOpen(false)}
                                className="px-4 py-1.5 text-[10px] font-black uppercase tracking-wider rounded bg-primary/15 text-primary border border-primary/30 hover:bg-primary/20 transition-all"
                            >
                                Close
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* ── Confirm modal ─────────────────────────────────────────────── */}
            {confirmOpen && (
                <div
                    className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
                    onClick={() => setConfirmOpen(false)}
                >
                    <div
                        className="bg-background border border-border rounded-xl p-6 w-[440px] shadow-2xl"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <div className="flex items-center gap-2 mb-4">
                            <div className="w-1 h-4 bg-amber-500 rounded-full" />
                            <h3 className="text-[10px] font-black uppercase tracking-widest">
                                Apply {activeLabel} Order
                            </h3>
                        </div>
                        <p className="text-[11px] text-muted-foreground leading-relaxed mb-5">
                            {activeSortTab === 'weapons'
                                ? 'Apply this weapon order to your save? This rewrites acquisition order for weapon inventory only. Storage is not affected.'
                                : `Apply this order to your save? This rewrites acquisition order for ${activeLabel.toLowerCase()} inventory only. Storage is not affected.`}
                        </p>
                        <div className="flex justify-end gap-2">
                            <button
                                onClick={() => setConfirmOpen(false)}
                                className="px-4 py-1.5 text-[10px] font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleApplyConfirm}
                                className="px-4 py-1.5 text-[10px] font-black uppercase tracking-wider rounded bg-green-700/80 text-white hover:bg-green-700 transition-all shadow-sm"
                            >
                                Confirm
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* ── Storage confirm modal ─────────────────────────────────────── */}
            {confirmStorageOpen && (
                <div
                    className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
                    onClick={() => setConfirmStorageOpen(false)}
                >
                    <div
                        className="bg-background border border-border rounded-xl p-6 w-[440px] shadow-2xl"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <div className="flex items-center gap-2 mb-4">
                            <div className="w-1 h-4 bg-amber-500 rounded-full" />
                            <h3 className="text-[10px] font-black uppercase tracking-widest">
                                Apply {activeLabel} Storage Order
                            </h3>
                        </div>
                        <p className="text-[11px] text-muted-foreground leading-relaxed mb-5">
                            Apply this order to your save? This rewrites acquisition order for{' '}
                            {activeLabel.toLowerCase()} in storage only. Inventory is not affected.
                        </p>
                        <div className="flex justify-end gap-2">
                            <button
                                onClick={() => setConfirmStorageOpen(false)}
                                className="px-4 py-1.5 text-[10px] font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleStorageApplyConfirm}
                                className="px-4 py-1.5 text-[10px] font-black uppercase tracking-wider rounded bg-green-700/80 text-white hover:bg-green-700 transition-all shadow-sm"
                            >
                                Confirm
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {weaponEditor && (
                <WeaponEditModal
                    charIndex={charIndex}
                    item={weaponEditor.item}
                    source={weaponEditor.source}
                    onClose={() => setWeaponEditor(null)}
                />
            )}

            <div className="flex flex-col h-full min-h-0 gap-2">
                {/* ── Category tab selector + sort dropdowns ───────────────── */}
                <div className="flex items-center gap-1 shrink-0 border-b border-border/30 pb-2">
                    {SORT_TABS.map(({ key, label }) => (
                        <button
                            key={key}
                            disabled={busy || loading}
                            onClick={() => setActiveSortTab(key)}
                            className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                activeSortTab === key
                                    ? 'bg-primary/15 text-primary border border-primary/30'
                                    : 'text-muted-foreground hover:text-foreground hover:bg-muted/40 border border-transparent'
                            }`}
                        >
                            {label}
                        </button>
                    ))}
                    <div className="ml-auto flex items-center gap-3">
                        <label className="flex items-center gap-1.5">
                            <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">
                                Storage:
                            </span>
                            <select
                                value={storageSortMode === 'custom' ? '' : storageSortMode}
                                disabled={busy || loading}
                                onChange={(e) => {
                                    const v = e.target.value as SortOption;
                                    if (v) applyStorageSort(v);
                                }}
                                className="text-[10px] font-bold uppercase tracking-wider bg-muted/30 hover:bg-muted/50 border border-border/40 rounded px-1.5 py-0.5 text-foreground disabled:opacity-40 disabled:cursor-not-allowed focus:outline-none focus:ring-1 focus:ring-primary/40 transition-all"
                            >
                                {storageSortMode === 'custom' && (
                                    <option value="" disabled>
                                        Custom
                                    </option>
                                )}
                                {SORT_OPTIONS.map((o) => (
                                    <option key={o.value} value={o.value}>
                                        {o.label}
                                    </option>
                                ))}
                            </select>
                        </label>
                        <label className="flex items-center gap-1.5">
                            <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">
                                Inventory:
                            </span>
                            <select
                                value={sortMode === 'custom' ? '' : sortMode}
                                disabled={busy || loading}
                                onChange={(e) => {
                                    const v = e.target.value as SortOption;
                                    if (v) applySort(v);
                                }}
                                className="text-[10px] font-bold uppercase tracking-wider bg-muted/30 hover:bg-muted/50 border border-border/40 rounded px-1.5 py-0.5 text-foreground disabled:opacity-40 disabled:cursor-not-allowed focus:outline-none focus:ring-1 focus:ring-primary/40 transition-all"
                            >
                                {sortMode === 'custom' && (
                                    <option value="" disabled>
                                        Custom
                                    </option>
                                )}
                                {SORT_OPTIONS.map((o) => (
                                    <option key={o.value} value={o.value}>
                                        {o.label}
                                    </option>
                                ))}
                            </select>
                        </label>
                        <button
                            type="button"
                            onClick={() => setHelpOpen(true)}
                            title="Sort Order help"
                            aria-label="Sort Order help"
                            className="w-5 h-5 flex items-center justify-center rounded-full text-[10px] font-black text-muted-foreground hover:text-foreground hover:bg-muted/40 border border-border/40 transition-all"
                        >
                            ?
                        </button>
                    </div>
                </div>

                {/* ── Loading ───────────────────────────────────────────────── */}
                {loading && (
                    <div className="flex-1 flex flex-col items-center justify-center gap-3 text-muted-foreground">
                        <div className="w-6 h-6 border-2 border-foreground/20 border-t-foreground rounded-full animate-spin" />
                        <p className="text-[10px] font-bold uppercase tracking-widest">
                            Loading {activeLabel.toLowerCase()}…
                        </p>
                    </div>
                )}

                {/* ── Error ─────────────────────────────────────────────────── */}
                {!loading && error && (
                    <div className="flex-1 flex items-center justify-center">
                        <div className="text-[10px] text-red-400 text-center max-w-sm">
                            <div className="font-black uppercase tracking-wider mb-1">Error</div>
                            <div>{error}</div>
                        </div>
                    </div>
                )}

                {/* ── Main content: two-column layout ──────────────────────── */}
                {!loading && !error && (
                    <div className="flex-1 min-h-0 flex gap-3">
                        {/* ── Storage column (left, drop target + sort/apply) ── */}
                        <section className="flex-1 min-w-0 flex flex-col min-h-0 gap-2">
                            <div className="flex items-center justify-between shrink-0 gap-2 min-h-7">
                                <div className="flex items-baseline gap-2">
                                    <h4 className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">
                                        Storage
                                    </h4>
                                    <span className="text-xs font-bold text-blue-700 tabular-nums whitespace-nowrap">
                                        {previewStorageItems.length} item{previewStorageItems.length === 1 ? '' : 's'}
                                    </span>
                                    {storageSortMode === 'custom' && (
                                        <span className="text-[9px] font-bold text-amber-400/80 uppercase tracking-widest whitespace-nowrap">
                                            Custom
                                        </span>
                                    )}
                                    {storageHasChanges && (
                                        <span
                                            title="Storage preview order differs from saved order. Click Apply Order to persist."
                                            className="text-[10px] font-black uppercase tracking-wider text-amber-300 bg-amber-500/15 border border-amber-500/30 rounded px-2 py-0.5 whitespace-nowrap"
                                        >
                                            Unsaved
                                        </span>
                                    )}
                                </div>
                                <div className="flex items-center gap-2">
                                    <button
                                        disabled={busy || previewStorageItems.length === 0}
                                        onClick={resetStoragePreview}
                                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40 disabled:cursor-not-allowed"
                                    >
                                        Reset Preview
                                    </button>
                                    <button
                                        disabled={!storageHasChanges || busy}
                                        title={
                                            applyingStorage
                                                ? 'Applying…'
                                                : storageHasChanges
                                                  ? 'Apply this storage order to your save'
                                                  : 'No storage order changes to apply.'
                                        }
                                        onClick={openStorageApplyConfirm}
                                        className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                            storageHasChanges && !busy
                                                ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                                : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                                        }`}
                                    >
                                        {applyingStorage ? 'Applying…' : 'Apply Order'}
                                    </button>
                                </div>
                            </div>
                            <div
                                className={`relative shrink-0 mx-auto rounded-xl border bg-background/40 overflow-y-auto transition-colors ${
                                    frameDropTarget === 'storage'
                                        ? 'border-amber-400/70 ring-2 ring-amber-400/40'
                                        : 'border-border/50'
                                }`}
                                style={{
                                    width: FRAME_WIDTH_PX,
                                    height: FRAME_HEIGHT_PX,
                                    padding: PAD_PX,
                                }}
                                onDragOver={(e) => {
                                    if (dragSourceRef.current === 'inventory') {
                                        e.preventDefault();
                                        if (frameDropTarget !== 'storage') setFrameDropTarget('storage');
                                    }
                                }}
                                onDragLeave={(e) => {
                                    // Only clear when leaving the frame itself, not when crossing
                                    // child boundaries inside the frame.
                                    const related = e.relatedTarget as Node | null;
                                    if (!related || !e.currentTarget.contains(related)) {
                                        if (frameDropTarget === 'storage') setFrameDropTarget(null);
                                    }
                                }}
                                onDrop={(e) => {
                                    if (dragSourceRef.current !== 'inventory') return;
                                    e.preventDefault();
                                    const draggedItem =
                                        dragFrom !== null ? previewItems[dragFrom] : null;
                                    if (draggedItem) {
                                        const useBatch =
                                            selectedHandles.has(draggedItem.handle) &&
                                            selectedHandles.size > 1;
                                        const handles = useBatch
                                            ? previewItems
                                                  .filter((it) => selectedHandles.has(it.handle))
                                                  .map((it) => it.handle)
                                            : [draggedItem.handle];
                                        handleCrossTransfer('to-storage', handles);
                                    }
                                    setFrameDropTarget(null);
                                }}
                            >
                                {storageBlockDragging && storageSelectedHandles.size > 1 && (
                                    <div className="absolute top-2 left-1/2 -translate-x-1/2 z-20 pointer-events-none px-3 py-1 rounded-full bg-amber-500/95 text-white text-[10px] font-black uppercase tracking-wider shadow-lg shadow-black/40 ring-1 ring-amber-300/40">
                                        Dragging {storageSelectedHandles.size} items
                                    </div>
                                )}
                                <div
                                    className="grid content-start"
                                    style={{
                                        gridTemplateColumns: GRID_TEMPLATE_COLUMNS,
                                        gap: GAP_PX,
                                    }}
                                >
                                    {storageGridCells.map((item, localIdx) =>
                                        item != null ? (
                                            <ItemTile
                                                key={item.handle}
                                                item={item}
                                                isDragging={storageDragFrom === localIdx}
                                                isDragOver={storageDragOver === localIdx}
                                                isSelected={storageSelectedHandles.has(item.handle)}
                                                isBlockDragging={storageBlockDragging}
                                                onClick={(e) => handleStorageTileClick(item, e)}
                                                onEditClick={
                                                    activeSortTab === 'weapons'
                                                        ? (e) => {
                                                              e.stopPropagation();
                                                              e.preventDefault();
                                                              openWeaponEditor(item, 'storage');
                                                          }
                                                        : undefined
                                                }
                                                onDragStart={() =>
                                                    handleStorageDragStart(localIdx, item)
                                                }
                                                onDragOver={(e) =>
                                                    handleStorageTileDragOver(e, localIdx)
                                                }
                                                onDrop={() => handleStorageTileDrop(localIdx)}
                                                onDragEnd={handleDragEnd}
                                            />
                                        ) : (
                                            <EmptyCell key={`s-empty-${localIdx}`} />
                                        ),
                                    )}
                                </div>
                            </div>
                        </section>

                        {/* ── Vertical separator ───────────────────────────── */}
                        <div className="w-px shrink-0 bg-border/40" aria-hidden="true" />

                        {/* ── Inventory column (right, sort + drag) ─────────── */}
                        <section className="flex-1 min-w-0 flex flex-col min-h-0 gap-2">
                            <div className="flex items-center justify-between shrink-0 gap-2 min-h-7">
                                <div className="flex items-baseline gap-2">
                                    <h4 className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">
                                        Inventory
                                    </h4>
                                    <span className="text-xs font-bold text-blue-700 tabular-nums whitespace-nowrap">
                                        {previewItems.length} item{previewItems.length === 1 ? '' : 's'}
                                    </span>
                                    {sortMode === 'custom' && (
                                        <span className="text-[9px] font-bold text-amber-400/80 uppercase tracking-widest whitespace-nowrap">
                                            Custom
                                        </span>
                                    )}
                                    {hasChanges && (
                                        <span
                                            title="Preview order differs from saved order. Click Apply Order to persist."
                                            className="text-[10px] font-black uppercase tracking-wider text-amber-300 bg-amber-500/15 border border-amber-500/30 rounded px-2 py-0.5 whitespace-nowrap"
                                        >
                                            Unsaved
                                        </span>
                                    )}
                                </div>
                                <div className="flex items-center gap-2">
                                    <button
                                        disabled={busy || previewItems.length === 0}
                                        onClick={resetPreview}
                                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40 disabled:cursor-not-allowed"
                                    >
                                        Reset Preview
                                    </button>
                                    <button
                                        disabled={!hasChanges || busy}
                                        title={
                                            applying
                                                ? 'Applying…'
                                                : hasChanges
                                                  ? 'Apply this order to your save'
                                                  : 'No order changes to apply.'
                                        }
                                        onClick={openInventoryApplyConfirm}
                                        className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                            hasChanges && !busy
                                                ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                                : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                                        }`}
                                    >
                                        {applying ? 'Applying…' : 'Apply Order'}
                                    </button>
                                </div>
                            </div>

                            <div
                                className={`relative shrink-0 mx-auto rounded-xl border bg-background/40 overflow-y-auto transition-colors ${
                                    frameDropTarget === 'inventory'
                                        ? 'border-amber-400/70 ring-2 ring-amber-400/40'
                                        : 'border-border/50'
                                }`}
                                style={{
                                    width: FRAME_WIDTH_PX,
                                    height: FRAME_HEIGHT_PX,
                                    padding: PAD_PX,
                                }}
                                onDragOver={(e) => {
                                    if (dragSourceRef.current === 'storage') {
                                        e.preventDefault();
                                        if (frameDropTarget !== 'inventory') setFrameDropTarget('inventory');
                                    }
                                }}
                                onDragLeave={(e) => {
                                    const related = e.relatedTarget as Node | null;
                                    if (!related || !e.currentTarget.contains(related)) {
                                        if (frameDropTarget === 'inventory') setFrameDropTarget(null);
                                    }
                                }}
                                onDrop={(e) => {
                                    // Only handle Storage→Inventory transfer here.
                                    // Inventory→Inventory reorder is handled by tile-level
                                    // onDrop handlers earlier in the bubble chain.
                                    if (dragSourceRef.current !== 'storage') return;
                                    e.preventDefault();
                                    const handle = draggedStorageHandleRef.current;
                                    if (handle != null) {
                                        const useBatch =
                                            storageSelectedHandles.has(handle) &&
                                            storageSelectedHandles.size > 1;
                                        const handles = useBatch
                                            ? previewStorageItems
                                                  .filter((it) =>
                                                      storageSelectedHandles.has(it.handle),
                                                  )
                                                  .map((it) => it.handle)
                                            : [handle];
                                        handleCrossTransfer('to-inventory', handles);
                                    }
                                    setFrameDropTarget(null);
                                }}
                            >
                                {isBlockDragging && selectedHandles.size > 1 && (
                                    <div className="absolute top-2 left-1/2 -translate-x-1/2 z-20 pointer-events-none px-3 py-1 rounded-full bg-amber-500/95 text-white text-[10px] font-black uppercase tracking-wider shadow-lg shadow-black/40 ring-1 ring-amber-300/40">
                                        Dragging {selectedHandles.size} items
                                    </div>
                                )}
                                <div
                                    className="grid content-start"
                                    style={{
                                        gridTemplateColumns: GRID_TEMPLATE_COLUMNS,
                                        gap: GAP_PX,
                                    }}
                                >
                                    {inventoryGridCells.map((item, localIdx) =>
                                        item != null ? (
                                            <ItemTile
                                                key={item.handle}
                                                item={item}
                                                isDragging={dragFrom === localIdx}
                                                isDragOver={dragOver === localIdx}
                                                isSelected={selectedHandles.has(item.handle)}
                                                isBlockDragging={isBlockDragging}
                                                onClick={(e) => handleTileClick(item, e)}
                                                onEditClick={
                                                    activeSortTab === 'weapons'
                                                        ? (e) => {
                                                              e.stopPropagation();
                                                              e.preventDefault();
                                                              openWeaponEditor(item, 'inventory');
                                                          }
                                                        : undefined
                                                }
                                                onDragStart={() => handleDragStart(localIdx, item)}
                                                onDragOver={(e) => handleDragOver(e, localIdx)}
                                                onDrop={() => handleDrop(localIdx)}
                                                onDragEnd={handleDragEnd}
                                            />
                                        ) : (
                                            <EmptyCell key={`empty-${localIdx}`} />
                                        ),
                                    )}
                                </div>
                            </div>
                        </section>
                    </div>
                )}
            </div>
        </>
    );
}

// ── Sub-components ─────────────────────────────────────────────────────────────

interface TileProps {
    item: main.InventoryOrderItem;
    isDragging: boolean;
    isDragOver: boolean;
    isSelected: boolean;
    isBlockDragging: boolean;
    onClick: (e: React.MouseEvent) => void;
    onEditClick?: (e: React.MouseEvent) => void;
    onDragStart: () => void;
    onDragOver: (e: React.DragEvent) => void;
    onDrop: () => void;
    onDragEnd: () => void;
}

function ItemTile({
    item,
    isDragging,
    isDragOver,
    isSelected,
    isBlockDragging,
    onClick,
    onEditClick,
    onDragStart,
    onDragOver,
    onDrop,
    onDragEnd,
}: TileProps) {
    const [imgError, setImgError] = useState(false);

    const upgradeLabel =
        item.currentUpgrade && item.currentUpgrade > 0
            ? item.infusionName
                ? `${item.infusionName} +${item.currentUpgrade}`
                : `+${item.currentUpgrade}`
            : item.infusionName || null;

    const weightLabel = item.weight && item.weight > 0 ? `${item.weight.toFixed(1)}` : null;
    const typeLabel = item.sortId && item.sortId > 0 ? `T${item.sortGroupId}` : null;

    const tooltip = [
        item.name,
        upgradeLabel,
        weightLabel ? `${weightLabel} kg` : null,
        typeLabel,
        `#${item.acquisitionIndex}`,
    ]
        .filter(Boolean)
        .join(' · ');

    const showIcon = !!item.iconPath && !imgError;

    return (
        <div
            title={tooltip}
            draggable
            onClick={onClick}
            onDragStart={onDragStart}
            onDragOver={onDragOver}
            onDrop={onDrop}
            onDragEnd={onDragEnd}
            className={`relative aspect-square bg-card border rounded-md overflow-hidden transition-all cursor-grab active:cursor-grabbing ${
                isDragging
                    ? isBlockDragging
                        ? 'opacity-50 border-amber-400/60 ring-2 ring-amber-400/40 bg-amber-400/[0.06]'
                        : 'opacity-40 border-border/20'
                    : isDragOver
                      ? 'border-primary ring-1 ring-primary/50 bg-primary/[0.06]'
                      : isBlockDragging && isSelected
                        ? 'border-amber-500/80 ring-2 ring-amber-500/70 bg-amber-500/[0.15]'
                        : isSelected
                          ? 'border-amber-400/70 ring-2 ring-amber-400/50 bg-amber-400/[0.08]'
                          : 'border-border/50 hover:border-primary/40 hover:bg-primary/[0.03]'
            }`}
        >
            {/* absolute inset-0 so content never affects cell height */}
            <div className="absolute inset-0 flex flex-col items-center p-1 gap-0.5">
                {/* Icon / fallback avatar — grows to fill available space */}
                <div className="flex-1 min-h-0 flex items-center justify-center w-full overflow-hidden">
                    {showIcon ? (
                        <img
                            src={item.iconPath}
                            alt=""
                            draggable={false}
                            className="max-w-full max-h-full object-contain drop-shadow-sm"
                            onError={() => setImgError(true)}
                        />
                    ) : (
                        <span className="text-xl font-black text-muted-foreground/35 select-none leading-none">
                            {item.name.charAt(0).toUpperCase()}
                        </span>
                    )}
                </div>

                {/* Name + upgrade pinned to bottom, never expands tile */}
                <div className="w-full shrink-0 overflow-hidden">
                    <div className="text-[8px] font-bold text-foreground/60 truncate text-center leading-tight">
                        {item.name}
                    </div>
                    {upgradeLabel && (
                        <div className="text-[7px] font-mono text-primary/50 truncate text-center leading-tight">
                            {upgradeLabel}
                        </div>
                    )}
                </div>
            </div>

            {/* Weight badge — top-right corner, hidden when weight is 0 */}
            {weightLabel && (
                <div className="absolute top-0.5 right-0.5 px-0.5 rounded text-[6px] font-mono font-bold text-blue-300/70 leading-tight pointer-events-none">
                    {weightLabel}
                </div>
            )}

            {/* Edit-weapon button — top-left, only rendered when onEditClick is provided
                (i.e. weapons tab). Stops propagation so it never triggers selection or DnD. */}
            {onEditClick && (
                <button
                    type="button"
                    draggable={false}
                    onClick={onEditClick}
                    onPointerDown={(e) => e.stopPropagation()}
                    onMouseDown={(e) => e.stopPropagation()}
                    onDragStart={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                    }}
                    title="Edit weapon"
                    aria-label="Edit weapon"
                    className="absolute top-0.5 left-0.5 z-10 w-4 h-4 flex items-center justify-center rounded bg-red-700/85 hover:bg-red-600 text-white shadow ring-1 ring-red-900/40 transition-colors cursor-pointer"
                >
                    <svg
                        className="w-2.5 h-2.5"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2.5"
                        viewBox="0 0 24 24"
                        aria-hidden="true"
                    >
                        <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            d="M14.7 6.3l3 3M4 20l3.5-1 9.8-9.8a2.1 2.1 0 0 0 0-3l-.5-.5a2.1 2.1 0 0 0-3 0L4 16.5 4 20z"
                        />
                    </svg>
                </button>
            )}
        </div>
    );
}

function EmptyCell() {
    return (
        <div
            aria-hidden="true"
            className="relative aspect-square rounded-md border border-dashed border-border/60 bg-card/40 opacity-75 pointer-events-none shadow-inner"
            style={{
                backgroundImage:
                    'linear-gradient(135deg, rgba(255,255,255,0.06) 0%, rgba(255,255,255,0.018) 45%, rgba(0,0,0,0.10) 100%), radial-gradient(circle at 50% 42%, rgba(255,255,255,0.08), transparent 45%)',
            }}
        />
    );
}

// ── Helpers ────────────────────────────────────────────────────────────────────

function sortByMode(items: main.InventoryOrderItem[], mode: SortMode): main.InventoryOrderItem[] {
    const arr = [...items];
    switch (mode) {
        case 'acquisition-asc':
            return arr.sort((a, b) => a.acquisitionIndex - b.acquisitionIndex);
        case 'acquisition-desc':
            return arr.sort((a, b) => b.acquisitionIndex - a.acquisitionIndex);
        case 'weight-asc':
            return arr.sort((a, b) => {
                const wa = a.weight ?? 0;
                const wb = b.weight ?? 0;
                if (wa === 0 && wb === 0) return cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
                if (wa === 0) return 1;
                if (wb === 0) return -1;
                return wa - wb || cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
            });
        case 'weight-desc':
            return arr.sort((a, b) => {
                const wa = a.weight ?? 0;
                const wb = b.weight ?? 0;
                if (wa === 0 && wb === 0) return cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
                if (wa === 0) return 1;
                if (wb === 0) return -1;
                return wb - wa || cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
            });
        case 'type-asc':
            return arr.sort((a, b) => {
                const sa = a.sortId ?? 0;
                const sb = b.sortId ?? 0;
                if (sa === 0 && sb === 0) return cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
                if (sa === 0) return 1;
                if (sb === 0) return -1;
                const ga = a.sortGroupId ?? 0;
                const gb = b.sortGroupId ?? 0;
                return ga - gb || sa - sb || cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
            });
        case 'type-desc':
            return arr.sort((a, b) => {
                const sa = a.sortId ?? 0;
                const sb = b.sortId ?? 0;
                if (sa === 0 && sb === 0) return cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
                if (sa === 0) return 1;
                if (sb === 0) return -1;
                const ga = a.sortGroupId ?? 0;
                const gb = b.sortGroupId ?? 0;
                return gb - ga || sb - sa || cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
            });
        default:
            return arr;
    }
}

function cmpName(a: main.InventoryOrderItem, b: main.InventoryOrderItem): number {
    return a.name.localeCompare(b.name);
}

