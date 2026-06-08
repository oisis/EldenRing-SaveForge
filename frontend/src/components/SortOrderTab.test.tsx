import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { editor } from '../../wailsjs/go/models';

vi.mock('../../wailsjs/go/main/App', () => ({
    StartInventoryEditSession: vi.fn(),
    GetInventoryEditSession: vi.fn(),
    ValidateInventoryWorkspace: vi.fn(),
    MoveInventoryWorkspaceItem: vi.fn(),
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
import { SortOrderTab } from './SortOrderTab';

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

// Phase 8A removed the JSON template export/import/apply surface from
// SortOrderTab. Templates now live exclusively in the global Templates
// shell modal (YAML-only). Tests for the removed Export Template menu,
// Import Template Preview, Apply Template, and the weapon-level
// override panel were dropped together with the code paths they
// exercised. The remaining tests above cover session lifecycle, dirty/
// save UX and confirm modals — all unchanged by Phase 8A.
