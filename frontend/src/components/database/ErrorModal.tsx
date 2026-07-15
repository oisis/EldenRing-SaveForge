// Presentational error modal — capacity / container cap / add failure /
// repair failure. All state, the decision paths and the mutations stay in
// DatabaseTab; this component only renders the error data and reports the
// close intent via onClose.
interface ErrorModalProps {
    title: string;
    message: string;
    onClose: () => void;
    // Optional secondary action (e.g. GaItem optimization). When absent the
    // modal renders exactly as before — OK only.
    cta?: { label: string; onClick: () => void };
}

export function ErrorModal({ title, message, onClose, cta }: ErrorModalProps) {
    return (
        <div className="fixed inset-0 z-[130] flex items-center justify-center bg-background/80 backdrop-blur-sm animate-in fade-in duration-300">
            <div className="bg-card p-8 rounded-2xl border-2 border-red-500/40 flex flex-col space-y-5 max-w-md w-full mx-4 shadow-2xl shadow-red-500/20 animate-in zoom-in-95 duration-300">
                <div className="flex items-center space-x-3">
                    <div className="w-10 h-10 rounded-full bg-red-500/15 border border-red-500/40 flex items-center justify-center">
                        <svg className="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </div>
                    <h3 className="text-sm font-black uppercase tracking-[0.15em] text-red-500">{title}</h3>
                </div>
                <p className="text-[11px] text-foreground leading-relaxed whitespace-pre-line">{message}</p>
                {cta && (
                    <button
                        onClick={cta.onClick}
                        className="w-full px-4 py-2.5 bg-primary text-primary-foreground rounded-md text-[10px] font-black uppercase tracking-widest hover:brightness-110 active:scale-95 transition-all"
                    >
                        {cta.label}
                    </button>
                )}
                <button
                    onClick={onClose}
                    className="w-full px-4 py-2.5 bg-red-500 text-white rounded-md text-[10px] font-black uppercase tracking-widest hover:brightness-110 active:scale-95 transition-all"
                >
                    OK
                </button>
            </div>
        </div>
    );
}
