package adapters_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/memstore"
	"github.com/beilunyang/openhongguo/sdk"
)

func TestNewClientSavesProfileAtPath(t *testing.T) {
	body, err := os.ReadFile("../testdata/protocol/register-response.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "dir", "anonymous.json")
	c, err := adapters.NewClient(path, sdk.Options{APIOrigin: srv.URL, RegisterOrigin: srv.URL, RetryAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	// 搜索失败也无妨（服务端只会应答注册）；只看档案是否存到了 path。
	c.Search(context.Background(), "短剧", sdk.SearchOptions{})
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientNeedsProfile(t *testing.T) {
	if _, err := adapters.NewClient("", sdk.Options{}); !errors.Is(err, sdk.ErrInvalidOptions) {
		t.Fatalf("err = %v", err)
	}
	if _, err := adapters.NewClient("", sdk.Options{ProfileStore: memstore.New()}); err != nil {
		t.Fatal(err)
	}
}
