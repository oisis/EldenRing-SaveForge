import { RecordDiagnosticClientAssetLoadFailure, RecordDiagnosticClientError } from '../../wailsjs/go/main/App';

export type ClientDiagnosticErrorKind = 'render' | 'unhandled_error' | 'unhandled_rejection';

function errorDetails(reason: unknown): { errorType: string; message: string } {
    if (reason instanceof Error) {
        return {errorType: reason.name || 'Error', message: reason.message};
    }
    if (typeof reason === 'string') {
        return {errorType: 'NonError', message: reason};
    }
    return {errorType: 'NonError', message: 'non-error frontend failure'};
}

// Sends only the bounded backend contract: kind, error class and message.
// Callers intentionally never pass a stack trace, source URL, filename, or
// browser event object. Backend sanitization applies before persistence.
export function reportDiagnosticClientError(kind: ClientDiagnosticErrorKind, reason: unknown): void {
    const {errorType, message} = errorDetails(reason);
    RecordDiagnosticClientError(kind, errorType, message).catch(() => {});
}

// Mirrors the backend isValidIconAsset rule: items/<category>/<file>.png,
// lowercase letters, digits, '_', '-', '.' only. Client-side validation is a
// defence-in-depth filter — the backend validates independently.
const iconAssetPattern = /^items\/[a-z0-9._-]+\/[a-z0-9._-]+\.png$/;

// Extracts the safe, relative icon path from an <img> src, or null. Rejects any
// cross-origin src first (so an external URL that merely mimics the items/…
// pathname is ignored), then keeps the same-origin pathname only when it matches
// the strict icon syntax, so a data URI, query string, or any other path drops.
function safeIconAsset(src: string): string | null {
    let parsed: URL;
    try {
        parsed = new URL(src);
    } catch {
        return null;
    }
    if (parsed.origin !== window.location.origin) {
        return null;
    }
    const asset = parsed.pathname.replace(/^\/+/, '');
    if (asset.includes('..') || !iconAssetPattern.test(asset)) {
        return null;
    }
    return asset;
}

// Reports one real item-icon load failure, deduplicated for the frontend
// session so a repeatedly failing icon costs at most one endpoint call.
const reportedAssets = new Set<string>();

export function reportDiagnosticAssetLoadFailure(asset: string): void {
    if (reportedAssets.has(asset)) {
        return;
    }
    reportedAssets.add(asset);
    RecordDiagnosticClientAssetLoadFailure(asset).catch(() => {});
}

// Installs the process-wide browser handlers once at application startup.
// Returning a cleanup function keeps this seam deterministic in tests.
export function installDiagnosticClientErrorHandlers(): () => void {
    // One install == one frontend session, so start dedup fresh here.
    reportedAssets.clear();
    const onError = (event: ErrorEvent) => {
        reportDiagnosticClientError('unhandled_error', event.error ?? event.message);
    };
    const onUnhandledRejection = (event: PromiseRejectionEvent) => {
        reportDiagnosticClientError('unhandled_rejection', event.reason);
    };
    // <img> load failures do not bubble, so this listener captures in the
    // capture phase and reacts only to image targets. It is separate from the
    // JavaScript-exception handlers above and does not touch their reporting.
    const onAssetError = (event: Event) => {
        const target = event.target;
        if (!(target instanceof HTMLImageElement)) {
            return;
        }
        const asset = safeIconAsset(target.src);
        if (asset) {
            reportDiagnosticAssetLoadFailure(asset);
        }
    };
    window.addEventListener('error', onError);
    window.addEventListener('unhandledrejection', onUnhandledRejection);
    document.addEventListener('error', onAssetError, true);
    return () => {
        window.removeEventListener('error', onError);
        window.removeEventListener('unhandledrejection', onUnhandledRejection);
        document.removeEventListener('error', onAssetError, true);
    };
}
