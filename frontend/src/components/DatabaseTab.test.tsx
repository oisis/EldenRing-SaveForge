import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { db } from '../../wailsjs/go/models';
import type { AddSettings } from '../App';

// vitest's jsdom can run with an opaque origin, which leaves `localStorage`
// undefined. DatabaseTab reads localStorage during render (ban-risk opt-out,
// full-chaos flag), so provide a minimal in-memory stub when it's missing.
// Test-only; does not touch production code or the shared setup file.
if (typeof globalThis.localStorage === 'undefined') {
    const store = new Map<string, string>();
    Object.defineProperty(globalThis, 'localStorage', {
        configurable: true,
        value: {
            getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
            setItem: (k: string, v: string) => { store.set(k, String(v)); },
            removeItem: (k: string) => { store.delete(k); },
            clear: () => { store.clear(); },
            key: (i: number) => Array.from(store.keys())[i] ?? null,
            get length() { return store.size; },
        },
    });
}

// Characterization tests — they freeze the CURRENT observable behavior of the
// flows that the upcoming presentational extraction (BanRiskWarningModal) will
// touch. They intentionally do NOT cover the full handleAdd matrix (capacity,
// quantity split, repair-retry, owned-by-baseID) — that belongs to a later
// checkpoint, before the mutation logic itself is refactored.

vi.mock('../../wailsjs/go/main/App', () => ({
    GetItemList: vi.fn(),
    GetItemListChunk: vi.fn(),
    GetInfuseTypes: vi.fn(),
    AddItemsToCharacter: vi.fn(),
    AddItemsToCharacterWithGameLimits: vi.fn(),
    GetCharacter: vi.fn(),
    GetBellBearings: vi.fn(),
    GetSlotCapacity: vi.fn(),
}));

vi.mock('../lib/toast', () => {
    const fn = vi.fn() as unknown as Record<string, unknown> & ((...args: unknown[]) => void);
    fn.success = vi.fn();
    fn.error = vi.fn();
    return { default: fn };
});

// Mark every item as a favorite so the "Add Favorites" entry point is available
// without driving the unlabeled selection checkboxes — keeps these tests on a
// visible, accessible-name path. The favorites entry point calls openModal with
// the same flow as "Add Selected".
vi.mock('../state/favorites', () => ({
    useFavorites: () => ({ isFav: () => true, toggle: vi.fn() }),
}));

import * as App from '../../wailsjs/go/main/App';
import { DatabaseTab, effectiveCap } from './DatabaseTab';

const mocks = App as unknown as Record<string, ReturnType<typeof vi.fn>>;

const BAN_ITEM_ID = 0x2000;

describe('effectiveCap', () => {
    const pebble = db.ItemEntry.createFrom({
        id: 0x40000FA0,
        name: 'Glintstone Pebble',
        category: 'sorceries',
        maxInventory: 1,
        maxStorage: 0,
        gameMaxInventory: 99,
        gameMaxStorage: 600,
        gameMaxInventoryKnown: true,
        gameMaxStorageKnown: true,
        maxUpgrade: 0,
        iconPath: '',
        flags: [],
    });

    it('keeps conservative caps in Normal Mode', () => {
        expect(effectiveCap(pebble, 'inv', 0, false)).toBe(1);
        expect(effectiveCap(pebble, 'storage', 0, false)).toBe(0);
    });

    it('uses technical game caps in Full Chaos Mode', () => {
        expect(effectiveCap(pebble, 'inv', 0, true)).toBe(99);
        expect(effectiveCap(pebble, 'storage', 0, true)).toBe(600);
    });

    it('falls back to conservative caps when game limits are unknown', () => {
        const unknown = db.ItemEntry.createFrom({
            ...pebble,
            gameMaxInventoryKnown: false,
            gameMaxStorageKnown: false,
        });
        expect(effectiveCap(unknown, 'inv', 0, true)).toBe(1);
        expect(effectiveCap(unknown, 'storage', 0, true)).toBe(0);
    });
});

