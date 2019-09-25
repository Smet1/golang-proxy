package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type errorResponse struct {
	Message string `json:"message"`
}

func addOkHeader(res http.ResponseWriter) {
	res.WriteHeader(http.StatusOK)
}

func addBody(res io.Writer, bodyMessage interface{}) {
	marshalBody, err := json.Marshal(bodyMessage)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, _ = res.Write(marshalBody)
}

func OkResponse(res http.ResponseWriter, bodyMessage interface{}) {
	addOkHeader(res)
	addBody(res, bodyMessage)
}

func addErrHeader(res http.ResponseWriter, errCode int) {
	res.WriteHeader(errCode)
}

func addErrBody(res io.Writer, errMsg string) {
	addBody(res, errorResponse{Message: errMsg})
}

func ErrResponse(res http.ResponseWriter, errCode int, errMsg string) {
	res.Header().Set("Content-Type", "application/json")
	addErrHeader(res, errCode)
	addErrBody(res, errMsg)
}

func ResponseBinaryObject(res http.ResponseWriter, code int, body []byte) {
	res.Header().Set("Content-Type", "application/json")
	addErrHeader(res, code)
	_, _ = res.Write(body)
}
