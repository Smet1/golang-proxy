package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/Smet1/golang-proxy/internal/pkg/httpclients"

	"github.com/Smet1/golang-proxy/internal/pkg/configreader"

	"github.com/Smet1/golang-proxy/internal/app/proxy"
	"github.com/onrik/logrus/filename"
	"github.com/sirupsen/logrus"
)

type codeRecorder struct {
	http.ResponseWriter

	code int
}

func (w *codeRecorder) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.code = code
}

var (
	hostname, _ = os.Hostname()

	dir      = path.Join(os.Getenv("HOME"), "Desktop", "golang-proxy")
	keyFile  = path.Join(dir, "ca-key.pem")
	certFile = path.Join(dir, "ca-cert.pem")
)

func main() {
	configPath := flag.String(
		"config",
		"./config.yaml",
		"path of proxy server config",
	)
	flag.Parse()

	filenameHook := filename.NewHook()
	filenameHook.Field = "sourcelog"

	log := logrus.New()
	log.AddHook(filenameHook)

	config := proxy.Config{}
	err := configreader.ReadConfig(*configPath, &config)
	if err != nil {
		log.WithError(err).Fatal("can't read config")
	}

	logrus.WithField("config", config).Info("started with data")

	err = config.Validate()
	if err != nil {
		log.WithError(err).Fatal("not valid config")
	}

	ca, err := loadCA()
	if err != nil {
		log.Fatal(err)
	}
	proxyService := proxy.Service{
		Config: config,
		CA:     &ca,
		Client: httpclients.HTTPClient(),
		Wrap: func(upstream http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cr := &codeRecorder{ResponseWriter: w}
				log.Println("Got Content-Type:", r.Header.Get("Content-Type"))
				upstream.ServeHTTP(cr, r)
				log.Println("Got Status:", cr.code)
			})
		},
	}

	serverBurst := proxyService.GetServerBurst(log)

	go func() {
		logrus.WithField("port", config.ServeAddrBurst).Info("burst service started")
		if err := serverBurst.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Println("graceful shutdown")
			} else {
				log.Fatalf("burst service, err: %s", err)
			}
		}
	}()

	log.Fatal(http.ListenAndServe(":8888", &proxyService))

	sgnl := make(chan os.Signal, 1)
	signal.Notify(sgnl,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	stop := <-sgnl

	if err := serverBurst.Shutdown(context.Background()); err != nil {
		log.Fatal("error on shutdown")
	}

	log.Printf("stopping, signal: %s", stop)
}

func loadCA() (cert tls.Certificate, err error) {
	// TODO(kr): check file permissions
	cert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if os.IsNotExist(err) {
		cert, err = genCA()
	}
	if err == nil {
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	}
	return
}

func genCA() (cert tls.Certificate, err error) {
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		return
	}
	certPEM, keyPEM, err := proxy.GenCA(hostname)
	if err != nil {
		return
	}
	cert, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return cert, err
	}
	err = ioutil.WriteFile(certFile, certPEM, 0400)
	if err == nil {
		err = ioutil.WriteFile(keyFile, keyPEM, 0400)
	}
	return cert, err
}
