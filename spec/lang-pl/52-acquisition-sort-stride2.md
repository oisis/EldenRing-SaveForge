# 52 — Acquisition Sort Stride-2: write model for in-game ordering

> **Typ**: Binary write-path spec + design rationale
> **Status**: ✅ canonical, implemented. Algorytm zweryfikowany empirycznie (sentinel v1/v2/v3 in-game na Steam Deck) i potwierdzony w kodzie `app_inventory_order.go::ReorderInventory/ReorderStorage` oraz `backend/editor/save.go::writeContainerLayout`. Allocator/capacity/counter invariants opisane są canonicalnie w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md); model danych Inventory/Storage w [07-inventory](07-inventory.md), [10-storage](10-storage.md).
> **Zakres**: Write-side model przypisywania `InventoryItem.Index` (acquisition index) tak, by gra renderowała items w żądanej kolejności pod sort "Acquisition Order ↑". Pokrywa: stride-2 base assignment, bucket-collision guard, Inventory + Storage paths, integrację z workspace save layer. Mechanika **sortowania** w grze (jak gra interpretuje `Index` jako bucket key) i discovery odkrycia są tutaj — to canonical home dla tego mechanizmu.

---

## Cel rozdziału

Rozdział opisuje:

- czym jest acquisition index (`InventoryItem.Index`) z perspektywy in-game sort;
- jak gra interpretuje wartość `Index` — bucket key `acqIdx >> 1` (odkryte empirycznie);
- dlaczego naiwny stride-1 nie działa i jak stride-2 z parzystą bazą gwarantuje sukces;
- defensywny guard wykrywający kolizje kubełków przed mutacją;
- trzy miejsca w kodzie używające stride-2 (Inventory reorder, Storage reorder, workspace save layer);
- jakie kategorie itemów są w zakresie reorder, a jakie poza;
- jak reorder współpracuje z `pushUndo` i dlaczego nie używa `SnapshotSlot`/`ValidatePostMutation`.

Co rozdział **NIE** robi:

- Nie opisuje `Index` jako pola binarnego ani layoutu rekordu inventory — [07-inventory → Binary structures](07-inventory.md#binary--runtime-structures).
- Nie opisuje semantyki transferu Inventory ↔ Storage (przypisywanie świeżego `Index` w `MoveItemsBetweenContainers` używa single-stride, nie stride-2) — [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- Nie opisuje `Index` assignment przy Add Items — [43-transactional-item-adding](43-transactional-item-adding.md).
- Nie opisuje allocator/counter invariantów ani capacity check — [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
- Nie opisuje SortOrder UI flow (drag-and-drop, dropdowny sort mode) — pełna semantyka po stronie frontendu pozostaje w [53-inventory-storage-transfer → Frontend](53-inventory-storage-transfer.md) (z zastrzeżeniem, że aktualny `SortOrderTab.tsx` nie eksponuje sort dropdownów — patrz "Direction naming and UI caveats" w tym rozdziale).

---

## Status

- canonical
- implemented (trzy ścieżki write-side: legacy `ReorderInventory`/`ReorderStorage` + workspace `writeContainerLayout`)
- source-of-truth: kod backend + empiryczna weryfikacja in-game (sentinel testy)
- Storage in-game verification: `needs verification` (świeży test in-game / Steam Deck nieaktualny)
- allocator details: **chapter 35**

---

## Source of truth w kodzie

| Temat | Pliki / funkcje | Notatka |
|---|---|---|
| Pole `InventoryItem.Index` | `backend/core/structures.go:118-122` | u32 na offset `+8` w 12 B rekordzie inventory/storage |
| Reserved equipment range | `backend/core/offset_defs.go:341` (`InvEquipReservedMax = 432`) | acquisition values ≤ 432 zarezerwowane dla equipment slots |
| Legacy Inventory reorder | `app_inventory_order.go:255-417` (`ReorderInventory`) | stride-2 + bucket-collision guard, `pushUndo` przed mutacją |
| Legacy Storage reorder | `app_inventory_order.go:439-...` (`ReorderStorage`) | mirror Inventory, używa `slot.Storage.NextAcquisitionSortId` |
| Workspace save (newer) | `backend/editor/save.go::writeContainerLayout` (linie 518-..., `baseAcq` w linii 566) | stride-2 z `reservedAcq` skip-over dla passthrough records (pre-existing acquisition values nie mogą się zderzyć z nową bazą) |
| Read path | `app_inventory_order.go:80-149` (`GetInventoryOrder`), `:160-238` (`GetStorageOrder`) | sort `Index` ascending, filtrowanie po kategorii tabu |
| Tab → kategorie mapa | `app_inventory_order.go:31-39` (`inventoryOrderTabs`) | 6 tabów: `weapons` (= `melee_armaments` + `ranged_and_catalysts` + `shields`), `talismans`, `head`, `chest`, `arms`, `legs` |
| Technical placeholder filter | `app_inventory_order.go:51-58` (`invUnarmedBaseID = 0x0001ADB0`, `isWeaponOrderTechnical`) | 3 instancje "Unarmed" w `CommonItems` jako empty-hand placeholder; wykluczone z Sort Order |
| Item DTO dla UI | `app_inventory_order.go:14-29` (`InventoryOrderItem`) | pola: `Handle`, `ItemID`, `Name`, `Category`, `AcquisitionIndex`, `Weight`, `SortId`, `SortGroupId`, `CurrentUpgrade`, `MaxUpgrade`, `InfusionName`, `IconPath`, `IsTechnical` |

---

## Mental model

```
in-game render
"Acquisition Order ↑"
        ▲
        │  sort key = acqIdx >> 1   (game-side, odkryte empirycznie)
        │
        │  bucket(item) = InventoryItem.Index >> 1
        │  → tied items wewnątrz kubełka rozstrzygane przez
        │    inny klucz (prawdopodobnie sortGroupId / handle)
        │
slot.Data
 │
 ├── Inventory.CommonItems[i].Index = u32 ← write-side stride-2 base + pos*2
 └── Storage.CommonItems[i].Index   = u32 ← analogicznie
```

**Kluczowa obserwacja**: gra **nie** porównuje pełnej wartości `Index` (32 bity), tylko `Index >> 1`. Sąsiednie pary `(k, k+1)` z parzystym `k` trafiają do tego samego kubełka i są zamieniane przez tie-breaker gry. Stride-2 z parzystą bazą gwarantuje unikalność kubełków: `(base + 2*i) >> 1 = base/2 + i` jest ściśle rosnące.

---

## Acquisition index semantics

`InventoryItem.Index` (offset `+8`, u32) niesie globalny acquisition counter — gra zapisuje go przy każdym podniesieniu/utworzeniu wpisu inventory, monotonicznie rosnąco z `NextAcquisitionSortId` (counter na końcu sekcji, zob. [07-inventory → Capacity and counters](07-inventory.md#capacity-and-counters)).

Konsekwencje:

- Pole jest **per-rekord** — każdy slot inventory/storage trzyma własną wartość.
- W trybie sort **innym niż Acquisition Order** (Weight, Type, Attack Power, Alphabetical) gra liczy klucz sort runtime z `regulation.bin` (parametry `EquipParamWeapon.sortGroupId/sortId/...`); zmiana `Index` w save jest dla nich niewidoczna.
- Wartości ≤ `InvEquipReservedMax = 432` są **zarezerwowane dla równich slotów** (`backend/core/offset_defs.go:341`); reorder MUSI używać `Index > 432`.
- Wartości `>= 10000` powodują **crash gry przy load** (sentinel v1 discovery — zob. niżej). Algorytm zaczyna od `NextAcquisitionSortId` (typowo 500–2000 dla long-running save), więc w praktyce nie zbliża się do tego limitu.

---

## Stride-2 write model

### Discovery: sentinel testy in-game

Empiryczna weryfikacja in-game na Steam Deck (zakładka talizmany, 65 itemów):

| Test | Przypisanie | Wynik |
|---|---|---|
| Sentinel v1 (hardcoded 10000-10012) | `10000, 10001, ..., 10012` | **Crash gry** przy load save |
| Sentinel v2 (safe, stride-1) | `minAcq, minAcq+1, ..., minAcq+N` | Save load OK, **sąsiednie pary zamienione** |
| Sentinel v3 (stride-2) | `base, base+2, ..., base+N*2` (parzyste `base`) | Save load OK, **kolejność poprawna** |

Wniosek: gra sortuje po `acqIdx >> 1`. Stride-1 z dowolną bazą `k` produkuje kolizje: `k >> 1 == (k+1) >> 1` gdy `k` parzyste.

### Algorytm

```go
// base: zaczyna od NextAcquisitionSortId; zapewnia parzysty > InvEquipReservedMax (432).
base := slot.Inventory.NextAcquisitionSortId
if base <= uint32(core.InvEquipReservedMax) {
    base = uint32(core.InvEquipReservedMax) + 2  // 434 — minimalna bezpieczna parzysta wartość
}
if base%2 != 0 {
    base++
}

// Przypisanie: item na pozycji pos otrzymuje Index = base + pos*2.
for pos, handle := range orderedHandles {
    newIdx := base + uint32(pos)*2
    // zapis do slot.Data[off+8:] i slot.Inventory.CommonItems[j].Index
}

// Inkrementacja NextAcquisitionSortId (tylko w górę).
expectedMax := base + uint32(len(orderedHandles)-1)*2
newNextAcq := expectedMax + 1
```

### Dowód unikalności kubełków

Dla parzystego `base` i pozycji `i`:

```
bucket(i) = (base + 2*i) >> 1
          = base/2 + i
```

Ponieważ `base` jest parzyste, `base/2` jest liczbą całkowitą. `base/2 + i` jest ściśle rosnące w `i` → żadne dwie pozycje nie dzielą kubełka.

---

## Bucket collision guard

`ReorderInventory` zawiera defensywny check przed jakąkolwiek mutacją (`app_inventory_order.go:371-378`):

```go
shiftKeys := make(map[uint32]int, len(orderedHandles))
for pos := range orderedHandles {
    key := (base + uint32(pos)*2) >> 1
    if prevPos, dup := shiftKeys[key]; dup {
        return fmt.Errorf("stride-2 reorder: bucket collision at key=%d positions %d and %d; refusing", key, prevPos, pos)
    }
    shiftKeys[key] = pos
}
```

**Status guarda**: nieosiągalny przy poprawnej parzystej bazie + stride-2 (dowód powyżej). Cel: lock-in regresji — jeśli ktoś w przyszłości zmieni `base/stride` logic, ten guard zatrzyma cichą regresję (np. stride-1 powracający do save).

Analogiczny pre-mutation guard istnieje też w `ReorderStorage` (mirror tej samej logiki dla `slot.Storage`).

---

## Inventory reorder path

`(*App).ReorderInventory(charIdx int, tab string, orderedHandles []uint32) error` (`app_inventory_order.go:255`).

Sekwencja:

1. **Walidacja args**: tab w `inventoryOrderTabs`, `a.save != nil`, `charIdx ∈ [0, 10)`, `slot.Version > 0`, `len(orderedHandles) > 0`.
2. **Duplicate handle check**: `seen` map; reject error.
3. **Locate handles**: iter `CommonItems`, dopasuj po handle, weryfikuj `itemData.Category ∈ tabCategorySet(tab)`, weryfikuj `!isWeaponOrderTechnical` (dla weapons tab).
4. **Complete list check**: liczba `orderedHandles` musi równać się liczbie eligible items dla tabu w slot — partial lists rejected.
5. **Compute stride-2 base**: round up `NextAcquisitionSortId` do parzystej wartości > `InvEquipReservedMax`.
6. **Bucket collision guard**: pre-mutation check (powyżej).
7. **`pushUndo(charIdx)`** — user-facing undo.
8. **Apply stride-2 indices**: zapis do `slot.Data[off+8:]` (binary) i `slot.Inventory.CommonItems[j].Index` (runtime); per handle z `orderedHandles`.
9. **Advance counters**: `slot.Inventory.NextAcquisitionSortId = expectedMax + 1`, `slot.Inventory.NextEquipIndex = max(current, newNextAcq)`, plus binary write-back pod `NextEquipIndexOff()` (gdy offset > 0).

Argumenty/błędy są deterministyczne — żaden krok nie zostawia slotu w pół-zmodyfikowanym stanie. Mutacje (krok 8-9) są wykonywane dopiero po pozytywnej walidacji (kroki 1-6).

### Convenience wrappers

- `GetWeaponInventoryOrder(charIdx int)` (`:607`) → delegacja do `GetInventoryOrder("weapons")`.
- `ReorderWeaponInventory(charIdx int, orderedHandles []uint32)` (`:613`) → delegacja do `ReorderInventory("weapons", ...)`.

---

## Storage reorder path

`(*App).ReorderStorage(charIdx int, tab string, orderedHandles []uint32) error` (`app_inventory_order.go:439`).

Identyczna semantyka jak Inventory, z różnicami:

| Aspekt | Inventory | Storage |
|---|---|---|
| Target slice | `slot.Inventory.CommonItems` | `slot.Storage.CommonItems` |
| Binary start offset | `slot.MagicOffset + InvStartFromMagic` | `slot.StorageBoxOffset + StorageHeaderSkip` |
| Counter source | `slot.Inventory.NextAcquisitionSortId` | `slot.Storage.NextAcquisitionSortId` |
| Counter write-back | `slot.Inventory.nextEquipIndexOff` | `slot.Storage.nextEquipIndexOff` |
| Post-mutation reconcile | brak (`Inventory.CommonItems` jest fizycznie pełną tablicą) | brak (zob. `needs verification` w [10-storage](10-storage.md)) |

> ⚠️ **Storage in-game verification — `needs verification`**: backend Storage reorder ma pełne testy (`app_storage_order_test.go`, 9 testów), ale ostatni sanity check in-game na Steam Deck (że "Acquisition Order ↑" w skrzyni w grze odpowiada preview edytora) **nie jest świeży**. Zob. [10-storage → Known limits](10-storage.md#known-limits--needs-verification).

---

## Workspace/editor save integration

Nowsza ścieżka write-side używana przez `SaveInventoryWorkspaceChanges` (`app_inventory_session.go:192`) → `backend/editor/save.go::ApplyWorkspaceSave` → `writeContainerLayout` (`save.go:518-...`).

Ta ścieżka **rebuilduje** całą sekcję `CommonItems` z workspace snapshot, nie wykonując in-place `Index` rewrite jak legacy reorder. Stride-2 jest wciąż używane (`save.go:565-...`), ale z dodatkową pre-condition:

- **`occupied`** mapa: pinned slots dla `passthrough` records (pre-existing items, których workspace nie modyfikuje).
- **`reservedAcq`** mapa: acquisition values już używane przez passthrough records.
- **`baseAcq`** start: `equip.NextAcquisitionSortId` zaokrąglone w górę do parzystej, > `InvEquipReservedMax`.
- **Skip-over collision loop** (`save.go:573-...`): jeśli `baseAcq + 2*i` koliduje z `reservedAcq[*]` dla któregokolwiek `i ∈ [0, len(editables))`, base inkrementowane o 2 i loop powtarza.

Skutek: workspace path zachowuje passthrough acquisition values bez ich modyfikacji, a editable items dostają świeżą bazę nie zderzającą się z istniejącymi values. Ten sam algorytm stride-2, inna pre-condition.

> ℹ️ Legacy `ReorderInventory`/`ReorderStorage` zakłada, że **wszystkie** eligible items dla tabu są w `orderedHandles` (complete list). Workspace path zakłada przeciwnie — tylko **editable items** są re-indexowane, passthrough zostają z oryginalnym `Index`.

---

## Category filtering and unknown-category behavior

`tabCategorySet(tab)` (`app_inventory_order.go:62`) zwraca:

- `map[category]bool` dla znanych tabów (`weapons` → `{melee_armaments, ranged_and_catalysts, shields}`; pozostałe — single-category sets).
- `error` dla unknown tab.

`ReorderInventory`/`ReorderStorage` rejectuje handle z `itemData.Category` poza setem tabu (`app_inventory_order.go:308-310`):

```
handle 0x%08X (category %q) does not belong to sort order tab %q
```

`GetInventoryOrder`/`GetStorageOrder` filtruje items pasujące do tabCategorySet — items spoza zakresu są **niewidoczne** w Sort Order UI i nie podlegają reorder. Ich `Index` w binarce pozostaje niezmieniony.

### Kategorie poza Sort Order

Aktualnie 6 tabów obsługuje: `weapons` (broń + tarcze + ranged + catalysts), `talismans`, `head`, `chest`, `arms`, `legs`. **Poza zakresem reorder**:

- `goods` (consumables, materiały, key items inventory) — handle prefix `0xB0`
- `ashes` (spirit ashes)
- `ashes_of_war` (AoW gems)
- `crafting_materials`
- `bolstering_materials`
- `tools`
- `info`
- `sorceries`, `incantations`

Te kategorie mają własną `Index` w binarce, ale ich kolejność w grze nie podlega edycji przez Save Forge.

> ℹ️ **Unarmed placeholder exclusion**: gra przechowuje 3 wpisy "Unarmed" (baseID `0x0001ADB0`) w `Inventory.CommonItems` jako technical slots dla empty-hand state. `isWeaponOrderTechnical` (`app_inventory_order.go:56`) wyklucza je z weapons tab — `GetInventoryOrder("weapons")` ich nie zwraca, `ReorderInventory("weapons", [handle Unarmed])` odrzuca z błędem "is a technical placeholder".

---

## Direction naming and UI caveats

Spec ten opisuje **write-side mechanikę** — przypisanie wartości `Index` tak, by gra renderowała items w żądanej kolejności pod "Acquisition Order ↑". Nie opisuje UI dropdownów ani semantyki ↑/↓ w aktualnym frontendzie.

> ⚠️ **Aktualny `SortOrderTab.tsx`** (`frontend/src/components/SortOrderTab.tsx`) **nie eksponuje sort dropdownów** Acquisition ↑/Acquisition ↓/Weight/Type. Komponent używa workspace API z workspace-internal `Position` field; user re-ordeuje przez drag-and-drop w 5×6 grid, a `SaveInventoryWorkspaceChanges` tłumaczy `Position` na stride-2 acquisition Index w `backend/editor/save.go`.
>
> Historyczne wzmianki o dropdownach Acquisition ↑/↓ w innych dokumentach (np. [53-inventory-storage-transfer](53-inventory-storage-transfer.md)) odnoszą się do wcześniejszej iteracji UI. **`needs verification`** czy obecny SortOrderTab jest finalną wersją UI, czy planowane jest przywrócenie sort dropdownów (i czy ↑/↓ naming będzie zgodny z grą).

---

## Validation and rollback relation

`ReorderInventory`/`ReorderStorage` wykonuje:

- **`pushUndo(charIdx)`** przed mutacją (user-facing undo stack).
- **Pre-mutation validation**: duplicate handle, missing handle, wrong category, technical placeholder, incomplete list, bucket collision guard.

**Czego brakuje** (w porównaniu z `AddItemsToCharacter`):

- ❌ **Brak `SnapshotSlot`/`RestoreSlot`** — reorder nie używa internal safety net.
- ❌ **Brak `ValidatePostMutation`** po reorder — `duplicate_index` check w validatorze nie jest egzekwowany dla tej ścieżki.

**`needs verification`**: czy to świadoma decyzja (reorder modyfikuje tylko `Index` — nie GaItems, nie GaMap, nie containers — więc mniej rzeczy może pójść nie tak), czy luka w transactional safety. Argument za świadomością: pre-mutation validation (kroki 1-6) jest wystarczająco restrykcyjne, żeby zapobiec entry-state corruption; po pomyślnym przejściu wszystkich pre-checks mutacje są mechaniczne i nie powinny failować.

Pełna semantyka transactional safety dla `AddItemsToCharacter`/`MoveItemsBetweenContainers` — [35 → Transactional safety](35-gaitem-allocator-invariants.md#transactional-safety-and-rollback), [43-transactional-item-adding](43-transactional-item-adding.md).

---

## Test coverage

### Inventory reorder (`app_inventory_order_test.go`)

| Test | Co weryfikuje |
|---|---|
| `TestGetWeaponInventoryOrder_NoSave` (84) | Reject przy `a.save == nil` |
| `TestGetWeaponInventoryOrder_InvalidIdx` (92) | Reject przy out-of-range `charIdx` |
| `TestGetWeaponInventoryOrder_EmptySlot` (102) | Handling slot z `Version == 0` |
| `TestGetWeaponInventoryOrder_ReturnsWeaponsAscending` (111) | Sort `Index` ascending |
| `TestGetWeaponInventoryOrder_HidesUnarmed` (136) | Wykluczenie technical Unarmed placeholders |
| `TestGetWeaponInventoryOrder_UpgradeInfusionDecoded` (153) | Decoder `+N` upgrade i infusion name |
| `TestReorderWeaponInventory_NoSave` (178), `_InvalidCharIdx` (186), `_DuplicateHandle` (196), `_MissingHandle` (205), `_UnarmedHandle` (216), `_IncompleteList` (227) | Pre-mutation validation błędów |
| `TestReorderWeaponInventory_HappyPath` (236) | Pełen happy path: reorder zmienia `Index` zgodnie z `orderedHandles` |
| `TestReorderWeaponInventory_DoesNotTouchGaItems` (291) | Reorder nie modyfikuje GaItems ani GaMap |
| `TestReorderWeaponInventory_StorageHandle` (334) | Handle ze storage nie może być użyty w Inventory reorder |
| `TestGetInventoryOrder_UnknownTab` (421) | Reject unknown tab |
| `TestGetInventoryOrder_Talismans_Items` (433), `_Head_Items` (458) | Sort Order dla pozostałych tabów |
| `TestReorderInventory_Talismans_HappyPath` (482), `_HeadOrChest_HappyPath` (513), `_WrongTabHandle_Blocks` (544) | Reorder dla talismans / armor + cross-tab rejection |

### Storage reorder (`app_storage_order_test.go`)

| Test | Co weryfikuje |
|---|---|
| `TestReorderStorage_RejectsMissingHandle` (124), `_RejectsDuplicateHandle` (147), `_RejectsIncompleteList` (169), `_RejectsHandleFromInventory` (342) | Pre-mutation validation |
| `TestReorderStorage_DoesNotTouchInventory` (192), `TestInventoryReorder_DoesNotTouchStorage` (403) | Cross-container izolacja |
| `TestReorderStorage_PersistsAcquisitionOrder` (267) | Stride-2 trwale zapisane w binarce |
| `TestReorderStorage_DoesNotTouchHandlesOrQty` (316) | Reorder modyfikuje tylko `Index`, nie handle/qty |
| `TestReorderStorage_RoundTripReread` (357) | Save → reorder → write → reload — kolejność zachowana |

### Workspace save (R7-related, częściowe pokrycie)

- `backend/editor/save_test.go` (jeśli istnieje) i `app_inventory_session_test.go` — pokrycie `writeContainerLayout` stride-2 + reservedAcq skip-over. **`needs verification`**: pełna mapa testów workspace path nie była zliczana w tej fazie.

### In-game empirical (Steam Deck)

- Sentinel v1/v2/v3 (talismans, 65 itemów) — wykonane przy odkryciu stride-2.
- **`needs verification`**: świeży test Storage reorder w grze (zob. [10-storage → Known limits](10-storage.md#known-limits--needs-verification)).

---

## Known limits / needs verification

- **Storage Apply in-game verification** — sanity check w grze (Steam Deck), że "Acquisition Order ↑" w skrzyni odpowiada preview edytora — `needs verification`.
- **Sort dropdown w `SortOrderTab.tsx`** — historycznie planowane (Acquisition ↑/↓, Weight, Type), ale aktualny komponent ich **nie eksponuje**. Czy zostały świadomie usunięte, czy planowane jest przywrócenie — `needs verification`.
- **ACQUISITION ↑/↓ direction naming** — wcześniejszy bug z odwróconą semantyką względem gry. Czy został rozwiązany, czy zniknął wraz z usunięciem dropdownów — `needs verification`.
- **Stable tie behavior przy multiple reorderach** — stride-2 gwarantuje brak kolizji w jednym zapisie. Multiple reorderów z różnymi `base` mogą produkować items z identycznym `Index` historycznie pochodzącym z różnych base; czy gra zachowuje stabilność tie-breaker — `needs verification`.
- **Brak `SnapshotSlot`/`RestoreSlot` i `ValidatePostMutation` w reorder path** — `needs verification`, czy świadoma decyzja czy luka transactional safety.
- **Workspace path test coverage** — kompletność testów dla `backend/editor/save.go::writeContainerLayout` (passthrough + editable + reservedAcq skip-over) — `needs verification` przez detailed test inventory.
- **`InventoryOrderItem.Weight`, `SortId`, `SortGroupId` populacja** — pola istnieją w DTO (`app_inventory_order.go:21-23`), ale czy są aktywnie wypełniane z `data.ItemData` (Faza 2 z [39-inventory-reorder](39-inventory-reorder.md)) — `needs verification`.

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — binary model GaItem/GaMap; rekord inventory referuje GaItem przez handle.
- [07-inventory](07-inventory.md) — Inventory data model, `InventoryItem` 12 B layout, `NextAcquisitionSortId`/`NextEquipIndex` counters, bramka widoczności.
- [10-storage](10-storage.md) — Storage data model, różnice względem Inventory, Storage Apply in-game verification status.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator/capacity/counters, transactional safety, snapshot/rollback (canonical reference dla reszty write-side semantyki).
- [39-inventory-reorder](39-inventory-reorder.md) — historyczny design doc; **stride-1 algorytm z 39 jest niepoprawny** (zob. sentinel v2 discovery).
- [43-transactional-item-adding](43-transactional-item-adding.md) — Add Items pipeline; `Index` przypisywany single-stride z `NextAcquisitionSortId` (nie stride-2).
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — Inventory ↔ Storage transfer; przypisanie `Index` przy transferze (single-stride, monotoniczne advancement); UI flow Sort Order (z historical wzmiankami o sort dropdownach — patrz "Direction naming and UI caveats" tutaj).

---

## Sources

- `app_inventory_order.go` — `ReorderInventory`, `ReorderStorage`, `GetInventoryOrder`, `GetStorageOrder`, `GetWeaponInventoryOrder`, `ReorderWeaponInventory`, `inventoryOrderTabs`, `isWeaponOrderTechnical`, `tabCategorySet`, `InventoryOrderItem`
- `backend/editor/save.go` — `writeContainerLayout` (workspace stride-2 z reservedAcq skip-over), `ApplyWorkspaceSave`
- `app_inventory_session.go` — `SaveInventoryWorkspaceChanges` (entry point dla workspace path)
- `backend/core/structures.go` — `InventoryItem`, `EquipInventoryData`, `mapInventory` (load-time NextAcquisitionSortId reconcile)
- `backend/core/offset_defs.go` — `InvEquipReservedMax = 432`, `InvRecordLen = 12`, `CommonItemCount`, `StorageCommonCount`, `StorageHeaderSkip`
- Tests: `app_inventory_order_test.go`, `app_storage_order_test.go`
- Empiryczna weryfikacja in-game (sentinel v1/v2/v3 na Steam Deck, prawdziwy save PS4 wdrożony przez SSH)
