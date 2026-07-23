package core

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// Issue code constants — match the codes used in backend/editor/validate.go
// and in the UI so the frontend can use a single stable key for each problem.
const (
	RepairCodeDuplicateHandle      = "duplicate_handle"
	RepairCodeDuplicateUID         = "duplicate_uid"
	RepairCodeUnknownItemID        = "unknown_item_id"
	RepairCodeUnknownHandleType    = "unknown_handle_type"
	RepairCodeMissingGaItemMapping = "missing_gaitem_mapping"
	RepairCodeQuantityZero         = "quantity_zero"
	RepairCodeQuantityAboveMax     = "quantity_above_max"
	// RepairCodeItemNotAllowedInContainer is emitted instead of
	// quantity_above_max when the effective container cap is zero: the item is
	// not permitted in that container at all, which is a distinct defect from an
	// excessive-but-legal quantity. Splitting the codes keeps the clamp repair
	// from ever driving a quantity down to zero (which would manufacture a new
	// quantity_zero defect).
	RepairCodeItemNotAllowedInContainer = "item_not_allowed_in_container"
	// RepairCodePassThroughRecords is retained for JSON/action-map
	// compatibility but is no longer emitted as an aggregate issue —
	// pass-through is a write strategy, not a defect. Per-record resolution
	// status is reported via the coverage model instead.
	RepairCodePassThroughRecords        = "pass_through_records"
	RepairCodeCurrentAoWMissing         = "current_aow_missing"
	RepairCodeCurrentAoWShared          = "current_aow_shared"
	RepairCodeCurrentAoWNonAoWCategory  = "current_aow_non_aow_category"
	RepairCodeDuplicateAcquisitionIndex = "duplicate_acquisition_index"
	RepairCodeStatsFormula              = "stats_formula"
	// RepairCodeContainerOveruse is a REPORT-ONLY aggregate: the total quantity
	// of pot/aromatic craftables mapped to one container (e.g. all Throwing Pots
	// against Cracked Pot) exceeds the number of that container the slot owns.
	// Per-record caps cannot see this — each individual pot stack may sit within
	// the owned count while their sum overflows the shared container. It carries
	// no mutating action (no safe generic auto-repair; the user chooses what to
	// trim), so its default action is no_action.
	RepairCodeContainerOveruse = "container_overuse"
	// RepairCodeDuplicatePhysicalHandle flags two non-empty records in the
	// physical slot.GaItems table sharing one handle. This is a DISTINCT defect
	// from RepairCodeDuplicateHandle (duplicate Inventory/Storage container
	// records): here the collision is between physical GaItem records that may
	// carry different ItemIDs, which the structural integrity scan refuses. It is
	// REPORT-ONLY — no safe generic auto-repair exists (which ItemID to retain is
	// the user's call), so its only action is no_action.
	RepairCodeDuplicatePhysicalHandle = "duplicate_physical_gaitem_handle"
)

// Repair action identifiers — proposed by the scanner, executed by the apply endpoint.
const (
	RepairActionCreateCopy   = "create_copy"
	RepairActionRemoveRecord = "remove_record"
	RepairActionClearAoW     = "clear_aow"
	RepairActionPickAoW      = "pick_aow"
	RepairActionRepairIndex  = "repair_index"
	RepairActionFixLevel     = "fix_level"
	RepairActionNoAction     = "no_action"
	// RepairActionClampQuantity clamps an over-cap record down to its
	// authoritative effective cap (ClampInventoryQuantityAt).
	RepairActionClampQuantity = "clamp_quantity"
)

const (
	repairDomainInventory = "inventory"
	repairDomainAoW       = "aow"
	repairDomainStats     = "stats"
	repairDomainGaItem    = "gaitem"

	repairScopeGaItems = "gaitems"

	repairScopeInventoryCommon = "inventory_common"
	repairScopeInventoryKey    = "inventory_key"
	repairScopeStorageCommon   = "storage_common"
	repairScopeStats           = "stats"

	repairSeverityError   = "error"
	repairSeverityWarning = "warning"
	repairSeverityInfo    = "info"
)

