import {
    ApplyRepairsExternal,
    ApplyRepairsLoaded,
    ScanRepairIssuesExternal,
    ScanRepairIssuesLoaded,
} from '../../wailsjs/go/main/App';
import { core, main } from '../../wailsjs/go/models';

export type RepairSource = 'loaded' | 'external';

export type IssueKey = core.IssueKey;
export type RepairIssueAction = main.RepairIssueAction;
export type RepairIssueRecord = main.RepairIssueRecord;
export type RepairCapacityRequirement = main.RepairCapacityRequirement;
export type RepairIssue = Omit<main.RepairIssueDTO, 'convertValues'>;
export type RepairIssueReport = Omit<main.RepairIssueReport, 'convertValues' | 'issues'> & {
    issues: RepairIssue[];
};
export type RepairApplyTarget = Omit<main.RepairApplyTarget, 'convertValues'>;
export type RepairActionResult = main.RepairActionResult;
export type RepairApplyReport = Omit<main.RepairApplyReport, 'convertValues'>;

export function scanRepairIssuesLoaded(charIdx: number): Promise<RepairIssueReport> {
    return ScanRepairIssuesLoaded(charIdx);
}

export function scanRepairIssuesExternal(): Promise<RepairIssueReport[]> {
    return ScanRepairIssuesExternal();
}

export function applyRepairsLoaded(charIdx: number, targets: RepairApplyTarget[], stopOnFirstFailure: boolean): Promise<RepairApplyReport> {
    return ApplyRepairsLoaded(charIdx, targets.map(toGeneratedTarget), stopOnFirstFailure);
}

export function applyRepairsExternal(targets: RepairApplyTarget[], stopOnFirstFailure: boolean): Promise<RepairApplyReport> {
    return ApplyRepairsExternal(targets.map(toGeneratedTarget), stopOnFirstFailure);
}

export function makeRepairTarget(issue: RepairIssue, selectedAction: string): RepairApplyTarget {
    return main.RepairApplyTarget.createFrom({
        issueID: issue.issueID,
        key: issue.key,
        fingerprint: issue.fingerprint,
        selectedAction,
    });
}

function toGeneratedTarget(target: RepairApplyTarget): main.RepairApplyTarget {
    return main.RepairApplyTarget.createFrom(target);
}
