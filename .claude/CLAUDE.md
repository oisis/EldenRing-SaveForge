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
├── scripts/             # extractor.go — import danych z Rust source
├-- tmp/repos            # repozytoria z kodem referencyjnym
├--- tmp/save            # pliki save do testow
└── Makefile

```

---

## Kluczowe konwencje

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
| Import danych | `go run scripts/extractor.go tmp/org-src/src/db/ backend/db/data/` |

---

## Co weryfikować po każdej zmianie

1. **Zmiana w `backend/core/`** → `go test -v ./tests/roundtrip_test.go` (PS4 round-trip, PC round-trip, PS4→PC, PC→PS4)
2. **Zmiana w `backend/db/`** → `go test -v ./backend/...`
3. **Zmiana w `app.go`** → `go build ./...` (sprawdź bindingsy) + `wails generate module`
4. **Zmiana w `frontend/`** → `cd frontend && npx tsc --noEmit && npm run lint`
5. **Każda zmiana** → `make build` jako ostateczna weryfikacja

---

## ROADMAP.md — zasady prowadzenia

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
- Post-mortem / root-cause analysis → `CHANGELOG.md`
- Szczegółowy design (fazy, structs, UI mockupy, testy) → `spec/NN-*.md`
- Research / investigation logs → `spec/NN-*.md`
- Bugfix lista → `CHANGELOG.md`
- Opisy ukończonych feature'ów dłuższe niż 1 linia → wyrzucić (CHANGELOG + spec pokrywają)

### Objętość docelowa
- Max ~150 linii. Jeśli ROADMAP rośnie ponad 200 — czas wyciągnąć design docs do spec/.

---

## Pułapki i ograniczenia

- `wails dev` wymaga środowiska GUI (uruchamia Cocoa window) — nie uruchamiaj w headless.
- `frontend/wailsjs/` jest nadpisywane przez Wails przy każdym `wails generate module` — zmiany tracone.
- Pliki `.sl2` w `tests/data/` to prawdziwe save'y (PC i PS4) — nigdy nie modyfikuj bez backupu.
- PS4 save nie ma nagłówka SteamID — nie dodawaj go przy konwersji PS4→PC przed `steamid.go`.
- Przy konwersji platform offsety Active Slots i Summaries są różne (patrz tabela wyżej) — błędny offset = uszkodzony save.
- `make extract-data` wymaga źródeł Rust w `tmp/org-src/` — nie są w repo.
- `wails.json` — nie zmieniaj `outputfilename` bez aktualizacji CI/build scripts.
