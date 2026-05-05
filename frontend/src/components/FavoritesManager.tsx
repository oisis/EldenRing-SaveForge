import {useEffect, useState, useMemo} from 'react';
import {useFavorites} from '../state/favorites';
import {GetItemListChunk} from '../../wailsjs/go/main/App';
import {db} from '../../wailsjs/go/models';
import {CATEGORY_VALUES} from './CategorySelect';

export function FavoritesManager() {
    const {favorites, remove, clear, count} = useFavorites();
    const [allItems, setAllItems] = useState<db.ItemEntry[]>([]);
    const [loading, setLoading] = useState(true);
    const [search, setSearch] = useState('');

    useEffect(() => {
        let cancelled = false;
        (async () => {
            const accumulated: db.ItemEntry[] = [];
            for (const cat of CATEGORY_VALUES) {
                if (cancelled) return;
                try {
                    const chunk = await GetItemListChunk(cat);
                    if (chunk) accumulated.push(...chunk);
                } catch { /* skip */ }
            }
            if (!cancelled) {
                setAllItems(accumulated);
                setLoading(false);
            }
        })();
        return () => { cancelled = true; };
    }, []);

    const itemMap = useMemo(() => {
        const m = new Map<number, db.ItemEntry>();
        allItems.forEach(i => m.set(i.id, i));
        return m;
    }, [allItems]);

    const favItems = useMemo(() => {
        const items: db.ItemEntry[] = [];
        favorites.forEach(id => {
            const entry = itemMap.get(id);
            if (entry) items.push(entry);
        });
        const q = search.toLowerCase();
        const filtered = q ? items.filter(i => i.name.toLowerCase().includes(q)) : items;
        return filtered.sort((a, b) => a.name.localeCompare(b.name));
    }, [favorites, itemMap, search]);

    const unknownCount = useMemo(() => {
        let n = 0;
        favorites.forEach(id => { if (!itemMap.has(id)) n++; });
        return n;
    }, [favorites, itemMap]);

    if (loading) {
        return (
            <div className="flex flex-col items-center justify-center py-16 space-y-4">
                <div className="w-8 h-8 border-4 border-primary/20 border-t-primary rounded-full animate-spin" />
                <span className="text-[10px] font-black uppercase tracking-[0.2em] text-primary animate-pulse">Loading item database...</span>
            </div>
        );
    }

    return (
        <div className="space-y-4">
            <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2 px-3 py-2 rounded-md bg-muted/20 border border-border/50">
                    <svg className="w-3.5 h-3.5 text-amber-500" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                    </svg>
                    <span className="text-[10px] font-bold tabular-nums text-foreground">{count} favorites</span>
                    {unknownCount > 0 && (
                        <span className="text-[9px] text-muted-foreground">({unknownCount} unknown)</span>
                    )}
                </div>

                <div className="flex items-center gap-2">
                    <div className="relative">
                        <input
                            type="text"
                            placeholder="Search favorites..."
                            value={search}
                            onChange={e => setSearch(e.target.value)}
                            className="bg-muted/30 border border-border rounded-md px-8 py-1.5 text-[10px] font-bold uppercase tracking-wider focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all w-48"
                        />
                        <svg className="w-3.5 h-3.5 text-muted-foreground absolute left-2.5 top-1/2 -translate-y-1/2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                        </svg>
                    </div>
                    {count > 0 && (
                        <button
                            onClick={clear}
                            className="px-3 py-1.5 text-[9px] font-black uppercase tracking-widest text-red-400 border border-red-500/30 rounded-md hover:bg-red-500/10 transition-all"
                        >
                            Clear All
                        </button>
                    )}
                </div>
            </div>

            {favItems.length === 0 ? (
                <div className="text-center py-12">
                    <svg className="w-10 h-10 text-muted-foreground/20 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                    </svg>
                    <p className="text-[10px] font-black uppercase tracking-widest text-muted-foreground/50">
                        {count === 0 ? 'No favorites yet' : 'No matches'}
                    </p>
                    {count === 0 && (
                        <p className="text-[9px] text-muted-foreground/40 mt-1">
                            Mark items as favorites in Inventory or Item Database
                        </p>
                    )}
                </div>
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
                    {favItems.map(item => (
                        <div key={item.id} className="flex items-center gap-3 p-2.5 rounded-lg border border-border/50 bg-card hover:bg-muted/20 transition-all group">
                            <div className="w-10 h-10 rounded-lg bg-muted/20 border border-border/50 flex items-center justify-center overflow-hidden shrink-0">
                                <img src={item.iconPath} alt="" className="w-full h-full p-0.5 object-contain" onError={e => { (e.target as HTMLImageElement).style.display = 'none'; }} />
                            </div>
                            <div className="flex-1 min-w-0">
                                <div className="text-[11px] font-bold text-foreground truncate">{item.name}</div>
                                <div className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">{item.category.replace(/_/g, ' ')}</div>
                            </div>
                            <button
                                onClick={() => remove(item.id)}
                                className="p-1.5 rounded-md text-muted-foreground/50 hover:text-red-400 hover:bg-red-500/10 transition-all opacity-0 group-hover:opacity-100"
                                title="Remove from favorites"
                            >
                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
