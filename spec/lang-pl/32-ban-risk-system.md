# 32 — Ban-Risk Awareness System (UI/UX)

> **Zakres**: Architektura systemu ostrzeżeń o ryzykownych edycjach save'a w UI edytora. Tier 0/1/2 + Online Safety Mode + dictionary `RISK_INFO` + komponenty React (`RiskInfoIcon`, `RiskBadge`, `RiskActionButton`, `RiskSectionBanner`).

> **Status**: ✅ Wdrożone na branchu `feat/ban-risk-system` (Apr 2026, fazy 1-5). Faza 6 (polish + dokumentacja) — ten dokument.

---

## Cel

Easy Anti-Cheat (EAC) waliduje stan save'a podczas synchronizacji online i flaguje konfiguracje niemożliwe do osiągnięcia w retail. System edukuje użytkownika o **dlaczego** dana edycja może skutkować banem — zamiast tylko blokować lub straszyć.

**Filozofia**: nigdy nie twierdzimy oficjalnie że "FromSoftware zbanuje za X". Tekstowe opisy używają sformułowań typu *"community-reported"*, *"reported bans on r/Eldenring"*, *"detection rules whose exact mechanism is not publicly documented"*.

---

## Tier system

| Tier | Znaczenie | Reakcja UI |
|---|---|---|
| **0** | Cosmetic / read-only / safe | Brak oznaczenia |
| **1** | Caution — popularne ale wykrywalne (bulk grace, map reveal, quest skip) | Modal-confirm wyłącznie gdy Online Safety Mode włączony; off-mode = akcja od razu |
| **2** | High risk — znane z banów (cut content, stat >99, runy >999M) | Modal-confirm (gdy Safety Mode) + outline pól + clamping na inputach |

---

## Online Safety Mode

Globalny przełącznik w `Settings → Safety`. Stan: `localStorage.setItem('setting:onlineSafetyMode', 'true'|'false')`.

Gdy aktywny:
- **Globalny banner** na górze aplikacji (żółty pasek `SafetyModeBanner.tsx`)
- **Tier 1 / Tier 2**: każda akcja z `RiskActionButton` pokazuje modal-confirm; modal nie ma checkboxa "Don't ask again" — zamiast tego stała notka "Online Safety Mode is on — confirmation required"
- **Tier 2 inputs**: edycje są clampowane do legalnych wartości (np. Runes auto-cap do `999_999_999` z toast)

Gdy wyłączony:
- **Brak modali** dla `RiskActionButton` — kliknięcie wywołuje akcję od razu. Edukacja pozostaje dostępna na żądanie przez ⚠ info icon obok każdego przycisku
- **Inputy** dalej walidują/clampują (Tier 2 nie potrzebuje modala — UI nie pozwala wpisać wartości spoza zakresu)

Hook: `useSafetyMode()` z `frontend/src/state/safetyMode.tsx` zwraca `{enabled, setEnabled, isDisabledFor(tier), requireConfirmFor(tier)}`. `requireConfirmFor` zwraca `true` tylko gdy `enabled && tier >= 1`.

---

## Dictionary `RISK_INFO`

Lokalizacja: `frontend/src/data/riskInfo.ts`.

Struktura wpisu:
```ts
interface RiskEntry {
    tier: 0 | 1 | 2;
    level: 'low' | 'medium' | 'high';  // wpływa na kolor (yellow/orange/red)
    title: string;
    whyBan: string;       // opis mechaniki detekcji
    reports: string;      // skala/częstość zgłoszeń bez wymyślania liczb
    mitigation: string;   // konkretna porada jak ograniczyć ryzyko
    sources: { label: string; url?: string }[];  // URL opcjonalny (puste gdy niezweryfikowane)
}
```

`RiskKey` (string union) jest typowany — nowe wpisy rozszerzają unię. TypeScript wymusza obecność każdego klucza w `Record<RiskKey, RiskEntry>`.

### Aktualne wpisy

**Per-flag (Tier 2)** — zsynchronizowane z `backend/db/db.go::ItemEntry.Flags`:
- `cut_content`, `pre_order`, `dlc_duplicate`, `ban_risk`

