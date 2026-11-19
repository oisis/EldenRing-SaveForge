import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { db } from '../../wailsjs/go/models';

if (typeof globalThis.localStorage === 'undefined') {
    const store = new Map<string, string>();
    Object.defineProperty(globalThis, 'localStorage', {
        configurable: true,
        value: {
            getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
            setItem: (k: string, v: string) => { store.set(k, String(v)); },
            removeItem: (k: string) => { store.delete(k); },
            clear: () => { store.clear(); },
            key: (i: number) => Array.from(store.keys())[i] ?? null,
            get length() { return store.size; },
        },
    });
}

vi.mock('../../wailsjs/go/main/App', () => ({
    GetGraces: vi.fn().mockResolvedValue([]),
    SetGraceVisited: vi.fn().mockResolvedValue(undefined),
    GetBosses: vi.fn().mockResolvedValue([]),
    SetBossDefeated: vi.fn().mockResolvedValue(undefined),
    GetSummoningPools: vi.fn().mockResolvedValue([]),
    SetSummoningPoolActivated: vi.fn().mockResolvedValue(undefined),
    GetColosseums: vi.fn().mockResolvedValue([]),
    SetColosseumUnlocked: vi.fn().mockResolvedValue(undefined),
    GetMapProgress: vi.fn().mockResolvedValue([]),
    SetMapFlag: vi.fn().mockResolvedValue(undefined),
    SetMapRegionFlags: vi.fn().mockResolvedValue(undefined),
    RevealAllMap: vi.fn().mockResolvedValue(undefined),
    ResetMapExploration: vi.fn().mockResolvedValue(undefined),
    RemoveFogOfWar: vi.fn().mockResolvedValue(undefined),
    GetCookbooks: vi.fn().mockResolvedValue([]),
    SetCookbookUnlocked: vi.fn().mockResolvedValue(undefined),
    BulkSetCookbooksUnlocked: vi.fn().mockResolvedValue(undefined),
    GetGestures: vi.fn().mockResolvedValue([]),
    SetGestureUnlocked: vi.fn().mockResolvedValue(undefined),
    BulkSetGesturesUnlocked: vi.fn().mockResolvedValue(undefined),
    GetQuestNPCs: vi.fn().mockResolvedValue(['Ranni']),
    GetQuestProgress: vi.fn(),
    SetQuestStep: vi.fn().mockResolvedValue(undefined),
    GetBellBearings: vi.fn().mockResolvedValue([]),
    SetBellBearingUnlocked: vi.fn().mockResolvedValue(undefined),
    BulkSetBellBearings: vi.fn().mockResolvedValue(undefined),
    GetWhetblades: vi.fn().mockResolvedValue([]),
    SetWhetbladeUnlocked: vi.fn().mockResolvedValue(undefined),
    GetUnlockedRegions: vi.fn().mockResolvedValue([]),
    SetRegionUnlocked: vi.fn().mockResolvedValue(undefined),
    BulkSetUnlockedRegions: vi.fn().mockResolvedValue(undefined),
    RecordDiagnosticWorldAction: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('../state/safetyMode', () => ({
    useSafetyMode: () => ({ enabled: false, tier: 0, requireConfirmFor: () => false }),
}));

import {
    GetGraces,
    GetQuestNPCs,
    GetQuestProgress,
    SetGraceVisited,
    SetMapFlag,
    SetQuestStep,
} from '../../wailsjs/go/main/App';
import { WorldTab } from './WorldTab';

const mockQuestData: db.QuestNPC = {
    name: 'Ranni',
    steps: [
        {
            description: 'Meet Ranni at Ranni Rise in Liurnia',
            location: 'Three Sisters',
            complete: false,
            flags: [{ id: 1001, current: false, target: 1 }],
        },
        {
            description: 'Speak with miniature Ranni at Ainsel River',
            location: 'Ainsel River',
            complete: false,
            flags: [{ id: 1002, current: false, target: 1 }],
        },
    ],
} as unknown as db.QuestNPC;

describe('WorldTab NPC Quests scroll and mount stability', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        localStorage.clear();
        vi.mocked(GetQuestNPCs).mockResolvedValue(['Ranni']);
        vi.mocked(GetQuestProgress).mockResolvedValue(mockQuestData);
    });

    it('keeps quest steps list mounted and disables action buttons during mutation reload', async () => {
        render(<WorldTab charIdx={0} />);

        // Switch to progress subtab
        const progressTabBtn = await screen.findByRole('button', { name: /^progress$/i });
        fireEvent.click(progressTabBtn);

        // Expand NPC Quests accordion
        const accordionBtn = await screen.findByRole('button', { name: /npc quests/i });
        fireEvent.click(accordionBtn);

        // Select Ranni from NPC dropdown
        const select = screen.getByRole('combobox');
        fireEvent.change(select, { target: { value: 'Ranni' } });

        // Verify initial quest step is displayed
        expect(await screen.findByText('Meet Ranni at Ranni Rise in Liurnia')).toBeInTheDocument();
        expect(screen.getByText('Speak with miniature Ranni at Ainsel River')).toBeInTheDocument();

        // Prepare deferred reload for SetQuestStep mutation
        let resolveReload: (value: db.QuestNPC) => void = () => {};
        const deferredReload = new Promise<db.QuestNPC>((resolve) => {
            resolveReload = resolve;
        });
        vi.mocked(GetQuestProgress).mockImplementationOnce(() => deferredReload);

        // Click "Set" on the first step
        const setButtons = screen.getAllByRole('button', { name: /^set$/i });
        fireEvent.click(setButtons[0]);

        await waitFor(() => {
            expect(SetQuestStep).toHaveBeenCalledWith(0, 'Ranni', 0);
        });

        // While reload is pending (questLoading === true):
        // 1. The step list must remain mounted in the DOM
        expect(screen.getByText('Meet Ranni at Ranni Rise in Liurnia')).toBeInTheDocument();
        expect(screen.getByText('Speak with miniature Ranni at Ainsel River')).toBeInTheDocument();

        // 2. Action buttons and selector should be disabled to prevent duplicate requests
        expect(screen.getAllByRole('button', { name: /^set$/i })[0]).toBeDisabled();
        expect(screen.getByRole('combobox')).toBeDisabled();

        // Resolve the reload promise with step 0 marked complete
        const updatedQuestData: db.QuestNPC = {
            ...mockQuestData,
            steps: [
                {
                    ...mockQuestData.steps[0],
                    complete: true,
                    flags: [{ id: 1001, current: true, target: 1 }],
                },
                mockQuestData.steps[1],
            ],
        } as unknown as db.QuestNPC;
        resolveReload(updatedQuestData);

        // Wait for update to apply
        await waitFor(() => {
            expect(screen.getByRole('button', { name: /^unset$/i })).toBeInTheDocument();
        });

        // Verify only 1 "Set" button remains (for step 1) and buttons are re-enabled
        expect(screen.getAllByRole('button', { name: /^set$/i })).toHaveLength(1);
        expect(screen.getByRole('combobox')).not.toBeDisabled();
    });

    it('keeps quest steps list mounted during individual flag toggle reload', async () => {
        render(<WorldTab charIdx={0} />);

        // Switch to progress subtab and open NPC Quests
        const progressTabBtn = await screen.findByRole('button', { name: /^progress$/i });
        fireEvent.click(progressTabBtn);

        const accordionBtn = await screen.findByRole('button', { name: /npc quests/i });
        fireEvent.click(accordionBtn);

        const select = screen.getByRole('combobox');
        fireEvent.change(select, { target: { value: 'Ranni' } });

        expect(await screen.findByText('Meet Ranni at Ranni Rise in Liurnia')).toBeInTheDocument();

        // Expand step 0 details to expose flag toggle buttons
        fireEvent.click(screen.getByText('Meet Ranni at Ranni Rise in Liurnia'));
        expect(screen.getByText('1001')).toBeInTheDocument();

        // Prepare deferred reload for flag toggle
        let resolveReload: (value: db.QuestNPC) => void = () => {};
        const deferredReload = new Promise<db.QuestNPC>((resolve) => {
            resolveReload = resolve;
        });
        vi.mocked(GetQuestProgress).mockImplementationOnce(() => deferredReload);

        // Toggle flag 1001
        fireEvent.click(screen.getByText('1001').closest('button')!);

        await waitFor(() => {
            expect(SetMapFlag).toHaveBeenCalledWith(0, 1001, true);
        });

        // While reload is pending, the list and expanded flag remain mounted
        expect(screen.getByText('Meet Ranni at Ranni Rise in Liurnia')).toBeInTheDocument();
        expect(screen.getByText('1001')).toBeInTheDocument();
        expect(screen.getByText('1001').closest('button')).toBeDisabled();

        // Resolve reload
        resolveReload(mockQuestData);
        await waitFor(() => {
            expect(screen.getByText('1001').closest('button')).not.toBeDisabled();
        });
    });
});

