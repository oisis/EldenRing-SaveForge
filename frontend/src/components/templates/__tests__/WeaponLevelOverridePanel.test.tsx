import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import {
    WeaponLevelOverridePanel,
    WeaponOverridePayload,
} from '../WeaponLevelOverridePanel';

// Phase 7a.2 — WeaponLevelOverridePanel unit tests. The panel is the
// runtime-options builder embedded inside ApplyOverridesModal for v2
// inventory.workspace apply paths. It is intentionally decoupled from
// the canonical-JSON mutator; this test file verifies the override
// payload + invalid-flag contract that ApplyOverridesModal relies on.

function renderPanel(onChange = vi.fn<[WeaponOverridePayload, boolean]>()) {
    render(<WeaponLevelOverridePanel onChange={onChange} />);
    return onChange;
}

function lastCall(mock: ReturnType<typeof vi.fn>): [WeaponOverridePayload, boolean] {
    const calls = mock.mock.calls;
    if (calls.length === 0) throw new Error('onChange was never called');
    return calls[calls.length - 1] as [WeaponOverridePayload, boolean];
}

describe('WeaponLevelOverridePanel — default', () => {
    it('renders disabled by default with no inputs visible', () => {
        const onChange = renderPanel();
        expect(screen.getByTestId('apply-overrides-weapon-panel')).toBeInTheDocument();
        expect(screen.getByTestId('apply-overrides-weapon-enabled')).not.toBeChecked();
        expect(screen.queryByTestId('apply-overrides-weapon-standard')).not.toBeInTheDocument();
        expect(screen.queryByTestId('apply-overrides-weapon-somber')).not.toBeInTheDocument();
        const [override, invalid] = lastCall(onChange);
        expect(override).toBeUndefined();
        expect(invalid).toBe(false);
    });

    it('reveals both inputs when the master toggle is enabled', () => {
        renderPanel();
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        expect(screen.getByTestId('apply-overrides-weapon-standard')).toBeInTheDocument();
        expect(screen.getByTestId('apply-overrides-weapon-somber')).toBeInTheDocument();
    });
});

describe('WeaponLevelOverridePanel — validation', () => {
    it('flags hasInvalid + shows error when enabled with both inputs empty', () => {
        const onChange = renderPanel();
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        expect(screen.getByTestId('apply-overrides-weapon-error')).toHaveTextContent(/at least one level/i);
        const [override, invalid] = lastCall(onChange);
        expect(override).toBeUndefined();
        expect(invalid).toBe(true);
    });

    it('flags hasInvalid when standard exceeds the +25 cap', () => {
        const onChange = renderPanel();
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        fireEvent.change(screen.getByTestId('apply-overrides-weapon-standard'), {
            target: { value: '26' },
        });
        expect(screen.getByTestId('apply-overrides-weapon-error')).toHaveTextContent(/Standard level must be 0–25/i);
        const [override, invalid] = lastCall(onChange);
        expect(override).toBeUndefined();
        expect(invalid).toBe(true);
    });

    it('flags hasInvalid when somber exceeds the +10 cap', () => {
        const onChange = renderPanel();
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        fireEvent.change(screen.getByTestId('apply-overrides-weapon-somber'), {
            target: { value: '11' },
        });
        expect(screen.getByTestId('apply-overrides-weapon-error')).toHaveTextContent(/Somber level must be 0–10/i);
        const [override, invalid] = lastCall(onChange);
        expect(override).toBeUndefined();
        expect(invalid).toBe(true);
    });
});

describe('WeaponLevelOverridePanel — emission', () => {
    it('emits standardLevel only when somber is left empty', () => {
        const onChange = renderPanel();
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        fireEvent.change(screen.getByTestId('apply-overrides-weapon-standard'), {
            target: { value: '25' },
        });
        const [override, invalid] = lastCall(onChange);
        expect(invalid).toBe(false);
        expect(override).toEqual({ enabled: true, standardLevel: 25 });
    });

    it('emits somberLevel only when standard is left empty', () => {
        const onChange = renderPanel();
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        fireEvent.change(screen.getByTestId('apply-overrides-weapon-somber'), {
            target: { value: '10' },
        });
        const [override, invalid] = lastCall(onChange);
        expect(invalid).toBe(false);
        expect(override).toEqual({ enabled: true, somberLevel: 10 });
    });

    it('emits both levels when both inputs are filled', () => {
        const onChange = renderPanel();
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        fireEvent.change(screen.getByTestId('apply-overrides-weapon-standard'), {
            target: { value: '20' },
        });
        fireEvent.change(screen.getByTestId('apply-overrides-weapon-somber'), {
            target: { value: '8' },
        });
        const [override, invalid] = lastCall(onChange);
        expect(invalid).toBe(false);
        expect(override).toEqual({ enabled: true, standardLevel: 20, somberLevel: 8 });
    });

    it('emits undefined and clears invalid when the master toggle is disabled after entering values', () => {
        const onChange = renderPanel();
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        fireEvent.change(screen.getByTestId('apply-overrides-weapon-standard'), {
            target: { value: '15' },
        });
        // sanity: enabled emits a payload first
        expect(lastCall(onChange)[0]).toEqual({ enabled: true, standardLevel: 15 });
        // disable
        fireEvent.click(screen.getByTestId('apply-overrides-weapon-enabled'));
        const [override, invalid] = lastCall(onChange);
        expect(override).toBeUndefined();
        expect(invalid).toBe(false);
        expect(screen.queryByTestId('apply-overrides-weapon-standard')).not.toBeInTheDocument();
    });
});
