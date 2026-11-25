package protocol

import (
	"net/url"
	"strconv"
	"time"
)

// URLLifetime reads the signed lifetime out of an image or media URL.
//
// Image URLs carry "x-expires" (unix seconds) next to "x-signature"; the
// signature covers the path and the size template, so neither can be edited
// client-side. A URL without the parameter returns the zero time.
func URLLifetime(raw string) time.Time {
	u, err := url.Parse(raw)
	if err != nil {
		return time.Time{}
	}
	raw2 := u.Query().Get("x-expires")
	if raw2 == "" {
		return time.Time{}
	}
	secs, err := strconv.ParseInt(raw2, 10, 64)
	if err != nil || secs <= 0 {
		return time.Time{}
	}
	return time.Unix(secs, 0)
}
