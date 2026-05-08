# 46 — Badanie Szybszych Inwazji

> **Typ**: Design doc
> **Status**: ✅ Badanie zakończone
> **Zakres**: Kompleksowe badanie mechaniki matchmakingu inwazji PS4/PC Elden Ring przez
> analizę pliku save: patchowanie NetworkParam w UD11, stan sesji UD10, binarna struktura
> MatchmakingCandidateSection w UD0 oraz finalny model automatu stanów.

---

## Architektura pliku save

Elden Ring `.sl2` / `.dat` to kontener BND4 z dokładnie 12 sekcjami:

```
UserData0–9   — sloty postaci (dane save per postać, 0x280000 bajtów każdy)
UserData10    — dane systemowe (0x60000 bajtów / 384 KB)
UserData11    — snapshot regulation.bin (tabele parametrów po stronie serwera)
```

Bazowe offsety (bezwzględne w pliku):

```
UD0  = 0x70
UD1  = 0x70 + 0x280000
...
UD10 = 0x70 + 10 × 0x280000 = 0x1900070
UD11 = 0x70 + 11 × 0x280000 = 0x2180070
```

---

## Analizowane save'y

| Etykieta | Plik | Stan sesji |
|----------|------|------------|
| H | `oisis_pl-vanilla-nopvpactivity.dat` | vanilla, pasywny/gotowy na sesję |
| J | `oisis_pl-vanilla-bf-on.dat` | vanilla, BF-init (kolejka aktywna) |
| F | `oisis_pl-pvp-ready-netparam-finger-on.dat` | UD11 spatchowany, pełny aktywny BF |
| I | `oisis_pl-vanilla-invasion-timeout.dat` | vanilla, inwazja zakończona/wyczyszczona |
| G | `oisis_pl-pvp-ready-netparam-invasion-timeout-break.dat` | UD11 spatchowany, timeout/wyczyszczony |
| E | `oisis_pl-pvp-ready-netparam-nopvpactivity.dat` | UD11 spatchowany, bezczynny (niewyjaśniony stan UD10) |

Skrypty: `tmp/scripts/diag/bf_statemachine.go`, `bf_targetlist.go`, `bf_candidatelist.go`
Surowe wyniki: `tmp/regulation-bin-debug/final-report.md`

---

## 1. Wyjściowa hipoteza

Cel: znaleźć parametry w pliku save, które przyspieszą lub zwiększą częstotliwość inwazji PvP.

**Hipoteza robocza**: `NetworkParam` wewnątrz `regulation.bin` (UserData11) kontroluje
czasowanie inwazji. Edycja `breakInRequestIntervalTimeSec` (30s → 5s) i
`breakInRequestTimeOutSec` (20s → 5s) powinna przyspieszyć inwazje.

**Hipoteza pomocnicza**: wartości NetworkParam mogą być cachowane w UD10, co pozwoliłoby
na edycję wyłącznie pliku save bez dotykania UD11.

**Ścieżka badania**:

1. Pełny skan UD10 pod kątem sygnatur NetworkParam → zmapowane stabilne regiony, brak trwałego NetworkParam
2. Struktura NetMan wewnątrz slotów postaci → cache historii tylko do odczytu, niekonfigorowalny
3. EventFlags → flagi questów/obszarów, nie parametry czasowe
4. UD11 regulation.bin → wartości zapisywalne, ale efekt runtime nieokreślony (patrz §3)
5. Automat stanów BF w UD10 → odkryto rzeczywisty stan sesji (`UD10+0x5070`)
6. Skan binarny UD0 → znaleziono `MatchmakingCandidateSection` pod `UD0+0x209B00..0x209C43`

---

## 2. Dekompozycja UD11

### Struktura pliku

**PC** (`.sl2`):

```
ud11[0x00:0x10]   suma kontrolna MD5 z ud11[0x10:]
ud11[0x10:]       regulation.bin zaszyfrowany AES-256 + skompresowany DCX
```

**PS4** (`.dat`):

```
ud11[0x00:0x10]   16-bajtowy nagłówek (NIE jest MD5)
ud11[0x10:]       regulation.bin skompresowany DCX (bez warstwy AES)
```

### Weryfikacja poprawności (PC)

| Plik | prefiks MD5 | MD5(ud11[0x10:]) | Zgodność? |
|------|-------------|------------------|-----------|
| vanilla | `7256cc79…` | `7256cc79…` | ✅ |
| spatchowany, bez przeliczenia MD5 | `7256cc79…` | `317a411a…` | ❌ → gra przywraca |
| nadpisany przez grę po patchu | `7256cc79…` | `7256cc79…` | ✅ |

