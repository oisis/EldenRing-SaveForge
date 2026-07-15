import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

// The default (no-injected) apply path must route through ApplyRepairsLoaded.
const applyRepairsLoadedMock = vi.hoisted(() => vi.fn());
vi.mock('../../wailsjs/go/main/App', () => ({
    ApplyRepairsLoaded: (charIdx: number, targets: unknown, stop: boolean) => applyRepairsLoadedMock(charIdx, targets, stop),
    ScanRepairIssuesLoaded: vi.fn(),
}));

import { InventoryIssuesModal } from './InventoryIssuesModal';
import type { IssueKey, RepairApplyReport, RepairIssue, RepairIssueReport, ValidationCoverage } from '../lib/repairIssues';

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

    it('routes the default apply path through ApplyRepairsLoaded with the char index', async () => {
        applyRepairsLoadedMock.mockResolvedValue(okReport());
        render(
            <InventoryIssuesModal reports={report([issue({})])} charIndex={3} onClose={vi.fn()} onSaved={vi.fn()} />,
        );

        fireEvent.click(screen.getByRole('button', { name: /Repair selected/i }));

        await waitFor(() => expect(applyRepairsLoadedMock).toHaveBeenCalledTimes(1));
        expect(applyRepairsLoadedMock.mock.calls[0][0]).toBe(3);
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

    it('labels quantity_above_max readably', () => {
        renderModal([
            issue({
                issueID: 'issue-quantity_above_max-0',
                key: { code: 'quantity_above_max' },
                description: 'item quantity exceeds max',
            }),
        ]);
        expect(screen.getByText('Quantity above max')).toBeInTheDocument();
    });

    // A positive-cap over-quantity: clamp is the default mutating action, checked
    // and submitted as clamp_quantity.
    function clampIssue(): RepairIssue {
        return issue({
            issueID: 'issue-quantity_above_max-0',
            key: { code: 'quantity_above_max' },
            description: 'item quantity 1500 exceeds inventory_common max 999',
            actions: [
                { id: 'clamp_quantity', label: 'Clamp quantity to allowed maximum' },
                { id: 'leave_unchanged', label: 'Leave unchanged' },
            ],
            defaultAction: 'clamp_quantity',
        });
    }

    it('selects clamp_quantity by default and submits it', async () => {
        const applyRepairs = vi.fn(async () => okReport());
        renderModal([clampIssue()], applyRepairs);

        const checkbox = screen.getByLabelText(/^Select /) as HTMLInputElement;
        expect(checkbox.checked).toBe(true);
        expect((screen.getByLabelText('Repair action') as HTMLSelectElement).value).toBe('clamp_quantity');

        fireEvent.click(screen.getByRole('button', { name: /Repair selected/i }));
        await waitFor(() => expect(applyRepairs).toHaveBeenCalledTimes(1));
        const calls = applyRepairs.mock.calls as unknown as Array<[Array<{ selectedAction: string }>, boolean]>;
        expect(calls[0][0][0].selectedAction).toBe('clamp_quantity');
    });

    // A zero-cap container violation: separate label, defaults to leave_unchanged
    // (not selected), and remove_record becomes selectable on demand.
    function notAllowedIssue(): RepairIssue {
        return issue({
            issueID: 'issue-item_not_allowed_in_container-0',
            key: { code: 'item_not_allowed_in_container', scope: 'storage_common' },
            description: 'item is not permitted in storage_common',
            actions: [
                { id: 'remove_record', label: 'Remove record' },
                { id: 'leave_unchanged', label: 'Leave unchanged' },
            ],
            defaultAction: 'leave_unchanged',
        });
    }

    it('labels item_not_allowed_in_container and defaults to leave_unchanged unselected', () => {
        renderModal([notAllowedIssue()]);
        expect(screen.getByText('Item not allowed in container')).toBeInTheDocument();

        const checkbox = screen.getByLabelText(/^Select /) as HTMLInputElement;
        expect(checkbox.checked).toBe(false);
        expect(checkbox.disabled).toBe(true);
        expect((screen.getByLabelText('Repair action') as HTMLSelectElement).value).toBe('leave_unchanged');
    });

    it('marks item_not_allowed_in_container as a mutating repair once remove_record is chosen', () => {
        renderModal([notAllowedIssue()]);
        fireEvent.change(screen.getByLabelText('Repair action'), { target: { value: 'remove_record' } });

        const checkbox = screen.getByLabelText(/^Select /) as HTMLInputElement;
        expect(checkbox.checked).toBe(true);
        expect(checkbox.disabled).toBe(false);
        expect(screen.getByRole('button', { name: /Repair selected \(1\)/i })).toBeInTheDocument();
    });
});

