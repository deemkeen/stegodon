package common

import "github.com/charmbracelet/lipgloss"

const (
	// === Primary UI Colors ===
	COLOR_ACCENT    = "69" // ANSI 69 (#5f87ff) - Primary accent: borders, selections, header
	COLOR_SECONDARY = "75" // ANSI 75 (#5fafff) - Secondary accent: timestamps, domains, hashtags

	// === Text Colors ===
	COLOR_WHITE = "255" // ANSI 255 (#eeeeee) - Primary text, post content
	COLOR_LIGHT = "250" // ANSI 250 (#bcbcbc) - Secondary text, slightly dimmed
	COLOR_MUTED = "245" // ANSI 245 (#8a8a8a) - Tertiary text, disabled, hints
	COLOR_DIM   = "240" // ANSI 240 (#585858) - Very dim text, borders, separators

	// === Semantic Colors ===
	COLOR_USERNAME = "48"  // ANSI 48 (#00ff87) - Usernames stand out
	COLOR_SUCCESS  = "48"  // ANSI 48 (#00ff87) - Success messages (same as username for cohesion)
	COLOR_ERROR    = "196" // ANSI 196 (#ff0000) - Errors, delete actions, warnings
	COLOR_CRITICAL = "9"   // ANSI 9 (#ff5555) - Critical errors, terminal size warnings
	COLOR_WARNING  = "214" // ANSI 214 (#ffaf00) - Content warnings, caution (amber)

	// === Interactive Elements ===
	COLOR_HASHTAG = "75"  // ANSI 75 (#5fafff) - Hashtags (same as secondary for harmony)
	COLOR_MENTION = "48"  // ANSI 48 (#00ff87) - Mentions (same as username for consistency)
	COLOR_LINK    = "48"  // ANSI 48 (#00ff87) - Hyperlinks (same as username/mention)
	COLOR_BUTTON  = "117" // ANSI 117 (#87d7ff) - Button highlights, active elements

	// === Section/Title Colors ===
	COLOR_CAPTION = "170" // ANSI 170 (#d75fd7) - Section captions, titles
	COLOR_HELP    = "245" // ANSI 245 (#8a8a8a) - Help text (same as muted)

	// === Background Colors ===
	COLOR_BLACK = "0" // ANSI 0 (#000000) - Button text on light backgrounds

	// === OSC8 Hyperlink Colors (RGB format for true color terminals) ===
	COLOR_LINK_RGB    = "0;255;135" // RGB for hyperlinks (#00ff87) - matches COLOR_LINK
	COLOR_MENTION_RGB = "0;255;135" // RGB for mentions (#00ff87) - matches COLOR_MENTION

	// === ANSI Escape Sequences (for inline coloring without breaking backgrounds) ===
	ANSI_WARNING_START = "\033[38;5;214m" // Start warning color (orange/yellow)
	ANSI_COLOR_RESET   = "\033[39m"       // Reset foreground to default

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
