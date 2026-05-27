# 15 — Event Flags (Flagi Zdarzeń)

> **Typ**: Binary format spec + helper API reference
> **Status**: ✅ canonical (Phase 4 Step 1 — 2026-05-21)
> **Zakres**: Fundament event-flag storage + adresowania + helper API. Specyficzne ID-y (mapa, łaski, bossowie, item companion, PvP) są w pochodnych rozdziałach — patrz §18 Cross-references.

---

## 1. Cel rozdziału

Event Flags to główny mechanizm progresji gry — bitfield o stałym rozmiarze `0x1BF99F` bajtów (1 833 375 B, ~1.75 MB) w każdym slocie. Każda flaga to pojedynczy bit kontrolujący jeden aspekt stanu gry (boss pokonany, NPC quest stage, przedmiot podniesiony, mapa odkryta, cutscena obejrzana, mechanika odblokowana itd.).

Ten rozdział opisuje **wyłącznie fundament**:

- gdzie flagi są przechowywane (`EventFlagsBlock`, slot offset)
- jak kod adresuje pojedynczą flagę po jej ID (3-poziomowy resolver)
- byte/bit indexing (big-endian bit order)
- helper API (`db.GetEventFlag`, `db.SetEventFlag`)
- semantyka set/get (idempotent vs SET-only — zależnie od feature)
- gdzie szukać domenowych ID-ów (cross-refs)

Ten rozdział **nie** zawiera:

- tabel ID-ów map / łask / bossów / przedmiotów → patrz odpowiednie rozdziały
- semantyki feature-specific (companion flags items vs graces) → patrz 50 / 47
- detali map reveal / PvP modules → patrz 27 / 48

---

## 2. Status

