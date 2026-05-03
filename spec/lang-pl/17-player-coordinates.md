# 17 — Player Coordinates (Pozycja Gracza)

> **Zakres**: Pozycja gracza w świecie 3D, identyfikator mapy, kąt obrotu.

---

## Opis ogólny

PlayerCoordinates przechowuje dokładną pozycję gracza w momencie zapisu. Rozmiar: 57 bajtów (0x39).

---

## Struktura (57 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | f32 × 3 | Coordinates (x, y, z) — główna pozycja |
| 0x0C | u8[4] | Map ID — identyfikator aktualnej mapy |
| 0x10 | f32 × 4 | Angle — quaternion obrotu (x, y, z, w) |
| 0x20 | u8 | game_man_0xbf0 (unknown) |
| 0x21 | f32 × 3 | Unknown Coordinates (backup? spawn?) |
| 0x2D | f32 × 4 | Unknown Angle |

---

## Map ID Format

Map ID to 4 bajty kodujące lokalizację w hierarchii świata:

| Byte | Opis |
|---|---|
| 0 | Region / Area type |
| 1 | Y coordinate (grid) |
| 2 | X coordinate (grid) |
| 3 | Layer (0x3C=overworld, 0x3D+=underground) |

---

## Znane Map ID wartości

| Byte[3] (Layer) | Opis |
|---|---|
| 0x3C | Overworld (powierzchnia) |
| 0x3D | Underground level 1 (Siofra, Ainsel, Deeproot) |
| 0x3E | Underground level 2 (Lake of Rot, Mohgwyn) |

| Byte[0] (Region) | Opis (przybliżone) |
|---|---|
| 0x3C | Limgrave, Stormveil |
| 0x3D | Liurnia |
| 0x3E | Altus, Leyndell |
| 0x3F | Caelid |
| 0x40 | Mountaintops |

**Uwaga**: Dokładne mapowanie wymaga weryfikacji — powyższe są orientacyjne z obserwacji save files.

---

## Teleportacja przez edycję — zasady bezpieczeństwa

1. **Map ID musi istnieć** — nieprawidłowe ID = nieskończone ładowanie lub crash
2. **Coordinates muszą być w granicach mapy** — pozycja out-of-bounds = falling death → respawn at last grace
3. **Y (vertical)** jest krytyczny — za nisko = pod mapą, za wysoko = falling
4. **Last Rested Grace** (spec/14) jest bezpieczniejszą alternatywą teleportacji
5. **Unknown coordinates** (offset 0x21) prawdopodobnie to "stable ground position" — backup przy unstuck

---

## Implikacje dla edycji

- Zmiana Coordinates = bezpośrednia teleportacja gracza
- Map ID musi odpowiadać prawidłowej mapie — błędny ID = crash/infinite loading
- **Bezpieczna teleportacja**: lepiej zmienić LastRestedGrace (Game State) niż surowe coords
- Second set of coordinates: prawdopodobnie spawn point / last stable position
- Quaternion: (0, 0, 0, 1) = brak obrotu (patrzenie na północ)
- game_man_0xbf0: prawdopodobnie flaga "on ground" / "in air"

---

## Źródła

- er-save-manager: `parser/world.py` — klasa `PlayerCoordinates` (linie 747-776)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — `PlayerCoords` struct (linie 73-119)
- er-save-manager: `parser/user_data_x.py` linia 175: `player_coordinates: PlayerCoordinates`
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — coordinates via WorldChrMan (runtime, f32 x/y/z)
- Cheat Engine: `ER_TGA_v1.9.0` — FieldArea MapID at +0x2C
