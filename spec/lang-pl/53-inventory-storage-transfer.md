# 53 — Transfer Inventory ↔ Storage

> **Typ**: Design doc
> **Status**: ✅ Wdrożony (workspace path) + ✅ Wdrożony (legacy core path)
> **Zakres**: Mechanika dwukierunkowego przenoszenia rekordów między `slot.Inventory.CommonItems` a `slot.Storage.CommonItems`, semantyka instance-move vs quantity-merge, rehandle dla duplikatów instancji, equipped guard, oraz integracja z dwukolumnowym workspace UI w `SortOrderTab`.

---

## Cel rozdziału

Rozdział opisuje **aktualną** mechanikę przenoszenia itemów między kontenerami postaci (Inventory ⇄ Storage) tak, jak realizują ją dwie współistniejące ścieżki w kodzie:

1. **Legacy core path** — `core.MoveItemsBetweenContainers` wołane przez App-level binding `App.MoveItemsBetweenInventoryAndStorage`. Mutuje binary bezpośrednio, oferuje cap-aware merge, rehandle i equipped guard. Pozostaje aktywne jako publiczny binding i pełna powierzchnia testowa.
2. **Workspace save path** — `editor.ApplyWorkspaceSave` → `writeContainerLayout`. Wipe-and-replay całych regionów `CommonItems`. Używane przez aktualne UI `SortOrderTab` (dwukolumnowy widok Storage|Inventory). **OriginalHandle jest zachowane** podczas transferu — workspace nie wywołuje rehandle.

Rozdział NIE dokumentuje sort dropdownów, per-side Apply ani per-side preview guardów — żaden z tych elementów nie istnieje w obecnym `SortOrderTab.tsx`. Zob. też [39](39-inventory-reorder.md) jako historical design note.

---

## Status

| Komponent | Status |
|---|---|
| `core.MoveItemsBetweenContainers` (legacy) | ✅ active, 28 testów w `tests/transfer_test.go` |
| `App.MoveItemsBetweenInventoryAndStorage` (App wrapper) | ✅ active, eksponowane przez Wails bindings |
| `editor.ApplyWorkspaceSave` + `writeContainerLayout` (workspace) | ✅ active, woła go każdy `workspace.save()` z `SortOrderTab` |
| Dwukolumnowy `SortOrderTab` UI | ✅ aktualnie używany |
| Sort dropdowny per strona | ❌ nie istnieją — wcześniejszy projekt nie został wdrożony |
| Per-side Apply / Reset Preview / per-side hasChanges guards | ❌ nie istnieją — workspace ma jedno globalne Save/Discard |
| In-game verification (Steam Deck) — Storage Apply / transfer | `needs verification` — brak świeżego raportu z fixtura PS4 |

---

## Source of truth w kodzie

| Topic | Plik / funkcja | Testy |
|---|---|---|
| Core engine | `backend/core/transfer.go:94` — `MoveItemsBetweenContainers(slot, handles, direction, opts)` | `tests/transfer_test.go` |
| App wrapper | `app.go:741` — `MoveItemsBetweenInventoryAndStorage(charIdx, handles, direction string)` | tamże |
| Instance-move | `backend/core/transfer.go:199` — gałąź dla `isInstanceMoveHandle` | `TestMoveNonStackableInvToStorage`, `TestMoveNonStackableStorageToInv` |
| Quantity-merge | `backend/core/transfer.go:252` — cap-aware merge / partial | `TestMoveStackable*`, `TestMoveStackableInvToStorage_MergePartialAtCap` |
| Rehandle | `backend/core/transfer.go:208` — `materializeRehandledInstance` + `rebuildAfterAllocation` | `TestMoveTalismanDestHasSameHandle_RehandlesAndMoves`, `TestMoveTalismanStorageToInventory_DuplicateHandle_RehandlesAndMoves` |
| Equipped guard | `backend/core/transfer.go:194,592` — `IsHandleEquipped` (ChrAsmEquipment scan) | `TestMoveEquippedInvToStorageSkipped` |
| Header reconcile | `backend/core/transfer.go:128-129` — `ReconcileInventoryHeader` / `ReconcileStorageHeader` | `TestMoveHeadersReconciled` |
| Workspace save | `backend/editor/save.go:79` — `ApplyWorkspaceSave` | `backend/editor/save_test.go` (collect/validate AoW, detect removed/transferred, pass-through indices, pickNewHandle), `backend/editor/workspace_test.go` (`TestBuildSnapshot_*`) |
| Workspace save App caller | `app_inventory_session.go:192` — `SaveInventoryWorkspaceChanges(sessionID)` z `core.SnapshotSlot`/`RestoreSlot` + `pushUndo` | `app_inventory_session_save_test.go`, `app_inventory_session_sequential_test.go` |
| Workspace layout writer | `backend/editor/save.go:518` — `writeContainerLayout` (wipe-and-replay, stride-2 acq base z linii 566) | tamże |
| Frontend dwukolumnowy UI | `frontend/src/components/SortOrderTab.tsx` (1303 linie) | — |
| Workspace hook (RAM-only ops) | `frontend/src/hooks/useInventoryWorkspace.ts` — `transferItem`, `moveItem`, `addItem`, `removeItem`, `save`, `discard` | — |

