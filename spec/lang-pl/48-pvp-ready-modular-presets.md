# 48 — Modularne Presety PvP-Ready

> **Typ**: Design doc
> **Status**: ✅ Zaimplementowane (Faza 1 kompletna)
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

**Ograniczenie formatu presetu:** `ExportWorldState` przechwytuje tylko flagi activate
(60350/60360/60370) do `World.Colosseums`. NPC (69xxx), Gate (710xxx) i globalne flagi
(6080, 60100, 69480) nie są eksportowane, bo nie figurują w żadnej nazwanej tabeli danych.
`ApplyWorldState` ustawia tylko to, co jest w presecie — nie rozszerza automatycznie jak
`SetColosseumUnlocked`. Aby w pełni odblokować colosseums przez preset, użytkownik musi
ręcznie dodać globalne flagi do `World.Colosseums` i upewnić się, że flagi MapPOI są w
`World.MapFlags`. `ValidateWorldColosseums` ostrzega o brakujących flagach.

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
- **Ostrzeżenie dla użytkownika**: „Patch NetworkParam jest potwierdzony skuteczny po
  przeładowaniu postaci (wyjście do menu → reload). Serwer nadpisuje regulation.bin przy
  następnym połączeniu online — efekt jest tymczasowy. Użycie online niesie ryzyko bana EAC."
- Wyświetl potwierdzoną procedurę aktywacji (spec/46 §11) inline

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
Patcher NetworkParam to funkcja zaawansowana. Efekt potwierdzony po przeładowaniu postaci
(offline); serwer nadpisuje UD11 przy połączeniu online — zmiany są tymczasowe.

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

### `ValidateWorldColosseums(world *WorldPresetData) []string` ✅ Zaimplementowane

**Sprawdza:**
- Każde ID w `World.Colosseums` jest rozpoznane w `data.ColosseumFlagSets` → `warning: world.colosseums: ID %d not found in colosseum database`
- Wykrywa duplikaty → `warning: world.colosseums: ID %d appears more than once — duplicate will be ignored`
- Dla każdej flagi activate sprawdza obecność MapPOI w `World.MapFlags` → `warning: world.colosseums: %s (ID %d) is missing companion map flag %d — colosseum icon will not appear on map`
- Flagi globalne (6080, 60100, 69480) muszą być w `World.Colosseums` → `warning: world.colosseums: global colosseum flag %d is missing — add to World.Colosseums for full unlock`
- Globalne flagi w `World.Colosseums` są akceptowane bez ostrzeżenia

**Zwraca:** ostrzeżenie (nie błąd). Import nie jest blokowany.

**Gdzie mieszka:** `backend/vm/preset.go` jako `validateWorldColosseums(world *WorldPresetData, warnings *[]string)`.
Helpery DB: `db.IsKnownColosseumID(id uint32) bool`, `db.GetColosseumFlagSet(id uint32) (data.ColosseumFlagSet, bool)` w `backend/db/db.go`.

**Dlaczego:** Colosseum z ustawioną tylko flagą Activate może pojawić się w matchmakingu,
ale bez ikony na mapie. `ApplyWorldState` nie rozszerza zestawów flag automatycznie jak
`SetColosseumUnlocked` — flagi towarzyszące muszą być jawne w presecie.

**Znane ograniczenie:** Flagi NPC (69xxx) i Gate (710xxx) nie są przechwytywane przez
`ExportWorldState` i nie można ich zweryfikować z danych presetu. Stan otwarcia fizycznej
bramy jest poza EventFlags (blob WorldGeom) — walidator tego nie sprawdza. Pozostaje
to tematem przyszłych badań.

**Testy (8/8 zaliczone):**
- Pełny zestaw: Limgrave activate (60360) + globalne (6080/60100/69480) + MapPOI (62730) → brak ostrzeżenia
- Nieznane ID (99999) → ostrzeżenie
- Duplikat (60360 ×2) → ostrzeżenie
- Tylko activate (brak MapPOI, brak globalnych) → ostrzeżenie MapPOI + 3 ostrzeżenia globalnych
- Brakujące flagi globalne (MapPOI obecne) → 3 ostrzeżenia globalnych
- `nil` World → brak ostrzeżenia
- Puste `World.Colosseums` → brak ostrzeżenia
- Wszystkie trzy kolosea (60350/60360/60370 + globalne + wszystkie MapPOI) → brak ostrzeżenia

---

### `validateKnownEventFlags(context string, ids []uint32, warnings *[]string)` ✅ Zaimplementowane

Generyczny safety-net dla list event flag bez dedykowanego walidatora per-baza-danych.

