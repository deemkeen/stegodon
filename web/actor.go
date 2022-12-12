package web

import (
	"fmt"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"strings"
)

type action uint

const (
	id action = iota
	inbox
	outbox
	followers
	following
	sharedInbox
)

func GetActor(actor string, conf *util.AppConfig) (error, string) {
	err, acc := db.GetDB().ReadAccByUsername(actor)
	if err != nil {
		return err, "{}"
	}

	username := acc.Username
	pubKey := strings.Replace(acc.WebPublicKey, "\n", "\\n", -1)
	return nil, fmt.Sprintf(
		`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],

					"id": "%s",
					"type": "Person",
					"preferredUsername": "%s",
					"name" : "%s",
					"inbox": "%s",
					"outbox": "%s",
					"followers": "%s",
					"following": "%s",
					"url": "%s",
  					"manuallyApprovesFollowers": false,
					"discoverable": true,
  					"endpoints": {
    					"sharedInbox": "%s"
  					},
					"publicKey": {
						"id": "%s#main-key",
						"owner": "%s",
						"publicKeyPem": "%s"
					}
				}`,
		getIRI(conf.Conf.SslDomain, username, id),
		username, username, getIRI(conf.Conf.SslDomain, username, inbox),
		getIRI(conf.Conf.SslDomain, username, outbox),
		getIRI(conf.Conf.SslDomain, username, followers),
		getIRI(conf.Conf.SslDomain, username, following),
		getIRI(conf.Conf.SslDomain, username, id),
		getIRI(conf.Conf.SslDomain, username, sharedInbox),
		getIRI(conf.Conf.SslDomain, username, id),
		getIRI(conf.Conf.SslDomain, username, id), pubKey)
}

func getIRI(domain string, username string, action action) string {

	prefix := fmt.Sprintf("https://%s/users/%s", domain, username)
	switch action {
	case inbox:
		return fmt.Sprintf("/%s/inbox", prefix)
	case outbox:
		return fmt.Sprintf("/%s/outbox", prefix)
	case followers:
		return fmt.Sprintf("%s/followers", prefix)
	case following:
		return fmt.Sprintf("%s/following", prefix)
	case id:
		return prefix
	case sharedInbox:
		return fmt.Sprintf("https://%s/inbox", domain)
	default:
		return ""
	}
}
