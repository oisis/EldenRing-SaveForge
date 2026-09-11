package core

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// Super Merchant edits ShopLineupParam (and, when a price drops below an item's
// EquipParam*.sellValue, that single sellValue field) inside the regulation
// embedded in UserData11. It never touches the game installation or an external
// regulation.bin.
//
// Scope is deliberately narrow: PC saves carrying Regulation 1.17 only. Every
// other container or PARAM variant is rejected before any mutation.

const (
	shopLineupParamName = "ShopLineupParam.param"
	shopLineupParamType = "SHOP_LINEUP_PARAM"

	// Regulation 1.17 ShopLineupParam fingerprint, verified against PC save
	// fixtures. Earlier regulations ship 1277 (pre-DLC PC) or 1061 (base game)
	// rows, so the row count alone already separates 1.17 from its ancestors.
	shopLineupRowCount     = 1296
	shopLineupRowSize      = 52
	shopLineupFormatVer    = 7
	shopLineupDefDataVer   = 3
	shopLineupFormatFlags  = 0x85
	shopLineupMaxValue     = 999999
	shopLineupMaxSellQuant = 255
)

// Field offsets within one 52-byte SHOP_LINEUP_PARAM row.
const (
	shopOffEquipID      = 0  // s32
	shopOffValue        = 4  // s32
	shopOffMtrlID       = 8  // s32
	shopOffSellQuantity = 20 // s16
	shopOffEquipType    = 23 // u8
)

// PARAM header field offsets shared by every Elden Ring PARAM file.
const (
	paramOffDefDataVersion = 0x08 // u16
	paramOffRowCount       = 0x0A // u16
	paramOffParamTypeOff   = 0x10 // u64
	paramOffFormatFlags    = 0x2D // u8
	paramOffFormatVersion  = 0x2E // u8
	paramRowEntryOff       = 0x40
	paramRowEntrySize      = 24
)

// errUnsupportedShopRegulation is the single user-facing rejection for every
// container or PARAM shape Super Merchant does not support.
var errUnsupportedShopRegulation = fmt.Errorf(
	"unsupported regulation: Super Merchant currently supports PC saves with Regulation 1.17 only")

// equipTypeItemIDBase maps a ShopLineupParam equipType to the SaveForge item-ID
// prefix. The two numbering schemes already agree — see db.ItemIDToHandlePrefix,
// which derives inventory handle prefixes from the same four high bits.
var equipTypeItemIDBase = [5]uint32{
	0: 0x00000000, // weapon
	1: 0x10000000, // armor
	2: 0x20000000, // talisman
	3: 0x40000000, // goods (consumables, spells, key items)
	4: 0x80000000, // ash of war
}

// shopEquipParam describes the EquipParam table backing one equipType, limited
// to what a staged sellValue reduction needs. Row sizes and the sellValue offset
// were verified against the Regulation 1.17 PARAM headers in a PC save fixture.
type shopEquipParam struct {
	fileName        string
	paramType       string
	rowSize         int
	sellValueOffset int
}

var shopEquipParams = [5]shopEquipParam{
	0: {"EquipParamWeapon.param", "EQUIP_PARAM_WEAPON_ST", 664, 32},
	1: {"EquipParamProtector.param", "EQUIP_PARAM_PROTECTOR_ST", 416, 32},
	2: {"EquipParamAccessory.param", "EQUIP_PARAM_ACCESSORY_ST", 96, 24},
	3: {"EquipParamGoods.param", "EQUIP_PARAM_GOODS_ST", 176, 20},
	4: {"EquipParamGem.param", "EQUIP_PARAM_GEM_ST", 96, 32},
}

// ShopMerchantInfo is one browsable merchant and how many of its rows the
// loaded regulation actually exposes.
type ShopMerchantInfo struct {
	Name         string `json:"name"`
	RowCount     int    `json:"rowCount"`
	EditableRows int    `json:"editableRows"`
}