**Sprawdza:**
- Duplikaty ID → `warning: <context>: ID %d appears more than once — duplicate will be ignored`
- ID ≥ 1 000 000 000 (poza przestrzenią adresową EventFlags) → `warning: <context>: ID %d is outside all known EventFlag ranges (>= 1,000,000,000) — likely invalid`

**Gdzie mieszka:** `backend/vm/preset.go` jako `validateKnownEventFlags` (prywatna). Wywoływana z `validatePresetModules` dla pól bez dedykowanego walidatora.

**Stosowana dla:** `World.Bosses`, `World.Cookbooks`, `World.BellBearings`, `World.Whetblades`, `World.WorldPickups`.

**Celowo wykluczona:**
- `World.SummoningPools` — `validateWorldSummoningPools` używa niższego progu (≥ 1 000 000); generyczny walidator spowodowałby podwójne ostrzeżenie dla ID takich jak `1035530040`
- `World.Graces`, `World.MapFlags`, `World.Colosseums` — mają dedykowane walidatory DB
- `World.Regions` — nie są EventFlags; zapis do binarnej struktury `UnlockedRegions`
- `World.Gestures` — ID slotów gestów, nie ID EventFlag

**Uwaga projektowa:** Generyczny walidator NIE sprawdza „czy to ID jest znane w bazie danych" dla objętych pól. Wymagałoby to dedykowanego lookup'u per pole i powodowałoby false positive dla prawidłowych ID gry nieobecnych w bazie. Sygnalizowane są tylko ewidentnie błędne dane (duplikaty, ekstremalne ID).

**Testy (8/8 zaliczone, przez integrację z `ValidatePreset`):**
- Duplikat ID boss → ostrzeżenie
- ID boss ≥ 1B → ostrzeżenie
- Nieznane, ale nie-ekstremalne ID boss → brak ostrzeżenia
- Duplikat ID cookbook → ostrzeżenie
- ID worldPickup ≥ 1B → ostrzeżenie
- Pre-v1.12 ID summoning pool (`1035530040`, ≥ 1B) → tylko ostrzeżenie „pre-v1.12", NIE generyczne „>= 1B" (brak podwójnego warningu)
- Puste `WorldPresetData` → brak ostrzeżeń
- Wiele nieprawidłowych pól → wszystkie ostrzeżenia agregowane

---

### `validatePresetModules(world *WorldPresetData, warnings *[]string)` ✅ Zaimplementowane

Orkiestrator wszystkich walidatorów presetu world. Zastępuje ręczne wywołania walidatorów w `ValidatePreset`, który teraz wywołuje tylko `validatePresetModules` gdy `preset.World != nil`.

**Kolejność wykonania (deterministyczna):**
1. `validateWorldSummoningPools` — dedykowany, próg ≥ 1 000 000
2. `validateWorldRegions` — dedykowany lookup DB
3. `validateWorldGraces` — dedykowany lookup DB
4. `validateWorldMapFlags` — dedykowany lookup DB (4 mapy danych mapy)
5. `validateWorldColosseums` — dedykowany lookup DB + MapPOI + sprawdzenie globalnych flag
6. `validateKnownEventFlags("world.bosses", ...)` — generyczny safety-net
7. `validateKnownEventFlags("world.cookbooks", ...)` — generyczny safety-net
8. `validateKnownEventFlags("world.bellBearings", ...)` — generyczny safety-net
9. `validateKnownEventFlags("world.whetblades", ...)` — generyczny safety-net
10. `validateKnownEventFlags("world.worldPickups", ...)` — generyczny safety-net

**Wykluczone z całej walidacji:** `World.Regions` (dedykowany), `World.Gestures` (nie są EventFlags).

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
- **UD11 NetworkParam** — patch potwierdzony skuteczny po przeładowaniu postaci/sesji.
  Serwer nadpisuje UD11 przy połączeniu online. Eksponuj jako Moduł F (zaawansowany/badawczy)
  z procedurą aktywacji i ostrzeżeniem o ryzyku bana EAC. Nie jako główną funkcję PvP.
- **Struktury sesji UD10 / UD0** — stan wyłącznie runtime. Eksponuj jako diagnostykę tylko do
  odczytu (Moduł F). Nie są celem patchowania.
- **Summoning Pools** — funkcja co-op / przywoływania. Wpływ Bloody Finger na inwazje jest
  niepotwierdzony. Nie przedstawiaj jako akceleratora inwazji (Moduł E).
- **Preset pvp-ready** — musi stać się modularny (ta specyfikacja). Pojedynczy nieprzejrzysty
  preset miesza potwierdzone efekty (regiony) z niepotwierdzonymi (summoning pools), badawczo-tymczasowymi
  (NetworkParam UD11 — potwierdzony po reload, ale serwer resetuje online) i wizualnymi (mapa, gracje).

