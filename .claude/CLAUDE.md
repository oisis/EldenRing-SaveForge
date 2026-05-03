# CLAUDE.md — EldenRing-SaveEditor
# Scope: ten projekt. Nadpisuje ~/.claude/CLAUDE.md tam gdzie się różni.

---

## Projekt

**Desktop app** — Wails v2 (Go backend + React/TypeScript frontend).
Edytor plików zapisu Elden Ring: odczyt/zapis binarnego formatu `.sl2`, krypto, backup, zarządzanie postacią i ekwipunkiem.

**Obsługiwane platformy save file:** PC (Steam `.sl2`) oraz **PS4** — z dwukierunkową konwersją między nimi. PS4 jest priorytetową platformą.

---

## Architektura

```
.
├── main.go              # Wails bootstrap (wails.Run)
├── app.go               # Wails App struct — eksponuje metody do JS przez Wails bindings
├── backend/
│   ├── core/            # I/O save file: reader, writer, crypto, backup, structures, steamid
│   ├── db/              # Baza danych gry: db.go + data/ (itemy, statsy, ikony)
│   └── vm/              # ViewModel: character_vm.go, validation.go
├── frontend/
│   └── src/
│       ├── components/  # Zakładki: GeneralTab, StatsTab, InventoryTab, DatabaseTab, ...
│       └── wailsjs/     # Auto-generowane bindingsy Go→JS (NIE EDYTUJ ręcznie)
├── tests/               # roundtrip_test.go, steamid_test.go + data/
├── tmp/
│   ├── scripts/         # Skrypty wspomagające (importery, parsery, narzędzia jednorazowe)
│   ├── regulation-bin-dump/ # Dump regulation.bin: csv/, defs/, params/ (194 tabel parametrów gry)
│   ├── repos/           # Repozytoria z kodem referencyjnym
│   └── save/            # Pliki save do testów
└── Makefile

```

---

## Kluczowe konwencje

### Skrypty wspomagające

- **Lokalizacja**: wyłącznie `tmp/scripts/`. Nigdy nie twórz skryptów pomocniczych (importery, parsery, narzędzia jednorazowe) poza tym katalogiem.
- Podkatalogi tematyczne dozwolone: `tmp/scripts/diag/`, `tmp/scripts/debug/`, itd.
- `tmp/` jest w `.gitignore` — skrypty wspomagające nie wchodzą do repozytorium.
- Przykład uruchomienia: `go run tmp/scripts/import_descriptions.go <args>`

### Regulation bin dump

- **Lokalizacja**: `tmp/regulation-bin-dump/`
- **Źródło**: zdekodowany `regulation.bin` z Elden Ring (parametry gry)
- **Zawartość**:
  - `csv/` — 194 tabel w formacie CSV (separator `;`), np. `EquipParamWeapon.csv`, `EquipParamProtector.csv`, `SpEffectParam.csv`
  - `defs/` — 194 definicji XML (schematy pól dla każdej tabeli)
  - `params/` — 194 plików `.param` (surowe dane binarne)
  - `regulation.bin` — oryginalny plik źródłowy
- **Zastosowanie**: źródło danych do importu statystyk broni, zbroi, czarów, efektów itp. do `backend/db/data/`
- Row ID w CSV odpowiada identyfikatorowi przedmiotu (np. weapon ID 1000000 → Row ID 1000000 w EquipParamWeapon.csv)

### Platformy i format save file

| Aspekt | PC | PS4 |
|---|---|---|
| Typ (`Platform`) | `PlatformPC = "PC"` | `PlatformPS = "PS4"` |
| Detekcja | magic bytes w `LoadSave()` | magic bytes w `LoadSave()` |
| Szyfrowanie | AES-128 (`crypto.go`) | brak |
| Active Slots offset | `0x1C` | `0x300` |
| Summaries offset | `0x26` | `0x30A` |
| SteamID | `steamid.go` (offset 4) | n/d |
| Konwersja | `WriteSave(platform)` z `app.go` | jw. |

- Konwersja jest dwukierunkowa: PS4→PC i PC→PS4, obsługiwana przez `WriteSave(targetPlatform string)`.
- Każda zmiana logiki I/O musi przejść **oba** round-trip testy (PS4 i PC) oraz test konwersji.
- UI: `SettingsTab.tsx` pozwala wybrać platformę docelową przed eksportem (`['PC', 'PS4']`).

### Go
- Funkcje eksponowane do JS muszą być metodami `App` (app.go) z `//go:generate wails` lub zarejestrowane w `wails.Run`.
- `frontend/wailsjs/` jest **auto-generowane** przez `wails generate module` — nigdy nie edytuj tych plików.
- Format binarny save file: big-endian, offset-based. Zmiany w `structures.go` muszą zachować kompatybilność z istniejącymi zapisami.
- Crypto w `backend/core/crypto.go` — AES-128, **tylko PC**. PS4 save nie jest szyfrowany.

### Frontend (React + TypeScript + Vite)
- Komponenty = zakładki edytora. Jeden komponent per zakładka w `frontend/src/components/`.
- Wails bindings: importuj z `../wailsjs/go/main` (nie pisz własnych fetch/XHR).
- Style: Tailwind CSS (PostCSS config w `frontend/postcss.config.js`).
- Wails dev server: port jest zarządzany przez Wails, nie konfiguruj ręcznie.

---

## Komendy

