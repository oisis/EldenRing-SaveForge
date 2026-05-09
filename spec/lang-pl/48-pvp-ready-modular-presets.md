# 48 — Modularne Presety PvP-Ready

> **Typ**: Design doc
> **Status**: 🔲 Planowane
> **Zakres**: Dekompozycja monolitycznego presetu world PvP-ready na niezależne,
> oznaczone moduły z poziomami ryzyka per moduł, walidatorami i granularnymi
> kontrolkami UI.

---

## Tło

Pierwotny preset `pvp-ready` aplikował płaski blob `WorldPresetData`: gracje + regiony +
summoning pools + colosseums + flagi mapy. Podejście to ma trzy problemy:

1. **Nieprzejrzyste powiązania** — użytkownik nie może wybrać „tylko regiony" bez
   jednoczesnego aplikowania gracji i summoning pools.
2. **Niepotwierdzone twierdzenia** — aktywacja summoning pools była przedstawiana jako
   akcelerator inwazji; to nie jest potwierdzone (patrz spec/46 §11).
3. **Przestarzałe dane** — stare presety zawierały ID poolów w formacie pre-v1.12
   (≥ 1 000 000). Walidator `validateWorldSummoningPools` to wykrywa, ale główną
   przyczyną jest brak formalnego przypisania każdego zakresu flag do modułu.

Rozwiązanie: dekompozycja na sześć modułów, przypisanie każdemu potwierdzonego efektu
i poziomu ryzyka, następnie indywidualna ekspozycja w UI.

---

## 1. Istniejąca struktura `WorldPresetData`

```go
type WorldPresetData struct {
    Graces         []uint32  // EventFlags 71xxx–76xxx
    Bosses         []uint32  // EventFlags pokonania bossów
    SummoningPools []uint32  // EventFlags 670xxx
    Colosseums     []uint32  // EventFlags 60xxx (tylko activate; multi-flag przez ColosseumFlagSets)
    MapFlags       []uint32  // EventFlags 62xxx / 82xxx
    Cookbooks      []uint32  // EventFlags kucharzy
    BellBearings   []uint32  // EventFlags dzwonków
    Whetblades     []uint32  // EventFlags kamieni szlifierskich
    Gestures       []uint32  // ID slotów gestów
    Regions        []uint32  // UnlockedRegions — osobna struktura binarna, NIE EventFlags
    WorldPickups   []uint32  // EventFlags podbiórki materiałów wzmacniających
}
```

`Regions` to jedyne pole, które zapisuje do `slot.UnlockedRegions` przez
`core.SetUnlockedRegions`. Wszystkie pozostałe pola zapisują bity EventFlag przez
`db.SetEventFlag`.

---

## 2. Definicje modułów

### Moduł A — PvP Core (Statystyki + Przedmioty)

| Właściwość | Wartość |
|---|---|
| Poziom ryzyka | Tier 1 (niski; standardowa edycja buildu) |
| Dotykane pola save | `PlayerGameData`, `Inventory`, `Storage` |
| Funkcje backend | `CharacterPreset.Character`, `Inventory`, `Storage` |

**Zawartość:**
- Poziom postaci, statystyki (Vigor/Mind/Endurance/Strength/Dex/Int/Faith/Arcane)
- Broń, zbroja, talizmany w slotach ekwipunku
- Consumables PvP w inwentarzu/skrzyni:
  - Bloody Finger
  - Festering Bloody Finger
  - Recusant Finger
  - Taunter's Tongue
  - Runy / materiały do ulepszania
- Memory Stones (liczba slotów zaklęć)

**Potwierdzony efekt:** Bezpośredni. Wszystkie pola to standardowe operacje edycji postaci.

---

### Moduł B — Regiony Matchmakingu

| Właściwość | Wartość |
|---|---|
| Poziom ryzyka | Tier 1 (niski; odwracalne) |
| Dotykane pola save | `slot.UnlockedRegions` (osobna struktura binarna) |
| Funkcje backend | `BulkSetUnlockedRegions`, `core.SetUnlockedRegions` |
| Źródło danych | `backend/db/data/regions.go` — 104 wpisy: 62 overworld base game (6100000–6899999) + 7 DLC (6900000–6999999) + 35 legacy dungeons (1000000–1999999) |

