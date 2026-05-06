# Elden Ring Save File Format — Specyfikacja

> **Cel**: Kompletna dokumentacja formatu binarnego pliku zapisu Elden Ring (.sl2 / memory.dat).
> Wystarczająca do implementacji edytora save od zera bez dostępu do kodu źródłowego gry.
>
> **Stan**: W trakcie tworzenia. Każda sekcja jest weryfikowana binarnie na prawdziwych plikach save.

---

## Platformy

| Platforma | Plik | Szyfrowanie | Checksumy |
|---|---|---|---|
| PC (Steam) | `ER0000.sl2` | AES-128-CBC (opcjonalne) | MD5 per slot |
| PS4 | `memory.dat` | Brak | Brak |
| PS5 | `memory.dat` | Brak | Brak |

---

## Struktura pliku — przegląd

Plik save składa się z następujących głównych bloków (w kolejności sekwencyjnej):

```
┌─────────────────────────────────────────────┐
│  HEADER (platforma-specyficzny)             │  → 01-header.md
├─────────────────────────────────────────────┤
│  SLOT 0: Character Save Data                │
│    ├── Slot Header                          │
│    ├── GaItem Map (mapa przedmiotów)        │  → 03-gaitem-map.md
│    ├── PlayerGameData (statystyki)          │  → 04-player-game-data.md
│    ├── SP Effects (efekty statusu)          │  → 05-sp-effects.md
│    ├── Equipment (ekwipunek)                │  → 06-equipment.md
│    ├── Inventory (inwentarz)                │  → 07-inventory.md
│    ├── Spells & Gestures (zaklęcia/gesty)   │  → 08-spells-gestures.md
│    ├── Face Data (kreator postaci)          │  → 09-face-data.md
│    ├── Storage Box (skrzynia)               │  → 10-storage.md
│    ├── Regions (odblokowane regiony)        │  → 11-regions.md
│    ├── Torrent (koń)                        │  → 12-torrent.md
│    ├── Blood Stain (plama krwi)             │  → 13-blood-stain.md
│    ├── Game State (stan gry)                │  → 14-game-state.md
│    ├── Event Flags (flagi zdarzeń)          │  → 15-event-flags.md
│    ├── World State (stan świata)            │  → 16-world-state.md
│    ├── Player Coordinates (pozycja)         │  → 17-player-coordinates.md
│    ├── Network Manager                      │  → 18-network.md
│    ├── Weather & Time (pogoda/czas)         │  → 19-weather-time.md
│    ├── Version & Platform Data              │  → 20-version-platform.md
│    ├── DLC                                  │  → 21-dlc.md
│    └── Player Data Hash                     │  → 22-player-hash.md
│  SLOT 1-9: (identyczna struktura)           │
├─────────────────────────────────────────────┤
│  USER_DATA_10 (profil konta)                │  → 23-user-data-10.md
├─────────────────────────────────────────────┤
│  USER_DATA_11 (regulation.bin)              │  → 24-user-data-11.md
└─────────────────────────────────────────────┘
```

---

## Sekcje dokumentacji