describe('WorldTab Sites of Grace boss-arena skip', () => {
    const grace = (id: number, name: string, isBossArena: boolean) =>
        ({ id, name, region: 'Limgrave', visited: false, isBossArena }) as unknown as db.GraceEntry;

    beforeEach(() => {
        vi.clearAllMocks();
        localStorage.clear();
        vi.mocked(GetQuestNPCs).mockResolvedValue([]);
        vi.mocked(GetGraces).mockResolvedValue([
            grace(1, 'Plain Grace', false),
            grace(2, 'Arena Grace', true),
        ]);
    });

    const renderAndLoad = async () => {
        render(<WorldTab charIdx={0} />);
        await waitFor(() => expect(GetGraces).toHaveBeenCalled());
        return screen.findByText('Skip Boss Arenas');
    };

    it('labels the checkbox "Skip Boss Arenas" with an explanatory tooltip', async () => {
        const label = await renderAndLoad();
        expect(screen.queryByText('Skip Bosses')).not.toBeInTheDocument();
        expect(label.closest('label')).toHaveAttribute(
            'title',
            'Unlock All leaves boss-arena graces (marked B) locked. It does not defeat bosses or change boss flags.');
    });

    it('Unlock All skips boss-arena graces while the checkbox is checked', async () => {
        const label = await renderAndLoad();
        expect(label.closest('label')!.querySelector('input')).toBeChecked();
        const unlockAll = screen.getAllByRole('button', { name: 'Unlock All' })[0];
        fireEvent.click(unlockAll);
        await waitFor(() => expect(SetGraceVisited).toHaveBeenCalledWith(0, 1, true));
        expect(SetGraceVisited).not.toHaveBeenCalledWith(0, 2, true);
    });

    it('Unlock All includes boss-arena graces when the checkbox is unchecked', async () => {
        const label = await renderAndLoad();
        fireEvent.click(label.closest('label')!.querySelector('input')!);
        fireEvent.click(screen.getAllByRole('button', { name: 'Unlock All' })[0]);
        await waitFor(() => expect(SetGraceVisited).toHaveBeenCalledWith(0, 2, true));
        expect(SetGraceVisited).toHaveBeenCalledWith(0, 1, true);
    });
});
