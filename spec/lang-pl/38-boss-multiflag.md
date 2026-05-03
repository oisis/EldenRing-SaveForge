# 38 — Przebudowa wieloflagowego zabijania bossów

> **Typ**: Dokument projektowy
> **Wyodrębniono z**: ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planowany

---

## Problem

Obecna implementacja ustawia tylko 1 flagę zdarzenia na bossa (flaga pokonania 9xxx). To przyznaje runy, ale boss pozostaje żywy w grze. Prawidłowe zabicie/respawn wymaga ustawienia wielu flag na bossa.

## Wymagane flagi na bossa

Każde zabicie bossa wymaga kombinacji:
- **Flaga stanu areny** — oznacza arenę jako "boss pokonany" (zapobiega ścianie mgły)
- **Flaga pokonania** (9xxx) — przyznaje runy, odnotowuje zabicie w zapisie
- **Flagi postępu questów** — NPC reagujące na śmierć bossa
- **Flagi aktywacji Łaski** — Miejsce Łaski po bossie
- **Flagi dropów** — Wspomnienie (Remembrance), unikalne dropy
- **Flagi stanu świata** — zmiany mapy, relokacje NPC

## Dane referencyjne

`tmp/repos/er-save-manager/src/er_save_manager/data/boss_data.py` zawiera 208 bossów z pełnymi listami flag. Struktura:

```python
boss_data = {
    "arena_state_flag": {
        "name": "Boss Name",
        "event_flags": [flag1, flag2, flag3, ...]
    }
}
```

Klucz: flaga stanu areny (identyfikator główny, NIE flaga pokonania 9xxx).

## Plan implementacji

### Zmiana struktury danych

```go
type BossData struct {
    ID          uint32   `json:"id"`          // arena state flag (klucz główny)
    Name        string   `json:"name"`
    Region      string   `json:"region"`
    Type        string   `json:"type"`        // "main" lub "field"
    Remembrance bool     `json:"remembrance"`
    EventFlags  []uint32 `json:"eventFlags"`  // WSZYSTKIE flagi do ustawienia przy zabiciu
}
```

### Algorytm

**Zabicie:**
```
for each flag in boss.EventFlags:
    SetEventFlag(slot.Data[EventFlagsOffset:], flag, true)
```

**Respawn:**
```
for each flag in boss.EventFlags:
    SetEventFlag(slot.Data[EventFlagsOffset:], flag, false)
```

### Migracja

- Zmiana klucza mapy `bosses.go` z flagi pokonania na flagę stanu areny
- Import wszystkich 208 wpisów z `boss_data.py` (obecnie tylko ~120)
- Dodanie bossów DLC z pełnymi listami flag
- Istniejąca metoda `SetBossDefeated(slotIndex, bossID, defeated)` zachowuje to samo API, jedynie iteruje wewnętrznie po wszystkich flagach

### Testowanie

- Wymagana weryfikacja in-game per boss (boss nie powinien pojawiać się po zabiciu)
- Priorytet: główni bossowie najpierw (Godrick, Rennala, Radahn, Morgott, Fire Giant, Godfrey, Radagon/EB)
- Bossowie terenowi w drugiej kolejności (Tree Sentinel, Agheel, Tibia Mariner, itd.)

### Znane komplikacje

- Niektórzy bossowie mają flagi warunkowe (np. Radahn wymaga flag festiwalu)
- Bossowie wielofazowi (Godfrey/Hoarah Loux) mogą wymagać flag stanów pośrednich
- Respawn bossa może nie działać dla bossów wywołujących nieodwracalne zmiany świata (Rykard → Volcano Manor)
