import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../../../wailsjs/go/main/App', () => ({
    PreviewBuildTemplateV2FromCharacter: vi.fn(),
    SaveBuildTemplateV2FromCharacterToLibrary: vi.fn(),
}));

import * as App from '../../../../wailsjs/go/main/App';
import { templates } from '../../../../wailsjs/go/models';
import { CreateTemplateV2Modal } from '../CreateTemplateV2Modal';

const mocks = App as unknown as Record<string, ReturnType<typeof vi.fn>>;

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

function makeV2Report(selected: string[] = ['profile', 'stats']) {
    return makeReport({
        summary: templates.ImportPreviewSummary.createFrom({
            inventoryItems: 0,
            storageItems: 0,
            weapons: 0,
            armor: 0,
            talismans: 0,
            stackables: 0,
            aowAssignments: 0,
            version: 2,
            selectedSections: selected,
            profileFieldsPresent: ['level'],
            statFieldsPresent: ['vigor'],
        }),
    });
}

beforeEach(() => {
    Object.values(mocks).forEach(m => typeof m?.mockReset === 'function' && m.mockReset());
});

afterEach(() => {
    vi.clearAllMocks();
});

function defaultProps(overrides: Partial<Parameters<typeof CreateTemplateV2Modal>[0]> = {}) {
    return {
        charIndex: 3,
        onClose: vi.fn(),
        onSaved: vi.fn(),
        onError: vi.fn(),
        ...overrides,
    };
}

describe('CreateTemplateV2Modal — form surface', () => {
    it('renders metadata inputs and all profile/stat checkboxes', () => {
        render(<CreateTemplateV2Modal {...defaultProps()} />);
        expect(screen.getByTestId('create-template-v2-modal')).toBeInTheDocument();
        expect(screen.getByTestId('create-template-v2-name')).toBeInTheDocument();
        expect(screen.getByTestId('create-template-v2-description')).toBeInTheDocument();
        expect(screen.getByTestId('create-template-v2-author')).toBeInTheDocument();
        expect(screen.getByTestId('create-template-v2-tags')).toBeInTheDocument();

        const profileKeys = [
            'name',
            'level',
            'runes',
            'soulMemory',
            'class',
            'clearCount',
            'scadutreeBlessing',
            'shadowRealmBlessing',
            'talismanSlots',
        ];
        expect(profileKeys).toHaveLength(9);
        for (const k of profileKeys) {
            expect(screen.getByTestId(`create-template-v2-profile-${k}`)).toBeInTheDocument();
        }
        // Explicit guard: backend selection key is "class", not "className".
        expect(screen.getByTestId('create-template-v2-profile-class')).toBeInTheDocument();
        expect(screen.queryByTestId('create-template-v2-profile-className')).not.toBeInTheDocument();

        const statKeys = [
            'vigor',
            'mind',
            'endurance',
            'strength',
            'dexterity',
            'intelligence',
            'faith',
            'arcane',
        ];
        expect(statKeys).toHaveLength(8);
        for (const k of statKeys) {
            expect(screen.getByTestId(`create-template-v2-stats-${k}`)).toBeInTheDocument();
        }
    });

    it('disables Preview when no fields are selected', () => {
        render(<CreateTemplateV2Modal {...defaultProps()} />);
        const preview = screen.getByTestId('create-template-v2-preview') as HTMLButtonElement;
        expect(preview).toBeDisabled();
    });

    it('Select all profile checks every profile field and enables Preview', () => {
        render(<CreateTemplateV2Modal {...defaultProps()} />);
        fireEvent.click(screen.getByTestId('create-template-v2-profile-select-all'));
        for (const k of [
            'name',
            'level',
            'runes',
            'soulMemory',
            'class',
            'clearCount',
            'scadutreeBlessing',
            'shadowRealmBlessing',
            'talismanSlots',
        ]) {
            const cb = screen.getByTestId(`create-template-v2-profile-${k}`) as HTMLInputElement;
            expect(cb.checked).toBe(true);
        }
        expect((screen.getByTestId('create-template-v2-preview') as HTMLButtonElement).disabled).toBe(false);
    });

    it('Clear profile clears every profile field and disables Preview when stats are empty', () => {
        render(<CreateTemplateV2Modal {...defaultProps()} />);
        fireEvent.click(screen.getByTestId('create-template-v2-profile-select-all'));
        fireEvent.click(screen.getByTestId('create-template-v2-profile-clear'));
        for (const k of [
            'name',
            'level',
            'runes',
            'soulMemory',
            'class',
            'clearCount',
            'scadutreeBlessing',
            'shadowRealmBlessing',
            'talismanSlots',
        ]) {
            const cb = screen.getByTestId(`create-template-v2-profile-${k}`) as HTMLInputElement;
            expect(cb.checked).toBe(false);
        }
        expect((screen.getByTestId('create-template-v2-preview') as HTMLButtonElement).disabled).toBe(true);
    });

    it('Select all stats / Clear stats toggles every stat field', () => {
        render(<CreateTemplateV2Modal {...defaultProps()} />);
        fireEvent.click(screen.getByTestId('create-template-v2-stats-select-all'));
        for (const k of ['vigor', 'mind', 'endurance', 'strength', 'dexterity', 'intelligence', 'faith', 'arcane']) {
            const cb = screen.getByTestId(`create-template-v2-stats-${k}`) as HTMLInputElement;
            expect(cb.checked).toBe(true);
        }
        expect((screen.getByTestId('create-template-v2-preview') as HTMLButtonElement).disabled).toBe(false);
        fireEvent.click(screen.getByTestId('create-template-v2-stats-clear'));
        for (const k of ['vigor', 'mind', 'endurance', 'strength', 'dexterity', 'intelligence', 'faith', 'arcane']) {
            const cb = screen.getByTestId(`create-template-v2-stats-${k}`) as HTMLInputElement;
            expect(cb.checked).toBe(false);
        }
        expect((screen.getByTestId('create-template-v2-preview') as HTMLButtonElement).disabled).toBe(true);
    });
});

