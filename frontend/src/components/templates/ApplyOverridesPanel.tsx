import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { WeaponLevelOverridePanel, WeaponOverridePayload } from './WeaponLevelOverridePanel';

// ApplyOverridesPanel + ApplyOverridesModal — Phase 6.
//
// Lets the user edit the profile / stats values of a schema v2 template
// before it is applied to a character, without changing the backend
// contract. The panel parses the canonical JSON the caller already
// produced (either from the YAML import preview or from
// PreviewBuildTemplateFromLibrary), keeps a per-field draft state, and
// re-emits a mutated canonical JSON string that the caller passes
// verbatim to ApplyBuildTemplateV2ToCharacterJSON.
//
// Editable fields are strictly the subset that the Phase 5 backend
// writer already accepts:
//   profile: name, level, runes, soulMemory, clearCount,
//            scadutreeBlessing, shadowRealmBlessing, talismanSlots
//   stats:   vigor, mind, endurance, strength, dexterity,
//            intelligence, faith, arcane
//
// profile.class is intentionally rendered read-only with a
// "skipped on apply" hint — see Phase 5A scope notes in
// app_templates_v2_apply.go and spec/56 §17a.
//
// Anything outside profile/stats is not rendered and is preserved
// verbatim in the JSON pass-through.

export type ProfileOverrideKey =
    | 'name'
    | 'level'
    | 'runes'
    | 'soulMemory'
    | 'clearCount'
    | 'scadutreeBlessing'
    | 'shadowRealmBlessing'
    | 'talismanSlots';

export type StatsOverrideKey =
    | 'vigor'
    | 'mind'
    | 'endurance'
    | 'strength'
    | 'dexterity'
    | 'intelligence'
    | 'faith'
    | 'arcane';

type NumericRange = { min: number; max: number };

interface FieldMeta<K extends string> {
    key: K;
    label: string;
    kind: 'integer' | 'text';
    range?: NumericRange;
    softCap?: number;
    hint?: string;
}

export const OVERRIDABLE_PROFILE_FIELDS: ReadonlyArray<FieldMeta<ProfileOverrideKey>> = [
    { key: 'name', label: 'Name', kind: 'text', hint: 'UTF-16 ≤ 16 code units (backend enforces).' },
    { key: 'level', label: 'Level', kind: 'integer', range: { min: 1, max: 713 } },
    {
        key: 'runes',
        label: 'Runes',
        kind: 'integer',
        range: { min: 0, max: 4_294_967_295 },
        softCap: 999_000_000,
        hint: 'Above the soft cap is unusual for vanilla saves.',
    },
    {
        key: 'soulMemory',
        label: 'Soul Memory',
        kind: 'integer',
        range: { min: 0, max: 4_294_967_295 },
        hint: 'Will be bumped to the runes-cost-for-level minimum if too low.',
    },
    { key: 'clearCount', label: 'NG+ Cycle', kind: 'integer', range: { min: 0, max: 7 } },
    { key: 'scadutreeBlessing', label: 'Scadutree Blessing', kind: 'integer', range: { min: 0, max: 20 } },
    { key: 'shadowRealmBlessing', label: 'Shadow Realm Blessing', kind: 'integer', range: { min: 0, max: 10 } },
    { key: 'talismanSlots', label: 'Talisman Slots', kind: 'integer', range: { min: 0, max: 3 } },
];

export const OVERRIDABLE_STATS_FIELDS: ReadonlyArray<FieldMeta<StatsOverrideKey>> = [
    { key: 'vigor', label: 'Vigor', kind: 'integer', range: { min: 1, max: 99 } },
    { key: 'mind', label: 'Mind', kind: 'integer', range: { min: 1, max: 99 } },
    { key: 'endurance', label: 'Endurance', kind: 'integer', range: { min: 1, max: 99 } },
    { key: 'strength', label: 'Strength', kind: 'integer', range: { min: 1, max: 99 } },
    { key: 'dexterity', label: 'Dexterity', kind: 'integer', range: { min: 1, max: 99 } },
    { key: 'intelligence', label: 'Intelligence', kind: 'integer', range: { min: 1, max: 99 } },
    { key: 'faith', label: 'Faith', kind: 'integer', range: { min: 1, max: 99 } },
    { key: 'arcane', label: 'Arcane', kind: 'integer', range: { min: 1, max: 99 } },
];

