import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { templates } from '../../../../wailsjs/go/models';
import { ImportTemplatePreviewModal, isCancelledPreview } from '../ImportTemplatePreviewModal';

function makeReport(overrides: Partial<templates.ImportPreviewReport> = {}): templates.ImportPreviewReport {
    return templates.ImportPreviewReport.createFrom({
        ok: true,
        errors: [],
        warnings: [],
        summary: {
            inventoryItems: 0,
            storageItems: 0,
            weapons: 0,
            armor: 0,
            talismans: 0,
            stackables: 0,
            aowAssignments: 0,
        },
        ...overrides,
    });
}

describe('ImportTemplatePreviewModal', () => {
    it('renders summary counts from the report', () => {
        const report = makeReport({
            summary: templates.ImportPreviewSummary.createFrom({
                inventoryItems: 5,
                storageItems: 2,
                weapons: 3,
                armor: 1,
                talismans: 1,
                stackables: 0,
                aowAssignments: 2,
            }),
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const summary = screen.getByTestId('import-preview-summary');
        expect(summary).toHaveTextContent(/Inventory items:.*5/);
        expect(summary).toHaveTextContent(/Storage items:.*2/);
        expect(summary).toHaveTextContent(/Weapons:.*3/);
        expect(summary).toHaveTextContent(/AoW assignments:.*2/);
    });

    it('renders the "preview only" disclaimer prominently', () => {
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} />);
        const note = screen.getByTestId('import-preview-disclaimer');
        expect(note).toHaveTextContent(/does not change your workspace or save/i);
    });

    it('renders error rows when errors are present', () => {
        const report = makeReport({
            ok: false,
            errors: [
                templates.ImportPreviewIssue.createFrom({
                    severity: 'error',
                    code: 'unknown_item',
                    message: 'baseItemID 0xDEADBEEF does not resolve',
                    container: 'inventory',
                    position: 0,
                    baseItemID: 0xdeadbeef,
                }),
            ],
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const errors = screen.getAllByTestId('import-preview-error');
        expect(errors).toHaveLength(1);
        expect(errors[0]).toHaveAttribute('data-code', 'unknown_item');
        expect(errors[0]).toHaveTextContent(/0xDEADBEEF/i);
        // Errors section heading mentions blocking copy.
        expect(screen.getByTestId('import-preview-errors')).toHaveTextContent(/must be fixed/i);
    });

    it('renders warning rows when warnings are present and labels them non-blocking', () => {
        const report = makeReport({
            warnings: [
                templates.ImportPreviewIssue.createFrom({
                    severity: 'warning',
                    code: 'name_mismatch_ignored',
                    message: 'template name does not match DB',
                    container: 'inventory',
                    position: 0,
                }),
            ],
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const warnings = screen.getAllByTestId('import-preview-warning');
        expect(warnings).toHaveLength(1);
        expect(warnings[0]).toHaveAttribute('data-code', 'name_mismatch_ignored');
        expect(screen.getByTestId('import-preview-warnings')).toHaveTextContent(/will not block import/i);
    });

    it('shows OK status when report.ok=true', () => {
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} />);
        expect(screen.getByTestId('import-preview-summary')).toHaveTextContent(/OK/);
    });

    it('shows Blocked status when report.ok=false', () => {
        const report = makeReport({
            ok: false,
            errors: [
                templates.ImportPreviewIssue.createFrom({
                    severity: 'error',
                    code: 'schema_invalid',
                    message: 'bad schema',
                }),
            ],
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        expect(screen.getByTestId('import-preview-summary')).toHaveTextContent(/Blocked/);
    });

    it('does NOT render an Apply button (Phase C is preview-only)', () => {
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} />);
        expect(screen.queryByRole('button', { name: /Apply/i })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Import to Workspace/i })).not.toBeInTheDocument();
    });
});

describe('isCancelledPreview', () => {
    it('returns true for the cancelled sentinel report', () => {
        expect(
            isCancelledPreview(
                templates.ImportPreviewReport.createFrom({
                    ok: false,
                    errors: [],
                    warnings: [],
                    summary: {
                        inventoryItems: 0,
                        storageItems: 0,
                        weapons: 0,
                        armor: 0,
                        talismans: 0,
                        stackables: 0,
                        aowAssignments: 0,
                    },
                }),
            ),
        ).toBe(true);
    });

    it('returns false when ok=true (a real OK preview)', () => {
        expect(isCancelledPreview(makeReport())).toBe(false);
    });

    it('returns false when issues are present', () => {
        const r = makeReport({
            ok: false,
            errors: [templates.ImportPreviewIssue.createFrom({ severity: 'error', code: 'x', message: 'y' })],
        });
        expect(isCancelledPreview(r)).toBe(false);
    });

    it('returns false when summary has any items even with ok=false (template parsed but blocked)', () => {
        const r = makeReport({
            ok: false,
            summary: templates.ImportPreviewSummary.createFrom({ inventoryItems: 3 }),
        });
        expect(isCancelledPreview(r)).toBe(false);
    });
});
