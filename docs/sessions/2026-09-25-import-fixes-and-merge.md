# 2026-09-25 — Import fixes, repair/fill, merge on pull, replay refusal

Driven by the homelab web vault (dashboards apps/vault), which reads this
vault's ciphertext from github.com/saadnvd1/xpass-vault and now writes it too.

## Found

- The 1Password importer stored every typed field that was not concealed or
  totp as Go's map print (`map[string:]`, `map[creditCardNumber:…]`).
- Card fields were matched by "title contains number": "verification number"
  and a blank "issue number" overwrote every card number; expiry (YYYYMM) was
  dropped.

## Built

- `internal/importer/unwrap.go`: every 1Password value type unwrapped; expiry
  as YYYYMM or MM/YYYY. Card fields by 1Password field ID (`applyCardField`);
  a blank field never overwrites.
- `xpass repair` (unwrap stored `map[...]` values) and `xpass fill <export>`
  (fill only EMPTY fields of matching entries from an export). Saad ran both.
- `xpass pull` merges entry by entry (3-way on ID, `internal/vault/merge.go`)
  instead of rebasing an encrypted blob, and refuses a remote vault that does
  not decrypt (`d5db5cb`, built by a helper agent). `push` refuses when the
  remote is ahead.
- `pull` refuses a remote vault.json this vault already had — a replayed old
  copy decrypts fine and would bring old passwords back (`278d854`).
- `AutoCommit` no longer fails when history.json is missing (it silently
  committed nothing on new vaults).

## State

Installed at /usr/local/bin/xpass. Tests: importer unit tests, merge rules,
pull end-to-end in a throwaway HOME (merge, undecryptable remote, replay).

## Next

- Saad: one edit from the phone, then `xpass pull` on the Mac.