// ShopRow is one merchant-owned ShopLineupParam row as currently stored.
type ShopRow struct {
	RowID        int32  `json:"rowId"`
	Merchant     string `json:"merchant"`
	ItemID       uint32 `json:"itemId"`
	EquipType    int32  `json:"equipType"`
	Value        int32  `json:"value"`
	SellQuantity int32  `json:"sellQuantity"`
	Editable     bool   `json:"editable"`
	LockReason   string `json:"lockReason,omitempty"`
}

// ShopRowEdit is one requested row change. All three mutable fields are always
// supplied, so an unchanged field is simply written back with its current value.
type ShopRowEdit struct {
	RowID        int32  `json:"rowId"`
	ItemID       uint32 `json:"itemId"`
	Value        int32  `json:"value"`
	SellQuantity int32  `json:"sellQuantity"`
}

// ShopMerchantList returns the browsable merchants in display order. It reads
// only the authored mapping, never a save.
func ShopMerchantList() []string {
	names := make([]string, 0, len(data.ShopMerchants))
	for _, m := range data.ShopMerchants {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return names
}

// ReadShopLineup decodes every merchant-owned ShopLineupParam row from the
// regulation embedded in UserData11. It never mutates its input.
func ReadShopLineup(ud11 []byte) ([]ShopRow, error) {
	bnd4, _, _, err := openShopRegulation(ud11)
	if err != nil {
		return nil, err
	}
	tbl, err := openShopLineupTable(bnd4)
	if err != nil {
		return nil, err
	}
	return tbl.merchantRows(), nil
}

// PatchShopLineup applies every edit atomically and returns a new UserData11.
//
// All edits are validated and staged against an in-memory copy of the BND4
// archive first; the caller's UserData11 is never touched. After staging, the
// modified tables are decoded again and each written field is verified before
// the regulation is recompressed and re-encrypted. Any failure returns an error
// and no patched buffer, so the caller keeps its original save bytes.
func PatchShopLineup(ud11 []byte, edits []ShopRowEdit) ([]byte, error) {
	if len(edits) == 0 {
		return nil, fmt.Errorf("no shop changes requested")
	}

	bnd4, iv, dcxFormat, err := openShopRegulation(ud11)
	if err != nil {
		return nil, err
	}

	shop, err := openShopLineupTable(bnd4)
	if err != nil {
		return nil, err
	}

	// --- validate every edit before writing a single byte ---
	type stagedSellValue struct {
		tbl    *paramTable
		rowID  int32
		offset int
		value  int32
	}
	seen := make(map[int32]bool, len(edits))
	equipTables := make(map[byte]*paramTable)
	staged := make([]stagedSellValue, 0, len(edits))

	for _, e := range edits {
		if seen[e.RowID] {
			return nil, fmt.Errorf("row %d listed twice in one apply", e.RowID)
		}
		seen[e.RowID] = true

		row, ok := shop.rowByID[e.RowID]
		if !ok {
			return nil, fmt.Errorf("row %d does not exist in this regulation", e.RowID)
		}
		if !shop.editable[e.RowID] {
			return nil, fmt.Errorf("row %d is not an editable merchant row", e.RowID)
		}
		if mtrl := int32(binary.LittleEndian.Uint32(row[shopOffMtrlID:])); mtrl != -1 {
			return nil, fmt.Errorf("row %d is material-priced and cannot be edited", e.RowID)
		}
		if e.Value < 0 || e.Value > shopLineupMaxValue {
			return nil, fmt.Errorf("row %d: price must be between 0 and %d", e.RowID, shopLineupMaxValue)
		}
		if e.SellQuantity != -1 && (e.SellQuantity < 0 || e.SellQuantity > shopLineupMaxSellQuant) {
			return nil, fmt.Errorf("row %d: stock must be -1 (unlimited) or 0-%d", e.RowID, shopLineupMaxSellQuant)
		}

		equipType, equipID, err := itemIDToEquipRef(e.ItemID)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", e.RowID, err)
		}

		// The item must exist in the shipped item database. Unknown and
		// technical IDs are rejected rather than written blind.
		if !shopItemKnown(e.ItemID) {
			return nil, fmt.Errorf("row %d: item 0x%08X is not a known sellable item", e.RowID, e.ItemID)
		}

		def := shopEquipParams[equipType]
		eq, ok := equipTables[equipType]
		if !ok {
			eq, err = openEquipParamTable(bnd4, def)
			if err != nil {
				return nil, err
			}
			equipTables[equipType] = eq
		}
		eqRow, ok := eq.rowByID[equipID]
		if !ok {
			return nil, fmt.Errorf("row %d: item 0x%08X has no %s entry", e.RowID, e.ItemID, def.paramType)
		}

		// The game hides a shop entry whose price is below the item's base
		// sell value. Stage the matching reduction so the swap stays visible.
		sellValue := int32(binary.LittleEndian.Uint32(eqRow[def.sellValueOffset:]))
		if sellValue > 0 && sellValue > e.Value {
			staged = append(staged, stagedSellValue{
				tbl:    eq,
				rowID:  equipID,
				offset: def.sellValueOffset,
				value:  e.Value,
			})
		}
	}

	// --- stage every write ---
	for _, e := range edits {
		equipType, equipID, _ := itemIDToEquipRef(e.ItemID)
		row := shop.rowByID[e.RowID]
		binary.LittleEndian.PutUint32(row[shopOffEquipID:], uint32(equipID))
		binary.LittleEndian.PutUint32(row[shopOffValue:], uint32(e.Value))
		binary.LittleEndian.PutUint16(row[shopOffSellQuantity:], uint16(int16(e.SellQuantity)))
		row[shopOffEquipType] = equipType
	}
	for _, s := range staged {
		binary.LittleEndian.PutUint32(s.tbl.rowByID[s.rowID][s.offset:], uint32(s.value))
	}

	// --- decode again and verify every written field ---
	verify, err := openShopLineupTable(bnd4)
	if err != nil {
		return nil, fmt.Errorf("verify shop rows: %w", err)
	}
	for _, e := range edits {
		row, ok := verify.rowByID[e.RowID]
		if !ok {
			return nil, fmt.Errorf("verify row %d: row disappeared after patch", e.RowID)
		}
		wantType, wantEquipID, _ := itemIDToEquipRef(e.ItemID)
		if got := int32(binary.LittleEndian.Uint32(row[shopOffEquipID:])); got != wantEquipID {
			return nil, fmt.Errorf("verify row %d: equipId = %d, want %d", e.RowID, got, wantEquipID)
		}
		if got := row[shopOffEquipType]; got != wantType {
			return nil, fmt.Errorf("verify row %d: equipType = %d, want %d", e.RowID, got, wantType)
		}
		if got := int32(binary.LittleEndian.Uint32(row[shopOffValue:])); got != e.Value {
			return nil, fmt.Errorf("verify row %d: value = %d, want %d", e.RowID, got, e.Value)
		}
		if got := int32(int16(binary.LittleEndian.Uint16(row[shopOffSellQuantity:]))); got != e.SellQuantity {
			return nil, fmt.Errorf("verify row %d: sellQuantity = %d, want %d", e.RowID, got, e.SellQuantity)
		}
	}
	for _, s := range staged {
		eq, err := openEquipParamTable(bnd4, shopEquipParams[equipTypeOfTable(s.tbl)])
		if err != nil {
			return nil, fmt.Errorf("verify sell value: %w", err)
		}
		row, ok := eq.rowByID[s.rowID]
		if !ok {
			return nil, fmt.Errorf("verify sell value: %s row %d disappeared", eq.paramType, s.rowID)
		}
		if got := int32(binary.LittleEndian.Uint32(row[s.offset:])); got != s.value {
			return nil, fmt.Errorf("verify sell value: %s row %d = %d, want %d", eq.paramType, s.rowID, got, s.value)
		}
	}

	// --- repackage ---
	newRegBlob, err := compressDCX(bnd4, dcxFormat)
	if err != nil {
		return nil, fmt.Errorf("compress DCX: %w", err)
	}

	regStart := ud11RegulationOffset(ud11)
	originalCiphertextLen := len(ud11) - regStart - 16
	if len(newRegBlob) > originalCiphertextLen {
		return nil, fmt.Errorf("patched regulation blob (%d bytes) exceeds ciphertext capacity (%d bytes)",
			len(newRegBlob), originalCiphertextLen)
	}

	reencrypted, err := encryptRegulation(newRegBlob, iv, originalCiphertextLen)
	if err != nil {
		return nil, fmt.Errorf("encrypt regulation: %w", err)
	}

	result := make([]byte, len(ud11))
	copy(result, ud11)
	copy(result[regStart:], reencrypted)

	// PC saves carry a 16-byte MD5 prefix over the rest of UserData11; the game
	// rejects the regulation when it does not match.
	h := md5.Sum(result[ud11MD5Size:])
	copy(result[:ud11MD5Size], h[:])

	return result, nil
}

