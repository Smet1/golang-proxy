package proxy

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/lib/pq"

	"github.com/pkg/errors"

	"github.com/gorilla/mux"

	"golang.org/x/crypto/acme/autocert"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/proxy"
	"github.com/jmoiron/sqlx"
)

type Service struct {
	Config      Config
	CertManager *autocert.Manager
	router      *mux.Router
	Client      *http.Client
	ConnDB      *sqlx.DB
}

func (s *Service) ensureDBConn() error {
	v := url.Values{}
	v.Add("sslmode", s.Config.DB.SSLMode)

	p := url.URL{
		Scheme:     s.Config.DB.Database,
		Opaque:     "",
		User:       url.UserPassword(s.Config.DB.Username, s.Config.DB.Password),
		Host:       s.Config.DB.Host,
		Path:       s.Config.DB.Name,
		RawPath:    "",
		ForceQuery: false,
		RawQuery:   v.Encode(),
		Fragment:   "",
	}

	connectURL, err := pq.ParseURL(p.String())
	if err != nil {
		return errors.Wrap(err, "can't create url for db connection")
	}

	instance, err := sqlx.Connect(s.Config.DB.Database, connectURL)
	if err != nil {
		return errors.Wrap(err, "can't connect db")
	}

	s.ConnDB = instance

	return nil
}

func (s *Service) GetServer(log *logrus.Logger) *http.Server {
	err := s.ensureDBConn()
	if err != nil {
		log.WithError(err).Fatal("can't ensure connection")
	}

	handlerHTTPS := proxy.GetHandleTunneling(s.Config.Timeout.Duration)
	handlerHTTP := proxy.GetHandleHTTP(s.Client, s.ConnDB)
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
