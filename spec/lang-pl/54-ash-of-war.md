# 54 — Ash of War

> **Type**: Design doc
> **Status**: ✅ Zaimplementowany
> **Scope**: Canonical chapter dla Ash of War — semantyka save'a, sentinele, dwie ścieżki zapisu (strict vs alokacja), guard alokacji, skan dostępności, model kompatybilności, semantyka workspace/WeaponEditModal oraz znane luki wymagające weryfikacji.

---

## 1. Cel rozdziału

Rozdział łączy wszystkie warstwy, które obsługują przypisywanie / odłączanie Ash of War w SaveForge:

- format on-disk (relacja weapon GaItem ↔ AoW GaItem, sentinele "no custom AoW"),
- model alokacji i jego nietrywialne ograniczenia (guard dotyczący strefy armament),
- ścieżki zapisu (`PatchWeaponAoWHandle` strict, `PatchWeaponAoW` legacy alokuje + rebuild),
- skan dostępności i wykrywanie shared-handle,
- model kompatybilności i mount semantics,
- ścieżka workspace (RAM-only) z punktu widzenia modala `WeaponEditModal`.

Rozdziały referencyjne, których treści **nie powtarzamy** w 54:

- [03-gaitem-map](03-gaitem-map.md) — binary layout GaItem (prefix handle, mapy, rekord 8B vs 21B).
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — pełne invarianty allocatora (`NextAoWIndex`, `NextArmamentIndex`, NextGaItemHandle), zasady walidacji post-mutation, capacity rules.
- [43-transactional-item-adding](43-transactional-item-adding.md) — transakcyjna ścieżka Add Items, post-add hooks (m.in. AoW acquisition flag).
- [36-inventory-categories-game-order](36-inventory-categories-game-order.md) — kategoria AoW w mapowaniu inventory.
- [06-equipment](06-equipment.md) — read-only model equipment i transfer guardy.
- [55-build-template](55-build-template.md) — portable JSON snapshot (przeniesienie AoW jako semantic ItemID, nie jako handle).

---

## 2. Status

| Warstwa | Status |
|---|---|
| Reader (`core.SaveSlot.parseFromData`) | ✅ akceptuje oba sentinele (`0x00000000` i `0xFFFFFFFF`). |
| Writer kanoniczny (`core.PatchWeaponAoWHandle`) | ✅ kanonizuje wyjście do `0x00000000`, guarduje shared-handle. |
| Writer legacy (`core.PatchWeaponAoW`) | ✅ alokuje nowy AoW + rebuild slotu (używany przez ukrytą legacy zakładkę). |
| Allocator guard | ✅ guard `NextArmamentIndex >= maxEntries` w `allocateGaItem` (commit `6881cb9`). |
| Skan dostępności (`core.ScanAoWAvailability`) | ✅ dwuprzebiegowy, wykrywa shared-handle. |
| Compatibility check (`db.IsAshOfWarCompatibleWithWeapon`) | ✅ z bitmaską `canMountWep_*` + `WepTypeToCanMountBit`. |
| Workspace AoW (`editor.UpdateWeapon` + `EditableItem` pending fields) | ✅ pending pattern z walidacją "fail-closed on unknown compat" przy save. |
| Frontend modal (`WeaponEditModal.tsx`) | ✅ dual-mode: workspace path i legacy path; default workspace. |
| DLC wepTypes 69/94/95 | ⚠️ allow-passthrough w backendzie (`known==false`), `needs verification` po stronie UI. |

---

## 3. Source of truth w kodzie

| Obszar | Pliki / symbole |
|---|---|
| Stałe formatu | `backend/core/structures.go`: `ItemTypeWeapon`, `ItemTypeAow`, `GaHandleTypeMask`, `NoCustomAoWHandle`, `LegacyNoCustomAoWHandle`, `IsNoCustomAoWHandle`, `GaItemFull.AoWGaItemHandle`. |
| Rozmiary rekordów | `backend/core/offset_defs.go`: `GaRecordWeapon = 21`, `GaRecordItem = 8`, `GaHandleTypeMask = 0xF0000000`. |
| Allocator + guard | `backend/core/writer.go`: `allocateGaItem` (linie ~430–501), guard linie 461–462. |
| Strict patch | `backend/core/writer.go`: `PatchWeaponAoWHandle` (linie ~1131–1207). |
| Legacy alloc + rebuild | `backend/core/writer.go`: `PatchWeaponAoW` (linie ~1209–1325). |
| Skan dostępności | `backend/core/aow_availability.go`: `ScanAoWAvailability`, `AoWCopyRaw`. |
| Compat dane | `backend/db/data/aow_compat.go`: `AoWCompatMasks`, `WepTypeToCanMountBit`, `CanMountWepNames`. |
| Weapon mount data | `backend/db/data/weapon_gem_mount.go`: `WeaponGemMounts`. |
| Compat helpers | `backend/db/db.go`: `CanWeaponMountAoW`, `IsAoWCompatibleWithWepType`, `IsAshOfWarCompatibleWithWeapon`. |
| App entrypoints | `app.go`: `ApplyWeaponAoW`, `ApplyWeaponAoWStrict`, `GetAoWAvailability`. |
| Editor patch DTO | `backend/editor/weapon.go`: `WeaponPatch`, `UpdateWeapon`. |
| Editor workspace state | `backend/editor/workspace.go`: `EditableItem.Current*`/`Pending*`/`CanMountAoW`/`WepType`, stałe `AoWStatus*`. |
| Save-side execution | `backend/editor/save.go`: `collectPendingAoWChanges`, `validatePendingAoWChanges`, `executePendingAoWPatches`. |
| Walidator workspace | `backend/editor/validate.go`: `CodePendingAoWUnknown`, `CodePendingAoWConflict`. |
| Frontend modal | `frontend/src/components/WeaponEditModal.tsx` (workspace + legacy ścieżki, własny `WEP_TYPE_TO_BIT` mirror). |
| Wewn. zakładka legacy | `frontend/src/components/WeaponEditTab.tsx` (ukryta / deprecated; używa zarówno `ApplyWeaponAoW` jak i `ApplyWeaponAoWStrict` w zależności od toggle `createCopy`; własny `WEP_TYPE_TO_BIT` mirror). |
| Workspace integration | `frontend/src/components/SortOrderTab.tsx` (wstrzykuje `workspace` + `workspaceItem` do `WeaponEditModal` — patrz §16). |

