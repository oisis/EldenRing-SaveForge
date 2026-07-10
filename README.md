# Elden Ring SaveForge

> **WARNING: This software is in early development and is NOT stable. It can corrupt your save files beyond recovery. Do NOT use it on your main account or primary save files. Always work on copies. You have been warned.**

Desktop application for editing Elden Ring save files (`.sl2` / `memory.dat`). Built with [Wails v2](https://wails.io/) (Go backend + React/TypeScript frontend).

## Screenshots

<p align="center">
  <a href="docs/screenshots/2.png"><img src="docs/screenshots/2.png" width="49%" alt="Character - stats and profile"/></a>
  <a href="docs/screenshots/1.png"><img src="docs/screenshots/1.png" width="49%" alt="Appearance presets gallery"/></a>
</p>
<p align="center">
  <a href="docs/screenshots/5.png"><img src="docs/screenshots/5.png" width="49%" alt="Item Database - weapons grid"/></a>
  <a href="docs/screenshots/3.png"><img src="docs/screenshots/3.png" width="49%" alt="Inventory list"/></a>
</p>
<p align="center">
  <a href="docs/screenshots/4.png"><img src="docs/screenshots/4.png" width="49%" alt="Item Database - list view with favorites"/></a>
  <a href="docs/screenshots/6.png"><img src="docs/screenshots/6.png" width="49%" alt="World - map reveal and Sites of Grace"/></a>
</p>
<p align="center">
  <a href="docs/screenshots/7.png"><img src="docs/screenshots/7.png" width="49%" alt="World - unlocks and progression"/></a>
  <a href="docs/screenshots/8.png"><img src="docs/screenshots/8.png" width="49%" alt="Advanced - NetworkParam tuning"/></a>
</p>
<p align="center">
  <a href="docs/screenshots/9.png"><img src="docs/screenshots/9.png" width="49%" alt="Tools - templates, diagnostics, conversion, console"/></a>
  <a href="docs/screenshots/10.png"><img src="docs/screenshots/10.png" width="49%" alt="Tools - deploy, safety profile, save management"/></a>
</p>

## Features

**Save file support**
- PC (Steam `.sl2`) and PS4 (`memory.dat`) - read, edit, write
- PC <-> PS4 format conversion
- PC AES-128-CBC encryption/decryption support
- Automatic backups for save writes and an explicit backup gate before Chaos Mode
- Preview mode for browsing appearance presets and the item database before loading a save

**Safety profiles**
- **Safe** - conservative vanilla-style caps, risky/cut-content items hidden, online-safety gates enabled
- **Expanded Limits** - technical game caps for normal items while risky/cut-content items stay hidden
- **Chaos** - technical game caps and risky/cut-content items visible, protected by a warning and optional backup step
- Risk badges, warnings, confirmation gates, and ban-risk labels on dangerous actions

**Character**
- Edit level, all 8 attributes, runes, NG+ cycle, class, body type, talisman slots, and memory stones
- Shadow of the Erdtree fields: Scadutree Blessing and Shadow Realm Blessing
- Quick-add controls for weapon upgrades, infusions, spirit ashes, talismans, and related loadout helpers
- Appearance preset gallery with preview images and Mirror Favorites integration
- Character slot management: clone, delete, activate/deactivate, clean residual inactive slots, undo recent edits

**Inventory & Item Database**
- Large item database with icons, search, categories, favorites, grid/list views, and item detail panels
- Add items to inventory and storage with capacity preflight and platform-aware save writing
- Equipment inventory, storage box, weapon edit workflow, transfer/remove/reorder actions, and sort-order tools
- Per-item conservative caps in Safe profile and regulation-backed technical caps in Expanded Limits / Chaos
- Owned/max counts, inventory/storage capacity bars, and safeguards for stackable key items, talismans, arrows, containers, and game-managed unlock items

**Diagnostics & repair**
- Automatic post-load inventory scan with issue grouping and repair actions
- Manual Diagnostics scan from Tools
- Binary save diagnostics plus workspace validation for inventory/storage issues
- Repair flows for supported issues, including quantity clamps, duplicate/index problems, weapon workspace issues, and selected loaded-save repairs

**World**
- Map reveal, map fragments, fog-of-war controls, DLC cover-layer handling
- Sites of Grace, Summoning Pools, Colosseums, bosses, invasion regions, and quest progress
- Unlocks for gestures, cookbooks, bell bearings, and whetblades
- Bulk actions with safety prompts where edits can affect online state

**Advanced / Network**
- NetworkParam tuning for invader, summon host, summon guest, and hunter/ring-search groups
- Vanilla, Faster/Better, and Aggressive group presets from backend-defined values
- Manual sliders with validation and in-app activation instructions
- Dangerous networking operations are labelled and gated by the active safety profile

**Templates**
- Global Templates library for reusable build templates
- Create templates from the currently loaded character
- Preview, apply, rename, delete, rebuild library index, and export templates
- Public YAML import from file or HTTPS URL with preview before saving or applying
- Template apply paths for supported character, stats, inventory, equipment, spells, items, and layout sections, with warnings for skipped or unsupported entries

**Tools**
- Format conversion between PC and PS4 saves
- Deploy targets over SSH or local filesystem: upload, download, launch, close game, deploy-and-launch, close-and-download
- Save Manager for configured deploy targets
- Favorite item manager and in-app operation console
- Theme, column visibility, Steam ID editing for PC saves, and debug-mode controls

## Supported Platforms

| Save Format | File | Encryption | Status |
|---|---|---|---|
| PC (Steam) | `ER0000.sl2` | AES-128-CBC | Supported |
| PS4 | `memory.dat` | None | Supported |

## Building

Requirements: Go 1.25+, Node.js 20+, [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)

```bash
# Install dependencies
make deps

# Build for current platform
make build

# Run in development mode (requires GUI)
make dev

# Run tests
make test
```

## Development

```
.
├── backend/
│   ├── core/        # Save file I/O, crypto, structures, diagnostics, repair primitives
│   ├── db/          # Game database: items, graces, regions, flags, limits
│   ├── deploy/      # SSH/local deploy target support
│   ├── editor/      # Inventory workspace, validation, repair, weapon editing
│   ├── templates/   # Build template schema, library, import/export/apply logic
│   └── vm/          # ViewModel mapping between binary data and UI structs
├── frontend/src/    # React + TypeScript + Tailwind CSS
├── tests/           # Round-trip and integration-style tests
└── Makefile
```

## Documentation

- [SL2 Binary Format Specification](docs/sl2-binary-format-spec.md) - full technical spec of the `.sl2` save file format (offsets, structures, crypto, checksums)
- [Roadmap](docs/ROADMAP.md) - planned features and progress
- [Changelog](docs/CHANGELOG.md) - release history

## License

This project is not affiliated with FromSoftware or Bandai Namco.
