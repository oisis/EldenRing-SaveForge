import { useState, useEffect, useMemo } from 'react';
import { GetWeaponInventoryOrder, ReorderWeaponInventory } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
import toast from '../lib/toast';

const PAGE_COLS = 5;
const PAGE_ROWS = 6;
const PAGE_SIZE = PAGE_COLS * PAGE_ROWS; // 30

interface Props {
    charIndex: number;
    inventoryVersion: number;
    onMutate?: () => void;
}

export function SortOrderTab({ charIndex, inventoryVersion, onMutate }: Props) {
    const [baseItems, setBaseItems] = useState<main.InventoryOrderItem[]>([]);
    const [previewItems, setPreviewItems] = useState<main.InventoryOrderItem[]>([]);
    const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');
    const [page, setPage] = useState(0);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [applying, setApplying] = useState(false);
    const [confirmOpen, setConfirmOpen] = useState(false);

    useEffect(() => {
        setLoading(true);
        setError(null);
        GetWeaponInventoryOrder(charIndex)
            .then((data) => {
                const sorted = asc(data);
                setBaseItems(sorted);
                setPreviewItems(sorted);
                setSortDir('asc');
                setPage(0);
            })
            .catch((err: unknown) => setError(String(err)))
            .finally(() => setLoading(false));
    }, [charIndex, inventoryVersion]);

    // True when preview handle sequence differs from backend baseline.
    const hasChanges = useMemo(() => {
        if (previewItems.length !== baseItems.length) return false;
        return previewItems.some((item, i) => item.handle !== baseItems[i].handle);
    }, [previewItems, baseItems]);

    const applySort = (dir: 'asc' | 'desc') => {
        const sorted =
            dir === 'asc'
                ? [...previewItems].sort((a, b) => a.acquisitionIndex - b.acquisitionIndex)
                : [...previewItems].sort((a, b) => b.acquisitionIndex - a.acquisitionIndex);
        setPreviewItems(sorted);
        setSortDir(dir);
        setPage(0);
    };

    const resetPreview = () => {
        setPreviewItems(asc(baseItems));
        setSortDir('asc');
        setPage(0);
    };

    const handleApplyConfirm = () => {
        setConfirmOpen(false);
        setApplying(true);
        const handles = previewItems.map((i) => i.handle);
        ReorderWeaponInventory(charIndex, handles)
            .then(() => {
                toast.success('Weapon order updated successfully.');
                onMutate?.();
                return GetWeaponInventoryOrder(charIndex);
            })
            .then((data) => {
                const sorted = asc(data);
                setBaseItems(sorted);
                setPreviewItems(sorted);
                setSortDir('asc');
                setPage(0);
            })
            .catch((err: unknown) => {
                toast.error('Failed to apply weapon order: ' + String(err));
            })
            .finally(() => setApplying(false));
    };

    // ── Loading / error / empty ────────────────────────────────────────────────

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

    // ── Grid / pagination ─────────────────────────────────────────────────────

    const totalPages = Math.max(1, Math.ceil(previewItems.length / PAGE_SIZE));
    const pageStart = page * PAGE_SIZE;
    const pageItems = previewItems.slice(pageStart, pageStart + PAGE_SIZE);
    const gridCells: (main.InventoryOrderItem | null)[] = [
        ...pageItems,
        ...Array<null>(Math.max(0, PAGE_SIZE - pageItems.length)).fill(null),
    ];

    const busy = applying;

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
                                Apply Weapon Order
                            </h3>
                        </div>
                        <p className="text-[11px] text-muted-foreground leading-relaxed mb-5">
                            Apply this weapon order to your save? This rewrites acquisition order
                            for weapon inventory only. Storage is not affected.
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

            <div className="flex flex-col gap-3 h-full min-h-0">
                {/* ── Controls row ──────────────────────────────────────────── */}
                <div className="flex items-center justify-between shrink-0 gap-4">
                    <div className="flex items-center gap-2">
                        <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground whitespace-nowrap">
                            Preview:
                        </span>
                        <button
                            disabled={busy}
                            onClick={() => applySort('asc')}
                            className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                sortDir === 'asc'
                                    ? 'bg-green-700/80 text-white shadow-sm'
                                    : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                            }`}
                        >
                            Acquisition ↑
                        </button>
                        <button
                            disabled={busy}
                            onClick={() => applySort('desc')}
                            className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all disabled:opacity-40 disabled:cursor-not-allowed ${
                                sortDir === 'desc'
                                    ? 'bg-green-700/80 text-white shadow-sm'
                                    : 'text-muted-foreground hover:text-foreground hover:bg-muted/40'
                            }`}
                        >
                            Acquisition ↓
                        </button>
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

                {/* ── Info banners ──────────────────────────────────────────── */}
                <div className="flex flex-col gap-1.5 shrink-0">
                    {hasChanges ? (
                        <div className="flex items-center gap-2 px-3 py-1.5 bg-amber-500/10 border border-amber-500/20 rounded-md">
                            <span className="text-amber-400 text-[11px]">⚠</span>
                            <span className="text-[10px] text-amber-400">
                                Preview order differs from saved order. Click Apply Order to save.
                            </span>
                        </div>
                    ) : (
                        <div className="flex items-center gap-2 px-3 py-1.5 bg-muted/10 border border-border/30 rounded-md">
                            <span className="text-muted-foreground/60 text-[11px]">ℹ</span>
                            <span className="text-[10px] text-muted-foreground/70">
                                Sort Order changes how weapons appear under in-game Acquisition Order. Storage is not affected.
                            </span>
                        </div>
                    )}
                </div>

                {/* ── 5×6 grid ──────────────────────────────────────────────── */}
                <div className="grid grid-cols-5 gap-1.5 content-start flex-1 overflow-y-auto custom-scrollbar">
                    {gridCells.map((item, idx) =>
                        item != null ? (
                            <WeaponTile key={item.handle} item={item} />
                        ) : (
                            <EmptyCell key={`empty-${idx}`} />
                        ),
                    )}
                </div>

                {/* ── Pagination ────────────────────────────────────────────── */}
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
            </div>
        </>
    );
}

