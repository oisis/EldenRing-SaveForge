# Item Caps Enforcement (NG+ Scaling + Full Chaos Mode)

## Cel

Limit `MaxInventory` / `MaxStorage` w bazie itemów odzwierciedla **realną liczbę sztuk osiągalnych w jednym przejściu gry** (single playthrough). Dla itemów które przenoszą się do NG+ i ponownie spawnują, cap skaluje się liniowo z licznikiem NG+ cyklu (`ClearCount`). Power-user może wyłączyć cały mechanizm w Settings (Full Chaos Mode).

**Główny cel:** ograniczenie ryzyka detekcji EAC poprzez utrzymanie ilości itemów w zakresie statystycznie zgodnym z vanilla. Drugoplanowy: spójność z UI gry (np. cookbook = 1 sztuka, fizycznie nie da się mieć 2).

## Mechanizm scaling

```
effective_cap = MaxInventory × (ClearCount + 1)   if item.flags has "scales_with_ng"
effective_cap = MaxInventory                      otherwise
effective_cap = 999                               if Full Chaos Mode is ON
```

`ClearCount` (uint32, 0..7+) leży w save w dynamicznym łańcuchu offsetów po BloodStain — patrz `backend/core/structures.go:178` i `offset_defs.go:101`. Eksponowany do frontendu jako `CharacterViewModel.clearCount` (`backend/vm/character_vm.go:44,72`).

| ClearCount | Cykl gry | Multiplier |
|---|---|---|
| 0 | NG | ×1 |
| 1 | NG+1 | ×2 |
| 2 | NG+2 | ×3 |
| ... | ... | ... |
| 7 | NG+7 | ×8 |

**TODO**: zweryfikować na realnym save kiedy gra inkrementuje `ClearCount` — przy zabiciu Elden Beast czy przy wejściu w menu NG+? Aktualnie zakładamy 0 = pre-Elden-Beast pierwszego cyklu.

## Item-y z flagą `scales_with_ng`

Wszystkie z bazą per single playthrough zgodnie z Fextralife (v1.16):

| Item | Plik | Base cap | Source |
|---|---|---|---|
| Stonesword Key | `key_items.go` | 55 | ~47 base + ~10 DLC purchases/drops; NG+ respawnuje imp statues |
| Dragon Heart | `key_items.go` | 22 | ~16 base (Greyoll 5 + named dragons) + ~6 DLC; NG+ respawnuje smoki |
| Larval Tear | `key_items.go` | 24 | 18 base + 6 DLC; NG+ respawnuje Albinaurics i drops |

## Item-y z hard cap (bez scaling) — caps to "max useful"

Cap nie zmienia się z NG+ — limit gry, nie limit zbieralności:

| Item | Cap | Powód |
|---|---|---|
| Memory Stone | 8 | Spell slot cap = 8 (game hard limit) |
| Talisman Pouch | 3 | Talisman slot count = 3 (game hard limit) |
| Golden Seed | 30 | Pełne ładunki flaski (14 charges, kumulatywny koszt 30 ziaren) |
| Sacred Tear | 12 | Pełna potency flaski (+12) |
| Scadutree Fragment | 50 | Max Scadutree Blessing (+20, kumulatywny koszt 50) |
| Revered Spirit Ash | 25 | Max Revered Spirit Ash Blessing (+10, kumulatywny koszt 25) |

**Zasada projektowa**: jeśli cap = "max useful" (przekroczenie nie daje funkcjonalnego zysku), nie skalujemy z NG+. Nawet po Elden Beast w NG+1, gracz ma już +12 potency / +14 flask charges / +20 blessing — kolejne ziarna/łzy/fragmenty są bezużyteczne.

## Item-y z cap 1/0 (unique drops)

Per single playthrough fizycznie 1 sztuka (consumable / unique pickup):

