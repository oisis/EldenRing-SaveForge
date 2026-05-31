import { useCallback, useEffect, useRef, useState } from 'react';

// ImportTemplateFromURLModal — Phase 9 of spec/56-templates-v2.md.
// Small dedicated modal that takes a single https:// URL, runs the
// caller-provided onPreview callback, and surfaces backend rejection
// errors inline without clearing the user's input. The parent
// (TemplatesShellModal) wires onPreview to the Wails binding
// PreviewBuildTemplateImportYAMLFromURL and is responsible for
// downstream state (opening the ImportTemplatePreviewModal on
// success).
//
// This component owns:
//   - the input field state,
//   - in-flight / disabled state,
//   - inline error rendering,
//   - cancel handling.
//
// It does NOT own:
//   - the Wails call itself (parent owns),
//   - the preview modal lifecycle (parent owns),
//   - any toast (parent owns).

interface Props {
    onPreview: (url: string) => Promise<{ ok: true } | { ok: false; error: string }>;
    onCancel: () => void;
}

export function ImportTemplateFromURLModal({ onPreview, onCancel }: Props) {
    const dialogRef = useRef<HTMLDivElement | null>(null);
    const inputRef = useRef<HTMLInputElement | null>(null);
    const [url, setURL] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string>('');

    useEffect(() => {
        // Autofocus the input so user can immediately paste.
        inputRef.current?.focus();
    }, []);

    // Light client-side validation — the backend is the source of
    // truth, but this keeps the Preview button honest in the obvious
    // empty-input case.
    const trimmed = url.trim();
    const looksLikeHTTPS = /^https?:\/\//i.test(trimmed);
    const canPreview = !loading && trimmed !== '' && looksLikeHTTPS;

    const handlePreview = useCallback(async () => {
        if (!canPreview) return;
        setLoading(true);
        setError('');
        try {
            const result = await onPreview(trimmed);
            if (!result.ok) {
                setError(result.error);
                // Keep input populated so the user can edit and retry.
            }
            // On success the parent closes us via its own state — we
            // don't auto-close here, the parent decides.
        } catch (err) {
            setError(String(err));
        } finally {
            setLoading(false);
        }
    }, [canPreview, trimmed, onPreview]);

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter' && canPreview) {
            e.preventDefault();
            void handlePreview();
        }
    };

    return (
        <div
            data-testid="import-url-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Import Build Template from URL"
            ref={dialogRef}
            tabIndex={-1}
            className="fixed inset-0 z-[55] flex items-center justify-center bg-black/60 p-4"
        >
            <div className="w-full max-w-lg rounded-lg bg-card border border-border/60 shadow-xl flex flex-col">
                <div className="px-4 py-3 border-b border-border/60">
                    <h2 className="text-sm font-black uppercase tracking-wider">
                        Import Build Template from URL
                    </h2>
                    <p className="mt-1 text-[11px] text-muted-foreground">
                        Paste an <code>https://</code> URL pointing to a public Build Template YAML.
                        The backend fetches it under strict SSRF guards and shows a preview before
                        anything is saved or applied.
                    </p>
                </div>

                <div className="px-4 py-3 space-y-3 text-[12px]">
                    <label className="block">
                        <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
                            Template URL
                        </span>
                        <input
                            ref={inputRef}
                            type="url"
                            data-testid="import-url-input"
                            value={url}
                            onChange={e => setURL(e.target.value)}
                            onKeyDown={handleKeyDown}
                            disabled={loading}
                            placeholder="https://example.com/template.yaml"
                            aria-invalid={error !== ''}
                            aria-describedby={error ? 'import-url-error' : undefined}
                            className={`mt-0.5 w-full rounded border px-2 py-1 text-[12px] bg-background/40 ${
                                error ? 'border-red-500/60' : 'border-border/60'
                            } disabled:opacity-40`}
                        />
                    </label>
                    {error && (
                        <div
                            id="import-url-error"
                            data-testid="import-url-error"
                            role="alert"
                            className="rounded border border-red-500/40 bg-red-500/10 px-2 py-1 text-[11px] text-red-200 break-all"
                        >
                            {error}
                        </div>
                    )}
                    <p className="text-[10px] text-muted-foreground">
                        Only <code>https://</code> URLs are accepted. The backend rejects loopback,
                        private, link-local, and metadata addresses; bodies above 1 MiB; and
                        unsupported Content-Type headers.
                    </p>
                </div>

                <div className="px-4 py-3 border-t border-border/60 flex items-center justify-end gap-2">
                    <button
                        type="button"
                        data-testid="import-url-cancel"
                        onClick={onCancel}
                        disabled={loading}
                        className="px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-all disabled:opacity-40"
                    >
                        Cancel
                    </button>
                    <button
                        type="button"
                        data-testid="import-url-preview"
                        onClick={handlePreview}
                        disabled={!canPreview}
                        title={
                            trimmed === ''
                                ? 'Enter a URL to preview.'
                                : !looksLikeHTTPS
                                  ? 'URL must start with https://'
                                  : loading
                                    ? 'Fetching…'
                                    : 'Fetch and preview this URL.'
                        }
                        className={`px-3 py-1 text-[10px] font-black uppercase tracking-wider rounded transition-all ${
                            canPreview
                                ? 'bg-blue-700/80 text-white hover:bg-blue-700 shadow-sm'
                                : 'opacity-40 cursor-not-allowed bg-muted/20 text-muted-foreground'
                        }`}
                    >
                        {loading ? 'Fetching…' : 'Preview'}
                    </button>
                </div>
            </div>
        </div>
    );
}
