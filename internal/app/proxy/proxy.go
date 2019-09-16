package proxy

import (
	"crypto/tls"
	"net/http"

	"github.com/gorilla/mux"

	"golang.org/x/crypto/acme/autocert"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/proxy"
)

type Service struct {
	Config      Config
	CertManager *autocert.Manager
	router      *mux.Router
	Client      *http.Client
}

func (s *Service) GetServer(log *logrus.Logger) *http.Server {
	handlerHTTPS := proxy.GetHandleTunneling(s.Config.Timeout.Duration)
	handlerHTTP := proxy.GetHandleHTTP(s.Client)
	s.router = mux.NewRouter()
	s.router.HandleFunc("/", handlerHTTPS).Methods(http.MethodConnect)
	s.router.HandleFunc("/", handlerHTTP)

	return &http.Server{

		Addr:    s.Config.ServeAddr,
		Handler: s.router,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}
