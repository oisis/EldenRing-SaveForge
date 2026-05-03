# 27 — Odkrywanie mapy (widocznosc, szybka podroz, Cover Layer, mgla wojny)

> **Zakres:** jak edytor udostepnia mape swiata graczowi.
> Zastepuje starszy `27-fog-of-war.md` — pole bitowe FoW to jedna z czterech
> niezaleznych warstw, eksponowana jako oddzielna akcja uzytkownika (`RemoveFogOfWar`)
> a nie jako czesc `RevealAllMap`.

---

## 1. Trzywarstowy model mapy

Mapa Elden Ring to stos niezaleznych warstw. Odkrycie mapy dla gracza
wymaga modyfikacji co najmniej warstw 1 i 2; warstwa 3 (FoW) jest
kosmetyczna, a warstwa 0 (regiony) sluzy szybkiej podrozy. Kazda warstwa
jest kontrolowana przez inny mechanizm w pliku zapisu.

| # | Warstwa | Co robi | Przechowywanie | Funkcja edytora |
|---|---------|---------|----------------|------------------|
| 0 | Unlocked Regions | Szybka podroz + dostepnosc stanu regionu | Lista `u32` ID o zmiennej dlugosci pod `gesturesOff` | `core.SetUnlockedRegions` (patrz §3) |
| 1 | Detailed Bitmap | Kolorowa tekstura mapy per-region | Flagi zdarzen `62xxx` (visible) + przedmioty Map Fragment w ekwipunku | `App.SetMapRegionFlags`, `App.RevealAllMap` (patrz §2) |
| 2 | Cover Layer | Czarne kafelki ukrywajace nieodkryte obszary DLC | 8 floatow wewnatrz sekcji BloodStain | `revealDLCMap` (patrz `spec/29-dlc-black-tiles.md`) |
| 3 | Fog of War | Szary overlay nad tekstura mapy | Geste pole bitowe w luce BloodStain→MenuProfile | `App.RemoveFogOfWar` (oddzielna akcja — patrz §4) |

**Implikacja:** `RevealAllMap` (akcja masowa) dotyka tylko warstw 1 i 2
— ustawia flagi widocznosci, dodaje odpowiednie przedmioty Map Fragment
(aby UI mapy w grze wyswietlalo teksture) i — dla regionow DLC —
nadpisuje wspolrzedne Cover Layer. Pole bitowe FoW **nie jest** czescia
sciezki "Reveal All"; jest eksponowane oddzielnie jako `RemoveFogOfWar`,
aby uzytkownik mogl zdecydowac o kosmetycznym usunieciu mgly niezaleznie
od odkrycia tekstur mapy. Patrz §4.

---

## 2. Warstwa 1 — Detailed Bitmap (flagi zdarzen + fragmenty)

To **glowny** mechanizm uzywany przez edytor. Implementacja referencyjna:
`app.go::revealBaseMap` i `app.go::revealDLCMap`.

### 2.1 Flagi systemowe

Cztery flagi zdarzen kontroluja czy UI mapy jest w ogole widoczne. Bez nich
odkrywanie tekstur regionow nie daje efektu — ekran mapy pozostaje pusty.

| Flag ID | Nazwa | Kiedy wymagana |
|---------|-------|----------------|
| 62000 | Allow Map Display | Base game — zawsze |
| 62001 | Allow Underground Map Display | Przy odkrywaniu 62060–62064 |
| 82001 | Show Underground | Przy odkrywaniu 62060–62064 |
| 62002 | Allow Shadow Realm Map Display | Przy odkrywaniu 62080–62084 (DLC) |
| 82002 | Show Shadow Realm Map | Przy odkrywaniu 62080–62084 (DLC) |

Zrodlo: `backend/db/data/maps.go::MapSystem`.

### 2.2 Flagi widocznosci regionow (62xxx)

Kazdy region mapy ma pojedyncza flage widocznosci w zakresie `62xxx`.
Ustawienie flagi odkrywa teksture regionu w UI mapy w grze.

| Zakres | Obszar |
|--------|--------|
| 62010–62012 | Limgrave |
| 62020–62022 | Liurnia |
| 62030–62032 | Altus Plateau |
| 62040–62041 | Caelid |
| 62050–62052 | Mountaintops / Snowfield |
| 62060–62064 | Underground |
| 62080–62084 | DLC (Shadow of the Erdtree) |
| 62100–62xxx | Mapy specyficzne dla dungeonow |

