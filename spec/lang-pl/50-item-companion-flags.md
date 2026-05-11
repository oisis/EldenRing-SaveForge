# 50 — Item Companion Flags (Towarzyszące Flagi Przedmiotów)

> **Typ**: Design doc
> **Status**: ✅ Zaimplementowano (v0.14.0) — SET przy dodaniu + CLEAR przy usunięciu
> **Zakres**: Mechanizm synchronizacji zależnych od przedmiotu EventFlag przy dodawaniu przedmiotów do slotu postaci i ich usuwaniu.

---

## Problem

Dodanie niektórych przedmiotów przez edytor powoduje, że przedmiot fizycznie trafia do ekwipunku, ale gra traktuje postać tak, jakby nigdy nie otrzymała go normalną ścieżką questową. Skutki:

- Ponowne uruchamianie cutscenek i dialogów NPC przy odpoczynku w Miejscu Łaski.
- Mechaniki gry zablokowane za EventFlagami pozostają niedostępne (np. Torrent nie może być przyzwany bez flagi 60100, nawet jeśli Spectral Steed Whistle jest w ekwipunku).
- Niespójność stanu questa — EMEVD gry czyta EventFlaги, nie inwentarz, dla bramek mechanicznych.

## Przyczyna źródłowa

Gra ustawia EventFlaги podczas normalnego przepływu pozyskania przedmiotu (wykonanie skryptu EMEVD). `AddItemsToCharacter()` edytora wcześniej ustawiało jedynie flagi jednoelementowe (`AoWItemToFlagID`, `WorldPickupFlagID`, `BolsteringPickupFlags`) i ID tutoriali. Przedmioty pozyskiwane przez dialog questowy (nie przez world pickup ani sklep) nie miały żadnego pokrycia flagowego.

---

## Design

### Companion flag set (zestaw flag towarzyszących)

**Companion flag set** to minimalna grupa EventFlag, które gra ustawia wspólnie podczas normalnego pozyskania przedmiotu, przy czym:

1. Bramka mechaniczna gry dla przedmiotu jest odblokowana (przedmiot jest używalny).
2. EMEVD gry nie uruchamia ponownie dialogu/cutscenki pozyskania.
3. Nie zawiera flag przejściowych (czyszczonych przez silnik po użyciu).
4. Nie zawiera flag obszarowych ani flagów out-of-bounds na PS4.

### Struktura danych

```go
// backend/db/data/item_companion_flags.go
var itemCompanionEventFlags = map[uint32][]uint32{
    0x40000082: {60100, 4680, 710520, 4681},
}

func CompanionEventFlagsForItem(itemID uint32) []uint32
```

### Miejsca podpięcia (hooks)

**SET** — blok POST-FLAGS w `AddItemsToCharacter()` (`app.go`), po `AboutTutorialID`. Uruchamia się dla każdego przedmiotu w slice `prepared` — w tym dla przedmiotów już na maksymalnej ilości w ekwipunku — co pozwala **naprawiać saves**, w których przedmiot był wcześniej dodany bez flag towarzyszących.

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

**CLEAR** — blok post-removal w `RemoveItemsFromCharacter()` (`app.go`). Uruchamia się tylko wtedy, gdy ostatnia instancja przedmiotu została usunięta ze slotu (sprawdzane przez skan `slot.GaItems`).

### Poza zakresem

- `CompanionEventFlagsForGrace` — nie istnieje żaden hook na poziomie grace.
- `SetGraceVisited` — nie jest modyfikowane przez ten mechanizm.
- Flagi zaproszenia do Roundtable Hold (`10009655`, `11109658`, `11109659`) — nie są częścią item companion flags.
- Flagi postępu Site of Grace — nie są częścią tego mechanizmu.
- Flagi `4656`, `11109786` oraz context flags (`710770`, `69090`, `69370`) — nigdy nie są ustawiane ani czyszczone przez ten mechanizm.

---

## Spectral Steed Whistle — `0x40000082`

### Flagi towarzyszące

| Flaga | Nazwa | Klasyfikacja | Źródło |
|---|---|---|---|
| **60100** | Obtained Spectral Steed Whistle | CONFIRMED_MINIMAL — odblokowuje mechanikę Torrenta | spec/12, spec/15, er-save-manager `event_flags_db.py`, 5× slot PC |
| **4680** | Melina gave Spectral Steed Whistle | CONFIRMED — stan questa „przekazano", zapobiega ponownemu dialogowi | quests.go krok 6, 5× slot PC |
| **710520** | Whistle world/map state | CONFIRMED — ustawiana razem z 60100 przez grę | quests.go krok 6, 5× slot PC |
| **4681** | Melina accept/refuse popup shown | CONFIRMED — warunek wstępny wyskakującego okna | quests.go krok 5, 5× slot PC |

### Weryfikacja

Potwierdzone w 5 aktywnych slotach `ER0000.sl2` (PC, postacie po Melinie, 2026-05-11). Wszystkie 4 flagi SET w każdym slocie. Potwierdzone przez vanilla save PS4 (wszystkie 0) i er-save-manager `event_flags_db.py`.

### Runtime Validation — Spectral Steed Whistle ✅ Runtime confirmed

**Data**: 2026-05-11  
**Platforma**: PS4  
**Wynik**: PASS

