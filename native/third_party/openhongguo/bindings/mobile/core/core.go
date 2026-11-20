// Package core 是 Android 和 iOS 绑定的 Go 入口，由 gomobile bind 编译成
// .aar 和 .xcframework。
//
// gomobile 只能导出字符串、数字和接口这类简单类型，所以这里只有一个按名字
// 调用的入口：Client.Start(method, argsJSON, callback) 在 goroutine 里调用
// SDK，结束时把 {"result": …} 或 {"error": {code, message, retryable,
// retryAfterMs}} 形式的 JSON 交给 callback.Done。Kotlin 和 Swift 的封装再把
// JSON 解码成各自的类型；协程或 Task 取消时调用 Start 返回的 Call.Cancel。
//
// 请求用 Go 的 net/http 直接发送，设备档案存成 profilePath 指向的 JSON 文件。
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/bindings/internal/dispatch"
	"github.com/beilunyang/openhongguo/sdk"
)

// Logger 接收诊断日志；日志里的 URL 已经脱敏。
type Logger interface {
	Log(line string)
}

// Client 是一个 SDK 客户端，可以在多个线程里同时使用。
type Client struct {
	c *sdk.Client
}

// NewClient 创建客户端。profilePath 是设备档案的路径，目录不存在时自动创建；
// configJSON 是 dispatch.Config 的 JSON，可以为空；logger 可以为 nil。
// 失败时，错误信息是 {code, message, retryable, retryAfterMs} 形式的 JSON。
func NewClient(profilePath, configJSON string, logger Logger) (*Client, error) {
	cfg, err := dispatch.ParseConfig(configJSON)
	if err != nil {
		return nil, jsonError(err)
	}
	var opts sdk.Options
	cfg.Apply(&opts)
	if logger != nil {
		opts.Logger = logf{logger}
	}
	c, err := adapters.NewClient(profilePath, opts)
	if err != nil {
		return nil, jsonError(err)
	}
	return &Client{c: c}, nil
}

// Callback 接收一次调用的结果，在 Go 的线程里调用，每次调用恰好一次。
type Callback interface {
	Done(resultJSON string)
}

// Start 在后台开始调用 method，argsJSON 是参数数组的 JSON。
func (c *Client) Start(method, argsJSON string, callback Callback) *Call {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		v, err := dispatch.Call(ctx, c.c, method, argsJSON)
		callback.Done(envelope(v, err))
	}()
	return &Call{cancel: cancel}
}

// Call 是一次进行中的调用。
type Call struct {
	cancel context.CancelFunc
}

// Cancel 取消调用；已经结束的调用不受影响。被取消的调用照样调用
// Callback.Done，结果是 CANCELLED 错误。
func (c *Call) Cancel() { c.cancel() }

func envelope(v any, err error) string {
	var out any
	if err != nil {
		out = map[string]any{"error": dispatch.Error(err)}
	} else {
		out = map[string]any{"result": v}
	}
	b, err := json.Marshal(out)
	if err != nil {
		b, _ = json.Marshal(map[string]any{"error": dispatch.Error(err)})
	}
	return string(b)
}

// jsonError 返回信息是错误对象 JSON 的错误，供另一端解析出错误码。
// gomobile 把错误值当作 map 的键，所以错误类型必须可以比较。
func jsonError(err error) error {
	b, _ := json.Marshal(dispatch.Error(err))
	return errors.New(string(b))
}

type logf struct{ l Logger }

func (l logf) Printf(format string, args ...any) { l.l.Log(fmt.Sprintf(format, args...)) }
