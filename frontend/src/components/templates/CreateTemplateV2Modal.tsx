import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
    PreviewBuildTemplateV2FromCharacter,
    SaveBuildTemplateV2FromCharacterToLibrary,
} from '../../../wailsjs/go/main/App';
import { main, templates } from '../../../wailsjs/go/models';
import { ImportTemplatePreviewModal } from './ImportTemplatePreviewModal';

// CreateTemplateV2Modal collects metadata + per-field profile/stats
// selection for a schema v2 build template, calls the preview backend
// from the currently-selected character, and (on a successful preview)
// hands off to ImportTemplatePreviewModal which surfaces the v2 metadata
// block from Phase 3D.1 and offers the Save to Library action.
//
// The component is intentionally self-contained: it owns its own form
// state, imports the Wails methods directly, and does not couple to the
// global Templates shell. Phase 3D.2b wires it into TemplatesShellModal.

// Profile field keys mirror backend/templates/schema.go:profileSelectionFields.
// IMPORTANT: the class key is exactly "class" — not "className" — because
// the backend allowlist rejects any other spelling. Do NOT add a mapping
// layer here; emit "class" verbatim into selection JSON.
const PROFILE_TEMPLATE_FIELDS = [
    { key: 'name', label: 'Name' },
    { key: 'level', label: 'Level' },
    { key: 'runes', label: 'Runes' },
    { key: 'soulMemory', label: 'Soul Memory' },
    { key: 'class', label: 'Class' },
    { key: 'clearCount', label: 'NG+ Cycle' },
    { key: 'scadutreeBlessing', label: 'Scadutree Blessing' },
    { key: 'shadowRealmBlessing', label: 'Shadow Realm Blessing' },
    { key: 'talismanSlots', label: 'Talisman Slots' },
] as const;

const STATS_TEMPLATE_FIELDS = [
    { key: 'vigor', label: 'Vigor' },
    { key: 'mind', label: 'Mind' },
    { key: 'endurance', label: 'Endurance' },
    { key: 'strength', label: 'Strength' },
    { key: 'dexterity', label: 'Dexterity' },
    { key: 'intelligence', label: 'Intelligence' },
    { key: 'faith', label: 'Faith' },
    { key: 'arcane', label: 'Arcane' },
] as const;

type ProfileFieldKey = (typeof PROFILE_TEMPLATE_FIELDS)[number]['key'];
type StatsFieldKey = (typeof STATS_TEMPLATE_FIELDS)[number]['key'];

type SelectionMap<K extends string> = Partial<Record<K, boolean>>;

interface Props {
    charIndex: number;
    onClose: () => void;
    onSaved?: (entry: templates.LibraryTemplateEntry) => void;
    onError?: (err: unknown) => void;
}

export function buildSelectionJSON(
    profile: SelectionMap<ProfileFieldKey>,
    stats: SelectionMap<StatsFieldKey>,
): string {
    const out: Record<string, Record<string, boolean>> = {};
    const profilePairs: [string, boolean][] = [];
    for (const f of PROFILE_TEMPLATE_FIELDS) {
        if (profile[f.key] === true) profilePairs.push([f.key, true]);
    }
    const statsPairs: [string, boolean][] = [];
    for (const f of STATS_TEMPLATE_FIELDS) {
        if (stats[f.key] === true) statsPairs.push([f.key, true]);
    }
    if (profilePairs.length > 0) {
        out.profile = Object.fromEntries(profilePairs);
    }
    if (statsPairs.length > 0) {
        out.stats = Object.fromEntries(statsPairs);
    }
    return JSON.stringify(out);
}

export function parseTags(tagsText: string): string[] {
    return tagsText
        .split(',')
        .map(t => t.trim())
        .filter(t => t.length > 0);
}