- ✅ Storage layout (`0x1BF99F` + 1B terminator) — hex-verified PS4/PC.
- ✅ Resolver tier-3 (precomputed → BST → fallback) — pokryty testem `TestBSTLookupMatchesEventFlags`.
- ✅ Helper API (`GetEventFlag`/`SetEventFlag`) — round-trip stable, używany przez 20+ callsites w `app_world.go`.
- ✅ BST embed (`eventflag_bst.txt`, 11 919 wpisów — snapshot 2026-05-21) — generowany offline ze skryptu, embedded via `//go:embed`.
- ✅ Precomputed lookup (`event_flags.go`, 865 wpisów `EventFlagInfo{Byte, Bit}` — snapshot 2026-05-21).
- ⚠️ Boss multi-flag editing — **nie zaimplementowany** (`SetEventFlag` ustawia pojedynczy bit; multi-flag jest planowany w `38-boss-multiflag.md`).
- ⚠️ PvP modular preset moduł E (Sites of Grace) — **nie aktywny** (warning „planned but not enabled" w `app_pvp.go:109`).
- ⚠️ Summoning pools moduł D — flagi się ustawiają, ale gra runtime nie honoruje (patrz `42-summoning-pools-bug.md`).

**Ostatnia weryfikacja**: 2026-05-21 na `tmp/save/ER0000.sl2` (PC) + `tmp/save/oisis_pl-org.txt` (PS4) via round-trip tests.

---

## 3. Source of truth w kodzie

| Komponent | Plik / linie | Notatka |
|---|---|---|
| Helper read | `backend/db/db.go::GetEventFlag` | Resolver + bit test |
| Helper write | `backend/db/db.go::SetEventFlag` | Resolver + bit set/clear |
| Position resolver | `backend/db/db.go::resolveEventFlagPosition` | 3-tier (precomputed → BST → fallback) |
| Precomputed table | `backend/db/data/event_flags.go` | `var EventFlags map[uint32]EventFlagInfo` — 865 wpisów |
| BST table | `backend/db/data/bst.go::EventFlagBST` + `eventflag_bst.txt` | 11 919 wpisów, `BSTBlockSize=125`, `BSTFlagDivisor=1000` |
| Container struct | `backend/core/section_eventflags.go::EventFlagsBlock` | `EventFlagsByteCount = 0x1BF99F`, `EventFlagsBlockSize = 0x1BF9A0` |
| Slot integration | `backend/core/structures.go::Slot.EventFlagsOffset` | `int` offset w `slot.Data`; może być 0 jeśli chain unreachable |
| Tests (round-trip) | `backend/core/section_eventflags_test.go` | `TestEventFlagsBlockPS4`, `TestEventFlagsBlockPC` |
| Tests (lookup) | `tests/map_flags_test.go::TestBSTLookupMatchesEventFlags` | Weryfikuje że BST-computed positions = precomputed positions |
| Tests (offset) | `tests/map_flags_test.go::TestEventFlagsOffsetCorrectness` | Weryfikuje że `slot.EventFlagsOffset` wskazuje na bitfield |
| Tests (write semantics) | `tests/save_modify_test.go` | `TestEventFlagsBossToggle`, `TestEventFlagsBossKillAll`, `TestEventFlagsGraceToggle`, `TestEventFlagsGraceUnlockAll` |

---

## 4. Mental model

```
slot.Data
  ├── ... wcześniejsze sekcje ...
  ├── PreEventFlagsScalars (29 B)            — GameMan bytes, deaths, LastRestedGrace itd.
  ├── EventFlagsBlock                        — slot.EventFlagsOffset
  │     ├── Flags []byte (0x1BF99F = 1 833 375 B)
  │     └── 1-byte terminator
  └── ... kolejne sekcje ...
```

Flaga jest adresowana po **uint32 ID** (np. `71190` = Roundtable Hold grace). Resolver mapuje ID → `(byteIdx, bitIdx)`:

1. **Precomputed** — `EventFlags[id]` → exact `(Byte, Bit)`. Najszybsze. 865 wpisów dla najczęściej używanych flag.
2. **BST lookup** — block = `id / 1000`. Jeśli `EventFlagBST[block]` istnieje → `bstPos*125 + (id%1000)/8` + `7 - (id%1000)%8`. Pokrywa większość pozostałych flag (gra używa `CSFD4VirtualMemoryFlag` block layout).
3. **Fallback formula** — `byte = id/8, bit = 7 - (id%8)`. Tylko dla małych ID (< 1000) lub jako last-resort.

Test `TestBSTLookupMatchesEventFlags` weryfikuje, że dla każdego ID w precomputed table BST-formula daje ten sam wynik (czyli oba tier-1 i tier-2 są spójne — tier-1 to tylko cache).

---

## 5. Event flag storage model

### 5.1 Container

```go
const EventFlagsByteCount = 0x1BF99F       // 1 833 375 bytes (~1.75 MB)
const EventFlagsBlockSize = EventFlagsByteCount + 1  // + 1-byte terminator

type EventFlagsBlock struct {
    Flags      []byte // length = EventFlagsByteCount
    // 1-byte terminator written/read after Flags
}
```

- **Rozmiar**: stały, nie zmienia się przy edycji.
- **Terminator**: 1 bajt po bitfieldzie (oryginalna wartość zachowana przez round-trip).
- **Endian**: bitfield jest płaską tablicą bajtów; bit order **wewnątrz bajtu** jest big-endian (bit 7 = najwyższy bit ID-modulo-8).

### 5.2 Slot integration

```go
type Slot struct {
    ...
    EventFlagsOffset int  // offset w slot.Data; może być 0 jeśli chain unreachable
}
```

- Offset jest obliczany dynamicznie podczas parsowania (`structures.go:423`):

  ```go
  s.EventFlagsOffset = s.IngameTimerOffset + DynEventFlags
  ```

- Jeśli offset ≥ `SlotSize` → clamped do 0 + warning (`event flags disabled`).
- Wszystkie callsite'y w `app_world.go` używają `slot.Data[slot.EventFlagsOffset:]` jako `flags []byte` argument.

### 5.3 Round-trip invariant

Test `TestEventFlagsBlockPS4` / `PC`:

1. Read `EventFlagsBlock` z save.
2. Marshall do bajtów.
3. Porównaj `expectedSize = PreEventFlagsScalarsSize + EventFlagsBlockSize` (29 + 0x1BF9A0).
4. Byte-equal z oryginałem.

---

## 6. Flag ID addressing — 3-tier resolver

```go
func resolveEventFlagPosition(id uint32) (byteIdx uint32, bitIdx uint8) {
    // Tier 1: Precomputed lookup
    if info, ok := data.EventFlags[id]; ok {
        return info.Byte, info.Bit
    }
    // Tier 2: BST lookup
    data.LoadBST()
    block := id / data.BSTFlagDivisor          // = 1000
    if bstPos, ok := data.EventFlagBST[block]; ok {
        idx := id % data.BSTFlagDivisor
        return bstPos*data.BSTBlockSize + idx/8, uint8(7 - (idx % 8))
    }
    // Tier 3: Fallback formula
    return id / 8, uint8(7 - (id % 8))
}
```

| Tier | Pokrycie | Źródło | Użycie |
|---|---|---|---|
| 1 Precomputed | 865 najczęściej używanych ID | `event_flags.go` — generowane offline | Hot path (boss/grace/map/companion) |
| 2 BST | ~11 919 bloków (każdy 1000 flag) | `eventflag_bst.txt` (embed) | Większość pozostałych ID |
| 3 Fallback | ID < 1000 lub nieznany block | Inline formula | Rzadkie, edge cases |

**Stała `BSTBlockSize = 125`** = 1000 flag / 8 bitów = 125 bajtów na blok BST.
**Stała `BSTFlagDivisor = 1000`** = liczba flag per blok BST.

`LoadBST()` jest idempotent (`sync.Once`) — pierwszy call parsuje embedded TXT (`block,offset` format) do mapy, kolejne są no-op.

---

## 7. Byte and bit indexing

### 7.1 Konwencja

Bity wewnątrz bajtu są adresowane **big-endian**:

```
byte at byteIdx:    [bit7][bit6][bit5][bit4][bit3][bit2][bit1][bit0]
bit ID modulo 8:      0     1     2     3     4     5     6     7
```

Dla BST resolver:

```
idx     = id % 1000               // pozycja flagi w bloku
byteIdx = bstPos*125 + idx/8      // bajt w bloku
bitIdx  = 7 - (idx % 8)           // bit wewnątrz bajtu (big-endian)
```

Dla fallback (ID < 1000):

```
byteIdx = id / 8
bitIdx  = 7 - (id % 8)
```

### 7.2 Przykład

Flaga `71190` (Roundtable Hold grace):

- Tier 1 (precomputed): `event_flags.go` zwraca `{Byte: 0xA58, Bit: 1}` → flaga w bajcie 0xA58, bit 1.
- Tier 2 (BST): block = 71, idx = 190, `bstPos = EventFlagBST[71]`. Dla `bstPos*125 + 190/8 = bstPos*125 + 23`, bit = `7 - (190 % 8) = 7 - 6 = 1`. Musi się zgadzać z Tier 1 (test `TestBSTLookupMatchesEventFlags` to weryfikuje).

### 7.3 Read

```go
flag_value = (flags[byteIdx] & (1 << bitIdx)) != 0
```

### 7.4 Write

```go
if value {
    flags[byteIdx] |= (1 << bitIdx)
} else {
    flags[byteIdx] &= ^(1 << bitIdx)
}
```

---

## 8. Helper API

### 8.1 Sygnatura

```go
// backend/db/db.go
func GetEventFlag(flags []byte, id uint32) (bool, error)
func SetEventFlag(flags []byte, id uint32, value bool) error
```

- **`flags []byte`** — slice bitfield, zwykle `slot.Data[slot.EventFlagsOffset:]`.
- **`id uint32`** — flaga ID (np. 71190).
- **Returns error** — gdy resolver wskazuje byte offset >= len(flags) (out of bounds).

### 8.2 Bounds check

Oba helpery walidują `int(byteIdx) >= len(flags)`:

```go
if int(byteIdx) >= len(flags) {
    return ..., fmt.Errorf("event flag %d (byte %d) out of bounds (flags len %d)", id, byteIdx, len(flags))
}
```

Konsekwencja: gdy slot ma `EventFlagsOffset == 0` (chain unreachable, ostrzeżenie z `structures.go:469`), `flags []byte` może mieć długość 0 → wszystkie wywołania zwrócą błąd. Caller (np. `GetGraces`, `SetBossDefeated`) musi obsłużyć ten warunek.

### 8.3 Brak rollback

`SetEventFlag` jest **stateless byte mutation** — modyfikuje pojedynczy bit w slice'ie. Nie ma snapshot/rollback. Rollback (jeśli potrzebny) musi być po stronie callera (np. `ApplyPvPPreparation` w `app_pvp.go` ma snapshot-based undo na poziomie save'a).

