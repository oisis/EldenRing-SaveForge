# 33 — DB Categorization Audit (Information Tab + Multiplayer/Remembrances/Crystal Tears reclass)

> **Zakres**: Audyt i migracja itemów w `backend/db/data/*.go` po stronie kategoryzacji per-tab gry. Stworzenie nowej kategorii `info` (Information / Informacje), reorganizacja `tools.go`, `key_items.go`, `crafting_materials.go`. Audyt flag `cut_content` / `ban_risk` przy okazji.

> **Status**: ✅ Wdrożone na `feat/db-info-category` (Apr 2026).
>
> **Rozszerzone przez**: [spec/36 — Inventory Categories: Game-Accurate Order &
> Sub-Grouping](36-inventory-categories-game-order.md). Spec/33 ustaliło source-of-truth
> (Fextralife per-item) i wyciągnęło Information tab; spec/36 dokończa pracę
> definiując 18 zakładek głównych w kolejności z gry, mapując sub-grupy i
> przenosząc ostatnie misclassifications (Larval Tears, Torches, Region Maps,
> Golden Runes, Whetblades/Cookbooks visibility).

---

## Cel

Aktualne grupowanie itemów w naszym DB nie matchowało 1:1 zakładek inventory w grze. Naprawiamy:

- **Information tab w grze** (Polish: Informacje) nie miał odpowiednika w DB. About tutoriale były w `key_items.go`, Letters/Maps/Notes w `tools.go` lub `key_items.go`.
- **`tools.go` był catch-all** — zawierał Multiplayer Items, Remembrances, Crystal Tears, Keys, Scrolls, Materials. Wszystko to ma osobne sub-kategorie w grze.
- **`key_items.go` zawierał Multiplayer Items i Remembrances** mimo że gra pokazuje je w Items tab pod sub-kategoriami.

---

## Filozofia źródła prawdy

Po dwóch iteracjach pierwsza wersja audytu (oparta o `er-save-manager/Goods/*.txt`) okazała się niewłaściwa — *user zweryfikował in-game że Letters Volcano Manor / Patches / Bernahl są na zakładce Informacje, mimo że er-save-manager klasyfikuje je w `KeyItems.txt`*.

**Nowy ranking źródeł:**

1. **In-game observation** (user na PC z make dev / Steam Deck) — autorytatywne.
2. **Fextralife per-item page breadcrumb** (np. *Equipment & Magic / Items / Multiplayer Items*) — autorytatywne.
3. **Fextralife master listy** (Info Items, Multiplayer Items, Crystal Tears) — pomocnicze, mogą się rozjeżdżać z per-item pages.
4. **er-save-manager `Goods/*.txt`** — wstępny hint, wymaga cross-check (NotesPaintings.txt to ich subiektywna grupa, nie tab gry).
5. **ER-Save-Editor (Rust) `db/item_name.rs`** — tylko nazwy + ID, brak kategoryzacji per-tab.

---

## Mapowanie kategorii DB → in-game tab

| `Category` w DB | In-game tab w grze | Notki |
|---|---|---|
| `weapons` / `ranged_and_catalysts` / `shields` / `arrows_and_bolts` | Equipment > Weapons | OK, czyste |
| `head` / `chest` / `arms` / `legs` | Equipment > Armor | OK, czyste |
| `talismans` | Equipment > Talismans | OK |
| `sorceries` / `incantations` | Equipment > Spells | OK |
| `gestures` | Gestures menu (osobny) | OK |
| `ashes` | Items > Spirit Ashes | OK |
| `ashes_of_war` (`aows.go`) | Items > Ashes of War | OK |
| `crafting_materials` | Items > Materials > Crafting | OK |
| `bolstering_materials` | Items > Materials > Upgrade (Smithing/Somber Stones, Runes) | Project decyzja: Runy Golden/Hero/Lord/Numen/Lands Between/Rune Arc tu, mimo że er-save-manager ma w Consumables. Defensible. |
| `key_items` | Key Items tab (z subkategorią Crystal Tears) | po audycie: zawiera Crystal Tears (sub-tab), Cookbooks/Bell Bearings/Whetblades (filtered out via `Is*ItemID()`) |
| `tools` | Items tab (Consumables, Multiplayer, Remembrances, Throwables, Pots, Greases, Perfumes, Flasks itd.) | Catch-all dla Items. Zawiera 13 Multiplayer Items + 25 Remembrances + 27 Flasks (full) + 27 Flasks (empty) + reszta consumables |
| `info` (nowy) | Information tab (About tutoriale, Letters, Notes, Paintings, Maps, Cross messages) | 114 entries |