Model binarny rekordu 12-bajtowego i offsetów `CommonItems` zob. [07](07-inventory.md) (Inventory) i [10](10-storage.md) (Storage). Mapowanie GaItem ↔ handle zob. [03](03-gaitem-map.md). Allocator invariants zob. [35](35-gaitem-allocator-invariants.md). Stride-2 acquisition order zob. [52](52-acquisition-sort-stride2.md).

---

## Mental model

```
            ┌─────────────────┐                ┌─────────────────┐
            │  Inventory      │  ◀────────▶    │  Storage        │
            │  CommonItems    │   transfer     │  CommonItems    │
            │  (fixed array,  │                │  (sparse list,  │
            │   CommonItem-   │                │   capacity      │
            │   Count slots)  │                │   StorageCommon-│
            │                 │                │   Count)        │
            └─────────────────┘                └─────────────────┘
                    ▲                                  ▲
                    │ wspólne                          │
                    └─────────  slot.GaItems  ─────────┘
                                slot.GaMap (handle → itemID)
```

- Prefiks handle (najwyższy bajt) koduje **typ** itemu, nigdy lokalizację. Inventory i Storage dzielą jedną przestrzeń handle. Pełna taksonomia w [03](03-gaitem-map.md).
- Transfer **nigdy** nie usuwa wpisów `GaItem` ani `GaMap`. Ten sam handle dalej identyfikuje tę samą instancję po stronie docelowej, niosąc poziom ulepszenia, infuzję i podpięty AoW.
- Dla mechaniki **transferu** (nie czytania) liczą się tylko dwie kategorie prefiksów:

| Prefiks | Kategoria itemu | Semantyka transferu |
|---|---|---|
| `0x80` Weapon, `0x90` Armor, `0xA0` Accessory/Talisman, `0xC0` AoW | instance-backed | instance-move (rekord transplantowany; cap ignorowany) |
| `0xB0` Goods | stackable | quantity-merge cap-aware (możliwy partial move) |

---

## Current SortOrderTab workspace model

`SortOrderTab` operuje wyłącznie na **sesji workspace** zwracanej przez hook `useInventoryWorkspace()`. Wszystkie mutacje (reorder, transfer, add, remove, weapon edit) zachodzą w pamięci RAM po stronie Go i nie dotykają `slot.Data` aż do `workspace.save()`.

### Operacje wystawione z hooka

| Operacja | Sygnatura | Co robi |
|---|---|---|
| `start(charIdx)` | inicjalizuje sesję dla postaci | tworzy snapshot z aktualnego save |
| `moveItem(uid, container, targetPos)` | reorder w obrębie jednego kontenera | dispatch do RAM snapshot; nie woła `Reorder*` z `app_inventory_order.go` |
| `transferItem(uid, target)` | przeniesienie cross-grid | przepisuje `Container` w snapshocie z `inventory` na `storage` lub odwrotnie |
| `addItem(spec, target, pos)` | dodanie nowego itemu | dispatch do core `AddItemsToSlotBatch` przy save (zob. [43](43-transactional-item-adding.md)) |
| `removeItem(uid)` | usunięcie itemu | element znika ze snapshotu |
| `updateWeapon(uid, patch)` | upgrade/infusion/AoW patch | flagi pending zapisywane w `EditableItem` |
| `save()` | commit całej sesji do save file | wywołuje `editor.ApplyWorkspaceSave(slot, snap, baseline)` |
| `discard()` | odrzuca pending zmiany | restart sesji z bieżącego save |

### Co `SortOrderTab` ma, a czego nie ma

Ma (zweryfikowane w `frontend/src/components/SortOrderTab.tsx`):
- Storage po lewej, Inventory po prawej (linie 799–911).
- Frame 5 kolumn × 6 rzędów minimum (`GRID_COLS=5`, `GRID_MIN_ROWS=6`), `overflow-y-auto`.
- 6 zakładek kategorii: Weapons / Talismans / Head / Chest / Arms / Legs (linie 56–63).
- `UNARMED_BASE_ID = 0x0001ADB0` jest wykluczony z zakładki Weapons (frontend `tabFilter` + backend `inventoryOrderTabs`).
- Per-strona selekcja oparta o **UIDs** (`editor.EditableItem.uid: string`), nie o u32 handles. Stan: `invSelectedUIDs: Set<string>`, `invAnchorUID: string | null`, analogicznie `sto*`.
- Plain click / Ctrl-Cmd click (toggle) / Shift click (range).
- Drag w obrębie kontenera → `workspace.moveItem(uid, container, targetPos)`.
- Drag cross-container → sekwencyjnie `workspace.transferItem(uid, target)` dla wszystkich UIDów w batchu (jeżeli dragowany jest w selekcji i selekcja > 1, batch = `Array.from(selection)`).
- Add Item modal → `workspace.addItem(spec, target, -1)`. Cap-aware semantyka dodawania jest mechaniką [43](43-transactional-item-adding.md).
- Remove (top bar) → `workspace.removeItem(uid)` per zaznaczony UID.
- `dirty` flag, `validation` report (errors/warnings), `Save changes` zablokowane gdy `errorCount > 0`.

