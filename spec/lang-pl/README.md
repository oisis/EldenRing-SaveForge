# Elden Ring Save File Format — Podręcznik dla moderów

> **Cel**: Kompletna dokumentacja formatu binarnego pliku zapisu Elden Ring (`.sl2` / `memory.dat`) oraz systemów edytora SaveForge. Wystarczająca do implementacji niezależnego edytora save od zera.
>
> **Stan**: 🚧 **Work in progress — book cleanup (Phase 1)**.
> Dokumentacja jest właśnie reorganizowana w układ książkowy (Part I-IV + Appendix). Treść merytoryczna rozdziałów pozostaje nietknięta — zmieniła się tylko lokalizacja plików i taksonomia.
>
> **Plan dalszych prac**: zobacz [`BOOK_PLAN.md`](BOOK_PLAN.md). Wynik audytu źródłowego: [`tmp/docs-book-audit.md`](../../tmp/docs-book-audit.md) (lokalny, gitignored).

---

## Jak czytać tę dokumentację

Dokumenty w głównym katalogu `spec/lang-pl/` to **kanoniczne rozdziały podręcznika** — zweryfikowana wiedza o formacie binarnym i zaimplementowanych systemach edytora. Trzy podkatalogi zawierają materiały, które nie należą do głównej narracji:

| Katalog | Zawartość | Status |
|---|---|---|
| `spec/lang-pl/` (root) | Kanoniczne rozdziały — aktualna wiedza referencyjna | source of truth |
| `spec/lang-pl/research/` | Historyczne badania, negatywne wyniki, wstrzymane investigacje | nie aktualny stan |
| `spec/lang-pl/planned/` | Design doci bez (pełnej) implementacji | nie odzwierciedla kodu |
| `spec/lang-pl/archive/` | Dokumenty zastąpione nowszymi rozdziałami — zachowane dla historii | historyczne |

**Legenda statusów** używana w spisie treści poniżej:

| Status | Znaczenie |
|---|---|
| `canonical` | Aktualny, zgodny z kodem, kandydat na finalny rozdział książki |
| `implemented, needs rewrite` | Kod istnieje i działa, ale dokument wymaga przepisania w canonical template |
| `partial` | Częściowo zweryfikowane / częściowo zaimplementowane — wymaga uzupełnień |
| `needs verification` | Konflikt doc vs code — wymaga manualnej weryfikacji per sekcja |
| `research` | Negatywny wynik lub wstrzymane badanie — `research/` |
| `planned` | Design bez implementacji — `planned/` |
| `archived` | Zastąpione nowszym dokumentem — `archive/` |

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

## Spis treści książki

### Part I — Save File Format Fundamentals

