# 44 — Tuning parametrów sieciowych (NetworkParam)

> **Type**: Design doc
> **Status**: ✅ Zaimplementowane
> **Scope**: Kompletna referencja pól NETWORK_PARAM_ST z regulation.bin istotnych dla tuningu PvP/multiplayer.

## Przegląd

`NetworkParam.param` to jednowierszowa tabela PARAM wewnątrz `regulation.bin` (osadzonego w UserData11).
Kontroluje wszystkie czasy matchmakingu, limity wyszukiwania i zachowanie multiplayer po stronie klienta.

Dane Row 0 zaczynają się na offsecie `0x58` w pliku .param. Wszystkie pola little-endian.

## Architektura

```
UserData11 → AES-256-CBC deszyfracja → DCX dekompresja → BND4 rozpakowanie → NetworkParam.param → Row 0
```

Różnice platform:
- **PC**: DCX używa kompresji ZSTD
- **PS4**: DCX używa DFLT (zlib)

## Grupy pól

### Grupa 1: Znaki przywołania (NET_SUMMON_SIGN_PARAM)

Kontroluje widoczność, częstotliwość odświeżania i pobieranie znaków (białych/złotych/czerwonych).

| Pole | Typ | Offset | Vanilla | Opis |
|---|---|---|---|---|
| `signPuddleActiveMessageIntervalSec` | f32 | 0x10 | 30.0 | Interwał powiadomienia "summoning pool aktywny" |
| `reloadSignIntervalTime2` | f32 | 0x1C | 60.0 | Interwał odświeżania listy znaków (tryb normalny). Niżej = znaki pojawiają się szybciej |
| `reloadSignTotalCount` | u32 | 0x20 | 20 | Max znaków pobieranych na raz (globalnie). Wyżej = widzisz więcej znaków |
| `reloadSignCellCount` | u32 | 0x24 | 10 | Max znaków per komórka mapy. Wyżej = gęstsza widoczność w okolicy |
| `updateSignIntervalTime` | f32 | 0x28 | 30.0 | Jak często TWÓJ postawiony sign jest aktualizowany na serwerze |
| `signDownloadSpan` | f32 | 0x64 | 30.0 | Interwał ściągania listy znaków |
| `signUpdateSpan` | f32 | 0x68 | 60.0 | Interwał uploadu danych znaków na serwer |
| `singGetMax` | u32 | 0x60 | 32 | Twardy limit pobranych znaków |

**Uwagi dot. tuningu:**
- `reloadSignIntervalTime2` ma największy wpływ — zmniejszenie z 60→10s = znaki pojawiają się 6x szybciej
- `reloadSignTotalCount` i `singGetMax` powinny być podnoszone razem
- Bezpieczne do modyfikacji: wpływają tylko na agresywność pollowania danych sign

### Grupa 2: Inwazje (NET_BREAKIN_PARAM)

Kontroluje matchmaking inwazji. **Zaimplementowane w v0.8.0.**

| Pole | Typ | Offset | Vanilla | Opis |
|---|---|---|---|---|
| `maxBreakInTargetListCount` | u32 | 0x70 | 5 | Kandydaci na cel inwazji per wyszukiwanie |
| `breakInRequestIntervalTimeSec` | f32 | 0x74 | 30.0 | Opóźnienie między ponownymi requestami matchmakingu |
| `breakInRequestTimeOutSec` | f32 | 0x78 | 20.0 | Timeout per request matchmakingu |
| `breakInRequestAreaCount` (niepotwierdzone) | u32 (kod) / u8 (defy) | 0x7C | 5 | **Semantyka NIEPOTWIERDZONA.** W lokalnym PARAMDEF oznaczone `dummy8 pad[4]` ("予約"/Rezerwa), w społecznościowym defie TGA `u8 unknown_0x7c` — żadne zewnętrzne źródło nie nazywa go `breakInRequestAreaCount`; ta nazwa istnieje tylko w SaveForge. Wartość vanilla w binarce to `5` (zweryfikowane). Presety SaveForge zawsze pozostawiają `5`; pole jest dostępne wyłącznie jako Experimental w UI. Nie traktuj go jako potwierdzonego pokrętła „area search". |

