package data

import "testing"

// Sword Lance is a greatsword/lance hybrid but is grouped under Heavy Thrusting
// Swords in this tool's subcategory scheme (see task: melee subcategory move).
func TestSwordLanceHeavyThrustingSword(t *testing.T) {
	if got := classifyMelee("Sword Lance"); got != SubcatMeleeHeavyThrustingSwords {
		t.Fatalf("Sword Lance: got %q, want %q", got, SubcatMeleeHeavyThrustingSwords)
	}
	// Flame Art variant strips its infusion and inherits the same class.
	if got := classifyMelee("Flame Art Sword Lance"); got != SubcatMeleeHeavyThrustingSwords {
		t.Fatalf("Flame Art Sword Lance: got %q, want %q", got, SubcatMeleeHeavyThrustingSwords)
	}
}
