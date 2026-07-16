import { RecordDiagnosticClientError } from '../../wailsjs/go/main/App';

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

// Installs the process-wide browser handlers once at application startup.
// Returning a cleanup function keeps this seam deterministic in tests.
export function installDiagnosticClientErrorHandlers(): () => void {
    const onError = (event: ErrorEvent) => {
        reportDiagnosticClientError('unhandled_error', event.error ?? event.message);
    };
    const onUnhandledRejection = (event: PromiseRejectionEvent) => {
        reportDiagnosticClientError('unhandled_rejection', event.reason);
    };
    window.addEventListener('error', onError);
    window.addEventListener('unhandledrejection', onUnhandledRejection);
    return () => {
        window.removeEventListener('error', onError);
        window.removeEventListener('unhandledrejection', onUnhandledRejection);
    };
}
