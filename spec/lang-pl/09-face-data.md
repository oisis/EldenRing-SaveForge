# 09 — Face Data (Kreator Postaci)

> **Zakres**: Parametry wyglądu postaci z kreatora — twarz, ciało, kolory, proporcje.

---

## Opis ogólny

Face Data to blok 303 bajtów (0x12F) zawierający wszystkie parametry wyglądu postaci ustawione w kreatorze. Występuje w dwóch wariantach:
- **W slocie**: 0x12F (303 bytes) — pełne dane
- **W ProfileSummary (UserData10)**: 0x120 (288 bytes) — skrócone (bez ostatnich 15 bajtów)

---

## Struktura ogólna

```
┌─────────────────────────────────────────────┐
│ Face Model IDs (8 × 4B = 32 bytes)           │  0x00–0x1F
├─────────────────────────────────────────────┤
│ Face Shape Parameters (~64 × u8)             │  0x20–0x5F (przybliżone)
├─────────────────────────────────────────────┤
│ Hair & Cosmetics (~30 × u8)                  │  0x60–0x7F (przybliżone)
├─────────────────────────────────────────────┤
│ Skin Colors & Body (~40 × u8)                │  0x80–0xAF (przybliżone)
├─────────────────────────────────────────────┤
│ Body Scale (7 × float? / byte?)              │  0xB0+ (przybliżone)
├─────────────────────────────────────────────┤
│ Trailing bytes (slot-only, 15B)              │  0x120–0x12E
└─────────────────────────────────────────────┘
```

**UWAGA**: Offsety poniżej pochodzą z Cheat Engine (runtime memory), gdzie Face Data zaczyna się na PlayerGameData+0x754. W pliku save offsety wewnątrz bloku Face Data mogą się różnić — wymaga weryfikacji hex dumpem. Kolejność i nazwy pól są potwierdzone.

---

## Face Model IDs (8 × u32 = 32 bytes)

| Offset (CT) | Typ | Pole | Opis |
|---|---|---|---|
| +0x00 | u32 | Face_Model_Id | Model bazowy twarzy |
| +0x04 | u32 | Hair_Model_Id | Fryzura (wartości = ID z bazy) |
| +0x08 | u32 | Eye_Model_Id | Model oczu |
| +0x0C | u32 | Eyebrow_Model_Id | Model brwi |
| +0x10 | u32 | Beard_Model_Id | Model zarostu |
| +0x14 | u32 | Accessories_Model_Id | Akcesoria (kolczyki, makijaż 3D) |
| +0x18 | u32 | Decal_Model_Id | Decal (tatuaż/blizna) |
| +0x1C | u32 | Eyelash_Model_Id | Model rzęs |

---

## Face Shape Parameters (u8 każdy, zakres 0–255)

Wartość `128` = neutralna/środkowa pozycja suwaka. Wartości poniżej/powyżej przesuwają w przeciwne strony.

### Ogólne proporcje twarzy

| Offset (CT: base 0x740) | Pole | Opis |
|---|---|---|
| +0x34 | Apparent Age | Pozorny wiek (0=młody, 255=stary) |
| +0x35 | Facial Aesthetic | Estetyka twarzy (ogólna "atrakcyjność") |
| +0x36 | Form Emphasis | Wyrazistość rysów (ostrzejsze vs łagodniejsze) |
| +0x37 | Unk (Numen = 128) | Nieznane — domyślna wartość 128, powiązane z rasą Numen? |

### Brwi (Brow Ridge)

| Offset | Pole | Opis |
|---|---|---|
| +0x38 | Brow Ridge Height | Wysokość łuku brwiowego |
| +0x39 | Inner Brow Ridge | Wewnętrzna część łuku brwi |
| +0x3A | Outer Brow Ridge | Zewnętrzna część łuku brwi |

### Kości policzkowe (Cheekbones)

| Offset | Pole | Opis |
|---|---|---|
| +0x3B | Cheekbone Height | Wysokość kości policzkowej |
| +0x3C | Cheekbone Depth | Głębokość (przód-tył) |
| +0x3D | Cheekbone Width | Szerokość |
| +0x3E | Cheekbone Protrusion | Wystanie kości policzkowej |
| +0x3F | Cheeks | Policzki (pełność/wklęsłość) |

