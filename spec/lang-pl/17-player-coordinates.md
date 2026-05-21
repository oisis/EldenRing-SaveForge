# 17 — Player Coordinates

> **Type**: Binary format spec (read-only reference)
> **Scope**: Sekcja `PlayerCoordinates` (61 B) + `SpawnPointBlock` (≥10 B version-gated) — co kod parsuje, dlaczego nie ma writera, jakie są ryzyka edycji.

Cross-refs: [14-game-state.md](14-game-state.md), [16-world-state.md](16-world-state.md).

---

## 1. Cel rozdziału

Zdefiniować jednoznacznie:

- co kod parsuje jako pozycję gracza (`PlayerCoordinates` struct, 61 B),
- co kod parsuje jako spawn point (`SpawnPointBlock`, version-gated),
- dlaczego SaveForge **nie udostępnia** publicznego endpointu do edycji koordynatów,
- jakie ryzyka wiążą się z manualną mutacją tych pól.

Nie powiela World State overview (patrz [16-world-state.md](16-world-state.md)) ani Game State (patrz [14-game-state.md](14-game-state.md)).

## 2. Status

| Aspekt | Status |
|---|---|
| `PlayerCoordinates` struct parser | ✅ `backend/core/section_player_coords.go:14-46` |
| `PlayerCoordinatesSize = 61` | ✅ `section_player_coords.go:23` (12+4+16+1+12+16) |
| `SpawnPointBlock` parser (version-gated) | ✅ `section_player_coords.go:73-128` |
| Read/Write verbatim w sekcji slotu | ✅ round-trip preserved |
| App-level public endpoint (`SetPlayerCoords`, `Teleport`, itp.) | ❌ **brak** — `grep` w `app*.go` zwraca 0 wyników |
| Frontend UI dla koordynatów | ❌ brak |
| Test coverage | ✅ `section_player_coords_test.go` — `TestPlayerCoordsRoundTripPS4`/`PC` (2 testy) |

`needs verification`: stary doc twierdził `PlayerCoordinatesSize = 57`. Aktualny kod ma **61** (`const PlayerCoordinatesSize = 12 + 4 + 16 + 1 + 12 + 16 = 61`). Komentarz w kodzie wyjaśnia: „er-save-manager labels this 57 bytes in a comment — the actual struct is 61 bytes; the comment is stale." Stara wartość 57 B w PL spec'u była dziedziczona ze stale-d komentarza er-save-managera.

## 3. Source of truth w kodzie

| Plik / symbol | Co zawiera | Tryb |
|---|---|---|
| `backend/core/section_player_coords.go::PlayerCoordinates` (linie 14-21) | 61 B struct: `Coordinates + MapID + Angle + GameMan0xbf0 + UnkCoords + UnkAngle` | read/write verbatim |
| `backend/core/section_player_coords.go::PlayerCoordinatesSize` (linia 23) | `= 12+4+16+1+12+16 = 61` | const |
| `backend/core/section_player_coords.go::PlayerCoordinates.Read/Write` (linie 25-56) | Parser + serializer per pole | RW |
| `backend/core/section_player_coords.go::SpawnPointBlock` (linie 73-81) | `PadAfterCoords + SpawnPointEntityID + GameMan0xb64 + (v>=65) TempSpawnPointEntityID + (v>=66) GameMan0xcb3` | read/write verbatim |
| `backend/core/section_player_coords.go::SpawnPointBlock.Read/Write/ByteSize` (linie 83-140) | Parser z gating po `version` slotu | RW |
| `FloatVector3` / `FloatVector4` / `MapID` | Pomocnicze prymitywy do parsowania | reader/writer |
| `backend/core/section_player_coords_test.go` | 2 round-trip testy (PS4 + PC) | tests |

`PlayerCoordinates` i `SpawnPointBlock` są częścią szerszej sekcji slotu — `Read()` jest wywoływany przez parser slotu wyżej (`backend/core/structures.go` lub odpowiednik); ten rozdział nie powiela całego pipeline'u parsowania.

## 4. Mental model

```
slot data
  ├─ ... (inne sekcje)
  ├─ PlayerCoordinates (61 B)
  │    ├─ Coordinates    FloatVector3 (12 B)  → bieżąca pozycja XYZ
  │    ├─ MapID          MapID        (4 B)   → identyfikator mapy (opaque 4-byte ID)
  │    ├─ Angle          FloatVector4 (16 B)  → quaternion obrotu
  │    ├─ GameMan0xbf0   u8           (1 B)   → nieznane (`unk`)
  │    ├─ UnkCoords      FloatVector3 (12 B)  → drugi zestaw koordynatów (semantyka nieznana)
  │    └─ UnkAngle       FloatVector4 (16 B)  → drugi quaternion
  ├─ SpawnPointBlock (≥10 B, version-gated)
  │    ├─ PadAfterCoords           [2]byte (verbatim)
  │    ├─ SpawnPointEntityID       u32
  │    ├─ GameMan0xb64             u32
  │    ├─ TempSpawnPointEntityID   u32  (tylko jeśli version >= 65)
  │    └─ GameMan0xcb3             u8   (tylko jeśli version >= 66)
  └─ ... (inne sekcje)
```

Aktualny kod traktuje obie sekcje jako **read-write verbatim** — load parsuje na typowane pola, save zapisuje 1:1 te same bajty. Brak walidacji ani transformacji.

## 5. Current parsed data

`PlayerCoordinates`:

| Pole | Typ | Rozmiar | Komentarz w kodzie |
|---|---|---|---|
| `Coordinates` | `FloatVector3` | 12 B | główna pozycja XYZ |
| `MapID` | `MapID` | 4 B | identyfikator mapy |
| `Angle` | `FloatVector4` | 16 B | quaternion obrotu |
| `GameMan0xbf0` | `u8` | 1 B | `unk` (nazwa od adresu w `GameMan`) |
| `UnkCoords` | `FloatVector3` | 12 B | semantyka **niezweryfikowana** |
| `UnkAngle` | `FloatVector4` | 16 B | semantyka **niezweryfikowana** |

**Suma**: `12 + 4 + 16 + 1 + 12 + 16 = 61 B`.

`SpawnPointBlock` (version-gated):

| Pole | Typ | Rozmiar | Gating |
|---|---|---|---|
| `PadAfterCoords` | `[2]byte` | 2 B | always verbatim |
| `SpawnPointEntityID` | `u32` | 4 B | always |
| `GameMan0xb64` | `u32` | 4 B | always |
| `TempSpawnPointEntityID` | `u32` | 4 B | tylko `version >= 65` (wszystkie nasze sloty mają `version >= 230`, więc zawsze obecne) |
| `GameMan0xcb3` | `u8` | 1 B | tylko `version >= 66` (jw.) |

`ByteSize()` zwraca rzeczywistą długość bazując na flagach `HasTempSpawnPoint`/`HasGameMan0xcb3` ustawionych przy `Read`. Patrz `section_player_coords.go:131-140`.

`needs verification`: znaczenie `UnkCoords`/`UnkAngle` — stara doc proponowała „backup ground position" / „spawn anchor". W aktualnym kodzie nie ma żadnego callera, który by używał tych pól w logice — są wyłącznie verbatim preserved. Stara doc o „spawn point / last stable position" pozostaje hipotezą bez code-confirmation.

`needs verification`: znaczenie `GameMan0xbf0`/`GameMan0xb64`/`GameMan0xcb3` (`needs verification` per wszystkie 3 pola — nazwy pochodzą od ich adresu w runtime `GameMan` strukturze, semantyka nieznana w SaveForge).

## 6. Read-only status

**SaveForge nie udostępnia publicznego endpointu do edycji pozycji gracza.**

`grep -rn "PlayerCoordinates\|SpawnPointBlock\|setCoord\|Teleport" app*.go` zwraca **0 wyników** dla wszystkich plików `app*.go`. Public API (Wails bindings) **nie pozwala**:

- ustawić `Coordinates`,
- ustawić `MapID`,
- ustawić `Angle`,
- ustawić `SpawnPointEntityID`,
- ustawić `TempSpawnPointEntityID`,
- ustawić jakichkolwiek pól w obu strukturach.

Frontend (`frontend/src/components`) nie ma żadnego komponentu wyświetlającego lub edytującego koordynaty.

Każda mutacja musiałaby być wykonana ręcznie przez direct memory hex edit poza SaveForge — patrz §8 dla ryzyk.

## 7. What SaveForge does not implement

