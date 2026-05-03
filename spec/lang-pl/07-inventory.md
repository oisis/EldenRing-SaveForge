# 07 — Inventory (Inwentarz)

> **Zakres**: Przedmioty noszone przez postać. Podział na common items i key items.

---

## Opis ogólny

Inwentarz postaci (Inventory Held) to tablica o stałej liczbie slotów, podzielona na dwie sekcje:
- **Common Items**: bronie, zbroje, talizmany, materiały, consumable — 0xA80 (2688) slotów
- **Key Items**: kluczowe przedmioty fabularne — 0x180 (384) slotów

Każdy slot to 12 bajtów. Puste sloty mają `gaitem_handle == 0` lub `gaitem_handle == 0xFFFFFFFF`.

---

## Format rekordu (12 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | GaItem Handle (wskaźnik do GaItem Map) |
| 0x04 | u32 | Quantity (ilość) |
| 0x08 | u32 | Acquisition Index (kolejność pozyskania, używany przez equip system) |

---

## Layout sekcji Inventory Held

```
┌────────────────────────────────────────┐
│ Common Items: 2688 × 12 = 32,256 bytes │  (0x7E00)
├────────────────────────────────────────┤
│ Key Items: 384 × 12 = 4,608 bytes      │  (0x1200)
├────────────────────────────────────────┤
│ Next Equip Index (u32)                  │  4 bytes
├────────────────────────────────────────┤
│ Next Acquisition Sort ID (u32)          │  4 bytes
└────────────────────────────────────────┘
Total: 0x7E00 + 0x1200 + 8 = 36,872 bytes (0x9008)
```

---

## Counters (trailing)

Po tablicach przedmiotów:

| Pole | Typ | Opis |
|---|---|---|
| Next Equip Index | u32 | Następny wolny equip index (inkrementowany przy pickup) |
| Next Acquisition Sort ID | u32 | Następny sort ID (porządek wyświetlania) |

Te countery MUSZĄ być aktualizowane przy dodawaniu nowych przedmiotów.

---

## Powiązanie z GaItem Map

```
Inventory Entry:
  gaitem_handle = 0x80000005
  quantity = 1
  acq_index = 15

    ↓ lookup w GaItem Map

GaItem Map Entry [5]:
  handle = 0x80000005
  item_id = 3010000        → Moonveil +0 (weapon)
  unk2, unk3, aow_handle, unk5   (pola broni)
```

---

## Typy przedmiotów (z item_id)

Item ID ma prefiks określający kategorię:

| Prefiks ID | Kategoria |
|---|---|
| 0xxxxxxx - 9xxxxxxx | Bronie (handle 0x8...) |
| 10xxxxxx - 19xxxxxx | Zbroje (handle 0x9...) |
| 20xxxxxx - 29xxxxxx | Akcesoria/Talizmany (handle 0xA...) |
| 30xxxxxx - 49xxxxxx | Przedmioty/Goods (handle 0xB...) |
| (Ash of War) | (handle 0xC...) |

---

## Implikacje dla edycji

- Dodanie przedmiotu: znajdź wolny slot (handle==0), wpisz handle+qty+index, zaktualizuj counter
- Nowy handle musi mieć odpowiedni wpis w GaItem Map
- `acq_index` musi być unikalny w obrębie inventory — użyj Next Equip Index i zinkrementuj
- Max quantity zależy od typu przedmiotu (bronie=1, materiały=999, itd.)
- Key Items to osobna sekcja — nie mieszaj z common

---

## Źródła

- er-save-manager: `parser/equipment.py` — klasa `InventoryItem` (linia 191+), `Inventory`
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — inventory structures
- er-save-manager: `parser/user_data_x.py` linia 104-105: `inventory_held: Inventory` (0xa80 common, 0x180 key)