function makeBanRiskItem(): db.ItemEntry {
    return db.ItemEntry.createFrom({
        id: BAN_ITEM_ID,
        name: 'Forbidden Trinket',
        category: 'key_items',
        maxInventory: 1,
        maxStorage: 0,
        maxUpgrade: 0,
        iconPath: 'items/key_items/forbidden_trinket.png',
        flags: ['ban_risk'],
    });
}

function makeBlessingOfMarika(): db.ItemEntry {
    return db.ItemEntry.createFrom({
        id: 0x401E8804,
        name: 'Blessing of Marika',
        category: 'tools',
        subCategory: 'Consumables',
        maxInventory: 1,
        maxStorage: 600,
        maxUpgrade: 0,
        iconPath: 'items/tools/blessing_of_marika.png',
        flags: ['dlc'],
    });
}

const DEFAULT_ADD_SETTINGS: AddSettings = {
    upgrade25: 0, upgrade10: 0, infuseOffset: 0, upgradeAsh: 0,
    talismansHighestOnly: false, includeAshenCapital: false,
};

function tabElement(overrides: Partial<Parameters<typeof DatabaseTab>[0]> = {}) {
    return (
        <DatabaseTab
            columnVisibility={{ id: true, category: true }}
            platform="PC"
            charIndex={0}
            inventoryVersion={0}
            onItemsAdded={vi.fn()}
            addSettings={DEFAULT_ADD_SETTINGS}
            showFlaggedItems={true}
            category="key_items"
            setCategory={vi.fn()}
            {...overrides}
        />
    );
}

function renderTab(overrides: Partial<Parameters<typeof DatabaseTab>[0]> = {}) {
    return render(tabElement(overrides));
}

// Two subcategories under 'tools' (Consumables x2, Great Runes x1) to exercise
// the subcategory dropdown, filtering, select-all scope and the Owned count.
function makeToolItems(): db.ItemEntry[] {
    const base = { category: 'tools', maxInventory: 99, maxStorage: 600, maxUpgrade: 0, iconPath: '', flags: [] as string[] };
    return [
        db.ItemEntry.createFrom({ ...base, id: 0x11, name: 'Cured Meat', subCategory: 'Consumables' }),
        db.ItemEntry.createFrom({ ...base, id: 0x12, name: 'Rowa Fruit', subCategory: 'Consumables' }),
        db.ItemEntry.createFrom({ ...base, id: 0x13, name: 'Golden Rune', subCategory: 'Great Runes' }),
    ];
}

// Upgradeable weapon row — drives the Max Up column visibility/value.
function makeWeapon(id: number, name: string, maxUpgrade: number): db.ItemEntry {
    return db.ItemEntry.createFrom({
        id, name, category: 'melee_armaments', subCategory: '',
        maxInventory: 1, maxStorage: 0, maxUpgrade, iconPath: '', flags: [],
    });
}

function ownedVm(id: number, quantity: number) {
    return { id, baseId: id, name: '', category: 'Item', subCategory: 'tools', quantity, maxInventory: 99, maxStorage: 600, flags: [] };
}

beforeEach(() => {
    localStorage.clear();
    mocks.GetInfuseTypes.mockResolvedValue([]);
    mocks.GetCharacter.mockResolvedValue({ inventory: [], storage: [], clearCount: 0 });
    mocks.GetBellBearings.mockResolvedValue([]);
    mocks.GetSlotCapacity.mockResolvedValue({ inventoryUsed: 0, inventoryMax: 2688, storageUsed: 0, storageMax: 1920 });
    mocks.GetItemList.mockResolvedValue([makeBanRiskItem()]);
    mocks.GetItemListChunk.mockResolvedValue([]);
    mocks.AddItemsToCharacter.mockResolvedValue({
        added: 1, requested: 1, trimmed: [], capHit: '',
        freeInv: 0, freeStore: 0, neededInv: 0, neededStore: 0,
    });
    mocks.AddItemsToCharacterWithGameLimits.mockResolvedValue({
        added: 1, requested: 1, trimmed: [], capHit: '',
        freeInv: 0, freeStore: 0, neededInv: 0, neededStore: 0,
    });
});