Nie ma (potwierdzone brakiem w kodzie):
- Sort dropdownu `Acquisition ↑/↓` / `Weight ↑/↓` / `Type ↑/↓` po żadnej stronie.
- Buttonu `Apply Order` / `Reset Preview` per strona.
- Per-side flag `hasChanges` / `storageHasChanges` ani trzech toast-gardów blokujących cross-transfer dla "unsaved order".
- Bannera "Dragging N items".

---

## Transfer directions and constraints

```go
const (
    TransferToStorage   TransferDirection = iota // Inventory.CommonItems → Storage.CommonItems
    TransferToInventory                          // Storage.CommonItems → Inventory.CommonItems
)
```

App wrapper przyjmuje dyrektywę stringową `"to-storage"` lub `"to-inventory"` i konwertuje na typ (zob. `app.go:749-757`). Każdy inny ciąg → błąd.

Constraints sprawdzane **w core**:
- `slot.Version == 0` → błąd (`empty slot`).
- `direction` poza dwoma znanymi wartościami → błąd.
- Handle `0` lub `0xFFFFFFFF` → skip z `SkipReasonInvalidHandle` (nigdy fatal).

App wrapper dodatkowo robi **dry-check** czy choć jeden handle istnieje w binary źródła. Jeżeli tak — `pushUndo(charIdx)` PRZED rzeczywistym wywołaniem core. Jeśli nie — undo nie jest pchane (oszczędność stosu undo).

---

## Drag/drop semantics

### Reorder w obrębie kontenera (`onTileDrop`, `workspace.moveItem`)

Pozycja docelowa jest obliczana z lokalnego indeksu kafelka w widoku per-zakładka i przeliczana na globalny indeks pełnej listy kontenera przez `computeTargetPosition` (`SortOrderTab.tsx:180-204`). Backend `MoveItem` interpretuje `targetPosition` w **post-pop slice** — przy downward move źródłowy element zostaje "wyciągnięty" z listy przed wstawieniem, więc frontend odejmuje 1 od `prePopTarget` gdy cel leży za źródłem.

Drop poza ostatnim kafelkiem zakładki → wstawienie tuż za ostatnim itemem widocznym w tej zakładce (`SortOrderTab.tsx:195-201`).

### Cross-grid transfer (`onFrameDrop`, `workspace.transferItem`)

Batch przy multi-select: jeśli dragowany UID jest w selekcji źródłowej **i** selekcja > 1 → batch = `Array.from(selection)`. W przeciwnym razie batch = `[draggedUID]`. UWAGA: kolejność batcha to iteracja po `Set<string>`, nie "visible order" — w obecnym kodzie nie jest stabilnie posortowana po pozycji w gridzie. (**needs verification** czy to zamierzone — z testów workspace nic to nie zmienia, bo każdy transfer dodaje item na koniec acquisition po stronie docelowej.)

Drop na frame przeciwnego kontenera wywołuje `await workspace.transferItem(uid, target)` sekwencyjnie po wszystkich UIDach.

### Occupied vs empty slot — workspace model

Workspace **nie ma pojęcia "slot zajęty"** w sensie binarnym. Snapshot to lista `EditableItem` posortowana po pozycji; drop na pozycję X = wstaw w pozycję X i przesuń resztę. Brak collision check, bo binary jest re-emitowane przez `writeContainerLayout` od zera.

---

## Inventory → Storage

Legacy core path (`TransferToStorage`):
1. Lookup źródłowego rekordu w `Inventory.CommonItems` po handle (`scanRecord`).
2. **Equipped guard**: `IsHandleEquipped(slot, handle)` — jeśli prawda → skip `SkipReasonEquipped`.
3. Klasyfikacja: instance-move (prefiksy `0x80`/`0x90`/`0xA0`/`0xC0`) vs quantity-merge (`0xB0`).
4. Instance-move:
   - Dest empty slot → `writeRecord(dst)` z tym samym handle, oryginalną `Quantity`, nowym `Index` (z `assignDestIndex` po stronie Storage).
   - Dest **already has same handle** → ścieżka **rehandle** (sekcja "Instance-backed item transfer and rehandle" poniżej).
