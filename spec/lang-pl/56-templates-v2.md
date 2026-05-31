# 56 — Templates v2 (Partially Implemented Extension)

> **Type**: Design doc
> **Status**: 🔄 Częściowo wdrożone — Phase 0..6 + Phase 9 dostarczone (addytywny schemat `version: 2`, globalny library shell Templates, publiczny YAML import/export, flow create-from-character dla profile/stats z per-field selection, Save to Library, badge v2 w bibliotece i preview, **Phase 5 = Apply z biblioteki + direct imported-YAML Apply dla profile/stats przez `ApplyBuildTemplateV2FromLibraryToCharacter` oraz `ApplyBuildTemplateV2ToCharacterJSON` na canonical JSON wyprodukowanym przez preview importu, Phase 6 = apply-time overrides dla tego samego subsetu profile/stats na obu powierzchniach przez frontend-only mutację canonical JSON przekazaną do tego samego JSON-owego endpointu, Phase 9 = import szablonów z `https://` URL przez `PreviewBuildTemplateImportYAMLFromURL` pod pełną listą guardów §12.3 (SSRF); preview URL reużywa tego samego `ImportTemplatePreviewModal`, więc Save to Library / Apply to character / Apply with overrides ship bez zmian również na ścieżce URL**). Apply dla sekcji v2 poza profile/stats pozostaje zablokowany — Phase 6b+ (weapon level override, writery equipment/talismans/spells, appearance przez preset, multi-character pack) pozostają wyłącznie design. Addytywne rozszerzenie wdrożonego podsystemu Build Template udokumentowanego w [55-build-template](55-build-template.md).
> **Scope**: Addytywne rozszerzenie istniejącego `saveforge.build-template` JSON v1 do `version: 2` — z publicznym formatem YAML do udostępniania na zewnątrz, nowym sidebar entry point `Templates`, granular selection model, sekcjami całej postaci (profile, stats, equipment, talismans, spells, appearance tylko przez preset), single-character first, weapon level override przy apply, plików `.yaml` import/export, importu z URL z pełnymi guardami bezpieczeństwa oraz późniejszą fazą multi-character pack. Dokument **nie** redefiniuje baseline'u v1 — dziedziczy go z [55-build-template](55-build-template.md).

---

## 1. Title, status and scope

