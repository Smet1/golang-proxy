package proxy

import "github.com/go-chi/chi"

type Service struct {
	Config Config
	Router *chi.Mux
}
