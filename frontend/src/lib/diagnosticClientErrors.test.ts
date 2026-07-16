import { afterEach, describe, expect, it, vi } from 'vitest';

const recordDiagnosticClientError = vi.fn<(...args: string[]) => Promise<void>>().mockResolvedValue(undefined);

vi.mock('../../wailsjs/go/main/App', () => ({
    RecordDiagnosticClientError: (...args: string[]) => recordDiagnosticClientError(...args),
}));

import { installDiagnosticClientErrorHandlers, reportDiagnosticClientError } from './diagnosticClientErrors';

describe('diagnostic client error bridge', () => {
    afterEach(() => recordDiagnosticClientError.mockClear());

    it('sends render errors without a stack trace or source URL', () => {
        const error = new TypeError('failed to render inventory');
        reportDiagnosticClientError('render', error);

        expect(recordDiagnosticClientError).toHaveBeenCalledWith('render', 'TypeError', 'failed to render inventory');
    });

    it('captures global error and rejection kinds and removes handlers cleanly', () => {
        const cleanup = installDiagnosticClientErrorHandlers();
        window.dispatchEvent(new ErrorEvent('error', {error: new Error('unexpected UI error')}));
        window.dispatchEvent(new PromiseRejectionEvent('unhandledrejection', {
            promise: Promise.resolve(), reason: 'network request failed',
        }));

        expect(recordDiagnosticClientError).toHaveBeenCalledWith('unhandled_error', 'Error', 'unexpected UI error');
        expect(recordDiagnosticClientError).toHaveBeenCalledWith('unhandled_rejection', 'NonError', 'network request failed');

        cleanup();
        recordDiagnosticClientError.mockClear();
        window.dispatchEvent(new PromiseRejectionEvent('unhandledrejection', {
            promise: Promise.resolve(), reason: 'ignored after cleanup',
        }));
        expect(recordDiagnosticClientError).not.toHaveBeenCalled();
    });
});
