package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Smet1/golang-proxy/internal/app/proxy"
)

func main() {
	var pemPath string
	flag.StringVar(&pemPath, "pem", "server.pem", "path to pem file")
	var keyPath string
	flag.StringVar(&keyPath, "key", "server.key", "path to key file")
	var proto string
	flag.StringVar(&proto, "proto", "http", "Proxy protocol (http or https)")
	flag.Parse()
	if proto != "http" && proto != "https" {
		log.Fatal("Protocol must be either http or https")
	}

	config := proxy.Config{}
	proxyService := proxy.Service{
		Config: config,
	}

	server := proxyService.GetServer()
	if proto == "http" {
		go func() {
			log.Printf("http service started on port %s", ":8888")
			if err := server.ListenAndServe(); err != nil {
				if err == http.ErrServerClosed {
					log.Println("graceful shutdown")
				} else {
					log.Fatalf("sync service, err: %s", err)
				}
			}
		}()
	} else {
		go func() {
			log.Printf("https service started on port %s", ":8888")
			if err := server.ListenAndServeTLS(pemPath, keyPath); err != nil {
				if err == http.ErrServerClosed {
					log.Println("graceful shutdown")
				} else {
					log.Fatalf("sync service, err: %s", err)
				}
			}
		}()
	}

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

	log.Printf("stopping, signal: %s", stop)
}
