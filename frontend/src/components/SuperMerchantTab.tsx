import {useCallback, useEffect, useMemo, useState} from 'react';
import toast from '../lib/toast';
import {ApplyShopChanges, GetItemList, GetShopMerchants, GetShopStock} from '../../wailsjs/go/main/App';
import type {application, core, db} from '../../wailsjs/go/models';
import {RiskBadge} from './RiskBadge';
import {loadSafetyProfile, revealsRiskyItems, SAFETY_PROFILE_EVENT, type SafetyProfile} from '../state/safetyProfile';

// Super Merchant edits the shop stock stored in the save's embedded regulation.
// Version and platform gating lives entirely in the backend: when the loaded
// save is not a supported PC Regulation 1.17, GetShopMerchants returns an error
// and this tab renders that message instead of an editor.

interface SuperMerchantTabProps {
    platform: string | null;
}

interface RowDraft {
    itemId: number;
    itemName: string;
    iconPath: string;
    value: number;
    sellQuantity: number;
}

const RISKY_ITEM_FLAGS = ['cut_content', 'ban_risk', 'pre_order', 'dlc_duplicate'];
const MAX_PRICE = 999999;
const MAX_STOCK = 255;

// ShopLineupParam addresses items through equipType; only these five item-ID
// prefixes have a shop equivalent, so anything else is not offerable.
const SHOP_ITEM_PREFIXES = [0x00000000, 0x10000000, 0x20000000, 0x40000000, 0x80000000];
const isShopSellable = (id: number) => SHOP_ITEM_PREFIXES.includes((id >>> 0) & 0xF0000000);

const rowKey = (r: application.ShopStockRow): RowDraft => ({
    itemId: r.itemId,
    itemName: r.itemName,
    iconPath: r.iconPath,
    value: r.value,
    sellQuantity: r.sellQuantity,
});

const sameDraft = (a: RowDraft, b: RowDraft) =>
    a.itemId === b.itemId && a.value === b.value && a.sellQuantity === b.sellQuantity;

