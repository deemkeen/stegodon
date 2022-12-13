package main

import (
	"context"
	"fmt"
	"github.com/charmbracelet/wish/logging"
	"github.com/deemkeen/stegodon/middleware"
	"github.com/deemkeen/stegodon/util"
	"github.com/deemkeen/stegodon/web"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/wish"
	"github.com/gliderlabs/ssh"
)

func main() {

	conf, err := util.ReadConf()
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Configuration: ")
	fmt.Println(util.PrettyPrint(conf))

	util.GeneratePemKeypair()

	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", conf.Conf.Host, conf.Conf.SshPort)),
		wish.WithHostKeyPath(".ssh/hostkey"),
		wish.WithPublicKeyAuth(publicKeyHandler),
		//wish.WithAuthorizedKeys(".ssh"),
		wish.WithMiddleware(
			middleware.MainTui(),
			middleware.AuthMiddleware(),
			logging.Middleware(), // last middleware executed first
		),
	)
	if err != nil {
		log.Fatalln(err)
	}

	startServing(err, s, conf)

}

func startServing(err error, s *ssh.Server, conf *util.AppConfig) {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("Starting SSH server on %s:%d", conf.Conf.Host, conf.Conf.SshPort)
	go func() {
		if err = s.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()

	go func() {

		if err = web.Router(conf); err != nil {
			log.Fatalln(err)
		}
	}()

	<-done
	log.Println("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatalln(err)
	}
}

func publicKeyHandler(ssh.Context, ssh.PublicKey) bool {
	return true
}
