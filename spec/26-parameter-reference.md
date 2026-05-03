# 26 — Complete Editable Parameter Reference

> **Type**: Binary format spec  
> **Scope**: Complete description of every editable parameter in the save file — with in-game effect, allowed values, and caveats.

---

## 1. Attributes

### Vigor
- **Effect**: Scales Max HP, secondarily Fire Defense and Immunity (from lvl 31)
- **Range**: 1–99
- **Softcaps**: 40 (first), 60 (second)
- **Save offset**: PlayerGameData + 0x34 (u32)

| Vigor | HP | Vigor | HP |
|---|---|---|---|
| 1 | 300 | 40 | 1,450 |
| 10 | 414 | 50 | 1,704 |
| 15 | 522 | 60 | 1,900 |
| 20 | 652 | 70 | 1,959 |
| 25 | 800 | 80 | 2,015 |
| 30 | 994 | 90 | 2,065 |
| 35 | 1,216 | 99 | 2,100 |

### Mind
- **Effect**: Scales Max FP; secondarily Focus (Sleep/Madness resistance, from lvl 31)
- **Does NOT give** Memory Slots (those come from Memory Stones)
- **Range**: 1–99
- **Softcaps**: 55–60
- **Save offset**: PlayerGameData + 0x38 (u32)

| Mind | FP | Mind | FP |
|---|---|---|---|
| 1 | 40 | 35 | 200 |
| 10 | 78 | 40 | 235 |
| 15 | 95 | 50 | 300 |
| 20 | 121 | 60 | 350 |
| 25 | 147 | 99 | 450 |
| 30 | 173 | — | — |

Practical breakpoint: Mind 38 (221 FP) = full refill from max-level Cerulean Flask.

### Endurance
- **Effect**: Scales Stamina + Equip Load; secondarily Robustness (from lvl 31)
- **Range**: 1–99
- **Softcaps**: Stamina = 50, Equip Load = 60
- **Save offset**: PlayerGameData + 0x3C (u32)

| Endurance | Stamina | Equip Load |
|---|---|---|
| 1 | 80 | 45.0 |
| 20 | 113 | 64.1 |
| 30 | 130 | 77.6 |
| 40 | 142 | 90.9 |
| 50 | 155 | 105.2 |
| 60 | 158 | 120.0 |
| 99 | 170 | 160.0 |

### Strength
- **Effect**: STR weapon damage scaling; Physical Defense
- **Special**: Two-handing multiplies effective STR × 1.5
- **Range**: 1–99
- **Softcaps**: 55 (first), 80 (second) for weapon scaling
- **Save offset**: PlayerGameData + 0x40 (u32)

### Dexterity
- **Effect**: DEX weapon damage scaling; casting speed; reduced fall damage
- **Range**: 1–99
- **Softcaps**: 55 (first), 80 (second)
- **Save offset**: PlayerGameData + 0x44 (u32)

### Intelligence
- **Effect**: Sorcery scaling; Magic Defense; requirement for sorceries
- **Range**: 1–99
- **Softcaps**: 60 (first), 80 (second)
- **Save offset**: PlayerGameData + 0x48 (u32)

### Faith
- **Effect**: Incantation scaling; Holy/elemental Defense; requirement for incantations
- **Range**: 1–99
- **Softcaps**: 60 (first), 80 (second)
- **Save offset**: PlayerGameData + 0x4C (u32)

### Arcane
- **Effect**: Status buildup (Bleed, Poison, Sleep); Item Discovery; Vitality (Deathblight resist); Holy Defense
- **Range**: 1–99
- **Softcaps**: 45 for Item Discovery
- **Save offset**: PlayerGameData + 0x50 (u32)

---

## 2. Level & Runes

### Level
- **Formula**: `Level = Vigor + Mind + Endurance + Strength + Dexterity + Intelligence + Faith + Arcane - 79`
- **Range**: 1–713
- **Save offset**: PlayerGameData + 0x60 (u32)
- **Caveat**: Must match the sum of attributes. The game doesn't validate on load but matchmaking and respec will be broken.

### Runes (held)
- **Description**: Currently held runes — lost on death (recoverable from bloodstain)
- **Range**: 0–999,999,999 (u32 max)
- **Save offset**: PlayerGameData + 0x64 (u32)

