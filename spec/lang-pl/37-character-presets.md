# 37 — Eksport / Import presetów postaci (JSON Profile)

> **Typ**: Dokument projektowy
> **Wyodrębniono z**: docs/ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planowany

---

## Cel

Czytelny dla człowieka zrzut profilu postaci do JSON (statystyki + ekwipunek + schowek + opcjonalnie wygląd / flagi świata) z możliwością re-importu na inny slot, oraz edycja presetu offline (bez ładowania save'a).

**Dlaczego:**
- Udostępnianie buildów — gracze wymieniają się buildami w community bez kopiowania całych `.sl2`
- Backup przed eksperymentami — szybki snapshot postaci do JSON, restore w 1 kliku
- Zastąpienie aktualnie wyłączonego `App.ImportCharacter` (`app.go:1719`, "temporarily disabled during architecture refactor") — nowa ścieżka jest czystsza (po BaseID, nie po surowych bajtach)
- Edycja samodzielna — power-user planuje build w aplikacji bez ładowania save'a, później aplikuje jednym kliknięciem

**Źródło prawdy:** istniejący `vm.CharacterViewModel` (już ma `json:` tagi) + spec/31 (layout FaceData) + spec/34 (limity itemów + skalowanie NG+, walidacja przy apply).

---

## Format pliku

JSON z `formatVersion: 1` i `appVersion` w nagłówku — wersjonowany dla backward-compat. **Nie YAML** (nowa zależność), **nie TXT** (nieparsowalny w drugą stronę). Itemy identyfikowane po `BaseID + upgrade + infuse + quantity` (NIE po runtime handle — handles są re-generowane przy apply).

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

## Faza 1 — Export MVP (statystyki + ekwipunek + schowek) — ~4-6h

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

**Metody App** (`app.go`):
- `ExportCharacterPreset(charIdx int) (*vm.CharacterPreset, error)` — zwraca strukturę (Wails serializuje do JS automatycznie)
- `ExportCharacterPresetToFile(charIdx int) (string, error)` — `runtime.SaveFileDialog` + `os.WriteFile` z `json.MarshalIndent`. Domyślna nazwa pliku: `<CharacterName>_<level>_<className>.preset.json`

**Frontend** (~30 linii, `CharacterTab.tsx`):
- Przycisk "Export Preset" w sekcji Profile (obok Add to Mirror)
- Toast: "Preset exported to: {path}"

**Testy** (`backend/vm/preset_test.go`):
- VMToPreset → PresetToVM round-trip zachowuje wszystkie pola
- Serializacja JSON stabilna (golden file)
- Tożsamość itemów: ekstrakcja BaseID poprawnie odcina upgrade/infuse

---

## Faza 2 — Import / Apply do slotu — ~6-8h

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

**Metody App:**
- `LoadCharacterPresetFromFile() (*vm.CharacterPreset, error)` — `runtime.OpenFileDialog` + `json.Unmarshal` + walidacja `FormatVersion == 1`
- `ValidateCharacterPreset(preset vm.CharacterPreset) []string` — pre-flight: nieznane BaseIDs, qty > cap wg spec/34, niezgodność stat floor klasy
- `ApplyCharacterPreset(charIdx int, preset vm.CharacterPreset, opts ApplyOptions) (*ApplyResult, error)`:
  1. `pushUndo(charIdx)`
  2. Statystyki → `ApplyVMToParsedSlot` (pomiń Name/Class wg opts)
  3. Czyszczenie ekwipunku → `RemoveItemsFromCharacter`
  4. Dodanie ekwipunku → `core.AddItemsToSlot(slot, finalID, qty, 0, forceStackable)`
  5. Schowek analogicznie
  6. Reuse flag AoW / flag world pickup / logika kontenerów

**Frontend** (~120 linii, `PresetImporter.tsx` w `ToolsTab`):
- Przycisk "Import Preset" → karta podglądu → checkboxy → dropdown slotu → ostrzeżenia → apply
- `RiskActionButton` z `riskKey="character_import"` (Tier 1)

**Testy** (`tests/preset_apply_test.go`):
- Apply na czysty slot — statystyki + itemy się zgadzają
- Round-trip Export → Apply → Export → diff zero
- Clamp limitu qty: preset `qty: 999` na itemie MaxInventory=10 → apply qty=10 + warning

---

## Faza 3 — Samodzielny edytor presetów (offline, bez save'a) — ~10h

Głównie frontend. Backend już obsługuje zapytania bezstanowe (`GetItemList`, `GetInfuseTypes`, `GetClassStats`).

**Zarządzanie stanem** (`App.tsx`):
```ts
type EditorMode = 'save' | 'preset';
const [editorMode, setEditorMode] = useState<EditorMode>('save');
const [editingPreset, setEditingPreset] = useState<CharacterPreset | null>(null);
```

- Przełącznik "Save / Preset Workspace" w górnym pasku
- Tryb presetu: sidebar z [New] [Load] [Save] [Apply to Slot]
- Zakładki Character + Inventory + Database działają normalnie; World/Tools/Settings ukryte
- Hook `useCurrentVM()` abstrahuje źródło slot vs preset

---

## Faza 4 (opcjonalna) — Wygląd (blob FaceData) — ~3-4h

```go
type CharacterPresetCore struct {
    // ... istniejące pola
    Gender      uint8  `json:"gender,omitempty"`
    VoiceType   uint8  `json:"voiceType,omitempty"`
    FaceDataB64 string `json:"faceDataB64,omitempty"`  // 303 B → base64
}
```

- Export: wycinek FaceData → base64
- Apply: tak samo jak `ApplyMirrorFavoriteToCharacter` (kopiowanie 5 segmentów)
- Uwaga cross-gender: ostrzeżenie w UI gdy `preset.gender != slot.gender`

---

## Faza 5 (opcjonalna) — Flagi świata — ~12-16h

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

Ryzyko scope creep — odłożone do feedbacku użytkowników po Fazie 1+2.

---

## Podsumowanie faz

| Faza | Nakład pracy | Wartość |
|---|---|---|
| **1** Export MVP | 4-6h | Snapshot postaci → JSON |
| **2** Import / Apply | 6-8h | Dwukierunkowy workflow presetów |
| **1+2 (rekomendowany start)** | **10-14h** | 80% wartości feature'u |
| 3 Samodzielny edytor | +10h | Edycja offline bez save'a |
| 4 Blob wyglądu | +3-4h | Pełny transfer wizualny |
| 5 Flagi świata | +12-16h | Pełny klon postaci |

---

## Otwarte pytania

1. **Założone itemy w MVP**: pominąć (gracz ponownie zakłada z ekwipunku po apply) czy dodać sekcję `equipped`? — sugestia: pominąć w v1.
2. **Zmiana klasy przy apply**: domyślnie `KeepClass=true` (klasa wpływa na walidację stat floor wg spec/34).
3. **Skalowanie NG+ przy walidacji**: użyj ClearCount slotu (nie presetu) do obliczenia efektywnego capu.
4. **Fail-fast vs best-effort**: best-effort z ostrzeżeniami w `ApplyResult` (precedens: `AddItemsToCharacter` zwraca `[]SkippedAdd`).
5. **Backup przed apply**: automatyczny eksport aktualnej postaci do `.preset.bak.json` przed nadpisaniem (szybki rollback poza limitem stosu undo).
