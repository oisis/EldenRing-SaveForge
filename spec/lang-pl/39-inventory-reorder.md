# 39 — Niestandardowa kolejność ekwipunku (siatka Drag & Drop)

> **Typ**: Dokument projektowy
> **Wyodrębniono z**: ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planowany — zablokowany przez weryfikację in-game w Fazie 0

---

## Cel

Umożliwienie graczowi wyboru trybu sortowania (Zdobycie / Alfabetycznie / Typ przedmiotu / Waga / Siła ataku / Ulepszenie) oraz ułożenia przedmiotów w **dowolnej niestandardowej kolejności drag & drop** w widoku siatki w stylu gry. Początkowy zakres: **broń + zbroja + talizmany** (niestackowalne). Tarcze + AoW + Popioły — opcjonalna kontynuacja.

**Unikalna funkcja** — ani `er-save-manager`, ani `ER-Save-Editor/Rust` tego nie oferują.

---

## Mechanika (zweryfikowana w researchu, PRZED kodowaniem wymaga testu in-game w Fazie 0)

- Save posiada pole `acquisition_index` (offset `0x08` w 12-bajtowym `InventoryItem`, mapowane na `core.InventoryItem.Index`)
- Jest to **globalny licznik** inkrementowany przy każdym podniesieniu (`next_acquisition_sort_id` na końcu sekcji)
- **To pole kontroluje sortowanie "Kolejność zdobycia"** w grze. Niestandardową kolejność można ustawić WYŁĄCZNIE manipulując `acquisition_index`
- Inne sortowania w grze (Typ przedmiotu / Waga / Siła ataku / Alfabetycznie) są **obliczane w runtime** z parametrów `regulation.bin` (`EquipParamWeapon.sortGroupId`/`sortId`, itd.) — NIE w save, edytor nie może ich zmienić
- Konsekwencja: nasza niestandardowa kolejność jest widoczna tylko gdy gracz ma sortowanie ustawione na "Kolejność zdobycia" (domyślne po świeżym załadowaniu)

---

## Pułapki

- **Zarezerwowany zakres indeksów 0-432** (`InvEquipReservedMax`) — zarezerwowane dla slotów wyposażenia. Niestandardowa kolejność MUSI używać `Index >= 433` — najlepiej: `base = max(NextAcquisitionSortId, 1000)` jako bufor
- **Zmiana kolejności per kategoria** — gra pokazuje pod-zakładki (Narzędzia / Broń biała / Tarcze / itd.), sortowanie jest per zakładka. `sortGroupId` definiuje grupę. Zmiana kolejności broni nie wpływa na talizmany
- **Stackowalne** (materiały eksploatacyjne, materiały, AoW) — gra może grupować je inaczej; początkowy zakres ograniczony do niestackowalnego wyposażenia, gdzie mechanika `acquisition_index` jest zweryfikowana
- **Brak danych dla pozostałych trybów sortowania** — `ItemData` nie ma `Weight` / `AttackPower` / `SortGroupId`. Trzeba zaimportować z `tmp/erdb/1.10.0/EquipParam*.csv` przez `scripts/import_erdb.go`

---

## Faza 0 — Weryfikacja in-game (KRYTYCZNA, 1-2h)

Test:
1. Save z 5 broniami w slotach
2. Hex edit `acquisition_index` 5 broni → 1000, 1001, 1002, 1003, 1004
3. Deploy na Steam Deck → załaduj save
4. W grze: przełącz sortowanie na Kolejność zdobycia
5. Sprawdź czy bronie są w kolejności [1000..1004] rosnąco
6. Podnieś nowy item w grze → zweryfikuj że nasze 5 broni zachowuje kolejność (nowy item na końcu jako `next_acquisition_sort_id`)

**Jeśli nie potwierdzone → ANULUJ cały feature** (hipoteza o `acquisition_index` jest błędna).

---

## Faza 1 — Backend API do zmiany kolejności + 2 tryby sortowania (3-5h)

```go
// Zwraca uporządkowaną listę uchwytów per kategoria dla aktualnego stanu.
func (a *App) GetInventoryOrder(charIdx int, category string) ([]uint32, error)

// Ustawia nowy acquisition_index per uchwyt w podanej kolejności.
// Indeksy: base, base+1, base+2... gdzie base = max(NextAcquisitionSortId, 1000)
// Aktualizuje licznik next_acquisition_sort_id po zakończeniu.
func (a *App) ReorderInventory(charIdx int, category string, orderedHandles []uint32) error

// Sortowanie zbiorcze wg trybu. Faza 1: "acquisition" | "alphabetical".
// Faza 2 dodaje: "weight" | "attackPower" | "sortGroupId" | "upgradeLevel".
func (a *App) SortInventory(charIdx int, category string, sortMode string) error
```

**Kategorie (zakres Fazy 1)**: `"melee_armaments"`, `"head"`, `"chest"`, `"arms"`, `"legs"`, `"talismans"`.

