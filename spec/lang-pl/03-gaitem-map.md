# 03 — GaItem Map: binary model, handles and item references

> **Typ**: Binary format spec
> **Status**: ✅ canonical, implemented, source-of-truth zweryfikowane przeciw aktualnemu backendowi. Allocator details, capacity invariants i transactional safety opisane są w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
> **Zakres**: Read-side binary model tablicy `slot.GaItems`, mapy `slot.GaMap`, formatów rekordów per typ przedmiotu, prefiksów handle i relacji handle ↔ ItemID. Rozdział pokrywa to, co parser robi przy ładowaniu save'a, i jak runtime struktury odwzorowują surowe bajty. Write-side allocation (kursory, capacity, validation, rollback) jest poza tym rozdziałem.

---

## Cel rozdziału

Rozdział opisuje **read-side binary model** GaItem:

- czym jest tablica `GaItems` i jak parser ją odczytuje;
- jak działa `slot.GaMap` (mapping `handle → ItemID`);
- jak rozróżnia się typy wpisów (broń / zbroja / talizman / goods / AoW) po prefiksie handle;
- jak inventory i storage referencują GaItem'y przez handle, nie przez ItemID;
- jakie relacje muszą zachodzić między tymi warstwami, aby gra zaakceptowała save.

Co rozdział **NIE** robi:

- Nie opisuje allocator'a (`allocateGaItem`), counterów (`NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`), capacity 5118/5120, AoW guard, snapshot/rollback ani post-mutation validation — to wszystko w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
- Nie wchodzi w pełną semantykę Ash of War (sentinele weapon `AoWGaItemHandle`, dual-destination, strict apply) — to w [54-ash-of-war](54-ash-of-war.md).
- Nie opisuje formatu sekcji inventory/storage ani bramki widoczności `NextEquipIndex` — to w [07-inventory](07-inventory.md), [10-storage](10-storage.md), [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

---

## Source of truth w kodzie

| Temat | Pliki / funkcje | Notatka |
|---|---|---|
| Struktura wpisu | `backend/core/structures.go:66-73` (`GaItemFull`) | pola: `Handle`, `ItemID`, `Unk2/Unk3`, `AoWGaItemHandle`, `Unk5` |
| Rozmiar rekordu | `backend/core/structures.go:75-89` (`GaItemRecordSize`) | rozmiar zależy od `ItemID` (nie od handle), dopasowany do Rust ER-Save-Editor |
| Stałe rozmiarów | `backend/core/offset_defs.go:46-49` | `GaRecordWeapon = 21`, `GaRecordArmor = 16`, `GaRecordItem = 8` |
| Handle constants | `backend/core/offset_defs.go:55-57` | `GaHandleEmpty = 0x00000000`, `GaHandleInvalid = 0xFFFFFFFF`, `GaHandleTypeMask = 0xF0000000` |
| Prefiksy typów | `backend/core/structures.go:20-24` | `ItemTypeWeapon = 0x80000000`, `ItemTypeArmor = 0x90000000`, `ItemTypeAccessory = 0xA0000000`, `ItemTypeItem = 0xB0000000`, `ItemTypeAow = 0xC0000000` |
| Punkt startu sekcji | `backend/core/offset_defs.go:40` | `GaItemsStart = 0x20` |
| Tablica `GaItems` | `backend/core/structures.go:226` (`SaveSlot.GaItems []GaItemFull`) | długość = `len(slot.GaItems)`, ustalona raz przy load |
| Mapa `GaMap` | `backend/core/structures.go::SaveSlot.GaMap map[uint32]uint32` | builds in `scanGaItems`; tylko non-empty entries; klucz = handle, wartość = ItemID |
| Parser | `backend/core/structures.go:612-728` (`scanGaItems`) | dwuprzebiegowe skanowanie: pierwsze parsuje wpisy, drugie rekonstruuje countery (countery udokumentowane w 35) |
| Container reference | `backend/core/structures.go:118-122` (`InventoryItem`) | pola: `GaItemHandle`, `Quantity`, `Index` |

---

## Mental model

Model GaItem składa się z trzech warstw:

```
┌─────────────────────────────────────────────────────────┐
│  Container layer                                         │
│  Inventory.CommonItems, Inventory.KeyItems, Storage.*    │
│  → 12-bajtowe rekordy { handle, quantity, index }        │
└─────────────────────────────────────────────────────────┘
                      │ handle (u32)
                      ▼
┌─────────────────────────────────────────────────────────┐
│  Reference layer                                         │
│  slot.GaMap : map[uint32]uint32                          │
│  → quick lookup handle → ItemID                          │
│  → wypełniana w scanGaItems dla non-empty wpisów         │
└─────────────────────────────────────────────────────────┘
                      │ handle (u32)
                      ▼
┌─────────────────────────────────────────────────────────┐
│  Object layer                                            │
│  slot.GaItems : []GaItemFull                             │
│  → fizyczna tablica (5118 lub 5120 wpisów, zależnie od   │
│    slot.Version — szczegóły w 35)                        │
│  → wpis trzyma handle, ItemID i variant-specific fields  │
│    (Unk2, Unk3, AoWGaItemHandle, Unk5)                   │
└─────────────────────────────────────────────────────────┘
```

Trzy warstwy są niezależne pod względem rozmiaru:

- **`GaItems`** — tablica obiektów; `len(slot.GaItems) ∈ {5118, 5120}` (per `slot.Version`).
- **`GaMap`** — mapa referencji; rozmiar ≤ liczba non-empty wpisów w `GaItems`.
- **Container layer** — `Inventory.CommonItems` ma 2688 slotów, `Inventory.KeyItems` 384, `Storage.CommonItems` 1920, `Storage.KeyItems` 128. Slot pusty = `handle == 0` lub `handle == 0xFFFFFFFF`.

Te rozmiary są niezależne: można mieć 60 non-empty wpisów w `GaItems`, ale wszystkie 2688 slotów inventory są dostępne dla referencji — pod warunkiem, że każdy używany handle istnieje w `GaMap`.

> ℹ️ Liczba **GaItem instancji** nie jest tym samym co **liczba wpisów w inventory/storage**. Wpis inventory niesie tylko handle (referencję); fizyczny obiekt żyje w `GaItems`. Goods (`0xB0`) są **stackable**: jeden wpis `GaItems` może reprezentować stos N sztuk, którego ilość jest przechowywana w `Quantity` rekordu container (`InventoryItem.Quantity`), nie w `GaItemFull`.

---

## Binary structures

### Tablica `GaItems`

- Lokalizacja: od `GaItemsStart = 0x20` do `slot.MagicOffset - DynPlayerData + 1` (`structures.go:618-620`).
- Liczba wpisów: 5118 dla `slot.Version ∈ [1, 81]`, 5120 dla `slot.Version > 81` (decyzja w `scanGaItems`, [35](35-gaitem-allocator-invariants.md#gaitem-capacity-by-slot-version)).
- Każdy wpis to `GaItemFull`; rozmiar serializowanego rekordu zależy od **`ItemID`**, nie od handle (`structures.go:75-89`):

```go
func GaItemRecordSize(itemID uint32) int {
    if itemID == 0 || itemID == GaHandleInvalid {
        return GaRecordItem // 8
    }
    switch itemID & 0xF0000000 {
    case 0x00000000:
        return GaRecordWeapon // 21
    case 0x10000000:
        return GaRecordArmor // 16
    default:
        return GaRecordItem // 8
    }
}
```

### Layout per typ (z `GaItemFull.Serialize`, `structures.go:103-116`)

| Offset | Typ | Pole | Obecność |
|---|---|---|---|
| 0x00 | u32 | `Handle` | zawsze |
| 0x04 | u32 | `ItemID` | zawsze |
| 0x08 | i32 | `Unk2` (default `-1`) | tylko gdy `recSize ≥ GaRecordArmor` (16 B) — zbroja i broń |
| 0x0C | i32 | `Unk3` (default `-1`) | tylko gdy `recSize ≥ GaRecordArmor` (16 B) — zbroja i broń |
| 0x10 | u32 | `AoWGaItemHandle` | tylko gdy `recSize ≥ GaRecordWeapon` (21 B) — wyłącznie broń |
| 0x14 | u8 | `Unk5` (default `0`) | tylko broń |

Rozmiary:

| Klasa przedmiotu | `ItemID` upper nibble | Rozmiar rekordu | Pola |
|---|---|---|---|
| Broń | `0x0...` | `GaRecordWeapon = 21` B | `Handle`, `ItemID`, `Unk2`, `Unk3`, `AoWGaItemHandle`, `Unk5` |
| Zbroja | `0x1...` | `GaRecordArmor = 16` B | `Handle`, `ItemID`, `Unk2`, `Unk3` |
| Goods / Accessory / Talisman / Ash of War | inne | `GaRecordItem = 8` B | `Handle`, `ItemID` |

> ℹ️ Rozmiar jest determinowany przez `ItemID`, ale **typ użycia** przedmiotu wynika z prefiksu **handle** (patrz "Handle prefixes" niżej). Te dwa kanały są spójne dla legalnych save'ów; rozjazd między nimi to symptom uszkodzenia. Pełna walidacja: `ValidatePostMutation` w [35](35-gaitem-allocator-invariants.md#post-mutation-validation).

### Mapa `GaMap`

- Typ: `map[uint32]uint32` (handle → ItemID).
- Budowana w `scanGaItems` (`structures.go:660-672`): dla każdego non-empty wpisu, którego `typeBits ∈ {ItemTypeWeapon, ItemTypeArmor, ItemTypeAccessory, ItemTypeItem, ItemTypeAow}`, dodaje wpis `GaMap[handle] = itemID`.
- **Nie zawiera empty entries** — `IsEmpty()` (`structures.go:96-99`) zwraca true, gdy `ItemID == 0` lub `ItemID == 0xFFFFFFFF`.
- Nie zawiera wpisów z `typeBits` poza pięcioma znanymi typami — gdyby parser napotkał handle z innym prefiksem, wpis trafia do `GaItems`, ale nie do `GaMap` (`structures.go:668-672`).

### Pusty wpis

`scanGaItems` używa dwóch reprezentacji "empty":

1. Wpisy poza zakresem danych: `GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}` (handle = 0, ItemID = 0).
2. Wpisy w zakresie, gdzie `Handle == 0` lub `ItemID ∈ {0, 0xFFFFFFFF}` — `IsEmpty()` zwraca true.

Implikacja: `slot.GaItems` jest **fizycznie zawsze pełną tablicą** o stałym rozmiarze. Pusty slot to wpis z `IsEmpty() == true`, nie brak wpisu.

---

## Handle prefixes and item classes

Handle to `u32`, którego górny nibble (`handle & 0xF0000000`) określa typ. Pięć potwierdzonych prefiksów w kodzie (`structures.go:20-24`):

| Handle prefix | Stała | Klasa | ItemID upper nibble (po `HandleToItemID`) | Typowy rozmiar GaItem |
|---|---|---|---|---|
| `0x80000000` | `ItemTypeWeapon` | Broń | `0x0...` | 21 B |
| `0x90000000` | `ItemTypeArmor` | Zbroja (Protector) | `0x1...` | 16 B |
| `0xA0000000` | `ItemTypeAccessory` | Akcesorium / Talizman | `0x2...` | 8 B |
| `0xB0000000` | `ItemTypeItem` | Goods (consumables, materiały, key items w `Inventory.KeyItems`) | `0x4...` | 8 B |
| `0xC0000000` | `ItemTypeAow` | Ash of War gem (instancja) | `0x8...` | 8 B |

Dodatkowo:

| Wartość | Stała | Znaczenie |
|---|---|---|
| `0x00000000` | `GaHandleEmpty` | slot pusty (canonical) |
| `0xFFFFFFFF` | `GaHandleInvalid` | slot pusty (legacy / nieprawidłowy) |

> ℹ️ **Mapping handle prefix ↔ ItemID prefix** jest deterministyczny dla pięciu klas powyżej (`backend/db/db.go:589-611`, `HandleToItemID`):
>
> ```
> 0x80 (weapon)    → 0x00
> 0x90 (armor)     → 0x10
> 0xA0 (talisman)  → 0x20
> 0xB0 (goods)     → 0x40
> 0xC0 (AoW gem)   → 0x80
> ```
>
> Funkcja zwraca `(handle & 0x0FFFFFFF) | <ItemID prefix>`. Odwrotny mapping w `ItemIDToHandlePrefix` (`backend/db/db.go:613-630`). Tabela powyżej opisuje high-level klasyfikację — dokładny mapping pod-zakresów ItemID dla konkretnych itemów (np. wszystkie DLC weapon classes) jest opracowywany per kategoria w [36-inventory-categories-game-order](36-inventory-categories-game-order.md). Kompletność DLC sub-mapping: `needs verification`.

### Special case: weapon → AoW reference

Rekord broni (`GaRecordWeapon = 21` B) zawiera 4-bajtowe pole `AoWGaItemHandle` na offset `0x10`. To pole referuje **inny GaItem** w tej samej tablicy — instancję Ash of War gem (handle z prefiksem `0xC0`). Wartości:

| Wartość | Znaczenie |
|---|---|
| `0x00000000` | Brak custom AoW — canonical sentinel zapisywany przez writer i grę. |
| `0xFFFFFFFF` | Brak custom AoW — legacy sentinel akceptowany przez reader (zapisywany przez starsze buildy SaveForge sprzed commita `4e800b9`). |
| `0xC0xxxxxx` | Valid custom AoW handle. Musi rozwiązywać się do istniejącego wpisu AoW w `GaMap`. |

> ⚠️ Pełna semantyka (built-in vs custom skill resolution, invariant unikalności handle, strict apply, dual-destination) — [54-ash-of-war](54-ash-of-war.md). Reguły allocator side dla nowego AoW (capacity guard `6881cb9`) — [35](35-gaitem-allocator-invariants.md#aow-allocation-edge-case-fixed-by-6881cb9).

---

## GaItems vs containers

Trzy warstwy z "Mental model" mają osobne pojemności, role i lifecycle.

| Warstwa | Rozmiar | Co przechowuje | Lifecycle |
|---|---|---|---|
| `slot.GaItems` | 5118 lub 5120 wpisów (fixed per slot) | obiekt/instancję przedmiotu z metadanymi (upgrade, infusion via ItemID, AoW reference) | trwały — entry nie znika przy wyrzuceniu z inventory (chyba że ktoś go celowo wyczyści) |
| `slot.GaMap` | ≤ liczba non-empty entries | quick handle → ItemID lookup | rebuildowane z `GaItems` przy load |
| `slot.Inventory.CommonItems` / `KeyItems` | 2688 / 384 | referencję `{handle, quantity, index}` | znika z inventory, gdy slot wyzerowany; obiekt w `GaItems` może zostać orphan |
| `slot.Storage.CommonItems` / `KeyItems` | 1920 / 128 | jak wyżej | analogicznie |

### Dlaczego rozmiar GaItems ≠ rozmiar inventory

- Inventory + Storage common łącznie = 2688 + 1920 = **4608 sloty** referencji.
- `GaItems` = 5120 wpisów (dla nowych save'ów).
- Różnica (`5120 - 4608 = 512`) daje przestrzeń dla:
  - Stackable goods (`0xB0`) — jeden GaItem może być referencjonowany przez `1..N` wpisów inventory ze wspólnym handle (ten sam stack).
  - AoW gems (`0xC0`) — referencjonowane przez weapon GaItem przez pole `AoWGaItemHandle`, nie przez container entry.
  - Orphans — wpisy GaItem, do których żaden container/weapon-AoW-reference nie wskazuje. To akumulują się, gdy edytor usuwa rekord container bez wyczyszczenia GaItem. Naprawa: `RepairOrphanedGaItems` (opisana w [35](35-gaitem-allocator-invariants.md#source-of-truth-w-kodzie)).

### Goods stackability (model)

Dla goods `0xB0`:
- Jeden GaItem (8 B) reprezentuje jedno **wystąpienie typu**.
- Inventory entry trzyma quantity stacku.
- Ten sam handle może być w **dwóch różnych container layerach** (np. Inventory.CommonItems i Storage.CommonItems) — wtedy obie strony używają tej samej referencji. Pełna semantyka transferu: [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

Dla instance-move types (`0x80/0x90/0xA0/0xC0`):
- Jeden GaItem to jedna fizyczna instancja.
- Każde "kopia" tego samego przedmiotu (np. dwie Uchigatany z różnymi affinities) ma osobny GaItem z osobnym handle.

---

## Read path

Parser ładuje sekcję GaItem dla każdego niepustego slotu w pojedynczym wywołaniu (`structures.go:301`):

```
s.scanGaItems(GaItemsStart)        // GaItemsStart = 0x20
```

`scanGaItems` (`structures.go:612-728`) wykonuje dwa przebiegi:

### Przebieg 1 — parsing wpisów (`structures.go:615-672`)

1. Wylicza `gaLimit = s.MagicOffset - DynPlayerData + 1` (górna granica sekcji w danych slotu).
2. Wybiera capacity (5118 lub 5120) per `slot.Version` (szczegóły w [35](35-gaitem-allocator-invariants.md#gaitem-capacity-by-slot-version)).
3. Alokuje `s.GaItems = make([]GaItemFull, maxEntries)` i inicjalizuje `s.GaMap`.
4. Iteruje od `start` do `gaLimit`, kursor `curr`:
   - Jeśli `curr + GaRecordItem > gaLimit` → pozostałe wpisy zostawia jako empty (`Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF`).
   - W przeciwnym razie czyta `Handle` (offset 0), `ItemID` (offset 4).
   - Liczy `recSize := GaItemRecordSize(itemID)`.
   - Jeśli `recSize >= GaRecordArmor` (16) i `curr + 16 ≤ len(Data)` → czyta `Unk2` i `Unk3`.
   - Jeśli `recSize >= GaRecordWeapon` (21) i `curr + 21 ≤ len(Data)` → czyta `AoWGaItemHandle` i `Unk5`.
   - Inkrementuje `curr += recSize`.
   - Jeśli `!entry.IsEmpty()` i `typeBits ∈ {Weapon, Armor, Accessory, Item, Aow}` → dodaje `GaMap[handle] = itemID`.

### Przebieg 2 — rekonstrukcja counterów (`structures.go:681-722`)

Drugi przebieg ustawia `NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`, `PartGaItemHandle`. Algorytm i właściwości tych counterów są w pełni opisane w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md#counter-reconstruction-on-load) — rozdział 03 nie powiela tego.

### Po skanowaniu

- `s.GaItems` jest pełną tablicą o stałej długości.
- `s.GaMap` zawiera wpisy tylko dla non-empty entries z legalnym prefiksem.
- `s.InventoryEnd` (`structures.go:677`) wskazuje na byte offset za ostatnim parsowanym wpisem — używany do walidacji i kolejnego segmentu sekcji slotu.

---

## Write path overview

Write-side semantics dla GaItem **nie są opisane w tym rozdziale**. Crucial dla implementatora:

- Alokacja nowych wpisów: `allocateGaItem` ([35 → Allocation zones](35-gaitem-allocator-invariants.md#allocation-zones)).
- Generowanie unikalnych handle: `generateUniqueHandle` ([35 → Handle generation](35-gaitem-allocator-invariants.md#handle-generation)).
- Batch add: `AddItemsToSlotBatch` + `RebuildSlotFull` ([35 → Transactional safety](35-gaitem-allocator-invariants.md#transactional-safety-and-rollback)).
- Rollback: `SnapshotSlot` / `RestoreSlot` ([35](35-gaitem-allocator-invariants.md#transactional-safety-and-rollback)).
- Full architecture pre-flight → snapshot → mutate → reconcile → validate: [43-transactional-item-adding](43-transactional-item-adding.md).
- Serializacja pojedynczego wpisu: `GaItemFull.Serialize` (`structures.go:103-116`) — używana wewnętrznie przez writer i `RebuildSlotFull`.

> **Allocator/counter/capacity invariants są udokumentowane w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).** Rozdział 03 świadomie nie powiela tej zawartości.

---

## Validation relationships

Read-side warstwa GaItem musi spełniać kilka relacji, by save był interpretowalny. Pełna lista invariantów i ich enforcement w `ValidatePostMutation` — [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants). Skrót dla read-side:

- Każdy non-empty inventory handle typu `0x80/0x90/0xC0` (weapon/armor/AoW) musi istnieć jako klucz w `slot.GaMap`. Handle bez wpisu w mapie → "orphan handle" (`diagnostics.go::ValidatePostMutation`, check `orphan_handle`).
- `slot.GaMap[h]` nigdy nie może mapować do `ItemID == 0` (check `gamap_zero_id`).
- Pole `AoWGaItemHandle` w rekordzie broni:
  - `0x00000000` lub `0xFFFFFFFF` (sentinele "brak custom AoW") → legalne.
  - `0xC0xxxxxx` → handle MUSI istnieć jako AoW wpis w `GaMap`.
  - Inna wartość → invalid / corrupted.
- Żadne dwie bronie nie mogą referować tego samego non-sentinel `AoWGaItemHandle` (handle uniqueness; pełna analiza w [54-ash-of-war](54-ash-of-war.md)).

---

## Examples

### Container entry → handle → GaMap → ItemID → DB

Save zawiera w `Inventory.CommonItems[5]`:

```
GaItemHandle = 0x80000003
Quantity     = 1
Index        = 15
```

Krok 1: kontener trzyma referencję handle, nic więcej.

Krok 2: `slot.GaMap[0x80000003] = 0x000F4240` (1 000 000 dec) — lookup w runtime mapie.

Krok 3: lookup pełnego rekordu `slot.GaItems[i]` po przejściu tablicą (lub po `i` znanym z parsowania):

```
GaItems[3].Handle           = 0x80000003
GaItems[3].ItemID           = 0x000F4240  → Uchigatana +0
GaItems[3].Unk2             = -1
GaItems[3].Unk3             = -1
GaItems[3].AoWGaItemHandle  = 0x00000000   ← brak custom AoW (canonical sentinel)
GaItems[3].Unk5             = 0
```

Krok 4: DB lookup: `db.GetItemDataFuzzy(0x000F4240)` → `ItemData{Name: "Uchigatana", Category: "melee_armaments", ...}`.

### Weapon z custom AoW

Załóżmy broń z przypiętym Lion's Claw:

```
GaItems[3].Handle           = 0x80000003       ← weapon Uchigatana +0
GaItems[3].ItemID           = 0x000F4240
GaItems[3].AoWGaItemHandle  = 0xC0000017       ← reference do AoW gem

GaItems[42].Handle          = 0xC0000017       ← osobna instancja AoW
GaItems[42].ItemID          = 0x80002710       → Lion's Claw
```

- Broń i AoW gem to **dwa osobne wpisy GaItem**.
- Handle AoW musi być unikalne w `GaMap` (zob. invariant uniqueness w [54](54-ash-of-war.md)).
- Usunięcie custom AoW = ustawienie `weapon.AoWGaItemHandle = 0x00000000`; rekord AoW gem zostaje jako wolna kopia (semantyka detach i strict apply w [54](54-ash-of-war.md)).

---

## Known limits / needs verification

- **Pełna lista DLC item ID ranges** per klasa — `needs verification`. Znamy pięć głównych prefiksów handle i ich powiązanie z ItemID upper nibble; kompletny mapping pod-zakresów dla DLC (np. Backhand Blades, Perfume Bottles) jest opracowywany per kategoria w [36](36-inventory-categories-game-order.md), ale rozdział 03 nie próbuje go powielać.
- **`PartGaItemHandle` znaczenie semantyczne** — pole `slot.PartGaItemHandle` (1 bajt, default `0x80`) wpływa na bity 16–23 generowanych handle. Pełen efekt na grze (czy gra waliduje to pole, czy je ignoruje) — `needs verification`.
- **Wpisy z handle prefiksem poza pięcioma znanymi typami** — parser czyta je jako `GaItem` (jeśli pasują rozmiarem), ale nie dodaje do `GaMap`. Czy gra je toleruje, czy traktuje jako corruption — `needs verification`.

---

## Cross-references

- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator, countery, capacity per slot version, snapshot/rollback, post-mutation validation, AoW guard `6881cb9`.
- [07-inventory](07-inventory.md) — Inventory layout (CommonItems, KeyItems, bramka widoczności, headers).
- [10-storage](10-storage.md) — Storage layout (różnice rozmiarów względem inventory).
- [43-transactional-item-adding](43-transactional-item-adding.md) — pełna architektura Add Items (pre-flight → snapshot → mutate → reconcile → validate).
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — `acquisition_index` w container entry: jak gra sortuje i dlaczego stride-2.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — transfer między inventory i storage; rehandle path dla duplicate handle.
- [54-ash-of-war](54-ash-of-war.md) — pełna semantyka AoW: sentinele `AoWGaItemHandle`, invariant unikalności handle, strict apply, dual-destination.

---

## Sources

- `backend/core/structures.go` — `GaItemFull`, `GaItemRecordSize`, `scanGaItems`, `SaveSlot.GaItems`, `SaveSlot.GaMap`, `InventoryItem`, `IsEmpty`, `Serialize`
- `backend/core/offset_defs.go` — `GaItemsStart`, `GaRecordWeapon/Armor/Item`, `GaHandleEmpty/Invalid/TypeMask`
- `backend/core/diagnostics.go` — `ValidatePostMutation` (read-side invariants)
- `backend/db/db.go` — `HandleToItemID`, `ItemIDToHandlePrefix`, `GetItemDataFuzzy`
- Rust ER-Save-Editor: `src/save/common/save_slot.rs` (`GaItem2` struct, referencyjny model)
- er-save-manager: `parser/er_types.py` (`Gaitem` class), `parser/user_data_x.py` (`gaitem_map`)
