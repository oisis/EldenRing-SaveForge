# 10 — Storage Box: data model and read path

> **Typ**: Binary format spec
> **Status**: ✅ canonical, implemented dla read-side i write-side core. Storage Apply (UI flow zapisu kolejności po stronie Storage) — wciąż `needs verification` przez świeży test in-game / Steam Deck. Allocator/capacity/counter invariants opisane są w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md). Transakcyjne Add Items — w [43-transactional-item-adding](43-transactional-item-adding.md).
> **Zakres**: Read-side binary model `slot.Storage` (common items + key items), header count, trailing counters, relacja do GaItems/GaMap, różnice względem Inventory. Pełna semantyka transferu Inventory ↔ Storage (instance-move, quantity-merge, rehandle, equipped guard) jest w [53-inventory-storage-transfer](53-inventory-storage-transfer.md).

---

## Cel rozdziału

Rozdział opisuje:

- gdzie Storage Box siedzi w pliku save (offset dynamiczny — `slot.StorageBoxOffset`);
- jak zbudowana jest tablica `slot.Storage` (1920 common + 128 key);
- format pojedynczego rekordu (12 B — identyczny z Inventory);
- header count na początku sekcji + trailing counters na końcu;
- co parser robi przy load i jakie reconciliations wykonuje;
- jak storage entry odwołuje się do warstw GaMap / GaItems;
- czym Storage różni się od Inventory pod względem layoutu i load path;
- które invarianty muszą obowiązywać po stronie read.

Co rozdział **NIE** robi:

- Nie opisuje pełnego transferu Inventory ↔ Storage (rehandle path, cap-aware partial merge, equipped guard) — to w [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- Nie opisuje stride-2 algorytmu Sort Order / Storage reorder — to w [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- Nie opisuje architektury Add Items (Storage Qty branch w `AddItemsToCharacter`) — to w [43-transactional-item-adding](43-transactional-item-adding.md).
- Nie opisuje allocator side (`allocateGaItem`, capacity, validation, snapshot/rollback) — to w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).

---

## Status

- canonical (read-side + write-side core)
- implemented (parser w `mapInventory` storage branch, transfer w `MoveItemsBetweenContainers`, reorder w `ReorderStorage`)
- source-of-truth: kod backend
- **Storage Apply in-game verification**: `needs verification` (zob. sekcja "Known limits")
- allocator details: **chapter 35**

---

## Source of truth w kodzie

| Temat | Pliki / funkcje | Notatka |
|---|---|---|
| Struktura storage | `backend/core/structures.go:227-228` (`SaveSlot.Storage EquipInventoryData`) | ten sam typ co Inventory, inne rozmiary i inny layout headera |
| Pojedynczy rekord | `backend/core/structures.go:118-122` (`InventoryItem`) | `GaItemHandle`, `Quantity`, `Index` — identyczny 12 B z Inventory |
| Stałe layoutu | `backend/core/offset_defs.go:66-78` | `StorageItemCount = 2048`, `StorageCommonCount = 0x780 (1920)`, `StorageKeyCount = 0x80 (128)`, `StorageHeaderSkip = 4`, `StorageSafetyMarg = 0x6000`, `StorageNextEquipIdxRel`, `StorageNextAcqSortRel` |
| Parser kontenera | `backend/core/structures.go:725-755` (`(*EquipInventoryData).ReadStorage`) | czyta `count` rekordów liniowo, **skipuje empty entries** i zwraca tylko niepuste w `CommonItems` |
| Top-level load | `backend/core/structures.go:805-825` (`mapInventory` — storage branch) | wywołuje `ReadStorage(StorageItemCount)`, czyta trailing counters z fixed offsets, zapamiętuje offsety do write-back |
| Reconcile header | `backend/core/writer.go:971-991` (`ReconcileStorageHeader`) | przelicza non-empty count w binarce i zapisuje pod `slot.StorageBoxOffset` |
| Storage offset (dynamic) | `backend/core/structures.go::SaveSlot.StorageBoxOffset` (linia 239) | wyliczany w chain offsets, nie jest relative do `MagicOffset` |
| UI capacity binding | `app_deploy.go:31-38, 289-306` (`SlotCapacity`, `GetSlotCapacity`) | eksponuje `non_empty / StorageCommonCount` (1920) |

---

## Mental model

Storage Box to drugi kontener referencji do GaItems, dostępny w grze przy Site of Grace. Z perspektywy modelu danych dzieli z Inventory:

- ten sam typ rekordu (`InventoryItem`, 12 B);
- ten sam Go typ kontenera (`EquipInventoryData`);
- ten sam pattern trailing counters (`NextEquipIndex`, `NextAcquisitionSortId`);
- te same invarianty (orphan handle, duplicate index — patrz [35](35-gaitem-allocator-invariants.md#required-invariants)).

Różnice względem Inventory:

| Aspekt | Inventory | Storage |
|---|---|---|
| Offset bazowy | `slot.MagicOffset + InvStartFromMagic` (relative) | `slot.StorageBoxOffset` (**dynamic**) |
| Header count | `invStart − 4` (przed pierwszym slotem) | `slot.StorageBoxOffset` (sama wartość offset; rekordy zaczynają się od `+ StorageHeaderSkip = +4`) |
| Common slots | 2688 (`CommonItemCount`) | 1920 (`StorageCommonCount`) |
| Key slots | 384 (`KeyItemCount`) | 128 (`StorageKeyCount`) |
| Parser | `(*EquipInventoryData).Read` — alokuje `[]InventoryItem` o stałej długości | `(*EquipInventoryData).ReadStorage` — filtruje puste, zwraca slice **tylko niepustych** |
| Reconcile counters przy load | tak (NextAcqSortId, NextEquipIndex) | nie reconciluje w `mapInventory`, tylko zapamiętuje offsety + czyta wartości |
| Reconcile header przy load | tak (`ReconcileInventoryHeader`) | nie przy load; tak po batch add (`AddItemsToCharacter` → `ReconcileStorageHeader`) |

```
slot.Data
 │
 ├── [slot.StorageBoxOffset]              ┐
 │       storage_count (u32)              │  4 B header
 │       (liczba non-empty w common;      │
 │        zapisywane przez ReconcileStorageHeader)
 │
 ├── [+ StorageHeaderSkip = +4]            ┘
 │       Common slots × 1920                  23 040 B  (0x5A00)
 │       — 12 B per record
 │
 ├── (po common)                          ┐
 │       key_count header (u32)           │  4 B
 │
 ├── (next)                                ┘
 │       Key slots × 128                       1 536 B  (0x0600)
 │       — 12 B per record
 │
 ├── (po key)                              ┐
 │       NextEquipIndex (u32)              │  4 B  (offset = StorageBoxOffset + StorageHeaderSkip + StorageNextEquipIdxRel)
 │       NextAcquisitionSortId (u32)       │  4 B  (offset = ... + StorageNextAcqSortRel)
 └── ───────────────────────────────────────┘
```

Łączny rozmiar sekcji (od `StorageBoxOffset`, łącznie z headerem):

```
storage_count hdr =                  4  (0x0004)
Common slots      = 1920 × 12 = 23 040  (0x5A00)
key_count hdr     =                  4  (0x0004)
Key slots         =  128 × 12 =  1 536  (0x0600)
NextEquipIdx      =                  4  (0x0004)
NextAcqSortId     =                  4  (0x0004)
─────────────────────────────────────
TOTAL                          = 24 592  (0x6010)
```

`StorageSafetyMarg = 0x6000` (`offset_defs.go:71`) jest progiem walidacji — parser woła `ReadStorage` tylko jeśli `storageStart + StorageSafetyMarg < len(slot.Data)`.

---

## Binary / runtime structures

### `InventoryItem` (12 B) — identyczne z Inventory

```go
type InventoryItem struct {
    GaItemHandle uint32  // 0x00 — handle do GaMap (lub 0/0xFFFFFFFF = pusty)
    Quantity     uint32  // 0x04 — ilość w stacku (1 dla instance items)
    Index        uint32  // 0x08 — acquisition index per-rekord
}
```

Pełny opis tego rekordu — [07 → Binary / runtime structures](07-inventory.md#binary--runtime-structures). Storage używa identycznego layoutu.

### `EquipInventoryData` po `ReadStorage`

W przeciwieństwie do Inventory, gdzie `CommonItems` ma fizycznie 2688 wpisów (z empty entries jako wyzerowane rekordy), **storage `CommonItems` zawiera tylko niepuste wpisy** — `ReadStorage` filtruje empty handle entries i appenduje pozostałe (`structures.go:741-751`):

```go
// Skip empty/invalid entries but continue reading — storage can have sparse gaps
// after item removal. Breaking on first empty would lose items after the gap.
if handle == GaHandleEmpty || handle == GaHandleInvalid {
    continue
}
e.CommonItems = append(e.CommonItems, InventoryItem{...})
```

Konsekwencje:

- `ReadStorage` jest wołane z `count = StorageItemCount = 2048` (`structures.go:810`), nie `StorageCommonCount = 1920`. Górną granicą `len(slot.Storage.CommonItems)` jest więc **2048** (a nie 1920) — wszystkie iteracje, które nie trafiają w empty handle, są appendowane. W praktyce niepuste handle w obszarze 1920–2048 to fałszywe odczyty wynikające z overshootu (zob. callout "Niespójność `StorageItemCount` vs `StorageCommonCount`" niżej oraz Known limits).
- `ReconcileStorageHeader` (`writer.go:971-991`) operuje wyłącznie na pierwszych `StorageCommonCount = 1920` slotach binarki — czyli pisze do headera liczbę non-empty w prawidłowej common section, ignorując overshoot reader'a.
- Storage `KeyItems` jest zawsze inicjalizowane jako pusty slice (`structures.go:753` — `e.KeyItems = []InventoryItem{}`). **Save Forge w aktualnym kodzie nie eksponuje storage key items w runtime modelu** — wszystkie operacje (transfer, reorder, capacity) operują na storage common only. `needs verification`, czy gra używa storage key items aktywnie.
- Index w `CommonItems` nie odpowiada pozycji binarnej — to indeks w "ścieśnionym" slice po skipie empty. Pozycja binarna może być rekonstruowana tylko przez ponowny skan `slot.Data`.

### `StorageBoxOffset`

`slot.StorageBoxOffset` jest **dynamicznym offsetem** wyliczanym w chain offsets (`structures.go:239`). W odróżnieniu od inventory, który jest na stałej pozycji `MagicOffset + 505`, storage może leżeć w innym miejscu w zależności od długości poprzednich sekcji (np. world geometry, event flags). Pełny chain — poza zakresem tego rozdziału (zob. `structures.go:331-...` dla detali offset chain resolution).

---

## Container entries

| Liczba | Stała | Bajty | Sekcja w slot.Data |
|---|---|---|---|
| 1920 | `StorageCommonCount` (`0x780`) | 23 040 (0x5A00) | `StorageBoxOffset + 4 ... + 0x5A04` |
| 128 | `StorageKeyCount` (`0x80`) | 1 536 (0x0600) | po `key_count` header (4 B) |

> ℹ️ **Niespójność `StorageItemCount` vs `StorageCommonCount`**: stała `StorageItemCount = 2048` (`offset_defs.go:66`) jest używana jako `count` argument w wywołaniu `ReadStorage` (`structures.go:810`), podczas gdy `StorageCommonCount = 1920` jest fizyczną długością common slots. Różnica 128 odpowiada `StorageKeyCount`. ReadStorage czyta liniowo `count × 12 = 24576` B (overshooting common section o 1536 B), filtrując empty entries po drodze. To może czytać przez `key_count` header (4 B na offset `StorageCommonCount × 12 = 23040`) i wpadać w sekcję key items jako "kontynuacja common". W praktyce niepuste handle z tego obszaru są albo nieobecne, albo zinterpretowane jako common entries. **Status: `needs verification`** — czy ten overshoot jest świadomy (akceptacja key items jako pseudo-common przy load), czy historyczna pomyłka, która działa "by accident" bo storage rzadko bywa pełny.

---

## Relationship to GaItems and GaMap

Identyczna jak dla Inventory (zob. [07 → Relationship to GaItems and GaMap](07-inventory.md#relationship-to-gaitems-and-gamap)). Skrót:

- Storage entry niesie tylko `GaItemHandle`; lookup `slot.GaMap[handle] → ItemID` i `slot.GaItems[?]` z pełnymi metadanymi.
- Storage **dzieli przestrzeń handle** z Inventory — ten sam handle może występować w obu kontenerach (legalne dla stackable goods `0xB0` po batch add lub transferze).
- Walidacja `orphan_handle` w `ValidatePostMutation` obejmuje **Inventory** entries (`diagnostics.go:373-389`); storage entries nie są bezpośrednio sprawdzane przez ten check w aktualnym kodzie — `needs verification`, czy storage orphans są wykrywane przez inny mechanizm (np. `RepairOrphanedGaItems` skanuje oba kontenery).

> ⚠️ Pełna semantyka **rehandle** przy transferze duplicate-handle (talisman, broń, zbroja) — [53-inventory-storage-transfer](53-inventory-storage-transfer.md). W skrócie: instance-move handle (`0x80/0x90/0xA0/0xC0`) istniejący już po stronie docelowej powoduje alokację nowego unikalnego handle dla przeniesionej instancji przez `generateUniqueHandle` ([35](35-gaitem-allocator-invariants.md#handle-generation)) — żeby zachować separację GaItem entries między kontenerami.

---

## Read path

Parser ładuje storage w dwóch krokach.

### Krok 1 — `EquipInventoryData.ReadStorage` (`structures.go:725-755`)

1. Inicjalizuje `CommonItems = []InventoryItem{}` (pusty slice).
2. Iteruje `count` rekordów liniowo (w aktualnym kodzie `count = StorageItemCount = 2048`):
   - Czyta `handle`, `quantity`, `index` (każdy u32).
   - Jeśli `handle ∈ {GaHandleEmpty, GaHandleInvalid}` — `continue` (skip empty, ale czyta dalej).
   - W przeciwnym razie — `append` do `CommonItems`.
3. Inicjalizuje `KeyItems = []InventoryItem{}` (zawsze pusty; storage key items nieobsługiwane runtime).

### Krok 2 — `(*SaveSlot).mapInventory` storage branch (`structures.go:805-824`)

Wywoływane raz przy `LoadSave`, po inventory branchu:

1. `storageStart := s.StorageBoxOffset + StorageHeaderSkip`.
2. Walidacja `storageStart + StorageSafetyMarg < len(s.Data)` — gdy slot jest pusty lub uszkodzony, parser pomija storage.
3. `Storage.ReadStorage(sr, StorageItemCount)` (czytanie liniowe z filtracją empty).
4. Czyta trailing counters z **fixed relative offsets** (`StorageNextEquipIdxRel`, `StorageNextAcqSortRel`):
   - `s.Storage.nextEquipIndexOff = storageNextEquipOff`
   - `s.Storage.NextEquipIndex = binary.LittleEndian.Uint32(s.Data[storageNextEquipOff:])`
   - `s.Storage.nextAcqSortIdOff = storageNextAcqOff`
   - `s.Storage.NextAcquisitionSortId = binary.LittleEndian.Uint32(s.Data[storageNextAcqOff:])`

Po tych krokach:

- `slot.Storage.CommonItems` ma 0..1920 wpisów (tylko niepuste; pozycja binarna niezachowana).
- `slot.Storage.KeyItems` jest pustym slice.
- `slot.Storage.nextEquipIndexOff` i `nextAcqSortIdOff` wskazują na pozycje w `slot.Data` używane przez mutacje runtime do write-back.

> ℹ️ W odróżnieniu od inventory branchu, **storage branch nie wykonuje reconcile counterów ani headera przy load**. Reconcile headera (`ReconcileStorageHeader`) jest wołany przez `AddItemsToCharacter` po batch add (`app.go:622-623`) i po batch transferze (`transfer.go::MoveItemsBetweenContainers:128`). Reconcile counterów po stronie storage — `needs verification`, czy w ogóle jest robione (Inventory ma to w `mapInventory`; Storage prawdopodobnie nie).

---

## Write path overview

Write-side semantics dla storage:

- **Add Items (Storage Qty branch)** — pre-flight + snapshot + batch add (z `ItemToAdd.StorageQty > 0`) + `ReconcileStorageHeader` + post-mutation validation: [43-transactional-item-adding](43-transactional-item-adding.md).
- **Transfer Inventory ↔ Storage** — `transfer.go::MoveItemsBetweenContainers` z dwukierunkowym instance-move/quantity-merge, equipped guard, rehandle path: [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- **Reorder Storage** — `app_inventory_order.go::ReorderStorage` ze stride-2: [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- **Reconcile storage_count header** — `writer.go::ReconcileStorageHeader` (przelicza non-empty count w common slots i zapisuje pod `slot.StorageBoxOffset`).

---

## Capacity and counters

### Container capacity (Storage layer)

| Co | Wartość | Stała | Egzekwowane przez |
|---|---|---|---|
| Common items max slots | 1920 | `StorageCommonCount` | `ReconcileStorageHeader` (binary write-back); transfer scan dst slots |
| Key items max slots | 128 | `StorageKeyCount` | layout offsets (`StorageNextEquipIdxRel`); runtime model nie eksponuje |
| Read count (linear scan) | 2048 | `StorageItemCount` | `ReadStorage` argument; overshoot względem common — patrz "Container entries" callout |
| Bytes per record | 12 | `InvRecordLen` | `ReadStorage`, transfer, allocator |
| Safety margin | 0x6000 | `StorageSafetyMarg` | `mapInventory` walidacja |

### Trailing counters

| Counter | Typ | Lokalizacja | Reconcile przy load? |
|---|---|---|---|
| `storage_count` header | u32 | `slot.StorageBoxOffset` (pierwsza wartość) | nie przy load; tak po batch add / transfer (`ReconcileStorageHeader`) |
| `key_count` header | u32 | `storageStart + StorageCommonCount × 12` | nie (parser nie czyta bezpośrednio; overshoot w `ReadStorage` może go obejść) |
| `NextEquipIndex` | u32 | `storageStart + StorageNextEquipIdxRel` | nie (storage nie ma reconcile w `mapInventory`) |
| `NextAcquisitionSortId` | u32 | `storageStart + StorageNextAcqSortRel` | nie |

### UI capacity gap (callout)

> ⚠️ `STORAGE used/max` w UI (`app_deploy.go::SlotCapacity` → `frontend/src/App.tsx:531`) raportuje `non_empty / StorageCommonCount` — czyli **kontenerową** pojemność (1920 slotów common). To **nie jest** to samo co realna wolna przestrzeń allocatorowa dla nowych weapon/AoW po stronie `slot.GaItems`. Container capacity (1920 slots) i GaItems allocator capacity (5118/5120 z zone-aware kursorami) to **różne wymiary**. Pełne wyjaśnienie i przykład patologiczny: [35 → UI counters vs allocator capacity](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity).

---

## Validation relationships

Pełna lista invariantów i ich enforcement przez `ValidatePostMutation` — [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants). Skrót dla storage:

- `storage_count` header == liczba non-empty rekordów w `Storage.CommonItems` po stronie binarki (check `storage_count` w validatorze, `diagnostics.go:434-465`); naprawa: `ReconcileStorageHeader`.
- Brak orphan GaItem entries po stronie storage — naprawa: `RepairOrphanedGaItems` (skanuje **oba** kontenery: inventory i storage).
- Validator check `duplicate_index` w aktualnym kodzie skanuje **tylko** `Inventory.CommonItems ∪ KeyItems` (`diagnostics.go:392-419`) — **nie** sprawdza duplikatów Index w `Storage.CommonItems`. Czy gra wymaga unikalności Index w storage tak samo jak w inventory — `needs verification`.

---

## Examples

### Storage entry → handle → GaMap → ItemID

```
slot.Storage.CommonItems[7]:
    GaItemHandle = 0xA0000005    ← talisman handle, prefix 0xA0
    Quantity     = 1
    Index        = 250

slot.GaMap[0xA0000005]   = 0x20000005    ← talisman ItemID (po HandleToItemID = (handle & 0x0FFFFFFF) | 0x20000000)
slot.GaItems[?].Handle   = 0xA0000005    ← 8-bajtowy rekord {Handle, ItemID}

db.GetItemDataFuzzy(0x20000005)          → np. dowolny talizman z DB (konkretne ItemIDy w backend/db/data/talismans.go)
```

### Storage entry vs Inventory entry — duplicate goods

Stackable goods (`0xB0`) mogą być **fizycznie obecne w obu** kontenerach z **tym samym handle** (legalna konfiguracja po batch add z `invQty > 0 && storageQty > 0`):

```
slot.Inventory.CommonItems[42]:
    GaItemHandle = 0xB0001234
    Quantity     = 50

slot.Storage.CommonItems[3]:
    GaItemHandle = 0xB0001234        ← TEN SAM handle
    Quantity     = 99
```

Obie strony używają tego samego `GaItems[?]` przez wspólny handle. Transfer quantity-merge ([53](53-inventory-storage-transfer.md)) operuje na obu rekordach niezależnie, ale handle zostaje współdzielony. W przeciwieństwie do instance-move (broń/zbroja/talizman/AoW), gdzie duplicate handle przy transferze powoduje rehandle (alokacja nowego handle dla przeniesionej instancji).

---

## Verified write contracts (native save evidence)

Laboratoria native save ustanawiają następujące kontrakty dla genuinely
**pustego** Storage — bez rekordów `Storage.CommonItems`, świeżej sygnatury
(`NextAcquisitionSortId<=1`, `NextEquipIndex==0`). Zob.
[43 → Verified native save-write evidence](43-transactional-item-adding.md#verified-native-save-write-evidence)
dla dowodów na poziomie pipeline (App-lifecycle), z którymi się to łączy, i
[07 → Verified write contracts](07-inventory.md#verified-write-contracts-native-save-evidence)
dla kontrastujących reguł po stronie Inventory.

- **T310**: pierwszy rekord direct-add do tego pustego Storage inicjalizuje
  się z `Index=2`, `NextAcquisitionSortId=2`, `NextEquipIndex=128`.
- **T330**: jeden natywny batch sześciu rekordów z dokładnie tego samego
  stanu startowego używa indeksów `2, 4, 6, 8, 10, 12` i kończy przy
  `NextEquipIndex=133`, `NextAcquisitionSortId=7` (`128+5`, `2+5`). Tylko w
  batchu, który wystartował z sygnaturą T310, każdy rekord — nie tylko
  pierwszy — zwiększa oba countery o dokładnie 1.
- **T352**: ten kontekst pustego startu obowiązuje też między **osobnymi**
  direct Database Add calls (wieloma wywołaniami `AddItemsToCharacter`, nie
  jednym batchem) wykonanymi w jednej nieprzerwanej sesji edytora — nie
  tylko wewnątrz jednego batcha. Mechanizm, który przenosi ten kontekst
  między wywołaniami (explicit stan `App`, resetowany przy save/load/close),
  jest opisany w
  [43 → Verified native save-write evidence](43-transactional-item-adding.md#verified-native-save-write-evidence);
  ten rozdział podaje wyłącznie wartości counterów, które ten kontekst
  zachowuje.

**Zachowanie counterów już zapopulowanego Storage nie jest ustanowione przez
T310/T330/T352.** Aktualny `default` branch writer'a (w switchu counterów
Storage w `addToInventory`) — który zostawia `NextEquipIndex` nietknięty i
zwiększa `NextAcquisitionSortId` jako high-water mark — to istniejący
fallback, niemodyfikowany przez ten dowód. **Nie** jest tu stwierdzany jako
zweryfikowany kontrakt native-save; zmiana go wymagałaby osobnego dowodu
native-save i testu regresyjnego.

---

## Known limits / needs verification

- **Storage Apply (UI flow Reorder Storage)** — sanity check in-game na Steam Deck nie jest świeżo potwierdzony w obecnym branchu. Backend (`ReorderStorage` w `app_inventory_order.go`, stride-2 algorytm) jest pokryty testami (`app_storage_order_test.go`), ale weryfikacja "Acquisition Order ↑" w skrzyni w grze odpowiada preview edytora dla każdej zakładki Sort Order — `needs verification` (zob. też [53 → Known limits](53-inventory-storage-transfer.md#g-znane-ograniczenia--future-work)).
- **`StorageItemCount = 2048` vs `StorageCommonCount = 1920` overshoot** — `ReadStorage` czyta 2048 rekordów liniowo, czyli przechodzi przez 4-bajtowy `key_count` header (na offset = 1920 × 12 = 23040) i może wpadać w sekcję key items. W praktyce empty filter usuwa większość fałszywych odczytów, ale formalna semantyka — `needs verification`. Możliwe że to historyczne, działa "by accident" przy typowych save'ach.
- **Storage key items** — `ReadStorage` zawsze inicjalizuje `KeyItems = []InventoryItem{}` (pusty slice). Save Forge runtime model **nie eksponuje** storage key items dla transferu, reorderu ani UI. Czy gra używa storage key items aktywnie — `needs verification`.
- **`duplicate_index` validator dla storage** — w aktualnym kodzie validator sprawdza tylko `Inventory.CommonItems ∪ KeyItems` (zob. [35 → Required invariants I5](35-gaitem-allocator-invariants.md#required-invariants)). Czy gra wymaga unikalności `Index` w storage — `needs verification`.
- **Reconcile counterów Storage przy load** — `mapInventory` storage branch nie wykonuje reconcile `NextAcquisitionSortId`/`NextEquipIndex` (w odróżnieniu od Inventory). Wartości z save'ów edytowanych zewnętrznymi narzędziami mogą być stale aż do pierwszej mutacji storage. `needs verification`, czy to powoduje observable issues w grze.

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — model GaItem/GaMap, prefiksy handle, relacja storage → GaMap → GaItems.
- [07-inventory](07-inventory.md) — Inventory data model (`InventoryItem` 12 B rekord, identyczny z Storage; pełne porównanie różnic Inventory vs Storage w "Mental model" tego rozdziału).
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator, capacity, counters, validation, snapshot/rollback, UI capacity gap.
- [43-transactional-item-adding](43-transactional-item-adding.md) — pełna architektura Add Items (Storage Qty branch jest częścią tej samej pipelinie).
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — `acquisition_index` w `InventoryItem.Index`; `ReorderStorage` używa identycznego algorytmu jak `ReorderInventory`.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — pełna semantyka transferu Inventory ↔ Storage (instance-move vs quantity-merge, rehandle, equipped guard, cap-aware partial, dwukolumnowy Sort Order UI).
- [54-ash-of-war](54-ash-of-war.md) — semantyka custom AoW przy transferze broni z AoW handle do/ze storage.

---

## Sources

- `backend/core/structures.go` — `EquipInventoryData`, `InventoryItem`, `(*EquipInventoryData).ReadStorage`, `(*SaveSlot).mapInventory` (storage branch)
- `backend/core/offset_defs.go` — `StorageItemCount`, `StorageCommonCount`, `StorageKeyCount`, `StorageHeaderSkip`, `StorageSafetyMarg`, `StorageNextEquipIdxRel`, `StorageNextAcqSortRel`, `InvRecordLen`, `InvKeyCountHeader`
- `backend/core/writer.go` — `ReconcileStorageHeader`, `RepairOrphanedGaItems`
- `backend/core/transfer.go` — `MoveItemsBetweenContainers` (storage-aware transfer + rehandle)
- `backend/core/diagnostics.go` — `ValidatePostMutation` (storage_count check)
- `app.go::AddItemsToCharacter` (Storage Qty branch), `app.go::MoveItemsBetweenInventoryAndStorage`
- `app_inventory_order.go::ReorderStorage`, `GetStorageOrder`
- `app_deploy.go::SlotCapacity`, `GetSlotCapacity`
- Tests: `tests/transfer_test.go`, `app_storage_order_test.go`
