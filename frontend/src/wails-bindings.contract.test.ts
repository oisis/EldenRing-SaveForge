import { describe, expect, expectTypeOf, it } from 'vitest';
import * as App from '../wailsjs/go/main/App';
import { core, editor, main, templates } from '../wailsjs/go/models';

// This is a contract test that locks the Wails-generated binding surface
// the inventory workspace UI depends on. Any unintentional rename or
// removal in app_inventory_session.go that propagates through `wails
// generate module` will turn this file red. The intent is to give the
// frontend an obvious "binding broke" signal long before runtime.
//
// Why this matters: the editor.WeaponPatch shape is mirrored in
// weaponPatch.ts; editor.AddItemSpec is constructed in SortOrderTab's
// add-item modal; editor.EditableItem.pendingAoW* is read directly by
// WeaponEditModal's workspace banner. Any field rename here would
// silently fall back to `undefined` at runtime — which is much harder
// to diagnose than a TypeScript error here.

describe('Wails binding contract: App methods', () => {
    it('exposes all workspace session methods used by useInventoryWorkspace', () => {
        expect(typeof App.StartInventoryEditSession).toBe('function');
        expect(typeof App.GetInventoryEditSession).toBe('function');
        expect(typeof App.ValidateInventoryWorkspace).toBe('function');
        expect(typeof App.MoveInventoryWorkspaceItem).toBe('function');
        expect(typeof App.TransferInventoryWorkspaceItem).toBe('function');
        expect(typeof App.AddInventoryWorkspaceItem).toBe('function');
        expect(typeof App.UpdateInventoryWorkspaceWeapon).toBe('function');
        expect(typeof App.RemoveInventoryWorkspaceItem).toBe('function');
        expect(typeof App.SaveInventoryWorkspaceChanges).toBe('function');
        expect(typeof App.DiscardInventoryEditSession).toBe('function');
    });
    it('exposes residual slot state and cleanup endpoints', () => {
        expect(typeof App.GetSlotStates).toBe('function');
        expect(typeof App.CleanResidualSlot).toBe('function');
        const sample = main.SlotState.createFrom({});
        expect('index' in sample).toBe(true);
        expect('name' in sample).toBe(true);
        expect('active' in sample).toBe(true);
        expect('residual' in sample).toBe(true);
        expect('empty' in sample).toBe(true);
    });

    it('exposes central loaded-save repair scan/apply endpoints', () => {
        expect(typeof App.ScanRepairIssuesLoaded).toBe('function');
        expect(typeof App.ApplyRepairsLoaded).toBe('function');
    });

    it('keeps legacy repair endpoints while UI migrates to central repair flow', () => {
        expect(typeof App.RepairInventoryWorkspaceItem).toBe('function');
        expect(typeof App.RepairInventoryWorkspaceItems).toBe('function');
        expect(typeof App.RepairDuplicateInventoryIndices).toBe('function');
    });

    it('no longer exposes the removed external diagnostics/repair endpoints', () => {
        expect((App as Record<string, unknown>).PickDiagnosticsFile).toBeUndefined();
        expect((App as Record<string, unknown>).RunDiagnosticsExternal).toBeUndefined();
        expect((App as Record<string, unknown>).RepairExternal).toBeUndefined();
        expect((App as Record<string, unknown>).SaveRepairedExternal).toBeUndefined();
        expect((App as Record<string, unknown>).ScanRepairIssuesExternal).toBeUndefined();
        expect((App as Record<string, unknown>).ApplyRepairsExternal).toBeUndefined();
        expect((App as Record<string, unknown>).RunDiagnosticsLoaded).toBeUndefined();
        expect((App as Record<string, unknown>).RepairLoadedSave).toBeUndefined();
    });

    it('exposes Phase E local build template library endpoints', () => {
        // Phase 8A removed the public JSON template exchange. Library
        // entries remain on-disk JSON (internal storage) but the public
        // exchange contract is YAML-only; the legacy v1 library apply
        // path (ApplyBuildTemplateFromLibrary) still ships because
        // already-stored v1 entries must remain applicable.
        expect(typeof App.SaveBuildTemplateToLibrary).toBe('function');
        expect(typeof App.ListBuildTemplateLibrary).toBe('function');
        expect(typeof App.PreviewBuildTemplateFromLibrary).toBe('function');
        expect(typeof App.ApplyBuildTemplateFromLibrary).toBe('function');
        expect(typeof App.DeleteBuildTemplateFromLibrary).toBe('function');
        expect(typeof App.RenameBuildTemplateInLibrary).toBe('function');
        expect(typeof App.ExportLibraryBuildTemplateAsYAMLToFile).toBe('function');
    });

    it('exposes Phase F library refresh / path endpoints', () => {
        expect(typeof App.RebuildBuildTemplateLibraryIndex).toBe('function');
        expect(typeof App.GetBuildTemplateLibraryPath).toBe('function');
    });
});

