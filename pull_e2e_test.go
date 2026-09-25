package main

// End-to-end test of `xpass pull` merging with a second clone that writes
// vault.json the way the phone page does. Everything lives under t.TempDir():
// HOME, the git config, the bare "remote" and both clones. It never touches
// the real ~/.xpass or the real remote.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/saadnvd1/xpass/internal/crypto"
	"github.com/saadnvd1/xpass/internal/vault"
)

const testPW = "throwaway-test-pw"

type e2e struct {
	t     *testing.T
	root  string
	bin   string
	home  string
	vault string // $HOME/.xpass
	phone string // second clone, written like the phone page
}

func setup(t *testing.T) *e2e {
	if testing.Short() {
		t.Skip("e2e: builds the binary and runs PBKDF2 many times")
	}
	root := t.TempDir()

	// Build with the real environment (module cache), before HOME moves.
	bin := filepath.Join(root, "xpass")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	home := filepath.Join(root, "home")
	os.MkdirAll(home, 0700)
	gitcfg := filepath.Join(root, "gitconfig")
	os.WriteFile(gitcfg, []byte("[user]\n\tname = test\n\temail = test@example.invalid\n[init]\n\tdefaultBranch = main\n[commit]\n\tgpgsign = false\n"), 0600)

	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", gitcfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	if h, _ := os.UserHomeDir(); !strings.HasPrefix(h, root) {
		t.Fatalf("HOME is not the temp dir (%s); refusing to run", h)
	}

	return &e2e{t: t, root: root, bin: bin, home: home, vault: filepath.Join(home, ".xpass"), phone: filepath.Join(root, "phone")}
}

