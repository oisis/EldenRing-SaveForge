# 08 — Spells, Gestures, Projectiles

> **Scope**: Memorized spells (attunement), gestures, projectiles and related equipment data.

---

## Overview

After the inventory comes a series of structures describing:
1. Equipped Spells — memorized spells/incantations (attunement slots)
2. Equipped Items — quick slots and pouch
3. Equipped Gestures — active gestures (mapped to the gesture wheel)
4. Acquired Projectiles — collected projectiles (variable length!)
5. Equipped Armaments & Items — additional equipment data
6. Equipped Physics — Wonderous Physick mixtures

---

## 1. Equipped Spells (14 slots × 8B = 112 bytes)

14 spell slots (attunement). The number of available slots depends on Mind (memory stones).

### Per-slot structure (8 bytes):

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u32 | SpellID | Spell magic ID (from MagicParam) |
| 0x04 | u32 | Quantity | Quantity (usually 1; 0 or 0xFFFFFFFF = empty) |

### Additional field:
| Offset | Type | Field | Description |
|---|---|---|---|
| (end) | i32 | SelectedSlotIdx | Currently selected slot (-1 = none, 0–13 = slot) |

- Each slot contains the Item ID of the spell (not a handle — spells have no instances in the GaItem Map)
- Unused slots: SpellID = `0xFFFFFFFF`, Quantity = 0
- Stride between slots: 8 bytes
- Max 14 slots (with Memory Stones); availability depends on the Mind stat

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
- Slots 1–10: items accessible via cycling (D-pad down)
- The player sees the currently selected one and can cycle

### Pouch (6):
- Slots 1–4: accessible via shortcut (hold Y + direction)
- Slots 5–6: accessible from the menu only

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

**This is one of the variable-length sections** — its size depends on the number of collected projectiles. All subsequent sections have shifted offsets.

---

## 5. Equipped Armaments & Items

Additional equipment structure — data about weapon equipment in the context of ash of war / affinity. Details needs verification.

---

## 6. Equipped Physics (Wonderous Physick)

2 slots for crystal tears used in the Flask of Wondrous Physick:

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u32 | CrystalTear1 | First tear ID |
| 0x04 | u32 | CrystalTear2 | Second tear ID |

Value `0xFFFFFFFF` = empty slot.

---

## 7. Gestures (full list — 256 bytes)

A separate section (not to be confused with Equipped Gestures!) — the full list of 64 gesture IDs (64 × u32 = 256 bytes). Contains ALL unlocked gestures, not only those in the gesture ring.

### Complete Gesture ID list

| ID | Gesture (EN) | ID | Gesture (EN) |
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

- **Spells**: Changing SpellID in an equipped slot = immediate spell change. No need to add to inventory.
- **Quick Slots/Pouch**: Changing the Item ID = changes the slot's item. The item must exist in inventory.
- **Acquired Projectiles**: Variable length — changing count shifts everything that follows in the file.
- **Gestures**: Gesture IDs must be valid (from the table above). Invalid IDs may crash the game.
- **Physics**: Crystal Tear IDs from GoodsParam. Invalid IDs = crash.
- **14 spell slots**: This is the cap regardless of Mind — the game simply will not let you equip if you have too few memory slots.

---

## Sources

- er-save-manager: `parser/equipment.py` — `EquippedSpells`, `EquippedItems`, `EquippedGestures`, `AcquiredProjectiles`, `EquippedArmamentsAndItems`, `EquippedPhysics`
- er-save-manager: `parser/user_data_x.py` lines 108-117
- er-save-manager: `parser/world.py` — `Gestures` class (lines 63-89, 64 × u32)
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — EquipMagicData (14 slots stride 8), GestureGameData, Gesture IDs dropdown
- Cheat Engine: `ER_TGA_v1.9.0` — EquipMagicData structure, Quick Items, Pouch offsets
