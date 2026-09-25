package vault

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/saadnvd1/xpass/internal/crypto"
)

// PullOutcome is what Pull did.
type PullOutcome string

const (
	PullUpToDate    PullOutcome = "up to date"
	PullAhead       PullOutcome = "ahead" // only local has new commits; nothing to take
	PullFastForward PullOutcome = "fast-forward"
	PullMerged      PullOutcome = "merged"
)

// PullResult reports a pull by entry names and counts only.
type PullResult struct {
	Outcome PullOutcome
	Report  MergeReport // set for PullMerged
	Before  int         // entry count before
	After   int         // entry count after
}

// ErrRemoteUndecryptable is returned when the remote vault.json does not open
// with this vault's password; nothing is accepted.
var ErrRemoteUndecryptable = fmt.Errorf("the remote vault does not open with your password — not accepted; git history has the previous version")

// Pull fetches the remote and brings its entries in, merging entry by entry
// when both sides changed. The vault must be unlocked (the password decrypts
// the base, local and remote copies). A remote vault.json that does not
// decrypt with the password, or does not parse as entries, is refused and
// the local repo is left where it was.
func (v *Vault) Pull() (*PullResult, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}
	s := v.sync
	if err := s.PrepareForPull(); err != nil {
		return nil, err
	}
	res := &PullResult{Before: len(v.entries), After: len(v.entries)}

	if !s.HasRemoteBranch() {
		res.Outcome = PullUpToDate // nothing pushed yet
		return res, nil
	}
	ahead, behind, err := s.AheadBehind()
	if err != nil {
		return nil, err
	}
	if behind == 0 {
		res.Outcome = PullUpToDate
		if ahead > 0 {
			res.Outcome = PullAhead
		}
		return res, nil
	}

	// Whatever happens next, the remote must open with our password first.
	remote, err := v.entriesAt(s.RemoteRef())
	if err != nil {
		return nil, ErrRemoteUndecryptable
	}

	orig, err := s.Head()
	if err != nil {
		return nil, err
	}

	if ahead == 0 {
		if err := s.FastForward(); err != nil {
			s.ResetHard(orig)
			return nil, fmt.Errorf("fast-forward failed: %w", err)
		}
		if err := v.reload(); err != nil {
			s.ResetHard(orig)
			v.reload()
			return nil, err
		}
		res.Outcome = PullFastForward
		res.After = len(v.entries)
		return res, nil
	}

	// Diverged: three-way merge by entry.
	baseRev, err := s.MergeBase()
	if err != nil {
		return nil, err
	}
	base, err := v.entriesAt(baseRev)
	if err != nil {
		return nil, fmt.Errorf("the common ancestor's vault does not open with your password: %w", err)
	}
	local, err := v.entriesAt("HEAD")
	if err != nil {
		return nil, fmt.Errorf("the local committed vault does not open with your password: %w", err)
	}

	merged, rep := Merge(base, local, remote)

	fail := func(err error) (*PullResult, error) {
		s.ResetHard(orig)
		v.reload()
		return nil, err
	}
	if err := s.MergeOurs(); err != nil {
		return fail(fmt.Errorf("recording the merge: %w", err))
	}
	prev := v.entries
	v.entries = merged
	if err := v.writeFiles(); err != nil {
		v.entries = prev
		return fail(fmt.Errorf("writing the merged vault: %w", err))
	}
	if err := s.Amend(VaultFile, ConfigFile); err != nil {
		v.entries = prev
		return fail(fmt.Errorf("committing the merged vault: %w", err))
	}

	res.Outcome = PullMerged
	res.Report = rep
	res.After = len(merged)
	return res, nil
}

// entriesAt decrypts and parses vault.json as committed at rev.
func (v *Vault) entriesAt(rev string) ([]Entry, error) {
	raw, err := v.sync.ShowFile(rev, VaultFile)
	if err != nil {
		return nil, err
	}
	return decodeEntries(raw, v.password)
}

func decodeEntries(raw []byte, password string) ([]Entry, error) {
	var enc crypto.EncryptedData
	if err := json.Unmarshal(raw, &enc); err != nil {
		return nil, fmt.Errorf("not an encrypted vault file")
	}
	plain, err := crypto.Decrypt(&enc, password)
	if err != nil {
		return nil, err
	}
	// An array, not just anything that unmarshals into a slice ("null" does).
	if t := strings.TrimSpace(plain); !strings.HasPrefix(t, "[") {
		return nil, fmt.Errorf("vault does not parse as entries")
	}
	var entries []Entry
	if err := json.Unmarshal([]byte(plain), &entries); err != nil {
		return nil, fmt.Errorf("vault does not parse as entries")
	}
	return entries, nil
}

// reload re-reads the vault files on disk with the current password.
func (v *Vault) reload() error {
	return v.Unlock(v.password)
}
