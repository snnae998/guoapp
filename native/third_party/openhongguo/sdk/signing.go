package sdk

import "context"

// SignInput 是 Signer 要签名的规范化请求。
type SignInput struct {
	// CanonicalRequest 是请求的字节，采用签名所覆盖的形式。
	CanonicalRequest []byte
}

// SignedHeaders 是 Signer 返回的请求头名称和值。
type SignedHeaders map[string]string

// Signer 生成请求签名。第一阶段不调用原生签名器。
type Signer interface {
	Sign(ctx context.Context, input SignInput) (SignedHeaders, error)
}

// UnavailableSigner 表示所请求的签名无法用可移植的纯 Go 实现。
type UnavailableSigner struct{}

// Sign 总是以 ErrSignerUnavailable 失败。
func (UnavailableSigner) Sign(context.Context, SignInput) (SignedHeaders, error) {
	return nil, ErrSignerUnavailable
}