// --- internal ---

// openShopRegulation unpacks UserData11 down to a mutable BND4 copy and rejects
// every container Super Merchant does not support.
func openShopRegulation(ud11 []byte) (bnd4 []byte, iv []byte, dcxFormat string, err error) {
	if len(ud11) == 0 {
		return nil, nil, "", fmt.Errorf("save has no UserData11 (regulation)")
	}
	// PS4 saves start the regulation directly at the unk header (no MD5 prefix)
	// and are not supported by Super Merchant.
	if ud11RegulationOffset(ud11) != ud11MD5Size+ud11UnkSize {
		return nil, nil, "", errUnsupportedShopRegulation
	}

	regBlob, iv, dcxFormat, err := extractRegulation(ud11)
	if err != nil {
		return nil, nil, "", fmt.Errorf("extract regulation: %w", err)
	}
	bnd4, err = decompressDCX(regBlob, dcxFormat)
	if err != nil {
		return nil, nil, "", fmt.Errorf("decompress DCX: %w", err)
	}
	return bnd4, iv, dcxFormat, nil
}

// paramTable is a minimal row index over one PARAM file. Row slices alias the
// caller's BND4 buffer, so writing through them stages a change in place.
type paramTable struct {
	paramType string
	rowSize   int
	order     []int32
	rowByID   map[int32][]byte
	editable  map[int32]bool
	merchant  map[int32]string
}