interface FieldDraft {
    enabled: boolean;
    value: string;
    presentInitially: boolean;
    initialValue: string;
}

interface DraftState {
    profile: Record<string, FieldDraft>;
    stats: Record<string, FieldDraft>;
}

function rawToString(raw: unknown): string {
    if (raw === null || raw === undefined) return '';
    if (typeof raw === 'string') return raw;
    if (typeof raw === 'number') return String(raw);
    if (typeof raw === 'boolean') return raw ? 'true' : 'false';
    return '';
}

function fieldIsPresent(section: Record<string, unknown> | undefined, key: string): boolean {
    if (!section) return false;
    const v = section[key];
    return v !== undefined && v !== null;
}

function initDraft(parsed: ParsedTemplate, fields: ReadonlyArray<FieldMeta<string>>, sectionKey: 'profile' | 'stats'): Record<string, FieldDraft> {
    const sec = parsed.sections?.[sectionKey] as Record<string, unknown> | undefined;
    const sel = parsed.selection?.[sectionKey] as Record<string, unknown> | undefined;
    const out: Record<string, FieldDraft> = {};
    for (const f of fields) {
        const present = fieldIsPresent(sec, f.key);
        const initial = present ? rawToString(sec?.[f.key]) : '';
        const selected = !!sel?.[f.key] && present;
        out[f.key] = {
            enabled: selected,
            value: initial,
            presentInitially: present,
            initialValue: initial,
        };
    }
    return out;
}

interface ParsedTemplate {
    sections?: { profile?: Record<string, unknown>; stats?: Record<string, unknown> } & Record<string, unknown>;
    selection?: { profile?: Record<string, unknown>; stats?: Record<string, unknown> } & Record<string, unknown>;
    [k: string]: unknown;
}

function safeParse(canonicalJSON: string): ParsedTemplate | null {
    try {
        const obj = JSON.parse(canonicalJSON);
        if (obj === null || typeof obj !== 'object' || Array.isArray(obj)) return null;
        return obj as ParsedTemplate;
    } catch {
        return null;
    }
}

interface ValidatedField {
    key: string;
    valid: boolean;
    value: number | string | null;
    error?: string;
}

function validateField(meta: FieldMeta<string>, draft: FieldDraft): ValidatedField {
    if (!draft.enabled) {
        return { key: meta.key, valid: true, value: null };
    }
    const trimmed = draft.value.trim();
    if (meta.kind === 'integer') {
        if (trimmed === '') {
            return { key: meta.key, valid: false, value: null, error: 'Value required.' };
        }
        if (!/^-?\d+$/.test(trimmed)) {
            return { key: meta.key, valid: false, value: null, error: 'Integer required.' };
        }
        const n = Number(trimmed);
        if (!Number.isFinite(n) || !Number.isInteger(n)) {
            return { key: meta.key, valid: false, value: null, error: 'Integer required.' };
        }
        if (meta.range) {
            if (n < meta.range.min || n > meta.range.max) {
                return {
                    key: meta.key,
                    valid: false,
                    value: null,
                    error: `Must be ${meta.range.min}–${meta.range.max}.`,
                };
            }
        }
        return { key: meta.key, valid: true, value: n };
    }
    // text
    if (trimmed === '') {
        return { key: meta.key, valid: false, value: null, error: 'Value required.' };
    }
    return { key: meta.key, valid: true, value: draft.value };
}

export interface MutatedOverrideResult {
    json: string | null;
    hasInvalid: boolean;
    hasOverrides: boolean;
    fieldErrors: Record<string, string>;
}