> **Nota o źródle prawdy**: wartości vanilla powyżej pochodzą z binarki `NetworkParam.param` (Row 0). Wyeksportowany `csv/NetworkParam.csv` pokazuje dla grupy znaków `reloadSignTotalCount=32` / `reloadSignCellCount=8`, co jest **błędne** — binarka (i `NetworkParamDefaults()`) ma `20` / `10`. Binarka jest autorytatywna. Inwazje nie mają też żadnego pola `allAreaSearchRate_*` (te istnieją tylko dla CoopBlue/VsBlue/BellGuard), więc nie ma potwierdzonego pokrętła local-vs-worldwide dla break-in search.

### Grupa 3: System Blue Phantom (NET_VISIT_PARAM)

Kontroluje automatyczne przywołanie przez Blue Cipher Ring i system Hunterów.

| Pole | Typ | Offset | Vanilla | Opis |
|---|---|---|---|---|
| `reloadVisitListCoolTime` | f32 | 0x180 | 20.0 | Cooldown między wyszukiwaniami blue phantomów. Niżej = szybsze dopasowanie blue |
| `maxCoopBlueSummonCount` | u32 | 0x184 | 2 | Max blue phantomów szukanych jednocześnie przez ring system. Serwer wymusza faktyczny limit sesji — to wpływa tylko na równoległość wyszukiwania klienta |
| `maxBellGuardSummonCount` | u32 | 0x188 | 4 | Max kandydatów Bell Guard (obrońca strefy) |
| `maxVisitListCount` | u32 | 0x18C | 5 | Liczba celów wizyty pobieranych per wyszukiwanie |
| `reloadSearch_CoopBlue_Min` | f32 | 0x190 | 30.0 | Min opóźnienie między reload searchami co-op blue |
| `reloadSearch_CoopBlue_Max` | f32 | 0x194 | 180.0 | Max opóźnienie (losowane między min/max) |
| `reloadSearch_BellGuard_Min` | f32 | 0x198 | 120.0 | Min opóźnienie Bell Guard reload |
| `reloadSearch_BellGuard_Max` | f32 | 0x19C | 240.0 | Max opóźnienie Bell Guard reload |

**Uwagi dot. tuningu:**
- `maxCoopBlueSummonCount` = 4 jest bezpieczne. Serwer limituje faktyczne joiny do dostępnych slotów sesji. Więcej kandydatów = szybsze prawdopodobieństwo pierwszego matcha.
- `reloadSearch_CoopBlue_Min/Max` ma największy wpływ na "jak długo czekasz na blue" — zmniejszenie z 30-180s do 5-20s jest drastyczne.
- `allAreaSearchRate_*` (w sekcji Extra) kumuluje się z tym — 100% = szukaj globalnie, nie tylko w pobliżu.

### Grupa 4: Extra / Różne (NET_EXTRA_PARAM)

