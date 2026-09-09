package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// UI Theme colors
var (
	PrimaryColor   = lipgloss.Color("#6366F1") // Indigo
	SecondaryColor = lipgloss.Color("#EC4899") // Pink
	SuccessColor   = lipgloss.Color("#10B981") // Emerald Green
	WarningColor   = lipgloss.Color("#F59E0B") // Amber
	DangerColor    = lipgloss.Color("#EF4444") // Red
	MutedColor     = lipgloss.Color("#64748B") // Slate Gray
	BgDark         = lipgloss.Color("#0F172A") // Dark Slate
	TextLight      = lipgloss.Color("#F8FAFC") // Bright White
	TextDim        = lipgloss.Color("#94A3B8") // Light Slate
)

// LipGloss Styles
var (
	HeaderTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(TextLight).
				Background(PrimaryColor).
				Padding(0, 1)

	HeaderTagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1E1B4B")).
			Background(lipgloss.Color("#A5B4FC")).
			Padding(0, 1)

	SelectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(SuccessColor)

	NormalRowStyle = lipgloss.NewStyle().
			Foreground(TextLight)

	DimStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	StatusRunningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(SuccessColor)

	StatusStoppedStyle = lipgloss.NewStyle().
				Foreground(DangerColor)

	StatusPausedStyle = lipgloss.NewStyle().
				Foreground(WarningColor)

	InfoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2)

	HelpBarStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			PaddingTop(1)
)
