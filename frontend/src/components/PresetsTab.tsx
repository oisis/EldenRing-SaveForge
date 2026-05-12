export function PresetsTab() {
    const categories = ['Stats', 'Inventory', 'Storage', 'World', 'Weapons'] as const;

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

            {/* Status card — no built-in presets */}
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
                        Built-in presets (PvP builds, quick-apply configs, world-state profiles) are in preparation and will be available in a future update.
                    </p>
                </div>
            </div>

            {/* Categories — static badges */}
            <div className="shrink-0">
                <p className="text-[10px] font-black uppercase tracking-widest text-muted-foreground/60 mb-2">
                    Preset modules (coming soon)
                </p>
                <div className="flex flex-wrap gap-1.5">
                    {categories.map(cat => (
                        <span
                            key={cat}
                            className="px-2.5 py-0.5 rounded-full border border-border/40 bg-muted/20 text-[10px] font-black uppercase tracking-widest text-muted-foreground/40 cursor-default select-none"
                        >
                            {cat}
                        </span>
                    ))}
                </div>
            </div>

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
