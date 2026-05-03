# 20 — Version & Platform Data

> **Zakres**: Wersja bazy gry, Steam ID, PS5 Activity.

---

## Opis ogólny

Trzy struktury identyfikujące wersję gry i platformę.

---

## BaseVersion (16 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | Base Version Copy (kopia wersji) |
| 0x04 | u32 | Base Version (wersja bazy gry) |
| 0x08 | u32 | Is Latest Version (flaga) |
| 0x0C | u32 | Unknown (unk0xc) |

---

## Steam ID (8 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u64 | Steam ID gracza |

- Tylko PC — na PS4 ta wartość jest 0 lub ignorowana
- Przy konwersji PS4→PC: trzeba wpisać prawidłowy Steam ID
- Przy konwersji PC→PS4: ignorowana

---

## PS5 Activity (32 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u8[32] | Opaque data — flagi aktywności PS5 |

Obecne także w save PC (zerowane). Struktura wewnętrzna nieznana.

---

## Implikacje dla edycji

- **Steam ID**: krytyczne przy konwersji platform. Zły Steam ID = save odrzucony przez Steam
- **Base Version**: zmiana może triggerować migrację save lub odrzucenie
- **PS5 Activity**: bezpieczne do zerowania

---

## Źródła

- er-save-manager: `parser/world.py` — `BaseVersion` (linie 890-914), `PS5Activity` (922-935)
- er-save-manager: `parser/user_data_x.py` linie 191-193
