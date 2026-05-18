# 42 — Summoning Pools: UI działa, brak efektu w grze

> **Typ**: Śledztwo / Bug
> **Wyciągnięto z**: docs/ROADMAP.md (czyszczenie 2026-05-03)
> **Status**: 🐛 Wstrzymane (od 2026-04-25)

---

## Objaw

UI przełącza summoning pools poprawnie (brak błędów), ale przełączone poole NIE są aktywne w grze (testowane offline aby uniknąć banów). Dotyczy wszystkich pooli, nie konkretnych.

## Lista kontrolna diagnostyki (wszystkie zaliczone ✅)

- [x] Baza danych pokrywa wszystkie ID pooli (165 pooli, więcej niż referencja ClayAmore/ER-Save-Editor z 162)
- [x] Tablica lookup `event_flags.go` zawiera ID pooli z offsetami bajt/bit identycznymi bit-po-bicie z ER-Save-Editor
- [x] Resolver BST produkuje identyczne offsety (zweryfikowano `1037530040`, `1051570840`, `1060440040`)
- [x] `SetEventFlag` odwraca poprawny bit w slice `slot.Data[EventFlagsOffset:]` (backing array — modyfikacje się propagują)
- [x] `SaveSlot.Write()` NIE nadpisuje regionu event flag (zapisuje tylko level/stats/name/runes)
- [x] `SaveFile()` serializuje `slot.Data` bezpośrednio bez przebudowy ze sparsowanych struktur

## Pozostałe hipotezy

1. **Brak testu persystencji** — napisać test integracyjny: `LoadSave → Set → SaveFile → LoadSave → Get` aby zweryfikować czy bit przeżywa round-trip. Jeśli nie przeżywa, szukać w `core/writer.go` lub pipeline szyfrowania.

2. **Gra wymaga stanu wtórnego** — bit może być ustawiony w event_flags, ale gra może też sprawdzać:
   - `unlocked_regions` dla obszaru mapy poola (zależność od feature Invasion Regions)
   - Sekcję trophy data (`trophy_data` 52 bajty)
   - Cross-referencje `world_area` / `gaitem_game`

3. **Region hash (`CSPlayerGameDataHash`, ostatnie 0x80 bajtów slota)** — aktualnie zachowywany dosłownie. Gra może go walidować względem stanu runtime gdy zainstalowane DLC.

4. **Specyficzne dla PS4** — save'y PS4 są nieszyfrowane, ale szyfrowanie PC powiązane z SteamID może wchodzić w interakcję z naszym zapisem flag.

## Plan działania (po wznowieniu)

1. Napisać `tests/event_flag_persistence_test.go` pokrywający round-trip Set → Save → Load → Get
2. Jeśli round-trip utrzymuje się → zbadać wymagania po stronie gry (porównać z referencyjnym save'em gdzie poole są aktywowane)
3. Jeśli round-trip zawodzi → prześledzić gdzie bit gubi się w pipeline writer/encryption
4. Cross-check z Invasion Regions — może aktywacja poola wymaga odblokowania matchującego regionu

## Powiązane

- Toggle Colosseum ma ten sam objaw (flagi ustawione, brak efektu w grze)
- Toggle Sites of Grace działa częściowo (mapa widoczna ale fast-travel nieaktywowane)
- Wszystkie mogą mieć wspólną przyczynę (wtórna walidacja po stronie gry wykraczająca poza event flags)
