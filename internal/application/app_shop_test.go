package application

import (
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func shopTestApp(t *testing.T, path string) *App {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("fixture not available: %s", path)
	}
	save, err := core.LoadSave(path)
	if err != nil {
		t.Fatalf("LoadSave %s: %v", path, err)
	}
	app := NewApp()
	app.save = save
	return app
}

// TestMain chdirs to the repository root, so fixture paths are repo-relative.
const (
	shopPC117Fixture = "tmp/save/ER0000-dlc-new-characters.sl2"
	shopPS4Fixture   = "tmp/save/oisisk_ps4.dat"
)

func TestShopMethods_NoSaveLoaded(t *testing.T) {
	app := NewApp()

	if _, err := app.GetShopMerchants(); err == nil {
		t.Error("GetShopMerchants: expected no-save error")
	}
	if _, err := app.GetShopStock("Merchant Kale"); err == nil {
		t.Error("GetShopStock: expected no-save error")
	}
	if err := app.ApplyShopChanges([]core.ShopRowEdit{{RowID: 100000, ItemID: 0x40000000 + 110, Value: 10, SellQuantity: -1}}); err == nil {
		t.Error("ApplyShopChanges: expected no-save error")
	}
}

func TestShopMethods_PCApplyUpdatesSaveInMemory(t *testing.T) {
	app := shopTestApp(t, shopPC117Fixture)

	merchants, err := app.GetShopMerchants()
	if err != nil {
		t.Fatalf("GetShopMerchants: %v", err)
	}
	if len(merchants) == 0 {
		t.Fatal("no merchants returned")
	}

	stock, err := app.GetShopStock("Gatekeeper Gostoc")
	if err != nil {
		t.Fatalf("GetShopStock: %v", err)
	}
	var before ShopStockRow
	for _, r := range stock {
		if r.RowID == 100000 {
			before = r
		}
	}
	if before.RowID != 100000 {
		t.Fatal("row 100000 missing from Gostoc stock")
	}
	if before.ItemName == "" {
		t.Error("row 100000 has no item name from the item database")
	}

	// A getter must not mutate the save.
	snapshot := append([]byte(nil), app.save.UserData11...)
	if _, err := app.GetShopStock("Gatekeeper Gostoc"); err != nil {
		t.Fatalf("GetShopStock (second call): %v", err)
	}
	if string(snapshot) != string(app.save.UserData11) {
		t.Fatal("getter mutated UserData11")
	}

	if err := app.ApplyShopChanges([]core.ShopRowEdit{{
		RowID: 100000, ItemID: before.ItemID, Value: 777, SellQuantity: 3,
	}}); err != nil {
		t.Fatalf("ApplyShopChanges: %v", err)
	}

	stock, err = app.GetShopStock("Gatekeeper Gostoc")
	if err != nil {
		t.Fatalf("GetShopStock after apply: %v", err)
	}
	for _, r := range stock {
		if r.RowID != 100000 {
			continue
		}
		if r.Value != 777 || r.SellQuantity != 3 {
			t.Errorf("row 100000 after apply = %d/%d, want 777/3", r.Value, r.SellQuantity)
		}
	}

	// A rejected apply must leave the in-memory save untouched.
	snapshot = append([]byte(nil), app.save.UserData11...)
	if err := app.ApplyShopChanges([]core.ShopRowEdit{{
		RowID: 100000, ItemID: before.ItemID, Value: 5000000, SellQuantity: 3,
	}}); err == nil {
		t.Error("expected rejection for an out-of-range price")
	}
	if string(snapshot) != string(app.save.UserData11) {
		t.Error("rejected apply mutated UserData11")
	}
}

func TestShopMethods_PS4IsRejected(t *testing.T) {
	app := shopTestApp(t, shopPS4Fixture)

	if _, err := app.GetShopMerchants(); err == nil {
		t.Error("GetShopMerchants: expected PS4 rejection")
	}
	if _, err := app.GetShopStock("Merchant Kale"); err == nil {
		t.Error("GetShopStock: expected PS4 rejection")
	}

	snapshot := append([]byte(nil), app.save.UserData11...)
	if err := app.ApplyShopChanges([]core.ShopRowEdit{{
		RowID: 100000, ItemID: 0x40000000 + 110, Value: 10, SellQuantity: -1,
	}}); err == nil {
		t.Error("ApplyShopChanges: expected PS4 rejection")
	}
	if string(snapshot) != string(app.save.UserData11) {
		t.Error("rejected PS4 apply mutated UserData11")
	}
}
