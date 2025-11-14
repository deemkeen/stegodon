package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

func Router(conf *util.AppConfig) error {
	log.Printf("Starting RSS Feed server on %s:%d", conf.Conf.Host, conf.Conf.HttpPort)
	g := gin.Default()
	g.Use(gzip.Gzip(gzip.DefaultCompression))

	// Global rate limiter: 10 requests per second per IP, burst of 20
	globalLimiter := NewRateLimiter(rate.Limit(10), 20)
	g.Use(RateLimitMiddleware(globalLimiter))

	// Load HTML templates
	g.LoadHTMLGlob("web/templates/*")

	// Web UI routes
	g.GET("/", func(c *gin.Context) {
		HandleIndex(c, conf)
	})

	g.GET("/u/:username", func(c *gin.Context) {
		HandleProfile(c, conf)
	})

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
		// Stricter rate limit for ActivityPub endpoints: 5 req/sec per IP
		apLimiter := NewRateLimiter(rate.Limit(5), 10)

		// Max 1MB request body size for ActivityPub activities
		maxBodySize := MaxBytesMiddleware(1 * 1024 * 1024) // 1MB

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

			c.Header("Content-Type", "application/activity+json; charset=utf-8")
			err, actor := GetActor(c.Param("actor"), conf)
			if err != nil {
				c.Render(404, render.String{Format: actor})
			} else {
				c.Render(200, render.String{Format: actor})
			}
		})

		g.POST("/inbox", RateLimitMiddleware(apLimiter), maxBodySize, func(c *gin.Context) {
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

			// Extract username from activity addressing
			var targetUsername string

			// Helper function to extract username from URI
			extractUsername := func(uri string) string {
				// Check if it's one of our users: https://domain/users/username
				if strings.Contains(uri, conf.Conf.SslDomain) && strings.Contains(uri, "/users/") {
					parts := strings.Split(uri, "/")
					for i, part := range parts {
						if part == "users" && i+1 < len(parts) {
							// Extract just the username, handle /followers suffix
							username := parts[i+1]
							// Remove /followers or /following if present
							if slashIdx := strings.Index(username, "/"); slashIdx > 0 {
								username = username[:slashIdx]
							}
							return username
						}
					}
				}
				return ""
			}

			// Try to find target in "to" field first
			if toArray, ok := activity["to"].([]interface{}); ok {
				for _, to := range toArray {
					if toStr, ok := to.(string); ok {
						if username := extractUsername(toStr); username != "" {
							targetUsername = username
							break
						}
					}
				}
			}

			// If not found, try "cc" field (followers collections)
			if targetUsername == "" {
				if ccArray, ok := activity["cc"].([]interface{}); ok {
					for _, cc := range ccArray {
						if ccStr, ok := cc.(string); ok {
							// Check for followers URI: https://domain/users/username/followers
							if username := extractUsername(ccStr); username != "" {
								targetUsername = username
								break
							}
						}
					}
				}
			}

			// For Follow activities, check the object field
			if targetUsername == "" {
				if objStr, ok := activity["object"].(string); ok {
					targetUsername = extractUsername(objStr)
				}
			}

			if targetUsername == "" {
				// For Create/Update/Delete activities, find which local user(s) follow this actor
				actorURI, _ := activity["actor"].(string)
				if actorURI != "" {
					database := db.GetDB()

					// Get the remote actor
					err, remoteActor := database.ReadRemoteAccountByActorURI(actorURI)
					if err == nil && remoteActor != nil {
						// Find followers of this remote actor (local users who follow them)
						err, followers := database.ReadFollowersByAccountId(remoteActor.Id)
						if err == nil && followers != nil && len(*followers) > 0 {
							// Get the first local user who follows this actor
							firstFollower := (*followers)[0]
							err, localAccount := database.ReadAccById(firstFollower.AccountId)
							if err == nil && localAccount != nil {
								targetUsername = localAccount.Username
								log.Printf("Shared inbox: Routing to follower %s of %s", targetUsername, actorURI)
							}
						} else {
							log.Printf("Shared inbox: No local followers found for %s", actorURI)
						}
					} else {
						log.Printf("Shared inbox: Remote actor %s not found in cache", actorURI)
					}
				}
			}

			if targetUsername == "" {
				log.Printf("Shared inbox: Could not determine target username from activity type %v", activity["type"])
				c.Status(202) // Accept anyway to be nice
				return
			}

			log.Printf("Shared inbox: Routing to user %s", targetUsername)
			// Create a new request with the body
			req := c.Request.Clone(c.Request.Context())
			req.Body = io.NopCloser(bytes.NewReader(body))
			activitypub.HandleInbox(c.Writer, req, targetUsername, conf)
		})

		g.POST("/users/:actor/inbox", RateLimitMiddleware(apLimiter), maxBodySize, func(c *gin.Context) {
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
