package vm

import "testing"

// TestNormalizeItemQuantity pins the shared clamp contract used by both
// updateItemsAndSync and the Character diagnostic quantity planner: clamp to the
// per-section max, but only when that max is non-zero.
func TestNormalizeItemQuantity(t *testing.T) {
	cases := []struct {
		name      string
		item      ItemViewModel
		isStorage bool
		want      uint32
	}{
		{"inventory clamps to MaxInventory", ItemViewModel{Quantity: 500, MaxInventory: 99, MaxStorage: 600}, false, 99},
		{"storage clamps to MaxStorage", ItemViewModel{Quantity: 700, MaxInventory: 99, MaxStorage: 600}, true, 600},
		{"inventory zero limit means no clamp", ItemViewModel{Quantity: 500, MaxInventory: 0}, false, 500},
		{"storage zero limit means no clamp", ItemViewModel{Quantity: 700, MaxStorage: 0}, true, 700},
		{"under limit is untouched", ItemViewModel{Quantity: 3, MaxInventory: 99}, false, 3},
	}
	for _, c := range cases {
		if got := NormalizeItemQuantity(c.item, c.isStorage); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
