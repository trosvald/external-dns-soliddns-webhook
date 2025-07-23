package main

import (
	"fmt"
	"github.com/trosvald/external-dns-soliddns-webhook/cmd/webhook/init/configuration"
	"github.com/trosvald/external-dns-soliddns-webhook/cmd/webhook/init/dnsprovider"
	"github.com/trosvald/external-dns-soliddns-webhook/cmd/webhook/init/logging"
	"github.com/trosvald/external-dns-soliddns-webhook/cmd/webhook/init/server"

	log "github.com/sirupsen/logrus"
)

const banner = `
     ______________ 
    / ____/  _/ __ \
   / __/  / // /_/ /
  / /____/ // ____/ 
 /_____/___/_/      
 external-dns-soliddns-webhook
 version: %s (%s)
`

var (
	Version = "local"
	Gitsha  = "?"
)

func main() {
	fmt.Printf(banner, Version, Gitsha)

	logging.Init()

	config := configuration.Init()
	provider, err := dnsprovider.Init(config)
	if err != nil {
		log.Fatalf("[ERROR] Failed to initialized provider: %v", err)
	}
	srv := server.NewServer()

	srv.StartHealth(config)
	srv.Start(config, provider)
}
