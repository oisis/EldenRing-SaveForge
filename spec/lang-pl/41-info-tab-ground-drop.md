# 41 — Wypadanie przedmiotów z zakładki Info na ziemię

> **Typ**: Śledztwo (wstrzymane)
> **Wyciągnięto z**: ROADMAP.md (czyszczenie 2026-05-03)
> **Status**: 🐛 Wstrzymane — wymagana dekompilacja EMEVD

---

## Objaw

Dodawanie przedmiotów z limitem 1/0 z zakładki Info (Notes, About tutorials, Letters, Maps, Cookbooks) przez edytor powoduje, że kopia światowa spada na ziemię gdy gracz przechodzi obok lokalizacji triggera w grze. Przykład: zakup Crafting Kit u Kalé spawnuje "Tworzenie przedmiotów" na ziemi, bo gracz już go posiada.

**Ryzyko bana:** Brak. Standardowe NG+ produkuje to samo zachowanie. Tylko kosmetyczny bałagan.

---

## Co próbowaliśmy (2026-04-29)

### Podejście 1: Mapa WorldPickupFlagID

- Wyekstrahowano `getItemFlagId` z `ItemLotParam_map` (cat=1) i `eventFlag_forStock` z `ShopLineupParam` (equipType=3) w regulation.bin
- 308 wpisów w `backend/db/data/world_pickup_flags.go`
- Podpięto do `AddItemsToCharacter` aby ustawić flagę dla kopii światowej
- **Rezultat:** flaga ustawiona poprawnie w save'ie (potwierdzone diffem save'a: flaga 550130 dla About Item Crafting zapisana), przedmiot dalej spada w grze
- **Wniosek:** `getItemFlagId` / `eventFlag_forStock` nie są flagami gatującymi spawn EMEVD

### Podejście 2: TutorialDataChunk (AboutTutorialID)

- Odkryto blok `TutorialDataChunk` (0x408 bajtów) pod `slot.TutorialDataOffset`
- Layout: `unk0x0 u16 | unk0x2 u16 | size u32 | count u32 | u32 IDs[count]`
- Zakup Crafting Kit dopisał ID `2010` do listy (zweryfikowane diffem save'a)
- Pre-populacja przez `core.AppendTutorialID` (edycja czystego save'a potwierdzona: count 8 → 9, chirurgiczna zmiana 13 bajtów)
- **Rezultat:** lista poprawnie zmodyfikowana w save'ie, przedmiot dalej spada w grze
- **Wniosek:** Tutorial ID 2010 kontroluje pojawienie się tekstu popup, nie akcję give EMEVD

---

## Wniosek końcowy

Akcja give/spawn dla przedmiotów tutorial z zakładki Info jest gatowana przez sprawdzenie, którego jeszcze nie zidentyfikowaliśmy. Prawdopodobni kandydaci:

- Hardkodowana instrukcja EMEVD (`event/m??_??_??_??.emevd.dcx` wewnątrz `Data0.bdt`) omijająca zarówno `getItemFlagId` jak i `TutorialDataChunk`
- Osobny bitset stanu regionu gdzieś w slot.Data, którego nie zlokalizowaliśmy
- Flaga w zakresie "tutorialFlagId" emitowanym przez EMEVD (710xxx, 720xxx), którego nie sprawdziliśmy wyczerpująco

---

## Następne kroki śledztwa (po wznowieniu)

1. Wyekstrahuj `Data0.bdt` ze Steam Deck (`~/.local/share/Steam/steamapps/common/ELDEN RING/Game/`)
2. Odszyfruj BHD publicznym kluczem RSA
3. Dekompiluj `event/common.emevd.dcx` (i area-specific `event/m11_*.emevd.dcx` dla Stranded Graveyard / Limgrave) narzędziami community
4. Przeszukaj EMEVD pod kątem wzorców `give_item(9113, 1)` / `give_item(9135, 1)` i zidentyfikuj gatującą flagę
5. Alternatywa: empiryczna macierz diffów save'a — dla każdego przedmiotu About: zrób save BEFORE → wyzwól DOKŁADNIE jeden przedmiot w grze → diff i znajdź unikalną zmianę bajtową

---

## Pliki zachowane do wznowienia

- `backend/core/tutorial_data.go` — parser/writer dla TutorialDataChunk (działa zgodnie z projektem)
- `backend/db/data/tutorial_ids.go` — mapa AboutTutorialID (populować w miarę przyszłych odkryć)
- `backend/db/data/world_pickup_flags.go` — 308 wpisów (przydatne dla przedmiotów gdzie flaga RZECZYWIŚCIE gatuje spawn)
- `app.go` hooki `AddItemsToCharacter` dla obu map — nieszkodliwy no-op gdy mechanizm gatowania nie jest wyzwolony