---

## 4. Mental model

Każda broń ma umiejętność widoczną w grze jako **Ash of War**. Pochodzi z jednego z dwóch źródeł:

- **Wbudowana umiejętność broni** — `EquipParamWeapon.swordArtsParamId` w `regulation.bin`. Zawsze dostępna, nigdy nie zapisywana w save.
- **Custom Ash of War gem** — przedmiot z inwentarza posiadający własny GaItem (prefix handle `0xC0…`). Po przypięciu nadpisuje wbudowaną umiejętność własnym `EquipParamGem.swordArtsParamId`.

Trzy parametry `regulation.bin` są warstwą referencyjną. SaveForge **nigdy** ich nie modyfikuje:

- `EquipParamWeapon.swordArtsParamId` — fallback skill.
- `EquipParamGem.swordArtsParamId` + `canMountWep_*` — skill i 36-bitowa maska kompatybilności gema.
- `SwordArtsParam` — definicje umiejętności (animacje, koszty, in-game name).

Konsekwencje pomięcia tego rozróżnienia (i błędy, które historycznie z tego wynikały):

- Traktowanie "brak custom AoW" jako "brak skilla w ogóle" myliło użytkownika — usunięcie custom AoW przywraca wbudowany skill, nie kasuje go. Naprawione w commicie `cb1a822`.
- Traktowanie dwóch kopii custom-AoW tego samego ItemID jako jednej instancji prowadziło do fałszywych "in use" w UI albo do groźniejszego shared-handle crashu (§10).

---

## 5. Weapon GaItem ↔ AoW GaItem relation

Binary layout opisany w [03-gaitem-map](03-gaitem-map.md). Tu tylko AoW-specific:

| Element | Wartość |
|---|---|
| Prefix handle weapon GaItem | `0x80000000` (`ItemTypeWeapon`). |
| Prefix handle AoW GaItem | `0xC0000000` (`ItemTypeAow`). |
| Rozmiar weapon GaItem | `GaRecordWeapon = 21` bajtów. |
| Rozmiar AoW GaItem | `GaRecordItem = 8` bajtów (taki sam jak inne stackable). |
| Offset pola `AoWGaItemHandle` w weapon GaItem | `[+0x10:+0x14]` (u32 LE). |
| Prefix ItemID gema AoW | upper nibble `0x8` (np. Lion's Claw = `0x80002710`). |

Relacja jest **handle-based**: weapon GaItem referuje przypięty gem przez handle, nie przez ItemID. Ten sam ItemID może mieć wiele osobnych kopii (różne handle) — każda fizycznie istnieje w `slot.GaItems` (np. dropy ze skarabeuszy, Lost Ashes of War).

```
weapon GaItem (21B)               AoW GaItem (8B)
+------+-------------+      +---->+--------+--------+
|  ... | Handle      |      |     | Handle | ItemID |
| [0x00] 0x80xxxxxx  |      |     | 0xC0.. | 0x80.. |
|  ... | ItemID      |      |     +--------+--------+
| [0x10] AoWGaItemHandle ---+
|       (u32: 0x00000000     resolved via slot.GaMap[handle] → ItemID
|        sentinel OR 0xC0…)
+--------------------+
```

Resolver gry przy load:

```
if !IsNoCustomAoWHandle(weapon.AoWGaItemHandle):
    gem    = GaMap[weapon.AoWGaItemHandle]
    skill  = EquipParamGem[gem.ItemID].swordArtsParamId
else:
    skill  = EquipParamWeapon[weapon.ItemID].swordArtsParamId
```

---

## 6. AoWGaItemHandle and sentinel values

Pole `AoWGaItemHandle` w weapon GaItem niesie jedną z czterech klas wartości:

| Wartość | Znaczenie | Writer | Reader |
|---|---|---|---|
| `0x00000000` | Brak custom AoW — **kanoniczny vanilla sentinel** (gra pisze tę wartość dla każdej świeżo utworzonej broni). | ✅ kanonizowane do tej wartości | ✅ akceptowane |
| `0xFFFFFFFF` | Brak custom AoW — **legacy SaveForge sentinel** emitowany przez buildy sprzed commita `4e800b9`. | ❌ nigdy nie emitowany | ✅ akceptowane dla kompat. |
| `0xC0xxxxxx` (prefix `ItemTypeAow`) | Valid custom AoW handle. Musi rozwiązywać się do istniejącego AoW GaItem w slocie. | ✅ zapisywane przy attach | ✅ akceptowane |
| Cokolwiek innego | Invalid / corrupted. | n/d | flagowane warstwę wyżej |

Stałe i helper:

```go
// backend/core/structures.go
const NoCustomAoWHandle       uint32 = 0x00000000
const LegacyNoCustomAoWHandle uint32 = 0xFFFFFFFF

func IsNoCustomAoWHandle(h uint32) bool {
    return h == NoCustomAoWHandle || h == LegacyNoCustomAoWHandle
}
```

Akceptacja obu sentineli pozwala otwierać save'y zedytowane starszymi wydaniami SaveForge bez re-flagowania każdej broni. Emisja pojedynczego kanonicznego sentinela utrzymuje świeżo zapisane save'y nieodróżnialne od vanilla output na tym offsecie (anti-flag).

---

## 7. AoW item data i categories

AoW gemy są wpisywane do DB SaveForge z kategorią `ashes_of_war` (zob. [36-inventory-categories-game-order](36-inventory-categories-game-order.md)). Workspace edits walidują kategorię w `editor.UpdateWeapon`:

```go
aow, _ := db.GetItemDataFuzzy(patch.AoWItemID)
if aow.Name == "" { return ErrUnknown }
if aow.Category != "ashes_of_war" { return ErrWrongCategory }
```

Save-side walidacja (`validatePendingAoWChanges`) powtarza ten sam check defense-in-depth. Walidator workspace (`validate.go`) ma osobny kod błędu `CodePendingAoWUnknown` dla bezpośrednich mutacji pola, które ominęły `UpdateWeapon`.

---

## 8. Compatibility model

Kompatybilność (AoW × broń) ma trzy poziomy gate'ów liczonych w `backend/db/db.go`:

### 8.1. Per-weapon: `CanWeaponMountAoW`

```go
func CanWeaponMountAoW(baseItemID uint32) bool {
    return GetItemData(baseItemID).GemMountType == 2
}
```

`GemMountType == 2` = "standard infusable" (np. Longsword); `1` = "special/somber" (np. Sword of Night and Flame); `0` = brak mountu.

### 8.2. Per-AoW × wepType: `IsAoWCompatibleWithWepType`

```go
func IsAoWCompatibleWithWepType(aowItemID uint32, wepType uint16) (compatible, known bool) {
    aow := GetItemData(aowItemID)
    if aow.AoWCompatBitmask == 0 { return false, false }
    bitPos, ok := data.WepTypeToCanMountBit[wepType]
    if !ok { return false, false }
    return (aow.AoWCompatBitmask>>bitPos)&1 == 1, true
}
```

Bitmaska 36-bitowa pochodzi z `EquipParamGem.canMountWep_*` (kolumny od `Dagger` do `Torch` — pełna lista w `data.CanMountWepNames`). `WepTypeToCanMountBit` mapuje `EquipParamWeapon.wepType` na pozycję bitu.

Sygnatura `(compatible, known)`:

- `known == false` → SaveForge **nie posiada danych** wystarczających do oceny (bitmask braku albo wepType poza mapą). Wywołujący decyduje: blok (fail-closed) czy passthrough.
- `known == true` → rozstrzygnięcie binarne.

### 8.3. Combined: `IsAshOfWarCompatibleWithWeapon`

```go
func IsAshOfWarCompatibleWithWeapon(aowItemID uint32, weaponItemID uint32) (compatible, known bool) {
    wep, _ := GetItemDataFuzzy(weaponItemID)
    if wep.GemMountType != 2 { return false, true }     // somber / no-mount
    if wep.WepType == 0      { return false, false }   // wepType unknown
    return IsAoWCompatibleWithWepType(aowItemID, wep.WepType)
}
```

`GetItemDataFuzzy` zwraca dane dla baseID (po stripie infusion offset i upgrade level), więc `wp+15 Cold` i `wp+0` resolwują tę samą podstawę.

### 8.4. Reguły wywołań (kto fail-closes vs fail-opens)

| Caller | `known == false` | `known && !compatible` |
|---|---|---|
| `app.ApplyWeaponAoW` | passthrough (mutuje slot) | block |
| `app.ApplyWeaponAoWStrict` | passthrough (mutuje slot) | block |
| `editor.validatePendingAoWChanges` (workspace save) | **fail-closed** (`refusing fail-closed`) | block |
| `WeaponEditModal` (UI default view) | hide (fail-closed widoczność) | hide |

Niezgodność `app.go` (passthrough) z save-side (fail-closed) jest celowa: ścieżka legacy ApplyWeapon* mutuje slot natychmiast i opiera się na fakcie, że `CanWeaponMountAoW(baseID) == true` przeszło wcześniej (UI listuje tylko mountable bronie). Workspace save jest bardziej restrykcyjny, bo szablony mogą wnieść AoW × weapon kombinację, której UI nigdy nie zaprezentowało.

---

## 9. Weapon gem mount semantics

`backend/db/data/weapon_gem_mount.go` jest generowany przez `tmp/scripts/import_aow_compat.py` z `EquipParamWeapon.csv`. Klucze obejmują warianty bazowe (upgrade 0) i warianty infusion (+100, +200, …). Tylko bronie z `gemMountType != 0` są w mapie. `db.go` mergeuje to do `globalItemIndex`, ustawiając `entry.GemMountType` i `entry.WepType`.

| `gemMountType` | Znaczenie | UI / writer |
|---|---|---|
| `0` | Brak mountu (np. catalyst, torch w niektórych przypadkach) | weapon nie jest editable AoW; modal nie pokazuje sekcji AoW |
| `1` | Special / somber (skill na stałe w `EquipParamWeapon`, gem nie może go zmienić) | `CanMountAoW == false`; modal może być otwarty, ale akcje AoW disabled |
| `2` | Standard infusable | `CanMountAoW == true`; pełna ścieżka AoW dostępna |

`needs verification`: w aktualnym kodzie `EditableItem.CanMountAoW = (itemData.GemMountType == 2)` — `gm == 1` jest traktowane jako "nie można zmienić AoW", ale **nie** wyłącza modyfikacji affinity/upgrade. Dokument nie potwierdza wszystkich edge cases (np. czy w UI pokazujemy informację, że to weapon `gm==1`).

---

## 10. Availability scanning

`core.ScanAoWAvailability(slot *SaveSlot) []AoWCopyRaw` jest pojedynczym przebiegiem dwupasowym po `slot.GaItems`.

### 10.1. Pass 1 — zbiór AoW + referencje weapon→AoW

```go
for i := range slot.GaItems {
    g := &slot.GaItems[i]
    if g.IsEmpty() { continue }
    switch g.Handle & GaHandleTypeMask {
    case ItemTypeAow:
        copies = append(copies, AoWCopyRaw{ItemID: g.ItemID, Handle: g.Handle})
    case ItemTypeWeapon:
        if !IsNoCustomAoWHandle(g.AoWGaItemHandle) {
            weaponRefs[g.AoWGaItemHandle] = append(weaponRefs[g.AoWGaItemHandle], g.Handle)
        }
    }
}
```

### 10.2. Pass 2 — used / free / shared

```go
for i := range copies {
    weapons := weaponRefs[copies[i].Handle]
    if len(weapons) == 0 { continue }       // wolna kopia
    copies[i].UsedByWeaponHandle = weapons[0]
    if len(weapons) > 1 {
        copies[i].HasSharedHandleConflict = true
    }
}
```

### 10.3. Agregat per ItemID (`app.GetAoWAvailability`)

`app.go::GetAoWAvailability` agreguje wynik do `vm.AoWAvailabilityEntry`:

| Pole | Sens |
|---|---|
| `TotalCopies` | Liczba kopii tego ItemID w slocie. |
| `AvailableCopies` | Kopie, których żadna broń nie referuje. |
| `UsedCopies` | Kopie referowane przez przynajmniej jedną broń. |
| `UsedByWeaponHandles` | Lista weapon handle'i (pierwszy ref per kopia). |
| `IsMissing` | Zawsze `false` z app.go (UI traktuje brak wpisu jako missing). |
| `HasSharedHandleConflict` | OR po wszystkich kopiach. |

UI mapuje to na pięć statusów:

| Status | Warunek |
|---|---|
| `current` | ItemID == currentAoWId broni edytowanej. |
| `available` | `availableCopies > 0`. |
| `in_use` | Wszystkie kopie referowane przez bronie. |
| `missing` | Brak wpisu (`!aowAvailability.has(id)` w UI). |
| `conflict` | `hasSharedHandleConflict == true`. |

Skan **nie** sprawdza kompatybilności (`canMountWep_*`) ani `gemMountType`. To są warstwy niezależne.

---

## 11. Shared-handle conflicts

Pojedynczy handle gema AoW identyfikuje jedną fizyczną instancję. Dwie różne bronie **nigdy** nie mogą referować tego samego non-sentinel `AoWGaItemHandle`. Naruszenie → `EXCEPTION_ACCESS_VIOLATION` w grze przy loadzie.

Trzy ścieżki egzekwują invariant:

| Ścieżka | Egzekwowanie |
|---|---|
| `ScanAoWAvailability` | Wykrywa multi-referenced handle (`len(weapons) > 1`), oznacza obie kopie `HasSharedHandleConflict`. |
| `PatchWeaponAoWHandle` (strict) | Skanuje słot i odrzuca attach, jeśli inny weapon GaItem już referuje ten handle. |
| `PatchWeaponAoW` (legacy) | `generateUniqueHandle(slot, ItemTypeAow)` mintuje świeży handle dla każdego wywołania — nigdy nie reużywa istniejącego. |

Co jest legalne (i często myli):

- Ten sam **ItemID** AoW może mieć wiele osobnych kopii (różne handle). Tak właśnie kumulują się Lost Ashes of War i dropy.
- Dwie różne bronie mogą obie pokazywać ten sam Ash of War w grze, o ile referują **różne** handle gema o tym samym ItemID.

UI ścieżki strict prezentuje `conflict` status w listingu AoW i blokuje apply tej kopii (zamiast obu broni). `App.ApplyWeaponAoWStrict` dodatkowo refusuje jeśli `hasConflict` w `ScanAoWAvailability` (linia ~1493 app.go).

---

## 12. Write paths overview

SaveForge ma **dwie** ścieżki zapisu AoW na poziomie core. Wybór jest funkcją punktu wejścia i historii UI:

| Path | Funkcja core | Punkt wejścia App | UI |
|---|---|---|---|
| **Strict** (in-place, no rebuild) | `core.PatchWeaponAoWHandle` | `app.ApplyWeaponAoWStrict` | `WeaponEditModal` (Sort Order) + `WeaponEditTab` w trybie `createCopy=false` |
| **Legacy / allocate + rebuild** | `core.PatchWeaponAoW` | `app.ApplyWeaponAoW` | wyłącznie `WeaponEditTab` w trybie `createCopy=true` (ukryta / deprecated zakładka) |
| **Workspace pending → save** | `core.PatchWeaponAoWHandle` (clear) lub `core.PatchWeaponAoW` (set) | `editor.executePendingAoWPatches` (wołany przez `ApplyWorkspaceSave`) | `WeaponEditModal` w `isWorkspaceMode == true` |

Wybór "strict vs allocate" ma znaczenie operacyjne:

- **Strict** wymaga istniejącej wolnej kopii AoW GaItem o pożądanym ItemID — UI musi to potwierdzić skanem dostępności.
- **Legacy** zawsze mintuje świeży handle i alokuje nowy AoW GaItem; wymaga pełnego `RebuildSlotFull` i reparse'u — droższe i podlega guardowi z §14.

Workspace save mostkuje obie ścieżki: clear → strict (in-place sentinel), set → legacy (alokuje nowy GaItem). Konsekwencja: każda zmiana z `PendingAoWClear=true` jest tania; zmiana z `PendingAoWItemID != 0` rebuilduje slot.

---

## 13. Strict patch path

`core.PatchWeaponAoWHandle(slot, weaponHandle, newAoWHandle) error`

Kontrakt:

1. Lokalizuje weapon GaItem po `weaponHandle` i jego offset bajtowy. Brak → błąd.
2. Jeśli typ rekordu ≠ `ItemTypeWeapon` → błąd.
3. Jeśli `IsNoCustomAoWHandle(newAoWHandle)` → kanonizuje do `NoCustomAoWHandle` i wpisuje na offset `[+0x10]`. (Akceptuje oba sentinele na wejściu, emituje tylko `0x00000000`.)
4. Inaczej:
   - prefix `newAoWHandle` musi być `ItemTypeAow` → inaczej błąd,
   - musi istnieć AoW GaItem o tym handle w slocie → inaczej błąd,
   - żaden **inny** weapon GaItem nie może referować tego handle (shared-handle guard) → inaczej błąd.
5. Pisze 4 bajty na `[weaponOff+16]`. Mutuje `slot.GaItems[idx].AoWGaItemHandle`.

Nie alokuje, nie rebuilduje, nie modyfikuje innych offsetów. Cała operacja to **dokładnie 4 bajty** zmiany na dysku.

`Weapon.ItemID` **nigdy** nie jest dotykany — affinity i upgrade level przeżywają każdy attach/detach (zob. test `TestPatchWeaponAoWHandle_RemovePreservesWeaponItemID`).

---

## 14. Allocation / rebuild path

`core.PatchWeaponAoW(slot, weaponHandle, newAoWItemID) error`

Dwa tryby:

### 14.1. Remove (`newAoWItemID == 0`)

Identyczny efekt jak strict-remove: pisze `NoCustomAoWHandle` na `[weaponOff+16]`. Brak alokacji, brak rebuildu.

### 14.2. Set (`newAoWItemID != 0`)

1. Waliduje upper nibble ItemID == `0x8`.
2. `generateUniqueHandle(slot, ItemTypeAow)` → świeży `0xC0…` handle.
3. `allocateGaItem(slot, newAoWHandle, newAoWItemID)` — wstawia rekord AoW na pozycję `NextAoWIndex`, advance'uje `NextAoWIndex` i `NextArmamentIndex` (zob. §15).
4. `slot.GaMap[newAoWHandle] = newAoWItemID`.
5. `upsertGaItemData(slot, newAoWItemID)` — registruje ItemID w sekcji GaItemData (jeśli już nie istnieje).
6. Snapshot `NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`.
7. `RebuildSlotFull(slot)` + `parseFromData()` — sekcja GaItems rozrosła się o 8B, każdy offset downstream jest przeliczany.
8. Restore indeksów, jeśli `parseFromData` underscanował.
9. Re-lokalizuje weapon GaItem po `weaponHandle` (offset bajtowy mógł się przesunąć po rebuildzie).
10. Pisze `newAoWHandle` na `[weaponOff+16]`.

Stary AoW GaItem (poprzednio referowany) **nie jest garbage-collected**. Gra toleruje sieroty (orphan copies) w GaMap; strict path może je później ponownie przypiąć bez alokacji.

Funkcja jest udokumentowana w kodzie jako "currently invoked only via App.ApplyWeaponAoW, which itself is reachable only from the hidden legacy Weapon Edit tab" — zob. §17.

---

## 15. AoW Allocation Safety (guard `6881cb9`)

Allocator `allocateGaItem` ma jeden AoW-specific guard, którego pełen kontekst capacity rules jest w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md). Tu opisujemy tylko AoW consequence.

