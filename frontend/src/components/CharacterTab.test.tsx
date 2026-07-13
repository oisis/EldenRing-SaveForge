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
    GetCharacterAppearancePreset: vi.fn(() => Promise.resolve(null)),
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

describe('CharacterTab — Type B appearance enabled', () => {
    const PRESETS = [
        { name: 'Geralt of Rivia', image: '', bodyType: 'Type A' },
        { name: 'Ciri of Cintra', image: '', bodyType: 'Type B' },
    ];

    it('applies a Type B preset (calls ApplyPresetToCharacter)', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.ListAppearancePresets.mockResolvedValue(PRESETS);
        mocks.ApplyPresetToCharacter.mockResolvedValue(undefined);
        renderTab();
        await openAppearance();

        // Both cards now share the same Apply title; the Type B card is second.
        const applyButtons = await screen.findAllByTitle('Apply appearance to current character');
        fireEvent.click(applyButtons[1]);

        await waitFor(() => expect(mocks.ApplyPresetToCharacter).toHaveBeenCalledWith(0, 'Ciri of Cintra'));
    });

    it('applies a Type A preset normally', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.ListAppearancePresets.mockResolvedValue(PRESETS);
        mocks.ApplyPresetToCharacter.mockResolvedValue(undefined);
        renderTab();
        await openAppearance();

        const applyButtons = await screen.findAllByTitle('Apply appearance to current character');
        fireEvent.click(applyButtons[0]);

        await waitFor(() => expect(mocks.ApplyPresetToCharacter).toHaveBeenCalledWith(0, 'Geralt of Rivia'));
    });

    it('adds a Type B preset to Mirror (calls WriteSelectedToFavorites)', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.ListAppearancePresets.mockResolvedValue(PRESETS);
        mocks.WriteSelectedToFavorites.mockResolvedValue(1);
        // A free Mirror slot so the Add button is enabled.
        mocks.GetFavoritesStatus.mockResolvedValue([{ index: 0, active: false, safe: true, name: '' }]);
        renderTab();
        await openAppearance();

        const addButtons = await screen.findAllByText('Add');
        fireEvent.click(addButtons[1]); // Ciri (Type B)

        await waitFor(() => expect(mocks.WriteSelectedToFavorites).toHaveBeenCalledWith(0, ['Ciri of Cintra']));
    });
});

describe('CharacterTab — Body Type switch', () => {
    function bodyTypeSelect() {
        return screen.getByRole('option', { name: 'Type B (Female)' }).closest('select') as HTMLSelectElement;
    }

    it('switches to Type B (calls SetCharacterGender)', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        // Start on Type A so selecting Type B is an actual value change.
        mocks.GetCharacter.mockResolvedValue({ ...MOCK_CHAR, gender: 1 });
        mocks.SetCharacterGender.mockResolvedValue(undefined);
        renderTab();

        fireEvent.click(await screen.findByText('Profile'));
        fireEvent.change(bodyTypeSelect(), { target: { value: '0' } });

        await waitFor(() => expect(mocks.SetCharacterGender).toHaveBeenCalledWith(0, 0));
    });

    it('switches to Type A (male) normally', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        // Start on Type B so selecting Type A is an actual value change.
        mocks.GetCharacter.mockResolvedValue({ ...MOCK_CHAR, gender: 0 });
        mocks.SetCharacterGender.mockResolvedValue(undefined);
        renderTab();

        fireEvent.click(await screen.findByText('Profile'));
        fireEvent.change(bodyTypeSelect(), { target: { value: '1' } });

        await waitFor(() => expect(mocks.SetCharacterGender).toHaveBeenCalledWith(0, 1));
    });
});