Pelne dane i sprawdzanie zakresu DLC: `backend/db/data/maps.go::MapVisible` /
`IsDLCMapFlag`. Flagi podregionow, ktore uszkadzaja Cover Layer przy recznym
ustawieniu, znajduja sie w `MapUnsafe` i sa wykluczone z "Reveal All".

### 2.3 Przedmioty Map Fragment

Kazda overworldowa flaga 62xxx ma sparowany przedmiot w ekwipunku (Map Fragment).
Flaga odkrywa teksture; przedmiot to co gracz podnosi w normalnej rozgrywce.
Dodajemy oba dla spojnosci z normalnym przebiegiem gry.

Mapowanie: `backend/db/data/maps.go::MapFragmentItems` (24 wpisy — 19 base
game + 5 DLC). ID przedmiotow obejmuja `0x40002198..0x400021AA` (base) i
`0x401EA618..0x401EA61C` (DLC).

### 2.4 Flagi pozyskania (63xxx) — NIE uzywane

`backend/db/data/maps.go::MapAcquired` dokumentuje flagi 63xxx odpowiadajace
kazdej fladze widocznosci (offset = `visibleID + 1000`). Sa to **przejsciowe
flagi powiadomien**, ktore gra podnosi aby wyswietlic popup "Map Fragment acquired",
a nastepnie kasuje. Nie maja wplywu na widocznosc mapy i sa celowo
nie przelaczane przez edytor. Zostaw je w spokoju.

### 2.5 Algorytm — `RevealAllMap`

```
revealBaseMap(slot):
    flags = slot.Data[slot.EventFlagsOffset:]

    # Faza 1 — flagi systemowe + widocznosci (brak mutacji slota, slice jest bezpieczny)
    for id in MapSystem (excluding DLC system flags): SetEventFlag(flags, id, true)
    for id in MapVisible where !IsDLCMapFlag(id):
        SetEventFlag(flags, id, true)
        if id in MapFragmentItems: queue item add

    # Faza 2 — dodawanie przedmiotow fragmentow (mutuje dlugosc slota, slice flag jest niewazny)
    for itemID in queue:
        AddItemsToSlot(slot, [itemID], qty=1, durability=0, isWeapon=false)

revealDLCMap(slot):
    # te same fazy dla flag DLC + fragmentow DLC
    SetEventFlag(flags, 62002, true); SetEventFlag(flags, 82002, true)
    ...

    # Faza 3 — zapis Cover Layer (patrz spec/29)
```

**Kolejnosc ma znaczenie.** `AddItemsToSlot` przesuwa bajty wewnatrz slota,
co uniewazniana kazdy wczesniej obliczony slice do `slot.Data`. Ustaw
wszystkie flagi przed dodaniem jakiegokolwiek przedmiotu lub przelicz
slice flag miedzy wywolaniami.

---

## 3. Warstwa 0 — Unlocked Regions (szybka podroz)

Lista `u32` ID regionow o zmiennej dlugosci pod `gesturesOff` wewnatrz slota.
Szczegolowy format: `spec/11-regions.md`.

### 3.1 Efekt

Kazdy region ID odpowiada obszarowi geograficznemu. Gra uzywa listy do:

- Wlaczenia szybkiej podrozy miedzy Sites of Grace wewnatrz regionu.
- Sledzenia stanu per-region dla inwazji i matchmakingu multiplayer.

**Region ID NIE usuwaja FoW ani nie odkrywaja tekstury mapy.** Sa
niezaleznym systemem — zweryfikowano empirycznie (test 1 w §5).

### 3.2 Zakresy ID (wybrane)

| Zakres | Obszar |
|--------|--------|
| 1001000–1001002 | Wewnetrzne regiony startowe (przeznaczenie nieznane) |
| 1800001 / 1800090 | Stranded Graveyard / Cave of Knowledge |
| 6100xxx | Limgrave overworld |
| 6102xxx | Weeping Peninsula |
| 6200xxx | Liurnia |
| 6300xxx | Altus Plateau |
| 6400xxx | Caelid / Dragonbarrow |
| 6500xxx | Mountaintops / Snowfield |
| 1xxxxx | Legacy dungeons (prefiksy 1000–1900) |
| 3xxxxxx | Katakumby / jaskinie / tunele |

