package proxy

import (
	"crypto/tls"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/proxy"
)

type Service struct {
	Config Config
}

func (s *Service) GetServer(log *logrus.Logger) *http.Server {
	handlerHTTPS := proxy.GetHandleTunneling(s.Config.Timeout.Duration)
	handlerHTTP := proxy.GetHandleHTTP()

	return &http.Server{
		Addr: s.Config.ServeAddr,
		//Handler: s.router,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handlerHTTPS(w, r)
			} else {
				handlerHTTP(w, r)
			}
		}),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}
