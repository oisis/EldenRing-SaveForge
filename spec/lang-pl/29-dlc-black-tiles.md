# 29 — DLC Black Tiles (warstwa zakrywająca mapę DLC)

> **Type**: Binary format spec + design doc (detail chapter)
> **Scope**: Warstwa L2 modelu Map Reveal — czarne kafelki zakrywające mapę DLC "Shadow of the Erdtree" oraz syntetyczne współrzędne BloodStain, które edytor zapisuje, aby zdjąć tę warstwę.

Ten rozdział jest **detail chapter** dla warstwy L2 z [27-map-reveal.md](27-map-reveal.md). Nie powiela master modelu 4 warstw — opisuje wyłącznie:

- skąd biorą się czarne kafelki DLC,
- jakie dane w `BloodStain` kontrolują tę warstwę,
- które stałe i wartości zapisuje aktualny kod (`app_world.go::revealDLCMap`),
- jakie pułapki/ryzyka są związane z tym rozwiązaniem.

Cross‑refs: [11-regions.md](11-regions.md) (L0 — UnlockedRegions), [15-event-flags.md](15-event-flags.md) (helper API event flag), [27-map-reveal.md](27-map-reveal.md) (master 4‑layer model), [50-item-companion-flags.md](50-item-companion-flags.md) (item companion flag policy dla Map Fragment items).

---

## 1. Cel rozdziału

W odróżnieniu od podstawowej gry, mapa DLC ma domyślnie zakryty cały obszar **hard‑blackoutem** — pełnymi czarnymi kafelkami, których nie zdejmują:

- event flagi `62080..62084` ani `628xx..629xx` (visible flags, warstwa L1),
- przedmioty Map Fragment z DLC (`0x401EA618..0x401EA61C`),
- Fog of War bitfield (warstwa L3, oddzielny system — patrz §17/27),
- system flagi `62002` / `82002`.

Ta warstwa istnieje fizycznie nad warstwą L1 i wymaga zapisu „odwiedzonych” koordynatów do sekcji `BloodStain`, żeby gra renderowała mapę DLC jako odsłoniętą. To jest L2 modelu Map Reveal (27 §4 / §17.2).

`needs verification`: powyższa lista negatywów odzwierciedla wyniki badań w `docs/CHANGELOG.md` (branch `fix/dlc-map-reveal-v2`). Wynik został potwierdzony w grze przez autora; nie jest objęty testem automatycznym.

## 2. Status

| Aspekt | Status |
|---|---|
| Implementacja w edytorze | ✅ `app_world.go::revealDLCMap` Phase 3 |
| Stałe offsetów | ✅ `backend/core/offset_defs.go` (`DLCTile*`) |
| Wymagana akcja użytkownika | Tier 1 risk gate (`RiskActionButton riskKey="map_reveal_full"`) |
| Pokrycie testem automatycznym | ❌ brak dedykowanego testu DLCTile (`needs verification` po patchach gry) |
| Weryfikacja in‑game | ✅ historyczne potwierdzenie (CHANGELOG, branch SOLVED) |
| Stabilność po patchach | ❓ `needs verification` — wartości są empiryczne, nie game‑guaranteed |

## 3. Source of truth w kodzie

| Plik | Co zawiera |
|---|---|
| `backend/core/offset_defs.go` lines 303‑323 | Stałe `DLCTileZeroStart/End`, `DLCTileRec1{X,Y,Flag}`, `DLCTileRec2{X,Y,Z,W,Flag}` |
| `app_world.go::revealDLCMap` (Phase 3) | Procedura zapisu syntetycznych współrzędnych |
| `app_world.go::resolveAfterRegs` | Wyznacza `afterRegs` (koniec UnlockedRegions) — relatywna baza offsetów |
| `app_world.go::RevealAllMap` | Public API; wywołuje `revealBaseMap` + `revealDLCMap` |
| `app_pvp.go::PrepPvP` (`opts.RevealMap`) | Drugi caller dla `revealDLCMap` |
| `backend/db/data/maps.go::IsDLCMapFlag` | Definiuje, które visible flags są DLC (`62080..62084 || 62800..62999`) |
| `backend/db/data/maps.go::MapFragmentItems` | 5 DLC fragmentów (`62080..62084` → `0x401EA618..0x401EA61C`) |

