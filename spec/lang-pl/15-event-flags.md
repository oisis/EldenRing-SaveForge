# 15 — Event Flags (Flagi Zdarzeń)

> **Zakres**: Główny mechanizm progresji gry — 1.8 MB flag bitowych sterujących stanem świata, questami, bossami, odkryciami.

---

## Opis ogólny

Event Flags to tablica 0x1BF99F bajtów (1,833,375 bytes / ~1.75 MB) z flagami bitowymi. Każda flaga to pojedynczy bit kontrolujący jeden aspekt stanu gry:
- Boss pokonany
- NPC quest stage
- Przedmiot podniesiony
- Mapa odkryta
- Cutscena obejrzana
- Mechanika odblokowana
- i tysiące innych

Po tablicy flag następuje 4-bajtowy **terminator**.

---

## Adresowanie flag — BST Lookup

Flagi nie są mapowane liniowo (flag_id ≠ byte_offset × 8 + bit). Zamiast tego używany jest **Binary Search Tree** (BST) do konwersji:

### Algorytm: Event ID → (byte_offset, bit_index)

```
BLOCK_SIZE = 125 bytes
FLAG_DIVISOR = 1000

1. block = event_id / 1000  (integer division)
2. index = event_id - (block × 1000)
3. offset = BST_LOOKUP[block] × 125     ← z eventflag_bst.txt
4. byte_index = index / 8
5. bit_index = 7 - (index - byte_index × 8)    ← BIG-ENDIAN bit order!
6. final_byte_pos = offset + byte_index
7. flag_value = (event_flags[final_byte_pos] >> bit_index) & 1
```

### BST Lookup Table

Plik `eventflag_bst.txt` zawiera 11,919 wpisów w formacie `block,offset`:
- `block` = event_id // 1000
- `offset` = numer bloku 125-bajtowego w tablicy event_flags

---

## Ustawianie flagi

```
byte_pos = BST_LOOKUP[block] × 125 + (index / 8)
bit_pos = 7 - (index % 8)

SET:   event_flags[byte_pos] |= (1 << bit_pos)
CLEAR: event_flags[byte_pos] &= ~(1 << bit_pos)
```

---

## Kategorie flag — szczegółowe ID

### Progresja globalna (0–999)

| ID | Opis |
|---|---|
| 20–24 | Game Clear (Normal, Ranni, Frenzied Flame endings) |
| 30 | Przejście do kolejnego cyklu NG+ |
| 50–58 | Śledzenie ukończonych cykli NG+ |
| 180–197 | Great Rune possession tracking |
| 300–575 | Stan świata (Erdtree, meteoryty, grawitacja) |

### Mechaniki — Core Unlocks (60000–60999)

