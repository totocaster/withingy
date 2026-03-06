package auth

import (
	"errors"
	"os/exec"
	"runtime"
)

// OpenBrowser tries to open the URL in the default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	if cmd == nil {
		return errors.New("unsupported platform")
	}
	return cmd.Start()
}
