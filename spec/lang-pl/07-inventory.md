# 07 — Inventory: data model and read path

> **Typ**: Binary format spec
> **Status**: ✅ canonical, implemented, source-of-truth zweryfikowane przeciw aktualnemu backendowi. Allocator/capacity/counter invariants opisane są w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md). Transakcyjne Add Items — w [43-transactional-item-adding](43-transactional-item-adding.md).
> **Zakres**: Read-side binary model `slot.Inventory` (common items + key items), trailing counters, load-time reconciliation, relacja do GaItems/GaMap. Rozdział pokrywa to, co parser robi przy ładowaniu inventory'ego, i jak runtime struktury odwzorowują surowe bajty. Pełna semantyka transferu Inventory ↔ Storage, sortowania i drag-and-drop UI pozostaje w [53-inventory-storage-transfer](53-inventory-storage-transfer.md) i [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).

---

## Cel rozdziału

Rozdział opisuje:

- gdzie inventory siedzi w pliku save (offset relatywny do `slot.MagicOffset`);
- jak zbudowane są dwie kolekcje: `CommonItems` (2688 slotów) i `KeyItems` (384 slotów);
- format pojedynczego rekordu inventory (12 B: `handle`, `quantity`, `index`);
- trailing counters: `NextEquipIndex`, `NextAcquisitionSortId`, oraz `common_item_count` header;
- co parser robi przy load i jakie reconciliations wykonuje automatycznie;
- jak inventory entry odwołuje się do warstw GaMap / GaItems / DB;
- które invarianty muszą obowiązywać po stronie read, a które są egzekwowane przez allocator/validator (link do 35).

Co rozdział **NIE** robi:

- Nie opisuje allocator side (alokacja nowych wpisów, capacity, AoW guard, post-mutation validation, snapshot/rollback) — to wszystko w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
- Nie opisuje pełnej semantyki transferu Inventory ↔ Storage (rehandle, cap-aware partial merge) — to w [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- Nie opisuje stride-2 algorytmu Sort Order / Acquisition reorder — to w [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- Nie opisuje architektury Add Items (pre-flight, snapshot, rollback) — to w [43-transactional-item-adding](43-transactional-item-adding.md).

---

## Status

- canonical
- implemented (parser w `mapInventory` zwery­fikowany przeciw fixturom round-trip PS4 + PC)
- source-of-truth: kod backend
- allocator details: **chapter 35**

---

## Source of truth w kodzie

| Temat | Pliki / funkcje | Notatka |
|---|---|---|
| Struktura inventory | `backend/core/structures.go:124-131` (`EquipInventoryData`) | pola: `CommonItems`, `KeyItems`, `NextEquipIndex`, `NextAcquisitionSortId`, prywatne `nextEquipIndexOff` i `nextAcqSortIdOff` |
| Pojedynczy rekord | `backend/core/structures.go:118-122` (`InventoryItem`) | `GaItemHandle uint32`, `Quantity uint32`, `Index uint32` |
| Stałe layoutu | `backend/core/offset_defs.go:60-73` | `InvStartFromMagic = 505`, `CommonItemCount = 0xA80 (2688)`, `KeyItemCount = 0x180 (384)`, `InvRecordLen = 12`, `InvKeyCountHeader = 4`, `InvSafetyMargin = 0x9000` |
| Parser kontenerów | `backend/core/structures.go:156-195` (`(*EquipInventoryData).Read`) | sekwencja: common × `commonCount`, skip 4 B key_count header, key × `keyCount`, NextEquipIndex (zapamiętany offset), NextAcquisitionSortId (zapamiętany offset) |
| Top-level load | `backend/core/structures.go:757-804` (`(*SaveSlot).mapInventory`) | wywołuje `Read`, reconciluje countery (acq sort id, equip index), reconciluje binarny `common_item_count` header przez `ReconcileInventoryHeader` |
| Reconcile header | `backend/core/writer.go:993-1013` (`ReconcileInventoryHeader`) | wylicza non-empty count i zapisuje go pod `invStart - 4` |
| Deep copy | `backend/core/structures.go:138-154` (`(*EquipInventoryData).Clone`) | używane przez `SnapshotSlot` |
| UI capacity binding | `app_deploy.go:31-38, 289-306` (`SlotCapacity`, `GetSlotCapacity`) | eksponuje `non_empty / max` (2688) per kontener Inventory; UI fix dla "armament zone free" — patrz [35](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity) |

---

## Mental model

Inventory siedzi w stałym obszarze danych slotu, zaadresowanym przez `slot.MagicOffset + InvStartFromMagic`. To **container layer** modelu GaItem (zob. [03-gaitem-map](03-gaitem-map.md#mental-model)) — trzyma referencje do wpisów `GaItems` przez `GaItemHandle`, plus per-stack `Quantity` i acquisition `Index` używane przez sortowanie in-game.

```
slot.Data
 │
 ├── [MagicOffset + InvStartFromMagic − 4] ┐
 │       common_item_count (u32)           │  4 B header
 │       (game uses this as "next insert    │
 │        index" and "inventory full"       │
 │        gate; reconciled at load)         │
 ├── [MagicOffset + InvStartFromMagic]     ┘
 │       CommonItems × 2688                    32 256 B  (0x7E00)
 │       — każdy rekord 12 B: {handle, qty, index}
 │
 ├── (po common)                          ┐
 │       key_count header (u32)           │  4 B (skipowane przy read, nie używane runtime)
 │
 ├── (next)                                ┘
 │       KeyItems × 384                       4 608 B  (0x1200)
 │       — identyczny rekord 12 B
 │
 ├── (po key)                              ┐
 │       NextEquipIndex (u32)              │  4 B  (offset zapamiętany w nextEquipIndexOff)
 │       NextAcquisitionSortId (u32)       │  4 B  (offset zapamiętany w nextAcqSortIdOff)
 └── ───────────────────────────────────────┘
```

Łączny rozmiar sekcji (bez wiodącego 4 B header, ale z trailing counters):

```
CommonItems   = 2688 × 12 = 32 256  (0x7E00)
key_count hdr =                  4  (0x0004)
KeyItems      =  384 × 12 =  4 608  (0x1200)
NextEquipIdx  =                  4  (0x0004)
NextAcqSortId =                  4  (0x0004)
──────────────────────────────────
TOTAL                     = 36 876  (0x900C)
```

`InvSafetyMargin = 0x9000` (`offset_defs.go:70`) jest progiem walidacji "czy slot ma w ogóle miejsce na inventory section". Wartość jest **niższa** od pełnej długości sekcji (`0x900C`) o 12 B; parser woła `Read` tylko jeśli `invStart + InvSafetyMargin < len(slot.Data)`. Czy ta 12-bajtowa różnica jest świadoma (akceptacja skrajnie krótkich save'ów, w których trailing counters mogą wystawać poza margin), czy historyczna pomyłka — `needs verification`.

---

## Binary / runtime structures

### `InventoryItem` (12 B)

```go
type InventoryItem struct {
    GaItemHandle uint32  // 0x00 — handle do GaMap (lub 0/0xFFFFFFFF = pusty)
    Quantity     uint32  // 0x04 — ilość w stacku (1 dla instance items)
    Index        uint32  // 0x08 — acquisition index per-rekord
}
```

- Pusty slot: `GaItemHandle == GaHandleEmpty (0x00000000)` lub `GaItemHandle == GaHandleInvalid (0xFFFFFFFF)`.
- `Quantity` dla instance-move types (weapon `0x80`, armor `0x90`, talisman `0xA0`, AoW `0xC0`) wynosi zawsze `1`. Dla stackable goods (`0xB0`) trzyma rzeczywistą ilość; górny bit (`& 0x80000000`) historycznie używany w innych slotach gry jako flag — w inventory Elden Ring nie obserwowany; aplikacje czytające qty maskują `& 0x7FFFFFFF` (zob. np. `app.go:364, 378, 389` w `AddItemsToCharacter`).
- `Index` to acquisition counter używany przez game sort "Acquisition Order" — pełna semantyka (stride-2 + parzysta baza + `InvEquipReservedMax`) w [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).

### `EquipInventoryData`

```go
type EquipInventoryData struct {
    CommonItems           []InventoryItem  // długość: CommonItemCount = 2688
    KeyItems              []InventoryItem  // długość: KeyItemCount = 384
    NextEquipIndex        uint32
    NextAcquisitionSortId uint32
    nextEquipIndexOff     int  // absolute byte offset w slot.Data (write-back)
    nextAcqSortIdOff      int  // absolute byte offset w slot.Data (write-back)
}
```

- `nextEquipIndexOff` / `nextAcqSortIdOff` są **prywatne** — przechowują byte offset w `slot.Data` dla in-place write-back. Reconcile przy load i każda mutacja counterów używają tych offsetów. Publiczna metoda `NextEquipIndexOff()` eksponuje pierwszy offset dla testów (`structures.go:135`).
- `Clone()` (`structures.go:138-154`) — deep copy, łącznie z prywatnymi offsetami. Używane przez `SnapshotSlot` (zob. [35 → Transactional safety](35-gaitem-allocator-invariants.md#transactional-safety-and-rollback)).

---

## Container entries

Każdy slot inventory (zarówno common, jak i key) jest fizycznie obecny w binarce — nigdy nie zostaje "skompresowany" przy braku itemu. Pusty slot = 12 B wyzerowanych albo z handle `0xFFFFFFFF`.

| Liczba | Stała | Bajty | Sekcja w slot.Data |
|---|---|---|---|
| 2688 | `CommonItemCount` (`0xA80`) | 32 256 (0x7E00) | `MagicOffset + InvStartFromMagic ... + 0x7E00` |
| 384 | `KeyItemCount` (`0x180`) | 4 608 (0x1200) | po `key_count` header (4 B) |

**Dlaczego common i key są rozdzielone**: gra renderuje je w osobnych zakładkach UI, a niektóre operacje (np. inwentaryzacja key_count header, sortowanie) różnie traktują obie sekcje. Z perspektywy modelu danych obie sekcje używają identycznego rekordu 12 B.

> ℹ️ Liczba rekordów inventory nie jest tożsama z liczbą wpisów `GaItems`. Wpis inventory niesie tylko referencję (handle) — fizyczny obiekt żyje w `GaItems`. Goods (`0xB0`) mogą być reprezentowane przez jeden wpis `GaItems` dzielony między rekordy inventory i storage przez ten sam handle. Pełna analiza relacji warstw — [03 → GaItems vs containers](03-gaitem-map.md#gaitems-vs-containers).

### Header `common_item_count`

Pod offsetem `invStart - 4` (czyli `MagicOffset + InvStartFromMagic - 4`) leży 4-bajtowe pole `common_item_count`. Komentarz w `structures.go:797-803` opisuje rolę headera w grze:

> "External editors may leave this counter stale (er-save-manager increments on add but not on remove; our old code never touched it at all). A stale counter causes the game to use the wrong insertion index, overwriting valid items or incorrectly triggering 'inventory full'."

Save Forge naprawia mismatch przy każdym load przez `ReconcileInventoryHeader` (`writer.go:993-1013`) — przelicza non-empty count w `Inventory.CommonItems` i zapisuje go w binarce. Dokładny mechanizm użycia tej wartości przez grę w aktualnym game patchu (literalne "next insert index" dla pickupów + literalny próg "inventory full") wynika z komentarza w kodzie i obserwacji edytorskich; pełna semantyka runtime — `needs verification`.

> ℹ️ Header dla key items (`key_count`) leży między sekcjami common i key (offset `invStart + CommonItemCount*12`). W obecnym kodzie parser go skipuje (`structures.go:170-172` — `r.ReadU32()` bez przypisania). Save Forge nie reconciluje key_count header. Native-save evidence (T071) potwierdza, że gra sama aktywnie zapisuje ten header — trzyma liczbę fizycznych rekordów KeyItems, inkrementując tylko przy dodaniu genuinely new rekordu, nie przy samej zmianie ilości. `needs verification` pozostaje tylko to, czy write path Save Forge (który obecnie nigdy go nie zapisuje) musi go reconciliować, żeby zostać spójnym z grą.

---

## Relationship to GaItems and GaMap

Inventory entry łączy się z modelem GaItem przez handle (`InventoryItem.GaItemHandle`). Łańcuch lookup:

```
Inventory.CommonItems[i].GaItemHandle  →  slot.GaMap[handle]  →  ItemID
                                                           ↓
                                       slot.GaItems[?]  z handle = ...
                                       (pełne dane: ItemID, AoWGaItemHandle, Unk2/3/5)
```

Wymagane relacje (read-side):

- Każdy non-empty handle typu **weapon (0x80)**, **armor (0x90)** lub **AoW (0xC0)** musi mieć wpis w `slot.GaMap` (egzekwowane przez `ValidatePostMutation` jako `orphan_handle`, [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants)).
- Talizman (`0xA0`) i goods (`0xB0`) — egzekwowanie identyczne (validator akceptuje wszystkie pięć prefixów; konkretny check zawężono w kodzie do `0x80/0x90/0xC0` jako te, których brak w GaMap prowadzi do crashu — pełny opis w [35](35-gaitem-allocator-invariants.md#post-mutation-validation)).
- `slot.GaMap[handle]` nie może zwracać `ItemID == 0` (validator check `gamap_zero_id`).
- Brak duplikatów `Index` w łącznym zbiorze `CommonItems ∪ KeyItems` (validator check `duplicate_index`); naprawa: `RepairDuplicateInventoryIndices` (`backend/core/inventory_index_repair.go`).

> ⚠️ Inventory entry **nie** musi mieć odpowiadającego wpisu `GaItems` dla stackable goods (`0xB0`) ze współdzielonym handle między inventory i storage — to legalna konfiguracja po batch add. Pełna semantyka relacji handle ↔ rekord dla stackable items — [03 → GaItems vs containers](03-gaitem-map.md#gaitems-vs-containers) i [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

---

## Read path

Parser ładuje inventory w dwóch krokach.

### Krok 1 — `EquipInventoryData.Read` (`structures.go:156-195`)

1. Alokuje `CommonItems = make([]InventoryItem, commonCount)` i czyta `commonCount × 12 B` rekordów.
2. Skipuje 4 B key_count header (`r.ReadU32()` bez zapisu).
3. Alokuje `KeyItems = make([]InventoryItem, keyCount)` i czyta `keyCount × 12 B` rekordów.
4. Zapisuje `r.Pos()` jako `nextEquipIndexOff`, czyta `NextEquipIndex`.
5. Zapisuje `r.Pos()` jako `nextAcqSortIdOff`, czyta `NextAcquisitionSortId`.

### Krok 2 — `(*SaveSlot).mapInventory` (`structures.go:757-804`)

Wywoływane raz przy `LoadSave`:

1. `invStart := s.MagicOffset + InvStartFromMagic`.
2. Walidacja `invStart + InvSafetyMargin < len(s.Data)` — gdy slot jest pusty lub uszkodzony, parser pomija inventory.
3. `Inventory.Read(ir, CommonItemCount, KeyItemCount)` ładuje rekordy + countery.
4. **Reconcile `NextAcquisitionSortId`**: znajdź `maxIdx = max(Index)` po `CommonItems` z non-empty handle; jeśli `NextAcquisitionSortId <= maxIdx`, ustaw `NextAcquisitionSortId = maxIdx + 1` (in-memory only).
5. **Reconcile `NextEquipIndex`**: jeśli `NextEquipIndex < NextAcquisitionSortId`, ustaw `NextEquipIndex = NextAcquisitionSortId` **i** wykonaj write-back binarny pod `nextEquipIndexOff` (dla `Version > 0`).
6. `ReconcileInventoryHeader(s)` — przelicza i zapisuje binarne `common_item_count` header pod `invStart - 4`.

Po tych krokach:
- `slot.Inventory.CommonItems` ma 2688 rekordów (fizyczna tablica).
- `slot.Inventory.KeyItems` ma 384 rekordów.
- `nextEquipIndexOff` i `nextAcqSortIdOff` wskazują na pozycje w `slot.Data` używane przez mutacje runtime do write-back.

---

## Write path overview

Write-side semantics dla inventory:

- **Add Items** — pre-flight + snapshot + batch add + reconcile + post-mutation validation: [43-transactional-item-adding](43-transactional-item-adding.md).
- **Remove / repair orphans** — `writer.go::RepairOrphanedGaItems`: zob. [35 → Source of truth](35-gaitem-allocator-invariants.md#source-of-truth-w-kodzie).
- **Reorder (Acquisition Sort)** — `app_inventory_order.go::ReorderInventory` ze stride-2: [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- **Transfer Inventory → Storage** — `transfer.go::MoveItemsBetweenContainers` z equipped guard i rehandle path: [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- **Pojedyncza mutacja countera** — np. `Inventory.NextEquipIndex` lub `NextAcquisitionSortId`: aktualizacja in-memory + write-back pod `nextEquipIndexOff` / `nextAcqSortIdOff`. `common_item_count` header zapisywany przez `ReconcileInventoryHeader`.

---

## Capacity and counters

### Container capacity (Inventory layer)

| Co | Wartość | Stała | Egzekwowane przez |
|---|---|---|---|
| Common items max slots | 2688 | `CommonItemCount` | `Inventory.Read` (alokacja slice'a o tej długości); `ReconcileInventoryHeader` |
| Key items max slots | 384 | `KeyItemCount` | jak wyżej |
| Bytes per record | 12 | `InvRecordLen` | `Inventory.Read`, allocator, transfer |
| Reserved acquisition range | 0..432 | `InvEquipReservedMax` | `app_inventory_order.go` (acquisition stride-2 base) |

### Trailing counters

| Counter | Typ | Lokalizacja | Reconcile przy load? |
|---|---|---|---|
| `common_item_count` header | u32 | `invStart - 4` | tak, `ReconcileInventoryHeader` (binary write-back) |
| `key_count` header | u32 | `invStart + CommonItemCount × 12` | nie (parser Save Forge go skipuje i obecnie go nie reconciluje; T071 dowodzi, że gra zapisuje go jako liczbę fizycznych rekordów KeyItems, zwiększaną tylko przy dodaniu genuinely new rekordu — `needs verification` pozostaje tylko to, czy Save Forge musi go reconciliować we własnym write path, żeby zostać w pełni spójnym z grą) |
| `NextEquipIndex` | u32 | `nextEquipIndexOff` (po KeyItems) | tak, jeśli `< NextAcquisitionSortId` (binary write-back) |
| `NextAcquisitionSortId` | u32 | `nextAcqSortIdOff` (po NextEquipIndex) | tak, w-memory bez write-back (binary aktualizowane przez allocator/reorder) |

### UI capacity gap (callout)

> ⚠️ `INVENTORY used/max` pokazywane w UI (`app_deploy.go::SlotCapacity` → `frontend/src/App.tsx:529-531`) raportuje `non_empty / CommonItemCount` — czyli **kontenerową** pojemność (2688 slotów common). To **nie jest** to samo co realna wolna przestrzeń allocatorowa dla nowych weapon/AoW. Allocator po stronie `slot.GaItems` ma własne limity zone-aware (`NextArmamentIndex`, capacity `len(GaItems)`), które mogą być wyczerpane mimo że Inventory pokazuje 60/2688. Pełne wyjaśnienie i przykład patologiczny: [35 → UI counters vs allocator capacity](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity).

---

## Validation relationships

Read-side warstwa inventory musi spełniać kilka relacji. Pełna lista invariantów i ich enforcement przez `ValidatePostMutation` — [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants). Skrót dla inventory:

- Każdy non-empty handle inventory typu `0x80/0x90/0xC0` musi istnieć w `slot.GaMap` (check `orphan_handle`).
- Żadne dwa wpisy `CommonItems ∪ KeyItems` z non-empty handle nie mogą mieć tego samego `Index` (check `duplicate_index`); naprawa: `RepairDuplicateInventoryIndices`.
- `slot.GaMap[h] != 0` dla każdego handle.
- `common_item_count` header == liczba non-empty rekordów w `CommonItems` (egzekwowane przez `ReconcileInventoryHeader` przy load i po każdej mutacji).
- `NextEquipIndex >= NextAcquisitionSortId` po reconcile (egzekwowane przez `mapInventory`).
- `NextAcquisitionSortId > max(Index)` po reconcile (egzekwowane przez `mapInventory`).

### Mechanika `NextEquipIndex`

Historycznie nazywana "bramką widoczności" — hipoteza, że gra ukrywa wpisy z `Index >= NextEquipIndex` (item istnieje w binarce, ale UI go nie pokazuje). Save Forge wykrywa i naprawia `NextEquipIndex < NextAcquisitionSortId` przy każdym load. Sama mechanika (czy gra **na pewno** ukrywa items z `Index >= NextEquipIndex` w aktualnym game patchu) — `needs verification` przez świeżą weryfikację in-game.

---

## Verified write contracts (native save evidence)

Laboratoria native save ustanawiają następujące kontrakty dla genuinely new
rekordów zapisywanych przez direct Add pipeline
([43-transactional-item-adding](43-transactional-item-adding.md)). To są
zachowania zweryfikowane save'ami, nie cele projektowe — zob.
[43 → Verified native save-write evidence](43-transactional-item-adding.md#verified-native-save-write-evidence)
dla dowodów na poziomie pipeline, z którymi się to łączy, i
[10 → Verified write contracts](10-storage.md#verified-write-contracts-native-save-evidence)
dla kontrastujących reguł po stronie Storage.

- **Wyprowadzenie `Index`**: dla genuinely new rekordu
  `Inventory.CommonItems` `Index` wyprowadza się z `NextAcquisitionSortId` jako
  high-water mark — `Index` nowego rekordu to `mark+1`.
- **`NextEquipIndex` (T050/T210)**: genuinely new rekord
  `Inventory.CommonItems` zwiększa `NextEquipIndex` o **dokładnie jeden**. To
  lokalny krok per-insert; `NextEquipIndex` nigdy nie może być reconciled do
  `NextAcquisitionSortId` ani ustawiony na `Index` nowego rekordu.
- **`Inventory.KeyItems` (T070/T071)**: KeyItems dzielą
  `Inventory.NextAcquisitionSortId` z CommonItems, ale dodanie Key Item **nie**
  zwiększa `NextEquipIndex` — kontrakt `+1` z CommonItems powyżej nie jest
  dowodem dla KeyItems. Cookbooki są routowane przez tę ścieżkę KeyItems
  (T071).

Te kontrakty pokrywają wyłącznie przypadek "genuinely new record" testowany
przez cytowane testy. Zachowanie tutorial, EventFlag, Info Item i
crafting-flag wyzwalane razem z tymi itemami jest tu poza zakresem.

---

## Examples

### Stackable goods w common inventory

Save zawiera w `Inventory.CommonItems[42]`:

```
GaItemHandle = 0xB0001234       ← goods, prefix 0xB0
Quantity     = 50               ← ilość w stacku
Index        = 312              ← acquisition index
```

Lookup:

```
slot.GaMap[0xB0001234] = 0x40001234   ← goods ItemID (prefix 0x4 po HandleToItemID)
slot.GaItems[?].Handle = 0xB0001234   ← 8-bajtowy rekord {Handle, ItemID}
db.GetItemDataFuzzy(0x40001234)       → np. Smithing Stone [3]
```

Inventory.CommonItems[42] i (opcjonalnie) Storage.CommonItems[X] mogą mieć ten sam handle — wtedy obie strony dzielą fizyczny stack przez ten sam rekord GaMap. Operacje quantity-merge na transferze: [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

### Instance weapon

```
Inventory.CommonItems[5].GaItemHandle = 0x80000003
Inventory.CommonItems[5].Quantity     = 1
Inventory.CommonItems[5].Index        = 15

slot.GaMap[0x80000003] = 0x000F4240        ← Uchigatana ItemID
slot.GaItems[3] = GaItemFull{
    Handle:          0x80000003,
    ItemID:          0x000F4240,
    Unk2:            -1,
    Unk3:            -1,
    AoWGaItemHandle: 0x00000000,             ← canonical sentinel
    Unk5:            0,
}
```

---

## Known limits / needs verification

- **Mechanika `NextEquipIndex` jako "bramki widoczności"** — kod reconciluje wartość przy każdym load, ale efekt na grze (czy items z `Index >= NextEquipIndex` są na pewno ukryte w aktualnym game patchu) — `needs verification`.
- **`key_count` header (4 B między common a key)** — parser go skipuje, Save Forge nie reconciluje. T071 potwierdza, że gra go zapisuje (trzyma liczbę fizycznych rekordów KeyItems, inkrementując tylko przy genuinely new rekordzie); czy write path Save Forge musi go reconciliować — `needs verification`.
- **Górny bit `Quantity` (`& 0x80000000`)** — aplikacja maskuje przez `& 0x7FFFFFFF` w wielu miejscach (`app.go:364, 378, 389`), co sugeruje, że gra używa tego bitu jako flag w niektórych slotach. Konkretne znaczenie — `needs verification`.
- **`InvSafetyMargin = 0x9000`** vs faktyczny rozmiar sekcji `0x900C` — 12-bajtowa różnica jest świadomym zaokrągleniem progu walidacji, ale **nie** rezerwą — `needs verification`, czy ten margin nigdy nie powoduje skipowania legalnego inventory dla skrajnych save'ów.

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — model GaItem/GaMap, prefiksy handle, relacja inventory → GaMap → GaItems.
- [10-storage](10-storage.md) — Storage data model, różnice względem Inventory.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator, capacity, counters, validation, snapshot/rollback, UI capacity gap.
- [43-transactional-item-adding](43-transactional-item-adding.md) — pełna architektura Add Items (pre-flight → snapshot → mutate → reconcile → validate).
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — `acquisition_index` w `InventoryItem.Index`: jak gra sortuje "Acquisition Order" i dlaczego stride-2.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — Inventory ↔ Storage transfer (instance-move vs quantity-merge, rehandle, equipped guard, cap-aware partial).
- [54-ash-of-war](54-ash-of-war.md) — semantyka custom AoW pinned do broni przechowywanej w inventory.

---

## Sources

- `backend/core/structures.go` — `EquipInventoryData`, `InventoryItem`, `(*EquipInventoryData).Read`, `(*SaveSlot).mapInventory`
- `backend/core/offset_defs.go` — `InvStartFromMagic`, `CommonItemCount`, `KeyItemCount`, `InvRecordLen`, `InvKeyCountHeader`, `InvSafetyMargin`, `InvEquipReservedMax`
- `backend/core/writer.go` — `ReconcileInventoryHeader`, `RepairOrphanedGaItems`
- `backend/core/diagnostics.go` — `ValidatePostMutation` (orphan_handle, duplicate_index, gamap_zero_id, gaitem_indices)
- `app.go::AddItemsToCharacter` — top-level orchestrator (pre-flight → snapshot → batch → reconcile → validate)
- `app_deploy.go::SlotCapacity`, `GetSlotCapacity` — UI binding (`non_empty / max`)
- Round-trip fixtures w `tests/roundtrip_test.go`, `tests/save_modify_test.go`, `tests/capacity_test.go`
