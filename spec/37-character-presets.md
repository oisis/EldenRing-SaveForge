# 37 — Character Preset Export / Import (JSON Profile)

> **Type**: Design doc
> **Extracted from**: docs/ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planned

---

## Goal

Human-readable JSON dump of a character profile (stats + inventory + storage + opcjonalnie wygląd / world flags) z możliwością re-importu na inny slot, oraz edycja presetu offline (bez ładowania save'a).

**Why:**
- Share builds — gracze wymieniają się buildami w community bez kopiowania całych `.sl2`
- Backup przed eksperymentami — szybki snapshot postaci do JSON, restore w 1 kliku
- Replace dla aktualnie wyłączonego `App.ImportCharacter` (`app.go:1719`, "temporarily disabled during architecture refactor") — nowa ścieżka jest cleaner (po BaseID, nie po surowych bajtach)
- Standalone editing — power-user planuje build w aplikacji bez load'owania save'a, później aplikuje jednym kliknięciem

**Source of truth:** istniejący `vm.CharacterViewModel` (już ma `json:` tagi) + spec/31 (FaceData layout) + spec/34 (item caps + NG+ scaling, walidacja przy apply).

---

## Format pliku

JSON z `formatVersion: 1` i `appVersion` w nagłówku — versioned dla backward-compat. **Nie YAML** (nowa zależność), **nie TXT** (nieparsowalny w drugą stronę). Items identyfikowane po `BaseID + upgrade + infuse + quantity` (NIE po runtime handle — handles są re-generowane przy apply).

```json
{
  "formatVersion": 1,
  "exportedAt": "2026-04-29T20:14:00Z",
  "appVersion": "0.7.0",
  "character": {
    "name": "OiSiSk", "class": 0, "className": "Vagabond",
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

**Backend** (~80 linii, nowy plik `backend/vm/preset.go`):
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
- `ExportCharacterPreset(charIdx int) (*vm.CharacterPreset, error)` — zwraca strukturę (Wails serializuje do JS auto)
- `ExportCharacterPresetToFile(charIdx int) (string, error)` — `runtime.SaveFileDialog` + `os.WriteFile` z `json.MarshalIndent`. Default filename: `<CharacterName>_<level>_<className>.preset.json`

**Frontend** (~30 linii, `CharacterTab.tsx`):
- Przycisk "Export Preset" w sekcji Profile (obok Add to Mirror)
- Toast: "Preset exported to: {path}"

**Tests** (`backend/vm/preset_test.go`):
- VMToPreset → PresetToVM round-trip preserves all fields
- JSON serialization stable (golden file)
- Item identity: BaseID extraction strips upgrade/infuse correctly

---

## Phase 2 — Import / Apply do slotu — ~6-8h

**Backend** (`backend/vm/preset.go` + `app.go`, ~150 linii):

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
- `LoadCharacterPresetFromFile() (*vm.CharacterPreset, error)` — `runtime.OpenFileDialog` + `json.Unmarshal` + walidacja `FormatVersion == 1`
- `ValidateCharacterPreset(preset vm.CharacterPreset) []string` — pre-flight: unknown BaseIDs, qty > cap per spec/34, stat floor klasy mismatch
- `ApplyCharacterPreset(charIdx int, preset vm.CharacterPreset, opts ApplyOptions) (*ApplyResult, error)`:
  1. `pushUndo(charIdx)`
  2. Stats → `ApplyVMToParsedSlot` (skip Name/Class per opts)
  3. Inventory clear → `RemoveItemsFromCharacter`
  4. Inventory add → `core.AddItemsToSlot(slot, finalID, qty, 0, forceStackable)`
  5. Storage analogicznie
  6. Reuse AoW flag / world pickup flag / container logic

**Frontend** (~120 linii, `PresetImporter.tsx` w `ToolsTab`):
- "Import Preset" button → preview card → checkboxes → slot dropdown → warnings → apply
- `RiskActionButton` z `riskKey="character_import"` (Tier 1)

**Tests** (`tests/preset_apply_test.go`):
- Apply on clean slot — stats + items match
- Round-trip Export → Apply → Export → diff zero
- Qty cap clamp: preset `qty: 999` on item MaxInventory=10 → apply qty=10 + warning

---

## Phase 3 — Standalone preset editor (offline, bez save'a) — ~10h

Frontend-heavy. Backend already supports stateless queries (`GetItemList`, `GetInfuseTypes`, `GetClassStats`).

**State management** (`App.tsx`):
```ts
type EditorMode = 'save' | 'preset';
const [editorMode, setEditorMode] = useState<EditorMode>('save');
const [editingPreset, setEditingPreset] = useState<CharacterPreset | null>(null);
```

- Toggle "Save / Preset Workspace" in top bar
- Preset mode: sidebar with [New] [Load] [Save] [Apply to Slot]
- Character + Inventory + Database tabs work normally; World/Tools/Settings hidden
- `useCurrentVM()` hook abstracts slot vs preset source

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
- Apply: same as `ApplyMirrorFavoriteToCharacter` (5 segments copy)
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
    AshOfWarFlags   []uint32       `json:"ashOfWarFlags,omitempty"`
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
| **1+2 (recommended start)** | **10-14h** | 80% of feature value |
| 3 Standalone editor | +10h | Offline editing without save |
| 4 Appearance blob | +3-4h | Full visual transfer |
| 5 World flags | +12-16h | Full character clone |

---

## Open questions

1. **Equipped items in MVP**: skip (player re-equips from inventory after apply) or add `equipped` section? — suggest skip in v1.
2. **Class change on apply**: default `KeepClass=true` (class affects stat floor validation per spec/34).
3. **NG+ scaling on validation**: use slot's ClearCount (not preset's) for effective cap calculation.
4. **Fail-fast vs best-effort**: best-effort with warnings in `ApplyResult` (precedent: `AddItemsToCharacter` returns `[]SkippedAdd`).
5. **Backup before apply**: auto-export current character to `.preset.bak.json` before overwrite (quick rollback beyond undo stack limit).