describe('CreateTemplateV2Modal — Preview flow', () => {
    it('builds per-field selection JSON and forwards metadata to PreviewBuildTemplateV2FromCharacter', async () => {
        mocks.PreviewBuildTemplateV2FromCharacter.mockResolvedValue({ report: makeV2Report() });
        render(<CreateTemplateV2Modal {...defaultProps({ charIndex: 7 })} />);

        fireEvent.change(screen.getByTestId('create-template-v2-name'), { target: { value: 'RL150' } });
        fireEvent.change(screen.getByTestId('create-template-v2-description'), {
            target: { value: 'PvP build' },
        });
        fireEvent.change(screen.getByTestId('create-template-v2-author'), { target: { value: 'tester' } });
        fireEvent.change(screen.getByTestId('create-template-v2-tags'), {
            target: { value: 'pvp, rl150, , test' },
        });

        fireEvent.click(screen.getByTestId('create-template-v2-profile-level'));
        fireEvent.click(screen.getByTestId('create-template-v2-profile-runes'));
        fireEvent.click(screen.getByTestId('create-template-v2-stats-vigor'));
        fireEvent.click(screen.getByTestId('create-template-v2-stats-mind'));

        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });

        await waitFor(() => expect(mocks.PreviewBuildTemplateV2FromCharacter).toHaveBeenCalledTimes(1));
        const call = mocks.PreviewBuildTemplateV2FromCharacter.mock.calls[0];
        expect(call[0]).toBe(7);
        expect(call[1]).toBe('{"profile":{"level":true,"runes":true},"stats":{"vigor":true,"mind":true}}');
        expect(call[2]).toMatchObject({
            name: 'RL150',
            description: 'PvP build',
            author: 'tester',
            tags: ['pvp', 'rl150', 'test'],
        });
    });

    it('uses the backend "class" selection key (not "className")', async () => {
        mocks.PreviewBuildTemplateV2FromCharacter.mockResolvedValue({ report: makeV2Report(['profile']) });
        render(<CreateTemplateV2Modal {...defaultProps({ charIndex: 1 })} />);

        fireEvent.click(screen.getByTestId('create-template-v2-profile-class'));
        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });

        await waitFor(() => expect(mocks.PreviewBuildTemplateV2FromCharacter).toHaveBeenCalledTimes(1));
        const json = mocks.PreviewBuildTemplateV2FromCharacter.mock.calls[0][1] as string;
        expect(json).toBe('{"profile":{"class":true}}');
        expect(json).not.toContain('className');
    });

    it('omits the stats section from selection JSON when no stat is selected', async () => {
        mocks.PreviewBuildTemplateV2FromCharacter.mockResolvedValue({ report: makeV2Report(['profile']) });
        render(<CreateTemplateV2Modal {...defaultProps()} />);

        fireEvent.click(screen.getByTestId('create-template-v2-profile-level'));
        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });

        await waitFor(() => expect(mocks.PreviewBuildTemplateV2FromCharacter).toHaveBeenCalledTimes(1));
        expect(mocks.PreviewBuildTemplateV2FromCharacter.mock.calls[0][1]).toBe('{"profile":{"level":true}}');
    });

    it('calls onError and restores the Preview button when Preview rejects', async () => {
        const err = new Error('preview boom');
        mocks.PreviewBuildTemplateV2FromCharacter.mockRejectedValue(err);
        const props = defaultProps();
        render(<CreateTemplateV2Modal {...props} />);

        fireEvent.click(screen.getByTestId('create-template-v2-profile-level'));
        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });

        await waitFor(() => expect(props.onError).toHaveBeenCalledWith(err));
        const preview = screen.getByTestId('create-template-v2-preview') as HTMLButtonElement;
        expect(preview.disabled).toBe(false);
        expect(preview.textContent).toMatch(/Preview/);
        expect(preview.textContent).not.toMatch(/Previewing/i);
        // Preview modal should not be mounted.
        expect(screen.queryByTestId('import-preview-modal')).not.toBeInTheDocument();
    });

    it('opens ImportTemplatePreviewModal with the v2 metadata block after a successful preview', async () => {
        mocks.PreviewBuildTemplateV2FromCharacter.mockResolvedValue({ report: makeV2Report() });
        render(<CreateTemplateV2Modal {...defaultProps()} />);

        fireEvent.click(screen.getByTestId('create-template-v2-profile-level'));
        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });

        await waitFor(() => expect(screen.getByTestId('import-preview-modal')).toBeInTheDocument());
        expect(screen.getByTestId('import-preview-v2-meta')).toBeInTheDocument();
        expect(screen.getByTestId('import-preview-schema-version')).toHaveTextContent(/v2/);
    });
});

