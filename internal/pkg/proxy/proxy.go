package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

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

func GetBurstHandler(client *http.Client, db *sqlx.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		log := getTraceLogger(req.Context())
		log.WithField("req", req).Info("got")

		query := req.URL.Query()
		_, err := strconv.Atoi(query.Get("id"))
		if err != nil {
			ErrResponse(res, http.StatusBadRequest, "invalid id in query")

			log.WithError(err).Error("can't convert id to int")
			return
		}

	}
}

func GetHandleHTTP(client *http.Client, db *sqlx.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		log := getTraceLogger(req.Context())
		log.WithField("req", req).Info("got")

		id, err := SaveUserRequest(db, req)
		if err != nil {
			log.WithError(err).Error("can't save user request")
		} else {
			res.Header().Add("request_id", strconv.Itoa(id))
		}

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
