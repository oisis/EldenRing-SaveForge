import { useState } from 'react';
import type { main } from '../../../wailsjs/go/models';

// Presentational blocking modal for load-time inventory integrity gate.
// All decisions stay in App.tsx; this component only renders the report
// and reports the user's intent via callbacks.
interface InventoryIntegrityModalProps {
    report: main.SaveInventoryIntegrityReport;
    busy: boolean;
    statusMessage?: string;
    errorMessage?: string;
    onRepair: () => void;
    onCloseSave: () => void;
}

type IntegrityConflictWithKind = main.InventoryIntegrityConflict & { kind?: string };

function formatHex(value: number): string {
    return '0x' + value.toString(16).toUpperCase().padStart(8, '0');
}

function slotHeading(slot: main.SlotInventoryIntegrityReport): string {
    if (!slot.active) {
        return `Inactive residual slot ${slot.slotIndex + 1}`;
    }
    if (slot.characterName) {
        return `Slot ${slot.slotIndex + 1} — ${slot.characterName}`;
    }
    return `Slot ${slot.slotIndex + 1}`;
}

function itemLabel(item: main.InventoryIntegrityConflictItem): string {
    if (item.unknown || !item.name) {
        return `Unknown item · ItemID ${formatHex(item.itemId)} · Handle ${formatHex(item.handle)}`;
    }

    const parts = [item.name];
    if (item.category) parts.push(item.category);
    if (item.currentUpgrade && item.currentUpgrade > 0) parts[0] = `${item.name} +${item.currentUpgrade}`;
    if (item.infusionName) parts.push(`Infusion: ${item.infusionName}`);
    if (item.quantity && item.quantity > 1) parts.push(`x${item.quantity}`);
    return parts.join(' · ');
}

function conflictLabel(conflict: main.InventoryIntegrityConflict): string {
    const kind = (conflict as IntegrityConflictWithKind).kind;
    if (kind === 'duplicate_physick') {
        return 'Duplicate Flask of Wondrous Physick';
    }
    return `Acquisition index ${conflict.index}`;
}

function duplicateSummary(slot: main.SlotInventoryIntegrityReport): string {
    if (slot.conflictingIndexCount > 0) {
        const entryWord = slot.duplicateEntryCount === 1 ? 'entry' : 'entries';
        const indexWord = slot.conflictingIndexCount === 1 ? 'index' : 'indices';
        return `${slot.duplicateEntryCount} duplicate ${entryWord} across ${slot.conflictingIndexCount} conflicting ${indexWord}`;
    }

    const entryWord = slot.duplicateEntryCount === 1 ? 'entry' : 'entries';
    return `${slot.duplicateEntryCount} duplicate ${entryWord} requiring repair`;
}

