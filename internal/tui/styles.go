package tui

import "github.com/charmbracelet/lipgloss"

// Adaptive colors that work across light and dark terminals.
// Light value is used when the terminal has a light background,
// Dark value when it has a dark background.
var (
	accentColor = lipgloss.AdaptiveColor{Light: "5", Dark: "6"}    // magenta / cyan
	subtleColor = lipgloss.AdaptiveColor{Light: "8", Dark: "7"}    // grey tones
	textColor   = lipgloss.AdaptiveColor{Light: "0", Dark: "15"}   // black / white
	selectFg    = lipgloss.AdaptiveColor{Light: "15", Dark: "0"}   // inverse text
	selectBg    = lipgloss.AdaptiveColor{Light: "4", Dark: "14"}   // blue / bright blue
	pauseColor  = lipgloss.AdaptiveColor{Light: "3", Dark: "11"}   // yellow / bright yellow

	headerStyle = lipgloss.NewStyle().
			Background(accentColor).
			Foreground(lipgloss.AdaptiveColor{Light: "15", Dark: "0"}).
			Bold(true).
			Padding(0, 1)

	pauseBannerStyle = lipgloss.NewStyle().
				Background(pauseColor).
				Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "0"}).
				Bold(true).
				Padding(0, 1)

	placeholderStyle = lipgloss.NewStyle().
				Foreground(subtleColor).
				Italic(true)
)
