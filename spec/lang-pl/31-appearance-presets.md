# 31 — Appearance Presets (Mirror Favorites + Apply Algorithm)

> **Zakres**: Format binarny slotów presetów wyglądu w Mirror Favorites (15 slotów wewnątrz UserData10) oraz algorytm aplikacji presetu na FaceData postaci. Reverse-engineered z save'ów testowych `tmp/re-character/`.

> **Status**: ✅ Zweryfikowane na prawdziwych save'ach PC (Apr 2026). Pełen raport RE: `tmp/re-character/findings.md`. PS4 — TODO (brak save'a testowego).

---

## Kontekst

Mirror Favorites to **menu presetów wyglądu** dostępne w grze przy lustrze w Roundtable Hold. Każdy z 15 slotów może przechowywać kompletny set parametrów wyglądu (face shape, body, skin, model IDs).

Cechy:
- **Wspólne dla wszystkich 10 slotów postaci** — dane żyją w UserData10, nie w pojedynczym slocie
- **Niezależne od płci postaci** — slot przechowuje preset kobiety lub mężczyzny niezależnie od płci aktualnej postaci
- **Editor z gry** — gracz tworzy preset w-grze podczas tworzenia postaci, slot 0 to "ostatni utworzony preset" (default)

---

## Layout w UserData10

```
UserData10.Data (post-checksum, dla PC pomijamy 16 bajtów MD5):

┌─────────────────────────────────────────────────────┐
│ Steam ID (u64) — 8 bytes                           │ @ 0x00
├─────────────────────────────────────────────────────┤
│ Settings (0x140 = 320 bytes — UI settings, account) │ @ 0x08
├─────────────────────────────────────────────────────┤
│ CSMenuSystemSaveLoad header (8 bytes: unk + length) │ @ 0x148
├─────────────────────────────────────────────────────┤
│ Mirror Favorites preset slots [15]                  │ @ 0x154
│  - Each slot: 0x130 bytes (304)                     │
│  - Total: 15 × 0x130 = 0x11D0 (4560)                │
│  - Span: 0x154..0x1323                              │
├─────────────────────────────────────────────────────┤
│ CSMenuSystemSaveLoad trailer (0x630 bytes)          │ @ 0x1324
├─────────────────────────────────────────────────────┤
│ Active Slots bitfield (10 bytes, u8 each)           │ @ 0x1954
│  - 0x01 = active, 0x00 = empty                      │
├─────────────────────────────────────────────────────┤
│ ProfileSummary[10] — character menu data            │ @ 0x195E
│  - Each: 0x24C bytes (588) — name + face snapshot   │
│  - Total: 10 × 0x24C = 0x16F8 (5880)                │
├─────────────────────────────────────────────────────┤
│ ... (more menu data, gestures, regulation versions) │ @ 0x3056
└─────────────────────────────────────────────────────┘
```

Pełen rozmiar UserData10.Data: 0x60000 (393,216 bajtów). Nieużywane końcówki to padding zera.

---

## Mirror Favorites slot — pełen layout (0x130 bajtów)

| Offset (slot-relative) | Size | Field | Notes |
|---|---|---|---|
| 0x000 | 20 | `unk0x00` (header) | Opaque blob. Zaobserwowane bajty na slocie 0: `CE FA 00 00 D0 11 00 00 01 01 00 00 00 00 ...`. Pierwsze 6 bajtów = `0xFACE` u16 + `0x11D0` u32 (sygnatury — semantyka nieznana, ale gra je wymaga). Bajty `body_flag` (offset 0x08) = 1, `body_type` (offset 0x09) = 0/1. |
| 0x014 | 4 | `face_data_marker` (i32) | -1 (`0xFFFFFFFF`) = empty slot, 0 = active (gra czyta). Inverted vs naszej dotychczasowej dokumentacji. Source: `er-save-manager character_presets.py:34,877`. |
| 0x018 | 4 | "FACE" magic | ASCII bytes — wymagana sygnatura aktywnego slotu. |
| 0x01C | 4 | alignment (u32) | Zawsze `4`. |
| 0x020 | 4 | size (u32) | Zawsze `0x120` (288). |
| **0x024** | **32** | **Model IDs (8 × u32)** | Internal PartsId values, NOT UI indices. Each is u8 stored as u32 little-endian (3 padding bytes). Order: face_model, hair_model, eye_model (zwykle 0), eyebrow_model, beard_model, eyepatch_model, decal_model, eyelash_model. **Przykład żeńskiego presetu**: face=21, hair=124 (0x7C), eye=0, eyebrow=14, beard=0, eyepatch=0, decal=29 (0x1D), eyelash=3. |
| **0x044** | **64** | **Face shape (64 sliders)** | Bajt-per-slider, kolejność identyczna z FaceData blob @ 0x30. |
| 0x084 | 64 | `unk0x6c` | Opaque. **Zachowywany przy apply** — gra NIE nadpisuje `unk0x6c` slotu postaci wartością z presetu. Zaobserwowana wartość bardzo zbliżona do "default sliders" (większość bajtów = 0x80). |
| **0x0C4** | **7** | **Body proportions** | Pełne 7 bajtów: head, chest, abdomen, arm_r, leg_r, arm_l, leg_l. ⚠️ er-save-manager `character_presets.py:328-335` interpretuje to jako "body 5 + unk0xb1 2" — to **mylące**: weryfikacja na żywym save'ie pokazuje że wszystkie 7 bajtów to body proportions identyczne z FaceData blob @ 0xB0..0xB6. |
| **0x0CB** | **91** | **Skin & cosmetics** | Identyczny układ jak FaceData blob @ 0xB7. Skin RGB, makeup, eyeliner, lipstick, tattoo, body_hair, eye colors, hair colors, beard colors, brow colors, eyelash + eyepatch colors. |
| 0x126 | 10 | trailing pad | Zera. |

**Razem**: 20 + 4 + 4 + 4 + 4 + 32 + 64 + 64 + 7 + 91 + 10 = **0x130** ✓

---

## Apply algorithm (preset → slot FaceData)

Gdy gracz wybiera w-grze preset z Mirror Favorites i akceptuje "Apply to character", gra wykonuje następujące kroki:

```
1. Wczytaj 304 bajty Mirror Favorites slot N.
2. Zapisz do FaceData blob aktywnej postaci (slot.Data[fd..fd+0x12F]):
   - slot[fd+0x10..0x30]  ← preset[0x24..0x44]   (32 bajty: model IDs)
   - slot[fd+0x30..0x70]  ← preset[0x44..0x84]   (64 bajty: face shape)
   - PRESERVE slot[fd+0x70..0xB0]                 (64 bajty: unk0x6c — NIE ruszane!)
   - slot[fd+0xB0..0xB7]  ← preset[0xC4..0xCB]   (7 bajtów: body proportions, łącznie z tym co ESM nazywa "unk0xb1")
   - slot[fd+0xB7..0x112] ← preset[0xCB..0x126]  (91 bajtów: skin & cosmetics)
3. Aktualizuj slot.Player.Gender:
   - Mirror Favorites body_type=0 → slot Gender=1 (mężczyzna)
   - Mirror Favorites body_type=1 → slot Gender=0 (kobieta)
   ⚠️ INVERSED względem `preset.BodyType` w naszym `presets.go` (1=male, 0=female).
4. Wyczyść equipment slot zależny od płci (helmet, chest etc.) — gra zapisuje 0xFFFFFFFF do GaItem handle slotów.
   Powód: oryginalny equipment może nie pasować do nowego modelu ciała.
5. Adjust trailing FaceData flags (slot[fd+0x124..0x126]):
   - Zaobserwowane: M→F apply zmienia `01 01 01` na `01 00 00`. Semantyka tych 2 bitów nieznana.
   - Source: `tmp/re-character/facedata_dump.txt` (Trailing bytes 0x112..0x12E).
```

**Kluczowy wniosek**: aby NASZ edytor poprawnie zaaplikował preset M↔F, musimy mieć dostęp do **prawdziwych internal PartsId** dla każdego modelu (face, hair, eye, eyebrow, beard, eyepatch, decal, eyelash). Te wartości NIE są sekwencyjne i NIE da się ich wyliczyć z UI sliderów typu `1, 2, 3, ...`. Patrz: female hair PartsId 124 (0x7C) dla UI hair index 1 (pierwsza dostępna w-grze opcja).

---

## Implikacje dla naszego kodu

### `app.go::WriteSelectedToFavorites`

Pisze preset do Mirror Favorites slot. Aktualnie używa naszych UI-decomposed presetów (`backend/db/data/presets.go`):
- ✅ Header: 0xFACE u16 @ 0x00, 0x11D0 u32 @ 0x04, body_flag @ 0x08, body_type @ 0x09 — zgodnie z layoutem
- ✅ FACE magic @ 0x18, alignment @ 0x1C, size @ 0x20 — zgodnie
- ⚠️ Model IDs (0x24..0x44) — pisane tylko dla `preset.BodyType==1` (męskie) z UI-1 mapping. **Dla żeńskich pomijane (zera) → bald character w grze.**
- ✅ FaceShape (0x44..0x84) — kopiowane verbatim z `preset.FaceShape[64]`
- ⚠️ unk0x6c (0x84..0xC4) — kopiowane z aktywnej postaci (`slot.Data[fd+FDOffUnknownBlock]`). To jest OK bo gra przy apply ignoruje unk0x6c presetu, więc wartość nie wpływa na rezultat. ALE może być widoczna w preview menu Mirror.
- ⚠️ Body (0xC4..0xCB) — pisane 7 bajtów z `preset.Body[7]`. ESM dokumentuje "5+2 unk0xb1" ale w rzeczywistości to jeden 7-bajtowy region. Aktualna logika POPRAWNA.
- ✅ Skin (0xCB..0x126) — kopiowane verbatim z `preset.Skin[91]`

**Problem strukturalny**: nasz `presets.go` przechowuje preset jako UI-decomposed (HairModel: 9 = "long curly hair" UI option), nie jako raw bytes. Bez kompletnego mapowania UI→PartsId dla obu płci nie wyprodukujemy poprawnych Model IDs.

### `app.go::ApplyAppearancePreset` (direct apply, bez Mirror)

Pisze bezpośrednio do FaceData blob slotu. Ten sam problem — bez prawdziwych PartsId nie wyprodukujemy poprawnego `face_model` etc.

**Praktyczne ścieżki naprawy:**

**A. Re-source `presets.go` jako raw bajty.** Każdy preset to 0x130-bajtowy blob skopiowany z prawdziwego save'a po stworzeniu w-grze. Wymaga sourcing każdego presetu manualnie. Po implementacji apply = bezpośrednie kopiowanie bajtów (bez UI mapowania).

**B. Nowy feature: "Apply from Mirror Favorites slot N to character".** Gracz tworzy/importuje preset do Mirror Favorites (lub używa już istniejącego), klikamy "Apply to character" i edytor kopiuje bajty z Mirror slotu zgodnie z algorytmem powyżej. **Cross-gender działa automatycznie** bo preset ma prawdziwe PartsId.

Opcja **B** jest tańsza w implementacji i wykorzystuje istniejące presety gracza.

### Status implementacji (Apr 2026)

- ✅ **Apply Mirror Favorites slot N to character** (Opcja B) — zaimplementowane, `app.go::ApplyMirrorFavoriteToCharacter`. Działa poprawnie dla preset stworzonych w-grze (lustro w Roundtable Hold). Test: `app_apply_mirror_test.go` (RE-zweryfikowany na `tmp/re-character/ER0000-before/after.sl2`).
- ❌ **Add to Mirror dla Type B (żeńskich)** — UI guard w `AppearanceTab.tsx::handleWriteFavorites` blokuje zapis. `WriteSelectedToFavorites` produkuje slot z `Model IDs = 0` (bald + default męska twarz w grze). Naprawa wymaga Opcji A.
- 🔜 **Re-source `presets.go` jako raw 0x130 B blobs** (Opcja A) — TODO, przyszły task. Po implementacji można usunąć UI guard i `WriteSelectedToFavorites` będzie produkował poprawne Mirror sloty dla obu płci.

---

## Slot allocation: znany bug w naszym kodzie

`backend/core/offset_defs.go::FavSafeSlots = [0, 10, 11, 12, 13, 14]` — historyczna proteza unikania kolizji z `save_manager.go::flushMetadata` które pisał ProfileSummary na `0x31A + i*0x100`.

**Root cause**: ProfileSummary offset był zły (0x31A zamiast 0x195E). Każdy zapis przez nasz edytor korumpował Mirror Favorites slot 1 (0x31A leży 0x96 bajtów wewnątrz slotu 1 @ 0x284..0x3B3).

Po naprawie ProfileSummary offsetu (osobny task — patrz `spec/23-user-data-10.md`), `FavSafeSlots` można usunąć i alokować presety w kolejności `0..14`.

---

## Źródła

- **Reference repo (Python)**: `tmp/repos/er-save-manager/src/er_save_manager/parser/character_presets.py:226-433` — definicja `FacePreset.read/write`. Uwaga: ESM dzieli body na "5+2 unk0xb1" co jest mylącą interpretacją (RE pokazuje że to ciągłe 7 bajtów body proportions).
- **Reference repo (Rust)**: `tmp/repos/ER-Save-Editor/src/save/pc/user_data_10.rs:84-122` — UserData10 layout post-checksum.
- **RE save'y**: `tmp/re-character/ER0000-before.sl2` (postać męska, slot 4, default) i `ER0000-after.sl2` (po apply preset 1 = żeński). Pełen diff: `tmp/re-character/findings.md`.
- **Nasz kod**: `app.go::ApplyAppearancePreset` (linie 2401+), `app.go::WriteSelectedToFavorites` (linie 2540+), `backend/core/offset_defs.go::FavOff*` constants.