func openShopLineupTable(bnd4 []byte) (*paramTable, error) {
	tbl, err := parseParamTable(bnd4, shopLineupParamName, shopLineupParamType, shopLineupRowSize)
	if err != nil {
		return nil, err
	}
	if len(tbl.order) != shopLineupRowCount {
		return nil, errUnsupportedShopRegulation
	}

	tbl.editable = make(map[int32]bool, shopLineupRowCount)
	tbl.merchant = make(map[int32]string, shopLineupRowCount)
	for _, m := range data.ShopMerchants {
		for _, r := range m.Rows {
			for id := r.First; id <= r.Last; id++ {
				row, ok := tbl.rowByID[id]
				if !ok {
					continue
				}
				tbl.merchant[id] = m.Name
				tbl.editable[id] = int32(binary.LittleEndian.Uint32(row[shopOffMtrlID:])) == -1
			}
		}
	}
	return tbl, nil
}

func openEquipParamTable(bnd4 []byte, def shopEquipParam) (*paramTable, error) {
	return parseParamTable(bnd4, def.fileName, def.paramType, def.rowSize)
}

// equipTypeOfTable resolves a staged EquipParam table back to its equipType so
// verification can re-open the same file. Unknown types cannot occur: staging
// only ever stores tables produced from shopEquipParams.
func equipTypeOfTable(tbl *paramTable) byte {
	for i, def := range shopEquipParams {
		if def.paramType == tbl.paramType {
			return byte(i)
		}
	}
	return 0
}

