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

// RepairIssueReport is returned by ScanRepairIssuesLoaded / ScanRepairIssuesExternal.
type RepairIssueReport struct {
	SlotIndex int              `json:"slotIndex"`
	CharName  string           `json:"charName"`
	Issues    []RepairIssueDTO `json:"issues"`
	HasIssues bool             `json:"hasIssues"`
	Source    string           `json:"source"` // "loaded" or absolute file path
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
		core.RepairCodeDuplicateHandle:           {[]string{core.RepairActionCreateCopy}, core.RepairActionCreateCopy},
		core.RepairCodeDuplicateUID:              {[]string{core.RepairActionCreateCopy}, core.RepairActionCreateCopy},
		core.RepairCodeUnknownItemID:             {[]string{RepairActionLeaveUnchanged, core.RepairActionRemoveRecord}, RepairActionLeaveUnchanged},
		core.RepairCodeQuantityZero:              {[]string{core.RepairActionRemoveRecord, RepairActionLeaveUnchanged}, core.RepairActionRemoveRecord},
		core.RepairCodePassThroughRecords:        {[]string{core.RepairActionNoAction}, core.RepairActionNoAction},
		core.RepairCodeInventoryReserved:         {[]string{core.RepairActionRepairIndex}, core.RepairActionRepairIndex},
		core.RepairCodeDuplicateAcquisitionIndex: {[]string{core.RepairActionRepairIndex}, core.RepairActionRepairIndex},
		core.RepairCodeCurrentAoWMissing:         {[]string{core.RepairActionClearAoW, core.RepairActionPickAoW}, core.RepairActionClearAoW},
		core.RepairCodeCurrentAoWShared:          {[]string{core.RepairActionCreateCopy, core.RepairActionClearAoW}, core.RepairActionCreateCopy},
		core.RepairCodeCurrentAoWNonAoWCategory:  {[]string{core.RepairActionClearAoW}, core.RepairActionClearAoW},
		core.RepairCodeStatsFormula:              {[]string{core.RepairActionFixLevel}, core.RepairActionFixLevel},
		editor.CodeUpgradeOutOfRange:             {[]string{RepairActionClampUpgrade}, RepairActionClampUpgrade},
		editor.CodeCategoryUnsupported:           {[]string{RepairActionReportOnly}, RepairActionReportOnly},
		editor.CodePendingAoWUnknown:             {[]string{core.RepairActionClearAoW}, core.RepairActionClearAoW},
		editor.CodePendingAoWConflict:            {[]string{core.RepairActionClearAoW}, core.RepairActionClearAoW},
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
func workspaceIssueToDTO(slotIndex int, iss editor.WorkspaceValidationIssue, snap *editor.InventoryWorkspaceSnapshot) RepairIssueDTO {
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
	actions, def := repairActionsForCode(iss.Code)
	return RepairIssueDTO{
		IssueID:       core.IssueKeyID(key),
		DebugKey:      fmt.Sprintf("slot:%d|domain:%s|code:%s|scope:%s|handle:0x%08X", slotIndex, domain, iss.Code, scope, handle),
		Fingerprint:   "",
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
func buildRepairIssueReport(slotIndex int, charName, source string, slot *core.SaveSlot, wsValidation *editor.WorkspaceValidationReport, snap *editor.InventoryWorkspaceSnapshot) RepairIssueReport {
	coreIssues := core.ScanRepairIssues(slotIndex, slot)

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
			dto := workspaceIssueToDTO(slotIndex, wi, snap)
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
		Source:    source,
	}
}

// ---- endpoints --------------------------------------------------------------

// ScanRepairIssuesLoaded scans the loaded save slot at charIdx and returns a
// unified repair issue report merging raw/core and workspace findings.
// Read-only — does not mutate the slot.
func (a *App) ScanRepairIssuesLoaded(charIdx int) (RepairIssueReport, error) {
	var empty RepairIssueReport

	snap, err := a.StartInventoryEditSession(charIdx)
	if err != nil {
		return empty, fmt.Errorf("ScanRepairIssuesLoaded: %w", err)
	}

	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, fmt.Errorf("ScanRepairIssuesLoaded: no save loaded")
	}
	if charIdx < 0 || charIdx >= maxCharacters {
		return empty, fmt.Errorf("ScanRepairIssuesLoaded: invalid charIdx %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	slot := &a.save.Slots[charIdx]
	charName := core.UTF16ToString(slot.Player.CharacterName[:])
	return buildRepairIssueReport(charIdx, charName, "loaded", slot, &snap.Validation, &snap), nil
}

// ScanRepairIssuesExternal scans all populated slots of the external save cached
// by RunDiagnosticsExternal. Read-only — does not mutate the save.
// diagState.mu is held for the duration of the scan to prevent concurrent mutations.
func (a *App) ScanRepairIssuesExternal() ([]RepairIssueReport, error) {
	diagState.mu.Lock()
	defer diagState.mu.Unlock()

	if diagState.save == nil {
		return nil, fmt.Errorf("ScanRepairIssuesExternal: no external file loaded; run RunDiagnosticsExternal first")
	}

	var reports []RepairIssueReport
	for i := range diagState.save.Slots {
		slot := &diagState.save.Slots[i]
		if slot.Version == 0 {
			continue
		}
		charName := core.UTF16ToString(slot.Player.CharacterName[:])
		report := buildRepairIssueReport(i, charName, "external", slot, nil, nil)
		if report.HasIssues {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

// _forceExportTypesRepairScan surfaces all new DTO types to the Wails type
// generator. Never called at runtime.
func (a *App) _forceExportTypesRepairScan() (RepairIssueReport, RepairIssueDTO, RepairIssueRecord, RepairIssueAction, RepairCapacityRequirement, RepairTarget) {
	return RepairIssueReport{}, RepairIssueDTO{}, RepairIssueRecord{}, RepairIssueAction{}, RepairCapacityRequirement{}, RepairTarget{}
}
