# 26 — Complete Editable Parameter Reference

> **Scope**: A complete description of every parameter in the save file that can be edited — with its in-game effect, allowed values and caveats.

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
- **Does NOT give** Memory Slots (Memory Stones do that)
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

Practical breakpoint: Mind 38 (221 FP) = full refill from a max-level Cerulean Flask.

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

## 2. Level and Runes

### Level
- **Formula**: `Level = Vigor + Mind + Endurance + Strength + Dexterity + Intelligence + Faith + Arcane - 79`
- **Range**: 1–713
- **Save offset**: PlayerGameData + 0x60 (u32)
- **Caveat**: Must match the sum of attributes. The game does not validate on load but matchmaking and respec will be broken.

### Runes (held)
- **Description**: Runes currently held — lost on death (recoverable from a bloodstain)
- **Range**: 0–999,999,999 (u32 max)
- **Save offset**: PlayerGameData + 0x64 (u32)

### Total Get Soul (lifetime runes)
- **Description**: Total runes earned over the character's history. Purely a statistic.
- **Save offset**: PlayerGameData + 0x68 (u32)

---

## 3. HP / FP / SP

### HP (current)
- **Description**: Current health at the moment of saving
- **Range**: 1 – BaseMaxHP
- **Save offset**: PlayerGameData + 0x08 (u32)
- **Caveat**: The game recalculates BaseMaxHP from Vigor after load. Current HP above max will be clamped.

### MaxHP / BaseMaxHP
- **MaxHP**: With talismans/Great Rune (runtime calculation)
- **BaseMaxHP**: From Vigor alone (see Vigor table)
- **Save offset**: 0x0C / 0x10 (u32)
- **Caveat**: Safe to overwrite — the game recalculates.

### FP (current) / MaxFP / BaseMaxFP
- **Analogous to HP**, scales with Mind
- **Save offset**: 0x14 / 0x18 / 0x1C (u32)

### SP (Stamina current) / MaxSP / BaseMaxSP
- **Analogous**, scales with Endurance
- **Save offset**: 0x24 / 0x28 / 0x2C (u32)

---

## 4. Status Buildups

Status-effect accumulators. Normally = 0. When buildup >= threshold → the effect activates.

| Field | Offset | Resists | Governing Attribute | Effect on trigger |
|---|---|---|---|---|
| Immunity | 0x70 | Poison | Vigor (from 31) | HP drain ~90s |
| Immunity2 | 0x74 | Scarlet Rot | Vigor (from 31) | Faster HP drain |
| Robustness | 0x78 | Hemorrhage (Bleed) | Endurance (from 31) | Burst damage (% max HP) |
| Vitality | 0x7C | Deathblight | Arcane (from 31) | **Instant death** |
| Robustness2 | 0x80 | Frostbite | Endurance (from 31) | Burst + 20% vuln + stamina penalty (30s) |
| Focus | 0x84 | Sleep | Mind (from 31) | Immobilized for a few seconds |
| Focus2 | 0x88 | Madness | Mind (from 31) | FP drain + burst + stagger (only humanoids) |

**Editing**: Setting to 0 = no buildup. Safe. The game does not store the "threshold" here — the threshold is computed from attributes+armor.

---

## 5. Character identity

### Character Name
- **Format**: UTF-16LE, max 16 characters + null terminator
- **Save offset**: PlayerGameData + 0x94 (34 bytes)
- **Caveat**: Must be synced with ProfileSummary in UserData10

### Gender (Body Type)
- **Values**: `0` = Type B (female model), `1` = Type A (male model)
- **Save offset**: PlayerGameData + 0xB6 (u8)
- **Caveat**: Body-model change — Face Data stays the same. Some clothes look different.

### Archetype (Class)
- **Values**: 0=Vagabond, 1=Warrior, 2=Hero, 3=Bandit, 4=Astrologer, 5=Prophet, 6=Confessor, 7=Samurai, 8=Prisoner, 9=Wretch
- **Save offset**: PlayerGameData + 0xB7 (u8)
- **Effect**: Only the label + minimum attributes at respec. Does NOT change starting stats.
- **Caveat**: Changing class allows a respec down to lower minimums (e.g., Wretch min=10 all).