// applyOverridesToCanonical takes the original canonical JSON and the
// current draft state, and returns a new JSON string with the user's
// edits applied to sections.profile/stats and the matching selection
// blocks. Anything outside profile/stats — including the rest of
// `selection` and any other sections — is preserved verbatim.
export function applyOverridesToCanonical(
    canonicalJSON: string,
    draft: DraftState,
): MutatedOverrideResult {
    const parsed = safeParse(canonicalJSON);
    if (!parsed) {
        return { json: null, hasInvalid: true, hasOverrides: false, fieldErrors: {} };
    }
    const fieldErrors: Record<string, string> = {};
    let hasInvalid = false;
    let hasOverrides = false;

    const profileValidated: Record<string, ValidatedField> = {};
    for (const meta of OVERRIDABLE_PROFILE_FIELDS) {
        const d = draft.profile[meta.key];
        if (!d) continue;
        const r = validateField(meta, d);
        if (!r.valid && r.error) {
            fieldErrors[`profile.${meta.key}`] = r.error;
            hasInvalid = true;
        }
        profileValidated[meta.key] = r;
    }
    const statsValidated: Record<string, ValidatedField> = {};
    for (const meta of OVERRIDABLE_STATS_FIELDS) {
        const d = draft.stats[meta.key];
        if (!d) continue;
        const r = validateField(meta, d);
        if (!r.valid && r.error) {
            fieldErrors[`stats.${meta.key}`] = r.error;
            hasInvalid = true;
        }
        statsValidated[meta.key] = r;
    }

    if (hasInvalid) {
        return { json: null, hasInvalid: true, hasOverrides: false, fieldErrors };
    }

    // Deep clone via JSON round-trip — the canonical JSON is already
    // JSON-safe by construction, so the round-trip preserves it exactly.
    const next = JSON.parse(canonicalJSON) as ParsedTemplate;
    next.sections = (next.sections ?? {}) as ParsedTemplate['sections'];
    next.selection = (next.selection ?? {}) as ParsedTemplate['selection'];

    const writeSection = (
        sectionKey: 'profile' | 'stats',
        fields: ReadonlyArray<FieldMeta<string>>,
        validated: Record<string, ValidatedField>,
    ) => {
        const secObj = (next.sections![sectionKey] as Record<string, unknown> | undefined) ?? {};
        const selObj = (next.selection![sectionKey] as Record<string, unknown> | undefined) ?? {};
        const drafts = sectionKey === 'profile' ? draft.profile : draft.stats;
        let touched = false;
        for (const meta of fields) {
            const d = drafts[meta.key];
            if (!d) continue;
            const r = validated[meta.key];
            if (!r) continue;
            if (d.enabled) {
                if (r.value === null) continue;
                secObj[meta.key] = r.value;
                selObj[meta.key] = true;
                touched = true;
                if (!d.presentInitially || d.value !== d.initialValue) {
                    hasOverrides = true;
                }
            } else {
                if (meta.key in secObj || meta.key in selObj) {
                    delete secObj[meta.key];
                    delete selObj[meta.key];
                    touched = true;
                }
                if (d.presentInitially) hasOverrides = true;
            }
        }
        if (touched) {
            if (Object.keys(secObj).length > 0) {
                (next.sections as Record<string, unknown>)[sectionKey] = secObj;
            } else {
                delete (next.sections as Record<string, unknown>)[sectionKey];
            }
            if (Object.keys(selObj).length > 0) {
                (next.selection as Record<string, unknown>)[sectionKey] = selObj;
            } else {
                delete (next.selection as Record<string, unknown>)[sectionKey];
            }
        }
    };

    writeSection('profile', OVERRIDABLE_PROFILE_FIELDS, profileValidated);
    writeSection('stats', OVERRIDABLE_STATS_FIELDS, statsValidated);

    return { json: JSON.stringify(next), hasInvalid: false, hasOverrides, fieldErrors };
}

interface ApplyOverridesPanelProps {
    canonicalJSON: string;
    onMutatedChange: (mutated: string | null, hasInvalid: boolean, fieldErrors: Record<string, string>) => void;
    disabled?: boolean;
}