| Pole | Typ | Offset | Vanilla | Opis |
|---|---|---|---|---|
| `darkPhantomLimitBoostTime` | f32 | 0x1BC | 60.0 | ⚠️ **LEGACY/NIEZWERYFIKOWANE** — mechanika DS3: po N sekundach timer invadera przyspiesza. W ER invaderzy nie mają obserwowanego limitu czasu — to pole prawdopodobnie nie działa |
| `darkPhantomLimitBoostScale` | f32 | 0x1C0 | 1.2 | ⚠️ **LEGACY/NIEZWERYFIKOWANE** — mechanika DS3: mnożnik przyspieszenia timera. Prawdopodobnie nieaktywne w ER |
| `multiplayDisableLifeTime` | f32 | 0x1C4 | 1800.0 | Jak długo multiplayer jest wyłączony po pewnych eventach [s] |
| `phantomWarpMinimumTime` | u8 | 0x1C9 | 6 | Min czas zanim phantom może się teleportować [s] |
| `phantomReturnDelayTime` | u8 | 0x1CA | 2 | Opóźnienie po użyciu Black Crystal przed powrotem [s] |
| `terminateTimeoutTime` | u8 | 0x1CB | 30 | Timeout detekcji rozłączenia [s] |
| `penaltyPointLanDisconnect` | u16 | 0x1CC | 10 | Punkty kary za rozłączenie LAN |
| `penaltyPointSignout` | u16 | 0x1CE | 0 | Punkty kary za signout (vanilla=0, brak kary) |
| `penaltyPointReboot` | u16 | 0x1D0 | 10 | Punkty kary za twardy restart/wyłączenie |
| `penaltyPointBeginPenalize` | u16 | 0x1D2 | 100 | Próg: kary aktywują się gdy punkty >= ta wartość |
| `penaltyForgiveItemLimitTime` | f32 | 0x1D4 | 36000.0 | Cooldown Way of White Circlet [s]. 36000 = 10 godzin |
| `allAreaSearchRate_CoopBlue` | u8 | 0x1D8 | 30 | % szans na przeszukanie WSZYSTKICH stref dla co-op blue (vs tylko lokalne) |
| `allAreaSearchRate_VsBlue` | u8 | 0x1D9 | 30 | % szans na globalny search retribution blue |
| `allAreaSearchRate_BellGuard` | u8 | 0x1DA | 30 | % szans na globalny search Bell Guard |
| `bloodMessageEvalHealRate` | u8 | 0x1DB | 20 | % leczenia HP gdy twoja wiadomość jest oceniona |
| `signDisplayMax` | u8 | 0x1E4 | 10 | Max znaków renderowanych jednocześnie |

**Uwagi dot. tuningu:**
- `darkPhantomLimitBoostTime/Scale`: **Prawdopodobnie legacy z Dark Souls 3.** Invaderzy w ER nie mają obserwowalnego timera sesji — mogą campować bez limitu aż host umrze, odpocznie lub wejdzie w boss fog. Pola istnieją w strukturze ale prawdopodobnie nie są czytane przez logikę gry ER. **Nie uwzględniać w presetach.**
- `allAreaSearchRate_*` na 100% = zawsze szukaj globalnie. Drastycznie przyspiesza przybycie blue phantomów.
- `penaltyPoint*`: ustawienie na 0 usuwa kary za rozłączenie po stronie klienta. **WYSOKIE RYZYKO BANA** — serwer może walidować te wartości.

### Grupa 5: Quick Match / Koloseum (QUICK_MATCH)

| Pole | Typ | Offset | Vanilla | Opis |
|---|---|---|---|---|
| `hostRegisterUpdateTime` | f32 | 0x1FC | 60.0 | Periodyczny status update hosta do serwera |
| `hostTimeOutTime` | f32 | 0x200 | 30.0 | Jak długo host czeka na gościa |
| `requestSearchQuickMatchLimit` | u32 | 0x210 | 5 | Max wyników per wyszukiwanie quick match |

### Grupa 6: System Visitor (VISITOR)

Kontroluje mechanikę Taunter's Tongue / summoning pool visitors.

| Pole | Typ | Offset | Vanilla | Opis |
|---|---|---|---|---|
| `VisitorListMax` | u32 | 0x240 | 10 | Max wpisów na liście celów visitor |
| `VisitorTimeOutTime` | f32 | 0x244 | 60.0 | Timeout oczekiwania na visitora [s] |
| `DownloadSpan` | f32 | 0x248 | 60.0 | Interwał pobierania listy visitorów [s] |

## Ocena ryzyka bana

| Ryzyko | Parametry | Uzasadnienie |
|---|---|---|
| **Niskie** | Grupa 1 (znaki), Grupa 5 (quick match) | Tylko częstotliwość pollingu po stronie klienta; serwer widzi normalny ruch |
| **Umiarkowane** | Grupa 3 (`maxCoopBlueSummonCount`, `allAreaSearchRate`), Grupa 6 (visitor timings) | Zmienia widoczne zachowanie matchmakingu ale nie łamie protokołu |
| **Wysokie** | Grupa 4 (`penaltyPoint*`, `penaltyForgiveItemLimitTime`) | Serwer prawdopodobnie waliduje stan kar; rozbieżność = flagowanie |

