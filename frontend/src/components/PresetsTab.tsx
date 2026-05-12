import {useState, useEffect} from 'react';
import {ListBuiltinCharacterPresets, GetBuiltinCharacterPreset, GetCharacter} from '../../wailsjs/go/main/App';
import {main, vm} from '../../wailsjs/go/models';

// ─── Types ────────────────────────────────────────────────────────────────────

interface PreviewData {
    preset: vm.CharacterPreset;
    char: vm.CharacterViewModel;
}

// ─── Stat field definitions ───────────────────────────────────────────────────

const STAT_FIELDS: {key: keyof vm.CharacterPresetCore & keyof vm.CharacterViewModel; label: string}[] = [
    {key: 'level',        label: 'Level'},
    {key: 'souls',        label: 'Runes'},
    {key: 'vigor',        label: 'Vigor'},
    {key: 'mind',         label: 'Mind'},
    {key: 'endurance',    label: 'Endurance'},
    {key: 'strength',     label: 'Strength'},
    {key: 'dexterity',    label: 'Dexterity'},
    {key: 'intelligence', label: 'Intelligence'},
    {key: 'faith',        label: 'Faith'},
    {key: 'arcane',       label: 'Arcane'},
];

// ─── Sub-components ───────────────────────────────────────────────────────────

const MODULE_COLORS: Record<string, string> = {
    Stats:     'text-blue-400 border-blue-500/40 bg-blue-500/10',
    Inventory: 'text-green-400 border-green-500/40 bg-green-500/10',
    Storage:   'text-purple-400 border-purple-500/40 bg-purple-500/10',
    World:     'text-amber-400 border-amber-500/40 bg-amber-500/10',
    Weapons:   'text-red-400 border-red-500/40 bg-red-500/10',
};

function ModuleBadge({module}: {module: string}) {
    const cls = MODULE_COLORS[module] ?? 'text-muted-foreground border-border/40 bg-muted/20';
    return (
        <span className={`px-2 py-0.5 rounded-full border text-[9px] font-black uppercase tracking-widest ${cls}`}>
            {module}
        </span>
    );
}

function DiffRow({label, current, preset}: {label: string; current: number; preset: number}) {
    const diff = preset - current;
    const dir = diff > 0 ? 'up' : diff < 0 ? 'down' : 'same';
    const presetColor = dir === 'up'
        ? 'text-green-400 font-bold'
        : dir === 'down'
            ? 'text-red-400 font-bold'
            : 'text-muted-foreground/60';
    const deltaColor = dir === 'up'
        ? 'text-green-500/70'
        : dir === 'down'
            ? 'text-red-500/70'
            : 'text-muted-foreground/30';
    const deltaText = dir === 'up' ? `+${diff}` : dir === 'down' ? `${diff}` : '=';

    return (
        <div className="grid grid-cols-[5.5rem_1fr_0.75rem_1fr_2.5rem] items-center gap-1 py-0.5">
            <span className="text-[10px] text-muted-foreground/70">{label}</span>
            <span className="text-[10px] tabular-nums text-right text-foreground/60">
                {current.toLocaleString()}
            </span>
            <span className="text-[9px] text-muted-foreground/30 text-center">→</span>
            <span className={`text-[10px] tabular-nums text-right ${presetColor}`}>
                {preset.toLocaleString()}
            </span>
            <span className={`text-[9px] tabular-nums text-right ${deltaColor}`}>
                {deltaText}
            </span>
        </div>
    );
}

function StatPreviewPanel({data}: {data: PreviewData}) {
    const c = data.char;
    const p = data.preset.character;

    return (
        <div className="mt-2 pt-3 border-t border-border/30 flex flex-col gap-2">
            {/* Notice */}
            <div className="flex items-center gap-1.5">
                <svg className="w-3 h-3 text-blue-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
                        d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <p className="text-[9px] font-black uppercase tracking-widest text-blue-400/80">
                    Preview only — no changes have been applied.
                </p>
            </div>

            {/* Class row */}
            <div className="flex items-center gap-2 pb-1.5 border-b border-border/20">
                <span className="text-[9px] text-muted-foreground/50 uppercase tracking-widest w-[5.5rem]">Class</span>
                <span className="text-[10px] text-foreground/60">{c.className}</span>
                <span className="text-[9px] text-muted-foreground/30">→</span>
                <span className={`text-[10px] font-bold ${p.className !== c.className ? 'text-amber-400' : 'text-muted-foreground/60'}`}>
                    {p.className}
                </span>
            </div>

            {/* Stat diff rows */}
            <div className="flex flex-col">
                <div className="grid grid-cols-[5.5rem_1fr_0.75rem_1fr_2.5rem] gap-1 pb-1 mb-0.5 border-b border-border/20">
                    <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground/40">Stat</span>
                    <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground/40 text-right">Current</span>
                    <span />
                    <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground/40 text-right">Preset</span>
                    <span className="text-[9px] font-black uppercase tracking-widest text-muted-foreground/40 text-right">Δ</span>
                </div>
                {STAT_FIELDS.map(({key, label}) => (
                    <DiffRow
                        key={key}
                        label={label}
                        current={c[key] as number}
                        preset={p[key] as number}
                    />
                ))}
            </div>
        </div>
    );
}