Pelna lista: `backend/db/data/regions.go` (eksponowana przez `db.GetAllRegions`).
Swiezy save ma 6 wpisow; "Unlock All" ustawia ~211 ID regionow base-game.

### 3.3 Edycja — uzyj `core.SetUnlockedRegions`

```go
err := core.SetUnlockedRegions(slot, []uint32{...})
```

Funkcja deduplikuje + sortuje ID i przebudowuje dotkniety fragment
slota przez `core.RebuildSlot` (pelny sekwencyjny serializator, patrz
`spec/30-slot-rebuild-research.md`). Zero ryzyka obciecia slota:
`RebuildSlot` dochodzi do konca danych okolo bajtu ~2.2 MB, pozostawiajac
408–432 KB paddingu na koncu wewnatrz slota o rozmiarze 0x280000 bajtow.
Przetestowano do ~100 000 regionow w syntetycznych testach obciazeniowych.

> **Nota historyczna.** Wczesniejsze iteracje tej specyfikacji (i oryginalna
> implementacja Stage-2) wstawialy region ID w miejscu przesuwajac reszte
> slota, z limitem bezpieczenstwa "max 10–20 regionow". Ta sciezka zostala
> usunieta w R-1 Step 14 — `SetUnlockedRegions` jest teraz jedynym
> wspieranym punktem wejscia.

---

## 4. Warstwa 3 — pole bitowe Fog of War (`RemoveFogOfWar`)

Gesta maska bitowa miedzy BloodStain a MenuProfile reprezentujaca stan
eksploracji per-kafelek. Edytor eksponuje dedykowana akcje uzytkownika
(`App.RemoveFogOfWar`), ktora wypelnia caly zakres wartoscia `0xFF`. Jest
**celowo oddzielniona od `RevealAllMap`** — odkrywanie tekstur mapy i
usuwanie nakladki mgly to koncepcyjnie rozne operacje i uzytkownik moze
chciec jednej bez drugiej.

### 4.1 Lokalizacja

```
afterRegs   = gesturesOff + 4 + regCount * 4
bitfield_start = afterRegs + 0x087E
bitfield_end   = afterRegs + 0x10B0      (inclusive last safe byte)
section_size   = 0x103C bytes total      (BloodStain → MenuProfile)
usable_range   = 2099 bytes (0x087E .. 0x10B0)
```

**Krytyczne:** zapis za `+0x10B0` naklada sie na `menuProfile` i
**powoduje crash gry**. Prefiks 0x0000..0x087D zawiera ustrukturyzowane
dane horse + bloodstain — rowniez nie dotykac z tej warstwy.

### 4.2 Format

Plaska maska bitowa, LSB-first w kazdym bajcie. `1` = kafelek odkryty,
`0` = kafelek ukryty. Mapowanie kafelek-do-bitu jest nieznane i nie
da sie go wywnioskowac z region ID (patrz otwarte pytania w §6).
Jeden teleport w grze przerzuca ~356 bitow w ciaglym oknie 157 bajtow.

### 4.3 Dlaczego pozostaje oddzielne od `RevealAllMap`

- Warstwa Detailed Bitmap (§2) daje graczowi *uzyteczny* sygnal
  — tekstury regionow, ikony dungeonow, posiadanie fragmentow. To jest to
  co wiekszosc uzytkownikow rozumie przez "pokaz mi mape".
- Wypelnienie pola bitowego FoW jest czysto kosmetyczne — usuwa szary
  overlay, ktory gra uzywa do oznaczenia "tu jeszcze nie chodziles". Nie
  odblokowuje nowej zawartosci; gracz moze juz uzywac mapy bez tego.
- Wlaczenie tego do `RevealAllMap` wymusiloby na uzytkownikach, ktorzy
  chcieli tylko fragmenty, utrate naturalnego sygnalu eksploracji.
  Trzymanie tego za wlasna akcja zachowuje wybor uzytkownika.
- Selektywne per-regionowe usuwanie FoW wymaga reverse-engineeringu
  mapowania bit-do-kafelka. Nie jest w roadmapie.

### 4.4 Algorytm `RemoveFogOfWar`

