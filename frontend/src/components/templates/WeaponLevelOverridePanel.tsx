import { useEffect, useState } from 'react';

// WeaponLevelOverridePanel — Phase 7a.2.
//
// Standalone runtime-options builder for the v2 inventory.workspace
// Apply path. Mirrors the Phase 6b dropdown in SortOrderTab but lives
// inside ApplyOverridesModal so the v2 Apply with overrides flow can
// override upgrade levels for weapons added by an inventory template.
//
// The panel owns its own state and emits a structurally-valid override
// payload (or undefined) to the parent through onChange. It also reports
// whether the current input is invalid so the parent can disable the
// Apply CTA. Profile / stats overrides are NOT this panel's concern —
// they live in ApplyOverridesPanel and travel through the canonical JSON.
//
// Wire shape: the emitted object matches the Go-side WeaponLevelOverride
// struct exactly (enabled + optional standardLevel / somberLevel). nil
// pointers in Go are mapped to omitted JSON fields here, so a disabled
// override emits undefined and the on-wire options object stays identical
// to a vanilla v2 apply.

export type WeaponOverridePayload =
    | { enabled: true; standardLevel?: number; somberLevel?: number }
    | undefined;

interface WeaponLevelOverridePanelProps {
    onChange: (override: WeaponOverridePayload, hasInvalid: boolean) => void;
    disabled?: boolean;
}

// parseOverrideLevel mirrors SortOrderTab.parseOverrideLevel:
//   null    — field empty, leave that weapon class unchanged
//   NaN     — non-integer / negative, surface as inline error
//   integer — accepted value (cap validated separately)
function parseOverrideLevel(text: string): number | null {
    const t = text.trim();
    if (t === '') return null;
    const n = Number(t);
    if (!Number.isInteger(n) || n < 0) return NaN;
    return n;
}

export function WeaponLevelOverridePanel({ onChange, disabled }: WeaponLevelOverridePanelProps) {
    const [enabled, setEnabled] = useState(false);
    const [standardText, setStandardText] = useState('');
    const [somberText, setSomberText] = useState('');

    const standardRaw = parseOverrideLevel(standardText);
    const somberRaw = parseOverrideLevel(somberText);
    const standardInvalid =
        enabled &&
        (Number.isNaN(standardRaw) ||
            (typeof standardRaw === 'number' && standardRaw > 25));
    const somberInvalid =
        enabled &&
        (Number.isNaN(somberRaw) ||
            (typeof somberRaw === 'number' && somberRaw > 10));
    const requiresOne = enabled && standardRaw === null && somberRaw === null;
    const hasInvalid = standardInvalid || somberInvalid || requiresOne;

    useEffect(() => {
        if (!enabled) {
            onChange(undefined, false);
            return;
        }
        if (hasInvalid) {
            onChange(undefined, true);
            return;
        }
        const payload: { enabled: true; standardLevel?: number; somberLevel?: number } = {
            enabled: true,
        };
        if (typeof standardRaw === 'number') payload.standardLevel = standardRaw;
        if (typeof somberRaw === 'number') payload.somberLevel = somberRaw;
        onChange(payload, false);
    }, [enabled, standardRaw, somberRaw, hasInvalid, onChange]);

    return (
        <section
            data-testid="apply-overrides-weapon-panel"
            aria-label="Weapon level override"
            className="space-y-1.5"
        >
            <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                Weapon levels
            </h3>
            <label className="flex items-center gap-1.5 text-[11px] cursor-pointer">
                <input
                    type="checkbox"
                    data-testid="apply-overrides-weapon-enabled"
                    checked={enabled}
                    onChange={(e) => setEnabled(e.target.checked)}
                    disabled={disabled}
                />
                Override weapon levels for imported items
            </label>
            {enabled && (
                <div className="space-y-1 pl-5">
                    <div className="flex items-center gap-2">
                        <label className="text-[10px] text-muted-foreground w-16">Standard +</label>
                        <input
                            type="number"
                            min={0}
                            max={25}
                            data-testid="apply-overrides-weapon-standard"
                            aria-invalid={standardInvalid}
                            value={standardText}
                            onChange={(e) => setStandardText(e.target.value)}
                            disabled={disabled}
                            placeholder="—"
                            className={`w-16 px-1 py-0.5 text-[11px] rounded border bg-background/40 ${
                                standardInvalid ? 'border-red-500/60' : 'border-border/60'
                            } disabled:opacity-40`}
                        />
                        <span className="text-[10px] text-muted-foreground">(0–25)</span>
                    </div>
                    <div className="flex items-center gap-2">
                        <label className="text-[10px] text-muted-foreground w-16">Somber +</label>
                        <input
                            type="number"
                            min={0}
                            max={10}
                            data-testid="apply-overrides-weapon-somber"
                            aria-invalid={somberInvalid}
                            value={somberText}
                            onChange={(e) => setSomberText(e.target.value)}
                            disabled={disabled}
                            placeholder="—"
                            className={`w-16 px-1 py-0.5 text-[11px] rounded border bg-background/40 ${
                                somberInvalid ? 'border-red-500/60' : 'border-border/60'
                            } disabled:opacity-40`}
                        />
                        <span className="text-[10px] text-muted-foreground">(0–10)</span>
                    </div>
                    {hasInvalid && (
                        <p
                            className="text-[10px] text-red-400"
                            data-testid="apply-overrides-weapon-error"
                        >
                            {requiresOne
                                ? 'At least one level must be set.'
                                : standardInvalid
                                  ? 'Standard level must be 0–25.'
                                  : 'Somber level must be 0–10.'}
                        </p>
                    )}
                    <p className="text-[10px] text-muted-foreground italic">
                        Applied AFTER the template's own upgrade values, only to weapons added by this apply.
                        Leave a field empty to skip that class.
                    </p>
                </div>
            )}
        </section>
    );
}