### Total Get Soul (lifetime runes)
- **Description**: Total runes earned in character's history. Purely statistical.
- **Save offset**: PlayerGameData + 0x68 (u32)

---

## 3. HP / FP / SP

### HP (current)
- **Description**: Current health at time of save
- **Range**: 1 – BaseMaxHP
- **Save offset**: PlayerGameData + 0x08 (u32)
- **Caveat**: Game recalculates BaseMaxHP from Vigor after loading. Current HP above max will be clamped.

### MaxHP / BaseMaxHP
- **MaxHP**: With talismans/Great Rune (runtime calculation)
- **BaseMaxHP**: From Vigor alone (see Vigor table)
- **Save offset**: 0x0C / 0x10 (u32)
- **Caveat**: Safe to overwrite — game recalculates.

### FP (current) / MaxFP / BaseMaxFP
- **Analogous to HP**, scales with Mind
- **Save offset**: 0x14 / 0x18 / 0x1C (u32)

### SP (Stamina current) / MaxSP / BaseMaxSP
- **Analogous**, scales with Endurance
- **Save offset**: 0x24 / 0x28 / 0x2C (u32)

---

## 4. Status Buildups

Status effect accumulators. Normally = 0. When buildup >= threshold → effect activates.

| Field | Offset | Protects against | Governing Attribute | Effect on trigger |
|---|---|---|---|---|
| Immunity | 0x70 | Poison | Vigor (from 31) | HP drain ~90s |
| Immunity2 | 0x74 | Scarlet Rot | Vigor (from 31) | Faster HP drain |
| Robustness | 0x78 | Hemorrhage (Bleed) | Endurance (from 31) | Burst damage (% max HP) |
| Vitality | 0x7C | Deathblight | Arcane (from 31) | **Instant death** |
| Robustness2 | 0x80 | Frostbite | Endurance (from 31) | Burst + 20% vuln + stamina penalty (30s) |
| Focus | 0x84 | Sleep | Mind (from 31) | Immobilized for several seconds |
| Focus2 | 0x88 | Madness | Mind (from 31) | FP drain + burst + stagger (only humanoids) |

**Editing**: Setting to 0 = no accumulation. Safe. The game doesn't store the "threshold" here — threshold is calculated from attributes+armor.

---

## 5. Character Identity

### Character Name
- **Format**: UTF-16LE, max 16 characters + null terminator
- **Save offset**: PlayerGameData + 0x94 (34 bytes)
- **Caveat**: Must be synchronized with ProfileSummary in UserData10

### Gender (Body Type)
- **Values**: `0` = Type B (female model), `1` = Type A (male model)
- **Save offset**: PlayerGameData + 0xB6 (u8)
- **Caveat**: Changes body model — Face Data remains the same. Some clothing looks different.

### Archetype (Class)
- **Values**: 0=Vagabond, 1=Warrior, 2=Hero, 3=Bandit, 4=Astrologer, 5=Prophet, 6=Confessor, 7=Samurai, 8=Prisoner, 9=Wretch
- **Save offset**: PlayerGameData + 0xB7 (u8)
- **Effect**: Only label + minimum attributes at respec. Does NOT change starting stats.
- **Caveat**: Changing class allows respec to lower minimums (e.g. Wretch min=10 all).

### Voice Type
- **Values**: 0=Young 1, 1=Young 2, 2=Mature 1, 3=Mature 2, 4=Aged 1, 5=Aged 2
- **Save offset**: PlayerGameData + 0xBA (u8)
- **Effect**: Changes player sounds (screams, groans, rolling grunts)

### Starting Gift (Keepsake)
- **Values**: 0=None, 1=Crimson Amber Medallion, 2=Lands Between Rune, 3=Golden Seed, 4=Fanged Imp Ashes, 5=Cracked Pot, 6=Stonesword Key ×2, 7=Bewitching Branch, 8=Boiled Prawn, 9=Shabriri's Woe
- **Save offset**: PlayerGameData + 0xBB (u8)
- **Effect**: Purely informational after creation — the item is already in inventory.

---

## 6. Online Settings

### Furlcalling Finger Remedy
- **Description**: Activates visibility of summon signs. Consumable item — gets used up.
- **Values**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xD8 (u8)

### White Cipher Ring
- **Description**: Automatically requests help when invaded
- **Values**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xDB (u8)

