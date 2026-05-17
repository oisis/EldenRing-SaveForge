# 54 — Ash of War

> **Type**: Design doc
> **Status**: ✅ Zaimplementowany
> **Scope**: Semantyka wbudowanej (built-in) i zewnętrznej (custom) umiejętności broni w save, sentinele braku custom AoW, invariant unikalności handle, reguły writera/readera oraz sposób, w jaki SaveForge utrzymuje output zgodny z vanilla save'ami gry.

---

## 1. Przegląd

Każda broń w Elden Ring ma umiejętność prezentowaną w grze jako jej **Ash of War**. Umiejętność może pochodzić z dwóch różnych źródeł:

- **Wbudowana / domyślna umiejętność broni** — zdefiniowana w `regulation.bin` per wiersz broni (`EquipParamWeapon.swordArtsParamId`). Zawsze dostępna, nigdy nie zapisywana w save.
- **Zewnętrzny / custom Ash of War gem** — opcjonalny przedmiot z inwentarza (mający własny wpis GaItem), który gracz może przypiąć do broni. Po przypięciu nadpisuje wbudowaną umiejętność własną umiejętnością gem'a (`EquipParamGem.swordArtsParamId`).

SaveForge musi modelować obie warstwy osobno. Pominięcie tej separacji produkuje dwie klasy bugów widocznych dla użytkownika:

- Traktowanie "brak custom AoW" jako "brak skilla w ogóle" — myli użytkownika, że remove niszczy umiejętność broni, czego nie robi.
- Traktowanie dwóch egzemplarzy custom-AoW tego samego gem ItemID jako jednej instancji — prowadzi albo do fałszywych statusów "in use" w UI, albo do dużo groźniejszego shared-handle crash opisanego w §6.

---

## 2. Model danych

### Tabele parametrów gry (regulation.bin, tylko do odczytu)

- `EquipParamWeapon` — jeden wiersz per ItemID broni. Kolumna `swordArtsParamId` wskazuje wiersz **wbudowanej** umiejętności w `SwordArtsParam`. Ta wartość jest fallbackiem, gdy brak custom AoW.
- `EquipParamGem` — jeden wiersz per Ash of War gem. Jej kolumna `swordArtsParamId` wskazuje umiejętność nadawaną przez gem po przypięciu, a `canMountWep_*` udostępnia 36-bitową bitmaskę kompatybilnych wartości `wepType`.
- `SwordArtsParam` — definicje umiejętności (animacje, koszty, FMG textId dla nazwy in-game).

Te trzy tabele żyją w `regulation.bin` (UserData11). SaveForge **nigdy do nich nie pisze** — patrz twarda reguła projektu o regulation.bin w `CLAUDE.md`.

### Layout save'a

- AoW gemy żyją w mapie GaItem slotu jako 8-bajtowe rekordy (patrz [03-gaitem-map](03-gaitem-map.md)):
    - Prefix handle GaItem: `0xC0000000` (ItemTypeAow).
    - Prefix ItemID gem'a AoW: `0x80000000` (np. Lion's Claw = `0x80002710`).
- Każdy rekord weapon GaItem ma 21 bajtów. Offset `0x10` przechowuje pole `u32` — `AoWGaItemHandle` — które referuje przypięty AoW gem po **handle**, nie po ItemID.
- Wiersze inwentarza referują przedmioty po handle; mapa GaItem rozwiązuje handle → ItemID przez `slot.GaMap`.

---

## 3. Rozwiązywanie umiejętności

Gra rozwiązuje aktywną umiejętność przy load używając zarówno save'a, jak i `regulation.bin`. Efektywna umiejętność to:

    if IsValidCustomAoWHandle(weapon.AoWGaItemHandle):
        gem = GaMap[weapon.AoWGaItemHandle]
        skill = EquipParamGem[gem.ItemID].swordArtsParamId
    else:
        skill = EquipParamWeapon[weapon.ItemID].swordArtsParamId

Dwa wnioski warto powiedzieć wprost:

- **Usunięcie custom AoW nie kasuje domyślnej umiejętności.** Odłączenie zewnętrznego gem'a czyści tylko referencję `AoWGaItemHandle`; ItemID broni pozostaje ten sam i gra przy następnym loadzie spada do `EquipParamWeapon.swordArtsParamId`.
- **"Brak custom AoW" nie znaczy "brak skilla".** Większość broni ma niezerowy `swordArtsParamId` (Lordsworn's Straight Sword → 115 "Square Off", Icerind Hatchet → 109 "Hoarfrost Stomp"). UI musi to jasno komunikować — patrz `WeaponEditModal.tsx` po commicie `cb1a822`.

---

## 4. Sentinele braku custom AoW

4 bajty na `weapon GaItem + 0x10` niosą jedną z następujących klas wartości:

| Wartość | Znaczenie | Writer? | Reader? |
|---|---|---|---|
| `0x00000000` | Brak custom AoW — **canonical vanilla sentinel** (gra pisze tę wartość dla każdego świeżo utworzonego rekordu broni). | ✅ kanonizowany do tej wartości | ✅ akceptowany |
| `0xFFFFFFFF` | Brak custom AoW — **legacy SaveForge sentinel** emitowany przez buildy sprzed commita `4e800b9`. | ❌ nigdy nie emitowany | ✅ akceptowany dla kompatybilności |
| `0xC0xxxxxx` (prefix pasuje do `ItemTypeAow`) | Valid custom AoW handle. Musi rozwiązywać się do istniejącego rekordu AoW GaItem. | ✅ zapisywany przy attach | ✅ akceptowany |
| Dowolna inna wartość | Invalid / corrupted. | n/d | flagowany w dalszych warstwach |

Akceptacja obu sentineli pozwala SaveForge otwierać save'y zedytowane starszymi wydaniami bez re-flagowania każdej broni. Emisja pojedynczego sentinela utrzymuje świeżo zapisane save'y nieodróżnialne od vanilla output na tym offsecie, co jest głównym anti-flag concern (patrz [45-ban-risk-reference](45-ban-risk-reference.md) dla szerszego frameworku ryzyka).

---

## 5. Semantyka remove

Usunięcie custom Ash of War to **odłączenie** (detach), nie **usunięcie** (delete):

- 4-bajtowe pole na weapon `[+0x10]` jest nadpisywane kanonicznym sentinelem `0x00000000` w miejscu.
- `Weapon.ItemID` nie jest dotykany. Affinity (np. Heavy Longsword vs Longsword) żyje w ItemID broni i przeżywa remove bez zmian.
- Poprzednio przypięty rekord AoW GaItem zostaje w mapie GaItem slotu. Staje się **orphanem / free copy**, którą gra toleruje i którą strict writer (`PatchWeaponAoWHandle`) może później ponownie przypiąć bez alokacji nowego wpisu GaItem.
- Przy następnym loadzie gry silnik rozwiązuje umiejętność przez gałąź fallback z §3 — wbudowana umiejętność broni pojawia się ponownie w UI gracza.

Toast i tooltip widoczny dla użytkownika po commicie `cb1a822` mówi to wprost: "Custom Ash of War removed — built-in skill restored."

---

## 6. Invariant unikalności handle (shared-handle)

Pojedynczy handle gem'a AoW identyfikuje pojedynczą fizyczną instancję Ash of War. Dwie różne bronie **nigdy** nie mogą referować tego samego non-sentinel `AoWGaItemHandle`. Naruszenie powoduje `EXCEPTION_ACCESS_VIOLATION` w grze przy loadzie.

Zachowanie widoczne dla gracza, które może być pomylone z współdzieleniem handle, jest legalne:

- Ten sam **ItemID** AoW (np. `0x80002710` Lion's Claw) może występować w mapie GaItem jako **wiele osobnych kopii**, każda z własnym handle (`0xC0...`). Tak właśnie kumulują się kopie z Lost Ashes of War, dropów ze skarabeuszy i zwykłych podniesień.
- Dwie bronie mogą obie pokazywać ten sam Ash of War w grze, o ile wskazują na **różne** handle gem'a AoW — czyli dwie odrębne instancje gem'a o tym samym ItemID.

SaveForge wymusza invariant w każdej ścieżce zapisu:

- `core.PatchWeaponAoWHandle` odrzuca każdy attach, w którym żądany AoW handle jest już referowany przez inny rekord broni w tym samym slocie.
- `core.PatchWeaponAoW` alokuje świeży `0xC0...` handle dla każdego wywołania "attach by ItemID" — nigdy nie reużywa istniejącego handle, więc nie może stworzyć konfliktu.
- Ścieżka strict z UI (`App.ApplyWeaponAoWStrict`) wybiera pierwszą **wolną** kopię AoW GaItem (taką, której żadna broń nie referuje) zanim deleguje do `PatchWeaponAoWHandle`.

Forensic audit po wszystkich fixture'ach `tmp/save/ER0000*.sl2` znalazł **zero** przypadków shared-handle (patrz audit z promptu 10).

---

## 7. Kompatybilność i dostępność

`core.ScanAoWAvailability` przechodzi mapę GaItem i emituje jeden `AoWCopyRaw` per instancję gem'a AoW. VM/UI agreguje per ItemID i prezentuje te stany:

| Status | Znaczenie |
|---|---|
| `current` | Ten ItemID AoW jest aktualnie przypięty do edytowanej broni. |
| `available` | Istnieje co najmniej jedna kopia AoW GaItem tego ItemID, której żadna broń nie referuje. |
| `in_use` | Wszystkie kopie tego ItemID AoW są już przypięte do broni. Strict apply nie może działać bez wcześniejszego odłączenia jednej; legacy apply zaalokowałby nowy wpis GaItem. |
| `missing` | W slocie nie ma żadnej kopii AoW GaItem tego ItemID. Strict apply jest niemożliwy, dopóki gracz nie zdobędzie kopii (np. duplikacja Lost Ashes). |
| `conflict` | `HasSharedHandleConflict == true`. Sygnał uszkodzenia save'a; strict path odmawia operacji na tym ItemID. |

Kompatybilność custom AoW z bronią docelową jest niezależna od dostępności i jest liczona z `EquipParamGem.canMountWep_*` względem `EquipParamWeapon.wepType` broni. Mapowanie `wepType → pozycja bitu` żyje w `frontend/src/components/WeaponEditModal.tsx` (`wepTypeToBitPos`) i lustruje stałe `backend/db/data`. AoW niekompatybilny z typem broni jest blokowany na poziomie UI niezależnie od dostępności.

---

## 8. Reguły writera / readera

| Reguła | Writer | Reader |
|---|---|---|
| Akceptuj zarówno `0x00000000` jak i `0xFFFFFFFF` jako no-custom na wejściu. | tak (kanonizuje do `0x00000000`) | tak |
| Emituj tylko `0x00000000` dla no-custom / remove. | tak | n/d |
| Emituj ItemID gem'a AoW zamiast handle. | **nigdy** — pole jest handle, nie ItemID | n/d |
| Odrzuć `newAoWHandle`, którego prefix nie jest `0xC0000000` (i nie jest sentinel'em no-custom). | tak | n/d |
| Odrzuć `newAoWHandle`, który nie wskazuje na istniejący rekord AoW GaItem. | tak | n/d |
| Odrzuć `newAoWHandle` już referowany przez inną broń (shared-handle guard). | tak | n/d — raportowany jako conflict w availability |
| Zachowaj `Weapon.ItemID` przez każde remove. | tak | n/d |
| `PatchWeaponAoW` (legacy) może alokować świeży `0xC0...` handle. | tak | n/d |
| `PatchWeaponAoWHandle` (strict) reużywa istniejący wolny handle, nigdy nie alokuje. | tak | n/d |

---

## 9. Odniesienia do kodu

| Zagadnienie | Plik | Symbol |
|---|---|---|
| Stałe i helper | `backend/core/structures.go` | `NoCustomAoWHandle`, `LegacyNoCustomAoWHandle`, `IsNoCustomAoWHandle`, `GaItemFull.AoWGaItemHandle` |
| Strict in-place patch | `backend/core/writer.go` | `PatchWeaponAoWHandle` (kanonizuje wejście, shared-handle guard) |
| Legacy alloc-and-attach | `backend/core/writer.go` | `PatchWeaponAoW` (remove pisze `NoCustomAoWHandle`; attach alokuje świeży handle) |
| Alokacja nowej broni | `backend/core/writer.go` | `allocateGaItem` (inicjalizuje `AoWGaItemHandle: NoCustomAoWHandle`) |
| Skan dostępności | `backend/core/aow_availability.go` | `ScanAoWAvailability`, `AoWCopyRaw` |
| Resolver UI | `backend/vm/character_vm.go` | gałąź AoW w `mapItems` (używa `core.IsNoCustomAoWHandle`) |
| Punkty wejścia poziomu App | `app.go` | `ApplyWeaponAoWStrict`, `ApplyWeaponAoW`, `GetAoWAvailability` |
| Tekst UI i UX remove | `frontend/src/components/WeaponEditModal.tsx` | Nagłówek "Default skill", tooltip Remove, toast o przywróceniu built-in skilla |

---

## 10. Historia / forensic notes

- Forensic scan `tmp/save/ER0000-kro55-vanilla.sl2` oraz niemodyfikowanych slotów `tmp/save/ER0000.sl2`: **każda** broń bez custom AoW przechowywała `0x00000000` na offsecie `[+0x10]`. Gra sama nigdy nie emituje `0xFFFFFFFF` dla tego pola.
- Slot 4 w `tmp/save/ER0000-out.sl2` (slot edytowany starszym SaveForge bulk-add): histogram mieszany — 196 broni z `0xFFFFFFFF` i 21 z `0x00000000`. Gra tolerowała tę mieszankę przy load (brak crashu), ale output nie był już vanilla-aligned.
- Audit wszystkich dostępnych save fixtures pokazał `DUPLICATE_AOW_HANDLE = 0` w każdym slocie, potwierdzając że invariant shared-handle w praktyce się utrzymał.
- Commit `4e800b9` (`fix(aow): use vanilla no-custom sentinel`) sprawił, że writer emituje `0x00000000` wszędzie, a każda ścieżka reader/availability toleruje oba sentinele przez `IsNoCustomAoWHandle`.
- Commit `cb1a822` (`fix(ui): clarify custom Ash of War removal`) zastąpił mylący nagłówek "None" na "Default skill", doprecyzował tooltip Remove i toast tak, by wspominały o przywróceniu built-in skilla, oraz dodał inline objaśnienie.

---

## Źródła

- Kod: `backend/core/structures.go`, `backend/core/writer.go`, `backend/core/aow_availability.go`, `backend/vm/character_vm.go`, `app.go`, `frontend/src/components/WeaponEditModal.tsx`.
- Testy: `backend/core/aow_strict_test.go` (suite regresyjny sentinela — `TestPatchWeaponAoWHandle_RemoveWritesZeroSentinel`, `TestAllocateGaItem_NewWeaponUsesZeroSentinel`, `TestScanAoWAvailability_ZeroSentinelNotCounted`, `TestScanAoWAvailability_LegacyFFFFFFFFSentinelNotCounted`, `TestIsNoCustomAoWHandle`, `TestPatchWeaponAoWHandle_RemovePreservesWeaponItemID`, `TestPatchWeaponAoW_LegacyRemoveWritesZeroSentinel`).
- Dane gry: `tmp/regulation-bin-dump/csv/EquipParamWeapon.csv` (kolumna 191 `swordArtsParamId`, 196 `wepType`, 248 `gemMountType`), `EquipParamGem.csv` (kolumna 12 `swordArtsParamId`, plus bitmaska `canMountWep_*`), `SwordArtsParam.csv`.
- Forensic fixture'y: `tmp/save/ER0000.sl2`, `tmp/save/ER0000-kro55-vanilla.sl2`, `tmp/save/ER0000-out.sl2`, `tmp/save/ER0000-kro55-out.sl2`.
- Powiązane spece: [03-gaitem-map](03-gaitem-map.md), [06-equipment](06-equipment.md), [53-inventory-storage-transfer](53-inventory-storage-transfer.md), [45-ban-risk-reference](45-ban-risk-reference.md).
- Historia: commity `4e800b9`, `cb1a822` na branchu `chore/items-audit`.
