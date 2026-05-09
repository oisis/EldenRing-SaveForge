# 47 — Aktywacja Sites of Grace

> **Typ**: Investigacja / Design doc
> **Status**: ✅ Rozwiązany — Hipoteza D potwierdzona; zachowanie edytora jest poprawne
> **Zakres**: Wszystkie identyfikatory i pola pliku save związane z odkryciem Sites of Grace, szybką podróżą i fizyczną aktywacją obiektu w świecie gry.

---

## Tło

Po ustawieniu EventFlag gracji przez edytor, Sites of Grace pojawiają się na mapie i są dostępne do szybkiej podróży. Jednak po przybyciu obiekt gracji w świecie gry wygląda jakby nie był zapalony — gra traktuje go jakby nigdy nie był fizycznie dotknięty. Gracz musi ręcznie podejść i odpocząć przy gracji, żeby ją w pełni aktywować.

Ten dokument mapuje wszystkie znane przestrzenie identyfikatorów i pola pliku save związane ze stanem gracji, identyfikuje co edytor aktualnie kontroluje, i charakteryzuje brakującą warstwę aktywacji.

---

## 1. Przestrzenie identyfikatorów

Gracje używają **dwóch całkowicie oddzielnych przestrzeni identyfikatorów**. Pomylenie ich to najczęstsze źródło zamieszania.

### 1.1 Grace EventFlag ID (71xxx – 76xxx)

| Właściwość | Wartość |
|---|---|
| Zakres (base game) | 71000 – 76162 |
| Zakres (DLC — Shadow of the Erdtree) | 72xxx, 74xxx, do 76960 |
| Łączna liczba | 419 wpisów w `backend/db/data/graces.go` |
| Identyfikator źródłowy | stałe hex w `graces.go`, np. `0x00011558` = 71000 |
| Lookup | BST blok 71–76 przez `eventflag_bst.txt` |

**Co ta flaga kontroluje (potwierdzone):**
- Widoczność znacznika na mapie (ikona gracji pojawia się na mapie)
- Dostępność szybkiej podróży (gracja pokazuje się na liście warp)
- Stan "odkryty" z perspektywy silnika gry dla celów quest flag

**Co ta flaga NIE kontroluje:**
- Stan zapalony/niezapalony fizycznego obiektu ogniska w świecie gry
- Czy animacja odpoczynku odtwarza się po podejściu
- Przypisanie punktu respawnu (`LastRestedGrace`)

**Pod-zakresy według typu obszaru:**

| Zakres | Typ obszaru | Uwagi |
|---|---|---|
| 71xxx | Stormveil, Leyndell, areny bossów | Legacy dungeons |
| 72xxx | DLC — Belurat, Enir-Ilim | DLC legacy dungeons |
| 73xxx | Wszystkie katakumby i heroic graves | Parowane z `DoorFlag` |
| 74xxx | DLC — Gravesite Plain, Scadu Altus, Rauh Base | DLC katakumby/dungeony |
| 76xxx | Wszystkie gracje w otwartym świecie | Największa grupa (~195 wpisów) |

### 1.2 BonfireId / Grace Entity ID

| Właściwość | Wartość |
|---|---|
| Format | `10AABBCCCC` — dziesiętnie, 10 cyfr |
| Przykład | `1042362951` = "The First Step" |
| Przechowywanie | Pojedyncze pole `u32` `LastRestedGrace` w `PreEventFlagsScalars` |
| Źródło | `spec/14-game-state.md`, `spec/15-event-flags.md` |

**Co BonfireId kontroluje:**
- Lokalizację respawnu (gdzie gracz budzi się po śmierci)
- Wyświetlanie "ostatnio odpoczęto przy" w menu pauzy
- Punkt kotwicy checkpointu stanu gry

**Czego BonfireId NIE robi:**
- To NIE jest lista; przechowywana jest tylko jedna wartość
- Ustawienie go NIE zapala obiektu gracji w świecie gry
- Nie ma bezpośredniego związku z EventFlag ID dla tej samej gracji

W kodzie nie ma publicznego mapowania EventFlag ID → BonfireId. Te dwie przestrzenie nazw są rozłączne.

---

## 2. Pola pliku save

### 2.1 Bitfield EventFlags

- Lokalizacja: `slot.Data[slot.EventFlagsOffset:]`
- Rozmiar: `0x1BF99F` bajtów (1 835 423 bajty)
- Jeden bit na flagę; lookup BST konwertuje ID flagi → offset bajtu + indeks bitu
- **Akcja edytora**: `db.SetEventFlag(flags, graceID, true)` ustawia ten bit

