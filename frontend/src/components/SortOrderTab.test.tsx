import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { editor } from '../../wailsjs/go/models';

vi.mock('../../wailsjs/go/main/App', () => ({
    StartInventoryEditSession: vi.fn(),
    GetInventoryEditSession: vi.fn(),
    ValidateInventoryWorkspace: vi.fn(),
    MoveInventoryWorkspaceItem: vi.fn(),
    ReorderInventoryWorkspaceItems: vi.fn(),
    TransferInventoryWorkspaceItem: vi.fn(),
    AddInventoryWorkspaceItem: vi.fn(),
    UpdateInventoryWorkspaceWeapon: vi.fn(),
    RemoveInventoryWorkspaceItem: vi.fn(),
    SaveInventoryWorkspaceChanges: vi.fn(),
    DiscardInventoryEditSession: vi.fn(),
    GetItemList: vi.fn().mockResolvedValue([]),
}));

vi.mock('../lib/toast', () => {
    const fn = vi.fn() as unknown as Record<string, unknown> & ((...args: unknown[]) => void);
    fn.success = vi.fn();
    fn.error = vi.fn();
    return { default: fn };
});

import * as App from '../../wailsjs/go/main/App';
import { SortOrderTab, buildSortOrderCards, getCardAutoScrollDirection, CARD_STEP_PX } from './SortOrderTab';

const mocks = App as unknown as Record<string, ReturnType<typeof vi.fn>>;

function makeItem(uid: string, container: 'inventory' | 'storage', position: number, overrides: Partial<editor.EditableItem> = {}): editor.EditableItem {
    return editor.EditableItem.createFrom({
        uid,
        source: 'original',
        container,
        position,
        originalHandle: 0x80800000 + position,
        itemID: 0x000F4240,
        baseItemID: 0x000F4240,
        name: `Sword ${position}`,
        category: 'melee_armaments',
        quantity: 1,
        acquisitionIndex: 1000 + position,
        currentUpgrade: 0,
        maxUpgrade: 25,
        hasGaItem: true,
        isWeapon: true,
        isArmor: false,
        isTalisman: false,
        ...overrides,
    });
}

function makeSnapshot(opts: {
    sessionID?: string;
    inventory?: editor.EditableItem[];
    storage?: editor.EditableItem[];
    dirty?: boolean;
} = {}): editor.InventoryWorkspaceSnapshot {
    return editor.InventoryWorkspaceSnapshot.createFrom({
        sessionID: opts.sessionID ?? 'ses-tab',
        characterIndex: 0,
        inventoryItems: opts.inventory ?? [],
        storageItems: opts.storage ?? [],
        unsupportedInventoryRecords: [],
        unsupportedStorageRecords: [],
        dirty: opts.dirty ?? false,
        validation: {
            ok: true,
            errors: [],
            warnings: [],
            inventoryItemCount: (opts.inventory ?? []).length,
            storageItemCount: (opts.storage ?? []).length,
            unsupportedInventoryCount: 0,
            unsupportedStorageCount: 0,
            duplicateUIDs: [],
            duplicateHandles: [],
        },
    });
}

async function mount(snap: editor.InventoryWorkspaceSnapshot, onMutate = vi.fn()) {
    mocks.StartInventoryEditSession.mockResolvedValue(snap);
    let utils!: ReturnType<typeof render>;
    await act(async () => {
        utils = render(<SortOrderTab charIndex={0} inventoryVersion={1} onMutate={onMutate} />);
    });
    return utils;
}

beforeEach(() => {
    Object.values(mocks).forEach(m => typeof m?.mockReset === 'function' && m.mockReset());
    mocks.DiscardInventoryEditSession.mockResolvedValue(undefined);
    mocks.GetItemList.mockResolvedValue([]);
});

afterEach(() => {
    vi.clearAllMocks();
});

