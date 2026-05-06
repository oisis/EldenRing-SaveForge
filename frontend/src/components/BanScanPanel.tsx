import {useState} from 'react';
import {ScanBanRisk} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {RiskInfoIcon} from './RiskInfoIcon';
import type {RiskKey} from '../data/riskInfo';
import toast from '../lib/toast';

const TIER_STYLES: Record<string, {badge: string; dot: string}> = {
    high:   {badge: 'bg-red-500/15 text-red-300 border border-red-500/30',   dot: 'bg-red-400'},
    medium: {badge: 'bg-orange-500/15 text-orange-300 border border-orange-500/30', dot: 'bg-orange-400'},
    info:   {badge: 'bg-blue-500/15 text-blue-300 border border-blue-500/30',  dot: 'bg-blue-400'},
};

const CATEGORY_LABELS: Record<string, string> = {
    cut_content:          'Cut Content',
    ban_risk:             'Ban Risk Item',
    stat_above_99:        'Stat > 99',
    level_above_713:      'Level > 713',
    upgrade_cap:          'Upgrade Cap',
    steamid_mismatch:     'SteamID Mismatch',
    soul_memory_mismatch: 'Soul Memory Too Low',
};

const KNOWN_RISK_KEYS = new Set<string>([
    'cut_content', 'ban_risk', 'stat_above_99', 'level_above_713',
    'upgrade_cap', 'steamid_mismatch', 'soul_memory_mismatch',
]);

const CHECKS_PERFORMED: {group: string; items: {label: string; desc: string; tier: 'high' | 'medium'}[]}[] = [
    {
        group: 'Character Stats',
        items: [
            {label: 'Level > 713',         desc: 'hard cap — all 8 attributes at 99 = Lv 713', tier: 'high'},
            {label: 'Vigor > 99',          desc: 'attribute above vanilla maximum', tier: 'high'},
            {label: 'Mind > 99',           desc: 'attribute above vanilla maximum', tier: 'high'},
            {label: 'Endurance > 99',      desc: 'attribute above vanilla maximum', tier: 'high'},
            {label: 'Strength > 99',       desc: 'attribute above vanilla maximum', tier: 'high'},
            {label: 'Dexterity > 99',      desc: 'attribute above vanilla maximum', tier: 'high'},
            {label: 'Intelligence > 99',   desc: 'attribute above vanilla maximum', tier: 'high'},
            {label: 'Faith > 99',          desc: 'attribute above vanilla maximum', tier: 'high'},
            {label: 'Arcane > 99',         desc: 'attribute above vanilla maximum', tier: 'high'},
        ],
    },
    {
        group: 'Rune Economy',
        items: [
            {label: 'Soul Memory too low', desc: 'SoulMemory must be ≥ cumulative rune cost from class start level to current level — direct level editing without spending runes creates a detectable mismatch', tier: 'medium'},
        ],
    },
    {
        group: 'Inventory & Storage',
        items: [
            {label: 'Cut content items',   desc: 'item IDs that were never shipped to retail — cannot be obtained through normal play', tier: 'high'},
            {label: 'Ban-risk items',      desc: 'items flagged by the community as known or suspected ban triggers', tier: 'high'},
            {label: 'Weapon upgrade cap',  desc: '+25 standard / +10 somber — upgrade level encoded in item ID; any value above the cap is impossible to achieve legitimately', tier: 'high'},
        ],
    },
    {
        group: 'Save Identity (PC only)',
        items: [
            {label: 'SteamID mismatch', desc: 'slot SteamID vs save-level SteamID — loading another player\'s save under your account is a confirmed ban trigger', tier: 'high'},
        ],
    },
];

function tierOrder(tier: string): number {
    return tier === 'high' ? 0 : tier === 'medium' ? 1 : 2;
}

function sortedFindings(findings: main.BanFinding[]): main.BanFinding[] {
    return [...findings].sort((a, b) => tierOrder(a.tier) - tierOrder(b.tier));
}

