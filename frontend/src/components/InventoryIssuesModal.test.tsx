import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { InventoryIssuesModal } from './InventoryIssuesModal';
import type { IssueKey, RepairApplyReport, RepairIssue, RepairIssueReport } from '../lib/repairIssues';

type IssueOverrides = Partial<Omit<RepairIssue, 'key'>> & { key?: Partial<IssueKey> };

function issue(overrides: IssueOverrides): RepairIssue {
    const { key: keyOverrides, ...rest } = overrides;
    const key: IssueKey = {
        slot: 0,
        domain: 'inventory',
        code: 'quantity_zero',
        scope: 'inventory_common',
        row: 0,
        handle: 0xA00017B6,
        ...keyOverrides,
    };
    return {
        issueID: 'issue-' + key.code + '-' + key.row,
        debugKey: 'slot:0|domain:inventory|code:quantity_zero|scope:inventory_common|row:0',
        fingerprint: 'abc123',
        key,
        description: 'Record has zero quantity.',
        severity: 'error',
        actions: [
            { id: 'remove_record', label: 'Remove record' },
            { id: 'leave_unchanged', label: 'Leave unchanged' },
        ],
        defaultAction: 'remove_record',
        record: {
            scope: 'inventory_common',
            row: 0,
            handle: 0xA00017B6,
            itemId: 0x200017B6,
            name: 'Sacrificial Twig',
            category: 'talismans',
            quantity: 0,
            currentUpgrade: 0,
            infusionName: '',
            unknown: false,
        },
        ...rest,
    };
}

function report(issues: RepairIssue[]): RepairIssueReport[] {
    return [{
        slotIndex: 0,
        charName: 'Tarnished',
        source: 'loaded',
        hasIssues: issues.length > 0,
        issues,
        coverage: {
            totalPhysical: 0,
            resolved: 0,
            knownDB: 0,
            technicalPlaceholder: 0,
            unknown: 0,
            resolutionChecksApplied: 0,
            structuralChecksApplied: 0,
            categoryChecksApplied: 0,
            perCategory: {},
            unknownByReason: {},
        },
    }];
}

function okReport(): RepairApplyReport {
    return {
        applied: 1,
        skipped: 0,
        failed: 0,
        needsUserInput: 0,
        stopped: false,
        results: [{
            issueID: 'issue-quantity_zero-0',
            slotIndex: 0,
            action: 'remove_record',
            outcome: 'applied',
            message: '',
        }],
    };
}

function renderModal(issues: RepairIssue[], applyRepairs = vi.fn(async () => okReport())) {
    render(
        <InventoryIssuesModal
            reports={report(issues)}
            source="loaded"
            charIndex={0}
            onClose={vi.fn()}
            onSaved={vi.fn()}
            applyRepairs={applyRepairs}
        />,
    );
    return { applyRepairs };
}

