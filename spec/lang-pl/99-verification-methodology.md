# 99 — Metodologia Weryfikacji i Odkrywania Parametrów

> **Zakres**: Procedury testowe do weryfikacji udokumentowanych offsetów oraz odkrywania nieznanych parametrów w pliku save.

---

## Dostępne zasoby testowe

| Zasób | Ścieżka | Opis |
|---|---|---|
| PC Save | `tmp/save/ER0000.sl2` | Prawdziwy save PC (wiele slotów) |
| PS4 Save | `tmp/save/oisisk_ps4.txt` | Prawdziwy save PS4 |
| Reference parser | `tmp/repos/er-save-manager/` | Python parser (ground truth) |
| Cheat Tables | `tmp/cheat_tables/` | Runtime offsets (Hexinton + TGA) |
| BST Lookup | `tmp/repos/er-save-manager/src/resources/eventflag_bst.txt` | Event flag addressing |
| Nasz parser | `backend/core/reader.go` | Go parser (do porównania) |

---

## Metoda 1: Hex Dump — Known Value Search

**Cel**: Znaleźć offset pola w pliku save na podstawie znanej wartości.

### Procedura:
1. Załaduj save przez nasz parser → wyciągnij znane wartości (name, level, stats)
2. Hex dump slotu do pliku
3. Szukaj wzorca znanej wartości (little-endian)
4. Potwierdź offset — porównaj z dokumentacją

### Przykład — szukanie Character Name:
```bash
# Imię "Zofia" w UTF-16LE = 5A 00 6F 00 66 00 69 00 61 00
xxd -s $SLOT_START -l 0x280000 save.sl2 | grep "5a00 6f00 6600"
```

### Przykład — szukanie Level (np. level 150 = 0x96 = 96 00 00 00 LE):
```bash
# Level 150 w u32 LE
xxd -s $SLOT_START -l 0x280000 save.sl2 | grep "9600 0000"
```

---

## Metoda 2: Binary Diff — Before/After Action

**Cel**: Odkryć które bajty zmieniają się po konkretnej akcji w grze.

### Procedura:
1. Zrób backup save PRZED akcją
2. Wykonaj jedną konkretną akcję w grze (np. level up, pick item, kill boss)
3. Zapisz grę
4. Binary diff dwóch save'ów → lista zmienionych bajtów
5. Odfiltruj znane zmiany (checksum, timestamps) → zostają nowe odkrycia

### Narzędzie:
```bash
# Wyciągnij konkretny slot (np. slot 0, PC z MD5)
dd if=save_before.sl2 bs=1 skip=$((0x300 + 0x10)) count=$((0x280000)) of=slot0_before.bin
dd if=save_after.sl2 bs=1 skip=$((0x300 + 0x10)) count=$((0x280000)) of=slot0_after.bin

# Diff
cmp -l slot0_before.bin slot0_after.bin | head -50
# Format: offset byte_before byte_after
```

### Akcje testowe do wykonania:
- Level up (1 atrybut) → zmiana w PlayerGameData
- Pick up item → zmiana w Inventory + Event Flags
- Kill boss → zmiana w Event Flags
- Rest at grace → zmiana w Game State (LastGrace) + HP/FP/SP
- Change time of day → zmiana w WorldAreaTime
- Equip/unequip item → zmiana w Equipment structures

---

## Metoda 3: Cross-Slot Comparison

**Cel**: Porównać te same offsety między różnymi slotami (postaciami) aby zidentyfikować pola per-character vs stałe.

### Procedura:
1. Wyciągnij N slotów z jednego save file
2. Porównaj bajt po bajcie na tych samych offsetach
3. Pola identyczne = stałe/template
4. Pola różne = per-character data

### Co szukamy:
- Offsety gdzie wartość odpowiada różnicy w levelach między postaciami
- Offsety gdzie pojawia się imię jednej postaci ale nie drugiej
- Bloki zerowe w jednym slocie a niezerowe w innym (unused vs used features)

---

## Metoda 4: Parser Comparison (er-save-manager vs nasz)

