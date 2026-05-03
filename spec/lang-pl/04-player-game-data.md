# 04 — PlayerGameData (Statystyki Postaci)

> **Zakres**: Główna struktura danych postaci — 432 bajty (0x1B0). Zawiera wszystkie atrybuty, level, runy, imię, klasę, ustawienia online.

---

## Opis ogólny

PlayerGameData to stała struktura 432 bajtów zawierająca kluczowe informacje o postaci. Jest parsowana bezpośrednio po GaItem Map.

---

## Pełna mapa pól (0x1B0 = 432 bytes)

### HP / FP / SP (0x00–0x33)

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | u32 | unk0x0 | Nieznane (prawdopodobnie PlayerNo / internal ID) |
| 0x04 | u32 | unk0x4 | Nieznane |
| 0x08 | u32 | HP | Aktualne zdrowie |
| 0x0C | u32 | MaxHP | Maksymalne HP (z buffami/taliszmanami) |
| 0x10 | u32 | BaseMaxHP | Bazowe max HP (czyste, z samych atrybutów) |
| 0x14 | u32 | FP | Aktualny Focus Points (mana) |
| 0x18 | u32 | MaxFP | Maksymalne FP (z buffami) |
| 0x1C | u32 | BaseMaxFP | Bazowe max FP |
| 0x20 | u32 | unk0x20 | Nieznane (może powiązane z FP regen?) |
| 0x24 | u32 | SP | Aktualna wytrzymałość (stamina) |
| 0x28 | u32 | MaxSP | Maksymalne SP (z buffami) |
| 0x2C | u32 | BaseMaxSP | Bazowe max SP |
| 0x30 | u32 | unk0x30 | Nieznane |

**Opis pól HP/FP/SP**:
- `HP/FP/SP` — aktualna wartość w momencie zapisu (odzyskuje się po rest)
- `MaxXX` — aktualny cap z uwzględnieniem taliszmanów, Great Rune itp.
- `BaseMaxXX` — cap obliczany wyłącznie z atrybutów (Vigor→HP, Mind→FP, Endurance→SP)

---

### Atrybuty (0x34–0x5F)

| Offset | Typ | Pole | Opis | Zakres |
|---|---|---|---|---|
| 0x34 | u32 | Vigor | Witalność — skaluje HP | 1–99 |
| 0x38 | u32 | Mind | Umysł — skaluje FP i attunement slots | 1–99 |
| 0x3C | u32 | Endurance | Wytrzymałość — skaluje SP i equip load | 1–99 |
| 0x40 | u32 | Strength | Siła — skalowanie broni STR | 1–99 |
| 0x44 | u32 | Dexterity | Zręczność — skalowanie broni DEX, cast speed | 1–99 |
| 0x48 | u32 | Intelligence | Inteligencja — skalowanie sorceries, magic dmg | 1–99 |
| 0x4C | u32 | Faith | Wiara — skalowanie incantations, holy dmg | 1–99 |
| 0x50 | u32 | Arcane | Tajemność — skalowanie discovery, buildup, dragon | 1–99 |
| 0x54 | u32 | unk0x54 | Nieznane (powiązane z atrybutami? padding?) |  |
| 0x58 | u32 | unk0x58 | Nieznane |  |
| 0x5C | u32 | unk0x5c | Nieznane |  |

**Formuła levelu**: `Level = Vigor + Mind + Endurance + Strength + Dexterity + Intelligence + Faith + Arcane - 79`
- Minimum: Level 1 (Wretch, wszystkie atrybuty = 10)
- Maksimum: Level 713 (wszystkie atrybuty = 99)

---

### Level i Runy (0x60–0x6F)

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x60 | u32 | Level | Aktualny level postaci (1–713) |
| 0x64 | u32 | Runes | Runy aktualnie posiadane (utracone przy śmierci) |
| 0x68 | u32 | TotalGetSoul | Łącznie zdobyte runy w historii postaci (lifetime) |
| 0x6C | u32 | unk0x6c | Nieznane |

---

### Status Buildups / Odporności (0x70–0x93)

