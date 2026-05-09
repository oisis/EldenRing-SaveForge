# 47 — Aktywacja Sites of Grace

> **Typ**: Investigacja / Design doc
> **Status**: ✅ Rozwiązany — zachowanie edytora potwierdzone jako poprawne; dodano wyjaśnienie w UI
> **Zakres**: Wszystkie przestrzenie identyfikatorów i pola pliku save związane z odkryciem Sites of Grace, szybką podróżą i fizycznym stanem obiektu w świecie gry.

---

## Tło

Ten dokument powstał, żeby zbadać czy `SetGraceVisited()` — ustawiające EventFlag gracji — jest wystarczające do pełnego odblokowania Site of Grace, czy też trzeba zapisywać dodatkowe pola w pliku save.

**Wniosek (2026-05-09)**: Edytor ustawia dokładnie tę samą EventFlag co gra. `LastRestedGrace` jest automatycznie zarządzane przez grę przy przybyciu do gracji. Nie istnieje żadne persystowane pole "zapalony/niezapalony" w pliku save. Obecna implementacja jest poprawna. Patrz §5 — runtime diff, który to potwierdził.

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
- Stan wizualny obiektu gracji w świecie gry (EMEVD wyprowadza stan zapalony/niezapalony z tej flagi przy ładowaniu obszaru)

**Co ta flaga NIE kontroluje:**
- Przypisanie punktu respawnu (`LastRestedGrace`) — zarządzane osobno przez grę

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
| Przykład | `1042362951` = "The First Step"; `1042362950` = "Church of Elleh" |
| Przechowywanie | Pojedyncze pole `u32` `LastRestedGrace` w `PreEventFlagsScalars` |
| Źródło | `spec/14-game-state.md`, `spec/15-event-flags.md` |

**Co BonfireId kontroluje:**
- Lokalizację respawnu (gdzie gracz budzi się po śmierci)
- Wyświetlanie "ostatnio odpoczęto przy" w menu pauzy
- Punkt kotwicy checkpointu stanu gry

**Czego BonfireId NIE robi:**
- To NIE jest lista; przechowywana jest tylko jedna wartość
- Nie ma bezpośredniego związku z EventFlag ID dla tej samej gracji
- Edytor NIE musi go ustawiać — gra zapisuje `LastRestedGrace` automatycznie przy każdym przybyciu do gracji (teleportacja lub podejście pieszo)

W kodzie nie ma publicznego mapowania EventFlag ID → BonfireId. Te dwie przestrzenie nazw są rozłączne.

---

## 2. Pola pliku save

### 2.1 Bitfield EventFlags

- Lokalizacja: `slot.Data[slot.EventFlagsOffset:]`
- Rozmiar: `0x1BF99F` bajtów (1 833 375 bajtów)
- Jeden bit na flagę; lookup BST konwertuje ID flagi → offset bajtu + indeks bitu
- **Akcja edytora**: `db.SetEventFlag(flags, graceID, true)` ustawia ten bit

Potwierdzone offsety (testowy save Church of Elleh, slot 0):

| Pole | Offset w `slot.Data` |
|---|---|
| `PreEventFlagsScalarsBase` | `0x3649A` |
| `EventFlagsOffset` | `0x364B7` |
| `EventFlagsEnd` | `0x1F5E56` |
| `LastRestedGrace` (surowe bajty) | `0x364AA` |

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

`LastRestedGrace` to jedyne pole pliku save przechowujące BonfireId. Jest to **pojedynczy skalar** — nie tablica, nie zbiór. Gra zapisuje je automatycznie przy przybyciu do gracji; edytor nie dotyka tego pola.

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
4. NIE dotyka `LastRestedGrace` (poprawne — gra zarządza tym automatycznie)
5. NIE ustawia żadnych MapFlags
6. NIE ustawia żadnych danych indeksowanych BonfireId

Jest to **identyczne** z wszystkimi trzema implementacjami referencyjnymi:

| Projekt | Implementacja |
|---|---|
| er-save-manager (Python) | `EventFlags.set_flag(event_flags, flag_id, True)` — pojedynczy bit |
| ER-Save-Editor (Rust) | Pojedyncze `u32` EventFlag ID per gracja, bez BonfireId |
| Elden-Ring-Save-Editor (Python) | `toggle_grace()`: ustawia jeden bit przy `grace["offset"] + grace["index"]` |

---

## 4. Potwierdzony model aktywacji

### Co kontroluje edytor

| Warstwa | Kontrolowana przez | Jak |
|---|---|---|
| Znacznik na mapie | Grace EventFlag (71xxx–76xxx) | `SetGraceVisited()` → `SetEventFlag()` |
| Wpis na liście szybkiej podróży | Grace EventFlag | jw. |
| Stan "odkryty" dla questów | Grace EventFlag | jw. |
| Drzwi wejściowe dungeonu | DoorFlag (towarzysząca EventFlag) | `SetGraceVisited()` dla gracji Cat/HG |
| Stan wizualny obiektu w świecie gry | EMEVD runtime, wyprowadzany z EventFlag | niepersystowany; edytor nic nie robi |
| Punkt respawnu (`LastRestedGrace`) | Gra, automatycznie przy przybyciu | edytor nie ustawia; gra zapisuje sama |

### Potwierdzone wartości dla Church of Elleh

| Element | Wartość |
|---|---|
| Grace EventFlag ID | `76100` (0x00012944) |
| The First Step BonfireId | `1042362951` (0x3E213247) |
| Church of Elleh BonfireId | `1042362950` (0x3E213246) |

### Dodatkowe flagi zaobserwowane przy fizycznym dotknięciu i teleportacji

