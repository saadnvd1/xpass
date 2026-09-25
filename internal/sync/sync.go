package sync

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Sync manages git-based vault synchronization
type Sync struct {
	dir string
}

// New creates a sync manager for the given vault directory
func New(dir string) *Sync {
	return &Sync{dir: dir}
}

// Init initializes a git repo in the vault directory
func (s *Sync) Init() error {
	if s.isGitRepo() {
		return nil
	}

	if err := s.git("init"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	// Ensure .gitignore exists
	gitignore := ".session\n*.bak\n*.tmp\n*.swp\n.DS_Store\n"
	os.WriteFile(filepath.Join(s.dir, ".gitignore"), []byte(gitignore), 0600)

	// Initial commit
	if err := s.git("add", "."); err != nil {
		return err
	}
	return s.git("commit", "-m", "init xpass vault")
}

// SetRemote sets the git remote URL
func (s *Sync) SetRemote(url string) error {
	if !s.isGitRepo() {
		if err := s.Init(); err != nil {
			return err
		}
	}

	// Check if origin exists
	out, err := s.gitOutput("remote")
	if err == nil && strings.Contains(out, "origin") {
		return s.git("remote", "set-url", "origin", url)
	}
	return s.git("remote", "add", "origin", url)
}

// GetRemote returns the current remote URL, or empty string
func (s *Sync) GetRemote() string {
	out, err := s.gitOutput("remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// AutoCommit stages and commits encrypted vault files
func (s *Sync) AutoCommit() error {
	if !s.isGitRepo() {
		return nil
	}

	// Check if there are changes
	out, err := s.gitOutput("status", "--porcelain")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil // nothing to commit
	}

	// Only files that exist: git add fails on the whole list if one pathspec
	// matches nothing (a vault without history.json never committed again).
	args := []string{"add", "--"}
	for _, f := range []string{"vault.json", "config.json", "history.json", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(s.dir, f)); err == nil {
			args = append(args, f)
		}
	}
	if err := s.git(args...); err != nil {
		return err
	}

	msg := fmt.Sprintf("xpass sync %s", time.Now().UTC().Format("2006-01-02 15:04:05"))
	return s.git("commit", "-m", msg)
}

// ErrRemoteAhead means the remote has commits this vault has not merged.
var ErrRemoteAhead = errors.New("the remote has changes this vault doesn't have yet — run 'xpass pull' (it merges them), then 'xpass push'")

// Push pushes to remote. It refuses, with ErrRemoteAhead, when the remote
// has commits this vault has not merged: merging needs the master password,
// so that is 'xpass pull', not something push does on its own.
func (s *Sync) Push() error {
	if err := s.requireRemote(); err != nil {
		return err
	}

	// Commit any pending changes
	s.AutoCommit()

	branch := s.Branch()

	if err := s.Fetch(); err != nil {
		return err
	}
	if s.HasRemoteBranch() {
		if _, behind, err := s.AheadBehind(); err == nil && behind > 0 {
			return ErrRemoteAhead
		}
	}

	if err := s.git("push", "-u", "origin", branch); err != nil {
		// Someone pushed between the fetch and the push.
		msg := err.Error()
		if strings.Contains(msg, "rejected") || strings.Contains(msg, "fetch first") || strings.Contains(msg, "non-fast-forward") {
			return ErrRemoteAhead
		}
		return fmt.Errorf("push failed: %w", err)
	}

	return nil
}

// PrepareForPull checks a remote is set, commits pending changes and fetches.
// The merge itself is the vault's (it needs the password): see vault.Pull.
func (s *Sync) PrepareForPull() error {
	if err := s.requireRemote(); err != nil {
		return err
	}
	s.AutoCommit()
	return s.Fetch()
}

// Fetch fetches the current branch from origin.
func (s *Sync) Fetch() error {
	if err := s.git("fetch", "origin"); err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}
	return nil
}

// RemoteRef is the remote-tracking ref for the current branch.
func (s *Sync) RemoteRef() string {
	return "origin/" + s.Branch()
}