| ID | Mechanika | Jak odblokować w grze |
|---|---|---|
| 60020 | Flask of Wondrous Physick | Trzecia Kościelna Ruina |
| 60100 | Torrent Whistle (Spectral Steed Ring) | Spotkanie Meliny w Grace |
| 60110 | Spirit Calling Bell | Ranni w Church of Elleh (noc) |
| 60120 | Crafting Kit | Kupno od Kale (Merchant) |
| 60130 | Whetstone Knife | Loot w Gatefront Ruins |
| 60140 | Tailoring Tools | Loot w Coastal Cave |
| 60150 | Golden Tailoring Tools | Boss drop (Godfrey's shade?) |

### Mechaniki — Whetblades (affinity unlock)

| ID | Whetblade | Affinity odblokowana |
|---|---|---|
| 65610 | Iron Whetblade | Heavy, Keen, Quality |
| 65640 | Glintstone Whetblade | Magic, Cold |
| 65660 | Red-Hot Whetblade | Fire, Flame Art |
| 65680 | Sanctified Whetblade | Lightning, Sacred |
| 65720 | Black Whetblade | Poison, Blood, Occult |

### Mechaniki — pozostałe (60200–60849)

| Zakres | Opis |
|---|---|
| 60200–60300 | Multiplayer features (signs, invasions, items) |
| 60400–60590 | Memory slots i talisman slot unlocks |
| 60800–60849 | Gesture unlocks |

### Bossy (61000–61999)

| ID | Opis |
|---|---|
| 61100–61135 | Major bosses (Margit, Godrick, Maliketh, Malenia, etc.) |
| 61200–61220 | Catacomb bosses |
| 61230–61248 | Cave bosses |
| 61260–61268 | Mine bosses |

### Mapa — Visibility (62000–62065)

Map visibility flags — kontrolują widoczność tekstury regionu na mapie.

| ID | Region | Obszar |
|---|---|---|
| 62010 | Limgrave West | Zachodnia część Limgrave |
| 62011 | Limgrave East | Wschodnia część Limgrave |
| 62012 | Weeping Peninsula | Weeping Peninsula |
| 62020 | Liurnia South | Południowa Liurnia |
| 62021 | Liurnia North | Północna Liurnia |
| 62022 | Liurnia East | Wschodnia Liurnia |
| 62030 | Altus Plateau | Altus Plateau |
| 62031 | Leyndell | Leyndell Royal Capital |
| 62032 | Mt. Gelmir | Mt. Gelmir |
| 62040 | Caelid South | Południowy Caelid |
| 62041 | Caelid North (Dragonbarrow) | Północny Caelid / Dragonbarrow |
| 62050 | Mountaintops West | Zachodnie Mountaintops |
| 62051 | Mountaintops East | Wschodnie Mountaintops |
| 62052 | Consecrated Snowfield | Consecrated Snowfield |
| 62060 | Siofra River | Siofra River (underground) |
| 62061 | Ainsel River | Ainsel River (underground) |
| 62062 | Deeproot Depths | Deeproot Depths |
| 62063 | Lake of Rot | Lake of Rot |
| 62064 | Mohgwyn Palace | Mohgwyn Palace |
| 82001 | Shadow of the Erdtree (DLC) | Realm of Shadow |

### Mapa — Fragment Acquisition (63000–63065)

Map fragment pickup flags — czy gracz podniósł fragment mapy.

| ID | Fragment | Lokalizacja |
|---|---|---|
| 63010 | Limgrave West Map | Gatefront |
| 63011 | Limgrave East Map | Waypoint Ruins area |
| 63012 | Weeping Peninsula Map | Castle Morne approach |
| 63020 | Liurnia South Map | Lake-Facing Cliffs |
| 63021 | Liurnia North Map | Academy Gate area |
| 63022 | Liurnia East Map | Eastern Liurnia |
| 63030 | Altus Plateau Map | Forest Spanning Greatbridge |
| 63031 | Leyndell Map | Capital Outskirts |
| 63032 | Mt. Gelmir Map | Road of Iniquity |
| 63040 | Caelid Map | Caelid Highway |
| 63041 | Dragonbarrow Map | Dragonbarrow |
| 63050 | Mountaintops West Map | Giants area |
| 63051 | Mountaintops East Map | Fire Giant area |
| 63052 | Consecrated Snowfield Map | Hidden Path |
| 63060 | Siofra River Map | Underground |
| 63061 | Ainsel River Map | Underground |
| 63062 | Deeproot Depths Map | Underground |
| 63063 | Lake of Rot Map | Underground |
| 63064 | Mohgwyn Palace Map | Underground |

### Cookbooks (67000–68500)

| Zakres | Cookbook Type | Ilość |
|---|---|---|
| 67000–67910 | Nomadic Warrior's Cookbook | ~10 entries |
| 67200–67300 | Armorer's Cookbook | ~7 entries |
| 67400–67480 | Glintstone Craftsman's Cookbook | ~5 entries |
| 67600–67700 | Missionary's Cookbook | ~5 entries |
| 67840–67920 | Perfumer's Cookbook | ~4 entries |
| 68000–68030 | Ancient Dragon Apostle's Cookbook | ~3 entries |
| 68200–68230 | Fevor's Cookbook | ~3 entries |
| 68400–68410 | Frenzied's Cookbook | ~2 entries |

### Przedmioty (65000–68999)

| Zakres | Opis |
|---|---|
| 65600–65790 | Ash of War affinity unlocks |
| 65810–65901 | Skill stone possession |
| 67000–68500 | Cookbook/recipe unlocks (powyżej szczegóły) |

### Graces — Event Flag IDs

| Flag ID | Grace | Lokalizacja |
|---|---|---|
| 71000 | Godrick the Grafted | Stormveil Castle (post-boss) |
| 71001 | Margit, the Fell Omen | Stormveil approach |
| 71190 | Table of Lost Grace | Roundtable Hold |
| 71800 | Cave of Knowledge | Tutorial area |
| 73xxx | Catacomb/Cave/Tunnel graces | Dungeons |
| 76100 | Church of Elleh | Limgrave |
| 76101 | The First Step | Limgrave (start) |
| 76111 | Gatefront | Limgrave |

### Graces — Byte Offsets w bitfieldzie (potwierdzone z CT)

Offset od bazy EventFlags (`[EventFlagMan]+0x28` w pamięci):

| Grace | Byte Offset | Bit | Flag ID |
|---|---|---|---|
| Table of Lost Grace / Roundtable Hold | +0xA58 | 1 | 71190 |
| The First Step | +0xCBE | 2 | 76101 |
| Church of Elleh | +0xCBE | 3 | 76100 |
| Gatefront | +0xCBF | 0 | 76111 |
| Stormhill Shack | +0xCBE | 1 | — |
| Castleward Tunnel | +0xA41 | 5 | — |
| Margit, the Fell Omen | +0xA41 | 6 | 71001 |
| Warmaster's Shack | +0xCC0 | 1 | — |
| Cave of Knowledge | +0xAA5 | 7 | 71800 |
| Stranded Graveyard | +0xAA5 | 6 | — |

### Mapy lokacyjne (30000–60999)

| Prefiks | Opis |
|---|---|
| 30xxx | Catacomb zone flags |
| 31xxx | Cave system flags |
| 32xxx | Mine network flags |
| 60xxx | Fortress/camp flags |

### Tutorial/Debug (710000+)

| ID | Opis |
|---|---|
| 710000–720200 | Tutorial completion tracking |
| 780000–780090 | Cinematic context flags |
| 9990–9999 | Developer test flags |

---

## Bonfire IDs (Grace Entity IDs — do teleportacji)

Bonfire IDs to **osobne identyfikatory** od Event Flag IDs. Używane w GameMan do teleportacji (Last Grace, Target Grace):

| BonfireId | Grace | Format |
|---|---|---|
| 1042362951 | The First Step | 10AABBCCCC |
| 10002951 | Margit, the Fell Omen | |
| 11052950–55 | Leyndell/Capital | |
| 12012950–71 | Underground (Ainsel, Siofra, Nokron) | |
| 13002950–60 | Crumbling Farum Azula | |
| 14002950–53 | Academy of Raya Lucaria | |
| 15002950–58 | Haligtree | |
| 16002950–64 | Volcano Manor | |
| 18002950–51 | Tutorial area | |
| 19002950 | Fractured Marika | |
| 20xxxxxxx | DLC (Shadow of the Erdtree) | |

---

## Znane problemy / soft-locks naprawialne przez edycję flag

1. **Ranni's Tower quest soft-lock** — korygowalna przez reset specyficznych flag questowych
2. **Warp sickness** (Radahn, Morgott, Radagon, Sealing Tree) — naprawialna edycją flag
3. **Niekompatybilne kombinacje flag** — np. boss killed + quest stage = before boss → NPC confused

---

## Implikacje dla edycji

- Zmiana flag NIE zmienia rozmiaru sekcji (stała 0x1BF99F)
- Nie wymaga przesuwania innych sekcji
- Wymaga BST lookup — nie da się adresować flag bez `eventflag_bst.txt`
- Ustawianie flagi bossa bez ustawienia powiązanych flag questowych może powodować soft-locks
- **Map visibility** (62xxx) — ustawienie = mapa widoczna nawet bez fragmentu
- **Map acquired** (63xxx) — ustawienie = fragment "podniesiony"; powinno też dodać item do inventory
- **Cookbook flags** — ustawienie = crafting recipes odblokowane
- **Whetblade flags** — ustawienie = nowe affinity dostępne w crafting
- Pełna lista flag: https://soulsmods.github.io/elden-ring-eventparam/
- Spreadsheet: https://docs.google.com/spreadsheets/d/1Nn-d4_mzEtGUSQXscCkQ41AhtqO_wF2Aw3yoTBdW9lk

---

## Źródła

- er-save-manager: `parser/event_flags.py` — klasa `EventFlags` (pełny algorytm BST)
- er-save-manager: `src/resources/eventflag_bst.txt` — 11,919 entries BST mapping
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` linie 197-223 — EventFlags (0x1bf99f bytes)
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Grace byte offsets, mechanic flags, map discovery
- Cheat Engine: `ER_TGA_v1.9.0` — Event flag categories, flag IDs, grace/boss/NPC references
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:event-flag-list
- Event Flags GitHub Pages: https://soulsmods.github.io/elden-ring-eventparam/
- TGA CE Table: https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA
