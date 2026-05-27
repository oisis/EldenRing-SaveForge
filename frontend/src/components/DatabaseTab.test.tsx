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
    GetCharacter: vi.fn(),
    RepairDuplicateInventoryIndices: vi.fn(),
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
import { DatabaseTab } from './DatabaseTab';

const mocks = App as unknown as Record<string, ReturnType<typeof vi.fn>>;

const BAN_ITEM_ID = 0x2000;

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

const DEFAULT_ADD_SETTINGS: AddSettings = {
    upgrade25: 0, upgrade10: 0, infuseOffset: 0, upgradeAsh: 0,
    talismansHighestOnly: false, includeAshenCapital: false,
};

function renderTab(overrides: Partial<Parameters<typeof DatabaseTab>[0]> = {}) {
    return render(
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
        />,
    );
}

beforeEach(() => {
    localStorage.clear();
    mocks.GetInfuseTypes.mockResolvedValue([]);
    mocks.GetCharacter.mockResolvedValue({ inventory: [], storage: [], clearCount: 0 });
    mocks.GetItemList.mockResolvedValue([makeBanRiskItem()]);
    mocks.GetItemListChunk.mockResolvedValue([]);
    mocks.AddItemsToCharacter.mockResolvedValue({
        added: 1, requested: 1, trimmed: [], capHit: '',
        freeInv: 0, freeStore: 0, neededInv: 0, neededStore: 0,
    });
    mocks.RepairDuplicateInventoryIndices.mockResolvedValue({ changed: 0, changes: [] });
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
});