Status buildups to **akumulatory** efektów statusu. Gdy buildup osiąga próg, efekt się aktywuje. Wartości te normalnie wynoszą 0 (brak nagromadzonego efektu).

| Offset | Typ | Pole | Opis gry |
|---|---|---|---|
| 0x70 | u32 | Immunity | Poison buildup — odporność na truciznę |
| 0x74 | u32 | Immunity2 | Scarlet Rot buildup — odporność na czerwoną zgniliznę |
| 0x78 | u32 | Robustness | Hemorrhage (Bleed) buildup — odporność na krwawienie |
| 0x7C | u32 | Vitality | Deathblight buildup — odporność na natychmiastową śmierć |
| 0x80 | u32 | Robustness2 | Frostbite buildup — odporność na mróz |
| 0x84 | u32 | Focus | Sleep buildup — odporność na uśpienie |
| 0x88 | u32 | Focus2 | Madness buildup — odporność na szaleństwo |
| 0x8C | u32 | unk0x8c | Nieznane (ewentualnie: Blight DLC?) |
| 0x90 | u32 | unk0x90 | Nieznane |

**Uwagi**:
- "Immunity" w grze = odporność na Poison + Scarlet Rot
- "Robustness" = odporność na Hemorrhage + Frostbite
- "Focus" = odporność na Sleep + Madness
- "Vitality" = odporność na Deathblight
- Ustawienie wartości na 0 = brak nagromadzenia efektu (bezpieczne)
- Gra recalkuluje progi przy załadowaniu — nadpisanie akumulatorów jest bezpieczne

---

### Imię postaci (0x94–0xB5)

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x94 | u16[16] | CharacterName | Imię postaci (UTF-16LE, max 16 znaków) |
| 0xB4 | u16 | NullTerminator | Terminator (0x0000) |

- Kodowanie: UTF-16 Little-Endian
- Maksimum: 16 znaków (32 bajty payload + 2B terminator = 34B)
- Nieużywane znaki: wypełnione 0x0000

---

### Kreacja postaci (0xB6–0xBF)

| Offset | Typ | Pole | Opis | Wartości |
|---|---|---|---|---|
| 0xB6 | u8 | Gender | Płeć/typ ciała | 0=Type B (female), 1=Type A (male) |
| 0xB7 | u8 | ArcheType | Klasa startowa | 0–9 (tabela poniżej) |
| 0xB8 | u8 | unk0xb8 | Nieznane (Appearance/VowType?) | |
| 0xB9 | u8 | unk0xb9 | Nieznane | |
| 0xBA | u8 | VoiceType | Typ głosu | 0=Young 1, 1=Young 2, 2=Mature 1, 3=Mature 2, 4=Aged 1, 5=Aged 2 |
| 0xBB | u8 | Gift | Starting Keepsake | 0–9 (tabela poniżej) |
| 0xBC | u8 | unk0xbc | Nieznane | |
| 0xBD | u8 | unk0xbd | Nieznane | |
| 0xBE | u8 | TalismanSlotCount | Dodatkowe sloty taliszmanów (odblokowane quest) | 0–2 |
| 0xBF | u8 | SummonSpiritLevel | Level ducha przywołania (Scadutree?) | |

---

### Unknown block (0xC0–0xD7)

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0xC0 | u8[24] | unk_block | Nieznany blok (0x18 bytes) — prawdopodobnie dodatkowe flagi stanu postaci |

---

### Ustawienia online (0xD8–0xF8)

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0xD8 | u8 | FurlcallingFingerRemedy | Furlcalling Finger Remedy aktywne (0=off, 1=on) |
| 0xD9 | u8 | unk0xd9 | Nieznane |
| 0xDA | u8 | MatchmakingWeaponLevel | Poziom broni do matchmakingu multiplayer |
| 0xDB | u8 | WhiteCipherRing | White Cipher Ring aktywne (0=off, 1=on) |
| 0xDC | u8 | BlueCipherRing | Blue Cipher Ring aktywne (0=off, 1=on) |
| 0xDD | u8[18] | unk0xdd | Nieznane (0x12 bytes) |
| 0xEF | u8 | ReinforceLv | Character reinforce level (wewnętrzny parametr) |
| 0xF0 | u8[7] | unk0xf0 | Nieznane |
| 0xF7 | u8 | GreatRuneActive | Great Rune aktywna (0=off, 1=on — wymaga Rune Arc) |
| 0xF8 | u8 | unk0xf8 | Nieznane |

