package web

import (
	"fmt"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
)

func GetWebfinger(user string, conf *util.AppConfig) (error, string) {

	err, acc := db.GetDB().ReadAccByUsername(user)
	if err != nil {
		return err, GetWebFingerNotFound()
	}

	username := acc.Username

	return nil, fmt.Sprintf(
		`{
					"subject": "acct:%s@%s",

					"links": [
						{
							"rel": "self",
							"type": "application/activity+json",
							"href": "https://%s/users/%s"
						}
					]
				}`, username, conf.Conf.SslDomain,
		conf.Conf.SslDomain, username)
}

func GetWebFingerNotFound() string {
	return `{"detail":"Not Found"}`
}