**Zawartość:**
- Wszystkie wpisy z `data.Regions` — overworld + legacy dungeons + DLC Land of Shadow
- Kontroluje kwalifikowalność do inwazji (Bloody Finger może dotrzeć tylko do obszarów z tej listy)
- Kontroluje pojawianie się NPC-invaderów (Recusant Henricus itp.)
- Kontroluje etykietę mapy „Wkroczyłeś do \<X\>" po teleportacji

**Potwierdzony efekt (spec/11, obserwowany w runtime):** Regiony muszą być obecne, żeby
matchmaking PvP brał slot pod uwagę do inwazji. Brakujące regiony = postać praktycznie
niewidoczna dla invaderów w danym obszarze.

**Uwaga:** `data.IsDLCRegion(id)` → zakres `6900000–6999999` = Shadow of the Erdtree.
Regiony DLC powinny być oznaczone oddzielnie w UI. Zastosowanie ich do postaci bez DLC
jest nieszkodliwe, ale logicznie niepoprawne.

**Walidator:** `ValidateWorldRegions` (patrz §4).

---

### Moduł C — Dostęp PvP / Flagi Postępu

| Właściwość | Wartość |
|---|---|
| Poziom ryzyka | Tier 1 (manipulacja flagami questów; ten sam tier co pokonywanie bossów) |
| Dotykane pola save | bitfield EventFlags |
| Funkcje backend | `db.SetEventFlag`, `SetQuestStep`, `SetColosseumUnlocked` |
| Źródło danych | `quests.go` (White Mask Varre), `summoning_pools.go` (Colosseums) |

**Zawartość:**

#### Colosseums (potwierdzone: odblokowanie mapy + matchmakingu)
- Limgrave Colosseum: `ColosseumFlagSets[60360]` → flagi `60360, 62730, 69460, 710860`
- Caelid Colosseum: `ColosseumFlagSets[60350]` → flagi `60350, 62720, 69450, 710850`
- Royal Colosseum: `ColosseumFlagSets[60370]` → flagi `60370, 62740, 69470, 710870`
- Globalne flagi colosseums: `6080, 60100, 69480`

Stan otwarcia bramy (fizyczne drzwi) jest przechowywany poza EventFlags (blob binarny WorldGeom) —
nie da się tego edytować z poziomu edytora save'ów. Gracz musi otworzyć bramę raz w grze.

#### Quest Varre (Bloody Finger / dostęp do Mohgwyn Palace)
Kluczowe kroki do odblokowania Bloody Finger przez ścieżkę questa:
- Otrzymanie „Lord of Blood's Favor" — flaga `400031`
- Nasączenie przysługi krwią dziewicy — flaga `400033`
- Ofiarowanie palca / Bloody Finger przyznany — flagi `1035449227, 9432, 9420, 1800`
- Otrzymanie Pureblood Knight's Medal (skrót do Mohgwyn) — flaga `400032`