---

## 9. BST / flag block model

### 9.1 Origin

Gra używa `CSFD4VirtualMemoryFlag` (Cheat Engine / RE) jako block-based layout. Block size = 1000 flag = 125 bajtów. Bloki nie są ciągłe — kolejność bloków w pamięci/save nie odpowiada kolejności ID-ów. Mapowanie `block → bstPos` jest ekstrahowane raz i embedded w `eventflag_bst.txt`.

### 9.2 Format pliku

```
block,offset
0,0
1,1
2,2
...
```

- `block` = `id / 1000` (np. 71 dla flag z grupy 71xxx).
- `offset` = pozycja bloku w bitfieldzie (mnożona przez `BSTBlockSize=125` daje byte offset).

11 919 wpisów = ~99 % pokrycia możliwych bloków (kilka bloków jest pustych / unused, brak ich w pliku).

### 9.3 Loader

```go
//go:embed eventflag_bst.txt
var eventflagBSTRaw string

var EventFlagBST map[uint32]uint32

func LoadBST() {
    bstOnce.Do(func() { /* parse eventflagBSTRaw */ })
}
```

- Embed via `//go:embed` (Go 1.16+).
- `sync.Once` → thread-safe lazy init.
- Komentarz mapy mówi `12000` (pre-allocate hint), ale rzeczywista liczba wpisów = 11 919.

