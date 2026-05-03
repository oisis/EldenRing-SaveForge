# 06 — Equipment (Ekwipunek)

> **Zakres**: Aktualnie założone przedmioty — bronie, zbroja, talizmany, strzały, bolty, quick items, pouch, Great Rune.

---

## Opis ogólny

Ekwipunek jest opisany przez 4 powiązane struktury, każda po 88 bajtów (22 × u32), plus jedna struktura 28 bajtów dla aktywnych slotów broni.

Każda z 3 głównych struktur (EquipIndex, ItemIDs, GaitemHandles) ma identyczny layout 22 slotów — ale przechowuje inne wartości dla tych samych slotów.

---

## 22 sloty ekwipunku (88 bytes = 22 × u32)

| # | Offset | Slot | Opis |
|---|---|---|---|
| 0 | 0x00 | Left Hand Armament 1 | Broń lewa ręka — primary |
| 1 | 0x04 | Right Hand Armament 1 | Broń prawa ręka — primary |
| 2 | 0x08 | Left Hand Armament 2 | Broń lewa — secondary |
| 3 | 0x0C | Right Hand Armament 2 | Broń prawa — secondary |
| 4 | 0x10 | Left Hand Armament 3 | Broń lewa — tertiary |
| 5 | 0x14 | Right Hand Armament 3 | Broń prawa — tertiary |
| 6 | 0x18 | Arrows 1 | Strzały (primary) |
| 7 | 0x1C | Bolts 1 | Bolty (primary) |
| 8 | 0x20 | Arrows 2 | Strzały (secondary) |
| 9 | 0x24 | Bolts 2 | Bolty (secondary) |
| 10 | 0x28 | Arrows 3 | Strzały (tertiary) — potwierdzone CT |
| 11 | 0x2C | Bolts 3 | Bolty (tertiary) — potwierdzone CT |
| 12 | 0x30 | Head | Hełm |
| 13 | 0x34 | Chest | Zbroja |
| 14 | 0x38 | Arms | Rękawice |
| 15 | 0x3C | Legs | Buty/spodnie |
| 16 | 0x40 | Hair | Fryzura (equipment slot — powiązany z Face Data) |
| 17 | 0x44 | Talisman 1 | Talizman slot 1 |
| 18 | 0x48 | Talisman 2 | Talizman slot 2 |
| 19 | 0x4C | Talisman 3 | Talizman slot 3 (wymaga quest unlock) |
| 20 | 0x50 | Talisman 4 | Talizman slot 4 (wymaga quest unlock) |
| 21 | 0x54 | Accessory 5 | Nieużywany w base game (reserved) |

---

## Struktura 1: EquippedItemsEquipIndex (88 bytes)

Indeksy do tablicy inwentarza. Wartość w każdym slocie to `acquisition_index` z odpowiedniego wpisu w Inventory.

Wartość `0xFFFFFFFF` = slot pusty (nic nie założone).

---

## Struktura 2: ActiveWeaponSlotsAndArmStyle (28 bytes = 7 × u32)

| Offset | Typ | Pole | Opis | Wartości |
|---|---|---|---|---|
| 0x00 | u32 | ArmStyle | Styl trzymania broni | 0=EmptyHand, 1=OneHand, 2=LeftBothHand (2H left), 3=RightBothHand (2H right) |
| 0x04 | u32 | LeftWeaponSlot | Aktywny slot lewej broni | 0=Primary, 1=Secondary, 2=Tertiary |
| 0x08 | u32 | RightWeaponSlot | Aktywny slot prawej broni | 0=Primary, 1=Secondary, 2=Tertiary |
| 0x0C | u32 | LeftArrowSlot | Aktywny slot strzał (lewa) | 0=Primary, 1=Secondary |
| 0x10 | u32 | RightArrowSlot | Aktywny slot strzał (prawa) | 0=Primary, 1=Secondary |
| 0x14 | u32 | LeftBoltSlot | Aktywny slot boltów (lewa) | 0=Primary, 1=Secondary |
| 0x18 | u32 | RightBoltSlot | Aktywny slot boltów (prawa) | 0=Primary, 1=Secondary |

---

## Struktura 3: EquippedItemsItemIds (88 bytes)

Item ID każdego założonego przedmiotu. Odpowiada ID z bazy danych gry.

| Prefix | Typ | Przykład |
|---|---|---|
| 0xxxxxxx – 9xxxxxxx | Weapon | 1000000 = Uchigatana +0 |
| 10xxxxxx – 19xxxxxx | Armor (Protector) | 10100000 = Banished Knight Helm |
| 20xxxxxx – 29xxxxxx | Accessory (Talisman) | 20001000 = Crimson Amber Medallion |
| 40xxxxxx – 49xxxxxx | Goods (Consumable) | 40001001 = Mushroom |
| 50xxxxxx – 59xxxxxx | Gem (Ash of War) | |

