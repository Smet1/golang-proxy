package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/lib/pq"

	"github.com/pkg/errors"

	"github.com/gorilla/mux"

	"golang.org/x/crypto/acme/autocert"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/proxy"
	"github.com/jmoiron/sqlx"
	"gopkg.in/mgo.v2"
)

type Service struct {
	Config      Config
	CertManager *autocert.Manager
	Router      *mux.Router
	Client      *http.Client
	ConnDB      *sqlx.DB
	Log         *logrus.Logger
	Wrap        func(upstream http.Handler) http.Handler

	// CA specifies the root CA for generating leaf certs for each incoming
	// TLS request.
	CA *tls.Certificate

	// TLSServerConfig specifies the tls.Config to use when generating leaf
	// cert using CA.
	TLSServerConfig *tls.Config

	// TLSClientConfig specifies the tls.Config to use when establishing
	// an upstream connection for proxying.
	TLSClientConfig *tls.Config
	Collection      *mgo.Collection
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

	session, err := mgo.DialWithInfo(&mgo.DialInfo{
		Addrs:    []string{fmt.Sprintf("%s:%s", "localhost", "27017")},
		Timeout:  10 * time.Second,
		Database: "requests",
	})
	if err != nil {
		return errors.Wrap(err, "can't dial mongo")
	}

	s.Collection = session.DB("requests").C("requests")

	return nil
}

func (s *Service) GetServerProxy(log *logrus.Logger) *http.Server {
	err := s.ensureDBConn()
	if err != nil {
		log.WithError(err).Fatal("can't ensure connection")
	}

	handlerHTTPS := proxy.GetHandleTunneling(s.Config.Timeout.Duration)
	handlerHTTP := proxy.GetHandleHTTP(s.Client, s.ConnDB)
	handlerBurst := proxy.GetBurstHandler(s.Client, s.Collection)

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

	handlerBurst := proxy.GetBurstHandler(s.Client, s.Collection)

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

func (s *Service) cert(names ...string) (*tls.Certificate, error) {
	return genCert(s.CA, names)
}

func (s *Service) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	err := proxy.SaveRequest(req, s.Collection)
	if err != nil {
		s.Log.WithError(err).Error("can't save request")
	}
	if req.Method == http.MethodConnect {
		logrus.WithField("request", req).Info("connect")

		var sconn *tls.Conn

		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			s.Log.WithError(err).Error("cannot determine cert name")
			http.Error(res, "no upstream", 503)
			return
		}

		provisionalCert, err := s.cert(host)
		if err != nil {
			s.Log.WithError(err).Error("cert")
			http.Error(res, "no upstream", 503)
			return
		}

		sConfig := &tls.Config{}
		if s.TLSServerConfig != nil {
			*sConfig = *s.TLSServerConfig
		}
		sConfig.Certificates = []tls.Certificate{*provisionalCert}
		sConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cConfig := &tls.Config{}
			if s.TLSClientConfig != nil {
				*cConfig = *s.TLSClientConfig
			}
			cConfig.ServerName = hello.ServerName
			sconn, err = tls.Dial("tcp", req.Host, cConfig)
			if err != nil {
				log.Println("dial", req.Host, err)
				return nil, err
			}
			return s.cert(hello.ServerName)
		}

		cconn, err := handshake(res, sConfig)
		if err != nil {
			s.Log.WithField("host", req.Host).WithError(err).Error("handshake err")
			return
		}
		defer cconn.Close()
		if sconn == nil {
			s.Log.WithError(err).Error("cannot determine cert name")
			return
		}
		defer sconn.Close()

		od := &oneShotDialer{c: sconn}
		rp := &httputil.ReverseProxy{
			Director:      httpsDirector,
			Transport:     &http.Transport{DialTLS: od.Dial},
			FlushInterval: 0,
		}

		ch := make(chan int)
		wc := &onCloseConn{cconn, func() { ch <- 0 }}
		err = http.Serve(&oneShotListener{wc}, s.Wrap(rp))
		<-ch
		if err != nil {
			s.Log.WithError(err).Error("one shot listener err")
		}

		return
	}

	rp := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			req.URL.Host = req.Host
			req.URL.Scheme = "http"
		},
		FlushInterval: 0,
	}
	s.Wrap(rp).ServeHTTP(res, req)
}

func httpsDirector(r *http.Request) {
	r.URL.Host = r.Host
	r.URL.Scheme = "https"
}

var okHeader = []byte("HTTP/1.1 200 OK\r\n\r\n")

func handshake(w http.ResponseWriter, config *tls.Config) (net.Conn, error) {
	raw, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, "no upstream", 503)
		return nil, err
	}
	if _, err = raw.Write(okHeader); err != nil {
		raw.Close()
		return nil, err
	}
	conn := tls.Server(raw, config)
	err = conn.Handshake()
	if err != nil {
		conn.Close()
		raw.Close()
		return nil, err
	}
	return conn, nil
}

// A oneShotDialer implements net.Dialer whos Dial only returns a
// net.Conn as specified by c followed by an error for each subsequent Dial.
type oneShotDialer struct {
	c  net.Conn
	mu sync.Mutex
}

func (d *oneShotDialer) Dial(network, addr string) (net.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.c == nil {
		return nil, errors.New("closed")
	}
	c := d.c
	d.c = nil
	return c, nil
}

// A oneShotListener implements net.Listener whos Accept only returns a
// net.Conn as specified by c followed by an error for each subsequent Accept.
type oneShotListener struct {
	c net.Conn
}

func (l *oneShotListener) Accept() (net.Conn, error) {
	if l.c == nil {
		return nil, errors.New("closed")
	}
	c := l.c
	l.c = nil
	return c, nil
}

func (l *oneShotListener) Close() error {
	return nil
}

func (l *oneShotListener) Addr() net.Addr {
	return l.c.LocalAddr()
}

// A onCloseConn implements net.Conn and calls its f on Close.
type onCloseConn struct {
	net.Conn
	f func()
}

func (c *onCloseConn) Close() error {
	if c.f != nil {
		c.f()
		c.f = nil
	}
	return c.Conn.Close()
}
