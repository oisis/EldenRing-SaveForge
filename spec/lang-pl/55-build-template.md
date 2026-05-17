# 55 — Build Template

> **Type**: Design doc
> **Status**: 🔲 Planowany — Phase A + B + C zaimplementowane (`backend/templates/`, `app_templates.go`, UI SortOrderTab z export + preview modals), Phase D/E pending
> **Scope**: Przenośna reprezentacja JSON snapshotu Inventory Workspace. Definiuje schemat `saveforge.build-template` v1, kontrakt eksportu (`BuildTemplateFromSnapshot`), reguły obsługi Ash of War oraz zakres pól celowo wykluczonych z payloadu — tak by szablon można było zaaplikować na dowolnym save'ie bez kolizji z jego przestrzenią handle'i.

---

## 1. Cel

Gracze chcą bootstrapować nową postać znanym setupem — te same bronie, poziomy upgrade'ów, infuzje, Ashes of War, sort order — bez ręcznego dodawania każdego itemu. "Build template" to przenośna reprezentacja tego setupu, która jest source-of-truth.

Szablon przechowuje **wyłącznie identyfikatory gameplay** przeżywające transfer między save'ami. Celowo wyklucza wszystko co wiąże dane z konkretnym save'em: handle GaItem, UIDs sesji, indeksy acquisition, flagi mapy GaItem. Dzięki temu szablony można bezpiecznie współdzielić między użytkownikami, platformami i postaciami w obrębie tego samego save'a.

Ten dokument opisuje schemat v1, kontrakt eksportera (Phase A) i placeholder design importu (Phase D/E). Settings export — preferencje UI, motyw, deploy targets — to osobny feature z własnym schematem i jest poza scope tego doca.

---

## 2. Nagłówek schematu

```json
{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-17T12:34:56Z",
  "appVersion": "0.15.0-beta",
  "metadata": { ... },
  "sections": { ... }
}
```

| Pole | Typ | Wymagane | Uwagi |
|---|---|---|---|
| `schema` | string | ✅ | Musi być równe `"saveforge.build-template"`. Importer odrzuca każdą inną wartość. |
| `version` | int | ✅ | Wersja schematu. W Phase A tylko v1. Importer akceptuje `1 ≤ v ≤ SchemaVersion`. |
| `createdAt` | string | ✅ | Timestamp RFC 3339 UTC. Informacyjny. |
| `appVersion` | string | optional | Wersja SaveForge która wyprodukowała szablon. Informacyjna. |
| `metadata` | object | optional | Etykiety dla użytkownika (nazwa, opis, tagi, źródłowa postać). Brak load-bearing pól. |
| `sections` | object | ✅ | Payload sekcji, klucz = stabilny identyfikator sekcji. |

Forward-compat: nieznane pola w `metadata` są tolerowane przez `encoding/json` w Go i ignorowane. Nieznane klucze sekcji są po cichu odrzucane przez parser v1 — v2 podejmie je później.

---

## 3. Sekcja: `inventory.workspace`

Payload Phase A. Lustrzane odbicie tego podzbioru `editor.InventoryWorkspaceSnapshot` który jest przenośny.

```json
"sections": {
  "inventory.workspace": {
    "inventoryItems": [ TemplateItem, ... ],
    "storageItems":   [ TemplateItem, ... ]
  }
}
```

Oba arraye muszą być obecne (mogą być puste), ale `ValidateBuildTemplate` odrzuca szablon gdy oba są puste — pusty szablon nie ma zastosowania.

### 3.1. `TemplateItem`

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

