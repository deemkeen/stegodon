package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"strings"

	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/assets"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

//go:embed templates/*.html
var embeddedTemplates embed.FS

//go:embed static/style.css
var embeddedCSS []byte

// isActorFromRelay checks if an actor URI matches any relay subscription (exact or domain match)
func isActorFromRelay(actorURI string, database *db.DB) bool {
	// First try exact match
	err, relay := database.ReadRelayByActorURI(actorURI)
	if err == nil && relay != nil {
		return true
	}

	// Fallback: domain-based matching
	actorDomain := extractDomainFromURI(actorURI)
	if actorDomain == "" {
		return false
	}

	err, relays := database.ReadActiveRelays()
	if err != nil || relays == nil {
		return false
	}

	for _, relay := range *relays {
		relayDomain := extractDomainFromURI(relay.ActorURI)
		if relayDomain != "" && relayDomain == actorDomain {
			return true
		}
	}

	return false
}

// extractDomainFromURI extracts the domain (host) from a URI
func extractDomainFromURI(uri string) string {
	if !strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "https://") {
		return ""
	}
	start := strings.Index(uri, "://")
	if start == -1 {
		return ""
	}
	start += 3
	rest := uri[start:]
	end := strings.Index(rest, "/")
	if end == -1 {
		return rest
	}
	return rest[:end]
}

