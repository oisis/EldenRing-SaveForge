# 03 — GaItem Map (Mapa Przedmiotów)

> **Zakres**: Tablica mapująca wewnętrzne "handle" na ID przedmiotów. Pierwsza duża sekcja po slot header.

---

## Opis ogólny

GaItem Map to tablica o stałej liczbie wpisów (5118 lub 5120 w zależności od wersji slotu), gdzie każdy wpis opisuje jeden "slot" na przedmiot w grze. Handle to unikalny identyfikator instancji przedmiotu w save.

Mapa jest krytyczna — inwentarz, ekwipunek i storage odwołują się do przedmiotów przez handle, a nie bezpośrednio przez item ID.

---

## Struktura

### Liczba wpisów
- `version <= 81`: 5118 wpisów (0x13FE)
- `version > 81`: 5120 wpisów (0x1400)

### Typy handle'i (upper nibble u32)

| Mask (upper 4 bits) | Typ | Rozmiar rekordu |
|---|---|---|
| `0x80000000` | Broń (Weapon) | 21 bytes |
| `0x90000000` | Zbroja (Armor) | 16 bytes |
| `0xA0000000` | Akcesorium (Accessory/Talisman) | 8 bytes |
| `0xB0000000` | Przedmiot (Item/Good) | 8 bytes |
| `0xC0000000` | Ash of War | 8 bytes |
| `0xFFFFFFFF` | Nieprawidłowy (invalid) | — |
| `0x00000000` | Pusty (empty) | — |

### Format rekordu — Broń (21 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Item ID |
| 0x08 | u32 | Unknown 2 |
| 0x0C | u32 | Unknown 3 |
| 0x10 | u32 | Ash of War GaItem Handle |
| 0x14 | u8 | Unknown 5 |

### Format rekordu — Zbroja (16 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Item ID |
| 0x08 | u32 | Unknown 2 |
| 0x0C | u32 | Unknown 3 |

### Format rekordu — Item/Accessory/AoW (8 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Item ID |

---

## Powiązanie z inwentarzem

```
Inventory Item (12 bytes):
  ├── gaitem_handle → wskazuje na wpis w GaItem Map
  ├── quantity      → ilość
  └── acq_index     → kolejność pozyskania

GaItem Map Entry:
  ├── handle        → ten sam co w inventory
  └── item_id       → rzeczywisty ID przedmiotu w bazie gry
```

---

## Implikacje dla edycji

- Dodanie nowego przedmiotu wymaga: znalezienia wolnego slotu w GaItem Map + dodania wpisu w Inventory
- Zmiana broni wymaga 21-bajtowego rekordu
- Typ handle (upper nibble) MUSI odpowiadać typowi przedmiotu
- Nieużywane sloty mają handle `0x00000000` lub `0xFFFFFFFF`

---

## Źródła

- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — GaItem2 struct (linie 156-194)
- er-save-manager: `parser/er_types.py` — Gaitem class
- er-save-manager: `parser/user_data_x.py` — `gaitem_map` field (linia 82)
