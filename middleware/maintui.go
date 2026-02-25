package middleware

import (
	"context"
	"io"
	"log"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/deemkeen/stegodon/cli"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// repaintWriter wraps an io.Writer to completely replace the cursed renderer's
// differential cell output with a full-repaint of the view content.
// This works around bubbletea v2's cursed renderer producing artifacts
// over SSH due to incorrect differential cell updates.
// See: https://github.com/charmbracelet/wish/pull/392
type repaintWriter struct {
	w     io.Writer
	store *ui.ViewStore
	once  sync.Once
}

func (rw *repaintWriter) Write(p []byte) (n int, err error) {
	// Enter alt screen + hide cursor on first write, then never forward
	// the cursed renderer's differential output again.
	rw.once.Do(func() {
		rw.w.Write([]byte("\033[?1049h")) // enter alt screen
		rw.w.Write([]byte("\033[?25l"))   // hide cursor
	})

	content := rw.store.Load()
	if len(content) > 0 {
		// Synchronized output prevents tearing during the repaint.
		rw.w.Write([]byte("\033[?2026h"))   // begin synchronized output
		rw.w.Write([]byte("\033[H\033[2J")) // cursor home + erase screen
		rw.w.Write([]byte(content))         // full view repaint
		rw.w.Write([]byte("\033[?2026l"))   // end synchronized output
	}

	// Report full length consumed so bubbletea doesn't retry.
	n = len(p)
	return
}

func MainTui() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			// Check for CLI command first (non-interactive mode)
			if cmd := s.Command(); len(cmd) > 0 {
				handleCLI(s, cmd)
				next(s)
				return
			}

			pty, windowChanges, active := s.Pty()
			if !active {
				wish.Println(s, "no active terminal, skipping")
				next(s)
				return
			}

			err, acc := db.GetDB().ReadAccBySession(s)
			if err != nil {
				log.Println("Could not retrieve the user:", err)
				next(s)
				return
			}

			m := ui.NewModel(*acc, pty.Window.Width, pty.Window.Height)
			viewStore := &ui.ViewStore{}
			m.ViewContent = viewStore
			in, out := ptyIO(s)
			rw := &repaintWriter{w: out, store: viewStore}
			p := tea.NewProgram(m,
				tea.WithFPS(30),
				tea.WithInput(in),
				tea.WithOutput(rw),
				tea.WithEnvironment(s.Environ()),
				tea.WithColorProfile(colorprofile.TrueColor),
				tea.WithWindowSize(pty.Window.Width, pty.Window.Height),
			)

			// Forward window resize events to the tea.Program
			ctx, cancel := context.WithCancel(s.Context())
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case w := <-windowChanges:
						if p != nil {
							p.Send(tea.WindowSizeMsg{Width: w.Width, Height: w.Height})
						}
					}
				}
			}()

			if _, err := p.Run(); err != nil {
				log.Printf("TUI program error: %v", err)
			}
			p.Kill()
			// Restore terminal: show cursor + leave alt screen
			rw.w.Write([]byte("\033[?25h"))   // show cursor
			rw.w.Write([]byte("\033[?1049l")) // leave alt screen
			cancel()
			next(s)
		}
	}
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

func (w *dbWrapper) CreateNoteWithReply(userId interface{}, message string, inReplyToURI string) (interface{}, error) {
	return w.db.CreateNoteWithReply(userId.(uuid.UUID), message, inReplyToURI)
}