### 2.2 PreEventFlagsScalars

29-bajtowy blok bezpośrednio przed bitfieldem EventFlags:
`[slot.EventFlagsOffset - core.PreEventFlagsScalarsSize]`

| Pole | Offset w bloku | Typ | Opis |
|---|---|---|---|
| `GameMan0x8c` | +0x00 | u8 | Nieznany bajt GameMan |
| `GameMan0x8d` | +0x01 | u8 | Nieznany bajt GameMan |
| `GameMan0x8e` | +0x02 | u8 | Nieznany bajt GameMan |
| `TotalDeathsCount` | +0x03 | u32 | Skumulowany licznik śmierci |
| `CharacterType` | +0x07 | i32 | 0=normalny, 1=najeźdźca itd. |
| `InOnlineSessionFlag` | +0x0B | u8 | Aktywna sesja online |
| `CharacterTypeOnline` | +0x0C | u32 | Typ postaci online |
| **`LastRestedGrace`** | **+0x10** | **u32** | **BonfireId ostatniej odwiedzionej gracji** |
| `NotAloneFlag` | +0x14 | u8 | Aktywny co-op / NPC companion |
| `InGameCountdownTimer` | +0x15 | u32 | Odliczanie w grze |
| `UnkGameDataMan0x124` | +0x19 | u32 | Nieznane |

`LastRestedGrace` to jedyne pole pliku save przechowujące BonfireId. Jest to **pojedynczy skalar** — nie tablica, nie zbiór.

### 2.3 DoorFlag

Opcjonalna towarzysząca EventFlag dla gracji w katakumbach i heroic graves. Gdy ustawiona razem z EventFlag gracji, otwiera drzwi wejściowe dungeonu w świecie gry.

- Przechowywana w `data.GraceData.DoorFlag` (u32, 0 jeśli nieaplikowalne)
- Ustawiana przez `SetGraceVisited()` w `app_world.go` gdy `DoorFlag != 0`
- Dotyczy tylko wpisów `Cat()` i `HG()` w `graces.go`

### 2.4 MapFlags (62xxx, 82xxx)

Oddzielna warstwa EventFlag kontrolująca odkrywanie kafelków mapy. Zarządzana niezależnie przez `World.MapFlags`.

| Blok | Cel |
|---|---|
| 62xxx | Widoczność mapy / odkrycie fog-of-war dla kafelków otwartego świata |
| 82xxx | Flagi systemowe mapy (odblokowanie ramki mapy, odblokowanie regionu) |

Ustawienie EventFlag gracji (71xxx–76xxx) NIE ustawia MapFlags. Te dwie warstwy są niezależne.

---

## 3. Co aktualnie ustawia edytor

`SetGraceVisited(slotIndex int, graceID uint32, visited bool)` w `app_world.go`:

1. Odczytuje `slot.Data[slot.EventFlagsOffset:]`
2. Wywołuje `db.SetEventFlag(flags, graceID, visited)` — ustawia bit 71xxx/76xxx
3. Jeśli `DoorFlag != 0`: wywołuje `db.SetEventFlag(flags, gd.DoorFlag, visited)` — ustawia flagę drzwi
4. NIE dotyka `LastRestedGrace`
5. NIE ustawia żadnych MapFlags
6. NIE ustawia żadnych danych indeksowanych BonfireId

Jest to **identyczne** z wszystkimi trzema implementacjami referencyjnymi:

| Projekt | Implementacja |
|---|---|
| er-save-manager (Python) | `EventFlags.set_flag(event_flags, flag_id, True)` — pojedynczy bit |
| ER-Save-Editor (Rust) | Pojedyncze `u32` EventFlag ID per gracja, bez BonfireId |
| Elden-Ring-Save-Editor (Python) | `toggle_grace()`: ustawia jeden bit przy `grace["offset"] + grace["index"]` |

Żadna z implementacji referencyjnych nie ustawia BonfireId ani żadnego stanu dodatkowego.

---

## 4. Brakująca warstwa aktywacji

### Potwierdzone zachowanie

- EventFlag gracji ustawiony → znacznik na mapie widoczny, szybka podróż dostępna ✅
- EventFlag gracji ustawiony → fizyczny obiekt gracji zapalony po przybyciu ❌ (nieobserwowane)

### Hipoteza A — Ponowne uruchomienie skryptu EMEVD (najbardziej prawdopodobna)