describe('Wails binding contract: central repair DTOs', () => {
    it('exposes core.IssueKey fields used by repair targets', () => {
        const sample = core.IssueKey.createFrom({});
        expect('slot' in sample).toBe(true);
        expect('domain' in sample).toBe(true);
        expect('code' in sample).toBe(true);
        expect('scope' in sample).toBe(true);
        expect('row' in sample).toBe(true);
        expect('handle' in sample).toBe(true);
        expect('field' in sample).toBe(true);
        expect('value' in sample).toBe(true);
    });

    it('exposes RepairIssueReport and nested issue/action/record/capacity DTOs', () => {
        const report = main.RepairIssueReport.createFrom({});
        expect('slotIndex' in report).toBe(true);
        expect('charName' in report).toBe(true);
        expect('issues' in report).toBe(true);
        expect('hasIssues' in report).toBe(true);

        const issue = main.RepairIssueDTO.createFrom({});
        expect('issueID' in issue).toBe(true);
        expect('debugKey' in issue).toBe(true);
        expect('fingerprint' in issue).toBe(true);
        expect('key' in issue).toBe(true);
        expect('description' in issue).toBe(true);
        expect('severity' in issue).toBe(true);
        expect('actions' in issue).toBe(true);
        expect('defaultAction' in issue).toBe(true);
        expect('record' in issue).toBe(true);
        expect('capacity' in issue).toBe(true);

        const action = main.RepairIssueAction.createFrom({});
        expect('id' in action).toBe(true);
        expect('label' in action).toBe(true);

        const record = main.RepairIssueRecord.createFrom({});
        expect('scope' in record).toBe(true);
        expect('row' in record).toBe(true);
        expect('handle' in record).toBe(true);
        expect('itemId' in record).toBe(true);
        expect('name' in record).toBe(true);
        expect('category' in record).toBe(true);
        expect('quantity' in record).toBe(true);
        expect('currentUpgrade' in record).toBe(true);
        expect('infusionName' in record).toBe(true);
        expect('unknown' in record).toBe(true);

        const capacity = main.RepairCapacityRequirement.createFrom({});
        expect('resource' in capacity).toBe(true);
        expect('needed' in capacity).toBe(true);
        expect('available' in capacity).toBe(true);
    });

    it('exposes RepairApplyTarget and RepairApplyReport result DTOs', () => {
        const target = main.RepairApplyTarget.createFrom({});
        expect('issueID' in target).toBe(true);
        expect('key' in target).toBe(true);
        expect('fingerprint' in target).toBe(true);
        expect('selectedAction' in target).toBe(true);
        expect('aowHandle' in target).toBe(true);
        expect('aowItemID' in target).toBe(true);

        const report = main.RepairApplyReport.createFrom({});
        expect('applied' in report).toBe(true);
        expect('skipped' in report).toBe(true);
        expect('failed' in report).toBe(true);
        expect('needsUserInput' in report).toBe(true);
        expect('stopped' in report).toBe(true);
        expect('results' in report).toBe(true);

        const result = main.RepairActionResult.createFrom({});
        expect('issueID' in result).toBe(true);
        expect('slotIndex' in result).toBe(true);
        expect('action' in result).toBe(true);
        expect('outcome' in result).toBe(true);
        expect('message' in result).toBe(true);
    });
});

