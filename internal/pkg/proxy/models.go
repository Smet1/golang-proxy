package proxy

import (
	"database/sql"
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
		insert.Scan(res)
	}

	return int(res.Int64), nil
}
