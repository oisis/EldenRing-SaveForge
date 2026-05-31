import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import {
    ApplyOverridesModal,
    ApplyOverridesPanel,
    applyOverridesToCanonical,
} from '../ApplyOverridesPanel';

const baseTemplate = {
    schema: 'saveforge.build-template',
    version: 2,
    selection: {
        profile: { level: true, name: true },
        stats: { vigor: true, faith: true },
    },
    sections: {
        profile: { name: 'Tarnished', level: 50, class: 'Vagabond' },
        stats: { vigor: 25, faith: 18 },
    },
};

function canonicalJSON(overrides?: Partial<typeof baseTemplate>) {
    return JSON.stringify({ ...baseTemplate, ...(overrides ?? {}) });
}

describe('ApplyOverridesPanel — rendering', () => {
    it('renders editable profile + stats inputs seeded from the canonical JSON', () => {
        render(
            <ApplyOverridesPanel
                canonicalJSON={canonicalJSON()}
                onMutatedChange={() => {}}
            />,
        );
        expect(screen.getByTestId('apply-overrides-profile-input-level')).toHaveValue('50');
        expect(screen.getByTestId('apply-overrides-profile-input-name')).toHaveValue('Tarnished');
        expect(screen.getByTestId('apply-overrides-stats-input-vigor')).toHaveValue('25');
        expect(screen.getByTestId('apply-overrides-stats-input-faith')).toHaveValue('18');
    });

    it('renders profile.class as read-only when present', () => {
        render(
            <ApplyOverridesPanel
                canonicalJSON={canonicalJSON()}
                onMutatedChange={() => {}}
            />,
        );
        const row = screen.getByTestId('apply-overrides-profile-class-readonly');
        expect(row).toHaveTextContent(/Vagabond/);
        expect(row).toHaveTextContent(/Skipped on apply/i);
    });

    it('does not render profile.class row when the field is absent from the template', () => {
        const tpl = {
            schema: 'saveforge.build-template',
            version: 2,
            selection: { profile: { level: true } },
            sections: { profile: { level: 10 } },
        };
        render(
            <ApplyOverridesPanel
                canonicalJSON={JSON.stringify(tpl)}
                onMutatedChange={() => {}}
            />,
        );
        expect(screen.queryByTestId('apply-overrides-profile-class-readonly')).not.toBeInTheDocument();
    });

    it('does not render any field outside the profile/stats overridable list', () => {
        const tpl = {
            schema: 'saveforge.build-template',
            version: 2,
            selection: { profile: { level: true } },
            sections: {
                profile: { level: 10 },
                equipment: { weapon: 'fake' }, // outside scope
                spells: { fireball: 1 },        // outside scope
            },
        };
        render(
            <ApplyOverridesPanel
                canonicalJSON={JSON.stringify(tpl)}
                onMutatedChange={() => {}}
            />,
        );
        // Only profile + stats grids are rendered. The grid lists the
        // overridable fields by key as the testid suffix; no equipment /
        // spells row should appear.
        expect(screen.queryByTestId('apply-overrides-profile-row-weapon')).not.toBeInTheDocument();
        expect(screen.queryByTestId('apply-overrides-stats-row-fireball')).not.toBeInTheDocument();
    });

    it('renders the invalid-json banner when canonical JSON is not parsable', () => {
        render(
            <ApplyOverridesPanel
                canonicalJSON="not-json"
                onMutatedChange={() => {}}
            />,
        );
        const panel = screen.getByTestId('apply-overrides-panel');
        expect(panel).toHaveAttribute('data-state', 'invalid-json');
        expect(panel).toHaveTextContent(/Could not parse template JSON/);
    });
});

