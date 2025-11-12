package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/util"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/google/uuid"
	"log"
	"strings"
)

func Router(conf *util.AppConfig) error {
	log.Printf("Starting RSS Feed server on %s:%d", conf.Conf.Host, conf.Conf.HttpPort)
	g := gin.Default()
	g.Use(gzip.Gzip(gzip.DefaultCompression))

	// RSS Feed
	g.GET("/feed", func(c *gin.Context) {

		c.Header("Content-Type", "application/xml; charset=utf-8")

		username := c.Query("username")
		rss, err := GetRSS(conf, username)
		if err != nil {
			c.Render(404, render.String{Format: ""})
		} else {
			c.Render(200, render.String{Format: rss})
		}
	})

	g.GET("/feed/:id", func(c *gin.Context) {
		c.Header("Content-Type", "application/xml; charset=utf-8")
		name := c.Param("id")
		feedId, err := uuid.Parse(name)
		if err != nil {
			c.Render(404, render.String{Format: ""})
			return
		}

		rssItem, err := GetRSSItem(conf, feedId)
		if err != nil {
			c.Render(404, render.String{Format: ""})
		} else {
			c.Render(200, render.String{Format: rssItem})
		}
	})

	// Endpoints for the ActivityPub functionality, WIP!
	if conf.Conf.WithAp {
		// Serve individual notes as ActivityPub objects
		g.GET("/notes/:id", func(c *gin.Context) {
			c.Header("Content-Type", "application/activity+json; charset=utf-8")

			noteIdStr := c.Param("id")
			noteId, err := uuid.Parse(noteIdStr)
			if err != nil {
				c.JSON(404, gin.H{"error": "Invalid note ID"})
				return
			}

			err, note := GetNoteObject(noteId, conf)
			if err != nil {
				c.JSON(404, gin.H{"error": "Note not found"})
			} else {
				c.Render(200, render.String{Format: note})
			}
		})

		g.GET("/users/:actor", func(c *gin.Context) {

			data, err := c.GetRawData()
			if err != nil {
				return
			}

			log.Println("RAW REQUEST #####")
			log.Println(string(data))
			log.Println("END RAW REQUEST #####")

			c.Header("Content-Type", "application/activity+json; charset=utf-8")
			err, actor := GetActor(c.Param("actor"), conf)
			if err != nil {
				c.Render(404, render.String{Format: actor})
			} else {
				c.Render(200, render.String{Format: actor})
			}
		})

		g.POST("/inbox", func(c *gin.Context) {
			log.Println("POST /inbox (shared inbox)")
			// Shared inbox - extract target username from activity object
			body, err := c.GetRawData()
			if err != nil {
				log.Printf("Shared inbox: Failed to read body: %v", err)
				c.Status(400)
				return
			}

			// Parse to get target actor
			var activity map[string]interface{}
			if err := json.Unmarshal(body, &activity); err != nil {
				log.Printf("Shared inbox: Failed to parse activity: %v", err)
				c.Status(400)
				return
			}

			// Extract username from object (for Create) or object field (for others)
			var targetUsername string
			if _, ok := activity["object"].(map[string]interface{}); ok {
				// For Create activities, object might have "to" or "cc" fields
				// Try to extract from the first "to" address
				if toArray, ok := activity["to"].([]interface{}); ok && len(toArray) > 0 {
					if toStr, ok := toArray[0].(string); ok {
						// Extract username from "https://domain/users/username"
						parts := strings.Split(toStr, "/")
						if len(parts) > 0 {
							targetUsername = parts[len(parts)-1]
						}
					}
				}
			} else if objStr, ok := activity["object"].(string); ok {
				// For Follow/Accept/etc, object is actor URI
				parts := strings.Split(objStr, "/")
				if len(parts) > 0 {
					targetUsername = parts[len(parts)-1]
				}
			}

			if targetUsername == "" {
				log.Printf("Shared inbox: Could not determine target username from activity: %v", activity)
				c.Status(202) // Accept anyway to be nice
				return
			}

			log.Printf("Shared inbox: Routing to user %s", targetUsername)
			// Create a new request with the body
			req := c.Request.Clone(c.Request.Context())
			req.Body = io.NopCloser(bytes.NewReader(body))
			activitypub.HandleInbox(c.Writer, req, targetUsername, conf)
		})

		g.POST("/users/:actor/inbox", func(c *gin.Context) {
			actor := c.Param("actor")
			log.Printf("POST /users/%s/inbox", actor)
			activitypub.HandleInbox(c.Writer, c.Request, actor, conf)
		})

		g.GET("/users/:actor/outbox", func(c *gin.Context) {
			log.Println("Get outbox..")
			c.Header("Content-Type", "application/activity+json; charset=utf-8")
			c.Render(200, render.String{Format: "{}"})
		})

		g.GET("/users/:actor/followers", func(c *gin.Context) {
			log.Println("Get followers..")
			c.Header("Content-Type", "application/activity+json; charset=utf-8")
			c.Render(200, render.String{Format: "{}"})
		})

		g.GET("/users/:actor/following", func(c *gin.Context) {
			log.Println("Get followers..")
			c.Header("Content-Type", "application/activity+json; charset=utf-8")
			c.Render(200, render.String{Format: "{}"})
		})

		g.GET("/.well-known/webfinger", func(c *gin.Context) {
			c.Header("Content-Type", "application/json; charset=utf-8")

			resource := c.Query("resource")
			if resource == "" || !strings.HasPrefix(resource, "acct:") {
				c.Render(404, render.String{Format: GetWebFingerNotFound()})
			} else {
				resource = strings.TrimPrefix(resource, "acct:")
				resource = strings.TrimSuffix(resource, fmt.Sprintf("@%s", conf.Conf.SslDomain))
				err, resp := GetWebfinger(resource, conf)
				if err != nil {
					c.Render(404, render.String{Format: GetWebFingerNotFound()})
				} else {
					c.Render(200, render.String{Format: resp})
				}
			}
		})

	}
	err := g.Run(fmt.Sprintf(":%d", conf.Conf.HttpPort))
	if err != nil {
		return err
	}
	return nil
}
