# 26 — Referencja Wszystkich Edytowalnych Parametrów

> **Zakres**: Kompletny opis każdego parametru w pliku save, który może być edytowany — z efektem w grze, dopuszczalnymi wartościami i caveats.

---

## 1. Atrybuty (Attributes)

### Vigor (Witalność)
- **Efekt**: Skaluje Max HP, drugorzędnie Fire Defense i Immunity (od lvl 31)
- **Zakres**: 1–99
- **Softcaps**: 40 (pierwszy), 60 (drugi)
- **Save offset**: PlayerGameData + 0x34 (u32)

| Vigor | HP | Vigor | HP |
|---|---|---|---|
| 1 | 300 | 40 | 1,450 |
| 10 | 414 | 50 | 1,704 |
| 15 | 522 | 60 | 1,900 |
| 20 | 652 | 70 | 1,959 |
| 25 | 800 | 80 | 2,015 |
| 30 | 994 | 90 | 2,065 |
| 35 | 1,216 | 99 | 2,100 |

### Mind (Umysł)
- **Efekt**: Skaluje Max FP; drugorzędnie Focus (Sleep/Madness resistance, od lvl 31)
- **NIE daje** Memory Slots (to robią Memory Stones)
- **Zakres**: 1–99
- **Softcaps**: 55–60
- **Save offset**: PlayerGameData + 0x38 (u32)

| Mind | FP | Mind | FP |
|---|---|---|---|
| 1 | 40 | 35 | 200 |
| 10 | 78 | 40 | 235 |
| 15 | 95 | 50 | 300 |
| 20 | 121 | 60 | 350 |
| 25 | 147 | 99 | 450 |
| 30 | 173 | — | — |

Praktyczny breakpoint: Mind 38 (221 FP) = pełny refill z max-level Cerulean Flask.

### Endurance (Wytrzymałość)
- **Efekt**: Skaluje Stamina + Equip Load; drugorzędnie Robustness (od lvl 31)
- **Zakres**: 1–99
- **Softcaps**: Stamina = 50, Equip Load = 60
- **Save offset**: PlayerGameData + 0x3C (u32)

| Endurance | Stamina | Equip Load |
|---|---|---|
| 1 | 80 | 45.0 |
| 20 | 113 | 64.1 |
| 30 | 130 | 77.6 |
| 40 | 142 | 90.9 |
| 50 | 155 | 105.2 |
| 60 | 158 | 120.0 |
| 99 | 170 | 160.0 |

### Strength (Siła)
- **Efekt**: Skalowanie obrażeń broni STR; Physical Defense
- **Special**: Two-handing mnoży efektywny STR × 1.5
- **Zakres**: 1–99
- **Softcaps**: 55 (pierwszy), 80 (drugi) dla skalowania broni
- **Save offset**: PlayerGameData + 0x40 (u32)

### Dexterity (Zręczność)
- **Efekt**: Skalowanie obrażeń broni DEX; casting speed; reduced fall damage
- **Zakres**: 1–99
- **Softcaps**: 55 (pierwszy), 80 (drugi)
- **Save offset**: PlayerGameData + 0x44 (u32)

### Intelligence (Inteligencja)
- **Efekt**: Skalowanie sorcery; Magic Defense; requirement dla sorceries
- **Zakres**: 1–99
- **Softcaps**: 60 (pierwszy), 80 (drugi)
- **Save offset**: PlayerGameData + 0x48 (u32)

### Faith (Wiara)
- **Efekt**: Skalowanie incantations; Holy/elemental Defense; requirement dla incantations
- **Zakres**: 1–99
- **Softcaps**: 60 (pierwszy), 80 (drugi)
- **Save offset**: PlayerGameData + 0x4C (u32)

### Arcane (Tajemność)
- **Efekt**: Status buildup (Bleed, Poison, Sleep); Item Discovery; Vitality (Deathblight resist); Holy Defense
- **Zakres**: 1–99
- **Softcaps**: 45 dla Item Discovery
- **Save offset**: PlayerGameData + 0x50 (u32)

---

## 2. Level i Runy

