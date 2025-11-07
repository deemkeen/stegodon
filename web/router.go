package web

import (
	"fmt"
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
			log.Println("Posted to shared inbox..")
			c.Header("Content-Type", "application/activity+json; charset=utf-8")
			c.Render(200, render.String{Format: ""})
		})

		g.POST("/users/:actor/inbox", func(c *gin.Context) {
			actor := c.Param("actor")
			log.Printf("POST /users/%s/inbox", actor)
			activitypub.HandleInbox(c.Writer, c.Request, actor)
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
			c.Header("Content-Type", "application/json; charset=utf-8")
		})

	}
	err := g.Run(fmt.Sprintf(":%d", conf.Conf.HttpPort))
	if err != nil {
		return err
	}
	return nil
}