function PresetCard({
    preset,
    isExpanded,
    loading,
    error,
    previewData,
    onPreview,
}: {
    preset: main.BuiltinCharacterPresetInfo;
    isExpanded: boolean;
    loading: boolean;
    error: string | null;
    previewData: PreviewData | null;
    onPreview: () => void;
}) {
    return (
        <div className={`bg-muted/20 border rounded-lg px-4 py-3 flex flex-col gap-2.5 transition-colors ${
            isExpanded ? 'border-border/80' : 'border-border/50'
        }`}>
            {/* Header row */}
            <div className="flex items-start justify-between gap-2">
                <div className="flex-1 min-w-0">
                    <p className="text-[11px] font-black uppercase tracking-[0.12em] text-foreground/90 truncate">
                        {preset.name}
                    </p>
                    <p className="text-[10px] text-muted-foreground/70 mt-0.5">
                        {preset.className} · Level {preset.level}
                    </p>
                </div>
                <span className="shrink-0 px-2 py-0.5 rounded bg-muted/40 border border-border/40 text-[9px] font-black text-muted-foreground/60 tabular-nums">
                    RL {preset.level}
                </span>
            </div>

            {/* Description */}
            <p className="text-[10px] text-muted-foreground leading-relaxed">
                {preset.description}
            </p>

            {/* Module badges + tags */}
            <div className="flex flex-wrap items-center gap-1.5">
                {preset.modules.map(m => (
                    <ModuleBadge key={m} module={m} />
                ))}
                {preset.tags.map(tag => (
                    <span
                        key={tag}
                        className="px-2 py-0.5 rounded-full border border-border/30 bg-transparent text-[9px] text-muted-foreground/40 font-mono"
                    >
                        #{tag}
                    </span>
                ))}
            </div>

            {/* Action buttons */}
            <div className="flex gap-2 pt-1 border-t border-border/30">
                <button
                    onClick={onPreview}
                    disabled={loading}
                    className={`flex-1 py-1.5 rounded border text-[10px] font-black uppercase tracking-widest transition-all ${
                        isExpanded
                            ? 'border-border/60 bg-muted/30 text-foreground/70 hover:bg-muted/40'
                            : 'border-border/40 bg-muted/10 text-muted-foreground/60 hover:border-border/60 hover:text-foreground/70 hover:bg-muted/20'
                    } ${loading ? 'cursor-wait opacity-60' : 'cursor-pointer'}`}
                >
                    {loading ? 'Loading…' : isExpanded ? 'Hide Preview ▲' : 'Preview ▼'}
                </button>
                <button
                    disabled
                    title="Apply coming in a future update"
                    className="flex-1 py-1.5 rounded border border-border/30 text-[10px] font-black uppercase tracking-widest text-muted-foreground/30 bg-muted/10 cursor-not-allowed"
                >
                    Apply
                </button>
            </div>

            {/* Inline preview panel */}
            {isExpanded && !loading && (
                <>
                    {error && (
                        <div className="mt-1 px-3 py-2 rounded border border-red-500/30 bg-red-500/10 text-[10px] text-red-400 leading-relaxed">
                            {error}
                        </div>
                    )}
                    {previewData && <StatPreviewPanel data={previewData} />}
                </>
            )}
        </div>
    );
}

