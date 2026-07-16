package data

// GestureEmptySentinel is the value used for empty gesture slots in the save file.
const GestureEmptySentinel = uint32(0xFFFFFFFE)

// GestureDef defines a single gesture. The ID is the canonical save-slot ID
// the game writes into the gesture array. All vanilla gesture IDs are odd —
// previous versions of this editor encoded an "EvenID/OddID body type" theory
// that turned out to be wrong (the game silently ignored even values, leaving
// edited gestures invisible in-game). Source: er-save-manager/data/gestures.py.
type GestureDef struct {
	Name     string
	Category string
	ID       uint32   // canonical gesture slot ID written to GestureGameData (always odd)
	ItemID   uint32   // matching key item ID in the inventory (0x4000xxxx / 0x401EA7xx)
	Flags    []string // "cut_content" | "pre_order" | "ban_risk" | "dlc_duplicate"
}

// AllGestures is the canonical list of all known gestures (51 base + 6 DLC).
// Cut-content entries (e.g. "The Carian Oath", "Fetal Position") are kept so
// existing saves that already contain them can be displayed correctly.
var AllGestures = []GestureDef{
	// Greetings
	{ID: 1, Name: "Bow", Category: "Greetings", ItemID: 0x40002328},
	{ID: 3, Name: "Polite Bow", Category: "Greetings", ItemID: 0x40002329},
	{ID: 5, Name: "My Thanks", Category: "Greetings", ItemID: 0x4000232A},
	{ID: 7, Name: "Curtsy", Category: "Greetings", ItemID: 0x4000232B},
	{ID: 9, Name: "Reverential Bow", Category: "Greetings", ItemID: 0x4000232C},
	{ID: 11, Name: "My Lord", Category: "Greetings", ItemID: 0x4000232D},
	{ID: 13, Name: "Warm Welcome", Category: "Greetings", ItemID: 0x4000232E},
	{ID: 15, Name: "Wave", Category: "Greetings", ItemID: 0x4000232F},
	{ID: 17, Name: "Casual Greeting", Category: "Greetings", ItemID: 0x40002330},
	{ID: 19, Name: "Strength!", Category: "Greetings", ItemID: 0x40002331},
	{ID: 21, Name: "As You Wish", Category: "Greetings", ItemID: 0x40002332},
	{ID: 229, Name: "Let Us Go Together", Category: "Greetings", ItemID: 0x401EA7AB}, // DLC
	// Gesturing
	{ID: 41, Name: "Point Forwards", Category: "Gesturing", ItemID: 0x40002333},
	{ID: 43, Name: "Point Upwards", Category: "Gesturing", ItemID: 0x40002334},
	{ID: 45, Name: "Point Downwards", Category: "Gesturing", ItemID: 0x40002335},
	{ID: 47, Name: "Beckon", Category: "Gesturing", ItemID: 0x40002336},
	{ID: 49, Name: "Wait!", Category: "Gesturing", ItemID: 0x40002337},
	{ID: 51, Name: "Calm Down!", Category: "Gesturing", ItemID: 0x40002338},
	{ID: 61, Name: "Nod In Thought", Category: "Gesturing", ItemID: 0x40002339},
	{ID: 225, Name: "The Two Fingers", Category: "Gesturing", ItemID: 0x401EA7AA}, // DLC
	// Submissive
	{ID: 81, Name: "Extreme Repentance", Category: "Submissive", ItemID: 0x4000233A},
	{ID: 83, Name: "Grovel For Mercy", Category: "Submissive", ItemID: 0x4000233B},
	// Battle
	{ID: 101, Name: "Rallying Cry", Category: "Battle", ItemID: 0x4000233C},
	{ID: 103, Name: "Heartening Cry", Category: "Battle", ItemID: 0x4000233D},
	{ID: 105, Name: "By My Sword", Category: "Battle", ItemID: 0x4000233E},
	{ID: 107, Name: "Hoslow's Oath", Category: "Battle", ItemID: 0x4000233F},
	{ID: 109, Name: "Fire Spur Me", Category: "Battle", ItemID: 0x40002340},
	{ID: 111, Name: "The Carian Oath", Category: "Battle", ItemID: 0x40002341, Flags: []string{"cut_content", "ban_risk"}},
	{ID: 223, Name: "May the Best Win", Category: "Battle", ItemID: 0x401EA7A9}, // DLC
	// Celebration
	{ID: 121, Name: "Bravo!", Category: "Celebration", ItemID: 0x40002342},
	{ID: 141, Name: "Jump for Joy", Category: "Celebration", ItemID: 0x40002343},
	{ID: 143, Name: "Triumphant Delight", Category: "Celebration", ItemID: 0x40002344},
	{ID: 145, Name: "Fancy Spin", Category: "Celebration", ItemID: 0x40002345},
	{ID: 147, Name: "Finger Snap", Category: "Celebration", ItemID: 0x40002346},
	// Emotion
	{ID: 161, Name: "Dejection", Category: "Emotion", ItemID: 0x40002347},
	{ID: 197, Name: "What Do You Want?", Category: "Emotion", ItemID: 0x40002350},
	// Resting
	{ID: 181, Name: "Patches' Crouch", Category: "Resting", ItemID: 0x40002348},
	{ID: 183, Name: "Crossed Legs", Category: "Resting", ItemID: 0x40002349},
	{ID: 185, Name: "Rest", Category: "Resting", ItemID: 0x4000234A},
	{ID: 187, Name: "Sitting Sideways", Category: "Resting", ItemID: 0x4000234B},
	{ID: 189, Name: "Dozing Cross-Legged", Category: "Resting", ItemID: 0x4000234C},
	{ID: 191, Name: "Spread Out", Category: "Resting", ItemID: 0x4000234D},
	{ID: 193, Name: "Fetal Position", Category: "Resting", ItemID: 0x4000234E, Flags: []string{"cut_content", "ban_risk"}},
	{ID: 195, Name: "Balled Up", Category: "Resting", ItemID: 0x4000234F},
	// Prayer
	{ID: 201, Name: "Prayer", Category: "Prayer", ItemID: 0x40002351},
	{ID: 203, Name: "Desperate Prayer", Category: "Prayer", ItemID: 0x40002352},
	{ID: 205, Name: "Rapture", Category: "Prayer", ItemID: 0x40002353},
	{ID: 207, Name: "Erudition", Category: "Prayer", ItemID: 0x40002355},
	{ID: 209, Name: "Outer Order", Category: "Prayer", ItemID: 0x40002356},
	{ID: 211, Name: "Inner Order", Category: "Prayer", ItemID: 0x40002357},
	{ID: 213, Name: "Golden Order Totality", Category: "Prayer", ItemID: 0x40002358},
	{ID: 231, Name: "O Mother", Category: "Prayer", ItemID: 0x401EA7AC}, // DLC
	// Special
	{ID: 217, Name: "The Ring (Pre-Order)", Category: "Special", ItemID: 0x40002359, Flags: []string{"pre_order", "ban_risk"}},
	{ID: 219, Name: "The Ring (Co-op)", Category: "Special", ItemID: 0x4000235A},
	{ID: 221, Name: "?GoodsName?", Category: "Special", ItemID: 0x40002354, Flags: []string{"cut_content", "ban_risk"}},
	{ID: 227, Name: "Ring of Miquella (Pre-Order)", Category: "Special", ItemID: 0x401EA7A8, Flags: []string{"pre_order", "ban_risk"}},
	{ID: 233, Name: "Ring of Miquella", Category: "Special", ItemID: 0x401EA7A8, Flags: []string{"dlc_duplicate", "ban_risk"}}, // alt slot for ID 227
}

