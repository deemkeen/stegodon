package web

import (
	"fmt"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"github.com/gorilla/feeds"
	"log"
	"time"
)

func GetRSS(conf *util.AppConfig) (string, error) {
	err, notes := db.GetDB().ReadAllNotes()
	if err != nil {
		log.Fatalln("Could not get notes!", err)
		return "", err
	}

	feed := &feeds.Feed{
		Title:       "All Stegodon Notes",
		Link:        &feeds.Link{Href: fmt.Sprintf("http://%s:%d/feed", conf.Conf.Host, conf.Conf.HttpPort)},
		Description: "rss feed for testing stegodon",
		Author:      &feeds.Author{Name: "nobody"},
		Created:     time.Now(),
	}

	var feedItems []*feeds.Item
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

	feed.Items = feedItems
	return feed.ToRss()
}

func GetRSSItem(id uuid.UUID) (string, error) {
	err, note := db.GetDB().ReadNoteId(id)
	if err != nil {
		log.Fatalln("Could not get notes!", err)
		return "", err
	}

	feed := &feeds.Feed{
		Title:       "All Stegodon Notes",
		Link:        &feeds.Link{Href: "https://"},
		Description: "rss feed for testing stegodon",
		Author:      &feeds.Author{Name: "nobody"},
		Created:     time.Now(),
	}

	var feedItems []*feeds.Item

	email := fmt.Sprintf("%s@stegodon", note.CreatedBy)
	feedItems = append(feedItems,
		&feeds.Item{
			Id:      note.Id.String(),
			Title:   note.CreatedAt.Format(util.DateTimeFormat()),
			Link:    &feeds.Link{Href: "https://"},
			Content: note.Message,
			Author:  &feeds.Author{Name: note.CreatedBy, Email: email},
			Created: note.CreatedAt,
		})

	feed.Items = feedItems
	return feed.ToRss()
}