function EmptyState() {
    return (
        <div className="bg-muted/20 border border-border/50 rounded-lg px-4 py-5 flex flex-col items-center gap-3 text-center shrink-0">
            <div className="w-9 h-9 rounded-full bg-muted/40 border border-border/50 flex items-center justify-center">
                <svg className="w-4 h-4 text-muted-foreground/50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5"
                        d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
            </div>
            <div>
                <p className="text-[11px] font-black uppercase tracking-[0.15em] text-foreground/50">
                    No built-in presets loaded yet
                </p>
                <p className="text-[10px] text-muted-foreground mt-1 max-w-xs leading-relaxed">
                    Built-in presets (PvP builds, quick-apply configs, world-state profiles) are in preparation.
                </p>
            </div>
        </div>
    );
}

// ─── Main component ────────────────────────────────────────────────────────────

interface PresetsTabProps {
    charIdx: number;
}

export function PresetsTab({charIdx}: PresetsTabProps) {
    const [presets, setPresets] = useState<main.BuiltinCharacterPresetInfo[]>([]);
    const [previewId, setPreviewId] = useState<string | null>(null);
    const [previewData, setPreviewData] = useState<PreviewData | null>(null);
    const [previewLoading, setPreviewLoading] = useState(false);
    const [previewError, setPreviewError] = useState<string | null>(null);

    useEffect(() => {
        ListBuiltinCharacterPresets().then(setPresets).catch(() => setPresets([]));
    }, []);

    async function handlePreview(id: string) {
        if (previewId === id) {
            setPreviewId(null);
            setPreviewData(null);
            setPreviewError(null);
            return;
        }
        setPreviewId(id);
        setPreviewData(null);
        setPreviewError(null);
        setPreviewLoading(true);
        try {
            const [preset, char] = await Promise.all([
                GetBuiltinCharacterPreset(id),
                GetCharacter(charIdx),
            ]);
            if (!char) {
                setPreviewError('No character loaded. Open a save file and select a character first.');
            } else {
                setPreviewData({preset, char});
            }
        } catch (e) {
            setPreviewError(String(e));
        } finally {
            setPreviewLoading(false);
        }
    }

    return (
        <div className="flex-1 overflow-y-auto custom-scrollbar pr-2 flex flex-col gap-4">

            {/* Header */}
            <div className="shrink-0">
                <p className="text-[11px] font-black uppercase tracking-[0.15em] text-foreground/80">
                    Character Presets
                </p>
                <p className="text-[10px] text-muted-foreground mt-1 leading-relaxed max-w-sm">
                    Apply complete or partial character configurations — stats, inventory, storage, world state or weapon setups — in a single step.
                </p>
            </div>

            {/* Backup warning */}
            <div className="px-3 py-2 rounded border-l-2 flex items-start gap-3 bg-yellow-500/10 border-yellow-500/40 text-yellow-200 shrink-0">
                <span className="text-base leading-none text-yellow-400">⚠</span>
                <p className="text-[10px] leading-relaxed flex-1">
                    <strong className="font-black uppercase tracking-widest">Backup first.</strong>{' '}
                    <span className="text-muted-foreground">Always export or backup your save before applying presets. Preset apply cannot always be undone.</span>
                </p>
            </div>

            {/* Preset list or empty state */}
            {presets.length === 0 ? (
                <EmptyState />
            ) : (
                <div className="flex flex-col gap-3 shrink-0">
                    {presets.map(p => (
                        <PresetCard
                            key={p.id}
                            preset={p}
                            isExpanded={previewId === p.id}
                            loading={previewLoading && previewId === p.id}
                            error={previewId === p.id ? previewError : null}
                            previewData={previewId === p.id ? previewData : null}
                            onPreview={() => handlePreview(p.id)}
                        />
                    ))}
                </div>
            )}

            {/* Tools hint */}
            <div className="bg-muted/20 border border-border/40 rounded-lg px-4 py-3 flex items-start gap-3 shrink-0">
                <svg className="w-4 h-4 text-muted-foreground/50 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5"
                        d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div>
                    <p className="text-[10px] font-black uppercase tracking-widest text-foreground/60">
                        Import preset from file or URL
                    </p>
                    <p className="text-[10px] text-muted-foreground mt-0.5 leading-relaxed">
                        Preset import is already available.{' '}
                        <span className="text-foreground/60 font-bold">Go to Tools → Preset Importer</span>{' '}
                        to load a <code className="font-mono text-[9px] bg-muted/40 px-1 py-0.5 rounded">.sfpreset</code> file or a URL.
                    </p>
                </div>
            </div>

        </div>
    );
}
