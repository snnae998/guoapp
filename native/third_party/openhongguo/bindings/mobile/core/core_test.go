package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/bindings/internal/fixtures"
)

type chanCallback chan string

func (c chanCallback) Done(out string) { c <- out }

func newTestClient(t *testing.T, origin string) (*Client, string) {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "sub", "anonymous.json")
	cfg, _ := json.Marshal(map[string]any{"apiOrigin": origin, "registerOrigin": origin, "retryAttempts": 1})
	c, err := NewClient(profile, string(cfg), nil)
	if err != nil {
		t.Fatal(err)
	}
	return c, profile
}

func await(t *testing.T, c chanCallback) map[string]json.RawMessage {
	t.Helper()
	select {
	case out := <-c:
		var env map[string]json.RawMessage
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return env
	case <-time.After(10 * time.Second):
		t.Fatal("no callback")
	}
	return nil
}

func TestStart(t *testing.T) {
	srv := httptest.NewServer(fixtures.Handler("../../../testdata/protocol"))
	defer srv.Close()
	c, profile := newTestClient(t, srv.URL)

	done := make(chanCallback, 1)
	c.Start("search", `["短剧"]`, done)
	env := await(t, done)
	var page struct {
		Items []struct{ ID string } `json:"items"`
	}
	if err := json.Unmarshal(env["result"], &page); err != nil || len(page.Items) == 0 {
		t.Fatalf("result = %s (%v)", env["result"], err)
	}
	// 设备档案写进了 profilePath，目录自动创建。
	if _, err := os.Stat(profile); err != nil {
		t.Fatal(err)
	}

	c.Start("nope", `[]`, done)
	var e struct{ Code string }
	if err := json.Unmarshal(await(t, done)["error"], &e); err != nil || e.Code != "INVALID_OPTIONS" {
		t.Fatalf("error = %+v (%v)", e, err)
	}
}

func TestCancel(t *testing.T) {
	// 不应答的服务：请求一直挂着，直到被取消。
	block := make(chan struct{})
	srv := httptest.NewServer(httptestHandler(block))
	defer srv.Close()
	defer close(block)
	c, _ := newTestClient(t, srv.URL)

	done := make(chanCallback, 1)
	call := c.Start("search", `["短剧"]`, done)
	time.Sleep(100 * time.Millisecond)
	call.Cancel()
	var e struct{ Code string }
	if err := json.Unmarshal(await(t, done)["error"], &e); err != nil || e.Code != "CANCELLED" {
		t.Fatalf("error = %+v (%v)", e, err)
	}
}

func TestNewClientErrors(t *testing.T) {
	for name, tc := range map[string]struct{ profile, cfg string }{
		"no profile": {"", ""},
		"bad JSON":   {"x.json", "{"},
		"bad device": {"x.json", `{"device":{"model":"x; evil"}}`},
	} {
		_, err := NewClient(tc.profile, tc.cfg, nil)
		var e struct{ Code string }
		if err == nil || json.Unmarshal([]byte(err.Error()), &e) != nil || e.Code != "INVALID_OPTIONS" {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func httptestHandler(block chan struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	})
}
