# 55 — Build Template

> **Type**: Design doc
> **Status**: ✅ Zaimplementowane — Phase A (eksporter) + Phase B (file I/O) + Phase C (preview) + Phase D (apply) + Phase E (lokalna biblioteka).
> **Scope**: Canonical chapter dla SaveForge Build Template — przenośnej reprezentacji JSON snapshotu Inventory Workspace. Definiuje schemat `saveforge.build-template` v1, kontrakty export / preview / apply / library, regułę portable payload (no save-local handles) oraz wszystkie reguły bezpieczeństwa importu (m.in. fail-closed na unknown AoW compat).

---

## 1. Cel rozdziału

Build Template pozwala zachować workspace inventory + storage (bronie, zbroje, talizmany, itemy stackable, AoW assignment, infusion, upgrade level) jako pojedynczy, przenośny dokument JSON. Cel:

- Bootstrap nowej postaci znanym setupem bez ręcznego dodawania każdego itemu.
- Współdzielenie buildów między użytkownikami / platformami / postaciami w tym samym save.
- Lokalna biblioteka per-user do szybkiego apply z UI.

Rozdział łączy:

- schema v1 (binary-stable JSON),
- export flow (Phase A + B),
- import preview + apply (Phase C + D),
- lokalna biblioteka (Phase E),
- reguły portability / safety,
- relacje do canonical rozdziałów inventory / AoW / allocator.

Rozdziały referencyjne, których **nie powtarzamy**:

- [54-ash-of-war](54-ash-of-war.md) — pełna semantyka AoW (sentinele, write paths, compat matrix).
- [03-gaitem-map](03-gaitem-map.md) — GaItem binary model.
- [06-equipment](06-equipment.md) — equipped gear (poza scope Phase A).
- [07-inventory](07-inventory.md) / [10-storage](10-storage.md) — inventory / storage section model.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator capacity rules.
- [36-inventory-categories-game-order](36-inventory-categories-game-order.md) — kategorie itemów.
- [43-transactional-item-adding](43-transactional-item-adding.md) — Add Items pipeline (używany przy save dopiero, nie przy apply).

---

## 2. Status

| Faza | Scope | Status |
|---|---|---|
| **A** | Schemat + eksporter (`backend/templates/schema.go`, `export.go`), spec, testy | ✅ |
| **B** | Wails bindings `ExportBuildTemplateJSON` / `ExportBuildTemplateToFile`, `ExportTemplateModal`, dropdown w SortOrderTab | ✅ |
| **C** | Preview importu: `ParseBuildTemplateJSON`, `PreviewBuildTemplateImport`, endpointy `PreviewBuildTemplateImportJSON/FromFile`, `ImportTemplatePreviewModal` (read-only) | ✅ |
| **D** | Apply do workspace: `ApplyBuildTemplateToWorkspaceJSON/FromFile`, capacity preflight, RAM-only mutation z rollbackiem | ✅ |
| **E** | Lokalna biblioteka szablonów pod `$UserConfigDir/EldenRing-SaveEditor/templates/`, 9 endpointów App, `TemplateLibraryModal` | ✅ |
| 2+ | Sekcja `character.profile` (level, stats, talisman slots), opt-in via `$enabled` | 🔲 odroczone |
| 2+ | Tryby `replace-inventory` / `replace-all` (reserved w `ApplyTemplateOptions.Mode`) | 🔲 odroczone |

---

## 3. Source of truth w kodzie

| Obszar | Pliki / symbole |
|---|---|
| Schema typy | `backend/templates/schema.go`: `BuildTemplate`, `TemplateMetadata`, `TemplateSections`, `InventoryWorkspaceSection`, `TemplateItem`, `ExportWarning`, `ExportReport`. |
| Schema stałe | `schema.go`: `SchemaKey = "saveforge.build-template"`, `SchemaVersion = 1`, `WarnCode*`, `ContainerInventory`/`ContainerStorage`. |
| Eksporter | `backend/templates/export.go`: `BuildTemplateFromSnapshot`, `ExportOptions`, `convertItems`, `ValidateBuildTemplate`, `validateItems`. |
| Import / preview | `backend/templates/import.go`: `ParseBuildTemplateJSON`, `PreviewBuildTemplateImport`, `ImportPreviewOptions`, `ImportPreviewReport`, `ImportPreviewIssue`, `ImportPreviewSummary`, `IssueCode*`. |
| Library | `backend/templates/library.go`: `TemplateLibrary`, `LibraryTemplateEntry`, `TemplateLibraryIndex`, `LibraryIndexFile = "_index.json"`, `LibraryIndexVersion = 1`, `DefaultTemplateLibraryDir`, `atomicWriteFile`. |
| App bindings | `app_templates.go`: `BuildTemplateExportOptions`, `BuildTemplateExportResult`, `ApplyTemplateOptions`, `ApplyTemplateResult`, `LoadedTemplatePreview` + 15 Wails-exposed metod `App` (Export×2, Preview×2, Apply×2, Library×9) plus 3 internal helpery (`buildAndValidateTemplate`, `sourceCharacterName`, `ensureTemplateLibrary`). |
| Capacity | `backend/core/offset_defs.go`: `CommonItemCount = 0xA80 (2688)`, `StorageCommonCount = 0x780 (1920)`. |
| Apply mutation | `app_templates.go::applyTemplateItemsToWorkspace` → `editor.AddItem` + `editor.UpdateWeapon`. |
| Capacity preflight | `app_templates.go::capacityPreflight`. |
| Rollback | `app_templates.go::deepCopySnapshot`. |
| Frontend modale | `frontend/src/components/templates/`: `ExportTemplateModal.tsx` (woła `ExportBuildTemplateToFile`, `SaveBuildTemplateToLibrary`), `ImportTemplatePreviewModal.tsx` (read-only props, nie woła Wails bezpośrednio), `TemplateLibraryModal.tsx` (woła 8 Wails methods biblioteki). |
| Frontend orchestrator | `frontend/src/components/SortOrderTab.tsx` (woła `ExportBuildTemplateToFile`, `PreviewBuildTemplateImportFromFile`, `ApplyBuildTemplateToWorkspaceJSON`; orkiestruje modale + `useInventoryWorkspace.replaceSnapshot` po Apply). |

---

## 4. Mental model

Build Template = portable snapshot zawartości **inventory workspace** w określonym momencie. Trzy warstwy odpowiedzialności:

1. **Schema** (`backend/templates/schema.go`) — typed JSON, bez zależności od `editor`, `core` lub `db`.
2. **Pipeline** (`backend/templates/{export,import,library}.go`) — czyste funkcje od snapshot do JSON i z JSON do `ImportPreviewReport`. Library to per-user store.
3. **App glue** (`app_templates.go`) — łączy schema/pipeline z `editor.InventoryEditSession`, file dialog, capacity preflight, rollback.

Kluczowy invariant: **template przechowuje wyłącznie semantyczne identyfikatory** (`baseItemID`, `quantity`, `upgrade`, `infusionName`, `aowItemID`). Nigdy save-local addressing (handle GaItem, UID sesji, acquisition index). Dzięki temu szablon stworzony w save A może być zaaplikowany w save B bez kolizji w przestrzeni handle.