| Pole | Typ | Wymagane | Uwagi |
|---|---|---|---|
| `baseItemID` | uint32 | ✅ | Bazowy ID z DB (bez kodowania upgrade/infusion). Source of truth przy imporcie. **Nie może być 0.** |
| `name` | string | optional | Tylko debug/display. Nie używane przy imporcie. |
| `category` | string | optional | Tylko debug/display. |
| `quantity` | uint32 | ✅ | Rozmiar stacka. **Nie może być 0.** |
| `upgrade` | int | optional | Poziom upgrade broni (0–25 / 0–10 w zależności od broni). Domyślnie 0. |
| `infusionName` | string | optional | Jedna z nazw `db.InfuseTypes` (`Heavy`, `Keen`, `Quality`, …). Brak lub `""` oznacza "Standard". |
| `aowItemID` | uint32 (pointer) | optional | Item ID custom Ash of War. Pominięty gdy nie ma custom AoW. **Nigdy `0`.** |
| `container` | string | ✅ | `"inventory"` lub `"storage"`. Musi pasować do rodzica. |
| `position` | int | ✅ | Stabilna pozycja sortowania w containerze, 0-indexed. |

### 3.2. Kodowanie AoW

`aowItemID` to pointer + `omitempty`, więc kontrakt JSON jest taki:

| Source `CurrentAoWStatus` | Wynik JSON |
|---|---|
| `"custom"` z `CurrentAoWItemID != 0` | `"aowItemID": <id>` |
| `"none"` (brak custom AoW, sentinel handle) | pole pominięte |
| `"missing"` (handle dangling) | pole pominięte + warning `aow_missing_skipped` |
| `"shared"` (handle współdzielony przez kilka broni) | pole pominięte + warning `aow_shared_skipped` |
| pusty status (non-weapon) | pole pominięte |

Eksporter **nigdy** nie zapisuje `aowItemID: 0` i **nigdy** nie zapisuje in-save sentinel handle (`0x00000000` / `0xFFFFFFFF`). Oba przeciekałyby save-local addressing do szablonu.

---

## 4. Pola wykluczone

Te pola istnieją na `editor.EditableItem`, ale celowo **nie** są emitowane przez eksporter. Są session- lub save-local i wiązałyby szablon z przestrzenią handle jednego save'a:

- `originalHandle` — handle GaItem w źródłowym save'ie.
- `currentAoWHandle` — handle GaItem źródłowego AoW.
- `uid` — UUID sesji workspace.
- `acquisitionIndex` — per-character licznik chronologii; będzie odnowiony przy imporcie.
- `pendingAoWItemID`, `pendingAoWName`, `pendingAoWClear`, `hasPendingWeaponPatch` — niezapisane edycje RAM-only; eksporter mirrorruje *aktualnie zapisany* stan, nie pending request.
- `hasGaItem`, `hasCurrentAoW`, `currentAoWShared`, `currentAoWStatus` — flagi pochodne; rekompilowane przy imporcie z DB docelowego save'a.
- `isWeapon`, `isArmor`, `isTalisman` — pochodne z lookup DB przy imporcie; może tylko drift'ować.
- `maxUpgrade` — stała z DB; nigdy nie autorytatywna w szablonie.
- `iconPath` — kosmetyka; zlokalizowane ścieżki zasobów.

Pass-through records (`UnsupportedInventoryRecords`, `UnsupportedStorageRecords`) są też wykluczone z Phase A. Opisują przedmioty poza allow-list'ą Phase 1 i nie przeżyłyby importu opartego o `AddInventoryWorkspaceItem`.

Test regresji w `backend/templates/schema_test.go` (`TestSchemaJSON_OmitsForbiddenFields`) marszuje w pełni wypełniony szablon i grep-asercjami sprawdza że żadna z tych nazw pól nie pojawia się w JSON.

---

## 5. Kontrakt eksportera (Phase A)

```go
func BuildTemplateFromSnapshot(
    snap editor.InventoryWorkspaceSnapshot,
    opts ExportOptions,
) (*BuildTemplate, *ExportReport, error)
```

Zachowanie:

- `opts.IncludeInventory` i `opts.IncludeStorage` niezależnie gatują dwa arraye. Co najmniej jeden musi być true.
- Itemy są stable-sortowane po `EditableItem.Position`; wynikowy array index staje się `position` w szablonie. Rozbieżność powoduje jeden warning `position_normalized` per affected item.
- `BaseItemID == 0` i `Quantity == 0` są błędami eksportera (zwracają `error`, brak produkcji szablonu).
- Itemy `Source: added` i `Source: original` są eksportowane identycznie — szablon jest shape-only.
- `opts.Now` jest exposed dla testów; produkcyjne wywołania zostawiają zero, eksporter używa `time.Now().UTC()`.

