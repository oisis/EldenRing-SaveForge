# 19 — Weather & Time

> **Type**: Binary format spec (read-only reference)
> **Scope**: `WorldAreaWeather` (12 B) + `WorldAreaTime` (12 B) — co kod parsuje, dlaczego nie ma settera, jakie są ryzyka edycji.

Cross-refs: [14-game-state.md](14-game-state.md), [16-world-state.md](16-world-state.md).

---

## 1. Cel rozdziału

Zdefiniować jednoznacznie:

- co kod parsuje jako stan pogody (`WorldAreaWeather`, 12 B),
- co kod parsuje jako czas (`WorldAreaTime`, 12 B),
- dlaczego SaveForge **nie udostępnia** publicznego endpointu do edycji pogody/czasu,
- jakie ryzyka wiążą się z manualną mutacją tych pól.

Nie powiela World State overview (patrz [16-world-state.md](16-world-state.md)).

## 2. Status

| Aspekt | Status |
|---|---|
| `WorldAreaWeather` struct parser | ✅ `backend/core/section_trailing.go:7-45` |
| `WorldAreaWeatherSize = 12` | ✅ `section_trailing.go:14` |
| `WorldAreaTime` struct parser | ✅ `backend/core/section_trailing.go:48-79` |
| `WorldAreaTimeSize = 12` | ✅ `section_trailing.go:54` |
| Read/Write verbatim w `TrailingFixedBlock` | ✅ round-trip preserved |
| App-level public endpoint (`SetWeather`, `SetTime`, itp.) | ❌ **brak** — `grep` w `app*.go` zwraca 0 wyników |
| Frontend UI dla pogody/czasu | ❌ brak |
| Test coverage | ✅ `section_trailing_test.go` — `TestTrailingRoundTripPS4`/`PC` (2 testy, pokrywają cały `TrailingFixedBlock`) |

## 3. Source of truth w kodzie

| Plik / symbol | Co zawiera | Tryb |
|---|---|---|
| `backend/core/section_trailing.go::WorldAreaWeather` (linie 7-12) | 12 B struct: `AreaID + WeatherType + Timer + Padding` | read/write verbatim |
| `backend/core/section_trailing.go::WorldAreaWeatherSize` (linia 14) | `= 12` | const |
| `backend/core/section_trailing.go::WorldAreaWeather.Read/Write` (linie 16-45) | Parser + serializer pól | RW |
| `backend/core/section_trailing.go::WorldAreaTime` (linie 48-52) | 12 B struct: `Hour + Minute + Second` (3× u32) | read/write verbatim |
| `backend/core/section_trailing.go::WorldAreaTimeSize` (linia 54) | `= 12` | const |
| `backend/core/section_trailing.go::WorldAreaTime.Read/Write` (linie 56-79) | Parser + serializer | RW |
| `backend/core/section_trailing.go::TrailingFixedBlock` (linie 187-196) | Łączna struktura: `Weather + Time + BaseVersion + SteamID + PS5Activity + DLC` (130 B total) | aggregate |
| `backend/core/section_trailing_test.go` | 2 round-trip testy (PS4 + PC) — pokrywają cały `TrailingFixedBlock` | tests |

Obie struktury są częścią `TrailingFixedBlock` — bloku końcowego slotu zawierającego również `BaseVersion`, `SteamID`, `PS5Activity`, `DLCSection`. Ten rozdział opisuje **tylko** Weather + Time; pozostałe pola są w innych rozdziałach.

## 4. Mental model