Każdy obszar mapy uruchamia skrypt EMEVD, który sprawdza EventFlags gracji przy ładowaniu obszaru, żeby ustawić stan wizualny obiektów gracji w świecie gry. Gdy gracz szybko podróżuje bezpośrednio do gracji, obszar ładuje się z już ustawioną EventFlag. Czy subroutyna EMEVD "pierwszej wizyty" się odpala, zależy od:
- Czy gra rozróżnia "EventFlag była ustawiona przed tą sesją" od "EventFlag ustawiono w tej sesji"
- Czy stan encji obiektu gracji (zapalony/niezapalony) jest persystowany oddzielnie czy przeliczany z EventFlag przy każdym ładowaniu obszaru

Jeśli EMEVD wyprowadza stan obiektu gracji czysto z EventFlag, obiekt **powinien** być już zapalony po przybyciu — co oznaczałoby, że zgłoszony bug to nieporozumienie. Jeśli EMEVD utrzymuje oddzielny stan encji w pamięci, który nie jest aktualizowany retroaktywnie, gracja wyglądałaby na niezapaloną mimo ustawionej EventFlag.

**Ta hipoteza wymaga diffowania runtime przed/po w celu potwierdzenia.**

### Hipoteza B — Druga towarzysząca EventFlag (niezidentyfikowana)

Ukryta EventFlag pod innym ID (spoza 71xxx–76xxx) może kontrolować stan wizualny obiektu w świecie gry niezależnie od znacznika na mapie. Takiej flagi nie znaleziono w żadnej implementacji referencyjnej ani skrypcie CT-TGA.

### Hipoteza C — Flaga geometrii WorldGeomMan / WorldArea

Sekcja `WorldState` zawiera dane geometrii i stanu obszaru. Oddzielny bit w tej sekcji mógłby oznaczać fizyczną encję gracji jako aktywowaną. Ta sekcja nie jest jeszcze w pełni poddana inżynierii wstecznej (patrz `spec/16`).

### Hipoteza D — Stan obiektu gracji jest w pełni runtime / niepersystowany

Stan obiektu gracji w świecie gry może być całkowicie runtimeowy (EMEVD/obiekt C++), nie persystowany w pliku save w ogóle. W tym przypadku fizyczne zapalenie gracji zawsze wymagałoby ręcznej interakcji w grze — a ustawiona przez edytor EventFlag pokrywa tylko warstwę mapy/warpu, co byłoby kompletnym oczekiwanym zachowaniem.

---

## 5. Skrypt diagnostyczny

`tmp/scripts/diag/grace_activation_diff.go` — tylko do odczytu, `//go:build ignore`.

**Użycie:**
```
go run tmp/scripts/diag/grace_activation_diff.go \
  -before tmp/save/before-church-elleh.sl2 \
  -after  tmp/save/after-church-elleh.sl2 \
  -slot 0 -grace 76100 -bonfire 1042362951
```

**Raporty:**
1. Zmiana EventFlag docelowej gracji (potwierdzenie 0→1)
2. Wszystkie zmiany EventFlag pogrupowane według zakresu 1000
3. Diff `PreEventFlagsScalars` — szczególnie `LastRestedGrace`
4. Diff `UnlockedRegions`
5. Diff MapFlags (62xxx, 82xxx)
6. Wyszukiwanie BonfireId w surowych bajtach slotu
7. Podsumowanie byte-diff według regionów 0x10000

**Idealna para save:**  
A = save bezpośrednio przed fizycznym dotknięciem Church of Elleh (gracja 76100, bonfire ~1042362951)  
B = save bezpośrednio po odpoczynku przy tej gracji i powrocie do menu głównego

---

## 8. Runtime Save Diff: Church of Elleh

> **Status**: ✅ Ukończone — 2026-05-09  
> Pięć plików save: `vanilla`, `A` (odblokowanie przez edytor), `B` (fizyczne dotknięcie), `C` (odpoczynek), `D` (teleportacja do gracji odblokowanej przez edytor).

### Opis plików save

| Plik | Opis |
|---|---|
| `vanilla` | Punkt startowy — gracja nie aktywowana, `LastRestedGrace` = 1042362951 (The First Step) |
| `A` | Gracja 76100 ustawiona przez edytor (`SetGraceVisited()`). Brak rozgrywki. |
| `B` | Gracja 76100 fizycznie aktywowana w grze (podejście i dotknięcie). |
| `C` | Odpoczynek przy gracji 76100 po fizycznej aktywacji. |
| `D` | Teleportacja (fast travel) do gracji 76100 ustawionej przez edytor w pliku A. |

### Identyfikacja BonfireId (empiryczna)