**Implementacja `ReorderInventory`:**
1. `pushUndo(charIdx)`
2. Walidacja: każdy uchwyt istnieje w `slot.Inventory.CommonItems`, należy do podanej kategorii
3. Rezerwacja zakresu: `base = max(slot.Inventory.NextAcquisitionSortId, 1000)`
4. Per uchwyt: znajdź slot w `CommonItems`, ustaw `Index = base + i`, zapisz przez `SlotAccessor.WriteU32` pod `commonStart + slotIdx*12 + 8`
5. Aktualizacja `slot.Inventory.NextAcquisitionSortId = base + len(orderedHandles)`, zapis pod `nextAcqSortIdOff`

**Testy:**
- Zmiana kolejności 5 broni → przeładuj save → kolejność się zgadza
- Zmiana kolejności respektuje zarezerwowany zakres (Index >= 433 zawsze)
- Mieszanie kategorii: zmiana kolejności broni nie zmienia indeksów talizmanów
- `next_acquisition_sort_id` poprawnie zaktualizowany
- Round-trip: Save → Reorder → Write → Read → indeksy się zgadzają

---

## Faza 2 — Import danych dla pozostałych trybów sortowania (2-3h)

Import z `tmp/erdb/1.10.0/EquipParam*.csv`:
- `weight` (broń + zbroja) → `ItemData.Weight float32`
- `attackBasePhysics` (broń) → `ItemData.AttackPower uint32`
- `sortGroupId` (broń + zbroja) → `ItemData.SortGroupId uint32`

Test: `SortInventory(weight)` → top 10 broni musi odpowiadać ręcznemu sortowaniu w grze.

---

## Faza 3 — Siatka w stylu gry + drag & drop (8-12h)

**Nowy komponent**: `InventoryGrid.tsx` — przełącznik widoku Lista / Siatka w `InventoryTab.tsx`.

**Biblioteka DnD**: `@dnd-kit/core` + `@dnd-kit/sortable` + `@dnd-kit/utilities` (~30KB łącznie)

**Szczegóły wizualne (wygląd jak w grze):**
- Tło: `bg-zinc-900` ze wzorem ziarna
- Komórka: `64x64px`, złota ramka przy zaznaczeniu/najechaniu
- Ikona: 56x56 wyśrodkowana, odznaka ilości (prawy-dół), odznaka ulepszenia (lewy-dół)
- Podgląd przeciągania: półprzezroczysta kopia, wskaźnik upuszczenia: złota linia pionowa
- Puste komórki na końcu siatki dla spójności wizualnej

**Interakcje:**
- Kliknięcie → podgląd w `ItemDetailPanel`
- Przeciągnięcie → zmiana kolejności
- Prawy klik / długie naciśnięcie → menu kontekstowe (Usuń / Ustaw ilość / Ulepsz)

**Przepływ stanu:**
```ts
const [sortMode, setSortMode] = useState<SortMode>('acquisition');
const [items, setItems] = useState<ItemViewModel[]>([]);
// Przy zmianie sortMode → SortInventory(charIdx, cat, sortMode), odśwież
// Przy zakończeniu przeciągania → optymistyczna lokalna zmiana kolejności + ReorderInventory(charIdx, cat, newHandles)
// "Reset do domyślnego" → SortInventory(charIdx, cat, 'sortGroupId')
```

---

## Faza 4 (opcjonalna) — Persystencja kolejności per postać (2-3h)

Integracja z eksportem/importem presetów postaci (spec/37):
- Dodaj `inventoryOrder: { weapons: [handle1, ...], armor: [...], talismans: [...] }` do `CharacterPreset`
- Apply: po `AddItemsToCharacter` → wywołaj `ReorderInventory` per kategoria

---

## Podsumowanie faz

| Faza | Nakład pracy |
|---|---|
| **0** Weryfikacja in-game (krytyczna!) | 1-2h |
| **1** Backend API + 2 tryby sortowania | 3-5h |
| **2** Import erdb + 4 tryby sortowania | 2-3h |
| **3** Siatka UI + drag & drop | 8-12h |
| 4 (opc.) Persystencja presetów | 2-3h |
| **Minimum łącznie (0+1+3)** | **12-19h** |
| **Pełne łącznie (0-3)** | **14-22h** |

---

## Otwarte pytania

1. **Weryfikacja Fazy 0**: użytkownik przygotowuje save z hex-editem, czy edytor produkuje testowy save?
2. **Zakres kategorii**: tylko broń + zbroja + talizmany, czy też tarcze / AoW / Popioły / materiały eksploatacyjne?
3. **Zachowanie przycisku Reset**: powrót do `sortGroupId` (domyślny w grze), `acquisition` (oryginalna kolejność podniesienia), czy wyłączenie po przeciągnięciu?
4. **Persystencja po WriteSave**: zachować niestandardową kolejność na zawsze (do Resetu), czy unieważnić po Add Items?
5. **Zarezerwowane sloty 0-432**: nasza zmiana kolejności je pomija (rekomendowane) czy re-indeksuje? Sugestia: nie ruszaj `Index <= 432` — to fizycznie założone przedmioty kontrolowane przez grę przez osobne offsety.
