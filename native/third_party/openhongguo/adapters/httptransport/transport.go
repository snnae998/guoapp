// Package httptransport 是 SDK 默认的网络请求实现（sdk.Transport），基于 net/http。
//
// 它只发送匿名请求：带 Authorization 或账号 Cookie 的请求一律拒绝，
// 也不保存 Cookie。
package httptransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beilunyang/openhongguo/internal/protocol"
	"github.com/beilunyang/openhongguo/sdk"
)

// Transport 发送匿名 HTTP 请求，可以并发使用。
type Transport struct {
	client *http.Client
	logger sdk.Logger
}

// New 返回单个请求超时 30 秒的 Transport。
func New() *Transport {
	return &Transport{client: &http.Client{Timeout: 30 * time.Second}}
}

// SetLogger 让每个请求的方法、脱敏后的 URL 和状态码写进 logger。
func (t *Transport) SetLogger(logger sdk.Logger) { t.logger = logger }

// Do 发送 req。响应体最多读取 8 MB，保持原样，不自动解压。
func (t *Transport) Do(ctx context.Context, req *sdk.Request) (*sdk.Response, error) {
	if req == nil {
		return nil, errors.New("httptransport: nil request")
	}
	for name, value := range req.Headers {
		if strings.EqualFold(name, "Authorization") || (strings.EqualFold(name, "Cookie") && !guestCookieAllowed(value)) {
			return nil, fmt.Errorf("%w: credential header refused", sdk.ErrAuthenticationRequired)
		}
	}
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	hreq, err := http.NewRequestWithContext(ctx, method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		hreq.Header.Set(k, v)
	}
	resp, err := t.client.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if t.logger != nil {
		t.logger.Printf("HTTP %s %s -> %d", method, protocol.RedactedURL(req.URL), resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	for k, values := range resp.Header {
		if strings.EqualFold(k, "Set-Cookie") || strings.EqualFold(k, "Cookie") {
			continue
		}
		if len(values) > 0 {
			headers[k] = values[0]
		}
	}
	cookies := map[string]string{}
	for _, cookie := range resp.Cookies() {
		if cookie.Name != "" {
			cookies[cookie.Name] = cookie.Value
		}
	}
	return &sdk.Response{StatusCode: resp.StatusCode, Headers: headers, Body: body, Cookies: cookies}, nil
}

// guestCookieAllowed 只放行 SDK 为匿名设备构造的 Cookie；
// 账号 Cookie（包括 session）一律拒绝。
func guestCookieAllowed(value string) bool {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "session") || strings.Contains(lower, "odin_tt") {
		return false
	}
	return strings.Contains(value, "passport_csrf_token=;") &&
		strings.Contains(value, "install_id=") &&
		strings.Contains(value, "ttreq=")
}
