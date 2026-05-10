# 49 — PS4 NetworkParam Patch: strategia Raw-Block ZSTD

> **Type**: Design doc
> **Status**: ✅ Implemented
> **Scope**: Jak `PatchNetworkParams` modyfikuje regulation.bin na PS4 bez wywoływania crashu na ekranie tytułowym.

---

## Problem

Po zastosowaniu presetu sieciowego (najazdy, przywoływanie, host) i wgraniu zmodyfikowanego save'a na PS4, gra crashowała na ekranie tytułowym ("Press any button").

**Przyczyna**: `PatchNetworkParams` wywoływał `compressDCX` do rekompresji ramki ZSTD regulation.bin po patchowaniu. Enkoder ZSTD klauspost produkuje ramkę z innymi flagami nagłówka niż enkoder FromSoftware:

| Parametr | FromSoftware | klauspost `SpeedDefault` |
|---|---|---|
| Bajt FHD | `0x00` | `0x84` |
| Pole Content size | nieobecne | obecne (8 bajtów) |
| Flaga checksum | wyłączona | włączona |
| Window descriptor | obecny, 64 MB | inna wartość |

Inny plaintext → inny ciphertext. Gra na PS4 weryfikuje integralność ciphertextu (zaszyfrowany blok AES-256-CBC musi być identyczny bajt po bajcie z tym, czego oczekuje) i odrzuca plik, co powoduje crash.

**PC nie jest dotknięty**, bo gra na PC weryfikuje tylko 16-bajtowy prefiks MD5 (obejmujący `ud11[0x10:]`), który jest przeliczany po patchowaniu.

---

## Detekcja

| Platforma | `ud11RegulationOffset()` zwraca | Powód |
|---|---|---|
| PS4 | `ud11UnkSize = 0x10` | brak prefiksu MD5 |
| PC | `ud11MD5Size + ud11UnkSize = 0x20` | 16-bajtowy prefiks MD5 w `ud11[0:0x10]` |

`PatchNetworkParams` rozgałęzia się na podstawie `regStart == ud11UnkSize` (ścieżka PS4) vs `regStart == ud11MD5Size+ud11UnkSize` (ścieżka PC).

---

## Rozwiązanie: zastępowanie bloków Raw ZSTD

Zamiast dekompresji i rekompresji całego strumienia, zastępujemy tylko konkretny blok (lub bloki) ZSTD zawierający patchowane pola **blokami Raw** (Block_Type=0). Wszystkie pozostałe bloki pozostają identyczne bajt po bajcie z oryginałem. Wynikowy strumień jest następnie szyfrowany AES-CBC z oryginalnym IV.

### Dlaczego to działa

- Nagłówek ramki ZSTD (magic, FHD, window descriptor) pozostaje niezmieniony.
- Bloki poza zakresem patcha są identyczne bajt po bajcie z oryginalnym ciphertextem.
- Blok Raw niesie nieskompresowany payload — dekompressor traktuje go jako bajty dosłowne, bez potrzeby drzewa Huffmana.
- Jedyne zmieniające się bajty w strumieniu to zastąpione bloki. Szyfr AES-CBC propaguje zmiany przez kolejne 16-bajtowe bloki szyfrogramu, ale PS4 nie weryfikuje sum kontrolnych poszczególnych bloków — wystarczy, że deszyfrowanie i dekompresja DCX się powiodą.

### Układ bloków

FromSoftware koduje regulation.bin z FLUSH_BLOCK co 64 KB plaintextu:

```
Block 0: BND4 bajty [0x00000, 0x10000)   → 65536 bajtów zdekompresowanych
Block 1: BND4 bajty [0x10000, 0x20000)
...
Block N: ostatni blok (może być < 65536 bajtów)
```

Indeks bloku dla danego offsetu BND4:

```
blockIdx = bnd4Offset / 65536
```

### Zakres pól NetworkParam

Patchowane pola rozciągają się od `offsetReloadSignIntervalTime2` (`0x1C`) przez `offsetVisitorDownloadSpan+3` (`0x24B`) w danych wiersza NetworkParam. Oba punkty krańcowe wyrażone są jako absolutne offsety BND4:

```
firstBND4 = paramOffset + rowDataOffset + 0x1C
lastBND4  = paramOffset + rowDataOffset + 0x24B
```

Jeśli dane wiersza przekraczają granicę 64 KB, `firstBlockIdx != lastBlockIdx` i wszystkie bloki w zakresie `[firstBlockIdx, lastBlockIdx]` są zastępowane.