5. Quantity-merge:
   - Dest istnieje → `dstExistingQty + transferQty` clamped do `caps[handle]`, partial → `srcQty -= transferQty`.
   - Dest nie istnieje → nowy rekord z `transferQty = min(srcQty, cap)`.
6. Po batchu: `ReconcileInventoryHeader`, `ReconcileStorageHeader`, `rescanInventoryList`, `rescanStorageList`.

Workspace path:
- Snapshot RAM przepisuje `EditableItem.Container = ContainerStorage`.
- Przy `workspace.save()` → `ApplyWorkspaceSave` → `writeContainerLayout(slot, snap, ContainerInventory)` (wipe-and-replay bez tego itemu) + `writeContainerLayout(slot, snap, ContainerStorage)` (wipe-and-replay z tym itemem na końcu kolejki edytowalnych).
- `OriginalHandle` jest **zachowane** — `writeContainerLayout` pisze rekord docelowy z `it.OriginalHandle`, więc GaItem dalej żyje pod tym samym handle.
- **Brak explicit equipped guard** w workspace pre-flight — w `ApplyWorkspaceSave` widoczne są tylko: `Validate(snap)`, pending AoW pre-flight, capacity check, pass-through SlotIndex check. `needs verification`: czy `Validate(snap)` blokuje transfer założonych itemów, czy equipped item przeniesiony do Storage przez workspace save zostanie odrzucony.

---

## Storage → Inventory

Legacy core (`TransferToInventory`):
- Lustro Inventory → Storage **bez** equipped guarda (przedmioty w Storage nigdy nie są założone).
- Instance-move: dest empty slot → ten sam handle, oryginalna qty, nowy `Index` z `Inventory.NextAcquisitionSortId` (clamped powyżej `InvEquipReservedMax`).
- Quantity-merge: cap = `MaxInventory` z DB.

Workspace path:
- Identyczna mechanika jak Inventory → Storage, tylko `EditableItem.Container = ContainerInventory`.

---

## Inventory reorder and Storage reorder relation

Reorder (zmiana kolejności wewnątrz kontenera) i transfer (zmiana kontenera) korzystają z **różnych** ścieżek w obecnym UI:

| Operacja | Legacy binding | Workspace path |
|---|---|---|
| Inventory reorder | `App.ReorderInventory` (`app_inventory_order.go:255`) — stride-2 write, pełne testy w `app_inventory_order_test.go` | `workspace.moveItem(uid, 'inventory', pos)` w RAM, finalny write w `writeContainerLayout` ze stride-2 base z linii 566 |
| Storage reorder | `App.ReorderStorage` (`app_inventory_order.go:439`) — stride-2 write, testy w `app_storage_order_test.go` | `workspace.moveItem(uid, 'storage', pos)` analogicznie |
| Inventory ↔ Storage transfer | `App.MoveItemsBetweenInventoryAndStorage` → `core.MoveItemsBetweenContainers` | `workspace.transferItem(uid, target)` |

`SortOrderTab` w aktualnym kodzie **nie woła** ani `ReorderInventory`, ani `ReorderStorage`, ani `MoveItemsBetweenInventoryAndStorage`. Wszystkie operacje idą przez workspace. Bezpośrednie bindingi są zachowane jako publiczny App API i pełna powierzchnia testowa.

Stride-2 algorytm i bucket-collision guard — kanonicznie w [52](52-acquisition-sort-stride2.md). `writeContainerLayout` używa tej samej rodziny inwariantów (parzyste `baseAcq` ≥ `InvEquipReservedMax+2`, `acq = base + pos*2`, skip-over kolizji w `reservedAcq` z linii 573-585).

**Storage → Storage** i **Inventory → Inventory** reorder są w pełni wspierane przez workspace (drag wewnątrz kontenera). Nie ma kodu, który by je blokował.

---

## Stackable goods merge and partial moves

Dotyczy wyłącznie legacy core path (`core.MoveItemsBetweenContainers`). Workspace path nie ma odpowiednika cap-aware merge na poziomie save — workspace pracuje na pre-batched `EditableItem.Quantity` z UI, więc capa goods są zarządzane na poziomie Add (zob. [43](43-transactional-item-adding.md)).

### Cap-aware partial merge — `transferOne` quantity-merge branch

1. `cap = opts.DestCaps[handle]`. Brak wpisu lub `cap == 0` → `SkipReasonMissingCap` (**żaden** silent unbounded merge — żelazna zasada).
2. Dest ma rekord z tym samym handle:
   - `dstExistingQty >= cap` → `SkipReasonDestAtCap`, brak ruchu.
   - Inaczej: `transferQty = min(srcQty, cap - dstExistingQty)`. Aktualizuje qty docelowego i źródłowego.
   - Pozostała ilość po stronie źródła > 0 → result: `moved=true`, skip = `SkipReasonDestAtCap` z `MovedQty` i `RemainingQty` populated.
