# 37 — Character Preset Export / Import (JSON Profile)

> **Type**: Design doc
> **Extracted from**: docs/ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planned

---

## Goal

A human-readable JSON dump of a character profile (stats + inventory + storage + optionally appearance / world flags) with the ability to re-import it onto another slot, plus offline preset editing (without loading a save).

**Why:**
- Share builds — players exchange builds in the community without copying entire `.sl2` files
- Backup before experiments — quick character snapshot to JSON, restore in 1 click
- Replacement for the currently disabled `App.ImportCharacter` (`app.go:1719`, "temporarily disabled during architecture refactor") — the new path is cleaner (by BaseID, not by raw bytes)
- Standalone editing — a power user plans a build in the app without loading a save, then applies it with a single click

**Source of truth:** the existing `vm.CharacterViewModel` (already has `json:` tags) + spec/31 (FaceData layout) + spec/34 (item caps + NG+ scaling, validation on apply).

---

## File format

JSON with `formatVersion: 1` and `appVersion` in the header — versioned for backward compatibility. **Not YAML** (a new dependency), **not TXT** (not parseable in the reverse direction). Items are identified by `BaseID + upgrade + infuse + quantity` (NOT by runtime handle — handles are re-generated on apply).

```json
{
  "formatVersion": 1,
  "exportedAt": "2026-04-29T20:14:00Z",
  "appVersion": "0.7.0",
  "character": {
    "name": "OiSiS", "class": 0, "className": "Vagabond",
    "level": 150, "souls": 999999999,
    "vigor": 60, "mind": 25, "endurance": 40,
    "strength": 50, "dexterity": 30, "intelligence": 9, "faith": 7, "arcane": 9,
    "talismanSlots": 3, "clearCount": 0,
    "greatRuneOn": true, "equippedGreatRune": 1073741909,
    "scadutreeBlessing": 20, "shadowRealmBlessing": 13
  },
  "inventory": [
    { "baseId": 134218848, "name": "Uchigatana", "quantity": 1, "upgrade": 25, "infuse": 800 },
    { "baseId": 1073741857, "name": "Crimson Tears Flask", "quantity": 14, "upgrade": 12 }
  ],
  "storage": []
}
```

---

## Phase 1 — Export MVP (stats + inventory + storage) — ~4-6h

**Backend** (~80 lines, new file `backend/vm/preset.go`):
```go
type CharacterPreset struct {
    FormatVersion int                 `json:"formatVersion"`
    ExportedAt    string              `json:"exportedAt"`
    AppVersion    string              `json:"appVersion"`
    Character     CharacterPresetCore `json:"character"`
    Inventory     []PresetItem        `json:"inventory"`
    Storage       []PresetItem        `json:"storage"`
}

type CharacterPresetCore struct {
    Name string `json:"name"`
    Class uint8 `json:"class"`
    ClassName string `json:"className"`
    Level uint32 `json:"level"`
    Souls uint32 `json:"souls"`
    Vigor uint32 `json:"vigor"`
    Mind uint32 `json:"mind"`
    Endurance uint32 `json:"endurance"`
    Strength uint32 `json:"strength"`
    Dexterity uint32 `json:"dexterity"`
    Intelligence uint32 `json:"intelligence"`
    Faith uint32 `json:"faith"`
    Arcane uint32 `json:"arcane"`
    TalismanSlots uint8 `json:"talismanSlots"`
    ClearCount uint32 `json:"clearCount"`
    GreatRuneOn bool `json:"greatRuneOn"`
    EquippedGreatRune uint32 `json:"equippedGreatRune"`
    ScadutreeBlessing uint8 `json:"scadutreeBlessing"`
    ShadowRealmBlessing uint8 `json:"shadowRealmBlessing"`
}

type PresetItem struct {
    BaseID         uint32 `json:"baseId"`
    Name           string `json:"name"`
    Quantity       uint32 `json:"quantity"`
    CurrentUpgrade uint32 `json:"upgrade"`
    InfuseOffset   uint32 `json:"infuse,omitempty"`
}
```

**App methods** (`app.go`):
- `ExportCharacterPreset(charIdx int) (*vm.CharacterPreset, error)` — returns the struct (Wails serializes it to JS automatically)
- `ExportCharacterPresetToFile(charIdx int) (string, error)` — `runtime.SaveFileDialog` + `os.WriteFile` with `json.MarshalIndent`. Default filename: `<CharacterName>_<level>_<className>.preset.json`

**Frontend** (~30 lines, `CharacterTab.tsx`):
- "Export Preset" button in the Profile section (next to Add to Mirror)
- Toast: "Preset exported to: {path}"

**Tests** (`backend/vm/preset_test.go`):
- VMToPreset → PresetToVM round-trip preserves all fields
- JSON serialization stable (golden file)
- Item identity: BaseID extraction strips upgrade/infuse correctly

---

## Phase 2 — Import / Apply to slot — ~6-8h

**Backend** (`backend/vm/preset.go` + `app.go`, ~150 lines):

