package ui

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

// Palette is the DESIGN.md color set.
type Palette struct {
	Accent  lipgloss.AdaptiveColor
	Success lipgloss.AdaptiveColor
	Danger  lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Info    lipgloss.AdaptiveColor
	Muted   lipgloss.AdaptiveColor
	Select  lipgloss.AdaptiveColor
}

// Colors are the spec palette.
var Colors = Palette{
	Accent:  lipgloss.AdaptiveColor{Dark: "#F5A524", Light: "#B76E00"},
	Success: lipgloss.AdaptiveColor{Dark: "#3FB950", Light: "#1A7F37"},
	Danger:  lipgloss.AdaptiveColor{Dark: "#F85149", Light: "#CF222E"},
	Warning: lipgloss.AdaptiveColor{Dark: "#D29922", Light: "#9A6700"},
	Info:    lipgloss.AdaptiveColor{Dark: "#58A6FF", Light: "#0969DA"},
	Muted:   lipgloss.AdaptiveColor{Dark: "#8B949E", Light: "#57606A"},
	Select:  lipgloss.AdaptiveColor{Dark: "#1F2428", Light: "#EAEEF2"},
}

// Styles are lipgloss styles for one render.
type Styles struct {
	Enabled bool
	Accent  lipgloss.Style
	Success lipgloss.Style
	Danger  lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style
	Muted   lipgloss.Style
	Bold    lipgloss.Style
	Select  lipgloss.Style
}

// NewStyles builds styles. Color is disabled when enabled is false.
func NewStyles(enabled bool) Styles {
	r := lipgloss.NewRenderer(os.Stdout)
	if !enabled {
		r.SetColorProfile(termenv.Ascii)
	}
	return Styles{
		Enabled: enabled,
		Accent:  r.NewStyle().Foreground(Colors.Accent).Bold(true),
		Success: r.NewStyle().Foreground(Colors.Success),
		Danger:  r.NewStyle().Foreground(Colors.Danger),
		Warning: r.NewStyle().Foreground(Colors.Warning),
		Info:    r.NewStyle().Foreground(Colors.Info),
		Muted:   r.NewStyle().Foreground(Colors.Muted),
		Bold:    r.NewStyle().Bold(true),
		Select:  r.NewStyle().Background(Colors.Select),
	}
}

// ColorEnabled reports whether color should be used.
func ColorEnabled(flag string, out io.Writer) bool {
	switch strings.ToLower(flag) {
	case "never":
		return false
	case "always":
		return os.Getenv("NO_COLOR") == ""
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") != "" {
		return true
	}
	f, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}

// Symbols is the glyph set, with an ASCII fallback.
type Symbols struct {
	OK, Fail, Warn, Selected, On, Off, Gap, Sep, Minus string
}

// ASCIIMode reports whether glyphs must be ASCII.
func ASCIIMode() bool {
	if os.Getenv("OOPS_ASCII") == "1" || os.Getenv("TERM") == "dumb" {
		return true
	}
	lang := os.Getenv("LC_ALL")
	if lang == "" {
		lang = os.Getenv("LC_CTYPE")
	}
	if lang == "" {
		lang = os.Getenv("LANG")
	}
	if lang != "" {
		up := strings.ToUpper(lang)
		if !strings.Contains(up, "UTF-8") && !strings.Contains(up, "UTF8") {
			return true
		}
	}
	return false
}

// Glyphs returns the active symbol set.
func Glyphs(ascii bool) Symbols {
	if ascii {
		return Symbols{OK: "ok", Fail: "x", Warn: "!", Selected: ">", On: "*", Off: "-", Gap: ".", Sep: "-", Minus: "-"}
	}
	return Symbols{OK: "✓", Fail: "✗", Warn: "!", Selected: "▸", On: "●", Off: "○", Gap: "·", Sep: "·", Minus: "−"}
}
