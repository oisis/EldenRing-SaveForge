# 14 — Game State

> **Type**: Binary format spec + canonical chapter
> **Scope**: Game / profile / progression state w SaveForge — `PreEventFlagsScalars` (29 B), `ClearCount` (NG+ cycle), powiązane write paths przez `CharacterViewModel`, oraz wyraźny rozgraniczenie od World State ([16](16-world-state.md)).

Cross-refs: [01-header.md](01-header.md), [15-event-flags.md](15-event-flags.md), [16-world-state.md](16-world-state.md), [17-player-coordinates.md](17-player-coordinates.md), [19-weather-time.md](19-weather-time.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md), [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md), [50-item-companion-flags.md](50-item-companion-flags.md), [planned/38-boss-multiflag.md](planned/38-boss-multiflag.md).

---

## 1. Cel rozdziału

Zdefiniować jednoznacznie:

- co SaveForge traktuje jako **Game State** (`PreEventFlagsScalars` 29 B + `ClearCount` + powiązane pola w `slot.Player`),
- które pola są write-capable przez `SaveCharacter` / `CharacterViewModel`,
- które pola są read-only (parse + roundtrip verbatim bez settera),
- jaka jest relacja do World State ([16](16-world-state.md)) i Event Flags ([15](15-event-flags.md)),
- co jest planned/research-only (boss multi-flag → [planned/38](planned/38-boss-multiflag.md)).

Nie powiela szczegółów event flag helper API (patrz [15](15-event-flags.md)), World State subsystem map (patrz [16](16-world-state.md)), ani Sites of Grace activation flow (patrz [47](47-site-of-grace-activation.md)).

## 2. Status

| Aspekt | Status |
|---|---|
| `PreEventFlagsScalars` struct parser (29 B) | ✅ `backend/core/section_eventflags.go:27-91` |
| `PreEventFlagsScalarsSize = 29` | ✅ `section_eventflags.go:41` (3+4+4+1+4+4+1+4+4) |
| `ClearCount` parsing (NG+ cycle) | ✅ `backend/core/structures.go:314-318` (dynamic chain offset) |
| `ClearCount` write path | ✅ przez `SaveCharacter` → `vm.ApplyVMToParsedSlot` (cap 7) + event flag sync 50-57 |
| Player stats / Level / Souls write | ✅ przez `SaveCharacter` (patrz §13) |
| `LastRestedGrace` write endpoint | ❌ **brak** — explicit komentarz w `app_world.go:41` „Does not touch LastRestedGrace — the game updates that automatically on arrival” |
| `TotalDeathsCount` write endpoint | ❌ brak — read-only (verbatim roundtrip) |
| `CharacterType`, `InOnlineSessionFlag`, itp. | ❌ brak — read-only verbatim |
| Boss multi-flag editor | ❌ **planned only** — patrz [planned/38](planned/38-boss-multiflag.md) |
| Frontend UI dla NG+ (`clearCount`) | ✅ `DatabaseTab.tsx` (item caps scaling) — read-only display, NIE editor |
| Test coverage | ✅ `section_eventflags_test.go`, `section_world_geom_test.go`, `pvp_test.go`, `tests/map_flags_test.go` (round-trip + EventFlagsOffset correctness) |

## 3. Source of truth w kodzie

| Plik / symbol | Co zawiera | Tryb |
|---|---|---|
| `backend/core/section_eventflags.go::PreEventFlagsScalars` (linie 27-39) | 29 B struct: 11 pól (GameMan x3 + scalars) | read/write verbatim |
| `backend/core/section_eventflags.go::PreEventFlagsScalarsSize` (linia 41) | `= 29` | const |
| `backend/core/section_eventflags.go::Read/Write` (linie 43-106) | Parser + serializer per pole | RW |
| `backend/core/section_eventflags.go::EventFlagsByteCount = 0x1BF99F` | Rozmiar bitfield event_flags (poza Game State scope, ale sąsiadujący) | const |
| `backend/core/structures.go::Player.ClearCount` (linia 214) | `uint32` — NG+ cycle (0=NG, 1-7=NG+N) | RW przez VM |
| `backend/core/structures.go::SaveSlot.ClearCountOffset` (linia 243) | Offset w dynamic chain (`horse + DynClearCount`) | computed |
| `backend/core/structures.go::ClearCount read` (linie 314-318) | Read + cap @ 7 | parser |
| `backend/core/offset_defs.go::DynClearCount = 0x44` (linia 101) | Offset relatywny od horse do ClearCount | const |
| `backend/vm/character_vm.go::CharacterViewModel.ClearCount` (linia 82) | Pole VM | bridge |
| `backend/vm/character_vm.go::ApplyVMToParsedSlot` (cap @ 7 dla ClearCount, linie 319-322) | Cap + write back z `vm.CharacterViewModel` do `slot.Player` | mutator |
| `app.go::SaveCharacter` (linie 178-217) | Wrapper public Wails endpoint | RW |
| `app.go:204-209` | NG+ event flag sync (50-57) po SaveCharacter | side effect |

