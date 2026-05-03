# 08 — Spells, Gestures, Projectiles

> **Zakres**: Zapamiętane zaklęcia (attunement), gesty, pociski i powiązane dane ekwipunku.

---

## Opis ogólny

Po inwentarzu następuje seria struktur opisujących:
1. Equipped Spells — zapamiętane zaklęcia/inkantacje (attunement slots)
2. Equipped Items — quick slots i pouch
3. Equipped Gestures — aktywne gesty (przypisane do kółka)
4. Acquired Projectiles — zebrane pociski (zmienna długość!)
5. Equipped Armaments & Items — dodatkowe dane ekwipunku
6. Equipped Physics — mieszanki Wonderous Physick

---

## 1. Equipped Spells (14 slotów × 8B = 112 bytes)

14 slotów zaklęć (attunement). Liczba dostępnych slotów zależy od Mind (memory stones).

### Struktura per slot (8 bytes):

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | u32 | SpellID | Magic ID zaklęcia (z MagicParam) |
| 0x04 | u32 | Quantity | Ilość (zwykle 1; 0 lub 0xFFFFFFFF = empty) |

### Dodatkowe pole:
| Offset | Typ | Pole | Opis |
|---|---|---|---|
| (end) | i32 | SelectedSlotIdx | Aktualnie wybrany slot (-1 = none, 0–13 = slot) |

- Każdy slot zawiera Item ID zaklęcia (nie handle — zaklęcia nie mają instancji w GaItem Map)
- Niewykorzystane sloty: SpellID = `0xFFFFFFFF`, Quantity = 0
- Stride między slotami: 8 bytes
- Max 14 slotów (z Memory Stones); dostępność zależy od Mind stat

### Przykładowe Spell IDs (z MagicParam):

| ID | Zaklęcie | Typ |
|---|---|---|
| 4000 | Glintstone Pebble | Sorcery |
| 4010 | Glintstone Arc | Sorcery |
| 4100 | Carian Slicer | Sorcery |
| 6000 | Heal | Incantation |
| 6300 | Lightning Spear | Incantation |
| 6600 | Catch Flame | Incantation |

---

## 2. Equipped Items (Quick Slots + Pouch) (64 bytes)

| Sekcja | Ilość | Rozmiar | Opis |
|---|---|---|---|
| Quick Slots | 10 | 40B (10 × u32) | Szybki dostęp (D-pad cycling) |
| Pouch | 6 | 24B (6 × u32) | Pouch slots (hold Y/Triangle + D-pad) |

Każdy slot: u32 Item ID. Wartość `0xFFFFFFFF` = pusty.

### Quick Slots (10):
- Slot 1–10: przedmioty dostępne przez cycling (D-pad dół)
- Gracz widzi aktualnie wybrany i może cycling'ować

### Pouch (6):
- Slots 1–4: dostępne przez skrót (hold Y + kierunek)
- Slots 5–6: dostępne tylko z menu

---

## 3. Equipped Gestures (gesture ring)

Gesty przypisane do "gesture ring" (szybki dostęp do emotes):
- 6–7 slotów × u32 gesture ID
- Stride: 4 bytes
- Wartość 254 (0xFE) = None (slot pusty)

---

## 4. Acquired Projectiles — UWAGA: ZMIENNA DŁUGOŚĆ

```
┌─────────────────────────────────┐
│ Count (u32)                      │  4 bytes
├─────────────────────────────────┤
│ Projectile entries: count × 8    │  [VARIABLE]
│   ├── projectile_id (u32)        │
│   └── unk (u32)                  │
└─────────────────────────────────┘
```

**To jest jedna z sekcji zmiennej długości** — jej rozmiar zależy od liczby zebranych pocisków. Wszystkie sekcje po niej mają przesunięte offsety.

---

## 5. Equipped Armaments & Items

Dodatkowa struktura equipment — dane o wyposażeniu broni w kontekście ash of war / affinity. Szczegóły do weryfikacji.

---

## 6. Equipped Physics (Wonderous Physick)

