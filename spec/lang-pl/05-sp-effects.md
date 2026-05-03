# 05 — SP Effects (Efekty Statusu)

> **Zakres**: Aktywne efekty na postaci — buffy, debuffy, statusy specjalne.

---

## Opis ogólny

Sekcja SP Effects (SpEffect) następuje bezpośrednio po PlayerGameData. Zawiera listę aktywnych efektów statusu na postaci w momencie zapisu.

SpEffect w Elden Ring to uniwersalny mechanizm — obejmuje wszystko od trutek przez Great Rune bonusy po efekty bossowych ataków.

---

## Struktura

### Format wpisu SPEffect

| Offset | Typ | Opis |
|---|---|---|
| 0x00 | u32 | SpEffect ID (z tabeli SpEffectParam) |
| 0x04 | f32 | Remaining duration (sekundy, -1.0 = permanentny) |
| 0x08 | u32 | Unknown field 1 |
| 0x0C | u32 | Unknown field 2 |

### Liczba wpisów

Dokładna liczba wpisów jest podana jako prefix lub stała (wymaga weryfikacji):
- er-save-manager parsuje SPEffect jako strukturę z `param_id` + `remaining_time` + unknown fields
- ER-Save-Editor (Rust) nie parsuje tej sekcji szczegółowo

---

## Przykłady SpEffect IDs

SpEffect IDs odnoszą się do tabeli `SpEffectParam` w regulation.bin. Kilka znanych kategorii:

- **Great Runes**: aktywowane efekty wielkich run
- **Buffs**: Wonderous Physick mieszanki, consumable
- **Debuffs**: poison, rot, frostbite ticking damage
- **Passive**: equipment bonuses (niektóre talizmany)

---

## Implikacje dla edycji

- Usunięcie wszystkich SpEffects jest bezpieczne — efekty zostaną ponownie nałożone przy loginie
- Modyfikacja duration pozwala na permanentne buffy (ustawienie -1.0f)
- SpEffect IDs muszą istnieć w SpEffectParam — nieistniejące ID mogą crashować grę

---

## Źródła

- er-save-manager: `parser/character.py` — klasa `SPEffect`
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam
- TGA CE Table: https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA — param scripts
