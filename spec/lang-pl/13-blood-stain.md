# 13 — Blood Stain (Plama Krwi)

> **Zakres**: Dane plamy krwi pozostawionej po śmierci gracza — lokalizacja i utracone runy.

---

## Opis ogólny

Blood Stain przechowuje informację o ostatniej śmierci gracza: gdzie umarł i ile run utracił. Gracz może odzyskać runy podnosząc plamę krwi. Rozmiar: 68 bajtów (0x44).

---

## Struktura (68 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | f32 × 3 | Coordinates (x, y, z) — pozycja plamy |
| 0x0C | f32 × 4 | Angle / Quaternion (orientacja) |
| 0x1C | u32 | Unknown (unk0x1c) |
| 0x20 | u32 | Unknown (unk0x20) |
| 0x24 | u32 | Unknown (unk0x24) |
| 0x28 | u32 | Unknown (unk0x28) |
| 0x2C | u32 | Unknown (unk0x2c) |
| 0x30 | i32 | Unknown (unk0x30) |
| 0x34 | i32 | Runes (ilość run do odzyskania) |
| 0x38 | u8[4] | Map ID (mapa na której jest plama) |
| 0x3C | u32 | Unknown (unk0x3c) |
| 0x40 | u32 | Unknown (unk0x38) |

---

## Implikacje dla edycji

- Modyfikacja `Runes` pozwala zmienić ile run gracz odzyska
- Ustawienie coordinates pozwala "przenieść" plamę w dostępne miejsce
- Wyzerowanie całej struktury = brak plamy krwi (gracz nie ma nic do odzyskania)
- Przydatne przy corrupted saves gdzie plama jest w niedostępnym miejscu

---

## Źródła

- er-save-manager: `parser/world.py` — klasa `BloodStain` (linie 182-229)
- er-save-manager: `parser/user_data_x.py` linia 136: `blood_stain: BloodStain`