`ExportReport.Warnings[]` jest niepusty gdy AoW state został pominięty lub pozycje znormalizowane. Każdy wpis ma `{code, uid, container, position, message}`. UI Phase B pokazuje warningi przed zapisem do pliku.

### 5.1. Kody warningów

| Kod | Znaczenie |
|---|---|
| `aow_missing_skipped` | Handle AoW broni nie został zresolve'owany w `slot.GaMap`. AoW nie wyeksportowany. |
| `aow_shared_skipped` | Handle AoW broni był referowany przez ≥2 broni (save corruption). AoW nie wyeksportowany. |
| `position_normalized` | Zgłoszony `Position` itemu nie odpowiadał ostatecznemu array indeksowi po stable sort. |

Te stringi są stabilne; UI importerów i testy je asercją sprawdzają.

---

## 6. Kontrakt walidatora

```go
func ValidateBuildTemplate(tpl *BuildTemplate) error
```

Scope Phase A: tylko sprawdzenia strukturalne i invariantowe. **Brak lookup'ów DB.** Waliduje:

- `tpl != nil`
- `tpl.Schema == "saveforge.build-template"`
- `0 < tpl.Version ≤ SchemaVersion`
- `tpl.Sections.InventoryWorkspace != nil`
- Co najmniej jeden z `inventoryItems` / `storageItems` jest niepusty
- Per item: `BaseItemID != 0`, `Quantity != 0`, `Container` zgodne z arrayem rodzica

Sprawdzenia na poziomie DB (item istnieje, AoW kompatybilne z typem broni, quantity w zakresie `MaxInventory*(NG+1)`) należą do ścieżki importu (Phase D), gdzie dostępny jest kontekst docelowego save'a.

---

## 7. Roadmapa faz

| Faza | Scope | Status |
|---|---|---|
| **A** | Schemat + eksporter (`backend/templates/`), spec doc, testy | ✅ |
| **B** | Wails bindings `ExportBuildTemplateJSON` / `ExportBuildTemplateToFile`, dropdown + modal w SortOrderTab | ✅ |
| **C** | Preview importu: `ParseBuildTemplateJSON`, `PreviewBuildTemplateImport`, endpointy App, `ImportTemplatePreviewModal` (read-only) | ✅ |
| **D** | Lokalna biblioteka szablonów pod `UserConfigDir`/`templates/` | 🔲 pending |
| **E** | `ApplyBuildTemplateToWorkspace` — re-use istniejącej ścieżki `AddInventoryWorkspaceItem` + `SaveInventoryWorkspaceChanges` | 🔲 pending |
| 2+ | Sekcja `character.profile` (level, stats, talisman slots), opt-in via `$enabled` | 🔲 odroczone |

### 7.1. Kontrakt endpointów Phase B

Dwie metody-receivery `App` exposed przez Wails:

```go
func (a *App) ExportBuildTemplateJSON(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error)
func (a *App) ExportBuildTemplateToFile(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error)
```

Zachowanie wspólne dla obu:

- Resolve aktywnej `InventoryEditSession` po `sessionID`. Nieznana sesja → error `inventory edit session %q not found`.
- Buduj `templates.ExportOptions` z `opts`, stempl `AppVersion` ze zmiennej `appVersion` z pakietu, wypełnij `Metadata.SourceCharacterIndex` z sesji i `Metadata.SourceCharacterName` z nazwy postaci w save'ie (pusty jeśli brak załadowanego save'a).
- Uruchom `templates.BuildTemplateFromSnapshot`, potem `ValidateBuildTemplate`. Walidacyjne błędy idą jako wrapped errors.
- **Nigdy** nie wołaj `SaveInventoryWorkspaceChanges`, **nigdy** nie mutuj slot data, **nigdy** nie alokuj handle'i. Eksport to czysty read ze snapshotu workspace.
- Workspace z `Dirty=true` jest poprawnym źródłem eksportu — feature istnieje właśnie po to by user mógł zachwycić build przed kliknięciem Save Changes.