### 15.1. Reguła

AoW insertion **bezwarunkowo** advance'uje `NextArmamentIndex` (prawą krawędź strefy armament). Jeśli `NextArmamentIndex` jest już równe `len(slot.GaItems)`, advance pchnąłby je poza tablicę, łamiąc invariant walidatora post-mutation (`NextArmamentIndex > len(GaItems)`).

```go
// backend/core/writer.go:445–479 (gałąź isAoW w allocateGaItem)
idx := slot.NextAoWIndex
if idx >= maxEntries {
    return error("AoW array full")
}
if slot.NextArmamentIndex >= maxEntries {
    return error("cannot insert AoW — armament zone at capacity (NextArmamentIndex %d == %d)")
}
// ... shift/insert ...
slot.NextAoWIndex++
slot.NextArmamentIndex++ // AoW insertion shifts armament zone right
```

### 15.2. Dlaczego, historycznie

Przed commitem `6881cb9` gałąź AoW sprawdzała tylko `NextAoWIndex < maxEntries`, ale inkrementowała również `NextArmamentIndex`. Save'y PS4 obserwowane w terenie (slot 1 "Bydlaczka") miały `NextAoWIndex=3` (jest miejsce w strefie AoW), ale `NextArmamentIndex == len(GaItems)` (najwyżej-indexowy non-empty entry siedział na pozycji `maxEntries-1`). Każdy AoW add powodował overflow i `ValidatePostMutation` zgłaszał `"NextArmamentIndex N > len(GaItems) N"` — komunikat numeryczny, nie informujący użytkownika o realnej przyczynie.

