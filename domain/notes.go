package domain

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type SaveNote struct {
	UserId  uuid.UUID
	Message string
}

type Note struct {
	Id        uuid.UUID
	CreatedBy string
	Message   string
	CreatedAt time.Time
}

func (note *Note) ToString() string {
	return fmt.Sprintf("\n\tId: %s \n\tCreatedBy: %s \n\tMessage: %s \n\tCreatedAt: %s)", note.Id, note.CreatedBy, note.Message, note.CreatedAt)
}
