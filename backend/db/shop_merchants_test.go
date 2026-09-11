package db

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestShopMerchants_RowsAreUniqueAndOrdered guards the authored Super Merchant
// mapping: no row may be claimed twice, ranges must be well-formed, and Enia's
// block must stay out of the browsable set.
func TestShopMerchants_RowsAreUniqueAndOrdered(t *testing.T) {
	owner := make(map[int32]string)
	seenName := make(map[string]bool)

	for _, m := range data.ShopMerchants {
		if m.Name == "" {
			t.Fatal("merchant with empty name")
		}
		if seenName[m.Name] {
			t.Errorf("duplicate merchant %q", m.Name)
		}
		seenName[m.Name] = true
		if m.Name == "Enia" {
			t.Errorf("Enia must not be browsable")
		}
		if len(m.Rows) == 0 {
			t.Errorf("merchant %q has no rows", m.Name)
		}
		for _, r := range m.Rows {
			if r.First > r.Last {
				t.Errorf("merchant %q: inverted range %d-%d", m.Name, r.First, r.Last)
			}
			for id := r.First; id <= r.Last; id++ {
				if prev, dup := owner[id]; dup {
					t.Errorf("row %d claimed by both %q and %q", id, prev, m.Name)
					continue
				}
				owner[id] = m.Name
			}
		}
	}

	// Documented row corrections must survive any future edit of the table.
	for id, want := range map[int32]string{
		100250: "Sorcerer Thops",
		100252: "Sorcerer Thops",
		100568: "Nomadic Merchant - East Limgrave",
		101896: "Twin Maiden Husks",
		100669: "Isolated Merchant - Weeping Peninsula",
		100713: "Nomadic Merchant - North Liurnia",
	} {
		if got := owner[id]; got != want {
			t.Errorf("row %d owner = %q, want %q", id, got, want)
		}
	}
}
