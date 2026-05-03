# 43 — Transakcyjne dodawanie przedmiotów (zapobieganie crashom)

> **Typ**: Dokument projektowy
> **Wyciągnięto z**: docs/ROADMAP.md (czyszczenie 2026-05-03)
> **Status**: ✅ Zaimplementowane (v0.7.2)

---

## Problem

`AddItemsToCharacter` modyfikuje slot bez walidacji capacity i bez rollbacku. Partial failure (pełny inventory, pełna tablica GaItems, pełny GaItemData) zostawia slot w niespójnym stanie: osierocone GaItems, handle bez wpisu inventory, uszkodzony counter. Gra crashuje przy ładowaniu (`EXCEPTION_ACCESS_VIOLATION`).

## Zasada projektowa

**ALL-OR-NOTHING** — albo wszystkie żądane itemy zostają dodane, albo żaden. Częściowy zapis = uszkodzony save = niedopuszczalny.

---

## Architektura

```
     ┌─────────────────────────────────────────────────────────┐
     │               AddItemsToCharacter (app.go)              │
     │                                                         │
     │  1. PRE-COMPUTE: finalIDs, quantities, container caps   │
     │  2. PRE-FLIGHT: CheckSlotCapacity() — wszystko się      │
     │     zmieści?                                            │
     │     └─ NIE → return AddResult{CapHit, 0 added}         │
     │     └─ TAK → kontynuuj                                  │
     │  3. SNAPSHOT: głęboka kopia stanu slota                  │
     │  4. MUTATE: AddItemsToSlotBatch() — jeden rebuild       │
     │  5. POST-FLAGS: event flags, tutorials, containers      │
     │  6. VALIDATE: ValidateSlotIntegrity() — inwarianty OK?  │
     │     └─ NIE → ROLLBACK do snapshota, return error        │
     │     └─ TAK → commit, return AddResult{success}          │
     └─────────────────────────────────────────────────────────┘
```

---

## Kroki implementacji

### Krok 1 — Naprawa cichego overflow w `upsertGaItemData` 🔴

Gdy `count >= GaItemDataMaxCount (7000)`, zwracał `nil` zamiast błędu. GaItem zostaje utworzony ale nigdy zarejestrowany → osierocone metadane → crash gry.

**Naprawa:** Zwróć `fmt.Errorf(...)` zamiast `nil`.

### Krok 2 — Wstępne sprawdzenie pojemności (pre-flight) 🔴

`CheckAddCapacity(slot, items []ItemToAdd) (canFitAll bool, details CapacityReport)`

Zlicza: wolne miejsca w inventory CommonItems (2688 - zajęte), storage CommonItems (1920 - zajęte), GaItems (5120 - zajęte), GaItemData (7000 - count).

### Krok 3 — Typ zwracany `AddResult` + semantyka all-or-nothing 🔴

```go
type AddResult struct {
    Added       int          `json:"added"`
    Requested   int          `json:"requested"`
    Trimmed     []SkippedAdd `json:"trimmed"`
    CapHit      string       `json:"capHit"`
    FreeInv     int          `json:"freeInv"`
    FreeStore   int          `json:"freeStore"`
    NeededInv   int          `json:"neededInv"`
    NeededStore int          `json:"neededStore"`
}
```

### Krok 4 — Snapshot + rollback 🔴

```go
snapshot := SnapshotSlot(slot)   // deep copy: Data, GaItems, GaMap, Inventory, Storage, all indices
// ... mutacja ...
// w przypadku błędu:
RestoreSlot(slot, snapshot)      // przywróć wszystkie pola
```

Oddzielny od `pushUndo()` — wewnętrzny mechanizm bezpieczeństwa, nie dotyka stosu undo.

### Krok 5 — Przebudowa batch (`AddItemsToSlotBatch`) 🟡

```go
type ItemToAdd struct {
    ItemID         uint32
    InvQty         int
    StorageQty     int
    ForceStackable bool
}

func AddItemsToSlotBatch(slot *SaveSlot, items []ItemToAdd) error
```

Jeden `RebuildSlotFull` zamiast N. 50 broni: ~100ms (batch) vs ~2.5-5s (per-item).

### Krok 6 — Walidacja po zapisie (`ValidatePostMutation`) 🔴

Szybkie sprawdzenie inwariancji po każdej mutacji:
1. Każdy niepusty handle inventory istnieje w GaMap
2. Każdy niestackowalny wpis GaMap ma rekord GaItem
3. Brak zduplikowanych wartości Index
4. NextEquipIndex > max(wszystkie indeksy)
5. Licznik GaItemData zgadza się z faktycznymi wpisami
6. Nagłówek count storage poprawny
7. NextAoWIndex <= NextArmamentIndex <= len(GaItems)
8. Żaden handle nie referencuje itemID=0

Cel wydajnościowy: <10ms.

### Krok 7 — Klasyfikacja błędów event flag 🟡

Zamień `_ = db.SetEventFlag(...)` na logowane ostrzeżenia. Niekrytyczne (duplikacja AoW, world pickup, tutorial, container) → tylko loguj.

### Krok 8 — Obsługa `AddResult` na frontendzie 🟡

- Brak pojemności → toast z błędem
- Przycięcia containerów → log w konsoli
- Sukces → toast z liczbą
- Modal zawsze się zamyka, refresh zawsze się odpala

### Krok 9 — Rekoncyliacja nagłówka count storage 🟡

Po batch add uzgodnij nagłówek count storage z faktyczną liczbą niepustych wpisów.

### Krok 10 — Testy 🔴

16 testów w `tests/capacity_test.go`:
- PreFlight (pusty/prawie-pełny/pełny/mixed-stackable)
- AllOrNothing (przekroczona pojemność / błąd w trakcie dodawania → rollback)
- BatchRebuild (pojedynczy vs wiele / wydajność <200ms)
- PostValidation (osierocony handle / zduplikowany index / niezgodność countera)
- StorageHeaderReconcile
- GaItemDataFull error
- Roundtrip (pełny inventory / batch 300 itemów)
- AddResult container cap trim

---

## Kolejność implementacji

| Kolejność | Krok | Priorytet | Szac. czas |
|-----------|------|-----------|------------|
| 1 | Naprawa upsertGaItemData | 🔴 | 15 min |
| 2 | Snapshot/rollback | 🔴 | 1-2h |
| 3 | Wstępne sprawdzenie pojemności | 🔴 | 2-3h |
| 4 | Walidacja po zapisie | 🔴 | 2-3h |
| 5 | Typ AddResult | 🔴 | 1-2h |
| 6 | Logowanie event flag | 🟡 | 30 min |
| 7 | Rekoncyliacja nagłówka storage | 🟡 | 1h |
| 8 | Przebudowa batch | 🟡 | 3-4h |
| 9 | Frontend AddResult | 🟡 | 1-2h |
| 10 | Pełny zestaw testów | 🔴 | 3-4h |

**Razem:** 15-22h
