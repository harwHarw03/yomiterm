package clipboard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ReadText reads text from the system or tmux clipboard, supporting WSL2 on Windows 11,
// Wayland (wl-paste), X11 (xclip/xsel), macOS (pbpaste), and tmux paste buffers.
func ReadText() (string, error) {
	type cmdCandidate struct {
		name string
		args []string
	}

	var candidates []cmdCandidate

	// If inside tmux, check tmux paste buffer if available, or fall through to OS clipboard
	isWSL := os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != ""

	if isWSL {
		candidates = append(candidates,
			cmdCandidate{"win32yank.exe", []string{"-o", "--lf"}},
			cmdCandidate{"powershell.exe", []string{"-NoProfile", "-Command", "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; Get-Clipboard -Raw"}},
		)
	}

	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates, cmdCandidate{"pbpaste", nil})
	default:
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			candidates = append(candidates, cmdCandidate{"wl-paste", []string{"-n"}})
		}
		if os.Getenv("DISPLAY") != "" {
			candidates = append(candidates,
				cmdCandidate{"xclip", []string{"-o", "-selection", "clipboard"}},
				cmdCandidate{"xsel", []string{"-ob"}},
			)
		}
	}

	if os.Getenv("TMUX") != "" {
		candidates = append(candidates, cmdCandidate{"tmux", []string{"show-buffer"}})
	}

	for _, c := range candidates {
		path, err := exec.LookPath(c.name)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, c.args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			txt := strings.TrimSpace(strings.ReplaceAll(out.String(), "\r\n", "\n"))
			if txt != "" {
				return txt, nil
			}
		}
	}

	return "", fmt.Errorf("no clipboard utility returned text (tried WSL/Wayland/X11/macOS/tmux)")
}
