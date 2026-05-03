# 30 — Slot Rebuild: Analiza luzu (R-1 Krok 3)

**Status:** wyniki badań; informuje Stage 2 funkcji Invasion Regions.
**Wygenerowane przez:** `backend/core/slot_slack_test.go::TestSlotSlackAnalysis`
**Data:** 2026-04-26

## Cel

Zmierzyć ile **końcowego zerowego luzu** (trailing zero slack) istnieje między
blokiem `unlocked_regions` a końcem każdego slotu save. Hybrydowy rebuild bloba
(`backend/core/slot_rebuild.go`) absorbuje deltę ze zmutowanego slajsa
`UnlockedRegions` przez przycinanie lub dopełnianie zerami bloba
`post_unlocked_regions`. Jeśli końcowe bajty tego bloba są niezerowe,
przycinanie uszkadza save.

`SlotSize = 0x280000`, `DlcSectionOffset = 0x27FF4E`, `HashOffset = 0x27FF80`.

## Surowe pomiary

| save                | slot | version | regions | regs_end   | post_blob_sz | trail_zero | extra_fits | last_nz_pre_DLC | last_nz_pre_hash | last_nz_all |
|---------------------|------|---------|---------|------------|--------------|------------|------------|-----------------|------------------|-------------|
| oisis_pl-org.txt    | 0    | 230     | 354     | 0x1EE8C    | 2,494,658    | 0          | 0          | 0x27FF4D        | 0x27FF7F         | 0x27FFFF    |
| oisis_pl-org.txt    | 1    | 251     | 6       | 0x19A51    | 2,516,221    | 0          | 0          | 0x27FF4D        | 0x27FF7E         | 0x27FFFF    |
| oisisk_ps4.txt      | 0    | 150     | 4       | 0x19891    | 2,516,669    | 0          | 0          | 0x27FF4D        | 0x27FF7D         | 0x27FFF9    |
| oisisk_ps4.txt      | 1    | 150     | 4       | 0x19889    | 2,516,677    | 1          | 0          | 0x27FF4C        | 0x27FF7C         | 0x27FFFC    |
| oisisk_ps4.txt      | 2    | 150     | 4       | 0x19889    | 2,516,677    | 41         | 10         | 0x27FF24        | 0x27FF24         | 0x27FFE4    |
| oisisk_ps4.txt      | 3    | 150     | 6       | 0x19A49    | 2,516,229    | 1          | 0          | 0x27FF4C        | 0x27FF7F         | 0x27FFFC    |
| ER0000.sl2 (PC)     | 0    | 250     | 319     | 0x1C2B3    | 2,505,883    | 0          | 0          | 0x27FF4D        | 0x27FF7D         | 0x27FFF9    |
| ER0000.sl2 (PC)     | 1    | 251     | 395     | 0x1C328    | 2,505,766    | 102        | 25         | 0x27FEE7        | 0x27FF7F         | 0x27FFF3    |
| ER0000.sl2 (PC)     | 2    | 230     | 29      | 0x19D50    | 2,515,454    | 6          | 1          | 0x27FF47        | 0x27FF7C         | 0x27FFFB    |
| ER0000.sl2 (PC)     | 3    | 250     | 110     | 0x1A97A    | 2,512,340    | 164        | 41         | 0x27FEA9        | 0x27FEA9         | 0x27FFEB    |
| ER0000.sl2 (PC)     | 4    | 251     | 6       | 0x199F3    | 2,516,315    | 9          | 2          | 0x27FF44        | 0x27FF7C         | 0x27FFF6    |

`extra_fits = trailing_zeros / 4` — każdy region ID to 4 bajty.

## Wnioski

1. **Luz jest skrajnie zmienny** — od 0 do 164 końcowych zerowych bajtów
   w blobie `post_unlocked_regions`. Pięć slotów z jedenastu ma
   **zerowy użyteczny luz**: jakikolwiek wzrost `unlocked_regions` nadpisuje
   żywe bajty.