### 9.4 Aktualizacja

`eventflag_bst.txt` jest generowany offline (skrypt poza scope `make` — `tmp/scripts/eventflag_bst_build.go` w intencji). Po aktualizacji `regulation.bin` z nowego patcha gry może być wymagana re-extracja.

---

## 10. Generic set/get semantics

Helper API jest **idempotent na poziomie bitu**:

- `SetEventFlag(flags, id, true)` × N razy = jednorazowe ustawienie. Bit pozostaje `1`.
- `SetEventFlag(flags, id, false)` × N razy = bit pozostaje `0`.
- `SetEventFlag(flags, id, true)` po `SetEventFlag(flags, id, false)` = `1` (clean toggle).

**Brak side effects** — `SetEventFlag` nie wywołuje innych funkcji, nie ustawia powiązanych flag, nie wpływa na inne sekcje save'a. Każdy bit jest niezależny.

Side effects (companion flags, door flags, region flags itd.) są realizowane przez **callerów** — zwykle w `app_world.go` (np. `SetGraceVisited` ustawia grace flag + door flag + companion flags w jednym wywołaniu).

---

## 11. Feature-specific policy differences

⚠️ **Bardzo ważne**: helper API jest generic (set/clear/toggle), ale konkretne feature'y mogą mieć **różną politykę** stosowania set/clear. Najważniejsze różnice:

| Feature | Polityka | Rozdział |
|---|---|---|
| Item companion flags (np. Spectral Steed Whistle → 60100, 4680, 710520, 4681) | **SET + CLEAR symmetric** — SET przy add, CLEAR przy remove | [50](50-item-companion-flags.md) |
| Grace companion flags (Gatefront 76111 → te same 4 ID-y) | **SET-only asymmetric** — SET przy visit, NIGDY nie CLEAR | [47](47-site-of-grace-activation.md) |
| Site of Grace `visited` flag (71xxx–76xxx) | Symmetric — caller toggluje swobodnie | [47](47-site-of-grace-activation.md) |
| DoorFlag (auto-open przy SetGraceVisited) | Set razem z grace; CLEAR razem z grace | [47](47-site-of-grace-activation.md) |
| Boss defeat flag (9xxx) | Symmetric pojedynczy bit; ⚠️ pełne defeat wymagałoby multi-flag (38-boss-multiflag.md) | [14](14-game-state.md), [38](38-boss-multiflag.md) |
| Map visible flag (62xxx) | Symmetric; revealBaseMap robi SET dla całej grupy | [27](27-map-reveal.md) |
| Map fragment item-pickup flag (63xxx, 66xxx) | SET-only w aktualnym kodzie (brak CLEAR-on-remove dla container pickup flags) — `needs verification` czy ten brak jest intencjonalny | [27](27-map-reveal.md), [50](50-item-companion-flags.md) |
| Colosseum unlock (60xxx + global 6080/60100/69480) | Symmetric, ale globalne flagi są shared między colosseums | [48](48-pvp-ready-modular-presets.md) |
| Summoning pool activation (670xxx) | Symmetric w save'ie, ale **runtime nie honoruje** | [48](48-pvp-ready-modular-presets.md), [42](42-summoning-pools-bug.md) |
| Ash of War menu unlock (`AoWMenuUnlockedFlag` = 65800) | Symmetric, auto-managed przy whetblade unlocks | (zob. `app_world.go::SetWhetbladeUnlocked`) |

**Zasada**: w `15` nie rozstrzygamy poszczególnych polityk. Każdy domenowy rozdział (47/50/48/27/14) jest source of truth dla swojej polityki.

---

## 12. Selected callers (known usages)

Wybrane callsity `db.SetEventFlag` / `db.GetEventFlag` w aktualnym kodzie (`app_world.go`, 2026-05-21). Tabela **nie jest pełną listą** — patrz §12.2 dla pozostałych plików.