Apply jest **RAM-only**: mutuje `sess.Workspace` (ustawia `Dirty=true`), ale nigdy nie woła `SaveInventoryWorkspaceChanges`, nie pisze do `slot.Data`, nie alokuje handle GaItem. Użytkownik musi nadal kliknąć "Save changes" — wtedy ścieżka save delegująca do `editor.AddItem` + `editor.UpdateWeapon` poprzez `ApplyWorkspaceSave` zaalokuje rzeczywiste handle przez allocator z [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).

---

## 5. Build Template vs Character Preset vs Advanced Preset

Trzy różne mechanizmy, często mylone:

| Mechanizm | Co zapisuje | Scope | Status w SaveForge |
|---|---|---|---|
| **Build Template** (ten dokument) | Workspace inventory + storage (bronie z upgrade/infusion/AoW, zbroje, talizmany, stackables) | Cross-save portable | ✅ Phase A–E |
| **Character Preset** ([37-character-presets](37-character-presets.md)) | Klasa startowa (level, stats, starting gear) | Domyślne presets gry + niestandardowe | osobny mechanizm |
| **Advanced Preset** | n/d | (poza scope SaveForge) | nie istnieje |

Build Template **nie eksportuje**:

- character level / stats / attributes,
- spell loadout / gestures / key items,
- equipped gear (`Equipment` section save'a) — patrz §11,
- map progress / event flags / acquisition history,
- regulation.bin overrides.

`needs verification` dla potencjalnych przyszłych sekcji `character.profile` / `equipment` — schema ma slot na nie (`$enabled` flag), ale w v1 są poza scope.

---

## 6. JSON v1 schema

### 6.1. Header

```json
{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-17T12:34:56Z",
  "appVersion": "0.15.0-beta",
  "metadata": { … },
  "sections": { … }
}
```

| Pole | Typ | Wymagane | Source | Notatka portability |
|---|---|---|---|---|
| `schema` | string | ✅ | `schema.go::SchemaKey` | Musi być dokładnie `"saveforge.build-template"`. Importer odrzuca każdą inną wartość. |
| `version` | int | ✅ | `schema.go::SchemaVersion` | Akceptowany zakres: `1 ≤ v ≤ SchemaVersion` (obecnie max = 1). |
| `createdAt` | string (RFC 3339 UTC) | ✅ | `time.Now().UTC().Format(time.RFC3339)` lub `opts.Now` (test-only) | Informacyjny. |
| `appVersion` | string | optional | `BuildTemplateExportOptions.AppVersion` | Informacyjny. |
| `metadata` | object | optional | `TemplateMetadata` (name/description/author/tags/sourceCharacterIndex/sourceCharacterName) | **Brak pól load-bearing** — nic w metadanych nie zmienia logiki importu. |
| `sections` | object | ✅ | `TemplateSections` | Klucz = stabilny identyfikator sekcji. v1 ma tylko `inventory.workspace`. |

### 6.2. Sekcja `inventory.workspace`

```json
"sections": {
  "inventory.workspace": {
    "inventoryItems": [ TemplateItem, … ],
    "storageItems":   [ TemplateItem, … ]
  }
}
```

Oba array muszą być obecne (mogą być puste), ale `ValidateBuildTemplate` odrzuca template, gdy oba są puste (`"inventory.workspace is empty"`).

### 6.3. `TemplateItem`

```json
{
  "baseItemID":   4030000,
  "name":         "Greatsword",
  "category":     "melee_armaments",
  "quantity":     1,
  "upgrade":      25,
  "infusionName": "Heavy",
  "aowItemID":    2168029136,
  "container":    "inventory",
  "position":     0
}
```

| Pole | Typ | Wymagane | Source | Portability note |
|---|---|---|---|---|
| `baseItemID` | uint32 | ✅ | `EditableItem.BaseItemID` (bez kodowania upgrade/infusion) | DB lookup key przy imporcie. **Nie może być 0.** |
| `name` | string | optional | `EditableItem.Name` | Tylko debug/display; przy imporcie DB name jest source of truth. Drift produkuje warning `name_mismatch_ignored`. |
| `category` | string | optional | `EditableItem.Category` | Tylko debug/display; resolved DB category jest source of truth. |
| `quantity` | uint32 | ✅ | `EditableItem.Quantity` | Rozmiar stacka. **Nie może być 0.** |
| `upgrade` | int | optional | `EditableItem.CurrentUpgrade` | Domyślnie 0; range `[0, MaxUpgrade]` walidowany przez DB przy imporcie. |
| `infusionName` | string | optional | `EditableItem.InfusionName` | Musi być w `db.InfuseTypes` jeśli != `""`. Empty / `"Standard"` → un-infused. |
| `aowItemID` | uint32 (pointer + `omitempty`) | optional | `EditableItem.CurrentAoWItemID` (gdy `CurrentAoWStatus == AoWStatusCustom`) | Patrz §6.4. **Nigdy `0`, nigdy in-save handle.** |
| `container` | string | ✅ | `"inventory"` / `"storage"` | Musi pasować do rodzica array (validator wymusza). |
| `position` | int | ✅ | array index po stable sort | Stabilna pozycja w containerze; rozbieżność vs `EditableItem.Position` → warning `position_normalized`. |

### 6.4. Kodowanie AoW

Pole `aowItemID` używa `*uint32 + omitempty`. Decyzja per stan AoW broni źródłowej (zob. [54-ash-of-war §16.4](54-ash-of-war.md)):

| `CurrentAoWStatus` | Wynik JSON |
|---|---|
| `"custom"` z `CurrentAoWItemID != 0` | `"aowItemID": <id>` (semantyczny ItemID gema, prefix `0x8…`) |
| `"none"` | pole pominięte |
| `"missing"` (dangling handle) | pole pominięte + warning `aow_missing_skipped` |
| `"shared"` (handle współdzielony) | pole pominięte + warning `aow_shared_skipped` |
| pusty / non-weapon | pole pominięte |

Eksporter **nigdy** nie zapisuje:

- `"aowItemID": 0` (zero jest reserved dla "brak custom AoW", którego encoding to "pole pominięte"),
- in-save sentinel handle (`0x00000000` / `0xFFFFFFFF`) — to są save-local addresses,
- AoW GaItem handle (`0xC0…`) — to też save-local.

Pełna semantyka sentineli AoW i shared-handle invariantu — [54-ash-of-war §6 i §11](54-ash-of-war.md).

### 6.5. Pola wykluczone

`EditableItem` ma więcej pól niż `TemplateItem`. Wykluczone (test `TestSchemaJSON_OmitsForbiddenFields`):

| Pole `EditableItem` | Powód wykluczenia |
|---|---|
| `originalHandle` | Save-local handle GaItem. |
| `currentAoWHandle` | Save-local handle AoW GaItem. |
| `uid` | UUID sesji workspace; reroll przy każdej sesji. |
| `acquisitionIndex` | Per-character licznik chronologii ([52-acquisition-sort-stride2](52-acquisition-sort-stride2.md)); odnawiany przy save. |
| `pendingAoWItemID`, `pendingAoWName`, `pendingAoWClear`, `hasPendingWeaponPatch` | RAM-only edycje workspace; eksporter mirroruje **zapisany** stan, nie pending request. |
| `hasGaItem`, `hasCurrentAoW`, `currentAoWShared`, `currentAoWStatus` | Flagi pochodne; rekompilowane przy imporcie z DB docelowego save'a. |
| `isWeapon`, `isArmor`, `isTalisman` | Pochodne z DB lookup; mogą drift'ować z templatem. |
| `maxUpgrade` | Stała z DB; nigdy nie autorytatywna w templatcie. |
| `iconPath` | Kosmetyka; zlokalizowane ścieżki zasobów. |

Pass-through records (`UnsupportedInventoryRecords`, `UnsupportedStorageRecords`) — itemy poza allow-list'ą Phase 1 — też wykluczone. Nie przeżyłyby importu opartego o `editor.AddItem`.

---

## 7. Portable payload rule

Reguła podsumowująca §6.5: **template nie może przenosić żadnej wartości, która ma sens tylko w przestrzeni handle konkretnego save'a**.

Konkretne zakazy:

| Forbidden | Powód |
|---|---|
| GaItem handle dowolnego typu (`0x80…` weapon, `0xC0…` AoW, etc.) | Save-local; alokowany per slot. |
| `AoWGaItemHandle` raw value (sentinel albo `0xC0…`) | Save-local; obsługiwany przez wartość semantyczną `aowItemID`. |
| Session UID, OriginalHandle, acquisition index | Per-session lub per-character. |
| Pending* fields | RAM-only w workspace; nie reprezentują zapisanego stanu. |
| ItemID broni z encoded upgrade/infusion (`baseID + level + infusionOffset`) | Template przechowuje `baseItemID` + osobno `upgrade` + `infusionName`; pełny encoded ID jest reconstructed przy imporcie. |

Test regresyjny: `backend/templates/schema_test.go::TestSchemaJSON_OmitsForbiddenFields` marshaluje pełny payload i grep-asercją sprawdza brak forbidden field names w JSON.

`needs verification`: brak automatycznego skanu binarnego po marshalled JSON — test polega na nazwach pól. Hipotetyczne future fields w `EditableItem` zostaną automatycznie wykluczone tylko jeśli `omitempty` jest poprawnie ustawione i nazwa nie jest w whiteliście testu.

---

## 8. Export flow

### 8.1. Kontrakt funkcji core

```go
// backend/templates/export.go
func BuildTemplateFromSnapshot(
    snap editor.InventoryWorkspaceSnapshot,
    opts ExportOptions,
) (*BuildTemplate, *ExportReport, error)
```

Zachowanie:

1. Co najmniej jeden z `opts.IncludeInventory` / `opts.IncludeStorage` musi być true. Inaczej → error przed produkcją.
2. Dla każdego włączonego containera: stable-sort `EditableItem.Position`, projekcja na `TemplateItem`. Rozbieżność `Position` vs final array index → warning `position_normalized`.
3. `BaseItemID == 0` lub `Quantity == 0` → **Go error** (nie warning), brak template'u.
4. AoW handling per §6.4 (status-driven, custom → emit, missing/shared → skip + warning).
5. Stempel `Schema`, `Version`, `CreatedAt` (`time.Now().UTC()` lub `opts.Now`), `AppVersion`, `Metadata`.

Funkcja jest **pure read** z `snap` — żadne pole snapshotu nie jest mutowane (testowane przez `TestExport_*`).

### 8.2. Walidator

```go
func ValidateBuildTemplate(tpl *BuildTemplate) error
```

Scope (sprawdzenia strukturalne, **brak DB lookups**):

- `tpl != nil`
- `tpl.Schema == SchemaKey`
- `0 < tpl.Version ≤ SchemaVersion`
- `tpl.Sections.InventoryWorkspace != nil`
- Przynajmniej jeden z `inventoryItems` / `storageItems` niepusty
- Per item: `BaseItemID != 0`, `Quantity != 0`, `Container` zgodne z arrayem rodzica

DB-level checks (item istnieje, AoW kompat) są w Phase C — patrz §9.

### 8.3. Endpointy App (Phase B)

```go
func (a *App) ExportBuildTemplateJSON(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error)
func (a *App) ExportBuildTemplateToFile(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error)
```

Wspólne zachowanie:

- Resolve sesji po `sessionID` (`a.editSessions[sessionID]`). Nieznana → error `inventory edit session %q not found`.
- Stempel `Metadata.SourceCharacterIndex` z sesji, `Metadata.SourceCharacterName` z aktualnego save'a (pusty jeśli brak save'a).
- `BuildTemplateFromSnapshot` + `ValidateBuildTemplate` — walidacyjne błędy są wrapped Go errors.
- **Nigdy** `SaveInventoryWorkspaceChanges`, **nigdy** mutacja `slot.Data`, **nigdy** alokacja handle. Workspace `Dirty=true` jest legalnym źródłem eksportu — feature istnieje by user mógł zapisać build przed kliknięciem Save.

