import {createContext, useCallback, useContext, useState, ReactNode, useEffect} from 'react';

const STORAGE_KEY = 'favorites:items';

interface FavoritesContextValue {
    favorites: Set<number>;
    isFav: (baseID: number) => boolean;
    toggle: (baseID: number) => void;
    remove: (baseID: number) => void;
    clear: () => void;
    count: number;
}

const FavoritesContext = createContext<FavoritesContextValue | null>(null);

function loadFromStorage(): Set<number> {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (!raw) return new Set();
        const arr = JSON.parse(raw);
        if (Array.isArray(arr)) return new Set(arr.filter((v): v is number => typeof v === 'number'));
    } catch { /* ignore */ }
    return new Set();
}

function saveToStorage(set: Set<number>) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify([...set]));
    } catch { /* ignore */ }
}

export function FavoritesProvider({children}: {children: ReactNode}) {
    const [favorites, setFavorites] = useState<Set<number>>(loadFromStorage);

    useEffect(() => {
        saveToStorage(favorites);
    }, [favorites]);

    const isFav = useCallback((baseID: number) => favorites.has(baseID), [favorites]);

    const toggle = useCallback((baseID: number) => {
        setFavorites(prev => {
            const next = new Set(prev);
            if (next.has(baseID)) next.delete(baseID); else next.add(baseID);
            return next;
        });
    }, []);

    const remove = useCallback((baseID: number) => {
        setFavorites(prev => {
            const next = new Set(prev);
            next.delete(baseID);
            return next;
        });
    }, []);

    const clear = useCallback(() => setFavorites(new Set()), []);

    return (
        <FavoritesContext.Provider value={{favorites, isFav, toggle, remove, clear, count: favorites.size}}>
            {children}
        </FavoritesContext.Provider>
    );
}

export function useFavorites(): FavoritesContextValue {
    const ctx = useContext(FavoritesContext);
    if (!ctx) throw new Error('useFavorites must be used within FavoritesProvider');
    return ctx;
}
