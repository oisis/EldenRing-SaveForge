# 11 — Regiony (lista odblokowanych regionow)

> **Zakres:** format binarny tablicy `unlocked_regions`. Sekcja o zmiennej
> dlugosci odpowiadajaca za szybka podroz i stan gry per-region.
> Semantyka wyzszego poziomu (regiony != odkrywanie mapy): patrz `spec/27-map-reveal.md`.

---

## Polozenie w slocie

Lista znajduje sie pod offsetem `gesturesOff` w dynamicznym lancuchu offsetow:

```
StorageBoxOffset
  + DynStorageBox        (0x6010)   = storageEnd
  + DynStorageToGestures (0x100)    = gesturesOff
  [ gesturesOff ]                   = unlocked_regions header
  + 4 + count * 4                   = afterRegs (horse / bloodstain start)
```

Stale zdefiniowane w `backend/core/offset_defs.go`.

---

## Format binarny

```
┌───────────────────────────────────┐
│ count : u32 (little-endian)       │  4 bytes
├───────────────────────────────────┤
│ region_id[0..count-1] : u32       │  count × 4 bytes
└───────────────────────────────────┘
Razem: 4 + count * 4 bytes
```

- Endianness: little-endian.
- Kolejnosc: na dysku przechowywane bez sortowania; edytor sortuje
  przy zapisie dla stabilnych diffow (`core.SetUnlockedRegions`).
- Duplikaty: nie zaobserwowane w zadnym referencyjnym safe'ie;
  `SetUnlockedRegions` deduplikuje prewencyjnie.

Swiezo utworzona postac ma 6 wpisow: `1001000, 1001001, 1001002, 1800001,
1800090, 6100000` (3 wewnetrzne markery startowe + Stranded Graveyard +
Cave of Knowledge + The First Step). W save'ach z poznego etapu gry
zaobserwowano do 395 wpisow.

---

## Zakresy ID regionow

| Zakres | Typ obszaru | Przyklady regionow |
|--------|-------------|---------------------|
| 1001000–1001002 | Wewnetrzne markery startowe (przeznaczenie nieznane) | — |
| 1xxxxxx (1010–1099, 1200–1207, 1300, 1400, 1500, 1600, 1800, 1900) | Legacy dungeons / strefy endgame | Stormveil Castle, Leyndell, Haligtree, Volcano Manor, Crumbling Farum Azula |
| 3xxxxxx | Katakumby / jaskinie / tunele | Stormfoot Catacombs, Murkwater Cave |
| 6100xxx | Limgrave overworld | The First Step, Stormhill |
| 6102xxx | Weeping Peninsula | Castle Morne |
| 6200xxx | Liurnia of the Lakes | Caria Manor |
| 6300xxx | Altus Plateau | Mt. Gelmir, Altus Highway |
| 6400xxx | Caelid / Dragonbarrow | Bestial Sanctum |
| 6500xxx | Mountaintops / Snowfield | Zamor Ruins, Forbidden Lands |

Pelna baza: `backend/db/data/regions.go` (211 wpisow base-game),
eksponowana przez `db.GetAllRegions()`. ID regionow DLC nie sa jeszcze
skatalogowane w naszej bazie — otwarta kwestia.

---

## Edycja

Uzyj `core.SetUnlockedRegions` — jedyny wspierany punkt wejscia:

```go
err := core.SetUnlockedRegions(slot, []uint32{6100000, 6100100, ...})
```

Zachowanie:

1. Deduplikuje wejscie.
2. Sortuje rosnaco.
3. Wywoluje `core.RebuildSlot` aby ponownie serializowac slot
   ze struktur typowanych (R-1 Step 13 — patrz `spec/30-slot-rebuild-research.md`).
4. Aktualizuje dynamiczne offsety (`GaItemDataOffset`, `IngameTimerOffset`,
   `EventFlagsOffset`) aby odpowiadaly nowemu ukladowi.

`RebuildSlot` jest bezpieczny dla kazdej realistycznej liczby: po
przebudowie koniec danych znajduje sie ~2.2 MB wglab slota o rozmiarze
0x280000 bajtow, pozostawiajac 408–432 KB zerowego paddingu na koncu.
Przetestowano do ~100 000 regionow w syntetycznych testach obciazeniowych;
uzytkownikowa sciezka "Unlock All" dodaje najwyzej ~211 regionow.

> **Nie edytuj** listy przez surowe wstawianie / przesuwanie bajtow.
> Historyczna sciezka "shift in place, max 10–20 regionow" zostala
> usunieta w R-1 Step 14, poniewaz obcinala region hasha powyzej ~205
> wstawien i uszkadzala save.

---

## Efekt (i czego NIE robi)

Ustawienie region ID:

- Wlacza szybka podroz miedzy Sites of Grace wewnatrz tego regionu.
- Oznacza region jako "odwiedzony" dla inwazji / matchmakingu multiplayer.

Ustawienie region ID **nie**:

- Nie odkrywa tekstury mapy (to flaga zdarzenia 62xxx — patrz
  `spec/27-map-reveal.md` §2).
- Nie usuwa mgly wojny (pole bitowe FoW jest niezalezne).
- Nie usuwa czarnych kafelkow DLC (Cover Layer jest niezalezny — patrz
  `spec/29-dlc-black-tiles.md`).

Te warstwy zostaly zweryfikowane empirycznie jako niezalezne (test 1 w
`spec/27-map-reveal.md` §5).

---

## Referencje

- Zrodlo formatu: `er-save-manager/parser/world.py::Regions` (linie 92–117);
  `ER-Save-Editor` (Rust) `src/save/common/save_slot.rs` lista z prefiksem dlugosci.
- Punkt wejscia edytora: `backend/core/writer.go::SetUnlockedRegions`.
- Przebudowa slota: `backend/core/slot_rebuild.go::RebuildSlot`.
- Baza regionow: `backend/db/data/regions.go`,
  `backend/db/db.go::GetAllRegions`.
- API Wails: `app.go::GetUnlockedRegions`, `SetRegionUnlocked`,
  `BulkSetUnlockedRegions`.