**Matchmaking Weapon Level**:
- Gra śledzi najwyższy level broni jaki postać kiedykolwiek posiadała
- Wpływa na matchmaking multiplayer (dopasowanie do graczy z podobnym level broni)
- Zakres: 0–25 (normal weapons) lub 0–10 (special/somber weapons) → normalizowane do jednej skali

---

### Flask counts (0xF9–0x10F)

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0xF9 | u8 | MaxCrimsonFlask | Maks. ilość Crimson Tears (HP flask) — domyślnie 3-14 |
| 0xFA | u8 | MaxCeruleanFlask | Maks. ilość Cerulean Tears (FP flask) — domyślnie 0-14 |
| 0xFB | u8[21] | unk0xfb | Nieznane (0x15 bytes) — prawdopodobnie zawiera flask upgrade level, physick data |

**Uwagi o flaskach**:
- Łączna pula: Crimson + Cerulean = 14 (max, po zebraniu wszystkich Golden Seeds)
- Poziom wzmocnienia flask: określa ilość odzyskiwanego HP/FP (Sacred Tears)
- Podział jest zmienny — gracz ustawia proporcje w grace

---

### Hasła (0x110–0x17B)

Każde hasło: UTF-16LE, max 8 znaków + u16 terminator = 0x12 bytes (18 bytes)

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x110 | u16[8]+u16 | MultiplayerPassword | Hasło multiplayer (ogranicza matchmaking do grupy) |
| 0x122 | u16[8]+u16 | GroupPassword1 | Hasło grupy 1 (ułatwia widzenie znaków grupy) |
| 0x134 | u16[8]+u16 | GroupPassword2 | Hasło grupy 2 |
| 0x146 | u16[8]+u16 | GroupPassword3 | Hasło grupy 3 |
| 0x158 | u16[8]+u16 | GroupPassword4 | Hasło grupy 4 |
| 0x16A | u16[8]+u16 | GroupPassword5 | Hasło grupy 5 |

---

### SwordArt Point Scaling (0x17C–0x17F) — z CT, offset przybliżony

| Offset (CT) | Typ | Pole | Opis |
|---|---|---|---|
| ~0x17C | u8 | SwordArtPoint_ByStr | Skalowanie Ash of War od Strength |
| ~0x17D | u8 | SwordArtPoint_ByDex | Skalowanie od Dexterity |
| ~0x17E | u8 | SwordArtPoint_ByInt | Skalowanie od Intelligence |
| ~0x17F | u8 | SwordArtPoint_ByFaith | Skalowanie od Faith |

---

### Correction Stats — atrybuty dla respecu (offset z CT: 0x288)

Kopia atrybutów przechowywana dla mechanizmu respecu (Rennala). Powinna odpowiadać aktualnym wartościom.

| Pole | Opis |
|---|---|
| Vigor [For Correction] | Kopia Vigor dla przeliczenia |
| Mind [For Correction] | Kopia Mind |
| Endurance [For Correction] | Kopia Endurance |
| Strength [For Correction] | Kopia Strength |
| Dexterity [For Correction] | Kopia Dexterity |
| Intelligence [For Correction] | Kopia Intelligence |
| Faith [For Correction] | Kopia Faith |
| Arcane [For Correction] | Kopia Arcane |

**Uwaga**: Te pola mogą nie być w obrębie 432B PlayerGameData — mogą być dalej w slocie. Offset 0x288 (CT) sugeruje, że to osobna struktura po PlayerGameData (0x1B0 = 432 bytes kończy się przed 0x288).

---

### Padding (0x180–0x1AF)

| Offset | Typ | Opis |
|---|---|---|
| 0x180 | u8[48] | Nieznany trailing block (0x30 bytes) — do zbadania |

---

## Klasy postaci (Archetype) — pełna tabela z base stats

