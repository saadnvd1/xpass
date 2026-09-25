package vault

import (
	"encoding/json"
	"time"
)

// MergeConflict names an entry both sides touched in ways that could not both
// be kept. Only the entry's name and why are recorded, never a field value.
type MergeConflict struct {
	ID     string
	Name   string
	Reason string
}

// MergeReport says what a three-way merge did, by entry name only.
type MergeReport struct {
	FromRemoteAdded   []string // new on the remote side, kept
	FromRemoteUpdated []string // changed only on the remote side, taken
	LocalAdded        []string // new on the local side, kept
	Deleted           []string // removed on one side, unchanged on the other
	Conflicts         []MergeConflict
}

// Changed reports whether the merge result differs from the local side.
func (r MergeReport) Changed() bool {
	return len(r.FromRemoteAdded) > 0 || len(r.FromRemoteUpdated) > 0 ||
		len(r.Deleted) > 0 || len(r.Conflicts) > 0
}

// Merge merges two edited copies of a vault against their common ancestor,
// entry by entry, keyed by ID:
//
//   - unchanged on one side (vs base) → the other side is taken;
//   - changed on both → the higher Version wins, a tie goes to the later
//     UpdatedAt, a full tie to local; reported as a conflict;
//   - in base, missing on one side and unchanged on the other → deleted;
//     missing on one side but changed on the other → the changed one is kept
//     and reported;
//   - not in base → kept (if both sides added the same ID, as "changed on both").
//
// "Changed" ignores LastAccessed/AccessCount, which every read bumps without
// a Version change. Order: local's order, then remote-only entries in
// remote's order.
func Merge(base, local, remote []Entry) ([]Entry, MergeReport) {
	var rep MergeReport
	baseBy := index(base)
	localBy := index(local)
	remoteBy := index(remote)

	merged := make([]Entry, 0, len(local)+len(remote))

	for _, l := range local {
		b, inBase := baseBy[l.ID]
		r, inRemote := remoteBy[l.ID]

		switch {
		case !inBase && !inRemote:
			rep.LocalAdded = append(rep.LocalAdded, l.Name)
			merged = append(merged, l)

		case !inBase && inRemote: // both added the same ID
			if sameContent(l, r) {
				merged = append(merged, l)
				continue
			}
			w, fromRemote := winner(l, r)
			rep.Conflicts = append(rep.Conflicts, MergeConflict{l.ID, w.Name, "added on both sides with different contents; kept " + side(fromRemote)})
			merged = append(merged, w)

		case inBase && !inRemote: // deleted on remote
			if sameContent(l, b) {
				rep.Deleted = append(rep.Deleted, l.Name)
				continue
			}
			rep.Conflicts = append(rep.Conflicts, MergeConflict{l.ID, l.Name, "deleted on remote but edited here; kept the edit"})
			merged = append(merged, l)

		default: // in base and on both sides
			lChanged := !sameContent(l, b)
			rChanged := !sameContent(r, b)
			switch {
			case !rChanged:
				merged = append(merged, l)
			case !lChanged:
				rep.FromRemoteUpdated = append(rep.FromRemoteUpdated, r.Name)
				merged = append(merged, r)
			case sameContent(l, r):
				merged = append(merged, l)
			default:
				w, fromRemote := winner(l, r)
				rep.Conflicts = append(rep.Conflicts, MergeConflict{l.ID, w.Name, "edited on both sides; kept " + side(fromRemote)})
				merged = append(merged, w)
			}
		}
	}

	for _, r := range remote {
		if _, inLocal := localBy[r.ID]; inLocal {
			continue
		}
		b, inBase := baseBy[r.ID]
		switch {
		case !inBase:
			rep.FromRemoteAdded = append(rep.FromRemoteAdded, r.Name)
			merged = append(merged, r)
		case sameContent(r, b): // deleted here, untouched there
			rep.Deleted = append(rep.Deleted, r.Name)
		default:
			rep.Conflicts = append(rep.Conflicts, MergeConflict{r.ID, r.Name, "deleted here but edited on remote; kept the edit"})
			merged = append(merged, r)
		}
	}

	// Entries in base that neither side has any more were deleted on both.
	for _, b := range base {
		_, inL := localBy[b.ID]
		_, inR := remoteBy[b.ID]
		if !inL && !inR {
			rep.Deleted = append(rep.Deleted, b.Name)
		}
	}

	return merged, rep
}

func index(entries []Entry) map[string]Entry {
	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		m[e.ID] = e
	}
	return m
}

// sameContent compares two entries by their JSON encoding, ignoring the
// access metadata that a plain read updates and treating no tags as "tags": [].
func sameContent(a, b Entry) bool {
	a.LastAccessed, b.LastAccessed = "", ""
	a.AccessCount, b.AccessCount = 0, 0
	if a.Tags == nil {
		a.Tags = []string{}
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
}

// winner picks between two conflicting edits: higher Version, then later
// UpdatedAt, then local.
func winner(local, remote Entry) (Entry, bool) {
	if local.Version != remote.Version {
		if remote.Version > local.Version {
			return remote, true
		}
		return local, false
	}
	if later(remote.UpdatedAt, local.UpdatedAt) {
		return remote, true
	}
	return local, false
}

// later reports whether a is strictly after b. Unparseable timestamps fall
// back to string order (RFC3339 in UTC sorts lexically).
func later(a, b string) bool {
	ta, errA := time.Parse(time.RFC3339, a)
	tb, errB := time.Parse(time.RFC3339, b)
	if errA == nil && errB == nil {
		return ta.After(tb)
	}
	return a > b
}

func side(fromRemote bool) string {
	if fromRemote {
		return "the remote version"
	}
	return "the local version"
}
