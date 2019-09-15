package proxy

import (
	"crypto/tls"
	"net/http"

	"github.com/Smet1/golang-proxy/internal/pkg/logger"

	"github.com/go-chi/chi"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/proxy"
)

type Service struct {
	Config Config
	router *chi.Mux
}

func (s *Service) GetServer(log logrus.Logger) *http.Server {
	s.router = chi.NewMux()

	s.router.Use(logger.GetLoggerMiddleware(log))

	s.router.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			proxy.GetHandleTunneling(s.Config.Timeout.Duration)
		} else {
			proxy.HandleHTTP(w, r)
		}
	}))

	return &http.Server{
		Addr:    s.Config.ServeAddr,
		Handler: s.router,
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}