```go
type ApplyOptions struct {
    ReplaceStats     bool `json:"replaceStats"`
    ReplaceInventory bool `json:"replaceInventory"`
    ReplaceStorage   bool `json:"replaceStorage"`
    KeepName         bool `json:"keepName"`
    KeepClass        bool `json:"keepClass"`
}

type ApplyResult struct {
    StatsApplied      bool     `json:"statsApplied"`
    ItemsAdded        int      `json:"itemsAdded"`
    ItemsRemoved      int      `json:"itemsRemoved"`
    Warnings          []string `json:"warnings"`
}
```

**App methods:**
- `LoadCharacterPresetFromFile() (*vm.CharacterPreset, error)` — `runtime.OpenFileDialog` + `json.Unmarshal` + validation of `FormatVersion == 1`
- `ValidateCharacterPreset(preset vm.CharacterPreset) []string` — pre-flight: unknown BaseIDs, qty > cap per spec/34, class stat-floor mismatch
- `ApplyCharacterPreset(charIdx int, preset vm.CharacterPreset, opts ApplyOptions) (*ApplyResult, error)`:
  1. `pushUndo(charIdx)`
  2. Stats → `ApplyVMToParsedSlot` (skip Name/Class per opts)
  3. Inventory clear → `RemoveItemsFromCharacter`
  4. Inventory add → `core.AddItemsToSlot(slot, finalID, qty, 0, forceStackable)`
  5. Storage analogously
  6. Reuse AoW flag / world pickup flag / container logic

**Frontend** (~120 lines, `PresetImporter.tsx` in `ToolsTab`):
- "Import Preset" button → preview card → checkboxes → slot dropdown → warnings → apply
- `RiskActionButton` with `riskKey="character_import"` (Tier 1)

**Tests** (`tests/preset_apply_test.go`):
- Apply on a clean slot — stats + items match
- Round-trip Export → Apply → Export → diff zero
- Qty cap clamp: preset `qty: 999` on an item with MaxInventory=10 → apply qty=10 + warning

---

## Phase 3 — Standalone preset editor (offline, without a save) — ~10h

Mostly frontend. The backend already supports stateless queries (`GetItemList`, `GetInfuseTypes`, `GetClassStats`).

**State management** (`App.tsx`):
```ts
type EditorMode = 'save' | 'preset';
const [editorMode, setEditorMode] = useState<EditorMode>('save');
const [editingPreset, setEditingPreset] = useState<CharacterPreset | null>(null);
```

- "Save / Preset Workspace" toggle in the top bar
- Preset mode: sidebar with [New] [Load] [Save] [Apply to Slot]
- Character + Inventory + Database tabs work normally; World/Tools/Settings hidden
- The `useCurrentVM()` hook abstracts the slot vs preset source

---

## Phase 4 (optional) — Appearance (FaceData blob) — ~3-4h

```go
type CharacterPresetCore struct {
    // ... existing fields
    Gender      uint8  `json:"gender,omitempty"`
    VoiceType   uint8  `json:"voiceType,omitempty"`
    FaceDataB64 string `json:"faceDataB64,omitempty"`  // 303 B → base64
}
```

- Export: FaceData slice → base64
- Apply: same as `ApplyMirrorFavoriteToCharacter` (copying 5 segments)
- Cross-gender caveat: UI warning when `preset.gender != slot.gender`

---

## Phase 5 (optional) — World flags — ~12-16h

```go
type CharacterPresetWorld struct {
    Graces          []uint32       `json:"graces,omitempty"`
    Bosses          []uint32       `json:"bosses,omitempty"`
    Quests          map[string]int `json:"quests,omitempty"`
    MapRegions      []uint32       `json:"mapRegions,omitempty"`
    Cookbooks       []uint32       `json:"cookbooks,omitempty"`
    BellBearings    []uint32       `json:"bellBearings,omitempty"`
    Whetblades      []uint32       `json:"whetblades,omitempty"`
    Gestures        []uint32       `json:"gestures,omitempty"`
    UnlockedRegions []uint32       `json:"unlockedRegions,omitempty"`
    FogOfWarRemoved bool           `json:"fogOfWarRemoved,omitempty"`
}
```

Scope creep risk — deferred to user feedback after Phase 1+2.

---

## Phase summary

| Phase | Effort | Value |
|---|---|---|
| **1** Export MVP | 4-6h | Snapshot character → JSON |
| **2** Import / Apply | 6-8h | Bidirectional preset workflow |
| **1+2 (recommended start)** | **10-14h** | 80% of the feature's value |
| 3 Standalone editor | +10h | Offline editing without a save |
| 4 Appearance blob | +3-4h | Full visual transfer |
| 5 World flags | +12-16h | Full character clone |

---

## Open questions

1. **Equipped items in the MVP**: skip them (the player re-equips from inventory after apply) or add an `equipped` section? — suggestion: skip in v1.
2. **Class change on apply**: default `KeepClass=true` (the class affects stat-floor validation per spec/34).
3. **NG+ scaling during validation**: use the slot's ClearCount (not the preset's) to compute the effective cap.
4. **Fail-fast vs best-effort**: best-effort with warnings in `ApplyResult` (precedent: `AddItemsToCharacter` returns `[]SkippedAdd`).
5. **Backup before apply**: auto-export the current character to `.preset.bak.json` before overwriting (quick rollback beyond the undo stack limit).
