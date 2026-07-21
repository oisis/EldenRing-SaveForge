# 43 — Transactional Item Adding: current write path

> **Typ**: Design doc + binary write-path spec
> **Status**: ✅ canonical, implemented. Architektura potwierdzona przeciw aktualnemu backendowi (`app.go::AddItemsToCharacter`, `backend/core/writer.go::AddItemsToSlotBatch`, `backend/core/snapshot.go`, `backend/core/diagnostics.go::ValidatePostMutation`).
> **Zakres**: Aktualny write-path dodawania przedmiotów do slotu (PRE-FLIGHT → SNAPSHOT → MUTATE → POST-FLAGS → RECONCILE → VALIDATE → ROLLBACK-ON-FAILURE). Rozdział pokazuje, jak warstwa aplikacji (`app.go`) orkiestruje allocator, container writes, companion flags i validator w jedną transakcyjną operację. Allocator/capacity/counter invariants są opisane canonicalnie w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md); pełna lista item companion flags w [50-item-companion-flags](50-item-companion-flags.md).

---

## Cel rozdziału

Rozdział opisuje:

- gdzie zaczyna się pipeline Add Items w warstwie aplikacji i co orkiestruje;
- jak działa pre-flight (capacity check, duplicate-index scan) — i dlaczego musi failować **przed** mutacją;
- jak `AddItemsToSlotBatch` różnicuje stackable goods/talismans od instance-backed weapon/armor/AoW;
- gdzie i kiedy używany jest allocator (`generateUniqueHandle`, `allocateGaItem`, `upsertGaItemData`);
- jak działa snapshot/rollback (`SnapshotSlot`/`RestoreSlot`) i czym różni się od user-facing undo stack;
- jak validator (`ValidatePostMutation`) zamyka transakcję i kiedy wymusza rollback;
- jakie companion flags są ustawiane wraz z itemami i jak są bezpieczne transakcyjnie (best-effort logging vs hard rollback);
- co jest poza zakresem Add Items.

Co rozdział **NIE** robi:

- Nie powiela binary modelu GaItem — [03-gaitem-map](03-gaitem-map.md).
- Nie powiela allocator/counter/capacity invariantów — [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
- Nie powiela Inventory/Storage read modelu — [07-inventory](07-inventory.md), [10-storage](10-storage.md).
- Nie opisuje semantyki transferu Inventory ↔ Storage — [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- Nie opisuje stride-2 acquisition sort — [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- Nie opisuje pełnej semantyki Ash of War (sentinele, strict apply, availability) — [54-ash-of-war](54-ash-of-war.md).
- Nie opisuje pełnej taksonomii companion flags — [50-item-companion-flags](50-item-companion-flags.md).

---

## Status

- canonical
- implemented (testy capacity, batch rebuild, post-validation, rollback)
- source-of-truth: kod backend + `app.go`
- allocator details: **chapter 35**
- companion flags taxonomy: **chapter 50**

---

## Source of truth w kodzie

| Temat | Pliki / funkcje | Notatka |
|---|---|---|
| App-level entry point | `app.go:320-644` (`AddItemsToCharacter`) | orkiestrator pełnej transakcji; args: `charIdx, itemIDs, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty` |
| Capacity report typ | `backend/core/capacity.go:71-83` (`CapacityReport`) | pola: `CanFitAll`, `CapHit`, `Free*`, `Needed*` |
| Capacity pre-flight | `backend/core/capacity.go:85-222` (`CheckAddCapacity`, `CountSlotUsage`) | all-or-nothing, używany przed mutacją |
| Duplicate-index pre-flight | `backend/core/diagnostics.go::ScanDuplicateInventoryIndices` + `app.go:344-350` | reject przy istniejących duplikatach Index w inventory |
| Snapshot/rollback | `backend/core/snapshot.go:28-102` (`SnapshotSlot`, `RestoreSlot`, `SlotSnapshot`) | deep copy: `Data`, `GaItems`, `GaMap`, `Inventory`, `Storage`, wszystkie countery + offsety dynamiczne |
| Batch add | `backend/core/writer.go:273-376` (`AddItemsToSlotBatch`) | 3 fazy: alokacja GaItems, jeden `RebuildSlotFull`, write do `Inventory`/`Storage` |
| Allocator | `backend/core/writer.go:402-501` (`generateUniqueHandle`, `allocateGaItem`) | full opis: [35](35-gaitem-allocator-invariants.md) |
| GaItemData (metadane broni/AoW) | `backend/core/writer.go:11-...` (`upsertGaItemData`) | limit `GaItemDataMaxCount = 7000`; rejection przy overflow (post-fix) |
| Slot rebuild | `backend/core/slot_rebuild.go:300-460` (`RebuildSlotFull`) | pełna re-serializacja sekcji z zachowaniem DLC + PlayerGameDataHash regions |
| Validator post-mutation | `backend/core/diagnostics.go:363-468` (`ValidatePostMutation`, `IntegrityError`) | 6 check keys, ostatnia linia obrony |
| Header reconcile | `backend/core/writer.go:971-1013` (`ReconcileStorageHeader`, `ReconcileInventoryHeader`) | po batch add zanim validator zbierze checki |
| Containers / cap-aware add | `backend/db/data/container_requirements.go:88-...` (`GetRequiredContainer`, `ApplyContainerCap`) | gated items (Throwing Pots, Aromatics) — best-effort qty trim |
| Companion flags (event flags) | `backend/db/data/ash_of_war_flags.go::AoWItemToFlagID`, `world_pickup_flags.go::WorldPickupFlagID`, `bolstering_pickup_flags.go::BolsteringPickupFlags`, `item_companion_flags.go::CompanionEventFlagsForItem` | best-effort, logged warnings |
| Tutorial IDs | `core.AppendTutorialID` + `data.AboutTutorialID` | tutorial completion flagi for "About *" items |
| Container pickup flags | `data.ContainerPickupFlags`, `data.ContainerVendorPurchaseFlags` | flagi aktywowane dla itemu-kontenera (Cracked Pot itp.) |
| Result typ | `app.go:295-310` (`SkippedAdd`, `AddResult`) | propagowane do JS: `Added`, `Requested`, `Trimmed`, `CapHit`, `Free*`, `Needed*` |
| Undo stack | `app.go::pushUndo` | osobny mechanizm od snapshot; push przed mutacją, niezależny od rollback |

---

## Pipeline overview

```
AddItemsToCharacter(charIdx, itemIDs, upgrades..., invQty, storageQty)
│
├── PRE-FLIGHT (no mutation)
│   ├── 1a. ScanDuplicateInventoryIndices(slot)
│   │   └── len(dups) > 0 → return error, slot nietknięty
│   ├── 1b. Build existing-qty maps (inv, storage, key items, containers)
│   ├── 1c. FCFS sort: gated items first (ascending ID), reszta stabilna
│   ├── 1d. Pre-compute prepared items
│   │   ├── apply upgrade25 / upgrade10 / infuseOffset / upgradeAsh
│   │   ├── trim qty for stackable already-at-max
│   │   └── apply container caps (Throwing Pots/Aromatics) → trimmed[]
│   └── 1e. CheckAddCapacity(slot, capacityItems)
│       └── !CanFitAll → return AddResult{CapHit, Free*, Needed*}, slot nietknięty
│
├── SNAPSHOT
│   ├── pushUndo(charIdx)            ← user-facing undo
│   └── snapshot = SnapshotSlot(slot)  ← internal safety net
│
├── MUTATE
│   ├── AddItemsToSlotBatch(slot, capacityItems)
│   │   ├── Phase 1: alokacja GaItems (allocateGaItem, generateUniqueHandle, upsertGaItemData)
│   │   ├── Phase 2: RebuildSlotFull(slot) — JEDEN rebuild dla całego batcha
│   │   └── Phase 3: zapis do Inventory.CommonItems / Storage.CommonItems
│   └── error → RestoreSlot(slot, snapshot), return wrapped error
│
├── POST-FLAGS (best-effort, log warnings, no rollback)
│   ├── AoW item flag (AoWItemToFlagID)
│   ├── World pickup flag (WorldPickupFlagID)
│   ├── Bolstering pickup flags (BolsteringPickupFlags)
│   ├── Tutorial ID (AboutTutorialID via AppendTutorialID)
│   └── Companion event flags (CompanionEventFlagsForItem)
│
├── CONTAINER KEY ITEMS (transactional, rollback-on-fail)
│   ├── Per used container: AddItemsToSlot(slot, [cID], desired, 0, false)
│   │   └── error → RestoreSlot(slot, snapshot), return wrapped error
│   └── Container pickup flags + Vendor purchase flags (best-effort)
│
├── RECONCILE
│   └── ReconcileStorageHeader(slot)
│
├── POST-VALIDATION
│   └── violations := ValidatePostMutation(slot)
│       └── len(violations) > 0 → RestoreSlot(slot, snapshot), return error z pierwszym violation
│
└── SUCCESS
    └── return AddResult{Added, Trimmed, FreeInv, FreeStore}
```

---

## Entry points and slot selection

Single public entry point: `App.AddItemsToCharacter(charIdx int, itemIDs []uint32, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty int) (AddResult, error)` (`app.go:327`).

- `charIdx` — slot postaci (`0..9`). Walidacja: `charIdx < 0 || charIdx >= 10` → error.
- `itemIDs` — lista **base** item IDs (canonical DB values, np. `0x000F4240` Uchigatana, `0x40001234` goods).
- `upgrade25`, `upgrade10` — addytywne shifty dla broni z `MaxUpgrade == 25` / `MaxUpgrade == 10`. Frontend wybiera poziom ulepszenia globalnie dla całego batcha.
- `infuseOffset` — addytywny shift dla itemów obsługujących infusion (`weaponCategorySupportsInfusion`: `melee_armaments` + `shields`). Bows/crossbows/staves/seals **nie** dostają infuse offset, mimo `MaxUpgrade == 25` — bo gra nie rozpoznaje ich infused IDs (komentarz w `app.go:312-318`).
- `upgradeAsh` — shift dla `category == "ashes"` (spirit ashes).
- `invQty`, `storageQty` — żądana ilość per item: `0` = skip, `-1` = max (z `db.ItemData.Max*`), `> 0` = `min(qty, max)`.

Pre-walidacja `a.save == nil` → error "no save loaded" przed jakąkolwiek robotą.

---

## Batch add model

Cała operacja jest **wsadowa**: jedna lista `itemIDs` → jeden `AddResult`. Korzyść wydajnościowa (`writer.go:271-272`):

> "All GaItem allocations happen in Phase 1, then ONE RebuildSlotFull in Phase 2, then all inventory/storage writes in Phase 3. This is O(1) rebuilds instead of O(N)."

W tradycyjnym podejściu "per item" każde dodanie wymagałoby rebuild slotu i revalidacji offsetów dynamicznych. Batch redukuje to do pojedynczej operacji.

`AddItemsToSlotBatch` przyjmuje `[]ItemToAdd` (`backend/core/capacity.go:62-69`):

```go
type ItemToAdd struct {
    ItemID         uint32  // już po wszystkich shiftach (upgrade, infuse, ash)
    InvQty         int     // ilość do dodania do Inventory.CommonItems
    StorageQty     int     // ilość do dodania do Storage.CommonItems
    ForceStackable bool    // wymuszone traktowanie jako stackable (np. dla arrows)
    IsStackable    bool    // hint z capacity check (informacyjne)
}
```

Frontend nigdy nie konstruuje `ItemToAdd` bezpośrednio — `AddItemsToCharacter` mapuje `itemIDs` → `capacityItems` po pre-flight.

---

## Stackable goods vs instance-backed items

Allocator handle prefixu różnicuje dwa modele alokacji (`writer.go:299-349`):

| Typ | Handle prefix | Model |
|---|---|---|
| Goods | `0xB0` (`ItemTypeItem`) | **stackable**: jeden GaItem dzielony między rekordy inv + storage |
| Accessory / Talisman | `0xA0` (`ItemTypeAccessory`) | **stackable** per item-ID (lookup existing handle w `slot.GaMap`) |
| Weapon | `0x80` (`ItemTypeWeapon`) | **instance-backed**: osobny GaItem dla każdego rekordu (per destination) |
| Armor | `0x90` (`ItemTypeArmor`) | **instance-backed** |
| AoW gem | `0xC0` (`ItemTypeAow`) | **instance-backed** |
| Arrow / Bolt | `0x80` ale `db.IsArrowID(id)` | wymuszone jako stackable (`ForceStackable = true`) |

### Stackable path (`writer.go:303-329`)

1. Szuka istniejącego handle w `slot.GaMap` po pasującym ItemID.
2. Jeśli znaleziono: reużywa handle dla obu `invQty` i `storageQty`.
3. Jeśli nie znaleziono:
   - **goods (`0xB0`) lub accessory (`0xA0`)**: konstruuje syntetyczny handle `(itemID & 0x0FFFFFFF) | prefix` i zapisuje `slot.GaMap[handle] = itemID` (bez alokacji GaItem entry — handle istnieje tylko w mapie).
   - **wymuszony stackable (np. arrows)**: regularna alokacja GaItem (`allocNewGaItem`).
4. Appenduje **jeden** `pendingInv` z (handle, invQty, storageQty).

### Instance-backed path (`writer.go:331-349`)

1. Jeśli `invQty != 0`: alokuje osobny GaItem przez `allocNewGaItem` (handle + `allocateGaItem` + `upsertGaItemData` jeśli weapon/AoW non-arrow).
2. Jeśli `storageQty != 0`: **drugi** osobny GaItem (separate handle, separate entry).
3. Appenduje **dwa** osobne `pendingInv` jeśli oba kontenery dostają item.

> ⚠️ Współdzielenie handle między inv i storage dla **non-stackable** items jest celowo zakazane — komentarz w `writer.go:332-333`: "see AddItemsToSlot for the explanation of why sharing a handle corrupts the save." Pełna analiza w [03 → GaItems vs containers](03-gaitem-map.md#gaitems-vs-containers) i [53-inventory-storage-transfer](53-inventory-storage-transfer.md) (rehandle path dla duplicate-handle transferu).

---

## GaItem allocator integration

`AddItemsToSlotBatch` używa allocator helper'a `allocNewGaItem(id, handlePrefix)` (`writer.go:282-297`):

```go
allocNewGaItem := func(id, handlePrefix uint32) (uint32, error) {
    h, err := generateUniqueHandle(slot, handlePrefix)
    if err != nil {
        return 0, err
    }
    if err := allocateGaItem(slot, h, id); err != nil {
        return 0, err
    }
    slot.GaMap[h] = id
    if (handlePrefix == ItemTypeWeapon && !db.IsArrowID(id)) || handlePrefix == ItemTypeAow {
        if err := upsertGaItemData(slot, id); err != nil {
            return 0, err
        }
    }
    return h, nil
}
```

Punkty integracji:

1. **`generateUniqueHandle`** — alokuje nowy handle z licznika `slot.NextGaItemHandle`, sprawdza unikalność w `slot.GaMap`, limit 10000 prób (`MaxHandleAttempts`). Pełna mechanika: [35 → Handle generation](35-gaitem-allocator-invariants.md#handle-generation).
2. **`allocateGaItem`** — wstawia rekord do `slot.GaItems[NextAoWIndex]` (dla AoW) lub `slot.GaItems[NextArmamentIndex]` (dla broni/zbroi/talizmanu), aktualizuje kursory zone-aware. **AoW guard po commicie `6881cb9`** odrzuca alloc przy `NextArmamentIndex >= len(GaItems)`. Pełna mechanika: [35 → Allocation zones](35-gaitem-allocator-invariants.md#allocation-zones).
3. **`upsertGaItemData`** — dodaje wpis do sekcji `GaItemData` (metadane broni/AoW; `GaItemDataMaxCount = 7000`). Tylko dla weapon (non-arrow) i AoW. Reject przy overflow.

> **Implementator note**: Allocator powinien failować **pre-mutation** lub wcześnie w batch — komunikat error musi być czytelny (np. "armament zone at capacity (NextArmamentIndex 5120 == 5120)"). Validator post-mutation jest ostatnią linią obrony, **nie** primary error path. Pełen przykład patologiczny (mała liczba non-empty wpisów, ale armament zone full): [35 → UI counters vs allocator capacity](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity).

---

## Inventory and Storage mutation boundaries

Phase 3 w `AddItemsToSlotBatch` (po `RebuildSlotFull`) iteruje `pending` i zapisuje per-record do binarki:

- **Inventory writes**: `addToInventory(slot, handle, invQty)` (jeśli `invQty > 0`).
- **Storage writes**: `addToStorage(slot, handle, storageQty)` (jeśli `storageQty > 0`).

Te funkcje nadpisują pierwszy empty slot w odpowiedniej tablicy i ustawiają `{handle, quantity, index}`. `index` jest przypisywany z `NextEquipIndex` / `NextAcquisitionSortId` po stronie kontenera.

Po pętli pending:

- `ReconcileStorageHeader(slot)` (`writer.go:971-991`) — synchronizuje `storage_count` header z liczbą non-empty.
- `ReconcileInventoryHeader(slot)` — analogicznie dla inventory `common_item_count`.

Granice:
- `AddItemsToSlotBatch` **nie** modyfikuje `Storage.KeyItems` (storage key items nie są runtime-eksponowane; zob. [10 → Read path](10-storage.md#read-path)).
- `Inventory.KeyItems` jest modyfikowane wyłącznie w sekcji "Container key items" wyższej warstwy (`app.go:582-595`), nie w core batch.

---

## Item companion flags

Część itemów wymaga ustawienia flag/eventów razem z dodaniem do inventory. Save Forge orkiestruje to w sekcji POST-FLAGS w `AddItemsToCharacter` (`app.go:526-579`).

### Mapowania (best-effort, log warning on error)

| Mapowanie | Plik | Sytuacja |
|---|---|---|
| `data.AoWItemToFlagID[itemID] → flagID` | `backend/db/data/ash_of_war_flags.go` | duplikacja-prevention flag dla Lost Ash of War: blokuje grę przed ponownym przyznaniem |
| `data.WorldPickupFlagID[itemID] → flagID` | `backend/db/data/world_pickup_flags.go` | "podniesiono w świecie" — flaga dla itemów z fixed pickup location |
| `data.BolsteringPickupFlags[itemID] → []flagID` | `backend/db/data/bolstering_pickup_flags.go` | per-instancja: dla każdej dodanej sztuki Golden Seed / Sacred Tear etc. setowana jest kolejna flaga z listy |
| `data.AboutTutorialID[itemID] → tutorialID` | `backend/db/data/tutorial_ids.go` | "About *" itemy ukończone w tutorial system; setowane przez `core.AppendTutorialID` (`backend/core/tutorial_data.go:78`) |
| `data.CompanionEventFlagsForItem(itemID) → []flagID` | `backend/db/data/item_companion_flags.go` | dodatkowe event flagi powiązane z konkretnym itemem (np. quest stepy) |

> ⚠️ **Pełna lista companion flags i ich semantyka** — [50-item-companion-flags](50-item-companion-flags.md). Rozdział 43 opisuje tylko **gdzie w pipeline** są ustawiane (po batch add, przed reconcile) oraz **gwarancje transakcyjne** (best-effort: log warn on error, brak rollback).

### Transactional bezpieczeństwo companion flags

Companion flag failure **nie** wywołuje rollback batch'a. Każde wywołanie `db.SetEventFlag` / `AppendTutorialID` zawinięte w `if err != nil { runtime.LogWarning... }` (`app.go:530-579`). Uzasadnienie:

- Flagi są pomocnicze dla doświadczenia w grze (UI shows item as "already collected"), nie krytyczne dla integralności save'a.
- Hard rollback po nieudanym `SetEventFlag` zostawiłby użytkownika bez itemów, których allocator i validator już zaakceptowały.
- Failure space: tylko `slot.EventFlagsOffset <= 0` lub bit poza zakresem — bardzo rzadkie.

### Container key items (rollback-on-fail)

W przeciwieństwie do companion flags, **container itemy** (Empty Cracked Pot dla Throwing Pots, Empty Cracked Ritual Pot dla Aromatics etc.) są dodawane przez osobny `AddItemsToSlot` w sekcji "Auto-add containers" (`app.go:582-595`):

```go
if err := core.AddItemsToSlot(slot, []uint32{cID}, desired, 0, false); err != nil {
    core.RestoreSlot(slot, snapshot)
    return result, fmt.Errorf("rollback after container add: %w", err)
}
```

Failure container itemu (np. capacity hit dla key items) **wywołuje rollback** — bo bez kontenera Throwing Pots nie są używalne w grze.

---

## Snapshot, rollback and failure semantics

### Snapshot (pre-mutation)

`snapshot := SnapshotSlot(slot)` (`snapshot.go:28-67`) wykonuje deep copy wszystkich mutable pól `SaveSlot`:

- `slot.Data` (cały bufor bajtów)
- `slot.Version`, `slot.Player`
- `slot.GaMap`, `slot.GaItems`
- `slot.Inventory.Clone()`, `slot.Storage.Clone()`
- `slot.Warnings`
- Wszystkie offsety dynamiczne (`MagicOffset`, `InventoryEnd`, `EventFlagsOffset`, `PlayerDataOffset`, `FaceDataOffset`, `StorageBoxOffset`, `IngameTimerOffset`, `GaItemDataOffset`, `TutorialDataOffset`)
- Wszystkie countery (`NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`, `PartGaItemHandle`)

### Rollback (`RestoreSlot`)

`RestoreSlot(slot, snap)` (`snapshot.go:69-102`) nadpisuje wszystkie pola z snapshota. Wywoływane w 3 miejscach `AddItemsToCharacter`:

1. Po `AddItemsToSlotBatch` zwracającym error (`app.go:521-524`).
2. Po failed container key-item add (`app.go:590-593`).
3. Po `ValidatePostMutation` zwracającym non-empty violations (`app.go:626-629`).

W każdym przypadku slot wraca do stanu sprzed `SnapshotSlot`, a funkcja zwraca wrapped error.

### Relacja z user-facing undo

`pushUndo(charIdx)` (`app.go:517`) jest wołane **przed** `SnapshotSlot`. Snapshot i undo to **osobne mechanizmy**:

- **Undo stack**: user może cofnąć udane Add Items operation. Stos zawiera kopie slot.Data sprzed mutacji.
- **Snapshot/Restore**: wewnętrzny safety net dla failed mutation. Nie tyka undo stack.

Konsekwencja: po failed Add Items, undo stack ma niepotrzebny wpis identyczny z aktualnym stanem (idempotent undo). Akceptowalne, bo undo bezstanowo cofa do stanu sprzed pushUndo.

### Failure semantics — kategorie

| Faza | Failure | Skutek |
|---|---|---|
| PRE-FLIGHT 1a | duplikat Index w istniejącym inventory | reject, slot nietknięty, return descriptive error |
| PRE-FLIGHT 1e | capacity hit | reject, slot nietknięty, return `AddResult{CapHit, Free*, Needed*}` |
| MUTATE — Phase 1 (allocator) | `armament zone at capacity`, `array full`, handle overflow | RestoreSlot, return wrapped error |
| MUTATE — Phase 2 (RebuildSlotFull) | section overflow, regions issue | RestoreSlot, return wrapped error |
| MUTATE — Phase 3 (container writes) | overflow inv/storage (powinno być pokryte pre-flight) | RestoreSlot, return wrapped error |
| POST-FLAGS | `SetEventFlag` error, `AppendTutorialID` error | log warning, **brak rollback** |
| CONTAINER KEY ITEMS | failed `AddItemsToSlot` | RestoreSlot, return wrapped error |
| RECONCILE | brak failure path | — |
| POST-VALIDATION | `ValidatePostMutation` zwraca violations | RestoreSlot, return error z pierwszym violation |

---

## Post-mutation validation

`ValidatePostMutation(slot)` (`diagnostics.go:363-468`) jest **ostatnią linią obrony**. Allocator i pre-flight powinny złapać 100% legalnych user-facing errors; validator łapie bugi (np. mismatch counterów po regression w nowej ścieżce kodu).

### Kategorie błędów (skrót)

Pełna lista 6 unikalnych check keys + szczegóły — [35 → Post-mutation validation](35-gaitem-allocator-invariants.md#post-mutation-validation). Z perspektywy Add Items kluczowe są:

- **`orphan_handle`** — non-empty handle inventory typu `0x80/0x90/0xC0` bez wpisu w `GaMap`. Bug w allocator path.
- **`duplicate_index`** — dwa wpisy `CommonItems ∪ KeyItems` z tym samym `Index`. Bug w stride-2 / sort logic.
- **`gaitemdata_count`** — `GaItemData count > 7000`. Powinno być złapane przez `upsertGaItemData` rejection.
- **`storage_count`** — `storage_count header != non-empty count`. Naprawia `ReconcileStorageHeader`; failure tu = bug w order operacji.
- **`gaitem_indices`** — `NextAoWIndex > NextArmamentIndex` lub `NextArmamentIndex > len(GaItems)`. Bug allocator (np. obejście AoW guard'a z `6881cb9`).
- **`gamap_zero_id`** — `GaMap[h] == 0`. Bug w `upsertGaItemData` / `allocateGaItem`.

### Czemu validator jest "last line"

- Validator zwraca pierwszy violation jako string — gubi kontekst (który item, jaka pozycja).
- Validator zawsze wymusza rollback — nie próbuje naprawiać.
- Validator **nie** rozróżnia user-facing errors (capacity) od bugów alokatora (mismatch counterów). Dla user-facing errors `AddResult.CapHit` jest właściwym kanałem.

Zasada projektowa: **allocator + pre-flight łapią 100% legalnych errors. Validator łapie bugi.**

---

## Capacity and UI counter caveat

Add Items **nie** może polegać na UI `ALL ITEMS used/max` / `INVENTORY used/max` / `STORAGE used/max` jako primary capacity check. Te paski (`SlotCapacity` z `app_deploy.go::GetSlotCapacity`) raportują `non_empty / max` per kontener — czyli **container layer usage**, nie allocator-side free space.

### Co używa Add Items

`CheckAddCapacity(slot, items)` (`capacity.go:85-222`) liczy **wszystkie** wymagane zasoby per item:

- `NeededInv` / `FreeInv` — Inventory.CommonItems slots
- `NeededStorage` / `FreeStorage` — Storage.CommonItems slots
- `NeededGaItems` / `FreeGaItems` — GaItem entries (`len(GaItems)` minus non-empty)
- `NeededGaItemData` / `FreeGaItemData` — `GaItemData` slots (`GaItemDataMaxCount = 7000`)

Pierwszy hit ustala `CapHit`:

```go
if neededGaItemData > FreeGaItemData      { CapHit = "gaitemdata_full" }
else if neededGaItems > FreeGaItems       { CapHit = "gaitem_full" }
else if neededInvSlots > FreeInv          { CapHit = "inventory_full" }
else if neededStorageSlots > FreeStorage  { CapHit = "storage_full" }
```

### Pułapka armament zone

Nawet `CheckAddCapacity` mówiące "fits" **nie gwarantuje** sukcesu allocator'a — `FreeGaItems = GaItemsMax - non_empty count` nie jest tym samym co **armament zone free** = `len(GaItems) - NextArmamentIndex`. Save z wieloma empty wpisami między kursorami a `NextArmamentIndex == len(GaItems)` przejdzie pre-flight, ale failuje w `allocateGaItem` z "armament zone at capacity". Pełen przykład patologiczny: [35 → UI counters vs allocator capacity](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity).

> ⚠️ Zone-aware capacity nie jest aktualnie eksponowane do pre-flight ani UI — `needs verification` jako planowane usprawnienie (zob. [35 → Known limits](35-gaitem-allocator-invariants.md#known-limits--open-questions)). Aktualnie taki failure jest zwracany jako wrapped error po RestoreSlot, nie jako `AddResult{CapHit}`.

---

## Error handling and user-facing messages

`AddResult` (`app.go:300-310`) jest jedynym path-em sukcesowym propagującym informacje do UI:

```go
type AddResult struct {
    Added       int          // liczba itemów faktycznie dodanych
    Requested   int          // len(itemIDs)
    Trimmed     []SkippedAdd // qty trimmed przez container caps (best-effort)
    CapHit      string       // "" jeśli sukces; inaczej "inventory_full" | ...
    FreeInv     int
    FreeStore   int
    NeededInv   int          // tylko jeśli CapHit != ""
    NeededStore int
}
```

### Kanały błędów

1. **`CapHit != ""`** — pre-flight capacity check failed. UI wyświetla "Not enough space" toast z `Needed* / Free*`. Slot nietknięty.
2. **`(AddResult, error)` z `error != nil`** — błąd techniczny (allocator failure, rebuild error, validation failure). UI loguje error string. Slot **przywrócony** przez RestoreSlot.
3. **`Trimmed != nil` ale `CapHit == ""`** — sukces częściowy (container cap wymusił trim qty). UI pokazuje `Added X / Requested Y`, opcjonalnie listuje trimmed items. Slot zmodyfikowany.
4. **Brak czegokolwiek** — pełny sukces. UI odświeża widok inventory.

### Best-effort warnings (nie propagowane do AddResult)

- Companion flag failures są logowane jako `runtime.LogWarning` (Wails runtime logger). Nie wracają do JS jako error.
- Tutorial ID failures analogicznie.
- Vendor purchase flags analogicznie.

---

## Test coverage

| Test | Co weryfikuje |
|---|---|
| `tests/capacity_test.go` (12 testów) | Pre-flight w różnych konfiguracjach (`TestPreFlightCapacity_Empty`, `TestPreFlightCapacity_CountsUsage`), batch add (`TestBatchAdd_SingleRebuild`, `TestBatchAdd_MixedStackableNonStackable`), snapshot/rollback (`TestSnapshotRestore_*`), post-validation (`TestPostValidation_*`), header reconcile (`TestStorageHeaderReconcile`), `TestGaItemDataFull_ErrorNotSilent`, `TestRoundtrip_BatchAdd` |
| `tests/bulk_add_test.go` | Stress test batch add: armament zone capacity, mixed AoW+weapon, multi-category batch |
| `tests/bulk_add_test.go::TestAddItems_RespectsArmamentCapacity` | Allocator AoW guard `6881cb9` (post-fix) |
| `backend/core/gaitem_placement_test.go::TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity` | Allocator-level reject przed mutacją |
| `app_additems_duplicate_index_test.go` | Pre-flight 1a reject przy istniejących duplikatach Index |
| `app_repair_duplicate_index_test.go` | `RepairDuplicateInventoryIndices` idempotency |
| `tests/item_companion_flags_test.go` | Companion flag setting przy Add Items |
| `tests/grace_companion_flags_test.go` | Grace-specific companion flags |
| Round-trip fixtures (`tests/roundtrip_test.go`, `save_modify_test.go`) | Pełna integracja: Add Items → Write → Reload → validate |

---

## Verified native save-write evidence

Laboratorium native save (`app_storage_add_session_test.go`) zweryfikowało
dodatkowe kontrakty dla opisanego wyżej direct Add pipeline:

- **Jeden aktywny wpis GaItemData per genuinely new ID** (T040/T050/T060/T062/T070/T071/T074/T090):
  genuinely new item ID dostaje dokładnie jeden aktywny wpis GaItemData przy
  pierwszym direct add; kolejne wywołanie Add dla ID, który postać już posiada,
  nie może stworzyć drugiego aktywnego wpisu. Dotyczy to całej pipeline niezależnie od tego, który
  kontener (`Inventory`/`Storage`) odbiera item — zob.
  [07 → Inventory.CommonItems](07-inventory.md#verified-write-contracts-native-save-evidence)
  i [10 → Verified write contracts](10-storage.md#verified-write-contracts-native-save-evidence)
  dla kontraktów counterów per-kontener, z którymi się to łączy.
- **T351** potwierdza pojedynczy submit ("one-submit") Storage Mega add w
  izolacji — jedno wywołanie `AddItemsToCharacter`, jeden batch, jeden
  `RebuildSlotFull`. T351 jest dowodem tylko dla tego jednorazowego scenariusza.
- **T352** potwierdza, że sześć osobnych direct Database Add calls wykonanych
  w jednej nieprzerwanej sesji edytora (czyli wiele wywołań
  `AddItemsToCharacter`, nie jeden batch) zachowuje oryginalnie pusty kontekst
  Storage ustanowiony przez pierwsze wywołanie w tej serii. Ten kontekst to
  explicit stan `App` (`storageAddSessions`), przekazywany do core przez
  `AddItemsToSlotBatchForStorageSession`, i nigdy nie jest wnioskowany ani
  persystowany przez sam core. Resetuje się przy canonical save, load/reload
  i zamknięciu okna — świeża sesja zaczyna się od nowa jako pusta. T351 **nie**
  zastępuje T352: przechodzący test one-submit nie jest dowodem, że kontekst
  multi-call session jest poprawnie spięty.

Poza zakresem tego dowodu: zachowanie EventFlag, Info Item, tutorial i
crafting-flag wyzwalane razem z tymi dodaniami (zob. "Known limits" niżej),
oraz każda ilość/wielkość batcha wykraczająca poza to, co przetestowały
T351/T352.

---

## Known limits / needs verification

- **Zone-aware capacity w pre-flight** — `CheckAddCapacity` nie liczy `armament_zone_free = len(GaItems) - NextArmamentIndex`. Save z `NextArmamentIndex` blisko `len(GaItems)` przejdzie pre-flight, ale failuje w allocator. Status: `needs verification` jako future task w [35](35-gaitem-allocator-invariants.md#known-limits--open-questions).
- **Best-effort companion flag failures** — nie ma kanału propagacji do UI. Użytkownik nie wie, że "About *" tutorial nie został oznaczony jako completed. `needs verification`, czy to powoduje observable issues w grze (UI duplikuje tutorial po reloadzie, fast-travel discovery nie działa, etc.).
- **Vendor purchase flags** (`ContainerVendorPurchaseFlags`) — dodawane dla container itemów, ale ich semantyka w grze (czy faktycznie ukrywają item u vendora po Add Items) — `needs verification`.
- **Pełna lista handle prefiksów obsługiwanych przez allocator** — `AddItemsToSlotBatch` zakłada pięć typów (`0x80/0x90/0xA0/0xB0/0xC0`). Handle z innym prefiksem nie jest oczekiwany ani testowany. `needs verification`, czy `db.ItemIDToHandlePrefix` może zwrócić nieobsługiwaną wartość dla skrajnych ItemID.
- **`upsertGaItemData` failure path** — pre-fix (sprzed `6881cb9` era) zwracał `nil` przy overflow zamiast error. Aktualnie zwraca error przy `count >= GaItemDataMaxCount`. `needs verification`, czy wszystkie call-sity propagują ten error poprawnie (kod review pełnego call-tree).

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — binary model GaItem/GaMap, prefiksy handle, rekord 21/16/8 B.
- [07-inventory](07-inventory.md) — Inventory data model, `InventoryItem` 12 B, `CommonItems`/`KeyItems`, header reconcile.
- [10-storage](10-storage.md) — Storage data model, `ReadStorage`, `ReconcileStorageHeader`.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator, capacity, counters, validation, snapshot/rollback (canonical reference dla całej write-side semantyki low-level).
- [50-item-companion-flags](50-item-companion-flags.md) — pełna taksonomia companion flags ustawianych przy Add Items.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — acquisition `Index` assignment (Add Items używa `NextEquipIndex`/`NextAcquisitionSortId` z kontenera; pełna mechanika sortowania w 52).
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — separate operation (po Add Items); rehandle path dla duplicate-handle.
- [54-ash-of-war](54-ash-of-war.md) — semantyka custom AoW przy Add Items (`upsertGaItemData` dla AoW gemów, sentinele `AoWGaItemHandle`).

---

## Sources

- `app.go` — `AddItemsToCharacter`, `AddResult`, `SkippedAdd`, `weaponCategorySupportsInfusion`, `resolveQty`
- `backend/core/writer.go` — `AddItemsToSlot`, `AddItemsToSlotBatch`, `allocateGaItem`, `generateUniqueHandle`, `upsertGaItemData`, `ReconcileInventoryHeader`, `ReconcileStorageHeader`
- `backend/core/capacity.go` — `CheckAddCapacity`, `CapacityReport`, `CountSlotUsage`, `SlotUsage`, `ItemToAdd`, `handlePrefixForStackable`, `needsGaItemData`
- `backend/core/snapshot.go` — `SnapshotSlot`, `RestoreSlot`, `SlotSnapshot`
- `backend/core/slot_rebuild.go` — `RebuildSlotFull`
- `backend/core/diagnostics.go` — `ValidatePostMutation`, `IntegrityError`, `ScanDuplicateInventoryIndices`
- `backend/db/data/container_requirements.go` — `GetRequiredContainer`, `ApplyContainerCap`
- `backend/db/data/{ash_of_war_flags,world_pickup_flags,bolstering_pickup_flags,container_pickup_flags,item_companion_flags,tutorial_ids}.go` — companion flag mappings
- `backend/core/tutorial_data.go` — `AppendTutorialID` (write path dla `AboutTutorialID`)
- Tests: `tests/{capacity,bulk_add,item_companion_flags,grace_companion_flags,roundtrip,save_modify}_test.go`, `backend/core/gaitem_placement_test.go`, `app_additems_duplicate_index_test.go`, `app_repair_duplicate_index_test.go`
- Commit `6881cb9 fix(core): guard AoW allocation at armament capacity` — kontekst dla AoW allocator guard
