package middleware

import (
	"log"

	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/ssh"
	"github.com/deemkeen/stegodon/cli"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// MainTui intercepts CLI commands (non-interactive mode) and passes
// interactive sessions through to the bubbletea.Middleware.
func MainTui() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			if cmd := s.Command(); len(cmd) > 0 {
				handleCLI(s, cmd)
				return
			}
			next(s)
		}
	}
}

// TeaHandler creates a bubbletea Model and ProgramOptions for each SSH session.
// Used with wish/v2's bubbletea.Middleware.
func TeaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, _ := s.Pty()

	err, acc := db.GetDB().ReadAccBySession(s)
	if err != nil {
		log.Println("Could not retrieve the user:", err)
		return nil, nil
	}

	m := ui.NewModel(*acc, pty.Window.Width, pty.Window.Height)
	return m, []tea.ProgramOption{
		tea.WithFPS(30),
		tea.WithColorProfile(colorprofile.TrueColor),
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