**Cel**: Porównać wyniki parsowania tego samego save przez Python reference i nasz Go parser.

### Procedura:
```bash
# Python parser (reference)
cd tmp/repos/er-save-manager
python -c "
from er_save_manager.parser.save import Save
s = Save.from_file('../../tmp/save/ER0000.sl2')
slot = s.slots[0]
print(f'Version: {slot.version}')
print(f'Level: {slot.player_game_data.soul_lv}')
print(f'Name: {slot.player_game_data.player_name}')
print(f'HP: {slot.player_game_data.hp}')
print(f'Vigor: {slot.player_game_data.vigor}')
# ... etc
"

# Nasz Go parser
go test -v -run TestParseComparison ./tests/
```

### Porównywane wartości:
- Wszystkie pola PlayerGameData
- GaItem count i pierwsze/ostatnie wpisy
- Inventory count + pierwsze items
- Event flag spot-checks (znane flagi)
- Dynamic offsets (MagicOffset, EventFlagsOffset, etc.)

---

## Metoda 5: Targeted Byte Probing (Unknown Fields)

**Cel**: Odkryć znaczenie nieznanych pól przez systematyczne modyfikowanie i obserwację efektu w grze.

### Procedura:
1. Wybierz nieznany bajt/pole
2. Zanotuj aktualną wartość
3. Zmień na wartość skrajną (0x00, 0xFF, lub odwrotność)
4. Załaduj save w grze
5. Obserwuj: crash? inne zachowanie? brak efektu?
6. Dokumentuj wynik

### Zasady bezpieczeństwa:
- ZAWSZE pracuj na KOPII save (nigdy oryginał)
- Zmieniaj JEDNO pole na raz
- Testuj najpierw wartości "bezpieczne" (0, max) przed losowymi
- Jeśli crash → pole jest krytyczne, zanotuj
- Jeśli brak efektu → pole jest prawdopodobnie runtime-only lub unused

---

## Metoda 6: Pattern Recognition (MagicPattern Anchor)

**Cel**: Użyć znanego 192-bajtowego MagicPattern jako kotwicy do obliczenia offsetów.

### Procedura:
1. Znajdź MagicPattern w slocie (192 bytes znanych wartości)
2. Oblicz PlayerGameData offset = MagicPattern + 192 + N (gdzie N zależy od wersji)
3. Od tego punktu — weryfikuj pola sekwencyjnie

### MagicPattern (hex):
```
00 FF FF FF FF 00 00 00 00 00 00 00 00 00 00 00 00
FF FF FF FF 00 00 00 00 00 00 00 00 00 00 00 00 00
(powtarza się 12× = 192 bytes total)
```

---

## Metoda 7: Event Flag Spot-Check

**Cel**: Zweryfikować algorytm BST na konkretnych, znanych flagach.

### Procedura:
1. Weź save z pokonanym bossem (np. Margit — flag 71001)
2. Oblicz offset przez BST: block=71, index=1, lookup BST[71], byte=offset×125+0, bit=7-1=6
3. Sprawdź czy bit jest ustawiony w hex dumpie sekcji Event Flags
4. Powtórz dla kilku znanych flag

### Spot-check flags:
- 60100 (Torrent) — mechanika, łatwa do weryfikacji
- 71001 (Margit killed) — boss
- 76101 (The First Step grace) — grace
- 62010 (Limgrave West map) — mapa

---

---

# CHECKLISTA WERYFIKACJI

## Status: ✅ = zweryfikowane, ⏳ = w trakcie, ❌ = do zrobienia, ❓ = nieznane/do odkrycia

---

## A. PlayerGameData (432 bytes) — spec/04