## Presety funkcjonalne (zaimplementowane — v0.10: Vanilla / Faster / Aggressive)

Trzy grupy przypisane do ról, każda z **trzema modułowymi presetami** (`Vanilla`, `Faster`,
`Aggressive`), zdefiniowane raz w `backend/core/regulation.go` i pobierane przez frontend via
`GetNetworkPreset` (jedno źródło prawdy — frontend i backend nie mogą się rozjechać).

Reguła modułowości: przycisk grupy zapisuje wyłącznie stabilne pola tej grupy
(`NETWORK_GROUP_KEYS[group]`). Zastosowanie presetu jednej grupy nigdy nie resetuje wartości
innej grupy, ręcznego tuningu użytkownika ani żadnego pola Experimental. `Aggressive` to
*mocniejszy profil do testów/tuningu* — **nie** jest to stary usunięty globalny `Aggressive`
(który obcinał `breakInRequestTimeOutSec` do 3s, psuł matchmaking blisko-i-daleko, prowadził
niemal ciągłą pętlę retry i zapisywał 0x7C). Nie ma cross-group Aggressive ani `aggressive-host`.

### Reds / Invader — `NetworkParam{Faster,Aggressive}Reds()`

| Pole | Vanilla | Faster | Aggressive |
|---|---:|---:|---:|
| `maxBreakInTargetListCount` | 5 | 8 | 12 |
| `breakInRequestIntervalTimeSec` | 30 | 12 | 8 |
| `breakInRequestTimeOutSec` | 20 | 15 | 12 |

Proxy liczby target-candidates na minutę: Vanilla `(60/30)×5 = 10`, Faster `(60/12)×8 = 40`,
Aggressive `(60/8)×12 = 90`. Aggressive zachowuje bezpieczny timeout 12s (nie stare 3s) i nigdy
nie rusza `breakInRequestAreaCount` (0x7C) — zob. pola Experimental niżej.

### Summon Signs — `NetworkParam{Faster,Aggressive}Summons()`

| Pole | Vanilla | Faster | Aggressive |
|---|---:|---:|---:|
| `reloadSignIntervalTime2` | 60 | 20 | 10 |
| `reloadSignTotalCount` | 20 | 40 | 64 |
| `reloadSignCellCount` | 10 | 20 | 32 |
| `updateSignIntervalTime` | 30 | 15 | 10 |
| `singGetMax` | 32 | 64 | 96 |
| `signDownloadSpan` | 30 | 15 | 10 |
| `signUpdateSpan` | 60 | 20 | 10 |

Wymuszony invariant (backend + clamp UI): `reloadSignCellCount ≤ reloadSignTotalCount ≤ singGetMax`
(Aggressive: 32 ≤ 64 ≤ 96). Ta grupa kontroluje wyłącznie **ścieżkę sieciową znaków przywołania** —
**aktywacja Summoning Pools jest osobną funkcją World / Exploration**, konfigurowaną w zakładce
World, nie tutaj. Wiele pooli działa w grze po wcześniejszych poprawkach, ale pełna kompletność
nie jest jeszcze formalnie zweryfikowana; znaki publikowane przez poole mogą korzystać z tej
ścieżki sieciowej, ale efekt nie jest formalnie zweryfikowany. `cellGroupHorizontalRange` /
`cellGroupTopRange` / `cellGroupBottomRange` pozostają **przyszłymi kandydatami Experimental**
dla tej grupy — niezaimplementowane (brak patchingu, brak UI).

### Blue / Hunter — `NetworkParam{Faster,Aggressive}Blue()`

| Pole | Vanilla | Faster | Aggressive |
|---|---:|---:|---:|
| `reloadVisitListCoolTime` | 20 | 8 | 5 |
| `reloadSearchCoopBlueMin` | 30 | 10 | 5 |
| `reloadSearchCoopBlueMax` | 180 | 40 | 20 |
| `maxVisitListCount` | 5 | 10 | 15 |
| `allAreaSearchRateCoopBlue` | 30 | 60 | 100 |