// IssueKey uniquely and structurally identifies one repair issue.
// issueID = hex(SHA-256(canonical JSON of IssueKey)[:8]) — 16 hex chars.
// debugKey is the human-readable pipe-separated form for logs only; never parse it.
type IssueKey struct {
	Slot   int    `json:"slot"`
	Domain string `json:"domain"`
	Code   string `json:"code"`
	Scope  string `json:"scope"`
	Row    int    `json:"row"`
	Handle uint32 `json:"handle"`
	Field  string `json:"field,omitempty"`
	Value  string `json:"value,omitempty"`
}

// RepairIssue is one problem found by ScanRepairIssues.
// It is read-only — no slot mutation occurs during scanning.
type RepairIssue struct {
	IssueID       string   `json:"issueID"`
	DebugKey      string   `json:"debugKey"`
	Fingerprint   string   `json:"fingerprint"`
	Key           IssueKey `json:"key"`
	Description   string   `json:"description"`
	Severity      string   `json:"severity"`
	Actions       []string `json:"actions"`
	DefaultAction string   `json:"defaultAction"`
}

// ScanRepairIssues returns all repair issues found in slot. Read-only.
// It resolves the physical record collection once and delegates to
// ScanRepairIssuesFromRecords so coverage and scanning share identical
// record semantics.
func ScanRepairIssues(slotIndex int, slot *SaveSlot) []RepairIssue {
	return ScanRepairIssuesFromRecords(slotIndex, slot, ResolveInventoryRecords(slot))
}

// ScanRepairIssuesFromRecords scans a pre-resolved record collection. Callers
// that also build a coverage report should resolve once (ResolveInventoryRecords)
// and pass the same slice to both this function and BuildCoverageReport to
// guarantee the two never diverge. Read-only.
func ScanRepairIssuesFromRecords(slotIndex int, slot *SaveSlot, records []ResolvedRecord) []RepairIssue {
	issues, _, _ := scanRepairIssuesFrom(slotIndex, slot, records)
	return issues
}

// ScanRepairIssuesWithCoverage runs the full scan over a pre-resolved record
// collection and returns both the issues and a coverage report whose
// StructuralChecksApplied reflects the records the structural scanner actually
// processed — not a count the coverage builder assumed. This is the pipeline
// entry point: it guarantees structural coverage is only reported AFTER the
// scanner has run. Read-only.
func ScanRepairIssuesWithCoverage(slotIndex int, slot *SaveSlot, records []ResolvedRecord) ([]RepairIssue, ValidationCoverage) {
	cov := BuildCoverageReport(records) // ResolutionChecksApplied set; StructuralChecksApplied/CategoryChecksApplied == 0
	issues, structuralChecked, categoryChecked := scanRepairIssuesFrom(slotIndex, slot, records)
	cov.StructuralChecksApplied = structuralChecked
	cov.CategoryChecksApplied = categoryChecked
	return issues, cov
}

// scanRepairIssuesFrom is the shared scan core. It returns the issue list, the
// number of records the structural (inventory) scanner actually iterated, and
// the number of KnownDB records the container/quantity validator executed, so
// callers can report honest structural and category coverage.
func scanRepairIssuesFrom(slotIndex int, slot *SaveSlot, records []ResolvedRecord) ([]RepairIssue, int, int) {
	inv, structuralChecked, categoryChecked := scanInventoryRepairIssues(slotIndex, records)
	var out []RepairIssue
	out = append(out, inv...)
	out = append(out, scanAoWRepairIssues(slotIndex, slot)...)
	out = append(out, scanStatsRepairIssues(slotIndex, slot)...)
	out = append(out, scanPhysicalGaItemHandleIssues(slotIndex, slot)...)
	return out, structuralChecked, categoryChecked
}

// scanPhysicalGaItemHandleIssues reports duplicate handles in the physical
// slot.GaItems table. Two non-empty GaItem records sharing one handle is a real
// integrity defect — the structural integrity scan refuses it — and is DISTINCT
// from a duplicate Inventory/Storage container record (RepairCodeDuplicateHandle):
// the colliding records may carry different ItemIDs. REPORT-ONLY: no safe generic
// auto-repair exists, so the only action is no_action. Deterministic — records
// are scanned in ascending physical index, emitting one issue per repeated index.
func scanPhysicalGaItemHandleIssues(slotIndex int, slot *SaveSlot) []RepairIssue {
	firstIndex := make(map[uint32]int)
	var out []RepairIssue
	for i := range slot.GaItems {
		g := &slot.GaItems[i]
		if g.IsEmpty() {
			continue
		}
		h := g.Handle
		if first, seen := firstIndex[h]; seen {
			key := IssueKey{Slot: slotIndex, Domain: repairDomainGaItem, Code: RepairCodeDuplicatePhysicalHandle,
				Scope: repairScopeGaItems, Row: i, Handle: h,
				Field: "first_index", Value: fmt.Sprintf("%d", first)}
			out = append(out, mkIssue(key,
				fmt.Sprintf("GaItem[%d] reuses handle 0x%08X from GaItem[%d]", i, h, first),
				repairSeverityError,
				[]string{RepairActionNoAction},
				RepairActionNoAction, ""))
			continue
		}
		firstIndex[h] = i
	}
	return out
}