```
TrailingFixedBlock (130 B total)
  ├─ WorldAreaWeather (12 B)
  │    ├─ AreaID       u16  (2 B)  → identyfikator regionu pogodowego
  │    ├─ WeatherType  u16  (2 B)  → typ pogody (in-game weather enum)
  │    ├─ Timer        u32  (4 B)  → czas trwania aktualnej pogody
  │    └─ Padding      u32  (4 B)  → zachowywane verbatim
  ├─ WorldAreaTime (12 B)
  │    ├─ Hour    u32  (4 B)  → godzina (0-23)
  │    ├─ Minute  u32  (4 B)  → minuta (0-59)
  │    └─ Second  u32  (4 B)  → sekunda (0-59)
  ├─ BaseVersion (16 B)   → patrz inne sekcje
  ├─ SteamID (8 B)        → patrz [01-header.md] / steamid section
  ├─ PS5Activity (32 B)   → trophy/activity data
  └─ DLCSection (50 B)    → DLC entry flag — patrz [29]
```

Aktualny kod traktuje obie sekcje jako **read-write verbatim** — load parsuje na typowane pola, save zapisuje 1:1 te same bajty. Brak walidacji ani transformacji.

## 5. Current parsed data

### 5.1 `WorldAreaWeather`

| Pole | Typ | Rozmiar | Komentarz |
|---|---|---|---|
| `AreaID` | `u16` | 2 B | Identyfikator regionu pogodowego (in-game area ID) |
| `WeatherType` | `u16` | 2 B | Typ pogody — wartości mapują się na in-game weather enum |
| `Timer` | `u32` | 4 B | Czas trwania aktualnej pogody (runtime counter) |
| `Padding` | `u32` | 4 B | Zachowywane verbatim |

**Suma**: `2 + 2 + 4 + 4 = 12 B`.

`needs verification`:

- Mapowanie wartości `WeatherType` → human-readable nazwy (rain/sunny/snow/itp.) — nie istnieje w kodzie SaveForge.
- Granica `AreaID` (które wartości są legalne) — niezweryfikowana; brak walidatora przeciw `regulation.bin`.
- Semantyka `Timer` (jednostka — frames/seconds/ms?) — niezweryfikowana.
- Czy `Padding` rzeczywiście jest paddingiem czy ukrytym polem — niezweryfikowane.

### 5.2 `WorldAreaTime`

| Pole | Typ | Rozmiar | Komentarz |
|---|---|---|---|
| `Hour` | `u32` | 4 B | Godzina (in-game) |
| `Minute` | `u32` | 4 B | Minuta |
| `Second` | `u32` | 4 B | Sekunda |

**Suma**: `4 + 4 + 4 = 12 B`.

`needs verification`:

- Czy `Hour` to 0-23 (24-godzinny) czy inna konwencja — komentarz w kodzie nie precyzuje.
- Czy `Minute`/`Second` mają zakres standardowy (0-59) czy gra dopuszcza inne wartości — niezweryfikowane.
- Czy 32-bitowe typy są celowe (gra prawdopodobnie używa mniejszych) — historyczna kompatybilność z parser format.

## 6. Read-only status

**SaveForge nie udostępnia publicznego endpointu do edycji pogody ani czasu.**

`grep -rn "WorldAreaWeather\|WorldAreaTime\|setWeather\|setTime" app*.go` zwraca **0 wyników** dla wszystkich plików `app*.go`. Public API (Wails bindings) **nie pozwala**:

- ustawić `AreaID`, `WeatherType`, `Timer`, `Padding`,
- ustawić `Hour`, `Minute`, `Second`,
- ustawić jakichkolwiek pól w obu strukturach.

Frontend (`frontend/src/components`) nie ma żadnego komponentu wyświetlającego lub edytującego pogodę/czas.

Każda mutacja musiałaby być wykonana ręcznie przez direct memory hex edit poza SaveForge — patrz §8 dla ryzyk.

## 7. What SaveForge does not implement