// ── Sub-components ─────────────────────────────────────────────────────────────

function WeaponTile({ item }: { item: main.InventoryOrderItem }) {
    const upgradeLabel =
        item.currentUpgrade && item.currentUpgrade > 0
            ? item.infusionName
                ? `${item.infusionName} +${item.currentUpgrade}`
                : `+${item.currentUpgrade}`
            : item.infusionName || null;

    return (
        <div className="border border-border/40 bg-muted/10 rounded p-1.5 flex flex-col items-center gap-0.5 min-h-[78px] hover:border-border/70 hover:bg-muted/20 transition-colors">
            <div className="w-8 h-8 rounded bg-muted/30 flex items-center justify-center text-[13px] font-black text-muted-foreground select-none shrink-0">
                {item.name.charAt(0).toUpperCase()}
            </div>
            <span className="text-[8.5px] font-semibold text-foreground leading-tight text-center line-clamp-2 w-full">
                {item.name}
            </span>
            {upgradeLabel && (
                <span className="text-[7.5px] font-bold text-amber-400 text-center leading-tight">
                    {upgradeLabel}
                </span>
            )}
            <span className="text-[7px] text-muted-foreground/50 text-center mt-auto tabular-nums">
                #{item.acquisitionIndex}
            </span>
        </div>
    );
}

function EmptyCell() {
    return <div className="border border-border/15 bg-muted/5 rounded min-h-[78px] opacity-40" />;
}

// ── Helpers ────────────────────────────────────────────────────────────────────

function asc(items: main.InventoryOrderItem[]): main.InventoryOrderItem[] {
    return [...items].sort((a, b) => a.acquisitionIndex - b.acquisitionIndex);
}
