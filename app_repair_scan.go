package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// ---- DTO types --------------------------------------------------------------

type RepairIssueAction struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// RepairIssueRecord enriches one inventory row involved in an issue.
// Fields mirror InventoryIntegrityConflictItem for UI consistency.
type RepairIssueRecord struct {
	Scope          string `json:"scope"`
	Row            int    `json:"row"`
	Handle         uint32 `json:"handle"`
	ItemID         uint32 `json:"itemId"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	Quantity       int    `json:"quantity"`
	CurrentUpgrade int    `json:"currentUpgrade"`
	InfusionName   string `json:"infusionName"`
	Unknown        bool   `json:"unknown"`
}

// RepairCapacityRequirement describes slot-level capacity for a repair action.
// Populated only when the action creates new GaItem / inventory entries.
// Resource identifies what resource is constrained (e.g. "gaitems", "inventory").
type RepairCapacityRequirement struct {
	Resource  string `json:"resource"`
	Needed    int    `json:"needed"`
	Available int    `json:"available"`
}

// RepairTarget identifies one issue and the chosen action for the apply endpoint.
type RepairTarget struct {
	IssueID        string `json:"issueID"`
	SelectedAction string `json:"selectedAction"`
}

// RepairIssueDTO is the wire-format issue returned by scan endpoints.
// It unifies raw/core issues (from core.ScanRepairIssues) and
// workspace issues (from editor.Validate) into a single shape.
type RepairIssueDTO struct {
	IssueID       string                     `json:"issueID"`
	DebugKey      string                     `json:"debugKey"`
	Fingerprint   string                     `json:"fingerprint"`
	Key           core.IssueKey              `json:"key"`
	Description   string                     `json:"description"`
	Severity      string                     `json:"severity"`
	Actions       []RepairIssueAction        `json:"actions"`
	DefaultAction string                     `json:"defaultAction"`
	Record        *RepairIssueRecord         `json:"record,omitempty"`
	Capacity      *RepairCapacityRequirement `json:"capacity,omitempty"`
}

// RepairIssueReport is returned by ScanRepairIssuesLoaded.
type RepairIssueReport struct {
	SlotIndex int                     `json:"slotIndex"`
	CharName  string                  `json:"charName"`
	Issues    []RepairIssueDTO        `json:"issues"`
	HasIssues bool                    `json:"hasIssues"`
	Coverage  core.ValidationCoverage `json:"coverage"`
}

// App-layer action ID constants (supplement core.RepairAction* constants).
const (
	RepairActionLeaveUnchanged = "leave_unchanged"
	RepairActionClampUpgrade   = "clamp_upgrade"
	RepairActionReportOnly     = "report_only"
)

// ---- action helpers ---------------------------------------------------------

func repairActionLabel(id string) string {
	switch id {
	case core.RepairActionCreateCopy:
		return "Create separate copy"
	case core.RepairActionRemoveRecord:
		return "Remove record"
	case core.RepairActionClearAoW:
		return "Clear Ash of War"
	case core.RepairActionPickAoW:
		return "Pick replacement Ash of War"
	case core.RepairActionRepairIndex:
		return "Assign new acquisition index"
	case core.RepairActionClampQuantity:
		return "Clamp quantity to allowed maximum"
	case core.RepairActionFixLevel:
		return "Set level to formula result"
	case core.RepairActionNoAction:
		return "No action"
	case RepairActionLeaveUnchanged:
		return "Leave unchanged"
	case RepairActionClampUpgrade:
		return "Clamp upgrade to max"
	case RepairActionReportOnly:
		return "Report only (no auto-repair)"
	default:
		return id
	}
}

// repairActionsForCode returns the canonical action list and default action for
// a given issue code, covering both core and workspace codes.
func repairActionsForCode(code string) ([]RepairIssueAction, string) {
	type mapping struct {
		ids []string
		def string
	}
	m := map[string]mapping{
		core.RepairCodeDuplicateHandle:           {[]string{core.RepairActionCreateCopy, RepairActionLeaveUnchanged}, core.RepairActionCreateCopy},
		core.RepairCodeDuplicateUID:              {[]string{core.RepairActionCreateCopy, RepairActionLeaveUnchanged}, core.RepairActionCreateCopy},
		core.RepairCodeUnknownItemID:             {[]string{RepairActionLeaveUnchanged, core.RepairActionRemoveRecord}, RepairActionLeaveUnchanged},
		core.RepairCodeUnknownHandleType:         {[]string{RepairActionLeaveUnchanged, core.RepairActionRemoveRecord}, RepairActionLeaveUnchanged},
		core.RepairCodeMissingGaItemMapping:      {[]string{core.RepairActionNoAction}, core.RepairActionNoAction},
		core.RepairCodeQuantityZero:              {[]string{core.RepairActionRemoveRecord, RepairActionLeaveUnchanged}, core.RepairActionRemoveRecord},
		core.RepairCodeQuantityAboveMax:          {[]string{core.RepairActionClampQuantity, RepairActionLeaveUnchanged}, core.RepairActionClampQuantity},
		core.RepairCodeItemNotAllowedInContainer: {[]string{core.RepairActionRemoveRecord, RepairActionLeaveUnchanged}, RepairActionLeaveUnchanged},
		core.RepairCodePassThroughRecords:        {[]string{core.RepairActionNoAction}, core.RepairActionNoAction},
		core.RepairCodeDuplicateAcquisitionIndex: {[]string{core.RepairActionRepairIndex, RepairActionLeaveUnchanged}, core.RepairActionRepairIndex},
		core.RepairCodeCurrentAoWMissing:         {[]string{core.RepairActionClearAoW, core.RepairActionPickAoW, RepairActionLeaveUnchanged}, core.RepairActionClearAoW},
		core.RepairCodeCurrentAoWShared:          {[]string{core.RepairActionCreateCopy, core.RepairActionClearAoW, RepairActionLeaveUnchanged}, core.RepairActionCreateCopy},
		core.RepairCodeCurrentAoWNonAoWCategory:  {[]string{core.RepairActionClearAoW, RepairActionLeaveUnchanged}, core.RepairActionClearAoW},
		core.RepairCodeStatsFormula:              {[]string{core.RepairActionFixLevel, RepairActionLeaveUnchanged}, core.RepairActionFixLevel},
		editor.CodeUpgradeOutOfRange:             {[]string{RepairActionClampUpgrade, RepairActionLeaveUnchanged}, RepairActionClampUpgrade},
		editor.CodeCategoryUnsupported:           {[]string{RepairActionReportOnly}, RepairActionReportOnly},
		editor.CodePendingAoWUnknown:             {[]string{core.RepairActionClearAoW, RepairActionLeaveUnchanged}, core.RepairActionClearAoW},
		editor.CodePendingAoWConflict:            {[]string{core.RepairActionClearAoW, RepairActionLeaveUnchanged}, core.RepairActionClearAoW},
	}
	p, ok := m[code]
	if !ok {
		p = mapping{ids: []string{core.RepairActionNoAction}, def: core.RepairActionNoAction}
	}
	actions := make([]RepairIssueAction, len(p.ids))
	for i, id := range p.ids {
		actions[i] = RepairIssueAction{ID: id, Label: repairActionLabel(id)}
	}
	return actions, p.def
}

// ---- record builder ---------------------------------------------------------

// buildRecordFromIssueKey resolves the inventory row described by key into a
// RepairIssueRecord, enriched via resolveConflictItem (name/category from DB).
// Returns nil when the key doesn't address a single concrete row.
func buildRecordFromIssueKey(slot *core.SaveSlot, key core.IssueKey) *RepairIssueRecord {
	var items []core.InventoryItem
	switch key.Scope {
	case "inventory_common":
		items = slot.Inventory.CommonItems
	case "inventory_key":
		items = slot.Inventory.KeyItems
	case "storage_common":
		items = slot.Storage.CommonItems
	default:
		return nil
	}
	if key.Row < 0 || key.Row >= len(items) {
		return nil
	}
	c := resolveConflictItem(key.Scope, key.Row, items[key.Row], slot)
	return &RepairIssueRecord{
		Scope:          c.Scope,
		Row:            c.Row,
		Handle:         c.Handle,
		ItemID:         c.ItemID,
		Name:           c.Name,
		Category:       c.Category,
		Quantity:       c.Quantity,
		CurrentUpgrade: c.CurrentUpgrade,
		InfusionName:   c.InfusionName,
		Unknown:        c.Unknown,
	}
}

// containerToScope converts a workspace ContainerKind to the core scope name
// used in IssueKey and RepairIssueRecord.
func containerToScope(c editor.ContainerKind) string {
	if c == editor.ContainerStorage {
		return "storage_common"
	}
	return "inventory_common"
}

// findWorkspaceItem looks up an EditableItem by UID across both snapshot containers.
func findWorkspaceItem(snap *editor.InventoryWorkspaceSnapshot, uid string) (editor.EditableItem, bool) {
	for _, it := range snap.InventoryItems {
		if it.UID == uid {
			return it, true
		}
	}
	for _, it := range snap.StorageItems {
		if it.UID == uid {
			return it, true
		}
	}
	return editor.EditableItem{}, false
}

// ---- DTO conversion ---------------------------------------------------------

func coreIssueToDTO(slot *core.SaveSlot, iss core.RepairIssue) RepairIssueDTO {
	actions, def := repairActionsForCode(iss.Key.Code)
	return RepairIssueDTO{
		IssueID:       iss.IssueID,
		DebugKey:      iss.DebugKey,
		Fingerprint:   iss.Fingerprint,
		Key:           iss.Key,
		Description:   iss.Description,
		Severity:      iss.Severity,
		Actions:       actions,
		DefaultAction: def,
		Record:        buildRecordFromIssueKey(slot, iss.Key),
	}
}

// workspaceIssueDomain maps a workspace issue code to a repair domain.
func workspaceIssueDomain(code string) string {
	switch code {
	case editor.CodeCurrentAoWMissing, editor.CodeCurrentAoWShared, editor.CodeCurrentAoWNonAoWCategory,
		editor.CodePendingAoWUnknown, editor.CodePendingAoWConflict, editor.CodeSharedAoWConflict:
		return "aow"
	default:
		return "inventory"
	}
}

// workspaceIssueToDTO converts a workspace validation issue to RepairIssueDTO.
//
// When the issue carries a UID and a snapshot is provided, the function resolves
// the matching EditableItem and uses its container/slot/handle to populate the
// IssueKey and Record — giving the UI the item context it needs for decisions.
// Issues without a UID (global or AoW-level) fall back to scope="workspace"/row=-1.
func workspaceIssueToDTO(slotIndex int, slot *core.SaveSlot, iss editor.WorkspaceValidationIssue, snap *editor.InventoryWorkspaceSnapshot) RepairIssueDTO {
	domain := workspaceIssueDomain(iss.Code)
	scope := "workspace"
	row := -1
	handle := iss.Handle
	var record *RepairIssueRecord

	if iss.UID != "" && snap != nil {
		if item, ok := findWorkspaceItem(snap, iss.UID); ok {
			scope = containerToScope(item.Container)
			row = item.OriginalSlotIndex
			handle = item.OriginalHandle
			record = &RepairIssueRecord{
				Scope:          scope,
				Row:            row,
				Handle:         item.OriginalHandle,
				ItemID:         item.ItemID,
				Name:           item.Name,
				Category:       item.Category,
				Quantity:       int(item.Quantity),
				CurrentUpgrade: item.CurrentUpgrade,
				InfusionName:   item.InfusionName,
				Unknown:        false,
			}
		}
	}

	key := core.IssueKey{
		Slot:   slotIndex,
		Domain: domain,
		Code:   iss.Code,
		Scope:  scope,
		Row:    row,
		Handle: handle,
	}
	// Workspace validation is derived from a throwaway snapshot, but upgrade
	// repairs still address one concrete binary inventory record. Carry its
	// fingerprint through the modal so ApplyRepairsLoaded can reject a stale
	// row just like the core-scanner repairs do.
	fingerprint := ""
	if slot != nil && scopeAddressesRecord(scope) {
		fingerprint, _ = core.FingerprintRecordAt(slot, scope, row)
	}
	actions, def := repairActionsForCode(iss.Code)
	return RepairIssueDTO{
		IssueID:       core.IssueKeyID(key),
		DebugKey:      fmt.Sprintf("slot:%d|domain:%s|code:%s|scope:%s|handle:0x%08X", slotIndex, domain, iss.Code, scope, handle),
		Fingerprint:   fingerprint,
		Key:           key,
		Description:   iss.Message,
		Severity:      iss.Severity,
		Actions:       actions,
		DefaultAction: def,
		Record:        record,
	}
}

// buildRepairIssueReport merges core and workspace issues for one slot.
// Deduplication is by IssueID (not code), so the same code for different
// records/handles is never silently dropped.
func buildRepairIssueReport(slotIndex int, charName string, slot *core.SaveSlot, wsValidation *editor.WorkspaceValidationReport, snap *editor.InventoryWorkspaceSnapshot) RepairIssueReport {
	// Resolve the physical record collection once and feed both the coverage
	// report and the scanner from it, so their semantics cannot diverge. Coverage
	// is produced by the scan itself so StructuralChecksApplied reflects the
	// checks the scanner actually executed (not a builder assumption).
	records := core.ResolveInventoryRecords(slot)
	coreIssues, coverage := core.ScanRepairIssuesWithCoverage(slotIndex, slot, records)

	seenIDs := make(map[string]bool, len(coreIssues))
	dtos := make([]RepairIssueDTO, 0, len(coreIssues))
	for _, ci := range coreIssues {
		dto := coreIssueToDTO(slot, ci)
		seenIDs[dto.IssueID] = true
		dtos = append(dtos, dto)
	}

	if wsValidation != nil {
		allWSIssues := append(wsValidation.Errors, wsValidation.Warnings...)
		for _, wi := range allWSIssues {
			dto := workspaceIssueToDTO(slotIndex, slot, wi, snap)
			if seenIDs[dto.IssueID] {
				continue
			}
			seenIDs[dto.IssueID] = true
			dtos = append(dtos, dto)
		}
	}

	return RepairIssueReport{
		SlotIndex: slotIndex,
		CharName:  charName,
		Issues:    dtos,
		HasIssues: len(dtos) > 0,
		Coverage:  coverage,
	}
}

// buildValidationSnapshot is editor.BuildSnapshot behind an unexported package
// var so the build_snapshot failure stage — otherwise unreachable, since
// BuildSnapshot only errors on a nil slot the endpoint already rejects — has a
// deterministic test seam. Production always runs editor.BuildSnapshot.
// ponytail: package-var seam; the only way to exercise the build_snapshot stage.
var buildValidationSnapshot = editor.BuildSnapshot

// ---- endpoints --------------------------------------------------------------

// ScanRepairIssuesLoaded scans the loaded save slot at charIdx and returns a
// unified repair issue report merging raw/core and workspace findings.
//
// Read-only with respect to both the slot and Inventory Workspaces: it builds
// a throwaway snapshot via editor.BuildSnapshot instead of publishing an edit
// session, so a diagnostic scan never creates, replaces, or discards an
// existing session or its pending edits.
func (a *App) ScanRepairIssuesLoaded(charIdx int) (RepairIssueReport, error) {
	// The operation lifecycle brackets the whole call. requested fires before any
	// lock or validation, so even an early rejection still leaves a
	// requested/finished pair. Both are debug-gated at the journal, so Debug off
	// leaves no trace. finished is emitted by THIS wrapper only after scanRepair
	// IssuesLoaded has returned and its deferred saveMu.RUnlock/slotMu.Unlock have
	// run, so the journal Sync inside finished never happens under a save/slot lock.
	a.journalToolsOperationRequested(actionToolsScanRepairIssuesLoaded)

	report, stage, err := a.scanRepairIssuesLoaded(charIdx)
	if err != nil {
		a.journalToolsOperationFinished(actionToolsScanRepairIssuesLoaded, characterChangeError, stage)
		return report, err
	}
	a.journalToolsOperationFinished(actionToolsScanRepairIssuesLoaded, characterChangeSuccess, toolsStageCompleted)
	return report, nil
}

// scanRepairIssuesLoaded is the locked, read-only core of ScanRepairIssuesLoaded.
// It performs the scan under saveMu.RLock/slotMu.Lock and returns the report, the
// closed operation stage to report on failure, and the public error. It emits NO
// journal record itself, so the wrapper can write the operation-finished event
// only after these locks are released — the journal Sync must never run under
// saveMu or slotMu. The public error strings are the exact pre-existing ones.
func (a *App) scanRepairIssuesLoaded(charIdx int) (RepairIssueReport, string, error) {
	var empty RepairIssueReport

	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, toolsStageNoActiveSave, fmt.Errorf("ScanRepairIssuesLoaded: no save loaded")
	}
	if charIdx < 0 || charIdx >= maxCharacters {
		return empty, toolsStageInvalidCharacter, fmt.Errorf("ScanRepairIssuesLoaded: invalid charIdx %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	slot := &a.save.Slots[charIdx]

	// Preserve the pre-refactor empty-slot failure. The prior implementation
	// delegated to StartInventoryEditSession, which errored on Version == 0;
	// BuildSnapshot alone would silently return an empty report instead.
	if slot.Version == 0 {
		return empty, toolsStageEmptySlot, fmt.Errorf("ScanRepairIssuesLoaded: slot %d is empty", charIdx)
	}

	// Build the validation snapshot inline (no session publish). Mirrors
	// editor.StartSession's Build+Validate, minus the registry side effects.
	snap, err := buildValidationSnapshot(slot, "", charIdx)
	if err != nil {
		return empty, toolsStageBuildSnapshot, fmt.Errorf("ScanRepairIssuesLoaded: %w", err)
	}
	snap.Validation = editor.Validate(snap)

	charName := core.UTF16ToString(slot.Player.CharacterName[:])
	return buildRepairIssueReport(charIdx, charName, slot, &snap.Validation, &snap), "", nil
}

// _forceExportTypesRepairScan surfaces all new DTO types to the Wails type
// generator. Never called at runtime.
func (a *App) _forceExportTypesRepairScan() (RepairIssueReport, RepairIssueDTO, RepairIssueRecord, RepairIssueAction, RepairCapacityRequirement, RepairTarget) {
	return RepairIssueReport{}, RepairIssueDTO{}, RepairIssueRecord{}, RepairIssueAction{}, RepairCapacityRequirement{}, RepairTarget{}
}