**Przyczyna starego błędu patcha PC**: `PatchNetworkParams` szyfrował ciphertext ponownie,
ale zostawiał oryginalny prefiks MD5. Gra wykrywała niezgodność i przywracała lokalną kopię.
**Fix**: przeliczenie `ud11[0x00:0x10] = MD5(ud11[0x10:])` po patchowaniu.
Zaimplementowane w `backend/core/regulation.go` (2026-05-08). → spec/44

**PS4**: spatchowane wartości przeżywają cykl upload/download i są odczytywalne po pobraniu
z konsoli. Wcześniejsze założenie, że „serwer zawsze nadpisuje", było nieweryfikowane i
zostało usunięte.

### Zawartość regulation.bin

Wewnątrz bloba DCX: kontener BND4 z tabelami `.param`.
`NetworkParam.param` (jeden wiersz) zawiera całą konfigurację sieci.

Kluczowe pola użyte w tym badaniu:

| Parametr | Offset CSV | Vanilla | Spatchowany | Opis |
|----------|-----------|---------|-------------|------|
| `maxBreakInTargetListCount` | `0x70` | 5 | 10 | Rozmiar listy celów w pamięci; >200 crashuje klienta |
| `breakInRequestIntervalTimeSec` | `0x74` | 30.0 | 5.0 | Jak często gra wysyła żądanie inwazji [s] |
| `breakInRequestTimeOutSec` | `0x78` | 20.0 | 5.0 | Oczekiwanie na odpowiedź serwera przed retry [s] |
| `breakInRequestAreCount` | TBD | ? | ~10 | Regiony przeszukiwane per próba |

Offset `breakInRequestAreCount` niezidentyfikowany w `tmp/regulation-bin-dump/csv/NetworkParam.csv`.

---

## 3. Test NetworkParam

### Skan UD10 pod kątem wartości NetworkParam

Pełny skan dwóch stabilnych regionów UD10 pod kątem `f32=30.0`, `f32=20.0`, `f32=5.0`:

- Rozproszone trafienia istnieją w lotnym UD10 (`0x00C5F8`, `0x020BE8`) — resetują się do `0.0`
  gdy gra nadpisuje UD10 przy następnym save.
- **Stabilne regiony nie zawierają trwałej kopii NetworkParam.**

### Porównanie między save'ami (spatchowany vs vanilla)

Save'y ze spatchowanym UD11 (F, G, E) vs save'y z vanilla UD11 (H, I, J):

- Słowo stanu `UD10+0x5070`: brak różnic przypisywalnych wartościom NetworkParam
- Konfiguracja kolejki V w UD0: identyczny układ dla równoważnych stanów BF
- Spatchowane save'y wykazują te same przejścia automatu stanów co vanilla

**Wynik**: brak obserwowalnej różnicy w UD0 lub UD10 między spatchowanym a vanilla UD11
w równoważnych stanach sesji.

### Struktura NetMan (dowód pomocniczy)

NetMan to struktura 131 076 bajtów wewnątrz każdego slotu postaci. Jest **cache historii
tylko do odczytu** — nie obszarem konfiguracyjnym.

```
NetMan total = 0x20004 bajtów
  ├── unk0x0       4 bajty   zawsze 2
  └── data     0x20000 bajtów
        ├── header      0x0A0 bajtów
        ├── records   0x134A0 bajtów  (128 × 0x268 rekordów spotkań z graczami)
        └── tail      0x0CB60 bajtów  (zera)
```

Nagłówki sub-list pod `data[0x000..0x01F]`:

| Offset | Pole | Wartość | Znaczenie |
|--------|------|---------|-----------|
| `0x000` | `list0_type` | 2 | Historia znaków co-op/wezwań |
| `0x004` | `list0_capacity` | 8 | Max cachowanych wpisów |
| `0x010` | `list1_type` | **5** | Historia celów inwazji |
| `0x014` | `list1_capacity` | 8 | Max cachowanych wpisów |

`list1_type=5` klasyfikuje tę listę jako historię celów inwazji. Wartość 5 to wewnętrzny
klasyfikator — NIE ma związku z `NetworkParam.maxBreakInTargetListCount=5`.

Próby edycji (wszystkie resetowane przez grę przy następnym save):
- `list1_capacity` 8 → 10: brak efektu
- `breakInRequestIntervalTimeSec` w regionie ogona (`0x134A0+`): gra resetuje do `0.0`

**Wniosek**: NetMan to log historii. Edycja nie wpływa na zachowanie matchmakingu.

### Otwarte pytanie

Czy runtime gry czyta NetworkParam **z UD11** czy z **osobnej kopii w instalacji** —
niepotwierdzono. Są nieodróżnialne wyłącznie na podstawie pliku save. Definitywne
potwierdzenie wymaga zmierzenia rzeczywistych interwałów inwazji w grze przed i po patchu.

