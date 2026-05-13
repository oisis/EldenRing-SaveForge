import { useState, useEffect, useMemo } from 'react';
import { GetInventoryOrder, ReorderInventory } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
import toast from '../lib/toast';

const PAGE_COLS = 5;
const PAGE_ROWS = 6;
const PAGE_SIZE = PAGE_COLS * PAGE_ROWS; // 30

type SortOrderTabKey = 'weapons' | 'talismans' | 'head' | 'chest' | 'arms' | 'legs';
type SortMode = 'acquisition-asc' | 'acquisition-desc' | 'weight-asc' | 'weight-desc' | 'custom';

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
    const [sortMode, setSortMode] = useState<SortMode>('acquisition-asc');
    const [page, setPage] = useState(0);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [applying, setApplying] = useState(false);
    const [confirmOpen, setConfirmOpen] = useState(false);
    const [dragFrom, setDragFrom] = useState<number | null>(null);
    const [dragOver, setDragOver] = useState<number | null>(null);

    useEffect(() => {
        setLoading(true);
        setError(null);
        setPage(0);
        setSortMode('acquisition-asc');
        setDragFrom(null);
        setDragOver(null);
        GetInventoryOrder(charIndex, activeSortTab)
            .then((data) => {
                const sorted = sortByMode(data, 'acquisition-asc');
                setBaseItems(sorted);
                setPreviewItems(sorted);
            })
            .catch((err: unknown) => setError(String(err)))
            .finally(() => setLoading(false));
    }, [charIndex, inventoryVersion, activeSortTab]);

    const hasChanges = useMemo(() => {
        if (previewItems.length !== baseItems.length) return false;
        return previewItems.some((item, i) => item.handle !== baseItems[i].handle);
    }, [previewItems, baseItems]);

    const applySort = (mode: SortMode) => {
        setPreviewItems(sortByMode(previewItems, mode));
        setSortMode(mode);
        setPage(0);
    };

    const resetPreview = () => {
        setPreviewItems(sortByMode(baseItems, 'acquisition-asc'));
        setSortMode('acquisition-asc');
        setPage(0);
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
                setPage(0);
            })
            .catch((err: unknown) => {
                toast.error(`Failed to apply ${displayName} order: ` + String(err));
            })
            .finally(() => setApplying(false));
    };

    // ── DnD handlers ──────────────────────────────────────────────────────────

    const pageStart = page * PAGE_SIZE;

    const handleDragStart = (localIdx: number) => {
        setDragFrom(localIdx);
        setDragOver(null);
    };

    const handleDragOver = (e: React.DragEvent, localIdx: number) => {
        e.preventDefault();
        if (dragFrom !== null && dragFrom !== localIdx) {
            setDragOver(localIdx);
        }
    };

    const handleDrop = (localIdx: number) => {
        if (dragFrom === null || dragFrom === localIdx) {
            setDragFrom(null);
            setDragOver(null);
            return;
        }
        const globalFrom = pageStart + dragFrom;
        const globalTo = pageStart + localIdx;
        const next = [...previewItems];
        const [moved] = next.splice(globalFrom, 1);
        next.splice(globalTo, 0, moved);
        setPreviewItems(next);
        setSortMode('custom');
        setDragFrom(null);
        setDragOver(null);
    };

    const handleDragEnd = () => {
        setDragFrom(null);
        setDragOver(null);
    };

    const busy = applying;
    const activeLabel = SORT_TABS.find((t) => t.key === activeSortTab)!.label;

    // ── Grid data ─────────────────────────────────────────────────────────────

    const totalPages = Math.max(1, Math.ceil(previewItems.length / PAGE_SIZE));
    const pageItems = previewItems.slice(pageStart, pageStart + PAGE_SIZE);
    const gridCells: (main.InventoryOrderItem | null)[] = [
        ...pageItems,
        ...Array<null>(Math.max(0, PAGE_SIZE - pageItems.length)).fill(null),
    ];

    return (
        <>
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

            <div className="flex flex-col h-full min-h-0 gap-2">
                {/* ── Category tab selector ────────────────────────────────── */}
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

                {/* ── Empty state ───────────────────────────────────────────── */}
                {!loading && !error && baseItems.length === 0 && (
                    <div className="flex-1 flex items-center justify-center text-[10px] text-muted-foreground text-center px-4">
                        No items in this tab.
                    </div>
                )}

                {/* ── Main content ─────────────────────────────────────────── */}
                {!loading && !error && baseItems.length > 0 && (
                    <>
                        {/* ── Controls row ────────────────────────────────── */}
                        <div className="flex items-center justify-between shrink-0 gap-4">
                            <div className="flex items-center gap-2 flex-wrap">
                                <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground whitespace-nowrap">
                                    Preview:
                                </span>
                                <button
                                    disabled={busy}
                                    onClick={() => applySort('acquisition-asc')}
                                    className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                        sortMode === 'acquisition-asc'
                                            ? 'bg-green-700/80 text-white shadow-sm'
                                            : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                                    }`}
                                >
                                    Acquisition ↑
                                </button>
                                <button
                                    disabled={busy}
                                    onClick={() => applySort('acquisition-desc')}
                                    className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                        sortMode === 'acquisition-desc'
                                            ? 'bg-green-700/80 text-white shadow-sm'
                                            : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                                    }`}
                                >
                                    Acquisition ↓
                                </button>
                                <button
                                    disabled={busy}
                                    onClick={() => applySort('weight-asc')}
                                    className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                        sortMode === 'weight-asc'
                                            ? 'bg-blue-700/80 text-white shadow-sm'
                                            : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                                    }`}
                                >
                                    Weight ↑
                                </button>
                                <button
                                    disabled={busy}
                                    onClick={() => applySort('weight-desc')}
                                    className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                        sortMode === 'weight-desc'
                                            ? 'bg-blue-700/80 text-white shadow-sm'
                                            : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                                    }`}
                                >
                                    Weight ↓
                                </button>
                                {sortMode === 'custom' && (
                                    <span className="text-[9px] font-bold text-amber-400/80 uppercase tracking-widest whitespace-nowrap">
                                        Custom
                                    </span>
                                )}
                                <button
                                    disabled={busy}
                                    onClick={resetPreview}
                                    className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40 disabled:cursor-not-allowed"
                                >
                                    Reset Preview
                                </button>
                            </div>

                            <button
                                disabled={!hasChanges || busy}
                                title={
                                    applying
                                        ? 'Applying…'
                                        : hasChanges
                                          ? 'Apply this order to your save'
                                          : 'No order changes to apply.'
                                }
                                onClick={() => setConfirmOpen(true)}
                                className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                    hasChanges && !busy
                                        ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                        : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                                }`}
                            >
                                {applying ? 'Applying…' : 'Apply Order'}
                            </button>
                        </div>

                        {/* ── Info banner ─────────────────────────────────── */}
                        <div className="shrink-0">
                            {hasChanges ? (
                                <div className="flex items-center gap-2 px-3 py-1.5 bg-amber-500/10 border border-amber-500/20 rounded-md">
                                    <span className="text-amber-400 text-[11px]">⚠</span>
                                    <span className="text-[10px] text-amber-400">
                                        Preview order differs from saved order. Click Apply Order to save.
                                        {(sortMode === 'weight-asc' || sortMode === 'weight-desc') &&
                                            ' Weight sort is a preview — Apply Order to persist it.'}
                                    </span>
                                </div>
                            ) : (
                                <div className="flex items-center gap-2 px-3 py-1.5 bg-muted/10 border border-border/30 rounded-md">
                                    <span className="text-muted-foreground/60 text-[11px]">ℹ</span>
                                    <span className="text-[10px] text-muted-foreground/70">
                                        {activeSortTab === 'weapons'
                                            ? 'Drag items to reorder. Sort Order changes how weapons appear under in-game Acquisition Order. Storage is not affected.'
                                            : 'Drag items to reorder. Reorder items within this equipment tab. Storage is not affected.'}
                                    </span>
                                </div>
                            )}
                        </div>

                        {/* ── 5×6 grid ────────────────────────────────────── */}
                        {/*
                            flex-1 min-h-0: takes all remaining height without overflow.
                            Inner wrapper uses aspect-ratio 5:6 so that each of the 5×6
                            cells is exactly square: cell_w = height*(5/6)/5 = height/6 = cell_h.
                            No overflow-y — all 30 slots always visible.
                        */}
                        <div className="flex-1 min-h-0 flex justify-center items-start overflow-hidden">
                            <div className="h-full" style={{ aspectRatio: '5 / 6' }}>
                                <div className="grid grid-cols-5 grid-rows-6 h-full gap-1">
                                    {gridCells.map((item, localIdx) =>
                                        item != null ? (
                                            <ItemTile
                                                key={item.handle}
                                                item={item}
                                                isDragging={dragFrom === localIdx}
                                                isDragOver={dragOver === localIdx}
                                                onDragStart={() => handleDragStart(localIdx)}
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
                        </div>

                        {/* ── Pagination ──────────────────────────────────── */}
                        {totalPages > 1 && (
                            <div className="flex items-center gap-3 justify-center shrink-0 py-1">
                                <button
                                    disabled={page === 0 || busy}
                                    onClick={() => setPage((p) => p - 1)}
                                    className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-30 disabled:cursor-not-allowed text-muted-foreground hover:text-foreground hover:bg-muted/40"
                                >
                                    ← Previous
                                </button>
                                <span className="text-[10px] font-bold text-muted-foreground tabular-nums">
                                    Page {page + 1} / {totalPages}
                                </span>
                                <button
                                    disabled={page >= totalPages - 1 || busy}
                                    onClick={() => setPage((p) => p + 1)}
                                    className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-30 disabled:cursor-not-allowed text-muted-foreground hover:text-foreground hover:bg-muted/40"
                                >
                                    Next →
                                </button>
                            </div>
                        )}
                    </>
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
    onDragStart: () => void;
    onDragOver: (e: React.DragEvent) => void;
    onDrop: () => void;
    onDragEnd: () => void;
}

function ItemTile({ item, isDragging, isDragOver, onDragStart, onDragOver, onDrop, onDragEnd }: TileProps) {
    const [imgError, setImgError] = useState(false);

    const upgradeLabel =
        item.currentUpgrade && item.currentUpgrade > 0
            ? item.infusionName
                ? `${item.infusionName} +${item.currentUpgrade}`
                : `+${item.currentUpgrade}`
            : item.infusionName || null;

    const weightLabel = item.weight && item.weight > 0 ? `${item.weight.toFixed(1)}` : null;

    const tooltip = [
        item.name,
        upgradeLabel,
        weightLabel ? `${weightLabel} kg` : null,
        `#${item.acquisitionIndex}`,
    ]
        .filter(Boolean)
        .join(' · ');

    const showIcon = !!item.iconPath && !imgError;

    return (
        <div
            title={tooltip}
            draggable
            onDragStart={onDragStart}
            onDragOver={onDragOver}
            onDrop={onDrop}
            onDragEnd={onDragEnd}
            className={`relative bg-card border rounded-md overflow-hidden transition-all cursor-grab active:cursor-grabbing ${
                isDragging
                    ? 'opacity-40 border-border/20'
                    : isDragOver
                      ? 'border-primary ring-1 ring-primary/50 bg-primary/[0.06]'
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
        </div>
    );
}

function EmptyCell() {
    return (
        <div className="relative bg-card/20 border border-border/15 rounded-md opacity-25" />
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
        default:
            return arr;
    }
}

function cmpName(a: main.InventoryOrderItem, b: main.InventoryOrderItem): number {
    return a.name.localeCompare(b.name);
}