// ---- helpers ----------------------------------------------------------------

func repairIssueID(key IssueKey) string { return IssueKeyID(key) }

// IssueKeyID computes the stable issueID for a given IssueKey (hex SHA-256[:8]).
func IssueKeyID(key IssueKey) string {
	b, _ := json.Marshal(key)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

func repairDebugKey(key IssueKey) string {
	s := fmt.Sprintf("slot:%d|domain:%s|code:%s|scope:%s|row:%d|handle:0x%08X",
		key.Slot, key.Domain, key.Code, key.Scope, key.Row, key.Handle)
	if key.Field != "" {
		s += fmt.Sprintf("|field:%s|value:%s", key.Field, key.Value)
	}
	return s
}

// FingerprintRecordAt returns the fingerprint of the inventory/storage record
// currently at scope+row, so a repair apply endpoint can stale-check a target
// against the state captured at scan time before dispatching a primitive.
// ok=false when scope is unknown or row is out of range.
func FingerprintRecordAt(slot *SaveSlot, scope string, row int) (string, bool) {
	if slot == nil {
		return "", false
	}
	var list []InventoryItem
	switch scope {
	case repairScopeInventoryCommon:
		list = slot.Inventory.CommonItems
	case repairScopeInventoryKey:
		list = slot.Inventory.KeyItems
	case repairScopeStorageCommon:
		list = slot.Storage.CommonItems
	default:
		return "", false
	}
	if row < 0 || row >= len(list) {
		return "", false
	}
	return fingerprintInventoryItem(list[row]), true
}

func fingerprintInventoryItem(item InventoryItem) string {
	var b [12]byte
	binary.LittleEndian.PutUint32(b[0:], item.GaItemHandle)
	binary.LittleEndian.PutUint32(b[4:], item.Quantity)
	binary.LittleEndian.PutUint32(b[8:], item.Index)
	h := sha256.Sum256(b[:])
	return fmt.Sprintf("%x", h[:8])
}

func mkIssue(key IssueKey, desc, severity string, actions []string, def, fp string) RepairIssue {
	return RepairIssue{
		IssueID:       repairIssueID(key),
		DebugKey:      repairDebugKey(key),
		Fingerprint:   fp,
		Key:           key,
		Description:   desc,
		Severity:      severity,
		Actions:       actions,
		DefaultAction: def,
	}
}

// EffectiveQuantityCap returns the authoritative per-record quantity cap for a
// resolved record. It is the single source of quantity-cap semantics for both
// the scanner and the clamp repair primitive, so the two can never disagree.
//
// applies is false — and limit is 0 — for any record that carries no
// authoritative cap: a record that did not resolve to a DB entry (unknown /
// technical placeholder) or one in an unrecognised scope. Callers must not
// category-check or clamp such a record.
//
// For a KnownDB record, storage uses GameMaxStorage. Inventory uses
// GameMaxInventory EXCEPT for pot/aromatic craftables listed in
// data.RequiredContainer: the game caps those by the runtime container limit,
// not by their raw maxNum. For those, the inventory cap is how many of the
// required container the slot currently owns (containerOwned[containerID]), which
// is 0 when the container is missing. Storage is never container-capped.
//
// containerOwned maps containerItemID -> owned inventory quantity; pass nil when
// no record in scope is container-gated (the map lookup then yields 0). This
// deliberately does not use the conservative Normal Mode caps or scales_with_ng:
// editor policy and single-playthrough availability are not save-integrity truths.
//
// A known zero is a legitimate value (item not permitted in the container, or no
// container owned) and still returns applies=true.
func EffectiveQuantityCap(rec ResolvedRecord, containerOwned map[uint32]uint64) (limit uint64, applies bool) {
	if rec.Resolution != ResolutionKnownDB {
		return 0, false
	}
	switch rec.Scope {
	case repairScopeStorageCommon:
		if !rec.GameMaxStorageKnown {
			return 0, false
		}
		return uint64(rec.GameMaxStorage), true
	case repairScopeInventoryCommon, repairScopeInventoryKey:
		if containerID, ok := data.GetRequiredContainer(rec.DisplayID); ok {
			return containerOwned[containerID], true
		}
		if !rec.GameMaxInventoryKnown {
			return 0, false
		}
		return uint64(rec.GameMaxInventory), true
	default:
		return 0, false
	}
}

// containerOwnedQuantities totals the owned INVENTORY quantity of every container
// key item (Cracked Pot, Ritual Pot, Perfume Bottle, Hefty Cracked Pot) present
// in the resolved records. Storage copies do not count toward carrying capacity.
// The result feeds EffectiveQuantityCap so pot/aromatic caps track the container
// count the character actually holds.
func containerOwnedQuantities(records []ResolvedRecord) map[uint32]uint64 {
	owned := make(map[uint32]uint64)
	for _, r := range records {
		if r.Scope == repairScopeStorageCommon {
			continue
		}
		if data.IsContainerItem(r.DisplayID) {
			owned[r.DisplayID] += uint64(r.Quantity & 0x7FFFFFFF)
		}
	}
	return owned
}

// containerUsedQuantities totals the owned INVENTORY quantity of all craftables
// mapped to each container. Compared against containerOwnedQuantities it reveals
// aggregate container overuse (RepairCodeContainerOveruse) that per-record caps
// cannot detect when several pot types share one container.
func containerUsedQuantities(records []ResolvedRecord) map[uint32]uint64 {
	used := make(map[uint32]uint64)
	for _, r := range records {
		if r.Scope == repairScopeStorageCommon {
			continue
		}
		if containerID, ok := data.GetRequiredContainer(r.DisplayID); ok {
			used[containerID] += uint64(r.Quantity & 0x7FFFFFFF)
		}
	}
	return used
}

// ---- inventory scanner ------------------------------------------------------

// scanInventoryRepairIssues emits per-record issues from a pre-resolved record
// collection. Resolution status (known DB / technical placeholder / unknown) is
// authoritative here — the scanner never re-derives item identity, guaranteeing
// coverage and issues agree. Pass-through is no longer an aggregate issue.
func scanInventoryRepairIssues(slotIndex int, records []ResolvedRecord) ([]RepairIssue, int, int) {
	var out []RepairIssue
	seenHandles := make(map[uint32]bool)
	seenBuckets := make(map[uint32]bool) // shared across index-dedup scopes only

	// Container ownership is resolved once for the whole slot: pot/aromatic caps
	// depend on how many of the required container the character owns, not on the
	// individual record. Shared verbatim with the clamp primitive via
	// EffectiveQuantityCap so scanner and repair can never disagree.
	containerOwned := containerOwnedQuantities(records)

	structuralChecked := 0
	categoryChecked := 0
	for _, r := range records {
		structuralChecked++ // every physical record is subjected to the structural rules below
		h := r.Handle

		// Unknown resolution — a distinct problem per reason. Unknown handle
		// type and missing DB entry are NOT the same defect, so they carry
		// separate codes. Both are read-only (no mutating default action).
		switch r.Resolution {
		case ResolutionUnknown:
			switch r.UnknownReason {
			case UnknownReasonUnknownHandleType:
				key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeUnknownHandleType,
					Scope: r.Scope, Row: r.Row, Handle: h}
				out = append(out, mkIssue(key,
					fmt.Sprintf("handle 0x%08X has unrecognised type prefix 0x%08X", h, r.HandleType),
					repairSeverityWarning,
					[]string{RepairActionNoAction},
					RepairActionNoAction, r.Fingerprint))
			default: // UnknownReasonMissingDBEntry
				key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeUnknownItemID,
					Scope: r.Scope, Row: r.Row, Handle: h}
				out = append(out, mkIssue(key,
					fmt.Sprintf("handle 0x%08X (itemID 0x%08X) not found in item DB", h, r.ItemID),
					repairSeverityError,
					[]string{RepairActionNoAction},
					RepairActionNoAction, r.Fingerprint))
			}
		default:
			// Instance-backed items require a per-instance GaItem. A resolved
			// weapon/armor/aow record with no GaMap entry is a distinct defect.
			if r.Identity == IdentityInstanceBacked && !r.HasGaItem {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeMissingGaItemMapping,
					Scope: r.Scope, Row: r.Row, Handle: h}
				out = append(out, mkIssue(key,
					fmt.Sprintf("instance-backed handle 0x%08X (itemID 0x%08X) has no GaItem mapping", h, r.ItemID),
					repairSeverityError,
					[]string{RepairActionNoAction},
					RepairActionNoAction, r.Fingerprint))
			}
		}

		// Duplicate handle — only instance-backed records are per-instance
		// unique. Handle-encoded goods/talismans and stackable ammo share
		// handles by design; technical placeholders are exempt too.
		if r.Identity == IdentityInstanceBacked {
			if seenHandles[h] {
				keyH := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeDuplicateHandle,
					Scope: r.Scope, Row: r.Row, Handle: h}
				out = append(out, mkIssue(keyH,
					fmt.Sprintf("handle 0x%08X appears more than once", h),
					repairSeverityError,
					[]string{RepairActionCreateCopy},
					RepairActionCreateCopy, r.Fingerprint))
			}
			seenHandles[h] = true
		}

		if r.Quantity&0x7FFFFFFF == 0 {
			key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeQuantityZero,
				Scope: r.Scope, Row: r.Row, Handle: h}
			out = append(out, mkIssue(key,
				fmt.Sprintf("item 0x%08X has quantity 0", h),
				repairSeverityError,
				[]string{RepairActionRemoveRecord},
				RepairActionRemoveRecord, r.Fingerprint))
		}

		// A low acquisition index is NOT corruption: genuine game-created
		// records legitimately use Index <= InvEquipReservedMax (e.g. Memory of
		// Grace at 432, Lordsworn weapons/shields). InvEquipReservedMax is a
		// conservative floor for newly generated editor indices only, not a
		// validation rule for existing records.
		//
		// Elden Ring keys "Order of Acquisition" by Index>>1 (spec 52), so two
		// records sharing a BUCKET (Index>>1) collide in-game even when their raw
		// indices differ by one (e.g. 670/671). Dedup on the bucket, not the raw
		// index, to match ScanDuplicateInventoryIndices and the Integrity Modal —
		// repair_index (AssignFreshInventoryIndex) moves the later record to a
		// fresh, bucket-unique index.
		if r.IndexDedup && r.AcquisitionIndex > 0 {
			bucket := r.AcquisitionIndex >> 1
			if seenBuckets[bucket] {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeDuplicateAcquisitionIndex,
					Scope: r.Scope, Row: r.Row, Handle: h,
					Field: "bucket", Value: fmt.Sprintf("%d", bucket)}
				out = append(out, mkIssue(key,
					fmt.Sprintf("item 0x%08X (index %d) shares acquisition-order bucket %d", h, r.AcquisitionIndex, bucket),
					repairSeverityError,
					[]string{RepairActionRepairIndex},
					RepairActionRepairIndex, r.Fingerprint))
			}
			seenBuckets[bucket] = true
		}

		// Category/container quantity rule — runs ONLY for records that carry an
		// authoritative cap (KnownDB in a known scope). Unknown records and
		// technical placeholders carry no cap, so guessing a limit for them would
		// fabricate a validation result. EffectiveQuantityCap is the single source
		// of cap semantics, shared verbatim with the clamp repair primitive.
		if limit, applies := EffectiveQuantityCap(r, containerOwned); applies {
			categoryChecked++
			// Preserve the high-bit quantity flag semantics by masking before the
			// comparison.
			eff := uint64(r.Quantity & 0x7FFFFFFF)
			if eff > limit {
				if limit == 0 {
					// A zero cap means the item is not permitted in this container
					// at all — a distinct defect from an excessive quantity. It is
					// removable, never clampable (clamping to 0 would create a
					// quantity_zero defect).
					key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeItemNotAllowedInContainer,
						Scope: r.Scope, Row: r.Row, Handle: h,
						Field: "quantity", Value: fmt.Sprintf("%d", eff)}
					// Core uses no_action as its conservative non-mutating default
					// so a direct core consumer never receives a destructive
					// default; the App DTO layer re-maps this to the user-facing
					// leave_unchanged action (see repairActionsForCode).
					out = append(out, mkIssue(key,
						fmt.Sprintf("item 0x%08X is not permitted in %s", h, r.Scope),
						repairSeverityWarning,
						[]string{RepairActionRemoveRecord, RepairActionNoAction},
						RepairActionNoAction, r.Fingerprint))
				} else {
					key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeQuantityAboveMax,
						Scope: r.Scope, Row: r.Row, Handle: h,
						Field: "quantity", Value: fmt.Sprintf("%d", eff)}
					out = append(out, mkIssue(key,
						fmt.Sprintf("item 0x%08X quantity %d exceeds %s max %d", h, eff, r.Scope, limit),
						repairSeverityWarning,
						[]string{RepairActionClampQuantity},
						RepairActionClampQuantity, r.Fingerprint))
				}
			}
		}
	}

	// Aggregate container overuse — REPORT ONLY. Emitted once per over-subscribed
	// container, in ascending container-ID order for deterministic output. This is
	// the true game rule (total pot units mapped to a container must not exceed the
	// owned container count); per-record caps only catch a single stack exceeding
	// the owned count, missing the case where several pot types individually fit
	// but collectively overflow. No mutating action is offered — the user decides
	// which stacks to trim — so it never manufactures a destructive default.
	out = append(out, aggregateContainerIssues(slotIndex, records)...)

	return out, structuralChecked, categoryChecked
}