| Funkcja | Status |
|---|---|
| `SetWeather(areaID, weatherType)` | ❌ nie ma |
| `SetTime(hour, minute, second)` | ❌ nie ma |
| `ResetWeatherTimer()` | ❌ nie ma |
| Walidacja `AreaID` (czy istnieje w grze) | ❌ nie ma |
| Walidacja `WeatherType` przeciw `regulation.bin` weather enum | ❌ nie ma |
| Walidacja `Hour` 0-23 / `Minute` 0-59 / `Second` 0-59 | ❌ nie ma |
| Frontend UI dla edycji pogody/czasu | ❌ nie ma |
| Tabela mapowania `WeatherType` → nazwa (rain/sunny/itp.) | ❌ nie ma |
| Detekcja „corrupted" weather/time (stara doc miała) | ❌ usunięta — heurystyka „area_id == 0 = korupcja" nie ma code-confirmation |

`needs verification`: czy istnieje planowana funkcja edycji pogody/czasu — nie znaleziono w `docs/ROADMAP.md` ani w issue tracker. Aktualnie wszystkie zmiany muszą być wykonane przez grę (czas postępuje runtime; pogoda zmienia się scriptami).

## 8. Relation to World State

Patrz [16-world-state.md](16-world-state.md) §6.1 + §5. `WorldAreaWeather` i `WorldAreaTime` są **odrębnymi** sekcjami w `TrailingFixedBlock` — nie częścią `WorldGeomBlock` (`FieldArea / WorldArea / WorldGeomMan / WorldGeomMan2 / RendMan`).

Tabela World State subsystem map ([16](16-world-state.md) §5) klasyfikuje `WorldAreaWeather / Time` jako osobny rozdział z trybem „RO verbatim" — w aktualnym kodzie de facto **RO** (read/write verbatim w I/O round-trip, brak app-level mutatora).

`needs verification`: czy gra przy load wymusza spójność pogody/czasu z `WorldGeomMan` blobami (script state może być powiązany). Niezweryfikowane.

## 9. Validation and safety notes

### 9.1 Manualna mutacja Weather

Bez app-level endpointu jedyna ścieżka edycji to direct hex edit. Ryzyka:

- **Wrong `AreaID`** → gra może próbować załadować weather script dla nieistniejącego regionu → undefined behavior, prawdopodobnie no-op lub default weather.
- **Wrong `WeatherType`** → poza zakresem game weather enum → gra może crashować lub fallback do default.
- **Timer rozsynchonizowany** z weather scriptem → możliwe desync wizualne (np. pogoda zmienia się natychmiast po load).

`needs verification`: czy gra ma defensive bounds-checking dla `WeatherType` enum, czy zaufa save'owi blind.

### 9.2 Manualna mutacja Time

- **Hour > 23** → undefined behavior; gra prawdopodobnie modulo 24 lub crash.
- **Minute/Second > 59** → jw.
- **Time rozsynchonizowany** z dynamicznym in-game day cycle script → krótkotrwałe desync (gra dostosuje przy następnym tick).