- Wszystkie **gestures** (54), **sorceries** (85), **incantations** (128) — memorized once.
- Wszystkie **About \* tutorials** (~37) i **listy / klucze fabuły** w `info.go`.
- **Paintings** (3 base + 9 DLC) — read-once items.
- **Notes** (15 base + 2 DLC) — read-once items.
- **Crystal Tears** (~22 base + 5 DLC) — unique pickups, Twin Maidens 1-time purchases (Cerulean/Viridian Hidden, etc.).
- **Great Runes / Bell Bearings / klucze** (~270) w `key_items.go`.
- **Cookbooks** (109) — wszystkie unique (cookbook się "zużywa" przy oddaniu merchantowi).
- **Multiplayer Fingers** (11) i **Remembrances** (25 = 15 base + 10 DLC) w `tools.go`.

## Full Chaos Mode

Toggle w `SettingsTab.tsx` → Safety section (red-bordered, ban-risk copy):

```
[setting:fullChaosMode] → 'true' / 'false' (default false)
```

- `localStorage` persisted
- Cross-component sync via `window` CustomEvent `'fullChaosModeChanged'` (detail: boolean)
- Po włączeniu: `effectiveCap` ignoruje `MaxInventory`/`scales_with_ng` i zwraca 999

**UX**: gdy włączony, modal w Database tab pokazuje czerwony banner *"⚠ Full Chaos Mode — caps bypassed (max 999)"*.

## Implementacja UI (`DatabaseTab.tsx`)

Helper:
```typescript
function effectiveCap(item, kind, clearCount, chaos): number {
    if (chaos) return 999;
    const base = kind === 'inv' ? item.maxInventory : item.maxStorage;
    if (item.flags?.includes('scales_with_ng')) return base * (clearCount + 1);
    return base;
}
```

Modal `min`/`max` używa `effectiveCap()` zamiast `item.maxInventory`/`item.maxStorage`. Banner **NG+ Scaling** renderowany gdy:
- jakikolwiek z wybranych itemów ma flag `scales_with_ng`
- AND `Full Chaos Mode` wyłączony

Banner pokazuje:
- `Vanilla NG: X` (zawsze)
- `NG+Y: Z` (gdy `clearCount > 0`)
- Edukacyjną linię *"Adding more increases EAC ban risk"*

## Architektura: gdzie clamp jest enforce'owany

| Warstwa | Clamp enforcement |
|---|---|
| `core` (load/save binarne) | **Brak** — odczyt/zapis transparentny |
| `vm.MapParsedSlotToVM` | **Brak** zmian — istniejący clamp w handleSetItemQty zostaje |
| `vm.handleSetItemQty` | Clamp do `MaxInventory`/`MaxStorage` (legacy behavior) |
| `frontend DatabaseTab modal` | **Główne miejsce enforcement** — `min/max` na `<input>`, walidacja przed `AddItemsToCharacter()` |
| `Full Chaos Mode` | Bypass clamp w UI tylko (vm clamp nadal działa, ale max 999 z UI praktycznie wystarczy) |

**Świadoma decyzja**: clamp przy load/save **NIE** modyfikuje wartości w istniejącym save. Gracz który ma 999 Larval Tears (np. z innego edytora) zachowa je przy load → save round-tripie.

## Weryfikacja

- `go test ./backend/...` — pass (data, db, vm, core)
- `npx tsc --noEmit && npm run build` — pass
- Manual test plan:
  1. Save NG → Database → Stonesword Key → modal pokazuje cap **55**
  2. Save NG+3 → cap **220** (=55×4), tooltip *"Vanilla NG: 55 · NG+3: 220"*
  3. Settings → Full Chaos Mode ON → cap **999**, czerwony banner widoczny
  4. Cookbook (Glintstone Craftsman's [1]) → cap **1**, próba dodania 2 zablokowana
  5. Painting → cap **1**, brak storage row
  6. Mohg's Great Rune wyświetla się w category Key Items, nieobecny w Bolstering

## Źródła

- Fextralife wiki per-item Locations (eldenring.wiki.fextralife.com) — primary source
- Cross-checked z: Game8, PowerPyx, GamesRadar, GameSpot, PCGamer
- v1.16 patch (2026-04 stan)
