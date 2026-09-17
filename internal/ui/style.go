package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	Muted = lipgloss.AdaptiveColor{
		Light: "#6B7280",
		Dark:  "#7C8798",
	}

	Command = lipgloss.AdaptiveColor{
		Light: "#0F766E",
		Dark:  "#67E8F9",
	}

	Failure = lipgloss.AdaptiveColor{
		Light: "#B91C1C",
		Dark:  "#FB7185",
	}

	Recovery = lipgloss.AdaptiveColor{
		Light: "#15803D",
		Dark:  "#4ADE80",
	}

	Note = lipgloss.AdaptiveColor{
		Light: "#A16207",
		Dark:  "#FACC15",
	}

	State = lipgloss.AdaptiveColor{
		Light: "#1D4ED8",
		Dark:  "#60A5FA",
	}

	Accent = lipgloss.AdaptiveColor{
		Light: "#334155",
		Dark:  "#CBD5E1",
	}
)

func ColorEnabled() bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}

	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return true
}

func Renderer() *lipgloss.Renderer {
	renderer := lipgloss.NewRenderer(os.Stdout)

	if !ColorEnabled() {
		renderer.SetColorProfile(0)
	}

	return renderer
}

func ShortPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == home {
		return "~"
	}

	prefix := home + string(filepath.Separator)

	if strings.HasPrefix(path, prefix) {
		return "~/" + strings.TrimPrefix(path, prefix)
	}

	return path
}