3. Dest nie ma rekordu → tworzy nowy z `transferQty = min(srcQty, cap)`. Reszta jak wyżej.

App wrapper rozwiązuje capy z DB:
```go
itemData, _ := db.GetItemDataFuzzy(itemID)
if dir == core.TransferToStorage { cap = itemData.MaxStorage } else { cap = itemData.MaxInventory }
caps[h] = cap
```

Instance-move handle (`0x80`/`0x90`/`0xA0`/`0xC0`) są **wyłączone** z `caps` (app.go:813). Wrapper celowo nie wpisuje `MaxInventory=1` dla nich, żeby kontrakt był jednoznaczny — instance-move ignoruje cap całkowicie.

---

## Instance-backed item transfer and rehandle

Instance-move dla handle prefiksów `0x80`/`0x90`/`0xA0`/`0xC0`:

### Empty dest slot — prosta ścieżka

`writeRecord(dst, handle, srcQty, newIndex)` + `clearRecord(src)`. Handle zostaje, GaItem zostaje, `Index` po stronie docelowej jest **świeży** (z `assignDestIndex` — clamp powyżej `InvEquipReservedMax`).

### Dest ma już ten sam handle — ścieżka rehandle (`materializeRehandledInstance`)

Dla talizmanów (`0xA0`) handle = `itemID | prefix`, więc dwa wpisy `AddItemsToSlot` dla tego samego itemu w Inventory i Storage produkują **dwa rekordy dzielące jeden handle**. Stara semantyka odrzucała transfer ("dest_duplicate"); aktualna **rehandluje**:

1. `materializeRehandledInstance(slot, oldHandle)`:
   - Resolve `itemID` z `slot.GaMap[oldHandle]`, fallback dla `0xA0`/`0xB0` przez bit-swap (`lower | 0x20000000` / `lower | 0x40000000`).
   - `generateUniqueHandle(slot, prefix)` — nowy globalnie unikalny handle z tym samym typowym prefiksem.
   - `allocateGaItem(slot, newHandle, itemID)` — nowy `GaItem` po stronie docelowej.
   - `slot.GaMap[newHandle] = itemID`.
2. `rebuildAfterAllocation(slot)` — `RebuildSlotFull` + `parseFromData`, odświeżenie dynamicznych offsetów. Snapshotuje `slot.GaMap` przed rebuildem i scala stackable handle-encoded entries z powrotem (mirror pattern z [43](43-transactional-item-adding.md)/writer.go).
3. Po rebuildzie: re-scan źródłowego rekordu (offsety mogły się przesunąć), `writeRecord(dst, newHandle, srcQty, newIndex)`, `clearRecord(src)`.

Skip `SkipReasonHandleAllocFailed` przy błędach alokacji. Allocator invariants — kanonicznie w [35](35-gaitem-allocator-invariants.md).

### Workspace path — brak rehandle

`writeContainerLayout` używa `it.OriginalHandle` jako handle docelowego. Jeżeli ten sam item był jednocześnie w obu kontenerach w workspace baseline, to każda strona miała swój `EditableItem.OriginalHandle` (od momentu utworzenia sesji). Nie ma scenariusza "duplicate handle on dest during transfer", bo wipe-and-replay re-emituje obie strony jednocześnie. **needs verification**: czy baseline workspace zawsze rozróżnia takie pary (np. dwa talizmany tego samego itemu w Inv i Storage) bez kolizji UID/handle.

---

## Equipped guard

Legacy core path:
- `IsHandleEquipped(slot, handle)` skanuje `slot.EquipItemsIDOffset .. +ChrAsmEquipmentSize` (`backend/core/transfer.go:592`).
- Kandydaci dopasowania: handle wprost, lower 28 bitów, `lower | 0x80000000`, `GaMap[handle]`, `GaMap[handle] | 0x80000000`, oraz prefix-swaps dla talizmanów (`0xA0 → 0x20`) i goods (`0xB0 → 0x40`).
- Aktywny tylko dla `TransferToStorage`. Storage → Inventory nie sprawdza equipped (zał. items w Storage nie są założone).
- Brak `EquipItemsIDOffset` → guard zwraca `false` (nie blokuje).

Workspace path:
- **Brak** explicit equipped check w `ApplyWorkspaceSave` ani `writeContainerLayout`. `needs verification`: czy `Validate(snap)` raportuje equipped-item-transferred warning/error. Empirycznie ze zrzutu kodu pre-flight nie widać.
- **needs verification**: zachowanie po przeniesieniu założonego talizmanu/broni przez workspace UI — czy gra przerejestruje slot ekwipunku, czy zwróci błąd renderingu.

---

## Capacity and caps

