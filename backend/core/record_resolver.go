package core

import (
	"encoding/binary"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// InvUnarmedBaseID is the "Unarmed" placeholder weapon base itemID the game
// keeps as a technical inventory slot (row 110000 in EquipParamWeapon). It is
// a resolved technical placeholder, never an unknown item.
const InvUnarmedBaseID = uint32(0x0001ADB0)

// Naked-armor placeholder itemIDs. The game stores a "bare" body-part record
// for each armor slot when nothing is equipped. These map to EquipParamProtector
// rows 10000/10100/10200/10300 (armor prefix 0x10000000 | rowID):
//   - 0x10002710 → row 10000 → bare head
//   - 0x10002774 → row 10100 → bare body
//   - 0x100027D8 → row 10200 → bare arms
//   - 0x1000283C → row 10300 → bare legs
//
// They resolve as technical placeholders (not unknown items): the rows are real
// game data absent from the editable armor DB by design (nothing to equip/edit).
const (
	nakedHeadItemID = uint32(0x10002710)
	nakedBodyItemID = uint32(0x10002774)
	nakedArmsItemID = uint32(0x100027D8)
	nakedLegsItemID = uint32(0x1000283C)
)

// IdentityClass classifies how a record's identity is derived. It is orthogonal
// to Resolution — a record's class is known even when it fails DB resolution.
type IdentityClass string

const (
	IdentityInstanceBacked       IdentityClass = "instance_backed"       // weapon/armor/aow with a per-instance GaItem
	IdentityHandleEncoded        IdentityClass = "handle_encoded"        // goods/talismans (itemID derived from handle)
	IdentityStackableAmmo        IdentityClass = "stackable_ammo"        // arrows/bolts — weapon prefix, but stackable
	IdentityTechnicalPlaceholder IdentityClass = "technical_placeholder" // Unarmed, naked armor
	IdentityUnknown              IdentityClass = "unknown"               // unresolved
)

// Resolution is the terminal resolution status of a physical record. The three
// values form an exhaustive partition of every non-empty record:
//
//	TotalPhysical = KnownDB + TechnicalPlaceholder + Unknown
//
// "Resolved" in the coverage report means KnownDB + TechnicalPlaceholder. A
// record being Resolved says nothing about whether it is *valid* — that is the
// scanner's job (structural / category checks), reported separately as issues.
type Resolution string

const (
	ResolutionKnownDB              Resolution = "known_db"
	ResolutionTechnicalPlaceholder Resolution = "technical_placeholder"
	ResolutionUnknown              Resolution = "unknown"
)

// UnknownReason explains why a record ended up with ResolutionUnknown. It is its
// own type (not Resolution) because the two describe different axes: Resolution
// is the terminal status, UnknownReason is the cause behind an Unknown status.
// Each reason maps to a distinct repair code and must not be collapsed.
type UnknownReason string

const (
	UnknownReasonUnknownHandleType UnknownReason = "unknown_handle_type" // handle prefix is not a known GaItem type
	UnknownReasonMissingDBEntry    UnknownReason = "missing_db_entry"    // known prefix, but itemID absent from DB
)

// ResolvedRecord is the canonical, read-only view of one physical inventory or
// storage record, shared by the core scanner, the coverage report, and the
// editor workspace. It is produced once per record by the resolver so all three
// consumers see identical semantics.
type ResolvedRecord struct {
	Scope            string        // repairScope* — inventory_common / inventory_key / storage_common
	Row              int           // compacted/action row within the scope — index into slot.*.CommonItems, addressed by repair primitives
	PhysicalRow      int           // physical slot index in the raw binary array; differs from Row when storage has gaps (== Row for inventory / when raw data is unavailable)
	IndexDedup       bool          // participates in duplicate_acquisition_index dedup (inventory, not storage)
	Handle           uint32        // raw GaItemHandle as stored
	HandleType       uint32        // handle & GaHandleTypeMask
	ItemID           uint32        // raw itemID (GaMap, else db.HandleToItemID) — never rewritten
	DisplayID        uint32        // normalized display itemID (Wondrous Physick raw→display alias)
	BaseID           uint32        // DB base itemID (upgrade/infusion stripped) when resolved
	Name             string        // DB name when resolved; placeholder name for naked armor
	Category         string        // DB category when resolved
	Quantity         uint32        // record quantity as stored
	AcquisitionIndex uint32        // record acquisition/sort index as stored
	HasGaItem        bool          // a per-instance GaItem actually exists for this handle. slot.GaMap is built 1:1 from the non-empty slot.GaItems entries (structures.go scanGaItems), so GaMap membership is equivalent to GaItem existence — this is not a mere numeric coincidence.
	Identity         IdentityClass // instance-backed / handle-encoded / stackable ammo / placeholder / unknown
	Resolution       Resolution    // KnownDB / TechnicalPlaceholder / Unknown
	UnknownReason    UnknownReason // set only when Resolution == Unknown
	Fingerprint      string        // record-state fingerprint (handle+qty+index) for stale-checks
}

// isKnownHandlePrefix reports whether prefix is one of the five GaItem type
// prefixes the game uses. A record whose prefix is outside this set cannot be
// interpreted at all (unknown_handle_type).
func isKnownHandlePrefix(prefix uint32) bool {
	switch prefix {
	case ItemTypeWeapon, ItemTypeArmor, ItemTypeAccessory, ItemTypeItem, ItemTypeAow:
		return true
	}
	return false
}

// isNakedArmorID reports whether itemID is one of the four naked-armor
// placeholder rows.
func isNakedArmorID(itemID uint32) bool {
	switch itemID {
	case nakedHeadItemID, nakedBodyItemID, nakedArmsItemID, nakedLegsItemID:
		return true
	}
	return false
}

// nakedArmorName returns a stable display name for a naked-armor placeholder.
func nakedArmorName(itemID uint32) string {
	switch itemID {
	case nakedHeadItemID:
		return "Bare Head"
	case nakedBodyItemID:
		return "Bare Body"
	case nakedArmsItemID:
		return "Bare Arms"
	case nakedLegsItemID:
		return "Bare Legs"
	}
	return ""
}

// requiresGaItem reports whether a handle prefix denotes an instance-backed
// item type that must carry a per-instance GaItem (weapon / armor / aow).
// Goods and talismans are handle-encoded and legitimately have no GaItem.
func requiresGaItem(prefix uint32) bool {
	return prefix == ItemTypeWeapon || prefix == ItemTypeArmor || prefix == ItemTypeAow
}

// ResolveRecord resolves a single physical record into its canonical form.
// It never mutates the slot and never rewrites the raw itemID; DisplayID
// carries the normalized ID used for DB lookups (e.g. Wondrous Physick).
func ResolveRecord(slot *SaveSlot, scope string, row int, handle, qty, acq uint32) ResolvedRecord {
	rec := ResolvedRecord{
		Scope:            scope,
		Row:              row,
		PhysicalRow:      row, // default: physical == compacted; ResolveInventoryRecords overrides for gapped storage
		IndexDedup:       scope == repairScopeInventoryCommon || scope == repairScopeInventoryKey,
		Handle:           handle,
		HandleType:       handle & GaHandleTypeMask,
		Quantity:         qty,
		AcquisitionIndex: acq,
		Fingerprint:      fingerprintInventoryItem(InventoryItem{GaItemHandle: handle, Quantity: qty, Index: acq}),
	}

	// itemID resolution order: GaMap (instance items) first, then handle
	// encoding (goods/talismans). Read for DIAGNOSTICS ONLY at this point — a
	// GaMap entry (even one pointing at a naked-armor itemID) must never let a
	// malformed handle skip the prefix gate below. Raw itemID is preserved verbatim.
	if id, ok := slot.GaMap[handle]; ok {
		rec.ItemID = id
		rec.HasGaItem = true
	} else {
		rec.ItemID = db.HandleToItemID(handle)
	}
	rec.DisplayID = db.WondrousPhysickDisplayID(rec.ItemID)

	// Handle-prefix gate FIRST. An unknown handle prefix cannot be interpreted at
	// all and always wins over GaMap, the DB lookup and technical-placeholder
	// detection — even when the raw itemID collides numerically with a real DB
	// entry (e.g. handle 0x10009C40 has illegal prefix 0x10 but equals Iron
	// Helmet's itemID) or maps via GaMap to a naked-armor row. Emitting
	// unknown_handle_type here prevents a lucky numeric match from masking a
	// malformed handle.
	if !isKnownHandlePrefix(rec.HandleType) {
		rec.Identity = IdentityUnknown
		rec.Resolution = ResolutionUnknown
		rec.UnknownReason = UnknownReasonUnknownHandleType
		return rec
	}

	// Naked-armor placeholders resolve before the DB lookup — they are real
	// game rows absent from the editable armor DB. Only reachable once the prefix
	// is confirmed legal.
	if isNakedArmorID(rec.ItemID) {
		rec.Identity = IdentityTechnicalPlaceholder
		rec.Resolution = ResolutionTechnicalPlaceholder
		rec.Name = nakedArmorName(rec.ItemID)
		rec.BaseID = rec.ItemID
		return rec
	}

	// Identity class is derived from the handle itself (prefix + ammo test),
	// independent of DB resolution success. A weapon handle we cannot name in
	// the DB is still an instance-backed record — this is what makes a
	// repeated weapon handle a genuine duplicate even when its base weapon is
	// missing from the DB. (The prefix is already known-legal here.)
	switch {
	case db.IsArrowID(rec.ItemID) || db.IsArrowID(rec.DisplayID):
		rec.Identity = IdentityStackableAmmo
	case rec.HandleType == ItemTypeAccessory || rec.HandleType == ItemTypeItem:
		rec.Identity = IdentityHandleEncoded
	case requiresGaItem(rec.HandleType):
		rec.Identity = IdentityInstanceBacked
	default:
		rec.Identity = IdentityHandleEncoded
	}

	itemData, baseID := db.GetItemDataFuzzy(rec.DisplayID)
	rec.BaseID = baseID
	rec.Name = itemData.Name
	rec.Category = itemData.Category

	if itemData.Name == "" {
		rec.Resolution = ResolutionUnknown
		rec.UnknownReason = UnknownReasonMissingDBEntry
		return rec
	}

	// Unarmed placeholder — resolves in DB but is a technical slot, not editable.
	if baseID == InvUnarmedBaseID || itemData.Name == "Unarmed" {
		rec.Identity = IdentityTechnicalPlaceholder
		rec.Resolution = ResolutionTechnicalPlaceholder
		return rec
	}

	rec.Resolution = ResolutionKnownDB
	return rec
}

// ResolveInventoryRecords resolves every non-empty physical record across the
// inventory-common, key-item, and storage-common sections exactly once, in a
// stable order. Empty/invalid handles are skipped. This is the single source of
// record semantics for both BuildCoverageReport and the scanner.
func ResolveInventoryRecords(slot *SaveSlot) []ResolvedRecord {
	if slot == nil {
		return nil
	}
	storagePhysical := storagePhysicalRows(slot)
	sections := []struct {
		items []InventoryItem
		scope string
	}{
		{slot.Inventory.CommonItems, repairScopeInventoryCommon},
		{slot.Inventory.KeyItems, repairScopeInventoryKey},
		{slot.Storage.CommonItems, repairScopeStorageCommon},
	}
	var out []ResolvedRecord
	for _, sec := range sections {
		for row, item := range sec.items {
			h := item.GaItemHandle
			if h == GaHandleEmpty || h == GaHandleInvalid {
				continue
			}
			rec := ResolveRecord(slot, sec.scope, row, h, item.Quantity, item.Index)
			// Row stays the compacted/action row (repair primitives address it).
			// PhysicalRow is the true binary slot; only storage is compacted with
			// gaps, so only storage needs the remap.
			if sec.scope == repairScopeStorageCommon && row < len(storagePhysical) {
				rec.PhysicalRow = storagePhysical[row]
			}
			out = append(out, rec)
		}
	}
	return out
}

// storagePhysicalRows maps each compacted slot.Storage.CommonItems index to its
// physical slot index in the raw storage array. ReadStorage compacts the slice
// by dropping empty physical slots, so compacted row N is not physical slot N
// once the array has gaps — the same remap removeStorageRow performs when it
// applies an action.
//
// Fallback: when the raw storage bytes are unavailable (StorageBoxOffset <= 0 or
// slot.Data too short — e.g. synthetic test slots), physical == compacted, so
// the returned slice is the identity mapping.
func storagePhysicalRows(slot *SaveSlot) []int {
	n := len(slot.Storage.CommonItems)
	out := make([]int, n)
	for i := range out {
		out[i] = i // fallback: identity mapping
	}
	if slot.StorageBoxOffset <= 0 {
		return out
	}
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	compacted := 0
	for i := 0; i < StorageCommonCount && compacted < n; i++ {
		off := storageStart + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == GaHandleEmpty || h == GaHandleInvalid {
			continue
		}
		out[compacted] = i
		compacted++
	}
	return out
}

// ValidationCoverage is a measurable, issue-independent report of how many
// physical records the scanner read and to what depth they were checked.
//
// Invariants (Prompt 12):
//
//	TotalPhysical           = KnownDB + TechnicalPlaceholder + Unknown
//	Resolved                = KnownDB + TechnicalPlaceholder
//	ResolutionChecksApplied = TotalPhysical   (the resolver classified every
//	                                            physical record — this is what
//	                                            BuildCoverageReport itself did)
//	StructuralChecksApplied = 0 until the scanner runs; the pipeline fills it
//	                          with the count the structural scanner actually
//	                          processed (see ScanRepairIssuesWithCoverage). The
//	                          builder never claims structural checks it did not
//	                          execute.
//	CategoryChecksApplied   = 0               (no category validator exists yet;
//	                                            arrives in Prompt 13)
//
// PerCategory counts only records that resolved to a recognised DB category, so
// its values sum to KnownDB minus any KnownDB record with an empty category —
// it is NOT expected to sum to TotalPhysical.
type ValidationCoverage struct {
	TotalPhysical           int            `json:"totalPhysical"`
	Resolved                int            `json:"resolved"`
	KnownDB                 int            `json:"knownDB"`
	TechnicalPlaceholder    int            `json:"technicalPlaceholder"`
	Unknown                 int            `json:"unknown"`
	ResolutionChecksApplied int            `json:"resolutionChecksApplied"`
	StructuralChecksApplied int            `json:"structuralChecksApplied"`
	CategoryChecksApplied   int            `json:"categoryChecksApplied"`
	PerCategory             map[string]int `json:"perCategory"`
	UnknownByReason         map[string]int `json:"unknownByReason"`
}

// BuildCoverageReport tallies a resolved record collection into a coverage
// report. It reads only the already-resolved records — it never rescans the
// slot, so its semantics can never diverge from the scanner's.
//
// It reports ResolutionChecksApplied (work the resolver genuinely did) but
// leaves StructuralChecksApplied at 0: the structural scanner has not run at
// this point, and the builder must not claim checks it did not execute. The
// pipeline sets StructuralChecksApplied afterwards (ScanRepairIssuesWithCoverage).
func BuildCoverageReport(records []ResolvedRecord) ValidationCoverage {
	cov := ValidationCoverage{
		PerCategory:     map[string]int{},
		UnknownByReason: map[string]int{},
	}
	for _, r := range records {
		cov.TotalPhysical++
		switch r.Resolution {
		case ResolutionKnownDB:
			cov.KnownDB++
			if r.Category != "" {
				cov.PerCategory[r.Category]++
			}
		case ResolutionTechnicalPlaceholder:
			cov.TechnicalPlaceholder++
		case ResolutionUnknown:
			cov.Unknown++
			cov.UnknownByReason[string(r.UnknownReason)]++
		}
	}
	cov.Resolved = cov.KnownDB + cov.TechnicalPlaceholder
	// The resolver classified every physical record; that is a genuine, executed
	// check. Structural and category checks have not run here.
	cov.ResolutionChecksApplied = cov.TotalPhysical
	cov.StructuralChecksApplied = 0
	cov.CategoryChecksApplied = 0
	return cov
}
