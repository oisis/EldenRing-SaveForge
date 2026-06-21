// @vitest-environment jsdom
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { editor, main } from '../../wailsjs/go/models';

// vi.hoisted runs before module imports so the shared mock fns are
// available inside vi.mock's hoisted factory. Each test reassigns the
// per-call behavior via mockResolvedValue / mockImplementation.
const mocks = vi.hoisted(() => ({
    GetInfuseTypes: vi.fn(),
    GetItemList: vi.fn(),
    GetCharacter: vi.fn(),
    GetAoWAvailability: vi.fn(),
}));

vi.mock('../../wailsjs/go/main/App', () => ({
    GetInfuseTypes: mocks.GetInfuseTypes,
    GetItemList: mocks.GetItemList,
    GetCharacter: mocks.GetCharacter,
    GetAoWAvailability: mocks.GetAoWAvailability,
}));

beforeEach(() => {
    mocks.GetInfuseTypes.mockResolvedValue([]);
    mocks.GetItemList.mockResolvedValue([]);
    mocks.GetCharacter.mockResolvedValue({ inventory: [], storage: [] });
    mocks.GetAoWAvailability.mockResolvedValue([]);
});

afterEach(() => {
    vi.clearAllMocks();
});

import { WeaponEditModal } from './WeaponEditModal';

function makeOrderItem(overrides: Partial<main.InventoryOrderItem> = {}): main.InventoryOrderItem {
    return main.InventoryOrderItem.createFrom({
        handle: 0x80800001,
        itemId: 0x000F4240,
        name: 'Dagger',
        category: 'melee_armaments',
        acquisitionIndex: 1000,
        currentUpgrade: 0,
        maxUpgrade: 25,
        ...overrides,
    });
}

function makeWorkspaceItem(overrides: Partial<editor.EditableItem> = {}): editor.EditableItem {
    return editor.EditableItem.createFrom({
        uid: 'hnd:0x80800001',
        source: 'original',
        container: 'inventory',
        position: 0,
        originalHandle: 0x80800001,
        itemID: 0x000F4240,
        baseItemID: 0x000F4240,
        name: 'Dagger',
        category: 'melee_armaments',
        quantity: 1,
        acquisitionIndex: 1000,
        currentUpgrade: 0,
        maxUpgrade: 25,
        hasGaItem: true,
        isWeapon: true,
        isArmor: false,
        isTalisman: false,
        ...overrides,
    });
}

async function renderModal(props: Parameters<typeof WeaponEditModal>[0]) {
    let result!: ReturnType<typeof render>;
    await act(async () => {
        result = render(<WeaponEditModal {...props} />);
    });
    return result;
}

