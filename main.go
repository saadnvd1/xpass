package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/saadnvd1/xpass/internal/clipboard"
	"github.com/saadnvd1/xpass/internal/crypto"
	"github.com/saadnvd1/xpass/internal/importer"
	"github.com/saadnvd1/xpass/internal/tui"
	"github.com/saadnvd1/xpass/internal/vault"
)

func main() {
	v := vault.New(vault.DefaultDir())

	// No args or "tui" — launch TUI
	if len(os.Args) < 2 || os.Args[1] == "tui" {
		runTUI(v)
		return
	}

	// CLI subcommands for scripting / quick access
	switch os.Args[1] {
	case "init":
		cmdInit(v)
	case "get":
		cmdGet(v)
	case "add":
		cmdAdd(v)
	case "list", "ls":
		cmdList(v)
	case "import":
		cmdImport(v)
	case "generate", "gen":
		cmdGenerate()
	case "version":
		fmt.Println("xpass v0.1.0")
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runTUI(v *vault.Vault) {
	m := tui.NewModel(v)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdInit(v *vault.Vault) {
	if v.Exists() {
		fmt.Println("Vault already exists at", v.Dir())
		return
	}

	fmt.Print("Master password: ")
	pw := readPassword()
	fmt.Print("\nConfirm password: ")
	pw2 := readPassword()
	fmt.Println()

	if pw != pw2 {
		fmt.Fprintln(os.Stderr, "Passwords don't match")
		os.Exit(1)
	}

	if err := v.Init(pw); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Println("Vault created at", v.Dir())
}

func cmdGet(v *vault.Vault) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: xpass get <name> [--copy]")
		os.Exit(1)
	}

	name := os.Args[2]
	copy := len(os.Args) > 3 && (os.Args[3] == "--copy" || os.Args[3] == "-c")

	pw := requireUnlock(v)
	_ = pw

	entry := v.GetByName(name)
	if entry == nil {
		// Try search
		results := v.Search(name)
		if len(results) == 0 {
			fmt.Fprintln(os.Stderr, "Not found:", name)
			os.Exit(1)
		}
		entry = &results[0]
	}

	secret := entry.Password
	if secret == "" {
		secret = entry.APIKey
	}
	if secret == "" {
		secret = entry.Content
	}

	if copy && secret != "" {
		clipboard.CopyWithClear(secret, 30*time.Second)
		fmt.Printf("Copied %s to clipboard (clears in 30s)\n", entry.Name)
	} else {
		fmt.Println(secret)
	}

	v.TrackAccess(entry.ID)
}

func cmdAdd(v *vault.Vault) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: xpass add <name> [--password <pw>] [--username <user>] [--url <url>]")
		os.Exit(1)
	}

	requireUnlock(v)

	entry := vault.Entry{
		Type: vault.TypeLogin,
		Name: os.Args[2],
	}

	// Parse flags
	for i := 3; i < len(os.Args)-1; i += 2 {
		switch os.Args[i] {
		case "--password", "-p":
			entry.Password = os.Args[i+1]
		case "--username", "-u":
			entry.Username = os.Args[i+1]
		case "--url":
			entry.URL = os.Args[i+1]
		case "--email", "-e":
			entry.Email = os.Args[i+1]
		case "--type", "-t":
			entry.Type = vault.EntryType(os.Args[i+1])
		}
	}

	added, err := v.Add(entry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Printf("Added %s (%s)\n", added.Name, added.ID[:8])
}

func cmdList(v *vault.Vault) {
	requireUnlock(v)

	entries := v.Entries()
	if len(entries) == 0 {
		fmt.Println("No entries")
		return
	}

	for _, e := range entries {
		star := " "
		if e.Favorite {
			star = "*"
		}
		fmt.Printf("%s [%s] %s  %s\n", star, strings.ToUpper(string(e.Type))[:3], e.Name, e.Subtitle())
	}
}

func cmdImport(v *vault.Vault) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: xpass import <file.csv|file.json>")
		fmt.Fprintln(os.Stderr, "\nSupported formats:")
		fmt.Fprintln(os.Stderr, "  - 1Password CSV export")
		fmt.Fprintln(os.Stderr, "  - 1Password JSON export")
		fmt.Fprintln(os.Stderr, "\nTo export from 1Password:")
		fmt.Fprintln(os.Stderr, "  1. Open 1Password desktop app")
		fmt.Fprintln(os.Stderr, "  2. File > Export > select vault")
		fmt.Fprintln(os.Stderr, "  3. Choose CSV or JSON format")
		os.Exit(1)
	}

	filePath := os.Args[2]

	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "File not found:", filePath)
		os.Exit(1)
	}

	// Parse the export file first (before unlocking, so user knows if file is valid)
	result, err := importer.Import(filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing file:", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d entries (%d parseable, %d skipped)\n", result.Total, result.Imported, result.Skipped)
	if len(result.Errors) > 0 {
		fmt.Println("\nWarnings:")
		for _, e := range result.Errors {
			fmt.Println("  -", e)
		}
	}

	if result.Imported == 0 {
		fmt.Println("Nothing to import.")
		return
	}

	fmt.Printf("\nImport %d entries? [y/N] ", result.Imported)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return
	}

	// Now unlock
	requireUnlock(v)

	// Import entries with progress
	imported := 0
	total := len(result.Entries)
	barWidth := 30

	for i, entry := range result.Entries {
		_, err := v.Add(entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  Error importing %s: %v\n", entry.Name, err)
			continue
		}
		imported++

		// Progress bar
		pct := float64(i+1) / float64(total)
		filled := int(pct * float64(barWidth))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		fmt.Fprintf(os.Stderr, "\r  [%s] %d/%d  %s", bar, i+1, total, entry.Name)
		// Clear rest of line in case previous name was longer
		fmt.Fprintf(os.Stderr, "\033[K")
	}
	fmt.Fprintf(os.Stderr, "\r\033[K") // clear progress line

	fmt.Printf("\nImported %d/%d entries into vault.\n", imported, result.Imported)

	// Security reminder
	fmt.Println("\nDon't forget to delete the export file:")
	fmt.Printf("  rm %s\n", filePath)
}

func cmdGenerate() {
	length := 20
	pw, err := crypto.GeneratePassword(length, true, true, true, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	copy := len(os.Args) > 2 && (os.Args[2] == "--copy" || os.Args[2] == "-c")
	if copy {
		clipboard.CopyWithClear(pw, 30*time.Second)
		fmt.Println("Generated and copied to clipboard (clears in 30s)")
	} else {
		fmt.Println(pw)
	}
}

func requireUnlock(v *vault.Vault) string {
	if !v.Exists() {
		fmt.Fprintln(os.Stderr, "No vault found. Run 'xpass init' first.")
		os.Exit(1)
	}

	fmt.Print("Master password: ")
	pw := readPassword()
	fmt.Println()

	if err := v.Unlock(pw); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	return pw
}

func readPassword() string {
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		// Fallback to plain read
		var s string
		fmt.Scanln(&s)
		return s
	}
	return string(pw)
}

func printUsage() {
	fmt.Println(`xpass - Terminal password manager

Usage:
  xpass              Launch TUI
  xpass init         Create a new vault
  xpass get <name>   Get entry (--copy to clipboard)
  xpass add <name>   Add entry (--password, --username, --url)
  xpass list         List all entries
  xpass import <f>   Import from 1Password (CSV/JSON)
  xpass gen          Generate password (--copy to clipboard)
  xpass version      Show version
  xpass help         Show this help`)
}