Dokument 29 jest dopełnieniem 27 — nie powiela `RevealAllMap` ani logiki Phase 1/Phase 2, tylko skupia się na Phase 3.

## 4. Mental model

Mapa DLC w grze ma trzy „filtry” renderujące, które trzeba zdjąć równolegle:

```
┌──────────────────────────────────────────────┐
│  L3  Fog of War overlay (afterRegs+0x087E..) │  ← oddzielny endpoint RemoveFogOfWar
├──────────────────────────────────────────────┤
│  L2  DLC Cover Layer  (czarne kafelki)       │  ← ten rozdział: BloodStain coords
├──────────────────────────────────────────────┤
│  L1  Detailed bitmap (event flags 62xxx +    │  ← revealDLCMap Phase 1+2
│      Map Fragment items)                      │
├──────────────────────────────────────────────┤
│  L0  UnlockedRegions ([]u32)                  │  ← spec/11
└──────────────────────────────────────────────┘
```

L2 jest sterowana danymi `BloodStain`, nie event flagami. To pojedyncza warstwa charakterystyczna dla DLC — w base game Cover Layer jest domyślnie przezroczysty (ten rozdział nie modyfikuje danych base game).

`needs verification`: założenie „base game Cover Layer jest domyślnie przezroczysty” pochodzi z empirycznego porównania świeżych saveów base game z saveami DLC — nie z reverse engineeringu mechaniki gry.

## 5. Relation to Map Reveal L2

| Aspekt | `27-map-reveal.md` (master) | `29` (ten rozdział) |
|---|---|---|
| 4‑layer overview | ✅ master | linkuje |
| L1 event flags 62xxx semantyka | ✅ master | linkuje |
| L1 Map Fragment items | ✅ master | linkuje (5 DLC) |
| L2 — istnienie warstwy | ✅ master | ✅ rozwija mechanikę |
| L2 — stałe `DLCTile*` | ❌ (linkuje) | ✅ szczegóły hex |
| L2 — syntetyczne współrzędne | ❌ (linkuje) | ✅ wartości i procedura |
| L3 FoW | ✅ master | linkuje |

Reguła: 27 = co się dzieje warstwowo; 29 = jak konkretnie kod realizuje warstwę L2.

## 6. DLC tile data model

Warstwa L2 jest sterowana **dwoma rekordami pozycji** umiejscowionymi wewnątrz sekcji `BloodStain` w obszarze danych pomiędzy końcem `UnlockedRegions` a początkiem bitfield Fog of War.

```
afterRegs + 0x0000  ← koniec UnlockedRegions (rozmiar zależny od slotu)
afterRegs + 0x0075  ← początek BloodStain
┃
┣━ afterRegs + 0x0088..0x0110  ← Zakres pól danych pozycji DLC (136 B)
┃    ┃
┃    ┣━ Rec1 (centrum mapy DLC):    2× f32 + 1× u8
┃    ┗━ Rec2 (kotwica obszaru DLC): 4× f32 + 1× u8
┃
afterRegs + 0x087E  ← początek bitfield FoW (L3, oddzielne)
afterRegs + 0x10B0  ← koniec bitfield FoW
afterRegs + 0x10B1  ← początek menuProfile
```

`afterRegs` wyznacza `resolveAfterRegs(slot)` w `app_world.go`:

```
storageEnd  = StorageBoxOffset + core.DynStorageBox
gesturesOff = storageEnd + core.DynStorageToGestures
regCount    = u32 LE @ gesturesOff
afterRegs   = gesturesOff + 4 + regCount * 4
```

Czyli — jakakolwiek mutacja długości `UnlockedRegions` (`SetUnlockedRegions` / `RebuildSlot`) przesuwa również `BloodStain` i tym samym **wszystkie** offsety DLCTile. Kod zawsze rozwiązuje `afterRegs` przed zapisem.

## 7. DLC tile constants and synthetic coordinates

### 7.1 Stałe offsetów

Z `backend/core/offset_defs.go:307‑323` (offsety relatywne do `afterRegs`):