| Limit | Wartość | Egzekwowane przez |
|---|---|---|
| Inventory `CommonItems` slots | `CommonItemCount` | `containerBinary` (legacy), `writeContainerLayout` pre-flight (workspace) |
| Storage `CommonItems` slots | `StorageCommonCount` | jw. |
| Goods per-handle `MaxInventory` | DB | App wrapper przy budowie `caps` |
| Goods per-handle `MaxStorage` | DB | jw. |
| Instance-move qty per record | 1 | konwencja (rekord trzyma `qty=1` dla instance) |

Pre-flight capacity check w workspace path (`backend/editor/save.go:115-122`):
```go
if invTotal := len(snap.InventoryItems) + len(snap.UnsupportedInventoryRecords); invTotal > core.CommonItemCount { ...reject... }
if stoTotal := len(snap.StorageItems) + len(snap.UnsupportedStorageRecords); stoTotal > core.StorageCommonCount { ...reject... }
```
Workspace **nie sprawdza** per-handle goods cap przed save — capa są domeną Add (zob. [43](43-transactional-item-adding.md)) i UI ogranicza wprowadzane ilości przy `addItem`/`updateQuantity`.

---

## Save / discard workflow

```
[user UI action]            [workspace state]                [save file]
──────────────              ──────────────────               ──────────
drag/drop reorder      ──▶  workspace.moveItem    ──▶  RAM (snap.InventoryItems / StorageItems)
drag cross-grid        ──▶  workspace.transferItem ─▶  RAM (it.Container = target)
Add Item modal         ──▶  workspace.addItem     ──▶  RAM (Source=Added, OriginalHandle=0)
Remove                 ──▶  workspace.removeItem  ──▶  RAM (item dropped from snapshot)
WeaponEditModal        ──▶  workspace.updateWeapon ─▶  RAM (PendingUpgrade/Infusion/AoW flags)

[Save changes click]
   confirm modal
   workspace.save()    ──▶  App.SaveInventoryWorkspaceChanges(sessionID)
                              ├─ rollback := core.SnapshotSlot(slot)
                              ├─ a.pushUndo(charIdx)
                              ├─ editor.ApplyWorkspaceSave(slot, snap, baseline)
                              │   ├─ Pre-flight: Validate, capacity, pass-through SlotIndex, pending AoW
                              │   ├─ executeAdds        — core.AddItemsToSlotBatch dla Source=Added
                              │   ├─ executeWeaponPatches — core.PatchWeaponItemID dla upgrade/infusion
                              │   ├─ executePendingAoWPatches — core.PatchWeaponAoW / PatchWeaponAoWHandle
                              │   ├─ writeContainerLayout(ContainerInventory) — wipe-and-replay
                              │   ├─ writeContainerLayout(ContainerStorage) — wipe-and-replay
                              │   └─ ReconcileInventoryHeader / ReconcileStorageHeader
                              └─ on error: core.RestoreSlot(slot, rollback) (sesja Dirty=true)
                          ──▶ slot.Data

[Discard changes click]
   confirm modal
   workspace.discard() ──▶  restart sesji z aktualnego save (RAM resetowane)
```

- `Save changes` jest **zablokowane**, jeżeli `validation.errors.length > 0`.
- `dirty` flag jest globalna dla sesji — nie istnieje per-side `dirty`. Każda zmiana (reorder w jednym kontenerze, transfer, add, remove) zapala jedną flagę.

---

## Validation and rollback caveats

### Legacy core path

- **Bez snapshot/rollback**. `MoveItemsBetweenContainers` mutuje `slot.Data` w pętli. Jeżeli middle-of-batch handle wyjdzie z `SkipReasonHandleAllocFailed`, wcześniejsze udane transfery **zostają**. To z założenia — `TransferResult` raportuje per-handle outcome.
- App wrapper robi `pushUndo(charIdx)` PRZED core call (gdy dry-check znalazł choć jeden valid handle), więc cały batch jest reversible przez Undo — ale nie jest atomicznie odrzucany przez core.
- Defensive `ReconcileInventoryHeader` / `ReconcileStorageHeader` + `rescanStorageList` / `rescanInventoryList` na końcu batcha — fixup, nie rollback.

### Workspace path

- `ApplyWorkspaceSave` **wymaga zewnętrznego rollbacku**. Atomicity contract z docstring (`backend/editor/save.go:70-78`):
  > Callers MUST snapshot slot via `core.SnapshotSlot` BEFORE calling this function and call `core.RestoreSlot` on a non-nil error to roll back partial state. This function does NOT manage its own undo. It only guarantees all rejection checks run BEFORE any mutation; if a check fails, slot.Data is byte-identical to the input. Once writes begin (after the rejection block), an error means slot.Data has been partially mutated. Caller MUST roll back.