| Funkcja | Status |
|---|---|
| `SetPlayerCoordinates(x, y, z)` | ❌ nie ma |
| `SetMapID(id)` | ❌ nie ma |
| `SetSpawnPoint(entityID)` | ❌ nie ma |
| `Teleport(x, y, z, mapID)` | ❌ nie ma |
| `RestoreLastBloodstain()` | ❌ nie ma |
| Walidacja `MapID` (czy ID istnieje w `regulation.bin`) | ❌ nie ma |
| Walidacja Coordinates in-bounds | ❌ nie ma |
| Frontend UI dla edycji pozycji | ❌ nie ma |
| Tabela mapowania `MapID` → nazwa regionu | ❌ nie ma (`needs verification`: stara doc miała hipotetyczną tabelę pre-byte; aktualny kod jej nie używa) |

`needs verification`: czy istnieje planowana funkcja teleportacji — nie znaleziono w `docs/ROADMAP.md` ani w issue tracker. Aktualnie wszystkie zmiany pozycji muszą być wykonane przez grę (rest at grace, teleport via menu).

## 8. Relation to World State

Patrz [16-world-state.md](16-world-state.md) §6.1 + §5. `PlayerCoordinates` jest **odrębną** sekcją binarną — nie częścią `WorldGeomBlock` (`FieldArea / WorldArea / WorldGeomMan / WorldGeomMan2 / RendMan`). Również nie jest częścią `BloodStain` (gdzie L2 DLC tile coords mają oddzielny partial mutator — patrz [29-dlc-black-tiles.md](29-dlc-black-tiles.md)).

Tabela World State subsystem map ([16](16-world-state.md) §5) klasyfikuje `Player Coordinates` jako osobny rozdział z trybem „RO/RW per chapter” — w aktualnym kodzie de facto **RO** (read/write verbatim, brak app-level mutatora).

## 9. Relation to Game State

Patrz [14-game-state.md](14-game-state.md). `PlayerCoordinates` **nie jest** częścią `PreEventFlagsScalars` ani `Game State`:

- `LastRestedGrace` (BonfireId — w `PreEventFlagsScalars`) jest **osobnym** mechanizmem respawn anchora. Patrz [14](14-game-state.md) i [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §6.3.
- Gra przy respawn używa `LastRestedGrace` jako kotwicy spawn-u; `PlayerCoordinates` jest aktualizowane przez grę przy każdym ruchu gracza.

**Bezpieczniejsza alternatywa „teleportu”**: zmiana `LastRestedGrace` w `PreEventFlagsScalars` — ale aktualny kod również tego **nie udostępnia** jako publiczny endpoint (gra sama nadpisuje przy pierwszym rest). Patrz [14](14-game-state.md).

`needs verification`: czy zmiana `LastRestedGrace` w save'ie bez ruszania `PlayerCoordinates` powoduje, że gra teleportuje gracza przy load do nowej gracji — historyczne dyskusje sugerują „nie, dopóki nie ma fizycznego rest", ale brak izolowanego testu.

## 10. Validation and safety notes

### 10.1 Manualna mutacja Coordinates

Bez app-level endpointu jedyna ścieżka edycji to direct hex edit. Ryzyka:

- **Out-of-bounds** XYZ (poza geometrią mapy) → falling death → respawn at last grace. Niski koszt, ale frustrujące.
- **Pod mapą** (Y zbyt nisko) → instant death loop jeśli respawn też pod mapą.
- **Wewnątrz geometrii** (clip) → softlock, brak collision recovery.
- **Wewnątrz boss arena bez aktywnego encounter** → możliwy crash lub stuck.

`needs verification`: czy gra ma defensive bounds-checking przy load, czy ufa save'owi blind.

### 10.2 MapID mismatch

`MapID` to opaque 4-byte ID. Jeśli ID:

- **Nie istnieje** w `regulation.bin` → infinite loading / crash.
- **Istnieje, ale gracz nie ma DLC** → crash (DLC area IDs).
- **Istnieje, ale niezgodny z Coordinates** → spawn w innej geometrii niż XYZ wskazuje. Możliwe falling/clip.

`needs verification`: czy SaveForge powinien dodawać walidator `MapID` przeciw `data.Regions` snapshot — aktualnie brak. Patrz [11-regions.md](11-regions.md).

### 10.3 SpawnPointBlock version mismatch

`SpawnPointBlock` ma trailing fields gated po `version` slotu (`>= 65` dla TempSpawnPoint, `>= 66` dla GameMan0xcb3). Jeśli ręcznie obniżyć `Version` bez usunięcia trailing bytes, parser at load zwróci out-of-range error lub mis-aligned read.

`PadAfterCoords` (2 B) jest zachowywane verbatim — nie próbować edytować jako „rezerwy".

### 10.4 Quaternion (Angle)

Quaternion `Angle` (16 B = 4× f32) musi być normalized (`x² + y² + z² + w² ≈ 1`). Mutacja zerująca lub niemormalizowana może powodować undefined behavior renderera. `needs verification`: czy gra normalizuje przy load czy ufa save'owi.

### 10.5 No write endpoint + no in-game verification

Brak publicznego endpointu oznacza:

- Brak risk gate w UI (Tier 0/1/2).
- Brak rollback hook (`pushUndo` etc.).
- Brak walidacji per-pole.
- Brak CI testu „set coords → reload → assert position".

Każda zmiana wykonana spoza SaveForge (np. hex editor) jest **całkowicie nieweryfikowana** przez SaveForge w I/O round-trip — tylko bajty są preserved, semantyka jest na odpowiedzialności użytkownika.

### 10.6 Platform / version differences

`PlayerCoordinatesSize = 61` i layout `SpawnPointBlock` są takie same dla PC i PS4 (nasze sloty mają `version >= 230`, więc oba trailing fields w `SpawnPointBlock` są zawsze obecne).

`needs verification`: czy przyszłe patche gry mogą rozszerzyć `PlayerCoordinates` (np. dodać nowe pole `Unk`). Brak detection mechanism.

## 11. Test coverage

| Test | Plik | Co weryfikuje |
|---|---|---|
| `TestPlayerCoordsRoundTripPS4` | `backend/core/section_player_coords_test.go:83` | Load → Write → diff = 0 dla PS4 save |
| `TestPlayerCoordsRoundTripPC` | `backend/core/section_player_coords_test.go:87` | jw. dla PC save |

**Brak**:

- Izolowanego testu walidacji `MapID` przeciw `regulation.bin`.
- Testu walidacji XYZ bounds.
- Testu walidacji quaternion normalization.
- Testu mutation + reload (bo brak mutatora).

## 12. Known limits / needs verification

1. **`UnkCoords` / `UnkAngle` semantyka** — niezweryfikowana, prawdopodobnie spawn anchor lub last bloodstain pozycja.
2. **`GameMan0xbf0` / `GameMan0xb64` / `GameMan0xcb3` semantyka** — niezweryfikowana, nazwy od adresu w runtime `GameMan` strukturze.
3. **`MapID` 4-byte format** — stara doc miała hipotetyczną tabelę byte breakdown; aktualny kod traktuje jako opaque ID. `needs verification`.
4. **In-bounds checking** w grze przy load — brak izolowanego testu.
5. **Quaternion normalization tolerance** — brak izolowanego testu.
6. **Cross-version layout stability** — brak detection mechanism po patchu.
7. **Brak app-level Set API** — aktualny stan; przyszłe wdrożenie wymagałoby walidatora + risk gate.
8. **Brak Teleport endpoint** — nie zaplanowane w aktualnym kodzie (poza SaveForge).
9. **`SpawnPointEntityID` mapowanie na in-game entity** — niezweryfikowane; ID jest opaque.
10. **Stary spec 57 B** — naprawione w tym rozdziale do 61 B.

## 13. Cross-references

- [11-regions.md](11-regions.md) — Region IDs (mogą być powiązane z `MapID`, ale brak code-level mapping).
- [14-game-state.md](14-game-state.md) — `LastRestedGrace` jako bezpieczniejsza alternatywa do „teleportacji" (też brak public API).
- [16-world-state.md](16-world-state.md) — overview World State; `PlayerCoordinates` jest osobną sekcją.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — `BloodStain` partial mutator z syntetycznymi koordynatami dla L2 DLC tiles (osobny od `PlayerCoordinates`).
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — `LastRestedGrace` BonfireId namespace; nie modyfikowane przez edytor.

## 14. Sources

- `backend/core/section_player_coords.go` — `PlayerCoordinates` (61 B), `SpawnPointBlock` (version-gated), helpery FloatVector3/4.
- `backend/core/section_player_coords_test.go` — 2 round-trip testy.
- Brak callerów w `app*.go` (verified via `grep`).
- Brak komponentów frontendowych (verified via `grep`).
