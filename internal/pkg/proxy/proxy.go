package proxy

import (
	"context"
	"io/ioutil"
	"net/http"

	"gopkg.in/mgo.v2"

	"github.com/sirupsen/logrus"

	"github.com/Smet1/golang-proxy/internal/pkg/logger"
	"github.com/google/uuid"
)

func getTraceLogger(ctx context.Context) *logrus.Logger {
	log := logger.GetLogger(ctx)
	log.WithField("TraceID", uuid.New().String())
	return log
}

func GetBurstHandler(client *http.Client, col *mgo.Collection) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		log := getTraceLogger(req.Context())
		log.WithField("req", req).Info("got")

		query := req.URL.Query()
		id := query.Get("id")

		savedReq, err := GetSavedRequest(id, col)
		if err != nil {
			ErrResponse(res, http.StatusBadRequest, "can't get request")

			log.WithError(err).Error("can't get request")
			return
		}

		savedReq.RequestURI = ""
		resp, err := client.Do(savedReq)
		if err != nil {
			ErrResponse(res, http.StatusInternalServerError, "can't do request")

			log.WithError(err).Error("can't do request")
			return
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			ErrResponse(res, http.StatusInternalServerError, "can't do read response body")

			log.WithError(err).Error("can't do read response body")
			return
		}
		ResponseBinaryObject(res, resp.StatusCode, b)
	}
}
