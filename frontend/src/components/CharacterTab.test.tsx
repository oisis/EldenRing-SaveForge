import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

// jsdom under an opaque origin may leave localStorage undefined; AccordionSection
// reads/writes it for expand persistence. Provide a minimal in-memory stub.
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

// Minimal character view model — just the fields CharacterTab reads during
// render, so the component gets past its `if (!char)` guard and mounts the
// Appearance Presets accordion.
const MOCK_CHAR = {
    name: 'Tarnished', class: 'Vagabond', gender: 0, level: 1,
    souls: 0, soulMemory: 0, clearCount: 0, memoryStones: 0, talismanSlots: 0,
    vigor: 10, mind: 10, endurance: 10, strength: 10,
    dexterity: 10, intelligence: 10, faith: 10, arcane: 10,
    classBaseStats: {},
};

vi.mock('../../wailsjs/go/main/App', () => ({
    GetCharacter: vi.fn(() => Promise.resolve(MOCK_CHAR)),
    SaveCharacter: vi.fn(),
    ListAppearancePresets: vi.fn(() => Promise.resolve([])),
    ApplyMirrorFavoriteToCharacter: vi.fn(),
    WriteSelectedToFavorites: vi.fn(),
    GetFavoritesStatus: vi.fn(() => Promise.resolve([])),
    RemoveFavoritePreset: vi.fn(),
    GetStartingClasses: vi.fn(() => Promise.resolve([])),
    SetCharacterGender: vi.fn(),
    ApplyPresetToCharacter: vi.fn(),
    GetFavoritesUndoDepth: vi.fn(),
    RevertFavorites: vi.fn(() => Promise.resolve()),
}));

vi.mock('../lib/toast', () => {
    const fn = vi.fn() as unknown as Record<string, unknown> & ((...args: unknown[]) => void);
    fn.success = vi.fn();
    fn.error = vi.fn();
    return { default: fn };
});

vi.mock('../state/safetyMode', () => ({
    useSafetyMode: () => ({ enabled: false, tier: 0, setEnabled: vi.fn() }),
}));

import * as App from '../../wailsjs/go/main/App';
import { CharacterTab } from './CharacterTab';

const mocks = App as unknown as Record<string, ReturnType<typeof vi.fn>>;

function renderTab() {
    return render(
        <CharacterTab
            charIndex={0}
            onMutate={vi.fn()}
            addSettings={{} as never}
            onAddSettingsChange={vi.fn()}
            infuseTypes={[]}
        />,
    );
}

// Open the Appearance Presets accordion so its children (incl. the Undo button)
// render — AccordionSection only mounts children when expanded.
async function openAppearance() {
    fireEvent.click(await screen.findByText('Appearance Presets'));
}

afterEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
});

describe('CharacterTab — Undo last Mirror change', () => {
    it('hides the Undo button when favorites undo depth is 0', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        renderTab();
        await openAppearance();
        await waitFor(() => expect(mocks.GetFavoritesUndoDepth).toHaveBeenCalled());
        expect(screen.queryByText(/Undo last Mirror change/i)).toBeNull();
    });

    it('shows the Undo button with the depth count and calls RevertFavorites on click', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(2);
        renderTab();
        await openAppearance();

        const btn = await screen.findByText(/Undo last Mirror change \(2\)/i);
        expect(btn).toBeTruthy();

        fireEvent.click(btn);
        await waitFor(() => expect(mocks.RevertFavorites).toHaveBeenCalledTimes(1));
        // Undo triggers a status refresh (favorites + depth) afterwards.
        await waitFor(() => expect(mocks.GetFavoritesUndoDepth.mock.calls.length).toBeGreaterThan(1));
    });
});

describe('CharacterTab — Apply blocked for Type B', () => {
    const PRESETS = [
        { name: 'Geralt of Rivia', image: '', bodyType: 'Type A' },
        { name: 'Ciri of Cintra', image: '', bodyType: 'Type B' },
    ];

    it('does not call ApplyPresetToCharacter for a Type B preset and shows a warning', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.ListAppearancePresets.mockResolvedValue(PRESETS);
        renderTab();
        await openAppearance();

        const typeBApply = await screen.findByTitle('Type B cannot be applied yet');
        fireEvent.click(typeBApply);

        await waitFor(() => expect(screen.getByText(/Type B not supported yet/i)).toBeTruthy());
        expect(mocks.ApplyPresetToCharacter).not.toHaveBeenCalled();
    });

    it('applies a Type A preset normally (not blocked)', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.ListAppearancePresets.mockResolvedValue(PRESETS);
        mocks.ApplyPresetToCharacter.mockResolvedValue(undefined);
        renderTab();
        await openAppearance();

        const typeAApply = await screen.findByTitle('Apply appearance to current character');
        fireEvent.click(typeAApply);

        await waitFor(() => expect(mocks.ApplyPresetToCharacter).toHaveBeenCalledWith(0, 'Geralt of Rivia'));
    });
});

describe('CharacterTab — Mirror Favorites labels after reload', () => {
    it('labels a named slot with the preset name and an unnamed active slot as "In-game favorite" (never "N/A")', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        // clearAllMocks keeps implementations, so reset the preset list a sibling
        // test may have populated — otherwise its card duplicates the slot label.
        mocks.ListAppearancePresets.mockResolvedValue([]);
        // After a save reload favSlotNames is empty (name ''), so slot 2 has no
        // session name; slot 5 was written this session and keeps its name.
        mocks.GetFavoritesStatus.mockResolvedValue([
            { index: 2, active: true, safe: true, name: '' },
            { index: 5, active: true, safe: true, name: 'Geralt of Rivia, the Witcher' },
        ]);
        renderTab();
        await openAppearance();

        // Unnamed active slot → honest fallback label, not "N/A".
        expect(await screen.findByText('In-game favorite')).toBeTruthy();
        expect(screen.queryByText('N/A')).toBeNull();

        // Named slot keeps the real preset name (first segment before comma).
        expect(screen.getByText('Geralt of Rivia')).toBeTruthy();
    });
});