| Stała | Offset | Typ | Komentarz w kodzie |
|---|---|---|---|
| `DLCTileZeroStart` | `0x0088` | — | start of range to zero out before writing coords |
| `DLCTileZeroEnd`   | `0x0110` | — | end of range (exclusive) |
| `DLCTileRec1X`     | `0x008D` | f32 | X coordinate |
| `DLCTileRec1Y`     | `0x0091` | f32 | Y coordinate |
| `DLCTileRec1Flag`  | `0x0095` | u8  | visited flag |
| `DLCTileRec2X`     | `0x00C5` | f32 | X |
| `DLCTileRec2Y`     | `0x00C9` | f32 | Y |
| `DLCTileRec2Z`     | `0x00CD` | f32 | Z |
| `DLCTileRec2W`     | `0x00D1` | f32 | W |
| `DLCTileRec2Flag`  | `0x00D5` | u8  | visited flag |

### 7.2 Syntetyczne wartości

Z `app_world.go::revealDLCMap` Phase 3:

| Pole | Wartość | Pochodzenie |
|---|---|---|
| `Rec1.X` | `9648.0` (f32) | empiryczna — odpowiada centrum mapy DLC w slocie 0/1 referencyjnego save'a |
| `Rec1.Y` | `9124.0` (f32) | jw. |
| `Rec1.Flag` | `0x01` (u8) | znacznik „visited” (Rec1) |
| `Rec2.X` | `3037.0` (f32) | empiryczna — anchor kotwicy obszaru DLC |
| `Rec2.Y` | `1869.0` (f32) | jw. |
| `Rec2.Z` | `7880.0` (f32) | jw. |
| `Rec2.W` | `7803.0` (f32) | jw. |
| `Rec2.Flag` | `0x01` (u8) | znacznik „visited” (Rec2) |

`needs verification` — wartości pochodzą z dwóch slotów referencyjnych (slot 0 i slot 1) z ukończonym DLC; CHANGELOG odnotowuje obserwację „slot 0 i slot 1 mają identyczne wartości w tym zakresie”. Charakter koordynatów (czy to ostatni Bloodstain? punkt odrodzenia? kotwica odkrywania?) **pozostaje niezweryfikowany**. Wartości NIE są game‑guaranteed — przyszły patch gry może je zmienić.

`needs verification` — czy gra akceptuje inne dowolne współrzędne z obszaru DLC, czy wymaga konkretnie tych wartości, **nie zostało rozstrzygnięte**. CHANGELOG sygnalizuje to jako otwarte pytanie.

### 7.3 Endianness i layout

Pola f32 zapisywane są przez `putF32` w **little‑endian** (`binary.LittleEndian.PutUint32` + `math.Float32bits`). Bajt flagi `0x01` to literalny zapis. Brak zmiennej długości — całe `[0x0088..0x0110)` to 136 bajtów stałego layoutu.

## 8. DLC event flags

L2 NIE jest sterowana przez event flagi. Event flagi 62xxx DLC (`62080..62084` + `62800..62999`) sterują warstwą **L1** (detailed bitmap), nie L2. `IsDLCMapFlag` (maps.go:378) jest predykatem klasyfikującym, której gałęzi (`revealBaseMap` vs `revealDLCMap`) flagi dotyczą — nie reprezentuje fizycznej obecności kafelka.

Ten dokument celowo NIE powiela semantyki helper API `db.GetEventFlag` / `db.SetEventFlag` — patrz [15-event-flags.md](15-event-flags.md).

`needs verification`: zakres `62800..62999` w `IsDLCMapFlag` obejmuje 200 ID; w `MapVisible` realnie istnieje ~39 sub‑regionów DLC (`62800..62999`) + 5 region anchors (`62080..62084`) = 44 DLC visible flags. Pozostałe ID w paśmie `62800..62999` traktowane są jako DLC jeśli pojawią się w `MapVisible` w przyszłości — kod nie zakłada gęstości.

## 9. 62002 / 82002 divergence

`revealDLCMap` Phase 1 ustawia inline (`app_world.go:1099‑1100`):

```go
_ = db.SetEventFlag(flags, 62002, true) // Allow Shadow Realm Map Display
_ = db.SetEventFlag(flags, 82002, true) // Show Shadow Realm Map
```

Tymczasem `MapSystem` (`backend/db/data/maps.go:10`) zawiera **tylko** `62000`, `62001`, `82001`, `82002` — `62002` nie jest w `MapSystem`. To znaczy, że:

- `82002` jest zarówno w `MapSystem` (ustawiane przez `revealBaseMap` w pętli), jak i ustawione inline w `revealDLCMap` — efektywnie ustawiane dwukrotnie. Idempotentne, bez efektów ubocznych.
- `62002` jest ustawiane **tylko** inline w `revealDLCMap` i nigdzie indziej.

Powód takiego rozdziału (czy `62002` jest celowo poza `MapSystem` bo jest „DLC‑only allow”, czy to historyczny przeoczony brak) — **nie został udokumentowany w kodzie**. Master analiza tej divergence znajduje się w [27-map-reveal.md](27-map-reveal.md) §9; tu wymieniamy tylko fakt, że Phase 3 (L2) nie zależy od żadnej z tych flag.

`needs verification` — czy ustawienie `62002`+`82002` bez Phase 3 wystarczy do zdjęcia hard‑blackoutu. Wyniki w CHANGELOG sugerują „nie” (event flags 62xxx jako kategoria były testowane i nie usuwają kafelków), ale nie ma izolowanego testu „set only 62002+82002, leave Phase 3 untouched, check in‑game”.

## 10. DLC ownership and map fragments

`revealDLCMap` Phase 2 dodaje do ekwipunku do 5 DLC Map Fragment items (`62080..62084` → `0x401EA618..0x401EA61C` z `MapFragmentItems`). Każdy item jest dokładany przez `core.AddItemsToSlot` z `quantity=1, durability=0`.

**Co kod NIE robi**:

- nie sprawdza, czy gracz posiada DLC (brak gate'u DLC ownership po stronie save'a — patrz `backend/core/offset_defs.go:325‑335` o `DlcSectionOffset` / `DlcEntryFlagByte`; ta sekcja nie jest konsultowana przez `revealDLCMap`),
- nie sprawdza, czy item DLC już istnieje (item companion flag policy patrzy na flagę pickup — patrz [50-item-companion-flags.md](50-item-companion-flags.md)),
- nie waliduje, czy syntetyczne koordynaty są sensowne dla wersji gry, w której uruchamia się save.

`needs verification` — czy próba zapisu DLC reveal w save graczu, który **nie ma DLC**, powoduje crash, soft‑lock loading, czy graceful no‑op. Historyczna nota CHANGELOG o `DlcEntryFlagByte = 1` ostrzega, że „non‑zero = entered DLC; causes infinite loading without DLC” — to dotyczy `CSDlc[1]`, NIE jest dotykane przez `revealDLCMap`, ale ryzyko cross‑section nie zostało izolowane testem.

Companion flag policy dla DLC Map Fragment items (czy `AddItemsToSlot` ustawia pickup flag, czy zostawia decyzję gry) — patrz [50-item-companion-flags.md](50-item-companion-flags.md). Ten rozdział nie powiela tej polityki.

## 11. Current implemented behavior

`app_world.go::revealDLCMap` wykonuje trzy fazy w stałej kolejności:

| Faza | Akcja | Powód kolejności |
|---|---|---|
| Phase 1 | Inline `SetEventFlag(62002, true)` + `SetEventFlag(82002, true)`. Następnie pętla po `MapVisible`: jeżeli `IsDLCMapFlag(id)`, ustaw event flag + zbierz odpowiadający item z `MapFragmentItems`. | Event flagi ustawiane PRZED `AddItemsToSlot`, bo dodawanie itemów przesuwa `slot.Data` i unieważnia `flags := slot.Data[slot.EventFlagsOffset:]`. |
| Phase 2 | Dla każdego zebranego DLC fragmentu: `core.AddItemsToSlot(slot, []uint32{itemID}, 1, 0, false)`. | Wykonywane PRZED Phase 3, bo `AddItemsToSlot` ponownie przesuwa `slot.Data` i unieważnia `afterRegs` policzony wcześniej. |
| Phase 3 | `resolveAfterRegs(slot)` → zero `[0x0088..0x0110)` → zapis Rec1/Rec2 + flagi. | Wykonywane PO Phase 2, żeby `afterRegs` był aktualny po przesunięciach. |