Potwierdzone: dodanie Spectral Steed Whistle wraz z companion flags 60100, 4680, 710520, 4681 zapobiega ponownemu odpaleniu sceny Meliny z przekazaniem gwizdka. Item działa w grze. Dodatkowe flagi cleanup Meliny/Gatefront nie były wymagane.

Flagi 710770, 69090, 69370 (Melina opuszcza Gatefront) **nie były ustawiane** i **nie były wymagane** — potwierdzone runtime. Pozostają wyłącznie kandydatami badawczymi.

### Flagi NIE uwzględnione (i dlaczego)

| Flaga(i) | Powód wykluczenia |
|---|---|
| 710770, 69090, 69370 | Melina opuszcza Gatefront — kandydaci badawczy; **potwierdzono runtime że nie są wymagane** (test PS4 2026-05-11). Obecne we wszystkich post-Melina savach PC, ale niepotrzebne dla poprawności mechaniki itemu. |
| 4698 | Wyzwalacz cutscenki Meliny — przejściowy, czyszczony przez silnik po odtworzeniu cutscenki. 0 we wszystkich prawdziwych savach. |
| 4651, 4652, 4653 | Stany dialogu Meliny — przejściowe, czyszczone po dialogu. 0 we wszystkich prawdziwych savach. |
| 4656 | Level Up wykonany — osobna akcja użytkownika, niezwiązana z pozyskaniem przedmiotu. |
| Zakres 1042xxx | Out of bounds na PS4 (offset BST ~130 MB vs ~2,3 MB tablica flag). Fizycznie nie można ustawić. |

### Kontekst: ColosseumGlobalFlags

Flaga 60100 jest też ustawiana przez `ApplyPvPPreparation()` przez `data.ColosseumGlobalFlags` gdy `opts.Colosseums = true`. To tłumaczy, dlaczego niektórzy użytkownicy zgłaszali działającego Torrenta po dodaniu gwizdka przez edytor — mieli wcześniej zastosowane PvP Preparation z Colosseums. Użytkownicy bez tego kroku mieli 60100=0 i Torrent był bezużyteczny.

---

## Small Golden Effigy — `0x4000006D`

**Kategoria**: Narzędzia → Multiplayer  
**Nazwa EN**: Small Golden Effigy

### Problem

Dodanie Small Golden Effigy przez edytor umieszcza item w inventory, ale stan odbioru/interakcji przy Statuetce Przyzywania (Effigy of the Martyr) może pozostawać widoczny — tak jakby item nigdy nie został odebrany normalną ścieżką.

### Flagi towarzyszące

| Flaga | Nazwa | Klasyfikacja |
|---|---|---|
| **60230** | Obtained Small Golden Effigy | SET przy dodaniu, CLEAR przy usunięciu |

### Zachowanie

- **Ścieżka SET** (`AddItemsToCharacter`): działa dla każdego itemu w `prepared`, w tym itemów już na maksymalnej ilości w ekwipunku — umożliwia **naprawę saves**, w których item był dodany bez flagi.
- **Ścieżka CLEAR** (`RemoveItemsFromCharacter`): działa tylko wtedy, gdy ostatnia instancja itemu została usunięta ze slotu (sprawdzane przez skan `slot.GaItems`).

### Flagi NIE uwzględnione (i dlaczego)

| Flaga(i) | Powód wykluczenia |
|---|---|
| 60220, 60240, 60250, 60260, 60270, 60300, 60310 | Inne przedmioty multiplayer — osobne zestawy flag, nie są częścią tego mappingu. |
| 670xxx (aktywacja Summoning Pool) | Osobny mechanizm. Aktywacja statki przyzywania to osobna akcja gracza, niezwiązana z pozyskaniem itemu. |
| Wszystkie flagi Spectral Steed Whistle | Niezwiązany łańcuch itemów — brak nakładania się. |

---

## Dodawanie kolejnych zestawów flag towarzyszących

Aby dodać flagi towarzyszące dla kolejnego przedmiotu:

1. Zbadaj które flagi gra ustawia podczas normalnego pozyskania (sprawdź `quests.go`, porównaj pary save przed/po, cross-reference er-save-manager `event_flags_db.py`).
2. Zweryfikuj każdą flagę w prawdziwych savach po pozyskaniu (PC i PS4, gdzie możliwe).
3. Wyklucz flagi przejściowe (wartości 0 w w pełni ustabilizowanych savach po pozyskaniu).
4. Wyklucz zakres 1042xxx (out of bounds na PS4).
5. Dodaj ID przedmiotu i listę flag do `itemCompanionEventFlags` w `backend/db/data/item_companion_flags.go`.
6. Dodaj testy jednostkowe do `backend/db/data/item_companion_flags_test.go`.
7. Dodaj przypadki testowe integracyjne do `tests/item_companion_flags_test.go`.

---

## Źródła

- `backend/db/data/item_companion_flags.go` — implementacja
- `backend/db/data/item_companion_flags_test.go` — testy jednostkowe
- `tests/item_companion_flags_test.go` — testy integracyjne
- `spec/12-torrent.md` — flagi mechaniki Torrenta
- `spec/15-event-flags.md` — rejestr EventFlag
- `backend/db/data/quests.go` — kroki questa łańcucha Meliny z flagami
- `tmp/repos/er-save-manager/src/er_save_manager/data/event_flags_db.py` — społecznościowa baza flag
- `tmp/regulation-bin-debug/spectral-steed-whistle-research.md` — pełny raport badawczy (2026-05-11)
