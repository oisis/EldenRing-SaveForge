import {useEffect, useState, useCallback, useRef} from 'react';
import {GetDiagnosticLogTail} from '../../wailsjs/go/main/App';

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';
export type LogEntry = { time: string; level: LogLevel; message: string; loading?: boolean };

type ConsoleEntry = {
    id: string;
    time: string;
    level: LogLevel;
    source: string;
    event: string;
    message: string;
    details: string;
    loading?: boolean;
};

type DiagnosticTailField = { key: string; value: string };
type DiagnosticTailRecord = {
    seq: number;
    ts: string;
    level: LogLevel;
    source: string;
    event: string;
    message: string;
    fields?: DiagnosticTailField[];
};

let globalLogFn: ((level: LogLevel, message: string) => void) | null = null;
let globalLoadingFn: ((id: string, message: string) => void) | null = null;
let globalDoneFn: ((id: string) => void) | null = null;

export function sfLog(level: LogLevel, message: string) {
    globalLogFn?.(level, message);
}

export function sfLoading(id: string, message: string) {
    globalLoadingFn?.(id, message);
}

export function sfDone(id: string) {
    globalDoneFn?.(id);
}

function LoadingDots() {
    return (
        <span className="inline-flex ml-0.5">
            <span className="animate-dot-1">.</span>
            <span className="animate-dot-2">.</span>
            <span className="animate-dot-3">.</span>
        </span>
    );
}

interface ToastBarProps {
    sidebarWidth?: number;
}

const MIN_WIDTH = 400;
const MIN_HEIGHT = 150;
const journalRefreshMs = 1000;

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null;
}

function isLogLevel(value: unknown): value is LogLevel {
    return value === 'debug' || value === 'info' || value === 'warn' || value === 'error';
}

// The Go endpoint returns the journal's JSON representation as a primitive so
// generated models.ts stays untouched. Treat it as transport input, not as a
// trusted TypeScript object: malformed records are skipped rather than rendered.
function parseDiagnosticTail(encoded: string): ConsoleEntry[] {
    let parsed: unknown;
    try {
        parsed = JSON.parse(encoded);
    } catch {
        return [];
    }
    if (!Array.isArray(parsed)) return [];

    return parsed.flatMap((value): ConsoleEntry[] => {
        if (!isRecord(value)
            || typeof value.seq !== 'number'
            || typeof value.ts !== 'string'
            || !isLogLevel(value.level)
            || typeof value.source !== 'string'
            || typeof value.event !== 'string'
            || typeof value.message !== 'string') {
            return [];
        }
        const fields = Array.isArray(value.fields)
            ? value.fields.flatMap((field): DiagnosticTailField[] => (
                isRecord(field) && typeof field.key === 'string' && typeof field.value === 'string'
                    ? [{key: field.key, value: field.value}]
                    : []
            ))
            : [];
        const record: DiagnosticTailRecord = {
            seq: value.seq,
            ts: value.ts,
            level: value.level,
            source: value.source,
            event: value.event,
            message: value.message,
            fields,
        };
        return [{
            id: `journal-${record.seq}`,
            time: record.ts,
            level: record.level,
            source: record.source,
            event: record.event,
            message: record.message,
            details: record.fields?.map(field => `${field.key}=${field.value}`).join(' ') ?? '',
        }];
    });
}

function localConsoleEntries(logs: LogEntry[]): ConsoleEntry[] {
    return logs.map((entry, index) => ({
        id: `local-${index}-${entry.time}`,
        time: entry.time,
        level: entry.level,
        source: 'ui',
        event: 'ui_log',
        message: entry.message,
        details: '',
        loading: entry.loading,
    }));
}

function formatConsoleTime(time: string): string {
    const parsed = new Date(time);
    return Number.isNaN(parsed.getTime())
        ? time
        : parsed.toLocaleTimeString('en-GB', {hour12: false});
}

