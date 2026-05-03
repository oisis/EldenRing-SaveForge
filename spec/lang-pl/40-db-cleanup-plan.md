# 40 — Czyszczenie bazy danych: rejestr wyciętej zawartości i deduplikacja multiplayer

> **Typ**: Dokument projektowy
> **Wyciągnięto z**: ROADMAP.md (czyszczenie 2026-05-03)
> **Status**: 🔲 Planowane

---

## Cel

Kompleksowe czyszczenie bazy danych przedmiotów w aplikacji na podstawie dowodów zgłoszonych przez użytkownika z gry (2026-04-28). Wiele przedmiotów pojawia się w grze z prefiksem `[ERROR]` (brakujące wpisy FMG), w złej sekcji ekwipunku (ryzyko bana) lub jako wizualne duplikaty.

**Źródło prawdy:** zrzuty ekranu z gry od użytkownika (`tmp/screeny-apki/`).

---

## Faza A — Czyszczenie wariantów pustych flakonów (Empty Flask)

**Plik:** `backend/db/data/tools.go`

1. Zweryfikuj w `tmp/erdb/1.10.0/EquipParamGoods.csv` czy ID z `(Empty)` mają odrębne parametry gry czy są zduplikowanymi placeholderami ID z `(Filled)`
2. Jeśli potwierdzona redundancja: usuń ~27 wpisów `(Empty)`:
   - Crimson Tears Flask Empty: 0x400003E8 + 12 wariantów ulepszeń
   - Cerulean Tears Flask Empty: base + 12 wariantów ulepszeń
   - Wondrous Physick Flask Empty: 0x400000FA
3. Sprawdź krzyżowo kolumnę `goodsType` — routing writera może zależeć od typu

---

## Faza B — Deduplikacja aktywny/nieaktywny multiplayer

**Pliki:** `backend/core/writer.go`, `backend/db/db.go`

**Hipoteza:** save przechowuje osobne handle dla stanu "inactive" (trzymany) vs "active" (wdrożony/użyty) przedmiotów multiplayer. Gra automatycznie przepisuje handle przy aktywacji. Nasz writer dodaje nieaktywny handle nawet gdy istnieje wariant aktywny → użytkownik widzi 2 w grze.

1. Forensic na `tmp/crash/ER0000.sl2` — przeszukaj CommonItems pod kątem znanych par multiplayer aby zidentyfikować ID stanów aktywnych
2. Zbuduj `MultiplayerStatePairs map[uint32]uint32` (active→inactive) w `db/db.go`
3. W `addToInventory`: przed insertem sprawdź czy wariant aktywny LUB nieaktywny już istnieje. Jeśli tak — pomiń
4. Dotknięte przedmioty: Tarnished's Wizened Finger (0x4000006A), Tarnished's Furled Finger (0x400000AA), Small Golden Effigy (0x400000B3), Small Red Effigy (0x400000B4) + prawdopodobnie wszystkie 11 przedmiotów multiplayer w `tools.go:7-19`

---

## Faza C — Rejestr wyciętej zawartości + flagowanie niepewnych przedmiotów

Przedmioty do oflagowania `cut_content, ban_risk` (użytkownik zgłosił `[ERROR]` w grze LUB zła sekcja save'a):

**Notes (info.go) — oflaguj, zostaw w DB:**

| ID | Nazwa |
|---|---|
| 0x4000222E | Note: Hidden Cave |
| 0x4000222F | Note: Imp Shades |
| 0x40002230 | Note: Flask of Wondrous Physick |
| 0x40002231 | Note: Stonedigger Trolls |
| 0x40002232 | Note: Walking Mausoleum |
| 0x40002233 | Note: Unseen Assassins |
| 0x40002235 | Note: Flame Chariots |
| 0x40002236 | Note: Demi-human Mobs |
| 0x40002237 | Note: Land Squirts |
| 0x40002238 | Note: Gravity's Advantage |
| 0x4000223A | Note: Waypoint Ruins |
| 0x4000223D | Note: Frenzied Flame Village |

**Tools (tools.go) — oflaguj:**
- 0x40000BCC Miranda's Prayer (użytkownik zgłosił `[Error]Modlitwa Mirandy` w grze)
- Scorpion Stew DLC — er-save-manager podaje 4 ID (2001200..2001203) ze zduplikowanymi nazwami; brakujące ID do weryfikacji: `0x401E8934`, `0x401E8935`

**Key Items (key_items.go) — oflaguj:**
- 0x4000229E Golden Order Principia (kandydat na `[ERROR]Zasady Złotego Porządku`)

**Helms/Chest — niezidentyfikowany zestaw ze zrzutu ekranu:**
- Oznacz jako cut_content po identyfikacji przez porównanie Fextralife + EquipParamProtector.csv

---

## Faza D — Usunięcie automatycznie otrzymywanych notatek

Gracz otrzymuje te notatki automatycznie na starcie gry (potwierdzone) i nie może ich usunąć → brak wartości w DB:
- 0x40002234 Note: Great Coffins
- 0x40002239 Note: Revenants
- 0x4000223B Note: Gateway

Sprawdź `presets/`, `audit/`, testy pod kątem referencji przed usunięciem.

---

## Faza E — Śledztwo w sprawie duplikatu Memory of Grace

Użytkownik zgłasza, że Memory of Grace (0x40000073) pojawia się 2× w ekwipunku w grze po dodaniu, pomimo pojedynczego wpisu w DB i istniejącej logiki deduplikacji. Hipoteza: legacy duplikat z wersji writera sprzed poprawki utrzymuje się w save'ie użytkownika.

Opcje:
1. Dodać deduplikator przy ładowaniu: skanuj CommonItems pod kątem zduplikowanych handle'i, merguj stacki
2. Dodać walidator przy zapisie: ostrzegaj jeśli wykryto zduplikowane handle
3. Udokumentować jako "naprawa przy następnym czystym save'ie" jeśli zbyt ryzykowne

---

## Faza F — Testy + weryfikacja buildu

1. `go test -v ./backend/db/...`
2. `go test -v ./backend/core/...` (testy jednostkowe deduplikacji multiplayer)
3. `cd frontend && npx tsc --noEmit && npm run lint`
4. `make build`

---

## Faza G — Dokumentacja

1. `CHANGELOG.md` — wpis per faza
2. `ROADMAP.md` — oznacz jako ukończone
3. Nowy dokument `spec/` jeśli rejestr wyciętej zawartości urośnie wystarczająco

---

## Otwarte pytania

- Przedmiot hełm/zbroja ze zrzutu ekranu — potrzebna dokładna nazwa EN (użytkownik dostarczy)
- Czy Faza B (deduplikacja multiplayer) wymaga osobnych ID "active state" w DB czy tylko inspekcji save'a w runtime
- Czy zostawić oflagowane-ale-nie-usunięte notatki w DB (obecna propozycja) czy przenieść do osobnego pliku `cut_content_archive.go`

**Szacunkowy nakład pracy:** 4-6 iteracji (Faza A trywialna, B+C+D+E wymagające śledztwa)