export function SuperMerchantTab({platform}: SuperMerchantTabProps) {
    const [merchants, setMerchants] = useState<core.ShopMerchantInfo[]>([]);
    const [selected, setSelected] = useState('');
    const [rows, setRows] = useState<application.ShopStockRow[]>([]);
    const [drafts, setDrafts] = useState<Record<number, RowDraft>>({});
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [applying, setApplying] = useState(false);
    const [pickerRow, setPickerRow] = useState<number | null>(null);
    const [catalog, setCatalog] = useState<db.ItemEntry[]>([]);
    const [search, setSearch] = useState('');
    const [safetyProfile, setSafetyProfile] = useState<SafetyProfile>(() => loadSafetyProfile());

    useEffect(() => {
        const onProfile = () => setSafetyProfile(loadSafetyProfile());
        window.addEventListener(SAFETY_PROFILE_EVENT, onProfile);
        return () => window.removeEventListener(SAFETY_PROFILE_EVENT, onProfile);
    }, []);

    // Merchant list — also the single unsupported-save probe.
    useEffect(() => {
        if (!platform) {
            setMerchants([]);
            setSelected('');
            setError('');
            return;
        }
        let active = true;
        setLoading(true);
        setError('');
        GetShopMerchants()
            .then(list => {
                if (!active) return;
                setMerchants(list ?? []);
                setSelected(prev => (prev && list?.some(m => m.name === prev) ? prev : (list?.[0]?.name ?? '')));
            })
            .catch(e => {
                if (!active) return;
                setMerchants([]);
                setSelected('');
                setError(String(e));
            })
            .finally(() => active && setLoading(false));
        return () => { active = false; };
    }, [platform]);

    const loadStock = useCallback((merchant: string) => {
        if (!merchant) {
            setRows([]);
            return;
        }
        setLoading(true);
        GetShopStock(merchant)
            .then(list => {
                setRows(list ?? []);
                setDrafts({});
                setError('');
            })
            .catch(e => {
                setRows([]);
                setError(String(e));
            })
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => { loadStock(selected); }, [selected, loadStock]);

    const draftFor = useCallback(
        (row: application.ShopStockRow): RowDraft => drafts[row.rowId] ?? rowKey(row),
        [drafts],
    );

    const dirtyRows = useMemo(
        () => rows.filter(r => !sameDraft(draftFor(r), rowKey(r))),
        [rows, draftFor],
    );

    const updateDraft = (row: application.ShopStockRow, patch: Partial<RowDraft>) => {
        setDrafts(prev => ({...prev, [row.rowId]: {...(prev[row.rowId] ?? rowKey(row)), ...patch}}));
    };

    const restoreRow = (row: application.ShopStockRow) => {
        setDrafts(prev => {
            const next = {...prev};
            delete next[row.rowId];
            return next;
        });
    };

    // Item catalog for the swap picker — loaded lazily, once.
    useEffect(() => {
        if (pickerRow === null || catalog.length > 0 || !platform) return;
        // 'all' is a category name, not a platform: GetItemList derives the platform
        // from the loaded save itself.
        GetItemList('all')
            .then(list => setCatalog(list ?? []))
            .catch(e => toast.error(`Could not load item list: ${e}`));
    }, [pickerRow, catalog.length, platform]);

    const pickerItems = useMemo(() => {
        const showRisky = revealsRiskyItems(safetyProfile);
        const q = search.trim().toLowerCase();
        return catalog
            .filter(i => isShopSellable(i.id))
            .filter(i => showRisky || !i.flags?.some(f => RISKY_ITEM_FLAGS.includes(f)))
            .filter(i => !q || i.name.toLowerCase().includes(q))
            .slice(0, 200);
    }, [catalog, safetyProfile, search]);

    const apply = async () => {
        if (dirtyRows.length === 0) return;
        setApplying(true);
        try {
            const edits = dirtyRows.map(r => {
                const d = draftFor(r);
                return {rowId: r.rowId, itemId: d.itemId, value: d.value, sellQuantity: d.sellQuantity} as core.ShopRowEdit;
            });
            await ApplyShopChanges(edits);
            toast.success(`Applied ${edits.length} shop row change(s).`);
            loadStock(selected);
        } catch (e) {
            toast.error(String(e));
        } finally {
            setApplying(false);
        }
    };

    if (!platform) {
        return <div className="flex items-center justify-center h-full text-muted-foreground text-sm">Load a save file first</div>;
    }

    const banner = (
        <div className="shrink-0 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-[10px] leading-relaxed text-amber-300">
            <span className="font-black uppercase tracking-wider">Online warning</span> — editing merchant stock changes the
            regulation data inside this save. It is not guaranteed to be safe for online play. Use an offline character.
        </div>
    );

    if (error && rows.length === 0) {
        return (
            <div className="flex-1 flex flex-col gap-3 min-h-0">
                {banner}
                <div className="flex-1 flex items-center justify-center">
                    <div className="max-w-md rounded-lg border border-border bg-card px-4 py-3 text-center text-[11px] text-muted-foreground"
                        data-testid="super-merchant-unsupported">
                        {error}
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="flex-1 flex flex-col gap-3 min-h-0">
            {banner}

            <div className="flex items-center gap-2 shrink-0">
                <label className="text-[10px] font-black uppercase tracking-wider text-muted-foreground">Merchant</label>
                <select
                    aria-label="Merchant"
                    value={selected}
                    onChange={e => setSelected(e.target.value)}
                    disabled={loading || applying || merchants.length === 0}
                    className="flex-1 max-w-md bg-card border border-border rounded-md px-2 py-1 text-[11px] text-foreground">
                    {merchants.map(m => (
                        <option key={m.name} value={m.name}>{m.name} ({m.editableRows}/{m.rowCount})</option>
                    ))}
                </select>
                {dirtyRows.length > 0 && (
                    <span className="text-[10px] font-black uppercase tracking-wider text-primary" data-testid="super-merchant-dirty">
                        {dirtyRows.length} unsaved change(s)
                    </span>
                )}
            </div>

            <div className="flex-1 overflow-y-auto custom-scrollbar pr-2 space-y-1.5 min-h-0">
                {loading && <div className="py-8 text-center text-[11px] text-muted-foreground">Loading merchant stock...</div>}
                {!loading && rows.length === 0 && (
                    <div className="py-8 text-center text-[11px] text-muted-foreground">This merchant has no editable rows.</div>
                )}
                {!loading && rows.map(row => {
                    const d = draftFor(row);
                    const dirty = !sameDraft(d, rowKey(row));
                    const locked = !row.editable;
                    return (
                        <div key={row.rowId} data-testid={`shop-row-${row.rowId}`}
                            className={`flex items-center gap-2 rounded-lg border px-2 py-1.5 bg-card ${dirty ? 'border-primary/60' : 'border-border/50'}`}>
                            {d.iconPath
                                ? <img src={d.iconPath} alt="" className="w-8 h-8 object-contain shrink-0" />
                                : <div className="w-8 h-8 shrink-0 rounded bg-muted/40" />}
                            <div className="min-w-0 flex-1">
                                <div className="truncate text-[11px] font-bold text-foreground">{d.itemName}</div>
                                <div className="text-[9px] text-muted-foreground font-mono">
                                    row {row.rowId}{locked && row.lockReason ? ` — ${row.lockReason}` : ''}
                                </div>
                            </div>
                            <button
                                onClick={() => { setSearch(''); setPickerRow(row.rowId); }}
                                disabled={locked || applying}
                                className="px-2 py-1 rounded text-[9px] font-black uppercase tracking-wider border border-border/50 text-muted-foreground hover:border-primary/50 hover:text-foreground disabled:opacity-40 disabled:cursor-not-allowed">
                                Change item
                            </button>
                            <label className="flex items-center gap-1 text-[9px] text-muted-foreground">
                                Price
                                <input type="number" min={0} max={MAX_PRICE}
                                    aria-label={`Price for row ${row.rowId}`}
                                    value={d.value}
                                    disabled={locked || applying}
                                    onChange={e => updateDraft(row, {value: Math.max(0, Math.min(MAX_PRICE, parseInt(e.target.value) || 0))})}
                                    className="w-20 bg-background border border-border rounded px-1 py-0.5 text-[10px] text-foreground tabular-nums disabled:opacity-40" />
                            </label>
                            <label className="flex items-center gap-1 text-[9px] text-muted-foreground">
                                Stock
                                <input type="number" min={-1} max={MAX_STOCK}
                                    aria-label={`Stock for row ${row.rowId}`}
                                    value={d.sellQuantity}
                                    disabled={locked || applying}
                                    onChange={e => {
                                        const raw = parseInt(e.target.value);
                                        const v = Number.isNaN(raw) ? -1 : Math.max(-1, Math.min(MAX_STOCK, raw));
                                        updateDraft(row, {sellQuantity: v});
                                    }}
                                    className="w-16 bg-background border border-border rounded px-1 py-0.5 text-[10px] text-foreground tabular-nums disabled:opacity-40" />
                            </label>
                            <button
                                onClick={() => restoreRow(row)}
                                disabled={!dirty || applying}
                                title="Restore this row to the values read from the save"
                                className="px-2 py-1 rounded text-[9px] font-black uppercase tracking-wider border border-border/50 text-muted-foreground hover:border-primary/50 hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed">
                                Restore
                            </button>
                        </div>
                    );
                })}
            </div>

            <div className="flex items-center gap-2 shrink-0">
                <button
                    onClick={apply}
                    disabled={dirtyRows.length === 0 || applying}
                    className="px-4 py-1.5 rounded-md bg-green-700/80 text-white text-[10px] font-black uppercase tracking-wider disabled:opacity-40 disabled:cursor-not-allowed">
                    {applying ? 'Applying...' : 'Apply'}
                </button>
                <button
                    onClick={() => setDrafts({})}
                    disabled={dirtyRows.length === 0 || applying}
                    className="px-3 py-1.5 rounded-md border border-border/60 text-[10px] font-black uppercase tracking-wider text-muted-foreground hover:text-foreground disabled:opacity-40 disabled:cursor-not-allowed">
                    Discard Changes
                </button>
                <button
                    onClick={() => loadStock(selected)}
                    disabled={applying || !selected}
                    title="Re-read this merchant's rows from the loaded save"
                    className="px-3 py-1.5 rounded-md border border-border/60 text-[10px] font-black uppercase tracking-wider text-muted-foreground hover:text-foreground disabled:opacity-40 disabled:cursor-not-allowed">
                    Restore Loaded Values
                </button>
            </div>

            {pickerRow !== null && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
                    onClick={() => setPickerRow(null)}>
                    <div className="bg-background border border-border rounded-xl p-4 w-full max-w-lg mx-4 shadow-2xl flex flex-col gap-2"
                        onClick={e => e.stopPropagation()}>
                        <div className="text-[11px] font-black uppercase tracking-wider text-foreground">Select item</div>
                        <input autoFocus value={search} onChange={e => setSearch(e.target.value)}
                            aria-label="Search items" placeholder="Search..."
                            className="bg-card border border-border rounded-md px-2 py-1 text-[11px] text-foreground" />
                        <div className="max-h-80 overflow-y-auto custom-scrollbar space-y-1">
                            {pickerItems.length === 0 && (
                                <div className="py-6 text-center text-[10px] text-muted-foreground">No matching items.</div>
                            )}
                            {pickerItems.map(item => (
                                <button key={item.id}
                                    onClick={() => {
                                        const row = rows.find(r => r.rowId === pickerRow);
                                        if (row) updateDraft(row, {itemId: item.id, itemName: item.name, iconPath: item.iconPath});
                                        setPickerRow(null);
                                    }}
                                    className="w-full flex items-center gap-2 px-2 py-1 rounded hover:bg-muted/40 text-left">
                                    {item.iconPath
                                        ? <img src={item.iconPath} alt="" className="w-6 h-6 object-contain shrink-0" />
                                        : <div className="w-6 h-6 shrink-0 rounded bg-muted/40" />}
                                    <span className="flex-1 truncate text-[11px] text-foreground">{item.name}</span>
                                    {item.flags?.includes('cut_content') && <RiskBadge flag="cut_content" showInfoIcon={false} />}
                                    {item.flags?.includes('ban_risk') && <RiskBadge flag="ban_risk" showInfoIcon={false} />}
                                </button>
                            ))}
                        </div>
                        <button onClick={() => setPickerRow(null)}
                            className="self-end px-3 py-1 rounded-md border border-border/60 text-[10px] font-black uppercase tracking-wider text-muted-foreground hover:text-foreground">
                            Cancel
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
