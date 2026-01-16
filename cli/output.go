package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Output handles formatting responses in text or JSON format
type Output struct {
	writer   io.Writer
	jsonMode bool
}

// NewOutput creates a new output handler
func NewOutput(w io.Writer, jsonMode bool) *Output {
	return &Output{
		writer:   w,
		jsonMode: jsonMode,
	}
}

// IsJSON returns true if output is in JSON mode
func (o *Output) IsJSON() bool {
	return o.jsonMode
}

// Error outputs an error message
func (o *Output) Error(err error) {
	if o.jsonMode {
		o.writeJSON(map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		fmt.Fprintf(o.writer, "Error: %v\n", err)
	}
}

// ErrorWithDetails outputs an error with additional details
func (o *Output) ErrorWithDetails(message string, details string) {
	if o.jsonMode {
		o.writeJSON(map[string]interface{}{
			"error":   message,
			"details": details,
		})
	} else {
		fmt.Fprintf(o.writer, "Error: %s (%s)\n", message, details)
	}
}

// Success outputs a success message (text mode only, JSON uses specific methods)
func (o *Output) Success(format string, args ...interface{}) {
	if !o.jsonMode {
		fmt.Fprintf(o.writer, format, args...)
	}
}

// Print outputs a line (text mode only)
func (o *Output) Print(format string, args ...interface{}) {
	if !o.jsonMode {
		fmt.Fprintf(o.writer, format, args...)
	}
}

// Println outputs a line with newline (text mode only)
func (o *Output) Println(text string) {
	if !o.jsonMode {
		fmt.Fprintln(o.writer, text)
	}
}

// JSON outputs any value as JSON
func (o *Output) JSON(v interface{}) {
	if o.jsonMode {
		o.writeJSON(v)
	}
}

// writeJSON marshals and writes JSON to the output
func (o *Output) writeJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		// Fallback to error JSON if marshaling fails
		fmt.Fprintf(o.writer, `{"error":"failed to marshal JSON: %s"}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(o.writer, string(data))
}

// PostResponse represents a post creation response
type PostResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// TimelinePost represents a post in timeline output
type TimelinePost struct {
	ID         string    `json:"id"`
	Author     string    `json:"author"`
	Domain     string    `json:"domain,omitempty"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
	ReplyCount int       `json:"reply_count"`
	LikeCount  int       `json:"like_count"`
	BoostCount int       `json:"boost_count"`
}

// TimelineResponse represents the timeline output
type TimelineResponse struct {
	Posts []TimelinePost `json:"posts"`
	Count int            `json:"count"`
}

// NotificationItem represents a notification in output
type NotificationItem struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Actor       string    `json:"actor"`
	NotePreview string    `json:"note_preview,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// NotificationsResponse represents the notifications output
type NotificationsResponse struct {
	Notifications []NotificationItem `json:"notifications"`
	UnreadCount   int                `json:"unread_count"`
}

// HelpCommand represents a command in help output
type HelpCommand struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Usage       string   `json:"usage"`
	Flags       []string `json:"flags,omitempty"`
}

// HelpResponse represents the help output
type HelpResponse struct {
	Version     string        `json:"version"`
	Commands    []HelpCommand `json:"commands"`
	GlobalFlags []string      `json:"global_flags"`
}

// FormatTimeAgo returns a human-readable time difference
func FormatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