// ---- Task 7.9c: shared duplicate-repair action ------------------------------

function duplicatePhysicalIssue(handle = 0x80000102): RepairIssue {
    return issue({
        issueID: 'issue-duplicate_physical_gaitem_handle-2',
        key: { domain: 'gaitem', code: 'duplicate_physical_gaitem_handle', scope: 'gaitems', row: 2, handle },
        actions: [{ id: 'no_action', label: 'No action' }],
        defaultAction: 'no_action',
        record: undefined,
    });
}

describe('InventoryIssuesModal duplicate GaItem action', () => {
    it('renders the dedicated CTA for a duplicate_physical_gaitem_handle issue and passes slot + handle', () => {
        const onResolveDuplicateGaItem = vi.fn();
        render(
            <InventoryIssuesModal
                reports={report([duplicatePhysicalIssue(0x80000102)])}
                charIndex={0}
                onClose={vi.fn()}
                onSaved={vi.fn()}
                onResolveDuplicateGaItem={onResolveDuplicateGaItem}
            />,
        );
        fireEvent.click(screen.getByRole('button', { name: /Resolve duplicate GaItem/ }));
        expect(onResolveDuplicateGaItem).toHaveBeenCalledWith(0, 0x80000102);
        // The issue stays report-only: no mutating repair is selected for it.
        expect(screen.getByRole('button', { name: /Repair selected \(0\)/i })).toBeInTheDocument();
    });

    it('does not render the CTA for other issue codes', () => {
        const onResolveDuplicateGaItem = vi.fn();
        render(
            <InventoryIssuesModal
                reports={report([issue({})])}
                charIndex={0}
                onClose={vi.fn()}
                onSaved={vi.fn()}
                onResolveDuplicateGaItem={onResolveDuplicateGaItem}
            />,
        );
        expect(screen.queryByRole('button', { name: /Resolve duplicate GaItem/ })).not.toBeInTheDocument();
    });

    it('does not render the CTA when no callback is provided', () => {
        render(
            <InventoryIssuesModal
                reports={report([duplicatePhysicalIssue()])}
                charIndex={0}
                onClose={vi.fn()}
                onSaved={vi.fn()}
            />,
        );
        expect(screen.queryByRole('button', { name: /Resolve duplicate GaItem/ })).not.toBeInTheDocument();
    });
});

// ---- Prompt 15: validation coverage -----------------------------------------

function coverage(overrides: Partial<ValidationCoverage> = {}): ValidationCoverage {
    return {
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
        ...overrides,
    };
}

function reportWith(slotIndex: number, charName: string, issues: RepairIssue[], cov: ValidationCoverage): RepairIssueReport {
    return { slotIndex, charName, hasIssues: issues.length > 0, issues, coverage: cov };
}

function renderReports(reports: RepairIssueReport[]) {
    render(
        <InventoryIssuesModal
            reports={reports}
            charIndex={0}
            onClose={vi.fn()}
            onSaved={vi.fn()}
            applyRepairs={vi.fn(async () => okReport())}
        />,
    );
}