export function ToastBar({ sidebarWidth = 256 }: ToastBarProps) {
    const [logs, setLogs] = useState<LogEntry[]>([]);
    const [consoleOpen, setConsoleOpen] = useState(false);
    const [lastMessage, setLastMessage] = useState<LogEntry | null>(null);
    const [journalEntries, setJournalEntries] = useState<ConsoleEntry[]>([]);
    const [journalAvailable, setJournalAvailable] = useState(false);
    const [levelFilter, setLevelFilter] = useState<'all' | LogLevel>('all');
    const [search, setSearch] = useState('');

    // Console dimensions (persisted in localStorage)
    const [consoleWidth, setConsoleWidth] = useState<number>(() => {
        const saved = localStorage.getItem('console:width');
        return saved ? parseInt(saved) : 0; // 0 = auto (full width minus margins)
    });
    const [consoleHeight, setConsoleHeight] = useState<number>(() => {
        const saved = localStorage.getItem('console:height');
        return saved ? parseInt(saved) : Math.round(window.innerHeight * 0.45);
    });

    const consoleRef = useRef<HTMLDivElement>(null);
    const resizingRef = useRef<{ edge: 'top' | 'left' | 'right' | 'topleft' | 'topright'; startX: number; startY: number; startW: number; startH: number } | null>(null);
    const initRef = useRef(false);

    const addLog = useCallback((level: LogLevel, message: string) => {
        const entry: LogEntry = {
            time: new Date().toLocaleTimeString('en-GB', { hour12: false }),
            level,
            message,
        };
        setLogs(prev => [...prev, entry]);
        setLastMessage(entry);
    }, []);

    const startLoading = useCallback((id: string, message: string) => {
        const entry: LogEntry = {
            time: new Date().toLocaleTimeString('en-GB', { hour12: false }),
            level: 'info',
            message,
            loading: true,
        };
        setLogs(prev => {
            const idx = prev.findIndex(e => (e as any)._loadId === id);
            if (idx >= 0) {
                const next = [...prev];
                next[idx] = Object.assign(entry, { _loadId: id });
                return next;
            }
            return [...prev, Object.assign(entry, { _loadId: id })];
        });
        setLastMessage(entry);
    }, []);

    const finishLoading = useCallback((id: string) => {
        setLogs(prev => prev.map(e =>
            (e as any)._loadId === id ? { ...e, loading: false } : e
        ));
        setLastMessage(prev => prev && (prev as any)._loadId === id ? { ...prev, loading: false } : prev);
    }, []);

    useEffect(() => {
        globalLogFn = addLog;
        globalLoadingFn = startLoading;
        globalDoneFn = finishLoading;
        if (!initRef.current) {
            initRef.current = true;
            addLog('info', 'SaveForge session started');
        }
        return () => { globalLogFn = null; globalLoadingFn = null; globalDoneFn = null; };
    }, [addLog, startLoading, finishLoading]);

    // Read only the bounded, already-sanitized journal tail while the console
    // is visible. The disk JSONL remains the crash-safe source of truth; this
    // polling never writes to it and stops as soon as the console closes.
    useEffect(() => {
        if (!consoleOpen) return;
        let active = true;
        const refresh = async () => {
            try {
                const encoded = await GetDiagnosticLogTail();
                if (!active) return;
                setJournalEntries(parseDiagnosticTail(encoded));
                setJournalAvailable(true);
            } catch {
                if (active) setJournalAvailable(false);
            }
        };
        void refresh();
        const timer = window.setInterval(() => { void refresh(); }, journalRefreshMs);
        return () => {
            active = false;
            window.clearInterval(timer);
        };
    }, [consoleOpen]);

    // Keyboard toggle
    useEffect(() => {
        const handler = (e: KeyboardEvent) => {
            if (e.key === '`' && !e.ctrlKey && !e.metaKey) {
                const tag = (e.target as HTMLElement).tagName;
                if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
                e.preventDefault();
                setConsoleOpen(v => !v);
            }
        };
        window.addEventListener('keydown', handler);
        return () => window.removeEventListener('keydown', handler);
    }, []);

    // Persist dimensions
    useEffect(() => {
        if (consoleWidth > 0) localStorage.setItem('console:width', String(consoleWidth));
        localStorage.setItem('console:height', String(consoleHeight));
    }, [consoleWidth, consoleHeight]);

    // Resize handlers
    const startResize = useCallback((e: React.MouseEvent, edge: 'top' | 'left' | 'right' | 'topleft' | 'topright') => {
        e.preventDefault();
        e.stopPropagation();
        const rect = consoleRef.current?.getBoundingClientRect();
        if (!rect) return;
        resizingRef.current = {
            edge,
            startX: e.clientX,
            startY: e.clientY,
            startW: rect.width,
            startH: rect.height,
        };

        const onMove = (ev: MouseEvent) => {
            if (!resizingRef.current) return;
            const { edge, startX, startY, startW, startH } = resizingRef.current;
            const dx = ev.clientX - startX;
            const dy = ev.clientY - startY;
            const maxW = window.innerWidth - sidebarWidth * 2;
            const maxH = window.innerHeight - 60;

            if (edge === 'top' || edge === 'topleft' || edge === 'topright') {
                setConsoleHeight(Math.max(MIN_HEIGHT, Math.min(maxH, startH - dy)));
            }
            if (edge === 'left' || edge === 'topleft') {
                setConsoleWidth(Math.max(MIN_WIDTH, Math.min(maxW, startW - dx * 2))); // *2 because centered
            }
            if (edge === 'right' || edge === 'topright') {
                setConsoleWidth(Math.max(MIN_WIDTH, Math.min(maxW, startW + dx * 2)));
            }
        };

        const onUp = () => {
            resizingRef.current = null;
            window.removeEventListener('mousemove', onMove);
            window.removeEventListener('mouseup', onUp);
        };

        window.addEventListener('mousemove', onMove);
        window.addEventListener('mouseup', onUp);
    }, [sidebarWidth]);

    const levelColor = (l: LogLevel) => {
        switch (l) {
            case 'debug': return 'text-muted-foreground';
            case 'info': return 'text-info';
            case 'warn': return 'text-warning';
            case 'error': return 'text-destructive';
        }
    };

    const allConsoleEntries = journalAvailable ? journalEntries : localConsoleEntries(logs);
    const searchTerm = search.trim().toLocaleLowerCase();
    const visibleConsoleEntries = allConsoleEntries.filter(entry => {
        if (levelFilter !== 'all' && entry.level !== levelFilter) return false;
        if (searchTerm === '') return true;
        return [entry.level, entry.source, entry.event, entry.message, entry.details]
            .join(' ')
            .toLocaleLowerCase()
            .includes(searchTerm);
    });

    // Compute console positioning
    const availableWidth = typeof window !== 'undefined' ? window.innerWidth - sidebarWidth * 2 : 800;
    const effectiveWidth = consoleWidth > 0 ? Math.min(consoleWidth, availableWidth) : availableWidth;
    const horizontalOffset = consoleWidth > 0
        ? Math.max(sidebarWidth, (window.innerWidth - effectiveWidth) / 2)
        : sidebarWidth;

    return (
        <>
            {/* Toast Bar — only visible when console is closed */}
            {!consoleOpen && (
                <div
                    className="fixed bottom-0 left-1/2 -translate-x-1/2 z-40 cursor-pointer"
                    style={{ width: '30%', minWidth: '300px' }}
                    onClick={() => setConsoleOpen(true)}
                >
                    <div className="backdrop-blur-md border border-b-0 rounded-t-lg px-3 py-1.5 flex items-center gap-2"
                        style={{ background: 'var(--sf-toast-bg)', borderColor: 'var(--sf-console-border)' }}>
                        <div className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${lastMessage?.loading ? 'bg-warning animate-spin-slow' : 'bg-primary animate-pulse'}`} />
                        <span className="text-[11px] font-mono truncate flex-1" style={{ color: 'var(--sf-console-text-dim)' }}>
                            {lastMessage ? lastMessage.message : 'Ready'}
                            {lastMessage?.loading && <LoadingDots />}
                        </span>
                        <span className="text-[9px] flex-shrink-0" style={{ color: 'var(--sf-console-text-dim)', opacity: 0.5 }}>
                            ▲ `
                        </span>
                    </div>
                </div>
            )}

            {/* Quake Console — expanded */}
            {consoleOpen && (
                <div
                    ref={consoleRef}
                    className="fixed bottom-0 z-30 backdrop-blur-md rounded-t-lg flex flex-col"
                    style={{
                        left: `${horizontalOffset}px`,
                        right: `${horizontalOffset}px`,
                        width: consoleWidth > 0 ? `${effectiveWidth}px` : undefined,
                        height: `${consoleHeight}px`,
                        background: 'var(--sf-console-bg)',
                        border: '1px solid var(--sf-console-border)',
                        borderBottom: 'none',
                    }}
                >
                    {/* Resize handles */}
                    {/* Top edge */}
                    <div className="absolute -top-1 left-3 right-3 h-2 cursor-ns-resize z-10"
                        onMouseDown={e => startResize(e, 'top')} />
                    {/* Left edge */}
                    <div className="absolute top-3 -left-1 w-2 bottom-0 cursor-ew-resize z-10"
                        onMouseDown={e => startResize(e, 'left')} />
                    {/* Right edge */}
                    <div className="absolute top-3 -right-1 w-2 bottom-0 cursor-ew-resize z-10"
                        onMouseDown={e => startResize(e, 'right')} />
                    {/* Top-left corner */}
                    <div className="absolute -top-1 -left-1 w-4 h-4 cursor-nwse-resize z-20"
                        onMouseDown={e => startResize(e, 'topleft')} />
                    {/* Top-right corner */}
                    <div className="absolute -top-1 -right-1 w-4 h-4 cursor-nesw-resize z-20"
                        onMouseDown={e => startResize(e, 'topright')} />

                    {/* Header */}
                    <div className="flex items-center justify-between gap-3 px-3 py-1.5 shrink-0" style={{ borderBottom: '1px solid var(--sf-console-border)' }}>
                        <div className="min-w-0">
                            <span className="text-[9px] font-black uppercase tracking-[0.2em]" style={{ color: 'var(--sf-console-text-dim)' }}>Console</span>
                            <span className="ml-2 text-[8px] uppercase tracking-wider" style={{ color: 'var(--sf-console-text-dim)', opacity: 0.65 }}>
                                {journalAvailable ? 'durable session log' : 'live UI log'}
                            </span>
                        </div>
                        <div className="flex items-center gap-2">
                            <select
                                aria-label="Log level"
                                value={levelFilter}
                                onChange={event => setLevelFilter(event.target.value as 'all' | LogLevel)}
                                className="h-6 rounded border px-1.5 text-[9px] font-mono outline-none"
                                style={{ background: 'var(--sf-console-bg)', borderColor: 'var(--sf-console-border)', color: 'var(--sf-console-text)' }}
                            >
                                <option value="all">All levels</option>
                                <option value="debug">Debug</option>
                                <option value="info">Info</option>
                                <option value="warn">Warnings</option>
                                <option value="error">Errors</option>
                            </select>
                            <input
                                aria-label="Search logs"
                                type="search"
                                value={search}
                                onChange={event => setSearch(event.target.value)}
                                placeholder="Search"
                                className="h-6 w-28 rounded border px-2 text-[9px] font-mono outline-none placeholder:opacity-50"
                                style={{ background: 'var(--sf-console-bg)', borderColor: 'var(--sf-console-border)', color: 'var(--sf-console-text)' }}
                            />
                            {consoleWidth > 0 && (
                                <button
                                    onClick={() => setConsoleWidth(0)}
                                    className="text-[8px] font-bold uppercase tracking-widest transition-colors"
                                    style={{ color: 'var(--sf-console-text-dim)' }}
                                    title="Reset size"
                                >
                                    Reset
                                </button>
                            )}
                            <button
                                onClick={() => setConsoleOpen(false)}
                                className="transition-colors"
                                style={{ color: 'var(--sf-console-text-dim)' }}
                            >
                                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>
                    </div>

                    {/* Log body — newest entries on top. Filtering is UI-only:
                        it never alters the durable journal. */}
                    <div className="flex-1 overflow-y-auto custom-scrollbar px-3 py-2 font-mono text-[11px] space-y-0.5">
                        {visibleConsoleEntries.slice().reverse().map(entry => (
                            <div key={entry.id} className="flex gap-2">
                                <span className="flex-shrink-0" style={{ color: 'var(--sf-console-text-dim)', opacity: 0.7 }}>{formatConsoleTime(entry.time)}</span>
                                <span className={`uppercase font-bold flex-shrink-0 w-10 ${levelColor(entry.level)}`}>
                                    {entry.level}
                                </span>
                                <span className="min-w-0" style={{ color: 'var(--sf-console-text)' }}>
                                    <span className="mr-1" style={{ color: 'var(--sf-console-text-dim)', opacity: 0.75 }}>[{entry.source}/{entry.event}]</span>
                                    {entry.message}
                                    {entry.loading && <LoadingDots />}
                                    {entry.details !== '' && (
                                        <span className="ml-1 break-all" style={{ color: 'var(--sf-console-text-dim)', opacity: 0.8 }}>{entry.details}</span>
                                    )}
                                </span>
                            </div>
                        ))}
                        {visibleConsoleEntries.length === 0 && (
                            <div className="text-center py-8" style={{ color: 'var(--sf-console-text-dim)', opacity: 0.3 }}>
                                {allConsoleEntries.length === 0 ? 'No log entries' : 'No matching log entries'}
                            </div>
                        )}
                    </div>
                </div>
            )}
        </>
    );
}