---

## 4. Runtime / stan sesji UD10

UD10 = 384 KB danych systemowych. ~90% jest lotne (przepisywane przy każdym save w grze).

Dwa stabilne regiony:

| Region | Zakres | Zawartość |
|--------|--------|-----------|
| Stabilny 1 | `0x000000–0x001984` | Ustawienia systemowe (grafika, audio, przełącznik matchmakingu) |
| Stabilny 2 | `0x001988–0x00511C` | Cache profilu FaceData + cache sub-area ID matchmakingu |

**`perform_matchmaking` pod `UD10[0x0013]`**: `0x01` = online, `0x00` = offline.
Jest to jedyne bezpośrednio użyteczne ustawienie sieciowe potwierdzone w pliku save.

### Markery stanu BF

| Marker | Opis |
|--------|------|
| `UD10+0x5070` | Główne słowo stanu BF (najbardziej wiarygodny wyróżnik) |
| `UD10+0x194E4` | `0xFFFFFFFF` = sentinel ACTIVE-BF (wyłącznie F) |
| `UD10+0x19504` | `0xFFFFFFFF` = sentinel PATCHED-IDLE (wyłącznie E) |
| `UD10+0x19508` | `f32=90.0` = timer okna aktywnego wyszukiwania (wyłącznie F) |
| `UD10+0x194F4` | `f32=-150.0` = licznik odliczający podczas aktywnego wyszukiwania (wyłącznie F) |
| `UD10+0x42C54` | `f32` — współrzędne lub licznik, aktywne wyszukiwanie (J/F) |
| `UD10+0x42C58` | `f32` — współrzędne lub licznik, aktywne wyszukiwanie (J/F) |
| `UD10+0x5080` | `f32=1.0` = inwazja zakończona sukcesem/wyczyszczona (wyłącznie I) |

### Pełna tabela markerów

| Marker | H | J | F | I | G | E |
|--------|---|---|---|---|---|---|
| `UD10+0x5070` | `0x0100018F` | `0x00000001` | `0x01000081` | `0x00000000` | `0x00000000` | `0x00002F60` |
| `UD10+0x194E4` | `0` | `0` | **`0xFFFFFFFF`** | `0` | `0` | `0` |
| `UD10+0x19504` | `0` | `0` | `0` | `0` | `0` | **`0xFFFFFFFF`** |
| `UD10+0x19508` | `0` | `~0` | **`f32=90.0`** | `~0` | `~0` | `0` |
| `UD10+0x194F4` | `0` | `0` | **`f32=-150.0`** | `0` | `0` | `0` |
| `UD10+0x42C54` | `0` | `f32=2.0` | `f32=-15.0` | `0` | `0` | `0` |
| `UD10+0x42C58` | `0` | `f32=1.0` | `f32=0.1` | `0` | `0` | `0` |
| `UD10+0x5080` | `0` | `~0` | `~0` | **`f32=1.0`** | `0` | `0x01000054` |

`~0` = szum float bliski zeru (nie jest wartością sygnałową).

---

## 5. MatchmakingCandidateSection w UD0

### Lokalizacja

```
UD0+0x209B00..0x209C43
Offset w pliku = baza UD0 (0x70) + 0x209B00 = 0x209B70
Rozmiar całkowity = 0x144 = 324 bajty
```

### Mapa układu

```
Offset UD0    rozmiar    podsekcja
0x209B00      0x14    Rekord nagłówka      (record_type=0x00014000, CONST)
0x209B14      0x14    Wpis A01[0]          (record_type=0x00000C00, CONST)
0x209B28      0x14    Wpis A01[1]          (record_type=0x00000C00, CONST)
0x209B3C      0x64    Blok statyczny C0-C4 (5 × 0x14, CONST)
0x209BA0      0xA0    Kolejka dynamiczna V0-V7 (8 × 0x14, zależna od stanu)
0x209C40      0x04    Terminator           (0x00000000)
```

### Struktura CandidateEntry (krok = 0x14)

```c
struct CandidateEntry {
    uint32_t record_type;  // +0x00  CONST = 0x00000C00
    uint32_t entry_id;     // +0x04  matchmaking_entry_id / candidate_id
    uint32_t flag_a;       // +0x08  klasa wpisu
    uint32_t flag_b;       // +0x0C  stan wyboru
    uint32_t flag_c;       // +0x10  sentinel pozycyjny (wyłącznie V7)
};
```

Semantyka pól:

| Pole | Wartość | Znaczenie |
|------|---------|-----------|
| `flag_a` | `0x00000A3E` | klasa target — wybrany do inwazji |
| `flag_a` | `0x00000A00` | zwykły kandydat |
| `flag_a` | `0x00000A01` | podtyp A01 (specjalny/NPC, tylko blok przed listą) |
| `flag_b` | `0x01010000` | wybrany (aktywny cel matchmakingu) |
| `flag_b` | `0x01000000` | zarejestrowany (znany sesji, niewybrany) |
| `flag_c` | `0x00FFFF00` | sentinel ogona — **zawsze na fizycznym V7**, NIE podąża za `entry_id` |
| `flag_c` | `0x00000000` | zwykła pozycja |

### Rekord nagłówka (CONST, wszystkie 6 save'ów)

```
UD0+0x209B00:  record_type=0x00014000  unk04=0x00000100  unk08=0x00000100
               unk0C=0x00000100        unk10=0x00000000
```

Tag typu `0x00014000` jest unikalny dla tego nagłówka — żaden CandidateEntry go nie używa.
`unk04/08/0C=0x00000100`: semantyka nieznana (licznik? pojemność? flagi typu?).

### Wpisy A01 (CONST, wszystkie 6 save'ów)

```
UD0+0x209B14:  id=0x12B01F00  flag_a=0x00000A01  flag_b=0x01000000  flag_c=0x00000000
UD0+0x209B28:  id=0x12B01E00  flag_a=0x00000A01  flag_b=0x01000000  flag_c=0x00000000
```

`flag_b=0x01000000` (zarejestrowany, niewybrany). Nie jest klasy target. Semantyka nieznana
(hosty NPC? specjalny typ sesji? markery obszarów?).

### Blok statyczny C0-C4 (CONST, wszystkie 6 save'ów)

```
C0  +0x209B3C  id=0x21556E00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
C1  +0x209B50  id=0x30498E00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
C2  +0x209B64  id=0x11C50E00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
C3  +0x209B78  id=0x212E5F00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
C4  +0x209B8C  id=0x212E5E00  flag_a=0x00000A3E  flag_b=0x01010000  flag_c=0x00000000
```

Wszystkie trwale oznaczone jako klasa target + wybrane (`flag_a=A3E`, `flag_b=0x01010000`).
Obecne we wszystkich stanach. Wartości `entry_id` nie mają żadnych cross-referencji poza tą sekcją.

### Kolejka dynamiczna V0-V7

Dwie konfiguracje w zależności od stanu BF:

**IDLE** (save'y H / I / G / E):

| poz | Offset UD0 | `entry_id` | `flag_a` | `flag_b` | `flag_c` |
|-----|-----------|-----------|---------|---------|---------|
| V0 | `+0x209BA0` | `0x989E2000` | `A00` | `0x01000000` | `0x00` |
| V1 | `+0x209BB4` | `0x989E2100` | `A00` | `0x01000000` | `0x00` |
| V2 | `+0x209BC8` | `0x989E2200` | `A00` | `0x01000000` | `0x00` |
| V3 | `+0x209BDC` | `0x989E2300` | `A00` | `0x01000000` | `0x00` |
| V4 | `+0x209BF0` | `0x989E2400` | `A00` | `0x01000000` | `0x00` |
| V5 | `+0x209C04` | `0x989E2500` | `A00` | `0x01000000` | `0x00` |
| V6 | `+0x209C18` | `0x989E2600` | `A00` | `0x01000000` | `0x00` |
| **V7** | **`+0x209C2C`** | **`0x3097AE00`** | **`A3E`** | **`0x01010000`** | **`0x00FFFF00`** |

**ACTIVE BF** (save'y J / F):

| poz | Offset UD0 | `entry_id` | `flag_a` | `flag_b` | `flag_c` |
|-----|-----------|-----------|---------|---------|---------|
| **V0** | **`+0x209BA0`** | **`0x3097AE00`** | **`A3E`** | **`0x01010000`** | **`0x00`** |
| V1 | `+0x209BB4` | `0x989E2600` | `A00` | `0x01000000` | `0x00` |
| V2 | `+0x209BC8` | `0x989E2500` | `A00` | `0x01000000` | `0x00` |
| V3 | `+0x209BDC` | `0x989E2400` | `A00` | `0x01000000` | `0x00` |
| V4 | `+0x209BF0` | `0x989E2300` | `A00` | `0x01000000` | `0x00` |
| V5 | `+0x209C04` | `0x989E2200` | `A00` | `0x01000000` | `0x00` |
| V6 | `+0x209C18` | `0x989E2100` | `A00` | `0x01000000` | `0x00` |
| **V7** | **`+0x209C2C`** | **`0x989E2000`** | `A00` | `0x01000000` | **`0x00FFFF00`** |

Kluczowe niezmienniki:
- `flag_c=0x00FFFF00` jest powiązany z **fizyczną pozycją V7** — NIE podąża za `0x3097AE00`
- `flag_a` i `flag_b` są **właściwościami wpisu** — podążają za `entry_id` przez wszystkie przeporządkowania
- Wpisy niebędące targetami V1-V6 w stanie ACTIVE = **dokładnie ODWRÓCONA** kolejność IDLE (nie rotacja)

### Terminator

`UD0+0x209C40`: `0x00000000` (4 bajty).
Cały zakres `0x209C40..0x209D00` jest zerowy we wszystkich 6 save'ach — brak wskaźnika head/tail.

### Skan cross-referencji

Wszystkie wartości `entry_id` z listy kandydatów wyszukano w:
- Całym UD0 (0x280000 bajtów) poza regionem listy → **zero trafień**
- Całym UD10 (0x60000 bajtów) → **zero trafień**
- UD10+0x42B00..0x42E00 (dedykowany blok wyszukiwania) → **zero trafień**

Wartości entry_id są samowystarczalne wewnątrz sekcji.

---

## 6. Finalny automat stanów

### Stany

| Stan | Save | `UD10+0x5070` | Kolejka V | Opis |
|------|------|--------------|-----------|------|
| PASSIVE | H | `0x0100018F` | IDLE | Gotowy na sesję, brak aktywnego wyszukiwania inwazji |
| BF-INIT | J | `0x00000001` | ACTIVE | Kolejka przepisana, wyszukiwanie zainicjowane |
| ACTIVE-BF | F | `0x01000081` | ACTIVE | Pełny aktywny BF, timery działają |
| SUCCESS | I | `0x00000000` | IDLE | Inwazja zakończona sukcesem |
| TIMEOUT | G | `0x00000000` | IDLE | Wyszukiwanie inwazji przekroczyło limit czasu |
| PATCHED-IDLE | E | `0x00002F60` | IDLE | Niewyjaśniony stan (wyłącznie spatchowany UD11) |

### Graf przejść

```
PASSIVE (H)
  UD10+0x5070 = 0x0100018F
  Kolejka V: IDLE — cel 0x3097AE00 @ V7
        │
        │  użyto Festering Bloody Finger
        ▼
  BF-INIT (J)
  UD10+0x5070 = 0x00000001
  Kolejka V: ACTIVE — cel 0x3097AE00 promowany V7→V0
                      pozostałe V1-V6 odwrócone
        │
        │  znaleziono match, nawiązano połączenie
        ▼
  ACTIVE-BF (F)
  UD10+0x5070 = 0x01000081
  UD10+0x194E4 = 0xFFFFFFFF
  UD10+0x19508 = f32=90.0 (timer okna wyszukiwania)
  UD10+0x194F4 = f32=-150.0 (licznik odliczający)
  Kolejka V: ACTIVE
        │
        ├────────────────────────────────────┐
        │ inwazja rozwiązana                 │ timer wygasł
        ▼                                    ▼
  SUCCESS (I)                          TIMEOUT (G)
  UD10+0x5070 = 0x00000000             UD10+0x5070 = 0x00000000
  UD10+0x5080 = f32=1.0                UD10+0x5080 = 0x00000000
  Kolejka V: IDLE (cel zresetowany do V7)   Kolejka V: IDLE (cel zresetowany do V7)
```

**PATCHED-IDLE (E)** nie pasuje do powyższego grafu. `UD10+0x5070=0x00002F60` i
`UD10+0x19504=0xFFFFFFFF` nie mają odpowiednika w pozostałych 5 save'ach. Może reprezentować
częściową ścieżkę inicjalizacji online wprowadzoną przez patch NetworkParam, interagującą
ze stanem sesji, którego kod vanilla nie osiąga.

---

## 7. Minimalny klasyfikator

Następujące 4 pola jednoznacznie identyfikują wszystkie 6 obserwowanych stanów BF:

```
Pole 1:  UD0+0x209BA4   (= V0.entry_id w kolejce dynamicznej)
Pole 2:  UD10+0x5070    (główne słowo stanu BF)
Pole 3:  UD10+0x194E4   (sentinel ACTIVE-BF)
Pole 4:  UD10+0x5080    (marker sukcesu/wyczyszczenia)
```

### Tabela klasyfikacyjna

| Save | `UD0+0x209BA4` | `UD10+0x5070` | `UD10+0x194E4` | `UD10+0x5080` | Stan |
|------|---------------|--------------|--------------|--------------|------|
| H | `0x989E2000` | `0x0100018F` | `0x00000000` | `0x00000000` | PASSIVE |
| J | `0x3097AE00` | `0x00000001` | `0x00000000` | `0x00000000` | BF-INIT |
| F | `0x3097AE00` | `0x01000081` | `0xFFFFFFFF` | `0x00000000` | ACTIVE-BF |
| I | `0x989E2000` | `0x00000000` | `0x00000000` | `0x3F800000` | SUCCESS |
| G | `0x989E2000` | `0x00000000` | `0x00000000` | `0x00000000` | TIMEOUT |
| E | `0x989E2000` | `0x00002F60` | `0x00000000` | `0x01000054` | PATCHED-IDLE |

**Uwaga**: przy tylko 3 polach (`UD0+0x209BA4`, `UD10+0x5070`, `UD10+0x194E4`), stany I i G
są **nierozróżnialne** — oba zwracają `(0x989E2000, 0x00000000, 0x00000000)`.
Do rozróżnienia wymagane jest `UD10+0x5080`.

### Drzewo decyzyjne

```
UD0+0x209BA4 == 0x3097AE00?
├── TAK → BF aktywny
│   ├── UD10+0x5070 == 0x00000001  →  BF-INIT
│   └── UD10+0x5070 == 0x01000081  →  ACTIVE-BF
└── NIE  (= 0x989E2000)
    ├── UD10+0x5070 == 0x0100018F  →  PASSIVE
    ├── UD10+0x5070 == 0x00002F60  →  PATCHED-IDLE
    └── UD10+0x5070 == 0x00000000
        ├── UD10+0x5080 == 0x3F800000  →  SUCCESS
        └── UD10+0x5080 == 0x00000000  →  TIMEOUT
```

---

## 8. Potwierdzone ustalenia

Wszystkie poniższe pozycje zostały zweryfikowane hexowo i są spójne we wszystkich 6 save'ach.

- Struktura CandidateEntry: krok=`0x14`, 5 pól pod offsetami `+0x00..+0x10`
- `record_type=0x00000C00` dla wszystkich 13 wpisów na głównej liście kandydatów
- Granice sekcji: `UD0+0x209B00..0x209C43`
- Rozmiary podsekcji: nagłówek `0x14`, blok A01 `0x28`, blok C0-C4 `0x64`, V0-V7 `0xA0`, terminator `0x04`
- Nagłówek, blok A01, blok C0-C4: CONST we wszystkich 6 save'ach
- `flag_a` i `flag_b` są właściwościami wpisu — podążają za `entry_id` przez wszystkie przeporządkowania kolejki
- `flag_c=0x00FFFF00` to marker pozycyjny — zawsze na fizycznym V7, niezależnie od tego, który `entry_id` go zajmuje
- Aktywacja BF = fizyczne przepisanie kolejki V — **NIE bufor pierścieniowy ze wskaźnikiem**
- Wzorzec rotacji: cel przesuwa się V7 → V0; pozostałe 7 wpisów jest ODWRÓCONYCH (kolejność V6..V0)
- Brak zewnętrznego wskaźnika head/tail w UD0 (`0x209C40..0x209D00` i `0x209AE0..0x209B00` są zerowe)
- Zero cross-referencji dla jakiegokolwiek `entry_id` w całym UD0 (0x280000 bajtów) i całym UD10 (0x60000 bajtów) poza regionem listy
- `UD10+0x5070` koreluje ze stanem BF — potwierdzone we wszystkich 6 save'ach
- `UD10+0x194E4=0xFFFFFFFF` → unikalny marker ACTIVE-BF (wyłącznie F)
- `UD10+0x5080=f32=1.0` → unikalny marker SUCCESS (wyłącznie I)
- `UD10+0x19504=0xFFFFFFFF` → unikalny marker PATCHED-IDLE (wyłącznie E)
- Stany I (sukces) i G (timeout): oba mają `UD10+0x5070=0x00000000`; rozróżniane wyłącznie przez `UD10+0x5080`
- Wartości NetworkParam ze spatchowanego UD11 NIE są obecne w żadnym stabilnym regionie UD0 ani UD10
- Save'y ze spatchowanym vs vanilla UD11: `UD10+0x5070` i kolejka V UD0 NIE wykazują różnic przypisywalnych NetworkParam

---

## 9. Prawdopodobne interpretacje

Wszystkie poniższe pozycje są spójne ze wszystkimi danymi i nie mają dowodów przeciwnych,
ale nie zostały bezpośrednio potwierdzone wyłącznie z danych pliku save. Traktuj jako hipotezy robocze.

- `flag_b=0x01010000` = „wybrany do matchmakingu" — drugi bajt `0x01` to bit wyboru
- `flag_b=0x01000000` = „zarejestrowany w sesji" — obecny, ale nieaktywnie wybrany
- `flag_a=0x00000A3E` = klasa target / priorytetowy cel inwazji (preferowany kandydat)
- `flag_a=0x00000A00` = zwykły kandydat (niższy priorytet)
- `flag_a=0x00000A01` = klasa specjalna — inne traktowanie niż A00/A3E; możliwe hosty NPC
  lub odrębny podtyp sesji
- Bajt MSB `UD10+0x5070` = stan aktywności; słowo LSB = kod podstanu
- `UD10+0x19508=f32=90.0` = długość okna wyszukiwania inwazji w sekundach
- `UD10+0x194F4=f32=-150.0` = timer odliczający od -150 ku 0 podczas aktywnego wyszukiwania
- `UD10+0x42C54/0x42C58` = współrzędne lub liczniki czasu używane podczas aktywnego wyszukiwania
- Przepisanie kolejki V = „promowane LRU": wybrany cel przechodzi na czoło, pozostałe wpisy odwrócone
  (spójne z semantyką pop stosu MRU — ostatnio napotkani kandydaci opadają na dół)
- Spatchowane wartości NetworkParam w UD11 przeżywają na PS4, ale mogą nie wpływać na gameplay
  jeśli runtime ładuje parametry z kopii w instalacji, a nie z UD11

---

## 10. Nieznane / niemożliwe do ustalenia offline

- **Semantyczne znaczenie poszczególnych wartości `entry_id`** (`0x989E20xx`, `0x3097AE00`,
  `0x12B01Exx`) — zero cross-referencji gdziekolwiek w UD0 lub UD10.
  NIE nazywaj ich „PSN account ID" ani „host ID" — używaj `candidate_id` lub
  `matchmaking_entry_id`. Pochodzenie niepotwierdzono.
- Dlaczego wpisy niebędące targetami są **ODWRÓCONE** zamiast rotowane przy aktywacji BF —
  artefakt algorytmu? Zachowanie stosu MRU/LIFO?
- Co koduje `header.unk_04/08/0C=0x00000100` (licznik? pojemność? zarezerwowane flagi?)
- Co reprezentują wpisy klasy A01 (`0x12B01Exx`) — stałe we wszystkich save'ach, nie klasy target
- Czy `flag_c=0x00FFFF00` koduje wartość 16-bitową `0xFFFF` semantycznie, czy jest po prostu
  wzorcem magicznego sentinela
- Czy I i G różnią się w jakimkolwiek polu UD poza `UD10+0x5080` — dane offline niewystarczające
- `UD10+0x5070=0x00002F60` w E — residuum timera? częściowa ścieżka online-init? brak dopasowania
  do żadnego znanego pola NetworkParam ani pola czasowego
- Dlaczego `UD10+0x19504=0xFFFFFFFF` pojawia się w E (patched-idle), a nie w żadnym innym idle save
- Pełna semantyka `UD10+0x42B00..0x42E00` — heterogeniczna struktura, brak dekodera
- Czy gra czyta NetworkParam z UD11 czy z kopii w instalacji — wymaga zmierzenia rzeczywistego
  interwału inwazji przed i po patchu

---

## 11. Finalny werdykt

### Patch NetworkParam w UD11 nie wpływa mierzalnie na stan runtime UD0/UD10.

Spatchowane wartości przeżywają w pliku PS4 i są odczytywalne po upload/download.
Żadna kopia nie pojawia się w stabilnych regionach UD0 ani UD10. Automat stanów BF
nie wykazuje obserwowalnej różnicy między spatchowanymi a vanilla save'ami w równoważnych
stanach sesji. Czy runtime faktycznie używa tych wartości — niepotwierdzono. Definitywny
dowód wymaga zmierzenia live timing inwazji, co wykracza poza zakres analizy offline.

### Rzeczywisty stan matchmakingu/wyszukiwania jest reprezentowany w UD10 i UD0.

Główne słowo stanu sesji: `UD10+0x5070`.
Aktywna kolejka kandydatów: `UD0+0x209BA0..0x209C3F`.
Są to główne wskaźniki tego, co silnik gry robi z matchmakingiem inwazji —
nie regulation.bin.

### Najcenniejsza struktura to `UD0+0x209B00..0x209C43`.

`MatchmakingCandidateSection` zawiera kompletną listę kandydatów inwazji w dobrze
zdefiniowanym formacie binarnym. CandidateEntry (krok=0x14) z polami `entry_id`, `flag_a`,
`flag_b`, `flag_c` jest w pełni scharakteryzowany. Kolejka jest fizycznie przepisywana
przy aktywacji BF z potwierdzonym wzorcem rotacji.

### Nie nazywaj `entry_id` PSN account ID ani host ID.

Używaj neutralnych nazw: `candidate_id` lub `matchmaking_entry_id`. Nie istnieją
cross-referencje pozwalające zdekodować te wartości wyłącznie z pliku save.

### Co faktycznie przyspiesza inwazje na PS4/PS5.

Empirycznie potwierdzony mechanizm: aktywacja wszystkich Summoning Pools (213 EventFlags
z zakresu `670xxx`). Przy wszystkich aktywnych pulach postać jest w puli matchmakingu
dla każdej dostępnej lokalizacji jednocześnie. Więcej dostępnych hostów → pierwsza próba
kończy się sukcesem → brak 30-sekundowego oczekiwania na retry → inwazje są natychmiastowe.

Dowody:
- regulation.bin został pomyślnie edytowany (bez crashy), spatchowane wartości odczytywalne na PS4 —
  ale czy runtime ich używa — nieweryfikowane
- Szybkie inwazje obserwowane na PS4/PS5 bez żadnych specjalnych ekwipowanych przedmiotów,
  w różnych obszarach, po aktywacji wszystkich Summoning Pools
- Patch v1.12 (~marzec 2025) zmienił wszystkie ID flag pul na zakres `670xxx`; baza danych
  edytora używała starych ID przez ~rok przed naprawą (2026-05-08)

### Praktyczne akcje dostępne przez plik save

| Akcja | Mechanizm | Wpływ |
|-------|-----------|-------|
| **Aktywuj wszystkie Summoning Pools** (213 flag, `670xxx`) | Ustawienie wsadowe EventFlags | **Największy wpływ** — jednoczesne przeszukiwanie wszystkich obszarów |
| Dodaj wszystkie 104 regiony matchmakingu do `unlocked_regions` | `SetUnlockedRegions` | Więcej przeszukiwanych obszarów per próba |
| Dodaj Taunter's Tongue do inwentarza | inject przedmiotu `0x4000006C` | Umożliwia inwazje bez phantomów co-op |
| Dodaj stos Festering Bloody Finger | inject przedmiotu `0x4000006F` | Materiał zużywalny do aktywnych inwazji |
| Ustaw EventFlags questlini Varrého | edycja EventFlags | Odblokowuje Mohgwyn + Festering Bloody Finger bez postępu fabularnego |
| `perform_matchmaking = 1` pod `UD10[0x0013]` | Zapis bajtu UD10 | Zapewnia włączony tryb online |
| Patch NetworkParam w UD11 | edycja regulation.bin | PS4: wartości przeżywają; efekt runtime **niepotwierdzony** |

### Porównanie z DS3

| Aspekt | Dark Souls 3 | Elden Ring |
|--------|-------------|------------|
| Inwazja solo hosta | Tak (domyślnie) | Nie — wymaga Taunter's Tongue |
| `breakInRequestIntervalTimeSec` | ~10s | 30s |
| `breakInRequestTimeOutSec` | ~10s | 20s |
| Dźwignie inwazji w pliku save | Żadne | `unlocked_regions`, Summoning Pools |
| Ochrona regulation.bin | Ryzyko bana EAC online | Ryzyko bana EAC online (patch PS4 przeżywa) |

Wąskim gardłem częstotliwości inwazji jest to, jak często i jak szeroko klient odpytuje
serwer matchmakingu — kontrolowane przez `breakInRequestIntervalTimeSec` i `allAreaSearchRate_*`.
W ER Summoning Pools osiągają efekt „szerokiego zapytania" na poziomie pliku save bez
zmian NetworkParam w runtime.

### Pokrycie Unlocked Regions

Przed tym badaniem: 77 z 103 ID regionów matchmakingu w `backend/db/data/regions.go`.
Dodano 27 brakujących regionów (wnętrza Legacy Dungeon). Łącznie: **104 ID regionów**.

Ryzyko bana za odblokowywanie regionów: **NISKIE**. ID pochodzą z zweryfikowanej listy
`er-save-manager`. Serwer gry weryfikuje kwalifikację matchmakingu po stronie serwera.

---

## Źródła

- `tmp/regulation-bin-dump/csv/NetworkParam.csv` — vanilla NetworkParam wartości pól
- `tmp/regulation-bin-dump/defs/NetworkParam.xml` — typy pól PARAMDEF i offsety
- `tmp/regulation-bin-debug/final-report.md` — surowe wyniki z analizy bf_candidatelist.go
- `tmp/repos/er-save-manager/src/er_save_manager/data/region_ids_map.py` — 103 ID regionów matchmakingu
- `tmp/netman_structure.md` — dokumentacja układu binarnego NetMan
- `backend/core/section_netman.go` — implementacja struktury NetMan
- `backend/core/structures.go` — parsowanie `SaveSlot.UnlockedRegions`
- `backend/core/regulation.go` — patcher NetworkParam z przeliczaniem MD5
- `spec/44-network-param-tuning.md` — pełna referencja pól NetworkParam