| Zadanie | Komenda |
|---|---|
| Build aplikacji | `make build` |
| Dev (hot reload) | `make dev` ⚠️ uruchamia GUI |
| Wszystkie testy | `make test` |
| Unit testy Go | `go test -v ./backend/...` |
| Round-trip test | `go test -v ./tests/roundtrip_test.go` |
| Linter Go | `golangci-lint run ./...` |
| Format Go | `gofmt -w <plik>.go` |
| TS typecheck | `cd frontend && npx tsc --noEmit` |
| Frontend lint | `cd frontend && npm run lint` |
| Import opisów | `go run tmp/scripts/import_descriptions.go tmp/opisy-stats/data/ backend/db/data/descriptions.go` |

---

## Co weryfikować po każdej zmianie

1. **Zmiana w `backend/core/`** → `go test -v ./tests/roundtrip_test.go` (PS4 round-trip, PC round-trip, PS4→PC, PC→PS4)
2. **Zmiana w `backend/db/`** → `go test -v ./backend/...`
3. **Zmiana w `app.go`** → `go build ./...` (sprawdź bindingsy) + `wails generate module`
4. **Zmiana w `frontend/`** → `cd frontend && npx tsc --noEmit && npm run lint`
5. **Każda zmiana** → `make build` jako ostateczna weryfikacja

---

## docs/ROADMAP.md — zasady prowadzenia

ROADMAP to **strategiczny przegląd** projektu — odpowiada na "co robimy i w jakiej kolejności", NIE na "jak to zrobimy".

### Struktura
- `## Done` — 1-liniowe wpisy pogrupowane po wersjach/fazach (nazwa + krótki opis)
- `## In Progress` — aktualnie realizowane (max 3 pozycje)
- `## Planned` — przyszłe featury z priorytetem (🔴/🟡/🟢/🔵), posortowane od najważniejszych
- `## Backlog` — luźne pomysły bez terminów

### Co WCHODZI do ROADMAP
- Nazwa feature'u + 1-2 zdania opisu (co i dlaczego)
- Priorytet + ewentualny blocker
- Link do `spec/` jeśli istnieje design doc (np. `→ spec/37`)
- Szacunkowy effort (opcjonalnie)

### Co NIE WCHODZI do ROADMAP
- Implementation details (pliki, offsety, API signatures) → kod jest source-of-truth
- Post-mortem / root-cause analysis → `docs/CHANGELOG.md`
- Szczegółowy design (fazy, structs, UI mockupy, testy) → `spec/NN-*.md`
- Research / investigation logs → `spec/NN-*.md`
- Bugfix lista → `docs/CHANGELOG.md`
- Opisy ukończonych feature'ów dłuższe niż 1 linia → wyrzucić (CHANGELOG + spec pokrywają)

### Objętość docelowa
- Max ~150 linii. Jeśli ROADMAP rośnie ponad 200 — czas wyciągnąć design docs do spec/.

---

## spec/ — zasady dokumentacji

Katalog `spec/` zawiera specyfikacje formatu binarnego i design doci. **Język główny: angielski.** Polskie tłumaczenia w `spec/lang-pl/`.

### Tworzenie nowego dokumentu

1. Wybierz numer: następny wolny `NN` (sprawdź `ls spec/*.md`).
2. Stwórz **oba** pliki jednocześnie:
   - `spec/NN-nazwa.md` — wersja angielska (source of truth)
   - `spec/lang-pl/NN-nazwa.md` — wersja polska (tłumaczenie)
3. Zaktualizuj `spec/README.md` (tabela EN) i `spec/lang-pl/README.md` (tabela PL).

### Kanoniczny nagłówek

**Binary format spec:**
```markdown
# NN — Title

> **Type**: Binary format spec
> **Scope**: One sentence describing what this section covers.
```

**Design doc:**
```markdown
# NN — Title

> **Type**: Design doc
> **Status**: 🔲 Planned | ✅ Implemented | 🐛 Bug/Paused
> **Scope**: One sentence.
```

### Reguły treści

- **Język plików w `spec/`**: angielski. Identyfikatory kodu, nazwy pól, hex — bez tłumaczenia.
- **Język plików w `spec/lang-pl/`**: polski. Identyfikatory kodu, hex, ścieżki plików — bez tłumaczenia.
- Tabele, diagramy ASCII, bloki kodu — identyczne w obu wersjach (tłumaczymy tylko tekst opisowy).
- Sekcja `## Sources` / `## Źródła` na końcu — linki do plików referencyjnych i URL.
- Offsety jako hex (`0x1B0`), rozmiary hex + dec (`0x12F (303 bytes)`).
- Nieznane pola: `unk_0xNN` + notatka co wiemy.
- Status weryfikacji: ✅ hex-verified | ⚠️ cross-reference only | ❓ uncertain.

### Aktualizacja istniejącego dokumentu

Przy edycji `spec/NN-*.md` — zaktualizuj również `spec/lang-pl/NN-*.md` (lub oznacz jako TODO w CHANGELOG jeśli zmiana jest pilna a tłumaczenie nie).

---

## Pułapki i ograniczenia

- `wails dev` wymaga środowiska GUI (uruchamia Cocoa window) — nie uruchamiaj w headless.
- `frontend/wailsjs/` jest nadpisywane przez Wails przy każdym `wails generate module` — zmiany tracone.
- Pliki `.sl2` w `tests/data/` to prawdziwe save'y (PC i PS4) — nigdy nie modyfikuj bez backupu.
- PS4 save nie ma nagłówka SteamID — nie dodawaj go przy konwersji PS4→PC przed `steamid.go`.
- Przy konwersji platform offsety Active Slots i Summaries są różne (patrz tabela wyżej) — błędny offset = uszkodzony save.
- `make extract-data` wymaga źródeł Rust w `tmp/org-src/` — nie są w repo.
- `wails.json` — nie zmieniaj `outputfilename` bez aktualizacji CI/build scripts.
