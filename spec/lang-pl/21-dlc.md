# 21 — DLC (Downloadable Content)

> **Zakres**: Flagi DLC — pre-order gesty i Shadow of the Erdtree entry.

---

## Opis ogólny

Sekcja DLC to 50 bajtów (0x32) zawierających flagi ownership i entry dla DLC. Struktura: CSDlc — tablica 1-bajtowych booli.

---

## Struktura (50 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u8 | Pre-order Gesture "The Ring" (0=nie, 1=tak) |
| 0x01 | u8 | Shadow of the Erdtree — entry flag (0=nie wchodzono, 1=wchodzono) |
| 0x02 | u8 | Pre-order Gesture "Ring of Miquella" (0=nie, 1=tak) |
| 0x03 | u8[47] | Unused (MUSZĄ być 0x00) |

---

## Shadow of the Erdtree Entry Flag

- `0`: Postać nie weszła do DLC
- `1`: Postać weszła do Shadow of the Erdtree

Ta flaga jest jednorazowa — po wejściu nie da się cofnąć w grze. Edycja pozwala na reset.

---

## Walidacja — nieużywane bajty

**WAŻNE**: Bajty 3-49 (47 bajtów) MUSZĄ być zerowe. Niezerowe wartości w tej sekcji **uniemożliwiają załadowanie save** — gra odrzuca plik.

---

## Implikacje dla edycji

- **Clear DLC flag**: ustawienie byte[1]=0 pozwala "cofnąć" wejście do DLC
- **Pre-order gestures**: ustawienie byte[0]=1 lub byte[2]=1 odblokowuje gesty
- **KRYTYCZNE**: nigdy nie ustawiaj niezerowych wartości w bytes 3-49
- Bezpieczne do edycji — stała pozycja w slocie (SlotSize - 0xB2 od końca)

---

## Źródła

- er-save-manager: `parser/world.py` — klasa `DLC` (linie 938-987)
- er-save-manager: `parser/user_data_x.py` linia 194: `dlc: DLC`