// ApplyOverridesPanel renders the editable profile/stats grid for a
// parsed canonical JSON template. It owns the draft state, validates on
// every keystroke, and emits the mutated canonical JSON through
// onMutatedChange. The caller decides what to do with the emitted JSON
// — pass it to ApplyBuildTemplateV2ToCharacterJSON or hold it until the
// user confirms.
export function ApplyOverridesPanel({ canonicalJSON, onMutatedChange, disabled }: ApplyOverridesPanelProps) {
    const parsed = useMemo(() => safeParse(canonicalJSON), [canonicalJSON]);

    const [draft, setDraft] = useState<DraftState>(() => ({
        profile: parsed ? initDraft(parsed, OVERRIDABLE_PROFILE_FIELDS, 'profile') : {},
        stats: parsed ? initDraft(parsed, OVERRIDABLE_STATS_FIELDS, 'stats') : {},
    }));

    // Re-seed draft when canonicalJSON identity changes (e.g. caller
    // opens overrides for a different template).
    const lastJSON = useRef(canonicalJSON);
    useEffect(() => {
        if (lastJSON.current === canonicalJSON) return;
        lastJSON.current = canonicalJSON;
        if (parsed) {
            setDraft({
                profile: initDraft(parsed, OVERRIDABLE_PROFILE_FIELDS, 'profile'),
                stats: initDraft(parsed, OVERRIDABLE_STATS_FIELDS, 'stats'),
            });
        }
    }, [canonicalJSON, parsed]);

    useEffect(() => {
        const result = applyOverridesToCanonical(canonicalJSON, draft);
        onMutatedChange(result.json, result.hasInvalid, result.fieldErrors);
    }, [canonicalJSON, draft, onMutatedChange]);

    const onToggle = useCallback((section: 'profile' | 'stats', key: string) => {
        setDraft(prev => {
            const cur = prev[section][key];
            if (!cur) return prev;
            return {
                ...prev,
                [section]: {
                    ...prev[section],
                    [key]: { ...cur, enabled: !cur.enabled },
                },
            };
        });
    }, []);

    const onValue = useCallback((section: 'profile' | 'stats', key: string, value: string) => {
        setDraft(prev => {
            const cur = prev[section][key];
            if (!cur) return prev;
            return {
                ...prev,
                [section]: {
                    ...prev[section],
                    [key]: { ...cur, value },
                },
            };
        });
    }, []);

    if (!parsed) {
        return (
            <div
                data-testid="apply-overrides-panel"
                data-state="invalid-json"
                className="px-3 py-3 text-[11px] text-red-300"
            >
                Could not parse template JSON — overrides cannot be edited.
            </div>
        );
    }

    const classRaw = (parsed.sections?.profile as Record<string, unknown> | undefined)?.['class'];
    const classText = typeof classRaw === 'string' ? classRaw : '';

    const liveResult = applyOverridesToCanonical(canonicalJSON, draft);

    return (
        <div data-testid="apply-overrides-panel" className="space-y-4 text-[12px]">
            <FieldGrid
                heading="Profile"
                testidPrefix="apply-overrides-profile"
                fields={OVERRIDABLE_PROFILE_FIELDS}
                section="profile"
                drafts={draft.profile}
                errors={liveResult.fieldErrors}
                disabled={disabled}
                onToggle={onToggle}
                onValue={onValue}
                trailingNote={
                    classText !== '' ? (
                        <ClassReadonlyRow value={classText} />
                    ) : null
                }
            />
            <FieldGrid
                heading="Stats"
                testidPrefix="apply-overrides-stats"
                fields={OVERRIDABLE_STATS_FIELDS}
                section="stats"
                drafts={draft.stats}
                errors={liveResult.fieldErrors}
                disabled={disabled}
                onToggle={onToggle}
                onValue={onValue}
            />
        </div>
    );
}

interface FieldGridProps<K extends string> {
    heading: string;
    testidPrefix: string;
    fields: ReadonlyArray<FieldMeta<K>>;
    section: 'profile' | 'stats';
    drafts: Record<string, FieldDraft>;
    errors: Record<string, string>;
    disabled?: boolean;
    onToggle: (section: 'profile' | 'stats', key: string) => void;
    onValue: (section: 'profile' | 'stats', key: string, value: string) => void;
    trailingNote?: ReactNode;
}