describe('CharacterTab — Mirror Favorites labels after reload', () => {
    it('labels a named slot with the preset name and an unnamed active slot as "In-game favorite" (never "N/A"), with no thumbnail when unmatched', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        // clearAllMocks keeps implementations, so reset the preset list a sibling
        // test may have populated — otherwise its card duplicates the slot label.
        mocks.ListAppearancePresets.mockResolvedValue([]);
        // After a save reload favSlotNames is empty (name ''), so slot 2 has no
        // session name; slot 5 was written this session and keeps its name.
        // Neither carries an image → the backend did not recognise them.
        mocks.GetFavoritesStatus.mockResolvedValue([
            { index: 2, active: true, safe: true, name: '', image: '' },
            { index: 5, active: true, safe: true, name: 'Geralt of Rivia, the Witcher', image: '' },
        ]);
        renderTab();
        await openAppearance();

        // Unnamed active slot → honest fallback label, not "N/A".
        expect(await screen.findByText('In-game favorite')).toBeTruthy();
        expect(screen.queryByText('N/A')).toBeNull();

        // Named slot keeps the real preset name (first segment before comma).
        expect(screen.getByText('Geralt of Rivia')).toBeTruthy();

        // Unmatched entries (empty image) render no thumbnail.
        expect(screen.queryByRole('img')).toBeNull();
    });

    it('renders the canonical name and thumbnail for an exact Mirror match after reload (no session name)', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.ListAppearancePresets.mockResolvedValue([]);
        // Reloaded save: favSlotNames empty, but the backend matched the entry and
        // filled canonical name + image directly from GetFavoritesStatus.
        mocks.GetFavoritesStatus.mockResolvedValue([
            { index: 3, active: true, safe: true, name: 'Casca, Berserk’s Band of the Falcon Commander', image: 'casca.jpg' },
        ]);
        renderTab();
        await openAppearance();

        expect(await screen.findByText('Casca')).toBeTruthy();
        const thumb = screen.getByRole('img');
        expect(thumb.getAttribute('src')).toBe('presets/casca.jpg');
    });
});

describe('CharacterTab — matched appearance refresh after apply', () => {
    it('re-queries GetCharacterAppearancePreset after a successful direct Apply', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.ListAppearancePresets.mockResolvedValue([
            { name: 'Geralt of Rivia', image: '', bodyType: 'Type A' },
        ]);
        mocks.ApplyPresetToCharacter.mockResolvedValue(undefined);
        renderTab();
        await openAppearance();

        await waitFor(() => expect(mocks.GetCharacterAppearancePreset).toHaveBeenCalled());
        const before = mocks.GetCharacterAppearancePreset.mock.calls.length;

        const applyButtons = await screen.findAllByTitle('Apply appearance to current character');
        fireEvent.click(applyButtons[0]);

        await waitFor(() => expect(mocks.ApplyPresetToCharacter).toHaveBeenCalled());
        await waitFor(() =>
            expect(mocks.GetCharacterAppearancePreset.mock.calls.length).toBeGreaterThan(before),
        );
    });

    it('re-queries GetCharacterAppearancePreset after a successful Apply from Mirror', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.ListAppearancePresets.mockResolvedValue([]);
        mocks.GetFavoritesStatus.mockResolvedValue([
            { index: 0, active: true, safe: true, name: 'Geralt of Rivia', image: '' },
        ]);
        mocks.ApplyMirrorFavoriteToCharacter.mockResolvedValue(undefined);
        renderTab();
        await openAppearance();

        await waitFor(() => expect(mocks.GetCharacterAppearancePreset).toHaveBeenCalled());
        const before = mocks.GetCharacterAppearancePreset.mock.calls.length;

        const applyBtn = await screen.findByTitle('Apply this preset to character');
        fireEvent.click(applyBtn);

        await waitFor(() => expect(mocks.ApplyMirrorFavoriteToCharacter).toHaveBeenCalled());
        await waitFor(() =>
            expect(mocks.GetCharacterAppearancePreset.mock.calls.length).toBeGreaterThan(before),
        );
    });
});

describe('CharacterTab — matched appearance card', () => {
    it('renders the thumbnail and canonical name when the character exactly matches a Type B preset', async () => {
        mocks.GetFavoritesUndoDepth.mockResolvedValue(0);
        mocks.GetCharacterAppearancePreset.mockResolvedValue({
            name: 'Casca, Berserk’s Band of the Falcon Commander', image: 'casca.jpg', bodyType: 'Type B',
        });
        renderTab();

        // The matched card lives inside the Profile accordion.
        fireEvent.click(await screen.findByText('Profile'));

        expect(await screen.findByText('Matched appearance')).toBeTruthy();
        expect(screen.getByText('Casca, Berserk’s Band of the Falcon Commander')).toBeTruthy();
        const thumb = screen.getByAltText('Casca, Berserk’s Band of the Falcon Commander');
        expect(thumb.getAttribute('src')).toBe('presets/casca.jpg');
    });
});
