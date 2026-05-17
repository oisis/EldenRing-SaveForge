import { act, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { editor, main } from '../../wailsjs/go/models';

// Mock all Wails App methods the modal touches at mount.
vi.mock('../../wailsjs/go/main/App', () => ({
    GetInfuseTypes: vi.fn().mockResolvedValue([]),
    GetItemList: vi.fn().mockResolvedValue([]),
    GetCharacter: vi.fn().mockResolvedValue({ inventory: [], storage: [] }),
    GetAoWAvailability: vi.fn().mockResolvedValue([]),
    ApplyWeaponUpgradeLevel: vi.fn().mockResolvedValue(undefined),
    ApplyWeaponInfusion: vi.fn().mockResolvedValue(undefined),
    ApplyWeaponAoWStrict: vi.fn().mockResolvedValue(undefined),
}));

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
        await renderModal({
            charIndex: 0,
            item: makeOrderItem({ name: 'Claymore' }),
            source: 'inventory',
            onClose: () => {},
        });

        expect(screen.getByText('Claymore')).toBeInTheDocument();
    });
});
