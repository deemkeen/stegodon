package common

import "charm.land/lipgloss/v2"

const (
	// === Primary UI Colors (true color hex — terminal-palette independent) ===
	COLOR_ACCENT    = "#5f87ff" // Primary accent: borders, selections, header
	COLOR_SECONDARY = "#5fafff" // Secondary accent: timestamps, domains, hashtags

	// === Text Colors ===
	COLOR_WHITE = "#eeeeee" // Primary text, post content
	COLOR_LIGHT = "#bcbcbc" // Secondary text, slightly dimmed
	COLOR_MUTED = "#8a8a8a" // Tertiary text, disabled, hints
	COLOR_DIM   = "#585858" // Very dim text, borders, separators

	// === Semantic Colors ===
	COLOR_USERNAME = "#00ff87" // Usernames stand out
	COLOR_SUCCESS  = "#00ff87" // Success messages (same as username for cohesion)
	COLOR_ERROR    = "#ff0000" // Errors, delete actions, warnings
	COLOR_CRITICAL = "#ff5555" // Critical errors, terminal size warnings
	COLOR_WARNING  = "#ffaf00" // Content warnings, caution (amber)

	// === Interactive Elements ===
	COLOR_HASHTAG = "#5fafff" // Hashtags (same as secondary for harmony)
	COLOR_MENTION = "#00ff87" // Mentions (same as username for consistency)
	COLOR_LINK    = "#00ff87" // Hyperlinks (same as username/mention)
	COLOR_BUTTON  = "#87d7ff" // Button highlights, active elements

	// === Section/Title Colors ===
	COLOR_CAPTION = "#d75fd7" // Section captions, titles
	COLOR_HELP    = "#8a8a8a" // Help text (same as muted)

	// === Background Colors ===
	COLOR_BLACK = "#000000" // Button text on light backgrounds

	// === ANSI Escape Sequences — canonical values live in util/ to avoid import cycles ===
	// Use util.AnsiHashtagStart, util.AnsiLinkStart, etc. directly.
	// These aliases exist for header/badge coloring that can't easily import util.
	ANSI_WARNING_START = "\033[38;2;255;175;0m" // #ffaf00 - matches COLOR_WARNING (keep in sync with util.AnsiWarningStart)
	ANSI_COLOR_RESET   = "\033[39m"             // Reset foreground to default (keep in sync with util.AnsiColorReset)

	// === Deprecated aliases (for backwards compatibility during transition) ===
	// These will be removed in a future version - use the semantic names above
	COLOR_GREY        = COLOR_MUTED     // Use COLOR_MUTED instead
	COLOR_MAGENTA     = COLOR_CAPTION   // Use COLOR_CAPTION instead
	COLOR_LIGHTBLUE   = COLOR_ACCENT    // Use COLOR_ACCENT instead
	COLOR_GREEN       = COLOR_USERNAME  // Use COLOR_USERNAME instead
	COLOR_BLUE        = COLOR_SECONDARY // Use COLOR_SECONDARY instead
	COLOR_DARK_GREY   = COLOR_DIM       // Use COLOR_DIM instead
	COLOR_BORDER_GREY = COLOR_DIM       // Use COLOR_DIM instead
	COLOR_RED         = COLOR_ERROR     // Use COLOR_ERROR instead
	COLOR_CYAN        = COLOR_BUTTON    // Use COLOR_BUTTON instead
	COLOR_BRIGHT_RED  = COLOR_CRITICAL  // Use COLOR_CRITICAL instead
)

var (
	HelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_GREY)).Padding(0, 2)
	CaptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(COLOR_MAGENTA)).Padding(2)

	// === Shared List Styles ===
	// Use these for consistent list rendering across followers, following, localusers, admin

	// ListItemStyle is the base style for unselected list items
	ListItemStyle = lipgloss.NewStyle()

	// ListItemSelectedStyle is for the selected item text (highlighted color + bold)
	ListItemSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(COLOR_USERNAME)).
				Bold(true)

	// ListEmptyStyle is for empty list messages
	ListEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(COLOR_DIM)).
			Italic(true)

	// ListStatusStyle is for status messages (success, info)
	ListStatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(COLOR_SUCCESS))

	// ListErrorStyle is for error messages
	ListErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(COLOR_ERROR))

	// ListBadgeStyle is for inline badges like [local], [pending], [ADMIN]
	ListBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(COLOR_DIM))

	// ListBadgeMutedStyle is for muted user badge
	ListBadgeMutedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(COLOR_ERROR))

	// ListBadgeEnabledStyle is for enabled/active status badges
	ListBadgeEnabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(COLOR_SUCCESS))
)

const (
	// ListSelectedPrefix is the indicator shown before selected items
	ListSelectedPrefix = "› "
	// ListUnselectedPrefix is the spacing for unselected items (same width as selected)
	ListUnselectedPrefix = "  "
)

// DefaultWindowWidth returns the usable width after accounting for outer margins
// The offset of 10 accounts for: outer margins (2*2=4) + potential scrollbar (2) + safety buffer (4)
func DefaultWindowWidth(width int) int {
	return width - 10
}

// DefaultWindowHeight returns the usable height after accounting for outer margins
// The offset of 10 accounts for: outer margins (2*2=4) + potential terminal chrome (6)
func DefaultWindowHeight(heigth int) int {
	return heigth - 10
}

func DefaultCreateNoteWidth(width int) int {
	return width / 4
}

func DefaultListWidth(width int) int {
	return width - DefaultCreateNoteWidth(width)
}

func DefaultListHeight(height int) int {
	return height
}

// Terms and Conditions dialog constants
const (
	TermsDialogBorderAndMargin = 8  // 2 (border) + 6 (margins)
	TermsDialogMinWidth        = 60 // Minimum width for terms dialog
)
