# 35 — GaItem Allocator & Invariants

> **Typ**: Binary format spec + design doc
> **Status**: ✅ canonical, implemented (oparte o aktualny kod backend; ostatnia weryfikacja przeciw branchowi `docs/lang-pl-book-cleanup`)
> **Zakres**: Write-side model alokacji wpisów `GaItem` — capacity, countery, strefy, sentinele, walidacja post-mutacji, snapshot/rollback, transakcyjność batch add. Rozdział uzupełnia [03-gaitem-map](03-gaitem-map.md), które opisuje wyłącznie binary layout. Tutaj opisana jest semantyka edytora *nad* tym layoutem.

---

## Cel rozdziału

Save Forge musi dokładać i przenosić wpisy w tablicy `slot.GaItems` w sposób, który gra zaakceptuje przy ładowaniu. To wymaga przestrzegania kilku invariantów, które nie wynikają wprost z binary layoutu: kursory ramek (`NextAoWIndex`, `NextArmamentIndex`), globalnego licznika handle (`NextGaItemHandle`), pojemności zależnej od wersji slotu (5118 vs 5120 wpisów) i zakazu współdzielenia handle. Rozdział spina te reguły, opisuje allocator (`allocateGaItem`), capacity check (`CheckAddCapacity`), validator (`ValidatePostMutation`) i transakcyjny snapshot/rollback (`SnapshotSlot`/`RestoreSlot`).

