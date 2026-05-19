# 06 — Equipment

> **Typ**: Design doc + canonical reference
> **Status**: ✅ Active — canonical reference dla aktualnego **read-only** modelu equipment w SaveForge.
> **Zakres**: Co edytor parsuje z sekcji equipment w save (`EquippedItemsItemIds` 88B + `EquippedGreatRune`), jak ta sekcja jest używana (`IsHandleEquipped` transfer guard, hash 7/8 dependency) oraz czego edytor **nie** implementuje (write API dla equipment slotów).

---

## Cel rozdziału

Rozdział dokumentuje, jak SaveForge traktuje sekcję ekwipunku w pliku save: które dane są parsowane, do czego są używane, oraz wyraźnie oddziela ten model od hipotetycznych struktur z `er-save-manager` / Cheat Engine, które obecny kod **nie** parsuje.

Kluczowe pytania, na które rozdział odpowiada:

- Co dokładnie SaveForge czyta z sekcji equipment w save?
- W jakiej formie ChrAsmEquipment przechowuje item references (encoded item-ID form, nie raw itemID ani handle)?
- Jak `IsHandleEquipped` chroni przed transferem założonych itemów?
- Czego SaveForge **nie** implementuje (brak public API dla zmiany equipment)?
- Jakie struktury z poprzedniej wersji rozdziału są hipotezami zewnętrznymi (er-save-manager) i pozostają niepotwierdzone w naszym kodzie?

Powiązane mechanizmy — w cross-references na końcu rozdziału: [03](03-gaitem-map.md) (GaItem/GaMap), [07](07-inventory.md) (Inventory), [53](53-inventory-storage-transfer.md) (transfer + equipped guard), [54](54-ash-of-war.md) (AoW patching).

---

## Status

| Komponent | Status |
|---|---|
| Sekcja `EquippedItemsItemIds` (88B = 22 × u32) | ✅ Parsowana w `parseFromData` przez `EquipItemsIDOffset` |
| Encoded item-ID form (3 konwencje) | ✅ Active w `IsHandleEquipped` |
| `IsHandleEquipped` (transfer guard) | ✅ Active, pokrycie testowe `TestMoveEquippedInvToStorageSkipped` |
| Hash 7 (weapons) i hash 8 (armor+talismans) — input z ChrAsmEquipment | ✅ Computed w `ComputeSlotHash` |
| `EquippedGreatRune` (1 × u32 wewnątrz sekcji, offset 0x28) | ✅ Read+Write w `parseFromData` / `WriteSave` |
| App-level write API dla equipment slotów | ❌ Brak — equipment jest read-only z perspektywy UI |
| Hash recompute (`RecalculateSlotHash`) w runtime | ⚠️ Wywoływany **tylko** w testach `hash_test.go`; main code path nie aktualizuje hashy |
| Sloty 10/11/16 oznaczone w kodzie jako `unk0x28/0x2C/0x40` | `needs verification` — nazwy `Arrows 3 / Bolts 3 / Hair` z poprzedniego dokumentu pochodziły z `er-save-manager` / Cheat Engine, nie zweryfikowane przez nasz parser |
| Hipotetyczne struktury `EquippedItemsEquipIndex`, `ActiveWeaponSlotsAndArmStyle`, `EquippedItemsGaitemHandles` | ❌ Nie parsowane w naszym kodzie — hipotezy z `er-save-manager` |

---

## Source of truth w kodzie

