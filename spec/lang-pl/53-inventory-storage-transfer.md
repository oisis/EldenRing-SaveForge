# 53 — Transfer Inventory ↔ Storage i dwukolumnowa zakładka Sort Order

> **Typ**: Design doc
> **Status**: ✅ Zaimplementowany
> **Zakres**: Dwukolumnowa zakładka Sort Order (Storage + Inventory), dwukierunkowy drag-and-drop transfer (single i multi), niezależne preview/apply po obu stronach, oraz ścieżka rehandle dla duplikowanych instancji, dzięki której ten sam talizman / broń / zbroja może istnieć w obu pojemnikach.

---

## A. Cel

Zakładka Sort Order renderuje skrzynię i ekwipunek aktywnej postaci obok siebie i pozwala graczowi przenosić itemy między nimi, sortować każdą stronę osobno i utrwalać dowolną kolejność do save:

- **Layout**: Storage po lewej, Inventory po prawej; identyczne ramki 5×6, identyczna semantyka kafelków.
- **Transfer**: drag-and-drop w obu kierunkach (Inventory → Storage i Storage → Inventory), single albo multi-batch.
- **Sortowanie**: każda strona ma własny dropdown (Acquisition ↑/↓, Weight ↑/↓, Type ↑/↓) działający na własnym preview.
- **Apply Order**:
    - *Inventory Apply* → przepisuje acquisition order tylko dla Inventory.
    - *Storage Apply* → przepisuje acquisition order tylko dla Storage.
    - Żaden Apply nie dotyka drugiego pojemnika.

Funkcja jest dostępna pod zakładką `Sort Order` edytora postaci i operuje per kategoria Sort Order (`weapons`, `talismans`, `head`, `chest`, `arms`, `legs`).

---

## B. Model Danych

### Prefiks handle — typ, nie lokalizacja

Najwyższy bajt handle `GaItem` koduje **typ przedmiotu**, nigdy pojemnik. Inventory i Storage dzielą tę samą przestrzeń handle oraz tablicę GaItem.

| Maska prefiksu | Typ przedmiotu | Semantyka przenoszenia |
|---|---|---|
| `0x80` | Broń | instance-move |
| `0x90` | Zbroja | instance-move |
| `0xA0` | Akcesorium / Talizman | instance-move |
| `0xB0` | Goods | quantity-merge (stackable) |
| `0xC0` | Ash of War | instance-move |

### Rekord 12-bajtowy

Inventory i Storage układają listy itemów jako stałe rekordy 12-bajtowe:

```
offset  size  pole
0       u32   handle (GaItem handle)
4       u32   quantity (1 dla instance items, stack count dla goods)
8       u32   Index (per-rekord acquisition index, używany przez sortowanie in-game)
```

Inventory zaczyna się od `slot.MagicOffset + InvStartFromMagic` (×`CommonItemCount`); Storage od `slot.StorageBoxOffset + StorageHeaderSkip` (×`StorageCommonCount`).

### Współdzielone GaItem / GaMap

`slot.GaItems` (tablica `GaItemFull`) i `slot.GaMap` (`handle → itemID`) opisują instancje przedmiotów na **poziomie save**, nie per pojemnik. Transfer **nigdy** nie może usuwać wpisów GaItem ani GaMap: ten sam handle dalej identyfikuje tę samą instancję po stronie docelowej, niosąc poziom ulepszenia, infuzję, podpięty AoW i ewentualne metadane rehandlingu.

---

## C. Semantyka transferu

Główne entry-point core'a:

```go
core.MoveItemsBetweenContainers(slot, handles, direction, opts) (TransferResult, error)
```

gdzie `direction ∈ {TransferToStorage, TransferToInventory}`, a `opts.DestCaps` niesie capy ilości per handle (`MaxInventory` / `MaxStorage` z DB) dla cap-aware moves. Wrapper App-level `App.MoveItemsBetweenInventoryAndStorage(charIdx, handles, direction)` rozwiązuje capy i woła core.

### Instance-move (broń, zbroja, akcesorium, AoW)

- Rekord źródłowy jest czyszczony (`handle=0, qty=0, Index=0`).
- Pusty slot po stronie docelowej otrzymuje **ten sam handle**, oryginalne `Quantity`, i świeżo przypisany `Index` (monotonicznie z `NextEquipIndex` / `NextAcquisitionSortId` po stronie docelowej).
- Capy są ignorowane (każdy rekord trzyma dokładnie 1).

### Quantity-merge (goods, stackable)