### HP / FP / SP Block (0x00–0x33)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0x00 | unk0x0 | ❓ | Probe | PlayerNo? Internal ID? |
| 0x04 | unk0x4 | ❓ | Probe | — |
| 0x08 | HP | ❌ | Known Value | Porównaj z CT value |
| 0x0C | MaxHP | ❌ | Known Value | — |
| 0x10 | BaseMaxHP | ❌ | Known Value | Oblicz z Vigor tabeli |
| 0x14 | FP | ❌ | Known Value | — |
| 0x18 | MaxFP | ❌ | Known Value | — |
| 0x1C | BaseMaxFP | ❌ | Known Value | Oblicz z Mind tabeli |
| 0x20 | unk0x20 | ❓ | Probe | FP regen? MaxFP2? BaseMaxMP? |
| 0x24 | SP | ❌ | Known Value | — |
| 0x28 | MaxSP | ❌ | Known Value | — |
| 0x2C | BaseMaxSP | ❌ | Known Value | Oblicz z Endurance tabeli |
| 0x30 | unk0x30 | ❓ | Probe | SP regen? |

### Attributes (0x34–0x5F)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0x34 | Vigor | ❌ | Known Value | Znana wartość z kreatora/level up |
| 0x38 | Mind | ❌ | Known Value | — |
| 0x3C | Endurance | ❌ | Known Value | — |
| 0x40 | Strength | ❌ | Known Value | — |
| 0x44 | Dexterity | ❌ | Known Value | — |
| 0x48 | Intelligence | ❌ | Known Value | — |
| 0x4C | Faith | ❌ | Known Value | — |
| 0x50 | Arcane | ❌ | Known Value | — |
| 0x54 | unk0x54 | ❓ | Cross-Slot | Padding? DLC attr? |
| 0x58 | unk0x58 | ❓ | Cross-Slot | — |
| 0x5C | unk0x5c | ❓ | Cross-Slot | — |

### Level & Runes (0x60–0x6F)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0x60 | Level | ❌ | Known Value | Formuła: sum(attrs) - 79 |
| 0x64 | Runes | ❌ | Known Value | Aktualnie posiadane |
| 0x68 | TotalGetSoul | ❌ | Known Value | Lifetime — porównaj high/low level chars |
| 0x6C | unk0x6c | ❓ | Diff | Runes lost? Memory runes? |

### Status Buildups (0x70–0x93)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0x70 | Immunity (Poison) | ❌ | Probe | Powinno być 0 w czystym save |
| 0x74 | Immunity2 (Scarlet Rot) | ❌ | Probe | — |
| 0x78 | Robustness (Bleed) | ❌ | Probe | — |
| 0x7C | Vitality (Death) | ❌ | Probe | — |
| 0x80 | Robustness2 (Frost) | ❌ | Probe | — |
| 0x84 | Focus (Sleep) | ❌ | Probe | — |
| 0x88 | Focus2 (Madness) | ❌ | Probe | — |
| 0x8C | unk0x8c | ❓ | Probe | DLC buildup? |
| 0x90 | unk0x90 | ❓ | Probe | — |

### Character Name (0x94–0xB5)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0x94 | CharacterName[16] | ❌ | Known Value | UTF-16LE — łatwy anchor |
| 0xB4 | NullTerminator | ❌ | Pattern | Powinno być 0x0000 |

### Creation Data (0xB6–0xBF)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0xB6 | Gender | ❌ | Known Value | 0=TypeB, 1=TypeA |
| 0xB7 | ArcheType (Class) | ❌ | Known Value | 0-9 |
| 0xB8 | unk0xb8 | ❓ | Cross-Slot | Appearance? VowType? |
| 0xB9 | unk0xb9 | ❓ | Cross-Slot | — |
| 0xBA | VoiceType | ❌ | Probe | 0-5 |
| 0xBB | Gift | ❌ | Known Value | 0-9 (starting keepsake) |
| 0xBC | unk0xbc | ❓ | Probe | — |
| 0xBD | unk0xbd | ❓ | Probe | — |
| 0xBE | TalismanSlotCount | ❌ | Probe | 0-3 |
| 0xBF | SummonSpiritLevel | ❓ | Probe | Co to dokładnie robi? |

### Unknown Block (0xC0–0xD7)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0xC0–0xD7 | 24 bytes unknown | ❓ | Diff + Probe | Porównaj fresh char vs endgame char |

