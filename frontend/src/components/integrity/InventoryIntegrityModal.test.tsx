import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { main } from '../../../wailsjs/go/models';
import { InventoryIntegrityModal } from './InventoryIntegrityModal';

function makeReport(overrides: Partial<main.SaveInventoryIntegrityReport> = {}): main.SaveInventoryIntegrityReport {
    return main.SaveInventoryIntegrityReport.createFrom({
        clean: false,
        slots: [],
        ...overrides,
    });
}

function knownItem(over: Partial<main.InventoryIntegrityConflictItem> = {}): main.InventoryIntegrityConflictItem {
    return main.InventoryIntegrityConflictItem.createFrom({
        scope: 'inventory_common',
        row: 0,
        handle: 0xB0000334,
        itemId: 0x40000334,
        name: 'Boiled Crab',
        category: 'tools',
        quantity: 5,
        currentUpgrade: 0,
        infusionName: '',
        unknown: false,
        ...over,
    });
}

function unknownItem(over: Partial<main.InventoryIntegrityConflictItem> = {}): main.InventoryIntegrityConflictItem {
    return main.InventoryIntegrityConflictItem.createFrom({
        scope: 'inventory_common',
        row: 1,
        handle: 0xB0FFFFFE,
        itemId: 0x40FFFFFE,
        name: '',
        category: '',
        quantity: 1,
        currentUpgrade: 0,
        infusionName: '',
        unknown: true,
        ...over,
    });
}

function activeSlot(over: Partial<main.SlotInventoryIntegrityReport> = {}): main.SlotInventoryIntegrityReport {
    return main.SlotInventoryIntegrityReport.createFrom({
        slotIndex: 0,
        characterName: 'Tarnished',
        active: true,
        duplicateEntryCount: 1,
        conflictingIndexCount: 1,
        conflicts: [
            main.InventoryIntegrityConflict.createFrom({
                index: 552,
                items: [knownItem(), knownItem({ row: 2, name: 'Clarifying Boluses', handle: 0xB00003C0, itemId: 0x400003C0 })],
            }),
        ],
        ...over,
    });
}

function inactiveSlot(over: Partial<main.SlotInventoryIntegrityReport> = {}): main.SlotInventoryIntegrityReport {
    return main.SlotInventoryIntegrityReport.createFrom({
        slotIndex: 2,
        characterName: '',
        active: false,
        duplicateEntryCount: 2,
        conflictingIndexCount: 1,
        conflicts: [
            main.InventoryIntegrityConflict.createFrom({
                index: 700,
                items: [knownItem({ row: 0 }), knownItem({ row: 1 }), knownItem({ row: 2 })],
            }),
        ],
        ...over,
    });
}

