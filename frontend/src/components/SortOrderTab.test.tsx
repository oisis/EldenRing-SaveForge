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
    ExportBuildTemplateToFile: vi.fn(),
    ExportBuildTemplateJSON: vi.fn(),
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

describe('SortOrderTab — Export Template (Phase B)', () => {
    it('renders Export Template button when session is active', async () => {
        await mount(makeSnapshot({ sessionID: 'ses-exp', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        const btn = await screen.findByRole('button', { name: /Export Template/i });
        expect(btn).toBeEnabled();
    });

    it('opens the dropdown menu when the button is clicked', async () => {
        await mount(makeSnapshot({ sessionID: 'ses-exp', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        const btn = await screen.findByRole('button', { name: /Export Template/i });
        await act(async () => {
            fireEvent.click(btn);
        });
        expect(screen.getByRole('menuitem', { name: /Export Inventory only/i })).toBeInTheDocument();
        expect(screen.getByRole('menuitem', { name: /Export Storage only/i })).toBeInTheDocument();
        expect(screen.getByRole('menuitem', { name: /Export Both/i })).toBeInTheDocument();
        expect(screen.getByRole('menuitem', { name: /Export with options/i })).toBeInTheDocument();
    });

    it('Export Inventory only sends includeInventory=true and includeStorage=false', async () => {
        mocks.ExportBuildTemplateToFile.mockResolvedValue({ path: '/tmp/x.json', warnings: [], skippedItems: 0 });
        await mount(makeSnapshot({ sessionID: 'ses-inv', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        await act(async () => {
            fireEvent.click(await screen.findByRole('button', { name: /Export Template/i }));
        });
        await act(async () => {
            fireEvent.click(screen.getByRole('menuitem', { name: /Export Inventory only/i }));
        });
        await waitFor(() => {
            expect(mocks.ExportBuildTemplateToFile).toHaveBeenCalledTimes(1);
        });
        const [, opts] = mocks.ExportBuildTemplateToFile.mock.calls[0] as [string, { includeInventory: boolean; includeStorage: boolean }];
        expect(opts.includeInventory).toBe(true);
        expect(opts.includeStorage).toBe(false);
    });

    it('Export Storage only sends includeInventory=false and includeStorage=true', async () => {
        mocks.ExportBuildTemplateToFile.mockResolvedValue({ path: '/tmp/x.json', warnings: [], skippedItems: 0 });
        await mount(makeSnapshot({ sessionID: 'ses-sto', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        await act(async () => {
            fireEvent.click(await screen.findByRole('button', { name: /Export Template/i }));
        });
        await act(async () => {
            fireEvent.click(screen.getByRole('menuitem', { name: /Export Storage only/i }));
        });
        await waitFor(() => {
            expect(mocks.ExportBuildTemplateToFile).toHaveBeenCalledTimes(1);
        });
        const [, opts] = mocks.ExportBuildTemplateToFile.mock.calls[0] as [string, { includeInventory: boolean; includeStorage: boolean }];
        expect(opts.includeInventory).toBe(false);
        expect(opts.includeStorage).toBe(true);
    });

    it('Export Both sends includeInventory=true and includeStorage=true', async () => {
        mocks.ExportBuildTemplateToFile.mockResolvedValue({ path: '/tmp/x.json', warnings: [], skippedItems: 0 });
        await mount(makeSnapshot({ sessionID: 'ses-both', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        await act(async () => {
            fireEvent.click(await screen.findByRole('button', { name: /Export Template/i }));
        });
        await act(async () => {
            fireEvent.click(screen.getByRole('menuitem', { name: /Export Both/i }));
        });
        await waitFor(() => {
            expect(mocks.ExportBuildTemplateToFile).toHaveBeenCalledTimes(1);
        });
        const [, opts] = mocks.ExportBuildTemplateToFile.mock.calls[0] as [string, { includeInventory: boolean; includeStorage: boolean }];
        expect(opts.includeInventory).toBe(true);
        expect(opts.includeStorage).toBe(true);
    });

    it('Export with options opens the modal and shows dirty note when workspace is dirty', async () => {
        await mount(makeSnapshot({ sessionID: 'ses-mod', dirty: true, inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        await act(async () => {
            fireEvent.click(await screen.findByRole('button', { name: /Export Template/i }));
        });
        await act(async () => {
            fireEvent.click(screen.getByRole('menuitem', { name: /Export with options/i }));
        });
        expect(screen.getByTestId('export-template-modal')).toBeInTheDocument();
        expect(screen.getByTestId('export-dirty-note')).toBeInTheDocument();
    });

    it('cancelled save dialog (empty path) does not toast success', async () => {
        const toastMod = (await import('../lib/toast')).default as unknown as {
            success: ReturnType<typeof vi.fn>;
            error: ReturnType<typeof vi.fn>;
        };
        mocks.ExportBuildTemplateToFile.mockResolvedValue({ path: '', warnings: [], skippedItems: 0 });
        await mount(makeSnapshot({ sessionID: 'ses-cancel', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] }));
        await act(async () => {
            fireEvent.click(await screen.findByRole('button', { name: /Export Template/i }));
        });
        await act(async () => {
            fireEvent.click(screen.getByRole('menuitem', { name: /Export Both/i }));
        });
        await waitFor(() => {
            expect(mocks.ExportBuildTemplateToFile).toHaveBeenCalledTimes(1);
        });
        // No success toast for cancelled dialog.
        expect(toastMod.success).not.toHaveBeenCalledWith(expect.stringContaining('Build template saved'));
    });
});
