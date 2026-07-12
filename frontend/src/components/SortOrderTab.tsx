import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { GetItemList, RepairInventoryWorkspaceItems } from '../../wailsjs/go/main/App';
import { db, editor, main } from '../../wailsjs/go/models';
import toast from '../lib/toast';
import { useInventoryWorkspace, ContainerKind } from '../hooks/useInventoryWorkspace';
import { WeaponEditModal } from './WeaponEditModal';

// Phase 8A removed the public JSON template import/export surface from
// this tab. Template export, import, library access and weapon level
// override now live in the global Templates shell modal (sidebar
// button), which speaks YAML only.

// Build a main.InventoryOrderItem-shaped adapter from a workspace EditableItem,
// so the legacy WeaponEditModal can render without changes. Workspace dispatch
// goes through the `workspace` prop, not through these fields.
function adaptForWeaponModal(it: editor.EditableItem): main.InventoryOrderItem {
    return main.InventoryOrderItem.createFrom({
        handle: it.originalHandle,
        itemId: it.itemID,
        baseItemId: it.baseItemID,
        name: it.name,
        category: it.category,
        currentUpgrade: it.currentUpgrade,
        maxUpgrade: it.maxUpgrade,
        infusionName: it.infusionName ?? '',
        quantity: it.quantity,
        acquisitionIndex: it.acquisitionIndex,
        iconPath: it.iconPath ?? '',
        isWeapon: it.isWeapon,
    });
}

type FrameDropTarget = ContainerKind | null;

const GRID_COLS = 5;
const GRID_MIN_ROWS = 6;
const GRID_MIN_CELLS = GRID_COLS * GRID_MIN_ROWS; // 30 — one card

const CELL_PX = 96;
const GAP_PX = 6;
const PAD_PX = 8;
const BORDER_PX = 1; // frame border width; box-sizing is border-box (Tailwind preflight)
const FRAME_WIDTH_PX = GRID_COLS * CELL_PX + (GRID_COLS - 1) * GAP_PX + 2 * PAD_PX;
// One card is a fixed 5×6 grid; the frame shows exactly one card and scrolls
// card-by-card. CARD_STEP is the scroll distance between consecutive card starts
// (card height + the gap that separates stacked cards).
const CARD_HEIGHT_PX = GRID_MIN_ROWS * CELL_PX + (GRID_MIN_ROWS - 1) * GAP_PX;
export const CARD_STEP_PX = CARD_HEIGHT_PX + GAP_PX;
// A physical wheel gesture fires a burst of events whose momentum tail can run
// for hundreds of ms. WHEEL_IDLE_MS is the quiet gap that marks the gesture as
// truly finished; it is reset by every event, so the tail keeps the lock held.
const WHEEL_IDLE_MS = 140;
// Fallback for engines that never emit `scrollend`: mark the smooth scroll
// settled well after it would have finished. Release still waits for the idle
// gap too, so this can never expire mid-scroll and cause a double-step.
const WHEEL_SETTLE_FALLBACK_MS = 700;
const AUTO_SCROLL_EDGE_PX = 44;
const AUTO_SCROLL_INITIAL_DELAY_MS = 260;
const AUTO_SCROLL_REPEAT_MS = 360;
// Frame outer height. With border-box the top/bottom borders eat into the box,
// so add them back: the interior (clientHeight) is then exactly one card and a
// single-card frame has scrollHeight === clientHeight (no phantom scroll).
const FRAME_HEIGHT_PX = CARD_HEIGHT_PX + 2 * BORDER_PX;
const GRID_TEMPLATE_COLUMNS = `repeat(${GRID_COLS}, ${CELL_PX}px)`;

type SortOrderTabKey = 'weapons' | 'talismans' | 'head' | 'chest' | 'arms' | 'legs';
// 'default' is an internal, non-dropdown mode: it reproduces the in-game
// section hierarchy (see sortEditableItems) and is what the "Default" ('')
// dropdown choice maps to. The user-selectable modes are in SORT_MODE_OPTIONS.
type SortMode =
    | 'default'
    | 'acquisition-asc' | 'acquisition-desc'
    | 'weight-asc' | 'weight-desc'
    | 'type-asc' | 'type-desc';
type CardScrollDirection = -1 | 0 | 1;

// In-game Weapons Sort Order hierarchy. Top-level section order is fixed
// (melee, then shields, then ranged/catalysts) and deliberately NOT the
// CategorySelect order. Within a section, items group by the canonical DB
// SubCategory order below, then by regulation sortGroupId/sortId.
const WEAPON_CATEGORY_ORDER: Record<string, number> = {
    melee_armaments: 0,
    shields: 1,
    ranged_and_catalysts: 2,
};
// Canonical sub-category order, mirroring backend/db/data/subcategories.go
// declaration order. Each item's subCategory VALUE comes from the DB via the
// workspace snapshot; only the ordering of the known groups lives here (we do
// not infer subcategories from names in the frontend).
const WEAPON_SUBCATEGORY_ORDER: Record<string, number> = Object.fromEntries(
    [
        // melee_armaments
        'Daggers', 'Throwing Blades', 'Straight Swords', 'Light Greatswords', 'Greatswords',
        'Colossal Swords', 'Thrusting Swords', 'Heavy Thrusting Swords', 'Curved Swords',
        'Curved Greatswords', 'Backhand Blades', 'Katanas', 'Great Katanas', 'Twinblades',
        'Axes', 'Greataxes', 'Hammers', 'Great Hammers', 'Flails', 'Spears', 'Great Spears',
        'Halberds', 'Reapers', 'Whips', 'Fists', 'Hand-to-Hand', 'Claws', 'Beast Claws',
        'Colossal Weapons', 'Perfume Bottles',
        // shields
        'Torches', 'Small Shields', 'Medium Shields', 'Greatshields',
        // ranged_and_catalysts
        'Bows', 'Light Bows', 'Greatbows', 'Crossbows', 'Ballistas', 'Glintstone Staffs', 'Sacred Seals',
    ].map((name, idx) => [name, idx]),
);
// Unknown/missing category or subcategory sorts deterministically AFTER every
// known group, never interleaved.
const RANK_AFTER_KNOWN = Number.MAX_SAFE_INTEGER;

function categoryRank(category: string): number {
    return WEAPON_CATEGORY_ORDER[category] ?? RANK_AFTER_KNOWN;
}
function subCategoryRank(subCategory: string | undefined): number {
    if (!subCategory) return RANK_AFTER_KNOWN;
    return WEAPON_SUBCATEGORY_ORDER[subCategory] ?? RANK_AFTER_KNOWN;
}

// Returns the requested card direction only when the pointer is inside a
// frame edge zone and there is another card in that direction.
export function getCardAutoScrollDirection(
    pointerY: number,
    frameTop: number,
    frameBottom: number,
    canScrollUp: boolean,
    canScrollDown: boolean,
): CardScrollDirection {
    if (pointerY <= frameTop + AUTO_SCROLL_EDGE_PX && canScrollUp) return -1;
    if (pointerY >= frameBottom - AUTO_SCROLL_EDGE_PX && canScrollDown) return 1;
    return 0;
}

// Number of 30-cell cards needed to hold `itemCount` items. A full final card
// always gets one extra empty card so there is a visible drop destination after
// it; an empty list still yields one empty card.
function sortOrderCardCount(itemCount: number): number {
    const filled = Math.ceil(itemCount / GRID_MIN_CELLS);
    const trailingEmpty = itemCount % GRID_MIN_CELLS === 0 ? 1 : 0;
    return Math.max(1, filled + trailingEmpty);
}

// Split a flat item list into fixed 5×6 cards, each padded to exactly 30 cells
// with nulls. Pure — the tab and its tests both consume this.
export function buildSortOrderCards(
    items: editor.EditableItem[],
): (editor.EditableItem | null)[][] {
    const cards: (editor.EditableItem | null)[][] = [];
    for (let c = 0; c < sortOrderCardCount(items.length); c++) {
        const slice = items.slice(c * GRID_MIN_CELLS, c * GRID_MIN_CELLS + GRID_MIN_CELLS);
        cards.push([...slice, ...Array<null>(GRID_MIN_CELLS - slice.length).fill(null)]);
    }
    return cards;
}