// parseParamTable reads the PARAM header and row index of one BND4 entry and
// fails closed on any shape other than the expected type, format and stride.
func parseParamTable(bnd4 []byte, fileName, wantType string, wantRowSize int) (*paramTable, error) {
	dataOff, size, err := findParamEntryInBND4(bnd4, fileName)
	if err != nil {
		return nil, err
	}
	p := bnd4[dataOff : dataOff+size]
	if len(p) < paramRowEntryOff+paramRowEntrySize {
		return nil, errUnsupportedShopRegulation
	}
	if p[paramOffFormatFlags] != shopLineupFormatFlags || p[paramOffFormatVersion] != shopLineupFormatVer {
		return nil, errUnsupportedShopRegulation
	}
	if wantType == shopLineupParamType &&
		binary.LittleEndian.Uint16(p[paramOffDefDataVersion:]) != shopLineupDefDataVer {
		return nil, errUnsupportedShopRegulation
	}

	typeOff := int(binary.LittleEndian.Uint64(p[paramOffParamTypeOff:]))
	if readParamCString(p, typeOff) != wantType {
		return nil, errUnsupportedShopRegulation
	}

	rowCount := int(binary.LittleEndian.Uint16(p[paramOffRowCount:]))
	if rowCount < 2 || paramRowEntryOff+rowCount*paramRowEntrySize > len(p) {
		return nil, errUnsupportedShopRegulation
	}

	tbl := &paramTable{
		paramType: wantType,
		rowSize:   wantRowSize,
		order:     make([]int32, 0, rowCount),
		rowByID:   make(map[int32][]byte, rowCount),
	}
	prevDataOff := -1
	for i := 0; i < rowCount; i++ {
		e := paramRowEntryOff + i*paramRowEntrySize
		id := int32(binary.LittleEndian.Uint32(p[e:]))
		off := int(binary.LittleEndian.Uint64(p[e+8 : e+16]))
		if off < 0 || off+wantRowSize > len(p) {
			return nil, errUnsupportedShopRegulation
		}
		// Rows are stored back to back; a differing stride means a different
		// PARAMDEF than the one these field offsets were verified against.
		if prevDataOff >= 0 && off-prevDataOff != wantRowSize {
			return nil, errUnsupportedShopRegulation
		}
		prevDataOff = off
		if _, dup := tbl.rowByID[id]; dup {
			return nil, errUnsupportedShopRegulation
		}
		tbl.order = append(tbl.order, id)
		tbl.rowByID[id] = p[off : off+wantRowSize : off+wantRowSize]
	}
	return tbl, nil
}

// merchantRows projects the mapped merchant rows in merchant/row order.
func (t *paramTable) merchantRows() []ShopRow {
	rows := make([]ShopRow, 0, len(t.merchant))
	for _, id := range t.order {
		name, ok := t.merchant[id]
		if !ok {
			continue
		}
		d := t.rowByID[id]
		equipType := d[shopOffEquipType]
		row := ShopRow{
			RowID:        id,
			Merchant:     name,
			EquipType:    int32(equipType),
			Value:        int32(binary.LittleEndian.Uint32(d[shopOffValue:])),
			SellQuantity: int32(int16(binary.LittleEndian.Uint16(d[shopOffSellQuantity:]))),
			Editable:     t.editable[id],
		}
		if int(equipType) < len(equipTypeItemIDBase) {
			row.ItemID = uint32(int32(binary.LittleEndian.Uint32(d[shopOffEquipID:]))) + equipTypeItemIDBase[equipType]
		} else {
			row.Editable = false
			row.LockReason = "unsupported item type"
		}
		if !row.Editable && row.LockReason == "" {
			row.LockReason = "material-priced row"
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Merchant != rows[j].Merchant {
			return rows[i].Merchant < rows[j].Merchant
		}
		return rows[i].RowID < rows[j].RowID
	})
	return rows
}

// itemIDToEquipRef splits a SaveForge item ID into the ShopLineupParam
// equipType and the EquipParam row ID it addresses.
func itemIDToEquipRef(itemID uint32) (equipType byte, equipID int32, err error) {
	base := itemID & 0xF0000000
	for i, b := range equipTypeItemIDBase {
		if b == base {
			return byte(i), int32(itemID & 0x0FFFFFFF), nil
		}
	}
	return 0, 0, fmt.Errorf("item 0x%08X has no shop item type", itemID)
}

// shopItemKnown reports whether the item database recognises the ID. Unknown
// and technical IDs are rejected instead of being written into a shop row.
func shopItemKnown(itemID uint32) bool {
	return db.GetItemData(itemID).Name != ""
}

func readParamCString(b []byte, off int) string {
	if off <= 0 || off >= len(b) {
		return ""
	}
	end := off
	for end < len(b) && b[end] != 0 {
		end++
	}
	return string(b[off:end])
}
