package middleware

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/deemkeen/stegodon/cli"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"github.com/muesli/termenv"
)

func MainTui() wish.Middleware {
	teaHandler := func(s ssh.Session) *tea.Program {
		// Check for CLI command first (non-interactive mode)
		if cmd := s.Command(); len(cmd) > 0 {
			handleCLI(s, cmd)
			return nil // Don't start TUI
		}

		pty, _, active := s.Pty()
		if !active {
			wish.Println(s, "no active terminal, skipping")
			return nil
		}

		err, acc := db.GetDB().ReadAccBySession(s)
		if err != nil {
			log.Println("Could not retrieve the user:", err)
			return nil
		}

		// Set the global color profile to ANSI256 for Docker compatibility
		lipgloss.SetColorProfile(termenv.ANSI256)

		m := ui.NewModel(*acc, pty.Window.Width, pty.Window.Height)
		return tea.NewProgram(m, tea.WithFPS(60), tea.WithInput(s), tea.WithOutput(s), tea.WithAltScreen())
	}
	return bm.MiddlewareWithProgramHandler(teaHandler, termenv.ANSI256)
}

// handleCLI processes CLI commands in non-interactive mode
func handleCLI(s ssh.Session, cmd []string) {
	database := db.GetDB()

	// Get authenticated user from session
	err, acc := database.ReadAccBySession(s)
	if err != nil {
		wish.Println(s, "Error: not authenticated")
		return
	}

	// Get config
	conf, err := util.ReadConf()
	if err != nil {
		wish.Printf(s, "Error: failed to load config: %v\n", err)
		return
	}

	// Create CLI handler and execute command
	handler := cli.NewHandler(s, &dbWrapper{database}, acc, conf)
	if err := handler.Execute(cmd); err != nil {
		// Error already printed by handler
		return
	}
}

// dbWrapper wraps *db.DB to implement cli.Database interface
type dbWrapper struct {
	db *db.DB
}

func (w *dbWrapper) CreateNote(userId interface{}, message string) (interface{}, error) {
	return w.db.CreateNote(userId.(uuid.UUID), message)
}

func (w *dbWrapper) ReadHomeTimelinePosts(accountId interface{}, limit int) (error, *[]domain.HomePost) {
	return w.db.ReadHomeTimelinePosts(accountId.(uuid.UUID), limit)
}

func (w *dbWrapper) ReadNotificationsByAccountId(accountId interface{}, limit int) (error, *[]domain.Notification) {
	return w.db.ReadNotificationsByAccountId(accountId.(uuid.UUID), limit)
}

func (w *dbWrapper) CountUnreadNotifications(accountId interface{}) (int, error) {
	return w.db.ReadUnreadNotificationCount(accountId.(uuid.UUID))
}
