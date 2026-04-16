package relay

import (
	"fmt"
	"log"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

type Model struct {
	AdminId   uuid.UUID
	AdminAcct *domain.Account
	Config    *util.AppConfig
	Relays    []domain.Relay
	Selected  int
	Offset    int // Pagination offset
	Width     int
	Height    int
	Status    string
	Error     string
	Input     textinput.Model // For entering relay URL
	Adding    bool            // Input mode for adding relay
}

func InitialModel(adminId uuid.UUID, adminAcct *domain.Account, config *util.AppConfig, width, height int) Model {
	ti := textinput.New()
	ti.Placeholder = "relay.example.com or https://relay.example.com/actor"
	ti.CharLimit = 256
	ti.SetWidth(60)

	return Model{
		AdminId:   adminId,
		AdminAcct: adminAcct,
		Config:    config,
		Relays:    []domain.Relay{},
		Selected:  0,
		Offset:    0,
		Width:     width,
		Height:    height,
		Status:    "",
		Error:     "",
		Input:     ti,
		Adding:    false,
	}
}

func (m Model) Init() tea.Cmd {
	return loadRelays()
}

// Messages

type relaysLoadedMsg struct {
	relays []domain.Relay
}

type relayAddedMsg struct {
	err error
}

type relayDeletedMsg struct {
	id  uuid.UUID
	err error
}

type relayRetryMsg struct {
	err error
}

type relayPausedMsg struct {
	id     uuid.UUID
	paused bool
	err    error
}

type relayContentDeletedMsg struct {
	count int64
	err   error
}

// Commands

func loadRelays() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, relays := database.ReadAllRelays()
		if err != nil {
			log.Printf("Relay panel: Failed to load relays: %v", err)
			return relaysLoadedMsg{relays: []domain.Relay{}}
		}
		if relays == nil {
			return relaysLoadedMsg{relays: []domain.Relay{}}
		}
		return relaysLoadedMsg{relays: *relays}
	}
}

func subscribeToRelay(adminAcct *domain.Account, relayURL string, config *util.AppConfig) tea.Cmd {
	return func() tea.Msg {
		// Normalize relay URL to actor URI
		actorURI := normalizeRelayURL(relayURL)

		err := activitypub.SendRelayFollow(adminAcct, actorURI, config)
		if err != nil {
			log.Printf("Relay panel: Failed to subscribe to relay %s: %v", actorURI, err)
			return relayAddedMsg{err: err}
		}
		return relayAddedMsg{err: nil}
	}
}

func unsubscribeFromRelay(adminAcct *domain.Account, relay *domain.Relay, config *util.AppConfig) tea.Cmd {
	return func() tea.Msg {
		// Send Undo Follow to relay
		err := activitypub.SendRelayUnfollow(adminAcct, relay, config)
		if err != nil {
			log.Printf("Relay panel: Failed to send unsubscribe to relay %s: %v", relay.ActorURI, err)
			// Still delete locally even if remote fails
		}

		// Delete from database
		database := db.GetDB()
		if err := database.DeleteRelay(relay.Id); err != nil {
			log.Printf("Relay panel: Failed to delete relay %s: %v", relay.Id, err)
			return relayDeletedMsg{id: relay.Id, err: err}
		}

		return relayDeletedMsg{id: relay.Id, err: nil}
	}
}

func retryRelay(adminAcct *domain.Account, relay *domain.Relay, config *util.AppConfig) tea.Cmd {
	return func() tea.Msg {
		// Delete old record and resubscribe
		database := db.GetDB()
		database.DeleteRelay(relay.Id)

		err := activitypub.SendRelayFollow(adminAcct, relay.ActorURI, config)
		if err != nil {
			log.Printf("Relay panel: Failed to retry relay %s: %v", relay.ActorURI, err)
			return relayRetryMsg{err: err}
		}
		return relayRetryMsg{err: nil}
	}
}

func toggleRelayPause(relayId uuid.UUID, paused bool) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.UpdateRelayPaused(relayId, paused)
		if err != nil {
			log.Printf("Relay panel: Failed to update relay pause status: %v", err)
			return relayPausedMsg{id: relayId, paused: paused, err: err}
		}
		return relayPausedMsg{id: relayId, paused: paused, err: nil}
	}
}

