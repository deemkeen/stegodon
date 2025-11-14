package web

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"github.com/gorilla/feeds"
)

func GetRSS(conf *util.AppConfig, username string) (string, error) {

	var err error
	var notes *[]domain.Note
	var title string
	var createdBy string
	var email string

	link := fmt.Sprintf("http://%s:%d/feed", conf.Conf.Host, conf.Conf.HttpPort)

	if username != "" {
		err, notes = db.GetDB().ReadNotesByUsername(username)
		if err != nil {
			log.Println(fmt.Sprintf("Could not get notes from %s!", username), err)
			return "", errors.New("error retrieving notes by username")
		}
		title = fmt.Sprintf("Stegodon Notes - %s", username)
		createdBy = username
		email = fmt.Sprintf("%s@stegodon", username)
		link = fmt.Sprintf("%s?username=%s", link, username)
		// If notes exist, use the actual createdBy from first note
		if notes != nil && len(*notes) > 0 {
			createdBy = (*notes)[0].CreatedBy
			email = fmt.Sprintf("%s@stegodon", (*notes)[0].CreatedBy)
		}
	} else {
		err, notes = db.GetDB().ReadAllNotes()
		if err != nil {
			log.Println("Could not get notes!", err)
			return "", errors.New("error retrieving notes")
		}
		title = "All Stegodon Notes"
		createdBy = "everyone"
		email = fmt.Sprintf("%s@stegodon", createdBy)
	}

	feed := &feeds.Feed{
		Title:       title,
		Link:        &feeds.Link{Href: link},
		Description: "rss feed for testing stegodon",
		Author:      &feeds.Author{Name: createdBy, Email: email},
		Created:     time.Now(),
	}

	var feedItems []*feeds.Item
	if notes != nil {
		for _, note := range *notes {
			email := fmt.Sprintf("%s@stegodon", note.CreatedBy)
			feedItems = append(feedItems,
				&feeds.Item{
					Id:      note.Id.String(),
					Title:   note.CreatedAt.Format(util.DateTimeFormat()),
					Link:    &feeds.Link{Href: fmt.Sprintf("http://%s:%d/feed/%s", conf.Conf.Host, conf.Conf.HttpPort, note.Id)},
					Content: note.Message,
					Author:  &feeds.Author{Name: note.CreatedBy, Email: email},
					Created: note.CreatedAt,
				})
		}
	}

	feed.Items = feedItems
	return feed.ToRss()
}

func GetRSSItem(conf *util.AppConfig, id uuid.UUID) (string, error) {
	err, note := db.GetDB().ReadNoteId(id)

	if err != nil || note == nil {
		log.Println("Could not get note!", err)
		return "", errors.New("error retrieving note by id")
	}

	email := fmt.Sprintf("%s@stegodon", note.CreatedBy)
	url := fmt.Sprintf("http://%s:%d/feed/%s", conf.Conf.Host, conf.Conf.HttpPort, note.Id)

	feed := &feeds.Feed{
		Title:       "Single Stegodon Note",
		Link:        &feeds.Link{Href: url},
		Description: "rss feed for testing stegodon",
		Author:      &feeds.Author{Name: note.CreatedBy, Email: email},
		Created:     time.Now(),
	}

	var feedItems []*feeds.Item

	feedItems = append(feedItems,
		&feeds.Item{
			Id:      note.Id.String(),
			Title:   note.CreatedAt.Format(util.DateTimeFormat()),
			Link:    &feeds.Link{Href: url},
			Content: note.Message,
			Author:  &feeds.Author{Name: note.CreatedBy, Email: email},
			Created: note.CreatedAt,
		})

	feed.Items = feedItems
	return feed.ToRss()
}