`needs verification`: czy zmiana time wpływa na spawn NPC / event timing (stara doc twierdziła „wpływa na spawny NPC i eventów", ale to było bez code/runtime verification).

### 9.3 Coupling z World State scripts

`WorldAreaWeather` może być częścią szerszego script state w `WorldGeomMan` blobie (patrz [16-world-state.md](16-world-state.md) §6.1). Zmiana weather samodzielnie, bez aktualizacji powiązanego script state, może spowodować desync visual vs script.

`needs verification`: czy `WorldGeomMan` zawiera weather-related state które wymaga sync z `WorldAreaWeather`. Brak izolowanego testu.

### 9.4 No write endpoint + no in-game verification

Brak publicznego endpointu oznacza:

- Brak risk gate w UI (Tier 0/1/2).
- Brak rollback hook (`pushUndo` etc.).
- Brak walidacji per-pole.
- Brak CI testu „set weather → reload → assert state".

Każda zmiana wykonana spoza SaveForge (np. hex editor) jest **całkowicie nieweryfikowana** przez SaveForge w I/O round-trip — tylko bajty są preserved, semantyka jest na odpowiedzialności użytkownika.

### 9.5 Platform / version differences

Oba `WorldAreaWeatherSize = 12` i `WorldAreaTimeSize = 12` są stałe dla PC i PS4 (sloty obu platform mają identyczny `TrailingFixedBlock` layout).

`needs verification`: czy przyszłe patche gry mogą rozszerzyć `WorldAreaWeather` (np. dodać extra fields). Brak detection mechanism.

### 9.6 Stara heurystyka „korupcja" — usunięta

Stara doc PL twierdziła „`Area ID == 0` → pogoda jest prawdopodobnie uszkodzona" i „`Hour == 0 AND Minute == 0 AND Second == 0` → czas potencjalnie uszkodzony". W aktualnym kodzie **brak** takiej walidacji — SaveForge nie sygnalizuje korupcji weather/time w żadnym endpoint diagnostic. Heurystyki były tylko sugestiami dla zewnętrznych narzędzi, nie code-confirmed; usunięte z tego rozdziału.

## 10. Test coverage

| Test | Plik | Co weryfikuje |
|---|---|---|
| `TestTrailingRoundTripPS4` | `backend/core/section_trailing_test.go:75` | Load → Write → diff = 0 dla całego `TrailingFixedBlock` (PS4); pokrywa Weather + Time + BaseVersion + SteamID + PS5Activity + DLC |
| `TestTrailingRoundTripPC` | `backend/core/section_trailing_test.go:79` | jw. dla PC save |

**Brak**:

- Izolowanego testu walidacji `WeatherType` enum.
- Izolowanego testu walidacji `Hour`/`Minute`/`Second` zakresów.
- Izolowanego testu mutation + reload (bo brak mutatora).
- Testu spójności Weather/Time z `WorldGeomMan` script state.

## 11. Known limits / needs verification

1. **`WeatherType` → human-readable mapowanie** — brak w SaveForge; `regulation.bin` mógłby zostać sparsowany dla tego.
2. **`AreaID` weather-region semantyka** — niezweryfikowana, brak mapowania na `data.Regions`.
3. **`Timer` jednostka** — niezweryfikowana (frames/sec/ms?).
4. **`Padding` u32 w `WorldAreaWeather`** — niezweryfikowane, prawdopodobnie padding ale może być hidden field.
5. **`Hour`/`Minute`/`Second` zakresy** — gra prawdopodobnie wymusza 0-23/0-59/0-59 ale brak walidatora w SaveForge.
6. **Coupling z `WorldGeomMan` script state** — niezweryfikowane, możliwy desync source.
7. **Brak app-level Set API** — aktualny stan; przyszłe wdrożenie wymagałoby walidatora + risk gate.
8. **In-game spawn/event timing impact** — stara doc twierdziła „wpływa na spawny NPC i eventów"; aktualnie `needs verification`.
9. **Cross-version layout stability** — brak detection mechanism po patchu.
10. **Stary „korupcja" heurystyka** — usunięta (była bez code-confirmation).

## 12. Cross-references

- [01-header.md](01-header.md) — SteamID w `TrailingFixedBlock` (osobne pole 8 B).
- [14-game-state.md](14-game-state.md) — `PreEventFlagsScalars` (`InGameCountdownTimer`, `LastRestedGrace` itp.); osobny block.
- [16-world-state.md](16-world-state.md) — overview World State; Weather/Time klasyfikowane jako RO verbatim.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — `DLCSection` w `TrailingFixedBlock`; osobne pole.

## 13. Sources

- `backend/core/section_trailing.go` — `WorldAreaWeather` (12 B), `WorldAreaTime` (12 B), `TrailingFixedBlock` aggregator.
- `backend/core/section_trailing_test.go` — 2 round-trip testy (PS4 + PC).
- Brak callerów w `app*.go` (verified via `grep`).
- Brak komponentów frontendowych (verified via `grep`).