export function BanScanPanel() {
    const [scanning, setScanning] = useState(false);
    const [reports, setReports] = useState<main.SlotBanReport[] | null>(null);
    const [checksOpen, setChecksOpen] = useState(false);

    const handleScan = async () => {
        setScanning(true);
        try {
            const result = await ScanBanRisk();
            setReports(result ?? []);
        } catch (e) {
            toast.error('Scan failed: ' + e);
        } finally {
            setScanning(false);
        }
    };

    const handleExport = () => {
        if (!reports) return;
        const json = JSON.stringify(reports, null, 2);
        const blob = new Blob([json], {type: 'application/json'});
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'ban-risk-report.json';
        a.click();
        URL.revokeObjectURL(url);
    };

    const totalFindings = reports?.reduce((s, r) => s + r.findings.length, 0) ?? 0;
    const isClean = reports !== null && totalFindings === 0;

    return (
        <div className="space-y-4 animate-in fade-in duration-300">
            {/* Header row */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <div className="w-1 h-3 bg-red-400 rounded-full" />
                    <h3 className="text-[11px] font-black uppercase tracking-widest text-muted-foreground">
                        Ban Risk Scanner
                    </h3>
                </div>
                <div className="flex items-center gap-2">
                    {reports !== null && (
                        <button
                            onClick={handleExport}
                            className="px-2 py-1 text-[11px] font-semibold rounded border border-muted/40 text-muted-foreground hover:text-foreground hover:border-muted/70 transition-colors">
                            Export JSON
                        </button>
                    )}
                    <button
                        onClick={handleScan}
                        disabled={scanning}
                        className="px-3 py-1.5 text-[11px] font-black uppercase tracking-widest rounded border border-red-500/40 bg-red-500/10 text-red-300 hover:bg-red-500/20 disabled:opacity-50 disabled:cursor-not-allowed transition-all">
                        {scanning ? 'Scanning…' : 'Scan All Slots'}
                    </button>
                </div>
            </div>

            {/* Checks performed — collapsible */}
            <div className="card overflow-hidden">
                <button
                    onClick={() => setChecksOpen(o => !o)}
                    className="w-full flex items-center justify-between px-4 py-3 hover:bg-muted/10 transition-colors">
                    <div className="flex items-center gap-2">
                        <svg className="w-3.5 h-3.5 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                        </svg>
                        <span className="text-[11px] font-black uppercase tracking-wider text-muted-foreground">Checks performed</span>
                        <span className="text-[11px] text-muted-foreground">
                            — {CHECKS_PERFORMED.reduce((s, g) => s + g.items.length, 0)} checks across {CHECKS_PERFORMED.length} categories
                        </span>
                    </div>
                    <svg
                        className={`w-3.5 h-3.5 text-muted-foreground transition-transform duration-200 ${checksOpen ? 'rotate-180' : ''}`}
                        fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M19 9l-7 7-7-7" />
                    </svg>
                </button>

                {checksOpen && (
                    <div className="px-4 pb-4 space-y-4 border-t border-border/30 pt-3">
                        {CHECKS_PERFORMED.map(group => (
                            <div key={group.group}>
                                <p className="text-[11px] font-black uppercase tracking-wider text-muted-foreground mb-2">{group.group}</p>
                                <div className="space-y-1.5">
                                    {group.items.map(item => (
                                        <div key={item.label} className="flex items-start gap-2">
                                            <div className={`mt-1.5 w-1.5 h-1.5 rounded-full flex-shrink-0 ${item.tier === 'high' ? 'bg-red-400' : 'bg-orange-400'}`} />
                                            <div>
                                                <span className="text-[11px] font-semibold text-foreground">{item.label}</span>
                                                <span className="text-[11px] text-muted-foreground"> — {item.desc}</span>
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {/* Clean result */}
            {isClean && (
                <div className="card p-5 flex items-center gap-3">
                    <div className="w-8 h-8 rounded-full bg-green-500/15 flex items-center justify-center flex-shrink-0">
                        <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M5 13l4 4L19 7" />
                        </svg>
                    </div>
                    <div>
                        <p className="text-[12px] font-bold text-green-300">No findings</p>
                        <p className="text-[11px] text-muted-foreground mt-0.5">All active slots passed the ban-risk check.</p>
                    </div>
                </div>
            )}

            {/* Per-slot cards */}
            {reports !== null && reports.map(report => (
                <div key={report.slotIndex} className="card p-4 space-y-3">
                    {/* Slot header */}
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                            <span className="text-[11px] font-black uppercase tracking-widest text-muted-foreground">
                                Slot {report.slotIndex + 1}
                            </span>
                            <span className="text-[13px] font-bold text-foreground">{report.charName}</span>
                            <span className="text-[11px] text-muted-foreground">Lv {report.level}</span>
                        </div>
                        {report.findings.length === 0 ? (
                            <span className="text-[10px] font-bold uppercase tracking-widest px-2 py-0.5 rounded bg-green-500/10 text-green-400 border border-green-500/20">
                                Clean
                            </span>
                        ) : (
                            <span className="text-[10px] font-bold uppercase tracking-widest px-2 py-0.5 rounded bg-red-500/10 text-red-400 border border-red-500/20">
                                {report.findings.length} finding{report.findings.length !== 1 ? 's' : ''}
                            </span>
                        )}
                    </div>

                    {/* Findings list */}
                    {report.findings.length > 0 && (
                        <div className="space-y-1.5">
                            {sortedFindings(report.findings).map((finding, idx) => {
                                const style = TIER_STYLES[finding.tier] ?? TIER_STYLES.info;
                                const label = CATEGORY_LABELS[finding.category] ?? finding.category;
                                const riskKey = KNOWN_RISK_KEYS.has(finding.category) ? finding.category as RiskKey : null;
                                return (
                                    <div key={idx} className="flex items-start gap-2 py-2 px-3 rounded bg-muted/20">
                                        <div className={`mt-1.5 w-1.5 h-1.5 rounded-full flex-shrink-0 ${style.dot}`} />
                                        <div className="flex-1 min-w-0">
                                            <div className="flex items-center gap-1.5 flex-wrap">
                                                <span className={`text-[10px] font-black uppercase tracking-wider px-1.5 py-0.5 rounded ${style.badge}`}>
                                                    {label}
                                                </span>
                                                {riskKey && <RiskInfoIcon riskKey={riskKey} />}
                                            </div>
                                            <p className="text-[11px] text-muted-foreground mt-1 break-words">{finding.detail}</p>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </div>
            ))}
        </div>
    );
}
