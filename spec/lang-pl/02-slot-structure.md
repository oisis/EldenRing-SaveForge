# 02 — Slot — Struktura Ogólna

> **Zakres**: Ogólna budowa jednego slotu postaci (SaveSlot / UserDataX). Kolejność sekcji, sekcje stałe vs zmienne.

---

## Podstawowe fakty

- **Rozmiar slotu**: 0x280000 bytes (2,621,440 bytes) — stały, niezależny od zawartości
- **Endianness**: Little-endian
- **Parsing**: Sekwencyjny — sekcje idą jedna po drugiej, wiele ma zmienną długość
- **Wersja**: Pierwszy u32 slotu. Określa wariant parsowania (np. 5118 vs 5120 GaItems, dodatkowe pola w nowszych wersjach)

---

## Kolejność sekcji w slocie

Poniższa lista przedstawia **dokładną sekwencję** w jakiej dane są zapisane w slocie.
Sekcje oznaczone `[VARIABLE]` mają zmienną długość — kolejne sekcje nie mają stałych offsetów.

```
Offset (przybliżony)   Sekcja                              Rozmiar
──────────────────────────────────────────────────────────────────────────
0x00                    Slot Header (version + map + unk)    32 bytes
0x20                    GaItem Map                           [VARIABLE: 5118×var lub 5120×var]
(dynamiczny)            PlayerGameData                       0x1B0 (432 bytes)
(dynamiczny)            SP Effects                           [VARIABLE: 13 entries]
(dynamiczny)            EquippedItems EquipIndex             0x58 (88 bytes)
(dynamiczny)            ActiveWeaponSlots + ArmStyle         0x1C (28 bytes)
(dynamiczny)            EquippedItems ItemIDs                0x58 (88 bytes)
(dynamiczny)            EquippedItems GaitemHandles          0x58 (88 bytes)
(dynamiczny)            Inventory Held                       [VARIABLE]
(dynamiczny)            Equipped Spells                      (stała)
(dynamiczny)            Equipped Items (quick/pouch)         (stała)
(dynamiczny)            Equipped Gestures                    (stała)
(dynamiczny)            Acquired Projectiles                 [VARIABLE: count × 8]
(dynamiczny)            Equipped Armaments & Items           (stała)
(dynamiczny)            Equipped Physics                     (stała)
(dynamiczny)            Face Data                            0x12F (303 bytes)
(dynamiczny)            Inventory Storage Box                [VARIABLE]
(dynamiczny)            Gestures                             0x100 (256 bytes = 64 × u32)
(dynamiczny)            Unlocked Regions                     [VARIABLE: 4 + count×4]
(dynamiczny)            Torrent / RideGameData               0x28 (40 bytes)
(dynamiczny)            Control Byte                         1 byte
(dynamiczny)            Blood Stain                          0x44 (68 bytes)
(dynamiczny)            Unknown fields (2 × u32)            8 bytes
(dynamiczny)            Menu Profile SaveLoad                [VARIABLE: 8 + size]
(dynamiczny)            Trophy Equip Data                    (stała)
(dynamiczny)            GaItem Game Data                     [VARIABLE: 8 + 7000×16]
(dynamiczny)            Tutorial Data                        [VARIABLE: 8 + size]
(dynamiczny)            GameMan bytes                        3 bytes
(dynamiczny)            Death/Character/Session state        [VARIABLE: ~32 bytes]
(dynamiczny)            Event Flags                          0x1BF99F (1,833,375 bytes)
(dynamiczny)            Event Flags Terminator               4 bytes
(dynamiczny)            FieldArea                            [VARIABLE: 4 + size]
(dynamiczny)            WorldArea                            [VARIABLE: 4 + size]
(dynamiczny)            WorldGeomMan (×2)                    [VARIABLE: 4 + size]
(dynamiczny)            RendMan                              [VARIABLE: 4 + size]
(dynamiczny)            Player Coordinates                   0x39 (57 bytes)
(dynamiczny)            GameMan spawn bytes                  ~12-20 bytes (version-dependent)
(dynamiczny)            NetMan                               0x20004 (131,076 bytes)
(dynamiczny)            WorldAreaWeather                     0x0C (12 bytes)
(dynamiczny)            WorldAreaTime                        0x0C (12 bytes)
(dynamiczny)            BaseVersion                          0x10 (16 bytes)
(dynamiczny)            Steam ID                             0x08 (8 bytes)
(dynamiczny)            PS5 Activity                         0x20 (32 bytes)
(dynamiczny)            DLC                                  0x32 (50 bytes)
(dynamiczny)            PlayerGameData Hash                  [reszta do końca slotu]
──────────────────────────────────────────────────────────────────────────
```