### 12.1 `app_world.go` (40 wywołań — 11× `GetEventFlag`, 29× `SetEventFlag`)

| Caller | Rola | Typowe ID range | Cross-ref |
|---|---|---|---|
| `GetGraces`/`SetGraceVisited` | Read+write grace visited; auto door flag + companion flags | 71000–76960 | [47](47-site-of-grace-activation.md) |
| `GetBosses`/`SetBossDefeated` | Read+write boss defeat (single flag) | 9xxx (+DLC) | [14](14-game-state.md), [38](38-boss-multiflag.md) |
| `GetSummoningPools`/`SetSummoningPoolActivated` | Read+write pool activation | 670xxx | [48](48-pvp-ready-modular-presets.md), [42](42-summoning-pools-bug.md) |
| `GetColosseums`/`SetColosseumUnlocked` | Read+write colosseum + global flags | 60xxx + 6080/60100/69480 | [48](48-pvp-ready-modular-presets.md) |
| `GetCookbooks`/`SetCookbookUnlocked` | Read+write recipe unlock | 67xxx, 68xxx | — |
| `revealBaseMap`/`revealDLCMap`/`ResetMapExploration` | Set map visible / clear; add map fragment items | 62xxx, 63xxx, 82xxx | [27](27-map-reveal.md) |
| `SetMapFlag`/`SetMapRegionFlags` | Set pojedynczy lub region map flag | 62xxx, 63xxx | [27](27-map-reveal.md) |
| `SetWhetbladeUnlocked` | Auto-manage `AoWMenuUnlockedFlag` (65800), Storm Stomp dup (65841) | 65610–65720, 65800, 65841 | (`app_world.go`) |
| `SetBellBearingUnlocked` | Bell bearing trader unlocks | mixed | — |
| `SetGestureUnlocked` | Gesture unlocks | 60800–60849 | — |
| `SetRegionUnlocked` (`app_world.go`) | Region flag toggle (NOT to be confused with `core.SetUnlockedRegions`) | 6100000–6999999 | [11](11-regions.md) |
| `SetQuestStep` | Bulk quest step skip (Tier 1 in `RISK_INFO`) | mixed | [32](32-ban-risk-system.md) |

### 12.2 Pozostałe pliki z `db.GetEventFlag`/`SetEventFlag`

| Plik | Cel | Krótka notatka |
|---|---|---|
| `app_pvp.go::ApplyPvPPreparation` | Composite — calls per moduł (colosseums, summoning pools) | [48](48-pvp-ready-modular-presets.md) |
| `app_appearance.go` | Companion flag toggles dla appearance/character imports | (zob. `app_appearance.go:201–229`) |
| `app.go` (m.in. ClearCount sync) | Np. `db.SetEventFlag(flags, 50+i, i == slot.Player.ClearCount)` (linia 208) — flagi 50–58 NG+ tracking | [14](14-game-state.md) |
| `backend/vm/preset.go` | Character preset round-trip | (zob. `preset.go`) |

Pełne wyliczenie callerów jest poza zakresem `15` — to katalog domenowy w pochodnych rozdziałach.

Każdy caller używa `slot.Data[slot.EventFlagsOffset:]` jako argument `flags []byte`.

---

## 13. Hardcoded flag IDs

Hardcodowane ID-y w kodzie (jako stałe `const` lub w mapach `data.*`), nie tylko w bazie statycznej:

| Stała / lokalizacja | ID | Cel |
|---|---|---|
| `data.AoWMenuUnlockedFlag` | `65800` | Włącz menu „Ashes of War" w grze |
| `data.StormStompDupFlag` | `65841` | AoW duplication for Storm Stomp |
| `data.GatefrontGraceEventFlagID` | `0x1294F` (= 76111) | Grace Gatefront — trigger companion flags |
| `data.ItemSpectralSteedWhistle` itd. (constants w `backend/db/data/`) | `0x40000082` itd. | Item IDs używane jako klucz w `itemCompanionEventFlags` |
| `data.EventFlagObtainedSpectralSteedWhistle` itd. (constants) | (faktyczne wartości w `backend/db/data/` companion flag constants) | Companion flag ID-y dla item/grace |

Pozostałe ID-y są w bazach statycznych:

- `backend/db/data/graces.go` → 419 wpisów `GraceData{Name, Region, DoorFlag, IsBossArena, DungeonType}`
- `backend/db/data/bosses.go` → 110 wpisów `BossData{Name, Region, Type, Remembrance}`
- `backend/db/data/summoning_pools.go` → 222+ wpisów
- `backend/db/data/colosseums.go` → 3 wpisy + companion flags
- `backend/db/data/container_pickup_flags.go` → mapa flag dla each container key item