### Online Settings (0xD8–0xF8)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0xD8 | FurlcallingFinger | ❌ | Probe | 0/1 |
| 0xD9 | unk0xd9 | ❓ | Probe | — |
| 0xDA | MatchmakingWepLvl | ❌ | Known Value | 0-25 |
| 0xDB | WhiteCipherRing | ❌ | Probe | 0/1 |
| 0xDC | BlueCipherRing | ❌ | Probe | 0/1 |
| 0xDD–0xEE | 18 bytes unknown | ❓ | Diff | Online flags? |
| 0xEF | ReinforceLv | ❓ | Probe | Character reinforce? |
| 0xF0–0xF6 | 7 bytes unknown | ❓ | Diff | — |
| 0xF7 | GreatRuneActive | ❌ | Probe | 0/1 |
| 0xF8 | unk0xf8 | ❓ | Probe | — |

### Flask Counts (0xF9–0x10F)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0xF9 | MaxCrimsonFlask | ❌ | Known Value | 0-14 |
| 0xFA | MaxCeruleanFlask | ❌ | Known Value | 0-14 |
| 0xFB | unk | ❓ | Diff | Flask upgrade level? |
| 0xFC–0x10F | 20 bytes unknown | ❓ | Diff | Physick tears? Flask state? |

### Passwords (0x110–0x17B)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0x110 | MultiplayerPassword | ❌ | Known Value | UTF-16LE |
| 0x122 | GroupPassword1 | ❌ | Known Value | — |
| 0x134–0x16A | GroupPasswords 2-5 | ❌ | Known Value | — |

### Trailing Block (0x17C–0x1AF)

| Offset | Pole | Status | Metoda | Notatki |
|---|---|---|---|---|
| 0x17C–0x17F | SwordArtPoint? | ❓ | CT Cross-ref | 4 × u8 scaling? |
| 0x180–0x1AF | 48 bytes | ❓ | Diff + Probe | Correction stats? Extended data? |

---

## B. Equipment Structures — spec/06

| Struktura | Status | Metoda | Notatki |
|---|---|---|---|
| EquippedItemsEquipIndex (88B) | ❌ | Known Value | Porównaj z inventory indices |
| ActiveWeaponSlots (28B) | ❌ | Probe | ArmStyle 0-3, slots 0-2 |
| EquippedItemsItemIds (88B) | ❌ | Known Value | Item IDs z bazy |
| EquippedItemsGaitemHandles (88B) | ❌ | Known Value | Handles z GaItem Map |
| Slot #10 (Arrows 3) | ❓ | Probe | CT mówi że istnieje |
| Slot #11 (Bolts 3) | ❓ | Probe | CT mówi że istnieje |
| Slot #16 (Hair) | ❓ | Probe | Wewnętrzny slot |
| Slot #21 (Accessory 5) | ❓ | Probe | Unused — czy zerowy? |
| Great Rune field | ❓ | Known Value | Gdzie dokładnie w save? |
| Quick Items (10 × u32) | ❌ | Known Value | — |
| Pouch (6 × u32) | ❌ | Known Value | — |

---

## C. Spells & Gestures — spec/08

| Element | Status | Metoda | Notatki |
|---|---|---|---|
| 14 spell slots (stride 8B) | ❌ | Known Value | Znane zaklęcia postaci |
| SelectedSlotIdx | ❓ | Probe | -1 or 0-13 |
| Quick Slots 10 × u32 | ❌ | Known Value | — |
| Pouch 6 × u32 | ❌ | Known Value | — |
| Equipped Gestures (ring) | ❌ | Known Value | ID z tabeli |
| Acquired Projectiles count | ❌ | Known Value | Porównaj postacie |
| Gestures 64 × u32 | ❌ | Cross-Slot | — |
| Equipped Physics (2 × u32) | ❌ | Known Value | Crystal Tear IDs |

---

## D. Face Data (303 bytes) — spec/09