### Voice Type
- **Values**: 0=Young 1, 1=Young 2, 2=Mature 1, 3=Mature 2, 4=Aged 1, 5=Aged 2
- **Save offset**: PlayerGameData + 0xBA (u8)
- **Effect**: Changes player vocal sounds (screams, grunts, rolling grunt)

### Starting Gift (Keepsake)
- **Values**: 0=None, 1=Crimson Amber Medallion, 2=Lands Between Rune, 3=Golden Seed, 4=Fanged Imp Ashes, 5=Cracked Pot, 6=Stonesword Key ×2, 7=Bewitching Branch, 8=Boiled Prawn, 9=Shabriri's Woe
- **Save offset**: PlayerGameData + 0xBB (u8)
- **Effect**: Purely informational after creation — the item is already in the inventory.

---

## 6. Online settings

### Furlcalling Finger Remedy
- **Description**: Activates summon-sign visibility. Consumable item — gets used up.
- **Values**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xD8 (u8)

### White Cipher Ring
- **Description**: Automatically requests help when invaded
- **Values**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xDB (u8)

### Blue Cipher Ring
- **Description**: The player is automatically summoned to help invaded players
- **Values**: 0=off, 1=on
- **Save offset**: PlayerGameData + 0xDC (u8)

### Matchmaking Weapon Level
- **Description**: The highest weapon level the character has ever owned. Permanent. Cannot be lowered in-game.
- **Range**: 0–25 (normalized; Somber ×2.5 = Regular equivalent)
- **Save offset**: PlayerGameData + 0xDA (u8)
- **Matchmaking effect**:

| Host Weapon | Can Match Min | Can Match Max |
|---|---|---|
| +0 | +0 | +3 |
| +5 | +2 | +8 |
| +10 | +6 | +14 |
| +15 | +11 | +20 |
| +20 | +15 | +25 |
| +25 | +20 | +25 |