// Tab → category set. Mirrors backend inventoryOrderTabs in app_inventory_order.go.
const TAB_CATEGORIES: Record<SortOrderTabKey, ReadonlySet<string>> = {
    weapons:   new Set(['melee_armaments', 'ranged_and_catalysts', 'shields']),
    talismans: new Set(['talismans']),
    head:      new Set(['head']),
    chest:     new Set(['chest']),
    arms:      new Set(['arms']),
    legs:      new Set(['legs']),
};

const SORT_TABS: { key: SortOrderTabKey; label: string }[] = [
    { key: 'weapons', label: 'Weapons' },
    { key: 'talismans', label: 'Talismans' },
    { key: 'head', label: 'Head' },
    { key: 'chest', label: 'Chest' },
    { key: 'arms', label: 'Arms' },
    { key: 'legs', label: 'Legs' },
];

const SORT_MODE_OPTIONS: { mode: SortMode; label: string }[] = [
    { mode: 'acquisition-asc', label: 'Acquisition ↑' },
    { mode: 'acquisition-desc', label: 'Acquisition ↓' },
    { mode: 'weight-asc', label: 'Weight ↑' },
    { mode: 'weight-desc', label: 'Weight ↓' },
    { mode: 'type-asc', label: 'Type ↑' },
    { mode: 'type-desc', label: 'Type ↓' },
];

// Unarmed placeholder — backend excludes it from legacy weapons tab, mirror that here.
const UNARMED_BASE_ID = 0x0001ADB0;

interface Props {
    charIndex: number;
    inventoryVersion: number;
    onMutate?: () => void;
}

function tabFilter(it: editor.EditableItem, tab: SortOrderTabKey): boolean {
    if (!TAB_CATEGORIES[tab].has(it.category)) return false;
    if (tab === 'weapons' && it.baseItemID === UNARMED_BASE_ID) return false;
    return true;
}