func Router(conf *util.AppConfig) (*gin.Engine, error) {
	log.Printf("Initializing HTTP router on port %d", conf.Conf.HttpPort)

	// Set Gin to use the same log writer as the rest of the application
	gin.DefaultWriter = util.GetLogWriter()
	gin.DefaultErrorWriter = util.GetLogWriter()

	g := gin.New()
	g.Use(gin.Logger(), gin.Recovery())
	g.Use(gzip.Gzip(gzip.DefaultCompression))

	// Load HTML templates from embedded filesystem (must be before routes)
	tmpl, err := template.ParseFS(embeddedTemplates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded templates: %w", err)
	}
	g.SetHTMLTemplate(tmpl)

	// Global rate limiter: 10 requests per second per IP, burst of 20
	globalLimiter := NewRateLimiter(rate.Limit(10), 20)
	g.Use(RateLimitMiddleware(globalLimiter))

	// Web UI routes (skip if SSH-only mode is enabled)
	if !conf.Conf.SshOnly {
		// Serve embedded static assets
		g.GET("/static/stegologo.png", func(c *gin.Context) {
			c.Header("Content-Type", "image/png")
			c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
			c.Data(200, "image/png", assets.StegoLogo)
		})
		g.GET("/static/style.css", func(c *gin.Context) {
			c.Header("Content-Type", "text/css; charset=utf-8")
			c.Data(200, "text/css; charset=utf-8", embeddedCSS)
		})

		g.GET("/", func(c *gin.Context) {
			HandleIndex(c, conf)
		})

		// Global timeline (optional feature)
		if conf.Conf.ShowGlobal {
			g.GET("/global", func(c *gin.Context) {
				HandleGlobalTimeline(c, conf)
			})
		}

		g.GET("/u/:username", func(c *gin.Context) {
			HandleProfile(c, conf)
		})

		g.GET("/u/:username/:noteid", func(c *gin.Context) {
			HandleSinglePost(c, conf)
		})

		// Redirect /@username to /u/username (Mastodon-style URLs)
		g.GET("/@:username", func(c *gin.Context) {
			username := c.Param("username")
			c.Redirect(301, "/u/"+username)
		})

		// Tag page
		g.GET("/tags/:tag", func(c *gin.Context) {
			HandleTagFeed(c, conf)
		})

		// API endpoint for engagement data (local posts by note ID)
		g.GET("/api/engagement/note-id/:noteid/:type", func(c *gin.Context) {
			noteIdStr := c.Param("noteid")
			engagementType := c.Param("type")

			noteId, err := uuid.Parse(noteIdStr)
			if err != nil {
				c.JSON(400, gin.H{"error": "Invalid note ID"})
				return
			}

			database := db.GetDB()
			var users []string

			if engagementType == "likes" {
				users, _ = database.ReadLikersInfoByNoteId(noteId)
			} else if engagementType == "boosts" {
				users, _ = database.ReadBoostersInfoByNoteId(noteId)
			} else {
				c.JSON(400, gin.H{"error": "Invalid engagement type"})
				return
			}

			if users == nil {
				users = []string{}
			}

			c.JSON(200, gin.H{"users": users})
		})

		// API endpoint for engagement data (remote posts by object URI - using query param)
		g.GET("/api/engagement/by-uri/:type", func(c *gin.Context) {
			objectURI := c.Query("uri")
			engagementType := c.Param("type")

			if objectURI == "" {
				c.JSON(400, gin.H{"error": "Missing uri parameter"})
				return
			}

			database := db.GetDB()
			var users []string
			var err error

			if engagementType == "likes" {
				users, err = database.ReadLikersInfoByObjectURI(objectURI)
			} else if engagementType == "boosts" {
				users, err = database.ReadBoostersInfoByObjectURI(objectURI)
			} else {
				c.JSON(400, gin.H{"error": "Invalid engagement type"})
				return
			}

			if err != nil {
				log.Printf("Failed to read engagement info for %s: %v", objectURI, err)
			}

			if users == nil {
				users = []string{}
			}

			c.JSON(200, gin.H{"users": users})
		})

		// Legacy API endpoint for engagement data (backward compatibility with index.html)
		g.GET("/api/engagement/:noteid/:type", func(c *gin.Context) {
			noteIdStr := c.Param("noteid")
			engagementType := c.Param("type")

			noteId, err := uuid.Parse(noteIdStr)
			if err != nil {
				c.JSON(400, gin.H{"error": "Invalid note ID"})
				return
			}

			database := db.GetDB()
			var users []string

			if engagementType == "likes" {
				users, _ = database.ReadLikersInfoByNoteId(noteId)
			} else if engagementType == "boosts" {
				users, _ = database.ReadBoostersInfoByNoteId(noteId)
			} else {
				c.JSON(400, gin.H{"error": "Invalid engagement type"})
				return
			}

			if users == nil {
				users = []string{}
			}

			c.JSON(200, gin.H{"users": users})
		})

		// Avatar upload routes
		g.GET("/upload/:token", func(c *gin.Context) {
			HandleUploadForm(c, conf)
		})
		g.POST("/upload/:token", func(c *gin.Context) {
			HandleUploadSubmit(c, conf)
		})

		// Serve avatar images
		g.GET("/avatars/:filename", func(c *gin.Context) {
			ServeAvatar(c, conf)
		})

		log.Println("Web UI routes enabled")
	} else {
		log.Println("SSH-only mode: Web UI routes disabled")
	}

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
			actorName := c.Param("actor")

			// Content negotiation: redirect browsers to /u/, serve JSON for ActivityPub
			// This fixes Lemmy federation which uses the 'id' field instead of 'url'
			accept := c.GetHeader("Accept")
			if IsHTMLRequest(accept) {
				c.Redirect(302, "/u/"+actorName)
				return
			}

			c.Header("Content-Type", "application/activity+json; charset=utf-8")
			err, actor := GetActor(actorName, conf)
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
			var activity map[string]any
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
			if toArray, ok := activity["to"].([]any); ok {
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
				if ccArray, ok := activity["cc"].([]any); ok {
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

			// Check if this is relay content (activity from an active relay)
			// Relays send to shared inbox, not individual user inboxes
			// Only route as relay content if the actor actually matches a relay subscription
			if targetUsername == "" {
				activityType, _ := activity["type"].(string)
				actorURI, _ := activity["actor"].(string)
				if (activityType == "Create" || activityType == "Announce") && actorURI != "" {
					database := db.GetDB()
					// Check if actor matches a relay subscription (exact or domain match)
					if isActorFromRelay(actorURI, database) {
						// Get any local user to process this (relay content is instance-wide)
						err, accounts := database.ReadAllAccounts()
						if err == nil && accounts != nil && len(*accounts) > 0 {
							targetUsername = (*accounts)[0].Username
							log.Printf("Shared inbox: Routing relay content from %s to %s", actorURI, targetUsername)
						}
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
			actor := c.Param("actor")
			pageStr := c.Query("page")
			page := ParsePageParam(pageStr)

			log.Printf("GET /users/%s/outbox (page=%d)", actor, page)

			err, outbox := GetOutbox(actor, page, conf)
			if err != nil {
				c.Header("Content-Type", "application/activity+json; charset=utf-8")
				c.Render(404, render.String{Format: "{}"})
				return
			}

			c.Header("Content-Type", "application/activity+json; charset=utf-8")
			c.Render(200, render.String{Format: outbox})
		})

		g.GET("/users/:actor/followers", func(c *gin.Context) {
			actor := c.Param("actor")
			page := c.Query("page")
			log.Printf("Get followers for %s (page=%s)", actor, page)
			c.Header("Content-Type", "application/activity+json; charset=utf-8")

			// Get the account
			database := db.GetDB()
			err, account := database.ReadAccByUsername(actor)
			if err != nil {
				log.Printf("Failed to get account %s: %v", actor, err)
				c.Render(404, render.String{Format: "{}"})
				return
			}

			// Get followers
			err, followers := database.ReadFollowersByAccountId(account.Id)
			if err != nil {
				log.Printf("Failed to get followers: %v", err)
				c.Render(200, render.String{Format: GetFollowersCollection(actor, conf, []string{})})
				return
			}

			// Build list of follower URIs
			followerURIs := []string{}
			if followers != nil {
				for _, follower := range *followers {
					// Get the remote account
					err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
					if err == nil && remoteActor != nil {
						followerURIs = append(followerURIs, remoteActor.ActorURI)
						log.Printf("Added remote follower: %s", remoteActor.ActorURI)
					} else {
						// Check if it's a local account
						err, localAcc := database.ReadAccById(follower.AccountId)
						if err == nil && localAcc != nil {
							localURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAcc.Username)
							followerURIs = append(followerURIs, localURI)
							log.Printf("Added local follower: %s", localURI)
						} else {
							log.Printf("Could not find account for follower AccountId=%s", follower.AccountId)
						}
					}
				}
			}

			log.Printf("Returning %d followers for %s", len(followerURIs), actor)

			// If page parameter is present, return a page
			if page != "" {
				c.Render(200, render.String{Format: GetFollowersPage(actor, conf, followerURIs, 1)})
			} else {
				// Return the collection with first link
				c.Render(200, render.String{Format: GetFollowersCollection(actor, conf, followerURIs)})
			}
		})

		g.GET("/users/:actor/following", func(c *gin.Context) {
			actor := c.Param("actor")
			page := c.Query("page")
			log.Printf("Get following for %s (page=%s)", actor, page)
			c.Header("Content-Type", "application/activity+json; charset=utf-8")

			// Get the account
			database := db.GetDB()
			err, account := database.ReadAccByUsername(actor)
			if err != nil {
				log.Printf("Failed to get account %s: %v", actor, err)
				c.Render(404, render.String{Format: "{}"})
				return
			}

			// Get following
			err, following := database.ReadFollowingByAccountId(account.Id)
			if err != nil {
				log.Printf("Failed to get following: %v", err)
				c.Render(200, render.String{Format: GetFollowingCollection(actor, conf, []string{})})
				return
			}

			// Build list of following URIs
			followingURIs := []string{}
			if following != nil {
				for _, follow := range *following {
					// Get the remote account
					err, remoteActor := database.ReadRemoteAccountById(follow.TargetAccountId)
					if err == nil && remoteActor != nil {
						followingURIs = append(followingURIs, remoteActor.ActorURI)
						log.Printf("Added remote following: %s", remoteActor.ActorURI)
					} else {
						// Check if it's a local account
						err, localAcc := database.ReadAccById(follow.TargetAccountId)
						if err == nil && localAcc != nil {
							localURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAcc.Username)
							followingURIs = append(followingURIs, localURI)
							log.Printf("Added local following: %s", localURI)
						} else {
							log.Printf("Could not find account for following TargetAccountId=%s", follow.TargetAccountId)
						}
					}
				}
			}

			log.Printf("Returning %d following for %s", len(followingURIs), actor)

			// If page parameter is present, return a page
			if page != "" {
				c.Render(200, render.String{Format: GetFollowingPage(actor, conf, followingURIs, 1)})
			} else {
				// Return the collection with first link
				c.Render(200, render.String{Format: GetFollowingCollection(actor, conf, followingURIs)})
			}
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

		// NodeInfo endpoints for server discovery and statistics
		g.GET("/.well-known/nodeinfo", func(c *gin.Context) {
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.Render(200, render.String{Format: GetWellKnownNodeInfo(conf)})
		})

		g.GET("/nodeinfo/2.0", func(c *gin.Context) {
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.Render(200, render.String{Format: GetNodeInfo20(conf)})
		})

		g.GET("/nodeinfo/2.1", func(c *gin.Context) {
			c.Header("Content-Type", "application/json; profile=\"http://nodeinfo.diaspora.software/ns/schema/2.1#\"; charset=utf-8")
			c.Render(200, render.String{Format: GetNodeInfo21(conf)})
		})

	}
	return g, nil
}
