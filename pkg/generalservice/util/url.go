package util

import (
	"net/url"
	"path"
)

func JoinUrl(baseURL string, subPath ...string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	for _, sub := range subPath {
		u.Path = path.Join(u.Path, sub)
	}
	return u.String()
}
