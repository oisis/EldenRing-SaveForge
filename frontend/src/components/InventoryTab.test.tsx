import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Equipment tab: subcategory filtering, Owned scoping and numeric sorting.
// Virtualized table rows don't render under jsdom, so order assertions read
// from the grid view (which shares the same sorted list).

vi.mock('../../wailsjs/go/main/App', () => ({
    GetCharacter: vi.fn(),
    SaveCharacter: vi.fn(),
    RemoveItemsFromCharacter: vi.fn(),
    GetItemDetail: vi.fn(),
}));

vi.mock('../lib/toast', () => {
    const fn = vi.fn() as unknown as Record<string, unknown> & ((...a: unknown[]) => void);
    fn.success = vi.fn();
    fn.error = vi.fn();
    return { default: fn };
});

vi.mock('../state/favorites', () => ({
    useFavorites: () => ({ isFav: () => false, toggle: vi.fn() }),
}));

// jsdom has no layout, so the real virtualizer renders zero rows — the editable
// quantity inputs never mount. Render every row so the Save flow is reachable.
vi.mock('@tanstack/react-virtual', () => ({
    useVirtualizer: (opts: { count: number }) => ({
        getVirtualItems: () => Array.from({ length: opts.count }, (_, index) => ({ index, start: 0, size: 40, key: index })),
        getTotalSize: () => opts.count * 40,
        measureElement: () => {},
    }),
}));

import * as App from '../../wailsjs/go/main/App';
import { InventoryTab } from './InventoryTab';

const mocks = App as unknown as Record<string, ReturnType<typeof vi.fn>>;

// Stackable owned rows in the melee_armaments category, split across two
// subgroups (Straight Sword x2, Axe x1).
function owned(id: number, name: string, subGroup: string, quantity = 1) {
    return {
        id, baseId: id, name, category: 'Item',
        subCategory: 'melee_armaments', subGroup,
        maxInventory: 99, maxStorage: 600, maxUpgrade: 0, currentUpgrade: 0,
        quantity, handle: id, iconPath: '', flags: [] as string[], readOnly: false,
    };
}

function tabElement(overrides: Partial<Parameters<typeof InventoryTab>[0]> = {}) {
    return (
        <InventoryTab
            charIndex={0}
            inventoryVersion={0}
            columnVisibility={{ id: true, category: true }}
            showFlaggedItems={true}
            category="melee_armaments"
            setCategory={vi.fn()}
            {...overrides}
        />
    );
}

beforeEach(() => {
    mocks.GetCharacter.mockResolvedValue({
        inventory: [
            owned(0x11, 'Longsword', 'Straight Sword', 5),
            owned(0x12, 'Broadsword', 'Straight Sword', 1),
            owned(0x13, 'Battle Axe', 'Axe', 10),
        ],
        storage: [],
    });
});

afterEach(() => vi.clearAllMocks());

describe('InventoryTab (Equipment)', () => {
    it('resets the subcategory filter to All when the category changes', async () => {
        const { rerender } = render(tabElement({ category: 'melee_armaments' }));
        const sub = await screen.findByLabelText('Subcategory') as HTMLSelectElement;
        await waitFor(() => expect(sub.options.length).toBeGreaterThan(1));
        fireEvent.change(sub, { target: { value: 'Axe' } });
        expect(sub.value).toBe('Axe');

        rerender(tabElement({ category: 'head' }));
        await waitFor(() =>
            expect((screen.getByLabelText('Subcategory') as HTMLSelectElement).value).toBe('all'));
    });

    it('Owned counts the whole category, then narrows to the subcategory', async () => {
        render(tabElement({ category: 'melee_armaments' }));
        const badge = (await screen.findByText('Owned:')).parentElement!;
        await waitFor(() => expect(badge).toHaveTextContent('Owned:3'));

        fireEvent.change(await screen.findByLabelText('Subcategory'), { target: { value: 'Axe' } });
        await waitFor(() => expect(badge).toHaveTextContent('Owned:1'));
    });

    it('text search does not change the Owned count', async () => {
        render(tabElement({ category: 'melee_armaments' }));
        const badge = (await screen.findByText('Owned:')).parentElement!;
        await waitFor(() => expect(badge).toHaveTextContent('Owned:3'));

        fireEvent.change(screen.getByPlaceholderText('Search owned items...'), { target: { value: 'zzz-no-match' } });
        expect(badge).toHaveTextContent('Owned:3');
    });

    it('fires onMutate after a successful SaveCharacter so App can bump inventoryVersion', async () => {
        mocks.SaveCharacter.mockResolvedValue(undefined);
        const onMutate = vi.fn();
        render(tabElement({ category: 'melee_armaments', onMutate }));

        // Edit a quantity so the Save Changes button appears, then save.
        const qtyInput = (await screen.findAllByRole('spinbutton'))[0];
        fireEvent.change(qtyInput, { target: { value: '7' } });
        fireEvent.click(await screen.findByText('Save Changes'));

        await waitFor(() => expect(mocks.SaveCharacter).toHaveBeenCalled());
        await waitFor(() => expect(onMutate).toHaveBeenCalled());
    });

    it('sorts by owned Inventory quantity', async () => {
        render(tabElement({ category: 'melee_armaments' }));
        fireEvent.click(await screen.findByText(/^Inventory/)); // table header → ascending
        fireEvent.click(screen.getByTitle('Grid view'));

        await waitFor(() => expect(screen.getByText('Broadsword')).toBeInTheDocument());
        const order = screen.getAllByText(/Longsword|Broadsword|Battle Axe/).map(e => e.textContent);
        expect(order).toEqual(['Broadsword', 'Longsword', 'Battle Axe']); // 1, 5, 10
    });
});
