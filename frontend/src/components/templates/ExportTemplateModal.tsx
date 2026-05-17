import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { ExportBuildTemplateToFile } from '../../../wailsjs/go/main/App';
import { main, templates } from '../../../wailsjs/go/models';

// ExportTemplateModal is the user-facing form for exporting an active
// Inventory Workspace session as a saveforge.build-template JSON file.
//
// Why this modal exists rather than just three "Export X" toolbar
// buttons: the metadata fields (name/description/tags) are most
// useful when the user has just curated a build and wants to label it
// before writing the file. Putting them on the path of the export keeps
// the labels in one place and saves the user a separate edit step on a
// generated file.
//
// State ownership: dirty / sessionID come from the parent SortOrderTab.
// This component does NOT mutate workspace state; it only reads the
// dirty flag to surface the "uses unsaved workspace state" note.

interface Props {
    sessionID: string;
    dirty: boolean;
    initialIncludeInventory?: boolean;
    initialIncludeStorage?: boolean;
    onClose: () => void;
    onSuccess: (result: main.BuildTemplateExportResult) => void;
    onError: (err: unknown) => void;
}

export function ExportTemplateModal({
    sessionID,
    dirty,
    initialIncludeInventory = true,
    initialIncludeStorage = true,
    onClose,
    onSuccess,
    onError,
}: Props) {
    const nameId = useId();
    const descId = useId();
    const tagsId = useId();
    const authorId = useId();

    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [author, setAuthor] = useState('');
    const [tagsInput, setTagsInput] = useState('');
    const [includeInventory, setIncludeInventory] = useState(initialIncludeInventory);
    const [includeStorage, setIncludeStorage] = useState(initialIncludeStorage);
    const [submitting, setSubmitting] = useState(false);

    const dialogRef = useRef<HTMLDivElement | null>(null);
    useEffect(() => {
        dialogRef.current?.focus();
    }, []);

    const canSubmit = useMemo(
        () => sessionID !== '' && (includeInventory || includeStorage) && !submitting,
        [sessionID, includeInventory, includeStorage, submitting],
    );

    const tags = useMemo(
        () =>
            tagsInput
                .split(',')
                .map(t => t.trim())
                .filter(t => t.length > 0),
        [tagsInput],
    );

    const onSubmit = async () => {
        if (!canSubmit) return;
        setSubmitting(true);
        try {
            const opts = main.BuildTemplateExportOptions.createFrom({
                includeInventory,
                includeStorage,
                name,
                description,
                author,
                tags,
            });
            const result = await ExportBuildTemplateToFile(sessionID, opts);
            onSuccess(result);
        } catch (err) {
            onError(err);
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <div
            data-testid="export-template-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Export Build Template"
            ref={dialogRef}
            tabIndex={-1}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
        >
            <div className="w-full max-w-md rounded-lg bg-card border border-border/60 shadow-xl">
                <div className="px-4 py-3 border-b border-border/60">
                    <h2 className="text-sm font-black uppercase tracking-wider">Export Build Template</h2>
                    <p className="mt-1 text-[11px] text-muted-foreground">
                        Writes a portable <code>saveforge.build-template</code> JSON file. Save-local handles are never included.
                    </p>
                </div>

                <div className="px-4 py-3 space-y-3 text-[12px]">
                    {dirty && (
                        <div
                            data-testid="export-dirty-note"
                            className="rounded border border-amber-500/40 bg-amber-500/10 px-2 py-1 text-amber-300"
                        >
                            This export uses the current unsaved workspace state.
                        </div>
                    )}

                    <fieldset className="space-y-2">
                        <legend className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            Sections
                        </legend>
                        <label className="flex items-center gap-2">
                            <input
                                type="checkbox"
                                checked={includeInventory}
                                onChange={e => setIncludeInventory(e.target.checked)}
                                aria-label="Include inventory"
                            />
                            <span>Include inventory items</span>
                        </label>
                        <label className="flex items-center gap-2">
                            <input
                                type="checkbox"
                                checked={includeStorage}
                                onChange={e => setIncludeStorage(e.target.checked)}
                                aria-label="Include storage"
                            />
                            <span>Include storage items</span>
                        </label>
                    </fieldset>

                    <div className="space-y-1">
                        <label htmlFor={nameId} className="block text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            Name
                        </label>
                        <input
                            id={nameId}
                            type="text"
                            value={name}
                            onChange={e => setName(e.target.value)}
                            placeholder="e.g. RL150 Quality Greatsword"
                            className="w-full rounded border border-border/60 bg-background/60 px-2 py-1 text-foreground"
                        />
                    </div>

                    <div className="space-y-1">
                        <label htmlFor={descId} className="block text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            Description
                        </label>
                        <textarea
                            id={descId}
                            value={description}
                            onChange={e => setDescription(e.target.value)}
                            rows={2}
                            className="w-full rounded border border-border/60 bg-background/60 px-2 py-1 text-foreground"
                        />
                    </div>

                    <div className="space-y-1">
                        <label htmlFor={authorId} className="block text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            Author
                        </label>
                        <input
                            id={authorId}
                            type="text"
                            value={author}
                            onChange={e => setAuthor(e.target.value)}
                            className="w-full rounded border border-border/60 bg-background/60 px-2 py-1 text-foreground"
                        />
                    </div>

                    <div className="space-y-1">
                        <label htmlFor={tagsId} className="block text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            Tags (comma-separated)
                        </label>
                        <input
                            id={tagsId}
                            type="text"
                            value={tagsInput}
                            onChange={e => setTagsInput(e.target.value)}
                            placeholder="pvp, rl150, quality"
                            className="w-full rounded border border-border/60 bg-background/60 px-2 py-1 text-foreground"
                        />
                    </div>
                </div>

                <div className="px-4 py-3 border-t border-border/60 flex items-center justify-end gap-2">
                    <button
                        type="button"
                        onClick={onClose}
                        disabled={submitting}
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40"
                    >
                        Cancel
                    </button>
                    <button
                        type="button"
                        onClick={onSubmit}
                        disabled={!canSubmit}
                        className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                            canSubmit
                                ? 'bg-green-700/80 text-white hover:bg-green-700 shadow-sm'
                                : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                        }`}
                    >
                        {submitting ? 'Exporting…' : 'Export JSON file'}
                    </button>
                </div>
            </div>
        </div>
    );
}

// formatWarnings is a small helper used by SortOrderTab's toast surface
// and by tests. Kept outside the component so it can be unit-tested
// without rendering.
export function formatWarningsSummary(warnings: templates.ExportWarning[] | undefined): string | null {
    if (!warnings || warnings.length === 0) return null;
    if (warnings.length === 1) {
        return `Export completed with 1 warning: ${warnings[0].message}`;
    }
    return `Export completed with ${warnings.length} warnings.`;
}
