import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// jsdom can run with an opaque origin, which leaves localStorage undefined.
// SuperMerchantTab reads the safety profile during render.
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

vi.mock('../../wailsjs/go/main/App', () => ({
    GetShopMerchants: vi.fn(),
    GetShopStock: vi.fn(),
    ApplyShopChanges: vi.fn(),
    GetItemList: vi.fn(),
}));

vi.mock('../lib/toast', () => ({
    default: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }),
}));

import { ApplyShopChanges, GetItemList, GetShopMerchants, GetShopStock } from '../../wailsjs/go/main/App';
import { SuperMerchantTab } from './SuperMerchantTab';

const merchants = [{ name: 'Merchant Kale', rowCount: 2, editableRows: 1 }];

const stock = [
    {
        rowId: 100500, itemId: 0x40000000 + 111, itemName: 'Crimson Flask', iconPath: 'items/goods/flask.png',
        category: 'tools', flags: [], value: 1000, sellQuantity: 2, editable: true,
    },
    {
        rowId: 100501, itemId: 0x10000000 + 900100, itemName: 'Locked Armor', iconPath: '',
        category: 'chest', flags: [], value: 500, sellQuantity: -1, editable: false, lockReason: 'material-priced row',
    },
];

const itemList = [
    { id: 0x40000000 + 110, name: 'Rune Arc', iconPath: 'items/goods/arc.png', flags: [] },
    { id: 0x40000000 + 900, name: 'Cut Consumable', iconPath: '', flags: ['ban_risk'] },
];

beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    (GetShopMerchants as ReturnType<typeof vi.fn>).mockResolvedValue(merchants);
    (GetShopStock as ReturnType<typeof vi.fn>).mockResolvedValue(stock);
    (ApplyShopChanges as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (GetItemList as ReturnType<typeof vi.fn>).mockResolvedValue(itemList);
});

describe('SuperMerchantTab', () => {
    it('edits a row, applies it, and can discard an unsaved draft', async () => {
        render(<SuperMerchantTab platform="PC" />);

        expect(await screen.findByText(/online warning/i)).toBeTruthy();
        await waitFor(() => expect(screen.getByTestId('shop-row-100500')).toBeTruthy());

        // A material-priced row is read-only.
        const lockedPrice = screen.getByLabelText('Price for row 100501') as HTMLInputElement;
        expect(lockedPrice.disabled).toBe(true);

        const price = screen.getByLabelText('Price for row 100500') as HTMLInputElement;
        fireEvent.change(price, { target: { value: '4242' } });
        const stockInput = screen.getByLabelText('Stock for row 100500') as HTMLInputElement;
        fireEvent.change(stockInput, { target: { value: '9' } });

        expect(screen.getByTestId('super-merchant-dirty').textContent).toContain('1');

        fireEvent.click(screen.getByText('Apply'));
        await waitFor(() => expect(ApplyShopChanges).toHaveBeenCalledTimes(1));
        expect((ApplyShopChanges as ReturnType<typeof vi.fn>).mock.calls[0][0]).toEqual([
            { rowId: 100500, itemId: 0x40000000 + 111, value: 4242, sellQuantity: 9 },
        ]);

        // Discard drops a fresh draft without calling the backend.
        fireEvent.change(screen.getByLabelText('Price for row 100500'), { target: { value: '7' } });
        expect(screen.getByTestId('super-merchant-dirty')).toBeTruthy();
        fireEvent.click(screen.getByText('Discard Changes'));
        await waitFor(() => expect(screen.queryByTestId('super-merchant-dirty')).toBeNull());
        expect(ApplyShopChanges).toHaveBeenCalledTimes(1);
    });

    it('hides risky items from the item picker unless the chaos profile is active', async () => {
        render(<SuperMerchantTab platform="PC" />);
        await waitFor(() => expect(screen.getByTestId('shop-row-100500')).toBeTruthy());

        fireEvent.click(screen.getAllByText('Change item')[0]);
        await waitFor(() => expect(screen.getByText('Rune Arc')).toBeTruthy());
        expect(screen.queryByText('Cut Consumable')).toBeNull();
        expect(GetItemList).toHaveBeenCalledWith('all');
    });

    it('renders the backend rejection instead of an editor on an unsupported save', async () => {
        (GetShopMerchants as ReturnType<typeof vi.fn>).mockRejectedValue(
            new Error('unsupported regulation: Super Merchant currently supports PC saves with Regulation 1.17 only'),
        );

        render(<SuperMerchantTab platform="PS4" />);

        const notice = await screen.findByTestId('super-merchant-unsupported');
        expect(notice.textContent).toContain('unsupported regulation');
        expect(screen.queryByText('Apply')).toBeNull();
    });
});
