package proxy

import "net/url"

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