`ExportBuildTemplateJSON` zwraca payload jako `string` w `BuildTemplateExportResult.JSON`. `ExportBuildTemplateToFile` pokazuje native save-file dialog (`runtime.SaveFileDialog`), pisze plik mode 0644. Anulowanie dialogu (pusta ścieżka) **nie** jest errorem — wynik z `Path == ""` i UI cicho ignoruje.

Warningi z `ExportReport` są przepuszczane do `BuildTemplateExportResult.Warnings`.

### 8.4. Kody warningów

| Kod | Trigger | Stabilność |
|---|---|---|
| `aow_missing_skipped` | `CurrentAoWStatus == "missing"` (dangling handle) | stable, asserted in tests |
| `aow_shared_skipped` | `CurrentAoWStatus == "shared"` (multi-weapon ref) | stable, asserted in tests |
| `position_normalized` | `EditableItem.Position != array_index_po_sort` | stable, asserted in tests |

---

## 9. Import / preview flow

### 9.1. Kontrakt funkcji core

```go
// backend/templates/import.go
func ParseBuildTemplateJSON(data []byte) (*BuildTemplate, error)
func PreviewBuildTemplateImport(tpl *BuildTemplate, opts ImportPreviewOptions) ImportPreviewReport
```

`ParseBuildTemplateJSON` = `json.Unmarshal` + `ValidateBuildTemplate`. Malformowany JSON / wrong schema → Go error.

`PreviewBuildTemplateImport` = **dry-run** validation przeciw live DB. **Nie** mutuje workspace, **nie** alokuje handle, **nie** pisze save. Pakiet `backend/templates` nie importuje `backend/editor` ani `backend/core` — gwarancja strukturalna.

### 9.2. Endpointy App (Phase C)

```go
func (a *App) PreviewBuildTemplateImportJSON(jsonText string) (templates.ImportPreviewReport, error)
func (a *App) PreviewBuildTemplateImportFromFile() (LoadedTemplatePreview, error)
```

