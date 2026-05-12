import { useState, useEffect } from 'react';
import { GetWeaponInventoryOrder } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';

const PAGE_COLS = 5;
const PAGE_ROWS = 6;
const PAGE_SIZE = PAGE_COLS * PAGE_ROWS; // 30

interface Props {
    charIndex: number;
    inventoryVersion: number;
}

export function SortOrderTab({ charIndex, inventoryVersion }: Props) {
    const [baseItems, setBaseItems] = useState<main.InventoryOrderItem[]>([]);
    const [previewItems, setPreviewItems] = useState<main.InventoryOrderItem[]>([]);
    const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');
    const [page, setPage] = useState(0);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        setLoading(true);
        setError(null);
        GetWeaponInventoryOrder(charIndex)
            .then((data) => {
                const sorted = [...data].sort((a, b) => a.acquisitionIndex - b.acquisitionIndex);
                setBaseItems(sorted);
                setPreviewItems(sorted);
                setSortDir('asc');
                setPage(0);
            })
            .catch((err: unknown) => setError(String(err)))
            .finally(() => setLoading(false));
    }, [charIndex, inventoryVersion]);

    const applySort = (dir: 'asc' | 'desc') => {
        const sorted = [...previewItems].sort((a, b) =>
            dir === 'asc'
                ? a.acquisitionIndex - b.acquisitionIndex
                : b.acquisitionIndex - a.acquisitionIndex,
        );
        setPreviewItems(sorted);
        setSortDir(dir);
        setPage(0);
    };

    const resetPreview = () => {
        const sorted = [...baseItems].sort((a, b) => a.acquisitionIndex - b.acquisitionIndex);
        setPreviewItems(sorted);
        setSortDir('asc');
        setPage(0);
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center h-48 text-[10px] text-muted-foreground">
                Loading weapon inventory…
            </div>
        );
    }

    if (error) {
        return (
            <div className="flex items-center justify-center h-48">
                <div className="text-[10px] text-red-400 text-center max-w-sm">
                    <div className="font-black uppercase tracking-wider mb-1">Error</div>
                    <div>{error}</div>
                </div>
            </div>
        );
    }

    if (baseItems.length === 0) {
        return (
            <div className="flex items-center justify-center h-48 text-[10px] text-muted-foreground text-center px-4">
                No weapons found in inventory for this character.
            </div>
        );
    }

    const totalPages = Math.max(1, Math.ceil(previewItems.length / PAGE_SIZE));
    const pageStart = page * PAGE_SIZE;
    const pageItems = previewItems.slice(pageStart, pageStart + PAGE_SIZE);
    const gridCells: (main.InventoryOrderItem | null)[] = [
        ...pageItems,
        ...Array<null>(Math.max(0, PAGE_SIZE - pageItems.length)).fill(null),
    ];

    return (
        <div className="flex flex-col gap-3 h-full min-h-0">
            {/* Controls row */}
            <div className="flex items-center justify-between shrink-0 gap-4">
                <div className="flex items-center gap-2">
                    <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground whitespace-nowrap">
                        Preview:
                    </span>
                    <button
                        onClick={() => applySort('asc')}
                        className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                            sortDir === 'asc'
                                ? 'bg-green-700/80 text-white shadow-sm'
                                : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                        }`}
                    >
                        Acquisition ↑
                    </button>
                    <button
                        onClick={() => applySort('desc')}
                        className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                            sortDir === 'desc'
                                ? 'bg-green-700/80 text-white shadow-sm'
                                : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                        }`}
                    >
                        Acquisition ↓
                    </button>
                    <button
                        onClick={resetPreview}
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                    >
                        Reset Preview
                    </button>
                </div>

                <button
                    disabled
                    title="Coming next"
                    className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground"
                >
                    Apply Order
                </button>
            </div>

            {/* Preview-only banner */}
            <div className="flex items-center gap-2 px-3 py-1.5 bg-amber-500/10 border border-amber-500/20 rounded-md shrink-0">
                <span className="text-amber-400 text-[11px]">⚠</span>
                <span className="text-[10px] text-amber-400">
                    Preview only — click Apply Order in a future step to save this order.
                </span>
            </div>

            {/* 5×6 grid */}
            <div className="grid grid-cols-5 gap-1.5 content-start flex-1 overflow-y-auto custom-scrollbar">
                {gridCells.map((item, idx) =>
                    item != null ? (
                        <WeaponTile key={item.handle} item={item} />
                    ) : (
                        <EmptyCell key={`empty-${idx}`} />
                    ),
                )}
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
                <div className="flex items-center gap-3 justify-center shrink-0 py-1">
                    <button
                        disabled={page === 0}
                        onClick={() => setPage((p) => p - 1)}
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-30 disabled:cursor-not-allowed text-muted-foreground hover:text-foreground hover:bg-muted/40"
                    >
                        ← Previous
                    </button>
                    <span className="text-[10px] font-bold text-muted-foreground tabular-nums">
                        Page {page + 1} / {totalPages}
                    </span>
                    <button
                        disabled={page >= totalPages - 1}
                        onClick={() => setPage((p) => p + 1)}
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-30 disabled:cursor-not-allowed text-muted-foreground hover:text-foreground hover:bg-muted/40"
                    >
                        Next →
                    </button>
                </div>
            )}
        </div>
    );
}

function WeaponTile({ item }: { item: main.InventoryOrderItem }) {
    const upgradeLabel =
        item.currentUpgrade && item.currentUpgrade > 0
            ? item.infusionName
                ? `${item.infusionName} +${item.currentUpgrade}`
                : `+${item.currentUpgrade}`
            : item.infusionName || null;

    return (
        <div className="border border-border/40 bg-muted/10 rounded p-1.5 flex flex-col items-center gap-0.5 min-h-[78px] hover:border-border/70 hover:bg-muted/20 transition-colors">
            {/* Icon placeholder — letter avatar */}
            <div className="w-8 h-8 rounded bg-muted/30 flex items-center justify-center text-[13px] font-black text-muted-foreground select-none shrink-0">
                {item.name.charAt(0).toUpperCase()}
            </div>
            {/* Name */}
            <span className="text-[8.5px] font-semibold text-foreground leading-tight text-center line-clamp-2 w-full">
                {item.name}
            </span>
            {/* Upgrade / Infusion */}
            {upgradeLabel && (
                <span className="text-[7.5px] font-bold text-amber-400 text-center leading-tight">
                    {upgradeLabel}
                </span>
            )}
            {/* Acquisition index — debug-style */}
            <span className="text-[7px] text-muted-foreground/50 text-center mt-auto tabular-nums">
                #{item.acquisitionIndex}
            </span>
        </div>
    );
}

function EmptyCell() {
    return (
        <div className="border border-border/15 bg-muted/5 rounded min-h-[78px] opacity-40" />
    );
}