describe('CreateTemplateV2Modal — Save from preview flow', () => {
    async function previewLevelRunes(props: ReturnType<typeof defaultProps>) {
        mocks.PreviewBuildTemplateV2FromCharacter.mockResolvedValue({ report: makeV2Report() });
        render(<CreateTemplateV2Modal {...props} />);

        fireEvent.change(screen.getByTestId('create-template-v2-name'), { target: { value: 'RL150' } });
        fireEvent.change(screen.getByTestId('create-template-v2-tags'), { target: { value: 'pvp' } });

        fireEvent.click(screen.getByTestId('create-template-v2-profile-level'));
        fireEvent.click(screen.getByTestId('create-template-v2-profile-runes'));

        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });
        await waitFor(() => expect(screen.getByTestId('import-preview-save-to-library')).toBeInTheDocument());
    }

    it('Save to Library forwards the same charIndex, selection JSON, and opts used during preview', async () => {
        const props = defaultProps({ charIndex: 5 });
        mocks.SaveBuildTemplateV2FromCharacterToLibrary.mockResolvedValue(
            templates.LibraryTemplateEntry.createFrom({
                id: 'tpl-new',
                name: 'RL150',
                filename: 'rl150.json',
                createdAt: '',
                updatedAt: '',
                inventoryItems: 0,
                storageItems: 0,
                warnings: 0,
                version: 2,
            }),
        );

        await previewLevelRunes(props);

        await act(async () => {
            fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        });

        await waitFor(() => expect(mocks.SaveBuildTemplateV2FromCharacterToLibrary).toHaveBeenCalledTimes(1));
        const previewCall = mocks.PreviewBuildTemplateV2FromCharacter.mock.calls[0];
        const saveCall = mocks.SaveBuildTemplateV2FromCharacterToLibrary.mock.calls[0];
        expect(saveCall[0]).toBe(previewCall[0]);
        expect(saveCall[1]).toBe(previewCall[1]);
        expect(saveCall[2]).toBe(previewCall[2]);
    });

    it('Save success calls onSaved with the new entry and then onClose', async () => {
        const props = defaultProps();
        const entry = templates.LibraryTemplateEntry.createFrom({
            id: 'tpl-new',
            name: 'RL150',
            filename: 'rl150.json',
            createdAt: '',
            updatedAt: '',
            inventoryItems: 0,
            storageItems: 0,
            warnings: 0,
            version: 2,
        });
        mocks.SaveBuildTemplateV2FromCharacterToLibrary.mockResolvedValue(entry);

        await previewLevelRunes(props);

        await act(async () => {
            fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        });

        await waitFor(() => expect(props.onSaved).toHaveBeenCalledTimes(1));
        expect(props.onSaved).toHaveBeenCalledWith(entry);
        expect(props.onClose).toHaveBeenCalledTimes(1);
    });

    it('Save failure calls onError and leaves the preview overlay open', async () => {
        const props = defaultProps();
        const err = new Error('save boom');
        mocks.SaveBuildTemplateV2FromCharacterToLibrary.mockRejectedValue(err);

        await previewLevelRunes(props);

        await act(async () => {
            fireEvent.click(screen.getByTestId('import-preview-save-to-library'));
        });

        await waitFor(() => expect(props.onError).toHaveBeenCalledWith(err));
        expect(props.onSaved).not.toHaveBeenCalled();
        expect(props.onClose).not.toHaveBeenCalled();
        expect(screen.getByTestId('import-preview-modal')).toBeInTheDocument();
    });
});