`LoadedTemplatePreview` bundluje `Report`, `JSON`, `Path` — pozwala UI przekazać payload do Apply bez ponownego otwarcia dialogu.

Malformowany payload nie produkuje Go error — wraca jako `ImportPreviewReport` z `OK=false` i jednym errorem `structure_invalid`. To pozwala frontendowi renderować parse i per-item failures przez ten sam panel.

Anulowanie file dialogu → sentinel `cancelledPreviewReport` (`OK=false`, puste error/warning/summary). Frontend wykrywa go po `isCancelledPreview` i nie pokazuje toasta ani modal.

### 9.3. Reguły walidacji per item

| Kod | Severity | Trigger |
|---|---|---|
| `structure_invalid` | error | `ParseBuildTemplateJSON` zwrócił Go error (nieparsowalne / niezgodne schema) |
| `schema_invalid` | error | `ValidateBuildTemplate` odrzucił non-nil template po konstrukcji |
| `unknown_item` | error | `db.GetItemDataFuzzy(BaseItemID).Name == ""` LUB AoW item nie w DB |
| `quantity_non_positive` | error | `Quantity == 0` (defense-in-depth przy validator) |
| `upgrade_out_of_range` | error | `Upgrade < 0` lub `Upgrade > db.ItemData.MaxUpgrade` |
| `unknown_infusion` | error | `InfusionName != ""` i brak w `db.InfuseTypes` |
| `aow_not_weapon_target` | error | `aowItemID != nil` ale target category nie w `{melee_armaments, ranged_and_catalysts, shields}` |
| `aow_not_ash_category` | error | `aowItemID` resolves, ale jego DB category nie jest `"ashes_of_war"` |
| `aow_incompatible` | error | `db.IsAshOfWarCompatibleWithWeapon` zwrócił `(false, true)` — known incompatible |
| `aow_compat_unknown` | error | `db.IsAshOfWarCompatibleWithWeapon` zwrócił `(_, false)` — **fail-closed** |
| `unsupported_category` | error | (reserved — używany przez apply-side checks) |
| `capacity_exceeded` | error | Per-container slot cap (`CommonItemCount` / `StorageCommonCount`) przekroczony — dodawany przez apply, nie preview |
| `name_mismatch_ignored` | warning | Template `Name` różni się od DB; DB jest source of truth |
| `unknown_mode` | warning | `ImportPreviewOptions.Mode` != `""`/`"append"` (forward-compat) |

`OK = (len(Errors) == 0)`. Warningi nigdy nie blokują importu.

### 9.4. Summary

`ImportPreviewSummary` bucketuje itemy po **resolved DB category** (nie po debug-only template `Category`):

- `Weapons` ← `{melee_armaments, ranged_and_catalysts, shields}`
- `Armor` ← `{head, chest, arms, legs}`
- `Talismans` ← `{talismans}`
- `Stackables` ← cokolwiek innego co zresolvowało się w DB
- `AoWAssignments` ← licznik itemów których AoW przeszło wszystkie compat checks

Items, które failed `unknown_item` / `quantity_non_positive`, są wykluczone z summary (continue w `previewItems`). Items z nie-fatalnymi błędami (np. `upgrade_out_of_range`) nadal liczą się, by user widział intended shape.

### 9.5. Fail-closed AoW compatibility — rationale

Trade-off portability vs safety jest jawny i celowy:

- **Portability**: template nie zna `wepType` ani `canMountWep_*` bitmask docelowego save'a. Storage przenośnych ItemID pozwala apply w save'ach z innym DLC / patch level.
- **Safety**: ciche akceptowanie nieznanego AoW assignment pozwoliłoby template zaaplikować stan, którego gra nie może reprezentować ([54-ash-of-war §8.4](54-ash-of-war.md) — pełna macierz fail-closed vs passthrough).

Decyzja: preview/apply **fail-closes** na `known=false`. User dostaje precyzyjny komunikat `"AoW compatibility data missing for X on Y — failing closed"` i może albo:

- zaaplikować template z innym save'em (mapping DB shape pasującą do AoW),
- ręcznie usunąć problematyczny `aowItemID` z template przed apply,
- czekać aż `data.AoWCompatMasks` / `data.WepTypeToCanMountBit` zostaną zaktualizowane dla danego wepType (np. DLC 69/94/95 — patrz [54-ash-of-war §17, §22.L2](54-ash-of-war.md)).

`needs verification`: affinity gating (Heavy Longsword vs Standard Longsword) — preview obecnie waliduje compat wyłącznie przez `wepType`, nie przez konkretną infusion variantu. Template z AoW na infused weapon przejdzie compat check tak długo, jak baseID + wepType pozwalają. Pełna macierz affinity gating opisana jako `needs verification` w [54-ash-of-war §22.L1](54-ash-of-war.md).

---

## 10. Apply flow

### 10.1. Kontrakt endpointów (Phase D)

```go
func (a *App) ApplyBuildTemplateToWorkspaceJSON(sessionID string, jsonText string, opts ApplyTemplateOptions) (ApplyTemplateResult, error)
func (a *App) ApplyBuildTemplateToWorkspaceFromFile(sessionID string, opts ApplyTemplateOptions) (ApplyTemplateResult, error)
```

```go
type ApplyTemplateResult struct {
    Preview   templates.ImportPreviewReport       `json:"preview"`
    Workspace editor.InventoryWorkspaceSnapshot   `json:"workspace"`
    Applied   bool                                `json:"applied"`
}
```

### 10.2. Invarianty Apply

- **RAM-only**. Mutuje `sess.Workspace` (`Dirty=true` po success). Nigdy nie woła `SaveInventoryWorkspaceChanges`, nie pisze `slot.Data`, nie alokuje GaItem handle. Save jest nietknięty dopóki user nie kliknie "Save changes".
- **Whitelist mode**. Phase D obsługuje tylko `mode="append"` (pusty string normalizowany). `"replace-inventory"` i `"replace-all"` są reserved → Go error.
- **Existing mutation path**. Itemy dopisywane przez `editor.AddItem` (ten sam call site co AddItem modal); weapon pola aplikowane przez `editor.UpdateWeapon` (ten sam co WeaponEditModal w workspace mode — [54-ash-of-war §16.3](54-ash-of-war.md)). Brak alternatywnej ścieżki.

### 10.3. Kolejność walidacji (early-exit per krok)

1. **Mode whitelist** — `""` lub `"append"`. Inne → Go error, brak mutacji.
2. **Session existence** — nieznana sesja → Go error.
3. **`ParseBuildTemplateJSON`** — failures wracają jako `ImportPreviewReport` z `structure_invalid` (NIE Go error), `Applied=false`.
4. **`PreviewBuildTemplateImport`** — per-item DB resolution + AoW compat. Failures zwracają preview report niezmieniony, `Applied=false`.
5. **Capacity preflight** (`capacityPreflight`):
   - inventory: `len(existing) + len(unsupported) + len(template) ≤ CommonItemCount (2688)`
   - storage: `len(existing) + len(unsupported) + len(template) ≤ StorageCommonCount (1920)`
   - Failures dopisują `capacity_exceeded` do `Preview.Errors`, `Applied=false`.
6. **Snapshot for rollback** — `deepCopySnapshot(sess.Workspace)` przed jakąkolwiek mutacją.
7. **RAM apply** — `applyTemplateItemsToWorkspace` iteruje per container:
   - `editor.AddItem(snap, AddItemSpec{BaseItemID, Quantity}, container, len(target_array))` → append mode.
   - Jeśli `IsWeapon` i któreś z `Upgrade>0` / `InfusionName!=""` / `AoWItemID!=nil`: `editor.UpdateWeapon(snap, added.UID, WeaponPatch{...})` z `SetUpgrade` / `SetInfusionName` / `SetAoWItemID` jak potrzebne.
   - Pierwszy error → restore z snapshotu, `Preview.Errors += apply_error`, `Applied=false`.
8. **Mark dirty** — `sess.Workspace.Dirty = true`, re-validate.

### 10.4. Zachowanie apply

- **Append ordering**: imported items lądują **po** istniejących w kolejności template. Istniejące nie są reorderowane.
- **No save-local handles**: każdy importowany item wchodzi jako `Source: added`, `OriginalHandle: 0`. Docelowy Save (przez `ApplyWorkspaceSave`) zaalokuje fresh handle przez allocator z [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) (constraint: `NextArmamentIndex < maxEntries` dla AoW — patrz [54-ash-of-war §15](54-ash-of-war.md)).
- **AoW assignment**: ustawia `PendingAoWItemID` / `PendingAoWName` + `HasPendingWeaponPatch=true`. Save resolvuje przez `editor.executePendingAoWPatches` → `core.PatchWeaponAoW` (allocate + rebuild — [54-ash-of-war §14](54-ash-of-war.md)).
- **Transactional rollback** dotyczy wyłącznie warstwy workspace (Phase D). Po Save (Phase 1.7+) fail dalej w ścieżce `ApplyWorkspaceSave` jest osobnym problemem.
- **Dialog cancellation**: `ApplyBuildTemplateToWorkspaceFromFile` z pustą ścieżką → `cancelledApplyResult` (`Applied=false`, pusty preview, bieżący workspace). Brak mutacji, brak toasta, brak errora.

### 10.5. Frontend Phase D flow

- Preview modal akceptuje opcjonalny `onApply` callback. Gdy provided AND `report.OK==true`, button "Apply to workspace" jest renderowany.
- `PreviewBuildTemplateImportFromFile` zwraca `LoadedTemplatePreview` z `JSON` żeby Apply mógł reuse payload bez ponownego dialogu.
- Po udanym apply: `useInventoryWorkspace.replaceSnapshot(result.Workspace)`, modal close, toast: *"Template applied to workspace. Click Save changes to persist."*
- Apply na `Applied=false` (capacity overflow post-preview): modal pozostaje open i re-renderuje nowy report.

`needs verification`: Phase D rollback testuje happy path + capacity overflow + structure invalid. Edge case "rollback po częściowym AddItem N gdzie N+1 fails" jest testowany pośrednio przez `deepCopySnapshot`, ale brak dedykowanego testu, że `Validation` jest restored w pełni (slices). Test `TestApplyBuildTemplate*` w `app_templates_test.go` należy sprawdzić pod kątem coverage.

---

## 11. Inventory and item records

Build Template eksportuje wyłącznie **editable allow-list** items workspace ([07-inventory](07-inventory.md), [10-storage](10-storage.md)):

| Kategoria | Eksportowane? | Notatka |
|---|---|---|
| `melee_armaments` (bronie + AoW + infusion + upgrade) | ✅ | Pełen weapon record w template. |
| `ranged_and_catalysts` (łuki, catalysty, sacred seals) | ✅ | Z AoW gdy applicable. |
| `shields` | ✅ | Z AoW gdy applicable. |
| `head`, `chest`, `arms`, `legs` | ✅ | Bez modyfikacji (upgrade=0 zawsze). |
| `talismans` | ✅ | Stackable=false, zawsze quantity=1. |
| Stackable misc items (np. herby, runes, weapons w stackable kategoriach) | ✅ | Eksportowane jeśli `EditableItem.IsWeapon == false` i są w workspace allow-list. |
| Spells / sorceries / incantations | ❌ poza scope Phase A | Reserved dla `character.profile` v2+. |
| Key items, gestures, cookbooks | ❌ | Nie są w workspace allow-list. |
| Equipment slots (equipped gear) | ❌ — patrz §12 | Save state, nie workspace item. |
| Spirit Ashes (Ash Summons) | ❌ poza scope Phase A | Reserved. |
| Magic / sorceries / prayers loadout | ❌ poza scope Phase A | Reserved. |
| Character level / stats / attributes | ❌ poza scope Phase A | Reserved dla `character.profile` v2+. |
| Pass-through `UnsupportedInventoryRecords` / `UnsupportedStorageRecords` | ❌ | Itemy poza allow-list — nie przeżyłyby `editor.AddItem`. |

`needs verification`: lista jest pełna dla v1, ale matrycę "co dokładnie jest workspace allow-list" warto cross-check'ować przy każdej zmianie `editor.SupportedCategories`. Frontend `weaponCategories` / `armorCategories` w `import.go` są copy backendowych — brak CI guardu.

---

## 12. Equipment relation