describe('InventoryIssuesModal validation coverage', () => {
    it('renders all coverage totals for a single report', () => {
        renderReports([reportWith(0, 'Tarnished', [], coverage({
            totalPhysical: 100, resolved: 95, knownDB: 90, technicalPlaceholder: 5,
            unknown: 0, resolutionChecksApplied: 100, structuralChecksApplied: 100, categoryChecksApplied: 90,
        }))]);

        expect(screen.getByTestId('coverage-totalPhysical')).toHaveTextContent('100');
        expect(screen.getByTestId('coverage-resolved')).toHaveTextContent('95');
        expect(screen.getByTestId('coverage-knownDB')).toHaveTextContent('90');
        expect(screen.getByTestId('coverage-technicalPlaceholder')).toHaveTextContent('5');
        expect(screen.getByTestId('coverage-unknown')).toHaveTextContent('0');
        expect(screen.getByTestId('coverage-resolutionChecksApplied')).toHaveTextContent('100');
        expect(screen.getByTestId('coverage-structuralChecksApplied')).toHaveTextContent('100');
        expect(screen.getByTestId('coverage-categoryChecksApplied')).toHaveTextContent('90');
    });

    it('shows the no-issues banner and coverage for a clean report', () => {
        renderReports([reportWith(0, 'Tarnished', [], coverage({ totalPhysical: 42, resolved: 42, knownDB: 42 }))]);
        expect(screen.getByText('No repair issues found')).toBeInTheDocument();
        expect(screen.getByTestId('validation-coverage')).toBeInTheDocument();
        expect(screen.getByTestId('coverage-totalPhysical')).toHaveTextContent('42');
    });

    it('renders a warning when there are unresolved records', () => {
        renderReports([reportWith(0, 'T', [], coverage({ totalPhysical: 3, unknown: 2 }))]);
        expect(screen.getByTestId('coverage-warning')).toHaveTextContent(/could not be resolved/i);
    });

    it('does not render the warning when nothing is unresolved', () => {
        renderReports([reportWith(0, 'T', [], coverage({ totalPhysical: 3, unknown: 0 }))]);
        expect(screen.queryByTestId('coverage-warning')).not.toBeInTheDocument();
    });

    it('describes technical placeholders as resolved but excluded from category validation', () => {
        renderReports([reportWith(0, 'T', [], coverage({ technicalPlaceholder: 2 }))]);
        expect(screen.getByText(/Technical placeholders are resolved but intentionally excluded from category validation/i)).toBeInTheDocument();
        expect(screen.getByText(/Category checks apply only to Known DB records/i)).toBeInTheDocument();
    });

    it('renders perCategory and unknownByReason maps in stable sorted order', () => {
        renderReports([reportWith(0, 'T', [], coverage({
            perCategory: { talismans: 2, bolstering_materials: 1 },
            unknownByReason: { unknown_handle_type: 1, missing_db_entry: 2 },
        }))]);

        fireEvent.click(screen.getByRole('button', { name: /Breakdown/i }));
        const breakdown = screen.getByTestId('coverage-breakdown');
        const items = within(breakdown).getAllByRole('listitem').map(li => li.textContent);
        // per-category (sorted) then unknown-by-reason (sorted)
        expect(items).toEqual([
            'bolstering_materials1',
            'talismans2',
            'missing_db_entry2',
            'unknown_handle_type1',
        ]);
    });

    it('shows empty states in the breakdown when maps are empty', () => {
        renderReports([reportWith(0, 'T', [], coverage({ totalPhysical: 1, knownDB: 1 }))]);
        fireEvent.click(screen.getByRole('button', { name: /Breakdown/i }));
        expect(screen.getByText('No categorised records')).toBeInTheDocument();
        expect(screen.getByText('No unresolved records')).toBeInTheDocument();
    });

    it('keeps coverage associated with the correct slot for multiple reports', () => {
        renderReports([
            reportWith(0, 'Alpha', [], coverage({ totalPhysical: 10 })),
            reportWith(1, 'Beta', [], coverage({ totalPhysical: 20 })),
        ]);

        expect(screen.getByText(/Slot 1 - Alpha/)).toBeInTheDocument();
        expect(screen.getByText(/Slot 2 - Beta/)).toBeInTheDocument();

        const sections = screen.getAllByTestId('validation-coverage');
        expect(sections).toHaveLength(2);
        expect(within(sections[0]).getByTestId('coverage-totalPhysical')).toHaveTextContent('10');
        expect(within(sections[1]).getByTestId('coverage-totalPhysical')).toHaveTextContent('20');
    });
});