afterEach(() => {
    vi.clearAllMocks();
});

describe('DatabaseTab', () => {
    // basic render of the active component with a real, loaded item.
    it('renders a loaded item from the active component', async () => {
        renderTab();
        // Grid view renders items directly (no row virtualizer), so the item is
        // deterministically present in jsdom.
        fireEvent.click(screen.getByTitle('Grid view'));
        expect(await screen.findByText('Forbidden Trinket')).toBeInTheDocument();
    });

    it('hides risk-flagged items when showFlaggedItems is off and Chaos is off', async () => {
        renderTab({ showFlaggedItems: false });
        fireEvent.click(screen.getByTitle('Grid view'));
        await waitFor(() => expect(mocks.GetItemList).toHaveBeenCalled());
        expect(screen.queryByText('Forbidden Trinket')).not.toBeInTheDocument();
    });

    // Visibility is now decoupled from cap mode: expanded_limits raises caps but
    // keeps cut/ban-risk items hidden. Only the chaos profile (which makes App
    // pass showFlaggedItems=true) reveals them.
    it('Expanded Limits keeps risk-flagged items hidden (visibility is prop-driven)', async () => {
        localStorage.setItem('setting:safetyProfile', 'expanded_limits');
        renderTab({ showFlaggedItems: false });
        fireEvent.click(screen.getByTitle('Grid view'));
        await waitFor(() => expect(mocks.GetItemList).toHaveBeenCalled());
        expect(screen.queryByText('Forbidden Trinket')).not.toBeInTheDocument();
    });

    it('Chaos reveals risk-flagged items via the showFlaggedItems prop', async () => {
        localStorage.setItem('setting:safetyProfile', 'chaos');
        renderTab({ showFlaggedItems: true });
        fireEvent.click(screen.getByTitle('Grid view'));
        expect(await screen.findByText('Forbidden Trinket')).toBeInTheDocument();
    });

    it('counts hybrid inventory and storage stack quantities', async () => {
        const blessing = makeBlessingOfMarika();
        mocks.GetItemList.mockResolvedValue([blessing]);
        mocks.GetCharacter.mockResolvedValue({
            inventory: [{
                id: blessing.id,
                baseId: blessing.id,
                name: blessing.name,
                category: 'Item',
                subCategory: 'tools',
                quantity: 1,
                maxInventory: 1,
                maxStorage: 600,
                flags: ['dlc'],
            }],
            storage: [{
                id: blessing.id,
                baseId: blessing.id,
                name: blessing.name,
                category: 'Item',
                subCategory: 'tools',
                quantity: 600,
                maxInventory: 1,
                maxStorage: 600,
                flags: ['dlc'],
            }],
            clearCount: 0,
        });

        renderTab({ category: 'tools' });
        fireEvent.click(screen.getByTitle('Grid view'));

        expect(await screen.findByText('Blessing of Marika')).toBeInTheDocument();
        expect(screen.getByText('I:1')).toBeInTheDocument();
        expect(screen.getByText('S:600')).toBeInTheDocument();
    });

    // ban-risk warning: cancel must not mutate the save.
    it('ban-risk warning: Cancel aborts without calling AddItemsToCharacter', async () => {
        renderTab();
        fireEvent.click(await screen.findByRole('button', { name: /Add Favorites/i }));

        expect(await screen.findByText('Ban Risk Warning')).toBeInTheDocument();

        fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

        await waitFor(() =>
            expect(screen.queryByText('Ban Risk Warning')).not.toBeInTheDocument());
        expect(mocks.AddItemsToCharacter).not.toHaveBeenCalled();
    });

    // ban-risk warning: "Add Anyway" continues the existing multi-step
    // flow (warning -> confirm modal -> Add -> mutation).
    it('ban-risk warning: Add Anyway proceeds to confirm and Add triggers AddItemsToCharacter', async () => {
        renderTab();
        fireEvent.click(await screen.findByRole('button', { name: /Add Favorites/i }));

        fireEvent.click(await screen.findByRole('button', { name: 'Add Anyway' }));

        // Warning dismissed, confirm modal now open.
        await waitFor(() =>
            expect(screen.queryByText('Ban Risk Warning')).not.toBeInTheDocument());
        const addBtn = await screen.findByRole('button', { name: /^Add$/ });
        fireEvent.click(addBtn);

        await waitFor(() =>
            expect(mocks.AddItemsToCharacter).toHaveBeenCalledTimes(1));
        // Single non-stackable item, inventory only (maxStorage 0): invQty=1, storQty=0.
        expect(mocks.AddItemsToCharacter).toHaveBeenCalledWith(
            0, [BAN_ITEM_ID], 0, 0, 0, 0, 1, 0);
    });

    // error modal: the capacity-exceeded path is the smallest deterministic way
    // to open the (purely presentational) error modal — AddItemsToCharacter
    // returns a capHit instead of throwing, so handleAdd renders the modal from
    // state. Freezes the current title/message and that OK only closes the UI.
    it('error modal: capacity exceeded renders the error and OK closes it without an additional mutation', async () => {
        // Single non-stackable item makes exactly one AddItemsToCharacter call;
        // returning capHit drives the Capacity Exceeded branch (not the throw path).
        mocks.AddItemsToCharacter.mockResolvedValue({
            added: 0, requested: 1, trimmed: [], capHit: 'inventory_full',
            freeInv: 0, freeStore: 0, neededInv: 5, neededStore: 0,
        });

        renderTab();
        fireEvent.click(await screen.findByRole('button', { name: /Add Favorites/i }));
        fireEvent.click(await screen.findByRole('button', { name: 'Add Anyway' }));
        const addBtn = await screen.findByRole('button', { name: /^Add$/ });
        fireEvent.click(addBtn);

        // The mutation endpoint was actually reached exactly once.
        await waitFor(() =>
            expect(mocks.AddItemsToCharacter).toHaveBeenCalledTimes(1));

        // Actual rendered title for capHit 'inventory_full' and a characteristic
        // message fragment (need/free line built in handleAdd).
        expect(await screen.findByText('Inventory Full')).toBeInTheDocument();
        expect(screen.getByText(/Inventory: need 5, free 0/)).toBeInTheDocument();

        // OK is the only action; it closes the modal via setErrorModal(null).
        fireEvent.click(screen.getByRole('button', { name: 'OK' }));
        await waitFor(() =>
            expect(screen.queryByText('Inventory Full')).not.toBeInTheDocument());

        // Closing is UI-only: no second mutation triggered.
        expect(mocks.AddItemsToCharacter).toHaveBeenCalledTimes(1);
    });

    function makePebble(): db.ItemEntry {
        return db.ItemEntry.createFrom({
            id: 0x40000FA0,
            name: 'Glintstone Pebble',
            category: 'sorceries',
            maxInventory: 1,
            maxStorage: 0,
            gameMaxInventory: 99,
            gameMaxStorage: 600,
            gameMaxInventoryKnown: true,
            gameMaxStorageKnown: true,
            maxUpgrade: 0,
            iconPath: '',
            flags: [],
        });
    }

    it('Expanded Limits routes adds through the game-limit endpoint', async () => {
        localStorage.setItem('setting:safetyProfile', 'expanded_limits');
        mocks.GetItemList.mockResolvedValue([makePebble()]);

        renderTab({category: 'sorceries'});
        fireEvent.click(await screen.findByRole('button', {name: /Add Favorites/i}));
        // Banner must read Expanded Limits, never Chaos.
        expect(await screen.findByText(/Expanded Limits — technical game caps/i)).toBeInTheDocument();
        expect(screen.queryByText(/Chaos Mode/i)).not.toBeInTheDocument();
        expect(screen.queryByText(/Full Chaos Mode/i)).not.toBeInTheDocument();
        fireEvent.click(screen.getByRole('button', {name: /^Add$/}));

        await waitFor(() =>
            expect(mocks.AddItemsToCharacterWithGameLimits).toHaveBeenCalledTimes(1));
        expect(mocks.AddItemsToCharacter).not.toHaveBeenCalled();
    });

    it('Chaos routes adds through the game-limit endpoint', async () => {
        localStorage.setItem('setting:safetyProfile', 'chaos');
        mocks.GetItemList.mockResolvedValue([makePebble()]);

        renderTab({category: 'sorceries'});
        fireEvent.click(await screen.findByRole('button', {name: /Add Favorites/i}));
        fireEvent.click(await screen.findByRole('button', {name: /^Add$/}));

        await waitFor(() =>
            expect(mocks.AddItemsToCharacterWithGameLimits).toHaveBeenCalledTimes(1));
        expect(mocks.AddItemsToCharacter).not.toHaveBeenCalled();
    });

    it('Safe profile routes adds through the standard endpoint', async () => {
        // no safetyProfile set → defaults to safe
        mocks.GetItemList.mockResolvedValue([makePebble()]);

        renderTab({category: 'sorceries'});
        fireEvent.click(await screen.findByRole('button', {name: /Add Favorites/i}));
        fireEvent.click(await screen.findByRole('button', {name: /^Add$/}));

        await waitFor(() =>
            expect(mocks.AddItemsToCharacter).toHaveBeenCalledTimes(1));
        expect(mocks.AddItemsToCharacterWithGameLimits).not.toHaveBeenCalled();
    });

    // --- Subcategory filter + toolbar cleanup (task 1) ---

    it('resets the subcategory filter to All when the category changes', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems());
        const { rerender } = render(tabElement({ category: 'tools' }));

        const sub = await screen.findByLabelText('Subcategory') as HTMLSelectElement;
        await waitFor(() => expect(sub.options.length).toBeGreaterThan(1));
        fireEvent.change(sub, { target: { value: 'Great Runes' } });
        expect(sub.value).toBe('Great Runes');

        rerender(tabElement({ category: 'ashes' }));
        await waitFor(() =>
            expect((screen.getByLabelText('Subcategory') as HTMLSelectElement).value).toBe('all'));
    });

    it('filters the grid by the selected subcategory', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems());
        renderTab({ category: 'tools' });
        fireEvent.click(screen.getByTitle('Grid view'));

        expect(await screen.findByText('Cured Meat')).toBeInTheDocument();
        fireEvent.change(screen.getByLabelText('Subcategory'), { target: { value: 'Great Runes' } });

        expect(screen.queryByText('Cured Meat')).not.toBeInTheDocument();
        expect(screen.getByText('Golden Rune')).toBeInTheDocument();
    });

    it('Select all selects only the filtered visible items', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems());
        renderTab({ category: 'tools' });

        const sub = await screen.findByLabelText('Subcategory');
        fireEvent.change(sub, { target: { value: 'Consumables' } });
        fireEvent.click(screen.getByTitle('Select all'));

        // 2 Consumables in scope → "Add Selected (2)", never the full 3.
        expect(await screen.findByRole('button', { name: 'Add Selected (2)' })).toBeInTheDocument();
    });

    it('prunes hidden selected items when the subcategory changes', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems());
        renderTab({ category: 'tools' });

        await screen.findByLabelText('Subcategory');
        fireEvent.click(screen.getByTitle('Select all'));
        expect(await screen.findByRole('button', { name: 'Add Selected (3)' })).toBeInTheDocument();

        fireEvent.change(screen.getByLabelText('Subcategory'), { target: { value: 'Great Runes' } });

        await waitFor(() =>
            expect(screen.getByRole('button', { name: 'Add Selected (1)' })).toBeInTheDocument());
        expect(screen.queryByRole('button', { name: 'Add Selected (3)' })).not.toBeInTheDocument();
    });

    it('Add Favorites is scoped to the selected subcategory', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems());
        renderTab({ category: 'tools' });

        fireEvent.change(await screen.findByLabelText('Subcategory'), { target: { value: 'Great Runes' } });

        const addFavorites = await screen.findByRole('button', { name: 'Add Favorites (1)' });
        fireEvent.click(addFavorites);
        fireEvent.click(await screen.findByRole('button', { name: /^Add$/ }));

        await waitFor(() =>
            expect(mocks.AddItemsToCharacter).toHaveBeenCalledWith(
                0, [0x13], 0, 0, 0, 0, 1, 1));
    });

    it('Owned counts the whole category, then narrows to the subcategory', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems());
        mocks.GetCharacter.mockResolvedValue({
            inventory: [ownedVm(0x11, 5), ownedVm(0x13, 1)], // one Consumable + one Great Rune owned
            storage: [], clearCount: 0,
        });
        renderTab({ category: 'tools' });

        const badge = (await screen.findByText('Owned:')).parentElement!;
        await waitFor(() => expect(badge).toHaveTextContent('Owned:2'));

        fireEvent.change(await screen.findByLabelText('Subcategory'), { target: { value: 'Consumables' } });
        await waitFor(() => expect(badge).toHaveTextContent('Owned:1'));
    });

    it('text search does not change the Owned count', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems());
        mocks.GetCharacter.mockResolvedValue({
            inventory: [ownedVm(0x11, 5), ownedVm(0x13, 1)],
            storage: [], clearCount: 0,
        });
        renderTab({ category: 'tools' });

        const badge = (await screen.findByText('Owned:')).parentElement!;
        await waitFor(() => expect(badge).toHaveTextContent('Owned:2'));

        fireEvent.change(screen.getByPlaceholderText('Search by name or ID...'), { target: { value: 'zzz-no-match' } });
        // Search only affects the visible table, never the owned scope.
        expect(badge).toHaveTextContent('Owned:2');
    });

    // --- Max Up column + numeric sorting (tasks 5) ---

    it('shows a Max Up column for upgradeable items even with no add-modal upgrade selected', async () => {
        // DEFAULT_ADD_SETTINGS has every upgrade/infuse at 0 — the old "Upgrade"
        // preview column would have stayed hidden; "Max Up" is item identity so
        // it must appear regardless.
        mocks.GetItemList.mockResolvedValue([makeWeapon(0x201, 'Longsword', 25)]);
        renderTab({ category: 'melee_armaments' });
        expect(await screen.findByText(/^Max Up/)).toBeInTheDocument();
    });

    it('hides the Max Up column when no visible item is upgradeable', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems()); // all maxUpgrade 0
        renderTab({ category: 'tools' });
        await screen.findByLabelText('Subcategory'); // wait for load
        expect(screen.queryByText(/^Max Up/)).not.toBeInTheDocument();
    });

    it('sorts the Item Database by owned Inventory count', async () => {
        mocks.GetItemList.mockResolvedValue(makeToolItems());
        mocks.GetCharacter.mockResolvedValue({
            inventory: [ownedVm(0x11, 5), ownedVm(0x12, 1), ownedVm(0x13, 10)],
            storage: [], clearCount: 0,
        });
        renderTab({ category: 'tools' });

        // Click the Inventory header (table view) → ascending owned-inventory sort.
        fireEvent.click(await screen.findByText(/^Inventory/));
        // Read the resulting order from the grid (virtualized table rows don't
        // render under jsdom; the grid shares the same sorted list).
        fireEvent.click(screen.getByTitle('Grid view'));

        await waitFor(() => expect(screen.getByText('Rowa Fruit')).toBeInTheDocument());
        const order = screen.getAllByText(/Cured Meat|Rowa Fruit|Golden Rune/).map(e => e.textContent);
        expect(order).toEqual(['Rowa Fruit', 'Cured Meat', 'Golden Rune']); // 1, 5, 10
    });

    // NOTE: the legacy "Repair & Retry" prompt that fired on a duplicate
    // acquisition-index error from AddItemsToCharacter was removed together
    // with the AddItems tolerance flow (see fix(save-integrity)). Duplicate
    // acquisition indices are now caught BEFORE editing by the load-time
    // InventoryIntegrityModal in App.tsx — covered by its own component
    // tests — so this tab no longer routes the integrity-issue error to a
    // retry-style prompt. handleAdd's catch path drops it into the generic
    // error modal instead.
});