export function SortOrderTab({ charIndex, inventoryVersion, onMutate }: Props) {
    const [activeSortTab, setActiveSortTab] = useState<SortOrderTabKey>('weapons');
    // Shared sort dropdown: '' is the "Default" (no active choice) state. The
    // label is persisted here so the select keeps showing the applied mode.
    const [sortMode, setSortMode] = useState<SortMode | ''>('');
    // Guards against overlapping shared-sort runs — a second dropdown change
    // while the atomic reorder is still committing would race the snapshot.
    // Ref = synchronous guard; state = visibly disable the Sort select.
    const sortInFlightRef = useRef(false);
    const [sortInFlight, setSortInFlight] = useState(false);
    const [helpOpen, setHelpOpen] = useState(false);
    const [confirmSaveOpen, setConfirmSaveOpen] = useState(false);
    const [confirmDiscardOpen, setConfirmDiscardOpen] = useState(false);

    const workspace = useInventoryWorkspace();
    const { sessionID, inventoryItems, storageItems, dirty, loading, saving, lastError, validation } = workspace;

    // Drag state — only minimal selection / drag refs needed; reorder commits
    // straight to the workspace, so we don't keep a separate preview list.
    const [dragSource, setDragSource] = useState<ContainerKind | null>(null);
    const dragSourceRef = useRef<ContainerKind | null>(null);
    const [dragFromUID, setDragFromUID] = useState<string | null>(null);
    const [dragOverLocal, setDragOverLocal] = useState<number | null>(null);
    const [dragOverContainer, setDragOverContainer] = useState<ContainerKind | null>(null);
    const [frameDropTarget, setFrameDropTarget] = useState<FrameDropTarget>(null);
    const didDragRef = useRef(false);

    const [invSelectedUIDs, setInvSelectedUIDs] = useState<Set<string>>(new Set());
    const [invAnchorUID, setInvAnchorUID] = useState<string | null>(null);
    const [stoSelectedUIDs, setStoSelectedUIDs] = useState<Set<string>>(new Set());
    const [stoAnchorUID, setStoAnchorUID] = useState<string | null>(null);

    // Weapon edit modal
    const [weaponEditor, setWeaponEditor] = useState<{ item: editor.EditableItem; source: ContainerKind } | null>(null);

    // Add-item modal
    const [addOpen, setAddOpen] = useState(false);
    const [addContainer, setAddContainer] = useState<ContainerKind>('inventory');

    // Session lifecycle — start a new workspace session whenever the user
    // switches characters or the upstream save reloads. Re-using the previous
    // session would silently leak its mutations into a different save context.
    useEffect(() => {
        if (dirty) {
            // Dirty workspace from previous save would be replaced silently. Surface this.
            toast('Inventory changes were not saved — they are being discarded.');
        }
        // Starting a session is a read-only op (no slot mutation); do NOT call
        // onMutate here — it would spuriously refresh undo depth on every
        // character switch or inventoryVersion bump.
        workspace.start(charIndex).catch(() => { /* surfaced via lastError */ });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [charIndex, inventoryVersion]);

    // Per-tab filtered views derived from the workspace.
    const inventoryView = useMemo(
        () => inventoryItems.filter(it => tabFilter(it, activeSortTab)),
        [inventoryItems, activeSortTab],
    );
    const storageView = useMemo(
        () => storageItems.filter(it => tabFilter(it, activeSortTab)),
        [storageItems, activeSortTab],
    );
    const inventoryCards = useMemo(() => buildSortOrderCards(inventoryView), [inventoryView]);
    const storageCards = useMemo(() => buildSortOrderCards(storageView), [storageView]);

    // Clear stale selections when the active tab changes — selected UIDs may no
    // longer be visible, which would block keyboard / batch operations.
    useEffect(() => {
        setInvSelectedUIDs(prev => {
            const visible = new Set(inventoryView.map(it => it.uid));
            const next = new Set([...prev].filter(uid => visible.has(uid)));
            return next.size === prev.size ? prev : next;
        });
        setStoSelectedUIDs(prev => {
            const visible = new Set(storageView.map(it => it.uid));
            const next = new Set([...prev].filter(uid => visible.has(uid)));
            return next.size === prev.size ? prev : next;
        });
    }, [activeSortTab, inventoryView, storageView]);

    const clearWorkspaceError = workspace.clearError;
    useEffect(() => {
        if (lastError) {
            toast.error(lastError);
            clearWorkspaceError();
        }
    }, [lastError, clearWorkspaceError]);

    // ── Position translation ──────────────────────────────────────────────────
    // Per-tab indices map to global container positions by reading the source
    // item's position in the full inventoryItems / storageItems list. The
    // backend MoveItem clamps targetPosition into the post-pop slice, so when
    // moving downward we subtract one to account for the source being removed
    // before the insert.
    const globalIndexOf = (container: ContainerKind, uid: string): number => {
        const list = container === 'inventory' ? inventoryItems : storageItems;
        return list.findIndex(it => it.uid === uid);
    };

    const computeTargetPosition = (
        container: ContainerKind,
        sourceUID: string,
        localTo: number,
        pageStart: number,
    ): number => {
        const view = container === 'inventory' ? inventoryView : storageView;
        const fullList = container === 'inventory' ? inventoryItems : storageItems;
        const srcGlobal = globalIndexOf(container, sourceUID);
        if (srcGlobal < 0) return -1;
        let prePopTarget: number;
        const absoluteTo = pageStart + localTo;
        if (absoluteTo < view.length) {
            const targetItem = view[absoluteTo];
            if (targetItem.uid === sourceUID) return srcGlobal; // no-op
            prePopTarget = fullList.findIndex(it => it.uid === targetItem.uid);
        } else {
            // Drop past the last item in the tab — insert just after the last tab item.
            const lastTab = view[view.length - 1];
            if (!lastTab) {
                prePopTarget = fullList.length;
            } else {
                prePopTarget = fullList.findIndex(it => it.uid === lastTab.uid) + 1;
            }
        }
        return prePopTarget > srcGlobal ? prePopTarget - 1 : prePopTarget;
    };

    // ── Selection ─────────────────────────────────────────────────────────────
    const onTileClick = (
        container: ContainerKind,
        item: editor.EditableItem,
        e: React.MouseEvent,
    ) => {
        if (didDragRef.current) {
            didDragRef.current = false;
            return;
        }
        const view = container === 'inventory' ? inventoryView : storageView;
        const selected = container === 'inventory' ? invSelectedUIDs : stoSelectedUIDs;
        const setSelected = container === 'inventory' ? setInvSelectedUIDs : setStoSelectedUIDs;
        const anchor = container === 'inventory' ? invAnchorUID : stoAnchorUID;
        const setAnchor = container === 'inventory' ? setInvAnchorUID : setStoAnchorUID;
        if (e.shiftKey && anchor !== null) {
            const idxA = view.findIndex(it => it.uid === anchor);
            const idxB = view.findIndex(it => it.uid === item.uid);
            if (idxA >= 0 && idxB >= 0) {
                const [lo, hi] = idxA < idxB ? [idxA, idxB] : [idxB, idxA];
                setSelected(new Set(view.slice(lo, hi + 1).map(it => it.uid)));
            }
            return;
        }
        if (e.ctrlKey || e.metaKey) {
            const next = new Set(selected);
            if (next.has(item.uid)) next.delete(item.uid); else next.add(item.uid);
            setSelected(next);
            setAnchor(item.uid);
            return;
        }
        setSelected(new Set([item.uid]));
        setAnchor(item.uid);
    };

    // ── DnD: drag start / over / end ─────────────────────────────────────────
    const setDragSrc = (src: ContainerKind | null) => {
        dragSourceRef.current = src;
        setDragSource(src);
    };

    const onDragStart = (container: ContainerKind, item: editor.EditableItem) => {
        didDragRef.current = true;
        setDragSrc(container);
        setDragFromUID(item.uid);
        setDragOverLocal(null);
        setDragOverContainer(container);
        // If item is not in selection, single-select it.
        if (container === 'inventory') {
            if (!invSelectedUIDs.has(item.uid)) {
                setInvSelectedUIDs(new Set([item.uid]));
                setInvAnchorUID(item.uid);
            }
        } else {
            if (!stoSelectedUIDs.has(item.uid)) {
                setStoSelectedUIDs(new Set([item.uid]));
                setStoAnchorUID(item.uid);
            }
        }
    };

    const onTileDragOver = (e: React.DragEvent, container: ContainerKind, localIdx: number) => {
        if (dragSourceRef.current !== container) return;
        e.preventDefault();
        setDragOverContainer(container);
        setDragOverLocal(localIdx);
    };

    const onDragEnd = () => {
        setDragSrc(null);
        setDragFromUID(null);
        setDragOverLocal(null);
        setDragOverContainer(null);
        setFrameDropTarget(null);
        setTimeout(() => { didDragRef.current = false; }, 0);
    };

    // ── DnD: reorder within container ─────────────────────────────────────────
    const onTileDrop = (container: ContainerKind, localIdx: number, pageStart: number) => {
        if (dragSourceRef.current !== container) return;
        if (!dragFromUID) { onDragEnd(); return; }
        const tgt = computeTargetPosition(container, dragFromUID, localIdx, pageStart);
        if (tgt < 0) { onDragEnd(); return; }
        const srcGlobal = globalIndexOf(container, dragFromUID);
        if (srcGlobal === tgt) { onDragEnd(); return; }
        const uid = dragFromUID;
        onDragEnd();
        workspace.moveItem(uid, container, tgt);
    };

    // ── DnD: cross-grid transfer ──────────────────────────────────────────────
    const onFrameDrop = async (target: ContainerKind) => {
        if (dragSourceRef.current && dragSourceRef.current !== target && dragFromUID) {
            const sourceContainer = dragSourceRef.current;
            const selection = sourceContainer === 'inventory' ? invSelectedUIDs : stoSelectedUIDs;
            const useBatch = selection.has(dragFromUID) && selection.size > 1;
            const uids = useBatch
                ? Array.from(selection)
                : [dragFromUID];
            onDragEnd();
            for (const uid of uids) {
                // Sequential transfer — workspace ops are RAM-only and the
                // returned snapshot already reflects the new container order.
                await workspace.transferItem(uid, target);
            }
        } else {
            onDragEnd();
        }
    };

    // ── Save / Discard ────────────────────────────────────────────────────────
    const onSave = async () => {
        setConfirmSaveOpen(false);
        const next = await workspace.save();
        if (next) {
            toast.success('Inventory changes saved.');
            onMutate?.();
        }
    };

    const onDiscard = async () => {
        setConfirmDiscardOpen(false);
        await workspace.discard();
    };

    // ── Validation summary ───────────────────────────────────────────────────
    const errorCount = validation?.errors?.length ?? 0;
    const warningCount = validation?.warnings?.length ?? 0;

    // ── Render ────────────────────────────────────────────────────────────────
    const activeLabel = SORT_TABS.find((t) => t.key === activeSortTab)!.label;
    const invSelectedHere = inventoryView.filter(it => invSelectedUIDs.has(it.uid));
    const stoSelectedHere = storageView.filter(it => stoSelectedUIDs.has(it.uid));
    const onRemoveSelected = async () => {
        // Phase 5A: remove operates on UIDs visible in the current tab.
        const targets = [...invSelectedHere, ...stoSelectedHere];
        if (targets.length === 0) return;
        for (const it of targets) {
            await workspace.removeItem(it.uid);
        }
        setInvSelectedUIDs(new Set());
        setStoSelectedUIDs(new Set());
    };

    // Computes the full desired UID order for a container under the given
    // sort mode, preserving the positions of items outside the active
    // category exactly (only visible items are reshuffled among their own
    // slots). Returns null when the container is a no-op: fewer than two
    // visible items, or already in the target order.
    const computeDesiredOrder = (container: ContainerKind, mode: SortMode): string[] | null => {
        const fullList = container === 'inventory' ? inventoryItems : storageItems;
        const view = container === 'inventory' ? inventoryView : storageView;
        if (view.length < 2) return null;

        const sortedView = sortEditableItems(view, mode);
        if (sortedView.every((it, idx) => it.uid === view[idx]?.uid)) return null;

        const desired = fullList.map(it => it.uid);
        const visiblePositions = view
            .map((it) => fullList.findIndex(candidate => candidate.uid === it.uid))
            .filter(idx => idx >= 0);
        visiblePositions.forEach((pos, idx) => {
            desired[pos] = sortedView[idx].uid;
        });
        return desired;
    };

    // Shared sort computes both desired UID orders and commits them in ONE
    // atomic backend reorder call, so the UI applies a single final
    // snapshot with no intermediate shuffling. A container that doesn't
    // change keeps its current order (still a valid permutation). Fires one
    // success toast; failures surface via workspace.lastError, no false toast.
    const applySortBoth = async (mode: SortMode) => {
        const invOrder = computeDesiredOrder('inventory', mode);
        const stoOrder = computeDesiredOrder('storage', mode);
        if (!invOrder && !stoOrder) return; // both no-op
        const inventoryUIDs = invOrder ?? inventoryItems.map(it => it.uid);
        const storageUIDs = stoOrder ?? storageItems.map(it => it.uid);
        const ok = await workspace.reorderItems(inventoryUIDs, storageUIDs);
        if (ok) toast.success('Inventory sorted.');
    };

    // Single entry point for the shared Sort select. Persists the chosen label
    // and blocks overlapping runs (ref for the synchronous guard, state to
    // visibly disable the select while running). 'Default' ('') actively
    // restores the in-game weapon section hierarchy via the 'default' sort
    // (category → sub-category → regulation order), NOT acquisition/pickup
    // order and NOT a flat regulation sort.
    const onSortModeChange = async (mode: SortMode | '') => {
        if (sortInFlightRef.current) return; // ignore while a sort is committing
        setSortMode(mode);
        sortInFlightRef.current = true;
        setSortInFlight(true);
        try {
            await applySortBoth(mode === '' ? 'default' : mode);
        } finally {
            sortInFlightRef.current = false;
            setSortInFlight(false);
        }
    };

    return (
        <>
            {helpOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setHelpOpen(false)}>
                    <div className="bg-background border border-border rounded-xl p-6 w-[520px] shadow-2xl" onClick={(e) => e.stopPropagation()}>
                        <div className="flex items-center gap-2 mb-4">
                            <div className="w-1 h-4 bg-primary rounded-full" />
                            <h3 className="text-[10px] font-black uppercase tracking-widest">Inventory Edit Session — Help</h3>
                        </div>
                        <ul className="text-[11px] text-muted-foreground leading-relaxed mb-5 space-y-1.5 list-disc pl-4">
                            <li>Reorder, transfer, add, remove and edit weapons all happen in a RAM-only session.</li>
                            <li>Nothing touches your save file until you press <span className="font-bold">Save changes</span>.</li>
                            <li>Press <span className="font-bold">Discard changes</span> to start the session over from the current save state.</li>
                            <li>Switch character or reload the save to abandon the current session.</li>
                            <li>Drag inside a grid to reorder; drag across grids to transfer between Inventory and Storage.</li>
                        </ul>
                        <div className="flex justify-end">
                            <button onClick={() => setHelpOpen(false)} className="px-4 py-1.5 text-[10px] font-black uppercase tracking-wider rounded bg-primary/15 text-primary border border-primary/30 hover:bg-primary/20 transition-all">Close</button>
                        </div>
                    </div>
                </div>
            )}

            {confirmSaveOpen && (
                <ConfirmModal
                    title="Save Inventory Changes"
                    body="Persist all pending inventory/storage edits to the save file? This wraps reorder, add, remove, transfer and weapon edits into a single write."
                    confirmLabel="Save"
                    onConfirm={onSave}
                    onCancel={() => setConfirmSaveOpen(false)}
                    confirmTone="green"
                />
            )}

            {confirmDiscardOpen && (
                <ConfirmModal
                    title="Discard Inventory Changes"
                    body="Throw away all pending edits and reload from the current save? This cannot be undone within the session."
                    confirmLabel="Discard"
                    onConfirm={onDiscard}
                    onCancel={() => setConfirmDiscardOpen(false)}
                    confirmTone="red"
                />
            )}

            {weaponEditor && (
                <WeaponEditModal
                    charIndex={charIndex}
                    item={adaptForWeaponModal(weaponEditor.item)}
                    source={weaponEditor.source}
                    onClose={() => setWeaponEditor(null)}
                    workspace={{
                        sessionID,
                        updateWeapon: (uid, patch) => workspace.updateWeapon(uid, patch),
                    }}
                    workspaceItem={weaponEditor.item}
                />
            )}

            {addOpen && (
                <AddItemModal
                    tab={activeSortTab}
                    target={addContainer}
                    onClose={() => setAddOpen(false)}
                    onAdd={async (spec, target) => {
                        const added = await workspace.addItem(spec, target, -1);
                        if (added) {
                            toast.success(`Added ${added.name} to ${target === 'inventory' ? 'Inventory' : 'Storage'}.`);
                            setAddOpen(false);
                        }
                    }}
                />
            )}

            <div className="flex flex-col h-full min-h-0 gap-2">
                {/* ── Shared toolbar: category + sort dropdowns + session controls ── */}
                <div className="flex items-center gap-2 shrink-0 border-b border-border/30 pb-2">
                    <label className="flex items-center gap-1.5 text-[10px] font-black uppercase tracking-wider text-muted-foreground">
                        Category
                        <select
                            aria-label="Category"
                            value={activeSortTab}
                            disabled={saving || loading}
                            onChange={(e) => setActiveSortTab(e.target.value as SortOrderTabKey)}
                            className="text-[10px] font-black uppercase tracking-wider bg-muted/30 border border-border/40 rounded px-2 py-1 text-foreground focus:outline-none focus:ring-1 focus:ring-primary/40 disabled:opacity-40 disabled:cursor-not-allowed"
                        >
                            {SORT_TABS.map(({ key, label }) => (
                                <option key={key} value={key}>{label}</option>
                            ))}
                        </select>
                    </label>
                    <label className="flex items-center gap-1.5 text-[10px] font-black uppercase tracking-wider text-muted-foreground">
                        Sort
                        <select
                            aria-label="Sort"
                            value={sortMode}
                            disabled={saving || loading || sortInFlight}
                            onChange={(e) => { void onSortModeChange(e.target.value as SortMode | ''); }}
                            className="text-[10px] font-black uppercase tracking-wider bg-muted/30 border border-border/40 rounded px-2 py-1 text-foreground focus:outline-none focus:ring-1 focus:ring-primary/40 disabled:opacity-40 disabled:cursor-not-allowed"
                        >
                            <option value="">Default</option>
                            {SORT_MODE_OPTIONS.map(({ mode, label }) => (
                                <option key={mode} value={mode}>{label}</option>
                            ))}
                        </select>
                    </label>
                    <div className="ml-auto flex items-center gap-2">
                        {dirty && (
                            <span
                                title="You have unsaved inventory edits. Press Save changes to persist."
                                className="text-xs font-black uppercase tracking-wider text-cyan-700 bg-cyan-500/15 border border-cyan-500/40 rounded px-2 py-0.5 whitespace-nowrap"
                            >
                                Unsaved
                            </span>
                        )}
                        {errorCount > 0 && (
                            <span className="text-xs font-black uppercase tracking-wider text-destructive bg-red-500/10 border border-destructive/40 rounded px-2 py-0.5 whitespace-nowrap">
                                {errorCount} error{errorCount === 1 ? '' : 's'}
                            </span>
                        )}
                        {warningCount > 0 && (
                            <span className="text-xs font-black uppercase tracking-wider text-info-foreground bg-blue-500/10 border border-blue-500/30 rounded px-2 py-0.5 whitespace-nowrap">
                                {warningCount} warn
                            </span>
                        )}
                        <button
                            disabled={saving || loading || invSelectedUIDs.size + stoSelectedUIDs.size === 0}
                            onClick={onRemoveSelected}
                            title="Remove selected items from the workspace"
                            className="px-3 py-1 text-xs font-black uppercase tracking-wider rounded text-destructive hover:text-destructive/80 hover:bg-red-500/20 border border-destructive/40 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            Remove
                        </button>
                        <button
                            disabled={saving || loading}
                            onClick={() => { setAddContainer('inventory'); setAddOpen(true); }}
                            className="px-3 py-1 text-xs font-black uppercase tracking-wider rounded text-foreground hover:bg-primary/20 border border-primary/40 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            Add Item…
                        </button>
                        <button
                            disabled={!dirty || saving || loading || errorCount > 0}
                            onClick={() => setConfirmSaveOpen(true)}
                            title={
                                errorCount > 0
                                    ? 'Validation errors block saving — fix them first.'
                                    : saving
                                      ? 'Saving…'
                                      : dirty
                                        ? 'Save all pending edits to the save file.'
                                        : 'No pending edits to save.'
                            }
                            className={`px-3 py-1 text-xs font-black uppercase tracking-wider rounded transition-all ${
                                dirty && !saving && errorCount === 0
                                    ? 'bg-primary text-primary-foreground hover:brightness-110 shadow-sm'
                                    : 'opacity-50 cursor-not-allowed bg-muted/20 text-muted-foreground border border-border/40'
                            }`}
                        >
                            {saving ? 'Saving…' : 'Save changes'}
                        </button>
                        <button
                            disabled={!dirty || saving || loading}
                            onClick={() => setConfirmDiscardOpen(true)}
                            className="px-3 py-1 text-xs font-black uppercase tracking-wider rounded text-foreground/70 hover:text-foreground hover:bg-muted/40 border border-border/60 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            Discard
                        </button>
                        <button
                            type="button"
                            onClick={() => setHelpOpen(true)}
                            title="Inventory Edit Session help"
                            aria-label="Inventory Edit Session help"
                            className="w-5 h-5 flex items-center justify-center rounded-full text-[10px] font-black text-muted-foreground hover:text-foreground hover:bg-muted/40 border border-border/40 transition-all"
                        >
                            ?
                        </button>
                    </div>
                </div>

                {/* ── Validation issues panel (collapsible — only when present) ── */}
                {(errorCount > 0 || warningCount > 0) && (
                    <ValidationPanel
                        validation={validation!}
                        sessionID={sessionID}
                        onRepaired={snap => workspace.replaceSnapshot(snap)}
                    />
                )}

                {loading && (
                    <div className="flex-1 flex flex-col items-center justify-center gap-3 text-muted-foreground">
                        <div className="w-6 h-6 border-2 border-foreground/20 border-t-foreground rounded-full animate-spin" />
                        <p className="text-[10px] font-bold uppercase tracking-widest">Loading session…</p>
                    </div>
                )}

                {!loading && !sessionID && !lastError && (
                    <div className="flex-1 flex items-center justify-center text-[10px] text-muted-foreground">
                        No active inventory session.
                    </div>
                )}

                {!loading && sessionID && (
                    <div className="flex-1 min-h-0 flex gap-3">
                        {/* ── Storage column ─────────────────────────────────── */}
                        <section className="flex-1 min-w-0 flex flex-col min-h-0 gap-2">
                            <ColumnHeader
                                label="Storage"
                                count={storageView.length}
                                selectedCount={stoSelectedHere.length}
                            />
                        <Frame
                                container="storage"
                                dragSource={dragSource}
                                isCrossDropTarget={frameDropTarget === 'storage'}
                                onDragOver={(e) => {
                                    if (dragSourceRef.current === 'inventory') {
                                        e.preventDefault();
                                        if (frameDropTarget !== 'storage') setFrameDropTarget('storage');
                                    }
                                }}
                                onDragLeave={(e) => {
                                    const related = e.relatedTarget as Node | null;
                                    if (!related || !e.currentTarget.contains(related)) {
                                        if (frameDropTarget === 'storage') setFrameDropTarget(null);
                                    }
                                }}
                                onDrop={(e) => {
                                    if (dragSourceRef.current !== 'inventory') return;
                                    e.preventDefault();
                                    void onFrameDrop('storage');
                                }}
                            >
                                {storageCards.map((cells, cardIdx) => (
                                    <Card key={cardIdx}>
                                        {cells.map((item, localIdx) => {
                                            const absIdx = cardIdx * GRID_MIN_CELLS + localIdx;
                                            return item != null ? (
                                                <ItemTile
                                                    key={item.uid}
                                                    item={item}
                                                    isDragging={dragSource === 'storage' && dragFromUID === item.uid}
                                                    isDragOver={dragOverContainer === 'storage' && dragOverLocal === absIdx}
                                                    isSelected={stoSelectedUIDs.has(item.uid)}
                                                    onClick={(e) => onTileClick('storage', item, e)}
                                                    onEditClick={activeSortTab === 'weapons' && item.isWeapon
                                                        ? (e) => { e.stopPropagation(); e.preventDefault(); setWeaponEditor({ item, source: 'storage' }); }
                                                        : undefined}
                                                    onDragStart={() => onDragStart('storage', item)}
                                                    onDragOver={(e) => onTileDragOver(e, 'storage', absIdx)}
                                                    onDrop={() => onTileDrop('storage', absIdx, 0)}
                                                    onDragEnd={onDragEnd}
                                                />
                                            ) : (
                                                <EmptyCell
                                                    key={`s-empty-${absIdx}`}
                                                    testId={`sto-empty-${absIdx}`}
                                                    isDragOver={dragOverContainer === 'storage' && dragOverLocal === absIdx}
                                                    onDragOver={(e) => onTileDragOver(e, 'storage', absIdx)}
                                                    onDrop={() => onTileDrop('storage', absIdx, 0)}
                                                />
                                            );
                                        })}
                                    </Card>
                                ))}
                            </Frame>
                        </section>

                        <div className="w-px shrink-0 bg-border/40" aria-hidden="true" />

                        {/* ── Inventory column ────────────────────────────────── */}
                        <section className="flex-1 min-w-0 flex flex-col min-h-0 gap-2">
                            <ColumnHeader
                                label="Inventory"
                                count={inventoryView.length}
                                selectedCount={invSelectedHere.length}
                            />
                        <Frame
                                container="inventory"
                                dragSource={dragSource}
                                isCrossDropTarget={frameDropTarget === 'inventory'}
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
                                    if (dragSourceRef.current !== 'storage') return;
                                    e.preventDefault();
                                    void onFrameDrop('inventory');
                                }}
                            >
                                {inventoryCards.map((cells, cardIdx) => (
                                    <Card key={cardIdx}>
                                        {cells.map((item, localIdx) => {
                                            const absIdx = cardIdx * GRID_MIN_CELLS + localIdx;
                                            return item != null ? (
                                                <ItemTile
                                                    key={item.uid}
                                                    item={item}
                                                    isDragging={dragSource === 'inventory' && dragFromUID === item.uid}
                                                    isDragOver={dragOverContainer === 'inventory' && dragOverLocal === absIdx}
                                                    isSelected={invSelectedUIDs.has(item.uid)}
                                                    onClick={(e) => onTileClick('inventory', item, e)}
                                                    onEditClick={activeSortTab === 'weapons' && item.isWeapon
                                                        ? (e) => { e.stopPropagation(); e.preventDefault(); setWeaponEditor({ item, source: 'inventory' }); }
                                                        : undefined}
                                                    onDragStart={() => onDragStart('inventory', item)}
                                                    onDragOver={(e) => onTileDragOver(e, 'inventory', absIdx)}
                                                    onDrop={() => onTileDrop('inventory', absIdx, 0)}
                                                    onDragEnd={onDragEnd}
                                                />
                                            ) : (
                                                <EmptyCell
                                                    key={`i-empty-${absIdx}`}
                                                    testId={`inv-empty-${absIdx}`}
                                                    isDragOver={dragOverContainer === 'inventory' && dragOverLocal === absIdx}
                                                    onDragOver={(e) => onTileDragOver(e, 'inventory', absIdx)}
                                                    onDrop={() => onTileDrop('inventory', absIdx, 0)}
                                                />
                                            );
                                        })}
                                    </Card>
                                ))}
                            </Frame>
                        </section>
                    </div>
                )}
                <p className="text-[9px] text-muted-foreground/70 shrink-0">
                    Active tab: <span className="font-bold">{activeLabel}</span> · Session ID: {sessionID || '—'}
                </p>
            </div>
        </>
    );
}