### Broda (Chin)

| Offset | Pole | Opis |
|---|---|---|
| +0x40 | Chin Tip Position | Pozycja czubka brody |
| +0x41 | Chin Length | Długość brody |
| +0x42 | Chin Protrusion | Wysunięcie brody do przodu |
| +0x43 | Chin Depth | Głębokość brody |
| +0x44 | Chin Size | Rozmiar brody |
| +0x45 | Chin Height | Wysokość brody |
| +0x46 | Chin Width | Szerokość brody |

### Oczy (Eyes)

| Offset | Pole | Opis |
|---|---|---|
| +0x47 | Eye Position | Pozycja oczu (wysokość) |
| +0x48 | Eye Size | Rozmiar oczu |
| +0x49 | Eye Slant | Skos oczu (góra-dół na krawędziach) |
| +0x4A | Eye Spacing | Rozstaw oczu |

### Nos (Nose) — 14 parametrów

| Offset | Pole | Opis |
|---|---|---|
| +0x4B | Nose Size | Ogólny rozmiar nosa |
| +0x4C | Nose/Forehead Ratio | Proporcja nos–czoło |
| +0x4D | Unk | Nieznany parametr nosa |
| +0x66 | Nose Ridge Depth | Głębokość grzbietu nosa |
| +0x67 | Nose Ridge Length | Długość grzbietu |
| +0x68 | Nose Position | Pozycja nosa |
| +0x69 | Nose Tip Height | Wysokość czubka nosa |
| +0x6A | Nostril Slant | Skos nozdrzy |
| +0x6B | Nostril Size | Rozmiar nozdrzy |
| +0x6C | Nostril Width | Szerokość nozdrzy |
| +0x6D | Nose Protrusion | Wysunięcie nosa |
| +0x6E | Nose Bridge Height | Wysokość nasady nosa |
| +0x6F | Nose Bridge Protrusion 1 | Wysunięcie nasady (górna) |
| +0x70 | Nose Bridge Protrusion 2 | Wysunięcie nasady (dolna) |
| +0x71 | Nose Bridge Width | Szerokość nasady |
| +0x72 | Nose Height | Ogólna wysokość nosa |
| +0x73 | Nose Slant | Skos nosa |

### Proporcje twarzy (Face General)

| Offset | Pole | Opis |
|---|---|---|
| +0x4E | Face Protrusion | Wysunięcie twarzy (profil) |
| +0x4F | Vertical Face Ratio | Proporcja pionowa twarzy |
| +0x50 | Facial Feature Slant | Skos rysów twarzy |
| +0x51 | Horizontal Face Ratio | Proporcja pozioma |
| +0x52 | Unk | Nieznany |
| +0x53 | Forehead Depth | Głębokość czoła |
| +0x54 | Forehead Protrusion | Wysunięcie czoła |
| +0x55 | Unk | Nieznany |

### Szczęka (Jaw)

| Offset | Pole | Opis |
|---|---|---|
| +0x56 | Jaw Protrusion | Wysunięcie szczęki |
| +0x57 | Jaw Width | Szerokość szczęki |
| +0x58 | Lower Jaw | Dolna szczęka |
| +0x59 | Jaw Contour | Kontur szczęki |

### Usta (Mouth/Lips)

| Offset | Pole | Opis |
|---|---|---|
| +0x5A | Lip Shape | Kształt ust |
| +0x5B | Lip Size | Rozmiar ust |
| +0x5C | Lip Fullness | Pełność ust |
| +0x5D | Mouth Expression | Wyraz ust (uśmiech/grymas) |
| +0x5E | Lip Protrusion | Wysunięcie ust |
| +0x5F | Lip Thickness | Grubość warg |
| +0x60 | Mouth Protrusion | Wysunięcie okolicy ust |
| +0x61 | Mouth Slant | Skos ust |
| +0x62 | Mouth Occlusion | Zamknięcie/otwarcie ust |
| +0x63 | Mouth Position | Pozycja ust (pionowa) |
| +0x64 | Mouth Width | Szerokość ust |
| +0x65 | Mouth-Chin Distance | Odległość usta–broda |

---

## Skin & Cosmetics (u8 każdy)