### Blue Cipher Ring
- **Description**: Player is automatically summoned to help invaded players
- **Values**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xDC (u8)

### Matchmaking Weapon Level
- **Description**: Highest weapon level the character has ever possessed. Permanent. Cannot be lowered in-game.
- **Range**: 0–25 (normalized; Somber ×2.5 = Regular equivalent)
- **Save offset**: PlayerGameData + 0xDA (u8)
- **Effect on matchmaking**:

| Host Weapon | Can Match Min | Can Match Max |
|---|---|---|
| +0 | +0 | +3 |
| +5 | +2 | +8 |
| +10 | +6 | +14 |
| +15 | +11 | +20 |
| +20 | +15 | +25 |
| +25 | +20 | +25 |

- **Caveat**: NPC-gifted upgraded weapons (e.g. Rogier's Rapier +8) permanently raise the bracket even without equip.
- **Editing**: Lowering allows matchmaking with lower-level players. Not "illegal" but detectable.

### Great Rune Active
- **Description**: Whether an active Great Rune is working (requires prior Rune Arc use)
- **Values**: 0=inactive, 1=active
- **Save offset**: PlayerGameData + 0xF7 (u8)
- **Caveat**: Requires owning a Great Rune (event flag) + equipped Great Rune (equipment slot)

---

## 7. Flask System

### Max Crimson Flask (HP)
- **Description**: Maximum Flask of Crimson Tears charges
- **Range**: 0–14
- **Save offset**: PlayerGameData + 0xF9 (u8)
- **How to increase in-game**: Golden Seeds (30 total needed for max)

### Max Cerulean Flask (FP)
- **Description**: Maximum Flask of Cerulean Tears charges
- **Range**: 0–14
- **Save offset**: PlayerGameData + 0xFA (u8)
- **Constraint**: Crimson + Cerulean = max 14 (game enforces the split)

### Flask Upgrade Level
- **Description**: Healing/regeneration power per use
- **Range**: +0 to +12
- **Save offset**: Unknown exactly (likely in block 0xFB–0x10F)
- **How to increase in-game**: Sacred Tears (12 total in game)
- **Caveat**: Needs investigation — may be an event flag rather than a direct value

---

## 8. Passwords

### Multiplayer Password
- **Description**: Restricts matchmaking to players with the same password
- **Format**: UTF-16LE, max 8 characters + null
- **Save offset**: PlayerGameData + 0x110 (18 bytes)

### Group Passwords (1–5)
- **Description**: Makes it easier to see group summon signs (yellow signs)
- **Format**: same as above
- **Save offset**: 0x122, 0x134, 0x146, 0x158, 0x16A (18 bytes each)
- **Caveat**: Max 5 groups active simultaneously

---

## 9. DLC Blessings

### Scadutree Blessing
- **Description**: Increases player damage + damage negation **only in Realm of Shadow (DLC)**
- **Range**: 0–20
- **Save offset**: Depends on implementation (u8, ~-187 from MagicOffset)
- **Effect per level**: ~5% attack + ~5% negation (diminishing after lvl 12)
- **Max (lvl 20)**: +80% attack, -40% damage received
- **Collectible**: 50 Scadutree Fragments (2-3 per level)
- **No effect** in base game areas (Lands Between)

### Shadow Realm Blessing (Revered Spirit Ash)
- **Description**: Increases damage + negation for summoned spirits and Torrent **in DLC**
- **Range**: 0–10
- **Save offset**: u8, ~-186 from MagicOffset
- **Effect (max lvl 10)**: Spirits deal 1.75x damage, take 0.625x
- **Collectible**: 25 Revered Spirit Ashes
- **Exception**: Mimic Tear does not receive full bonuses (nerfed in patch 1.13)

---

## 10. Talisman Slots

### Additional Talisman Slot Count
- **Description**: How many additional talisman slots have been unlocked (beyond base 1)
- **Range**: 0–3 (total slots = 1 + value = max 4)
- **Save offset**: PlayerGameData + 0xBE (u8)
- **How to unlock in-game**:
  1. Defeat Margit → Talisman Pouch (slot 2)
  2. Defeat 2 Shardbearers + Enia → Talisman Pouch (slot 3)
  3. Defeat Godfrey, First Elden Lord → Talisman Pouch (slot 4)
- **Caveat**: Also requires corresponding event flags. The parameter alone without flags → game may ignore.

---

## 11. Equipment — Editable fields

### ArmStyle (Weapon Stance)
- **Values**: 0=EmptyHand, 1=OneHand, 2=LeftBothHand (2H left), 3=RightBothHand (2H right)
- **Save offset**: ActiveWeaponSlots + 0x00 (u32)
- **Caveat**: Must be consistent with equipped weapons. Powerstance has no separate value — activates automatically when two weapons of the same type in both hands.
- **Crash risk**: Invalid value (>3) may crash the game

### Active Weapon Slots
- **Values**: 0=Primary, 1=Secondary, 2=Tertiary
- **Save offset**: ActiveWeaponSlots + 0x04–0x18 (7 × u32)
- **Caveat**: Value beyond 0–2 = undefined behavior

---

## 12. Torrent (Horse)

### State
- **Values**: 1=INACTIVE (not summoned), 3=DEAD, 13=ACTIVE (riding)
- **Save offset**: RideGameData + 0x24 (u32)
- **BUG**: HP=0 + State=13 = **infinite loading screen** (known save corruption bug)
- **Fix**: When HP=0, set State=3

### HP
- **Description**: Torrent's health — scales with player level
- **Save offset**: RideGameData + 0x20 (i32)
- **Caveat**: Value above expected for the given level is likely clamped by the game

### Coordinates / Map ID / Angle
- **Description**: Torrent's last position
- **Save offset**: RideGameData + 0x00 (f32×3 pos, u8[4] map, f32×4 quat)
- **Editing**: Safe — Torrent teleports to the player when summoned

---

## 13. Blood Stain (Bloodstain)

### Runes (recoverable)
- **Description**: Runes lost on last death, recoverable from bloodstain
- **Save offset**: BloodStain + 0x34 (i32)
- **Editing**: Changing the value = changing recoverable runes. Setting to 0 = no bloodstain.

### Coordinates / Map ID
- **Description**: Bloodstain position in the world
- **Save offset**: BloodStain + 0x00 (f32×3 + f32×4 quat + u8[4] map)
- **Editing**: Changing position = bloodstain in new location. Watch for out-of-bounds.

---

## 14. Player Coordinates (Teleportation)

### Position (X, Y, Z)
- **Save offset**: PlayerCoordinates + 0x00 (3 × f32)
- **Editing**: Direct player teleportation! But requires a valid Map ID.

### Map ID (4 bytes)
- **Format**: [region_type, Y_grid, X_grid, layer]
- **Layer**: 0x3C = overworld, 0x3D+ = underground
- **Save offset**: PlayerCoordinates + 0x0C (u8[4])
- **Caveat**: Wrong MapID + position = spawn in void → falling → death → respawn at last grace

### Rotation (Quaternion)
- **Save offset**: PlayerCoordinates + 0x10 (4 × f32)
- **Editing**: Look direction after loading (low importance)

---

## 15. Game State

### Death Count
- **Description**: Total character death count
- **Save offset**: Game State section 7 + 0x00 (u32)
- **Editing**: Purely cosmetic — reset to 0 doesn't affect gameplay

### NG+ Cycle (ClearCount)
- **Description**: Journey number
- **Values**: 0=Journey 1 (first play), 1=NG+1, ..., 7=NG+7 (max)
- **Save offset**: Game State section 1 + 0x00 (u32)
- **Effect**:
  - Enemy scaling: NG+1 = 3–3.4× HP (early areas); NG+7 = ~1.45× over NG+1
  - Rune rewards: NG+1 = 5.5× (early); diminishing after NG+2
  - NG+7 = cap, further cycles are identical
- **Caveat**: Changing from 0 to N doesn't reset quest flags/items — character will be in Journey N+1 with current progress

### Last Rested Grace
- **Description**: Grace entity ID (BonfireId) of the last rest
- **Save offset**: Game State section 7 + 0x10 (u32)
- **Effect**: Spawn point after death and after loading save
- **Editing**: Changing = teleporting player to a different grace!
- **Caveat**: Value must be a valid BonfireId (see spec/15 Bonfire IDs table)

### Play Time
- **Description**: Play time in milliseconds
- **Save offset**: Separate section (IngameTimer)
- **Editing**: Cosmetic — changes time displayed in menu

---

## 16. Weather & Time

### WorldAreaTime
- **Hour**: 0–23 (u32)
- **Minute**: 0–59 (u32)
- **Second**: 0–59 (u32)
- **Save offset**: WorldAreaTime section (3 × u32 = 12B)
- **Effect**: Time of day after loading. Affects: NPC spawns (Night's Cavalry, Bell Bearing Hunter), lighting, ambient.

### WorldAreaWeather
- **Area ID**: u16 — region identifier
- **Weather Type**: u16 — weather type
- **Timer**: u32 — duration
- **Save offset**: WorldAreaWeather section (12B)
- **Effect**: Weather at time of loading

---

## 17. DLC Flags

### Shadow of the Erdtree Entry Flag
- **Description**: Whether character has entered the DLC (Realm of Shadow)
- **Values**: 0=not entered, 1=entered
- **Save offset**: DLC section + 0x01 (u8)
- **Effect**: One-time — once entered, cannot be undone in-game. Editing allows reset.
- **CRITICAL**: During platform conversion this byte MUST be zeroed if DLC is not installed → infinite loading

### Pre-order Gestures
- **DLC[0]**: "The Ring" gesture (0=no, 1=yes)
- **DLC[2]**: "Ring of Miquella" gesture (0=no, 1=yes)
- **CRITICAL**: Bytes 3–49 MUST be 0x00. Non-zero = save rejected.

---

## 18. Unlocked Regions (Fog of War)

### Region IDs
- **Description**: List of unlocked map regions (Fog of War removal)
- **Format**: u32 count + count × u32 region_id
- **Editing**: Adding a region ID = removing fog in that area
- **WARNING**: VARIABLE LENGTH — changing count shifts all subsequent sections!
- **Full list**: ~200+ region IDs (AllRegionIDs in db/data/maps.go)

---

## 19. Event Flags — most important editable

### Boss Defeat Flags
- **Effect**: Marks a boss as defeated. Opens new areas, NPC reactions.
- **Range**: 9100–9281 (global bosses), 10000000+ (field bosses)
- **Caveat**: Setting boss flag without quest flags → NPCs may be confused

### Grace Discovery Flags
- **Effect**: Discovering a Site of Grace — enables fast travel
- **Range**: 71xxx, 73xxx, 76xxx
- **Caveat**: Discovering grace without defeating the blocking boss = normal

### Map Visibility / Acquisition
- **Visibility (62xxx)**: Map visible (region texture displayed)
- **Acquired (63xxx)**: Map fragment "picked up" — should also add item to inventory
- **Caveat**: Visibility without Acquired = map visible but fragment not in inventory (cosmetically OK)

### Mechanic Unlocks (60xxx)
- **Editing**: Unlocking mechanics without quests (Torrent, crafting, etc.)
- **Safe**: Game doesn't check "how" a mechanic was unlocked

### Cookbook Flags (67xxx–68xxx)
- **Effect**: Unlocking crafting recipes
- **Alternative**: Adding cookbook item to inventory (same thing — possessing item = flag set)

---

## 20. Inventory & Storage — per-item fields

### Quantity
- **Description**: Amount of stackable items (weapons/armor = always 1)
- **Range**: 1 – MaxInventory/MaxStorage (per item, from database)
- **Save offset**: InventoryItem + 0x04 (u32)
- **Caveat**: Exceeding max = game clamps to max on use

### GaItem Handle
- **Description**: Reference to item instance in GaItem Map
- **Format**: 0xTTPPCCCC (TT=type, PP=part, CCCC=counter)
- **Caveat**: Handle MUST exist in GaItem Map. Orphaned handle = item doesn't display.

---

## 21. Face Data — editable groups

### Model IDs (change hairstyle/beard/eyebrows without creator)
- **Hair_Model_Id**: Change hairstyle (u32, values from game params)
- **Beard_Model_Id**: Change beard
- **Safe**: Model change doesn't affect stability

### Shape Parameters (creator sliders)
- **Range**: 0–255 (u8), 128 = neutral/center
- **Safe**: Any value in range is valid

### Colors (RGBA)
- **Range**: 0–255 per channel
- **Safe**: Any combination is valid

### Body Scale
- **Description**: Body proportions (Head, Chest, Abdomen, Arms, Legs)
- **Format in memory**: float (1.0 = normal)
- **Format in save**: Needs verification (may be u8 with 128=1.0)

---

## 22. Memory Slots & Spell System

### Memory Slot Count
- **Base**: 2 slots (all characters)
- **Memory Stones**: +8 (8 findable in game)
- **Moon of Nokstella** (talisman): +2 (equipment, not save field)
- **Max**: 12 slots
- **Tracking**: Memory Stones are key items in inventory. Possession = unlock.

### Equipped Spells (14 slots)
- **Format**: SpellID (u32) + Quantity (u32), stride 8B
- **Editing**: Changing SpellID = instant spell change
- **Caveat**: Game allows equipping max N slots (N = memory slots). But physically 14 slots exist.

---

## 23. Weapon & Spirit Ash Upgrade System

### Normal Weapons
- **Range**: +0 to +25
- **Materials**: Smithing Stones [1]–[8] + Ancient Dragon Smithing Stone (+25)
- **ID encoding**: baseID + upgrade_level (e.g. Uchigatana = 1000000, +5 = 1000005)
- **Infusions**: Supported (Standard, Heavy, Keen, Quality, Magic, Cold, Fire, Flame Art, Lightning, Sacred, Poison, Blood, Occult)
- **Infusion encoding**: baseID + infusion_offset + upgrade_level

### Somber Weapons
- **Range**: +0 to +10
- **Materials**: Somber Smithing Stones [1]–[9] + Somber Ancient Dragon Smithing Stone (+10)
- **No infusions** — unique weapons with fixed skills
- **ID encoding**: baseID + upgrade_level

### Spirit Ashes
- **Range**: +0 to +10
- **Materials**: Grave Glovewort [1]–[9] + Great Grave Glovewort (regular) OR Ghost Glovewort [1]–[9] + Great Ghost Glovewort (legendary)
- **ID encoding**: baseID + upgrade_level (same as weapons)

---

## 24. Summoning Pools & Colosseums

### Summoning Pools (Martyr Effigies)
- **Total**: ~165 in game (base + DLC)
- **Tracking**: Event flags (10000000+ range)
- **Editing**: Set flag = activated

### Colosseums
- **Total**: 3 (Caelid=60350, Limgrave=60360, Royal/Leyndell=60370)
- **Editing**: Set event flag = unlocked
- **Added**: Patch 1.08 (December 2022)

---

## 25. Unknown Parameters (to investigate)

| Offset | Description | Hypothesis |
|---|---|---|
| PlayerGameData 0x00–0x07 | 2 × u32 unknown | PlayerNo / internal ID? Runtime only? |
| PlayerGameData 0x20 | u32 between FP/SP | FP regen? MaxMP2? |
| PlayerGameData 0x30 | u32 after SP | SP regen? |
| PlayerGameData 0x54–0x5C | 3 × u32 after Arcane | DLC attributes? Unused extension? |
| PlayerGameData 0x6C | u32 after TotalGetSoul | Runes lost at death (recoverable)? |
| PlayerGameData 0x8C–0x90 | 2 × u32 after Madness | DLC buildups? Reserved? |
| PlayerGameData 0xB8–0xB9 | 2 × u8 in creation | Appearance/VowType? |
| PlayerGameData 0xBC–0xBD | 2 × u8 in creation | Related to starting equipment? |
| PlayerGameData 0xC0–0xD7 | 24 bytes unknown | Character flags? Online state? |
| PlayerGameData 0xDD–0xF6 | 26 bytes in online | Additional online settings? |
| PlayerGameData 0xFB–0x10F | 21 bytes after flask | Flask upgrade level? Physick data? |
| PlayerGameData 0x180–0x1AF | 48 bytes trailing | SwordArt scaling? Correction stats beginning? |

---

## Sources

- Elden Ring Wiki (Fextralife): https://eldenring.wiki.fextralife.com/
- Game8 Elden Ring: https://game8.co/games/Elden-Ring/
- Souls Modding Wiki: https://www.soulsmodding.com/
- Cheat Engine tables: ER_all-in-one_Hexinton_v3.10, ER_TGA_v1.9.0
- er-save-manager (Python parser): sequential field order reference
- Community spreadsheets: event flag mappings, param tables
