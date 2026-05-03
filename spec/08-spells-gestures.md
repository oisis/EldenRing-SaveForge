# 08 — Spells, Gestures, Projectiles

> **Type**: Binary format spec  
> **Scope**: Attuned spells (attunement), gestures, projectiles and related equipment data.

---

## Overview

After inventory, a series of structures follows describing:
1. Equipped Spells — attuned spells/incantations (attunement slots)
2. Equipped Items — quick slots and pouch
3. Equipped Gestures — active gestures (assigned to gesture ring)
4. Acquired Projectiles — collected projectiles (variable length!)
5. Equipped Armaments & Items — additional equipment data
6. Equipped Physics — Wondrous Physick mixes

---

## 1. Equipped Spells (14 slots × 8B = 112 bytes)

14 spell slots (attunement). Number of available slots depends on Mind (memory stones).

### Structure per slot (8 bytes):

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u32 | SpellID | Magic ID of the spell (from MagicParam) |
| 0x04 | u32 | Quantity | Amount (usually 1; 0 or 0xFFFFFFFF = empty) |

### Additional field:
| Offset | Type | Field | Description |
|---|---|---|---|
| (end) | i32 | SelectedSlotIdx | Currently selected slot (-1 = none, 0–13 = slot) |

- Each slot contains the spell's Item ID (not a handle — spells don't have instances in GaItem Map)
- Unused slots: SpellID = `0xFFFFFFFF`, Quantity = 0
- Stride between slots: 8 bytes
- Max 14 slots (with Memory Stones); availability depends on Mind stat

### Example Spell IDs (from MagicParam):

| ID | Spell | Type |
|---|---|---|
| 4000 | Glintstone Pebble | Sorcery |
| 4010 | Glintstone Arc | Sorcery |
| 4100 | Carian Slicer | Sorcery |
| 6000 | Heal | Incantation |
| 6300 | Lightning Spear | Incantation |
| 6600 | Catch Flame | Incantation |

---

## 2. Equipped Items (Quick Slots + Pouch) (64 bytes)

| Section | Count | Size | Description |
|---|---|---|---|
| Quick Slots | 10 | 40B (10 × u32) | Quick access (D-pad cycling) |
| Pouch | 6 | 24B (6 × u32) | Pouch slots (hold Y/Triangle + D-pad) |

Each slot: u32 Item ID. Value `0xFFFFFFFF` = empty.

### Quick Slots (10):
- Slots 1–10: items accessible by cycling (D-pad down)
- Player sees currently selected and can cycle through

### Pouch (6):
- Slots 1–4: accessible via shortcut (hold Y + direction)
- Slots 5–6: accessible only from menu

---

## 3. Equipped Gestures (gesture ring)

Gestures assigned to the "gesture ring" (quick access to emotes):
- 6–7 slots × u32 gesture ID
- Stride: 4 bytes
- Value 254 (0xFE) = None (empty slot)

---

## 4. Acquired Projectiles — WARNING: VARIABLE LENGTH

```
┌─────────────────────────────────┐
│ Count (u32)                      │  4 bytes
├─────────────────────────────────┤
│ Projectile entries: count × 8    │  [VARIABLE]
│   ├── projectile_id (u32)        │
│   └── unk (u32)                  │
└─────────────────────────────────┘
```

**This is one of the variable-length sections** — its size depends on the number of collected projectiles. All sections after it have shifted offsets.

---

## 5. Equipped Armaments & Items

Additional equipment structure — weapon equipment data in the context of ash of war / affinity. Details to verify.

---

## 6. Equipped Physics (Wondrous Physick)

2 slots for crystal tears for Flask of Wondrous Physick:

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u32 | CrystalTear1 | ID of first tear |
| 0x04 | u32 | CrystalTear2 | ID of second tear |

Value `0xFFFFFFFF` = empty slot.

---

## 7. Gestures (full list — 256 bytes)

Separate section (not to be confused with Equipped Gestures!) — full list of 64 gesture IDs (64 × u32 = 256 bytes). Contains ALL unlocked gestures, not just those in the gesture ring.

### Complete Gesture ID List

| ID | Gesture | ID | Gesture |
|---|---|---|---|
| 0 | Bow | 108 | Fire Spur Me |
| 2 | Polite Bow | 110 | The Carian Oath |
| 4 | My Thanks | 120 | Bravo! |
| 6 | Curtsy | 140 | Jump for Joy |
| 8 | Reverential Bow | 142 | Triumphant Delight |
| 10 | My Lord | 144 | Fancy Spin |
| 12 | Warm Welcome | 146 | Finger Snap |
| 14 | Wave | 160 | Dejection |
| 16 | Casual Greeting | 180 | Patches' Crouch |
| 18 | Strength! | 182 | Crossed Legs |
| 20 | As You Wish | 184 | Rest |
| 40 | Point Forwards | 186 | Sitting Sideways |
| 42 | Point Upwards | 188 | Dozing Cross-Legged |
| 44 | Point Downwards | 190 | Spread Out |
| 46 | Beckon | 192 | Fetal Position |
| 48 | Wait! | 194 | Balled Up |
| 50 | Calm Down! | 196 | What Do You Want? |
| 60 | Nod In Thought | 200 | Prayer |
| 80 | Extreme Repentance | 202 | Desperate Prayer |
| 82 | Grovel For Mercy | 204 | Rapture |
| 100 | Rallying Cry | 206 | Erudition |
| 102 | Heartening Cry | 208 | Outer Order |
| 104 | By My Sword | 210 | Inner Order |
| 106 | Hoslow's Oath | 212 | Golden Order Totality |
| — | — | 216 | The Ring (Pre-order DLC) |
| — | — | 218 | The Ring (Co-op variant) |
| — | — | 254 | None (empty slot) |

**Gesture unlock** is controlled by Event Flags (range 60800–60849).

---

## Editing implications

- **Spells**: Changing SpellID in an equipped slot = instant spell change. Doesn't require adding to inventory.
- **Quick Slots/Pouch**: Changing Item ID = changing the item in slot. Item must exist in inventory.
- **Acquired Projectiles**: Variable length — changing count shifts everything after it in the file.
- **Gestures**: Gesture IDs must be valid (from the table above). Invalid ones may cause crash.
- **Physics**: Crystal Tear IDs from GoodsParam. Invalid ID = crash.
- **14 spell slots**: This is the max regardless of Mind — the game simply won't allow equip if insufficient memory slots.

---

## Sources

- er-save-manager: `parser/equipment.py` — `EquippedSpells`, `EquippedItems`, `EquippedGestures`, `AcquiredProjectiles`, `EquippedArmamentsAndItems`, `EquippedPhysics`
- er-save-manager: `parser/user_data_x.py` lines 108-117
- er-save-manager: `parser/world.py` — `Gestures` class (line 63-89, 64 × u32)
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — EquipMagicData (14 slots stride 8), GestureGameData, Gesture IDs dropdown
- Cheat Engine: `ER_TGA_v1.9.0` — EquipMagicData structure, Quick Items, Pouch offsets