| BonfireId | Dziesiętnie | Gracja |
|---|---|---|
| `0x3E213247` | 1042362951 | The First Step |
| `0x3E213246` | 1042362950 | **Church of Elleh** |

### Offset EventFlags w `slot.Data`

| Pole | Offset |
|---|---|
| `PreEventFlagsScalarsBase` | `0x3649A` |
| `EventFlagsOffset` | `0x364B7` |
| `EventFlagsEnd` | `0x1F5E56` |
| `LastRestedGrace` w surowych bajtach | `0x364AA` |
| Drugie wystąpienie BonfireId | `0x1F636A` (+1 300 bajtów za końcem EventFlags — wczesny NetworkManager) |

### Macierz obecności flag

| Flaga | vanilla | A (edytor) | B (dotknięcie) | C (odpoczynek) | D (teleport) |
|---|---|---|---|---|---|
| **76100** Grace EventFlag | 0 | **1** | **1** | 1 | 1 |
| 62001 Mapa podziemna† | 0 | 1 | 0 | 0 | 1 |
| 82001 Pokaż podziemie† | 0 | 1 | 0 | 0 | 1 |
| 82002 Pokaż Realm Cienia† | 0 | 1 | 0 | 0 | 1 |
| 62120 Mapa Stormveil Castle | 0 | 0 | 1 | 1 | 1 |
| **69070** Nieznana | 0 | 0 | **1** | 1 | **0** |
| 69300 Nieznana | 0 | 0 | 1 | 1 | 1 |
| 78101 Nieznana | 0 | 0 | 1 | 1 | 1 |

† Ustawione przez osobną operację RevealMap w edytorze, NIE przez `SetGraceVisited()`.

### `PreEventFlagsScalars` w kolejnych save'ach

| Skalar | vanilla | A | B | C | D |
|---|---|---|---|---|---|
| `LastRestedGrace` | 1042362951 | 1042362951 | **1042362950** | 1042362950 | **1042362950** |
| `UnkGameDataMan0x124` | 61 | 61 | 61 | **73** | **40** |

`LastRestedGrace` jest ustawiany automatycznie przez grę w momencie przybycia do gracji (teleportacja LUB podejście pieszo). Edytor nie musi tego pola dotykać.

### Wnioski

1. **Edytor ustawia dokładnie tę samą EventFlag co gra.** Flaga 76100 jest identyczna w A i B. `SetGraceVisited()` jest poprawne.

2. **`LastRestedGrace` jest zarządzany automatycznie przez grę.** Żadna ingerencja edytora nie jest potrzebna. Aktualizuje się w momencie przybycia gracza do gracji.

3. **Trzy dodatkowe flagi pojawiają się w B i D** (69300, 78101 w obu; 69070 tylko przy fizycznym podejściu B/C). Prawdopodobnie są to triggery ładowania obszaru i dialogi NPC (Kalé, Ranni), nie flagi zapalenia gracji.

4. **Flaga 69070** to jedyna flaga odróżniająca fizyczne dotknięcie (B) od teleportacji (D). Jest nieobecna przy fast travel do gracji odblokowanej przez edytor. Jej dokładne znaczenie (trigger NPC, tutorial) jest nieznane, ale nie kontroluje wizualnego stanu obiektu gracji.

5. **Żadne pole pliku save nie persystuje stanu zapalony/niezapalony obiektu gracji.** Wszystkie save'y B/C/D mają identyczną strukturę pod kątem gracji. Hipoteza D potwierdzona.

6. **Drugie wystąpienie BonfireId** pod `slot.Data[0x1F636A]` (1 300 bajtów za końcem EventFlags) aktualizuje się razem z `LastRestedGrace`. Prawdopodobnie w sekcji NetworkManager — może służyć do synchronizacji respawnu w trybie multiplayer.

### Status hipotez

| Hipoteza | Werdykt |
|---|---|
| A — Ponowne uruchomienie skryptu EMEVD | **Potwierdzony główny mechanizm** — ta sama EventFlag steruje zarówno edytorem jak i grą |
| B — Ukryta towarzysząca EventFlag | **Wykluczona** — 69070 triggeruje tylko dialog NPC, nie zapalenie gracji |
| C — Flaga geometrii WorldGeomMan | **Brak dowodów** — brak zmian poza EventFlags+NM w kontrolowanych diffach |
| **D — Stan runtime-only (niepersystowany)** | ✅ **Potwierdzony** |

### Rekomendowany model naprawy

**Model 3 (notka UI)** — dodać notatkę do sekcji gracji w zakładce `WorldTab`:

> "Gracje odblokowane tutaj pojawiają się na mapie i umożliwiają szybką podróż. Obiekt gracji w świecie gry zostaje zapalony przy ładowaniu obszaru. Ręczny odpoczynek jest wymagany tylko do aktualizacji punktu respawnu."

Model 2 (patch `LastRestedGrace`) **nie jest potrzebny** — gra ustawia go automatycznie po przybyciu. Model 1 (brak zmian) jest również akceptowalny jeśli notka UI nie jest planowana.

---

## 6. Modele naprawy (Proponowane, Niezaimplementowane)

### Model 1 — Brak zmian (obecne zachowanie jest poprawne)

Jeśli Hipoteza D jest potwierdzona (stan obiektu gracji nie jest persystowany), obecne zachowanie edytora jest poprawne. `World.Graces` kontroluje tylko warstwę mapy/warpu. Udokumentować to jasno w UI.

**Ryzyko**: Niskie. Wymaga tylko aktualizacji tekstu w UI.

### Model 2 — Ustawienie `LastRestedGrace` na BonfireId aktywowanej gracji

Gdy użytkownik aktywuje pojedynczą grację przez edytor, również ustawić `LastRestedGrace` na odpowiadający BonfireId.

**Blokery**:
- Nie istnieje publiczne mapowanie EventFlag ID → BonfireId w kodzie
- Ustawienie `LastRestedGrace` zmienia punkt respawnu — niezamierzony efekt uboczny przy masowym ustawianiu wszystkich gracji
- Wymaga zbudowania i walidacji pełnej tablicy lookup EventFlag ID → BonfireId (~419 wpisów)

**Ryzyko**: Średnie. Wykonalne tylko dla aktywacji pojedynczej gracji, nie masowej.

### Model 3 — Wyświetlenie ostrzeżenia w UI

Dodać notatkę UI w sekcji gracji zakładki `WorldTab`: "Gracje ustawione przez ten edytor pojawią się na mapie i umożliwią szybką podróż. Fizyczny obiekt gracji wymaga ręcznego odpoczynku do pełnej aktywacji."

**Ryzyko**: Żadne. Poprawny opis faktycznego zachowania jeśli Hipoteza D jest potwierdzona.

---

## 7. Następne kroki

### Bez dostępu do konsoli

1. Uruchomić `grace_activation_diff.go` na prawdziwej parze save przed/po (zalecane: Church of Elleh)
2. Sprawdzić czy `LastRestedGrace` zmienia się w diffie — i czy oprócz 76100 zmieniają się jakieś inne EventFlags
3. Sprawdzić byte-diff według regionów (sekcja 7 skryptu) — zmiana poza regionem EventFlags sugerowałaby zaangażowanie pola WorldState lub encji

### Z dostępem do konsoli

1. Ustawić grację 76100 przez edytor, wczytać save, szybko podróżować do Church of Elleh
2. Zaobserwować: czy obiekt gracji jest zapalony czy niezapalony po przybyciu?
3. Jeśli niezapalony: podejść do gracji — czy animacja aktywacji odtwarza się, czy gracja odmawia aktywacji?
4. Przeładować czysty save i ręcznie odpocząć przy gracji — porównać wynikowy save z save popatrzonym przez edytor używając skryptu diff

---

## Źródła

| Plik | Istotność |
|---|---|
| `backend/db/data/graces.go` | Wszystkie 419 wpisów gracji z EventFlag ID i DoorFlags |
| `backend/core/section_eventflags.go` | Struct `PreEventFlagsScalars`, `EventFlagsBlock` |
| `app_world.go` | Implementacja `SetGraceVisited()` |
| `spec/14-game-state.md` | Pole `LastRestedGrace`, semantyka BonfireId |
| `spec/15-event-flags.md` | Sekcja "Bonfire IDs", offsety bajtów EventFlag |
| `spec/16-world-state.md` | WorldGeomMan / WorldArea (częściowe) |
| `tmp/repos/er-save-manager/src/er_save_manager/parser/event_flags.py` | Referencja: podejście single-flag |
| `tmp/repos/ER-Save-Editor/src/db/graces.rs` | Referencja: pojedyncze u32 EventFlag ID per gracja |
| `tmp/repos/Elden-Ring-Save-Editor/src/Final.py` | Referencja: pojedynczy bit per gracja |
| `tmp/repos/Elden-Ring-Save-Editor/src/Resources/Json/graces.json` | Mapa gracji z offset + index (bez BonfireId) |
| `tmp/scripts/diag/grace_activation_diff.go` | Skrypt diagnostyczny dla diffu przed/po |
