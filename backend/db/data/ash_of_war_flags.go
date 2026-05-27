package data

// AoWItemToFlagID maps AoW inventory item ID → duplication event flag ID.
// When an AoW item is added to inventory, set the corresponding flag
// so the AoW appears in the Lost Grace duplication menu.
var AoWItemToFlagID = map[uint32]uint32{
	0x80002710: 65820, // Lion's Claw
	0x80002774: 65810, // Impaling Thrust
	0x800027D8: 65811, // Piercing Fang
	0x8000283C: 65812, // Spinning Slash
	0x80002904: 65833, // Charge Forth
	0x80002968: 65821, // Stamp (Upward Cut)
	0x800029CC: 65822, // Stamp (Sweep)
	0x80002A30: 65876, // Blood Tax
	0x80002A94: 65813, // Repeating Thrust
	0x80002AF8: 65823, // Wild Strikes
	0x80002B5C: 65836, // Spinning Strikes
	0x80002BC0: 65814, // Double Slash
	0x80002C24: 65844, // Prelate's Charge
	0x80002C88: 65815, // Unsheathe
	0x80002CEC: 65834, // Square Off
	0x80002D50: 65835, // Giant Hunt
	0x80002E18: 65852, // Loretta's Slash
	0x80002E7C: 65874, // Poison Moth Flight
	0x80002EE0: 65853, // Spinning Weapon
	0x80002FA8: 65837, // Storm Assault
	0x8000300C: 65838, // Stormcaller
	0x80003070: 65816, // Sword Dance
	0x80004E20: 65854, // Glintblade Phalanx
	0x80004E84: 65861, // Sacred Blade
	0x80004EE8: 65882, // Ice Spear
	0x80004F4C: 65855, // Glintstone Pebble
	0x80004FB0: 65877, // Bloody Slash
	0x80005014: 65878, // Lifesteal Fist
	0x800050DC: 65845, // Eruption
	0x80005140: 65862, // Prayerful Strike
	0x800051A4: 65856, // Gravitas
	0x80005208: 65839, // Storm Blade
	0x800052D0: 65824, // Earthshaker
	0x80005334: 65863, // Golden Land
	0x80005398: 65846, // Flaming Strike
	0x80005460: 65849, // Thunderbolt
	0x800054C4: 65850, // Lightning Slash
	0x80005528: 65857, // Carian Grandeur
	0x8000558C: 65858, // Carian Greatsword
	0x800055F0: 65840, // Vacuum Slice → Vacuum Strike
	0x80005654: 65847, // Black Flame Tornado
	0x800056B8: 65864, // Sacred Ring of Light
	0x80005780: 65879, // Blood Blade
	0x800057E4: 65870, // Phantom Slash
	0x80005848: 65871, // Spectral Lance
	0x800058AC: 65883, // Chilling Mist
	0x80005910: 65875, // Poisonous Mist → Poison Mist
	0x80007530: 65886, // Shield Bash
	0x80007594: 65888, // Barricade Shield
	0x800075F8: 65889, // Parry
	0x80007724: 65890, // Carian Retaliation
	0x80007788: 65891, // Storm Wall
	0x800077EC: 65892, // Golden Parry
	0x80007850: 65887, // Shield Crash
	0x800078B4: 65885, // No Skill
	0x80007918: 65893, // Thops's Barrier
	0x80009C40: 65899, // Through and Through
	0x80009CA4: 65896, // Barrage
	0x80009D08: 65897, // Mighty Shot
	0x80009DD0: 65900, // Enchanted Shot
	0x80009E34: 65898, // Sky Shot
	0x80009E98: 65901, // Rain of Arrows
	0x8000C3B4: 65884, // Hoarfrost Stomp
	0x8000C418: 65841, // Storm Stomp
	0x8000C47C: 65825, // Kick
	0x8000C4E0: 65851, // Lightning Ram
	0x8000C544: 65848, // Flame of the Redmanes
	0x8000C5A8: 65826, // Ground Slam
	0x8000C60C: 65865, // Golden Slam
	0x8000C670: 65859, // Waves of Darkness
	0x8000C6D4: 65827, // Hoarah Loux's Earthshaker
	0x8000EA60: 65842, // Determination
	0x8000EAC4: 65843, // Royal Knight's Resolve
	0x8000EB28: 65880, // Assassin's Gambit
	0x8000EB8C: 65866, // Golden Vow
	0x8000EBF0: 65867, // Sacred Order
	0x8000EC54: 65868, // Shared Order
	0x8000ECB8: 65881, // Seppuku
	0x8000ED1C: 65860, // Cragblade
	0x8000FDE8: 65828, // Barbaric Roar
	0x8000FE4C: 65829, // War Cry
	0x8000FEB0: 65869, // Beast's Roar
	0x8000FF14: 65830, // Troll's Roar
	0x8000FF78: 65831, // Braggart's Roar
	0x80011170: 65832, // Endure
	0x800111D4: 65895, // Vow of the Indomitable
	0x80011238: 65894, // Holy Ground
	0x80013880: 65818, // Quickstep
	0x800138E4: 65819, // Bloodhound's Step
	0x80013948: 65872, // Raptor of the Mists
	0x80014C08: 65873, // White Shadow's Lure
	0x80030D40: 65910, // Dryleaf Whirlwind
	0x80030DA4: 65911, // Aspects of the Crucible: Wings
	0x80061A80: 65912, // Spinning Gravity Thrust
	0x80061E68: 65913, // Palm Blast
	0x80062250: 65914, // Piercing Throw
	0x80062638: 65915, // Scattershot Throw
	0x80062A20: 65916, // Wall of Sparks
	0x80062E08: 65917, // Rolling Sparks
	0x800631F0: 65918, // Raging Beast
	0x800635D8: 65919, // Savage Claws
	0x80063DA8: 65920, // Blind Spot
	0x80064190: 65921, // Swift Slash
	0x80064578: 65922, // Overhead Stance
	0x80064960: 65923, // Wing Stance
	0x80064D48: 65924, // Blinkbolt
	0x80065130: 65925, // Flame Skewer
	0x80065518: 65926, // Savage Lion's Claw
	0x80065900: 65927, // Divine Beast Frost Stomp
	0x80065CE8: 65928, // Flame Spear
	0x800660D0: 65929, // Carian Sovereignty
	0x800664B8: 65930, // Shriek of Sorrow
	0x80067070: 65931, // Ghostflame Call
	0x8007B4A8: 65932, // The Poison Flower Blooms Twice
	0x80085CA0: 65933, // Igon's Drake Hunt
	0x800C3500: 65934, // Shield Strike
}