**Brak globalnej tabeli „all known flag IDs"** — ID-y żyją w dedykowanych tabelach domenowych, a `EventFlags` mapa w `event_flags.go` jest tylko **byte/bit lookup cache**, nie indeksem znaczeń.

---

## 14. Validation and rollback caveats

### 14.1 Bounds check

`GetEventFlag` / `SetEventFlag` zwracają error gdy `byteIdx >= len(flags)`:

```
event flag 71190 (byte 2648) out of bounds (flags len 0)
```

Callerzy muszą obsłużyć ten warunek — w `app_world.go` typowo zwracają błąd do UI z `wrap` (`return fmt.Errorf("set boss defeated: %w", err)`).

### 14.2 Brak transactional API

`SetEventFlag` jest **stateless bit mutation**. Nie ma snapshot/rollback. Konsekwencje:

- Multi-flag operacje (np. `SetGraceVisited` z 4 companion flags) **nie są atomic** — jeśli flaga 3 z 4 zawiedzie (bounds error), pozostałe 2 są już ustawione. Naprawienie po stronie callera wymaga manualnego cleanup.
- Bulk operacje (np. `ApplyPvPPreparation` z N modułami) zwykle używają snapshot na poziomie save'a (przed Apply), nie na poziomie pojedynczych flag.

### 14.3 Slot offset 0

Jeśli `slot.EventFlagsOffset == 0` (chain unreachable, ostrzeżenie z parsera), `slot.Data[0:]` zaczyna na początku slotu → wszystkie wywołania `SetEventFlag` będą operować na **niewłaściwym bajcie** lub zwrócą bounds error. UI musi sprawdzić warning z parsera przed wywołaniem flagowych mutacji.

### 14.4 Walidacja semantyki

Resolver tier-3 (fallback) **nie waliduje** czy ID jest znane. Każdy `uint32` jest przyjmowany — dla ID poza precomputed/BST resolver użyje fallback formula (`byte = id/8, bit = 7-(id%8)`), co **nie zgadza się** z BST layout gry. Skutek: ustawienie nieznanego ID może trafić w przypadkowy bit i uszkodzić inny stan.

⚠️ **Zasada**: nie wywołuj `SetEventFlag` dla ID, którego nie znasz z precomputed lub BST. Test `TestBSTLookupMatchesEventFlags` pokrywa precomputed × BST, ale nie odrzuca ID poza tymi zakresami.

---

## 15. Safety notes

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| S1 | Ustawienie boss-defeat flag bez powiązanych flag (arena, quest, drop, world) = inconsistent state (Tier 1 ban-risk) | 🔴 high | Multi-flag planned w [38](38-boss-multiflag.md); current single-flag = best effort. UI ostrzega via `RISK_INFO` „Kill All Bosses". |
| S2 | CLEAR grace companion flag (np. ręcznie wyczyszczenie 60100) gdy gracz posiada Whistle przez quest = quest regression | 🔴 high | Grace companion flags są SET-only by design (zob. [47](47-site-of-grace-activation.md)). Nigdy nie wywołuj `SetEventFlag(60100, false)` manualnie. |
| S3 | Ustawienie nieznanego ID (poza precomputed + BST) → fallback formula trafia w przypadkowy bit | 🔴 high | Nie wywołuj `SetEventFlag` dla raw uint32 z UI; zawsze przez domenowe API (`SetGraceVisited` itp.). |
| S4 | Bulk unlock (np. `Unlock All Graces`) bez snapshot-undo → user może nie być w stanie cofnąć | ⚠️ medium | UI ma Tier 1 confirmation (zob. [32](32-ban-risk-system.md)) + save level snapshot przed Apply. |
| S5 | Map visible flag set bez fragmentu item → mapa widoczna ale gra może próbować re-trigger pickup cutscene | ⚠️ medium | `revealBaseMap` robi BOTH (flag + item) atomic — używaj jego, nie raw flag. Patrz [27](27-map-reveal.md). |
| S6 | Summoning pool flag set, ale gra runtime nie honoruje → user thinks pool aktywny, ale invasion nie działa | ⚠️ low | `app_pvp.go` zwraca warning „Bloody Finger invasion impact is unconfirmed". Patrz [42](42-summoning-pools-bug.md). |
| S7 | Slot offset 0 (chain unreachable) → SetEventFlag na bajt 0 slotu = corrupted other section | 🔴 high | Parser emituje warning `EventFlagsOffset 0x... >= SlotSize` → caller musi to wykryć. Test `TestEventFlagsOffsetCorrectness` weryfikuje offset. |