Build Template **nie eksportuje** equipped gear (Equipment section save'a, [06-equipment](06-equipment.md)). Konsekwencje:

- Po Apply + Save: dodane bronie / zbroje wchodzą do inventory **niezaequippowane**. User musi je ręcznie założyć w grze.
- Brak ścieżki write do equipment section w Phase A–E. Nawet jeśli template był eksportowany z equipped weapon, ten fakt nie jest persisted.
- AoW przypisany do equipped weapon w source save → po apply staje się "AoW on inventory weapon" w target save. Resolver gry pokazuje AoW w grze dopiero gdy user założy broń.

`needs verification`: brak testu E2E "equip → save → reload → equip status preserved" w pełnym kontekście Build Template. Equipment section sama w sobie jest pokryta testami w `06-equipment`, ale interakcja z Build Template apply nie jest explicite testowana.

---

## 13. Ash of War relation

Cała semantyka AoW jest w [54-ash-of-war](54-ash-of-war.md). Build Template robi:

1. **Export**: czyta `EditableItem.CurrentAoWItemID` (semantyczny prefix `0x8…`) gdy `CurrentAoWStatus == AoWStatusCustom`. Nigdy nie eksportuje handle (`0xC0…`) ani sentineli (`0x00000000` / `0xFFFFFFFF`).
2. **Preview**: woła `db.IsAshOfWarCompatibleWithWeapon(aowID, baseItemID)`. Fail-closed na `known=false`.
3. **Apply**: ustawia `PendingAoWItemID` w `EditableItem`; save delegujący do `core.PatchWeaponAoW` alokuje fresh AoW GaItem przez existing flow Phase 1.7.

Co Build Template **nie robi**:

- Nie sprawdza affinity gating (`defaultWepAttr` / `configurableWepAttr00..23`) — to jest `needs verification` w [54-ash-of-war §22.L1](54-ash-of-war.md).
- Nie wybiera między strict vs legacy AoW path — apply zawsze idzie przez legacy (allocate + rebuild) na save, bo użytkownik nie ma kontroli nad istniejącymi free copies w target save.
- Nie obsługuje shared-handle conflict resolution — to jest [54-ash-of-war §11](54-ash-of-war.md) (egzekwowane przez `core.PatchWeaponAoW` które mintuje nowy handle).

---

## 14. Infusion and weapon metadata

Weapon-side fields ekstraktu z `EditableItem` (per [54-ash-of-war §16.3](54-ash-of-war.md) WeaponPatch semantics):

| Pole | Source | Apply path |
|---|---|---|
| `upgrade` | `EditableItem.CurrentUpgrade` (po `decodeWeaponUpgradeInfusion`) | `WeaponPatch.SetUpgrade=true, Upgrade=N` → `editor.UpdateWeapon` |
| `infusionName` | `EditableItem.InfusionName` (string z `db.InfuseTypes`) | `WeaponPatch.SetInfusionName=true, InfusionName="Heavy"` → `editor.UpdateWeapon` |
| `aowItemID` | `EditableItem.CurrentAoWItemID` | `WeaponPatch.SetAoWItemID=true, AoWItemID=…` → `PendingAoWItemID` (save handle) |

`editor.UpdateWeapon` jest wołany **tylko** jeśli któreś z fields jest non-default (`Upgrade>0` OR `InfusionName!=""` OR `AoWItemID!=nil`). Brak weapon-side patch dla broni z domyślnymi wartościami → szybsza ścieżka apply.

Encoding ItemID broni przy save jest reconstructed przez `encodeWeaponItemID(baseID, level, infusionName)` w [54-ash-of-war §16.3 + editor/weapon.go](54-ash-of-war.md).

---

## 15. Compatibility and validation

### 15.1. Multi-warstwowa walidacja

| Warstwa | Co waliduje | Kiedy |
|---|---|---|
| `ValidateBuildTemplate` (Phase A) | Schema, struktura, basic invariants. **Brak DB.** | Eksport (output) + przy każdym import parse. |
| `PreviewBuildTemplateImport` (Phase C) | Per-item DB resolution, AoW compat, upgrade/infusion range. | Preview + apply step 4. |
| `capacityPreflight` (Phase D) | Per-container slot caps. | Apply step 5 (po preview). |
| `editor.AddItem` / `editor.UpdateWeapon` (Phase D apply step 7) | Workspace allow-list, AoW DB category, AoW infusion conflict. | Apply step 7 — per item. |
| `editor.Validate` (po apply step 8) | Pending* fields conflicts, `CodePendingAoWUnknown`/`Conflict`. | Po apply (workspace re-validate). |
| `editor.validatePendingAoWChanges` (przy Save, [54-ash-of-war §16.3](54-ash-of-war.md)) | Final AoW compat fail-closed check. | Przy `ApplyWorkspaceSave`, nie przy apply template. |

### 15.2. Co NIE jest walidowane

- **Affinity gating** — `needs verification` (patrz §13).
- **Equipment slot occupancy** — Build Template nie touches equipment.
- **DLC presence in target save** — template z DLC items zaaplikowany na non-DLC save → `unknown_item` (DB doesn't ship DLC entries jeśli DB build is non-DLC). `needs verification` dla edge case'ów.
- **Regulation.bin version drift** — wszystkie DB lookups są przeciw aktualnemu compiled-in DB SaveForge; nie ma version check vs save's regulation.

### 15.3. Forward-compat

| Pole | Zachowanie |
|---|---|
| Nieznane pola w `metadata` | Tolerowane przez `encoding/json` (no tagged ignore — Go cicho zapomina). |
| Nieznane klucze w `sections` | `TemplateSections.InventoryWorkspace` jest jedynym znanym kluczem; nieznane są ignorowane przez parser (tag-based unmarshalling). |
| `ImportPreviewOptions.Mode` != `""`/`"append"` | Warning `unknown_mode` w preview; apply z innym mode → Go error. |
| `version > SchemaVersion` | `ValidateBuildTemplate` zwraca error `unsupported version`. |

---

## 16. Allocator and capacity relation

Build Template **nie** wywołuje `core.allocateGaItem` ani `core.PatchWeapon*` bezpośrednio. Wszystkie alokacje są odroczone do save'a:

| Faza | Allocator touch? |
|---|---|
| Export | ❌ pure read |
| Preview | ❌ pure DB read |
| Apply (Phase D) | ❌ tylko `editor.AddItem` / `editor.UpdateWeapon` (RAM-only) |
| Save (po Apply, `ApplyWorkspaceSave`) | ✅ `editor.executeAdds` → `core.AddItemsToSlotBatch` → `core.allocateGaItem` per item |
| Save AoW (jeśli `PendingAoWItemID`) | ✅ `editor.executePendingAoWPatches` → `core.PatchWeaponAoW` → `core.allocateGaItem` per AoW |

Capacity preflight (§10.3 krok 5) sprawdza tylko per-container slot count (inventory: 2688, storage: 1920) **przed** apply do workspace. Nie sprawdza:

- `NextArmamentIndex < len(GaItems)` (capacity allocator GaItem) — odbywa się przy save i może fail jeśli słot jest blisko pełny + AoW add zawiedzie zgodnie z [54-ash-of-war §15](54-ash-of-war.md).
- `NextAoWIndex < maxEntries` — patrz wyżej.
- `NextGaItemHandle` overflow — bardzo wysoki budżet, ale nie sprawdzany.

`needs verification`: scenariusz "preview OK, capacity preflight OK, apply OK, ale save fails z `armament zone at capacity`" jest możliwy w teorii. Nie ma E2E testu pokrywającego pełną ścieżkę template → apply → save. Brak też user-facing error przy capacity overflow w save (komunikat allocatora przepuszczany jako Go error przez `ApplyWorkspaceSave`).

---

## 17. Error handling and failure semantics

### 17.1. Failure surfaces

| Faza | Failure surface | Format |
|---|---|---|
| Export | Go error (path: backend → app → frontend exception) | `BuildTemplateExportResult{}, error` |
| Preview | `ImportPreviewReport` z `OK=false` + errors | `templates.ImportPreviewReport` |
| Apply structure invalid | `ApplyTemplateResult{Preview.OK=false, Applied=false}` | `Preview.Errors[].Code = "structure_invalid"` |
| Apply preview errors | tak samo jak preview phase | Per-item issues w `Preview.Errors` |
| Apply capacity | tak samo, plus `"capacity_exceeded"` | Per-container summary message |
| Apply mutation (AddItem/UpdateWeapon) | rollback + `Preview.Errors` append + `Applied=false` | Wrapped error w message field |
| Apply mode invalid | Go error | `ApplyTemplateResult{}, error` |
| Apply session not found | Go error | `ApplyTemplateResult{}, error` |
| File dialog cancelled | sentinel non-error | `Applied=false`, puste preview |

### 17.2. Co kod **nie** waliduje (pełna lista safety gaps)

- ❌ Affinity per AoW infusion variant ([54-ash-of-war §22.L1](54-ash-of-war.md)).
- ❌ DLC presence cross-check przy apply (`unknown_item` flaguje pojedyncze itemy, ale brak global "this template requires DLC X").
- ❌ Cross-platform portability (PS4 vs PC save) — template jest format-agnostic; DLC mapping jest jedynym praktycznym constraintem, ale nie ma explicit cross-check.
- ❌ Equipment slot side effects ([06-equipment](06-equipment.md)).
- ❌ Regulation.bin version drift.
- ❌ Save-side allocator failure prediction (zob. §16).
- ❌ Spell / gesture / key item loadout.
- ❌ Character stats / level.

Każda z tych gaps jest `needs verification` przy rozszerzaniu schema lub przy raportach od użytkowników.

---

## 18. Versioning and forward compatibility

| Aspekt | Reguła |
|---|---|
| `schema` field | Twardo `"saveforge.build-template"`. Inne wartości → reject. |
| `version` field | `1 ≤ v ≤ SchemaVersion`. Future v2 musi być backward-compatible lub Major bump. |
| Breaking change policy | Bump `SchemaVersion`. Old templates load read-only z migration path opcjonalnym. |
| Nieznane pola w metadata | `encoding/json` ignoruje (tag-based unmarshal). |
| Nieznane klucze w `sections` | Parser v1 ignoruje (TemplateSections struct ma fixed tagged fields). |
| Forward-compat mode | `ImportPreviewOptions.Mode = "<unknown>"` → warning `unknown_mode`; preview proceeds jak append. Apply z unknown mode → Go error. |
| Hash / integrity check | ❌ brak. Template files są plain JSON; user może edytować ręcznie (jest zachęcany w §19). |
| Schema migration tool | ❌ brak. v2+ powinien dostarczyć migration utility. |

`needs verification`: brak testów dla `version=2` w aktualnym codzie (`SchemaVersion=1` jest jedynym akceptowanym), więc forward-compat reguły są design intent, nie execution-verified.

---

## 19. Lokalna biblioteka (Phase E)

Per-user lokalny store templatów na dysku.

### 19.1. Disk layout

- **Root**: `$UserConfigDir/EldenRing-SaveEditor/templates/`
  - macOS: `~/Library/Application Support/EldenRing-SaveEditor/templates/`
  - Linux: `~/.config/EldenRing-SaveEditor/templates/`
  - Windows: `%APPDATA%\EldenRing-SaveEditor\templates\`
- **Tryb katalogu**: 0700 (tworzony przy pierwszym użyciu).
- **Jeden szablon = jeden plik** `<sanitized-name>-<id-tail>.json`, mode 0644.
- **Indeks** `_index.json` (`LibraryIndexVersion = 1`) — tylko metadane (id, name, description, tags, filename, timestamps, item counts). Nigdy surowych danych save.
- **Atomic writes** — każdy plik pisany jako `.saveforge-tmp-*` + fsync + rename (`atomicWriteFile`). Crash w trakcie zapisu zostawia poprzedni plik nietknięty.
- Katalog **nie może** być reused dla settings ani innych danych.

### 19.2. Recovery semantics

- **Brakujący `_index.json`** → pusty indeks (nie error). User wrzucający pliki ręcznie musi wywołać `RebuildIndex` (frontend: przycisk Refresh).
- **Uszkodzony `_index.json`** → auto-rebuild ze zawartości katalogu (`rebuildIndexLocked`). Pliki nieparsowalne / non-validating są pomijane (zostają na dysku).
- **Rebuild zachowuje ID i `CreatedAt`** dla plików, których filename pasuje do poprzedniego indeksu — UI keyowane po ID jest stabilne między recovery.

### 19.3. Endpointy App (Phase E)

| Metoda | Cel | Mutuje save/workspace? |
|---|---|---|
| `SaveBuildTemplateToLibrary(sessionID, opts)` → `LibraryTemplateEntry` | Zbuduj z aktywnego workspace + zapisz. | ❌ |
| `ListBuildTemplateLibrary()` → `[]LibraryTemplateEntry` | Wpisy z indeksu sorted newest-first. | ❌ |
| `PreviewBuildTemplateFromLibrary(id)` → `LoadedTemplatePreview` | Load + validator dry-run; zwraca JSON do round-tripu Apply. | ❌ |
| `ApplyBuildTemplateFromLibrary(sessionID, id, opts)` → `ApplyTemplateResult` | Deleguje do `ApplyBuildTemplateToWorkspaceJSON`. RAM-only workspace mutation. | workspace tak, save nie |
| `DeleteBuildTemplateFromLibrary(id)` | Usuń plik + wpis indeksu. | ❌ |
| `RenameBuildTemplateInLibrary(id, name, description, tags)` → `LibraryTemplateEntry` | Update metadata w pliku i indeksie; bump `updatedAt`. | ❌ |
| `ExportLibraryBuildTemplateToFile(id)` → `BuildTemplateExportResult` | Save-file dialog + kopia do wybranej ścieżki. Cancel → pusty Path. | ❌ |
| `RebuildBuildTemplateLibraryIndex()` → `[]LibraryTemplateEntry` | Rescan katalogu, przebuduj indeks. | ❌ |
| `GetBuildTemplateLibraryPath()` → `string` | Absolute path katalogu (UI używa w empty-state i footerze). | ❌ |

### 19.4. Invarianty

- **Apply z biblioteki = RAM-only** (delegates to `ApplyBuildTemplateToWorkspaceJSON`).
- **`SaveInventoryWorkspaceChanges` nigdy nie jest wywoływane z akcji biblioteki.** User musi kliknąć Save changes.
- **Delete i Rename dotykają wyłącznie biblioteki** — workspace i save nie są ruszane.
- **Lazy init** przez `App.ensureTemplateLibrary` — testy jednostkowe mogą wstrzyknąć tymczasowy katalog bez `DefaultTemplateLibraryDir`.
- **Settings export NIE jest w zakresie** — to osobny przyszły feature, nie może współdzielić tego katalogu.

### 19.5. Manualne zarządzanie plikami

**Pliki szablonów na dysku są source of truth**; `_index.json` to cache metadanych. User może:

- Skopiować `.json` szablony z innego komputera do katalogu.
- Ręcznie wyedytować `metadata.name` / `metadata.tags`.
- Wyciągnąć szablony żeby zarchiwizować.

Po manualnych zmianach przycisk **Refresh** woła `RebuildBuildTemplateLibraryIndex`:

- Skanuje pliki `*.json`.
- Parsuje + waliduje każdy kandydat; uszkodzone / non-validating są w ciszy pomijane (zostają na dysku).
- Atomowo nadpisuje `_index.json`.
- Zachowuje ID i `createdAt` dla plików matching poprzedni indeks.

### 19.6. Frontend (Phase E)

- Dropdown `Export Template ▾` zyskuje pozycję `Template Library…` otwierającą `TemplateLibraryModal`.
- `ExportTemplateModal` zyskuje opcjonalny callback `onSavedToLibrary` + przycisk **Save to local library** obok istniejącego **Export JSON file**.
- `TemplateLibraryModal` akcje per row: Preview, Apply, Export, Rename (inline), Delete (custom React confirm — bez native dialogu, by testy mogły pod jsdom).
- Header: przycisk **Refresh** wołający `RebuildBuildTemplateLibraryIndex`. Empty-state + footer eksponują ścieżkę biblioteki.
- Apply z biblioteki: `useInventoryWorkspace.replaceSnapshot(result.Workspace)` + toast *"Template applied to workspace. Click Save changes to persist."*

---

## 20. Test coverage

| Plik | Liczba testów | Co lockuje |
|---|---|---|
| `backend/templates/schema_test.go` | 13 | Roundtrip JSON, forbidden fields exclusion, AoW pointer omitempty, validator reject paths (schema/version/empty/zeroes/container mismatch). |
| `backend/templates/export_test.go` | 15 | Happy path inv/storage/mixed, order preserved, position normalization warning, custom AoW emit, none/missing/shared AoW handling, zero baseID/quantity error, pending fields ignored, empty snapshot, `Now` fallback. |
| `backend/templates/import_test.go` | 22 | Parse round-trip, schema reject paths, preview happy path, name mismatch warning, unknown item/infusion/upgrade range, AoW on non-weapon, AoW not ash category, AoW incompatible (known), AoW compat unknown (fail-closed), mode whitelist. |
| `backend/templates/library_test.go` | 14 | Save/load/delete/rename, index recovery (missing/corrupt), unique filename, reject invalid, export to file, atomic write cleanup, default dir. |
| `app_templates_test.go` | 29 | App-level glue: export JSON/file, preview JSON/file, apply JSON/file, capacity preflight, file dialog cancel, rollback po `AddItem` failure. |
| `app_templates_library_test.go` | 17 | Library-bound App methods (Save/List/Preview/Apply/Delete/Rename/Export from library, `RebuildIndex`, `GetBuildTemplateLibraryPath`). |
| `frontend/src/components/templates/__tests__/ExportTemplateModal.test.tsx` | — | UI behavior export modal. |
| `frontend/src/components/templates/__tests__/ImportTemplatePreviewModal.test.tsx` | — | UI behavior preview/apply modal. |
| `frontend/src/components/templates/__tests__/TemplateLibraryModal.test.tsx` | — | UI behavior library modal. |

Łącznie: **110 testów Go** (zliczonych z `grep -c "^func Test"` w 6 plikach) + 3 frontend test files. `needs verification`: liczba scenariuszy w testach frontend nie jest enumerowana per-test w tym dokumencie — pełną listę można uzyskać przez `grep "test(" frontend/src/components/templates/__tests__/*.test.tsx`.

---

## 21. Known limits / needs verification

| # | Obszar | Status |
|---|---|---|
| L1 | Affinity gating w preview (Heavy vs Standard infusion variants) | `needs verification` — preview waliduje compat tylko po `wepType`, nie po infusion variant. Patrz [54-ash-of-war §22.L1](54-ash-of-war.md). |
| L2 | DLC wepType 69/94/95 | `aow_compat_unknown` fail-closes w preview/apply (zob. §9.5). UI nie sugeruje user-facing rozwiązania. |
| L3 | Equipment write API | ❌ brak. Apply zostawia bronie unequipped. `needs verification` dla future Phase. |
| L4 | Spell / gesture / key item loadout export | ❌ poza scope v1. Reserved dla `character.profile` v2+. |
| L5 | Character stats / level / attributes | ❌ poza scope v1. Reserved. |
| L6 | DLC item availability cross-check przy apply | `needs verification` — `unknown_item` flaguje pojedyncze itemy, brak global "needs DLC X" warning. |
| L7 | Save-side allocator failure prediction | `needs verification` — capacity preflight nie sprawdza `NextArmamentIndex` / `NextAoWIndex`. Szczegóły [54-ash-of-war §15](54-ash-of-war.md). |
| L8 | Forward-compat `version=2` testy | `needs verification` — brak testów (SchemaVersion=1 jedyny akceptowany). |
| L9 | Apply rollback macierz | `needs verification` — happy path + capacity + structure invalid są pokryte, ale full Validation slice restoration test brakuje. |
| L10 | Tryby `replace-inventory` / `replace-all` | Reserved w `ApplyTemplateOptions.Mode`, ale **nie zaimplementowane**. Apply z innym mode → Go error. |
| L11 | Cross-platform portability (PS4 vs PC) | `needs verification` — schema jest format-agnostic, ale brak explicit cross-check przy apply. |
| L12 | Test coverage frontend modali | Trzy test files istnieją (Export/Import/Library), ale macierz scenariuszy nie enumerowana w tym dokumencie. |

---

## 22. Cross-references

- [03-gaitem-map](03-gaitem-map.md) — GaItem binary model i handle prefix semantics.
- [06-equipment](06-equipment.md) — equipped gear (poza scope Build Template).
- [07-inventory](07-inventory.md) / [10-storage](10-storage.md) — inventory / storage section model.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator capacity rules przy save.
- [36-inventory-categories-game-order](36-inventory-categories-game-order.md) — kategorie itemów (mapowanie DB).
- [37-character-presets](37-character-presets.md) — osobny mechanizm, different scope.
- [43-transactional-item-adding](43-transactional-item-adding.md) — Add Items pipeline używany przy save.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — acquisition sort, dlaczego `acquisitionIndex` jest wykluczony z template.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — prior art workspace mutations.
- [54-ash-of-war](54-ash-of-war.md) — canonical chapter dla AoW (sentinele, write paths, compat matrix, workspace modal).

---

## 23. Źródła

- Kod schema/pipeline: `backend/templates/schema.go`, `backend/templates/export.go`, `backend/templates/import.go`, `backend/templates/library.go`.
- App glue: `app_templates.go`.
- Workspace / editor: `backend/editor/workspace.go` (`EditableItem`, `InventoryWorkspaceSnapshot`, stałe `AoWStatus*`), `backend/editor/add.go` (`AddItemSpec`), `backend/editor/weapon.go` (`WeaponPatch`, `UpdateWeapon`), `backend/editor/save.go` (`ApplyWorkspaceSave`).
- DB: `backend/db/db.go` (`GetItemDataFuzzy`, `InfuseTypes`, `IsAshOfWarCompatibleWithWeapon`).
- Capacity: `backend/core/offset_defs.go` (`CommonItemCount`, `StorageCommonCount`).
- Frontend: `frontend/src/components/templates/ExportTemplateModal.tsx`, `ImportTemplatePreviewModal.tsx`, `TemplateLibraryModal.tsx`, `frontend/src/components/SortOrderTab.tsx`.
- Testy: `backend/templates/{schema,export,import,library}_test.go`, `app_templates_test.go`, `app_templates_library_test.go`, `frontend/src/components/templates/__tests__/*.test.tsx`.
- Historia: commity `feat(templates): build template schema + exporter (Phase A)`, `feat(templates): export modal + dropdown (Phase B)`, `feat(templates): import preview (Phase C)`, `feat(templates): apply to workspace (Phase D)`, `feat(templates): local library (Phase E)` — dokładne SHA dostępne przez `git log --grep="templates"`.
