# 29 — Czarne kafelki DLC (warstwa zakrywająca mapę)

## Przegląd

DLC "Shadow of the Erdtree" używa systemu "Hard Blackout" — czarnych kafelków, które
całkowicie zakrywają obszar mapy DLC, dopóki gracz fizycznie go nie zbada. Jest to
oddzielne od Fog of War (FoW) i flag widoczności fragmentów mapy.

## Trójwarstwowy system mapy

| Warstwa | Nazwa | Mechanizm | Wsparcie edytora |
|---------|-------|-----------|------------------|
| 1. Cover Layer | Czarne kafelki | **Sekcja BloodStain** (afterRegs+0x0088..0x0110) | WIP — ta specyfikacja |
| 2. Basic Topography | Szary szkic | Automatyczne (pod warstwą 1) | n/d |
| 3. Detailed Bitmap | Kolorowa mapa | Event flags 62xxx + przedmioty Map Fragment | ✅ działa |

FoW (afterRegs+0x087E..0x10B0) to oddzielny system dodający szarą mgłę.
Podstawowa gra ma Cover Layer domyślnie przezroczysty. DLC ma pełne zaciemnienie.

## Wyniki przeszukiwania binarnego (2025-04-25)

Systematycznie zawężono 117 342-bajtową sekcję [afterRegs..EventFlags]:

| Test | Zakres (od afterRegs) | Rozmiar | Czarne kafelki? |
|------|------------------------|---------|-----------------|
| 7 | Kopia pełnej sekcji | 117 342 B | ✅ BEZ kafelków |
| 8 | 0x10B1..0x1C639 (menuProfile) | 112 008 B | ❌ kafelki obecne |
| 9 | 0x0000..0x087E + 0x1C639..end (A+D) | 3 235 B | ✅ BEZ kafelków |
| 10 | 0x0000..0x087E (tylko A) | 2 174 B | ✅ BEZ kafelków |
| 11 | 0x0000..0x0440 (A1) | 1 088 B | ✅ BEZ kafelków |
| 12 | 0x0000..0x0220 (A1a) | 544 B | ✅ BEZ kafelków |
| 13 | 0x0000..0x0110 | 272 B | ✅ BEZ kafelków |
| 14 | 0x0000..0x0088 | 136 B | ❌ kafelki obecne |
| 15 | 0x0088..0x0110 | 136 B | ✅ BEZ kafelków |
| 16 | 0x0088..0x00CC | 68 B | ❌ kafelki tylko w DLC |
| 17 | 0x00CC..0x0110 | 68 B | ❌ częściowe kafelki + artefakt FoW |
| 18 | 0x0085..0x0110 wyzerowane | 139 B | ❌ kafelki obecne (zerowanie nie działa) |

**Wniosek:** Krytyczne dane znajdują się pod **afterRegs+0x0088..0x0110** (136 bajtów),
wewnątrz sekcji BloodStain. Obie połówki (0x0088..0x00CC i 0x00CC..0x0110) są
potrzebne razem — żadna z osobna nie usuwa w pełni czarnych kafelków.

Zerowanie zakresu NIE działa — gra potrzebuje **konkretnych wartości** (współrzędne/stan)
z save'a, który ma zbadane DLC.

## Struktura danych pod afterRegs+0x0088..0x0110

Ten 136-bajtowy zakres znajduje się wewnątrz sekcji BloodStain (afterRegs+0x0075..0x10B1).
Wydaje się zawierać **dwa rekordy pozycji/stanu**:

### Record 1 (afterRegs+0x0085..0x00C4)
```
+0x0085: u32  — nieznane (ref: 0x00000000)
+0x0089: u32  — nieznane (ref: 0x00000000)  
+0x008D: f32  — współrzędna X (ref: 9648.0 = obszar DLC)
+0x0091: f32  — współrzędna Y (ref: 9123.8 = obszar DLC)
+0x0095: u8   — flaga (ref: 0x01)
+0x0096..0x00C4: padding/dodatkowe dane (głównie zera w ref)
```

### Record 2 (afterRegs+0x00C5..0x00D5)
```
+0x00C5: f32  — współrzędna X (ref: 3037.0)
+0x00C9: f32  — współrzędna Y (ref: 1869.0)
+0x00CD: f32  — współrzędna Z (ref: 7880.0)
+0x00D1: f32  — współrzędna W (ref: 7803.0)
+0x00D5: u8   — flaga (ref: 0x01, clean: 0x00)
```

**Kluczowa różnica:** Ref ma współrzędne z obszaru DLC; clean ma współrzędne z podstawowej gry.
Gdy wartości ref są skopiowane, gra renderuje mapę tak, jakby gracz zbadał DLC.

## Co NIE kontroluje czarnych kafelków

Wyczerpująco przetestowane i potwierdzone jako nieodpowiedzialne:

| Element | Przetestowany | Wynik |
|---------|---------------|-------|
| Event flags 62080-62084 | ✅ | Przeżywają wczytanie gry, brak wpływu na kafelki |
| Event flags 62xxx (269 flag) | ✅ | Wszystkie przeżywają, brak wpływu |
| Discovery flags 60xxx, 61xxx | ✅ | Przeżywają, brak wpływu |
| FoW bitfield 0xFF | ✅ | Usuwa tylko FoW, nie kafelki |
| FoW bitfield 0x00 | ✅ | Przywraca FoW, brak wpływu na kafelki |
| FoW bitfield z ref | ✅ | Ten sam wzorzec, brak wpływu na kafelki |
| Przedmioty Map Fragment | ✅ | Przeżywają w ekwipunku, brak wpływu |
| Flagi DLC grace | ✅ | Przeżywają, brak wpływu |
| CsDlc byte[1] (0x30, 0x80) | ✅ | Gra resetuje lub modyfikuje, brak wpływu |
| Unlocked regions (395) | ✅ | Przeżywają, same w sobie brak wpływu |
| menuProfile (112KB) | ✅ | Brak wpływu |
| gaItemsOther + tutorialData + ingameTimer (1KB) | ✅ | Brak wpływu |

## Referencja układu sekcji

```
afterRegs + 0x0000                = start (po unlocked regions)
afterRegs + 0x0029                = koniec sekcji horse
afterRegs + 0x006D                = clearCount  
afterRegs + 0x0075                = początek bloodStain
afterRegs + 0x0088..0x0110        = *** DANE CZARNYCH KAFELKÓW *** (136 bajtów)
afterRegs + 0x087E                = początek bitfield FoW
afterRegs + 0x10B0                = koniec bitfield FoW
afterRegs + 0x10B1                = początek menuProfile
afterRegs + 0x10B1 + 0x1B588     = gaItemsOther
afterRegs + gaItemsOther + 0x40B  = tutorialData
afterRegs + tutorialData + 0x1A   = ingameTimer  
afterRegs + ingameTimer + 0       = EventFlags
```

## ROZWIĄZANIE (Test 19 — potwierdzone działające)

Syntetyczne wartości usuwające czarne kafelki DLC. Wyzeruj 0x0085..0x0110, potem zapisz:

### Record 1 (afterRegs+0x008D)
```
+0x008D: f32 = 9648.0   (X — centrum mapy DLC)
+0x0091: f32 = 9124.0   (Y — centrum mapy DLC)
+0x0095: u8  = 0x01     (flaga — "odwiedzone")
```

### Record 2 (afterRegs+0x00C5)
```
+0x00C5: f32 = 3037.0   (X)
+0x00C9: f32 = 1869.0   (Y)
+0x00CD: f32 = 7880.0   (Z)
+0x00D1: f32 = 7803.0   (W)
+0x00D5: u8  = 0x01     (flaga — "odwiedzone")
```

Te współrzędne odpowiadają obszarowi DLC overworld. Gra używa ich do określenia,
które kafelki mapy renderować jako "odkryte". Sloty 0 i 1 (oba z ukończonym DLC)
mają identyczne wartości.

### Implementacja

```go
func removeDLCBlackTiles(slot *SaveSlot) {
    storageEnd := slot.StorageBoxOffset + DynStorageBox
    gesturesOff := storageEnd + DynStorageToGestures
    regCount := readU32(slot.Data, gesturesOff)
    afterRegs := gesturesOff + 4 + regCount*4

    // Zero out bloodstain position data
    for i := afterRegs + 0x0085; i < afterRegs + 0x0110; i++ {
        slot.Data[i] = 0x00
    }

    // Record 1: DLC map center coordinates
    putF32(slot.Data, afterRegs+0x008D, 9648.0)
    putF32(slot.Data, afterRegs+0x0091, 9124.0)
    slot.Data[afterRegs+0x0095] = 0x01

    // Record 2: DLC area coordinates
    putF32(slot.Data, afterRegs+0x00C5, 3037.0)
    putF32(slot.Data, afterRegs+0x00C9, 1869.0)
    putF32(slot.Data, afterRegs+0x00CD, 7880.0)
    putF32(slot.Data, afterRegs+0x00D1, 7803.0)
    slot.Data[afterRegs+0x00D5] = 0x01
}
```

### Weryfikacja między slotami

Slot 0 (ukończona gra podstawowa, ukończone DLC) i slot 1 (w pełni zbadane DLC)
mają **identyczne** wartości w tym zakresie. Świeży slot 4 ma współrzędne
z podstawowej gry i wartości sentinel -1.0.

## Pozostałe prace

1. Naprawić częściowy FoW — zerowanie 0x0085..0x0088 może nachodzić na sąsiednie dane
2. Ustalić, czy te współrzędne wpływają na mapę podstawowej gry (nie powinny — Cover Layer
   podstawowej gry jest domyślnie przezroczysty)
3. Zbadać, co dokładnie reprezentują te współrzędne (ostatnia plama krwi? punkt odrodzenia?
   czy kotwica odkrywania mapy?)
4. Przetestować z różnymi współrzędnymi DLC, aby sprawdzić czy konkretne wartości mają
   znaczenie, czy wystarczy "dowolna współrzędna z obszaru DLC"