describe('Wails binding contract: editor.EditableItem', () => {
    it('exposes the read-side AoW fields the modal banner reads', () => {
        const sample = editor.EditableItem.createFrom({});
        expect('currentAoWHandle' in sample).toBe(true);
        expect('currentAoWItemID' in sample).toBe(true);
        expect('currentAoWName' in sample).toBe(true);
        expect('currentAoWStatus' in sample).toBe(true);
        expect('hasCurrentAoW' in sample).toBe(true);
    });

    it('exposes the pending AoW fields the modal write-path mirrors', () => {
        const sample = editor.EditableItem.createFrom({});
        expect('pendingAoWItemID' in sample).toBe(true);
        expect('pendingAoWName' in sample).toBe(true);
        expect('pendingAoWClear' in sample).toBe(true);
        expect('hasPendingWeaponPatch' in sample).toBe(true);
    });

    it('exposes the core identification fields used everywhere', () => {
        const sample = editor.EditableItem.createFrom({});
        expect('uid' in sample).toBe(true);
        expect('source' in sample).toBe(true);
        expect('container' in sample).toBe(true);
        expect('position' in sample).toBe(true);
        expect('originalHandle' in sample).toBe(true);
        expect('itemID' in sample).toBe(true);
        expect('baseItemID' in sample).toBe(true);
        expect('isWeapon' in sample).toBe(true);
    });
});

describe('Wails binding contract: editor.AddItemSpec', () => {
    it('exposes the fields SortOrderTab AddItemModal sends', () => {
        const sample = editor.AddItemSpec.createFrom({});
        expect('itemID' in sample).toBe(true);
        expect('baseItemID' in sample).toBe(true);
        expect('quantity' in sample).toBe(true);
        expect('upgrade' in sample).toBe(true);
        expect('infusionName' in sample).toBe(true);
        expect('aowItemID' in sample).toBe(true);
    });
});

describe('Wails binding contract: editor.WeaponPatch', () => {
    it('exposes set+payload pairs and the clearAoW flag', () => {
        const sample = editor.WeaponPatch.createFrom({});
        expect('setUpgrade' in sample).toBe(true);
        expect('upgrade' in sample).toBe(true);
        expect('setInfusionName' in sample).toBe(true);
        expect('infusionName' in sample).toBe(true);
        expect('setAoWItemID' in sample).toBe(true);
        expect('aowItemID' in sample).toBe(true);
        expect('clearAoW' in sample).toBe(true);
    });

    it('round-trips a full assignment through createFrom', () => {
        const p = editor.WeaponPatch.createFrom({
            setUpgrade: true, upgrade: 25,
            setInfusionName: true, infusionName: 'Keen',
            setAoWItemID: true, aowItemID: 0x80002710, clearAoW: false,
        });
        expect(p.setUpgrade).toBe(true);
        expect(p.upgrade).toBe(25);
        expect(p.setInfusionName).toBe(true);
        expect(p.infusionName).toBe('Keen');
        expect(p.setAoWItemID).toBe(true);
        expect(p.aowItemID).toBe(0x80002710);
        expect(p.clearAoW).toBe(false);
    });
});

describe('Wails binding contract: editor.InventoryWorkspaceSnapshot', () => {
    it('exposes the top-level fields the hook reads', () => {
        const sample = editor.InventoryWorkspaceSnapshot.createFrom({});
        expect('sessionID' in sample).toBe(true);
        expect('characterIndex' in sample).toBe(true);
        expect('inventoryItems' in sample).toBe(true);
        expect('storageItems' in sample).toBe(true);
        expect('unsupportedInventoryRecords' in sample).toBe(true);
        expect('unsupportedStorageRecords' in sample).toBe(true);
        expect('dirty' in sample).toBe(true);
        expect('validation' in sample).toBe(true);
    });

    it('snapshot.dirty is a boolean (type-level)', () => {
        const sample = editor.InventoryWorkspaceSnapshot.createFrom({ dirty: true });
        expectTypeOf(sample.dirty).toEqualTypeOf<boolean>();
    });
});

describe('Wails binding contract: editor.WorkspaceValidationReport', () => {
    it('exposes ok/errors/warnings the validation panel reads', () => {
        const sample = editor.WorkspaceValidationReport.createFrom({});
        expect('ok' in sample).toBe(true);
        expect('errors' in sample).toBe(true);
        expect('warnings' in sample).toBe(true);
        expect('inventoryItemCount' in sample).toBe(true);
        expect('storageItemCount' in sample).toBe(true);
    });
});