// aggregateContainerIssues builds the report-only container_overuse issues for a
// resolved record collection. Deterministic (sorted by container ID).
func aggregateContainerIssues(slotIndex int, records []ResolvedRecord) []RepairIssue {
	owned := containerOwnedQuantities(records)
	used := containerUsedQuantities(records)
	containers := make([]uint32, 0, len(used))
	for cID := range used {
		containers = append(containers, cID)
	}
	sort.Slice(containers, func(i, j int) bool { return containers[i] < containers[j] })

	var out []RepairIssue
	for _, cID := range containers {
		have := owned[cID]
		if used[cID] <= have {
			continue
		}
		key := IssueKey{Slot: slotIndex, Domain: repairDomainInventory, Code: RepairCodeContainerOveruse,
			Scope: repairScopeInventoryCommon, Row: -1, Handle: cID,
			Field: "container", Value: fmt.Sprintf("%d/%d", used[cID], have)}
		out = append(out, mkIssue(key,
			fmt.Sprintf("container 0x%08X holds %d pot/aromatic units but only %d container(s) are owned", cID, used[cID], have),
			repairSeverityWarning,
			[]string{RepairActionNoAction},
			RepairActionNoAction, ""))
	}
	return out
}

// ---- AoW scanner ------------------------------------------------------------

