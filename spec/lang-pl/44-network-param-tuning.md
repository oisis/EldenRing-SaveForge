# 44 — Tuning parametrów sieciowych (NetworkParam)

> **Type**: Design doc
> **Status**: ✅ Zaimplementowane (częściowo — parametry invasion gotowe, rozszerzone planowane)
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

## Sugerowane presety

### "Fast Summons" (Niskie ryzyko)
```
reloadSignIntervalTime2:  60 → 10
reloadSignTotalCount:     20 → 64
reloadSignCellCount:      10 → 20
updateSignIntervalTime:   30 → 5
signDownloadSpan:         30 → 5
signUpdateSpan:           60 → 10
singGetMax:               32 → 64
```

### "Aggressive PvP" (Umiarkowane ryzyko)
Wszystko z "Fast Summons" plus:
```
reloadVisitListCoolTime:      20 → 5
maxCoopBlueSummonCount:        2 → 4
maxVisitListCount:             5 → 15
reloadSearch_CoopBlue_Min:    30 → 5
reloadSearch_CoopBlue_Max:   180 → 20
allAreaSearchRate_CoopBlue:   30 → 100
allAreaSearchRate_VsBlue:     30 → 100
VisitorListMax:               10 → 30
VisitorTimeOutTime:           60 → 120
DownloadSpan (Visitor):       60 → 10
```

### "No Penalty" (Wysokie ryzyko bana)
```
penaltyPointLanDisconnect:   10 → 0
penaltyPointReboot:          10 → 0
penaltyPointBeginPenalize:  100 → 9999
penaltyForgiveItemLimitTime: 36000 → 0
```

## Źródła

- `tmp/regulation-bin-dump/defs/NetworkParam.xml` — schemat PARAMDEF (typy pól, nazwy, zakresy)
- `tmp/regulation-bin-dump/csv/NetworkParam.csv` — rzeczywiste wartości vanilla (Row ID 0)
- `backend/core/regulation.go` — implementacja pipeline'u read/patch
- Japońskie nazwy pól przetłumaczone wg konwencji nazewnictwa parametrów FromSoftware