Phase 3 jest **wyłącznym writerem** danych w `[0x0088..0x0110)` z poziomu `revealDLCMap`. Endpoint `ResetMapExploration` (`app_world.go:1162`) **nie cofa** Phase 3 — czyści tylko event flagi `MapVisible` / `MapAcquired` / `MapUnsafe`. Cofnięcie syntetycznych współrzędnych wymaga `Undo` (snapshot pre‑reveal) albo manualnego odtworzenia.

Wywołania:

- `app_world.go::RevealAllMap(slotIndex)` — public Wails endpoint, łączy `revealBaseMap` + `revealDLCMap`.
- `app_pvp.go::PrepPvP(opts.RevealMap)` — module w PvP prep flow.

UI:

- `frontend/src/components/WorldTab.tsx:219` — `handleRevealAllMap` wywołuje `RevealAllMap` + `RemoveFogOfWar` jednym zatwierdzeniem; chronione `RiskActionButton riskKey="map_reveal_full"` (Tier 1 risk).

`needs verification` — czy `handleRevealAllMap` w UI faktycznie cache'uje stan jako „wszystko ujawnione” bez round‑tripu przez `GetMapProgress`. W kodzie linia 219 ustawia `setMapEntries(prev => prev.map(e => ({...e, enabled: true})))` lokalnie — jeśli `revealDLCMap` z dowolnego powodu nie powiódł się dla części flag, UI pokaże stan rozjeżdżony z rzeczywistością do następnego `GetMapProgress`. Master nota o cache stale risk: [15-event-flags.md](15-event-flags.md) §L10.

## 12. Write path and rollback caveats

### 12.1 Atomowość

`RevealAllMap` / `PrepPvP(RevealMap)` **nie są atomowe per‑warstwa**. Sekwencja `revealBaseMap` → `revealDLCMap` → (Phase 1 flagi → Phase 2 items → Phase 3 BloodStain) zawiera 3 punkty mutacji `slot.Data`. Jeśli błąd nastąpi w którejkolwiek z faz, brak per‑fazowego rollbacku — zmiany wykonane do tej pory zostają w `slot.Data`. Test pokrycia tego ścieżki braku → patrz §14.

### 12.2 Snapshot undo (save‑level)

`RevealAllMap` wywołuje `a.pushUndo(slotIndex)` PRZED `revealBaseMap`. To jest snapshot całego slotu — `Undo` przywróci stan przed Phase 1 włącznie z surowymi danymi BloodStain. To **jedyna** ścieżka rollbacku Phase 3.

`PrepPvP` (`app_pvp.go:41`) wywołuje `a.pushUndo(slotIndex)` raz na początku, tym samym mechanizmem — Phase 3 podlega temu samemu single‑snapshot rollbackowi (patrz [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md)).

### 12.3 Brak per‑slot detekcji „już ujawnione”

`revealDLCMap` nie wykrywa, czy `[0x0088..0x0110)` zawiera już syntetyczne koordynaty. Każde wywołanie nadpisuje pełnym zerowaniem + zapisem stałych wartości. W praktyce idempotentne, ale **destrukcyjne** dla ewentualnych autentycznych danych BloodStain z prawdziwej eksploracji DLC przez gracza — patrz §13.

### 12.4 Refresh `flags` po Phase 2

Wewnątrz `revealDLCMap` (`app_world.go:1096`) `flags` jest wyprowadzane raz na wejściu funkcji i używane tylko w Phase 1. Phase 2 mutuje `slot.Data` przez `AddItemsToSlot` (slice `flags` staje się nieaktualne), ale Phase 3 nie odwołuje się do `flags` — operuje na `slot.Data` po świeżym `resolveAfterRegs`, więc refresh wewnątrz `revealDLCMap` nie jest potrzebny.

Caller, który chce użyć `flags` po `revealDLCMap`, musi sam zrobić refresh — przykład w `app_pvp.go:94` (`flags = slot.Data[slot.EventFlagsOffset:]` po `revealBaseMap`+`revealDLCMap`, przed modułami `SummoningPools` / `Colosseum`). `RevealAllMap` nie potrzebuje refreshu, bo nie używa `flags` po delegacji.

## 13. Validation and safety notes

### 13.1 Ryzyko stale koordynatów po patchu

