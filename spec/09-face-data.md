# 09 — Face Data (Character Creator)

> **Scope**: Character appearance parameters from the creator — face, body, colors, proportions.

---

## Overview

Face Data is a 303-byte block (0x12F) containing all the character appearance parameters set in the creator. It appears in two variants:
- **In a slot**: 0x12F (303 bytes) — full data
- **In ProfileSummary (UserData10)**: 0x120 (288 bytes) — truncated (without the last 15 bytes)

---

## General structure

```
┌─────────────────────────────────────────────┐
│ Face Model IDs (8 × 4B = 32 bytes)           │  0x00–0x1F
├─────────────────────────────────────────────┤
│ Face Shape Parameters (~64 × u8)             │  0x20–0x5F (approximate)
├─────────────────────────────────────────────┤
│ Hair & Cosmetics (~30 × u8)                  │  0x60–0x7F (approximate)
├─────────────────────────────────────────────┤
│ Skin Colors & Body (~40 × u8)                │  0x80–0xAF (approximate)
├─────────────────────────────────────────────┤
│ Body Scale (7 × float? / byte?)              │  0xB0+ (approximate)
├─────────────────────────────────────────────┤
│ Trailing bytes (slot-only, 15B)              │  0x120–0x12E
└─────────────────────────────────────────────┘
```

**NOTE**: The offsets below come from Cheat Engine (runtime memory), where Face Data starts at PlayerGameData+0x754. In the save file, the offsets inside the Face Data block may differ — needs verification via hex dump. Field order and names are confirmed.

---

## Face Model IDs (8 × u32 = 32 bytes)

| Offset (CT) | Type | Field | Description |
|---|---|---|---|
| +0x00 | u32 | Face_Model_Id | Base face model |
| +0x04 | u32 | Hair_Model_Id | Hairstyle (values = IDs from the database) |
| +0x08 | u32 | Eye_Model_Id | Eye model |
| +0x0C | u32 | Eyebrow_Model_Id | Eyebrow model |
| +0x10 | u32 | Beard_Model_Id | Beard model |
| +0x14 | u32 | Accessories_Model_Id | Accessories (earrings, 3D makeup) |
| +0x18 | u32 | Decal_Model_Id | Decal (tattoo/scar) |
| +0x1C | u32 | Eyelash_Model_Id | Eyelash model |

---

## Face Shape Parameters (u8 each, range 0–255)

The value `128` = neutral/center slider position. Values below/above push in opposite directions.

### General face proportions

| Offset (CT: base 0x740) | Field | Description |
|---|---|---|
| +0x34 | Apparent Age | Apparent age (0=young, 255=old) |
| +0x35 | Facial Aesthetic | Facial aesthetic (general "attractiveness") |
| +0x36 | Form Emphasis | Feature emphasis (sharper vs softer) |
| +0x37 | Unk (Numen = 128) | Unknown — default 128, possibly tied to the Numen race? |

### Brows (Brow Ridge)

| Offset | Field | Description |
|---|---|---|
| +0x38 | Brow Ridge Height | Brow ridge height |
| +0x39 | Inner Brow Ridge | Inner brow ridge |
| +0x3A | Outer Brow Ridge | Outer brow ridge |

### Cheekbones

| Offset | Field | Description |
|---|---|---|
| +0x3B | Cheekbone Height | Cheekbone height |
| +0x3C | Cheekbone Depth | Depth (front–back) |
| +0x3D | Cheekbone Width | Width |
| +0x3E | Cheekbone Protrusion | Cheekbone protrusion |
| +0x3F | Cheeks | Cheeks (fullness/hollowness) |

### Chin

| Offset | Field | Description |
|---|---|---|
| +0x40 | Chin Tip Position | Chin tip position |
| +0x41 | Chin Length | Chin length |
| +0x42 | Chin Protrusion | Forward chin protrusion |
| +0x43 | Chin Depth | Chin depth |
| +0x44 | Chin Size | Chin size |
| +0x45 | Chin Height | Chin height |
| +0x46 | Chin Width | Chin width |

### Eyes

| Offset | Field | Description |
|---|---|---|
| +0x47 | Eye Position | Eye position (height) |
| +0x48 | Eye Size | Eye size |
| +0x49 | Eye Slant | Eye slant (up–down at the edges) |
| +0x4A | Eye Spacing | Eye spacing |

### Nose — 14 parameters

| Offset | Field | Description |
|---|---|---|
| +0x4B | Nose Size | Overall nose size |
| +0x4C | Nose/Forehead Ratio | Nose–forehead ratio |
| +0x4D | Unk | Unknown nose parameter |
| +0x66 | Nose Ridge Depth | Nose ridge depth |
| +0x67 | Nose Ridge Length | Ridge length |
| +0x68 | Nose Position | Nose position |
| +0x69 | Nose Tip Height | Nose tip height |
| +0x6A | Nostril Slant | Nostril slant |
| +0x6B | Nostril Size | Nostril size |
| +0x6C | Nostril Width | Nostril width |
| +0x6D | Nose Protrusion | Nose protrusion |
| +0x6E | Nose Bridge Height | Nose bridge height |
| +0x6F | Nose Bridge Protrusion 1 | Bridge protrusion (upper) |
| +0x70 | Nose Bridge Protrusion 2 | Bridge protrusion (lower) |
| +0x71 | Nose Bridge Width | Bridge width |
| +0x72 | Nose Height | Overall nose height |
| +0x73 | Nose Slant | Nose slant |

