package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// handlePost creates a new note
func (h *Handler) handlePost(args []string) error {
	var message string

	if len(args) == 0 {
		err := fmt.Errorf("usage: post <message> or post -")
		h.output.Error(err)
		return err
	}

	if args[0] == "-" {
		// Read from stdin
		data, err := io.ReadAll(h.session)
		if err != nil {
			h.output.Error(err)
			return err
		}
		message = strings.TrimSpace(string(data))
	} else {
		message = strings.Join(args, " ")
	}

	// Validate message not empty
	if message == "" {
		err := fmt.Errorf("message cannot be empty")
		h.output.Error(err)
		return err
	}

	// Validate visible character count
	visibleChars := util.CountVisibleChars(message)
	maxChars := h.conf.Conf.MaxChars
	if visibleChars > maxChars {
		err := fmt.Errorf("message too long (%d chars, max %d)", visibleChars, maxChars)
		h.output.ErrorWithDetails("message too long", fmt.Sprintf("%d chars, max %d", visibleChars, maxChars))
		return err
	}

	// Validate total note length (including markdown syntax)
	if err := util.ValidateNoteLength(message); err != nil {
		h.output.Error(err)
		return err
	}

	// Create the note
	noteId, err := h.db.CreateNote(h.account.Id, message)
	if err != nil {
		h.output.Error(err)
		return err
	}

	// Output response
	if h.output.IsJSON() {
		h.output.JSON(PostResponse{
			ID:        fmt.Sprintf("%v", noteId),
			Message:   message,
			CreatedAt: time.Now(),
		})
	} else {
		// Convert noteId to string for display
		var idStr string
		switch id := noteId.(type) {
		case uuid.UUID:
			idStr = id.String()
		default:
			idStr = fmt.Sprintf("%v", id)
		}
		h.output.Success("Posted: %s\n", idStr)
	}

	return nil
}
