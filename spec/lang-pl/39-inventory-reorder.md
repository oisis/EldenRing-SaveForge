# 39 — Inventory Reorder — historical design note

> **Typ**: Historical project decision log (superseded by current implementation)
> **Status**: 📜 Historical / partial — pełni rolę logu decyzji projektowych, **nie** aktualnego technical spec.
> **Wyodrębniono z**: `docs/ROADMAP.md` (2026-05-03 cleanup).

---

## Status banner

> ⚠️ **Ten dokument jest historyczny.** Pierwotnie był planem feature'u "Inventory Reorder / Sort Order" w 5 fazach (0-4). Implementacja wzięła z niego **kierunek**, ale w kilku miejscach poszła innym torem:
>
> - **Algorytm reorder** opisany w tym dokumencie (Faza 1, stride-1: `base + i`) jest **niepoprawny**. Faktyczna implementacja używa **stride-2 z parzystą bazą** — odkrycie zostało udokumentowane i jest opisane canonicalnie w [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
> - **UI ↔ transfer mechanika** (inventory drag&drop, transfer Inventory ↔ Storage, dwukolumnowy layout 5×6, dropdowny sort mode, rehandle path) opisana częściowo w starszych iteracjach planu — aktualny canonical opis pozostaje w [53-inventory-storage-transfer](53-inventory-storage-transfer.md). Bieżący komponent UI to `frontend/src/components/SortOrderTab.tsx` (workspace-session model), **niekoniecznie** dokładnie tożsamy z planowanym `InventoryGrid.tsx` toggle w `InventoryTab.tsx`.
> - **Persystencja kolejności per postać** (Faza 4) **nie jest wdrożona** — `backend/vm/preset.go::CharacterPreset` nie zawiera pola `InventoryOrder` (zweryfikowane 2026-05-19).
>
> **Aktualny source of truth**:
> - Algorytm reorder + acquisition index semantics: [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md)
> - Inventory data model: [07-inventory](07-inventory.md)
> - Storage data model: [10-storage](10-storage.md)
> - Inventory ↔ Storage transfer i Sort Order UI: [53-inventory-storage-transfer](53-inventory-storage-transfer.md)
> - Add Items pipeline (capacity + reorder integration): [43-transactional-item-adding](43-transactional-item-adding.md)
>
> Dokument zachowany jako **log decyzji projektowych** dla przyszłych implementatorów chcących zrozumieć, dlaczego feature wygląda tak jak wygląda.

---

## Dlaczego ten dokument jest historyczny

Plan z maja 2026 opisywał:

1. **Mechanikę edycji `acquisition_index`** — z założeniem stride-1 i hipotezą, że Faza 0 weryfikacji in-game potwierdzi semantykę. Weryfikacja Fazy 0 została wykonana (sentinel v1/v2/v3 na Steam Deck) i **odkryła, że stride-1 nie działa** — gra sortuje po `acqIdx >> 1`, więc sąsiednie pary są zamieniane. Stride-2 z parzystą bazą został przyjęty jako finalny algorytm.
2. **Pełny game-styled `InventoryGrid.tsx`** jako toggle widoku Lista / Siatka w `InventoryTab.tsx`, z `@dnd-kit/sortable`, 64×64 kafelkami, custom theme'em i kontekstowym menu. Wdrożenie **nie poszło tą drogą**: zamiast tego powstał osobny `SortOrderTab.tsx` z dwukolumnowym layoutem 5×6 (Inventory + Storage), workspace-session model (`useInventoryWorkspace` hook), i bez sort dropdownów.
3. **Persystencję `inventoryOrder` w `CharacterPreset`** — opcjonalna Faza 4 (2-3h). Nie została wdrożona; obecne presety zachowują items + add settings + world + character core, ale **nie** kolejność acquisition per kategoria.

---

## Current source of truth

Czytelnik szukający **aktualnej** semantyki Inventory Reorder powinien zacząć od:

| Co | Gdzie |
|---|---|
| Algorytm stride-2 (write-side) | [52 → Stride-2 write model](52-acquisition-sort-stride2.md#stride-2-write-model) |
| Bucket-collision guard | [52 → Bucket collision guard](52-acquisition-sort-stride2.md#bucket-collision-guard) |
| Inventory reorder path (legacy `ReorderInventory`) | [52 → Inventory reorder path](52-acquisition-sort-stride2.md#inventory-reorder-path) |
| Storage reorder path (legacy `ReorderStorage`) | [52 → Storage reorder path](52-acquisition-sort-stride2.md#storage-reorder-path) |
| Workspace save integration (`writeContainerLayout`) | [52 → Workspace/editor save integration](52-acquisition-sort-stride2.md#workspaceeditor-save-integration) |
| Inventory data model i `Index` field | [07 → Binary structures](07-inventory.md#binary--runtime-structures) |
| Storage data model i różnice względem Inventory | [10-storage](10-storage.md) |
| `InvEquipReservedMax = 432` (reserved equipment range) | [52 → Acquisition index semantics](52-acquisition-sort-stride2.md#acquisition-index-semantics) + [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants) |
| Sort Order UI flow (workspace, drag&drop, transfer) | [53-inventory-storage-transfer](53-inventory-storage-transfer.md) |

---

## Phase status table

| Faza w planie | Original plan | Actual implementation status | Current source of truth | Notes |
|---|---|---|---|---|
| **0** — Weryfikacja in-game (KRYTYCZNA, 1-2h) | Save z 5 broniami, hex edit `acquisition_index` → 1000..1004, deploy na Steam Deck, weryfikacja sortowania | ✅ **Wykonana** — odkrycie sentinel v1/v2/v3 udokumentowane | [52 → Stride-2 write model → Discovery](52-acquisition-sort-stride2.md#stride-2-write-model) | Wynik weryfikacji: stride-1 z planu Fazy 1 **odrzucony**; stride-2 z parzystą bazą przyjęty jako finalny |
| **1** — Backend API + 2 tryby sortowania (3-5h) | `GetInventoryOrder`, `ReorderInventory`, `SortInventory(mode)`. Algorytm: stride-1 `base + i`, `base = max(NextAcquisitionSortId, 1000)`. Kategorie: weapons, talismans, head/chest/arms/legs. | ✅ **Wdrożone** — z modyfikacjami: algorytm = **stride-2** (nie stride-1), `base = max(NextAcquisitionSortId, InvEquipReservedMax+2)` zaokrąglona do parzystej; `SortInventory(mode)` jako bulk-sort **nie istnieje** (frontend zarządza kolejnością w workspace, backend tylko zapisuje finalne `Index`). Wdrożone funkcje: `app_inventory_order.go::ReorderInventory`, `GetInventoryOrder`, `ReorderStorage`, `GetStorageOrder`, `GetWeaponInventoryOrder`, `ReorderWeaponInventory`. | `app_inventory_order.go` + [52](52-acquisition-sort-stride2.md) | Stride-1 z planu jest **niepoprawny** (sentinel v2 — sąsiednie pary zamienione przez grę). |
| **2** — Import erdb (4 tryby sortowania, 2-3h) | Import `weight`, `attackBasePhysics`, `sortGroupId` do `ItemData` z `tmp/erdb/1.10.0/EquipParam*.csv` przez `scripts/import_erdb.go`. Bulk-sort modes: weight, attackPower, sortGroupId, upgradeLevel. | ⚠️ **Częściowe** — `InventoryOrderItem` DTO (`app_inventory_order.go:14-29`) zawiera pola `Weight`, `SortId`, `SortGroupId`; **`needs verification`**, czy są aktywnie populowane z `data.ItemData` i czy bulk-sort UI byłby kiedykolwiek użyty (sort dropdowny w obecnym UI nie istnieją). | `app_inventory_order.go:14-29` + `data.ItemData` | Komponent `SortOrderTab.tsx` nie pokazuje sort dropdownów — workspace UI używa `Position` field, nie acquisition. |
| **3** — Siatka w stylu gry + drag & drop (8-12h) | `InventoryGrid.tsx` jako toggle widoku Lista/Siatka w `InventoryTab.tsx`. `@dnd-kit/core` + `@dnd-kit/sortable`. 64×64 kafelki, custom theme. Sort dropdown z 6 trybami. | ❌ **Nie wdrożone w opisanej formie** — aktualne UI to `frontend/src/components/SortOrderTab.tsx` (osobna zakładka, **nie** toggle w InventoryTab), dwukolumnowy layout 5×6 (Inventory + Storage), workspace-session model bez sort dropdownów. **`@dnd-kit/sortable` nie jest zainstalowane** w `frontend/package.json` (zweryfikowane 2026-05-19); drag&drop w obecnym `SortOrderTab.tsx` jest realizowane natywnymi HTML drag events. | `frontend/src/components/SortOrderTab.tsx` + [53-inventory-storage-transfer](53-inventory-storage-transfer.md) | Faktyczny UI jest **bliższy** wizji Faza 3 niż brak feature'u, ale **inny** w detalach — zob. spec/53 dla aktualnego opisu. |
| **4** (opcjonalna) — Persystencja kolejności per postać (2-3h) | Dodaj `inventoryOrder: { weapons: [handle1, ...], armor: [...], talismans: [...] }` do `CharacterPreset`. Apply: po `AddItemsToCharacter` → `ReorderInventory` per kategoria. | ❌ **Nie wdrożone** — `backend/vm/preset.go::CharacterPreset` (zweryfikowane 2026-05-19) nie zawiera pola `InventoryOrder` ani `Order` ani analogu. Preset zachowuje: `Inventory []PresetItem`, `Storage []PresetItem`, `AddSettings`, `World`, `CharacterPresetCore` — bez kolejności acquisition. | `backend/vm/preset.go:15-24` (`CharacterPreset`) | Po imporcie preset itemy dostają świeże stride-2 `Index` (z `AddItemsToCharacter` Phase 3 → counters advance); oryginalna kolejność z source save jest **utracona**. |

---

## What remains useful

Sekcje tego planu, które mają wartość historyczną/projektową **mimo** superseded mechaniki:

- **Sekcja "Mechanika"** (linie 17-23 oryginału) — wstępna analiza, że `acquisition_index` jest pole na offset `+8`, globalny licznik, kontroluje sort "Kolejność zdobycia". Wszystkie te fakty pozostają poprawne. Tylko **sposób przypisywania nowych wartości** (stride-1) okazał się błędny.
- **Sekcja "Pułapki"** (linie 27-32) — czterocyfrowe pułapki dotyczące `InvEquipReservedMax = 432`, sortowania per kategoria, stackowalnych itemów, braku danych dla `Weight`/`AttackPower`/`SortGroupId` — wszystkie wartościowe jako early-stage risk assessment. Aktualny canonical w [52](52-acquisition-sort-stride2.md) i [07](07-inventory.md).
- **Sekcja "Otwarte pytania"** (linie 146-152) — niektóre rozstrzygnięte (Faza 0 verification z sentinel v1/v2/v3 → ANULUJ stride-1 → stride-2), inne wciąż otwarte (np. reset behavior, persistence po WriteSave). Patrz "Historical notes" niżej.

---

## Superseded mechanics

| Pierwotne stwierdzenie w 39 | Status |
|---|---|
| "stride-1 `base + i`, `base = max(NextAcquisitionSortId, 1000)`" | ❌ **superseded** — algorytm rzeczywisty: stride-2 + parzysta baza `>= InvEquipReservedMax + 2`. Zob. [52 → Stride-2 write model](52-acquisition-sort-stride2.md#stride-2-write-model). |
| "Faza 0 KRYTYCZNA — jeśli `acquisition_index` nie kontroluje sortowania, ANULUJ feature" | ✅ Weryfikacja wykonana, hipoteza częściowo prawdziwa: `acquisition_index` kontroluje sort, ale gra używa `acqIdx >> 1` jako bucket key, nie pełnej wartości. Zob. [52 → Discovery](52-acquisition-sort-stride2.md#stride-2-write-model). |
| "Faza 1 — `SortInventory(mode)` jako bulk-sort API" | ❌ **niewdrożony** — frontend zarządza kolejnością przez workspace `Position`; backend tylko zapisuje finalne `Index`. |
| "Faza 3 — `InventoryGrid.tsx` toggle w `InventoryTab.tsx`, 64×64 kafelki, sort dropdown z 6 trybami" | ❌ **niewdrożony w opisanej formie** — aktualne `SortOrderTab.tsx` to osobna zakładka, 5×6 grid, brak sort dropdownów. Zob. [53-inventory-storage-transfer](53-inventory-storage-transfer.md) dla aktualnego UI. |
| "Faza 4 — `inventoryOrder` w `CharacterPreset`" | ❌ **niewdrożony** — preset format nie zawiera kolejności acquisition. |

---

## Links to canonical chapters

- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — **canonical** dla algorytmu reorder, stride-2, bucket collision, Inventory/Storage paths, workspace integration.
- [07-inventory](07-inventory.md) — **canonical** dla Inventory data model, `InventoryItem` 12 B layout, `NextAcquisitionSortId`/`NextEquipIndex` counters.
- [10-storage](10-storage.md) — **canonical** dla Storage data model i różnic względem Inventory.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — **canonical** dla Inventory ↔ Storage transfer i Sort Order UI flow (z notą, że obecny `SortOrderTab.tsx` nie eksponuje sort dropdownów — patrz "Direction naming and UI caveats" w [52](52-acquisition-sort-stride2.md#direction-naming-and-ui-caveats)).
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator/capacity/counter invariants; reserved equipment range `InvEquipReservedMax = 432`.
- [43-transactional-item-adding](43-transactional-item-adding.md) — Add Items pipeline; przypisanie `Index` przy add (single-stride, nie stride-2).

---

## Historical notes

Sekcje tego planu, które chcemy zachować **dosłownie** jako log decyzji projektowych (NIE są aktualnym spec):

### Z oryginalnej sekcji "Faza 0 — Weryfikacja in-game"

> Test:
> 1. Save z 5 broniami w slotach
> 2. Hex edit `acquisition_index` 5 broni → 1000, 1001, 1002, 1003, 1004
> 3. Deploy na Steam Deck → załaduj save
> 4. W grze: przełącz sortowanie na Kolejność zdobycia
> 5. Sprawdź czy bronie są w kolejności [1000..1004] rosnąco
> 6. Podnieś nowy item w grze → zweryfikuj że nasze 5 broni zachowuje kolejność (nowy item na końcu jako `next_acquisition_sort_id`)

**Realne wykonanie**: krok 2 użył 65 talizmanów zamiast 5 broni; testy v1/v2/v3 odkryły bucket key `acqIdx >> 1`. Wynik udokumentowany w [52](52-acquisition-sort-stride2.md).

### Z oryginalnej sekcji "Otwarte pytania"

Lista pytań pozostawiona w oryginale:

1. ~~Weryfikacja Fazy 0: użytkownik przygotowuje save z hex-editem, czy edytor produkuje testowy save?~~ → **rozwiązane**: użytkownik przygotował, weryfikacja odbyła się przez Steam Deck deploy.
2. ~~Zakres kategorii: tylko broń + zbroja + talizmany, czy też tarcze / AoW / Popioły?~~ → **rozwiązane**: 6 tabów (`weapons` = melee + ranged + shields, `talismans`, `head`, `chest`, `arms`, `legs`). AoW / ashes / goods **poza** zakresem reorder.
3. **Zachowanie przycisku Reset**: powrót do `sortGroupId` (domyślny w grze), `acquisition` (oryginalna kolejność podniesienia), czy wyłączenie po przeciągnięciu? → **nie ma "Reset" przycisku** w obecnym UI; workspace session ma `Position`, nie sort mode.
4. **Persystencja po WriteSave**: zachować niestandardową kolejność na zawsze (do Resetu), czy unieważnić po Add Items? → Add Items advance `NextAcquisitionSortId` — nowe itemy lądują **na końcu** Acquisition ↑; istniejąca kolejność zachowana.
5. ~~Zarezerwowane sloty 0-432: nasza zmiana kolejności je pomija (rekomendowane) czy re-indeksuje?~~ → **rozwiązane**: `ReorderInventory`/`ReorderStorage` zaczyna `base = max(NextAcquisitionSortId, InvEquipReservedMax + 2)` — slotów ≤ 432 nie dotyka.

### Z oryginalnej sekcji "Podsumowanie faz"

Tabela szacunkowych nakładów pracy (oryginalna):

| Faza | Nakład pracy oryginalny |
|---|---|
| 0 Weryfikacja in-game | 1-2h |
| 1 Backend API + 2 tryby sortowania | 3-5h |
| 2 Import erdb + 4 tryby sortowania | 2-3h |
| 3 Siatka UI + drag & drop | 8-12h |
| 4 (opc.) Persystencja presetów | 2-3h |
| **Minimum łącznie (0+1+3)** | **12-19h** |
| **Pełne łącznie (0-3)** | **14-22h** |

**Faktyczne wdrożenie** zajęło osobne sesje rozwoju, w innym kształcie niż plan — zob. `docs/CHANGELOG.md` dla wpisów `acquisition`/`sort`/`reorder`.

---

## Status końcowy dokumentu

- **Rola obecna**: historical project decision log.
- **Aktualizacje**: dokument nie jest aktywnie utrzymywany. Wszelkie zmiany write-path / algorytm / UI idą do canonical chapter ([52](52-acquisition-sort-stride2.md), [53](53-inventory-storage-transfer.md), [07](07-inventory.md), [10](10-storage.md)).
- **Wartość**: log decyzji projektowych dla implementatorów chcących zrozumieć **dlaczego** Sort Order wygląda tak jak wygląda i **co** zostało odrzucone (stride-1, `InventoryGrid.tsx` toggle, preset persistence).
- **Status w księdze**: pozostaje w głównym katalogu `spec/lang-pl/` jako historical design note (superseded by 52/53).
