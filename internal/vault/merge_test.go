package vault

import (
	"reflect"
	"testing"
)

func e(id, name string, version int, updated, password string) Entry {
	return Entry{ID: id, Name: name, Type: TypeLogin, Version: version, UpdatedAt: updated, Password: password, Tags: []string{}}
}

const (
	t0 = "2026-09-01T00:00:00Z"
	t1 = "2026-09-02T00:00:00Z"
	t2 = "2026-09-03T00:00:00Z"
)

func ids(entries []Entry) []string {
	out := []string{}
	for _, x := range entries {
		out = append(out, x.ID)
	}
	return out
}

func byID(entries []Entry, id string) *Entry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}

func TestMergeUnchangedTakesOtherSide(t *testing.T) {
	a := e("a", "A", 1, t0, "p0")
	b := e("b", "B", 1, t0, "p0")
	base := []Entry{a, b}
	local := []Entry{a, e("b", "B", 2, t1, "local")}
	remote := []Entry{e("a", "A", 2, t1, "remote"), b}

	got, rep := Merge(base, local, remote)
	if byID(got, "a").Password != "remote" || byID(got, "b").Password != "local" {
		t.Fatalf("wrong sides taken: %+v", got)
	}
	if !reflect.DeepEqual(rep.FromRemoteUpdated, []string{"A"}) || len(rep.Conflicts) != 0 {
		t.Fatalf("report: %+v", rep)
	}
}

func TestMergeBothChangedHigherVersionWins(t *testing.T) {
	base := []Entry{e("a", "A", 1, t0, "p0")}
	local := []Entry{e("a", "A", 2, t2, "local")}
	remote := []Entry{e("a", "A", 3, t1, "remote")}

	got, rep := Merge(base, local, remote)
	if got[0].Password != "remote" {
		t.Fatalf("higher version should win: %+v", got)
	}
	if len(rep.Conflicts) != 1 || rep.Conflicts[0].Name != "A" {
		t.Fatalf("conflict not reported: %+v", rep)
	}

	// and the other way round
	got, _ = Merge(base, []Entry{e("a", "A", 4, t0, "local")}, remote)
	if got[0].Password != "local" {
		t.Fatalf("local higher version should win: %+v", got)
	}
}

func TestMergeBothChangedTieLaterUpdatedAtWins(t *testing.T) {
	base := []Entry{e("a", "A", 1, t0, "p0")}
	local := []Entry{e("a", "A", 2, t1, "local")}
	remote := []Entry{e("a", "A", 2, t2, "remote")}

	got, rep := Merge(base, local, remote)
	if got[0].Password != "remote" || len(rep.Conflicts) != 1 {
		t.Fatalf("later UpdatedAt should win: %+v %+v", got, rep)
	}

	got, _ = Merge(base, []Entry{e("a", "A", 2, t2, "local")}, []Entry{e("a", "A", 2, t1, "remote")})
	if got[0].Password != "local" {
		t.Fatalf("later local UpdatedAt should win: %+v", got)
	}

	// full tie → local
	got, _ = Merge(base, []Entry{e("a", "A", 2, t1, "local")}, []Entry{e("a", "A", 2, t1, "remote")})
	if got[0].Password != "local" {
		t.Fatalf("full tie should keep local: %+v", got)
	}
}

func TestMergeUpdatedAtComparesInstantsNotStrings(t *testing.T) {
	base := []Entry{e("a", "A", 1, t0, "p0")}
	// 10:00+05:00 is 05:00Z, earlier than 06:00Z, though it sorts later as text.
	local := []Entry{e("a", "A", 2, "2026-09-02T06:00:00Z", "local")}
	remote := []Entry{e("a", "A", 2, "2026-09-02T10:00:00+05:00", "remote")}
	got, _ := Merge(base, local, remote)
	if got[0].Password != "local" {
		t.Fatalf("expected the later instant (local): %+v", got)
	}
}

func TestMergeSameEditBothSidesIsNotAConflict(t *testing.T) {
	base := []Entry{e("a", "A", 1, t0, "p0")}
	same := e("a", "A", 2, t1, "same")
	_, rep := Merge(base, []Entry{same}, []Entry{same})
	if len(rep.Conflicts) != 0 {
		t.Fatalf("identical edits reported as conflict: %+v", rep)
	}
}

func TestMergeDeletedUnchangedOtherSide(t *testing.T) {
	a := e("a", "A", 1, t0, "p0")
	b := e("b", "B", 1, t0, "p0")
	base := []Entry{a, b}

	// remote deleted a, local untouched
	got, rep := Merge(base, []Entry{a, b}, []Entry{b})
	if !reflect.DeepEqual(ids(got), []string{"b"}) || !reflect.DeepEqual(rep.Deleted, []string{"A"}) {
		t.Fatalf("remote delete: %v %+v", ids(got), rep)
	}

	// local deleted b, remote untouched
	got, rep = Merge(base, []Entry{a}, []Entry{a, b})
	if !reflect.DeepEqual(ids(got), []string{"a"}) || !reflect.DeepEqual(rep.Deleted, []string{"B"}) {
		t.Fatalf("local delete: %v %+v", ids(got), rep)
	}

	// deleted on both
	got, rep = Merge(base, []Entry{b}, []Entry{b})
	if !reflect.DeepEqual(ids(got), []string{"b"}) || !reflect.DeepEqual(rep.Deleted, []string{"A"}) {
		t.Fatalf("both deleted: %v %+v", ids(got), rep)
	}
}

func TestMergeDeletedButChangedOtherSideKeepsChange(t *testing.T) {
	a := e("a", "A", 1, t0, "p0")
	base := []Entry{a}

	// remote deleted, local edited
	got, rep := Merge(base, []Entry{e("a", "A", 2, t1, "local")}, []Entry{})
	if len(got) != 1 || got[0].Password != "local" || len(rep.Conflicts) != 1 || rep.Conflicts[0].Name != "A" {
		t.Fatalf("remote delete vs local edit: %+v %+v", got, rep)
	}

	// local deleted, remote edited
	got, rep = Merge(base, []Entry{}, []Entry{e("a", "A", 2, t1, "remote")})
	if len(got) != 1 || got[0].Password != "remote" || len(rep.Conflicts) != 1 {
		t.Fatalf("local delete vs remote edit: %+v %+v", got, rep)
	}
}

func TestMergeNewOnEitherSideKeptInStableOrder(t *testing.T) {
	a := e("a", "A", 1, t0, "p0")
	b := e("b", "B", 1, t0, "p0")
	base := []Entry{a, b}
	local := []Entry{b, e("l1", "L1", 1, t1, "x"), a, e("l2", "L2", 1, t1, "x")}
	remote := []Entry{e("r1", "R1", 1, t1, "x"), a, b, e("r2", "R2", 1, t1, "x")}

	got, rep := Merge(base, local, remote)
	want := []string{"b", "l1", "a", "l2", "r1", "r2"}
	if !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("order: got %v want %v", ids(got), want)
	}
	if !reflect.DeepEqual(rep.FromRemoteAdded, []string{"R1", "R2"}) || !reflect.DeepEqual(rep.LocalAdded, []string{"L1", "L2"}) {
		t.Fatalf("report: %+v", rep)
	}
}

func TestMergeSameNewIDBothSides(t *testing.T) {
	got, rep := Merge(nil, []Entry{e("n", "N", 1, t1, "local")}, []Entry{e("n", "N", 1, t2, "remote")})
	if len(got) != 1 || got[0].Password != "remote" || len(rep.Conflicts) != 1 {
		t.Fatalf("same new id: %+v %+v", got, rep)
	}
}

func TestMergeIgnoresAccessMetadata(t *testing.T) {
	a := e("a", "A", 1, t0, "p0")
	base := []Entry{a}
	read := a
	read.LastAccessed = t2
	read.AccessCount = 5

	// A local read is not an edit: the remote edit is taken, no conflict.
	got, rep := Merge(base, []Entry{read}, []Entry{e("a", "A", 2, t1, "remote")})
	if got[0].Password != "remote" || len(rep.Conflicts) != 0 {
		t.Fatalf("access metadata treated as an edit: %+v %+v", got, rep)
	}

	// Nothing changed remotely: local (with its access count) is kept.
	got, _ = Merge(base, []Entry{read}, []Entry{a})
	if got[0].AccessCount != 5 {
		t.Fatalf("local access metadata lost: %+v", got)
	}

	// A remote delete of an entry that was only read locally is a delete.
	got, rep = Merge(base, []Entry{read}, []Entry{})
	if len(got) != 0 || len(rep.Conflicts) != 0 {
		t.Fatalf("read-only entry should be deleted: %+v %+v", got, rep)
	}
}

func TestMergeNilAndEmptyTagsAreTheSame(t *testing.T) {
	a := e("a", "A", 1, t0, "p0")
	nilTags := a
	nilTags.Tags = nil
	got, rep := Merge([]Entry{a}, []Entry{nilTags}, []Entry{e("a", "A", 2, t1, "remote")})
	if got[0].Password != "remote" || len(rep.Conflicts) != 0 {
		t.Fatalf("tags null vs [] treated as an edit: %+v %+v", got, rep)
	}
}