- **Caveat**: NPC-gifted upgraded weapons (e.g., Rogier's Rapier +8) permanently raise the bracket even without equipping.
- **Editing**: Lowering it allows matchmaking with lower-level players. Not "illegal" but detectable.

### Great Rune Active
- **Description**: Whether the active Great Rune works (requires prior Rune Arc use)
- **Values**: 0=inactive, 1=active
- **Save offset**: PlayerGameData + 0xF7 (u8)
- **Caveat**: Requires owning the Great Rune (event flag) + an equipped Great Rune (equipment slot)

---

## 7. Flask system

### Max Crimson Flask (HP)
- **Description**: Maximum number of Flask of Crimson Tears charges
- **Range**: 0–14
- **Save offset**: PlayerGameData + 0xF9 (u8)
- **How to increase in-game**: Golden Seeds (30 total needed for max)

### Max Cerulean Flask (FP)
- **Description**: Maximum number of Flask of Cerulean Tears charges
- **Range**: 0–14
- **Save offset**: PlayerGameData + 0xFA (u8)
- **Constraint**: Crimson + Cerulean = max 14 (the game enforces the split)

### Flask Upgrade Level
- **Description**: Healing/regeneration power per use
- **Range**: +0 to +12
- **Save offset**: Not known exactly (probably in the block 0xFB–0x10F)
- **How to increase in-game**: Sacred Tears (12 total in-game)
- **Caveat**: needs verification — may be an event flag rather than a direct value

---

## 8. Passwords

### Multiplayer Password
- **Description**: Restricts matchmaking to players using the same password
- **Format**: UTF-16LE, max 8 characters + null
- **Save offset**: PlayerGameData + 0x110 (18 bytes)

### Group Passwords (1–5)
- **Description**: Helps see group summon signs (yellow signs)
- **Format**: as above
- **Save offset**: 0x122, 0x134, 0x146, 0x158, 0x16A (18 bytes each)
- **Caveat**: Max 5 groups active simultaneously

---

## 9. DLC Blessings

### Scadutree Blessing
- **Description**: Increases player damage + damage negation **only in the Realm of Shadow (DLC)**
- **Range**: 0–20
- **Save offset**: Implementation-dependent (u8, ~-187 from MagicOffset)
- **Effect per level**: ~5% attack + ~5% negation (diminishing after lvl 12)
- **Max (lvl 20)**: +80% attack, -40% damage received
- **Collectible**: 50 Scadutree Fragments (2-3 per level)
- **No effect** in base-game areas (Lands Between)

### Shadow Realm Blessing (Revered Spirit Ash)
- **Description**: Increases damage + negation for summoned spirits and Torrent **in the DLC**
- **Range**: 0–10
- **Save offset**: u8, ~-186 from MagicOffset
- **Effect (max lvl 10)**: Spirits deal 1.75x damage, take 0.625x
- **Collectible**: 25 Revered Spirit Ashes
- **Exception**: Mimic Tear does not receive full bonuses (nerf in patch 1.13)

---

## 10. Talisman Slots

### Additional Talisman Slot Count
- **Description**: How many extra talisman slots have been unlocked (beyond the base 1)
- **Range**: 0–3 (total slots = 1 + value = max 4)
- **Save offset**: PlayerGameData + 0xBE (u8)
- **How to unlock in-game**:
  1. Defeat Margit → Talisman Pouch (slot 2)
  2. Defeat 2 Shardbearers + Enia → Talisman Pouch (slot 3)
  3. Defeat Godfrey, First Elden Lord → Talisman Pouch (slot 4)
- **Caveat**: Also requires the matching event flags. The parameter alone without the flags → the game may ignore it.

---

## 11. Equipment — editable fields

### ArmStyle (Weapon Stance)
- **Values**: 0=EmptyHand, 1=OneHand, 2=LeftBothHand (2H left), 3=RightBothHand (2H right)
- **Save offset**: ActiveWeaponSlots + 0x00 (u32)
- **Caveat**: Must be consistent with equipped weapons. Powerstance has no separate value — it activates automatically when two weapons of the same type are held in both hands.
- **Crash risk**: Invalid value (>3) may crash the game

### Active Weapon Slots
- **Values**: 0=Primary, 1=Secondary, 2=Tertiary
- **Save offset**: ActiveWeaponSlots + 0x04–0x18 (7 × u32)
- **Caveat**: Value outside 0–2 = undefined behavior

---

## 12. Torrent (Horse)

### State
- **Values**: 1=INACTIVE (not summoned), 3=DEAD, 13=ACTIVE (riding)
- **Save offset**: RideGameData + 0x24 (u32)
- **BUG**: HP=0 + State=13 = **infinite loading screen** (known save-corruption bug)
- **Fix**: When HP=0, set State=3

### HP
- **Description**: Torrent's health — scales with player level
- **Save offset**: RideGameData + 0x20 (i32)
- **Caveat**: A value above the expected one for the given level is probably clamped by the game

### Coordinates / Map ID / Angle
- **Description**: Torrent's last position
- **Save offset**: RideGameData + 0x00 (f32×3 pos, u8[4] map, f32×4 quat)
- **Editing**: Safe — Torrent teleports to the player when summoned

---

## 13. Blood Stain

### Runes (recoverable)
- **Description**: Runes lost on the most recent death, recoverable from the bloodstain
- **Save offset**: BloodStain + 0x34 (i32)
- **Editing**: Changing the value = changing the runes to recover. Setting 0 = no bloodstain.

### Coordinates / Map ID
- **Description**: Bloodstain position in the world
- **Save offset**: BloodStain + 0x00 (f32×3 + f32×4 quat + u8[4] map)
- **Editing**: Changing the position = bloodstain in a new place. Watch out for out-of-bounds.

---

## 14. Player Coordinates (Teleportation)

### Position (X, Y, Z)
- **Save offset**: PlayerCoordinates + 0x00 (3 × f32)
- **Editing**: Direct player teleportation! But it requires a valid Map ID.

### Map ID (4 bytes)
- **Format**: [region_type, Y_grid, X_grid, layer]
- **Layer**: 0x3C = overworld, 0x3D+ = underground
- **Save offset**: PlayerCoordinates + 0x0C (u8[4])
- **Caveat**: A bad MapID + position = spawn into the void → falling → death → respawn at the last grace

### Rotation (Quaternion)
- **Save offset**: PlayerCoordinates + 0x10 (4 × f32)
- **Editing**: Facing direction after load (minor)

---

## 15. Game State

### Death Count
- **Description**: Total deaths of the character
- **Save offset**: Game State section 7 + 0x00 (u32)
- **Editing**: Purely cosmetic — reset to 0 does not affect gameplay

### NG+ Cycle (ClearCount)
- **Description**: Journey number
- **Values**: 0=Journey 1 (first play), 1=NG+1, ..., 7=NG+7 (max)
- **Save offset**: Game State section 1 + 0x00 (u32)
- **Effect**:
  - Enemies scaling: NG+1 = 3–3.4× HP (early areas); NG+7 = ~1.45× over NG+1
  - Rune rewards: NG+1 = 5.5× (early); diminishing after NG+2
  - NG+7 = cap, further cycles identical
- **Caveat**: Changing from 0 to N does not reset quest flags/items — the character will be in Journey N+1 with the current progress

### Last Rested Grace
- **Description**: Grace entity ID (BonfireId) of the last rest
- **Save offset**: Game State section 7 + 0x10 (u32)
- **Effect**: Spawn point after death and after loading the save
- **Editing**: Changing = teleports the player to another grace!
- **Caveat**: The value must be a valid BonfireId (see spec/15 Bonfire IDs table)

### Play Time
- **Description**: Playtime in milliseconds
- **Save offset**: Separate section (IngameTimer)
- **Editing**: Cosmetic — changes the time displayed in the menu

---

## 16. Weather & Time

### WorldAreaTime
- **Hour**: 0–23 (u32)
- **Minute**: 0–59 (u32)
- **Second**: 0–59 (u32)
- **Save offset**: WorldAreaTime section (3 × u32 = 12B)
- **Effect**: Time of day after load. Affects: NPC spawns (Night's Cavalry, Bell Bearing Hunter), lighting, ambient.

### WorldAreaWeather
- **Area ID**: u16 — region identifier
- **Weather Type**: u16 — weather type
- **Timer**: u32 — duration
- **Save offset**: WorldAreaWeather section (12B)
- **Effect**: Weather at load time

---

## 17. DLC Flags

### Shadow of the Erdtree Entry Flag
- **Description**: Whether the character has entered the DLC (Realm of Shadow)
- **Values**: 0=not entered, 1=entered
- **Save offset**: DLC section + 0x01 (u8)
- **Effect**: One-way — once entered there is no way to undo it in-game. Editing allows a reset.
- **CRITICAL**: On platform conversion this byte MUST be zeroed if the DLC is not installed → infinite loading

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

## 19. Event Flags — most important editable ones

### Boss Defeat Flags
- **Effect**: Marks a boss as defeated. Opens new areas, NPC reactions.
- **Range**: 9100–9281 (global bosses), 10000000+ (field bosses)
- **Caveat**: Setting a boss flag without the questline → NPCs may be confused

### Grace Discovery Flags
- **Effect**: Discovering a Site of Grace — enables fast travel
- **Range**: 71xxx, 73xxx, 76xxx
- **Caveat**: Discovering a grace without defeating the boss blocking access = normal

### Map Visibility / Acquisition
- **Visibility (62xxx)**: Map visible (region texture displayed)
- **Acquired (63xxx)**: Map fragment "picked up" — should also add the item to inventory
- **Caveat**: Visibility without Acquired = map visible but fragment not in inventory (cosmetically OK)

### Mechanic Unlocks (60xxx)
- **Editing**: Unlocking mechanics without questlines (Torrent, crafting, etc.)
- **Safe**: The game does not check "how" the mechanic was unlocked

### Cookbook Flags (67xxx–68xxx)
- **Effect**: Unlocking crafting recipes
- **Alternative**: Adding the cookbook item to inventory (that is the same thing — owning the item = flag set)

---

## 20. Inventory & Storage — per-item fields

### Quantity
- **Description**: Quantity of stackable items (weapons/armor = always 1)
- **Range**: 1 – MaxInventory/MaxStorage (per item, from the database)
- **Save offset**: InventoryItem + 0x04 (u32)
- **Caveat**: Exceeding max = the game clamps to max on use

### GaItem Handle
- **Description**: Reference to the item instance in the GaItem Map
- **Format**: 0xTTPPCCCC (TT=type, PP=part, CCCC=counter)
- **Caveat**: The handle MUST exist in the GaItem Map. Orphaned handle = the item does not show up.

---

## 21. Face Data — editable groups

### Model IDs (changing hair/beard/brows without the creator)
- **Hair_Model_Id**: Hairstyle change (u32, values from game params)
- **Beard_Model_Id**: Beard change
- **Safe**: Model change does not affect stability

### Shape Parameters (creator sliders)
- **Range**: 0–255 (u8), 128 = neutral/center
- **Safe**: Any in-range value is valid

### Colors (RGBA)
- **Range**: 0–255 per channel
- **Safe**: Any combination is valid

### Body Scale
- **Description**: Body proportions (Head, Chest, Abdomen, Arms, Legs)
- **In-memory format**: float (1.0 = normal)
- **In-save format**: needs verification (may be u8 with 128=1.0)

---

## 22. Memory Slots & Spell System

### Memory Slot Count
- **Base**: 2 slots (everyone)
- **Memory Stones**: +8 (8 findable in-game)
- **Moon of Nokstella** (talisman): +2 (equipment, not a save field)
- **Max**: 12 slots
- **Tracking**: Memory Stones are key items in inventory. Owning = unlocked.

### Equipped Spells (14 slots)
- **Format**: SpellID (u32) + Quantity (u32), stride 8B
- **Editing**: Changing SpellID = immediate spell change
- **Caveat**: The game lets you equip max N slots (N = memory slots). But 14 slots physically exist.

---

## 23. Weapon & Spirit Ash Upgrade System

### Normal Weapons
- **Range**: +0 to +25
- **Materials**: Smithing Stones [1]–[8] + Ancient Dragon Smithing Stone (+25)
- **ID encoding**: baseID + upgrade_level (e.g., Uchigatana = 1000000, +5 = 1000005)
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
- **ID encoding**: baseID + upgrade_level (like weapons)

---

## 24. Summoning Pools & Colosseums

### Summoning Pools (Martyr Effigies)
- **Total**: ~165 in-game (base + DLC)
- **Tracking**: Event flags (10000000+ range)
- **Editing**: Set flag = activated

### Colosseums
- **Total**: 3 (Caelid=60350, Limgrave=60360, Royal/Leyndell=60370)
- **Editing**: Set event flag = unlocked
- **Added**: Patch 1.08 (December 2022)

---

## 25. UNKNOWN parameters (to investigate)

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
| PlayerGameData 0x180–0x1AF | 48 bytes trailing | SwordArt scaling? Beginning of correction stats? |

---

## Sources

- Elden Ring Wiki (Fextralife): https://eldenring.wiki.fextralife.com/
- Game8 Elden Ring: https://game8.co/games/Elden-Ring/
- Souls Modding Wiki: https://www.soulsmodding.com/
- Cheat Engine tables: ER_all-in-one_Hexinton_v3.10, ER_TGA_v1.9.0
- er-save-manager (Python parser): sequential field-order reference
- Community spreadsheets: event flag mappings, param tables