| ID | Klasa | Start Lvl | Vig | Mnd | End | Str | Dex | Int | Fai | Arc |
|---|---|---|---|---|---|---|---|---|---|---|
| 0 | Vagabond | 9 | 15 | 10 | 11 | 14 | 13 | 9 | 9 | 7 |
| 1 | Warrior | 8 | 11 | 12 | 11 | 10 | 16 | 10 | 8 | 9 |
| 2 | Hero | 7 | 14 | 9 | 12 | 16 | 9 | 7 | 8 | 11 |
| 3 | Bandit | 5 | 10 | 11 | 10 | 9 | 13 | 9 | 8 | 14 |
| 4 | Astrologer | 6 | 9 | 15 | 9 | 8 | 12 | 16 | 7 | 9 |
| 5 | Prophet | 7 | 10 | 14 | 8 | 11 | 10 | 7 | 16 | 10 |
| 6 | Confessor | 10 | 10 | 13 | 10 | 12 | 12 | 9 | 14 | 9 |
| 7 | Samurai | 9 | 12 | 11 | 13 | 12 | 15 | 9 | 8 | 8 |
| 8 | Prisoner | 9 | 11 | 12 | 11 | 11 | 14 | 14 | 6 | 9 |
| 9 | Wretch | 1 | 10 | 10 | 10 | 10 | 10 | 10 | 10 | 10 |

**Uwaga o numeracji**: CT Hexinton podaje Confessor=6, Samurai=7 — to odwrócone względem niektórych źródeł online. Potwierdzone przez oba CT.

---

## Starting Keepsake (Gift) — wartości

| ID | Gift | Opis |
|---|---|---|
| 0 | None | Brak prezentu |
| 1 | Crimson Amber Medallion | Talizman +HP |
| 2 | Lands Between Rune | Konsumable — runy |
| 3 | Golden Seed | Flask upgrade |
| 4 | Fanged Imp Ashes | Spirit summon |
| 5 | Cracked Pot | Crafting container |
| 6 | Stonesword Key ×2 | Klucze do imp statues |
| 7 | Bewitching Branch | NPC charm |
| 8 | Boiled Prawn | HP buff food |
| 9 | Shabriri's Woe | Aggro talizman |

---

## Great Rune Values (Item IDs z CT)

| Hex ID | Great Rune | Efekt (z Rune Arc) |
|---|---|---|
| 0x00000000 | None | — |
| 0xB00000BF | Godrick's Great Rune | +5 do wszystkich atrybutów |
| 0xB00000C0 | Radahn's Great Rune | +HP/FP/SP |
| 0xB00000C1 | Morgott's Great Rune | +Max HP |
| 0xB00000C2 | Rykard's Great Rune | HP recovery on kill |
| 0xB00000C3 | Mohg's Great Rune | Phantom bleed effect |
| 0xB00000C4 | Malenia's Great Rune | HP recovery on attack |

---

## Implikacje dla edycji

- **Zmiana atrybutów** wymaga też aktualizacji Level (formuła: sum - 79)
- **Max HP/FP/SP** — gra recalkuluje po załadowaniu na podstawie atrybutów; bezpieczne do nadpisania
- **Status buildups** — ustawienie na 0 = wyzerowanie nagromadzenia (bezpieczne)
- **Matchmaking Weapon Level** — zmiana wpływa na multiplayer; nie da się obniżyć w grze normalnie
- **Gender** — zmiana 0↔1 zmienia model postaci, ale Face Data pozostaje to samo
- **Class** — zmiana to tylko label; nie zmienia starting stats, ale wpływa na walidację respecu
- **Passwords** — zmiana/wyzerowanie = immediate effect w multiplayer
- **Great Rune** — musi odpowiadać posiadanej (event flag) i bycie aktywna wymaga GreatRuneActive=1
- **Correction Stats** — muszą być zsynchronizowane z aktualnymi atrybutami

---

## Źródła

- er-save-manager: `parser/character.py` — klasa `PlayerGameData` (linie 22-123, pełny read 124-200)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — PlayerGameData struct referenced
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — PlayerParam offsets, class reset scripts
- Cheat Engine: `ER_TGA_v1.9.0` — PlayerGameData structure, ChrAsm, EquipItemData
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam
