# 50 — Item Companion Flags

> **Type**: Binary format spec + design doc (canonical chapter)
> **Scope**: Synchronizacja event flag z dodawaniem / usuwaniem przedmiotów w SaveForge — symetryczny SET+CLEAR contract, mapowanie 6 itemów na zestawy companion flag, write path w `AddItemsToCharacter` / `RemoveItemsFromCharacter`.

Cross‑refs: [11-regions.md](11-regions.md), [14-game-state.md](14-game-state.md), [15-event-flags.md](15-event-flags.md), [16-world-state.md](16-world-state.md), [27-map-reveal.md](27-map-reveal.md), [29-dlc-black-tiles.md](29-dlc-black-tiles.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md), [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md).

---

## 1. Cel rozdziału

Zdefiniować jednoznacznie:

- czym jest **item companion flag** (zestaw event flag „dokładanych” do bitfieldu razem z dodawanym itemem),
- które itemy są obsługiwane w aktualnym kodzie (6 wpisów na 2026‑05‑21),
- jak działa **symetryczny SET+CLEAR**: SET na dodanie, CLEAR na usunięcie ostatniej instancji,
- jak się to różni od asymetrycznego SET‑only kontraktu w `grace_companion_flags` (patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md)),
- jak się to różni od `MapFragmentItems` (osobny mechanizm w [27-map-reveal.md](27-map-reveal.md) — **nie** część item companion flags),
- gdzie kończy się implementacja a zaczyna `needs verification`.

Nie powiela helper API event flag (patrz [15-event-flags.md](15-event-flags.md)) ani polityki SET‑only dla gracji (patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md)).

## 2. Status

| Aspekt | Status |
|---|---|
| Backend map `itemCompanionEventFlags` | ✅ `backend/db/data/item_companion_flags.go` — 6 wpisów (Whistle + 5 multiplayer items) |
| API `CompanionEventFlagsForItem(itemID)` | ✅ tamże, linia 104 |
| Hook SET — `AddItemsToCharacter` | ✅ `app.go:569‑578` |
| Hook CLEAR — `RemoveItemsFromCharacter` | ✅ `app.go:706‑725` (warunek: ostatnia instancja itemu usunięta) |
| Repair‑mode SET (item na maxie ilości też SET'uje flagi) | ✅ — patrz §11 |
| Unit tests | ✅ 11 testów w `backend/db/data/item_companion_flags_test.go` |
| Integration tests | ✅ 17 testów w `tests/item_companion_flags_test.go` |
| Runtime verification PS4 | ✅ historycznie potwierdzony PS4 test 2026‑05‑11 (Spectral Steed Whistle) |
| Talisman Pouch sync (60500/60510/60520) | ❌ **nie** w `itemCompanionEventFlags` (`needs verification` historycznych iteracji) |
| `MapFragmentItems` jako item companion | ❌ osobny mechanizm (patrz §10) |

## 3. Source of truth w kodzie

| Plik / symbol | Co zawiera |
|---|---|
| `backend/db/data/item_companion_flags.go::itemCompanionEventFlags` | Mapa `itemID → []companionFlag` (6 wpisów) |
| `backend/db/data/item_companion_flags.go::CompanionEventFlagsForItem` | Lookup API |
| `backend/db/data/item_companion_flags.go` (const block) | 6 stałych `Item...` (item IDs) + 9 stałych `EventFlag...` (companion flag IDs) |
| `app.go::AddItemsToCharacter` (linie 569‑578) | Hook SET — wywoływany **per‑prepared‑item** po `AboutTutorialID` |
| `app.go::RemoveItemsFromCharacter` (linie 658‑666 pre‑scan, 706‑725 CLEAR) | Hook CLEAR — wywoływany tylko gdy `slot.GaItems` po remove nie zawiera już danego itemID |
| `backend/db/data/item_companion_flags_test.go` | 11 unit testów (Whistle, Unknown, NoTransientFlags, 4 multiplayer per-item, SmallGoldenEffigy NoForbiddenFlags, Multiplayer NoForbiddenFlags, Multiplayer NoCrossContamination) |
| `tests/item_companion_flags_test.go` | 17 integration testów (SetOnRealSave, ForbiddenAbsent, MechanicFlagPresent, ClearOnFlagData, RemainingItemPreventsClearing, per‑item Set/Clear/Repair, NoSummoningPoolFlags, UnrelatedItemDoesNotAffect) |

## 4. Mental model

```
AddItemsToCharacter(prepared)
  for each p in prepared:
    ... (item add logic) ...
    AppendTutorialID(...)
    if companions := CompanionEventFlagsForItem(p.baseID); len(companions) > 0:
        for f in companions:
            SetEventFlag(eflags, f, true)        ← SET-only-on-add hook

RemoveItemsFromCharacter(handles)
  pre-scan: collect itemIDs with companion flags being removed
  ... (item remove logic) ...
  for itemID in companionRemovals:
      if no remaining instance of itemID in slot.GaItems:
          for f in CompanionEventFlagsForItem(itemID):
              SetEventFlag(eflags, f, false)    ← CLEAR-on-last-removed hook
```

**Symetria SET/CLEAR** jest jednak ograniczona — patrz §9.

## 5. Item companion flag data model

`itemCompanionEventFlags` (`item_companion_flags.go:13`) ma typ `map[uint32][]uint32` — klucz to **item baseID** (`0x4000xxxx` itd.), wartość to slice **event flag IDs** ustawianych razem z itemem. Aktualnie 6 wpisów:

| Wpis | Item baseID | Liczba companion flags |
|---|---|---|
| `ItemSpectralSteedWhistle` | `0x40000082` | 4 |
| `ItemSmallGoldenEffigy` | `0x4000006D` | 1 |
| `ItemDuelistsFurledFinger` | `0x40000065` | 1 |
| `ItemSmallRedEffigy` | `0x4000006E` | 1 |
| `ItemWhiteCipherRing` | `0x40000068` | 1 |
| `ItemBlueCipherRing` | `0x40000069` | 1 |

Wszystkie item IDs i flag IDs są deklarowane jako stałe `const` w tym samym pliku — brak hardkodowanych literalów w mapie. Pozwala to na **dzielenie stałych** z innymi modułami (np. `EventFlagObtainedSpectralSteedWhistle` używana także przez `grace_companion_flags.go` dla Gatefront — patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §8).

`needs verification`: liczba 6 wpisów jest snapshotem z `item_companion_flags.go` na 2026‑05‑21. Brak automatycznej regeneracji ze źródeł gry — przyszłe wpisy są dodawane ręcznie. Patrz §13 dla procedury.

## 6. Supported companion mappings

### 6.1 Spectral Steed Whistle — `0x40000082`

| Flag const | ID dec | Komentarz w kodzie |
|---|---|---|
| `EventFlagObtainedSpectralSteedWhistle` | `60100` | Unlocks Torrent. Without it the game refuses to summon Torrent even when whistle is in inventory. |
| `EventFlagMelinaGaveWhistle` | `4680` | Marks Melina quest‑give step as complete. Prevents the "accept Torrent?" dialogue from re‑triggering at graces. |
| `EventFlagWhistleWorldState` | `710520` | Map/world counterpart set simultaneously with 60100 during the in‑game event. |
| `EventFlagMelinaAcceptRefusePopup` | `4681` | Marks accept/refuse popup as shown. Prerequisite before the give step in the Melina quest chain. |

Wszystkie 4 stałe są też używane w `grace_companion_flags.go::GatefrontGraceEventFlagID` companion set — **współdzielenie zamierzone**. Patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §8.

`needs verification`: korelacja 4 flag i mechaniki Torrenta jest potwierdzona historycznie (PS4 runtime test 2026‑05‑11 — patrz CHANGELOG). Nie istnieje izolowany test, który sprawdza wszystkie 4 flagi `=1` po `AddItemsToCharacter` z `ItemSpectralSteedWhistle` (test `TestCompanionEventFlagsForItem_Whistle` weryfikuje wyłącznie *zawartość* slice'a, nie *roundtrip* na realnym slocie).

### 6.2 Multiplayer pickup items

Każdy z 5 itemów ma **dokładnie 1** companion flag — własną flagę `Obtained...`:

| Item | Item baseID | Companion flag | ID dec | Komentarz w kodzie |
|---|---|---|---|---|
| Small Golden Effigy | `0x4000006D` | `EventFlagObtainedSmallGoldenEffigy` | `60230` | Without it the pickup/interact state at Effigies of the Martyr can remain visible even when the item is already in inventory. |
| Duelist's Furled Finger | `0x40000065` | `EventFlagObtainedDuelistsFurledFinger` | `60240` | Without it the pickup/interact state can remain visible in the world. |
| Small Red Effigy | `0x4000006E` | `EventFlagObtainedSmallRedEffigy` | `60250` | jw. |
| White Cipher Ring | `0x40000068` | `EventFlagObtainedWhiteCipherRing` | `60280` | Marks item as obtained / shop sold‑out. Source: er-save-manager + ShopLineupParam.csv row 101800 `eventFlag_forStock`. |
| Blue Cipher Ring | `0x40000069` | `EventFlagObtainedBlueCipherRing` | `60290` | Confirmed by ShopLineupParam.csv row 101801 + Steam Deck before/after shop purchase diff. |

`needs verification`: dla Cipher Ringów flaga pełni rolę „shop sold‑out” w `Twin Maiden Husks` (ShopLineupParam.csv `eventFlag_forStock`). Czy gra waliduje uniqueness Cipher Ringów *wyłącznie* przez tę flagę, czy także przez inwentarz — **nie zostało rozstrzygnięte**. Komentarz w kodzie sygnalizuje, że bez tej flagi sklep „nadal oferuje item”.

### 6.3 Co NIE wchodzi do `itemCompanionEventFlags`

Wpisy explicit excluded w komentarzach kodu:

| Pominięte | Powód (z komentarza) |
|---|---|
| `710770`, `69090`, `69370` (Melina opuszcza Gatefront) | Research candidate; runtime PS4 test 2026‑05‑11 potwierdził, że nie są wymagane |
| Summoning Pool activation flags (`670xxx`) | Osobny mechanizm — aktywacja Martyr Effigy |
| Flagi innych multiplayer items niewymienionych | Po prostu nie dodane jeszcze; nie ma negatywnej decyzji |

Test `TestCompanionEventFlagsForItem_NoTransientFlags` (`item_companion_flags_test.go:35`) enforce'uje wykluczenie 5 transient ID: `4698`, `4651`, `4652`, `4653`, `4656`. Pasmo `670xxx` jest enforce'owane oddzielnie w `TestCompanionEventFlagsForItem_SmallGoldenEffigy_NoForbiddenFlags` i `TestCompanionEventFlagsForItem_MultiplayerPickup_NoForbiddenFlags`.

`needs verification`: brak Talisman Pouch (`60500`/`60510`/`60520`) — wpis ten **nie istnieje** w `itemCompanionEventFlags`. Historyczne dyskusje sugerowały hipotezę „SetGraceVisited synchronizuje Talisman Pouch slots”, ale aktualny kod nie ma tego mechanizmu — ani w `item_companion_flags.go`, ani w `grace_companion_flags.go`, ani w `app_world.go`. Czy gra rzeczywiście wymaga sync — `needs verification`.

## 7. SET + CLEAR symmetric behavior

**Kontrakt symetryczny** dla item companion flags:

| Akcja | Co się dzieje z companion flags |
|---|---|
| `AddItemsToCharacter` z itemem mającym companion set | **SET** wszystkich flag z setu (idempotent — bit już SET pozostaje SET) |
| `RemoveItemsFromCharacter` z handle ostatniej instancji itemu | **CLEAR** wszystkich flag z setu |
| `RemoveItemsFromCharacter` z handle nie‑ostatniej instancji (po remove jeszcze są inne stacky/handles) | **No‑op** — flagi pozostają SET |
| Re‑add po usunięciu ostatniej instancji | **SET** ponownie |

Test `TestCompanionFlagsRemainingItemPreventsClearing` (`tests/item_companion_flags_test.go:354`) enforce'uje warunek „nie‑ostatnia instancja → no‑op CLEAR”.

To **zasadniczo inny** kontrakt niż w `grace_companion_flags`:

| Cecha | `item_companion_flags` (ten rozdział) | `grace_companion_flags` ([47](47-site-of-grace-activation.md) §9) |
|---|---|---|
| SET on activate | ✅ | ✅ |
| CLEAR on deactivate | ✅ (gdy ostatnia instancja usunięta) | ❌ (SET‑only) |
| Argument | item baseID | grace event flag ID |
| Wywoływane z | `AddItems` / `RemoveItems` (item lifecycle) | `SetGraceVisited` |
| Asymetria | brak — pełna symetria | celowa — companion flags mogą być SET inną ścieżką |

Powód, dla którego item companion flags **mogą** być CLEAR'owane: dla item lifecycle decyzja „item zniknął ze slotu” jest jednoznaczna (`slot.GaItems` skanowane post‑removal). Dla grace lifecycle taka jednoznaczność nie istnieje — companion flag może być SET przez item (Whistle), grace (Gatefront) lub progress, więc CLEAR na deactivate gracji ryzykowałby regression.

## 8. Difference from grace companion flags

Już omówione w §7. Skrótowo: **wspólne stałe, różne polityki**:

- Stałe `EventFlagObtainedSpectralSteedWhistle`, `EventFlagMelinaGaveWhistle`, `EventFlagWhistleWorldState`, `EventFlagMelinaAcceptRefusePopup` są **współdzielone** między oboma plikami (jedyna deklaracja w `item_companion_flags.go`, `grace_companion_flags.go` re‑używa importu).
- Mapy `itemCompanionEventFlags` i `graceCompanionEventFlags` są **rozłączne** (różne klucze — item baseID vs grace event flag ID).
- API `CompanionEventFlagsForItem` i `CompanionEventFlagsForGrace` zwracają niezależnie skomponowane sety. Nie ma cross‑lookup.

`needs verification`: czy w przyszłości któryś z dotychczas pustych pól (np. inne grace, więcej itemów) wprowadzi nakładające się companion sets — wymaga decyzji policy. Aktualnie cross‑contamination jest enforce'owane testem `TestCompanionEventFlagsForItem_MultiplayerPickup_NoCrossContamination`.

## 9. Difference from map fragment flags

`MapFragmentItems` (`backend/db/data/maps.go:318`) to **osobny** mechanizm:

| Aspekt | `itemCompanionEventFlags` (ten rozdział) | `MapFragmentItems` ([27](27-map-reveal.md), [29](29-dlc-black-tiles.md)) |
|---|---|---|
| Typ mapy | `map[uint32][]uint32` (itemID → []eventFlag) | `map[uint32]uint32` (visible flag ID → item ID) |
| Kierunek | item → companion flags | visible flag → fragment item |
| Caller | `AddItemsToCharacter` / `RemoveItemsFromCharacter` (item lifecycle) | `revealBaseMap` / `revealDLCMap` (map reveal) |
| Liczba wpisów | 6 | 24 (19 base game + 5 DLC) |
| Lookup | `CompanionEventFlagsForItem(itemID)` | inline `data.MapFragmentItems[flagID]` |
| Set/Clear contract | symmetric SET+CLEAR | nie ma — `revealBaseMap/DLC` tylko ADD itemów (przez `AddItemsToSlot`); `ResetMapExploration` USUWA itemy przez `RemoveItemByBaseID` |

`MapFragmentItems` **nie korzysta** z `CompanionEventFlagsForItem`. To dwa niezależne mechanizmy:

- Map reveal: visible flag SET → dodaj odpowiadający fragment item (`AddItemsToSlot`).
- Item companion: fragment item dodany → … sprawdza `CompanionEventFlagsForItem(fragmentID)` → zwraca `nil` (brak wpisu), więc **żadne** flagi event nie są SET'owane jako companion.

Czyli: ustawienie `MapVisible` flagi w Phase 1 `revealDLCMap` (patrz [27](27-map-reveal.md) §11 / [29](29-dlc-black-tiles.md) §11) nie pociąga za sobą companion flag fan‑outu. Phase 2 dodaje fragment item, ale `AddItemsToSlot` (core, bezpośrednio) nie idzie ścieżką `AddItemsToCharacter` (app), więc hook SET (`app.go:569`) **nie jest wywołany** dla map fragment items.

`needs verification`: czy mapowanie fragment item → companion flag *byłoby* potrzebne (czyli czy gra robi coś specjalnego z fragment items poza ich obecnością w inventarzu) — nie zweryfikowano. Aktualnie założenie jest, że fragment items są wystarczające bez companion flags.

## 10. Current implemented behavior

### 10.1 Hook SET — `AddItemsToCharacter`

`app.go:569‑578`:

```go
if companions := data.CompanionEventFlagsForItem(p.baseID); len(companions) > 0 {
    if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
        eflags := slot.Data[slot.EventFlagsOffset:]
        for _, f := range companions {
            if err := db.SetEventFlag(eflags, f, true); err != nil {
                runtime.LogWarningf(a.ctx, "companion flag %d for item 0x%08X: %v", f, p.baseID, err)
            }
        }
    }
}
```

Charakterystyka:

- **Per‑prepared‑item** — iteracja po `prepared` slice, czyli dla każdego itemu pasującego do request (nawet jeśli `actualInv == 0 && actualStorage == 0` — patrz §11 repair mode).
- **Po `AboutTutorialID`** — flagi towarzyszące są SET po sekcji tutorial ID, w tym samym wewnętrznym bloku.
- **Pre‑rollback** — hook wykonuje się **przed** `ValidatePostMutation` (`app.go:626`). W razie post‑validation failure cały slot jest restored z `snapshot` (`core.RestoreSlot`), więc companion flags są cofnięte razem ze wszystkim innym.
- **Tolerancja `EventFlagsOffset`** — jeśli offset nie jest sensowny, blok SET jest pomijany **bez** propagacji błędu (silent skip).
- **Error handling per‑flag** — `db.SetEventFlag` może zwrócić error (out‑of‑range etc.). Error jest **logowany przez `runtime.LogWarningf`**, **nie propagowany**. Iteracja kontynuuje — pozostałe flagi w secie nadal SET'owane.

### 10.2 Hook CLEAR — `RemoveItemsFromCharacter`

`app.go:658‑666` (pre‑scan) + `app.go:706‑725` (CLEAR):

```go
// Pre-scan: collect item IDs with companion flags being removed.
companionRemovals := make(map[uint32]bool)
for _, handle := range handles {
    if itemID, ok := slot.GaMap[handle]; ok {
        if len(data.CompanionEventFlagsForItem(itemID)) > 0 {
            companionRemovals[itemID] = true
        }
    }
}
// ... (remove items) ...
// Clear companion flags for items no longer present in slot after removal.
if len(companionRemovals) > 0 && slot.EventFlagsOffset > 0 && ... {
    eflags := slot.Data[slot.EventFlagsOffset:]
    for itemID := range companionRemovals {
        remaining := false
        for _, g := range slot.GaItems {
            if !g.IsEmpty() && g.ItemID == itemID {
                remaining = true
                break
            }
        }
        if !remaining {
            for _, f := range data.CompanionEventFlagsForItem(itemID) {
                if err := db.SetEventFlag(eflags, f, false); err != nil {
                    runtime.LogWarningf(...)
                }
            }
        }
    }
}
```

Charakterystyka:

- **Pre‑scan przed remove** — `companionRemovals` wypełniany przed właściwym usunięciem przez `core.RemoveItemFromSlot`.
- **CLEAR tylko gdy ostatnia instancja usunięta** — pełen skan `slot.GaItems` post‑remove sprawdza, czy item nadal istnieje gdziekolwiek w slocie. Jeśli tak — CLEAR jest pomijany.
- **Brak `pushUndo` dla CLEAR** — `RemoveItemsFromCharacter` ma `a.pushUndo(charIdx)` na linii 654, **przed** pętlą remove. Snapshot obejmuje cały slot, więc CLEAR jest cofalny przez Undo.
- **Error handling per‑flag** — analogicznie do SET: `runtime.LogWarningf`, brak propagacji.

### 10.3 Brak walidacji `itemID`

Hook SET nie waliduje, czy `p.baseID` jest *znanym* itemem (`db.ItemByBaseID` lookup). `CompanionEventFlagsForItem(unknownID)` zwraca `nil`, więc blok jest no‑op. To OK z punktu widzenia bezpieczeństwa, ale oznacza, że typo w item ID w wywołaniu zewnętrznym nie zostanie wykryte przez ten mechanizm.

## 11. Write path and rollback caveats

### 11.1 Repair mode

Hook SET wywoływany jest **dla każdego itemu w `prepared`** — także dla itemów, które już są w slocie na maksymalnej ilości (`actualInv + actualStorage == 0` po `AddItemsToCharacter` clamping). Konsekwencja: ponowne kliknięcie „Add” na itemie typu Spectral Steed Whistle na slocie który już go ma, ale **bez** companion flag (np. save edytowany przed wprowadzeniem mechanizmu) **naprawi** brakujące companion flags.

Test `TestSmallGoldenEffigyRepair` (`tests/item_companion_flags_test.go:274`) + `TestMultiplayerPickupFlagRepair` (`tests/item_companion_flags_test.go:506`) — pokrycie tej ścieżki.

### 11.2 Atomowość i rollback w `AddItemsToCharacter`

`AddItemsToCharacter` ma **snapshot‑level rollback**:

```go
snapshot := core.SnapshotSlot(slot)   // przed mutacjami
... // item add + flags + tutorials + companions
if violations := core.ValidatePostMutation(slot); len(violations) > 0 {
    core.RestoreSlot(slot, snapshot)
    return result, fmt.Errorf("rollback: post-mutation validation failed: %s", ...)
}
```

Hook SET (companion flags) jest **pre‑validation** — jeśli post‑validation failuje, snapshot restore cofnie companion flags razem z item add.

Brak per‑flag atomicity wewnątrz hook'a SET. Jeśli `SetEventFlag(eflags, f, true)` failuje w połowie listy (np. f=3/4 zwraca error), pozostałe flagi nie są retried; ich stan zależy od kolejności i tego, czy poprzednie wywołania zdążyły SET. Snapshot restore (jeśli post‑validation failuje) sprząta to ostatecznie.

### 11.3 Rollback w `RemoveItemsFromCharacter`

`RemoveItemsFromCharacter` **nie ma** post‑validation rollback. `a.pushUndo` na początku tworzy snapshot dla potencjalnego użytkowego Undo, ale błąd w trakcie remove nie cofa zmian automatycznie. Hook CLEAR wykonuje się **po** pętli remove — jeśli któraś flaga failuje, error jest logowany, pozostałe flagi są nadal CLEAR'owane.

### 11.4 Brak atomic multi‑item

Bulk add wielu itemów per call (`prepared` slice) jest „atomic” tylko w sensie post‑validation snapshot — albo wszystkie itemy + ich companion flags trafiają do slotu, albo żaden. Per‑item rollback w trakcie iteracji nie istnieje.

### 11.5 Slice invalidation

`slot.Data` może być realokowany przez `core.AddItemsToSlot` (powiększanie GaItems / inventory). Hook SET (`app.go:569`) re‑derywuje `eflags := slot.Data[slot.EventFlagsOffset:]` *po* sekcji add — czyli na świeżym `slot.Data`. Bezpieczne.

W `RemoveItemsFromCharacter` analogicznie — hook CLEAR re‑derywuje `eflags` po pętli remove.

## 12. Validation and safety notes

### 12.1 Stale hardcoded flag IDs po patchu

Stałe `EventFlag...` w `item_companion_flags.go` są **hardkodowane** literałami `uint32(60100)` etc. Brak referencji do `regulation.bin` snapshot. Po patchu gry, który zmieniłby semantykę tych flag (mało prawdopodobne dla flag obtained, ale możliwe dla world state flags jak `710520`), companion flags mogą:

- działać OK (gra nadal czyta flagę dla tego samego celu),
- działać częściowo (część flag valid, część obsolete),
- zakłócać questy (flaga teraz oznacza coś innego).

`needs verification` po każdym dużym patchu gry. Brak mechanizmu detekcji.

### 12.2 Wrong flag ID = quest/world side effects

Każda flaga w companion set ma „side effect surface” — `60100` odblokowuje Torrent, `4680` blokuje re‑trigger Melina dialogue, etc. Wpisanie błędnej flagi do `itemCompanionEventFlags` (np. `60101` zamiast `60100`) mogłoby przypadkowo SET'ować flagę zupełnie innego mechanizmu (item, quest, world state). Brak walidacji typu „czy ta flaga jest event flag obtained pattern”.

Mitigation: 8 unit testów per‑item + 17 integration testów. Wszystkie weryfikują *konkretne* ID, więc literówka byłaby wykryta przy pierwszym `go test ./...`.

### 12.3 Missing companion flag mimo itemu w inventarzu

Możliwy w 2 scenariuszach:

1. Save edytowany przed wprowadzeniem mechanizmu (legacy save) — repair mode (§11.1) naprawia po ponownym Add.
2. `EventFlagsOffset` nie ustawiony → silent skip hook'a SET. Item w slocie, ale flag nie SET. Mitigation: repair mode po ustawieniu offsetu.

`needs verification`: czy `EventFlagsOffset` może być `<= 0` w realnym save'ie po `LoadSave`. Aktualne testy zakładają, że jest valid. Defensywny skip w hook'u SET jest „belt and suspenders”.

### 12.4 Event flag SET bez itemu

Możliwy w scenariuszu „użytkownik usunął item, ale słuchawka jest poza ostatnią instancją” (stacked items, multiple handles). Pre‑scan w `RemoveItemsFromCharacter` zlicza po itemID; CLEAR działa tylko gdy `remaining == false`. Brak race condition.

Inny scenariusz: użytkownik manualnie modyfikuje bitfield przez inne endpointy (`SetGraceVisited`, PvP prep) — wtedy flaga może być SET bez itemu. Patrz [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §9 dla rationale dla SET‑only policy gracji.

### 12.5 SET/CLEAR mismatch (race)

W ramach jednego call (`AddItemsToCharacter` lub `RemoveItemsFromCharacter`) hook jest sekwencyjny — brak współbieżności. Cross‑call race (np. Add + Remove równolegle z UI) — `app_world.go` / `app.go` zakładają single‑threaded execution per slot (Wails binding model). `needs verification`: czy Wails dispatcher serializuje wywołania per‑slot.

### 12.6 Platform / version differences

Stałe flag IDs są identyczne dla PC i PS4 (BST resolver jest cross‑platform — patrz [15-event-flags.md](15-event-flags.md)). Jednak:

- Flaga `710520` (Whistle world state) jest w paśmie wysokim — `needs verification` czy BST resolver poprawnie ją lokalizuje na PS4 (test `TestBSTLookupMatchesEventFlags` w `tests/map_flags_test.go` pokrywa pasma 62xxx/63xxx/76xxx/82xxx, ale nie 710xxx).
- Zakres `1042xxx` jest explicite excluded z companion sets (komentarz w kodzie: „Out of bounds na PS4 (offset BST ~130 MB vs ~2,3 MB tablica flag). Fizycznie nie można ustawić”). Stała `EventFlagWhistleWorldState=710520` jest w paśmie 710xxx — sprawdzony osobno (`needs verification`: ile bajtów flagi 710520 byte address vs `EventFlagsByteCount=0x1BF99F`).

### 12.7 Rollback gaps

Patrz §11. Główna gap: `RemoveItemsFromCharacter` nie ma post‑mutation validation. Jeśli CLEAR companion flag failuje, brak rollbacku — error tylko logowany. W praktyce hook CLEAR nie powinien failować (write do existing bitfield bit), ale defensywnie można dodać `pushUndo` snapshot + validation.

## 13. Test coverage

### 13.1 Unit tests (`backend/db/data/item_companion_flags_test.go`) — 11 testów

| Test | Linia | Co weryfikuje |
|---|---|---|
| `TestCompanionEventFlagsForItem_Whistle` | 5 | Whistle companion set = dokładnie 4 stałe (`EventFlagObtained...`/`MelinaGave.../WhistleWorldState/MelinaAcceptRefusePopup`) |
| `TestCompanionEventFlagsForItem_Unknown` | 28 | `CompanionEventFlagsForItem(0xDEADBEEF)` → `nil` |
| `TestCompanionEventFlagsForItem_NoTransientFlags` | 35 | Brak `4698`/`4651`/`4652`/`4653`/`4656` w żadnym companion set |
| `TestCompanionEventFlagsForItem_SmallGoldenEffigy` | 55 | SmallGoldenEffigy companion = `[60230]` (dokładnie 1 flaga) |
| `TestCompanionEventFlagsForItem_SmallGoldenEffigy_NoForbiddenFlags` | 68 | Brak `60220/60240/.../60310` ani `670xxx` ani Whistle flag w SmallGoldenEffigy |
| `TestCompanionEventFlagsForItem_DuelistsFurledFinger` | 89 | DuelistsFurledFinger = `[60240]` |
| `TestCompanionEventFlagsForItem_SmallRedEffigy` | 99 | SmallRedEffigy = `[60250]` |
| `TestCompanionEventFlagsForItem_WhiteCipherRing` | 109 | WhiteCipherRing = `[60280]` |
| `TestCompanionEventFlagsForItem_BlueCipherRing` | 119 | BlueCipherRing = `[60290]` |
| `TestCompanionEventFlagsForItem_MultiplayerPickup_NoForbiddenFlags` | 129 | Wszystkie 5 multiplayer items — brak forbidden, brak `670xxx`, brak cross‑contamination |
| `TestCompanionEventFlagsForItem_MultiplayerPickup_NoCrossContamination` | 177 | Każdy multiplayer item ma tylko własną `Obtained` flagę, nie cudze |

(11 funkcji testowych — tabela powyżej wymienia wszystkie z linią startu.)

### 13.2 Integration tests (`tests/item_companion_flags_test.go`) — 17 testów

| Test | Linia | Co weryfikuje |
|---|---|---|
| `TestCompanionFlagsSetOnRealSave` | 17 | Whistle companion flags settable + readable na realnym slocie |
| `TestCompanionFlagsForbiddenAbsent` | 52 | Forbidden lista absent w companion sets |
| `TestCompanionFlagsMechanicFlagPresent` | 71 | Mechanic flag `60100` faktycznie SET dla Whistle |
| `TestCompanionFlagsNoRoundtableFlags` | 84 | RTH flags `10009655`/`11109658`/`11109659` absent |
| `TestCompanionFlagsClearOnFlagData` | 107 | CLEAR ścieżka działa na bitfieldzie |
| `TestCompanionFlagsNotClearedForUnknownItem` | 149 | Item bez companion set → no‑op |
| `TestSmallGoldenEffigyFlagSet` | 193 | `60230` SET po `AddItemsToCharacter(SmallGoldenEffigy)` |
| `TestSmallGoldenEffigyFlagClear` | 223 | `60230` CLEAR po `RemoveItemsFromCharacter` ostatniej instancji |
| `TestSmallGoldenEffigyRepair` | 274 | Re‑add SmallGoldenEffigy na slocie z brakującą flagą `60230` → naprawia |
| `TestSmallGoldenEffigyNoSummoningPoolFlags` | 309 | `670xxx` nie są SET'owane przez Effigy hook |
| `TestSmallGoldenEffigyUnrelatedItemDoesNotAffectFlag` | 320 | Add innego itemu nie zmienia `60230` |
| `TestCompanionFlagsRemainingItemPreventsClearing` | 354 | Nie‑ostatnia instancja → no‑op CLEAR |
| `TestMultiplayerPickupFlagSet` | 407 | `Obtained` flagi SET dla wszystkich 5 multiplayer items |
| `TestMultiplayerPickupFlagClear` | 441 | jw. CLEAR |
| `TestMultiplayerPickupFlagRepair` | 506 | Repair mode dla multiplayer items |
| `TestMultiplayerPickupNoSummoningPoolFlags` | 542 | jw. `670xxx` |
| `TestMultiplayerPickupUnrelatedItemDoesNotAffectFlags` | 554 | jw. unrelated item |

`needs verification`: brak izolowanego testu **Whistle full roundtrip** (PS4 platform, 4 flagi SET + readback). PS4 runtime test 2026‑05‑11 był ad‑hoc, nie zautomatyzowany.

## 14. Known limits / needs verification

Skondensowana lista otwartych pytań:

1. **Liczba 6 wpisów stale after patch** — brak auto‑regeneracji.
2. **Talisman Pouch sync** (`60500`/`60510`/`60520`) — historyczna hipoteza, **nie** w aktualnym kodzie; nieznane, czy gra wymaga.
3. **Cipher Rings shop uniqueness** — czy gra waliduje tylko przez flagę, czy także przez inwentarz.
4. **Whistle 4‑flag combo runtime test** — manualny PS4 2026‑05‑11, brak CI test.
5. **Cross‑platform flagi 710xxx** — BST resolver pokrycie pasma niezweryfikowane testem.
6. **`EventFlagsOffset <= 0` silent skip** — defensywny, ale tworzy ciche zerwanie sync.
7. **Stale flag IDs po patchu gry** — brak detekcji.
8. **Wails serialization** — założenie single‑threaded execution per slot niezweryfikowane.
9. **MapFragmentItems nie korzystają z companion hook** — confirmed, ale `needs verification` czy *powinny* (czy gra robi coś więcej z fragment items).
10. **Cross‑module flag policy decisions** — w przyszłości może wymagać policy „która polityka wygrywa” dla nakładających się companion sets.

## 15. Cross‑references

- [11-regions.md](11-regions.md) — L0 Map Reveal; nieruszony przez item companion flags.
- [14-game-state.md](14-game-state.md) — `LastRestedGrace` / `PreEventFlagsScalars`; nieruszony.
- [15-event-flags.md](15-event-flags.md) — master event flag API; ten rozdział jest callerem.
- [16-world-state.md](16-world-state.md) — `WorldGeomMan` / `WorldArea`; nieruszony.
- [27-map-reveal.md](27-map-reveal.md) — `MapFragmentItems` jako osobny mechanizm (§9).
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — L2 DLC Cover Layer; niezależne.
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — `grace_companion_flags`; współdzielone stałe, różna polityka SET‑only vs SET+CLEAR.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — `ColosseumGlobalFlags` SET niezależnie od item companion flags (kontekst: `60100` może być SET przez PvP prep, niezależnie od Whistle).

## 16. Sources

- `backend/db/data/item_companion_flags.go` — `itemCompanionEventFlags`, `CompanionEventFlagsForItem`, 6 stałych item IDs, 9 stałych companion flag IDs.
- `backend/db/data/item_companion_flags_test.go` — 11 funkcji testowych unit.
- `tests/item_companion_flags_test.go` — 17 funkcji testowych integration.
- `app.go::AddItemsToCharacter` linie 569‑578 — hook SET.
- `app.go::RemoveItemsFromCharacter` linie 658‑666 + 706‑725 — hook CLEAR.
- `backend/db/data/grace_companion_flags.go` — referencja policy SET‑only dla gracji.
- `backend/db/data/maps.go::MapFragmentItems` — referencja osobnego mechanizmu fragment items.
- `docs/CHANGELOG.md` — historyczne PS4 runtime test 2026‑05‑11.