// ── Sub-components ─────────────────────────────────────────────────────────────

interface ColumnHeaderProps {
    label: string;
    count: number;
    selectedCount: number;
}

function ColumnHeader({ label, count, selectedCount }: ColumnHeaderProps) {
    return (
        <div className="flex items-center shrink-0 gap-2 min-h-7">
            <span className="text-xs font-bold text-blue-700 tabular-nums whitespace-nowrap">
                {label}: {count} item{count === 1 ? '' : 's'}
            </span>
            {selectedCount > 0 && (
                <span className="text-[9px] font-bold text-cyan-300/80 uppercase tracking-widest whitespace-nowrap">
                    {selectedCount} selected
                </span>
            )}
        </div>
    );
}

interface FrameProps {
    container: ContainerKind;
    dragSource: ContainerKind | null;
    isCrossDropTarget: boolean;
    onDragOver: (e: React.DragEvent) => void;
    onDragLeave: (e: React.DragEvent) => void;
    onDrop: (e: React.DragEvent) => void;
    children: React.ReactNode;
}

function Frame({ container, dragSource, isCrossDropTarget, onDragOver, onDragLeave, onDrop, children }: FrameProps) {
    const scrollRef = useRef<HTMLDivElement>(null);
    // Guards against a single trackpad gesture (many wheel events) skipping
    // multiple cards — we consume one card per notch, then ignore the burst.
    const lockRef = useRef(false);
    // The card index the wheel is currently animating toward. While a smooth
    // scroll is still settling, the next notch steps from this target instead
    // of the mid-animation scrollTop — otherwise a half-finished scroll rounds
    // to the target and the next step overshoots by a whole card.
    const wheelTargetRef = useRef<number | null>(null);
    const autoScrollStartRef = useRef<number | null>(null);
    const autoScrollRepeatRef = useRef<number | null>(null);
    const autoScrollDirectionRef = useRef<CardScrollDirection>(0);
    const [autoScrollDirection, setAutoScrollDirection] = useState<CardScrollDirection>(0);

    const stopAutoScroll = useCallback(() => {
        if (autoScrollStartRef.current !== null) {
            window.clearTimeout(autoScrollStartRef.current);
            autoScrollStartRef.current = null;
        }
        if (autoScrollRepeatRef.current !== null) {
            window.clearInterval(autoScrollRepeatRef.current);
            autoScrollRepeatRef.current = null;
        }
        autoScrollDirectionRef.current = 0;
        setAutoScrollDirection(0);
    }, []);

    const scrollOneCard = useCallback((direction: CardScrollDirection): boolean => {
        const el = scrollRef.current;
        if (!el || direction === 0) return false;
        const maxScroll = Math.max(0, el.scrollHeight - el.clientHeight);
        const currentCard = Math.round(el.scrollTop / CARD_STEP_PX);
        const targetTop = Math.max(0, Math.min(maxScroll, (currentCard + direction) * CARD_STEP_PX));
        if (Math.abs(targetTop - el.scrollTop) <= 1) return false;
        el.scrollTo({ top: targetTop, behavior: 'auto' });
        return true;
    }, []);

    const armAutoScroll = useCallback((direction: CardScrollDirection) => {
        if (direction === 0) {
            stopAutoScroll();
            return;
        }
        if (autoScrollDirectionRef.current === direction) return;

        stopAutoScroll();
        autoScrollDirectionRef.current = direction;
        setAutoScrollDirection(direction);
        autoScrollStartRef.current = window.setTimeout(() => {
            autoScrollStartRef.current = null;
            if (!scrollOneCard(direction)) {
                stopAutoScroll();
                return;
            }
            autoScrollRepeatRef.current = window.setInterval(() => {
                if (!scrollOneCard(direction)) stopAutoScroll();
            }, AUTO_SCROLL_REPEAT_MS);
        }, AUTO_SCROLL_INITIAL_DELAY_MS);
    }, [scrollOneCard, stopAutoScroll]);

    useEffect(() => {
        const el = scrollRef.current;
        if (!el) return;
        // Native non-passive listener: React's onWheel is passive, so
        // preventDefault would be ignored there. scroll-snap (CSS below) is the
        // safety fallback if this handler is bypassed.
        // One physical gesture (momentum tail included) pages exactly one card.
        // We lock on the leading event and only re-open once the smooth scroll
        // has actually landed (scrollend) AND the momentum has gone quiet — never
        // on a bare timeout, which can expire mid-scroll and let a late momentum
        // event page a second card.
        let settled = true;   // no scroll currently in flight
        let idle = true;      // no recent wheel events
        let idleTimer: number | null = null;
        let settleFallback: number | null = null;
        const clearSettleFallback = () => {
            if (settleFallback !== null) { window.clearTimeout(settleFallback); settleFallback = null; }
        };
        const releaseIfDone = () => {
            if (settled && idle) {
                lockRef.current = false;
                wheelTargetRef.current = null;
            }
        };
        const onWheel = (e: WheelEvent) => {
            const maxScroll = el.scrollHeight - el.clientHeight;
            if (maxScroll <= 0) return; // single card — nothing to page
            const dir = e.deltaY > 0 ? 1 : e.deltaY < 0 ? -1 : 0;
            if (dir === 0) return;
            const maxCard = Math.round(maxScroll / CARD_STEP_PX);
            // Step from the in-flight target if a scroll is still settling, else
            // from the current resting card.
            const base = wheelTargetRef.current ?? Math.round(el.scrollTop / CARD_STEP_PX);
            // At the ends, don't trap the user — let the outer page scroll.
            if ((dir < 0 && base <= 0) || (dir > 0 && base >= maxCard)) return;
            e.preventDefault();
            // Every event (leading or momentum) keeps the gesture "active". The
            // idle timer is what eventually re-opens the lock, and the momentum
            // tail keeps resetting it, so it can never sneak in a second step.
            idle = false;
            if (idleTimer !== null) window.clearTimeout(idleTimer);
            idleTimer = window.setTimeout(() => { idle = true; releaseIfDone(); }, WHEEL_IDLE_MS);
            if (lockRef.current) return; // gesture already handled; swallow momentum
            lockRef.current = true;
            settled = false;
            const next = Math.max(0, Math.min(maxCard, base + dir));
            wheelTargetRef.current = next;
            el.scrollTo({ top: next * CARD_STEP_PX, behavior: 'smooth' });
            clearSettleFallback();
            settleFallback = window.setTimeout(() => { settled = true; releaseIfDone(); }, WHEEL_SETTLE_FALLBACK_MS);
        };
        // Smooth scroll landed — mark settled and try to re-open (still gated on
        // the momentum having gone quiet).
        const onScrollEnd = () => { settled = true; clearSettleFallback(); releaseIfDone(); };
        el.addEventListener('wheel', onWheel, { passive: false });
        el.addEventListener('scrollend', onScrollEnd);
        return () => {
            el.removeEventListener('wheel', onWheel);
            el.removeEventListener('scrollend', onScrollEnd);
            if (idleTimer !== null) window.clearTimeout(idleTimer);
            clearSettleFallback();
        };
    }, []);

    useEffect(() => () => {
        if (autoScrollStartRef.current !== null) window.clearTimeout(autoScrollStartRef.current);
        if (autoScrollRepeatRef.current !== null) window.clearInterval(autoScrollRepeatRef.current);
    }, []);

    const handleDragOver = (event: React.DragEvent) => {
        onDragOver(event);
        if (dragSource !== container) {
            stopAutoScroll();
            return;
        }
        // Keep receiving dragover events even while the pointer is over a card
        // gap or frame padding rather than an individual tile.
        event.preventDefault();
        const el = scrollRef.current;
        if (!el) return;
        const maxScroll = Math.max(0, el.scrollHeight - el.clientHeight);
        const rect = el.getBoundingClientRect();
        const direction = getCardAutoScrollDirection(
            event.clientY,
            rect.top,
            rect.bottom,
            el.scrollTop > 1,
            el.scrollTop < maxScroll - 1,
        );
        armAutoScroll(direction);
    };

    const handleDragLeave = (event: React.DragEvent) => {
        const related = event.relatedTarget as Node | null;
        if (!related || !event.currentTarget.contains(related)) stopAutoScroll();
        onDragLeave(event);
    };

    const handleDrop = (event: React.DragEvent) => {
        stopAutoScroll();
        onDrop(event);
    };

    return (
        <div
            ref={scrollRef}
            className={`relative shrink-0 mx-auto rounded-xl border bg-background/40 overflow-y-auto transition-colors ${
                isCrossDropTarget ? 'border-cyan-400/70 ring-2 ring-cyan-400/40' : 'border-border/50'
            }`}
            style={{
                width: FRAME_WIDTH_PX,
                height: FRAME_HEIGHT_PX,
                paddingLeft: PAD_PX,
                paddingRight: PAD_PX,
                scrollSnapType: 'y mandatory',
            }}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            onDragEnd={stopAutoScroll}
        >
            {autoScrollDirection !== 0 && (
                <div
                    aria-hidden="true"
                    className={`pointer-events-none absolute inset-x-0 z-10 flex h-11 items-center justify-center bg-primary/10 text-primary ${
                        autoScrollDirection < 0 ? 'top-0 border-b border-primary/30' : 'bottom-0 border-t border-primary/30'
                    }`}
                >
                    <span className="text-xs font-black">{autoScrollDirection < 0 ? '↑' : '↓'}</span>
                </div>
            )}
            <div className="flex flex-col" style={{ gap: GAP_PX }}>
                {children}
            </div>
        </div>
    );
}