Wymuszone invarianty: `reloadSearchCoopBlueMin ≤ reloadSearchCoopBlueMax` oraz
`allAreaSearchRateCoopBlue` pozostaje w `0–100`. Testuj po stronie huntera na postaci z aktywnym
Blue Cipher Ring. `maxCoopBlueSummonCount` i `allAreaSearchRateVsBlue` **nie** należą do tej
grupy — są Experimental (zob. niżej).

### Pola Experimental (per grupa, nigdy nie zmieniane przez presety)

Dawna pojedyncza sekcja UI „Experimental" jest usunięta; pola żyją teraz w obrębie swojej grupy
funkcjonalnej, każde oznaczone `Experimental`, i **żaden aktywny preset (Vanilla / Faster /
Aggressive) ich nie zapisuje**:

- **Reds:** `breakInRequestAreaCount` (0x7C) — nieznane pole break-in pod 0x7C. Znaczenie
  gameplayowe niepotwierdzone; wartość vanilla to 5; aktywne presety go nie modyfikują.
- **Blue:** `maxCoopBlueSummonCount` („Blue Search Parallelism") i `allAreaSearchRateVsBlue`
  („Retribution Global %"). Oba Experimental/niezweryfikowane w Elden Ring; aktywne presety Blue
  ich nie zmieniają.

**Pola Visitor** (`visitorListMax`, `visitorTimeOutTime`, `visitorDownloadSpan`) są **ukryte z
aktywnego UI** — brak potwierdzonego zastosowania i brak dowodu związku z Taunter's Tongue. Pola
backendowe i kompatybilność zapisu są zachowane (round-trip nietknięty); są po prostu nie
renderowane i nie należą do żadnego presetu.

### Setup testów PS5

1. Do testów Reds/Blue odblokuj pełną pulę base+DLC przez **World → Exploration → Invasion
   Regions → Unlock All** (274-regionowa skuratowana allowlista; zob. [11](11-regions.md)).
2. Aktywacja NetworkParam wymaga procedury drugiego loadu: skonfiguruj w SaveForge → skopiuj
   save na PS5 → wczytaj postać raz → wróć do menu głównego → wczytaj tę samą postać ponownie →
   dopiero wtedy testuj online (pierwszy load resetuje regulation.bin; drugi czyta go z save).
   Przyczyna zachowania pierwszy/drugi-load nie jest badana.
3. Dla Blue / Hunter testuj na postaci-hunterze z **aktywnym Blue Cipher Ring**.

### Niezaimplementowane (research-only)
- **Taunter's Tongue / reds po stronie hosta** — nie znaleziono potwierdzonego parametru tempa (Goods 108/178 → SpEffect 533 ustawia tylko stan sesji). Pola Visitor (`VisitorListMax/TimeOutTime/DownloadSpan`) należą do systemu visitor/ring-search i są dostępne wyłącznie jako Experimental „legacy ring-search fields".
- **Koloseum / arena** — brak dedykowanej tabeli matchmakingu; `AvatarMatchSearchMax` / `BattleRoyalMatchSearch*` oznaczone 未使用 (nieużywane); użycie `requestSearchQuickMatchLimit` w ER niepotwierdzone.
- **`NetworkAreaParam.cellSize*`** — high-risk (zmiana lokalnej siatki komórek może rozsynchronizować buckety matchmakingu względem serwera); poza zakresem.
- **`penaltyPoint*` / `penaltyForgiveItemLimitTime`** — wysokie ryzyko bana (serwer prawdopodobnie waliduje stan kar); świadomie nie oferowane jako preset.

## Źródła

- `tmp/regulation-bin-dump/defs/NetworkParam.xml` — schemat PARAMDEF (typy pól, nazwy, zakresy)
- `tmp/regulation-bin-dump/csv/NetworkParam.csv` — rzeczywiste wartości vanilla (Row ID 0)
- `backend/core/regulation.go` — implementacja pipeline'u read/patch
- Japońskie nazwy pól przetłumaczone wg konwencji nazewnictwa parametrów FromSoftware