| Aspekt | Wartość |
|---|---|
| Numer dokumentu | 56 |
| Typ dokumentu | Design doc — częściowo wdrożone rozszerzenie |
| Status | 🔄 Częściowo wdrożone. Phase 0..6 + Phase 9 dostarczone (Phase 5 = Apply z biblioteki + direct imported-YAML Apply wyłącznie dla subsetu profile/stats; Phase 6 = apply-time overrides dla tego samego subsetu profile/stats, frontend-only mutacja canonical JSON przekazana do istniejącego endpointu Phase 5; Phase 9 = import szablonów z `https://` URL przez `PreviewBuildTemplateImportYAMLFromURL` pod pełną listą guardów §12.3 (SSRF), reużywając tego samego `ImportTemplatePreviewModal` co ścieżka file-import, więc wszystkie trzy akcje downstream ship bez zmian na powierzchni URL); Phase 6b+ pozostają wyłącznie design. Każda kolejna faza wymaga osobnej akceptacji użytkownika zgodnie z workflow z `~/.claude/CLAUDE.md`. |
| Referencja baseline | [55-build-template](55-build-template.md) — wdrożone `version: 1`, wyłącznie JSON, wyłącznie inventory + storage, lokalna biblioteka w `$UserConfigDir/EldenRing-SaveEditor/templates/`. |
| Klucz schematu | Pozostaje `saveforge.build-template` (bez rename). Wdrożone. |
| Wersja schematu | Reader range `1 ≤ version ≤ MaxSchemaVersion (=2)`. Builder v1 nadal emituje `SchemaVersion = 1`; explicit builder v2 (`backend/templates/export_v2.go`) emituje `version: 2`. Wdrożone. |
| Zewnętrzny format publiczny | YAML (`.yaml`). JSON pozostaje dla obecnej lokalnej biblioteki i dla backward-compatible importu. Wdrożone dla payloadów v1 i dla dokumentów v2 produkowanych przez builder v2. |
| Pierwszy widoczny entry point | Niebieski przycisk `Templates` w sidebarze, bezpośrednio nad `Save as...` w `frontend/src/App.tsx` (istniejący footer `<aside>`); otwiera `TemplatesShellModal.tsx`. Wdrożone. |
| Scope postaci (pierwsza iteracja) | Pojedyncza postać. Multi-character pack odroczony do późniejszej fazy (§15). |
| URL import | **Dostarczone (Phase 9, 2026-05-31)**. Backendowy fetch przez `PreviewBuildTemplateImportYAMLFromURL` → `backend/templates/url_import.go::FetchYAMLFromURL` pod pełną listą guardów §12.3 (HTTPS-only, pre-connect IP filter dla literałów IP i każdego DNS-resolved adresu, redirect re-check ×3, body cap 1 MiB, 10 s total / 5 s idle timeouts, strict TLS z system root CAs, identifying User-Agent, Content-Type allowlist, brak auth / cookies / custom headers, strict struct-typed YAML decode reużyty bez zmian, brak auto-refresh; sam fetch nigdy nic nie mutuje). Preview URL reużywa istniejącego `ImportTemplatePreviewModal`, więc Save to Library / Apply to character / Apply with overrides ship bez zmian na powierzchni URL. **Brak zmian schemy biblioteki** — metadata `sourceURL` nie jest dodawana do biblioteki w tej fazie. |
| Zmiana kodu produkcyjnego | Phase 0..6 + Phase 9 dostarczone; późniejsze fazy pozostają wyłącznie design. Szczegóły w §17 i §17a. |

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
| `equipment` (equipped sloty: `weaponRight1/2`, `weaponLeft1/2`, `armorHead/Chest/Arms/Legs`, plus opcjonalnie `equippedGreatRune`) | późniejsza faza (Phase 7a) | nazwa slotu → item ID | **Brak publicznego write API dzisiaj** dla `ChrAsmEquipment` slotów 0..9, 12–15, 17–21 — [06-equipment §App-level write API](06-equipment.md) jest jednoznaczne ("None — equipment is read-only from the UI perspective"). Jedyny istniejący wyjątek to `EquippedGreatRune` (slot 10), już zapisywany przez `SyncPlayerToData` w `structures.go:850–852`. **Wymagany nowy kontrolowany writer** dla pozostałych slotów (encoded item-ID form, hash 7/8 dependency — zob. [06-equipment §hash](06-equipment.md)). | requires-new-writer | **nie** (poza GreatRune) |
| `equippedTalismans` (które talismany zajmują `ChrAsmEquipment` sloty 17–21) | późniejsza faza (Phase 7b) | tablica do 5 item IDs talizmanów w kolejności slotów | **Brak publicznego write API dzisiaj** — equipped talismans żyją w tym samym bloku `ChrAsmEquipment` co zbroja; są read-only razem z resztą equipment. **Wymagany nowy kontrolowany writer** (companion do Phase 7a) i musi respektować limit Pouch z `profile.talismanSlots`. Odrębne od `profile.talismanSlots` (additional slot count, który już ma writer). | requires-new-writer | nie |
| `spells` (equipped sorcery / incantation / gesture loadout — 14 spell slotów) | późniejsza faza (Phase 7c) | spell / sorcery / incantation / gesture item IDs | **Brak publicznego write API dzisiaj.** `EquippedSpells` (14 slotów) jest obecnie referowane wyłącznie przez hash-recompute (`backend/core/hash.go:150–195`, `section_hash.go:24`). **Wymagany nowy kontrolowany writer.** Odrębne od unlocked-spell inventory entries (które są częścią `inventoryWorkspace` i są już wspierane przez v1). | requires-new-writer | nie |
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
- Lokalizacja helpera: ✅ Zrobione — czysty helper clampujący żyje teraz w `backend/editor/weapon.go` jako eksportowane `editor.ClampUpgrade` (przeniesione ze starego `app.go::clampUpgrade`, zachowanie byte-for-byte bez zmiany). Planowana warstwa `TemplateApplyPlan` pod `backend/templates/` lub `backend/editor/` może go teraz importować bezpośrednio, bez wciągania package `main`. To była czysta relokacja — bez zmiany policy, bez nowej reguły clamp — i odblokowuje Phase 6b weapon-level override oraz dowolny przyszły inventory-apply pipeline z lokalizacji importowalnej z backendu. Weapon-level override przy apply nadal **nie jest** zaimplementowany — odblokowana jest tylko ścieżka importu.

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
| Equipment slot write path (`ChrAsmEquipment` sloty 0..9, 12–15, 17–21) | requires design decision + new writer | Brak istniejącego publicznego write API ([06-equipment](06-equipment.md) "App-level write API for equipment slots | ❌ None"). Wymagany nowy kontrolowany writer dla Phase 7a; respektuje hash 7/8 dependency. |
| Equipped talismans write path (`ChrAsmEquipment` sloty 17–21) | requires design decision + new writer | Tak samo jak equipment; companion do Phase 7a, planowane jako Phase 7b. Musi respektować limit Pouch z `profile.talismanSlots`. |
| Equipped spell loadout write path (`EquippedSpells` 14 slotów) | requires design decision + new writer | Brak istniejącego publicznego write API; tylko hash recompute referuje to pole dzisiaj. Phase 7c. |
| Equipment referential integrity (szablon referuje item nieobecny w inventory docelowym) | requires design decision | Default = warn + skip; opt-in `addMissingEquippedItems` odroczone (§13.7). Dotyczy Phase 7a/7b. |
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