Wartości `9648/9124` i `3037/1869/7880/7803` są empiryczne — kopiowane z dwóch slotów referencyjnych. Nic w kodzie ani w `regulation.bin` ich nie waliduje. Jeśli przyszły patch gry zmieni map geometry DLC albo logic interpretacji BloodStain coordynatów, Phase 3 może:

- przestać zdejmować czarne kafelki (no‑op behavior),
- ustawić gracza w niepoprawnej lokalizacji jeśli koordy są też traktowane jako last‑bloodstain pozycja respawn,
- spowodować artefakty wizualne (partial reveal).

Mitigation: oznaczyć stałe `DLCTile*` jako game‑version dependent przy detection patcha (brak takiej detekcji w aktualnym kodzie). `needs verification`.

### 13.2 Ryzyko „nadpisana prawdziwa eksploracja”

Jeśli gracz ma save z **częściowo** zbadanym DLC (autentyczna eksploracja), Phase 3 zeruje `[0x0085..0x0110)` (zakres od `DLCTileZeroStart=0x0088` wzwyż) i nadpisuje syntetycznymi wartościami z slotu 0/1 referencyjnego. Oryginalna pozycja ostatniego Bloodstain / kotwica eksploracji jest **tracona bez ostrzeżenia**.

`needs verification` — czy nadpisana sekcja BloodStain ma efekty inne niż wizualne (np. respawn point, last‑grace anchor). CHANGELOG odnotowuje hipotezę „ostatnia plama krwi? punkt odrodzenia? kotwica odkrywania mapy?” jako nierozstrzygniętą.

### 13.3 Ryzyko DLC ownership mismatch

Patrz §10. Edytor zapisuje DLC reveal bez sprawdzania `CSDlc` (`DlcSectionOffset = SlotSize - 0xB2`). Gracz, który nie ma DLC, otrzyma save z ustawionymi DLC flagami + 5 DLC items + zsyntetyzowanym BloodStain. Wpływ na launchowanie gry bez DLC — **nieprzetestowany**.

### 13.4 Ryzyko platform/version differences

Layout `BloodStain` (i tym samym offsety `DLCTile*`) zakładany jest identyczny dla:

- PC PS4 wersji save'a (różne magic bytes, ale ten sam slot layout — patrz [01-header.md](01-header.md) i [49-ps4-zstd-rawblock-patch.md](49-ps4-zstd-rawblock-patch.md)),
- różnych wersji `regulation.bin`.

`needs verification` — brak izolowanego testu `TestDLCTileLayoutPS4` ani `TestDLCTileLayoutAcrossVersions`.

### 13.5 Visual reveal vs gameplay progression

Phase 3 zdejmuje **wizualną** warstwę L2 (czarne kafelki) bez wpływu na progres rozgrywki DLC:

- nie ustawia flag postępu questów DLC,
- nie odblokowuje miejsc Grace w DLC (patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md)),
- nie modyfikuje `UnlockedRegions` dla regionów DLC (warstwa L0 — patrz [11-regions.md](11-regions.md)).

`needs verification` — czy gra po zdjęciu L2 traktuje region DLC jako „odwiedzony” w kontekście trofeów / achievementów. Domyślne założenie: nie, bo trofea zwykle bazują na osobnych flagach postępu, ale brak izolowanego testu.

### 13.6 Brak in‑game verification w CI

Phase 3 zostało zweryfikowane in‑game manualnie (autor + branch SOLVED). Brak automatycznej weryfikacji „save edytora → załaduj w grze → screenshot mapy DLC → diff” w testach. Każdy nontrivialny patch tego flow wymaga manualnego re‑testu in‑game.

## 14. Test coverage

| Test | Plik | Co pokrywa |
|---|---|---|
| `TestBSTLookupMatchesEventFlags` | `tests/map_flags_test.go:12` | Pośrednio dotyka DLC visible flags (`62080..62084`); weryfikuje BST lookup, NIE Phase 3 BloodStain |
| `TestGetAllMapEntries` | `tests/map_flags_test.go:53` | Liczy entries map; nic o DLCTile |
| `TestMapFlagsRoundtrip` | `tests/map_flags_test.go:96` | Roundtrip set/get pojedynczej flagi; nic o BloodStain |
| (`pvp_test.go`) | `pvp_test.go` | Komentarz wprost: „revealBaseMap/revealDLCMap (RevealMap) will fail — those are tested via …” → wskazuje na BRAK pokrycia w pvp_test |

