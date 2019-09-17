package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Smet1/golang-proxy/internal/pkg/httpclients"

	"github.com/Smet1/golang-proxy/internal/pkg/configreader"

	"github.com/Smet1/golang-proxy/internal/app/proxy"
	"github.com/onrik/logrus/filename"
	"github.com/sirupsen/logrus"
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

	proxyService := proxy.Service{
		Config: config,
		Client: httpclients.HTTPClient(),
	}

	server := proxyService.GetServerProxy(log)
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
	go func() {
		logrus.WithField("port", config.ServeAddrProxy).Info("https service started")
		if err := server.ListenAndServeTLS(config.Certificate.Pem, config.Certificate.Key); err != nil {
			if err == http.ErrServerClosed {
				log.Println("graceful shutdown")
			} else {
				log.Fatalf("proxy service, err: %s", err)
			}
		}
	}()

	sgnl := make(chan os.Signal, 1)
	signal.Notify(sgnl,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	stop := <-sgnl

	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatal("error on shutdown")
	}
	if err := serverBurst.Shutdown(context.Background()); err != nil {
		log.Fatal("error on shutdown")
	}

	log.Printf("stopping, signal: %s", stop)
}
