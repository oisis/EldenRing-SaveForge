# 47 — Site of Grace Activation

> **Type**: Binary format spec + design doc (canonical chapter)
> **Scope**: Aktywacja Sites of Grace w SaveForge — model danych, kontrakt SET‑only dla flag towarzyszących, write path, status modułów PvP, relacje do event flags / map / world / game state.

Cross‑refs: [11-regions.md](11-regions.md), [14-game-state.md](14-game-state.md), [15-event-flags.md](15-event-flags.md), [16-world-state.md](16-world-state.md), [27-map-reveal.md](27-map-reveal.md), [29-dlc-black-tiles.md](29-dlc-black-tiles.md), [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md), [50-item-companion-flags.md](50-item-companion-flags.md).

---

## 1. Cel rozdziału

Zdefiniować jednoznacznie, co edytor **realnie** robi przy aktywacji Site of Grace:

- jaki bit jest ustawiany dla samej gracji,
- jaki bit drzwi dla dungeonu (Cat/HG),
- które flagi towarzyszące są ustawiane (SET‑only) i dlaczego nie są czyszczone,
- czego edytor **nie** dotyka (`LastRestedGrace`, MapFlags, BonfireId),
- jaki jest status modułu „Sites of Grace” w PvP prep,
- gdzie kończy się implementacja a zaczyna `needs verification`.

Nie powiela helper API event flag (patrz [15-event-flags.md](15-event-flags.md)) ani semantyki item companion flags (patrz [50-item-companion-flags.md](50-item-companion-flags.md)).

## 2. Status

| Aspekt | Status |
|---|---|
| Backend endpoint `SetGraceVisited` | ✅ `app_world.go:43` |
| Backend endpoint `GetGraces` | ✅ `app_world.go:14` |
| Static DB | ✅ `backend/db/data/graces.go` — 419 wpisów (snapshot z `data.Graces`) |
| Companion flags map | ✅ `backend/db/data/grace_companion_flags.go` — 1 wpis (Gatefront 76111) |
| `IsKnownGraceID` helper | ✅ `backend/db/db.go:1126` |
| UI `WorldTab` Sites of Grace | ✅ per‑grace + per‑region + Unlock All / Lock All |
| Tier risk gate `bulk_grace_unlock` | ✅ Tier 1 (`RiskActionButton`) |
| Companion flag policy SET‑only | ✅ enforced w `app_world.go:73‑81`, pokryte testem (`tests/grace_companion_flags_test.go`) |
| PvP preset module `SitesOfGrace` | ❌ placeholder — zwraca warning „planned but not enabled in this version” (`app_pvp.go:108‑110`) |
| In‑world activation sequence (cutscena/animacja) | ⚠️ `needs verification` per‑category (Church of Elleh overworld — testowany, inne kategorie — nie) |

## 3. Source of truth w kodzie

| Plik / symbol | Co zawiera |
|---|---|
| `app_world.go::GetGraces` | Odczyt visited per‑grace przez `db.GetEventFlag` |
| `app_world.go::SetGraceVisited` | Write path: pushUndo + grace flag + door flag + companion flags |
| `backend/db/data/graces.go::Graces` | 419 wpisów `GraceData{Name, DungeonType, DoorFlag, BossArena}` |
| `backend/db/data/grace_companion_flags.go::graceCompanionEventFlags` | Mapa `graceID → []companionFlag` (SET‑only) |
| `backend/db/data/grace_companion_flags.go::CompanionEventFlagsForGrace` | Lookup API |
| `backend/db/data/grace_companion_flags.go::GatefrontGraceEventFlagID` | Stała `0x0001294F` (= 76111) |
| `backend/db/db.go::GraceEntry` | Public struct `{ID, Name, Region, Visited, IsBossArena, DungeonType}` |
| `backend/db/db.go::GetAllGraces` | sync.OnceValue cache; sortowanie po Region/Name + regex region mapping |
| `backend/db/db.go::IsKnownGraceID` | Predykat (członkostwo w `data.Graces`) |
| `app_pvp.go` (`opts.SitesOfGrace`) | Placeholder; brak własnej logiki |
| `frontend/src/components/WorldTab.tsx` | UI: `GetGraces`/`SetGraceVisited`, bulk handlers, Tier 1 risk gate |
| `tests/grace_companion_flags_test.go` | 3 integration tests (SetOnRealSave / NoRTHFlags / SetOnlyNotCleared) |
| `backend/db/data/grace_companion_flags_test.go` | Unit test stałych flag |

## 4. Mental model

Aktywacja gracji w SaveForge to pojedynczy bit w bitfieldzie `EventFlags`, opcjonalnie z drobnym fan‑outem:

```
SetGraceVisited(slot, graceID, visited)
  ├─ a.pushUndo(slotIndex)                  ← single snapshot rollback
  ├─ SetEventFlag(flags, graceID, visited)  ← bit gracji (71xxx–76xxx)
  ├─ if DoorFlag != 0:
  │     SetEventFlag(flags, DoorFlag, visited)   ← drzwi dungeonu (Cat/HG)
  └─ if visited == true:
        for each f in CompanionEventFlagsForGrace(graceID):
          SetEventFlag(flags, f, true)       ← SET-only; brak ścieżki clear
```

Bit gracji + opcjonalny `DoorFlag` to **w pełni symetryczna** para SET/CLEAR. Companion flags są **asymetryczne** — patrz §8.

## 5. Site of Grace data model

`data.Graces` (`backend/db/data/graces.go`) jest mapą `map[uint32]GraceData` z 419 wpisami (snapshot źródła w repo na 2026‑05‑21). Każdy wpis to:

```go
GraceData{
    Name        string  // np. "Church of Elleh (Limgrave)"
    DungeonType string  // "catacomb" | "hero_grave" | "" (none)
    DoorFlag    uint32  // 0 jeśli brak drzwi dungeonu
    BossArena   bool    // true jeśli gracja jest arena bossa
}
```

`db.GetAllGraces()` (`db.go:858`) konwertuje to do `[]GraceEntry` z dodatkowym field `Region` derywowanym ze stringa po nawiasie + regex‑owe rozcinanie podregionów Limgrave (East/West), Liurnia (East/North/West) i Mountaintops (East/West). To **derywacja runtime**, nie pole zapisywane w save.

| Aspekt | Wartość | Snapshot |
|---|---|---|
| Łączna liczba | **419** | weryfikowane `awk '/^var Graces/,/^}/' ... \| grep -cE '^\s*0x'` na 2026‑05‑21 |
| Snapshot | static Go map | regenerowany ręcznie z `regulation.bin` przy patchach gry |
| Per‑grace fields | `Name`, `DungeonType`, `DoorFlag`, `BossArena` | brak ID region — region derywowany w `GetAllGraces` |

`needs verification`: liczba 419 może się różnić po przyszłych patchach DLC; brak automatycznej regeneracji z `regulation.bin` w buildzie. Snapshot date check: data ostatniej edycji `graces.go`.

## 6. Grace IDs and event flags

### 6.1 ID space

Grace event flags zajmują pasma **71000–76960** (z lukami) — patrz [15-event-flags.md](15-event-flags.md) §BST i tabela byte/bit. Charakter pasm (najczęściej spotykany subset):

| Pasmo | Typ obszaru |
|---|---|
| 71xxx | Legacy dungeons (Stormveil, Leyndell, areny bossów base game) |
| 72xxx | DLC legacy dungeons (Belurat, Enir‑Ilim) |
| 73xxx | Catacombs / Hero's Graves (parowane z `DoorFlag`) |
| 74xxx | DLC catacombs / dungeony |
| 76xxx | Overworld (największa grupa) |
| 76xxx (DLC subset, do 76960) | Overworld DLC |

`needs verification`: powyższa segregacja pasm jest opisowa — kod jej nie używa do klasyfikacji. `IsKnownGraceID(id)` to po prostu `_, ok := data.Graces[id]`. Klasyfikator DLC vs base game **nie istnieje** dla gracji w obecnym kodzie (kontrast: `IsDLCMapFlag` dla map flags — patrz [27-map-reveal.md](27-map-reveal.md)).

### 6.2 Door flags (dungeon catacombs / hero's graves)

`GraceData.DoorFlag` (jeśli `!= 0`) jest ustawiany **symetrycznie** z bitem gracji: zarówno przy `visited=true` jak i `visited=false`. To jedyna część `SetGraceVisited` która faktycznie CLEAR'uje cokolwiek przy deaktywacji.

Dotyczy wyłącznie wpisów `DungeonType == "catacomb"` lub `"hero_grave"` — patrz konstruktor `Cat()` / `HG()` w `graces.go:19/24`.

`needs verification`: czy gra rzeczywiście **zamyka** drzwi dungeonu, gdy DoorFlag zostanie wyclear'owane przy `visited=false`, **nie zostało zweryfikowane in‑game**. Mechanika oparta na założeniu, że `DoorFlag` jest two‑way trigger (open jeśli SET, closed jeśli CLEAR) — w grze może być one‑way (open trigger, ale CLEAR nie ma efektu).

### 6.3 BonfireId — osobny namespace

Gra używa też **drugiej** przestrzeni ID dla gracji — `BonfireId` formatu 10‑cyfrowego (np. `1042362950` = Church of Elleh). Jest przechowywany jako pojedynczy skalar `LastRestedGrace` w `PreEventFlagsScalars` (patrz [14-game-state.md](14-game-state.md)) i zarządzany **wyłącznie przez grę** — edytor go nie ustawia. SaveForge **nie utrzymuje** mapowania `EventFlag ID → BonfireId`.

## 7. Visited / activated semantics

| Stan | Co ustawia bit gracji | Co edytor robi przy `SetGraceVisited(visited=true)` |
|---|---|---|
| **Bit gracji `=1`** | Gra przy pierwszym odpoczynku przy gracji; edytor przez `SetGraceVisited` | ✅ `SetEventFlag(graceID, true)` |
| **Znacznik na mapie** | Wyprowadzane przez UI/EMEVD z bit gracji | ✅ (efekt uboczny SET bitu) |
| **Wpis na fast‑travel** | jw. | ✅ (efekt uboczny SET bitu) |
| **`LastRestedGrace` BonfireId** | Gra przy fizycznym dotknięciu / odpoczynku | ❌ edytor nie dotyka — patrz §13 |
| **In‑world activation sequence** (animacja, cutscena NPC) | EMEVD przy load obszaru (większość kategorii); może wymagać dodatkowych flag area‑load (np. 69300, 78101) | ❌ edytor nie ustawia |
| **MapFlags (62xxx/82xxx)** | Map reveal — osobna warstwa | ❌ — patrz [27-map-reveal.md](27-map-reveal.md) |

`needs verification`: założenie „EMEVD wyprowadza in‑world state z bit gracji przy load obszaru” jest zweryfikowane **tylko dla Church of Elleh (76100, overworld)** w historycznym save diff (2026‑05‑09 — patrz `docs/CHANGELOG.md`). Inne kategorie (legacy dungeons, catacombs, DLC) — nie zostały zweryfikowane indywidualnie.

## 8. Grace companion flags

`graceCompanionEventFlags` w `backend/db/data/grace_companion_flags.go` mapuje **grace EventFlag ID → minimalny zestaw flag**, które gra ustawia razem z gracją podczas pierwszej autentycznej wizyty. Aktualnie:

| Grace | EventFlag ID | Companion flags |
|---|---|---|
| Gatefront (Limgrave West) | `0x0001294F` (= 76111) — `GatefrontGraceEventFlagID` | `EventFlagObtainedSpectralSteedWhistle`, `EventFlagMelinaGaveWhistle`, `EventFlagWhistleWorldState`, `EventFlagMelinaAcceptRefusePopup` |

**Tylko jeden wpis** w aktualnej bazie. Companion flag pool dla pozostałych ~418 gracji jest pusty — `CompanionEventFlagsForGrace(otherGrace)` zwraca `nil` i loop `for _, f := range companions` jest no‑op.

`needs verification`: czy inne gracje również wymagają companion flag fan‑out aby silnik gry nie re‑triggerował dialogu / cutsceny — nie zostało wyczerpująco zbadane. Konstrukcja kodu jest forward‑compatible (każda nowa para w mapie automatycznie aktywna), ale wymaga ręcznego badania per‑grace.

Companion flag policy wspólnie używana z item path (np. Spectral Steed Whistle) — patrz [50-item-companion-flags.md](50-item-companion-flags.md). Stałe `EventFlagObtained...` są współdzielone między `grace_companion_flags.go` i `item_companion_flags.go`.

### 8.1 Co NIE wchodzi do companion set (explicit exclusion)

Test `TestCompanionEventFlagsForGrace_NoForbiddenFlags` (`backend/db/data/grace_companion_flags_test.go:35`) **i** `TestGraceCompanionFlagsNoRTHFlags` (`tests/grace_companion_flags_test.go:49`) enforce'ują negatywną listę identycznych 9 ID:

| Excluded flag | Powód (z komentarzy w testach / `grace_companion_flags.go`) |
|---|---|
| `10009655` | Melina RTH invitation trigger — osobny krok postępu |
| `11109658` | Gideon welcome (RTH visited marker) |
| `11109659` | Gideon advice |
| `11109786` | RTH transport trigger — transient (czyszczone przez silnik gry) |
| `4698` | Melina cutscene trigger — transient |
| `4656` | Level up performed — osobna akcja użytkownika |
| `710770` | Melina leaves Gatefront — research candidate, runtime test potwierdził, że nie wymagane (spec/50 PS4 test 2026‑05‑11) |
| `69090` | jw. |
| `69370` | jw. |

`needs verification`: starsze iteracje spec/47 wymieniały dodatkowo `4651`–`4653` jako „Melina dialog states transient” — te ID **nie** są w żadnej z dwóch list `forbidden` w testach (są jedynie w `backend/db/data/quests.go:487` jako wymagania quest popup‑u). Status ich klasyfikacji jako forbidden vs po prostu unverified — otwarty.

Reguła z komentarza w `grace_companion_flags.go`: **tylko flagi zweryfikowane w realnych save'ach po faktycznym wystąpieniu in‑game**.

## 9. SET‑only asymmetric behavior

**Kontrakt SET‑only** dla companion flags:

```go
// app_world.go:70‑81
// SET-only: companion flags are set on activation but never cleared on deactivation.
// They may also be set by item companion flags or normal game progression — clearing
// them on visited=false would regress saves that obtained the flags through other paths.
if visited {
    if companions := data.CompanionEventFlagsForGrace(graceID); len(companions) > 0 {
        for _, f := range companions {
            if err := db.SetEventFlag(flags, f, true); err != nil {
                fmt.Printf("Warning: companion flag %d for grace %d: %v\n", f, graceID, err)
            }
        }
    }
}
```

Konsekwencje:

| Akcja | Co się dzieje z bitem gracji | Co się dzieje z companion flags |
|---|---|---|
| `SetGraceVisited(visited=true)`  | `=1` | każda flaga z setu `=1` |
| `SetGraceVisited(visited=false)` | `=0` | **nie tknięte** (zostają w stanie sprzed wywołania) |
| `SetGraceVisited(visited=true)` po wcześniejszym `=true` | `=1` (idempotent) | `=1` (idempotent) |

Asymetria jest **celowa** — uzasadnienie w komentarzu kodu: companion flags mogą być ustawione przez **inną** ścieżkę (item companion flag z [50-item-companion-flags.md](50-item-companion-flags.md), normalny game progress). CLEAR przy `visited=false` cofałby progress osiągnięty inną drogą — regression risk.

`needs verification`: czy istnieją sytuacje, w których użytkownik **chce** wyczyścić companion flags po deaktywacji gracji (np. testowanie). Nie ma żadnego API ani UI do tego — jedyna ścieżka to manualna mutacja bitfieldu.

Kontrast: bit gracji + `DoorFlag` są **symetryczne** (SET/CLEAR razem z `visited`). Tylko companion flags są SET‑only. Patrz §13.4 dla rollback implications.

Test enforce'ujący kontrakt: `TestGraceCompanionFlagsSetOnlyNotCleared` (`tests/grace_companion_flags_test.go:74`).

## 10. Current implemented behavior

### 10.1 Backend endpoints (Wails bindings)

| Endpoint | Sygnatura | Co robi |
|---|---|---|
| `GetGraces(slotIndex) ([]db.GraceEntry, error)` | `app_world.go:14` | Zwraca pełną listę gracji (`db.GetAllGraces`) z `Visited` wypełnionym z bitfieldu |
| `SetGraceVisited(slotIndex, graceID, visited) error` | `app_world.go:43` | Patrz §4 mental model |

`GetGraces` jest **read‑only** i toleruje brakujący `EventFlagsOffset` (skip i `Visited=false`). Niewzdraża się na warningach (`fmt.Printf` zamiast error propagation).

`SetGraceVisited` validuje:
- `a.save != nil`,
- `slotIndex ∈ [0, 10)`,
- `slot.EventFlagsOffset` w sensownym range.

Brak walidacji `graceID` przeciw `IsKnownGraceID(graceID)` — endpoint **akceptuje dowolne ID**. Próba ustawienia nieznanej flagi przejdzie do `db.SetEventFlag` i albo zaktualizuje bit albo wróci błąd z resolvera. `needs verification`: czy nieznane `graceID` poza `data.Graces` ale w paśmie BST powoduje SET na bicie nie‑gracji.

### 10.2 UI write paths (frontend)

Z `frontend/src/components/WorldTab.tsx`:

| Handler | Co wywołuje | Tier risk |
|---|---|---|
| `handleGraceToggle(grace, visited)` | `SetGraceVisited` per‑grace | brak modalu (single toggle) |
| `handleUnlockRegionGraces(rg)` | `SetGraceVisited(true)` per gracja regionu (Promise.all) | brak modalu (bulk per region) |
| `handleUnlockAllGraces()` | `SetGraceVisited(true)` na wszystkich nieaktywnych (filter: skipBossArenas + Ashen Capital opt‑in) (Promise.all) | **Tier 1** `RiskActionButton riskKey="bulk_grace_unlock"` |
| `handleLockAllGraces()` | `SetGraceVisited(false)` na wszystkich aktywnych (Promise.all) | brak modalu |

UI nota w `WorldTab.tsx:406`:

> Sites of Grace unlocked here will appear on the map and become available for fast travel. Some graces may still play their in‑world activation sequence when visited.

Ta nota świadomie nie obiecuje pełnej aktywacji wizualnej.

`needs verification`: bulk handlery wywołują `SetGraceVisited` w `Promise.all` — każdy call jest **osobnym** `pushUndo`. Sekwencja N wywołań tworzy N snapshotów undo, nie 1 bulk snapshot. Test pokrycia: brak isolated test "Promise.all bulk SetGraceVisited — undo stack size N".

## 11. PvP module status

`app_pvp.go::PrepPvP` ma w `PvPPreparationOptions` field `SitesOfGrace bool`. Branch w kodzie (linie 108‑110):

```go
if opts.SitesOfGrace {
    warnings = append(warnings, "Sites of Grace module is planned but not enabled in this version.")
}
```

To **placeholder** — moduł:

- nie czyta `data.Graces`,
- nie wywołuje `SetGraceVisited`,
- nie ustawia żadnych flag,
- nie modyfikuje `slot.Data` w żaden sposób.

Cała aktywacja gracji w PvP prep ścieżce jest **no‑op z warning'iem**. Użytkownik PvP, który chce odblokować wszystkie gracje, musi użyć osobno `Unlock All` z `WorldTab` UI.

`needs verification`: czy intencja jest, żeby moduł PvP w przyszłości robił bulk unlock (analog do `handleUnlockAllGraces`), czy bardziej selektywny zestaw (tylko gracje arena bossów, tylko overworld) — nie udokumentowane w kodzie. Master status PvP modułów — patrz [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md).

## 12. Relation to Event Flags

Wszystkie operacje na flagach gracji + DoorFlag + companion flags przechodzą przez **generic helper API** z [15-event-flags.md](15-event-flags.md):

- `db.GetEventFlag(flags, id)` — odczyt bitu,
- `db.SetEventFlag(flags, id, value)` — symetryczny SET/CLEAR,
- `resolveEventFlagPosition` — 3‑tier resolver (precomputed → BST → fallback).

Ten rozdział **nie powiela** byte/bit indexing, BST mechaniki, snapshot dat. Jedyny lokalny dodatek to:

- ID range 71000–76960 jako używana przestrzeń,
- companion flag fan‑out (§8) i kontrakt SET‑only (§9) jako specyficzna polityka gracji.

`needs verification`: zakres 71000–76960 jest opisowy (top‑level pasmo BST). Dokładny `min/max` dla 419 wpisów w `data.Graces` nie jest cytowany — można policzyć `awk` na bieżącej bazie.

## 13. Relation to Map / World State

`SetGraceVisited` **nie dotyka** żadnej z warstw Map Reveal (patrz [27-map-reveal.md](27-map-reveal.md)):

- L0 `UnlockedRegions` — nieruszone,
- L1 MapVisible / MapAcquired event flags 62xxx/63xxx — nieruszone,
- L2 DLC Cover Layer (BloodStain coords) — nieruszone (patrz [29-dlc-black-tiles.md](29-dlc-black-tiles.md)),
- L3 FoW bitfield — nieruszone.

Odwrotnie: `RevealAllMap` / `revealBaseMap` / `revealDLCMap` (patrz 27) **nie ustawiają** żadnych grace flag ani companion flags. Te dwa systemy są **niezależne** w SaveForge.

`WorldState` / `WorldArea` / `WorldGeomMan` (patrz [16-world-state.md](16-world-state.md)) — `SetGraceVisited` nie modyfikuje żadnej z tych sekcji.

`needs verification`: czy gra wyświetla znacznik gracji na mapie tylko jeśli odpowiedni `MapVisible` flag jest też SET (map fragment posiadany) — nie zweryfikowano. Możliwe że bit gracji wystarcza per se, możliwe że region musi być też ujawniony.

## 14. Relation to Game State

`SetGraceVisited` **nie modyfikuje** `LastRestedGrace` (`PreEventFlagsScalars`, patrz [14-game-state.md](14-game-state.md)):

```go
// app_world.go:41
// Does not touch LastRestedGrace — the game updates that automatically on arrival.
```

Powód: `LastRestedGrace` to BonfireId (osobny namespace — §6.3), zarządzany runtime przez grę przy fizycznym dotknięciu / teleportacji. Ustawienie tego pola edytorem nie jest potrzebne; gra je nadpisze przy pierwszym arrival.

Drugie wystąpienie BonfireId — w sekcji NetworkManager (`slot.Data[≈0x1F636A]` w testowym slocie, ~1300 B za końcem EventFlags) — również **nieruszone** przez edytor. Empiryczna obserwacja z save diff 2026‑05‑09. `needs verification` co do dokładnego offsetu w innych slotach / wersjach.

`SetGraceVisited` nie wpływa też na: `TotalDeathsCount`, `InGameCountdownTimer`, NG+ stan ([14-game-state.md](14-game-state.md)).

## 15. Write path and rollback caveats

### 15.1 Atomowość

`SetGraceVisited` ma **3 mutacje** w jednym wywołaniu (przy `visited=true` z DoorFlag + companion flags):

1. bit gracji (`SetEventFlag(graceID, true)`),
2. opcjonalnie bit drzwi (`SetEventFlag(DoorFlag, true)`),
3. opcjonalnie N bitów companion (`SetEventFlag(companions[i], true)`).

Brak per‑flag rollbacku. Jeśli krok 1 powiedzie się, a krok 2 zwróci error, error jest propagowany do callera ale **krok 1 zostaje** (bit gracji już ustawiony). Krok 3 (companion flags) używa `fmt.Printf` zamiast error propagation — błędy są **logowane, nie zwracane**.

### 15.2 Snapshot undo (save‑level)

`a.pushUndo(slotIndex)` na początku `SetGraceVisited` — **jedyna** ścieżka rollbacku. Pełen snapshot slotu pre‑mutation. Per‑bit selektywny undo nie istnieje.

Bulk handler z UI (`handleUnlockAllGraces`) robi N×`SetGraceVisited` przez `Promise.all` — czyli N×`pushUndo`. Stack rośnie liniowo z liczbą gracji. `needs verification`: czy `pushUndo` ma limit głębokości i czy bulk z ~419 graces nie wycina starszych snapshotów.

### 15.3 Brak idempotency check

`SetGraceVisited(graceID, true)` na już SET bicie ponownie przejdzie przez `pushUndo` + `SetEventFlag` + companion fan‑out. Idempotentne w sensie wyniku (bit pozostaje `=1`), ale **nie no‑op** w sensie kosztu (snapshot, mutacje).

### 15.4 Companion flags rollback asymmetry

Snapshot undo cofnie **wszystkie** zmiany bitfieldu — łącznie z companion flags ustawionymi w tym samym call. To OK z punktu widzenia logiki undo, ale tworzy subtelny edge case:

- jeśli companion flag był już `=1` z innej ścieżki (item, game progress),
- a `SetGraceVisited(true)` + `Undo` zostanie wykonany,
- snapshot cofnie do stanu pre‑mutation, w którym companion flag był `=1`.

W praktyce: companion flag może wrócić do `=1` po Undo, mimo że logika SET‑only by go „kasowała”. To poprawne zachowanie (Undo to czysty restore), ale warto być świadomym.

## 16. Validation and safety notes

### 16.1 Stale data po patchu gry

419 wpisów w `data.Graces` to **snapshot** z `regulation.bin` — nie regenerowany automatycznie. Po patchu DLC dodającym gracje pasmo `data.Graces` może nie pokrywać nowych ID. Konsekwencje:

- nowe gracje nie pojawią się w UI,
- bulk Unlock All nie ustawi nowych bitów,
- `IsKnownGraceID(newID)` zwróci `false`.

`needs verification`: brak automatycznej detekcji „regulation.bin newer than graces.go snapshot”.

### 16.2 Wrong EventFlag IDs

`SetGraceVisited` nie waliduje `graceID` przeciw `data.Graces`. Wywołanie z UI zawsze jest poprawne (UI iteruje po `db.GetAllGraces`), ale ktoś callujący z Wails JS / testów może przekazać dowolny ID. Skutek: niezweryfikowana flaga zostanie ustawiona.

### 16.3 Quest / NPC progression side effects

Companion flags Gatefront (Spectral Steed Whistle, Melina Accord) **są** progression flagami quest‑NPC. Ustawienie ich edytorem na save sprzed pierwszego Melina encounter:

- może pominąć cutscenę Meliny,
- może spowodować, że NPC nie pojawi się w spodziewanych miejscach,
- może zakłócić sekwencję triggerów RTH (Melina invitation).

`needs verification`: SaveForge **nie** ostrzega przed tym przed wywołaniem `SetGraceVisited(76111, true)` na świeżym save.

### 16.4 SET‑only nie można undo per‑flag

Patrz §9 + §15.4. Jeżeli użytkownik chce **rozdzielić** „gracja visited” od „companion flags set”, brak takiej kontroli — single endpoint.

### 16.5 Brak atomic rollback dla multi‑grace bulk

Bulk Unlock All — N×`SetGraceVisited` przez `Promise.all`. Jeśli np. 200/419 wywołań przejdzie a 201. zwróci error, brak `rollback to pre‑bulk state`. UI cache (`setGraces(prev => ...)`) zaktualizuje tylko te udane, ale stack undo jest 200 osobnych snapshotów.

### 16.6 PvP module placeholder

`opts.SitesOfGrace=true` w `ApplyPvPPreparation` nie ustawi gracji. User‑facing efekt: warning w return value, ale brak fail‑loud (no error). Możliwe, że użytkownik PvP zakłada, że moduł robi unlock — patrz [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md).

### 16.7 In‑world activation sequence po patchu

Założenie „EMEVD wyprowadza visual state z bit gracji przy load obszaru” było zweryfikowane na Church of Elleh w 2026‑05‑09. Nowsze patche mogą zmienić EMEVD scripting. `needs verification` po każdym dużym patchu gry.

## 17. Test coverage

| Test | Plik | Co pokrywa |
|---|---|---|
| `TestGraceCompanionFlagsSetOnRealSave` | `tests/grace_companion_flags_test.go:15` | `CompanionEventFlagsForGrace(76111)` zwraca niepustą listę; każda flaga settable + readable na realnym slocie |
| `TestGraceCompanionFlagsNoRTHFlags` | `tests/grace_companion_flags_test.go:49` | Forbidden list (RTH, transient, level‑up) NIE pojawia się w Gatefront companion set |
| `TestGraceCompanionFlagsSetOnlyNotCleared` | `tests/grace_companion_flags_test.go:74` | Symuluje „flagi ustawione inną ścieżką”; weryfikuje że pozostają SET (SET‑only contract) |
| `TestCompanionEventFlagsForGrace_Gatefront` | `backend/db/data/grace_companion_flags_test.go:5` | Companion set dla Gatefront zawiera dokładnie 4 oczekiwane stałe (`EventFlagObtainedSpectralSteedWhistle`, `EventFlagMelinaGaveWhistle`, `EventFlagWhistleWorldState`, `EventFlagMelinaAcceptRefusePopup`) |
| `TestCompanionEventFlagsForGrace_Unknown` | `backend/db/data/grace_companion_flags_test.go:28` | `CompanionEventFlagsForGrace(0xDEADBEEF)` zwraca `nil` (unknown grace ID) |
| `TestCompanionEventFlagsForGrace_NoForbiddenFlags` | `backend/db/data/grace_companion_flags_test.go:35` | Iteruje **wszystkie** companion sets w `graceCompanionEventFlags`; sprawdza, że żaden z 9 forbidden ID nie występuje |

**Brak isolated testu dla**:

- `SetGraceVisited` endpoint (jako całość — pushUndo + 3 mutacje + error propagation),
- `GetGraces` z różnymi `EventFlagsOffset` stanami,
- bulk Promise.all z UI,
- DoorFlag SET/CLEAR symmetry per‑category (catacomb vs hero_grave).

`needs verification`: dodanie `TestSetGraceVisitedRoundtrip` (load save → SetGraceVisited(true) → assert bit gracji + DoorFlag + companion flags) byłoby pożądane. Aktualnie pokrycie jest pośrednie — przez companion tests + manualny in‑game test.

## 18. Known limits / needs verification

Skondensowana lista otwartych pytań:

1. **Companion flags per pozostałe ~418 gracji** — pusty pool (`needs verification`: które gracje wymagają fan‑out).
2. **Liczba 419 stale after patch** — brak auto‑regeneracji z `regulation.bin`.
3. **DLC vs base game klasyfikator** — brak `IsDLCGraceID` analogu do `IsDLCMapFlag`.
4. **DoorFlag two‑way symmetry** — gra może traktować CLEAR jako no‑op.
5. **In‑world activation per‑category** — zweryfikowane tylko dla Church of Elleh overworld.
6. **`SetGraceVisited` walidacja `graceID`** — brak `IsKnownGraceID` check przed mutacją.
7. **Bulk Promise.all undo stack** — N snapshotów; brak per‑bulk single snapshot.
8. **PvP module intent** — placeholder, brak udokumentowanej docelowej semantyki.
9. **Quest progression side effects** — brak warningu pre‑mutation dla flag NPC‑progression‑critical.
10. **Drugie wystąpienie BonfireId w NetworkManager** — empiryczna obserwacja jednego slotu; nie zweryfikowane cross‑slot / cross‑platform.

## 19. Cross‑references

- [11-regions.md](11-regions.md) — `UnlockedRegions`; nieruszone przez `SetGraceVisited`.
- [14-game-state.md](14-game-state.md) — `LastRestedGrace`, `PreEventFlagsScalars`; nieruszone przez edytor.
- [15-event-flags.md](15-event-flags.md) — master event flag API; rozdział 47 jest jego callerem.
- [16-world-state.md](16-world-state.md) — `WorldGeomMan` / `WorldArea`; nieruszone.
- [27-map-reveal.md](27-map-reveal.md) — 4‑layer Map Reveal; niezależny od grace activation.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — L2 DLC Cover Layer; niezależne.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — PvP modules; `SitesOfGrace` placeholder.
- [50-item-companion-flags.md](50-item-companion-flags.md) — item companion flags; **współdzielone** stałe `EventFlagObtainedSpectralSteedWhistle` itd.

## 20. Sources

- `app_world.go::GetGraces` / `SetGraceVisited`.
- `backend/db/data/graces.go` — `Graces` map, `Cat()`/`HG()` constructors, `GraceData` struct.
- `backend/db/data/grace_companion_flags.go` — `graceCompanionEventFlags`, `CompanionEventFlagsForGrace`, `GatefrontGraceEventFlagID`.
- `backend/db/db.go` — `GraceEntry`, `GetAllGraces`, `IsKnownGraceID`, region mapping.
- `app_pvp.go:108‑110` — placeholder `SitesOfGrace` module.
- `frontend/src/components/WorldTab.tsx` — UI bindings i bulk handlers.
- `tests/grace_companion_flags_test.go` — 3 integration testy.
- `backend/db/data/grace_companion_flags_test.go` — unit testy stałych.
- `docs/CHANGELOG.md` — historyczny runtime save diff Church of Elleh (2026‑05‑09).
