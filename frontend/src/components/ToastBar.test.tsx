import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getDiagnosticLogTail = vi.fn<() => Promise<string>>();

vi.mock('../../wailsjs/go/main/App', () => ({
    GetDiagnosticLogTail: () => getDiagnosticLogTail(),
}));

import { ToastBar, sfLog, sfLoading } from './ToastBar';

const localStorageEntries: Record<string, string> = {};
vi.stubGlobal('localStorage', {
    getItem: (key: string) => localStorageEntries[key] ?? null,
    setItem: (key: string, value: string) => { localStorageEntries[key] = value; },
    removeItem: (key: string) => { delete localStorageEntries[key]; },
    clear: () => {
        for (const key of Object.keys(localStorageEntries)) delete localStorageEntries[key];
    },
});

const tail = JSON.stringify([
    {
        schema_version: 1,
        seq: 1,
        ts: '2026-07-16T12:00:00Z',
        level: 'info',
        source: 'app',
        event: 'save_loaded',
        message: 'active save loaded',
        fields: [{key: 'platform', value: 'PC'}],
    },
    {
        schema_version: 1,
        seq: 2,
        ts: '2026-07-16T12:00:01Z',
        level: 'error',
        source: 'app',
        event: 'save_write_failed',
        message: 'save write failed',
        fields: [{key: 'stage', value: 'write'}],
    },
]);

function openConsole() {
    fireEvent.keyDown(window, {key: '`'});
}

describe('ToastBar diagnostic console', () => {
    beforeEach(() => {
        localStorage.clear();
        getDiagnosticLogTail.mockReset();
        getDiagnosticLogTail.mockResolvedValue(tail);
    });

    it('renders the durable tail with structured details', async () => {
        render(<ToastBar />);
        openConsole();

        // The whole record is one line of text; only the level word is a
        // separate inline <span>, colored per level, everything else white.
        const infoLine = await screen.findByText(/\[app\/save_loaded\] active save loaded/);
        expect(infoLine).toHaveTextContent(/^\d{2}:\d{2}:\d{2} INFO \[app\/save_loaded\] active save loaded — platform: PC$/);
        expect(infoLine).toHaveClass('text-white');
        const infoLevel = within(infoLine).getByText('INFO');
        expect(infoLevel.tagName).toBe('SPAN');
        expect(infoLevel).toHaveClass('text-green-500');

        const errorLine = screen.getByText(/\[app\/save_write_failed\] save write failed/);
        expect(errorLine).toHaveTextContent(/^\d{2}:\d{2}:\d{2} ERROR \[app\/save_write_failed\] save write failed — stage: write$/);
        const errorLevel = within(errorLine).getByText('ERROR');
        expect(errorLevel).toHaveClass('text-red-500');
        expect(screen.getByText('durable session log')).toBeInTheDocument();
    });

    it('filters by level and searches event, message, and fields', async () => {
        render(<ToastBar />);
        openConsole();
        await screen.findByText(/active save loaded/);

        fireEvent.change(screen.getByLabelText('Log level'), {target: {value: 'error'}});
        expect(screen.queryByText(/active save loaded/)).not.toBeInTheDocument();
        expect(screen.getByText(/save write failed/)).toBeInTheDocument();

        fireEvent.change(screen.getByLabelText('Search logs'), {target: {value: 'stage: write'}});
        expect(screen.getByText(/save write failed/)).toBeInTheDocument();

        fireEvent.change(screen.getByLabelText('Search logs'), {target: {value: 'missing'}});
        await waitFor(() => expect(screen.getByText('No matching log entries')).toBeInTheDocument());
    });
    // Regression: the console used to replace the UI log with the durable
    // journal whenever the backend tail answered, so a deploy/SSH failure
    // reported only by the frontend was visible solely in the truncated,
    // non-copyable one-line status bar.
    it('keeps UI log entries in the console while the durable journal is available', async () => {
        render(<ToastBar />);
        openConsole();
        await screen.findByText(/active save loaded/);

        act(() => {
            sfLoading('deploy-1', 'Testing connection to "deck"...');
            sfLog('error', 'Connection test to "deck" failed: [tcp-dial] cannot reach 10.0.0.5:22: dial tcp 10.0.0.5:22: connect: connection refused');
            sfLog('info', 'Connection test OK — Connected to deck@10.0.0.5:22');
        });

        expect(screen.getByText(/Testing connection to "deck"/)).toBeInTheDocument();
        expect(screen.getByText(/\[tcp-dial\] cannot reach 10\.0\.0\.5:22/)).toHaveTextContent(
            'connect: connection refused',
        );
        expect(screen.getByText(/Connection test OK/)).toBeInTheDocument();
        // Backend journal records stay visible alongside the UI log.
        expect(screen.getByText(/active save loaded/)).toBeInTheDocument();
    });
});
