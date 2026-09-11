package application

import (
	"fmt"
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// Super Merchant (Advanced tab) reads and edits the merchant stock stored in the
// regulation embedded in the loaded save's UserData11.
//
// All version and platform gating lives in backend/core: this layer only wires
// the save lock, enriches rows with item-database metadata for display, and
// journals the apply lifecycle. It never touches the Network parameter path.

// ShopStockRow is one merchant row as presented to the renderer: the raw shop
// fields plus the item-database metadata the UI needs to draw it.
type ShopStockRow struct {
	RowID        int32    `json:"rowId"`
	ItemID       uint32   `json:"itemId"`
	ItemName     string   `json:"itemName"`
	IconPath     string   `json:"iconPath"`
	Category     string   `json:"category"`
	Flags        []string `json:"flags"`
	Value        int32    `json:"value"`
	SellQuantity int32    `json:"sellQuantity"`
	Editable     bool     `json:"editable"`
	LockReason   string   `json:"lockReason,omitempty"`
}

// GetShopMerchants lists the merchants whose stock the loaded save exposes.
//
// Read-only. Returns an error — not a partial list — when no save is loaded or
// the save's regulation is not a supported PC Regulation 1.17, so the renderer
// can present one unsupported state instead of an empty editor.
func (a *App) GetShopMerchants() ([]core.ShopMerchantInfo, error) {
	rows, err := a.readShopRows()
	if err != nil {
		return nil, err
	}

	order := make([]string, 0, 8)
	byName := make(map[string]*core.ShopMerchantInfo, 8)
	for _, r := range rows {
		info, ok := byName[r.Merchant]
		if !ok {
			order = append(order, r.Merchant)
			info = &core.ShopMerchantInfo{Name: r.Merchant}
			byName[r.Merchant] = info
		}
		info.RowCount++
		if r.Editable {
			info.EditableRows++
		}
	}

	out := make([]core.ShopMerchantInfo, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}

// GetShopStock returns one merchant's rows, enriched for display. Read-only.
func (a *App) GetShopStock(merchant string) ([]ShopStockRow, error) {
	if merchant == "" {
		return nil, fmt.Errorf("no merchant selected")
	}

	rows, err := a.readShopRows()
	if err != nil {
		return nil, err
	}

	out := make([]ShopStockRow, 0, 32)
	for _, r := range rows {
		if r.Merchant != merchant {
			continue
		}
		row := ShopStockRow{
			RowID:        r.RowID,
			ItemID:       r.ItemID,
			Value:        r.Value,
			SellQuantity: r.SellQuantity,
			Editable:     r.Editable,
			LockReason:   r.LockReason,
		}
		if entry := db.GetItemEntryByID(r.ItemID); entry != nil {
			row.ItemName = entry.Name
			row.IconPath = entry.IconPath
			row.Category = entry.Category
			row.Flags = entry.Flags
		} else {
			row.ItemName = "Unknown item 0x" + strconv.FormatUint(uint64(r.ItemID), 16)
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("merchant %q has no rows in this regulation", merchant)
	}
	return out, nil
}

// ApplyShopChanges writes every requested row change in one atomic operation.
//
// core.PatchShopLineup validates all edits, stages them together with any
// required EquipParam sellValue reduction, and re-decodes the patched tables
// before returning. a.save.UserData11 is reassigned only on full success, so a
// rejected apply leaves the in-memory save byte-identical. The patched save
// stays in memory until the standard WriteSave, which remains the sole owner of
// file backups.
func (a *App) ApplyShopChanges(edits []core.ShopRowEdit) error {
	a.journalLog(levelInfo, "super_merchant_requested", "super merchant apply requested",
		field("rows", strconv.Itoa(len(edits))))

	stage := "patch"
	err := func() error {
		a.saveMu.Lock()
		defer a.saveMu.Unlock()
		if a.save == nil {
			stage = "no_active_save"
			return fmt.Errorf("no save loaded")
		}
		patched, perr := core.PatchShopLineup(a.save.UserData11, edits)
		if perr != nil {
			return perr
		}
		a.save.UserData11 = patched
		return nil
	}()
	if err != nil {
		a.journalLog(levelError, "super_merchant_finished", "super merchant apply failed",
			field("outcome", "error"), field("stage", stage))
		return err
	}

	a.journalLog(levelInfo, "super_merchant_finished", "super merchant apply finished",
		field("outcome", "success"), field("rows", strconv.Itoa(len(edits))))
	return nil
}

// readShopRows is the shared read path for both getters. It takes the read lock
// only and never mutates the save.
func (a *App) readShopRows() ([]core.ShopRow, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	return core.ReadShopLineup(a.save.UserData11)
}