describe('ApplyOverridesPanel — mutation', () => {
    it('emits a mutated canonical JSON when a selected stat value is changed', () => {
        const onMutatedChange = vi.fn();
        render(
            <ApplyOverridesPanel
                canonicalJSON={canonicalJSON()}
                onMutatedChange={onMutatedChange}
            />,
        );
        onMutatedChange.mockClear();
        fireEvent.change(screen.getByTestId('apply-overrides-stats-input-vigor'), {
            target: { value: '40' },
        });
        const lastCall = onMutatedChange.mock.calls.at(-1);
        expect(lastCall).toBeDefined();
        const [json, invalid] = lastCall!;
        expect(invalid).toBe(false);
        const parsed = JSON.parse(json as string);
        expect(parsed.sections.stats.vigor).toBe(40);
        expect(parsed.selection.stats.vigor).toBe(true);
    });

    it('marks state invalid when stat value is outside [1, 99]', () => {
        const onMutatedChange = vi.fn();
        render(
            <ApplyOverridesPanel
                canonicalJSON={canonicalJSON()}
                onMutatedChange={onMutatedChange}
            />,
        );
        fireEvent.change(screen.getByTestId('apply-overrides-stats-input-vigor'), {
            target: { value: '200' },
        });
        const lastCall = onMutatedChange.mock.calls.at(-1);
        expect(lastCall).toBeDefined();
        const [json, invalid, fieldErrors] = lastCall!;
        expect(json).toBeNull();
        expect(invalid).toBe(true);
        expect(fieldErrors).toMatchObject({ 'stats.vigor': expect.stringMatching(/1–99|1-99/) });
        expect(screen.getByTestId('apply-overrides-stats-error-vigor')).toBeInTheDocument();
    });

    it('disables input for an unselected field; toggle on enables editing and selection', () => {
        const tpl = {
            schema: 'saveforge.build-template',
            version: 2,
            selection: { profile: { level: true } },
            sections: { profile: { level: 50 } },
        };
        const onMutatedChange = vi.fn();
        render(
            <ApplyOverridesPanel
                canonicalJSON={JSON.stringify(tpl)}
                onMutatedChange={onMutatedChange}
            />,
        );
        const vigorInput = screen.getByTestId('apply-overrides-stats-input-vigor');
        expect(vigorInput).toBeDisabled();
        const vigorToggle = screen.getByTestId('apply-overrides-stats-toggle-vigor');
        fireEvent.click(vigorToggle);
        expect(screen.getByTestId('apply-overrides-stats-input-vigor')).not.toBeDisabled();
        // Enabling without a value yields invalid (Value required) until user types.
        fireEvent.change(screen.getByTestId('apply-overrides-stats-input-vigor'), {
            target: { value: '15' },
        });
        const lastCall = onMutatedChange.mock.calls.at(-1);
        const [json, invalid] = lastCall!;
        expect(invalid).toBe(false);
        const parsed = JSON.parse(json as string);
        expect(parsed.selection.stats.vigor).toBe(true);
        expect(parsed.sections.stats.vigor).toBe(15);
    });

    it('toggling off an originally-present field removes it from sections + selection', () => {
        const onMutatedChange = vi.fn();
        render(
            <ApplyOverridesPanel
                canonicalJSON={canonicalJSON()}
                onMutatedChange={onMutatedChange}
            />,
        );
        fireEvent.click(screen.getByTestId('apply-overrides-stats-toggle-vigor'));
        const lastCall = onMutatedChange.mock.calls.at(-1);
        const [json, invalid] = lastCall!;
        expect(invalid).toBe(false);
        const parsed = JSON.parse(json as string);
        expect(parsed.sections.stats?.vigor).toBeUndefined();
        expect(parsed.selection.stats?.vigor).toBeUndefined();
        // faith was originally selected; should remain
        expect(parsed.sections.stats.faith).toBe(18);
    });

    it('preserves non-profile/non-stats sections verbatim', () => {
        const tpl = {
            schema: 'saveforge.build-template',
            version: 2,
            selection: {
                profile: { level: true },
                inventory: { workspace: true },
            },
            sections: {
                profile: { level: 50 },
                inventory: { workspace: { entries: [{ id: 'item1' }] } },
            },
        };
        const onMutatedChange = vi.fn();
        render(
            <ApplyOverridesPanel
                canonicalJSON={JSON.stringify(tpl)}
                onMutatedChange={onMutatedChange}
            />,
        );
        fireEvent.change(screen.getByTestId('apply-overrides-profile-input-level'), {
            target: { value: '60' },
        });
        const lastCall = onMutatedChange.mock.calls.at(-1);
        const [json] = lastCall!;
        const parsed = JSON.parse(json as string);
        expect(parsed.sections.inventory).toEqual({ workspace: { entries: [{ id: 'item1' }] } });
        expect(parsed.selection.inventory).toEqual({ workspace: true });
    });

    it('clearCount field enforces [0, 7] range', () => {
        const tpl = {
            schema: 'saveforge.build-template',
            version: 2,
            selection: { profile: { clearCount: true } },
            sections: { profile: { clearCount: 3 } },
        };
        render(
            <ApplyOverridesPanel
                canonicalJSON={JSON.stringify(tpl)}
                onMutatedChange={() => {}}
            />,
        );
        fireEvent.change(screen.getByTestId('apply-overrides-profile-input-clearCount'), {
            target: { value: '8' },
        });
        expect(screen.getByTestId('apply-overrides-profile-error-clearCount')).toHaveTextContent(/0–7|0-7/);
    });

    it('runes soft-cap surfaces an amber warning without invalidating the field', () => {
        const tpl = {
            schema: 'saveforge.build-template',
            version: 2,
            selection: { profile: { runes: true } },
            sections: { profile: { runes: 1_000_000 } },
        };
        const onMutatedChange = vi.fn();
        render(
            <ApplyOverridesPanel
                canonicalJSON={JSON.stringify(tpl)}
                onMutatedChange={onMutatedChange}
            />,
        );
        fireEvent.change(screen.getByTestId('apply-overrides-profile-input-runes'), {
            target: { value: '1500000000' },
        });
        const lastCall = onMutatedChange.mock.calls.at(-1);
        const [json, invalid] = lastCall!;
        expect(invalid).toBe(false);
        expect(JSON.parse(json as string).sections.profile.runes).toBe(1_500_000_000);
        expect(screen.getByTestId('apply-overrides-profile-soft-warning-runes')).toBeInTheDocument();
    });
});