describe('InventoryIntegrityModal', () => {
    it('renders the integrity heading and warning copy', () => {
        const report = makeReport({ slots: [activeSlot()] });
        render(
            <InventoryIntegrityModal
                report={report}
                busy={false}
                onRepair={vi.fn()}
                onCloseSave={vi.fn()}
            />,
        );
        expect(screen.getByText('Inventory integrity issue detected')).toBeInTheDocument();
        expect(screen.getByText(/duplicated inventory acquisition indices/i)).toBeInTheDocument();
        expect(screen.getByText(/Keep a backup/i)).toBeInTheDocument();
    });

    it('shows an active slot summary with the character name and counters', () => {
        const report = makeReport({ slots: [activeSlot({ characterName: 'Hero', duplicateEntryCount: 3, conflictingIndexCount: 2 })] });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={vi.fn()} />,
        );
        expect(screen.getByText('Slot 1 — Hero')).toBeInTheDocument();
        expect(screen.getByText('3 colliding entries across 2 acquisition-order bucket collisions')).toBeInTheDocument();
    });

    it('singularises the counter wording for one entry / one index', () => {
        const report = makeReport({ slots: [activeSlot({ duplicateEntryCount: 1, conflictingIndexCount: 1 })] });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={vi.fn()} />,
        );
        expect(screen.getByText('1 colliding entry across 1 acquisition-order bucket collision')).toBeInTheDocument();
    });

    it('labels an inactive residual slot and shows the residual explanation', () => {
        const report = makeReport({ slots: [inactiveSlot()] });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={vi.fn()} />,
        );
        expect(screen.getByText('Inactive residual slot 3')).toBeInTheDocument();
        expect(
            screen.getByText(/residual data belongs to a character slot not currently shown/i),
        ).toBeInTheDocument();
    });

    it('omits the residual note when every affected slot is active', () => {
        const report = makeReport({ slots: [activeSlot()] });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={vi.fn()} />,
        );
        expect(
            screen.queryByText(/residual data belongs to a character slot not currently shown/i),
        ).not.toBeInTheDocument();
    });

    it('expands Show affected items and groups items per acquisition index', async () => {
        const report = makeReport({ slots: [activeSlot()] });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={vi.fn()} />,
        );
        const toggle = screen.getByRole('button', { name: /Show affected items/i });
        fireEvent.click(toggle);
        expect(await screen.findByText('Acquisition-order bucket 552')).toBeInTheDocument();
        // Both items in the conflict are listed.
        expect(screen.getByText(/Boiled Crab/)).toBeInTheDocument();
        expect(screen.getByText(/Clarifying Boluses/)).toBeInTheDocument();
    });

    it('renders unknown items with the ItemID / Handle hex fallback', () => {
        const report = makeReport({
            slots: [
                activeSlot({
                    conflicts: [
                        main.InventoryIntegrityConflict.createFrom({
                            index: 900,
                            items: [knownItem(), unknownItem()],
                        }),
                    ],
                }),
            ],
        });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={vi.fn()} />,
        );
        fireEvent.click(screen.getByRole('button', { name: /Show affected items/i }));
        expect(screen.getByText(/Unknown item · ItemID 0x40FFFFFE · Handle 0xB0FFFFFE/)).toBeInTheDocument();
    });

    it('renders weapon upgrade and infusion when DTO supplies them', () => {
        const weaponItem = main.InventoryIntegrityConflictItem.createFrom({
            scope: 'inventory_common',
            row: 0,
            handle: 0x80000001,
            itemId: 0x00000165,
            name: 'Longsword',
            category: 'melee_armaments',
            quantity: 1,
            currentUpgrade: 5,
            infusionName: 'Heavy',
            unknown: false,
        });
        const report = makeReport({
            slots: [
                activeSlot({
                    conflicts: [
                        main.InventoryIntegrityConflict.createFrom({
                            index: 1234,
                            items: [weaponItem, knownItem({ row: 1 })],
                        }),
                    ],
                }),
            ],
        });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={vi.fn()} />,
        );
        fireEvent.click(screen.getByRole('button', { name: /Show affected items/i }));
        // The label combines upgrade level + infusion.
        expect(screen.getByText(/Longsword \+5 · melee_armaments · Infusion: Heavy/)).toBeInTheDocument();
    });

    it('Repair button invokes onRepair', () => {
        const onRepair = vi.fn();
        const report = makeReport({ slots: [activeSlot()] });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={onRepair} onCloseSave={vi.fn()} />,
        );
        fireEvent.click(screen.getByRole('button', { name: 'Repair duplicates' }));
        expect(onRepair).toHaveBeenCalledTimes(1);
    });

    it('Close save button invokes onCloseSave', () => {
        const onCloseSave = vi.fn();
        const report = makeReport({ slots: [activeSlot()] });
        render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={onCloseSave} />,
        );
        fireEvent.click(screen.getByRole('button', { name: 'Close save' }));
        expect(onCloseSave).toHaveBeenCalledTimes(1);
    });

    it('busy disables both buttons and surfaces the Repairing… label', () => {
        const onRepair = vi.fn();
        const onCloseSave = vi.fn();
        const report = makeReport({ slots: [activeSlot()] });
        render(
            <InventoryIntegrityModal report={report} busy={true} onRepair={onRepair} onCloseSave={onCloseSave} />,
        );
        const repairBtn = screen.getByRole('button', { name: /Repairing/ });
        const closeBtn = screen.getByRole('button', { name: 'Close save' });
        expect(repairBtn).toBeDisabled();
        expect(closeBtn).toBeDisabled();
        fireEvent.click(repairBtn);
        fireEvent.click(closeBtn);
        expect(onRepair).not.toHaveBeenCalled();
        expect(onCloseSave).not.toHaveBeenCalled();
    });

    it('shows an error message when errorMessage is provided', () => {
        const report = makeReport({ slots: [activeSlot()] });
        render(
            <InventoryIntegrityModal
                report={report}
                busy={false}
                errorMessage="Repair did not resolve all duplicate inventory acquisition indices. Saving remains blocked."
                onRepair={vi.fn()}
                onCloseSave={vi.fn()}
            />,
        );
        expect(
            screen.getByText('Repair did not resolve all duplicate inventory acquisition indices. Saving remains blocked.'),
        ).toBeInTheDocument();
    });

    it('renders multiple affected slots with each summary line', async () => {
        const report = makeReport({ slots: [activeSlot({ slotIndex: 0, characterName: 'A' }), inactiveSlot({ slotIndex: 4 })] });
        const { container } = render(
            <InventoryIntegrityModal report={report} busy={false} onRepair={vi.fn()} onCloseSave={vi.fn()} />,
        );
        expect(within(container).getByText('Slot 1 — A')).toBeInTheDocument();
        expect(within(container).getByText('Inactive residual slot 5')).toBeInTheDocument();
        // Verifies modal does not pre-expand the affected list (sanity).
        await waitFor(() => expect(screen.queryByText('Acquisition-order bucket 552')).not.toBeInTheDocument());
    });
});
