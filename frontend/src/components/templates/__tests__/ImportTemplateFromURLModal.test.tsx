import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ImportTemplateFromURLModal } from '../ImportTemplateFromURLModal';

describe('ImportTemplateFromURLModal — Phase 9 URL import', () => {
    it('renders the dialog with input, Preview, and Cancel', () => {
        render(<ImportTemplateFromURLModal onPreview={vi.fn()} onCancel={vi.fn()} />);
        expect(screen.getByTestId('import-url-modal')).toBeInTheDocument();
        expect(screen.getByTestId('import-url-input')).toBeInTheDocument();
        expect(screen.getByTestId('import-url-preview')).toBeInTheDocument();
        expect(screen.getByTestId('import-url-cancel')).toBeInTheDocument();
    });

    it('Preview is disabled when the input is empty', () => {
        render(<ImportTemplateFromURLModal onPreview={vi.fn()} onCancel={vi.fn()} />);
        expect(screen.getByTestId('import-url-preview')).toBeDisabled();
    });

    it('Preview is disabled when the input does not start with http(s)://', () => {
        render(<ImportTemplateFromURLModal onPreview={vi.fn()} onCancel={vi.fn()} />);
        fireEvent.change(screen.getByTestId('import-url-input'), {
            target: { value: 'ftp://example.com/x.yaml' },
        });
        expect(screen.getByTestId('import-url-preview')).toBeDisabled();
    });

    it('Preview is enabled for a well-formed https URL and calls onPreview with the trimmed value', async () => {
        const onPreview = vi.fn().mockResolvedValue({ ok: true });
        render(<ImportTemplateFromURLModal onPreview={onPreview} onCancel={vi.fn()} />);
        fireEvent.change(screen.getByTestId('import-url-input'), {
            target: { value: '   https://example.com/template.yaml   ' },
        });
        const btn = screen.getByTestId('import-url-preview');
        expect(btn).toBeEnabled();
        fireEvent.click(btn);
        await waitFor(() => {
            expect(onPreview).toHaveBeenCalledTimes(1);
        });
        expect(onPreview).toHaveBeenCalledWith('https://example.com/template.yaml');
    });

    it('shows "Fetching…" and disables inputs while the preview is in flight', async () => {
        let resolve!: (v: { ok: true }) => void;
        const onPreview = vi.fn().mockImplementation(
            () =>
                new Promise(r => {
                    resolve = r;
                }),
        );
        render(<ImportTemplateFromURLModal onPreview={onPreview} onCancel={vi.fn()} />);
        fireEvent.change(screen.getByTestId('import-url-input'), {
            target: { value: 'https://example.com/x.yaml' },
        });
        fireEvent.click(screen.getByTestId('import-url-preview'));
        await waitFor(() => {
            expect(screen.getByTestId('import-url-preview')).toHaveTextContent(/Fetching/);
        });
        expect(screen.getByTestId('import-url-input')).toBeDisabled();
        expect(screen.getByTestId('import-url-cancel')).toBeDisabled();
        resolve({ ok: true });
        await waitFor(() => {
            expect(screen.getByTestId('import-url-input')).not.toBeDisabled();
        });
    });

    it('renders inline error from onPreview and preserves the input value', async () => {
        const onPreview = vi
            .fn()
            .mockResolvedValue({ ok: false as const, error: 'url_forbidden_ip: 127.0.0.1 is not allowed.' });
        render(<ImportTemplateFromURLModal onPreview={onPreview} onCancel={vi.fn()} />);
        fireEvent.change(screen.getByTestId('import-url-input'), {
            target: { value: 'https://127.0.0.1/x' },
        });
        fireEvent.click(screen.getByTestId('import-url-preview'));
        await waitFor(() => {
            expect(screen.getByTestId('import-url-error')).toBeInTheDocument();
        });
        expect(screen.getByTestId('import-url-error')).toHaveTextContent(/127\.0\.0\.1/);
        // Input value is preserved so the user can fix and retry.
        expect(screen.getByTestId('import-url-input')).toHaveValue('https://127.0.0.1/x');
    });

    it('clears a prior error when the user changes the URL and retries', async () => {
        const onPreview = vi
            .fn()
            .mockResolvedValueOnce({ ok: false as const, error: 'first error' })
            .mockResolvedValueOnce({ ok: true as const });
        render(<ImportTemplateFromURLModal onPreview={onPreview} onCancel={vi.fn()} />);
        fireEvent.change(screen.getByTestId('import-url-input'), {
            target: { value: 'https://bad.example/x' },
        });
        fireEvent.click(screen.getByTestId('import-url-preview'));
        await waitFor(() => {
            expect(screen.getByTestId('import-url-error')).toHaveTextContent('first error');
        });
        fireEvent.change(screen.getByTestId('import-url-input'), {
            target: { value: 'https://good.example/x' },
        });
        fireEvent.click(screen.getByTestId('import-url-preview'));
        await waitFor(() => {
            expect(onPreview).toHaveBeenCalledTimes(2);
        });
        // After successful retry the error is cleared.
        expect(screen.queryByTestId('import-url-error')).not.toBeInTheDocument();
    });

    it('renders error when onPreview throws', async () => {
        const onPreview = vi.fn().mockRejectedValue(new Error('boom'));
        render(<ImportTemplateFromURLModal onPreview={onPreview} onCancel={vi.fn()} />);
        fireEvent.change(screen.getByTestId('import-url-input'), {
            target: { value: 'https://example.com/x' },
        });
        fireEvent.click(screen.getByTestId('import-url-preview'));
        await waitFor(() => {
            expect(screen.getByTestId('import-url-error')).toHaveTextContent(/boom/);
        });
    });

    it('Cancel button calls onCancel and does not call onPreview', () => {
        const onCancel = vi.fn();
        const onPreview = vi.fn();
        render(<ImportTemplateFromURLModal onPreview={onPreview} onCancel={onCancel} />);
        fireEvent.click(screen.getByTestId('import-url-cancel'));
        expect(onCancel).toHaveBeenCalledTimes(1);
        expect(onPreview).not.toHaveBeenCalled();
    });

    it('Enter key triggers Preview when the URL is valid', async () => {
        const onPreview = vi.fn().mockResolvedValue({ ok: true });
        render(<ImportTemplateFromURLModal onPreview={onPreview} onCancel={vi.fn()} />);
        const input = screen.getByTestId('import-url-input');
        fireEvent.change(input, { target: { value: 'https://example.com/x.yaml' } });
        fireEvent.keyDown(input, { key: 'Enter' });
        await waitFor(() => {
            expect(onPreview).toHaveBeenCalledTimes(1);
        });
    });
});