func (x *e2e) xpass(stdin string, args ...string) (string, error) {
	cmd := exec.Command(x.bin, args...)
	cmd.Dir = x.root
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (x *e2e) mustXpass(stdin string, args ...string) string {
	out, err := x.xpass(stdin, args...)
	if err != nil {
		x.t.Fatalf("xpass %v: %v\n%s", args, err, out)
	}
	return out
}

func (x *e2e) git(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		x.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// phoneEntries decrypts the phone clone's vault.json.
func (x *e2e) phoneEntries() ([]vault.Entry, crypto.EncryptedData) {
	raw, err := os.ReadFile(filepath.Join(x.phone, "vault.json"))
	if err != nil {
		x.t.Fatal(err)
	}
	var enc crypto.EncryptedData
	json.Unmarshal(raw, &enc)
	plain, err := crypto.Decrypt(&enc, testPW)
	if err != nil {
		x.t.Fatalf("phone decrypt: %v", err)
	}
	var entries []vault.Entry
	if err := json.Unmarshal([]byte(plain), &entries); err != nil {
		x.t.Fatal(err)
	}
	return entries, enc
}

// phoneWrite re-encrypts entries the way the phone page does (the vault's
// existing salt, a fresh 12-byte nonce padded to a 16-byte IV), commits
// vault.json alone and pushes.
func (x *e2e) phoneWrite(entries []vault.Entry, saltHex, password, msg string) {
	plain, _ := json.Marshal(entries)
	salt, _ := hex.DecodeString(saltHex)
	block, _ := aes.NewCipher(crypto.DeriveKey(password, salt))
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, crypto.NonceLength)
	rand.Read(nonce)
	sealed := gcm.Seal(nil, nonce, plain, nil)
	iv := make([]byte, crypto.IVLength)
	copy(iv, nonce)
	enc := crypto.EncryptedData{
		Salt:    saltHex,
		IV:      hex.EncodeToString(iv),
		Data:    hex.EncodeToString(sealed[:len(sealed)-crypto.AuthTagLength]),
		AuthTag: hex.EncodeToString(sealed[len(sealed)-crypto.AuthTagLength:]),
		Version: "1.0",
	}
	data, _ := json.MarshalIndent(enc, "", "  ")
	os.WriteFile(filepath.Join(x.phone, "vault.json"), data, 0600)
	x.git(x.phone, "add", "vault.json")
	x.git(x.phone, "commit", "-m", msg)
	x.git(x.phone, "push", "origin", "main")
}

func (x *e2e) macVault() *vault.Vault {
	v := vault.New(x.vault)
	if err := v.Unlock(testPW); err != nil {
		x.t.Fatalf("unlock mac vault: %v", err)
	}
	return v
}

func find(entries []vault.Entry, name string) *vault.Entry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func names(entries []vault.Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

func TestPullMergesPhoneEdits(t *testing.T) {
	x := setup(t)
	pw := testPW + "\n"
	remote := filepath.Join(x.root, "remote.git")
	x.git(x.root, "init", "--bare", remote)

	x.mustXpass(pw+pw, "init")
	x.mustXpass("", "remote", remote)
	for _, n := range []string{"Alpha", "Beta", "Gamma", "Delta"} {
		x.mustXpass(pw, "add", n, "--password", "secret-"+strings.ToLower(n)+"-v1")
	}
	x.mustXpass("", "push")

	// --- Phone: edit Alpha and Delta, delete Gamma, add "Phone New".
	x.git(x.root, "clone", remote, x.phone)
	entries, enc := x.phoneEntries()
	later := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	var phone []vault.Entry
	for _, e := range entries {
		switch e.Name {
		case "Alpha":
			e.Password, e.Version, e.UpdatedAt = "secret-alpha-phone", e.Version+1, later
		case "Delta":
			e.Password, e.Version, e.UpdatedAt = "secret-delta-phone", e.Version+2, later
		case "Gamma":
			continue
		}
		phone = append(phone, e)
	}
	newID, _ := crypto.GenerateID()
	phone = append(phone, vault.Entry{ID: newID, Type: vault.TypeLogin, Name: "Phone New", Password: "secret-phonenew", Tags: []string{}, Version: 1, CreatedAt: later, UpdatedAt: later})
	x.phoneWrite(phone, enc.Salt, testPW, "phone edit")

	// --- Mac: add Epsilon via the CLI, edit Beta and Delta.
	x.mustXpass(pw, "add", "Epsilon", "--password", "secret-epsilon-v1")
	mac := x.macVault()
	for _, n := range []string{"Beta", "Delta"} {
		upd := *mac.GetByName(n)
		upd.Password = "secret-" + strings.ToLower(n) + "-mac"
		if _, err := mac.Update(upd.ID, upd); err != nil {
			t.Fatal(err)
		}
	}

	// Push while the remote is ahead: refused, pointing at pull.
	out, err := x.xpass("", "push")
	if err == nil || !strings.Contains(out, "xpass pull") {
		t.Fatalf("push with remote ahead should refuse and say run pull; got err=%v\n%s", err, out)
	}

	// Pull merges.
	out = x.mustXpass(pw, "pull")
	t.Logf("pull output (throwaway vault):\n%s", out)
	if strings.Contains(out, "secret-") {
		t.Fatalf("pull output contains a secret value:\n%s", out)
	}
	for _, want := range []string{"Merged", "Phone New", "Alpha", "Gamma", "Delta", "conflicts (1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("pull output missing %q:\n%s", want, out)
		}
	}

	got := x.macVault().Entries()
	wantOrder := []string{"Alpha", "Beta", "Delta", "Epsilon", "Phone New"}
	if strings.Join(names(got), ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("merged entries: %v, want %v", names(got), wantOrder)
	}
	checks := map[string]string{
		"Alpha":     "secret-alpha-phone", // phone only
		"Beta":      "secret-beta-mac",    // mac only
		"Delta":     "secret-delta-phone", // both; phone has the higher version
		"Epsilon":   "secret-epsilon-v1",
		"Phone New": "secret-phonenew",
	}
	for n, pwv := range checks {
		if e := find(got, n); e == nil || e.Password != pwv {
			t.Errorf("%s: wrong value after merge", n)
		}
	}

	// A real merge commit, with a clean tree.
	if parents := strings.Fields(x.git(x.vault, "rev-list", "--parents", "-n", "1", "HEAD")); len(parents) != 3 {
		t.Fatalf("HEAD is not a merge commit: %v", parents)
	}
	if st := x.git(x.vault, "status", "--porcelain"); st != "" {
		t.Fatalf("tree not clean after merge:\n%s", st)
	}

	x.mustXpass("", "push")
	x.git(x.phone, "pull", "--ff-only", "origin", "main")
	phoneNow, _ := x.phoneEntries()
	if strings.Join(names(phoneNow), ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("phone after pull: %v", names(phoneNow))
	}

	// --- Fast-forward: phone edits again, Mac has nothing new.
	phoneNow, enc = x.phoneEntries()
	find(phoneNow, "Epsilon").Password = "secret-epsilon-phone"
	find(phoneNow, "Epsilon").Version++
	x.phoneWrite(phoneNow, enc.Salt, testPW, "phone edit 2")
	out = x.mustXpass(pw, "pull")
	if !strings.Contains(out, "Fast-forwarded") {
		t.Fatalf("expected fast-forward:\n%s", out)
	}
	if find(x.macVault().Entries(), "Epsilon").Password != "secret-epsilon-phone" {
		t.Fatal("fast-forward did not bring the phone edit")
	}

	// Nothing new: up to date.
	if out = x.mustXpass(pw, "pull"); !strings.Contains(out, "up to date") {
		t.Fatalf("expected up to date:\n%s", out)
	}
}

func TestPullRefusesRemoteThatDoesNotDecrypt(t *testing.T) {
	x := setup(t)
	pw := testPW + "\n"
	remote := filepath.Join(x.root, "remote.git")
	x.git(x.root, "init", "--bare", remote)

	x.mustXpass(pw+pw, "init")
	x.mustXpass("", "remote", remote)
	x.mustXpass(pw, "add", "Alpha", "--password", "secret-alpha")
	x.mustXpass("", "push")

	x.git(x.root, "clone", remote, x.phone)
	entries, enc := x.phoneEntries()

	refused := func(label string) {
		t.Helper()
		before := x.git(x.vault, "rev-parse", "HEAD")
		out, err := x.xpass(pw, "pull")
		if err == nil || !strings.Contains(out, "does not open with your password") {
			t.Fatalf("%s: pull should refuse; err=%v\n%s", label, err, out)
		}
		if after := x.git(x.vault, "rev-parse", "HEAD"); after != before {
			t.Fatalf("%s: HEAD moved from %s to %s", label, before, after)
		}
		if st := x.git(x.vault, "status", "--porcelain"); st != "" {
			t.Fatalf("%s: tree dirty after refusal:\n%s", label, st)
		}
		if find(x.macVault().Entries(), "Alpha") == nil {
			t.Fatalf("%s: local vault damaged", label)
		}
	}

	// Encrypted with another password (behind only → would be a fast-forward).
	x.phoneWrite(entries, enc.Salt, "some-other-password", "wrong password")
	refused("other password, fast-forward")

	// Local change too (diverged → would be a merge).
	x.mustXpass(pw, "add", "Beta", "--password", "secret-beta")
	refused("other password, diverged")

	// Garbage that is not an envelope at all.
	os.WriteFile(filepath.Join(x.phone, "vault.json"), []byte("not json"), 0600)
	x.git(x.phone, "commit", "-am", "garbage")
	x.git(x.phone, "push", "origin", "main")
	refused("garbage")

	// Right password but not a list of entries.
	plainObj := `{"not":"entries"}`
	salt, _ := hex.DecodeString(enc.Salt)
	block, _ := aes.NewCipher(crypto.DeriveKey(testPW, salt))
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, crypto.NonceLength)
	rand.Read(nonce)
	sealed := gcm.Seal(nil, nonce, []byte(plainObj), nil)
	iv := make([]byte, crypto.IVLength)
	copy(iv, nonce)
	data, _ := json.Marshal(crypto.EncryptedData{Salt: enc.Salt, IV: hex.EncodeToString(iv), Data: hex.EncodeToString(sealed[:len(sealed)-16]), AuthTag: hex.EncodeToString(sealed[len(sealed)-16:]), Version: "1.0"})
	os.WriteFile(filepath.Join(x.phone, "vault.json"), data, 0600)
	x.git(x.phone, "commit", "-am", "not entries")
	x.git(x.phone, "push", "origin", "main")
	refused("not entries")

	// Right password, JSON null: not a vault either.
	x.phoneWrite(nil, enc.Salt, testPW, "null")
	refused("null")
}

