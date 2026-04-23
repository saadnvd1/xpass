package sync

import (
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

	if err := s.git("add", "vault.json", "config.json", ".gitignore"); err != nil {
		return err
	}

	msg := fmt.Sprintf("xpass sync %s", time.Now().UTC().Format("2006-01-02 15:04:05"))
	return s.git("commit", "-m", msg)
}

// Push pushes to remote
func (s *Sync) Push() error {
	if !s.isGitRepo() {
		return fmt.Errorf("not a git repo — run 'xpass remote <url>' first")
	}

	remote := s.GetRemote()
	if remote == "" {
		return fmt.Errorf("no remote configured — run 'xpass remote <url>' first")
	}

	// Commit any pending changes
	s.AutoCommit()

	// Get current branch
	branch := s.currentBranch()

	// Push
	if err := s.git("push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	return nil
}

// Pull pulls from remote and rebases local changes
func (s *Sync) Pull() error {
	if !s.isGitRepo() {
		return fmt.Errorf("not a git repo — run 'xpass remote <url>' first")
	}

	remote := s.GetRemote()
	if remote == "" {
		return fmt.Errorf("no remote configured — run 'xpass remote <url>' first")
	}

	// Commit any pending changes first
	s.AutoCommit()

	// Get current branch
	branch := s.currentBranch()

	// Pull with rebase
	if err := s.git("pull", "--rebase", "origin", branch); err != nil {
		return fmt.Errorf("pull failed: %w", err)
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

func (s *Sync) currentBranch() string {
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *Sync) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.dir
	out, err := cmd.Output()
	return string(out), err
}