- Pre-flight rejection (przed jakąkolwiek mutacją): `Validate(snap)`, brak baseline, pending AoW unknown/incompatible, capacity overflow, pass-through SlotIndex collision/range.
- **App-level caller egzekwuje kontrakt**: `App.SaveInventoryWorkspaceChanges` (`app_inventory_session.go:192-228`) bierze `rollback := core.SnapshotSlot(slot)` przed `pushUndo` + `ApplyWorkspaceSave`, i wywołuje `core.RestoreSlot(slot, rollback)` zarówno gdy `ApplyWorkspaceSave` zwróci błąd, jak i gdy post-save snapshot rebuild zawiedzie. Sesja pozostaje `Dirty=true` po nieudanym save, co zachowuje pending edits dla retry.
- W odróżnieniu od ścieżki "Add Items" (zob. [43](43-transactional-item-adding.md)), gdzie cała transakcja ma własne wewnętrzne snapshot/restore i `ValidatePostMutation`, workspace save **nie ma** post-mutation validation — wyłącznie pre-flight + zewnętrzny rollback po stronie App callera. **needs verification**: czy planowany jest dodatek post-mutation validacji analogicznej do 43.

---

## UI counters and allocator caveats

- `ColumnHeader` (`SortOrderTab.tsx:931`) pokazuje `count` = `view.length` per aktywna zakładka, NIE total kontenera.
- `selectedCount` jest również per-tab — `invSelectedHere = inventoryView.filter(it => invSelectedUIDs.has(it.uid))`. UIDs poza widoczną zakładką pozostają w selekcji ale nie są liczone w nagłówku.
- `useEffect` przy zmianie zakładki (`SortOrderTab.tsx:148-159`) czyści UIDs które wypadły z `visible` — stale selections nie blokują batch operations po przełączeniu zakładki.
- Bottom bar pokazuje `Session ID: {sessionID || '—'}` — zmienna sesji od `workspace.start(charIdx)`.

Allocator-related caveats (sumarycznie — szczegóły w [35](35-gaitem-allocator-invariants.md)):
- Każdy rehandle w legacy core path alokuje nowy handle + nowy GaItem. Dla batcha N duplikatów talizmanów to N alokacji sekwencyjnie. **needs verification**: benchmark dla duży preset import (np. >5 duplikatów AoW jednocześnie).
- Workspace path **nigdy nie rehandluje** — zachowuje `OriginalHandle`. Brak presji na alokator przy transferze workspace.

---

## Test coverage

Backend testy pokrywające transfer + ordering:

| Klasa | Reprezentatywne testy | Lokalizacja |
|---|---|---|
| Instance-move oba kierunki | `TestMoveNonStackableInvToStorage`, `TestMoveNonStackableStorageToInv` | `tests/transfer_test.go` |
| Quantity-merge + cap-aware partial | `TestMoveStackableInvToStorage_MergePartialAtCap`, `TestMoveStackableStorageToInv_PartialAtCap`, `TestMoveStackableInvToStorage_NoDest`, `TestMoveStackableStorageToInv_Merge` | `tests/transfer_test.go` |
| Missing cap / dest full | `TestMoveStackableMissingCap`, `TestMoveDestFull` | `tests/transfer_test.go` |
| Equipped guard | `TestMoveEquippedInvToStorageSkipped` | `tests/transfer_test.go` |
| Invalid / mieszany batch | `TestMoveInvalidHandle`, `TestMoveMixedValidInvalid` | `tests/transfer_test.go` |
| Header reconcile + brak orphana GaItem | `TestMoveHeadersReconciled`, `TestMoveNoOrphanedGaItem` | `tests/transfer_test.go` |
| Round-trip (write + reload) | `TestMoveRoundTripSave` | `tests/transfer_test.go` |
| Rehandle (talizman) | `TestMoveTalismanDestHasSameHandle_RehandlesAndMoves`, `TestMoveTalismanStorageToInventory_DuplicateHandle_RehandlesAndMoves`, `TestMoveTalismanInvToEmptyStorage_PhysicalMove`, `TestMoveTalismanStorageToEmptyInventory_PhysicalMove` | `tests/transfer_test.go` |
| Duplikat broni/zbroi dozwolony | `TestMoveWeaponAllowsDuplicateSameItemIDOnDestination`, `TestMoveArmorAllowsDuplicateSameItemIDOnDestination` | `tests/transfer_test.go` |
| Goods duplicate handle (cap merge) | `TestMoveGoodsDuplicateHandle_StillQuantityMerge` | `tests/transfer_test.go` |
| Widoczność w VM po rehandle | `TestTransferTalismanVisibleInVM_InvToStorage`, `TestTransferTalismanVisibleInVM_StorageToInv` | `tests/transfer_test.go` |
| Storage order uses record Index | `TestStorageOrderUsesRecordIndex` | `tests/transfer_test.go` |
| Świeży transfer pojawia się na końcu acquisition | `TestTransferInvToStorageAppearsAtEndByAcquisition`, `TestTransferStorageToInvAppearsAtEndOfInventory` | `tests/transfer_test.go` |
| Pełne reorder validation surface (Inventory) | `TestReorderWeaponInventory_*`, `TestReorderInventory_*` | `app_inventory_order_test.go` |
| Storage reorder validation + invariants | `TestReorderStorage_*`, `TestInventoryReorder_DoesNotTouchStorage` | `app_storage_order_test.go` |