**Per-action / per-field (Tier 2)** — Faza 3:
- `runes_above_999m`, `stat_above_99`, `level_above_713`, `talisman_pouch_above_3`
- `quantity_above_max`, `spirit_ash_above_10`, `derived_stat_manual`

**Per-bulk-action (Tier 1)** — Faza 4:
- `bulk_grace_unlock`, `bulk_boss_kill`, `bulk_cookbook`, `bulk_bell_bearing`
- `bulk_gestures_unlock`, `bulk_region_unlock`, `bulk_summoning_pool`, `bulk_colosseum`
- `map_reveal_full`, `fow_remove`
- `quest_step_skip`, `ng_plus_write`, `character_import`

### Helper functions

```ts
getRunesRiskKey(runes: number): RiskKey | null;          // > 999_999_999
getAttributeRiskKey(value: number): RiskKey | null;       // > 99
getLevelRiskKey(level: number): RiskKey | null;           // > 713
getTalismanPouchRiskKey(slots: number): RiskKey | null;   // > 3
getQuantityRiskKey(qty, max): RiskKey | null;             // qty > max
getSpiritAshRiskKey(upgrade: number): RiskKey | null;     // > 10
```

Zwracają `RiskKey` (string) gdy wartość jest Tier 2, `null` w innym wypadku — gotowe do użycia w warunku `value && <Component riskKey={value}/>`.

---

## Komponenty

### `<RiskInfoIcon riskKey="..."/>`
Plik: `frontend/src/components/RiskInfoIcon.tsx`.

Klikalna ikona ⚠ (kolor zsync z `level`: yellow/orange/red). Klik → popover (positioned via `getBoundingClientRect`, renderowany przez `createPortal(..., document.body)` — niezależny od overflow rodziców).

Popover zawiera: tytuł, kropki ryzyka (`● ○ ○` / `● ● ○` / `● ● ●`), 3 sekcje (Why / Reports / Mitigation), listę źródeł, ESC + outside-click + click-toggle dismissal.

### `<RiskBadge flag="cut_content"/>`
Plik: `RiskBadge.tsx`.

