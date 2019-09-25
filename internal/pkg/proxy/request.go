package proxy

import (
	"bytes"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	"gopkg.in/mgo.v2"

	"gopkg.in/mgo.v2/bson"
)

type RequestSave struct {
	ID               bson.ObjectId   `bson:"_id"`
	Method           string          `bson:"method"`
	URL              *url.URL        `bson:"url"`
	Proto            string          `bson:"proto"`
	ProtoMajor       int             `bson:"proto_major"`
	ProtoMinor       int             `bson:"proto_minor"`
	Header           http.Header     `bson:"header"`
	Body             []byte          `bson:"body"`
	ContentLength    int64           `bson:"content_length"`
	TransferEncoding []string        `bson:"transfer_encoding"`
	Host             string          `bson:"host"`
	Form             url.Values      `bson:"form"`
	PostForm         url.Values      `bson:"post_form"`
	MultipartForm    *multipart.Form `bson:"multipart_form"`
	Trailer          http.Header     `bson:"trailer"`
	RemoteAddr       string          `bson:"remote_addr"`
	RequestURI       string          `bson:"request_uri"`
}

func (rs *RequestSave) GetHTTPForm() *http.Request {
	return &http.Request{
		Method:           rs.Method,
		URL:              rs.URL,
		Proto:            rs.Proto,
		ProtoMajor:       rs.ProtoMajor,
		ProtoMinor:       rs.ProtoMinor,
		Header:           rs.Header,
		Body:             ioutil.NopCloser(bytes.NewReader(rs.Body)),
		ContentLength:    rs.ContentLength,
		TransferEncoding: rs.TransferEncoding,
		Host:             rs.Host,
		Form:             rs.Form,
		PostForm:         rs.PostForm,
		MultipartForm:    rs.MultipartForm,
		Trailer:          rs.Trailer,
		RemoteAddr:       rs.RemoteAddr,
		RequestURI:       rs.RequestURI,
	}
}

func SaveRequest(req *http.Request, collection *mgo.Collection) (string, error) {
	b, _ := ioutil.ReadAll(req.Body)
	reqSave := RequestSave{
		ID:               bson.NewObjectId(),
		Method:           req.Method,
		URL:              req.URL,
		Proto:            req.Proto,
		ProtoMajor:       req.ProtoMajor,
		ProtoMinor:       req.ProtoMinor,
		Header:           req.Header,
		Body:             b,
		ContentLength:    req.ContentLength,
		TransferEncoding: req.TransferEncoding,
		Host:             req.Host,
		Form:             req.Form,
		PostForm:         req.PostForm,
		MultipartForm:    req.MultipartForm,
		Trailer:          req.Trailer,
		RemoteAddr:       req.RemoteAddr,
		RequestURI:       req.RequestURI,
	}

	err := collection.Insert(reqSave)
	if err != nil {
		return "", errors.Wrap(err, "can't insert request")
	}

	return reqSave.ID.Hex(), nil
}

func GetSavedRequest(id string, collection *mgo.Collection) (*http.Request, error) {
	r := &RequestSave{}
	err := collection.FindId(bson.ObjectIdHex(id)).One(r)
	if err != nil {
		return nil, errors.Wrap(err, "can't get request")
	}

	return r.GetHTTPForm(), nil
}
