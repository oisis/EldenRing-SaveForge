import { afterEach, describe, expect, it, vi } from 'vitest';

const recordDiagnosticClientError = vi.fn<(...args: string[]) => Promise<void>>().mockResolvedValue(undefined);
const recordDiagnosticClientAssetLoadFailure = vi.fn<(asset: string) => Promise<void>>().mockResolvedValue(undefined);

vi.mock('../../wailsjs/go/main/App', () => ({
    RecordDiagnosticClientError: (...args: string[]) => recordDiagnosticClientError(...args),
    RecordDiagnosticClientAssetLoadFailure: (asset: string) => recordDiagnosticClientAssetLoadFailure(asset),
}));

import { installDiagnosticClientErrorHandlers, reportDiagnosticClientError } from './diagnosticClientErrors';

function dispatchImageError(src: string): void {
    const img = document.createElement('img');
    img.src = src;
    document.body.appendChild(img);
    img.dispatchEvent(new Event('error'));
    img.remove();
}

// Builds a same-origin absolute URL so the <img>.src reflects Wails' real
// origin instead of a hardcoded host the jsdom origin would never match.
function sameOrigin(path: string): string {
    return new URL(path, window.location.href).href;
}

describe('diagnostic client error bridge', () => {
    afterEach(() => {
        recordDiagnosticClientError.mockClear();
        recordDiagnosticClientAssetLoadFailure.mockClear();
    });

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

    it('reports a real image load failure once and deduplicates', () => {
        const cleanup = installDiagnosticClientErrorHandlers();

        dispatchImageError(sameOrigin('/items/tools/fire_pot.png'));
        dispatchImageError(sameOrigin('/items/tools/fire_pot.png'));

        expect(recordDiagnosticClientAssetLoadFailure).toHaveBeenCalledTimes(1);
        expect(recordDiagnosticClientAssetLoadFailure).toHaveBeenCalledWith('items/tools/fire_pot.png');
        cleanup();
    });

    it('ignores external URLs, data URIs and unsafe paths', () => {
        const cleanup = installDiagnosticClientErrorHandlers();

        dispatchImageError('https://external.example/assets/logo.png');
        dispatchImageError('data:image/png;base64,AAAA');
        dispatchImageError(sameOrigin('/items/../secrets/key.png'));

        expect(recordDiagnosticClientAssetLoadFailure).not.toHaveBeenCalled();
        cleanup();
    });

    it('ignores a cross-origin URL that mimics the item icon path', () => {
        const cleanup = installDiagnosticClientErrorHandlers();

        dispatchImageError('https://external.example/items/tools/fire_pot.png');

        expect(recordDiagnosticClientAssetLoadFailure).not.toHaveBeenCalled();
        cleanup();
    });

    it('stops reporting image failures after cleanup', () => {
        const cleanup = installDiagnosticClientErrorHandlers();
        cleanup();

        dispatchImageError(sameOrigin('/items/tools/fire_pot.png'));

        expect(recordDiagnosticClientAssetLoadFailure).not.toHaveBeenCalled();
    });
});