- Jeśli rekord z tym samym handle już istnieje po stronie docelowej, ilości są mergowane.
- Cap-aware partial moves emitują `SkipReasonDestAtCap` z `movedQty` / `remainingQty` w odpowiedzi, więc UI może raportować "moved X, Y stayed".
- Brakujące capy (`DestCaps[handle] == 0` dla stackable) odrzucają move z `SkipReasonMissingCap` — żadnego cichego unbounded merge.

### Ścieżka rehandle dla duplikowanej instancji

Dla handle instance-move ten sam handle już istniejący po stronie docelowej wcześniej blokował transfer. Aktualna ścieżka **rehandluje** instancję:

1. Alokuje nowy, globalnie unikalny handle z tym samym prefiksem typu.
2. Tworzy nowy wpis `GaItem` po stronie docelowej niosący metadane instancji źródłowej (upgrade, infuzja, handle podpiętego AoW, …).
3. Rejestruje nowy mapping handle → itemID w `GaMap`.
4. Zapisuje rekord źródłowy jako wyczyszczony, a rekord docelowy z nowym handle.

To właśnie pozwala graczowi zachować **ten sam talizman / broń / zbroję** zarówno w Storage, jak i w Inventory po transferze — instancja docelowa to osobny GaItem, nie duplikat źródłowego.

### Strażnik equipped-item

Transfery Inventory → Storage odrzucają każdy handle aktualnie wskazywany przez `ChrAsmEquipment` ze `SkipReasonEquipped`. Gracz musi zdjąć przedmiot przed transferem. W odwrotnym kierunku nie ma sprawdzenia equipped (itemy ze storage nigdy nie są założone).

### Kształt wyniku

```go
type TransferResult struct {
    Moved   int            // liczba rekordów przeniesionych (w tym partial-stack moves)
    Skipped []TransferSkip // powody skipów per handle
}

type TransferSkip struct {
    Handle       uint32
    Reason       string // "equipped" | "dest_full" | "dest_at_cap" | "missing_cap" | …
    MovedQty     uint32 // tylko partial-cap moves
    RemainingQty uint32 // tylko partial-cap moves
}
```

Funkcja **nigdy** nie partial-fail'uje batcha: niepoprawne handle trafiają do `Skipped`, poprawne są przetwarzane niezależnie. Po batchu binarne nagłówki `common_item_count` są uzgadniane przez `ReconcileInventoryHeader` / `ReconcileStorageHeader`.

---

## D. Zachowanie frontendu

### Niezależne selekcje

Inventory i Storage trzymają rozłączny stan selekcji:

| Strona | Zbiór selekcji | Anchor handle | Flaga block-drag |
|---|---|---|---|
| Inventory | `selectedHandles: Set<number>` | `anchorHandle` | `isBlockDragging` |
| Storage | `storageSelectedHandles: Set<number>` | `storageAnchorHandle` | `storageBlockDragging` |

Modyfikatory selekcji (te same po obu stronach):

- **Plain click** — wybiera jeden, ustawia anchor.
- **Ctrl/Cmd click** — toggle przynależności, anchor = klikany handle.
- **Shift click** — zakres od anchor do klikanego handle w aktualnej widocznej kolejności.

### Wybór batcha przy drag/drop transferze

Przy starcie draga handle przeciągany jest sprawdzany względem selekcji po stronie źródłowej. Rozstrzygnięty batch dla cross-container dropa:

- Jeśli przeciągany handle jest w selekcji źródłowej **i** selekcja źródłowa ma więcej niż jeden wpis → batch = wszystkie zaznaczone source handles w widocznej kolejności.
- W przeciwnym razie → batch = `[draggedHandle]`.

Drop na przeciwną ramkę uruchamia `App.MoveItemsBetweenInventoryAndStorage(charIdx, handles, direction)`.

### Strażnicy niezapisanego preview

UI utrzymuje parę base/preview po każdej stronie i wyprowadza `hasChanges` / `storageHasChanges` z porównania. Trzy guardy zapobiegają utracie niezapisanego preview:

| Akcja | Guard | Toast |
|---|---|---|
| Cross-transfer (dowolny kierunek) | `hasChanges \|\| storageHasChanges` | `"Apply or reset the current order before transferring items."` |
| Inventory Apply | `storageHasChanges` | `"Apply or reset Storage order before applying Inventory order."` |
| Storage Apply | `hasChanges` | `"Apply or reset Inventory order before applying Storage order."` |

Po Apply po którejkolwiek stronie ta strona reloaduje z backendu (`GetInventoryOrder` / `GetStorageOrder`), a druga zachowuje aktualną parę base/preview.

### Dropdowny i Apply per strona

Każda strona ma:

- **Sort dropdown** — sortuje tylko preview tej strony przez `sortByMode`; ręczny drag przełącza `sortMode` strony na `'custom'`.
- **Reset Preview** — przywraca preview strony z base.
- **Apply Order** — otwiera dedykowany confirm modal; po potwierdzeniu woła `ReorderInventory` / `ReorderStorage`, potem reloaduje tę stronę.

Banner "Dragging N items" pojawia się nad tą ramką, która jest aktualnie źródłem aktywnego multi-draga.

---

## E. Sortowanie

### Inventory

- `App.GetInventoryOrder(charIdx, tab)` czyta inwentarz `CommonItems`, filtruje po kategorii Sort Order, zwraca itemy posortowane po `Index` rosnąco.
- `App.ReorderInventory(charIdx, tab, orderedHandles)` przepisuje wyłącznie `slot.Data[off+8:]` (per-rekord `Index`) używając przypisania **stride-2** z parzystym base powyżej `InvEquipReservedMax`. Uzasadnienie i dowód w [spec/52](52-acquisition-sort-stride2.md): gra kubełkuje sortowanie Acquisition Order po `acqIdx >> 1`, więc stride-1 zamienia sąsiednie pary.

### Storage

- `App.GetStorageOrder(charIdx, tab)` czyta `slot.Data[StorageBoxOffset + StorageHeaderSkip .. ]` i sortuje po wewnętrznym `Index` rosnąco — prawdziwy storage acquisition index, nie pozycja binarna.
- `App.ReorderStorage(charIdx, tab, orderedHandles)` lustrzanie powiela ścieżkę Inventory na `slot.Storage.CommonItems` i `slot.Storage.NextEquipIndex` / `NextAcquisitionSortId`:
    - Ta sama walidacja: pełna lista, brak duplikatów, zgodność kategorii, brak placeholderów technicznych (np. Unarmed).
    - To samo przypisanie **stride-2**. Stride-2 jest tu poprawny z tego samego powodu — przeglądarka storage w grze używa tego samego kubełkowania `acqIdx >> 1` co inventory.
    - `pushUndo` przed jakąkolwiek mutacją, monotoniczny postęp counterów, defensywne `ReconcileStorageHeader` na końcu.
    - **Brak zapisu** do bajtów ani counterów inventory.

### Przypisanie Index przy transferze

`MoveItemsBetweenContainers` przypisuje rekordowi docelowemu `Index` z monotonicznych counterów po stronie docelowej (`NextEquipIndex` / `NextAcquisitionSortId`), gwarantując że świeżo przeniesiony item pojawia się na końcu Acquisition ↑ siatki docelowej. Po transferze `GetStorageOrder` / `GetInventoryOrder` natychmiast to odzwierciedlają.

---

## F. Pokrycie testowe

Testy backend pokrywają całą powierzchnię transfer + ordering. Główne klasy:

| Klasa | Reprezentatywne testy | Lokalizacja |
|---|---|---|
| Instance-move (oba kierunki) | `TestMoveNonStackableInvToStorage`, `TestMoveNonStackableStorageToInv` | `tests/transfer_test.go` |
| Quantity-merge + cap-aware partial | `TestMoveStackableInvToStorage_MergePartialAtCap`, `TestMoveStackableStorageToInv_PartialAtCap`, `TestMoveStackableMissingCap`, `TestMoveDestFull`, `TestMoveStackableStorageToInv_Merge` | `tests/transfer_test.go` |
| Strażnik equipped | `TestMoveEquippedInvToStorageSkipped` | `tests/transfer_test.go` |
| Invalid / mieszany / duplicate batch | `TestMoveInvalidHandle`, `TestMoveMixedValidInvalid` | `tests/transfer_test.go` |
| Header reconcile + zachowanie GaItem | `TestMoveHeadersReconciled`, `TestMoveNoOrphanedGaItem` | `tests/transfer_test.go` |
| Round-trip (write + reload) | `TestMoveRoundTripSave` | `tests/transfer_test.go` |
| Rehandle duplikat talizmanu | `TestMoveTalismanDestHasSameHandle_RehandlesAndMoves`, `TestMoveTalismanStorageToInventory_DuplicateHandle_RehandlesAndMoves`, `TestMoveTalismanInvToEmptyStorage_PhysicalMove`, `TestMoveTalismanStorageToEmptyInventory_PhysicalMove` | `tests/transfer_test.go` |
| Duplikat broni / zbroi dozwolony | `TestMoveWeaponAllowsDuplicateSameItemIDOnDestination`, `TestMoveArmorAllowsDuplicateSameItemIDOnDestination` | `tests/transfer_test.go` |
| Duplikat goods nadal merge'uje | `TestMoveGoodsDuplicateHandle_StillQuantityMerge` | `tests/transfer_test.go` |
| Widoczność w VM po rehandle | `TestTransferTalismanVisibleInVM_InvToStorage`, `TestTransferTalismanVisibleInVM_StorageToInv` | `tests/transfer_test.go` |
| Storage order czyta record Index | `TestStorageOrderUsesRecordIndex` | `tests/transfer_test.go` |
| Świeżo przeniesiony item na końcu acquisition | `TestTransferInvToStorageAppearsAtEndByAcquisition`, `TestTransferStorageToInvAppearsAtEndOfInventory` | `tests/transfer_test.go` |
| ReorderStorage odrzuca złe wejście | `TestReorderStorage_RejectsMissingHandle`, `TestReorderStorage_RejectsDuplicateHandle`, `TestReorderStorage_RejectsIncompleteList`, `TestReorderStorage_RejectsHandleFromInventory` | `app_storage_order_test.go` |
| ReorderStorage trwale zapisuje order | `TestReorderStorage_PersistsAcquisitionOrder`, `TestReorderStorage_DoesNotTouchHandlesOrQty`, `TestReorderStorage_RoundTripReread` | `app_storage_order_test.go` |
| Izolacja między pojemnikami | `TestReorderStorage_DoesNotTouchInventory`, `TestInventoryReorder_DoesNotTouchStorage` | `app_storage_order_test.go` |
| Pełna powierzchnia Inventory reorder | `TestReorderWeaponInventory_*`, `TestReorderInventory_*` | `app_inventory_order_test.go` |

---

## G. Znane Ograniczenia / Future Work

- **Manualna weryfikacja in-game** wciąż wymagana dla każdej zakładki Sort Order (talizmany, bronie, części zbroi) na prawdziwym save wdrożonym na Steam Deck / konsoli. Automatyczne testy pokrywają poprawność binarną, ale nie renderowanej siatki w grze.
- **Wykrywanie equipped** opiera się na referencjach `ChrAsmEquipment`. Przypadki brzegowe (np. ekwipunek wymieniony w przejściowym stanie menu, summony, dziedziczenie NG+) mogą wymagać szerszej walidacji na większej liczbie fixturów save.
- **Wydajność batch rehandle**: każda duplikowana instancja aktualnie alokuje nowy handle + nowy GaItem sekwencyjnie. Dla multi-transferu wielu duplikowanych talizmanów to O(N) alokacji handle; profiling i batched alokacja mogą się przydać, jeśli duży preset import stanie się realnym workflow.
- **Storage Apply — sanity check in-game**: deploy na Steam Deck → uruchom grę → zweryfikuj że "Acquisition Order ↑" w skrzyni odpowiada preview edytora dla każdej zakładki Sort Order. Ta sama procedura jest już używana dla Inventory (spec/52).
- **Layout drag/drop Storage**: siatka renderuje obecnie 5×6 = 30 kafelków z pionowym overflow. Dla magazynów ze setkami itemów scroll działa, ale nie jest idealny; przyszła praca może dodać chunked rendering / wirtualizację (przekrojowe z ogólnym backlogiem inventory virtualization w `docs/ROADMAP.md`).

---

## Lokalizacja Implementacji

- `app_inventory_order.go` — `GetInventoryOrder`, `GetStorageOrder`, `ReorderInventory`, `ReorderStorage`
- `app.go` — `MoveItemsBetweenInventoryAndStorage` (wrapper App-level, rozwiązywanie capów, push undo)
- `backend/core/transfer.go` — `MoveItemsBetweenContainers`, ścieżka rehandle, header reconcile
- `frontend/src/components/SortOrderTab.tsx` — dwukolumnowy layout, stan per strona, drag/drop, guardy, flow Apply

---

## Źródła

- spec/39: oryginalny projekt Inventory Reorder
- spec/52: odkrycie i dowód stride-2
- Empiryczne testy in-game na Steam Deck (prawdziwy save PS4 wdrożony przez SSH)
- `tests/transfer_test.go` — szerokie pokrycie duplikatów instancji i widoczności w VM
