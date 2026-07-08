export type RepairSource = 'loaded' | 'external';

export interface IssueKey {
    slot: number;
    domain: string;
    code: string;
    scope: string;
    row: number;
    handle: number;
    field?: string;
    value?: string;
}

export interface RepairIssueAction {
    id: string;
    label: string;
}

export interface RepairIssueRecord {
    scope: string;
    row: number;
    handle: number;
    itemId: number;
    name: string;
    category: string;
    quantity: number;
    currentUpgrade: number;
    infusionName: string;
    unknown: boolean;
}

export interface RepairCapacityRequirement {
    resource: string;
    needed: number;
    available: number;
}

export interface RepairIssue {
    issueID: string;
    debugKey: string;
    fingerprint: string;
    key: IssueKey;
    description: string;
    severity: string;
    actions: RepairIssueAction[];
    defaultAction: string;
    record?: RepairIssueRecord;
    capacity?: RepairCapacityRequirement;
}

export interface RepairIssueReport {
    slotIndex: number;
    charName: string;
    issues: RepairIssue[];
    hasIssues: boolean;
    source: string;
}

export interface RepairApplyTarget {
    issueID: string;
    key: IssueKey;
    fingerprint: string;
    selectedAction: string;
    aowHandle?: number;
    aowItemID?: number;
}

export interface RepairActionResult {
    issueID: string;
    slotIndex: number;
    action: string;
    outcome: 'applied' | 'skipped' | 'failed' | 'needsUserInput' | string;
    message: string;
}

export interface RepairApplyReport {
    applied: number;
    skipped: number;
    failed: number;
    needsUserInput: number;
    stopped: boolean;
    results: RepairActionResult[];
}

type WailsApp = {
    ScanRepairIssuesLoaded?: (charIdx: number) => Promise<RepairIssueReport>;
    ScanRepairIssuesExternal?: () => Promise<RepairIssueReport[]>;
    ApplyRepairsLoaded?: (charIdx: number, targets: RepairApplyTarget[], stopOnFirstFailure: boolean) => Promise<RepairApplyReport>;
    ApplyRepairsExternal?: (targets: RepairApplyTarget[], stopOnFirstFailure: boolean) => Promise<RepairApplyReport>;
};

function app(): WailsApp {
    return ((window as unknown as { go?: { main?: { App?: WailsApp } } }).go?.main?.App) ?? {};
}

function requireEndpoint<K extends keyof WailsApp>(name: K): NonNullable<WailsApp[K]> {
    const fn = app()[name];
    if (!fn) {
        throw new Error(`${String(name)} binding is not available. Regenerate Wails bindings.`);
    }
    return fn as NonNullable<WailsApp[K]>;
}

export function scanRepairIssuesLoaded(charIdx: number): Promise<RepairIssueReport> {
    return requireEndpoint('ScanRepairIssuesLoaded')(charIdx);
}

export function scanRepairIssuesExternal(): Promise<RepairIssueReport[]> {
    return requireEndpoint('ScanRepairIssuesExternal')();
}

export function applyRepairsLoaded(charIdx: number, targets: RepairApplyTarget[], stopOnFirstFailure: boolean): Promise<RepairApplyReport> {
    return requireEndpoint('ApplyRepairsLoaded')(charIdx, targets, stopOnFirstFailure);
}

export function applyRepairsExternal(targets: RepairApplyTarget[], stopOnFirstFailure: boolean): Promise<RepairApplyReport> {
    return requireEndpoint('ApplyRepairsExternal')(targets, stopOnFirstFailure);
}

export function makeRepairTarget(issue: RepairIssue, selectedAction: string): RepairApplyTarget {
    return {
        issueID: issue.issueID,
        key: issue.key,
        fingerprint: issue.fingerprint,
        selectedAction,
    };
}