describe('CreateTemplateV2Modal — containers (Phase 8C.1)', () => {
    it('renders items, inventory layout, and storage layout checkboxes with the export-only badge', () => {
        render(<CreateTemplateV2Modal {...defaultProps()} />);
        expect(screen.getByTestId('create-template-v2-items')).toBeInTheDocument();
        expect(screen.getByTestId('create-template-v2-inventory-layout')).toBeInTheDocument();
        expect(screen.getByTestId('create-template-v2-storage-layout')).toBeInTheDocument();
        expect(screen.getByTestId('create-template-v2-containers-export-only')).toBeInTheDocument();
    });

    it('keeps layout checkboxes disabled until Items is selected', () => {
        render(<CreateTemplateV2Modal {...defaultProps()} />);
        const inv = screen.getByTestId('create-template-v2-inventory-layout') as HTMLInputElement;
        const sto = screen.getByTestId('create-template-v2-storage-layout') as HTMLInputElement;
        expect(inv.disabled).toBe(true);
        expect(sto.disabled).toBe(true);
        fireEvent.click(screen.getByTestId('create-template-v2-items'));
        expect((screen.getByTestId('create-template-v2-inventory-layout') as HTMLInputElement).disabled).toBe(false);
        expect((screen.getByTestId('create-template-v2-storage-layout') as HTMLInputElement).disabled).toBe(false);
    });

    it('unchecking Items clears layout selections and re-disables the layout checkboxes', () => {
        render(<CreateTemplateV2Modal {...defaultProps()} />);
        fireEvent.click(screen.getByTestId('create-template-v2-items'));
        fireEvent.click(screen.getByTestId('create-template-v2-inventory-layout'));
        fireEvent.click(screen.getByTestId('create-template-v2-storage-layout'));
        fireEvent.click(screen.getByTestId('create-template-v2-items'));
        const inv = screen.getByTestId('create-template-v2-inventory-layout') as HTMLInputElement;
        const sto = screen.getByTestId('create-template-v2-storage-layout') as HTMLInputElement;
        expect(inv.checked).toBe(false);
        expect(sto.checked).toBe(false);
        expect(inv.disabled).toBe(true);
        expect(sto.disabled).toBe(true);
    });

    it('Items alone enables Preview and sends items=true in selection JSON', async () => {
        mocks.PreviewBuildTemplateV2FromCharacter.mockResolvedValue({ report: makeV2Report(['items']) });
        render(<CreateTemplateV2Modal {...defaultProps({ charIndex: 4 })} />);

        fireEvent.click(screen.getByTestId('create-template-v2-items'));
        expect((screen.getByTestId('create-template-v2-preview') as HTMLButtonElement).disabled).toBe(false);

        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });

        await waitFor(() => expect(mocks.PreviewBuildTemplateV2FromCharacter).toHaveBeenCalledTimes(1));
        const json = mocks.PreviewBuildTemplateV2FromCharacter.mock.calls[0][1] as string;
        expect(JSON.parse(json)).toEqual({ items: true });
    });

    it('Items + Inventory layout sends both selectors; storage stays off', async () => {
        mocks.PreviewBuildTemplateV2FromCharacter.mockResolvedValue({
            report: makeV2Report(['items', 'inventoryLayout']),
        });
        render(<CreateTemplateV2Modal {...defaultProps({ charIndex: 2 })} />);

        fireEvent.click(screen.getByTestId('create-template-v2-items'));
        fireEvent.click(screen.getByTestId('create-template-v2-inventory-layout'));

        await act(async () => {
            fireEvent.click(screen.getByTestId('create-template-v2-preview'));
        });

        await waitFor(() => expect(mocks.PreviewBuildTemplateV2FromCharacter).toHaveBeenCalledTimes(1));
        const json = mocks.PreviewBuildTemplateV2FromCharacter.mock.calls[0][1] as string;
        expect(JSON.parse(json)).toEqual({ items: true, inventoryLayout: true });
    });
});
