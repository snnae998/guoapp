package httptransport

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/beilunyang/openhongguo/sdk"
)

func TestTransportRefusesCredentialHeaders(t *testing.T) {
	tr := New()
	_, err := tr.Do(context.Background(), &sdk.Request{
		Method: http.MethodGet, URL: "http://127.0.0.1/unused",
		Headers: map[string]string{"Cookie": "session=nope"},
	})
	if !errors.Is(err, sdk.ErrAuthenticationRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestTransportAllowsAnonymousGuestCookie(t *testing.T) {
	tr := New()
	_, err := tr.Do(context.Background(), &sdk.Request{
		Method: http.MethodGet, URL: "http://127.0.0.1:1/unused",
		Headers: map[string]string{"Cookie": "passport_csrf_token=; passport_csrf_token_default=; store-region=cn; store-region-src=did; install_id=1; ttreq=abc"},
	})
	if errors.Is(err, sdk.ErrAuthenticationRequired) {
		t.Fatal("guest cookie refused")
	}
}
