# 25 — Runtime vs Save File Offsets

> **Zakres**: Mapowanie między offsetami w pamięci (Cheat Engine) a offsetami w pliku save. Ostrzeżenia i konwersje.

---

## Problem

Cheat Engine tables operują na **runtime memory** — dane w pamięci RAM procesu gry. Plik save (.sl2) ma **inny layout** niż pamięć runtime. Nie można bezpośrednio użyć offsetów z CT jako offsetów w pliku save.

---

## Kluczowe różnice

### 1. Runtime pointers vs sequential save

W pamięci dane są rozproszone i dostępne przez pointer chains:
```
GameDataMan → +0x08 → PlayerGameData (struktura w RAM)
GameDataMan → +0x08 → +0x408 → EquipInventoryData
GameDataMan → +0x08 → +0x518 → EquipMagicData
```

W pliku save dane są **sekwencyjne** — jedna sekcja po drugiej bez pointerów.

### 2. Runtime header offset

PlayerGameData w pamięci ma dodatkowy runtime header (vtable pointer, PlayerNo):
- **Memory**: HP na offset +0x10 od base
- **Save file**: HP na offset ~0x08 od początku PlayerGameData sekcji

Różnica NIE jest stała:
| Pole | Memory (CT) | Save file | Diff |
|---|---|---|---|
| HP | +0x10 | +0x08 | 0x08 |
| Vigor | +0x3C | +0x34 | 0x08 |
| Level | +0x68 | +0x60 | 0x08 |
| Name | +0x9C | +0x94 | 0x08 |
| Gender | +0xBE | +0xB6 | 0x08 |

Dla PlayerGameData: **save_offset ≈ memory_offset - 0x08** (po odjęciu 8-bajtowego runtime header).

### 3. Struktury dostępne tylko w pamięci

Niektóre dane z CT **nie istnieją w pliku save** — są obliczane przy load:
- Aktualne odporności (Immunity/Robustness/Focus/Vitality current values)
- Character flags (NoDead, NoDamage, etc.)
- Team type (host/phantom/invader)
- Poise/Toughness (live calculation)
- Animation state
- AI state (NPC)

### 4. Struktury dostępne tylko w save

Niektóre dane istnieją w save ale nie mają bezpośredniego runtime odpowiednika:
- GaItem Map (zarządzane przez game engine)
- Event flags raw bitfield (dostępne przez EventFlagMan API)
- World state blobs (FieldArea, WorldGeomMan, RendMan)
- PlayerGameData Hash

---

## Bezpieczne mapowanie (potwierdzone)

Te pola mają **potwierdzone** odpowiedniki w obu domenach:

| Dane | CT (memory) | Save file |
|---|---|---|
| Attributes (8×u32) | GameDataMan+0x08+0x3C..0x58 | PlayerGameData+0x34..0x50 |
| Level | GameDataMan+0x08+0x68 | PlayerGameData+0x60 |
| Runes | GameDataMan+0x08+0x6C | PlayerGameData+0x64 |
| Name | GameDataMan+0x08+0x9C | PlayerGameData+0x94 |
| Gender | GameDataMan+0x08+0xBE | PlayerGameData+0xB6 |
| Class | GameDataMan+0x08+0xBF | PlayerGameData+0xB7 |
| Death Count | GameDataMan+0x94 | Game State section [0x00] |
| NG+ | GameDataMan+0x120 | Game State section 1 [0x00] |
| Event Flags | EventFlagMan+0x28+offset | EventFlags section [offset] |

---

## Zasady korzystania z CT danych

1. **Nazwy i typy pól**: Zawsze wiarygodne (np. "Vigor" = u32, "Gender" = u8)
2. **Kolejność pól**: Wiarygodna (pola w tej samej strukturze zachowują kolejność)
3. **Wartości enum**: Wiarygodne (np. Class: 0=Vagabond, ArmStyle: 0=Empty/1=OneHand)
4. **Offsety bezwzględne**: NIGDY nie używaj bezpośrednio — przelicz lub zweryfikuj hex dumpem
5. **Pointer chains**: Informują o strukturze logicznej, nie o layout pliku save
6. **"Runtime only" dane**: Nie szukaj ich w save (HP regen rate, AI flags, poise)

---

## Weryfikacja offset — metoda

Aby potwierdzić offset w save file:
1. Załaduj znany save do hex editora
2. Znajdź znaną wartość (np. character name w UTF-16LE)
3. Oblicz offset od początku slotu
4. Porównaj z oczekiwanym offsetem z parsera (er-save-manager)

---

## Źródła

- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — runtime pointer chains
- Cheat Engine: `ER_TGA_v1.9.0` — runtime offsets
- er-save-manager: `parser/` — save file sequential parsing (ground truth)
