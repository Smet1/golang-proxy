package proxy

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
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
	Router      *mux.Router
	Client      *http.Client
	ConnDB      *sqlx.DB
	Wrap        func(upstream http.Handler) http.Handler
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

func (s *Service) GetServerProxy(log *logrus.Logger) *http.Server {
	err := s.ensureDBConn()
	if err != nil {
		log.WithError(err).Fatal("can't ensure connection")
	}

	handlerHTTPS := proxy.GetHandleTunneling(s.Config.Timeout.Duration)
	handlerHTTP := proxy.GetHandleHTTP(s.Client, s.ConnDB)
	handlerBurst := proxy.GetBurstHandler(s.Client, s.ConnDB)

	s.Router = mux.NewRouter()
	s.Router.HandleFunc("/", handlerHTTPS).Methods(http.MethodConnect)
	s.Router.HandleFunc("/", handlerHTTP)
	s.Router.HandleFunc("/burst", handlerBurst).Methods(http.MethodPost)
	return &http.Server{
		Addr:    s.Config.ServeAddrProxy,
		Handler: s.Router,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
			GetCertificate: func(info *tls.ClientHelloInfo) (certificate *tls.Certificate, e error) {
				log.WithField("RemoteAddr", info.Conn.RemoteAddr()).Info("kek")
				log.WithField("LocalAddr", info.Conn.LocalAddr()).Info("kek1")
				log.WithField("ServerName", info.ServerName).Info("kek2")
				log.Println("here")
				return nil, nil
			},
		},

		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}

func (s *Service) GetServerBurst(log *logrus.Logger) *http.Server {
	err := s.ensureDBConn()
	if err != nil {
		log.WithError(err).Fatal("can't ensure connection")
	}

	handlerBurst := proxy.GetBurstHandler(s.Client, s.ConnDB)

	s.Router = mux.NewRouter()
	s.Router.HandleFunc("/burst", handlerBurst).Methods(http.MethodPost)
	return &http.Server{
		Addr:    s.Config.ServeAddrBurst,
		Handler: s.Router,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},

		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		logrus.Info("connect")
		// handle connect
		return
	}

	rp := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			r.URL.Host = r.Host
			r.URL.Scheme = "http"
		},
		FlushInterval: 0,
	}
	s.Wrap(rp).ServeHTTP(w, r)
}
