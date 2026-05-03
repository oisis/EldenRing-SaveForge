# 19 — Weather & Time (Pogoda i Czas)

> **Zakres**: Stan pogody i czas w świecie gry.

---

## Opis ogólny

Dwie małe struktury po 12 bajtów każda — stan pogody i czas w grze.

---

## WorldAreaWeather (12 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u16 | Area ID (identyfikator regionu pogodowego) |
| 0x02 | u16 | Weather Type (typ pogody) |
| 0x04 | u32 | Timer (czas trwania aktualnej pogody) |
| 0x08 | u32 | Padding |

### Detekcja korupcji
`Area ID == 0` → pogoda jest prawdopodobnie uszkodzona. Może powodować glitche wizualne.

---

## WorldAreaTime (12 bytes)

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | Hour (godzina: 0-23) |
| 0x04 | u32 | Minute (minuta: 0-59) |
| 0x08 | u32 | Second (sekunda: 0-59) |

### Detekcja korupcji
`Hour == 0 AND Minute == 0 AND Second == 0` → czas potencjalnie uszkodzony (choć 00:00:00 jest technicznie prawidłowe).

---

## Implikacje dla edycji

- **Time**: zmiana godziny = zmiana pory dnia (wpływa na spawny NPC i eventów)
- **Weather**: zmiana typu pogody — wartości mapują się na wewnętrzne ID pogody w grze
- Obie struktury bezpieczne do modyfikacji — gra resetuje je przy zmianach w grze
- Korupcja pogody: wyzeruj Weather Type i ustaw prawidłowy Area ID

---

## Źródła

- er-save-manager: `parser/world.py` — `WorldAreaWeather` (linie 810-838), `WorldAreaTime` (846-882)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — `WorldAreaWeather`, `WorldAreaTime` (linie 6-70)
