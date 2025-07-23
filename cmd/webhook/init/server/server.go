package server

import (
	"fmt"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/trosvald/external-dns-soliddns-webhook/cmd/webhook/init/configuration"
	"sigs.k8s.io/external-dns/provider"
	"sigs.k8s.io/external-dns/provider/webhook/api"
)

type WebhookServer struct {
	Ready   bool
	Channel chan struct{}
}

func NewServer() *WebhookServer {
	return &WebhookServer{
		Ready:   false,
		Channel: make(chan struct{}, 1),
	}
}

func (ws *WebhookServer) Start(config configuration.Config, p provider.Provider) {
	api.StartHTTPApi(p, ws.Channel, 0, 0, fmt.Sprintf("%s:%d", config.ServerHost, config.ServerPort))
}

func (ws *WebhookServer) StartHealth(config configuration.Config) {
	go func() {
		listenAddr := fmt.Sprintf("0.0.0.0:%d", config.HealthCheckPort)
		m := http.NewServeMux()
		m.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-ws.Channel:
				ws.Ready = true
			default:
			}
			if ws.Ready {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		})
		s := &http.Server{
			Addr:    listenAddr,
			Handler: m,
		}

		l, err := net.Listen("tcp", listenAddr)
		if err != nil {
			log.Fatal(err)
		}
		err = s.Serve(l)
		if err != nil {
			log.Fatalf("[ERROR] Health listener stopped: %s", err)
		}
	}()
}