### Phase 6b — weapon level override dla v1 inventory / storage apply

- **Cel**: dodać `weaponLevelOverride.{standard,somber}` do opcji apply i Apply Preview UI; pre-encode item IDs w warstwie planu dla broni pochodzących z szablonu.
- **Pliki (planowany scope)**: `app_templates.go` (options DTO), apply layer; frontend preview modal. **Prerequisite refactor zrobiony**: `clampUpgrade` zostało relokowane do `backend/editor/weapon.go` jako eksportowane `editor.ClampUpgrade` (zachowanie byte-for-byte zachowane, zob. §14.4); warstwa planu może go importować bezpośrednio. Weapon-level override przy apply sam w sobie nadal jest design-only — relokacja odblokowuje tylko ścieżkę importu.
- **Backend impact**: plan-layer override + relokacja helpera; istniejący writer item-add bez zmiany.
- **Frontend impact**: dwie nowe kontrolki w preview modal, obie default `Keep`.
- **Testy**: `Set Standard to +25` przeciw mixed template → somber bronie clampowane do `+10` z warningiem `upgrade_clamped_by_override`; `Keep` zachowuje template levels; non-upgradeable bronie pozostają `+0`; round-trip obie platformy.
- **Manual validation**: zaaplikować szablon z mixed weapons pod każdą kombinacją kontrolek.
- **Ryzyka**: niskie jeśli clamping jest per-weapon i reużywa relokowane helpery.
- **Out of scope**: zmiana infusion lub AoW; writery equipped-weapon.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: tak.

### Phase 7a — equipment slot writer (nowa write path)

- **Cel**: zaimplementować nowy publiczny write path dla `ChrAsmEquipment` slotów 0..9, 12–15 (bronie + zbroja), respektujący encoded item-ID form i hash 7/8 dependency udokumentowany w [06-equipment](06-equipment.md). Zaaplikować `sections.equipment` z szablonu przez ten nowy writer.
- **Pliki (planowany scope)**: nowy writer w `backend/core/` (prawdopodobnie `backend/core/equip_write.go`), wystawiony przez `backend/editor/` do planu; rozszerzenie warstwy apply; testy włącznie z hex-verified round-trip; frontend preview row dla equipment.
- **Backend impact**: nowy publiczny API dla zapisów equipment. Istniejący wyjątek `EquippedGreatRune` (już w `SyncPlayerToData`) jest zachowany.
- **Testy**: hex-identity round-trip dla no-op write; per-slot write; PC + PS4 platform parity; integrity gate interaction; default warn-and-skip gdy referowany item brakuje w inventory (§13.7).
- **Manual validation**: zaaplikować equipment do fixture character; round-trip obie platformy; in-game verification na co najmniej jednej platformie.
- **Ryzyka**: wysokie — pierwsza nowa write path do `ChrAsmEquipment`; dotyka sekcji którą wszystkie reference editors traktują jako read-only; hash 7/8 dependency musi być re-walidowana.
- **Out of scope**: equipped talismans, spell loadout, appearance.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: tak.