| # | Plik | Opis |
|---|---|---|
| 01 | [Header i layout pliku](01-header.md) | Magic bytes, detekcja platformy, BND4 container, offsety slotów, checksumy MD5 |
| 02 | [Slot — struktura ogólna](02-slot-structure.md) | Rozmiar slotu, wersja, sekwencyjny parsing, sekcje zmiennej długości |
| 03 | [GaItem Map](03-gaitem-map.md) | Mapa handle→itemID, typy przedmiotów, rozmiary rekordów per typ |
| 04 | [PlayerGameData](04-player-game-data.md) | Statystyki postaci: HP/FP/SP, atrybuty, level, runy, imię, płeć, klasa, hasła, ustawienia online |
| 05 | [SP Effects](05-sp-effects.md) | Aktywne efekty statusu (buffy/debuffy), buildup values |
| 06 | [Equipment](06-equipment.md) | Ekwipunek: 22 sloty (bronie, zbroja, talizmany), active weapon slots, arm style |
| 07 | [Inventory](07-inventory.md) | Inwentarz postaci: common items + key items, format rekordu, indeksy |
| 08 | [Spells & Gestures](08-spells-gestures.md) | Zapamiętane zaklęcia, gesty, pociski |
| 09 | [Face Data](09-face-data.md) | Kreator postaci — parametry wyglądu (303 bajty) |
| 10 | [Storage Box](10-storage.md) | Skrzynia w grace: common + key items, countery |
| 11 | [Regions](11-regions.md) | Lista `unlocked_regions` (fast travel) — format binarny + `core.SetUnlockedRegions` |
| 12 | [Torrent](12-torrent.md) | Dane konia: pozycja, HP, stan (żywy/martwy/nieaktywny) |
| 13 | [Blood Stain](13-blood-stain.md) | Plama krwi po śmierci: pozycja, runy, mapa |
| 14 | [Game State](14-game-state.md) | Menu profile, tutorial data, GameMan bytes, death count, character type, last grace |
| 15 | [Event Flags](15-event-flags.md) | 1.8 MB flag bitowych: progresja, bossy, questy, przedmioty, mapa — BST lookup |
| 16 | [World State](16-world-state.md) | FieldArea, WorldArea, WorldGeomMan, RendMan — geometria i stan świata |
| 17 | [Player Coordinates](17-player-coordinates.md) | Pozycja gracza 3D + mapa + kąt obrotu |
| 18 | [Network Manager](18-network.md) | Dane multiplayer (131 KB) |
| 19 | [Weather & Time](19-weather-time.md) | Pogoda i czas w świecie gry |
| 20 | [Version & Platform](20-version-platform.md) | Wersja save, Steam ID, PS5 Activity |
| 21 | [DLC](21-dlc.md) | Flagi DLC: pre-order gesty, Shadow of the Erdtree entry |
| 22 | [Player Data Hash](22-player-hash.md) | Hash końcowy danych gracza |
| 23 | [UserData10](23-user-data-10.md) | Profil konta: ProfileSummary ×10, SteamID, active slots |
| 24 | [UserData11](24-user-data-11.md) | regulation.bin — parametry gry (params) |
| 25 | [Runtime vs Save](25-runtime-vs-save.md) | Mapowanie offsetów pamięć↔plik, ostrzeżenia |
| 26 | [Parameter Reference](26-parameter-reference.md) | **Kompletna referencja** wszystkich edytowalnych parametrów |
| 27 | [Map Reveal](27-map-reveal.md) | 4-warstwowy model odkrywania mapy: regions / event flags 62xxx + Map Fragments / DLC Cover Layer / FoW bitfield |
| 29 | [DLC Black Tiles](29-dlc-black-tiles.md) | Cover Layer SoE — koordynaty discovery w sekcji BloodStain (`afterRegs+0x0088..0x0110`) |
| 30 | [Slot Rebuild Research](30-slot-rebuild-research.md) | Analiza slack + przejście od byte-shift do `RebuildSlot` (R-1 Step 13–14) |
| 31 | [Appearance Presets](31-appearance-presets.md) | Layout slotu presetu Mirror Favorites (0x130 bajtów), algorytm apply preset → FaceData, obsługa cross-gender M↔F — RE z prawdziwych save'ów |
| 32 | [Ban-Risk System](32-ban-risk-system.md) | Architektura UI: Tier 0/1/2 + Online Safety Mode + słownik `RISK_INFO` + komponenty `Risk*` (Faza 6) |
| 33 | [DB Categorization Audit](33-db-categorization-audit.md) | Wyciągnięcie zakładki Information + reklasyfikacja Multiplayer/Remembrances/Crystal Tears/Materials wg Fextralife per-item breadcrumb |
| 34 | [Item Caps Enforcement](34-item-caps.md) | Vanilla-realistic MaxInventory/MaxStorage + flaga `scales_with_ng` (effective_cap = base × (ClearCount+1)) + tryb Full Chaos Mode |
| 36 | [Inventory Categories — Game Order](36-inventory-categories-game-order.md) | Kanoniczna kolejność 18 zakładek + sub-grupowanie (Tools/Key Items/Melee/etc.) + reklasyfikacje (Larval Tears, Torches, Region Maps, Golden Runes) — rozszerza spec/33 |
| 37 | [Character Presets](37-character-presets.md) | **Design doc** — format JSON export/import, struktury Go, fazy 1-5 (stats + inventory + appearance + world flags) |
| 38 | [Boss Kill Multi-Flag](38-boss-multiflag.md) | **Design doc** — podejście multi-flag dla poprawnego kill/respawn bossa (arena state + quest + grace flags) |
| 39 | [Inventory Reorder](39-inventory-reorder.md) | **Design doc** — drag & drop siatka z reordering przez manipulację `acquisition_index`, fazy 0-4, biblioteka DnD |
| 40 | [DB Cleanup Plan](40-db-cleanup-plan.md) | **Design doc** — rejestr cut-content, dedup multiplayer, usunięcie pustych flasków, fazy A-G |
| 41 | [Info-Tab Ground Drop](41-info-tab-ground-drop.md) | **Investigacja** — flagi world pickup + TutorialDataChunk zbadane, gating EMEVD nieznany |
| 42 | [Summoning Pools Bug](42-summoning-pools-bug.md) | **Investigacja** — UI działa, brak efektu in-game; checklista diagnostyczna + hipotezy |
| 43 | [Transactional Item Adding](43-transactional-item-adding.md) | **Design doc** (✅ zaimplementowany v0.7.2) — architektura ALL-OR-NOTHING, pre-flight + snapshot/rollback |
| 44 | [NetworkParam PvP Tuning](44-network-param-tuning.md) | **Design doc** (✅ częściowo) — pełna referencja pól NETWORK_PARAM_ST: offsety, wartości vanilla, ryzyko bana, presety |
| 45 | [Dokumentacja Ryzyka Bana](45-ban-risk-reference.md) | Community-reportowane triggery banów, poziomy kar (mechanika 180-dniowego softbana, flaga zapisana w save), zasady bezpiecznej edycji — podstawa dla tiers ryzyka w spec/32 |
| 99 | [Verification Methodology](99-verification-methodology.md) | Metody testowania, checklista weryfikacji, plan odkryć |

