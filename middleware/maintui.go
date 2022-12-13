package middleware

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/ui"
	"github.com/gliderlabs/ssh"
	"github.com/muesli/termenv"
	"log"
)

func MainTui() wish.Middleware {
	teaHandler := func(s ssh.Session) *tea.Program {

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

		m := ui.NewModel(*acc, pty.Window.Width, pty.Window.Height)
		return tea.NewProgram(m, tea.WithInput(s), tea.WithOutput(s), tea.WithAltScreen())
	}
	return bm.MiddlewareWithProgramHandler(teaHandler, termenv.ANSI256)
}