Wartość `0xFFFFFFFF` = slot pusty.

---

## Struktura 4: EquippedItemsGaitemHandles (88 bytes)

GaItem Handle dla każdego założonego przedmiotu — wskazuje na konkretną instancję w GaItem Map.

Wartość `0xFFFFFFFF` = slot pusty.

---

## Great Rune (equipped)

Great Rune jest osobnym polem w ekwipunku (nie w 22-slotowej tablicy):

| Typ | Pole | Opis |
|---|---|---|
| u32 | EquippedGreatRune | ID założonego Great Rune |

### Great Rune Item IDs:

| Hex ID | Decimal | Great Rune | Efekt (aktywny z Rune Arc) |
|---|---|---|---|
| 0x00000000 | 0 | None | — |
| 0xB00000BF | 2952790207 | Godrick's Great Rune | +5 do wszystkich atrybutów |
| 0xB00000C0 | 2952790208 | Radahn's Great Rune | +HP, +FP, +SP (max) |
| 0xB00000C1 | 2952790209 | Morgott's Great Rune | +Max HP (znacznie) |
| 0xB00000C2 | 2952790210 | Rykard's Great Rune | HP recovery on kill |
| 0xB00000C3 | 2952790211 | Mohg's Great Rune | Phantom bleed effect na summony |
| 0xB00000C4 | 2952790212 | Malenia's Great Rune | HP recovery on attack (po otrzymaniu obrażeń) |

**Uwaga**: Great Rune wymaga aktywacji Rune Arc (PlayerGameData offset 0xF7 = GreatRuneActive). Posiadanie Great Rune jest kontrolowane przez Event Flags (180–197).

---

## Powiązanie między strukturami

```
Slot "Right Hand 1" (index 1):
  EquipIndex[1]     = 42          (42. element w inventory — acquisition_index)
  ItemIds[1]        = 1000000     (Uchigatana +0)
  GaitemHandles[1]  = 0x80000003  (handle w GaItem Map → wpis broni)
  
ActiveSlots:
  ArmStyle          = 1           (OneHand)
  RightWeaponSlot   = 0           (aktywny primary = slot 1)
```

---

## Quick Items & Pouch (opisane w spec/08)

Quick Items (10 slotów) i Pouch (6 slotów) są opisane w **spec/08-spells-gestures.md** ponieważ sekwencyjnie następują po Equipped Spells. Tutaj krótkie podsumowanie:

- Quick Slots 1–10: 10 × u32 Item ID (cycling through D-pad)
- Pouch 1–6: 6 × u32 Item ID (hold Y/Triangle + direction)
- Wartość `0xFFFFFFFF` = pusty slot
- Items muszą istnieć w inventory — to referencje, nie kopie

---

## Hair Slot (slot #16)

Slot "Hair" w ekwipunku jest powiązany z Face Data (Hair_Model_Id). Zmiana fryzury w kreatorze aktualizuje zarówno Face Data jak i equipment slot Hair. Ten slot jest wewnętrzny — nie jest widoczny w UI ekwipunku gracza.

---

## Implikacje dla edycji

- **Zmiana broni** wymaga aktualizacji WSZYSTKICH 3 struktur (equip index, item id, handle)
- **Handle** musi istnieć w GaItem Map i mieć prawidłowy typ (0x8... dla broni)
- **Active slot values**: 0, 1, lub 2 (odpowiadają slotom 1/2/3)
- **ArmStyle** wpływa na animacje — nieprawidłowa wartość może crashować
- **Great Rune**: zmiana wymaga też odpowiedniego Event Flag (posiadanie) i GreatRuneActive
- **Talisman 3&4**: wymagają quest unlock (Talisman Pouch items) — slot jest fizycznie obecny ale gra go zablokuje
- **Accessory 5**: nie używane — wstawienie wartości może być niestabilne
- **Item ID encoding**: Weapon +X = baseID + X (np. Uchigatana +5 = 1000005)

---

## Źródła

- er-save-manager: `parser/equipment.py` — klasy `EquipmentSlots`, `ActiveWeaponSlotsAndArmStyle` (linie 20-163)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — referenced structures
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — ChrAsm offsets (ArmStyle, weapon slots, all equipment IDs)
- Cheat Engine: `ER_TGA_v1.9.0` — ChrAsm 2 (equipment item IDs 0x5D8-0x62C), Quick Items, Pouch, Great Rune
