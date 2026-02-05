package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
)

// Session interface represents the minimal session requirements for CLI operations
type Session interface {
	io.Reader
	io.Writer
}

// Database interface for CLI operations
type Database interface {
	CreateNote(userId interface{}, message string) (interface{}, error)
	CreateNoteWithReply(userId interface{}, message string, inReplyToURI string) (interface{}, error)
	ReadNoteIdWithReplyInfo(id interface{}) (error, *domain.Note)
	ReadHomeTimelinePosts(accountId interface{}, limit int) (error, *[]domain.HomePost)
	ReadNotificationsByAccountId(accountId interface{}, limit int) (error, *[]domain.Notification)
	CountUnreadNotifications(accountId interface{}) (int, error)
	DeleteAllNotifications(accountId interface{}) error

	// Note/Activity lookups for like/boost
	ReadNoteId(id interface{}) (error, *domain.Note)
	ReadActivityById(id interface{}) (error, *domain.Activity)
	ReadAccByUsername(username string) (error, *domain.Account)

	// Like operations
	HasLike(accountId, noteId interface{}) (bool, error)
	HasLikeByObjectURI(accountId interface{}, objectURI string) (bool, error)
	CreateLike(like *domain.Like) error
	CreateLikeByObjectURI(like *domain.Like, objectURI string) error
	DeleteLikeByAccountAndNote(accountId, noteId interface{}) error
	DeleteLikeByAccountAndObjectURI(accountId interface{}, objectURI string) error
	IncrementLikeCountByNoteId(noteId interface{}) error
	DecrementLikeCountByNoteId(noteId interface{}) error
	IncrementLikeCountByObjectURI(objectURI string) error
	DecrementLikeCountByObjectURI(objectURI string) error
	ReadLikeByAccountAndNote(accountId, noteId interface{}) (error, *domain.Like)
	ReadLikeByAccountAndObjectURI(accountId interface{}, objectURI string) (error, *domain.Like)
	CreateNotification(notification *domain.Notification) error

	// Boost operations
	HasBoost(accountId, noteId interface{}) (bool, error)
	HasBoostByObjectURI(accountId interface{}, objectURI string) (bool, error)
	CreateBoost(boost *domain.Boost) error
	CreateBoostByObjectURI(boost *domain.Boost, objectURI string) error
	DeleteBoostByAccountAndNote(accountId, noteId interface{}) error
	DeleteBoostByAccountAndObjectURI(accountId interface{}, objectURI string) error
	IncrementBoostCountByNoteId(noteId interface{}) error
	DecrementBoostCountByNoteId(noteId interface{}) error
	IncrementBoostCountByObjectURI(objectURI string) error
	DecrementBoostCountByObjectURI(objectURI string) error
	ReadBoostByAccountAndNote(accountId, noteId interface{}) (error, *domain.Boost)
	ReadBoostByAccountAndObjectURI(accountId interface{}, objectURI string) (error, *domain.Boost)

	// Follow operations
	ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount)
	ReadFollowByAccountIds(accountId, targetAccountId interface{}) (error, *domain.Follow)
	CreateLocalFollow(followerAccountId, targetAccountId interface{}) error
	DeleteLocalFollow(followerAccountId, targetAccountId interface{}) error
	IsFollowingLocal(followerAccountId, targetAccountId interface{}) (bool, error)
	DeleteFollowByURI(uri string) error
	ReadAccById(id interface{}) (error, *domain.Account)
}

// Handler processes CLI commands
type Handler struct {
	session  Session
	db       Database
	account  *domain.Account
	output   *Output
	jsonMode bool
	conf     *util.AppConfig
}

// NewHandler creates a new CLI handler
func NewHandler(s Session, db Database, acc *domain.Account, conf *util.AppConfig) *Handler {
	return &Handler{
		session:  s,
		db:       db,
		account:  acc,
		jsonMode: false,
		conf:     conf,
	}
}