### 15.3. Co guard gwarantuje

Reject jest **before-mutation**: ani `NextAoWIndex`, ani `NextArmamentIndex`, ani `slot.GaItems[idx]` nie są dotknięte. `ValidatePostMutation` przechodzi czysto dla stanu pre-call. Test `TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity` (`backend/core/gaitem_placement_test.go:324`) lockuje:

- `slot.NextAoWIndex` pozostaje `3`,
- `slot.NextArmamentIndex` pozostaje `8` (== `maxEntries`),
- `slot.GaItems[3]` nadal `IsEmpty()`,
- `ValidatePostMutation` → 0 violations.

### 15.4. Co guard NIE gwarantuje

- Nie chroni przed sytuacją, w której weapon/armor add wypełnia ostatnie miejsce — to jest pokryte przez gałąź `!isAoW` ("armament/armor array full").
- Nie sprawdza, czy między `NextAoWIndex` a `NextArmamentIndex` jest gap (shift-right logic obsługuje to niezależnie).
- Pełne capacity rules (NextEquipIndex, NextAcquisitionSortId, NextGaItemHandle) — opisane w [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).

---

## 16. Workspace and WeaponEditModal state

WeaponEditModal działa w **dwóch trybach** w zależności od propsów:

```typescript
const isWorkspaceMode = !!workspace && !!workspaceItem;
```

