# 56 — Templates v2 (Partially Implemented Extension)

> **Type**: Design doc
> **Status**: 🔄 Częściowo wdrożone — Phase 0..6 + Phase 6b + Phase 7a + Phase 7a.2 + Phase 9 dostarczone (addytywny schemat `version: 2`, globalny library shell Templates, publiczny YAML import/export, flow create-from-character dla profile/stats z per-field selection, Save to Library, badge v2 w bibliotece i preview, **Phase 5 = Apply z biblioteki + direct imported-YAML Apply dla profile/stats przez `ApplyBuildTemplateV2FromLibraryToCharacter` oraz `ApplyBuildTemplateV2ToCharacterJSON` na canonical JSON wyprodukowanym przez preview importu, Phase 6 = apply-time overrides dla tego samego subsetu profile/stats na obu powierzchniach przez frontend-only mutację canonical JSON przekazaną do tego samego JSON-owego endpointu, Phase 6b = apply-time weapon level override dla istniejącej ścieżki Apply v1 `inventory.workspace` przez addytywne pole `WeaponLevelOverride` na `app_templates.go::ApplyTemplateOptions`, aplikowane po każdym template-side patchu broni przez `editor.UpdateWeapon` + `editor.ClampUpgrade(req, MaxUpgrade)`; w pełni workspace-only, bez zmiany schematu v2, bez writera ekwipunku; kontrolki UI żyją wewnątrz istniejącego dropdownu Templates w `SortOrderTab.tsx`, Phase 7a = pierwsza realna ścieżka Apply v2 dla `inventory.workspace` przerzucona przez aktywną `InventoryEditSession`/`InventoryWorkspaceSnapshot`, gated nowym lookupem `GetActiveInventoryEditSessionForCharacter(charIdx)` oraz nowym addytywnym `ApplyTemplateV2Options.SessionID`; brak sesji dla szablonu z inventory → `IssueCodeInventorySessionRequired`, nieznana / niewłaściwa sesja → `IssueCodeInventorySessionInvalid`; mixed apply profile+stats+inventory.workspace jest atomowy przez dual slot+workspace snapshot rollback; weapon level override Phase 6b w Phase 7a pozostawał feature'em dropdownu v1 i został podłączony do ścieżki v2 w Phase 7a.2 poniżej, Phase 7a.2 = apply-time weapon level override podłączony do ścieżki Apply v2 `inventory.workspace` z Phase 7a przez addytywne pole `WeaponLevelOverride` na `ApplyTemplateV2Options` (reuse v1 typu i walidatora `validateWeaponLevelOverride` 1:1); opcja runtime-only (nigdy w canonical JSON); nowy `WeaponLevelOverridePanel` osadzony w istniejącym `ApplyOverridesModal` i renderowany tylko gdy obecne `selection.inventory.workspace`; fast library Apply nadal nie wysyła override; strukturalnie poprawny override + template profile/stats-only silently ignored; warningi `weapon_level_clamped` / `weapon_unupgradeable` lecą do `ApplyTemplateV2Result.Preview.Warnings`; v1 dropdown Phase 6b w `SortOrderTab.tsx` bez zmian, Phase 9 = import szablonów z `https://` URL przez `PreviewBuildTemplateImportYAMLFromURL` pod pełną listą guardów §12.3 (SSRF); preview URL reużywa tego samego `ImportTemplatePreviewModal`, więc Save to Library / Apply to character / Apply with overrides ship bez zmian również na ścieżce URL**). Apply dla v2 equipment / equipped-talismans / spells / appearance pozostaje zablokowany — Phase 7b+ (writery equipment/talismans/spells, appearance przez preset, multi-character pack) pozostają wyłącznie design. Addytywne rozszerzenie wdrożonego podsystemu Build Template udokumentowanego w [55-build-template](55-build-template.md).
> **Scope**: Addytywne rozszerzenie istniejącego `saveforge.build-template` JSON v1 do `version: 2` — z publicznym formatem YAML do udostępniania na zewnątrz, nowym sidebar entry point `Templates`, granular selection model, sekcjami całej postaci (profile, stats, equipment, talismans, spells, appearance tylko przez preset), single-character first, weapon level override przy apply, plików `.yaml` import/export, importu z URL z pełnymi guardami bezpieczeństwa oraz późniejszą fazą multi-character pack. Dokument **nie** redefiniuje baseline'u v1 — dziedziczy go z [55-build-template](55-build-template.md).

---

## 1. Title, status and scope

| Aspekt | Wartość |
|---|---|
| Numer dokumentu | 56 |
| Typ dokumentu | Design doc — częściowo wdrożone rozszerzenie |
| Status | 🔄 Częściowo wdrożone. Phase 0..6 + Phase 6b + Phase 7a + Phase 7a.2 + Phase 9 dostarczone (Phase 5 = Apply z biblioteki + direct imported-YAML Apply wyłącznie dla subsetu profile/stats; Phase 6 = apply-time overrides dla tego samego subsetu profile/stats, frontend-only mutacja canonical JSON przekazana do istniejącego endpointu Phase 5; Phase 6b = apply-time weapon level override dla istniejącej ścieżki Apply v1 `inventory.workspace` przez addytywną runtime-opcję `WeaponLevelOverride` na `app_templates.go::ApplyTemplateOptions`, aplikowana po każdym template-side patchu broni przez `editor.UpdateWeapon` + `editor.ClampUpgrade(req, MaxUpgrade)`; workspace-only, bez zmiany schematu v2, bez writera ekwipunku, kontrolki UI wewnątrz istniejącego dropdownu Templates w `SortOrderTab.tsx`; Phase 7a = pierwsza realna ścieżka Apply v2 dla `inventory.workspace` przerzucona przez aktywną `InventoryEditSession`/`InventoryWorkspaceSnapshot` przez nowy lookup `GetActiveInventoryEditSessionForCharacter` oraz nowy addytywny `ApplyTemplateV2Options.SessionID`; mixed apply profile+stats+inventory.workspace jest atomowy przez dual slot+workspace snapshot rollback; Phase 7a.2 = apply-time weapon level override podłączony do ścieżki Apply v2 inventory z Phase 7a przez addytywne pole `WeaponLevelOverride` na `ApplyTemplateV2Options` (reuse v1 typu i walidatora 1:1); opcja runtime-only przekazywana przez `ApplyTemplateV2Options`, nigdy w canonical JSON; nowy `WeaponLevelOverridePanel` osadzony w istniejącym `ApplyOverridesModal` i renderowany tylko dla szablonów wybierających `inventory.workspace`; fast library Apply nie wysyła override; warningi lecą do `ApplyTemplateV2Result.Preview.Warnings`; Phase 9 = import szablonów z `https://` URL przez `PreviewBuildTemplateImportYAMLFromURL` pod pełną listą guardów §12.3 (SSRF), reużywając tego samego `ImportTemplatePreviewModal` co ścieżka file-import, więc wszystkie trzy akcje downstream ship bez zmian na powierzchni URL); Phase 7b+ pozostają wyłącznie design. Każda kolejna faza wymaga osobnej akceptacji użytkownika zgodnie z workflow z `~/.claude/CLAUDE.md`. |
| Referencja baseline | [55-build-template](55-build-template.md) — wdrożone `version: 1`, wyłącznie JSON, wyłącznie inventory + storage, lokalna biblioteka w `$UserConfigDir/EldenRing-SaveEditor/templates/`. |
| Klucz schematu | Pozostaje `saveforge.build-template` (bez rename). Wdrożone. |
| Wersja schematu | Reader range `1 ≤ version ≤ MaxSchemaVersion (=2)`. Builder v1 nadal emituje `SchemaVersion = 1`; explicit builder v2 (`backend/templates/export_v2.go`) emituje `version: 2`. Wdrożone. |
| Zewnętrzny format publiczny | YAML (`.yaml`). JSON pozostaje dla obecnej lokalnej biblioteki i dla backward-compatible importu. Wdrożone dla payloadów v1 i dla dokumentów v2 produkowanych przez builder v2. |
| Pierwszy widoczny entry point | Niebieski przycisk `Templates` w sidebarze, bezpośrednio nad `Save as...` w `frontend/src/App.tsx` (istniejący footer `<aside>`); otwiera `TemplatesShellModal.tsx`. Wdrożone. |
| Scope postaci (pierwsza iteracja) | Pojedyncza postać. Multi-character pack odroczony do późniejszej fazy (§15). |
| URL import | **Dostarczone (Phase 9, 2026-05-31)**. Backendowy fetch przez `PreviewBuildTemplateImportYAMLFromURL` → `backend/templates/url_import.go::FetchYAMLFromURL` pod pełną listą guardów §12.3 (HTTPS-only, pre-connect IP filter dla literałów IP i każdego DNS-resolved adresu, redirect re-check ×3, body cap 1 MiB, 10 s total / 5 s idle timeouts, strict TLS z system root CAs, identifying User-Agent, Content-Type allowlist, brak auth / cookies / custom headers, strict struct-typed YAML decode reużyty bez zmian, brak auto-refresh; sam fetch nigdy nic nie mutuje). Preview URL reużywa istniejącego `ImportTemplatePreviewModal`, więc Save to Library / Apply to character / Apply with overrides ship bez zmian na powierzchni URL. **Brak zmian schemy biblioteki** — metadata `sourceURL` nie jest dodawana do biblioteki w tej fazie. |
| Zmiana kodu produkcyjnego | Phase 0..6 + Phase 6b + Phase 7a + Phase 7a.2 + Phase 9 dostarczone; późniejsze fazy pozostają wyłącznie design. Szczegóły w §17 i §17a. |

---

## 2. Implemented baseline inherited from spec/55

Poniższe to wdrożony stan Build Template udokumentowany w [55-build-template §2-§20](55-build-template.md). Templates v2 **buduje się na tych faktach**; nie zaprzecza im ani nie przepisuje.

| Obszar | Wdrożone (v1) | Source of truth w kodzie |
|---|---|---|
| Klucz schematu | `saveforge.build-template` | `backend/templates/schema.go::SchemaKey` |
| Wersja schematu | `1` | `backend/templates/schema.go::SchemaVersion` |
| Format | JSON (indentowany) | `backend/templates/`, `encoding/json` |
| Pokrycie sekcji | tylko `inventory.workspace.{inventoryItems,storageItems}` | `backend/templates/schema.go::TemplateSections` |
| Pola per item | `baseItemID`, `name`, `category`, `quantity`, `upgrade`, `infusionName`, `aowItemID (*uint32, omitempty)`, `container`, `position` | `backend/templates/schema.go::TemplateItem` |
| Export | `BuildTemplateFromSnapshot`, `ExportBuildTemplateJSON/ToFile` | `backend/templates/export.go`, `app_templates.go` |
| Preview | `ParseBuildTemplateJSON`, `PreviewBuildTemplateImport`, `PreviewBuildTemplateImportJSON/FromFile` | `backend/templates/import.go`, `app_templates.go` |
| Apply (RAM-only, append-only) | `ApplyBuildTemplateToWorkspaceJSON/FromFile`, capacity preflight, `deepCopySnapshot` rollback | `app_templates.go` |
| Lokalna biblioteka | `$UserConfigDir/EldenRing-SaveEditor/templates/` z `_index.json` (`LibraryIndexVersion=1`), atomic writes, sanitized filenames | `backend/templates/library.go` |
| Entry point UI dzisiaj | Dropdown `Export Template ▾` w `frontend/src/components/SortOrderTab.tsx` (Inventory → Weapons & Sort Order) + trzy modale w `frontend/src/components/templates/` | jak wyżej |
| Model concurrency | Per-session lock (`backend/editor/session.go`) + slot lock (`slotMu[i]`) + ascending lock order udokumentowany w [55 §10](55-build-template.md) | jak wyżej |
| Integrity gate | Pre-flight w `AddItemsToCharacter` odrzuca przy duplikatach acquisition index; repair przez `RepairDuplicateInventoryIndices` | `app_save_integrity.go`, `backend/core/inventory_index_repair.go` |
| Obsługa AoW | Wyłącznie stabilny `aowItemID`; nigdy raw handles; fail-closed dla unknown compat | [55 §6.4, §9.5, §13](55-build-template.md), [54-ash-of-war](54-ash-of-war.md) |

Templates v2 **nie wolno** zmieniać żadnego z powyższych dla dokumentów deklarujących `version: 1`. Czytniki v1 muszą zachowywać się dokładnie jak dzisiaj.

---

## 3. Product goals

1. **Pojedynczy, centralny entry point UI.** Wprowadzić niebieski przycisk `Templates` w sidebarze, bezpośrednio nad `Save as...`. Wyeksponować Library / Create / Import / Apply Preview z jednego miejsca, odsprzężone od `Weapons & Sort Order`.
2. **Publiczny, czytelny dla człowieka format YAML do udostępniania.** Dodać YAML jako format wymiany dla eksportu do pliku, importu z pliku i importu z URL. Format musi być ręcznie edytowalny przez zaawansowanych użytkowników.
3. **Planowane pokrycie większej ilości danych postaci.** Rozszerzyć z inventory + storage na kolejne safe-semantic sekcje pojedynczej postaci: profile (name/level/runes), stats, equipment selection (slot → item id), talisman item ids i slot count, spell item ids, opcjonalnie appearance wyłącznie przez stabilną nazwę preset.
4. **Granular selection.** Per template i per apply użytkownik wybiera dokładnie które sekcje (a w sekcjach które sub-grupy, np. per stat) uczestniczą. Wybór jest zakodowany w samym YAML-u, więc odbiorca widzi dokładnie to, co nadawca chciał udostępnić.
5. **Lokalna biblioteka pozostaje JSON-kompatybilna.** Obecna biblioteka na dysku i `_index.json` działają bez zmian dla payloadów v1. Payloady v2 zapisane lokalnie pozostają JSON-em w bibliotece; YAML jest formatem wyłącznie zewnętrznym/udostępnieniowym.
6. **Import z pliku i z URL.** Import szablonu `.yaml` z lokalnego pliku lub bezpośrednio z URL-a `https://`. Oba flow przechodzą przez preview przed apply; sam fetch z URL nigdy nie modyfikuje save ani biblioteki bez explicit confirm.
7. **Weapon level override przy apply.** Przy apply szablonu użytkownik może nadpisać upgrade level importowanych broni, osobno dla standard (+0..+25) i somber/special (+0..+10). Domyślnie `Keep`.
8. **Safety-first apply.** Planowana abstrakcja `TemplateApplyPlan` jest odpowiedzialna za połączenie sekcji v2 w jedną, atomowo zaaplikowaną lub w pełni cofniętą operację, respektującą istniejący integrity gate, edit session locking i post-write validation.
9. **Bez regresji v1.** Istniejące lokalne szablony, istniejący dropdown w `SortOrderTab` i wszystkie istniejące testy muszą nadal działać bez modyfikacji. Dropdown jest zachowany jako shortcut w pierwszej fazie; jego usunięcie albo zmiana pozycji to osobna, późniejsza decyzja.
10. **Multi-character pack jako późniejsza iteracja.** Schema musi zostawić miejsce na `all_characters` packs (§15), ale pierwsza iteracja dostarcza wyłącznie single-character.

---

## 4. Non-goals and explicitly excluded unsafe fields

### 4.1. Non-goals dla pierwszej iteracji Templates v2

- Import ani eksport **raw event flag IDs** jakiegokolwiek rodzaju.
- Edycja **progression / unlocks** (graces, bell bearings, cookbooks, bosses, NG+) przez raw flag manipulation. Takie efekty, jeśli w ogóle, pozostają mediowane przez istniejące nazwane moduły (np. `app_pvp.go:ApplyPvPPreparation`) oraz przez niejawne POST-FLAGS hooks `AddItemsToCharacter` udokumentowane w [50-item-companion-flags](50-item-companion-flags.md) (companion-flag SET w `app.go:569-578`, pickup-flag SET w `app.go:743+`).
- Eksport ani import **raw FaceData blobs** (`backend/core` sekcja FaceData). Appearance dozwolone wyłącznie przez stabilną nazwę preset z `data.Presets`, i wyłącznie w późniejszej fazie.
- Zastąpienie wdrożonego podsystemu v1. Templates v2 jest wyłącznie addytywne.
- Auto-migracja istniejących on-disk v1 entries do v2.
- Usunięcie istniejącego dropdownu `Export Template ▾` w `SortOrderTab` w pierwszej fazie.
- Jakikolwiek nowy HTTP fetch surface poza ściśle ograniczonym URL template importem (zob. §12).
- Settings export, app-config sync, lub jakakolwiek persistencja współdzielona z katalogiem templates.

### 4.2. Pola zakazane w publicznym YAML i w jakimkolwiek planowanym schemacie v2

Reguła portable-payload z v1 ([55 §7](55-build-template.md)) jest zachowana i rozszerzona. Następujące pola **nigdy** nie mogą pojawić się jako pola w publicznym YAML, bez względu na sekcję:

- `GaItemHandle` (dowolny prefix: `0x80…` weapon, `0xC0…` AoW, itd.).
- `AoWGaItemHandle` raw value (sentinele `0x00000000`, `0xFFFFFFFF`, lub jakikolwiek alokowany `0xC0…`).
- Absolutne wartości `AcquisitionIndex`.
- `NextAcquisitionSortId`, `NextEquipIndex`.
- Wpisy `GaMap`.
- Raw event flag IDs.
- Binary offsets w slot data.
- Encryption IV, MD5 / hash bytes, material klucza AES.
- `SessionID`, `BaseRevision` (SHA256 prefix), `BaselineEditableHandles`.
- Steam ID, identyfikatory PSN.
- Raw save blobs (binarny FaceData, regulation slices, world geometry, itp.).
- Per-item `originalHandle`, `currentAoWHandle`, `uid`, ani żadne pola `Pending*` wewnętrzne dla workspace.

Co publiczny YAML **może** nieść (wyłącznie wartości semantyczne):

- Item / weapon / AoW / armor / talisman / spell item IDs (`uint32`).
- Weapon `upgrade` integer + `infusionName` string + `aowItemID` `uint32`.
- Relative `position` integer (index w tablicy).
- Pola profile: `name`, `level`, `runes`, `gender`, `voiceType`.
- Stats: `vigor`, `mind`, `endurance`, `strength`, `dexterity`, `intelligence`, `faith`, `arcane`.
- Talisman slot count (mała liczba całkowita, clampowana przy apply przez istniejący Pouch-upgrade machinery).
- Equipment slot assignment (nazwa slotu → item ID).
- Lista spelli (item IDs).
- Nazwa appearance preset (string z `data.Presets`).
- Metadata: `name`, `description`, `author`, `tags`, `createdAt`, `appVersion`, `sourceCharacterName`, `sourceURL`.

---

## 5. Compatibility strategy

### 5.1. Klucz i wersja schematu

- Klucz schematu pozostaje `saveforge.build-template`. **Bez rename.**
- Dodana zostaje nowa akceptowana wersja `2`. Akceptowany zakres readera staje się `1 ≤ version ≤ 2`. Writery rozszerzonych szablonów produkują `version: 2`.
- Dokumenty v1 nadal parsują i aplikują się dokładnie jak dzisiaj.
- Reader musi wykonać addytywny forward-fill przy czytaniu v1: brakujące sekcje domyślnie "not selected" / "not present" (semantycznie równoważne aktualnemu zachowaniu).
- v2 wprowadza opcjonalne nowe top-level klucze; brakujące klucze oznaczają "nieobecne w tym szablonie". Dokument v2 zawierający wyłącznie `sections.inventory.workspace` jest semantycznie równoważny dokumentowi v1.

### 5.2. JSON vs YAML

- Istniejąca on-disk **lokalna biblioteka pozostaje JSON-wewnętrzna**. Atomic writes, `_index.json`, sanitized filenames, recovery semantics — wszystko zachowane dokładnie wg [55 §19](55-build-template.md).
- Nowa publiczna **reprezentacja YAML** dodana dla eksportu do pliku, importu z pliku i importu z URL. YAML musi być 1:1 mapowaniem tych samych struktur Go, które stoją za formą JSON. Przełączanie formatu nie może gubić ani transformować żadnego pola.
- Import pliku YAML jest dozwolony do **zapisania w bibliotece jako JSON** (przezroczyste transkodowanie), więc biblioteka na dysku pozostaje jednorodna.
- Plik JSON v1 zaimportowany do biblioteki **nigdy** nie jest przepisywany na v2 na dysku. Auto-migracja jest jawnie poza scope (§4.1).

### 5.3. Brak destruktywnych rewrites

- Czytanie szablonu v1 nigdy nie nadpisuje ani nie upgrade'uje pliku na dysku.
- Pisanie szablonu v2 nigdy nie dotyka wpisów v1 w `_index.json` poza dodawaniem nowych wpisów.
- `RebuildIndex` nadal pomija nieparsowalne / niewalidujące się pliki (wg [55 §19.2](55-build-template.md)).

### 5.4. Readery v1 vs dokumenty v2

- Reader wyłącznie v1 (np. starsza wersja appu) napotykający dokument v2 musi go odrzucić przez istniejącą ścieżkę `ValidateBuildTemplate` "unsupported version". Nie ma cichego downgrade.
- Reader v2 musi zawsze akceptować dokumenty v1.

---

## 6. Planned UI

### 6.1. Sidebar entry point

- **Lokalizacja**: `frontend/src/App.tsx`, wewnątrz istniejącego footer bloku `<div className="p-4 border-t border-border bg-muted/5 space-y-3">` (obecny zakres linii ~503–515), wstawiony **bezpośrednio nad** istniejącym przyciskiem `Save As`.
- **Styl**: niebieski przycisk, dopasowany do istniejących niebieskich wzorców w aplikacji (np. `border-blue-500/40 bg-blue-500/10 text-blue-600 hover:bg-blue-500/20` z headerowego przycisku `Review Changes`, lub ciemniejszy wariant równoważny `DatabaseTab.tsx`).
- **Widoczność**: zawsze widoczny gdy aplikacja pokazuje sidebar. Library / Preview / Import / Export pozostają używalne bez aktywnej `InventoryEditSession`. Akcje wymagające aktywnej postaci lub workspace (Create from current save, Apply to workspace) są disabled dopóki taki kontekst nie istnieje.

### 6.2. Templates shell

Przycisk otwiera pojedynczy Templates UI surface (modal lub panel — dokładny kształt to decyzja UI fazy implementacyjnej). Konceptualnie oferuje cztery sekcje / taby:

| Sekcja | Wymaga otwartej postaci? | Wymaga otwartej sesji? |
|---|---|---|
| Library | nie | nie |
| Create | tak | tak |
| Import (file + URL) | nie | tylko gdy wybrane "Apply directly" |
| Apply Preview | tak (postać docelowa) | tak (sesja docelowa) |

### 6.3. Retention of the existing dropdown

- Dropdown `Export Template ▾` w `frontend/src/components/SortOrderTab.tsx` jest **zachowany jako shortcut** w pierwszej fazie implementacyjnej.
- Nadal woła istniejące Wails bindings dokładnie jak dzisiaj.
- Późniejsza, osobno akceptowana decyzja określi czy go usunąć, przekierować do nowego sidebar surface, czy zachować na stałe jako power-user shortcut.

### 6.4. State management

- Templates surface podąża za istniejącym wzorcem `useState`-per-component (bez globalnego store). Zob. istniejące modale jak `cloneModal`, `deleteModal`, `diffModal` w `App.tsx`.
- Jeśli surface potrzebuje `sessionID`, faza implementacyjna decyduje czy (a) podnieść `sessionID` do `App.tsx` state, (b) zbudować lżejszy library-only modal niezależny od sesji, czy (c) trzymać `sessionID` w `SortOrderTab` i przekazywać w dół przez props/context. To otwarta decyzja produktowa (§18).

---

## 7. Planned single-character data sections

Poniższe sekcje mogą pojawić się w szablonie v2, oznaczone indywidualnie jako supported w selection mask (§8). Wszystkie sekcje są opcjonalne; szablon v2 zawierający wyłącznie sekcję workspace jest valid i jest funkcjonalnie identyczny z dokumentem v1.

Kolumna `Apply path` poniżej rozróżnia między (a) **istniejącymi** writerami, które Templates v2 może reużyć, a (b) **nowymi write path** które muszą zostać zaprojektowane i dodane zanim odpowiednia sekcja będzie mogła być zaaplikowana. Klasyfikacja jest oparta o zweryfikowane ścieżki kodu — zob. §13.6 i kolumnę "Istniejący writer?" poniżej.

| Klucz sekcji | Status fazy | Treść | Apply path (planowany) | Klasa ryzyka | Istniejący writer? |
|---|---|---|---|---|---|
| `inventoryWorkspace` (klucz v1 `inventory.workspace` zachowany przez reader) | dziedziczony z v1 | jak dzisiaj | jak dzisiaj (`editor.AddItem` / `editor.UpdateWeapon` → `ApplyWorkspaceSave`) | requires-dependent-writers (v1) | tak (v1) |
| `profile` | planowane | `name`, `level`, `runes` (Souls/SoulMemory), `class`, `clearCount` (cap 7), `scadutreeBlessing`, `shadowRealmBlessing` | reużywa istniejący `vm.ApplyVMToParsedSlot` → `slot.SyncPlayerToData` (ta sama ścieżka co `app.go::SaveCharacter`, linie 297–345 i 823–860) | safe-semantic | tak |
| `stats` | planowane | per-stat scalars: Vigor / Mind / Endurance / Strength / Dexterity / Intelligence / Faith / Arcane | tak samo jak `profile` (mapowane przez `vm.ApplyVMToParsedSlot`, zapisywane przez `slot.SyncPlayerToData`) | safe-semantic | tak |
| `profile.talismanSlots` (additional Pouch slot count `0..3`) | planowane | `uint8`, clampowane do `0..3` (cap gry; total slots = `1 + value`) | reużywa istniejący `vm.ApplyVMToParsedSlot` → `slot.SyncPlayerToData` (pole `Player.TalismanSlots` w `OffTalismanSlots`, zapis w `structures.go:841`) | safe-semantic | tak |
| `appearance.gender` i `appearance.voiceType` | późniejsza faza (Phase 8) | stabilna nazwa preset (preferowane), lub explicit wartości bajtowe `gender` / `voiceType` | **Nie** mapowane przez `vm.ApplyVMToParsedSlot` z VM. Muszą iść przez istniejące helpery `app_appearance.go::ApplyPresetToCharacter` / `SetCharacterGender`, które biorą `slotMu[charIdx]`, pushują undo i same wołają `SyncPlayerToData` | safe-semantic (tylko preset) | tak (helpery) |
| `equipment` (equipped sloty: `weaponRight1/2`, `weaponLeft1/2`, `armorHead/Chest/Arms/Legs`, plus opcjonalnie `equippedGreatRune`) | późniejsza faza (Phase 7b) | nazwa slotu → item ID | **Brak publicznego write API dzisiaj** dla `ChrAsmEquipment` slotów 0..9, 12–15, 17–21 — [06-equipment §App-level write API](06-equipment.md) jest jednoznaczne ("None — equipment is read-only from the UI perspective"). Jedyny istniejący wyjątek to `EquippedGreatRune` (slot 10), już zapisywany przez `SyncPlayerToData` w `structures.go:850–852`. **Wymagany nowy kontrolowany writer** dla pozostałych slotów (encoded item-ID form, hash 7/8 dependency — zob. [06-equipment §hash](06-equipment.md)). | requires-new-writer | **nie** (poza GreatRune) |
| `equippedTalismans` (które talismany zajmują `ChrAsmEquipment` sloty 17–21) | późniejsza faza (Phase 7c) | tablica do 5 item IDs talizmanów w kolejności slotów | **Brak publicznego write API dzisiaj** — equipped talismans żyją w tym samym bloku `ChrAsmEquipment` co zbroja; są read-only razem z resztą equipment. **Wymagany nowy kontrolowany writer** (companion do Phase 7b) i musi respektować limit Pouch z `profile.talismanSlots`. Odrębne od `profile.talismanSlots` (additional slot count, który już ma writer). | requires-new-writer | nie |
| `spells` (equipped sorcery / incantation / gesture loadout — 14 spell slotów) | późniejsza faza (Phase 7d) | spell / sorcery / incantation / gesture item IDs | **Brak publicznego write API dzisiaj.** `EquippedSpells` (14 slotów) jest obecnie referowane wyłącznie przez hash-recompute (`backend/core/hash.go:150–195`, `section_hash.go:24`). **Wymagany nowy kontrolowany writer.** Odrębne od unlocked-spell inventory entries (które są częścią `inventoryWorkspace` i są już wspierane przez v1). | requires-new-writer | nie |
| `weapons` (overlay na `inventoryWorkspace`) | planowane | opcjonalne explicit `upgrade`, `infusionName`, `aowItemID` per inventory / storage weapon już wyliczona w sekcji workspace | reużywa istniejącą mutację workspace `editor.UpdateWeapon`; zob. §14 | safe-semantic (level / infusion), requires-dependent-writers (AoW) | tak (v1) |
| `appearance.preset` | późniejsza faza (Phase 8) | wyłącznie stabilna nazwa `preset` (musi istnieć w `data.Presets` — `backend/db/data/presets.go::Presets`) | reużywa istniejący `app_appearance.go::ApplyPresetToCharacter` (zapisuje FaceData blob + gender + voiceType przez preset, pod `slotMu[charIdx]`). Raw FaceData blob **nigdy** nie jest w YAML. | safe-semantic (tylko preset) | tak |

Co jest celowo **nie** w v2 pierwszej iteracji:

- Brak pola raw FaceData blob. Appearance, jeśli aplikowane, idzie wyłącznie przez ścieżkę preset.
- Brak raw event flag manipulation. Gdzie efekty progression-like są potrzebne, pozostają mediowane przez niejawne POST-FLAGS hooks `AddItemsToCharacter` ([50-item-companion-flags](50-item-companion-flags.md); zob. `app.go:569-578` dla companion-flag SET i `app.go:743+` dla pickup-flag SET) i przez nazwane moduły jak `ApplyPvPPreparation`.
- Brak stanu PvP preparation wewnątrz szablonu bezpośrednio. Jeśli/gdy potrzebne, późniejsza faza może dodać `modules` niosące listę nazwanych module presets (np. `pvp.colosseums`) bez nigdy kodowania raw flag IDs.
- Brak raw `Player.Gender` ani `Player.VoiceType` bajtów poza ścieżką appearance preset. Mimo że oba pola są bajtowo zapisywalne przez `SyncPlayerToData`, `vm.ApplyVMToParsedSlot` **nie** mapuje ich z VM — są dziś sterowane wyłącznie przez helpery `app_appearance.go`, i Templates v2 musi to utrzymać.
- Brak raw zapisu dodatkowych Pouch event flags. Additional Talisman Pouch slot count (`profile.talismanSlots`, `0..3`) to zwykłe pole `u8` w `PlayerGameData` zapisywane przez istniejącą ścieżkę profile/stats; nie wymaga dotykania żadnego Pouch event flag.

---

## 8. Granular selection model

### 8.1. Obiekt `selection`

Szablon v2 niesie obiekt `selection`, którego kształt odbija `sections`. Każdy leaf jest albo booleanem (`true` = include w apply, `false` = ignore nawet jeśli dane są obecne), albo, dla per-element groups, listą explicit kluczy.

Właściwości:

- **Autorytatywne dla apply.** Applier działa wyłącznie na sekcjach (i sub-keys) gdzie `selection` jest `true`. Sekcje obecne w YAML ale nieselected są traktowane jako metadata do review only.
- **Autorytatywne dla export.** Gdy nadawca eksportuje szablon, zapisywane są wyłącznie selected pola. Nieselected pola są pomijane (nie zerowane).
- **Forward-compatible.** Nieznane klucze w `selection` są ignorowane przez reader; brakujące klucze są traktowane jako `false`.
- **Per-stat granularność.** `selection.stats` może być `true` (apply all 8), `false` (apply none), lub obiektem per-stat booleans (`{ vigor: true, mind: false, ... }`). Ten sam per-element wzorzec dozwolony dla talismans (per-item-id), equipment (per-slot), i spells (per-item-id).

### 8.2. Implikacja UI

- Create / Export modal: użytkownik wybiera które sekcje i (gdzie applicable) które pola dołączyć. Wybory są zapisane do `selection`, więc odbiorca widzi ten sam kształt przy imporcie.
- Apply Preview modal: użytkownik może dalej zawęzić selection w czasie apply (np. "import everything except Endurance"). Zawężony selection jest lokalny — YAML na dysku nie jest przepisywany.
- Defaults: `selection.inventory.workspace = true` gdy sekcja jest obecna (by odbić obecne zachowanie v1). Wszystkie inne sekcje domyślnie `true` gdy obecne i `false` gdy nieobecne.

### 8.3. Reguły walidacji (planowane)

- Klucze `selection` muszą pasować do znanej sekcji / subkey. Nieznane produkują warning, nie error.
- Sekcja oznaczona `true` ale nieobecna w `sections` produkuje error (`selection_missing_section`).
- Sekcja obecna w `sections` ale nieselected jest dozwolona i cicho pomijana przez applier.

---

## 9. Public YAML direction (illustrative high-level example only)

Poniższe to non-normative ilustracja zakotwiczająca dyskusję. To **nie** jest finalny schemat. Faza implementacyjna produkuje kanoniczny schemat, generowany z tagów Go struct, więc JSON i YAML współdzielą pojedyncze źródło prawdy.

```yaml
schema: saveforge.build-template
version: 2
appVersion: 1.1.0-alpha
createdAt: 2026-06-01T12:00:00Z

metadata:
  name: RL150 INT Cold-infused
  description: PvP build for invasions
  author: someone
  tags: [pvp, int, cold]
  sourceCharacterName: Tarnished      # informacyjny cytat, read-only
  sourceURL: https://example.org/builds/rl150-int.yaml  # tylko jeśli zaimportowane z URL

selection:
  profile: true
  stats:
    vigor: true
    mind: true
    endurance: true
    strength: false
    dexterity: false
    intelligence: true
    faith: false
    arcane: false
  equipment: true
  equippedTalismans: true
  spells: true
  inventoryWorkspace: true

sections:
  profile:
    name: Tarnished
    level: 150
    runes: 0
    talismanSlots: 2   # additional Pouch slot count (0..3); total = 1 + value

  stats:
    vigor: 60
    mind: 25
    endurance: 25
    strength: 12
    dexterity: 18
    intelligence: 80
    faith: 9
    arcane: 7

  # equipment, equippedTalismans, spells i appearance.preset
  # to PÓŹNIEJSZE FAZY — wymagają nowych write paths (zob. §7 i §13.6).
  # Pokazane tutaj dla kształtu, nie dla pierwszej fazy apply.
  equipment:
    weaponRight1: 4030000   # base item ID, bez encoded upgrade/infusion
    weaponRight2: null
    weaponLeft1:  2000000
    weaponLeft2:  null
    armorHead:    10000000
    armorChest:   10010000
    armorArms:    10020000
    armorLegs:    10030000

  equippedTalismans:
    items: [80000000, 80010000, 80020000, 80030000]  # do 5; respektuje profile.talismanSlots

  spells:
    sorceries: [40000000]
    incantations: []
    gestures: [50000000]

  appearance:
    preset: geralt   # musi rozwiązać się w data.Presets

  inventoryWorkspace:
    inventoryItems:
      - baseItemID: 4030000
        quantity: 1
        upgrade: 10
        infusionName: Cold
        aowItemID: 2168029136
        container: inventory
        position: 0
    storageItems: []
```

Uwagi do przykładu:

- Brak `GaItemHandle`, brak `AoWGaItemHandle`, brak `acquisitionIndex`, brak offsetów — zob. §4.2.
- Klucz `inventoryWorkspace` to preferowana pisownia v2. Reader musi nadal akceptować klucz v1 `inventory.workspace` dla pełnej backward compatibility (dokument v1 ponownie odczytany jako v2 pozostaje semantycznie niezmieniony). Dokładna polityka aliasu — otwarta decyzja §18.
- `equipment` referuje itemy wyłącznie przez base ID; ścieżka apply egzekwuje że item jest obecny (lub, przez późniejszy opt-in, może być dodany) w inventory zanim slot assignment jest commitowany (zob. §13.7).
- `equippedTalismans` to **equipped** loadout (które talismany zajmują sloty 17–21 `ChrAsmEquipment`). Jest odrębne od `profile.talismanSlots`, które jest **liczbą** dodatkowych slotów Pouch (0..3). Tych dwóch pól nigdy nie wolno mylić; zob. §7.
- `selection.stats` pokazuje mixed-mode (forma obiektu) granular selection. Boolean shortcut też jest legalny.
- Kanoniczny, wyczerpujący schemat z typami pól i constraints to deliverable fazy implementacyjnej dostarczającej YAML; nie jest produkowany przez ten dokument.

---

## 10. Local library and external export strategy

### 10.1. Lokalna biblioteka (istniejąca — bez zmian)

- Ścieżka: `$UserConfigDir/EldenRing-SaveEditor/templates/`.
- Per-template plik: `<sanitized-name>-<id-tail>.json`, mode 0644.
- Indeks: `_index.json` (`LibraryIndexVersion = 1`).
- Atomic writes (`atomicWriteFile`), recovery semantics, lazy init — bez zmian z [55 §19](55-build-template.md).
- Szablon v2 przechowywany w bibliotece jest przechowywany jako JSON. Ta sama struktura Go stoi za obiema formami; serializer jest jedyną różnicą.

### 10.2. Planowane dodatki do metadata biblioteki

`LibraryTemplateEntry` może być rozszerzony (wyłącznie addytywnie) o:

- `scope`: `"single"` (default) / `"pack"` (multi-character; zarezerwowane dla §15).
- Podsumowanie `selection`: zwięzła reprezentacja które sekcje są w pliku (np. countery lub mała maska), więc lista biblioteki może pokazać "Profile + Stats + Inventory" bez re-parsowania pliku.
- `sourceURL`: tylko gdy wpis pochodzi z URL import.
- `importedFrom`: free-text origin (np. ścieżka pliku, host URL).

Wszystkie dodatki są opcjonalne. Starsze wpisy biblioteki bez tych pól pozostają valid.

### 10.3. Eksport zewnętrzny

- Nowa Wails-bound metoda App (planowana nazwa: `ExportLibraryTemplateAsYAMLToFile(id)`) otwiera `SaveFileDialog` z `.yaml` jako głównym filtrem i `.json` jako secondary. Użytkownik wybiera pożądany format.
- Istniejąca `ExportLibraryBuildTemplateToFile(id)` (JSON) pozostaje dostępna dla backward compatibility.
- Eksport z aktywnego workspace zyskuje równoległą metodę `ExportBuildTemplateAsYAMLToFile(sessionID, opts)`.
- Anulowanie dialogu zwraca istniejący sentinel (pusty `Path`, brak error) — ta sama konwencja co v1.

### 10.4. Loss-of-data prevention

- Istniejące operacje dotykające biblioteki na dysku (`SaveTemplate`, `DeleteTemplate`, `RenameTemplate`, `RebuildIndex`) pozostają atomowe.
- Rozważanie future-only (nie w scope pierwszej fazy): periodyczny snapshot `_index.json` do `_index.bak.json` przed `RebuildIndex`, by umożliwić user-driven recovery w edge case'ach.

---

## 11. File import flow

### 11.1. Trigger

- Z sidebar Templates surface, `Import → From file…`.
- Opcjonalnie: dostępne także z aktualnego dropdownu w `SortOrderTab` (`Import Template Preview…`) w pierwszej fazie, dla shortcut continuity.

### 11.2. Flow

1. Użytkownik wybiera plik `.yaml` lub `.json` przez `OpenFileDialog`. Anulowanie zwraca istniejący sentinel (brak error, brak toasta).
2. Backend czyta plik z twardym size cap (planowane: 1 MB; subject to confirmation w fazie implementacyjnej).
3. Format detection: rozszerzenie najpierw, content-type heurystyka (magic bytes) druga.
4. Parse do tych samych struktur Go stojących za JSON. YAML musi być parsowany w strict, struct-typed mode (no `interface{}` decode, no `!!include` / aliases / anchors expanding cross-document — zob. §12.6).
5. `ValidateBuildTemplate` (rozszerzony by akceptować `version: 2` i nowe sekcje).
6. `PreviewBuildTemplateImport` (rozszerzony by walidować treść per-section).
7. Raport preview pokazywany użytkownikowi w non-destructive modal. Modal oferuje dwie next steps:
   - **Save to library** — transkoduje do JSON i pisze przez istniejącą library `SaveTemplate` path. Nie dotyka otwartego save ani workspace.
   - **Apply to workspace** — wymaga aktywnej sesji; inaczej przycisk disabled. Idzie przez apply architecture w §13.
   - **Cancel** — odrzuca sparsowany payload.

### 11.3. Errors and warnings

- Parse failure wychodzi jako `ImportPreviewReport` z pojedynczym `structure_invalid` errorem, dokładnie jak dzisiaj dla malformed JSON ([55 §9.2](55-build-template.md)).
- Schema/version mismatch wychodzi jako `schema_invalid`.
- Per-section walidacja produkuje dedykowane issue codes (planowane, addytywne). Ich dokładne nazewnictwo to deliverable fazy implementacyjnej; podążają za istniejącą konwencją `IssueCode*`.

### 11.4. Atomowość

- Czytanie pliku nigdy nie modyfikuje save.
- Zapis do biblioteki używa istniejącej atomic write path.
- Apply to workspace podąża za architekturą w §13.

---

## 12. URL import flow and security constraints

URL import jest **dostarczony (Phase 9, 2026-05-31)**, zwalidowany manualnie end-to-end na `https://` endpoincie serwującym v2 YAML szablon. Flow i guardy poniżej opisują zaimplementowane zachowanie; przyszłe rozszerzenia (auth, domain allowlist, auto-refresh, direct apply bez preview, opcjonalna metadata `sourceURL` w bibliotece) **nie** są częścią Phase 9 i wymagają osobnej akceptacji zanim jakakolwiek praca się rozpocznie.

### 12.1. High-level flow

1. Użytkownik wybiera `Import → From URL…` z sidebar Templates surface.
2. Użytkownik wkleja URL `https://`.
3. Backend Go wykonuje fetch pod ścisłymi guardami (§12.3).
4. Body odpowiedzi jest parsowane jako YAML (lub JSON, wg content-type / heurystyki).
5. Schema validation działa identycznie jak file import (§11.5).
6. Pokazany jest preview wraz z **source URL** wyraźnie wyświetlonym.
7. Użytkownik może albo zapisać do biblioteki, albo przejść do Apply Preview. Anulowanie odrzuca payload. **Sam fetch nigdy nie modyfikuje save ani biblioteki.**

### 12.2. Gdzie żyje fetch

- Fetching musi być zaimplementowany w **backend (Go)**. Frontend nigdy nie wykonuje request HTTP.
- Rationale: backend ma pełną kontrolę nad TLS, redirect policy, IP filtering, body size, content-type validation. Frontendowy `fetch` w WKWebView dziedziczyłby CSP/CORS niespodzianki i komplikował auditability.

### 12.3. Wymagane guardy dla pierwszej implementacji URL-import

- **Scheme**: wyłącznie `https://`. Odrzucać każdy inny scheme (`http`, `file`, `ftp`, `data`, `javascript`, `about`, `blob`, `chrome`, `chrome-extension`, itp.) w parse time.
- **DNS + IP destination filter (defense in depth)**: resolve host, odrzucać wszystkie poniższe zakresy przed connect i re-verify po każdym redirect:
  - Loopback: `127.0.0.0/8`, `::1`.
  - Link-local: `169.254.0.0/16`, `fe80::/10`.
  - RFC1918 private: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`.
  - ULA: `fc00::/7`.
  - Multicast / broadcast / wildcard / quad-zero.
  - Cloud metadata endpoints (np. `169.254.169.254`, `fd00:ec2::254`).
- **Custom redirect policy**: max 3 redirecty; każdy musi przejść IP filter ponownie (TOCTOU defense).
- **Body size cap**: hard `io.LimitReader` — **1 MiB (`1 << 20`)**, zdecydowane przy implementacji Phase 9 i eksportowane jako `templates.URLImportMaxBodyBytes`.
- **Timeouts**: total request timeout **10 s** (`URLImportTotalTimeout`); idle / TLS-handshake / response-header / dial timeouts **5 s** każdy (`URLImportIdleTimeout`). Zdecydowane przy implementacji Phase 9.
- **TLS**: wyłącznie system root CAs, `MinVersion: tls.VersionTLS12`, **bez** `InsecureSkipVerify`, bez custom CA injection z URL.
- **User-Agent**: stabilny identyfikujący string — `"EldenRing-SaveForge Templates-v2-URL-import"` (`URLImportUserAgent`). Zdecydowane przy implementacji Phase 9.
- **Content-Type acceptance list**: `application/json`, `application/yaml`, `application/x-yaml`, `text/yaml`, `text/plain`. Odrzucać `text/html`, `application/octet-stream`, itp.
- **Brak interpretacji body jako code lub commands.** YAML jest parsowany ściśle do typed Go structs.
- **Brak YAML includes / aliases poza local anchors resolving do scalar values.** Brak cross-document references, brak `!!include`, brak custom tagów, brak executable types. Faza implementacyjna wybiera bibliotekę YAML (prawdopodobnie `gopkg.in/yaml.v3` decode do typed structs) z tymi constraints egzekwowanymi.
- **Brak autoryzacji (basic / bearer / cookies) w pierwszej fazie.** Authenticated downloads poza scope.
- **Brak follow-up auto-refresh z URL.** URL jest fetchowany wyłącznie na explicit user action.

### 12.4. State invariants

- **Sam fetch nigdy nic nie mutuje.** Żaden plik nie jest pisany, żaden wpis biblioteki nie jest tworzony, żaden workspace nie jest dotykany dopóki użytkownik nie kliknie explicit confirm w Apply Preview lub w kroku save do biblioteki.
- Wszystkie reguły apply-side (§13) stosują się do URL-imported templates bez wyjątku.

### 12.5. Errors and warnings

- DNS failure / connection refused / TLS error / timeout / body size exceeded / forbidden IP destination / disallowed scheme / bad content-type / parse failure / schema mismatch — każdy produkuje precyzyjny user-visible error oznaczony kategorią. Żaden z tych warunków nie pisze do biblioteki ani do save.

### 12.6. Czym URL import **nie** jest

- Nie jest templating engine. Pobrany dokument to dane, nie instrukcje.
- Nie jest zamiennikiem biblioteki. URL import zawsze ląduje najpierw w preview.
- Nie jest sposobem na bypass jakiejkolwiek reguły walidacji v1 / v2.

---

## 13. Preview / apply architecture

### 13.1. Planowany `TemplateApplyPlan`

Implementacja sekcji v2 bez koordynującej warstwy albo duplikowałaby apply logic, albo ryzykowała partial state. Planowana abstrakcja `TemplateApplyPlan` jest wprowadzona jako pojedynczy koordynator:

1. **Plan phase** (brak mutacji): bierze sparsowany szablon v2 + aktualny otwarty save + opcje apply. Produkuje explicit plan: listę operacji per-section, ich wymagane preconditions, i per-section listę issue/warning. Woła istniejące walidatory (`ValidateBuildTemplate`, `PreviewBuildTemplateImport`) i dodaje nowe walidatory per-section.
2. **Confirm phase** (brak mutacji): plan jest renderowany użytkownikowi jako apply preview. Użytkownik może ponownie zawęzić selection (§8.2). Plan jest regenerowany.
3. **Apply phase** (mutacja, atomowa per slot): plan jest wykonywany pod istniejącym modelem lock per-slot. Każda sekcja używa swojego istniejącego writera; plan tylko orkiestruje kolejność i rollback.
4. **Post-apply validation**: re-runs integrity gate scan (`GetSaveInventoryIntegrityReport` / `core.ScanDuplicateInventoryIndices`) i jakąkolwiek planowaną per-section sanity check. Regresja triggeruje rollback.

### 13.2. Rollback / atomowość

- **Jeden snapshot per affected slot** wzięty przed jakąkolwiek mutacją sekcji. Na każdy error (preview, capacity, mutation, post-apply validation) slot jest restore'owany ze snapshotu.
- Dla workspace mutations istniejąca `deepCopySnapshot` rollback path jest reużyta; dla direct slot edits (profile, stats, equipment, talismans, spells), per-slot byte-level snapshot jest użyty, modelowany na `core.SnapshotSlot` / `core.RestoreSlot` już używanych przez integrity gate.
- Plan nigdy nie zaczyna mutacji dopóki wszystkie per-section walidatory nie przeszły.
- Plan ma prawo przerwać mid-way wyłącznie wewnątrz per-slot critical section; rollback restore'uje slot do pre-plan state.

### 13.3. Interakcja z integrity gate

- Pre-flight guard w `AddItemsToCharacter` (odrzucenie na duplikatach acquisition indices) pozostaje w mocy. Plan nie może go bypassować.
- Plan re-checks integrity przed mutacją i znowu po mutacji. Post-apply integrity regression jest traktowana jako hard failure i triggeruje rollback.

### 13.4. Interakcja z edit session locking

- Plan acquireuje te same locki co underlying writers, w tej samej ascending order (`saveMu.RLock` → `lifecycleMu[charIdx]` → `editSessionsMu` → `sess.mu` → `slotMu[charIdx]`) — zob. [55 §10](55-build-template.md) i istniejące notatki audytu.
- Plan nie wymyśla nowego locka. Składa istniejące w jednej critical section.
- Save / SaveAs jest zabronione gdy plan jest mid-mutation; to jest naturalnie egzekwowane przez `slotMu[charIdx]`.

### 13.5. Sekcja Inventory / workspace

- Apply `sections.inventory.workspace` nadal jest RAM-only wewnątrz aktywnej edit session, dokładnie jak dzisiaj. Użytkownik nadal musi kliknąć `Save changes` by persistować.
- Plan nigdy nie woła `SaveInventoryWorkspaceChanges` automatycznie.

### 13.6. Per-section write paths (zweryfikowane w kodzie)

Warstwa apply kieruje każdą sekcję do innego writera. Plan musi te ścieżki jawnie składać; **nie istnieje** pojedyncza per-section rodzina `slot.Sync…ToData` poza `SyncPlayerToData`. Klasyfikacja poniżej jest oparta o kod:

- **`profile` (name / level / runes / class / clearCount / Scadutree / Shadow Realm) i `stats` i `profile.talismanSlots` (additional Pouch slot count 0..3) i `weapons` overlay na `inventoryWorkspace`** → reużywają istniejącą ścieżkę używaną przez `app.go::SaveCharacter`: `vm.ApplyVMToParsedSlot(&charVM, &slot)` (zob. `backend/vm/character_vm.go:297-345`) a następnie `slot.SyncPlayerToData()` (zob. `backend/core/structures.go:823-860`). Wszystkie zapisy odbywają się pod `slotMu[charIdx]`, z per-slot snapshotem wziętym przed wywołaniem i rollbackiem na każde błędne wykonanie.
- **`inventoryWorkspace`** → reużywa istniejącą ścieżkę apply v1 (RAM-only `editor.AddItem` / `editor.UpdateWeapon`, persistowane przez kliknięcie `Save changes` przez użytkownika, które woła `ApplyWorkspaceSave`). Plan nigdy nie woła `SaveInventoryWorkspaceChanges` automatycznie.
- **`appearance.preset`, `appearance.gender`, `appearance.voiceType`** → reużywają istniejące helpery `app_appearance.go::ApplyPresetToCharacter` i `SetCharacterGender`. **`vm.ApplyVMToParsedSlot` nie mapuje Gender / VoiceType z VM**, mimo że `SyncPlayerToData` je zapisuje — więc plan apply musi routować je przez helpery appearance, nie przez ścieżkę profile/stats. To znaczy że sekcja appearance zależy od osobnego writera, który już istnieje i jest niezależnie zarządzany przez undo.
- **`equipment`, `equippedTalismans`, `spells`** → **brak publicznego write API dzisiaj.** Editor jest dziś read-only dla `ChrAsmEquipment` slotów 0..9, 12–15, 17–21 ([06-equipment](06-equipment.md): "App-level write API for equipment slots | ❌ None") i dla `EquippedSpells` (14 slotów w `backend/core/hash.go:150` i `section_hash.go:24`, referowane wyłącznie przez hash recompute). Jedyny istniejący wyjątek to `EquippedGreatRune` (slot 10), już zapisywany przez `SyncPlayerToData` w `structures.go:850-852`. Templates v2 **wymaga nowych kontrolowanych writerów** w `backend/core/` dla pozostałych slotów equipment, dla equipped talismans i dla spell loadout. Każdy nowy writer musi (a) honorować encoded item-ID form per typ slotu ([06-equipment §encoded item-ID form](06-equipment.md)), (b) respektować hash 7/8 dependency, (c) brać `slotMu[charIdx]`, (d) być pokryty per-platform round-trip testami (PC + PS4). Te nowe writery są wprowadzane w odpowiednich Phases 7a / 7b / 7c (zob. §17).
- Per-section snapshot dla rollbacku: plan bierze jeden snapshot `slot.Data` per affected slot przed jakąkolwiek mutacją w critical section slotu, używając wzorca `core.SnapshotSlot` / `core.RestoreSlot` już używanego przez integrity gate.

### 13.7. Equipment slot referential integrity

- Jeśli `sections.equipment` referuje item nie obecny w inventory postaci docelowej, default behavior planu to **warning + zostaw slot bez zmian** (brak silent auto-add). Opcjonalny opt-in `addMissingEquippedItems: true` może być rozważony w późniejszej fazie, ale nie może być default.

### 13.8. Appearance via preset (późniejsza faza)

- Gdy wprowadzony, apply appearance idzie wyłącznie przez `app_appearance.go::ApplyPresetToCharacter`. Raw FaceData nigdy nie jest pisany z szablonu.

### 13.9. Post-apply user step

- Dla inventory.workspace użytkownik nadal klika `Save changes` by persistować do `slot.Data`.
- Dla direct slot edits (profile, stats, equipment, talismans, spells) zmiany są już w `slot.Data` po apply, ale persistowane na dysk dopiero na następnym `WriteSave`/`SaveAs` — matching istniejące zachowanie `SaveCharacter`.

---

## 14. Weapon level override semantics

### 14.1. Cel

Pozwolić użytkownikowi w czasie apply nadpisać upgrade level broni, które **pochodzą z szablonu**, osobno dla standard i somber/special, bez dotykania:

- Broni już na postaci docelowej, które nie są w szablonie.
- `infusionName` niesionego przez bronie szablonu.
- `aowItemID` niesionego przez bronie szablonu.

### 14.2. Kontrolki UI (planowane)

Trzy niezależne kontrolki na ekranie Apply Preview, każda defaultująca do `Keep`:

| Kontrolka | Default | Zakres |
|---|---|---|
| `Standard weapons (+0..+25)` | `Keep` | `Keep` lub `Set to +N` z `0 ≤ N ≤ 25` |
| `Somber/special weapons (+0..+10)` | `Keep` | `Keep` lub `Set to +N` z `0 ≤ N ≤ 10` |
| Non-upgradeable (MaxUpgrade=0) | locked at +0 | wyłącznie informacyjna |

### 14.3. Źródło klasyfikacji

- Standard vs somber jest czytany z `backend/db/data/types.go::WeaponStatsV1.IsSomber` i `MaxUpgrade`, które są populowane z `regulation.bin` (`EquipParamWeapon`).
- Bronie non-upgradeable mają `MaxUpgrade == 0` i nigdy nie są affected przez override.

### 14.4. Apply path

- Override jest stosowany **w warstwie planu** zanim każda broń jest przekazana do `editor.AddItem` / `editor.UpdateWeapon`. Encoded item ID jest rekomputowany przez istniejący `editor.encodeWeaponItemID(baseID, level, infusionName)`.
- Per-weapon `MaxUpgrade` z DB jest hard clamp. Żądanie `Set to +N` z `N > MaxUpgrade` skutkuje `N := MaxUpgrade` i per-item warningiem w raporcie (`upgrade_clamped_by_override`, planowany kod).
- `infusionName` i `aowItemID` z szablonu są passowane bez zmian.
- Override stosuje się do obu `inventoryItems` i `storageItems` jeśli obie sekcje są częścią szablonu.
- Lokalizacja helpera: ✅ Zrobione — czysty helper clampujący żyje w `backend/editor/weapon.go` jako eksportowane `editor.ClampUpgrade` (przeniesione ze starego `app.go::clampUpgrade`, zachowanie byte-for-byte bez zmiany).
- ✅ Dostarczone 2026-05-31 (Phase 6b, wyłącznie ścieżka Apply v1 `inventory.workspace`) — ścieżka v1 apply w `app_templates.go::applyTemplateItemsToWorkspace` wywołuje teraz `applyWeaponLevelOverride` **po** każdym template-side patchu `editor.UpdateWeapon`. Override switchuje na `editor.EditableItem.MaxUpgrade` (populowane z `db.GetItemDataFuzzy` przez `editor.AddItem`): `25` konsumuje `StandardLevel`, `10` konsumuje `SomberLevel`, `0` jest pomijany z `templates.IssueCodeWeaponUnupgradeable` ("weapon_unupgradeable") na `report.Warnings`, każda inna wartość to silent skip. Over-cap żądania są clampowane przez `editor.ClampUpgrade(req, MaxUpgrade)` i re-aplikowane przez `editor.UpdateWeapon{Upgrade: &clamped}` z `templates.IssueCodeWeaponLevelClamped` ("weapon_level_clamped") na `report.Warnings`. Override nigdy nie dodaje, nie usuwa ani nie relokuje przedmiotów; nigdy nie dotyka `Infusion` ani `AoW`. Mutacja zostaje w pełni wewnątrz aktywnego `InventoryWorkspaceSnapshot`; z ścieżki override nie ma żadnych bajtów do `slot.Data`, a `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu. Apply v2 inventory pozostaje zablokowany przez scope guard `inventory.workspace` w `app_templates_v2_apply.go` — Phase 6b to opcja ścieżki v1 apply, nie nowa sekcja schematu v2.

### 14.5. Dlaczego pojedynczy wspólny level dla wszystkich broni jest zły

- Standard upgradeable bronie cappują na `+25`; somber/special na `+10`; niektóre bronie cappują na `+0`.
- Naiwny pojedynczy globalny `+N` z implicit clamp produkuje cicho niespójne rezultaty (np. user wybiera `+25`, oczekuje uniform reinforcement, dostaje cicho mixed levels).
- Rozdzielenie kontrolki na Standard / Somber wyraża intencję użytkownika precyzyjnie i wyrównuje z modelem danych.

### 14.6. Poza scope tego override

- Override nie zmienia infusion ani AoW.
- Override nie dodaje ani nie usuwa broni.
- Override nie wpływa na bronie na postaci docelowej, które nie są częścią szablonu.

---

## 15. Multi-character pack as a later phase

### 15.1. Pozycja w roadmapie

Multi-character packs (`scope: pack`) są **odroczone** do późniejszej fazy. Pierwsza iteracja Templates v2 dostarcza wyłącznie single-character (`scope: single`).

### 15.2. Planowany kształt (wyłącznie szkic — finalny schemat odroczony)

- YAML niesie `scope: pack` i listę per-character entries, każdy z własnymi `sections` i `selection`.
- Applier wymaga mapowania `sourceCharacter → destinationSlot` wybranego przez użytkownika. Default mappings (np. identity) wymagają explicit user confirmation step.
- Applier obsługuje slot occupancy: zajęte destination slots wymagają explicit replace confirmation; pusty destination slot jest wypełniany wtedy i tylko wtedy gdy user opts in.
- Plan wykonuje każdą postać jako własną per-slot critical section, z per-slot rollback na failure. Failure w jednym slocie nie zostawia cicho zmutowanego innego slotu.

### 15.3. Constraints bez względu na to kiedy ta faza dostarczy

- Wszystkie reguły forbidden-fields (§4.2) i reguły apply (§13) stosują się per-slot identycznie.
- Mapping UI musi być explicit. Implicit / silent slot assignment jest zabronione.
- Integrity gate uruchamia się per-slot, pre- i post-apply.

---

## 16. Risk matrix

Każde ryzyko sklasyfikowane jako jedno z:

- `safe / straightforward` — reużywa istniejące wzorce, minimalna nowa design surface.
- `requires design decision` — potrzebuje otwartej decyzji produktowej zanim implementacja może zacząć się.
- `high-risk / must not implement without separate approval` — potrzebuje explicit user sign-off w fazie implementacyjnej, prawdopodobnie z dodatkowymi guardrails.

| Ryzyko | Klasa | Notatki |
|---|---|---|
| Sidebar entry point + nowy modal shell | safe / straightforward | Reużywa istniejące modal patterns; tylko nowa UI surface. |
| Addytywny schemat (`version: 2`, nowe sekcje, `selection`) | safe / straightforward | Reader range staje się `1 ≤ v ≤ 2`; v1 reader odrzuca v2 przez istniejącą ścieżkę. |
| Trzymanie biblioteki JSON-wewnętrznej | safe / straightforward | Brak migracji on-disk; recovery semantics bez zmian. |
| Dodanie YAML serializera / deserializera | requires design decision | Wybór biblioteki + polityka struct-tag (single source of truth across JSON + YAML). |
| Semantyka `selection` (boolean vs object per group) | requires design decision | Per-stat vs per-item-id granularność do sfinalizowania. |
| Profile / stats apply path (Level / Class / Souls / SoulMemory / 8 stats / CharacterName / ScadutreeBlessing / ShadowRealmBlessing / ClearCount / additional `profile.talismanSlots` 0..3) | safe / straightforward | Reużywa zweryfikowaną istniejącą ścieżkę `vm.ApplyVMToParsedSlot` → `slot.SyncPlayerToData` (`app.go::SaveCharacter`). |
| Gender / VoiceType apply path | requires design decision | `vm.ApplyVMToParsedSlot` **nie** mapuje ich z VM; musi reużywać `app_appearance.go::ApplyPresetToCharacter` / `SetCharacterGender`, nie ścieżkę profile/stats. |
| Equipment slot write path (`ChrAsmEquipment` sloty 0..9, 12–15, 17–21) | requires design decision + new writer | Brak istniejącego publicznego write API ([06-equipment](06-equipment.md) "App-level write API for equipment slots | ❌ None"). Wymagany nowy kontrolowany writer dla Phase 7b; respektuje hash 7/8 dependency. |
| Equipped talismans write path (`ChrAsmEquipment` sloty 17–21) | requires design decision + new writer | Tak samo jak equipment; companion do Phase 7b, planowane jako Phase 7c. Musi respektować limit Pouch z `profile.talismanSlots`. |
| Equipped spell loadout write path (`EquippedSpells` 14 slotów) | requires design decision + new writer | Brak istniejącego publicznego write API; tylko hash recompute referuje to pole dzisiaj. Phase 7d. |
| Equipment referential integrity (szablon referuje item nieobecny w inventory docelowym) | requires design decision | Default = warn + skip; opt-in `addMissingEquippedItems` odroczone (§13.7). Dotyczy Phase 7b/7c. |
| Additional Talisman Pouch slot count (`profile.talismanSlots`, 0..3) | safe / straightforward | Już zapisywane przez `SyncPlayerToData` (`structures.go:841`); zwykłe pole bajtowe, bez raw event-flag write. Odrębne od equipped-talismans writer. |
| Appearance via preset name | requires design decision | Reużywa istniejący `app_appearance.go::ApplyPresetToCharacter`. Ograniczone do wpisów w `data.Presets`; raw FaceData blob jest osobną high-risk decyzją. |
| Raw FaceData | high-risk / must not implement without separate approval | Poza scope pierwszej iteracji v2. |
| Raw event flag manipulation | high-risk / must not implement without separate approval | Wykluczone przez §4. Każdy przyszły opt-in musi przyjść z named-module mediation. |
| Stan PvP preparation w szablonach | requires design decision | Wyłącznie via nazwane moduły (np. `pvp.colosseums`), nigdy raw flagi. |
| Weapon level override (Standard + Somber, osobne) | safe / straightforward | Reużywa istniejące `editor.ClampUpgrade` (✅ relokowane do `backend/editor/weapon.go`, zob. §14.4) + `encodeWeaponItemID` (`backend/editor/weapon.go`). |
| Semantyka inventory / storage rebuild dla dodanych broni | dziedziczone z v1 — safe | Te same fail-closed rules. |
| Acquisition indices / interakcja z `NextAcquisitionSortId` | dziedziczone z v1 — safe | Szablony nigdy nie wystawiają; integrity gate nadal chroni. |
| AoW handles | dziedziczone z v1 — safe (z ostrożnością) | Tylko `aowItemID` w YAML; fail-closed compat check bez zmian. |
| Equipment zależne od itemów nie w inventory | requires design decision | Zob. §13.7. |
| File import (YAML) | safe / straightforward | Istniejący file dialog + nowy parser. |
| URL import — SSRF, redirect TOCTOU, body size, scheme | high-risk / must not implement without separate approval | Wymagane ścisłe guardy (§12). |
| URL import — YAML includes / aliases / executable types | high-risk / must not implement without separate approval | Struct-typed decode only. |
| Migracja schematu v2 → v3 w przyszłości | requires design decision | Poza scope; dokumentować politykę w czasie. |
| Migracja / koegzystencja dropdownu `Export Template ▾` i nowego sidebar entry | requires design decision | Dropdown zachowany w pierwszej fazie; późniejsza decyzja osobna. |
| Multi-character pack | requires design decision | Cała funkcja pack odroczona do późniejszej fazy (§15). |
| Per-platform parity (PC vs PS4) dla template apply | safe — ale do walidacji per faza | Oba round-trip testy muszą pozostać green dla każdej fazy feature dotykającej `backend/core/`. |
| Concurrency z `WriteSave`, edit session lifecycle, clone/delete | dziedziczone z v1 — safe | Plan acquireuje istniejące locki w istniejącej ascending order. |
| Backwards compatibility dla użytkowników udostępniających pliki v1 | safe / straightforward | Readery v2 zawsze akceptują dokumenty v1. |

---

## 17. Recommended phased implementation plan

Każda faza jest mała, niezależnie shippable, i wymaga osobnej akceptacji użytkownika zanim się zacznie. Żadna faza nie commituje kodu bez przejścia przez standardowy workflow w `~/.claude/CLAUDE.md` (Plan → OK → Implementation → Tests → Verification → Git).

### Uzasadnienie kolejności

Pierwsza user-visible wartość to publiczny format wymiany (YAML) dla **już wdrożonego** scope v1 inventory/storage, za stabilnym sidebar entry. Rozszerzenie schematu o sekcje pełnej postaci przychodzi po tym, ponieważ:

- YAML jest flagową funkcją interoperability dla społeczności użytkowników i może zostać dostarczony dla scope v1 bez żadnego ryzyka save-mutation.
- Scope v1 inventory/storage jest już stabilny, przetestowany i ograniczony — to najbezpieczniejsza powierzchnia do ustabilizowania warstwy transportu YAML.
- Nowe sekcje pełnej postaci wymagają nowych write paths w `backend/core/` (zob. §7, §13.6) i nie mogą blokować dostarczenia YAML dla istniejącego scope.
- Każda nowa write path może następnie być dodawana niezależnie, za własną per-phase akceptacją.

### Phase 0 — ten dokument i decyzje produktowe (bieżący)

- **Status**: ✅ Dostarczone.
- **Cel**: wyprodukować ten design document; rozwiązać otwarte decyzje w §18.
- **Pliki**: ten spec + mirror PL; rejestracje README / BOOK_PLAN.
- **Backend / Frontend impact**: brak.
- **Testy**: brak.
- **Manual validation**: review.
- **Ryzyka**: brak.
- **Out of scope**: jakakolwiek zmiana kodu.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 1 — sidebar entry + Templates shell wired do istniejącego backendu v1

- **Status**: ✅ Dostarczone.
- **Cel**: dodać niebieski przycisk `Templates` w `frontend/src/App.tsx`; otworzyć shell wystawiający Library / Import-from-file / Export-from-current-session, wszystko bindowane do **istniejących Wails methods v1**. Brak zmiany schematu, brak zmiany apply.
- **Pliki (planowany scope)**: `frontend/src/App.tsx` (sidebar JSX + modal state), nowy `frontend/src/components/templates/TemplatesShellModal.tsx` (wrapper), testy nowego shella.
- **Backend impact**: brak (reużywa istniejące bindings).
- **Frontend impact**: nowy wrapper, nowy sidebar button, możliwy `sessionID` lift (jedna z opcji w §6.4).
- **Testy**: render testy shella; visibility testy buttona; brak regresji w dropdownie `SortOrderTab`.
- **Manual validation**: otworzyć app, potwierdzić że button się pojawia, potwierdzić że Library / Import / Export nadal działają dokładnie jak v1.
- **Ryzyka**: drobny refactor przekazywania `sessionID`.
- **Out of scope**: jakakolwiek zmiana schematu, jakikolwiek YAML, URL import, granular selection, sekcje pełnej postaci.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 2 — publiczny YAML import / export dla v1 inventory + storage

- **Status**: ✅ Dostarczone (rozbita na 2A backend YAML I/O + 2B frontend dialog wiring).
- **Cel**: wprowadzić reprezentację YAML **istniejącego schematu v1** jako publiczny format udostępniania. Lokalna biblioteka pozostaje JSON-wewnętrzna wg §10.1. Import plików `.yaml` transkoduje do JSON dla storage biblioteki. Brak nowych pól schematu, brak sekcji pełnej postaci.
- **Pliki (planowany scope)**: `backend/templates/yaml.go` (nowy), `go.mod` (nowa zależność YAML, strict struct-typed decode), `app_templates.go` (nowe Wails bindings `ExportBuildTemplateAsYAMLToFile`, `ExportLibraryTemplateAsYAMLToFile`, file import akceptuje `.yaml`), frontend dialog wiring.
- **Backend impact**: nowy serializer/deserializer; biblioteka na dysku pozostaje JSON; istniejące ścieżki JSON bez zmian.
- **Frontend impact**: filtry dialogów obejmują `.yaml`; preview modal akceptuje YAML payload identycznie jak JSON.
- **Testy**: YAML ↔ JSON round-trip dla payloadów v1; odrzucanie nieobsługiwanych YAML tagów / anchors expanding cross-document; odrzucanie body które nie waliduje się przeciw `ValidateBuildTemplate`.
- **Manual validation**: wyeksportować szablon v1 jako YAML, ręcznie edytować plik, re-importować, potwierdzić że preview pasuje, potwierdzić że apply do workspace działa dokładnie jak wcześniej.
- **Ryzyka**: wybór biblioteki YAML — musi egzekwować strict, struct-typed decode (otwarta decyzja §18 #1, rozwiązana przez przyjęcie `gopkg.in/yaml.v3` z struct-typed decode).
- **Out of scope**: pola schematu v2, sekcje pełnej postaci, URL import.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 3 — addytywny schemat v2 + `selection` (tylko export, brak apply)

- **Status**: ✅ Dostarczone (rozbita na 3A structural schema draft, 3B.0 apply guard, 3B pure builder dla profile/stats, 3C metadata dla preview/library, 3C.1/3C.2 App-layer JSON + YAML export i Save to Library z `charIndex`, 3D.0 bindings regen, 3D.1 UI v2 metadata badge, 3D.2a/2b CreateTemplateV2Modal wiring).
- **Cel**: rozszerzyć `backend/templates/schema.go` by deklarować `version: 2`, nowe opcjonalne sekcje (tylko placeholder shape) i `selection`. Update `ValidateBuildTemplate` by akceptował rozszerzony kształt. Reader range staje się `1 ≤ v ≤ 2`. Writery mogą emitować dokumenty v2 zawierające wyłącznie sekcję workspace v1 (semantycznie równoważne v1).
- **Pliki (planowany scope)**: `backend/templates/schema.go`, `backend/templates/schema_test.go`, `backend/templates/export.go` (builder rozszerzony), `backend/templates/import.go` (validator rozszerzony), Mapowanie YAML utrzymywane w spójności (przy założeniu, że Phase 2 została wcześniej dostarczona).
- **Backend impact**: pure type extension; brak zmiany apply-side jeszcze.
- **Frontend impact**: brak.
- **Testy**: ekstensywne schema_test scenariusze w obu kierunkach, włącznie z v1 → v2 reader compat i v2-only-with-workspace round-trip; reader v1 (starsza wersja app) musi odrzucić v2 czysto przez `ValidateBuildTemplate`.
- **Manual validation**: otworzyć istniejący wpis v1 w bibliotece; potwierdzić że nadal się ładuje i aplikuje; wyeksportować jako v2; potwierdzić round-trip.
- **Ryzyka**: silent JSON / YAML field collisions jeśli tag names się nakładają — zabezpieczone testami.
- **Out of scope**: apply nowych sekcji, weapon override, writery equipment / talismans / spells.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 4 — export + preview nowych bezpiecznych sekcji (brak apply jeszcze)

- **Status**: ✅ Dostarczone (CreateTemplateV2Modal steruje per-section + per-field selection; preview pokazuje v2 metadata + wybrane sekcje / pola; apply button nieobecny dla v2; apply v1 workspace bez zmian).
- **Cel**: zaimplementować obiekt `selection` po stronie eksportu (per-section / per-stat checkboxy) i per-section preview walidatory. Apply button pozostaje ukryty dla nowych sekcji w tej fazie; ścieżka apply v1 workspace jest niezmieniona.
- **Pliki (planowany scope)**: `backend/templates/export.go`, `backend/templates/import.go` (addytywne per-section walidatory z nowymi issue codes), `frontend/src/components/templates/ExportTemplateModal.tsx`, `frontend/src/components/templates/ImportTemplatePreviewModal.tsx`.
- **Backend impact**: builder respektuje `selection`; per-section walidatory dodane.
- **Frontend impact**: nowe kontrolki UI na eksporcie; preview renderuje nowe sekcje z warningami/errorami.
- **Testy**: builder emituje wyłącznie selected sekcje / sub-fields; round-trip; per-section preview cases.
- **Manual validation**: wyeksportować szablon v2 "tylko stats"; preview; potwierdzić strukturę i że apply button nie jest oferowany dla nowych sekcji jeszcze.
- **Ryzyka**: niskie.
- **Out of scope**: aplikowanie nowych sekcji.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 5 — apply: profile + stats (minimalny `TemplateApplyPlan`) — ✅ Dostarczone 2026-05-31

- **Cel**: zaimplementować najbezpieczniejszy subset planowanego `TemplateApplyPlan` (§13). Zaaplikować wyłącznie pola które istniejący `vm.ApplyVMToParsedSlot` faktycznie mapuje z VM i które `slot.SyncPlayerToData` zapisuje do `slot.Data`:
  - `profile.name`, `profile.level`, `profile.souls`, `profile.soulMemory` (z istniejącym clampem `runesCostForLevel`), `profile.clearCount` (cap 7), `profile.scadutreeBlessing`, `profile.shadowRealmBlessing`, `profile.talismanSlots` (additional Pouch slot count 0..3, clampowane), `stats.*` (wszystkie 8).
  - `profile.class` jest celowo **pomijane** przez writer Phase 5 i raportowane przez `ApplyTemplateV2Result.Skipped`; `className` **nie jest** aliasem `class`.
  - Wszystko powyższe idzie pod `slotMu[charIdx]` z per-slot `core.SnapshotSlot` wziętym najpierw i `core.RestoreSlot` na każde failure. Flagi `clearCount` i side effects `ProfileSummary` są przeliczane na success.
- **Pliki (dostarczony scope)**: `app_templates_v2_apply.go` (`ApplyBuildTemplateV2ToCharacterJSON`, `ApplyBuildTemplateV2FromLibraryToCharacter`, `ApplyBuildTemplateV2FromFileToCharacter`, `ApplyTemplateV2Options`, `ApplyTemplateV2Result` z `Character` typowanym jako `vm.CharacterViewModel`), regenerowane bindingsy dla tych samych symboli, UI w `frontend/src/components/templates/TemplatesShellModal.tsx` + `TemplateLibraryModal.tsx` (Apply z biblioteki, inline confirm, `mode: "append"`) oraz `frontend/src/components/templates/ImportTemplatePreviewModal.tsx` (Phase 5D.2 — przycisk direct imported-YAML Apply + gating), `frontend/src/App.tsx` (post-apply refresh `inventoryVersion`, `saveLoadKey`, slotów, undo — Phase 5D.2 reużywa go bez zmian).
- **Backend impact**: nowa warstwa apply; reużywa istniejące writery dokładnie. Phase 5D.2 nie dodała zmian w backendzie ani bindingsach.
- **Frontend impact**: Apply enabled na wpisach biblioteki i na preview zaimportowanego YAML, których `selectedSections ⊆ { profile, stats }`; wpisy v2 niosące dowolną inną sekcję pozostają disabled; szablony v1 zaimportowane nigdy nie pokazują nowego przycisku v2 Apply. Ścieżka direct imported-YAML Apply reużywa `ApplyBuildTemplateV2ToCharacterJSON` na canonical JSON wyprodukowanym już przez preview importu — brak drugiego file dialogu, brak TOCTOU re-read między preview a apply. `ApplyBuildTemplateV2FromFileToCharacter` istnieje backend/bindings-side, ale celowo pozostaje niepodpięty w UI; supported flow obejmuje teraz zarówno `Import YAML → Save to Library → Apply from Library`, jak i `Import YAML → Preview → Apply to character`.
- **Testy**: backend apply happy path; rollback na error; `profile.class` raportowany w `Skipped`; ścieżki delegacji library + file pokryte; frontend (Phase 5D.2) pokrywa szablony v1 nigdy nie oferujące nowego przycisku, wszystkie ścieżki gating failure dla importów v2, przekazanie kliknięcia do `ApplyBuildTemplateV2ToCharacterJSON` z `mode: "append"`, ścieżkę sukcesu `applied=true` (close + toasty + `onCharacterTemplateApplied`), ścieżki `applied=false` i thrown-error (error toast + preview pozostaje otwarty) oraz niezależność Save-to-Library.
- **Manual validation**: 2026-05-31 — Phase 5D.1: zaaplikowano wpis biblioteki v2 z selekcją profile + stats do aktywnej postaci na `feature/templates-v2-apply-profile-stats`; inline confirm działa; Apply zakończony sukcesem; wybrane pola zmieniają się; post-apply refresh odbija nowy stan; wpisy v1 pozostają disabled w global shell (brak `sessionID`); niewspierane wpisy v2 pozostają disabled. Phase 5D.2: na `feature/templates-v2-direct-yaml-apply` import YAML v2 z `selectedSections ⊆ { profile, stats }` przez `Import YAML from File…` i kliknięcie "Apply to character" zaaplikowały te same pola co ścieżka biblioteki; pominięcie `profile.class` zostało zaraportowane info-toastem gdy `class` było wybrane; preview zamknął się przy sukcesie i refresh dance w `App.tsx` zaktualizował widoczny stan; zaimportowane YAML v1 nadal pokazywały wyłącznie `Save to Library` bez przycisku v2 Apply; importy v2 niosące niewspierane sekcje pozostały z disabled Apply z tooltipem o wspieranym zakresie; ścieżka library Apply z Phase 5D.1 pozostała bez zmian.
- **Ryzyka**: respektowane — istniejące locking i integrity gate zachowane. Phase 5D.2 nie wprowadziła nowej powierzchni lock; reużyła endpoint z Phase 5D.1 as-is.
- **Out of scope**: Gender / VoiceType (Phase 8 przez helpery appearance), equipment / equipped talismans / spells / appearance / weapon-level override; apply-time value editing / overrides dla subsetu profile/stats dostarczone w Phase 6 poniżej.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 6 — apply-time overrides dla profile + stats — ✅ Dostarczone 2026-05-31

- **Cel**: pozwolić użytkownikowi edytować wartości profile + stats **przed** dotarciem apply do backendu, na tych samych powierzchniach co Phase 5 (preview direct YAML import + lista biblioteki). Reużyć writer backendu Phase 5 bez nowego kodu backendu, bez nowych bindings i bez zmian w `App.tsx`.
- **Podejście**: frontend-only mutacja canonical JSON, którego użytkownik już widział w preview (direct YAML) lub którego entry był już w bibliotece (library path). Zmutowany JSON jest posyłany przez istniejący `ApplyBuildTemplateV2ToCharacterJSON(charIdx, mutatedCanonicalJSON, { mode: "append" })`; ścieżka "Apply with overrides…" z biblioteki pobiera canonical JSON istniejącego wpisu przez już dostarczone binding `PreviewBuildTemplateFromLibrary`, bez dodawania nowego endpointu.
- **Pliki (dostarczony scope)**: `frontend/src/components/templates/ApplyOverridesPanel.tsx` (nowy, eksportuje `ApplyOverridesPanel`, `ApplyOverridesModal` oraz czysty helper `applyOverridesToCanonical`), `frontend/src/components/templates/ImportTemplatePreviewModal.tsx` (drugi v2 przycisk `Apply with overrides…` obok istniejącego `Apply to character`), `frontend/src/components/templates/TemplateLibraryModal.tsx` (per-entry przycisk `Apply with overrides…` obok istniejącego Apply), `frontend/src/components/templates/TemplatesShellModal.tsx` (wspólny `OverridesSource` discriminator + handlery dla obu powierzchni). Testy frontendowe we wszystkich czterech komponentach (+1 nowy plik testowy). Backend, bindings i `App.tsx` są nietknięte.
- **Edytowalny zakres**: identyczny z writerem Phase 5 — `profile.{name,level,runes,soulMemory,clearCount,scadutreeBlessing,shadowRealmBlessing,talismanSlots}` oraz wszystkie osiem `stats.*`. `profile.class` jest renderowane jako read-only z hintem "Skipped on apply (Phase 5)" zamiast edytowalnego inputu.
- **Zakresy UI (mirror walidatora schema)**: `level [1, 713]`, `clearCount [0, 7]`, `scadutreeBlessing [0, 20]`, `shadowRealmBlessing [0, 10]`, `talismanSlots [0, 3]`, stats `[1, 99]`. `runes` ma miękki warning powyżej `999_000_000`, ale nie ma hard-cap. Backend pozostaje źródłem prawdy dla całej walidacji; UI pre-sprawdza zakresy by Apply pozostawał uczciwy i pokazuje per-field inline error.
- **Semantyka selection**: odznaczenie pola usuwa z mutowanego JSON zarówno `sections.{profile,stats}[field]` jak i `selection.{profile,stats}[field]`. Zaznaczenie pola dodaje oba. Kontrakt Phase 5 — "applied = selected ∧ present" — pozostaje zachowany bez zmian.
- **Backend impact**: brak. `ApplyTemplateV2Options` zachowuje pojedyncze pole `Mode`; JSON-owy endpoint re-walidatuje wszystko end-to-end.
- **Frontend impact**: dwa nowe przyciski (jeden na preview importu, jeden per v2 wiersz biblioteki); jeden nowy modal; jeden nowy czysty helper; istniejący przycisk `Save to Library` na preview importu pozostaje niezależny i zapisuje oryginalny canonical JSON, nie edits z modala. Szablony v1 nigdy nie widzą nowych przycisków. Szablony v2 z niewspieranymi sekcjami (equipment / spells / equippedTalismans / appearance / inventory.workspace) zachowują oba v2 przyciski disabled z istniejącym tooltipem "profile / stats only in this phase". Szybka ścieżka library Apply przez `ApplyBuildTemplateV2FromLibraryToCharacter` jest bez zmian.
- **Testy**: +43 case'y frontendowe — 19 w `ApplyOverridesPanel.test.tsx` (nowy), +7 w `ImportTemplatePreviewModal.test.tsx`, +5 w `TemplateLibraryModal.test.tsx`, +12 w `TemplatesShellModal.test.tsx`. Pokrywają rendering / mutację / range validation / soft cap / toggle-off removal / `profile.class` read-only / preservację non-profile/stats sekcji / invalid-JSON banner / obie powierzchnie forwardujące zmutowany JSON / success-close / `applied=false` i thrown-error trzymające modal otwarty / cancel porzucający edits / invalid blokujący Apply / szybką library Apply path nietkniętą / `PreviewBuildTemplateFromLibrary` zwracające brak canonical JSON jako error toast / info toast pominięcia `profile.class`.
- **Manual validation**: 2026-05-31 — na `feature/templates-v2-apply-overrides` edytowano wartości profile + stats przez obie ścieżki (direct YAML import i library `Apply with overrides…`); edycje wylądowały na wybranej postaci bez dotykania pozostałych pól; szybka library Apply path pozostała bez zmian; importy v1 nadal pokazywały wyłącznie legacy `Save to Library` bez nowego przycisku; importy v2 z niewspieranymi sekcjami zachowały disabled przyciski z tooltipem o wspieranym zakresie; cancel modala porzucał edits bez mutacji save; `Save to Library` nadal zapisywał oryginalny canonical JSON, ignorując edits w modalu.
- **Ryzyka**: respektowane — Phase 6 nie wprowadza nowej powierzchni lock, nowej write path, nowego endpointu. Kontrakt backendu Phase 5 jest jedynym miejscem mutacji.
- **Out of scope**: weapon level override przy apply (Phase 6b poniżej), inventory / storage / equipment / spells / appearance / sort order / world progress edits przy apply, item quantities, URL import, multi-character pack, "Save edited copy" edits z modala z powrotem do biblioteki.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 6b — weapon level override dla ścieżki Apply v1 inventory.workspace — ✅ Dostarczone 2026-05-31

- **Cel**: pozwolić użytkownikowi, przy apply na istniejącej ścieżce v1 `inventory.workspace` (akcja `Apply Template ▾` wewnątrz `SortOrderTab.tsx`), wymusić na każdej standard-upgradeable broni dodawanej przez szablon `+N`, a na każdej somber/special broni `+M`, clampowane do `MaxUpgrade` każdej broni z DB. Default = brak override (`Enabled = false`), zachowanie byte-for-byte identyczne ze ścieżką sprzed Phase 6b. Bez zmiany schematu v2, bez writera v2 inventory, bez writera equipment.
- **Pliki (dostarczony scope)**: `backend/templates/import.go` (dwa nowe kody warningu `IssueCode*`: `IssueCodeWeaponLevelClamped = "weapon_level_clamped"`, `IssueCodeWeaponUnupgradeable = "weapon_unupgradeable"`); `app_templates.go` (addytywna struct `WeaponLevelOverride`, addytywne pole `WeaponLevelOverride *WeaponLevelOverride` na istniejącym `ApplyTemplateOptions`, helper `validateWeaponLevelOverride`, nowy helper `applyWeaponLevelOverride`, rozszerzony sygnatura `applyTemplateItemsToWorkspace`); `app_templates_weapon_override_test.go` (**nowy**, ~390 linii, 14 funkcji testowych / 16 case'ów z subtestami); `frontend/src/components/SortOrderTab.tsx` (nowy state, parsery, builder payloadu `buildWeaponLevelOverride`, panel override wewnątrz istniejącego dropdownu Templates — testid `weapon-override-panel` z master checkboxem `weapon-override-enabled` i dwoma number inputami `weapon-override-standard` range `0..25` / `weapon-override-somber` range `0..10`, inline walidacja `weapon-override-error`, filtr toastów dla warningów override); `frontend/src/components/SortOrderTab.test.tsx` (+233 linii, nowy blok `Phase 6b weapon level override`, +8 case'ów); `frontend/wailsjs/go/models.ts` zregenerowane przez wewnętrzny krok `wails generate module` w `make build` (dodaje klasę `WeaponLevelOverride` i opcjonalne pole `weaponLevelOverride?: WeaponLevelOverride` na `ApplyTemplateOptions`). `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, `TemplateLibraryModal`, `App.tsx` i warstwa apply v2 (`app_templates_v2_apply.go`) celowo pozostają nietknięte.
- **Finalny kształt runtime option**:
  ```go
  type WeaponLevelOverride struct {
      Enabled       bool `json:"enabled,omitempty"`
      StandardLevel *int `json:"standardLevel,omitempty"`
      SomberLevel   *int `json:"somberLevel,omitempty"`
  }
  ```
  Oba pointery na level są niezależne: UI może celować tylko w standard, tylko w somber albo w obie klasy naraz. `Enabled = false` czyni cały override no-opem niezależnie od wartości pointerów; `validateWeaponLevelOverride` odrzuca `Enabled = true` z oboma pointerami nil i odrzuca ujemne poziomy (`StandardLevel < 0` lub `SomberLevel < 0`) standardowym prefiksem błędu `ApplyBuildTemplate: …`, **zanim** uruchomi się jakakolwiek mutacja workspace.
- **Apply layer**: `applyTemplateItemsToWorkspace` przepuszcza teraz override i wywołuje `applyWeaponLevelOverride` **po** każdym template-side patchu `editor.UpdateWeapon` (Upgrade / Infusion / AoW z szablonu). Helper switchuje na `added.MaxUpgrade` (populowane już przez `editor.AddItem` przez `db.GetItemDataFuzzy` — zob. §14.3): `25` → standard (używa `StandardLevel` jeśli non-nil); `10` → somber/special (używa `SomberLevel` jeśli non-nil); `0` → unupgradeable; inna wartość → silent skip. Dla standard / somber helper liczy `clamped := editor.ClampUpgrade(req, added.MaxUpgrade)` i wywołuje `editor.UpdateWeapon(snap, added.Handle, container, editor.WeaponPatch{Upgrade: &clamped})`. Jeśli `clamped != req`, warning `templates.IssueCodeWeaponLevelClamped` jest dopisywany do `report.Warnings`. Dla unupgradeable warning `templates.IssueCodeWeaponUnupgradeable` jest dopisywany, a override pominięty. Override nigdy nie dodaje, nie usuwa ani nie relokuje przedmiotów i nigdy nie dotyka `Infusion` ani `AoW`. Mutacja w pełni wewnątrz aktywnego `InventoryWorkspaceSnapshot`; z ścieżki override żadne bajty nie trafiają do `slot.Data`; `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu i nigdy nie jest wywoływane automatycznie przez override.
- **Decyzja o lokalizacji UI**: kontrolki żyją wewnątrz istniejącego dropdownu Templates w `SortOrderTab.tsx`, więc ta faza ship bez dotykania żadnego z czterech globalnych modali Templates. Globalny Templates shell nie udostępniał override, ponieważ jego ścieżki Apply v2 nie sięgały do writera v1 `inventory.workspace` w momencie shipu Phase 6b; Phase 7a dostarczył ścieżkę Apply v2 `inventory.workspace`, a Phase 7a.2 dodał siostrzany `WeaponLevelOverridePanel` wewnątrz istniejącego `ApplyOverridesModal` dla powierzchni v2 — dropdown v1 pozostaje na miejscu byte-for-byte. Czy zrelokować / skonsolidować dropdown v1 jest teraz osobną decyzją (Phase 12). Puste pola number oznaczają "zostaw tę klasę w spokoju" (odpowiadający pointer zostaje nil w payloadzie); inline element `weapon-override-error` blokuje Apply gdy panel jest enabled z obydwoma pustymi polami albo z dowolnym polem poza zakresem.
- **Backend impact**: addytywna opcja na istniejącym DTO apply; nowe czyste helpery `validateWeaponLevelOverride` + `applyWeaponLevelOverride`; istniejący template-side writer bez zmiany. Scope guard v2 `inventory.workspace` zachowany.
- **Frontend impact**: jeden nowy panel (testid `weapon-override-panel`) wewnątrz dropdownu Templates; jeden nowy helper budujący payload; jeden nowy filtr toastów dla warningów override. Żadna inna powierzchnia UI nie zmieniona.
- **Testy**: targeted `go test . -run 'TestApplyTemplate_Override|TestValidateWeaponLevelOverride'` — wszystkie PASS (14 funkcji / 16 case'ów z subtestami). Case'y: validator akceptuje nil + disabled; validator odrzuca `Enabled = true` z oboma pointerami nil; validator odrzuca ujemne `StandardLevel` i `SomberLevel`; override `nil` i `Enabled = false` zostawiają upgrade nietknięty; `Enabled = true` z oboma polami nil odrzucone przed mutacją workspace; `StandardLevel` dotyka tylko standardowych broni; `SomberLevel` dotyka tylko somber broni; obie naraz dotykają każdej klasy niezależnie; żądania `+26` standard / `+11` somber clampowane w dół z `IssueCodeWeaponLevelClamped` i lądują na `+25` / `+10`; wartości dokładnie na `MaxUpgrade` nie produkują warninga; `MaxUpgrade == 0` (unarmed) emituje `IssueCodeWeaponUnupgradeable` i skipuje; przy błędzie preview workspace zostaje niezmieniony; clamp do zera z zerowego żądania nie emituje warninga. Pełny backend `go test . ./backend/... ./tests/...` 8/8 pakietów PASS; `go vet` clean; `tsc --noEmit` clean; targeted vitest dla `SortOrderTab.test.tsx` 31 PASS (8 nowych dla Phase 6b); pełny vitest 16 suit / 328 PASS; `make build` PASS.
- **Manual validation**: 2026-05-31 na `feature/templates-v1-weapon-level-override`. Zaaplikowano szablon v1 z mixed standard + somber + unupgradeable broniami pod każdą kombinacją kontrolek (override off / tylko `StandardLevel` / tylko `SomberLevel` / obie / `StandardLevel` ponad `+25` / `SomberLevel` ponad `+10`); zachowanie per-klasa zgodne z liniami warningu w raporcie; workspace zmutowany tylko przez `editor.UpdateWeapon`; użytkownik dalej committuje przez `Save changes`; ścieżki Apply v2 Phase 5 / 5D.2 / 6 oraz Phase 9 URL import bez zmian.
- **Ryzyka**: zachowane — Phase 6b nie wprowadza nowej powierzchni lock, nowej write path ani nowego endpointu. Mutacja zostaje wewnątrz istniejącej workspace edit session, gated przez istniejący `slotMu[charIdx]` callera `applyTemplateItemsToWorkspace`.
- **Out of scope**: writery equipped-weapon (Phase 7b); zmiana `Infusion` lub `AoW` (wartości szablonu są passowane bez zmian); override per-broń (zamiast per-klasa); direct save mutation przez `core.PatchWeaponItemID` ze ścieżki template apply. (Phase 7a.2 poniżej podniósł override na ścieżkę Apply v2 `inventory.workspace`; semantyka dropdownu v1 pozostaje bez zmian.)
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: ukończone.

### Phase 7a — Apply v2 `inventory.workspace` do aktywnej Inventory Edit Session — ✅ Dostarczone 2026-05-31

- **Cel**: dostarczyć pierwszą realną ścieżkę Apply v2 dla `inventory.workspace`. Do tej fazy warstwa apply v2 (`app_templates_v2_apply.go`) odrzucała `inventory.workspace` outright przez scope guard odziedziczony z Phase 5. Phase 7a podnosi ten guard wyłącznie dla `inventory.workspace` i przerzuca payload v2 przez **aktywną `InventoryEditSession` / `InventoryWorkspaceSnapshot`** tak, żeby zapisy lądowały w workspace na którym operuje `SortOrderTab.tsx` — nigdy bezpośrednio do `slot.Data`. Użytkownik najpierw otwiera workspace Sort Order, aplikuje szablon v2 (z biblioteki, z preview direct YAML, z preview URL import, lub przez `Apply with overrides…`), a następnie commituje zmianę dokładnie tak jak dzisiaj przez `Save changes` (`SaveInventoryWorkspaceChanges`). Brak writera equipment / equipped-talismans / spells / appearance — te pozostają Phase 7b / 7c / 7d / 8. Weapon level override Phase 6b jest celowo **niepodłączony** do ścieżki v2 w tej fazie — pozostaje feature'em dropdownu Templates w `SortOrderTab.tsx`; apply v2 inventory przekazuje hard-coded `nil` override do `applyTemplateItemsToWorkspace`. Podłączenie Phase 6b do v2 było trackowane jako Phase 7a.2 poniżej i zostało dostarczone — pin `nil` został zastąpiony przez `opts.WeaponLevelOverride`.
- **Podejście**: warstwa apply klasyfikuje sparsowany payload v2 do trzech flag — `hasProfile`, `hasStats`, `hasInventory` — i gatuje pracę na nich. Dla `hasInventory == true` apply acquires caller-supplied session (pełna kolejność `lifecycleMu → sess.mu` zachowana), waliduje `sess.CharacterIndex == charIdx`, uruchamia preflight pojemności na **istniejącym** workspace **przed** jakąkolwiek mutacją, robi `core.SnapshotSlot` na `slot.Data` i value-type deep copy `sess.Workspace`, następnie aplikuje inventory + storage items przez `applyTemplateItemsToWorkspace(&sess.Workspace, …, editor.ContainerInventory, nil)` i storage equivalent. Zapisy profile/stats lecą **po** gałęzi inventory w tym samym oknie `slotMu[charIdx]`, reużywając ścieżkę Phase 5 bez zmian. Closure `rollbackBoth()` przywraca zarówno snapshot bajtów slotu jak i snapshot value workspace na każdym error exit, więc mixed profile+stats+inventory.workspace apply jest atomowy. Dla `hasInventory == false` istniejący Phase 5 edit-session conflict guard jest zachowany bez zmian.
- **Pliki (dostarczony scope)**: `app_inventory_session.go` (nowy struct `ActiveInventoryEditSession` + nowy endpoint `App.GetActiveInventoryEditSessionForCharacter(charIdx int)` który konsultuje `editSessionsMu` + `editSessionByChar` i zwraca `{ active: bool, sessionID: string }`; nigdy nie erroruje); `app_templates_v2_apply.go` (rozszerzony `ApplyTemplateV2Options` o `SessionID string json:"sessionID,omitempty"`, rozszerzony `ApplyTemplateV2Result` o `InventoryItemsApplied int`, `StorageItemsApplied int`, opcjonalny `Workspace *editor.InventoryWorkspaceSnapshot`, nowy session/scope branching w `ApplyBuildTemplateV2ToCharacterJSON`, dual snapshot rollback, preflight pojemności przed mutacją, sentinel `"inventory.workspace"` dopisany do `Applied` gdy items lądują); `backend/templates/import.go` (nowe issue codes `IssueCodeInventorySessionRequired = "inventory_session_required"` oraz `IssueCodeInventorySessionInvalid = "inventory_session_invalid"`); `app_templates_v2_apply_inventory_test.go` (**nowy**, ~280 linii, 8 testów); `app_templates_v2_apply_test.go` (trzy istniejące testy "without session" zaktualizowane do oczekiwania nowego kodu); `frontend/src/components/templates/ImportTemplatePreviewModal.tsx` (module-level `V2_APPLY_SUPPORTED_SECTIONS = ['profile', 'stats', 'inventory.workspace']`); `frontend/src/components/templates/TemplateLibraryModal.tsx` (per-row `v2HasApplyableSections` akceptuje teraz `inventory.workspace`); `frontend/src/components/templates/TemplatesShellModal.tsx` (importuje `GetActiveInventoryEditSessionForCharacter`, dodaje stałe `INVENTORY_WORKSPACE_SECTION` + `NO_SESSION_MESSAGE`, helper `fetchActiveSessionID(charIndex)`, helper `canonicalJSONNeedsSession(canonical)`, oraz nowe session checks w `handleApplyV2FromLibrary` / `handleApplyV2FromImportedPreview` / `handleConfirmOverrides`); `frontend/src/components/templates/__tests__/TemplatesShellModal.test.tsx` (nowy blok `Phase 7a v2 inventory.workspace apply`, +8 cases); `frontend/src/components/templates/__tests__/TemplateLibraryModal.test.tsx` (dwa cases przepisane — entries v2 `inventory.workspace`-only włączają teraz Apply i renderują przycisk overrides); `frontend/wailsjs/go/main/App.{d.ts,js}` + `frontend/wailsjs/go/models.ts` zregenerowane przez wewnętrzny krok `wails generate module` w `make build` (bez ręcznych edycji — nowa klasa `ActiveInventoryEditSession` + metoda `GetActiveInventoryEditSessionForCharacter`, `sessionID?: string` na `ApplyTemplateV2Options`, `inventoryItemsApplied`, `storageItemsApplied`, opcjonalny `workspace?: editor.InventoryWorkspaceSnapshot` na `ApplyTemplateV2Result`).
- **Backend impact**: brak nowej sekcji schemy — Phase 7a reużywa istniejącą sekcję `inventory.workspace` z readera v1. Brak nowej powierzchni lockowej — session acquire reużywa istniejącą ścieżkę `acquireSession`. Brak nowego writera — `applyTemplateItemsToWorkspace` to dokładnie ta sama mutacja workspace, której używa apply v1 `inventory.workspace`. Mutacja nigdy nie dotyka `slot.Data` bezpośrednio; `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu.
- **Frontend impact**: profile/stats-only apply v2 lecą nadal z `sessionID = ''` — backend ignoruje to na gałęzi non-inventory. Library / direct YAML / URL Apply / `Apply with overrides…` reużywają tę samą ścieżkę canonical JSON; dla szablonów niosących `inventory.workspace` shell wyszukuje aktywną sesję przez `GetActiveInventoryEditSessionForCharacter` i odmawia tostem "Open the Sort Order workspace before applying inventory templates." gdy żadna nie jest aktywna. Szablony v2 niosące nieobsługiwane sekcje poza `{profile, stats, inventory.workspace}` nadal trzymają oba przyciski v2 wyłączone z tooltipem o wspieranym scope.
- **Testy**: targeted backendowe testy Phase 7a (`TestApplyBuildTemplateV2_Inventory_UnknownSessionRejected`, `..._WrongCharacterSessionRejected`, `..._HappyPath`, `..._EmptyItems_NoOp`, `TestApplyBuildTemplateV2_Mixed_ProfileStatsInventory_HappyPath`, `TestApplyTemplateV2Options_FieldSurface`, `TestApplyBuildTemplateV2_UnknownSectionStillRejected`, plus trzy zaktualizowane testy `..._InventoryWorkspaceWithoutSessionRejected`) wszystkie PASS. Regresja Phase 6b (`TestApplyTemplate_Override*`, `TestValidateWeaponLevelOverride*`) wszystkie PASS. Pełny backend `go test . ./backend/... ./tests/...` 8/8 paczek PASS; `go vet` clean; `tsc --noEmit` clean; targeted vitest `TemplatesShellModal.test.tsx` 66 PASS / `ImportTemplatePreviewModal.test.tsx` 43 PASS / `TemplateLibraryModal.test.tsx` 45 PASS; pełny vitest 16 suit / **336 PASS** (było 328 przed Phase 7a, +8 cases); `make build` PASS.
- **Manual validation**: 2026-05-31 na `feature/templates-v2-inventory-workspace-apply` (HEAD `3e448f0`). Potwierdzone na prawdziwym save: zaaplikowanie szablonu v2 z zaznaczonym `inventory.workspace` **bez** otwartego workspace Sort Order surfacowało toast "Open the Sort Order workspace before applying inventory templates." i **nie** wywołało bindingu; otwarcie workspace i ponowne zaaplikowanie wylądowało itemy w gridzie workspace; `Save changes` commitowało je do `slot.Data`; reload save'a pokazywał itemy z poprawnymi acquisition indices i bez integrity warnings. Mixed szablon profile + stats + inventory.workspace v2 zaaplikował się atomowo — pola profile/stats zaktualizowały się i itemy inventory wylądowały w jednej akcji użytkownika. Zarówno URL import jak i library apply reużywały tę samą ścieżkę canonical JSON. Ścieżki Phase 5 / 5D.2 / 6 v2 dla profile/stats-only, Phase 6b weapon level override na ścieżce v1 SortOrderTab, i ścieżka Phase 9 URL import — wszystkie pozostały bez zmian.
- **Ryzyka**: respektowane — Phase 7a nie wprowadza nowej powierzchni lockowej poza istniejącym session acquire. Mutacja pozostaje wewnątrz `sess.Workspace`. Dual snapshot rollback zamyka okno, w którym partial inventory write mógłby wyciec po późniejszym błędzie walidacji profile/stats.
- **Out of scope (nadal future work)**: Phase 7b — equipment slot writer (zob. niżej). Phase 7c — equipped talismans writer. Phase 7d — spell loadout writer. Phase 8 — appearance via preset. Phase 10 — multi-character pack. (Phase 7a.2 — podłączenie Phase 6b weapon level override do ścieżki Apply v2 `inventory.workspace` — zostało dostarczone, zob. poniżej.)
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 7a.2 — weapon level override na ścieżce Apply v2 `inventory.workspace` — ✅ Dostarczone 2026-05-31

- **Cel**: podłączyć weapon level override z Phase 6b do ścieżki Apply v2 `inventory.workspace` z Phase 7a, tak żeby użytkownicy mogli nadpisywać poziomy upgrade'u broni standardowych / somber przy apply na każdej powierzchni v2 (library `Apply with overrides…`, preview direct YAML, preview URL import). Phase 7a celowo hard-codował `nil` override w dwóch wywołaniach `applyTemplateItemsToWorkspace` i trackował podłączenie jako Phase 7a.2; ta faza zamienia oba piny na `opts.WeaponLevelOverride`. Override pozostaje **runtime apply option**, nigdy pole schematu szablonu — canonical JSON, który użytkownik podgląda, nigdy go nie niesie, a opcja podróżuje wyłącznie przez strukturę `ApplyTemplateV2Options`. Dropdown v1 Phase 6b w `SortOrderTab.tsx` jest bez zmian; ścieżka apply v1 (`ApplyBuildTemplateToWorkspaceJSON` z `ApplyTemplateOptions.WeaponLevelOverride`) działa byte-for-byte bez zmian.
- **Podejście**: reuse, nie duplikacja. `ApplyTemplateV2Options` dostaje addytywny `WeaponLevelOverride *WeaponLevelOverride json:"weaponLevelOverride,omitempty"`; **ten sam typ** zadeklarowany w `app_templates.go` dla v1 jest referowany — bindings tym samym wystawiają jedną klasę `WeaponLevelOverride` współdzieloną między powierzchniami v1 i v2. `validateWeaponLevelOverride` ze ścieżki v1 odpala się **przed** `acquireSession`, więc strukturalnie broken override (`Enabled = true` z oboma pointerami nil albo z negatywnym poziomem) odbija się z `templates.IssueCodeStructureInvalid` i zerowymi side effects. Na gałęzi inventory dwa wywołania `applyTemplateItemsToWorkspace(&sess.Workspace, …, nil)` stają się `…, opts.WeaponLevelOverride)`; sam helper, `applyWeaponLevelOverride`, oraz dual snapshot rollback z Phase 7a są bez zmian. Warningi (`weapon_level_clamped`, `weapon_unupgradeable`) lecą do `ApplyTemplateV2Result.Preview.Warnings` przez istniejącą agregację `invWarn` / `stoWarn`; są to warningi, nigdy błędy. Strukturalnie poprawny override na szablonie profile/stats-only (`selection.inventory.workspace` nieobecne) jest silently ignored — pętla override po prostu nie ma na czym operować, lustrując sposób, w jaki `SessionID` jest silently ignored na gałęzi non-inventory.
- **Pliki (dostarczony scope)**: `app_templates_v2_apply.go` (rozszerzony `ApplyTemplateV2Options` o `WeaponLevelOverride *WeaponLevelOverride`, wywołanie walidacji przed `acquireSession`, dwie podmiany `nil` → `opts.WeaponLevelOverride` w wywołaniach `applyTemplateItemsToWorkspace` dla inventory + storage); `app_templates_v2_apply_inventory_test.go` (pin compile-time `TestApplyTemplateV2Options_FieldSurface` zaktualizowany do nowej trzy-polowej shape `{Mode, SessionID, WeaponLevelOverride}`); `app_templates_v2_apply_weapon_override_test.go` (**nowy**, ~340 linii, 14 cases); `frontend/src/components/templates/WeaponLevelOverridePanel.tsx` (**nowy**, ~145 linii, posiada własny state, emituje `{ enabled: true, standardLevel?, somberLevel? } | undefined` plus flag `hasInvalid`, testidy `apply-overrides-weapon-{panel,enabled,standard,somber,error}` różne od testidów v1 `weapon-override-*` na `SortOrderTab.tsx`); `frontend/src/components/templates/ApplyOverridesPanel.tsx` (`ApplyOverridesModal` wykrywa `selection['inventory.workspace']` i warunkowo renderuje `WeaponLevelOverridePanel` pod gridem profile/stats; `onConfirm` poszerzone do `(mutatedJSON, weaponOverride?) ⇒ …`; `canApply` blokuje gdy któryś z paneli invalid; status pill przełącza się na "Fix weapon level override to apply." gdy panel weapon jest blokerem; `ApplyOverridesPanel` sam — JSON mutator — bez zmian); `frontend/src/components/templates/TemplatesShellModal.tsx` (`handleConfirmOverrides` przyjmuje nowy drugi argument i przekazuje go jako `weaponLevelOverride` wewnątrz `main.ApplyTemplateV2Options.createFrom({ mode, sessionID, weaponLevelOverride })`); `frontend/wailsjs/go/models.ts` zregenerowane przez wewnętrzny krok `wails generate module` w `make build` (jedyna delta: `ApplyTemplateV2Options` niesie `weaponLevelOverride?: WeaponLevelOverride` plus odpowiednia linia konstruktora; klasa `WeaponLevelOverride` już istniała ze ścieżki v1 — brak duplikatu). Nowe testy: `frontend/src/components/templates/__tests__/WeaponLevelOverridePanel.test.tsx` (**nowy**, 9 cases) oraz dopisy do `__tests__/ApplyOverridesPanel.test.tsx` (+5 Phase 7a.2 cases) i `__tests__/TemplatesShellModal.test.tsx` (+6 Phase 7a.2 cases).
- **Backend impact**: **brak nowej sekcji schemy** — Phase 7a.2 to runtime option, nie pole szablonu. **Brak nowych issue codes** — warningi (`weapon_level_clamped`, `weapon_unupgradeable`) i kod odrzucenia (`structure_invalid`) wszystkie istnieją przed tą fazą. **Brak nowej powierzchni lockowej** — walidacja odpala się przed `acquireSession`, mutacja reużywa session lock i dual snapshot rollback z Phase 7a. Mutacja override odpala się w pełni wewnątrz `sess.Workspace` przez `editor.UpdateWeapon`; ze ścieżki override żadne bajty nie trafiają do `slot.Data`; `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu.
- **Frontend impact**: `ApplyOverridesPanel` (JSON mutator) jest niezmieniony na szablonach profile/stats-only — panel weapon nie renderuje się w ogóle w tym przypadku. Dla szablonów wybierających `inventory.workspace` modal rośnie o weapon sub-panel pod gridem profile/stats; szablony inventory-only mogą używać modalu wyłącznie dla weapon level override (grid profile/stats renderuje puste nagłówki, bez pól). Weapon override **nigdy** nie podróżuje wewnątrz canonical JSON — wyłącznie przez nowe pole `weaponLevelOverride` na `ApplyTemplateV2Options`. Phase 7a session gating nadal wygrywa: szablon niosący `inventory.workspace` bez aktywnej Inventory Edit Session jest odrzucany upstream niezależnie od stanu override, a binding nigdy nie jest wołany. Fast library Apply (`ApplyBuildTemplateV2FromLibraryToCharacter`) nadal nie wysyła override; tylko `Apply with overrides…` udostępnia panel. Dropdown v1 Phase 6b w `SortOrderTab.tsx` jest bez zmian.
- **Testy**: targeted backendowe testy Phase 7a.2 pokrywają nil override no-op; disabled override no-op; standard-only override dotyka tylko standardowych broni; somber-only override dotyka tylko somber broni; oba poziomy dotykają każdej klasy niezależnie; standard +99 clampuje do +25 z `weapon_level_clamped`; somber +99 clampuje do +10 z tym samym kodem; `MaxUpgrade == 0` (unupgradeable arm) emituje `weapon_unupgradeable` i skipuje; `Enabled = true` z oboma pointerami nil odrzucone z `IssueCodeStructureInvalid` przed jakąkolwiek mutacją; negatywny `StandardLevel` odrzucony; mixed profile+stats+inventory.workspace + override happy path landuje wszystkie trzy sekcje i override; mixed apply z invalid override shape rolluje cały state (brak mutacji bajtów slotu, brak mutacji workspace, flaga `Dirty` zachowana); szablon profile/stats-only z valid override silently ignored. Regresja Phase 6b (`TestApplyTemplate_Override*`, `TestValidateWeaponLevelOverride*`) wszystkie PASS bez zmian. Pełny backend `go test . ./backend/... ./tests/...` PASS; `go vet` clean; `tsc --noEmit` clean; frontendowe testy Phase 7a.2 (`WeaponLevelOverridePanel.test.tsx` 9 PASS, `ApplyOverridesPanel.test.tsx` +5 PASS, `TemplatesShellModal.test.tsx` +6 PASS); pełny vitest 17 suit / **357 PASS** (było 336 przed Phase 7a.2, +21 cases); `make build` PASS.
- **Manual validation**: 2026-05-31 na `feature/templates-v2-weapon-override` (HEAD `8fccd72`). Potwierdzone end-to-end na prawdziwym save: fast Apply bez overrides zachowuje poziomy upgrade'u broni zadeklarowane w szablonie; `Apply with overrides…` ze Standard = 25 ustawia każdą standardową broń na +25 (lub clampuje z `weapon_level_clamped`) i zostawia somber broni na ich template-side poziomach; Somber = 10 mirroruje symetryczny przypadek; oba poziomy zaaplikowane per-klasa niezależnie; enabling override z obydwoma pustymi polami wyłącza Apply; Standard = 26 / Somber = 11 / wartości negatywne wyłączają Apply z inline błędem; zamknięcie workspace Sort Order przed aplikacją surfacowało toast no-session z Phase 7a i override nigdy nie sięgnął backendu; mixed szablon profile + stats + inventory.workspace z override zaaplikował się atomowo w jednej akcji użytkownika; szablon profile/stats-only z valid override zaaplikował się bez renderowania panelu weapon i bez warningów override; URL import i library `Apply with overrides…` reużywały tę samą ścieżkę canonical JSON i to samo pole override; dropdown v1 Phase 6b w `SortOrderTab.tsx` działał byte-for-byte bez zmian.
- **Ryzyka**: respektowane — reuse walidatora i helpera override z v1 utrzymuje kontrakt identyczny między powierzchniami v1 i v2. Walidacja odpala się przed jakąkolwiek mutacją, więc broken override nie może zostawić partially-applied szablonu. Dual snapshot rollback z Phase 7a nadal pokrywa error exits z `applyTemplateItemsToWorkspace`.
- **Out of scope (nadal future work)**: equipment writer dla `ChrAsmEquipment` slotów 0..9 / 12–15 (Phase 7b); equipped-talismans writer (Phase 7c); spell loadout writer (Phase 7d); appearance przez preset (Phase 8); multi-character pack (Phase 10); opcjonalna przyszła konsolidacja UX dropdownu v1 Phase 6b w `SortOrderTab.tsx` z Templates shell teraz, gdy ścieżka v2 też niesie override (Phase 12, osobno akceptowany); override per-broń (zamiast per-klasa); zmiana `Infusion` lub `AoW` z panelu override.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: zakończone.

### Phase 7b — equipment slot writer (nowa write path)

- **Cel**: zaimplementować nowy publiczny write path dla `ChrAsmEquipment` slotów 0..9, 12–15 (bronie + zbroja), respektujący encoded item-ID form i hash 7/8 dependency udokumentowany w [06-equipment](06-equipment.md). Zaaplikować `sections.equipment` z szablonu przez ten nowy writer. Phase 7b to druga noga Phase 7+ — Phase 7a dostarczył już apply v2 `inventory.workspace`, zob. wyżej.
- **Pliki (planowany scope)**: nowy writer w `backend/core/` (prawdopodobnie `backend/core/equip_write.go`), wystawiony przez `backend/editor/` do planu; rozszerzenie warstwy apply; testy włącznie z hex-verified round-trip; frontend preview row dla equipment.
- **Backend impact**: nowy publiczny API dla zapisów equipment. Istniejący wyjątek `EquippedGreatRune` (już w `SyncPlayerToData`) jest zachowany.
- **Testy**: hex-identity round-trip dla no-op write; per-slot write; PC + PS4 platform parity; integrity gate interaction; default warn-and-skip gdy referowany item brakuje w inventory (§13.7).
- **Manual validation**: zaaplikować equipment do fixture character; round-trip obie platformy; in-game verification na co najmniej jednej platformie.
- **Ryzyka**: wysokie — pierwsza nowa write path do `ChrAsmEquipment`; dotyka sekcji którą wszystkie reference editors traktują jako read-only; hash 7/8 dependency musi być re-walidowana.
- **Out of scope**: equipped talismans, spell loadout, appearance.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: tak.

### Phase 7c — equipped talismans writer (nowa write path)

- **Cel**: zaimplementować writer equipped-talismans (`ChrAsmEquipment` sloty 17–21). Zaaplikować `sections.equippedTalismans` clampowane do aktualnego `profile.talismanSlots` celu (odrzuć jeśli długość przekracza `1 + slotCount`, albo warn + truncate wg decyzji §18 #9).
- **Pliki (planowany scope)**: rozszerzenie writera equipment z Phase 7b, lub sibling writer; rozszerzenie warstwy apply; testy; frontend preview row.
- **Backend impact**: rozszerza nowy publiczny API equipment write do slotów talismanów.
- **Testy**: respektuje limit Pouch; odrzuca overflow wg wybranej polityki; hex round-trip; PC + PS4 parity.
- **Manual validation**: zaaplikować equipped talismans; round-trip obie platformy.
- **Ryzyka**: średnie — opiera się na infrastrukturze Phase 7b.
- **Out of scope**: spell loadout; appearance.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: tak.

### Phase 7d — spell loadout writer (nowa write path)

- **Cel**: zaimplementować nowy publiczny write path dla 14 slotów `EquippedSpells`. Zaaplikować `sections.spells` przez niego. Dzisiaj to pole jest referowane wyłącznie przez hash recompute (`backend/core/hash.go:150`, `section_hash.go:24`) — brak edytor write surface.
- **Pliki (planowany scope)**: nowy writer w `backend/core/` dla `EquippedSpells`; rozszerzenie warstwy apply; testy; frontend preview row.
- **Backend impact**: nowy publiczny write API dla spell loadout.
- **Testy**: hex round-trip; PC + PS4 parity; preview odrzuca nieznane spell IDs.
- **Manual validation**: zaaplikować spells; round-trip obie platformy; in-game verification na co najmniej jednej platformie.
- **Ryzyka**: średnie — pierwsza zapis do obszaru spell loadout; per-platform offsety muszą być re-confirmed.
- **Out of scope**: appearance; URL import; multi-character pack.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: tak.

### Phase 8 — appearance via preset (reużywa istniejące helpery)

- **Cel**: zaaplikować `sections.appearance.preset` (i tym samym gender + voiceType związane z presetem) przez istniejący helper `app_appearance.go::ApplyPresetToCharacter`. Raw FaceData blob nigdy nie jest pisany z szablonu.
- **Pliki (planowany scope)**: rozszerzenie warstwy apply by routować sekcję appearance przez `app_appearance.go`; testy.
- **Backend impact**: reużywa istniejące helpery; brak nowej write path.
- **Testy**: apply preset; gender / voice consistency; rollback na failure; preview pokazuje preset name.
- **Manual validation**: zaaplikować preset; potwierdzić in-game appearance (Steam Deck verification path).
- **Ryzyka**: appearance jest wizualnie disruptive — preview musi wyraźnie pokazać docelową nazwę preset i ostrzec użytkownika przed apply.
- **Out of scope**: raw FaceData, multi-character pack, URL import.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: tak.

### Phase 9 — URL import (dostarczony 2026-05-31, pełne guardy)

- **Status**: ✅ Dostarczony 2026-05-31 na `feature/templates-v2-url-import`, ff-zmergowany do `main`.
- **Cel**: zaimplementować URL import wg §12 ze wszystkimi guardami (https-only, IP filter, redirect re-check, body size, timeouts, strict TLS, struct-typed YAML, brak includes, brak executable types, preview przed library, osobny confirm przed apply). Wszystkie guardy dostarczone.
- **Podejście**: rozszerzyć istniejącą ścieżkę preview file-import o `https://` source. Shell prosi backend o fetch URL pod guardami §12.3, backend oddaje bajty temu samemu `previewYAMLPayload`, którego używa ścieżka pliku, a wynikowy `LoadedTemplatePreview { Report, JSON, Path }` przepływa przez ten sam `ImportTemplatePreviewModal`. Wszystkie trzy akcje downstream (Save to Library, Apply to character przez Phase 5D.2, Apply with overrides przez Phase 6) ship bez zmian na powierzchni URL.
- **Pliki (dostarczony scope)**: `backend/templates/url_import.go` (**nowy**, eksportuje `FetchYAMLFromURL`, stałe `URLImport*`, `FetchError` oraz kody `IssueCodeURL*`); `backend/templates/url_import_test.go` (**nowy**, 28 case'ów fetchera + 21 subtestów `TestIsAllowedAddr`); `app_templates_url.go` (**nowy**, Wails handler `PreviewBuildTemplateImportYAMLFromURL`); `app_templates_url_test.go` (**nowy**, 6 case'ów integracyjnych); `frontend/src/components/templates/ImportTemplateFromURLModal.tsx` (**nowy**); `frontend/src/components/templates/TemplatesShellModal.tsx` (przycisk `Import from URL…` + wiring callbacka `onURLImportPreview`); `frontend/wailsjs/go/main/App.{d.ts,js}` zregenerowane przez wewnętrzny krok `wails generate module` w `make build`. `models.ts` bez zmian.
- **Backend impact**: nowy backend fetch przez standard library `net/http` client z custom `Transport.DialContext` (pre-connect IP filter dla literałów IP **i** każdego DNS-resolved adresu) oraz custom `Client.CheckRedirect` (re-check scheme + re-resolve + re-filter na każdym hopie, limit 3 redirecty). Strict `tls.Config { MinVersion: tls.VersionTLS12 }`, wyłącznie system root CAs, bez `InsecureSkipVerify`, bez custom CA. Body odpowiedzi cap'owane przez `io.LimitReader` na `URLImportMaxBodyBytes = 1 << 20` (1 MiB). Content-Type parsowany przez `mime.ParseMediaType` i sprawdzany względem allowlisty. Bez auth, bez cookies, bez custom headers. `http.Transport.DisableKeepAlives: true`.
- **Frontend impact**: nowy mały modal z pojedynczym inputem URL `https://`, lekka walidacja po stronie klienta (regex `^https?://`), Enter-to-submit, in-flight state "Fetching…", inline rendering błędu który zachowuje input przy odrzuceniu, Cancel który nie wywołuje bindingu. Shell pokazuje przycisk nagłówkowy `Import from URL…` obok istniejącego file-import (testid `templates-shell-import-url`). Preview URL reużywa `ImportTemplatePreviewModal` bez zmian — nie ma równoległej powierzchni "URL preview".
- **Testy**: każdy guard ma explicit test. Backend (`backend/templates/url_import_test.go`): wymuszenie `https`-only, odrzucenie loopback / RFC1918 / link-local / ULA / multicast / broadcast / unspecified, odrzucenie literałów cloud-metadata (`169.254.169.254`, `fd00:ec2::254`), filter na DNS-resolved IP, re-check redirectów na każdym hopie, cap redirectów, cap body, total timeout, mapowanie błędów TLS, allowlist Content-Type, mapowanie bad-status, brak auth / cookies / custom headers. Handler (`app_templates_url_test.go`): pusty URL, URL whitespace, scheme `http://`, scheme `data:`, literał loopback, literał IP cloud-metadata. Frontend (`__tests__/ImportTemplateFromURLModal.test.tsx`, 10 case'ów; `__tests__/TemplatesShellModal.test.tsx` blok `Phase 9 URL import`, 9 case'ów + 1 zaktualizowana inwariantna): render, disabled-empty, disabled-non-http(s), forwarding trimmed-URL, in-flight state, inline błąd zachowujący input, retry czyści błąd, thrown error surfacowany, Cancel nie wywołuje bindingu, Enter triggeruje Preview, przycisk widoczny w shellu, udany preview otwiera `ImportTemplatePreviewModal` ze ścieżką URL, Save to Library forwarduje canonical JSON, Apply to character forwarduje canonical JSON, Apply with overrides routuje przez Phase 6, importy v1 URL nigdy nie widzą przycisków v2, `report.ok = false` trzyma modal URL otwarty z inline błędem, thrown binding error trzyma modal otwarty, Cancel zamyka bez wywołania bindingu.
- **Manual validation**: 2026-05-31 — wklejenie publicznego URL `https://` serwującego v2 YAML otworzyło preview w tym samym modalu co file import; Save to Library zapisał payload URL jako wpis biblioteki, którego canonical JSON pasuje do preview; Apply to character zapisał wybrane pola przez endpoint Phase 5D.2; Apply with overrides routował edytowany canonical JSON przez ten sam endpoint. Guardy SSRF odrzuciły kolejno: URL `http://`, literał `127.0.0.1`, literał `169.254.169.254`, pusty URL, URL whitespace — każdy z poprawnym tagiem `IssueCode*` widocznym inline w modalu URL.
- **Ryzyka**: SSRF — gated przez §12.3 i pokryte zestawami testów fetchera i handlera.
- **Out of scope (nadal future work)**: authenticated downloads (basic / bearer / cookies); URL auto-refresh; domain allowlist; direct apply bez preview; opcjonalna metadata `sourceURL` persystowana w schemie biblioteki (odroczona dopóki osobna decyzja nie zaakceptuje rozszerzenia shape biblioteki); multi-character pack.

### Phase 10 — multi-character pack (osobno akceptowany)

- **Cel**: zob. §15. Source→destination mapping UI; per-slot rollback; explicit replace confirmation.
- **Out of scope dopóki nie jest osobno zaakceptowany**.

### Phase 11 — nazwane PvP / progression modules w szablonach (advanced, osobno akceptowany)

- **Cel**: opcjonalnie dodać `sections.modules` niosące listę nazwanych module presets (np. `pvp.colosseums`) które delegują do istniejących kontrolowanych flow jak `ApplyPvPPreparation`. **Nigdy** nie niesie raw flag IDs.
- **Out of scope dopóki nie jest osobno zaakceptowany**.

### Phase 12 — opcjonalne usunięcie / repozycjonowanie dropdownu `SortOrderTab` (osobno akceptowany)

- **Cel**: zdecydować czy istniejący dropdown staje się redirect do sidebar surface, jest usunięty, czy pozostaje jako shortcut.
- **Out of scope dopóki nie jest osobno zaakceptowany**.

---

## 17a. Status walidacji

### 17a.1. Log manual validation

| Pole | Wartość |
|---|---|
| Data walidacji | 2026-05-31 |
| Branch pod testem | `feature/templating-system` |
| Wynik | ✅ Pass — użytkownik potwierdził, że pełen flow create / preview / save / export / re-import działa end-to-end na prawdziwym save. |

| Pole | Wartość |
|---|---|
| Data walidacji | 2026-05-31 |
| Branch pod testem | `feature/templates-v2-apply-profile-stats` |
| Wynik | ✅ Pass — Phase 5 Apply v2 dla profile/stats z biblioteki zwalidowany manualnie end-to-end (`ApplyBuildTemplateV2FromLibraryToCharacter`, `mode: "append"`). |

| Pole | Wartość |
|---|---|
| Data walidacji | 2026-05-31 |
| Branch pod testem | `feature/templates-v2-direct-yaml-apply` |
| Wynik | ✅ Pass — Phase 5D.2 direct imported-YAML Apply dla profile/stats zwalidowany manualnie end-to-end (`ApplyBuildTemplateV2ToCharacterJSON` na canonical JSON wyprodukowanym przez `PreviewBuildTemplateImportYAMLFromFile`, `mode: "append"`); brak drugiego file dialogu, brak TOCTOU re-read między preview a apply; importy v1 zachowały legacy zachowanie wyłącznie Save-to-Library; importy v2 z niewspieranymi sekcjami zachowały disabled Apply. |

| Pole | Wartość |
|---|---|
| Data walidacji | 2026-05-31 |
| Branch pod testem | `feature/templates-v2-apply-overrides` |
| Wynik | ✅ Pass — Phase 6 apply-time overrides dla profile/stats zwalidowane manualnie end-to-end na obu powierzchniach (direct YAML import preview + library "Apply with overrides…"). Edytowane wartości wylądowały na wybranej postaci; pozostałe pola nietknięte; szybka library Apply path (`ApplyBuildTemplateV2FromLibraryToCharacter`) pozostała bez zmian; importy v1 nigdy nie pokazały przycisku overrides; importy v2 z niewspieranymi sekcjami zachowały oba v2 przyciski disabled z tooltipem o wspieranym zakresie; cancel modala overrides porzucał edits bez mutacji save; `Save to Library` nadal zapisywał oryginalny canonical JSON, ignorując edits w modalu. |

| Pole | Wartość |
|---|---|
| Data walidacji | 2026-05-31 |
| Branch pod testem | `feature/templates-v2-url-import` |
| Wynik | ✅ Pass — Phase 9 URL import zwalidowany manualnie end-to-end na publicznym endpoincie `https://` serwującym v2 YAML. Preview URL pojawił się w tym samym `ImportTemplatePreviewModal` co file import; Save to Library zapisał payload URL jako wpis biblioteki, którego canonical JSON pasuje do tego co preview wyświetlił (bez dodanej metadata `sourceURL`); Apply to character zapisał wybrane pola przez `ApplyBuildTemplateV2ToCharacterJSON` z Phase 5D.2; Apply with overrides routował edytowany canonical JSON przez ten sam endpoint. Guardy SSRF odrzuciły kolejno: URL `http://` (`url_disallowed_scheme`), literał `127.0.0.1` (`url_forbidden_ip`), literał `169.254.169.254` (`url_forbidden_ip`), pusty URL (`url_empty`), URL whitespace (`url_empty` po trim) — każdy z poprawnym tagiem `IssueCode*` widocznym inline w modalu URL bez utraty inputu użytkownika. |

| Pole | Wartość |
|---|---|
| Data walidacji | 2026-05-31 |
| Branch pod testem | `feature/templates-v1-weapon-level-override` |
| Wynik | ✅ Pass — Phase 6b weapon level override dla ścieżki Apply v1 `inventory.workspace` zwalidowany manualnie end-to-end na prawdziwym save. Zaaplikowano szablon v1 z mixed standard + somber + unupgradeable broniami pod każdą kombinacją kontrolek (override disabled / wyłącznie `StandardLevel` / wyłącznie `SomberLevel` / obie / `StandardLevel = 26` clampowane do `+25` z `weapon_level_clamped` / `SomberLevel = 11` clampowane do `+10` z `weapon_level_clamped` / unupgradeable z `weapon_unupgradeable`). Override mutował workspace tylko przez `editor.UpdateWeapon`; użytkownik dalej committował przez `Save changes`; ścieżki Apply v2 Phase 5 / 5D.2 / 6 oraz ścieżka Phase 9 URL import bez wpływu; scope guard v2 `inventory.workspace` w `app_templates_v2_apply.go` nadal odrzucał v2 inventory.workspace Apply; `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal` i `TemplateLibraryModal` pozostały bez zmian na każdej powierzchni. |

| Pole | Wartość |
|---|---|
| Data walidacji | 2026-05-31 |
| Branch pod testem | `feature/templates-v2-inventory-workspace-apply` (HEAD `3e448f0`) |
| Wynik | ✅ Pass — Phase 7a Apply v2 `inventory.workspace` przez aktywną Inventory Edit Session zwalidowany manualnie end-to-end na prawdziwym save. Zaaplikowanie szablonu v2 z zaznaczonym `inventory.workspace` bez otwartego workspace Sort Order surfacowało toast "Open the Sort Order workspace before applying inventory templates." i **nie** wywołało bindingu; otwarcie workspace i ponowne zaaplikowanie wylądowało itemy w gridzie workspace; `Save changes` commitowało je do `slot.Data`; reload save'a pokazywał itemy z poprawnymi acquisition indices i bez integrity warnings. Mixed szablon profile + stats + inventory.workspace v2 zaaplikował się atomowo — pola profile/stats i itemy inventory wylądowały w jednej akcji użytkownika; dual slot+workspace snapshot rollback został wyćwiczony przez celowe wprowadzenie błędu walidacji stats po wcześniejszym zapisaniu itemów inventory — zarówno slot jak i workspace cofnęły się, żadne itemy nie wyciekły. Zarówno URL import jak i library Apply reużywały tę samą ścieżkę canonical JSON. Szablony v2 niosące niewspierane sekcje poza `{profile, stats, inventory.workspace}` zachowały oba v2 przyciski disabled z tooltipem o wspieranym zakresie. Ścieżki Phase 5 / 5D.2 / 6 v2 dla profile/stats-only, Phase 6b weapon level override na ścieżce v1 SortOrderTab i ścieżka Phase 9 URL import pozostały bez wpływu. `App.tsx`, `SortOrderTab.tsx` oraz `ApplyOverridesPanel.tsx` pozostały nietknięte. |

| Pole | Wartość |
|---|---|
| Data walidacji | 2026-05-31 |
| Branch pod testem | `feature/templates-v2-weapon-override` (HEAD `8fccd72`) |
| Wynik | ✅ Pass — Phase 7a.2 apply-time weapon level override na ścieżce Apply v2 `inventory.workspace` zwalidowany manualnie end-to-end na prawdziwym save. Fast Apply bez overrides zachował poziomy upgrade'u broni zadeklarowane w szablonie (ścieżka v2 niosła `weaponLevelOverride = undefined`). `Apply with overrides…` ze `Standard = 25` ustawił każdą standardową broń na +25 (i clampował over-cap żądania z `weapon_level_clamped`) zostawiając somber broni na ich template-side poziomach; `Somber = 10` mirrorował symetryczny przypadek dla somber/special broni; oba poziomy ustawione jednocześnie zaaplikowały się per-klasa niezależnie; włączenie panelu z obydwoma pustymi inputami wyłączyło Apply z inline elementem `apply-overrides-weapon-error`; `Standard = 26` / `Somber = 11` / wartości negatywne wyłączyły Apply z range-specific komunikatem błędu. Zamknięcie workspace Sort Order przed aplikacją surfacowało toast no-session z Phase 7a ("Open the Sort Order workspace before applying inventory templates.") — override nigdy nie sięgnął backendu niezależnie od stanu panelu. Mixed szablon profile + stats + inventory.workspace v2 z override zaaplikował się atomowo — pola profile/stats, itemy inventory oraz poziomy broni wylądowały w jednej akcji użytkownika; celowe wprowadzenie błędu walidacji stats po zapisaniu itemów inventory wyćwiczyło ponownie dual snapshot rollback z Phase 7a, żadne override-mutowane itemy nie wyciekły. Szablon profile/stats-only ze strukturalnie poprawnym override zaaplikował się bez renderowania panelu weapon i bez warningów override (silent ignore). URL import i library `Apply with overrides…` reużywały tę samą ścieżkę canonical JSON i to samo pole `weaponLevelOverride` na `ApplyTemplateV2Options`. Dropdown v1 Phase 6b w `SortOrderTab.tsx` działał byte-for-byte bez zmian — jego testidy `weapon-override-*` nie kolidowały z nowymi testidami `apply-overrides-weapon-*` na powierzchni v2. `App.tsx` i `SortOrderTab.tsx` pozostały nietknięte; `WeaponLevelOverridePanel.tsx` jest nowy; `ApplyOverridesPanel.tsx` i `TemplatesShellModal.tsx` niosą conditional-render + rozszerzone `onConfirm` plumbing. |

### 17a.2. Zwalidowany flow

Poniższy user-facing flow został przeprowadzony manualnie i potwierdzony jako działający:

1. Otworzyć globalny sidebar entry `Templates` → `Create from Character…`.
2. Wybrać postać źródłową; modal otwiera się ze sekcjami profile / stats domyślnie zwiniętymi.
3. Per-section enable + per-field toggle: włączyć `profile`, wybrać subset pól profile (np. `name`, `level`, `class`); włączyć `stats`, wybrać subset z 8 stat fields. Kanoniczny selection key dla pola klasy to `class` — `className` nie jest poprawnym selection key.
4. `Preview schema v2` renderuje v2 metadata (klucz schematu, wersja, podsumowanie selection) i rozwiązane wartości per-field z postaci źródłowej.
5. `Save to Library` zapisuje dokument v2 do lokalnej biblioteki (JSON na dysku wg §10.1) z badge'em `v2` i podsumowaniem wybranych sekcji w liście biblioteki.
6. `Export YAML from library` produkuje plik `.yaml` zawierający ten sam payload v2.
7. `Import` wyeksportowanego `.yaml` z powrotem przez Templates shell; preview pokazuje to samo v2 metadata i wybrane sekcje.
8. `Apply` dla wpisu biblioteki v2 jest enabled **wyłącznie** gdy jego `selectedSections ⊆ { profile, stats }`. Kliknięcie Apply wywołuje inline confirm bezpośrednio w wierszu biblioteki, a następnie `TemplatesShellModal` wywołuje `ApplyBuildTemplateV2FromLibraryToCharacter(charIdx, libraryEntryID, { mode: "append" })`. Po sukcesie `App.tsx` bumpuje `inventoryVersion` + `saveLoadKey` oraz uruchamia `refreshSlots` + `refreshUndoDepth`. Wpisy v2 niosące dowolną inną sekcję pozostają z disabled Apply; istniejący guard Phase 3B.0 w `app_templates.go` nadal odrzuca v1 Apply dla każdego dokumentu v2.

### 17a.2a. Flow Phase 5 zwalidowany

1. Otworzyć globalny sidebar entry `Templates` → biblioteka.
2. Wybrać wpis biblioteki v2, którego `selectedSections ⊆ { profile, stats }`. Przycisk Apply jest enabled; wpisy v2 niosące dowolną inną sekcję pozostają disabled, a wpisy v1 nadal używają niezmienionej ścieżki v1 Apply.
3. Kliknąć Apply → inline confirm pojawia się w wierszu biblioteki.
4. Confirm → `TemplatesShellModal` wywołuje `ApplyBuildTemplateV2FromLibraryToCharacter` z aktywnym `charIdx` i `mode: "append"`.
5. Backend uruchamia warstwę apply Phase 5 pod `slotMu[charIdx]` (snapshot + rollback na error), pomijając `profile.class` i raportując je w `ApplyTemplateV2Result.Skipped`.
6. `App.tsx` odświeża `inventoryVersion`, `saveLoadKey`, sloty i głębokość undo, więc widoczny stan postaci / save aktualizuje się bez reloadu.

### 17a.2b. Flow direct imported-YAML Phase 5D.2 zwalidowany

1. Otworzyć globalny sidebar entry `Templates` → `Import YAML from File…`.
2. Wybrać szablon v2 `.yaml`, którego `selectedSections ⊆ { profile, stats }`. Shell wywołuje `PreviewBuildTemplateImportYAMLFromFile`, który zwraca parsowany report **oraz** canonical JSON serializację tego samego payloadu.
3. `ImportTemplatePreviewModal` otwiera się z v2 metadata, panelem report, istniejącym przyciskiem `Save to Library` i — wyłącznie dla importów v2 — nowym przyciskiem `Apply to character` (testid `import-preview-apply-v2`).
4. Nowy przycisk jest enabled tylko gdy report jest OK, save jest załadowany, postać jest wybrana, selekcja jest niepusta, a każda wybrana sekcja jest w module-level `V2_APPLY_SUPPORTED_SECTIONS = ['profile', 'stats']`. Importy v1 pomijają prop `onApplyV2`, więc przycisk w ogóle nie jest renderowany.
5. Kliknąć Apply → `TemplatesShellModal.handleApplyV2FromImportedPreview` wywołuje `ApplyBuildTemplateV2ToCharacterJSON(charIndex, importedPreview.canonicalJSON, { mode: "append" })`. Bajty, które są aplikowane, to byte-for-byte canonical JSON, który użytkownik widział w preview — brak drugiego file dialogu i brak ponownego odczytu YAML z dysku.
6. Na `result.applied === true` preview zamyka się, success toast podaje ścieżkę YAML i nazwę slotu, `onCharacterTemplateApplied(charIndex)` odpala (więc `App.tsx` uruchamia istniejący post-Phase-5D.1 refresh dance — `inventoryVersion`, `saveLoadKey`, sloty, undo), a info toast ogłasza pominięcie `profile.class`, jeśli `class` pojawiło się w `result.skippedFields`.
7. Na `result.applied === false` lub thrown binding error podnoszony jest error toast i preview pozostaje otwarty, by użytkownik mógł spróbować ponownie lub zamknąć go manualnie.
8. Istniejąca akcja `Save to Library` pozostaje nienaruszona; kliknięcie jej na tym samym preview zapisuje zaimportowany szablon do biblioteki jak wcześniej, a ścieżka library Apply z Phase 5D.1 pozostaje source of truth dla wpisów już zapisanych lokalnie.

### 17a.2c. Flow apply-time overrides Phase 6 zwalidowany

1. Otworzyć globalny sidebar entry `Templates`.
2. **Ścieżka direct YAML** — kliknąć `Import YAML from File…`, wybrać v2 `.yaml`, którego `selectedSections ⊆ { profile, stats }`. Preview shell wywołuje `PreviewBuildTemplateImportYAMLFromFile`, który zwraca ten sam canonical JSON, który Phase 5D.2 już konsumuje. Modal preview renderuje v2 metadata, istniejący przycisk `Save to Library`, istniejący przycisk `Apply to character` (Phase 5D.2) i nowy przycisk `Apply with overrides…` (Phase 6, testid `import-preview-apply-v2-overrides`).
3. Kliknąć `Apply with overrides…` → `TemplatesShellModal` zapisuje `OverridesSource` kind `'import'` (z labelką ścieżki YAML) i otwiera `ApplyOverridesModal` z canonical JSON z preview.
4. `ApplyOverridesPanel` parsuje JSON, renderuje edytowalne wiersze dla ośmiu nadpisywalnych pól profile i ośmiu stats, renderuje `profile.class` jako read-only z hintem "Skipped on apply (Phase 5)" gdy jest obecne, ignoruje dowolną inną sekcję. Range-walidatuje każdy keystroke; emituje zmutowany canonical JSON gdy draft się zmienia.
5. Edytować wartości (np. podnieść `profile.level` z 50 do 55, podnieść `stats.vigor` z 25 do 30, podnieść `profile.scadutreeBlessing` z 0 do 5). Włączyć wcześniej nieselektowane pole klikając checkbox przed wpisaniem.
6. Kliknąć `Apply to character` w modal overrides → `TemplatesShellModal.handleConfirmOverrides` posyła zmutowany JSON przez `ApplyBuildTemplateV2ToCharacterJSON(charIdx, mutatedCanonicalJSON, { mode: "append" })`. Library `…FromLibraryToCharacter` endpoint nie jest wywoływany z tej powierzchni.
7. Na `result.applied === true` oba modale zamykają się, success toast nazywa source label i slot, `onCharacterTemplateApplied(charIndex)` odpala (więc `App.tsx` uruchamia istniejący post-Phase-5D.1 refresh dance), a info toast ogłasza pominięcie `profile.class`, jeśli szablon niósł `class`.
8. Na `result.applied === false` lub thrown binding error modal overrides pozostaje otwarty i podnoszony jest error toast, by użytkownik mógł poprawić wartości i ponowić.
9. **Ścieżka library** — kliknąć `Apply with overrides…` (testid `library-apply-overrides`) na v2 wierszu biblioteki, którego `selectedSections ⊆ { profile, stats }`. `TemplatesShellModal.handleOpenOverridesFromLibrary` wywołuje `PreviewBuildTemplateFromLibrary(entry.id)` by pobrać canonical JSON i report, następnie otwiera ten sam `ApplyOverridesModal` z `OverridesSource` kind `'library'` (z labelką entry). Kroki 4–8 powyżej obowiązują identycznie.
10. Istniejący szybki library Apply przez `ApplyBuildTemplateV2FromLibraryToCharacter` pozostaje click targetem oryginalnego przycisku `Apply` na tym samym wierszu, z nietkniętym inline confirm row.
11. Importy v1 i wpisy v1 biblioteki nigdy nie renderują przycisku overrides. Importy / wpisy v2 niosące dowolną niewspieraną sekcję zachowują oba v2 przyciski disabled z istniejącym tooltipem "profile / stats only in this phase".

### 17a.2d. Flow URL import Phase 9 zwalidowany

1. Otworzyć globalny sidebar entry `Templates` → `Import from URL…` (testid `templates-shell-import-url`).
2. Wkleić publiczny URL `https://` serwujący v2 YAML i kliknąć `Preview` (lub nacisnąć Enter). `TemplatesShellModal.onURLImportPreview(rawURL)` wywołuje `PreviewBuildTemplateImportYAMLFromURL(rawURL)`, który trimuje whitespace, deleguje do `templates.FetchYAMLFromURL`, a po sukcesie oddaje bajty temu samemu `previewYAMLPayload`, którego używa ścieżka file-import.
3. Na `report.ok = true` modal URL zamyka się, shell ustawia `importedPreview = { report, canonicalJSON: bundle.json, path: bundle.path ?? rawURL }`, a istniejący `ImportTemplatePreviewModal` otwiera się z URL jako labelką source. Modal renderuje się identycznie jak preview v2 file-import — to samo v2 metadata, to samo `Save to Library`, to samo `Apply to character` (Phase 5D.2), to samo `Apply with overrides…` (Phase 6).
4. Na `report.ok = false` modal URL pozostaje otwarty z inline elementem `import-url-error` renderującym wiadomość `IssueCode*` z backendu (np. `url_forbidden_ip: 127.0.0.1 is not allowed.`), wartość inputu jest zachowana by użytkownik mógł edytować i ponowić, modal preview nie otwiera się.
5. `Save to Library` na URL-imported preview reużywa istniejącej ścieżki file-import: persystowany wpis biblioteki jest byte-for-byte tym samym canonical JSON, który preview wyświetlił; metadata `sourceURL` nie jest zapisywana w bibliotece w tej fazie.
6. `Apply to character` na URL-imported preview reużywa `ApplyBuildTemplateV2ToCharacterJSON(charIdx, canonicalJSON, { mode: "append" })` z Phase 5D.2; brak drugiego fetch, brak drugiego URL hit, brak TOCTOU re-read.
7. `Apply with overrides…` na URL-imported preview reużywa `ApplyOverridesModal` z Phase 6 end-to-end; zmutowany canonical JSON jest posyłany przez ten sam endpoint Phase 5D.2.
8. Importy URL v1 pomijają propy `onApplyV2` / `onApplyV2WithOverrides` na poziomie modala, więc żaden z v2 przycisków nie jest renderowany. Importy URL v2 niosące dowolną niewspieraną sekcję zachowują oba v2 przyciski disabled z istniejącym tooltipem "profile / stats only in this phase".
9. Guardy SSRF działają zanim otworzy się jakikolwiek preview: schematy `http://`, schematy `data:`, literały loopback IP, literały IP cloud-metadata, DNS-resolved private IPs, oversized bodies, niedozwolone Content-Types, timeouty, błędy TLS i redirecty na forbidden destinations — wszystkie odrzucane z precyzyjnym tagiem `IssueCodeURL*` i surfacowane inline. Sam fetch nigdy nie modyfikuje biblioteki ani save.

### 17a.2e. Flow Phase 6b weapon level override zwalidowany

1. Otworzyć tab Sort Order; załadować save i wybrać slot; otworzyć dropdown Templates (istniejąca akcja `Apply Template ▾` — `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal` i `TemplateLibraryModal` pozostają nietknięte na tej powierzchni w Phase 6b).
2. Nowy `weapon-override-panel` (testid) siedzi inline wewnątrz dropdownu Templates. Master checkbox `weapon-override-enabled` jest domyślnie odznaczony; oba inputy `weapon-override-standard` (range `0..25`) i `weapon-override-somber` (range `0..10`) są ukryte. Z masterem off Apply zachowuje się byte-for-byte jak ścieżka sprzed Phase 6b — `WeaponLevelOverride = undefined` trafia do backendu.
3. Włączenie mastera odsłania oba inputy number. Oba puste + master on surfacuje inline element `weapon-override-error` i wyłącza Apply (pasuje do `validateWeaponLevelOverride` odrzucającego `Enabled = true` z oboma pointerami nil).
4. Wypełnienie wyłącznie `weapon-override-standard` wartością `+25` i kliknięcie `Apply` buduje `{ enabled: true, standardLevel: 25 }`. Shell wywołuje backend Apply v1 inventory.workspace, który uruchamia `applyTemplateItemsToWorkspace`; per dodaną broń `editor.AddItem` populuje `editor.EditableItem.MaxUpgrade` z `db.GetItemDataFuzzy`. Po każdym template-side patchu `editor.UpdateWeapon` (Upgrade / Infusion / AoW z szablonu) uruchamia się `applyWeaponLevelOverride`: bronie standard (`MaxUpgrade == 25`) dostają ponownie `editor.UpdateWeapon{Upgrade: &25}`; bronie somber (`MaxUpgrade == 10`) zachowują swój template-side level; unupgradeable (`MaxUpgrade == 0`) są silent skip, bo `SomberLevel` jest nil. Żadnych warningów w tym przypadku.
5. Wypełnienie wyłącznie `weapon-override-somber` wartością `+10` i kliknięcie `Apply` buduje `{ enabled: true, somberLevel: 10 }`. Somber bronie lądują na `+10`; bronie standard zachowują swój template-side level; unupgradeable są silent skip.
6. Wypełnienie obu na maksimum dla ich klasy — `25` standard + `10` somber — aplikuje każdą klasę niezależnie w tym samym apply.
7. Żądanie `+26` standard lub `+11` somber: helper liczy `clamped := editor.ClampUpgrade(req, MaxUpgrade)` (`+25` i `+10`), re-aplikuje przez `editor.UpdateWeapon{Upgrade: &clamped}` i dopisuje `templates.IssueCodeWeaponLevelClamped` ("weapon_level_clamped") do `report.Warnings`. Frontendowy filtr toastów surfacuje hint warningu override bez blokowania apply.
8. Dodanie broni unupgradeable (`MaxUpgrade == 0`, np. Unarmed) emituje `templates.IssueCodeWeaponUnupgradeable` ("weapon_unupgradeable") do `report.Warnings` i pomija override dla tego wpisu. Stan template-side jest zachowany.
9. Ścieżka override nigdy nie dodaje, nie usuwa ani nie relokuje przedmiotów; nigdy nie dotyka `Infusion` ani `AoW`; mutacja zostaje w pełni wewnątrz aktywnego `InventoryWorkspaceSnapshot`. `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu. Użytkownik dalej klika `Save changes`, by zapisać do `slot.Data`.
10. Ścieżki Apply v2 Phase 5 / 5D.2 / 6 oraz Phase 9 URL import nie udostępniają panelu override (ich powierzchnia UI to `TemplatesShellModal`, nie `SortOrderTab.tsx`); ich zachowanie jest byte-for-byte bez zmian. Scope guard v2 `inventory.workspace` w `app_templates_v2_apply.go` nadal odrzuca v2 inventory.workspace Apply.

### 17a.2f. Flow Phase 7a Apply v2 inventory.workspace zwalidowany

1. Otworzyć tab Sort Order na docelowej postaci, aby `InventoryEditSession` została acquired, a jej `sessionID` zarejestrowany w `editSessionByChar[charIdx]`. Bez tego kroku każdy Apply v2 dla szablonu niosącego `inventory.workspace` zostanie odrzucony upstream z nowym tostem "Open the Sort Order workspace before applying inventory templates.".
2. Otworzyć globalny sidebar entry `Templates`. Wybrać jedną z trzech powierzchni — Library, `Import YAML from File…` lub `Import from URL…`.
3. Dla dowolnego szablonu v2, którego `selectedSections` zawiera `'inventory.workspace'`, `TemplatesShellModal` wywołuje `GetActiveInventoryEditSessionForCharacter(charIndex)` przed bindingiem apply. Endpoint czyta `editSessionsMu` i zwraca `{ active: true, sessionID }` dla aktywnej sesji, `{ active: false }` dla niepoprawnego `charIdx` lub braku aktywnej sesji — nigdy nie erroruje.
4. Brak aktywnej sesji → czerwony toast surfacuje `NO_SESSION_MESSAGE` ("Open the Sort Order workspace before applying inventory templates.") i binding **nie** jest wywoływany. Brak mutacji backendowej. Wiersz biblioteki / przycisk preview pozostaje klikalny, aby użytkownik mógł ponowić po otwarciu workspace.
5. Aktywna sesja → shell przekazuje `{ mode: "append", sessionID }` do `ApplyBuildTemplateV2ToCharacterJSON(charIdx, canonicalJSON, opts)` dla powierzchni direct YAML / URL / overrides lub do `ApplyBuildTemplateV2FromLibraryToCharacter(charIdx, entry.id, opts)` dla powierzchni library Apply (ścieżka library używa tego samego structu `ApplyTemplateV2Options`).
6. Backend klasyfikuje sparsowany payload do `hasProfile`, `hasStats`, `hasInventory`. Z `hasInventory == true` wywołuje `acquireSession(opts.SessionID)`; pusty `SessionID` → `IssueCodeInventorySessionRequired`; nieznane session id lub sesja związana z inną postacią → `IssueCodeInventorySessionInvalid` (a sesja jest odblokowywana na gałęzi wrong-character przed zwrotem). Na sukces `defer sess.Unlock()` trzyma sesję przez czas trwania apply.
7. Apply uruchamia preflight pojemności na **istniejącym** `sess.Workspace` przed jakąkolwiek mutacją. Na błędzie preflightu apply zwraca `Applied = false`, a workspace pozostaje bez zmian.
8. Pobierane są snapshoty slot + workspace: `slotBackup := core.SnapshotSlot(slot)` i `workspaceBackup := deepCopySnapshot(sess.Workspace)`. Od tego momentu każde wyjście błędem wywołuje `rollbackBoth()`, więc partial mixed apply nie może wyciec.
9. Itemy inventory są aplikowane przez `applyTemplateItemsToWorkspace(&sess.Workspace, sec.InventoryItems, editor.ContainerInventory, nil)`; storage items przez storage equivalent z tym samym pinem `nil` dla override. `nil` jest celowe i dokumentuje, że weapon level override Phase 6b **nie** jest podłączony do ścieżki v2 w tej fazie — Phase 6b pozostaje feature'em dropdownu v1 `SortOrderTab.tsx`.
10. Gdy itemy inventory lądują, `sess.Workspace.Dirty = true` i snapshot jest rewalidowany. Sentinel `"inventory.workspace"` jest dopisany do `result.Applied`, by downstream consumers mogli wykryć, że sekcja uczestniczyła w apply. `result.InventoryItemsApplied` i `result.StorageItemsApplied` niosą per-container counts; `result.Workspace` niesie post-apply snapshot do refresh UI.
11. Zapisy profile / stats lecą **po** gałęzi inventory w tym samym oknie `slotMu[charIdx]`, reużywając ścieżkę Phase 5 (`vm.ApplyVMToParsedSlot` + `slot.SyncPlayerToData`) byte-for-byte. Jeśli zawiodą po tym, jak itemy inventory wylądowały już w `sess.Workspace`, `rollbackBoth()` przywraca obie — atomowość zachowana.
12. Szablony v2, których `selectedSections` to wyłącznie `{profile, stats}`, nadal aplikują się z `sessionID = ''` dokładnie jak przed Phase 7a; `acquireSession` **nie** jest wywoływany na tej ścieżce, a istniejący Phase 5 edit-session conflict guard (odmawiający apply gdy sesja jest trzymana dla innego open edit) jest zachowany bez zmian.
13. Użytkownik nadal commituje workspace do `slot.Data` przez istniejący przycisk `Save changes` w `SortOrderTab.tsx` — `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu. Sam Apply v2 inventory nigdy nie pisze do `slot.Data`.

### 17a.2g. Flow Phase 7a.2 v2 weapon level override zwalidowany

1. Otworzyć tab Sort Order na docelowej postaci, by `InventoryEditSession` zostało nabyte i kontrakt session-gating z Phase 7a był spełniony. Bez aktywnej sesji każdy Apply v2 dla szablonu niosącego `inventory.workspace` jest nadal odrzucany upstream — override z Phase 7a.2 nigdy nie sięga backendu w tym przypadku.
2. Otworzyć globalny sidebar entry `Templates`. Wybrać jedną z trzech powierzchni — Library, `Import YAML from File…` lub `Import from URL…` — i wybrać szablon v2, którego `selectedSections` zawiera `'inventory.workspace'` (z lub bez `profile` / `stats`).
3. Kliknąć `Apply with overrides…` (testid `library-apply-overrides`, `import-preview-apply-v2-overrides` lub odpowiednik URL). `TemplatesShellModal` otwiera `ApplyOverridesModal` z canonical JSON z preview dokładnie tak, jak już robiły Phase 6 / Phase 7a.
4. Modal teraz parsuje `selection['inventory.workspace']` z canonical JSON. Gdy obecne (boolean `true` lub obiekt z `all: true` / niepustymi polami), modal renderuje nowy `WeaponLevelOverridePanel` (testid `apply-overrides-weapon-panel`) pod istniejącym gridem profile/stats. Szablony profile/stats-only zostawiają panel nierenderowany; szablony inventory-only mogą używać modalu wyłącznie dla weapon level override (grid profile/stats renderuje puste nagłówki, bez pól).
5. Panel weapon jest domyślnie disabled z oboma inputami ukrytymi. Toggle `apply-overrides-weapon-enabled` ujawnia `apply-overrides-weapon-standard` (zakres `0..25`) i `apply-overrides-weapon-somber` (zakres `0..10`) plus inline element `apply-overrides-weapon-error` (renderowany tylko gdy panel jest w stanie invalid). Oba inputy są puste przy pierwszym ujawnieniu — użytkownik musi explicitly wpisać przynajmniej jeden poziom, by override był actionable.
6. Panel emituje swój stan przez `onChange(override, hasInvalid)`. `override` to `{ enabled: true, standardLevel?: number, somberLevel?: number } | undefined` — disabled master toggle, lub enabled master bez wpisanych inputów, emituje `undefined`; enabled master z jednym lub oboma inputami wypełnionymi w-zakres wartościami emituje strukturalnie poprawny payload. `hasInvalid` jest `true` gdy master jest enabled i (a) oba inputy są puste, (b) `standardLevel` jest `< 0` / `> 25`, lub (c) `somberLevel` jest `< 0` / `> 10`.
7. `ApplyOverridesModal` blokuje Apply (`canApply = false`) zawsze gdy któryś z paneli profile/stats lub weapon raportuje invalid input. Status pill przełącza się między trzema komunikatami: `"Ready to apply."`, `"N field(s) need attention."` (profile/stats invalid) oraz `"Fix weapon level override to apply."` (panel weapon invalid).
8. Kliknięcie Apply wywołuje `onConfirm(mutatedJSON, weaponOverride)` z aktualną emisją panelu. `TemplatesShellModal.handleConfirmOverrides` parsuje mutated JSON, uruchamia session check z Phase 7a (`canonicalJSONNeedsSession` + `fetchActiveSessionID`) i postuje przez `ApplyBuildTemplateV2ToCharacterJSON(charIdx, mutatedJSON, main.ApplyTemplateV2Options.createFrom({ mode: "append", sessionID, weaponLevelOverride }))`. Fast library Apply (`ApplyBuildTemplateV2FromLibraryToCharacter`) **nie** jest wywoływany z tej powierzchni i nigdy nie niesie override.
9. Backend waliduje override **przed** `acquireSession` przez `validateWeaponLevelOverride(opts.WeaponLevelOverride)`. Strukturalnie broken override → `Applied = false`, `Preview.Errors = [{ code: structure_invalid, … }]`, zero side effects. Strukturalnie poprawny override na szablonie profile/stats-only (`hasInventory == false`) jest silently ignored — pętla override nie ma na czym operować. Dla `hasInventory == true` dwa wywołania `applyTemplateItemsToWorkspace(&sess.Workspace, …, opts.WeaponLevelOverride)` przepuszczają override przez istniejący helper; `applyWeaponLevelOverride` uruchamia się po każdym template-side patchu broni i emituje warningi `weapon_level_clamped` / `weapon_unupgradeable` do `report.Warnings` przez istniejącą agregację `invWarn` / `stoWarn`. Warningi nigdy nie rollbackują.
10. Na `Applied = true` oba modale zamykają się, success toast nazywa source label i slot, `onCharacterTemplateApplied(charIndex)` odpala (więc `App.tsx` uruchamia swój istniejący post-Phase-5D.1 refresh dance), a info toast ogłasza skip `profile.class` gdy szablon niósł `class`. Użytkownik nadal commituje workspace do `slot.Data` przez istniejący przycisk `Save changes` w `SortOrderTab.tsx` — `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu. Sam Apply v2 inventory nigdy nie pisze do `slot.Data` ze ścieżki override.
11. Dropdown v1 Phase 6b w `SortOrderTab.tsx` jest byte-for-byte bez zmian. Jego testidy `weapon-override-*` nie kolidują z testidami `apply-overrides-weapon-*` na powierzchni v2 w `ApplyOverridesModal`, więc testy dropdownu v1 i testy modalu v2 mogą koegzystować w tym samym Vitest runie bez interferencji. Ścieżka apply v1 (`ApplyBuildTemplateToWorkspaceJSON` z `ApplyTemplateOptions.WeaponLevelOverride`) jest bez zmian.

### 17a.3. Scope tego, co **nie** jest jeszcze zwalidowane

- ✅ **Dostarczone 2026-05-31 (Phase 5D.1)** — Apply `sections.profile` i `sections.stats` do prawdziwego save przez `ApplyBuildTemplateV2FromLibraryToCharacter` (ścieżka biblioteki, `mode: "append"`, `profile.class` celowo pomijane, snapshot + rollback pod `slotMu[charIdx]`).
- ✅ **Dostarczone 2026-05-31 (Phase 5D.2)** — Direct apply zaimportowanego YAML bez wcześniejszego Save to Library, przez `ApplyBuildTemplateV2ToCharacterJSON` na canonical JSON wyprodukowanym przez `PreviewBuildTemplateImportYAMLFromFile` (`Import YAML → Preview → Apply to character`, `mode: "append"`, brak drugiego file dialogu, brak TOCTOU re-read). `ApplyBuildTemplateV2FromFileToCharacter` nadal istnieje backend/bindings-side, ale celowo pozostaje niepodpięty w UI — ścieżka JSON jest preferowana, bo jest WYSIWYG z preview, który użytkownik właśnie potwierdził.
- ✅ **Dostarczone 2026-05-31 (Phase 6)** — Apply-time overrides dla tego samego subsetu profile/stats na obu powierzchniach, przez frontend-only mutację canonical JSON przekazaną do `ApplyBuildTemplateV2ToCharacterJSON`. Brak zmian backendu, bindings, `App.tsx`. `profile.class` pozostaje read-only. v1 szablony i niewspierane v2 sekcje pozostają zablokowane. Weapon level override przy apply pozostawało odroczone do Phase 6b w momencie shippingu Phase 6 — a teraz jest dostarczone, zob. poniżej.
- ✅ **Dostarczone 2026-05-31 (Phase 6b)** — Apply-time weapon level override dla istniejącej ścieżki Apply v1 `inventory.workspace`. Runtime opcja `WeaponLevelOverride { Enabled bool, StandardLevel *int, SomberLevel *int }` dodana do `app_templates.go::ApplyTemplateOptions`. Override uruchamia się **po** każdym template-side patchu `editor.UpdateWeapon` przez nowy helper `applyWeaponLevelOverride`, który switchuje na `editor.EditableItem.MaxUpgrade` (25 = standard / 10 = somber / 0 = unupgradeable / inaczej silent skip), clampuje over-cap żądania przez `editor.ClampUpgrade(req, MaxUpgrade)` i surfacuje `templates.IssueCodeWeaponLevelClamped` ("weapon_level_clamped") lub `templates.IssueCodeWeaponUnupgradeable` ("weapon_unupgradeable") na `report.Warnings`. Mutacja zostaje wewnątrz aktywnego `InventoryWorkspaceSnapshot`; z samej ścieżki override żadne bajty nie trafiają do `slot.Data`; `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu. Kontrolki UI żyją wewnątrz istniejącego dropdownu Templates w `SortOrderTab.tsx`; `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, `TemplateLibraryModal`, `App.tsx` oraz warstwa apply v2 są bez zmian. Bez zmiany schematu v2, bez writera equipment, bez direct save mutation przez `PatchWeaponItemID` ze ścieżki template apply.
- ✅ **Dostarczone 2026-05-31 (Phase 9)** — Import szablonów z `https://` URL przez `PreviewBuildTemplateImportYAMLFromURL`, oparte o `backend/templates/url_import.go::FetchYAMLFromURL` pod pełną listą guardów §12.3 (SSRF). Preview URL reużywa istniejącego `ImportTemplatePreviewModal`, więc Save to Library / Apply to character / Apply with overrides ship bez zmian na powierzchni URL. Brak zmiany schemy biblioteki (metadata `sourceURL` nie jest dodawana); brak zmiany `App.tsx`. Authenticated downloads, domain allowlist, URL auto-refresh oraz direct apply bez preview pozostają poza scope.
- ✅ **Dostarczone 2026-05-31 (Phase 7a)** — Pierwsza realna ścieżka Apply v2 dla `inventory.workspace` przerzucona przez **aktywną `InventoryEditSession` / `InventoryWorkspaceSnapshot`**. Nowy backend endpoint `App.GetActiveInventoryEditSessionForCharacter(charIdx) → { active, sessionID }`; addytywne `ApplyTemplateV2Options.SessionID string`; nowe issue codes `IssueCodeInventorySessionRequired = "inventory_session_required"` oraz `IssueCodeInventorySessionInvalid = "inventory_session_invalid"`; rozszerzony `ApplyTemplateV2Result` o `InventoryItemsApplied`, `StorageItemsApplied`, opcjonalny `Workspace *editor.InventoryWorkspaceSnapshot`. Mixed apply profile+stats+inventory.workspace jest atomowy przez dual snapshot rollback (`core.SnapshotSlot` + value-type workspace deep copy, przywracane przez pojedynczy closure `rollbackBoth()` na każdym error exit). Apply v2 inventory nigdy nie dotyka `slot.Data` bezpośrednio — `SaveInventoryWorkspaceChanges` pozostaje jedynym punktem commitu. `TemplatesShellModal` wyszukuje aktywną sesję dla powierzchni library / direct YAML / URL / overrides; profile/stats-only apply v2 nadal lecą z `sessionID = ''`. Weapon level override Phase 6b był celowo **niepodłączony** do ścieżki v2 w Phase 7a — apply v2 inventory przekazywał hard-coded `nil` override do `applyTemplateItemsToWorkspace` — i został podłączony w Phase 7a.2 poniżej.
- ✅ **Dostarczone 2026-05-31 (Phase 7a.2)** — Apply-time weapon level override podłączony do ścieżki Apply v2 `inventory.workspace` z Phase 7a. `ApplyTemplateV2Options` dostaje addytywne pole `WeaponLevelOverride *WeaponLevelOverride`, które reużywa v1 typ `WeaponLevelOverride { Enabled bool, StandardLevel *int, SomberLevel *int }` oraz walidator v1 `validateWeaponLevelOverride` 1:1 (więc bindings wystawiają pojedynczą klasę `WeaponLevelOverride` współdzieloną między powierzchniami v1 i v2). Dwa hard-coded piny `nil` override w wywołaniach `applyTemplateItemsToWorkspace` dla inventory + storage w `app_templates_v2_apply.go` są zastąpione przez `opts.WeaponLevelOverride`; sam helper, `applyWeaponLevelOverride` oraz dual snapshot rollback z Phase 7a są bez zmian. Walidacja odpala się **przed** `acquireSession`, więc strukturalnie broken override odbija się z `templates.IssueCodeStructureInvalid` i zerowymi side effects; strukturalnie poprawny override na szablonie profile/stats-only jest silently ignored (brak items → no-op). Warningi `weapon_level_clamped` i `weapon_unupgradeable` lecą do `ApplyTemplateV2Result.Preview.Warnings` przez istniejącą agregację `invWarn` / `stoWarn`. UI: nowy `WeaponLevelOverridePanel` (testidy `apply-overrides-weapon-{panel,enabled,standard,somber,error}`) jest osadzony wewnątrz istniejącego `ApplyOverridesModal` i renderowany tylko gdy `selection.inventory.workspace` jest obecne; szablony profile/stats-only zostawiają panel weapon nierenderowany; szablony inventory-only mogą używać modalu wyłącznie dla weapon level override. `ApplyOverridesModal.onConfirm` jest rozszerzony do `(mutatedJSON, weaponOverride?) ⇒ …`, a runtime option podróżuje wyłącznie przez ten argument — nigdy w canonical JSON. `TemplatesShellModal.handleConfirmOverrides` przekazuje `weaponLevelOverride` do `main.ApplyTemplateV2Options.createFrom({ mode, sessionID, weaponLevelOverride })`. Fast library Apply (`ApplyBuildTemplateV2FromLibraryToCharacter`) nadal nie wysyła override; dropdown v1 Phase 6b w `SortOrderTab.tsx` jest bez zmian. Bez nowej sekcji schemy v2, bez writera equipment, bez direct save mutation, bez zmian `App.tsx`.
- Nowe write paths dla `sections.equipment`, `sections.equippedTalismans`, `sections.spells` — gated przez Phase 7b / 7c / 7d.
- Apply appearance preset przez Templates surface — gated przez Phase 8 (underlying writer `app_appearance.go::ApplyPresetToCharacter` już istnieje, ale brak warstwy apply, która routuje szablon przez niego).
- Multi-character pack flow — gated przez Phase 10.

Praca Phase 7b+ pozostaje wyłącznie design w tym dokumencie. Każda faza wymaga osobnej akceptacji użytkownika przed implementacją zgodnie z workflow z `~/.claude/CLAUDE.md`.

---

## 18. Open decisions intentionally deferred

Następujące decyzje są celowo nierozwiązane przez ten dokument. Każda wymaga osobnej, explicit akceptacji użytkownika zanim odpowiednia faza się rozpocznie.

1. **Wybór biblioteki YAML** (prawdopodobnie `gopkg.in/yaml.v3` dekodowany strictly do typed structs).
2. **Strategia source-of-truth across JSON + YAML** (pojedynczy Go struct z oboma tagami `json:` i `yaml:` vs osobne DTOs).
3. **Plumbing `sessionID` dla sidebar surface** (lift do App.tsx, lżejszy session-less library modal, lub context).
4. **Finalna lista kluczy sekcji v2 i ich kanoniczne nazwy** (np. `sections.profile` vs `sections.character.profile`).
5. ~~**Dokładny body-size cap dla URL import**~~ — **zdecydowane 2026-05-31 przy implementacji Phase 9: 1 MiB (`1 << 20`)**, eksportowane jako `templates.URLImportMaxBodyBytes`.
6. ~~**Dokładne request/idle timeouts dla URL import**~~ — **zdecydowane 2026-05-31 przy implementacji Phase 9: total 10 s, idle / TLS / header / dial 5 s każdy**, eksportowane jako `templates.URLImportTotalTimeout` i `templates.URLImportIdleTimeout`.
7. **Granularność `selection` dla per-spell / per-talisman lists** (boolean shortcut vs explicit list).
8. **Default policy equipment referential integrity** (warn-and-skip vs opt-in auto-add).
9. **Polityka clampingu talisman slot count** (refuse jeśli Pouch upgrade insufficient vs clamp + warning).
10. **Disposition istniejącego dropdownu `Export Template ▾`** po dostarczeniu sidebar surface (retain / remove / redirect).
11. **Tryby `replace-*` dla v2** — poza scope pierwszej iteracji; czy dostarczyć w późniejszej fazie to osobna decyzja.
12. **Auto-rebuild snapshotu `_index.bak.json`** przed `RebuildIndex` — opcjonalne późniejsze hardening.
13. **PvP / progression named modules w szablonach** — czy kiedykolwiek dostarczyć i które moduły dołączyć.
14. **Konwencje UI mapowania multi-character pack** — pełny design odroczony do własnej fazy.
15. **Czy pola wyłącznie v2 wymagają minimum gate `appVersion`** (np. otagować sekcję minimum app version which supports it).
16. **Polityka testów parity PS4 ↔ PC** dla nowych faz apply (proponowane: każda faza dotykająca kodu musi trzymać oba round-trip testy green).

---

## 19. Cross-references

- [55-build-template](55-build-template.md) — wdrożony baseline (schemat v1, eksporter, preview, apply, biblioteka).
- [54-ash-of-war](54-ash-of-war.md) — sentinele AoW, fail-closed compat, write paths.
- [37-character-presets](37-character-presets.md) — osobny mechanizm character-stat-focused; nie to samo co Templates.
- [03-gaitem-map](03-gaitem-map.md) — model GaItem; semantyka handle wykluczona z publicznego YAML.
- [06-equipment](06-equipment.md) — model equipment slot; read-only Equipment write API dzisiaj.
- [07-inventory](07-inventory.md), [10-storage](10-storage.md) — model sekcji inventory / storage.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — invarianty allocatora istotne dla apply.
- [43-transactional-item-adding](43-transactional-item-adding.md) — Add Items pipeline, wzorzec pre-flight + snapshot + post-mutation validation reużyty przez `TemplateApplyPlan`.
- [50-item-companion-flags](50-item-companion-flags.md) — niejawny POST-FLAGS contract zachowany przez apply.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — dlaczego absolute acquisition indices pozostają poza szablonami.
- [48-pvp-ready-modular-presets](48-pvp-ready-modular-presets.md) — named-module wzorzec dla wszelkich przyszłych progression effects w szablonach.

---

## 20. Sources

- Istniejący doc baseline: `spec/55-build-template.md`.
- Istniejący kod (informacyjnie, brak zmiany w tej turze): `backend/templates/{schema,export,import,library}.go`, `app_templates.go`, `frontend/src/components/templates/`, `frontend/src/components/SortOrderTab.tsx`, `frontend/src/App.tsx`.
- Apply-side zależności (informacyjnie): `backend/editor/{session,workspace,add,weapon,save}.go`, `backend/core/{inventory_index_repair,save_manager,writer,backup}.go`, `app_save_integrity.go`, `app.go::SaveCharacter`, `app_appearance.go::ApplyPresetToCharacter`, `app_pvp.go::ApplyPvPPreparation`.
- DB references (informacyjnie): `backend/db/db.go::{GetItemDataFuzzy,InfuseTypes,IsAshOfWarCompatibleWithWeapon}`, `backend/db/data/{types,weapon_gem_mount,aow_compat}.go`.
- Ograniczenia workflow: `~/.claude/CLAUDE.md` (globalne), `.claude/CLAUDE.md` (projektowe).