function FieldGrid<K extends string>({
    heading,
    testidPrefix,
    fields,
    section,
    drafts,
    errors,
    disabled,
    onToggle,
    onValue,
    trailingNote,
}: FieldGridProps<K>) {
    return (
        <section aria-label={`${heading} overrides`} className="space-y-1.5">
            <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{heading}</h3>
            <ul className="grid grid-cols-1 gap-1.5">
                {fields.map(f => {
                    const draft = drafts[f.key];
                    if (!draft) return null;
                    const errorKey = `${section}.${f.key}`;
                    const error = errors[errorKey];
                    const rangeHint = f.range ? `${f.range.min}–${f.range.max}` : f.hint ?? '';
                    const softWarning =
                        draft.enabled && f.softCap && !error && draft.value.trim() !== ''
                            ? (() => {
                                const n = Number(draft.value);
                                return Number.isFinite(n) && n > f.softCap ? `Above the soft cap (${f.softCap}).` : '';
                            })()
                            : '';
                    return (
                        <li
                            key={f.key}
                            data-testid={`${testidPrefix}-row-${f.key}`}
                            className="grid grid-cols-[18px_140px_1fr_140px] items-center gap-2"
                        >
                            <input
                                type="checkbox"
                                data-testid={`${testidPrefix}-toggle-${f.key}`}
                                checked={draft.enabled}
                                onChange={() => onToggle(section, f.key)}
                                disabled={disabled}
                                aria-label={`Apply ${f.label}`}
                            />
                            <label
                                htmlFor={`${testidPrefix}-input-${f.key}`}
                                className="select-none cursor-pointer"
                            >
                                {f.label}
                            </label>
                            <input
                                id={`${testidPrefix}-input-${f.key}`}
                                data-testid={`${testidPrefix}-input-${f.key}`}
                                type={f.kind === 'integer' ? 'text' : 'text'}
                                inputMode={f.kind === 'integer' ? 'numeric' : 'text'}
                                value={draft.value}
                                onChange={e => onValue(section, f.key, e.target.value)}
                                disabled={disabled || !draft.enabled}
                                aria-invalid={!!error}
                                className={`rounded border px-2 py-1 text-foreground bg-background/40 ${
                                    error ? 'border-red-500/60' : 'border-border/60'
                                } disabled:opacity-40`}
                            />
                            <div className="text-[10px] text-muted-foreground">
                                {rangeHint && <div data-testid={`${testidPrefix}-range-${f.key}`}>{rangeHint}</div>}
                                {error && (
                                    <div
                                        data-testid={`${testidPrefix}-error-${f.key}`}
                                        className="text-red-300"
                                    >
                                        {error}
                                    </div>
                                )}
                                {!error && softWarning && (
                                    <div
                                        data-testid={`${testidPrefix}-soft-warning-${f.key}`}
                                        className="text-amber-300"
                                    >
                                        {softWarning}
                                    </div>
                                )}
                            </div>
                        </li>
                    );
                })}
            </ul>
            {trailingNote}
        </section>
    );
}

function ClassReadonlyRow({ value }: { value: string }) {
    return (
        <div
            data-testid="apply-overrides-profile-class-readonly"
            className="grid grid-cols-[18px_140px_1fr_140px] items-center gap-2 text-muted-foreground"
        >
            <span aria-hidden="true" />
            <span>Class</span>
            <span className="rounded border border-border/40 bg-background/20 px-2 py-1 italic">
                {value}
            </span>
            <span className="text-[10px] italic">Skipped on apply (Phase 5).</span>
        </div>
    );
}

interface ApplyOverridesModalProps {
    sourceLabel: string;
    canonicalJSON: string;
    onCancel: () => void;
    onConfirm: (mutatedJSON: string, weaponOverride?: WeaponOverridePayload) => void | Promise<void>;
    applying?: boolean;
}

