# 14 — Game State (Stan Gry)

> **Zakres**: Menu profile, tutorial data, GameMan bytes, death count, session info, last grace, NG+ cycle, play time.

---

## Opis ogólny

Sekcja Game State zawiera różne dane o stanie rozgrywki — od licznika śmierci przez typ postaci po ostatni odpoczynek w grace. Składa się z kilku mniejszych pod-struktur.

---

## Kolejność pod-sekcji

```
1. Unknown fields (2 × u32)                           8 bytes
2. Menu Profile SaveLoad                               [VARIABLE]
3. Trophy Equip Data                                   (stała)
4. GaItem Game Data                                    8 + 7000×16 bytes
5. Tutorial Data                                       [VARIABLE]
6. GameMan bytes                                       3 bytes
7. Death/Character/Session state                       ~32 bytes (version-dependent)
```

---

## 1. Unknown Fields (8 bytes)

| Offset | Typ | Pole | Opis (z CT) |
|---|---|---|---|
| 0x00 | u32 | ClearCount | **NG+ cycle** (0=Journey 1, 1=NG+1, ..., 7=NG+7) |
| 0x04 | u32 | unk_gamedataman_0x88 | Nieznane (wewnętrzny GameDataMan field) |

**ClearCount** — potwierdzone z CT (`GameDataMan -> +0x120`):
- Wartość 0 = pierwsza podróż (Journey 1)
- Wartość 1–7 = NG+1 przez NG+7
- Max NG+7 (Journey 8) — mechaniki się nie zmieniają po NG+7

---

## 2. Menu Profile SaveLoad [VARIABLE]

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | u16 | unk0x0 | Nieznane |
| 0x02 | u16 | unk0x2 | Nieznane |
| 0x04 | u32 | Size | Rozmiar danych po headerze |
| 0x08 | u8[size] | Data | Dane profilu menu (typowo 0x1000 = 4096 bytes) |

Total: 8 + size bytes (typowo 0x1008)

---

## 3. Trophy Equip Data

Dane equipmentu do trofeum/achievement tracking. Stała struktura (rozmiar do weryfikacji).

---

## 4. GaItem Game Data (8 + 7000 × 16 = 112,008 bytes)

Tablica 7000 wpisów opisujących "historię" pozyskanych przedmiotów. **Krytyczna** — każda broń/AoW musi mieć wpis tutaj, inaczej gra crashuje (EXCEPTION_ACCESS_VIOLATION).

### Header:
| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | i64 | Count | Liczba unikalnych pozyskanych przedmiotów |

### Entry (16 bytes):
| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | u32 | ItemID | ID przedmiotu |
| 0x04 | u8 | ReinforceType | Typ wzmocnienia (itemID % 100 — powiązane z upgrade path) |
| 0x05 | u8[3] | Padding | Padding |
| 0x08 | u32 | NextItemID | Następny ID w łańcuchu (linked list structure) |
| 0x0C | u8 | Unk | Nieznane |
| 0x0D | u8[3] | Padding | Padding |

---

## 5. Tutorial Data [VARIABLE]

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | u16 | unk0x0 | Nieznane |
| 0x02 | u16 | unk0x2 | Nieznane |
| 0x04 | u32 | Size | Rozmiar sekcji |
| 0x08 | u32 | Count | Liczba ukończonych tutoriali |
| 0x0C | u32[Count] | TutorialIDs | ID ukończonych tutoriali |

---

## 6. GameMan Bytes (3 bytes)

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | u8 | gameman_0x8c | Nieznane (flaga stanu?) |
| 0x01 | u8 | gameman_0x8d | Nieznane |
| 0x02 | u8 | gameman_0x8e | Nieznane |

---

## 7. Death/Character/Session State

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | u32 | TotalDeathCount | **Łączna liczba śmierci** postaci |
| 0x04 | u32 | CharacterType | Typ postaci (0=Host, 1=WhitePhantom, 2=DarkSpirit, 3=Ghost) |
| 0x08 | u32 | InOnlineSession | Flaga sesji online (0=offline, 1=w sesji) |
| 0x0C | u32 | CharacterTypeOnline | Typ postaci w online (jak CharacterType) |
| 0x10 | u32 | LastRestedGrace | **Grace entity ID** (BonfireId) ostatniego odpoczynku |
| 0x14 | u32 | NotAloneFlag | Nie sam (1 = w co-op/invasion) |
| 0x18 | u32 | InGameTimer | Timer rozgrywki (countdown?) |
| 0x1C | u32 | unk_gamedataman | Nieznane pole GameDataMan |

### Version-dependent extra fields:
- `version >= 65`: + `temp_spawn_point_entity_id` (u32) — tymczasowy punkt spawnu
- `version >= 66`: + `game_man_0xcb3` (u8)

---

## Play Time (osobna sekcja w slocie)

**Play Time** w milisekundach jest przechowywany w osobnym polu slotu (nie w tej sekcji Game State):
- CT offset: `GameDataMan -> +0xA0`
- Typ: u32 (milisekundy od początku gry)
- Konwersja: `hours = value / 3,600,000`

---

## Pola z CT potwierdzone jako save-relevant

| Pole | CT Offset | Save sekcja | Edytowalne | Efekt |
|---|---|---|---|---|
| Death Count | GameDataMan+0x94 | Game State 7 [0x00] | Tak | Kosmetyczne (statystyka) |
| Play Time | GameDataMan+0xA0 | Osobna sekcja | Tak | Czas wyświetlany w menu |
| NG+ Cycle | GameDataMan+0x120 | Game State 1 [0x00] | Tak | Journey number, scaling enemies |
| Last Grace | GameMan+0xB30 | Game State 7 [0x10] | Tak | Punkt respawnu po śmierci / fast travel |
| Target Grace | GameMan+0xB3C | — | Do weryfikacji | Grace docelowa (warp in progress?) |
| Save Slot Index | GameMan+0xAC0 | — | Runtime only | Aktualny profil (nie w save per-slot) |

---

## Implikacje dla edycji

- **Death Count**: można wyzerować (czysto kosmetyczne, nie wpływa na gameplay)
- **Last Rested Grace**: zmiana = gracz spawni w innym grace po załadowaniu (teleportacja!)
- **NG+ Cycle**: zmiana 0→N = przejście do Journey N+1 (enemies scaling, boss loot reset)
- **Play Time**: zmiana czasu wyświetlanego w menu (kosmetyczne)
- **Tutorial Data**: wyzerowanie = ponowne wyświetlenie tutoriali
- **GaItem Game Data**: MUSI zawierać wpis dla każdej posiadanej broni/AoW — brak = crash
- **CharacterType**: powinien być 0 (Host) w normalnym save. Inne wartości = stan multiplayer
- **InOnlineSession**: powinien być 0 w zapisie offline

---

## Źródła

- er-save-manager: `parser/world.py` — `MenuSaveLoad` (linie 237-267), `GaitemGameData` (275-335), `TutorialData` (372-402)
- er-save-manager: `parser/user_data_x.py` linie 139-165
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Death Num, PlayTime, NG+ (ClearCount), LastGrace
- Cheat Engine: `ER_TGA_v1.9.0` — GameDataMan offsets, GameMan offsets, Save Slot Index