### Face General

| Offset | Field | Description |
|---|---|---|
| +0x4E | Face Protrusion | Face protrusion (profile) |
| +0x4F | Vertical Face Ratio | Vertical face ratio |
| +0x50 | Facial Feature Slant | Feature slant |
| +0x51 | Horizontal Face Ratio | Horizontal ratio |
| +0x52 | Unk | Unknown |
| +0x53 | Forehead Depth | Forehead depth |
| +0x54 | Forehead Protrusion | Forehead protrusion |
| +0x55 | Unk | Unknown |

### Jaw

| Offset | Field | Description |
|---|---|---|
| +0x56 | Jaw Protrusion | Jaw protrusion |
| +0x57 | Jaw Width | Jaw width |
| +0x58 | Lower Jaw | Lower jaw |
| +0x59 | Jaw Contour | Jaw contour |

### Mouth/Lips

| Offset | Field | Description |
|---|---|---|
| +0x5A | Lip Shape | Lip shape |
| +0x5B | Lip Size | Lip size |
| +0x5C | Lip Fullness | Lip fullness |
| +0x5D | Mouth Expression | Mouth expression (smile/frown) |
| +0x5E | Lip Protrusion | Lip protrusion |
| +0x5F | Lip Thickness | Lip thickness |
| +0x60 | Mouth Protrusion | Mouth area protrusion |
| +0x61 | Mouth Slant | Mouth slant |
| +0x62 | Mouth Occlusion | Mouth closure/openness |
| +0x63 | Mouth Position | Mouth position (vertical) |
| +0x64 | Mouth Width | Mouth width |
| +0x65 | Mouth-Chin Distance | Mouth–chin distance |

---

## Skin & Cosmetics (u8 each)

| Field | Description | Range |
|---|---|---|
| Skin_Color_R | Skin color — Red | 0–255 |
| Skin_Color_G | Skin color — Green | 0–255 |
| Skin_Color_B | Skin color — Blue | 0–255 |
| Skin_Color_A | Skin color — Alpha/Intensity | 0–255 |
| Skin_Pores | Skin pore visibility | 0–255 |
| Beard_Stubble | Stubble overlay | 0–255 |
| Skin_Dark_Circle | Under-eye shadows (intensity) | 0–255 |
| Skin_Dark_Circle_Color_R/G/B | Under-eye shadow color | 0–255 |
| Cheeks | Cheek blush | 0–255 |
| Cheeks_Color_R/G/B | Blush color | 0–255 |
| Eyeliner | Eyeliner (intensity) | 0–255 |
| Eyeliner_Color_R/G/B | Eyeliner color | 0–255 |
| Eyeshadow_Lower | Lower eyeshadow (intensity) | 0–255 |
| Eyeshadow_Lower_Color_R/G/B | Lower eyeshadow color | 0–255 |
| Eyeshadow_Upper | Upper eyeshadow (intensity) | 0–255 |
| Eyeshadow_Upper_Color_R/G/B | Upper eyeshadow color | 0–255 |
| Lipstick | Lipstick (intensity) | 0–255 |
| Lipstick_Color_R/G/B | Lipstick color | 0–255 |
| Decal_Position_X | Decal/tattoo X position | 0–255 |
| Decal_Position_Y | Decal/tattoo Y position | 0–255 |
| Body_Hair | Body hair (intensity) | 0–255 |
| Body_Hair_Color_R/G/B | Body hair color | 0–255 |

---

## Body Scale (7 parameters)

In memory (CT): float (4B each) at offsets 0x870–0x888 from the PlayerGameData base.
In the save file: probably also float or u8 (needs verification).

| Field | Description | Default value |
|---|---|---|
| Head | Head proportions | 1.0 (float) / 128 (u8) |
| Chest (Breast) | Chest proportions | 1.0 / 128 |
| Abdomen (Waist) | Abdomen/waist proportions | 1.0 / 128 |
| Arm Right | Right arm proportions | 1.0 / 128 |
| Leg Right | Right leg proportions | 1.0 / 128 |
| Arm Left | Left arm proportions | 1.0 / 128 |
| Leg Left | Left leg proportions | 1.0 / 128 |

---

## Usage context

- Copying face data between characters = an exact copy of 0x12F bytes
- Editing Model IDs changes hair/beard/brow without needing to know the shape parameters
- Face Data in ProfileSummary is used to display the character in the menu — it should be kept in sync
- Variant 0x120 vs 0x12F — when copying to ProfileSummary, trim the last 15 bytes

---

## Editing implications

- **Safe to copy** blob-to-blob between characters
- **Model IDs**: changing Hair_Model_Id = changes hairstyle (values from the game database)
- **Shape parameters**: value 128 = neutral; change of ±1 = minimal slider movement
- **Colors**: plain RGBA (0–255 per channel)
- **Body Scale**: float in memory; in the save it may be u8 (128=1.0) — needs verification
- **Trailing 15 bytes** (slot-only): probably additional parameters not exposed in the creator, or internal flags

---

## Sources

- er-save-manager: `parser/world.py` — class `FaceData` (lines 27-54)
- er-save-manager: `parser/user_data_x.py` line 119: `face_data: FaceData`
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Face Data AOB (PlayerGameData+0x754)
- Cheat Engine: `ER_TGA_v1.9.0` — Face Model IDs, Face Details (PlayerGameData+0x754+)
