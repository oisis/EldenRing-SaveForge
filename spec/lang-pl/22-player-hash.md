# 22 — Player Data Hash

> **Zakres**: Hash końcowy danych gracza — ostatnia sekcja w slocie.

---

## Opis ogólny

PlayerGameDataHash to ostatnia sekcja w slocie. Zajmuje resztę bajtów do końca slotu (0x280000). Zawiera hash lub checksum danych gracza.

---

## Struktura

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u8[...] | Hash / checksum data (do końca slotu) |

Dokładna długość: `slot_end - current_position` po sparsowaniu wszystkich poprzednich sekcji.

---

## Znane właściwości

- Gra **NIE waliduje** tego hasha przy ładowaniu (potwierdzone przez ER-Save-Editor)
- Jest read-only z perspektywy edytora — nie trzeba go przeliczać
- Zawartość prawdopodobnie to wewnętrzny hash FromSoftware do detekcji tampering (ale nie egzekwowany)

---

## Implikacje dla edycji

- **Nie wymaga przeliczania** — gra go ignoruje
- Bezpieczne do pozostawienia bez zmian po edycji slotu
- Przy tworzeniu slotu od zera: wypełnij zerami

---

## Źródła

- er-save-manager: `parser/user_data_x.py` linia 195: `player_data_hash: PlayerGameDataHash`
- er-save-manager: `parser/world.py` — klasa `PlayerGameDataHash`