// ApplyOverridesModal is the thin modal shell around ApplyOverridesPanel.
// The shell owns the panel's mutated-JSON state and the apply CTA; the
// caller wires onConfirm to the actual ApplyBuildTemplateV2ToCharacterJSON
// invocation.
//
// Phase 7a.2 — when the canonical JSON's selection nominates
// inventory.workspace, the modal also renders WeaponLevelOverridePanel
// below the profile/stats grid. The weapon override is a runtime apply
// option (not a JSON-mutable field), so it travels through onConfirm's
// second argument instead of being baked into the mutated canonical JSON.
// Profile/stats-only templates leave the weapon panel unrendered and
// onConfirm receives undefined for that argument.
export function ApplyOverridesModal({
    sourceLabel,
    canonicalJSON,
    onCancel,
    onConfirm,
    applying,
}: ApplyOverridesModalProps) {
    const dialogRef = useRef<HTMLDivElement | null>(null);
    const [mutatedJSON, setMutatedJSON] = useState<string | null>(canonicalJSON);
    const [hasInvalid, setHasInvalid] = useState(false);
    const [errorCount, setErrorCount] = useState(0);
    const [weaponOverride, setWeaponOverride] = useState<WeaponOverridePayload>(undefined);
    const [weaponInvalid, setWeaponInvalid] = useState(false);

    const showWeaponOverride = useMemo(() => {
        const parsed = safeParse(canonicalJSON);
        const sel = parsed?.selection as Record<string, unknown> | undefined;
        const inv = sel?.['inventory.workspace'];
        if (inv === undefined || inv === null) return false;
        if (typeof inv === 'boolean') return inv;
        if (typeof inv === 'object' && !Array.isArray(inv)) {
            const obj = inv as Record<string, unknown>;
            return obj.all === true || Object.keys(obj).length > 0;
        }
        return false;
    }, [canonicalJSON]);

    useEffect(() => {
        dialogRef.current?.focus();
    }, []);

    const handleMutated = useCallback(
        (json: string | null, invalid: boolean, fieldErrors: Record<string, string>) => {
            setMutatedJSON(json);
            setHasInvalid(invalid);
            setErrorCount(Object.keys(fieldErrors).length);
        },
        [],
    );

    const handleWeaponChange = useCallback(
        (override: WeaponOverridePayload, invalid: boolean) => {
            setWeaponOverride(override);
            setWeaponInvalid(invalid);
        },
        [],
    );

    const canApply = !applying && !hasInvalid && !weaponInvalid && mutatedJSON !== null;

    const onApplyClick = () => {
        if (!canApply || mutatedJSON === null) return;
        void onConfirm(mutatedJSON, weaponOverride);
    };

    return (
        <div
            data-testid="apply-overrides-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Apply Build Template with Overrides"
            ref={dialogRef}
            tabIndex={-1}
            className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 p-4"
        >
            <div className="w-full max-w-2xl rounded-lg bg-card border border-border/60 shadow-xl flex flex-col max-h-[85vh]">
                <div className="px-4 py-3 border-b border-border/60">
                    <h2 className="text-sm font-black uppercase tracking-wider">
                        Apply with overrides
                    </h2>
                    <p
                        data-testid="apply-overrides-source-label"
                        className="mt-1 text-[11px] text-muted-foreground break-all"
                    >
                        {sourceLabel}
                    </p>
                    <p className="mt-1 text-[11px] text-muted-foreground">
                        Edit profile / stats values before applying. The backend still validates ranges and rejects
                        any value it can't accept — this panel pre-checks the obvious ranges so you don't have to
                        round-trip.
                    </p>
                </div>

                <div className="px-4 py-3 overflow-y-auto space-y-4">
                    <ApplyOverridesPanel
                        canonicalJSON={canonicalJSON}
                        onMutatedChange={handleMutated}
                        disabled={applying}
                    />
                    {showWeaponOverride && (
                        <WeaponLevelOverridePanel
                            onChange={handleWeaponChange}
                            disabled={applying}
                        />
                    )}
                </div>

                <div className="px-4 py-3 border-t border-border/60 flex items-center justify-between gap-2">
                    <div data-testid="apply-overrides-status" className="text-[10px] text-muted-foreground">
                        {hasInvalid
                            ? `${errorCount} field${errorCount === 1 ? '' : 's'} need attention.`
                            : weaponInvalid
                              ? 'Fix weapon level override to apply.'
                              : 'Ready to apply.'}
                    </div>
                    <div className="flex items-center gap-2">
                        <button
                            type="button"
                            data-testid="apply-overrides-cancel"
                            onClick={onCancel}
                            disabled={applying}
                            className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40"
                        >
                            Cancel
                        </button>
                        <button
                            type="button"
                            data-testid="apply-overrides-apply"
                            onClick={onApplyClick}
                            disabled={!canApply}
                            title={hasInvalid ? 'Fix invalid values to apply.' : 'Apply with current values.'}
                            aria-label={hasInvalid ? 'Fix invalid values to apply.' : 'Apply with current values.'}
                            className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                                canApply
                                    ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                    : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                            }`}
                        >
                            {applying ? 'Applying…' : 'Apply to character'}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}