---

## 7. Fazy Implementacji

| Faza | Zakres | Status |
|---|---|---|
| Faza 1 | `ValidateWorldRegions`, `ValidateWorldColosseums`, `ValidateWorldMapFlags`, `ValidateWorldGraces` + `validateKnownEventFlags` (generyczny) + `validatePresetModules` (orkiestrator) + podłączenie do `ValidatePreset` | ✅ Kompletna |
| Faza 2 | Zakładka `PvP Preparation` — high-level orkiestrator UI z checkboxami per moduł | ✅ Kompletna (MVP) |
| Faza 3 | Moduł F: klasyfikator stanu BF tylko do odczytu (czytniki UD10 + UD0) | 🔲 Planowane (blokada: spec/46 §7) |
| Faza 4 | Moduł F: UD11 NetworkParam — Network Speed Panel (selektor presetów) | ✅ Kompletna |
| Faza 5 | Krok presetu skrótu questa Varre (Moduł C) | 🔲 Planowane (blokada: wzorzec spec/38) |

**Faza 1 jest kompletna.** `validateKnownEventFlags` i `validatePresetModules` zostały skonsolidowane do Fazy 1 (pierwotnie zaplanowane jako Faza 2), ponieważ zależą wyłącznie od walidatorów Fazy 1.

**MVP Fazy 2 jest kompletne.** Zakładka `PvP Preparation` zawiera 4 aktywne moduły (Matchmaking Regions, Colosseums, Map Reveal, Summoning Pools) i 1 placeholder (Sites of Grace). Patrz §8 poniżej.

**Faza 4 jest kompletna.** Network Speed Panel (`NetworkSpeedPanel.tsx`) dostępny jako accordion pod zakładką PvP Preparation — patrz §8.1 poniżej.

---

## 8. Implementacja UI MVP

**Nowa zakładka**: `PvP Preparation` (dodana do listy zakładek w `frontend/src/App.tsx`: `['character', 'inventory', 'world', 'pvp', 'tools', 'settings']`).

**Architektura**:
- `WorldTab` pozostaje granularnym edytorem (przełączniki per element, akordeony per sekcja).
- `PvP Preparation` jest high-level orkiestratorem: checkboxy per moduł + jeden przycisk `Apply`.
- Frontend NIE duplikuje list regionów/koloseów/pul — wszystkie dane są w backendowej DB.

**Backend**:
- Nowy plik `app_pvp.go` z typem `PvPPreparationOptions` i metodą `ApplyPvPPreparation(slotIndex int, opts PvPPreparationOptions) ([]string, error)`.
- Pojedynczy `pushUndo` dla całej operacji (bez stackowania undo per moduł).
- Deleguje do wewnętrznych funkcji core/DB; NIE wywołuje innych metod App (brak podwójnego undo).

**Moduły MVP**:

| Moduł | Domyślnie | Implementacja |
|---|---|---|
| Matchmaking Regions | ON | `core.SetUnlockedRegions` dla wszystkich 104 regionów z DB |
| Colosseums | OFF | `data.ColosseumFlagSets` + `db.SetEventFlag` dla 3 aren + flagi globalne |
| Map Reveal | OFF | `revealBaseMap` + `revealDLCMap` (funkcje package-level) |
| Summoning Pools | OFF | `db.SetEventFlag` dla wszystkich ID 670xxx z DB |
| Sites of Grace | OFF (disabled) | Placeholder — checkbox disabled w UI, backend zwraca warning "planned" |

**Poza zakresem MVP (Faza 3+)**:
- Presety build postaci (Moduł A)
- Skrót questa Varre (Moduł C)
- Klasyfikator stanu BF / diagnostyki UD10+UD0 (Moduł F)
- Inspektor UD11 / NetworkParam (Moduł F)
- Brak patchowania UD10, brak patchowania UD11

### Profile przygotowania

Selektor profilu wyświetlany jest nad checklistą modułów. Profile są wyłącznie warstwą wygody w UI — ustawiają checkboxy modułów i nie wprowadzają nowego API backendu ani formatu presetów.

| Profil | matchmakingRegions | colosseums | revealMap | summoningPools | sitesOfGrace |
|---|---|---|---|---|---|
| Minimal PvP Ready (domyślny) | ON | OFF | OFF | OFF | OFF |
| Full PvP Convenience | ON | ON | ON | OFF | OFF |
| Co-op Ready | OFF | OFF | ON | ON | OFF |
| Custom | (zachowane) | (zachowane) | (zachowane) | (zachowane) | OFF |