2. **`last_nz_pre_DLC` to prawie zawsze 0x27FF4D**, tj. jeden bajt przed
   `DlcSectionOffset`. Blob `post_unlocked_regions` styka się bezpośrednio
   z sekcją DLC w większości save'ów — podejście
   "przesuń-wszystko-o-N-bajtów", które próbowaliśmy w Stage 2, nie miało
   miejsca na absorpcję przesunięcia.

3. **Bajty sekcji DLC [3..49] nie są zerowe w naszych save'ach** —
   `last_nz_pre_hash` trafia w zakres DLC (0x27FF4E..0x27FF7F) dla
   8 z 11 slotów, co przeczy starszemu założeniu, że "bajty DLC
   [3..49] muszą być 0x00". `oisisk_ps4.txt slot 2` to wyjątek
   (cała sekcja DLC zerowa), co sugeruje, że zawartość DLC jest *wypełniana
   w miarę postępu gracza*, a nie statycznie zerowa.

4. **Maksymalny luz w testowanej flocie to 41 dodatkowych regionów**
   (`ER0000.sl2 slot 3`, 164 końcowe zera). Stage 2 musi móc dodać
   do 78 regionów ("Unlock All"). Dla 5 z 11 slotów **nawet jeden
   dodatkowy region nie mieści się**. Hybrydowe przycinanie jest zatem
   niewystarczające dla funkcji widocznej dla użytkownika.

## Implikacje dla R-1

Podejście z hybrydowym blobem (obecna implementacja w
`backend/core/slot_rebuild.go`) **nie jest w stanie dostarczyć Stage 2
zgodnie ze specyfikacją**. Zadziała dla `ER0000.sl2 slot 3` (41 dodatkowych
regionów), częściowo dla kilku innych slotów PC, i całkowicie zawiedzie
na wszystkich slotach PS4.

Dwie wykonalne ścieżki naprzód:

### Opcja A — Konserwatywny Stage 2 (pragmatyczny)

Zachować hybrydowy rebuild. Przy zapisie obliczać efektywny luz
(`trailing_zeros / 4`) i odrzucać mutacje, które by go przekroczyły,
z jasnym komunikatem: "save ma miejsce na N dodatkowych regionów;
proszę najpierw usunąć kilka." Frontend blokowałby "Unlock All" za
sprawdzeniem pojemności per-slot.

- **Zalety:** gotowe w dni, bez dodatkowej pracy nad parserem.
- **Wady:** "Unlock All" de facto nigdy nie działa dla świeżych / ciężkich
  save'ów (5/11 slotów nie może dodać nawet jednego regionu). Mylący UX.

### Opcja B — Pełny rebuild struktur (odpowiada er-save-manager)

Sparsować każdą sekcję po `unlocked_regions` (horse, blood_stain,
menu_profile_save_load, gaitem_game_data, tutorial_data, event_flags,
field_area, world_area, world_geom_man, world_geom_man2, rend_man,
player_coordinates, net_man, weather, time, base_version, steam_id,
ps5_activity, dlc, hash) do typowanych struktur. Przy rebuild serializować
każdą w jej **oryginalnym rozmiarze** w kolejności; slot kończy się
dopełnieniem zerami, które absorbuje dowolną deltę `unlocked_regions`.

- **Zalety:** solidne; odpowiada sprawdzonemu referencyjnemu Pythonowi;
  obsługuje dowolną rozsądną liczbę regionów.
- **Wady:** ~30 mini-parserów + serializerów; sekcje z prefiksem rozmiaru
  (`field_area`, `world_area`, `world_geom_man`, `world_geom_man2`,
  `rend_man`) mają nieudokumentowane wewnętrzne layouty po naszej stronie —
  muszą być traktowane jako nieprzezroczyste bloby `(size: u32, data: bytes)`
  co najmniej. Szacowany nakład pracy 15–25 h.

### Opcja C — Hybrid z rozluźnieniem zakotwiczonego ogona