describe('applyOverridesToCanonical — direct helper', () => {
    it('returns null on invalid input JSON', () => {
        const result = applyOverridesToCanonical('not-json', { profile: {}, stats: {} });
        expect(result.json).toBeNull();
        expect(result.hasInvalid).toBe(true);
    });

    it('round-trips an unedited template byte-equivalent (modulo key order)', () => {
        const json = canonicalJSON();
        const result = applyOverridesToCanonical(json, {
            profile: {},
            stats: {},
        });
        expect(result.json).not.toBeNull();
        expect(JSON.parse(result.json!)).toEqual(JSON.parse(json));
    });
});

describe('ApplyOverridesModal — modal wrapper', () => {
    it('renders the panel inside a dialog with the source label', () => {
        render(
            <ApplyOverridesModal
                sourceLabel="Imported YAML — /tmp/test.yaml"
                canonicalJSON={canonicalJSON()}
                onCancel={() => {}}
                onConfirm={() => {}}
            />,
        );
        expect(screen.getByTestId('apply-overrides-modal')).toBeInTheDocument();
        expect(screen.getByTestId('apply-overrides-source-label')).toHaveTextContent(/test\.yaml/);
        expect(screen.getByTestId('apply-overrides-panel')).toBeInTheDocument();
    });

    it('Apply button is enabled when overrides are valid and clicking forwards mutated JSON', async () => {
        const onConfirm = vi.fn();
        render(
            <ApplyOverridesModal
                sourceLabel="Imported YAML"
                canonicalJSON={canonicalJSON()}
                onCancel={() => {}}
                onConfirm={onConfirm}
            />,
        );
        const applyBtn = screen.getByTestId('apply-overrides-apply');
        expect(applyBtn).toBeEnabled();
        fireEvent.change(screen.getByTestId('apply-overrides-stats-input-vigor'), {
            target: { value: '40' },
        });
        fireEvent.click(applyBtn);
        expect(onConfirm).toHaveBeenCalledTimes(1);
        const arg = onConfirm.mock.calls[0][0];
        const parsed = JSON.parse(arg as string);
        expect(parsed.sections.stats.vigor).toBe(40);
    });

    it('Apply button disables when any field is invalid', () => {
        render(
            <ApplyOverridesModal
                sourceLabel="Imported YAML"
                canonicalJSON={canonicalJSON()}
                onCancel={() => {}}
                onConfirm={() => {}}
            />,
        );
        fireEvent.change(screen.getByTestId('apply-overrides-stats-input-vigor'), {
            target: { value: '999' },
        });
        const applyBtn = screen.getByTestId('apply-overrides-apply');
        expect(applyBtn).toBeDisabled();
        expect(applyBtn.getAttribute('title')).toMatch(/fix/i);
        expect(screen.getByTestId('apply-overrides-status')).toHaveTextContent(/need attention/i);
    });

    it('Cancel button calls onCancel and does not call onConfirm', () => {
        const onCancel = vi.fn();
        const onConfirm = vi.fn();
        render(
            <ApplyOverridesModal
                sourceLabel="Imported YAML"
                canonicalJSON={canonicalJSON()}
                onCancel={onCancel}
                onConfirm={onConfirm}
            />,
        );
        fireEvent.click(screen.getByTestId('apply-overrides-cancel'));
        expect(onCancel).toHaveBeenCalledTimes(1);
        expect(onConfirm).not.toHaveBeenCalled();
    });

    it('Apply button label switches to "Applying…" and disables while applying', () => {
        render(
            <ApplyOverridesModal
                sourceLabel="Imported YAML"
                canonicalJSON={canonicalJSON()}
                onCancel={() => {}}
                onConfirm={() => {}}
                applying
            />,
        );
        const applyBtn = screen.getByTestId('apply-overrides-apply');
        expect(applyBtn).toBeDisabled();
        expect(applyBtn).toHaveTextContent(/Applying/);
        expect(screen.getByTestId('apply-overrides-cancel')).toBeDisabled();
    });
});
