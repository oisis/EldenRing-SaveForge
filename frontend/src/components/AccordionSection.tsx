import {useState, ReactNode, useEffect, useRef} from 'react';

interface AccordionSectionProps {
    id?: string;
    title: string;
    defaultOpen?: boolean;
    badge?: string | number;
    progress?: { current: number; total: number };
    summary?: ReactNode;
    actions?: ReactNode;
    headerRight?: ReactNode;
    children: ReactNode;
    className?: string;
    titleClassName?: string;
    // When defined, section state persists in sessionStorage (survives tab switches /
    // remounts) but resets to defaultOpen whenever this value changes — used to wipe
    // expansion when a different save file is loaded.
    resetSignal?: number | string;
}

export function AccordionSection({
    id,
    title,
    defaultOpen = false,
    badge,
    progress,
    summary,
    actions,
    headerRight,
    children,
    className = '',
    titleClassName,
    resetSignal,
}: AccordionSectionProps) {
    const useLocal = id !== undefined && resetSignal === undefined;
    const useSession = id !== undefined && resetSignal !== undefined;
    const localKey = useLocal ? `accordion:${id}` : null;
    const sessionKey = useSession ? `accordion:${id}` : null;

    const [open, setOpen] = useState(() => {
        if (localKey) {
            const saved = localStorage.getItem(localKey);
            if (saved !== null) return saved === '1';
        }
        if (sessionKey) {
            const saved = sessionStorage.getItem(sessionKey);
            if (saved !== null) return saved === '1';
        }
        return defaultOpen;
    });

    useEffect(() => {
        if (localKey) localStorage.setItem(localKey, open ? '1' : '0');
        else if (sessionKey) sessionStorage.setItem(sessionKey, open ? '1' : '0');
    }, [open, localKey, sessionKey]);

    // Track last seen resetSignal — only collapse when the value actually changes.
    // Using a ref-based equality guard (instead of "first run" boolean) so StrictMode's
    // double-invoked effects in dev don't accidentally trigger a reset on remount.
    const lastResetSignalRef = useRef(resetSignal);
    useEffect(() => {
        if (resetSignal === undefined) return;
        if (lastResetSignalRef.current === resetSignal) return;
        lastResetSignalRef.current = resetSignal;
        setOpen(defaultOpen);
        if (sessionKey) sessionStorage.removeItem(sessionKey);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [resetSignal]);

    const pct = progress ? Math.round((progress.current / Math.max(progress.total, 1)) * 100) : null;

    return (
        <div className={`border border-border rounded-lg overflow-hidden ${className}`}>
            {/* Header */}
            <button
                onClick={() => setOpen(v => !v)}
                className="w-full flex items-center gap-2 px-3 py-2 bg-muted/10 hover:bg-muted/20 transition-all text-left"
            >
                <svg
                    className={`w-3 h-3 text-muted-foreground transition-transform duration-200 flex-shrink-0 ${open ? 'rotate-90' : ''}`}
                    fill="none" stroke="currentColor" viewBox="0 0 24 24"
                >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 5l7 7-7 7" />
                </svg>

                <span className={`text-[10px] font-black uppercase tracking-[0.15em] flex-shrink-0 ${titleClassName ?? 'text-foreground/80'}`}>
                    {title}
                </span>

                {badge !== undefined && (
                    <span className="text-[11px] font-bold bg-primary/10 text-primary px-1.5 py-0.5 rounded-full flex-shrink-0">
                        {badge}
                    </span>
                )}

                {!open && pct !== null && (
                    <div className="flex items-center gap-2 flex-1 min-w-0 ml-2">
                        <div className="flex-1 h-1.5 bg-border rounded-full overflow-hidden">
                            <div
                                className="h-full bg-primary rounded-full transition-all duration-300"
                                style={{ width: `${pct}%` }}
                            />
                        </div>
                        {actions && (
                            <div className="flex items-center gap-1 flex-shrink-0" onClick={e => e.stopPropagation()}>
                                {actions}
                            </div>
                        )}
                        <span className="text-[11px] font-mono text-muted-foreground flex-shrink-0">
                            {progress!.current}/{progress!.total}
                        </span>
                    </div>
                )}

                {!open && summary && !progress && (
                    <div className="flex items-center justify-center flex-1 min-w-0 ml-2 truncate">
                        {typeof summary === 'string'
                            ? <span className="text-[11px] text-muted-foreground font-medium truncate">{summary}</span>
                            : summary
                        }
                    </div>
                )}

                {open && <div className="flex-1" />}

                {open && actions && (
                    <div className="flex items-center gap-1 flex-shrink-0" onClick={e => e.stopPropagation()}>
                        {actions}
                    </div>
                )}

                {headerRight && (
                    <div className="flex items-center flex-shrink-0 ml-auto" onClick={e => e.stopPropagation()}>
                        {headerRight}
                    </div>
                )}
            </button>

            {open && (
                <div className="px-3 py-2 border-t border-border/50">
                    {children}
                </div>
            )}
        </div>
    );
}