Renderuje stylowany badge (`CUT`, `⚠ BAN`, `PRE-ORDER`, `DLC DUP`) + obok klikalna `RiskInfoIcon`. Tylko dla per-flag riskKeys (per-action keys nie mają badge'a).

### `<RiskActionButton riskKey="..." onConfirm={...}>Label</RiskActionButton>`
Plik: `RiskActionButton.tsx`.

Wrapper na `<button>`:
- Renderuje button + obok klikalną ikonę ⚠ (osobny target — klik ikony NIE odpala akcji)
- Klik buttona: jeśli `RISK_INFO[riskKey]` istnieje i `safetyMode.requireConfirmFor(tier)` → modal-confirm; w przeciwnym razie `onConfirm()` od razu
- Modal: opis z dictionary, stała notka "Online Safety Mode is on — confirmation required", buttony Cancel/Proceed
- Bez SafetyMode → ⚠ ikona obok pozostaje jako edukacyjna afordancja na żądanie

### `<RiskSectionBanner riskKey="..."/>`
Plik: `RiskSectionBanner.tsx`.

Pasek nad sekcją z ostrzeżeniem o ryzykach całej kategorii edycji (np. "Quest Step Skip" nad listą questów). Pierwsze zdanie z `whyBan` + ikona info. Kolor zsync z `level`.

### `<SafetyModeBanner/>`
Plik: `SafetyModeBanner.tsx`.

Globalny pasek na górze aplikacji widoczny gdy `useSafetyMode().enabled`. Statyczny tekst "Online Safety Mode — Tier 2 edits disabled, Tier 1 requires confirmation".

---

## Mapa pokrycia (gdzie używamy)

| Komponent | Tier | Wzorzec |
|---|---|---|
| **CharacterTab** | | |
| ↳ Runes input | 2 | Outline + ikona ⚠ + clamping pod SafetyMode |
| **InventoryTab / DatabaseTab** | | |
| ↳ Item z flagą ban_risk/cut_content | 2 | RiskBadge inline |
| ↳ Modal "Add Anyway" przed dodaniem ban_risk | 2 | Modal warning (osobny od RiskActionButton) |
| **WorldTab** | | |
| ↳ Map → Reveal All | 1 | RiskActionButton (`map_reveal_full`) + section banner |
| ↳ Graces → Unlock All | 1 | RiskActionButton (`bulk_grace_unlock`) |
| ↳ Summoning Pools → Activate All | 1 | RiskActionButton (`bulk_summoning_pool`) |
| ↳ Colosseums → Unlock All | 1 | RiskActionButton (`bulk_colosseum`) |
| ↳ Bosses → Kill All | 1 | RiskActionButton (`bulk_boss_kill`) |
| ↳ Quests → Set (per step) | 1 | RiskActionButton (`quest_step_skip`) + section banner |
| ↳ Gestures → Unlock All | 1 | RiskActionButton (`bulk_gestures_unlock`) |
| ↳ Gestures z flagą ban_risk | 2 | RiskInfoIcon obok labela |
| ↳ Cookbooks → Unlock All | 1 | RiskActionButton (`bulk_cookbook`) |
| ↳ Bell Bearings → Unlock All | 1 | RiskActionButton (`bulk_bell_bearing`) |
| ↳ Regions → Unlock All | 1 | RiskActionButton (`bulk_region_unlock`) |
| **CharacterImporter (Tools)** | 1 | RiskActionButton (`character_import`) na Confirm |
| **SettingsTab** | | |
| ↳ Online Safety Mode toggle | — | Checkbox + opis |
| ↳ Show Cut & Ban-Risk Items toggle | — | Checkbox display filter |

**NIE objęte** (świadomie):
- `Lock All` / `Reset` / `Respawn All` — Tier 0 (revert do bezpiecznego stanu)
- Talisman pouch / NG+ / atrybuty / quantity inputy — clamping w `onChange` zapobiega Tier 2 przez UI; outline jest martwy dla tych pól
- Whetblades — niski risk (bonus weapon upgrade), pominięte w pierwszej iteracji

---

## Jak dodać nowy risk

1. **Dictionary entry**: dodaj klucz do `RiskKey` union i wpis w `RISK_INFO` w `frontend/src/data/riskInfo.ts`. TypeScript wymusi że nie zapomnisz pól.
2. **Per-flag (badge)**: dodaj wpis w `STYLE` w `RiskBadge.tsx` (label + Tailwind classes).
3. **Per-action (modal-confirm)**: użyj `<RiskActionButton riskKey="...">` zamiast `<button>` w miejscu wywołującym akcję.
4. **Per-field (outline)**: dodaj helper `getXxxRiskKey()` w `riskInfo.ts`, w komponencie warunkowo renderuj outline + `<RiskInfoIcon>`.
5. **Per-section (banner)**: użyj `<RiskSectionBanner riskKey="...">` na górze sekcji.

---

## Plan rozszerzeń (przyszłość)

- **Per-action override dla per-flag wpisów**: gdy konkretna akcja ma inny opis niż generic `cut_content` (np. dodanie cut helmu vs cut quest itemu) — można wprowadzić warstwę override.
- **CharacterTab top banner gdy save ma Tier 2 wartości**: wykryć po loadzie i wyświetlić ostrzeżenie "This save was edited with values flagged as Tier 2 by the community".
- **Sources URLs**: weryfikacja i uzupełnienie pustych `url` w `sources` (np. linki do konkretnych Reddit threads).
- **Dictionary completeness test**: skrypt który grepuje wszystkie użycia `riskKey="..."` / `RISK_INFO[...]` i sprawdza spójność z typem `RiskKey`.

---

## Źródła

- **Pliki**: `frontend/src/data/riskInfo.ts`, `frontend/src/state/safetyMode.tsx`, `frontend/src/components/Risk{InfoIcon,Badge,ActionButton,SectionBanner}.tsx`, `frontend/src/components/SafetyModeBanner.tsx`.
- **Backend tags**: `backend/db/db.go::ItemEntry.Flags` — lista flag (`cut_content`, `pre_order`, `dlc_duplicate`, `ban_risk`, `dlc`, `stackable`).
- **Społeczność**: r/Eldenring threads o banach (2022-2024), Fextralife notatki o cut content.