func deleteRelayContent() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		count, err := database.DeleteRelayActivities()
		if err != nil {
			log.Printf("Relay panel: Failed to delete relay content: %v", err)
			return relayContentDeletedMsg{count: 0, err: err}
		}
		return relayContentDeletedMsg{count: count, err: nil}
	}
}

// normalizeRelayURL converts a relay URL to a full actor URI
func normalizeRelayURL(input string) string {
	input = strings.TrimSpace(input)

	// If it already looks like a full URI, use it
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		return input
	}

	// Otherwise, assume it's a domain and construct the actor URI
	// Most relays use /actor as the actor endpoint
	return fmt.Sprintf("https://%s/actor", input)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case relaysLoadedMsg:
		m.Relays = msg.relays
		m.Selected = 0
		m.Offset = 0
		if m.Selected >= len(m.Relays) {
			m.Selected = max(0, len(m.Relays)-1)
		}
		return m, nil

	case relayAddedMsg:
		m.Adding = false
		m.Input.Blur()
		m.Input.SetValue("")
		if msg.err != nil {
			m.Error = msg.err.Error()
			m.Status = ""
		} else {
			m.Status = "Subscription request sent (pending acceptance)"
			m.Error = ""
		}
		return m, loadRelays()

	case relayDeletedMsg:
		if msg.err != nil {
			m.Error = msg.err.Error()
			m.Status = ""
		} else {
			m.Status = "Relay unsubscribed"
			m.Error = ""
		}
		return m, loadRelays()

	case relayRetryMsg:
		if msg.err != nil {
			m.Error = msg.err.Error()
			m.Status = ""
		} else {
			m.Status = "Retry sent (pending acceptance)"
			m.Error = ""
		}
		return m, loadRelays()

	case relayPausedMsg:
		if msg.err != nil {
			m.Error = msg.err.Error()
			m.Status = ""
		} else {
			if msg.paused {
				m.Status = "Relay paused"
			} else {
				m.Status = "Relay resumed"
			}
			m.Error = ""
		}
		return m, loadRelays()

	case relayContentDeletedMsg:
		if msg.err != nil {
			m.Error = msg.err.Error()
			m.Status = ""
		} else {
			m.Status = fmt.Sprintf("Deleted %d relay activities", msg.count)
			m.Error = ""
		}
		return m, nil

	case tea.KeyPressMsg:
		// In adding mode, handle input
		if m.Adding {
			switch msg.String() {
			case "esc":
				m.Adding = false
				m.Input.Blur()
				m.Input.SetValue("")
				m.Error = ""
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.Input.Value())
				if value == "" {
					m.Error = "Please enter a relay URL"
					return m, nil
				}
				m.Status = "Subscribing..."
				m.Error = ""
				return m, subscribeToRelay(m.AdminAcct, value, m.Config)
			default:
				m.Input, cmd = m.Input.Update(msg)
				return m, cmd
			}
		}

		// Normal mode
		m.Status = ""
		m.Error = ""

		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				// Scroll up if needed
				if m.Selected < m.Offset {
					m.Offset = m.Selected
				}
			}
		case "down", "j":
			if len(m.Relays) > 0 && m.Selected < len(m.Relays)-1 {
				m.Selected++
				// Scroll down if needed
				if m.Selected >= m.Offset+common.DefaultItemsPerPage {
					m.Offset = m.Selected - common.DefaultItemsPerPage + 1
				}
			}
		case "a":
			// Add relay mode
			m.Adding = true
			m.Input.Focus()
			return m, textinput.Blink
		case "d":
			// Delete/unsubscribe from selected relay
			if len(m.Relays) > 0 && m.Selected < len(m.Relays) {
				selectedRelay := m.Relays[m.Selected]
				m.Status = "Unsubscribing..."
				return m, unsubscribeFromRelay(m.AdminAcct, &selectedRelay, m.Config)
			}
		case "r":
			// Retry failed relay subscription
			if len(m.Relays) > 0 && m.Selected < len(m.Relays) {
				selectedRelay := m.Relays[m.Selected]
				if selectedRelay.Status == "failed" {
					m.Status = "Retrying..."
					return m, retryRelay(m.AdminAcct, &selectedRelay, m.Config)
				} else {
					m.Error = "Only failed relays can be retried"
				}
			}
		case "p":
			// Pause/resume selected relay
			if len(m.Relays) > 0 && m.Selected < len(m.Relays) {
				selectedRelay := m.Relays[m.Selected]
				if selectedRelay.Status == "active" {
					newPaused := !selectedRelay.Paused
					if newPaused {
						m.Status = "Pausing relay..."
					} else {
						m.Status = "Resuming relay..."
					}
					return m, toggleRelayPause(selectedRelay.Id, newPaused)
				} else {
					m.Error = "Only active relays can be paused/resumed"
				}
			}
		case "x":
			// Delete all relay content from timeline
			m.Status = "Deleting relay content..."
			return m, deleteRelayContent()
		}
	}

	return m, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) View() tea.View {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("relay management (%d relays)", len(m.Relays))))
	s.WriteString("\n\n")

	// Input mode for adding relay
	if m.Adding {
		s.WriteString("Enter relay URL:\n")
		s.WriteString(m.Input.View())
		s.WriteString("\n\n")
		s.WriteString(common.HelpStyle.Render("enter: subscribe | esc: cancel"))
		return tea.NewView(s.String())
	}

	if len(m.Relays) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No relays configured."))
		s.WriteString("\n\n")
		s.WriteString(common.HelpStyle.Render("Press 'a' to add a relay"))
	} else {
		start := m.Offset
		end := min(start+common.DefaultItemsPerPage, len(m.Relays))

		for i := start; i < end; i++ {
			relay := m.Relays[i]

			// Extract domain from actor URI for display
			displayName := relay.ActorURI
			if relay.Name != "" {
				displayName = relay.Name
			} else {
				// Try to extract domain from URI
				displayName = extractDomain(relay.ActorURI)
			}

			// Status badge
			var statusBadge string
			switch relay.Status {
			case "active":
				if relay.Paused {
					statusBadge = common.ListBadgeMutedStyle.Render("[paused]")
				} else {
					statusBadge = common.ListBadgeStyle.Render("[active]")
				}
			case "pending":
				statusBadge = common.ListBadgeMutedStyle.Render("[pending]")
			case "failed":
				statusBadge = common.ListErrorStyle.Render("[failed]")
			default:
				statusBadge = common.ListBadgeMutedStyle.Render("[" + relay.Status + "]")
			}

			if i == m.Selected {
				// Selected item with arrow prefix
				text := common.ListItemSelectedStyle.Render(displayName + " " + statusBadge)
				s.WriteString(common.ListSelectedPrefix + text)
			} else {
				// Normal item
				text := displayName + " " + statusBadge
				s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
			}
			s.WriteString("\n")
		}

		// Show pagination info if there are more items
		if len(m.Relays) > common.DefaultItemsPerPage {
			s.WriteString("\n")
			paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.Relays))
			s.WriteString(common.ListBadgeStyle.Render(paginationText))
		}
	}

	s.WriteString("\n")

	if m.Status != "" {
		s.WriteString(common.ListStatusStyle.Render(m.Status))
		s.WriteString("\n")
	}

	if m.Error != "" {
		s.WriteString(common.ListErrorStyle.Render("Error: " + m.Error))
		s.WriteString("\n")
	}

	// Footer with available keys
	s.WriteString("\n")
	s.WriteString(common.HelpStyle.Render("keys: a add | d delete | p pause/resume | r retry | x clear content"))

	return tea.NewView(s.String())
}

// extractDomain extracts the domain from a URL
func extractDomain(uri string) string {
	// Remove protocol prefix
	uri = strings.TrimPrefix(uri, "https://")
	uri = strings.TrimPrefix(uri, "http://")

	// Extract domain (before first /)
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return uri
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
