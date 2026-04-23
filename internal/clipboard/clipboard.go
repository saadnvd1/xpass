package clipboard

import (
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Copy copies text to system clipboard
func Copy(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		cmd = exec.Command("pbcopy") // fallback
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// CopyWithClear copies to clipboard and clears after duration
func CopyWithClear(text string, clearAfter time.Duration) error {
	if err := Copy(text); err != nil {
		return err
	}

	go func() {
		time.Sleep(clearAfter)
		Clear()
	}()

	return nil
}

// Clear empties the clipboard
func Clear() error {
	return Copy("")
}