### Level
- **Formuła**: `Level = Vigor + Mind + Endurance + Strength + Dexterity + Intelligence + Faith + Arcane - 79`
- **Zakres**: 1–713
- **Save offset**: PlayerGameData + 0x60 (u32)
- **Caveat**: Musi odpowiadać sumie atrybutów. Gra nie waliduje przy ładowaniu ale matchmaking i respec będą broken.

### Runes (held)
- **Opis**: Runy aktualnie posiadane — utracone przy śmierci (odzyskiwalne z bloodstain)
- **Zakres**: 0–999,999,999 (u32 max)
- **Save offset**: PlayerGameData + 0x64 (u32)

### Total Get Soul (lifetime runes)
- **Opis**: Łączna liczba run zdobytych w historii postaci. Czysto statystyczna.
- **Save offset**: PlayerGameData + 0x68 (u32)

---

## 3. HP / FP / SP

### HP (current)
- **Opis**: Aktualne zdrowie w momencie zapisu
- **Zakres**: 1 – BaseMaxHP
- **Save offset**: PlayerGameData + 0x08 (u32)
- **Caveat**: Gra recalkuluje BaseMaxHP z Vigor po załadowaniu. Current HP powyżej max będzie clamped.

### MaxHP / BaseMaxHP
- **MaxHP**: Z taliszmanami/Great Rune (runtime calculation)
- **BaseMaxHP**: Z samego Vigor (patrz tabela Vigor)
- **Save offset**: 0x0C / 0x10 (u32)
- **Caveat**: Bezpieczne do nadpisania — gra recalkuluje.

### FP (current) / MaxFP / BaseMaxFP
- **Analogicznie do HP**, skaluje z Mind
- **Save offset**: 0x14 / 0x18 / 0x1C (u32)

### SP (Stamina current) / MaxSP / BaseMaxSP
- **Analogicznie**, skaluje z Endurance
- **Save offset**: 0x24 / 0x28 / 0x2C (u32)

---

## 4. Status Buildups

Akumulatory efektów statusu. Normalnie = 0. Gdy buildup >= próg → efekt się aktywuje.

| Pole | Offset | Chroni przed | Governing Attribute | Efekt po trigger |
|---|---|---|---|---|
| Immunity | 0x70 | Poison | Vigor (od 31) | HP drain ~90s |
| Immunity2 | 0x74 | Scarlet Rot | Vigor (od 31) | Szybszy HP drain |
| Robustness | 0x78 | Hemorrhage (Bleed) | Endurance (od 31) | Burst damage (% max HP) |
| Vitality | 0x7C | Deathblight | Arcane (od 31) | **Instant death** |
| Robustness2 | 0x80 | Frostbite | Endurance (od 31) | Burst + 20% vuln + stamina penalty (30s) |
| Focus | 0x84 | Sleep | Mind (od 31) | Immobilized kilka sekund |
| Focus2 | 0x88 | Madness | Mind (od 31) | FP drain + burst + stagger (only humanoids) |

**Edycja**: Ustawienie na 0 = brak nagromadzenia. Bezpieczne. Gra nie przechowuje "progu" tutaj — próg jest obliczany z atrybutów+armor.

---

## 5. Tożsamość postaci

### Character Name
- **Format**: UTF-16LE, max 16 znaków + null terminator
- **Save offset**: PlayerGameData + 0x94 (34 bytes)
- **Caveat**: Musi być zsynchronizowane z ProfileSummary w UserData10

### Gender (Body Type)
- **Wartości**: `0` = Type B (female model), `1` = Type A (male model)
- **Save offset**: PlayerGameData + 0xB6 (u8)
- **Caveat**: Zmiana modelu ciała — Face Data pozostaje to samo. Niektóre ubrania wyglądają inaczej.

### Archetype (Class)
- **Wartości**: 0=Vagabond, 1=Warrior, 2=Hero, 3=Bandit, 4=Astrologer, 5=Prophet, 6=Confessor, 7=Samurai, 8=Prisoner, 9=Wretch
- **Save offset**: PlayerGameData + 0xB7 (u8)
- **Efekt**: Tylko label + minimum attributes przy respec. NIE zmienia starting stats.
- **Caveat**: Zmiana klasy pozwala na respec do niższych minimów (np. Wretch min=10 all).