Co rozdział **NIE** robi:
- Nie opisuje pełnej semantyki Ash of War — to [54-ash-of-war](54-ash-of-war.md).
- Nie opisuje transferu Inventory ↔ Storage — to [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- Nie projektuje napraw UI capacity bar — wskazuje jedynie istniejący gap (sekcja "UI counters vs allocator capacity"), naprawa jest poza Phase 2.

---

## Source of truth w kodzie

| Temat | Pliki / funkcje | Notatka |
|---|---|---|
| Pole `slot.Version` | `backend/core/structures.go:223`, czytane w `structures.go:281` | u32 na offset 0x00 surowych danych slotu; `Version == 0` = pusty slot |
| Capacity threshold | `backend/core/offset_defs.go:346-348` | `GaItemCountOld = 5118 (0x13FE)`, `GaItemCountNew = 5120 (0x1400)`, `GaItemVersionBreak = 81` |
| Wybór capacity przy load | `backend/core/structures.go:623-628` (`scanGaItems`) | jedyne miejsce, gdzie kod konsultuje `slot.Version` przy alokacji tablicy |
| Rekonstrukcja counterów | `backend/core/structures.go:681-722` (drugi pass `scanGaItems`) | deterministyczna funkcja stanu `GaItems`; brak fallbacku |
| Allocator | `backend/core/writer.go:422-501` (`allocateGaItem`) | type-segregated placement, shift-right przy konflikcie, AoW guard po commicie `6881cb9` |
| Generator unikalnych handle | `backend/core/writer.go:402-419` (`generateUniqueHandle`) | iteruje do `MaxHandleAttempts = 10000` (`offset_defs.go:290`) |
| Capacity pre-flight | `backend/core/capacity.go:7-238` (`SlotUsage`, `CountSlotUsage`, `CheckAddCapacity`, `CapacityReport`) | mapy stackable/non-stackable na potrzebne sloty/wpisy |
| GaItemData (`upsertGaItemData`) | `backend/core/writer.go:11-...` | rejestracja metadanych broni; limit `GaItemDataMaxCount = 7000` |
| Walidacja post-mutacji | `backend/core/diagnostics.go:363-468` (`ValidatePostMutation`, `IntegrityError`) | 6 unikalnych check keys (gaitem_indices generuje 2 sub-violation messages); ostatnia linia obrony |
| Snapshot/rollback | `backend/core/snapshot.go:28-102` (`SlotSnapshot`, `SnapshotSlot`, `RestoreSlot`) | deep copy: `Data`, `GaItems`, `GaMap`, `Inventory`, `Storage`, wszystkie countery, wszystkie offsety dynamiczne |
| Batch add | `backend/core/writer.go:109` (`AddItemsToSlot`), `backend/core/writer.go:273` (`AddItemsToSlotBatch`) + `backend/core/slot_rebuild.go:300` (`RebuildSlotFull`) | jeden rebuild zamiast N |
| Repair orphan | `backend/core/writer.go:1015` (`RepairOrphanedGaItems`) | clear wpisów, których handle nie występuje w żadnym inventory/storage; idempotent, returns int |
| Repair duplicate index | `backend/core/inventory_index_repair.go::RepairDuplicateInventoryIndices` | re-przypisanie `Index` dla duplikatów w `Inventory.CommonItems` + `KeyItems` |
| AoW availability scanner | `backend/core/aow_availability.go::ScanAoWAvailability` | wykrywa współdzielony handle między dwoma broniami |
| UI capacity binding | `app_deploy.go:31-38, 289-306` (`SlotCapacity`, `GetSlotCapacity`) | eksponuje `non_empty / max` per kontener — nie `armament zone free` |
| Top-level orchestrator | `app.go:327-644` (`AddItemsToCharacter`) | pełen łańcuch pre-flight → snapshot → batch → post-flags → reconcile → validate → rollback-on-error |

---

## GaItem capacity by slot version

Pojemność tablicy `slot.GaItems` zależy od wersji konkretnego slotu, nie od save'a jako całości:

| `slot.Version` | `len(slot.GaItems)` | Stała w kodzie |
|---|---|---|
| `0` (pusty/nieużywany slot) | `GaItemCountNew = 5120` (warunek `Version > 0 && Version <= 81` nie jest spełniony) | wszystkie wpisy zostają empty |
| `1 .. 81` | `GaItemCountOld = 5118` | `GaItemVersionBreak = 81` |
| `> 81` | `GaItemCountNew = 5120` | nowsze save'y po game patchu |

Decyzja zapada **raz**, w `scanGaItems` (`structures.go:623-628`):

```go
maxEntries := GaItemCountNew
if s.Version > 0 && s.Version <= GaItemVersionBreak {
    maxEntries = GaItemCountOld
}
s.GaItems = make([]GaItemFull, maxEntries)
```

Wpisy poza zakresem danych w pliku (gdy sekcja kończy się wcześniej) wypełniane są wartością "empty" (`GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}`, `structures.go:632`).

> ℹ️ Stałe `5118` i `81` występują również w `tests/save_integrity_test.go:132-134`. Test celowo replikuje produkcyjną logikę dla regresji — każda zmiana progu wymaga aktualizacji testu razem z kodem.

---

## Runtime capacity rule

Po loadzie **żadna inna ścieżka kodu** nie czyta `slot.Version` przy operacjach na `GaItems`. Source-of-truth dla pojemności runtime to:

```go
len(slot.GaItems)
```

Ta reguła jest zaimplementowana spójnie:

| Plik:linia | Co używa |
|---|---|
| `writer.go:443` | `maxEntries := len(slot.GaItems)` w `allocateGaItem` |
| `writer.go:517` | `maxBuf := len(slot.GaItems) * GaRecordWeapon` w `FlushGaItems` (deprecated) |
| `slot_rebuild.go:318` | `maxBuf := len(slot.GaItems) * GaRecordWeapon` w `RebuildSlotFull` |
| `capacity.go:22` | `GaItemsMax: len(slot.GaItems)` w `CountSlotUsage` |
| `diagnostics.go:145` | log: `len(slot.GaItems)` |
| `diagnostics.go:453-456` | `if slot.NextArmamentIndex > len(slot.GaItems)` w `ValidatePostMutation` |
| `snapshot.go:40` | `gaItemsCopy = make([]GaItemFull, len(slot.GaItems))` |

**Implikacja dla implementatorów**: nigdy nie zakładaj `5120` po loadzie. Slot z `Version == 81` ma `len = 5118`; każdy hardcoded `5120` w nowej ścieżce kodu spowoduje out-of-bounds przy operacjach na takim slocie.

---

## Counter reconstruction on load

Po sparsowaniu wpisów `GaItems` parser przebiega tablicę po raz drugi (`structures.go:685-722`) i wylicza countery deterministycznie:

```go
s.NextAoWIndex = 0
s.NextArmamentIndex = 0
s.NextGaItemHandle = 0
s.PartGaItemHandle = 0x80 // default

maxGlobalCounter := uint32(0)
maxCounterIndex := 0

for i, g := range s.GaItems {
    if g.IsEmpty() { continue }
    h := g.Handle
    typeBits := h & GaHandleTypeMask

    // last AoW position + 1
    if typeBits == ItemTypeAow {
        s.NextAoWIndex = i + 1
    }

    // global handle counter — najwyższy 16-bitowy counter wśród wszystkich typów
    counter := h & 0xFFFF
    if counter >= maxGlobalCounter {
        maxGlobalCounter = counter
        maxCounterIndex = i
    }
}

s.NextArmamentIndex = maxCounterIndex + 1
s.NextGaItemHandle = maxGlobalCounter + 1
```

Cechy:
- **Brak fallbacku** — countery wynikają wyłącznie ze stanu `GaItems`.
- Pusty slot (`Version == 0`, wszystkie wpisy empty) → wszystkie countery `0`.
- `PartGaItemHandle` (bits 16-23 pierwszego non-empty handle) ma default `0x80`; jeśli żaden handle nie ma niezerowego `(h >> 16) & 0xFF`, default zostaje.
- Logika replikuje Rust ER-Save-Editor (`inventory/mod.rs::from_save`).

---

## Counter meanings

| Counter | Pole `SaveSlot` | Definicja | Wpis przy alokacji |
|---|---|---|---|
| `NextAoWIndex` | `slot.NextAoWIndex int` (`structures.go:256`) | Pozycja, na którą zostanie wstawiony **kolejny** AoW (handle prefix `0xC0`). Reprezentuje "prawą krawędź" lewej strefy AoW. | `allocateGaItem` umieszcza nowy AoW pod `slot.GaItems[NextAoWIndex]`, potem inkrementuje |
| `NextArmamentIndex` | `slot.NextArmamentIndex int` (`structures.go:257`) | Pozycja, na którą zostanie wstawiona kolejna broń/zbroja/talizman (prefix `0x80`, `0x90`, `0xA0`). Reprezentuje "prawą krawędź" strefy armament. AoW alloc też ją inkrementuje (bo shiftuje armament zone w prawo). | `allocateGaItem` umieszcza non-AoW pod `slot.GaItems[NextArmamentIndex]`, potem inkrementuje |
| `NextGaItemHandle` | `slot.NextGaItemHandle uint32` (`structures.go:258`) | Globalny licznik dolnych 16 bitów handle. `generateUniqueHandle(prefix)` używa go jako bazy: `prefix \| (counter & 0xFFFF) \| (PartGaItemHandle << 16)`. Inkrementowane po każdej alokacji handle. | rosnące monotonicznie do `MaxHandleAttempts = 10000` prób; wrap-around odrzucony jako overflow |

> ℹ️ Pole `PartGaItemHandle` (1 bajt) jest wewnętrznym tagiem partu używanym w handle layout. Default `0x80`. Wpływa na `(h >> 16) & 0xFF` — nie jest counterem alokacji i nie podlega tym samym invariantom co trzy główne.

---

## Required invariants

Te invarianty muszą obowiązywać po każdej zakończonej mutacji slotu. Naruszenie któregokolwiek powoduje rollback w `app.go::AddItemsToCharacter` (`ValidatePostMutation` zwraca non-empty `[]IntegrityError`):

| ID | Invariant | Egzekwujący kod |
|---|---|---|
| **I1** | `len(slot.GaItems) ∈ {GaItemCountOld, GaItemCountNew}` — wybór per `slot.Version` | `structures.go::scanGaItems` (load-time) |
| **I2** | `0 ≤ NextAoWIndex ≤ NextArmamentIndex ≤ len(GaItems)` | `diagnostics.go:448-460` (check `gaitem_indices`) |
| **I3** | AoW allocation odrzucana, gdy `NextArmamentIndex >= len(GaItems)` | `writer.go:461-463` (commit `6881cb9`); patrz sekcja "AoW allocation edge case" |
| **I4** | Każdy non-empty handle inventory typu `0x80/0x90/0xC0` istnieje w `slot.GaMap` | `diagnostics.go:373-389` (check `orphan_handle`) |
| **I5** | Brak duplikatów `Index` w łącznym zbiorze `Inventory.CommonItems` + `Inventory.KeyItems` | `diagnostics.go:392-419` (check `duplicate_index`); naprawa: `RepairDuplicateInventoryIndices` |
| **I6** | `GaItemData count ≤ GaItemDataMaxCount (7000)` | `diagnostics.go:421-430` (check `gaitemdata_count`); reject w `upsertGaItemData` |
| **I7** | `slot.Data[StorageBoxOffset:][:4]` (storage header count) == liczba non-empty rekordów w `Storage.CommonItems` | `diagnostics.go:434-465` (check `storage_count`); naprawa: `ReconcileStorageHeader` |
| **I8** | Żaden `slot.GaMap[h] == 0` (handle → `itemID 0` niedopuszczalne) | `diagnostics.go:469-477` (check `gamap_zero_id`) |
| **I9** | Allocation **nie może** przesuwać żadnego countera poza `len(GaItems)` | `writer.go::allocateGaItem` (gates na linii 447, 461, 482) — zwraca error przed mutacją |
| **I10** | Żadne dwie bronie (`0x80...`) nie referują tego samego non-sentinel `AoWGaItemHandle` (`0xC0xxxxxx`) | `aow_availability.go::ScanAoWAvailability` (pole `HasSharedHandleConflict`); pełna analiza w [54-ash-of-war](54-ash-of-war.md) |

Invarianty I1–I9 są **lokalne** dla allocatora i sąsiednich systemów (capacity, validation). I10 jest cross-cutting z AoW — tu zostaje wymieniony, ale szczegóły są w 54.

---

## Allocation zones

Tablica `GaItems` jest dzielona logicznie na dwie strefy bez fizycznego separatora — granica to wartości counterów:

```
indeks: 0 ─────────────────────────── len(GaItems)
        │ AoW zone │ armament zone   │
        │  0xC0    │ 0x80, 0x90, 0xA0 │
        ↑          ↑                  ↑
     start    NextAoWIndex    NextArmamentIndex
```

### AoW zone (prefix `0xC0`)

- Rośnie od indeksu `0` w prawo.
- `allocateGaItem` z `handleType == ItemTypeAow` wstawia pod `NextAoWIndex`, potem inkrementuje **oba** `NextAoWIndex` i `NextArmamentIndex` (bo wstawienie AoW pcha strefę armament o 1 w prawo).
- Konflikt (pozycja zajęta): shift-right do pierwszego empty wpisu (`writer.go:464-475`).

### Armament zone (prefix `0x80`, `0x90`, `0xA0`)

- Rośnie od `NextArmamentIndex` w prawo (po prawej stronie AoW zone).
- `allocateGaItem` z non-AoW handle wstawia pod `NextArmamentIndex`, inkrementuje tylko `NextArmamentIndex` (`writer.go:481-498`).
- Konflikt: shift-right tak samo jak dla AoW.

### Handle generation

`generateUniqueHandle(slot, prefix)` (`writer.go:402-420`):

1. Kopiuje `slot.NextGaItemHandle` do **lokalnej** zmiennej `counter`.
2. Buduje kandydata: `h := prefix | (uint32(slot.PartGaItemHandle) << 16) | counter`.
3. Sprawdza, czy `h` nie występuje już w `slot.GaMap`. Jeśli występuje — inkrementuje **lokalny** `counter`, przelicza `h` i próbuje znowu.
4. Po znalezieniu wolnego handle zapisuje `slot.NextGaItemHandle = counter + 1` i zwraca `h`. Pole `slot.NextGaItemHandle` jest aktualizowane **tylko przy sukcesie**, nigdy w pętli.
5. Limit: `MaxHandleAttempts = 10000` iteracji; po wyczerpaniu zwraca błąd `"failed to generate unique handle after %d attempts (prefix 0x%X)"`.
6. Dolne 16 bitów (`counter & 0xFFFF`) dają teoretyczną maksymalną populację 65536 handle per `prefix` per `partID`; w praktyce limit egzekwowany jest przez `MaxHandleAttempts` długo wcześniej (chyba że ktoś poda zły partID i zwęzi przestrzeń).

### Relacja AoW ↔ armament

- AoW alloc jest **dwustronna** — modyfikuje oba kursory.
- Armament alloc jest **jednostronna** — modyfikuje tylko `NextArmamentIndex`.
- Konsekwencja: po long-running save (dużo in-game pickups, sporo AoW przez Lost Ashes), AoW zone może zająć większość tablicy. Save z `NextAoWIndex == 500`, `NextArmamentIndex == 5120`, ale `non_empty == 60` jest **legalnym stanem** — co wynika z faktu, że empty entries po prawej stronie counterów też się liczą do "zone width".

---

## AoW allocation edge case fixed by 6881cb9

> ⚠️ **Historyczny bug + obecny invariant — czytaj uważnie przy implementacji allocatora.**

### Przed fixem

Wcześniejsza wersja `allocateGaItem` w gałęzi AoW sprawdzała tylko, czy `NextAoWIndex < maxEntries` (tj. czy lewa strefa ma miejsce). Po wstawieniu inkrementowała `NextArmamentIndex` bezwarunkowo. Jeśli save miał `NextArmamentIndex == len(GaItems)` (np. ze zwykłego play-through PS4, gdzie strefa armament rośnie do końca tablicy), AoW alloc:

1. Przechodził check `NextAoWIndex < maxEntries`.
2. Wstawiał AoW na `slot.GaItems[NextAoWIndex]`.
3. Inkrementował `NextArmamentIndex` z `len(GaItems)` do `len(GaItems) + 1`.
4. `ValidatePostMutation` wykrywał `NextArmamentIndex > len(GaItems)` (check `gaitem_indices`) → rollback.
5. Użytkownik widział "post-mutation validation failed" z numerycznym komunikatem zamiast czytelnego błędu pojemności.

### Po fixie (`6881cb9 fix(core): guard AoW allocation at armament capacity`)

Allocator dodaje guard **przed mutacją** (`writer.go:461-463`):

```go
if slot.NextArmamentIndex >= maxEntries {
    return fmt.Errorf(
        "allocateGaItem: cannot insert AoW — armament zone at capacity (NextArmamentIndex %d == %d)",
        slot.NextArmamentIndex, maxEntries)
}
```

Skutki:
- Alloc fail dostaje czytelny, allocator-level komunikat o capacity zamiast post-validation numeric error.
- `ValidatePostMutation` pozostaje ostatnią linią obrony — gdyby ktoś dodał nową ścieżkę zapisu, która ominęłaby ten guard, validator wciąż złapie naruszenie I2.
- Lock-in regresji: `backend/core/gaitem_placement_test.go:324-344` (`TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity`).
- Batch coverage: `tests/bulk_add_test.go::TestAddItems_RespectsArmamentCapacity` (linia 669+).

### Wytyczna dla implementatorów

Każda nowa ścieżka, która konstruuje wpis w `GaItems` (np. nowy typ przedmiotu, custom alloc dla testu, narzędzia diagnostyczne), **musi** używać `allocateGaItem` lub powtórzyć ten guard. Nie zakładaj, że validator post-mutation "wystarczy" — on tylko cofa zmiany, nie poprawia user-facing error message.

---

## UI counters vs allocator capacity

`SlotCapacity` (`app_deploy.go:31`) eksponuje do JS trzy pary `used / max`:

```go
type SlotCapacity struct {
    GaItemsUsed   int  // count(!IsEmpty(g)) for g in slot.GaItems
    GaItemsMax    int  // len(slot.GaItems)
    InventoryUsed int  // non-empty handles w Inventory.CommonItems
    InventoryMax  int  // CommonItemCount = 2688
    StorageUsed   int  // non-empty handles w Storage.CommonItems
    StorageMax    int  // StorageCommonCount = 1920
}
```

UI pasek `All Items used / max` (`frontend/src/App.tsx:529`) pokazuje **liczbę non-empty wpisów względem rozmiaru tablicy**. To **nie jest** liczba wolnych slotów alokacyjnych.

### Przykład patologiczny

Załóżmy save po długim play-through:

- `len(slot.GaItems) = 5120`
- `gaItemsUsed = 59` (tyle non-empty wpisów w tablicy)
- `NextArmamentIndex = 5120` (strefa armament dotarła do końca tablicy; empty entries są **pomiędzy** kursorami i po lewej stronie, nie po prawej)
- `NextAoWIndex = 500`

UI pokazuje `ALL ITEMS 59/5120` — sugeruje 5061 wolnych. Użytkownik klika "Add Weapon" → allocator zwraca:

```
allocateGaItem: armament/armor array full (index 5120 >= 5120)
```

Próba dodania AoW (przez ścieżkę, która tę alokację wykonuje) trafia w guard z `6881cb9`:

```
allocateGaItem: cannot insert AoW — armament zone at capacity (NextArmamentIndex 5120 == 5120)
```

### Skąd różnica

- "Wolny" w UI = wpis `IsEmpty()`. Empty entries istnieją wszędzie w tablicy (gra używa `0x00000000` lub `0xFFFFFFFF` jako placeholder); shift-right w `allocateGaItem` przesuwa zawartość, ale empty wpisy między kursorami nie są dostępne dla nowego alloc bez nadpisania.
- "Wolny" dla allocatora = pozycja, którą kursor wskazuje **i** mieści się w tablicy. Praktyczna miara dla obu typów alloc:

  ```
  allocator_free = len(slot.GaItems) - NextArmamentIndex
  ```

  - **Weapon/Armor/Talisman alloc** wymaga `NextArmamentIndex < len(GaItems)` (`writer.go:482-483`).
  - **AoW alloc** wymaga `NextAoWIndex < len(GaItems)` **oraz** `NextArmamentIndex < len(GaItems)` (`writer.go:447-463`) — bo AoW alloc inkrementuje **oba** kursory.
- W obu przypadkach to **`NextArmamentIndex` jest twardą granicą**. Wartość `NextArmamentIndex - NextAoWIndex` opisuje aktualną szerokość strefy armament (ile broni/zbroja/talizmanów mieści się między kursorami), nie "wolne miejsce dla AoW".

### Co rozdział zaleca dokumentacji

W rozdziałach [07-inventory](07-inventory.md) i [10-storage](10-storage.md) dodać callout o tym gapie. UI fix (eksponowanie `armament_zone_free` jako osobnego pola w `SlotCapacity`) jest **out of scope** Phase 2 — wymaga code change w `app_deploy.go` + `frontend/src/App.tsx`. Wpis w future work / `docs/ROADMAP.md`.

---

## Transactional safety and rollback

`app.go::AddItemsToCharacter` (`app.go:327-644`) implementuje pełny cykl all-or-nothing:

```
1. PRE-FLIGHT (no mutation)
   1a. ScanDuplicateInventoryIndices(slot) — odrzuć batch, jeśli slot już ma duplicate Index
   1b. Pre-compute finalIDs (z upgrades, infusions, container caps), trim qty
   1c. CheckAddCapacity(slot, items) — sprawdź wszystkie limity (inventory, storage, gaItems, gaItemData)
       → CapacityReport.CanFitAll == false → return AddResult{CapHit, ...}, slot nietknięty

2. SNAPSHOT
   2a. pushUndo(charIdx)
   2b. snapshot := SnapshotSlot(slot)

3. MUTATE
   3a. AddItemsToSlotBatch(slot, capacityItems)
       — alokuje GaItems, dodaje do Inventory/Storage, jeden RebuildSlotFull
       — error → RestoreSlot(slot, snapshot), return error
   3b. POST-FLAGS (event flags, tutorials, container pickups) — best-effort, log warn

4. RECONCILE
   4a. ReconcileStorageHeader(slot) — uzgodnij binarny count

5. VALIDATE (last line of defense)
   5a. violations := ValidatePostMutation(slot)
       → len(violations) > 0 → RestoreSlot(slot, snapshot), return error z pierwszym violation

6. COMMIT (implicit)
   6a. Zwróć AddResult{Added, Trimmed, FreeInv, FreeStore}
```

### Snapshot zakres

`SnapshotSlot` (`snapshot.go:28-67`) wykonuje deep copy:

- `slot.Data` (cały bufor bajtów)
- `slot.Version`
- `slot.Player` (struct PlayerGameData)
- `slot.GaMap` (map handle → itemID)
- `slot.GaItems` (slice GaItemFull)
- `slot.Inventory.Clone()`, `slot.Storage.Clone()`
- `slot.Warnings`
- Wszystkie offsety dynamiczne: `MagicOffset`, `InventoryEnd`, `EventFlagsOffset`, `PlayerDataOffset`, `FaceDataOffset`, `StorageBoxOffset`, `IngameTimerOffset`, `GaItemDataOffset`, `TutorialDataOffset`
- Wszystkie countery: `NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`, `PartGaItemHandle`

`RestoreSlot` (`snapshot.go:69-102`) nadpisuje wszystkie te pola z snapshota. **Nie** dotyka stosu undo — snapshot jest niezależnym mechanizmem bezpieczeństwa wewnątrz jednej operacji.

### Wzajemna relacja z undo

`pushUndo` jest wołane *przed* `SnapshotSlot`. Po sukcesie batch addy w stosie undo zostaje stan sprzed operacji (user-facing "Cofnij"). Po failu rollback przez `RestoreSlot` cofa zmiany w runtime; stos undo zostaje z dodanym, niepotrzebnym, wpisem identycznym co aktualny stan — dopuszczalne, bo undo bezstanowo cofa do snapshotu sprzed pushUndo.

---

## Post-mutation validation

`ValidatePostMutation` (`diagnostics.go:363-468`) jest **ostatnią linią obrony**. Allocator i inne ścieżki write-side powinny failować wcześniej (i z czytelnym komunikatem), bo validator zwraca tylko opis ogólny i wymusza rollback całej operacji.

### Pełna lista checków (current code, 6 check keys)

| Check key | Co weryfikuje | Source |
|---|---|---|
| `orphan_handle` | Każdy non-empty handle inventory typu `0x80/0x90/0xC0` istnieje w `slot.GaMap` | `diagnostics.go:373-389` |
| `duplicate_index` | Brak duplikatów `Index` w `Inventory.CommonItems` ∪ `KeyItems` | `diagnostics.go:392-419` |
| `gaitemdata_count` | Header count `GaItemData ≤ GaItemDataMaxCount (7000)` | `diagnostics.go:421-430` |
| `storage_count` | Header storage count == liczba non-empty rekordów w `Storage.CommonItems` | `diagnostics.go:434-465` |
| `gaitem_indices` | `NextAoWIndex ≤ NextArmamentIndex` **i** `NextArmamentIndex ≤ len(GaItems)` (jeden check key, dwa sub-violation messages) | `diagnostics.go:448-460` |
| `gamap_zero_id` | `slot.GaMap[h] != 0` dla każdego h | `diagnostics.go:469-477` |

> ℹ️ Validator może wyemitować więcej niż jeden `IntegrityError` na ten sam slot (np. wiele osieroconych handle, wiele duplikatów Index). Liczba pozycji w zwróconym `[]IntegrityError` zależy od stanu slotu; pierwsza pozycja jest tym, co woła `app.go::AddItemsToCharacter` propaguje do user-facing error string.

### Czemu validator nie powinien być pierwszą linią obrony

- Validator zwraca pierwszy violation jako string — gubi kontekst (który item ID, jaka pozycja).
- Validator nie próbuje naprawiać — zawsze rollback całej operacji.
- Validator nie rozróżnia between "user-facing error" (np. armament zone full) i "bug allocatora" (np. mismatch counterów).

Zasada projektowa: **allocator i pre-flight powinny złapać 100% legalnych user-facing errors. Validator łapie bugi.**

---

## Test coverage

### Allocator (`backend/core/gaitem_placement_test.go`)

| Test | Co weryfikuje |
|---|---|
| `TestAllocateGaItem_AoWAtLowIndex` (linia 25) | AoW wstawiany na `NextAoWIndex` |
| `TestAllocateGaItem_WeaponAfterAoW` (linia 48) | Weapon wstawiany za AoW zone |
| `TestAllocateGaItem_TypeSegregation` (linia 80) | AoW i armament nie mieszają się w pozycjach |
| `TestAllocateGaItem_ShiftPreservesExisting` (linia 132) | Shift-right przy konflikcie zachowuje sąsiadujące entries |
| `TestAllocateGaItem_ReturnsFullErrorAtCapacity` (linia 253) | Reject przy `NextArmamentIndex == maxEntries` dla weapon/armor |
| `TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity` (linia 324) | **Guard z commita `6881cb9`** — error "armament zone at capacity" |
| `TestGenerateUniqueHandle_GlobalCounter` (linia 158) | Counter monotoniczny; rejection przy duplikatach |
| `TestScanGaItems_TrackedIndices` (linia 187) | Reconstruction counterów po loadzie |

### Capacity + batch (`tests/`)

| Test | Co weryfikuje |
|---|---|
| `tests/capacity_test.go` (16 testów) | Pre-flight, all-or-nothing, container caps, post-validation, header reconcile, storage edge cases |
| `tests/bulk_add_test.go::TestAddItems_RespectsArmamentCapacity` (linia 669) | Batch add nie przekracza armament zone capacity |
| `tests/bulk_add_test.go::TestAddItems_BulkArmamentZoneOnly` (linia 164) | Stress test przy granicy armament zone |
| `tests/bulk_add_test.go::TestAddItems_BulkAoWAndArmamentSplit` (linia 336) | Mixed AoW + weapon batch |

### Snapshot/rollback i validator

| Test | Co weryfikuje |
|---|---|
| `tests/capacity_test.go::TestAllOrNothing*` | Snapshot/restore przy capacity hit |
| `app_additems_duplicate_index_test.go` | Reject przy pre-existing duplicate Index |
| `app_repair_duplicate_index_test.go` | `RepairDuplicateInventoryIndices` idempotency |
| `backend/core/duplicate_index_scan_test.go` | `ScanDuplicateInventoryIndices` poprawność |

### AoW guards i availability

| Test | Co weryfikuje |
|---|---|
| `backend/core/aow_strict_test.go::TestAllocateGaItem_NewWeaponUsesZeroSentinel` (linia 303) | Nowa broń ma `AoWGaItemHandle = NoCustomAoWHandle (0x00)` |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_ZeroSentinelNotCounted` | Sentinel nie liczy się jako użyta kopia AoW |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_LegacyFFFFFFFFSentinelNotCounted` | Legacy sentinel (`0xFFFFFFFF`) też ignorowany |

---

## Known limits / open questions

### UI capacity bar (`needs verification` w sensie UX, nie poprawności kodu)

- Pasek `All Items used/max` w UI jest **mylący** po fixie `6881cb9` dla save'ów z `NextArmamentIndex` blisko `len(GaItems)`. Kod nie jest błędny — UI po prostu nie ujawnia "armament zone free".
- Naprawa wymaga rozszerzenia `SlotCapacity` o pole `ArmamentZoneFree` lub równoważne i update frontend. To **out of scope** rozdziału 35 (i Phase 2 dokumentacji).
- Wpis w `docs/ROADMAP.md` jako follow-up.

### Rehandle przy armament zone full — potencjalna luka testowa

- Transfer instance-move (`backend/core/transfer.go::transferOne` ścieżka rehandle) wywołuje `materializeRehandledInstance`, które używa `allocateGaItem`. Jeśli `NextArmamentIndex == len(GaItems)`, allocator zwróci error → `SkipReasonHandleAllocFailed`.
- Nie znalazłem explicit testu dla tego scenariusza w `tests/transfer_test.go`. **Status**: `needs verification` przez dodanie regresyjnego testu (np. `TestMoveTalismanDuplicate_FailsWhenArmamentZoneFull`). Kod jest deterministyczny przez wspólny `allocateGaItem`, więc nie jest to bug — tylko brakujący lock-in regresji.

### Storage Apply in-game verification

- Spec [53-inventory-storage-transfer](53-inventory-storage-transfer.md) wymienia weryfikację Storage Apply na Steam Deck jako "future work" / sanity check. Aktualnie status w 53 to `needs verification` aż do potwierdzenia.
- Rozdział 35 nie zależy od tej weryfikacji — allocator i validator są pokryte testami niezależnie od zachowania UI Storage Apply.

### `slot.Version` typowe wartości w obiegu

- Kod nie ma listy explicit wartości `Version`. Ostatnia obserwowana wartość ze save fixturów: do weryfikacji w [20-version-platform](20-version-platform.md). Próg `81` był wprowadzony historycznie przez game patch — exact patch number `needs verification`.

---

## Implementation checklist

Lista dla autorów nowych ścieżek write-side w Save Forge oraz innych edytorów Elden Ring save:

- [ ] **Nigdy nie zakładaj hardcoded `5120`** w runtime path. Po `LoadSave` zawsze używaj `len(slot.GaItems)`.
- [ ] **Czytaj `slot.Version` tylko w load path** (analogicznie do `scanGaItems`). Po load `len(slot.GaItems)` jest source-of-truth.
- [ ] **Nie ufaj `non_empty count` jako allocator free space**. Sprawdzaj `NextArmamentIndex` (dla weapon/armor/talisman) i `NextAoWIndex` (dla AoW pre-check) zanim spróbujesz alokacji.
- [ ] **AoW alloc wymaga `NextArmamentIndex < len(GaItems)`**, nie tylko `NextAoWIndex < len(GaItems)`. Wstawienie AoW shiftuje armament zone w prawo.
- [ ] **Przed mutacją** wywołaj `CheckAddCapacity` (lub equivalent w nowej ścieżce). All-or-nothing — nie zostawiaj batcha w częściowo zaaplikowanym stanie.
- [ ] **Snapshot przed mutacją** przez `SnapshotSlot`. Rollback przez `RestoreSlot` przy każdym błędzie (włącznie z błędami post-validation).
- [ ] **Po mutacji** wywołaj `ReconcileStorageHeader` (i `ReconcileInventoryHeader`, jeśli zmieniono common count inventory) przed `ValidatePostMutation`. Validator inaczej rzuci `storage_count` violation.
- [ ] **Sprawdź `ValidatePostMutation`** zawsze. Niech będzie nieosiągalne (jak w obecnym kodzie), ale traktuj jako kontraktową gwarancję integralności.
- [ ] **Nigdy nie współdziel `AoWGaItemHandle`** między dwoma broniami. Naruszenie I10 → `EXCEPTION_ACCESS_VIOLATION` przy load gry. Szczegóły w [54-ash-of-war](54-ash-of-war.md).
- [ ] **Używaj `generateUniqueHandle`** zamiast ręcznego konstruowania handle. Counter wrap-around (>10000 prób) jest błędem konfiguracji — propaguj go.
- [ ] **Test każdy nowy code path** przeciwko fixturze save'a z różnymi `slot.Version` (≤ 81 i > 81) — kod używa `len(GaItems)`, ale fixtury weryfikują, że nie przeoczyłeś hardcoded constant w nowej ścieżce.

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — binary layout `GaItem` (handles, item IDs, rozmiary rekordów). Phase 2 rewrite tego rozdziału jest planowany po zatwierdzeniu 35.
- [07-inventory](07-inventory.md) — Inventory data model (`Inventory.CommonItems`/`KeyItems`, bramka widoczności, `NextEquipIndex`, `NextAcquisitionSortId`). Phase 2 rewrite zaplanowany.
- [10-storage](10-storage.md) — Storage data model. Phase 2 rewrite zaplanowany.
- [43-transactional-item-adding](43-transactional-item-adding.md) — pełen architektura add-items (PRE-FLIGHT → SNAPSHOT → MUTATE → RECONCILE → VALIDATE). Phase 2 rewrite zaplanowany.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — algorytm stride-2 dla reorder. Niezwiązany bezpośrednio z allocatorem, ale dzieli `slot.Inventory.NextAcquisitionSortId`.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — transfer Inventory ↔ Storage z rehandle path (używa `allocateGaItem` wewnątrz `materializeRehandledInstance`).
- [54-ash-of-war](54-ash-of-war.md) — pełna semantyka AoW, sentinele weapon `AoWGaItemHandle`, invariant unikalności handle (I10).

---

## Sources

- `backend/core/structures.go` — `SaveSlot`, `scanGaItems`, counter reconstruction
- `backend/core/writer.go` — `allocateGaItem`, `generateUniqueHandle`, `AddItemsToSlot`, `AddItemsToSlotBatch`, `RepairOrphanedGaItems`
- `backend/core/capacity.go` — `SlotUsage`, `CountSlotUsage`, `CheckAddCapacity`
- `backend/core/diagnostics.go` — `ValidatePostMutation`, `IntegrityError`
- `backend/core/snapshot.go` — `SnapshotSlot`, `RestoreSlot`
- `backend/core/slot_rebuild.go` — `RebuildSlotFull`
- `backend/core/aow_availability.go` — `ScanAoWAvailability`, `AoWCopyRaw`
- `backend/core/offset_defs.go` — `GaItemCountOld`, `GaItemCountNew`, `GaItemVersionBreak`, `GaItemDataMaxCount`, `MaxHandleAttempts`, `CommonItemCount`, `StorageCommonCount`
- `app.go::AddItemsToCharacter`, `app_deploy.go::SlotCapacity` / `GetSlotCapacity`
- Tests: `backend/core/gaitem_placement_test.go`, `backend/core/aow_strict_test.go`, `tests/capacity_test.go`, `tests/bulk_add_test.go`
- Commit `6881cb9 fix(core): guard AoW allocation at armament capacity` (root cause + lock-in test)
- `tmp/docs-phase2-gaitem-inventory-plan.md` — plan Phase 2 (decyzje o consolidation)
- `tmp/docs-phase2-micro-research.md` — research H5/H6 (capacity threshold, transfer caps)