## 4. Mental model

```
slot.Data
  ├─ ... (inne sekcje)
  ├─ Dynamic chain (post-horse):
  │    ├─ horse  +  DynClearCount (0x44)  →  ClearCount (u32, capped @ 7)
  │    └─ ... (inne dynamic offsets)
  ├─ ... (TutorialData + ostatnie pola)
  ├─ PreEventFlagsScalars (29 B):  ←─── ten rozdział
  │    ├─ GameMan0x8c/0x8d/0x8e   3× u8  (unk)
  │    ├─ TotalDeathsCount        u32
  │    ├─ CharacterType           i32  (0=Host, 1=WhitePhantom, ...)
  │    ├─ InOnlineSessionFlag     u8
  │    ├─ CharacterTypeOnline     u32
  │    ├─ LastRestedGrace         u32  (BonfireId — NIE Event Flag ID!)
  │    ├─ NotAloneFlag            u8
  │    ├─ InGameCountdownTimer    u32
  │    └─ UnkGameDataMan0x124     u32
  ├─ EventFlagsBlock (0x1BF99F + 1 terminator)  ←─── patrz [15]
  └─ ... (inne sekcje — World State, Trailing block, etc.)
```

**Write path dla ClearCount**:

```
User → CharacterViewModel.ClearCount → SaveCharacter(index, vm)
                                          ├─ a.pushUndo(index)                 ← snapshot
                                          ├─ vm.ApplyVMToParsedSlot()
                                          │    ├─ if ClearCount > 7: cap @ 7
                                          │    └─ data.ClearCount = vm.ClearCount
                                          ├─ a.applyMemoryStonesToSlot()
                                          ├─ slot.SyncPlayerToData()           ← flush to slot.Data
                                          ├─ app.go:204-209: SetEventFlag(50+i, i == ClearCount) for i in 0..7
                                          │                                    ← NG+ flags sync
                                          └─ ProfileSummary[index].Level/Name update
```

**Brak write path dla** `LastRestedGrace`, `TotalDeathsCount`, i innych pól `PreEventFlagsScalars` — verbatim roundtrip.

## 5. Game State vs World State

Tabela rozdziału:

| Domain | Co obejmuje | Canonical chapter |
|---|---|---|
| **Game State** (ten rozdział) | Profile postaci, stats, ClearCount/NG+, `PreEventFlagsScalars` (29 B), `LastRestedGrace` jako BonfireId | 14 |
| **World State** | Mapy / regiony / DLC tiles / graces / item flags / PvP presets — wszystko via Event Flags lub UnlockedRegions | [16](16-world-state.md) overview |
| **Event Flags fundament** | Bitfield API (BST + helpers) | [15](15-event-flags.md) |
| **Player Coordinates** | XYZ + MapID + quaternion | [17](17-player-coordinates.md) |
| **Weather / Time** | `WorldAreaWeather` + `WorldAreaTime` (12+12 B) | [19](19-weather-time.md) |
| **Sites of Grace** | Activation flags 71xxx-76xxx, NIE BonfireId | [47](47-site-of-grace-activation.md) |

**Krytyczne rozróżnienie**: `LastRestedGrace` (Game State, BonfireId namespace) ≠ Grace EventFlag ID (World State, 71xxx-76xxx pasmo).