### Voice Type
- **Wartości**: 0=Young 1, 1=Young 2, 2=Mature 1, 3=Mature 2, 4=Aged 1, 5=Aged 2
- **Save offset**: PlayerGameData + 0xBA (u8)
- **Efekt**: Zmiana dźwięków gracza (krzyki, jęki, grunt przy rolling)

### Starting Gift (Keepsake)
- **Wartości**: 0=None, 1=Crimson Amber Medallion, 2=Lands Between Rune, 3=Golden Seed, 4=Fanged Imp Ashes, 5=Cracked Pot, 6=Stonesword Key ×2, 7=Bewitching Branch, 8=Boiled Prawn, 9=Shabriri's Woe
- **Save offset**: PlayerGameData + 0xBB (u8)
- **Efekt**: Czysto informacyjne po kreacji — item jest już w inventory.

---

## 6. Ustawienia Online

### Furlcalling Finger Remedy
- **Opis**: Aktywuje widoczność summon signs. Konsumable item — zużywa się.
- **Wartości**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xD8 (u8)

### White Cipher Ring
- **Opis**: Automatycznie prosi o pomoc gdy invaded
- **Wartości**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xDB (u8)

### Blue Cipher Ring
- **Opis**: Gracz jest automatycznie summonowany do pomocy invaded players
- **Wartości**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xDC (u8)

### Matchmaking Weapon Level
- **Opis**: Najwyższy level broni jaki postać kiedykolwiek posiadała. Permanentny. Nie da się obniżyć w grze.
- **Zakres**: 0–25 (normalizowany; Somber ×2.5 = Regular equivalent)
- **Save offset**: PlayerGameData + 0xDA (u8)
- **Efekt na matchmaking**:

| Host Weapon | Can Match Min | Can Match Max |
|---|---|---|
| +0 | +0 | +3 |
| +5 | +2 | +8 |
| +10 | +6 | +14 |
| +15 | +11 | +20 |
| +20 | +15 | +25 |
| +25 | +20 | +25 |

