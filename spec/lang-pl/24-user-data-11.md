# 24 — UserData11 (regulation.bin)

> **Zakres**: Regulation data — parametry gry (params), ostatnia sekcja pliku.

---

## Opis ogólny

UserData11 zawiera `regulation.bin` — plik z parametrami gry. To nie są dane gracza, a definicje game design: statystyki broni, efektów, NPC, balans.

Rozmiar: ~0x240010 bytes (~2.36 MB).

Na PC: poprzedzone 16-bajtowym MD5 checksumem.

---

## Zawartość (regulation.bin)

Regulation.bin zawiera **200+ tabel parametrów** (params):

| Param Table | Opis |
|---|---|
| EquipParamWeapon | Statystyki broni |
| EquipParamProtector | Statystyki zbroi |
| EquipParamAccessory | Statystyki taliszmanów |
| EquipParamGoods | Statystyki przedmiotów |
| SpEffectParam | Definicje efektów statusu |
| AtkParam_Pc | Parametry ataku gracza |
| AtkParam_Npc | Parametry ataku NPC |
| NpcParam | Parametry NPC |
| BulletParam | Parametry pocisków |
| Magic | Parametry zaklęć |
| ... | (200+ dodatkowych) |

---

## Format wewnętrzny

Regulation.bin to spakowany kontener BND4 z wieloma plikami `.param`. Każdy `.param` to tablica rekordów o stałym formacie specyficznym dla danego typu.

Parser param:
1. Rozpakuj BND4
2. Dla każdego .param: odczytaj header (row size, row count)
3. Czytaj rekordy sekwencyjnie

---

## Implikacje dla edycji

- **Regulation.bin jest identyczny** dla wszystkich graczy z tą samą wersją gry
- Modyfikacja = "modding" parametrów gry (nie save editing)
- W kontekście save editora: **readonly** — kopiuj as-is między platformami
- Przy konwersji platform: regulation.bin powinno być identyczne
- Mody (Convergence, Reforged) mają zmodyfikowany regulation.bin

---

## Źródła

- er-save-manager: `parser/save.py` linie 231-237 (raw read)
- ER-Save-Editor (Rust): `src/save/common/user_data_11.rs`
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=format:param
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:param:equipparamweapon (przykład param table)
