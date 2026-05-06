# 45 — Dokumentacja Ryzyka Bana

> **Typ**: Dokument referencyjny
> **Status**: ✅ Aktualny (ostatnia weryfikacja 2026-05)
> **Zakres**: Community-reportowane triggery banów, poziomy kar i zasady bezpiecznej edycji dla trybu online w Elden Ring. Podstawa dla systemu tier ryzyka opisanego w spec/32.

**Ważne zastrzeżenie**: FromSoftware i Bandai Namco nie opublikowały dokładnych reguł detekcji. Sekcje oznaczone **[Oficjalne]** pochodzą z oficjalnych źródeł pierwszorzędnych. Sekcje **[RE/zweryfikowane]** zostały potwierdzone z własnych plików danych gry (dump regulation.bin w `tmp/regulation-bin-dump/`). Sekcje **[Community-technical]** to technicznie wiarygodne raporty RE ze społeczności. Sekcje **[Community]** to niezweryfikowane raporty graczy.

---

## 1. Architektura systemu detekcji

### 1.1 Easy Anti-Cheat (EAC)

**[Community-technical]** — Działa lokalnie w trybie **user-mode** (nie kernel-level; Elden Ring używa user-mode EAC, w przeciwieństwie do Nightreign 2025, który używa kernel-level EAC). Przy starcie gry EAC:

1. Wykrywa procesy hookujące lub manipulujące pamięcią gry (np. Cheat Engine uruchomiony obok gry).
2. Weryfikuje integralność binarek gry na dysku przed uruchomieniem.
3. Skanuje obszary pamięci zarezerwowane przez proces gry w czasie runtime.

EAC **nie** czyta pliku `.sl2` — weryfikuje wyłącznie żywy proces gry i binarka na dysku.

