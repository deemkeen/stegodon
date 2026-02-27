package common

import "charm.land/lipgloss/v2"

// Layout constants for the TUI
// These values are derived from the actual styling applied to components

const (
	// HeaderHeight is the height of the header bar (single line with Inline(true))
	HeaderHeight = 1

	// HeaderNewline is the newline added after the header in View()
	HeaderNewline = 1

	// FooterHeight is the height of the help/footer text
	FooterHeight = 1

	// PanelMarginVertical is the vertical margin applied to each panel (Margin(1) = 1 top + 1 bottom)
	PanelMarginVertical = 2

	// PanelMarginLeft is the left margin applied to panels (MarginLeft(1))
	PanelMarginLeft = 1

	// BorderWidth is the width of a normal border (1 char on each side)
	BorderWidth = 1

	// TwoPanelBorderWidth is total horizontal space taken by borders when two panels are shown
	// Left panel: hidden border (1) + right edge (1) = 2
	// Right panel: left edge (1) + right edge (1) = 2
	// Total = 4
	TwoPanelBorderWidth = 4

	// TwoPanelMarginWidth is total horizontal space taken by margins for two panels
	// Left panel margin (1) + right panel margin (1) = 2
	TwoPanelMarginWidth = 2

	// HeaderSidePadding is the padding on each side of the header (2 spaces each side)
	HeaderSidePadding = 2

	// DefaultItemHeight is the estimated height of a single list item in lines
	DefaultItemHeight = 3

	// MinItemsPerPage is the minimum number of items to show per page
	MinItemsPerPage = 3

	// DefaultItemsPerPage is used when dynamic calculation isn't possible
	DefaultItemsPerPage = 10

	// HeaderTotalPadding is the total horizontal padding for header content (2 spaces each side)
	// Used in header.go for spacing calculation: width - totalTextLen - 4
	HeaderTotalPadding = 4

	// CreateUserDialogMargin is the horizontal margin for the create user dialog
	// Style.Margin(0, 3) = 3 chars on each side = 6, plus border (2) = 8
	CreateUserDialogBorderAndMargin = 8

	// CreateUserMinWidth is the minimum width for the create user dialog
	CreateUserMinWidth = 40

	// TextInputDefaultWidth is a reasonable default width for text input fields
	TextInputDefaultWidth = 30

	// MaxContentTruncateWidth is the maximum width for truncating post content
	// This prevents very long lines on wide terminals
	MaxContentTruncateWidth = 150

	// ReplyIndentWidth is the number of spaces used to indent replies in thread view
	ReplyIndentWidth = 4

	// TimelineRefreshSeconds is the interval for auto-refreshing timeline views
	TimelineRefreshSeconds = 10

	// HomeTimelinePostLimit is the maximum number of posts to load in home timeline
	HomeTimelinePostLimit = 50

	// MaxNoteDBLength is the maximum character length for notes in the database
	MaxNoteDBLength = 1000

	// HoursPerDay is used for time formatting calculations
	HoursPerDay = 24

	// MaxDisplayContentLength is the maximum characters to display for note content
	// Users can press 'o' to view full content via original link
	MaxDisplayContentLength = 1000
)

// VerticalLayoutOffset returns the total vertical space taken by header, footer, and margins
// Use this to calculate available height for panel content
func VerticalLayoutOffset() int {
	return HeaderHeight + HeaderNewline + PanelMarginVertical + FooterHeight
}

// HorizontalPanelOffset returns the total horizontal space taken by borders and margins
// when rendering two side-by-side panels
func HorizontalPanelOffset() int {
	return TwoPanelBorderWidth + TwoPanelMarginWidth
}

// CalculateAvailableHeight returns the height available for panel content
// after accounting for header, footer, and panel margins
func CalculateAvailableHeight(totalHeight int) int {
	return totalHeight - VerticalLayoutOffset()
}

// CalculateRightPanelWidth returns the width for the right panel
// after accounting for left panel width and borders/margins
func CalculateRightPanelWidth(totalWidth, leftPanelWidth int) int {
	return totalWidth - leftPanelWidth - HorizontalPanelOffset()
}

// CalculateLeftPanelWidth returns the width for the left panel (1/3 of total)
func CalculateLeftPanelWidth(totalWidth int) int {
	return totalWidth / 3
}

// CalculateItemsPerPage returns the number of items that fit in the available height
// based on the estimated item height
func CalculateItemsPerPage(availableHeight, itemHeight int) int {
	if itemHeight <= 0 {
		itemHeight = DefaultItemHeight
	}
	items := availableHeight / itemHeight
	if items < MinItemsPerPage {
		return MinItemsPerPage
	}
	return items
}

// CalculateContentWidth returns the width for content inside a panel
// after accounting for internal padding
func CalculateContentWidth(panelWidth, padding int) int {
	return panelWidth - (padding * 2)
}

// MeasureHeight returns the height of a rendered string using lipgloss
func MeasureHeight(rendered string) int {
	return lipgloss.Height(rendered)
}

// MeasureWidth returns the width of a rendered string using lipgloss
func MeasureWidth(rendered string) int {
	return lipgloss.Width(rendered)
}
