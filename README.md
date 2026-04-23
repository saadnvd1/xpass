# xpass

Terminal password manager. A single-binary replacement for 1Password.

![Go](https://img.shields.io/badge/Go-1.23-blue)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **TUI** — Bubble Tea interactive interface with vim-style navigation
- **AES-256-GCM** encryption with PBKDF2-SHA256 key derivation (600k iterations)
- **Entry types** — logins, API keys, SSH keys, secure notes, credit cards, databases, servers, crypto wallets
- **Password generator** — configurable length, character sets, passphrase mode
- **Clipboard integration** — auto-clears after 30 seconds
- **TOTP support** — store and generate 2FA codes
- **Fuzzy search** — find entries fast
- **Git sync** — multi-device via private git repo (coming soon)
- **CLI mode** — scriptable commands for automation
- **Single binary** — no runtime dependencies

## Install

```bash
# From source
go install github.com/saadnvd1/xpass@latest

# Or build locally
git clone https://github.com/saadnvd1/xpass.git
cd xpass
go build -o xpass .
```

## Usage

### TUI (default)

```bash
xpass
```

#### Keybindings

| Key | Action |
|-----|--------|
| `j/k` | Navigate up/down |
| `enter` | View entry details |
| `/` | Search |
| `a` | Add login |
| `1-4` | Add by type (login/api/ssh/note) |
| `c` | Copy password |
| `s` | Show/hide secrets |
| `e` | Edit entry |
| `d` | Delete entry |
| `f` | Toggle favorite |
| `p` | Password generator |
| `q` | Lock & quit |

### CLI

```bash
xpass init                          # Create vault
xpass get github --copy             # Copy password to clipboard
xpass add github -u user -p pass    # Add entry
xpass list                          # List all entries
xpass gen --copy                    # Generate & copy password
```

## Security

- AES-256-GCM authenticated encryption
- PBKDF2-SHA256 with 600,000 iterations (OWASP minimum)
- Unique salt + IV per encryption operation
- Clipboard auto-clear (30s)
- Vault files written with 0600 permissions
- No telemetry, no network calls (except optional git sync)

## Vault format

Vault stored at `~/.xpass/`. Files are JSON encrypted with AES-256-GCM — compatible with the original xpass-cli TypeScript version for migration.

## License

MIT