2 sloty na crystal tears (krystaliczne łzy) do Flask of Wondrous Physick:

| Offset | Typ | Pole | Opis |
|---|---|---|---|
| 0x00 | u32 | CrystalTear1 | ID pierwszej łzy |
| 0x04 | u32 | CrystalTear2 | ID drugiej łzy |

Wartość `0xFFFFFFFF` = slot pusty.

---

## 7. Gestures (pełna lista — 256 bytes)

Oddzielna sekcja (nie mylić z Equipped Gestures!) — pełna lista 64 gesture IDs (64 × u32 = 256 bytes). Zawiera WSZYSTKIE odblokowane gesty, nie tylko te w gesture ring.

### Kompletna lista Gesture IDs

| ID | Gesture (EN) | ID | Gesture (EN) |
|---|---|---|---|
| 0 | Bow | 108 | Fire Spur Me |
| 2 | Polite Bow | 110 | The Carian Oath |
| 4 | My Thanks | 120 | Bravo! |
| 6 | Curtsy | 140 | Jump for Joy |
| 8 | Reverential Bow | 142 | Triumphant Delight |
| 10 | My Lord | 144 | Fancy Spin |
| 12 | Warm Welcome | 146 | Finger Snap |
| 14 | Wave | 160 | Dejection |
| 16 | Casual Greeting | 180 | Patches' Crouch |
| 18 | Strength! | 182 | Crossed Legs |
| 20 | As You Wish | 184 | Rest |
| 40 | Point Forwards | 186 | Sitting Sideways |
| 42 | Point Upwards | 188 | Dozing Cross-Legged |
| 44 | Point Downwards | 190 | Spread Out |
| 46 | Beckon | 192 | Fetal Position |
| 48 | Wait! | 194 | Balled Up |
| 50 | Calm Down! | 196 | What Do You Want? |
| 60 | Nod In Thought | 200 | Prayer |
| 80 | Extreme Repentance | 202 | Desperate Prayer |
| 82 | Grovel For Mercy | 204 | Rapture |
| 100 | Rallying Cry | 206 | Erudition |
| 102 | Heartening Cry | 208 | Outer Order |
| 104 | By My Sword | 210 | Inner Order |
| 106 | Hoslow's Oath | 212 | Golden Order Totality |
| — | — | 216 | The Ring (Pre-order DLC) |
| — | — | 218 | The Ring (Co-op variant) |
| — | — | 254 | None (empty slot) |

**Gesture unlock** jest kontrolowane przez Event Flags (zakres 60800–60849).

---

## Implikacje dla edycji

- **Spells**: Zmiana SpellID w equipped slot = natychmiastowa zmiana zaklęcia. Nie wymaga dodawania do inventory.
- **Quick Slots/Pouch**: Zmiana Item ID = zmiana przedmiotu w slocie. Przedmiot musi istnieć w inventory.
- **Acquired Projectiles**: Zmienna długość — zmiana count przesuwa wszystko dalej w pliku.
- **Gestures**: Gesture IDs muszą być prawidłowe (z tabeli powyżej). Nieprawidłowe mogą powodować crash.
- **Physics**: Crystal Tear IDs z GoodsParam. Nieprawidłowe ID = crash.
- **14 spell slots**: To max niezależnie od Mind — gra po prostu nie pozwala equip jeśli za mało memory slots.

---

## Źródła

- er-save-manager: `parser/equipment.py` — `EquippedSpells`, `EquippedItems`, `EquippedGestures`, `AcquiredProjectiles`, `EquippedArmamentsAndItems`, `EquippedPhysics`
- er-save-manager: `parser/user_data_x.py` linie 108-117
- er-save-manager: `parser/world.py` — `Gestures` class (linia 63-89, 64 × u32)
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — EquipMagicData (14 slots stride 8), GestureGameData, Gesture IDs dropdown
- Cheat Engine: `ER_TGA_v1.9.0` — EquipMagicData structure, Quick Items, Pouch offsets
