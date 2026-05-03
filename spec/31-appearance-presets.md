# 31 — Appearance Presets (Mirror Favorites + Apply Algorithm)

> **Type**: Design doc  
> **Scope**: Binary format of appearance preset slots in Mirror Favorites (15 slots inside UserData10) and the algorithm for applying a preset to a character's FaceData. Reverse-engineered from test saves in `tmp/re-character/`.

> **Status**: ✅ Verified on real PC saves (Apr 2026). Full RE report: `tmp/re-character/findings.md`. PS4 — TODO (no test save available).

---

## Context

Mirror Favorites is the **appearance preset menu** accessible in-game at the mirror in Roundtable Hold. Each of the 15 slots can store a complete set of appearance parameters (face shape, body, skin, model IDs).

Characteristics:
- **Shared across all 10 character slots** — data lives in UserData10, not in individual slots
- **Gender-independent** — a slot stores a male or female preset regardless of the current character's gender
- **In-game editor** — the player creates presets in-game during character creation; slot 0 is the "last created preset" (default)

---

## Layout in UserData10

```
UserData10.Data (post-checksum, for PC skip 16 bytes MD5):

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

Full UserData10.Data size: 0x60000 (393,216 bytes). Unused trailing space is zero padding.

---

## Mirror Favorites slot — full layout (0x130 bytes)

| Offset (slot-relative) | Size | Field | Notes |
|---|---|---|---|
| 0x000 | 20 | `unk0x00` (header) | Opaque blob. Observed bytes on slot 0: `CE FA 00 00 D0 11 00 00 01 01 00 00 00 00 ...`. First 6 bytes = `0xFACE` u16 + `0x11D0` u32 (signatures — semantics unknown but the game requires them). Bytes `body_flag` (offset 0x08) = 1, `body_type` (offset 0x09) = 0/1. |
| 0x014 | 4 | `face_data_marker` (i32) | -1 (`0xFFFFFFFF`) = empty slot, 0 = active (game reads it). Inverted vs our prior documentation. Source: `er-save-manager character_presets.py:34,877`. |
| 0x018 | 4 | "FACE" magic | ASCII bytes — required signature of an active slot. |
| 0x01C | 4 | alignment (u32) | Always `4`. |
| 0x020 | 4 | size (u32) | Always `0x120` (288). |
| **0x024** | **32** | **Model IDs (8 × u32)** | Internal PartsId values, NOT UI indices. Each is u8 stored as u32 little-endian (3 padding bytes). Order: face_model, hair_model, eye_model (usually 0), eyebrow_model, beard_model, eyepatch_model, decal_model, eyelash_model. **Female preset example**: face=21, hair=124 (0x7C), eye=0, eyebrow=14, beard=0, eyepatch=0, decal=29 (0x1D), eyelash=3. |
| **0x044** | **64** | **Face shape (64 sliders)** | Byte-per-slider, order identical to FaceData blob @ 0x30. |
| 0x084 | 64 | `unk0x6c` | Opaque. **Preserved on apply** — the game does NOT overwrite the character slot's `unk0x6c` with the preset's value. Observed value very close to "default sliders" (most bytes = 0x80). |
| **0x0C4** | **7** | **Body proportions** | Full 7 bytes: head, chest, abdomen, arm_r, leg_r, arm_l, leg_l. ⚠️ er-save-manager `character_presets.py:328-335` interprets this as "body 5 + unk0xb1 2" — this is **misleading**: live save verification shows all 7 bytes are body proportions identical to FaceData blob @ 0xB0..0xB6. |
| **0x0CB** | **91** | **Skin & cosmetics** | Identical layout to FaceData blob @ 0xB7. Skin RGB, makeup, eyeliner, lipstick, tattoo, body_hair, eye colors, hair colors, beard colors, brow colors, eyelash + eyepatch colors. |
| 0x126 | 10 | trailing pad | Zeros. |

**Total**: 20 + 4 + 4 + 4 + 4 + 32 + 64 + 64 + 7 + 91 + 10 = **0x130** ✓

---

## Apply algorithm (preset → slot FaceData)

When the player selects an in-game preset from Mirror Favorites and accepts "Apply to character", the game executes the following steps:

```
1. Read 304 bytes from Mirror Favorites slot N.
2. Write to the active character's FaceData blob (slot.Data[fd..fd+0x12F]):
   - slot[fd+0x10..0x30]  ← preset[0x24..0x44]   (32 bytes: model IDs)
   - slot[fd+0x30..0x70]  ← preset[0x44..0x84]   (64 bytes: face shape)
   - PRESERVE slot[fd+0x70..0xB0]                 (64 bytes: unk0x6c — NOT touched!)
   - slot[fd+0xB0..0xB7]  ← preset[0xC4..0xCB]   (7 bytes: body proportions, including what ESM calls "unk0xb1")
   - slot[fd+0xB7..0x112] ← preset[0xCB..0x126]  (91 bytes: skin & cosmetics)
3. Update slot.Player.Gender:
   - Mirror Favorites body_type=0 → slot Gender=1 (male)
   - Mirror Favorites body_type=1 → slot Gender=0 (female)
   ⚠️ INVERSED relative to `preset.BodyType` in our `presets.go` (1=male, 0=female).
4. Clear gender-dependent equipment slots (helmet, chest etc.) — game writes 0xFFFFFFFF to GaItem handle slots.
   Reason: original equipment may not fit the new body model.
5. Adjust trailing FaceData flags (slot[fd+0x124..0x126]):
   - Observed: M→F apply changes `01 01 01` to `01 00 00`. Semantics of these 2 bits unknown.
   - Source: `tmp/re-character/facedata_dump.txt` (Trailing bytes 0x112..0x12E).
```

**Key insight**: for our editor to correctly apply a preset across M↔F, we need access to the **real internal PartsId** for each model (face, hair, eye, eyebrow, beard, eyepatch, decal, eyelash). These values are NOT sequential and CANNOT be derived from UI slider values like `1, 2, 3, ...`. See: female hair PartsId 124 (0x7C) for UI hair index 1 (first available in-game option).

---

## Implications for our code

### `app.go::WriteSelectedToFavorites`

Writes a preset to a Mirror Favorites slot. Currently uses our UI-decomposed presets (`backend/db/data/presets.go`):
- ✅ Header: 0xFACE u16 @ 0x00, 0x11D0 u32 @ 0x04, body_flag @ 0x08, body_type @ 0x09 — matches layout
- ✅ FACE magic @ 0x18, alignment @ 0x1C, size @ 0x20 — matches
- ⚠️ Model IDs (0x24..0x44) — written only for `preset.BodyType==1` (male) with UI-1 mapping. **For female presets omitted (zeros) → bald character in-game.**
- ✅ FaceShape (0x44..0x84) — copied verbatim from `preset.FaceShape[64]`
- ⚠️ unk0x6c (0x84..0xC4) — copied from active character (`slot.Data[fd+FDOffUnknownBlock]`). This is OK because the game ignores the preset's unk0x6c on apply, so the value doesn't affect the result. BUT it may be visible in the Mirror preview menu.
- ⚠️ Body (0xC4..0xCB) — writes 7 bytes from `preset.Body[7]`. ESM documents "5+2 unk0xb1" but in reality it's one contiguous 7-byte region. Current logic is CORRECT.
- ✅ Skin (0xCB..0x126) — copied verbatim from `preset.Skin[91]`

**Structural problem**: our `presets.go` stores presets as UI-decomposed (HairModel: 9 = "long curly hair" UI option), not as raw bytes. Without a complete UI→PartsId mapping for both genders, we cannot produce correct Model IDs.

### `app.go::ApplyAppearancePreset` (direct apply, without Mirror)

Writes directly to the slot's FaceData blob. Same problem — without real PartsId values we cannot produce correct `face_model` etc.

**Practical fix paths:**

**A. Re-source `presets.go` as raw bytes.** Each preset is a 0x130-byte blob copied from a real save after creating it in-game. Requires sourcing each preset manually. After implementation, apply = direct byte copy (no UI mapping needed).

**B. New feature: "Apply from Mirror Favorites slot N to character".** Player creates/imports a preset into Mirror Favorites (or uses an existing one), we click "Apply to character" and the editor copies bytes from the Mirror slot following the algorithm above. **Cross-gender works automatically** because the preset contains real PartsId values.

Option **B** is cheaper to implement and leverages existing player presets.

### Implementation status (Apr 2026)

- ✅ **Apply Mirror Favorites slot N to character** (Option B) — implemented, `app.go::ApplyMirrorFavoriteToCharacter`. Works correctly for presets created in-game (mirror in Roundtable Hold). Test: `app_apply_mirror_test.go` (RE-verified on `tmp/re-character/ER0000-before/after.sl2`).
- ❌ **Add to Mirror for Type B (female)** — UI guard in `AppearanceTab.tsx::handleWriteFavorites` blocks write. `WriteSelectedToFavorites` produces a slot with `Model IDs = 0` (bald + default male face in-game). Fix requires Option A.
- 🔜 **Re-source `presets.go` as raw 0x130 B blobs** (Option A) — TODO, future task. After implementation the UI guard can be removed and `WriteSelectedToFavorites` will produce correct Mirror slots for both genders.

---

## Slot allocation: known bug in our code

`backend/core/offset_defs.go::FavSafeSlots = [0, 10, 11, 12, 13, 14]` — historical workaround to avoid collisions with `save_manager.go::flushMetadata` which wrote ProfileSummary at `0x31A + i*0x100`.

**Root cause**: ProfileSummary offset was wrong (0x31A instead of 0x195E). Every write by our editor corrupted Mirror Favorites slot 1 (0x31A lies 0x96 bytes inside slot 1 @ 0x284..0x3B3).

After fixing the ProfileSummary offset (separate task — see `spec/23-user-data-10.md`), `FavSafeSlots` can be removed and presets allocated in order `0..14`.

---

## Sources

- **Reference repo (Python)**: `tmp/repos/er-save-manager/src/er_save_manager/parser/character_presets.py:226-433` — `FacePreset.read/write` definition. Note: ESM splits body into "5+2 unk0xb1" which is a misleading interpretation (RE shows it's contiguous 7 bytes of body proportions).
- **Reference repo (Rust)**: `tmp/repos/ER-Save-Editor/src/save/pc/user_data_10.rs:84-122` — UserData10 layout post-checksum.
- **RE saves**: `tmp/re-character/ER0000-before.sl2` (male character, slot 4, default) and `ER0000-after.sl2` (after applying preset 1 = female). Full diff: `tmp/re-character/findings.md`.
- **Our code**: `app.go::ApplyAppearancePreset` (lines 2401+), `app.go::WriteSelectedToFavorites` (lines 2540+), `backend/core/offset_defs.go::FavOff*` constants.
