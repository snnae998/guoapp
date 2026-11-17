package sdk

import "context"

// AuthProvider 可以附加账号状态。第一阶段只提供匿名实现。
type AuthProvider interface {
	// Apply 把账号的请求头或参数加到 req 上。
	Apply(ctx context.Context, req *Request) error
}

// AnonymousProvider 不添加任何账号凭据。
type AnonymousProvider struct{}

// Apply 保持请求原样不变。
func (AnonymousProvider) Apply(context.Context, *Request) error { return nil }