// gestureByID is a reverse lookup: save slot ID → index in AllGestures.
var gestureByID map[uint32]int

func init() {
	gestureByID = make(map[uint32]int, len(AllGestures))
	for i, g := range AllGestures {
		gestureByID[g.ID] = i
	}
}

// LookupGestureBySlotID returns the gesture index and true if found, -1 and false otherwise.
func LookupGestureBySlotID(id uint32) (int, bool) {
	idx, ok := gestureByID[id]
	return idx, ok
}

// SanitizeGestureSlots rewrites the gesture array in-place: any entry that is
// even but (entry+1) is a known gesture is replaced with (entry+1). This
// repairs saves written by the previous editor build, which incorrectly used
// "even/odd body type" variants and produced gestures the game silently
// ignored. Returns the number of slots changed.
func SanitizeGestureSlots(slots []uint32) int {
	changed := 0
	for i, v := range slots {
		if v == GestureEmptySentinel || v == 0 {
			continue
		}
		if _, ok := gestureByID[v]; ok {
			continue // already a known canonical ID
		}
		if v%2 == 0 {
			if _, ok := gestureByID[v+1]; ok {
				slots[i] = v + 1
				changed++
			}
		}
	}
	return changed
}

// Gestures is the item database for gestures (for the item browser / DatabaseTab).
// These use ITEM IDs (0x4000xxxx), not gesture slot IDs.
var Gestures = map[uint32]ItemData{
	0x40002328: {Name: "Bow", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/bow.png"},
	0x40002329: {Name: "Polite Bow", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/polite_bow.png"},
	0x4000232A: {Name: "My Thanks", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/my_thanks.png"},
	0x4000232B: {Name: "Curtsy", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/curtsy.png"},
	0x4000232C: {Name: "Reverential Bow", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/reverential_bow.png"},
	0x4000232D: {Name: "My Lord", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/my_lord.png"},
	0x4000232E: {Name: "Warm Welcome", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/warm_welcome.png"},
	0x4000232F: {Name: "Wave", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/wave.png"},
	0x40002330: {Name: "Casual Greeting", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/casual_greeting.png"},
	0x40002331: {Name: "Strength!", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/strength.png"},
	0x40002332: {Name: "As You Wish", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/as_you_wish.png"},
	0x40002333: {Name: "Point Forwards", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/point_forwards.png"},
	0x40002334: {Name: "Point Upwards", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/point_upwards.png"},
	0x40002335: {Name: "Point Downwards", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/point_downwards.png"},
	0x40002336: {Name: "Beckon", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/beckon.png"},
	0x40002337: {Name: "Wait!", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/wait.png"},
	0x40002338: {Name: "Calm Down!", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/calm_down.png"},
	0x40002339: {Name: "Nod In Thought", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/nod_in_thought.png"},
	0x4000233A: {Name: "Extreme Repentance", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/extreme_repentance.png"},
	0x4000233B: {Name: "Grovel For Mercy", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/grovel_for_mercy.png"},
	0x4000233C: {Name: "Rallying Cry", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/rallying_cry.png"},
	0x4000233D: {Name: "Heartening Cry", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/heartening_cry.png"},
	0x4000233E: {Name: "By My Sword", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/by_my_sword.png"},
	0x4000233F: {Name: "Hoslow's Oath", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/hoslows_oath.png"},
	0x40002340: {Name: "Fire Spur Me", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/fire_spur_me.png"},
	0x40002341: {Name: "The Carian Oath", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/missing_icon.png"},
	0x40002342: {Name: "Bravo!", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/bravo.png"},
	0x40002343: {Name: "Jump for Joy", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/jump_for_joy.png"},
	0x40002344: {Name: "Triumphant Delight", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/triumphant_delight.png"},
	0x40002345: {Name: "Fancy Spin", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/fancy_spin.png"},
	0x40002346: {Name: "Finger Snap", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/finger_snap.png"},
	0x40002347: {Name: "Dejection", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/dejection.png"},
	0x40002348: {Name: "Patches' Crouch", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/patches_crouch.png"},
	0x40002349: {Name: "Crossed Legs", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/crossed_legs.png"},
	0x4000234A: {Name: "Rest", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/rest.png"},
	0x4000234B: {Name: "Sitting Sideways", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/sitting_sideways.png"},
	0x4000234C: {Name: "Dozing Cross-Legged", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/dozing_cross_legged.png"},
	0x4000234D: {Name: "Spread Out", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/spread_out.png"},
	0x4000234E: {Name: "Fetal Position", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/missing_icon.png"},
	0x4000234F: {Name: "Balled Up", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/balled_up.png"},
	0x40002350: {Name: "What Do You Want?", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/what_do_you_want.png"},
	0x40002351: {Name: "Prayer", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/prayer.png"},
	0x40002352: {Name: "Desperate Prayer", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/desperate_prayer.png"},
	0x40002353: {Name: "Rapture", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/rapture.png"},
	0x40002355: {Name: "Erudition", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/erudition.png"},
	0x40002356: {Name: "Outer Order", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/outer_order.png"},
	0x40002357: {Name: "Inner Order", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/inner_order.png"},
	0x40002358: {Name: "Golden Order Totality", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/golden_order_totality.png"},
	0x4000235A: {Name: "The Ring", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/the_ring.png"},
	0x401EA7A8: {Name: "Ring of Miquella", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/ring_of_miquella.png", Flags: []string{"dlc"}},
	0x401EA7A9: {Name: "May the Best Win", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/may_the_best_win.png", Flags: []string{"dlc"}},
	0x401EA7AA: {Name: "The Two Fingers", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/missing_icon.png", Flags: []string{"dlc"}},
	0x401EA7AB: {Name: "Let Us Go Together", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/let_us_go_together.png", Flags: []string{"dlc"}},
	0x401EA7AC: {Name: "O Mother", Category: "gestures", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/gestures/o_mother.png", Flags: []string{"dlc"}},
}