### Phase 7b — equipped talismans writer (nowa write path)

- **Cel**: zaimplementować writer equipped-talismans (`ChrAsmEquipment` sloty 17–21). Zaaplikować `sections.equippedTalismans` clampowane do aktualnego `profile.talismanSlots` celu (odrzuć jeśli długość przekracza `1 + slotCount`, albo warn + truncate wg decyzji §18 #9).
- **Pliki (planowany scope)**: rozszerzenie writera equipment z Phase 7a, lub sibling writer; rozszerzenie warstwy apply; testy; frontend preview row.
- **Backend impact**: rozszerza nowy publiczny API equipment write do slotów talismanów.
- **Testy**: respektuje limit Pouch; odrzuca overflow wg wybranej polityki; hex round-trip; PC + PS4 parity.
- **Manual validation**: zaaplikować equipped talismans; round-trip obie platformy.
- **Ryzyka**: średnie — opiera się na infrastrukturze Phase 7a.
- **Out of scope**: spell loadout; appearance.
- **Wymaga osobnej decyzji użytkownika przed kontynuacją**: tak.

### Phase 7c — spell loadout writer (nowa write path)

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

### 17a.3. Scope tego, co **nie** jest jeszcze zwalidowane

- ✅ **Dostarczone 2026-05-31 (Phase 5D.1)** — Apply `sections.profile` i `sections.stats` do prawdziwego save przez `ApplyBuildTemplateV2FromLibraryToCharacter` (ścieżka biblioteki, `mode: "append"`, `profile.class` celowo pomijane, snapshot + rollback pod `slotMu[charIdx]`).
- ✅ **Dostarczone 2026-05-31 (Phase 5D.2)** — Direct apply zaimportowanego YAML bez wcześniejszego Save to Library, przez `ApplyBuildTemplateV2ToCharacterJSON` na canonical JSON wyprodukowanym przez `PreviewBuildTemplateImportYAMLFromFile` (`Import YAML → Preview → Apply to character`, `mode: "append"`, brak drugiego file dialogu, brak TOCTOU re-read). `ApplyBuildTemplateV2FromFileToCharacter` nadal istnieje backend/bindings-side, ale celowo pozostaje niepodpięty w UI — ścieżka JSON jest preferowana, bo jest WYSIWYG z preview, który użytkownik właśnie potwierdził.
- ✅ **Dostarczone 2026-05-31 (Phase 6)** — Apply-time overrides dla tego samego subsetu profile/stats na obu powierzchniach, przez frontend-only mutację canonical JSON przekazaną do `ApplyBuildTemplateV2ToCharacterJSON`. Brak zmian backendu, bindings, `App.tsx`. `profile.class` pozostaje read-only. v1 szablony i niewspierane v2 sekcje pozostają zablokowane. Weapon level override przy apply pozostaje odroczone do Phase 6b.
- ✅ **Dostarczone 2026-05-31 (Phase 9)** — Import szablonów z `https://` URL przez `PreviewBuildTemplateImportYAMLFromURL`, oparte o `backend/templates/url_import.go::FetchYAMLFromURL` pod pełną listą guardów §12.3 (SSRF). Preview URL reużywa istniejącego `ImportTemplatePreviewModal`, więc Save to Library / Apply to character / Apply with overrides ship bez zmian na powierzchni URL. Brak zmiany schemy biblioteki (metadata `sourceURL` nie jest dodawana); brak zmiany `App.tsx`. Authenticated downloads, domain allowlist, URL auto-refresh oraz direct apply bez preview pozostają poza scope.
- Weapon level override przy apply — gated przez Phase 6b.
- Nowe write paths dla `sections.equipment`, `sections.equippedTalismans`, `sections.spells` — gated przez Phase 7a / 7b / 7c.
- Apply appearance preset przez Templates surface — gated przez Phase 8 (underlying writer `app_appearance.go::ApplyPresetToCharacter` już istnieje, ale brak warstwy apply, która routuje szablon przez niego).
- Multi-character pack flow — gated przez Phase 10.

Praca Phase 6b+ pozostaje wyłącznie design w tym dokumencie. Każda faza wymaga osobnej akceptacji użytkownika przed implementacją zgodnie z workflow z `~/.claude/CLAUDE.md`.

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