- **Caveat**: NPC-gifted upgraded weapons (np. Rogier's Rapier +8) permanentnie podnoszą bracket nawet bez equip.
- **Edycja**: Obniżenie pozwala na matchmaking z niżej-levelowymi graczami. Nie jest "nielegalne" ale jest wykrywalne.

### Great Rune Active
- **Opis**: Czy aktywna Great Rune działa (wymaga wcześniejszego użycia Rune Arc)
- **Wartości**: 0=inactive, 1=active
- **Save offset**: PlayerGameData + 0xF7 (u8)
- **Caveat**: Wymaga posiadania Great Rune (event flag) + equipped Great Rune (equipment slot)

---

## 7. Flask System

### Max Crimson Flask (HP)
- **Opis**: Maksymalna liczba Flask of Crimson Tears charges
- **Zakres**: 0–14
- **Save offset**: PlayerGameData + 0xF9 (u8)
- **Jak zwiększyć w grze**: Golden Seeds (30 total needed for max)

### Max Cerulean Flask (FP)
- **Opis**: Maksymalna liczba Flask of Cerulean Tears charges
- **Zakres**: 0–14
- **Save offset**: PlayerGameData + 0xFA (u8)
- **Constraint**: Crimson + Cerulean = max 14 (gra wymusza podział)

### Flask Upgrade Level
- **Opis**: Moc leczenia/regeneracji per use
- **Zakres**: +0 do +12
- **Save offset**: Nieznany dokładnie (prawdopodobnie w bloku 0xFB–0x10F)
- **Jak zwiększyć w grze**: Sacred Tears (12 total w grze)
- **Caveat**: Do zbadania — może być event flag a nie bezpośrednia wartość

---

## 8. Passwords

### Multiplayer Password
- **Opis**: Ogranicza matchmaking do graczy z tym samym hasłem
- **Format**: UTF-16LE, max 8 znaków + null
- **Save offset**: PlayerGameData + 0x110 (18 bytes)

### Group Passwords (1–5)
- **Opis**: Ułatwia widzenie summon signs grupy (żółte znaki)
- **Format**: jak wyżej
- **Save offset**: 0x122, 0x134, 0x146, 0x158, 0x16A (18 bytes each)
- **Caveat**: Max 5 grup aktywnych jednocześnie

---

## 9. DLC Blessings

### Scadutree Blessing
- **Opis**: Zwiększa obrażenia + damage negation gracza **tylko w Realm of Shadow (DLC)**
- **Zakres**: 0–20
- **Save offset**: Zależy od implementacji (u8, ~-187 od MagicOffset)
- **Efekt per level**: ~5% attack + ~5% negation (diminishing po lvl 12)
- **Max (lvl 20)**: +80% attack, -40% damage received
- **Collectible**: 50 Scadutree Fragments (2-3 per level)
- **Brak efektu** w base game areas (Lands Between)

### Shadow Realm Blessing (Revered Spirit Ash)
- **Opis**: Zwiększa obrażenia + negation dla summoned spirits i Torrent **w DLC**
- **Zakres**: 0–10
- **Save offset**: u8, ~-186 od MagicOffset
- **Efekt (max lvl 10)**: Spirits deal 1.75x damage, take 0.625x
- **Collectible**: 25 Revered Spirit Ashes
- **Exception**: Mimic Tear nie otrzymuje pełnych bonusów (nerf w patch 1.13)

---

## 10. Talisman Slots

### Additional Talisman Slot Count
- **Opis**: Ile dodatkowych slotów taliszmanów odblokowano (poza bazowym 1)
- **Zakres**: 0–3 (total slots = 1 + value = max 4)
- **Save offset**: PlayerGameData + 0xBE (u8)
- **Jak odblokować w grze**:
  1. Pokonaj Margit → Talisman Pouch (slot 2)
  2. Pokonaj 2 Shardbearers + Enia → Talisman Pouch (slot 3)
  3. Pokonaj Godfrey, First Elden Lord → Talisman Pouch (slot 4)
- **Caveat**: Wymaga też odpowiednich event flags. Sam parametr bez flag → gra może zignorować.

---

## 11. Equipment — Edytowalne pola

### ArmStyle (Weapon Stance)
- **Wartości**: 0=EmptyHand, 1=OneHand, 2=LeftBothHand (2H left), 3=RightBothHand (2H right)
- **Save offset**: ActiveWeaponSlots + 0x00 (u32)
- **Caveat**: Musi być konsystentne z equipped weapons. Powerstance nie ma osobnej wartości — aktywuje się automatycznie gdy dwie bronie tego samego typu w obu rękach.
- **Crash risk**: Nieprawidłowa wartość (>3) może crashować grę

### Active Weapon Slots
- **Wartości**: 0=Primary, 1=Secondary, 2=Tertiary
- **Save offset**: ActiveWeaponSlots + 0x04–0x18 (7 × u32)
- **Caveat**: Wartość poza 0–2 = undefined behavior

---

## 12. Torrent (Koń)

### State
- **Wartości**: 1=INACTIVE (nie wezwany), 3=DEAD, 13=ACTIVE (jedziemy)
- **Save offset**: RideGameData + 0x24 (u32)
- **BUG**: HP=0 + State=13 = **infinite loading screen** (znany bug save corruption)
- **Fix**: Gdy HP=0, ustaw State=3

### HP
- **Opis**: Zdrowie Torrenta — skaluje z player level
- **Save offset**: RideGameData + 0x20 (i32)
- **Caveat**: Wartość powyżej oczekiwanej dla danego levelu prawdopodobnie clamped przez grę

### Coordinates / Map ID / Angle
- **Opis**: Ostatnia pozycja Torrenta
- **Save offset**: RideGameData + 0x00 (f32×3 pos, u8[4] map, f32×4 quat)
- **Edycja**: Bezpieczna — Torrent teleportuje się do gracza gdy wezwany

---

## 13. Blood Stain (Bloodstain)

### Runes (recoverable)
- **Opis**: Runy utracone przy ostatniej śmierci, możliwe do odzyskania z bloodstain
- **Save offset**: BloodStain + 0x34 (i32)
- **Edycja**: Zmiana wartości = zmiana run do odzyskania. Ustawienie 0 = brak bloodstain.

### Coordinates / Map ID
- **Opis**: Pozycja bloodstain w świecie
- **Save offset**: BloodStain + 0x00 (f32×3 + f32×4 quat + u8[4] map)
- **Edycja**: Zmiana pozycji = bloodstain w nowym miejscu. Uwaga na out-of-bounds.

---

## 14. Player Coordinates (Teleportacja)

### Position (X, Y, Z)
- **Save offset**: PlayerCoordinates + 0x00 (3 × f32)
- **Edycja**: Bezpośrednia teleportacja gracza! Ale wymaga prawidłowego Map ID.

### Map ID (4 bytes)
- **Format**: [region_type, Y_grid, X_grid, layer]
- **Layer**: 0x3C = overworld, 0x3D+ = underground
- **Save offset**: PlayerCoordinates + 0x0C (u8[4])
- **Caveat**: Zły MapID + pozycja = spawn w pustce → falling → death → respawn at last grace

### Rotation (Quaternion)
- **Save offset**: PlayerCoordinates + 0x10 (4 × f32)
- **Edycja**: Kierunek patrzenia po załadowaniu (mało istotne)

---

## 15. Game State

### Death Count
- **Opis**: Łączna liczba śmierci postaci
- **Save offset**: Game State sekcja 7 + 0x00 (u32)
- **Edycja**: Czysto kosmetyczne — reset do 0 nie wpływa na gameplay

### NG+ Cycle (ClearCount)
- **Opis**: Numer podróży (Journey)
- **Wartości**: 0=Journey 1 (first play), 1=NG+1, ..., 7=NG+7 (max)
- **Save offset**: Game State sekcja 1 + 0x00 (u32)
- **Efekt**:
  - Enemies scaling: NG+1 = 3–3.4× HP (early areas); NG+7 = ~1.45× over NG+1
  - Rune rewards: NG+1 = 5.5× (early); diminishing po NG+2
  - NG+7 = cap, dalsze cykle identyczne
- **Caveat**: Zmiana z 0 na N nie resetuje quest flags/items — postać będzie w Journey N+1 z aktualnym progressem

### Last Rested Grace
- **Opis**: Grace entity ID (BonfireId) ostatniego odpoczynku
- **Save offset**: Game State sekcja 7 + 0x10 (u32)
- **Efekt**: Spawn point po śmierci i po załadowaniu save
- **Edycja**: Zmiana = teleportacja gracza do innego grace!
- **Caveat**: Wartość musi być prawidłowym BonfireId (patrz spec/15 tabela Bonfire IDs)

### Play Time
- **Opis**: Czas gry w milisekundach
- **Save offset**: Osobna sekcja (IngameTimer)
- **Edycja**: Kosmetyczne — zmiana czasu wyświetlanego w menu

---

## 16. Weather & Time

### WorldAreaTime
- **Hour**: 0–23 (u32)
- **Minute**: 0–59 (u32)
- **Second**: 0–59 (u32)
- **Save offset**: WorldAreaTime sekcja (3 × u32 = 12B)
- **Efekt**: Pora dnia po załadowaniu. Wpływa na: NPC spawny (Night's Cavalry, Bell Bearing Hunter), oświetlenie, ambient.

### WorldAreaWeather
- **Area ID**: u16 — identyfikator regionu
- **Weather Type**: u16 — typ pogody
- **Timer**: u32 — czas trwania
- **Save offset**: WorldAreaWeather sekcja (12B)
- **Efekt**: Pogoda w momencie załadowania

---

## 17. DLC Flags

### Shadow of the Erdtree Entry Flag
- **Opis**: Czy postać weszła do DLC (Realm of Shadow)
- **Wartości**: 0=nie wchodzono, 1=wchodzono
- **Save offset**: DLC sekcja + 0x01 (u8)
- **Efekt**: Jednorazowa — po wejściu nie da się cofnąć w grze. Edycja pozwala reset.
- **KRYTYCZNE**: Przy konwersji platform ten bajt MUSI być wyzerowany jeśli DLC nie jest zainstalowane → infinite loading

### Pre-order Gestures
- **DLC[0]**: "The Ring" gesture (0=nie, 1=tak)
- **DLC[2]**: "Ring of Miquella" gesture (0=nie, 1=tak)
- **KRYTYCZNE**: Bajty 3–49 MUSZĄ być 0x00. Niezerowe = save odrzucony.

---

## 18. Unlocked Regions (Fog of War)

### Region IDs
- **Opis**: Lista odblokowanych regionów mapy (usunięcie Fog of War)
- **Format**: u32 count + count × u32 region_id
- **Edycja**: Dodanie region ID = usunięcie mgły w tym obszarze
- **UWAGA**: ZMIENNA DŁUGOŚĆ — zmiana count przesuwa wszystkie następne sekcje!
- **Pełna lista**: ~200+ region IDs (AllRegionIDs w db/data/maps.go)

---

## 19. Event Flags — najważniejsze edytowalne

### Boss Defeat Flags
- **Efekt**: Oznacza bossa jako pokonanego. Otwiera nowe areas, NPC reactions.
- **Zakres**: 9100–9281 (global bosses), 10000000+ (field bosses)
- **Caveat**: Ustawienie flagi bossa bez questowych → NPC mogą być zdezorientowani

### Grace Discovery Flags
- **Efekt**: Odkrycie Site of Grace — pozwala na fast travel
- **Zakres**: 71xxx, 73xxx, 76xxx
- **Caveat**: Odkrycie grace bez pokonania bossa blokującego dostęp = normalne

### Map Visibility / Acquisition
- **Visibility (62xxx)**: Mapa widoczna (tekstura regionu wyświetlona)
- **Acquired (63xxx)**: Fragment mapy "podniesiony" — powinien też dodać item do inventory
- **Caveat**: Visibility bez Acquired = mapa widoczna ale fragment nie w inventory (kosmetycznie OK)

### Mechanic Unlocks (60xxx)
- **Edycja**: Odblokowanie mechanik bez questów (Torrent, crafting, etc.)
- **Bezpieczne**: Gra nie sprawdza "jak" mechanika została odblokowana

### Cookbook Flags (67xxx–68xxx)
- **Efekt**: Odblokowanie crafting recipes
- **Alternatywa**: Dodanie cookbook item do inventory (to jest ta sama rzecz — posiadanie item = flag set)

---

## 20. Inventory & Storage — pola per item

### Quantity
- **Opis**: Ilość stackable items (weapons/armor = zawsze 1)
- **Zakres**: 1 – MaxInventory/MaxStorage (per item, z bazy danych)
- **Save offset**: InventoryItem + 0x04 (u32)
- **Caveat**: Przekroczenie max = gra clampuje do max przy użyciu

### GaItem Handle
- **Opis**: Referencja do instancji przedmiotu w GaItem Map
- **Format**: 0xTTPPCCCC (TT=type, PP=part, CCCC=counter)
- **Caveat**: Handle MUSI istnieć w GaItem Map. Orphaned handle = item nie wyświetla się.

---

## 21. Face Data — edytowalne grupy

### Model IDs (zmiana fryzury/zarostu/brwi bez kreatora)
- **Hair_Model_Id**: Zmiana fryzury (u32, wartości z game params)
- **Beard_Model_Id**: Zmiana zarostu
- **Bezpieczne**: Zmiana modelu nie wpływa na stabilność

### Shape Parameters (suwaki kreatora)
- **Zakres**: 0–255 (u8), 128 = neutralne/środkowe
- **Bezpieczne**: Dowolna wartość w zakresie jest prawidłowa

### Colors (RGBA)
- **Zakres**: 0–255 per kanał
- **Bezpieczne**: Dowolna kombinacja jest prawidłowa

### Body Scale
- **Opis**: Proporcje ciała (Head, Chest, Abdomen, Arms, Legs)
- **Format w pamięci**: float (1.0 = normal)
- **Format w save**: Do weryfikacji (może u8 z 128=1.0)

---

## 22. Memory Slots & Spell System

### Memory Slot Count
- **Baza**: 2 sloty (wszyscy)
- **Memory Stones**: +8 (8 findable w grze)
- **Moon of Nokstella** (talizman): +2 (equipment, nie save field)
- **Max**: 12 slotów
- **Tracking**: Memory Stones to key items w inventory. Posiadanie = odblokowanie.

### Equipped Spells (14 slotów)
- **Format**: SpellID (u32) + Quantity (u32), stride 8B
- **Edycja**: Zmiana SpellID = natychmiastowa zmiana zaklęcia
- **Caveat**: Gra pozwala equip max N slotów (N = memory slots). Ale fizycznie 14 slotów istnieje.

---

## 23. Weapon & Spirit Ash Upgrade System

### Normal Weapons
- **Zakres**: +0 do +25
- **Materials**: Smithing Stones [1]–[8] + Ancient Dragon Smithing Stone (+25)
- **ID encoding**: baseID + upgrade_level (np. Uchigatana = 1000000, +5 = 1000005)
- **Infusions**: Obsługiwane (Standard, Heavy, Keen, Quality, Magic, Cold, Fire, Flame Art, Lightning, Sacred, Poison, Blood, Occult)
- **Infusion encoding**: baseID + infusion_offset + upgrade_level

### Somber Weapons
- **Zakres**: +0 do +10
- **Materials**: Somber Smithing Stones [1]–[9] + Somber Ancient Dragon Smithing Stone (+10)
- **Brak infuzji** — unique weapons z fixed skills
- **ID encoding**: baseID + upgrade_level

### Spirit Ashes
- **Zakres**: +0 do +10
- **Materials**: Grave Glovewort [1]–[9] + Great Grave Glovewort (regular) LUB Ghost Glovewort [1]–[9] + Great Ghost Glovewort (legendary)
- **ID encoding**: baseID + upgrade_level (jak bronie)

---

## 24. Summoning Pools & Colosseums

### Summoning Pools (Martyr Effigies)
- **Łącznie**: ~165 w grze (base + DLC)
- **Tracking**: Event flags (10000000+ range)
- **Edycja**: Set flag = activated

### Colosseums
- **Łącznie**: 3 (Caelid=60350, Limgrave=60360, Royal/Leyndell=60370)
- **Edycja**: Set event flag = unlocked
- **Added**: Patch 1.08 (December 2022)

---

## 25. Parametry NIEZNANE (do zbadania)

| Offset | Opis | Hipoteza |
|---|---|---|
| PlayerGameData 0x00–0x07 | 2 × u32 unknown | PlayerNo / internal ID? Runtime only? |
| PlayerGameData 0x20 | u32 between FP/SP | FP regen? MaxMP2? |
| PlayerGameData 0x30 | u32 after SP | SP regen? |
| PlayerGameData 0x54–0x5C | 3 × u32 after Arcane | Atrybuty DLC? Unused extension? |
| PlayerGameData 0x6C | u32 after TotalGetSoul | Runes lost at death (recoverable)? |
| PlayerGameData 0x8C–0x90 | 2 × u32 after Madness | DLC buildups? Reserved? |
| PlayerGameData 0xB8–0xB9 | 2 × u8 in creation | Appearance/VowType? |
| PlayerGameData 0xBC–0xBD | 2 × u8 in creation | Related to starting equipment? |
| PlayerGameData 0xC0–0xD7 | 24 bytes unknown | Character flags? Online state? |
| PlayerGameData 0xDD–0xF6 | 26 bytes in online | Additional online settings? |
| PlayerGameData 0xFB–0x10F | 21 bytes after flask | Flask upgrade level? Physick data? |
| PlayerGameData 0x180–0x1AF | 48 bytes trailing | SwordArt scaling? Correction stats beginning? |

---

## Źródła

- Elden Ring Wiki (Fextralife): https://eldenring.wiki.fextralife.com/
- Game8 Elden Ring: https://game8.co/games/Elden-Ring/
- Souls Modding Wiki: https://www.soulsmodding.com/
- Cheat Engine tables: ER_all-in-one_Hexinton_v3.10, ER_TGA_v1.9.0
- er-save-manager (Python parser): sequential field order reference
- Community spreadsheets: event flag mappings, param tables