| Topic | Plik / funkcja |
|---|---|
| `EquipItemsIDOffset` (absolutny offset sekcji) | `backend/core/structures.go:244` + offset chain w `parseFromData` (`structures.go:351-354`) |
| `ChrAsmFieldCount` (22) i `ChrAsmEquipmentSize` (88) | `backend/core/hash.go:24-27` |
| Layout 22 slotów (komentarz docstring) | `backend/core/hash.go:29-42` |
| `weaponSlotIndices` (hash 7 input) | `backend/core/hash.go:45` — `[0..9]` |
| `armorSlotIndices` (hash 8 input) | `backend/core/hash.go:49` — `[12, 13, 14, 15, 17, 18, 19, 20, 21]` |
| `readEquipSection` | `backend/core/hash.go:124` — czyta 22 × u32 |
| `extractSlots` | `backend/core/hash.go:137` — picks values from indices |
| `equipmentHash` | `backend/core/hash.go:107` — `bytesHash` over u32 slice |
| `ComputeSlotHash` (hash 7/8 wpis) | `backend/core/hash.go:197-258` |
| `RecalculateSlotHash` | `backend/core/hash.go:290` — wywoływany **wyłącznie** w testach (`hash_test.go`) |
| `IsHandleEquipped` (encoded item-ID matching) | `backend/core/transfer.go:592-633` |
| `EquippedGreatRune` field | `backend/core/structures.go:216` (typ), `:321-324` (read), `:855-857` (write w `WriteSave`) |
| `DynEquipGreatRune = 0x28` (offset wewnątrz sekcji) | `backend/core/offset_defs.go:100` |
| `DynEquip*` offsets dla offset chain | `backend/core/offset_defs.go:86-100` |
| Transfer equipped guard użycie | `backend/core/transfer.go:194-196` (`SkipReasonEquipped` dla `TransferToStorage`) |
| Test pokrywający equipped guard | `tests/transfer_test.go::TestMoveEquippedInvToStorageSkipped` |

---

## Mental model

```
                       ┌──────────────────────────────────────────┐
                       │  Save (binary)                           │
                       │  EquipItemsIDOffset ──▶  ChrAsmEquipment │
                       │                          88 bytes        │
                       │                          22 × u32        │
                       │                          (encoded form)  │
                       └──────────────────────┬───────────────────┘
                                              │
                                  parseFromData reads
                                              │
                                              ▼
                       ┌──────────────────────────────────────────┐
                       │  slot.EquipItemsIDOffset (int)           │
                       │  slot.Player.EquippedGreatRune (u32)     │
                       │  (Great Rune żyje wewnątrz sekcji        │
                       │   na offsecie +0x28 = slot 10)           │
                       └────────────┬──────────────┬──────────────┘
                                    │              │
                  IsHandleEquipped  │              │  readEquipSection
                  (transfer guard)  │              │  (hash 7 / 8 input)
                                    ▼              ▼
                          ┌─────────────────────────────────────┐
                          │  Transfer (legacy core) ─▶ skip     │
                          │    handle gdy IsHandleEquipped      │
                          │    (zob. spec/53)                   │
                          │                                     │
                          │  ComputeSlotHash:                   │
                          │    hash[7] = equipmentHash(slots    │
                          │              0..9)                  │
                          │    hash[8] = equipmentHash(slots    │
                          │              12-15, 17-21)          │
                          └─────────────────────────────────────┘
                                              ▲
                                              │
                                  (RecalculateSlotHash
                                   wywoływany **tylko**
                                   w testach — main code
                                   nie aktualizuje hashy)
```

Edytor **nie ma** kodu zapisującego ChrAsmEquipment slot 0..9, 12-15, 17-21. Jedyny write w obrębie tej sekcji to `EquippedGreatRune` w slocie 10 (offset 0x28), realizowany w `WriteSave` jako część `Player` state.

---

## What SaveForge currently parses

W tym rozdziale "parsuje" oznacza: kod ekstrahuje wartość z `slot.Data` do typowanego pola w `SaveSlot` / `Player` i opcjonalnie wykorzystuje ją w business logic.

### Sekcja `EquippedItemsItemIds`

- Absolutny offset w `slot.Data`: `slot.EquipItemsIDOffset` (`structures.go:244`).
- Wyliczany w `parseFromData` (`structures.go:351-354`) jako:
  ```go
  spEffect           := mo + DynSpEffect
  equipedItemIndex   := spEffect + DynEquipedItemIndex
  activeEquipedItems := equipedItemIndex + DynActiveEquipedItems
  equipedItemsID     := activeEquipedItems + DynEquipedItemsID
  s.EquipItemsIDOffset = equipedItemsID
  ```
  gdzie `DynEquipedItemsID = 0x58` (`offset_defs.go:88`).
- Sekcja ma rozmiar `ChrAsmEquipmentSize = 88` bajtów (`hash.go:27`), zawiera `ChrAsmFieldCount = 22` u32 wartości (`hash.go:24`).
- **Nie ma** typowanego struct `ChrAsmEquipment` w naszym kodzie — sekcja jest czytana ad-hoc przez `readEquipSection(data, off)` (`hash.go:124-134`), które zwraca `[]uint32` długości 22.

### `EquippedGreatRune` (1 × u32 wewnątrz sekcji)

- Pole: `slot.Player.EquippedGreatRune uint32` (`structures.go:216`).
- Offset: `slot.EquipItemsIDOffset + DynEquipGreatRune`, gdzie `DynEquipGreatRune = 0x28` (`offset_defs.go:100`).
- Offset 0x28 = bajt 40 = **slot 10** w 22-slot tabeli ChrAsmEquipment. Czyli `EquippedGreatRune` *fizycznie żyje wewnątrz* sekcji 88B, tylko z osobnym typowanym dostępem.
- Read: `parseFromData` (`structures.go:321-324`).
- Write: `WriteSave` → `SyncPlayerToData` (`structures.go:855-857`).
- Wartość = u32 item ID (0 = none).

### Co NIE jest typowane

Pozostałe 21 wartości z 22-slot tabeli (sloty 0-9, 11-21) **nie mają** dedykowanych pól w `Player` ani w `SaveSlot`. Są dostępne ad-hoc przez `readEquipSection(slot.Data, slot.EquipItemsIDOffset)`.

---

## EquippedItemsItemIds section

Sekcja jest jedyną parsowaną sekcją equipment w SaveForge. Layout 22 u32 (`hash.go:29-42`):

| # | Offset | Nazwa wg kodu | Komentarz |
|---|---|---|---|
| 0 | 0x00 | `LeftHandArmament1` | hash 7 (weapon) |
| 1 | 0x04 | `RightHandArmament1` | hash 7 |
| 2 | 0x08 | `LeftHandArmament2` | hash 7 |
| 3 | 0x0C | `RightHandArmament2` | hash 7 |
| 4 | 0x10 | `LeftHandArmament3` | hash 7 |
| 5 | 0x14 | `RightHandArmament3` | hash 7 |
| 6 | 0x18 | `Arrows1` | hash 7 |
| 7 | 0x1C | `Bolts1` | hash 7 |
| 8 | 0x20 | `Arrows2` | hash 7 |
| 9 | 0x24 | `Bolts2` | hash 7 |
| 10 | 0x28 | `unk0x28` (`EquippedGreatRune` na tym offsecie) | poza hash 7 i 8 |
| 11 | 0x2C | `unk0x2C` | poza hash 7 i 8 |
| 12 | 0x30 | `Head` | hash 8 |
| 13 | 0x34 | `Chest` | hash 8 |
| 14 | 0x38 | `Arms` | hash 8 |
| 15 | 0x3C | `Legs` | hash 8 |
| 16 | 0x40 | `unk0x40` | poza hash 7 i 8 |
| 17 | 0x44 | `Talisman1` | hash 8 |
| 18 | 0x48 | `Talisman2` | hash 8 |
| 19 | 0x4C | `Talisman3` | hash 8 |
| 20 | 0x50 | `Talisman4` | hash 8 |
| 21 | 0x54 | `Talisman5` | hash 8 |

Sloty 10/11/16 są w komentarzu jako `unk` — kod nie nadaje im semantycznych nazw. Poprzednia wersja dokumentu opisywała je jako `Arrows 3 / Bolts 3 / Hair`; ta atrybucja pochodzi z `er-save-manager` / Cheat Engine i pozostaje `needs verification` w naszym parserze.

Slot 21 (`Talisman5`) — poprzednia wersja dokumentu opisywała go jako `Accessory 5 (reserved, nieużywany)`. Kod traktuje go jako Talisman5 (hash 8 go uwzględnia jako jedno z 9 wejść armor+talisman hash).

Wartość specjalna `0xFFFFFFFF` w slocie = slot pusty.

---

## Encoded item-ID forms

Wartości w 22-slot tabeli **nie są** bezpośrednio item IDs ani handles. Z docstring `IsHandleEquipped` (`transfer.go:600-613`):

> ChrAsmEquipment uses an encoded item-ID form: for weapons/armor/AoW the value is `itemID | 0x80000000` (the item ID resolved via GaMap with a 0x80 high-bit flag). For talismans the value is the bare lower 28 bits of the handle. Match against every plausible representation to keep the check robust across upgrade levels and infusions.

Trzy konwencje encoded form:

| Typ itemu | Handle prefix | Encoded form w slot equipment |
|---|---|---|
| Weapon / Armor / AoW | `0x80` / `0x90` / `0xC0` | `itemID \| 0x80000000` (item ID z `GaMap` + 0x80 flag) |
| Talisman (Accessory) | `0xA0` | `handle & 0x0FFFFFFF` (bare lower 28 bits = itemID) |
| Goods | `0xB0` (zwykle nie w equipment) | prefix-swap `0xB0 → 0x40`: `lower \| 0x40000000` |

### `IsHandleEquipped` candidate matching

`IsHandleEquipped(slot, handle)` (`transfer.go:592`) buduje **set kandydatów** dla danego `handle`, a następnie skanuje wszystkie 22 sloty sekcji szukając matchu. Set zawiera (z `transfer.go:605-621`):

1. `handle` wprost (defensive — niektóre save'y mogą trzymać handle bezpośrednio).
2. `handle & 0x0FFFFFFF` — bare lower 28 bits (forma talizmana).
3. `(handle & 0x0FFFFFFF) | 0x80000000` — bare lower + 0x80 flag.
4. `GaMap[handle]` — true item ID (zwracane przez `GaMap` lookup) — dla weapon/armor/AoW.
5. `GaMap[handle] | 0x80000000` — `GaMap` value + 0x80 flag.
6. Prefix-swap zależnie od typu:
   - `0xA0` (talisman) → dodaje `lower | 0x20000000` (talisman item ID prefix).
   - `0xB0` (goods) → dodaje `lower | 0x40000000` (goods item ID prefix).

Match z dowolnym ze slotów (poza `0` i `0xFFFFFFFF`) → `true`. Multi-form matching jest defensive: różne wersje gry mogą zapisywać equipment w różnych kanonach, więc `IsHandleEquipped` przyjmuje wszystkie znane reprezentacje.

---

## Slot names and unknown slots

Sloty 10/11/16 są w komentarzu kodu jako `unk0x28 / unk0x2C / unk0x40`. Nasz parser nie nadaje im semantyki ani nie używa ich w żadnej funkcji business logic.

**Hipotezy z poprzednich wersji dokumentu** (zachowane jako `needs verification`):

- Slot 10 / `unk0x28` — w poprzednim 06 jako "Arrows 3" z notą "potwierdzone CT". W naszym kodzie ten offset zawiera `EquippedGreatRune`, więc Arrows 3 jest niespójne z aktualnym użyciem. **`needs verification`** — czy gra używa tego slotu jako trzeciego setu strzał albo wyłącznie jako Great Rune.
- Slot 11 / `unk0x2C` — w poprzednim 06 jako "Bolts 3". Nasz kod nie odczytuje. **`needs verification`**.
- Slot 16 / `unk0x40` — w poprzednim 06 jako "Hair" (powiązane z Face Data Hair_Model_Id). Nasz kod nie odczytuje. **`needs verification`** — w szczególności hipotetyczne powiązanie z Face Data nie jest potwierdzone w żadnym z parserów save'a w SaveForge.

Sloty `Talisman3` (19), `Talisman4` (20), `Talisman5` (21) — kod traktuje wszystkie jako pełnoprawne talisman slots i uwzględnia w `armorSlotIndices` (hash 8). Hipotetyczna nota "Talisman 3&4 wymagają quest unlock; Accessory 5 nieużywany" z poprzedniego dokumentu — `needs verification` co do egzekwowania przez grę.

---

## EquippedGreatRune

`Player.EquippedGreatRune` (`structures.go:216`) jest:
- Czytany w `parseFromData` (`structures.go:321-324`) z offsetu `EquipItemsIDOffset + 0x28` = slot 10 w sekcji ChrAsmEquipment.
- Zapisywany w `WriteSave` → `SyncPlayerToData` (`structures.go:855-857`) na ten sam offset.

Edytor **nie** udostępnia publicznego API dla zmiany `Player.EquippedGreatRune` z UI (brak `App.SetGreatRune` / podobnych funkcji eksportowanych do JS). Pole jest *zachowywane* podczas round-trip save/load, ale nie jest *edytowalne* z poziomu interfejsu.

**Według obecnych funkcji hash 7/8**: slot 10 (offset 0x28), na którym mieszka Great Rune, nie znajduje się w `weaponSlotIndices` (`[0..9]`) ani w `armorSlotIndices` (`[12-15, 17-21]`). Czyli **inputy** do `equipmentHash` dla hash 7 i hash 8 nie zawierają wartości z tego slotu. Implikuje to, że **w naszej implementacji hash** zmiana wartości na offsecie `EquipItemsIDOffset+0x28` nie zmieniłaby wyniku `ComputeSlotHash` dla wpisów 7 i 8. **`needs verification`** czy gra używa hashy w identyczny sposób (mogą istnieć inne hashe slot-level poza tymi 12, których nasz kod nie liczy).

### Great Rune item IDs (reference)

| Hex ID | Decimal | Great Rune |
|---|---|---|
| `0x00000000` | 0 | None |
| `0xB00000BF` | 2952790207 | Godrick's |
| `0xB00000C0` | 2952790208 | Radahn's |
| `0xB00000C1` | 2952790209 | Morgott's |
| `0xB00000C2` | 2952790210 | Rykard's |
| `0xB00000C3` | 2952790211 | Mohg's |
| `0xB00000C4` | 2952790212 | Malenia's |

Źródło: Cheat Engine community tables + Fextralife. **`needs verification`** co do DLC Great Runes (np. Bayle's, Promised Consort Radahn's) — nasz kod traktuje pole jako opaque u32 i nie ma listy konstant.

Powiązanie z buffem: `Player.GreatRuneOn` (`structures.go:215`, `OffGreatRuneOn = -184` = PGD 0xF7) — 1 = active, 0 = off. To osobne pole poza sekcją equipment.

---

## Read path and hash dependency

### Hash 7 (Equipped Weapons)

Z `ComputeSlotHash` (`hash.go:248-254`):

```go
equipSection := readEquipSection(slot.Data, equipItemsIDOff)
weaponIDs := extractSlots(equipSection, weaponSlotIndices)  // sloty [0..9]
writeEntry(7, equipmentHash(weaponIDs))
```

Input: 10 u32 wartości z slotów 0-9 (L1, R1, L2, R2, L3, R3, Arrows1, Bolts1, Arrows2, Bolts2).
Hash: `bytesHash(weaponIDs)` (`hash.go:75-85`) — modified Adler-like checksum.

### Hash 8 (Equipped Armors + Talismans)

Z `ComputeSlotHash` (`hash.go:256-258`):

```go
armorIDs := extractSlots(equipSection, armorSlotIndices)  // sloty [12,13,14,15,17,18,19,20,21]
writeEntry(8, equipmentHash(armorIDs))
```

Input: 9 u32 wartości z slotów 12-15 (Head, Chest, Arms, Legs) i 17-21 (Talisman1..Talisman5).

### Hash recompute discipline

`RecalculateSlotHash` (`hash.go:290`) zapisuje wynik `ComputeSlotHash` do `slot.Data[HashOffset:HashOffset+HashSize]`. **W naszym main code path funkcja ta nie jest wywoływana** — wszystkie odnośniki w repo są w `backend/core/hash_test.go`. To znaczy:

- Edytor nie aktualizuje hashy przy `WriteSave`.
- Gra prawdopodobnie waliduje hashy przy load — `needs verification` jak zachowuje się gra gdy zapisujemy save w niezmienionym stanie equipment (hash powinien pozostać ważny z poprzedniego save / runtime).
- **`needs verification`** całościowe: czy edytor powinien wywołać `RecalculateSlotHash` po edycjach które wpływają na hash (np. zmiana stats, level, souls — hashe 0, 1, 5).

---

## Current runtime usage

Sekcja `EquippedItemsItemIds` jest używana w SaveForge w trzech miejscach:

1. **`IsHandleEquipped`** (`transfer.go:592`) — guard używany przez legacy core path transferu (`core.MoveItemsBetweenContainers`, [53](53-inventory-storage-transfer.md)). Skanuje wszystkie 22 sloty z multi-form candidate matching.
2. **`ComputeSlotHash`** wpisy 7 i 8 — `readEquipSection` + `extractSlots` + `equipmentHash`.
3. **`EquippedGreatRune`** read/write w `parseFromData` / `WriteSave` — wewnątrz sekcji na offset 0x28.

Brak innych runtime usages w naszym kodzie. UI komponenty (frontend) nie wołają żadnych App-level bindings dla equipment slotów.

---

## Equipped guard in transfer

`core.MoveItemsBetweenContainers` z [53](53-inventory-storage-transfer.md):
- Dla każdego handle przekazanego do transferu:
  - Jeśli `direction == TransferToStorage` i `IsHandleEquipped(slot, handle)` → skip z `SkipReasonEquipped`.
  - Storage → Inventory **nie** sprawdza equipped (założenie: items w Storage nie są założone).
- Skutek: użytkownik nie może przenieść założonego itemu z Inventory do Storage przez legacy core path. Musi go zdjąć w grze najpierw.

Pokrycie testowe: `tests/transfer_test.go::TestMoveEquippedInvToStorageSkipped` + 4 dodatkowe lokalizacje używające `IsHandleEquipped` do skip nie-equipped weapons w fixturach.

### Workspace path gap

Workspace save path (`editor.ApplyWorkspaceSave` + `writeContainerLayout`, [53](53-inventory-storage-transfer.md)) **nie** ma explicit `IsHandleEquipped` check. Cross-ref do [53](53-inventory-storage-transfer.md) (sekcja "Equipped guard / Workspace path") gdzie ten gap jest jawnie udokumentowany jako `needs verification`. W obecnym SortOrderTab UI użytkownik może (potencjalnie) przenieść założony item cross-grid bez blokady — implikacje in-game `needs verification`.

---

## What SaveForge does not implement

Edytor jest **read-only** dla sekcji ChrAsmEquipment z perspektywy publicznego API:

- ❌ Brak `App.SetEquipment*` — nie można zmienić itemu w konkretnym slocie equipment.
- ❌ Brak `App.SwapWeapon*` / `App.SwapArmor*` — nie ma swap między slotami.
- ❌ Brak `App.UnequipItem*` — nie można odekwipować przedmiotu.
- ❌ Brak `App.SetGreatRune` — `Player.EquippedGreatRune` jest read+write w `parseFromData`/`WriteSave`, ale brak public setter z UI.
- ❌ Brak walidacji equipment reference w workspace save path (item założony może być usunięty z inventory bez czyszczenia equipment slot — `needs verification` co do dangling reference w grze).

Pole `EquippedGreatRune` round-tripuje (load → save zachowuje wartość bez zmiany), ale **nie** może być modyfikowane z UI w obecnej wersji.

---

## Historical / external hypotheses

Poprzednia wersja `06-equipment.md` opisywała **cztery** struktury equipment (4 × 88B = 352B + 28B `ActiveWeaponSlotsAndArmStyle`) na podstawie zewnętrznych źródeł (`er-save-manager/parser/equipment.py`, ER-Save-Editor Rust, Cheat Engine tables). SaveForge **nie parsuje** tych hipotetycznych struktur:

| Hipotetyczna struktura | Status w SaveForge |
|---|---|
| `EquippedItemsEquipIndex` (88B) — acquisition_index do inventory | ❌ Nie parsowana. `needs verification` w `slot.Data` — może lub nie istnieć fizycznie w save. |
| `ActiveWeaponSlotsAndArmStyle` (28B) — `ArmStyle`, `LeftWeaponSlot`, `RightWeaponSlot`, `LeftArrowSlot`, …, `LeftBoltSlot`, `RightBoltSlot` | ❌ Nie parsowana. `needs verification`. |
| `EquippedItemsItemIds` (88B) — item IDs | ✅ Parsowana w SaveForge jako `EquipItemsIDOffset` (jedyna z czterech). |
| `EquippedItemsGaitemHandles` (88B) — GaItem handles per slot | ❌ Nie parsowana. `needs verification`. |

Hipotetyczne "Implikacje dla edycji" z poprzedniej wersji 06:
- "Zmiana broni wymaga aktualizacji WSZYSTKICH 3 struktur" — nieaktualne. SaveForge nie implementuje zmiany ekwipunku.
- "Active slot values: 0/1/2", "ArmStyle wpływa na animacje, nieprawidłowa wartość crashuje" — `needs verification` (nasz kod nie waliduje tych wartości).
- "Talisman 3&4 wymagają quest unlock", "Accessory 5 nieużywany" — `needs verification` (kod traktuje wszystkie 5 talizmanów równo).
- "Item ID encoding: Weapon +X = baseID + X" — semantyka stosowana w Add Items (zob. [43](43-transactional-item-adding.md)), ale w kontekście equipment slotów `needs verification`.
- "Slot Hair powiązany z Face Data Hair_Model_Id" — `needs verification`, nasz parser nie ma tej relacji.

---

## Relationship to GaItems and Inventory

- Encoded item-ID form w ChrAsmEquipment **nie zawiera** handle — zawiera item ID (z `GaMap`) lub bare lower bits. Bezpośrednie wnioskowanie handle → equipment slot wymaga skanu wszystkich kandydatów (zob. sekcja "Encoded item-ID forms").
- `IsHandleEquipped(slot, handle)` używa `slot.GaMap` lookup żeby zbudować candidate set — wymaga ważnego GaMap (zob. [03](03-gaitem-map.md)).
- Equipment slot nie wskazuje na konkretny rekord w `slot.Inventory.CommonItems` ani w `slot.Storage.CommonItems`. Powiązanie equipment → inventory entry istnieje **wyłącznie** przez `GaMap`: equipment trzyma item ID, GaMap mapuje handle → item ID, inventory record ma handle. Wnioskowanie back-reference (item ID → handle → inventory record) wymaga skanu.

---

## Relationship to Ash of War

AoW patching (zob. [54](54-ash-of-war.md)) modyfikuje `slot.GaItems[*].AoWGaItemHandle` w GaItem array, **nie** sekcję ChrAsmEquipment. Założenie nowego AoW na broń nie zmienia equipment slot ani nie wymaga jego aktualizacji. Hash 7 nie jest wrażliwy na AoW handle changes wewnątrz GaItem — wymaga tylko spójności item IDs w equipment slots.

---

## Validation and safety notes

- **Brak walidacji equipment reference**: SaveForge nie sprawdza, czy itemID w slocie ChrAsmEquipment faktycznie istnieje w `slot.GaItems` / `slot.GaMap`. Hipotetycznie dangling reference (slot wskazuje na usunięty item) jest możliwy, ale w obecnym kodzie nie ma operacji która by go wprowadziła (brak delete-equipped path).
- **In-game fallback (empirycznie)** — `needs verification`:
  - Equipped weapon usunięty z inventory → gra fallbackuje do "Unarmed" (`invUnarmedBaseID = 0x0001ADB0`, [36](36-inventory-categories-game-order.md) "Unarmed exclusion").
  - Equipped armor usunięty → default armor.
  - Equipped talisman usunięty → slot pusty.
- **Hash recompute**: w obecnym kodzie `RecalculateSlotHash` jest wywoływany **tylko w testach**. Edytor nie aktualizuje hash przy `WriteSave`. **`needs verification`** całościowe: czy gra waliduje hash przy load, oraz czy są scenariusze w których edytor powinien wywołać recompute (zmiany stats/level/souls, które wpływają na hashe 0/1/5).

---

## Test coverage

| Klasa | Pliki testów |
|---|---|
| Equipped guard transfer | `tests/transfer_test.go::TestMoveEquippedInvToStorageSkipped` |
| `IsHandleEquipped` dla nie-equipped fixtures | `tests/transfer_test.go` (4 lokalizacje: `:226`, `:259`, `:373`, `:618` — skip not-equipped weapons w fixturach) |
| Hash determinizm + recompute | `backend/core/hash_test.go::TestComputeSlotHash_Deterministic`, `TestRecalculateSlotHash_WritesToData`, `TestComputeSlotHash_ChangingStatsChangesHash` |

**Brakuje**:
- Testów `IsHandleEquipped` dla każdej z 6 form kandydatów (lower 28 bits, prefix-swap dla 0xA0/0xB0).
- Testów dla equipped armor, equipped talisman, equipped AoW jako odrębnych prefixes — obecne testy pokrywają głównie weapons.
- Testów workspace path equipped behavior (czy `Validate(snap)` lub `writeContainerLayout` blokuje transfer założonych).
- Round-trip testu `EquippedGreatRune` (load → write → load, czy wartość się zachowuje).

---

## Known limits / needs verification

- **Sloty 10/11/16** (`unk0x28 / unk0x2C / unk0x40`) — kod nie nadaje semantyki. Hipotezy z `er-save-manager`/CE: Arrows3/Bolts3/Hair. `needs verification`.
- **Slot 10 vs Great Rune offset** — slot 10 fizycznie zawiera `EquippedGreatRune` (offset 0x28 = bajt 40 = slot 10). Jednoczesna hipoteza "Arrows 3" jest sprzeczna z tym użyciem. `needs verification` co do faktycznej semantyki gry.
- **`Talisman3` / `Talisman4` / `Talisman5`** — hipotetycznie wymagają quest unlock (Talisman Pouch items). Kod traktuje wszystkie jako pełne sloty. `needs verification` co do egzekwowania przez grę przy próbie założenia bez unlocka.
- **Hipotetyczne 3 dodatkowe struktury equipment** (`EquipIndex`, `ActiveWeaponSlotsAndArmStyle`, `GaitemHandles`) — nie parsowane przez SaveForge. `needs verification` co do ich rzeczywistej obecności w binary save.
- **Workspace path equipped guard** — brak explicit checku. Cross-ref do [53](53-inventory-storage-transfer.md).
- **In-game behavior dangling equipment reference** — nieweryfikowane empirycznie.
- **DLC Great Rune item IDs** — lista 7 vanilla ID's nie obejmuje potencjalnych DLC dodatków. `needs verification` po stronie DB.
- **Hash recompute discipline** — `RecalculateSlotHash` wywoływany wyłącznie w testach. `needs verification` całościowe: czy edytor powinien aktualizować hash w `WriteSave` po edycjach które wpływają na hash 0/1/5 (stats, level, souls).
- **Encoded form a edytor** — gdyby kiedyś powstał `App.SetEquipment*`, musiałby:
  1. Zapisywać `itemID | 0x80000000` (weapons/armor/AoW) lub bare lower (talismans), nie raw handle.
  2. Aktualizować hash 7/8 jeśli zmiana dotyka slotów 0-9 / 12-15 / 17-21.
  3. Walidować że item ID istnieje w `slot.GaMap` / `slot.GaItems`.
  Żadne z tych nie jest obecnie zaimplementowane.

---

## Cross-references

- [03 — GaItem map](03-gaitem-map.md) — handle ↔ itemID mapping przez `GaMap`, prefix-swap funkcje (`HandleToItemID`, `ItemIDToHandlePrefix`).
- [07 — Inventory model](07-inventory.md) — read-side rekord 12B, offsety CommonItems, dlaczego equipment nie wskazuje bezpośrednio na inventory record.
- [10 — Storage model](10-storage.md) — analogiczny read-side dla Storage.
- [35 — GaItem allocator invariants](35-gaitem-allocator-invariants.md) — handle allocation (niezależnie od equipment slotów).
- [36 — Inventory Categories and Game Order](36-inventory-categories-game-order.md) — handle prefix classification (`GetItemCategoryFromHandle`), Unarmed placeholder, DLC flag mechanism.
- [43 — Transactional item adding](43-transactional-item-adding.md) — Add Items nie modyfikuje equipment slotów.
- [53 — Inventory ↔ Storage transfer](53-inventory-storage-transfer.md) — pełny opis equipped guard w legacy core path + workspace path gap.
- [54 — Ash of War](54-ash-of-war.md) — AoW patching modyfikuje GaItem, nie equipment.

---

## Sources

### Local code

- `backend/core/structures.go` — `EquipItemsIDOffset`, `Player.EquippedGreatRune`, `parseFromData`, `WriteSave`.
- `backend/core/hash.go` — `ChrAsmFieldCount`, `ChrAsmEquipmentSize`, layout komentarz, `weaponSlotIndices`, `armorSlotIndices`, `readEquipSection`, `extractSlots`, `equipmentHash`, `ComputeSlotHash`, `RecalculateSlotHash`.
- `backend/core/transfer.go` — `IsHandleEquipped`, candidate matching, equipped guard użycie.
- `backend/core/offset_defs.go` — `DynEquipGreatRune = 0x28`, `DynEquipedItemsID = 0x58`, offset chain konstanty.
- `tests/transfer_test.go` — `TestMoveEquippedInvToStorageSkipped`, użycia `IsHandleEquipped` w fixture skipach.
- `backend/core/hash_test.go` — `TestComputeSlotHash_*`, `TestRecalculateSlotHash_WritesToData`.

### External hypotheses (historical context, nie używane w SaveForge)

- `er-save-manager/parser/equipment.py` — klasy `EquipmentSlots`, `ActiveWeaponSlotsAndArmStyle` (linie 20-163).
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — referenced structures.
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — ChrAsm offsets (ArmStyle, weapon slots, equipment IDs).
- Cheat Engine: `ER_TGA_v1.9.0` — ChrAsm 2 (equipment item IDs 0x5D8-0x62C), Quick Items, Pouch, Great Rune.

Te źródła pozostają jako kontekst dla badań/eksperymentów, ale **nie są** source-of-truth dla aktualnego modelu SaveForge.