`ExportBuildTemplateJSON` zwraca payload JSON jako string (pole `JSON` w `BuildTemplateExportResult`) i całkowicie pomija I/O na disk. Przydatne do podglądu i testów.

`ExportBuildTemplateToFile` pokazuje natywny save-file dialog przez `runtime.SaveFileDialog`, zapisuje plik z mode 0644, zwraca wybraną ścieżkę. Anulowanie (pusta ścieżka z dialogu) **nie** jest errorem: wynik zwracany z `Path == ""`, frontend cicho ignoruje. Domyślna nazwa pliku jest derived z `metadata.name`, fallback do `<character>-build.json`, potem `saveforge-build-template.json`.

Warningi z `templates.ExportReport` są przekazywane przez `BuildTemplateExportResult.Warnings` tak by UI mogło je pokazać po udanym zapisie bez mutowania state workspace'a.

Settings export (motyw, deploy targets, preferencje UI) **nie** jest częścią tego designu. Będzie osobnym schematem `saveforge.settings-export` jeśli/gdy stanie się priorytetem.

---

## 7.2. Kontrakt endpointów Phase C (Import Preview)

Dwie metody-receivery `App` exposed przez Wails:

```go
func (a *App) PreviewBuildTemplateImportJSON(jsonText string) (templates.ImportPreviewReport, error)
func (a *App) PreviewBuildTemplateImportFromFile() (templates.ImportPreviewReport, error)
```

Zachowanie wspólne dla obu:

- **Tylko dry-run.** Sesja nie jest resolvowana, workspace nie mutowany, save nie zapisywany, handle nie alokowane. `PreviewBuildTemplateImport` jest w pakiecie (`backend/templates`), który NIE importuje `backend/editor` ani `backend/core` — brak tych importów to gwarancja strukturalna.
- Parsuj JSON przez `templates.ParseBuildTemplateJSON`, który wrapuje `json.Unmarshal` + `ValidateBuildTemplate`. Malformowane payloady i schema mismatch produkują NON-OK report z jednym errorem `structure_invalid`, NIE Go `error` — to pozwala frontendowi renderować parse i per-item failures przez ten sam panel.
- Walidacja per-item idzie przeciw live DB i produkuje wpisy `ImportPreviewIssue` z `Code`, `Container`, `Position`, item ID-kami tak by UI mogło deep-linkować.

### Reguły walidacji

| Kod | Severity | Trigger |
|---|---|---|
| `structure_invalid` | error | parse JSON / `ValidateBuildTemplate` odrzucił na warstwie schematu |
| `schema_invalid` | error | `ValidateBuildTemplate` odrzucił non-nil template po konstrukcji |
| `unknown_item` | error | `db.GetItemDataFuzzy(BaseItemID).Name == ""` LUB AoW item brak w DB |
| `quantity_non_positive` | error | `Quantity == 0` (defence in depth obok schema validatora) |
| `upgrade_out_of_range` | error | `Upgrade < 0` lub `Upgrade > db.ItemData.MaxUpgrade` |
| `unknown_infusion` | error | `InfusionName != ""` i brak w `db.InfuseTypes` |
| `aow_not_weapon_target` | error | `aowItemID != nil` ale target item DB category nie w {melee_armaments, ranged_and_catalysts, shields} |
| `aow_not_ash_category` | error | `aowItemID` resolves ale jego DB category nie jest `"ashes_of_war"` |
| `aow_incompatible` | error | `db.IsAshOfWarCompatibleWithWeapon` zwrócił `(false, true)` |
| `aow_compat_unknown` | error | `db.IsAshOfWarCompatibleWithWeapon` zwrócił `(_, false)` — **fail-closed** |
| `name_mismatch_ignored` | warning | template `Name` różni się od DB name; DB jest source of truth |
| `unknown_mode` | warning | `ImportPreviewOptions.Mode` to coś innego niż `""` / `"append"` (forward-compat) |