- `LastRestedGrace = 1042362950` → BonfireId Church of Elleh (10-cyfrowy ID przechowywany w `PreEventFlagsScalars`).
- Grace EventFlag 76100 → bit w bitfield event_flags, ustawia widoczność znacznika i fast travel.
- **SaveForge edytuje wyłącznie EventFlag** (przez `SetGraceVisited`, [47](47-site-of-grace-activation.md) §10). `LastRestedGrace` jest read-only.

## 6. Current parsed data

### 6.1 `PreEventFlagsScalars` (29 B)

| Pole | Typ | Rozmiar | Write endpoint | Komentarz |
|---|---|---|---|---|
| `GameMan0x8c` | u8 | 1 B | ❌ brak | unk, runtime GameMan offset |
| `GameMan0x8d` | u8 | 1 B | ❌ brak | unk |
| `GameMan0x8e` | u8 | 1 B | ❌ brak | unk |
| `TotalDeathsCount` | u32 | 4 B | ❌ brak | łączna liczba śmierci postaci |
| `CharacterType` | i32 | 4 B | ❌ brak | `0=Host, 1=WhitePhantom, 2=DarkSpirit, 3=Ghost` (`needs verification`) |
| `InOnlineSessionFlag` | u8 | 1 B | ❌ brak | `0=offline, 1=w sesji` (`needs verification`) |
| `CharacterTypeOnline` | u32 | 4 B | ❌ brak | typ postaci online (`needs verification`) |
| `LastRestedGrace` | u32 | 4 B | ❌ brak (gra zarządza runtime) | BonfireId — patrz §10 |
| `NotAloneFlag` | u8 | 1 B | ❌ brak | `1 = w co-op/invasion` (`needs verification`) |
| `InGameCountdownTimer` | u32 | 4 B | ❌ brak | timer countdown (`needs verification` — semantyka) |
| `UnkGameDataMan0x124` | u32 | 4 B | ❌ brak | unk |

**Suma**: `3 + 4 + 4 + 1 + 4 + 4 + 1 + 4 + 4 = 29 B`.

### 6.2 `ClearCount` (poza `PreEventFlagsScalars`)