export function CreateTemplateV2Modal({ charIndex, onClose, onSaved, onError }: Props) {
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [author, setAuthor] = useState('');
    const [tagsText, setTagsText] = useState('');
    const [profileSelection, setProfileSelection] = useState<SelectionMap<ProfileFieldKey>>({});
    const [statsSelection, setStatsSelection] = useState<SelectionMap<StatsFieldKey>>({});
    const [previewing, setPreviewing] = useState(false);
    const [savingToLibrary, setSavingToLibrary] = useState(false);
    const [previewReport, setPreviewReport] = useState<templates.ImportPreviewReport | null>(null);
    const [pendingSelectionJSON, setPendingSelectionJSON] = useState<string | null>(null);
    const [pendingOpts, setPendingOpts] = useState<main.BuildTemplateV2ExportOptions | null>(null);

    const dialogRef = useRef<HTMLDivElement | null>(null);
    useEffect(() => {
        dialogRef.current?.focus();
    }, []);

    const hasAnyProfile = useMemo(
        () => PROFILE_TEMPLATE_FIELDS.some(f => profileSelection[f.key] === true),
        [profileSelection],
    );
    const hasAnyStats = useMemo(
        () => STATS_TEMPLATE_FIELDS.some(f => statsSelection[f.key] === true),
        [statsSelection],
    );
    const canPreview = (hasAnyProfile || hasAnyStats) && !previewing;

    const selectAllProfile = useCallback(() => {
        const next: SelectionMap<ProfileFieldKey> = {};
        for (const f of PROFILE_TEMPLATE_FIELDS) next[f.key] = true;
        setProfileSelection(next);
    }, []);
    const clearProfile = useCallback(() => setProfileSelection({}), []);
    const selectAllStats = useCallback(() => {
        const next: SelectionMap<StatsFieldKey> = {};
        for (const f of STATS_TEMPLATE_FIELDS) next[f.key] = true;
        setStatsSelection(next);
    }, []);
    const clearStats = useCallback(() => setStatsSelection({}), []);

    const onPreview = useCallback(async () => {
        if (!hasAnyProfile && !hasAnyStats) return;
        const selectionJSON = buildSelectionJSON(profileSelection, statsSelection);
        const opts = main.BuildTemplateV2ExportOptions.createFrom({
            name,
            description,
            author,
            tags: parseTags(tagsText),
        });
        setPreviewing(true);
        try {
            const result = await PreviewBuildTemplateV2FromCharacter(charIndex, selectionJSON, opts);
            setPendingSelectionJSON(selectionJSON);
            setPendingOpts(opts);
            setPreviewReport(result.report);
        } catch (err) {
            onError?.(err);
        } finally {
            setPreviewing(false);
        }
    }, [
        charIndex,
        hasAnyProfile,
        hasAnyStats,
        profileSelection,
        statsSelection,
        name,
        description,
        author,
        tagsText,
        onError,
    ]);

    const handleSaveFromPreview = useCallback(async () => {
        if (pendingSelectionJSON === null || pendingOpts === null) {
            onError?.(new Error('No preview available to save'));
            return;
        }
        setSavingToLibrary(true);
        try {
            const entry = await SaveBuildTemplateV2FromCharacterToLibrary(
                charIndex,
                pendingSelectionJSON,
                pendingOpts,
            );
            onSaved?.(entry);
            setPreviewReport(null);
            onClose();
        } catch (err) {
            onError?.(err);
        } finally {
            setSavingToLibrary(false);
        }
    }, [charIndex, pendingSelectionJSON, pendingOpts, onSaved, onClose, onError]);

    return (
        <div
            data-testid="create-template-v2-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Create Build Template"
            ref={dialogRef}
            tabIndex={-1}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
        >
            <div className="w-full max-w-2xl rounded-lg bg-card border border-border/60 shadow-xl flex flex-col max-h-[85vh]">
                <div className="px-4 py-3 border-b border-border/60">
                    <h2 className="text-sm font-black uppercase tracking-wider">Create Build Template</h2>
                    <p className="mt-1 text-[11px] text-muted-foreground">
                        Pick the profile and stat fields to capture from the current character. Schema v2 templates
                        carry only the fields you check — nothing else is exported.
                    </p>
                </div>

                <div className="px-4 py-3 space-y-4 overflow-y-auto text-[12px]">
                    <section aria-label="Metadata" className="space-y-2">
                        <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            Metadata
                        </h3>
                        <label className="block">
                            <span className="text-[10px] uppercase tracking-wider text-muted-foreground">Name</span>
                            <input
                                type="text"
                                data-testid="create-template-v2-name"
                                value={name}
                                onChange={e => setName(e.target.value)}
                                className="mt-0.5 w-full rounded border border-border/60 bg-background/40 px-2 py-1 text-[12px]"
                            />
                        </label>
                        <label className="block">
                            <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
                                Description
                            </span>
                            <input
                                type="text"
                                data-testid="create-template-v2-description"
                                value={description}
                                onChange={e => setDescription(e.target.value)}
                                className="mt-0.5 w-full rounded border border-border/60 bg-background/40 px-2 py-1 text-[12px]"
                            />
                        </label>
                        <label className="block">
                            <span className="text-[10px] uppercase tracking-wider text-muted-foreground">Author</span>
                            <input
                                type="text"
                                data-testid="create-template-v2-author"
                                value={author}
                                onChange={e => setAuthor(e.target.value)}
                                className="mt-0.5 w-full rounded border border-border/60 bg-background/40 px-2 py-1 text-[12px]"
                            />
                        </label>
                        <label className="block">
                            <span className="text-[10px] uppercase tracking-wider text-muted-foreground">Tags</span>
                            <input
                                type="text"
                                data-testid="create-template-v2-tags"
                                value={tagsText}
                                onChange={e => setTagsText(e.target.value)}
                                placeholder="comma-separated, e.g. pvp, rl150"
                                className="mt-0.5 w-full rounded border border-border/60 bg-background/40 px-2 py-1 text-[12px]"
                            />
                        </label>
                    </section>

                    <FieldSection
                        heading="Profile"
                        testidPrefix="create-template-v2-profile"
                        fields={PROFILE_TEMPLATE_FIELDS}
                        selection={profileSelection}
                        onToggle={(key, checked) =>
                            setProfileSelection(prev => ({ ...prev, [key]: checked }))
                        }
                        onSelectAll={selectAllProfile}
                        onClear={clearProfile}
                    />

                    <FieldSection
                        heading="Stats"
                        testidPrefix="create-template-v2-stats"
                        fields={STATS_TEMPLATE_FIELDS}
                        selection={statsSelection}
                        onToggle={(key, checked) =>
                            setStatsSelection(prev => ({ ...prev, [key]: checked }))
                        }
                        onSelectAll={selectAllStats}
                        onClear={clearStats}
                    />
                </div>

                <div className="px-4 py-3 border-t border-border/60 flex items-center justify-end gap-2">
                    <button
                        type="button"
                        data-testid="create-template-v2-cancel"
                        onClick={onClose}
                        disabled={previewing || savingToLibrary}
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40"
                    >
                        Cancel
                    </button>
                    <button
                        type="button"
                        data-testid="create-template-v2-preview"
                        onClick={onPreview}
                        disabled={!canPreview}
                        className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                            canPreview
                                ? 'bg-blue-700/80 text-white hover:bg-blue-700 shadow-sm'
                                : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                        }`}
                    >
                        {previewing ? 'Previewing…' : 'Preview'}
                    </button>
                </div>
            </div>

            {previewReport && (
                <ImportTemplatePreviewModal
                    report={previewReport}
                    onClose={() => setPreviewReport(null)}
                    onSaveToLibrary={handleSaveFromPreview}
                    savingToLibrary={savingToLibrary}
                />
            )}
        </div>
    );
}

interface FieldSectionProps<K extends string> {
    heading: string;
    testidPrefix: string;
    fields: ReadonlyArray<{ key: K; label: string }>;
    selection: SelectionMap<K>;
    onToggle: (key: K, checked: boolean) => void;
    onSelectAll: () => void;
    onClear: () => void;
}

function FieldSection<K extends string>({
    heading,
    testidPrefix,
    fields,
    selection,
    onToggle,
    onSelectAll,
    onClear,
}: FieldSectionProps<K>) {
    return (
        <section aria-label={heading} className="space-y-2">
            <div className="flex items-center justify-between">
                <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{heading}</h3>
                <div className="flex items-center gap-1">
                    <button
                        type="button"
                        data-testid={`${testidPrefix}-select-all`}
                        onClick={onSelectAll}
                        className="px-2 py-0.5 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                    >
                        Select all {heading.toLowerCase()}
                    </button>
                    <button
                        type="button"
                        data-testid={`${testidPrefix}-clear`}
                        onClick={onClear}
                        className="px-2 py-0.5 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all"
                    >
                        Clear {heading.toLowerCase()}
                    </button>
                </div>
            </div>
            <ul className="grid grid-cols-2 gap-x-3 gap-y-1">
                {fields.map(f => (
                    <li key={f.key} className="flex items-center gap-2">
                        <input
                            type="checkbox"
                            id={`${testidPrefix}-${f.key}`}
                            data-testid={`${testidPrefix}-${f.key}`}
                            checked={selection[f.key] === true}
                            onChange={e => onToggle(f.key, e.target.checked)}
                        />
                        <label htmlFor={`${testidPrefix}-${f.key}`} className="cursor-pointer select-none">
                            {f.label}
                        </label>
                    </li>
                ))}
            </ul>
        </section>
    );
}
