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
    merge.go               # Merge(base, local, remote): pure three-way entry merge
    pull.go                # Vault.Pull: fetch, refuse undecryptable remote, ff or merge
  sync/sync.go             # git plumbing (AutoCommit, Push refuses when behind, fetch/merge helpers)
  tui/
    model.go               # Top-level Bubble Tea model
    styles.go              # Lipgloss styles
    unlock.go              # Password unlock screen
    list.go                # Entry list with search
    detail.go              # Entry detail view
    add.go                 # Add/edit entry form
    generate.go            # Password generator
    delete.go              # Delete confirmation
  otp/otp.go               # TOTP generation, otpauth:// URI parsing
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

## Sync and merge

`xpass pull` never rebases. It fetches, refuses a remote `vault.json` that does
not decrypt with the password (HEAD is left where it was), fast-forwards when
only the remote moved, and otherwise decrypts base (merge-base), local (HEAD)
and remote and runs `vault.Merge` by entry ID. The result is recorded as
`git merge -s ours` + the merged vault.json/config.json amended in (fresh
salt); config.json/history.json keep the local side. It does not push.
"Changed" ignores `lastAccessed`/`accessCount` and treats `tags: null` as `[]`.
Only entry names are ever printed. `pull_e2e_test.go` runs the whole flow in
a temp HOME with a bare repo as the remote (`go test -short` skips it).