| Tryb | Warunek propsów | Read state | Write state |
|---|---|---|---|
| Workspace | `workspace` + `workspaceItem` przekazane | `EditableItem` (RAM, pending fields uwzględnione) | `workspace.updateWeapon(uid, patch)` → mutuje snapshot RAM |
| Legacy | tylko `charIndex` + `item` | `GetCharacter(charIndex)` (snapshot save'a) | `ApplyWeaponAoWStrict` (in-place patch slot) |

### 16.1. Workspace read path

```typescript
const [currentAoWId, setCurrentAoWId]   = useState(workspaceItem?.currentAoWItemID ?? 0);
const [canMountAoW, setCanMountAoW]     = useState(workspaceItem?.canMountAoW ?? false);
const [wepType, setWepType]             = useState(workspaceItem?.wepType ?? 0);
```

Po każdym `workspace.updateWeapon(...)` zwrócony świeży `EditableItem` synchronizuje state przez `useEffect`.

### 16.2. Dlaczego workspace mode NIE używa `GetCharacter`

Komentarz w kodzie (`WeaponEditModal.tsx:131–136`):

> *"In workspace mode the AoW-mount metadata (currentAoWId / canMountAoW / wepType) comes straight from the editable workspace item — GetCharacter reads the *save* state, which can drift from the workspace (added items have no save-side handle, prior Saves may re-allocate handles). Legacy mode keeps the GetCharacter fallback so WeaponEditTab and other non-workspace callers are unaffected."*

Konkretnie:

- Item dodany w workspace (Source=Added) **nie ma jeszcze** rzeczywistego `OriginalHandle` w save'ie — `GetCharacter` zwróci snapshot bez tego itemu.
- Ostatnie save'y mogą realokować handle przy `core.PatchWeaponAoW` (rebuild slotu) — `GetCharacter` cache może zwrócić stary handle.
- Pending edits (`PendingAoWItemID`, `PendingAoWClear`) są wyłącznie w RAM workspace — `GetCharacter` ich nie widzi.

### 16.3. Workspace write path

`workspace.updateWeapon(uid, patch)` → `editor.UpdateWeapon`:

| `WeaponPatch` field | Efekt |
|---|---|
| `SetAoWItemID = true, AoWItemID != 0` | Waliduje DB (kategoria `ashes_of_war`), ustawia `PendingAoWItemID` + `PendingAoWName`, zeruje `PendingAoWClear`. |
| `SetAoWItemID = true, AoWItemID == 0` | Traktowane jak `ClearAoW` — `PendingAoWClear = true`. |
| `ClearAoW = true` | `PendingAoWClear = true`, zeruje `PendingAoWItemID`/`PendingAoWName`. |
| Każdy z powyższych | `HasPendingWeaponPatch = true`, `snap.Dirty = true`, re-walidacja. |

Walidacja przy save (`editor.validatePendingAoWChanges`) **dodatkowo** sprawdza kompatybilność i jest **fail-closed on unknown**:

```
if !known {
    return error("AoW/weapon compatibility unknown ... refusing fail-closed")
}
if !compatible {
    return error("AoW ... is not compatible with weapon ...")
}
```

Egzekucja (`executePendingAoWPatches`):

- `Clear` → `core.PatchWeaponAoWHandle(slot, handle, NoCustomAoWHandle)` (strict, in-place).
- `Set`  → `core.PatchWeaponAoW(slot, handle, c.AoWItemID)` (legacy, alloc + rebuild).

### 16.4. Workspace AoW status (`EditableItem.CurrentAoWStatus`)

Stałe w `backend/editor/workspace.go`:

| Wartość | Znaczenie |
|---|---|
| `AoWStatusNone = "none"` | Brak custom AoW (sentinel handle). |
| `AoWStatusCustom = "custom"` | Custom AoW rozwiązany do znanego DB ItemID. |
| `AoWStatusMissing = "missing"` | Handle non-sentinel, ale nie resolwuje do AoW GaItem (orphan / dangling). |
| `AoWStatusShared = "shared"` | Handle referowany przez >1 broń (save corruption). |

`needs verification`: pełna ścieżka populacji `CurrentAoW*` jest w `populateCurrentAoW` + `buildWeaponAoWMaps` (`workspace.go:367+`). Edge cases (np. item Added bez handle, item Removed z czekającym Pending) zostały zweryfikowane testami `current_aow_test.go`, ale nie wszystkie kombinacje są opisane tu — nie powtarzam pełnej macierzy.

---

## 17. Frontend / backend compatibility drift

UI utrzymuje **dwa równoległe** mirrory `WepTypeToCanMountBit`: jeden w `WeaponEditModal.tsx` (linie ~48–54) i drugi w `WeaponEditTab.tsx` (linia ~13). Oba zbiory par są dziś identyczne z `backend/db/data/aow_compat.go::WepTypeToCanMountBit`, ale każdy jest podtrzymywany ręcznie — żaden generator ani test CI nie wymusza zgodności:

```typescript
// frontend/src/components/WeaponEditModal.tsx
const WEP_TYPE_TO_BIT: Record<number, number> = {
    1: 0, 3: 1, 5: 2, 7: 3, 9: 8, 11: 9, 13: 6, 14: 5, 15: 4, 16: 7, 17: 7,
    19: 11, 21: 13, 23: 10, 24: 10, 25: 12, 28: 14, 29: 14, 31: 15, 32: 17,
    33: 18, 35: 20, 37: 19, 39: 20, 41: 21, 43: 22, 50: 23, 51: 24, 52: 25,
    53: 26, 54: 27, 55: 28, 57: 29, 61: 30, 65: 32, 66: 33, 67: 34, 68: 35,
    87: 25, 88: 25, 89: 26, 90: 27, 91: 26, 92: 26, 93: 26,
};
```

**Drift risk**: backend `data.WepTypeToCanMountBit` jest źródłem prawdy (zob. `backend/db/data/aow_compat.go:245–291`). Każda aktualizacja po stronie backendu (np. nowy DLC wepType) musi być propagowana **ręcznie** do obu mirrorów frontend; brak CI guardu i brak generatora. CHANGELOG (commit `25fa240` "wire ash of war edit in weapon modal" oraz dyskusja w późniejszych entries) wprost flaguje docelowy plan refaktoryzacji do shared frontend helper — dopóki to się nie wydarzy, każda zmiana jest `needs verification` w obu plikach.

UI fail-closes na nieznanym `wepType`:

```typescript
function getAoWCompatStatus(aowCompatBitmask: number, wepType: number) {
    if (aowCompatBitmask === 0 || wepType === 0) return 'unknown';
    const bitPos = WEP_TYPE_TO_BIT[wepType];
    if (bitPos === undefined) return 'unknown';
    // …
}
```

`unknown` jest blokowany w domyślnym widoku (`Show unavailable = false`). Po włączeniu toggle'a `Show unavailable` wpisy `unknown`/`incompatible` są widoczne, ale **nie są aplikowalne** (`canApplyAoW` wymaga `compat === 'compatible'`).

Kontrast vs backend:

| Warstwa | `known == false` behavior |
|---|---|
| `app.ApplyWeaponAoW*` (legacy + strict) | Allow passthrough — zakłada, że `CanWeaponMountAoW` (GemMountType==2) wystarcza. |
| `editor.validatePendingAoWChanges` | Fail-closed. |
| `WeaponEditModal` UI | Hide / disable. |

Decyzja `app.go` allow-passthrough vs UI fail-closed jest celowa: legacy ApplyWeaponAoW pozwala bypassować ograniczenie UI dla developera/testowania DLC. Workspace save jest restrykcyjny, bo szablony i auto-apply mogą wniesć kombinacje niepotwierdzone przez UI.

`needs verification`: bitmask może być niekompletny dla DLC AoW gemów (rows z `EquipParamGem` nie objęte importem `import_aow_compat.py`). Affinity gating dla wariantów infusion (np. Heavy Longsword vs Standard Longsword) **nie jest** obsługiwany przez bitmask — `EquipParamWeapon.defaultWepAttr` / `configurableWepAttr00..23` nie są zaimportowane do `WeaponGemMounts`. Nie znaleziono dowodu, że affinity gating jest egzekwowany w SaveForge.

---

## 18. Relationship to Equipment

Equipment ([06-equipment](06-equipment.md)) jest read-only z perspektywy AoW: equipped weapon handle wskazuje na weapon GaItem w `slot.GaItems`, którego `AoWGaItemHandle` jest mutowany przez ścieżki §13/§14 niezależnie. Equipment slot **nie** trzyma żadnej referencji do AoW.

Konsekwencje:

- Edycja AoW na bronie zaequippowanej działa bez touching equipment section.
- Transfer broni między inventory ↔ storage ([53-inventory-storage-transfer](53-inventory-storage-transfer.md)) **zachowuje** `OriginalHandle` i `AoWGaItemHandle` — AoW przenosi się razem z bronią.
- Remove broni z inventory **nie** kasuje przypiętego AoW GaItem — staje się orphanem.

---

## 19. Relationship to Build Templates

Build Templates ([55-build-template](55-build-template.md)) eksportują AoW jako **portable ItemID**, nie jako handle:

- Template field `aowItemID` jest pointer + `omitempty` — pominięty oznacza "no custom AoW".
- Eksporter **nigdy** nie zapisuje save-local handle (`OriginalHandle`, `AoWGaItemHandle`, `CurrentAoWHandle`).
- Import preview liczy AoW compatibility przez `db.IsAshOfWarCompatibleWithWeapon` i **fail-closes on unknown** — odrzuca template, jeśli docelowy save ma weapon `wepType` poza `WepTypeToCanMountBit` albo AoW poza `AoWCompatMasks`.
- Apply mostkuje przez workspace `EditableItem.PendingAoWItemID`, czyli używa ścieżki §16. Pending Set → save → `core.PatchWeaponAoW` (alokuje nowy AoW GaItem).

Pełny opis (schema, fazy A–E, library) — w `55-build-template.md` (canonical rewrite w następnym kroku).

---

## 20. Validation and safety notes

| Warstwa | Reguła |
|---|---|
| Reader | Akceptuje `0x00000000` i `0xFFFFFFFF` jako "no custom AoW". |
| Writer (strict + legacy) | Emituje wyłącznie `0x00000000` dla no-custom. |
| Writer (strict) | Odrzuca attach handle, który nie jest sentinelem ani prefixem `0xC0`. |
| Writer (strict) | Odrzuca attach handle, który nie wskazuje na istniejący AoW GaItem. |
| Writer (strict) | Odrzuca attach handle już referowany przez inną broń (shared-handle). |
| Writer (legacy set) | Mintuje **nowy** handle dla każdego attach — nigdy nie reużywa istniejącego. |
| Allocator | Odrzuca AoW add, jeśli `NextArmamentIndex == len(GaItems)` (§15). |
| Editor (UpdateWeapon) | Odrzuca AoW ItemID poza DB lub poza kategorią `ashes_of_war`. |
| Editor (validate) | `CodePendingAoWUnknown`, `CodePendingAoWConflict` jako defense-in-depth. |
| Editor (save) | Fail-closed on unknown compat (`refusing fail-closed`). |
| UI modal | Domyślnie ukrywa `incompatible` i `unknown`; explicite toggle pokazuje, ale nie pozwala aplikować. |
| Weapon ItemID | **Nigdy** nie modyfikowany przez AoW operations — affinity i upgrade level są niezmiennikami. |

Anty-pattern, którego dokument nie ma dokumentować jako "safe":

- ❌ Share AoW handle między weapon entries (crash gry).
- ❌ Klonowanie handle z save'a A do save'a B (handle jest save-local; nie ma znaczenia w innym slocie).
- ❌ Fail-open na unknown compat w workspace save path (workspace fail-closes celowo).
- ❌ Affinity gating per AoW (`defaultWepAttr`/`configurableWepAttr00..23`) — `needs verification`, nie zaimplementowane.

---

## 21. Test coverage

| Plik testu | Co lockuje |
|---|---|
| `backend/core/aow_strict_test.go::TestIsNoCustomAoWHandle` | Helper akceptuje oba sentinele. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoWHandle_RemoveWritesZeroSentinel` | Strict remove emituje `0x00000000`. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoWHandle_RemovePreservesWeaponItemID` | Weapon ItemID przeżywa remove. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoWHandle_AoWAlreadyUsed` | Shared-handle guard w strict. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoWHandle_AttachFreeHandle` | Attach wolnej kopii działa. |
| `backend/core/aow_strict_test.go::TestPatchWeaponAoW_LegacyRemoveWritesZeroSentinel` | Legacy remove emituje `0x00000000`. |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_FreeAndUsedCopies` | Skan rozróżnia free vs used. |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_SharedHandleConflict` | Skan flaguje shared-handle. |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_ZeroSentinelNotCounted` | Sentinel `0x00000000` nie liczy się jako użycie. |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_LegacyFFFFFFFFSentinelNotCounted` | Sentinel `0xFFFFFFFF` nie liczy się jako użycie. |
| `backend/core/aow_strict_test.go::TestAllocateGaItem_NewWeaponUsesZeroSentinel` | Allocator inicjalizuje weapon GaItem z `NoCustomAoWHandle`. |
| `backend/core/gaitem_placement_test.go::TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity` | Guard `6881cb9` (§15). |
| `backend/core/aow_dual_destination_test.go::TestNonStackableDualDestinationUniqueHandles` | Dodanie AoW do inv + storage tworzy dwa osobne handle. |
| `backend/editor/current_aow_test.go::*` | Populacja `CurrentAoW*` po skanie + pending flow. |
| `backend/editor/weapon_test.go::*` | `WeaponPatch` semantics (SetAoWItemID, ClearAoW, HasPendingWeaponPatch). |
| `backend/editor/save_test.go::TestValidatePendingAoWChanges_*` | Fail-closed on unknown compat, reject non-AoW category, accept clear. |
| `app_weapon_aow_dlc_test.go::TestApplyWeaponAoW_DLCUnmappedWepType_Allows` | DLC wepType=69 (Dragon Towershield) — passthrough. |
| `app_weapon_aow_dlc_test.go::TestApplyWeaponAoW_DLCGreatKatana_Allows` | DLC wepType=94 — passthrough. |
| `app_weapon_aow_dlc_test.go::TestApplyWeaponAoWStrict_DLCUnmappedWepType_Allows` | Strict mode również passthrough na DLC. |
| `app_weapon_aow_dlc_test.go::TestApplyWeaponAoW_KnownIncompatible_Blocks` | known + !compatible blokuje. |
| `app_weapon_aow_dlc_test.go::TestApplyWeaponAoW_NonMountableWeapon_Blocks` | `gemMountType != 2` blokuje. |
| `app_weapon_aow_dlc_test.go::TestApplyWeaponAoW_RemoveAlwaysAllowed` | Remove pomija compat check. |
| `app_weapon_aow_editor_test.go::*` | Pełna macierz legacy `ApplyWeaponAoW` (no-save, invalid char, handle-not-found, remove, existing-free, missing-create, used-create). |
| `frontend/src/components/WeaponEditModal.workspace.test.tsx` | Workspace mode read/write z workspaceItem. |

---

## 22. Known limits / needs verification

| # | Obszar | Status |
|---|---|---|
| L1 | Affinity gating per AoW (`defaultWepAttr`/`configurableWepAttr00..23`) | `needs verification` — nie znaleziono ścieżki gating w kodzie. UI nie różnicuje variantów infusion przy compat check. |
| L2 | DLC wepType 69/94/95 | Backend allow-passthrough; UI traktuje jako `unknown` i fail-closes widoczność. Brak danych w `WepTypeToCanMountBit` po obu stronach. `needs verification` czy w UI istnieje informacja dla użytkownika, że to "DLC, kompatybilność nieznana". |
| L3 | `gemMountType == 1` (somber) semantyka edycji AoW | UI ustawia `CanMountAoW = false` → sekcja AoW disabled. `needs verification` czy istnieje placeholder/explanation, że to nie błąd. |
| L4 | Frontend ↔ backend `WEP_TYPE_TO_BIT` drift | Dwa frontend mirrory (`WeaponEditModal.tsx`, `WeaponEditTab.tsx`), oba ręcznie podtrzymywane; w aktualnym stanie identyczne z backendem. Brak guardu CI / generatora. `needs verification` przy każdej zmianie backendu. |
| L5 | Compat bitmask completeness | `AoWCompatMasks` jest generowany z `EquipParamGem`; możliwe że nowe DLC rows nie były re-imported. `needs verification` po update'cie regulation. |
| L6 | Orphan AoW GaItem garbage collection | Nie istnieje (celowo). Gra toleruje, strict path może re-attach. `needs verification` w długich workflow user-facing (czy save rośnie liniowo z liczbą AoW edits). |
| L7 | Workspace `populateCurrentAoW` edge cases | Pełna macierz Added × Removed × Pending nie jest opisana tu — pokryta testami w `current_aow_test.go`. `needs verification` dla nowych sources/sinks. |
| L8 | `app.ApplyWeaponAoW` (legacy) future removal | Dokumentowane w kodzie jako "currently invoked only via hidden legacy Weapon Edit tab". Plan cleanupu nie jest w tym dokumencie. |

---

## 23. Cross-references

- [03-gaitem-map](03-gaitem-map.md) — GaItem binary model i handle prefix semantics.
- [06-equipment](06-equipment.md) — read-only equipment relation.
- [07-inventory](07-inventory.md) i [10-storage](10-storage.md) — inventory / storage section model.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — pełne capacity rules (`NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`, post-mutation validator).
- [36-inventory-categories-game-order](36-inventory-categories-game-order.md) — kategoria `ashes_of_war` w mapowaniu inventory.
- [43-transactional-item-adding](43-transactional-item-adding.md) — transakcyjny add z AoW companion flags.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — AoW jako non-stackable; reguły sortowania.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — transfer broni z przypiętym AoW.
- [55-build-template](55-build-template.md) — portable JSON snapshot, AoW jako semantic ItemID.
- [45-ban-risk-reference](45-ban-risk-reference.md) — anty-flag rationale dla emisji kanonicznego sentinela.

---

## 24. Źródła

- Kod: `backend/core/structures.go`, `backend/core/offset_defs.go`, `backend/core/writer.go`, `backend/core/aow_availability.go`, `backend/db/db.go`, `backend/db/data/aow_compat.go`, `backend/db/data/weapon_gem_mount.go`, `backend/editor/weapon.go`, `backend/editor/save.go`, `backend/editor/validate.go`, `backend/editor/workspace.go`, `app.go`, `frontend/src/components/WeaponEditModal.tsx`, `frontend/src/components/WeaponEditTab.tsx`, `frontend/src/components/SortOrderTab.tsx`.
- Testy: `backend/core/aow_strict_test.go`, `backend/core/aow_dual_destination_test.go`, `backend/core/gaitem_placement_test.go`, `backend/editor/current_aow_test.go`, `backend/editor/weapon_test.go`, `backend/editor/save_test.go`, `app_weapon_aow_dlc_test.go`, `app_weapon_aow_editor_test.go`, `frontend/src/components/WeaponEditModal.workspace.test.tsx`.
- Dane gry: `tmp/regulation-bin-dump/csv/EquipParamWeapon.csv` (kolumna `swordArtsParamId`, `wepType`, `gemMountType`), `EquipParamGem.csv` (`swordArtsParamId`, `canMountWep_*`), `SwordArtsParam.csv`.
- Historia / forensic: commity `4e800b9` (`fix(aow): use vanilla no-custom sentinel`), `cb1a822` (`fix(ui): clarify custom Ash of War removal`), `6881cb9` (`fix(core): guard AoW allocation at armament capacity`), `f3d64c1` (`fix(inventory): restore AoW editing in workspace mode`), `0b62cfd` (`feat(inventory): save pending Ashes of War edits`), `8fcc97f` (`feat(inventory): expose current AoW in workspace`).
