# 51 — Advanced Save Editor

> **Typ**: Design doc
> **Status**: 🔲 Planowany
> **Zakres**: Jedna zakładka "Advanced → Save Editor" dla power-userów umożliwiająca odczyt/zapis znanych i eksperymentalnych wartości save'a w trzech technicznie różnych warstwach: Event Flags, regulation snapshot params i raw offsets.

---

## Przegląd

Jedna zakładka edytora dostępna pod **Advanced → Save Editor**, pozwalająca użytkownikowi budować listę oczekujących zmian, podglądać je i aplikować atomicznie.

Docelowi odbiorcy: badacze, moderzy, power-userzy potrzebujący dostępu do wartości nieeksponowanych przez główne zakładki edytora.

> **Ostrzeżenie pokazywane w UI:**
> Zaawansowany edytor. Nie zmieniaj wartości, jeśli nie wiesz dokładnie, co robią.
> Błędne wartości mogą uszkodzić save, zepsuć questy lub zwiększyć ryzyko egzekwowania bana online.
> Zawsze trzymaj kopię zapasową.

Checkbox potwierdzający ("Rozumiem, że to może uszkodzić mój save lub zwiększyć ryzyko bana") musi być zaznaczony, zanim **Apply changes** zostanie odblokowane.

---

## Kluczowe rozróżnienie

```
regulation.bin params ≠ event flags ≠ raw offsets
```

| Warstwa | Źródło | R/W |
|---|---|---|
| Event Flags | Bitfield EventFlags w slocie (1.8 MB) | R/W |
| Regulation Snapshot Params | Snapshot NETWORK_PARAM_ST w UD11 w slocie | R/W |
| Raw Offsets | Dowolna pozycja bajtowa UD10/UD11 | R/W (Faza 2+) |

Wszystkie są pokazywane w **jednej zakładce**, ale technicznie są to różne regiony pamięci save'a. UI musi to rozróżnienie komunikować jasno przez etykiety typów targetów.

---

## Układ UI

### Format wiersza (wpis oczekującej zmiany)

```
[Target dropdown/search] [Current/default value] [New value] [+]
```

### Lista oczekujących zmian

```
Pending changes:
- EventFlag 60290: false → true
- NetworkParam.breakInRequestIntervalTimeSec: 30.0 → 4.0
- RawOffset UD10+0x19524 f32: 0.033333 → 0.1

[Apply changes] [Reset] [Export patch report]
```

### Podgląd diff (pokazywany przed Apply)

```
Will change:
EventFlag 60290: false → true
NetworkParam.breakInRequestIntervalTimeSec: 30.0 → 4.0
RawOffset UD10+0x19524 f32: 0.033333 → 0.1
```

---

## Typy targetów

### 1. Znane EventFlags

Nazwane flagi z opisami, wybieralne z dropdown/wyszukiwarki.

| ID flagi | Opis |
|---|---|
| 60100 | Obtained Spectral Steed Whistle |
| 4680 | Melina accord / whistle-given state |
| 4681 | Melina accord popup state |
| 710520 | Whistle world state |
| 60230 | Small Golden Effigy obtained |
| 60240 | Duelist's Furled Finger obtained |
| 60250 | Small Red Effigy obtained |
| 60280 | White Cipher Ring obtained |
| 60290 | Blue Cipher Ring shop/obtained state |
| 60300 | Taunter's Tongue obtained |
| 76111 | Gatefront grace |
| 10009655 | Roundtable invitation handled |
| 11109658 | RTH / Gideon marker |
| 11109659 | Gideon advice marker |

Format wyświetlania: `EventFlag 60290 — Blue Cipher Ring shop/obtained state`

Typ wartości: `bool` (true/false).

### 2. Znane Regulation Snapshot Params

Parametry ze snapshotu regulation.bin, które mają zweryfikowaną obsługę R/W w pliku save.

**Zakres MVP: tylko NetworkParam** (UD11 NETWORK_PARAM_ST, indeks 0).

| Pole | Offset | Typ | Wartość vanilla |
|---|---|---|---|
| maxBreakInTargetListCount | +0x70 | u32 | 5 |
| breakInRequestIntervalTimeSec | +0x74 | f32 | 30.0 |
| breakInRequestTimeOutSec | +0x78 | f32 | 20.0 |
| breakInRequestAreaCount | +0x7C | u32 | 5 |

