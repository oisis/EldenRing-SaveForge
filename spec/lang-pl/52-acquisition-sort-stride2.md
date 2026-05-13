# 52 — Sortowanie Acquisition: Przypisywanie Indeksów Stride-2

> **Typ**: Design doc
> **Status**: ✅ Zaimplementowany
> **Zakres**: Wyjaśnia dlaczego `ReorderInventory` używa przypisania stride-2 (`base + pos*2`) zamiast stride-1 (`base + pos`), i dokumentuje odkrycie in-game klucza sortowania.

---

## Tło

Gra sortuje przedmioty w widoku "Kolejność zakupu" używając pola `acquisition_index` na `offset+8` w 12-bajtowym rekordzie `InventoryItem` (`handle u32 | qty u32 | acqIdx u32`). Pierwotny plan (spec/39 Faza 1) zakładał, że stride-1 jest wystarczający: pozycji `i` przypisać indeks `base + i`. To było błędne.

---

## Odkrycie: Gra Sortuje po `acqIdx >> 1`

### Metodologia testów sentinel

Przeprowadzono trzy testy sentinel na prawdziwym save wdrożonym na Steam Deck, używając zakładki talizmany (65 przedmiotów):

| Test | Przypisanie | Wynik |
|---|---|---|
| Sentinel v1 (hardkodowane 10000–10012) | `10000, 10001, ..., 10012` | **Crash gry** przy ładowaniu save |
| Sentinel v2 (bezpieczny, stride-1) | `minAcq, minAcq+1, ..., minAcq+N` | Gra załadowana, **sąsiednie pary zamienione** |
| Sentinel v3 (stride-2) | `base, base+2, ..., base+N*2` (parzyste base) | Gra załadowana, **kolejność poprawna** |

### Przyczyna źródłowa

Gra grupuje przedmioty do sortowania "Kolejność zakupu" po `acqIdx >> 1` (przesunięcie bitowe w prawo o 1), nie po pełnej wartości 32-bitowej. Dwa sąsiednie indeksy stride-1 `k` i `k+1` trafiają do tego samego kubełka gdy `k` jest parzyste: `k>>1 == (k+1)>>1`. Wewnątrz wspólnego kubełka gra stosuje dodatkowy klucz sortowania (prawdopodobnie `sortGroupId` lub handle), nadpisując zamierzoną kolejność.

Stride-2 z parzystą bazą całkowicie tego unika: `(base + 2*i) >> 1 = base/2 + i`, co jest ściśle rosnące — unikalny kubełek na przedmiot.

---

## Algorytm

```go
// base: zaczynamy od NextAcquisitionSortId; zapewniamy parzyste i > InvEquipReservedMax (432).
base := slot.Inventory.NextAcquisitionSortId
if base <= uint32(core.InvEquipReservedMax) {
    base = uint32(core.InvEquipReservedMax) + 2  // 434 — minimalna bezpieczna wartość parzysta
}
if base%2 != 0 {
    base++
}

// Przypisanie: przedmiot na pozycji pos otrzymuje indeks base + pos*2.
for pos, handle := range orderedHandles {
    newIdx := base + uint32(pos)*2
    // zapis do slot.Data[off+8:] i slot.Inventory.CommonItems[j].Index
}

// Zwiększenie NextAcquisitionSortId (tylko w górę).
expectedMax := base + uint32(len(orderedHandles)-1)*2
newNextAcq := expectedMax + 1
```

### Dowód unikalności kubełków

Dla parzystej `base` i pozycji `i`:

```
bucket(i) = (base + 2*i) >> 1
           = base/2 + i
```

Ponieważ `base` jest parzyste, `base/2` jest liczbą całkowitą. `base/2 + i` jest ściśle rosnące w `i` → żadne dwie pozycje nie dzielą kubełka.

---

## Bezpieczny Zakres Wartości

Wartości ≤ 432 są zarezerwowane dla slotów ekwipunku (`InvEquipReservedMax = 432`). Wartości `>= 10000` powodują crash gry przy ładowaniu (odkrycie sentinel v1). Prawdziwe indeksy postaci typowo mieszczą się w zakresie 500–2000 w zależności od czasu gry. Algorytm stride-2 pozostaje w tym bezpiecznym zakresie używając `NextAcquisitionSortId` jako bazy.

---

## Zabezpieczenie Defensywne

`ReorderInventory` zawiera sprawdzenie kolizji przed jakąkolwiek mutacją:

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

Ta blokada jest nieosiągalna przy poprawnej parzystej bazie i kroku stride-2, ale zapobiega cichym regresjom jeśli logika base/stride kiedykolwiek zostanie zmieniona.

---

## Lokalizacja Implementacji

- `app_inventory_order.go` — funkcja `ReorderInventory`
- Dotyczy wszystkich zakładek: `weapons`, `talismans`, `head`, `chest`, `arms`, `legs`

---

## Źródła

- spec/39: oryginalny design doc dla funkcji Inventory Reorder
- Empiryczne testy in-game na Steam Deck (prawdziwy save PS4 wdrożony przez SSH)