- Typ: `uint32`
- Lokalizacja: dynamic chain offset `horse + DynClearCount (0x44)`
- **Maksymalna wartość**: 7 (kod cap'uje przy read i write — `structures.go:316-317`, `character_vm.go:319-322`)
- Mapowanie: `0 = Journey 1 (NG), 1 = NG+1, ..., 7 = NG+7`
- Write endpoint: ✅ `SaveCharacter` przez `vm.CharacterViewModel.ClearCount`

### 6.3 `slot.Player` (szersze Game State)

`slot.Player` zawiera dodatkowo:

- `Name` (`[16]uint16` UTF-16 LE)
- `Level` (uint32)
- `Souls` (uint32)
- `SoulMemory` (uint32, clamped do `runesCostForLevel(level)`)
- `Class` (uint8)
- 8 atrybutów (Vigor/Mind/Endurance/Str/Dex/Int/Faith/Arcane) — uint32 each
- `TalismanSlots` (uint8, capped @ 3)
- `Gender` (uint8)
- `ScadutreeBlessing`, `ShadowRealmBlessing` (uint8 each — DLC)

Wszystkie te pola mają write endpoint przez `SaveCharacter`. Szczegóły layout slotu **nie są** w tym rozdziale — to spec dla `slot.Player` jest poza scope (patrz `backend/core/structures.go`).

`needs verification`: czy `SoulMemory` clamping do `runesCostForLevel(level)` jest game-accurate — kod sygnalizuje minimum, ale brak izolowanego runtime testu, czy gra by zaakceptowała wartość niższą.

## 7. Read-only vs write-capable fields

### 7.1 Write-capable (przez `SaveCharacter` / `CharacterViewModel`)

| Pole | Cap / walidacja | NG+ event flag side effect |
|---|---|---|
| `Player.Name` | 16 znaków UTF-16 | — |
| `Player.Level` | brak | — |
| `Player.Souls` | brak | — |
| `Player.SoulMemory` | clamp ≥ `runesCostForLevel(level)` | — |
| `Player.Class` | brak walidacji | — |
| Vigor/Mind/Endurance/Str/Dex/Int/Faith/Arcane | brak (UI może ograniczać) | — |
| `Player.TalismanSlots` | cap @ 3 | — |
| `Player.ClearCount` | cap @ 7 | ✅ event flags 50-57 sync (app.go:204-209) |
| `Player.ScadutreeBlessing` / `ShadowRealmBlessing` | brak | — |
| `Player.Gender` | brak | — |
| `Player.Inventory` / `Player.Storage` | przez item lifecycle (patrz [50](50-item-companion-flags.md)) | — |

### 7.2 Read-only (parse + roundtrip verbatim, brak settera)

Wszystkie 11 pól `PreEventFlagsScalars` (§6.1) — żadne nie ma public endpointu w `app*.go`. `grep -rn "SetLastRested\|SetTotalDeaths\|SetCharacterType\|SetGameMan" app*.go` zwraca **0 wyników**.

Inne read-only sekcje powiązane z Game State:

| Sekcja | Tryb | Lokalizacja |
|---|---|---|
| `TutorialData` (variable) | RW (append przez `core.AppendTutorialID`) | `backend/core/` |
| `GaItemData` (7000 × 16 B) | RW (auto-zarządzane przez item operations) | `backend/core/` |
| `MenuSaveLoad` (variable) | verbatim | `backend/core/` |
| `TrophyEquipData` | verbatim | `backend/core/` |
| `ProfileSummary` (per slot, w `SaveFile`) | RW przez SaveCharacter (Level + Name) | `backend/core/` |
| `WorldGeomBlock` (5 blobów) | verbatim | patrz [16](16-world-state.md) §6.1 |

## 8. ClearCount / NG+ relation

### 8.1 Write flow

```
User UI → vm.CharacterViewModel.ClearCount (0-7)
       → SaveCharacter(index, vm)
            ├─ vm.ApplyVMToParsedSlot(slot)
            │    └─ cap @ 7 (character_vm.go:319-322)
            │    └─ data.ClearCount = vm.ClearCount
            ├─ slot.SyncPlayerToData()
            └─ SetEventFlag(50+i, i == ClearCount) for i in 0..7
                                ← NG+ event flags sync
```

### 8.2 NG+ event flags 50-57

`app.go:204-209` synchronizuje event flags `50, 51, 52, 53, 54, 55, 56, 57` z aktualnym `ClearCount`:

- Flag `50 + ClearCount` jest ustawiana na `true`.
- Pozostałe 7 flag w zakresie 50-57 jest ustawianych na `false`.

To utrzymuje **inwariant**: dokładnie jedna flaga z `50..57` jest SET w danym momencie, odpowiadająca aktualnemu NG+ cycle.

`needs verification`: czy gra używa flag `50..57` do określenia NG+ cycle, czy tylko `ClearCount` field. Komentarz w `app.go:204` mówi „Sync NG+ event flags (50-57) with ClearCount" — implikacja: gra czyta obie ścieżki, więc sync jest defensywny.

### 8.3 NG+ side effects

Edycja `ClearCount` z 0 → N (lub N → M) **nie odwraca**:

- Pokonanych bossów (event flags w pasmach bossowych — patrz [planned/38](planned/38-boss-multiflag.md)).
- Ukończonych questów.
- Pozyskanych Map Fragmentów.
- Aktywowanych Sites of Grace.
- Otwartych regionów (UnlockedRegions — patrz [11](11-regions.md)).

**Konsekwencja**: NG+ value może być rozsynchronizowane z faktycznym state of progression flagów. Gra prawdopodobnie ufa `ClearCount` jako głównego counter'a (`needs verification`).

Mitigation: brak. SaveForge **nie waliduje** spójności ClearCount ↔ flags progression.

## 9. LastRestedGrace / BonfireId relation

### 9.1 Field

- Lokalizacja: `PreEventFlagsScalars.LastRestedGrace` (`u32`, offset +0x10 w bloku 29 B)
- Format: **BonfireId** — 10-cyfrowy ID (np. `1042362950` = Church of Elleh, `1042362951` = The First Step)
- **NIE** jest Event Flag ID (zakres 71xxx-76xxx z [47](47-site-of-grace-activation.md))

### 9.2 Write path

**Brak.** Explicit komentarz w `app_world.go:41`:

```
// Does not touch LastRestedGrace — the game updates that automatically on arrival.
```

`SetGraceVisited` (z [47](47-site-of-grace-activation.md) §10) ustawia **wyłącznie** event flag gracji + opcjonalny DoorFlag + companion flags — **nie** zmienia `LastRestedGrace`. Gra sama nadpisuje to pole przy fizycznym dotknięciu / teleportacji do innej gracji.

### 9.3 Drugi BonfireId offset

Empiryczna obserwacja (CHANGELOG 2026-05-09): drugi wystąpienie BonfireId w slot data zaobserwowane pod offset ≈`0x1F636A` w jednym slocie testowym, ~1300 B za końcem EventFlags. Aktualizuje się identycznie z `LastRestedGrace`.

`needs verification`: czy ten drugi offset jest stabilny cross-slot/cross-platform, czy artifact konkretnego save'a. Aktualnie SaveForge **nie parsuje** go jako osobne pole — jest częścią surowych bajtów sekcji `NetworkManager`. Patrz [16](16-world-state.md) §3.

### 9.4 Mapowanie BonfireId ↔ Grace EventFlag ID

**Brak w aktualnym kodzie**. Nie ma publicznej tabeli ani API `GetBonfireIDForGrace(eventFlagID)` ani odwrotnej. Te dwie przestrzenie nazw są rozłączne w SaveForge.

`needs verification`: czy w `regulation.bin` istnieje sparowana tabela (`BonfireParam.csv` lub podobna). Aktualnie nie sparsowana.

## 10. Event Flags relation

`PreEventFlagsScalars` (29 B) sąsiaduje bezpośrednio z bitfield `EventFlagsBlock` (`EventFlagsByteCount = 0x1BF99F`). Komentarz w `section_eventflags.go:10-11`:

> PreEventFlagsScalars — block of scalar fields between TutorialData and the event_flags bitfield.

Funkcjonalnie:

- `PreEventFlagsScalars` to **Game State scalars** (per-character progress counters).
- Event Flags to **World State bits** (per-world boolean state).

Ten rozdział **nie powiela** Event Flags helper API (BST resolver, byte/bit indexing) — patrz [15](15-event-flags.md).

Side effect: `app.go:204-209` synchronizuje 8 event flags (50-57) z `ClearCount` — bridge między Game State scalar a Event Flags bits.

## 11. Sites of Grace relation

Patrz [47](47-site-of-grace-activation.md). Krytyczne rozróżnienie:

| Aspekt | Game State (ten rozdział) | Sites of Grace ([47](47-site-of-grace-activation.md)) |
|---|---|---|
| Pole | `LastRestedGrace` (BonfireId) | event flags 71xxx-76xxx |
| Namespace | BonfireId (10-digit format) | Event Flag ID (`data.Graces` keys) |
| Write endpoint | ❌ brak (gra zarządza runtime) | ✅ `SetGraceVisited` |
| Co kontroluje | Spawn point po śmierci, "ostatnio odpoczęto" w menu pauzy | Znacznik mapy + fast travel + quest gates |
| Companion flag fan-out | n/d | ✅ tylko Gatefront (patrz [47](47-site-of-grace-activation.md) §8) |
| Cross-mapping | brak w SaveForge | brak (`needs verification`) |

## 12. Boss / progression flags

Boss state w aktualnym kodzie:

- **Read endpoint**: `GetBosses(slotIndex)` w `app_world.go:87` — zwraca pełną listę bosses z `data.Bosses` z `Defeated=true/false` z bitfield event flags.
- **Write endpoint**: `SetBossDefeated(slotIndex, bossID, defeated)` w `app_world.go:113` — pojedyncza event flag SET/CLEAR.

To jest **single-flag editor** — działa na pojedynczych event flagach per boss.

**Boss multi-flag editor** (kompleksowa sync wielu flag per boss, np. cutscene flags + memory flags + reward flags) jest **planned only**:

- Patrz [planned/38-boss-multiflag.md](planned/38-boss-multiflag.md).
- Aktualny kod NIE ma implementacji multi-flag sync.
- UI w `WorldTab.tsx` używa wyłącznie single-flag `SetBossDefeated`.

`needs verification`: czy gra przy "fake defeat" przez single event flag re-triggeruje cutscenę bossa lub questy. Stara doc i CHANGELOG sugerują "tak dla niektórych bossów" — boss multi-flag editor planowany jako fix.

## 13. Current implemented behavior

### 13.1 `SaveCharacter(index, vm)`

`app.go:178-217`:

1. Validuje `a.save != nil` i `index` ∈ [0, 10).
2. **`a.pushUndo(index)`** (linia 186) — snapshot przed wszystkimi mutacjami.
3. Wywołuje `vm.ApplyVMToParsedSlot(&charVM, &a.save.Slots[index])` — flush VM fields do `slot.Player` (m.in. cap ClearCount @ 7).
4. Aplikuje MemoryStones przez `a.applyMemoryStonesToSlot(slot, charVM.MemoryStones)`.
5. `slot.SyncPlayerToData()` — flush `slot.Player` → `slot.Data` (binary serialize).
6. **Event flag sync NG+ 50-57** (`app.go:204-209`) — error swallowed.
7. Aktualizuje `ProfileSummary[index].Level` + `.CharacterName`.

`SaveCharacter` ma single‑snapshot rollback przez user Undo (snapshot z `pushUndo`). Per‑step rollback nie istnieje — jeśli `ApplyVMToParsedSlot` lub `applyMemoryStonesToSlot` zwróci error, kod robi `return err` bez auto‑restore, ale snapshot pozostaje na stosie Undo dla user-initiated rollback.

`needs verification`: czy snapshot z `pushUndo` jest zachowany na stosie nawet po `return err` (typowo tak — `pushUndo` jest synchronous push, nie wrapped w defer/conditional pop).

### 13.2 `GetCharacter(index)`

`app.go:164-176` — read-only, mapuje `slot.Player` + flag values na `CharacterViewModel`.

### 13.3 Read-only verbatim roundtrip

11 pól `PreEventFlagsScalars` jest preserved przez `Read`/`Write` per pole. Test `section_eventflags_test.go` weryfikuje round-trip (parse → serialize → diff = 0).

## 14. Planned / research-only areas

| Obszar | Status | Lokalizacja |
|---|---|---|
| Boss multi-flag editor | planned | [planned/38-boss-multiflag.md](planned/38-boss-multiflag.md) |
| `LastRestedGrace` write endpoint | brak — gra zarządza runtime, brak planowanej zmiany | n/d |
| `TotalDeathsCount` / `CharacterType` / inne `PreEventFlagsScalars` write | brak — read-only | n/d |
| BonfireId ↔ Grace EventFlag mapping | brak — nie sparsowane z `regulation.bin` | `needs verification` |
| NG+ progression validator (ClearCount vs flags consistency) | brak | n/d |
| Boss/quest re-trigger detection po `ClearCount` change | brak | `needs verification` |
| `InGameCountdownTimer` semantyka | nieznana | `needs verification` |
| Drugie wystąpienie BonfireId w NetworkManager | empirycznie zaobserwowane, nie parsowane | patrz [47](47-site-of-grace-activation.md) §10 |

## 15. Write path and rollback caveats

### 15.1 `SaveCharacter` jako single transaction

`SaveCharacter` wykonuje sekwencję mutacji (`ApplyVMToParsedSlot` → MemoryStones → SyncPlayerToData → event flag sync → ProfileSummary update) **pod jednym `pushUndo` snapshotem** (`app.go:186`). User-initiated Undo cofa **wszystkie** zmiany do stanu pre‑`SaveCharacter`.

Jeśli któryś z kroków failuje (np. `applyMemoryStonesToSlot` zwróci error), kod robi `return err` bez auto‑restore — wcześniejsze mutacje zostają w `slot.Player` i prawdopodobnie w `slot.Data`. Snapshot pozostaje na stosie Undo, więc **user może ręcznie cofnąć** (`needs verification` — patrz §13.1 nota o stack behavior po `return err`).

### 15.2 Cap @ 7 dla ClearCount

`character_vm.go:319-322` zawsze cap'uje ClearCount na 7 przy write — nawet jeśli UI prześle 99. Jest to bezpieczne ograniczenie (gra ma NG+0 do NG+7).

### 15.3 Event flag sync 50-57

`app.go:204-209` ustawia 8 flag (50, 51, ..., 57) — jedną SET, siedem CLEAR — bazując na `ClearCount`. Brak walidacji `EventFlagsOffset` w tym bloku poza początkowym `if slot.EventFlagsOffset > 0`.

Jeśli `EventFlagsOffset` jest valid, ale `SetEventFlag(50+i, ...)` failuje (np. resolver error), error jest **ignorowany** (`_ = db.SetEventFlag(...)`). NG+ event flag sync może pozostać częściowo niespójny po `SaveCharacter`.

### 15.4 No hash recalculation w SaveCharacter

`SaveCharacter` nie wywołuje `RebuildSlot` ani hash recalculation. Te są wywoływane w `WriteSave` (a.go:220) przy zapisie pliku — Save → Write Save jest dwustopniowy.

`needs verification`: czy częściowo zmutowany slot.Data po SaveCharacter (bez subsequent WriteSave) może corruption save'a przy następnym Load. Round-trip testy pokrywają pełen Save→Write→Load loop, nie partial state.

### 15.5 No multi-field consistency validator

SaveCharacter **nie sprawdza** spójności typu „ClearCount > 0 → niektóre bossy powinny być defeated” ani „Souls > requiredForLevel". Walidacja jest **fail-open** (nawet jeśli UI ostrzega, backend akceptuje).

## 16. Validation and safety notes

### 16.1 ClearCount progression desync

Edycja `ClearCount` z 0 → 7 **nie**:

- Pokonuje bossów dla NG+0..6.
- Resetuje quest progression.
- Resetuje pozycji bossów (Roundtable Hold dialogs).

Gra może w środku NG+7 nie znaleźć oczekiwanych flag z poprzednich cykli → soft-lock niektórych quest lines lub odblokowanie scaled enemies bez progression rewards.

`needs verification`: dokładny boss/quest impact w runtime.

### 16.2 Flag mismatch po ClearCount edit

NG+ event flags 50-57 są syncowane (§8.2), ale **boss event flags** w pasmach defeat-id (per boss arena) **nie są**. Gra może próbować re-trigger bossa, którego flag pozostał z poprzedniego NG.

### 16.3 LastRestedGrace invalid BonfireId

`LastRestedGrace` jest read-only w SaveForge. Jeśli zewnętrzne narzędzie zmieni je na nieprawidłowy BonfireId:

- Gra może crash przy load (próba spawn w nieistniejącym grace).
- Gra może fallback do default starting grace.

`needs verification`: jaki dokładnie fallback gra wykonuje.

### 16.4 PreEventFlagsScalars verbatim — co jeśli się zepsuje

Pola `GameMan0x8c/0x8d/0x8e`, `UnkGameDataMan0x124` są verbatim, ale ich semantyka jest nieznana. Każda mutacja zewnętrzna jest ryzykiem nieprzewidywalnym.

`InGameCountdownTimer` — `needs verification` co kontroluje (game timer? quest timer? death respawn delay?).

### 16.5 Platform / version differences

`PreEventFlagsScalarsSize = 29` jest identyczne PC i PS4. Brak walidacji per-platform offsetów dla `LastRestedGrace`.

`ClearCount` offset w dynamic chain (`horse + DynClearCount = 0x44`) może się różnić binarnie między platformami (`needs verification` — brak izolowanego testu).

### 16.6 In-game verification gaps

Brak CI testów typu „set ClearCount = 5 → reload save in-game → assert NG+5 displayed in menu". Wszystkie verifikacje są runtime ad-hoc (CHANGELOG entries).

### 16.7 Rollback limits

`SaveCharacter` **ma** `pushUndo(index)` (`app.go:186`) — single snapshot pokrywający całą operację. Brak per-field undo wewnątrz `SaveCharacter`.

Subsequent `WriteSave` zapisuje plik na dysk (z backupem `.sl2.YYYYMMDD_HHMMSS.bkp`). User-facing rollback ścieżki:

1. **Undo stack** (in-memory) — działa do `WriteSave` lub aplikacji restart.
2. **Backup file restore** — po `WriteSave` jedyna ścieżka cofnięcia.

## 17. Test coverage

| Test | Plik | Co weryfikuje |
|---|---|---|
| Round-trip `PreEventFlagsScalars` + `EventFlagsBlock` | `backend/core/section_eventflags_test.go:11+` | Parse + serialize identity |
| Round-trip slot integrity | `backend/core/section_world_geom_test.go:18+` | `PreEventFlagsScalars` parsed po `Read` |
| `TestEventFlagsOffsetCorrectness` | `tests/map_flags_test.go:145` | EventFlagsOffset wskazuje na poprawny bitfield (poprzez weryfikację known graces visited) |
| `pvp_test.go` | `pvp_test.go` (10 testów) | Patrz [48](48-pvp-ready-modular-presets.md) §17 — w tym roundtrip event flag |

**Brak**:

- Izolowanego testu `SaveCharacter` z ClearCount = N → assert NG+ event flags 50-57 sync.
- Walidacji ClearCount cap @ 7 na poziomie API public (test poziomu VM tak, public API nie).
- Cross-platform parity testu dla ClearCount offset.
- Testu rollback po failure w środku `SaveCharacter`.

## 18. Known limits / needs verification

1. **`CharacterType` i32 wartości** — komentarz w starej doc był „0=Host, 1=WhitePhantom, 2=DarkSpirit, 3=Ghost"; aktualny kod nie waliduje tego mapowania. `needs verification`.
2. **`InGameCountdownTimer` semantyka** — kompletnie nieznana, prawdopodobnie game session timer lub quest timer.
3. **`NotAloneFlag` boolean ranges** — `needs verification` co ustawia (co-op, invasion, NPC summon?).
4. **`UnkGameDataMan0x124`** — kompletnie nieznane.
5. **Drugie wystąpienie BonfireId w NetworkManager** — empiryczne, nie parsowane jako field.
6. **Boss multi-flag editor** — planned, patrz [planned/38](planned/38-boss-multiflag.md).
7. **BonfireId ↔ Grace EventFlag mapping** — brak w SaveForge.
8. **ClearCount progression validator** — brak; możliwe boss/quest desync.
9. **NG+ event flags 50-57 jako single source of truth** — `needs verification` czy gra używa flag czy `ClearCount` field.
10. **PreEventFlagsScalars unknown fields** — 4 z 11 pól nieznane.
11. **Platform parity ClearCount offset** — brak izolowanego testu.
12. **`SaveCharacter` rollback po partial mutation** — brak; user musi restore z backup.

## 19. Cross-references

- [01-header.md](01-header.md) — SteamID w `TrailingFixedBlock`; nie część Game State, ale powiązany identifier.
- [15-event-flags.md](15-event-flags.md) — bitfield event flags i NG+ flagi 50-57.
- [16-world-state.md](16-world-state.md) — overview World State; Game State jest osobną domeną.
- [17-player-coordinates.md](17-player-coordinates.md) — `PlayerCoordinates` oddzielna sekcja read-only.
- [19-weather-time.md](19-weather-time.md) — `WorldAreaWeather` / `WorldAreaTime` read-only blobs.
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — Grace EventFlag (71xxx-76xxx) NS, rozłączny z BonfireId.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — PvP modular presets (orchestrator world state, nie Game State).
- [50-item-companion-flags.md](50-item-companion-flags.md) — item lifecycle hooks, side-effect na event flagi.
- [planned/38-boss-multiflag.md](planned/38-boss-multiflag.md) — planowany boss multi-flag editor.

## 20. Sources

- `backend/core/section_eventflags.go` — `PreEventFlagsScalars` (29 B), `EventFlagsBlock`.
- `backend/core/structures.go` — `Player.ClearCount`, `SaveSlot.ClearCountOffset`.
- `backend/core/offset_defs.go` — `DynClearCount = 0x44`.
- `backend/vm/character_vm.go` — `CharacterViewModel.ClearCount` + cap @ 7 w `ApplyVMToParsedSlot`.
- `app.go::SaveCharacter` (linie 178-217) — write path + NG+ event flag sync (linie 204-209).
- `app_world.go:41` — explicit komentarz „Does not touch LastRestedGrace".
- `frontend/src/components/DatabaseTab.tsx` — `clearCount` jako read-only input dla item caps scaling.
- Tests: `backend/core/section_eventflags_test.go`, `section_world_geom_test.go`, `tests/map_flags_test.go`, `pvp_test.go`.