| Pole | Opis | Zakres |
|---|---|---|
| Skin_Color_R | Kolor skóry — Red | 0–255 |
| Skin_Color_G | Kolor skóry — Green | 0–255 |
| Skin_Color_B | Kolor skóry — Blue | 0–255 |
| Skin_Color_A | Kolor skóry — Alpha/Intensity | 0–255 |
| Skin_Pores | Widoczność porów skóry | 0–255 |
| Beard_Stubble | Zarost (stubble overlay) | 0–255 |
| Skin_Dark_Circle | Cienie pod oczami (intensywność) | 0–255 |
| Skin_Dark_Circle_Color_R/G/B | Kolor cieni pod oczami | 0–255 |
| Cheeks | Rumieniec na policzkach | 0–255 |
| Cheeks_Color_R/G/B | Kolor rumieńca | 0–255 |
| Eyeliner | Eyeliner (intensywność) | 0–255 |
| Eyeliner_Color_R/G/B | Kolor eyelinera | 0–255 |
| Eyeshadow_Lower | Cień dolny (intensywność) | 0–255 |
| Eyeshadow_Lower_Color_R/G/B | Kolor cienia dolnego | 0–255 |
| Eyeshadow_Upper | Cień górny (intensywność) | 0–255 |
| Eyeshadow_Upper_Color_R/G/B | Kolor cienia górnego | 0–255 |
| Lipstick | Szminka (intensywność) | 0–255 |
| Lipstick_Color_R/G/B | Kolor szminki | 0–255 |
| Decal_Position_X | Pozycja decalu/tatuażu X | 0–255 |
| Decal_Position_Y | Pozycja decalu/tatuażu Y | 0–255 |
| Body_Hair | Owłosienie ciała (intensywność) | 0–255 |
| Body_Hair_Color_R/G/B | Kolor owłosienia ciała | 0–255 |

---

## Body Scale (7 parametrów)

W pamięci (CT): float (4B każdy) na offsetach 0x870–0x888 od PlayerGameData base.
W pliku save: prawdopodobnie też float lub u8 (do weryfikacji).

| Pole | Opis | Wartość domyślna |
|---|---|---|
| Head | Proporcje głowy | 1.0 (float) / 128 (u8) |
| Chest (Breast) | Proporcje klatki piersiowej | 1.0 / 128 |
| Abdomen (Waist) | Proporcje brzucha/talii | 1.0 / 128 |
| Arm Right | Proporcje prawego ramienia | 1.0 / 128 |
| Leg Right | Proporcje prawej nogi | 1.0 / 128 |
| Arm Left | Proporcje lewego ramienia | 1.0 / 128 |
| Leg Left | Proporcje lewej nogi | 1.0 / 128 |

---

## Kontekst użycia

- Kopiowanie face data między postaciami = dokładne skopiowanie 0x12F bajtów
- Edycja Model IDs zmienia fryzurę/zarost/brwi bez konieczności znajomości parametrów kształtu
- Face Data w ProfileSummary służy do wyświetlania postaci w menu — powinno być zsynchronizowane
- Wariant 0x120 vs 0x12F — przy kopiowaniu do ProfileSummary obcinaj ostatnie 15 bajtów

---

## Implikacje dla edycji

- **Bezpieczne do kopiowania** blob-to-blob między postaciami
- **Model IDs**: zmiana Hair_Model_Id = zmiana fryzury (wartości z bazy gry)
- **Shape parameters**: wartość 128 = neutralna; zmiana ±1 = minimalny ruch suwaka
- **Kolory**: proste RGBA (0–255 per kanał)
- **Body Scale**: w pamięci float; w save może być u8 (128=1.0) — do weryfikacji
- **Trailing 15 bytes** (slot-only): prawdopodobnie dodatkowe parametry niedostępne w kreatorze lub wewnętrzne flagi

---

## Źródła

- er-save-manager: `parser/world.py` — klasa `FaceData` (linie 27-54)
- er-save-manager: `parser/user_data_x.py` linia 119: `face_data: FaceData`
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Face Data AOB (PlayerGameData+0x754)
- Cheat Engine: `ER_TGA_v1.9.0` — Face Model IDs, Face Details (PlayerGameData+0x754+)
