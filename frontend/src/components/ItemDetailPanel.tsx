import {useState} from 'react';
import {db} from '../../wailsjs/go/models';

interface ItemDetailPanelProps {
    item: db.ItemEntry;
    onClose: () => void;
}

export function ItemDetailPanel({item, onClose}: ItemDetailPanelProps) {
    const [brokenIcon, setBrokenIcon] = useState(false);
    const [iconPreview, setIconPreview] = useState(false);

    const V = (v: number | undefined) => v != null ? String(v) : 'N/A';
    const VF = (v: number | undefined) => v != null ? v.toFixed(1) : 'N/A';

    // Phase 3B.4: prefer the generated text payload (item.text) when present,
    // fall back to legacy item.description / item.location. App-curated
    // item.name is kept as the panel title so disambiguated entries like
    // "Letter from Volcano Manor (Istvan)" do not regress to the bare FMG
    // canonical name.
    const text = item.text;
    const caption = text?.Caption?.trim() || '';
    const description = (text?.Description?.trim()) || (item.description?.trim() ?? '');
    const location = (text?.Location?.trim()) || (item.location?.trim() ?? '');

    const isWeaponCategory = ['melee_armaments', 'ranged_and_catalysts', 'shields'].includes(item.category);
    const isArmorCategory = ['head', 'chest', 'arms', 'legs'].includes(item.category);
    const isSpellCategory = ['sorceries', 'incantations'].includes(item.category);

    const showWeapon = item.weapon || isWeaponCategory;
    const showArmor = item.armor || isArmorCategory;
    const showSpell = item.spell || isSpellCategory;

    // Phase 3C.4: prefer the typed Phase 3C.3 stats payload (item.stats.weapon)
    // for weapon-like items; legacy item.weapon stays the fallback so items
    // covered only by descriptions.go (outside the V1 generator's four
    // weapon-like categories) keep rendering as before.
    //
    // R-STA-01: V1.AttackHoly / V1.GuardHoly are sourced from Elden Ring's
    // legacy "Dark"-named CSV columns. The backend already performed the
    // Dark→Holy rename — the UI must surface "Holy" only, never "Dark".
    const v1Weapon = item.stats?.kind === 'weapon' ? item.stats.weapon : undefined;
    const legacyWeapon = item.weapon;

    // Nullish-aware preference: V1 number 0 is a valid value (e.g. Longsword
    // Holy damage), so use `??` rather than `||` to keep zeros from falling
    // through to the legacy field.
    const wAttackPhys = v1Weapon?.AttackPhysical ?? legacyWeapon?.PhysDamage;
    const wAttackMagic = v1Weapon?.AttackMagic ?? legacyWeapon?.MagDamage;
    const wAttackFire = v1Weapon?.AttackFire ?? legacyWeapon?.FireDamage;
    const wAttackLight = v1Weapon?.AttackLightning ?? legacyWeapon?.LitDamage;
    const wAttackHoly = v1Weapon?.AttackHoly ?? legacyWeapon?.HolyDamage;
    const wAttackStamina = v1Weapon?.AttackStamina;

    const wGuardPhys = v1Weapon?.GuardPhysical;
    const wGuardMagic = v1Weapon?.GuardMagic;
    const wGuardFire = v1Weapon?.GuardFire;
    const wGuardLight = v1Weapon?.GuardLightning;
    const wGuardHoly = v1Weapon?.GuardHoly;
    const wGuardBoost = v1Weapon?.GuardBoost;

    const wScaleStr = v1Weapon?.ScalingStrRaw ?? legacyWeapon?.ScaleStr;
    const wScaleDex = v1Weapon?.ScalingDexRaw ?? legacyWeapon?.ScaleDex;
    const wScaleInt = v1Weapon?.ScalingIntRaw ?? legacyWeapon?.ScaleInt;
    const wScaleFai = v1Weapon?.ScalingFaiRaw ?? legacyWeapon?.ScaleFai;
    const wScaleArc = v1Weapon?.ScalingArcRaw;

    const wReqStr = v1Weapon?.StatReqStr ?? legacyWeapon?.ReqStr;
    const wReqDex = v1Weapon?.StatReqDex ?? legacyWeapon?.ReqDex;
    const wReqInt = v1Weapon?.StatReqInt ?? legacyWeapon?.ReqInt;
    const wReqFai = v1Weapon?.StatReqFai ?? legacyWeapon?.ReqFai;
    const wReqArc = v1Weapon?.StatReqArc ?? legacyWeapon?.ReqArc;

    const wWeight = v1Weapon?.Weight ?? legacyWeapon?.Weight ?? item.armor?.Weight ?? item.weight ?? 0;

    // V1-only metadata for the Item Info section. Empty when no V1 payload
    // (e.g. non-weapon items, or weapon-like IDs outside V1 coverage).
    const v1MaxUpgrade = v1Weapon?.MaxUpgrade;
    const reinforcementLabel = v1Weapon
        ? v1Weapon.IsSomber
            ? 'Somber'
            : v1Weapon.IsInfusable
                ? 'Standard'
                : '—'
        : undefined;

    // Build Attack Power rows dynamically so Stamina only shows when V1
    // reports a non-zero value (most weapons leave it 0 — surfacing the row
    // unconditionally would clutter the panel).
    const attackRows: [string, number | undefined | 'N/A'][] = [
        ['Physical', wAttackPhys],
        ['Magic', wAttackMagic],
        ['Fire', wAttackFire],
        ['Lightning', wAttackLight],
        ['Holy', wAttackHoly],
    ];
    if (wAttackStamina != null && wAttackStamina > 0) {
        attackRows.push(['Stamina', wAttackStamina]);
    }
    attackRows.push(['Critical', 'N/A']);

    return (
        <div className="h-full flex flex-col border-l border-border bg-card overflow-hidden">
            {/* Header */}
            <div className="bg-card/95 backdrop-blur-md border-b border-border p-4 flex items-start gap-3 shrink-0">
                <div className="w-28 h-28 rounded-lg bg-muted/30 border border-border/50 flex items-center justify-center overflow-hidden shrink-0 cursor-pointer hover:border-primary/50 transition-all"
                    onClick={() => setIconPreview(true)}>
                    {brokenIcon ? (
                        <span className="text-2xl font-black text-muted-foreground/30">?</span>
                    ) : (
                        <img src={item.iconPath} alt="" className="w-20 h-20 object-contain drop-shadow-md" onError={() => setBrokenIcon(true)} />
                    )}
                </div>
                <div className="flex-1 min-w-0">
                    <h3 className="text-[11px] font-black uppercase tracking-widest text-foreground truncate">{item.name}</h3>
                    <p className="text-[8px] font-bold text-muted-foreground uppercase tracking-widest mt-0.5">
                        {item.category.replace(/_/g, ' ')}
                    </p>
                    <p className="text-[8px] font-mono text-muted-foreground/60 mt-0.5">
                        0x{item.id.toString(16).toUpperCase()}
                    </p>
                </div>
                <button onClick={onClose}
                    className="p-1 rounded-md hover:bg-muted/50 text-muted-foreground hover:text-foreground transition-all shrink-0">
                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.5" d="M6 18L18 6M6 6l12 12"/></svg>
                </button>
            </div>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-4 space-y-4">
                {/* Sub-category + Weight row */}
                <div className="flex items-center justify-between">
                    {item.subCategory && (
                        <div>
                            <span className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Type </span>
                            <span className="text-[10px] font-bold text-foreground">{item.subCategory}</span>
                        </div>
                    )}
                    <div>
                        <span className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Weight </span>
                        <span className="text-[10px] font-bold text-foreground">
                            {wWeight}
                        </span>
                    </div>
                </div>

                {/* Caption — short FMG flavour text shown above Description when present */}
                {caption && (
                    <div className="space-y-1.5">
                        <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Caption</h4>
                        <p className="text-[10px] leading-relaxed italic text-foreground/70 whitespace-pre-line">
                            {caption}
                        </p>
                    </div>
                )}

                {/* Description */}
                {description && (
                    <div className="space-y-1.5">
                        <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Description</h4>
                        <p className="text-[10px] leading-relaxed text-foreground/80 whitespace-pre-line">
                            {description}
                        </p>
                    </div>
                )}

                {/* Location — curated source, surfaced via item.text.Location with legacy fallback */}
                {location && (
                    <div className="space-y-1.5">
                        <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Location</h4>
                        <p className="text-[10px] leading-relaxed text-foreground/80 whitespace-pre-line">
                            {location}
                        </p>
                    </div>
                )}

                {/* Weapon Stats */}
                {showWeapon && (
                    <div className="space-y-3">
                        {!v1Weapon && !legacyWeapon && (
                            <p className="text-[8px] font-bold uppercase tracking-widest text-amber-500/80 text-center">stats data missing</p>
                        )}
                        <div className="grid grid-cols-2 gap-3">
                            <div className="space-y-1">
                                <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Attack Power</h4>
                                <table className="w-full text-[9px]">
                                    <tbody>
                                        {attackRows.map(([label, raw]) => {
                                            const val = raw === 'N/A' ? 'N/A' : V(raw);
                                            return (
                                                <tr key={label} className="border-b border-border/20">
                                                    <td className="py-0.5 text-muted-foreground font-medium">{label}</td>
                                                    <td className={`py-0.5 text-right font-black ${val === 'N/A' ? 'text-muted-foreground/40' : 'text-foreground'}`}>{val}</td>
                                                </tr>
                                            );
                                        })}
                                    </tbody>
                                </table>
                            </div>
                            <div className="space-y-1">
                                <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Guarded Dmg Negation</h4>
                                <table className="w-full text-[9px]">
                                    <tbody>
                                        {([
                                            ['Physical', V(wGuardPhys)],
                                            ['Magic', V(wGuardMagic)],
                                            ['Fire', V(wGuardFire)],
                                            ['Lightning', V(wGuardLight)],
                                            ['Holy', V(wGuardHoly)],
                                            ['Guard Boost', V(wGuardBoost)],
                                        ] as [string, string][]).map(([label, val]) => (
                                            <tr key={label} className="border-b border-border/20">
                                                <td className="py-0.5 text-muted-foreground font-medium">{label}</td>
                                                <td className={`py-0.5 text-right font-black ${val === 'N/A' ? 'text-muted-foreground/40' : 'text-foreground'}`}>{val}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        </div>
                        <div className="grid grid-cols-2 gap-3">
                            <div className="space-y-1">
                                <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Attribute Scaling</h4>
                                <table className="w-full text-[9px]">
                                    <tbody>
                                        {([
                                            ['Str', V(wScaleStr)],
                                            ['Dex', V(wScaleDex)],
                                            ['Int', V(wScaleInt)],
                                            ['Fai', V(wScaleFai)],
                                            ['Arc', V(wScaleArc)],
                                        ] as [string, string][]).map(([label, val]) => (
                                            <tr key={label} className="border-b border-border/20">
                                                <td className="py-0.5 text-muted-foreground font-medium">{label}</td>
                                                <td className={`py-0.5 text-right font-black ${val === 'N/A' ? 'text-muted-foreground/40' : 'text-foreground'}`}>{val}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                            <div className="space-y-1">
                                <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Attributes Required</h4>
                                <table className="w-full text-[9px]">
                                    <tbody>
                                        {([
                                            ['Str', V(wReqStr)],
                                            ['Dex', V(wReqDex)],
                                            ['Int', V(wReqInt)],
                                            ['Fai', V(wReqFai)],
                                            ['Arc', V(wReqArc)],
                                        ] as [string, string][]).map(([label, val]) => (
                                            <tr key={label} className="border-b border-border/20">
                                                <td className="py-0.5 text-muted-foreground font-medium">{label}</td>
                                                <td className={`py-0.5 text-right font-black ${val === 'N/A' ? 'text-muted-foreground/40' : 'text-foreground'}`}>{val}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    </div>
                )}

                {showArmor && (() => {
                    const a = item.armor;
                    return (
                    <div className="space-y-3">
                        {!a && (
                            <p className="text-[8px] font-bold uppercase tracking-widest text-amber-500/80 text-center">stats data missing</p>
                        )}
                        <div className="grid grid-cols-2 gap-3">
                            <div className="space-y-1">
                                <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Damage Negation</h4>
                                <table className="w-full text-[9px]">
                                    <tbody>
                                        {([
                                            ['Physical', VF(a?.Physical)],
                                            ['Strike', VF(a?.Strike)],
                                            ['Slash', VF(a?.Slash)],
                                            ['Pierce', VF(a?.Pierce)],
                                            ['Magic', VF(a?.Magic)],
                                            ['Fire', VF(a?.Fire)],
                                            ['Lightning', VF(a?.Lightning)],
                                            ['Holy', VF(a?.Holy)],
                                        ] as [string, string][]).map(([label, val]) => (
                                            <tr key={label} className="border-b border-border/20">
                                                <td className="py-0.5 text-muted-foreground font-medium">{label}</td>
                                                <td className={`py-0.5 text-right font-black ${val === 'N/A' ? 'text-muted-foreground/40' : 'text-foreground'}`}>{val}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                            <div className="space-y-1">
                                <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Resistance</h4>
                                <table className="w-full text-[9px]">
                                    <tbody>
                                        {([
                                            ['Immunity', V(a?.Immunity)],
                                            ['Robustness', V(a?.Robustness)],
                                            ['Focus', V(a?.Focus)],
                                            ['Vitality', V(a?.Vitality)],
                                        ] as [string, string][]).map(([label, val]) => (
                                            <tr key={label} className="border-b border-border/20">
                                                <td className="py-0.5 text-muted-foreground font-medium">{label}</td>
                                                <td className={`py-0.5 text-right font-black ${val === 'N/A' ? 'text-muted-foreground/40' : 'text-foreground'}`}>{val}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                                <div className="pt-0.5">
                                    <span className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Poise </span>
                                    <span className={`text-[10px] font-bold ${a ? 'text-foreground' : 'text-muted-foreground/40'}`}>{VF(a?.Poise)}</span>
                                </div>
                            </div>
                        </div>
                    </div>
                    );
                })()}

                {showSpell && (() => {
                    const sp = item.spell;
                    return (
                    <div className="space-y-3">
                        {!sp && (
                            <p className="text-[8px] font-bold uppercase tracking-widest text-amber-500/80 text-center">stats data missing</p>
                        )}
                        <div className="grid grid-cols-2 gap-3">
                            <div className="space-y-1">
                                <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Spell Info</h4>
                                <table className="w-full text-[9px]">
                                    <tbody>
                                        {([
                                            ['FP Cost', V(sp?.FPCost)],
                                            ['Slots', V(sp?.Slots)],
                                        ] as [string, string][]).map(([label, val]) => (
                                            <tr key={label} className="border-b border-border/20">
                                                <td className="py-0.5 text-muted-foreground font-medium">{label}</td>
                                                <td className={`py-0.5 text-right font-black ${val === 'N/A' ? 'text-muted-foreground/40' : 'text-foreground'}`}>{val}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                            <div className="space-y-1">
                                <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Attributes Required</h4>
                                <table className="w-full text-[9px]">
                                    <tbody>
                                        {([
                                            ['Int', V(sp?.ReqInt)],
                                            ['Fai', V(sp?.ReqFai)],
                                            ['Arc', V(sp?.ReqArc)],
                                        ] as [string, string][]).map(([label, val]) => (
                                            <tr key={label} className="border-b border-border/20">
                                                <td className="py-0.5 text-muted-foreground font-medium">{label}</td>
                                                <td className={`py-0.5 text-right font-black ${val === 'N/A' ? 'text-muted-foreground/40' : 'text-foreground'}`}>{val}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    </div>
                    );
                })()}

                {/* Item info */}
                <div className="space-y-1.5 pt-2 border-t border-border/30">
                    <h4 className="text-[8px] font-black uppercase tracking-widest text-muted-foreground">Item Info</h4>
                    <div className="grid grid-cols-2 gap-1.5 text-[9px]">
                        <div className="flex justify-between bg-muted/10 rounded px-2 py-1">
                            <span className="text-muted-foreground font-bold">Max Inventory</span>
                            <span className="font-black text-foreground">{item.maxInventory}</span>
                        </div>
                        <div className="flex justify-between bg-muted/10 rounded px-2 py-1">
                            <span className="text-muted-foreground font-bold">Max Storage</span>
                            <span className="font-black text-foreground">{item.maxStorage}</span>
                        </div>
                        {(v1MaxUpgrade != null ? v1MaxUpgrade : item.maxUpgrade) > 0 && (
                            <div className="flex justify-between bg-muted/10 rounded px-2 py-1">
                                <span className="text-muted-foreground font-bold">Max Upgrade</span>
                                <span className="font-black text-foreground">+{v1MaxUpgrade ?? item.maxUpgrade}</span>
                            </div>
                        )}
                        {reinforcementLabel && reinforcementLabel !== '—' && (
                            <div className="flex justify-between bg-muted/10 rounded px-2 py-1">
                                <span className="text-muted-foreground font-bold">Reinforcement</span>
                                <span className="font-black text-foreground">{reinforcementLabel}</span>
                            </div>
                        )}
                    </div>
                </div>

                {/* No data fallback */}
                {!description && !caption && !location && !showWeapon && !showArmor && !showSpell && (
                    <p className="text-[9px] text-muted-foreground/60 italic">No description or stats available for this item.</p>
                )}
            </div>

            {/* Icon Preview Modal */}
            {iconPreview && (
                <div className="fixed inset-0 bg-background/80 backdrop-blur-xl z-[100] flex items-center justify-center p-8 animate-in fade-in duration-300"
                    onClick={() => setIconPreview(false)}>
                    <div className="relative max-w-2xl w-full flex flex-col items-center space-y-8 animate-in zoom-in-95 duration-300">
                        <div className="w-64 h-64 bg-muted/20 rounded-3xl border border-border/50 flex items-center justify-center shadow-2xl shadow-primary/10 relative group">
                            <div className="absolute inset-0 bg-primary/5 rounded-3xl blur-3xl group-hover:bg-primary/10 transition-all duration-500" />
                            {brokenIcon ? (
                                <span className="text-3xl font-black text-muted-foreground/30 select-none">?</span>
                            ) : (
                                <img src={item.iconPath} alt={item.name} className="w-48 h-48 object-contain drop-shadow-2xl relative z-10" />
                            )}
                        </div>
                        <div className="text-center space-y-2">
                            <h3 className="text-2xl font-black uppercase tracking-[0.2em] text-foreground">{item.name}</h3>
                            <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-[0.3em]">{item.iconPath}</p>
                        </div>
                        <button className="px-8 py-3 bg-primary text-primary-foreground rounded-full text-[10px] font-black uppercase tracking-[0.2em] shadow-xl shadow-primary/20 hover:scale-105 active:scale-95 transition-all">
                            Close Preview
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