export function InventoryIntegrityModal({
    report,
    busy,
    statusMessage,
    errorMessage,
    onRepair,
    onCloseSave,
}: InventoryIntegrityModalProps) {
    const [showAffected, setShowAffected] = useState(false);
    const hasResidual = report.slots.some(slot => !slot.active);

    return (
        <div
            className="fixed inset-0 z-[150] flex items-center justify-center bg-background/85 backdrop-blur-sm animate-in fade-in duration-300"
            role="dialog"
            aria-modal="true"
            aria-labelledby="inventory-integrity-title"
        >
            <div className="bg-card p-8 rounded-2xl border-2 border-amber-500/40 flex flex-col space-y-5 max-w-2xl w-full mx-4 shadow-2xl shadow-amber-500/20 animate-in zoom-in-95 duration-300 max-h-[85vh] overflow-hidden">
                <div className="flex items-center space-x-3">
                    <div className="w-10 h-10 rounded-full bg-amber-500/15 border border-amber-500/40 flex items-center justify-center">
                        <svg className="w-5 h-5 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M12 9v3m0 3h.01M4.93 19h14.14a2 2 0 001.74-3L13.74 4a2 2 0 00-3.48 0L3.19 16a2 2 0 001.74 3z" />
                        </svg>
                    </div>
                    <h3 id="inventory-integrity-title" className="text-sm font-black uppercase tracking-[0.15em] text-amber-500">
                        Inventory integrity issue detected
                    </h3>
                </div>

                <div className="space-y-2 text-[11px] text-foreground leading-relaxed">
                    <p>This save contains duplicated inventory acquisition indices or duplicate unique tools in one or more character slots.</p>
                    <p>This issue may have been created by an older version of SaveForge.</p>
                    <p>It may cause incorrect in-game inventory ordering or duplicate unique tools and should be repaired before editing this file.</p>
                    <p>Repair rebuilds duplicate acquisition indices and removes surplus Flask of Wondrous Physick records. It does not change quantities, weapon upgrades, infusions, Ashes of War, storage contents, or character progression.</p>
                    <p className="text-warning-foreground">Keep a backup of the original save before saving repaired changes.</p>
                </div>

                <div className="border border-border/50 rounded-md divide-y divide-border/50 overflow-y-auto custom-scrollbar">
                    {report.slots.map(slot => (
                        <div key={slot.slotIndex} className="p-3 text-[11px] space-y-1">
                            <div className="font-bold">{slotHeading(slot)}</div>
                            <div className="text-muted-foreground">{duplicateSummary(slot)}</div>
                        </div>
                    ))}
                </div>

                {hasResidual && (
                    <p className="text-[10px] text-muted-foreground leading-relaxed">
                        Some residual data belongs to a character slot not currently shown in the normal character list. It is included because it is still present in the loaded save file.
                    </p>
                )}

                <div>
                    <button
                        type="button"
                        onClick={() => setShowAffected(v => !v)}
                        className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors"
                    >
                        {showAffected ? 'Hide' : 'Show'} affected items
                    </button>

                    {showAffected && (
                        <div className="mt-2 max-h-64 overflow-y-auto custom-scrollbar border border-border/50 rounded-md p-3 space-y-3">
                            {report.slots.map(slot => (
                                <div key={slot.slotIndex} className="space-y-2">
                                    <div className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">
                                        {slotHeading(slot)}
                                    </div>
                                    {slot.conflicts.map((conflict, conflictIdx) => (
                                        <div key={`${conflictLabel(conflict)}-${conflict.index}-${conflictIdx}`} className="ml-2 text-[11px]">
                                            <div className="font-bold text-warning-foreground">{conflictLabel(conflict)}</div>
                                            <ul className="ml-3 list-disc space-y-0.5 text-foreground/80">
                                                {conflict.items.map((item, idx) => (
                                                    <li key={`${item.scope}-${item.row}-${idx}`}>
                                                        {itemLabel(item)}
                                                    </li>
                                                ))}
                                            </ul>
                                        </div>
                                    ))}
                                </div>
                            ))}
                        </div>
                    )}
                </div>

                {(statusMessage || errorMessage) && (
                    <div className="text-[11px] leading-relaxed">
                        {errorMessage ? <p className="text-red-400">{errorMessage}</p> : <p className="text-muted-foreground">{statusMessage}</p>}
                    </div>
                )}

                <div className="flex flex-col space-y-2">
                    <button
                        type="button"
                        onClick={onRepair}
                        disabled={busy}
                        className="w-full px-4 py-2.5 bg-amber-500 text-white rounded-md text-[10px] font-black uppercase tracking-widest hover:brightness-110 active:scale-95 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {busy ? 'Repairing...' : 'Repair duplicates'}
                    </button>
                    <button
                        type="button"
                        onClick={onCloseSave}
                        disabled={busy}
                        className="w-full px-4 py-2.5 bg-muted text-foreground rounded-md text-[10px] font-black uppercase tracking-widest hover:brightness-110 active:scale-95 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        Close save
                    </button>
                </div>
            </div>
        </div>
    );
}