// Execute parses and executes a CLI command
func (h *Handler) Execute(args []string) error {
	// Parse global flags first
	args, h.jsonMode = parseGlobalFlags(args)

	// Create output handler
	h.output = NewOutput(h.session, h.jsonMode)

	// No command provided
	if len(args) == 0 {
		return h.showHelp()
	}

	// Route to command handler
	cmd := strings.ToLower(args[0])
	cmdArgs := args[1:]

	switch cmd {
	case "post":
		return h.handlePost(cmdArgs)
	case "timeline":
		return h.handleTimeline(cmdArgs)
	case "like":
		return h.handleLike(cmdArgs)
	case "boost":
		return h.handleBoost(cmdArgs)
	case "follow":
		return h.handleFollow(cmdArgs)
	case "notifications":
		return h.handleNotifications(cmdArgs)
	case "clear-notifications":
		return h.handleClearNotifications(cmdArgs)
	case "--help", "-h", "help":
		return h.showHelp()
	default:
		err := fmt.Errorf("unknown command: %s", cmd)
		h.output.Error(err)
		return err
	}
}

// parseGlobalFlags extracts global flags like --json from args
func parseGlobalFlags(args []string) ([]string, bool) {
	jsonMode := false
	var filtered []string

	for _, arg := range args {
		switch arg {
		case "--json", "-j":
			jsonMode = true
		default:
			filtered = append(filtered, arg)
		}
	}

	return filtered, jsonMode
}

// showHelp displays help information
func (h *Handler) showHelp() error {
	if h.output.IsJSON() {
		help := HelpResponse{
			Version: util.GetVersion(),
			Commands: []HelpCommand{
				{
					Name:        "post",
					Description: "Create a new note",
					Usage:       "post <message> [--reply-to <id>]",
					Flags:       []string{"-: read message from stdin", "--reply-to <id>: reply to a post"},
				},
				{
					Name:        "timeline",
					Description: "Show recent home timeline",
					Usage:       "timeline [-n <count>]",
					Flags:       []string{"-n <count>: limit number of posts (default 20)"},
				},
				{
					Name:        "like",
					Description: "Like or unlike a post (toggle)",
					Usage:       "like <post-id>",
				},
				{
					Name:        "boost",
					Description: "Boost or unboost a post (toggle)",
					Usage:       "boost <post-id>",
				},
				{
					Name:        "follow",
					Description: "Follow or unfollow a user (toggle)",
					Usage:       "follow <user@domain> or follow <profile-url>",
					Flags:       []string{"@user: follow local user", "user@domain: follow remote user"},
				},
				{
					Name:        "notifications",
					Description: "Show unread notifications",
					Usage:       "notifications",
				},
				{
					Name:        "clear-notifications",
					Description: "Clear all notifications",
					Usage:       "clear-notifications",
				},
				{
					Name:        "help",
					Description: "Show this help message",
					Usage:       "help",
				},
			},
			GlobalFlags: []string{
				"--json, -j: output in JSON format",
			},
		}
		h.output.JSON(help)
	} else {
		h.output.Println("stegodon CLI - SSH-first fediverse blog")
		h.output.Println("")
		h.output.Println("Usage: ssh -p <port> <server> <command> [options]")
		h.output.Println("")
		h.output.Println("Commands:")
		h.output.Println("  post <message>        Create a new note")
		h.output.Println("  post - [--reply-to]   Read message from stdin")
		h.output.Println("  timeline              Show recent home timeline")
		h.output.Println("  timeline -n <N>       Limit to N posts")
		h.output.Println("  like <id>             Like/unlike a post (toggle)")
		h.output.Println("  boost <id>            Boost/unboost a post (toggle)")
		h.output.Println("  follow <user>         Follow/unfollow a user (toggle)")
		h.output.Println("  notifications         Show unread notifications")
		h.output.Println("  clear-notifications   Clear all notifications")
		h.output.Println("  help                  Show this help message")
		h.output.Println("")
		h.output.Println("Global flags:")
		h.output.Println("  --json, -j            Output in JSON format")
		h.output.Println("")
		h.output.Println("Examples:")
		h.output.Println("  ssh -p 23232 localhost post \"Hello world\"")
		h.output.Println("  ssh -p 23232 localhost post \"Reply\" --reply-to <id>")
		h.output.Println("  ssh -p 23232 localhost timeline -j")
		h.output.Println("  ssh -p 23232 localhost like <id>")
		h.output.Println("  ssh -p 23232 localhost follow user@mastodon.social")
		h.output.Println("  echo \"Hello\" | ssh -p 23232 localhost post -")
	}
	return nil
}
