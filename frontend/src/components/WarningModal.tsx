import {useEffect} from 'react';
import {createPortal} from 'react-dom';

interface Props {
    title: string;
    children: React.ReactNode;
    onClose: () => void;
}

export function WarningModal({title, children, onClose}: Props) {
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, [onClose]);

    return createPortal(
        <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm animate-in fade-in duration-150"
            onClick={onClose}
        >
            <div
                className="relative bg-card border border-border rounded-xl shadow-2xl p-6 max-w-md w-full mx-4 animate-in zoom-in-95 duration-200"
                onClick={e => e.stopPropagation()}
            >
                {/* Header */}
                <div className="flex items-start gap-3 mb-4">
                    <div className="w-9 h-9 rounded-lg bg-yellow-500/10 border border-yellow-500/30 flex items-center justify-center shrink-0">
                        <svg className="w-5 h-5 text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
                                d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                        </svg>
                    </div>
                    <div className="flex-1">
                        <h3 className="text-[13px] font-black uppercase tracking-[0.12em] text-foreground">{title}</h3>
                    </div>
                    <button
                        onClick={onClose}
                        className="text-muted-foreground/50 hover:text-foreground transition-colors p-1"
                    >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                {/* Body */}
                <div className="text-[11px] text-muted-foreground leading-relaxed space-y-2 mb-5">
                    {children}
                </div>

                {/* Footer */}
                <div className="flex justify-end gap-2">
                    <button
                        onClick={onClose}
                        className="px-5 py-2 text-[10px] font-black uppercase tracking-[0.15em] bg-foreground text-background hover:opacity-90 active:scale-[0.98] transition-all rounded-md"
                    >
                        Understood
                    </button>
                </div>

                <p className="mt-3 text-[10px] uppercase tracking-widest text-muted-foreground/70 text-center">
                    Press Esc to close
                </p>
            </div>
        </div>,
        document.body
    );
}
