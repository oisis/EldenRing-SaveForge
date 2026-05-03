# 10 — Storage Box (Skrzynia)

> **Zakres**: Skrzynia dostępna w Site of Grace — przechowywanie przedmiotów poza inwentarzem.

---

## Opis ogólny

Storage Box (Inventory Storage) ma identyczny format co Inventory Held, ale z innymi rozmiarami:
- **Common Items**: 0x780 (1920) slotów
- **Key Items**: 0x80 (128) slotów

Każdy slot to 12 bajtów (identyczny format jak w inwentarzu).

---

## Layout

```
┌────────────────────────────────────────┐
│ Common Items: 1920 × 12 = 23,040 bytes │  (0x5A00)
├────────────────────────────────────────┤
│ Key Items: 128 × 12 = 1,536 bytes      │  (0x600)
├────────────────────────────────────────┤
│ Next Equip Index (u32)                  │  4 bytes
├────────────────────────────────────────┤
│ Next Acquisition Sort ID (u32)          │  4 bytes
└────────────────────────────────────────┘
Total: 0x5A00 + 0x600 + 8 = 24,584 bytes (0x6008)
```

---

## Format rekordu (12 bytes) — identyczny jak Inventory

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Quantity |
| 0x08 | u32 | Acquisition Index |

---

## Counters

Storage ma własne, niezależne countery Next Equip Index i Next Acquisition Sort ID.

---

## Implikacje dla edycji

- Przenoszenie przedmiotu Inventory↔Storage: przenieś rekord, zaktualizuj oba zestawy counterów
- Storage shared z GaItem Map — ten sam handle musi istnieć w mapie
- Capacity: 1920 common + 128 key = max pojemność skrzyni

---

## Źródła

- er-save-manager: `parser/user_data_x.py` linia 123: `inventory_storage_box: Inventory` (0x780 common, 0x80 key)
- er-save-manager: `parser/equipment.py` — ta sama klasa `Inventory` z innymi parametrami count
