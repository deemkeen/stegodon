package writenote

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"log"
)

const MaxLetters = 150

type Model struct {
	Textarea    textarea.Model
	Err         util.ErrMsg
	userId      uuid.UUID
	lettersLeft int
	width       int
}

func InitialNote(contentWidth int, userId uuid.UUID) Model {
	width := common.DefaultCreateNoteWidth(contentWidth)
	ti := textarea.New()
	ti.Placeholder = "enter your message"
	ti.CharLimit = MaxLetters
	ti.ShowLineNumbers = false
	ti.SetWidth(30)

	return Model{
		Textarea:    ti,
		Err:         nil,
		userId:      userId,
		lettersLeft: MaxLetters,
		width:       width,
	}
}

func createNoteModelCmd(note *domain.SaveNote) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Create note in database
		err := database.CreateNote(note.UserId, note.Message)
		if err != nil {
			log.Println("Note could not be saved!")
			return common.UpdateNoteList
		}

		// Federate the note via ActivityPub (background task)
		go func() {
			// Get the account
			err, account := database.ReadAccById(note.UserId)
			if err != nil {
				log.Printf("Failed to get account for federation: %v", err)
				return
			}

			// Get config
			conf, err := util.ReadConf()
			if err != nil {
				log.Printf("Failed to read config for federation: %v", err)
				return
			}

			// Only federate if ActivityPub is enabled
			if !conf.Conf.WithAp {
				return
			}

			// Create domain Note from SaveNote
			domainNote := &domain.Note{
				Id:        uuid.New(),
				CreatedBy: account.Username,
				Message:   note.Message,
				CreatedAt: account.CreatedAt, // Will be overridden by actual creation time
			}

			// Send Create activity to all followers
			if err := activitypub.SendCreate(domainNote, account, conf); err != nil {
				log.Printf("Failed to federate note: %v", err)
			} else {
				log.Printf("Note federated successfully for %s", account.Username)
			}
		}()

		return common.UpdateNoteList
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlA:
			if m.Textarea.Focused() {
				m.Textarea.Blur()
			}
		case tea.KeyCtrlS:
			value := util.NormalizeInput(m.Textarea.Value())
			note := domain.SaveNote{
				UserId:  m.userId,
				Message: value,
			}
			m.Textarea.SetValue("")
			return m, createNoteModelCmd(&note)
		case tea.KeyCtrlC:
			return m, tea.Quit
		default:
			if !m.Textarea.Focused() {
				cmd = m.Textarea.Focus()
				cmds = append(cmds, cmd)
			}
		}

	// We handle errors just like any other message
	case util.ErrMsg:
		m.Err = msg
		return m, nil
	}

	m.Textarea, cmd = m.Textarea.Update(msg)
	m.lettersLeft = m.CharCount()
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) CharCount() int {
	return m.Textarea.CharLimit - m.Textarea.Length() + m.Textarea.LineCount() - 1
}

func (m Model) View() string {
	styledTextarea := lipgloss.NewStyle().PaddingLeft(5).PaddingRight(5).Margin(2).Render(m.Textarea.View())
	charsLeft := common.HelpStyle.PaddingLeft(7).Render(fmt.Sprintf("characters left: %d\n\npost message: ctrl+s",
		m.lettersLeft))
	caption := common.CaptionStyle.PaddingLeft(7).Render("new note")

	return fmt.Sprintf("%s\n\n%s\n\n%s", caption, styledTextarea, charsLeft)
}
