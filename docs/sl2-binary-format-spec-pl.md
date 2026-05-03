# Pełny Audyt Techniczny — EldenRing-SaveEditor

**Data**: 2026-04-11
**Zakres**: Analiza formatu binarnego `.sl2`, algorytm odczytu/zapisu/edycji, porównanie z 3 referencyjnymi edytorami, crash logi, porównanie binarne plików save.

---

## Spis treści

1. [Specyfikacja formatu pliku `.sl2`](#1-specyfikacja-formatu-pliku-sl2)
2. [Struktura nagłówka pliku](#2-struktura-nagłówka-pliku)
3. [Struktura slotu postaci (UserDataX)](#3-struktura-slotu-postaci-userdatax)
4. [Sekcja GaItems — rejestr przedmiotów](#4-sekcja-gaitems--rejestr-przedmiotów)
5. [Dynamiczny łańcuch offsetów](#5-dynamiczny-łańcuch-offsetów)
6. [System inwentarza](#6-system-inwentarza)
7. [UserData10 — dane globalne](#7-userdata10--dane-globalne)
8. [UserData11 — regulacja gry](#8-userdata11--regulacja-gry)
9. [Sumy kontrolne (MD5)](#9-sumy-kontrolne-md5)
10. [Szyfrowanie (AES)](#10-szyfrowanie-aes)
11. [Różnice PC vs PS4](#11-różnice-pc-vs-ps4)
12. [Hash integralności slotu (PlayerGameDataHash)](#12-hash-integralności-slotu-playergamedatahash)
13. [Algorytm odczytu pliku save](#13-algorytm-odczytu-pliku-save)
14. [Algorytm zapisu pliku save](#14-algorytm-zapisu-pliku-save)
15. [Algorytm edycji danych postaci](#15-algorytm-edycji-danych-postaci)
16. [Analiza naszego kodu vs referencje](#16-analiza-naszego-kodu-vs-referencje)
17. [Znalezione błędy w naszym kodzie](#17-znalezione-błędy-w-naszym-kodzie)
18. [Analiza crashy gry](#18-analiza-crashy-gry)
19. [Porównanie binarne plików save](#19-porównanie-binarne-plików-save)
20. [Rekomendacje napraw](#20-rekomendacje-napraw)

---

## 1. Specyfikacja formatu pliku `.sl2`

### Przegląd

Plik `.sl2` (Elden Ring save file) to kontener binarny zawierający do 10 slotów postaci + dane globalne + dane regulacji gry. Format jest **little-endian** w całości (z wyjątkiem nagłówka DCX w regulacji, który jest big-endian).

### Rozmiary

| Komponent | Rozmiar |
|---|---|
| Slot postaci (dane) | `0x280000` (2,621,440 B) |
| MD5 checksum (PC only) | `0x10` (16 B) |
| Slot PC łącznie | `0x280010` (2,621,456 B) |
| Slot PS4 łącznie | `0x280000` (2,621,440 B) |
| Nagłówek PC (BND4) | `0x300` (768 B) |
| Nagłówek PS4 | `0x70` (112 B) |
| UserData10 (dane) | `0x60000` (393,216 B) |
| UserData11 (dane) | `0x240000` (2,359,296 B) |
| Liczba slotów | 10 |

### Layout pliku PC

```
Offset          Rozmiar      Sekcja
─────────────────────────────────────────────────────
0x000           0x300        Nagłówek BND4
0x300           0x10         MD5 checksum slot 0
0x310           0x280000     Dane slotu 0
0x280310        0x10         MD5 checksum slot 1
0x280320        0x280000     Dane slotu 1
  ...           ...          ... (powtórz dla slotów 2-9)
0x19003A0       0x10         MD5 checksum UserData10
0x19003B0       0x60000      Dane UserData10
0x1F003B0       0x10         MD5 checksum UserData11
0x1F003C0       0x240000     Dane UserData11
─────────────────────────────────────────────────────
```

**Wzór na offset slotu N (PC)**:
- Checksum: `0x300 + N * 0x280010`
- Dane: `0x310 + N * 0x280010`

### Layout pliku PS4

```
Offset          Rozmiar      Sekcja
─────────────────────────────────────────────────────
0x000           0x70         Nagłówek PS4
0x70            0x280000     Dane slotu 0
0x280070        0x280000     Dane slotu 1
  ...           ...          ... (powtórz dla slotów 2-9)
0x1900070       0x60000      Dane UserData10 (bez MD5)
0x1960070       0x240000     Dane UserData11 (bez MD5)
─────────────────────────────────────────────────────
```

---

## 2. Struktura nagłówka pliku

### PC — nagłówek BND4 (0x300 bajtów)

Magic bytes na offset 0: `42 4E 44 34` = ASCII `"BND4"`

Nagłówek BND4 to standardowy kontener FromSoftware zawierający tabelę wpisów (12 wpisów):
- 10 × UserData000..009 (sloty postaci)
- 1 × UserData010 (dane globalne)
- 1 × UserData011 (regulacja)

Każdy wpis zawiera: offset danych, rozmiar danych, ID, hash nazwy. Nazwy plików w UTF-16LE.

### PS4 — nagłówek prosty (0x70 bajtów)

Magic bytes: `CB 01 9C 2C`

Stały 112-bajtowy nagłówek zawierający pary `{id, 0x7F7F7F7F}` dla wpisów 7-18.

---

## 3. Struktura slotu postaci (UserDataX)

Każdy slot to dokładnie `0x280000` bajtów. Struktura jest sekwencyjna — pola są czytane po kolei, a pozycja każdego pola zależy od rozmiaru pól poprzednich (w tym pól o zmiennej długości).

### Pola slotu (sekwencyjnie)

| Offset | Rozmiar | Pole | Opis |
|---|---|---|---|
| 0x00 | 4 | `version` | Wersja formatu (0 = pusty slot) |
| 0x04 | 4 | `map_id` | Aktualna lokalizacja na mapie |
| 0x08 | 24 | padding | Nieznane |
| **0x20** | **zmienny** | **`gaitem_map`** | **Tablica GaItem (5118 lub 5120 wpisów)** |
| zmienny | 0x1B0 (432) | `player_game_data` | Statystyki, nazwa, poziom |
| +0xD0 | 0xD0 (208) | `sp_effects` | 13 aktywnych efektów |
| +0x58 | 0x58 (88) | `equipped_items_equip_index` | Indeksy ekwipunku |
| +0x1C | 0x1C (28) | `active_weapon_slots` | Aktywne sloty broni |
| +0x58 | 0x58 (88) | `equipped_items_item_id` | ID przedmiotów |
| +0x58 | 0x58 (88) | `equipped_items_gaitem_handle` | Handle'e GaItem |
| zmienny | zmienny | `inventory_held` | Inwentarz (common: 2688, key: 384 slotów) |
| +0x74 | 0x74 (116) | `equipped_spells` | 14 slotów zaklęć + aktywny |
| +0x8C | 0x8C (140) | `equipped_items` | 10 quick + 6 pouch |
| +0x18 | 0x18 (24) | `equipped_gestures` | 6 gestów |
| zmienny | zmienny | `acquired_projectiles` | count + count×8 bajtów |
| +0x9C | 0x9C (156) | `equipped_armaments` | Pełny stan ekwipunku |
| +0x0C | 0x0C (12) | `equipped_physics` | Łzy Physick |
| +0x12F | 0x12F (303) | `face_data` | Wygląd postaci |
| zmienny | zmienny | `inventory_storage_box` | Skrzynia (common: 1920, key: 128) |
| +0x100 | 0x100 (256) | `gesture_game_data` | 64 gesty |
| zmienny | zmienny | `unlocked_regions` | count + count×4 |
| +0x28 | 0x28 (40) | `ride_game_data` | Torrent (pozycja, HP) |
| +1 | 1 | `control_byte` | |
| +0x44 | 0x44 (68) | `blood_stain` | Lokalizacja śmierci + runy |
| +8 | 8 | padding | |
| zmienny | zmienny | `menu_profile_save_load` | H+H+I header + dane |
| +0x34 | 0x34 (52) | `trophy_equip_data` | |
| zmienny | zmienny | `gaitem_game_data` | i64 count + 7000 × 16B |
| zmienny | zmienny | `tutorial_data` | H+H+I header + dane |
| +3 | 3 | `gameman_flags` | |
| +4 | 4 | `total_deaths` | Licznik śmierci |
| +4 | 4 | `character_type` | |
| +1 | 1 | `in_online_session` | |
| +4 | 4 | `character_type_online` | |
| +4 | 4 | `last_rested_grace` | |
| +1 | 1 | `not_alone_flag` | |
| +4 | 4 | `in_game_timer` | |
| +4 | 4 | padding | |
| **+0x1BF99F** | **0x1BF99F** | **`event_flags`** | **1,833,375 bajtów flag bitowych** |
| +1 | 1 | terminator | |
| zmienny | zmienny | 5× `UnknownList` | Prefiksy rozmiaru + dane |
| +0x39 | 0x39 (57) | `player_coordinates` | Pozycja + kąt |
| +2 | 2 | padding | |
| +4 | 4 | `spawn_point_entity_id` | |
| +4 | 4 | padding | |
| +4 | 4 | `temp_spawn_point` | (version >= 65) |
| +1 | 1 | padding | (version >= 66) |
| +0x20004 | 0x20004 | `net_man` | Dane sieciowe |
| +0x0C | 0x0C | `world_area_weather` | |
| +0x0C | 0x0C | `world_area_time` | |
| +0x10 | 0x10 | `base_version` | |
| +8 | 8 | `steam_id` | SteamID per-slot |
| +0x20 | 0x20 | `ps5_activity` | |
| +0x32 | 0x32 (50) | `dlc` | DLC flagi |
| +0x80 | 0x80 (128) | `player_data_hash` | Hash integralności |
| zmienny | zmienny | padding | Wypełnienie do 0x280000 |

### PlayerGameData (432 bajty / 0x1B0)

Pola statystyk relatywne do **MagicOffset** (= `PlayerDataOffset`):

| Offset od MagicOffset | Typ | Pole |
|---|---|---|
| -379 | u32 | Vigor |
| -375 | u32 | Mind |
| -371 | u32 | Endurance |
| -367 | u32 | Strength |
| -363 | u32 | Dexterity |
| -359 | u32 | Intelligence |
| -355 | u32 | Faith |
| -351 | u32 | Arcane |
| -347 | u32 | Humanity (wewnętrzne) |
| -335 | u32 | Level |
| -331 | u32 | Souls (Runes) |
| -327 | u32 | SoulMemory |
| -283 (0x11B) | 16×u16 | CharacterName (UTF-16LE) |
| -249 | u8 | Gender |
| -248 | u8 | Class (starting class) |
| -187 | u8 | ScadutreeBlessing (DLC) |
| -186 | u8 | ShadowRealmBlessing (DLC) |

**Wzór na level**: `level = vigor + mind + endurance + strength + dexterity + intelligence + faith + arcane - 79`

### Wersjonowanie slotu

| Wersja | Zmiana |
|---|---|
| ≤ 81 | GaItem count = 5118 |
| > 81 | GaItem count = 5120 |
| ≥ 65 | Dodano `temp_spawn_point_entity_id` (4B) |
| ≥ 66 | Dodano `game_man_0xcb3` (1B) |

---

## 4. Sekcja GaItems — rejestr przedmiotów

### Lokalizacja

Sekcja GaItems zaczyna się na offset **0x20** w danych slotu i kończy tuż przed `PlayerGameData` (432 bajty = 0x1B0 przed MagicOffset).

### Format rekordów (zmienna długość!)

To **najkrytyczniejsza** struktura w formacie save. Nieprawidłowe parsowanie desynchronizuje wszystkie kolejne dane.

```
Bazowy rekord: 8 bajtów
┌──────────────────┬──────────────────┐
│ gaitem_handle (4B)│ item_id (4B)     │
└──────────────────┴──────────────────┘

Jeśli handle != 0 i typ != AoW (0xC0):
  +8 bajtów: unk2 (i32) + unk3 (i32) = 16 bajtów łącznie

  Jeśli typ == Weapon (0x80):
    +5 bajtów: aow_handle (i32) + unk5 (u8) = 21 bajtów łącznie
```

### Rozmiary rekordów per typ

| Typ | Maska handle | Rozmiar rekordu | Pola dodatkowe |
|---|---|---|---|
| **Weapon** | `0x80000000` | **21 B** | unk2, unk3, aow_handle, unk5 |
| **Armor** | `0x90000000` | **16 B** | unk2, unk3 |
| **Accessory** | `0xA0000000` | **8 B** | brak |
| **Item/Goods** | `0xB0000000` | **8 B** | brak |
| **Ash of War** | `0xC0000000` | **8 B** | brak |
| **Empty** | `0x00000000` | **8 B** | (id = 0xFFFFFFFF) |

### Detekcja typu rekordu

Typ rekordu jest określany na podstawie **górnego nibble'a** (4 bity) pola `gaitem_handle`:

```
handle & 0xF0000000:
  0x80000000 → Weapon (21B)
  0x90000000 → Armor (16B)
  0xA0000000 → Accessory (8B)
  0xB0000000 → Item/Goods (8B)
  0xC0000000 → Ash of War (8B)
  0x00000000 → Empty (8B, id = 0xFFFFFFFF)
```

### Referencyjny format rekordu broni (21B)

```
Offset  Rozmiar  Pole
0x00    4        gaitem_handle     (np. 0x80010001)
0x04    4        item_id           (np. 0x003D0900 = Moonveil+0)
0x08    4        unk2              (zazwyczaj -1 = 0xFFFFFFFF)
0x0C    4        unk3              (zazwyczaj -1 = 0xFFFFFFFF)
0x10    4        aow_gaitem_handle (handle AoW lub 0xFFFFFFFF)
0x14    1        unk5              (zazwyczaj 0x00)
```

### Prefix systemu — Handle vs ItemID

**WAŻNE**: Istnieją **dwa niezależne systemy prefiksów**:

| Typ | Handle prefix | ItemID prefix |
|---|---|---|
| Weapon | `0x80xxxxxx` | `0x00xxxxxx` |
| Armor | `0x90xxxxxx` | `0x10xxxxxx` |
| Accessory | `0xA0xxxxxx` | `0x20xxxxxx` |
| Item/Goods | `0xB0xxxxxx` | `0x40xxxxxx` |
| Ash of War | `0xC0xxxxxx` | `0x60xxxxxx` |

### Liczba wpisów

- **Wersja slotu ≤ 81**: 5118 wpisów (0x13FE)
- **Wersja slotu > 81**: 5120 wpisów (0x1400)

Referencyjne edytory (Rust, Python) czytają **stałą liczbę wpisów** zamiast skanować do końca sekcji. Nasz edytor skanuje do MagicOffset, co jest mniej bezpieczne.

---

## 5. Dynamiczny łańcuch offsetów

### Punkt kotwiczenia — MagicPattern

64-bajtowy wzorzec służący jako kotwica w każdym slocie:

```hex
00 FFFFFFFF 000000000000000000000000
   FFFFFFFF 000000000000000000000000
   FFFFFFFF 000000000000000000000000
   FFFFFFFF 000000000000000000000000
```

Wzorzec: 4× powtórzenie `[0x00, 0xFF,0xFF,0xFF,0xFF, 12×0x00]` (16 bajtów × 4 = 64 bajty).

**MagicOffset** = pozycja tego wzorca w danych slotu. Jest to jednocześnie `PlayerDataOffset`.

### Łańcuch offsetów (od MagicOffset)

Poniższy łańcuch definiuje pozycje wszystkich sekcji danych. Każdy offset jest **addytywny** od poprzedniego:

```
PlayerDataOffset    = MagicOffset
                      │
                      ├── + 0xD0 ──→ SpEffect
                      ├── + 0x58 ──→ EquipedItemIndex
                      ├── + 0x1C ──→ ActiveEquipedItems
                      ├── + 0x58 ──→ EquipedItemsID
                      ├── + 0x58 ──→ ActiveEquipedItemsGa
                      ├── + 0x9010 ─→ InventoryHeld
                      ├── + 0x74 ──→ EquipedSpells
                      ├── + 0x8C ──→ EquipedItems
                      ├── + 0x18 ──→ EquipedGestures
                      │
                      ├── + DYNAMIC: projSize × 8 + 4  ← odczyt z danych save!
                      │
                      ├── + 0x9C ──→ EquipedArmaments
                      ├── + 0x0C ──→ EquipPhysics
                      ├── + 0x12F ─→ FaceData
                      ├── + 0x6010 ─→ StorageBox
                      ├── + 0x100 ─→ GestureGameData
                      │
                      ├── + DYNAMIC: unlockedRegSz × 4 + 4  ← odczyt z danych save!
                      │
                      ├── + 0x29 ──→ Horse (Torrent)
                      ├── + 0x4C ──→ BloodStain
                      ├── + 0x103C ─→ MenuProfile
                      ├── + 0x1B588 → GaItemDataOther
                      ├── + 0x40B ─→ TutorialData
                      ├── + 0x1A ──→ IngameTimer
                      └── + 0x1C0000 → EventFlags
```

### Dwa pola dynamiczne

1. **`projSize`** — odczyt `uint32` z aktualnej pozycji. Rzeczywisty rozmiar = `projSize × 8 + 4`. Max: 256.
2. **`unlockedRegSz`** — odczyt `uint32` z aktualnej pozycji. Rzeczywisty rozmiar = `unlockedRegSz × 4 + 4`. Max: 1024.

**KRYTYCZNE**: Każdy błąd w obliczeniu pozycji w łańcuchu kaskadowo przesuwa WSZYSTKIE kolejne offsety, prowadząc do odczytu/zapisu śmieci.

---

## 6. System inwentarza

### Rekord inwentarza (12 bajtów)

```
┌──────────────────┬──────────────────┬──────────────────┐
│ gaitem_handle (4B)│ quantity (4B)    │ index (4B)       │
└──────────────────┴──────────────────┴──────────────────┘
```

### Pojemności

| Lista | Common | Key | Razem slotów |
|---|---|---|---|
| **Inventory (held)** | 0xA80 (2688) | 0x180 (384) | 3072 |
| **Storage Box** | 0x780 (1920) | 0x80 (128) | 2048 |

### Layout inwentarza

```
common_item_count:  u32
common_items:       InventoryItem[capacity]  ← WSZYSTKIE sloty zawsze zapisane
key_item_count:     u32
key_items:          InventoryItem[capacity]
equip_index_ctr:    u32                      ← NextEquipIndex
acquisition_ctr:    u32                      ← NextAcquisitionSortId
```

### Indeksy rezerwowane

Indeksy `0–432` (`InvEquipReservedMax`) w polu `CSGaItemIns` są zarezerwowane dla ekwipunku postaci. Nowe przedmioty muszą mieć indeks > 432, aby uniknąć kolizji z systemem ekwipunku gry.

---

## 7. UserData10 — dane globalne

### Layout (po checksumie MD5 dla PC)

```
Offset w UD10    Rozmiar     Pole                     Platforma
──────────────────────────────────────────────────────────────────
0x00             4           version                  Obie
0x04             8           steam_id                 PC only
0x0C             0x140       Settings                 Obie
0x14C            0x1808      MenuSystemSaveLoad       Obie (15 presetów)
                 ---         (różne offsety dla active_slots) ---
PC: 0x310        10          active_slots[10]         PC
PS4: 0x300       10          active_slots[10]         PS4
PC: 0x31A        10×0x24C    profile_summaries[10]    PC
PS4: 0x30A       10×0x24C    profile_summaries[10]    PS4
                 5           gamedataman fields       Obie
PC only          0xB2        PCOptionData             PC only
                 zmienny     KeyConfigSaveLoad        Obie
                 8           game_man field           Obie
                 zmienny     padding do 0x60000       Obie
```

### ProfileSummary (0x24C = 588 bajtów)

```
character_name:  16×u16 (32B, UTF-16LE) + 2B terminator
level:           u32
seconds_played:  u32
runes_memory:    u32
map_id:          4B
unk:             u32
face_data:       0x124B (292B, wersja skrócona)
equipment:       0xE8B (232B)
body_type:       u8
archetype:       u8
starting_gift:   u8
padding:         7B
```

### DLC w slocie (50 bajtów)

| Bajt | Znaczenie |
|---|---|
| [0] | Pre-order gesture "The Ring" |
| [1] | Shadow of the Erdtree (non-zero = wejście do DLC, powoduje nieskończone ładowanie bez DLC) |
| [2] | Pre-order gesture "Ring of Miquella" |
| [3-49] | Nieużywane (MUSZĄ być 0x00, non-zero = korupcja) |

---

## 8. UserData11 — regulacja gry

### Layout

```
[16B unk header]
[0x1C5F70 regulation data]  ← zaszyfrowane AES-256-CBC + skompresowane DCX/zlib
[0x7A090 rest data]
```

Łączny rozmiar: `0x240000` bajtów.

### Dekrypcja regulacji (AES-256-CBC)

**Uwaga**: To jest szyfrowanie **wewnątrz regulacji**, NIE szyfrowanie pliku `.sl2`.

- **Klucz (32 bajty)**: `99 BF FC 36 6A 6B C8 C6 F5 82 7D 09 36 02 D6 76 C4 28 92 A0 1C 20 7F B0 24 D3 AF 4E 49 3F EF 99`
- **IV**: pierwsze 16 bajtów ciphertext
- **Tryb**: CBC, bez paddingu
- Po odszyfrowaniu: kontener DCX (FromSoftware compression) → zlib deflate → archiwum BND4 z plikami `.param`

---

## 9. Sumy kontrolne (MD5)

### Dotyczy TYLKO platformy PC

PS4 save **nie ma żadnych sum kontrolnych**.

### Algorytm

Standard MD5 (`crypto/md5` w Go, `hashlib.md5` w Python).

### Co jest checksumowane

| Sekcja | Dane wejściowe MD5 | Rozmiar danych | Lokalizacja MD5 |
|---|---|---|---|
| Slot N | Dane slotu (0x280000 B) | 2,621,440 B | 16B tuż przed danymi slotu |
| UserData10 | Dane UD10 (0x60000 B) | 393,216 B | 16B tuż przed danymi UD10 |
| UserData11 | Dane UD11 | zmienny | 16B tuż przed danymi UD11 |

### Detekcja pustego slotu

Jeśli wszystkie 16 bajtów checksum = 0x00 → slot jest pusty.

### Rekalkulacja

Po **każdej modyfikacji danych slotu** trzeba przeliczyć MD5 i zapisać nową sumę. Wszystkie 3 referencyjne edytory to robią.

---

## 10. Szyfrowanie (AES)

### Kluczowe odkrycie

**Żaden z 3 referencyjnych edytorów NIE szyfruje/deszyfruje pliku `.sl2`.**

Plik `.sl2` na PC jest normalnie zapisany jako plaintext BND4. Szyfrowanie AES-128-CBC, które implementuje nasz edytor, dotyczy **warstwy Steam** — Steam na niektórych platformach (Windows desktop) szyfruje plik przed zapisem na dysk. Na **Steam Deck** plik NIE jest szyfrowany.

### Nasz kod AES-128-CBC (crypto.go)

- **Klucz (16 bajtów)**: `99 AD 2D 50 ED F2 FB 01 C5 F3 EC 3A 2B CA B6 9D`
- **IV**: pierwsze 16 bajtów pliku
- **Tryb**: CBC
- **Detekcja**: Jeśli plik zaczyna się od `"BND4"` → nieszyfrowany. Jeśli po odszyfrowaniu zaczyna się od `"BND4"` → szyfrowany.

### Kiedy NIE szyfrować

- PS4: nigdy
- Steam Deck: save jest plaintext `.sl2`
- Konwersja PS4→PC: nasz edytor generuje losowy IV i szyfruje — to poprawne dla Windows Steam, ale niepotrzebne dla Steam Deck

---

## 11. Różnice PC vs PS4

| Aspekt | PC | PS4 |
|---|---|---|
| Nagłówek | BND4, 0x300 B | Prosty, 0x70 B |
| Magic bytes | `42 4E 44 34` ("BND4") | `CB 01 9C 2C` |
| Szyfrowanie pliku | Opcjonalne (AES-128-CBC) | Brak |
| MD5 per slot | Tak (16B prefix) | Nie |
| MD5 na UD10/UD11 | Tak | Nie |
| SteamID w UD10 | Tak (offset 0x04) | Nie |
| SteamID w slocie | Tak (ostatnie 8B) | Nie |
| ActiveSlots offset w UD10 | `0x310` | `0x300` |
| ProfileSummaries offset | `0x31A` | `0x30A` |
| PCOptionData w UD10 | Tak (0xB2 B) | Nie |
| Slot data | Identyczny `0x280000` B | Identyczny `0x280000` B |
| Wewnętrzna struktura slotu | **Identyczna** | **Identyczna** |

**Wniosek**: Różnice dotyczą TYLKO kontenera (nagłówek, checksums, szyfrowanie). Dane postaci wewnątrz slotu mają identyczny format na obu platformach.

---

## 12. Hash integralności slotu (PlayerGameDataHash)

### Lokalizacja

Ostatnie 0x80 (128) bajtów każdego slotu, offset `SlotSize - 0x80 = 0x27FF80`.

### Algorytm (Adler-like)

32 pola `uint32` (tylko 12 użytych), reszta zerowana.

**Funkcja bazowa `computeHashedValue(input)`**:
```
product = uint64(0x80078071) × uint64(input)
upper = uint32(product >> 32)
shifted = upper >> 15
mod = int32(shifted) × (-0xFFF1)    // -65521 = modulus Adler-32
return input + uint32(mod)
```

Jest to operacja `input mod 65521` (stała Adler-32).

**Funkcja `bytesHash(data)`**:
```
lo = 1, hi = 0
for each byte b:
    lo += b
    hi += lo
loH = computeHashedValue(lo)
hiH = computeHashedValue(hi)
return (loH | (hiH << 16)) × 2
```

### Wpisy hash

| Indeks | Zawartość |
|---|---|
| 0 | valueHash(Level) |
| 1 | statsHash(9 statów + Humanity) — **Int i Faith zamienione!** |
| 2 | valueHash(Class) |
| 3 | bytesHash(PGD+0xB8 bajt) |
| 4 | 0 (padding) |
| 5 | valueHash(Souls) |
| 6 | valueHash(SoulMemory) |
| 7 | equipmentHash(10 weapon slot IDs) |
| 8 | equipmentHash(4 armor + 5 talisman IDs) |
| 9 | quickItemsHash(16 quick/pouch IDs, & 0x0FFFFFFF) |
| 10 | equipmentHash(14 spell IDs) |
| 11 | 0 (padding) |

### Czy gra waliduje hash?

**Niejednoznaczne**. Nasz kod zawiera komentarz "gra nie waliduje", ale jednocześnie implementuje `RecalculateSlotHash()`. Referencyjne edytory (Rust) czytają hash i mogą go przeliczać. Nasz round-trip test wyklucza region hash z porównania, co sugeruje, że hash jest nieprawidłowo przeliczany lub zerowany.

---

## 13. Algorytm odczytu pliku save

### Krok po kroku (na podstawie 3 referencyjnych edytorów)

```
1. Wczytaj cały plik do pamięci (bytearray)

2. DETEKCJA PLATFORMY:
   - Bajty 0-3 == "BND4" → PC (nieszyfrowany)
   - Po AES-128 decryption bajty 0-3 == "BND4" → PC (szyfrowany)
   - Bajty 0-3 == 0xCB019C2C → PS4
   
3. NAGŁÓWEK:
   PC:  przeczytaj 0x300 bajtów (BND4 header)
   PS4: przeczytaj 0x70 bajtów
   
4. DLA KAŻDEGO SLOTU (0-9):
   PC:  skip 0x10 (MD5), czytaj 0x280000 bajtów
   PS4: czytaj 0x280000 bajtów (bez MD5)
   
   4a. Sprawdź version (u32 @ offset 0) — jeśli 0, slot pusty
   
   4b. Parsuj GaItems:
       - Start: offset 0x20
       - Czytaj handle (u32), sprawdź typ, czytaj odpowiednią liczbę bajtów
       - Powtarzaj dla 5118/5120 wpisów (LUB do końca sekcji)
       - Zapamiętaj mapę handle → itemID
   
   4c. Znajdź MagicPattern (64B wzorzec) → ustaw MagicOffset
   
   4d. Czytaj PlayerGameData z MagicOffset + negatywne offsety
   
   4e. Oblicz łańcuch dynamicznych offsetów (2 pola dynamiczne)
   
   4f. Parsuj inwentarz, storage, event flags, koordynaty, itp.

5. USERDATA10:
   PC:  skip 0x10 (MD5), czytaj 0x60000 bajtów
   PS4: czytaj 0x60000 bajtów
   - Parsuj active_slots, profile_summaries, SteamID

6. USERDATA11:
   PC:  skip 0x10 (MD5), czytaj resztę
   PS4: czytaj resztę
```

---

## 14. Algorytm zapisu pliku save

### Krok po kroku (na podstawie referencji)

```
1. WALIDACJA PRE-SAVE:
   - Sprawdź len(slot.Data) == 0x280000
   - Sprawdź poprawność łańcucha offsetów
   - Sprawdź granice statystyk (level 1-713, atrybuty 1-99)

2. FLUSH METADANYCH:
   - Zapisz active_slots do UserData10
   - Zapisz profile_summaries do UserData10
   - Zapisz SteamID do UserData10 (PC only)

3. DLA KAŻDEGO SLOTU:
   - Zapisz statystyki do slot.Data (MagicOffset + offsety)
   - Zapisz SteamID na końcu slotu (PC only, offset SlotSize - 8)
   
4. SERIALIZACJA:
   PC:
   - Nagłówek BND4 (0x300)
   - Dla każdego slotu: MD5(slot.Data) + slot.Data
   - MD5(UD10.Data) + UD10.Data
   - MD5(UD11) + UD11
   
   PS4:
   - Nagłówek PS4 (0x70)
   - Dla każdego slotu: slot.Data (bez MD5)
   - UD10.Data (bez MD5)
   - UD11

5. SZYFROWANIE (opcjonalnie, PC only):
   - Jeśli save był szyfrowany: AES-128-CBC encrypt z zachowanym IV

6. ZAPIS ATOMOWY:
   - Zapisz do pliku .tmp
   - Rename .tmp → docelowy plik
```

---

## 15. Algorytm edycji danych postaci

### Zmiana statystyk

```
1. Zmodyfikuj wartości w strukturze PlayerGameData
2. Zapisz nowe wartości do slot.Data[MagicOffset + offset]
3. Przelicz level: sum(all_stats) - 79
4. Zaktualizuj ProfileSummary w UserData10 (level, name)
```

### Dodawanie przedmiotów

```
1. DODAJ REKORD GaItem (sekcja 0x20+):
   a. Określ typ z itemID prefix → handle prefix
   b. Wygeneruj unikalny handle (lub użyj istniejącego dla stackable)
   c. Zapisz rekord odpowiedniej długości (8/16/21B) na InventoryEnd
   d. Zaktualizuj InventoryEnd
   e. CZYŚĆ pozostałe bajty do granicy sekcji (zapobiegaj desync)

2. DODAJ DO GaItemData (sekcja GaItemDataOther):
   a. Dla broni/AoW: dodaj wpis (id, unk, reinforce_type, unk1)
   b. Inkrementuj counter

3. DODAJ DO INWENTARZA:
   a. Znajdź pusty slot (handle == 0 lub 0xFFFFFFFF)
   b. Ustaw handle, quantity, index
   c. Index MUSI być > 432 (InvEquipReservedMax)
   d. Inkrementuj NextAcquisitionSortId
```

### Usuwanie przedmiotów

```
1. Wyzeruj matching wpisy w inwentarzu/storage
2. Jeśli handle nie istnieje w żadnej liście → usuń z GaMap
```

---

## 16. Analiza naszego kodu vs referencje

### Poprawne implementacje

| Aspekt | Status | Notatki |
|---|---|---|
| Detekcja platformy (magic bytes) | ✅ | PC/PS4 poprawnie rozpoznawane |
| Nagłówek BND4/PS4 | ✅ | Poprawne rozmiary i budowa |
| AES-128-CBC decrypt/encrypt | ✅ | Klucz i tryb poprawne |
| MD5 per-slot checksums | ✅ | Poprawnie przeliczane |
| MagicPattern wyszukiwanie | ✅ | 64B wzorzec, fallback offset |
| Stat offsety (negatywne od Magic) | ✅ | Zgodne z referencjami |
| Inwentarz held/storage layout | ✅ | Pojemności i format zgodne |
| GaItem rozmiary rekordów | ✅ | 21/16/8 bajtów |
| Dynamiczny łańcuch offsetów | ✅ | Offsety stałe zgodne z referencjami |
| Profile summaries | ✅ | Poprawne offsety PC/PS4 |
| Atomowy zapis (tmp + rename) | ✅ | Poprawna implementacja |
| Walidacja pre-save | ✅ | Sprawdza granice, offsets |

### Kluczowe różnice

| Aspekt | Nasz kod | Referencje |
|---|---|---|
| **GaItem scan** | Skanuje od 0x20 do MagicOffset-0x1B0 | Czyta stałą liczbę wpisów (5118/5120) |
| **ReadBytes** | Zwraca slice (aliasing) | Kopia danych |
| **Hash rekalkulacja** | NIE wywołana w ścieżce zapisu | Niektóre referencje przeliczają |
| **Version field check** | Nie sprawdzamy version per slot | Referencje sprawdzają (0 = empty, >81 = nowy format) |
| **Strategia zapisu** | Modyfikujemy buffer w miejscu | er-save-manager: raw-data patching z opcjonalnym pełnym rebuild |

---

## 17. Znalezione błędy w naszym kodzie

### 🔴 KRYTYCZNE (powodują crash gry)

#### BUG-1: Brak przesunięcia danych po dodaniu GaItems

**Plik**: `backend/core/writer.go` → `writeGaItem()`

**Problem**: Dodanie nowych rekordów GaItem przesuwa `InventoryEnd` w przód, ale dane za sekcją GaItems (PlayerGameData, equipment, event flags, etc.) **nie są przesuwane** w buforze. Gra oblicza offsety dynamicznie na podstawie rozmiaru sekcji GaItems, więc po dodaniu przedmiotów gra szuka danych statystyk/ekwipunku pod błędnymi offsetami.

**Efekt**: Gra czyta śmieci jako wskaźniki i dereferencjonuje adres `0xFFFFFFFFFFFFFFFF` → crash `page fault on read access`.

**Uwaga**: To jest główna przyczyna crash'y z katalogu `analiza/`. Na stosie widać wartość `0x808000ba` — nieprawidłowy GaItemHandle.

#### BUG-2: Brak wersjonowania GaItem count

**Plik**: `backend/core/structures.go` → `scanGaItems()`

**Problem**: Nasz skaner nie sprawdza pola `version` (offset 0x00 w slocie) i zawsze skanuje do MagicOffset. Referencje czytają stałą liczbę wpisów: 5118 (version ≤ 81) lub 5120 (version > 81). Skanowanie "do końca" jest ryzykowne — garbage bytes mogą być mylnie interpretowane jako rekordy.

#### BUG-3: Nieprawidłowe handle'e dla strzał/bełtów

**Plik**: `backend/core/writer.go` → `AddItemsToSlot()`

**Problem**: Strzały/bełty (item ID prefix 0x02/0x03) mogą być traktowane jako broń (`ItemTypeWeapon`), co daje 21-bajtowe rekordy GaItem zamiast 8-bajtowych. To desynchronizuje cały skaner GaItems.

### 🟡 WAŻNE (mogą powodować problemy)

#### BUG-4: Hash offset chain nie uwzględnia dynamicznego projSize

**Plik**: `backend/core/hash.go` → `ComputeSlotHash()`

**Problem**: Obliczanie offsetów dla wpisów hash [9] (quickItems) i [10] (spells) pomija dynamiczny `projSize`. Używa uproszczonego łańcucha offsetów, który nie odczytuje `projSize` z danych save. Hash będzie obliczony z błędnych danych.

**Wpływ**: Aktualnie brak wpływu, bo `RecalculateSlotHash()` nie jest wywoływany w ścieżce zapisu. Ale stanie się krytyczny, gdy hash zostanie włączony.

#### BUG-5: quickItemsHash czyta z błędnego offsetu

**Plik**: `backend/core/hash.go` → `readQuickItemIDs()`

**Problem**: Czyta 16 × u32 od początku sekcji `equipedItems` bez pominięcia nagłówka ChrAsmEquipment. Czyta dane ekwipunku zamiast quick items.

#### BUG-6: upsertGaItemData zawsze ustawia reinforce_type = 0

**Plik**: `backend/core/writer.go` → `upsertGaItemData()`

**Problem**: Nowe wpisy GaItemData mają `reinforce_type = 0` niezależnie od poziomu ulepszenia broni. Jeśli gra sprawdza to pole, broń +10 może zachowywać się jak +0.

#### BUG-7: Undo nie zachowuje unexported offset fields

**Plik**: `app.go` → `pushUndo()`

**Problem**: Deep-copy `Inventory`/`Storage` tworzy nowe struct'y bez ustawionych `nextEquipIndexOff`/`nextAcqSortIdOff`. Po revert `addToInventory` sprawdza `if slot.Inventory.nextAcqSortIdOff > 0` — warunek fałszywy → counter write-back nie działa → stale acquisition sort IDs.

### 🟢 DROBNE

#### BUG-8: ReadStorage przerywa na pierwszym pustym handle

**Problem**: Jeśli storage ma sparse slots (handle=0 w środku), przedmioty po przerwie są tracone.

#### BUG-9: EquipInventoryData.Read ignoruje błędy

**Problem**: Wszystkie `_ = err` patterny w `ReadU32`. Truncated slot produkuje cicho błędne dane.

#### BUG-10: ComputeSHA256 jest dead code

**Plik**: `backend/core/crypto.go`

**Problem**: Zadeklarowana ale nigdzie nie używana.

---

## 18. Analiza crashy gry

### Źródło: `analiza/steam-1245620.log.*.txt`

Crash'e występują na **Steam Deck** (Linux, Proton 10.0-4).

### Sygnatura crash'a

```
wine: Unhandled page fault on read access to FFFFFFFFFFFFFFFF
at address 000000014067150A (thread 01b0/01b4)

rip: 0x14067150a (eldenring.exe + 0x67150a)
r14: 0xffffffff
```

### Diagnoza

1. Wartość `0xFFFFFFFF` w rejestrze r14 to sentinel "invalid handle" z sekcji GaItems
2. Gra próbuje dereferencjonować ten handle jako wskaźnik → page fault
3. Na stosie widoczne `0x808000ba` — nieprawidłowy format handle'a (poprawny to `0x80xxxxxx` z sekwencyjnym dolnym word'em)

### Przyczyna główna

**BUG-1** (patrz sekcja 17): Dodanie przedmiotów do GaItems przesuwa granicę sekcji, ale nie przesuwa danych za nią. Gra oblicza `PlayerDataOffset = GaItems_End + 0x1B0` dynamicznie i trafia na garbage data.

### Pliki bez crash'a

`steam-1245620.log.7.txt` — normalne uruchomienie gry. Prawdopodobnie użyto niezmodyfikowanego save'a.

---

## 19. Porównanie binarne plików save

### Pliki

- **Oryginalny**: `tmp/save/ER0000.sl2` (28,967,888 B)
- **Edytowany**: `tmp/save/ER0000-out.sl2` (28,967,888 B)

### Wyniki

| Metryka | Wartość |
|---|---|
| Rozmiar | Identyczny (28,967,888 B) |
| Łączna liczba różnic | **20,249 bajtów** (0.07%) |
| Zmodyfikowane sloty | **Slot 0** i **Slot 4** |
| Nagłówek BND4 | Identyczny |
| UserData10 | Identyczny |
| UserData11 | Identyczny |

### Rozkład zmian

#### Slot 0 (6 klastrów, ~650 B zmian danych)

| Offset w pliku | Offset w slocie | Rozmiar | Opis |
|---|---|---|---|
| 0x300-0x30F | — | 16B | MD5 checksum (przeliczony) |
| 0xBBEE-0xC0C7 | 0xB8DE | 630B | Region inwentarza (dodane przedmioty) |
| 0xCD46-0xCD4D | 0xCA36 | 5B | Mała edycja |
| 0x1547C-0x1547D | 0x1516C | 2B | Mała edycja |
| 0x1B782-0x1B78F | 0x1B472 | 7B | Mała edycja |
| 0x2178E-0x2178F | 0x2147E | 2B | Mała edycja |
| 0x37EBF-0x37EC9 | 0x37BAF | 4B | Mała edycja |

#### Slot 4 (6 klastrów, ~19,570 B zmian danych)

| Offset w pliku | Offset w slocie | Rozmiar | Opis |
|---|---|---|---|
| 0xA00340-0xA0034F | — | 16B | MD5 checksum (przeliczony) |
| **0xA00C9A-0xA0A53B** | **0x94A** | **19,546B** | **Masowe dodanie przedmiotów** |
| 0xA0AA22-0xA0AA29 | 0xA6D2 | 5B | Mała edycja |
| 0xA138F0-0xA138F1 | 0x135A0 | 2B | Mała edycja |
| 0xA19BF6-0xA19C05 | 0x198A6 | 9B | Mała edycja |
| 0xA1FC02-0xA1FC02 | 0x1F8B2 | 1B | Pojedynczy bajt |
| 0xA36333-0xA3633D | 0x35FE3 | 4B | Mała edycja |

### Wnioski z porównania

1. **MD5 checksums przeliczane poprawnie** — zmiana 16B na początku każdego zmodyfikowanego slotu
2. **Główna zmiana w Slot 4** (19,546B) to region GaItems (offset 0x94A w slocie, zaczyna się blisko 0x20) — widać wzorzec `00000000 FFFFFFFF` (puste sloty) zastąpiony strukturalnymi danymi przedmiotów
3. **Dane NIE wychodzą poza granice slotu** — brak korupcji nagłówka/UD10
4. **Ale**: dodane przedmioty przesuwają `InventoryEnd` bez przesunięcia kolejnych sekcji — to jest BUG-1

---

## 20. Rekomendacje napraw

### Priorytet 1 — KRYTYCZNE (fix crashy)

#### R-1: Implementacja pełnego slot rebuild zamiast in-place patching

Wzorując się na `er-save-manager/parser/slot_rebuild.py`:

```
Zamiast: modyfikuj buffer[InventoryEnd] i licz, że reszta się zgadza
Zrób:    
  1. Sparsuj cały slot do struktur danych
  2. Zmodyfikuj struktury (dodaj/usuń przedmioty)
  3. Serializuj WSZYSTKO od nowa do bufora 0x280000
  4. Wypełnij resztę zerami
```

To eliminuje BUG-1 w sposób fundamentalny — nie ma problemu z przesunięciem danych, bo cały slot jest pisany od zera.

**Koszt**: Duży refaktor. Wymaga pełnego parsowania i serializacji każdej sekcji.

#### R-2: Alternatywa — przesuwanie danych po GaItems

Jeśli pełny rebuild jest zbyt kosztowny:

```
1. Oblicz delta = newInventoryEnd - oldInventoryEnd
2. Przesuń bytes[oldInventoryEnd:] o delta w prawo
3. Zaktualizuj WSZYSTKIE offsety w łańcuchu dynamicznym
4. Przelicz MagicOffset (MagicPattern się przesunęło!)
```

**Ryzyko**: Każdy pominięty offset = korupcja. Łańcuch ma 20+ pól + 2 dynamiczne.

#### R-3: Stała liczba GaItems zamiast skanowania

```go
// Zamiast:
func (s *SaveSlot) scanGaItems(start int) { ... scan to magic ... }

// Zrób:
version := binary.LittleEndian.Uint32(s.Data[0:4])
count := 5120
if version <= 81 { count = 5118 }
for i := 0; i < count; i++ { ... read fixed count ... }
```

### Priorytet 2 — WAŻNE

#### R-4: Poprawka rozmiaru rekordu dla strzał/bełtów

Strzały/bełty to `ItemTypeItem` (goods), nie `ItemTypeWeapon`. Powinny mieć 8B rekordy GaItem.

#### R-5: Włącz RecalculateSlotHash z poprawnym łańcuchem offsetów

Napraw `ComputeSlotHash()` żeby uwzględniał dynamiczny `projSize`, potem włącz w ścieżce zapisu.

#### R-6: Fix undo snapshot

Kopiuj `nextEquipIndexOff` i `nextAcqSortIdOff` przy deep-copy. Wymaga exportowania tych pól lub dodania metody `Clone()`.

### Priorytet 3 — ULEPSZENIA

#### R-7: Strategia raw-data patching (jak er-save-manager)

Zamiast budować cały plik od nowa, trzymaj oryginalny `[]byte` i modyfikuj in-place z tracked offsets. Bezpieczniejsze niż pełna serializacja, bo nie musisz znać formatu każdej sekcji — zmieniasz tylko to co trzeba.

#### R-8: Walidacja version field

Sprawdzaj `version` na offset 0 każdego slotu. Version 0 = pusty slot. Version > 81 = nowy format (5120 GaItems).

#### R-9: Obsługa DLC flags przy konwersji

Bajt DLC[1] (Shadow of Erdtree entry flag) powinien być wyzerowany przy konwersji, jeśli docelowa platforma nie ma DLC. Non-zero = nieskończone ładowanie.

---

## Załącznik A: Stałe i magiczne liczby (pełna lista)

```
// Rozmiary
SlotSize                = 0x280000  // 2,621,440
PCHeaderSize            = 0x300     // 768
PSHeaderSize            = 0x70      // 112
MD5Size                 = 0x10      // 16
UserData10Size          = 0x60000   // 393,216
UserData11Size          = 0x240000  // 2,359,296
EventFlagsSize          = 0x1BF99F  // 1,833,375
NetManDataSize          = 0x20000   // 131,072
FaceDataSize            = 0x12F     // 303
FaceDataProfileSize     = 0x120     // 288 (skrócona wersja w profile summary)
ProfileSummarySize      = 0x24C     // 588

// GaItems
GaItemsStart            = 0x20
GaItemCountOld          = 0x13FE    // 5118 (version ≤ 81)
GaItemCountNew          = 0x1400    // 5120 (version > 81)
GaItemGameDataEntries   = 7000      // 0x1B58
GaItemDataEntrySize     = 16        // id(4) + unk(4) + reinforce(4) + unk1(4)
WeaponRecordSize        = 21
ArmorRecordSize         = 16
DefaultRecordSize       = 8

// Inwentarz
HeldCommonCapacity      = 0xA80     // 2688
HeldKeyCapacity         = 0x180     // 384
StorageCommonCapacity   = 0x780     // 1920
StorageKeyCapacity      = 0x80      // 128
InvRecordSize           = 12        // handle(4) + qty(4) + idx(4)
InvEquipReservedMax     = 432

// Handle masks
HandleWeapon            = 0x80000000
HandleArmor             = 0x90000000
HandleAccessory         = 0xA0000000
HandleItem              = 0xB0000000
HandleAoW               = 0xC0000000
HandleEmpty             = 0x00000000
HandleInvalid           = 0xFFFFFFFF
HandleTypeMask          = 0xF0000000

// ItemID prefixes
ItemIDWeapon            = 0x00000000
ItemIDArmor             = 0x10000000
ItemIDAccessory         = 0x20000000
ItemIDGoods             = 0x40000000
ItemIDAoW               = 0x60000000

// Crypto
AES128Key               = [16]byte{0x99, 0xAD, 0x2D, 0x50, 0xED, 0xF2, 0xFB, 0x01,
                                    0xC5, 0xF3, 0xEC, 0x3A, 0x2B, 0xCA, 0xB6, 0x9D}
RegulationAES256Key     = [32]byte{0x99, 0xBF, 0xFC, 0x36, ...}

// Hash
HashMagic               = 0x80078071
HashSize                = 0x80      // 128
HashOffset              = SlotSize - HashSize  // 0x27FF80
HashEntries             = 12
AdlerModulus            = 65521

// Platform magic bytes
BND4Magic               = "BND4"    // 0x42 0x4E 0x44 0x34
PS4Magic                = []byte{0xCB, 0x01, 0x9C, 0x2C}

// Dynamiczny łańcuch (stałe offsety)
DynSpEffect             = 0xD0
DynEquipedItemIndex     = 0x58
DynActiveEquipedItems   = 0x1C
DynEquipedItemsID       = 0x58
DynActiveEquipedItemsGa = 0x58
DynInventoryHeld        = 0x9010
DynEquipedSpells        = 0x74
DynEquipedItems         = 0x8C
DynEquipedGestures      = 0x18
DynEquipedArmaments     = 0x9C
DynEquipePhysics        = 0x0C
DynFaceData             = 0x12F
DynStorageBox           = 0x6010
DynGestureGameData      = 0x100
DynHorse                = 0x29
DynBloodStain           = 0x4C
DynMenuProfile          = 0x103C
DynGaItemsOther         = 0x1B588
DynTutorialData         = 0x40B
DynIngameTimer          = 0x1A
DynEventFlags           = 0x1C0000

// Limity dynamicznych pól
MaxProjSize             = 256
MaxUnlockedRegSz        = 1024
```

---

## Załącznik B: Referencyjne edytory — podsumowanie

| Edytor | Język | Szyfrowanie .sl2 | Checksums | GaItem parsing | Slot rebuild |
|---|---|---|---|---|---|
| **Elden-Ring-Save-Editor** | Python | ❌ Brak | ✅ MD5 | Skanowanie wzorcowe | ❌ |
| **ER-Save-Editor** | Rust | ❌ Brak | ✅ MD5 | Stała liczba (5118/5120) | ✅ Pełny |
| **er-save-manager** | Python | ❌ Brak | ✅ MD5 | Stała liczba (5118/5120) | ✅ Pełny |
| **Nasz edytor** | Go | ✅ AES-128 | ✅ MD5 | Skanowanie do Magic | ❌ In-place |

### Kluczowe lekcje z referencji

1. **Żaden referencyjny edytor nie szyfruje/deszyfruje pliku .sl2** — wszyscy operują na plaintext BND4
2. **Rust i Python (er-save-manager) używają pełnego rebuildu slotu** — serializują wszystkie sekcje od nowa
3. **Stała liczba GaItem wpisów** (nie skanowanie) jest bezpieczniejsza
4. **Version field** (offset 0) jest sprawdzany — 0 = pusty slot, wartość wpływa na liczbę GaItems

---

*Dokument wygenerowany na podstawie analizy kodu źródłowego projektu, 3 referencyjnych edytorów (Elden-Ring-Save-Editor/Python, ER-Save-Editor/Rust, er-save-manager/Python), logów crashy gry, oraz porównania binarnego plików save.*