Pośrednie rozwiązanie: zachować obecny model sekcji, ale **przestać
przypinać DLC i hash na koniec slotu**. Zamiast tego umieścić blob
post-regions, potem DLC, potem hash, potem dopełnić zerami do `SlotSize`.
Opiera się to na hipotezie, że pozycje DLC i hash *nie są* stałymi offsetami,
ale raczej "gdziekolwiek kończą się poprzednie sekcje". Weryfikacja wymaga
ręcznego spreparowania save'a z przesuniętym offsetem DLC i potwierdzenia,
że gra go wczytuje.

- **Zalety:** mała delta kodu (~50 LOC zmian w `RebuildSlot`), brak nowych
  parserów.
- **Wady:** niezweryfikowana hipoteza; jeśli błędna, save psuje się tak samo
  jak w Stage 2. **Pojedynczy test na Steam Decku sfalsyfikowałby lub
  potwierdził ją.**

## Rekomendacja

**Najpierw przetestować Opcję C** — kosztuje jeden dzień i jeden round-trip
na Steam Decku. Jeśli gra akceptuje save z DLC pod niekanonicznym offsetem,
możemy dostarczyć Stage 2 z minimalnym dodatkowym kodem. Jeśli odrzuci save,
wracamy do Opcji B (pełny rebuild struktur) i planujemy budżet odpowiednio.

Decyzja śledzona w `ROADMAP.md` pod wpisem
"Invasion Regions Toggle" → Stage 2.

---

## Aktualizacja 2026-04-26 — oryginalne pomiary były mylące

Po zaimplementowaniu pełnego parsera sekwencyjnego (R-1 Kroki 4–13), rzeczywisty
obraz luzu okazał się znacznie korzystniejszy niż sugerowała ta sekcja.
Oryginalny pomiar patrzył na lukę między `unlocked_regions_end` a **stałym**
`DlcSectionOffset = 0x27FF4E`. Ten offset to *konwencja* — każdy save jaki mamy
umieszcza DLC dokładnie pod `SlotSize - 0xB2`, ale nie jest to przypięty
offset po stronie gry.

Pełny parser dociera do końca danych w okolicy bajtu ~2.2 MB dla każdego aktywnego
slotu (PS4 + PC), zostawiając **408KB–432KB zerowego dopełnienia na końcu** przed
końcem 0x280000-bajtowego slotu. DLC to po prostu przedostatnia sekcja o stałym
rozmiarze *wewnątrz* `TrailingFixedBlock`; to co wyglądało na "przypiętą" pozycję
DLC jest po prostu miejscem, w którym sekcja ląduje w typowym save'ie.

| save             | slot | parsed (pos) | tail rest |
|------------------|------|--------------|-----------|
| oisis_pl-org PS4 | 0    | 2,212,520    | 408,920   |
| oisis_pl-org PS4 | 1    | 2,189,717    | 431,723   |
| ER0000.sl2 PC    | 0    | 2,202,359    | 419,081   |
| ER0000.sl2 PC    | 1    | 2,204,780    | 416,660   |
| ER0000.sl2 PC    | 2    | 2,189,228    | 432,212   |
| ER0000.sl2 PC    | 3    | 2,196,222    | 425,218   |
| ER0000.sl2 PC    | 4    | 2,188,767    | 432,673   |

**Implikacja:** `RebuildSlot` (Krok 13) po którym następuje
`SetUnlockedRegions` (Krok 14) obsługuje dodanie ~100 000 regionów na każdym
przetestowanym slocie — Stage 2 jest nieograniczony dla realistycznego
wejścia (≤ 78 regionów). Test na Steam Decku w Kroku 15 nadal blokuje
ścieżkę, ale budżet "Opcja B kosztuje 15–25 h" zwrócił się: teraz mamy
pełne sekwencyjne parsery dla każdej sekcji, gotowe na przyszłe funkcje
weather/teleport/itp.