Format wyświetlania: `NetworkParam[0] +0x74 — breakInRequestIntervalTimeSec`

Późniejsze rozszerzenia (Faza 3+): przeglądarka ShopLineupParam, ItemLotParam, EquipParamGoods, read-only browser parametrów regulation.

### 3. Makra znane aplikacji

Predefiniowane zestawy zmian flag odzwierciedlające to, co aplikacja robi wewnętrznie.

| Makro | Zmiany |
|---|---|
| Mark Spectral Steed Whistle obtained | ustawia 60100, 4680, 4681, 710520 |
| Mark Blue Cipher Ring purchased | ustawia 60290 |
| Mark Gatefront / Melina accord handled | ustawia 60100, 4680, 4681, 710520 |

Wybranie makra rozwija się w indywidualne wpisy w liście oczekujących zmian, każda pokazana jawnie.

### 4. Ręczny EventFlag po ID

Tryb expert dla nieznanych/badawczych flag.

```
EventFlag ID: [pole wejściowe]
Current:      [odczyt z save'a]
New:          [true / false]
```

Zastosowanie: badania, testowanie nowo odkrytych flag, szybkie debugowanie.

### 5. Raw / Unknown Offsets

**Faza 2 — poza MVP.**

Format: `RawOffset UD10+0x19524 — Unknown f32`

Selektor typów: `u8 / u16 / u32 / s32 / f32 / bytes`

Zasady:
- Wysokie ryzyko uszkodzenia save'a.
- Wymagany backup przed użyciem.
- Brak wartości domyślnej bez zweryfikowanego źródła.
- Reset tylko do wartości bieżącej/oryginalnej (bez wartości vanilla).
- Obowiązkowy patch report przy apply.

---

## Zakres MVP

**W MVP:**
- Znane EventFlags — odczyt/zapis
- Ręczny EventFlag po ID
- Znane pola NetworkParam — odczyt/zapis (NETWORK_PARAM_ST)
- Makra znane aplikacji
- Lista oczekujących zmian
- Podgląd diff przed Apply
- Apply changes (atomyczny)
- Export patch report

**Poza MVP:**
- Edytor raw offsets
- Arbitralne patchowanie binarne
- Pełna przeglądarka regulation params
- Zapis ShopLineupParam
- Zapis ItemLotParam

---

## Plan fazowy

### Faza 1 — MVP (opisany powyżej)

Znane EventFlags + ręczna flaga + NetworkParam + makra + lista pending + apply + eksport patcha.

### Faza 2 — Edytor Raw Offsets

- Znane kandydacie offsety UD10/UD11 z selektorem typów
- Odczyt wartości bieżącej, zapis nowej
- Obowiązkowe ostrzeżenie o backupie
- Patch report przy apply

### Faza 3 — Przeglądarka Regulation

- Read-only przeglądanie wyeksportowanych parametrów XML/CSV regulation
- Wyszukiwanie po typie parametru, row ID, nazwie pola, offsetie, opisie
- Źródło: `tmp/regulation-bin-dump/csv/` + `tmp/regulation-bin-dump/defs/`

### Faza 4 — Import/Eksport Patchy

- Eksport wielokrotnego użytku patch JSON (zestaw flag + parametrów + opis)
- Import patch JSON
- Bezpieczne udostępnianie patchy badawczych
- Walidacja patcha względem bieżącego stanu save'a przed apply

---

## Wymagania bezpieczeństwa

- Backup musi istnieć przed wykonaniem Apply (użyj istniejącego backup manager).
- Apply jest atomiczny: wszystko-albo-nic w ramach listy oczekujących.
- Podgląd diff pokazywany przed Apply.
- Wymagany checkbox potwierdzający.
- Patch report eksportowany na żądanie (tekstowe podsumowanie zmian).
- Adnotacja ryzyka bana per typ targetu (zgodna z systemem Tier z spec/32).

---

## Źródła

- `spec/15-event-flags.md` — layout bitfieldu EventFlags
- `spec/18-network.md` — sekcja Network Manager
- `spec/24-user-data-11.md` — snapshot regulation.bin (UD11)
- `spec/44-network-param-tuning.md` — referencja pól NETWORK_PARAM_ST
- `spec/32-ban-risk-system.md` — architektura poziomów ryzyka
- `spec/50-item-companion-flags.md` — mechanika companion flags (referencja dla makr)
