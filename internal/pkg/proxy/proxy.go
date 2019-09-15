package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/logger"
	"github.com/google/uuid"
)

func getTraceLogger(ctx context.Context) logrus.Logger {
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
		go transfer(ctx, destConn, clientConn)
		go transfer(ctx, clientConn, destConn)

		log.Info("ok handled")
	}
}

func transfer(ctx context.Context, destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()

	log := logger.GetLogger(ctx)

	_, err := io.Copy(destination, source)
	if err != nil {
		log.WithError(err).Error("can't copy from destination to source")
	}
}

func HandleHTTP(w http.ResponseWriter, req *http.Request) {
	log := getTraceLogger(req.Context())
	log.WithField("req", req).Info("got")

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.WithError(err).Error("can't copy from destination to source")
	}

}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
