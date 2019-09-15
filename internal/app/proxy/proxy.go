package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"github.com/Smet1/golang-proxy/internal/pkg/proxy"
	"github.com/go-chi/chi"
)

type Service struct {
	Config Config
	router *chi.Mux
}

func (s *Service) GetServer() *http.Server {
	s.router = chi.NewMux()
	s.router.Route("/", func(r chi.Router) {
		r.Connect("/", (&ochttp.Handler{
			Handler: ochttp.WithRouteTag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Println("connect")
				proxy.HandleTunneling(w, r)
			}), "connect"),
		}).ServeHTTP)
		r.HandleFunc("/", (&ochttp.Handler{
			Handler: ochttp.WithRouteTag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Println("http")
				proxy.HandleHTTP(w, r)
			}), "handle http"),
		}).ServeHTTP)
	})

	return &http.Server{
		Addr:    ":8888",
		Handler: s.router,
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}