---

## 16. Test coverage

| Test | Cel | Plik |
|---|---|---|
| `TestEventFlagsBlockPS4` | PS4 round-trip `EventFlagsBlock` (read → marshall → byte-equal) | `backend/core/section_eventflags_test.go:100` |
| `TestEventFlagsBlockPC` | PC round-trip jak wyżej | `backend/core/section_eventflags_test.go:104` |
| `TestBSTLookupMatchesEventFlags` | BST formula matches precomputed positions dla wszystkich ID w precomputed table | `tests/map_flags_test.go:12` |
| `TestGetAllMapEntries` | Map flag DB coverage (entries niezerowe, brak duplikatów) | `tests/map_flags_test.go:53` |
| `TestMapFlagsRoundtrip` | Map flag toggle → save → reload → state preserved | `tests/map_flags_test.go:96` |
| `TestEventFlagsOffsetCorrectness` | `slot.EventFlagsOffset` wskazuje na faktyczny bitfield | `tests/map_flags_test.go:145` |
| `TestEventFlagsBossToggle` | Single boss toggle round-trip | `tests/save_modify_test.go:190` |
| `TestEventFlagsBossKillAll` | Bulk all bosses set → save → reload → all defeated | `tests/save_modify_test.go:277` |
| `TestEventFlagsGraceToggle` | Single grace toggle round-trip | `tests/save_modify_test.go:327` |
| `TestEventFlagsGraceUnlockAll` | Bulk all graces set → save → reload → all visited | `tests/save_modify_test.go:399` |

10 testów Phase 4-fundament + map. Pokrywają round-trip, lookup consistency, offset correctness, bulk + single ops, map flag DB coverage.

Brak dedicated test:

- **`SetEventFlag` z out-of-bounds ID** (negative case dla bounds check).
- **Multi-flag atomic rollback** (callerzy nie mają snapshot — design choice).
- **PS4↔PC cross-platform bit-equal diff** dla tego samego logical state (round-trip jest per-platforma; brak `TestConvert*` w `backend/core/` ani `tests/`).
- **Companion flag policy enforcement w 15 scope** — pokryte przez `grace_companion_flags_test.go` i `item_companion_flags_test.go` (w domenie 47/50).

---

## 17. Known limits / needs verification

| # | Limit / gap | Status | Notatka |
|---|---|---|---|
| L1 | Boss multi-flag editing | ❌ not implemented | Patrz [38](38-boss-multiflag.md). Current = pojedyncza flaga 9xxx. |
| L2 | PvP modular preset moduł E (Sites of Grace) | ❌ placeholder | `app_pvp.go:109` warning „planned but not enabled". Patrz [48](48-pvp-ready-modular-presets.md). |
| L3 | Summoning pool runtime honor | ⚠️ flag set, ale gra nie honoruje | Patrz [42](42-summoning-pools-bug.md). |
| L4 | Walidacja unknown ID w `SetEventFlag` | ❌ brak | Tier-3 fallback trafia w przypadkowy bit. `needs verification` przy każdej zmianie zewnętrznego ID. |
| L5 | BST coverage po update'cie regulation | ⚠️ `eventflag_bst.txt` może wymagać re-extracji po patchu gry | Brak automatycznego diff vs regulation. |
| L6 | Multi-flag atomic rollback | ❌ brak | Multi-flag ops nie są atomic. Caller-level snapshot wymagany. |
| L7 | Cross-platform PS4 vs PC bit-equal | ⚠️ round-trip pokryty per-platforma | Brak `TestConvert*` w `backend/core/` ani `tests/` weryfikującego, że PS4 i PC zapisują ten sam logical state (toggle grace na PS4 → save → convert → load PC → ten sam bit). `needs verification`. |
| L8 | DLC flag-block coverage (82xxx, 83xxx, 84xxx itd.) | ⚠️ DLC patche mogą dodawać nowe bloki | `needs verification` przy nowym DLC. |
| L9 | „Talisman Pouch sync" historical note | ⚠️ wspomniane w wcześniejszych iteracjach jako 60500/60510/60520 | W aktualnym kodzie (2026-05-21): `Talisman Pouch` jest key item `0x40002738`; **brak** hardcoded 60500/60510/60520 w `itemCompanionEventFlags` (jedyny tam item z 4-flag companion = Spectral Steed Whistle). `needs verification` czy 60500/60510/60520 sync był kiedykolwiek wdrożony, czy to tylko intencja z research notes. |
| L10 | Tier 1 cache stale | ⚠️ `event_flags.go` to snapshot offline-generated | Jeśli `event_flags.go` nie został regenerowany po update'cie regulation, Tier 1 może mieć nieaktualne `(Byte, Bit)`. Test `TestBSTLookupMatchesEventFlags` wykryje rozbieżność Tier 1 vs Tier 2 dla tych samych ID, ale tylko jeśli zarówno precomputed jak i BST są regenerowane spójnie. |