describe('Wails binding contract: build template export DTOs', () => {
    it('exposes BuildTemplateExportOptions fields the modal sends', () => {
        const sample = main.BuildTemplateExportOptions.createFrom({});
        expect('includeInventory' in sample).toBe(true);
        expect('includeStorage' in sample).toBe(true);
        expect('name' in sample).toBe(true);
        expect('description' in sample).toBe(true);
        expect('author' in sample).toBe(true);
        expect('tags' in sample).toBe(true);
    });

    it('exposes BuildTemplateExportResult fields the UI reads back', () => {
        const sample = main.BuildTemplateExportResult.createFrom({});
        expect('path' in sample).toBe(true);
        expect('json' in sample).toBe(true);
        expect('warnings' in sample).toBe(true);
        expect('skippedItems' in sample).toBe(true);
    });

    it('exposes templates.ExportWarning fields for surface in toasts', () => {
        const sample = templates.ExportWarning.createFrom({});
        expect('code' in sample).toBe(true);
        expect('message' in sample).toBe(true);
        expect('container' in sample).toBe(true);
        expect('position' in sample).toBe(true);
    });
});

describe('Wails binding contract: import preview DTOs (Phase C)', () => {
    it('exposes ImportPreviewReport top-level fields', () => {
        const sample = templates.ImportPreviewReport.createFrom({});
        expect('ok' in sample).toBe(true);
        expect('errors' in sample).toBe(true);
        expect('warnings' in sample).toBe(true);
        expect('summary' in sample).toBe(true);
    });

    it('exposes ImportPreviewIssue positional fields', () => {
        const sample = templates.ImportPreviewIssue.createFrom({});
        expect('severity' in sample).toBe(true);
        expect('code' in sample).toBe(true);
        expect('message' in sample).toBe(true);
        expect('container' in sample).toBe(true);
        expect('position' in sample).toBe(true);
        expect('baseItemID' in sample).toBe(true);
        expect('aowItemID' in sample).toBe(true);
    });

    it('exposes ImportPreviewSummary bucket counters', () => {
        const sample = templates.ImportPreviewSummary.createFrom({});
        expect('inventoryItems' in sample).toBe(true);
        expect('storageItems' in sample).toBe(true);
        expect('weapons' in sample).toBe(true);
        expect('armor' in sample).toBe(true);
        expect('talismans' in sample).toBe(true);
        expect('stackables' in sample).toBe(true);
        expect('aowAssignments' in sample).toBe(true);
    });
});

describe('Wails binding contract: apply DTOs (Phase D)', () => {
    it('exposes ApplyTemplateOptions fields', () => {
        const sample = main.ApplyTemplateOptions.createFrom({});
        expect('mode' in sample).toBe(true);
    });

    it('exposes ApplyTemplateResult fields the hook reads back', () => {
        const sample = main.ApplyTemplateResult.createFrom({});
        expect('preview' in sample).toBe(true);
        expect('workspace' in sample).toBe(true);
        expect('applied' in sample).toBe(true);
    });

    it('exposes LoadedTemplatePreview fields the preview flow reads', () => {
        const sample = main.LoadedTemplatePreview.createFrom({});
        expect('report' in sample).toBe(true);
        expect('json' in sample).toBe(true);
        expect('path' in sample).toBe(true);
    });

    // Phase 8E.2 — the apply result modal reads back layout counters
    // produced by the Phase 8E.1 writer. A binding rename here would
    // silently zero those numbers in the UI; lock the field names.
    it('exposes ApplyTemplateV2Result layout counters the result modal reads', () => {
        const sample = main.ApplyTemplateV2Result.createFrom({});
        expect('layoutInventoryEntriesApplied' in sample).toBe(true);
        expect('layoutStorageEntriesApplied' in sample).toBe(true);
        expect('layoutInventoryEntriesMissing' in sample).toBe(true);
        expect('layoutStorageEntriesMissing' in sample).toBe(true);
        expect('layoutInventoryExtrasPreserved' in sample).toBe(true);
        expect('layoutStorageExtrasPreserved' in sample).toBe(true);
    });
});

describe('Wails binding contract: template library DTOs (Phase E)', () => {
    it('exposes LibraryTemplateEntry fields the UI list renders', () => {
        const sample = templates.LibraryTemplateEntry.createFrom({});
        expect('id' in sample).toBe(true);
        expect('name' in sample).toBe(true);
        expect('description' in sample).toBe(true);
        expect('tags' in sample).toBe(true);
        expect('filename' in sample).toBe(true);
        expect('createdAt' in sample).toBe(true);
        expect('updatedAt' in sample).toBe(true);
        expect('inventoryItems' in sample).toBe(true);
        expect('storageItems' in sample).toBe(true);
        expect('warnings' in sample).toBe(true);
    });
});
