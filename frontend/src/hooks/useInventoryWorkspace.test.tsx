import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { editor } from '../../wailsjs/go/models';

// Mock the Wails-generated App module before importing the hook.
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
}));

import * as App from '../../wailsjs/go/main/App';
import { useInventoryWorkspace } from './useInventoryWorkspace';

const mocks = App as unknown as {
    StartInventoryEditSession: ReturnType<typeof vi.fn>;
    ValidateInventoryWorkspace: ReturnType<typeof vi.fn>;
    MoveInventoryWorkspaceItem: ReturnType<typeof vi.fn>;
    TransferInventoryWorkspaceItem: ReturnType<typeof vi.fn>;
    AddInventoryWorkspaceItem: ReturnType<typeof vi.fn>;
    UpdateInventoryWorkspaceWeapon: ReturnType<typeof vi.fn>;
    RemoveInventoryWorkspaceItem: ReturnType<typeof vi.fn>;
    SaveInventoryWorkspaceChanges: ReturnType<typeof vi.fn>;
    DiscardInventoryEditSession: ReturnType<typeof vi.fn>;
};

function makeItem(uid: string, container: 'inventory' | 'storage', position: number, overrides: Partial<editor.EditableItem> = {}): editor.EditableItem {
    return editor.EditableItem.createFrom({
        uid,
        source: 'original',
        container,
        position,
        originalHandle: 0x80800000 + position,
        itemID: 0x000F4240,
        baseItemID: 0x000F4240,
        name: `Item ${position}`,
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
    characterIndex?: number;
    inventory?: editor.EditableItem[];
    storage?: editor.EditableItem[];
    dirty?: boolean;
} = {}): editor.InventoryWorkspaceSnapshot {
    return editor.InventoryWorkspaceSnapshot.createFrom({
        sessionID: opts.sessionID ?? 'ses-test',
        characterIndex: opts.characterIndex ?? 0,
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

beforeEach(() => {
    Object.values(mocks).forEach(m => m.mockReset());
    mocks.DiscardInventoryEditSession.mockResolvedValue(undefined);
});

afterEach(() => {
    vi.clearAllMocks();
});

describe('useInventoryWorkspace', () => {
    it('starts a session and exposes the snapshot', async () => {
        const inv = [makeItem('hnd:0x80800001', 'inventory', 0)];
        const snap = makeSnapshot({ sessionID: 'ses-1', inventory: inv });
        mocks.StartInventoryEditSession.mockResolvedValue(snap);

        const { result } = renderHook(() => useInventoryWorkspace());

        await act(async () => {
            await result.current.start(3);
        });

        expect(mocks.StartInventoryEditSession).toHaveBeenCalledWith(3);
        expect(result.current.sessionID).toBe('ses-1');
        expect(result.current.characterIndex).toBe(0);
        expect(result.current.inventoryItems).toHaveLength(1);
        expect(result.current.dirty).toBe(false);
        expect(result.current.lastError).toBeNull();
    });

    it('records start failures in lastError without throwing', async () => {
        mocks.StartInventoryEditSession.mockRejectedValue(new Error('boom'));

        const { result } = renderHook(() => useInventoryWorkspace());

        await act(async () => {
            const snap = await result.current.start(0);
            expect(snap).toBeNull();
        });

        expect(result.current.lastError).toMatch(/boom/);
        expect(result.current.sessionID).toBe('');
    });

    it('clearError resets lastError', async () => {
        mocks.StartInventoryEditSession.mockRejectedValue(new Error('boom'));

        const { result } = renderHook(() => useInventoryWorkspace());

        await act(async () => {
            await result.current.start(0);
        });
        expect(result.current.lastError).not.toBeNull();

        act(() => result.current.clearError());
        expect(result.current.lastError).toBeNull();
    });

    it('moveItem forwards sessionID and updates snapshot', async () => {
        const startSnap = makeSnapshot({
            sessionID: 'ses-m',
            inventory: [makeItem('hnd:0x80800001', 'inventory', 0)],
        });
        const movedSnap = makeSnapshot({
            sessionID: 'ses-m',
            inventory: [],
            storage: [makeItem('hnd:0x80800001', 'storage', 0)],
            dirty: true,
        });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.MoveInventoryWorkspaceItem.mockResolvedValue(movedSnap);

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        await act(async () => {
            await result.current.moveItem('hnd:0x80800001', 'storage', 0);
        });

        expect(mocks.MoveInventoryWorkspaceItem).toHaveBeenCalledWith('ses-m', 'hnd:0x80800001', 'storage', 0);
        expect(result.current.storageItems).toHaveLength(1);
        expect(result.current.dirty).toBe(true);
    });

    it('refuses to call backend when no session is active', async () => {
        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => {
            await result.current.moveItem('hnd:0x80800001', 'storage', 0);
        });
        expect(mocks.MoveInventoryWorkspaceItem).not.toHaveBeenCalled();
        expect(result.current.lastError).toMatch(/Move failed/);
    });

    it('transferItem forwards target and updates snapshot', async () => {
        const startSnap = makeSnapshot({ sessionID: 'ses-t', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] });
        const afterSnap = makeSnapshot({ sessionID: 'ses-t', storage: [makeItem('hnd:0x80800001', 'storage', 0)], dirty: true });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.TransferInventoryWorkspaceItem.mockResolvedValue(afterSnap);

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        await act(async () => { await result.current.transferItem('hnd:0x80800001', 'storage'); });

        expect(mocks.TransferInventoryWorkspaceItem).toHaveBeenCalledWith('ses-t', 'hnd:0x80800001', 'storage');
        expect(result.current.storageItems).toHaveLength(1);
    });

    it('addItem translates targetPosition=-1 to inventory length (append)', async () => {
        const a = makeItem('hnd:0x80800001', 'inventory', 0);
        const b = makeItem('hnd:0x80800002', 'inventory', 1);
        const startSnap = makeSnapshot({ sessionID: 'ses-a', inventory: [a, b] });
        const added = editor.EditableItem.createFrom({
            ...a, uid: 'new:1', source: 'added', position: 2, originalHandle: 0, name: 'New Dagger',
        });
        const afterSnap = makeSnapshot({ sessionID: 'ses-a', inventory: [a, b, added], dirty: true });

        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.AddInventoryWorkspaceItem.mockResolvedValue(afterSnap);

        const spec = editor.AddItemSpec.createFrom({
            itemID: 0x000F4240, baseItemID: 0x000F4240, quantity: 1, upgrade: 0, infusionName: '', aowItemID: 0,
        });

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        let returned: editor.EditableItem | null = null;
        await act(async () => {
            returned = await result.current.addItem(spec, 'inventory', -1);
        });

        // Backend must NOT see -1 — hook translates to length=2 (append after both items).
        expect(mocks.AddInventoryWorkspaceItem).toHaveBeenCalledWith('ses-a', spec, 'inventory', 2);
        expect(returned).not.toBeNull();
        expect((returned as unknown as editor.EditableItem).uid).toBe('new:1');
        expect(result.current.inventoryItems).toHaveLength(3);
    });

    it('addItem with explicit targetPosition is passed through unchanged', async () => {
        const startSnap = makeSnapshot({
            sessionID: 'ses-a2',
            inventory: [makeItem('hnd:0x80800001', 'inventory', 0), makeItem('hnd:0x80800002', 'inventory', 1)],
        });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.AddInventoryWorkspaceItem.mockResolvedValue(startSnap);

        const spec = editor.AddItemSpec.createFrom({
            itemID: 0x000F4240, baseItemID: 0x000F4240, quantity: 1, upgrade: 0, infusionName: '', aowItemID: 0,
        });

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        await act(async () => { await result.current.addItem(spec, 'inventory', 0); });

        expect(mocks.AddInventoryWorkspaceItem).toHaveBeenCalledWith('ses-a2', spec, 'inventory', 0);
    });

    it('addItem uses storage length for storage target', async () => {
        const startSnap = makeSnapshot({
            sessionID: 'ses-a3',
            storage: [makeItem('hnd:0x80800010', 'storage', 0), makeItem('hnd:0x80800011', 'storage', 1), makeItem('hnd:0x80800012', 'storage', 2)],
        });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.AddInventoryWorkspaceItem.mockResolvedValue(startSnap);

        const spec = editor.AddItemSpec.createFrom({ itemID: 1, baseItemID: 1, quantity: 1, upgrade: 0, infusionName: '', aowItemID: 0 });

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        await act(async () => { await result.current.addItem(spec, 'storage', -1); });

        expect(mocks.AddInventoryWorkspaceItem).toHaveBeenCalledWith('ses-a3', spec, 'storage', 3);
    });

    it('addItem returns null and sets lastError when backend fails', async () => {
        mocks.StartInventoryEditSession.mockResolvedValue(makeSnapshot({ sessionID: 'ses-e' }));
        mocks.AddInventoryWorkspaceItem.mockRejectedValue(new Error('cap full'));

        const spec = editor.AddItemSpec.createFrom({ itemID: 1, baseItemID: 1, quantity: 1, upgrade: 0, infusionName: '', aowItemID: 0 });

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        let returned: editor.EditableItem | null = null;
        await act(async () => {
            returned = await result.current.addItem(spec, 'inventory', 5);
        });

        expect(returned).toBeNull();
        expect(result.current.lastError).toMatch(/cap full/);
    });

    it('removeItem calls backend and refreshes snapshot', async () => {
        const startSnap = makeSnapshot({
            sessionID: 'ses-r',
            inventory: [makeItem('hnd:0x80800001', 'inventory', 0), makeItem('hnd:0x80800002', 'inventory', 1)],
        });
        const afterSnap = makeSnapshot({
            sessionID: 'ses-r',
            inventory: [makeItem('hnd:0x80800002', 'inventory', 0)],
            dirty: true,
        });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.RemoveInventoryWorkspaceItem.mockResolvedValue(afterSnap);

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        await act(async () => { await result.current.removeItem('hnd:0x80800001'); });

        expect(mocks.RemoveInventoryWorkspaceItem).toHaveBeenCalledWith('ses-r', 'hnd:0x80800001');
        expect(result.current.inventoryItems).toHaveLength(1);
    });

    it('updateWeapon forwards patch and resolves to mutated item', async () => {
        const inv = [makeItem('hnd:0x80800001', 'inventory', 0)];
        const startSnap = makeSnapshot({ sessionID: 'ses-w', inventory: inv });
        const updated = editor.EditableItem.createFrom({
            ...inv[0], currentUpgrade: 25, hasPendingWeaponPatch: true,
        });
        const afterSnap = makeSnapshot({ sessionID: 'ses-w', inventory: [updated], dirty: true });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.UpdateInventoryWorkspaceWeapon.mockResolvedValue(afterSnap);

        const patch = editor.WeaponPatch.createFrom({
            setUpgrade: true, upgrade: 25, setInfusionName: false, infusionName: '',
            setAoWItemID: false, aowItemID: 0, clearAoW: false,
        });

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        let returned: editor.EditableItem | null = null;
        await act(async () => {
            returned = await result.current.updateWeapon('hnd:0x80800001', patch);
        });

        expect(mocks.UpdateInventoryWorkspaceWeapon).toHaveBeenCalledWith('ses-w', 'hnd:0x80800001', patch);
        expect(returned).not.toBeNull();
        expect((returned as unknown as editor.EditableItem).currentUpgrade).toBe(25);
    });

    it('updateWeapon returns null on backend rejection', async () => {
        mocks.StartInventoryEditSession.mockResolvedValue(makeSnapshot({ sessionID: 'ses-w2' }));
        mocks.UpdateInventoryWorkspaceWeapon.mockRejectedValue(new Error('incompatible AoW'));

        const patch = editor.WeaponPatch.createFrom({
            setUpgrade: false, upgrade: 0, setInfusionName: false, infusionName: '',
            setAoWItemID: true, aowItemID: 0x80002710, clearAoW: false,
        });

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        let returned: editor.EditableItem | null = null;
        await act(async () => {
            returned = await result.current.updateWeapon('hnd:0x80800001', patch);
        });

        expect(returned).toBeNull();
        expect(result.current.lastError).toMatch(/incompatible AoW/);
    });

    it('save calls SaveInventoryWorkspaceChanges and clears dirty via returned snapshot', async () => {
        const startSnap = makeSnapshot({ sessionID: 'ses-s', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: true });
        const savedSnap = makeSnapshot({ sessionID: 'ses-s', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: false });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.SaveInventoryWorkspaceChanges.mockResolvedValue(savedSnap);

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        expect(result.current.dirty).toBe(true);
        await act(async () => { await result.current.save(); });

        expect(mocks.SaveInventoryWorkspaceChanges).toHaveBeenCalledWith('ses-s');
        expect(result.current.dirty).toBe(false);
        expect(result.current.saving).toBe(false);
    });

    it('save surfaces backend errors and leaves snapshot intact', async () => {
        const startSnap = makeSnapshot({ sessionID: 'ses-s', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: true });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.SaveInventoryWorkspaceChanges.mockRejectedValue(new Error('validation failed'));

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        await act(async () => { await result.current.save(); });

        expect(result.current.lastError).toMatch(/validation failed/);
        expect(result.current.dirty).toBe(true);
        expect(result.current.saving).toBe(false);
    });

    it('discard deletes session and restarts a fresh one', async () => {
        const first = makeSnapshot({ sessionID: 'ses-d', characterIndex: 2, inventory: [makeItem('hnd:0x80800001', 'inventory', 0)], dirty: true });
        const second = makeSnapshot({ sessionID: 'ses-d2', characterIndex: 2, inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] });
        mocks.StartInventoryEditSession
            .mockResolvedValueOnce(first)
            .mockResolvedValueOnce(second);

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(2); });
        await act(async () => { await result.current.discard(); });

        expect(mocks.DiscardInventoryEditSession).toHaveBeenCalledWith('ses-d');
        expect(mocks.StartInventoryEditSession).toHaveBeenCalledTimes(2);
        expect(mocks.StartInventoryEditSession.mock.calls[1]).toEqual([2]);
        expect(result.current.sessionID).toBe('ses-d2');
        expect(result.current.dirty).toBe(false);
    });

    it('refresh re-runs validation and merges into snapshot', async () => {
        const startSnap = makeSnapshot({ sessionID: 'ses-v', inventory: [makeItem('hnd:0x80800001', 'inventory', 0)] });
        mocks.StartInventoryEditSession.mockResolvedValue(startSnap);
        mocks.ValidateInventoryWorkspace.mockResolvedValue(editor.WorkspaceValidationReport.createFrom({
            ok: false,
            errors: [{ severity: 'error', code: 'TEST', message: 'fail' }],
            warnings: [],
            inventoryItemCount: 1,
            storageItemCount: 0,
            unsupportedInventoryCount: 0,
            unsupportedStorageCount: 0,
            duplicateUIDs: [],
            duplicateHandles: [],
        }));

        const { result } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });
        await act(async () => { await result.current.refresh(); });

        expect(mocks.ValidateInventoryWorkspace).toHaveBeenCalledWith('ses-v');
        expect(result.current.validation?.ok).toBe(false);
        expect(result.current.validation?.errors).toHaveLength(1);
    });

    it('cleans up the session on unmount', async () => {
        const snap = makeSnapshot({ sessionID: 'ses-cleanup' });
        mocks.StartInventoryEditSession.mockResolvedValue(snap);

        const { result, unmount } = renderHook(() => useInventoryWorkspace());
        await act(async () => { await result.current.start(0); });

        unmount();
        await waitFor(() => {
            expect(mocks.DiscardInventoryEditSession).toHaveBeenCalledWith('ses-cleanup');
        });
    });
});
