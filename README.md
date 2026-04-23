# xpass

A terminal password manager. Single binary, no cloud, no subscription.

Your passwords encrypted locally with AES-256-GCM. A Bubble Tea TUI for browsing, or CLI commands for scripting. Import your 1Password vault and never look back.

## Install

```bash
go install github.com/saadnvd1/xpass@latest
```

Or build from source:

```bash
git clone https://github.com/saadnvd1/xpass.git
cd xpass && go build -o xpass .
```

## Quick start

```bash
# Create your vault
xpass init

# Launch the TUI
xpass
```

That's it. Pick a master password, start adding entries.

## Import from 1Password

Export your 1Password vault (desktop app > File > Export), then:

```bash
xpass import 1password-export.csv
```

Supports CSV and JSON exports. All entry types are mapped: logins, secure notes, credit cards, identities. TOTP secrets carry over too.

After import, delete the export file — it contains your passwords in plaintext.

## TUI

The default mode. Vim-style navigation, fuzzy search, everything you need.

```
  xpass
```

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate |
| `enter` | View entry |
| `/` | Search |
| `a` | Add login |
| `1` `2` `3` `4` | Add login / API key / SSH key / note |
| `c` | Copy password to clipboard |
| `s` | Show/hide secrets |
| `u` | Copy username |
| `e` | Edit entry |
| `d` | Delete entry |
| `f` | Toggle favorite |
| `p` | Password generator |
| `q` | Lock vault |

## CLI

For scripting, cron jobs, and quick lookups.

```bash
# Get a password (prints to stdout)
xpass get github

# Get and copy to clipboard (auto-clears in 30s)
xpass get github --copy

# Add an entry
xpass add github --username user --password pass --url github.com

# List everything
xpass list

# Generate a password
xpass gen
xpass gen --copy

# Import from 1Password
xpass import export.csv
```

## Security

| | |
|---|---|
| Encryption | AES-256-GCM |
| Key derivation | PBKDF2-SHA256, 600,000 iterations |
| Salt | Random 32 bytes per encryption |
| IV/Nonce | Random per encryption |
| Clipboard | Auto-clears after 30 seconds |
| File permissions | 0600 on all vault files |
| Network | Zero network calls (no telemetry, no cloud) |
| Master password | Never stored — derived key verified via GCM auth tag |

The vault at `~/.xpass/` contains only encrypted JSON. Wrong password = GCM authentication failure. No password hash stored anywhere.

## Entry types

- **Login** — username, email, password, URL, TOTP
- **API Key** — key, secret, endpoint
- **SSH Key** — private/public key, passphrase
- **Secure Note** — freeform encrypted text
- **Credit Card** — number, CVV, expiry, PIN
- **Database** — host, port, credentials, connection string
- **Server** — host, protocol, credentials
- **Crypto Wallet** — address, private key, seed phrase

## How it works

```
Master password
      |
      v
  PBKDF2-SHA256 (600k iterations + random salt)
      |
      v
  256-bit key
      |
      v
  AES-256-GCM encrypt/decrypt
      |
      v
  ~/.xpass/vault.json  (encrypted entries)
  ~/.xpass/config.json (encrypted config)
```

No daemon, no agent, no background process. Unlock, use, quit. Data stays encrypted at rest.

## License

MIT