Format binarny pliku save — kontener, sloty, layout sekcji.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 01 | [Header i layout pliku](01-header.md) | `canonical` | Magic bytes, detekcja platformy, BND4, MD5 |
| 02 | [Slot — struktura ogólna](02-slot-structure.md) | `canonical` | Rozmiar slotu, sekwencyjny parsing |
| 03 | [GaItem Map](03-gaitem-map.md) | `implemented, needs rewrite` | Layout 21/16/8B; semantyka AoW zdubluje 54 → planowany merge |
| 04 | [PlayerGameData](04-player-game-data.md) | `canonical` | 432 B, atrybuty, runy, online settings |
| 05 | [SP Effects](05-sp-effects.md) | `needs verification` | Sekcja krótka, „wymaga weryfikacji" w treści; brak parsera w `backend/core/` |
| 06 | [Equipment](06-equipment.md) | `canonical` | 22 sloty, active weapon slots, arm style |
| 07 | [Inventory](07-inventory.md) | `implemented, needs rewrite` | Reconciler działa; layout chaotyczny — kandydat do canonical template |
| 08 | [Spells & Gestures](08-spells-gestures.md) | `canonical` | 14 attunement + 8B gesture stride |
| 09 | [Face Data](09-face-data.md) | `partial` | 303 B, pola 0x20-0x5F „przybliżone" — kod (`app_appearance.go`) zna więcej |
| 10 | [Storage Box](10-storage.md) | `canonical` (merge candidate) | Format identyczny jak Inventory, inne countery |
| 11 | [Regions](11-regions.md) | `canonical` | `core.SetUnlockedRegions`, cross-link do 27 |
| 12 | [Torrent](12-torrent.md) | `canonical` | State enum 1/3/13; bug HP=0+State=13 |
| 13 | [Blood Stain](13-blood-stain.md) | `partial` | unk_0x1c..0x40 — w spec/29 te offsety to DLC Cover Layer (konflikt do rozwiązania) |
| 14 | [Game State](14-game-state.md) | `canonical` | LastRestedGrace, ClearCount, GaItem Game Data |
| 15 | [Event Flags](15-event-flags.md) | `canonical` | BST + bit-order big-endian, kanon dla flag |
| 16 | [World State](16-world-state.md) | `partial` | Container hex-verified, content uncertain |
| 17 | [Player Coordinates](17-player-coordinates.md) | `canonical` | 57 B, mapa decompose ⚠️ orientacyjne |
| 18 | [Network Manager](18-network.md) | `partial` (merge candidate) | 131 KB opaque — slot-local, nie regulation NetworkParam |
| 19 | [Weather & Time](19-weather-time.md) | `canonical` | 12+12 B, trivial |
| 20 | [Version & Platform](20-version-platform.md) | `canonical` | Steam ID + BaseVersion + PS5Activity |
| 21 | [DLC](21-dlc.md) | `canonical` | 50 B, invariant bytes 3-49 = 0 |
| 22 | [Player Data Hash](22-player-hash.md) | `canonical` | „Gra ignoruje hash" — `backend/core/hash.go` potwierdza |
| 23 | [UserData10](23-user-data-10.md) | `canonical` | PC + PS4 offsety zweryfikowane Apr 2026 |
| 24 | [UserData11](24-user-data-11.md) | `canonical` (read-only) | TWARDA REGUŁA — nie modyfikujemy |
| 25 | [Runtime vs Save](25-runtime-vs-save.md) | `canonical` | CT memory vs save offsety — wartość edukacyjna |
| 26 | [Parameter Reference](26-parameter-reference.md) | `partial` (needs rewrite) | Tytuł obiecuje więcej niż dostarcza — atrybuty + softcaps |

### Part II — Implemented SaveForge Systems

Zaimplementowane mechanizmy edytora — działają w aktualnym kodzie.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 32 | [Ban-Risk System (UI)](32-ban-risk-system.md) | `canonical` | SafetyMode, RISK_INFO, komponenty `Risk*` |
| 34 | [Item Caps Enforcement](34-item-caps.md) | `canonical` | `scales_with_ng` + NG+ scaling — TODO o ClearCount otwarte |
| 36 | [Inventory Categories — Game Order](36-inventory-categories-game-order.md) | `canonical` | Kanonicza taksonomia 18 zakładek (nadpisuje 33) |
| 39 | [Inventory Reorder](39-inventory-reorder.md) | `implemented, needs rewrite` | ⚠️ Status w doc deklaruje „Planowany", ale `ReorderInventory` + stride-2 działają (patrz konflikt F2 w audycie) |
| 43 | [Transactional Item Adding](43-transactional-item-adding.md) | `canonical` | Pre-flight + snapshot/rollback (v0.7.2) |
| 44 | [NetworkParam Tuning](44-network-param-tuning.md) | `canonical` | `regulation.go::PatchNetworkParams` |
| 50 | [Item Companion Flags](50-item-companion-flags.md) | `canonical` | SET+CLEAR mechanism (v0.14.0) |
| 52 | [Acquisition Sort — Stride-2](52-acquisition-sort-stride2.md) | `canonical` | Dlaczego klucz to `acqIdx>>1` |
| 53 | [Inventory ↔ Storage Transfer](53-inventory-storage-transfer.md) | `canonical` | Two-way drag-and-drop, rehandle path |
| 54 | [Ash of War](54-ash-of-war.md) | `canonical` | Sentinele 0x00/0xFFFFFFFF + invariant unikalności + AoW guard (commit `6881cb9`) |
| 55 | [Build Template](55-build-template.md) | `canonical` | JSON v1, portable export bez save-local handles |