---

## Kluczowe właściwości formatu

- **Endianness**: Little-endian (wszystkie wartości liczbowe)
- **Stringi**: UTF-16LE z null-terminatorem
- **Rozmiar slotu**: 0x280000 (2,621,440 bajtów) — stały
- **Sekcje zmiennej długości**: inventory projectiles, regions, world areas — wymagają sekwencyjnego parsingu
- **Checksumy**: MD5 (tylko PC), przeliczane przy zapisie
- **Szyfrowanie**: AES-128-CBC (tylko PC, opcjonalne — nowsze wersje gry)

---

## Źródła wiedzy

### Projekty referencyjne (lokalne kopie w `tmp/repos/`)

| Projekt | Język | Priorytet | Opis |
|---|---|---|---|
| [er-save-manager](https://github.com/Jeius/er-save-manager) | Python | **1 (najwyższy)** | Najnowszy, pełny sekwencyjny parser z DLC support |
| [ER-Save-Editor](https://github.com/ClayAmore/ER-Save-Editor) | Rust | **2** | Dobrze typowany parser, potwierdza rozmiary struktur |
| [Elden-Ring-Save-Editor](https://github.com/shalzuth/Elden-Ring-Save-Editor) | Python | **3 (najniższy)** | Stary, pattern-matching approach, ale pierwsze odkrycia offsetów |

### Dokumentacja online

| Źródło | URL | Zawartość |
|---|---|---|
| Souls Modding Wiki — SL2 Format | https://www.soulsmodding.com/doku.php?id=format:sl2 | Format kontenera save |
| Souls Modding Wiki — Event Flags | https://www.soulsmodding.com/doku.php?id=er-refmat:event-flag-list | Lista flag zdarzeń |
| Event Flags GitHub Pages | https://soulsmods.github.io/elden-ring-eventparam/ | Pełna lista 1000+ flag z opisami |
| Event Flags Spreadsheet | https://docs.google.com/spreadsheets/d/1Nn-d4_mzEtGUSQXscCkQ41AhtqO_wF2Aw3yoTBdW9lk | Szczegółowy arkusz flag |
| Steam Guide — Save Structure | https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037 | Offsety slotów, MD5 checksumy |
| SoulsFormats (C#) | https://github.com/JKAnderson/SoulsFormats | BND4 format parsing library |
| TGA Cheat Engine Table | https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA | Event flags, param scripts, item IDs |
| Souls Modding Wiki — Params | https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam | SpEffect i inne param tables |

### Pliki danych (lokalne)

| Plik | Ścieżka | Opis |
|---|---|---|
| eventflag_bst.txt | `tmp/repos/er-save-manager/src/resources/eventflag_bst.txt` | 11919 wpisów — mapowanie block→offset dla event flags |
| PC Save | `tmp/save/ER0000.sl2` | Prawdziwy save PC (5 slotów) |
| PS4 Save | `tmp/save/oisisk_ps4.txt` | Prawdziwy save PS4 |

---

## Konwencje w dokumentacji

- **Offsety** zapisujemy jako hex: `0x1B0`
- **Rozmiary** w bajtach hex i dziesiętnie: `0x12F (303 bytes)`
- **Typy danych**: u8, u16, u32, i32, f32, u64 (little-endian)
- **Stringi**: UTF-16LE, podajemy max liczbę znaków (nie bajtów)
- **Sekcje zmiennej długości**: oznaczamy jako `[VARIABLE]`
- **Nieznane pola**: oznaczamy jako `unk_0xNN` z notatką co wiemy
- **Status weryfikacji**: ✅ zweryfikowano hex | ⚠️ cross-reference only | ❓ niepewne

---

## Tłumaczenia

Angielskie wersje wszystkich dokumentów specyfikacji znajdują się w [`spec/`](../) (katalog nadrzędny).
