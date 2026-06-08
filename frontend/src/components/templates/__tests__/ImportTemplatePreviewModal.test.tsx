import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
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

    it('does NOT render an Apply button when onApply prop is omitted (Phase C preview-only)', () => {
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} />);
        expect(screen.queryByTestId('import-preview-apply')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-apply-note')).not.toBeInTheDocument();
    });

    it('renders Apply button only when onApply is provided (Phase D)', () => {
        const onApply = () => {};
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} onApply={onApply} />);
        const btn = screen.getByTestId('import-preview-apply');
        expect(btn).toBeInTheDocument();
        expect(btn).toBeEnabled();
        // Apply note must explain that save remains required.
        expect(screen.getByTestId('import-preview-apply-note')).toHaveTextContent(/Save changes/i);
    });

    it('disables Apply button when report is not OK even with onApply provided', () => {
        const report = makeReport({
            ok: false,
            errors: [
                templates.ImportPreviewIssue.createFrom({
                    severity: 'error',
                    code: 'capacity_exceeded',
                    message: 'too full',
                }),
            ],
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} onApply={() => {}} />);
        expect(screen.getByTestId('import-preview-apply')).toBeDisabled();
    });

    it('Apply button calls onApply when clicked', async () => {
        const { fireEvent } = await import('@testing-library/react');
        let called = false;
        const onApply = () => { called = true; };
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} onApply={onApply} />);
        fireEvent.click(screen.getByTestId('import-preview-apply'));
        expect(called).toBe(true);
    });

    it('Apply button shows "Applying…" and is disabled while applying=true', () => {
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} onApply={() => {}} applying={true} />);
        const btn = screen.getByTestId('import-preview-apply');
        expect(btn).toBeDisabled();
        expect(btn).toHaveTextContent(/Applying/);
    });
});

describe('ImportTemplatePreviewModal — Phase 2B Save to Library', () => {
    it('does NOT render Save to Library when onSaveToLibrary is omitted', () => {
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} />);
        expect(screen.queryByTestId('import-preview-save-to-library')).not.toBeInTheDocument();
    });

    it('renders Save to Library when onSaveToLibrary is provided and report.ok=true', () => {
        render(
            <ImportTemplatePreviewModal
                report={makeReport()}
                onClose={() => {}}
                onSaveToLibrary={() => {}}
            />,
        );
        const btn = screen.getByTestId('import-preview-save-to-library');
        expect(btn).toBeInTheDocument();
        expect(btn).toBeEnabled();
    });

    it('renders Save to Library visible but disabled when report.ok=false', async () => {
        const report = makeReport({
            ok: false,
            errors: [
                templates.ImportPreviewIssue.createFrom({
                    severity: 'error',
                    code: 'structure_invalid',
                    message: 'multi-document YAML payloads are not supported',
                }),
            ],
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onSaveToLibrary={() => {}}
            />,
        );
        const btn = screen.getByTestId('import-preview-save-to-library');
        expect(btn).toBeInTheDocument();
        expect(btn).toBeDisabled();
    });

    it('Save to Library button calls onSaveToLibrary when clicked', async () => {
        const { fireEvent } = await import('@testing-library/react');
        const onSaveToLibrary = vi.fn();
        render(
            <ImportTemplatePreviewModal
                report={makeReport()}
                onClose={() => {}}
                onSaveToLibrary={onSaveToLibrary}
            />,
        );
        fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        expect(onSaveToLibrary).toHaveBeenCalledTimes(1);
    });

    it('shows "Saving…" and is disabled while savingToLibrary=true', () => {
        render(
            <ImportTemplatePreviewModal
                report={makeReport()}
                onClose={() => {}}
                onSaveToLibrary={() => {}}
                savingToLibrary={true}
            />,
        );
        const btn = screen.getByTestId('import-preview-save-to-library');
        expect(btn).toBeDisabled();
        expect(btn).toHaveTextContent(/Saving/);
    });

    it('onApply and onSaveToLibrary are independent — neither presence affects the other', () => {
        // Both provided: both buttons render.
        const { rerender } = render(
            <ImportTemplatePreviewModal
                report={makeReport()}
                onClose={() => {}}
                onApply={() => {}}
                onSaveToLibrary={() => {}}
            />,
        );
        expect(screen.getByTestId('import-preview-apply')).toBeInTheDocument();
        expect(screen.getByTestId('import-preview-save-to-library')).toBeInTheDocument();

        // Only onApply: legacy SortOrderTab behavior unchanged.
        rerender(
            <ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} onApply={() => {}} />,
        );
        expect(screen.getByTestId('import-preview-apply')).toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-save-to-library')).not.toBeInTheDocument();

        // Only onSaveToLibrary: global YAML import flow — no Apply ever.
        rerender(
            <ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} onSaveToLibrary={() => {}} />,
        );
        expect(screen.queryByTestId('import-preview-apply')).not.toBeInTheDocument();
        expect(screen.getByTestId('import-preview-save-to-library')).toBeInTheDocument();
    });
});