Workspace save end-to-end coverage (`editor.ApplyWorkspaceSave` + `writeContainerLayout`) — **needs verification**: pełna lista testów `backend/editor/*_test.go` i czy obejmują scenariusze cross-container transfer + workspace save end-to-end z reloadem.

---

## Known limits / needs verification

- **Storage Apply / transfer in-game (Steam Deck)**: brak świeżego raportu, że po `workspace.save()` z transferem cross-grid skrzynia w grze odpowiada preview edytora dla każdej zakładki Sort Order. Cross-ref do [52](52-acquisition-sort-stride2.md) — wspólny gap dla całego pipeline'u stride-2.
- **Equipped guard w workspace path**: nie ma explicit checku w `ApplyWorkspaceSave` / `writeContainerLayout`. `needs verification` czy `Validate(snap)` blokuje transfer założonych itemów i jak gra renderuje slot ekwipunku po takim transferze.
- **Workspace post-mutation validation**: brak — w odróżnieniu od [43](43-transactional-item-adding.md). Rollback realizowany w App wrapperze (`SaveInventoryWorkspaceChanges`) przez `SnapshotSlot`/`RestoreSlot`, ale po stronie save itself nie ma post-write sanity check. `needs verification` czy jest planowane.
- **Batch rehandle performance**: legacy core alokuje sekwencyjnie. `needs verification` dla scenariuszy ≥5 duplikatów talizmanu/AoW jednocześnie (preset import).
- **Batch order w cross-grid transfer**: workspace path iteruje `Array.from(selection)` — kolejność Set iteration, nie visible-grid. Funkcjonalnie OK (każdy transfer ląduje na końcu acquisition po stronie docelowej), ale nie jest udokumentowane jako stabilne.
- **Equipped detection NG+ / summon / menu transient state**: testowane na pojedynczym fixturze. `needs verification` dla szerszego pokrycia.
- **Workspace baseline collision dla par instance items**: ten sam talizman po obu stronach w baseline. `needs verification` czy UID rozdziela je niezawodnie i `writeContainerLayout` nie wpisze tego samego handle dwukrotnie.

---

## Cross-references

- [03 — GaItem map](03-gaitem-map.md) — handle ↔ itemID, prefiksy typów.
- [07 — Inventory model](07-inventory.md) — read-side rekord 12B, offsety `CommonItems`, `CommonItemCount`.
- [10 — Storage model](10-storage.md) — read-side rekord 12B, `StorageBoxOffset`, `StorageHeaderSkip`, `StorageCommonCount`.
- [35 — GaItem allocator invariants](35-gaitem-allocator-invariants.md) — `generateUniqueHandle`, `allocateGaItem`, `RebuildSlotFull`, `parseFromData` invariants.
- [39 — Inventory reorder (historical)](39-inventory-reorder.md) — historyczny design note dla per-side Apply / sort dropdownów (nie wdrożone w opisanej formie).
- [43 — Transactional item adding](43-transactional-item-adding.md) — cap-aware semantyka Add, `SnapshotSlot`/`RestoreSlot`/`ValidatePostMutation` jako kontrast z workspace save.
- [52 — Acquisition stride-2 sort order](52-acquisition-sort-stride2.md) — algorytm stride-2, bucket-collision guard, ścieżka write w `ReorderInventory`/`ReorderStorage` i w `writeContainerLayout`.

---

## Sources

- `backend/core/transfer.go` (720 linii) — core engine, instance-move, quantity-merge, rehandle, equipped guard.
- `app.go:741+` — App-level wrapper `MoveItemsBetweenInventoryAndStorage`, resolve capów, push undo.
- `backend/editor/save.go:79+, :518+` — workspace save (`ApplyWorkspaceSave`) i layout writer (`writeContainerLayout`).
- `frontend/src/components/SortOrderTab.tsx` — dwukolumnowy UI, workspace session jako jedyny tryb operacji.
- `frontend/src/hooks/useInventoryWorkspace.ts` — operacje RAM-only.
- `tests/transfer_test.go` — 28 testów pokrywających legacy core path.
- `app_storage_order_test.go`, `app_inventory_order_test.go` — testy reorder bindings.
- `docs/CHANGELOG.md` — wpisy "feat(sort-order): dual-grid Inventory + Storage with bidirectional transfer".