| Element | Status | Metoda | Notatki |
|---|---|---|---|
| Face_Model_Id (u32) | ❌ | Cross-Slot | Porównaj chars z różnymi twarzami |
| Hair_Model_Id (u32) | ❌ | Known Value | Znana fryzura |
| Beard_Model_Id (u32) | ❌ | Cross-Slot | — |
| Face shape params (~50 u8) | ❓ | Diff | Porównaj identyczne vs różne twarze |
| Skin colors (RGBA) | ❓ | Diff | — |
| Body Scale (7 values) | ❓ | Probe | float vs u8? |
| Trailing 15 bytes (slot-only) | ❓ | Probe | Co to? |

---

## E. Inventory & Storage — spec/07, spec/10

| Element | Status | Metoda | Notatki |
|---|---|---|---|
| Common item count (2688 max) | ❌ | Known Value | — |
| Key item count (384 max) | ❌ | Known Value | — |
| Item record (12B: handle+qty+idx) | ❌ | Known Value | — |
| NextEquipIndex | ❌ | Monotonic check | Powinien rosnąć |
| NextAcquisitionSortId | ❌ | Monotonic check | — |
| Storage common (1920 max) | ❌ | Known Value | — |
| Storage key (128 max) | ❌ | Known Value | — |
| Empty slot pattern | ❌ | Pattern | 0x00000000 or 0xFFFFFFFF |

---

## F. GaItem Map — spec/03

| Element | Status | Metoda | Notatki |
|---|---|---|---|
| Entry count (5118 vs 5120) | ❌ | Version check | Zależy od slot.Version |
| Weapon record (21B) | ❌ | Known Value | Handle + ItemID + extras |
| Armor record (16B) | ❌ | Known Value | — |
| Other record (8B) | ❌ | Known Value | — |
| Type segregation (AoW first) | ❌ | Scan | Sprawdź czy 0xC0... przed 0x80... |
| Empty entry pattern | ❌ | Pattern | 0x00 or 0xFFFFFFFF |

---

## G. Event Flags — spec/15

| Element | Status | Metoda | Notatki |
|---|---|---|---|
| BST algorithm correctness | ❌ | Spot-check | 4-5 znanych flag |
| Size = 0x1BF99F | ❌ | Measure | — |
| Terminator (4B after) | ❌ | Pattern | — |
| Grace flag 76101 (First Step) | ❌ | BST + hex | — |
| Boss flag (Margit killed?) | ❌ | BST + hex | — |
| Mechanic flag 60100 (Torrent) | ❌ | BST + hex | — |
| Map flag 62010 (Limgrave W) | ❌ | BST + hex | — |

---

## H. Game State — spec/14

| Element | Status | Metoda | Notatki |
|---|---|---|---|
| ClearCount (NG+) | ❌ | Known Value | Sprawdź postacie pre/post NG+ |
| Death Count | ❌ | Known Value | Znana wartość z menu? |
| Last Rested Grace | ❌ | Known Value | BonfireId — zweryfikuj z grace lista |
| Play Time | ❌ | Known Value | Porównaj z menu display |
| GaItem Game Data count | ❌ | Known Value | — |
| Tutorial Data count | ❌ | Cross-Slot | Fresh vs endgame |

---

## I. World Data — spec/12, 13, 16, 17, 18, 19

| Element | Status | Metoda | Notatki |
|---|---|---|---|
| Torrent State | ❌ | Probe | 1/3/13 |
| Torrent HP | ❌ | Known Value | — |
| Blood Stain runes | ❌ | Known Value | Ile run stracono |
| Player Coords (x,y,z) | ❌ | Known Value | Porównaj z CT readout |
| Map ID | ❌ | Known Value | — |
| Weather type | ❓ | Diff | Porównaj różne pory dnia |
| Time (H/M/S) | ❌ | Known Value | — |
| Regions count | ❌ | Known Value | Ile regionów odkryto |
| FieldArea size | ❌ | Measure | — |
| WorldArea size | ❌ | Measure | — |
| NetMan size (0x20004) | ❌ | Measure | — |

