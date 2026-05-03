# 41 — Info-Tab Item Ground Drop

> **Type**: Investigation (paused)
> **Extracted from**: ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🐛 Paused — EMEVD decompilation needed

---

## Symptom

Adding 1/0-cap items from the Info tab (Notes, About tutorials, Letters, Maps, Cookbooks) via the editor causes the world copy to drop on the ground when the player walks past the trigger location in-game. Example: Crafting Kit purchase at Kalé spawns "Tworzenie przedmiotów" on the ground because the player already has it.

**Ban risk:** None. Vanilla NG+ produces the same behaviour. Cosmetic clutter only.

---

## What we tried (2026-04-29)

### Approach 1: WorldPickupFlagID map

- Extracted `getItemFlagId` from `ItemLotParam_map` (cat=1) and `eventFlag_forStock` from `ShopLineupParam` (equipType=3) in regulation.bin
- 308 entries in `backend/db/data/world_pickup_flags.go`
- Hooked into `AddItemsToCharacter` to set the flag for the world copy
- **Result:** flag set correctly in save (confirmed by save diff: flag 550130 for About Item Crafting written), item still drops in-game
- **Conclusion:** `getItemFlagId` / `eventFlag_forStock` are not the flags that gate the EMEVD spawn

### Approach 2: TutorialDataChunk (AboutTutorialID)

- Discovered `TutorialDataChunk` block (0x408 bytes) at `slot.TutorialDataOffset`
- Layout: `unk0x0 u16 | unk0x2 u16 | size u32 | count u32 | u32 IDs[count]`
- Buying Crafting Kit appended ID `2010` to the list (verified by save diff)
- Pre-populated via `core.AppendTutorialID` (clean save edit confirmed: count 8 → 9, surgical 13-byte change)
- **Result:** list correctly modified in save, item still drops in-game
- **Conclusion:** Tutorial ID 2010 controls the popup text appearance, not the item-give EMEVD action

---

## Conclusion

The give/spawn action for Info-tab tutorial items is gated by a check we have not yet identified. Likely candidates:

- Hardcoded EMEVD instruction (`event/m??_??_??_??.emevd.dcx` inside `Data0.bdt`) that bypasses both `getItemFlagId` and `TutorialDataChunk`
- A separate region-state bitset somewhere in slot.Data we have not located
- A flag in the EMEVD-emitted "tutorialFlagId" range (710xxx, 720xxx) that we have not exhaustively tried

---

## Next investigation steps (when resumed)

1. Extract `Data0.bdt` from Steam Deck (`~/.local/share/Steam/steamapps/common/ELDEN RING/Game/`)
2. Decrypt BHD with public RSA key
3. Decompile `event/common.emevd.dcx` (and area-specific `event/m11_*.emevd.dcx` for Stranded Graveyard / Limgrave) using community tools
4. Search EMEVD for `give_item(9113, 1)` / `give_item(9135, 1)` patterns and identify the gating flag
5. Alternative: empirical save-diff matrix — for each About item, take BEFORE save → trigger EXACTLY one item in-game → diff and find the unique byte change

---

## Files retained for resumption

- `backend/core/tutorial_data.go` — parser/writer for TutorialDataChunk (works as designed)
- `backend/db/data/tutorial_ids.go` — AboutTutorialID map (populate as future findings come in)
- `backend/db/data/world_pickup_flags.go` — 308 entries (useful for items where the flag DOES gate the spawn)
- `app.go` `AddItemsToCharacter` hooks for both maps — harmless no-op when gating mechanism isn't triggered
