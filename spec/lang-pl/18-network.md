# 18 — Network Manager (Dane Sieciowe)

> **Zakres**: Dane multiplayer — 131 KB opaque blob.

---

## Opis ogólny

NetMan przechowuje dane sesji multiplayer. Duży blok (131,076 bajtów = 0x20004), w dużej mierze niezbadany.

---

## Struktura (131,076 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | Unknown (unk0x0) — prawdopodobnie flaga stanu |
| 0x04 | u8[0x20000] | Data (131,072 bytes) — opaque network state |

---

## Spawn Point Data (bezpośrednio po RendMan, przed NetMan)

Po Player Coordinates i przed NetMan znajdują się dodatkowe pola GameMan:

| Offset | Typ | Opis |
|---|---|---|
| +0 | u8 | game_man_0x5be |
| +1 | u8 | game_man_0x5bf |
| +2 | u32 | spawn_point_entity_id (Grace entity ID dla respawnu) |
| +6 | u32 | game_man_0xb64 |
| +10 | u32 | temp_spawn_point_entity_id (version >= 65) |
| +14 | u8 | game_man_0xcb3 (version >= 66) |

---

## Implikacje dla edycji

- **spawn_point_entity_id**: zmiana = gracz respawnuje w innym Site of Grace
- Network data: typowo nie edytuje się — dane sesji są efemeryczne
- Cały blob można wyzerować bezpiecznie (multiplayer state resetnięty)

---

## Źródła

- er-save-manager: `parser/world.py` — klasa `NetMan` (linie 785-802)
- er-save-manager: `parser/user_data_x.py` linie 178-186 (GameMan spawn fields)