```go
storageEnd  := slot.StorageBoxOffset + core.DynStorageBox
gesturesOff := storageEnd + core.DynStorageToGestures
regCount    := int(binary.LittleEndian.Uint32(slot.Data[gesturesOff:]))
afterRegs   := gesturesOff + 4 + regCount*4
for i := afterRegs + 0x087E; i <= afterRegs+0x10B0; i++ {
    slot.Data[i] = 0xFF
}
```

Nadpisanie w miejscu, bez przesuwania bajtow, bez przeliczania offsetow.
Referencja: `app.go::RemoveFogOfWar`.

---

## 5. Log weryfikacji

Wyniki empiryczne z badania FoW, ktore uzasadnily podzial z §1
na niezalezne warstwy.

| # | Test | Wynik |
|---|------|-------|
| 1 | Dodanie region_id (bez flagi, bez przedmiotu) | Tekstura mapy bez zmian (regiony != widocznosc) |
| 2 | 0xFF w polu bitowym (maly zakres) + 1 region | Mgla usunieta lokalnie |
| 3 | 0xFF zapisane za `+0x10B0` (naklada sie na menuProfile) | **Crash gry** |
| 4 | 0xFF w pelnym zakresie pola bitowego, brak zmiany regionow | Cala mgla usunieta (tylko kosmetycznie) |
| 5 | Wstawienie 205 regionow przez byte-shift | **Crash gry** (slot obciety) — naprawione przejsciem na `RebuildSlot` |
| 6 | Teleport w grze (Warmaster's Shack) | Dodaje region 6101000 + ustawia 356 bitow |
| 7 | Ustawienie flagi 62xxx visible bez przedmiotu fragment | Tekstura mapy odkryta, ale gracz nie ma podpowiedzi w UI |
| 8 | Tylko flagi DLC visible, bez zapisu Cover Layer | Tekstura pojawia sie, ale czarne kafelki nadal pokrywaja obszar DLC |

### Pliki testowe (`tmp/save/`)

| Plik | Opis |
|------|------|
| `ER0000.sl2` | Oryginalny save, pelna mgla wojny, 6 regionow |
| `ER0000-fow-before.sl2` | Po edytorze (mapy + grace dodane), FoW bez zmian |
| `ER0000-from-deck.sl2` | Po graniu w grze (1 teleport), lokalna mgla usunieta |
| `ER0000-no-fow-test.sl2` | Region + czesciowe pole bitowe, mgla usunieta lokalnie |
| `ER0000-no-fow.sl2` | Pelne wypelnienie pola bitowego, cala mgla usunieta |

---

## 6. Otwarte pytania

1. **Mapowanie bit-do-kafelka** — ktore konkretne bity w polu bitowym FoW
   odpowiadaja ktorym kafelkom mapy. Nieznane; wymagaloby systematycznych
   diffow eksploracji pojedynczych obszarow.
2. **Region ID `1001000–1001002`** — wystepuja w kazdym swiezym safe'ie,
   ale nie ma ich w zadnej bazie regionow referencyjnych edytorow.
   Prawdopodobnie wewnetrzne markery startowe.
3. **Ustrukturyzowany prefiks `+0x0800..+0x087D`** — powtarzajacy sie wzorzec
   `00 00 01 80 BF FF FF FF FF 00...`. Prawdopodobnie kotwice wspolrzednych
   per-kafelek uzywane przez sciezke renderowania gry. Nie nadpisywac.

---

## 7. Referencje

- `backend/db/data/maps.go` — `MapSystem`, `MapVisible`, `MapUnsafe`,
  `MapFragmentItems`, `MapAcquired`, `IsDLCMapFlag`.
- `backend/db/data/regions.go` — pelna baza odblokowanych regionow.
- `app.go::SetMapRegionFlags`, `SetMapFlag`, `RevealAllMap`,
  `revealBaseMap`, `revealDLCMap`, `RemoveFogOfWar`,
  `ResetMapExploration` — API edycji mapy eksponowane przez Wails.
- `backend/core/writer.go::SetUnlockedRegions` — edytor listy regionow.
- `backend/core/slot_rebuild.go::RebuildSlot` — pelny serializator slota.
- `spec/11-regions.md` — format binarny listy regionow.
- `spec/29-dlc-black-tiles.md` — wspolrzedne Cover Layer (warstwa 2).
- `spec/30-slot-rebuild-research.md` — analiza slacku + sciezka od
  byte-shift do `RebuildSlot`.
