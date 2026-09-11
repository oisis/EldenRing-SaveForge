package core

import (
	"os"
	"path/filepath"
	"testing"
)

// Super Merchant fixtures. Both default paths are optional local evidence, so
// every test skips when its fixture is absent.
func loadShopPCSave117(t *testing.T) *SaveFile {
	t.Helper()
	path := os.Getenv("ER_TEST_PC_SAVE_117")
	if path == "" {
		path = "../../tmp/save/ER0000-dlc-new-characters.sl2"
	}
	return loadShopFixture(t, path)
}

func loadShopFixture(t *testing.T, path string) *SaveFile {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("fixture not available: %s", path)
	}
	save, err := LoadSave(path)
	if err != nil {
		t.Fatalf("LoadSave %s: %v", path, err)
	}
	return save
}

func findShopRow(t *testing.T, rows []ShopRow, id int32) ShopRow {
	t.Helper()
	for _, r := range rows {
		if r.RowID == id {
			return r
		}
	}
	t.Fatalf("row %d not present in merchant rows", id)
	return ShopRow{}
}

func TestReadShopLineup_Regulation117(t *testing.T) {
	save := loadShopPCSave117(t)

	rows, err := ReadShopLineup(save.UserData11)
	if err != nil {
		t.Fatalf("ReadShopLineup: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no merchant rows decoded")
	}

	// Row 100000 — Gostoc, goods equipId 111, price 1000, finite stock 2.
	r := findShopRow(t, rows, 100000)
	if r.Merchant != "Gatekeeper Gostoc" {
		t.Errorf("row 100000 merchant = %q, want %q", r.Merchant, "Gatekeeper Gostoc")
	}
	if r.EquipType != 3 {
		t.Errorf("row 100000 equipType = %d, want 3", r.EquipType)
	}
	if r.ItemID != 0x40000000+111 {
		t.Errorf("row 100000 itemId = 0x%08X, want 0x%08X", r.ItemID, 0x40000000+111)
	}
	if r.Value != 1000 || r.SellQuantity != 2 {
		t.Errorf("row 100000 value/stock = %d/%d, want 1000/2", r.Value, r.SellQuantity)
	}
	if !r.Editable {
		t.Errorf("row 100000 should be editable (mtrlId == -1)")
	}

	// Row 101896 — a Regulation 1.17 retail addition on the Twin Maiden Husks.
	r = findShopRow(t, rows, 101896)
	if r.Merchant != "Twin Maiden Husks" {
		t.Errorf("row 101896 merchant = %q, want %q", r.Merchant, "Twin Maiden Husks")
	}
	if r.EquipType != 0 || r.ItemID != 64530000 {
		t.Errorf("row 101896 equipType/itemId = %d/%d, want 0/64530000", r.EquipType, r.ItemID)
	}

	// Enia is never browsable, so none of her rows may appear.
	for _, row := range rows {
		if row.RowID >= 101000 && row.RowID <= 101799 {
			t.Errorf("row %d (Enia block) must not be browsable", row.RowID)
		}
	}
}

func TestPatchShopLineup_SwapItemPriceAndStock_RoundTrip(t *testing.T) {
	save := loadShopPCSave117(t)

	const rowID = 100000
	const newItem = uint32(0x40000000 + 110) // another goods entry
	edit := ShopRowEdit{RowID: rowID, ItemID: newItem, Value: 4242, SellQuantity: 7}

	patched, err := PatchShopLineup(save.UserData11, []ShopRowEdit{edit})
	if err != nil {
		t.Fatalf("PatchShopLineup: %v", err)
	}
	if len(patched) != len(save.UserData11) {
		t.Fatalf("patched UserData11 length = %d, want %d", len(patched), len(save.UserData11))
	}

	// serialize -> reload -> decode again
	save.UserData11 = patched
	out := filepath.Join(t.TempDir(), "ER0000.sl2")
	if err := save.SaveFile(out); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	reloaded, err := LoadSave(out)
	if err != nil {
		t.Fatalf("LoadSave patched: %v", err)
	}
	rows, err := ReadShopLineup(reloaded.UserData11)
	if err != nil {
		t.Fatalf("ReadShopLineup after reload: %v", err)
	}

	r := findShopRow(t, rows, rowID)
	if r.ItemID != newItem {
		t.Errorf("reloaded itemId = 0x%08X, want 0x%08X", r.ItemID, newItem)
	}
	if r.Value != 4242 {
		t.Errorf("reloaded value = %d, want 4242", r.Value)
	}
	if r.SellQuantity != 7 {
		t.Errorf("reloaded stock = %d, want 7", r.SellQuantity)
	}

	// An unrelated merchant row must be untouched.
	other := findShopRow(t, rows, 100100)
	if other.Value != 5000 {
		t.Errorf("unrelated row 100100 value = %d, want 5000", other.Value)
	}
}

func TestPatchShopLineup_RejectsUnsupported(t *testing.T) {
	save := loadShopPCSave117(t)
	valid := ShopRowEdit{RowID: 100000, ItemID: 0x40000000 + 110, Value: 100, SellQuantity: -1}

	t.Run("unlimited stock is accepted", func(t *testing.T) {
		if _, err := PatchShopLineup(save.UserData11, []ShopRowEdit{valid}); err != nil {
			t.Fatalf("unexpected rejection: %v", err)
		}
	})

	cases := []struct {
		name string
		edit ShopRowEdit
	}{
		{"price above cap", ShopRowEdit{RowID: 100000, ItemID: valid.ItemID, Value: 1000000, SellQuantity: -1}},
		{"stock above byte range", ShopRowEdit{RowID: 100000, ItemID: valid.ItemID, Value: 100, SellQuantity: 256}},
		{"unknown item", ShopRowEdit{RowID: 100000, ItemID: 0x4000FFFF, Value: 100, SellQuantity: -1}},
		{"row outside merchant mapping", ShopRowEdit{RowID: 1, ItemID: valid.ItemID, Value: 100, SellQuantity: -1}},
		{"Enia row", ShopRowEdit{RowID: 101000, ItemID: valid.ItemID, Value: 100, SellQuantity: -1}},
		{"material-priced row", ShopRowEdit{RowID: 110000, ItemID: valid.ItemID, Value: 100, SellQuantity: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]byte(nil), save.UserData11...)
			if _, err := PatchShopLineup(save.UserData11, []ShopRowEdit{tc.edit}); err == nil {
				t.Fatal("expected rejection")
			}
			if string(before) != string(save.UserData11) {
				t.Fatal("rejected apply mutated the caller's UserData11")
			}
		})
	}
}

func TestPatchShopLineup_RejectsOlderRegulationAndPS4(t *testing.T) {
	edit := []ShopRowEdit{{RowID: 100000, ItemID: 0x40000000 + 110, Value: 100, SellQuantity: -1}}

	for _, tc := range []struct{ name, path string }{
		{"pre-1.17 PC regulation", "../../tmp/save/ER0000-oisisk_pl-vanilla.sl2"},
		{"PS4 save", "../../tmp/save/oisisk_ps4.dat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			save := loadShopFixture(t, tc.path)
			if _, err := ReadShopLineup(save.UserData11); err == nil {
				t.Error("ReadShopLineup: expected unsupported-regulation rejection")
			}
			if _, err := PatchShopLineup(save.UserData11, edit); err == nil {
				t.Error("PatchShopLineup: expected unsupported-regulation rejection")
			}
		})
	}
}
