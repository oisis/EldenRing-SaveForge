# 23 — UserData10 (Profil Konta)

> **Zakres**: Sekcja wspólna dla wszystkich slotów — Mirror Favorites preset slots, ProfileSummary, SteamID, active slots, CSMenuSystemSaveLoad.

> **Status**: ✅ PC i PS4 offsety zweryfikowane na żywych save'ach (Apr 2026): PC z `tmp/re-character/ER0000-{before,after}.sl2`, PS4 z `tmp/save/oisisk_ps4.txt`. **PC i PS4 mają IDENTYCZNY layout UserData10** (różnią się tylko nagłówkami pliku save i obecnością/brakiem checksumu — sama UserData10.Data jest taka sama).

> **Sekcja powiązana**: [31 — Appearance Presets](31-appearance-presets.md) — szczegółowy layout Mirror Favorites preset slot (0x130 bajtów each).

---

## Opis ogólny

UserData10 to sekcja po 10 slotach postaci. Zawiera:
- Informacje o koncie (Steam ID, ustawienia UI)
- 15 slotów presetów wyglądu (Mirror Favorites — wspólne dla wszystkich postaci)
- 10 podsumowań postaci (ProfileSummary) — wyświetlane w menu wyboru postaci
- Flagi aktywnych slotów (10 bajtów)
- Dodatkowe dane menu systemowego

Rozmiar: 0x60000 bajtów (393,216 bytes) — stały, niezależnie od liczby aktywnych postaci.

Na PC: poprzedzone 16-bajtowym MD5 checksumem (jak sloty postaci). PS4 nie ma checksumu.

---

## Layout (post-checksum, PC zweryfikowane)

```
┌─────────────────────────────────────────────────────┐
│ [PC only] MD5 Checksum (16 bytes)                   │ — przed UserData10.Data
╞═════════════════════════════════════════════════════╡
│ Steam ID (u64) — 8 bytes                            │ @ 0x00
├─────────────────────────────────────────────────────┤
│ Settings / UI preferences (0x140 = 320 bytes)        │ @ 0x08
├─────────────────────────────────────────────────────┤
│ CSMenuSystemSaveLoad header (8 bytes: unk + length) │ @ 0x148
├─────────────────────────────────────────────────────┤
│ Mirror Favorites preset slots [15]                  │ @ 0x154
│  - Each slot: 0x130 bytes (304)                     │
│  - Total: 15 × 0x130 = 0x11D0 (4560 bytes)          │
│  - Span: 0x154..0x1323                              │
│  - Szczegóły layoutu: spec/31-appearance-presets.md │
├─────────────────────────────────────────────────────┤
│ CSMenuSystemSaveLoad trailer (~0x630 bytes)         │ @ 0x1324
├─────────────────────────────────────────────────────┤
│ Active Slots (10 × u8: 0x01 active, 0x00 empty)     │ @ 0x1954
├─────────────────────────────────────────────────────┤
│ ProfileSummary[10]                                  │ @ 0x195E
│  - Each: 0x24C bytes (588) — name + face snapshot   │
│  - Total: 10 × 0x24C = 0x16F8 (5880 bytes)          │
│  - Span: 0x195E..0x3055                             │
├─────────────────────────────────────────────────────┤
│ ... (więcej danych menu, gestures, regulation ver.) │ @ 0x3056
│                                                     │
│ Reszta to zera (padding do 0x60000)                 │
└─────────────────────────────────────────────────────┘
```

---

## Offsety (PC i PS4 identyczne, zweryfikowane)

| Pole | Offset | Notes |
|---|---|---|
| Steam ID | 0x00 (u64) | tylko PC; na PS4 te 8 bajtów ma inne znaczenie / zera |
| Settings | 0x08..0x147 | UI preferences, account |
| CSMenuSystemSaveLoad header | 0x148 | unk + length |
| Mirror Favorites preset[0] | **0x154** | każdy slot 0x130 bytes, 15 slotów |
| Active Slots | **0x1954** | 10 × u8 |
| ProfileSummary[0] | **0x195E** | każdy 0x24C bytes |
| ProfileSummary stride | **0x24C** | × 10 slotów = 0x16F8 bajtów |

⚠️ **HISTORYCZNY BUG (do końca Q2 2026)**: Nasz `backend/core/save_manager.go` zapisywał ProfileSummary na `0x31A + i*0x100` (PC) i `0x30A + i*0x100` (PS4). Te offsety leżą **wewnątrz Mirror Favorites preset slot 1** (slot 1 spans 0x284..0x3B3), więc każdy zapis korumpował slot 1 prezeta. Stąd istnienie `FavSafeSlots = [0, 10..14]` jako proteza. Po naprawie offsetu `FavSafeSlots` można usunąć — wszystkie 15 slotów presetów jest dostępnych.

---

## ProfileSummary (0x24C = 588 bytes per slot)

Podsumowanie postaci widoczne w menu wyboru postaci. Gra czyta TYLKO te dane przy pokazywaniu listy postaci.

| Offset (slot-relative) | Typ | Opis |
|---|---|---|
| 0x000 | 5 × u8 | Marker bytes (zaobserwowane: `01 01 01 01 01`) |
| 0x005 | 5 × u8 | Padding (zera) |
| 0x00A | u16[16] | **Character Name** (UTF-16LE, max 16 znaków + null) |
| 0x02A | 4 × u8 | Padding |
| 0x02E | u32 | Level |
| 0x032 | ... | (TODO szczegóły — zaobserwowano FACE magic, model IDs, FaceShape, etc.) |
| 0x040 | u8[0x12C] | FaceData snapshot (mirror slotu) — gra używa do podglądu wyglądu w menu |
| ... | ... | Pozostałe pola: equipment summary, archetype, starting gift, body_type |

**Ważne**: nasz kod aktualnie pisze tylko Name (32 bajty UTF-16) + Level (4 bajty) = 36 bajtów. Pozostałe 552 bajty na slot zachowują wartość ostatnio zapisaną przez grę (poprzedni zapis z gry). To jest **OK funkcjonalnie** — game odczytuje Name i Level (nasze poprawne) plus FaceData snapshot (stare ale zgodne z game data), więc menu pokazuje aktualne imię i poziom, ale snapshot wyglądu może być nieaktualny (kosmetyka).

ProfileSummary MUSI być zsynchronizowane z danymi w slocie postaci — inaczej menu pokazuje złe informacje.

---

## Active Slots (10 × u8 @ 0x1954)

Tablica `[10]u8` — wskazuje które sloty postaci (0-9) są aktywne.
- `0x01` = aktywny (postać istnieje)
- `0x00` = pusty

Modyfikacja: po dodaniu/usunięciu postaci trzeba zaktualizować odpowiedni bajt.

---

## Active Slots

Bitfield lub tablica flag — wskazuje które sloty (0-9) mają aktywne postacie.

---

## CSMenuSystemSaveLoad (0x60000 bytes)

Duży blok danych systemu menu — ustawienia HUD, preferencje wyświetlania, quickslot konfiguracja na poziomie konta.

---

## Implikacje dla edycji

- **Steam ID**: musi odpowiadać Steam ID gracza na PC — inaczej save nie załaduje się
- **ProfileSummary**: po edycji imienia/levelu w slocie TRZEBA zaktualizować też tutaj
- **Active Slots**: po dodaniu/usunięciu postaci trzeba zaktualizować
- **MD5**: po modyfikacji UserData10 na PC — przeliczyć checksum
- **Konwersja platform**: offsety Active Slots i ProfileSummary są RÓŻNE — błędny offset = uszkodzony save

---

## Źródła

- er-save-manager: `parser/user_data_10.py` — klasa `UserData10`
- er-save-manager: `parser/save.py` linie 209-228
- Steam Guide: https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037