---

## Migracje wykonane

### 1. Wyciągnięcie Information tab (105 entries)

**Z `key_items.go` → `info.go`** (52):
- 35 About* tutoriali (`0x4000238C` — `0x400023B4`, plus `0x400023EB` cut)
- 7 Paintings (`0x40002008` — `0x4000200E`)
- 1 Map: Dragonbarrow (`0x400021A2`)
- 2 DLC About (`0x401EA848`, `0x401EA84A`)
- 7 DLC info messages: Castle Cross, Ancient Ruins Cross, Monk's Missive, Storehouse Cross, Torn Diary, Message from Leda, Tower of Shadow

**Z `tools.go` → `info.go`** (53 + 6 z drugiej iteracji):
- 1 About the Map (`0x40002393`) — w tools bo IconPath `tools/quest/`
- 19 base region maps (`0x40002198` — `0x400021AA` minus Dragonbarrow)
- 5 DLC region maps (`0x401EA618` — `0x401EA61C`)
- 6 Letters: Volcano Manor, Patches, Bernahl, Burial Crow's, Zorayas's, Rogier's
- 17 Notes (`0x4000222E` — `0x4000223D` + `0x4000220A` Miquella's Needle + `0x4000220C` Lord of Frenzied Flame)
- 1 DLC Letter for Freyja (`0x401EA3CF`)
- 2 DLC Notes (`0x401EA3D9` Furnace Keeper's, `0x401EA443` Sealed Spiritsprings)
- 3 DLC Paintings (`0x401EA488` — `0x401EA48A`)
- **Druga iteracja po user feedback** (6 dodanych — user zauważył brakujące): Irina's Letter (`0x40001FC3`), Red Letter (`0x40001FC5`), Cross Map (`0x401EA3C7`), 3× Ruins Map (`0x401EA3D0` / `D1` / `D2`)

### 2. Reklasyfikacja `tools.go` → `key_items.go` (31 entries)

**Crystal Tears** (11) — Fextralife: *"Crystal Tears in Elden Ring are Key Items that can be mixed in the Flask of Wondrous Physick."*: Speckled Hardtear, Crimson/Opaline Bubbletear, Opaline/Leaden Hardtear, Crimsonwhorl Bubbletear, Cerulean Hidden Tear + 4 DLC (Viridian Hidden, Crimsonburst Dried, Oil-Soaked, Deflecting Hardtear).

**Keys & Scrolls** (13): Stonesword Key, Rusty Key, Drawing-Room Key, Imbued Sword Key, Royal House Scroll, Well Depths Key (DLC), Gaol Upper/Lower Level Key (DLC), Storeroom Key (DLC), Secret Rite Scroll (DLC), Keep Wall Key (DLC, cut_content + ban_risk preserved), Prayer Room Key (DLC), Academy Glintstone Key.

**Items z IconPath `key_items/`** (7): Whetstone Knife, Glintstone Whetblade, Conspectus Scroll, Academy Scroll, Margit's Shackle, Mohg's Shackle, Pureblood Knight's Medal. 5 z 7 miało już IconPath wskazujący `items/key_items/` — `tools.go` był po prostu wrong category.

### 3. Reklasyfikacja `tools.go` → `crafting_materials.go` (5 entries)

Golden Centipede, Sanctuary Stone, Glintstone Firefly, Volcanic Stone, Gravel Stone — wszystkie crafting materials zgodnie z Fextralife per-item pages.

### 4. Reklasyfikacja `key_items.go` → `tools.go` (38 entries)

**Multiplayer Items** (13) — Fextralife breadcrumb *Equipment & Magic / Items / Multiplayer Items*: Bloody Finger, Tarnished's/Phantom Bloody Finger, Tarnished's/Phantom Recusant Finger, Recusant Finger, Festering Bloody Finger, Tarnished's Wizened Finger, Tarnished's/Duelist's Furled Finger, Igon's Furled Finger (DLC), Furlcalling Finger Remedy, Small Golden/Red Effigy, Taunter's Tongue.

**Remembrances** (25, 15 base + 10 DLC) — Fextralife breadcrumb *Equipment & Magic / Items / Remembrances*. Wszystkie boss remembrances do trade z Finger Reader Enia i Twin Maiden Husks.

---

## Audyt flag `cut_content` / `ban_risk`

Trzy wpisy oflagowane były jako `cut_content + ban_risk`. Web research zweryfikował:

| ID | Item | Verdict | Akcja |
|---|---|---|---|
| `0x400023EB` | About Multiplayer | ✅ Cut (Fextralife: "Unavailable" + spawned ma `[ERROR]` prefix) | Keep `cut_content + ban_risk`. Information item. |
| `0x400023A7` | About Monument Icon | ❌ NOT cut (was reachable v1.0 disc, broken w patch 1.06) | Drop `cut_content`, keep `ban_risk` (EAC nie whitelistuje wersji). Comment wyjaśnia. |
| `0x4000229D` | Erdtree Codex | ✅ Cut (Fextralife/Fandom/GameRant unanimous) | Keep `cut_content + ban_risk`. **Key Item, NIE Information** — zostaje w `key_items.go`. |
| `0x40001FF5` | Burial Crow's Letter | ⚠️ Fextralife mówi cut, user widzi w grze | Keep `cut_content + ban_risk`. Information item per user observation. |
| `0x401EA3CF` | Letter for Freyja | ⚠️ Master list: Info, per-item: Key Item | Information per master list. TODO comment do verify in-game. |

---

## Items intentionally NOT moved

| Item | Tab w grze | Decyzja | Powód |
|---|---|---|---|
| Empty Flasks (Cerulean/Crimson/Wondrous Physick) | Items > Consumables (Flasks) | Pozostają w `tools.go` | Fextralife: "Items / Consumables" = nasze tools (catch-all consumables). |
| Dragon Heart (`0x4000274C`) | Key Items | Pozostaje w `key_items.go` | Fextralife eksplicite: *"Dragon Heart is a Key Item"*. (er-save-manager ma w UpgradeMaterials.txt — Fextralife trumps.) |
| 5 borderline pot/bottle/kit (Cracked Pot, Ritual Pot, Hefty Cracked Pot, Perfume Bottle, Crafting Kit) | Multiple (Key Item + Crafting Material + Keepsake) | Pozostają w `key_items.go` | Fextralife eksplicite dla Cracked Pot: *"Key Item, Crafting Material, and optional Keepsake"* — funkcjonalnie 3 zakładki. |
| Cookbooks (`IsCookbookItemID`), Bell Bearings (`IsBellBearingItemID`), Whetblades (`IsWhetbladeItemID`) | Key Items / Bell Bearings sub / Whetblades sub | Pozostają w `key_items.go` | Project decision — data home. Filtered out z dropdown poprzez `Is*ItemID()` w `db.go::GetItemsByCategory("key_items")`. |

---

## Dyskrepancje wymagające in-game verification (TODO)

| ID | Item | Fextralife master | Fextralife per-item | Decyzja |
|---|---|---|---|---|
| `0x4000219E` | Meeting Place Map | Information | Key Item | Pozostawione w `tools.go` jako Map (= info via current categorization). User nie zweryfikował. |
| `0x400021D4` | Mirage Riddle | Information | Key Item | Pozostawione w `info.go` (decimal 8660 jest w er-save-manager NotesPaintings.txt). |
| `0x401EA3CF` | Letter for Freyja | Information | Key Item | Pozostawione w `info.go` per master list. TODO verify. |

---

## Counts (final)

| Plik | Before | After | Δ |
|---|---|---|---|
| `info.go` (new) | — | 114 | +114 |
| `key_items.go` | 396 | 343 | -53 |
| `tools.go` | 364 | 315 | -49 |
| `crafting_materials.go` | 80 | 85 | +5 |

Suma: 1,840 → 1,857 entries (+17 net — kilka dodanych po stronie merge'u).

---

## Known issues

### Prayer Room Key icon

**Problem**: `frontend/public/items/tools/quest/prayer_room_key.png` jest **binarnie identyczny** z `frontend/public/items/gestures/prayer.png` (oba 8762 bajty, identyczne bity). Ktoś podstawił ikonę gestu modlitwy zamiast prawdziwego klucza.

**Próby fixu**: Pobranie z Fextralife/Fandom CDN przez `curl` zwróciło 27-bajtowy text "Source image is unreachable" — CDNy blokują direct download. Manual download z przeglądarki będzie potrzebny.

**Workaround**: Kod ma `// TODO icon:` comment przy wpisie. User zweryfikuje i ręcznie podmieni plik.

---

## Źródła

### Web (autorytatywne dla per-item tab)

- https://eldenring.wiki.fextralife.com/Crystal+Tears — *"Crystal Tears in Elden Ring are Key Items that can be mixed in the Flask of Wondrous Physick."*
- https://eldenring.wiki.fextralife.com/Multiplayer+Items — breadcrumb *Items / Multiplayer Items*.
- https://eldenring.wiki.fextralife.com/Remembrance+of+the+Grafted — breadcrumb *Items / Remembrances*.
- https://eldenring.wiki.fextralife.com/About+Multiplayer — Unavailable, `[ERROR]` prefix when spawned.
- https://eldenring.wiki.fextralife.com/About+Monument+Icon — *"It can no longer be triggered in version 1.6 [sic, 1.06]"*.
- https://eldenring.wiki.fextralife.com/Erdtree+Codex — *"Indication points to this being cut content"*.
- https://eldenring.wiki.fextralife.com/Cracked+Pot — *"Cracked Pot is a Key Item, Crafting Material, and optional Keepsake."*
- https://eldenring.wiki.fextralife.com/Dragon+Heart — *"Dragon Heart is a Key Item."*
- https://eldenring.wiki.fextralife.com/Info+Items — master list 100+ entries.
- https://gamerant.com/elden-ring-cut-content/ — Erdtree Codex listed.
- https://kotaku.com/elden-ring-patch-1-06-fia-underwear-item-ban-fromsoft-1849389818 — illegal-item warnings.
- https://www.svg.com/961328/elden-rings-new-illegal-item-warnings-explained/ — same.

### Local

- `backend/db/data/info.go` (new), `key_items.go`, `tools.go`, `crafting_materials.go`, `db.go`
- `frontend/src/components/CategorySelect.tsx`, `DatabaseTab.tsx`
- Verification: `tmp/repos/er-save-manager/.../Goods/*.txt` (used as initial hint, not authoritative).

---

## Future work

1. **Empty Flasks** — może warto extract do osobnego `flasks.go` i sub-kategorii UI "Flasks" (Fextralife eksplicite ma "Flasks" jako sub-tab w Items). Aktualnie zostają w `tools.go` per user decision.
2. **Crystal Tears subcategory** — aktualnie pod kategorią `key_items`. Jeśli kiedyś będzie potrzeba osobnego filtra w dropdown, można extract do `crystal_tears.go`. 41 entries (11 z tools + 30 już było w key_items).
3. **Prayer Room Key icon fix** — manual artwork drop-in.
4. **Tools.go cleanup** — nadal jest catch-all (~315 entries: pots, throwables, perfumes, greases, multiplayer, remembrances, flasks). Może warto sub-foldery DB per Items sub-tab.
5. **5 borderline items in-game verification** — Cracked Pot / Ritual Pot / Hefty Cracked Pot / Perfume Bottle / Crafting Kit. Fextralife mówi multiple tabs, in-game verification rozstrzygnie.