// HasRemoteBranch reports whether origin has the current branch.
func (s *Sync) HasRemoteBranch() bool {
	return s.git("rev-parse", "--verify", "--quiet", s.RemoteRef()+"^{commit}") == nil
}

// AheadBehind counts commits HEAD has that the remote branch lacks, and the reverse.
func (s *Sync) AheadBehind() (ahead, behind int, err error) {
	out, err := s.gitOutput("rev-list", "--left-right", "--count", "HEAD..."+s.RemoteRef())
	if err != nil {
		return 0, 0, fmt.Errorf("comparing with remote: %w", err)
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(out), "%d %d", &ahead, &behind); err != nil {
		return 0, 0, fmt.Errorf("comparing with remote: %q", out)
	}
	return ahead, behind, nil
}

// Head returns the commit HEAD points at.
func (s *Sync) Head() (string, error) {
	out, err := s.gitOutput("rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

// MergeBase returns the common ancestor of HEAD and the remote branch.
func (s *Sync) MergeBase() (string, error) {
	out, err := s.gitOutput("merge-base", "HEAD", s.RemoteRef())
	if err != nil {
		return "", fmt.Errorf("the local and remote vaults share no history")
	}
	return strings.TrimSpace(out), nil
}

// ShowFile returns a file's contents at a revision.
func (s *Sync) ShowFile(rev, path string) ([]byte, error) {
	cmd := exec.Command("git", "show", rev+":"+path)
	cmd.Dir = s.dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("reading %s at %s: %w", path, rev, err)
	}
	return out, nil
}

// FastForward moves HEAD (and the working tree) to the remote branch.
func (s *Sync) FastForward() error {
	return s.git("merge", "--ff-only", s.RemoteRef())
}

// MergeOurs records a merge with the remote branch that keeps this side's
// files; the caller then writes the real merged files and calls Amend.
func (s *Sync) MergeOurs() error {
	return s.git("merge", "-s", "ours", "--no-edit", s.RemoteRef())
}

// Amend stages the given files and folds them into the last commit.
func (s *Sync) Amend(files ...string) error {
	if err := s.git(append([]string{"add"}, files...)...); err != nil {
		return err
	}
	return s.git("commit", "--amend", "--no-edit")
}

// ResetHard puts HEAD and the working tree back at rev.
func (s *Sync) ResetHard(rev string) error {
	return s.git("reset", "--hard", rev)
}

func (s *Sync) requireRemote() error {
	if !s.isGitRepo() {
		return fmt.Errorf("not a git repo — run 'xpass remote <url>' first")
	}
	if s.GetRemote() == "" {
		return fmt.Errorf("no remote configured — run 'xpass remote <url>' first")
	}
	return nil
}

// Status returns sync status info
func (s *Sync) Status() string {
	if !s.isGitRepo() {
		return "not initialized"
	}

	remote := s.GetRemote()
	if remote == "" {
		return "no remote"
	}

	// Check ahead/behind
	s.git("fetch", "origin")

	out, err := s.gitOutput("status", "--short", "--branch")
	if err != nil {
		return "remote: " + remote
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	status := "synced"
	if len(lines) > 0 {
		branch := lines[0]
		if strings.Contains(branch, "ahead") {
			status = "ahead (needs push)"
		}
		if strings.Contains(branch, "behind") {
			status = "behind (needs pull)"
		}
		if strings.Contains(branch, "ahead") && strings.Contains(branch, "behind") {
			status = "diverged (pull then push)"
		}
	}

	// Check for uncommitted changes
	dirty, _ := s.gitOutput("status", "--porcelain")
	if strings.TrimSpace(dirty) != "" {
		status += " + uncommitted changes"
	}

	return fmt.Sprintf("%s | %s", remote, status)
}

// Branch returns the current branch name (main if it cannot tell).
func (s *Sync) Branch() string {
	out, err := s.gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "main"
	}
	b := strings.TrimSpace(out)
	if b == "" {
		return "main"
	}
	return b
}

func (s *Sync) isGitRepo() bool {
	_, err := os.Stat(filepath.Join(s.dir, ".git"))
	return err == nil
}

func (s *Sync) git(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Sync) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.dir
	out, err := cmd.Output()
	return string(out), err
}
