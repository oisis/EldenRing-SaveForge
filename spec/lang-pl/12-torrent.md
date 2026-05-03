# 12 — Torrent (Koń)

> **Zakres**: Dane Torrenta — pozycja, HP, stan.

---

## Opis ogólny

RideGameData przechowuje stan konia (Torrent): jego pozycję w świecie, punkty życia i stan aktywności. Rozmiar: 40 bajtów (0x28).

---

## Struktura (40 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | f32 × 3 | Coordinates (x, y, z) — pozycja Torrenta |
| 0x0C | u8[4] | Map ID (identyfikator mapy) |
| 0x10 | f32 × 4 | Angle / Quaternion (orientacja) |
| 0x20 | i32 | HP (punkty życia) |
| 0x24 | u32 | State (stan) |

---

## Horse State

| Wartość | Stan | Opis |
|---|---|---|
| 1 | INACTIVE | Torrent nie jest przywołany |
| 3 | DEAD | Torrent martwy (wymaga Crimson Flask) |
| 13 | ACTIVE | Torrent przywołany, gracz jeździ |

---

## Znany bug: Infinite Loading Screen

**Warunek buga**: `HP == 0` AND `State == ACTIVE (13)`

Powinno być: `HP == 0` AND `State == DEAD (3)`

Ten bug powoduje infinite loading screen przy wejściu do gry. Fix: zmień State na 3 (DEAD) gdy HP == 0.

---

## Torrent Unlock

Torrent jest odblokowany przez Event Flag **60100** (Spectral Steed Ring, otrzymany po spotkaniu Meliny w Site of Grace).

Bez tej flagi gracz nie może przywołać Torrenta nawet jeśli posiada Spectral Steed Ring w inventory.

---

## Torrent HP Scaling

HP Torrenta skaluje z poziomem gracza. Nie jest to stała wartość — gra oblicza max HP na podstawie player level. Dokładna formuła nieznana, ale:
- ~lvl 1: ~400 HP
- ~lvl 50: ~800 HP
- ~lvl 100+: ~1200+ HP

Revived Spirit Ash Blessing (DLC) zwiększa HP i damage negation Torrenta w Realm of Shadow.

---

## Implikacje dla edycji

- Bezpieczne do modyfikacji
- **HP**: Wartość z zakresu 0 do max HP Torrenta. Wartość powyżej max = clamped przez grę.
- **State**: Używaj TYLKO znanych wartości (1, 3, 13). Inne = undefined behavior.
- **Coordinates**: Zmiana teleportuje Torrenta (ale gra resetuje pozycję przy przywołaniu).
- **Fix buga**: ZAWSZE koryguj State=3 (DEAD) gdy HP=0. To zapobiega infinite loading.
- **Unlock check**: Weryfikuj event flag 60100 przed ustawieniem State=ACTIVE.

---

## Źródła

- er-save-manager: `parser/world.py` — klasa `RideGameData` (linie 126-173)
- er-save-manager: `parser/er_types.py` — enum `HorseState`
- er-save-manager: `parser/user_data_x.py` linia 130: `horse: RideGameData`
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Torrent coordinates (WorldChrMan chain)
- Elden Ring Wiki: https://eldenring.wiki.fextralife.com/Torrent
