# xpass

Terminal password manager — Go rewrite of xpass-cli (TypeScript). Single binary, Bubble Tea TUI, AES-256-GCM encryption.

## Stack

- **Language:** Go
- **TUI:** Bubble Tea + Lipgloss
- **Crypto:** AES-256-GCM, PBKDF2-SHA256 (600k iterations)
- **Storage:** Encrypted JSON files at ~/.xpass/

## Structure

```
main.go                    # CLI entry point + subcommands
internal/
  crypto/crypto.go         # AES-256-GCM encrypt/decrypt, password gen
  vault/
    types.go               # Entry types, config structs
    vault.go               # Vault CRUD, encrypt/decrypt files
  tui/
    model.go               # Top-level Bubble Tea model
    styles.go              # Lipgloss styles
    unlock.go              # Password unlock screen
    list.go                # Entry list with search
    detail.go              # Entry detail view
    add.go                 # Add/edit entry form
    generate.go            # Password generator
    delete.go              # Delete confirmation
  importer/onepassword.go  # 1Password CSV/JSON import
  clipboard/clipboard.go   # System clipboard with auto-clear
```

## Commands

```bash
go build -o xpass .    # Build
go run .               # Run TUI
go run . help          # CLI help
```

## Vault compatibility

Vault format matches xpass-cli (TypeScript) for migration. Same AES-256-GCM + PBKDF2 params, same JSON structure.