### Part III — Verified Game Mechanics

Mechaniki gry zweryfikowane przez RE / testy in-game.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 27 | [Map Reveal](27-map-reveal.md) | `canonical` | 4-warstwowy model; `revealBaseMap/revealDLCMap/RemoveFogOfWar` |
| 29 | [DLC Black Tiles](29-dlc-black-tiles.md) | `implemented, needs rewrite` | Split planowany: layout (do Ch.11) + research log (do Appendix E) |
| 31 | [Appearance Presets](31-appearance-presets.md) | `canonical` | Apply algorithm + Mirror Favorites; PC verified |
| 47 | [Sites of Grace — Activation](47-site-of-grace-activation.md) | `canonical` | Hipoteza D potwierdzona, Model 3 (UI nota) |
| 48 | [PvP Modular Presets](48-pvp-ready-modular-presets.md) | `partial` (needs rewrite) | ⚠️ Faza 1 częściowo wdrożona — `app_pvp.go:109` mówi „Sites of Grace module planned but not enabled" (konflikt F9) |
| 49 | [PS4 ZSTD Raw-Block Patch](49-ps4-zstd-rawblock-patch.md) | `canonical` | Krytyczna wiedza PS4 — `regulation.go:604` |

### Part IV — Research Archive / Negative Results

Historia badań, wstrzymane investigacje, negatywne wyniki.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 30 | [Slot Rebuild — Research](research/30-slot-rebuild-research.md) | `research` | Dziennik pomiarów slack 11 slotów; finalna implementacja: `RebuildSlot` |
| 41 | [Info-Tab Ground Drop](research/41-info-tab-ground-drop.md) | `research` | 🐛 Wstrzymane — wymagana dekompilacja EMEVD |
| 42 | [Summoning Pools Bug](research/42-summoning-pools-bug.md) | `research` | 🐛 Wstrzymane — UI działa, brak efektu in-game |
| 46 | [Faster Invasions](research/46-faster-invasions-research.md) | `research` (negative) | Werdykt: niemożliwe przez plik save |

### Planowane

Design doci bez implementacji w kodzie.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 37 | [Character Presets (JSON)](37-character-presets.md) | `needs verification` ⚠️ | **Pozostaje w głównym katalogu** — `backend/vm/preset.go` ma `CharacterPreset/VMToPreset/PresetToVM/ValidatePreset`, ale doc deklaruje „Planowany". Wymaga weryfikacji per faza przed przesunięciem do `planned/`. |
| 38 | [Boss Multi-Flag](planned/38-boss-multiflag.md) | `planned` | Kod ma 1-flag model; design wymaga `EventFlags []uint32` (nie wdrożone) |
| 40 | [DB Cleanup Plan](planned/40-db-cleanup-plan.md) | `planned` | Fazy A-G; weryfikacja per faza wymagana |
| 51 | [Advanced Save Editor](planned/51-advanced-save-editor.md) | `planned` | Brak `AdvancedSaveEditor` w kodzie |

### Archiwum

Dokumenty zastąpione nowszymi rozdziałami — zachowane dla historii.

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 33 | [DB Categorization Audit](archive/33-db-categorization-audit.md) | `archived` | Post-mortem migracji DB; zastąpione przez 36 |

### Appendix (planowany)

| Dok | Tytuł | Status | Notatka |
|---|---|---|---|
| 45 | [Dokumentacja Ryzyka Bana](45-ban-risk-reference.md) | `canonical` (App. A) | Community triggers — knowledge base, podstawa dla tiers w 32 |
| 99 | [Verification Methodology](99-verification-methodology.md) | `canonical` (App. B) | Metodyka research |

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

Angielskie wersje wszystkich dokumentów specyfikacji znajdują się w [`spec/`](../) (katalog nadrzędny). **Uwaga**: reorganizacja Phase 1 dotyczy tylko `spec/lang-pl/` — wersja angielska zostanie zsynchronizowana w późniejszej fazie.
