//go:build js && wasm

// Command wasm 把 SDK 编译成 WebAssembly，供 npm 包 openhongguo 使用。
//
// 它在 JS 全局对象上注册 __openhongguo.newClient。网络请求、设备档案的读写
// 和日志都通过回调交给 JS 完成；SDK 的每个方法都经由 call(name, argsJSON)
// 调用，返回一个 Promise，结果是 JSON 字符串。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/beilunyang/openhongguo/bindings/internal/dispatch"
	"github.com/beilunyang/openhongguo/sdk"
)

func main() {
	js.Global().Set("__openhongguo", js.ValueOf(map[string]any{
		"newClient": js.FuncOf(newClient),
	}))
	select {}
}

// newClient(config) 创建客户端。config 包含：
//
//   - transport(request) => Promise<response>：发送一个请求；
//   - profile.load() => Promise<string|null>、profile.save(json) =>
//     Promise、profile.reset() => Promise：读写设备档案；
//   - log(line)：可选，接收诊断日志；
//   - options：dispatch.Config 的 JSON 字符串。
//
// 成功时返回 { call }，失败时返回 { error }。
func newClient(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return map[string]any{"error": dispatch.Error(fmt.Errorf("%w: missing config", sdk.ErrInvalidOptions))}
	}
	cfg := args[0]
	var raw string
	if v := cfg.Get("options"); v.Type() == js.TypeString {
		raw = v.String()
	}
	opts, err := dispatch.ParseConfig(raw)
	if err != nil {
		return map[string]any{"error": dispatch.Error(err)}
	}
	options := sdk.Options{
		Transport:    jsTransport{fn: cfg.Get("transport")},
		ProfileStore: jsProfileStore{v: cfg.Get("profile")},
	}
	opts.Apply(&options)
	if logFn := cfg.Get("log"); logFn.Type() == js.TypeFunction {
		options.Logger = jsLogger{fn: logFn}
	}
	client, err := sdk.NewClient(options)
	if err != nil {
		return map[string]any{"error": dispatch.Error(err)}
	}
	return map[string]any{"call": js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return rejected(fmt.Errorf("%w: call needs a method name and arguments", sdk.ErrInvalidOptions))
		}
		return promise(func() (any, error) {
			return dispatch.Call(context.Background(), client, args[0].String(), args[1].String())
		})
	})}
}

// promise 在新的 goroutine 里运行 f，并返回对应的 JS Promise。
// f 可以等待 JS 的 Promise，因为它不在 JS 的事件循环里运行。
func promise(f func() (any, error)) js.Value {
	var executor js.Func
	executor = js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve, reject := args[0], args[1]
		go func() {
			defer executor.Release()
			out, err := f()
			if err != nil {
				reject.Invoke(dispatch.Error(err))
				return
			}
			b, err := json.Marshal(out)
			if err != nil {
				reject.Invoke(dispatch.Error(err))
				return
			}
			resolve.Invoke(string(b))
		}()
		return nil
	})
	return js.Global().Get("Promise").New(executor)
}

func rejected(err error) js.Value {
	return js.Global().Get("Promise").Call("reject", dispatch.Error(err))
}

// await 等待一个 JS Promise 完成。
func await(p js.Value) (js.Value, error) {
	if p.Type() != js.TypeObject || p.Get("then").Type() != js.TypeFunction {
		return p, nil // 回调直接返回了值，而不是 Promise
	}
	type result struct {
		v   js.Value
		err error
	}
	done := make(chan result, 1)
	onOK := js.FuncOf(func(_ js.Value, args []js.Value) any {
		done <- result{v: arg0(args)}
		return nil
	})
	onErr := js.FuncOf(func(_ js.Value, args []js.Value) any {
		done <- result{err: jsError(arg0(args))}
		return nil
	})
	p.Call("then", onOK, onErr)
	r := <-done
	onOK.Release()
	onErr.Release()
	return r.v, r.err
}

func arg0(args []js.Value) js.Value {
	if len(args) == 0 {
		return js.Undefined()
	}
	return args[0]
}

func jsError(v js.Value) error {
	if v.Type() == js.TypeObject && v.Get("message").Type() == js.TypeString {
		return errors.New(v.Get("message").String())
	}
	return errors.New(v.String())
}

// jsTransport 把请求交给 JS 的 transport 回调发送。
type jsTransport struct{ fn js.Value }

func (t jsTransport) Do(_ context.Context, req *sdk.Request) (*sdk.Response, error) {
	if t.fn.Type() != js.TypeFunction {
		return nil, fmt.Errorf("%w: transport is not a function", sdk.ErrInvalidOptions)
	}
	headers := map[string]any{}
	for k, v := range req.Headers {
		headers[k] = v
	}
	body := js.Global().Get("Uint8Array").New(len(req.Body))
	js.CopyBytesToJS(body, req.Body)
	v, err := await(t.fn.Invoke(map[string]any{
		"method": req.Method, "url": req.URL, "headers": headers, "body": body,
	}))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", sdk.ErrTransport, err)
	}
	resp := &sdk.Response{
		StatusCode: v.Get("status").Int(),
		Headers:    stringMap(v.Get("headers")),
		Cookies:    stringMap(v.Get("cookies")),
	}
	if b := v.Get("body"); b.Type() == js.TypeObject {
		resp.Body = make([]byte, b.Get("length").Int())
		js.CopyBytesToGo(resp.Body, b)
	}
	return resp, nil
}

func stringMap(v js.Value) map[string]string {
	out := map[string]string{}
	if v.Type() != js.TypeObject {
		return out
	}
	keys := js.Global().Get("Object").Call("keys", v)
	for i := 0; i < keys.Length(); i++ {
		k := keys.Index(i).String()
		out[k] = v.Get(k).String()
	}
	return out
}

// jsProfileStore 通过 JS 回调读写设备档案（JSON）。
type jsProfileStore struct{ v js.Value }

func (s jsProfileStore) call(name string, args ...any) (js.Value, error) {
	fn := s.v.Get(name)
	if fn.Type() != js.TypeFunction {
		return js.Undefined(), fmt.Errorf("%w: profile.%s is not a function", sdk.ErrInvalidOptions, name)
	}
	return await(fn.Invoke(args...))
}

func (s jsProfileStore) Load(context.Context) (*sdk.DeviceProfile, error) {
	v, err := s.call("load")
	if err != nil {
		return nil, err
	}
	if v.Type() != js.TypeString || v.String() == "" {
		return nil, sdk.ErrProfileNotFound
	}
	var p sdk.DeviceProfile
	if err := json.Unmarshal([]byte(v.String()), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s jsProfileStore) Save(_ context.Context, p *sdk.DeviceProfile) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.call("save", string(b))
	return err
}

func (s jsProfileStore) Reset(context.Context) error {
	_, err := s.call("reset")
	return err
}

// jsLogger 把诊断日志交给 JS 的 log 回调。日志里的 URL 已经脱敏。
type jsLogger struct{ fn js.Value }

func (l jsLogger) Printf(format string, args ...any) { l.fn.Invoke(fmt.Sprintf(format, args...)) }