**Zasady:**
- Stan domyślny przy otwieraniu zakładki: **żaden moduł nie jest zaznaczony** (wszystkie checkboxy odznaczone). Profil pokazuje `Custom`.
- Kliknięcie nazwanego profilu ustawia wszystkie checkboxy zgodnie z profilem.
- Ręczna zmiana dowolnego checkboxa automatycznie wybiera `Custom` (wyliczane z aktualnych opts, nie przechowywane jako osobny stan).
- Kliknięcie `Custom` nie ma efektu — jest to wskaźnik stanu tylko do odczytu.
- `Sites of Grace` jest zawsze `OFF` we wszystkich profilach — to wyłączony placeholder.
- Profile nie są pełnym systemem presetów budowania postaci (planowane jako osobna funkcja).

**Persystencja sesji i odświeżanie WorldTab:**
- Opcje PvP Preparation są przechowywane jako zwykły interfejs `PvPOptions` w stanie `App.tsx`, zachowując się podczas nawigacji między zakładkami. Nie wymaga localStorage.
- `PvPPreparationTab` jest w pełni kontrolowanym komponentem — cały stan checkboxów płynie przez props `pvpOpts` i callback `onPvpOptsChange`. Brak lokalnego stanu opcji w komponencie.
- Kliknięcie Apply wywołuje `handlePvPMutate`, który inkrementuje współdzielony licznik `saveDataRevision` w `App.tsx`. Licznik jest przekazywany do `WorldTab` i uwzględniony w tablicy zależności `useCallback` funkcji `loadExplorationData`, co powoduje natychmiastowe przeładowanie danych.
- Nie zmienia logiki zapisu w backendzie — `ApplyPvPPreparation` w `app_pvp.go` pozostaje bez zmian.

**Nowy plik testowy**: `pvp_test.go` (pakiet `main`) — 7 testów: brak save, nieprawidłowy slot, pusty slot, zły offset flag, warning SitesOfGrace, warning Colosseums, warning SummoningPools.

---

## 8.1 Network Speed Panel

**Komponent**: `frontend/src/components/NetworkSpeedPanel.tsx`

Renderowany wewnątrz `PvPPreparationTab.tsx`, pod checklistą modułów, oddzielony separatorem.
Owinięty w `AccordionSection` — **domyślnie zwinięty**.

**Co robi:**
- Patchuje UD11 NetworkParam wewnątrz pliku save.
- Efekt jest **globalny dla całego save'a** — wszystkie sloty postaci dzielą UD11.
- **Wymaga przeładowania postaci** do aktywacji: wczytaj postać → wyjdź do menu głównego → wczytaj ponownie.
- Serwer nadpisuje UD11 przy następnym połączeniu online (efekt jest tymczasowy).
- Agresywne ustawienia mogą zwiększać ryzyko wykrycia online.

**Presety:**

| Preset | `maxBreakInTargetListCount` | `breakInRequestIntervalTimeSec` | `breakInRequestTimeOutSec` | `breakInRequestAreaCount` | Ryzyko | Backend |
|--------|---|---|---|---|---|---|
| Vanilla | 5 | 30.0 | 20.0 | 5 | Brak | `ResetNetworkParams()` |
| Light / Safer | 8 | 10.0 | 8.0 | 5 | Niższe | `GetNetworkPreset("light-invasions")` → `SetNetworkParams()` |
| Fast Invasions | 10 | 4.0 | 4.0 | 5 | Wyższe | `GetNetworkPreset("fast-invasions")` → `SetNetworkParams()` |

`breakInRequestAreaCount` pozostaje vanilla=5. Nie eksponowane w MVP.

**Nowy backend:**
- `backend/core/regulation.go`: `NetworkParamLightInvasions()` — fabryka presetu Light/Safer.
- `app.go` `GetNetworkPreset()`: dodano przypadek `"light-invasions"`.

**Brak nowych metod app.go.** Używa istniejących: `GetNetworkPreset`, `SetNetworkParams`, `ResetNetworkParams`.

**NIE patchuje:** UD10, UD0, flag world-state, slotów postaci.

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
| `spec/46-faster-invasions-research.md` | Werdykt końcowy: patch UD11 potwierdzony skuteczny po przeładowaniu postaci; Track A (timery UD10) — nieskuteczny |
| `spec/47-site-of-grace-activation.md` | Odblokowanie gracji potwierdzone dla mapy/fast-travel; animacja in-world otwarta |
| `spec/44-network-param-tuning.md` | Referencja pól NetworkParam |
| `spec/32-ban-risk-system.md` | Definicje Tier 0/1/2 |
| `tmp/coloseum-debug/` | Hex-zweryfikowane zestawy flag colosseums |