**Wniosek**: brak izolowanego testu jednostkowego/integracyjnego dla `revealDLCMap` Phase 3. Brak testu, który załadowałby save referencyjny, wywołał `revealDLCMap`, i zweryfikował, że bajty w `[0x0088..0x0110)` zgadzają się ze specyfikacją.

`needs verification`: dodanie testu typu `TestRevealDLCMapPhase3WritesExpectedBytes` (load save → `revealDLCMap` → assert na `DLCTileRec1/Rec2` bajtach po `afterRegs`) byłoby pożądane, ale nie istnieje na branchu `main`.

## 15. Known limits / needs verification

Lista otwartych pytań (skondensowana z CHANGELOG + samodzielnej analizy):

1. **Charakter koordynatów** — czy `9648/9124` i `3037/1869/7880/7803` reprezentują last‑bloodstain, respawn anchor, czy „discovery anchor”? `needs verification`.
2. **Tolerancja wartości** — czy gra akceptuje dowolne koordy z obszaru DLC, czy wymaga konkretnie tych? `needs verification`.
3. **Stabilność po patchu gry** — brak gameVersion‑gating. `needs verification`.
4. **DLC ownership without DLC installed** — `needs verification`, jak gra reaguje na `revealDLCMap` w save'ie gracza bez DLC.
5. **Cross‑platform layout** — `needs verification`, czy offsety `DLCTile*` są identyczne w PS4 i PC slotach.
6. **Częściowo zbadane DLC by gracza** — Phase 3 nadpisuje autentyczne dane BloodStain. `needs verification`, czy oryginalna lokalizacja Bloodstain ma efekty poza wizualnymi.
7. **62002 izolowany efekt** — `needs verification`, czy ustawienie samego `62002`+`82002` (bez Phase 3) usuwa hard‑blackout. Historycznie ❌, ale brak isolated testu.
8. **Trofea / achievementy** — `needs verification`, czy zdjęcie L2 wpływa na progres trofeów map DLC.
9. **Rollback per‑warstwa** — brak per‑Phase atomic rollback; jedyna ścieżka cofnięcia to snapshot‑undo całego slotu. `as designed`, ale warto udokumentować.
10. **Automatyczna detekcja patcha** — kod nie ma żadnego mechanizmu „aktualne stałe są stale”. `needs verification` po każdym dużym patchu gry.

## 16. Cross‑references

- [11-regions.md](11-regions.md) — L0 UnlockedRegions; baza `afterRegs` zależy od długości tej tablicy.
- [15-event-flags.md](15-event-flags.md) — generic event flag helper API; `IsDLCMapFlag`, `SetEventFlag` semantyka.
- [27-map-reveal.md](27-map-reveal.md) — master 4‑layer Map Reveal; ten rozdział jest detail chapter dla L2.
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — DLC graces; **nie** są ustawiane przez `revealDLCMap`.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — drugi caller `revealDLCMap` przez `opts.RevealMap`.
- [50-item-companion-flags.md](50-item-companion-flags.md) — companion flag policy dla Map Fragment items.

## 17. Sources

- `app_world.go` — `RevealAllMap`, `revealBaseMap`, `revealDLCMap`, `resolveAfterRegs`, `putF32`, `ResetMapExploration`.
- `app_pvp.go` — `PrepPvP` (`opts.RevealMap`).
- `backend/core/offset_defs.go:303‑323` — stałe `DLCTile*`, `DlcSectionOffset`, `DlcEntryFlagByte`.
- `backend/db/data/maps.go` — `MapSystem`, `MapVisible`, `MapFragmentItems`, `IsDLCMapFlag`.
- `frontend/src/components/WorldTab.tsx` — `handleRevealAllMap`, `RiskActionButton riskKey="map_reveal_full"`.
- `tests/map_flags_test.go` — testy event flag (NIE Phase 3).
- `docs/CHANGELOG.md` — wpis "Branch: fix/dlc-map-reveal-v2 — DLC black tile removal (SOLVED)" oraz historia binary search (2025‑04‑25).
