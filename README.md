# xpass

A terminal password manager. Single binary, no cloud, no subscription.

Your passwords live on your machine, encrypted with AES-256-GCM. Browse them in a Bubble Tea TUI or use CLI commands for scripting. Sync across devices with a private git repo. Import your 1Password vault and never look back.

## Install

### From source (recommended)

```bash
git clone https://github.com/saadnvd1/xpass.git
cd xpass
make install
```

This builds the binary and copies it to `/usr/local/bin/xpass`.

### With Go

```bash
go install github.com/saadnvd1/xpass@latest
```

Make sure `~/go/bin` is in your `PATH`:

```bash
export PATH="$HOME/go/bin:$PATH"  # add to ~/.zshrc
```

### Verify

```bash
xpass version
```

## Quick start

```bash
# Create your vault
xpass init

# Launch the TUI
xpass
```

Pick a master password. That's the only thing between your data and the void — there's no recovery if you forget it.

## Import from 1Password

Export your vault from the 1Password desktop app (File > Export), then:

```bash
xpass import ~/Downloads/export.1pux
```

Supports `.1pux`, `.csv`, and `.json` exports. All entry types carry over — logins, secure notes, credit cards, identities, TOTP secrets.

Delete the export file after import. It contains your passwords in plaintext.

## TUI

The default mode. Vim-style navigation, real-time search, live TOTP codes.

```
xpass
```

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate up/down |
| `enter` | View entry details |
| `/` | Search (filters as you type) |
| `a` | Add login |
| `1` `2` `3` `4` | Add login / API key / SSH key / note |
| `c` | Copy password to clipboard |
| `s` | Show/hide secrets |
| `u` | Copy username |
| `t` | Copy TOTP code |
| `e` | Edit entry |
| `d` | Delete entry |
| `f` | Toggle favorite |
| `p` | Password generator |
| `G` / `g` | Jump to bottom/top |
| `q` | Lock vault |

## CLI

For scripting, cron jobs, and quick lookups. Password input is hidden.

```bash
xpass get github              # print password
xpass get github --copy       # copy to clipboard (auto-clears 30s)
xpass add github -u user -p pass --url github.com
xpass list
xpass gen                     # generate password
xpass gen --copy              # generate and copy
xpass import export.csv       # import from 1Password
```

## Multi-device sync

Sync your encrypted vault across machines using a private git repo. The files pushed are AES-256-GCM ciphertext — useless without your master password.

```bash
# Machine 1: set up
xpass remote git@github.com:you/my-vault.git
xpass push

# Machine 2: pull
xpass init        # use the SAME master password
xpass remote git@github.com:you/my-vault.git
xpass pull
```

Every add/edit/delete auto-commits locally. `push` when done, `pull` on other machines.

No key file to transfer between machines. Same password = same decryption key via PBKDF2. The salt travels with the encrypted files.

```bash
xpass push        # push changes to remote
xpass pull        # pull from remote
xpass sync        # check sync status
```

## Security

| | |
|---|---|
| Encryption | AES-256-GCM |
| Key derivation | PBKDF2-SHA256, 600,000 iterations |
| Salt | Random 32 bytes, unique per encryption |
| IV/Nonce | Random, unique per encryption |
| Clipboard | Auto-clears after 30 seconds |
| File permissions | 0600 on all vault files |
| Sync | Git-based, encrypted files only |
| Master password | Never stored — verified via GCM auth tag |
| Dependencies | Go stdlib crypto + x/crypto. No third-party crypto. |

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
  ~/.xpass/vault.json   (encrypted entries)
  ~/.xpass/config.json  (encrypted config)
  ~/.xpass/history.json (encrypted history)
```

No daemon. No agent. No background process. Unlock, use, quit. Data encrypted at rest.

## Entry types

- **Login** — username, email, password, URL, TOTP
- **API Key** — key, secret, endpoint
- **SSH Key** — private/public key, passphrase
- **Secure Note** — freeform encrypted text
- **Credit Card** — number, CVV, expiry, PIN
- **Database** — type, host, port, credentials
- **Server** — host, protocol, credentials
- **Crypto Wallet** — address, seed phrase, private key

## Vault location

Everything lives at `~/.xpass/`. Back it up, git-sync it, or leave it local.

## License

MIT