func scanAoWRepairIssues(slotIndex int, slot *SaveSlot) []RepairIssue {
	// Replicate editor.buildWeaponAoWMaps without importing that package.
	weaponAoWRefs := make(map[uint32]uint32) // weaponHandle → aowHandle
	aowRefCount := make(map[uint32]int)      // aowHandle → reference count

	for _, g := range slot.GaItems {
		if g.IsEmpty() {
			continue
		}
		if g.Handle&GaHandleTypeMask != ItemTypeWeapon {
			continue
		}
		if IsNoCustomAoWHandle(g.AoWGaItemHandle) {
			continue
		}
		weaponAoWRefs[g.Handle] = g.AoWGaItemHandle
		aowRefCount[g.AoWGaItemHandle]++
	}

	type scopedSection struct {
		items []InventoryItem
		scope string
	}
	sections := []scopedSection{
		{slot.Inventory.CommonItems, repairScopeInventoryCommon},
		{slot.Storage.CommonItems, repairScopeStorageCommon},
	}

	var out []RepairIssue
	for _, sec := range sections {
		for row, item := range sec.items {
			h := item.GaItemHandle
			if h == GaHandleEmpty || h == GaHandleInvalid {
				continue
			}
			if h&GaHandleTypeMask != ItemTypeWeapon {
				continue
			}
			aowHandle, hasRef := weaponAoWRefs[h]
			if !hasRef {
				continue
			}
			fp := fingerprintInventoryItem(item)

			// current_aow_missing: AoW handle not in GaMap (matches populateCurrentAoW).
			aowItemID, mapped := slot.GaMap[aowHandle]
			if !mapped || aowItemID == 0 {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainAoW, Code: RepairCodeCurrentAoWMissing,
					Scope: sec.scope, Row: row, Handle: h,
					Field: "aow", Value: fmt.Sprintf("0x%08X", aowHandle)}
				out = append(out, mkIssue(key,
					fmt.Sprintf("weapon 0x%08X references AoW 0x%08X which has no GaItem", h, aowHandle),
					repairSeverityWarning,
					[]string{RepairActionClearAoW, RepairActionPickAoW},
					RepairActionClearAoW, fp))
				continue
			}

			// current_aow_shared: same AoW handle used by more than one weapon.
			if aowRefCount[aowHandle] > 1 {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainAoW, Code: RepairCodeCurrentAoWShared,
					Scope: sec.scope, Row: row, Handle: h,
					Field: "aow", Value: fmt.Sprintf("0x%08X", aowHandle)}
				out = append(out, mkIssue(key,
					fmt.Sprintf("weapon 0x%08X shares AoW 0x%08X with %d other weapon(s)", h, aowHandle, aowRefCount[aowHandle]-1),
					repairSeverityWarning,
					[]string{RepairActionCreateCopy, RepairActionClearAoW},
					RepairActionCreateCopy, fp))
				continue
			}

			// current_aow_non_aow_category: AoW itemID resolves in DB but category
			// is not "ashes_of_war". Matches editor.Validate / validate.go check.
			aowData, _ := db.GetItemDataFuzzy(aowItemID)
			if aowData.Name != "" && aowData.Category != "ashes_of_war" {
				key := IssueKey{Slot: slotIndex, Domain: repairDomainAoW, Code: RepairCodeCurrentAoWNonAoWCategory,
					Scope: sec.scope, Row: row, Handle: h,
					Field: "aow", Value: fmt.Sprintf("0x%08X", aowHandle)}
				out = append(out, mkIssue(key,
					fmt.Sprintf("weapon 0x%08X AoW 0x%08X is %q (category %q, not ashes_of_war)",
						h, aowHandle, aowData.Name, aowData.Category),
					repairSeverityWarning,
					[]string{RepairActionClearAoW},
					RepairActionClearAoW, fp))
			}
		}
	}
	return out
}

