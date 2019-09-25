package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/pkg/errors"

	"github.com/gorilla/mux"

	"golang.org/x/crypto/acme/autocert"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/proxy"
	"gopkg.in/mgo.v2"
)

type Service struct {
	Config      Config
	CertManager *autocert.Manager
	Router      *mux.Router
	Client      *http.Client
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

func (s *Service) EnsureDBConn(config *DB) error {
	session, err := mgo.DialWithInfo(&mgo.DialInfo{
		Addrs:    []string{fmt.Sprintf("%s:%s", config.Host, config.Port)},
		Timeout:  config.Timeout.Duration,
		Database: config.DatabaseName,
	})
	if err != nil {
		return errors.Wrap(err, "can't dial mongo")
	}

	s.Collection = session.DB(config.DatabaseName).C(config.CollectionName)

	return nil
}

func (s *Service) GetServerBurst(logger *logrus.Logger) *http.Server {
	handlerBurst := proxy.GetBurstHandler(s.Client, s.Collection)

	s.Router = mux.NewRouter()
	s.Router.HandleFunc("/burst", handlerBurst).Methods(http.MethodPost)
	return &http.Server{
		Addr:    s.Config.ServeAddrBurst,
		Handler: s.Router,
	}
}

func (s *Service) cert(names ...string) (*tls.Certificate, error) {
	return genCert(s.CA, names)
}

func (s *Service) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		var sconn *tls.Conn

		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			s.Log.WithError(err).Error("cannot determine cert name")
			proxy.ErrResponse(res, http.StatusForbidden, "no upstream")

			return
		}

		provisionalCert, err := s.cert(host)
		if err != nil {
			s.Log.WithError(err).Error("cert")
			proxy.ErrResponse(res, http.StatusForbidden, "no upstream")

			return
		}

		sConfig := &tls.Config{}
		if s.TLSServerConfig != nil {
			sConfig = s.TLSServerConfig
		}
		sConfig.Certificates = []tls.Certificate{*provisionalCert}
		sConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cConfig := &tls.Config{}
			if s.TLSClientConfig != nil {
				cConfig = s.TLSClientConfig
			}
			cConfig.ServerName = hello.ServerName
			sconn, err = tls.Dial("tcp", req.Host, cConfig)
			if err != nil {
				log.Println("dial", req.Host, err)
				s.Log.WithError(err).WithField("host", req.Host).Error("got err on tls dial")

				return nil, err
			}
			return s.cert(hello.ServerName)
		}

		cconn, err := handshake(res, sConfig, s.Log)
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
		_ = http.Serve(&oneShotListener{wc}, s.Wrap(rp))
		<-ch

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

func handshake(res http.ResponseWriter, config *tls.Config, logger *logrus.Logger) (net.Conn, error) {
	raw, _, err := res.(http.Hijacker).Hijack()
	if err != nil {
		logger.WithError(err).Error("no upstream")
		proxy.ErrResponse(res, http.StatusForbidden, "no upstream")

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
