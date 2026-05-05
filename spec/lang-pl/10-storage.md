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

Sekcja jest poprzedzona 4-bajtowym nagłówkiem `storage_count` bezpośrednio przed pierwszym slotem
common item (na `StorageBoxOffset`):

```
┌────────────────────────────────────────┐
│ storage_count (u32)                     │  4 bajty  ← na StorageBoxOffset
├────────────────────────────────────────┤
│ Common Items: 1920 × 12 = 23,040 bytes │  (0x5A00)
├────────────────────────────────────────┤
│ Key Items: 128 × 12 = 1,536 bytes      │  (0x600)
├────────────────────────────────────────┤
│ Next Equip Index (u32)                  │  4 bajty
├────────────────────────────────────────┤
│ Next Acquisition Sort ID (u32)          │  4 bajty
└────────────────────────────────────────┘
Razem (bez nagłówka): 0x5A00 + 0x600 + 8 = 24,584 bajtów (0x6008)
```

---

## Format rekordu (12 bytes) — identyczny jak Inventory

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Quantity |
| 0x08 | u32 | Acquisition Index |

---

## Countery

Storage ma własne, niezależne countery Next Equip Index i Next Acquisition Sort ID.
Obowiązuje ta sama reguła bramki widoczności co w Inventory Held (patrz spec/07):

**Niezmiennik**: `item.AcquisitionIndex < NextEquipIndex` dla każdego niepustego slotu storage.
Itemy naruszające ten niezmiennik są niewidoczne dla gry mimo że istnieją w binarce.

Zewnętrzne edytory mogą pozostawić `NextEquipIndex < NextAcquisitionSortId` również w storage.
Nasz edytor koryguje lukę przy ładowaniu w `mapStorage()` stosując tę samą strategię binarnego
write-back co w inventory.

---

## Implikacje dla edycji

- Przenoszenie przedmiotu Inventory↔Storage: przenieś rekord, zaktualizuj oba zestawy counterów
- Storage shared z GaItem Map — ten sam handle musi istnieć w mapie
- Capacity: 1920 common + 128 key = max pojemność skrzyni
- Aktualizuj nagłówek `storage_count` przy dodawaniu lub usuwaniu przedmiotów

---

## Źródła

- er-save-manager: `parser/user_data_x.py` linia 123: `inventory_storage_box: Inventory` (0x780 common, 0x80 key)
- er-save-manager: `parser/equipment.py` — ta sama klasa `Inventory` z innymi parametrami count
- `backend/core/structures.go`: `StorageBoxOffset`, `mapStorage()`