// A replayed old vault decrypts perfectly — it is a genuine old copy — and
// would bring back rotated passwords. Pull must refuse it.
func TestPullRefusesAReplayedOldVault(t *testing.T) {
	x := setup(t)
	pw := testPW + "\n"
	remote := filepath.Join(x.root, "remote.git")
	x.git(x.root, "init", "--bare", remote)

	x.mustXpass(pw+pw, "init")
	x.mustXpass("", "remote", remote)
	x.mustXpass(pw, "add", "Alpha", "--password", "old-password")
	x.mustXpass("", "push")
	old, err := os.ReadFile(filepath.Join(x.vault, "vault.json"))
	if err != nil {
		t.Fatal(err)
	}

	// The password is rotated on the phone and the Mac takes it.
	x.git(x.root, "clone", remote, x.phone)
	entries, enc := x.phoneEntries()
	entries[0].Password = "new-password"
	entries[0].Version++
	x.phoneWrite(entries, enc.Salt, testPW, "rotate")
	x.mustXpass(pw, "pull")

	// Someone with push access brings the old ciphertext back, byte for byte.
	x.git(x.phone, "pull", "--ff-only")
	os.WriteFile(filepath.Join(x.phone, "vault.json"), old, 0600)
	x.git(x.phone, "commit", "-am", "replay")
	x.git(x.phone, "push", "origin", "main")

	before := x.git(x.vault, "rev-parse", "HEAD")
	out, err := x.xpass(pw, "pull")
	if err == nil || !strings.Contains(out, "OLDER copy") {
		t.Fatalf("pull should refuse the replay; err=%v\n%s", err, out)
	}
	if after := x.git(x.vault, "rev-parse", "HEAD"); after != before {
		t.Fatalf("HEAD moved from %s to %s", before, after)
	}
	if strings.Contains(out, "old-password") || strings.Contains(out, "new-password") {
		t.Fatal("a secret reached the output")
	}
}