describe('SortOrderTab (workspace mode)', () => {
    it('starts a workspace session on mount with the active charIndex', async () => {
        await mount(makeSnapshot({ sessionID: 'ses-mount' }));
        await waitFor(() => {
            expect(mocks.StartInventoryEditSession).toHaveBeenCalledWith(0);
        });
    });

    it('does NOT render an Apply Order button', async () => {
        await mount(makeSnapshot({ inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        expect(screen.queryByRole('button', { name: /Apply Order/i })).not.toBeInTheDocument();
    });

    it('the shared Sort dropdown starts on Default and keeps the chosen label', async () => {
        await mount(makeSnapshot({ inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        const sort = screen.getByLabelText('Sort') as HTMLSelectElement;

        // Initial option is Default (no active sort choice).
        expect(screen.getByRole('option', { name: 'Default' })).toBeInTheDocument();
        expect(sort.value).toBe('');

        await act(async () => {
            fireEvent.change(sort, { target: { value: 'weight-desc' } });
        });

        // The label persists — it does not snap back to Default.
        expect((screen.getByLabelText('Sort') as HTMLSelectElement).value).toBe('weight-desc');
    });

    it('selecting a non-default sort then Default reproduces the in-game section hierarchy (category → subCategory → regulation), NOT acquisitionIndex or a flat sortGroupId sort', async () => {
        // Every axis is deliberately in conflict:
        //   - acquisitionIndex ascending  → ranged first, melee last
        //   - flat sortGroupId ascending  → ranged, then shields, then melee
        //   - category order (Default)    → melee, then shields, then ranged
        // subCategory then orders within a category (Daggers before Straight
        // Swords; Small before Medium Shields; Bows before Light Bows) and
        // sortId breaks ties within a subCategory (DggB before DggA).
        type Spec = { tok: string; cat: string; sub: string; grp: number; sid: number; acq: number };
        const specs: Spec[] = [
            { tok: 'DggA', cat: 'melee_armaments',      sub: 'Daggers',         grp: 200, sid: 20, acq: 6 },
            { tok: 'DggB', cat: 'melee_armaments',      sub: 'Daggers',         grp: 200, sid: 10, acq: 7 },
            { tok: 'SS',   cat: 'melee_armaments',      sub: 'Straight Swords', grp: 210, sid: 5,  acq: 5 },
            { tok: 'SmS',  cat: 'shields',              sub: 'Small Shields',   grp: 50,  sid: 5,  acq: 3 },
            { tok: 'MdS',  cat: 'shields',              sub: 'Medium Shields',  grp: 60,  sid: 5,  acq: 4 },
            { tok: 'Bow',  cat: 'ranged_and_catalysts', sub: 'Bows',            grp: 5,   sid: 5,  acq: 1 },
            { tok: 'LBow', cat: 'ranged_and_catalysts', sub: 'Light Bows',      grp: 6,   sid: 5,  acq: 2 },
        ];
        // Canonical Default result: melee (Daggers by sortId, then Straight
        // Swords), then shields (Small, Medium), then ranged (Bows, Light Bows).
        const desiredToks = ['DggB', 'DggA', 'SS', 'SmS', 'MdS', 'Bow', 'LBow'];

        const build = (container: 'inventory' | 'storage', base: number, tag: string) =>
            specs.map((s, i) =>
                makeItem(`hnd:0x${(base + i).toString(16).toUpperCase()}`, container, i, {
                    name: `${tag} ${s.tok}`, category: s.cat, subCategory: s.sub,
                    sortGroupId: s.grp, sortId: s.sid, acquisitionIndex: s.acq,
                    isWeapon: s.cat === 'melee_armaments',
                }),
            );
        const byToks = (items: editor.EditableItem[], tag: string, toks: string[]) =>
            toks.map(t => items.find(it => it.name === `${tag} ${t}`)!);
        const acqOrder = (items: editor.EditableItem[]) => [...items].sort((a, b) => a.acquisitionIndex - b.acquisitionIndex);

        const inv = build('inventory', 0x80800001, 'Inv');
        const sto = build('storage', 0x80800011, 'Sto');
        const invDesired = byToks(inv, 'Inv', desiredToks);
        const stoDesired = byToks(sto, 'Sto', desiredToks);

        // First: acquisition-asc moves away from the hierarchy. Second: Default restores it.
        mocks.ReorderInventoryWorkspaceItems
            .mockResolvedValueOnce(makeSnapshot({ inventory: acqOrder(inv), storage: acqOrder(sto), dirty: true }))
            .mockResolvedValueOnce(makeSnapshot({ inventory: invDesired, storage: stoDesired, dirty: true }));

        await mount(makeSnapshot({ inventory: inv, storage: sto }));

        await act(async () => {
            fireEvent.change(screen.getByLabelText('Sort'), { target: { value: 'acquisition-asc' } });
        });
        await waitFor(() => expect(mocks.ReorderInventoryWorkspaceItems).toHaveBeenCalledTimes(1));

        await act(async () => {
            fireEvent.change(screen.getByLabelText('Sort'), { target: { value: '' } });
        });

        // Default is NOT a no-op: it issues a reorder into the section hierarchy.
        await waitFor(() => expect(mocks.ReorderInventoryWorkspaceItems).toHaveBeenCalledTimes(2));
        const [, invUIDs, stoUIDs] = mocks.ReorderInventoryWorkspaceItems.mock.calls[1];
        expect(invUIDs).toEqual(invDesired.map(it => it.uid));
        expect(stoUIDs).toEqual(stoDesired.map(it => it.uid));
        // Explicitly neither acquisition order nor a flat sortGroupId sort.
        expect(invUIDs).not.toEqual(acqOrder(inv).map(it => it.uid));
        const flatByGroup = [...inv].sort((a, b) => (a.sortGroupId ?? 0) - (b.sortGroupId ?? 0));
        expect(invUIDs).not.toEqual(flatByGroup.map(it => it.uid));

        // Grid renders contiguous, correctly ordered sections in both containers.
        const expectOrder = (tag: string) => {
            const nodes = desiredToks.map(t => screen.getByText(`${tag} ${t}`));
            for (let i = 0; i < nodes.length - 1; i++) {
                expect(nodes[i].compareDocumentPosition(nodes[i + 1]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
            }
        };
        await waitFor(() => expectOrder('Inv'));
        await waitFor(() => expectOrder('Sto'));
    });

    // Five out-of-order items per container so sorting is a real permutation.
    // weight ascends with position, so weight-desc reverses each container.
    function fiveByWeight(container: 'inventory' | 'storage', base: number, tag: string) {
        // Deliberately scrambled input order; UIDs stay tied to weight.
        const w = [3, 1, 5, 2, 4];
        return w.map((weight, i) =>
            makeItem(`hnd:0x${(base + i).toString(16).toUpperCase()}`, container, i, {
                name: `${tag} ${weight}`, weight,
            }),
        );
    }

    it('shared Sort issues ONE atomic reorder and both grids render the final snapshot', async () => {
        const inv = fiveByWeight('inventory', 0x80800001, 'Inv');
        const sto = fiveByWeight('storage', 0x80800011, 'Sto');
        const desc = (items: editor.EditableItem[]) => [...items].sort((a, b) => (b.weight ?? 0) - (a.weight ?? 0));
        const finalSnap = makeSnapshot({ inventory: desc(inv), storage: desc(sto), dirty: true });
        mocks.ReorderInventoryWorkspaceItems.mockResolvedValue(finalSnap);

        await mount(makeSnapshot({ inventory: inv, storage: sto }));

        await act(async () => {
            fireEvent.change(screen.getByLabelText('Sort'), { target: { value: 'weight-desc' } });
        });

        // Exactly one atomic reorder call — no per-item MoveInventoryWorkspaceItem loop.
        await waitFor(() => {
            expect(mocks.ReorderInventoryWorkspaceItems).toHaveBeenCalledTimes(1);
        });
        expect(mocks.MoveInventoryWorkspaceItem).not.toHaveBeenCalled();
        const [sid, invUIDs, stoUIDs] = mocks.ReorderInventoryWorkspaceItems.mock.calls[0];
        expect(sid).toBe('ses-tab');
        expect(invUIDs).toEqual(desc(inv).map(it => it.uid));
        expect(stoUIDs).toEqual(desc(sto).map(it => it.uid));

        // Both rendered grids reflect the one final snapshot: heavy (5) before light (1).
        await waitFor(() => {
            const heavy = screen.getByText('Sto 5');
            const light = screen.getByText('Sto 1');
            expect(heavy.compareDocumentPosition(light) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        });
        await waitFor(() => {
            const heavy = screen.getByText('Inv 5');
            const light = screen.getByText('Inv 1');
            expect(heavy.compareDocumentPosition(light) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        });
    });

    it('blocks an overlapping shared-sort request while one is running', async () => {
        const inv = fiveByWeight('inventory', 0x80800001, 'Inv');
        const sto = fiveByWeight('storage', 0x80800011, 'Sto');
        // The reorder never resolves, so the shared sort stays in flight.
        mocks.ReorderInventoryWorkspaceItems.mockReturnValue(new Promise<never>(() => {}));

        await mount(makeSnapshot({ inventory: inv, storage: sto }));
        const sort = screen.getByLabelText('Sort');

        await act(async () => { fireEvent.change(sort, { target: { value: 'weight-desc' } }); });
        // Second change arrives mid-flight and must be ignored.
        await act(async () => { fireEvent.change(sort, { target: { value: 'weight-asc' } }); });

        // The first mode's label stuck; the racing selection did not take over.
        expect((screen.getByLabelText('Sort') as HTMLSelectElement).value).toBe('weight-desc');
        // Only one atomic reorder was issued; no interleaved second run.
        expect(mocks.ReorderInventoryWorkspaceItems).toHaveBeenCalledTimes(1);
    });

    it('the Category dropdown switches the filtered view', async () => {
        const weapon = makeItem('hnd:0x80800001', 'inventory', 0, { name: 'A Weapon' });
        const talisman = makeItem('hnd:0x80800002', 'inventory', 1, {
            name: 'A Talisman', category: 'talismans', isWeapon: false, isTalisman: true,
        });
        await mount(makeSnapshot({ inventory: [weapon, talisman] }));

        // Default category is Weapons.
        expect(screen.getByText('A Weapon')).toBeInTheDocument();
        expect(screen.queryByText('A Talisman')).not.toBeInTheDocument();

        await act(async () => {
            fireEvent.change(screen.getByLabelText('Category'), { target: { value: 'talismans' } });
        });

        expect(screen.getByText('A Talisman')).toBeInTheDocument();
        expect(screen.queryByText('A Weapon')).not.toBeInTheDocument();
    });

    it('renders every item at once as stacked cards, with the simplified toolbar', async () => {
        const items = Array.from({ length: 31 }, (_, idx) =>
            makeItem(`hnd:0x8080${idx.toString(16).padStart(4, '0')}`, 'inventory', idx),
        );
        await mount(makeSnapshot({ inventory: items }));

        // Card scrolling keeps all items mounted — the 31st spills onto card 2.
        expect(screen.getByText('Sword 0')).toBeInTheDocument();
        expect(screen.getByText('Sword 29')).toBeInTheDocument();
        expect(screen.getByText('Sword 30')).toBeInTheDocument();

        // Contextual labels precede each item count.
        expect(screen.getByText('Storage: 0 items')).toBeInTheDocument();
        expect(screen.getByText('Inventory: 31 items')).toBeInTheDocument();

        // Shared toolbar dropdowns replace the tab-button row and per-column sort groups.
        expect(screen.getByLabelText('Category')).toBeInTheDocument();
        expect(screen.getByLabelText('Sort')).toBeInTheDocument();

        // Gone: page controls, tab buttons, per-column sort buttons, per-column Add.
        expect(screen.queryByRole('button', { name: /next page/i })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /previous page/i })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /^Weapons$/i })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Weight ↓/ })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /^\+ Add$/i })).not.toBeInTheDocument();
        // The global Add item action remains.
        expect(screen.getByRole('button', { name: /Add Item/i })).toBeInTheDocument();
    });

    // 90 items → 3 cards, so a card step is clamped at neither end. Returns the
    // frame element with faked layout metrics and a scrollTo spy.
    function mountPagedFrame(): { frame: HTMLElement; scrollTo: ReturnType<typeof vi.fn> } {
        const frame = screen.getByText('Sword 0').closest('[class*="overflow-y-auto"]') as HTMLElement;
        Object.defineProperty(frame, 'clientHeight', { value: 100, configurable: true });
        Object.defineProperty(frame, 'scrollHeight', { value: 100 + 2 * CARD_STEP_PX, configurable: true });
        frame.scrollTop = 0;
        const scrollTo = vi.fn();
        frame.scrollTo = scrollTo as unknown as typeof frame.scrollTo;
        return { frame, scrollTo };
    }

    it('a single mouse-wheel notch pages the frame exactly one 30-cell card', async () => {
        const items = Array.from({ length: 90 }, (_, idx) =>
            makeItem(`hnd:0x8080${idx.toString(16).padStart(4, '0')}`, 'inventory', idx),
        );
        await mount(makeSnapshot({ inventory: items }));
        const { frame, scrollTo } = mountPagedFrame();

        // One physical notch fires a burst of wheel events; they must coalesce
        // into a single one-card step, never a 60-item / two-card jump.
        await act(async () => {
            fireEvent.wheel(frame, { deltaY: 120 });
            fireEvent.wheel(frame, { deltaY: 120 });
            fireEvent.wheel(frame, { deltaY: 120 });
        });

        expect(scrollTo).toHaveBeenCalledTimes(1);
        expect(scrollTo).toHaveBeenCalledWith({ top: CARD_STEP_PX, behavior: 'smooth' });
    });

    it('a momentum wheel event that arrives after the old 180 ms lock — while the scroll is still between cards — does not page a second card', async () => {
        const items = Array.from({ length: 90 }, (_, idx) =>
            makeItem(`hnd:0x8080${idx.toString(16).padStart(4, '0')}`, 'inventory', idx),
        );
        await mount(makeSnapshot({ inventory: items }));
        const { frame, scrollTo } = mountPagedFrame();

        vi.useFakeTimers();
        try {
            // Leading event of the gesture starts a smooth move toward card 1.
            fireEvent.wheel(frame, { deltaY: 120 });
            expect(scrollTo).toHaveBeenCalledTimes(1);
            expect(scrollTo).toHaveBeenCalledWith({ top: CARD_STEP_PX, behavior: 'smooth' });

            // Time passes the old 180 ms lock window WITHOUT a scrollend — the
            // smooth scroll has not settled (scrollTop still 0, between cards).
            vi.advanceTimersByTime(220);

            // A late momentum event from the SAME gesture arrives. It must be
            // swallowed, not scheduled as card 2 / 60 items.
            fireEvent.wheel(frame, { deltaY: 120 });
            expect(scrollTo).toHaveBeenCalledTimes(1);
        } finally {
            vi.useRealTimers();
        }
    });

    it('reorders an item onto an empty cell on the trailing card (drops to the end)', async () => {
        // 30 items exactly fill card 1; card 2 is entirely empty (absIdx 30–59).
        const items = Array.from({ length: 30 }, (_, idx) =>
            makeItem(`hnd:0x8080${idx.toString(16).padStart(4, '0')}`, 'inventory', idx),
        );
        mocks.MoveInventoryWorkspaceItem.mockResolvedValue(makeSnapshot({ inventory: items, dirty: true }));
        await mount(makeSnapshot({ inventory: items }));

        const firstTile = screen.getByText('Sword 0').closest('[draggable="true"]')!;
        const emptyCell = screen.getByTestId('inv-empty-30'); // first cell of the trailing card

        // Two acts so the drag-source state flushes before the drop reads it.
        await act(async () => { fireEvent.dragStart(firstTile); });
        await act(async () => { fireEvent.drop(emptyCell); });

        // Dropping item 0 past the end reorders it to the last absolute position (29).
        await waitFor(() => {
            expect(mocks.MoveInventoryWorkspaceItem).toHaveBeenCalledWith(
                'ses-tab', 'hnd:0x80800000', 'inventory', 29,
            );
        });
    });

    it('renders Save changes button disabled when not dirty', async () => {
        await mount(makeSnapshot({ inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: false }));
        const save = screen.getByRole('button', { name: /Save changes/i });
        expect(save).toBeDisabled();
    });

    it('renders Save changes button enabled when snapshot is dirty', async () => {
        await mount(makeSnapshot({ inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: true }));
        const save = screen.getByRole('button', { name: /Save changes/i });
        expect(save).not.toBeDisabled();
    });

    it('renders Unsaved badge when dirty', async () => {
        await mount(makeSnapshot({ inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: true }));
        expect(screen.getByText('Unsaved')).toBeInTheDocument();
    });

    it('clicking Save changes opens confirm, then calls SaveInventoryWorkspaceChanges', async () => {
        const dirtySnap = makeSnapshot({ sessionID: 'ses-save', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: true });
        const savedSnap = makeSnapshot({ sessionID: 'ses-save', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: false });
        const onMutate = vi.fn();
        mocks.SaveInventoryWorkspaceChanges.mockResolvedValue(savedSnap);
        await mount(dirtySnap, onMutate);

        const save = screen.getByRole('button', { name: /Save changes/i });
        await act(async () => { fireEvent.click(save); });

        // Confirm modal appears with explicit Save button.
        const confirmSave = await screen.findByRole('button', { name: /^Save$/i });
        await act(async () => { fireEvent.click(confirmSave); });

        await waitFor(() => {
            expect(mocks.SaveInventoryWorkspaceChanges).toHaveBeenCalledWith('ses-save');
        });
        // Successful save fires onMutate so parent can refresh undo depth etc.
        await waitFor(() => {
            expect(onMutate).toHaveBeenCalled();
        });
    });

    it('clicking Discard opens confirm, then calls DiscardInventoryEditSession + restarts session', async () => {
        const dirtySnap = makeSnapshot({ sessionID: 'ses-disc', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: true });
        const freshSnap = makeSnapshot({ sessionID: 'ses-disc2', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: false });
        mocks.StartInventoryEditSession
            .mockResolvedValueOnce(dirtySnap)
            .mockResolvedValueOnce(freshSnap);

        let utils!: ReturnType<typeof render>;
        await act(async () => {
            utils = render(<SortOrderTab charIndex={0} inventoryVersion={1} />);
        });
        await waitFor(() => expect(mocks.StartInventoryEditSession).toHaveBeenCalledTimes(1));

        const discard = utils.getByRole('button', { name: /^Discard$/i });
        await act(async () => { fireEvent.click(discard); });

        // Wait until the confirm modal heading is visible.
        await screen.findByText(/Discard Inventory Changes/i);
        const confirms = screen.getAllByRole('button', { name: /^Discard$/i });
        expect(confirms.length).toBe(2);
        // The modal confirm button is the one whose closest container holds
        // the heading "Discard Inventory Changes".
        const modalConfirm = confirms.find(btn =>
            btn.closest('div')?.parentElement?.textContent?.includes('Discard Inventory Changes'),
        );
        expect(modalConfirm).toBeDefined();
        await act(async () => {
            fireEvent.click(modalConfirm!);
            // Let pending microtasks settle so workspace.discard() (async)
            // can begin and await DiscardInventoryEditSession.
            await Promise.resolve();
        });

        await waitFor(() => {
            expect(mocks.DiscardInventoryEditSession).toHaveBeenCalledWith('ses-disc');
        });
        await waitFor(() => {
            // Second StartInventoryEditSession call to re-seed after discard.
            expect(mocks.StartInventoryEditSession).toHaveBeenCalledTimes(2);
        });
    });
});

describe('buildSortOrderCards', () => {
    const items = (n: number) =>
        Array.from({ length: n }, (_, idx) => makeItem(`u${idx}`, 'inventory', idx));

    it('empty list produces one empty 30-cell card', () => {
        const cards = buildSortOrderCards([]);
        expect(cards).toHaveLength(1);
        expect(cards[0]).toHaveLength(30);
        expect(cards[0].every(c => c === null)).toBe(true);
    });

    it('1–29 items produce a single 30-cell card', () => {
        for (const n of [1, 15, 29]) {
            const cards = buildSortOrderCards(items(n));
            expect(cards).toHaveLength(1);
            expect(cards[0]).toHaveLength(30);
            expect(cards[0].filter(Boolean)).toHaveLength(n);
        }
    });

    it('exactly 30 items produce two cards, the second empty', () => {
        const cards = buildSortOrderCards(items(30));
        expect(cards).toHaveLength(2);
        expect(cards[0].filter(Boolean)).toHaveLength(30);
        expect(cards[1]).toHaveLength(30);
        expect(cards[1].every(c => c === null)).toBe(true);
    });

    it('31 items produce two cards', () => {
        const cards = buildSortOrderCards(items(31));
        expect(cards).toHaveLength(2);
        expect(cards[1].filter(Boolean)).toHaveLength(1);
    });

    it('never omits or duplicates items across cards, order preserved', () => {
        const list = items(47);
        const flat = buildSortOrderCards(list).flat().filter(Boolean);
        expect(flat).toHaveLength(47);
        expect(new Set(flat.map(i => i!.uid)).size).toBe(47);
        flat.forEach((it, idx) => expect(it!.uid).toBe(list[idx].uid));
    });

    it('pads every card to exactly 30 cells', () => {
        for (const n of [0, 5, 30, 31, 60]) {
            for (const card of buildSortOrderCards(items(n))) {
                expect(card).toHaveLength(30);
            }
        }
    });
});

describe('getCardAutoScrollDirection', () => {
    const top = 100;
    const bottom = 708;

    it('arms previous-card scrolling only in the top edge zone', () => {
        expect(getCardAutoScrollDirection(120, top, bottom, true, true)).toBe(-1);
        expect(getCardAutoScrollDirection(120, top, bottom, false, true)).toBe(0);
    });

    it('arms next-card scrolling only in the bottom edge zone', () => {
        expect(getCardAutoScrollDirection(690, top, bottom, true, true)).toBe(1);
        expect(getCardAutoScrollDirection(690, top, bottom, true, false)).toBe(0);
    });

    it('does not arm auto-scroll in the middle of a frame', () => {
        expect(getCardAutoScrollDirection(400, top, bottom, true, true)).toBe(0);
    });
});

// Phase 8A removed the JSON template export/import/apply surface from
// SortOrderTab. Templates now live exclusively in the global Templates
// shell modal (YAML-only). Tests for the removed Export Template menu,
// Import Template Preview, Apply Template, and the weapon-level
// override panel were dropped together with the code paths they
// exercised. The remaining tests above cover session lifecycle, dirty/
// save UX and confirm modals — all unchanged by Phase 8A.
