# ER Save Editor

> **WARNING: This software is in early development and is NOT stable. It can corrupt your save files beyond recovery. Do NOT use it on your main account or primary save files. Always work on copies. You have been warned.**

Desktop application for editing Elden Ring save files (`.sl2` / `memory.dat`). Built with [Wails v2](https://wails.io/) (Go backend + React/TypeScript frontend).

## Features

- Read and write binary save files for **PC (Steam)** and **PS4**
- Two-way platform conversion (PS4 ↔ PC)
- Character stats editing (level, attributes, runes)
- Inventory and storage management (add/remove items, set quantities)
- Equipment and spell loadout editing
- Sites of Grace unlock/lock
- Event flags (bosses, summon pools)
- DLC support (Scadutree Blessing, Shadow Realm Blessing)
- Automatic backup before every save
- AES-128 encryption/decryption for PC saves

## Supported Platforms

| Save Format | File | Encryption | Status |
|---|---|---|---|
| PC (Steam) | `ER0000.sl2` | AES-128-CBC | Supported |
| PS4 | `memory.dat` | None | Supported (priority) |

## Building

Requirements: Go 1.23+, Node.js 20+, [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)

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
│   ├── core/        # Save file I/O: reader, writer, crypto, structures
│   ├── db/          # Game database: items, graces, event flags
│   └── vm/          # ViewModel: maps binary data to UI-friendly structs
├── frontend/src/    # React + TypeScript + Tailwind CSS
├── tests/           # Round-trip and unit tests
└── Makefile
```

## Documentation

- [SL2 Binary Format Specification](docs/sl2-binary-format-spec.md) — full technical spec of the `.sl2` save file format (offsets, structures, crypto, checksums)
- [Roadmap](docs/ROADMAP.md) — planned features and progress
- [Changelog](docs/CHANGELOG.md) — release history

## License

This project is not affiliated with FromSoftware or Bandai Namco.