---

## 18. Cross-references

| Topic | Master rozdział |
|---|---|
| Map reveal (4 warstwy: regiony, bitmapa, cover layer DLC, fog of war) | [27 — Map Reveal](27-map-reveal.md) |
| Unlocked Regions (Layer 0 map reveal) | [11 — Regions](11-regions.md) |
| DLC Cover Layer (Layer 2 map reveal) | [29 — DLC Black Tiles](29-dlc-black-tiles.md) |
| Sites of Grace activation + companion flags (SET-only) | [47 — Sites of Grace Activation](47-site-of-grace-activation.md) |
| Item companion flags (SET + CLEAR symmetric) | [50 — Item Companion Flags](50-item-companion-flags.md) |
| Boss defeat flag + LastRestedGrace + ClearCount | [14 — Game State](14-game-state.md) |
| Boss multi-flag design (planowane) | [38 — Boss Multi-Flag](38-boss-multiflag.md) |
| PvP modular presets (regions/colosseums/map/pools/graces) | [48 — PvP Modular Presets](48-pvp-ready-modular-presets.md) |
| Summoning pools runtime gap | [42 — Summoning Pools Bug](42-summoning-pools-bug.md) |
| World state (FieldArea, WorldArea, WorldGeomMan, RendMan — read-only) | [16 — World State](16-world-state.md) |
| Ban-risk Tier 1 UI for bulk flag operations | [32 — Ban-Risk System](32-ban-risk-system.md) |

---

## 19. Sources

### Code

- `backend/db/db.go::GetEventFlag` / `SetEventFlag` / `resolveEventFlagPosition`
- `backend/db/data/event_flags.go` — 865 precomputed `EventFlagInfo{Byte, Bit}`
- `backend/db/data/bst.go` + `eventflag_bst.txt` — 11 919 wpisów BST mapping
- `backend/core/section_eventflags.go` — `EventFlagsByteCount`, `EventFlagsBlock`, `PreEventFlagsScalars`
- `backend/core/structures.go::Slot.EventFlagsOffset` + chain calc (`s.EventFlagsOffset = s.IngameTimerOffset + DynEventFlags`)
- `app_world.go` — 20+ callsite'ów `db.SetEventFlag`/`GetEventFlag` (graces, bosses, pools, colosseums, cookbooks, map reveal, whetblades, quest)
- `app_pvp.go::ApplyPvPPreparation` — composite caller
- `backend/db/data/whetblades.go::AoWMenuUnlockedFlag`, `StormStompDupFlag`
- `backend/db/data/grace_companion_flags.go::GatefrontGraceEventFlagID`
- `backend/db/data/item_companion_flags.go`

### Tests

- `backend/core/section_eventflags_test.go` — round-trip PS4/PC
- `tests/map_flags_test.go::TestBSTLookupMatchesEventFlags`, `TestEventFlagsOffsetCorrectness`
- `tests/save_modify_test.go` — `TestEventFlagsBossToggle/KillAll`, `TestEventFlagsGraceToggle/UnlockAll`

### Reference parsers / community

- er-save-manager: `parser/event_flags.py` — klasa `EventFlags` (referencyjny algorytm BST)
- er-save-manager: `src/resources/eventflag_bst.txt` — source BST mapping
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs:197–223` — EventFlags container (`0x1bf99f` bytes)
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:event-flag-list
- Event Flags GitHub Pages: https://soulsmods.github.io/elden-ring-eventparam/
- Event Flags Spreadsheet: https://docs.google.com/spreadsheets/d/1Nn-d4_mzEtGUSQXscCkQ41AhtqO_wF2Aw3yoTBdW9lk
- TGA Cheat Engine Table: https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA

### Hex-verified saves

- `tmp/save/ER0000.sl2` (PC, 5 slotów) — round-trip 2026-05-21
- `tmp/save/oisis_pl-org.txt` (PS4) — round-trip 2026-05-21
