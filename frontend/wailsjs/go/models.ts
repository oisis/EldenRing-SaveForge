export namespace core {
	
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
	
	export class AshOfWarFlagEntry {
	    id: number;
	    name: string;
	    unlocked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AshOfWarFlagEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.unlocked = source["unlocked"];
	    }
	}
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

export namespace main {
	
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

export namespace vm {
	
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