---

## J. Platform & Meta — spec/01, 20, 21, 22, 23

| Element | Status | Metoda | Notatki |
|---|---|---|---|
| BND4 header (PC) | ❌ | Pattern | Magic bytes "BND4" |
| MD5 checksum (PC) | ❌ | Recalculate | Compute i porównaj |
| PS4 header | ❌ | Pattern | Porównaj z known magic |
| Active Slots offset PC | ❌ | **KRYTYCZNE** | Spec: 0x1C vs kod: 0x310 |
| Active Slots offset PS4 | ❌ | Known Value | 0x300 |
| ProfileSummary offset PC | ❌ | **KRYTYCZNE** | Spec: 0x26 vs kod: 0x31A |
| ProfileSummary offset PS4 | ❌ | Known Value | 0x30A |
| SteamID w UserData10 | ❌ | Known Value | 8 bytes, known Steam ID |
| DLC flags (50B) | ❌ | Pattern | Bytes 3-49 = zero? |
| DLC entry flag | ❌ | Probe | 0/1 |
| BaseVersion | ❌ | Known Value | — |
| PlayerGameData Hash | ❌ | Measure | Reszta do końca slotu |

---

## K. Dynamic Offset Chain — KRYTYCZNE

Weryfikacja łańcucha offsetów obliczanych sekwencyjnie:

| Krok | Od → Do | Status | Metoda |
|---|---|---|---|
| 1 | Slot start → GaItem Map | ❌ | Version + header (32B) |
| 2 | GaItem Map → PlayerGameData | ❌ | Scan all entries, sum sizes |
| 3 | PlayerGameData → SP Effects | ❌ | +432B |
| 4 | SP Effects → EquipIndex | ❌ | Variable (13 entries?) |
| 5 | EquipIndex → ActiveWeapons | ❌ | +88B |
| 6 | ActiveWeapons → ItemIds | ❌ | +28B |
| 7 | ItemIds → GaitemHandles | ❌ | +88B |
| 8 | GaitemHandles → Inventory | ❌ | +88B |
| 9 | Inventory → Spells | ❌ | Count-based (common+key × 12B + 8B counters) |
| 10 | Spells → Projectiles | ❌ | Fixed? |
| 11 | Projectiles → Face Data | ❌ | VARIABLE (count × 8B) |
| 12 | Face Data → Storage | ❌ | +303B |
| 13 | Storage → Gestures | ❌ | Count-based |
| 14 | Gestures → Regions | ❌ | +256B |
| 15 | Regions → Torrent | ❌ | VARIABLE (4 + count×4) |
| 16 | Torrent → Blood Stain | ❌ | +40B + 1B (control byte) |
| 17 | Blood Stain → Game State | ❌ | +68B |
| 18 | Game State → Event Flags | ❌ | VARIABLE (multiple sub-sections) |
| 19 | Event Flags → World State | ❌ | +0x1BF99F + 4B |
| 20 | World State → Coords | ❌ | VARIABLE (5 size-prefixed sections) |
| 21 | Coords → NetMan | ❌ | +57B + spawn bytes |
| 22 | NetMan → Weather | ❌ | +0x20004 |
| 23 | Weather → Time | ❌ | +12B |
| 24 | Time → Version | ❌ | +12B |
| 25 | Version → SteamID | ❌ | +16B |
| 26 | SteamID → PS5Activity | ❌ | +8B |
| 27 | PS5Activity → DLC | ❌ | +32B |
| 28 | DLC → Hash | ❌ | +50B |
| 29 | Hash → Slot End | ❌ | Reszta do 0x280000 |

---

## L. ODKRYCIA — Nieznane parametry do zbadania

### Priorytet WYSOKI (prawdopodobnie edytowalne)

