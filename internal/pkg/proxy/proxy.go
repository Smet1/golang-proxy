package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/logger"
	"github.com/google/uuid"
)

func getTraceLogger(ctx context.Context) *logrus.Logger {
	log := logger.GetLogger(ctx)
	log.WithField("TraceID", uuid.New().String())
	return log
}

func GetHandleTunneling(timeout time.Duration) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		log := getTraceLogger(req.Context())
		ctx := req.Context()

		log.WithField("req", req).Info("got")

		destConn, err := net.DialTimeout("tcp", req.Host, timeout)
		if err != nil {
			http.Error(res, err.Error(), http.StatusServiceUnavailable)
			return
		}
		res.WriteHeader(http.StatusOK)
		hijacker, ok := res.(http.Hijacker)
		if !ok {
			http.Error(res, "Hijacking not supported", http.StatusInternalServerError)
			return
		}
		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(res, err.Error(), http.StatusServiceUnavailable)
		}

		wg := &sync.WaitGroup{}

		wg.Add(2)
		go transfer(ctx, destConn, clientConn, wg)
		go transfer(ctx, clientConn, destConn, wg)
		wg.Wait()

		log.Info("ok handled")
	}
}

func transfer(ctx context.Context, destination io.WriteCloser, source io.ReadCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	defer destination.Close()
	defer source.Close()

	log := logger.GetLogger(ctx)

	_, err := io.Copy(destination, source)
	if err != nil {
		log.WithError(err).Error("can't copy from destination to source")
	}
}

func GetHandleHTTP(client *http.Client) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		log := getTraceLogger(req.Context())
		log.WithField("req", req).Info("got")

		//resp, err := http.DefaultTransport.RoundTrip(req)
		//if err != nil {
		//	http.Error(res, err.Error(), http.StatusServiceUnavailable)
		//	return
		//}
		req.RequestURI = ""
		resp, err := client.Do(req)
		if err != nil {
			http.Error(res, err.Error(), http.StatusServiceUnavailable)
			log.WithField("err", err).Error("can't do request")
			return
		}

		defer resp.Body.Close()
		copyHeader(res.Header(), resp.Header)
		res.WriteHeader(resp.StatusCode)

		_, err = io.Copy(res, resp.Body)
		if err != nil {
			log.WithError(err).Error("can't copy from destination to source")
		}
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
