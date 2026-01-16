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
	ReadHomeTimelinePosts(accountId interface{}, limit int) (error, *[]domain.HomePost)
	ReadNotificationsByAccountId(accountId interface{}, limit int) (error, *[]domain.Notification)
	CountUnreadNotifications(accountId interface{}) (int, error)
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
	case "notifications":
		return h.handleNotifications(cmdArgs)
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
					Usage:       "post <message> or post -",
					Flags:       []string{"-: read message from stdin"},
				},
				{
					Name:        "timeline",
					Description: "Show recent home timeline",
					Usage:       "timeline [-n <count>]",
					Flags:       []string{"-n <count>: limit number of posts (default 20)"},
				},
				{
					Name:        "notifications",
					Description: "Show unread notifications",
					Usage:       "notifications",
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
		h.output.Println("  post <message>      Create a new note")
		h.output.Println("  post -              Read message from stdin")
		h.output.Println("  timeline            Show recent home timeline")
		h.output.Println("  timeline -n <N>     Limit to N posts")
		h.output.Println("  notifications       Show unread notifications")
		h.output.Println("  help                Show this help message")
		h.output.Println("")
		h.output.Println("Global flags:")
		h.output.Println("  --json, -j          Output in JSON format")
		h.output.Println("")
		h.output.Println("Examples:")
		h.output.Println("  ssh -p 23232 localhost post \"Hello world\"")
		h.output.Println("  ssh -p 23232 localhost timeline -j")
		h.output.Println("  echo \"Hello\" | ssh -p 23232 localhost post -")
	}
	return nil
}