| ID | Lokalizacja | Hipoteza | Plan badania |
|---|---|---|---|
| L1 | PGD 0x00–0x07 | Runtime-only header? | Sprawdź czy różne między slotami |
| L2 | PGD 0x20 | FP-related (between MaxFP and SP) | Porównaj z Mind value; probe |
| L3 | PGD 0x30 | SP-related | Porównaj z Endurance value |
| L4 | PGD 0x54–0x5C | Extended attrs? DLC? | Sprawdź pre-DLC vs post-DLC save |
| L5 | PGD 0x6C | Runes on bloodstain? | Kill char, compare before/after |
| L6 | PGD 0x8C–0x90 | DLC buildups? | DLC save vs base save |
| L7 | PGD 0xFB–0x10F | Flask upgrade level + Physick | Porównaj fresh vs 12 Sacred Tears |
| L8 | PGD 0xC0–0xD7 | Character state flags? | Diff between online/offline save |

### Priorytet ŚREDNI (prawdopodobnie informacyjne)

| ID | Lokalizacja | Hipoteza | Plan badania |
|---|---|---|---|
| L9 | PGD 0xB8–0xB9 | Appearance/VowType? | Cross-ref z CT |
| L10 | PGD 0xBC–0xBD | Starting equip related? | Porównaj klasy |
| L11 | PGD 0xBF | SummonSpiritLevel | Co to robi? DLC? |
| L12 | PGD 0xDD–0xF6 | Extended online settings | Diff online vs offline |
| L13 | PGD 0xEF | ReinforceLv | Character reinforce — co to? |
| L14 | PGD 0x180–0x1AF | Trailing 48B | SwordArt? Correction? Overflow? |

### Priorytet NISKI (prawdopodobnie stałe/unused)

| ID | Lokalizacja | Hipoteza | Plan badania |
|---|---|---|---|
| L15 | Equipment slot #21 | Accessory 5 — unused? | Sprawdź czy zawsze 0xFFFFFFFF |
| L16 | Face Data trailing 15B | Slot-only extra params? | Diff vs ProfileSummary version |
| L17 | Body Scale format | float vs u8 w save? | Hex dump known proportions |
| L18 | Correction Stats location | W PGD czy po PGD? | Szukaj kopii atrybutów |

---

## Procedura sesji weryfikacyjnej

### Przed sesją:
1. `cp tmp/save/ER0000.sl2 tmp/save/ER0000.sl2.bak` — backup
2. Przygotuj skrypt do wycinania slotów z save file
3. Przygotuj `xxd` / `hexdump` commands z prawidłowymi offsetami

### Podczas sesji:
1. Wybierz sekcję do weryfikacji (np. "A. PlayerGameData")
2. Wykonaj odpowiednią metodę (Known Value / Diff / Probe)
3. Zapisuj wyniki w kolumnie "Status" i "Notatki"
4. Przy odkryciu — dodaj do sekcji L z pełnym opisem

### Po sesji:
1. Zaktualizuj ten plik ze statusami
2. Zaktualizuj odpowiednie pliki spec/ z potwierdzonymi informacjami
3. Dodaj nowe odkrycia do spec/26-parameter-reference.md

---

## Narzędzia pomocnicze (do napisania)

| Narzędzie | Cel | Status |
|---|---|---|
| `scripts/dump_slot.py` | Wyciąga surowy slot z .sl2 | ❌ Do napisania |
| `scripts/find_pattern.py` | Szuka wzorca w hex dump | ❌ Do napisania |
| `scripts/verify_bst.py` | Testuje algorytm BST na known flags | ❌ Do napisania |
| `scripts/diff_slots.py` | Porównuje dwa sloty bajt po bajcie | ❌ Do napisania |
| `scripts/parse_pgd.py` | Parsuje PlayerGameData i wypisuje pola | ❌ Do napisania |
| `tests/offset_chain_test.go` | Weryfikuje cały łańcuch offsetów | ❌ Do napisania |

---

## Źródła

- Hex editor: `xxd`, `hexdump`, lub GUI (HxD, ImHex)
- Python: struct module do parsowania binary
- Go test framework: `go test -v -run TestXxx`
- er-save-manager: reference parser do porównań
- Cheat Engine tables: runtime values do cross-reference