describe('InventoryIssuesModal', () => {
    it('renders an action dropdown and submits the selected action', async () => {
        const applyRepairs = vi.fn(async () => okReport());
        renderModal([issue({})], applyRepairs);

        fireEvent.change(screen.getByLabelText('Repair action'), { target: { value: 'leave_unchanged' } });
        fireEvent.change(screen.getByLabelText('Repair action'), { target: { value: 'remove_record' } });
        fireEvent.click(screen.getByRole('button', { name: /Repair selected/i }));

        await waitFor(() => expect(applyRepairs).toHaveBeenCalledTimes(1));
        const calls = applyRepairs.mock.calls as unknown as Array<[unknown[], boolean]>;
        const firstTarget = calls[0][0][0];
        expect(firstTarget).toMatchObject({
            issueID: 'issue-quantity_zero-0',
            selectedAction: 'remove_record',
            fingerprint: 'abc123',
        });
    });

    it('expands record details with identifiers and capacity', () => {
        renderModal([
            issue({
                capacity: { resource: 'gaitems', needed: 1, available: 0 },
            }),
        ]);

        fireEvent.click(screen.getByRole('button', { name: 'Details' }));

        expect(screen.getByText('Record has zero quantity.')).toBeInTheDocument();
        expect(screen.getByText(/Capacity: gaitems needs 1, available 0/)).toBeInTheDocument();
        expect(screen.getByText(/Handle: 0xA00017B6/)).toBeInTheDocument();
        expect(screen.getByText(/ItemID: 0x200017B6/)).toBeInTheDocument();
    });

    it('uses default actions for quantity_zero and unknown_item_id', () => {
        const unknown = issue({
            issueID: 'issue-unknown_item_id-1',
            key: { code: 'unknown_item_id', row: 1, handle: 0xB0FFFFFE },
            actions: [
                { id: 'leave_unchanged', label: 'Leave unchanged' },
                { id: 'remove_record', label: 'Remove record' },
            ],
            defaultAction: 'leave_unchanged',
            description: 'Item ID is unknown.',
            record: {
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
            },
        });

        renderModal([issue({}), unknown]);

        const selects = screen.getAllByLabelText('Repair action') as HTMLSelectElement[];
        expect(selects[0].value).toBe('remove_record');
        expect(selects[1].value).toBe('leave_unchanged');
        expect(screen.getByRole('button', { name: /Repair selected \(1\)/i })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Repair all default actions/i })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Defaults selected/i })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Defaults all/i })).not.toBeInTheDocument();
    });

    it('shows weapon and AoW context for current AoW issues', () => {
        renderModal([
            issue({
                issueID: 'issue-current_aow_shared-2',
                key: {
                    domain: 'aow',
                    code: 'current_aow_shared',
                    row: 2,
                    handle: 0x80800001,
                },
                description: 'Longsword currently shares Sword Dance with another weapon.',
                actions: [
                    { id: 'create_copy', label: 'Create separate copy' },
                    { id: 'clear_aow', label: 'Clear Ash of War' },
                ],
                defaultAction: 'create_copy',
                record: {
                    scope: 'inventory_common',
                    row: 2,
                    handle: 0x80800001,
                    itemId: 0x000F4240,
                    name: 'Longsword',
                    category: 'melee_armaments',
                    quantity: 1,
                    currentUpgrade: 5,
                    infusionName: 'Heavy',
                    unknown: false,
                },
            }),
        ]);

        expect(screen.getByText('Longsword +5 · Heavy')).toBeInTheDocument();
        expect(screen.getAllByText(/Ash of War/).length).toBeGreaterThan(0);
        fireEvent.click(screen.getByRole('button', { name: 'Details' }));
        expect(screen.getByText(/Sword Dance/)).toBeInTheDocument();
    });

    it('keeps pick_aow visible but marks it as needing a concrete compatible handle', () => {
        renderModal([
            issue({
                key: { domain: 'aow', code: 'current_aow_missing', handle: 0x80800001 },
                actions: [
                    { id: 'clear_aow', label: 'Clear Ash of War' },
                    { id: 'pick_aow', label: 'Pick replacement Ash of War' },
                ],
                defaultAction: 'clear_aow',
            }),
        ]);

        const card = screen.getByText('Sacrificial Twig').closest('div')?.parentElement?.parentElement as HTMLElement;
        fireEvent.change(within(card).getByLabelText('Repair action'), { target: { value: 'pick_aow' } });

        expect(screen.getByText(/Compatible Ash of War selection needs a concrete AoW handle/i)).toBeInTheDocument();
    });

    it('renders a static action label instead of a dropdown when only one action exists', () => {
        renderModal([
            issue({
                issueID: 'issue-pass-through-0',
                key: { code: 'pass_through_records' },
                actions: [{ id: 'no_action', label: 'No action' }],
                defaultAction: 'no_action',
            }),
        ]);

        expect(screen.getByText('No action')).toBeInTheDocument();
        expect(screen.queryByLabelText('Repair action')).not.toBeInTheDocument();
    });
});
