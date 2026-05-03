# 16 — World State (Stan Świata)

> **Zakres**: FieldArea, WorldArea, WorldGeomMan, RendMan — geometria, NPC, obiekty świata.

---

## Opis ogólny

Po Event Flags następuje seria struktur opisujących stan fizycznego świata gry — pozycje NPC, stan obiektów, geometrię terenu, dane renderera. Wszystkie mają zmienną długość (size-prefixed).

---

## Kolejność sekcji

```
1. FieldArea                    [VARIABLE: 4 + size]
2. WorldArea                    [VARIABLE: 4 + size]
3. WorldGeomMan (instancja 1)   [VARIABLE: 4 + size]
4. WorldGeomMan (instancja 2)   [VARIABLE: 4 + size]
5. RendMan                      [VARIABLE: 4 + size]
```

---

## 1. FieldArea [VARIABLE]

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | i32 | Size (ilość bajtów danych po tym polu) |
| 0x04 | u8[size] | Data |

Zawartość FieldArea — dane regionu w którym gracz się znajduje. Szczegóły wewnętrzne nieznane.

---

## 2. WorldArea [VARIABLE]

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | i32 | Size |
| 0x04 | u8[size] | Data |

Wewnątrz: WorldAreaChrData — dane NPC/postaci w świecie, podzielone na bloki per mapa:

### WorldAreaChrData (wewnętrzna struktura):
```
┌─────────────────────────────────┐
│ Magic (4 bytes)                  │
│ unk_0x21042700 (u32)            │
│ unk0x8 (u32)                    │
│ unk0xc (u32)                    │
├─────────────────────────────────┤
│ WorldBlockChrData[] (powtarzane) │
│   ├── magic (4B)                 │
│   ├── map_id (4B)               │
│   ├── size (i32)                │
│   ├── unk0xc (u32)              │
│   └── data[size-0x10]           │
│ ... (do size < 1 = terminator)   │
└─────────────────────────────────┘
```

---

## 3-4. WorldGeomMan (×2) [VARIABLE]

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | i32 | Size |
| 0x04 | u8[size] | Data |

Wewnątrz: WorldGeomData — geometria świata per mapa:

### WorldGeomData (wewnętrzna struktura):
```
┌─────────────────────────────────┐
│ Magic (4 bytes)                  │
│ unk_0x4 (u32)                   │
├─────────────────────────────────┤
│ WorldGeomDataChunk[] (powtarzane)│
│   ├── map_id (4B)               │
│   ├── size (i32)                │
│   ├── unk_0x8 (u64)            │
│   └── data[size-0x10]           │
│ ... (do size < 1 = terminator)   │
└─────────────────────────────────┘
```

Dwie instancje — prawdopodobnie "before" i "after" state, lub two layers geometrii.

---

## 5. RendMan [VARIABLE]

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | i32 | Size |
| 0x04 | u8[size] | Data |

Renderer Manager — dane stanu renderera (oświetlenie, particles, efekty wizualne świata).

Wewnątrz: StageMan — lista wpisów o stałym rozmiarze:
```
count (i32) + count × entry_data
```

---

## Implikacje dla edycji

- Sekcje World State są duże i mają zmienną długość — edycja wymaga ostrożności
- **Typowo nie edytuje się bezpośrednio** — te sekcje odtwarzają się z danych gry
- Uszkodzenie tych sekcji = respawn NPC w złych miejscach, brak obiektów
- Bezpieczne podejście: kopiowanie blob-to-blob przy transferze save między platformami
- Size == 0: sekcja pusta (legalne dla nowych postaci)

---

## Źródła

- er-save-manager: `parser/world.py` — `FieldArea` (410-437), `WorldArea` (525-552), `WorldGeomMan` (621-648), `RendMan` (709-738)
- er-save-manager: `parser/world.py` — `WorldAreaChrData` (486-522), `WorldGeomData` (589-618)
- er-save-manager: `parser/user_data_x.py` linie 168-172
