export namespace core {
	
	export class InventoryIndexRepairChange {
	    scope: string;
	    row: number;
	    handle: number;
	    oldIndex: number;
	    newIndex: number;
	
	    static createFrom(source: any = {}) {
	        return new InventoryIndexRepairChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.row = source["row"];
	        this.handle = source["handle"];
	        this.oldIndex = source["oldIndex"];
	        this.newIndex = source["newIndex"];
	    }
	}
	export class InventoryIndexRepairReport {
	    changed: number;
	    changes: InventoryIndexRepairChange[];
	
	    static createFrom(source: any = {}) {
	        return new InventoryIndexRepairReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changed = source["changed"];
	        this.changes = this.convertValues(source["changes"], InventoryIndexRepairChange);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NetworkParamValues {
	    maxBreakInTargetListCount: number;
	    breakInRequestIntervalTimeSec: number;
	    breakInRequestTimeOutSec: number;
	    breakInRequestAreaCount: number;
	    reloadSignIntervalTime2: number;
	    reloadSignTotalCount: number;
	    reloadSignCellCount: number;
	    updateSignIntervalTime: number;
	    singGetMax: number;
	    signDownloadSpan: number;
	    signUpdateSpan: number;
	    reloadVisitListCoolTime: number;
	    maxCoopBlueSummonCount: number;
	    maxVisitListCount: number;
	    reloadSearchCoopBlueMin: number;
	    reloadSearchCoopBlueMax: number;
	    allAreaSearchRateCoopBlue: number;
	    allAreaSearchRateVsBlue: number;
	    visitorListMax: number;
	    visitorTimeOutTime: number;
	    visitorDownloadSpan: number;
	
	    static createFrom(source: any = {}) {
	        return new NetworkParamValues(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxBreakInTargetListCount = source["maxBreakInTargetListCount"];
	        this.breakInRequestIntervalTimeSec = source["breakInRequestIntervalTimeSec"];
	        this.breakInRequestTimeOutSec = source["breakInRequestTimeOutSec"];
	        this.breakInRequestAreaCount = source["breakInRequestAreaCount"];
	        this.reloadSignIntervalTime2 = source["reloadSignIntervalTime2"];
	        this.reloadSignTotalCount = source["reloadSignTotalCount"];
	        this.reloadSignCellCount = source["reloadSignCellCount"];
	        this.updateSignIntervalTime = source["updateSignIntervalTime"];
	        this.singGetMax = source["singGetMax"];
	        this.signDownloadSpan = source["signDownloadSpan"];
	        this.signUpdateSpan = source["signUpdateSpan"];
	        this.reloadVisitListCoolTime = source["reloadVisitListCoolTime"];
	        this.maxCoopBlueSummonCount = source["maxCoopBlueSummonCount"];
	        this.maxVisitListCount = source["maxVisitListCount"];
	        this.reloadSearchCoopBlueMin = source["reloadSearchCoopBlueMin"];
	        this.reloadSearchCoopBlueMax = source["reloadSearchCoopBlueMax"];
	        this.allAreaSearchRateCoopBlue = source["allAreaSearchRateCoopBlue"];
	        this.allAreaSearchRateVsBlue = source["allAreaSearchRateVsBlue"];
	        this.visitorListMax = source["visitorListMax"];
	        this.visitorTimeOutTime = source["visitorTimeOutTime"];
	        this.visitorDownloadSpan = source["visitorDownloadSpan"];
	    }
	}
	export class TransferSkip {
	    handle: number;
	    reason: string;
	    movedQty?: number;
	    remainingQty?: number;
	
	    static createFrom(source: any = {}) {
	        return new TransferSkip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.handle = source["handle"];
	        this.reason = source["reason"];
	        this.movedQty = source["movedQty"];
	        this.remainingQty = source["remainingQty"];
	    }
	}
	export class TransferResult {
	    moved: number;
	    skipped: TransferSkip[];
	
	    static createFrom(source: any = {}) {
	        return new TransferResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.moved = source["moved"];
	        this.skipped = this.convertValues(source["skipped"], TransferSkip);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace data {
	
	export class ArmorStats {
	    Weight: number;
	    Physical: number;
	    Strike: number;
	    Slash: number;
	    Pierce: number;
	    Magic: number;
	    Fire: number;
	    Lightning: number;
	    Holy: number;
	    Immunity: number;
	    Robustness: number;
	    Focus: number;
	    Vitality: number;
	    Poise: number;
	
	    static createFrom(source: any = {}) {
	        return new ArmorStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Weight = source["Weight"];
	        this.Physical = source["Physical"];
	        this.Strike = source["Strike"];
	        this.Slash = source["Slash"];
	        this.Pierce = source["Pierce"];
	        this.Magic = source["Magic"];
	        this.Fire = source["Fire"];
	        this.Lightning = source["Lightning"];
	        this.Holy = source["Holy"];
	        this.Immunity = source["Immunity"];
	        this.Robustness = source["Robustness"];
	        this.Focus = source["Focus"];
	        this.Vitality = source["Vitality"];
	        this.Poise = source["Poise"];
	    }
	}
	export class WeaponPassiveEffect {
	    Kind: string;
	    Source: string;
	    SpEffectID: number;
	    Label: string;
	    Value: number;
	    Known: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WeaponPassiveEffect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Kind = source["Kind"];
	        this.Source = source["Source"];
	        this.SpEffectID = source["SpEffectID"];
	        this.Label = source["Label"];
	        this.Value = source["Value"];
	        this.Known = source["Known"];
	    }
	}
	export class WeaponStatsV1 {
	    ItemID: number;
	    WepType: number;
	    SortGroupID: number;
	    ReinforceTypeID: number;
	    GemMountType: number;
	    Weight: number;
	    AttackPhysical: number;
	    AttackMagic: number;
	    AttackFire: number;
	    AttackLightning: number;
	    AttackHoly: number;
	    AttackStamina: number;
	    GuardPhysical: number;
	    GuardMagic: number;
	    GuardFire: number;
	    GuardLightning: number;
	    GuardHoly: number;
	    GuardBoost: number;
	    StatReqStr: number;
	    StatReqDex: number;
	    StatReqInt: number;
	    StatReqFai: number;
	    StatReqArc: number;
	    Critical: number;
	    ScalingStrRaw: number;
	    ScalingDexRaw: number;
	    ScalingIntRaw: number;
	    ScalingFaiRaw: number;
	    ScalingArcRaw: number;
	    StatusPoison: number;
	    StatusBleed: number;
	    StatusFrost: number;
	    StatusSleep: number;
	    StatusMadness: number;
	    StatusScarletRot: number;
	    PassiveEffects: WeaponPassiveEffect[];
	    DefaultAoWID: number;
	    IsInfusable: boolean;
	    IsSomber: boolean;
	    MaxUpgrade: number;
	    SourceRowID: number;
	    Warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new WeaponStatsV1(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ItemID = source["ItemID"];
	        this.WepType = source["WepType"];
	        this.SortGroupID = source["SortGroupID"];
	        this.ReinforceTypeID = source["ReinforceTypeID"];
	        this.GemMountType = source["GemMountType"];
	        this.Weight = source["Weight"];
	        this.AttackPhysical = source["AttackPhysical"];
	        this.AttackMagic = source["AttackMagic"];
	        this.AttackFire = source["AttackFire"];
	        this.AttackLightning = source["AttackLightning"];
	        this.AttackHoly = source["AttackHoly"];
	        this.AttackStamina = source["AttackStamina"];
	        this.GuardPhysical = source["GuardPhysical"];
	        this.GuardMagic = source["GuardMagic"];
	        this.GuardFire = source["GuardFire"];
	        this.GuardLightning = source["GuardLightning"];
	        this.GuardHoly = source["GuardHoly"];
	        this.GuardBoost = source["GuardBoost"];
	        this.StatReqStr = source["StatReqStr"];
	        this.StatReqDex = source["StatReqDex"];
	        this.StatReqInt = source["StatReqInt"];
	        this.StatReqFai = source["StatReqFai"];
	        this.StatReqArc = source["StatReqArc"];
	        this.Critical = source["Critical"];
	        this.ScalingStrRaw = source["ScalingStrRaw"];
	        this.ScalingDexRaw = source["ScalingDexRaw"];
	        this.ScalingIntRaw = source["ScalingIntRaw"];
	        this.ScalingFaiRaw = source["ScalingFaiRaw"];
	        this.ScalingArcRaw = source["ScalingArcRaw"];
	        this.StatusPoison = source["StatusPoison"];
	        this.StatusBleed = source["StatusBleed"];
	        this.StatusFrost = source["StatusFrost"];
	        this.StatusSleep = source["StatusSleep"];
	        this.StatusMadness = source["StatusMadness"];
	        this.StatusScarletRot = source["StatusScarletRot"];
	        this.PassiveEffects = this.convertValues(source["PassiveEffects"], WeaponPassiveEffect);
	        this.DefaultAoWID = source["DefaultAoWID"];
	        this.IsInfusable = source["IsInfusable"];
	        this.IsSomber = source["IsSomber"];
	        this.MaxUpgrade = source["MaxUpgrade"];
	        this.SourceRowID = source["SourceRowID"];
	        this.Warnings = source["Warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ItemStatsData {
	    kind: string;
	    weapon?: WeaponStatsV1;
	    sourceParam?: string;
	    sourceRowId?: number;
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ItemStatsData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.weapon = this.convertValues(source["weapon"], WeaponStatsV1);
	        this.sourceParam = source["sourceParam"];
	        this.sourceRowId = source["sourceRowId"];
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ItemTextData {
	    DisplayName: string;
	    CanonicalName: string;
	    Caption: string;
	    Description: string;
	    Location: string;
	    DisplayNameSource: string;
	    CanonicalSource: string;
	    CaptionSource: string;
	    DescriptionSource: string;
	    LocationSource: string;
	    DLCSource: string;
	    Notes: string;
	
	    static createFrom(source: any = {}) {
	        return new ItemTextData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.DisplayName = source["DisplayName"];
	        this.CanonicalName = source["CanonicalName"];
	        this.Caption = source["Caption"];
	        this.Description = source["Description"];
	        this.Location = source["Location"];
	        this.DisplayNameSource = source["DisplayNameSource"];
	        this.CanonicalSource = source["CanonicalSource"];
	        this.CaptionSource = source["CaptionSource"];
	        this.DescriptionSource = source["DescriptionSource"];
	        this.LocationSource = source["LocationSource"];
	        this.DLCSource = source["DLCSource"];
	        this.Notes = source["Notes"];
	    }
	}
	export class SpellStats {
	    FPCost: number;
	    Slots: number;
	    ReqInt: number;
	    ReqFai: number;
	    ReqArc: number;
	
	    static createFrom(source: any = {}) {
	        return new SpellStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FPCost = source["FPCost"];
	        this.Slots = source["Slots"];
	        this.ReqInt = source["ReqInt"];
	        this.ReqFai = source["ReqFai"];
	        this.ReqArc = source["ReqArc"];
	    }
	}
	
	export class WeaponStats {
	    Weight: number;
	    PhysDamage: number;
	    MagDamage: number;
	    FireDamage: number;
	    LitDamage: number;
	    HolyDamage: number;
	    ScaleStr: number;
	    ScaleDex: number;
	    ScaleInt: number;
	    ScaleFai: number;
	    ReqStr: number;
	    ReqDex: number;
	    ReqInt: number;
	    ReqFai: number;
	    ReqArc: number;
	
	    static createFrom(source: any = {}) {
	        return new WeaponStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Weight = source["Weight"];
	        this.PhysDamage = source["PhysDamage"];
	        this.MagDamage = source["MagDamage"];
	        this.FireDamage = source["FireDamage"];
	        this.LitDamage = source["LitDamage"];
	        this.HolyDamage = source["HolyDamage"];
	        this.ScaleStr = source["ScaleStr"];
	        this.ScaleDex = source["ScaleDex"];
	        this.ScaleInt = source["ScaleInt"];
	        this.ScaleFai = source["ScaleFai"];
	        this.ReqStr = source["ReqStr"];
	        this.ReqDex = source["ReqDex"];
	        this.ReqInt = source["ReqInt"];
	        this.ReqFai = source["ReqFai"];
	        this.ReqArc = source["ReqArc"];
	    }
	}

}

export namespace db {
	
	export class BellBearingEntry {
	    id: number;
	    name: string;
	    category: string;
	    unlocked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BellBearingEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.unlocked = source["unlocked"];
	    }
	}
	export class BossEntry {
	    id: number;
	    name: string;
	    region: string;
	    type: string;
	    remembrance: boolean;
	    defeated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BossEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.region = source["region"];
	        this.type = source["type"];
	        this.remembrance = source["remembrance"];
	        this.defeated = source["defeated"];
	    }
	}
	export class ClassStats {
	    id: number;
	    name: string;
	    level: number;
	    vigor: number;
	    mind: number;
	    endurance: number;
	    strength: number;
	    dexterity: number;
	    intelligence: number;
	    faith: number;
	    arcane: number;
	
	    static createFrom(source: any = {}) {
	        return new ClassStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.level = source["level"];
	        this.vigor = source["vigor"];
	        this.mind = source["mind"];
	        this.endurance = source["endurance"];
	        this.strength = source["strength"];
	        this.dexterity = source["dexterity"];
	        this.intelligence = source["intelligence"];
	        this.faith = source["faith"];
	        this.arcane = source["arcane"];
	    }
	}
	export class ColosseumEntry {
	    id: number;
	    name: string;
	    region: string;
	    unlocked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ColosseumEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.region = source["region"];
	        this.unlocked = source["unlocked"];
	    }
	}
	export class CookbookEntry {
	    id: number;
	    name: string;
	    category: string;
	    unlocked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CookbookEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.unlocked = source["unlocked"];
	    }
	}
	export class GestureEntry {
	    id: number;
	    name: string;
	    category: string;
	    unlocked: boolean;
	    flags: string[];
	
	    static createFrom(source: any = {}) {
	        return new GestureEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.unlocked = source["unlocked"];
	        this.flags = source["flags"];
	    }
	}
	export class GraceEntry {
	    id: number;
	    name: string;
	    region: string;
	    visited: boolean;
	    isBossArena: boolean;
	    dungeonType?: string;
	
	    static createFrom(source: any = {}) {
	        return new GraceEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.region = source["region"];
	        this.visited = source["visited"];
	        this.isBossArena = source["isBossArena"];
	        this.dungeonType = source["dungeonType"];
	    }
	}
	export class InfuseType {
	    name: string;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new InfuseType(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.offset = source["offset"];
	    }
	}
	export class ItemEntry {
	    id: number;
	    name: string;
	    category: string;
	    subCategory?: string;
	    maxInventory: number;
	    maxStorage: number;
	    maxUpgrade: number;
	    iconPath: string;
	    flags: string[];
	    description?: string;
	    location?: string;
	    weight?: number;
	    weapon?: data.WeaponStats;
	    armor?: data.ArmorStats;
	    spell?: data.SpellStats;
	    aowCompatBitmask?: number;
	    text?: data.ItemTextData;
	    stats?: data.ItemStatsData;
	
	    static createFrom(source: any = {}) {
	        return new ItemEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.subCategory = source["subCategory"];
	        this.maxInventory = source["maxInventory"];
	        this.maxStorage = source["maxStorage"];
	        this.maxUpgrade = source["maxUpgrade"];
	        this.iconPath = source["iconPath"];
	        this.flags = source["flags"];
	        this.description = source["description"];
	        this.location = source["location"];
	        this.weight = source["weight"];
	        this.weapon = this.convertValues(source["weapon"], data.WeaponStats);
	        this.armor = this.convertValues(source["armor"], data.ArmorStats);
	        this.spell = this.convertValues(source["spell"], data.SpellStats);
	        this.aowCompatBitmask = source["aowCompatBitmask"];
	        this.text = this.convertValues(source["text"], data.ItemTextData);
	        this.stats = this.convertValues(source["stats"], data.ItemStatsData);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MapEntry {
	    id: number;
	    name: string;
	    area: string;
	    category: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MapEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.area = source["area"];
	        this.category = source["category"];
	        this.enabled = source["enabled"];
	    }
	}
	export class QuestFlagState {
	    id: number;
	    target: number;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QuestFlagState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.target = source["target"];
	        this.current = source["current"];
	    }
	}
	export class QuestStep {
	    description: string;
	    location?: string;
	    flags: QuestFlagState[];
	    complete: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QuestStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.description = source["description"];
	        this.location = source["location"];
	        this.flags = this.convertValues(source["flags"], QuestFlagState);
	        this.complete = source["complete"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QuestNPC {
	    name: string;
	    steps: QuestStep[];
	
	    static createFrom(source: any = {}) {
	        return new QuestNPC(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.steps = this.convertValues(source["steps"], QuestStep);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RegionEntry {
	    id: number;
	    name: string;
	    area: string;
	    unlocked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RegionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.area = source["area"];
	        this.unlocked = source["unlocked"];
	    }
	}
	export class SummoningPoolEntry {
	    id: number;
	    name: string;
	    region: string;
	    activated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SummoningPoolEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.region = source["region"];
	        this.activated = source["activated"];
	    }
	}
	export class WhetbladeEntry {
	    id: number;
	    name: string;
	    unlocked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WhetbladeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.unlocked = source["unlocked"];
	    }
	}

}

export namespace deploy {
	
	export class Target {
	    type: string;
	    name: string;
	    host: string;
	    port: number;
	    user: string;
	    keyPath: string;
	    savePath: string;
	    gameStartCmd: string;
	    gameStopCmd: string;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.keyPath = source["keyPath"];
	        this.savePath = source["savePath"];
	        this.gameStartCmd = source["gameStartCmd"];
	        this.gameStopCmd = source["gameStopCmd"];
	    }
	}

}

export namespace editor {
	
	export class AddItemSpec {
	    itemID: number;
	    baseItemID: number;
	    quantity: number;
	    upgrade: number;
	    infusionName: string;
	    aowItemID: number;
	
	    static createFrom(source: any = {}) {
	        return new AddItemSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemID = source["itemID"];
	        this.baseItemID = source["baseItemID"];
	        this.quantity = source["quantity"];
	        this.upgrade = source["upgrade"];
	        this.infusionName = source["infusionName"];
	        this.aowItemID = source["aowItemID"];
	    }
	}
	export class EditableItem {
	    uid: string;
	    source: string;
	    container: string;
	    position: number;
	    originalHandle: number;
	    itemID: number;
	    baseItemID: number;
	    name: string;
	    category: string;
	    quantity: number;
	    acquisitionIndex: number;
	    currentUpgrade: number;
	    maxUpgrade: number;
	    infusionName?: string;
	    iconPath?: string;
	    hasGaItem: boolean;
	    isWeapon: boolean;
	    isArmor: boolean;
	    isTalisman: boolean;
	    wepType?: number;
	    canMountAoW?: boolean;
	    currentAoWHandle?: number;
	    currentAoWItemID?: number;
	    currentAoWName?: string;
	    hasCurrentAoW?: boolean;
	    currentAoWShared?: boolean;
	    currentAoWStatus?: string;
	    pendingAoWItemID?: number;
	    pendingAoWName?: string;
	    pendingAoWClear?: boolean;
	    hasPendingWeaponPatch?: boolean;
	    originalSlotIndex: number;
	
	    static createFrom(source: any = {}) {
	        return new EditableItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.source = source["source"];
	        this.container = source["container"];
	        this.position = source["position"];
	        this.originalHandle = source["originalHandle"];
	        this.itemID = source["itemID"];
	        this.baseItemID = source["baseItemID"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.quantity = source["quantity"];
	        this.acquisitionIndex = source["acquisitionIndex"];
	        this.currentUpgrade = source["currentUpgrade"];
	        this.maxUpgrade = source["maxUpgrade"];
	        this.infusionName = source["infusionName"];
	        this.iconPath = source["iconPath"];
	        this.hasGaItem = source["hasGaItem"];
	        this.isWeapon = source["isWeapon"];
	        this.isArmor = source["isArmor"];
	        this.isTalisman = source["isTalisman"];
	        this.wepType = source["wepType"];
	        this.canMountAoW = source["canMountAoW"];
	        this.currentAoWHandle = source["currentAoWHandle"];
	        this.currentAoWItemID = source["currentAoWItemID"];
	        this.currentAoWName = source["currentAoWName"];
	        this.hasCurrentAoW = source["hasCurrentAoW"];
	        this.currentAoWShared = source["currentAoWShared"];
	        this.currentAoWStatus = source["currentAoWStatus"];
	        this.pendingAoWItemID = source["pendingAoWItemID"];
	        this.pendingAoWName = source["pendingAoWName"];
	        this.pendingAoWClear = source["pendingAoWClear"];
	        this.hasPendingWeaponPatch = source["hasPendingWeaponPatch"];
	        this.originalSlotIndex = source["originalSlotIndex"];
	    }
	}
	export class WorkspaceValidationIssue {
	    severity: string;
	    code: string;
	    message: string;
	    uid?: string;
	    handle?: number;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceValidationIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.uid = source["uid"];
	        this.handle = source["handle"];
	    }
	}
	export class WorkspaceValidationReport {
	    ok: boolean;
	    errors: WorkspaceValidationIssue[];
	    warnings: WorkspaceValidationIssue[];
	    inventoryItemCount: number;
	    storageItemCount: number;
	    unsupportedInventoryCount: number;
	    unsupportedStorageCount: number;
	    duplicateUIDs: string[];
	    duplicateHandles: number[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceValidationReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.errors = this.convertValues(source["errors"], WorkspaceValidationIssue);
	        this.warnings = this.convertValues(source["warnings"], WorkspaceValidationIssue);
	        this.inventoryItemCount = source["inventoryItemCount"];
	        this.storageItemCount = source["storageItemCount"];
	        this.unsupportedInventoryCount = source["unsupportedInventoryCount"];
	        this.unsupportedStorageCount = source["unsupportedStorageCount"];
	        this.duplicateUIDs = source["duplicateUIDs"];
	        this.duplicateHandles = source["duplicateHandles"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RawInventoryRecord {
	    container: string;
	    slotIndex: number;
	    handle: number;
	    quantity: number;
	    acquisitionIndex: number;
	    itemID: number;
	    name?: string;
	    category?: string;
	    reason: string;
	    hasGaItem: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RawInventoryRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.container = source["container"];
	        this.slotIndex = source["slotIndex"];
	        this.handle = source["handle"];
	        this.quantity = source["quantity"];
	        this.acquisitionIndex = source["acquisitionIndex"];
	        this.itemID = source["itemID"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.reason = source["reason"];
	        this.hasGaItem = source["hasGaItem"];
	    }
	}
	export class InventoryWorkspaceSnapshot {
	    sessionID: string;
	    characterIndex: number;
	    inventoryItems: EditableItem[];
	    storageItems: EditableItem[];
	    unsupportedInventoryRecords: RawInventoryRecord[];
	    unsupportedStorageRecords: RawInventoryRecord[];
	    dirty: boolean;
	    validation: WorkspaceValidationReport;
	
	    static createFrom(source: any = {}) {
	        return new InventoryWorkspaceSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionID = source["sessionID"];
	        this.characterIndex = source["characterIndex"];
	        this.inventoryItems = this.convertValues(source["inventoryItems"], EditableItem);
	        this.storageItems = this.convertValues(source["storageItems"], EditableItem);
	        this.unsupportedInventoryRecords = this.convertValues(source["unsupportedInventoryRecords"], RawInventoryRecord);
	        this.unsupportedStorageRecords = this.convertValues(source["unsupportedStorageRecords"], RawInventoryRecord);
	        this.dirty = source["dirty"];
	        this.validation = this.convertValues(source["validation"], WorkspaceValidationReport);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class WeaponPatch {
	    setUpgrade: boolean;
	    upgrade: number;
	    setInfusionName: boolean;
	    infusionName: string;
	    setAoWItemID: boolean;
	    aowItemID: number;
	    clearAoW: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WeaponPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setUpgrade = source["setUpgrade"];
	        this.upgrade = source["upgrade"];
	        this.setInfusionName = source["setInfusionName"];
	        this.infusionName = source["infusionName"];
	        this.setAoWItemID = source["setAoWItemID"];
	        this.aowItemID = source["aowItemID"];
	        this.clearAoW = source["clearAoW"];
	    }
	}
	

}

export namespace main {
	
	export class ActiveInventoryEditSession {
	    active: boolean;
	    sessionID?: string;
	
	    static createFrom(source: any = {}) {
	        return new ActiveInventoryEditSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.sessionID = source["sessionID"];
	    }
	}
	export class SkippedAdd {
	    itemID: number;
	    cutQty: number;
	
	    static createFrom(source: any = {}) {
	        return new SkippedAdd(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemID = source["itemID"];
	        this.cutQty = source["cutQty"];
	    }
	}
	export class AddResult {
	    added: number;
	    requested: number;
	    trimmed: SkippedAdd[];
	    capHit: string;
	    freeInv: number;
	    freeStore: number;
	    neededInv: number;
	    neededStore: number;
	
	    static createFrom(source: any = {}) {
	        return new AddResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.requested = source["requested"];
	        this.trimmed = this.convertValues(source["trimmed"], SkippedAdd);
	        this.capHit = source["capHit"];
	        this.freeInv = source["freeInv"];
	        this.freeStore = source["freeStore"];
	        this.neededInv = source["neededInv"];
	        this.neededStore = source["neededStore"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WeaponLevelOverride {
	    enabled?: boolean;
	    standardLevel?: number;
	    somberLevel?: number;
	
	    static createFrom(source: any = {}) {
	        return new WeaponLevelOverride(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.standardLevel = source["standardLevel"];
	        this.somberLevel = source["somberLevel"];
	    }
	}
	export class ApplyTemplateOptions {
	    mode?: string;
	    weaponLevelOverride?: WeaponLevelOverride;
	
	    static createFrom(source: any = {}) {
	        return new ApplyTemplateOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.weaponLevelOverride = this.convertValues(source["weaponLevelOverride"], WeaponLevelOverride);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ApplyTemplateResult {
	    preview: templates.ImportPreviewReport;
	    workspace: editor.InventoryWorkspaceSnapshot;
	    applied: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ApplyTemplateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.preview = this.convertValues(source["preview"], templates.ImportPreviewReport);
	        this.workspace = this.convertValues(source["workspace"], editor.InventoryWorkspaceSnapshot);
	        this.applied = source["applied"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ApplyTemplateV2Options {
	    mode?: string;
	    sessionID?: string;
	    weaponLevelOverride?: WeaponLevelOverride;
	
	    static createFrom(source: any = {}) {
	        return new ApplyTemplateV2Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.sessionID = source["sessionID"];
	        this.weaponLevelOverride = this.convertValues(source["weaponLevelOverride"], WeaponLevelOverride);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ApplyTemplateV2Result {
	    preview: templates.ImportPreviewReport;
	    applied: boolean;
	    charIndex: number;
	    appliedFields: string[];
	    skippedFields: string[];
	    character?: vm.CharacterViewModel;
	    inventoryItemsApplied: number;
	    storageItemsApplied: number;
	    workspace?: editor.InventoryWorkspaceSnapshot;
	
	    static createFrom(source: any = {}) {
	        return new ApplyTemplateV2Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.preview = this.convertValues(source["preview"], templates.ImportPreviewReport);
	        this.applied = source["applied"];
	        this.charIndex = source["charIndex"];
	        this.appliedFields = source["appliedFields"];
	        this.skippedFields = source["skippedFields"];
	        this.character = this.convertValues(source["character"], vm.CharacterViewModel);
	        this.inventoryItemsApplied = source["inventoryItemsApplied"];
	        this.storageItemsApplied = source["storageItemsApplied"];
	        this.workspace = this.convertValues(source["workspace"], editor.InventoryWorkspaceSnapshot);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BuildTemplateExportOptions {
	    includeInventory: boolean;
	    includeStorage: boolean;
	    name: string;
	    description: string;
	    author: string;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new BuildTemplateExportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.includeInventory = source["includeInventory"];
	        this.includeStorage = source["includeStorage"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.author = source["author"];
	        this.tags = source["tags"];
	    }
	}
	export class BuildTemplateExportResult {
	    path?: string;
	    json?: string;
	    warnings?: templates.ExportWarning[];
	    skippedItems: number;
	
	    static createFrom(source: any = {}) {
	        return new BuildTemplateExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.json = source["json"];
	        this.warnings = this.convertValues(source["warnings"], templates.ExportWarning);
	        this.skippedItems = source["skippedItems"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BuildTemplateV2ExportOptions {
	    name: string;
	    description: string;
	    author: string;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new BuildTemplateV2ExportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.author = source["author"];
	        this.tags = source["tags"];
	    }
	}
	export class BuiltinCharacterPresetInfo {
	    id: string;
	    name: string;
	    description: string;
	    tags: string[];
	    modules: string[];
	    level: number;
	    className: string;
	
	    static createFrom(source: any = {}) {
	        return new BuiltinCharacterPresetInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.tags = source["tags"];
	        this.modules = source["modules"];
	        this.level = source["level"];
	        this.className = source["className"];
	    }
	}
	export class DiffEntry {
	    category: string;
	    action: string;
	    field: string;
	    oldValue: string;
	    newValue: string;
	
	    static createFrom(source: any = {}) {
	        return new DiffEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.action = source["action"];
	        this.field = source["field"];
	        this.oldValue = source["oldValue"];
	        this.newValue = source["newValue"];
	    }
	}
	export class FavoriteSlotInfo {
	    index: number;
	    active: boolean;
	    safe: boolean;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new FavoriteSlotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.active = source["active"];
	        this.safe = source["safe"];
	        this.name = source["name"];
	    }
	}
	export class InventoryIntegrityConflictItem {
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
	
	    static createFrom(source: any = {}) {
	        return new InventoryIntegrityConflictItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.row = source["row"];
	        this.handle = source["handle"];
	        this.itemId = source["itemId"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.quantity = source["quantity"];
	        this.currentUpgrade = source["currentUpgrade"];
	        this.infusionName = source["infusionName"];
	        this.unknown = source["unknown"];
	    }
	}
	export class InventoryIntegrityConflict {
	    index: number;
	    items: InventoryIntegrityConflictItem[];
	
	    static createFrom(source: any = {}) {
	        return new InventoryIntegrityConflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.items = this.convertValues(source["items"], InventoryIntegrityConflictItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class InventoryOrderItem {
	    handle: number;
	    itemId: number;
	    name: string;
	    category: string;
	    acquisitionIndex: number;
	    weight?: number;
	    sortId?: number;
	    sortGroupId?: number;
	    currentUpgrade?: number;
	    maxUpgrade?: number;
	    infusionName?: string;
	    iconPath?: string;
	    isTechnical?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InventoryOrderItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.handle = source["handle"];
	        this.itemId = source["itemId"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.acquisitionIndex = source["acquisitionIndex"];
	        this.weight = source["weight"];
	        this.sortId = source["sortId"];
	        this.sortGroupId = source["sortGroupId"];
	        this.currentUpgrade = source["currentUpgrade"];
	        this.maxUpgrade = source["maxUpgrade"];
	        this.infusionName = source["infusionName"];
	        this.iconPath = source["iconPath"];
	        this.isTechnical = source["isTechnical"];
	    }
	}
	export class LoadedTemplatePreview {
	    report: templates.ImportPreviewReport;
	    json?: string;
	    path?: string;
	
	    static createFrom(source: any = {}) {
	        return new LoadedTemplatePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.report = this.convertValues(source["report"], templates.ImportPreviewReport);
	        this.json = source["json"];
	        this.path = source["path"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PresetInfo {
	    name: string;
	    image: string;
	    bodyType: string;
	
	    static createFrom(source: any = {}) {
	        return new PresetInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.image = source["image"];
	        this.bodyType = source["bodyType"];
	    }
	}
	export class PvPPreparationOptions {
	    matchmakingRegions: boolean;
	    colosseums: boolean;
	    revealMap: boolean;
	    summoningPools: boolean;
	    sitesOfGrace: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PvPPreparationOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.matchmakingRegions = source["matchmakingRegions"];
	        this.colosseums = source["colosseums"];
	        this.revealMap = source["revealMap"];
	        this.summoningPools = source["summoningPools"];
	        this.sitesOfGrace = source["sitesOfGrace"];
	    }
	}
	export class SlotInventoryIntegrityReport {
	    slotIndex: number;
	    characterName: string;
	    active: boolean;
	    duplicateEntryCount: number;
	    conflictingIndexCount: number;
	    conflicts: InventoryIntegrityConflict[];
	
	    static createFrom(source: any = {}) {
	        return new SlotInventoryIntegrityReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slotIndex = source["slotIndex"];
	        this.characterName = source["characterName"];
	        this.active = source["active"];
	        this.duplicateEntryCount = source["duplicateEntryCount"];
	        this.conflictingIndexCount = source["conflictingIndexCount"];
	        this.conflicts = this.convertValues(source["conflicts"], InventoryIntegrityConflict);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveInventoryIntegrityReport {
	    clean: boolean;
	    slots: SlotInventoryIntegrityReport[];
	
	    static createFrom(source: any = {}) {
	        return new SaveInventoryIntegrityReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clean = source["clean"];
	        this.slots = this.convertValues(source["slots"], SlotInventoryIntegrityReport);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveIssue {
	    slot: number;
	    code: string;
	    message: string;
	    fixTab: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slot = source["slot"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.fixTab = source["fixTab"];
	    }
	}
	
	export class SlotCapacity {
	    gaItemsUsed: number;
	    gaItemsMax: number;
	    inventoryUsed: number;
	    inventoryMax: number;
	    storageUsed: number;
	    storageMax: number;
	
	    static createFrom(source: any = {}) {
	        return new SlotCapacity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gaItemsUsed = source["gaItemsUsed"];
	        this.gaItemsMax = source["gaItemsMax"];
	        this.inventoryUsed = source["inventoryUsed"];
	        this.inventoryMax = source["inventoryMax"];
	        this.storageUsed = source["storageUsed"];
	        this.storageMax = source["storageMax"];
	    }
	}
	export class SlotDiffSummary {
	    slotIndex: number;
	    charName: string;
	    changeCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SlotDiffSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slotIndex = source["slotIndex"];
	        this.charName = source["charName"];
	        this.changeCount = source["changeCount"];
	    }
	}
	

}

export namespace templates {
	
	export class ExportWarning {
	    code: string;
	    uid?: string;
	    container?: string;
	    position: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportWarning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.uid = source["uid"];
	        this.container = source["container"];
	        this.position = source["position"];
	        this.message = source["message"];
	    }
	}
	export class ImportPreviewIssue {
	    severity: string;
	    code: string;
	    message: string;
	    container?: string;
	    position?: number;
	    baseItemID?: number;
	    aowItemID?: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportPreviewIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.container = source["container"];
	        this.position = source["position"];
	        this.baseItemID = source["baseItemID"];
	        this.aowItemID = source["aowItemID"];
	    }
	}
	export class ImportPreviewSummary {
	    inventoryItems: number;
	    storageItems: number;
	    weapons: number;
	    armor: number;
	    talismans: number;
	    stackables: number;
	    aowAssignments: number;
	    version?: number;
	    selectedSections?: string[];
	    profileFieldsPresent?: string[];
	    statFieldsPresent?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportPreviewSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inventoryItems = source["inventoryItems"];
	        this.storageItems = source["storageItems"];
	        this.weapons = source["weapons"];
	        this.armor = source["armor"];
	        this.talismans = source["talismans"];
	        this.stackables = source["stackables"];
	        this.aowAssignments = source["aowAssignments"];
	        this.version = source["version"];
	        this.selectedSections = source["selectedSections"];
	        this.profileFieldsPresent = source["profileFieldsPresent"];
	        this.statFieldsPresent = source["statFieldsPresent"];
	    }
	}
	export class ImportPreviewReport {
	    ok: boolean;
	    errors: ImportPreviewIssue[];
	    warnings: ImportPreviewIssue[];
	    summary: ImportPreviewSummary;
	
	    static createFrom(source: any = {}) {
	        return new ImportPreviewReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.errors = this.convertValues(source["errors"], ImportPreviewIssue);
	        this.warnings = this.convertValues(source["warnings"], ImportPreviewIssue);
	        this.summary = this.convertValues(source["summary"], ImportPreviewSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LibraryTemplateEntry {
	    id: string;
	    name: string;
	    description?: string;
	    tags?: string[];
	    filename: string;
	    createdAt: string;
	    updatedAt: string;
	    inventoryItems: number;
	    storageItems: number;
	    warnings: number;
	    version?: number;
	    selectedSections?: string[];
	
	    static createFrom(source: any = {}) {
	        return new LibraryTemplateEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.tags = source["tags"];
	        this.filename = source["filename"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.inventoryItems = source["inventoryItems"];
	        this.storageItems = source["storageItems"];
	        this.warnings = source["warnings"];
	        this.version = source["version"];
	        this.selectedSections = source["selectedSections"];
	    }
	}

}

export namespace vm {
	
	export class AoWAvailabilityEntry {
	    itemId: number;
	    totalCopies: number;
	    availableCopies: number;
	    usedCopies: number;
	    usedByWeaponHandles: number[];
	    isMissing: boolean;
	    hasSharedHandleConflict: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AoWAvailabilityEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.totalCopies = source["totalCopies"];
	        this.availableCopies = source["availableCopies"];
	        this.usedCopies = source["usedCopies"];
	        this.usedByWeaponHandles = source["usedByWeaponHandles"];
	        this.isMissing = source["isMissing"];
	        this.hasSharedHandleConflict = source["hasSharedHandleConflict"];
	    }
	}
	export class ApplyOptions {
	    replaceStats: boolean;
	    replaceInventory: boolean;
	    replaceStorage: boolean;
	    replaceWorld: boolean;
	    keepName: boolean;
	    keepClass: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ApplyOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.replaceStats = source["replaceStats"];
	        this.replaceInventory = source["replaceInventory"];
	        this.replaceStorage = source["replaceStorage"];
	        this.replaceWorld = source["replaceWorld"];
	        this.keepName = source["keepName"];
	        this.keepClass = source["keepClass"];
	    }
	}
	export class WorldPresetData {
	    graces?: number[];
	    bosses?: number[];
	    summoningPools?: number[];
	    colosseums?: number[];
	    mapFlags?: number[];
	    cookbooks?: number[];
	    bellBearings?: number[];
	    whetblades?: number[];
	    gestures?: number[];
	    regions?: number[];
	    worldPickups?: number[];
	
	    static createFrom(source: any = {}) {
	        return new WorldPresetData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.graces = source["graces"];
	        this.bosses = source["bosses"];
	        this.summoningPools = source["summoningPools"];
	        this.colosseums = source["colosseums"];
	        this.mapFlags = source["mapFlags"];
	        this.cookbooks = source["cookbooks"];
	        this.bellBearings = source["bellBearings"];
	        this.whetblades = source["whetblades"];
	        this.gestures = source["gestures"];
	        this.regions = source["regions"];
	        this.worldPickups = source["worldPickups"];
	    }
	}
	export class PresetAddSettings {
	    upgrade25: number;
	    upgrade10: number;
	    infuseOffset: number;
	    upgradeAsh: number;
	    talismansHighestOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PresetAddSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upgrade25 = source["upgrade25"];
	        this.upgrade10 = source["upgrade10"];
	        this.infuseOffset = source["infuseOffset"];
	        this.upgradeAsh = source["upgradeAsh"];
	        this.talismansHighestOnly = source["talismansHighestOnly"];
	    }
	}
	export class PresetItem {
	    baseId: number;
	    name: string;
	    quantity: number;
	    upgrade: number;
	    infuse?: number;
	
	    static createFrom(source: any = {}) {
	        return new PresetItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseId = source["baseId"];
	        this.name = source["name"];
	        this.quantity = source["quantity"];
	        this.upgrade = source["upgrade"];
	        this.infuse = source["infuse"];
	    }
	}
	export class CharacterPresetCore {
	    name: string;
	    class: number;
	    className: string;
	    level: number;
	    souls: number;
	    vigor: number;
	    mind: number;
	    endurance: number;
	    strength: number;
	    dexterity: number;
	    intelligence: number;
	    faith: number;
	    arcane: number;
	    talismanSlots: number;
	    clearCount: number;
	    memoryStones: number;
	
	    static createFrom(source: any = {}) {
	        return new CharacterPresetCore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.class = source["class"];
	        this.className = source["className"];
	        this.level = source["level"];
	        this.souls = source["souls"];
	        this.vigor = source["vigor"];
	        this.mind = source["mind"];
	        this.endurance = source["endurance"];
	        this.strength = source["strength"];
	        this.dexterity = source["dexterity"];
	        this.intelligence = source["intelligence"];
	        this.faith = source["faith"];
	        this.arcane = source["arcane"];
	        this.talismanSlots = source["talismanSlots"];
	        this.clearCount = source["clearCount"];
	        this.memoryStones = source["memoryStones"];
	    }
	}
	export class CharacterPreset {
	    formatVersion: number;
	    exportedAt: string;
	    appVersion: string;
	    character: CharacterPresetCore;
	    inventory: PresetItem[];
	    storage: PresetItem[];
	    addSettings?: PresetAddSettings;
	    world?: WorldPresetData;
	
	    static createFrom(source: any = {}) {
	        return new CharacterPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.formatVersion = source["formatVersion"];
	        this.exportedAt = source["exportedAt"];
	        this.appVersion = source["appVersion"];
	        this.character = this.convertValues(source["character"], CharacterPresetCore);
	        this.inventory = this.convertValues(source["inventory"], PresetItem);
	        this.storage = this.convertValues(source["storage"], PresetItem);
	        this.addSettings = this.convertValues(source["addSettings"], PresetAddSettings);
	        this.world = this.convertValues(source["world"], WorldPresetData);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class StatValidationResult {
	    valid: boolean;
	    errors: string[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new StatValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.errors = source["errors"];
	        this.warnings = source["warnings"];
	    }
	}
	export class ItemViewModel {
	    handle: number;
	    id: number;
	    baseId: number;
	    name: string;
	    category: string;
	    subCategory: string;
	    subGroup: string;
	    quantity: number;
	    maxInventory: number;
	    maxStorage: number;
	    maxUpgrade: number;
	    currentUpgrade: number;
	    iconPath: string;
	    flags: string[];
	    readOnly: boolean;
	    aowId: number;
	    canMountAoW: boolean;
	    wepType: number;
	    aowCompatBitmask: number;
	
	    static createFrom(source: any = {}) {
	        return new ItemViewModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.handle = source["handle"];
	        this.id = source["id"];
	        this.baseId = source["baseId"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.subCategory = source["subCategory"];
	        this.subGroup = source["subGroup"];
	        this.quantity = source["quantity"];
	        this.maxInventory = source["maxInventory"];
	        this.maxStorage = source["maxStorage"];
	        this.maxUpgrade = source["maxUpgrade"];
	        this.currentUpgrade = source["currentUpgrade"];
	        this.iconPath = source["iconPath"];
	        this.flags = source["flags"];
	        this.readOnly = source["readOnly"];
	        this.aowId = source["aowId"];
	        this.canMountAoW = source["canMountAoW"];
	        this.wepType = source["wepType"];
	        this.aowCompatBitmask = source["aowCompatBitmask"];
	    }
	}
	export class CharacterViewModel {
	    name: string;
	    level: number;
	    souls: number;
	    class: number;
	    className: string;
	    vigor: number;
	    mind: number;
	    endurance: number;
	    strength: number;
	    dexterity: number;
	    intelligence: number;
	    faith: number;
	    arcane: number;
	    talismanSlots: number;
	    clearCount: number;
	    scadutreeBlessing: number;
	    shadowRealmBlessing: number;
	    memoryStones: number;
	    gender: number;
	    soulMemory: number;
	    inventory: ItemViewModel[];
	    storage: ItemViewModel[];
	    warnings: string[];
	    statValidation?: StatValidationResult;
	    eventFlagsAvailable: boolean;
	    classBaseStats: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new CharacterViewModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.level = source["level"];
	        this.souls = source["souls"];
	        this.class = source["class"];
	        this.className = source["className"];
	        this.vigor = source["vigor"];
	        this.mind = source["mind"];
	        this.endurance = source["endurance"];
	        this.strength = source["strength"];
	        this.dexterity = source["dexterity"];
	        this.intelligence = source["intelligence"];
	        this.faith = source["faith"];
	        this.arcane = source["arcane"];
	        this.talismanSlots = source["talismanSlots"];
	        this.clearCount = source["clearCount"];
	        this.scadutreeBlessing = source["scadutreeBlessing"];
	        this.shadowRealmBlessing = source["shadowRealmBlessing"];
	        this.memoryStones = source["memoryStones"];
	        this.gender = source["gender"];
	        this.soulMemory = source["soulMemory"];
	        this.inventory = this.convertValues(source["inventory"], ItemViewModel);
	        this.storage = this.convertValues(source["storage"], ItemViewModel);
	        this.warnings = source["warnings"];
	        this.statValidation = this.convertValues(source["statValidation"], StatValidationResult);
	        this.eventFlagsAvailable = source["eventFlagsAvailable"];
	        this.classBaseStats = source["classBaseStats"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class PresetApplyResult {
	    statsApplied: boolean;
	    worldApplied: boolean;
	    itemsAdded: number;
	    itemsSkipped: number;
	    itemsRemoved: number;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new PresetApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statsApplied = source["statsApplied"];
	        this.worldApplied = source["worldApplied"];
	        this.itemsAdded = source["itemsAdded"];
	        this.itemsSkipped = source["itemsSkipped"];
	        this.itemsRemoved = source["itemsRemoved"];
	        this.warnings = source["warnings"];
	    }
	}
	
	

}

