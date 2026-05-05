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

Sekcja jest poprzedzona 4-bajtowym nagłówkiem count (`common_item_count`) bezpośrednio przed
pierwszym slotem common item (`MagicOffset + InvStartFromMagic - 4`):

```
┌────────────────────────────────────────┐
│ common_item_count (u32)                 │  4 bajty  ← na MagicOffset + 0x1F9
├────────────────────────────────────────┤
│ Common Items: 2688 × 12 = 32,256 bytes │  (0x7E00)
├────────────────────────────────────────┤
│ Key Items: 384 × 12 = 4,608 bytes      │  (0x1200)
├────────────────────────────────────────┤
│ Next Equip Index (u32)                  │  4 bajty
├────────────────────────────────────────┤
│ Next Acquisition Sort ID (u32)          │  4 bajty
└────────────────────────────────────────┘
Razem (bez nagłówka): 0x7E00 + 0x1200 + 8 = 36,872 bajtów (0x9008)
```

---

## Countery (trailing)

Po tablicach przedmiotów:

| Pole | Typ | Opis |
|---|---|---|
| Next Equip Index | u32 | Bramka widoczności — itemy z `Acquisition Index >= NextEquipIndex` są niewidoczne dla gry |
| Next Acquisition Sort ID | u32 | Następny sort ID przypisywany kolejnemu dodanemu przedmiotowi |

### Bramka widoczności (krytyczne)

Gra traktuje `item.AcquisitionIndex >= NextEquipIndex` jako **niewidoczny** — przedmiot istnieje
w binarce, ale equip system go ignoruje. To mechanizm odpowiedzialny za buga "item dodany, ale
niewidoczny w grze".

**Niezmiennik**: po każdym dodaniu, `NextEquipIndex` MUSI być ściśle większy niż `AcquisitionIndex`
każdego itema. Poprawna aktualizacja:

```
item.AcquisitionIndex = NextAcquisitionSortId   // przypisz przed inkrementacją
NextAcquisitionSortId++
if item.AcquisitionIndex >= NextEquipIndex {
    NextEquipIndex = item.AcquisitionIndex + 1
} else {
    NextEquipIndex++
}
```

Zewnętrzne edytory (np. `er-save-manager`) czasem inkrementują `NextAcquisitionSortId` bez
przesuwania `NextEquipIndex`, tworząc lukę. Itemy dodane przez takie edytory z wysokim
`AcquisitionIndex` pozostają na stałe niewidoczne dopóki luka nie zostanie zamknięta.

**Przy ładowaniu**: nasz edytor wykrywa `NextEquipIndex < NextAcquisitionSortId` w `mapInventory()`
i koryguje zarówno wartość w pamięci, jak i bajty binarne pod `nextEquipIndexOff`. Samo otwarcie
i ponowny zapis pliku save naprawia bramkę widoczności dla gry.

### Nagłówek `common_item_count`

4-bajtowy count pod `invStart - 4` jest odczytywany przez runtime gry jako "następny wolny index
wstawiania" dla in-game pickupów i próg dla "inventory full" (`count == 2688`). Musi być równy
rzeczywistej liczbie niepustych slotów common item.

- Przestarzały count (za niski po dodaniach przez zewnętrzne narzędzia): gra wstawia pod złą
  pozycją, potencjalnie nadpisując prawidłowe itemy.
- Przestarzały count (za wysoki po usunięciach): gra przedwcześnie zgłasza "inventory full".

**Przy ładowaniu**: `ReconcileInventoryHeader()` przelicza i zapisuje poprawną wartość do binarki.

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

## Osierocone rekordy GaItem (Orphaned GaItems)

Rekord GaItem, którego handle nie pojawia się w żadnym slocie inventory ani storage, jest
**osierocony**. Takie rekordy zajmują pojemność GaItem (łącznie 5118–5120 wpisów) bez bycia
dostępnymi dla gracza. Akumulują się gdy itemy są usuwane z inventory/storage bez czyszczenia
binarnego wpisu GaItem. Użyj `RepairOrphanedGaItems(slot)` aby je przeskanować i wyczyścić.

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

- Dodanie przedmiotu: znajdź wolny slot (handle==0), wpisz handle+qty+index, zaktualizuj wszystkie
  trzy countery (`NextEquipIndex`, `NextAcquisitionSortId`, `common_item_count`)
- Usunięcie przedmiotu: wyzeruj slot, dekrementuj `common_item_count`, wyczyść binarny wpis GaItem
- Nowy handle musi mieć odpowiedni wpis w GaItem Map
- `AcquisitionIndex` musi spełniać `< NextEquipIndex` bezpośrednio po dodaniu
- Max quantity zależy od typu przedmiotu (bronie=1, materiały=999, itd.)
- Key Items to osobna sekcja — nie mieszaj z common

---

## Źródła

- er-save-manager: `parser/equipment.py` — klasa `InventoryItem` (linia 191+), `Inventory`
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — inventory structures
- er-save-manager: `parser/user_data_x.py` linia 104-105: `inventory_held: Inventory` (0xa80 common, 0x180 key)
- `backend/core/offset_defs.go`: `InvStartFromMagic = 505`
- `backend/core/structures.go`: `mapInventory()`, `ReconcileInventoryHeader()`
- `backend/core/writer.go`: `addToInventory()`, `RepairOrphanedGaItems()`
