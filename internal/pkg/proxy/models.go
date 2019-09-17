package proxy

import (
	"database/sql"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type UserInfo struct {
	Username string `db:"username"`
	Password string `db:"password"`
}

func (u *UserInfo) FromURL(url *url.URL) {
	u.Username = url.User.Username()
	if p, ok := url.User.Password(); ok {
		u.Password = p
	}
}

func (u *UserInfo) Insert(db *sqlx.DB) (int, error) {
	insert, err := db.NamedQuery(`INSERT INTO user_info(username, password)
		VALUES (:password, :username) RETURNING id`, u)
	if err != nil {
		return 0, errors.Wrap(err, "can't insert user info")
	}
	defer insert.Close()

	res := &sql.NullInt64{}
	for insert.Next() {
		err := insert.Scan(res)
		if err != nil {
			return 0, errors.Wrap(err, "can't get id")
		}
	}

	return int(res.Int64), nil
}

type URL struct {
	Scheme     string `db:"scheme"`
	Opaque     string `db:"opaque"`      // encoded opaque data
	User       int    `db:"user"`        // username and password information
	Host       string `db:"host"`        // host or host:port
	Path       string `db:"path"`        // path (relative paths may omit leading slash)
	RawPath    string `db:"raw_path"`    // encoded path hint (see EscapedPath method)
	ForceQuery bool   `db:"force_query"` // append a query ('?') even if RawQuery is empty
	RawQuery   string `db:"raw_query"`   // encoded query values, without '?'
	Fragment   string `db:"fragment"`    // fragment for references, without '#'
}

func (u *URL) FromURL(url *url.URL) {
	u.Scheme = url.Scheme
	u.Opaque = url.Opaque
	u.Host = url.Host
	u.Path = url.Path
	u.RawPath = url.RawPath
	u.ForceQuery = url.ForceQuery
	u.RawQuery = url.RawQuery
	u.Fragment = url.Fragment
}

func (u *URL) Insert(db *sqlx.DB, id int) (int, error) {
	u.User = id

	insert, err := db.NamedQuery(`INSERT INTO url(scheme, opaque, "user", host, path, raw_path, force_query, raw_query, fragment)
VALUES (:scheme, :opaque, :user, :host, :path, :raw_path, :force_query, :raw_query, :fragment) RETURNING id`, u)
	if err != nil {
		return 0, errors.Wrap(err, "can't insert url")
	}
	defer insert.Close()

	res := &sql.NullInt64{}
	for insert.Next() {
		err := insert.Scan(res)
		if err != nil {
			return 0, errors.Wrap(err, "can't get id")
		}
	}

	return int(res.Int64), nil
}

type Request struct {
	Method     string `db:"method"`
	URL        int    `db:"url"`
	Proto      string `db:"proto"`
	ProtoMajor int    `db:"proto_major"`
	ProtoMinor int    `db:"proto_minor"`
	//Header Header
	Body          []byte `db:"body"`
	ContentLength int64  `db:"content_length"`
	Host          string `db:"host"`
	//Form url.Values
	//PostForm url.Values
	//MultipartForm *multipart.Form
	RemoteAddr string `db:"remote_addr"`
	RequestURI string `db:"request_uri"`
}

func (r *Request) FromRequest(req *http.Request) error {
	r.Method = req.Method
	r.Proto = req.Proto
	r.ProtoMajor = req.ProtoMajor
	r.ProtoMinor = req.ProtoMinor
	bytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return errors.Wrap(err, "can't read request body")
	}
	r.Body = bytes
	r.ContentLength = req.ContentLength
	r.Host = req.Host
	r.RemoteAddr = req.RemoteAddr
	r.RequestURI = req.RequestURI

	return nil
}

func (r *Request) Insert(db *sqlx.DB, id int) (int, error) {
	r.URL = id

	insert, err := db.NamedQuery(`INSERT INTO request (url_id, proto, proto_major, proto_minor, body, content_length, host, remote_addr, request_uri)
VALUES (:url, :proto, :proto_major, :proto_minor, :body, :content_length, :host, :remote_addr, :request_uri) RETURNING id`, r)
	if err != nil {
		return 0, errors.Wrap(err, "can't insert request")
	}

	defer insert.Close()

	res := &sql.NullInt64{}
	for insert.Next() {
		err := insert.Scan(res)
		if err != nil {
			return 0, errors.Wrap(err, "can't get id")
		}
	}

	return int(res.Int64), nil
}

type Header struct {
	Name      string `db:"name"`
	Value     string `db:"value"`
	RequestID int    `db:"request_id"`
}

type Headers struct {
	Arr []Header
}

func (h *Headers) FromRequest(req *http.Request, reqID int) {
	for k, v := range req.Header {
		for _, vv := range v {
			header := Header{
				Name:      k,
				Value:     vv,
				RequestID: reqID,
			}

			h.Arr = append(h.Arr, header)
		}
	}
}

func (h *Headers) Insert(db *sqlx.DB) error {
	for i := range h.Arr {
		insert, err := db.NamedExec(`INSERT INTO header (request_id, name, value)
VALUES (:request_id, :name, :value)`, h.Arr[i])
		if err != nil {
			return errors.Wrap(err, "can't insert header")
		}

		rows, err := insert.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "can't get affected rows")
		}

		if rows == 0 {
			return errors.New("no rows inserted")
		}
	}

	return nil
}

func SaveUserRequest(db *sqlx.DB, req *http.Request) (int, error) {
	ui := &UserInfo{}
	ui.FromURL(req.URL)

	idUserInfo, err := ui.Insert(db)
	if err != nil {
		return 0, errors.Wrap(err, "can't insert user info")
	}

	u := &URL{}
	u.FromURL(req.URL)
	idUrl, err := u.Insert(db, idUserInfo)
	if err != nil {
		return 0, errors.Wrap(err, "can't insert url")
	}

	request := &Request{}
	err = request.FromRequest(req)
	if err != nil {
		return 0, errors.Wrap(err, "can't read request")
	}

	idReq, err := request.Insert(db, idUrl)
	if err != nil {
		return 0, errors.Wrap(err, "can't insert request")
	}

	headers := &Headers{}
	headers.FromRequest(req, idReq)
	err = headers.Insert(db)
	if err != nil {
		return 0, errors.Wrap(err, "can't insert headers")
	}

	return idReq, nil
}