// One card = a fixed 5×6 grid; its start edge is a scroll-snap point so the
// frame always rests on a whole card.
function Card({ children }: { children: React.ReactNode }) {
    return (
        <div
            className="grid content-start shrink-0"
            style={{
                gridTemplateColumns: GRID_TEMPLATE_COLUMNS,
                gap: GAP_PX,
                height: CARD_HEIGHT_PX,
                scrollSnapAlign: 'start',
            }}
        >
            {children}
        </div>
    );
}

interface TileProps {
    item: editor.EditableItem;
    isDragging: boolean;
    isDragOver: boolean;
    isSelected: boolean;
    onClick: (e: React.MouseEvent) => void;
    onEditClick?: (e: React.MouseEvent) => void;
    onDragStart: () => void;
    onDragOver: (e: React.DragEvent) => void;
    onDrop: () => void;
    onDragEnd: () => void;
}

function ItemTile({ item, isDragging, isDragOver, isSelected, onClick, onEditClick, onDragStart, onDragOver, onDrop, onDragEnd }: TileProps) {
    const [imgError, setImgError] = useState(false);
    const upgradeLabel = item.currentUpgrade > 0
        ? (item.infusionName ? `${item.infusionName} +${item.currentUpgrade}` : `+${item.currentUpgrade}`)
        : (item.infusionName || null);
    const pendingBadge = item.hasPendingWeaponPatch ? '★' : null;
    const aowBadge = renderAoWBadge(item);
    const tooltip = [item.name, upgradeLabel, `#${item.acquisitionIndex}`].filter(Boolean).join(' · ');
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
                    ? 'opacity-40 border-border/20'
                    : isDragOver
                      ? 'border-primary ring-1 ring-primary/50 bg-primary/[0.06]'
                    : isSelected
                        ? 'border-cyan-400/70 ring-2 ring-cyan-400/50 bg-cyan-400/[0.08]'
                        : 'border-border/50 hover:border-primary/40 hover:bg-primary/[0.03]'
            }`}
        >
            <div className="absolute inset-0 flex flex-col items-center p-1 gap-0.5">
                <div className="flex-1 min-h-0 flex items-center justify-center w-full overflow-hidden">
                    {showIcon ? (
                        <img src={item.iconPath} alt="" draggable={false} className="max-w-full max-h-full object-contain drop-shadow-sm" onError={() => setImgError(true)} />
                    ) : (
                        <span className="text-xl font-black text-muted-foreground/60 select-none leading-none">{item.name.charAt(0).toUpperCase()}</span>
                    )}
                </div>
                <div className="w-full shrink-0 overflow-hidden">
                    <div className="text-[8px] font-bold text-foreground/60 truncate text-center leading-tight">{item.name}</div>
                    {upgradeLabel && (
                        <div className="text-[7px] font-mono text-primary/50 truncate text-center leading-tight">{upgradeLabel}</div>
                    )}
                </div>
            </div>
            {aowBadge}
            {pendingBadge && (
                <div className="absolute bottom-0.5 right-0.5 px-0.5 rounded text-[7px] font-mono font-bold text-cyan-300 leading-tight pointer-events-none" title="Pending unsaved weapon edit">
                    {pendingBadge}
                </div>
            )}
            {onEditClick && (
                <button
                    type="button"
                    draggable={false}
                    onClick={onEditClick}
                    onPointerDown={(e) => e.stopPropagation()}
                    onMouseDown={(e) => e.stopPropagation()}
                    onDragStart={(e) => { e.preventDefault(); e.stopPropagation(); }}
                    title="Edit weapon"
                    aria-label="Edit weapon"
                    className="absolute top-0.5 left-0.5 z-10 w-4 h-4 flex items-center justify-center rounded bg-red-700/85 hover:bg-red-600 text-white shadow ring-1 ring-red-900/40 transition-colors cursor-pointer"
                >
                    <svg className="w-2.5 h-2.5" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M14.7 6.3l3 3M4 20l3.5-1 9.8-9.8a2.1 2.1 0 0 0 0-3l-.5-.5a2.1 2.1 0 0 0-3 0L4 16.5 4 20z" />
                    </svg>
                </button>
            )}
        </div>
    );
}

function renderAoWBadge(item: editor.EditableItem): React.ReactNode {
    if (!item.isWeapon) return null;
    if (item.pendingAoWClear) {
        return <div className="absolute top-0.5 right-0.5 px-1 rounded text-[7px] font-black text-red-300 bg-red-500/20 border border-red-400/40 leading-tight pointer-events-none" title="Pending: clear Ash of War">CLR</div>;
    }
    if (item.pendingAoWItemID && item.pendingAoWName) {
        return <div className="absolute top-0.5 right-0.5 px-1 rounded text-[7px] font-black text-blue-300 bg-blue-500/20 border border-blue-400/40 leading-tight pointer-events-none truncate max-w-[60%]" title={`Pending: ${item.pendingAoWName}`}>{item.pendingAoWName.slice(0, 4)}…</div>;
    }
    if (item.hasCurrentAoW && item.currentAoWName) {
        return <div className="absolute top-0.5 right-0.5 px-1 rounded text-[7px] font-bold text-blue-200/80 bg-blue-500/15 border border-blue-400/30 leading-tight pointer-events-none truncate max-w-[60%]" title={`Current AoW: ${item.currentAoWName}`}>{item.currentAoWName.slice(0, 4)}…</div>;
    }
    return null;
}

interface EmptyCellProps {
    isDragOver?: boolean;
    onDragOver?: (e: React.DragEvent) => void;
    onDrop?: () => void;
    testId?: string;
}

// Empty cells are valid same-container reorder destinations (including on the
// trailing empty card). The reorder path clamps an out-of-range index to "insert
// after the last item", so dropping on any empty cell drops to the end.
function EmptyCell({ isDragOver, onDragOver, onDrop, testId }: EmptyCellProps) {
    return (
        <div
            aria-hidden="true"
            data-testid={testId}
            onDragOver={onDragOver}
            onDrop={onDrop}
            className={`relative aspect-square rounded-md border border-dashed bg-card/40 shadow-inner transition-all ${
                isDragOver ? 'border-primary ring-1 ring-primary/50 bg-primary/[0.06] opacity-100' : 'border-border/60 opacity-75'
            }`}
            style={{
                backgroundImage:
                    'linear-gradient(135deg, rgba(255,255,255,0.06) 0%, rgba(255,255,255,0.018) 45%, rgba(0,0,0,0.10) 100%), radial-gradient(circle at 50% 42%, rgba(255,255,255,0.08), transparent 45%)',
            }}
        />
    );
}

function sortEditableItems(items: editor.EditableItem[], mode: SortMode): editor.EditableItem[] {
    const arr = [...items];
    switch (mode) {
        case 'acquisition-asc':
            return arr.sort((a, b) => a.acquisitionIndex - b.acquisitionIndex || cmpName(a, b));
        case 'acquisition-desc':
            return arr.sort((a, b) => b.acquisitionIndex - a.acquisitionIndex || cmpName(a, b));
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
        case 'default':
            // In-game section hierarchy: category (melee → shields → ranged),
            // then canonical sub-category order, then regulation order within a
            // sub-category. Unknown category/subcategory ranks sort after known.
            return arr.sort((a, b) =>
                categoryRank(a.category) - categoryRank(b.category)
                || subCategoryRank(a.subCategory) - subCategoryRank(b.subCategory)
                || compareByRegulation(a, b));
        case 'type-asc':
            return arr.sort(compareByRegulation);
        case 'type-desc':
            return arr.sort((a, b) => {
                const ga = a.sortGroupId ?? 0;
                const gb = b.sortGroupId ?? 0;
                const sa = a.sortId ?? 0;
                const sb = b.sortId ?? 0;
                if (ga === 0 && gb === 0 && sa === 0 && sb === 0) {
                    return cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
                }
                if (ga === 0 && sa === 0) return 1;
                if (gb === 0 && sb === 0) return -1;
                return gb - ga || sb - sa || cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
            });
        default:
            return arr;
    }
}

function cmpName(a: editor.EditableItem, b: editor.EditableItem): number {
    return a.name.localeCompare(b.name);
}

// Regulation ordering shared by 'type-asc' and 'default': ascending sortGroupId
// then sortId, with items lacking both keys pushed to the end, and name /
// acquisitionIndex as deterministic final tie-breakers.
function compareByRegulation(a: editor.EditableItem, b: editor.EditableItem): number {
    const ga = a.sortGroupId ?? 0;
    const gb = b.sortGroupId ?? 0;
    const sa = a.sortId ?? 0;
    const sb = b.sortId ?? 0;
    if (ga === 0 && gb === 0 && sa === 0 && sb === 0) {
        return cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
    }
    if (ga === 0 && sa === 0) return 1;
    if (gb === 0 && sb === 0) return -1;
    return ga - gb || sa - sb || cmpName(a, b) || a.acquisitionIndex - b.acquisitionIndex;
}

// ── Confirm modal ─────────────────────────────────────────────────────────────
interface ConfirmModalProps {
    title: string;
    body: string;
    confirmLabel: string;
    confirmTone: 'green' | 'red';
    onConfirm: () => void;
    onCancel: () => void;
}

function ConfirmModal({ title, body, confirmLabel, confirmTone, onConfirm, onCancel }: ConfirmModalProps) {
    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onCancel}>
            <div className="bg-background border border-border rounded-xl p-6 w-[460px] shadow-2xl" onClick={(e) => e.stopPropagation()}>
                <div className="flex items-center gap-2 mb-4">
                    <div className={`w-1 h-4 rounded-full ${confirmTone === 'green' ? 'bg-green-500' : 'bg-red-500'}`} />
                    <h3 className="text-[10px] font-black uppercase tracking-widest">{title}</h3>
                </div>
                <p className="text-[11px] text-muted-foreground leading-relaxed mb-5">{body}</p>
                <div className="flex justify-end gap-2">
                    <button onClick={onCancel} className="px-4 py-1.5 text-[10px] font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all">Cancel</button>
                    <button
                        onClick={onConfirm}
                        className={`px-4 py-1.5 text-[10px] font-black uppercase tracking-wider rounded text-white shadow-sm transition-all ${
                            confirmTone === 'green' ? 'bg-green-700/80 hover:bg-green-700' : 'bg-red-700/80 hover:bg-red-700'
                        }`}
                    >
                        {confirmLabel}
                    </button>
                </div>
            </div>
        </div>
    );
}

// ── Validation panel ──────────────────────────────────────────────────────────
const WORKSPACE_AUTO_REPAIR_CODES = new Set(['upgrade_out_of_range', 'pending_aow_unknown', 'pending_aow_conflict']);

function ValidationPanel({
    validation,
    sessionID,
    onRepaired,
}: {
    validation: editor.WorkspaceValidationReport;
    sessionID: string;
    onRepaired: (snap: editor.InventoryWorkspaceSnapshot) => void;
}) {
    const [open, setOpen] = useState(true);
    const [repairing, setRepairing] = useState(false);

    const issues = [
        ...validation.errors.map(i => ({ ...i, severity: 'error' as const })),
        ...validation.warnings.map(i => ({ ...i, severity: 'warning' as const })),
    ];

    const repairableSpecs = issues
        .filter(i => WORKSPACE_AUTO_REPAIR_CODES.has(i.code) && i.uid)
        .map(i => main.WorkspaceRepairSpec.createFrom({ uid: i.uid, code: i.code }));

    const handleRepairAll = async () => {
        if (!sessionID || repairing || repairableSpecs.length === 0) return;
        setRepairing(true);
        try {
            const snap = await RepairInventoryWorkspaceItems(sessionID, repairableSpecs);
            onRepaired(snap);
            toast.success(`Repaired ${repairableSpecs.length} issue(s) successfully.`);
        } catch (e) {
            toast.error(`Repair failed: ${String(e)}`);
        } finally {
            setRepairing(false);
        }
    };

    if (issues.length === 0) return null;
    return (
        <div className="shrink-0 rounded border border-border/40 bg-background/30">
            <div className="flex items-center">
                <button
                    onClick={() => setOpen(o => !o)}
                    className="flex-1 px-3 py-1.5 text-left text-xs font-black uppercase tracking-widest text-muted-foreground hover:text-foreground hover:bg-muted/30 transition-all"
                >
                    {open ? '▼' : '▶'} Validation ({validation.errors.length} error · {validation.warnings.length} warn)
                </button>
                {repairableSpecs.length > 0 && (
                    <button
                        onClick={handleRepairAll}
                        disabled={repairing || !sessionID}
                        className="mr-2 px-2 py-0.5 text-[8px] font-black uppercase tracking-widest rounded bg-primary/20 text-primary border border-primary/30 hover:bg-primary/30 disabled:opacity-40 disabled:cursor-not-allowed transition-all"
                    >
                        {repairing ? '…' : `Repair auto-fixable (${repairableSpecs.length})`}
                    </button>
                )}
            </div>
            {open && (
                <ul className="max-h-48 overflow-y-auto text-xs divide-y divide-border/40">
                    {issues.map((i, idx) => (
                        <li key={`${i.code}-${idx}`} className={`px-3 py-1.5 ${i.severity === 'error' ? 'text-destructive' : 'text-info-foreground'}`}>
                            <span className="font-bold mr-1">[{i.code}]</span>
                            <span className="text-foreground/80">{i.message}</span>
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}

// ── Add-item modal ────────────────────────────────────────────────────────────
interface AddItemModalProps {
    tab: SortOrderTabKey;
    target: ContainerKind;
    onClose: () => void;
    onAdd: (spec: editor.AddItemSpec, target: ContainerKind) => Promise<void>;
}

function AddItemModal({ tab, target, onClose, onAdd }: AddItemModalProps) {
    const [items, setItems] = useState<db.ItemEntry[]>([]);
    const [loadingList, setLoadingList] = useState(false);
    const [search, setSearch] = useState('');
    const [selectedID, setSelectedID] = useState<number | null>(null);
    const [quantity, setQuantity] = useState(1);
    const [container, setContainer] = useState<ContainerKind>(target);
    const [adding, setAdding] = useState(false);

    useEffect(() => {
        setLoadingList(true);
        const tabCats = Array.from(TAB_CATEGORIES[tab]);
        Promise.all(tabCats.map(cat => GetItemList(cat)))
            .then(results => {
                const flat = results.flat();
                // Sort by name; backend already filters to valid items.
                flat.sort((a, b) => a.name.localeCompare(b.name));
                setItems(flat);
            })
            .catch(err => toast.error(`Failed to load ${tab}: ${String(err)}`))
            .finally(() => setLoadingList(false));
    }, [tab]);

    const filtered = useMemo(() => {
        const q = search.trim().toLowerCase();
        if (!q) return items.slice(0, 200);
        return items.filter(it => it.name.toLowerCase().includes(q)).slice(0, 200);
    }, [items, search]);

    const onConfirm = useCallback(async () => {
        if (selectedID == null) return;
        setAdding(true);
        const spec = editor.AddItemSpec.createFrom({
            baseItemID: selectedID,
            quantity: quantity > 0 ? quantity : 1,
        });
        try {
            await onAdd(spec, container);
        } finally {
            setAdding(false);
        }
    }, [selectedID, quantity, container, onAdd]);

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onClose}>
            <div className="bg-background border border-border rounded-xl p-5 w-[520px] max-h-[80vh] shadow-2xl flex flex-col gap-3" onClick={(e) => e.stopPropagation()}>
                <div className="flex items-center gap-2">
                    <div className="w-1 h-4 bg-primary rounded-full" />
                    <h3 className="text-sm font-black uppercase tracking-widest">Add Item to {container === 'inventory' ? 'Inventory' : 'Storage'} · {tab}</h3>
                </div>
                <input
                    autoFocus
                    type="text"
                    placeholder="Search item name…"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="w-full px-3 py-1.5 text-sm bg-muted/30 border border-border/40 rounded text-foreground focus:outline-none focus:ring-1 focus:ring-primary/40"
                />
                <div className="flex-1 min-h-0 overflow-y-auto border border-border/40 rounded">
                    {loadingList ? (
                        <div className="p-4 text-center text-sm text-muted-foreground">Loading…</div>
                    ) : filtered.length === 0 ? (
                        <div className="p-4 text-center text-sm text-muted-foreground">No matches.</div>
                    ) : (
                        <ul>
                            {filtered.map(it => (
                                <li key={it.id}>
                                    <button
                                        onClick={() => setSelectedID(it.id)}
                                        className={`w-full text-left px-3 py-1.5 text-sm flex items-center gap-2 ${
                                            selectedID === it.id
                                                ? 'bg-primary/20 text-primary'
                                                : 'hover:bg-muted/30 text-foreground/90'
                                        }`}
                                    >
                                        {it.iconPath ? (
                                            <img src={it.iconPath} alt="" className="w-10 h-10 object-contain flex-shrink-0" />
                                        ) : (
                                            <span className="w-10 h-10 inline-block flex-shrink-0" />
                                        )}
                                        <span className="flex-1 truncate">{it.name}</span>
                                        <span className="text-xs text-muted-foreground font-mono">#{it.id.toString(16)}</span>
                                    </button>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
                <div className="flex items-center gap-3">
                    <label className="flex items-center gap-1.5 text-xs font-bold uppercase text-muted-foreground">
                        Container:
                        <select
                            value={container}
                            onChange={(e) => setContainer(e.target.value as ContainerKind)}
                            className="text-xs font-bold uppercase bg-muted/30 border border-border/40 rounded px-2 py-1"
                        >
                            <option value="inventory">Inventory</option>
                            <option value="storage">Storage</option>
                        </select>
                    </label>
                    <label className="flex items-center gap-1.5 text-xs font-bold uppercase text-muted-foreground">
                        Quantity:
                        <input
                            type="number"
                            min={1}
                            max={999}
                            value={quantity}
                            onChange={(e) => setQuantity(parseInt(e.target.value || '1', 10))}
                            className="w-16 text-sm bg-muted/30 border border-border/40 rounded px-2 py-1 text-foreground focus:outline-none focus:ring-1 focus:ring-primary/40"
                        />
                    </label>
                    <div className="ml-auto flex gap-2">
                        <button onClick={onClose} className="px-3 py-1.5 text-xs font-black uppercase tracking-wider rounded text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all">Cancel</button>
                        <button
                            disabled={selectedID == null || adding}
                            onClick={onConfirm}
                            className={`px-3 py-1.5 text-xs font-black uppercase tracking-wider rounded transition-all ${
                                selectedID != null && !adding
                                    ? 'bg-primary text-primary-foreground hover:brightness-110 shadow-sm'
                                    : 'opacity-50 cursor-not-allowed bg-muted/20 text-muted-foreground'
                            }`}
                        >
                            {adding ? 'Adding…' : 'Add'}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}