func (w *dbWrapper) ReadNoteIdWithReplyInfo(id interface{}) (error, *domain.Note) {
	return w.db.ReadNoteIdWithReplyInfo(id.(uuid.UUID))
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

func (w *dbWrapper) DeleteAllNotifications(accountId interface{}) error {
	return w.db.DeleteAllNotifications(accountId.(uuid.UUID))
}

// Note/Activity lookups for like/boost
func (w *dbWrapper) ReadNoteId(id interface{}) (error, *domain.Note) {
	return w.db.ReadNoteId(id.(uuid.UUID))
}

func (w *dbWrapper) ReadActivityById(id interface{}) (error, *domain.Activity) {
	return w.db.ReadActivityById(id.(uuid.UUID))
}

func (w *dbWrapper) ReadAccByUsername(username string) (error, *domain.Account) {
	return w.db.ReadAccByUsername(username)
}

// Like operations
func (w *dbWrapper) HasLike(accountId, noteId interface{}) (bool, error) {
	return w.db.HasLike(accountId.(uuid.UUID), noteId.(uuid.UUID))
}

func (w *dbWrapper) HasLikeByObjectURI(accountId interface{}, objectURI string) (bool, error) {
	return w.db.HasLikeByObjectURI(accountId.(uuid.UUID), objectURI)
}

func (w *dbWrapper) CreateLike(like *domain.Like) error {
	return w.db.CreateLike(like)
}

func (w *dbWrapper) CreateLikeByObjectURI(like *domain.Like, objectURI string) error {
	return w.db.CreateLikeByObjectURI(like, objectURI)
}

func (w *dbWrapper) DeleteLikeByAccountAndNote(accountId, noteId interface{}) error {
	return w.db.DeleteLikeByAccountAndNote(accountId.(uuid.UUID), noteId.(uuid.UUID))
}

func (w *dbWrapper) DeleteLikeByAccountAndObjectURI(accountId interface{}, objectURI string) error {
	return w.db.DeleteLikeByAccountAndObjectURI(accountId.(uuid.UUID), objectURI)
}

func (w *dbWrapper) IncrementLikeCountByNoteId(noteId interface{}) error {
	return w.db.IncrementLikeCountByNoteId(noteId.(uuid.UUID))
}

func (w *dbWrapper) DecrementLikeCountByNoteId(noteId interface{}) error {
	return w.db.DecrementLikeCountByNoteId(noteId.(uuid.UUID))
}

func (w *dbWrapper) IncrementLikeCountByObjectURI(objectURI string) error {
	return w.db.IncrementLikeCountByObjectURI(objectURI)
}

func (w *dbWrapper) DecrementLikeCountByObjectURI(objectURI string) error {
	return w.db.DecrementLikeCountByObjectURI(objectURI)
}

func (w *dbWrapper) ReadLikeByAccountAndNote(accountId, noteId interface{}) (error, *domain.Like) {
	return w.db.ReadLikeByAccountAndNote(accountId.(uuid.UUID), noteId.(uuid.UUID))
}

func (w *dbWrapper) ReadLikeByAccountAndObjectURI(accountId interface{}, objectURI string) (error, *domain.Like) {
	return w.db.ReadLikeByAccountAndObjectURI(accountId.(uuid.UUID), objectURI)
}

func (w *dbWrapper) CreateNotification(notification *domain.Notification) error {
	return w.db.CreateNotification(notification)
}

// Boost operations
func (w *dbWrapper) HasBoost(accountId, noteId interface{}) (bool, error) {
	return w.db.HasBoost(accountId.(uuid.UUID), noteId.(uuid.UUID))
}

func (w *dbWrapper) HasBoostByObjectURI(accountId interface{}, objectURI string) (bool, error) {
	return w.db.HasBoostByObjectURI(accountId.(uuid.UUID), objectURI)
}

func (w *dbWrapper) CreateBoost(boost *domain.Boost) error {
	return w.db.CreateBoost(boost)
}

func (w *dbWrapper) CreateBoostByObjectURI(boost *domain.Boost, objectURI string) error {
	return w.db.CreateBoostByObjectURI(boost, objectURI)
}

func (w *dbWrapper) DeleteBoostByAccountAndNote(accountId, noteId interface{}) error {
	return w.db.DeleteBoostByAccountAndNote(accountId.(uuid.UUID), noteId.(uuid.UUID))
}

func (w *dbWrapper) DeleteBoostByAccountAndObjectURI(accountId interface{}, objectURI string) error {
	return w.db.DeleteBoostByAccountAndObjectURI(accountId.(uuid.UUID), objectURI)
}

func (w *dbWrapper) IncrementBoostCountByNoteId(noteId interface{}) error {
	return w.db.IncrementBoostCountByNoteId(noteId.(uuid.UUID))
}

func (w *dbWrapper) DecrementBoostCountByNoteId(noteId interface{}) error {
	return w.db.DecrementBoostCountByNoteId(noteId.(uuid.UUID))
}

func (w *dbWrapper) IncrementBoostCountByObjectURI(objectURI string) error {
	return w.db.IncrementBoostCountByObjectURI(objectURI)
}

func (w *dbWrapper) DecrementBoostCountByObjectURI(objectURI string) error {
	return w.db.DecrementBoostCountByObjectURI(objectURI)
}

func (w *dbWrapper) ReadBoostByAccountAndNote(accountId, noteId interface{}) (error, *domain.Boost) {
	return w.db.ReadBoostByAccountAndNote(accountId.(uuid.UUID), noteId.(uuid.UUID))
}

func (w *dbWrapper) ReadBoostByAccountAndObjectURI(accountId interface{}, objectURI string) (error, *domain.Boost) {
	return w.db.ReadBoostByAccountAndObjectURI(accountId.(uuid.UUID), objectURI)
}

// Follow operations
func (w *dbWrapper) ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount) {
	return w.db.ReadRemoteAccountByActorURI(actorURI)
}

func (w *dbWrapper) ReadFollowByAccountIds(accountId, targetAccountId interface{}) (error, *domain.Follow) {
	return w.db.ReadFollowByAccountIds(accountId.(uuid.UUID), targetAccountId.(uuid.UUID))
}

func (w *dbWrapper) CreateLocalFollow(followerAccountId, targetAccountId interface{}) error {
	return w.db.CreateLocalFollow(followerAccountId.(uuid.UUID), targetAccountId.(uuid.UUID))
}

func (w *dbWrapper) DeleteLocalFollow(followerAccountId, targetAccountId interface{}) error {
	return w.db.DeleteLocalFollow(followerAccountId.(uuid.UUID), targetAccountId.(uuid.UUID))
}

func (w *dbWrapper) IsFollowingLocal(followerAccountId, targetAccountId interface{}) (bool, error) {
	return w.db.IsFollowingLocal(followerAccountId.(uuid.UUID), targetAccountId.(uuid.UUID))
}

func (w *dbWrapper) DeleteFollowByURI(uri string) error {
	return w.db.DeleteFollowByURI(uri)
}

func (w *dbWrapper) ReadAccById(id interface{}) (error, *domain.Account) {
	return w.db.ReadAccById(id.(uuid.UUID))
}