| Flaga | Fizyczne dotknięcie | Teleportacja | Prawdopodobne znaczenie |
|---|---|---|---|
| `69300` | ✅ | ✅ | Trigger ładowania obszaru (wejście w rejon Church of Elleh) |
| `78101` | ✅ | ✅ | Trigger ładowania obszaru (wejście w rejon Church of Elleh) |
| `69070` | ✅ | ❌ | Trigger fizycznej bliskości — NPC/cutscena (Kalé, Ranni), NIE zapalenie gracji |

Żadna z tych flag nie jest wymagana po stronie edytora do odblokowania gracji (znacznik + fast travel).

### Werdykty hipotez

| Hipoteza | Werdykt |
|---|---|
| A — EMEVD re-triggeruje z EventFlag przy ładowaniu obszaru | ✅ **Potwierdzony główny mechanizm** |
| B — Ukryta towarzysząca EventFlag kontroluje stan wizualny | ❌ Wykluczona — 69070 to tylko trigger bliskości NPC |
| C — Flaga geometrii WorldGeomMan persystuje stan wizualny | ❌ Brak dowodów w kontrolowanych diffach |
| D — Stan obiektu gracji jest w pełni runtime, niepersystowany | ✅ **Potwierdzony** |

---

## 5. Runtime Save Diff: Church of Elleh

> Ukończono 2026-05-09. Porównano pięć plików save: `vanilla` / `A` (edytor) / `B` (fizyczne dotknięcie) / `C` (odpoczynek) / `D` (teleportacja do gracji odblokowanej przez edytor).

### Macierz obecności flag

| Flaga | vanilla | A (edytor) | B (dotknięcie) | C (odpoczynek) | D (teleport) |
|---|---|---|---|---|---|
| **76100** Grace EventFlag | 0 | **1** | **1** | 1 | 1 |
| 62120 Mapa Stormveil Castle | 0 | 0 | 1 | 1 | 1 |
| 69070 Nieznana (trigger NPC?) | 0 | 0 | **1** | 1 | **0** |
| 69300 Nieznana (area-load) | 0 | 0 | 1 | 1 | 1 |
| 78101 Nieznana (area-load) | 0 | 0 | 1 | 1 | 1 |

Flagi 62001 / 82001 / 82002 (wyświetlanie mapy podziemnej) pojawiły się w save A, bo użytkownik uruchomił też `RevealBaseMap` — NIE są ustawiane przez `SetGraceVisited()`.

### `PreEventFlagsScalars` w kolejnych save'ach

| Skalar | vanilla | A | B | C | D |
|---|---|---|---|---|---|
| `LastRestedGrace` | 1042362951 | 1042362951 | **1042362950** | 1042362950 | **1042362950** |
| `UnkGameDataMan0x124` | 61 | 61 | 61 | **73** | **40** |

`LastRestedGrace` przechodzi z The First Step na Church of Elleh zarówno w B (fizyczne dotknięcie) jak i D (teleportacja). Gra ustawia to automatycznie; edytor nic nie musi robić.

### Drugie wystąpienie BonfireId

BonfireId znaleziono też pod `slot.Data[0x1F636A]` — 1 300 bajtów za końcem EventFlags, prawdopodobnie wczesna sekcja NetworkManager. Aktualizuje się identycznie z `LastRestedGrace`. Prawdopodobnie używane do synchronizacji respawnu w trybie multiplayer.

### Kluczowy wniosek

Flaga 76100 jest **identyczna** w A (edytor) i B (gra). Edytor ustawia dokładnie to samo co gra. Żadne dodatkowe pola pliku save nie są wymagane dla warstwy odblokowania mapy/fast travel.

---

## 6. Skrypt diagnostyczny

`tmp/scripts/diag/grace_activation_diff.go` — tylko do odczytu, `//go:build ignore`.

**Użycie:**
```
go run tmp/scripts/diag/grace_activation_diff.go \
  -before tmp/site-of-grace-debug/ER0000-kro55-vanilla.sl2 \
  -after  tmp/site-of-grace-debug/ER0000-b.sl2 \
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

Pliki save: `tmp/site-of-grace-debug/` (vanilla, A, B, C, D).  
Raport analizy: `tmp/site-of-grace-debug/grace-activation-analysis.md`.

---

## 7. Modele naprawy

### Model 1 — Brak zmian backendu ✅ Zaimplementowany

`SetGraceVisited()` jest poprawne. Żadnych zmian logiki.

### Model 2 — Wyjaśnienie w UI ✅ Zaimplementowany

Krótka notka dodana do sekcji Sites of Grace w zakładce `WorldTab`:

> "Gracje odblokowane tutaj pojawią się na mapie i będą dostępne do szybkiej podróży. Odpoczynek w grze nadal kontroluje normalny punkt odrodzenia/odpoczynku."

### Model 3 — Opcjonalnie: zbadanie flagi 69070

Flaga 69070 jest ustawiana tylko przy fizycznym podejściu (nie przez edytor ani teleportację). Jeśli użytkownicy zgłoszą, że cutsceny NPC przy Church of Elleh (powitanie Kalé, pierwsze pojawienie Ranni) nie triggerują po przybyciu przez fast travel z gracji odblokowanej edytorem, ustawienie 69070 razem z EventFlag gracji może to naprawić. Nie jest wymagane do samego odblokowania gracji.

**Odrzucone podejścia:**
- Ustawianie `LastRestedGrace` — zbędne; gra zarządza tym automatycznie przy przybyciu
- Budowanie tablicy lookup EventFlag ID → BonfireId — zbędne
- Szukanie ukrytej towarzyszącej flagi aktywacji gracji — wykluczone przez runtime diff

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
| `tmp/site-of-grace-debug/grace-activation-analysis.md` | Pełny raport analizy runtime diff |