---

## Slot Header (32 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | Version — numer wersji formatu save |
| 0x04 | u8[4] | Map ID — identyfikator aktualnej mapy |
| 0x08 | u8[8] | Unknown |
| 0x10 | u8[16] | Unknown |

### Wersja slotu

- `version == 0` → slot jest pusty
- `version <= 81` → stary format (5118 GaItems)
- `version > 81` → nowy format (5120 GaItems)
- `version >= 65` → dodatkowe pole `temp_spawn_point_entity_id`
- `version >= 66` → dodatkowe pole `game_man_0xcb3`

---

## Sekcje zmiennej długości — kluczowy problem

Wiele sekcji ma rozmiar zależny od stanu postaci. To oznacza, że **nie da się użyć stałych offsetów** do sekcji po GaItem Map. Trzeba parsować sekwencyjnie.

Główne źródła zmienności:
1. **GaItem Map** — rozmiar rekordu zależy od typu przedmiotu (broń=21B, zbroja=16B, inne=8B)
2. **Acquired Projectiles** — count × 8 bytes
3. **Unlocked Regions** — count × 4 bytes
4. **Inventory** — stała liczba slotów, ale powiązane countery
5. **World areas** — size-prefixed, zmienne
6. **GaItem Game Data** — 7000 entries ale 8-bajtowy header

---

## Implikacje dla edycji

1. **Modyfikacja sekcji stałej** (np. PlayerGameData) — wystarczy zapisać nowe bajty w tym samym miejscu
2. **Modyfikacja sekcji zmiennej** (np. Regions) — zmiana rozmiaru wymaga przesunięcia WSZYSTKICH kolejnych sekcji
3. **Checksum** (PC) — po każdej modyfikacji MUSI być przeliczony MD5 całego slotu

---

## Klasy startowe — Base Stats Reference

| ID | Klasa | Start Lvl | Vig | Mnd | End | Str | Dex | Int | Fai | Arc | Sum |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 0 | Vagabond | 9 | 15 | 10 | 11 | 14 | 13 | 9 | 9 | 7 | 88 |
| 1 | Warrior | 8 | 11 | 12 | 11 | 10 | 16 | 10 | 8 | 9 | 87 |
| 2 | Hero | 7 | 14 | 9 | 12 | 16 | 9 | 7 | 8 | 11 | 86 |
| 3 | Bandit | 5 | 10 | 11 | 10 | 9 | 13 | 9 | 8 | 14 | 84 |
| 4 | Astrologer | 6 | 9 | 15 | 9 | 8 | 12 | 16 | 7 | 9 | 85 |
| 5 | Prophet | 7 | 10 | 14 | 8 | 11 | 10 | 7 | 16 | 10 | 86 |
| 6 | Confessor | 10 | 10 | 13 | 10 | 12 | 12 | 9 | 14 | 9 | 89 |
| 7 | Samurai | 9 | 12 | 11 | 13 | 12 | 15 | 9 | 8 | 8 | 88 |
| 8 | Prisoner | 9 | 11 | 12 | 11 | 11 | 14 | 14 | 6 | 9 | 88 |
| 9 | Wretch | 1 | 10 | 10 | 10 | 10 | 10 | 10 | 10 | 10 | 80 |

**Formuła**: `Level = Sum(all_attributes) - 79`

Wartości potwierdzone z dwóch niezależnych Cheat Engine tables (Hexinton + TGA).

---

## Źródła

- er-save-manager: `parser/user_data_x.py` — klasa `UserDataX` z pełną sekwencją pól (linie 54-198)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — struktury w kolejności parsowania
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Class reset scripts (base stats)
- Cheat Engine: `ER_TGA_v1.9.0` — Class definitions
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=format:sl2