### Zabezpieczenie Treeless_Literals

Jeśli blok bezpośrednio następujący po ostatnim zastąpionym bloku ma `Literals_Section_Type = Treeless` (wartość `3` w dwóch niskich bitach bajtu nagłówka literałów), reuse'uje drzewo Huffmana z poprzedniego bloku Compressed. Ponieważ ten blok jest zastępowany blokiem Raw (bez drzewa Huffmana), następnik nie mógłby się zdekompresować.

Fix: zastąp też następnik Treeless blokiem Raw niosącym odpowiedni wycinek (zmodyfikowanych) danych BND4.

---

## Implementacja

`patchZSTDStreamRawBlock(regBlob, bnd4 []byte, firstBND4, lastBND4 int) ([]byte, error)`  
w `backend/core/regulation.go`

Kroki:
1. Wyodrębnij strumień ZSTD z `regBlob[76 : 76+compSize]`.
2. Oblicz `firstBlockIdx`, `lastBlockIdx` z offsetów BND4.
3. Przejdź bloki przez `walkZSTDBlocks` (zatrzymaj się na `lastBlockIdx+3` dla sprawdzenia Treeless).
4. Sprawdź, czy bloki docelowe mają typ 2 (Compressed) lub 0 (Raw z poprzedniego patcha).
5. Sprawdź `blocks[lastBlockIdx+1]` pod kątem flagi Treeless_Literals; ustaw `replaceAfterBlock` jeśli prawda.
6. Zbuduj nowy strumień:
   - Prefiks vanilla: `stream[:firstBlock.streamStart]`
   - Bloki Raw dla `[firstBlockIdx, lastBlockIdx]` (i ewentualnie `afterIdx`)
   - Sufiks vanilla: `stream[replaceStreamEnd:]`
7. Zapisz nowy DCX: skopiuj `regBlob[:76]`, zaktualizuj `compressedSize` na offsecie `[32:36]` (big-endian), dołącz nowy strumień.

`walkZSTDBlocks(stream []byte, maxBlocks int) ([]zstdBlock, error)`  
Parsuje nagłówek ramki ZSTD (pola FHD, window descriptor, DID, FCS) żeby znaleźć pierwszy blok, następnie przechodzi nagłówki bloków zbierając `{streamStart, streamEnd, btype, last}`.

`makeRawBlockHeader(dataSize int, last bool) []byte`  
Zwraca 3-bajtowy nagłówek bloku ZSTD: `Block_Size << 3 | Last_Block_Bit`.

---

## Sprawdzenie pojemności

Bloki Raw zwiększają strumień (brak kompresji). Spatchowany strumień musi zmieścić się w oryginalnej alokacji ciphertextu AES-CBC (`len(ud11) - regStart - 16`). Jest to sprawdzane przed szyfrowaniem:

```go
if len(newRegBlob) > originalCiphertextLen {
    return nil, fmt.Errorf("patched regulation blob (%d bytes) exceeds ciphertext capacity (%d bytes)", ...)
}
```

W praktyce rozszerzone bloki Raw dla NetworkParam.param mieszczą się z dużym zapasem, ponieważ regulation.bin jest wypełniony do stałej granicy bloku AES.

---

## Idempotentność

Drugie wywołanie `patchZSTDStreamRawBlock` na już spatchowanym strumieniu działa poprawnie: `walkZSTDBlocks` akceptuje bloki docelowe typu 0 (Raw) oprócz typu 2 (Compressed).

---

## Ścieżka PC (bez zmian)

Save'y PC używają `compressDCX` + przeliczenie MD5. MD5 obejmuje `ud11[0x10:]` i jest przechowywany w `ud11[0:0x10]`. Ponieważ gra weryfikuje tylko MD5 (nie integralność ciphertextu na poziomie bloków), każde poprawne kodowanie ZSTD jest akceptowane.

---

## Źródła

- `backend/core/regulation.go` — `PatchNetworkParams`, `patchZSTDStreamRawBlock`, `walkZSTDBlocks`, `makeRawBlockHeader`
- `tests/regulation_test.go` — `TestPatchNetworkParams_PC_RoundTrip`, `TestPatchNetworkParams_PS4_RoundTrip`
- `tmp/scripts/gen_rawblock_ps4.go` — skrypt referencyjny użyty do wygenerowania i weryfikacji podejścia rawblock
- Format ramki ZSTD: RFC 8878 / zstd spec §3.1 (kodowanie Block_Header)