**[Oficjalne]** — W czerwcu 2024 FromSoftware oficjalnie przyznało, że "Inappropriate activity detected" może pojawić się jako **fałszywy alarm** bez żadnego cheata, spowodowany uszkodzonymi plikami gry. Oficjalnym zaleceniem było użycie "Verify integrity of game files" w Steam.
> Źródło: [ELDENRING na X, czerwiec 2024](https://x.com/ELDENRING/status/1806689176449855497)

**Metody omijania EAC** — udokumentowane na SoulsSpeedruns Wiki:
- **Metoda `steam_appid.txt`** (nie blokuje online): utwórz `steam_appid.txt` z wartością `1245620` w katalogu `Game/`, uruchom `eldenring.exe` bezpośrednio. EAC nie ładuje się; online nadal dostępne przez Steam.
- **Metoda rename `start_protected_game.exe`** (blokuje online): zmień nazwę `start_protected_game.exe` + podmień kopią `eldenring.exe`. Online wyłączone do czasu przywrócenia.
> Źródło: [SoulsSpeedruns — EAC Bypass](https://soulsspeedruns.com/eldenring/eac-bypass)

Ominięcie EAC wyłącza tylko lokalny skan — **nie** omija walidacji save file po stronie serwera.

### 1.2 Walidacja save file po stronie serwera

**[Community-technical]** — Działa na serwerach FromSoftware podczas synchronizacji online. Przy połączeniu online serwer ładuje wgrany stan postaci i sprawdza go pod kątem tego, co można osiągnąć w legalnej rozgrywce. Detekcja jest triggerowana przez:

- ID przedmiotów nieistniejących w detalicznych tabelach itemów.
- Kombinacje statystyk spoza osiągalnego zakresu (patrz §3.2 — Soul Memory check).
- Ilości w ekwipunku przekraczające waniliowe maksima.
- Stany flag quest/world niespójne z flagami wymaganymi jako warunek wstępny.
- Przedmioty z dropów bossów bez odpowiednich flag zabicia bossa.

Bany za edycję save file pochodzą stąd, nie z EAC. Gracz może grać offline ze zmodyfikowanym save przy aktywnym EAC — kara uruchamia się dopiero przy następnej synchronizacji online. Potwierdzony przypadek: ~5 miesięcy opóźnienia między naruszeniem a karą.

**[Community-technical]** — Serwer **nie** waliduje hasha klienta `regulation.bin`. EAC weryfikuje binarny plik na dysku przed uruchomieniem; serwer sprawdza wynikowy stan postaci przy połączeniu. Edytowanie wartości `regulation.bin` (np. `NetworkParam`) nie jest wykrywane przez server-side hash comparison, bo FromSoftware go nie implementuje — EAC wyłapuje zmodyfikowany plik po stronie klienta przed połączeniem (gdy EAC jest aktywne). Z wyłączonym EAC offline, zmodyfikowany `regulation.bin` ładuje się normalnie; czy zmienione wartości NetworkParam powodują wykrywalne anomalie stanu postaci przy synchronizacji online — niepotwierdzono.
> Źródło: konsensus społeczności RE FearlessRevolution; [waygate-server projekt RE](https://github.com/vswarte/waygate-server)

**[Oficjalne]** — Treść komunikatu bana w grze:
> *"Unauthorized tampering with the game detected. A quarantine penalty of 180 days will be imposed."*

### 1.3 "Inappropriate activity detected" vs "Your account has been penalized" — dwa odrębne systemy

To definitywnie **dwa osobne komunikaty** z dwóch osobnych systemów:

| Komunikat | System | Znaczenie |
| :--- | :--- | :--- |
| "Inappropriate activity detected" (żółty baner przy starcie) | Lokalny — EAC lub sprawdzenie binarek | Save lub binarka flagowane jako podejrzane, LUB fałszywy alarm (uszkodzone pliki). Granie online nadal możliwe. |
| "Your account has been penalized" (czerwony ekran przy starcie) | Server-side — odpowiedź serwera FromSoftware | Aktywna kwarantanna 180-dniowa. Matchmaking ograniczony do puli ukaranych graczy. |

---

## 2. Poziomy kar (wnioskowane przez community)

**[Community]** FromSoftware nie opublikowało formalnej drabinki kar.

| Krok | Komunikat w grze | Opis community |
| :--- | :--- | :--- |
| **1 — Ostrzeżenie** | "Inappropriate activity detected" (żółty baner) | Save flagowany. Granie online nadal możliwe. Może być fałszywy alarm. |
| **2 — Softban (180 dni)** | "Your account has been penalized" (czerwony ekran) | Przeniesienie do ograniczonej puli matchmakingu. |
| **3 — Kolejny softban** | Ten sam czerwony ekran | Kolejne cykle 180-dniowe. Patrz §2.1. |
| **4 — Permanent ban** | Trwałe wykluczenie | Community-reportowany, zwykle po wielokrotnych naruszeniach lub złośliwych działaniach. |

### 2.1 Dlaczego załadowanie tego samego save powoduje kolejne bany — wyjaśniony mechanizm

**[RE/zweryfikowane + Community-technical]** — 180-dniowy stan kwarantanny jest **po stronie serwera**, nie w pliku `.sl2`. Wyczerpujący RE wszystkich znanych edytorów save nie znalazł żadnego pola "flaga bana" w formacie pliku. Pola `unk0x*` w UserData10 pozostają nieudokumentowane we wszystkich publicznych źródłach RE.

Powodem kolejnych banów po tym samym save: **flagowana zawartość** (nielegalne ID, niemożliwe sumy statystyk itp.) nadal jest w save. Przy następnej synchronizacji serwer wykrywa to samo naruszenie i wydaje nową kwarantannę.

Bezpieczna ścieżka odzyskania: przywróć czysty backup sprzed flagowanych edycji.

### 2.2 "Bany we wtorek" — Niepotwierdzone

**[Niezweryfikowana plotka]** — Bany są przetwarzane falami, nie w czasie rzeczywistym (potwierdzony przypadek 5-miesięcznego opóźnienia). Schemat konkretnego dnia tygodnia nie ma potwierdzonego źródła.

---

## 3. Znane triggery banów

### 3.1 Nielegalne przedmioty — mechanizm `disableMultiDropShare`

**[Oficjalne + RE/zweryfikowane]** — Głównym mechanizmem flagowania przedmiotów jako nielegalnych do udostępniania w trybie multiplayer jest pole bitowe `disableMultiDropShare`, obecne we wszystkich tabelach param ekwipunku w `regulation.bin`.

**Zweryfikowane z `tmp/regulation-bin-dump/` (nasz własny dump regulation.bin):**

| Tabela param | Pole | Japońska nazwa | Itemów flagowanych w aktualnym regulation.bin |
| :--- | :--- | :--- | :--- |
| `EquipParamWeapon` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **1** (ID 24590000 — isDrop=1, isDiscard=1) |
| `EquipParamGoods` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **306** (głównie key items/runy z isDrop=0; 7 z isDrop=1) |
| `EquipParamProtector` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |
| `EquipParamAccessory` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |
| `EquipParamGem` | `u8 disableMultiDropShare:1` | マルチドロップ共有禁止か | **0** |

Tłumaczenie: `マルチドロップ共有禁止か` = "Czy zabroniony jest udział w multi-drop sharing?"

`EquipParamGoods` zawiera też `u8 isUseMultiPenaltyOnly:1` (japoński: `クライアント切断ペナルティが発生しているときのみ使用可能` = "Przedmiot dostępny tylko gdy aktywna kara za rozłączenie klienta"). **Żaden item** nie ma tej flagi ustawionej w aktualnym regulation.bin.

**[Oficjalne]** — **Deathbed Smalls** (kwiecień 2022): cut content bielizna dystrybuowana przez drop. Patch 1.04:
> *"Fixed a bug that allowed unauthorized items to be passed to other players."*

Deathbed Smalls to cut content — nie pojawia się w aktualnym `EquipParamProtector` (brak wiersza). ID przedmiotu nie przechodzi server-side sprawdzenia legalności itemów niezależnie od flag.

> Źródło: [Elden Ring Patch Notes 1.04 — Bandai Namco Europe (Oficjalne)](https://en.bandainamcoent.eu/elden-ring/news/elden-ring-patch-notes-104)

### 3.2 Niemożliwe wartości statystyk — Soul Memory check

**[Community-technical]** — Serwer wykrywa kombinacje statystyk niemożliwe do osiągnięcia normalną grą. Na podstawie analizy poprzednich gier FromSoftware (Dark Souls 3 używa tego samego mechanizmu), serwer prawdopodobnie implementuje **Soul Memory check**: suma `current_runes + total_runes_spent` musi równać się `total_runes_earned`. Bezpośrednia edycja `current_runes` bez aktualizacji `total_runes_earned` tworzy wykrywalną rozbieżność. Dlatego dodawanie rune consumables i wydawanie ich w grze jest bezpieczniejsze — aktualizuje wszystkie trzy liczniki normalnie.

> Źródło: [FearLess Cheat Engine forum, "InfiniteWant" (analiza anty-cheat DS3/ER)](https://fearlessrevolution.com/viewtopic.php?t=19320)

| Typ edycji | Ryzyko | Uwagi |
| :--- | :--- | :--- |
| **Bezpośredni zapis pola `souls`** | Wysokie | Tworzy rozbieżność Soul Memory. |
| **Dowolny atrybut powyżej 99** | Wysokie | Sztywny cap w waniliowej grze. |
| **Poziom postaci powyżej 713** | N/D | Poziom 713 = wszystkie 8 atrybutów po 99 = faktyczne maksimum. |

### 3.3 Przedmioty bossów bez flag zabicia

**[Community-technical]** — Dodawanie drop-ów bossów (pamiątki, bronie bossów) do inwentarza bez odpowiednich flag zabicia bossa jest potwierdzonym triggerem bana. Serwer zna stan event flags gracza; loot bossa bez flagi zabicia to wykrywalna niespójność.

> Źródło: [FearLess CE forum, kontrolowany test "duducasarotto"](https://fearlessrevolution.com/viewtopic.php?t=19320)

### 3.4 Naruszenia poziomu ulepszenia broni

**[Community — plausible, niepotwierdzony jako osobna reguła]** — Broń ulepszona powyżej waniliowego maksimum reprezentuje wartości niemożliwe do osiągnięcia legalnie.

Caps ulepszenia:
- Standardowe bronie: +0 do +25
- Specjalne/Somber: +0 do +10

### 3.5 Rollback save file

**[Community-verified]** — Przywrócenie save'a do wcześniejszego stanu jest wykrywalne. Potwierdzony przypadek: Bandai Namco Support przyznał ban za przywrócenie save'a sprzed kilku miesięcy. Opóźnienie ~5 miesięcy.

> Źródło: [Steam: Prohibited Activity PSA](https://steamcommunity.com/app/1245620/discussions/0/3820780968128167841/)

### 3.6 Niezgodność SteamID

**[Community-verified]** — Każdy `.sl2` jest powiązany ze Steam ID. Załadowanie cudzego save pod własnym kontem = wykrywalna niezgodność.

### 3.7 Niestandardowe mody gestów/animacji

**[Community-verified, kwiecień 2024]** — Kosmetyczny mod gestów zastępujący animacje treścią spoza zestawu detalicznego spowodował ostrzeżenie o karze. Wskazuje, że serwer waliduje ID gestów/animacji oprócz ID przedmiotów.

### 3.8 Edycja NetworkParam (system kar za rozłączenie)

**[RE/zweryfikowane]** — `NetworkParam` w `regulation.bin` zawiera **system punktacji kar za rozłączenie sesji**, osobny od systemu banowania save file. Wartości zweryfikowane z naszego dumpa regulation.bin:

| Pole | Wartość | Znaczenie |
| :--- | :--- | :--- |
| `penaltyPointLanDisconnect` | 10 | Punkty za rozłączenie LAN |
| `penaltyPointSignout` | 0 | Punkty za wylogowanie |
| `penaltyPointReboot` | 10 | Punkty za reboot/wyłączenie |
| `penaltyPointBeginPenalize` | 100 | Próg aktywacji kary |
| `penaltyForgiveItemLimitTime` | 36000.0 sec | Czas odpuszczenia za "Seedbed Curse" |

System karze za DC-quitting w sesjach multiplayer. Jest **osobny** od systemu banowania zawartości save file.

**Ryzyko edycji NetworkParam**: EAC weryfikuje `regulation.bin` na dysku przed uruchomieniem (z aktywnym EAC, zmodyfikowany plik zostałby wykryty). Z wyłączonym EAC offline, zmodyfikowany `NetworkParam` ładuje się; czy zmienione wartości powodują anomalie server-side przy sync online — niepotwierdzono. Brak publicznego raportu bana wyłącznie z edycji NetworkParam.

---

## 4. Wiersze debug w regulation.bin

**[RE/zweryfikowane]** — Wszystkie tabele param ekwipunku zawierają pole `u8 disableParam_NT`. Wiersze z tą flagą ustawioną na `1` to debug/wewnętrzne placeholdery (np. weapon IDs 1000, 1100, 1200–1260, 1400). Dodanie itemów z tymi ID do save byłoby wykryte jako treść spoza zestawu detalicznego.

---

## 5. RE protokołu sieciowego

**[RE]** — Protokół matchmakingowy został częściowo odwrócony przez społeczność:
- Protokół używa **NaCl key exchange** (libsodium KX) z zakodowanymi na stałe keypairami w binarce gry.
- RE pokrywa: plamy krwi, duchy, wiadomości, znaki przywołania, inwazje, quickmatches, hasła grupowe.
- **Brak udokumentowanego endpointu penalty/quarantine.** Mechanizm sprawdzania kary przez serwer nie jest publicznie zreversowany.

> Źródło: [waygate-server — open-source serwer matchmakingowy ER (Rust)](https://github.com/vswarte/waygate-server)

---

## 6. Tabela oceny ryzyka

| Kategoria | Przykład — Wysokie ryzyko | Przykład — Niższe ryzyko |
| :--- | :--- | :--- |
| **Przedmioty** | Cut content / debug items (brak wiersza w retail param) | Legalne przedmioty z retail ID |
| **Statystyki** | Bezpośredni zapis pola `souls` (rozbieżność Soul Memory); atrybut >99 | Dodawanie rune consumables i wydawanie w grze |
| **Ekwipunek** | Broń powyżej cap +25/+10 | Każda broń w standardowych granicach |
| **Event flags** | Loot bossa bez flag zabicia | — |
| **Stan save'a** | Przywracanie backupu sprzed tygodni/miesięcy | Przywracanie backupu z tej samej sesji |
| **Poziom** | N/D (713 = twarde maksimum) | Dodawanie rune consumables |
| **Tożsamość** | Cudzy `.sl2` pod własnym kontem | — |

---

## 7. Zasady bezpiecznej edycji

1. **Tylko offline podczas edycji**: Rozłącz się lub użyj Anti-Cheat Toggler przed otwarciem zmodyfikowanego save.
2. **Trzymaj czysty backup**: Przy "Inappropriate activity detected" najpierw "Verify integrity of game files" (może fałszywy alarm). Jeśli nie pomoże — przywróć backup przed następną synchronizacją.
3. **Preferuj pośrednie edycje**: Dodaj rune consumables i wydaj w grze zamiast pisać bezpośrednio do pola `souls`.
4. **Używaj tylko legalnych ID**: Flagi `cut_content` / `ban_risk` w `backend/db/db.go` identyfikują znane ryzykowne itemy.
5. **Nie dodawaj lootu bossów bez flag zabicia**: Serwer sprawdza spójność event flags.
6. **Nie wczytuj flagowanej zawartości po banie**: Załaduj czysty backup — kwarantanna jest server-side, ale treść nadal jest w pliku.
7. **Nie ładuj cudzego save'a**: Niezgodność SteamID jest potwierdzonym triggerem.

---

## 8. Platforma PS4

**[Community-verified, bez oficjalnego potwierdzenia]**

- PS4 nie używa EAC — technologia wyłącznie PC.
- Sony nie implementuje własnego anti-cheat na poziomie gry.
- Serwery FromSoftware obsługują wszystkie platformy — walidacja server-side dotyczy save'ów z każdej platformy.
- **Brak RE** potwierdzającego flagę bana w `memory.dat`. Skoro stan kary jest server-side na PC, ta sama architektura obowiązuje prawie na pewno na PS4.
- Brak potwierdzonych PS4-specific ban case'ów z dokumentacją techniczną.

---

## 9. Związek z tym edytorem

| Ten dokument | Implementacja spec/32 |
| :--- | :--- |
| §3.1 — Nielegalne itemy / `disableMultiDropShare` | Flagi `cut_content`, `ban_risk` + `RiskBadge` |
| §3.2 — Soul Memory / atrybut >99 | Risk key `stat_above_99` + `RiskInfoIcon` na atrybutach |
| §3.2 — Bezpośrednia edycja statystyk | Risk key `derived_stat_manual` |
| §3.4 — Cap ulepszenia | `quantity_above_max` + flagi itemów |
| §3.8 — Edycja NetworkParam | Tier 1 ban-risk labels w zakładce Networking |
| §4 — Wiersze debug | Flaga `cut_content` na itemach z zakresem debug ID |

---

## Źródła

| Źródło | URL | Typ |
| :--- | :--- | :--- |
| Elden Ring Patch Notes 1.04 | https://en.bandainamcoent.eu/elden-ring/news/elden-ring-patch-notes-104 | **Oficjalne** |
| ELDENRING na X — fałszywy alarm | https://x.com/ELDENRING/status/1806689176449855497 | **Oficjalne** |
| Elden Ring EULA (Steam) | https://store.steampowered.com/eula/1245620_eula_0 | **Oficjalne** |
| regulation.bin dump (lokalny) | `tmp/regulation-bin-dump/defs/` + `csv/` | **RE/zweryfikowane** |
| waygate-server — RE protokołu ER | https://github.com/vswarte/waygate-server | RE |
| SoulsSpeedruns — EAC Bypass | https://soulsspeedruns.com/eldenring/eac-bypass | Community-technical |
| FearLess CE — kontrolowane testy banów | https://fearlessrevolution.com/viewtopic.php?t=19320 | Community-technical |
| Steam: Prohibited Activity PSA | https://steamcommunity.com/app/1245620/discussions/0/3820780968128167841/ | Community |
| Steam: Ban again after 180 days | https://steamcommunity.com/app/1245620/discussions/0/6679473478679984703/ | Community |
| Steam: 180 days ban countdown | https://steamcommunity.com/app/1245620/discussions/0/3758850762509707736/ | Community |
| Steam: 180 Day ban after coming back | https://steamcommunity.com/app/1245620/discussions/0/4343239957176299084/ | Community |
| Steam: Cheating levels thread | https://steamcommunity.com/app/1245620/discussions/0/4526764179303674425/?l=english | Community |
| Steam: Deathbed Smalls ban wave | https://steamcommunity.com/app/1245620/discussions/0/3278065083958673810/ | Community |
| Automaton Media: Incydent Deathbed Smalls | https://automaton-media.com/en/news/20220416-11617/ | Prasa (pierwotne) |
| PCGamesN: Inappropriate activity fix | https://www.pcgamesn.com/elden-ring/inappropriate-activity-detected-fix | Prasa |
| SVG: Illegal Item Warnings | https://www.svg.com/961328/elden-rings-new-illegal-item-warnings-explained/ | Prasa |
| Comicbook.com: Deathbed Smalls | https://comicbook.com/gaming/news/elden-ring-players-banned-underwear-deathbed-smalls-cut-from-the-game/ | Prasa |
| Nexus Mods: Anti-Cheat Toggler | https://www.nexusmods.com/eldenring/mods/90 | Narzędzie |
| GitHub: EldenRingEacToggler | https://github.com/techiew/EldenRingEacToggler | Narzędzie (open source) |
| Nexus Mods: 713 max level save | https://www.nexusmods.com/eldenring/mods/4056 | Community |
| Fextralife: Stats | https://eldenring.wiki.fextralife.com/Stats | Wiki |
