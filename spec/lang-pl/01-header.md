# 01 — Header i Layout Pliku

> **Zakres**: Detekcja platformy, struktura BND4, rozmieszczenie slotów i checksums w pliku.

---

## Detekcja platformy

Pierwszych 4 bajtów pliku determinuje platformę:

| Magic Bytes | Platforma | Uwagi |
|---|---|---|
| `42 4E 44 34` ("BND4") | PC (Steam) | Nieszyfrowany |
| `53 4C 32 00` ("SL2\0") | PC (alternatywny) | Spotykany rzadko |
| `CB 01 9C 2C` | PS4 / PS5 | Brak szyfrowania i checksums |

Jeśli pierwsze 4 bajty nie pasują do żadnego — plik może być **zaszyfrowany AES-128-CBC**. IV to pierwsze 16 bajtów pliku. Po deszyfrowaniu powinien zaczynać się od "BND4".

### Źródła
- er-save-manager: `parser/save.py` linie 131-142
- Steam Guide: https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037

---

## Layout PC (ER0000.sl2)

```
Offset          Rozmiar         Zawartość
─────────────────────────────────────────────────────────
0x000           0x300           BND4 Header (kontener FromSoftware)
0x300           0x010           MD5 Checksum — Slot 0
0x310           0x280000        SaveSlot[0] — dane postaci
0x280310        0x010           MD5 Checksum — Slot 1
0x280320        0x280000        SaveSlot[1]
...             ...             (powtórzenie ×10 slotów)
0x19003A0       0x010           MD5 Checksum — UserData10
0x19003B0       0x60000         UserData10 (profil konta)
0x19603B0       ~0x240010       UserData11 (regulation.bin)
─────────────────────────────────────────────────────────
TOTAL:          ~28.9 MB
```

### Formuła offsetu slotu N (PC):
- **Checksum**: `0x300 + N × 0x280010`
- **Dane**: `0x310 + N × 0x280010`

---

## Layout PS4 (memory.dat)

```
Offset          Rozmiar         Zawartość
─────────────────────────────────────────────────────────
0x000           0x070           PS4 Header (stały)
0x070           0x280000        SaveSlot[0] (bez MD5)
0x280070        0x280000        SaveSlot[1]
...             ...             (×10 slotów)
0x1900070       0x60000         UserData10 (bez MD5)
0x1960070       ~0x240010       UserData11
─────────────────────────────────────────────────────────
```

### Formuła offsetu slotu N (PS4):
- **Dane**: `0x70 + N × 0x280000`

---

## BND4 Header (0x300 bytes) — tylko PC

Standardowy kontener FromSoftware. Zawiera metadane o 11 "plikach" wewnątrz (10 slotów + UserData).

### Struktura nagłówka (pierwsze 0x40 bytes):

| Offset | Typ | Wartość | Opis |
|---|---|---|---|
| 0x00 | char[4] | "BND4" | Magic identifier |
| 0x04 | u32 | 0x00000000 | Stała |
| 0x08 | u32 | 0x00010000 | Numer rewizji (spekulacja) |
| 0x0C | u32 | 11 | Liczba slotów (10 postaci + UserData) |
| 0x10 | u32 | 0x00000040 | Stała |
| 0x14 | u32 | 0x00000000 | Stała |
| 0x18 | u32 | 0x00000020 | Rozmiar slot header entry |
| 0x1C | u32 | 0x000002C0 | Rozmiar całego file headera? |
| 0x20 | u32 | 0x00000000 | Stała |
| 0x24 | u32 | 0x00002001 | Flagi? |
| 0x28 | u8[12] | 0x00... | Padding |

Po nagłówku — 11 × Slot Header Entry (po 0x20 bytes każdy) opisujących offset i rozmiar każdego "pliku".

### Źródła
- SoulsFormats: https://github.com/JKAnderson/SoulsFormats
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=format:sl2
- ER-Save-Editor (Rust): `src/save/pc/save_header.rs` — czyta 0x300 jako opaque blob

---

## Checksumy MD5 (tylko PC)

Każdy slot posiada 16-bajtowy prefix z hashem MD5 obliczonym z danych slotu (0x280000 bytes).

- **Przy odczycie**: gra sprawdza MD5 — niezgodność = save odrzucony
- **Przy zapisie**: editor MUSI przeliczyć MD5 po modyfikacji danych slotu
- **PS4/PS5**: brak checksums — dane slotu zaczynają się bezpośrednio

### Algorytm przeliczania:
```
checksum = MD5(slot_data[0:0x280000])
write checksum at (0x300 + slot_index * 0x280010)
```

### Źródła
- Steam Guide: https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037
- er-save-manager: `parser/save.py` metoda `recalculate_checksums()`

---

## Szyfrowanie AES-128-CBC (tylko PC, opcjonalne)

Starsze wersje save mogą być zaszyfrowane. Nowsze wersje gry zapisują nieszyfrowane "BND4".

- **IV**: pierwsze 16 bajtów pliku
- **Klucz**: stały, wbudowany w exe gry
- **Po deszyfrowaniu**: plik zaczyna się od "BND4"

### Źródła
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=format:sl2
- ER-Save-Editor (Rust): obsługuje decrypt w `src/save/pc/`

---

## PS4 Header (0x70 bytes)

Stały nagłówek PS4. Magic: `CB 01 9C 2C`. Reszta headerа jest stała i nie wymaga edycji.

### Źródła
- er-save-manager: `parser/save.py` linia 146 (`header_size = 0x6C` po magic)

---

## Puste sloty

Slot jest pusty gdy:
- **PC**: checksum = 16 × `0x00` (wszystkie zera)
- **Ogólnie**: `version` (pierwszy u32 danych slotu) = 0

### Źródła
- er-save-manager: `parser/save.py` linie 174-178
- er-save-manager: `parser/user_data_x.py` — metoda `is_empty()`