describe('ImportTemplatePreviewModal — Phase 3D.1 schema v2 metadata', () => {
    it('renders the Schema v2 version row when summary.version >= 2', () => {
        const report = makeReport({
            summary: templates.ImportPreviewSummary.createFrom({
                inventoryItems: 0,
                storageItems: 0,
                weapons: 0,
                armor: 0,
                talismans: 0,
                stackables: 0,
                aowAssignments: 0,
                version: 2,
                selectedSections: ['profile'],
            }),
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const versionRow = screen.getByTestId('import-preview-schema-version');
        expect(versionRow).toHaveTextContent(/v2/i);
        expect(screen.getByTestId('import-preview-v2-meta')).toBeInTheDocument();
    });

    it('renders selectedSections only when version >= 2', () => {
        const report = makeReport({
            summary: templates.ImportPreviewSummary.createFrom({
                inventoryItems: 0,
                storageItems: 0,
                weapons: 0,
                armor: 0,
                talismans: 0,
                stackables: 0,
                aowAssignments: 0,
                version: 2,
                selectedSections: ['profile', 'stats'],
            }),
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const sectionRow = screen.getByTestId('import-preview-selected-sections');
        expect(sectionRow).toHaveTextContent(/profile/);
        expect(sectionRow).toHaveTextContent(/stats/);
    });

    it('renders profileFieldsPresent and statFieldsPresent when non-empty', () => {
        const report = makeReport({
            summary: templates.ImportPreviewSummary.createFrom({
                inventoryItems: 0,
                storageItems: 0,
                weapons: 0,
                armor: 0,
                talismans: 0,
                stackables: 0,
                aowAssignments: 0,
                version: 2,
                selectedSections: ['profile', 'stats'],
                profileFieldsPresent: ['level', 'runes', 'name'],
                statFieldsPresent: ['vigor', 'mind', 'endurance'],
            }),
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const profileRow = screen.getByTestId('import-preview-profile-fields');
        expect(profileRow).toHaveTextContent(/level/);
        expect(profileRow).toHaveTextContent(/runes/);
        expect(profileRow).toHaveTextContent(/name/);
        const statRow = screen.getByTestId('import-preview-stat-fields');
        expect(statRow).toHaveTextContent(/vigor/);
        expect(statRow).toHaveTextContent(/mind/);
        expect(statRow).toHaveTextContent(/endurance/);
    });

    it('keeps v1 previews quiet — no Schema v1 row and no selected-sections row', () => {
        const report = makeReport({
            summary: templates.ImportPreviewSummary.createFrom({
                inventoryItems: 12,
                storageItems: 3,
                weapons: 4,
                armor: 1,
                talismans: 0,
                stackables: 0,
                aowAssignments: 0,
                version: 1,
                selectedSections: ['inventory.workspace'],
            }),
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        expect(screen.queryByTestId('import-preview-v2-meta')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-schema-version')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-selected-sections')).not.toBeInTheDocument();
        // Existing summary content remains visible.
        const summary = screen.getByTestId('import-preview-summary');
        expect(summary).toHaveTextContent(/Inventory items:.*12/);
        expect(summary).toHaveTextContent(/Storage items:.*3/);
    });

    it('omits the v2 meta block entirely when summary.version is undefined and no profile/stat fields', () => {
        render(<ImportTemplatePreviewModal report={makeReport()} onClose={() => {}} />);
        expect(screen.queryByTestId('import-preview-v2-meta')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-schema-version')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-profile-fields')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-stat-fields')).not.toBeInTheDocument();
    });
});

describe('ImportTemplatePreviewModal — Phase 5D.2 direct v2 Apply', () => {
    function v2Summary(overrides: Partial<templates.ImportPreviewSummary> = {}) {
        return templates.ImportPreviewSummary.createFrom({
            inventoryItems: 0,
            storageItems: 0,
            weapons: 0,
            armor: 0,
            talismans: 0,
            stackables: 0,
            aowAssignments: 0,
            version: 2,
            selectedSections: ['profile', 'stats'],
            profileFieldsPresent: ['level'],
            statFieldsPresent: ['vigor'],
            ...overrides,
        });
    }

    it('v1 preview never renders Apply to character even when onApplyV2 is provided', () => {
        const report = makeReport({
            summary: templates.ImportPreviewSummary.createFrom({
                inventoryItems: 4,
                storageItems: 0,
                weapons: 1,
                armor: 0,
                talismans: 0,
                stackables: 0,
                aowAssignments: 0,
                version: 1,
                selectedSections: ['inventory.workspace'],
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        expect(screen.queryByTestId('import-preview-apply-v2')).not.toBeInTheDocument();
    });

    it('v2 preview with supported sections, save loaded, charIndex set: Apply enabled', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={1}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeInTheDocument();
        expect(btn).toBeEnabled();
        expect(btn).toHaveTextContent(/Apply to character/);
    });

    it('v2 preview with unsupported section keeps Apply visible but disabled with explanatory title', () => {
        // Phase 7b.1 added `equipment` and Phase 7d.4 added `spells` to the
        // supported set; pick a section name that is still genuinely outside
        // the supported list to keep the negative case meaningful.
        const report = makeReport({
            summary: v2Summary({ selectedSections: ['profile', 'inventory.unknown'] }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={1}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
        expect(btn.getAttribute('title')).toMatch(/Unsupported v2 sections/i);
    });

    it('v2 preview without saveLoaded keeps Apply disabled with "Load a save" title', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
        expect(btn.getAttribute('title')).toMatch(/Load a save/i);
    });

    it('v2 preview without charIndex keeps Apply disabled with "Select a character" title', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
        expect(btn.getAttribute('title')).toMatch(/Select a character/i);
    });

    it('v2 preview with report.ok=false keeps Apply disabled', () => {
        const report = makeReport({
            ok: false,
            errors: [
                templates.ImportPreviewIssue.createFrom({
                    severity: 'error',
                    code: 'structure_invalid',
                    message: 'bad payload',
                }),
            ],
            summary: v2Summary(),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
        expect(btn.getAttribute('title')).toMatch(/fix errors/i);
    });

    it('clicking enabled Apply calls onApplyV2 exactly once', async () => {
        const { fireEvent } = await import('@testing-library/react');
        const onApplyV2 = vi.fn();
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={onApplyV2}
                charIndex={2}
                saveLoaded
            />,
        );
        fireEvent.click(screen.getByTestId('import-preview-apply-v2'));
        expect(onApplyV2).toHaveBeenCalledTimes(1);
    });

    it('Apply shows "Applying…" and is disabled while applyingV2=true', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                applyingV2
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
        expect(btn).toHaveTextContent(/Applying/);
    });

    it('Save to Library button continues to work independently of onApplyV2', async () => {
        const { fireEvent } = await import('@testing-library/react');
        const onSaveToLibrary = vi.fn();
        const onApplyV2 = vi.fn();
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onSaveToLibrary={onSaveToLibrary}
                onApplyV2={onApplyV2}
                charIndex={0}
                saveLoaded
            />,
        );
        expect(screen.getByTestId('import-preview-save-to-library')).toBeEnabled();
        expect(screen.getByTestId('import-preview-apply-v2')).toBeEnabled();
        fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        expect(onSaveToLibrary).toHaveBeenCalledTimes(1);
        expect(onApplyV2).not.toHaveBeenCalled();
    });

    it('v2 preview without onApplyV2 callback does not render the button', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        expect(screen.queryByTestId('import-preview-apply-v2')).not.toBeInTheDocument();
    });
});

describe('ImportTemplatePreviewModal — Phase 6 Apply with overrides', () => {
    function v2Summary(overrides: Partial<templates.ImportPreviewSummary> = {}) {
        return templates.ImportPreviewSummary.createFrom({
            inventoryItems: 0,
            storageItems: 0,
            weapons: 0,
            armor: 0,
            talismans: 0,
            stackables: 0,
            aowAssignments: 0,
            version: 2,
            selectedSections: ['profile', 'stats'],
            profileFieldsPresent: ['level'],
            statFieldsPresent: ['vigor'],
            ...overrides,
        });
    }

    it('v1 preview never renders the overrides button even when onApplyV2WithOverrides is provided', () => {
        const report = makeReport({
            summary: templates.ImportPreviewSummary.createFrom({
                inventoryItems: 1,
                storageItems: 0,
                weapons: 0,
                armor: 0,
                talismans: 0,
                stackables: 0,
                aowAssignments: 0,
                version: 1,
                selectedSections: ['inventory.workspace'],
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2WithOverrides={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        expect(screen.queryByTestId('import-preview-apply-v2-overrides')).not.toBeInTheDocument();
    });

    it('v2 preview with supported sections + save + char: overrides button enabled', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2WithOverrides={() => {}}
                charIndex={1}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2-overrides');
        expect(btn).toBeInTheDocument();
        expect(btn).toBeEnabled();
        expect(btn).toHaveTextContent(/Apply with overrides/i);
    });

    it('v2 preview without saveLoaded keeps overrides disabled with "Load a save" title', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2WithOverrides={() => {}}
                charIndex={0}
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2-overrides');
        expect(btn).toBeDisabled();
        expect(btn.getAttribute('title')).toMatch(/Load a save/i);
    });

    it('v2 preview with unsupported sections keeps overrides disabled with explanatory title', () => {
        // Phase 7b.1 added `equipment` and Phase 7d.4 added `spells` to the
        // supported set; pick a section name that is still genuinely outside
        // the supported list to keep the negative case meaningful.
        const report = makeReport({
            summary: v2Summary({ selectedSections: ['profile', 'inventory.unknown'] }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2WithOverrides={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2-overrides');
        expect(btn).toBeDisabled();
        expect(btn.getAttribute('title')).toMatch(/Unsupported v2 sections/i);
    });

    it('clicking the enabled overrides button calls onApplyV2WithOverrides exactly once', async () => {
        const { fireEvent } = await import('@testing-library/react');
        const onApplyV2WithOverrides = vi.fn();
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2WithOverrides={onApplyV2WithOverrides}
                charIndex={3}
                saveLoaded
            />,
        );
        fireEvent.click(screen.getByTestId('import-preview-apply-v2-overrides'));
        expect(onApplyV2WithOverrides).toHaveBeenCalledTimes(1);
    });

    it('both Apply to character and Apply with overrides render independently when both callbacks are provided', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                onApplyV2WithOverrides={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        expect(screen.getByTestId('import-preview-apply-v2')).toBeEnabled();
        expect(screen.getByTestId('import-preview-apply-v2-overrides')).toBeEnabled();
    });

    it('v2 preview without onApplyV2WithOverrides callback does not render the overrides button', () => {
        const report = makeReport({ summary: v2Summary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        expect(screen.queryByTestId('import-preview-apply-v2-overrides')).not.toBeInTheDocument();
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

describe('ImportTemplatePreviewModal — Phase 7b.1 equipment section', () => {
    function equipSummary(overrides: Partial<templates.ImportPreviewSummary> = {}) {
        return templates.ImportPreviewSummary.createFrom({
            inventoryItems: 0,
            storageItems: 0,
            weapons: 0,
            armor: 0,
            talismans: 0,
            stackables: 0,
            aowAssignments: 0,
            version: 2,
            selectedSections: ['equipment'],
            equipmentSlotsPresent: ['weaponRightHand1', 'armorHead'],
            ...overrides,
        });
    }

    it('treats equipment as an applyable v2 section', () => {
        const report = makeReport({ summary: equipSummary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeEnabled();
    });

    it('renders the equipment slots row when summary lists slots', () => {
        const report = makeReport({ summary: equipSummary() });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const row = screen.getByTestId('import-preview-equipment-slots');
        expect(row).toHaveTextContent(/weaponRightHand1/);
        expect(row).toHaveTextContent(/armorHead/);
    });

    it('does not render the equipment slots row when none are present', () => {
        const report = makeReport({
            summary: equipSummary({ equipmentSlotsPresent: [] }),
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        expect(screen.queryByTestId('import-preview-equipment-slots')).not.toBeInTheDocument();
    });

    it('renders talisman slot keys in the equipment row (Phase 7c)', () => {
        const report = makeReport({
            summary: equipSummary({
                equipmentSlotsPresent: ['talisman1', 'talisman2', 'talisman4'],
            }),
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const row = screen.getByTestId('import-preview-equipment-slots');
        expect(row).toHaveTextContent(/talisman1/);
        expect(row).toHaveTextContent(/talisman2/);
        expect(row).toHaveTextContent(/talisman4/);
    });

    it('keeps Apply enabled for an equipment-only talisman template (Phase 7c)', () => {
        const report = makeReport({
            summary: equipSummary({
                equipmentSlotsPresent: ['talisman1', 'talisman3'],
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeEnabled();
    });

    it('surface the combo error from backend preview', () => {
        const report = makeReport({
            ok: false,
            errors: [
                templates.ImportPreviewIssue.createFrom({
                    severity: 'error',
                    code: 'equipment_inventory_combo_unsupported',
                    message: 'sections.equipment cannot be applied together with sections.inventory.workspace',
                }),
            ],
            summary: equipSummary({
                selectedSections: ['equipment', 'inventory.workspace'],
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        // The combo error is surfaced as a normal preview error and Apply is
        // disabled because report.ok=false.
        const errBlock = screen.getByTestId('import-preview-errors');
        expect(errBlock).toHaveTextContent(/equipment_inventory_combo_unsupported/);
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
    });
});

describe('ImportTemplatePreviewModal — Phase 7d.4 spells section', () => {
    function spellsSummary(overrides: Partial<templates.ImportPreviewSummary> = {}) {
        return templates.ImportPreviewSummary.createFrom({
            inventoryItems: 0,
            storageItems: 0,
            weapons: 0,
            armor: 0,
            talismans: 0,
            stackables: 0,
            aowAssignments: 0,
            version: 2,
            selectedSections: ['spells'],
            spellSlotsPresent: ['spell1', 'spell2', 'spell3'],
            ...overrides,
        });
    }

    it('treats spells as an applyable v2 section', () => {
        const report = makeReport({ summary: spellsSummary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeEnabled();
    });

    it('renders the spell slots row when summary lists slots', () => {
        const report = makeReport({ summary: spellsSummary() });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        const row = screen.getByTestId('import-preview-spell-slots');
        expect(row).toHaveTextContent(/spell1/);
        expect(row).toHaveTextContent(/spell2/);
        expect(row).toHaveTextContent(/spell3/);
    });

    it('does not render the spell slots row when none are present', () => {
        const report = makeReport({
            summary: spellsSummary({ spellSlotsPresent: [] }),
        });
        render(<ImportTemplatePreviewModal report={report} onClose={() => {}} />);
        expect(screen.queryByTestId('import-preview-spell-slots')).not.toBeInTheDocument();
    });

    it('keeps Apply enabled for a spells-only template combined with equipment', () => {
        const report = makeReport({
            summary: spellsSummary({
                selectedSections: ['equipment', 'spells'],
                equipmentSlotsPresent: ['weaponRightHand1'],
                spellSlotsPresent: ['spell1'],
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeEnabled();
    });

    it('disables Apply when an unsupported section accompanies spells', () => {
        const report = makeReport({
            summary: spellsSummary({
                selectedSections: ['spells', 'inventory.unknown'],
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
        expect(btn).toHaveAttribute(
            'title',
            'Unsupported v2 sections — apply is available only for profile, stats, inventory.workspace, equipment, spells, and items in this phase. Inventory layout / storage layout are export-only.',
        );
    });
});

describe('ImportTemplatePreviewModal — Phase 8C.1 items / inventoryLayout / storageLayout', () => {
    function itemsSummary(overrides: Partial<templates.ImportPreviewSummary> = {}) {
        return templates.ImportPreviewSummary.createFrom({
            inventoryItems: 0,
            storageItems: 0,
            weapons: 0,
            armor: 0,
            talismans: 0,
            stackables: 0,
            aowAssignments: 0,
            version: 2,
            selectedSections: ['profile', 'items', 'inventoryLayout', 'storageLayout'],
            profileFieldsPresent: ['level'],
            itemsEntries: 12,
            inventoryLayoutCount: 8,
            storageLayoutCount: 4,
            ...overrides,
        });
    }

    it('renders items entries and layout counts from the summary', () => {
        const report = makeReport({ summary: itemsSummary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        expect(screen.getByTestId('import-preview-items-entries')).toHaveTextContent('12');
        expect(screen.getByTestId('import-preview-inventory-layout-count')).toHaveTextContent('8');
        expect(screen.getByTestId('import-preview-storage-layout-count')).toHaveTextContent('4');
        // Phase 8D.2 — when items + layout coexist, apply IS supported
        // (the layout sections are dropped by the backend with a warning).
        expect(
            screen.getByTestId('import-preview-items-apply-supported'),
        ).toBeInTheDocument();
        expect(
            screen.queryByTestId('import-preview-items-export-only'),
        ).not.toBeInTheDocument();
    });

    it('Phase 8D.2 — enables Apply for an items + layout template and shows layout-ignored copy', () => {
        const report = makeReport({ summary: itemsSummary() });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).not.toBeDisabled();
        expect(
            screen.getByTestId('import-preview-items-apply-supported').textContent,
        ).toMatch(/layout.*ignored/i);
    });

    it('Phase 8D.2 — items-only template enables Apply and renders the supported copy without a layout-ignored note', () => {
        const report = makeReport({
            summary: itemsSummary({
                selectedSections: ['items'],
                profileFieldsPresent: [],
                itemsEntries: 5,
                inventoryLayoutCount: 0,
                storageLayoutCount: 0,
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).not.toBeDisabled();
        const copy = screen.getByTestId('import-preview-items-apply-supported');
        expect(copy.textContent).toMatch(/Items: apply supported \(add missing only\)\./);
        expect(copy.textContent).not.toMatch(/ignored/i);
    });

    it('Phase 8D.2 — layout-only template disables Apply with the layout-only reason', () => {
        const report = makeReport({
            summary: itemsSummary({
                selectedSections: ['inventoryLayout'],
                profileFieldsPresent: [],
                itemsEntries: 0,
                inventoryLayoutCount: 3,
                storageLayoutCount: 0,
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        const btn = screen.getByTestId('import-preview-apply-v2');
        expect(btn).toBeDisabled();
        expect(btn.getAttribute('title')).toMatch(
            /Inventory layout \/ storage layout are export-only/,
        );
        expect(
            screen.getByTestId('import-preview-items-export-only'),
        ).toBeInTheDocument();
        expect(
            screen.queryByTestId('import-preview-items-apply-supported'),
        ).not.toBeInTheDocument();
    });

    it('hides the items block when no items/layout counts are reported', () => {
        const report = makeReport({
            summary: itemsSummary({
                selectedSections: ['profile'],
                itemsEntries: 0,
                inventoryLayoutCount: 0,
                storageLayoutCount: 0,
            }),
        });
        render(
            <ImportTemplatePreviewModal
                report={report}
                onClose={() => {}}
                onApplyV2={() => {}}
                charIndex={0}
                saveLoaded
            />,
        );
        expect(screen.queryByTestId('import-preview-items-entries')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-inventory-layout-count')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-storage-layout-count')).not.toBeInTheDocument();
        expect(screen.queryByTestId('import-preview-items-export-only')).not.toBeInTheDocument();
    });
});