**Uwaga:** Flagi questa Varre to złożony łańcuch wielu kroków (patrz `quests.go`
„White Mask Varre" — 19 kroków). Ustawienie tylko flag ukończenia bez wcześniejszych
flag dialogowych może pozostawić NPC w niespójnym stanie w grze. UI powinno oferować
„pomiń do Bloody Finger przyznany" jako krok presetu, nie surowe przełączanie flag.
Implementacja powinna używać `SetQuestStep` zamiast surowego `SetEventFlag`.

**Potwierdzony efekt (system questów):** Ustawianie flag questa Varre przez system questów
to ten sam mechanizm co dla wszystkich ukończeń questów. Flagi colosseums są hex-zweryfikowane
(patrz `tmp/coloseum-debug/`).

**Walidator:** `ValidateWorldColosseums` (patrz §4).

---

### Moduł D — QoL Świata (Mapa + Gracje)

| Właściwość | Wartość |
|---|---|
| Poziom ryzyka | Tier 0 (tylko wizualny/nawigacyjny; brak efektu kompetytywnego) |
| Dotykane pola save | bitfield EventFlags |
| Funkcje backend | `RevealAllMap`, `SetGraceVisited` (przez `ApplyWorldState`) |
| Źródło danych | `backend/db/data/graces.go`, `maps.go` |

**Zawartość:**
- Odsłonięcie mapy: `62xxx` (widoczność kafli mapy) + `82xxx` (flagi systemowe mapy)
- Sites of Grace: `71xxx–76xxx` — kontroluje znacznik na mapie + wpis w liście fast-travel
  - Towarzysząca `DoorFlag` dla katakumb/hero graves (`73xxx`)
  - Animacja aktywacji in-world: zarządzana przez runtime/EMEVD, może nadal odtwarzać się przy pierwszej wizycie (spec/47)

**Potwierdzony efekt:** Znaczniki pojawiają się na mapie; lista fast-travel jest wypełniona.
Brak przewagi kompetytywnej — nie wpływa na matchmaking inwazji, czas oczekiwania na BF ani
stan sesji.

**Uwaga:** Odsłonięcie mapy i odblokowanie gracji to niezależne pod-opcje w ramach QoL.
UI powinno pozwalać na ich osobne wybranie (odsłonięcie mapy bez odblokowania gracji i odwrotnie).

**Walidatory:** `ValidateWorldGraces`, `ValidateWorldMapFlags` (patrz §4).

---

### Moduł E — Co-op / Przywoływanie (Summoning Pools)

| Właściwość | Wartość |
|---|---|
| Poziom ryzyka | Tier 1 (ustawienie EventFlag; odwracalne) |
| Dotykane pola save | bitfield EventFlags (blok 670xxx) |
| Funkcje backend | `SetSummoningPoolActivated` (przez `ApplyWorldState`) |
| Źródło danych | `backend/db/data/summoning_pools.go` — 670xxx, 199 wpisów |
| Istniejący walidator | `validateWorldSummoningPools` w `vm/preset.go` |

**Zawartość:**
- Flagi aktywacji Martyr Effigy (`670xxx`) — umożliwia znaki przywoływania co-op w lochach
- DLC summoning pools: `670800–670999` — Shadow of the Erdtree
- Pre-v1.12 IDs (≥ 1 000 000): wykrywane i ostrzegane przez istniejący walidator

**Potwierdzony efekt:** Umożliwia znaki przywoływania co-op przy Martyr Effigies.

**Wpływ Bloody Finger na inwazje: NIEPOTWIERDZONY.** Aktywacja summoning pools była
historycznie przyjmowana jako skrócenie czasu oczekiwania na matchmaking inwazji. Nie
zostało to potwierdzone przez analizę binarną spec/46. Nie przedstawiaj tego modułu jako
akceleratora inwazji. Prezentuj go wyłącznie jako aktywację co-op/przywoływania.

**Walidator:** `validateWorldSummoningPools` — już zaimplementowany.

---

### Moduł F — Zaawansowane / Badania (Diagnostyka)

| Właściwość | Wartość |
|---|---|
| Poziom ryzyka | Tier 2 (zapis UD11: plik PS4 przeżywa, serwer nadpisuje przy połączeniu online) |
| Dotykane pola save | UD11 NetworkParam (odczyt/zapis), UD10 stan BF (tylko odczyt), UD0 CandidateSection (tylko odczyt) |
| Funkcje backend | `core.NetworkParamValues` (istniejące), nowe czytniki diagnostyczne |
| Źródło danych | spec/44 (NetworkParam), spec/46 (maszynastanów UD10/UD0) |

**Zawartość:**

#### Inspektor / Patcher NetworkParam UD11
- Odczyt bieżących wartości `breakInRequestIntervalTimeSec`, `breakInRequestTimeOutSec` itp.
- Opcjonalny zapis dostrojonych wartości (patrz spec/44)
- **Ostrzeżenie dla użytkownika**: „Konsola PS4/PS5 nadpisuje regulation.bin z serwera przy
  połączeniu online. Zmiany UD11 są lokalne i nie zostało potwierdzone, że wpływają na rzeczywisty
  czas inwazji."

#### Klasyfikator Stanu BF UD10
- Tylko odczyt: `UD10+0x5070`, `UD10+0x194E4`, `UD10+0x5080`
- Mapowanie na czytelny stan: `PASSIVE / BF-INIT / ACTIVE-BF / SUCCESS / TIMEOUT / PATCHED-IDLE`
- Drzewo decyzyjne ze spec/46 §7

#### Przeglądarka Sekcji Kandydatów UD0
- Tylko odczyt: `UD0+0x209B00..0x209C43`
- Parsowanie struktur `CandidateEntry` (krok `0x14`, 5 pól)
- Wyświetlanie statusu `SPEC-VALID / DEVIATES / NOT-INITIALIZED`
- Wyświetlanie stanu V-queue (IDLE vs ACTIVE, który `entry_id` jest na V0)

**Ten moduł jest wyłącznie diagnostyczny.** Nie eksponuj jako przycisku „przyspiesz PvP".
Patcher NetworkParam to funkcja zaawansowana dla użytkowników-badaczy, którzy rozumieją,
że egzekwowanie po stronie serwera czyni zmiany UD11 nieskutecznymi online.

---

## 3. Przypisanie Zakresów Flag do Modułów

| Dane / zakres flag | Aktualne znaczenie w kodzie | Moduł | Pewność | Uwagi |
|---|---|---|---|---|
| `1000000–1999999` | `data.Regions` — wnętrza legacy dungeons (35 wpisów) | B — Regiony Matchmakingu | ✅ potwierdzone | Osobna struktura binarna przez `core.SetUnlockedRegions` |
| `6100000–6899999` | `data.Regions` — overworld base game (62 wpisy) | B — Regiony Matchmakingu | ✅ potwierdzone | Osobna struktura binarna przez `core.SetUnlockedRegions` |
| `6900000–6999999` | DLC Regions — Land of Shadow (7 wpisów) | B — Regiony (DLC) | ✅ potwierdzone | Guard `IsDLCRegion()` |
| `60350, 60360, 60370` | Aktywacja Colosseum | C — Dostęp PvP | ✅ hex-zweryfikowane | Używać `ColosseumFlagSets`, nie tylko activate |
| `62xxx` | Widoczność kafli mapy | D — QoL Świata | ✅ potwierdzone | `RevealBaseMap` / `MapFlags` |
| `82xxx` | Flagi systemowe mapy | D — QoL Świata | ✅ potwierdzone | `RevealBaseMap` / `MapFlags` |
| `71xxx–76xxx` | EventFlags Sites of Grace | D — QoL Świata | ✅ potwierdzone | spec/47 — znacznik mapy + fast-travel |
| `670xxx` (base, 100–799) | Summoning Pools — Martyr Effigies | E — Co-op / Przywoływanie | ✅ potwierdzone | Wpływ na inwazje BF: niepotwierdzony |
| `670800–670999` | DLC Summoning Pools | E — Co-op (DLC) | ✅ potwierdzone | `IsDLCSummoningPool()` |
| `≥ 1 000 000` (stare ID poolów) | Format pre-v1.12 — nieprawidłowy | Legacy / Przestarzałe | ✅ potwierdzone | `validateWorldSummoningPools` ostrzega |
| Flagi questa Varre | `1035449xxx`, `400031–400037` | C — Dostęp PvP | ⚠️ cross-ref | Wieloetapowy; używać `SetQuestStep` |
| Nieznane flagi `1033438600–1050558540` | Niesklasyfikowany stan obiektu/obszaru | Nieznane | ❓ | Nie przypisywać do modułu |
| `6080, 60100, 69480` | Globalne flagi colosseums | C — Dostęp PvP | ✅ hex-zweryfikowane | `ColosseumGlobalFlags` |

---

## 4. Proponowane Walidatory

Wszystkie walidatory znajdują się w `backend/vm/validation.go` (istniejący) lub `backend/vm/preset.go`.
Żaden nie blokuje produkcji — emitują `warnings []string`, nie błędy fatalne.

### `ValidateWorldRegions(ids []uint32) []string` ✅ Zaimplementowany

**Sprawdza:**
- Każde ID jest obecne w mapie `data.Regions` → `warning: world.regions: ID %d not found in region database`
- Wykrywa duplikaty → `warning: world.regions: ID %d appears more than once — duplicate will be ignored`
- Legacy dungeon IDs (`1000000–1999999`) są prawidłowe — brak ostrzeżenia
- DLC region IDs (`6900000–6999999`) są prawidłowe — brak ostrzeżenia (sprawdzenie kontekstu non-DLC zarezerwowane dla przyszłej warstwy UI)

**Zwraca:** ostrzeżenie (nie błąd). Blokowanie/odblokowanie regionu jest odwracalne.

**Lokalizacja:** `backend/vm/preset.go` jako `validateWorldRegions` (prywatna, wywoływana z `ValidatePreset`).
Helper DB: `db.IsKnownRegionID(id uint32) bool` w `backend/db/db.go`.

**Testy (7/7 passing):**
- Znane ID overworld → brak ostrzeżenia
- Znane ID legacy dungeon → brak ostrzeżenia
- Znane ID DLC → brak ostrzeżenia
- Nieznane ID (`9999999`) → ostrzeżenie
- Duplikat znanych ID → ostrzeżenie
- `nil` World → brak ostrzeżenia
- Pusta lista regionów → brak ostrzeżenia

---

### `ValidateWorldGraces(ids []uint32) []string` ✅ Zaimplementowany

**Sprawdza:**
- Każde ID jest obecne w mapie `data.Graces` → `warning: world.graces: ID %d not found in grace database`
- Wykrywa duplikaty → `warning: world.graces: ID %d appears more than once — duplicate will be ignored`
- DLC grace IDs (`72xxx`, `74xxx`) są prawidłowe — brak ostrzeżenia
- `DoorFlag` NIE jest wymagana w presecie — `SetGraceVisited()` ustawia ją automatycznie z `data.Graces`
- Nie waliduje `LastRestedGrace`, `BonfireId` ani stanu aktywacji EMEVD (patrz spec/47)

**Zwraca:** ostrzeżenie (nie błąd).

**Lokalizacja:** `backend/vm/preset.go` jako `validateWorldGraces` (prywatna, wywoływana z `ValidatePreset`).
Helper DB: `db.IsKnownGraceID(id uint32) bool` w `backend/db/db.go`.

**Testy (7/7 passing):**
- Znana gracja base (`71000`, `76100`) → brak ostrzeżenia
- Znana gracja DLC (`72000`, `72001`) → brak ostrzeżenia
- Gracja katakumbowa z `DoorFlag` (`73000`) → brak ostrzeżenia (DoorFlag nie wymagana w presecie)
- Nieznane ID (`99999`) → ostrzeżenie
- Duplikat znanych ID → ostrzeżenie
- `nil` World → brak ostrzeżenia
- Pusta lista gracji → brak ostrzeżenia

---

### `ValidateWorldMapFlags(ids []uint32) []string` ✅ Zaimplementowane

**Sprawdza:**
- Każde ID jest obecne w jednej z czterech map danych (`data.MapVisible`, `data.MapSystem`, `data.MapAcquired`, `data.MapUnsafe`) → `warning: world.mapFlags: ID %d not found in map flag database`
- Wykrywa duplikaty ID → `warning: world.mapFlags: ID %d appears more than once — duplicate will be ignored`
- Grace ID (`71xxx–76xxx`) przypadkowo umieszczone w `MapFlags` → ostrzeżenie (nie znalezione w żadnej mapie danych mapy)
- Summoning pool ID (`670xxx`) przypadkowo umieszczone w `MapFlags` → ostrzeżenie

**Zwraca:** ostrzeżenie (nie błąd).

**Gdzie mieszka:** `backend/vm/preset.go` jako `validateWorldMapFlags` (prywatna, wywoływana z `ValidatePreset`).
Helper DB: `db.IsKnownMapFlagID(id uint32) bool` w `backend/db/db.go` — sprawdza wszystkie cztery mapy danych.

**Pokrycie danych:**
- `data.MapVisible` — 85 wpisów (`62010–62221`), flagi widoczności kafelków
- `data.MapSystem` — 79 wpisów (`62000–82002`), w tym `62000` (Allow Map Display), `82001` (Show Underground), `82002` (Show Shadow Realm Map)
- `data.MapAcquired` — 24 wpisy
- `data.MapUnsafe` — 56 wpisów

**Testy (8/8 zaliczone):**
- Znane IDs `MapVisible` (`62000`, `62010`) → brak ostrzeżenia
- Znane IDs `MapSystem` (`82001`, `82002`) → brak ostrzeżenia
- Nieznane ID (`99999`) → ostrzeżenie
- Duplikat znanych ID (`62010` powtórzone) → ostrzeżenie
- `nil` World → brak ostrzeżenia
- Pusta lista flag mapy → brak ostrzeżenia
- Grace ID (`76100`) umieszczone w `MapFlags` → ostrzeżenie (błędna lokalizacja)
- Summoning pool ID (`670100`) umieszczone w `MapFlags` → ostrzeżenie (błędna lokalizacja)

---

### `ValidateWorldColosseums(ids []uint32) []string`

**Sprawdza:**
- Każde ID jest w mapie `data.Colosseums`
- Dla każdej obecnej flagi Activate sprawdza, czy flagi towarzyszące (`MapPOI`, `NPC`, `Gate`)
  są również obecne w `MapFlags` presetu → `warning: colosseum X brakuje flag towarzyszących`
- Flagi obecności `ColosseumGlobalFlags` (`6080, 60100, 69480`)

**Zwraca:** ostrzeżenie.

**Dlaczego:** Colosseum z ustawioną tylko flagą Activate może pojawić się w matchmakingu,
ale nie na mapie, lub mieć bramę wizualnie zamkniętą. Wymagany jest pełny `ColosseumFlagSet`.

**Testy:**
- Pełny `ColosseumFlagSet` dla Limgrave → brak ostrzeżenia
- Tylko activate → ostrzeżenie: „brakuje MapPOI, NPC, Gate"

---

### `ValidateKnownEventFlags(ids []uint32, context string) []string`

Generyczny walidator dla dowolnej listy event flag:

**Sprawdza:**
- Flagi ≥ 1 000 000 000 (poza wszystkimi znanymi blokami EventFlag) → `warning: ID poza wszystkimi znanymi zakresami`
- Flagi pasujące do wzorca pre-v1.12 summoning pool (≥ 1 000 000 I context == "summoningPools") → przekieruj do `validateWorldSummoningPools`
- Flagi w nieznanych zakresach bloków → `info: niesklasyfikowany zakres`

**Zwraca:** ostrzeżenie/info.

---

### `ValidatePresetModules(preset *CharacterPreset) []string`

Orkiestrator. Wywołuje:
1. `validateWorldSummoningPools` (istniejący)
2. `ValidateWorldRegions`
3. `ValidateWorldGraces`
4. `ValidateWorldMapFlags`
5. `ValidateWorldColosseums`

Wywoływany z `ValidatePreset` gdy `preset.World != nil`.

---

## 5. Propozycja UI / UX

Aktualny UI: pojedynczy checkbox „Apply World Preset" bez granularności.

**Propozycja:** Zastąpienie listą checkboxów modułów w zakładce WorldTab lub oknie dialogowym
aplikacji presetu.

```
Przygotowanie PvP
──────────────────────────────────────────────────────────────
[✓] Moduł A — Statystyki i Ekwipunek       ryzyko: Tier 1
[✓] Moduł B — Odblokuj Regiony Inwazji     ryzyko: Tier 1  ← główny efekt
──────────────────────────────────────────────────────────────
[ ] Moduł C — Skrót Questa Varre            ryzyko: Tier 1
[ ] Moduł C — Odblokuj Colosseums           ryzyko: Tier 1
──────────────────────────────────────────────────────────────
[ ] Moduł D — Odsłoń Mapę                  ryzyko: Tier 0  (tylko wizualny)
[ ] Moduł D — Odblokuj Sites of Grace       ryzyko: Tier 0  (tylko fast-travel)
──────────────────────────────────────────────────────────────
[ ] Moduł E — Aktywuj Summoning Pools       ryzyko: Tier 1
                (co-op; wpływ BF na inwazje niepotwierdzony)
──────────────────────────────────────────────────────────────
[ ] Moduł F — Inspektor NetworkParam        ryzyko: Tier 2  (badania)
[ ] Moduł F — Klasyfikator Stanu BF         tylko odczyt diagnostyczny
[ ] Moduł F — Przeglądarka Sekcji Kandydatów  tylko odczyt diagnostyczny
```

**Reguły projektowe:**
- Moduły A + B domyślnie zaznaczone przy otwieraniu przepływu „PvP Preset"
- Każdy moduł pokazuje plakietkę poziomu ryzyka i opis jednolinijkowy
- Pozycje Modułu F złożone za akordeonem „Zaawansowane / Badania"
- Moduł Summoning Pool (E) ma notatkę inline: „Aktywuje znaki przywoływania co-op.
  Wpływ na częstotliwość inwazji Bloody Finger jest niepotwierdzony."
- Pod-moduł Colosseum (C) pokazuje: „Odblokowuje matchmaking colosseums i znaczniki mapy.
  Fizyczna brama musi być otwarta raz w grze."
- Pod-moduł Sites of Grace (D) pokazuje: „Odblokowuje znaczniki mapy i fast-travel.
  Niektóre gracje mogą nadal odtwarzać animację aktywacji przy pierwszej wizycie." (ze spec/47)

**Czego NIE usuwamy:**
- Istniejących indywidualnych przełączników flag w WorldTab (akordeon Graces, Summoning Pools itp.)
- Istniejącej funkcji `ApplyWorldState` — modularny UI jest nad nią zbudowany
- Istniejącego exportu/importu presetów — format `WorldPresetData` pozostaje niezmieniony

---

## 6. Notatka o Kierunku Produktu dla spec/46

> Do dodania do `spec/46-faster-invasions-research.md` jako nowa końcowa sekcja.

**Kierunek produktu / implikacje dla SaveForge:**

Badanie save-file (spec/46) przyniosło jednoznaczny werdykt: nie istnieje żadne zapisywalne
pole save-file, które bezpośrednio skraca czas oczekiwania na inwazję. Praktyczna ścieżka
przygotowania PvP przez edycję save'a to:

- **Odblokowanie regionów** (Moduł B) — potwierdzone kontrolowanie kwalifikowalności do inwazji
  i widoczności obszaru dla invaderów. To jest główna funkcja PvP-relevantna na poziomie save.
- **UD11 NetworkParam** — wartości przeżywają w pliku PS4; efekt runtime na timing inwazji jest
  niepotwierdzony. Eksponuj jako diagnostykę klasy badawczej (Moduł F), nie jako główną funkcję PvP.
- **Struktury sesji UD10 / UD0** — stan wyłącznie runtime. Eksponuj jako diagnostykę tylko do
  odczytu (Moduł F). Nie są celem patchowania.
- **Summoning Pools** — funkcja co-op / przywoływania. Wpływ Bloody Finger na inwazje jest
  niepotwierdzony. Nie przedstawiaj jako akceleratora inwazji (Moduł E).
- **Preset pvp-ready** — musi stać się modularny (ta specyfikacja). Pojedynczy nieprzejrzysty
  preset miesza potwierdzone efekty (regiony) z niepotwierdzonymi (summoning pools, UD11) oraz
  zmiany tylko wizualne (mapa, gracje).

---

## 7. Fazy Implementacji

| Faza | Zakres | Zablokowana na |
|---|---|---|
| Faza 1 | Implementacja pozostałych walidatorów (`ValidateWorldRegions`, `ValidateWorldColosseums`, `ValidateWorldMapFlags`, `ValidateWorldGraces`) + podłączenie do `ValidatePreset` | Nic |
| Faza 2 | Dodanie generycznego walidatora `ValidateKnownEventFlags` | Faza 1 |
| Faza 3 | Lista checkboxów modułów w UI WorldTab / okno dialogowe apply presetu | Faza 1 |
| Faza 4 | Moduł F: klasyfikator stanu BF tylko do odczytu (czytniki UD10 + UD0) | spec/46 §7 |
| Faza 5 | Moduł F: UI inspektora NetworkParam UD11 | spec/44 |
| Faza 6 | Krok presetu skrótu questa Varre (Moduł C) | wzorzec spec/38 dla flag questów |

**Faza 1 to minimalne dostarczalne MVP.** Wszystkie kolejne fazy na niej bazują.

---

## Źródła

| Plik | Znaczenie |
|---|---|
| `backend/vm/preset.go` | `WorldPresetData`, `ApplyWorldState`, `ValidatePreset` |
| `backend/db/data/regions.go` | 104 ID regionów z nazwą + grupą obszaru (62 overworld + 7 DLC + 35 legacy dungeons) |
| `backend/db/data/summoning_pools.go` | 199 ID summoning pools + `Colosseums` + `ColosseumFlagSets` |
| `backend/db/data/graces.go` | 419 wpisów gracji z ID EventFlag + DoorFlags |
| `backend/db/data/quests.go` | `"White Mask Varre"` — 19 kroków questa z flagami |
| `app_world.go` | Wszystkie funkcje backend zakładki World |
| `spec/46-faster-invasions-research.md` | Werdykt końcowy: patch UD11 bez potwierdzonego efektu runtime |
| `spec/47-site-of-grace-activation.md` | Odblokowanie gracji potwierdzone dla mapy/fast-travel; animacja in-world otwarta |
| `spec/44-network-param-tuning.md` | Referencja pól NetworkParam |
| `spec/32-ban-risk-system.md` | Definicje Tier 0/1/2 |
| `tmp/coloseum-debug/` | Hex-zweryfikowane zestawy flag colosseums |