`OK = (len(Errors) == 0)`. Warningi nigdy nie blokują.

### Liczniki summary

`ImportPreviewSummary` bucketuje itemy po **resolved DB category** (nie po debug-only `Category` z template):
- `Weapons` ← `{melee_armaments, ranged_and_catalysts, shields}`
- `Armor` ← `{head, chest, arms, legs}`
- `Talismans` ← `{talismans}`
- `Stackables` ← cokolwiek innego co zresolvowało się w DB
- `AoWAssignments` ← licznik itemów których AoW przeszło wszystkie compat checks

### Fail-closed AoW compatibility

Gdy `db.IsAshOfWarCompatibleWithWeapon` raportuje `known=false` (brak compat bitmask, lub nieznany `wepType`), preview emituje `aow_compat_unknown` i report jest NOT OK. To jest intencjonalne: ciche akceptowanie nieznanego AoW assignment pozwoliłoby templatowi zaaplikować stan którego gra nie może reprezentować.

### Anulowanie file dialogu

`PreviewBuildTemplateImportFromFile` zwraca sentinel report (`OK=false`, puste errors/warnings, puste summary) gdy user dismissował open-file dialog. Frontend używa `isCancelledPreview` do wykrycia tego shape i trzyma preview modal closed — bez toasta, bez errora.

---

## 8. Ścieżka importu (apply design — Phase E)

Phase A–C nie aplikują szablonów do workspace'a. Schemat jest tak zaprojektowany, by go wesprzeć bez zmian. Szkic:

1. `LoadBuildTemplateFromFile(path)` → `*BuildTemplate` + walidacja strukturalna.
2. `PreviewBuildTemplateImport(sessionID, tpl, opts)` → raport diff względem bieżącego workspace:
   - Resolve każdego `baseItemID` przez `db.GetItemDataFuzzy`. Nieznany → error per item.
   - Dla broni z `aowItemID`: sprawdź `db.IsAoWCompatibleWithWepType` względem `wepType` broni. Niekompatybilne → warning per item.
   - Clamp `quantity` do `MaxInventory * (ClearCount + 1)` docelowego save'a (warning gdy clampowano).
3. `ApplyBuildTemplateToWorkspace(sessionID, tpl, mode)` tłumaczy każdy `TemplateItem` na `editor.AddItemSpec` i woła istniejącą mutację `AddInventoryWorkspaceItem`. Workspace zostaje dirty w RAM; nic nie idzie do save'a dopóki user nie kliknie **Save Changes**, które idzie przez istniejące `SaveInventoryWorkspaceChanges` i reuse'uje sprawdzony alokator handle'i w `core.AddItemsToSlotBatch`.

Krytyczny invariant: import **nigdy** nie robi auto-save i **nigdy** nie czyta `originalHandle` z szablonu — szablon żadnego nie niesie.

---

## 9. Źródła

- `backend/editor/workspace.go` — `EditableItem`, `InventoryWorkspaceSnapshot`, stałe `AoWStatus*`.
- `backend/editor/add.go` — `AddItemSpec`, naturalne odbicie `TemplateItem` po stronie importu.
- `backend/editor/save.go` — `ApplyWorkspaceSave`, docelowa ścieżka zapisu przy imporcie.
- `backend/db/db.go` — `GetItemDataFuzzy`, `InfuseTypes`, `IsAoWCompatibleWithWepType`.
- [54-ash-of-war.md](54-ash-of-war.md) — semantyka sentinel handle, invariant shared-handle.
- [37-character-presets.md](37-character-presets.md) — wcześniejszy design eksportu presetów; różny od build templates, ale informuje konwencje wersjonowania.
- [39-inventory-reorder.md](39-inventory-reorder.md), [52-acquisition-sort-stride2.md](52-acquisition-sort-stride2.md), [53-inventory-storage-transfer.md](53-inventory-storage-transfer.md) — prior art UI workspace.