// ---- stats scanner ----------------------------------------------------------

func scanStatsRepairIssues(slotIndex int, slot *SaveSlot) []RepairIssue {
	if slot.Player.Level == 0 {
		return nil
	}
	attrSum := slot.Player.Vigor + slot.Player.Mind + slot.Player.Endurance +
		slot.Player.Strength + slot.Player.Dexterity + slot.Player.Intelligence +
		slot.Player.Faith + slot.Player.Arcane
	expected := attrSum - 79
	if slot.Player.Level == expected {
		return nil
	}

	// Fingerprint covers Level + all 8 attributes — expectedLevel depends on attrs.
	var b [36]byte
	binary.LittleEndian.PutUint32(b[0:], slot.Player.Level)
	binary.LittleEndian.PutUint32(b[4:], slot.Player.Vigor)
	binary.LittleEndian.PutUint32(b[8:], slot.Player.Mind)
	binary.LittleEndian.PutUint32(b[12:], slot.Player.Endurance)
	binary.LittleEndian.PutUint32(b[16:], slot.Player.Strength)
	binary.LittleEndian.PutUint32(b[20:], slot.Player.Dexterity)
	binary.LittleEndian.PutUint32(b[24:], slot.Player.Intelligence)
	binary.LittleEndian.PutUint32(b[28:], slot.Player.Faith)
	binary.LittleEndian.PutUint32(b[32:], slot.Player.Arcane)
	h := sha256.Sum256(b[:])
	fp := fmt.Sprintf("%x", h[:8])

	key := IssueKey{Slot: slotIndex, Domain: repairDomainStats, Code: RepairCodeStatsFormula,
		Scope: repairScopeStats, Field: "level", Value: fmt.Sprintf("%d", slot.Player.Level)}
	return []RepairIssue{mkIssue(key,
		fmt.Sprintf("Level %d ≠ expected %d (sum(attrs)=%d − 79)", slot.Player.Level, expected, attrSum),
		repairSeverityWarning,
		[]string{RepairActionFixLevel},
		RepairActionFixLevel, fp)}
}
