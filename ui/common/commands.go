package common

type SessionState uint

const (
	CreateNoteView SessionState = iota
	ListNotesView
	CreateUserView
	UpdateNoteList
)