describe('WeaponEditModal (workspace mode)', () => {
    it('renders the pending assign banner when workspaceItem has pendingAoWItemID', async () => {
        const workspace = { sessionID: 'ses-1', updateWeapon: vi.fn() };
        const item = makeOrderItem();
        const workspaceItem = makeWorkspaceItem({
            pendingAoWItemID: 0x80002710,
            pendingAoWName: "Lion's Claw",
        });

        await renderModal({
            charIndex: 0,
            item,
            source: 'inventory',
            onClose: () => {},
            workspace,
            workspaceItem,
        });

        expect(screen.getByText(/Pending save:\s*Lion's Claw/)).toBeInTheDocument();
    });

    it('renders the pending clear banner when workspaceItem has pendingAoWClear', async () => {
        const workspace = { sessionID: 'ses-2', updateWeapon: vi.fn() };
        const item = makeOrderItem();
        const workspaceItem = makeWorkspaceItem({ pendingAoWClear: true });

        await renderModal({
            charIndex: 0,
            item,
            source: 'inventory',
            onClose: () => {},
            workspace,
            workspaceItem,
        });

        expect(screen.getByText(/built-in skill will be restored/)).toBeInTheDocument();
    });

    it('does not render the pending banner when no pending edits exist', async () => {
        const workspace = { sessionID: 'ses-3', updateWeapon: vi.fn() };
        await renderModal({
            charIndex: 0,
            item: makeOrderItem(),
            source: 'inventory',
            onClose: () => {},
            workspace,
            workspaceItem: makeWorkspaceItem(),
        });

        expect(screen.queryByText(/Pending save/)).not.toBeInTheDocument();
    });

    it('shows the workspace item name in the header', async () => {
        const workspace = { sessionID: 'ses-header', updateWeapon: vi.fn() };
        await renderModal({
            charIndex: 0,
            item: makeOrderItem({ name: 'Claymore' }),
            source: 'inventory',
            onClose: () => {},
            workspace,
            workspaceItem: makeWorkspaceItem({ name: 'Claymore' }),
        });

        expect(screen.getByText('Claymore')).toBeInTheDocument();
    });

    // Regression: workspace-mode WeaponEditModal must render compatible
    // Ashes of War without depending on GetCharacter. Previously the modal
    // fell back to GetCharacter for wepType/canMountAoW; for newly-added
    // weapons (no save-side handle) or after a Save that re-allocated
    // handles, the lookup missed and the modal filtered every AoW out as
    // unknown-compat. The workspace item now carries wepType/canMountAoW
    // directly, so the modal can resolve compatibility off the workspace
    // snapshot alone.
    it('renders compatible AoWs from workspace metadata without GetCharacter', async () => {
        // Intentionally make GetCharacter return nothing matching — the
        // modal must NOT depend on it in workspace mode.
        mocks.GetCharacter.mockResolvedValue({ inventory: [], storage: [] });
        mocks.GetAoWAvailability.mockResolvedValue([
            {
                itemId: 0x80003070,
                totalCopies: 1,
                availableCopies: 1,
                usedCopies: 0,
                usedByWeaponHandles: [],
                isMissing: false,
                hasSharedHandleConflict: false,
            },
        ]);
        mocks.GetItemList.mockImplementation(async (cat: string) => {
            if (cat !== 'ashes_of_war') return [];
            return [
                {
                    id: 0x80003070,
                    name: 'Sword Dance',
                    iconPath: '',
                    aowCompatBitmask: 1, // bit 0 set ⇒ compatible with wepType=1 (Dagger)
                },
            ];
        });

        const workspace = { sessionID: 'ses-aow', updateWeapon: vi.fn() };
        await renderModal({
            charIndex: 0,
            item: makeOrderItem(),
            source: 'inventory',
            onClose: () => {},
            workspace,
            workspaceItem: makeWorkspaceItem({
                wepType: 1,        // Dagger
                canMountAoW: true, // gemMountType==2
            }),
        });

        // The list should contain the compatible AoW and not show the
        // "No compatible Ashes of War available." fallback.
        await waitFor(() => {
            expect(screen.getByText('Sword Dance')).toBeInTheDocument();
        });
 expect(screen.queryByText(/No compatible Ashes of War available/i))
 .not.toBeInTheDocument();
 });

 it('allows applying compatible AoW when no free copy exists in the save', async () => {
 mocks.GetAoWAvailability.mockResolvedValue([]);
 mocks.GetItemList.mockImplementation(async (cat: string) => {
 if (cat !== 'ashes_of_war') return [];
 return [
 {
 id: 0x80003070,
 name: 'Sword Dance',
 iconPath: '',
 aowCompatBitmask: 1, // bit 0 set ⇒ compatible with wepType=1 (Dagger)
 },
 ];
 });

 const workspace = {
 sessionID: 'ses-aow-missing',
 updateWeapon: vi.fn().mockResolvedValue(makeWorkspaceItem({
 pendingAoWItemID: 0x80003070,
 pendingAoWName: 'Sword Dance',
 })),
 };
 await renderModal({
 charIndex: 0,
 item: makeOrderItem(),
 source: 'inventory',
 onClose: () => {},
 workspace,
 workspaceItem: makeWorkspaceItem({
 wepType: 1,
 canMountAoW: true,
 }),
 });

 await waitFor(() => {
 expect(screen.getByText('Sword Dance')).toBeInTheDocument();
 });
 expect(screen.queryByText(/No compatible Ashes of War available/i))
 .not.toBeInTheDocument();

 fireEvent.click(screen.getByText('Sword Dance'));
 fireEvent.click(screen.getByRole('button', { name: /Apply Ash of War/i }));

 await waitFor(() => {
 expect(workspace.updateWeapon).toHaveBeenCalledTimes(1);
 });
 expect(workspace.updateWeapon).toHaveBeenCalledWith(
 'hnd:0x80800001',
 expect.objectContaining({ aowItemID: 0x80003070 }),
 );
 });
});
